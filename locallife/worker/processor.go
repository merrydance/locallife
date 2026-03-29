package worker

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
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
	// ProcessTaskCombinedPaymentOrderTimeout 处理合单支付超时任务
	ProcessTaskCombinedPaymentOrderTimeout(ctx context.Context, task *asynq.Task) error
	// ProcessTaskReservationPaymentTimeout 处理预定支付超时任务
	ProcessTaskReservationPaymentTimeout(ctx context.Context, task *asynq.Task) error
	// ProcessTaskReservationNoShowAlert 处理预定未到店提醒任务
	ProcessTaskReservationNoShowAlert(ctx context.Context, task *asynq.Task) error
	// ProcessTaskPaymentSuccess 处理支付成功任务
	ProcessTaskPaymentSuccess(ctx context.Context, task *asynq.Task) error
	// ProcessTaskRefundResult 处理退款结果任务
	ProcessTaskRefundResult(ctx context.Context, task *asynq.Task) error
	// ProcessTaskProfitSharing 处理分账任务
	ProcessTaskProfitSharing(ctx context.Context, task *asynq.Task) error
	// ProcessTaskProfitSharingReturnResult 处理分账回退结果任务
	ProcessTaskProfitSharingReturnResult(ctx context.Context, task *asynq.Task) error
	// ProcessTaskClaimPayout 处理索赔平台赔付任务
	ProcessTaskClaimPayout(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server            *asynq.Server
	store             db.Store
	distributor       TaskDistributor                 // 用于在任务中分发后续任务
	wechatClient      wechat.WechatClient             // 微信小程序客户端（用于证照OCR等）
	paymentClient     wechat.PaymentClientInterface   // 平台直连支付客户端（赔付到零钱）
	ecommerceClient   wechat.EcommerceClientInterface // 平台收付通客户端（分账）
	pubSubPublisher   websocket.PubSubPublisher       // Pub/Sub 发布器（用于推送通知）
	deliveryBroadcast *logic.DeliveryBroadcastLogic
	mediaRegistry     *media.Registry
	ocrService        *ocr.Service
	config            util.Config
	roleCache         map[int64]cachedUserRoles
	roleCacheMu       sync.RWMutex
	roleCacheTTL      time.Duration
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
	ecommerceClient wechat.EcommerceClientInterface,
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

	return &RedisTaskProcessor{
		server:            server,
		store:             store,
		distributor:       distributor,
		wechatClient:      wechatClient,
		paymentClient:     nil,
		ecommerceClient:   ecommerceClient,
		pubSubPublisher:   pubSubPublisher,
		deliveryBroadcast: deliveryBroadcast,
		mediaRegistry:     mediaRegistry,
		ocrService:        ocrService,
		config:            config,
		roleCache:         make(map[int64]cachedUserRoles),
		roleCacheTTL:      1 * time.Minute,
	}
}

func (processor *RedisTaskProcessor) SetPaymentClient(paymentClient wechat.PaymentClientInterface) {
	processor.paymentClient = paymentClient
}

