package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskOperatorPendingDispatchAlert_CreatesOperatorNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := &RedisTaskProcessor{store: store, roleCache: map[int64]cachedUserRoles{}}

	delivery := db.Delivery{ID: 101, OrderID: 201, Status: "pending", CreatedAt: time.Now().Add(-4 * time.Minute)}
	order := db.Order{ID: 201, MerchantID: 301}
	merchant := db.Merchant{ID: 301, RegionID: 401}
	recipients := []db.ListActiveOperatorNotificationRecipientsByRegionRow{{
		OperatorID:   1,
		UserID:       501,
		RegionID:     401,
		RegionName:   "测试区域",
		OperatorName: "测试运营商",
	}}

	store.EXPECT().GetDelivery(gomock.Any(), delivery.ID).Return(delivery, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().ListActiveOperatorNotificationRecipientsByRegion(gomock.Any(), merchant.RegionID).Return(recipients, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
		require.Equal(t, recipients[0].UserID, arg.UserID)
		require.Equal(t, "delivery", arg.Type)
		require.Equal(t, "待接单提醒", arg.Title)
		var extra map[string]any
		require.NoError(t, json.Unmarshal(arg.ExtraData, &extra))
		require.Equal(t, "operator", extra["audience"])
		require.Equal(t, "dispatch_timeout", extra["category"])
		require.EqualValues(t, 401, extra["region_id"])
		return db.Notification{ID: 9001, UserID: arg.UserID, Type: arg.Type, Title: arg.Title, Content: arg.Content, CreatedAt: time.Now()}, nil
	})
	store.EXPECT().ListUserRoles(gomock.Any(), recipients[0].UserID).Return([]db.UserRole{}, nil)

	payloadBytes, err := json.Marshal(&OperatorPendingDispatchAlertPayload{DeliveryID: delivery.ID, AlertKey: "pending_dispatch_3m", ThresholdMinutes: 3})
	require.NoError(t, err)

	err = processor.ProcessTaskOperatorPendingDispatchAlert(context.Background(), asynq.NewTask(TaskOperatorPendingDispatchAlert, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskOperatorPendingDispatchAlert_SkipsBeforeThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := &RedisTaskProcessor{store: store, roleCache: map[int64]cachedUserRoles{}}

	delivery := db.Delivery{ID: 102, OrderID: 202, Status: "pending", CreatedAt: time.Now().Add(-1 * time.Minute)}
	store.EXPECT().GetDelivery(gomock.Any(), delivery.ID).Return(delivery, nil)

	payloadBytes, err := json.Marshal(&OperatorPendingDispatchAlertPayload{DeliveryID: delivery.ID, AlertKey: "pending_dispatch_3m", ThresholdMinutes: 3})
	require.NoError(t, err)

	err = processor.ProcessTaskOperatorPendingDispatchAlert(context.Background(), asynq.NewTask(TaskOperatorPendingDispatchAlert, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskOperatorPendingDispatchAlert_SkipsWhenNoActiveOperatorRecipients(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := &RedisTaskProcessor{store: store, roleCache: map[int64]cachedUserRoles{}}

	delivery := db.Delivery{ID: 103, OrderID: 203, Status: "pending", CreatedAt: time.Now().Add(-5 * time.Minute)}
	order := db.Order{ID: 203, MerchantID: 303}
	merchant := db.Merchant{ID: 303, RegionID: 403}

	store.EXPECT().GetDelivery(gomock.Any(), delivery.ID).Return(delivery, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().ListActiveOperatorNotificationRecipientsByRegion(gomock.Any(), merchant.RegionID).Return([]db.ListActiveOperatorNotificationRecipientsByRegionRow{}, nil)

	payloadBytes, err := json.Marshal(&OperatorPendingDispatchAlertPayload{DeliveryID: delivery.ID, AlertKey: "pending_dispatch_3m", ThresholdMinutes: 3})
	require.NoError(t, err)

	err = processor.ProcessTaskOperatorPendingDispatchAlert(context.Background(), asynq.NewTask(TaskOperatorPendingDispatchAlert, payloadBytes))
	require.NoError(t, err)
}
