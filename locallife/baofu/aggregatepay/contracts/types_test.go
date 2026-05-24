package contracts

import (
	"encoding/json"
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestUnifiedOrderRequestForWechatJSAPIUsesBaoCaiTongSharingFields(t *testing.T) {
	req := NewWechatJSAPISharingUnifiedOrderRequest(UnifiedOrderInput{
		OutTradeNo: "BF202605030001",
		AmountFen:  12800,
		SubMchID:   "1900000109",
		SubAppID:   "wx1234567890abcdef",
		SubOpenID:  "openid-payer-001",
		Body:       "本地生活订单",
		NotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		ClientIP:   "203.0.113.1",
		TimeExpire: 30,
		Attach:     "order:1001",
		TxnTime:    "20260503120000",
		MerchantID: "102004465",
		TerminalID: "200005200",
	})

	require.Equal(t, ProductTypeSharing, req.ProductType)
	require.Equal(t, BaoCaiTongOrderTypeSharing, req.OrderType)
	require.Equal(t, PayCodeWechatJSAPI, req.PayCode)
	require.Equal(t, "1900000109", req.SubMchID)
	require.Equal(t, "wx1234567890abcdef", req.PayExtend.SubAppID)
	require.Equal(t, "openid-payer-001", req.PayExtend.SubOpenID)

	body, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(body), `"prodType":"SHARING"`)
	require.Contains(t, string(body), `"orderType":"7"`)
	require.Contains(t, string(body), `"payCode":"WECHAT_JSAPI"`)
	require.Contains(t, string(body), `"sub_openid":"openid-payer-001"`)
	require.Contains(t, string(body), `"riskInfo":{"clientIp":"203.0.113.1"}`)
	require.NotContains(t, string(body), "sharingMerId")
}

func TestUnifiedOrderRequestForWechatJSAPIRequiresRiskInfoClientIP(t *testing.T) {
	req := NewWechatJSAPISharingUnifiedOrderRequest(UnifiedOrderInput{
		OutTradeNo: "BF202605030002",
		AmountFen:  12800,
		SubMchID:   "1900000109",
		SubAppID:   "wx1234567890abcdef",
		SubOpenID:  "openid-payer-001",
		Body:       "本地生活订单",
		NotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		TimeExpire: 30,
		TxnTime:    "20260503120000",
		MerchantID: "102004465",
		TerminalID: "200005200",
	})

	require.ErrorIs(t, req.Validate(), ErrUnifiedOrderRiskInfoClientIPRequired)
}

func TestUnifiedOrderRequestAllowsMissingSubMchIDOnlyInSandbox(t *testing.T) {
	req := validWechatUnifiedOrderRequestForTest()
	req.SubMchID = ""

	require.ErrorIs(t, req.Validate(), ErrUnifiedOrderSubMchIDRequired)
	require.ErrorIs(t, req.ValidateForEnvironment("production"), ErrUnifiedOrderSubMchIDRequired)
	require.NoError(t, req.ValidateForEnvironment("sandbox"))
}

