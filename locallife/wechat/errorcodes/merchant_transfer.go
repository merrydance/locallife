package errorcodes

import "strings"

type MerchantTransferCodeSet map[string]struct{}

func newMerchantTransferCodeSet(codes ...string) MerchantTransferCodeSet {
	set := make(MerchantTransferCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalMerchantTransferCode(code)] = struct{}{}
	}
	return set
}

func (s MerchantTransferCodeSet) Has(code string) bool {
	_, ok := s[CanonicalMerchantTransferCode(code)]
	return ok
}

const (
	MerchantTransferCodeParamError           = "PARAM_ERROR"
	MerchantTransferCodeInvalidRequest       = "INVALID_REQUEST"
	MerchantTransferCodeNoAuth               = "NO_AUTH"
	MerchantTransferCodeSignError            = "SIGN_ERROR"
	MerchantTransferCodeSystemError          = "SYSTEM_ERROR"
	MerchantTransferCodeNotEnough            = "NOT_ENOUGH"
	MerchantTransferCodeFrequencyLimited     = "FREQUENCY_LIMITED"
	MerchantTransferCodeFrequencyLimit       = "FREQUENCY_LIMIT"
	MerchantTransferCodeFrequencyLimitExceed = "FREQUENCY_LIMIT_EXCEED"
	MerchantTransferCodeRateLimitExceeded    = "RATELIMIT_EXCEEDED"
	MerchantTransferCodeAlreadyExists        = "ALREADY_EXISTS"
	MerchantTransferCodeNotFound             = "NOT_FOUND"
)

func CanonicalMerchantTransferCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

var MerchantTransferCommonCodes = newMerchantTransferCodeSet(
	MerchantTransferCodeParamError,
	MerchantTransferCodeInvalidRequest,
	MerchantTransferCodeSignError,
	MerchantTransferCodeSystemError,
)

// POST /v3/fund-app/mch-transfer/transfer-bills
var MerchantTransferCreateDocumentedCodes = newMerchantTransferCodeSet(
	MerchantTransferCodeParamError,
	MerchantTransferCodeInvalidRequest,
	MerchantTransferCodeNoAuth,
	MerchantTransferCodeSignError,
	MerchantTransferCodeSystemError,
	MerchantTransferCodeNotEnough,
	MerchantTransferCodeFrequencyLimitExceed,
	MerchantTransferCodeRateLimitExceeded,
	MerchantTransferCodeFrequencyLimit,
	MerchantTransferCodeAlreadyExists,
)

// GET /v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}
// GET /v3/fund-app/mch-transfer/transfer-bills/transfer-bill-no/{transfer_bill_no}
var MerchantTransferQueryDocumentedCodes = newMerchantTransferCodeSet(
	MerchantTransferCodeParamError,
	MerchantTransferCodeInvalidRequest,
	MerchantTransferCodeNoAuth,
	MerchantTransferCodeSignError,
	MerchantTransferCodeNotFound,
	MerchantTransferCodeFrequencyLimited,
	MerchantTransferCodeSystemError,
	MerchantTransferCodeFrequencyLimitExceed,
	MerchantTransferCodeRateLimitExceeded,
)

// POST /v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}/cancel
var MerchantTransferCancelDocumentedCodes = newMerchantTransferCodeSet(
	MerchantTransferCodeParamError,
	MerchantTransferCodeInvalidRequest,
	MerchantTransferCodeSignError,
	MerchantTransferCodeSystemError,
	MerchantTransferCodeFrequencyLimitExceed,
	MerchantTransferCodeRateLimitExceeded,
)
