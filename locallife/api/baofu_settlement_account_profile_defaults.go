package api

import (
	"context"
	"encoding/json"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/util"
)

type baofuSettlementAccountProfileDefaultsWithSecrets struct {
	defaults              baofuSettlementAccountProfileDefaults
	legalPersonIDNumber   string
	certificateNo         string
	email                 string
	bankAccountNo         string
	corporateMobile       string
	contactMobile         string
	businessLicenseNumber string
	legalName             string
	legalPersonName       string
	cardUserName          string
	bankName              string
	depositBankProvince   string
	depositBankCity       string
	depositBankName       string
	contactName           string
	selfEmployed          bool
}

func (server *Server) applyBaofuSettlementAccountProfileDefaults(ctx context.Context, scope baofuSettlementAccountScope, resp *baofuSettlementAccountResponse, profile db.BaofuAccountOpeningProfile, profileFound bool) error {
	if resp == nil || strings.TrimSpace(scope.OwnerType) == "" {
		return nil
	}
	var existingDefaults baofuSettlementAccountProfileDefaultsWithSecrets
	var existingFound bool
	if profileFound {
		defaults, found, err := server.baofuSettlementAccountProfileDefaultsFromProfile(ctx, profile)
		if err != nil {
			return err
		}
		existingDefaults = defaults
		existingFound = found
	}
	defaults, found, err := server.loadBaofuSettlementAccountProfileDefaultsWithExisting(ctx, scope, existingDefaults, existingFound)
	if err != nil {
		return err
	}
	if !found || defaults.defaults.isZero() {
		return nil
	}
	resp.ProfileDefaults = &defaults.defaults
	if resp.BankAccountNoMask == "" {
		resp.BankAccountNoMask = defaults.defaults.BankAccountNoMask
	}
	if resp.ContactMobileMask == "" {
		resp.ContactMobileMask = defaults.defaults.ContactMobileMask
	}
	if resp.EmailMask == "" {
		resp.EmailMask = defaults.defaults.EmailMask
	}
	return nil
}

func (server *Server) baofuSettlementAccountProfileInputWithDefaults(ctx context.Context, scope baofuSettlementAccountScope, input *logic.BaofuAccountOpeningProfileInput) (*logic.BaofuAccountOpeningProfileInput, error) {
	defaults, found, err := server.loadBaofuSettlementAccountProfileDefaults(ctx, scope)
	if err != nil {
		return nil, err
	}
	if input == nil {
		if !found {
			return nil, nil
		}
		return defaults.toOpeningProfileInput(), nil
	}
	if !found {
		return input, nil
	}
	merged := *input
	defaults.mergeIntoOpeningProfileInput(&merged)
	return &merged, nil
}

func (server *Server) loadBaofuSettlementAccountProfileDefaults(ctx context.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	existingDefaults, existingFound, err := server.loadExistingBaofuSettlementAccountProfileDefaults(ctx, scope)
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	return server.loadBaofuSettlementAccountProfileDefaultsWithExisting(ctx, scope, existingDefaults, existingFound)
}

func (server *Server) loadBaofuSettlementAccountProfileDefaultsWithExisting(ctx context.Context, scope baofuSettlementAccountScope, existingDefaults baofuSettlementAccountProfileDefaultsWithSecrets, existingFound bool) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	var externalDefaults baofuSettlementAccountProfileDefaultsWithSecrets
	var externalFound bool
	var err error
	switch strings.TrimSpace(scope.OwnerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		externalDefaults, externalFound, err = server.loadMerchantBaofuSettlementAccountProfileDefaults(ctx, scope.OwnerID)
	case db.BaofuAccountOwnerTypeOperator:
		externalDefaults, externalFound, err = server.loadOperatorBaofuSettlementAccountProfileDefaults(ctx, scope)
	default:
		externalDefaults, externalFound = baofuProfileDefaultsFromScope(scope)
	}
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	if existingFound {
		existingDefaults.mergeFrom(externalDefaults)
		if !existingDefaults.isZero() {
			return existingDefaults, true, nil
		}
	}
	if externalFound && !externalDefaults.isZero() {
		return externalDefaults, true, nil
	}
	return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
}

