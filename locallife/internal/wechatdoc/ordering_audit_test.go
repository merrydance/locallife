package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditOrderingAlignment_FindsOnlyRealGaps(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-小程序下单", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/pay/partner/transactions/jsapi"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "普通支付-小程序下单", "Body 参数"},
				Fields:  []Field{{Name: "sp_appid"}, {Name: "sp_mchid"}, {Name: "sub_mchid"}, {Name: "description"}, {Name: "out_trade_no"}, {Name: "notify_url"}, {Name: "amount"}, {Name: "amount.total", Description: "总金额，单位为分"}, {Name: "payer"}, {Name: "payer.sp_openid"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"支付下单组", "普通支付-小程序下单", "应答参数"},
				Fields:  []Field{{Name: "prepay_id"}},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"支付下单组", "普通支付-小程序下单", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "SYSTEM_ERROR"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-微信支付订单号查询订单", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/pay/partner/transactions/id/{transaction_id}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Path 参数"},
				Fields:  []Field{{Name: "transaction_id"}},
			},
			{
				Heading: "Query 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Query 参数"},
				Fields:  []Field{{Name: "sp_mchid"}, {Name: "sub_mchid"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "应答参数"},
				Fields:  []Field{{Name: "sp_appid"}, {Name: "sp_mchid"}, {Name: "sub_mchid"}, {Name: "out_trade_no"}, {Name: "transaction_id"}, {Name: "trade_type"}, {Name: "trade_state", EnumValues: []string{"SUCCESS", "USERPAYING"}}, {Name: "trade_state_desc"}},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"支付下单组", "普通支付-微信支付订单号查询订单", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "ORDER_NOT_EXIST"}, {Code: "SYSTEM_ERROR"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-关闭订单", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"支付下单组", "普通支付-关闭订单", "Path 参数"},
				Fields:  []Field{{Name: "out_trade_no"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "普通支付-关闭订单", "Body 参数"},
				Fields:  []Field{{Name: "sp_mchid"}, {Name: "sub_mchid"}},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"支付下单组", "普通支付-关闭订单", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "SYSTEM_ERROR"}, {Code: "NEW_CLOSE_CODE"}},
			},
		},
	}

	report := AuditOrderingAlignment(extraction)
	require.Equal(t, "ordering", report.Scope)
	require.Equal(t, 3, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 3, report.Summary.AuditedEndpointCount)
	require.Len(t, report.Endpoints, 1)
	require.Empty(t, report.SuppressedGaps)
	require.Len(t, report.CompatibilityGaps, 2)

	closeAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close")
	require.NotNil(t, closeAudit)
	require.Equal(t, []string{"NEW_CLOSE_CODE"}, closeAudit.MissingErrorCodes)
	closeCompatibility := findCompatibilityAudit(report.CompatibilityGaps, "POST", "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close")
	require.NotNil(t, closeCompatibility)
	require.Equal(t, []string{"BANK_ERROR", "ORDER_CLOSED", "ORDER_NOT_EXIST", "USERPAYING"}, closeCompatibility.CompatibilityErrorCodes)
	require.Equal(t, 1, report.Summary.MissingErrorCodeCount)
	require.Equal(t, 2, report.Summary.CompatibilityEndpointCount)
	require.Equal(t, 5, report.Summary.CompatibilityErrorCodeCount)
	require.Equal(t, 0, report.Summary.MissingRequestEnumCount)
	require.Equal(t, 0, report.Summary.SuppressedRequestEnumCount)
}

