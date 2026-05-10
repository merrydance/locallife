package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func (s *BaofuAccountOnboardingService) ApplyAccountOpenResult(ctx context.Context, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult) (BaofuAccountOpenApplyResult, error) {
	if s == nil || s.store == nil {
		return BaofuAccountOpenApplyResult{}, ErrBaofuAccountOnboardingNotConfigured
	}
	normalized := result.Normalized()
	switch normalized.OpenState {
	case db.BaofuAccountOpenStateActive:
		if err := s.ensureAccountOpenResultSerialMatchesFlow(ctx, flow, normalized); err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		if err := s.ensureAccountOpenContractMatchesFlow(ctx, flow, normalized); err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		binding, err := s.applyAccountOpenActive(ctx, flow, normalized)
		if err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateReady ||
			strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateMerchantReportProcessing ||
			strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateAppletAuthPending {
			return BaofuAccountOpenApplyResult{Flow: flow, Binding: &binding}, nil
		}
		if flow.OwnerType == db.BaofuAccountOwnerTypeMerchant {
			updated, err := s.store.MarkBaofuAccountOpeningFlowMerchantReportProcessing(ctx, db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams{
				ID:               flow.ID,
				AccountBindingID: pgtype.Int8{Int64: binding.ID, Valid: binding.ID > 0},
				RawSnapshot:      baofuAccountRawSnapshot(normalized.Raw),
			})
			if err != nil {
				return BaofuAccountOpenApplyResult{}, err
			}
			if s.merchantReportClient != nil {
				merchantReportStore, ok := s.store.(baofuAccountMerchantReportStore)
				if !ok {
					return BaofuAccountOpenApplyResult{}, ErrBaofuMerchantReportServiceNotConfigured
				}
				updated, err = NewBaofuAccountMerchantReportService(merchantReportStore, s.merchantReportClient, s.encryptor, s.merchantReportConfig).RecoverMerchantReportFlow(ctx, updated)
				if err != nil {
					return BaofuAccountOpenApplyResult{}, err
				}
			}
			return BaofuAccountOpenApplyResult{Flow: updated, Binding: &binding}, nil
		}
		updated, err := s.store.MarkBaofuAccountOpeningFlowReady(ctx, db.MarkBaofuAccountOpeningFlowReadyParams{
			ID:               flow.ID,
			AccountBindingID: pgtype.Int8{Int64: binding.ID, Valid: binding.ID > 0},
			RawSnapshot:      baofuAccountRawSnapshot(normalized.Raw),
		})
		if err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		return BaofuAccountOpenApplyResult{Flow: updated, Binding: &binding}, nil
	case db.BaofuAccountOpenStateFailed, db.BaofuAccountOpenStateAbnormal:
		binding, bindingErr := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID})
		if bindingErr != nil && !errors.Is(bindingErr, db.ErrRecordNotFound) {
			return BaofuAccountOpenApplyResult{}, bindingErr
		}
		if bindingErr == nil && strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
			var err error
			if normalized.OpenState == db.BaofuAccountOpenStateFailed {
				if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateFailed {
					binding, err = s.store.MarkBaofuAccountBindingFailed(ctx, db.MarkBaofuAccountBindingFailedParams{ID: binding.ID, RawSnapshot: baofuAccountRawSnapshot(normalized.Raw)})
				}
			} else if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateAbnormal {
				binding, err = s.store.MarkBaofuAccountBindingAbnormal(ctx, db.MarkBaofuAccountBindingAbnormalParams{ID: binding.ID, RawSnapshot: baofuAccountRawSnapshot(normalized.Raw)})
			}
			if err != nil {
				return BaofuAccountOpenApplyResult{}, err
			}
		}
		if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateReady || strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateVoided {
			if bindingErr == nil {
				return BaofuAccountOpenApplyResult{Flow: flow, Binding: &binding}, nil
			}
			return BaofuAccountOpenApplyResult{Flow: flow}, nil
		}
		updated, err := s.store.MarkBaofuAccountOpeningFlowFailed(ctx, db.MarkBaofuAccountOpeningFlowFailedParams{
			ID:             flow.ID,
			FailureCode:    pgText(normalized.FailCode),
			FailureMessage: pgText(normalized.FailMessage),
			RawSnapshot:    baofuAccountRawSnapshot(normalized.Raw),
		})
		if err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		if bindingErr == nil {
			return BaofuAccountOpenApplyResult{Flow: updated, Binding: &binding}, nil
		}
		return BaofuAccountOpenApplyResult{Flow: updated}, nil
	default:
		return BaofuAccountOpenApplyResult{Flow: flow}, nil
	}
}

