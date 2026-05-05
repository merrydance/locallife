package notification

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"testing"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	"github.com/stretchr/testify/require"
)

func TestParserParsePaymentNotificationNormalizesPaymentFact(t *testing.T) {
	body := []byte(`{
		"notifyId":"BFN202605030001",
		"notifyType":"PAYMENT.SUCCESS",
		"agentMerId":"AGENT_MER",
		"agentTerId":"AGENT_TER",
		"merId":"102004465",
		"terId":"200005200",
		"outTradeNo":"PO202605030001",
		"tradeNo":"BFPAY202605030001",
		"txnState":"SUCCESS",
		"finishTime":"20260503101500",
		"succAmt":12345,
		"feeAmt":37,
		"instFeeAmt":0,
		"resultCode":"SUCCESS",
		"reqChlNo":"REQCHL001",
		"payCode":"WECHAT_JSAPI",
		"chlRetParam":{"sub_openid":"payer-openid-must-not-be-receiver","bank_type":"OTHERS"},
		"clearingDate":"20260503",
		"sub_openid":"payer-openid-must-not-be-receiver",
		"occurredAt":"2026-05-03T10:15:00Z"
	}`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Equal(t, "BFN202605030001", notification.NotifyID)
	require.Equal(t, "PAYMENT.SUCCESS", notification.NotifyType)
	require.Equal(t, "PO202605030001", notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605030001", notification.Fact.TradeNo)
	require.Equal(t, "AGENT_MER", notification.Fact.AgentMerchantID)
	require.Equal(t, "102004465", notification.Fact.MerchantID)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, "20260503101500", notification.Fact.FinishTime)
	require.Equal(t, int64(12345), notification.Fact.SuccessAmountFen)
	require.Equal(t, int64(37), notification.Fact.FeeAmountFen)
	require.Equal(t, "REQCHL001", notification.Fact.RequestChannelNo)
	require.Equal(t, "WECHAT_JSAPI", notification.Fact.PayCode)
	require.JSONEq(t, `{"sub_openid":"payer-openid-must-not-be-receiver","bank_type":"OTHERS"}`, string(notification.Fact.ChannelReturnParam))
	require.Equal(t, "20260503", notification.Fact.ClearingDate)
	require.Equal(t, "success", notification.TerminalStatus)
	require.True(t, notification.IsTerminal)
	require.Equal(t, "2026-05-03T10:15:00Z", notification.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"))
	require.JSONEq(t, string(body), string(notification.Raw))
	require.NotContains(t, string(notification.Raw), "sharingMerId")
}

func TestParserParsePaymentNotificationAcceptsFormURLEncodedBody(t *testing.T) {
	body := []byte(`resultCode=SUCCESS&merId=102004465&terId=200005200&outTradeNo=PO202605050001&tradeNo=BFPAY202605050001&txnState=SUCCESS&succAmt=100&feeAmt=1&payCode=WECHAT_JSAPI&notifyTime=20260505115804`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Equal(t, "PO202605050001", notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605050001", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, int64(100), notification.Fact.SuccessAmountFen)
	require.Equal(t, int64(1), notification.Fact.FeeAmountFen)
	require.Equal(t, "WECHAT_JSAPI", notification.Fact.PayCode)
	require.Equal(t, "success", notification.TerminalStatus)
	require.True(t, notification.IsTerminal)
	require.JSONEq(t, `{"feeAmt":1,"merId":"102004465","notifyTime":"20260505115804","outTradeNo":"PO202605050001","payCode":"WECHAT_JSAPI","resultCode":"SUCCESS","succAmt":100,"terId":"200005200","tradeNo":"BFPAY202605050001","txnState":"SUCCESS"}`, string(notification.Raw))
}

func TestParserParsePaymentNotificationUnwrapsOfficialPublicEnvelopeForm(t *testing.T) {
	body := []byte(`merId=102004465&terId=200005200&charset=UTF-8&version=1.0&format=json&notifyType=PAYMENT&signType=RSA&signSn=1&ncrptnSn=1&signStr=abc&dataContent=%7B%22merId%22%3A%22102004465%22%2C%22terId%22%3A%22200005200%22%2C%22resultCode%22%3A%22SUCCESS%22%2C%22outTradeNo%22%3A%22PO202605050002%22%2C%22tradeNo%22%3A%22BFPAY202605050002%22%2C%22txnState%22%3A%22SUCCESS%22%2C%22succAmt%22%3A100%2C%22feeAmt%22%3A1%2C%22payCode%22%3A%22WECHAT_JSAPI%22%7D`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Equal(t, "PAYMENT", notification.NotifyType)
	require.Equal(t, "PO202605050002", notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605050002", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, int64(100), notification.Fact.SuccessAmountFen)
	require.Equal(t, int64(1), notification.Fact.FeeAmountFen)
	require.Equal(t, "WECHAT_JSAPI", notification.Fact.PayCode)
	require.JSONEq(t, `{"feeAmt":1,"merId":"102004465","notifyType":"PAYMENT","outTradeNo":"PO202605050002","payCode":"WECHAT_JSAPI","resultCode":"SUCCESS","succAmt":100,"terId":"200005200","tradeNo":"BFPAY202605050002","txnState":"SUCCESS"}`, string(notification.Raw))
}

func TestParserParsePaymentNotificationVerifiesOfficialPublicEnvelopeSignature(t *testing.T) {
	privatePEM, publicPEM := generateBaofuNotificationTestKeyPair(t)
	dataContent := `{"merId":"102004465","terId":"200005200","resultCode":"SUCCESS","tradeNo":"BFPAY202605050004","txnState":"SUCCESS","succAmt":100,"payCode":"WECHAT_JSAPI"}`
	signature, err := baofu.SignSHA256WithRSA(privatePEM, []byte(dataContent))
	require.NoError(t, err)
	values := signedNotificationEnvelopeValues("PAYMENT", dataContent, signature)
	parser := NewParserWithPublicKey(publicPEM)

	notification, err := parser.ParsePaymentNotification([]byte(values.Encode()))

	require.NoError(t, err)
	require.Equal(t, "PAYMENT", notification.NotifyType)
	require.Empty(t, notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605050004", notification.Fact.TradeNo)

	values.Set("dataContent", `{"merId":"102004465","terId":"200005200","resultCode":"SUCCESS","tradeNo":"tampered","txnState":"SUCCESS","succAmt":100,"payCode":"WECHAT_JSAPI"}`)
	_, err = parser.ParsePaymentNotification([]byte(values.Encode()))
	require.ErrorIs(t, err, baofu.ErrInvalidSignature)
}

func TestParserParseShareNotificationVerifiesOfficialPublicEnvelopeSignature(t *testing.T) {
	privatePEM, publicPEM := generateBaofuNotificationTestKeyPair(t)
	dataContent := `{"tradeNo":"BFSHARE_UP_3004","txnState":"SUCCESS","resultCode":"SUCCESS","succAmt":9470}`
	signature, err := baofu.SignSHA256WithRSA(privatePEM, []byte(dataContent))
	require.NoError(t, err)
	parser := NewParserWithPublicKey(publicPEM)

	notification, err := parser.ParseShareNotification([]byte(signedNotificationEnvelopeValues("SHARE", dataContent, signature).Encode()))

	require.NoError(t, err)
	require.Equal(t, "SHARE", notification.NotifyType)
	require.Equal(t, "BFSHARE_UP_3004", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.ShareStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, int64(9470), notification.Fact.SuccessAmountFen)
}

func TestParserParseRefundNotificationVerifiesOfficialPublicEnvelopeSignature(t *testing.T) {
	privatePEM, publicPEM := generateBaofuNotificationTestKeyPair(t)
	dataContent := `{"merId":"102004465","terId":"200005200","outTradeNo":"BFRFD_5104","tradeNo":"BFREFUND_UP_5104","refundState":"SUCCESS","resultCode":"SUCCESS","txnTime":"20260505120500","succAmt":1200}`
	signature, err := baofu.SignSHA256WithRSA(privatePEM, []byte(dataContent))
	require.NoError(t, err)
	parser := NewParserWithPublicKey(publicPEM)

	notification, err := parser.ParseRefundNotification([]byte(signedNotificationEnvelopeValues("REFUND", dataContent, signature).Encode()))

	require.NoError(t, err)
	require.Equal(t, "REFUND", notification.NotifyType)
	require.Equal(t, "BFRFD_5104", notification.Fact.OutTradeNo)
	require.Equal(t, "BFREFUND_UP_5104", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.RefundStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, int64(1200), notification.Fact.SuccessAmountFen)
}

func TestParserWithPublicKeyRequiresOfficialSignedEnvelope(t *testing.T) {
	_, publicPEM := generateBaofuNotificationTestKeyPair(t)
	parser := NewParserWithPublicKey(publicPEM)

	_, err := parser.ParsePaymentNotification([]byte(`resultCode=SUCCESS&merId=102004465&terId=200005200&tradeNo=BFPAY202605050001&txnState=SUCCESS&succAmt=100&payCode=WECHAT_JSAPI`))

	require.EqualError(t, err, "baofu aggregate notification signed public envelope is required")
}

func TestParserParsePaymentNotificationUnwrapsOfficialPublicEnvelopeJSON(t *testing.T) {
	body := []byte(`{
		"merId":"102004465",
		"terId":"200005200",
		"charset":"UTF-8",
		"version":"1.0",
		"format":"json",
		"notifyType":"PAYMENT",
		"signType":"RSA",
		"signSn":"1",
		"ncrptnSn":"1",
		"signStr":"abc",
		"dataContent":"{\"merId\":\"102004465\",\"terId\":\"200005200\",\"resultCode\":\"SUCCESS\",\"outTradeNo\":\"PO202605050003\",\"tradeNo\":\"BFPAY202605050003\",\"txnState\":\"SUCCESS\",\"succAmt\":100,\"feeAmt\":1,\"payCode\":\"WECHAT_JSAPI\"}"
	}`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Equal(t, "PAYMENT", notification.NotifyType)
	require.Equal(t, "PO202605050003", notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605050003", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, notification.Fact.TransactionState)
}

func TestParserParsePaymentNotificationAcceptsOfficialTradeNoOnly(t *testing.T) {
	body := []byte(`{
		"merId":"102004465",
		"terId":"200005200",
		"tradeNo":"BFPAY202605050010",
		"txnState":"SUCCESS",
		"succAmt":100,
		"resultCode":"SUCCESS",
		"payCode":"WECHAT_JSAPI"
	}`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Empty(t, notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605050010", notification.Fact.TradeNo)
	require.Equal(t, "102004465", notification.Fact.MerchantID)
	require.Equal(t, "200005200", notification.Fact.TerminalID)
}

func TestParserParsePaymentNotificationRequiresOfficialRequiredFields(t *testing.T) {
	parser := NewParser()

	_, err := parser.ParsePaymentNotification([]byte(`{"merId":"102004465","terId":"200005200","tradeNo":"BFPAY202605050010","txnState":"SUCCESS"}`))

	require.ErrorIs(t, err, ErrPaymentNotificationPayCodeRequired)
}

func TestParserParseShareNotificationNormalizesShareFact(t *testing.T) {
	body := []byte(`{
		"notifyId":"BFSN202605030001",
		"notifyType":"SHARE.SUCCESS",
		"merId":"102004465",
		"terId":"200005200",
		"outTradeNo":"BFSHARE202605030001",
		"tradeNo":"BFSHAREUP202605030001",
		"txnState":"SUCCESS",
		"resultCode":"SUCCESS",
		"finishTime":"20260503120500",
		"succAmt":10000,
		"clearingDate":"20260503",
		"sharingMerId":"CP_MUST_STAY_RAW_ONLY",
		"notifyTime":"20260503120500"
	}`)
	parser := NewParser()

	notification, err := parser.ParseShareNotification(body)

	require.NoError(t, err)
	require.Equal(t, "BFSN202605030001", notification.NotifyID)
	require.Equal(t, "SHARE.SUCCESS", notification.NotifyType)
	require.Equal(t, "BFSHARE202605030001", notification.Fact.OutTradeNo)
	require.Equal(t, "BFSHAREUP202605030001", notification.Fact.TradeNo)
	require.Equal(t, "102004465", notification.Fact.MerchantID)
	require.Equal(t, aggregatecontracts.ShareStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, "20260503120500", notification.Fact.FinishTime)
	require.Equal(t, int64(10000), notification.Fact.SuccessAmountFen)
	require.Equal(t, "20260503", notification.Fact.ClearingDate)
	require.Equal(t, "SUCCESS", notification.Fact.ResultCode)
	require.Equal(t, "success", notification.TerminalStatus)
	require.True(t, notification.IsTerminal)
	require.Equal(t, "2026-05-03T12:05:00Z", notification.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"))
	require.JSONEq(t, string(body), string(notification.Raw))
}

func TestParserParseShareNotificationAcceptsOfficialTradeNoOnly(t *testing.T) {
	parser := NewParser()

	notification, err := parser.ParseShareNotification([]byte(`{"tradeNo":"BFSHARE_UP_3001","txnState":"SUCCESS","resultCode":"SUCCESS"}`))

	require.NoError(t, err)
	require.Empty(t, notification.Fact.OutTradeNo)
	require.Equal(t, "BFSHARE_UP_3001", notification.Fact.TradeNo)
}

func TestParserParseShareNotificationRequiresOfficialRequiredFields(t *testing.T) {
	parser := NewParser()

	_, err := parser.ParseShareNotification([]byte(`{"tradeNo":"BFSHARE_UP_3001","resultCode":"SUCCESS"}`))

	require.ErrorIs(t, err, ErrShareNotificationTransactionStateRequired)
}

func TestParserParseRefundNotificationNormalizesRefundFact(t *testing.T) {
	body := []byte(`{
		"notifyId":"BFRN202605040001",
		"notifyType":"REFUND.SUCCESS",
		"agentMerId":"AGENT_MER",
		"agentTerId":"AGENT_TER",
		"merId":"102004465",
		"terId":"200005200",
		"outTradeNo":"RF202605040001",
		"tradeNo":"BFREFUND202605040001",
		"refundState":"SUCCESS",
		"succAmt":300,
		"resultCode":"SUCCESS",
		"txnTime":"20260504120900",
		"finishTime":"20260504121000"
	}`)
	parser := NewParser()

	notification, err := parser.ParseRefundNotification(body)

	require.NoError(t, err)
	require.Equal(t, "BFRN202605040001", notification.NotifyID)
	require.Equal(t, "RF202605040001", notification.Fact.OutTradeNo)
	require.Equal(t, "BFREFUND202605040001", notification.Fact.TradeNo)
	require.Equal(t, "AGENT_MER", notification.Fact.AgentMerchantID)
	require.Equal(t, "102004465", notification.Fact.MerchantID)
	require.Equal(t, aggregatecontracts.RefundStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, "20260504121000", notification.Fact.FinishTime)
	require.Equal(t, int64(300), notification.Fact.SuccessAmountFen)
	require.Equal(t, "20260504120900", notification.Fact.TransactionTime)
	require.Equal(t, "success", notification.TerminalStatus)
	require.True(t, notification.IsTerminal)
	require.Equal(t, "2026-05-04T12:10:00Z", notification.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func TestParserParseRefundNotificationRequiresOfficialRequiredFields(t *testing.T) {
	parser := NewParser()

	_, err := parser.ParseRefundNotification([]byte(`{"merId":"102004465","terId":"200005200","outTradeNo":"BFRFD_5101","refundState":"SUCCESS","resultCode":"SUCCESS","txnTime":"20260504120900"}`))

	require.ErrorIs(t, err, ErrRefundNotificationTradeNoRequired)
}

func signedNotificationEnvelopeValues(notifyType, dataContent, signature string) url.Values {
	values := url.Values{}
	values.Set("merId", "102004465")
	values.Set("terId", "200005200")
	values.Set("charset", baofu.PublicEnvelopeCharsetUTF8)
	values.Set("version", baofu.PublicEnvelopeVersion10)
	values.Set("format", baofu.PublicEnvelopeFormatJSON)
	values.Set("notifyType", notifyType)
	values.Set("signType", baofu.SignTypeRSA)
	values.Set("signSn", "1")
	values.Set("ncrptnSn", "1")
	values.Set("signStr", signature)
	values.Set("dataContent", dataContent)
	return values
}

func generateBaofuNotificationTestKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
}
