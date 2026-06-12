package worker

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
)

// TaskProcessor 任务处理接口
type TaskProcessor interface {
	Start() error
	Shutdown()
	// ProcessTaskPaymentOrderTimeout 处理支付订单超时任务
	ProcessTaskPaymentOrderTimeout(ctx context.Context, task *asynq.Task) error
	// ProcessTaskReservationPaymentTimeout 处理预定支付超时任务
	ProcessTaskReservationPaymentTimeout(ctx context.Context, task *asynq.Task) error
	// ProcessTaskReservationNoShowAlert 处理预定未到店提醒任务
	ProcessTaskReservationNoShowAlert(ctx context.Context, task *asynq.Task) error
	// ProcessTaskReservationFoodSafetyAlert 处理食安停业预订提醒任务
	ProcessTaskReservationFoodSafetyAlert(ctx context.Context, task *asynq.Task) error
	// ProcessTaskRefundResult 处理退款结果任务
	ProcessTaskRefundResult(ctx context.Context, task *asynq.Task) error
	// ProcessTaskClaimPayout 处理索赔平台赔付任务
	ProcessTaskClaimPayout(ctx context.Context, task *asynq.Task) error
	// ProcessTaskPaymentFactApplication 处理外部支付事实应用任务
	ProcessTaskPaymentFactApplication(ctx context.Context, task *asynq.Task) error
	// ProcessTaskBaofuProfitSharing 处理宝付确认分账任务
	ProcessTaskBaofuProfitSharing(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server                    *asynq.Server
	store                     db.Store
	distributor               TaskDistributor                     // 用于在任务中分发后续任务
	wechatClient              wechat.WechatClient                 // 微信小程序客户端（用于证照OCR等）
	directPaymentClient       wechat.DirectPaymentClientInterface // 直连支付客户端（骑手押金/追偿退款）
	transferClient            wechat.TransferClientInterface      // 商家转账客户端（索赔赔付到零钱）
	baofuAggregateClient      aggregatepay.Client                 // 宝付聚合支付/分账客户端
	baofuAccountClient        logic.BaofuAccountClient
	baofuWithdrawClient       logic.BaofuWithdrawClient
	baofuMerchantReportClient baofuMerchantReportContinuationClient
	dataEncryptor             util.DataEncryptor        // 本地敏感资料加解密
	pubSubPublisher           websocket.PubSubPublisher // Pub/Sub 发布器（用于推送通知）
	deliveryBroadcast         *logic.DeliveryBroadcastLogic
	mediaRegistry             *media.Registry
	ocrService                *ocr.Service
	onboardingReviewSvc       *logic.OnboardingReviewService
	credentialGovSvc          *logic.CredentialGovernanceService
	merchantReviewSvc         *logic.MerchantOnboardingReviewService
	riderReviewSvc            *logic.RiderOnboardingReviewService
	cloudPrinterManager       cloudprint.Manager
	printerClient             cloudprint.Client
	config                    util.Config
	baofuProfitSharingConfig  BaofuProfitSharingWorkerConfig
	baofuWithdrawalConfig     BaofuWithdrawalCommandDispatchConfig
	roleCache                 map[int64]cachedUserRoles
	roleCacheMu               sync.RWMutex
	roleCacheTTL              time.Duration
}

type baofuMerchantReportContinuationClient interface {
	SubmitWechatReport(ctx context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error)
	QueryReport(ctx context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error)
	BindSubConfig(ctx context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error)
}

type testStoreWithNoopPlatformAlertPersistence struct {
	db.Store
}

func (s testStoreWithNoopPlatformAlertPersistence) CreatePlatformAlertEvent(_ context.Context, _ db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
	return db.PlatformAlertEvent{}, nil
}

type cachedUserRoles struct {
	roles     []db.UserRole
	expiresAt time.Time
}

func NewRedisTaskProcessor(
	redisOpt asynq.RedisClientOpt,
	store db.Store,
	distributor TaskDistributor,
	wechatClient wechat.WechatClient,
	deliveryBroadcast *logic.DeliveryBroadcastLogic,
	mediaRegistry *media.Registry,
	config util.Config,
) *RedisTaskProcessor {
	logger := NewLogger()
	redis.SetLogger(logger)

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Queues: map[string]int{
				QueueCritical: 10,
				QueueDefault:  5,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				payload := task.Payload()
				log.Error().Err(err).Str("type", task.Type()).
					Int("payload_len", len(payload)).
					Str("payload_sha256", hashBytes(payload)).
					Msg("process task failed")
			}),
			Logger:          logger,
			ShutdownTimeout: 10 * time.Second,
		},
	)

	// 创建Redis客户端（用于Pub/Sub）
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisOpt.Addr,
		Password: redisOpt.Password,
		DB:       redisOpt.DB,
	})
	pubSubPublisher := websocket.NewRedisPublisher(redisClient)

	ocrService, err := newMerchantApplicationOCRService(store, mediaRegistry, wechatClient, config)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create food permit ocr service for task processor")
	}
	onboardingReviewSvc := logic.NewOnboardingReviewService(store)
	credentialGovSvc := logic.NewCredentialGovernanceService(store)
	cloudPrinterManager := buildRuntimeCloudPrinterManager(config)
	printerClient, _ := cloudPrinterManager.Provider(string(cloudprint.ProviderFeieyun))

	return &RedisTaskProcessor{
		server:              server,
		store:               store,
		distributor:         distributor,
		wechatClient:        wechatClient,
		transferClient:      nil,
		pubSubPublisher:     pubSubPublisher,
		deliveryBroadcast:   deliveryBroadcast,
		mediaRegistry:       mediaRegistry,
		ocrService:          ocrService,
		onboardingReviewSvc: onboardingReviewSvc,
		credentialGovSvc:    credentialGovSvc,
		merchantReviewSvc:   logic.NewMerchantOnboardingReviewService(store, onboardingReviewSvc, credentialGovSvc),
		riderReviewSvc:      logic.NewRiderOnboardingReviewService(store, onboardingReviewSvc, credentialGovSvc),
		cloudPrinterManager: cloudPrinterManager,
		printerClient:       printerClient,
		config:              config,
		roleCache:           make(map[int64]cachedUserRoles),
		roleCacheTTL:        1 * time.Minute,
	}
}

