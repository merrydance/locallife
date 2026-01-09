package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
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

// Server serves HTTP requests for our banking service.
type Server struct {
	config          util.Config
	store           db.Store
	tokenMaker      token.Maker
	wechatClient    wechat.WechatClient
	paymentClient   wechat.PaymentClientInterface   // 小程序直连支付（押金、充值）
	ecommerceClient wechat.EcommerceClientInterface // 平台收付通（订单支付分账）
	dataEncryptor   util.DataEncryptor              // 敏感数据加密器（本地存储加密）
	mapClient       maps.TencentMapClientInterface  // 腾讯地图（路径规划）
	weatherCache    weather.WeatherCache
	taskDistributor worker.TaskDistributor
	wsHub           *websocket.Hub           // WebSocket连接管理（骑手和商户）
	wsPubSub        *websocket.PubSubManager // Redis Pub/Sub管理（跨进程推送）
	router          *gin.Engine
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(config util.Config, store db.Store, weatherCache weather.WeatherCache, taskDistributor worker.TaskDistributor) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	wechatClient := wechat.NewClient(config.WechatMiniAppID, config.WechatMiniAppSecret, store)

	// 创建微信支付客户端（如果配置了支付参数）
	var paymentClient wechat.PaymentClientInterface
	var ecommerceClient wechat.EcommerceClientInterface
	if config.WechatPayMchID != "" && config.WechatPayPrivateKeyPath != "" {
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
			// SpMchID 和 SpAppID 默认与 MchID/AppID 相同
		})
		if err != nil {
			return nil, fmt.Errorf("cannot create ecommerce client: %w", err)
		}
	}

	// 创建腾讯地图客户端（如果配置了）
	var mapClient maps.TencentMapClientInterface
	if config.TencentMapKey != "" {
		mapClient = maps.NewTencentMapClient(config.TencentMapKey)
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
		log.Warn().Msg("⚠️ DATA_ENCRYPTION_KEY not configured, sensitive data will be stored in plaintext")
	}

	// 创建WebSocket Hub（用于骑手和商户实时通知）
	// 创建WebSocket Hub（管理骑手和商户的实时连接）
	wsHub := websocket.NewHub(context.Background())

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
			log.Warn().Err(err).Msg("failed to initialize Casbin, RBAC will use fallback middleware")
		}
	}

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		wechatClient:    wechatClient,
		paymentClient:   paymentClient,
		ecommerceClient: ecommerceClient,
		dataEncryptor:   dataEncryptor,
		mapClient:       mapClient,
		weatherCache:    weatherCache,
		taskDistributor: taskDistributor,
		wsHub:           wsHub,
		wsPubSub:        wsPubSub,
	}

	server.setupRouter()
	return server, nil
}

