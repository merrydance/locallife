package api

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

func TestAPITaskSchedulerScheduleProfitSharing_SkipsTakeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(77)).
		Return(db.Order{ID: 77, OrderType: db.OrderTypeTakeout}, nil)

	scheduler := apiTaskScheduler{server: &Server{store: store, taskDistributor: distributor}}
	err := scheduler.ScheduleProfitSharing(context.Background(), 21, 77)
	require.NoError(t, err)
}

func TestAPITaskSchedulerScheduleProfitSharing_DistributesNonTakeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(78)).
		Return(db.Order{ID: 78, OrderType: "takeaway"}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{})).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(21), payload.PaymentOrderID)
			require.Equal(t, int64(78), payload.OrderID)
			return nil
		})

	scheduler := apiTaskScheduler{server: &Server{store: store, taskDistributor: distributor}}
	err := scheduler.ScheduleProfitSharing(context.Background(), 21, 78)
	require.NoError(t, err)
}
