package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

func (s *BaofuAccountOnboardingService) RecoverOpeningFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow) (BaofuAccountOpenApplyResult, error) {
	if s == nil || s.store == nil || s.accountClient == nil {
		return BaofuAccountOpenApplyResult{}, ErrBaofuAccountOnboardingNotConfigured
	}
	if !baofuOpeningFlowCanQueryRecover(flow) {
		return BaofuAccountOpenApplyResult{Flow: flow}, nil
	}
	req, err := s.queryRequestForFlow(ctx, flow)
	if err != nil {
		return BaofuAccountOpenApplyResult{}, err
	}
	result, err := s.accountClient.QueryAccount(ctx, req)
	if err != nil {
		if baofuAccountQueryNotFoundIsRecoverable(err) {
			return BaofuAccountOpenApplyResult{}, mapBaofuAccountOpenError(err)
		}
		if updated, markErr := s.markFlowFailedFromProviderError(ctx, flow, err); markErr == nil {
			flow = updated
		} else {
			return BaofuAccountOpenApplyResult{}, markErr
		}
		return BaofuAccountOpenApplyResult{Flow: flow}, mapBaofuAccountOpenError(err)
	}
	if result == nil {
		return BaofuAccountOpenApplyResult{}, errors.New("baofu account query returned empty result")
	}
	normalized := result.Normalized()
	if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateFailed && baofuAccountDuplicateFailureCode(flow.FailureCode.String) {
		normalized = baofuAccountDuplicateReconcileResult(flow, normalized)
	}
	return s.ApplyAccountOpenResult(ctx, flow, normalized)
}

func (s *BaofuAccountOnboardingService) markFlowFailedFromProviderError(ctx context.Context, flow db.BaofuAccountOpeningFlow, err error) (db.BaofuAccountOpeningFlow, error) {
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return flow, nil
	}
	if !baofu.IsProviderBusinessResponseError(providerErr) {
		return flow, nil
	}
	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
	if classified.Category == baofu.BaofuErrorCategoryRetryable {
		return flow, nil
	}
	binding, bindingErr := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: flow.OwnerType,
		OwnerID:   flow.OwnerID,
	})
	rawSnapshot := baofuOpeningProviderFailureSnapshot(classified.Code, providerErr.DiagnosticSnapshot)
	if bindingErr == nil && strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
		if _, err := s.store.MarkBaofuAccountBindingFailed(ctx, db.MarkBaofuAccountBindingFailedParams{
			ID:          binding.ID,
			RawSnapshot: rawSnapshot,
		}); err != nil {
			return db.BaofuAccountOpeningFlow{}, err
		}
	} else if bindingErr != nil && !errors.Is(bindingErr, db.ErrRecordNotFound) {
		return db.BaofuAccountOpeningFlow{}, bindingErr
	}
	return s.store.MarkBaofuAccountOpeningFlowFailed(ctx, db.MarkBaofuAccountOpeningFlowFailedParams{
		ID:             flow.ID,
		FailureCode:    pgtype.Text{String: classified.Code, Valid: classified.Code != ""},
		FailureMessage: baofuAccountSafeFailureMessage(classified.Code, providerErr.UpstreamMessage),
		RawSnapshot:    rawSnapshot,
	})
}

func baofuAccountQueryNotFoundIsRecoverable(err error) bool {
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}
	return strings.TrimSpace(providerErr.Operation) == "T-1001-013-03" &&
		strings.EqualFold(strings.TrimSpace(providerErr.UpstreamCode), "BF00064")
}

func baofuOpeningFlowCanQueryRecover(flow db.BaofuAccountOpeningFlow) bool {
	switch strings.TrimSpace(flow.State) {
	case db.BaofuAccountOpeningStateOpeningProcessing:
		return true
	case db.BaofuAccountOpeningStateFailed:
		return baofuAccountDuplicateFailureCode(flow.FailureCode.String) || baofuAccountQueryNotFoundFailureCode(flow.FailureCode.String)
	default:
		return false
	}
}

func baofuAccountDuplicateFailureCode(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "BF00060", "EXISTED_LOGIN_NO":
		return true
	default:
		return false
	}
}

func baofuAccountQueryNotFoundFailureCode(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), "BF00064")
}

func baofuAccountDuplicateReconcileResult(flow db.BaofuAccountOpeningFlow, result baofucontracts.AccountResult) baofucontracts.AccountResult {
	queryOutRequestNo := strings.TrimSpace(result.OutRequestNo)
	if queryOutRequestNo != "" && queryOutRequestNo != strings.TrimSpace(flow.OpenTransSerialNo.String) {
		result.OutRequestNo = ""
	}
	return result
}

func (s *BaofuAccountOnboardingService) queryRequestForFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow) (baofucontracts.QueryAccountRequest, error) {
	contractNo := ""
	if binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}); err == nil {
		contractNo = strings.TrimSpace(binding.ContractNo.String)
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return baofucontracts.QueryAccountRequest{}, err
	}
	req := baofucontracts.QueryAccountRequest{
		ContractNo:  contractNo,
		AccountType: flow.AccountType,
	}
	if contractNo != "" {
		return req, nil
	}
	profileID := flow.ProfileID
	if !profileID.Valid {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening profile is required for query by loginNo")
	}
	profile, err := s.store.GetBaofuAccountOpeningProfile(ctx, profileID.Int64)
	if err != nil {
		return baofucontracts.QueryAccountRequest{}, err
	}
	certificateNo, err := decryptOptional(s.encryptor, profile.CertificateNoCiphertext.String)
	if err != nil {
		return baofucontracts.QueryAccountRequest{}, err
	}
	req.LoginNo = strings.TrimSpace(flow.LoginNo.String)
	req.CertificateNo = strings.TrimSpace(certificateNo)
	req.CertificateType = strings.TrimSpace(profile.CertificateType.String)
	req.PlatformNo = strings.TrimSpace(s.config.normalized().CollectMerchantID)
	if strings.TrimSpace(req.LoginNo) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening loginNo is required for query")
	}
	if strings.TrimSpace(req.CertificateNo) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening certificateNo is required for query")
	}
	if strings.TrimSpace(req.CertificateType) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening certificateType is required for query")
	}
	if strings.TrimSpace(req.PlatformNo) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening collect merchant id is required for query")
	}
	return req, nil
}