func TestUnifiedOrderRequestValidateOfficialRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*UnifiedOrderRequest)
		want   error
	}{
		{"missing merchant", func(r *UnifiedOrderRequest) { r.MerchantID = "" }, ErrUnifiedOrderMerchantIDRequired},
		{"missing terminal", func(r *UnifiedOrderRequest) { r.TerminalID = "" }, ErrUnifiedOrderTerminalIDRequired},
		{"missing out trade no", func(r *UnifiedOrderRequest) { r.OutTradeNo = "" }, ErrUnifiedOrderOutTradeNoRequired},
		{"missing txn amount", func(r *UnifiedOrderRequest) { r.TransactionAmt = 0 }, ErrUnifiedOrderAmountInvalid},
		{"total less than txn", func(r *UnifiedOrderRequest) { r.TotalAmt = r.TransactionAmt - 1 }, ErrUnifiedOrderTotalAmountInvalid},
		{"missing txn time", func(r *UnifiedOrderRequest) { r.TransactionTime = "" }, ErrUnifiedOrderTransactionTimeInvalid},
		{"invalid txn time", func(r *UnifiedOrderRequest) { r.TransactionTime = "2026-05-04T12:00:00" }, ErrUnifiedOrderTransactionTimeInvalid},
		{"invalid product", func(r *UnifiedOrderRequest) { r.ProductType = "PAYMENT" }, ErrUnifiedOrderProductTypeUnsupported},
		{"invalid order type", func(r *UnifiedOrderRequest) { r.OrderType = "1" }, ErrUnifiedOrderOrderTypeUnsupported},
		{"invalid pay code", func(r *UnifiedOrderRequest) { r.PayCode = "WECHAT_NATIVE" }, ErrUnifiedOrderPayCodeUnsupported},
		{"missing sub mch for wechat", func(r *UnifiedOrderRequest) { r.SubMchID = "" }, ErrUnifiedOrderSubMchIDRequired},
		{"missing sub appid", func(r *UnifiedOrderRequest) { r.PayExtend.SubAppID = "" }, ErrUnifiedOrderPayExtendSubAppIDRequired},
		{"missing sub openid", func(r *UnifiedOrderRequest) { r.PayExtend.SubOpenID = "" }, ErrUnifiedOrderPayExtendSubOpenIDRequired},
		{"missing body", func(r *UnifiedOrderRequest) { r.PayExtend.Body = "" }, ErrUnifiedOrderPayExtendBodyRequired},
		{"missing risk client ip", func(r *UnifiedOrderRequest) { r.RiskInfo.ClientIP = "" }, ErrUnifiedOrderRiskInfoClientIPRequired},
		{"non https page url", func(r *UnifiedOrderRequest) { r.PageURL = "http://example.com/pay" }, ErrUnifiedOrderPageURLInvalid},
		{"invalid forbid credit", func(r *UnifiedOrderRequest) { r.ForbidCredit = "2" }, ErrUnifiedOrderForbidCreditUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validWechatUnifiedOrderRequestForTest()
			tc.mutate(&req)
			require.ErrorIs(t, req.Validate(), tc.want)
		})
	}
}

func TestUnifiedOrderRequestValidateOfficialLengthsAndConditionalNestedFields(t *testing.T) {
	t.Run("outTradeNo S32", func(t *testing.T) {
		req := validWechatUnifiedOrderRequestForTest()
		req.OutTradeNo = stringOfLen("O", 33)
		require.EqualError(t, req.Validate(), "baofu unified order outTradeNo must be at most 32 characters")
	})

	t.Run("timeExpire max seven days in minutes", func(t *testing.T) {
		req := validWechatUnifiedOrderRequestForTest()
		req.TimeExpire = 7*24*60 + 1
		require.EqualError(t, req.Validate(), "baofu unified order timeExpire must be at most 10080 minutes")
	})

	t.Run("marketing merchant required when mktInfo present", func(t *testing.T) {
		req := validWechatUnifiedOrderRequestForTest()
		req.MarketingInfo = &MarketingInfo{AmountFen: 100}
		require.EqualError(t, req.Validate(), "baofu unified order mktInfo.mktMerId is required")
	})

	t.Run("marketing amount positive when mktInfo present", func(t *testing.T) {
		req := validWechatUnifiedOrderRequestForTest()
		req.MarketingInfo = &MarketingInfo{MerchantID: "102004465"}
		require.EqualError(t, req.Validate(), "baofu unified order mktInfo.mktAmt must be positive")
	})
}

func validWechatUnifiedOrderRequestForTest() UnifiedOrderRequest {
	req := NewWechatJSAPISharingUnifiedOrderRequest(UnifiedOrderInput{
		OutTradeNo: "BF202605030099",
		AmountFen:  12800,
		SubMchID:   "1900000109",
		SubAppID:   "wx1234567890abcdef",
		SubOpenID:  "openid-payer-099",
		Body:       "本地生活订单",
		NotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		PageURL:    "https://example.com/pay",
		ClientIP:   "203.0.113.99",
		TimeExpire: 30,
		TxnTime:    "20260503120000",
		MerchantID: "102004465",
		TerminalID: "200005200",
	})
	req.ForbidCredit = "0"
	return req
}

