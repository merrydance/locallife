package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

// TaskDistributor 任务分发接口
type TaskDistributor interface {
	// DistributeTaskPaymentOrderTimeout 分发支付订单超时任务
	DistributeTaskPaymentOrderTimeout(
		ctx context.Context,
		payload *PayloadPaymentOrderTimeout,
		opts ...asynq.Option,
	) error

	// DistributeTaskReservationPaymentTimeout 分发预定支付超时任务
	DistributeTaskReservationPaymentTimeout(
		ctx context.Context,
		payload *PayloadReservationPaymentTimeout,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessPaymentSuccess 分发支付成功处理任务
	DistributeTaskProcessPaymentSuccess(
		ctx context.Context,
		payload *PaymentSuccessPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessRefund 分发发起退款任务
	DistributeTaskProcessRefund(
		ctx context.Context,
		payload *PayloadProcessRefund,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessRefundResult 分发退款结果处理任务
	DistributeTaskProcessRefundResult(
		ctx context.Context,
		payload *RefundResultPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessProfitSharing 分发分账处理任务
	DistributeTaskProcessProfitSharing(
		ctx context.Context,
		payload *ProfitSharingPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessApplymentResult 分发进件结果处理任务
	DistributeTaskProcessApplymentResult(
		ctx context.Context,
		payload *ApplymentResultPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessProfitSharingResult 分发分账结果处理任务
	DistributeTaskProcessProfitSharingResult(
		ctx context.Context,
		payload *ProfitSharingResultPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskSendNotification 分发发送通知任务
	DistributeTaskSendNotification(
		ctx context.Context,
		payload *SendNotificationPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskCheckMerchantForeignObject 分发商户异物索赔检查任务
	DistributeTaskCheckMerchantForeignObject(
		ctx context.Context,
		merchantID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskCheckRiderDamage 分发骑手餐损检查任务
	DistributeTaskCheckRiderDamage(
		ctx context.Context,
		riderID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessAppealResult 分发申诉审核结果处理任务
	DistributeTaskProcessAppealResult(
		ctx context.Context,
		payload *ProcessAppealResultPayload,
		opts ...asynq.Option,
	) error

	// ==================== 商户入驻证照OCR（异步） ====================

	// DistributeTaskMerchantApplicationBusinessLicenseOCR 分发营业执照OCR任务
	DistributeTaskMerchantApplicationBusinessLicenseOCR(
		ctx context.Context,
		applicationID int64,
		imagePath string,
		opts ...asynq.Option,
	) error

	// DistributeTaskMerchantApplicationFoodPermitOCR 分发食品经营许可证OCR任务
	DistributeTaskMerchantApplicationFoodPermitOCR(
		ctx context.Context,
		applicationID int64,
		imagePath string,
		opts ...asynq.Option,
	) error

	// DistributeTaskMerchantApplicationIDCardOCR 分发身份证OCR任务（side: Front/Back）
	DistributeTaskMerchantApplicationIDCardOCR(
		ctx context.Context,
		applicationID int64,
		imagePath string,
		side string,
		opts ...asynq.Option,
	) error
}

type RedisTaskDistributor struct {
	client *asynq.Client
}

func NewRedisTaskDistributor(redisOpt asynq.RedisClientOpt) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
	}
}
