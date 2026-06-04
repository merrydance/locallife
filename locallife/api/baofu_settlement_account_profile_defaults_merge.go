package api

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) toOpeningProfileInput(accountType string) *logic.BaofuAccountOpeningProfileInput {
	if defaults.isZero() {
		return nil
	}
	input := logic.BaofuAccountOpeningProfileInput{}
	if strings.TrimSpace(accountType) == db.BaofuAccountTypePersonal {
		defaults.mergeIntoPersonalOpeningProfileInput(&input)
	} else {
		defaults.mergeIntoOpeningProfileInput(&input)
		defaults.applyIndividualBusinessPrivateCardDefault(&input)
		baofuSettlementAccountNormalizeBusinessPrivateCardInput(&input)
	}
	return &input
}

func (defaults *baofuSettlementAccountProfileDefaultsWithSecrets) mergeFrom(source baofuSettlementAccountProfileDefaultsWithSecrets) {
	if defaults == nil || source.isZero() {
		return
	}
	defaults.mergeResponseDefaults(source)
	defaults.mergeSecretDefaults(source)
	defaults.mergeDefaultFlags(source)
}

func (defaults *baofuSettlementAccountProfileDefaultsWithSecrets) overrideMerchantIdentityFrom(source baofuSettlementAccountProfileDefaultsWithSecrets) {
	if defaults == nil || source.isZero() {
		return
	}
	if strings.TrimSpace(source.defaults.Source) != "" {
		defaults.defaults.Source = source.defaults.Source
	}
	if strings.TrimSpace(source.defaults.LegalName) != "" {
		defaults.defaults.LegalName = source.defaults.LegalName
	}
	if strings.TrimSpace(source.defaults.BusinessLicenseNumber) != "" {
		defaults.defaults.BusinessLicenseNumber = source.defaults.BusinessLicenseNumber
	}
	if strings.TrimSpace(source.defaults.LegalPersonName) != "" {
		defaults.defaults.LegalPersonName = source.defaults.LegalPersonName
	}
	if strings.TrimSpace(source.defaults.CardUserName) != "" {
		defaults.defaults.CardUserName = source.defaults.CardUserName
	}
	if len(source.defaults.SettlementAccountAllowedTypes) > 0 {
		defaults.defaults.SettlementAccountAllowedTypes = append([]string(nil), source.defaults.SettlementAccountAllowedTypes...)
	}
	if source.accountTypesAuthoritative {
		defaults.accountTypesAuthoritative = true
		if !baofuSettlementAccountAllowsPrivate(source.defaults.SettlementAccountAllowedTypes) {
			defaults.defaults.SelfEmployed = false
			defaults.defaults.CardUserName = ""
			defaults.selfEmployed = false
			defaults.cardUserName = ""
		}
	}
	if strings.TrimSpace(source.defaults.LegalPersonIDNumber) != "" {
		defaults.defaults.LegalPersonIDNumber = source.defaults.LegalPersonIDNumber
	}
	if strings.TrimSpace(source.defaults.LegalPersonIDNumberMask) != "" {
		defaults.defaults.LegalPersonIDNumberMask = source.defaults.LegalPersonIDNumberMask
	}
	if strings.TrimSpace(source.legalName) != "" {
		defaults.legalName = source.legalName
	}
	if strings.TrimSpace(source.businessLicenseNumber) != "" {
		defaults.businessLicenseNumber = source.businessLicenseNumber
	}
	if strings.TrimSpace(source.legalPersonName) != "" {
		defaults.legalPersonName = source.legalPersonName
	}
	if strings.TrimSpace(source.legalPersonIDNumber) != "" {
		defaults.legalPersonIDNumber = source.legalPersonIDNumber
	}
	if strings.TrimSpace(source.cardUserName) != "" {
		defaults.cardUserName = source.cardUserName
	}
	if source.defaults.HasLegalPersonIDNumber {
		defaults.defaults.HasLegalPersonIDNumber = true
	}
	if source.defaults.HasSavedSensitiveDefaults {
		defaults.defaults.HasSavedSensitiveDefaults = true
	}
}

