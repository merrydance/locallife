package logic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProfitSharingReceiverLifecycleService_ProcessOperatorReceiverTarget_AddAlreadyExistsMarksIdempotentSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewProfitSharingReceiverLifecycleService(store, ecommerceClient)
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	operator := db.Operator{ID: 71, UserID: 171, Name: "运营商甲", ContactName: "联系人甲"}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-171"}
	target := receiverLifecycleTestTarget(901, operator.ID, user.WechatOpenid, db.ProfitSharingReceiverDesiredStatePresent)
	attempt := db.ProfitSharingReceiverAttempt{ID: 991, TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing}

	gomock.InOrder(
		store.EXPECT().ClaimProfitSharingReceiverTarget(gomock.Any(), db.ClaimProfitSharingReceiverTargetParams{NowAt: receiverLifecycleTimestamp(now), ID: target.ID}).Return(target, nil),
		store.EXPECT().CreateProfitSharingReceiverAttempt(gomock.Any(), db.CreateProfitSharingReceiverAttemptParams{TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing, StartedAt: now}).Return(attempt, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil),
		store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		ecommerceClient.EXPECT().EncryptSensitiveData(operator.ContactName).Return("encrypted-contact", nil),
		ecommerceClient.EXPECT().AddProfitSharingReceiver(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.AddReceiverRequest) (*wechatcontracts.AddReceiverResponse, error) {
			require.Equal(t, "wx_sp_app_123", req.AppID)
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Type)
			require.Equal(t, user.WechatOpenid, req.Account)
			require.Equal(t, "encrypted-contact", req.EncryptedName)
			return nil, &wechat.WechatPayError{StatusCode: 400, Code: "RESOURCE_ALREADY_EXISTS", Message: "receiver exists"}
		}),
		store.EXPECT().MarkProfitSharingReceiverTargetSynced(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverTargetSyncedParams) (db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, target.ID, arg.ID)
			require.True(t, arg.SyncedAt.Valid)
			return target, nil
		}),
		store.EXPECT().MarkProfitSharingReceiverAttemptSucceeded(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverAttemptSucceededParams) (db.ProfitSharingReceiverAttempt, error) {
			require.Equal(t, attempt.ID, arg.ID)
			require.True(t, arg.IdempotentSuccess)
			require.True(t, arg.FinishedAt.Valid)
			return attempt, nil
		}),
	)

	result, err := service.ProcessOperatorReceiverTarget(context.Background(), target.ID, now)
	require.NoError(t, err)
	require.Equal(t, db.ProfitSharingReceiverSyncStatusSynced, result.Status)
	require.Equal(t, db.ProfitSharingReceiverAttemptActionEnsure, result.Action)
}

func TestProfitSharingReceiverLifecycleService_ProcessOperatorReceiverTarget_MissingOpenIDMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewProfitSharingReceiverLifecycleService(store, ecommerceClient)
	now := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	operator := db.Operator{ID: 72, UserID: 172, Name: "运营商乙"}
	target := receiverLifecycleTestTarget(902, operator.ID, "old-openid", db.ProfitSharingReceiverDesiredStatePresent)
	attempt := db.ProfitSharingReceiverAttempt{ID: 992, TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing}

	gomock.InOrder(
		store.EXPECT().ClaimProfitSharingReceiverTarget(gomock.Any(), db.ClaimProfitSharingReceiverTargetParams{NowAt: receiverLifecycleTimestamp(now), ID: target.ID}).Return(target, nil),
		store.EXPECT().CreateProfitSharingReceiverAttempt(gomock.Any(), db.CreateProfitSharingReceiverAttemptParams{TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionEnsure, Status: db.ProfitSharingReceiverAttemptStatusProcessing, StartedAt: now}).Return(attempt, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil),
		store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(db.User{ID: operator.UserID}, nil),
		store.EXPECT().MarkProfitSharingReceiverTargetFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverTargetFailedParams) (db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, target.ID, arg.ID)
			require.Equal(t, "operator_openid_missing", arg.LastErrorCode.String)
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.NextRetryAt.Time.After(now))
			return target, nil
		}),
		store.EXPECT().MarkProfitSharingReceiverAttemptFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverAttemptFailedParams) (db.ProfitSharingReceiverAttempt, error) {
			require.Equal(t, attempt.ID, arg.ID)
			require.Equal(t, "operator_openid_missing", arg.ErrorCode.String)
			require.True(t, arg.FinishedAt.Valid)
			return attempt, nil
		}),
	)

	result, err := service.ProcessOperatorReceiverTarget(context.Background(), target.ID, now)
	require.NoError(t, err)
	require.Equal(t, db.ProfitSharingReceiverSyncStatusFailed, result.Status)
	require.Equal(t, "operator_openid_missing", result.ErrorCode)
}

