package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskMerchantCancelWithdrawResultSyncsTerminalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	record := db.MerchantCancelWithdrawApplication{
		ID:             1001,
		OutRequestNo:   "MCW1001",
		LocalSyncState: db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
	}

	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceCancelWithdrawByOutRequestNo(gomock.Any(), record.OutRequestNo).Return(&wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:             "WX-CANCEL-1001",
		OutRequestNo:            record.OutRequestNo,
		CancelState:             db.MerchantCancelStateFinish,
		CancelStateDescription:  "完成",
		WithdrawState:           db.MerchantCancelWithdrawStateSucceed,
		WithdrawStateDescription:"提现成功",
	}, nil)
	store.EXPECT().
		UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantCancelWithdrawApplicationSyncParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
			require.Equal(t, db.MerchantCancelStateFinish, arg.CancelState.String)
			require.Equal(t, db.MerchantCancelWithdrawStateSucceed, arg.WithdrawState.String)
			require.Equal(t, "WX-CANCEL-1001", arg.ApplymentID.String)
			require.True(t, arg.LastQueryAt.Valid)
			return db.MerchantCancelWithdrawApplication{
				ID:                       record.ID,
				OutRequestNo:             record.OutRequestNo,
				ApplymentID:              pgtype.Text{String: "WX-CANCEL-1001", Valid: true},
				LocalSyncState:           db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded,
				CancelState:              pgtype.Text{String: db.MerchantCancelStateFinish, Valid: true},
				CancelStateDescription:   pgtype.Text{String: "完成", Valid: true},
				WithdrawState:            pgtype.Text{String: db.MerchantCancelWithdrawStateSucceed, Valid: true},
				WithdrawStateDescription: pgtype.Text{String: "提现成功", Valid: true},
			}, nil
		})

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.MerchantCancelWithdrawResultPayload{ApplicationID: record.ID, RetryCount: 0})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessMerchantCancelWithdrawResult, payloadBytes)
	err = processor.ProcessTaskMerchantCancelWithdrawResult(context.Background(), task)
	require.NoError(t, err)
}