package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuWithdrawalRecoverySchedulerQueriesProcessingWithdrawals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawalRecoveryClient{
		withdrawResult: &baofucontracts.WithdrawResult{
			TransSerialNo:   "WD_OUT_001",
			BaofuWithdrawNo: "BF_WD_001",
			UpstreamState:   "3",
			Status:          db.BaofuWithdrawalStatusReturned,
			Raw:             []byte(`{"state":"3"}`),
		},
	}
	distributor := &baofuWithdrawalRecoveryDistributor{}
	withdrawalCreatedAt := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
	withdrawal := db.BaofuWithdrawalOrder{
		ID:           901,
		OutRequestNo: "WD_OUT_001",
		Status:       db.BaofuWithdrawalStatusProcessing,
		CreatedAt:    withdrawalCreatedAt,
	}
	store.EXPECT().ListProcessingBaofuWithdrawalOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.BaofuWithdrawalOrder{withdrawal}, nil)

	scheduler := worker.NewBaofuWithdrawalRecoveryScheduler(store, distributor, client, worker.BaofuWithdrawalRecoveryConfig{
		PayoutMerchantID: "PAYOUT_MER",
		PayoutTerminalID: "PAYOUT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, "PAYOUT_MER", client.queryReq.MerchantID)
	require.Equal(t, "PAYOUT_TER", client.queryReq.TerminalID)
	require.Equal(t, "WD_OUT_001", client.queryReq.TransSerialNo)
	require.Equal(t, "2026-05-05", client.queryReq.TradeTime)
	require.Equal(t, []int64{withdrawal.ID}, distributor.withdrawalOrderIDs)
	require.Equal(t, []string{"3"}, distributor.upstreamStates)
}

type baofuWithdrawalRecoveryClient struct {
	queryReq       baofucontracts.WithdrawQueryRequest
	withdrawResult *baofucontracts.WithdrawResult
}

func (c *baofuWithdrawalRecoveryClient) QueryWithdraw(_ context.Context, req baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error) {
	c.queryReq = req
	return c.withdrawResult, nil
}

type baofuWithdrawalRecoveryDistributor struct {
	worker.NoopTaskDistributor
	withdrawalOrderIDs []int64
	upstreamStates     []string
}

func (d *baofuWithdrawalRecoveryDistributor) DistributeTaskProcessBaofuWithdrawalFactApplication(_ context.Context, payload *worker.BaofuWithdrawalFactApplicationPayload, _ ...asynq.Option) error {
	d.withdrawalOrderIDs = append(d.withdrawalOrderIDs, payload.WithdrawalOrderID)
	d.upstreamStates = append(d.upstreamStates, payload.UpstreamState)
	return nil
}