// NewTestTaskProcessor 创建用于测试的处理器实例（不需要Redis连接）
func NewTestTaskProcessor(
	store db.Store,
	distributor TaskDistributor,
	wechatClient wechat.WechatClient,
	ecommerceClient wechat.EcommerceClientInterface,
) *RedisTaskProcessor {
	ocrService, _ := newMerchantApplicationOCRService(store, nil, wechatClient, util.Config{})
	return &RedisTaskProcessor{
		store:           store,
		distributor:     distributor,
		wechatClient:    wechatClient,
		ecommerceClient: ecommerceClient,
		ocrService:      ocrService,
		pubSubPublisher: websocket.NoopPublisher{},
		roleCache:       make(map[int64]cachedUserRoles),
		roleCacheTTL:    1 * time.Minute,
	}
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
	} else if wechatClient != nil {
		routes[ocr.DocumentTypeFoodPermit] = ocr.Route{Provider: ocr.NewWechatPrintedTextProvider(wechatClient), Capability: ocr.CapabilityWechatPrintedText}
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
	mux.HandleFunc(TaskCombinedPaymentOrderTimeout, processor.ProcessTaskCombinedPaymentOrderTimeout)
	mux.HandleFunc(TaskReservationPaymentTimeout, processor.ProcessTaskReservationPaymentTimeout)
	mux.HandleFunc(TaskOrderPaymentTimeout, processor.ProcessTaskOrderPaymentTimeout)
	mux.HandleFunc(TaskReservationNoShowAlert, processor.ProcessTaskReservationNoShowAlert)
	mux.HandleFunc(TaskProcessPaymentSuccess, processor.ProcessTaskPaymentSuccess)
	mux.HandleFunc(TaskProcessRefund, processor.ProcessTaskInitiateRefund)
	mux.HandleFunc(TaskProcessRefundResult, processor.ProcessTaskRefundResult)
	mux.HandleFunc(TaskProcessProfitSharing, processor.ProcessTaskProfitSharing)
	mux.HandleFunc(TaskSendNotification, processor.ProcessTaskSendNotification)
	mux.HandleFunc(TaskProcessProfitSharingReturnResult, processor.ProcessTaskProfitSharingReturnResult)
	mux.HandleFunc(TaskProcessMerchantWithdrawResult, processor.ProcessTaskMerchantWithdrawResult)
	mux.HandleFunc(TaskProcessAnomalyRefund, processor.ProcessTaskAnomalyRefund)

	// TrustScore系统任务
	mux.HandleFunc(TypeHandleSuspiciousPattern, processor.HandleSuspiciousPattern)
	mux.HandleFunc(TypeCheckMerchantForeignObject, processor.HandleCheckMerchantForeignObject)
	mux.HandleFunc(TypeCheckRiderDamage, processor.HandleCheckRiderDamage)

	// 申诉处理任务
	mux.HandleFunc(TaskProcessAppealResult, processor.ProcessTaskProcessAppealResult)

	// 索赔退款任务
	mux.HandleFunc(TaskClaimPayout, processor.ProcessTaskClaimPayout)

	// 进件/分账结果处理任务
	mux.HandleFunc(TaskProcessApplymentResult, processor.ProcessTaskApplymentResult)
	mux.HandleFunc(TaskProcessProfitSharingResult, processor.ProcessTaskProfitSharingResult)

	// 商户入驻证照OCR任务
	mux.HandleFunc(TaskMerchantApplicationBusinessLicenseOCR, processor.ProcessTaskMerchantApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskMerchantApplicationFoodPermitOCR, processor.ProcessTaskMerchantApplicationFoodPermitOCR)
	mux.HandleFunc(TaskMerchantApplicationIDCardOCR, processor.ProcessTaskMerchantApplicationIDCardOCR)
	mux.HandleFunc(TaskOperatorApplicationBusinessLicenseOCR, processor.ProcessTaskOperatorApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskOperatorApplicationIDCardOCR, processor.ProcessTaskOperatorApplicationIDCardOCR)
	mux.HandleFunc(TaskRiderApplicationIDCardOCR, processor.ProcessTaskRiderApplicationIDCardOCR)
	mux.HandleFunc(TaskRiderApplicationHealthCertOCR, processor.ProcessTaskRiderApplicationHealthCertOCR)
	mux.HandleFunc(TaskGroupApplicationBusinessLicenseOCR, processor.ProcessTaskGroupApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskGroupApplicationIDCardOCR, processor.ProcessTaskGroupApplicationIDCardOCR)

	// 微信发货信息上报任务（合规）
	mux.HandleFunc(TaskUploadShippingInfo, processor.ProcessTaskUploadShippingInfo)

	// 微信投诉单同步任务
	mux.HandleFunc(TaskSyncComplaints, processor.ProcessTaskSyncComplaints)

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Shutdown() {
	processor.server.Shutdown()
}