func TestProfitSharingReceiverLifecycleService_RequestRiderReceiverPresentWritesTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewProfitSharingReceiverLifecycleService(store, ecommerceClient)
	rider := db.Rider{ID: 81, UserID: 181, RealName: "骑手甲"}
	user := db.User{ID: rider.UserID, WechatOpenid: "rider-openid-181"}

	gomock.InOrder(
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(user, nil),
		store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ProfitSharingReceiverOwnerTypeRider, arg.OwnerType)
			require.Equal(t, rider.ID, arg.OwnerID)
			require.Equal(t, db.ProfitSharingReceiverTypePersonalOpenID, arg.ReceiverType)
			require.Equal(t, "wx_sp_app_123", arg.Appid)
			require.Equal(t, receiverLifecycleHash(user.WechatOpenid), arg.AccountHash)
			require.Equal(t, receiverLifecycleHash(rider.RealName), arg.DisplayNameHash.String)
			require.Equal(t, db.ProfitSharingReceiverDesiredStatePresent, arg.DesiredState)
			return db.ProfitSharingReceiverTarget{ID: 903, OwnerType: arg.OwnerType, DesiredState: arg.DesiredState}, nil
		}),
	)

	target, err := service.RequestRiderReceiverPresent(context.Background(), rider)
	require.NoError(t, err)
	require.Equal(t, int64(903), target.ID)
}

func TestProfitSharingReceiverLifecycleService_ProcessRiderReceiverTarget_DeleteNotExistsMarksIdempotentSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewProfitSharingReceiverLifecycleService(store, ecommerceClient)
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	rider := db.Rider{ID: 82, UserID: 182, RealName: "骑手乙"}
	user := db.User{ID: rider.UserID, WechatOpenid: "rider-openid-182"}
	target := receiverLifecycleRiderTestTarget(904, rider.ID, user.WechatOpenid, db.ProfitSharingReceiverDesiredStateAbsent)
	attempt := db.ProfitSharingReceiverAttempt{ID: 994, TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionDelete, Status: db.ProfitSharingReceiverAttemptStatusProcessing}

	gomock.InOrder(
		store.EXPECT().ClaimProfitSharingReceiverTarget(gomock.Any(), db.ClaimProfitSharingReceiverTargetParams{NowAt: receiverLifecycleTimestamp(now), ID: target.ID}).Return(target, nil),
		store.EXPECT().CreateProfitSharingReceiverAttempt(gomock.Any(), db.CreateProfitSharingReceiverAttemptParams{TargetID: target.ID, Action: db.ProfitSharingReceiverAttemptActionDelete, Status: db.ProfitSharingReceiverAttemptStatusProcessing, StartedAt: now}).Return(attempt, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil),
		store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(user, nil),
		ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123"),
		ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.DeleteReceiverRequest) (*wechatcontracts.DeleteReceiverResponse, error) {
			require.Equal(t, "wx_sp_app_123", req.AppID)
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Type)
			require.Equal(t, user.WechatOpenid, req.Account)
			return nil, &wechat.WechatPayError{StatusCode: 404, Code: "RESOURCE_NOT_EXISTS", Message: "receiver not exists"}
		}),
		store.EXPECT().MarkProfitSharingReceiverTargetSynced(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverTargetSyncedParams) (db.ProfitSharingReceiverTarget, error) {
			require.Equal(t, target.ID, arg.ID)
			require.True(t, arg.SyncedAt.Valid)
			return target, nil
		}),
		store.EXPECT().MarkProfitSharingReceiverAttemptSucceeded(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkProfitSharingReceiverAttemptSucceededParams) (db.ProfitSharingReceiverAttempt, error) {
			require.Equal(t, attempt.ID, arg.ID)
			require.True(t, arg.IdempotentSuccess)
			require.True(t, arg.FinishedAt.Valid)
			return attempt, nil
		}),
	)

	result, err := service.ProcessReceiverTarget(context.Background(), target.ID, now)
	require.NoError(t, err)
	require.Equal(t, db.ProfitSharingReceiverSyncStatusSynced, result.Status)
	require.Equal(t, db.ProfitSharingReceiverAttemptActionDelete, result.Action)
}

func TestReceiverLifecycleFailureFromError_SanitizesWechatDetail(t *testing.T) {
	err := fmt.Errorf("add profit sharing receiver: %w", &wechat.WechatPayError{
		StatusCode: 400,
		Code:       "PARAM_ERROR",
		Message:    "invalid receiver",
		Detail:     "account=operator-openid-secret",
	})

	failure := receiverLifecycleFailureFromError(err, 1)
	require.Equal(t, "wechat_param_error", failure.code)
	require.Equal(t, "wechat receiver sync failed: param_error", failure.message)
	require.NotContains(t, failure.message, "operator-openid-secret")
}

func receiverLifecycleTestTarget(id int64, operatorID int64, openID string, desiredState string) db.ProfitSharingReceiverTarget {
	return db.ProfitSharingReceiverTarget{
		ID:           id,
		Provider:     db.ExternalPaymentProviderWechat,
		Channel:      db.PaymentChannelEcommerce,
		OwnerType:    db.ProfitSharingReceiverOwnerTypeOperator,
		OwnerID:      operatorID,
		ReceiverType: db.ProfitSharingReceiverTypePersonalOpenID,
		Appid:        "wx_sp_app_123",
		AccountHash:  receiverLifecycleHash(openID),
		DesiredState: desiredState,
		SyncStatus:   db.ProfitSharingReceiverSyncStatusProcessing,
		AttemptCount: 1,
		LastAttemptAt: pgtype.Timestamptz{
			Time:  time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC),
			Valid: true,
		},
	}
}

func receiverLifecycleRiderTestTarget(id int64, riderID int64, openID string, desiredState string) db.ProfitSharingReceiverTarget {
	target := receiverLifecycleTestTarget(id, riderID, openID, desiredState)
	target.OwnerType = db.ProfitSharingReceiverOwnerTypeRider
	target.OwnerID = riderID
	return target
}
