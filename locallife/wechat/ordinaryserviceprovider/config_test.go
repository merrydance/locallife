package ordinaryserviceprovider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testAPIV3Key() string {
	seed := "ordinary-service-provider-test"
	return seed + strings.Repeat("!", 32-len(seed))
}

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey, &privateKey.PublicKey
}

func createTestPrivateKeyFile(t *testing.T, dir string, privateKey *rsa.PrivateKey) string {
	t.Helper()
	path := filepath.Join(dir, "private_key.pem")
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})
	require.NoError(t, os.WriteFile(path, privateKeyPEM, 0600))
	return path
}

func createTestPublicKeyFile(t *testing.T, dir string, publicKey *rsa.PublicKey) string {
	t.Helper()
	path := filepath.Join(dir, "platform_public_key.pem")
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes})
	require.NoError(t, os.WriteFile(path, publicKeyPEM, 0644))
	return path
}

func validConfig(t *testing.T) Config {
	t.Helper()
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)
	tempDir := t.TempDir()

	return Config{
		ServiceProviderAppID:       "wxspappid123",
		ServiceProviderMchID:       "1900000109",
		ServiceProviderMchName:     "LocalLife Service Provider",
		CertificateSerialNumber:    "4F5E6D7C8B9A00112233445566778899",
		PrivateKeyPath:             createTestPrivateKeyFile(t, tempDir, merchantPrivateKey),
		APIV3Key:                   testAPIV3Key(),
		PlatformPublicKeyPath:      createTestPublicKeyFile(t, tempDir, platformPublicKey),
		PlatformPublicKeyID:        "PUB_KEY_ID_0112233445566778899",
		BaseURL:                    DefaultBaseURL,
		PaymentNotifyURL:           "https://api.example.com/wechat/ordinary/payment",
		CombineNotifyURL:           "https://api.example.com/wechat/ordinary/combine",
		RefundNotifyURL:            "https://api.example.com/wechat/ordinary/refund",
		ProfitSharingNotifyURL:     "https://api.example.com/wechat/ordinary/profit-sharing",
		MerchantViolationNotifyURL: "https://api.example.com/wechat/ordinary/merchant-violation",
		HTTPTimeout:                5 * time.Second,
	}
}

func TestValidateConfigRequiresExplicitServiceProviderIdentity(t *testing.T) {
	cfg := validConfig(t)
	cfg.ServiceProviderMchID = "  "

	err := cfg.Validate()

	require.ErrorContains(t, err, "service_provider_mchid is required")
}

func TestValidateConfigRejectsNonHTTPSNotifyURL(t *testing.T) {
	cfg := validConfig(t)
	cfg.PaymentNotifyURL = "http://api.example.com/wechat/ordinary/payment"

	err := cfg.Validate()

	require.ErrorContains(t, err, "payment_notify_url must use https")
}

func TestNewOrdinaryServiceProviderClientStoresValidatedConfig(t *testing.T) {
	cfg := validConfig(t)

	client, err := NewOrdinaryServiceProviderClient(cfg, WithSDKClientFactory(func(Config) (sdkClient, error) {
		return nil, nil
	}))

	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, "1900000109", client.ServiceProviderMchID())
	require.Equal(t, "wxspappid123", client.ServiceProviderAppID())
}