func (server *Server) loadExistingBaofuSettlementAccountProfileDefaults(ctx context.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	profile, err := server.store.GetBaofuAccountOpeningProfileByOwner(ctx, db.GetBaofuAccountOpeningProfileByOwnerParams{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
		}
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	return server.baofuSettlementAccountProfileDefaultsFromProfile(ctx, profile)
}

func (server *Server) baofuSettlementAccountProfileDefaultsFromProfile(_ context.Context, profile db.BaofuAccountOpeningProfile) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	certificateNo, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.CertificateNoCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	legalPersonIDNumber, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.CorporateCertIDCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	email, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.EmailCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	bankAccountNo, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.BankAccountNoCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	bankMobile, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.BankMobileCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	contactMobile, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.ContactMobileCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	corporateMobile, err := util.DecryptSensitiveField(server.dataEncryptor, pgTextString(profile.CorporateMobileCiphertext))
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}

	accountType := strings.TrimSpace(profile.AccountType)
	selfEmployed := baofuSettlementAccountProfileUsesPrivateBusinessCard(profile)
	defaults := baofuSettlementAccountProfileDefaultsWithSecrets{
		defaults: baofuSettlementAccountProfileDefaults{
			Source:                    "baofu_profile",
			LegalName:                 strings.TrimSpace(profile.LegalName.String),
			BusinessLicenseNumber:     businessLicenseNumberFromBaofuProfile(accountType, certificateNo),
			LegalPersonName:           strings.TrimSpace(profile.CorporateName.String),
			CardUserName:              strings.TrimSpace(profile.CardUserName.String),
			SelfEmployed:              selfEmployed,
			LegalPersonIDNumberMask:   firstNonBlank(pgTextString(profile.CorporateCertIDMask), maskSensitiveTail(legalPersonIDNumber, 4)),
			CorporateMobileMask:       firstNonBlank(pgTextString(profile.CorporateMobileMask), maskMobileForBaofuResponse(corporateMobile)),
			CertificateNoMask:         firstNonBlank(pgTextString(profile.CertificateNoMask), maskSensitiveTail(personalCertificateNoFromBaofuProfile(accountType, certificateNo), 4)),
			EmailMask:                 firstNonBlank(pgTextString(profile.EmailMask), maskEmailForBaofuResponse(email)),
			BankAccountNoMask:         firstNonBlank(pgTextString(profile.BankAccountNoMask), maskSensitiveTail(bankAccountNo, 4)),
			BankName:                  strings.TrimSpace(profile.BankName.String),
			DepositBankProvince:       strings.TrimSpace(profile.DepositBankProvince.String),
			DepositBankCity:           strings.TrimSpace(profile.DepositBankCity.String),
			DepositBankName:           strings.TrimSpace(profile.DepositBankName.String),
			ContactName:               strings.TrimSpace(profile.ContactName.String),
			ContactMobileMask:         firstNonBlank(pgTextString(profile.ContactMobileMask), maskMobileForBaofuResponse(contactMobile)),
			HasLegalPersonIDNumber:    strings.TrimSpace(legalPersonIDNumber) != "",
			HasCorporateMobile:        strings.TrimSpace(corporateMobile) != "",
			HasCertificateNo:          accountType == db.BaofuAccountTypePersonal && strings.TrimSpace(certificateNo) != "",
			HasEmail:                  strings.TrimSpace(email) != "",
			HasBankAccountNo:          strings.TrimSpace(bankAccountNo) != "",
			HasContactMobile:          strings.TrimSpace(contactMobile) != "",
			HasSavedSensitiveDefaults: firstNonBlank(certificateNo, legalPersonIDNumber, corporateMobile, email, bankAccountNo, bankMobile, contactMobile) != "",
		},
		legalName:             strings.TrimSpace(profile.LegalName.String),
		businessLicenseNumber: businessLicenseNumberFromBaofuProfile(accountType, certificateNo),
		certificateNo:         personalCertificateNoFromBaofuProfile(accountType, certificateNo),
		legalPersonName:       strings.TrimSpace(profile.CorporateName.String),
		legalPersonIDNumber:   strings.TrimSpace(legalPersonIDNumber),
		cardUserName:          strings.TrimSpace(profile.CardUserName.String),
		corporateMobile:       strings.TrimSpace(corporateMobile),
		email:                 strings.TrimSpace(email),
		bankAccountNo:         strings.TrimSpace(bankAccountNo),
		bankName:              strings.TrimSpace(profile.BankName.String),
		depositBankProvince:   strings.TrimSpace(profile.DepositBankProvince.String),
		depositBankCity:       strings.TrimSpace(profile.DepositBankCity.String),
		depositBankName:       strings.TrimSpace(profile.DepositBankName.String),
		contactName:           strings.TrimSpace(profile.ContactName.String),
		contactMobile:         strings.TrimSpace(contactMobile),
		selfEmployed:          selfEmployed,
	}
	if defaults.isZero() {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
	}
	return defaults, true, nil
}

func businessLicenseNumberFromBaofuProfile(accountType string, certificateNo string) string {
	if strings.TrimSpace(accountType) == db.BaofuAccountTypeBusiness {
		return strings.TrimSpace(certificateNo)
	}
	return ""
}

