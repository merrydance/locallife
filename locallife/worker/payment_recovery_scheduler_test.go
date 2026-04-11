package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentRecoverySchedulerRunOnceSkipsRefundingOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

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

func TestPaymentRecoverySchedulerRunOnceEnqueuesEligibleOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	order := db.PaymentOrder{
		ID:           202,
		OutTradeNo:   "PAY_RECOVERY_OK",
		BusinessType: "order",
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
	distributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(
			gomock.Any(),
			gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}),
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil)

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestPaymentRecoverySchedulerRunOncePassesTransactionIDAndBusinessType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	order := db.PaymentOrder{
		ID:           203,
		OutTradeNo:   "PAY_RECOVERY_PARTNER",
		BusinessType: "reservation",
		TransactionID: pgtype.Text{
			String: "wx_tx_203",
			Valid:  true,
		},
	}

	store.EXPECT().
		ListPaidUnprocessedPaymentOrders(gomock.Any(), gomock.Any()).
		Return([]db.PaymentOrder{order}, nil)
	store.EXPECT().
		ListRefundOrdersByPaymentOrder(gomock.Any(), order.ID).
		Return([]db.RefundOrder{}, nil)
	distributor.EXPECT().
		DistributeTaskProcessPaymentSuccess(gomock.Any(), gomock.AssignableToTypeOf(&worker.PaymentSuccessPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.PaymentSuccessPayload, _ ...asynq.Option) error {
			require.Equal(t, order.ID, payload.PaymentOrderID)
			require.Equal(t, "wx_tx_203", payload.TransactionID)
			require.Equal(t, "reservation", payload.BusinessType)
			return nil
		})

	scheduler := worker.NewPaymentRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}
