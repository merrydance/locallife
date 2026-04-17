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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskClaimPayout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

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
	transferClient.EXPECT().GetAppID().Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, claimPayoutOutBillNo(action.ID), persisted.OutBillNo)
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error) {
			require.Equal(t, int64(3300), req.TransferAmount)
			require.Equal(t, "openid-22", req.OpenID)
			require.Equal(t, "张三", req.UserName)
			require.Equal(t, "platform payout", req.TransferRemark)
			require.Equal(t, wechatcontracts.DirectMerchantTransferSceneEnterpriseCompensation, req.TransferSceneID)
			require.Equal(t, claimPayoutOutBillNo(action.ID), req.OutBillNo)
			return &wechatcontracts.DirectMerchantTransferCreateResponse{OutBillNo: req.OutBillNo, TransferBillNo: "wx-bill-99", State: wechatcontracts.DirectMerchantTransferStateAccepted, CreateTime: "2026-03-27T10:00:00+08:00"}, nil
		},
	)
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      claimPayoutOutBillNo(action.ID),
		TransferBillNo: "wx-bill-99",
		State:          wechatcontracts.DirectMerchantTransferStateSuccess,
		CreateTime:     "2026-03-27T10:00:00+08:00",
		UpdateTime:     "2026-03-27T10:01:00+08:00",
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
			require.Equal(t, wechatcontracts.DirectMerchantTransferStateSuccess, persisted.TransferState)
			require.Equal(t, "wx-bill-99", persisted.TransferBillNo)
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
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

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
	transferClient.EXPECT().GetAppID().Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			return nil
		},
	)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).Return(nil, errors.New("wx down"))
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

func TestProcessTaskClaimPayout_WechatCreateTransferErrorsPersistRetryableDetail(t *testing.T) {
	testCases := []struct {
		name         string
		wxErr        *wechat.WechatPayError
		expectInLast string
	}{
		{
			name:         "NoAuth",
			wxErr:        &wechat.WechatPayError{StatusCode: 403, Code: "NO_AUTH", Message: "权限不足"},
			expectInLast: "NO_AUTH",
		},
		{
			name:         "SystemError",
			wxErr:        &wechat.WechatPayError{StatusCode: 500, Code: "SYSTEM_ERROR", Message: "系统错误"},
			expectInLast: "SYSTEM_ERROR",
		},
		{
			name:         "FrequencyLimited",
			wxErr:        &wechat.WechatPayError{StatusCode: 429, Code: "FREQUENCY_LIMITED", Message: "请求频繁"},
			expectInLast: "FREQUENCY_LIMITED",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
			processor := NewTestTaskProcessor(store, nil, nil, nil)
			processor.SetTransferClient(transferClient)

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
			transferClient.EXPECT().GetAppID().Return("wx-mini-app")
			store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
					require.Equal(t, action.ID, arg.ID)
					require.Equal(t, "running", arg.Status)
					var persisted claimPayoutActionDetail
					require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
					require.Equal(t, claimPayoutOutBillNo(action.ID), persisted.OutBillNo)
					require.Empty(t, persisted.LastError)
					require.False(t, persisted.TerminalFailure)
					return nil
				},
			)
			transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).Return(nil, tc.wxErr)
			store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
					require.Equal(t, action.ID, arg.ID)
					require.Equal(t, "failed", arg.Status)
					var persisted claimPayoutActionDetail
					require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
					require.Equal(t, claimPayoutOutBillNo(action.ID), persisted.OutBillNo)
					require.Contains(t, persisted.LastError, tc.expectInLast)
					require.False(t, persisted.TerminalFailure)
					require.Empty(t, persisted.TransferBillNo)
					return nil
				},
			)

			payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
			require.NoError(t, err)

			err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
			require.Error(t, err)
			require.Contains(t, err.Error(), "create claim payout transfer")
			require.ErrorIs(t, err, tc.wxErr)
		})
	}
}

func TestProcessTaskClaimPayout_WithoutTransferClientRemainsRetryable(t *testing.T) {
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
			require.Contains(t, persisted.LastError, "transfer client is not configured")
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.Error(t, err)
	require.Contains(t, err.Error(), "transfer client is not configured for claim payout")
}

func TestProcessTaskClaimPayout_DuplicateTransferStillMarksClaimPaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

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
	transferClient.EXPECT().GetAppID().Return("wx-mini-app")
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			return nil
		},
	)
	transferClient.EXPECT().CreateTransfer(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{Code: "ALREADY_EXISTS", Message: "duplicate", StatusCode: 400})
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      claimPayoutOutBillNo(action.ID),
		TransferBillNo: "wx-bill-99",
		State:          wechatcontracts.DirectMerchantTransferStateSuccess,
	}, nil)
	store.EXPECT().MarkClaimPaid(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.True(t, arg.ExecutedAt.Valid)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, wechatcontracts.DirectMerchantTransferStateSuccess, persisted.TransferState)
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
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

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
			require.Equal(t, "appeal compensation", req.TransferRemark)
			return &wechatcontracts.DirectMerchantTransferCreateResponse{OutBillNo: req.OutBillNo, TransferBillNo: "wx-bill-109", State: wechatcontracts.DirectMerchantTransferStateAccepted}, nil
		},
	)
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      claimPayoutOutBillNo(action.ID),
		TransferBillNo: "wx-bill-109",
		State:          wechatcontracts.DirectMerchantTransferStateSuccess,
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
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:        11,
		UserID:         22,
		Amount:         3300,
		Remark:         "platform payout",
		OutBillNo:      claimPayoutOutBillNo(99),
		TransferBillNo: "wx-bill-99",
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
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
		OutBillNo:      claimPayoutOutBillNo(action.ID),
		TransferBillNo: "wx-bill-99",
		State:          wechatcontracts.DirectMerchantTransferStateSuccess,
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

