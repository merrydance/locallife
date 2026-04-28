package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentRecoverySchedulerRunOnceSkipsRefundingOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentRecoverySchedulerTestDistributor{}

	order := db.PaymentOrder{
		ID:           101,
		OutTradeNo:   "PAY_RECOVERY_SKIP",
		BusinessType: "order",
		TransactionID: pgtype.Text{
			String: "wx_tx_101",
			Valid:  true,
		},
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{{ID: 9001, PaymentOrderID: order.ID, Status: "pending"}}, nil)

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestPaymentRecoverySchedulerRunOnceCreatesOrderPaymentFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentRecoverySchedulerTestDistributor{}

	order := db.PaymentOrder{
		ID:             202,
		OutTradeNo:     "PAY_RECOVERY_OK",
		BusinessType:   "order",
		PaymentChannel: db.PaymentChannelEcommerce,
		TransactionID: pgtype.Text{
			String: "wx_tx_202",
			Valid:  true,
		},
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceManualReconciliation, arg.FactSource)
		require.Equal(t, order.OutTradeNo, arg.ExternalObjectKey)
		return db.ExternalPaymentFact{ID: 3001, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(3001), arg.FactID)
		require.Equal(t, int64(202), arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 4001, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(4001), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestPaymentRecoverySchedulerRunOnceCreatesRiderDepositPaymentFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentRecoverySchedulerTestDistributor{}

	order := db.PaymentOrder{
		ID:             204,
		OutTradeNo:     "PAY_RECOVERY_RIDER_DEPOSIT",
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
		PaymentChannel: db.PaymentChannelDirect,
		TransactionID: pgtype.Text{
			String: "wx_tx_204",
			Valid:  true,
		},
		Amount: 8800,
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelDirect, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceManualReconciliation, arg.FactSource)
		require.Equal(t, db.ExternalPaymentBusinessOwnerRiderDeposit, arg.BusinessOwner.String)
		require.Equal(t, order.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		return db.ExternalPaymentFact{ID: 3002, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(3002), arg.FactID)
		require.Equal(t, int64(204), arg.BusinessObjectID)
		require.Equal(t, "rider_deposit_domain", arg.Consumer)
		return db.ExternalPaymentFactApplication{ID: 4002, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(4002), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestPaymentRecoverySchedulerRunOnceCreatesClaimRecoveryPaymentFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentRecoverySchedulerTestDistributor{}

	order := db.PaymentOrder{
		ID:             203,
		OutTradeNo:     "PAY_RECOVERY_CLAIM",
		BusinessType:   db.ExternalPaymentBusinessOwnerClaimRecovery,
		PaymentChannel: db.PaymentChannelDirect,
		TransactionID: pgtype.Text{
			String: "wx_tx_203",
			Valid:  true,
		},
		Amount: 9100,
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelDirect, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceManualReconciliation, arg.FactSource)
		require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, arg.BusinessOwner.String)
		require.Equal(t, order.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		return db.ExternalPaymentFact{ID: 3003, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(3003), arg.FactID)
		require.Equal(t, int64(203), arg.BusinessObjectID)
		require.Equal(t, "claim_recovery_domain", arg.Consumer)
		return db.ExternalPaymentFactApplication{ID: 4003, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(4003), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestPaymentRecoverySchedulerRunOnceRecordsReservationPaymentFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentRecoverySchedulerTestDistributor{}

	order := db.PaymentOrder{
		ID:             305,
		OutTradeNo:     "PAY_RECOVERY_RESERVATION",
		BusinessType:   db.ExternalPaymentBusinessOwnerReservation,
		PaymentChannel: db.PaymentChannelEcommerce,
		ReservationID:  pgtype.Int8{Int64: 405, Valid: true},
		TransactionID: pgtype.Text{
			String: "wx_tx_305",
			Valid:  true,
		},
		Amount: 5600,
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
		require.Equal(t, order.ID, arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 811, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(811), arg.FactID)
		require.Equal(t, order.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 911, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(911), payload.ApplicationID)
		return nil
	}

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

type paymentRecoverySchedulerTestDistributor struct {
	worker.NoopTaskDistributor
	processPaymentFactApplication func(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error
}

func (d *paymentRecoverySchedulerTestDistributor) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	if d.processPaymentFactApplication == nil {
		return nil
	}
	return d.processPaymentFactApplication(ctx, payload, opts...)
}
