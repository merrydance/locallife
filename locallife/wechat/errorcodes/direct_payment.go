package errorcodes

import "strings"

type DirectPaymentCodeSet map[string]struct{}

func newDirectPaymentCodeSet(codes ...string) DirectPaymentCodeSet {
	set := make(DirectPaymentCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalDirectPaymentCode(code)] = struct{}{}
	}
	return set
}

func (s DirectPaymentCodeSet) Has(code string) bool {
	_, ok := s[CanonicalDirectPaymentCode(code)]
	return ok
}

const (
	DirectPaymentCodeParamError          = "PARAM_ERROR"
	DirectPaymentCodeInvalidRequest      = "INVALID_REQUEST"
	DirectPaymentCodeAppIDMchIDNotMatch  = "APPID_MCHID_NOT_MATCH"
	DirectPaymentCodeMchNotExists        = "MCH_NOT_EXISTS"
	DirectPaymentCodeNoAuth              = "NO_AUTH"
	DirectPaymentCodeSignError           = "SIGN_ERROR"
	DirectPaymentCodeOutTradeNoUsed      = "OUT_TRADE_NO_USED"
	DirectPaymentCodeTradeError          = "TRADE_ERROR"
	DirectPaymentCodeRuleLimit           = "RULE_LIMIT"
	DirectPaymentCodeFrequencyLimited    = "FREQUENCY_LIMITED"
	DirectPaymentCodeSystemError         = "SYSTEM_ERROR"
	DirectPaymentCodeOrderNotExist       = "ORDER_NOT_EXIST"
	DirectPaymentCodeNotEnough           = "NOT_ENOUGH"
	DirectPaymentCodeUserAccountAbnormal = "USER_ACCOUNT_ABNORMAL"
	DirectPaymentCodeResourceNotExists   = "RESOURCE_NOT_EXISTS"
)

func CanonicalDirectPaymentCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

var DirectPaymentCommonCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeSignError,
	DirectPaymentCodeSystemError,
)

// POST /v3/pay/transactions/jsapi
var DirectPaymentCreateDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeAppIDMchIDNotMatch,
	DirectPaymentCodeMchNotExists,
	DirectPaymentCodeNoAuth,
	DirectPaymentCodeSignError,
	DirectPaymentCodeOutTradeNoUsed,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)

// GET /v3/pay/transactions/id/{transaction_id}
// GET /v3/pay/transactions/out-trade-no/{out_trade_no}
var DirectPaymentQueryDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeMchNotExists,
	DirectPaymentCodeSignError,
	DirectPaymentCodeRuleLimit,
	DirectPaymentCodeTradeError,
	DirectPaymentCodeOrderNotExist,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)

// POST /v3/pay/transactions/out-trade-no/{out_trade_no}/close
var DirectPaymentCloseDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeMchNotExists,
	DirectPaymentCodeSignError,
	DirectPaymentCodeRuleLimit,
	DirectPaymentCodeTradeError,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)

// POST /v3/refund/domestic/refunds
var DirectRefundCreateDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeSignError,
	DirectPaymentCodeNotEnough,
	DirectPaymentCodeUserAccountAbnormal,
	DirectPaymentCodeMchNotExists,
	DirectPaymentCodeResourceNotExists,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)

// GET /v3/refund/domestic/refunds/{out_refund_no}
var DirectRefundQueryDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeSignError,
	DirectPaymentCodeMchNotExists,
	DirectPaymentCodeResourceNotExists,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)

// POST /v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund
var DirectRefundAbnormalDocumentedCodes = newDirectPaymentCodeSet(
	DirectPaymentCodeParamError,
	DirectPaymentCodeInvalidRequest,
	DirectPaymentCodeSignError,
	DirectPaymentCodeResourceNotExists,
	DirectPaymentCodeFrequencyLimited,
	DirectPaymentCodeSystemError,
)
