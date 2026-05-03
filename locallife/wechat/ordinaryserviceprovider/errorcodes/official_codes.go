package errorcodes

import "strings"

// CodeSet is the canonical matcher used by ordinary service provider endpoint
// code sets. OfficialCodes on DocumentedCodeSet keeps the endpoint's raw
// documented spellings, including WeChat's legacy aliases such as NOAUTH,
// RULELIMIT, SYSTEMERROR and FREQENCY_LIMIT.
type CodeSet map[string]struct{}

func newCodeSet(codes ...string) CodeSet {
	set := make(CodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalCode(code)] = struct{}{}
	}
	return set
}

func (s CodeSet) Has(code string) bool {
	_, ok := s[CanonicalCode(code)]
	return ok
}

func (s CodeSet) Len() int { return len(s) }

type DocumentedCodeSet struct {
	Name          string
	OfficialCodes []string
	set           CodeSet
}

func documentedCodeSet(name string, codes ...string) DocumentedCodeSet {
	copied := append([]string(nil), codes...)
	return DocumentedCodeSet{Name: name, OfficialCodes: copied, set: newCodeSet(copied...)}
}

func (s DocumentedCodeSet) Has(code string) bool { return s.set.Has(code) }
func (s DocumentedCodeSet) Len() int             { return len(s.OfficialCodes) }

const (
	CodeAlreadyExists           = "ALREADY_EXISTS"
	CodeAppIDMchIDNotMatch      = "APPID_MCHID_NOT_MATCH"
	CodeApplymentNotExist       = "APPLYMENT_NOT_EXIST"
	CodeApplymentNotExistLegacy = "APPLYMENT_NOTEXIST"
	CodeFrequencyLimit          = "FREQENCY_LIMIT"
	CodeFrequencyLimited        = "FREQUENCY_LIMITED"
	CodeFrequencyLimitExceed    = "FREQUENCY_LIMIT_EXCEED"
	CodeInvalidRequest          = "INVALID_REQUEST"
	CodeMchNotExists            = "MCH_NOT_EXISTS"
	CodeNoAuth                  = "NO_AUTH"
	CodeNoAuthLegacy            = "NOAUTH"
	CodeNotEnough               = "NOT_ENOUGH"
	CodeNotFound                = "NOT_FOUND"
	CodeOpenIDMismatch          = "OPENID_MISMATCH"
	CodeOrderClosed             = "ORDER_CLOSED"
	CodeOrderNotExist           = "ORDER_NOT_EXIST"
	CodeOutTradeNoUsed          = "OUT_TRADE_NO_USED"
	CodeParamError              = "PARAM_ERROR"
	CodeProcessing              = "PROCESSING"
	CodeRateLimitExceeded       = "RATELIMIT_EXCEEDED"
	CodeRateLimited             = "RATE_LIMITED"
	CodeRequestBlocked          = "REQUEST_BLOCKED"
	CodeResourceNotExists       = "RESOURCE_NOT_EXISTS"
	CodeRuleLimit               = "RULE_LIMIT"
	CodeRuleLimitLegacy         = "RULELIMIT"
	CodeSignError               = "SIGN_ERROR"
	CodeSystemError             = "SYSTEM_ERROR"
	CodeSystemErrorLegacy       = "SYSTEMERROR"
	CodeTradeError              = "TRADE_ERROR"
	CodeUserAccountAbnormal     = "USER_ACCOUNT_ABNORMAL"
)

const CodeFrequencyLimitCompat = "FREQUENCY_LIMIT"

var codeAliases = map[string]string{
	CodeNoAuthLegacy:            CodeNoAuth,
	CodeRuleLimitLegacy:         CodeRuleLimit,
	CodeSystemErrorLegacy:       CodeSystemError,
	CodeFrequencyLimitCompat:    CodeFrequencyLimit,
	CodeApplymentNotExistLegacy: CodeApplymentNotExist,
}

func CanonicalCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if canonical, ok := codeAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

