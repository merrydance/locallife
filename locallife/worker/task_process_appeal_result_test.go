package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExecuteAppealCompensation_ApprovedExecutesPayoutAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetPaymentClient(paymentClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		AppealID:   21,
		UserID:     22,
		Amount:     1800,
		SourceType: "platform",
		Remark:     "appeal compensation",
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
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			return nil
		},
	)
	paymentClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *wechat.TransferRequest) (*wechat.TransferResponse, error) {
			require.Equal(t, int64(1800), req.TransferAmount)
			require.Equal(t, "openid-22", req.OpenID)
			return &wechat.TransferResponse{OutBatchNo: req.OutBatchNo, BatchID: "batch-109", BatchStatus: "ACCEPTED"}, nil
		},
	)
	paymentClient.EXPECT().QueryTransfer(gomock.Any(), claimPayoutOutBatchNo(action.ID)).Return(&wechat.TransferQueryResponse{
		OutBatchNo:  claimPayoutOutBatchNo(action.ID),
		BatchID:     "batch-109",
		BatchStatus: "FINISHED",
	}, nil)
	store.EXPECT().MarkAppealCompensated(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkAppealCompensatedParams) error {
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

	err = processor.executeAppealCompensation(context.Background(), ProcessAppealResultPayload{
		AppealID:             21,
		Status:               "approved",
		CompensationActionID: action.ID,
	})
	require.NoError(t, err)
}

func TestExecuteAppealCompensation_SkipsNonApprovedPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	err := processor.executeAppealCompensation(context.Background(), ProcessAppealResultPayload{
		AppealID:             21,
		Status:               "rejected",
		CompensationActionID: 109,
	})
	require.NoError(t, err)
}

func TestResumeClaimRecovery_RestoresOverdueWhenDueAtPassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	recovery := db.ClaimRecovery{
		ID:      66,
		ClaimID: 55,
		Status:  "appealed",
		DueAt:   time.Now().Add(-time.Hour),
	}
	updated := recovery
	updated.Status = "overdue"

	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), int64(55)).Return(recovery, nil)
	store.EXPECT().ResumeClaimRecoveryAfterAppeal(gomock.Any(), recovery.ID).Return(updated, nil)

	err := processor.resumeClaimRecovery(context.Background(), ProcessAppealResultPayload{
		ClaimID: 55,
		Status:  "rejected",
	})
	require.NoError(t, err)
}

func TestResumeClaimRecovery_SkipsWhenRecoveryNotAppealed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), int64(55)).Return(db.ClaimRecovery{
		ID:      66,
		ClaimID: 55,
		Status:  "pending",
	}, nil)

	err := processor.resumeClaimRecovery(context.Background(), ProcessAppealResultPayload{
		ClaimID: 55,
		Status:  "rejected",
	})
	require.NoError(t, err)
}

func TestRollbackClaimRecovery_SkipsWhenRecoveryNotAppealed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), int64(55)).Return(db.ClaimRecovery{
		ID:      66,
		ClaimID: 55,
		Status:  "waived",
	}, nil)

	err := processor.rollbackClaimRecovery(context.Background(), ProcessAppealResultPayload{
		ClaimID:            55,
		Status:             "approved",
		ClaimantUserID:     99,
		CompensationAmount: 0,
	})
	require.NoError(t, err)
}

func TestRollbackClaimRecovery_ApprovedWaivesRecoveryWithoutReversingClaimantPayout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	store.EXPECT().GetClaimRecoveryByClaimID(gomock.Any(), int64(55)).Return(db.ClaimRecovery{
		ID:      66,
		ClaimID: 55,
		Status:  "appealed",
	}, nil)
	store.EXPECT().MarkClaimRecoveryWaived(gomock.Any(), int64(66)).Return(db.ClaimRecovery{
		ID:      66,
		ClaimID: 55,
		Status:  "waived",
	}, nil)

	err := processor.rollbackClaimRecovery(context.Background(), ProcessAppealResultPayload{
		ClaimID:            55,
		Status:             "approved",
		ClaimantUserID:     99,
		CompensationAmount: 0,
	})
	require.NoError(t, err)
}