func buildRuntimeCloudPrinterManager(config util.Config) cloudprint.Manager {
	runtimeConfig := config
	runtimeConfig.CloudPrinterFailOnProviderConfigError = false
	manager, err := cloudprint.NewRuntimeManagerFromConfig(runtimeConfig)
	if err != nil {
		log.Warn().Err(err).Msg("cloud printer provider config invalid, using validated runtime provider set for task processor")
	}
	if manager == nil {
		return cloudprint.NewManagerFromConfig(util.Config{})
	}
	return manager
}

func (processor *RedisTaskProcessor) SetDirectPaymentClient(directPaymentClient wechat.DirectPaymentClientInterface) {
	processor.directPaymentClient = directPaymentClient
}

func (processor *RedisTaskProcessor) SetTransferClient(transferClient wechat.TransferClientInterface) {
	processor.transferClient = transferClient
}

func (processor *RedisTaskProcessor) SetBaofuAggregateClient(client aggregatepay.Client, config BaofuProfitSharingWorkerConfig) {
	processor.baofuAggregateClient = client
	processor.baofuProfitSharingConfig = config.normalized()
}

func (processor *RedisTaskProcessor) SetBaofuAccountClient(client logic.BaofuAccountClient, encryptor util.DataEncryptor) {
	processor.baofuAccountClient = client
	if withdrawClient, ok := client.(logic.BaofuWithdrawClient); ok {
		processor.baofuWithdrawClient = withdrawClient
	}
	processor.dataEncryptor = encryptor
}

func (processor *RedisTaskProcessor) SetBaofuWithdrawClientForTest(client logic.BaofuWithdrawClient, config BaofuWithdrawalCommandDispatchConfig) {
	processor.baofuWithdrawClient = client
	processor.baofuWithdrawalConfig = config.normalized()
}

func (processor *RedisTaskProcessor) SetBaofuMerchantReportClient(client baofuMerchantReportContinuationClient) {
	processor.baofuMerchantReportClient = client
}

func (processor *RedisTaskProcessor) SetBaofuAggregateClientForTest(client aggregatepay.Client, config BaofuProfitSharingWorkerConfig) {
	processor.SetBaofuAggregateClient(client, config)
}

func (processor *RedisTaskProcessor) SetPrinterClientForTest(client cloudprint.Client) {
	processor.printerClient = client
	processor.cloudPrinterManager = staticCloudPrinterManager{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderFeieyun): client,
	}}
}

