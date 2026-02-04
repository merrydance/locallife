package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTakeoutAutoCompleteScheduler_AutoCompletesWithoutClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	now := time.Now()
	order := db.Order{
		ID:        101,
		UserID:    11,
		Status:    "rider_delivered",
		CreatedAt: now.Add(-2 * time.Hour),
	}

	store.EXPECT().ListTakeoutOrdersDeliveredBefore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ListTakeoutOrdersDeliveredBeforeParams) ([]db.Order, error) {
			require.True(t, arg.DeliveredBefore.Valid)
			require.Equal(t, int32(100), arg.Limit)
			return []db.Order{order}, nil
		},
	)

	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), db.ListUserClaimsInPeriodParams{
		UserID:    order.UserID,
		CreatedAt: order.CreatedAt,
	}).Return([]db.Claim{}, nil)

	updated := order
	updated.Status = "completed"
	store.EXPECT().AutoCompleteTakeoutOrder(gomock.Any(), order.ID).Return(updated, nil)

	store.EXPECT().CreateOrderStatusLog(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.CreateOrderStatusLogParams) (db.OrderStatusLog, error) {
			require.Equal(t, updated.ID, arg.OrderID)
			require.True(t, arg.FromStatus.Valid)
			require.Equal(t, order.Status, arg.FromStatus.String)
			require.Equal(t, "completed", arg.ToStatus)
			require.True(t, arg.OperatorType.Valid)
			require.Equal(t, "system", arg.OperatorType.String)
			require.True(t, arg.Notes.Valid)
			return db.OrderStatusLog{}, nil
		},
	)

	po := db.PaymentOrder{ID: 9001, Status: "paid", PaymentType: "profit_sharing"}
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: "order",
	}).Return(po, nil)

	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *worker.ProfitSharingPayload, opts ...asynq.Option) error {
			require.Equal(t, po.ID, payload.PaymentOrderID)
			require.Equal(t, order.ID, payload.OrderID)
			return nil
		},
	)

	s := NewTakeoutAutoCompleteScheduler(store, distributor)
	s.autoCompleteTakeoutOrders()
}

func TestTakeoutAutoCompleteScheduler_SkipsWhenHasClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	now := time.Now()
	order := db.Order{
		ID:        202,
		UserID:    22,
		Status:    "rider_delivered",
		CreatedAt: now.Add(-2 * time.Hour),
	}

	store.EXPECT().ListTakeoutOrdersDeliveredBefore(gomock.Any(), gomock.Any()).Return([]db.Order{order}, nil)

	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), db.ListUserClaimsInPeriodParams{
		UserID:    order.UserID,
		CreatedAt: order.CreatedAt,
	}).Return([]db.Claim{{ID: 1, OrderID: order.ID, UserID: order.UserID, Status: "pending"}}, nil)

	// If it tries to complete or enqueue profit sharing, gomock will fail due to missing expectations.
	s := NewTakeoutAutoCompleteScheduler(store, distributor)
	s.autoCompleteTakeoutOrders()
}
