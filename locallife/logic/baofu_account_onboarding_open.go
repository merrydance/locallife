package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

func (s *BaofuAccountOnboardingService) openFromProfile(ctx context.Context, flow db.BaofuAccountOpeningFlow, profile db.BaofuAccountOpeningProfile, cfg BaofuAccountOnboardingConfig) (db.BaofuAccountOpeningFlow, *db.BaofuAccountBinding, error) {
	if s.accountClient == nil {
		return db.BaofuAccountOpeningFlow{}, nil, ErrBaofuAccountOnboardingNotConfigured
	}
	openTrans := strings.TrimSpace(flow.OpenTransSerialNo.String)
	if openTrans == "" {
		var err error
		openTrans, err = util.GenerateOutTradeNo("BFO")
		if err != nil {
			return db.BaofuAccountOpeningFlow{}, nil, err
		}
	}
	loginNo := strings.TrimSpace(flow.LoginNo.String)
	if loginNo == "" {
		loginNo = baofuOpeningLoginNo(flow.OwnerType, flow.OwnerID)
	}
	req, err := s.buildOpenRequest(profile, openTrans, loginNo, cfg)
	if err != nil {
		return db.BaofuAccountOpeningFlow{}, nil, err
	}
	requestSnapshot := baofuOpeningRequestSnapshot(req)
	flow, err = s.store.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, db.MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      flow.ID,
		ProfileID:               pgtype.Int8{Int64: profile.ID, Valid: profile.ID > 0},
		VerifyFeePaymentOrderID: flow.VerifyFeePaymentOrderID,
		OpenTransSerialNo:       pgtype.Text{String: openTrans, Valid: true},
		LoginNo:                 pgtype.Text{String: loginNo, Valid: true},
		ProviderRequestSnapshot: requestSnapshot,
		RawSnapshot:             baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateOpeningProcessing}),
	})
	if err != nil {
		return db.BaofuAccountOpeningFlow{}, nil, err
	}
	binding, err := s.store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             flow.OwnerType,
		OwnerID:               flow.OwnerID,
		AccountType:           flow.AccountType,
		LoginNo:               pgtype.Text{String: loginNo, Valid: true},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		LastOpenTransSerialNo: pgtype.Text{String: openTrans, Valid: true},
		RawSnapshot:           baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateOpeningProcessing}),
	})
	if err != nil {
		return db.BaofuAccountOpeningFlow{}, nil, err
	}
	result, err := s.accountClient.OpenAccount(ctx, req)
	if err != nil {
		if updated, markErr := s.markFlowFailedFromProviderError(ctx, flow, err); markErr == nil {
			flow = updated
		} else {
			return db.BaofuAccountOpeningFlow{}, nil, markErr
		}
		return flow, nil, mapBaofuAccountOpenError(err)
	}
	if result == nil {
		return db.BaofuAccountOpeningFlow{}, nil, errors.New("baofu account open returned empty result")
	}
	normalized := result.Normalized()
	switch normalized.OpenState {
	case db.BaofuAccountOpenStateActive, db.BaofuAccountOpenStateFailed, db.BaofuAccountOpenStateAbnormal:
		applied, err := s.ApplyAccountOpenResult(ctx, flow, normalized)
		if err != nil {
			return db.BaofuAccountOpeningFlow{}, nil, err
		}
		if applied.Binding != nil {
			return applied.Flow, applied.Binding, nil
		}
		return applied.Flow, &binding, nil
	default:
		return flow, &binding, nil
	}
}

func (s *BaofuAccountOnboardingService) buildOpenRequest(profile db.BaofuAccountOpeningProfile, openTrans, loginNo string, cfg BaofuAccountOnboardingConfig) (baofucontracts.OpenAccountRequest, error) {
	certNo, err := decryptOptional(s.encryptor, profile.CertificateNoCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	corpCertID, err := decryptOptional(s.encryptor, profile.CorporateCertIDCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	bankAccountNo, err := decryptOptional(s.encryptor, profile.BankAccountNoCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	bankMobile, err := decryptOptional(s.encryptor, profile.BankMobileCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	corporateMobile, err := decryptOptional(s.encryptor, profile.CorporateMobileCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	email, err := decryptOptional(s.encryptor, profile.EmailCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	contactMobile, err := decryptOptional(s.encryptor, profile.ContactMobileCiphertext.String)
	if err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	req := baofucontracts.OpenAccountRequest{
		OwnerType:           profile.OwnerType,
		OwnerID:             profile.OwnerID,
		AccountType:         profile.AccountType,
		OutRequestNo:        openTrans,
		LoginNo:             loginNo,
		LegalName:           profile.LegalName.String,
		CertificateNo:       certNo,
		CertificateType:     profile.CertificateType.String,
		BankAccountNo:       bankAccountNo,
		BankMobile:          bankMobile,
		Email:               email,
		CustomerName:        firstTrimmed(profile.CustomerName.String, profile.LegalName.String),
		CorporateName:       profile.CorporateName.String,
		CorporateCertType:   profile.CorporateCertType.String,
		CorporateCertID:     corpCertID,
		CorporateMobile:     corporateMobile,
		IndustryID:          firstTrimmed(profile.IndustryID.String, cfg.IndustryID),
		ContactName:         profile.ContactName.String,
		ContactMobile:       contactMobile,
		BankName:            profile.BankName.String,
		DepositBankProvince: profile.DepositBankProvince.String,
		DepositBankCity:     profile.DepositBankCity.String,
		DepositBankName:     profile.DepositBankName.String,
		CardUserName:        firstTrimmed(profile.CardUserName.String, profile.LegalName.String),
		SelfEmployed:        baofuProfileUsesPrivateBusinessCard(profile),
	}
	if err := req.Validate(); err != nil {
		return baofucontracts.OpenAccountRequest{}, err
	}
	return req, nil
}