func (defaults *baofuSettlementAccountProfileDefaultsWithSecrets) mergeResponseDefaults(source baofuSettlementAccountProfileDefaultsWithSecrets) {
	if strings.TrimSpace(defaults.defaults.Source) == "" {
		defaults.defaults.Source = source.defaults.Source
	}
	if strings.TrimSpace(defaults.defaults.LegalName) == "" {
		defaults.defaults.LegalName = source.defaults.LegalName
	}
	if strings.TrimSpace(defaults.defaults.CertificateNo) == "" {
		defaults.defaults.CertificateNo = source.defaults.CertificateNo
	}
	if strings.TrimSpace(defaults.defaults.CertificateNoMask) == "" {
		defaults.defaults.CertificateNoMask = source.defaults.CertificateNoMask
	}
	if strings.TrimSpace(defaults.defaults.BusinessLicenseNumber) == "" {
		defaults.defaults.BusinessLicenseNumber = source.defaults.BusinessLicenseNumber
	}
	if strings.TrimSpace(defaults.defaults.LegalPersonName) == "" {
		defaults.defaults.LegalPersonName = source.defaults.LegalPersonName
	}
	if strings.TrimSpace(defaults.defaults.CardUserName) == "" {
		defaults.defaults.CardUserName = source.defaults.CardUserName
	}
	if source.defaults.SelfEmployed {
		defaults.defaults.SelfEmployed = true
	}
	if strings.TrimSpace(defaults.defaults.LegalPersonIDNumber) == "" {
		defaults.defaults.LegalPersonIDNumber = source.defaults.LegalPersonIDNumber
	}
	if strings.TrimSpace(defaults.defaults.LegalPersonIDNumberMask) == "" {
		defaults.defaults.LegalPersonIDNumberMask = source.defaults.LegalPersonIDNumberMask
	}
	if strings.TrimSpace(defaults.defaults.CorporateMobile) == "" {
		defaults.defaults.CorporateMobile = source.defaults.CorporateMobile
	}
	if strings.TrimSpace(defaults.defaults.CorporateMobileMask) == "" {
		defaults.defaults.CorporateMobileMask = source.defaults.CorporateMobileMask
	}
	if strings.TrimSpace(defaults.defaults.Email) == "" {
		defaults.defaults.Email = source.defaults.Email
	}
	if strings.TrimSpace(defaults.defaults.EmailMask) == "" {
		defaults.defaults.EmailMask = source.defaults.EmailMask
	}
	if strings.TrimSpace(defaults.defaults.BankAccountNo) == "" {
		defaults.defaults.BankAccountNo = source.defaults.BankAccountNo
	}
	if strings.TrimSpace(defaults.defaults.BankAccountNoMask) == "" {
		defaults.defaults.BankAccountNoMask = source.defaults.BankAccountNoMask
	}
	if strings.TrimSpace(defaults.defaults.BankMobile) == "" {
		defaults.defaults.BankMobile = source.defaults.BankMobile
	}
	if strings.TrimSpace(defaults.defaults.BankName) == "" {
		defaults.defaults.BankName = source.defaults.BankName
	}
	if strings.TrimSpace(defaults.defaults.DepositBankProvince) == "" {
		defaults.defaults.DepositBankProvince = source.defaults.DepositBankProvince
	}
	if strings.TrimSpace(defaults.defaults.DepositBankCity) == "" {
		defaults.defaults.DepositBankCity = source.defaults.DepositBankCity
	}
	if strings.TrimSpace(defaults.defaults.DepositBankName) == "" {
		defaults.defaults.DepositBankName = source.defaults.DepositBankName
	}
	if strings.TrimSpace(defaults.defaults.BankAddressCode) == "" {
		defaults.defaults.BankAddressCode = source.defaults.BankAddressCode
	}
	if strings.TrimSpace(defaults.defaults.BankBranchID) == "" {
		defaults.defaults.BankBranchID = source.defaults.BankBranchID
	}
	if strings.TrimSpace(defaults.defaults.AccountBank) == "" {
		defaults.defaults.AccountBank = source.defaults.AccountBank
	}
	if defaults.defaults.AccountBankCode == 0 {
		defaults.defaults.AccountBankCode = source.defaults.AccountBankCode
	}
	if strings.TrimSpace(defaults.defaults.BankAlias) == "" {
		defaults.defaults.BankAlias = source.defaults.BankAlias
	}
	if strings.TrimSpace(defaults.defaults.BankAliasCode) == "" {
		defaults.defaults.BankAliasCode = source.defaults.BankAliasCode
	}
	if strings.TrimSpace(defaults.defaults.ContactName) == "" {
		defaults.defaults.ContactName = source.defaults.ContactName
	}
	if strings.TrimSpace(defaults.defaults.ContactMobileMask) == "" {
		defaults.defaults.ContactMobileMask = source.defaults.ContactMobileMask
	}
	if len(defaults.defaults.SettlementAccountAllowedTypes) == 0 && len(source.defaults.SettlementAccountAllowedTypes) > 0 {
		defaults.defaults.SettlementAccountAllowedTypes = append([]string(nil), source.defaults.SettlementAccountAllowedTypes...)
	}
}

