package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskBaofuWithdrawalFactApplicationMapsReturnedState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	withdrawal := db.BaofuWithdrawalOrder{ID: 77, OutRequestNo: "WD_RETURNED", Status: db.BaofuWithdrawalStatusProcessing}
	updated := withdrawal
	updated.Status = db.BaofuWithdrawalStatusReturned

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), db.UpdateBaofuWithdrawalOrderStatusParams{
		ID:              withdrawal.ID,
		Status:          db.BaofuWithdrawalStatusReturned,
		BaofuWithdrawNo: pgtype.Text{String: "BF_RETURNED", Valid: true},
		RawSnapshot:     []byte(`{"state":"3"}`),
	}).Return(updated, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawal.ID,
		UpstreamState:     "3",
		BaofuWithdrawNo:   "BF_RETURNED",
		RawSnapshot:       []byte(`{"state":"3"}`),
	})
	require.NoError(t, err)

	err = processor.ProcessTaskBaofuWithdrawalFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalFactApplication, payload))
	require.NoError(t, err)
}
