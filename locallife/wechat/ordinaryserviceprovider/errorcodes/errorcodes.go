package errorcodes

import (
	"net/http"
	"strings"
)

type Category string

const (
	CategoryRetryableProvider Category = "retryable_provider_failure"
	CategoryBusinessConflict  Category = "business_conflict"
	CategoryMerchantControl   Category = "merchant_control"
	CategoryAuthConfig        Category = "auth_config"
	CategoryValidation        Category = "validation_failure"
	CategoryProvider          Category = "provider_failure"
)

type Metadata struct {
	ProviderCode    string
	StatusCode      int
	Category        Category
	FrontendCode    string
	FrontendMessage string
	OperatorAction  string
	Retryable       bool
}

func Classify(code string, statusCode int) Metadata {
	canonical := Canonical(code)
	category := categoryFor(canonical, statusCode)
	metadata := GuidanceForCategory(category)
	metadata.ProviderCode = canonical
	metadata.StatusCode = statusCode
	return metadata
}

func GuidanceForCategory(category Category) Metadata {
	switch category {
	case CategoryRetryableProvider:
		return Metadata{Category: category, FrontendCode: "WECHAT_PROVIDER_RETRYABLE", FrontendMessage: "微信支付服务暂时不可用，请稍后重试", OperatorAction: "保留原业务单号和请求参数重试；若持续失败请联系平台排查微信支付侧状态", Retryable: true}
	case CategoryAuthConfig:
		return Metadata{Category: category, FrontendCode: "WECHAT_AUTH_CONFIG_REQUIRED", FrontendMessage: "微信支付服务商配置或授权异常", OperatorAction: "请核验服务商商户号、AppID、证书、公钥、接口权限和特约商户绑定关系"}
	case CategoryValidation:
		return Metadata{Category: category, FrontendCode: "WECHAT_REQUEST_INVALID", FrontendMessage: "微信支付请求参数不符合要求", OperatorAction: "请检查商户号、订单号、金额、回调地址和必填资料后重试"}
	case CategoryBusinessConflict:
		return Metadata{Category: category, FrontendCode: "WECHAT_BUSINESS_CONFLICT", FrontendMessage: "微信支付返回业务状态冲突", OperatorAction: "请刷新微信侧订单、退款或分账状态后再处理，避免更换幂等单号"}
	case CategoryMerchantControl:
		return Metadata{Category: category, FrontendCode: "WECHAT_MERCHANT_CONTROLLED", FrontendMessage: "特约商户能力被微信支付限制", OperatorAction: "请在平台财务-普通服务商商户管控诊断中查询受限能力和解脱路径，按微信返回的恢复指引处理"}
	default:
		return Metadata{Category: CategoryProvider, FrontendCode: "WECHAT_PROVIDER_ERROR", FrontendMessage: "微信支付处理失败", OperatorAction: "请查看微信支付服务日志并核对本地业务单号、回调记录和微信返回状态后处理"}
	}
}

func Canonical(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func categoryFor(code string, statusCode int) Category {
	switch code {
	case "SYSTEM_ERROR", "SYSTEMERROR", "FREQUENCY_LIMITED", "FREQENCY_LIMIT", "FREQUENCY_LIMIT_EXCEED", "RATE_LIMITED", "RATELIMIT_EXCEEDED", "BANK_ERROR":
		return CategoryRetryableProvider
	case "NO_AUTH", "NOAUTH", "SIGN_ERROR", "MCH_NOT_EXISTS", "APPID_MCHID_NOT_MATCH", "INVALID_MCHID":
		return CategoryAuthConfig
	case "PARAM_ERROR", "INVALID_REQUEST":
		return CategoryValidation
	case "NOT_ENOUGH", "RESOURCE_ALREADY_EXISTS", "RESOURCE_NOT_EXISTS", "ORDER_NOT_EXIST", "ORDER_CLOSED", "OUT_TRADE_NO_USED", "PROCESSING", "APPLYMENT_NOTEXIST", "APPLYMENT_NOT_EXIST", "ALREADY_EXISTS", "NOT_FOUND", "OPENID_MISMATCH":
		return CategoryBusinessConflict
	case "REQUEST_BLOCKED", "MERCHANT_NOT_ACTIVE", "RISK_CONTROL", "TRADE_ERROR", "RULELIMIT", "RULE_LIMIT", "USER_ACCOUNT_ABNORMAL":
		return CategoryMerchantControl
	default:
		if statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError {
			return CategoryRetryableProvider
		}
		return CategoryProvider
	}
}
