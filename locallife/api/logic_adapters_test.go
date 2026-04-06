package api

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAPITaskSchedulerScheduleProfitSharing_SkipsTakeoutUntilSettlement(t *testing.T) {
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

func TestAPITaskSchedulerScheduleProfitSharing_SkipsNonProfitSharingOrderTypes(t *testing.T) {
	testCases := []struct {
		name      string
		orderID   int64
		orderType string
	}{
		{name: "DineIn", orderID: 78, orderType: "dine_in"},
		{name: "Takeaway", orderID: 79, orderType: "takeaway"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			distributor := mockwk.NewMockTaskDistributor(ctrl)

			store.EXPECT().
				GetOrder(gomock.Any(), tc.orderID).
				Return(db.Order{ID: tc.orderID, OrderType: tc.orderType}, nil)

			scheduler := apiTaskScheduler{server: &Server{store: store, taskDistributor: distributor}}
			err := scheduler.ScheduleProfitSharing(context.Background(), 21, tc.orderID)
			require.NoError(t, err)
		})
	}
}

func TestAPITaskSchedulerScheduleProfitSharing_DistributesReservationLinkedDineInOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(81)).
		Return(db.Order{ID: 81, OrderType: "dine_in", ReservationID: pgtype.Int8{Int64: 901, Valid: true}}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{})).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(21), payload.PaymentOrderID)
			require.Equal(t, int64(81), payload.OrderID)
			return nil
		})

	scheduler := apiTaskScheduler{server: &Server{store: store, taskDistributor: distributor}}
	err := scheduler.ScheduleProfitSharing(context.Background(), 21, 81)
	require.NoError(t, err)
}

func TestAPITaskSchedulerScheduleProfitSharing_DistributesReservationOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), int64(80)).
		Return(db.Order{ID: 80, OrderType: db.OrderTypeReservation}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{})).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(21), payload.PaymentOrderID)
			require.Equal(t, int64(80), payload.OrderID)
			return nil
		})

	scheduler := apiTaskScheduler{server: &Server{store: store, taskDistributor: distributor}}
	err := scheduler.ScheduleProfitSharing(context.Background(), 21, 80)
	require.NoError(t, err)
}
