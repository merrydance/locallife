package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditFundManagementAlignment_CoversBalanceWithdrawAndNotify(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"账户资金管理组", "查询二级商户账户日终余额", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/ecommerce/fund/enddaybalance/{sub_mchid}"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"账户资金管理组", "查询二级商户账户日终余额", "请求参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "date", Description: "日期，格式YYYY-MM-DD"},
					{Name: "account_type", EnumValues: []string{"BASIC", "DEPOSIT"}},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"账户资金管理组", "查询二级商户账户日终余额", "应答参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "available_amount", Description: "可用余额（单位：分）"},
					{Name: "pending_amount", Description: "不可用余额（单位：分）"},
					{Name: "account_type", EnumValues: []string{"BASIC", "DEPOSIT"}},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"账户资金管理组", "查询二级商户账户日终余额", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "NO_AUTH"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"账户资金管理组", "平台预约提现", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/merchant/fund/withdraw"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"账户资金管理组", "平台预约提现", "Body 参数"},
				Fields: []Field{
					{Name: "out_request_no"},
					{Name: "amount", Description: "提现金额，单位：分，不能超过8亿元"},
					{Name: "remark"},
					{Name: "bank_memo"},
					{Name: "account_type", EnumValues: []string{"BASIC", "FEES", "OPERATION"}},
					{Name: "notify_url"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"账户资金管理组", "平台预约提现", "应答参数"},
				Fields:  []Field{{Name: "withdraw_id"}, {Name: "out_request_no"}},
			},
			{
				Heading:    "错误码",
				Path:       []string{"账户资金管理组", "平台预约提现", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "ACCOUNT_ERROR"}, {Code: "ACCOUNT_NOT_VERIFIED"}, {Code: "CONTRACT_NOT_CONFIRMED"}, {Code: "NO_AUTH"}, {Code: "NOT_ENOUGH"}, {Code: "REQUEST_BLOCKED"}, {Code: "ORDER_NOT_EXIST"}, {Code: "FREQUENCY_LIMITED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"账户资金管理组", "商户提现状态变更通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-ecommerce/withdraw-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"账户资金管理组", "商户提现状态变更通知", "Body 参数"},
				Fields: []Field{
					{Name: "sp_mchid"},
					{Name: "sub_mchid"},
					{Name: "status", EnumValues: []string{"CREATE_SUCCESS", "SUCCESS", "FAIL", "REFUND", "CLOSE", "INIT", "CREATED", "PROCESSING", "FINISHED", "ABNORMAL"}},
					{Name: "withdraw_id"},
					{Name: "out_request_no"},
					{Name: "amount", Description: "提现金额，单位：分"},
					{Name: "total_amount", Description: "提现金额，单位：分"},
					{Name: "success_amount", Description: "提现成功金额，单位：分"},
					{Name: "fail_amount", Description: "提现失败金额，单位：分"},
					{Name: "refund_amount", Description: "提现退票金额，单位：分"},
					{Name: "create_time", Description: "提交预约时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "update_time", Description: "提现状态更新时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "reason"},
					{Name: "remark"},
					{Name: "bank_memo"},
					{Name: "account_type", EnumValues: []string{"BASIC", "FEES", "OPERATION"}},
					{Name: "solution"},
					{Name: "account_number"},
					{Name: "account_bank"},
					{Name: "bank_name"},
				},
			},
		},
	}

	report := AuditFundManagementAlignment(extraction)
	require.Equal(t, "fund_management", report.Scope)
	require.Equal(t, 3, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 3, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
}

func TestAuditFundManagementAlignment_CoversDayEndWithdrawAndBill(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"账户资金管理组", "二级商户按日终余额预约提现", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"账户资金管理组", "二级商户按日终余额预约提现", "Body 参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "out_request_no"},
					{Name: "calculate_amount_type", EnumValues: []string{"ONLY_DAY_END_BALANCE", "ALLOW_CURRENT_BALANCE"}},
					{Name: "remark"},
					{Name: "bank_memo"},
					{Name: "notify_url"},
					{Name: "reserve_amount", Description: "留存额，单位：分"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"账户资金管理组", "二级商户按日终余额预约提现", "应答参数"},
				Fields: []Field{
					{Name: "sp_mchid"},
					{Name: "sub_mchid"},
					{Name: "status", EnumValues: []string{"CREATED", "PROCESSING", "FINISHED", "ABNORMAL"}},
					{Name: "withdraw_id"},
					{Name: "out_request_no"},
					{Name: "total_amount", Description: "提现金额，单位：分"},
					{Name: "success_amount", Description: "提现成功金额，单位：分"},
					{Name: "fail_amount", Description: "提现失败金额，单位：分"},
					{Name: "refund_amount", Description: "提现退票金额，单位：分"},
					{Name: "create_time", Description: "遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "update_time", Description: "遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "reason"},
					{Name: "remark"},
					{Name: "bank_memo"},
					{Name: "account_type", EnumValues: []string{"BASIC", "FEES", "OPERATION"}},
					{Name: "account_number"},
					{Name: "account_bank"},
					{Name: "bank_name"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"账户资金管理组", "二级商户按日终余额预约提现", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "ACCOUNT_ERROR"}, {Code: "ACCOUNT_NOT_VERIFIED"}, {Code: "CONTRACT_NOT_CONFIRMED"}, {Code: "NO_AUTH"}, {Code: "NOT_ENOUGH"}, {Code: "REQUEST_BLOCKED"}, {Code: "ORDER_NOT_EXIST"}, {Code: "FREQUENCY_LIMITED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"账户资金管理组", "按日下载提现异常文件", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/merchant/fund/withdraw/bill-type/{bill_type}"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"账户资金管理组", "按日下载提现异常文件", "请求参数"},
				Fields: []Field{
					{Name: "bill_type", EnumValues: []string{"NO_SUCC"}},
					{Name: "bill_date", Description: "表示所在日期的提现账单，格式YYYY-MM-DD"},
					{Name: "tar_type", EnumValues: []string{"GZIP"}},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"账户资金管理组", "按日下载提现异常文件", "应答参数"},
				Fields:  []Field{{Name: "hash_type", EnumValues: []string{"SHA1"}}, {Name: "hash_value"}, {Name: "download_url"}},
			},
			{
				Heading:    "错误码",
				Path:       []string{"账户资金管理组", "按日下载提现异常文件", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "NO_STATEMENT_EXIST"}, {Code: "STATEMENT_CREATING"}, {Code: "NO_AUTH"}},
			},
		},
	}

	report := AuditFundManagementAlignment(extraction)
	require.Equal(t, 2, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 2, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
}
