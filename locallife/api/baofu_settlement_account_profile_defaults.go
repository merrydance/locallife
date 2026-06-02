package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/util"
)

type baofuSettlementAccountProfileDefaultsWithSecrets struct {
	defaults                  baofuSettlementAccountProfileDefaults
	legalPersonIDNumber       string
	certificateNo             string
	email                     string
	bankAccountNo             string
	bankMobile                string
	corporateMobile           string
	contactMobile             string
	businessLicenseNumber     string
	legalName                 string
	legalPersonName           string
	cardUserName              string
	bankName                  string
	depositBankProvince       string
	depositBankCity           string
	depositBankName           string
	contactName               string
	selfEmployed              bool
	accountTypesAuthoritative bool
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
	if input.SelfEmployed && defaults.accountTypesAuthoritative && !baofuSettlementAccountAllowsPrivate(defaults.defaults.SettlementAccountAllowedTypes) {
		return nil, logic.NewRequestError(http.StatusBadRequest, errors.New("当前主体仅支持对公结算账户，请选择对公账户后重新提交"))
	}
	merged := *input
	defaults.mergeIntoOpeningProfileInput(&merged)
	if strings.TrimSpace(scope.OwnerType) == db.BaofuAccountOwnerTypeMerchant && strings.TrimSpace(scope.AccountType) != db.BaofuAccountTypePersonal {
		defaults.overrideMerchantIdentityIntoOpeningProfileInput(&merged)
	}
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
		externalDefaults, externalFound, err = server.loadMerchantBaofuSettlementAccountProfileDefaults(ctx, scope)
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
		if strings.TrimSpace(scope.OwnerType) == db.BaofuAccountOwnerTypeMerchant {
			existingDefaults.overrideMerchantIdentityFrom(externalDefaults)
		}
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
			LegalPersonIDNumber:       strings.TrimSpace(legalPersonIDNumber),
			LegalPersonIDNumberMask:   firstNonBlank(pgTextString(profile.CorporateCertIDMask), maskSensitiveTail(legalPersonIDNumber, 4)),
			CorporateMobile:           strings.TrimSpace(corporateMobile),
			CorporateMobileMask:       firstNonBlank(pgTextString(profile.CorporateMobileMask), maskMobileForBaofuResponse(corporateMobile)),
			CertificateNoMask:         firstNonBlank(pgTextString(profile.CertificateNoMask), maskSensitiveTail(personalCertificateNoFromBaofuProfile(accountType, certificateNo), 4)),
			Email:                     strings.TrimSpace(email),
			EmailMask:                 firstNonBlank(pgTextString(profile.EmailMask), maskEmailForBaofuResponse(email)),
			BankAccountNo:             strings.TrimSpace(bankAccountNo),
			BankAccountNoMask:         firstNonBlank(pgTextString(profile.BankAccountNoMask), maskSensitiveTail(bankAccountNo, 4)),
			BankMobile:                strings.TrimSpace(bankMobile),
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
		bankMobile:            strings.TrimSpace(bankMobile),
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
		defaults.bankMobile = strings.TrimSpace(scope.DefaultProfile.BankMobile)
		defaults.bankName = strings.TrimSpace(scope.DefaultProfile.BankName)
		defaults.contactName = strings.TrimSpace(scope.DefaultProfile.ContactName)
		defaults.contactMobile = strings.TrimSpace(scope.DefaultProfile.ContactMobile)
		if strings.TrimSpace(defaults.defaults.LegalName) == "" {
			defaults.defaults.LegalName = defaults.legalName
		}
		if strings.TrimSpace(defaults.defaults.CertificateNoMask) == "" {
			defaults.defaults.CertificateNoMask = maskSensitiveTail(defaults.certificateNo, 4)
		}
		if strings.TrimSpace(defaults.defaults.BankAccountNo) == "" {
			defaults.defaults.BankAccountNo = defaults.bankAccountNo
		}
		if strings.TrimSpace(defaults.defaults.BankAccountNoMask) == "" {
			defaults.defaults.BankAccountNoMask = maskSensitiveTail(defaults.bankAccountNo, 4)
		}
		if strings.TrimSpace(defaults.defaults.BankMobile) == "" {
			defaults.defaults.BankMobile = defaults.bankMobile
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
		if strings.TrimSpace(defaults.certificateNo) != "" || strings.TrimSpace(defaults.bankAccountNo) != "" || strings.TrimSpace(defaults.bankMobile) != "" || strings.TrimSpace(defaults.corporateMobile) != "" || strings.TrimSpace(defaults.contactMobile) != "" {
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
			CertificateNo:             certificateNo,
			CertificateNoMask:         maskSensitiveTail(certificateNo, 4),
			HasCertificateNo:          certificateNo != "",
			HasSavedSensitiveDefaults: certificateNo != "",
		},
		legalName:     legalName,
		certificateNo: certificateNo,
	}
}

