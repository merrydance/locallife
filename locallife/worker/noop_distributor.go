package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

// NoopTaskDistributor is a safe fallback when Redis is disabled.
// It drops tasks and returns nil to avoid blocking core flows.
type NoopTaskDistributor struct{}

func NewNoopTaskDistributor() TaskDistributor {
	return NoopTaskDistributor{}
}

func (NoopTaskDistributor) DistributeTaskPaymentOrderTimeout(ctx context.Context, payload *PayloadPaymentOrderTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskReservationPaymentTimeout(ctx context.Context, payload *PayloadReservationPaymentTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessPaymentSuccess(ctx context.Context, payload *PaymentSuccessPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessRefund(ctx context.Context, payload *PayloadProcessRefund, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessRefundResult(ctx context.Context, payload *RefundResultPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessProfitSharing(ctx context.Context, payload *ProfitSharingPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessApplymentResult(ctx context.Context, payload *ApplymentResultPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessProfitSharingResult(ctx context.Context, payload *ProfitSharingResultPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskSendNotification(ctx context.Context, payload *SendNotificationPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskCheckMerchantForeignObject(ctx context.Context, merchantID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskCheckRiderDamage(ctx context.Context, riderID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessAppealResult(ctx context.Context, payload *ProcessAppealResultPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(ctx context.Context, applicationID int64, imagePath string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(ctx context.Context, applicationID int64, imagePath string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationIDCardOCR(ctx context.Context, applicationID int64, imagePath string, side string, opts ...asynq.Option) error {
	return nil
}
