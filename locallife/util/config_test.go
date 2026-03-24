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
	configDir := writeTestConfigFile(t, "ENVIRONMENT=test\nDB_SOURCE=postgresql:///test\nMIGRATION_URL=file://db/migration\nWECHAT_MINI_APP_ID=wx-mini-app\nWECHAT_MINI_APP_SECRET=mini-secret\nWECHAT_PAY_MCH_ID=1900000109\nWECHAT_PAY_SERIAL_NUMBER=serial-001\nWECHAT_PAY_PRIVATE_KEY_PATH=./certs/apiclient_key.pem\nWECHAT_PAY_API_V3_KEY=0123456789abcdef0123456789abcdef\nWECHAT_PAY_NOTIFY_URL=https://example.com/pay/notify\nWECHAT_PAY_REFUND_NOTIFY_URL=https://example.com/pay/refund-notify\nWECHAT_SHIPPING_SETTLE_NOTIFY_URL=https://example.com/pay/settlement-notify\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH=./certs/platform.pem\nWECHAT_PAY_PLATFORM_PUBLIC_KEY_ID=PUB_KEY_ID_001\nWECHAT_PAY_HTTP_TIMEOUT=45s\nWECHAT_ECOMMERCE_SP_MCHID=service-mchid-001\nWECHAT_ECOMMERCE_SP_APPID=service-appid-001\nREDIS_REQUIRED=true\n")

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
	require.True(t, config.RedisRequired)
}
