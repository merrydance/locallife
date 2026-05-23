package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type capturingPublisher struct {
	channel string
	payload []byte
}

func (p *capturingPublisher) Publish(_ context.Context, channel string, payload []byte) error {
	p.channel = channel
	p.payload = payload
	return nil
}

func TestNewMerchantStatusChangeMessage(t *testing.T) {
	autoCloseAt := time.Date(2026, 5, 22, 18, 30, 0, 0, time.Local)

	msg, err := NewMerchantStatusChangeMessage(42, true, &autoCloseAt, "manual")
	require.NoError(t, err)
	require.Equal(t, MessageTypeMerchantStatusChange, msg.Type)
	require.False(t, msg.Timestamp.IsZero())

	var data MerchantStatusChangeData
	require.NoError(t, json.Unmarshal(msg.Data, &data))
	require.Equal(t, int64(42), data.MerchantID)
	require.True(t, data.IsOpen)
	require.NotNil(t, data.AutoCloseAt)
	require.True(t, autoCloseAt.Equal(*data.AutoCloseAt))
	require.Equal(t, "manual", data.Source)
}

func TestRedisMerchantStatusChangePublisher_PublishMerchantStatusChange(t *testing.T) {
	publisher := &capturingPublisher{}
	broadcaster := NewRedisMerchantStatusChangePublisher(publisher)

	err := broadcaster.PublishMerchantStatusChange(context.Background(), 55, false, nil, "business_hours")
	require.NoError(t, err)
	require.Equal(t, "notification:merchant:55", publisher.channel)

	var push NotificationPushMessage
	require.NoError(t, json.Unmarshal(publisher.payload, &push))
	require.Equal(t, EntityMerchant, push.EntityType)
	require.Equal(t, int64(55), push.EntityID)
	require.Equal(t, MessageTypeMerchantStatusChange, push.Message.Type)

	var data MerchantStatusChangeData
	require.NoError(t, json.Unmarshal(push.Message.Data, &data))
	require.Equal(t, int64(55), data.MerchantID)
	require.False(t, data.IsOpen)
	require.Equal(t, "business_hours", data.Source)
}