func TestAuditOrderingAlignment_SeparatesCompatibilityCodes(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-微信支付订单号查询订单", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/pay/partner/transactions/id/{transaction_id}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Path 参数"},
				Fields:  []Field{{Name: "transaction_id"}},
			},
			{
				Heading: "Query 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Query 参数"},
				Fields:  []Field{{Name: "sp_mchid"}, {Name: "sub_mchid"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "应答参数"},
				Fields: []Field{
					{Name: "sp_appid"},
					{Name: "sp_mchid"},
					{Name: "sub_appid"},
					{Name: "sub_mchid"},
					{Name: "out_trade_no"},
					{Name: "transaction_id"},
					{Name: "trade_type", EnumValues: []string{"JSAPI", "NATIVE", "APP", "MICROPAY", "MWEB", "FACEPAY"}},
					{Name: "trade_state", EnumValues: []string{"SUCCESS", "REFUND", "NOTPAY", "CLOSED", "REVOKED", "USERPAYING", "PAYERROR"}},
					{Name: "trade_state_desc"},
					{Name: "bank_type"},
					{Name: "attach"},
					{Name: "success_time", Description: "支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "payer"},
					{Name: "payer.sp_openid"},
					{Name: "payer.sub_openid"},
					{Name: "amount"},
					{Name: "amount.total", Description: "总金额，单位为分"},
					{Name: "amount.payer_total", Description: "用户支付金额，单位为分"},
					{Name: "amount.currency", EnumValues: []string{"CNY"}},
					{Name: "amount.payer_currency", EnumValues: []string{"CNY"}},
					{Name: "scene_info"},
					{Name: "scene_info.device_id"},
					{Name: "promotion_detail"},
					{Name: "promotion_detail.coupon_id"},
					{Name: "promotion_detail.name"},
					{Name: "promotion_detail.scope", EnumValues: []string{"GLOBAL", "SINGLE"}},
					{Name: "promotion_detail.type", EnumValues: []string{"CASH", "NOCASH"}},
					{Name: "promotion_detail.amount", Description: "优惠券面额，单位为分"},
					{Name: "promotion_detail.stock_id"},
					{Name: "promotion_detail.wechatpay_contribute", Description: "微信出资，单位为分"},
					{Name: "promotion_detail.merchant_contribute", Description: "商户出资，单位为分"},
					{Name: "promotion_detail.other_contribute", Description: "其他出资，单位为分"},
					{Name: "promotion_detail.currency", EnumValues: []string{"CNY"}},
					{Name: "promotion_detail.goods_detail"},
					{Name: "promotion_detail.goods_detail.goods_id"},
					{Name: "promotion_detail.goods_detail.quantity"},
					{Name: "promotion_detail.goods_detail.unit_price", Description: "商品单价，单位为分"},
					{Name: "promotion_detail.goods_detail.discount_amount", Description: "商品优惠金额，单位为分"},
					{Name: "promotion_detail.goods_detail.goods_remark"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"支付下单组", "普通支付-微信支付订单号查询订单", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "MCH_NOT_EXISTS"}, {Code: "SIGN_ERROR"}, {Code: "TRADE_ERROR"}, {Code: "RULE_LIMIT"}, {Code: "FREQUENCY_LIMITED"}, {Code: "SYSTEM_ERROR"}, {Code: "ORDER_NOT_EXIST"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-关闭订单", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"支付下单组", "普通支付-关闭订单", "Path 参数"},
				Fields:  []Field{{Name: "out_trade_no"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "普通支付-关闭订单", "Body 参数"},
				Fields:  []Field{{Name: "sp_mchid"}, {Name: "sub_mchid"}},
			},
			{
				Heading:    "错误码",
				Path:       []string{"支付下单组", "普通支付-关闭订单", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "MCH_NOT_EXISTS"}, {Code: "SIGN_ERROR"}, {Code: "RULE_LIMIT"}, {Code: "TRADE_ERROR"}, {Code: "FREQUENCY_LIMITED"}, {Code: "SYSTEM_ERROR"}},
			},
		},
	}

	report := AuditOrderingAlignment(extraction)
	require.Empty(t, report.Endpoints)
	require.Empty(t, report.SuppressedGaps)
	require.Len(t, report.CompatibilityGaps, 2)
	require.Equal(t, 0, report.Summary.MissingErrorCodeCount)
	require.Equal(t, 2, report.Summary.CompatibilityEndpointCount)
	require.Equal(t, 5, report.Summary.CompatibilityErrorCodeCount)

	queryCompatibility := findCompatibilityAudit(report.CompatibilityGaps, "GET", "/v3/pay/partner/transactions/id/{transaction_id}")
	require.NotNil(t, queryCompatibility)
	require.Equal(t, []string{"BANK_ERROR"}, queryCompatibility.CompatibilityErrorCodes)

	closeCompatibility := findCompatibilityAudit(report.CompatibilityGaps, "POST", "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close")
	require.NotNil(t, closeCompatibility)
	require.Equal(t, []string{"BANK_ERROR", "ORDER_CLOSED", "ORDER_NOT_EXIST", "USERPAYING"}, closeCompatibility.CompatibilityErrorCodes)
}

