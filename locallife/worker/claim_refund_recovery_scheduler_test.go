package worker_test

import (
	"encoding/json"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClaimPayoutRecoverySchedulerRunOnceRecoversCreatedPayoutAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)

	recoverableDetail, err := json.Marshal(map[string]any{
		"claim_id":    int64(11),
		"user_id":     int64(22),
		"amount":      int64(3300),
		"source_type": "platform",
		"source_id":   int64(0),
		"remark":      "platform payout",
	})
	require.NoError(t, err)

	terminalDetail, err := json.Marshal(map[string]any{"terminal_failure": true})
	require.NoError(t, err)

	createdAction := db.BehaviorAction{
		ID:           99,
		DecisionID:   77,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "created",
		Detail:       recoverableDetail,
	}

	store.EXPECT().ListBehaviorActionsByStatusAndType(gomock.Any(), db.ListBehaviorActionsByStatusAndTypeParams{
		Status:       "created",
		ActionType:   "payout",
		TargetEntity: "user",
		Limit:        200,
	}).Return([]db.BehaviorAction{createdAction}, nil)
	store.EXPECT().ListBehaviorActionsByStatusAndType(gomock.Any(), db.ListBehaviorActionsByStatusAndTypeParams{
		Status:       "failed",
		ActionType:   "payout",
		TargetEntity: "user",
		Limit:        200,
	}).Return([]db.BehaviorAction{{
		ID:           100,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "failed",
		Detail:       terminalDetail,
	}}, nil)
	store.EXPECT().ListBehaviorActionsByStatusAndType(gomock.Any(), db.ListBehaviorActionsByStatusAndTypeParams{
		Status:       "running",
		ActionType:   "payout",
		TargetEntity: "user",
		Limit:        200,
	}).Return([]db.BehaviorAction{}, nil)

	store.EXPECT().GetBehaviorAction(gomock.Any(), createdAction.ID).Return(createdAction, nil)
	store.EXPECT().GetClaim(gomock.Any(), int64(11)).Return(db.Claim{ID: 11, Status: db.ClaimStatusApproved}, nil)
	store.EXPECT().GetUser(gomock.Any(), int64(22)).Return(db.User{ID: 22, WechatOpenid: "openid-22", FullName: "张三"}, nil)
	transferClient.EXPECT().GetAppID().Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, createdAction.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			return nil
		},
	)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req *contracts.DirectMerchantTransferCreateRequest) (*contracts.DirectMerchantTransferCreateResponse, error) {
			require.Equal(t, int64(3300), req.TransferAmount)
			require.Equal(t, "openid-22", req.OpenID)
			return &contracts.DirectMerchantTransferCreateResponse{
				OutBillNo:      req.OutBillNo,
				TransferBillNo: "wx-bill-99",
				State:          contracts.DirectMerchantTransferStateAccepted,
				CreateTime:     "2026-04-20T10:00:00+08:00",
			}, nil
		},
	)
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), "claimpayout99").Return(&contracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      "claimpayout99",
		TransferBillNo: "wx-bill-99",
		State:          contracts.DirectMerchantTransferStateSuccess,
		CreateTime:     "2026-04-20T10:00:00+08:00",
		UpdateTime:     "2026-04-20T10:01:00+08:00",
	}, nil)
	store.EXPECT().MarkClaimPaid(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().FinalizeClaimCompensationAfterPayoutTx(gomock.Any(), db.FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: 11}).Return(db.FinalizeClaimCompensationAfterPayoutTxResult{
		Claim: db.Claim{ID: 11},
	}, nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, createdAction.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			return nil
		},
	)

	scheduler := worker.NewClaimPayoutRecoveryScheduler(store, transferClient)
	scheduler.RunOnce()
}
