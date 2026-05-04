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

func TestNormalizePaymentTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizePaymentTerminalStatus("WAIT_PAYING"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizePaymentTerminalStatus("SUCCESS"))
	require.Equal(t, db.ExternalPaymentTerminalStatusClosed, NormalizePaymentTerminalStatus("CLOSED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizePaymentTerminalStatus("PAY_ERROR"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizePaymentTerminalStatus("REFUND"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizePaymentTerminalStatus("ABNORMAL"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizePaymentTerminalStatus("unexpected"))
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
	require.NotContains(t, string(body), "openid")
	require.NotContains(t, string(body), "sub_openid")
}

func TestNormalizeShareTerminalStatus(t *testing.T) {
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, NormalizeShareTerminalStatus("PROCESSING"))
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, NormalizeShareTerminalStatus("SUCCESS"))
	require.Equal(t, db.ExternalPaymentTerminalStatusFailed, NormalizeShareTerminalStatus("CANCELED"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeShareTerminalStatus("ABNORMAL"))
	require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, NormalizeShareTerminalStatus("unexpected"))
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