func (defaults *baofuSettlementAccountProfileDefaultsWithSecrets) mergeSecretDefaults(source baofuSettlementAccountProfileDefaultsWithSecrets) {
	if strings.TrimSpace(defaults.legalName) == "" {
		defaults.legalName = source.legalName
	}
	if strings.TrimSpace(defaults.businessLicenseNumber) == "" {
		defaults.businessLicenseNumber = source.businessLicenseNumber
	}
	if strings.TrimSpace(defaults.certificateNo) == "" {
		defaults.certificateNo = source.certificateNo
	}
	if strings.TrimSpace(defaults.legalPersonName) == "" {
		defaults.legalPersonName = source.legalPersonName
	}
	if strings.TrimSpace(defaults.legalPersonIDNumber) == "" {
		defaults.legalPersonIDNumber = source.legalPersonIDNumber
	}
	if strings.TrimSpace(defaults.cardUserName) == "" {
		defaults.cardUserName = source.cardUserName
	}
	if strings.TrimSpace(defaults.corporateMobile) == "" {
		defaults.corporateMobile = source.corporateMobile
	}
	if strings.TrimSpace(defaults.email) == "" {
		defaults.email = source.email
	}
	if strings.TrimSpace(defaults.bankAccountNo) == "" {
		defaults.bankAccountNo = source.bankAccountNo
	}
	if strings.TrimSpace(defaults.bankMobile) == "" {
		defaults.bankMobile = source.bankMobile
	}
	if strings.TrimSpace(defaults.bankName) == "" {
		defaults.bankName = source.bankName
	}
	if strings.TrimSpace(defaults.depositBankProvince) == "" {
		defaults.depositBankProvince = source.depositBankProvince
	}
	if strings.TrimSpace(defaults.depositBankCity) == "" {
		defaults.depositBankCity = source.depositBankCity
	}
	if strings.TrimSpace(defaults.depositBankName) == "" {
		defaults.depositBankName = source.depositBankName
	}
	if strings.TrimSpace(defaults.contactName) == "" {
		defaults.contactName = source.contactName
	}
	if strings.TrimSpace(defaults.contactMobile) == "" {
		defaults.contactMobile = source.contactMobile
	}
	if source.selfEmployed {
		defaults.selfEmployed = true
	}
	if source.accountTypesAuthoritative {
		defaults.accountTypesAuthoritative = true
	}
}