func (s *BaofuAccountOnboardingService) ensureAccountOpenResultSerialMatchesFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult) error {
	resultOutRequestNo := strings.TrimSpace(result.OutRequestNo)
	if resultOutRequestNo == "" {
		return nil
	}
	flowOpenTransSerialNo := strings.TrimSpace(flow.OpenTransSerialNo.String)
	if flowOpenTransSerialNo == "" || resultOutRequestNo == flowOpenTransSerialNo {
		return nil
	}
	s.persistAccountOpenResultMismatchAlert(ctx, flow, result, "out_request_no_mismatch")
	return fmt.Errorf("baofu account open result out request no mismatch for flow %d", flow.ID)
}

func (s *BaofuAccountOnboardingService) ensureAccountOpenContractMatchesFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult) error {
	contractNo := strings.TrimSpace(result.ContractNo)
	if contractNo == "" {
		return nil
	}
	binding, err := s.store.GetBaofuAccountBindingByContractNo(ctx, pgText(contractNo))
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(binding.OwnerType) == strings.TrimSpace(flow.OwnerType) &&
		binding.OwnerID == flow.OwnerID &&
		(strings.TrimSpace(binding.AccountType) == "" || strings.TrimSpace(binding.AccountType) == strings.TrimSpace(flow.AccountType)) {
		return nil
	}
	s.persistAccountOpenContractMismatchAlert(ctx, flow, binding, result, "contract_owner_mismatch")
	return fmt.Errorf("baofu account open contract owner mismatch for flow %d", flow.ID)
}

func (s *BaofuAccountOnboardingService) persistAccountOpenContractMismatchAlert(ctx context.Context, flow db.BaofuAccountOpeningFlow, binding db.BaofuAccountBinding, result baofucontracts.AccountResult, reason string) {
	extra := map[string]any{
		"reason":                 strings.TrimSpace(reason),
		"flow_id":                flow.ID,
		"flow_owner_type":        strings.TrimSpace(flow.OwnerType),
		"flow_owner_id":          flow.OwnerID,
		"flow_account_type":      strings.TrimSpace(flow.AccountType),
		"flow_state":             strings.TrimSpace(flow.State),
		"open_trans_serial_no":   strings.TrimSpace(flow.OpenTransSerialNo.String),
		"contract_no":            strings.TrimSpace(result.ContractNo),
		"upstream_state":         strings.TrimSpace(result.UpstreamState),
		"open_state":             strings.TrimSpace(result.OpenState),
		"binding_id":             binding.ID,
		"binding_owner_type":     strings.TrimSpace(binding.OwnerType),
		"binding_owner_id":       binding.OwnerID,
		"binding_account_type":   strings.TrimSpace(binding.AccountType),
		"binding_open_state":     strings.TrimSpace(binding.OpenState),
		"provider_operation":     "baofu_account_open_result_apply",
		"requires_manual_review": true,
	}
	extraJSON, err := json.Marshal(extra)
	if err != nil {
		log.Error().Err(err).
			Int64("flow_id", flow.ID).
			Str("owner_type", strings.TrimSpace(flow.OwnerType)).
			Int64("owner_id", flow.OwnerID).
			Str("open_trans_serial_no", strings.TrimSpace(flow.OpenTransSerialNo.String)).
			Msg("marshal baofu account contract mismatch alert extra failed")
		return
	}
	_, err = s.store.CreatePlatformAlertEvent(ctx, db.CreatePlatformAlertEventParams{
		AlertType:   db.PlatformAlertTypeSystemError,
		Level:       db.PlatformAlertLevelCritical,
		Title:       "宝付开户结果合约号归属冲突",
		Message:     fmt.Sprintf("宝付开户流程 %d 返回的合约号已绑定到其他主体，系统已阻断本地账户绑定更新，请人工核对。", flow.ID),
		RelatedID:   flow.ID,
		RelatedType: "baofu_account_opening_flow",
		Extra:       extraJSON,
		EmittedAt:   time.Now(),
	})
	if err != nil {
		log.Error().Err(err).
			Int64("flow_id", flow.ID).
			Str("owner_type", strings.TrimSpace(flow.OwnerType)).
			Int64("owner_id", flow.OwnerID).
			Str("open_trans_serial_no", strings.TrimSpace(flow.OpenTransSerialNo.String)).
			Str("contract_no", strings.TrimSpace(result.ContractNo)).
			Str("provider_operation", "baofu_account_open_result_apply").
			Msg("persist baofu account contract mismatch alert failed")
	}
}