func (processor *RedisTaskProcessor) SetCloudPrinterManagerForTest(manager cloudprint.Manager) {
	processor.cloudPrinterManager = manager
	if manager == nil {
		processor.printerClient = nil
		return
	}
	processor.printerClient, _ = manager.Provider(string(cloudprint.ProviderFeieyun))
}

// NewTestTaskProcessor 创建用于测试的处理器实例（不需要Redis连接）
func NewTestTaskProcessor(
	store db.Store,
	distributor TaskDistributor,
	wechatClient wechat.WechatClient,
	paymentClient ...wechat.DirectPaymentClientInterface,
) *RedisTaskProcessor {
	if store != nil {
		store = testStoreWithNoopPlatformAlertPersistence{Store: store}
	}
	ocrService, _ := newMerchantApplicationOCRService(store, nil, wechatClient, util.Config{})
	onboardingReviewSvc := logic.NewOnboardingReviewService(store)
	credentialGovSvc := logic.NewCredentialGovernanceService(store)
	p := &RedisTaskProcessor{
		store:               store,
		distributor:         distributor,
		wechatClient:        wechatClient,
		ocrService:          ocrService,
		onboardingReviewSvc: onboardingReviewSvc,
		credentialGovSvc:    credentialGovSvc,
		merchantReviewSvc:   logic.NewMerchantOnboardingReviewService(store, onboardingReviewSvc, credentialGovSvc),
		riderReviewSvc:      logic.NewRiderOnboardingReviewService(store, onboardingReviewSvc, credentialGovSvc),
		printerClient:       nil,
		pubSubPublisher:     websocket.NoopPublisher{},
		roleCache:           make(map[int64]cachedUserRoles),
		roleCacheTTL:        1 * time.Minute,
	}
	for _, client := range paymentClient {
		if client != nil {
			p.directPaymentClient = client
			break
		}
	}
	return p
}

type staticCloudPrinterManager struct {
	providers map[string]cloudprint.Client
}

func (m staticCloudPrinterManager) Provider(providerType string) (cloudprint.Client, bool) {
	provider, ok := m.providers[providerType]
	return provider, ok && provider != nil
}

func (m staticCloudPrinterManager) Supported(providerType string) bool {
	_, ok := m.Provider(providerType)
	return ok
}

func (m staticCloudPrinterManager) Configured() bool {
	for _, provider := range m.providers {
		if provider != nil {
			return true
		}
	}
	return false
}

