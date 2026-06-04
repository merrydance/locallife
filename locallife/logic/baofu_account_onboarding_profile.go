package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/val"
)

func (s *BaofuAccountOnboardingService) resolveProfile(ctx context.Context, ownerType string, ownerID int64, accountType string, input *BaofuAccountOpeningProfileInput, allowExistingAccountType bool) (db.BaofuAccountOpeningProfile, error) {
	if input != nil {
		return s.upsertProfile(ctx, ownerType, ownerID, accountType, *input)
	}
	profile, err := s.store.GetBaofuAccountOpeningProfileByOwner(ctx, db.GetBaofuAccountOpeningProfileByOwnerParams{OwnerType: ownerType, OwnerID: ownerID})
	if err == nil {
		if !allowExistingAccountType && strings.TrimSpace(profile.AccountType) != strings.TrimSpace(accountType) {
			return s.upsertProfile(ctx, ownerType, ownerID, accountType, BaofuAccountOpeningProfileInput{})
		}
		return profile, nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return db.BaofuAccountOpeningProfile{}, err
	}
	return s.upsertProfile(ctx, ownerType, ownerID, accountType, BaofuAccountOpeningProfileInput{})
}

func (s *BaofuAccountOnboardingService) upsertProfile(ctx context.Context, ownerType string, ownerID int64, accountType string, input BaofuAccountOpeningProfileInput) (db.BaofuAccountOpeningProfile, error) {
	input.AccountType = accountType
	completed := baofuProfileComplete(ownerType, input)
	if completed {
		if err := validateBaofuOpeningProfileInput(input); err != nil {
			return db.BaofuAccountOpeningProfile{}, err
		}
	}
	status := db.BaofuAccountOpeningProfileStatusIncomplete
	if completed {
		status = db.BaofuAccountOpeningProfileStatusComplete
	}
	certificateType := ""
	certificateNo := strings.TrimSpace(input.IdentityCertificateNo())
	corporateCertID := strings.TrimSpace(input.LegalPersonIDNumber)
	if accountType == db.BaofuAccountTypePersonal {
		corporateCertID = ""
	}
	if accountType == db.BaofuAccountTypePersonal {
		certificateType = baofucontracts.OfficialCertificateTypeID
	} else {
		certificateType = baofucontracts.OfficialBusinessCertificateTypeLicense
	}
	certificateCipher, err := encryptOptional(s.encryptor, certificateNo)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	corporateCipher, err := encryptOptional(s.encryptor, corporateCertID)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	bankCipher, err := encryptOptional(s.encryptor, input.BankAccountNo)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	bankMobileCipher, err := encryptOptional(s.encryptor, input.BankMobile)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	emailCipher, err := encryptOptional(s.encryptor, input.Email)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	corporateMobileCipher, err := encryptOptional(s.encryptor, input.CorporateMobile)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}
	contactMobileCipher, err := encryptOptional(s.encryptor, input.ContactMobile)
	if err != nil {
		return db.BaofuAccountOpeningProfile{}, err
	}

	sourceSnapshot := baofuOpeningSnapshot(map[string]any{
		"source":        "baofu_settlement_profile_api",
		"status":        status,
		"self_employed": input.SelfEmployed,
	})
	return s.store.UpsertBaofuAccountOpeningProfile(ctx, db.UpsertBaofuAccountOpeningProfileParams{
		OwnerType:                 ownerType,
		OwnerID:                   ownerID,
		AccountType:               accountType,
		ProfileStatus:             status,
		LegalName:                 pgText(input.LegalName),
		CertificateType:           pgText(certificateType),
		CertificateNoCiphertext:   pgText(certificateCipher),
		CertificateNoMask:         pgText(maskSensitiveTail(certificateNo, 4)),
		EmailCiphertext:           pgText(emailCipher),
		EmailMask:                 pgText(maskEmail(input.Email)),
		CustomerName:              pgText(input.LegalName),
		CorporateName:             pgText(input.LegalPersonName),
		CorporateCertType:         pgText(baofuCorporateCertType(accountType)),
		CorporateCertIDCiphertext: pgText(corporateCipher),
		CorporateCertIDMask:       pgText(maskSensitiveTail(corporateCertID, 4)),
		CorporateMobileCiphertext: pgText(corporateMobileCipher),
		CorporateMobileMask:       pgText(maskMobile(input.CorporateMobile)),
		IndustryID:                pgText(s.config.normalized().IndustryID),
		ContactName:               pgText(input.ContactName),
		ContactMobileCiphertext:   pgText(contactMobileCipher),
		ContactMobileMask:         pgText(maskMobile(input.ContactMobile)),
		BankAccountNoCiphertext:   pgText(bankCipher),
		BankAccountNoMask:         pgText(maskBankAccount(input.BankAccountNo)),
		BankMobileCiphertext:      pgText(bankMobileCipher),
		BankMobileMask:            pgText(maskMobile(input.BankMobile)),
		BankName:                  pgText(input.BankName),
		DepositBankProvince:       pgText(input.DepositBankProvince),
		DepositBankCity:           pgText(input.DepositBankCity),
		DepositBankName:           pgText(input.DepositBankName),
		CardUserName:              pgText(input.CardUserName),
		SourceSnapshot:            sourceSnapshot,
	})
}

