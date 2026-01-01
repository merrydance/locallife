package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/merrydance/locallife/api"
	"github.com/merrydance/locallife/autotag"
	db "github.com/merrydance/locallife/db/sqlc"
	_ "github.com/merrydance/locallife/docs" // Swagger docs
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/weather"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// @title           LocalLife API
// @version         1.0
// @description     本地生活服务平台 API 文档，包含用户、商户、订单、配送、支付等完整业务功能。
// @description
// @description     【图片URL约定】API 中的图片字段（如 image_url / avatar_url / logo_url）通常返回以 /uploads/ 开头的路径。
// @description     - 公共展示素材（例如菜品/桌台/包间/评价图片、商户logo等）可直接通过 GET /uploads/... 访问。
// @description     - 敏感材料（证照、身份证、健康证等）必须先调用 POST /v1/uploads/sign 获取短期签名URL，再用该URL下载。
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

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()

	// ✅ P1-4: 优化数据库连接池配置
	poolConfig, err := pgxpool.ParseConfig(config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot parse db config")
	}

	// 连接池优化参数（根据生产环境调整）
	poolConfig.MaxConns = 25                       // 最大连接数（默认4）
	poolConfig.MinConns = 5                        // 最小空闲连接（默认0）
	poolConfig.MaxConnLifetime = 1 * time.Hour     // 连接最大生命周期
	poolConfig.MaxConnIdleTime = 30 * time.Minute  // 空闲连接超时
	poolConfig.HealthCheckPeriod = 1 * time.Minute // 健康检查频率

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

	runDBMigration(config.MigrationURL, config.DBSource)

	store := db.NewStore(connPool)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
		// Support authenticated Redis deployments
		Password: config.RedisPassword,
	}

	// ✅ P1-1: 验证Redis连接
	if config.RedisAddress == "" {
		log.Fatal().Msg("REDIS_ADDRESS is not configured")
	}

	// 初始化天气缓存（用于测试Redis连接）
	var weatherCache weather.WeatherCache
	weatherCache, err = weather.NewWeatherCache(config.RedisAddress, config.RedisPassword)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis - check REDIS_ADDRESS configuration")
	}
	log.Info().Str("redis_address", config.RedisAddress).Msg("✅ Redis connection verified")

	waitGroup, ctx := errgroup.WithContext(ctx)

	taskDistributor := runTaskProcessor(ctx, waitGroup, config, redisOpt, store)
	runWeatherScheduler(ctx, waitGroup, config, store, weatherCache)
	runAutoTagScheduler(ctx, waitGroup, store)
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

func runTaskProcessor(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	redisOpt asynq.RedisClientOpt,
	store db.Store,
) worker.TaskDistributor {
	// 创建任务分发器
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	// 创建平台收付通客户端（用于分账）
	wechatClient := wechat.NewClient(config.WechatMiniAppID, config.WechatMiniAppSecret, store)

	var ecommerceClient wechat.EcommerceClientInterface
	if config.WechatPayMchID != "" && config.WechatPayPrivateKeyPath != "" {
		client, err := wechat.NewEcommerceClient(wechat.EcommerceClientConfig{
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
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to create ecommerce client, profit sharing disabled")
		} else {
			ecommerceClient = client
			log.Info().Msg("ecommerce client created for profit sharing")
		}
	}

	// 创建并启动任务处理器（传入 distributor 以支持任务链）
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, taskDistributor, wechatClient, ecommerceClient)
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
} // runWeatherScheduler starts the weather data fetching scheduler
func runWeatherScheduler(
	ctx context.Context,
	waitGroup *errgroup.Group,
	config util.Config,
	store db.Store,
	weatherCache weather.WeatherCache,
) {
	// 如果没有配置和风天气 API，跳过
	if config.QweatherAPIKey == "" || config.QweatherAPIHost == "" {
		log.Warn().Msg("qweather API not configured, weather scheduler disabled")
		return
	}

	// 创建天气客户端
	weatherClient := weather.NewQweatherClient(config.QweatherAPIKey, config.QweatherAPIHost)

	// 创建调度器
	scheduler := weather.NewScheduler(store, weatherClient, weatherCache)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start weather scheduler")
		return
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown weather scheduler")
		scheduler.Stop()
		return nil
	})
}

// runAutoTagScheduler starts the auto-tagging scheduler
func runAutoTagScheduler(
	ctx context.Context,
	waitGroup *errgroup.Group,
	store db.Store,
) {
	// 创建自动标签调度器
	scheduler := autotag.NewScheduler(store)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start auto-tag scheduler")
		return
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown auto-tag scheduler")
		scheduler.Stop()
		return nil
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
	server, err := api.NewServer(config, store, weatherCache, taskDistributor)
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

		log.Info().Msg("HTTP server is stopped")
		return nil
	})
}