func newMerchantApplicationOCRService(store db.Store, reader ocr.BinaryReader, wechatClient wechat.WechatClient, config util.Config) (*ocr.Service, error) {
	routes := make(map[ocr.DocumentType]ocr.Route)
	if config.AliyunOCREnabled {
		aliyunProvider, err := ocr.NewAliyunProviderFromConfig(config)
		if err != nil {
			return nil, err
		}
		routes[ocr.DocumentTypeBusinessLicense] = ocr.Route{Provider: aliyunProvider, Capability: ocr.CapabilityAliyunBusinessLicense}
		routes[ocr.DocumentTypeIDCard] = ocr.Route{Provider: aliyunProvider, Capability: ocr.CapabilityAliyunIDCard}
		routes[ocr.DocumentTypeFoodPermit] = ocr.Route{Provider: aliyunProvider, Capability: ocr.CapabilityAliyunFoodPermit}
		routes[ocr.DocumentTypeHealthCert] = ocr.Route{Provider: aliyunProvider, Capability: ocr.CapabilityAliyunHealthCert}
	} else if wechatClient != nil {
		routes[ocr.DocumentTypeFoodPermit] = ocr.Route{Provider: ocr.NewWechatPrintedTextProvider(wechatClient), Capability: ocr.CapabilityWechatPrintedText}
		routes[ocr.DocumentTypeHealthCert] = ocr.Route{Provider: ocr.NewWechatPrintedTextProvider(wechatClient), Capability: ocr.CapabilityWechatPrintedText}
		routes[ocr.DocumentTypeBusinessLicense] = ocr.Route{Provider: ocr.NewWechatBusinessLicenseProvider(wechatClient), Capability: ocr.CapabilityWechatBusinessLicense}
		routes[ocr.DocumentTypeIDCard] = ocr.Route{Provider: ocr.NewWechatIDCardProvider(wechatClient), Capability: ocr.CapabilityWechatIDCard}
	}
	if len(routes) == 0 {
		return nil, nil
	}
	router, err := ocr.NewStaticRouter(routes)
	if err != nil {
		return nil, err
	}
	return ocr.NewService(store, router, reader), nil
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	// 注册任务处理器
	mux.HandleFunc(TaskPaymentOrderTimeout, processor.ProcessTaskPaymentOrderTimeout)
	mux.HandleFunc(TaskReservationPaymentTimeout, processor.ProcessTaskReservationPaymentTimeout)
	mux.HandleFunc(TaskOrderPaymentTimeout, processor.ProcessTaskOrderPaymentTimeout)
	mux.HandleFunc(TaskReservationNoShowAlert, processor.ProcessTaskReservationNoShowAlert)
	mux.HandleFunc(TaskReservationFoodSafetyAlert, processor.ProcessTaskReservationFoodSafetyAlert)
	mux.HandleFunc(TaskProcessRefund, processor.ProcessTaskInitiateRefund)
	mux.HandleFunc(TaskProcessRefundResult, processor.ProcessTaskRefundResult)
	mux.HandleFunc(TaskSendNotification, processor.ProcessTaskSendNotification)
	mux.HandleFunc(TaskOperatorPendingDispatchAlert, processor.ProcessTaskOperatorPendingDispatchAlert)
	mux.HandleFunc(TaskProcessAnomalyRefund, processor.ProcessTaskAnomalyRefund)
	mux.HandleFunc(TaskPrintOrder, processor.ProcessTaskPrintOrder)

	// TrustScore系统任务
	mux.HandleFunc(TypeCheckMerchantForeignObject, processor.HandleCheckMerchantForeignObject)
	mux.HandleFunc(TypeCheckRiderDamage, processor.HandleCheckRiderDamage)

	// 追偿争议处理任务
	mux.HandleFunc(TaskAutomaticRecoveryDisputeResolution, processor.ProcessTaskAutomaticRecoveryDisputeResolution)
	mux.HandleFunc(TaskProcessRecoveryDisputeResult, processor.ProcessTaskRecoveryDisputeResult)

	// 索赔退款任务
	mux.HandleFunc(TaskClaimBehaviorAction, processor.ProcessTaskClaimBehaviorAction)
	mux.HandleFunc(TaskClaimPayout, processor.ProcessTaskClaimPayout)

	// 支付事实/outbox 处理任务
	mux.HandleFunc(TaskProcessPaymentFactApplication, processor.ProcessTaskPaymentFactApplication)
	mux.HandleFunc(TaskProcessPaymentDomainOutbox, processor.ProcessTaskPaymentDomainOutbox)
	mux.HandleFunc(TaskProcessBaofuProfitSharing, processor.ProcessTaskBaofuProfitSharing)
	mux.HandleFunc(TaskProcessBaofuAccountOpening, processor.ProcessTaskBaofuAccountOpening)
	mux.HandleFunc(TaskProcessBaofuWithdrawalFactApplication, processor.ProcessTaskBaofuWithdrawalFactApplication)
	mux.HandleFunc(TaskProcessBaofuWithdrawalCommandDispatch, processor.ProcessTaskBaofuWithdrawalCommandDispatch)

	// 商户入驻证照OCR任务
	mux.HandleFunc(TaskMerchantApplicationBusinessLicenseOCR, processor.ProcessTaskMerchantApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskMerchantApplicationFoodPermitOCR, processor.ProcessTaskMerchantApplicationFoodPermitOCR)
	mux.HandleFunc(TaskMerchantApplicationIDCardOCR, processor.ProcessTaskMerchantApplicationIDCardOCR)
	mux.HandleFunc(TaskOperatorApplicationBusinessLicenseOCR, processor.ProcessTaskOperatorApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskOperatorApplicationIDCardOCR, processor.ProcessTaskOperatorApplicationIDCardOCR)
	mux.HandleFunc(TaskRiderApplicationIDCardOCR, processor.ProcessTaskRiderApplicationIDCardOCR)
	mux.HandleFunc(TaskRiderApplicationHealthCertOCR, processor.ProcessTaskRiderApplicationHealthCertOCR)
	mux.HandleFunc(TaskOnboardingReview, processor.ProcessTaskOnboardingReview)
	mux.HandleFunc(TaskGroupApplicationBusinessLicenseOCR, processor.ProcessTaskGroupApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskGroupApplicationIDCardOCR, processor.ProcessTaskGroupApplicationIDCardOCR)

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Shutdown() {
	processor.server.Shutdown()
}
