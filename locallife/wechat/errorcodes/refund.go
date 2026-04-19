package errorcodes

import "strings"

type RefundCodeSet map[string]struct{}

func newRefundCodeSet(codes ...string) RefundCodeSet {
	set := make(RefundCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalRefundCode(code)] = struct{}{}
	}
	return set
}

func (s RefundCodeSet) Has(code string) bool {
	_, ok := s[CanonicalRefundCode(code)]
	return ok
}

const (
	RefundCodeParamError          = "PARAM_ERROR"
	RefundCodeInvalidRequest      = "INVALID_REQUEST"
	RefundCodeSignError           = "SIGN_ERROR"
	RefundCodeSystemError         = "SYSTEM_ERROR"
	RefundCodeMchNotExists        = "MCH_NOT_EXISTS"
	RefundCodeNoAuth              = "NO_AUTH"
	RefundCodeNotEnough           = "NOT_ENOUGH"
	RefundCodeUserAccountAbnormal = "USER_ACCOUNT_ABNORMAL"
	RefundCodeResourceNotExists   = "RESOURCE_NOT_EXISTS"
	RefundCodeFrequencyLimited    = "FREQUENCY_LIMITED"
	RefundCodeRequestBlocked      = "REQUEST_BLOCKED"
)

func CanonicalRefundCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

var RefundCommonCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
)

// POST /v3/ecommerce/refunds/apply
var EcommerceRefundCreateDocumentedCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
	RefundCodeMchNotExists,
	RefundCodeNoAuth,
	RefundCodeNotEnough,
	RefundCodeUserAccountAbnormal,
	RefundCodeResourceNotExists,
	RefundCodeFrequencyLimited,
)

// GET /v3/ecommerce/refunds/id/{refund_id}
// GET /v3/ecommerce/refunds/out-refund-no/{out_refund_no}
var EcommerceRefundQueryDocumentedCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
	RefundCodeMchNotExists,
	RefundCodeNoAuth,
	RefundCodeRequestBlocked,
	RefundCodeResourceNotExists,
	RefundCodeFrequencyLimited,
)

// POST /v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund
// 当前官方页面仅列出公共错误码，未单独给出业务错误码表。
var EcommerceRefundAbnormalDocumentedCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
)

// GET /v3/ecommerce/refunds/{refund_id}/return-advance
var EcommerceRefundAdvanceReturnQueryDocumentedCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
	RefundCodeMchNotExists,
	RefundCodeResourceNotExists,
)

// POST /v3/ecommerce/refunds/{refund_id}/return-advance
var EcommerceRefundAdvanceReturnCreateDocumentedCodes = newRefundCodeSet(
	RefundCodeParamError,
	RefundCodeInvalidRequest,
	RefundCodeSignError,
	RefundCodeSystemError,
	RefundCodeMchNotExists,
	RefundCodeNoAuth,
	RefundCodeNotEnough,
	RefundCodeRequestBlocked,
	RefundCodeResourceNotExists,
)
