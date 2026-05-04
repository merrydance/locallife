package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/merrydance/locallife/api"
	"github.com/merrydance/locallife/autotag"
	"github.com/merrydance/locallife/baofu"
	baofuaccount "github.com/merrydance/locallife/baofu/account"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	db "github.com/merrydance/locallife/db/sqlc"
	_ "github.com/merrydance/locallife/docs" // Swagger docs
	"github.com/merrydance/locallife/internal/wechatruntime"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/scheduler"
	"github.com/merrydance/locallife/session"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/weather"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// @title           LocalLife API
// @version         1.0
// @description     本地生活服务平台 API 文档，包含用户、商户、订单、配送、支付等完整业务功能。
// @description
// @description     【图片URL约定】公共展示图片字段（如 image_url / avatar_url / logo_url）应返回可直接访问的绝对 URL（通常为 CDN 地址）。
// @description     - 公共展示素材不应再依赖客户端拼接 /uploads/... 路径。
// @description     - 敏感材料应使用 media_asset_id + POST /v1/media/private-access 获取短期访问地址。
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@locallife.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token with the `Bearer: ` prefix, e.g. "Bearer abcde12345".

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}
	if err := config.ValidateAliyunOCRConfig(); err != nil {
		log.Fatal().Err(err).Msg("invalid aliyun ocr config")
	}

	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()

	// Fail-fast: 生产环境必须配置显式 CORS 白名单，且不能包含通配符 *
	if config.Environment == "production" {
		if len(config.AllowedOrigins) == 0 {
			log.Fatal().Msg("ALLOWED_ORIGINS must not be empty in production")
		}
		for _, origin := range config.AllowedOrigins {
			if origin == "*" {
				log.Fatal().Msg("ALLOWED_ORIGINS must not contain wildcard '*' in production")
			}
		}
		// 生产环境资金流（赔付/退款/分账）依赖 Redis 任务队列，不允许降级为 NoopDistributor
		if config.RedisAddress == "" {
			log.Fatal().Msg("REDIS_ADDRESS is required in production (financial tasks require a real task queue)")
		}
		if err := validateProductionPaymentRuntime(config); err != nil {
			log.Fatal().Err(err).Msg("invalid production payment runtime")
		}
	}

	// ✅ P1-4: 优化数据库连接池配置
	poolConfig, err := pgxpool.ParseConfig(config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot parse db config")
	}

	// 连接池参数从配置读取，默认值在 util/config.go 中定义
	poolConfig.MaxConns = config.DBMaxConns
	poolConfig.MinConns = config.DBMinConns
	poolConfig.MaxConnLifetime = config.DBMaxConnLifetime
	poolConfig.MaxConnIdleTime = config.DBMaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.DBHealthCheckPeriod

	connPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

	// 验证连接池健康
	if err := connPool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("cannot ping database")
	}

	log.Info().
		Int32("max_conns", poolConfig.MaxConns).
		Int32("min_conns", poolConfig.MinConns).
		Msg("✅ database connection pool configured")

	if config.Environment == "production" {
		if config.AutoMigrate {
			runDBMigration(config.MigrationURL, config.DBSource)
		} else {
			log.Warn().Msg("AUTO_MIGRATE disabled in production, skipping migrations")
		}
	} else {
		runDBMigration(config.MigrationURL, config.DBSource)
	}

	store := db.NewStore(connPool)

	var weatherCache weather.WeatherCache
	var taskDistributor worker.TaskDistributor
	var redisOpt asynq.RedisClientOpt

	if config.RedisAddress == "" {
		if config.RedisRequired {
			log.Fatal().Msg("REDIS_ADDRESS is not configured but REDIS_REQUIRED is true")
		}
		log.Warn().Msg("REDIS_ADDRESS not configured, redis-dependent features disabled")
		taskDistributor = worker.NewNoopTaskDistributor()
	} else {
		redec := asynq.RedisClientOpt{
			Addr: config.RedisAddress,
			// Support authenticated Redis deployments
			Password: config.RedisPassword,
		}
		redisOpt = redec

		// 初始化天气缓存（用于测试Redis连接）
		weatherCache, err = weather.NewWeatherCache(config.RedisAddress, config.RedisPassword)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to Redis - check REDIS_ADDRESS configuration")
		}
		log.Info().Str("redis_address", config.RedisAddress).Msg("✅ Redis connection verified")
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	runDBMetricsCollector(ctx, waitGroup, connPool)

	var reconciliationPublisher websocket.PubSubPublisher
	merchantRuntimeClient, err := buildMerchantWechatClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create merchant wechat client for runtime")
	}
	var directPaymentClient wechat.DirectPaymentClientInterface
	var transferClient wechat.TransferClientInterface
	if merchantRuntimeClient != nil {
		directPaymentClient = merchantRuntimeClient
		transferClient = merchantRuntimeClient
	}
	ecommerceClient, err := buildEcommerceClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create ecommerce client for runtime")
	}
	ordinarySPClient, err := buildOrdinaryServiceProviderClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create ordinary service provider client for runtime")
	}
	baofuAggregateClient, err := buildBaofuAggregateClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create baofu aggregate client for runtime")
	}
	baofuAccountClient, err := buildBaofuAccountClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create baofu account client for runtime")
	}
	if config.RedisAddress != "" {
		// 初始化逻辑层
		redisClient := redis.NewClient(&redis.Options{
			Addr:     config.RedisAddress,
			Password: config.RedisPassword,
		})
		deliveryBroadcast := logic.NewDeliveryBroadcastLogic(store, redisClient)
		taskDistributor = runTaskProcessor(ctx, waitGroup, config, redisOpt, store, directPaymentClient, transferClient, ecommerceClient, ordinarySPClient, baofuAggregateClient, deliveryBroadcast)
		reconciliationPublisher = websocket.NewRedisPublisher(redisClient)
	}

	schedulerManager := scheduler.NewManager()
	if config.RedisAddress != "" {
		if config.QweatherAPIKey == "" || config.QweatherAPIHost == "" {
			log.Warn().Msg("qweather API not configured, weather scheduler disabled")
		} else {
			weatherClient := weather.NewQweatherClient(config.QweatherAPIKey, config.QweatherAPIHost)
			schedulerManager.Register("weather", weather.NewScheduler(store, weatherClient, weatherCache))
		}
	}

	schedulerManager.Register("auto-tag", autotag.NewScheduler(store))
	schedulerManager.Register("session-cleanup", session.NewScheduler(store))
	schedulerManager.Register("payment-recovery", worker.NewPaymentRecoveryScheduler(store, taskDistributor))
	schedulerManager.Register("wechat-notification-recovery", worker.NewWechatNotificationRecoveryScheduler(store))
	if paymentFactApplicationDistributor, ok := taskDistributor.(worker.PaymentFactApplicationTaskDistributor); ok {
		schedulerManager.Register("payment-fact-application", worker.NewPaymentFactApplicationScheduler(store, paymentFactApplicationDistributor))
	} else {
		log.Warn().Msg("payment fact application scheduler disabled: task distributor does not support payment fact application tasks")
	}
	if paymentDomainOutboxDistributor, ok := taskDistributor.(worker.PaymentDomainOutboxTaskDistributor); ok {
		schedulerManager.Register("payment-domain-outbox", worker.NewPaymentDomainOutboxScheduler(store, paymentDomainOutboxDistributor))
	} else {
		log.Warn().Msg("payment domain outbox scheduler disabled: task distributor does not support payment domain outbox tasks")
	}
	profitSharingRecoveryScheduler := worker.NewProfitSharingRecoveryScheduler(store, taskDistributor, ecommerceClient)
	profitSharingRecoveryScheduler.SetOrdinaryServiceProviderClient(ordinarySPClient)
	schedulerManager.Register("profit-sharing-recovery", profitSharingRecoveryScheduler)
	baofuPaymentRecoveryScheduler := worker.NewBaofuPaymentRecoveryScheduler(store, taskDistributor)
	if baofuAggregateClient != nil {
		baofuPaymentRecoveryScheduler.SetBaofuAggregateClient(baofuAggregateClient, worker.BaofuProfitSharingWorkerConfig{
			CollectMerchantID: config.BaofuCollectMerchantID,
			CollectTerminalID: config.BaofuCollectTerminalID,
			ShareNotifyURL:    config.EffectiveBaofuProfitSharingNotifyURL(),
		})
	} else if config.BaofuMainBusinessEnabled {
		log.Warn().Msg("baofu payment recovery scheduler remote-query branch disabled: baofu aggregate client not configured")
	}
	schedulerManager.Register("baofu-payment-recovery", baofuPaymentRecoveryScheduler)
	if baofuAccountClient != nil {
		schedulerManager.Register("baofu-withdrawal-recovery", worker.NewBaofuWithdrawalRecoveryScheduler(store, taskDistributor, baofuAccountClient, worker.BaofuWithdrawalRecoveryConfig{
			PayoutMerchantID: config.BaofuPayoutMerchantID,
			PayoutTerminalID: config.BaofuPayoutTerminalID,
		}))
	} else if config.BaofuMainBusinessEnabled {
		log.Warn().Msg("baofu withdrawal recovery scheduler disabled: baofu account client not configured")
	}
	schedulerManager.Register("profit-sharing-receiver-lifecycle", worker.NewProfitSharingReceiverLifecycleScheduler(store, taskDistributor))
	if directPaymentClient == nil {
		log.Warn().Msg("refund recovery direct status branch disabled: payment client not configured")
	}
	if ecommerceClient == nil {
		log.Warn().Msg("refund recovery ecommerce status branch disabled: ecommerce client not configured")
		log.Warn().Msg("profit sharing return recovery remote-query branch disabled: ecommerce client not configured")
	}
	if ordinarySPClient == nil {
		log.Warn().Msg("applyment recovery remote-query branch disabled: ordinary service provider client not configured")
	}
	refundRecoveryScheduler := worker.NewRefundRecoveryScheduler(store, taskDistributor, directPaymentClient, ecommerceClient)
	refundRecoveryScheduler.SetOrdinaryServiceProviderClient(ordinarySPClient)
	if baofuAggregateClient != nil {
		refundRecoveryScheduler.SetBaofuAggregateClient(baofuAggregateClient, worker.BaofuProfitSharingWorkerConfig{
			CollectMerchantID: config.BaofuCollectMerchantID,
			CollectTerminalID: config.BaofuCollectTerminalID,
		})
	} else if config.BaofuMainBusinessEnabled {
		log.Warn().Msg("refund recovery baofu status branch disabled: baofu aggregate client not configured")
	}
	schedulerManager.Register("refund-recovery", refundRecoveryScheduler)
	schedulerManager.Register("applyment-recovery", worker.NewApplymentRecoveryScheduler(store, taskDistributor, ordinarySPClient))
	if ordinarySPClient != nil {
		schedulerManager.Register("applyment-settlement-verification", worker.NewApplymentSettlementVerificationScheduler(store, taskDistributor, ordinarySPClient))
	} else {
		log.Warn().Msg("applyment settlement verification scheduler disabled: ordinary service provider client not configured")
	}
	schedulerManager.Register("merchant-withdraw-recovery", worker.NewMerchantWithdrawRecoveryScheduler(store, taskDistributor))
	schedulerManager.Register("merchant-cancel-withdraw-recovery", worker.NewMerchantCancelWithdrawRecoveryScheduler(store, taskDistributor))
	if transferClient != nil {
		schedulerManager.Register("claim-payout-recovery", worker.NewClaimPayoutRecoveryScheduler(store, transferClient))
	} else {
		log.Warn().Msg("claim payout recovery scheduler disabled: transfer client not configured")
	}
	schedulerManager.Register("claim-behavior-action-recovery", worker.NewClaimBehaviorActionRecoveryScheduler(store, taskDistributor))
	schedulerManager.Register("claim-recovery", worker.NewClaimRecoveryScheduler(store, taskDistributor))
	schedulerManager.Register("merchant-open-status", scheduler.NewMerchantOpenStatusScheduler(store))
	schedulerManager.Register("order-timeout", scheduler.NewOrderTimeoutScheduler(store))
	schedulerManager.Register("takeout-auto-complete", scheduler.NewTakeoutAutoCompleteScheduler(store, taskDistributor))
	schedulerManager.Register("data-cleanup", scheduler.NewDataCleanupScheduler(store, taskDistributor, reconciliationPublisher, ecommerceClient))
	schedulerManager.StartAll(ctx, waitGroup)

	runGinServer(ctx, waitGroup, config, store, weatherCache, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("error from wait group")
	}
}

