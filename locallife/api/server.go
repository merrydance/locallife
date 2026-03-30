package api

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/docs"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/weather"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// MessageResponse 通用消息响应
type MessageResponse struct {
	Message string `json:"message" example:"ok"`
}

type healthCheckResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type readinessCheckResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database"`
}

type serviceUnavailableResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type successMessageResponse struct {
	Message string `json:"message"`
}

// Server serves HTTP requests for our banking service.
type Server struct {
	config             util.Config
	store              db.Store
	tokenMaker         token.Maker
	auditWriter        AuditWriter
	wechatClient       wechat.WechatClient
	paymentClient      wechat.PaymentClientInterface   // 小程序直连支付（押金、充值）
	ecommerceClient    wechat.EcommerceClientInterface // 平台收付通（订单支付分账）
	dataEncryptor      util.DataEncryptor              // 敏感数据加密器（本地存储加密）
	mapClient          maps.TencentMapClientInterface  // 地图客户端（自建 OSM）
	weatherCache       weather.WeatherCache
	taskDistributor    worker.TaskDistributor
	wsHub              *websocket.Hub           // WebSocket连接管理（骑手和商户）
	wsPubSub           *websocket.PubSubManager // Redis Pub/Sub管理（跨进程推送）
	deliveryBroadcast  *logic.DeliveryBroadcastLogic
	rateLimiter        *RateLimiter
	mediaRegistry      *media.Registry
	mediaResolver      *media.URLResolver
	imageDeleter       *imageDeleteWorker   // 有界异步图片删除 worker pool
	keywordWorker      *searchKeywordWorker // 有界异步搜索关键词记录 worker pool
	rulesEngine        rules.Engine
	routeService       *logic.RouteService
	orderCommandSvc    logic.OrderCommandService
	orderQuerySvc      logic.OrderQueryService
	paymentFacade      logic.PaymentFacade
	refundOrchestrator logic.RefundOrchestrator
	mediaStorage       media.ObjectStorage
	router             *gin.Engine
}

// SetPaymentClientForTest injects a payment client in tests.
// It rebuilds the cached order services immediately so they pick up the new
// client; this prevents nil-pointer panics in handlers that access
// orderCommandSvc / orderQuerySvc directly.
func (server *Server) SetPaymentClientForTest(client wechat.PaymentClientInterface) {
	server.paymentClient = client
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
}

// SetTaskDistributorForTest injects a task distributor in tests.
func (server *Server) SetTaskDistributorForTest(distributor worker.TaskDistributor) {
	server.taskDistributor = distributor
}

// SetEcommerceClientForTest injects an ecommerce client in tests.
// It also clears the cached paymentFacade and refundOrchestrator so they are
// rebuilt with the new client on the next request.
func (server *Server) SetEcommerceClientForTest(client wechat.EcommerceClientInterface) {
	server.ecommerceClient = client
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
	server.paymentFacade = nil
	server.refundOrchestrator = nil
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(config util.Config, store db.Store, weatherCache weather.WeatherCache, taskDistributor worker.TaskDistributor, auditWriter AuditWriter) (*Server, error) {
	if taskDistributor == nil {
		taskDistributor = worker.NewNoopTaskDistributor()
	}
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	wechatClient := wechat.NewClient(config.WechatMiniAppID, config.WechatMiniAppSecret, store)

	// 创建微信支付客户端（如果配置了支付参数）
	var paymentClient wechat.PaymentClientInterface
	var ecommerceClient wechat.EcommerceClientInterface
	if config.WechatPayMchID != "" && config.WechatPayPrivateKeyPath != "" {
		if config.WechatEcommerceSpMchID == "" || config.WechatEcommerceSpAppID == "" {
			return nil, fmt.Errorf("WECHAT_ECOMMERCE_SP_MCHID and WECHAT_ECOMMERCE_SP_APPID are required when wechat pay is enabled")
		}

		// 小程序直连支付客户端（用于押金、充值等）
		paymentClient, err = wechat.NewPaymentClient(wechat.PaymentClientConfig{
			MchID:                   config.WechatPayMchID,
			AppID:                   config.WechatMiniAppID,
			SerialNumber:            config.WechatPaySerialNumber,
			HTTPTimeout:             config.WechatPayHTTPTimeout,
			PrivateKeyPath:          config.WechatPayPrivateKeyPath,
			APIV3Key:                config.WechatPayAPIV3Key,
			NotifyURL:               config.WechatPayNotifyURL,
			RefundNotifyURL:         config.WechatPayRefundNotifyURL,
			PlatformCertificatePath: config.WechatPayPlatformCertificatePath,
			PlatformPublicKeyPath:   config.WechatPayPlatformPublicKeyPath,
			PlatformPublicKeyID:     config.WechatPayPlatformPublicKeyID,
		})
		if err != nil {
			return nil, fmt.Errorf("cannot create payment client: %w", err)
		}

		// 平台收付通客户端（用于订单支付分账）
		ecommerceClient, err = wechat.NewEcommerceClient(wechat.EcommerceClientConfig{
			PaymentClientConfig: wechat.PaymentClientConfig{
				MchID:                   config.WechatPayMchID,
				AppID:                   config.WechatMiniAppID,
				SerialNumber:            config.WechatPaySerialNumber,
				HTTPTimeout:             config.WechatPayHTTPTimeout,
				PrivateKeyPath:          config.WechatPayPrivateKeyPath,
				APIV3Key:                config.WechatPayAPIV3Key,
				NotifyURL:               config.WechatPayNotifyURL,
				RefundNotifyURL:         config.WechatPayRefundNotifyURL,
				PlatformCertificatePath: config.WechatPayPlatformCertificatePath,
				PlatformPublicKeyPath:   config.WechatPayPlatformPublicKeyPath,
				PlatformPublicKeyID:     config.WechatPayPlatformPublicKeyID,
			},
			SpMchID: config.WechatEcommerceSpMchID,
			SpAppID: config.WechatEcommerceSpAppID,
		})
		if err != nil {
			return nil, fmt.Errorf("cannot create ecommerce client: %w", err)
		}
	}

	// 创建 LBS 地图客户端（统一使用腾讯地图）
	var mapClient maps.TencentMapClientInterface
	if config.TencentMapKey != "" {
		mapClient = maps.NewTencentMapClient(config.TencentMapKey)
		log.Info().Str("provider", maps.MapProviderTencent).Msg("✅ LBS initialized with Tencent Maps")
	} else {
		log.Warn().Msg("⚠️ TENCENT_MAP_KEY not configured, map features will be disabled")
	}

	// 创建本地数据加密器（用于加密存储敏感信息）
	var dataEncryptor util.DataEncryptor
	if config.DataEncryptionKey != "" {
		dataEncryptor, err = util.NewAESEncryptor(config.DataEncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("cannot create data encryptor: %w", err)
		}
		log.Info().Msg("✅ Data encryptor initialized for sensitive data storage")
	} else {
		if config.Environment == "production" {
			return nil, fmt.Errorf("DATA_ENCRYPTION_KEY is required in production")
		}
		log.Warn().Msg("⚠️ DATA_ENCRYPTION_KEY not configured, sensitive data will be stored in plaintext")
	}

	// 创建WebSocket Hub（管理骑手和商户的实时连接）
	hubOptions := []websocket.HubOption{
		websocket.WithMetricsRecorder(WSMetricsRecorder{}),
		websocket.WithReliableGate(wsReliableGate(config.WebSocketReliableEnabled, config.WebSocketReliablePercent)),
	}
	if config.RedisAddress != "" {
		wsRedisClient := redis.NewClient(&redis.Options{
			Addr:     config.RedisAddress,
			Password: config.RedisPassword,
		})
		// 背压队列、消息回放、ACK 去重均使用 Redis，跨进程/重启均有效。
		hubOptions = append(hubOptions,
			websocket.WithQueueStore(websocket.NewRedisQueueStore(wsRedisClient, 30*time.Minute, 200)),
			websocket.WithMessageStore(websocket.NewRedisMessageStore(wsRedisClient, 30*time.Minute, 200)),
			websocket.WithAckStore(websocket.NewRedisAckStore(wsRedisClient, 30*time.Minute)),
		)
	} else {
		hubOptions = append(hubOptions, websocket.WithQueueStore(websocket.NewMemoryQueueStore(30*time.Minute, 200, time.Now)))
	}

	// 骑手回放过滤器：delivery_pool_new 类消息仅当订单仍在配送池（未被抢）时才回放。
	hubOptions = append(hubOptions, websocket.WithReplayFilter(
		func(ctx context.Context, info websocket.ClientInfo, msg websocket.Message) bool {
			if info.ClientType != websocket.ClientTypeRider {
				return true // 非骑手客户端不做业务过滤
			}
			if msg.Type != websocket.MessageTypeDeliveryPoolNew {
				return true // 只过滤配送池新单通知，其他消息正常回放
			}
			// 解析消息中的 order_id
			var payload struct {
				OrderID int64 `json:"order_id"`
			}
			if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.OrderID == 0 {
				return false // 无法解析则丢弃，避免推送无效单
			}
			// 查询配送池：若记录已被删除则说明订单已被抢或已取消，跳过回放
			_, err := store.GetDeliveryPoolByOrderID(ctx, payload.OrderID)
			return err == nil
		},
	))
	wsHub := websocket.NewHub(context.Background(), hubOptions...)

	// 创建Redis Pub/Sub管理器（用于跨进程推送通知）
	var wsPubSub *websocket.PubSubManager
	if config.RedisAddress != "" {
		var err error
		wsPubSub, err = websocket.NewPubSubManager(config.RedisAddress, config.RedisPassword, wsHub)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create PubSub manager, WebSocket push will be disabled")
		} else {
			wsPubSub.Start()
			log.Info().Msg("✅ WebSocket PubSub manager started")
		}
	}

	// 初始化 Casbin 权限控制（仅当尚未初始化时）
	if GetGlobalCasbinEnforcer() == nil {
		if err := InitCasbin("casbin"); err != nil {
			return nil, fmt.Errorf("failed to initialize Casbin: %w", err)
		}
	}

	var engine rules.Engine = rules.NewNoopEngine()
	if config.RulesEngineEnabled {
		engine = NewDBRulesEngine(store)
	}

	if auditWriter == nil {
		auditWriter = NewDBAuditWriter(store)
	}

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		auditWriter:     auditWriter,
		wechatClient:    wechatClient,
		paymentClient:   paymentClient,
		ecommerceClient: ecommerceClient,
		dataEncryptor:   dataEncryptor,
		mapClient:       mapClient,
		weatherCache:    weatherCache,
		taskDistributor: taskDistributor,
		wsHub:           wsHub,
		wsPubSub:        wsPubSub,
		rulesEngine:     engine,
		imageDeleter:    newImageDeleteWorker(),
		keywordWorker:   newSearchKeywordWorker(store),
	}
	server.orderCommandSvc = server.buildOrderCommandService()
	server.orderQuerySvc = server.buildOrderQueryService()
	server.paymentFacade = server.buildPaymentFacade()
	server.refundOrchestrator = server.buildRefundOrchestrator()

	if wsPubSub != nil {
		server.deliveryBroadcast = logic.NewDeliveryBroadcastLogic(store, wsPubSub.GetRedisClient())
	}

	server.routeService = logic.NewRouteService(mapClient)

	// 初始化媒体中心
	var mediaStorage media.ObjectStorage
	if config.FileStorageProvider == "oss" {
		mediaStorage, err = media.NewOSSStorage(
			config.OSSEndpoint,
			config.OSSRegion,
			config.OSSAccessKeyID,
			config.OSSAccessKeySecret,
			config.OSSPublicBucket,
			config.OSSPrivateBucket,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create OSS storage: %w", err)
		}
		log.Info().Msg("✅ Media storage initialized with Aliyun OSS")
	} else {
		mediaStorage = media.NewLocalStorage(config.ExternalBaseURL, "uploads/dev")
		log.Info().Msg("✅ Media storage initialized with local fallback")
	}
	server.mediaStorage = mediaStorage
	server.mediaRegistry = media.NewRegistry(store, mediaStorage)
	server.mediaResolver = media.NewURLResolver(media.ResolverConfig{
		CDNPublicBaseURL: config.CDNPublicBaseURL,
		ThumbWidth:       config.ImageVariantThumbWidth,
		CardWidth:        config.ImageVariantCardWidth,
		DetailWidth:      config.ImageVariantDetailWidth,
	}, mediaStorage)

	server.setupRouter()

	// 本地开发模式：在 /v1/media/_devupload 注册直传代理（模拟 OSS 直传，不需要认证）
	if config.FileStorageProvider != "oss" {
		if ls, ok := mediaStorage.(*media.LocalStorage); ok {
			server.router.POST("/v1/media/_devupload", gin.WrapF(ls.DevUploadHandler()))
		}
	}

	return server, nil
}

