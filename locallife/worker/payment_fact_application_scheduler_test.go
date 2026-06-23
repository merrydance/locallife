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

func TestPaymentFactApplicationSchedulerRunOnceEnqueuesConfiguredTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentFactApplicationSchedulerTestDistributor{}

	expectedTargets := []struct {
		consumer           string
		businessObjectType string
		applicationID      int64
	}{
		{consumer: "profit_sharing_domain", businessObjectType: "profit_sharing_order", applicationID: 801},
		{consumer: "profit_sharing_domain", businessObjectType: "profit_sharing_return", applicationID: 802},
		{consumer: "claim_recovery_domain", businessObjectType: "payment_order", applicationID: 803},
		{consumer: "rider_deposit_domain", businessObjectType: "payment_order", applicationID: 804},
		{consumer: "baofu_account_verify_fee_domain", businessObjectType: "payment_order", applicationID: 805},
		{consumer: "order_domain", businessObjectType: "payment_order", applicationID: 806},
		{consumer: "reservation_domain", businessObjectType: "payment_order", applicationID: 807},
		{consumer: "order_domain", businessObjectType: "refund_order", applicationID: 808},
		{consumer: "reservation_domain", businessObjectType: "refund_order", applicationID: 809},
		{consumer: "rider_deposit_domain", businessObjectType: "refund_order", applicationID: 810},
	}
	calls := make([]any, 0, len(expectedTargets))
	for _, expected := range expectedTargets {
		expected := expected
		calls = append(calls, store.EXPECT().ReclaimStaleExternalPaymentFactApplicationsByTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStaleExternalPaymentFactApplicationsByTargetParams) ([]db.ExternalPaymentFactApplication, error) {
			require.Equal(t, expected.consumer, arg.Consumer)
			require.Equal(t, expected.businessObjectType, arg.BusinessObjectType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}))
		calls = append(calls, store.EXPECT().ListRetryableExternalPaymentFactApplicationsByTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListRetryableExternalPaymentFactApplicationsByTargetParams) ([]db.ExternalPaymentFactApplication, error) {
			require.Equal(t, expected.consumer, arg.Consumer)
			require.Equal(t, expected.businessObjectType, arg.BusinessObjectType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.ExternalPaymentFactApplication{{
				ID:                 expected.applicationID,
				FactID:             expected.applicationID - 100,
				Consumer:           expected.consumer,
				BusinessObjectType: expected.businessObjectType,
				BusinessObjectID:   expected.applicationID + 2000,
				Status:             db.ExternalPaymentFactApplicationStatusPending,
			}}, nil
		}))
	}
	gomock.InOrder(calls...)

	scheduler := worker.NewPaymentFactApplicationScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{801, 802, 803, 804, 805, 806, 807, 808, 809, 810}, distributor.applicationIDs)
	require.Len(t, distributor.optionCounts, len(expectedTargets))
	for _, optionCount := range distributor.optionCounts {
		require.GreaterOrEqual(t, optionCount, 3)
	}
}

func TestPaymentFactApplicationSchedulerRunOnceReclaimsStaleProcessingApplications(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentFactApplicationSchedulerTestDistributor{}

	expectedTargets := []struct {
		consumer           string
		businessObjectType string
		applicationID      int64
	}{
		{consumer: "profit_sharing_domain", businessObjectType: "profit_sharing_order", applicationID: 901},
		{consumer: "profit_sharing_domain", businessObjectType: "profit_sharing_return"},
		{consumer: "claim_recovery_domain", businessObjectType: "payment_order"},
		{consumer: "rider_deposit_domain", businessObjectType: "payment_order"},
		{consumer: "baofu_account_verify_fee_domain", businessObjectType: "payment_order"},
		{consumer: "order_domain", businessObjectType: "payment_order"},
		{consumer: "reservation_domain", businessObjectType: "payment_order"},
		{consumer: "order_domain", businessObjectType: "refund_order"},
		{consumer: "reservation_domain", businessObjectType: "refund_order"},
		{consumer: "rider_deposit_domain", businessObjectType: "refund_order"},
	}

	calls := make([]any, 0, len(expectedTargets)*2)
	for _, expected := range expectedTargets {
		expected := expected
		calls = append(calls, store.EXPECT().ReclaimStaleExternalPaymentFactApplicationsByTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStaleExternalPaymentFactApplicationsByTargetParams) ([]db.ExternalPaymentFactApplication, error) {
			require.Equal(t, expected.consumer, arg.Consumer)
			require.Equal(t, expected.businessObjectType, arg.BusinessObjectType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			if expected.applicationID == 0 {
				return nil, nil
			}
			return []db.ExternalPaymentFactApplication{{
				ID:                 expected.applicationID,
				FactID:             expected.applicationID + 100,
				Consumer:           expected.consumer,
				BusinessObjectType: expected.businessObjectType,
				BusinessObjectID:   expected.applicationID + 2000,
				Status:             db.ExternalPaymentFactApplicationStatusFailed,
			}}, nil
		}))
		calls = append(calls, store.EXPECT().ListRetryableExternalPaymentFactApplicationsByTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListRetryableExternalPaymentFactApplicationsByTargetParams) ([]db.ExternalPaymentFactApplication, error) {
			require.Equal(t, expected.consumer, arg.Consumer)
			require.Equal(t, expected.businessObjectType, arg.BusinessObjectType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			if expected.applicationID == 0 {
				return nil, nil
			}
			return []db.ExternalPaymentFactApplication{{
				ID:                 expected.applicationID,
				FactID:             expected.applicationID + 100,
				Consumer:           expected.consumer,
				BusinessObjectType: expected.businessObjectType,
				BusinessObjectID:   expected.applicationID + 2000,
				Status:             db.ExternalPaymentFactApplicationStatusFailed,
			}}, nil
		}))
	}
	gomock.InOrder(calls...)

	scheduler := worker.NewPaymentFactApplicationScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{901}, distributor.applicationIDs)
}

func TestPaymentFactApplicationSchedulerRunOnceSkipsWithoutDistributor(t *testing.T) {
	scheduler := worker.NewPaymentFactApplicationScheduler(nil, nil)
	scheduler.RunOnce()
}

type paymentFactApplicationSchedulerTestDistributor struct {
	applicationIDs []int64
	optionCounts   []int
}

func (d *paymentFactApplicationSchedulerTestDistributor) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	d.applicationIDs = append(d.applicationIDs, payload.ApplicationID)
	d.optionCounts = append(d.optionCounts, len(opts))
	return nil
}