func TestUnifiedOrderResultExtractsWechatPayData(t *testing.T) {
	result := UnifiedOrderResult{
		ResultCode: "SUCCESS",
		TradeNo:    "BFTRADE123",
		TxnState:   "WAIT_PAYING",
		ChannelReturn: ChannelReturn{
			PrepayID:      "wx-prepay-id",
			WechatPayData: json.RawMessage(`{"appId":"wx123","timeStamp":"1710000000","nonceStr":"nonce","package":"prepay_id=wx-prepay-id","signType":"RSA","paySign":"sign"}`),
			OrderID:       "1188000078909",
		},
	}

	payData, err := result.WechatPayData()

	require.NoError(t, err)
	require.JSONEq(t, `{"appId":"wx123","timeStamp":"1710000000","nonceStr":"nonce","package":"prepay_id=wx-prepay-id","signType":"RSA","paySign":"sign"}`, string(payData))
}

func TestUnifiedOrderResultExtractsStringWrappedWechatPayData(t *testing.T) {
	var result UnifiedOrderResult

	err := json.Unmarshal([]byte(`{
		"resultCode":"SUCCESS",
		"chlRetParam":{
			"wc_pay_data":"{\"timeStamp\":\"1767225600\",\"nonceStr\":\"nonce\",\"package\":\"prepay_id=wx-prepay-id\",\"signType\":\"RSA\",\"paySign\":\"sign\"}"
		}
	}`), &result)
	require.NoError(t, err)

	payData, err := result.WechatPayData()

	require.NoError(t, err)
	require.JSONEq(t, `{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx-prepay-id","signType":"RSA","paySign":"sign"}`, string(payData))
}

func TestUnifiedOrderResultExtractsDoubleStringWrappedWechatPayData(t *testing.T) {
	var result UnifiedOrderResult

	err := json.Unmarshal([]byte(`{
		"resultCode":"SUCCESS",
		"chlRetParam":{
			"wc_pay_data":"\"{\\\"timeStamp\\\":\\\"1767225600\\\",\\\"nonceStr\\\":\\\"nonce\\\",\\\"package\\\":\\\"prepay_id=wx-prepay-id\\\",\\\"signType\\\":\\\"RSA\\\",\\\"paySign\\\":\\\"sign\\\"}\""
		}
	}`), &result)
	require.NoError(t, err)

	payData, err := result.WechatPayData()

	require.NoError(t, err)
	require.JSONEq(t, `{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx-prepay-id","signType":"RSA","paySign":"sign"}`, string(payData))
}

func TestUnifiedOrderResultAcceptsNumericChannelOrderID(t *testing.T) {
	var result UnifiedOrderResult

	err := json.Unmarshal([]byte(`{"resultCode":"SUCCESS","chlRetParam":{"order_id":1234567890}}`), &result)

	require.NoError(t, err)
	require.Equal(t, "1234567890", result.ChannelReturn.OrderID)
}

func TestUnifiedOrderResultCoversOfficialOrderQueryFields(t *testing.T) {
	var result UnifiedOrderResult

	err := json.Unmarshal([]byte(`{
		"agentMerId":"agent-merchant",
		"agentTerId":"agent-terminal",
		"merId":"102004465",
		"terId":"200005200",
		"outTradeNo":"PO202605050001",
		"tradeNo":"260500000001",
		"txnState":"SUCCESS",
		"finishTime":"20260505120000",
		"succAmt":100,
		"feeAmt":1,
		"instFeeAmt":2,
		"reqChlNo":"REQCHL001",
		"payCode":"WECHAT_JSAPI",
		"chlRetParam":{"openId":"openid-001","subOpenid":"sub-openid-001","order_id":"260500000001"},
		"clearingDate":"20260505",
		"resultCode":"SUCCESS"
	}`), &result)

	require.NoError(t, err)
	require.Equal(t, "agent-merchant", result.AgentMerchantID)
	require.Equal(t, "agent-terminal", result.AgentTerminalID)
	require.Equal(t, "20260505120000", result.FinishTime)
	require.Equal(t, int64(100), result.SuccessAmountFen)
	require.Equal(t, int64(1), result.FeeAmountFen)
	require.Equal(t, int64(2), result.InstallmentFeeAmountFen)
	require.Equal(t, "20260505", result.ClearingDate)
	require.Equal(t, "openid-001", result.ChannelReturn.OpenID)
	require.Equal(t, "sub-openid-001", result.ChannelReturn.SubOpenID)
}

