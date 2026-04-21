package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type merchantWithdrawRecoverySchedulerTestDistributor struct {
	worker.NoopTaskDistributor
	payloads []*worker.MerchantWithdrawResultPayload
}

func (d *merchantWithdrawRecoverySchedulerTestDistributor) DistributeTaskProcessMerchantWithdrawResult(ctx context.Context, payload *worker.MerchantWithdrawResultPayload, opts ...asynq.Option) error {
	d.payloads = append(d.payloads, payload)
	return nil
}

func TestMerchantWithdrawRecoverySchedulerRunOnceEnqueuesPendingMerchantRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantWithdrawRecoverySchedulerTestDistributor{}

	merchantRecord := db.WithdrawalRecord{ID: 301, Channel: "wechat_ecommerce_fund", Status: "pending"}

	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund",
			Limit:   int32(200),
		}).
		Return([]db.WithdrawalRecord{merchantRecord}, nil)

	scheduler := worker.NewMerchantWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
	require.Len(t, distributor.payloads, 1)
	require.Equal(t, merchantRecord.ID, distributor.payloads[0].WithdrawalRecordID)
	require.Zero(t, distributor.payloads[0].RetryCount)
}

func TestMerchantWithdrawRecoverySchedulerRunOnceReturnsAfterMerchantChannelFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantWithdrawRecoverySchedulerTestDistributor{}

	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund",
			Limit:   int32(200),
		}).
		Return(nil, assertAnError("merchant channel unavailable"))

	scheduler := worker.NewMerchantWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
	require.Empty(t, distributor.payloads)
}

type schedulerTestError string

func (e schedulerTestError) Error() string {
	return string(e)
}

func assertAnError(message string) error {
	return schedulerTestError(message)
}