// Handler exposes the HTTP handler for integration tests.
func (server *Server) Handler() http.Handler {
	return server.router
}

func wsReliableGate(enabled bool, percent int) func(websocket.ClientInfo) bool {
	if !enabled {
		return func(websocket.ClientInfo) bool { return false }
	}
	if percent <= 0 {
		return func(websocket.ClientInfo) bool { return false }
	}
	if percent >= 100 {
		return func(websocket.ClientInfo) bool { return true }
	}

	return func(info websocket.ClientInfo) bool {
		h := fnv.New32a()
		_, _ = h.Write([]byte(string(info.ClientType)))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(info.EntityID, 10)))
		bucket := int(h.Sum32() % 100)
		return bucket < percent
	}
}

// GetWebSocketHub returns the WebSocket hub for external access
func (server *Server) GetWebSocketHub() *websocket.Hub {
	return server.wsHub
}

// Shutdown releases server-side resources created outside the HTTP server.
func (server *Server) Shutdown() {
	if server.wsPubSub != nil {
		server.wsPubSub.Stop()
	}
	if server.wsHub != nil {
		server.wsHub.Shutdown()
	}
	if server.rateLimiter != nil {
		server.rateLimiter.Stop()
	}
	if server.imageDeleter != nil {
		server.imageDeleter.shutdown()
	}
	if server.keywordWorker != nil {
		server.keywordWorker.shutdown()
	}
	if c, ok := server.auditWriter.(interface{ Close() }); ok {
		c.Close()
	}
}

