package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/stretchr/testify/require"
)

func writeTestConfigFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "app.env")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)
	return dir
}

func testWeChatKey(seed string) string {
	if len(seed) >= 32 {
		return seed[:32]
	}

	return seed + strings.Repeat("!", 32-len(seed))
}

func testWechatPayAPIV3Key() string {
	return testWeChatKey("wechat-pay-config-test")
}

func testWechatEcommerceAPIV3Key() string {
	return testWeChatKey("wechat-ecommerce-config-test")
}

func TestLoadConfig_DefaultsAndTrimQuotes(t *testing.T) {
	configDir := writeTestConfigFile(t, "ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nREDIS_PASSWORD=\"quoted-secret\"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "test", config.Environment)
	require.Equal(t, "postgresql:///test", config.DBSource)
	require.Equal(t, "file://db/migration", config.MigrationURL)
	require.Equal(t, "quoted-secret", config.RedisPassword)
	require.False(t, config.RedisRequired)
	require.Equal(t, int32(25), config.DBMaxConns)
	require.Equal(t, int32(5), config.DBMinConns)
	require.Equal(t, time.Hour, config.DBMaxConnLifetime)
	require.Equal(t, 30*time.Minute, config.DBMaxConnIdleTime)
	require.Equal(t, time.Minute, config.DBHealthCheckPeriod)
	require.True(t, config.WebSocketReliableEnabled)
	require.Equal(t, 100, config.WebSocketReliablePercent)
	require.Equal(t, 80, config.GeofenceRadiusMeters)
	require.Equal(t, 60, config.GeofenceDwellMinSeconds)
	require.Equal(t, 3, config.GeofenceDwellMinSamples)
	require.Equal(t, 80, config.GeofenceMinAccuracyMeters)
	require.False(t, config.GeofenceAutoAdvanceEnabled)
	require.False(t, config.GeofenceAutoPickupEnabled)
	require.False(t, config.GeofenceAutoDeliverEnabled)
	require.Equal(t, 15000, config.RiderAverageSpeed)
	require.Equal(t, 20, config.DefaultPrepareTime)
	require.Equal(t, time.Minute, config.ProfitSharingReturnRetryInterval)
	require.Equal(t, 10, config.ProfitSharingReturnMaxRetries)
}

func TestLoadConfig_ReadsWechatPaymentAndEcommerceConfig(t *testing.T) {
	payKey := testWechatPayAPIV3Key()
	spKey := testWechatEcommerceAPIV3Key()
	configDir := writeTestConfigFile(t, fmt.Sprintf("ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nWECHAT_MINI_APP_ID=wx-mini-app\nWECHAT_MINI_APP_SECRET=mini-secret\nWECHAT_PAY_MCH_ID=1900000109\nWECHAT_PAY_SERIAL_NUMBER=serial-001\nWECHAT_PAY_PRIVATE_KEY_PATH=./certs/apiclient_key.pem\nWECHAT_PAY_API_V3_KEY=%s\nWECHAT_PAY_NOTIFY_URL=https://example.com/pay/notify\nWECHAT_PAY_REFUND_NOTIFY_URL=https://example.com/pay/refund-notify\nWECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL=https://example.com/pay/merchant-transfer-notify\nWECHAT_SHIPPING_SETTLE_NOTIFY_URL=https://example.com/pay/settlement-notify\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH=./certs/platform.pem\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_001\nWECHAT_PAY_HTTP_TIMEOUT=45s\nWECHAT_ECOMMERCE_SP_MCHID=service-mchid-001\nWECHAT_ECOMMERCE_SP_APPID=service-appid-001\nWECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL=https://example.com/ecommerce/payment-notify\nWECHAT_ECOMMERCE_COMBINE_NOTIFY_URL=https://example.com/ecommerce/combine-notify\nWECHAT_ECOMMERCE_REFUND_NOTIFY_URL=https://example.com/ecommerce/refund-notify\nWECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL=https://example.com/ecommerce/withdraw-notify\nWECHAT_ECOMMERCE_VIOLATION_NOTIFY_URL=https://example.com/ecommerce/violation-notify\nWECHAT_ECOMMERCE_SP_NAME=测试平台服务商\nWECHAT_ECOMMERCE_SP_SERIAL_NUMBER=sp-serial-001\nWECHAT_ECOMMERCE_SP_PRIVATE_KEY_PATH=./certs/sp_apiclient_key.pem\nWECHAT_ECOMMERCE_SP_API_V3_KEY=%s\nWECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH=./certs/sp-platform.pem\nWECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_SP_001\nREDIS_REQUIRED=true\n", payKey, spKey))

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "wx-mini-app", config.WechatMiniAppID)
	require.Equal(t, "mini-secret", config.WechatMiniAppSecret)
	require.Equal(t, "1900000109", config.WechatPayMchID)
	require.Equal(t, "serial-001", config.WechatPaySerialNumber)
	require.Equal(t, "./certs/apiclient_key.pem", config.WechatPayPrivateKeyPath)
	require.Equal(t, payKey, config.WechatPayAPIV3Key)
	require.Equal(t, "https://example.com/pay/notify", config.WechatPayNotifyURL)
	require.Equal(t, "https://example.com/pay/refund-notify", config.WechatPayRefundNotifyURL)
	require.Equal(t, "https://example.com/pay/merchant-transfer-notify", config.WechatPayMerchantTransferNotifyURL)
	require.Equal(t, "https://example.com/pay/settlement-notify", config.WechatShippingSettleNotifyURL)
	require.Equal(t, "./certs/platform.pem", config.WechatPayPlatformPublicKeyPath)
	require.Equal(t, "PUB_KEY_ID_001", config.WechatPayPlatformPublicKeyID)
	require.Equal(t, 45*time.Second, config.WechatPayHTTPTimeout)
	require.Equal(t, "service-mchid-001", config.WechatEcommerceSpMchID)
	require.Equal(t, "service-appid-001", config.WechatEcommerceSpAppID)
	require.Equal(t, "https://example.com/ecommerce/payment-notify", config.WechatEcommercePaymentNotifyURL)
	require.Equal(t, "https://example.com/ecommerce/combine-notify", config.WechatEcommerceCombineNotifyURL)
	require.Equal(t, "https://example.com/ecommerce/refund-notify", config.WechatEcommerceRefundNotifyURL)
	require.Equal(t, "https://example.com/ecommerce/withdraw-notify", config.WechatEcommerceWithdrawNotifyURL)
	require.Equal(t, "https://example.com/ecommerce/violation-notify", config.WechatEcommerceViolationNotifyURL)
	require.Equal(t, "测试平台服务商", config.WechatEcommerceSpName)
	require.Equal(t, "sp-serial-001", config.WechatEcommerceSpSerialNumber)
	require.Equal(t, "./certs/sp_apiclient_key.pem", config.WechatEcommerceSpPrivateKeyPath)
	require.Equal(t, spKey, config.WechatEcommerceSpAPIV3Key)
	require.Equal(t, "./certs/sp-platform.pem", config.WechatEcommerceSpPlatformPublicKeyPath)
	require.Equal(t, "PUB_KEY_ID_SP_001", config.WechatEcommerceSpPlatformPublicKeyID)
	require.True(t, config.RedisRequired)
}

