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

	WechatServiceTypeJSAPI  = "JSAPI"
	WechatServiceTypeApplet = "APPLET"

	WechatCertificateTypeNationalLegalMerge = "NATIONAL_LEGAL_MERGE"
	WechatCertificateTypeNationalLegal      = "NATIONAL_LEGAL"
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
	case WechatServiceTypeJSAPI, WechatServiceTypeApplet:
		return true
	default:
		return false
	}
}

func IsValidWechatCertificateType(certificateType string) bool {
	switch strings.ToUpper(strings.TrimSpace(certificateType)) {
	case WechatCertificateTypeNationalLegalMerge, WechatCertificateTypeNationalLegal:
		return true
	default:
		return false
	}
}
