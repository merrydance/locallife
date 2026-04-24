package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
)

type directPaymentRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn directPaymentRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testWeChatKey(seed string) string {
	if len(seed) >= 32 {
		return seed[:32]
	}

	return seed + strings.Repeat("!", 32-len(seed))
}

func testAPIV3Key() string {
	return testWeChatKey("wechat-direct-payment-client-test")
}

// 生成测试用的 RSA 密钥对
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey, &privateKey.PublicKey
}

// 创建测试用的私钥 PEM 文件（PKCS8 格式）
func createTestPrivateKeyFile(t *testing.T, dir string, privateKey *rsa.PrivateKey) string {
	path := filepath.Join(dir, "private_key.pem")
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	err = os.WriteFile(path, privateKeyPEM, 0600)
	require.NoError(t, err)
	return path
}

// 创建测试用的公钥 PEM 文件
func createTestPublicKeyFile(t *testing.T, dir string, publicKey *rsa.PublicKey) string {
	path := filepath.Join(dir, "platform_public_key.pem")
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	err = os.WriteFile(path, publicKeyPEM, 0644)
	require.NoError(t, err)
	return path
}

func TestNewDirectPaymentClient_WithPlatformPublicKey(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 测试使用平台公钥创建客户端
	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})

	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.platformPublicKey)
	require.Equal(t, "PUB_KEY_ID_0123456789", client.platformPublicKeyID)
}

func TestNewDirectPaymentClient_PublicKeyWithoutID_Error(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 测试：提供公钥但不提供公钥ID应该报错
	_, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		// PlatformPublicKeyID 缺失
		NotifyURL: "https://example.com/notify",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "platform public key path and ID are required")
}

func TestNewDirectPaymentClient_PublicKeyRequired(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)

	_, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:          "test_mch_id",
		AppID:          "test_app_id",
		SerialNumber:   "test_serial",
		APIV3Key:       testAPIV3Key(),
		PrivateKeyPath: privateKeyPath,
		NotifyURL:      "https://example.com/notify",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "platform public key path and ID are required")
}

func TestEncryptSensitiveData_WithPublicKey(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 创建客户端
	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	// 测试加密
	plaintext := "330123199001011234"
	ciphertext, err := client.EncryptSensitiveData(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	// 验证可以解密（使用对应的私钥）
	decrypted := decryptWithPrivateKey(t, platformPrivateKey, ciphertext)
	require.Equal(t, plaintext, decrypted)
}

func TestGetPlatformCertificateSerial_WithPublicKey(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 创建客户端
	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	// 测试获取序列号（应返回公钥ID）
	publicKeyID := client.GetPlatformPublicKeyID()
	require.Equal(t, "PUB_KEY_ID_0123456789", publicKeyID)
}

func TestGenerateJSAPIPayParams_UsesCanonicalRequestPaymentContract(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	payParams, err := client.GenerateJSAPIPayParams("prepay_id_test_001")
	require.NoError(t, err)
	require.Equal(t, JSAPIPaySignTypeRSA, payParams.SignType)
	require.Equal(t, "prepay_id=prepay_id_test_001", payParams.Package)
	require.NotEmpty(t, payParams.TimeStamp)
	require.NotEmpty(t, payParams.NonceStr)
	require.NotEmpty(t, payParams.PaySign)
}

func TestLoadPublicKey(t *testing.T) {
	// 生成测试密钥
	_, publicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()
	publicKeyPath := createTestPublicKeyFile(t, tempDir, publicKey)

	// 测试加载公钥
	loadedKey, err := loadPublicKey(publicKeyPath)
	require.NoError(t, err)
	require.NotNil(t, loadedKey)
	require.Equal(t, publicKey.N, loadedKey.N)
	require.Equal(t, publicKey.E, loadedKey.E)
}

func TestLoadPublicKey_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	invalidPath := filepath.Join(tempDir, "invalid.pem")

	// 测试文件不存在
	_, err := loadPublicKey(invalidPath)
	require.Error(t, err)

	// 测试无效 PEM 内容
	err = os.WriteFile(invalidPath, []byte("not a valid PEM"), 0644)
	require.NoError(t, err)
	_, err = loadPublicKey(invalidPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to decode PEM block")
}

// 辅助函数：使用私钥解密
func decryptWithPrivateKey(t *testing.T, privateKey *rsa.PrivateKey, ciphertextBase64 string) string {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	require.NoError(t, err)

	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey, ciphertext, nil)
	require.NoError(t, err)

	return string(plaintext)
}

