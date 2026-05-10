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
	if strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateOpeningProcessing {
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
	return s.ApplyAccountOpenResult(ctx, flow, *result)
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
	if strings.TrimSpace(req.LoginNo) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening loginNo is required for query")
	}
	if strings.TrimSpace(req.CertificateNo) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening certificateNo is required for query")
	}
	if strings.TrimSpace(req.CertificateType) == "" {
		return baofucontracts.QueryAccountRequest{}, errors.New("baofu account opening certificateType is required for query")
	}
	return req, nil
}