func (server *Server) setupRouter() {
	// 🚀 生产环境设置 Release 模式
	if server.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	// Limit in-memory multipart parsing to reduce RAM spikes under concurrent uploads.
	// Parts larger than this will be stored in temporary files.
	router.MaxMultipartMemory = 8 << 20 // 8 MiB

	// 🖼️ 上传文件访问（仅本地开发模式启用）
	// 生产环境使用对象存储直链/短期访问地址，此路由无需开放。
	// local 模式下仅通过 dev-only 路由暴露调试读路径，不再复用 /uploads/* 公共契约。
	if server.config.FileStorageProvider == "local" {
		router.GET(devUploadsRoutePrefix+"*filepath", server.serveDevUploadFile)
	}

	// 📝 注册自定义验证器
	registerCustomValidators()

	// 🌐 跨域资源共享中间件
	router.Use(CORSMiddleware(server.config.AllowedOrigins))

	// 🔒 安全响应头中间件（防止 XSS、点击劫持等）
	router.Use(SecurityHeadersMiddleware())

	// 🔒 HSTS 中间件（强制 HTTPS）
	if server.config.Environment == "production" {
		router.Use(HSTSMiddleware(31536000))
	}

	// 📊 请求追踪中间件（生成 X-Request-ID）
	router.Use(RequestTracingMiddleware())
	router.Use(RequestLoggingMiddleware())

	// 📈 Prometheus 指标中间件
	router.Use(PrometheusMiddleware())

	// 🛡️ 速率限制中间件（防止 DDoS）
	// 说明：集成测试在同一进程内会快速串行/并行触发大量请求，
	// 为避免 429 干扰业务旅程验收，在 test 环境禁用该中间件。
	var rateLimiter *RateLimiter
	if server.config.Environment != "test" {
		rateLimiter = NewRateLimiter(DefaultRateLimiterConfig())
		server.rateLimiter = rateLimiter
		router.Use(rateLimiter.Middleware())
	}

	// 🕐 全局超时中间件：防止慢查询、外部API卡死导致goroutine泄漏
	router.Use(TimeoutMiddleware(30 * time.Second))

	// 📊 Prometheus 指标端点（供监控系统抓取）
	router.GET("/metrics", MetricsHandler())

	// 🏥 健康检查端点（供 Nginx/K8s 使用，无需认证）
	router.GET("/health", server.healthCheck)
	router.GET("/ready", server.readinessCheck)

	// Swagger API 文档（开发环境）
	if server.config.Environment == "development" {
		// 用运行时配置覆盖 swag 注解中硬编码的 localhost:8080，
		// 使 Swagger UI 在任意开发/测试环境中均能正确请求 API。
		if server.config.ExternalBaseURL != "" {
			docs.SwaggerInfo.Host = server.config.ExternalBaseURL
		}
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// API v1
	v1 := router.Group("/v1")
	// ✅ 统一 JSON 响应格式：{code,message,data}
	// 注意：webhooks 与 websocket upgrade 在中间件内部会自动跳过
	v1.Use(ResponseEnvelopeMiddleware())

	// 元数据：角色访问矩阵（供前端/SDK消费）
	v1.GET("/role-access", server.getRoleAccessMetadata)

	// 微信认证路由(无需认证，但需要额外的速率限制)
	authPublicGroup := v1.Group("/auth")
	if rateLimiter != nil {
		authPublicGroup.Use(rateLimiter.SensitiveAPIMiddleware(10)) // 敏感 API 更严格限制：每分钟 10 次
	}
	authPublicGroup.POST("/wechat-login", server.wechatLogin)
	authPublicGroup.POST("/refresh", server.renewAccessToken)
	authPublicGroup.POST("/web-login/sessions", server.createWebLoginSession)
	authPublicGroup.GET("/web-login/sessions/:code", server.getWebLoginSessionStatus)
	authPublicGroup.POST("/web-login/consume", server.consumeWebLoginSession)

	// 微信支付回调路由（无需认证，微信服务器调用）
	webhooksGroup := v1.Group("/webhooks")
	{
		webhooksGroup.GET("/wechat-miniprogram/media-check", server.verifyMiniProgramMediaCheckWebhook)
		webhooksGroup.POST("/wechat-miniprogram/media-check", server.handleMiniProgramMediaCheckNotify)
		// 小程序直连支付回调
		webhooksGroup.POST("/wechat-pay/notify", server.handlePaymentNotify)
		webhooksGroup.POST("/wechat-pay/refund-notify", server.handleRefundNotify)
		// 平台收付通回调
		webhooksGroup.POST("/wechat-ecommerce/notify", server.handleCombinePaymentNotify)
		webhooksGroup.POST("/wechat-ecommerce/refund-notify", server.handleEcommerceRefundNotify)
		webhooksGroup.POST("/wechat-ecommerce/applyment-notify", server.handleApplymentStateNotify)
		webhooksGroup.POST("/wechat-ecommerce/profit-sharing-notify", server.handleProfitSharingNotify)
		// 微信用户投诉通知（合规要求，状态变更实时推送）
		webhooksGroup.POST("/wechat-ecommerce/complaint-notify", server.handleComplaintNotify)
		// 小程序「发货信息管理」结算事件（trade_manage_order_settlement）
		webhooksGroup.POST("/wechat-miniprogram/settlement-notify", server.handleOrderSettlementNotify)
	}

	// 需要认证的路由
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware(server.tokenMaker))
	authClientLogGroup := authGroup.Group("/logs")
	if rateLimiter != nil {
		authClientLogGroup.Use(rateLimiter.SensitiveAPIMiddleware(20)) // 客户端错误上报限流：每分钟 20 次/客户端
	}
	authClientLogGroup.POST("/error", server.reportClientErrorLog)

	// M2: 地区查询路由
	// 说明：前端已改为使用自建 OSM 获取行政区划/POI 数据。
	// 这里的 /v1/regions* 接口作为后备能力保留（降级/灾备/未来切回），暂时可能不会被调用。
	authGroup.GET("/regions/available", server.listAvailableRegions)
	authGroup.GET("/regions/:id/check", server.checkRegionAvailability)
	authGroup.GET("/regions/:id", server.getRegion)
	authGroup.GET("/regions", server.listRegions)
	authGroup.GET("/regions/:id/children", server.listRegionChildren)
	authGroup.GET("/regions/search", server.searchRegions)

	// 搜索路由
	searchGroup := authGroup.Group("/search")
	if rateLimiter != nil {
		searchGroup.Use(rateLimiter.SensitiveAPIMiddleware(60)) // 搜索接口限流：每分钟 60 次/客户端
	}
	{
		searchGroup.GET("/dishes", server.searchDishes)
		searchGroup.GET("/merchants", server.searchMerchants)
		searchGroup.GET("/combos", server.searchCombos)                // 套餐搜索
		searchGroup.GET("/rooms", server.searchRooms)                  // 包间搜索
		searchGroup.GET("/categories", server.searchCategories)        // 区域活跃菜系品类（首页网格）
		searchGroup.GET("/history", server.listSearchHistory)          // 搜索历史
		searchGroup.DELETE("/history", server.clearSearchHistory)      // 清除全部历史
		searchGroup.DELETE("/history/:id", server.deleteSearchHistory) // 删除单条
		searchGroup.GET("/popular", server.getPopularKeywords)         // 热门关键词
		searchGroup.GET("/suggestions", server.getSearchSuggestions)   // 实时建议
	}

	// 协议中心
	agreementsGroup := authGroup.Group("/agreements")
	{
		agreementsGroup.GET("", server.listAgreements)
		agreementsGroup.GET("/:type", server.getAgreement)
	}

	// 餐厅优惠活动
	authGroup.GET("/merchants/:id/promotions", server.getMerchantPromotions)

	// 扫码点餐路由
	scanGroup := authGroup.Group("/scan")
	if rateLimiter != nil {
		scanGroup.Use(rateLimiter.SensitiveAPIMiddleware(60)) // 扫码接口限流：每分钟 60 次/客户端
	}
	{
		scanGroup.GET("/table", server.scanTable)
	}

	// 消费者菜品详情（需认证，但不需要商户权限）
	authGroup.GET("/public/dishes/:id", server.getPublicDishDetail)
	authGroup.GET("/public/combos/:id", server.getPublicComboDetail)
	// 消费者商户详情（需认证，但不需要商户权限）
	authGroup.GET("/public/merchants/:id", server.getPublicMerchantDetail)
	authGroup.GET("/public/merchants/:id/dishes", server.getPublicMerchantDishes)
	authGroup.GET("/public/merchants/:id/combos", server.getPublicMerchantCombos)
	authGroup.GET("/public/merchants/:id/rooms", server.getPublicMerchantRooms)
	authGroup.GET("/public/merchants/:id/recharge-rules", server.getPublicRechargeRules)
	authGroup.GET("/public/merchants/:id/has-ordered", server.getPublicMerchantHasOrdered)

	// 分享功能由小程序前端 share 属性实现，无需后端API

	// M5.1: 运营商入驻申请路由（草稿模式+人工审核）
	authGroup.POST("/operator/application", server.getOrCreateOperatorApplicationDraft)     // 创建或获取申请草稿
	authGroup.GET("/operator/application", server.getOperatorApplication)                   // 获取申请状态
	authGroup.PUT("/operator/application/region", server.updateOperatorApplicationRegion)   // 更新申请区域
	authGroup.PUT("/operator/application/basic", server.updateOperatorApplicationBasicInfo) // 更新基础信息
	authGroup.DELETE("/operator/application/documents/:document_type", server.deleteOperatorApplicationDocument)
	authGroup.POST("/operator/application/submit", server.submitOperatorApplication)      // 提交申请
	authGroup.POST("/operator/application/reset", server.resetOperatorApplicationToDraft) // 重置为草稿

	// M5.2: 运营商开户（微信支付二级商户进件）
	operatorApplymentGroup := authGroup.Group("/operator/applyment")
	{
		operatorApplymentGroup.POST("/bindbank", server.operatorBindBank)        // 绑定银行卡开户
		operatorApplymentGroup.GET("/status", server.getOperatorApplymentStatus) // 获取开户状态
	}

	// M1: 用户相关路由
	authGroup.GET("/users/me", server.getCurrentUser)
	authGroup.PATCH("/users/me", server.updateCurrentUser)

	authGroup.POST("/auth/web-login/confirm", server.confirmWebLoginSession)

	// 媒体中心路由
	mediaGroup := authGroup.Group("/media")
	{
		mediaGroup.POST("/upload-sessions", server.createMediaUploadSession)
		mediaGroup.POST("/complete", server.completeMediaUpload)
		mediaGroup.POST("/private-access", server.getMediaPrivateAccess)
		mediaGroup.GET("/:id", server.getMediaAsset)
		mediaGroup.DELETE("/:id", server.deleteMediaAsset)
	}

	ocrGroup := authGroup.Group("/ocr")
	{
		ocrGroup.POST("/jobs", server.createOCRJob)
		ocrGroup.GET("/jobs/dead-letter", server.listOCRDeadLetterJobs)
		ocrGroup.GET("/jobs/:id", server.getOCRJob)
		ocrGroup.GET("/jobs/:id/result", server.getOCRJobResult)
		ocrGroup.POST("/jobs/:id/retry", server.retryOCRJob)
		ocrGroup.POST("/jobs/batch-query", server.batchQueryOCRJobs)
	}

	// M2: 用户地址路由
	authGroup.POST("/addresses", server.createUserAddress)
	authGroup.GET("/addresses", server.listUserAddresses)
	authGroup.GET("/addresses/:id", server.getUserAddress)
	authGroup.PATCH("/addresses/:id", server.updateUserAddress)
	authGroup.PATCH("/addresses/:id/default", server.setDefaultAddress)
	authGroup.DELETE("/addresses/:id", server.deleteUserAddress)

	// M2: 位置服务（需要认证，避免滥用地图 Key）
	authGroup.GET("/location/current-region", server.getCurrentRegionByLocation)
	authGroup.GET("/location/reverse-geocode", server.reverseGeocode)
	authGroup.GET("/location/direction/bicycling", server.getBicyclingRoute)

	// M3: 商户管理路由
	// 以下上传路由已废弃，统一返回 410 Gone（不经过业务中间件以避免不必要的 DB 查询）
	authGroup.POST("/merchants/images/upload", server.uploadMerchantImage)
	authGroup.POST("/dishes/images/upload", server.uploadDishImage)
	authGroup.POST("/tables/images/upload", server.uploadTableImage)
	authGroup.POST("/reviews/images/upload", server.uploadReviewImage)
	authGroup.GET("/merchants/me", server.getCurrentMerchant)
	authGroup.GET("/merchants/my", server.listMyMerchants) // 获取用户所有商户（多店铺切换）
	authGroup.PATCH("/merchants/me", server.updateCurrentMerchant)
	authGroup.PATCH("/merchants/me/shop-images", server.updateCurrentMerchantShopImages)
	authGroup.GET("/merchants/me/status", server.getMerchantOpenStatus)
	authGroup.PATCH("/merchants/me/status", server.updateMerchantOpenStatus)
	authGroup.GET("/merchants/me/business-hours", server.getMerchantBusinessHours)
	authGroup.PUT("/merchants/me/business-hours", server.setMerchantBusinessHours)
	authGroup.GET("/merchants/me/tags", server.getMerchantTags) // 获取商户经营类目
	authGroup.PUT("/merchants/me/tags", server.setMerchantTags) // 设置商户经营类目
	authGroup.GET("/merchants/me/membership-settings", server.getMerchantMembershipSettings)
	authGroup.PUT("/merchants/me/membership-settings", server.updateMerchantMembershipSettings)

	// M3.1: 商户入驻申请（新版 - 自动审核）
	merchantAppGroup := authGroup.Group("/merchant/application")
	{
		merchantAppGroup.GET("", server.getOrCreateMerchantApplicationDraft)      // 创建/获取草稿
		merchantAppGroup.PUT("/basic", server.updateMerchantApplicationBasicInfo) // 更新基础信息
		merchantAppGroup.PUT("/images", server.updateMerchantApplicationImages)   // 更新门头照/环境照
		merchantAppGroup.DELETE("/documents/:document_type", server.deleteMerchantApplicationDocument)
		merchantAppGroup.POST("/submit", server.submitMerchantApplication) // 提交申请（自动审核）
		merchantAppGroup.POST("/reset", server.resetMerchantApplication)   // 重置申请（被拒后）
	}

	// M3.2: 商户开户（微信支付二级商户进件）
	merchantApplymentGroup := authGroup.Group("/merchant/applyment")
	{
		merchantApplymentGroup.POST("/bindbank", server.merchantBindBank)        // 绑定银行卡开户
		merchantApplymentGroup.GET("/status", server.getMerchantApplymentStatus) // 获取开户状态
	}

	// 商户端：用户投诉管理（合规要求，商户需在指定时效内回复）
	merchantComplaintsGroup := authGroup.Group("/merchant/complaints")
	merchantComplaintsGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		merchantComplaintsGroup.GET("", server.listMerchantComplaints)
		merchantComplaintsGroup.GET("/:id", server.getMerchantComplaintDetail)
		merchantComplaintsGroup.POST("/:id/response", server.respondToComplaint)
		merchantComplaintsGroup.POST("/:id/complete", server.completeComplaint)
	}

	// M3.3: 员工绑定商户（任意登录用户）
	authGroup.POST("/bind-merchant", server.bindMerchant)

	// M3.4: 员工管理路由（需要商户权限）
	merchantStaffGroup := authGroup.Group("/merchant/staff")
	merchantStaffGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		merchantStaffGroup.GET("", server.listMerchantStaff)
		merchantStaffGroup.POST("/invite-code", server.generateInviteCode)
	}

	// M3.5: 仅老板可操作的员工管理
	merchantStaffOwnerGroup := authGroup.Group("/merchant/staff")
	merchantStaffOwnerGroup.Use(server.MerchantStaffMiddleware("owner"))
	{
		merchantStaffOwnerGroup.POST("", server.addMerchantStaff)
		merchantStaffOwnerGroup.PATCH("/:id/role", server.updateMerchantStaffRole)
		merchantStaffOwnerGroup.DELETE("/:id", server.deleteMerchantStaff)
	}

	// M3.6: 集团入驻申请
	groupAppGroup := authGroup.Group("/groups/applications")
	{
		groupAppGroup.POST("", server.createGroupApplicationDraft)
		groupAppGroup.GET("/me", server.getOrCreateGroupApplicationDraft)
		groupAppGroup.PUT("/basic", server.updateGroupApplicationBasic)
		groupAppGroup.DELETE("/documents/:document_type", server.deleteGroupApplicationDocument)
		groupAppGroup.POST("/submit", server.submitGroupApplication)
		groupAppGroup.POST("/:id/review", server.CasbinRoleMiddleware(RoleAdmin), server.reviewGroupApplication)
	}

	// M3.7: 集团/品牌管理
	groupsGroup := authGroup.Group("/groups")
	{
		groupsGroup.GET("", server.searchGroups)
		groupsGroup.POST("", server.CasbinRoleMiddleware(RoleAdmin), server.createGroup)
		groupsGroup.GET("/:id", server.getGroup)
		groupsGroup.PATCH("/:id", server.updateGroup)
		groupsGroup.GET("/:id/merchants", server.listGroupMerchants)
		groupsGroup.GET("/:id/brands", server.listGroupBrands)
		groupsGroup.POST("/:id/brands", server.createGroupBrand)
		groupsGroup.POST("/:id/join-requests", server.MerchantStaffMiddleware("owner"), server.createGroupJoinRequest)
		groupsGroup.GET("/:id/join-requests", server.listGroupJoinRequests)
		groupsGroup.POST("/:id/join-requests/:request_id/approve", server.approveGroupJoinRequest)
		groupsGroup.POST("/:id/join-requests/:request_id/reject", server.rejectGroupJoinRequest)
		groupsGroup.POST("/:id/join-requests/:request_id/cancel", server.cancelGroupJoinRequest)
		groupsGroup.GET("/:id/policies", server.getGroupPolicies)
		groupsGroup.PUT("/:id/policies", server.upsertGroupPolicies)
		groupsGroup.POST("/:id/menu-templates", server.createGroupMenuTemplate)
	}

	brandsGroup := authGroup.Group("/brands")
	{
		brandsGroup.GET("/:id", server.getBrand)
		brandsGroup.POST("/:id/menu-templates", server.createBrandMenuTemplate)
	}

	// M4: 标签管理路由
	tagsGroup := authGroup.Group("/tags")
	{
		tagsGroup.GET("", server.listTags)                                           // 获取标签列表（按类型）
		tagsGroup.POST("", server.CasbinRoleMiddleware(RoleAdmin), server.createTag) // 创建标签
		tagsGroup.DELETE("/:id", server.CasbinRoleMiddleware(RoleAdmin), server.deleteTag)
	}

	// M4: 菜品管理路由
	dishesGroup := authGroup.Group("/dishes")
	dishesGroup.Use(server.MerchantStaffMiddleware("owner", "manager", "chef"))

	{
		// 菜品分类
		dishesGroup.POST("/categories", server.createDishCategory)
		dishesGroup.GET("/categories", server.listDishCategories)
		dishesGroup.GET("/categories/global", server.listGlobalDishCategories)
		dishesGroup.PATCH("/categories/:id", server.updateDishCategory)
		dishesGroup.DELETE("/categories/:id", server.deleteDishCategory)

		// 菜品管理
		dishesGroup.POST("", server.createDish)
		dishesGroup.GET("", server.listDishesByMerchant)
		dishesGroup.GET("/:id", server.getDish)
		dishesGroup.PUT("/:id", server.updateDish)
		dishesGroup.DELETE("/:id", server.deleteDish)
		dishesGroup.PATCH("/:id/status", server.updateDishStatus)            // 单个菜品上下架
		dishesGroup.PATCH("/batch/status", server.batchUpdateDishStatus)     // 批量上下架
		dishesGroup.GET("/:id/customizations", server.getDishCustomizations) // 获取定制选项
		dishesGroup.PUT("/:id/customizations", server.setDishCustomizations) // 设置定制选项
		dishesGroup.PUT("/:id/specs", server.setDishCustomizations)          // 设置菜品规格（customizations别名）
		dishesGroup.PUT("/:id/featured-tags", server.setDishFeaturedTags)    // 设置推荐/热卖标签
	}

	// M4: 套餐管理路由
	combosGroup := authGroup.Group("/combos")
	combosGroup.Use(server.MerchantStaffMiddleware("owner", "manager", "chef"))

	{
		// 套餐管理
		combosGroup.POST("", server.createComboSet)
		combosGroup.GET("", server.listComboSets)
		combosGroup.GET("/:id", server.getComboSet)
		combosGroup.PUT("/:id", server.updateComboSet)
		combosGroup.PUT("/:id/online", server.toggleComboOnline)
		combosGroup.DELETE("/:id", server.deleteComboSet)

		// 套餐-菜品关联
		combosGroup.POST("/:id/dishes", server.addComboDish)
		combosGroup.DELETE("/:id/dishes/:dish_id", server.removeComboDish)
	}

	// M4: 库存管理路由
	inventoryGroup := authGroup.Group("/inventory")
	inventoryGroup.Use(server.MerchantStaffMiddleware("owner", "manager", "chef"))

	{
		inventoryGroup.POST("", server.createDailyInventory)
		inventoryGroup.GET("", server.listDailyInventory)
		inventoryGroup.PUT("", server.updateDailyInventory)
		inventoryGroup.PATCH("/:dish_id", server.updateSingleInventory) // 更新单品库存
		inventoryGroup.POST("/check", server.checkAndDecrementInventory)
		inventoryGroup.GET("/stats", server.getInventoryStats)
	}

	// M6: 配送费管理路由（运营商管理）
	// 运营商相关路由使用 RBAC 中间件
	deliveryFeeGroup := authGroup.Group("/delivery-fee")
	{
		// 配送费配置（按区域）- 运营商权限，验证 operator 管理该区域
		deliveryFeeOperatorGroup := deliveryFeeGroup.Group("")
		deliveryFeeOperatorGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware(), server.ValidateOperatorRegionMiddleware("region_id"))
		{
			deliveryFeeOperatorGroup.POST("/regions/:region_id/config", server.createDeliveryFeeConfig)
			deliveryFeeOperatorGroup.PATCH("/regions/:region_id/config", server.updateDeliveryFeeConfig)
		}

		// 配送费查询（公开访问）
		deliveryFeeGroup.GET("/regions/:region_id/config", server.getDeliveryFeeConfig)

		// 商家配送优惠（商户权限 - 使用 MerchantStaffMiddleware 支持员工角色）
		deliveryFeeMerchantGroup := deliveryFeeGroup.Group("/merchants/:merchant_id")
		deliveryFeeMerchantGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
		{
			deliveryFeeMerchantGroup.POST("/promotions", server.createDeliveryPromotion)
			deliveryFeeMerchantGroup.GET("/promotions", server.listDeliveryPromotions)
			deliveryFeeMerchantGroup.PATCH("/promotions/:id", server.updateDeliveryPromotion)
			deliveryFeeMerchantGroup.DELETE("/promotions/:id", server.deleteDeliveryPromotion)
		}

		// 运费计算（核心接口 - 无需特殊权限）
		deliveryFeeGroup.POST("/calculate", server.calculateDeliveryFee)
	}

	// M5: 桌台与包间管理路由
	tablesGroup := authGroup.Group("/tables")
	{
		tablesGroup.POST("", server.createTable)
		tablesGroup.GET("/:id", server.getTable)
		tablesGroup.GET("", server.listTables)
		tablesGroup.PATCH("/:id", server.updateTable)
		tablesGroup.PATCH("/:id/status", server.updateTableStatus)
		tablesGroup.DELETE("/:id", server.deleteTable)

		// 桌台标签
		tablesGroup.POST("/:id/tags", server.addTableTag)
		tablesGroup.DELETE("/:id/tags/:tag_id", server.removeTableTag)
		tablesGroup.GET("/:id/tags", server.listTableTags)

		// 桌台图片
		tablesGroup.POST("/:id/images", server.addTableImage)
		tablesGroup.GET("/:id/images", server.listTableImages)
		tablesGroup.PUT("/:id/images/:image_id/primary", server.setTablePrimaryImage)
		tablesGroup.DELETE("/:id/images/:image_id", server.deleteTableImage)

		// 桌台二维码
		tablesGroup.GET("/:id/qrcode", server.generateTableQRCode)
	}

	// M5: 商户包间查询（C端用户）
	authGroup.GET("/merchants/:id/rooms", server.listAvailableRooms)
	authGroup.GET("/merchants/:id/rooms/all", server.listMerchantRoomsForCustomer)

	// M5: 包间详情和可用性（C端用户）
	roomsGroup := authGroup.Group("/rooms")
	{
		roomsGroup.GET("/:id", server.getRoomDetail)
		roomsGroup.GET("/:id/availability", server.getRoomAvailability)
	}

	// M5: 包间预定路由
	reservationsGroup := authGroup.Group("/reservations")
	{
		// 用户预定
		reservationsGroup.POST("", server.createReservation)
		reservationsGroup.GET("/me", server.listUserReservations)
		reservationsGroup.GET("/:id", server.getReservation)
		// 注：支付由支付网关回调触发，预定支付通过通用支付订单接口处理
		reservationsGroup.POST("/:id/cancel", server.cancelReservation)
		reservationsGroup.POST("/:id/add-dishes", server.addDishesToReservation)     // 追加菜品
		reservationsGroup.POST("/:id/modify-dishes", server.modifyReservationDishes) // 改菜（差量）
		reservationsGroup.POST("/:id/checkin", server.checkInReservation)            // 到店签到
		reservationsGroup.POST("/:id/start-cooking", server.startCookingReservation) // 起菜通知

		// 商户管理
		reservationsGroup.GET("/merchant", server.listMerchantReservations)
		reservationsGroup.GET("/merchant/dishes", server.listMerchantReservationDishes)
		reservationsGroup.GET("/merchant/today", server.listTodayReservations) // 今日预订
		reservationsGroup.GET("/merchant/stats", server.getReservationStats)
		reservationsGroup.POST("/merchant/create", server.merchantCreateReservation) // 商户代客创建
		reservationsGroup.PUT("/:id/update", server.merchantUpdateReservation)       // 商户修改预订
		reservationsGroup.POST("/:id/confirm", server.confirmReservation)
		reservationsGroup.POST("/:id/complete", server.completeReservation)
		reservationsGroup.POST("/:id/no-show", server.markNoShow)
	}

	// 用餐会话
	diningSessionsGroup := authGroup.Group("/dining-sessions")
	{
		diningSessionsGroup.GET("/precheck", server.precheckDiningSession)
		diningSessionsGroup.POST("/open", server.openDiningSession)
		diningSessionsGroup.POST("/:id/transfer-table", server.transferDiningSessionTable)
		diningSessionsGroup.POST("/:id/checkout", server.checkoutDiningSession)
	}

	// 账单组
	billingGroupsGroup := authGroup.Group("/billing-groups")
	{
		billingGroupsGroup.POST("", server.createBillingGroup)
		billingGroupsGroup.GET("", server.listBillingGroups)
		billingGroupsGroup.POST("/:id/join", server.joinBillingGroup)
		billingGroupsGroup.GET("/:id/orders", server.listBillingGroupOrders)
	}

	// M7: 订单管理路由
	ordersGroup := authGroup.Group("/orders")
	{
		// 用户端
		ordersGroup.GET("/calculate", server.calculateOrder) // 计算订单金额
		ordersGroup.POST("", server.createOrder)
		ordersGroup.GET("", server.listOrders)
		ordersGroup.GET("/:id", server.getOrder)
		ordersGroup.POST("/:id/cancel", server.cancelOrder)
		ordersGroup.POST("/:id/replace", server.replaceOrder)
		ordersGroup.POST("/:id/urge", server.urgeOrder)
		ordersGroup.POST("/:id/confirm", server.confirmOrder)
	}

	// M7: 商户端订单管理路由
	merchantOrdersGroup := authGroup.Group("/merchant/orders")
	merchantOrdersGroup.Use(server.MerchantStaffMiddleware("owner", "manager", "cashier"))

	{
		merchantOrdersGroup.GET("", server.listMerchantOrders)
		merchantOrdersGroup.GET("/:id", server.getMerchantOrder)
		merchantOrdersGroup.POST("/:id/accept", server.acceptOrder)
		merchantOrdersGroup.POST("/:id/reject", server.rejectOrder) // 拒单
		merchantOrdersGroup.POST("/:id/ready", server.markOrderReady)
		merchantOrdersGroup.POST("/:id/complete", server.completeOrder)
		merchantOrdersGroup.GET("/stats", server.getOrderStats)
	}

	// M7-KDS: 厨房显示系统路由
	kitchenGroup := authGroup.Group("/kitchen")
	kitchenGroup.Use(server.MerchantStaffMiddleware("owner", "manager", "chef"))

	{
		kitchenGroup.GET("/orders", server.listKitchenOrders)
		kitchenGroup.GET("/orders/:id", server.getKitchenOrderDetails)
		kitchenGroup.POST("/orders/:id/preparing", server.startPreparing)
		kitchenGroup.POST("/orders/:id/ready", server.markKitchenOrderReady)
	}

	// 商户索赔与申诉路由
	merchantClaimsGroup := authGroup.Group("/merchant")
	{
		merchantClaimsGroup.GET("/claims", server.listMerchantClaims)
		merchantClaimsGroup.GET("/claims/:id", server.getMerchantClaimDetail)
		merchantClaimsGroup.GET("/claims/:id/decision", server.getMerchantClaimDecision)
		merchantClaimsGroup.GET("/claims/behavior-summary", server.getMerchantClaimBehaviorSummary)
		merchantClaimsGroup.GET("/claims/:id/recovery", server.getMerchantClaimRecovery)
		merchantClaimsGroup.POST("/claims/:id/recovery/pay", server.payMerchantClaimRecovery)
		merchantClaimsGroup.POST("/appeals", server.createMerchantAppeal)
		merchantClaimsGroup.GET("/appeals", server.listMerchantAppeals)
		merchantClaimsGroup.GET("/appeals/:id", server.getMerchantAppealDetail)
	}

	merchantRiskGroup := authGroup.Group("/merchant/risk")
	{
		merchantRiskGroup.GET("/users/:id", server.getMerchantUserRisk)
	}

	// M7.5: 支付订单路由
	paymentGroup := authGroup.Group("/payments")
	{
		paymentGroup.POST("", server.createPaymentOrder)
		paymentGroup.POST("/combined", server.createCombinedPaymentOrder)
		paymentGroup.GET("/combined/:id", server.getCombinedPaymentOrder)
		paymentGroup.POST("/combined/:id/close", server.closeCombinedPaymentOrder)
		paymentGroup.GET("/ledger", server.listPaymentLedger)
		paymentGroup.GET("", server.listPaymentOrders)
		paymentGroup.GET("/:id", server.getPaymentOrder)
		paymentGroup.POST("/:id/close", server.closePaymentOrder)
		paymentGroup.GET("/:id/refunds", server.listRefundOrdersByPayment)
	}

	// M7.5: 退款订单路由（商户端）
	refundGroup := authGroup.Group("/refunds")
	{
		refundGroup.POST("", server.createRefundOrder)
		refundGroup.GET("/:id", server.getRefundOrder)
		refundGroup.GET("/:id/returns", server.listProfitSharingReturnsByRefund)
	}

	// M8: 骑手管理路由
	riderGroup := authGroup.Group("/rider")
	{
		// 骑手申请流程（新版）
		riderGroup.GET("/application", server.createOrGetRiderApplicationDraft)  // 创建/获取草稿
		riderGroup.PUT("/application/basic", server.updateRiderApplicationBasic) // 更新基础信息
		riderGroup.DELETE("/application/documents/:document_type", server.deleteRiderApplicationDocument)
		riderGroup.DELETE("/application/health-cert", server.deleteRiderApplicationHealthCert)
		riderGroup.POST("/application/submit", server.submitRiderApplication) // 提交申请
		riderGroup.POST("/application/reset", server.resetRiderApplication)   // 重置待处理申请
		riderGroup.GET("/me", server.getRiderMe)

		// 押金管理
		riderGroup.GET("/deposit", server.getRiderDepositBalance)
		riderGroup.POST("/deposit", server.depositRider)
		riderGroup.POST("/withdraw", server.withdrawRider)
		riderGroup.GET("/deposits", server.listRiderDeposits)

		// 上下线与状态
		riderGroup.GET("/status", server.getRiderStatus)
		riderGroup.POST("/online", server.goOnline)
		riderGroup.POST("/offline", server.goOffline)

		// 位置上报
		riderGroup.POST("/location", server.updateRiderLocation)

		// 骑手订单操作
		riderGroup.POST("/orders/:id/delay", server.reportDelay)         // 延时申报
		riderGroup.POST("/orders/:id/exception", server.reportException) // 异常上报

		// 高值单资格积分
		riderGroup.GET("/score", server.getRiderPremiumScore)                 // 获取高值单资格积分
		riderGroup.GET("/score/history", server.listRiderPremiumScoreHistory) // 获取积分变更历史

		// 骑手索赔与申诉
		riderGroup.GET("/claims", server.listRiderClaims)
		riderGroup.GET("/claims/:id", server.getRiderClaimDetail)
		riderGroup.GET("/claims/:id/decision", server.getRiderClaimDecision)
		riderGroup.GET("/claims/behavior-summary", server.getRiderClaimBehaviorSummary)
		riderGroup.GET("/claims/:id/recovery", server.getRiderClaimRecovery)
		riderGroup.POST("/claims/:id/recovery/pay", server.payRiderClaimRecovery)
		riderGroup.POST("/appeals", server.createRiderAppeal)
		riderGroup.GET("/appeals", server.listRiderAppeals)
		riderGroup.GET("/appeals/:id", server.getRiderAppealDetail)
	}

	// M8: 配送管理路由
	deliveryGroup := authGroup.Group("/delivery")
	{
		// 推荐订单（骑手获取附近可接订单）
		deliveryGroup.GET("/recommend", server.getRecommendedOrders)

		// 抢单
		deliveryGroup.POST("/grab/:order_id", server.grabOrder)

		// 骑手当前配送列表
		deliveryGroup.GET("/active", server.listMyActiveDeliveries)
		deliveryGroup.GET("/history", server.listMyDeliveries)

		// 配送状态更新
		deliveryGroup.POST("/:delivery_id/start-pickup", server.startPickup)
		deliveryGroup.POST("/:delivery_id/confirm-pickup", server.confirmPickup)
		deliveryGroup.POST("/:delivery_id/start-delivery", server.startDelivery)
		deliveryGroup.POST("/:delivery_id/confirm-delivery", server.confirmDelivery)

		// 配送详情
		deliveryGroup.GET("/order/:order_id", server.getDeliveryByOrder)
		deliveryGroup.GET("/:delivery_id/track", server.getDeliveryTrack)
		deliveryGroup.GET("/:delivery_id/rider-location", server.getRiderLatestLocation)
	}

	// M8: 运营商骑手审核路由（需要运营商或管理员角色）
	adminRiderGroup := authGroup.Group("/admin/riders")
	adminRiderGroup.Use(server.CasbinMiddleware())
	{
		adminRiderGroup.GET("", server.listRiders)
	}

	// 平台管理员审核运营商申请
	adminOperatorGroup := authGroup.Group("/admin/operators/applications")
	adminOperatorGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		adminOperatorGroup.GET("", server.listPendingOperatorApplicationsAdmin)
		adminOperatorGroup.GET("/:id", server.getOperatorApplicationDetailAdmin)
		adminOperatorGroup.POST("/:id/approve", server.approveOperatorApplicationAdmin)
		adminOperatorGroup.POST("/:id/reject", server.rejectOperatorApplicationAdmin)
	}

	// 平台管理员查询运营商（实体）的管理区域列表
	adminOperatorEntityGroup := authGroup.Group("/admin/operators")
	adminOperatorEntityGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		adminOperatorEntityGroup.GET("/:operator_id/regions", server.getOperatorRegionsAdmin)
	}

	// 平台管理员审核运营商区域扩展申请
	adminRegionExpansionGroup := authGroup.Group("/admin/operators/region-applications")
	adminRegionExpansionGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		adminRegionExpansionGroup.GET("", server.listPendingRegionApplicationsAdmin)
		adminRegionExpansionGroup.POST("/:id/approve", server.approveRegionApplicationAdmin)
		adminRegionExpansionGroup.POST("/:id/reject", server.rejectRegionApplicationAdmin)
	}

	// 平台管理员审核集团入驻申请
	adminGroupApplicationGroup := authGroup.Group("/admin/groups/applications")
	adminGroupApplicationGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		adminGroupApplicationGroup.GET("", server.listGroupApplicationsAdmin)
		adminGroupApplicationGroup.GET("/:id", server.getGroupApplicationAdmin)
		adminGroupApplicationGroup.POST("/:id/review", server.reviewGroupApplication)
	}

	// M14: 通知系统路由
	notificationsGroup := authGroup.Group("/notifications")
	{
		notificationsGroup.GET("", server.listNotifications)
		notificationsGroup.GET("/unread/count", server.getUnreadCount)
		notificationsGroup.PUT("/:id/read", server.markNotificationAsRead)
		notificationsGroup.PUT("/read-all", server.markAllAsRead)
		notificationsGroup.DELETE("/:id", server.deleteNotification)
		notificationsGroup.GET("/preferences", server.getNotificationPreferences)
		notificationsGroup.PUT("/preferences", server.updateNotificationPreferences)
	}

	// M14: WebSocket路由（骑手和商户实时通知）
	authGroup.GET("/ws", server.handleWebSocket)

	// M14: 平台运营人员WebSocket路由（接收告警推送）
	authGroup.GET("/platform/ws", server.handlePlatformWebSocket)

	// M12: 商户统计BI路由
	merchantStatsGroup := authGroup.Group("/merchant/stats")
	merchantStatsGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		merchantStatsGroup.GET("/daily", server.getMerchantDailyStats)
		merchantStatsGroup.GET("/overview", server.getMerchantOverview)
		merchantStatsGroup.GET("/dishes/top", server.getTopSellingDishes)
		merchantStatsGroup.GET("/customers", server.listMerchantCustomers)
		merchantStatsGroup.GET("/customers/:user_id", server.getCustomerDetail)
		// 新增多维度分析
		merchantStatsGroup.GET("/hourly", server.getMerchantHourlyStats)
		merchantStatsGroup.GET("/sources", server.getMerchantOrderSourceStats)
		merchantStatsGroup.GET("/repurchase", server.getMerchantRepurchaseRate)
		merchantStatsGroup.GET("/categories", server.getMerchantDishCategoryStats)
	}

	// 商户财务路由
	merchantFinanceGroup := authGroup.Group("/merchant/finance")
	merchantFinanceGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		merchantFinanceGroup.GET("/overview", server.getMerchantFinanceOverview)
		merchantFinanceGroup.GET("/orders", server.listMerchantFinanceOrders)
		merchantFinanceGroup.GET("/service-fees", server.listMerchantServiceFees)
		merchantFinanceGroup.GET("/promotions", server.listMerchantPromotionExpenses)
		merchantFinanceGroup.GET("/daily", server.listMerchantDailyFinance)
		merchantFinanceGroup.GET("/settlements", server.listMerchantSettlements)
		merchantFinanceGroup.GET("/settlement-timeline", server.listMerchantSettlementTimeline)
		merchantFinanceGroup.GET("/account/balance", server.getMerchantAccountBalance)
		merchantFinanceGroup.POST("/account/withdraw", server.createMerchantAccountWithdraw)
		merchantFinanceGroup.GET("/account/withdrawals", server.listMerchantAccountWithdrawals)
		merchantFinanceGroup.GET("/account/withdrawals/:id", server.getMerchantAccountWithdrawal)
	}

	// 商户设备管理路由
	merchantDevicesGroup := authGroup.Group("/merchant/devices")
	{
		merchantDevicesGroup.POST("", server.createPrinter)
		merchantDevicesGroup.GET("", server.listPrinters)
		merchantDevicesGroup.GET("/:id", server.getPrinter)
		merchantDevicesGroup.PUT("/:id", server.updatePrinter)
		merchantDevicesGroup.DELETE("/:id", server.deletePrinter)
		merchantDevicesGroup.POST("/:id/test", server.testPrinter)
	}

	// 商户订单展示配置路由
	merchantDisplayGroup := authGroup.Group("/merchant/display-config")
	{
		merchantDisplayGroup.GET("", server.getDisplayConfig)
		merchantDisplayGroup.PUT("", server.updateDisplayConfig)
	}

	// M12: 运营商统计BI路由
	// 使用 Casbin 中间件验证 operator 角色并加载 operator 信息
	operatorStatsGroup := authGroup.Group("/operator")
	operatorStatsGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware())
	{

		// 区域扩展申请
		operatorStatsGroup.POST("/region-expansion", server.applyOperatorRegionExpansion)  // 申请运营更多区域
		operatorStatsGroup.GET("/region-expansion", server.listOperatorRegionApplications) // 查看自己的扩展申请

		// 区域相关路由（需要额外验证区域管理权限）
		operatorStatsGroup.GET("/regions", server.listOperatorRegions) // 获取管理的区域列表
		operatorStatsGroup.GET("/regions/:region_id/stats", server.getRegionStats)
		operatorStatsGroup.POST("/regions/:region_id/peak-hours", server.createPeakHourConfig)
		operatorStatsGroup.GET("/regions/:region_id/peak-hours", server.listPeakHourConfigs)

		// 实时数据 (New)
		operatorStatsGroup.GET("/stats/realtime", server.getOperatorRealtimeStats)

		// 多维度分析
		operatorStatsGroup.GET("/merchants/ranking", server.getOperatorMerchantRanking)
		operatorStatsGroup.GET("/riders/ranking", server.getOperatorRiderRanking)
		operatorStatsGroup.GET("/trend/daily", server.getRegionDailyTrend)

		// 高峰时段删除（handler 内部验证区域）
		operatorStatsGroup.DELETE("/peak-hours/:id", server.deletePeakHourConfig)

		// 商户管理（完整CRUD + 暂停/恢复）
		operatorStatsGroup.GET("/merchants", server.listOperatorMerchants)
		operatorStatsGroup.GET("/merchants/:id", server.getOperatorMerchant)
		operatorStatsGroup.GET("/merchants/:id/stats", server.getOperatorMerchantStats)
		operatorStatsGroup.POST("/merchants/:id/resume", server.ResumeMerchant)

		// 骑手管理（完整CRUD + 暂停/恢复）
		operatorStatsGroup.GET("/riders", server.listOperatorRiders)
		operatorStatsGroup.GET("/riders/:id", server.getOperatorRider)
		operatorStatsGroup.GET("/riders/:id/stats", server.getOperatorRiderStats)
		// 规则驱动：运营商不提供暂停/恢复入口

		// 申诉处理（运营商审核商户/骑手申诉）
		// operatorStatsGroup.GET("/appeals", server.listOperatorAppeals) // Already exists or covered by our new file
		// If collision, we will use our new one or check grep result.
		// Assuming we simply add our new specific ones or keep existing if same name.
		// Actually, let's wait for grep result in next turn to decide on 'listOperatorAppeals'.
		// But I need to output something here.
		// I will just add the safe ones for now: realtime and safety report.
		// And withdraw.

		// 食安熔断 (New)
		operatorStatsGroup.GET("/reports/safety", server.listSafetyReports)
		operatorStatsGroup.POST("/reports/safety", server.submitSafetyReport)
		operatorStatsGroup.GET("/reports/safety/:id", server.getSafetyReportDetail)
		operatorStatsGroup.POST("/reports/safety/:id/resolve", server.resolveSafetyReport)

		operatorStatsGroup.GET("/appeals", server.listOperatorAppeals)
		operatorStatsGroup.GET("/appeals/:id", server.getOperatorAppealDetail)
		operatorStatsGroup.POST("/appeals/:id/review", server.reviewAppeal)
		operatorStatsGroup.GET("/claims/:id/recovery", server.getOperatorClaimRecovery)
		operatorStatsGroup.POST("/claims/:id/recovery/waive", server.waiveClaimRecovery)

		// 规则管理
		operatorStatsGroup.GET("/rules", server.listOperatorRules)
		operatorStatsGroup.PATCH("/rules/:key", server.updateOperatorRule)
	}

	// 运营商财务路由 (使用 /operators/me 路径)
	operatorsGroup := authGroup.Group("/operators/me")
	operatorsGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware())
	{
		operatorsGroup.GET("/finance/overview", server.getOperatorFinanceOverview)
		operatorsGroup.GET("/commission", server.getOperatorCommission)
		operatorsGroup.GET("/finance/account/balance", server.getOperatorAccountBalance)
		operatorsGroup.POST("/finance/withdraw", server.withdrawOperator) // New
		operatorsGroup.GET("/finance/withdrawals", server.listOperatorWithdrawals)
		operatorsGroup.GET("/finance/withdrawals/:id", server.getOperatorWithdrawal)
		operatorsGroup.GET("/profit-sharing/configs", server.listOperatorProfitSharingConfigs)

		// 用户投诉管理（运营商视角：查看所有待处理投诉，可完结投诉）
		operatorsGroup.GET("/complaints", server.listPendingComplaints)
		operatorsGroup.POST("/complaints/:id/complete", server.completeComplaint)

		// 补差管理（运营商发起/退回/取消平台补差）
		operatorPaymentGroup := operatorsGroup.Group("/payment-orders/:id")
		{
			operatorPaymentGroup.POST("/subsidies", server.createSubsidy)
			operatorPaymentGroup.POST("/subsidies/return", server.returnSubsidy)
			operatorPaymentGroup.POST("/subsidies/cancel", server.cancelSubsidy)
		}

		operatorRulesProxyGroup := operatorsGroup.Group("/rules")
		{
			operatorRulesProxyGroup.GET("", server.listOperatorRulesProxy)
			operatorRulesProxyGroup.GET("/hits", server.listOperatorRuleHitsProxy)
			operatorRulesProxyGroup.GET("/:id", server.getOperatorRuleProxy)
			operatorRulesProxyGroup.POST("", server.createOperatorRuleProxy)
			operatorRulesProxyGroup.POST("/:id/versions", server.createOperatorRuleVersionProxy)
			operatorRulesProxyGroup.POST("/:id/publish", server.publishOperatorRuleProxy)
			operatorRulesProxyGroup.POST("/:id/rollback", server.rollbackOperatorRuleProxy)
			operatorRulesProxyGroup.POST("/:id/disable", server.disableOperatorRuleProxy)
		}
	}

	// M12: 平台统计BI路由
	// 使用 Casbin 中间件验证 admin 角色
	platformStatsGroup := authGroup.Group("/platform/stats")
	platformStatsGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		platformStatsGroup.GET("/overview", server.getPlatformOverview)
		platformStatsGroup.GET("/daily", server.getPlatformDailyStats)
		platformStatsGroup.GET("/profit-sharing/reconciliation", server.getPlatformProfitSharingReconciliation)
		platformStatsGroup.GET("/profit-sharing/sla", server.getPlatformProfitSharingSlaSummary)
		platformStatsGroup.GET("/profit-sharing/config-audits", server.getPlatformProfitSharingConfigAudits)
		platformStatsGroup.GET("/regions/compare", server.getRegionComparison)
		platformStatsGroup.GET("/merchants/ranking", server.getMerchantRanking)
		platformStatsGroup.GET("/categories", server.getCategoryStats)
		platformStatsGroup.GET("/growth/users", server.getUserGrowthStats)
		platformStatsGroup.GET("/growth/merchants", server.getMerchantGrowthStats)
		platformStatsGroup.GET("/riders/ranking", server.getRiderRanking)
		platformStatsGroup.GET("/hourly", server.getHourlyDistribution)
		platformStatsGroup.GET("/realtime", server.getRealtimeDashboard)
		platformStatsGroup.GET("/bill-reconciliation", server.getBillReconciliationReports)
	}

	// 平台分账规则配置（管理）
	platformProfitSharingGroup := authGroup.Group("/platform/profit-sharing")
	platformProfitSharingGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		platformProfitSharingGroup.POST("/configs", server.createProfitSharingConfig)
		platformProfitSharingGroup.GET("/configs", server.listProfitSharingConfigs)
		platformProfitSharingGroup.PATCH("/configs/:id", server.updateProfitSharingConfig)
		platformProfitSharingGroup.POST("/configs/:id/disable", server.disableProfitSharingConfig)
	}

	platformRulesGroup := authGroup.Group("/platform/rules")
	platformRulesGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		platformRulesGroup.POST("", server.createRule)
		platformRulesGroup.GET("", server.listRules)
		platformRulesGroup.GET("/:id", server.getRule)
		platformRulesGroup.POST("/:id/versions", server.createRuleVersion)
		platformRulesGroup.POST("/:id/publish", server.publishRule)
		platformRulesGroup.POST("/:id/disable", server.disableRule)
		platformRulesGroup.POST("/:id/rollback", server.rollbackRule)
		platformRulesGroup.GET("/hits", server.listRuleHits)
	}

	platformOperatorRulesGroup := authGroup.Group("/platform/operator-rules")
	platformOperatorRulesGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		platformOperatorRulesGroup.GET("", server.listPlatformOperatorRules)
		platformOperatorRulesGroup.PATCH("/:key", server.updatePlatformOperatorRule)
	}

	// 用户索赔路由
	claimsGroup := authGroup.Group("/claims")
	{
		claimsGroup.POST("", server.SubmitClaim)
		claimsGroup.GET("", server.ListUserClaims)
		claimsGroup.GET("/:id", server.GetClaimDetail)
		// ReviewClaim 入口停止使用：裁决全自动，仅保留审计旁路
	}

	// 食安上报路由
	foodSafetyGroup := authGroup.Group("/food-safety")
	{
		foodSafetyGroup.POST("/report", server.ReportFoodSafety)
		foodSafetyGroup.PATCH("/merchants/:id/suspend", server.SuspendMerchant)
	}

	// 欺诈检测路由
	fraudGroup := authGroup.Group("/fraud")
	{
		fraudGroup.POST("/detect", server.TriggerFraudDetection)
	}

	// 购物车路由
	cartGroup := authGroup.Group("/cart")
	{
		cartGroup.GET("", server.getCart)
		cartGroup.GET("/summary", server.getUserCartsSummary)
		cartGroup.GET("/user-carts", server.getUserCartsSummary)                     // 多商户购物车汇总
		cartGroup.POST("/combined-checkout/preview", server.previewCombinedCheckout) // 合单结算预览
		cartGroup.POST("/items", server.addCartItem)
		cartGroup.PATCH("/items/:id", server.updateCartItem)
		cartGroup.DELETE("/items/:id", server.deleteCartItem)
		cartGroup.POST("/clear", server.clearCart)
		cartGroup.POST("/calculate", server.calculateCart)
	}

	// 收藏路由
	favoritesGroup := authGroup.Group("/favorites")
	{
		// 商户收藏
		favoritesGroup.POST("/merchants", server.addFavoriteMerchant)
		favoritesGroup.GET("/merchants", server.listFavoriteMerchants)
		favoritesGroup.DELETE("/merchants/:id", server.deleteFavoriteMerchant)

		// 菜品收藏
		favoritesGroup.POST("/dishes", server.addFavoriteDish)
		favoritesGroup.GET("/dishes", server.listFavoriteDishes)
		favoritesGroup.DELETE("/dishes/:id", server.deleteFavoriteDish)
	}

	// 浏览历史路由
	historyGroup := authGroup.Group("/history")
	{
		historyGroup.GET("/browse", server.listBrowseHistory)
	}

	// M10: 会员营销系统路由
	// 会员管理
	membershipGroup := authGroup.Group("/memberships")
	{
		// 用户加入会员
		membershipGroup.POST("", server.joinMembership)

		// 用户充值
		membershipGroup.POST("/recharge", server.rechargeMembership)

		// 查询用户的所有会员卡
		membershipGroup.GET("", server.listUserMemberships)

		// 查询单个会员卡详情
		membershipGroup.GET("/:id", server.getMembership)

		// 查询会员消费记录
		membershipGroup.GET("/:id/transactions", server.listMembershipTransactions)
	}

	// M13: 评价系统路由
	reviewsGroup := authGroup.Group("/reviews")
	{
		// 创建评价
		reviewsGroup.POST("", server.createReview)

		// 查询评价详情
		reviewsGroup.GET("/:id", server.getReview)

		// 根据订单ID查询评价
		reviewsGroup.GET("/orders/:id", server.getReviewByOrder)

		// 查询用户的评价列表
		reviewsGroup.GET("/me", server.listUserReviews)

		// 查询商户的评价列表（顾客视角，仅可见评价）
		reviewsGroup.GET("/merchants/:id", server.listMerchantReviews)

		// 商户查看所有评价（包含不可见的）
		// 商户回复评价
		// 见 merchantReviewsGroup
	}

	// 商户评价管理（仅店主）
	merchantReviewsGroup := authGroup.Group("/reviews")
	merchantReviewsGroup.Use(server.MerchantStaffMiddleware("owner"))
	{
		merchantReviewsGroup.GET("/merchants/:id/all", server.listMerchantAllReviews)
		merchantReviewsGroup.POST("/:id/reply", server.replyReview)
	}

	// 删除评价（运营商权限）
	// 使用 Casbin 中间件验证 operator 角色
	reviewsOperatorGroup := authGroup.Group("/reviews")
	reviewsOperatorGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware())
	{
		reviewsOperatorGroup.DELETE("/:id", server.deleteReview)
	}

	// M11: 千人千面推荐引擎路由已下线

	// 充值规则管理（商户）
	rechargeRuleGroup := authGroup.Group("/merchants/:id/recharge-rules")
	rechargeRuleGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		// 创建充值规则
		rechargeRuleGroup.POST("", server.createRechargeRule)

		// 查询商户的充值规则列表（所有状态）
		rechargeRuleGroup.GET("", server.listRechargeRules)

		// 查询商户的生效中充值规则
		rechargeRuleGroup.GET("/active", server.listActiveRechargeRules)

		// 更新充值规则
		rechargeRuleGroup.PATCH("/:rule_id", server.updateRechargeRule)

		// 删除充值规则
		rechargeRuleGroup.DELETE("/:rule_id", server.deleteRechargeRule)
	}

	// 优惠券管理（商户创建和管理）
	voucherGroup := authGroup.Group("/merchants/:id/vouchers")
	voucherGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		// 创建优惠券
		voucherGroup.POST("", server.createVoucher)

		// 查询商户的优惠券列表（所有状态）
		voucherGroup.GET("", server.listMerchantVouchers)

		// 查询商户的生效中优惠券
		voucherGroup.GET("/active", server.listActiveVouchers)

		// 更新优惠券
		voucherGroup.PATCH("/:voucher_id", server.updateVoucher)

		// 删除优惠券
		voucherGroup.DELETE("/:voucher_id", server.deleteVoucher)
	}

	// 商户会员管理（查看会员列表、详情、调整余额）
	merchantMembersGroup := authGroup.Group("/merchants/:id/members")
	merchantMembersGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		// 查询商户的会员列表
		merchantMembersGroup.GET("", server.listMerchantMembers)

		// 获取会员详情（含交易记录）
		merchantMembersGroup.GET("/:user_id", server.getMerchantMemberDetail)

		// 调整会员余额（退款/扣减）
		merchantMembersGroup.POST("/:user_id/balance", server.adjustMemberBalance)
	}

	// 用户优惠券操作
	userVoucherGroup := authGroup.Group("/vouchers")
	{
		// 用户领取优惠券
		userVoucherGroup.POST("/:voucher_id/claim", server.claimVoucher)

		// 查询用户的所有优惠券
		userVoucherGroup.GET("/me", server.listUserVouchers)

		// 查询用户某商户的可用优惠券
		userVoucherGroup.GET("/available/:merchant_id", server.listUserAvailableVouchersForMerchant)

		// 查询用户的所有可用优惠券（不限商户）
		userVoucherGroup.GET("/available", server.listUserAvailableVouchers)
	}

	// 折扣规则管理（商户）
	discountGroup := authGroup.Group("/merchants/:id/discounts")
	discountGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
	{
		// 创建折扣规则
		discountGroup.POST("", server.createDiscountRule)

		// 查询商户的折扣规则列表（所有状态）
		discountGroup.GET("", server.listMerchantDiscountRules)

		// 查询商户的生效中折扣规则
		discountGroup.GET("/active", server.listActiveDiscountRules)

		// 查询单个折扣规则
		discountGroup.GET("/:id", server.getDiscountRule)

		// 更新折扣规则
		discountGroup.PATCH("/:id", server.updateDiscountRule)

		// 删除折扣规则
		discountGroup.DELETE("/:id", server.deleteDiscountRule)

		// 查询可用折扣规则（下单时使用）
		discountGroup.GET("/applicable", server.getApplicableDiscountRules)

		// 查询最优折扣（下单时自动应用）
		discountGroup.GET("/best", server.getBestDiscountRule)
	}

	server.router = router
}