func signTestHTTPResponse(t *testing.T, privateKey *rsa.PrivateKey, serial string, resp *http.Response) *http.Response {
	t.Helper()
	require.NotNil(t, resp)
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(strings.NewReader(""))
	}

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "test_response_nonce"
	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, string(body))
	hashed := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	require.NoError(t, err)

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Wechatpay-Timestamp", timestamp)
	resp.Header.Set("Wechatpay-Nonce", nonce)
	resp.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
	resp.Header.Set("Wechatpay-Serial", serial)

	return resp
}

func signNotificationForTest(t *testing.T, privateKey *rsa.PrivateKey, timestamp, nonce, body string) string {
	t.Helper()
	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, body)
	hashed := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func signedDirectPaymentTransport(t *testing.T, privateKey *rsa.PrivateKey, serial string, fn directPaymentRoundTripFunc) http.RoundTripper {
	t.Helper()
	return directPaymentRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := fn(req)
		if err != nil {
			return nil, err
		}
		return signTestHTTPResponse(t, privateKey, serial, resp), nil
	})
}

func signedEcommerceTransport(t *testing.T, privateKey *rsa.PrivateKey, serial string, fn ecommerceRoundTripFunc) http.RoundTripper {
	t.Helper()
	return ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := fn(req)
		if err != nil {
			return nil, err
		}
		return signTestHTTPResponse(t, privateKey, serial, resp), nil
	})
}

