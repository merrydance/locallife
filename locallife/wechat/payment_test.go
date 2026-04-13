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
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type paymentRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn paymentRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testWeChatKey(seed string) string {
	if len(seed) >= 32 {
		return seed[:32]
	}

	return seed + strings.Repeat("!", 32-len(seed))
}

func testAPIV3Key() string {
	return testWeChatKey("wechat-pay-client-test")
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

// 创建测试用的证书 PEM 文件
func createTestCertificateFile(t *testing.T, dir string, privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) string {
	path := filepath.Join(dir, "platform_certificate.pem")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1234567890),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(5 * 365 * 24 * time.Hour), // 5 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	err = os.WriteFile(path, certPEM, 0644)
	require.NoError(t, err)
	return path
}

func TestNewPaymentClient_WithPlatformPublicKey(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 测试使用平台公钥创建客户端
	client, err := NewPaymentClient(PaymentClientConfig{
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
	require.Nil(t, client.platformCertificate)
	require.Equal(t, "PUB_KEY_ID_0123456789", client.platformPublicKeyID)
}

func TestNewPaymentClient_WithPlatformCertificate(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	certPath := createTestCertificateFile(t, tempDir, platformPrivateKey, platformPublicKey)

	// 测试使用平台证书创建客户端（旧方式）
	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:                   "test_mch_id",
		AppID:                   "test_app_id",
		SerialNumber:            "test_serial",
		APIV3Key:                testAPIV3Key(),
		PrivateKeyPath:          privateKeyPath,
		PlatformCertificatePath: certPath,
		NotifyURL:               "https://example.com/notify",
	})

	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.platformCertificate)
	require.Nil(t, client.platformPublicKey)
}

func TestNewPaymentClient_PublicKeyWithoutID_Error(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	// 测试：提供公钥但不提供公钥ID应该报错
	_, err := NewPaymentClient(PaymentClientConfig{
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
	require.Contains(t, err.Error(), "platform public key ID is required")
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
	client, err := NewPaymentClient(PaymentClientConfig{
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

func TestEncryptSensitiveData_WithCertificate(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	certPath := createTestCertificateFile(t, tempDir, platformPrivateKey, platformPublicKey)

	// 创建客户端（使用证书）
	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:                   "test_mch_id",
		AppID:                   "test_app_id",
		SerialNumber:            "test_serial",
		APIV3Key:                testAPIV3Key(),
		PrivateKeyPath:          privateKeyPath,
		PlatformCertificatePath: certPath,
		NotifyURL:               "https://example.com/notify",
	})
	require.NoError(t, err)

	// 测试加密
	plaintext := "330123199001011234"
	ciphertext, err := client.EncryptSensitiveData(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	// 验证可以解密
	decrypted := decryptWithPrivateKey(t, platformPrivateKey, ciphertext)
	require.Equal(t, plaintext, decrypted)
}

func TestEncryptSensitiveData_NoCertOrKey_Error(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)

	// 创建客户端（不提供证书或公钥）
	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:          "test_mch_id",
		AppID:          "test_app_id",
		SerialNumber:   "test_serial",
		APIV3Key:       testAPIV3Key(),
		PrivateKeyPath: privateKeyPath,
		NotifyURL:      "https://example.com/notify",
	})
	require.NoError(t, err)

	// 测试加密应该失败
	_, err = client.EncryptSensitiveData("test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "neither platform public key nor platform certificate loaded")
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
	client, err := NewPaymentClient(PaymentClientConfig{
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
	serial := client.GetPlatformCertificateSerial()
	require.Equal(t, "PUB_KEY_ID_0123456789", serial)
}

func TestGetPlatformCertificateSerial_WithCertificate(t *testing.T) {
	// 生成测试密钥
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试文件
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	certPath := createTestCertificateFile(t, tempDir, platformPrivateKey, platformPublicKey)

	// 创建客户端
	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:                   "test_mch_id",
		AppID:                   "test_app_id",
		SerialNumber:            "test_serial",
		APIV3Key:                testAPIV3Key(),
		PrivateKeyPath:          privateKeyPath,
		PlatformCertificatePath: certPath,
		NotifyURL:               "https://example.com/notify",
	})
	require.NoError(t, err)

	// 测试获取序列号（应返回证书序列号）
	serial := client.GetPlatformCertificateSerial()
	require.NotEmpty(t, serial)
	require.Equal(t, "499602D2", serial) // 1234567890 的十六进制
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

func signedPaymentTransport(t *testing.T, privateKey *rsa.PrivateKey, serial string, fn paymentRoundTripFunc) http.RoundTripper {
	t.Helper()
	return paymentRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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

	client, err := NewPaymentClient(PaymentClientConfig{
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
		Transport: signedPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/pay/transactions/out-trade-no/order-001", req.URL.Path)
			require.Equal(t, "mchid=test_mch_id", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"appid":"test_app_id","mchid":"test_mch_id","out_trade_no":"order-001","transaction_id":"wx_txn_001","trade_state":"SUCCESS"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryOrderByOutTradeNo(context.Background(), "order-001")
	require.NoError(t, err)
	require.Equal(t, "order-001", resp.OutTradeNo)
	require.Equal(t, "wx_txn_001", resp.TransactionID)
	require.Equal(t, TradeStateSuccess, resp.TradeState)
}

func TestQueryOrderByOutTradeNo_MissingResponseSignatureFails(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewPaymentClient(PaymentClientConfig{
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
		Transport: paymentRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
