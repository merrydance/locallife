package util

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variable.
type Config struct {
	Environment               string        `mapstructure:"ENVIRONMENT"`
	LogLevel                  string        `mapstructure:"LOG_LEVEL"`
	AllowedOrigins            []string      `mapstructure:"ALLOWED_ORIGINS"`
	LBSProvider               string        `mapstructure:"LBS_PROVIDER"`        // 运行时统一使用 "tencent"（兼容旧配置保留）
	OSMBaseURL                string        `mapstructure:"OSM_BASE_URL"`        // 已废弃（历史 OSM 配置，保留兼容）
	OSMBaseURLBackup          string        `mapstructure:"OSM_BASE_URL_BACKUP"` // 已废弃（历史 OSM 备用配置，保留兼容）
	DBSource                  string        `mapstructure:"DB_SOURCE"`
	MigrationURL              string        `mapstructure:"MIGRATION_URL"`
	AutoMigrate               bool          `mapstructure:"AUTO_MIGRATE"`
	RedisAddress              string        `mapstructure:"REDIS_ADDRESS"`
	RedisPassword             string        `mapstructure:"REDIS_PASSWORD"`
	RedisRequired             bool          `mapstructure:"REDIS_REQUIRED"`
	HTTPServerAddress         string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GRPCServerAddress         string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	TokenSymmetricKey         string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration       time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration      time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
	WechatMiniAppID           string        `mapstructure:"WECHAT_MINI_APP_ID"`
	WechatMiniAppSecret       string        `mapstructure:"WECHAT_MINI_APP_SECRET"`
	WechatMiniAppMessageToken string        `mapstructure:"WECHAT_MINI_APP_MESSAGE_TOKEN"`

	// 和风天气 API 配置
	QweatherAPIKey  string `mapstructure:"QWEATHER_API_KEY"`
	QweatherAPIHost string `mapstructure:"QWEATHER_API_HOST"`

	// 微信支付配置
	WechatPayMchID                                string        `mapstructure:"WECHAT_PAY_MCH_ID"`                                  // 商户号
	WechatPaySerialNumber                         string        `mapstructure:"WECHAT_PAY_SERIAL_NUMBER"`                           // 商户API证书序列号
	WechatPayPrivateKeyPath                       string        `mapstructure:"WECHAT_PAY_PRIVATE_KEY_PATH"`                        // 商户API私钥文件路径
	WechatPayAPIV3Key                             string        `mapstructure:"WECHAT_PAY_API_V3_KEY"`                              // APIv3密钥
	WechatPayNotifyURL                            string        `mapstructure:"WECHAT_PAY_NOTIFY_URL"`                              // 支付回调URL
	WechatPayRefundNotifyURL                      string        `mapstructure:"WECHAT_PAY_REFUND_NOTIFY_URL"`                       // 退款回调URL
	WechatPayMerchantTransferNotifyURL            string        `mapstructure:"WECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL"`            // 商家转账回调URL
	WechatShippingSettleNotifyURL                 string        `mapstructure:"WECHAT_SHIPPING_SETTLE_NOTIFY_URL"`                  // 发货结算事件回调URL（trade_manage_order_settlement）
	WechatPayPlatformPublicKeyPath                string        `mapstructure:"WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH"`                // 微信支付平台公钥路径（推荐）
	WechatPayPlatformPublicKeyID                  string        `mapstructure:"WECHAT_PAY_PLATFORM_PUBLIC_KEY_ID"`                  // 微信支付平台公钥ID
	WechatPayHTTPTimeout                          time.Duration `mapstructure:"WECHAT_PAY_HTTP_TIMEOUT"`                            // HTTP请求超时时间
	WechatEcommerceSpMchID                        string        `mapstructure:"WECHAT_ECOMMERCE_SP_MCHID"`                          // 收付通服务商商户号
	WechatEcommerceSpAppID                        string        `mapstructure:"WECHAT_ECOMMERCE_SP_APPID"`                          // 收付通服务商 AppID
	WechatEcommercePaymentNotifyURL               string        `mapstructure:"WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL"`                // 收付通普通支付回调URL
	WechatEcommerceCombineNotifyURL               string        `mapstructure:"WECHAT_ECOMMERCE_COMBINE_NOTIFY_URL"`                // 收付通合单支付回调URL
	WechatEcommerceRefundNotifyURL                string        `mapstructure:"WECHAT_ECOMMERCE_REFUND_NOTIFY_URL"`                 // 收付通退款回调URL
	WechatEcommerceWithdrawNotifyURL              string        `mapstructure:"WECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL"`               // 收付通提现回调URL
	WechatEcommerceViolationNotifyURL             string        `mapstructure:"WECHAT_ECOMMERCE_VIOLATION_NOTIFY_URL"`              // 收付通商户违规通知回调URL
	WechatEcommerceSpName                         string        `mapstructure:"WECHAT_ECOMMERCE_SP_NAME"`                           // 收付通服务商主体全称（可选，用于分账接收方姓名）
	WechatEcommerceSpSerialNumber                 string        `mapstructure:"WECHAT_ECOMMERCE_SP_SERIAL_NUMBER"`                  // 收付通服务商 API 证书序列号
	WechatEcommerceSpPrivateKeyPath               string        `mapstructure:"WECHAT_ECOMMERCE_SP_PRIVATE_KEY_PATH"`               // 收付通服务商 API 私钥文件路径
	WechatEcommerceSpAPIV3Key                     string        `mapstructure:"WECHAT_ECOMMERCE_SP_API_V3_KEY"`                     // 收付通服务商 APIv3 密钥
	WechatEcommerceSpPlatformPublicKeyPath        string        `mapstructure:"WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH"`       // 收付通服务商平台公钥路径
	WechatEcommerceSpPlatformPublicKeyID          string        `mapstructure:"WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_ID"`         // 收付通服务商平台公钥ID
	WechatOrdinarySpMchID                         string        `mapstructure:"WECHAT_ORDINARY_SP_MCHID"`                           // 普通服务商商户号
	WechatOrdinarySpAppID                         string        `mapstructure:"WECHAT_ORDINARY_SP_APPID"`                           // 普通服务商 AppID
	WechatOrdinarySpName                          string        `mapstructure:"WECHAT_ORDINARY_SP_NAME"`                            // 普通服务商主体全称
	WechatOrdinarySpSerialNumber                  string        `mapstructure:"WECHAT_ORDINARY_SP_SERIAL_NUMBER"`                   // 普通服务商 API 证书序列号
	WechatOrdinarySpPrivateKeyPath                string        `mapstructure:"WECHAT_ORDINARY_SP_PRIVATE_KEY_PATH"`                // 普通服务商 API 私钥文件路径
	WechatOrdinarySpAPIV3Key                      string        `mapstructure:"WECHAT_ORDINARY_SP_API_V3_KEY"`                      // 普通服务商 APIv3 密钥
	WechatOrdinarySpPlatformPublicKeyPath         string        `mapstructure:"WECHAT_ORDINARY_SP_PLATFORM_PUBLIC_KEY_PATH"`        // 普通服务商平台公钥路径
	WechatOrdinarySpPlatformPublicKeyID           string        `mapstructure:"WECHAT_ORDINARY_SP_PLATFORM_PUBLIC_KEY_ID"`          // 普通服务商平台公钥ID
	WechatOrdinaryPaymentNotifyURL                string        `mapstructure:"WECHAT_ORDINARY_PAYMENT_NOTIFY_URL"`                 // 普通服务商支付回调URL
	WechatOrdinaryCombineNotifyURL                string        `mapstructure:"WECHAT_ORDINARY_COMBINE_NOTIFY_URL"`                 // 普通服务商合单回调URL
	WechatOrdinaryRefundNotifyURL                 string        `mapstructure:"WECHAT_ORDINARY_REFUND_NOTIFY_URL"`                  // 普通服务商退款回调URL
	WechatOrdinaryProfitSharingNotifyURL          string        `mapstructure:"WECHAT_ORDINARY_PROFIT_SHARING_NOTIFY_URL"`          // 普通服务商分账回调URL
	WechatOrdinaryViolationNotifyURL              string        `mapstructure:"WECHAT_ORDINARY_VIOLATION_NOTIFY_URL"`               // 普通服务商违规通知回调URL
	WechatOrdinaryApplymentSettlementIDIndividual string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_INDIVIDUAL"` // 普通服务商个体工商户进件结算规则ID
	WechatOrdinaryApplymentSettlementIDEnterprise string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE"` // 普通服务商企业进件结算规则ID
	WechatOrdinaryApplymentQualification          string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_QUALIFICATION_TYPE"`       // 普通服务商进件结算资质类型
	WechatOrdinaryApplymentContactEmail           string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_CONTACT_EMAIL"`            // 普通服务商进件默认联系人邮箱
	WechatOrdinaryApplymentServicePhone           string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_SERVICE_PHONE"`            // 普通服务商进件默认客服电话
	WechatOrdinaryApplymentActivitiesID           string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_ACTIVITIES_ID"`            // 普通服务商进件优惠费率活动ID
	WechatOrdinaryApplymentDebitActivitiesRate    string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_DEBIT_ACTIVITIES_RATE"`    // 普通服务商进件非信用卡活动费率
	WechatOrdinaryApplymentCreditActivitiesRate   string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_CREDIT_ACTIVITIES_RATE"`   // 普通服务商进件信用卡活动费率
	WechatOrdinaryApplymentActivitiesAdditions    string        `mapstructure:"WECHAT_ORDINARY_APPLYMENT_ACTIVITIES_ADDITIONS"`     // 普通服务商进件优惠费率补充材料 media_id，逗号分隔

	// 宝付/宝财通配置。开启 BAOFU_MAIN_BUSINESS_ENABLED 后，主业务支付使用宝付聚合支付，不回退普通服务商或平台收付通。
	BaofuMainBusinessEnabled       bool          `mapstructure:"BAOFU_MAIN_BUSINESS_ENABLED"`
	BaofuEnvironment               string        `mapstructure:"BAOFU_ENVIRONMENT"`
	BaofuAccountGatewayBaseURL     string        `mapstructure:"BAOFU_ACCOUNT_GATEWAY_BASE_URL"`
	BaofuAggregatePayBaseURL       string        `mapstructure:"BAOFU_AGGREGATE_PAY_BASE_URL"`
	BaofuAggregatePayBackupBaseURL string        `mapstructure:"BAOFU_AGGREGATE_PAY_BACKUP_BASE_URL"`
	BaofuMerchantReportBaseURL     string        `mapstructure:"BAOFU_MERCHANT_REPORT_BASE_URL"`
	BaofuCollectMerchantID         string        `mapstructure:"BAOFU_COLLECT_MERCHANT_ID"`
	BaofuCollectTerminalID         string        `mapstructure:"BAOFU_COLLECT_TERMINAL_ID"`
	BaofuPayoutMerchantID          string        `mapstructure:"BAOFU_PAYOUT_MERCHANT_ID"`
	BaofuPayoutTerminalID          string        `mapstructure:"BAOFU_PAYOUT_TERMINAL_ID"`
	BaofuAppID                     string        `mapstructure:"BAOFU_APP_ID"`
	BaofuPrivateKeyPEM             string        `mapstructure:"BAOFU_PRIVATE_KEY_PEM"`
	BaofuPublicKeyPEM              string        `mapstructure:"BAOFU_PUBLIC_KEY_PEM"`
	BaofuSignSerialNo              string        `mapstructure:"BAOFU_SIGN_SERIAL_NO"`
	BaofuEncryptionSerialNo        string        `mapstructure:"BAOFU_ENCRYPTION_SERIAL_NO"`
	BaofuAESKey                    string        `mapstructure:"BAOFU_AES_KEY"`
	BaofuNotifyBaseURL             string        `mapstructure:"BAOFU_NOTIFY_BASE_URL"`
	BaofuPaymentNotifyURL          string        `mapstructure:"BAOFU_PAYMENT_NOTIFY_URL"`
	BaofuRefundNotifyURL           string        `mapstructure:"BAOFU_REFUND_NOTIFY_URL"`
	BaofuHTTPTimeout               time.Duration `mapstructure:"BAOFU_HTTP_TIMEOUT"`

	// 数据加密配置
	DataEncryptionKey string `mapstructure:"DATA_ENCRYPTION_KEY"` // 本地数据加密密钥（16/24/32字节）

	// 腾讯地图配置（运行时必填）
	TencentMapKey string `mapstructure:"TENCENT_MAP_KEY"`

	// 天地图配置（仅历史/离线工具使用）
	TiandituMapKey  string `mapstructure:"TIANDITU_MAP_KEY"`
	TiandituBaseURL string `mapstructure:"TIANDITU_BASE_URL"`

	// Web前端配置
	WebBaseURL string `mapstructure:"WEB_BASE_URL"` // H5页面基础URL，用于分享功能

	// 飞鹅云打印配置（平台统一账号）
	FeieyunEnabled     bool          `mapstructure:"FEIEYUN_ENABLED"`
	FeieyunAPIBaseURL  string        `mapstructure:"FEIEYUN_API_BASE_URL"`
	FeieyunUser        string        `mapstructure:"FEIEYUN_USER"`
	FeieyunUkey        string        `mapstructure:"FEIEYUN_UKEY"`
	FeieyunHTTPTimeout time.Duration `mapstructure:"FEIEYUN_HTTP_TIMEOUT"`

	// 对外服务的基础 URL（生产环境必填）。设置后 API 生成的签名 URL 将以此为前缀，
	// 避免依赖客户端可控的 Origin/Host 头（SSRF/开放重定向防护）。
	// 示例：https://api.example.com
	ExternalBaseURL string `mapstructure:"EXTERNAL_BASE_URL"`

	// Web 登录扫码会话
	WebLoginSessionTTL   time.Duration `mapstructure:"WEB_LOGIN_SESSION_TTL"`
	WebLoginQRSigningKey string        `mapstructure:"WEB_LOGIN_QR_SIGNING_KEY"`

	// 上传文件安全访问（签名URL）
	UploadURLSigningKey string        `mapstructure:"UPLOAD_URL_SIGNING_KEY"` // HMAC签名密钥（建议随机长字符串）
	UploadURLTTL        time.Duration `mapstructure:"UPLOAD_URL_TTL"`         // 签名URL有效期（例如 10m, 1h）

	// 本地文件存储根目录（绝对路径）。图片删除以此为基础拼接相对路径，避免依赖进程工作目录。
	// 默认为空字符串，实际使用时应配置为绝对路径，例如 /app/uploads 对应的上级目录。
	UploadsBaseDir string `mapstructure:"UPLOADS_BASE_DIR"`

	// 数据库连接池参数
	DBMaxConns          int32         `mapstructure:"DB_MAX_CONNS"`
	DBMinConns          int32         `mapstructure:"DB_MIN_CONNS"`
	DBMaxConnLifetime   time.Duration `mapstructure:"DB_MAX_CONN_LIFETIME"`
	DBMaxConnIdleTime   time.Duration `mapstructure:"DB_MAX_CONN_IDLE_TIME"`
	DBHealthCheckPeriod time.Duration `mapstructure:"DB_HEALTH_CHECK_PERIOD"`

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

	// 媒体存储配置
	// FILE_STORAGE_PROVIDER=local 时后端自身充当上传接收端（仅开发环境）。
	// FILE_STORAGE_PROVIDER=oss 时使用阿里云 OSS 直传，生产环境必须设为 oss。
	FileStorageProvider string `mapstructure:"FILE_STORAGE_PROVIDER"` // local | oss

	// 阿里云 OSS 配置（FILE_STORAGE_PROVIDER=oss 时必填）
	OSSEndpoint        string `mapstructure:"OSS_ENDPOINT"`          // OSS 地域端点，如 https://oss-cn-hangzhou.aliyuncs.com
	OSSPublicBucket    string `mapstructure:"OSS_PUBLIC_BUCKET"`     // 公共桶名称
	OSSPrivateBucket   string `mapstructure:"OSS_PRIVATE_BUCKET"`    // 私有桶名称
	OSSAccessKeyID     string `mapstructure:"OSS_ACCESS_KEY_ID"`     // AccessKey ID（服务端使用，不下发客户端）
	OSSAccessKeySecret string `mapstructure:"OSS_ACCESS_KEY_SECRET"` // AccessKey Secret（服务端使用，不下发客户端）
	OSSRegion          string `mapstructure:"OSS_REGION"`            // OSS 地域标识，如 cn-hangzhou（v2 SDK V4 签名必填）

	// 阿里云 CDN 配置
	CDNPublicBaseURL string `mapstructure:"CDN_PUBLIC_BASE_URL"` // 公共图 CDN 域名，如 https://cdn.example.com

	// 阿里云 OCR 配置
	AliyunOCREnabled         bool          `mapstructure:"ALIYUN_OCR_ENABLED"`
	AliyunOCREndpoint        string        `mapstructure:"ALIYUN_OCR_ENDPOINT"`
	AliyunOCRRegion          string        `mapstructure:"ALIYUN_OCR_REGION"`
	AliyunOCRAccessKeyID     string        `mapstructure:"ALIYUN_OCR_ACCESS_KEY_ID"`
	AliyunOCRAccessKeySecret string        `mapstructure:"ALIYUN_OCR_ACCESS_KEY_SECRET"`
	AliyunOCRSTSEnabled      bool          `mapstructure:"ALIYUN_OCR_STS_ENABLED"`
	AliyunOCRRoleARN         string        `mapstructure:"ALIYUN_OCR_ROLE_ARN"`
	AliyunOCRRoleSessionName string        `mapstructure:"ALIYUN_OCR_ROLE_SESSION_NAME"`
	AliyunOCRRoleExternalID  string        `mapstructure:"ALIYUN_OCR_ROLE_EXTERNAL_ID"`
	AliyunOCRHTTPTimeout     time.Duration `mapstructure:"ALIYUN_OCR_HTTP_TIMEOUT"`

	// 媒体访问与上传参数
	PrivateDownloadURLTTL   time.Duration `mapstructure:"PRIVATE_DOWNLOAD_URL_TTL"`   // 私有图签名地址有效期，如 5m
	MediaMaxUploadBytes     int64         `mapstructure:"MEDIA_MAX_UPLOAD_BYTES"`     // 单文件最大字节数，如 10485760（10MB）
	MediaDirectUploadExpire time.Duration `mapstructure:"MEDIA_DIRECT_UPLOAD_EXPIRE"` // 直传凭证有效期，如 15m

	// 图片规格宽度（px）。MediaURLResolver 使用这些值构造 OSS 图片处理参数。
	ImageVariantThumbWidth  int `mapstructure:"IMAGE_VARIANT_THUMB_WIDTH"`  // 列表缩略图，默认 200
	ImageVariantCardWidth   int `mapstructure:"IMAGE_VARIANT_CARD_WIDTH"`   // 商品卡片，默认 400
	ImageVariantDetailWidth int `mapstructure:"IMAGE_VARIANT_DETAIL_WIDTH"` // 详情主图，默认 960
}

