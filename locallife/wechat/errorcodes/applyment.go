package errorcodes

import "strings"

type ApplymentCodeSet map[string]struct{}

func newApplymentCodeSet(codes ...string) ApplymentCodeSet {
	set := make(ApplymentCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalApplymentCode(code)] = struct{}{}
	}
	return set
}

func (s ApplymentCodeSet) Has(code string) bool {
	_, ok := s[CanonicalApplymentCode(code)]
	return ok
}

const (
	ApplymentCodeParamError            = "PARAM_ERROR"
	ApplymentCodeInvalidRequest        = "INVALID_REQUEST"
	ApplymentCodeSignError             = "SIGN_ERROR"
	ApplymentCodeSystemError           = "SYSTEM_ERROR"
	ApplymentCodeNoAuth                = "NO_AUTH"
	ApplymentCodeNotFound              = "NOT_FOUND"
	ApplymentCodeFrequencyLimited      = "FREQUENCY_LIMITED"
	ApplymentCodeResourceAlreadyExists = "RESOURCE_ALREADY_EXISTS"
	ApplymentCodeResourceNotExists     = "RESOURCE_NOT_EXISTS"
	ApplymentCodeRateLimitExceeded     = "RATELIMIT_EXCEEDED"
	ApplymentCodeOrderNotExist         = "ORDER_NOT_EXIST"
	ApplymentCodeFrequencyLimit        = "FREQENCY_LIMIT"
	ApplymentCodeFrequencyLimitExceed  = "FREQUENCY_LIMIT_EXCEED"
	ApplymentCompatCodeFrequencyLimit  = "FREQUENCY_LIMIT"
)

var applymentCodeAliases = map[string]string{
	ApplymentCompatCodeFrequencyLimit: ApplymentCodeFrequencyLimit,
}

func CanonicalApplymentCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if canonical, ok := applymentCodeAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

var ApplymentCommonCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
)

// POST /v3/ecommerce/applyments/
var EcommerceApplymentCreateDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeResourceAlreadyExists,
	ApplymentCodeNoAuth,
	ApplymentCodeResourceNotExists,
)

// GET /v3/ecommerce/applyments/out-request-no/{out_request_no}
// GET /v3/ecommerce/applyments/{applyment_id}
var EcommerceApplymentQueryDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeResourceAlreadyExists,
	ApplymentCodeNoAuth,
	ApplymentCodeResourceNotExists,
	ApplymentCodeRateLimitExceeded,
)

// POST /v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement
var SubMerchantSettlementModifyDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNoAuth,
	ApplymentCodeFrequencyLimit,
)

// GET /v3/apply4sub/sub_merchants/{sub_mchid}/settlement
var SubMerchantSettlementQueryDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
)

// GET /v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}
var SubMerchantSettlementApplicationQueryDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNoAuth,
	ApplymentCodeOrderNotExist,
)

// POST /v3/merchant/media/upload
var MerchantMediaUploadDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNoAuth,
	ApplymentCodeFrequencyLimitExceed,
)

// GET /v3/capital/capitallhh/banks/personal-banking
var CapitalPersonalBankListDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNotFound,
	ApplymentCodeFrequencyLimited,
)

// GET /v3/capital/capitallhh/banks/corporate-banking
var CapitalCorporateBankListDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNotFound,
	ApplymentCodeFrequencyLimited,
)

// GET /v3/capital/capitallhh/banks/search-banks-by-bank-account
var CapitalBankAccountSearchKnownCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNotFound,
	ApplymentCodeFrequencyLimited,
)

// GET /v3/capital/capitallhh/areas/provinces
var CapitalProvinceListKnownCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
)

// GET /v3/capital/capitallhh/areas/provinces/{province_code}/cities
var CapitalCityListKnownCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
)

// GET /v3/capital/capitallhh/banks/{bank_alias_code}/branches
var CapitalBankBranchListDocumentedCodes = newApplymentCodeSet(
	ApplymentCodeParamError,
	ApplymentCodeInvalidRequest,
	ApplymentCodeSignError,
	ApplymentCodeSystemError,
	ApplymentCodeNotFound,
	ApplymentCodeFrequencyLimited,
)
