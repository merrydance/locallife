package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBuildEcommerceClient_UsesDedicatedPlatformPublicKey(t *testing.T) {
	merchantPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, platformPublicKey := generateMainPackageTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createMainPackageTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createMainPackageTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := buildEcommerceClient(util.Config{
		WechatEcommerceSpMchID:                 "service-mchid-001",
		WechatEcommerceSpAppID:                 "service-appid-001",
		WechatEcommerceSpSerialNumber:          "sp-serial-001",
		WechatEcommerceSpPrivateKeyPath:        privateKeyPath,
		WechatEcommerceSpAPIV3Key:              "12345678901234567890123456789012",
		WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
		WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
		WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
		WechatEcommerceWithdrawNotifyURL:       "https://example.com/ecommerce/withdraw-notify",
		WechatEcommerceViolationNotifyURL:      "https://example.com/ecommerce/violation-notify",
		WechatEcommerceSpPlatformPublicKeyPath: publicKeyPath,
		WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
	})
	require.NoError(t, err)

	require.NotNil(t, client)
	require.Equal(t, "PUB_KEY_ID_SP_001", client.GetPlatformPublicKeyID())
}

func TestBuildMerchantWechatClient_PartialDirectConfigReturnsError(t *testing.T) {
	client, err := buildMerchantWechatClient(util.Config{
		WechatPayMchID: "1900000109",
	})
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_PAY_SERIAL_NUMBER")
}

func TestBuildEcommerceClient_PartialEcommerceConfigReturnsError(t *testing.T) {
	client, err := buildEcommerceClient(util.Config{
		WechatEcommerceSpAppID: "service-appid-001",
	})
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ECOMMERCE_SP_MCHID")
}

func TestBuildEcommerceClient_MissingRequiredNotifyURLsReturnsError(t *testing.T) {
	merchantPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, platformPublicKey := generateMainPackageTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createMainPackageTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createMainPackageTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := buildEcommerceClient(util.Config{
		WechatEcommerceSpMchID:                 "service-mchid-001",
		WechatEcommerceSpAppID:                 "service-appid-001",
		WechatEcommerceSpSerialNumber:          "sp-serial-001",
		WechatEcommerceSpPrivateKeyPath:        privateKeyPath,
		WechatEcommerceSpAPIV3Key:              "12345678901234567890123456789012",
		WechatEcommerceSpPlatformPublicKeyPath: publicKeyPath,
		WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
	})
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL")
}

func TestBuildEcommerceClient_RequiresExplicitNotifyURLs(t *testing.T) {
	merchantPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, platformPublicKey := generateMainPackageTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createMainPackageTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createMainPackageTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := buildEcommerceClient(util.Config{
		WechatEcommerceSpMchID:                 "service-mchid-001",
		WechatEcommerceSpAppID:                 "service-appid-001",
		WechatEcommerceSpSerialNumber:          "sp-serial-001",
		WechatEcommerceSpPrivateKeyPath:        privateKeyPath,
		WechatEcommerceSpAPIV3Key:              "12345678901234567890123456789012",
		WechatEcommerceSpPlatformPublicKeyPath: publicKeyPath,
		WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
	})
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL")
}

func TestValidateProductionPaymentRuntime_RequiresOrdinaryServiceProviderInProduction(t *testing.T) {
	err := validateProductionPaymentRuntime(util.Config{Environment: "production"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "wechat ordinary service provider runtime config is required in production")
}

func TestValidateProductionPaymentRuntime_AllowsConfiguredProductionOrdinaryServiceProvider(t *testing.T) {
	err := validateProductionPaymentRuntime(util.Config{
		Environment:                                   "production",
		WechatMiniAppID:                               "wx-mini-appid-001",
		WechatOrdinarySpMchID:                         "service-mchid-001",
		WechatOrdinarySpAppID:                         "wx-mini-appid-001",
		WechatOrdinarySpSerialNumber:                  "sp-serial-001",
		WechatOrdinarySpPrivateKeyPath:                "./certs/sp_apiclient_key.pem",
		WechatOrdinarySpAPIV3Key:                      "12345678901234567890123456789012",
		WechatOrdinaryPaymentNotifyURL:                "https://example.com/ordinary/payment-notify",
		WechatOrdinaryCombineNotifyURL:                "https://example.com/ordinary/combine-notify",
		WechatOrdinaryRefundNotifyURL:                 "https://example.com/ordinary/refund-notify",
		WechatOrdinaryProfitSharingNotifyURL:          "https://example.com/ordinary/profit-sharing-notify",
		WechatOrdinaryViolationNotifyURL:              "https://example.com/ordinary/violation-notify",
		WechatOrdinaryApplymentSettlementIDIndividual: "719",
		WechatOrdinaryApplymentSettlementIDEnterprise: "716",
		WechatOrdinaryApplymentQualification:          "餐饮",
		WechatOrdinaryApplymentContactEmail:           "merchant@example.com",
		WechatOrdinarySpPlatformPublicKeyPath:         "./certs/sp-platform.pem",
		WechatOrdinarySpPlatformPublicKeyID:           "PUB_KEY_ID_SP_001",
	})
	require.NoError(t, err)
}

func TestValidateProductionPaymentRuntime_SkipsNonProduction(t *testing.T) {
	err := validateProductionPaymentRuntime(util.Config{Environment: "development"})
	require.NoError(t, err)
}

func generateMainPackageTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey, &privateKey.PublicKey
}