func (c Config) EffectiveWechatEcommercePaymentNotifyURL() string {
	return strings.TrimSpace(c.WechatEcommercePaymentNotifyURL)
}

func (c Config) EffectiveWechatEcommerceCombineNotifyURL() string {
	return strings.TrimSpace(c.WechatEcommerceCombineNotifyURL)
}

func (c Config) EffectiveWechatEcommerceRefundNotifyURL() string {
	return strings.TrimSpace(c.WechatEcommerceRefundNotifyURL)
}

func (c Config) EffectiveWechatEcommerceWithdrawNotifyURL() string {
	return strings.TrimSpace(c.WechatEcommerceWithdrawNotifyURL)
}

func (c Config) EffectiveWechatEcommerceViolationNotifyURL() string {
	return strings.TrimSpace(c.WechatEcommerceViolationNotifyURL)
}

func (c Config) EffectiveWechatPayMerchantTransferNotifyURL() string {
	return strings.TrimSpace(c.WechatPayMerchantTransferNotifyURL)
}

func (c Config) EffectiveWechatOrdinaryPaymentNotifyURL() string {
	return strings.TrimSpace(c.WechatOrdinaryPaymentNotifyURL)
}

func (c Config) EffectiveWechatOrdinaryCombineNotifyURL() string {
	return strings.TrimSpace(c.WechatOrdinaryCombineNotifyURL)
}

