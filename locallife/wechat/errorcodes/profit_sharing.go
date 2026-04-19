package errorcodes

import "strings"

type ProfitSharingCodeSet map[string]struct{}

func newProfitSharingCodeSet(codes ...string) ProfitSharingCodeSet {
	set := make(ProfitSharingCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalProfitSharingCode(code)] = struct{}{}
	}
	return set
}

func (s ProfitSharingCodeSet) Has(code string) bool {
	_, ok := s[CanonicalProfitSharingCode(code)]
	return ok
}

const (
	ProfitSharingCodeParamError       = "PARAM_ERROR"
	ProfitSharingCodeInvalidRequest   = "INVALID_REQUEST"
	ProfitSharingCodeSignError        = "SIGN_ERROR"
	ProfitSharingCodeSystemError      = "SYSTEM_ERROR"
	ProfitSharingCodeNoAuth           = "NO_AUTH"
	ProfitSharingCodeRuleLimit        = "RULE_LIMIT"
	ProfitSharingCodeNotEnough        = "NOT_ENOUGH"
	ProfitSharingCodeFrequencyLimited = "FREQUENCY_LIMITED"
	ProfitSharingCodeResourceNotExist = "RESOURCE_NOT_EXISTS"
)

func CanonicalProfitSharingCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func ProfitSharingCodeEquals(code, expected string) bool {
	return CanonicalProfitSharingCode(code) == CanonicalProfitSharingCode(expected)
}

func IsProfitSharingReturnProcessingCode(code string) bool {
	switch CanonicalProfitSharingCode(code) {
	case ProfitSharingCodeNotEnough:
		return true
	default:
		return false
	}
}

var ProfitSharingCommonCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
)

var ProfitSharingCreateDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeNoAuth,
	ProfitSharingCodeRuleLimit,
	ProfitSharingCodeNotEnough,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingQueryDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeResourceNotExist,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingAmountsDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeResourceNotExist,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingFinishDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeNoAuth,
	ProfitSharingCodeNotEnough,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingAddReceiverDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeNoAuth,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingDeleteReceiverDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeNoAuth,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingReturnCreateDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeNoAuth,
	ProfitSharingCodeNotEnough,
	ProfitSharingCodeFrequencyLimited,
)

var ProfitSharingReturnQueryDocumentedCodes = newProfitSharingCodeSet(
	ProfitSharingCodeParamError,
	ProfitSharingCodeInvalidRequest,
	ProfitSharingCodeSignError,
	ProfitSharingCodeSystemError,
	ProfitSharingCodeResourceNotExist,
	ProfitSharingCodeFrequencyLimited,
)
