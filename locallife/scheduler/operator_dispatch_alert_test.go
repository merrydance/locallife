package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type operatorDispatchAlertTestDistributor struct {
	worker.NoopTaskDistributor
	alerts []*worker.OperatorPendingDispatchAlertPayload
	err    error
}

func (d *operatorDispatchAlertTestDistributor) DistributeTaskOperatorPendingDispatchAlert(_ context.Context, payload *worker.OperatorPendingDispatchAlertPayload, _ ...asynq.Option) error {
	if d.err != nil {
		return d.err
	}
	clone := *payload
	d.alerts = append(d.alerts, &clone)
	return nil
}

func TestDataCleanupScheduler_EnqueueOperatorPendingDispatchAlerts_QueuesNewAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &operatorDispatchAlertTestDistributor{}
	s := NewDataCleanupScheduler(store, distributor, nil)

	delivery := db.Delivery{ID: 101, OrderID: 201, Status: "pending", CreatedAt: time.Now().Add(-4 * time.Minute)}
	store.EXPECT().ListPendingDeliveriesBeforeWithoutAlert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingDeliveriesBeforeWithoutAlertParams) ([]db.Delivery, error) {
		require.Equal(t, "pending", arg.Status)
		require.Equal(t, operatorPendingDispatchAlertKey, arg.AlertKey)
		require.Equal(t, operatorPendingDispatchBatchLimit, arg.Limit)
		require.WithinDuration(t, time.Now().Add(-operatorPendingDispatchAlertThreshold), arg.CreatedAt, 2*time.Second)
		return []db.Delivery{delivery}, nil
	})
	store.EXPECT().CreateDeliveryTimeoutAlert(gomock.Any(), db.CreateDeliveryTimeoutAlertParams{
		DeliveryID: delivery.ID,
		AlertKey:   operatorPendingDispatchAlertKey,
	}).Return(db.DeliveryTimeoutAlert{ID: 1, DeliveryID: delivery.ID, AlertKey: operatorPendingDispatchAlertKey, CreatedAt: time.Now()}, nil)

	s.enqueueOperatorPendingDispatchAlerts()

	require.Len(t, distributor.alerts, 1)
	require.Equal(t, delivery.ID, distributor.alerts[0].DeliveryID)
	require.EqualValues(t, 3, distributor.alerts[0].ThresholdMinutes)
}

func TestDataCleanupScheduler_EnqueueOperatorPendingDispatchAlerts_SkipsDuplicateLedger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &operatorDispatchAlertTestDistributor{}
	s := NewDataCleanupScheduler(store, distributor, nil)

	delivery := db.Delivery{ID: 102, OrderID: 202, Status: "pending", CreatedAt: time.Now().Add(-5 * time.Minute)}
	store.EXPECT().ListPendingDeliveriesBeforeWithoutAlert(gomock.Any(), gomock.Any()).Return([]db.Delivery{delivery}, nil)
	store.EXPECT().CreateDeliveryTimeoutAlert(gomock.Any(), db.CreateDeliveryTimeoutAlertParams{
		DeliveryID: delivery.ID,
		AlertKey:   operatorPendingDispatchAlertKey,
	}).Return(db.DeliveryTimeoutAlert{}, db.ErrUniqueViolation)

	s.enqueueOperatorPendingDispatchAlerts()

	require.Empty(t, distributor.alerts)
}

func TestDataCleanupScheduler_EnqueueOperatorPendingDispatchAlerts_RollsBackLedgerWhenEnqueueFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &operatorDispatchAlertTestDistributor{err: errors.New("redis unavailable")}
	s := NewDataCleanupScheduler(store, distributor, nil)

	delivery := db.Delivery{ID: 103, OrderID: 203, Status: "pending", CreatedAt: time.Now().Add(-5 * time.Minute)}
	store.EXPECT().ListPendingDeliveriesBeforeWithoutAlert(gomock.Any(), gomock.Any()).Return([]db.Delivery{delivery}, nil)
	store.EXPECT().CreateDeliveryTimeoutAlert(gomock.Any(), db.CreateDeliveryTimeoutAlertParams{
		DeliveryID: delivery.ID,
		AlertKey:   operatorPendingDispatchAlertKey,
	}).Return(db.DeliveryTimeoutAlert{ID: 2, DeliveryID: delivery.ID, AlertKey: operatorPendingDispatchAlertKey, CreatedAt: time.Now()}, nil)
	store.EXPECT().DeleteDeliveryTimeoutAlert(gomock.Any(), db.DeleteDeliveryTimeoutAlertParams{
		DeliveryID: delivery.ID,
		AlertKey:   operatorPendingDispatchAlertKey,
	}).Return(nil)

	s.enqueueOperatorPendingDispatchAlerts()

	require.Empty(t, distributor.alerts)
}
