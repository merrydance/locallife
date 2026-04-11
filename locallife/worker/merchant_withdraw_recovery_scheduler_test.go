package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMerchantWithdrawRecoverySchedulerRunOnceEnqueuesPendingRecordsAcrossChannels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	merchantRecord := db.WithdrawalRecord{ID: 301, Channel: "wechat_ecommerce_fund", Status: "pending"}
	operatorRecord := db.WithdrawalRecord{ID: 302, Channel: "wechat_ecommerce_fund_operator", Status: "pending"}

	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund",
			Limit:   int32(200),
		}).
		Return([]db.WithdrawalRecord{merchantRecord}, nil)
	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund_operator",
			Limit:   int32(200),
		}).
		Return([]db.WithdrawalRecord{operatorRecord}, nil)

	distributor.EXPECT().
		DistributeTaskProcessMerchantWithdrawResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.MerchantWithdrawResultPayload{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.MerchantWithdrawResultPayload, _ ...asynq.Option) error {
			require.Equal(t, merchantRecord.ID, payload.WithdrawalRecordID)
			require.Zero(t, payload.RetryCount)
			return nil
		})
	distributor.EXPECT().
		DistributeTaskProcessMerchantWithdrawResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.MerchantWithdrawResultPayload{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.MerchantWithdrawResultPayload, _ ...asynq.Option) error {
			require.Equal(t, operatorRecord.ID, payload.WithdrawalRecordID)
			require.Zero(t, payload.RetryCount)
			return nil
		})

	scheduler := worker.NewMerchantWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestMerchantWithdrawRecoverySchedulerRunOnceContinuesAfterSingleChannelFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	operatorRecord := db.WithdrawalRecord{ID: 401, Channel: "wechat_ecommerce_fund_operator", Status: "pending"}

	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund",
			Limit:   int32(200),
		}).
		Return(nil, assertAnError("merchant channel unavailable"))
	store.EXPECT().
		ListPendingWithdrawalRecordsByChannel(gomock.Any(), db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: "wechat_ecommerce_fund_operator",
			Limit:   int32(200),
		}).
		Return([]db.WithdrawalRecord{operatorRecord}, nil)

	distributor.EXPECT().
		DistributeTaskProcessMerchantWithdrawResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.MerchantWithdrawResultPayload{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.MerchantWithdrawResultPayload, _ ...asynq.Option) error {
			require.Equal(t, operatorRecord.ID, payload.WithdrawalRecordID)
			return nil
		})

	scheduler := worker.NewMerchantWithdrawRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

type schedulerTestError string

func (e schedulerTestError) Error() string {
	return string(e)
}

func assertAnError(message string) error {
	return schedulerTestError(message)
}