package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBuildMerchantWechatClient_PartialDirectConfigReturnsError(t *testing.T) {
	client, err := buildMerchantWechatClient(util.Config{
		WechatPayMchID: "1900000109",
	})
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_PAY_SERIAL_NUMBER")
}

func TestValidateProductionPaymentRuntime_RequiresBaofuMainBusinessInProduction(t *testing.T) {
	err := validateProductionPaymentRuntime(util.Config{Environment: "production"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "baofu main business runtime config is required in production")
}

func TestValidateProductionPaymentRuntime_RejectsDirectWechatAsMainBusinessReplacement(t *testing.T) {
	err := validateProductionPaymentRuntime(util.Config{
		Environment:                    "production",
		WechatMiniAppID:                "wx-mini-appid-001",
		WechatPayMchID:                 "1900000109",
		WechatPaySerialNumber:          "direct-serial-001",
		WechatPayPrivateKeyPath:        "./certs/apiclient_key.pem",
		WechatPayAPIV3Key:              "12345678901234567890123456789012",
		WechatPayNotifyURL:             "https://example.com/pay/notify",
		WechatPayRefundNotifyURL:       "https://example.com/pay/refund-notify",
		WechatPayPlatformPublicKeyPath: "./certs/platform.pem",
		WechatPayPlatformPublicKeyID:   "PUB_KEY_ID_DIRECT_001",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "baofu main business runtime config is required in production")
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
		BaofuSignSerialNo:           "signsn1",
		BaofuEncryptionSerialNo:     "encsn1",
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
		BaofuSignSerialNo:           "signsn1",
		BaofuEncryptionSerialNo:     "encsn1",
		BaofuNotifyBaseURL:          "https://api.example.com/v1/webhooks/baofu",
		BaofuPaymentNotifyURL:       "https://api.example.com/v1/webhooks/baofu/payment",
		BaofuProfitSharingNotifyURL: "https://api.example.com/v1/webhooks/baofu/share",
		BaofuRefundNotifyURL:        "https://api.example.com/v1/webhooks/baofu/refund",
	}
}