func runDBMigration(migrationURL string, dbSource string) {
	migration, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create new migrate instance")
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().Err(err).Msg("failed to run migrate up")
	}

	log.Info().Msg("db migrated successfully")
}

func buildMerchantWechatClient(config util.Config) (*wechat.DirectPaymentClient, error) {
	if !config.HasWechatPayRuntimeConfig() {
		return nil, nil
	}

	if err := config.ValidateWechatPayConfig(); err != nil {
		return nil, err
	}

	directPaymentClient, err := wechat.NewDirectPaymentClient(wechat.DirectPaymentClientConfig{
		MchID:                     config.WechatPayMchID,
		AppID:                     config.WechatMiniAppID,
		SerialNumber:              config.WechatPaySerialNumber,
		HTTPTimeout:               config.WechatPayHTTPTimeout,
		PrivateKeyPath:            config.WechatPayPrivateKeyPath,
		APIV3Key:                  config.WechatPayAPIV3Key,
		NotifyURL:                 config.WechatPayNotifyURL,
		RefundNotifyURL:           config.WechatPayRefundNotifyURL,
		MerchantTransferNotifyURL: config.EffectiveWechatPayMerchantTransferNotifyURL(),
		PlatformPublicKeyPath:     config.WechatPayPlatformPublicKeyPath,
		PlatformPublicKeyID:       config.WechatPayPlatformPublicKeyID,
	})
	if err != nil {
		return nil, err
	}

	return directPaymentClient, nil
}

