package aggregatepay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
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
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","txnState":"WAIT_PAYING","tradeNo":"BFPAY202605040001","payCode":"WECHAT_JSAPI","chlRetParam":{"wc_pay_data":{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx","signType":"RSA","paySign":"sign"}}}`)}
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
	require.JSONEq(t, `{"outTradeNo":"BF202605040001","riskInfo":{"clientIp":"203.0.113.1"}}`, partialJSONForTest(t, json.RawMessage(env.BizContent), "outTradeNo", "riskInfo"))
	require.NotContains(t, string(env.BizContent), "subMchId")
	require.Contains(t, string(doer.requestBody), "bizContent=%7B%22")
}

func TestAggregateClientProductionRequiresUnifiedOrderSubMchID(t *testing.T) {
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS"}`)}
	client := NewClient(testBaofuRootClientWithEnvironment(t, doer, baofu.BaofuEnvironmentProduction))
	req := validUnifiedOrderRequestForClientTest()
	req.SubMchID = ""

	_, err := client.CreateUnifiedOrder(context.Background(), req)

	require.ErrorIs(t, err, contracts.ErrUnifiedOrderSubMchIDRequired)
	require.Nil(t, doer.request)
}

func TestAggregateClientProductionKeepsUnifiedOrderSubMchID(t *testing.T) {
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","tradeNo":"BFPAY202605040002","txnState":"WAIT_PAYING","payCode":"WECHAT_JSAPI"}`)}
	client := NewClient(testBaofuRootClientWithEnvironment(t, doer, baofu.BaofuEnvironmentProduction))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.NoError(t, err)
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Contains(t, string(env.BizContent), `"subMchId":"1900000109"`)
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
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"FAIL","errCode":"MERCHANT_NOT_REPORT","errMsg":"上游原始报备错误","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","payCode":"WECHAT_JSAPI"}`)}
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

func TestAggregateClientReturnsBusinessErrorForShareQueryFailureWithoutTxnState(t *testing.T) {
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"FAIL","errCode":"ORDER_NOT_EXIST","errMsg":"上游原始订单不存在","merId":"102004465","terId":"200005200","outTradeNo":"BFSHARE202605040001"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryProfitSharing(context.Background(), contracts.ShareQueryRequest{
		MerchantID: "102004465",
		TerminalID: "200005200",
		OutTradeNo: "BFSHARE202605040001",
	})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游原始订单不存在")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "share_query", providerErr.Operation)
	require.Equal(t, "ORDER_NOT_EXIST", providerErr.UpstreamCode)
	require.Equal(t, "上游原始订单不存在", providerErr.UpstreamMessage)
}

func TestAggregateClientValidatesBusinessFailurePayloadBeforeReturningProviderError(t *testing.T) {
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"FAIL","errCode":"MERCHANT_NOT_REPORT","errMsg":"上游原始报备错误","outTradeNo":"BF202605040001","payCode":"WECHAT_JSAPI"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	requireAggregateContractProviderError(t, err, "unified_order", "baofu unified order response merId is required")
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

func TestAggregateClientClassifiesSuccessEnvelopeMissingDataContent(t *testing.T) {
	doer := &aggregateRecordingDoer{responseBody: []byte(`{"returnCode":"SUCCESS","returnMsg":"OK","merId":"102004465","terId":"200005200","charset":"UTF-8","version":"1.0","format":"json","signType":"RSA","signSn":"1","ncrptnSn":"1","signStr":"test-signature"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, baofu.PublicEnvelopeUpstreamCodeMissingDataContent, providerErr.UpstreamCode)
	require.Equal(t, "支付通道异常，请联系平台处理", providerErr.Frontend.Message)
}

func TestAggregateClientRejectsInvalidSignedPublicResponse(t *testing.T) {
	dataContent := baofu.JSONString(`{"resultCode":"SUCCESS","outTradeNo":"BF202605040001"}`)
	responseBody, _ := json.Marshal(baofu.PublicResponseEnvelope{
		ReturnCode:         baofu.PublicEnvelopeReturnCodeSuccess,
		ReturnMessage:      "OK",
		MerchantID:         "102004465",
		TerminalID:         "200005200",
		Charset:            baofu.PublicEnvelopeCharsetUTF8,
		Version:            baofu.PublicEnvelopeVersion10,
		Format:             baofu.PublicEnvelopeFormatJSON,
		SignType:           baofu.SignTypeRSA,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         "bad-signature",
		DataContent:        dataContent,
	})
	doer := &aggregateRecordingDoer{responseBody: responseBody}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, baofu.PublicEnvelopeUpstreamCodeInvalidSignature, providerErr.UpstreamCode)
	require.ErrorIs(t, errors.Unwrap(providerErr), baofu.ErrInvalidSignature)
}

