package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type profitSharingFactApplicationEnqueueRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
}

func (r *profitSharingFactApplicationEnqueueRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

func TestProcessTaskRefundResult_RiderDepositReturnsSkipRetryBecauseMovedToFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)

	refundOrder := db.RefundOrder{ID: 702, PaymentOrderID: 551, OutRefundNo: "REFUND_702", RefundAmount: 30000}
	paymentOrder := db.PaymentOrder{ID: 551, UserID: 77, Amount: 30000, BusinessType: "rider_deposit"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)

	payload, err := json.Marshal(worker.RefundResultPayload{OutRefundNo: refundOrder.OutRefundNo, RefundStatus: "SUCCESS", RefundID: "WX_REFUND_702"})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(worker.TaskProcessRefundResult, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "rider deposit refund results must be applied via payment fact application")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}
