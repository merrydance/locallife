package aggregatepay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	"github.com/stretchr/testify/require"
)

func TestAggregateClientCreateUnifiedOrderPostsPublicEnvelope(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","txnState":"WAIT_PAYING","tradeNo":"BFPAY202605040001","chlRetParam":{"wc_pay_data":{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx","signType":"RSA","paySign":"sign"}}}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.NoError(t, err)
	require.Equal(t, "BFPAY202605040001", result.TradeNo)
	require.Equal(t, baofu.SandboxAggregatePayBaseURL, doer.request.URL.String())
	require.Equal(t, http.MethodPost, doer.request.Method)
	require.Contains(t, doer.request.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "unified_order", env.Method)
	require.Equal(t, "102004465", env.MerchantID)
	require.Equal(t, "200005200", env.TerminalID)
	require.NotEmpty(t, env.SignString)
	require.JSONEq(t, `{"outTradeNo":"BF202605040001","subMchId":"1900000109","riskInfo":{"clientIp":"203.0.113.1"}}`, partialJSONForTest(t, json.RawMessage(env.BizContent), "outTradeNo", "subMchId", "riskInfo"))
	require.Contains(t, string(doer.requestBody), "bizContent=%7B%22")
}

func TestAggregateClientReturnsSanitizedProviderError(t *testing.T) {
	doer := &aggregateRecordingDoer{statusCode: http.StatusBadGateway, responseBody: []byte(`upstream raw failure with signature payload`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	require.NotContains(t, err.Error(), "upstream raw failure")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "unified_order", providerErr.Operation)
	require.Equal(t, http.StatusBadGateway, providerErr.StatusCode)
	require.NotContains(t, providerErr.Frontend.Message, "upstream raw")
}

func TestAggregateClientReturnsProviderErrorForBusinessFailure(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"FAIL","errCode":"MERCHANT_NOT_REPORT","errMsg":"上游原始报备错误","outTradeNo":"BF202605040001"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游原始报备错误")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "MERCHANT_NOT_REPORT", providerErr.UpstreamCode)
	require.Equal(t, "上游原始报备错误", providerErr.UpstreamMessage)
	require.Equal(t, "商户微信支付通道待开通，请联系平台处理", providerErr.Frontend.Message)
	require.Equal(t, "contact_platform", providerErr.Frontend.Action)
}

func TestAggregateClientReturnsProviderErrorForEnvelopeFailure(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBody: []byte(`{"returnCode":"FAIL","returnMsg":"上游签名证书不匹配"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryPayment(context.Background(), contracts.PaymentQueryRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001"})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游签名证书不匹配")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, baofu.PublicEnvelopeReturnCodeFail, providerErr.UpstreamCode)
	require.Equal(t, "上游签名证书不匹配", providerErr.UpstreamMessage)
	require.Equal(t, "资料信息不完整，请核对后重新提交", providerErr.Frontend.Message)
}

func validUnifiedOrderRequestForClientTest() contracts.UnifiedOrderRequest {
	return contracts.NewWechatJSAPISharingUnifiedOrderRequest(contracts.UnifiedOrderInput{
		MerchantID: "102004465",
		TerminalID: "200005200",
		OutTradeNo: "BF202605040001",
		AmountFen:  1200,
		TxnTime:    "20260504120000",
		TimeExpire: 30,
		SubMchID:   "1900000109",
		SubAppID:   "wx1234567890abcdef",
		SubOpenID:  "openid-001",
		Body:       "本地生活订单",
		NotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		ClientIP:   "203.0.113.1",
	})
}

type aggregateRecordingDoer struct {
	request            *http.Request
	requestBody        []byte
	statusCode         int
	responseBizContent json.RawMessage
	responseBody       []byte
}

func (d *aggregateRecordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.request = req
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.requestBody = body
	status := d.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	responseBody := d.responseBody
	if responseBody == nil {
		reqEnv := publicEnvelopeFromFormBytes(body)
		responseBody, _ = json.Marshal(baofu.PublicResponseEnvelope{
			ReturnCode:         baofu.PublicEnvelopeReturnCodeSuccess,
			MerchantID:         reqEnv.MerchantID,
			TerminalID:         reqEnv.TerminalID,
			Charset:            baofu.PublicEnvelopeCharsetUTF8,
			Version:            baofu.PublicEnvelopeVersion10,
			Format:             baofu.PublicEnvelopeFormatJSON,
			SignType:           baofu.SignTypeRSA,
			SignSerialNo:       "test-sign-sn",
			EncryptionSerialNo: "test-enc-sn",
			SignString:         "test-signature",
			BizContent:         baofu.JSONString(d.responseBizContent),
		})
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(responseBody)), Header: make(http.Header)}, nil
}

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	client, err := baofu.NewClient(baofu.Config{
		Environment:        baofu.BaofuEnvironmentSandbox,
		CollectMerchantID:  "102004465",
		CollectTerminalID:  "200005200",
		PayoutMerchantID:   "102004466",
		PayoutTerminalID:   "200005201",
		AppID:              "wx1234567890abcdef",
		PrivateKeyPEM:      privatePEM,
		BaofuPublicKeyPEM:  publicPEM,
		NotifyBaseURL:      "https://api.example.com/v1/webhooks/baofu",
		SignSerialNo:       "test-sign-sn",
		EncryptionSerialNo: "test-enc-sn",
		Timeout:            5 * time.Second,
	}, doer)
	require.NoError(t, err)
	return client
}

func generateClientTestKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
}

func partialJSONForTest(t *testing.T, raw json.RawMessage, keys ...string) string {
	t.Helper()
	var full map[string]any
	require.NoError(t, json.Unmarshal(raw, &full))
	partial := make(map[string]any, len(keys))
	for _, key := range keys {
		partial[key] = full[key]
	}
	body, err := json.Marshal(partial)
	require.NoError(t, err)
	return string(body)
}

func TestAggregateClientCreateRefundPostsPublicEnvelope(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605040001","tradeNo":"BFREFUND202605040001","refundState":"REFUND","refundAmt":300,"totalAmt":300}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.CreateRefund(context.Background(), validRefundBeforeShareRequestForClientTest())

	require.NoError(t, err)
	require.Equal(t, "BFREFUND202605040001", result.TradeNo)
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "order_refund", env.Method)
	require.JSONEq(t, `{"originOutTradeNo":"BF202605040001","outTradeNo":"RF202605040001","refundAmt":300,"totalAmt":300}`, partialJSONForTest(t, json.RawMessage(env.BizContent), "originOutTradeNo", "outTradeNo", "refundAmt", "totalAmt"))
	require.NotContains(t, string(env.BizContent), "sharingRefundInfo")
	require.NotContains(t, string(env.BizContent), "advanceAmt")
}

func TestAggregateClientQueryRefundAndCloseOrderPostPublicEnvelope(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605040001","tradeNo":"BFREFUND202605040001","refundState":"SUCCESS"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryRefund(context.Background(), contracts.RefundQueryRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "RF202605040001"})
	require.NoError(t, err)
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "refund_query", env.Method)

	doer.responseBizContent = json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"BF202605040001"}`)
	_, err = client.CloseOrder(context.Background(), contracts.OrderCloseRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001"})
	require.NoError(t, err)
	env = publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "order_close", env.Method)
}

func publicEnvelopeFromFormForTest(t *testing.T, raw []byte) baofu.PublicRequestEnvelope {
	t.Helper()
	return publicEnvelopeFromFormBytes(raw)
}

func publicEnvelopeFromFormBytes(raw []byte) baofu.PublicRequestEnvelope {
	values, _ := url.ParseQuery(string(raw))
	return baofu.PublicRequestEnvelope{
		MerchantID:         values.Get("merId"),
		TerminalID:         values.Get("terId"),
		Method:             values.Get("method"),
		Charset:            values.Get("charset"),
		Version:            values.Get("version"),
		Format:             values.Get("format"),
		Timestamp:          values.Get("timestamp"),
		SignType:           values.Get("signType"),
		SignSerialNo:       values.Get("signSn"),
		EncryptionSerialNo: values.Get("ncrptnSn"),
		DigitalEnvelope:    values.Get("dgtlEnvlp"),
		SignString:         values.Get("signStr"),
		BizContent:         baofu.JSONString(values.Get("bizContent")),
	}
}

func validRefundBeforeShareRequestForClientTest() contracts.RefundBeforeShareRequest {
	return contracts.RefundBeforeShareRequest{
		MerchantID:       "102004465",
		TerminalID:       "200005200",
		OriginOutTradeNo: "BF202605040001",
		OutTradeNo:       "RF202605040001",
		NotifyURL:        "https://api.example.com/v1/webhooks/baofu/refund",
		RefundAmountFen:  300,
		TotalAmountFen:   300,
		TransactionTime:  "20260504120500",
		RefundReason:     "用户申请退款",
	}
}