func (s *BaofuAccountOnboardingService) persistAccountOpenResultMismatchAlert(ctx context.Context, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult, reason string) {
	extra := map[string]any{
		"reason":                    strings.TrimSpace(reason),
		"flow_id":                   flow.ID,
		"flow_owner_type":           strings.TrimSpace(flow.OwnerType),
		"flow_owner_id":             flow.OwnerID,
		"flow_account_type":         strings.TrimSpace(flow.AccountType),
		"flow_state":                strings.TrimSpace(flow.State),
		"flow_open_trans_serial_no": strings.TrimSpace(flow.OpenTransSerialNo.String),
		"result_out_request_no":     strings.TrimSpace(result.OutRequestNo),
		"contract_no":               strings.TrimSpace(result.ContractNo),
		"upstream_state":            strings.TrimSpace(result.UpstreamState),
		"open_state":                strings.TrimSpace(result.OpenState),
		"provider_operation":        "baofu_account_open_result_apply",
		"requires_manual_review":    true,
	}
	extraJSON, err := json.Marshal(extra)
	if err != nil {
		log.Error().Err(err).
			Int64("flow_id", flow.ID).
			Str("owner_type", strings.TrimSpace(flow.OwnerType)).
			Int64("owner_id", flow.OwnerID).
			Str("open_trans_serial_no", strings.TrimSpace(flow.OpenTransSerialNo.String)).
			Str("provider_operation", "baofu_account_open_result_apply").
			Msg("marshal baofu account result mismatch alert extra failed")
		return
	}
	_, err = s.store.CreatePlatformAlertEvent(ctx, db.CreatePlatformAlertEventParams{
		AlertType:   db.PlatformAlertTypeSystemError,
		Level:       db.PlatformAlertLevelCritical,
		Title:       "宝付开户结果流水号不一致",
		Message:     fmt.Sprintf("宝付开户流程 %d 返回的流水号与本地流程不一致，系统已阻断本地账户绑定更新，请人工核对。", flow.ID),
		RelatedID:   flow.ID,
		RelatedType: "baofu_account_opening_flow",
		Extra:       extraJSON,
		EmittedAt:   time.Now(),
	})
	if err != nil {
		log.Error().Err(err).
			Int64("flow_id", flow.ID).
			Str("owner_type", strings.TrimSpace(flow.OwnerType)).
			Int64("owner_id", flow.OwnerID).
			Str("open_trans_serial_no", strings.TrimSpace(flow.OpenTransSerialNo.String)).
			Str("result_out_request_no", strings.TrimSpace(result.OutRequestNo)).
			Str("contract_no", strings.TrimSpace(result.ContractNo)).
			Str("provider_operation", "baofu_account_open_result_apply").
			Msg("persist baofu account result mismatch alert failed")
	}
}

func (s *BaofuAccountOnboardingService) applyAccountOpenActive(ctx context.Context, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult) (db.BaofuAccountBinding, error) {
	contractNo := strings.TrimSpace(result.ContractNo)
	if contractNo == "" {
		return db.BaofuAccountBinding{}, errors.New("baofu account active result requires contractNo")
	}
	binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID})
	if errors.Is(err, db.ErrRecordNotFound) {
		binding, err = s.store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
			OwnerType:             flow.OwnerType,
			OwnerID:               flow.OwnerID,
			AccountType:           flow.AccountType,
			LoginNo:               flow.LoginNo,
			OpenState:             db.BaofuAccountOpenStateProcessing,
			LastOpenTransSerialNo: flow.OpenTransSerialNo,
			RawSnapshot:           baofuAccountRawSnapshot(result.Raw),
		})
	}
	if err != nil {
		return db.BaofuAccountBinding{}, err
	}
	if strings.TrimSpace(binding.AccountType) != "" && strings.TrimSpace(binding.AccountType) != strings.TrimSpace(flow.AccountType) {
		return db.BaofuAccountBinding{}, fmt.Errorf("baofu account binding account type mismatch for flow %d", flow.ID)
	}
	if strings.TrimSpace(binding.OpenState) == db.BaofuAccountOpenStateActive {
		return binding, nil
	}
	return s.markBindingActive(ctx, binding, flow, result, s.config.normalized())
}

func (s *BaofuAccountOnboardingService) markBindingActive(ctx context.Context, binding db.BaofuAccountBinding, flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult, cfg BaofuAccountOnboardingConfig) (db.BaofuAccountBinding, error) {
	contractNo := strings.TrimSpace(result.ContractNo)
	sharingMerID := strings.TrimSpace(result.SharingMerID)
	if sharingMerID == "" {
		sharingMerID = contractNo
	}
	activeParams := db.MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgText(contractNo),
		SharingMerID: pgText(sharingMerID),
		RawSnapshot:  baofuAccountRawSnapshot(result.Raw),
	}
	if flow.OwnerType == db.BaofuAccountOwnerTypeMerchant || flow.OwnerType == db.BaofuAccountOwnerTypePlatform {
		txResult, err := s.store.MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx, db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams{
			ActiveBinding: activeParams,
			AccountOpenFeeLedger: db.CreateBaofuFeeLedgerParams{
				FeeType:            db.BaofuFeeTypeAccountOpenVerifyFee,
				PayerType:          db.BaofuFeePayerTypePlatform,
				PayerID:            pgtype.Int8{Valid: false},
				BusinessObjectType: "baofu_account_binding",
				BusinessObjectID:   binding.ID,
				Amount:             cfg.VerifyFeeFen,
				Status:             "recorded",
			},
		})
		if err != nil {
			return db.BaofuAccountBinding{}, err
		}
		return txResult.Binding, nil
	}
	return s.store.MarkBaofuAccountBindingActive(ctx, activeParams)
}