func (c Config) EffectiveWechatOrdinaryRefundNotifyURL() string {
	return strings.TrimSpace(c.WechatOrdinaryRefundNotifyURL)
}

func (c Config) EffectiveWechatOrdinaryProfitSharingNotifyURL() string {
	return strings.TrimSpace(c.WechatOrdinaryProfitSharingNotifyURL)
}

func (c Config) EffectiveWechatOrdinaryViolationNotifyURL() string {
	return strings.TrimSpace(c.WechatOrdinaryViolationNotifyURL)
}

func (c Config) HasWechatPayRuntimeConfig() bool {
	return strings.TrimSpace(c.WechatPayMchID) != "" ||
		strings.TrimSpace(c.WechatPaySerialNumber) != "" ||
		strings.TrimSpace(c.WechatPayPrivateKeyPath) != "" ||
		strings.TrimSpace(c.WechatPayAPIV3Key) != "" ||
		strings.TrimSpace(c.WechatPayPlatformPublicKeyPath) != "" ||
		strings.TrimSpace(c.WechatPayPlatformPublicKeyID) != ""
}

func (c Config) HasWechatEcommerceRuntimeConfig() bool {
	return strings.TrimSpace(c.WechatEcommerceSpMchID) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpAppID) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpSerialNumber) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpPrivateKeyPath) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpAPIV3Key) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpPlatformPublicKeyPath) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpPlatformPublicKeyID) != "" ||
		strings.TrimSpace(c.WechatEcommerceSpName) != ""
}

