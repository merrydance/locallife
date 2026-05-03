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
	require.NotContains(t, string(body), "sharingMerId")
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