func TestLoadConfig_ReadsWechatOrdinaryApplymentSettlementConfig(t *testing.T) {
	configDir := writeTestConfigFile(t, strings.Join([]string{
		"ENVIRONMENT=test",
		"DB_SOURCE=postgresql:///test",
		"MIGRATION_URL=file://db/migration",
		"WECHAT_ORDINARY_APPLYMENT_QUALIFICATION_TYPE=餐饮",
		"WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_INDIVIDUAL=719",
		"WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE=716",
		"WECHAT_ORDINARY_APPLYMENT_ACTIVITIES_ID=20191030111cff5b5e",
		"WECHAT_ORDINARY_APPLYMENT_DEBIT_ACTIVITIES_RATE=0.38",
		"WECHAT_ORDINARY_APPLYMENT_CREDIT_ACTIVITIES_RATE=0.38",
		"WECHAT_ORDINARY_APPLYMENT_ACTIVITIES_ADDITIONS=media-a, media-b",
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "餐饮", config.WechatOrdinaryApplymentQualification)
	require.Equal(t, "719", config.WechatOrdinaryApplymentSettlementIDIndividual)
	require.Equal(t, "716", config.WechatOrdinaryApplymentSettlementIDEnterprise)
	require.Equal(t, "20191030111cff5b5e", config.WechatOrdinaryApplymentActivitiesID)
	require.Equal(t, "0.38", config.WechatOrdinaryApplymentDebitActivitiesRate)
	require.Equal(t, "0.38", config.WechatOrdinaryApplymentCreditActivitiesRate)
	require.Equal(t, "media-a, media-b", config.WechatOrdinaryApplymentActivitiesAdditions)
}

func TestLoadConfig_ReadsBaofuMainBusinessConfig(t *testing.T) {
	configDir := writeTestConfigFile(t, strings.Join([]string{
		"ENVIRONMENT=test",
		"DB_SOURCE=postgresql:///test",
		"MIGRATION_URL=file://db/migration",
		"WECHAT_MINI_APP_ID=wx-local-life",
		"BAOFU_MAIN_BUSINESS_ENABLED=true",
		"BAOFU_COLLECT_MERCHANT_ID=COLLECT_MER",
		"BAOFU_COLLECT_TERMINAL_ID=COLLECT_TER",
		"BAOFU_PAYOUT_MERCHANT_ID=PAYOUT_MER",
		"BAOFU_PAYOUT_TERMINAL_ID=PAYOUT_TER",
		"BAOFU_APP_ID=baofu-app",
		"BAOFU_PRIVATE_KEY_PEM=test-private-key",
		"BAOFU_PUBLIC_KEY_PEM=test-public-key",
		"BAOFU_SIGN_SERIAL_NO=sign-sn",
		"BAOFU_ENCRYPTION_SERIAL_NO=enc-sn",
		"BAOFU_AES_KEY=0123456789abcdef0123456789abcdef",
		"BAOFU_NOTIFY_BASE_URL=https://api.example.com/v1/webhooks/baofu",
		"BAOFU_PAYMENT_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/payment",
		"BAOFU_REFUND_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/refund",
		"BAOFU_HTTP_TIMEOUT=12s",
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)
	require.True(t, config.BaofuMainBusinessEnabled)
	require.True(t, config.HasBaofuRuntimeConfig())
	require.NoError(t, config.ValidateBaofuConfig())
	require.Equal(t, "wx-local-life", config.WechatMiniAppID)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/payment", config.EffectiveBaofuPaymentNotifyURL())
	require.Equal(t, 12*time.Second, config.BaofuHTTPTimeout)

	baofuConfig := config.ToBaofuConfig().Normalized()
	require.Equal(t, baofu.SandboxAggregatePayBaseURL, baofuConfig.AggregatePayBaseURL)
	require.Equal(t, "COLLECT_MER", baofuConfig.CollectMerchantID)
	require.Equal(t, "PAYOUT_MER", baofuConfig.PayoutMerchantID)
}

func TestEffectiveWechatEcommerceNotifyURLs(t *testing.T) {
	config := Config{
		WechatEcommercePaymentNotifyURL:    "https://example.com/v1/webhooks/wechat-ecommerce/payment-notify",
		WechatEcommerceCombineNotifyURL:    "https://example.com/v1/webhooks/wechat-ecommerce/combine-notify",
		WechatEcommerceRefundNotifyURL:     "https://example.com/v1/webhooks/wechat-ecommerce/refund-notify",
		WechatEcommerceWithdrawNotifyURL:   "https://example.com/v1/webhooks/wechat-ecommerce/withdraw-notify",
		WechatEcommerceViolationNotifyURL:  "https://example.com/v1/webhooks/wechat-ecommerce/violation-notify",
		WechatPayMerchantTransferNotifyURL: "https://example.com/v1/webhooks/wechat-pay/merchant-transfer-notify",
	}

	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/payment-notify", config.EffectiveWechatEcommercePaymentNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/combine-notify", config.EffectiveWechatEcommerceCombineNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/refund-notify", config.EffectiveWechatEcommerceRefundNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/withdraw-notify", config.EffectiveWechatEcommerceWithdrawNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/violation-notify", config.EffectiveWechatEcommerceViolationNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-pay/merchant-transfer-notify", config.EffectiveWechatPayMerchantTransferNotifyURL())

	config.WechatEcommercePaymentNotifyURL = "https://override.example.com/ecommerce/payment-notify"
	config.WechatEcommerceCombineNotifyURL = "https://override.example.com/ecommerce/combine-notify"
	config.WechatEcommerceRefundNotifyURL = "https://override.example.com/ecommerce/refund-notify"
	config.WechatEcommerceWithdrawNotifyURL = "https://override.example.com/ecommerce/withdraw-notify"
	config.WechatEcommerceViolationNotifyURL = "https://override.example.com/ecommerce/violation-notify"
	config.WechatPayMerchantTransferNotifyURL = "https://override.example.com/pay/merchant-transfer-notify"

	require.Equal(t, "https://override.example.com/ecommerce/payment-notify", config.EffectiveWechatEcommercePaymentNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/combine-notify", config.EffectiveWechatEcommerceCombineNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/refund-notify", config.EffectiveWechatEcommerceRefundNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/withdraw-notify", config.EffectiveWechatEcommerceWithdrawNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/violation-notify", config.EffectiveWechatEcommerceViolationNotifyURL())
	require.Equal(t, "https://override.example.com/pay/merchant-transfer-notify", config.EffectiveWechatPayMerchantTransferNotifyURL())
}

func TestValidateWechatEcommerceConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "disabled skips validation",
			config: Config{},
		},
		{
			name: "same merchant still requires dedicated credentials",
			config: Config{
				WechatPayMchID:          "1900000109",
				WechatPayPrivateKeyPath: "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:  "1900000109",
				WechatEcommerceSpAppID:  "wx-app",
			},
			want: "WECHAT_ECOMMERCE_SP_SERIAL_NUMBER",
		},
		{
			name: "different merchant requires dedicated credentials",
			config: Config{
				WechatPayMchID:          "1900000109",
				WechatPayPrivateKeyPath: "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:  "service-mchid-001",
				WechatEcommerceSpAppID:  "service-appid-001",
			},
			want: "WECHAT_ECOMMERCE_SP_SERIAL_NUMBER",
		},
		{
			name: "ecommerce payment notify url is required without fallback",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
			},
			want: "WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL",
		},
		{
			name: "ecommerce combine notify url is required",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
				WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
			},
			want: "WECHAT_ECOMMERCE_COMBINE_NOTIFY_URL",
		},
		{
			name: "ecommerce payment notify url cannot fall back to direct notify url",
			config: Config{
				WechatPayNotifyURL:                     "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:               "https://example.com/pay/refund-notify",
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
			},
			want: "WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL",
		},
		{
			name: "ecommerce payment notify url must be absolute",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "/ecommerce/payment-notify",
				WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
				WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
				WechatEcommerceWithdrawNotifyURL:       "https://example.com/ecommerce/withdraw-notify",
				WechatEcommerceViolationNotifyURL:      "https://example.com/ecommerce/violation-notify",
			},
			want: "WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL must be a valid absolute URL",
		},
		{
			name: "ecommerce refund notify url must be absolute",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
				WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
				WechatEcommerceRefundNotifyURL:         "/ecommerce/refund-notify",
				WechatEcommerceWithdrawNotifyURL:       "https://example.com/ecommerce/withdraw-notify",
				WechatEcommerceViolationNotifyURL:      "https://example.com/ecommerce/violation-notify",
			},
			want: "WECHAT_ECOMMERCE_REFUND_NOTIFY_URL must be a valid absolute URL",
		},
		{
			name: "ecommerce withdraw notify url is required and must be absolute",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
				WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
				WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
				WechatEcommerceWithdrawNotifyURL:       "/ecommerce/withdraw-notify",
				WechatEcommerceViolationNotifyURL:      "https://example.com/ecommerce/violation-notify",
			},
			want: "WECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL must be a valid absolute URL",
		},
		{
			name: "ecommerce violation notify url is required",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
				WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
				WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
				WechatEcommerceWithdrawNotifyURL:       "https://example.com/ecommerce/withdraw-notify",
			},
			want: "WECHAT_ECOMMERCE_VIOLATION_NOTIFY_URL",
		},
		{
			name: "platform public key pair must be complete",
			config: Config{
				WechatPayMchID:                         "1900000109",
				WechatPayPrivateKeyPath:                "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
			},
			want: "WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH",
		},
		{
			name: "platform public key pair is required",
			config: Config{
				WechatPayMchID:                  "1900000109",
				WechatPayPrivateKeyPath:         "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:          "service-mchid-001",
				WechatEcommerceSpAppID:          "service-appid-001",
				WechatEcommerceSpSerialNumber:   "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath: "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:       testWechatEcommerceAPIV3Key(),
			},
			want: "WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH",
		},
		{
			name: "full ecommerce runtime config passes",
			config: Config{
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              testWechatEcommerceAPIV3Key(),
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
				WechatEcommerceSpPlatformPublicKeyID:   "PUB_KEY_ID_SP_001",
				WechatEcommercePaymentNotifyURL:        "https://example.com/ecommerce/payment-notify",
				WechatEcommerceCombineNotifyURL:        "https://example.com/ecommerce/combine-notify",
				WechatEcommerceRefundNotifyURL:         "https://example.com/ecommerce/refund-notify",
				WechatEcommerceWithdrawNotifyURL:       "https://example.com/ecommerce/withdraw-notify",
				WechatEcommerceViolationNotifyURL:      "https://example.com/ecommerce/violation-notify",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.ValidateWechatEcommerceConfig()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateWechatPayConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "disabled skips validation",
			config: Config{},
		},
		{
			name: "direct pay requires dedicated credentials",
			config: Config{
				WechatPayMchID: "1900000109",
			},
			want: "WECHAT_PAY_SERIAL_NUMBER",
		},
		{
			name: "direct pay requires mini app id",
			config: Config{
				WechatPayMchID:                 "1900000109",
				WechatPaySerialNumber:          "serial-001",
				WechatPayPrivateKeyPath:        "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:              testWechatPayAPIV3Key(),
				WechatPayNotifyURL:             "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:       "https://example.com/pay/refund-notify",
				WechatPayPlatformPublicKeyPath: "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:   "PUB_KEY_ID_001",
			},
			want: "WECHAT_MINI_APP_ID",
		},
		{
			name: "direct pay requires payment notify url",
			config: Config{
				WechatMiniAppID:                "wx-mini-app",
				WechatPayMchID:                 "1900000109",
				WechatPaySerialNumber:          "serial-001",
				WechatPayPrivateKeyPath:        "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:              testWechatPayAPIV3Key(),
				WechatPayRefundNotifyURL:       "https://example.com/pay/refund-notify",
				WechatPayPlatformPublicKeyPath: "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:   "PUB_KEY_ID_001",
			},
			want: "WECHAT_PAY_NOTIFY_URL",
		},
		{
			name: "direct pay requires refund notify url",
			config: Config{
				WechatMiniAppID:                "wx-mini-app",
				WechatPayMchID:                 "1900000109",
				WechatPaySerialNumber:          "serial-001",
				WechatPayPrivateKeyPath:        "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:              testWechatPayAPIV3Key(),
				WechatPayNotifyURL:             "https://example.com/pay/notify",
				WechatPayPlatformPublicKeyPath: "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:   "PUB_KEY_ID_001",
			},
			want: "WECHAT_PAY_REFUND_NOTIFY_URL",
		},
		{
			name: "direct pay requires merchant transfer notify url",
			config: Config{
				WechatMiniAppID:                "wx-mini-app",
				WechatPayMchID:                 "1900000109",
				WechatPaySerialNumber:          "serial-001",
				WechatPayPrivateKeyPath:        "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:              testWechatPayAPIV3Key(),
				WechatPayNotifyURL:             "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:       "https://example.com/pay/refund-notify",
				WechatPayPlatformPublicKeyPath: "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:   "PUB_KEY_ID_001",
			},
			want: "WECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL",
		},
		{
			name: "direct pay notify url must be absolute",
			config: Config{
				WechatMiniAppID:                    "wx-mini-app",
				WechatPayMchID:                     "1900000109",
				WechatPaySerialNumber:              "serial-001",
				WechatPayPrivateKeyPath:            "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:                  testWechatPayAPIV3Key(),
				WechatPayNotifyURL:                 "/pay/notify",
				WechatPayRefundNotifyURL:           "https://example.com/pay/refund-notify",
				WechatPayMerchantTransferNotifyURL: "https://example.com/pay/merchant-transfer-notify",
				WechatPayPlatformPublicKeyPath:     "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:       "PUB_KEY_ID_001",
			},
			want: "WECHAT_PAY_NOTIFY_URL must be a valid absolute URL",
		},
		{
			name: "merchant transfer notify url must be absolute",
			config: Config{
				WechatMiniAppID:                    "wx-mini-app",
				WechatPayMchID:                     "1900000109",
				WechatPaySerialNumber:              "serial-001",
				WechatPayPrivateKeyPath:            "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:                  testWechatPayAPIV3Key(),
				WechatPayNotifyURL:                 "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:           "https://example.com/pay/refund-notify",
				WechatPayMerchantTransferNotifyURL: "/pay/merchant-transfer-notify",
				WechatPayPlatformPublicKeyPath:     "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:       "PUB_KEY_ID_001",
			},
			want: "WECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL must be a valid absolute URL",
		},
		{
			name: "platform public key pair must be complete",
			config: Config{
				WechatMiniAppID:                    "wx-mini-app",
				WechatPayMchID:                     "1900000109",
				WechatPaySerialNumber:              "serial-001",
				WechatPayPrivateKeyPath:            "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:                  testWechatPayAPIV3Key(),
				WechatPayNotifyURL:                 "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:           "https://example.com/pay/refund-notify",
				WechatPayMerchantTransferNotifyURL: "https://example.com/pay/merchant-transfer-notify",
				WechatPayPlatformPublicKeyPath:     "./certs/platform.pem",
			},
			want: "WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH",
		},
		{
			name: "platform public key pair is required",
			config: Config{
				WechatMiniAppID:                    "wx-mini-app",
				WechatPayMchID:                     "1900000109",
				WechatPaySerialNumber:              "serial-001",
				WechatPayPrivateKeyPath:            "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:                  testWechatPayAPIV3Key(),
				WechatPayNotifyURL:                 "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:           "https://example.com/pay/refund-notify",
				WechatPayMerchantTransferNotifyURL: "https://example.com/pay/merchant-transfer-notify",
			},
			want: "WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH",
		},
		{
			name: "full merchant credential set passes",
			config: Config{
				WechatMiniAppID:                    "wx-mini-app",
				WechatPayMchID:                     "1900000109",
				WechatPaySerialNumber:              "serial-001",
				WechatPayPrivateKeyPath:            "./certs/apiclient_key.pem",
				WechatPayAPIV3Key:                  testWechatPayAPIV3Key(),
				WechatPayNotifyURL:                 "https://example.com/pay/notify",
				WechatPayRefundNotifyURL:           "https://example.com/pay/refund-notify",
				WechatPayMerchantTransferNotifyURL: "https://example.com/pay/merchant-transfer-notify",
				WechatPayPlatformPublicKeyPath:     "./certs/platform.pem",
				WechatPayPlatformPublicKeyID:       "PUB_KEY_ID_001",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.ValidateWechatPayConfig()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateWechatOrdinaryServiceProviderConfigRejectsMiniAppMismatch(t *testing.T) {
	config := Config{
		WechatMiniAppID:                               "wx-mini-app",
		WechatOrdinarySpMchID:                         "1900000109",
		WechatOrdinarySpAppID:                         "wx-other-app",
		WechatOrdinarySpName:                          "普通服务商",
		WechatOrdinarySpSerialNumber:                  "ordinary-serial-001",
		WechatOrdinarySpPrivateKeyPath:                "./certs/ordinary_key.pem",
		WechatOrdinarySpAPIV3Key:                      testWeChatKey("ordinary-service-provider"),
		WechatOrdinarySpPlatformPublicKeyPath:         "./certs/ordinary-platform.pem",
		WechatOrdinarySpPlatformPublicKeyID:           "PUB_KEY_ID_ORDINARY_001",
		WechatOrdinaryPaymentNotifyURL:                "https://example.com/ordinary/payment",
		WechatOrdinaryCombineNotifyURL:                "https://example.com/ordinary/combine",
		WechatOrdinaryRefundNotifyURL:                 "https://example.com/ordinary/refund",
		WechatOrdinaryProfitSharingNotifyURL:          "https://example.com/ordinary/profit-sharing",
		WechatOrdinaryViolationNotifyURL:              "https://example.com/ordinary/violation",
		WechatOrdinaryApplymentSettlementIDIndividual: "719",
		WechatOrdinaryApplymentSettlementIDEnterprise: "716",
		WechatOrdinaryApplymentQualification:          "餐饮",
		WechatOrdinaryApplymentContactEmail:           "ops@example.com",
		WechatOrdinaryApplymentServicePhone:           "13800138000",
	}

	err := config.ValidateWechatOrdinaryServiceProviderConfig()

	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ORDINARY_SP_APPID must match WECHAT_MINI_APP_ID")
}

func TestValidateWechatOrdinaryServiceProviderConfigRequiresSubjectSettlementIDs(t *testing.T) {
	config := Config{
		WechatMiniAppID:                               "wx-mini-app",
		WechatOrdinarySpMchID:                         "1900000109",
		WechatOrdinarySpAppID:                         "wx-mini-app",
		WechatOrdinarySpName:                          "普通服务商",
		WechatOrdinarySpSerialNumber:                  "ordinary-serial-001",
		WechatOrdinarySpPrivateKeyPath:                "./certs/ordinary_key.pem",
		WechatOrdinarySpAPIV3Key:                      testWeChatKey("ordinary-service-provider"),
		WechatOrdinarySpPlatformPublicKeyPath:         "./certs/ordinary-platform.pem",
		WechatOrdinarySpPlatformPublicKeyID:           "PUB_KEY_ID_ORDINARY_001",
		WechatOrdinaryPaymentNotifyURL:                "https://example.com/ordinary/payment",
		WechatOrdinaryCombineNotifyURL:                "https://example.com/ordinary/combine",
		WechatOrdinaryRefundNotifyURL:                 "https://example.com/ordinary/refund",
		WechatOrdinaryProfitSharingNotifyURL:          "https://example.com/ordinary/profit-sharing",
		WechatOrdinaryViolationNotifyURL:              "https://example.com/ordinary/violation",
		WechatOrdinaryApplymentSettlementIDIndividual: "719",
		WechatOrdinaryApplymentQualification:          "餐饮",
		WechatOrdinaryApplymentContactEmail:           "ops@example.com",
		WechatOrdinaryApplymentServicePhone:           "13800138000",
	}

	err := config.ValidateWechatOrdinaryServiceProviderConfig()

	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_INDIVIDUAL")
	require.Contains(t, err.Error(), "WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE")
}

func TestValidateWechatOrdinaryServiceProviderConfigRequiresCompleteActivitiesConfig(t *testing.T) {
	config := Config{
		WechatMiniAppID:                               "wx-mini-app",
		WechatOrdinarySpMchID:                         "1900000109",
		WechatOrdinarySpAppID:                         "wx-mini-app",
		WechatOrdinarySpName:                          "普通服务商",
		WechatOrdinarySpSerialNumber:                  "ordinary-serial-001",
		WechatOrdinarySpPrivateKeyPath:                "./certs/ordinary_key.pem",
		WechatOrdinarySpAPIV3Key:                      testWeChatKey("ordinary-service-provider"),
		WechatOrdinarySpPlatformPublicKeyPath:         "./certs/ordinary-platform.pem",
		WechatOrdinarySpPlatformPublicKeyID:           "PUB_KEY_ID_ORDINARY_001",
		WechatOrdinaryPaymentNotifyURL:                "https://example.com/ordinary/payment",
		WechatOrdinaryCombineNotifyURL:                "https://example.com/ordinary/combine",
		WechatOrdinaryRefundNotifyURL:                 "https://example.com/ordinary/refund",
		WechatOrdinaryProfitSharingNotifyURL:          "https://example.com/ordinary/profit-sharing",
		WechatOrdinaryViolationNotifyURL:              "https://example.com/ordinary/violation",
		WechatOrdinaryApplymentSettlementIDIndividual: "719",
		WechatOrdinaryApplymentSettlementIDEnterprise: "716",
		WechatOrdinaryApplymentQualification:          "餐饮",
		WechatOrdinaryApplymentContactEmail:           "ops@example.com",
		WechatOrdinaryApplymentActivitiesID:           "20191030111cff5b5e",
		WechatOrdinaryApplymentDebitActivitiesRate:    "0.38",
	}

	err := config.ValidateWechatOrdinaryServiceProviderConfig()

	require.Error(t, err)
	require.Contains(t, err.Error(), "WECHAT_ORDINARY_APPLYMENT_CREDIT_ACTIVITIES_RATE")
}

func TestLoadConfig_ReadsAliyunOCRConfig(t *testing.T) {
	configDir := writeTestConfigFile(t, "ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nALIYUN_OCR_ENABLED=true\nALIYUN_OCR_ENDPOINT=https://ocr-api.cn-hangzhou.aliyuncs.com\nALIYUN_OCR_REGION=cn-hangzhou\nALIYUN_OCR_ACCESS_KEY_ID=test-ak\nALIYUN_OCR_ACCESS_KEY_SECRET=test-sk\nALIYUN_OCR_HTTP_TIMEOUT=45s\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)
	require.True(t, config.AliyunOCREnabled)
	require.Equal(t, "https://ocr-api.cn-hangzhou.aliyuncs.com", config.AliyunOCREndpoint)
	require.Equal(t, "cn-hangzhou", config.AliyunOCRRegion)
	require.Equal(t, "test-ak", config.AliyunOCRAccessKeyID)
	require.Equal(t, "test-sk", config.AliyunOCRAccessKeySecret)
	require.Equal(t, 45*time.Second, config.AliyunOCRHTTPTimeout)
	require.NoError(t, config.ValidateAliyunOCRConfig())
}

func TestValidateAliyunOCRConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "disabled skips validation", config: Config{}, want: ""},
		{name: "missing endpoint", config: Config{AliyunOCREnabled: true, AliyunOCRRegion: "cn-hangzhou", AliyunOCRAccessKeyID: "ak", AliyunOCRAccessKeySecret: "sk", AliyunOCRHTTPTimeout: time.Second}, want: "ALIYUN_OCR_ENDPOINT"},
		{name: "missing region", config: Config{AliyunOCREnabled: true, AliyunOCREndpoint: "https://ocr-api.cn-hangzhou.aliyuncs.com", AliyunOCRAccessKeyID: "ak", AliyunOCRAccessKeySecret: "sk", AliyunOCRHTTPTimeout: time.Second}, want: "ALIYUN_OCR_REGION"},
		{name: "missing access key", config: Config{AliyunOCREnabled: true, AliyunOCREndpoint: "https://ocr-api.cn-hangzhou.aliyuncs.com", AliyunOCRRegion: "cn-hangzhou", AliyunOCRHTTPTimeout: time.Second}, want: "ALIYUN_OCR_ACCESS_KEY_ID"},
		{name: "sts missing role arn", config: Config{AliyunOCREnabled: true, AliyunOCREndpoint: "https://ocr-api.cn-hangzhou.aliyuncs.com", AliyunOCRRegion: "cn-hangzhou", AliyunOCRSTSEnabled: true, AliyunOCRHTTPTimeout: time.Second}, want: "ALIYUN_OCR_ROLE_ARN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.ValidateAliyunOCRConfig()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}