func (c Config) HasWechatOrdinaryServiceProviderRuntimeConfig() bool {
	return strings.TrimSpace(c.WechatOrdinarySpMchID) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpAppID) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpSerialNumber) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpPrivateKeyPath) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpAPIV3Key) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpPlatformPublicKeyPath) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpPlatformPublicKeyID) != "" ||
		strings.TrimSpace(c.WechatOrdinarySpName) != "" ||
		strings.TrimSpace(c.WechatOrdinaryPaymentNotifyURL) != "" ||
		strings.TrimSpace(c.WechatOrdinaryCombineNotifyURL) != "" ||
		strings.TrimSpace(c.WechatOrdinaryRefundNotifyURL) != "" ||
		strings.TrimSpace(c.WechatOrdinaryProfitSharingNotifyURL) != "" ||
		strings.TrimSpace(c.WechatOrdinaryViolationNotifyURL) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentSettlementIDIndividual) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentSettlementIDEnterprise) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentQualification) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentContactEmail) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentServicePhone) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentActivitiesID) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentDebitActivitiesRate) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentCreditActivitiesRate) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentActivitiesAdditions) != ""
}

func (c Config) HasBaofuRuntimeConfig() bool {
	return c.BaofuMainBusinessEnabled ||
		strings.TrimSpace(c.BaofuAccountGatewayBaseURL) != "" ||
		strings.TrimSpace(c.BaofuAggregatePayBaseURL) != "" ||
		strings.TrimSpace(c.BaofuAggregatePayBackupBaseURL) != "" ||
		strings.TrimSpace(c.BaofuMerchantReportBaseURL) != "" ||
		strings.TrimSpace(c.BaofuCollectMerchantID) != "" ||
		strings.TrimSpace(c.BaofuCollectTerminalID) != "" ||
		strings.TrimSpace(c.BaofuPayoutMerchantID) != "" ||
		strings.TrimSpace(c.BaofuPayoutTerminalID) != "" ||
		strings.TrimSpace(c.BaofuAppID) != "" ||
		strings.TrimSpace(c.BaofuPrivateKeyPEM) != "" ||
		strings.TrimSpace(c.BaofuPublicKeyPEM) != "" ||
		strings.TrimSpace(c.BaofuSignSerialNo) != "" ||
		strings.TrimSpace(c.BaofuEncryptionSerialNo) != "" ||
		strings.TrimSpace(c.BaofuAESKey) != "" ||
		strings.TrimSpace(c.BaofuNotifyBaseURL) != "" ||
		strings.TrimSpace(c.BaofuPaymentNotifyURL) != "" ||
		strings.TrimSpace(c.BaofuRefundNotifyURL) != ""
}

