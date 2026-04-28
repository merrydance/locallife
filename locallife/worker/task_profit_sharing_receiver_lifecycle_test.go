package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskProfitSharingReceiverTarget_PublishesAlertAfterRepeatedFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	target := profitSharingReceiverAlertTestTarget(902, 72, 3)
	operator := db.Operator{ID: 72, UserID: 172, Name: "运营商乙"}
	attempt := db.ProfitSharingReceiverAttempt{ID: 992, TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing}
	failedTarget := target
	failedTarget.SyncStatus = db.ProfitSharingReceiverSyncStatusFailed
	failedTarget.LastErrorCode = pgtype.Text{String: "operator_openid_missing", Valid: true}
	failedTarget.LastErrorMessage = pgtype.Text{String: "operator wechat openid is empty", Valid: true}
	failedTarget.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 4, 26, 9, 30, 0, 0, time.UTC), Valid: true}
	failedTarget.NextRetryAt = pgtype.Timestamptz{Time: time.Date(2026, 4, 27, 9, 30, 0, 0, time.UTC), Valid: true}

	gomock.InOrder(
		store.EXPECT().ClaimProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).Return(target, nil),
		store.EXPECT().CreateProfitSharingReceiverAttempt(gomock.Any(), gomock.Any()).Return(attempt, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil),
		store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(db.User{ID: operator.UserID}, nil),
		store.EXPECT().MarkProfitSharingReceiverTargetFailed(gomock.Any(), gomock.Any()).Return(failedTarget, nil),
		store.EXPECT().MarkProfitSharingReceiverAttemptFailed(gomock.Any(), gomock.Any()).Return(attempt, nil),
		store.EXPECT().GetProfitSharingReceiverTarget(gomock.Any(), target.ID).Return(failedTarget, nil),
	)

	payload, err := json.Marshal(ProfitSharingReceiverTargetPayload{TargetID: target.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReceiverTarget(context.Background(), asynq.NewTask(TaskProcessProfitSharingReceiverTarget, payload))
	require.NoError(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]any)
	require.Equal(t, string(AlertTypeProfitSharingReceiverFailed), data["alert_type"])
	require.Equal(t, "profit_sharing_receiver_target", data["related_type"])
	require.Equal(t, float64(target.ID), data["related_id"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, target.OwnerType, extra["owner_type"])
	require.Equal(t, float64(target.OwnerID), extra["owner_id"])
	require.Equal(t, target.DesiredState, extra["desired_state"])
	require.Equal(t, db.ProfitSharingReceiverSyncStatusFailed, extra["sync_status"])
	require.Equal(t, float64(target.AttemptCount), extra["attempt_count"])
	require.Equal(t, "operator_openid_missing", extra["last_error_code"])
	require.Equal(t, "operator wechat openid is empty", extra["last_error_message"])
	require.NotEmpty(t, extra["last_attempt_at"])
	require.NotEmpty(t, extra["next_retry_at"])
}

func TestProcessTaskProfitSharingReceiverTarget_DoesNotPublishAlertBelowRepeatedFailureThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	target := profitSharingReceiverAlertTestTarget(903, 73, 2)
	operator := db.Operator{ID: 73, UserID: 173, Name: "运营商丙"}
	attempt := db.ProfitSharingReceiverAttempt{ID: 993, TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing}
	failedTarget := target
	failedTarget.SyncStatus = db.ProfitSharingReceiverSyncStatusFailed
	failedTarget.LastErrorCode = pgtype.Text{String: "operator_openid_missing", Valid: true}
	failedTarget.LastErrorMessage = pgtype.Text{String: "operator wechat openid is empty", Valid: true}

	gomock.InOrder(
		store.EXPECT().ClaimProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).Return(target, nil),
		store.EXPECT().CreateProfitSharingReceiverAttempt(gomock.Any(), gomock.Any()).Return(attempt, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil),
		store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(db.User{ID: operator.UserID}, nil),
		store.EXPECT().MarkProfitSharingReceiverTargetFailed(gomock.Any(), gomock.Any()).Return(failedTarget, nil),
		store.EXPECT().MarkProfitSharingReceiverAttemptFailed(gomock.Any(), gomock.Any()).Return(attempt, nil),
	)

	payload, err := json.Marshal(ProfitSharingReceiverTargetPayload{TargetID: target.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharingReceiverTarget(context.Background(), asynq.NewTask(TaskProcessProfitSharingReceiverTarget, payload))
	require.NoError(t, err)
	require.Empty(t, publisher.channel)
	require.Empty(t, publisher.payload)
}

func profitSharingReceiverAlertTestTarget(targetID, ownerID int64, attemptCount int32) db.ProfitSharingReceiverTarget {
	return db.ProfitSharingReceiverTarget{
		ID:           targetID,
		Provider:     db.ExternalPaymentProviderWechat,
		Channel:      db.PaymentChannelEcommerce,
		OwnerType:    db.ProfitSharingReceiverOwnerTypeOperator,
		OwnerID:      ownerID,
		ReceiverType: db.ProfitSharingReceiverTypePersonalOpenID,
		Appid:        "wx_sp_app_123",
		AccountHash:  profitSharingReceiverAlertTestHash("old-openid"),
		DesiredState: db.ProfitSharingReceiverDesiredStatePresent,
		SyncStatus:   db.ProfitSharingReceiverSyncStatusProcessing,
		AttemptCount: attemptCount,
		UpdatedAt:    time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC),
	}
}

func profitSharingReceiverAlertTestHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