func (defaults *baofuSettlementAccountProfileDefaultsWithSecrets) mergeDefaultFlags(source baofuSettlementAccountProfileDefaultsWithSecrets) {
	if source.defaults.HasLegalPersonIDNumber {
		defaults.defaults.HasLegalPersonIDNumber = true
	}
	if source.defaults.HasCorporateMobile {
		defaults.defaults.HasCorporateMobile = true
	}
	if source.defaults.HasCertificateNo {
		defaults.defaults.HasCertificateNo = true
	}
	if source.defaults.HasEmail {
		defaults.defaults.HasEmail = true
	}
	if source.defaults.HasBankAccountNo {
		defaults.defaults.HasBankAccountNo = true
	}
	if source.defaults.HasContactMobile {
		defaults.defaults.HasContactMobile = true
	}
	if source.defaults.HasSavedSensitiveDefaults {
		defaults.defaults.HasSavedSensitiveDefaults = true
	}
}

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) mergeIntoOpeningProfileInput(input *logic.BaofuAccountOpeningProfileInput) {
	if input == nil {
		return
	}
	if strings.TrimSpace(input.LegalName) == "" {
		input.LegalName = defaults.legalName
	}
	if strings.TrimSpace(input.BusinessLicenseNo) == "" {
		input.BusinessLicenseNo = defaults.businessLicenseNumber
	}
	if strings.TrimSpace(input.CertificateNo) == "" {
		input.CertificateNo = defaults.certificateNo
	}
	if strings.TrimSpace(input.LegalPersonName) == "" {
		input.LegalPersonName = defaults.legalPersonName
	}
	if strings.TrimSpace(input.LegalPersonIDNumber) == "" {
		input.LegalPersonIDNumber = defaults.legalPersonIDNumber
	}
	if strings.TrimSpace(input.CardUserName) == "" {
		input.CardUserName = defaults.cardUserName
	}
	if !input.SelfEmployedSet && defaults.selfEmployed {
		input.SelfEmployed = true
		input.SelfEmployedSet = true
	}
	if defaults.accountTypesAuthoritative && !baofuSettlementAccountAllowsPrivate(defaults.defaults.SettlementAccountAllowedTypes) {
		input.SelfEmployed = false
		input.SelfEmployedSet = false
	}
	if strings.TrimSpace(input.CorporateMobile) == "" {
		input.CorporateMobile = defaults.corporateMobile
	}
	if strings.TrimSpace(input.Email) == "" {
		input.Email = defaults.email
	}
	if strings.TrimSpace(input.BankAccountNo) == "" {
		input.BankAccountNo = defaults.bankAccountNo
	}
	if strings.TrimSpace(input.BankName) == "" {
		input.BankName = defaults.bankName
	}
	if strings.TrimSpace(input.DepositBankProvince) == "" {
		input.DepositBankProvince = defaults.depositBankProvince
	}
	if strings.TrimSpace(input.DepositBankCity) == "" {
		input.DepositBankCity = defaults.depositBankCity
	}
	if strings.TrimSpace(input.DepositBankName) == "" {
		input.DepositBankName = defaults.depositBankName
	}
	if strings.TrimSpace(input.ContactName) == "" {
		input.ContactName = defaults.contactName
	}
	if strings.TrimSpace(input.ContactMobile) == "" {
		input.ContactMobile = defaults.contactMobile
	}
}

