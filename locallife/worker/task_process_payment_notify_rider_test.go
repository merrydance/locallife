package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type publishedMessageRecord struct {
	channel string
	payload []byte
}

type recordingPublisher struct {
	records []publishedMessageRecord
}

func (p *recordingPublisher) Publish(_ context.Context, channel string, payload []byte) error {
	p.records = append(p.records, publishedMessageRecord{
		channel: channel,
		payload: append([]byte(nil), payload...),
	})
	return nil
}

func TestNotifyRidersNewDelivery_PublishesStructuredPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.pubSubPublisher = publisher

	order := db.Order{ID: 101, MerchantID: 201, DeliveryFee: riderHighValueDeliveryFeeThreshold, OrderType: "takeout"}
	merchant := db.Merchant{ID: 201, Name: "测试商户"}
	delivery := &db.Delivery{
		ID:                  301,
		PickupAddress:       "取餐点A",
		DeliveryAddress:     "送达点B",
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC), Valid: true},
	}
	poolItem := &db.DeliveryPool{
		ID:               401,
		Distance:         1800,
		Priority:         2,
		ExpectedPickupAt: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		CreatedAt:        time.Date(2026, 4, 26, 11, 50, 0, 0, time.UTC),
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().ListNearbyRiders(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListNearbyRidersParams) ([]db.ListNearbyRidersRow, error) {
		require.Equal(t, riderDeliverySearchStartDistanceM, arg.MaxDistance)
		return []db.ListNearbyRidersRow{{ID: 11}, {ID: 22}, {ID: 33}}, nil
	})

	processor.notifyRidersNewDelivery(context.Background(), order, delivery, poolItem)

	require.Len(t, publisher.records, 3)
	for index, riderID := range []int64{11, 22, 33} {
		require.Equal(t, fmt.Sprintf("%s%d", riderNotificationChannelPrefix, riderID), publisher.records[index].channel)

		var pushMsg websocket.NotificationPushMessage
		require.NoError(t, json.Unmarshal(publisher.records[index].payload, &pushMsg))
		require.Equal(t, riderNotificationEntityType, pushMsg.EntityType)
		require.Equal(t, riderID, pushMsg.EntityID)
		require.Equal(t, riderDeliveryPoolUpdateMessageType, pushMsg.Message.Type)

		var payload riderDeliveryOrderNotificationPayload
		require.NoError(t, json.Unmarshal(pushMsg.Message.Data, &payload))
		require.Equal(t, riderNewDeliveryOrderPayloadType, payload.Type)
		require.Equal(t, order.ID, payload.OrderID)
		require.Equal(t, delivery.ID, payload.DeliveryID)
		require.Equal(t, merchant.ID, payload.MerchantID)
		require.Equal(t, merchant.Name, payload.MerchantName)
		require.Equal(t, delivery.PickupAddress, payload.PickupAddress)
		require.Equal(t, delivery.DeliveryAddress, payload.DeliveryAddress)
		require.Equal(t, order.DeliveryFee, payload.DeliveryFee)
		require.Equal(t, poolItem.Distance, payload.Distance)
		require.Equal(t, poolItem.Priority, payload.Priority)
		require.Equal(t, poolItem.ExpectedPickupAt, payload.ExpectedPickupAt)
		require.Equal(t, delivery.EstimatedDeliveryAt.Time, payload.ExpectedDeliveryAt)
		require.Equal(t, poolItem.CreatedAt, payload.CreatedAt)
		require.True(t, payload.IsHighValue)
	}
}
