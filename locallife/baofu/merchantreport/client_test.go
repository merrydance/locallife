package merchantreport

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
	"github.com/merrydance/locallife/baofu/merchantreport/contracts"
	"github.com/stretchr/testify/require"
)

func TestMerchantReportClientSubmitReportPostsPublicEnvelope(t *testing.T) {
	doer := &merchantReportRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"SUCCESS","reportType":"WECHAT","reportNo":"MR202605040001","reportState":"SUCCESS","subMchId":"1900000109","platformBizNo":"PB202605040001"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.SubmitWechatReport(context.Background(), validWechatReportRequestForClientTest())

	require.NoError(t, err)
	require.Equal(t, "1900000109", result.SubMchID)
	require.Equal(t, baofu.SandboxMerchantReportBaseURL, doer.request.URL.String())
	require.Contains(t, doer.request.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "merchant_report", env.Method)
	require.Contains(t, string(env.BizContent), `"bctMerId":"CM202605040001"`)
	require.NotContains(t, string(env.BizContent), "sharingMerId")
}

func TestMerchantReportClientBindSubConfigPostsAppletAuth(t *testing.T) {
	doer := &merchantReportRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"SUCCESS","subMchId":"1900000109","authType":"APPLET"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.BindSubConfig(context.Background(), contracts.BindSubConfigRequest{MerchantID: "102004465", TerminalID: "200005200", SubMchID: "1900000109", AuthType: contracts.AuthTypeApplet, AuthContent: "wx1234567890abcdef", Remark: "LocalLife mini program"})

	require.NoError(t, err)
	env := publicEnvelopeFromFormForTest(t, doer.requestBody)
	require.Equal(t, "bind_sub_config", env.Method)
	require.Contains(t, string(env.BizContent), `"authType":"APPLET"`)
	require.Contains(t, string(env.BizContent), `"authContent":"wx1234567890abcdef"`)
}

func TestMerchantReportClientReturnsProviderErrorForBusinessFailure(t *testing.T) {
	doer := &merchantReportRecordingDoer{responseBizContent: json.RawMessage(`{"resultCode":"FAIL","errCode":"INVALID_PARAMETER","errMsg":"上游原始参数错误"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.SubmitWechatReport(context.Background(), validWechatReportRequestForClientTest())

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游原始参数错误")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "INVALID_PARAMETER", providerErr.UpstreamCode)
	require.Equal(t, "资料信息不完整，请核对后重新提交", providerErr.Frontend.Message)
}

func validWechatReportRequestForClientTest() contracts.WechatMerchantReportRequest {
	return contracts.WechatMerchantReportRequest{
		MerchantID:    "102004465",
		TerminalID:    "200005200",
		ReportType:    contracts.ReportTypeWechat,
		ReportNo:      "MR202605040001",
		BCTMerchantID: "CM202605040001",
		ReportInfo: contracts.WechatReportInfo{
			MerchantName:        "上海某某餐饮有限公司",
			MerchantShortName:   "某某餐饮",
			ServicePhone:        "02112345678",
			ChannelID:           "channel-001",
			ChannelName:         "乐客来福",
			Business:            "758-2",
			ServiceCodes:        []string{contracts.WechatServiceTypeApplet},
			AddressInfo:         contracts.WechatAddressInfo{Province: "上海市", City: "上海市", District: "浦东新区", Address: "世纪大道 1 号"},
			BusinessLicenseType: contracts.WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        contracts.WechatBankCardInfo{AccountName: "上海某某餐饮有限公司", AccountNo: "6222000000000000000", BankName: "招商银行", BankBranchName: "招商银行上海分行"},
		},
	}
}

type merchantReportRecordingDoer struct {
	request            *http.Request
	requestBody        []byte
	responseBizContent json.RawMessage
}

func (d *merchantReportRecordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.request = req
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.requestBody = body
	reqEnv := publicEnvelopeFromFormForTest(nil, body)
	responseBody, _ := json.Marshal(baofu.PublicResponseEnvelope{ReturnCode: baofu.PublicEnvelopeReturnCodeSuccess, MerchantID: reqEnv.MerchantID, TerminalID: reqEnv.TerminalID, Charset: baofu.PublicEnvelopeCharsetUTF8, Version: baofu.PublicEnvelopeVersion10, Format: baofu.PublicEnvelopeFormatJSON, SignType: baofu.SignTypeRSA, SignSerialNo: "1", EncryptionSerialNo: "1", SignString: "test-signature", BizContent: baofu.JSONString(d.responseBizContent)})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(responseBody)), Header: make(http.Header)}, nil
}

func publicEnvelopeFromFormForTest(t require.TestingT, raw []byte) baofu.PublicRequestEnvelope {
	if helper, ok := t.(interface{ Helper() }); ok {
		helper.Helper()
	}
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

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	client, err := baofu.NewClient(baofu.Config{Environment: baofu.BaofuEnvironmentSandbox, CollectMerchantID: "102004465", CollectTerminalID: "200005200", PayoutMerchantID: "102004466", PayoutTerminalID: "200005201", AppID: "wx1234567890abcdef", PrivateKeyPEM: privatePEM, BaofuPublicKeyPEM: publicPEM, NotifyBaseURL: "https://api.example.com/v1/webhooks/baofu", SignSerialNo: "1", EncryptionSerialNo: "1", Timeout: 5 * time.Second}, doer)
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
