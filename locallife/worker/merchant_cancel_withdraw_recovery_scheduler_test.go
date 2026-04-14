package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMerchantCancelWithdrawRecoverySchedulerRunOnceEnqueuesPendingApplications(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	application := db.MerchantCancelWithdrawApplication{ID: 901, OutRequestNo: "MCW001"}

	store.EXPECT().
		ListPendingMerchantCancelWithdrawApplications(gomock.Any(), int32(200)).
		Return([]db.MerchantCancelWithdrawApplication{application}, nil)

	distributor.EXPECT().
		DistributeTaskProcessMerchantCancelWithdrawResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.MerchantCancelWithdrawResultPayload{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.MerchantCancelWithdrawResultPayload, _ ...asynq.Option) error {
			require.Equal(t, application.ID, payload.ApplicationID)
			require.Zero(t, payload.RetryCount)
			return nil
		})

	scheduler := worker.NewMerchantCancelWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestMerchantCancelWithdrawRecoverySchedulerRunOnceReturnsAfterQueryFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		ListPendingMerchantCancelWithdrawApplications(gomock.Any(), int32(200)).
		Return(nil, assertAnError("cancel withdraw list unavailable"))

	scheduler := worker.NewMerchantCancelWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}