func (input BaofuAccountOpeningProfileInput) IdentityCertificateNo() string {
	if strings.TrimSpace(input.AccountType) == db.BaofuAccountTypePersonal {
		return strings.TrimSpace(input.CertificateNo)
	}
	if strings.TrimSpace(input.BusinessLicenseNo) != "" {
		return strings.TrimSpace(input.BusinessLicenseNo)
	}
	if strings.TrimSpace(input.CertificateNo) != "" {
		return strings.TrimSpace(input.CertificateNo)
	}
	return strings.TrimSpace(input.LegalPersonIDNumber)
}

func validateBaofuOpeningProfileInput(input BaofuAccountOpeningProfileInput) error {
	if strings.TrimSpace(input.AccountType) == db.BaofuAccountTypePersonal {
		if strings.TrimSpace(input.LegalName) == "" {
			return NewRequestError(http.StatusBadRequest, errors.New("请输入开户人姓名"))
		}
		if err := val.ValidateIDCard(strings.TrimSpace(input.CertificateNo)); err != nil {
			return NewRequestError(http.StatusBadRequest, errors.New("请输入正确身份证号"))
		}
		if strings.TrimSpace(input.BankAccountNo) == "" {
			return NewRequestError(http.StatusBadRequest, errors.New("请输入银行卡号"))
		}
		if err := val.ValidatePhone(strings.TrimSpace(input.BankMobile)); err != nil {
			return NewRequestError(http.StatusBadRequest, errors.New("请输入银行预留手机号"))
		}
	}
	return nil
}

func validateBaofuOpenRequestProfile(req baofucontracts.OpenAccountRequest) error {
	return validateBaofuOpeningProfileInput(BaofuAccountOpeningProfileInput{
		AccountType:   req.AccountType,
		LegalName:     req.LegalName,
		CertificateNo: req.CertificateNo,
		BankAccountNo: req.BankAccountNo,
		BankMobile:    req.BankMobile,
	})
}

func (s *BaofuAccountOnboardingService) getOrCreateFlow(ctx context.Context, ownerType string, ownerID int64, accountType string, profile db.BaofuAccountOpeningProfile) (db.BaofuAccountOpeningFlow, error) {
	flow, err := s.store.GetActiveBaofuAccountOpeningFlowByOwner(ctx, db.GetActiveBaofuAccountOpeningFlowByOwnerParams{OwnerType: ownerType, OwnerID: ownerID})
	if err == nil {
		return s.getOrCreateFlowWithExisting(ctx, ownerType, ownerID, accountType, profile, flow, true)
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return db.BaofuAccountOpeningFlow{}, err
	}
	return s.getOrCreateFlowWithExisting(ctx, ownerType, ownerID, accountType, profile, db.BaofuAccountOpeningFlow{}, false)
}

func (s *BaofuAccountOnboardingService) getOrCreateFlowWithExisting(ctx context.Context, ownerType string, ownerID int64, accountType string, profile db.BaofuAccountOpeningProfile, flow db.BaofuAccountOpeningFlow, found bool) (db.BaofuAccountOpeningFlow, error) {
	if !found {
		return s.createBaofuAccountOpeningFlow(ctx, ownerType, ownerID, accountType, profile)
	}
	if strings.TrimSpace(flow.AccountType) != strings.TrimSpace(accountType) {
		if strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateProfilePending {
			return db.BaofuAccountOpeningFlow{}, baofuAccountOpeningModeConflictError()
		}
		if _, err := s.store.VoidBaofuAccountOpeningFlow(ctx, db.VoidBaofuAccountOpeningFlowParams{
			ID:             flow.ID,
			FailureCode:    pgText("ACCOUNT_OPENING_MODE_CHANGED"),
			FailureMessage: pgText("商户切换宝付开户方式，旧草稿已作废"),
			RawSnapshot:    baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateVoided, "reason": "account_opening_mode_changed", "previous_account_type": flow.AccountType, "next_account_type": accountType}),
		}); err != nil {
			return db.BaofuAccountOpeningFlow{}, err
		}
		return s.createBaofuAccountOpeningFlow(ctx, ownerType, ownerID, accountType, profile)
	}
	return flow, nil
}

func (s *BaofuAccountOnboardingService) createBaofuAccountOpeningFlow(ctx context.Context, ownerType string, ownerID int64, accountType string, profile db.BaofuAccountOpeningProfile) (db.BaofuAccountOpeningFlow, error) {
	return s.store.CreateBaofuAccountOpeningFlow(ctx, db.CreateBaofuAccountOpeningFlowParams{
		OwnerType:               ownerType,
		OwnerID:                 ownerID,
		AccountType:             accountType,
		ProfileID:               pgtype.Int8{Int64: profile.ID, Valid: profile.ID > 0},
		State:                   db.BaofuAccountOpeningStateProfilePending,
		VerifyFeeAmount:         0,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateProfilePending}),
	})
}

func (s *BaofuAccountOnboardingService) markProfilePending(ctx context.Context, flow db.BaofuAccountOpeningFlow, profile db.BaofuAccountOpeningProfile) (db.BaofuAccountOpeningFlow, error) {
	if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateProfilePending {
		return flow, nil
	}
	return s.store.SetBaofuAccountOpeningFlowProfilePending(ctx, db.SetBaofuAccountOpeningFlowProfilePendingParams{
		ID:          flow.ID,
		ProfileID:   pgtype.Int8{Int64: profile.ID, Valid: profile.ID > 0},
		RawSnapshot: baofuOpeningSnapshot(map[string]any{"state": db.BaofuAccountOpeningStateProfilePending, "profile_id": profile.ID}),
	})
}
