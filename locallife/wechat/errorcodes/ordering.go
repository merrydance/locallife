package errorcodes

import "strings"

type CodeSet map[string]struct{}

func newCodeSet(codes ...string) CodeSet {
	set := make(CodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalOrderingCode(code)] = struct{}{}
	}
	return set
}

func (s CodeSet) Has(code string) bool {
	_, ok := s[CanonicalOrderingCode(code)]
	return ok
}

const (
	OrderingCodeParamError           = "PARAM_ERROR"
	OrderingCodeInvalidRequest       = "INVALID_REQUEST"
	OrderingCodeAppIDMchIDNotMatch   = "APPID_MCHID_NOT_MATCH"
	OrderingCodeOpenIDMismatch       = "OPENID_MISMATCH"
	OrderingCodeInvalidTransactionID = "INVALID_TRANSACTIONID"
	OrderingCodeMchNotExists         = "MCH_NOT_EXISTS"
	OrderingCodeNoAuth               = "NO_AUTH"
	OrderingCodeSignError            = "SIGN_ERROR"
	OrderingCodeOrderClosed          = "ORDER_CLOSED"
	OrderingCodeOutTradeNoUsed       = "OUT_TRADE_NO_USED"
	OrderingCodeNotEnough            = "NOTENOUGH"
	OrderingCodeAccountError         = "ACCOUNT_ERROR"
	OrderingCodeTradeError           = "TRADE_ERROR"
	OrderingCodeRuleLimit            = "RULE_LIMIT"
	OrderingCodeFrequencyLimited     = "FREQUENCY_LIMITED"
	OrderingCodeSystemError          = "SYSTEM_ERROR"
	OrderingCodeBankError            = "BANK_ERROR"
	OrderingCodeOrderNotExist        = "ORDER_NOT_EXIST"
	OrderingCodeUserPaying           = "USERPAYING"
	OrderingCompatCodeNoAuth         = "NOAUTH"
	OrderingCompatCodeOrderNotExist  = "ORDERNOTEXIST"
	OrderingCompatCodeRuleLimit      = "RULELIMIT"
	OrderingCompatCodeAccountError   = "ACCOUNTERROR"
	OrderingCompatCodeSystemError    = "SYSTEMERROR"
	OrderingCompatCodeBankError      = "BANKERROR"
	OrderingCompatCodeRateLimit      = "RATELIMIT_EXCEEDED"
)

var orderingCodeAliases = map[string]string{
	OrderingCompatCodeNoAuth:        OrderingCodeNoAuth,
	OrderingCompatCodeOrderNotExist: OrderingCodeOrderNotExist,
	OrderingCompatCodeRuleLimit:     OrderingCodeRuleLimit,
	OrderingCompatCodeAccountError:  OrderingCodeAccountError,
	OrderingCompatCodeSystemError:   OrderingCodeSystemError,
	OrderingCompatCodeBankError:     OrderingCodeBankError,
}

// OrderingConfigurationCodes centralizes request-shape, merchant-binding,
// permission, and signature failures used across the ordering capability group.
var OrderingConfigurationCodes = newCodeSet(
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeAppIDMchIDNotMatch,
	OrderingCodeOpenIDMismatch,
	OrderingCodeInvalidTransactionID,
	OrderingCodeMchNotExists,
	OrderingCodeNoAuth,
	OrderingCodeSignError,
)

// OrderingInfrastructureCodes centralizes upstream rate-limit and system
// degradation codes used across ordering create/query/close flows.
var OrderingInfrastructureCodes = newCodeSet(
	OrderingCodeRuleLimit,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
	OrderingCodeBankError,
	OrderingCompatCodeRateLimit,
)

// PartnerSingleCreateDocumentedCodes follows the active official audit for
// POST /v3/pay/partner/transactions/jsapi.
var PartnerSingleCreateDocumentedCodes = newCodeSet(
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeAppIDMchIDNotMatch,
	OrderingCodeOpenIDMismatch,
	OrderingCodeInvalidTransactionID,
	OrderingCodeMchNotExists,
	OrderingCodeNoAuth,
	OrderingCodeSignError,
	OrderingCodeOrderClosed,
	OrderingCodeOutTradeNoUsed,
	OrderingCodeAccountError,
	OrderingCodeTradeError,
	OrderingCodeOrderNotExist,
	OrderingCodeRuleLimit,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
	OrderingCodeBankError,
)

