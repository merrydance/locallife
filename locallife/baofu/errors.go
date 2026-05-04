package baofu

import "strings"

type BaofuErrorCategory string

const (
	BaofuErrorCategoryUserActionRequired    BaofuErrorCategory = "user_action_required"
	BaofuErrorCategoryPlatformConfiguration BaofuErrorCategory = "platform_configuration"
	BaofuErrorCategoryRetryable             BaofuErrorCategory = "retryable"
	BaofuErrorCategoryManualReview          BaofuErrorCategory = "manual_review"
)

type ClassifiedError struct {
	Code          string
	Category      BaofuErrorCategory
	PublicMessage string
	PublicAction  string
	Retryable     bool
}

func ClassifyBaofuError(code string, upstreamMessage string) ClassifiedError {
	_ = upstreamMessage
	canonical := strings.ToUpper(strings.TrimSpace(code))
	classified := ClassifiedError{Code: canonical}
	switch canonical {
	case "PARAM_ERROR", "INVALID_PARAM", "PARAMS_ERROR", "ARGUMENT_INVALID":
		classified.Category = BaofuErrorCategoryUserActionRequired
		classified.PublicMessage = "资料信息不完整，请核对后重新提交"
		classified.PublicAction = "check_and_resubmit"
	case "MERCHANT_NOT_REPORTED", "SUB_MCH_NOT_REPORTED", "MERCHANT_NOT_AUTHORIZED", "NO_AUTH", "MCH_NOT_EXISTS":
		classified.Category = BaofuErrorCategoryPlatformConfiguration
		classified.PublicMessage = "微信支付通道待开通，请联系平台处理"
		classified.PublicAction = "contact_platform"
	case "SYSTEM_BUSY", "SYSTEM_ERROR", "TIMEOUT", "PROCESSING", "ABNORMAL":
		classified.Category = BaofuErrorCategoryRetryable
		classified.PublicMessage = "支付通道处理中，请稍后重试"
		classified.PublicAction = "retry_later"
		classified.Retryable = true
	default:
		classified.Category = BaofuErrorCategoryManualReview
		classified.PublicMessage = "支付通道异常，请联系平台处理"
		classified.PublicAction = "contact_platform"
	}
	return classified
}

func (c ClassifiedError) FrontendGuidance() FrontendGuidance {
	return FrontendGuidance{
		Code:      "BAOFU_" + strings.ToUpper(string(c.Category)),
		Message:   c.PublicMessage,
		Action:    c.PublicAction,
		Retryable: c.Retryable,
	}
}