func personalCertificateNoFromBaofuProfile(accountType string, certificateNo string) string {
	if strings.TrimSpace(accountType) == db.BaofuAccountTypePersonal {
		return strings.TrimSpace(certificateNo)
	}
	return ""
}

func baofuSettlementAccountProfileUsesPrivateBusinessCard(profile db.BaofuAccountOpeningProfile) bool {
	if strings.TrimSpace(profile.AccountType) != db.BaofuAccountTypeBusiness {
		return false
	}
	if strings.TrimSpace(profile.CardUserName.String) == "" {
		return false
	}
	var payload struct {
		SelfEmployed bool `json:"self_employed"`
	}
	if err := json.Unmarshal(profile.SourceSnapshot, &payload); err != nil {
		return false
	}
	return payload.SelfEmployed
}

func baofuProfileDefaultsFromScope(scope baofuSettlementAccountScope) (baofuSettlementAccountProfileDefaultsWithSecrets, bool) {
	if scope.DefaultProfile == nil && scope.DefaultProfileMasks == nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false
	}
	defaults := baofuSettlementAccountProfileDefaultsWithSecrets{}
	if scope.DefaultProfileMasks != nil {
		defaults.defaults = *scope.DefaultProfileMasks
	}
	if strings.TrimSpace(defaults.defaults.Source) == "" {
		defaults.defaults.Source = strings.TrimSpace(scope.Audience)
	}
	if scope.DefaultProfile != nil {
		defaults.legalName = strings.TrimSpace(scope.DefaultProfile.LegalName)
		defaults.certificateNo = strings.TrimSpace(scope.DefaultProfile.CertificateNo)
		defaults.bankAccountNo = strings.TrimSpace(scope.DefaultProfile.BankAccountNo)
		defaults.bankName = strings.TrimSpace(scope.DefaultProfile.BankName)
		defaults.contactName = strings.TrimSpace(scope.DefaultProfile.ContactName)
		defaults.contactMobile = strings.TrimSpace(scope.DefaultProfile.ContactMobile)
		if strings.TrimSpace(defaults.defaults.LegalName) == "" {
			defaults.defaults.LegalName = defaults.legalName
		}
		if strings.TrimSpace(defaults.defaults.CertificateNoMask) == "" {
			defaults.defaults.CertificateNoMask = maskSensitiveTail(defaults.certificateNo, 4)
		}
		if strings.TrimSpace(defaults.defaults.BankAccountNoMask) == "" {
			defaults.defaults.BankAccountNoMask = maskSensitiveTail(defaults.bankAccountNo, 4)
		}
		if strings.TrimSpace(defaults.defaults.BankName) == "" {
			defaults.defaults.BankName = defaults.bankName
		}
		if strings.TrimSpace(defaults.defaults.ContactName) == "" {
			defaults.defaults.ContactName = defaults.contactName
		}
		if strings.TrimSpace(defaults.defaults.ContactMobileMask) == "" {
			defaults.defaults.ContactMobileMask = maskMobileForBaofuResponse(defaults.contactMobile)
		}
		if strings.TrimSpace(defaults.certificateNo) != "" {
			defaults.defaults.HasCertificateNo = true
		}
		if strings.TrimSpace(defaults.bankAccountNo) != "" {
			defaults.defaults.HasBankAccountNo = true
		}
		if strings.TrimSpace(defaults.corporateMobile) != "" {
			defaults.defaults.HasCorporateMobile = true
		}
		if strings.TrimSpace(defaults.contactMobile) != "" {
			defaults.defaults.HasContactMobile = true
		}
		if strings.TrimSpace(defaults.certificateNo) != "" || strings.TrimSpace(defaults.bankAccountNo) != "" || strings.TrimSpace(defaults.corporateMobile) != "" || strings.TrimSpace(defaults.contactMobile) != "" {
			defaults.defaults.HasSavedSensitiveDefaults = true
		}
	}
	return defaults, !defaults.isZero()
}

