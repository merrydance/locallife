package account

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
	"testing"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
	"github.com/stretchr/testify/require"
)

func TestAccountClientQueryBalancePostsUnionGatewayEnvelope(t *testing.T) {
	doer := &accountRecordingDoer{responseBizContent: json.RawMessage(`{"retCode":"SUCCESS","contractNo":"CM202605040001","availableBal":"123.45","pendingBal":"1.00","currBal":"124.45","freezeBal":"0.00"}`)}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{MerchantID: "102004465", TerminalID: "200005200", ContractNo: "CM202605040001"})

	require.NoError(t, err)
	require.Equal(t, int64(12345), result.AvailableAmountFen)
	require.Equal(t, baofu.SandboxAccountGatewayBaseURL+"/T-1001-013-06/transReq.do", doer.request.URL.String())
	var env baofu.PublicRequestEnvelope
	require.NoError(t, json.Unmarshal(doer.requestBody, &env))
	require.Equal(t, "T-1001-013-06", env.Method)
	require.Contains(t, string(env.BizContent), `"contractNo":"CM202605040001"`)
}

type accountRecordingDoer struct {
	request            *http.Request
	requestBody        []byte
	responseBizContent json.RawMessage
}

func (d *accountRecordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.request = req
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.requestBody = body
	var reqEnv baofu.PublicRequestEnvelope
	_ = json.Unmarshal(body, &reqEnv)
	responseBody, _ := json.Marshal(baofu.PublicResponseEnvelope{ReturnCode: baofu.PublicEnvelopeReturnCodeSuccess, MerchantID: reqEnv.MerchantID, TerminalID: reqEnv.TerminalID, Charset: baofu.PublicEnvelopeCharsetUTF8, Version: baofu.PublicEnvelopeVersion10, Format: baofu.PublicEnvelopeFormatJSON, SignType: baofu.SignTypeRSA, SignSerialNo: "test-sign-sn", EncryptionSerialNo: "test-enc-sn", SignString: "test-signature", BizContent: d.responseBizContent})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(responseBody)), Header: make(http.Header)}, nil
}

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	client, err := baofu.NewClient(baofu.Config{Environment: baofu.BaofuEnvironmentSandbox, CollectMerchantID: "102004465", CollectTerminalID: "200005200", PayoutMerchantID: "102004466", PayoutTerminalID: "200005201", AppID: "wx1234567890abcdef", PrivateKeyPEM: privatePEM, BaofuPublicKeyPEM: publicPEM, AESKey: "0123456789abcdef0123456789abcdef", NotifyBaseURL: "https://api.example.com/v1/webhooks/baofu", SignSerialNo: "test-sign-sn", EncryptionSerialNo: "test-enc-sn", Timeout: 5 * time.Second}, doer)
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
