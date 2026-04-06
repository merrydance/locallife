package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskInitiateRefund_ReservationAddonRefund_UsesProvidedOutRefundNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	reservationID := int64(88)
	paymentOrder := db.PaymentOrder{
		ID:            12,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		OutTradeNo:    "RA_PAY_12",
		Amount:        600,
		Status:        "paid",
		BusinessType:  "reservation_addon",
		PaymentType:   "profit_sharing",
	}
	refundOrder := db.RefundOrder{ID: 33, PaymentOrderID: paymentOrder.ID, RefundAmount: 300, Status: "pending", OutRefundNo: "RF_RA_12_1"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, MerchantID: 55}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(55)).Return(db.MerchantPaymentConfig{MerchantID: 55, SubMchID: "sub_mch_55"}, nil)
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
		require.Equal(t, "sub_mch_55", req.SubMchID)
		require.Equal(t, paymentOrder.OutTradeNo, req.OutTradeNo)
		require.Equal(t, refundOrder.OutRefundNo, req.OutRefundNo)
		require.Equal(t, int64(300), req.RefundAmount)
		require.Equal(t, paymentOrder.Amount, req.TotalAmount)
		return &wechat.EcommerceRefundResponse{RefundID: "refund_ra_12", Status: wechat.RefundStatusSuccess}, nil
	})
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(300), nil)
	store.EXPECT().AddReservationPrepaidAmount(gomock.Any(), db.AddReservationPrepaidAmountParams{ID: reservationID, PrepaidAmount: -300}).Return(db.TableReservation{ID: reservationID}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
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
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, MerchantID: 55}, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(400), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	store.EXPECT().AddReservationPrepaidAmount(gomock.Any(), db.AddReservationPrepaidAmountParams{ID: reservationID, PrepaidAmount: -400}).Return(db.TableReservation{ID: reservationID}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_ra_66",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.NoError(t, err)
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
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, MerchantID: 55}, nil)
	store.EXPECT().UpdateRefundOrderToClosed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "closed", OutRefundNo: refundOrder.OutRefundNo}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "CLOSED",
		RefundID:     "refund_ra_76",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskRefundResult_OrderRefundSuccess_SendsNotificationAndMarksPaymentRefunded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	refundOrder := db.RefundOrder{ID: 88, PaymentOrderID: 78, RefundAmount: 500, Status: "processing", OutRefundNo: "RF_ORDER_78_1"}
	paymentOrder := db.PaymentOrder{
		ID:           78,
		OrderID:      pgtype.Int8{Int64: 68, Valid: true},
		Amount:       500,
		Status:       "paid",
		BusinessType: "takeout_order",
		UserID:       567,
	}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(68)).Return(db.Order{ID: 68, MerchantID: 66}, nil)
	store.EXPECT().UpdateRefundOrderToSuccess(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, PaymentOrderID: refundOrder.PaymentOrderID, RefundAmount: refundOrder.RefundAmount, Status: "success", OutRefundNo: refundOrder.OutRefundNo}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(500), nil)
	store.EXPECT().UpdatePaymentOrderToRefunded(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "refunded"}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(567), payload.UserID)
			require.Equal(t, "refund", payload.Type)
			require.Equal(t, "退款成功", payload.Title)
			require.Contains(t, payload.Content, "5.00")
			require.Equal(t, int64(88), payload.RelatedID)
			require.Equal(t, "refund", payload.RelatedType)
			require.Equal(t, refundOrder.OutRefundNo, payload.ExtraData["out_refund_no"])
			require.Equal(t, "refund_order_78", payload.ExtraData["refund_id"])
			require.Equal(t, int64(500), payload.ExtraData["amount"])
			return nil
		},
	)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
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
