package worker

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
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
	// ProcessTaskPaymentSuccess 处理支付成功任务
	ProcessTaskPaymentSuccess(ctx context.Context, task *asynq.Task) error
	// ProcessTaskRefundResult 处理退款结果任务
	ProcessTaskRefundResult(ctx context.Context, task *asynq.Task) error
	// ProcessTaskProfitSharing 处理分账任务
	ProcessTaskProfitSharing(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server          *asynq.Server
	store           db.Store
	distributor     TaskDistributor                 // 用于在任务中分发后续任务
	wechatClient    wechat.WechatClient             // 微信小程序客户端（用于证照OCR等）
	ecommerceClient wechat.EcommerceClientInterface // 平台收付通客户端（分账）
	redisClient     *redis.Client                   // Redis客户端（用于Pub/Sub推送通知）
}

func NewRedisTaskProcessor(
	redisOpt asynq.RedisClientOpt,
	store db.Store,
	distributor TaskDistributor,
	wechatClient wechat.WechatClient,
	ecommerceClient wechat.EcommerceClientInterface,
) TaskProcessor {
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
				log.Error().Err(err).Str("type", task.Type()).
					Bytes("payload", task.Payload()).Msg("process task failed")
			}),
			Logger:          logger,
			ShutdownTimeout: 10 * time.Second,
		},
	)

	// 创建Redis客户端（用于Pub/Sub）
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisOpt.Addr,
		Password: redisOpt.Password,
		DB:   redisOpt.DB,
	})

	return &RedisTaskProcessor{
		server:          server,
		store:           store,
		distributor:     distributor,
		wechatClient:    wechatClient,
		ecommerceClient: ecommerceClient,
		redisClient:     redisClient,
	}
}

// NewTestTaskProcessor 创建用于测试的处理器实例（不需要Redis连接）
func NewTestTaskProcessor(
	store db.Store,
	distributor TaskDistributor,
	wechatClient wechat.WechatClient,
	ecommerceClient wechat.EcommerceClientInterface,
) *RedisTaskProcessor {
	return &RedisTaskProcessor{
		store:           store,
		distributor:     distributor,
		wechatClient:    wechatClient,
		ecommerceClient: ecommerceClient,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	// 注册任务处理器
	mux.HandleFunc(TaskPaymentOrderTimeout, processor.ProcessTaskPaymentOrderTimeout)
	mux.HandleFunc(TaskReservationPaymentTimeout, processor.ProcessTaskReservationPaymentTimeout)
	mux.HandleFunc(TaskProcessPaymentSuccess, processor.ProcessTaskPaymentSuccess)
	mux.HandleFunc(TaskProcessRefund, processor.ProcessTaskInitiateRefund)
	mux.HandleFunc(TaskProcessRefundResult, processor.ProcessTaskRefundResult)
	mux.HandleFunc(TaskProcessProfitSharing, processor.ProcessTaskProfitSharing)
	mux.HandleFunc(TaskSendNotification, processor.ProcessTaskSendNotification)

	// TrustScore系统任务
	mux.HandleFunc(TypeHandleSuspiciousPattern, processor.HandleSuspiciousPattern)
	mux.HandleFunc(TypeCheckMerchantForeignObject, processor.HandleCheckMerchantForeignObject)
	mux.HandleFunc(TypeCheckRiderDamage, processor.HandleCheckRiderDamage)

	// 申诉处理任务
	mux.HandleFunc(TaskProcessAppealResult, processor.ProcessTaskProcessAppealResult)

	// 进件/分账结果处理任务
	mux.HandleFunc(TaskProcessApplymentResult, processor.ProcessTaskApplymentResult)
	mux.HandleFunc(TaskProcessProfitSharingResult, processor.ProcessTaskProfitSharingResult)

	// 商户入驻证照OCR任务
	mux.HandleFunc(TaskMerchantApplicationBusinessLicenseOCR, processor.ProcessTaskMerchantApplicationBusinessLicenseOCR)
	mux.HandleFunc(TaskMerchantApplicationFoodPermitOCR, processor.ProcessTaskMerchantApplicationFoodPermitOCR)
	mux.HandleFunc(TaskMerchantApplicationIDCardOCR, processor.ProcessTaskMerchantApplicationIDCardOCR)

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Shutdown() {
	processor.server.Shutdown()
}
