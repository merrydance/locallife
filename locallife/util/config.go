package util

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variable.
type Config struct {
	Environment          string        `mapstructure:"ENVIRONMENT"`
	AllowedOrigins       []string      `mapstructure:"ALLOWED_ORIGINS"`
	DBSource             string        `mapstructure:"DB_SOURCE"`
	MigrationURL         string        `mapstructure:"MIGRATION_URL"`
	RedisAddress         string        `mapstructure:"REDIS_ADDRESS"`
	RedisPassword        string        `mapstructure:"REDIS_PASSWORD"`
	HTTPServerAddress    string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress    string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
	WechatMiniAppID      string        `mapstructure:"WECHAT_MINI_APP_ID"`
	WechatMiniAppSecret  string        `mapstructure:"WECHAT_MINI_APP_SECRET"`

	// 和风天气 API 配置
	QweatherAPIKey  string `mapstructure:"QWEATHER_API_KEY"`
	QweatherAPIHost string `mapstructure:"QWEATHER_API_HOST"`

	// 微信支付配置
	WechatPayMchID                   string        `mapstructure:"WECHAT_PAY_MCH_ID"`                    // 商户号
	WechatPaySerialNumber            string        `mapstructure:"WECHAT_PAY_SERIAL_NUMBER"`             // 商户API证书序列号
	WechatPayPrivateKeyPath          string        `mapstructure:"WECHAT_PAY_PRIVATE_KEY_PATH"`          // 商户API私钥文件路径
	WechatPayAPIV3Key                string        `mapstructure:"WECHAT_PAY_API_V3_KEY"`                // APIv3密钥
	WechatPayNotifyURL               string        `mapstructure:"WECHAT_PAY_NOTIFY_URL"`                // 支付回调URL
	WechatPayRefundNotifyURL         string        `mapstructure:"WECHAT_PAY_REFUND_NOTIFY_URL"`         // 退款回调URL
	WechatPayPlatformCertificatePath string        `mapstructure:"WECHAT_PAY_PLATFORM_CERTIFICATE_PATH"` // 微信支付平台证书路径（已弃用，建议使用公钥）
	WechatPayPlatformPublicKeyPath   string        `mapstructure:"WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH"`  // 微信支付平台公钥路径（推荐）
	WechatPayPlatformPublicKeyID     string        `mapstructure:"WECHAT_PAY_PLATFORM_PUBLIC_KEY_ID"`    // 微信支付平台公钥ID
	WechatPayHTTPTimeout             time.Duration `mapstructure:"WECHAT_PAY_HTTP_TIMEOUT"`              // HTTP请求超时时间

	// 数据加密配置
	DataEncryptionKey string `mapstructure:"DATA_ENCRYPTION_KEY"` // 本地数据加密密钥（16/24/32字节）

	// 腾讯地图配置
	TencentMapKey string `mapstructure:"TENCENT_MAP_KEY"` // 腾讯位置服务 WebService API Key

	// Web前端配置
	WebBaseURL string `mapstructure:"WEB_BASE_URL"` // H5页面基础URL，用于分享功能

	// 上传文件安全访问（签名URL）
	UploadURLSigningKey string        `mapstructure:"UPLOAD_URL_SIGNING_KEY"` // HMAC签名密钥（建议随机长字符串）
	UploadURLTTL        time.Duration `mapstructure:"UPLOAD_URL_TTL"`         // 签名URL有效期（例如 10m, 1h）
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}

	// Normalize common quoted values from .env (e.g. REDIS_PASSWORD="...")
	config.RedisPassword = trimOptionalQuotes(config.RedisPassword)
	return
}

func trimOptionalQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return s
}
