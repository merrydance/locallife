package errorcodes

import "strings"

type SubsidyCodeSet map[string]struct{}

func newSubsidyCodeSet(codes ...string) SubsidyCodeSet {
	set := make(SubsidyCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalSubsidyCode(code)] = struct{}{}
	}
	return set
}

func (s SubsidyCodeSet) Has(code string) bool {
	_, ok := s[CanonicalSubsidyCode(code)]
	return ok
}

const (
	SubsidyCodeFrequencyLimited = "FREQUENCY_LIMITED"
	SubsidyCodeNoAuth           = "NO_AUTH"
	SubsidyCodeNotEnough        = "NOT_ENOUGH"
	SubsidyCodeOrderNotExist    = "ORDER_NOT_EXIST"
	SubsidyCodeInvalidRequest   = "INVALID_REQUEST"
	SubsidyCodeParamError       = "PARAM_ERROR"
	SubsidyCodeSignError        = "SIGN_ERROR"
	SubsidyCodeSystemError      = "SYSTEM_ERROR"
)

func CanonicalSubsidyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func SubsidyCodeEquals(code, expected string) bool {
	return CanonicalSubsidyCode(code) == CanonicalSubsidyCode(expected)
}

var SubsidyCommonCodes = newSubsidyCodeSet(
	SubsidyCodeParamError,
	SubsidyCodeInvalidRequest,
	SubsidyCodeSignError,
	SubsidyCodeSystemError,
)

var SubsidyCreateDocumentedCodes = newSubsidyCodeSet(
	SubsidyCodeParamError,
	SubsidyCodeInvalidRequest,
	SubsidyCodeSignError,
	SubsidyCodeSystemError,
	SubsidyCodeFrequencyLimited,
)

var SubsidyReturnDocumentedCodes = newSubsidyCodeSet(
	SubsidyCodeParamError,
	SubsidyCodeInvalidRequest,
	SubsidyCodeSignError,
	SubsidyCodeSystemError,
	SubsidyCodeFrequencyLimited,
)

var SubsidyCancelDocumentedCodes = newSubsidyCodeSet(
	SubsidyCodeParamError,
	SubsidyCodeInvalidRequest,
	SubsidyCodeSignError,
	SubsidyCodeSystemError,
	SubsidyCodeFrequencyLimited,
)
