package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func recoveryDisputeResultRecoveryQuery(claimID int64, recoveryTarget string) db.GetClaimRecoveryByClaimIDAndTargetParams {
	return db.GetClaimRecoveryByClaimIDAndTargetParams{
		ClaimID:        claimID,
		RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: true},
	}
}

func recoveryDisputeResolutionContextQuery(claimID int64, recoveryTarget string) db.GetClaimRecoveryContextByClaimIDAndTargetParams {
	return db.GetClaimRecoveryContextByClaimIDAndTargetParams{
		ClaimID:        claimID,
		RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: true},
	}
}

func TestExecuteRecoveryDisputeCompensation_ApprovedExecutesPayoutAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		RecoveryDisputeID: 21,
		UserID:            22,
		Amount:            1800,
		SourceType:        "platform",
		Remark:            "appeal compensation",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           109,
		DecisionID:   77,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "created",
		Detail:       detailBytes,
	}

	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().GetUser(gomock.Any(), int64(22)).Return(db.User{ID: 22, WechatOpenid: "openid-22", FullName: "张三"}, nil)
	transferClient.EXPECT().GetAppID().Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			return nil
		},
	)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error) {
			require.Equal(t, int64(1800), req.TransferAmount)
			require.Equal(t, "openid-22", req.OpenID)
			return &wechatcontracts.DirectMerchantTransferCreateResponse{OutBillNo: req.OutBillNo, TransferBillNo: "wx-bill-109", State: wechatcontracts.DirectMerchantTransferStateAccepted}, nil
		},
	)
	expectClaimPayoutTransferCommandAccepted(t, store, action.ID, claimPayoutOutBillNo(action.ID), "wx-bill-109", false)
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      claimPayoutOutBillNo(action.ID),
		TransferBillNo: "wx-bill-109",
		State:          wechatcontracts.DirectMerchantTransferStateSuccess,
	}, nil)
	store.EXPECT().MarkRecoveryDisputeCompensated(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkRecoveryDisputeCompensatedParams) error {
			require.Equal(t, int64(21), arg.ID)
			require.True(t, arg.CompensatedAt.Valid)
			return nil
		},
	)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			return nil
		},
	)

	err = executeRecoveryDisputeCompensation(context.Background(), store, nil, transferClient, ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID:    21,
		Status:               "approved",
		CompensationActionID: action.ID,
	})
	require.NoError(t, err)
}

func TestExecuteRecoveryDisputeCompensation_SkipsNonApprovedPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	err := executeRecoveryDisputeCompensation(context.Background(), store, nil, nil, ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID:    21,
		Status:               "rejected",
		CompensationActionID: 109,
	})
	require.NoError(t, err)
}

func TestResumeClaimRecovery_RestoresOverdueWhenDueAtPassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	recovery := db.ClaimRecovery{
		ID:             66,
		ClaimID:        55,
		Status:         "disputed",
		DueAt:          time.Now().Add(-time.Hour),
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	updated := recovery
	updated.Status = "overdue"

	store.EXPECT().GetClaimRecoveryByClaimIDAndTarget(gomock.Any(), recoveryDisputeResultRecoveryQuery(55, "merchant")).Return(recovery, nil)
	store.EXPECT().ResumeClaimRecoveryAfterDispute(gomock.Any(), recovery.ID).Return(updated, nil)

	err := resumeClaimRecoveryAfterRecoveryDispute(context.Background(), store, ProcessRecoveryDisputeResultPayload{
		ClaimID:        55,
		RecoveryTarget: "merchant",
		Status:         "rejected",
	})
	require.NoError(t, err)
}

func TestResumeClaimRecovery_SkipsWhenRecoveryNotDisputed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().GetClaimRecoveryByClaimIDAndTarget(gomock.Any(), recoveryDisputeResultRecoveryQuery(55, "merchant")).Return(db.ClaimRecovery{
		ID:             66,
		ClaimID:        55,
		Status:         "pending",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}, nil)

	err := resumeClaimRecoveryAfterRecoveryDispute(context.Background(), store, ProcessRecoveryDisputeResultPayload{
		ClaimID:        55,
		RecoveryTarget: "merchant",
		Status:         "rejected",
	})
	require.NoError(t, err)
}

