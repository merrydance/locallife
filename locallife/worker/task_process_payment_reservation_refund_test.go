package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_ReservationAddonRefund_UsesProvidedOutRefundNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	baofuClient := &fakeWorkerBaofuRefundClient{refundResult: &aggregatecontracts.RefundResult{
		OriginTradeNo:    "BF_RA_PAY_12",
		OutTradeNo:       "RF_RA_12_1",
		TradeNo:          "BFREFUND_RA_12",
		RefundAmountFen:  300,
		TotalAmountFen:   300,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		RefundState:      aggregatecontracts.RefundStateAccepted,
		SuccessAmountFen: 300,
	}}

	reservationID := int64(88)
	paymentOrder := db.PaymentOrder{
		ID:             12,
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		OutTradeNo:     "RA_PAY_12",
		TransactionID:  pgtype.Text{String: "BF_RA_PAY_12", Valid: true},
		Amount:         600,
		Status:         "paid",
		BusinessType:   "reservation_addon",
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
	}
	refundOrder := db.RefundOrder{ID: 33, PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Status: "pending", OutRefundNo: "RF_RA_12_1"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, MerchantID: 55}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{ID: refundOrder.ID, RefundID: pgtype.Text{String: "BFREFUND_RA_12", Valid: true}}).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "processing", OutRefundNo: refundOrder.OutRefundNo}, nil)
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund, refundOrder.ID, refundOrder.OutRefundNo, "BFREFUND_RA_12", db.ExternalPaymentBusinessOwnerReservation, db.ExternalPaymentCommandStatusAccepted, "", 9602)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	payloadBytes, err := json.Marshal(worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		ReservationID:  reservationID,
		RefundAmount:   300,
		Reason:         "Reservation dish change refund",
		OutRefundNo:    refundOrder.OutRefundNo,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefund, payloadBytes)
	err = processor.ProcessTaskInitiateRefund(context.Background(), task)
	require.NoError(t, err)
	require.True(t, baofuClient.called)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRefundRequest.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastRefundRequest.TerminalID)
	require.Equal(t, "BF_RA_PAY_12", baofuClient.lastRefundRequest.OriginTradeNo)
	require.Empty(t, baofuClient.lastRefundRequest.OriginOutTradeNo)
	require.Equal(t, refundOrder.OutRefundNo, baofuClient.lastRefundRequest.OutTradeNo)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/refund", baofuClient.lastRefundRequest.NotifyURL)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.RefundAmountFen)
	require.Equal(t, int64(300), baofuClient.lastRefundRequest.TotalAmountFen)
	require.Equal(t, "Reservation dish change refund", baofuClient.lastRefundRequest.RefundReason)
}

func TestProcessTaskRefundResult_ReservationRefundSuccess_UpdatesPrepaidAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	reservationID := int64(99)
	refundOrder := db.RefundOrder{ID: 77, PaymentOrderID: 66, RefundAmount: 400, Status: "processing", OutRefundNo: "RF_RA_66_1"}
	paymentOrder := db.PaymentOrder{
		ID:            66,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		Amount:        400,
		Status:        "paid",
		BusinessType:  "reservation_addon",
		UserID:        123,
	}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_ra_66",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reservation refund results must be applied via payment fact application")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestProcessTaskRefundResult_ReservationRefundClosed_DoesNotAdjustPrepaidAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	reservationID := int64(109)
	refundOrder := db.RefundOrder{ID: 87, PaymentOrderID: 76, RefundAmount: 280, Status: "processing", OutRefundNo: "RF_RA_76_1"}
	paymentOrder := db.PaymentOrder{
		ID:            76,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		Amount:        400,
		Status:        "paid",
		BusinessType:  "reservation_addon",
		UserID:        456,
	}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "CLOSED",
		RefundID:     "refund_ra_76",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reservation refund results must be applied via payment fact application")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestProcessTaskRefundResult_OrderRefundSuccess_SendsNotificationAndMarksPaymentRefunded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	refundOrder := db.RefundOrder{ID: 88, PaymentOrderID: 78, RefundAmount: 500, Status: "processing", OutRefundNo: "RF_ORDER_78_1"}
	paymentOrder := db.PaymentOrder{
		ID:             78,
		OrderID:        pgtype.Int8{Int64: 68, Valid: true},
		Amount:         500,
		Status:         "paid",
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		UserID:         567,
	}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().GetTotalSuccessfulRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder.Amount, nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_order_78",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskRefundResult_PaymentOrderLookupFailureReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	refundOrder := db.RefundOrder{ID: 89, PaymentOrderID: 79, RefundAmount: 500, Status: "processing", OutRefundNo: "RF_ORDER_79_1"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(db.PaymentOrder{}, errors.New("db unavailable"))

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_order_79",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "get payment order for refund result routing")
	require.False(t, errors.Is(err, asynq.SkipRetry))
}

func TestProcessTaskRefundResult_ClosedDuplicateSkipsWithoutLoadingPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	refundOrder := db.RefundOrder{ID: 91, PaymentOrderID: 81, RefundAmount: 180, Status: "closed", OutRefundNo: "RF_DUP_CLOSED_1"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "CLOSED",
		RefundID:     "refund_dup_closed",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskRefundResult_SuccessDuplicateSkipsWithoutLoadingPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	refundOrder := db.RefundOrder{ID: 92, PaymentOrderID: 82, RefundAmount: 180, Status: "success", OutRefundNo: "RF_DUP_SUCCESS_1"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_dup_success",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskRefundResult_AbnormalDuplicateSkipsWithoutLoadingPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	refundOrder := db.RefundOrder{ID: 93, PaymentOrderID: 83, RefundAmount: 180, Status: "failed", OutRefundNo: "RF_DUP_ABNORMAL_1"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "ABNORMAL",
		RefundID:     "refund_dup_abnormal",
	})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes))
	require.NoError(t, err)
}