func TestUnifiedOrderResultValidatesMethodSpecificResponses(t *testing.T) {
	unified := UnifiedOrderResult{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001", ResultCode: BusinessResultCodeSuccess, PayCode: PayCodeWechatJSAPI, TxnState: PaymentStateWaitPaying}
	require.NoError(t, unified.ValidateUnifiedOrderResponse())

	unified.PayCode = "WECHAT_NATIVE"
	require.EqualError(t, unified.ValidateUnifiedOrderResponse(), "baofu unified order response payCode is unsupported")

	unified = UnifiedOrderResult{MerchantID: "102004465", TerminalID: "200005200", ResultCode: BusinessResultCodeSuccess, TxnState: PaymentStateSuccess, PayCode: PayCodeWechatJSAPI}
	require.NoError(t, unified.ValidateOrderQueryResponse())

	unified.TxnState = "UNKNOWN"
	require.EqualError(t, unified.ValidateOrderQueryResponse(), "baofu order query response txnState is unsupported")
}

func TestAggregateResultsValidateOfficialDateFormats(t *testing.T) {
	payment := UnifiedOrderResult{MerchantID: "102004465", TerminalID: "200005200", ResultCode: BusinessResultCodeSuccess, TxnState: PaymentStateSuccess, PayCode: PayCodeWechatJSAPI, FinishTime: "2026-05-05T12:00:00"}
	require.EqualError(t, payment.ValidateOrderQueryResponse(), "baofu order query response finishTime must use yyyyMMddHHmmss")

	payment.FinishTime = "20260505120000"
	payment.ClearingDate = "2026-05-05"
	require.EqualError(t, payment.ValidateOrderQueryResponse(), "baofu order query response clearingDate must use yyyyMMdd")

	share := ShareResult{MerchantID: "102004465", TerminalID: "200005200", ResultCode: BusinessResultCodeSuccess, TxnState: ShareStateSuccess, FinishTime: "2026-05-05 12:00:00"}
	require.EqualError(t, share.ValidateShareQueryResponse(), "baofu share response finishTime must use yyyyMMddHHmmss")

	share.FinishTime = "20260505120000"
	share.ClearingDate = "2026/05/05"
	require.EqualError(t, share.ValidateShareQueryResponse(), "baofu share response clearingDate must use yyyyMMdd")

	refund := RefundResult{ResultCode: BusinessResultCodeSuccess, RefundState: RefundStateSuccess, FinishTime: "2026-05-05T12:00:00Z"}
	require.EqualError(t, refund.ValidateRefundQueryResponse(), "baofu refund query response finishTime must use yyyyMMddHHmmss")
}

func TestNormalizePaymentTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizePaymentTerminalStatus("WAIT_PAYING"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizePaymentTerminalStatus("SUCCESS"))
	require.Equal(t, db.ExternalPaymentTerminalStatusClosed, NormalizePaymentTerminalStatus("CLOSED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizePaymentTerminalStatus("PAY_ERROR"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizePaymentTerminalStatus("REFUND"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizePaymentTerminalStatus("ABNORMAL"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizePaymentTerminalStatus("unexpected"))
}

func TestPaymentQueryRequestValidateOfficialRequiredFields(t *testing.T) {
	req := PaymentQueryRequest{AgentMerchantID: "agent-merchant", AgentTerminalID: "agent-terminal", MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001"}
	require.NoError(t, req.Validate())
	body, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(body), `"agentMerId":"agent-merchant"`)
	require.Contains(t, string(body), `"agentTerId":"agent-terminal"`)

	req = PaymentQueryRequest{TerminalID: "200005200", OutTradeNo: "BF202605040001"}
	require.ErrorIs(t, req.Validate(), ErrBaofuPaymentQueryMerchantIDRequired)

	req = PaymentQueryRequest{MerchantID: "102004465", OutTradeNo: "BF202605040001"}
	require.ErrorIs(t, req.Validate(), ErrBaofuPaymentQueryTerminalIDRequired)

	req = PaymentQueryRequest{MerchantID: "102004465", TerminalID: "200005200"}
	require.ErrorIs(t, req.Validate(), ErrBaofuPaymentQueryReferenceRequired)
}

func TestShareAfterPayRequestRequiresPaymentReferenceAndReceiverIDs(t *testing.T) {
	req := ShareAfterPayRequest{
		MerchantID:        "102004465",
		TerminalID:        "200005200",
		OriginOutTradeNo:  "BF202605030001",
		OutTradeNo:        "BFSHARE202605030001",
		TxnTime:           "20260503120500",
		NotifyURL:         "https://api.example.com/v1/webhooks/baofu/share",
		SharingDetails:    []SharingDetail{{SharingMerID: "CP_MERCHANT", SharingAmountFen: 12000}},
		OriginTradeNo:     "",
		AgentMerchantID:   "",
		AgentTerminalID:   "",
		RequestReserved:   "",
		BusinessReserved:  "",
		MarketingReserved: "",
	}

	require.NoError(t, req.Validate())

	body, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(body), `"originOutTradeNo":"BF202605030001"`)
	require.Contains(t, string(body), `"sharingMerId":"CP_MERCHANT"`)
	require.NotContains(t, string(body), "subMchId")
	require.NotContains(t, string(body), "openid")
	require.NotContains(t, string(body), "sub_openid")

	req.TxnTime = "2026-05-03"
	require.ErrorIs(t, req.Validate(), ErrBaofuShareTransactionTimeInvalid)
}

func TestShareAfterPayRequestValidateOfficialLengths(t *testing.T) {
	req := ShareAfterPayRequest{
		MerchantID:       "102004465",
		TerminalID:       "200005200",
		OriginOutTradeNo: "BF202605030001",
		OutTradeNo:       "BFSHARE202605030001",
		TxnTime:          "20260503120500",
		SharingDetails:   []SharingDetail{{SharingMerID: "CP_MERCHANT", SharingAmountFen: 12000}},
	}
	req.OutTradeNo = stringOfLen("S", 51)

	require.EqualError(t, req.Validate(), "baofu share outTradeNo must be at most 50 characters")
}

func TestShareQueryRequestValidateOfficialRequiredFields(t *testing.T) {
	req := ShareQueryRequest{AgentMerchantID: "agent-merchant", AgentTerminalID: "agent-terminal", MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BFSHARE202605040001"}
	require.NoError(t, req.Validate())
	body, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(body), `"agentMerId":"agent-merchant"`)
	require.Contains(t, string(body), `"agentTerId":"agent-terminal"`)

	req = ShareQueryRequest{TerminalID: "200005200", OutTradeNo: "BFSHARE202605040001"}
	require.ErrorIs(t, req.Validate(), ErrBaofuShareQueryMerchantIDRequired)

	req = ShareQueryRequest{MerchantID: "102004465", OutTradeNo: "BFSHARE202605040001"}
	require.ErrorIs(t, req.Validate(), ErrBaofuShareQueryTerminalIDRequired)

	req = ShareQueryRequest{MerchantID: "102004465", TerminalID: "200005200"}
	require.ErrorIs(t, req.Validate(), ErrBaofuShareQueryReferenceRequired)
}

func TestNormalizeShareTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizeShareTerminalStatus("PROCESSING"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizeShareTerminalStatus("SUCCESS"))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizeShareTerminalStatus("CANCELED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeShareTerminalStatus("ABNORMAL"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeShareTerminalStatus("unexpected"))
}

func TestShareResultCoversOfficialAgentFields(t *testing.T) {
	var result ShareResult

	err := json.Unmarshal([]byte(`{"agentMerId":"agent-merchant","agentTerId":"agent-terminal","merId":"102004465","terId":"200005200","resultCode":"SUCCESS","tradeNo":"260500000001","outTradeNo":"SH202605050001","txnState":"SUCCESS","succAmt":100,"clearingDate":"20260505"}`), &result)

	require.NoError(t, err)
	require.Equal(t, "agent-merchant", result.AgentMerchantID)
	require.Equal(t, "agent-terminal", result.AgentTerminalID)
	require.Equal(t, "SH202605050001", result.OutTradeNo)
	require.Equal(t, int64(100), result.SuccessAmountFen)
}

func TestShareResultAcceptsStringSuccessAmount(t *testing.T) {
	var result ShareResult

	err := json.Unmarshal([]byte(`{"agentMerId":"agent-merchant","agentTerId":"agent-terminal","merId":"102004465","terId":"200005200","resultCode":"SUCCESS","tradeNo":"260500000001","outTradeNo":"SH202605050001","txnState":"SUCCESS","succAmt":"100","clearingDate":"20260505"}`), &result)

	require.NoError(t, err)
	require.Equal(t, int64(100), result.SuccessAmountFen)
	require.NoError(t, result.ValidateShareQueryResponse())
}

func TestShareResultRejectsFractionalStringSuccessAmount(t *testing.T) {
	var result ShareResult

	err := json.Unmarshal([]byte(`{"merId":"102004465","terId":"200005200","resultCode":"SUCCESS","tradeNo":"260500000001","outTradeNo":"SH202605050001","txnState":"SUCCESS","succAmt":"100.5"}`), &result)

	require.EqualError(t, err, "baofu share response succAmt must be an integer")
}

func TestShareResultValidatesMethodSpecificResponses(t *testing.T) {
	result := ShareResult{MerchantID: "102004465", TerminalID: "200005200", ResultCode: BusinessResultCodeSuccess, TxnState: ShareStateProcessing}
	require.NoError(t, result.ValidateShareAfterPayResponse())
	require.NoError(t, result.ValidateShareQueryResponse())

	result.TxnState = ""
	require.EqualError(t, result.ValidateShareAfterPayResponse(), "baofu share response txnState is required")

	result.TxnState = "UNKNOWN"
	require.EqualError(t, result.ValidateShareQueryResponse(), "baofu share response txnState is unsupported")
}

func TestRefundBeforeShareRequestRejectsPostShareFields(t *testing.T) {
	req := validBaofuRefundBeforeShareRequestForTest()
	req.SharingRefundInfo = []SharingRefundDetail{{SharingMerID: "CM1", SharingAmountFen: 100}}
	require.ErrorIs(t, req.Validate(), ErrBaofuRefundSharingRefundInfoUnsupported)

	req = validBaofuRefundBeforeShareRequestForTest()
	req.AdvanceAmountFen = 100
	require.ErrorIs(t, req.Validate(), ErrBaofuRefundAdvanceUnsupported)
}

func TestRefundBeforeShareRequestRequiresOfficialFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RefundBeforeShareRequest)
		want   error
	}{
		{"missing merchant", func(r *RefundBeforeShareRequest) { r.MerchantID = "" }, ErrBaofuRefundMerchantIDRequired},
		{"missing terminal", func(r *RefundBeforeShareRequest) { r.TerminalID = "" }, ErrBaofuRefundTerminalIDRequired},
		{"missing origin ref", func(r *RefundBeforeShareRequest) { r.OriginTradeNo = ""; r.OriginOutTradeNo = "" }, ErrBaofuRefundOriginalReferenceRequired},
		{"missing out trade no", func(r *RefundBeforeShareRequest) { r.OutTradeNo = "" }, ErrBaofuRefundOutTradeNoRequired},
		{"invalid amount", func(r *RefundBeforeShareRequest) { r.RefundAmountFen = 0 }, ErrBaofuRefundAmountInvalid},
		{"total less", func(r *RefundBeforeShareRequest) { r.TotalAmountFen = r.RefundAmountFen - 1 }, ErrBaofuRefundTotalAmountInvalid},
		{"invalid txn time", func(r *RefundBeforeShareRequest) { r.TransactionTime = "2026-05-04" }, ErrBaofuRefundTransactionTimeInvalid},
		{"missing reason", func(r *RefundBeforeShareRequest) { r.RefundReason = "" }, ErrBaofuRefundReasonRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBaofuRefundBeforeShareRequestForTest()
			tc.mutate(&req)
			require.ErrorIs(t, req.Validate(), tc.want)
		})
	}
}