func TestProcessTaskClaimPayout_RunningActionQueryWechatErrorsPersistRetryableDetail(t *testing.T) {
	testCases := []struct {
		name         string
		wxErr        *wechat.WechatPayError
		expectInLast string
	}{
		{
			name:         "NoAuth",
			wxErr:        &wechat.WechatPayError{StatusCode: 403, Code: "NO_AUTH", Message: "权限不足"},
			expectInLast: "NO_AUTH",
		},
		{
			name:         "SystemError",
			wxErr:        &wechat.WechatPayError{StatusCode: 500, Code: "SYSTEM_ERROR", Message: "系统错误"},
			expectInLast: "SYSTEM_ERROR",
		},
		{
			name:         "FrequencyLimited",
			wxErr:        &wechat.WechatPayError{StatusCode: 429, Code: "FREQUENCY_LIMITED", Message: "请求频繁"},
			expectInLast: "FREQUENCY_LIMITED",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
			processor := NewTestTaskProcessor(store, nil, nil, nil)
			processor.SetTransferClient(transferClient)

			detailBytes, err := json.Marshal(claimPayoutActionDetail{
				ClaimID:        11,
				UserID:         22,
				Amount:         3300,
				Remark:         "platform payout",
				OutBillNo:      claimPayoutOutBillNo(99),
				TransferBillNo: "wx-bill-99",
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
			transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(nil, tc.wxErr)
			store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
					require.Equal(t, action.ID, arg.ID)
					require.Equal(t, "running", arg.Status)
					var persisted claimPayoutActionDetail
					require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
					require.Equal(t, claimPayoutOutBillNo(action.ID), persisted.OutBillNo)
					require.Equal(t, "wx-bill-99", persisted.TransferBillNo)
					require.Contains(t, persisted.LastError, tc.expectInLast)
					require.False(t, persisted.TerminalFailure)
					return nil
				},
			)

			payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
			require.NoError(t, err)

			err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
			require.Error(t, err)
			require.Contains(t, err.Error(), "query claim payout transfer")
			require.ErrorIs(t, err, tc.wxErr)
		})
	}
}

func TestProcessTaskClaimPayout_RunningActionQueryNotFoundKeepsActionRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetTransferClient(transferClient)

	detailBytes, err := json.Marshal(claimPayoutActionDetail{
		ClaimID:        11,
		UserID:         22,
		Amount:         3300,
		Remark:         "platform payout",
		OutBillNo:      claimPayoutOutBillNo(99),
		TransferBillNo: "wx-bill-99",
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
	transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(nil, &wechat.WechatPayError{StatusCode: 404, Code: "NOT_FOUND", Message: "单据不存在"})
	store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
			require.Equal(t, action.ID, arg.ID)
			require.Equal(t, "running", arg.Status)
			var persisted claimPayoutActionDetail
			require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
			require.Equal(t, claimPayoutOutBillNo(action.ID), persisted.OutBillNo)
			require.Equal(t, "wx-bill-99", persisted.TransferBillNo)
			require.Empty(t, persisted.LastError)
			require.False(t, persisted.TerminalFailure)
			return nil
		},
	)

	payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskClaimPayout_RunningActionTerminalFailurePersistsFailureReason(t *testing.T) {
	testCases := []struct {
		name              string
		state             string
		failReason        string
		expectedLastError string
	}{
		{
			name:              "FailWithReason",
			state:             wechatcontracts.DirectMerchantTransferStateFail,
			failReason:        wechatcontracts.DirectMerchantTransferFailReasonAccountFrozen,
			expectedLastError: "transfer failed: ACCOUNT_FROZEN",
		},
		{
			name:              "CancelledWithoutReason",
			state:             wechatcontracts.DirectMerchantTransferStateCancelled,
			failReason:        "",
			expectedLastError: "transfer reached terminal failure state: CANCELLED",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
			processor := NewTestTaskProcessor(store, nil, nil, nil)
			processor.SetTransferClient(transferClient)

			detailBytes, err := json.Marshal(claimPayoutActionDetail{
				ClaimID:        11,
				UserID:         22,
				Amount:         3300,
				Remark:         "platform payout",
				OutBillNo:      claimPayoutOutBillNo(99),
				TransferBillNo: "wx-bill-99",
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
			transferClient.EXPECT().QueryTransferByOutBillNo(gomock.Any(), claimPayoutOutBillNo(action.ID)).Return(&wechatcontracts.DirectMerchantTransferQueryResponse{
				OutBillNo:      claimPayoutOutBillNo(action.ID),
				TransferBillNo: "wx-bill-99",
				State:          tc.state,
				FailReason:     tc.failReason,
			}, nil)
			store.EXPECT().UpdateBehaviorActionExecution(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, arg db.UpdateBehaviorActionExecutionParams) error {
					require.Equal(t, action.ID, arg.ID)
					require.Equal(t, "failed", arg.Status)
					require.False(t, arg.ExecutedAt.Valid)
					var persisted claimPayoutActionDetail
					require.NoError(t, json.Unmarshal(arg.Detail, &persisted))
					require.Equal(t, tc.state, persisted.TransferState)
					require.Equal(t, tc.failReason, persisted.FailReason)
					require.Equal(t, tc.expectedLastError, persisted.LastError)
					require.True(t, persisted.TerminalFailure)
					return nil
				},
			)

			payloadBytes, err := json.Marshal(ClaimPayoutPayload{ActionID: action.ID})
			require.NoError(t, err)

			err = processor.ProcessTaskClaimPayout(context.Background(), asynq.NewTask(TaskClaimPayout, payloadBytes))
			require.NoError(t, err)
		})
	}
}