func TestAggregateClientClassifiesBusinessPayloadUnmarshalFailure(t *testing.T) {
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":{"bad":"shape"}}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, baofu.PublicEnvelopeUpstreamCodeInvalidDataContent, providerErr.UpstreamCode)
	require.NotEqual(t, baofu.PublicEnvelopeReturnCodeSuccess, providerErr.UpstreamCode)
	require.Contains(t, errors.Unwrap(providerErr).Error(), "json")
}

func TestAggregateClientRejectsSignedEnvelopeIdentityMismatch(t *testing.T) {
	doer := &aggregateRecordingDoer{
		responseMerchantID:  "102004999",
		responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","payCode":"WECHAT_JSAPI","txnState":"WAIT_PAYING"}`),
	}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())

	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, baofu.PublicEnvelopeReturnCodeSuccess, providerErr.UpstreamCode)
	require.Contains(t, errors.Unwrap(providerErr).Error(), "response merId")
}

func TestAggregateClientRunsMethodSpecificResponseValidation(t *testing.T) {
	t.Run("unified order missing payCode", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001","txnState":"WAIT_PAYING"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())
		requireAggregateContractProviderError(t, err, "unified_order", "baofu unified order response payCode is required")
	})

	t.Run("share missing txnState", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BFSHARE202605040001"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateProfitSharing(context.Background(), validShareAfterPayRequestForClientTest())
		requireAggregateContractProviderError(t, err, "share_after_pay", "baofu share response txnState is required")
	})

	t.Run("refund missing tradeNo", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605040001","refundAmt":300,"totalAmt":300}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateRefund(context.Background(), validRefundBeforeShareRequestForClientTest())
		requireAggregateContractProviderError(t, err, "order_refund", "baofu refund response tradeNo is required")
	})

	t.Run("close missing merId", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","terId":"200005200","outTradeNo":"BF202605040001"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CloseOrder(context.Background(), contracts.OrderCloseRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001"})
		requireAggregateContractProviderError(t, err, "order_close", "baofu order close response merId is required")
	})
}

func TestAggregateClientRejectsBusinessResponseNotBoundToRequest(t *testing.T) {
	t.Run("unified order outTradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605049999","txnState":"WAIT_PAYING","payCode":"WECHAT_JSAPI"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateUnifiedOrder(context.Background(), validUnifiedOrderRequestForClientTest())
		requireAggregateContractProviderError(t, err, "unified_order", "outTradeNo does not match request")
	})

	t.Run("payment query tradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","tradeNo":"BFPAY_MISMATCH","outTradeNo":"BF202605040001","txnState":"SUCCESS","payCode":"WECHAT_JSAPI"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.QueryPayment(context.Background(), contracts.PaymentQueryRequest{MerchantID: "102004465", TerminalID: "200005200", TradeNo: "BFPAY_EXPECTED"})
		requireAggregateContractProviderError(t, err, "order_query", "tradeNo does not match request")
	})

	t.Run("share after pay outTradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BFSHARE_MISMATCH","txnState":"PROCESSING"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateProfitSharing(context.Background(), validShareAfterPayRequestForClientTest())
		requireAggregateContractProviderError(t, err, "share_after_pay", "outTradeNo does not match request")
	})

	t.Run("share query tradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","tradeNo":"BFSHARE_MISMATCH","txnState":"SUCCESS"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.QueryProfitSharing(context.Background(), contracts.ShareQueryRequest{MerchantID: "102004465", TerminalID: "200005200", TradeNo: "BFSHARE_EXPECTED"})
		requireAggregateContractProviderError(t, err, "share_query", "tradeNo does not match request")
	})

	t.Run("refund outTradeNo and amount mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605049999","tradeNo":"BFREFUND202605040001","refundState":"REFUND","refundAmt":301,"totalAmt":301}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CreateRefund(context.Background(), validRefundBeforeShareRequestForClientTest())
		requireAggregateContractProviderError(t, err, "order_refund", "outTradeNo does not match request")
	})

	t.Run("refund query outTradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605049999","tradeNo":"BFREFUND202605040001","refundState":"SUCCESS"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.QueryRefund(context.Background(), contracts.RefundQueryRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "RF202605040001"})
		requireAggregateContractProviderError(t, err, "refund_query", "outTradeNo does not match request")
	})

	t.Run("close order outTradeNo mismatch", func(t *testing.T) {
		doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605049999"}`)}
		client := NewClient(testBaofuRootClient(t, doer))

		_, err := client.CloseOrder(context.Background(), contracts.OrderCloseRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "BF202605040001"})
		requireAggregateContractProviderError(t, err, "order_close", "outTradeNo does not match request")
	})
}