func TestRefundBeforeShareRequestValidateOfficialLengthsAndMarketingInfo(t *testing.T) {
	t.Run("outTradeNo S50", func(t *testing.T) {
		req := validBaofuRefundBeforeShareRequestForTest()
		req.OutTradeNo = stringOfLen("R", 51)
		require.EqualError(t, req.Validate(), "baofu refund outTradeNo must be at most 50 characters")
	})

	t.Run("marketing merchant required", func(t *testing.T) {
		req := validBaofuRefundBeforeShareRequestForTest()
		req.MarketingInfo = &MarketingRefundInfo{AmountFen: 100}
		require.EqualError(t, req.Validate(), "baofu refund mktRefundInfo.mktMerId is required")
	})

	t.Run("marketing amount positive", func(t *testing.T) {
		req := validBaofuRefundBeforeShareRequestForTest()
		req.MarketingInfo = &MarketingRefundInfo{MerchantID: "102004465"}
		require.EqualError(t, req.Validate(), "baofu refund mktRefundInfo.mktAmt must be positive")
	})
}

func TestOrderCloseRequiresOriginalPaymentReference(t *testing.T) {
	req := OrderCloseRequest{MerchantID: "102004465", TerminalID: "200005200"}
	require.ErrorIs(t, req.Validate(), ErrBaofuOrderCloseReferenceRequired)

	req.OutTradeNo = "BF202605040001"
	require.NoError(t, req.Validate())
}

