package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type recordedPublish struct {
	channel string
	payload []byte
}

type recordingPublisher struct {
	mu        sync.Mutex
	published []recordedPublish
}

func (p *recordingPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, recordedPublish{channel: channel, payload: append([]byte(nil), payload...)})
	return nil
}

func (p *recordingPublisher) snapshot() []recordedPublish {
	p.mu.Lock()
	defer p.mu.Unlock()
	cloned := make([]recordedPublish, len(p.published))
	copy(cloned, p.published)
	return cloned
}

func TestDataCleanupScheduler_RemindExpiringRiderDepositCredits_DistributesNotificationTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	s := NewDataCleanupScheduler(store, distributor, nil)

	credit := db.RiderDepositCredit{
		ID:               1,
		RiderID:          88,
		PaymentOrderID:   9001,
		RefundableAmount: 12345,
		RefundableUntil:  startOfDay(time.Now()).AddDate(0, 0, 7).Add(2 * time.Hour),
	}
	rider := db.Rider{ID: 88, UserID: 66}

	store.EXPECT().ListRiderDepositCreditsForReminderWindow(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ListRiderDepositCreditsForReminderWindowParams) ([]db.RiderDepositCredit, error) {
			windowStart := startOfDay(time.Now()).AddDate(0, 0, 7)
			if arg.RefundableUntil.Equal(windowStart) {
				return []db.RiderDepositCredit{credit}, nil
			}
			return []db.RiderDepositCredit{}, nil
		},
	).Times(len(riderDepositReminderOffsets))

	store.EXPECT().GetRider(gomock.Any(), credit.RiderID).Return(rider, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, rider.UserID, payload.UserID)
			require.Equal(t, "system", payload.Type)
			require.Contains(t, payload.Title, "7 天")
			require.Contains(t, payload.Content, "123.45")
			require.True(t, payload.IgnorePreferences)
			require.EqualValues(t, credit.ID, payload.ExtraData["credit_id"])
			require.EqualValues(t, 7, payload.ExtraData["days_remaining"])
			return nil
		},
	)
	store.EXPECT().TouchRiderDepositCreditReminder(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.TouchRiderDepositCreditReminderParams) (db.RiderDepositCredit, error) {
			require.Equal(t, credit.ID, arg.ID)
			require.True(t, arg.LastRemindedAt.Valid)
			return credit, nil
		},
	)

	s.remindExpiringRiderDepositCredits()
}

func TestDataCleanupScheduler_RemindExpiringRiderDepositCredits_FallbackDirectNotificationAndPublishAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, worker.NewNoopTaskDistributor(), publisher)

	credit := db.RiderDepositCredit{
		ID:               2,
		RiderID:          77,
		PaymentOrderID:   9002,
		RefundableAmount: 8800,
		RefundableUntil:  startOfDay(time.Now()).Add(3 * time.Hour),
	}
	rider := db.Rider{ID: 77, UserID: 55}

	store.EXPECT().ListRiderDepositCreditsForReminderWindow(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ListRiderDepositCreditsForReminderWindowParams) ([]db.RiderDepositCredit, error) {
			windowStart := startOfDay(time.Now())
			if arg.RefundableUntil.Equal(windowStart) {
				return []db.RiderDepositCredit{credit}, nil
			}
			return []db.RiderDepositCredit{}, nil
		},
	).Times(len(riderDepositReminderOffsets))
	store.EXPECT().GetRider(gomock.Any(), credit.RiderID).Return(rider, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
			require.Equal(t, rider.UserID, arg.UserID)
			require.Equal(t, "system", arg.Type)
			require.Contains(t, arg.Title, "今日到期")

			var extra map[string]any
			require.NoError(t, json.Unmarshal(arg.ExtraData, &extra))
			require.EqualValues(t, float64(credit.ID), extra["credit_id"])
			require.EqualValues(t, float64(0), extra["days_remaining"])
			return db.Notification{ID: 1, UserID: rider.UserID}, nil
		},
	)
	store.EXPECT().TouchRiderDepositCreditReminder(gomock.Any(), gomock.Any()).Return(credit, nil)

	s.remindExpiringRiderDepositCredits()

	published := publisher.snapshot()
	require.Len(t, published, 1)
	require.Equal(t, worker.AlertChannel, published[0].channel)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(published[0].payload, &payload))
	alertData := payload["data"].(map[string]any)
	require.Equal(t, string(worker.AlertTypeRiderDepositExpiry), alertData["alert_type"])
	require.Equal(t, string(worker.AlertLevelWarning), alertData["level"])
	require.Equal(t, "骑手押金退款今日到期提醒已发送", alertData["title"])
}