func validateProductionPaymentRuntime(config util.Config) error {
	if config.Environment != "production" {
		return nil
	}
	if config.BaofuMainBusinessEnabled {
		return config.ValidateBaofuConfig()
	}
	if !config.HasWechatOrdinaryServiceProviderRuntimeConfig() {
		return fmt.Errorf("wechat ordinary service provider runtime config is required in production for main-business payments")
	}
	return config.ValidateWechatOrdinaryServiceProviderConfig()
}

func buildEcommerceClient(config util.Config) (wechat.EcommerceClientInterface, error) {
	return wechatruntime.BuildEcommerceClient(config)
}

func buildOrdinaryServiceProviderClient(config util.Config) (*ordinaryserviceprovider.Client, error) {
	return wechatruntime.BuildOrdinaryServiceProviderClient(config)
}

func buildBaofuAggregateClient(config util.Config) (aggregatepay.Client, error) {
	if !config.HasBaofuRuntimeConfig() {
		return nil, nil
	}
	if err := config.ValidateBaofuConfig(); err != nil {
		return nil, err
	}
	root, err := baofu.NewClient(config.ToBaofuConfig(), nil)
	if err != nil {
		return nil, err
	}
	return aggregatepay.NewClient(root), nil
}

func buildBaofuAccountClient(config util.Config) (*baofuaccount.Client, error) {
	if !config.HasBaofuRuntimeConfig() {
		return nil, nil
	}
	if err := config.ValidateBaofuConfig(); err != nil {
		return nil, err
	}
	root, err := baofu.NewClient(config.ToBaofuConfig(), nil)
	if err != nil {
		return nil, err
	}
	return baofuaccount.NewClient(root), nil
}

