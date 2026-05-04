package contracts

import "strings"

const (
	ReportTypeWechat = "WECHAT"
	ReportTypeAlipay = "ALIPAY"

	ReportStateProcessing = "processing"
	ReportStateSucceeded  = "succeeded"
	ReportStateFailed     = "failed"
	ReportStateUnknown    = "unknown"

	AuthTypeAuth   = "AUTH"
	AuthTypeJSAPI  = "JSAPI"
	AuthTypeApplet = "APPLET"

	WechatServiceTypeJSAPI    = "JSAPI"
	WechatServiceTypeApplet   = "APPLET"
	WechatServiceTypeMicropay = "MICROPAY"

	WechatCertificateTypeNationalLegalMerge = "NATIONAL_LEGAL_MERGE"
	WechatCertificateTypeNationalLegal      = "NATIONAL_LEGAL"
	WechatCertificateTypeInstRegistration   = "INST_RGST_CTF"
	WechatCertificateTypeIdentityCard       = "IDENTITY_CARD"
	WechatCertificateTypeOthers             = "OTHERS"

	AlipayServiceTypeFaceToFace = "F2F"

	AlipayCertificateTypeNationalLegalMerge = "NATIONAL_LEGAL_MERGE"
	AlipayCertificateTypeNationalLegal      = "NATIONAL_LEGAL"
	AlipayCertificateTypeInstRegistration   = "INST_RGST_CTF"

	AlipayContactTypeLegalPerson = "LEGAL_PERSON"
	AlipayContactTypeController  = "CONTROLLER"
	AlipayContactTypeAgent       = "AGENT"
	AlipayContactTypeOther       = "OTHER"

	TerminalDeviceTypeStore       = "01"
	TerminalDeviceTypeWebsite     = "02"
	TerminalDeviceTypeMobileSite  = "03"
	TerminalDeviceTypeApp         = "04"
	TerminalDeviceTypeMiniProgram = "05"
	TerminalDeviceTypeOther06     = "06"
	TerminalDeviceTypeOther07     = "07"
	TerminalDeviceTypeOther08     = "08"
	TerminalDeviceTypeOther09     = "09"
	TerminalDeviceTypeOther10     = "10"
	TerminalDeviceTypeOther11     = "11"
	TerminalDeviceTypeOther12     = "12"
	TerminalDeviceTypeOther13     = "13"

	OperationFlagCreate = "00"
	OperationFlagModify = "01"
	OperationFlagCancel = "02"

	DeviceStatusEnabled  = "00"
	DeviceStatusCanceled = "01"

	ContactBusinessTypeMerchantContact = "02"
	ContactBusinessTypeLegalPerson     = "06"
	ContactBusinessTypeController      = "08"
	ContactBusinessTypeBeneficiary     = "11"

	SiteTypePC          = "01"
	SiteTypeMobile      = "02"
	SiteTypeApp         = "03"
	SiteTypeMiniProgram = "04"
	SiteTypeOther05     = "05"
	SiteTypeOther06     = "06"

	IndirectLevelM1 = "INDIRECT_LEVEL_M1"
	IndirectLevelM2 = "INDIRECT_LEVEL_M2"
	IndirectLevelM3 = "INDIRECT_LEVEL_M3"
	IndirectLevelM4 = "INDIRECT_LEVEL_M4"

	MerchantStatusEnabled  = "00"
	MerchantStatusCanceled = "01"

	TransactionControlAllowed = "00"
	TransactionControlBlocked = "01"

	AuthOrderStateAuditing          = "AUDITING"
	AuthOrderStateContactConfirm    = "CONTACT_CONFIRM"
	AuthOrderStateLegalConfirm      = "LEGAL_CONFIRM"
	AuthOrderStateAuditPass         = "AUDIT_PASS"
	AuthOrderStateAuditReject       = "AUDIT_REJECT"
	AuthOrderStateAuditFreeze       = "AUDIT_FREEZE"
	AuthOrderStateCanceled          = "CANCELED"
	AuthOrderStateContactProcessing = "CONTACT_PROCESSING"

	MerchantAuthStateAuthorized   = "AUTHORIZED"
	MerchantAuthStateUnauthorized = "UNAUTHORIZED"
	MerchantAuthStateClosed       = "CLOSED"
	MerchantAuthStateSMIDNotExist = "SMID_NOT_EXIST"
)