func baofuSettlementAccountNormalizeBusinessPrivateCardInput(input *logic.BaofuAccountOpeningProfileInput) {
	if input == nil || input.SelfEmployed {
		return
	}
	input.CardUserName = ""
	input.CorporateMobile = ""
}

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) mergeIntoPersonalOpeningProfileInput(input *logic.BaofuAccountOpeningProfileInput) {
	if input == nil {
		return
	}
	personalName := defaults.personalIdentityName()
	if strings.TrimSpace(input.LegalName) == "" {
		input.LegalName = personalName
	}
	if strings.TrimSpace(input.CertificateNo) == "" {
		input.CertificateNo = defaults.certificateNo
	}
	if strings.TrimSpace(input.CardUserName) == "" {
		input.CardUserName = firstNonBlank(personalName, input.LegalName)
	}
	if strings.TrimSpace(input.BankAccountNo) == "" {
		input.BankAccountNo = defaults.bankAccountNo
	}
	if strings.TrimSpace(input.BankMobile) == "" {
		input.BankMobile = defaults.bankMobile
	}
	if strings.TrimSpace(input.ContactName) == "" {
		input.ContactName = firstNonBlank(defaults.contactName, personalName, input.LegalName)
	}
	if strings.TrimSpace(input.ContactMobile) == "" {
		input.ContactMobile = defaults.contactMobile
	}
}

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) personalIdentityName() string {
	if strings.TrimSpace(defaults.certificateNo) == "" {
		return ""
	}
	return firstNonBlank(defaults.legalName, defaults.cardUserName, defaults.legalPersonName, defaults.defaults.LegalName)
}

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) overrideMerchantIdentityIntoOpeningProfileInput(input *logic.BaofuAccountOpeningProfileInput) {
	if input == nil {
		return
	}
	if strings.TrimSpace(defaults.legalName) != "" {
		input.LegalName = defaults.legalName
	}
	if strings.TrimSpace(defaults.businessLicenseNumber) != "" {
		input.BusinessLicenseNo = defaults.businessLicenseNumber
	}
	if strings.TrimSpace(defaults.legalPersonName) != "" {
		input.LegalPersonName = defaults.legalPersonName
	}
	if strings.TrimSpace(defaults.legalPersonIDNumber) != "" {
		input.LegalPersonIDNumber = defaults.legalPersonIDNumber
	}
	if input.SelfEmployed && strings.TrimSpace(defaults.cardUserName) != "" {
		input.CardUserName = defaults.cardUserName
	}
}

func (defaults baofuSettlementAccountProfileDefaultsWithSecrets) isZero() bool {
	return defaults.defaults.isZero() &&
		firstNonBlank(defaults.legalName, defaults.certificateNo, defaults.businessLicenseNumber, defaults.legalPersonName, defaults.legalPersonIDNumber, defaults.cardUserName, defaults.corporateMobile, defaults.email, defaults.bankAccountNo, defaults.bankMobile, defaults.bankName, defaults.depositBankProvince, defaults.depositBankCity, defaults.depositBankName, defaults.contactName, defaults.contactMobile) == "" &&
		!defaults.selfEmployed
}

func (defaults baofuSettlementAccountProfileDefaults) isZero() bool {
	return firstNonBlank(defaults.LegalName, defaults.CertificateNo, defaults.CertificateNoMask, defaults.BusinessLicenseNumber, defaults.LegalPersonName, defaults.CardUserName, defaults.LegalPersonIDNumber, defaults.LegalPersonIDNumberMask, defaults.CorporateMobile, defaults.CorporateMobileMask, defaults.BankName, defaults.DepositBankProvince, defaults.DepositBankCity, defaults.DepositBankName, defaults.ContactName, defaults.BankAccountNo, defaults.BankAccountNoMask, defaults.BankMobile, defaults.Email, defaults.EmailMask, defaults.ContactMobileMask) == "" &&
		len(defaults.SettlementAccountAllowedTypes) == 0 &&
		!defaults.HasLegalPersonIDNumber &&
		!defaults.HasCorporateMobile &&
		!defaults.HasCertificateNo &&
		!defaults.HasEmail &&
		!defaults.HasBankAccountNo &&
		!defaults.HasContactMobile &&
		!defaults.SelfEmployed
}

func pgInt8Value(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func maskMobileForBaofuResponse(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 7 {
		return maskSensitiveTail(value, 4)
	}
	return value[:3] + "****" + value[len(value)-4:]
}

func maskEmailForBaofuResponse(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return maskSensitiveTail(value, 4)
	}
	return parts[0][:1] + "***@" + parts[1]
}