func (server *Server) loadOperatorBaofuSettlementAccountProfileDefaults(ctx context.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	scopeDefaults, found := baofuProfileDefaultsFromScope(scope)
	if scope.OwnerUserID <= 0 {
		return scopeDefaults, found, nil
	}
	app, err := server.store.GetOperatorApplicationByUserID(ctx, scope.OwnerUserID)
	if err != nil {
		if isNotFoundError(err) {
			return scopeDefaults, found, nil
		}
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	defaults := baofuProfileDefaultsFromOperatorApplication(app)
	defaults.mergeFrom(scopeDefaults)
	if defaults.isZero() {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
	}
	return defaults, true, nil
}

func baofuProfileDefaultsFromOperatorApplication(app db.OperatorApplication) baofuSettlementAccountProfileDefaultsWithSecrets {
	legalName := pgTextString(app.LegalPersonName)
	certificateNo := pgTextString(app.LegalPersonIDNumber)
	return baofuSettlementAccountProfileDefaultsWithSecrets{
		defaults: baofuSettlementAccountProfileDefaults{
			Source:                    "operator_application",
			LegalName:                 legalName,
			CertificateNoMask:         maskSensitiveTail(certificateNo, 4),
			HasCertificateNo:          certificateNo != "",
			HasSavedSensitiveDefaults: certificateNo != "",
		},
		legalName:     legalName,
		certificateNo: certificateNo,
	}
}

func (server *Server) loadMerchantBaofuSettlementAccountProfileDefaults(ctx context.Context, merchantID int64) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchantID,
	})
	if err == nil {
		return baofuProfileDefaultsFromEcommerceApplyment(applyment), true, nil
	}
	if !isNotFoundError(err) {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
}

func baofuProfileDefaultsFromEcommerceApplyment(applyment db.EcommerceApplyment) baofuSettlementAccountProfileDefaultsWithSecrets {
	businessLicenseNumber := pgTextString(applyment.BusinessLicenseNumber)
	legalPersonIDNumber := strings.TrimSpace(applyment.IDCardNumber)
	email := pgTextString(applyment.ContactEmail)
	bankAccountNo := strings.TrimSpace(applyment.AccountNumber)
	contactMobile := strings.TrimSpace(applyment.MobilePhone)
	legalPersonName := strings.TrimSpace(applyment.LegalPerson)
	selfEmployed := strings.TrimSpace(applyment.AccountType) == "ACCOUNT_TYPE_PRIVATE"
	cardUserName := ""
	corporateMobile := ""
	if selfEmployed {
		cardUserName = firstNonBlank(applyment.AccountName, legalPersonName)
		if strings.TrimSpace(applyment.ContactName) == "" || strings.TrimSpace(applyment.ContactName) == legalPersonName {
			corporateMobile = contactMobile
		}
	}
	bankName := firstNonBlank(pgTextString(applyment.BankAlias), applyment.AccountBank)
	depositBankName := pgTextString(applyment.BankName)
	return baofuSettlementAccountProfileDefaultsWithSecrets{
		defaults: baofuSettlementAccountProfileDefaults{
			Source:                    "wechat_applyment",
			LegalName:                 strings.TrimSpace(applyment.MerchantName),
			BusinessLicenseNumber:     businessLicenseNumber,
			LegalPersonName:           legalPersonName,
			CardUserName:              cardUserName,
			SelfEmployed:              selfEmployed,
			LegalPersonIDNumberMask:   maskSensitiveTail(legalPersonIDNumber, 4),
			CorporateMobileMask:       maskMobileForBaofuResponse(corporateMobile),
			EmailMask:                 maskEmailForBaofuResponse(email),
			BankAccountNoMask:         maskSensitiveTail(bankAccountNo, 4),
			BankName:                  bankName,
			DepositBankName:           depositBankName,
			BankAddressCode:           strings.TrimSpace(applyment.BankAddressCode),
			BankBranchID:              pgTextString(applyment.BankBranchID),
			AccountBank:               strings.TrimSpace(applyment.AccountBank),
			AccountBankCode:           pgInt8Value(applyment.AccountBankCode),
			BankAlias:                 pgTextString(applyment.BankAlias),
			BankAliasCode:             pgTextString(applyment.BankAliasCode),
			ContactName:               strings.TrimSpace(applyment.ContactName),
			ContactMobileMask:         maskMobileForBaofuResponse(contactMobile),
			HasLegalPersonIDNumber:    legalPersonIDNumber != "",
			HasCorporateMobile:        corporateMobile != "",
			HasEmail:                  email != "",
			HasBankAccountNo:          bankAccountNo != "",
			HasContactMobile:          contactMobile != "",
			HasSavedSensitiveDefaults: legalPersonIDNumber != "" || corporateMobile != "" || email != "" || bankAccountNo != "" || contactMobile != "",
		},
		legalName:             strings.TrimSpace(applyment.MerchantName),
		businessLicenseNumber: businessLicenseNumber,
		legalPersonName:       legalPersonName,
		legalPersonIDNumber:   legalPersonIDNumber,
		cardUserName:          cardUserName,
		corporateMobile:       corporateMobile,
		email:                 email,
		bankAccountNo:         bankAccountNo,
		bankName:              bankName,
		depositBankName:       depositBankName,
		contactName:           strings.TrimSpace(applyment.ContactName),
		contactMobile:         contactMobile,
		selfEmployed:          selfEmployed,
	}
}