func TestRefundQueryRequiresRefundReference(t *testing.T) {
	req := RefundQueryRequest{MerchantID: "102004465", TerminalID: "200005200"}
	require.ErrorIs(t, req.Validate(), ErrBaofuRefundQueryReferenceRequired)

	req.OutTradeNo = "RF202605040001"
	require.NoError(t, req.Validate())
}

func TestRefundAndCloseResultsValidateMethodSpecificResponses(t *testing.T) {
	refund := RefundResult{OutTradeNo: "RF202605040001", TradeNo: "BFREFUND202605040001", RefundAmountFen: 300, TotalAmountFen: 300, ResultCode: BusinessResultCodeSuccess, RefundState: RefundStateAccepted}
	require.NoError(t, refund.ValidateOrderRefundResponse())
	require.NoError(t, refund.ValidateRefundQueryResponse())

	refund.TradeNo = ""
	require.EqualError(t, refund.ValidateOrderRefundResponse(), "baofu refund response tradeNo is required")

	refund = RefundResult{ResultCode: BusinessResultCodeSuccess, RefundState: "UNKNOWN"}
	require.EqualError(t, refund.ValidateRefundQueryResponse(), "baofu refund query response refundState is unsupported")

	closeResult := OrderCloseResult{MerchantID: "102004465", TerminalID: "200005200", ResultCode: BusinessResultCodeSuccess}
	require.NoError(t, closeResult.ValidateOrderCloseResponse())

	closeResult.MerchantID = ""
	require.EqualError(t, closeResult.ValidateOrderCloseResponse(), "baofu order close response merId is required")
}

func TestNormalizeRefundTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizeRefundTerminalStatus(RefundStateSuccess))
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizeRefundTerminalStatus(RefundStateAccepted))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizeRefundTerminalStatus(RefundStateError))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeRefundTerminalStatus(RefundStateAbnormal))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeRefundTerminalStatus("unexpected"))
}

func validBaofuRefundBeforeShareRequestForTest() RefundBeforeShareRequest {
	return RefundBeforeShareRequest{
		MerchantID:       "102004465",
		TerminalID:       "200005200",
		OriginOutTradeNo: "BF202605040001",
		OutTradeNo:       "RF202605040001",
		NotifyURL:        "https://api.example.com/v1/webhooks/baofu/refund",
		RefundAmountFen:  300,
		TotalAmountFen:   300,
		TransactionTime:  "20260504120500",
		RefundReason:     "用户申请退款",
		RequestReserved:  "refund-order:9001",
	}
}

func stringOfLen(ch string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += ch
	}
	return out
}
