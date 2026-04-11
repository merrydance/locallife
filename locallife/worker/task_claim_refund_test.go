package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskClaimPayout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetPaymentClient(paymentClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:    11,
		UserID:     22,
		Amount:     3300,
		SourceType: "platform",
		SourceID:   0,
		Remark:     "platform payout",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           99,
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
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, claimPayoutOutBatchNo(action.ID), persisted.OutBatchNo)
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)
	paymentClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *wechat.TransferRequest) (*wechat.TransferResponse, error) {
			require.Equal(t, int64(3300), req.TransferAmount)
			require.Equal(t, "openid-22", req.OpenID)
			require.Equal(t, "张三", req.UserName)
			require.Equal(t, "platform payout", req.TransferRemark)
			require.Equal(t, "claim payout", req.BatchRemark)
			return &wechat.TransferResponse{OutBatchNo: req.OutBatchNo, BatchID: "batch-99", BatchStatus: "ACCEPTED"}, nil
		},
	)
	paymentClient.EXPECT().QueryTransfer(gomock.Any(), claimPayoutOutBatchNo(action.ID)).Return(&wechat.TransferQueryResponse{
		OutBatchNo:  claimPayoutOutBatchNo(action.ID),
		BatchID:     "batch-99",
		BatchStatus: "FINISHED",
		CreateTime:  "2026-03-27T10:00:00+08:00",
		UpdateTime:  "2026-03-27T10:01:00+08:00",
	}, nil)
	store.EXPECT().MarkClaimPaid(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkClaimPaidParams) error {
			require.Equal(t, int64(11), arg.ID)
			require.True(t, arg.PaidAt.Valid)
			return nil
		},
	)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, "FINISHED", persisted.BatchStatus)
			require.Equal(t, "batch-99", persisted.BatchID)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimPayout_FailureMarksActionFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetPaymentClient(paymentClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:    11,
		UserID:     22,
		Amount:     3300,
		SourceType: "platform",
		Remark:     "platform payout",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           99,
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
	paymentClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).Return(nil, errors.New("wx down"))
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Contains(t, persisted.LastError, "wx down")
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "create claim payout transfer")
}

func TestProcessTaskClaimPayout_WithoutPaymentClientRemainsRetryable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:    11,
		UserID:     22,
		Amount:     3300,
		SourceType: "platform",
		Remark:     "platform payout",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           99,
		DecisionID:   77,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "created",
		Detail:       detailBytes,
	}

	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Contains(t, persisted.LastError, "payment client is not configured")
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment client is not configured for claim payout")
}

func TestProcessTaskClaimPayout_DuplicateTransferStillMarksClaimPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetPaymentClient(paymentClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID: 11,
		UserID:  22,
		Amount:  3300,
		Remark:  "platform payout",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           99,
		DecisionID:   77,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "running",
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
	paymentClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{Code: "OUT_BATCH_NO_USED", Message: "duplicate", StatusCode: 409})
	paymentClient.EXPECT().QueryTransfer(gomock.Any(), claimPayoutOutBatchNo(action.ID)).Return(&wechat.TransferQueryResponse{
		OutBatchNo:  claimPayoutOutBatchNo(action.ID),
		BatchID:     "batch-99",
		BatchStatus: "FINISHED",
	}, nil)
	store.EXPECT().MarkClaimPaid(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, "FINISHED", persisted.BatchStatus)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimPayout_AppealCompensationSuccess(t *testing.T) {
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
			require.Equal(t, "appeal compensation", req.TransferRemark)
			require.Equal(t, "appeal compensation", req.BatchRemark)
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

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimPayout_RunningActionQueriesUntilFinished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetPaymentClient(paymentClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:    11,
		UserID:     22,
		Amount:     3300,
		Remark:     "platform payout",
		OutBatchNo: claimPayoutOutBatchNo(99),
		BatchID:    "batch-99",
	})
	require.NoError(t, err)

	action := db.BehaviorAction{
		ID:           99,
		DecisionID:   77,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "running",
		Detail:       detailBytes,
	}

	store.EXPECT().GetBehaviorAction(gomock.Any(), action.ID).Return(action, nil)
	paymentClient.EXPECT().QueryTransfer(gomock.Any(), claimPayoutOutBatchNo(action.ID)).Return(&wechat.TransferQueryResponse{
		OutBatchNo:  claimPayoutOutBatchNo(action.ID),
		BatchID:     "batch-99",
		BatchStatus: "FINISHED",
	}, nil)
	store.EXPECT().MarkClaimPaid(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.NoError(t, err)
}
