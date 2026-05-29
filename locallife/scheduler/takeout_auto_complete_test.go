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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type takeoutAutoCompleteDistributor struct {
	worker.NoopTaskDistributor
	profitSharingOrderIDs []int64
}

func (d *takeoutAutoCompleteDistributor) DistributeTaskProcessBaofuProfitSharing(ctx context.Context, payload *worker.BaofuProfitSharingPayload, opts ...asynq.Option) error {
	d.profitSharingOrderIDs = append(d.profitSharingOrderIDs, payload.ProfitSharingOrderID)
	return nil
}

func TestTakeoutAutoCompleteScheduler_AutoCompletesWithoutClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &takeoutAutoCompleteDistributor{}

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
	paymentOrder := db.PaymentOrder{
		ID:                    301,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		Amount:                1000,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                401,
		PaymentOrderID:    paymentOrder.ID,
		Provider:          db.ExternalPaymentProviderBaofu,
		Channel:           db.PaymentChannelBaofuAggregate,
		Status:            db.ProfitSharingOrderStatusPending,
		OrderSource:       db.OrderTypeTakeout,
		TotalAmount:       paymentOrder.Amount,
		DeliveryFee:       500,
		RiderID:           pgtype.Int8{Int64: 501, Valid: true},
		RiderSharingMerID: pgtype.Text{String: "RIDER_SHARE", Valid: true},
		RiderGrossAmount:  500,
		RiderAmount:       490,
	}
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: updated.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetTotalActiveRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)

	s := NewTakeoutAutoCompleteScheduler(store, distributor)
	s.autoCompleteTakeoutOrders()

	require.Equal(t, []int64{profitSharingOrder.ID}, distributor.profitSharingOrderIDs)
}

func TestTakeoutAutoCompleteScheduler_SkipsWhenHasClaim(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &takeoutAutoCompleteDistributor{}

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