func (server *Server) loadMerchantBaofuSettlementAccountProfileDefaults(ctx context.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	ownerUserID := scope.OwnerUserID
	var merchant db.Merchant
	var merchantFound bool
	if ownerUserID <= 0 && scope.OwnerID > 0 {
		var err error
		merchant, err = server.store.GetMerchant(ctx, scope.OwnerID)
		if err != nil {
			if isNotFoundError(err) {
				return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
			}
			return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
		}
		merchantFound = true
		ownerUserID = merchant.OwnerUserID
	}
	if ownerUserID <= 0 {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
	}
	app, err := server.store.GetUserMerchantApplication(ctx, ownerUserID)
	if err != nil {
		if isNotFoundError(err) {
			return server.loadMerchantBaofuSettlementAccountProfileDefaultsFromSnapshot(ctx, scope, merchant, merchantFound)
		}
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	defaults := baofuProfileDefaultsFromMerchantApplication(app)
	if defaults.isZero() {
		return server.loadMerchantBaofuSettlementAccountProfileDefaultsFromSnapshot(ctx, scope, merchant, merchantFound)
	}
	return defaults, true, nil
}

func (server *Server) loadMerchantBaofuSettlementAccountProfileDefaultsFromSnapshot(ctx context.Context, scope baofuSettlementAccountScope, merchant db.Merchant, merchantFound bool) (baofuSettlementAccountProfileDefaultsWithSecrets, bool, error) {
	if !merchantFound {
		loaded, err := server.store.GetMerchant(ctx, scope.OwnerID)
		if err != nil {
			if isNotFoundError(err) {
				return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
			}
			return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
		}
		merchant = loaded
	}
	defaults, err := baofuProfileDefaultsFromMerchantApplicationSnapshot(merchant)
	if err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, err
	}
	if defaults.isZero() {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, false, nil
	}
	return defaults, true, nil
}

func baofuProfileDefaultsFromMerchantApplication(app db.MerchantApplication) baofuSettlementAccountProfileDefaultsWithSecrets {
	if strings.TrimSpace(app.Status) != db.MerchantApplicationStatusApproved {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}
	}
	legalName := strings.TrimSpace(app.MerchantName)
	businessLicenseNumber := strings.TrimSpace(app.BusinessLicenseNumber)
	legalPersonName := strings.TrimSpace(app.LegalPersonName)
	legalPersonIDNumber := strings.TrimSpace(app.LegalPersonIDNumber)
	allowedTypes, accountTypesAuthoritative := baofuSettlementAccountAllowedTypesFromMerchantBusinessLicenseOCR(app.BusinessLicenseOcr)
	return baofuSettlementAccountProfileDefaultsWithSecrets{
		defaults: baofuSettlementAccountProfileDefaults{
			Source:                        "merchant_application",
			LegalName:                     legalName,
			BusinessLicenseNumber:         businessLicenseNumber,
			LegalPersonName:               legalPersonName,
			CardUserName:                  legalPersonName,
			LegalPersonIDNumber:           legalPersonIDNumber,
			LegalPersonIDNumberMask:       maskSensitiveTail(legalPersonIDNumber, 4),
			SettlementAccountAllowedTypes: allowedTypes,
			HasLegalPersonIDNumber:        legalPersonIDNumber != "",
			HasSavedSensitiveDefaults:     legalPersonIDNumber != "",
		},
		legalName:                 legalName,
		businessLicenseNumber:     businessLicenseNumber,
		legalPersonName:           legalPersonName,
		legalPersonIDNumber:       legalPersonIDNumber,
		cardUserName:              legalPersonName,
		accountTypesAuthoritative: accountTypesAuthoritative,
	}
}

