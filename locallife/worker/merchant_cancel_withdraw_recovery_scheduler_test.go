package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type merchantCancelWithdrawRecoveryTestDistributor struct {
	worker.NoopTaskDistributor
	processMerchantCancelWithdrawResult func(ctx context.Context, payload *worker.MerchantCancelWithdrawResultPayload, opts ...asynq.Option) error
}

func (d merchantCancelWithdrawRecoveryTestDistributor) DistributeTaskProcessMerchantCancelWithdrawResult(ctx context.Context, payload *worker.MerchantCancelWithdrawResultPayload, opts ...asynq.Option) error {
	if d.processMerchantCancelWithdrawResult != nil {
		return d.processMerchantCancelWithdrawResult(ctx, payload, opts...)
	}
	return nil
}

func TestMerchantCancelWithdrawRecoverySchedulerRunOnceEnqueuesPendingApplications(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	application := db.MerchantCancelWithdrawApplication{ID: 901, OutRequestNo: "MCW001"}
	distributor := merchantCancelWithdrawRecoveryTestDistributor{
		processMerchantCancelWithdrawResult: func(_ context.Context, payload *worker.MerchantCancelWithdrawResultPayload, _ ...asynq.Option) error {
			require.Equal(t, application.ID, payload.ApplicationID)
			require.Zero(t, payload.RetryCount)
			return nil
		},
	}

	store.EXPECT().
		ListPendingMerchantCancelWithdrawApplications(gomock.Any(), gomock.AssignableToTypeOf(db.ListPendingMerchantCancelWithdrawApplicationsParams{})).
		DoAndReturn(func(_ context.Context, arg db.ListPendingMerchantCancelWithdrawApplicationsParams) ([]db.MerchantCancelWithdrawApplication, error) {
			require.True(t, arg.QueryBefore.Valid)
			require.WithinDuration(t, time.Now().Add(-5*time.Minute), arg.QueryBefore.Time, 5*time.Second)
			require.Equal(t, int32(200), arg.LimitCount)
			return []db.MerchantCancelWithdrawApplication{application}, nil
		})

	scheduler := worker.NewMerchantCancelWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestMerchantCancelWithdrawRecoverySchedulerRunOnceReturnsAfterQueryFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := merchantCancelWithdrawRecoveryTestDistributor{}

	store.EXPECT().
		ListPendingMerchantCancelWithdrawApplications(gomock.Any(), gomock.AssignableToTypeOf(db.ListPendingMerchantCancelWithdrawApplicationsParams{})).
		DoAndReturn(func(_ context.Context, arg db.ListPendingMerchantCancelWithdrawApplicationsParams) ([]db.MerchantCancelWithdrawApplication, error) {
			require.True(t, arg.QueryBefore.Valid)
			require.WithinDuration(t, time.Now().Add(-5*time.Minute), arg.QueryBefore.Time, 5*time.Second)
			require.Equal(t, int32(200), arg.LimitCount)
			return nil, assertAnError("cancel withdraw list unavailable")
		})

	scheduler := worker.NewMerchantCancelWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}