func TestAuditOrderingAlignment_ReportsMissingSemanticConstraints(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-小程序下单", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/pay/partner/transactions/jsapi"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "普通支付-小程序下单", "Body 参数"},
				Fields: []Field{
					{Name: "time_expire", Description: "支付结束时间"},
					{Name: "amount.total", Description: "总金额"},
				},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-微信支付订单号查询订单", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/pay/partner/transactions/id/{transaction_id}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Path 参数"},
				Fields:  []Field{{Name: "transaction_id"}},
			},
			{
				Heading: "Query 参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "Query 参数"},
				Fields:  []Field{{Name: "sp_mchid"}, {Name: "sub_mchid"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"支付下单组", "普通支付-微信支付订单号查询订单", "应答参数"},
				Fields: []Field{
					{Name: "success_time", Description: "支付完成时间"},
					{Name: "amount.total", Description: "总金额"},
				},
			},
		},
	}

	report := AuditOrderingAlignment(extraction)
	require.Equal(t, 2, report.Summary.MissingRequestConstraintCount)
	require.Equal(t, 2, report.Summary.MissingResponseConstraintCount)

	createAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/pay/partner/transactions/jsapi")
	require.NotNil(t, createAudit)
	require.Equal(t, []FieldConstraintAudit{
		{Field: "amount.total", MissingConstraints: []string{"unit_fen"}},
		{Field: "time_expire", MissingConstraints: []string{"format_rfc3339"}},
	}, createAudit.MissingRequestConstraints)

	queryAudit := findEndpointAudit(report.Endpoints, "GET", "/v3/pay/partner/transactions/id/{transaction_id}")
	require.NotNil(t, queryAudit)
	require.Equal(t, []FieldConstraintAudit{
		{Field: "amount.total", MissingConstraints: []string{"unit_fen"}},
		{Field: "success_time", MissingConstraints: []string{"format_rfc3339"}},
	}, queryAudit.MissingResponseConstraints)
}

