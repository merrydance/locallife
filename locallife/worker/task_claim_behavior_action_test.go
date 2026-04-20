package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskClaimBehaviorAction_RestrictionCreatesBlocklist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	detailBytes, err := json.Marshal(claimRestrictionActionDetail{
		Action:            "apply_user_restriction",
		ClaimID:           41,
		UserID:            22,
		DecisionMode:      db.BehaviorDecisionModeUserRestricted,
		RestrictionReason: "confirmed_high_user_risk",
		Remark:            "user restricted action created",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           501,
		DecisionID:   401,
		ActionType:   "block",
		TargetEntity: "user",
		Status:       "created",
		Detail:       detailBytes,
	}

	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().UpdateBehaviorActionExecutionIfCurrent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionIfCurrentParams) (db.BehaviorAction, error) {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "created", arg.Status)
			require.Equal(t, "running", arg.Status_2)
			return db.BehaviorAction{ID: action.ID, Status: "running"}, nil
		},
	)
	store.EXPECT().GetClaim(gomock.Any(), int64(41)).Return(db.Claim{
		ID:     41,
		Status: db.ClaimStatusApproved,
		PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: int64(22)}).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{ConfigKey: "behavior_trace.reject_service_cooldown_days", ScopeType: "global", ScopeID: pgtype.Int8{Valid: false}}).Return(db.PlatformConfig{
		ConfigKey:   "behavior_trace.reject_service_cooldown_days",
		ConfigValue: []byte(`{"days":21}`),
	}, nil)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.CreateBehaviorBlocklistParams) (db.BehaviorBlocklist, error) {
			require.Equal(t, int64(22), arg.EntityID)
			require.Equal(t, "user", arg.EntityType)
			require.Equal(t, "malicious-claims", arg.ReasonCode)
			require.True(t, arg.BlockUntil.Valid)
			return db.BehaviorBlocklist{ID: 9001}, nil
		},
	)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			var persisted claimRestrictionActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, int64(22), persisted.UserID)
			require.Empty(t, persisted.LastError)
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimBehaviorActionPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimBehaviorAction(context.Background(), asynq.NewTask(TaskClaimBehaviorAction, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimBehaviorAction_NotificationCreatesNotificationAndPushes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	detailBytes, err := json.Marshal(claimNotifyActionDetail{
		Action:           "notify_responsible_party",
		ClaimID:          51,
		TargetEntity:     "merchant",
		TargetID:         18,
		RecipientUserID:  118,
		NotificationType: "system",
		Title:            "异常订单判责通知",
		Content:          "平台已向用户先行赔付，并已生成追偿单，请尽快处理。",
		RelatedType:      "claim",
		RelatedID:        51,
		Remark:           "notification action created",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           601,
		DecisionID:   402,
		ActionType:   "notify",
		TargetEntity: "merchant",
		Status:       "created",
		Detail:       detailBytes,
	}

	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().UpdateBehaviorActionExecutionIfCurrent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionIfCurrentParams) (db.BehaviorAction, error) {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "created", arg.Status)
			require.Equal(t, "running", arg.Status_2)
			return db.BehaviorAction{ID: action.ID, Status: "running"}, nil
		},
	)
	store.EXPECT().GetClaim(gomock.Any(), int64(51)).Return(db.Claim{
		ID:     51,
		Status: db.ClaimStatusApproved,
		PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil)
	store.EXPECT().GetNotificationsByRelated(gomock.Any(), db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "claim", Valid: true},
		RelatedID:   pgtype.Int8{Int64: 51, Valid: true},
	}).Return([]db.Notification{}, nil)
	store.EXPECT().GetUserNotificationPreferences(gomock.Any(), int64(118)).Times(0)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
			require.Equal(t, int64(118), arg.UserID)
			require.Equal(t, "system", arg.Type)
			require.Equal(t, "异常订单判责通知", arg.Title)
			return db.Notification{
				ID:        701,
				UserID:    118,
				Type:      arg.Type,
				Title:     arg.Title,
				Content:   arg.Content,
				CreatedAt: time.Now(),
			}, nil
		},
	)
	store.EXPECT().ListUserRoles(gomock.Any(), int64(118)).Return([]db.UserRole{{
		Role:            db.UserRoleMerchantOwner,
		RelatedEntityID: pgtype.Int8{Int64: 18, Valid: true},
	}}, nil)
	store.EXPECT().MarkNotificationAsPushed(gomock.Any(), int64(701)).Return(nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			var persisted claimNotifyActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, int64(118), persisted.RecipientUserID)
			require.Empty(t, persisted.LastError)
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimBehaviorActionPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimBehaviorAction(context.Background(), asynq.NewTask(TaskClaimBehaviorAction, payloadBytes))
	require.NoError(t, err)
	require.Equal(t, "notification:merchant:18", publisher.channel)
}