func NormalizeMerchantReportState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "SUCCESS", "SUCCEED", "SUCCEEDED", "S", "1":
		return ReportStateSucceeded
	case "FAIL", "FAILED", "FAILURE", "F", "0":
		return ReportStateFailed
	case "PROCESSING", "PROCESS", "PENDING", "WAIT", "WAITING", "2":
		return ReportStateProcessing
	default:
		return ReportStateUnknown
	}
}

func IsValidReportType(reportType string) bool {
	switch strings.ToUpper(strings.TrimSpace(reportType)) {
	case ReportTypeWechat, ReportTypeAlipay:
		return true
	default:
		return false
	}
}

func IsValidAuthType(authType string) bool {
	switch strings.ToUpper(strings.TrimSpace(authType)) {
	case AuthTypeAuth, AuthTypeJSAPI, AuthTypeApplet:
		return true
	default:
		return false
	}
}

func IsValidWechatServiceType(serviceType string) bool {
	switch strings.ToUpper(strings.TrimSpace(serviceType)) {
	case WechatServiceTypeJSAPI, WechatServiceTypeApplet, WechatServiceTypeMicropay:
		return true
	default:
		return false
	}
}

func IsValidWechatCertificateType(certificateType string) bool {
	switch strings.ToUpper(strings.TrimSpace(certificateType)) {
	case WechatCertificateTypeNationalLegalMerge, WechatCertificateTypeNationalLegal, WechatCertificateTypeInstRegistration, WechatCertificateTypeIdentityCard, WechatCertificateTypeOthers:
		return true
	default:
		return false
	}
}

func IsValidAlipayServiceType(serviceType string) bool {
	return strings.ToUpper(strings.TrimSpace(serviceType)) == AlipayServiceTypeFaceToFace
}

func IsValidAlipayCertificateType(certificateType string) bool {
	switch strings.ToUpper(strings.TrimSpace(certificateType)) {
	case AlipayCertificateTypeNationalLegalMerge, AlipayCertificateTypeNationalLegal, AlipayCertificateTypeInstRegistration:
		return true
	default:
		return false
	}
}

func IsValidAlipayContactType(contactType string) bool {
	switch strings.ToUpper(strings.TrimSpace(contactType)) {
	case AlipayContactTypeLegalPerson, AlipayContactTypeController, AlipayContactTypeAgent, AlipayContactTypeOther:
		return true
	default:
		return false
	}
}

func IsValidTerminalDeviceType(value string) bool {
	switch strings.TrimSpace(value) {
	case "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13":
		return true
	default:
		return false
	}
}

func IsValidOperationFlag(value string) bool {
	switch strings.TrimSpace(value) {
	case OperationFlagCreate, OperationFlagModify, OperationFlagCancel:
		return true
	default:
		return false
	}
}

func IsValidDeviceStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case DeviceStatusEnabled, DeviceStatusCanceled:
		return true
	default:
		return false
	}
}

func IsValidContactBusinessType(value string) bool {
	switch strings.TrimSpace(value) {
	case ContactBusinessTypeMerchantContact, ContactBusinessTypeLegalPerson, ContactBusinessTypeController, ContactBusinessTypeBeneficiary:
		return true
	default:
		return false
	}
}

func IsValidSiteType(value string) bool {
	switch strings.TrimSpace(value) {
	case SiteTypePC, SiteTypeMobile, SiteTypeApp, SiteTypeMiniProgram, SiteTypeOther05, SiteTypeOther06:
		return true
	default:
		return false
	}
}

func IsValidIndirectLevel(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case IndirectLevelM1, IndirectLevelM2, IndirectLevelM3, IndirectLevelM4:
		return true
	default:
		return false
	}
}

func IsValidMerchantStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case MerchantStatusEnabled, MerchantStatusCanceled:
		return true
	default:
		return false
	}
}

func IsValidTransactionControl(value string) bool {
	switch strings.TrimSpace(value) {
	case TransactionControlAllowed, TransactionControlBlocked:
		return true
	default:
		return false
	}
}

func IsValidAuthOrderState(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case AuthOrderStateAuditing, AuthOrderStateContactConfirm, AuthOrderStateLegalConfirm, AuthOrderStateAuditPass, AuthOrderStateAuditReject, AuthOrderStateAuditFreeze, AuthOrderStateCanceled, AuthOrderStateContactProcessing:
		return true
	default:
		return false
	}
}

func IsValidMerchantAuthState(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case MerchantAuthStateAuthorized, MerchantAuthStateUnauthorized, MerchantAuthStateClosed, MerchantAuthStateSMIDNotExist:
		return true
	default:
		return false
	}
}