func TestQueryOrderByOutTradeNo_VerifiesResponseSignature(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedDirectPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/pay/transactions/out-trade-no/order-001", req.URL.Path)
			require.Equal(t, "mchid=test_mch_id", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"appid":"test_app_id","mchid":"test_mch_id","out_trade_no":"order-001","transaction_id":"wx_txn_001","trade_state":"SUCCESS","trade_state_desc":"支付成功"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryOrderByOutTradeNo(context.Background(), "order-001")
	require.NoError(t, err)
	require.Equal(t, "order-001", resp.OutTradeNo)
	require.Equal(t, "wx_txn_001", resp.TransactionID)
	require.Equal(t, wechatcontracts.DirectTradeStateSuccess, resp.TradeState)
}

func TestQueryOrderByOutTradeNo_MissingResponseSignatureFails(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: directPaymentRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"appid":"test_app_id"}`)),
			}, nil
		}),
	}

	_, err = client.QueryOrderByOutTradeNo(context.Background(), "order-001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify response signature")
}

func TestCreateRefund_UsesLatestDocumentedFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedDirectPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, refundURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "order-001", body["out_trade_no"])
			require.Equal(t, "refund-001", body["out_refund_no"])
			require.Equal(t, "退款原因", body["reason"])
			require.Equal(t, "AVAILABLE", body["funds_account"])

			amount, ok := body["amount"].(map[string]any)
			require.True(t, ok)
			require.EqualValues(t, 88, amount["refund"])
			require.EqualValues(t, 188, amount["total"])
			require.Equal(t, wechatcontracts.DirectRefundCurrencyCNY, amount["currency"])

			from, ok := amount["from"].([]any)
			require.True(t, ok)
			require.Len(t, from, 1)

			goodsDetail, ok := body["goods_detail"].([]any)
			require.True(t, ok)
			require.Len(t, goodsDetail, 1)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-001","transaction_id":"wx-transaction-001","out_trade_no":"order-001","channel":"ORIGINAL","user_received_account":"招商银行信用卡0403","create_time":"2025-06-06T10:34:56+08:00","status":"PROCESSING","funds_account":"AVAILABLE","amount":{"total":188,"refund":88,"from":[{"account":"AVAILABLE","amount":88}],"payer_total":188,"payer_refund":88,"settlement_refund":88,"settlement_total":188,"discount_refund":1,"currency":"CNY"},"promotion_detail":[{"promotion_id":"promo-001","scope":"SINGLE","type":"CASH","amount":1,"refund_amount":1}]}`)),
			}, nil
		}),
	}

	resp, err := client.CreateRefund(context.Background(), &RefundRequest{
		OutTradeNo:   "order-001",
		OutRefundNo:  "refund-001",
		Reason:       "退款原因",
		FundsAccount: "AVAILABLE",
		RefundAmount: 88,
		TotalAmount:  188,
		AmountFrom: []wechatcontracts.DirectRefundAmountFrom{{
			Account: "AVAILABLE",
			Amount:  88,
		}},
		GoodsDetail: []wechatcontracts.DirectRefundGoodsDetail{{
			MerchantGoodsID: "goods-001",
			GoodsName:       "可乐",
			UnitPrice:       88,
			RefundAmount:    88,
			RefundQuantity:  1,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "refund-id-001", resp.RefundID)
	require.Equal(t, RefundStatusProcessing, resp.Status)
	require.Equal(t, "AVAILABLE", resp.FundsAccount)
	require.Len(t, resp.PromotionDetail, 1)
}

func TestCreateRefund_AcceptsProcessingResponseWithoutOptionalFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedDirectPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-accepted-001","out_refund_no":"refund-accepted-001","transaction_id":"wx-transaction-accepted-001","out_trade_no":"order-accepted-001","create_time":"2026-04-24T18:50:03+08:00","status":"PROCESSING","amount":{"refund":100,"currency":"CNY"}}`)),
			}, nil
		}),
	}

	resp, err := client.CreateRefund(context.Background(), &RefundRequest{
		OutTradeNo:   "order-accepted-001",
		OutRefundNo:  "refund-accepted-001",
		Reason:       "退款原因",
		RefundAmount: 100,
		TotalAmount:  100,
	})
	require.NoError(t, err)
	require.Equal(t, "refund-id-accepted-001", resp.RefundID)
	require.Equal(t, RefundStatusProcessing, resp.Status)
	require.Equal(t, int64(100), resp.Amount.Refund)
	require.Empty(t, resp.UserReceivedAccount)
	require.Empty(t, resp.FundsAccount)
	require.Empty(t, resp.Channel)
}

func TestQueryRefund_MissingDocumentedFieldsFails(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedDirectPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-001","transaction_id":"wx-transaction-001","out_trade_no":"order-001","channel":"ORIGINAL","user_received_account":"招商银行信用卡0403","create_time":"2025-06-06T10:34:56+08:00","status":"SUCCESS","amount":{"total":188,"refund":88,"payer_total":188,"payer_refund":88,"settlement_refund":88,"settlement_total":188,"discount_refund":1,"currency":"CNY"}}`)),
			}, nil
		}),
	}

	_, err = client.QueryRefund(context.Background(), "refund-001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "funds_account is required")
}

func TestVerifyNotificationSignature_WithMatchingSerial(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	body := `{"id":"notify-001"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "test_notification_nonce"
	signature := signNotificationForTest(t, platformPrivateKey, timestamp, nonce, body)

	err = client.VerifyNotificationSignature(signature, timestamp, nonce, "PUB_KEY_ID_0123456789", body)
	require.NoError(t, err)
}

func TestVerifyNotificationSignature_SerialMismatchFails(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewDirectPaymentClient(DirectPaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              testAPIV3Key(),
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	body := `{"id":"notify-001"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "test_notification_nonce"
	signature := signNotificationForTest(t, platformPrivateKey, timestamp, nonce, body)

	err = client.VerifyNotificationSignature(signature, timestamp, nonce, "PUB_KEY_ID_OTHER", body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected notification serial")
}
