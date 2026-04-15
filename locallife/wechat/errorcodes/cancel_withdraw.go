package errorcodes

// 官方文档：商户注销组
// 注销预校验：https://pay.weixin.qq.com/doc/v3/partner/4016420099.md
// 注销提现-提交：https://pay.weixin.qq.com/doc/v3/partner/4013892756.md
// 注销提现-查询（按商户申请单号）：https://pay.weixin.qq.com/doc/v3/partner/4013892759.md
// 注销提现-查询（按微信申请单号）：https://pay.weixin.qq.com/doc/v3/partner/4013892765.md

import "strings"

// CancelWithdrawCodeSet 注销提现错误码集合
type CancelWithdrawCodeSet map[string]struct{}

func newCancelWithdrawCodeSet(codes ...string) CancelWithdrawCodeSet {
	set := make(CancelWithdrawCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalCancelWithdrawCode(code)] = struct{}{}
	}
	return set
}

func (s CancelWithdrawCodeSet) Has(code string) bool {
	_, ok := s[CanonicalCancelWithdrawCode(code)]
	return ok
}

// CanonicalCancelWithdrawCode 规范化错误码（大写+trim）
func CanonicalCancelWithdrawCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if canonical, ok := cancelWithdrawCodeAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

// 公共错误码（三个接口共用）
const (
	CancelWithdrawCodeParamError           = "PARAM_ERROR"
	CancelWithdrawCodeInvalidRequest       = "INVALID_REQUEST"
	CancelWithdrawCodeSignError            = "SIGN_ERROR"
	CancelWithdrawCodeSystemError          = "SYSTEM_ERROR"
	CancelWithdrawCodeNoAuth               = "NO_AUTH"
	CancelWithdrawCodeRateLimitExceeded    = "RATELIMIT_EXCEEDED"
	CancelWithdrawCodeFrequencyLimited     = "FREQUENCY_LIMITED"
	CancelWithdrawCodeFrequencyLimit       = "FREQENCY_LIMIT"
	CancelWithdrawCompatCodeFrequencyLimit = "FREQUENCY_LIMIT"
)

// Create 接口业务错误码
const (
	CancelWithdrawCodeBizErrNeedRetry = "BIZ_ERR_NEED_RETRY"
	CancelWithdrawCodeAlreadyExists   = "ALREADY_EXISTS"
)

var cancelWithdrawCodeAliases = map[string]string{
	CancelWithdrawCompatCodeFrequencyLimit: CancelWithdrawCodeFrequencyLimit,
}

// CancelWithdrawCommonCodes 三个接口均有的公共错误码集合
var CancelWithdrawCommonCodes = newCancelWithdrawCodeSet(
	CancelWithdrawCodeParamError,
	CancelWithdrawCodeInvalidRequest,
	CancelWithdrawCodeSignError,
	CancelWithdrawCodeSystemError,
	CancelWithdrawCodeNoAuth,
)

// EcommerceCancelWithdrawValidateDocumentedCodes
// GET /v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/{sub_mchid}
var EcommerceCancelWithdrawValidateDocumentedCodes = newCancelWithdrawCodeSet(
	CancelWithdrawCodeParamError,
	CancelWithdrawCodeInvalidRequest,
	CancelWithdrawCodeSignError,
	CancelWithdrawCodeSystemError,
	CancelWithdrawCodeNoAuth,
	CancelWithdrawCodeRateLimitExceeded,
	CancelWithdrawCodeFrequencyLimited,
	CancelWithdrawCodeFrequencyLimit,
)

// EcommerceCancelWithdrawCreateDocumentedCodes
// POST /v3/ecommerce/account/apply-cancel-withdraw
var EcommerceCancelWithdrawCreateDocumentedCodes = newCancelWithdrawCodeSet(
	CancelWithdrawCodeParamError,
	CancelWithdrawCodeInvalidRequest,
	CancelWithdrawCodeSignError,
	CancelWithdrawCodeSystemError,
	CancelWithdrawCodeNoAuth,
	CancelWithdrawCodeRateLimitExceeded,
	CancelWithdrawCodeFrequencyLimited,
	CancelWithdrawCodeFrequencyLimit,
	CancelWithdrawCodeBizErrNeedRetry,
	CancelWithdrawCodeAlreadyExists,
)

// EcommerceCancelWithdrawQueryDocumentedCodes
// GET /v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{} 及 applyment-id/{}
var EcommerceCancelWithdrawQueryDocumentedCodes = newCancelWithdrawCodeSet(
	CancelWithdrawCodeParamError,
	CancelWithdrawCodeInvalidRequest,
	CancelWithdrawCodeSignError,
	CancelWithdrawCodeSystemError,
	CancelWithdrawCodeNoAuth,
	CancelWithdrawCodeRateLimitExceeded,
	CancelWithdrawCodeFrequencyLimited,
	CancelWithdrawCodeFrequencyLimit,
)