func TestDataCleanupScheduler_MarkExpiredRiderDepositCredits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, nil, publisher)

	expiredCredit := db.RiderDepositCredit{ID: 12, RiderID: 5, RefundableAmount: 3456}

	gomock.InOrder(
		store.EXPECT().ListExpiredRiderDepositCredits(gomock.Any(), gomock.Any()).Return([]db.RiderDepositCredit{expiredCredit}, nil),
		store.EXPECT().MarkRiderDepositCreditExpired(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, arg db.MarkRiderDepositCreditExpiredParams) (db.RiderDepositCredit, error) {
				require.Equal(t, expiredCredit.ID, arg.ID)
				require.True(t, arg.ExpiredAt.Valid)
				return expiredCredit, nil
			},
		),
	)

	s.markExpiredRiderDepositCredits()

	published := publisher.snapshot()
	require.Len(t, published, 1)
	require.Equal(t, worker.AlertChannel, published[0].channel)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(published[0].payload, &payload))
	alertData := payload["data"].(map[string]any)
	require.Equal(t, string(worker.AlertTypeRiderDepositExpiry), alertData["alert_type"])
	require.Equal(t, "骑手押金退款凭证已过期", alertData["title"])
}

func TestDataCleanupScheduler_CleanupExpiredPaymentOrders_ClosesExpiredCombinedPaymentOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	combinedA := db.CombinedPaymentOrder{ID: 3001, Status: "pending"}
	combinedB := db.CombinedPaymentOrder{ID: 3002, Status: "pending"}

	gomock.InOrder(
		store.EXPECT().CloseExpiredPaymentOrders(gomock.Any()).Return(int64(2), nil),
		store.EXPECT().ListPendingCombinedPaymentOrders(gomock.Any(), expiredCombinedPaymentBatchLimit).Return([]db.CombinedPaymentOrder{combinedA, combinedB}, nil),
		store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedA.ID).Return(db.CombinedPaymentOrder{ID: combinedA.ID, Status: "closed"}, nil),
		store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedB.ID).Return(db.CombinedPaymentOrder{ID: combinedB.ID, Status: "closed"}, nil),
	)

	s.cleanupExpiredPaymentOrders()
}

func TestDataCleanupScheduler_CleanupExpiredPaymentOrders_IgnoresCombinedPaymentStateRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	combinedA := db.CombinedPaymentOrder{ID: 3101, Status: "pending"}
	combinedB := db.CombinedPaymentOrder{ID: 3102, Status: "pending"}

	gomock.InOrder(
		store.EXPECT().CloseExpiredPaymentOrders(gomock.Any()).Return(int64(0), nil),
		store.EXPECT().ListPendingCombinedPaymentOrders(gomock.Any(), expiredCombinedPaymentBatchLimit).Return([]db.CombinedPaymentOrder{combinedA, combinedB}, nil),
		store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedA.ID).Return(db.CombinedPaymentOrder{}, db.ErrRecordNotFound),
		store.EXPECT().UpdateCombinedPaymentOrderToClosed(gomock.Any(), combinedB.ID).Return(db.CombinedPaymentOrder{ID: combinedB.ID, Status: "closed"}, nil),
	)

	s.cleanupExpiredPaymentOrders()
}
