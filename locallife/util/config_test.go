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
	require.EqualValues(t, 200, config.BaofuAccountVerifyFeeFen)
	require.Equal(t, "9931", config.BaofuBusinessIndustryID)
	require.Equal(t, "758-2", config.BaofuMerchantReportBusiness)
}

func TestLoadConfig_ReadsWechatPaymentConfig(t *testing.T) {
	payKey := testWechatPayAPIV3Key()
	configDir := writeTestConfigFile(t, fmt.Sprintf("ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nWECHAT_MINI_APP_ID=wx-mini-app\nWECHAT_MINI_APP_SECRET=mini-secret\nWECHAT_PAY_MCH_ID=1900000109\nWECHAT_PAY_SERIAL_NUMBER=serial-001\nWECHAT_PAY_PRIVATE_KEY_PATH=./certs/apiclient_key.pem\nWECHAT_PAY_API_V3_KEY=%s\nWECHAT_PAY_NOTIFY_URL=https://example.com/pay/notify\nWECHAT_PAY_REFUND_NOTIFY_URL=https://example.com/pay/refund-notify\nWECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL=https://example.com/pay/merchant-transfer-notify\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH=./certs/platform.pem\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_001\nWECHAT_PAY_HTTP_TIMEOUT=45s\nREDIS_REQUIRED=true\n", payKey))

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
	require.Equal(t, "./certs/platform.pem", config.WechatPayPlatformPublicKeyPath)
	require.Equal(t, "PUB_KEY_ID_001", config.WechatPayPlatformPublicKeyID)
	require.Equal(t, 45*time.Second, config.WechatPayHTTPTimeout)
	require.True(t, config.RedisRequired)
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
		"BAOFU_NOTIFY_BASE_URL=https://api.example.com/v1/webhooks/baofu",
		"BAOFU_PAYMENT_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/payment",
		"BAOFU_PROFIT_SHARING_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/share",
		"BAOFU_REFUND_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/refund",
		"BAOFU_WITHDRAW_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/withdraw",
		"BAOFU_HTTP_TIMEOUT=12s",
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)
	require.True(t, config.BaofuMainBusinessEnabled)
	require.True(t, config.HasBaofuRuntimeConfig())
	require.NoError(t, config.ValidateBaofuConfig())
	require.Equal(t, "wx-local-life", config.WechatMiniAppID)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/payment", config.EffectiveBaofuPaymentNotifyURL())
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/share", config.EffectiveBaofuProfitSharingNotifyURL())
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", config.EffectiveBaofuWithdrawNotifyURL())
	require.Equal(t, 12*time.Second, config.BaofuHTTPTimeout)

	baofuConfig := config.ToBaofuConfig().Normalized()
	require.Equal(t, baofu.SandboxAggregatePayBaseURL, baofuConfig.AggregatePayBaseURL)
	require.Equal(t, "COLLECT_MER", baofuConfig.CollectMerchantID)
	require.Equal(t, "PAYOUT_MER", baofuConfig.PayoutMerchantID)
}

func TestEffectiveBaofuWithdrawNotifyURLFallsBackToNotifyBase(t *testing.T) {
	config := Config{BaofuNotifyBaseURL: "https://api.example.com/v1/webhooks/baofu"}

	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", config.EffectiveBaofuWithdrawNotifyURL())
}

func TestEffectiveBaofuWithdrawNotifyURLPrefersExplicitURL(t *testing.T) {
	config := Config{
		BaofuNotifyBaseURL:     "https://api.example.com/v1/webhooks/baofu",
		BaofuWithdrawNotifyURL: "https://notify.example.com/custom/withdraw",
	}

	require.Equal(t, "https://notify.example.com/custom/withdraw", config.EffectiveBaofuWithdrawNotifyURL())
}

func TestLoadConfig_NormalizesEscapedBaofuPEMValues(t *testing.T) {
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
		`BAOFU_PRIVATE_KEY_PEM=-----BEGIN PRIVATE KEY-----\\nprivate-body\\n-----END PRIVATE KEY-----`,
		`BAOFU_PUBLIC_KEY_PEM=-----BEGIN CERTIFICATE-----\\npublic-body\\n-----END CERTIFICATE-----`,
		"BAOFU_SIGN_SERIAL_NO=sign-sn",
		"BAOFU_ENCRYPTION_SERIAL_NO=enc-sn",
		"BAOFU_NOTIFY_BASE_URL=https://api.example.com/v1/webhooks/baofu",
		"BAOFU_PAYMENT_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/payment",
		"BAOFU_PROFIT_SHARING_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/share",
		"BAOFU_REFUND_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/refund",
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	baofuConfig := config.ToBaofuConfig()
	require.Contains(t, baofuConfig.PrivateKeyPEM, "\nprivate-body\n")
	require.NotContains(t, baofuConfig.PrivateKeyPEM, `\n`)
	require.Contains(t, baofuConfig.BaofuPublicKeyPEM, "\npublic-body\n")
	require.NotContains(t, baofuConfig.BaofuPublicKeyPEM, `\n`)
}

