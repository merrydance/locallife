package util

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	configDir := writeTestConfigFile(t, "ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nWECHAT_MINI_APP_ID=wx-mini-app\nWECHAT_MINI_APP_SECRET=mini-secret\nWECHAT_PAY_MCH_ID=1900000109\nWECHAT_PAY_SERIAL_NUMBER=serial-001\nWECHAT_PAY_PRIVATE_KEY_PATH=./certs/apiclient_key.pem\nWECHAT_PAY_API_V3_KEY=0123456789abcdef0123456789abcdef\nWECHAT_PAY_NOTIFY_URL=https://example.com/pay/notify\nWECHAT_PAY_REFUND_NOTIFY_URL=https://example.com/pay/refund-notify\nWECHAT_SHIPPING_SETTLE_NOTIFY_URL=https://example.com/pay/settlement-notify\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH=./certs/platform.pem\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_001\nWECHAT_PAY_HTTP_TIMEOUT=45s\nWECHAT_ECOMMERCE_SP_MCHID=service-mchid-001\nWECHAT_ECOMMERCE_SP_APPID=service-appid-001\nWECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL=https://example.com/ecommerce/payment-notify\nWECHAT_ECOMMERCE_COMBINE_NOTIFY_URL=https://example.com/ecommerce/combine-notify\nWECHAT_ECOMMERCE_REFUND_NOTIFY_URL=https://example.com/ecommerce/refund-notify\nWECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL=https://example.com/ecommerce/withdraw-notify\nWECHAT_ECOMMERCE_SP_NAME=测试平台服务商\nWECHAT_ECOMMERCE_SP_SERIAL_NUMBER=sp-serial-001\nWECHAT_ECOMMERCE_SP_PRIVATE_KEY_PATH=./certs/sp_apiclient_key.pem\nWECHAT_ECOMMERCE_SP_API_V3_KEY=abcdef0123456789abcdef0123456789\nWECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH=./certs/sp-platform.pem\nWECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_SP_001\nREDIS_REQUIRED=true\n")

	config, err := LoadConfig(configDir)
	require.NoError(t, err)

	require.Equal(t, "wx-mini-app", config.WechatMiniAppID)
	require.Equal(t, "mini-secret", config.WechatMiniAppSecret)
	require.Equal(t, "1900000109", config.WechatPayMchID)
	require.Equal(t, "serial-001", config.WechatPaySerialNumber)
	require.Equal(t, "./certs/apiclient_key.pem", config.WechatPayPrivateKeyPath)
	require.Equal(t, "0123456789abcdef0123456789abcdef", config.WechatPayAPIV3Key)
	require.Equal(t, "https://example.com/pay/notify", config.WechatPayNotifyURL)
	require.Equal(t, "https://example.com/pay/refund-notify", config.WechatPayRefundNotifyURL)
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
	require.Equal(t, "测试平台服务商", config.WechatEcommerceSpName)
	require.Equal(t, "sp-serial-001", config.WechatEcommerceSpSerialNumber)
	require.Equal(t, "./certs/sp_apiclient_key.pem", config.WechatEcommerceSpPrivateKeyPath)
	require.Equal(t, "abcdef0123456789abcdef0123456789", config.WechatEcommerceSpAPIV3Key)
	require.Equal(t, "./certs/sp-platform.pem", config.WechatEcommerceSpPlatformPublicKeyPath)
	require.Equal(t, "PUB_KEY_ID_SP_001", config.WechatEcommerceSpPlatformPublicKeyID)
	require.True(t, config.RedisRequired)
}

func TestEffectiveWechatEcommerceNotifyURLs(t *testing.T) {
	config := Config{
		WechatPayNotifyURL:       "https://example.com/v1/webhooks/wechat-pay/notify",
		WechatPayRefundNotifyURL: "https://example.com/v1/webhooks/wechat-pay/refund-notify",
	}

	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/payment-notify", config.EffectiveWechatEcommercePaymentNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/combine-notify", config.EffectiveWechatEcommerceCombineNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/refund-notify", config.EffectiveWechatEcommerceRefundNotifyURL())
	require.Equal(t, "https://example.com/v1/webhooks/wechat-ecommerce/withdraw-notify", config.EffectiveWechatEcommerceWithdrawNotifyURL())

	config.WechatEcommercePaymentNotifyURL = "https://override.example.com/ecommerce/payment-notify"
	config.WechatEcommerceCombineNotifyURL = "https://override.example.com/ecommerce/combine-notify"
	config.WechatEcommerceRefundNotifyURL = "https://override.example.com/ecommerce/refund-notify"
	config.WechatEcommerceWithdrawNotifyURL = "https://override.example.com/ecommerce/withdraw-notify"

	require.Equal(t, "https://override.example.com/ecommerce/payment-notify", config.EffectiveWechatEcommercePaymentNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/combine-notify", config.EffectiveWechatEcommerceCombineNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/refund-notify", config.EffectiveWechatEcommerceRefundNotifyURL())
	require.Equal(t, "https://override.example.com/ecommerce/withdraw-notify", config.EffectiveWechatEcommerceWithdrawNotifyURL())
}

func TestValidateWechatEcommerceConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "same merchant allows fallback",
			config: Config{
				WechatPayMchID:          "1900000109",
				WechatPayPrivateKeyPath: "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:  "1900000109",
				WechatEcommerceSpAppID:  "wx-app",
			},
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
			name: "platform public key pair must be complete",
			config: Config{
				WechatPayMchID:                         "1900000109",
				WechatPayPrivateKeyPath:                "./certs/apiclient_key.pem",
				WechatEcommerceSpMchID:                 "service-mchid-001",
				WechatEcommerceSpAppID:                 "service-appid-001",
				WechatEcommerceSpSerialNumber:          "sp-serial-001",
				WechatEcommerceSpPrivateKeyPath:        "./certs/sp_apiclient_key.pem",
				WechatEcommerceSpAPIV3Key:              "abcdef0123456789abcdef0123456789",
				WechatEcommerceSpPlatformPublicKeyPath: "./certs/sp-platform.pem",
			},
			want: "WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH",
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