func TestAuditOrderingAlignment_CoversNotificationWebhooks(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "普通支付-支付结果通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-ecommerce/payment-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "普通支付-支付结果通知", "Body 参数"},
				Fields: []Field{
					{Name: "sp_appid"},
					{Name: "sp_mchid"},
					{Name: "sub_mchid"},
					{Name: "out_trade_no"},
					{Name: "transaction_id"},
					{Name: "trade_type", EnumValues: []string{"JSAPI", "NATIVE", "APP", "MICROPAY", "MWEB", "FACEPAY"}},
					{Name: "trade_state", EnumValues: []string{"SUCCESS", "REFUND", "NOTPAY", "CLOSED", "REVOKED", "USERPAYING", "PAYERROR"}},
					{Name: "trade_state_desc"},
					{Name: "bank_type"},
					{Name: "success_time", Description: "支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "payer"},
					{Name: "payer.sp_openid"},
					{Name: "payer.sub_openid"},
					{Name: "amount"},
					{Name: "amount.total", Description: "总金额，单位为分"},
					{Name: "amount.payer_total", Description: "用户支付金额，单位为分"},
					{Name: "amount.currency", EnumValues: []string{"CNY"}},
					{Name: "amount.payer_currency", EnumValues: []string{"CNY"}},
					{Name: "promotion_detail"},
					{Name: "promotion_detail.coupon_id"},
					{Name: "promotion_detail.scope", EnumValues: []string{"GLOBAL", "SINGLE"}},
					{Name: "promotion_detail.type", EnumValues: []string{"CASH", "NOCASH"}},
					{Name: "promotion_detail.amount", Description: "优惠券面额，单位为分"},
					{Name: "promotion_detail.currency", EnumValues: []string{"CNY"}},
					{Name: "promotion_detail.goods_detail"},
					{Name: "promotion_detail.goods_detail.goods_id"},
					{Name: "promotion_detail.goods_detail.quantity"},
					{Name: "promotion_detail.goods_detail.unit_price", Description: "商品单价，单位为分"},
					{Name: "promotion_detail.goods_detail.discount_amount", Description: "商品优惠金额，单位为分"},
				},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "合单支付-合单支付通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-ecommerce/combine-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"支付下单组", "合单支付-合单支付通知", "Body 参数"},
				Fields: []Field{
					{Name: "combine_appid"},
					{Name: "combine_mchid"},
					{Name: "combine_out_trade_no"},
					{Name: "sub_orders"},
					{Name: "sub_orders.mchid"},
					{Name: "sub_orders.trade_type", EnumValues: []string{"NATIVE", "JSAPI", "APP", "MWEB"}},
					{Name: "sub_orders.trade_state", EnumValues: []string{"SUCCESS", "REFUND", "NOTPAY", "CLOSED", "PAYERROR"}},
					{Name: "sub_orders.bank_type"},
					{Name: "sub_orders.success_time", Description: "支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss.sss+TIMEZONE"},
					{Name: "sub_orders.transaction_id"},
					{Name: "sub_orders.out_trade_no"},
					{Name: "sub_orders.amount"},
					{Name: "sub_orders.amount.total_amount", Description: "商品单金额，单位为分"},
					{Name: "sub_orders.amount.currency", EnumValues: []string{"CNY"}},
					{Name: "sub_orders.amount.payer_amount", Description: "用户支付金额，单位为分"},
					{Name: "sub_orders.amount.payer_currency", EnumValues: []string{"CNY"}},
					{Name: "sub_orders.promotion_detail"},
					{Name: "sub_orders.promotion_detail.coupon_id"},
					{Name: "sub_orders.promotion_detail.scope", EnumValues: []string{"GLOBAL", "SINGLE"}},
					{Name: "sub_orders.promotion_detail.type", EnumValues: []string{"CASH", "NOCASH"}},
					{Name: "sub_orders.promotion_detail.amount", Description: "优惠券面额，单位为分"},
					{Name: "sub_orders.promotion_detail.currency", EnumValues: []string{"CNY"}},
					{Name: "sub_orders.promotion_detail.goods_detail"},
					{Name: "sub_orders.promotion_detail.goods_detail.goods_id"},
					{Name: "sub_orders.promotion_detail.goods_detail.quantity"},
					{Name: "sub_orders.promotion_detail.goods_detail.unit_price", Description: "商品单价，单位为分"},
					{Name: "sub_orders.promotion_detail.goods_detail.discount_amount", Description: "商品优惠金额，单位为分"},
					{Name: "combine_payer_info"},
					{Name: "combine_payer_info.openid"},
				},
			},
		},
	}

	report := AuditOrderingAlignment(extraction)
	require.Equal(t, 2, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 2, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
	require.Empty(t, report.CompatibilityGaps)
}

func TestAuditOrderingAlignment_ReportsUnknownEndpoint(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"支付下单组", "未知接口", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/pay/unknown/endpoint"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"支付下单组", "未知接口", "请求参数"},
				Fields:  []Field{{Name: "foo"}},
			},
		},
	}

	report := AuditOrderingAlignment(extraction)
	require.Len(t, report.Endpoints, 1)
	require.True(t, report.Endpoints[0].MissingEndpoint)
	require.Equal(t, []string{"foo"}, report.Endpoints[0].MissingRequestFields)
}

func findCompatibilityAudit(endpoints []EndpointCompatibilityAudit, method, path string) *EndpointCompatibilityAudit {
	for index := range endpoints {
		endpoint := &endpoints[index]
		if endpoint.Method == method && endpoint.Path == path {
			return endpoint
		}
	}
	return nil
}
