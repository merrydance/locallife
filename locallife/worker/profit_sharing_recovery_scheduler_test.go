package worker_test

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"go.uber.org/mock/gomock"
)

func TestProfitSharingRecoverySchedulerRunOnceEnqueuesCompletedOrdersMissingProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListCompletedOrdersMissingProfitSharingRow{{
			PaymentOrderID: 301,
			OrderID:        pgtype.Int8{Int64: 401, Valid: true},
		}}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(
			gomock.Any(),
			gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}),
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil)
	store.EXPECT().
		ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingReturn{}, nil)

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestProfitSharingRecoverySchedulerRunOnceEnqueuesReservationProfitSharingRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{{ID: 21, PaymentOrderID: 301}}, nil)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(301)).
		Return(db.PaymentOrder{ID: 301, ReservationID: pgtype.Int8{Int64: 901, Valid: true}}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(
			gomock.Any(),
			gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}),
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(func(_ interface{}, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			if payload.PaymentOrderID != 301 || payload.ReservationID != 901 || payload.OrderID != 0 {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			return nil
		})
	store.EXPECT().
		ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListCompletedOrdersMissingProfitSharingRow{}, nil)
	store.EXPECT().
		ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingReturn{}, nil)

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}

func TestWithProfitSharingEnqueueDedupAppendsUniqueOption(t *testing.T) {
	merged := worker.WithProfitSharingEnqueueDedupForTest()
	if len(merged) != 1 {
		t.Fatalf("expected 1 option, got %d", len(merged))
	}

	merged = worker.WithProfitSharingEnqueueDedupForTest(asynq.Queue(worker.QueueCritical))
	if len(merged) != 2 {
		t.Fatalf("expected 2 options, got %d", len(merged))
	}
}

func TestProfitSharingPayloadNormalizationUsesStableIdempotencyKey(t *testing.T) {
	normalized := worker.NormalizeProfitSharingPayloadForTest(&worker.ProfitSharingPayload{
		PaymentOrderID: 301,
		OrderID:        401,
	})

	if normalized.IdempotencyKey != "profit_sharing:payment_order:301" {
		t.Fatalf("unexpected idempotency key: %s", normalized.IdempotencyKey)
	}

	if got := worker.ProfitSharingTaskIdempotencyKeyForTest(worker.ProfitSharingPayload{PaymentOrderID: 301, OrderID: 999}); got != normalized.IdempotencyKey {
		t.Fatalf("expected stable idempotency key, got %s", got)
	}
}
