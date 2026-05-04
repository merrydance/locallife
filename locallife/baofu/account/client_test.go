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

func TestAccountClientQueryBalancePostsOfficialUnionGatewayRequest(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": "SUCCESS", "contractNo": "CM202605040001", "availableBal": "123.45", "pendingBal": "1.00", "currBal": "124.45", "freezeBal": "0.00"}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{MerchantID: "102004465", TerminalID: "200005200", ContractNo: "CM202605040001"})

	require.NoError(t, err)
	require.Equal(t, int64(12345), result.AvailableAmountFen)
	require.Equal(t, baofu.SandboxAccountGatewayBaseURL+"/T-1001-013-06/transReq.do", doer.request.URL.Scheme+"://"+doer.request.URL.Host+doer.request.URL.Path)
	require.Empty(t, doer.requestBody)
	query := doer.request.URL.Query()
	require.Equal(t, "102004465", query.Get("memberId"))
	require.Equal(t, "200005200", query.Get("terminalId"))
	require.Equal(t, baofu.UnionGWVerifyTypeRSA, query.Get("verifyType"))
	require.NotEmpty(t, query.Get("content"))
	require.Empty(t, query.Get("veryfyString"))
	plaintext, err := baofu.DecodeUnionGWVerifyType1Content(doer.baofuPublicPEM, query.Get("content"))
	require.NoError(t, err)
	var env baofu.UnionGWPlaintextEnvelope
	require.NoError(t, json.Unmarshal(plaintext, &env))
	require.Equal(t, "102004465", env.Header.MemberID)
	require.Equal(t, "200005200", env.Header.TerminalID)
	require.Equal(t, "T-1001-013-06", env.Header.ServiceType)
	require.Equal(t, baofu.UnionGWVerifyTypeRSA, env.Header.VerifyType)
	require.Contains(t, string(env.Body), `"contractNo":"CM202605040001"`)
}

type accountRecordingDoer struct {
	request         *http.Request
	requestBody     []byte
	responseBody    map[string]any
	baofuPrivatePEM string
	baofuPublicPEM  string
}

func (d *accountRecordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.request = req
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.requestBody = body
	query := req.URL.Query()
	plain, err := baofu.DecodeUnionGWVerifyType1Content(d.baofuPublicPEM, query.Get("content"))
	if err != nil {
		return nil, err
	}
	var requestEnvelope baofu.UnionGWPlaintextEnvelope
	if err := json.Unmarshal(plain, &requestEnvelope); err != nil {
		return nil, err
	}
	responsePlain, err := baofu.CanonicalJSON(baofu.UnionGWPlaintextEnvelope{
		Header: baofu.UnionGWHeader{
			MemberID:       query.Get("memberId"),
			TerminalID:     query.Get("terminalId"),
			ServiceType:    requestEnvelope.Header.ServiceType,
			SystemRespCode: baofu.UnionGWSystemRespSuccess,
			SystemRespDesc: "",
		},
		Body: mustAccountResponseRaw(d.responseBody),
	})
	if err != nil {
		return nil, err
	}
	content, err := baofu.EncodeUnionGWVerifyType1Content(d.baofuPrivatePEM, responsePlain)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(content))), Header: make(http.Header)}, nil
}

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	if recorder, ok := doer.(*accountRecordingDoer); ok {
		recorder.baofuPrivatePEM = privatePEM
		recorder.baofuPublicPEM = publicPEM
	}
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

func mustAccountResponseRaw(value map[string]any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