func createMainPackageTestPrivateKeyFile(t *testing.T, dir string, privateKey *rsa.PrivateKey) string {
	t.Helper()
	path := filepath.Join(dir, "private_key.pem")
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})
	err = os.WriteFile(path, privateKeyPEM, 0600)
	require.NoError(t, err)
	return path
}

func createMainPackageTestPublicKeyFile(t *testing.T, dir string, publicKey *rsa.PublicKey) string {
	t.Helper()
	path := filepath.Join(dir, "platform_public_key.pem")
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes})
	err = os.WriteFile(path, publicKeyPEM, 0644)
	require.NoError(t, err)
	return path
}

func TestBuildBaofuAggregateClient_UsesRuntimeConfig(t *testing.T) {
	privateKey, publicKey := generateMainPackageTestKeyPair(t)
	client, err := buildBaofuAggregateClient(util.Config{
		BaofuMainBusinessEnabled:    true,
		WechatMiniAppID:             "wx1234567890abcdef",
		BaofuCollectMerchantID:      "102004465",
		BaofuCollectTerminalID:      "200005200",
		BaofuPayoutMerchantID:       "102004466",
		BaofuPayoutTerminalID:       "200005201",
		BaofuPrivateKeyPEM:          mainPackagePrivateKeyPEM(t, privateKey),
		BaofuPublicKeyPEM:           mainPackagePublicKeyPEM(t, publicKey),
		BaofuSignSerialNo:           "test-sign-sn",
		BaofuEncryptionSerialNo:     "test-enc-sn",
		BaofuNotifyBaseURL:          "https://api.example.com/v1/webhooks/baofu",
		BaofuPaymentNotifyURL:       "https://api.example.com/v1/webhooks/baofu/payment",
		BaofuProfitSharingNotifyURL: "https://api.example.com/v1/webhooks/baofu/share",
		BaofuRefundNotifyURL:        "https://api.example.com/v1/webhooks/baofu/refund",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func mainPackagePrivateKeyPEM(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}))
}

func mainPackagePublicKeyPEM(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}))
}

func TestValidateProductionPaymentRuntime_AllowsConfiguredBaofuMainBusiness(t *testing.T) {
	privateKey, publicKey := generateMainPackageTestKeyPair(t)
	err := validateProductionPaymentRuntime(validMainPackageBaofuConfig(t, privateKey, publicKey))
	require.NoError(t, err)
}

func validMainPackageBaofuConfig(t *testing.T, privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) util.Config {
	t.Helper()
	return util.Config{
		Environment:                 "production",
		BaofuMainBusinessEnabled:    true,
		WechatMiniAppID:             "wx1234567890abcdef",
		BaofuCollectMerchantID:      "102004465",
		BaofuCollectTerminalID:      "200005200",
		BaofuPayoutMerchantID:       "102004466",
		BaofuPayoutTerminalID:       "200005201",
		BaofuPrivateKeyPEM:          mainPackagePrivateKeyPEM(t, privateKey),
		BaofuPublicKeyPEM:           mainPackagePublicKeyPEM(t, publicKey),
		BaofuSignSerialNo:           "test-sign-sn",
		BaofuEncryptionSerialNo:     "test-enc-sn",
		BaofuNotifyBaseURL:          "https://api.example.com/v1/webhooks/baofu",
		BaofuPaymentNotifyURL:       "https://api.example.com/v1/webhooks/baofu/payment",
		BaofuProfitSharingNotifyURL: "https://api.example.com/v1/webhooks/baofu/share",
		BaofuRefundNotifyURL:        "https://api.example.com/v1/webhooks/baofu/refund",
	}
}