func (c Config) ToBaofuConfig() baofu.Config {
	return baofu.Config{
		Environment:               c.BaofuEnvironment,
		AccountGatewayBaseURL:     c.BaofuAccountGatewayBaseURL,
		AggregatePayBaseURL:       c.BaofuAggregatePayBaseURL,
		AggregatePayBackupBaseURL: c.BaofuAggregatePayBackupBaseURL,
		MerchantReportBaseURL:     c.BaofuMerchantReportBaseURL,
		CollectMerchantID:         c.BaofuCollectMerchantID,
		CollectTerminalID:         c.BaofuCollectTerminalID,
		PayoutMerchantID:          c.BaofuPayoutMerchantID,
		PayoutTerminalID:          c.BaofuPayoutTerminalID,
		AppID:                     c.BaofuAppID,
		PrivateKeyPEM:             c.BaofuPrivateKeyPEM,
		BaofuPublicKeyPEM:         c.BaofuPublicKeyPEM,
		SignSerialNo:              c.BaofuSignSerialNo,
		EncryptionSerialNo:        c.BaofuEncryptionSerialNo,
		AESKey:                    c.BaofuAESKey,
		NotifyBaseURL:             c.BaofuNotifyBaseURL,
		Timeout:                   c.BaofuHTTPTimeout,
	}
}

