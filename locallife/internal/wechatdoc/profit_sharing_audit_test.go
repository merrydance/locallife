package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditProfitSharingAlignment_CoversQueryReturnOrderIDAndFinishEnum(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"分账组", "请求分账", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/ecommerce/profitsharing/orders"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"分账组", "请求分账", "Body 参数"},
				Fields: []Field{
					{Name: "appid"},
					{Name: "sub_mchid"},
					{Name: "transaction_id"},
					{Name: "out_order_no"},
					{Name: "receivers"},
					{Name: "receivers.type", EnumValues: []string{"MERCHANT_ID", "PERSONAL_OPENID"}},
					{Name: "receivers.receiver_account"},
					{Name: "receivers.amount", Description: "分账金额，单位为分"},
					{Name: "receivers.description"},
					{Name: "receivers.receiver_name"},
					{Name: "finish", EnumValues: []string{"true", "false"}},
				},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"分账组", "查询分账回退结果", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/ecommerce/profitsharing/returnorders"}},
			},
			{
				Heading: "Query 参数",
				Path:    []string{"分账组", "查询分账回退结果", "Query 参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "out_return_no"},
					{Name: "order_id"},
					{Name: "out_order_no"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"分账组", "查询分账回退结果", "应答参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "order_id"},
					{Name: "out_order_no"},
					{Name: "out_return_no"},
					{Name: "return_no"},
					{Name: "return_mchid"},
					{Name: "amount", Description: "回退金额，单位为分"},
					{Name: "result", EnumValues: []string{"PROCESSING", "SUCCESS", "FAILED"}},
					{Name: "fail_reason", EnumValues: []string{"ACCOUNT_ABNORMAL", "TIME_OUT_CLOSED", "PAYER_ACCOUNT_ABNORMAL", "INVALID_REQUEST"}},
					{Name: "finish_time", Description: "完成时间，需遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "transaction_id"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"分账组", "查询分账回退结果", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "RESOURCE_NOT_EXISTS"}, {Code: "FREQUENCY_LIMITED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"分账组", "分账动账通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-ecommerce/profit-sharing-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"分账组", "分账动账通知", "Body 参数"},
				Fields: []Field{
					{Name: "sp_mchid"},
					{Name: "sub_mchid"},
					{Name: "transaction_id"},
					{Name: "order_id"},
					{Name: "out_order_no"},
					{Name: "receiver"},
					{Name: "receiver.type", EnumValues: []string{"MERCHANT_ID", "PERSONAL_OPENID"}},
					{Name: "receiver.account"},
					{Name: "receiver.amount", Description: "分账动账金额，单位为分"},
					{Name: "receiver.description"},
					{Name: "success_time", Description: "成功时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
				},
			},
		},
	}

	report := AuditProfitSharingAlignment(extraction)
	require.Equal(t, "profit_sharing", report.Scope)
	require.Equal(t, 3, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 3, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)

	queryReturnAudit := findEndpointAudit(report.Endpoints, "GET", "/v3/ecommerce/profitsharing/returnorders")
	require.Nil(t, queryReturnAudit)

	createAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/ecommerce/profitsharing/orders")
	require.Nil(t, createAudit)

	notifyAudit := findEndpointAudit(report.Endpoints, "POST", "/v1/webhooks/wechat-ecommerce/profit-sharing-notify")
	require.Nil(t, notifyAudit)
}