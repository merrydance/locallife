package wechat

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
		APIV3Key:              "test_api_v3_key_32bytes_long__",
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
		APIV3Key:                "test_api_v3_key_32bytes_long__",
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
		APIV3Key:              "test_api_v3_key_32bytes_long__",
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
		APIV3Key:              "test_api_v3_key_32bytes_long__",
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
		APIV3Key:                "test_api_v3_key_32bytes_long__",
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
		APIV3Key:       "test_api_v3_key_32bytes_long__",
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
		APIV3Key:              "test_api_v3_key_32bytes_long__",
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
		APIV3Key:                "test_api_v3_key_32bytes_long__",
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

	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
	require.NoError(t, err)

	return string(plaintext)
}