func (c Config) EffectiveBaofuPaymentNotifyURL() string {
	return strings.TrimSpace(c.BaofuPaymentNotifyURL)
}

func (c Config) EffectiveBaofuRefundNotifyURL() string {
	return strings.TrimSpace(c.BaofuRefundNotifyURL)
}

func (c Config) ValidateBaofuConfig() error {
	if !c.HasBaofuRuntimeConfig() {
		return nil
	}
	if strings.TrimSpace(c.WechatMiniAppID) == "" {
		return fmt.Errorf("WECHAT_MINI_APP_ID is required when baofu main business pay is enabled")
	}
	if err := c.ToBaofuConfig().Validate(); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("BAOFU_PAYMENT_NOTIFY_URL", c.BaofuPaymentNotifyURL, "baofu main business pay is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("BAOFU_REFUND_NOTIFY_URL", c.BaofuRefundNotifyURL, "baofu main business pay is enabled"); err != nil {
		return err
	}
	return nil
}

func (c Config) ValidateWechatPayConfig() error {
	if !c.HasWechatPayRuntimeConfig() {
		return nil
	}

	if c.WechatPayMchID == "" || c.WechatPaySerialNumber == "" || c.WechatPayPrivateKeyPath == "" || c.WechatPayAPIV3Key == "" {
		return fmt.Errorf("WECHAT_PAY_MCH_ID, WECHAT_PAY_SERIAL_NUMBER, WECHAT_PAY_PRIVATE_KEY_PATH and WECHAT_PAY_API_V3_KEY are required when wechat pay is enabled")
	}

	if strings.TrimSpace(c.WechatMiniAppID) == "" {
		return fmt.Errorf("WECHAT_MINI_APP_ID is required when wechat pay is enabled")
	}

	if err := validateAbsoluteConfigURL("WECHAT_PAY_NOTIFY_URL", c.WechatPayNotifyURL); err != nil {
		return err
	}

	if err := validateAbsoluteConfigURL("WECHAT_PAY_REFUND_NOTIFY_URL", c.WechatPayRefundNotifyURL); err != nil {
		return err
	}

	if err := validateAbsoluteConfigURL("WECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL", c.WechatPayMerchantTransferNotifyURL); err != nil {
		return err
	}

	if c.WechatPayPlatformPublicKeyPath == "" || c.WechatPayPlatformPublicKeyID == "" {
		return fmt.Errorf("WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH and WECHAT_PAY_PLATFORM_PUBLIC_KEY_ID are required when wechat pay is enabled")
	}

	return nil
}

func validateAbsoluteConfigURL(fieldName string, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%s is required when wechat pay is enabled", fieldName)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid absolute URL when wechat pay is enabled", fieldName)
	}

	return nil
}

func validateRequiredAbsoluteConfigURL(fieldName string, raw string, scope string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%s is required when %s", fieldName, scope)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid absolute URL when %s", fieldName, scope)
	}

	return nil
}

func (c Config) ValidateWechatEcommerceConfig() error {
	if !c.HasWechatEcommerceRuntimeConfig() {
		return nil
	}

	if c.WechatEcommerceSpMchID == "" || c.WechatEcommerceSpAppID == "" {
		return fmt.Errorf("WECHAT_ECOMMERCE_SP_MCHID and WECHAT_ECOMMERCE_SP_APPID are required when wechat pay is enabled")
	}

	if c.WechatEcommerceSpSerialNumber == "" || c.WechatEcommerceSpPrivateKeyPath == "" || c.WechatEcommerceSpAPIV3Key == "" {
		return fmt.Errorf("WECHAT_ECOMMERCE_SP_SERIAL_NUMBER, WECHAT_ECOMMERCE_SP_PRIVATE_KEY_PATH and WECHAT_ECOMMERCE_SP_API_V3_KEY are required when wechat ecommerce is enabled")
	}

	if c.WechatEcommerceSpPlatformPublicKeyPath == "" || c.WechatEcommerceSpPlatformPublicKeyID == "" {
		return fmt.Errorf("WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_PATH and WECHAT_ECOMMERCE_SP_PLATFORM_PUBLIC_KEY_ID are required when wechat ecommerce is enabled")
	}

	if err := validateRequiredAbsoluteConfigURL("WECHAT_ECOMMERCE_PAYMENT_NOTIFY_URL", c.WechatEcommercePaymentNotifyURL, "wechat ecommerce is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ECOMMERCE_COMBINE_NOTIFY_URL", c.WechatEcommerceCombineNotifyURL, "wechat ecommerce is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ECOMMERCE_REFUND_NOTIFY_URL", c.WechatEcommerceRefundNotifyURL, "wechat ecommerce is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL", c.WechatEcommerceWithdrawNotifyURL, "wechat ecommerce is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ECOMMERCE_VIOLATION_NOTIFY_URL", c.WechatEcommerceViolationNotifyURL, "wechat ecommerce is enabled"); err != nil {
		return err
	}

	return nil
}