func TestResumeClaimRecovery_ReturnsLookupFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().GetClaimRecoveryByClaimIDAndTarget(gomock.Any(), recoveryDisputeResultRecoveryQuery(55, "merchant")).Return(db.ClaimRecovery{}, errors.New("lookup unavailable"))

	err := resumeClaimRecoveryAfterRecoveryDispute(context.Background(), store, ProcessRecoveryDisputeResultPayload{
		ClaimID:        55,
		RecoveryTarget: "merchant",
		Status:         "rejected",
	})
	require.ErrorContains(t, err, "get claim recovery by claim id")
}

func TestProcessRecoveryDisputeResult_ApprovedRequiresReleaseActionWhenRecoveryExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetClaimRecoveryByClaimIDAndTarget(gomock.Any(), recoveryDisputeResultRecoveryQuery(55, "merchant")).Return(db.ClaimRecovery{
		ID:             66,
		ClaimID:        55,
		Status:         "waived",
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}, nil)

	err := processor.processRecoveryDisputeResult(context.Background(), ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID:  77,
		ClaimID:            55,
		Status:             "approved",
		AppellantType:      "merchant",
		AppellantID:        88,
		ClaimantUserID:     99,
		CompensationAmount: 0,
	})
	require.ErrorContains(t, err, "missing release action id")
}

func TestProcessRecoveryDisputeResult_ApprovedReturnsReleaseActionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetBehaviorAction(gomock.Any(), int64(88)).Return(db.BehaviorAction{}, errors.New("release unavailable"))

	err := processor.processRecoveryDisputeResult(context.Background(), ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID: 77,
		ClaimID:           55,
		ReleaseActionID:   88,
		Status:            "approved",
		AppellantType:     "merchant",
		AppellantID:       99,
		ClaimantUserID:    100,
	})
	require.ErrorContains(t, err, "execute claim recovery release action for recovery dispute 77")
}

func TestProcessRecoveryDisputeResult_RejectedReturnsResumeFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetClaimRecoveryByClaimIDAndTarget(gomock.Any(), recoveryDisputeResultRecoveryQuery(55, "rider")).Return(db.ClaimRecovery{
		ID:             66,
		ClaimID:        55,
		Status:         "disputed",
		RecoveryTarget: pgtype.Text{String: "rider", Valid: true},
	}, nil)
	store.EXPECT().ResumeClaimRecoveryAfterDispute(gomock.Any(), int64(66)).Return(db.ClaimRecovery{}, errors.New("resume unavailable"))

	err := processor.processRecoveryDisputeResult(context.Background(), ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID: 77,
		ClaimID:           55,
		RecoveryTarget:    "rider",
		Status:            "rejected",
	})
	require.ErrorContains(t, err, "resume claim recovery after rejected recovery dispute 77")
}

func TestProcessRecoveryDisputeResult_ApprovedReturnsPenaltyFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: int64(100)}).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{ConfigKey: "behavior_trace.reject_service_cooldown_days", ScopeType: "global", ScopeID: pgtype.Int8{Valid: false}}).Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, errors.New("blocklist unavailable"))

	err := processor.processRecoveryDisputeResult(context.Background(), ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID: 77,
		Status:            "approved",
		AppellantType:     "merchant",
		AppellantID:       99,
		ClaimantUserID:    100,
	})
	require.ErrorContains(t, err, "penalize claimant for approved recovery dispute 77")
}

func TestPenalizeRecoveryDisputeClaimant_IgnoresWarningWriteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: int64(100)}).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{ConfigKey: "behavior_trace.reject_service_cooldown_days", ScopeType: "global", ScopeID: pgtype.Int8{Valid: false}}).Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{ID: 1}, nil)
	store.EXPECT().GetUserClaimWarningStatus(gomock.Any(), int64(100)).Return(db.UserClaimWarning{}, db.ErrRecordNotFound)
	store.EXPECT().CreateUserClaimWarning(gomock.Any(), gomock.Any()).Return(db.UserClaimWarning{}, errors.New("warning unavailable"))

	err := penalizeRecoveryDisputeClaimant(context.Background(), store, ProcessRecoveryDisputeResultPayload{
		ClaimantUserID: 100,
	})
	require.NoError(t, err)
}