func requireAggregateContractProviderError(t *testing.T, err error, operation string, causeSubstring string) {
	t.Helper()
	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, operation, providerErr.Operation)
	require.Equal(t, baofu.PublicEnvelopeUpstreamCodeInvalidDataContent, providerErr.UpstreamCode)
	require.Contains(t, errors.Unwrap(providerErr).Error(), causeSubstring)
	require.NotContains(t, err.Error(), causeSubstring)
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
	request             *http.Request
	requestBody         []byte
	statusCode          int
	responseMerchantID  string
	responseTerminalID  string
	responseDataContent json.RawMessage
	responseBody        []byte
	baofuPrivatePEM     string
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
		signature, err := baofu.SignSHA256WithRSA(d.baofuPrivatePEM, []byte(d.responseDataContent))
		if err != nil {
			return nil, err
		}
		responseMerchantID := d.responseMerchantID
		if responseMerchantID == "" {
			responseMerchantID = reqEnv.MerchantID
		}
		responseTerminalID := d.responseTerminalID
		if responseTerminalID == "" {
			responseTerminalID = reqEnv.TerminalID
		}
		responseBody, _ = json.Marshal(baofu.PublicResponseEnvelope{
			ReturnCode:         baofu.PublicEnvelopeReturnCodeSuccess,
			ReturnMessage:      "OK",
			MerchantID:         responseMerchantID,
			TerminalID:         responseTerminalID,
			Charset:            baofu.PublicEnvelopeCharsetUTF8,
			Version:            baofu.PublicEnvelopeVersion10,
			Format:             baofu.PublicEnvelopeFormatJSON,
			SignType:           baofu.SignTypeRSA,
			SignSerialNo:       "1",
			EncryptionSerialNo: "1",
			SignString:         signature,
			DataContent:        baofu.JSONString(d.responseDataContent),
		})
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(responseBody)), Header: make(http.Header)}, nil
}

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	return testBaofuRootClientWithEnvironment(t, doer, baofu.BaofuEnvironmentSandbox)
}

func testBaofuRootClientWithEnvironment(t *testing.T, doer baofu.HTTPDoer, environment string) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	if recorder, ok := doer.(*aggregateRecordingDoer); ok {
		recorder.baofuPrivatePEM = privatePEM
	}
	client, err := baofu.NewClient(baofu.Config{
		Environment:        environment,
		CollectMerchantID:  "102004465",
		CollectTerminalID:  "200005200",
		PayoutMerchantID:   "102004466",
		PayoutTerminalID:   "200005201",
		AppID:              "wx1234567890abcdef",
		PrivateKeyPEM:      privatePEM,
		BaofuPublicKeyPEM:  publicPEM,
		NotifyBaseURL:      "https://api.example.com/v1/webhooks/baofu",
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
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
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605040001","tradeNo":"BFREFUND202605040001","refundState":"REFUND","refundAmt":300,"totalAmt":300}`)}
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
	doer := &aggregateRecordingDoer{responseDataContent: json.RawMessage(`{"resultCode":"SUCCESS","outTradeNo":"RF202605040001","tradeNo":"BFREFUND202605040001","refundState":"SUCCESS"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryRefund(context.Background(), contracts.RefundQueryRequest{MerchantID: "102004465", TerminalID: "200005200", OutTradeNo: "RF202605040001"})
	require.NoError(t, err)
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "refund_query", env.Method)

	doer.responseDataContent = json.RawMessage(`{"resultCode":"SUCCESS","merId":"102004465","terId":"200005200","outTradeNo":"BF202605040001"}`)
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

func validShareAfterPayRequestForClientTest() contracts.ShareAfterPayRequest {
	return contracts.ShareAfterPayRequest{
		MerchantID:       "102004465",
		TerminalID:       "200005200",
		OriginOutTradeNo: "BF202605040001",
		OutTradeNo:       "BFSHARE202605040001",
		TxnTime:          "20260504120600",
		NotifyURL:        "https://api.example.com/v1/webhooks/baofu/share",
		SharingDetails:   []contracts.SharingDetail{{SharingMerID: "CM202605040001", SharingAmountFen: 300}},
	}
}