func TestLoadConfig_ReadsBaofuPEMValuesFromFilePaths(t *testing.T) {
	configDir := t.TempDir()
	certsDir := filepath.Join(configDir, "certs")
	require.NoError(t, os.MkdirAll(certsDir, 0o700))
	privatePEM := "-----BEGIN PRIVATE KEY-----\nprivate-body\n-----END PRIVATE KEY-----\n"
	publicPEM := "-----BEGIN CERTIFICATE-----\npublic-body\n-----END CERTIFICATE-----\n"
	require.NoError(t, os.WriteFile(filepath.Join(certsDir, "baofu_private.pem"), []byte(privatePEM), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(certsDir, "baofu_public.pem"), []byte(publicPEM), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "app.env"), []byte(strings.Join([]string{
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
		"BAOFU_PRIVATE_KEY_PEM=bad-private-env",
		"BAOFU_PUBLIC_KEY_PEM=bad-public-env",
		"BAOFU_PRIVATE_KEY_PATH=./certs/baofu_private.pem",
		"BAOFU_PUBLIC_KEY_PATH=./certs/baofu_public.pem",
		"BAOFU_SIGN_SERIAL_NO=sign-sn",
		"BAOFU_ENCRYPTION_SERIAL_NO=enc-sn",
		"BAOFU_NOTIFY_BASE_URL=https://api.example.com/v1/webhooks/baofu",
		"BAOFU_PAYMENT_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/payment",
		"BAOFU_PROFIT_SHARING_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/share",
		"BAOFU_REFUND_NOTIFY_URL=https://api.example.com/v1/webhooks/baofu/refund",
	}, "\n")+"\n"), 0o600))

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "./certs/baofu_private.pem", config.BaofuPrivateKeyPath)
	require.Equal(t, "./certs/baofu_public.pem", config.BaofuPublicKeyPath)
	baofuConfig := config.ToBaofuConfig()
	require.Equal(t, strings.TrimSpace(privatePEM), baofuConfig.PrivateKeyPEM)
	require.Equal(t, strings.TrimSpace(publicPEM), baofuConfig.BaofuPublicKeyPEM)
}