// Start runs the HTTP server on a specific address.
func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

// GetRouter returns the gin router for creating http.Server
func (server *Server) GetRouter() *gin.Engine {
	return server.router
}

// healthCheck 健康检查 - 基础存活检查
// GET /health
func (server *Server) healthCheck(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, healthCheckResponse{Status: "ok", Service: "locallife-api"})
}

// readinessCheck 就绪检查 - 检查依赖服务
// GET /ready
func (server *Server) readinessCheck(ctx *gin.Context) {
	// 检查数据库连接
	if err := server.store.Ping(ctx); err != nil {
		ctx.JSON(http.StatusServiceUnavailable, serviceUnavailableResponse{
			Status: "not ready",
			Error:  "database connection failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, readinessCheckResponse{Status: "ready", Service: "locallife-api", Database: "connected"})
}

// ErrorResponse represents an API error response
// ErrorResponse 是所有 4xx HTTP 错误的统一响应体。
// 若错误来自 *APIError，则同时返回数字 code 供前端程序化分支；
// 普通错误只有 error 字段。
type ErrorResponse struct {
	// Code 为数字错误码（仅 APIError 时存在），前端应以此为准做多语言处理。
	Code int `json:"code,omitempty" example:"40401"`
	// Error 为人类可读的错误描述，降级展示或日志使用。
	Error string `json:"error" example:"error message"`
}

// errorMessage is kept for backward-compatible Swagger annotations.
// It aliases ErrorResponse so swag can resolve the type.
type errorMessage = ErrorResponse

var _ errorMessage

// errorRes is kept for backward-compatible Swagger annotations.
// It aliases ErrorResponse so swag can resolve the type.
type errorRes = ErrorResponse

var _ errorRes

// errorResponse creates an error response.
// For 4xx client errors: returns the actual error message (and code if *APIError).
// For 5xx server errors: use internalError() instead to avoid leaking details.
func errorResponse(err error) ErrorResponse {
	if apiErr := AsAPIError(err); apiErr != nil {
		return ErrorResponse{Code: apiErr.Code, Error: apiErr.Message}
	}
	return ErrorResponse{Error: err.Error()}
}

// internalError logs the actual error and returns a safe generic message.
// Use this for 5xx errors to prevent leaking internal implementation details.
//
// Example:
//
//	ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
func internalError(ctx *gin.Context, err error) ErrorResponse {
	// Attach to gin context so RequestLoggingMiddleware can include it
	_ = ctx.Error(err)

	evt := log.Error().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method)

	// If it's a Postgres error, log structured fields for faster debugging
	if pgErr, ok := err.(*pgconn.PgError); ok {
		evt = evt.
			Str("sqlstate", pgErr.Code).
			Str("pg_message", pgErr.Message).
			Str("pg_detail", pgErr.Detail).
			Str("pg_hint", pgErr.Hint).
			Str("pg_where", pgErr.Where).
			Str("pg_constraint", pgErr.ConstraintName)
	}

	evt.Msg("internal error")

	return ErrorResponse{Error: "internal server error"}
}

// successMessage creates a standard message response for simple ok/action-complete results.
func successMessage(msg string) successMessageResponse {
	return successMessageResponse{Message: msg}
}
