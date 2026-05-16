package logic

import (
	"context"
	"errors"
	"strings"

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
		return BaofuAccountOpenApplyResult{}, err
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

func baofuOpeningFlowCanQueryRecover(flow db.BaofuAccountOpeningFlow) bool {
	switch strings.TrimSpace(flow.State) {
	case db.BaofuAccountOpeningStateOpeningProcessing:
		return true
	case db.BaofuAccountOpeningStateFailed:
		return baofuAccountDuplicateFailureCode(flow.FailureCode.String)
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