func runTaskProcessor(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	redisOpt asynq.RedisClientOpt,
	store db.Store,
	directPaymentClient wechat.DirectPaymentClientInterface,
	transferClient wechat.TransferClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
	ordinarySPClient worker.OrdinaryServiceProviderWorkerClient,
	baofuAggregateClient aggregatepay.Client,
	deliveryBroadcast *logic.DeliveryBroadcastLogic,
) worker.TaskDistributor {
	// 创建任务分发器
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	// 创建基础微信小程序客户端，用于 OCR 与发货信息等非普通服务商资金协议能力。
	wechatClient := wechat.NewClient(config.WechatMiniAppID, config.WechatMiniAppSecret, store)

	if ecommerceClient != nil {
		log.Info().Msg("ecommerce client created for historical and cold-reserve platform ecommerce paths")
	}
	if ordinarySPClient != nil {
		log.Info().Msg("ordinary service provider client created for main-business payments")
	}

	var mediaStorage media.ObjectStorage
	if config.FileStorageProvider == "oss" {
		storage, err := media.NewOSSStorage(
			config.OSSEndpoint,
			config.OSSRegion,
			config.OSSAccessKeyID,
			config.OSSAccessKeySecret,
			config.OSSPublicBucket,
			config.OSSPrivateBucket,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot create media storage for task processor")
		}
		mediaStorage = storage
	} else {
		mediaStorage = media.NewLocalStorage(config.ExternalBaseURL, "uploads/dev")
	}
	mediaRegistry := media.NewRegistry(store, mediaStorage)

	// 创建并启动任务处理器（传入 distributor 以支持任务链）
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, taskDistributor, wechatClient, ecommerceClient, deliveryBroadcast, mediaRegistry, config)
	taskProcessor.SetDirectPaymentClient(directPaymentClient)
	taskProcessor.SetTransferClient(transferClient)
	taskProcessor.SetOrdinaryServiceProviderClient(ordinarySPClient)
	if baofuAggregateClient != nil {
		taskProcessor.SetBaofuAggregateClient(baofuAggregateClient, worker.BaofuProfitSharingWorkerConfig{
			CollectMerchantID: config.BaofuCollectMerchantID,
			CollectTerminalID: config.BaofuCollectTerminalID,
			ShareNotifyURL:    config.EffectiveBaofuProfitSharingNotifyURL(),
		})
	}
	log.Info().Msg("start task processor")

	waitGroup.Go(func() error {
		return taskProcessor.Start()
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown task processor")
		taskProcessor.Shutdown()
		log.Info().Msg("task processor is stopped")
		return nil
	})

	return taskDistributor
}

