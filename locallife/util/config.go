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
	LogLevel             string        `mapstructure:"LOG_LEVEL"`
	AllowedOrigins       []string      `mapstructure:"ALLOWED_ORIGINS"`
	LBSProvider          string        `mapstructure:"LBS_PROVIDER"`        // 运行时统一使用 "tencent"（兼容旧配置保留）
	OSMBaseURL           string        `mapstructure:"OSM_BASE_URL"`        // 已废弃（历史 OSM 配置，保留兼容）
	OSMBaseURLBackup     string        `mapstructure:"OSM_BASE_URL_BACKUP"` // 已废弃（历史 OSM 备用配置，保留兼容）
	DBSource             string        `mapstructure:"DB_SOURCE"`
	MigrationURL         string        `mapstructure:"MIGRATION_URL"`
	AutoMigrate          bool          `mapstructure:"AUTO_MIGRATE"`
	RedisAddress         string        `mapstructure:"REDIS_ADDRESS"`
	RedisPassword        string        `mapstructure:"REDIS_PASSWORD"`
	RedisRequired        bool          `mapstructure:"REDIS_REQUIRED"`
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

	// 腾讯地图配置（运行时必填）
	TencentMapKey string `mapstructure:"TENCENT_MAP_KEY"`

	// 天地图配置（仅历史/离线工具使用）
	TiandituMapKey  string `mapstructure:"TIANDITU_MAP_KEY"`
	TiandituBaseURL string `mapstructure:"TIANDITU_BASE_URL"`

	// Web前端配置
	WebBaseURL string `mapstructure:"WEB_BASE_URL"` // H5页面基础URL，用于分享功能

	// Web 登录扫码会话
	WebLoginSessionTTL   time.Duration `mapstructure:"WEB_LOGIN_SESSION_TTL"`
	WebLoginQRSigningKey string        `mapstructure:"WEB_LOGIN_QR_SIGNING_KEY"`

	// 上传文件安全访问（签名URL）
	UploadURLSigningKey string        `mapstructure:"UPLOAD_URL_SIGNING_KEY"` // HMAC签名密钥（建议随机长字符串）
	UploadURLTTL        time.Duration `mapstructure:"UPLOAD_URL_TTL"`         // 签名URL有效期（例如 10m, 1h）

	// WebSocket reliable push rollout
	WebSocketReliableEnabled bool `mapstructure:"WS_RELIABLE_ENABLED"`
	WebSocketReliablePercent int  `mapstructure:"WS_RELIABLE_PERCENT"`

	// Rules engine toggle
	RulesEngineEnabled bool `mapstructure:"RULES_ENGINE_ENABLED"`

	// Geofence configs for delivery events
	GeofenceRadiusMeters       int  `mapstructure:"GEOFENCE_RADIUS_M"`
	GeofenceDwellMinSeconds    int  `mapstructure:"GEOFENCE_DWELL_MIN_SECONDS"`
	GeofenceDwellMinSamples    int  `mapstructure:"GEOFENCE_DWELL_MIN_SAMPLES"`
	GeofenceMinAccuracyMeters  int  `mapstructure:"GEOFENCE_MIN_ACCURACY_M"`
	GeofenceAutoAdvanceEnabled bool `mapstructure:"GEOFENCE_AUTO_ADVANCE_ENABLED"`
	GeofenceAutoPickupEnabled  bool `mapstructure:"GEOFENCE_AUTO_PICKUP_ENABLED"`
	GeofenceAutoDeliverEnabled bool `mapstructure:"GEOFENCE_AUTO_DELIVER_ENABLED"`

	// Delivery and LBS configs
	RiderAverageSpeed  int `mapstructure:"RIDER_AVERAGE_SPEED"`  // m/h
	DefaultPrepareTime int `mapstructure:"DEFAULT_PREPARE_TIME"` // minutes

	// Profit sharing return retry configs
	ProfitSharingReturnRetryInterval time.Duration `mapstructure:"PROFIT_SHARING_RETURN_RETRY_INTERVAL"`
	ProfitSharingReturnMaxRetries    int           `mapstructure:"PROFIT_SHARING_RETURN_MAX_RETRIES"`

	// Reservation cancel refund policy (%), role-based and deadline-based
	ReservationUserRefundPercentBeforeDeadline     int `mapstructure:"RESERVATION_USER_REFUND_PERCENT_BEFORE_DEADLINE"`
	ReservationUserRefundPercentAfterDeadline      int `mapstructure:"RESERVATION_USER_REFUND_PERCENT_AFTER_DEADLINE"`
	ReservationMerchantRefundPercentBeforeDeadline int `mapstructure:"RESERVATION_MERCHANT_REFUND_PERCENT_BEFORE_DEADLINE"`
	ReservationMerchantRefundPercentAfterDeadline  int `mapstructure:"RESERVATION_MERCHANT_REFUND_PERCENT_AFTER_DEADLINE"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	viper.SetDefault("AUTO_MIGRATE", false)
	viper.SetDefault("REDIS_REQUIRED", false)
	viper.SetDefault("LOG_LEVEL", "info")
	// WebSocket rollout defaults
	viper.SetDefault("WS_RELIABLE_ENABLED", true)
	viper.SetDefault("WS_RELIABLE_PERCENT", 100)
	viper.SetDefault("RULES_ENGINE_ENABLED", false)
	// Geofence defaults
	viper.SetDefault("GEOFENCE_RADIUS_M", 80)
	viper.SetDefault("GEOFENCE_DWELL_MIN_SECONDS", 60)
	viper.SetDefault("GEOFENCE_DWELL_MIN_SAMPLES", 3)
	viper.SetDefault("GEOFENCE_MIN_ACCURACY_M", 80)
	viper.SetDefault("GEOFENCE_AUTO_ADVANCE_ENABLED", false)
	viper.SetDefault("GEOFENCE_AUTO_PICKUP_ENABLED", false)
	viper.SetDefault("GEOFENCE_AUTO_DELIVER_ENABLED", false)
	// Delivery defaults
	viper.SetDefault("RIDER_AVERAGE_SPEED", 15000)
	viper.SetDefault("DEFAULT_PREPARE_TIME", 20)
	// Profit sharing return retry defaults
	viper.SetDefault("PROFIT_SHARING_RETURN_RETRY_INTERVAL", "1m")
	viper.SetDefault("PROFIT_SHARING_RETURN_MAX_RETRIES", 10)
	viper.SetDefault("RESERVATION_USER_REFUND_PERCENT_BEFORE_DEADLINE", 100)
	viper.SetDefault("RESERVATION_USER_REFUND_PERCENT_AFTER_DEADLINE", 0)
	viper.SetDefault("RESERVATION_MERCHANT_REFUND_PERCENT_BEFORE_DEADLINE", 100)
	viper.SetDefault("RESERVATION_MERCHANT_REFUND_PERCENT_AFTER_DEADLINE", 100)
	// Web 登录默认过期时间
	viper.SetDefault("WEB_LOGIN_SESSION_TTL", "5m")

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