func TestProcessTaskClaimBehaviorAction_RestrictionWaitsForPayoutCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	detailBytes, err := json.Marshal(claimRestrictionActionDetail{
		Action:       "apply_user_restriction",
		ClaimID:      61,
		UserID:       31,
		DecisionMode: db.BehaviorDecisionModeUserRestricted,
		Remark:       "user restricted action created after payout",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{ID: 901, ActionType: "block", TargetEntity: "user", Status: "created", Detail: detailBytes}
	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().UpdateBehaviorActionExecutionIfCurrent(gomock.Any(), gomock.Any()).Return(db.BehaviorAction{ID: action.ID, Status: "running"}, nil)
	store.EXPECT().GetClaim(gomock.Any(), int64(61)).Return(db.Claim{ID: 61, Status: db.ClaimStatusApproved}, nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "created", arg.Status)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimBehaviorActionPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimBehaviorAction(context.Background(), asynq.NewTask(TaskClaimBehaviorAction, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimBehaviorAction_NotificationReusesExistingNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	detailBytes, err := json.Marshal(claimNotifyActionDetail{
		Action:           "notify_responsible_party",
		ClaimID:          71,
		TargetEntity:     "merchant",
		TargetID:         18,
		RecipientUserID:  118,
		NotificationType: "system",
		Title:            "异常订单判责通知",
		Content:          "平台已向用户先行赔付，并已生成追偿单，请尽快处理。",
		RelatedType:      "claim",
		RelatedID:        71,
		Remark:           "notification action created",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{ID: 902, ActionType: "notify", TargetEntity: "merchant", Status: "running", Detail: detailBytes}
	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().GetClaim(gomock.Any(), int64(71)).Return(db.Claim{
		ID:     71,
		Status: db.ClaimStatusApproved,
		PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil)
	store.EXPECT().GetNotificationsByRelated(gomock.Any(), db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "claim", Valid: true},
		RelatedID:   pgtype.Int8{Int64: 71, Valid: true},
	}).Return([]db.Notification{{
		ID:        811,
		UserID:    118,
		Type:      "system",
		Title:     "异常订单判责通知",
		Content:   "平台已向用户先行赔付，并已生成追偿单，请尽快处理。",
		IsPushed:  false,
		CreatedAt: time.Now(),
	}}, nil)
	store.EXPECT().ListUserRoles(gomock.Any(), int64(118)).Return([]db.UserRole{{
		Role:            db.UserRoleMerchantOwner,
		RelatedEntityID: pgtype.Int8{Int64: 18, Valid: true},
	}}, nil)
	store.EXPECT().MarkNotificationAsPushed(gomock.Any(), int64(811)).Return(nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimBehaviorActionPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimBehaviorAction(context.Background(), asynq.NewTask(TaskClaimBehaviorAction, payloadBytes))
	require.NoError(t, err)
	require.Equal(t, "notification:merchant:18", publisher.channel)
}
