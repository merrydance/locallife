package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditRefundAlignment_CoversCreateNotifyAndAdvanceQuery(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"交易退款组", "申请退款", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/ecommerce/refunds/apply"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"交易退款组", "申请退款", "Body 参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "sp_appid"},
					{Name: "sub_appid"},
					{Name: "transaction_id"},
					{Name: "out_trade_no"},
					{Name: "out_refund_no"},
					{Name: "reason"},
					{Name: "amount"},
					{Name: "amount.refund", Description: "退款金额，单位为分"},
					{Name: "amount.from"},
					{Name: "amount.from.account", EnumValues: []string{"AVAILABLE", "UNAVAILABLE"}},
					{Name: "amount.from.amount", Description: "出资金额，单位为分"},
					{Name: "amount.total", Description: "原订单金额，单位为分"},
					{Name: "amount.currency", EnumValues: []string{"CNY"}},
					{Name: "notify_url"},
					{Name: "refund_account", EnumValues: []string{"REFUND_SOURCE_PARTNER_ADVANCE", "REFUND_SOURCE_SUB_MERCHANT"}},
					{Name: "funds_account", EnumValues: []string{"AVAILABLE"}},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"交易退款组", "申请退款", "应答参数"},
				Fields: []Field{
					{Name: "refund_id"},
					{Name: "out_refund_no"},
					{Name: "create_time", Description: "退款创建时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "amount"},
					{Name: "amount.refund", Description: "退款金额，单位为分"},
					{Name: "amount.from"},
					{Name: "amount.from.account", EnumValues: []string{"AVAILABLE", "UNAVAILABLE"}},
					{Name: "amount.from.amount", Description: "出资金额，单位为分"},
					{Name: "amount.payer_refund", Description: "用户退款金额，单位为分"},
					{Name: "amount.discount_refund", Description: "优惠退款金额，单位为分"},
					{Name: "amount.currency", EnumValues: []string{"CNY"}},
					{Name: "amount.advance", Description: "垫付金额，单位为分"},
					{Name: "promotion_detail"},
					{Name: "promotion_detail.promotion_id"},
					{Name: "promotion_detail.scope", EnumValues: []string{"GLOBAL", "SINGLE"}},
					{Name: "promotion_detail.type", EnumValues: []string{"COUPON", "DISCOUNT"}},
					{Name: "promotion_detail.amount", Description: "优惠券面额，单位为分"},
					{Name: "promotion_detail.refund_amount", Description: "优惠退款金额，单位为分"},
					{Name: "refund_account", EnumValues: []string{"REFUND_SOURCE_PARTNER_ADVANCE", "REFUND_SOURCE_SUB_MERCHANT"}},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"交易退款组", "申请退款", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "MCH_NOT_EXISTS"}, {Code: "NO_AUTH"}, {Code: "NOT_ENOUGH"}, {Code: "USER_ACCOUNT_ABNORMAL"}, {Code: "RESOURCE_NOT_EXISTS"}, {Code: "FREQUENCY_LIMITED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"交易退款组", "退款结果通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-ecommerce/refund-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"交易退款组", "退款结果通知", "Body 参数"},
				Fields: []Field{
					{Name: "sp_mchid"},
					{Name: "sub_mchid"},
					{Name: "out_trade_no"},
					{Name: "transaction_id"},
					{Name: "out_refund_no"},
					{Name: "refund_id"},
					{Name: "refund_status", EnumValues: []string{"SUCCESS", "CLOSED", "ABNORMAL"}},
					{Name: "success_time", Description: "退款成功时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "user_received_account"},
					{Name: "amount"},
					{Name: "amount.total", Description: "订单总金额，单位为分"},
					{Name: "amount.refund", Description: "退款金额，单位为分"},
					{Name: "amount.payer_total", Description: "用户实际支付金额，单位为分"},
					{Name: "amount.payer_refund", Description: "用户退款金额，单位为分"},
					{Name: "refund_account", EnumValues: []string{"REFUND_SOURCE_PARTNER_ADVANCE", "REFUND_SOURCE_SUB_MERCHANT"}},
				},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"交易退款组", "查询垫付回补通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/ecommerce/refunds/{refund_id}/return-advance"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"交易退款组", "查询垫付回补通知", "请求参数"},
				Fields:  []Field{{Name: "refund_id"}, {Name: "sub_mchid"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"交易退款组", "查询垫付回补通知", "应答参数"},
				Fields: []Field{
					{Name: "refund_id"},
					{Name: "advance_return_id"},
					{Name: "return_amount", Description: "垫付回补金额，单位为分"},
					{Name: "payer_mchid"},
					{Name: "payer_account", EnumValues: []string{"BASIC", "OPERATION"}},
					{Name: "payee_mchid"},
					{Name: "payee_account", EnumValues: []string{"BASIC", "OPERATION"}},
					{Name: "result", EnumValues: []string{"SUCCESS", "FAILED", "PROCESSING"}},
					{Name: "success_time", Description: "垫付回补完成时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"交易退款组", "查询垫付回补通知", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "MCH_NOT_EXISTS"}, {Code: "RESOURCE_NOT_EXISTS"}},
			},
		},
	}

	report := AuditRefundAlignment(extraction)
	require.Equal(t, "refund", report.Scope)
	require.Equal(t, 3, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 3, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
}

func TestAuditRefundAlignment_CoversAbnormalRefund(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"交易退款组", "发起异常退款", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"交易退款组", "发起异常退款", "请求参数"},
				Fields: []Field{
					{Name: "refund_id"},
					{Name: "sub_mchid"},
					{Name: "out_refund_no"},
					{Name: "type", EnumValues: []string{"USER_BANK_CARD", "MERCHANT_BANK_CARD"}},
					{Name: "bank_type"},
					{Name: "bank_account"},
					{Name: "real_name"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"交易退款组", "发起异常退款", "应答参数"},
				Fields: []Field{
					{Name: "refund_id"},
					{Name: "out_refund_no"},
					{Name: "transaction_id"},
					{Name: "out_trade_no"},
					{Name: "channel", EnumValues: []string{"ORIGINAL", "BALANCE", "OTHER_BALANCE", "OTHER_BANKCARD"}},
					{Name: "user_received_account"},
					{Name: "success_time", Description: "退款成功时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "create_time", Description: "退款创建时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "status", EnumValues: []string{"SUCCESS", "CLOSED", "PROCESSING", "ABNORMAL"}},
					{Name: "amount"},
					{Name: "amount.refund", Description: "退款金额，单位为分"},
					{Name: "amount.from"},
					{Name: "amount.from.account", EnumValues: []string{"AVAILABLE", "UNAVAILABLE"}},
					{Name: "amount.from.amount", Description: "出资金额，单位为分"},
					{Name: "amount.payer_refund", Description: "用户退款金额，单位为分"},
					{Name: "amount.discount_refund", Description: "优惠退款金额，单位为分"},
					{Name: "amount.currency", EnumValues: []string{"CNY"}},
					{Name: "amount.advance", Description: "垫付金额，单位为分"},
					{Name: "promotion_detail"},
					{Name: "promotion_detail.promotion_id"},
					{Name: "promotion_detail.scope", EnumValues: []string{"GLOBAL", "SINGLE"}},
					{Name: "promotion_detail.type", EnumValues: []string{"COUPON", "DISCOUNT"}},
					{Name: "promotion_detail.amount", Description: "优惠券面额，单位为分"},
					{Name: "promotion_detail.refund_amount", Description: "优惠退款金额，单位为分"},
					{Name: "refund_account", EnumValues: []string{"REFUND_SOURCE_PARTNER_ADVANCE", "REFUND_SOURCE_SUB_MERCHANT"}},
					{Name: "funds_account", EnumValues: []string{"UNSETTLED", "AVAILABLE", "UNAVAILABLE", "OPERATION", "BASIC", "ECNY_BASIC"}},
				},
			},
			{
				Heading:    "公共错误码",
				Path:       []string{"交易退款组", "发起异常退款", "公共错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}},
			},
		},
	}

	report := AuditRefundAlignment(extraction)
	require.Equal(t, 1, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 1, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
}
