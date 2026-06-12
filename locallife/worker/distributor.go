package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
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

	// DistributeTaskOrderPaymentTimeout 分发订单支付超时任务
	DistributeTaskOrderPaymentTimeout(
		ctx context.Context,
		payload *PayloadOrderPaymentTimeout,
		opts ...asynq.Option,
	) error

	// DistributeTaskReservationNoShowAlert 分发预定未到店提醒任务
	DistributeTaskReservationNoShowAlert(
		ctx context.Context,
		payload *PayloadReservationNoShowAlert,
		opts ...asynq.Option,
	) error

	// DistributeTaskReservationFoodSafetyAlert 分发食安停业预订提醒任务
	DistributeTaskReservationFoodSafetyAlert(
		ctx context.Context,
		payload *PayloadReservationFoodSafetyAlert,
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

	// DistributeTaskProcessBaofuProfitSharing 分发宝付确认分账任务
	DistributeTaskProcessBaofuProfitSharing(
		ctx context.Context,
		payload *BaofuProfitSharingPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessBaofuWithdrawalFactApplication 分发宝付提现查询/通知结果应用任务
	DistributeTaskProcessBaofuWithdrawalFactApplication(
		ctx context.Context,
		payload *BaofuWithdrawalFactApplicationPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessBaofuWithdrawalCommandDispatch 分发宝付提现提交命令派发任务
	DistributeTaskProcessBaofuWithdrawalCommandDispatch(
		ctx context.Context,
		payload *BaofuWithdrawalCommandDispatchPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskSendNotification 分发发送通知任务
	DistributeTaskSendNotification(
		ctx context.Context,
		payload *SendNotificationPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskOperatorPendingDispatchAlert 分发运营商待接单超时提醒任务
	DistributeTaskOperatorPendingDispatchAlert(
		ctx context.Context,
		payload *OperatorPendingDispatchAlertPayload,
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

	// DistributeTaskProcessRecoveryDisputeResult 分发追偿争议审核结果处理任务
	DistributeTaskProcessRecoveryDisputeResult(
		ctx context.Context,
		payload *ProcessRecoveryDisputeResultPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskAutomaticRecoveryDisputeResolution 分发追偿争议自动复核重试任务
	DistributeTaskAutomaticRecoveryDisputeResolution(
		ctx context.Context,
		payload *AutomaticRecoveryDisputeResolutionPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskClaimPayout 分发索赔平台赔付任务
	DistributeTaskClaimPayout(
		ctx context.Context,
		payload *ClaimPayoutPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskClaimBehaviorAction 分发索赔行为动作执行任务
	DistributeTaskClaimBehaviorAction(
		ctx context.Context,
		payload *ClaimBehaviorActionPayload,
		opts ...asynq.Option,
	) error

	// ==================== 商户入驻证照OCR（异步） ====================

	// DistributeTaskMerchantApplicationBusinessLicenseOCR 分发营业执照OCR任务
	DistributeTaskMerchantApplicationBusinessLicenseOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskMerchantApplicationFoodPermitOCR 分发食品经营许可证OCR任务
	DistributeTaskMerchantApplicationFoodPermitOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskMerchantApplicationIDCardOCR 分发身份证OCR任务（side: Front/Back）
	DistributeTaskMerchantApplicationIDCardOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		side string,
		opts ...asynq.Option,
	) error

	// DistributeTaskOperatorApplicationBusinessLicenseOCR 分发运营商营业执照 OCR 任务
	DistributeTaskOperatorApplicationBusinessLicenseOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskOperatorApplicationIDCardOCR 分发运营商身份证 OCR 任务
	DistributeTaskOperatorApplicationIDCardOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		side string,
		opts ...asynq.Option,
	) error

	// DistributeTaskRiderApplicationIDCardOCR 分发骑手身份证 OCR 任务
	DistributeTaskRiderApplicationIDCardOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		side string,
		opts ...asynq.Option,
	) error

	// DistributeTaskRiderApplicationHealthCertOCR 分发骑手健康证 OCR 任务
	DistributeTaskRiderApplicationHealthCertOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskOnboardingReview 分发入驻审核任务
	DistributeTaskOnboardingReview(
		ctx context.Context,
		payload *OnboardingReviewPayload,
		opts ...asynq.Option,
	) error

	// DistributeTaskGroupApplicationBusinessLicenseOCR 分发集团营业执照 OCR 任务
	DistributeTaskGroupApplicationBusinessLicenseOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		opts ...asynq.Option,
	) error

	// DistributeTaskGroupApplicationIDCardOCR 分发集团身份证 OCR 任务
	DistributeTaskGroupApplicationIDCardOCR(
		ctx context.Context,
		applicationID int64,
		mediaAssetID int64,
		ocrJobID int64,
		side string,
		opts ...asynq.Option,
	) error

	// DistributeTaskProcessAnomalyRefund 分发已关闭/失败订单异常退款任务
	DistributeTaskProcessAnomalyRefund(
		ctx context.Context,
		payload *PayloadProcessAnomalyRefund,
		opts ...asynq.Option,
	) error

	// DistributeTaskPrintOrder 分发订单打印任务
	DistributeTaskPrintOrder(
		ctx context.Context,
		payload *PrintOrderPayload,
		opts ...asynq.Option,
	) error
}

type RedisTaskDistributor struct {
	client taskEnqueueClient
}

type taskEnqueueClient interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value("request_id"); value != nil {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func (distributor *RedisTaskDistributor) enqueueTask(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := distributor.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_type", task.Type()).
			Str("request_id", requestIDFromContext(ctx)).
			Msg("enqueue task failed")
		return nil, err
	}

	log.Info().
		Str("task_type", task.Type()).
		Str("queue", info.Queue).
		Str("request_id", requestIDFromContext(ctx)).
		Msg("task enqueued")

	return info, nil
}

func NewRedisTaskDistributor(redisOpt asynq.RedisClientOpt) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
	}
}