// GetWebSocketHub returns the WebSocket hub for external access
func (server *Server) GetWebSocketHub() *websocket.Hub {
	return server.wsHub
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

	// 🖼️ 上传文件访问
	// 安全策略：证件照/营业执照等敏感图片不允许匿名直出；
	// 通过 /v1/uploads/sign 生成短期签名URL，再由 /uploads/*filepath 校验签名后提供下载。
	router.GET("/uploads/*filepath", server.getSignedUpload)

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
	rateLimiter := NewRateLimiter(DefaultRateLimiterConfig())
	router.Use(rateLimiter.Middleware())

	// 🕐 全局超时中间件：防止慢查询、外部API卡死导致goroutine泄漏
	router.Use(TimeoutMiddleware(30 * time.Second))

	// 📊 Prometheus 指标端点（供监控系统抓取）
	router.GET("/metrics", MetricsHandler())

	// 🏥 健康检查端点（供 Nginx/K8s 使用，无需认证）
	router.GET("/health", server.healthCheck)
	router.GET("/ready", server.readinessCheck)

	// Swagger API 文档（开发环境）
	if server.config.Environment == "development" {
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
	authPublicGroup.Use(rateLimiter.SensitiveAPIMiddleware(10)) // 敏感 API 更严格限制：每分钟 10 次
	authPublicGroup.POST("/wechat-login", server.wechatLogin)
	authPublicGroup.POST("/refresh", server.renewAccessToken)

	// 微信支付回调路由（无需认证，微信服务器调用）
	webhooksGroup := v1.Group("/webhooks")
	{
		// 小程序直连支付回调
		webhooksGroup.POST("/wechat-pay/notify", server.handlePaymentNotify)
		webhooksGroup.POST("/wechat-pay/refund-notify", server.handleRefundNotify)
		// 平台收付通回调
		webhooksGroup.POST("/wechat-ecommerce/notify", server.handleCombinePaymentNotify)
		webhooksGroup.POST("/wechat-ecommerce/refund-notify", server.handleEcommerceRefundNotify)
		webhooksGroup.POST("/wechat-ecommerce/applyment-notify", server.handleApplymentStateNotify)
		webhooksGroup.POST("/wechat-ecommerce/profit-sharing-notify", server.handleProfitSharingNotify)
	}

	// M2: 地区查询路由(无需认证)
	// 说明：当前线上联调阶段前端直接使用腾讯 LBS 接口获取行政区划/POI 等数据。
	// 这里的 /v1/regions* 接口作为后备能力保留（降级/灾备/未来切回），暂时可能不会被调用。
	v1.GET("/regions/available", server.listAvailableRegions)
	v1.GET("/regions/:id/check", server.checkRegionAvailability)
	v1.GET("/regions/:id", server.getRegion)
	v1.GET("/regions", server.listRegions)
	v1.GET("/regions/:id/children", server.listRegionChildren)
	v1.GET("/regions/search", server.searchRegions)

	// 搜索路由（无需认证）
	searchGroup := v1.Group("/search")
	{
		searchGroup.GET("/dishes", server.searchDishes)
		searchGroup.GET("/merchants", server.searchMerchants)
		searchGroup.GET("/combos", server.searchCombos) // 套餐搜索
		searchGroup.GET("/rooms", server.searchRooms)   // 包间搜索：按日期、时段、人数、菜系等条件
	}

	// 餐厅优惠活动（无需认证）
	v1.GET("/merchants/:id/promotions", server.getMerchantPromotions)

	// 扫码点餐路由（无需认证）
	v1.GET("/scan/table", server.scanTable)

	// 需要认证的路由
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware(server.tokenMaker))
	authGroup.POST("/uploads/sign", server.signUploadURL)

	// 消费者菜品详情（需认证，但不需要商户权限）
	authGroup.GET("/public/dishes/:id", server.getPublicDishDetail)
	// 消费者商户详情（需认证，但不需要商户权限）
	authGroup.GET("/public/merchants/:id", server.getPublicMerchantDetail)
	authGroup.GET("/public/merchants/:id/dishes", server.getPublicMerchantDishes)
	authGroup.GET("/public/merchants/:id/combos", server.getPublicMerchantCombos)
	authGroup.GET("/public/merchants/:id/rooms", server.getPublicMerchantRooms)

	// 分享功能由小程序前端 share 属性实现，无需后端API

	// M5.1: 运营商入驻申请路由（草稿模式+人工审核）
	authGroup.POST("/operator/application", server.getOrCreateOperatorApplicationDraft)          // 创建或获取申请草稿
	authGroup.GET("/operator/application", server.getOperatorApplication)                        // 获取申请状态
	authGroup.PUT("/operator/application/region", server.updateOperatorApplicationRegion)        // 更新申请区域
	authGroup.PUT("/operator/application/basic", server.updateOperatorApplicationBasicInfo)      // 更新基础信息
	authGroup.POST("/operator/application/license/ocr", server.uploadOperatorBusinessLicenseOCR) // 上传营业执照OCR
	authGroup.POST("/operator/application/idcard/ocr", server.uploadOperatorIDCardOCR)           // 上传身份证OCR
	authGroup.POST("/operator/application/submit", server.submitOperatorApplication)             // 提交申请
	authGroup.POST("/operator/application/reset", server.resetOperatorApplicationToDraft)        // 重置为草稿

	// M5.2: 运营商开户（微信支付二级商户进件）
	operatorApplymentGroup := authGroup.Group("/operator/applyment")
	{
		operatorApplymentGroup.POST("/bindbank", server.operatorBindBank)        // 绑定银行卡开户
		operatorApplymentGroup.GET("/status", server.getOperatorApplymentStatus) // 获取开户状态
	}

	// M1: 用户相关路由
	authGroup.GET("/users/me", server.getCurrentUser)
	authGroup.PATCH("/users/me", server.updateCurrentUser)
	authGroup.POST("/auth/bind-phone", server.bindPhone)

	// M2: 用户地址路由
	authGroup.POST("/addresses", server.createUserAddress)
	authGroup.GET("/addresses", server.listUserAddresses)
	authGroup.GET("/addresses/:id", server.getUserAddress)
	authGroup.PATCH("/addresses/:id", server.updateUserAddress)
	authGroup.PATCH("/addresses/:id/default", server.setDefaultAddress)
	authGroup.DELETE("/addresses/:id", server.deleteUserAddress)

	// M2: 位置服务（需要认证，避免滥用地图 Key）
	authGroup.GET("/location/reverse-geocode", server.reverseGeocode)
	authGroup.GET("/location/direction/bicycling", server.proxyTencentBicyclingDirection)

	// M3: 商户管理路由
	authGroup.POST("/merchants/images/upload", server.uploadMerchantImage)
	authGroup.POST("/merchants/applications", server.createMerchantApplication)
	authGroup.GET("/merchants/applications/me", server.getUserMerchantApplication)
	authGroup.GET("/merchants/me", server.getCurrentMerchant)
	authGroup.GET("/merchants/my", server.listMyMerchants) // 获取用户所有商户（多店铺切换）
	authGroup.PATCH("/merchants/me", server.updateCurrentMerchant)
	authGroup.GET("/merchants/me/status", server.getMerchantOpenStatus)
	authGroup.PATCH("/merchants/me/status", server.updateMerchantOpenStatus)
	authGroup.GET("/merchants/me/business-hours", server.getMerchantBusinessHours)
	authGroup.PUT("/merchants/me/business-hours", server.setMerchantBusinessHours)
	authGroup.GET("/merchants/me/membership-settings", server.getMerchantMembershipSettings)
	authGroup.PUT("/merchants/me/membership-settings", server.updateMerchantMembershipSettings)

	// M3.1: 商户入驻申请（新版 - 自动审核）
	merchantAppGroup := authGroup.Group("/merchant/application")
	{
		merchantAppGroup.GET("", server.getOrCreateMerchantApplicationDraft)           // 创建/获取草稿
		merchantAppGroup.PUT("/basic", server.updateMerchantApplicationBasicInfo)      // 更新基础信息
		merchantAppGroup.PUT("/images", server.updateMerchantApplicationImages)        // 更新门头照/环境照
		merchantAppGroup.POST("/license/ocr", server.uploadMerchantBusinessLicenseOCR) // 上传营业执照OCR
		merchantAppGroup.POST("/foodpermit/ocr", server.uploadMerchantFoodPermitOCR)   // 上传食品许可证OCR
		merchantAppGroup.POST("/idcard/ocr", server.uploadMerchantIDCardOCR)           // 上传身份证OCR
		merchantAppGroup.POST("/submit", server.submitMerchantApplication)             // 提交申请（自动审核）
		merchantAppGroup.POST("/reset", server.resetMerchantApplication)               // 重置申请（被拒后）
	}

	// M3.2: 商户开户（微信支付二级商户进件）
	merchantApplymentGroup := authGroup.Group("/merchant/applyment")
	{
		merchantApplymentGroup.POST("/bindbank", server.merchantBindBank)        // 绑定银行卡开户
		merchantApplymentGroup.GET("/status", server.getMerchantApplymentStatus) // 获取开户状态
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

	// M3: 商户审核路由（管理员）
	authGroup.GET("/admin/merchants/applications", server.listMerchantApplications)
	authGroup.POST("/admin/merchants/applications/review", server.reviewMerchantApplication)

	// M3.6: Boss 认领店铺（任意登录用户）
	authGroup.POST("/claim-boss", server.claimBoss)

	// M3.7: Boss 店铺列表
	bossGroup := authGroup.Group("/boss")
	{
		bossGroup.GET("/merchants", server.listBossMerchants)
	}

	// M3.8: Boss 管理（仅店主可操作）
	merchantBossGroup := authGroup.Group("/merchant")
	merchantBossGroup.Use(server.MerchantStaffMiddleware("owner"))
	{
		merchantBossGroup.POST("/boss-bind-code", server.generateBossBindCode)
		merchantBossGroup.GET("/bosses", server.listMerchantBosses)
		merchantBossGroup.DELETE("/bosses/:id", server.removeBoss)
	}

	// M4: 标签管理路由
	tagsGroup := authGroup.Group("/tags")
	{
		tagsGroup.GET("", server.listTags)   // 获取标签列表（按类型）
		tagsGroup.POST("", server.createTag) // 创建标签
	}

	// M4: 菜品管理路由
	dishesGroup := authGroup.Group("/dishes")
	{
		dishesGroup.POST("/images/upload", server.uploadDishImage)
		// 菜品分类
		dishesGroup.POST("/categories", server.createDishCategory)
		dishesGroup.GET("/categories", server.listDishCategories)
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
	}

	// M4: 套餐管理路由
	combosGroup := authGroup.Group("/combos")
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
			deliveryFeeMerchantGroup.DELETE("/promotions/:id", server.deleteDeliveryPromotion)
		}

		// 运费计算（核心接口 - 无需特殊权限）
		deliveryFeeGroup.POST("/calculate", server.calculateDeliveryFee)
	}

	// M5: 桌台与包间管理路由
	tablesGroup := authGroup.Group("/tables")
	{
		tablesGroup.POST("/images/upload", server.uploadTableImage)
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
		reservationsGroup.POST("/:id/checkin", server.checkInReservation)            // 到店签到
		reservationsGroup.POST("/:id/start-cooking", server.startCookingReservation) // 起菜通知

		// 商户管理
		reservationsGroup.GET("/merchant", server.listMerchantReservations)
		reservationsGroup.GET("/merchant/today", server.listTodayReservations) // 今日预订
		reservationsGroup.GET("/merchant/stats", server.getReservationStats)
		reservationsGroup.POST("/merchant/create", server.merchantCreateReservation) // 商户代客创建
		reservationsGroup.PUT("/:id/update", server.merchantUpdateReservation)       // 商户修改预订
		reservationsGroup.POST("/:id/confirm", server.confirmReservation)
		reservationsGroup.POST("/:id/complete", server.completeReservation)
		reservationsGroup.POST("/:id/no-show", server.markNoShow)
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
		ordersGroup.POST("/:id/urge", server.urgeOrder)
		ordersGroup.POST("/:id/confirm", server.confirmOrder)
	}

	// M7: 商户端订单管理路由
	merchantOrdersGroup := authGroup.Group("/merchant/orders")
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
		merchantClaimsGroup.POST("/appeals", server.createMerchantAppeal)
		merchantClaimsGroup.GET("/appeals", server.listMerchantAppeals)
		merchantClaimsGroup.GET("/appeals/:id", server.getMerchantAppealDetail)
	}

	// M7.5: 支付订单路由
	paymentGroup := authGroup.Group("/payments")
	{
		paymentGroup.POST("", server.createPaymentOrder)
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
	}

	// M8: 骑手管理路由
	riderGroup := authGroup.Group("/rider")
	{
		// 骑手申请流程（新版）
		riderGroup.GET("/application", server.createOrGetRiderApplicationDraft)  // 创建/获取草稿
		riderGroup.PUT("/application/basic", server.updateRiderApplicationBasic) // 更新基础信息
		riderGroup.POST("/application/idcard/ocr", server.uploadRiderIDCardOCR)  // 上传身份证OCR
		riderGroup.POST("/application/healthcert", server.uploadRiderHealthCert) // 上传健康证
		riderGroup.POST("/application/submit", server.submitRiderApplication)    // 提交申请
		riderGroup.POST("/application/reset", server.resetRiderApplication)      // 重置申请（被拒后）

		// 骑手开户（微信支付二级商户进件）
		riderGroup.POST("/applyment/bindbank", server.riderBindBank)        // 绑定银行卡开户
		riderGroup.GET("/applyment/status", server.getRiderApplymentStatus) // 获取开户状态

		// 骑手入驻（旧版，保留兼容）
		riderGroup.POST("/apply", server.applyRider)
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
	adminRiderGroup.Use(server.RoleMiddleware(RoleOperator, RoleAdmin))
	{
		adminRiderGroup.GET("", server.listRiders)
		adminRiderGroup.POST("/:rider_id/approve", server.approveRider)
		adminRiderGroup.POST("/:rider_id/reject", server.rejectRider)
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
	{
		merchantFinanceGroup.GET("/overview", server.getMerchantFinanceOverview)
		merchantFinanceGroup.GET("/orders", server.listMerchantFinanceOrders)
		merchantFinanceGroup.GET("/service-fees", server.listMerchantServiceFees)
		merchantFinanceGroup.GET("/promotions", server.listMerchantPromotionExpenses)
		merchantFinanceGroup.GET("/daily", server.listMerchantDailyFinance)
		merchantFinanceGroup.GET("/settlements", server.listMerchantSettlements)
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
		// 区域相关路由（需要额外验证区域管理权限）
		operatorStatsGroup.GET("/regions/:region_id/stats", server.getRegionStats)
		operatorStatsGroup.POST("/regions/:region_id/peak-hours", server.createPeakHourConfig)
		operatorStatsGroup.GET("/regions/:region_id/peak-hours", server.listPeakHourConfigs)

		// 多维度分析
		operatorStatsGroup.GET("/merchants/ranking", server.getOperatorMerchantRanking)
		operatorStatsGroup.GET("/riders/ranking", server.getOperatorRiderRanking)
		operatorStatsGroup.GET("/trend/daily", server.getRegionDailyTrend)

		// 高峰时段删除（handler 内部验证区域）
		operatorStatsGroup.DELETE("/peak-hours/:id", server.deletePeakHourConfig)

		// 商户管理（完整CRUD + 暂停/恢复）
		operatorStatsGroup.GET("/merchants", server.listOperatorMerchants)
		operatorStatsGroup.GET("/merchants/:id", server.getOperatorMerchant)
		operatorStatsGroup.POST("/merchants/:id/suspend", server.suspendOperatorMerchant)
		operatorStatsGroup.POST("/merchants/:id/resume", server.resumeOperatorMerchant)

		// 骑手管理（完整CRUD + 暂停/恢复）
		operatorStatsGroup.GET("/riders", server.listOperatorRiders)
		operatorStatsGroup.GET("/riders/:id", server.getOperatorRider)
		operatorStatsGroup.POST("/riders/:id/suspend", server.suspendOperatorRider)
		operatorStatsGroup.POST("/riders/:id/resume", server.resumeOperatorRider)

		// 申诉处理（运营商审核商户/骑手申诉）
		operatorStatsGroup.GET("/appeals", server.listOperatorAppeals)
		operatorStatsGroup.GET("/appeals/:id", server.getOperatorAppealDetail)
		operatorStatsGroup.POST("/appeals/:id/review", server.reviewAppeal)
	}

	// 运营商财务路由 (使用 /operators/me 路径)
	operatorsGroup := authGroup.Group("/operators/me")
	operatorsGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware())
	{
		operatorsGroup.GET("/finance/overview", server.getOperatorFinanceOverview)
		operatorsGroup.GET("/commission", server.getOperatorCommission)
	}

	// M12: 平台统计BI路由
	// 使用 Casbin 中间件验证 admin 角色
	platformStatsGroup := authGroup.Group("/platform/stats")
	platformStatsGroup.Use(server.CasbinRoleMiddleware(RoleAdmin))
	{
		platformStatsGroup.GET("/overview", server.getPlatformOverview)
		platformStatsGroup.GET("/daily", server.getPlatformDailyStats)
		platformStatsGroup.GET("/regions/compare", server.getRegionComparison)
		platformStatsGroup.GET("/merchants/ranking", server.getMerchantRanking)
		platformStatsGroup.GET("/categories", server.getCategoryStats)
		platformStatsGroup.GET("/growth/users", server.getUserGrowthStats)
		platformStatsGroup.GET("/growth/merchants", server.getMerchantGrowthStats)
		platformStatsGroup.GET("/riders/ranking", server.getRiderRanking)
		platformStatsGroup.GET("/hourly", server.getHourlyDistribution)
		platformStatsGroup.GET("/realtime", server.getRealtimeDashboard)
	}

	// M9: TrustScore信任分系统路由
	trustScoreGroup := authGroup.Group("/trust-score")
	{
		// 查询信用分画像
		trustScoreGroup.GET("/profiles/:role/:id", server.GetTrustScoreProfile)

		// 查询信用分变更历史
		trustScoreGroup.GET("/history/:role/:id", server.GetTrustScoreHistory)

		// 提交索赔
		trustScoreGroup.POST("/claims", server.SubmitClaim)

		// 审核索赔（管理员/客服）
		trustScoreGroup.PATCH("/claims/:id/review", server.ReviewClaim)

		// 上报食安问题
		trustScoreGroup.POST("/food-safety/report", server.ReportFoodSafety)

		// 熔断商户（管理员）
		trustScoreGroup.PATCH("/merchants/:id/suspend", server.SuspendMerchant)

		// 触发欺诈检测（管理员/自动触发）
		trustScoreGroup.POST("/fraud/detect", server.TriggerFraudDetection)

		// 提交恢复申请（商户/骑手）
		trustScoreGroup.POST("/recovery", server.SubmitRecoveryRequest)

		// 提交申诉
		trustScoreGroup.POST("/appeals", server.SubmitAppeal)
	}

	// 用户索赔路由
	claimsGroup := authGroup.Group("/claims")
	{
		claimsGroup.GET("", server.ListUserClaims)
		claimsGroup.GET("/:id", server.GetClaimDetail)
	}

	// 购物车路由
	cartGroup := authGroup.Group("/cart")
	{
		cartGroup.GET("", server.getCart)
		cartGroup.GET("/summary", server.getUserCartsSummary)                        // 多商户购物车汇总
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
		// 上传评价图片
		reviewsGroup.POST("/images/upload", server.uploadReviewImage)

		// 创建评价
		reviewsGroup.POST("", server.createReview)

		// 查询评价详情
		reviewsGroup.GET("/:id", server.getReview)

		// 查询用户的评价列表
		reviewsGroup.GET("/me", server.listUserReviews)

		// 查询商户的评价列表（顾客视角，仅可见评价）
		reviewsGroup.GET("/merchants/:id", server.listMerchantReviews)

		// 商户查看所有评价（包含不可见的）
		reviewsGroup.GET("/merchants/:id/all", server.listMerchantAllReviews)

		// 商户回复评价
		reviewsGroup.POST("/:id/reply", server.replyReview)
	}

	// 删除评价（运营商权限）
	// 使用 Casbin 中间件验证 operator 角色
	reviewsOperatorGroup := authGroup.Group("/reviews")
	reviewsOperatorGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware())
	{
		reviewsOperatorGroup.DELETE("/:id", server.deleteReview)
	}

	// M11: 千人千面推荐引擎路由
	behaviorsGroup := authGroup.Group("/behaviors")
	{
		// 用户行为埋点
		behaviorsGroup.POST("/track", server.trackBehavior)
	}

	recommendationsGroup := authGroup.Group("/recommendations")
	{
		// 推荐菜品
		recommendationsGroup.GET("/dishes", server.recommendDishes)

		// 推荐套餐
		recommendationsGroup.GET("/combos", server.recommendCombos)

		// 推荐商户
		recommendationsGroup.GET("/merchants", server.recommendMerchants)

		// 探索包间
		recommendationsGroup.GET("/rooms", server.exploreRooms)
	}

	// 推荐配置管理（运营商）
	// 使用 Casbin 中间件验证 operator 角色、加载 operator 信息，并验证管理该区域
	regionConfigGroup := authGroup.Group("/regions")
	regionConfigGroup.Use(server.CasbinRoleMiddleware(RoleOperator), server.LoadOperatorMiddleware(), server.ValidateOperatorRegionMiddleware("id"))
	{
		regionConfigGroup.PATCH("/:id/recommendation-config", server.updateRecommendationConfig)
		regionConfigGroup.GET("/:id/recommendation-config", server.getRecommendationConfig)
	}

	// 充值规则管理（商户）
	rechargeRuleGroup := authGroup.Group("/merchants/:id/recharge-rules")
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
	ctx.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "locallife-api",
	})
}

// readinessCheck 就绪检查 - 检查依赖服务
// GET /ready
func (server *Server) readinessCheck(ctx *gin.Context) {
	// 检查数据库连接
	if err := server.store.Ping(ctx); err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "database connection failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"service":  "locallife-api",
		"database": "connected",
	})
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
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
// For 4xx client errors: returns the actual error message
// For 5xx server errors: use internalError() instead to avoid leaking details
func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

// internalError logs the actual error and returns a safe generic message.
// Use this for 5xx errors to prevent leaking internal implementation details.
//
// Example:
//
//	ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
func internalError(ctx *gin.Context, err error) gin.H {
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

	return gin.H{"error": "internal server error"}
}
