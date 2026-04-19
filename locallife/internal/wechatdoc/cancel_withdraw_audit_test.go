package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditCancelWithdrawAlignment_CoversQuerySemantics(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"商户注销组", "商户申请单号查询申请单状态", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{out_request_no}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"商户注销组", "商户申请单号查询申请单状态", "Path 参数"},
				Fields:  []Field{{Name: "out_request_no"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"商户注销组", "商户申请单号查询申请单状态", "应答参数"},
				Fields: []Field{
					{Name: "applyment_id"},
					{Name: "out_request_no"},
					{Name: "cancel_state", EnumValues: []string{"ACCEPTED", "REVIEWING", "REJECTED", "WAITING_MERCHANT_CONFIRM", "REVOKED", "SYSTEM_PROCESSING", "CANCELED", "FUND_PROCESSING", "FINISH"}},
					{Name: "cancel_state_description"},
					{Name: "withdraw", EnumValues: []string{"NOT_APPLY_WITHDRAW", "APPLY_WITHDRAW"}},
					{Name: "withdraw_state", EnumValues: []string{"WITHDRAW_PROCESSING", "WITHDRAW_EXCEPTION", "WITHDRAW_SUCCEED"}},
					{Name: "withdraw_state_description"},
					{Name: "account_withdraw_result"},
					{Name: "account_withdraw_result.out_account_type", EnumValues: []string{"BASIC_ACCOUNT", "OPERATE_ACCOUNT", "MARGIN_ACCOUNT", "TRADE_FEE_ACCOUNT"}},
					{Name: "account_withdraw_result.pay_state", EnumValues: []string{"PAY_PROCESSING", "PAY_SUCCEED", "PAY_FAIL", "BANK_REFUNDED"}},
					{Name: "account_withdraw_result.state_description"},
					{Name: "modify_time", Description: "最后更新时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "sub_mchid"},
					{Name: "account_info"},
					{Name: "account_info.out_account_type", EnumValues: []string{"BASIC_ACCOUNT", "OPERATE_ACCOUNT", "MARGIN_ACCOUNT", "TRADE_FEE_ACCOUNT"}},
					{Name: "account_info.amount", Description: "账户金额，单位：分"},
					{Name: "confirm_cancel"},
					{Name: "confirm_cancel.confirm_cancel_url"},
				},
			},
			{
				Heading:    "公共错误码",
				Path:       []string{"商户注销组", "商户申请单号查询申请单状态", "公共错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}},
			},
		},
	}

	report := AuditCancelWithdrawAlignment(extraction)
	require.Equal(t, "cancel_withdraw", report.Scope)
	require.Equal(t, 1, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 1, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
	q := findEndpointAudit(report.Endpoints, "GET", "/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{out_request_no}")
	require.Nil(t, q)
}