func (c Config) ValidateWechatOrdinaryServiceProviderConfig() error {
	if !c.HasWechatOrdinaryServiceProviderRuntimeConfig() {
		return nil
	}

	if c.WechatOrdinarySpMchID == "" || c.WechatOrdinarySpAppID == "" {
		return fmt.Errorf("WECHAT_ORDINARY_SP_MCHID and WECHAT_ORDINARY_SP_APPID are required when ordinary service provider pay is enabled")
	}
	if strings.TrimSpace(c.WechatMiniAppID) == "" {
		return fmt.Errorf("WECHAT_MINI_APP_ID is required when ordinary service provider pay is enabled")
	}
	if strings.TrimSpace(c.WechatOrdinarySpAppID) != strings.TrimSpace(c.WechatMiniAppID) {
		return fmt.Errorf("WECHAT_ORDINARY_SP_APPID must match WECHAT_MINI_APP_ID when ordinary service provider pay uses the project mini program appid")
	}

	if c.WechatOrdinarySpSerialNumber == "" || c.WechatOrdinarySpPrivateKeyPath == "" || c.WechatOrdinarySpAPIV3Key == "" {
		return fmt.Errorf("WECHAT_ORDINARY_SP_SERIAL_NUMBER, WECHAT_ORDINARY_SP_PRIVATE_KEY_PATH and WECHAT_ORDINARY_SP_API_V3_KEY are required when ordinary service provider pay is enabled")
	}

	if c.WechatOrdinarySpPlatformPublicKeyPath == "" || c.WechatOrdinarySpPlatformPublicKeyID == "" {
		return fmt.Errorf("WECHAT_ORDINARY_SP_PLATFORM_PUBLIC_KEY_PATH and WECHAT_ORDINARY_SP_PLATFORM_PUBLIC_KEY_ID are required when ordinary service provider pay is enabled")
	}

	if err := validateRequiredAbsoluteConfigURL("WECHAT_ORDINARY_PAYMENT_NOTIFY_URL", c.WechatOrdinaryPaymentNotifyURL, "ordinary service provider pay is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ORDINARY_COMBINE_NOTIFY_URL", c.WechatOrdinaryCombineNotifyURL, "ordinary service provider pay is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ORDINARY_REFUND_NOTIFY_URL", c.WechatOrdinaryRefundNotifyURL, "ordinary service provider pay is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ORDINARY_PROFIT_SHARING_NOTIFY_URL", c.WechatOrdinaryProfitSharingNotifyURL, "ordinary service provider pay is enabled"); err != nil {
		return err
	}
	if err := validateRequiredAbsoluteConfigURL("WECHAT_ORDINARY_VIOLATION_NOTIFY_URL", c.WechatOrdinaryViolationNotifyURL, "ordinary service provider pay is enabled"); err != nil {
		return err
	}
	if strings.TrimSpace(c.WechatOrdinaryApplymentSettlementIDIndividual) == "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentSettlementIDEnterprise) == "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentQualification) == "" {
		return fmt.Errorf("WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_INDIVIDUAL, WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE and WECHAT_ORDINARY_APPLYMENT_QUALIFICATION_TYPE are required when ordinary service provider applyment is enabled")
	}
	if strings.TrimSpace(c.WechatOrdinaryApplymentContactEmail) == "" {
		return fmt.Errorf("WECHAT_ORDINARY_APPLYMENT_CONTACT_EMAIL is required when ordinary service provider applyment is enabled")
	}
	if err := validateOrdinaryApplymentActivitiesConfig(c); err != nil {
		return err
	}

	return nil
}

func validateOrdinaryApplymentActivitiesConfig(c Config) error {
	hasActivitiesConfig := strings.TrimSpace(c.WechatOrdinaryApplymentActivitiesID) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentDebitActivitiesRate) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentCreditActivitiesRate) != "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentActivitiesAdditions) != ""
	if !hasActivitiesConfig {
		return nil
	}

	if strings.TrimSpace(c.WechatOrdinaryApplymentActivitiesID) == "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentDebitActivitiesRate) == "" ||
		strings.TrimSpace(c.WechatOrdinaryApplymentCreditActivitiesRate) == "" {
		return fmt.Errorf("WECHAT_ORDINARY_APPLYMENT_ACTIVITIES_ID, WECHAT_ORDINARY_APPLYMENT_DEBIT_ACTIVITIES_RATE and WECHAT_ORDINARY_APPLYMENT_CREDIT_ACTIVITIES_RATE are required when ordinary service provider applyment discount activities are configured")
	}

	return nil
}

