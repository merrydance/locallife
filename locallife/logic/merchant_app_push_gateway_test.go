package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type recordingMerchantAppPushProvider struct {
	err      error
	targets  []MerchantAppPushTarget
	messages []MerchantAppPushMessage
}

func (p *recordingMerchantAppPushProvider) Send(_ context.Context, target MerchantAppPushTarget, message MerchantAppPushMessage) error {
	p.targets = append(p.targets, target)
	p.messages = append(p.messages, message)
	return p.err
}

func TestMerchantAppPushDispatcherDispatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	xiaomi := &recordingMerchantAppPushProvider{}
	vivo := &recordingMerchantAppPushProvider{err: NewRetryableMerchantAppPushError(errors.New("vivo timeout"))}
	huawei := &recordingMerchantAppPushProvider{err: NewPermanentMerchantAppPushError(errors.New("huawei invalid token"))}
	dispatcher := NewMerchantAppPushDispatcher(store, StaticMerchantAppPushProviderRegistry{
		db.MerchantAppDeviceProviderXiaomi: xiaomi,
		db.MerchantAppDeviceProviderVivo:   vivo,
		db.MerchantAppDeviceProviderHuawei: huawei,
	})

	payload := MerchantAppNotificationPayload{
		MessageID: "merchant:new_order:501",
		Event:     MerchantNotificationEventNewOrder,
		OrderID:   501,
		OrderNo:   "ORD501",
		Title:     "新订单",
		Content:   "您有一笔新订单 ORD501，请及时处理",
		Amount:    8800,
		ShopName:  "测试商户",
	}

	store.EXPECT().ListActiveMerchantAppDevicesByMerchant(gomock.Any(), int64(601)).Return([]db.MerchantAppDevice{
		{ID: 11, DeviceID: "device-xiaomi", Provider: db.MerchantAppDeviceProviderXiaomi, PushToken: "token-xiaomi"},
		{ID: 12, DeviceID: "device-vivo", Provider: db.MerchantAppDeviceProviderVivo, PushToken: "token-vivo"},
		{ID: 13, DeviceID: "device-unknown", Provider: db.MerchantAppDeviceProviderUnknown, PushToken: "token-unknown"},
		{ID: 14, DeviceID: "device-huawei", Provider: db.MerchantAppDeviceProviderHuawei, PushToken: "token-huawei"},
	}, nil)
	store.EXPECT().
		RecordMerchantAppDevicePermanentPushFailure(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.RecordMerchantAppDevicePermanentPushFailureParams) (int64, error) {
			require.Equal(t, int64(14), arg.ID)
			require.Equal(t, "token-huawei", arg.PushToken)
			require.Equal(t, pgtype.Text{String: "huawei invalid token", Valid: true}, arg.LastPushFailureReason)
			require.Equal(t, int32(3), arg.DeactivateAfterCount)
			return int64(1), nil
		})

	result, err := dispatcher.Dispatch(context.Background(), MerchantAppPushDispatchInput{MerchantID: 601, Payload: payload})
	require.NoError(t, err)
	require.Equal(t, 3, result.Attempted)
	require.Equal(t, 1, result.Sent)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, result.RetryableFailures)
	require.Equal(t, 1, result.PermanentFailures)
	require.Len(t, result.DeviceResultSummaries, 4)

	require.Len(t, xiaomi.targets, 1)
	require.Equal(t, "token-xiaomi", xiaomi.targets[0].PushToken)
	require.Equal(t, payload.MessageID, xiaomi.messages[0].MessageID)
	require.Equal(t, payload, xiaomi.messages[0].Data)
	require.True(t, result.DeviceResultSummaries[0].Sent)
	require.True(t, result.DeviceResultSummaries[1].Retryable)
	require.Equal(t, "push provider retryable failure", result.DeviceResultSummaries[1].Error)
	require.True(t, result.DeviceResultSummaries[2].Skipped)
	require.False(t, result.DeviceResultSummaries[3].Retryable)
	require.Equal(t, "push provider permanent failure", result.DeviceResultSummaries[3].Error)
}

func TestMerchantAppPushDispatcherClearsDeviceDegradationAfterSuccessfulSend(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	xiaomi := &recordingMerchantAppPushProvider{}
	dispatcher := NewMerchantAppPushDispatcher(store, StaticMerchantAppPushProviderRegistry{
		db.MerchantAppDeviceProviderXiaomi: xiaomi,
	})

	payload := MerchantAppNotificationPayload{
		MessageID: "merchant:new_order:502",
		Event:     MerchantNotificationEventNewOrder,
		OrderID:   502,
		Title:     "新订单",
		Content:   "您有一笔新订单 ORD502，请及时处理",
	}

	store.EXPECT().ListActiveMerchantAppDevicesByMerchant(gomock.Any(), int64(601)).Return([]db.MerchantAppDevice{
		{
			ID:                    21,
			DeviceID:              "device-xiaomi",
			Provider:              db.MerchantAppDeviceProviderXiaomi,
			PushToken:             "token-xiaomi",
			PushFailureCount:      1,
			LastPushFailureReason: pgtype.Text{String: "xiaomi invalid token", Valid: true},
			LastPushFailureAt:     pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			PushDegradedAt:        pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		},
	}, nil)
	store.EXPECT().
		ClearMerchantAppDevicePushFailure(gomock.Any(), db.ClearMerchantAppDevicePushFailureParams{
			ID:        21,
			PushToken: "token-xiaomi",
		}).
		Times(1).
		Return(int64(1), nil)

	result, err := dispatcher.Dispatch(context.Background(), MerchantAppPushDispatchInput{MerchantID: 601, Payload: payload})
	require.NoError(t, err)
	require.Equal(t, 1, result.Attempted)
	require.Equal(t, 1, result.Sent)
	require.Len(t, xiaomi.targets, 1)
	require.Equal(t, "token-xiaomi", xiaomi.targets[0].PushToken)
}

func TestMerchantAppPushDispatcherDispatchRequiresMessageID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	dispatcher := NewMerchantAppPushDispatcher(store, StaticMerchantAppPushProviderRegistry{})

	result, err := dispatcher.Dispatch(context.Background(), MerchantAppPushDispatchInput{MerchantID: 601})
	require.Error(t, err)
	require.Empty(t, result)
}
