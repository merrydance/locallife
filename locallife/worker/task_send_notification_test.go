package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTryWebSocketPush_MerchantRolesPublishToMerchantChannel(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{name: "merchant owner", role: "merchant_owner"},
		{name: "merchant staff", role: "merchant_staff"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().ListUserRoles(gomock.Any(), int64(88)).Return([]db.UserRole{{
				Role:            tc.role,
				RelatedEntityID: pgtype.Int8{Int64: 12, Valid: true},
			}}, nil)
			store.EXPECT().MarkNotificationAsPushed(gomock.Any(), int64(1)).Return(nil)

			processor := NewTestTaskProcessor(store, nil, nil, nil)
			publisher := &testPublisher{}
			processor.pubSubPublisher = publisher

			err := processor.tryWebSocketPush(context.Background(), 88, db.Notification{
				ID:        1,
				UserID:    88,
				Type:      "order",
				Title:     "新订单",
				Content:   "请及时处理",
				CreatedAt: time.Unix(1700000000, 0),
			})
			require.NoError(t, err)
			require.Equal(t, "notification:merchant:12", publisher.channel)

			var push websocket.NotificationPushMessage
			require.NoError(t, json.Unmarshal(publisher.payload, &push))
			require.Equal(t, "merchant", push.EntityType)
			require.Equal(t, int64(12), push.EntityID)
		})
	}
}