var OrdinaryCommonCodes = documentedCodeSet("OrdinaryCommonCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError)

// Official endpoint error-code sets from .github/standards/domains/wechat-payment/README.md 4.10.
var (
	ApplymentSubmitDocumentedCodes             = documentedCodeSet("ApplymentSubmitDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeApplymentNotExistLegacy, CodeProcessing, CodeNoAuth, CodeRequestBlocked, CodeRateLimited)
	ApplymentQueryDocumentedCodes              = documentedCodeSet("ApplymentQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeApplymentNotExist, CodeNoAuth, CodeProcessing, CodeRateLimited)
	SettlementModifyDocumentedCodes            = documentedCodeSet("SettlementModifyDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeFrequencyLimit)
	SettlementQueryDocumentedCodes             = documentedCodeSet("SettlementQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth)
	SettlementModificationQueryDocumentedCodes = documentedCodeSet("SettlementModificationQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeOrderNotExist)
	MerchantMediaUploadDocumentedCodes         = documentedCodeSet("MerchantMediaUploadDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeFrequencyLimitExceed, CodeNoAuth)

	ViolationNotificationConfigQueryDocumentedCodes           = documentedCodeSet("ViolationNotificationConfigQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeNotFound, CodeFrequencyLimited)
	ViolationNotificationConfigUpdateDocumentedCodes          = documentedCodeSet("ViolationNotificationConfigUpdateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeNotFound, CodeFrequencyLimited)
	ViolationNotificationConfigCreateDocumentedCodes          = documentedCodeSet("ViolationNotificationConfigCreateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeAlreadyExists, CodeFrequencyLimited)
	ViolationNotificationConfigDeleteDocumentedCodes          = documentedCodeSet("ViolationNotificationConfigDeleteDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeNotFound, CodeFrequencyLimited)
	MerchantLimitationQueryDocumentedCodes                    = documentedCodeSet("MerchantLimitationQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeRateLimitExceeded)
	InactiveMerchantIdentityVerificationCreateDocumentedCodes = documentedCodeSet("InactiveMerchantIdentityVerificationCreateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeFrequencyLimitExceed)
	InactiveMerchantIdentityVerificationQueryDocumentedCodes  = documentedCodeSet("InactiveMerchantIdentityVerificationQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeNotFound, CodeFrequencyLimitExceed)

	PaymentPrepayDocumentedCodes = documentedCodeSet("PaymentPrepayDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeAppIDMchIDNotMatch, CodeMchNotExists, CodeNoAuth, CodeOutTradeNoUsed, CodeFrequencyLimited)
	PaymentQueryDocumentedCodes  = documentedCodeSet("PaymentQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeMchNotExists, CodeRuleLimit, CodeTradeError, CodeOrderNotExist, CodeFrequencyLimited)
	PaymentCloseDocumentedCodes  = documentedCodeSet("PaymentCloseDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeMchNotExists, CodeRuleLimit, CodeTradeError, CodeFrequencyLimited)

	RefundCreateDocumentedCodes = documentedCodeSet("RefundCreateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNotEnough, CodeUserAccountAbnormal, CodeMchNotExists, CodeResourceNotExists, CodeFrequencyLimited)
	RefundQueryDocumentedCodes  = documentedCodeSet("RefundQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeMchNotExists, CodeResourceNotExists)

	CombinePrepayDocumentedCodes = documentedCodeSet("CombinePrepayDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeAppIDMchIDNotMatch, CodeMchNotExists, CodeOrderClosed, CodeNoAuthLegacy, CodeOutTradeNoUsed, CodeRuleLimitLegacy, CodeFrequencyLimited, CodeOpenIDMismatch, CodeSystemErrorLegacy)
	CombineQueryDocumentedCodes  = documentedCodeSet("CombineQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeAppIDMchIDNotMatch, CodeMchNotExists, CodeOrderClosed, CodeNoAuthLegacy, CodeOutTradeNoUsed, CodeRuleLimitLegacy, CodeFrequencyLimited, CodeSystemErrorLegacy)
	CombineCloseDocumentedCodes  = documentedCodeSet("CombineCloseDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeAppIDMchIDNotMatch, CodeMchNotExists, CodeOrderClosed, CodeNoAuthLegacy, CodeOutTradeNoUsed, CodeRuleLimitLegacy, CodeFrequencyLimited, CodeSystemErrorLegacy)

	ProfitSharingCreateDocumentedCodes          = documentedCodeSet("ProfitSharingCreateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeRuleLimit, CodeNotEnough, CodeFrequencyLimited)
	ProfitSharingQueryDocumentedCodes           = documentedCodeSet("ProfitSharingQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeResourceNotExists, CodeFrequencyLimited)
	ProfitSharingReturnCreateDocumentedCodes    = documentedCodeSet("ProfitSharingReturnCreateDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeFrequencyLimited)
	ProfitSharingReturnQueryDocumentedCodes     = documentedCodeSet("ProfitSharingReturnQueryDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeResourceNotExists, CodeFrequencyLimited)
	ProfitSharingUnfreezeDocumentedCodes        = documentedCodeSet("ProfitSharingUnfreezeDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeNotEnough, CodeFrequencyLimited)
	ProfitSharingRemainingAmountDocumentedCodes = documentedCodeSet("ProfitSharingRemainingAmountDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeResourceNotExists, CodeFrequencyLimited)
	ProfitSharingReceiverAddDocumentedCodes     = documentedCodeSet("ProfitSharingReceiverAddDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeFrequencyLimited)
	ProfitSharingReceiverDeleteDocumentedCodes  = documentedCodeSet("ProfitSharingReceiverDeleteDocumentedCodes", CodeParamError, CodeInvalidRequest, CodeSignError, CodeSystemError, CodeNoAuth, CodeFrequencyLimited)
)
