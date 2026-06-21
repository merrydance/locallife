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

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().ApplyBaofuWithdrawalTerminalStatusTx(gomock.Any(), db.ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: withdrawal.ID,
		Status:            db.BaofuWithdrawalStatusReturned,
		BaofuWithdrawNo:   pgtype.Text{String: "BF_RETURNED", Valid: true},
		RawSnapshot:       []byte(`{"state":"3"}`),
	}).Return(db.ApplyBaofuWithdrawalTerminalStatusTxResult{
		WithdrawalOrder: db.BaofuWithdrawalOrder{ID: withdrawal.ID, Status: db.BaofuWithdrawalStatusReturned},
		Applied:         true,
	}, nil)

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

func TestProcessTaskBaofuWithdrawalFactApplicationConsumesSucceededReservation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	withdrawal := db.BaofuWithdrawalOrder{ID: 79, OutRequestNo: "WD_SUCCEEDED", Status: db.BaofuWithdrawalStatusProcessing}

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().ApplyBaofuWithdrawalTerminalStatusTx(gomock.Any(), db.ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: withdrawal.ID,
		Status:            db.BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo:   pgtype.Text{String: "BF_SUCCEEDED", Valid: true},
		RawSnapshot:       []byte(`{"state":"1"}`),
	}).Return(db.ApplyBaofuWithdrawalTerminalStatusTxResult{
		WithdrawalOrder: db.BaofuWithdrawalOrder{ID: withdrawal.ID, Status: db.BaofuWithdrawalStatusSucceeded},
		Applied:         true,
	}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawal.ID,
		UpstreamState:     "1",
		BaofuWithdrawNo:   "BF_SUCCEEDED",
		RawSnapshot:       []byte(`{"state":"1"}`),
	})
	require.NoError(t, err)

	err = processor.ProcessTaskBaofuWithdrawalFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskBaofuWithdrawalFactApplicationKeepsProcessingWithoutReservationSettlement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	withdrawal := db.BaofuWithdrawalOrder{ID: 80, OutRequestNo: "WD_PROCESSING", Status: db.BaofuWithdrawalStatusProcessing}
	updated := withdrawal
	updated.BaofuWithdrawNo = pgtype.Text{String: "BF_PROCESSING", Valid: true}

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID:              withdrawal.ID,
		BaofuWithdrawNo: pgtype.Text{String: "BF_PROCESSING", Valid: true},
		RawSnapshot:     []byte(`{"state":"2"}`),
	}).Return(updated, nil)
	store.EXPECT().ApplyBaofuWithdrawalTerminalStatusTx(gomock.Any(), gomock.Any()).Times(0)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawal.ID,
		UpstreamState:     "2",
		BaofuWithdrawNo:   "BF_PROCESSING",
		RawSnapshot:       []byte(`{"state":"2"}`),
	})
	require.NoError(t, err)

	err = processor.ProcessTaskBaofuWithdrawalFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskBaofuWithdrawalFactApplicationReplaysSameTerminalOrderThroughReservationTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	withdrawal := db.BaofuWithdrawalOrder{ID: 78, OutRequestNo: "WD_TERMINAL", Status: db.BaofuWithdrawalStatusSucceeded}

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().ApplyBaofuWithdrawalTerminalStatusTx(gomock.Any(), db.ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: withdrawal.ID,
		Status:            db.BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo:   pgtype.Text{String: "BF_SUCCEEDED", Valid: true},
		RawSnapshot:       []byte(`{"state":"1"}`),
	}).Return(db.ApplyBaofuWithdrawalTerminalStatusTxResult{
		WithdrawalOrder: withdrawal,
		Applied:         false,
	}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawal.ID,
		UpstreamState:     "1",
		BaofuWithdrawNo:   "BF_SUCCEEDED",
		RawSnapshot:       []byte(`{"state":"1"}`),
	})
	require.NoError(t, err)

	err = processor.ProcessTaskBaofuWithdrawalFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskBaofuWithdrawalFactApplicationSurfacesConflictingTerminalOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	withdrawal := db.BaofuWithdrawalOrder{ID: 81, OutRequestNo: "WD_TERMINAL_CONFLICT", Status: db.BaofuWithdrawalStatusSucceeded}

	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().ApplyBaofuWithdrawalTerminalStatusTx(gomock.Any(), db.ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: withdrawal.ID,
		Status:            db.BaofuWithdrawalStatusFailed,
		BaofuWithdrawNo:   pgtype.Text{String: "BF_FAILED", Valid: true},
		RawSnapshot:       []byte(`{"state":"0"}`),
	}).Return(db.ApplyBaofuWithdrawalTerminalStatusTxResult{}, db.ErrBaofuWithdrawalTerminalReservationMismatch)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawal.ID,
		UpstreamState:     "0",
		BaofuWithdrawNo:   "BF_FAILED",
		RawSnapshot:       []byte(`{"state":"0"}`),
	})
	require.NoError(t, err)

	err = processor.ProcessTaskBaofuWithdrawalFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalFactApplication, payload))
	require.ErrorIs(t, err, db.ErrBaofuWithdrawalTerminalReservationMismatch)
	require.ErrorContains(t, err, "apply baofu withdrawal terminal status")
}
