package worker_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProfitSharingRecoverySchedulerRunOnceEnqueuesCompletedOrdersMissingProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingProcessEnqueueRecorder{}

	store.EXPECT().
		ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListCompletedOrdersMissingProfitSharingRow{{
			PaymentOrderID: 301,
			OrderID:        pgtype.Int8{Int64: 401, Valid: true},
		}}, nil)
	store.EXPECT().
		ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingReturn{}, nil)

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
	require.Equal(t, 1, distributor.calls)
}

func TestProfitSharingRecoverySchedulerRunOnceEnqueuesReservationProfitSharingRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingProcessEnqueueRecorder{
		validate: func(payload *worker.ProfitSharingPayload) {
			require.Equal(t, int64(301), payload.PaymentOrderID)
			require.Equal(t, int64(901), payload.ReservationID)
			require.Zero(t, payload.OrderID)
		},
	}

	store.EXPECT().
		ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{{ID: 21, PaymentOrderID: 301}}, nil)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(301)).
		Return(db.PaymentOrder{ID: 301, ReservationID: pgtype.Int8{Int64: 901, Valid: true}}, nil)
	store.EXPECT().
		ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListCompletedOrdersMissingProfitSharingRow{}, nil)
	store.EXPECT().
		ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingReturn{}, nil)

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor, nil)
	scheduler.RunOnce()
	require.Equal(t, 1, distributor.calls)
}

type profitSharingProcessEnqueueRecorder struct {
	worker.NoopTaskDistributor
	calls    int
	validate func(*worker.ProfitSharingPayload)
}

func (d *profitSharingProcessEnqueueRecorder) DistributeTaskProcessProfitSharing(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
	d.calls++
	if d.validate != nil {
		d.validate(payload)
	}
	return nil
}

func TestProfitSharingRecoverySchedulerRunOnceRecordsStuckReturnFactInsteadOfLegacyResultTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:                   188,
		RefundOrderID:        288,
		ProfitSharingOrderID: 388,
		PaymentOrderID:       488,
		SubMchid:             "sub-mchid-188",
		OutOrderNo:           "PS188",
		OutReturnNo:          "PR188",
		ReturnMchid:          "190000188",
		Amount:               420,
		Status:               "processing",
	}

	store.EXPECT().
		ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().
		ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).
		Return([]db.ListCompletedOrdersMissingProfitSharingRow{}, nil)
	store.EXPECT().
		ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).
		Return([]db.ProfitSharingReturn{returnRecord}, nil)
	ecommerceClient.EXPECT().
		QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).
		Return(&wechatcontracts.ProfitSharingReturnResponse{
			SubMchID:    returnRecord.SubMchid,
			OutOrderNo:  returnRecord.OutOrderNo,
			OutReturnNo: returnRecord.OutReturnNo,
			ReturnID:    "wx-return-188",
			ReturnMchID: returnRecord.ReturnMchid,
			Amount:      returnRecord.Amount,
			Result:      wechatcontracts.ProfitSharingReturnResultSuccess,
		}, nil)
	expectProfitSharingReturnQueryFact(t, store, returnRecord, "wx-return-188", wechatcontracts.ProfitSharingReturnResultSuccess, db.ExternalPaymentTerminalStatusSuccess, "")

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()

	require.Equal(t, []int64{10188}, distributor.applicationIDs)
}

func TestProfitSharingRecoverySchedulerRunOnceKeepsProcessingReturnWithoutLegacyResultTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingReturnRecoveryNoLegacyDistributor{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	returnRecord := db.ProfitSharingReturn{
		ID:            189,
		RefundOrderID: 289,
		SubMchid:      "sub-mchid-189",
		OutOrderNo:    "PS189",
		OutReturnNo:   "PR189",
		Amount:        360,
		Status:        "processing",
	}

	store.EXPECT().ListProfitSharingOrdersForRetry(gomock.Any(), gomock.Any()).Return([]db.ProfitSharingOrder{}, nil)
	store.EXPECT().ListCompletedOrdersMissingProfitSharing(gomock.Any(), gomock.Any()).Return([]db.ListCompletedOrdersMissingProfitSharingRow{}, nil)
	store.EXPECT().ListStuckProcessingProfitSharingReturns(gomock.Any(), gomock.Any()).Return([]db.ProfitSharingReturn{returnRecord}, nil)
	ecommerceClient.EXPECT().
		QueryProfitSharingReturn(gomock.Any(), returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo).
		Return(&wechatcontracts.ProfitSharingReturnResponse{
			SubMchID:    returnRecord.SubMchid,
			OutOrderNo:  returnRecord.OutOrderNo,
			OutReturnNo: returnRecord.OutReturnNo,
			ReturnID:    "wx-return-189",
			Amount:      returnRecord.Amount,
			Result:      wechatcontracts.ProfitSharingReturnResultProcessing,
		}, nil)
	store.EXPECT().
		UpdateProfitSharingReturnToProcessing(gomock.Any(), db.UpdateProfitSharingReturnToProcessingParams{
			ID:       returnRecord.ID,
			ReturnID: pgtype.Text{String: "wx-return-189", Valid: true},
		}).
		Return(returnRecord, nil)

	scheduler := worker.NewProfitSharingRecoveryScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()

	require.False(t, distributor.legacyResultTaskCalled)
}

type profitSharingReturnRecoveryNoLegacyDistributor struct {
	worker.NoopTaskDistributor
	legacyResultTaskCalled bool
}

func (d *profitSharingReturnRecoveryNoLegacyDistributor) DistributeTaskProcessProfitSharingReturnResult(context.Context, *worker.ProfitSharingReturnResultPayload, ...asynq.Option) error {
	d.legacyResultTaskCalled = true
	return nil
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