// PartnerSingleQueryDocumentedCodes follows the active official audit for
// GET partner single-order query interfaces.
var PartnerSingleQueryDocumentedCodes = newCodeSet(
	OrderingCodeOrderNotExist,
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeMchNotExists,
	OrderingCodeSignError,
	OrderingCodeTradeError,
	OrderingCodeRuleLimit,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
)

// PartnerSingleQueryCompatibilityCodes records extra upstream-compatible query
// codes LocalLife still maps safely without treating them as the official
// documented query-code set for partner single-order query interfaces.
var PartnerSingleQueryCompatibilityCodes = newCodeSet(
	OrderingCodeBankError,
)

// PartnerSingleCloseDocumentedCodes follows the active official audit for
// POST /v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close.
var PartnerSingleCloseDocumentedCodes = newCodeSet(
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeMchNotExists,
	OrderingCodeSignError,
	OrderingCodeRuleLimit,
	OrderingCodeTradeError,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
)

// PartnerSingleCloseCompatibilityCodes records additional upstream-compatible
// close codes that LocalLife still maps explicitly without treating them as the
// official documented close-code set for this endpoint.
var PartnerSingleCloseCompatibilityCodes = newCodeSet(
	OrderingCodeOrderClosed,
	OrderingCodeUserPaying,
	OrderingCodeOrderNotExist,
	OrderingCodeBankError,
)

// CombineCreateDocumentedCodes follows the active official audit for
// POST /v3/combine-transactions/jsapi.
var CombineCreateDocumentedCodes = newCodeSet(
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeAppIDMchIDNotMatch,
	OrderingCodeOpenIDMismatch,
	OrderingCodeNoAuth,
	OrderingCodeSignError,
	OrderingCodeOrderClosed,
	OrderingCodeOutTradeNoUsed,
	OrderingCodeAccountError,
	OrderingCodeTradeError,
	OrderingCodeRuleLimit,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
)

// CombineQueryDocumentedCodes follows the active official audit for
// GET /v3/combine-transactions/out-trade-no/{combine_out_trade_no}.
var CombineQueryDocumentedCodes = newCodeSet(
	OrderingCodeUserPaying,
	OrderingCodeAppIDMchIDNotMatch,
	OrderingCodeOpenIDMismatch,
	OrderingCodeInvalidTransactionID,
	OrderingCodeOrderNotExist,
	OrderingCodeOrderClosed,
	OrderingCodeParamError,
	OrderingCodeInvalidRequest,
	OrderingCodeMchNotExists,
	OrderingCodeNoAuth,
	OrderingCodeSignError,
	OrderingCodeOutTradeNoUsed,
	OrderingCodeAccountError,
	OrderingCodeTradeError,
	OrderingCodeRuleLimit,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
	OrderingCodeBankError,
)

// CombineCloseDocumentedCodes follows the active official audit for
// POST /v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close.
var CombineCloseDocumentedCodes = newCodeSet(
	OrderingCodeUserPaying,
	OrderingCodeAppIDMchIDNotMatch,
	OrderingCodeOpenIDMismatch,
	OrderingCodeInvalidTransactionID,
	OrderingCodeOrderClosed,
	OrderingCodeOrderNotExist,
	OrderingCodeInvalidRequest,
	OrderingCodeParamError,
	OrderingCodeMchNotExists,
	OrderingCodeNoAuth,
	OrderingCodeSignError,
	OrderingCodeNotEnough,
	OrderingCodeOutTradeNoUsed,
	OrderingCodeRuleLimit,
	OrderingCodeTradeError,
	OrderingCodeFrequencyLimited,
	OrderingCodeSystemError,
	OrderingCodeBankError,
)

// CombineCloseCompatibilityCodes records extra close codes handled for caller
// safety without treating them as the official documented close-code set.
var CombineCloseCompatibilityCodes = newCodeSet()

func CanonicalOrderingCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if canonical, ok := orderingCodeAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func OrderingCodeEquals(code, expected string) bool {
	return CanonicalOrderingCode(code) == CanonicalOrderingCode(expected)
}

func OrderingCodeIn(code string, expected ...string) bool {
	actual := CanonicalOrderingCode(code)
	for _, candidate := range expected {
		if actual == CanonicalOrderingCode(candidate) {
			return true
		}
	}
	return false
}
