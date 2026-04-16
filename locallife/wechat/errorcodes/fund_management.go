package errorcodes

import "strings"

type FundManagementCodeSet map[string]struct{}

func newFundManagementCodeSet(codes ...string) FundManagementCodeSet {
	set := make(FundManagementCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalFundManagementCode(code)] = struct{}{}
	}
	return set
}

func (s FundManagementCodeSet) Has(code string) bool {
	_, ok := s[CanonicalFundManagementCode(code)]
	return ok
}

const (
	FundManagementCodeParamError           = "PARAM_ERROR"
	FundManagementCodeInvalidRequest       = "INVALID_REQUEST"
	FundManagementCodeSignError            = "SIGN_ERROR"
	FundManagementCodeSystemError          = "SYSTEM_ERROR"
	FundManagementCodeNoAuth               = "NO_AUTH"
	FundManagementCodeAccountError         = "ACCOUNT_ERROR"
	FundManagementCodeAccountNotVerified   = "ACCOUNT_NOT_VERIFIED"
	FundManagementCodeContractNotConfirmed = "CONTRACT_NOT_CONFIRMED"
	FundManagementCodeNotEnough            = "NOT_ENOUGH"
	FundManagementCodeRequestBlocked       = "REQUEST_BLOCKED"
	FundManagementCodeOrderNotExist        = "ORDER_NOT_EXIST"
	FundManagementCodeFrequencyLimited     = "FREQUENCY_LIMITED"
	FundManagementCodeNoStatementExist     = "NO_STATEMENT_EXIST"
	FundManagementCodeStatementCreating    = "STATEMENT_CREATING"
)

func CanonicalFundManagementCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

var FundManagementCommonCodes = newFundManagementCodeSet(
	FundManagementCodeParamError,
	FundManagementCodeInvalidRequest,
	FundManagementCodeSignError,
	FundManagementCodeSystemError,
)

// 四个余额查询接口的官方错误码集合。
var FundManagementBalanceDocumentedCodes = newFundManagementCodeSet(
	FundManagementCodeParamError,
	FundManagementCodeInvalidRequest,
	FundManagementCodeSignError,
	FundManagementCodeSystemError,
	FundManagementCodeNoAuth,
)

// 二级商户/平台预约提现与查询、按日终余额预约提现与查询的官方错误码集合。
var FundManagementWithdrawDocumentedCodes = newFundManagementCodeSet(
	FundManagementCodeParamError,
	FundManagementCodeInvalidRequest,
	FundManagementCodeSignError,
	FundManagementCodeSystemError,
	FundManagementCodeAccountError,
	FundManagementCodeAccountNotVerified,
	FundManagementCodeContractNotConfirmed,
	FundManagementCodeNoAuth,
	FundManagementCodeNotEnough,
	FundManagementCodeRequestBlocked,
	FundManagementCodeOrderNotExist,
	FundManagementCodeFrequencyLimited,
)

// GET /v3/merchant/fund/withdraw/bill-type/{bill_type}
var FundManagementWithdrawBillDocumentedCodes = newFundManagementCodeSet(
	FundManagementCodeParamError,
	FundManagementCodeInvalidRequest,
	FundManagementCodeSignError,
	FundManagementCodeSystemError,
	FundManagementCodeNoStatementExist,
	FundManagementCodeStatementCreating,
	FundManagementCodeNoAuth,
)