func TestLoadConfig_ReadsFeieyunCallbackConfig(t *testing.T) {
	configDir := writeTestConfigFile(t, strings.Join([]string{
		"ENVIRONMENT=test",
		"DB_SOURCE=postgresql:///test",
		"MIGRATION_URL=file://db/migration",
		"FEIEYUN_ENABLED=true",
		"FEIEYUN_API_BASE_URL=https://api.feieyun.cn",
		"FEIEYUN_USER=feie-user",
		"FEIEYUN_UKEY=feie-ukey",
		"FEIEYUN_HTTP_TIMEOUT=9s",
		"FEIEYUN_PRINT_CALLBACK_URL=https://api.example.com/v1/webhooks/feieyun/print-result",
		`FEIEYUN_CALLBACK_PUBLIC_KEY_PEM=-----BEGIN PUBLIC KEY-----\\npublic-body\\n-----END PUBLIC KEY-----`,
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.True(t, config.FeieyunEnabled)
	require.Equal(t, "https://api.feieyun.cn", config.FeieyunAPIBaseURL)
	require.Equal(t, "feie-user", config.FeieyunUser)
	require.Equal(t, "feie-ukey", config.FeieyunUkey)
	require.Equal(t, 9*time.Second, config.FeieyunHTTPTimeout)
	require.Equal(t, "https://api.example.com/v1/webhooks/feieyun/print-result", config.FeieyunPrintCallbackURL)
	require.Contains(t, config.FeieyunCallbackPublicKeyPEM, "\npublic-body\n")
	require.NotContains(t, config.FeieyunCallbackPublicKeyPEM, `\n`)
}

func TestLoadConfig_ReadsFeieyunCallbackPublicKeyFromPath(t *testing.T) {
	configDir := t.TempDir()
	certsDir := filepath.Join(configDir, "certs")
	require.NoError(t, os.MkdirAll(certsDir, 0o700))
	publicPEM := "-----BEGIN PUBLIC KEY-----\npublic-body\n-----END PUBLIC KEY-----\n"
	require.NoError(t, os.WriteFile(filepath.Join(certsDir, "feieyun_public.key"), []byte(publicPEM), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "app.env"), []byte(strings.Join([]string{
		"ENVIRONMENT=test",
		"DB_SOURCE=postgresql:///test",
		"MIGRATION_URL=file://db/migration",
		"FEIEYUN_CALLBACK_PUBLIC_KEY_PEM=bad-public-env",
		"FEIEYUN_CALLBACK_PUBLIC_KEY_PATH=./certs/feieyun_public.key",
	}, "\n")+"\n"), 0o600))

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "./certs/feieyun_public.key", config.FeieyunCallbackPublicKeyPath)
	require.Equal(t, strings.TrimSpace(publicPEM), config.FeieyunCallbackPublicKeyPEM)
}

func TestLoadConfig_ReadsCloudPrinterProviderConfig(t *testing.T) {
	configDir := writeTestConfigFile(t, strings.Join([]string{
		"ENVIRONMENT=test",
		"DB_SOURCE=postgresql:///test",
		"MIGRATION_URL=file://db/migration",
		"YILIANYUN_ENABLED=true",
		"YILIANYUN_API_BASE_URL=https://open-api.10ss.net/v2",
		"YILIANYUN_CUSTOMER_ID=yly-customer",
		"YILIANYUN_APP_ID=yly-app",
		"YILIANYUN_APP_SECRET=yly-secret",
		"YILIANYUN_HTTP_TIMEOUT=7s",
		"YILIANYUN_AUTH_CALLBACK_URL=https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
		"YILIANYUN_PRINT_CALLBACK_URL=https://api.example.com/v1/webhooks/yilianyun/print-result",
		"YILIANYUN_PRINT_CALLBACK_FRESHNESS_WINDOW=9m",
		"SHANGPENG_ENABLED=true",
		"SHANGPENG_API_BASE_URL=https://open.spyun.net",
		"SHANGPENG_APPID=spyun-app",
		"SHANGPENG_APPSECRET=spyun-secret",
		"SHANGPENG_HTTP_TIMEOUT=8s",
		"CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR=true",
		"CLOUD_PRINTER_STATUS_POLL_INTERVAL=2m",
		"CLOUD_PRINTER_STATUS_POLL_BATCH_SIZE=25",
		"CLOUD_PRINTER_STATUS_POLL_INITIAL_DELAY=45s",
		"CLOUD_PRINTER_STATUS_POLL_MAX_AGE=10h",
	}, "\n")+"\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.True(t, config.YilianyunEnabled)
	require.Equal(t, "https://open-api.10ss.net/v2", config.YilianyunAPIBaseURL)
	require.Equal(t, "yly-customer", config.YilianyunCustomerID)
	require.Equal(t, "yly-app", config.YilianyunAppID)
	require.Equal(t, "yly-secret", config.YilianyunAppSecret)
	require.Equal(t, 7*time.Second, config.YilianyunHTTPTimeout)
	require.Equal(t, "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback", config.YilianyunAuthCallbackURL)
	require.Equal(t, "https://api.example.com/v1/webhooks/yilianyun/print-result", config.YilianyunPrintCallbackURL)
	require.Equal(t, 9*time.Minute, config.YilianyunPrintCallbackFreshnessWindow)
	require.True(t, config.ShangpengEnabled)
	require.Equal(t, "https://open.spyun.net", config.ShangpengAPIBaseURL)
	require.Equal(t, "spyun-app", config.ShangpengAppID)
	require.Equal(t, "spyun-secret", config.ShangpengAppSecret)
	require.Equal(t, 8*time.Second, config.ShangpengHTTPTimeout)
	require.True(t, config.CloudPrinterFailOnProviderConfigError)
	require.Equal(t, 2*time.Minute, config.CloudPrinterStatusPollInterval)
	require.Equal(t, 25, config.CloudPrinterStatusPollBatchSize)
	require.Equal(t, 45*time.Second, config.CloudPrinterStatusPollInitialDelay)
	require.Equal(t, 10*time.Hour, config.CloudPrinterStatusPollMaxAge)
}

func TestValidateCloudPrinterProviderConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "disabled skips validation", config: Config{}},
		{
			name: "yilianyun open app does not require global access token",
			config: Config{
				YilianyunEnabled:                   true,
				YilianyunAPIBaseURL:                "https://open-api.10ss.net/v2",
				YilianyunAppID:                     "app",
				YilianyunAppSecret:                 "secret",
				YilianyunHTTPTimeout:               time.Second,
				YilianyunAuthCallbackURL:           "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
				CloudPrinterStatusPollInterval:     time.Minute,
				CloudPrinterStatusPollBatchSize:    50,
				CloudPrinterStatusPollInitialDelay: time.Second,
				CloudPrinterStatusPollMaxAge:       time.Hour,
			},
		},
		{
			name: "yilianyun scan-code open app does not require auth callback url",
			config: Config{
				YilianyunEnabled:                   true,
				YilianyunAPIBaseURL:                "https://open-api.10ss.net/v2",
				YilianyunAppID:                     "app",
				YilianyunAppSecret:                 "secret",
				YilianyunHTTPTimeout:               time.Second,
				CloudPrinterStatusPollInterval:     time.Minute,
				CloudPrinterStatusPollBatchSize:    50,
				CloudPrinterStatusPollInitialDelay: time.Second,
				CloudPrinterStatusPollMaxAge:       time.Hour,
			},
		},
		{
			name: "yilianyun auth callback url must be absolute",
			config: Config{
				YilianyunEnabled:         true,
				YilianyunAPIBaseURL:      "https://open-api.10ss.net/v2",
				YilianyunAppID:           "app",
				YilianyunAppSecret:       "secret",
				YilianyunHTTPTimeout:     time.Second,
				YilianyunAuthCallbackURL: "/v1/merchant/devices/yilianyun/auth/callback",
			},
			want: "YILIANYUN_AUTH_CALLBACK_URL must be a valid absolute URL",
		},
		{
			name: "yilianyun callback url must be absolute",
			config: Config{
				YilianyunEnabled:          true,
				YilianyunAPIBaseURL:       "https://open-api.10ss.net/v2",
				YilianyunAppID:            "app",
				YilianyunAppSecret:        "secret",
				YilianyunHTTPTimeout:      time.Second,
				YilianyunAuthCallbackURL:  "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
				YilianyunPrintCallbackURL: "/webhooks/yilianyun/print-result",
			},
			want: "YILIANYUN_PRINT_CALLBACK_URL must be a valid absolute URL",
		},
		{
			name: "yilianyun api base url must be absolute",
			config: Config{
				YilianyunEnabled:         true,
				YilianyunAPIBaseURL:      "open-api.10ss.net",
				YilianyunAppID:           "app",
				YilianyunAppSecret:       "secret",
				YilianyunHTTPTimeout:     time.Second,
				YilianyunAuthCallbackURL: "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
			},
			want: "YILIANYUN_API_BASE_URL must be a valid absolute URL",
		},
		{
			name: "shangpeng requires app secret",
			config: Config{
				ShangpengEnabled:     true,
				ShangpengAPIBaseURL:  "https://open.spyun.net",
				ShangpengAppID:       "appid",
				ShangpengHTTPTimeout: time.Second,
			},
			want: "SHANGPENG_APPSECRET",
		},
		{
			name: "shangpeng api base url must be absolute",
			config: Config{
				ShangpengEnabled:     true,
				ShangpengAPIBaseURL:  "open.spyun.net",
				ShangpengAppID:       "appid",
				ShangpengAppSecret:   "secret",
				ShangpengHTTPTimeout: time.Second,
			},
			want: "SHANGPENG_API_BASE_URL must be a valid absolute URL",
		},
		{
			name: "enabled provider requires positive polling config",
			config: Config{
				YilianyunEnabled:         true,
				YilianyunAPIBaseURL:      "https://open-api.10ss.net/v2",
				YilianyunAppID:           "app",
				YilianyunAppSecret:       "secret",
				YilianyunHTTPTimeout:     time.Second,
				YilianyunAuthCallbackURL: "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
			},
			want: "CLOUD_PRINTER_STATUS_POLL_INTERVAL must be > 0",
		},
		{
			name: "valid providers pass",
			config: Config{
				YilianyunEnabled:                   true,
				YilianyunAPIBaseURL:                "https://open-api.10ss.net/v2",
				YilianyunAppID:                     "app",
				YilianyunAppSecret:                 "secret",
				YilianyunHTTPTimeout:               time.Second,
				YilianyunAuthCallbackURL:           "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
				ShangpengEnabled:                   true,
				ShangpengAPIBaseURL:                "https://open.spyun.net",
				ShangpengAppID:                     "appid",
				ShangpengAppSecret:                 "secret",
				ShangpengHTTPTimeout:               time.Second,
				CloudPrinterStatusPollInterval:     time.Minute,
				CloudPrinterStatusPollBatchSize:    50,
				CloudPrinterStatusPollInitialDelay: time.Second,
				CloudPrinterStatusPollMaxAge:       time.Hour,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.ValidateCloudPrinterProviderConfig()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestEffectiveWechatPayMerchantTransferNotifyURL(t *testing.T) {
	config := Config{
		WechatPayMerchantTransferNotifyURL: "https://example.com/v1/webhooks/wechat-pay/merchant-transfer-notify",
	}

	require.Equal(t, "https://example.com/v1/webhooks/wechat-pay/merchant-transfer-notify", config.EffectiveWechatPayMerchantTransferNotifyURL())

	config.WechatPayMerchantTransferNotifyURL = "https://override.example.com/pay/merchant-transfer-notify"

	require.Equal(t, "https://override.example.com/pay/merchant-transfer-notify", config.EffectiveWechatPayMerchantTransferNotifyURL())
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