// LoadConfig reads configuration from file or environment variables.
// Each call creates an isolated viper instance so multiple invocations
// (e.g. parallel tests) do not share or pollute global state.
func LoadConfig(path string) (config Config, err error) {
	v := viper.New()
	v.AddConfigPath(path)
	v.SetConfigName("app")
	v.SetConfigType("env")

	v.AutomaticEnv()
	v.SetDefault("AUTO_MIGRATE", false)
	v.SetDefault("REDIS_REQUIRED", false)
	v.SetDefault("LOG_LEVEL", "info")
	// WebSocket rollout defaults
	v.SetDefault("WS_RELIABLE_ENABLED", true)
	v.SetDefault("WS_RELIABLE_PERCENT", 100)
	v.SetDefault("RULES_ENGINE_ENABLED", false)
	// Geofence defaults
	v.SetDefault("GEOFENCE_RADIUS_M", 80)
	v.SetDefault("GEOFENCE_DWELL_MIN_SECONDS", 60)
	v.SetDefault("GEOFENCE_DWELL_MIN_SAMPLES", 3)
	v.SetDefault("GEOFENCE_MIN_ACCURACY_M", 80)
	v.SetDefault("GEOFENCE_AUTO_ADVANCE_ENABLED", false)
	v.SetDefault("GEOFENCE_AUTO_PICKUP_ENABLED", false)
	v.SetDefault("GEOFENCE_AUTO_DELIVER_ENABLED", false)
	// Delivery defaults
	v.SetDefault("RIDER_AVERAGE_SPEED", 15000)
	v.SetDefault("DEFAULT_PREPARE_TIME", 20)
	// Profit sharing return retry defaults
	v.SetDefault("PROFIT_SHARING_RETURN_RETRY_INTERVAL", "1m")
	v.SetDefault("PROFIT_SHARING_RETURN_MAX_RETRIES", 10)
	v.SetDefault("RESERVATION_USER_REFUND_PERCENT_BEFORE_DEADLINE", 100)
	v.SetDefault("RESERVATION_USER_REFUND_PERCENT_AFTER_DEADLINE", 0)
	v.SetDefault("RESERVATION_MERCHANT_REFUND_PERCENT_BEFORE_DEADLINE", 100)
	v.SetDefault("RESERVATION_MERCHANT_REFUND_PERCENT_AFTER_DEADLINE", 100)
	// Web 登录默认过期时间
	v.SetDefault("WEB_LOGIN_SESSION_TTL", "5m")
	v.SetDefault("FEIEYUN_ENABLED", false)
	v.SetDefault("FEIEYUN_API_BASE_URL", "https://api.feieyun.cn")
	v.SetDefault("FEIEYUN_HTTP_TIMEOUT", "5s")
	v.SetDefault("BAOFU_MAIN_BUSINESS_ENABLED", false)
	v.SetDefault("BAOFU_HTTP_TIMEOUT", "30s")
	// 媒体存储默认值
	v.SetDefault("FILE_STORAGE_PROVIDER", "local")
	v.SetDefault("PRIVATE_DOWNLOAD_URL_TTL", "5m")
	v.SetDefault("MEDIA_MAX_UPLOAD_BYTES", 10485760) // 10MB
	v.SetDefault("MEDIA_DIRECT_UPLOAD_EXPIRE", "15m")
	v.SetDefault("IMAGE_VARIANT_THUMB_WIDTH", 200)
	v.SetDefault("IMAGE_VARIANT_CARD_WIDTH", 400)
	v.SetDefault("IMAGE_VARIANT_DETAIL_WIDTH", 960)
	v.SetDefault("ALIYUN_OCR_ENABLED", false)
	v.SetDefault("ALIYUN_OCR_STS_ENABLED", false)
	v.SetDefault("ALIYUN_OCR_HTTP_TIMEOUT", "30s")

	// 数据库连接池默认值
	v.SetDefault("DB_MAX_CONNS", 25)
	v.SetDefault("DB_MIN_CONNS", 5)
	v.SetDefault("DB_MAX_CONN_LIFETIME", "1h")
	v.SetDefault("DB_MAX_CONN_IDLE_TIME", "30m")
	v.SetDefault("DB_HEALTH_CHECK_PERIOD", "1m")

	err = v.ReadInConfig()
	if err != nil {
		return
	}

	err = v.Unmarshal(&config)
	if err != nil {
		return
	}

	// Normalize common quoted values from .env (e.g. REDIS_PASSWORD="...")
	config.RedisPassword = trimOptionalQuotes(config.RedisPassword)
	return
}

// ValidateAliyunOCRConfig validates Aliyun OCR startup configuration.
func (config Config) ValidateAliyunOCRConfig() error {
	if !config.AliyunOCREnabled {
		return nil
	}
	if config.AliyunOCREndpoint == "" {
		return fmt.Errorf("ALIYUN_OCR_ENDPOINT is required when ALIYUN_OCR_ENABLED=true")
	}
	if config.AliyunOCRRegion == "" {
		return fmt.Errorf("ALIYUN_OCR_REGION is required when ALIYUN_OCR_ENABLED=true")
	}
	if config.AliyunOCRHTTPTimeout <= 0 {
		return fmt.Errorf("ALIYUN_OCR_HTTP_TIMEOUT must be > 0 when ALIYUN_OCR_ENABLED=true")
	}
	if config.AliyunOCRSTSEnabled {
		if config.AliyunOCRRoleARN == "" {
			return fmt.Errorf("ALIYUN_OCR_ROLE_ARN is required when ALIYUN_OCR_STS_ENABLED=true")
		}
		if config.AliyunOCRRoleSessionName == "" {
			return fmt.Errorf("ALIYUN_OCR_ROLE_SESSION_NAME is required when ALIYUN_OCR_STS_ENABLED=true")
		}
		return nil
	}
	if config.AliyunOCRAccessKeyID == "" {
		return fmt.Errorf("ALIYUN_OCR_ACCESS_KEY_ID is required when ALIYUN_OCR_ENABLED=true and STS is disabled")
	}
	if config.AliyunOCRAccessKeySecret == "" {
		return fmt.Errorf("ALIYUN_OCR_ACCESS_KEY_SECRET is required when ALIYUN_OCR_ENABLED=true and STS is disabled")
	}
	return nil
}

func trimOptionalQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return s
}
