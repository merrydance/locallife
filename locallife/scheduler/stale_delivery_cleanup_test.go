package scheduler

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func numericFromFloatScheduler(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestDataCleanupScheduler_CleanupStaleDeliveries_PublishesPoolGoneAfterAutoCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, nil, publisher)

	order := db.Order{ID: 501, OrderNo: "LL501", Status: db.OrderStatusPaid}
	delivery := db.Delivery{
		ID:              601,
		OrderID:         order.ID,
		Status:          "pending",
		PickupLatitude:  numericFromFloatScheduler(30.123456),
		PickupLongitude: numericFromFloatScheduler(120.654321),
		CreatedAt:       time.Now().Add(-2 * time.Hour),
	}

	store.EXPECT().
		ListPendingDeliveriesBefore(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ListPendingDeliveriesBeforeParams) ([]db.Delivery, error) {
			require.Equal(t, "pending", arg.Status)
			require.EqualValues(t, 50, arg.Limit)
			return []db.Delivery{delivery}, nil
		})
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().
		CancelOrderTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CancelOrderTxParams) (db.CancelOrderTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, order.Status, arg.OldStatus)
			return db.CancelOrderTxResult{Order: order}, nil
		})
	store.EXPECT().UpdateDeliveryToCancelled(gomock.Any(), delivery.ID).Return(delivery, nil)
	store.EXPECT().
		GetActiveRecommendConfig(gomock.Any()).
		Return(db.RecommendConfig{MaxDistance: 4500}, nil)
	store.EXPECT().
		ListNearbyRiders(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ListNearbyRidersParams) ([]db.ListNearbyRidersRow, error) {
			require.Equal(t, 30.123456, arg.CenterLat)
			require.Equal(t, 120.654321, arg.CenterLng)
			require.Equal(t, 4500.0, arg.MaxDistance)
			require.Equal(t, staleDeliveryPoolGoneLimitCount, arg.LimitCount)
			return []db.ListNearbyRidersRow{{ID: 701}, {ID: 702}, {ID: 703}, {ID: 704}}, nil
		})
	store.EXPECT().
		GetPaymentOrdersByOrder(gomock.Any(), pgtype.Int8{Int64: order.ID, Valid: true}).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingDeliveriesBefore(gomock.Any(), gomock.Any()).
		Return([]db.Delivery{}, nil)

	s.cleanupStaleDeliveries()

	published := publisher.snapshot()
	require.Len(t, published, 4)

	seen := map[int64]bool{}
	for _, item := range published {
		require.Contains(t, []string{"notification:rider:701", "notification:rider:702", "notification:rider:703", "notification:rider:704"}, item.channel)

		var push websocket.NotificationPushMessage
		require.NoError(t, json.Unmarshal(item.payload, &push))
		require.Equal(t, websocket.EntityRider, push.EntityType)
		require.Equal(t, websocket.MessageTypeDeliveryPoolGone, push.Message.Type)
		seen[push.EntityID] = true

		var data map[string]any
		require.NoError(t, json.Unmarshal(push.Message.Data, &data))
		require.EqualValues(t, order.ID, data["order_id"])
		require.Equal(t, "gone", data["event"])
		require.Equal(t, "scheduler_auto_cancel", data["source"])
	}
	require.True(t, seen[701])
	require.True(t, seen[702])
	require.True(t, seen[703])
	require.True(t, seen[704])
}
