package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProfitSharingReceiverLifecycleSchedulerRunOnceEnqueuesOperatorAndRiderTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingReceiverLifecycleSchedulerTestDistributor{}
	operatorTarget := db.ProfitSharingReceiverTarget{
		ID:        801,
		OwnerType: db.ProfitSharingReceiverOwnerTypeOperator,
		OwnerID:   701,
	}
	riderTarget := db.ProfitSharingReceiverTarget{
		ID:        802,
		OwnerType: db.ProfitSharingReceiverOwnerTypeRider,
		OwnerID:   702,
	}

	gomock.InOrder(
		store.EXPECT().ListRetryableProfitSharingReceiverTargetsByOwnerType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListRetryableProfitSharingReceiverTargetsByOwnerTypeParams) ([]db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, arg.OwnerType)
			require.True(t, arg.NowAt.Valid)
			require.Greater(t, arg.LimitCount, int32(0))
			return []db.ProfitSharingReceiverTarget{operatorTarget}, nil
		}),
		store.EXPECT().ListRetryableProfitSharingReceiverTargetsByOwnerType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListRetryableProfitSharingReceiverTargetsByOwnerTypeParams) ([]db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, db.ProfitSharingReceiverOwnerTypeRider, arg.OwnerType)
			require.True(t, arg.NowAt.Valid)
			require.Greater(t, arg.LimitCount, int32(0))
			return []db.ProfitSharingReceiverTarget{riderTarget}, nil
		}),
	)

	scheduler := worker.NewProfitSharingReceiverLifecycleScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{operatorTarget.ID, riderTarget.ID}, distributor.targetIDs)
	require.Len(t, distributor.optionCounts, 2)
	require.GreaterOrEqual(t, distributor.optionCounts[0], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[1], 3)
}

type profitSharingReceiverLifecycleSchedulerTestDistributor struct {
	worker.NoopTaskDistributor
	targetIDs    []int64
	optionCounts []int
}

func (d *profitSharingReceiverLifecycleSchedulerTestDistributor) DistributeTaskProcessProfitSharingReceiverTarget(ctx context.Context, payload *worker.ProfitSharingReceiverTargetPayload, opts ...asynq.Option) error {
	d.targetIDs = append(d.targetIDs, payload.TargetID)
	d.optionCounts = append(d.optionCounts, len(opts))
	return nil
}