func runDBMetricsCollector(ctx context.Context, waitGroup *errgroup.Group, pool *pgxpool.Pool) {
	waitGroup.Go(func() error {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				stats := pool.Stat()
				api.UpdateDBMetrics(int(stats.AcquiredConns()), int(stats.IdleConns()))
			}
		}
	})
}

// runGinServer starts the Gin HTTP server
// Dependency Injection: config and store are passed as parameters
func runGinServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	weatherCache weather.WeatherCache,
	taskDistributor worker.TaskDistributor,
) {
	server, err := api.NewServer(config, store, weatherCache, taskDistributor, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	// 创建 http.Server 用于优雅关闭
	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: server.GetRouter(),
		// Avoid slowloris and stuck connections under pressure.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 启动WebSocket Hub（处理骑手和商户的实时通知）
	waitGroup.Go(func() error {
		log.Info().Msg("start WebSocket Hub")
		server.GetWebSocketHub().Run()
		return nil
	})

	waitGroup.Go(func() error {
		log.Info().Msgf("start HTTP server at %s", config.HTTPServerAddress)
		err = httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server failed to serve")
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown HTTP server")

		// 给予10秒时间完成正在处理的请求
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("HTTP server forced to shutdown")
			return err
		}

		server.Shutdown()

		log.Info().Msg("HTTP server is stopped")
		return nil
	})
}