func baofuSettlementAccountAllowedTypesFromMerchantBusinessLicenseOCR(raw []byte) ([]string, bool) {
	if len(raw) == 0 {
		return []string{baofuSettlementAccountTypeBusiness}, true
	}
	var payload struct {
		TypeOfEnterprise string `json:"type_of_enterprise"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return []string{baofuSettlementAccountTypeBusiness}, true
	}
	subjectType := baofuMerchantBusinessSubjectTypeFromEnterpriseType(payload.TypeOfEnterprise)
	switch subjectType {
	case "individual_business":
		return []string{baofuSettlementAccountTypeBusiness, baofuSettlementAccountTypePrivate}, true
	default:
		return []string{baofuSettlementAccountTypeBusiness}, true
	}
}

func baofuSettlementAccountAllowsPrivate(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == baofuSettlementAccountTypePrivate {
			return true
		}
	}
	return false
}

func baofuMerchantBusinessSubjectTypeFromEnterpriseType(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "unknown"
	}
	if strings.Contains(normalized, "个体工商户") {
		return "individual_business"
	}
	return "company"
}

func baofuProfileDefaultsFromMerchantApplicationSnapshot(merchant db.Merchant) (baofuSettlementAccountProfileDefaultsWithSecrets, error) {
	if len(merchant.ApplicationData) == 0 {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, nil
	}
	var snapshot struct {
		BusinessLicenseNumber string          `json:"business_license_number"`
		LegalPersonName       string          `json:"legal_person_name"`
		LegalPersonIDNumber   string          `json:"legal_person_id_number"`
		BusinessLicenseOCR    json.RawMessage `json:"business_license_ocr"`
	}
	if err := json.Unmarshal(merchant.ApplicationData, &snapshot); err != nil {
		return baofuSettlementAccountProfileDefaultsWithSecrets{}, err
	}
	legalName := strings.TrimSpace(merchant.Name)
	businessLicenseNumber := strings.TrimSpace(snapshot.BusinessLicenseNumber)
	legalPersonName := strings.TrimSpace(snapshot.LegalPersonName)
	legalPersonIDNumber := strings.TrimSpace(snapshot.LegalPersonIDNumber)
	allowedTypes, accountTypesAuthoritative := baofuSettlementAccountAllowedTypesFromMerchantBusinessLicenseOCR(snapshot.BusinessLicenseOCR)
	return baofuSettlementAccountProfileDefaultsWithSecrets{
		defaults: baofuSettlementAccountProfileDefaults{
			Source:                        "merchant_application_snapshot",
			LegalName:                     legalName,
			BusinessLicenseNumber:         businessLicenseNumber,
			LegalPersonName:               legalPersonName,
			CardUserName:                  legalPersonName,
			LegalPersonIDNumber:           legalPersonIDNumber,
			LegalPersonIDNumberMask:       maskSensitiveTail(legalPersonIDNumber, 4),
			SettlementAccountAllowedTypes: allowedTypes,
			HasLegalPersonIDNumber:        legalPersonIDNumber != "",
			HasSavedSensitiveDefaults:     legalPersonIDNumber != "",
		},
		legalName:                 legalName,
		businessLicenseNumber:     businessLicenseNumber,
		legalPersonName:           legalPersonName,
		legalPersonIDNumber:       legalPersonIDNumber,
		cardUserName:              legalPersonName,
		accountTypesAuthoritative: accountTypesAuthoritative,
	}, nil
}
