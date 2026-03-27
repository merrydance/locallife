package worker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"go.uber.org/mock/gomock"
)

func TestWechatNotificationRecoverySchedulerRunOnceReleasesStaleClaims(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		ListStaleUnprocessedWechatNotifications(gomock.Any(), gomock.Any()).
		Return([]db.WechatNotification{
			{ID: "notif_1", EventType: "TRANSACTION.SUCCESS", CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-20 * time.Minute), Valid: true}},
			{ID: "notif_2", EventType: "REFUND.SUCCESS", CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-18 * time.Minute), Valid: true}},
		}, nil)

	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), "notif_1").Return(nil)
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), "notif_2").Return(nil)

	scheduler := worker.NewWechatNotificationRecoveryScheduler(store)
	scheduler.RunOnce()
}

func TestWechatNotificationRecoverySchedulerRunOnceContinuesAfterReleaseFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		ListStaleUnprocessedWechatNotifications(gomock.Any(), gomock.Any()).
		Return([]db.WechatNotification{
			{ID: "notif_fail", EventType: "TRANSACTION.SUCCESS", CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-20 * time.Minute), Valid: true}},
			{ID: "notif_ok", EventType: "REFUND.SUCCESS", CreatedAt: pgtype.Timestamp{Time: time.Now().Add(-18 * time.Minute), Valid: true}},
		}, nil)

	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), "notif_fail").Return(errors.New("delete failed"))
	store.EXPECT().ReleaseWechatNotificationClaim(gomock.Any(), "notif_ok").Return(nil)

	scheduler := worker.NewWechatNotificationRecoveryScheduler(store)
	scheduler.RunOnce()
}
