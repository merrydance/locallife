package worker

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
)

// NoopTaskDistributor is a safe fallback when Redis is disabled.
// Most tasks are dropped, but flows with inline fallback or explicit retry
// semantics must return an error so callers can degrade safely.
type NoopTaskDistributor struct{}

func financialTaskDistributorUnavailable(taskName string) error {
	return errors.New(taskName + " task distributor unavailable without redis")
}

func NewNoopTaskDistributor() TaskDistributor {
	return NoopTaskDistributor{}
}

func (NoopTaskDistributor) DistributeTaskPaymentOrderTimeout(ctx context.Context, payload *PayloadPaymentOrderTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskCombinedPaymentOrderTimeout(ctx context.Context, payload *PayloadCombinedPaymentOrderTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskReservationPaymentTimeout(ctx context.Context, payload *PayloadReservationPaymentTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskOrderPaymentTimeout(ctx context.Context, payload *PayloadOrderPaymentTimeout, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskReservationNoShowAlert(ctx context.Context, payload *PayloadReservationNoShowAlert, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskReservationFoodSafetyAlert(ctx context.Context, payload *PayloadReservationFoodSafetyAlert, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessPaymentSuccess(ctx context.Context, payload *PaymentSuccessPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("payment success")
}

func (NoopTaskDistributor) DistributeTaskProcessRefund(ctx context.Context, payload *PayloadProcessRefund, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("refund")
}

func (NoopTaskDistributor) DistributeTaskProcessRefundResult(ctx context.Context, payload *RefundResultPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("refund result")
}

func (NoopTaskDistributor) DistributeTaskProcessProfitSharing(ctx context.Context, payload *ProfitSharingPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("profit sharing")
}

func (NoopTaskDistributor) DistributeTaskProcessApplymentResult(ctx context.Context, payload *ApplymentResultPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessProfitSharingResult(ctx context.Context, payload *ProfitSharingResultPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("profit sharing result")
}

func (NoopTaskDistributor) DistributeTaskProcessProfitSharingReturnResult(ctx context.Context, payload *ProfitSharingReturnResultPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("profit sharing return result")
}

func (NoopTaskDistributor) DistributeTaskProcessMerchantWithdrawResult(ctx context.Context, payload *MerchantWithdrawResultPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("merchant withdraw result")
}

func (NoopTaskDistributor) DistributeTaskProcessMerchantCancelWithdrawResult(ctx context.Context, payload *MerchantCancelWithdrawResultPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("merchant cancel withdraw result")
}

func (NoopTaskDistributor) DistributeTaskSendNotification(ctx context.Context, payload *SendNotificationPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskOperatorPendingDispatchAlert(ctx context.Context, payload *OperatorPendingDispatchAlertPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskCheckMerchantForeignObject(ctx context.Context, merchantID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskCheckRiderDamage(ctx context.Context, riderID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessRecoveryDisputeResult(ctx context.Context, payload *ProcessRecoveryDisputeResultPayload, opts ...asynq.Option) error {
	return errors.New("recovery dispute result task distributor unavailable without redis")
}

func (NoopTaskDistributor) DistributeTaskAutomaticRecoveryDisputeResolution(ctx context.Context, payload *AutomaticRecoveryDisputeResolutionPayload, opts ...asynq.Option) error {
	return errors.New("automatic recovery dispute resolution task distributor unavailable without redis")
}

func (NoopTaskDistributor) DistributeTaskClaimPayout(ctx context.Context, payload *ClaimPayoutPayload, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("claim payout")
}

func (NoopTaskDistributor) DistributeTaskClaimBehaviorAction(ctx context.Context, payload *ClaimBehaviorActionPayload, opts ...asynq.Option) error {
	return errors.New("claim behavior action task distributor unavailable without redis")
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskMerchantApplicationIDCardOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, side string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskOperatorApplicationBusinessLicenseOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskOperatorApplicationIDCardOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, side string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskRiderApplicationIDCardOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, side string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskRiderApplicationHealthCertOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskGroupApplicationBusinessLicenseOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskGroupApplicationIDCardOCR(ctx context.Context, applicationID int64, mediaAssetID int64, ocrJobID int64, side string, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskUploadShippingInfo(ctx context.Context, payload *UploadShippingInfoPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskSyncComplaints(ctx context.Context, payload *SyncComplaintsPayload, opts ...asynq.Option) error {
	return nil
}

func (NoopTaskDistributor) DistributeTaskProcessAnomalyRefund(ctx context.Context, payload *PayloadProcessAnomalyRefund, opts ...asynq.Option) error {
	return financialTaskDistributorUnavailable("anomaly refund")
}

func (NoopTaskDistributor) DistributeTaskPrintOrder(ctx context.Context, payload *PrintOrderPayload, opts ...asynq.Option) error {
	return nil
}
