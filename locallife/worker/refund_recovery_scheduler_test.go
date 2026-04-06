package worker_test

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"go.uber.org/mock/gomock"
)

func TestRefundRecoverySchedulerRunOnceProcessesPendingReservationRefundsWithoutOrderRefunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{{
			ID:             12,
			PaymentOrderID: 34,
			RefundAmount:   560,
			OutRefundNo:    "RFD_RECOVERY_001",
			ReservationID:  pgtype.Int8{Int64: 78, Valid: true},
			BusinessType:   "reservation",
		}}, nil)
	store.EXPECT().
		ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).
		Return([]db.ListStuckProcessingRefundOrdersRow{}, nil)
	distributor.EXPECT().
		DistributeTaskProcessRefund(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadProcessRefund{})).
		DoAndReturn(func(_ any, payload *worker.PayloadProcessRefund, _ ...asynq.Option) error {
			if payload.PaymentOrderID != 34 || payload.ReservationID != 78 || payload.OutRefundNo != "RFD_RECOVERY_001" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			return nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceQueriesDirectRefundStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             51,
		PaymentOrderID: 81,
		OutRefundNo:    "RFD_STUCK_DIRECT_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:          81,
		PaymentType: "miniprogram",
	}

	store.EXPECT().
		ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().
		ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).
		Return([]db.ListStuckProcessingRefundOrdersRow{{
			ID:          stuckRefund.ID,
			OutRefundNo: stuckRefund.OutRefundNo,
			Status:      stuckRefund.Status,
			CreatedAt:   time.Now().Add(-20 * time.Minute),
			PaymentType: paymentOrder.PaymentType,
		}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	paymentClient.EXPECT().
		QueryRefund(gomock.Any(), stuckRefund.OutRefundNo).
		Return(&wechat.RefundResponse{RefundID: "wx_refund_direct_001", Status: wechat.RefundStatusSuccess}, nil)
	distributor.EXPECT().
		DistributeTaskProcessRefundResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.RefundResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, payload *worker.RefundResultPayload, _ ...asynq.Option) error {
			if payload.OutRefundNo != stuckRefund.OutRefundNo || payload.RefundStatus != wechat.RefundStatusSuccess || payload.RefundID != "wx_refund_direct_001" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			return nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceQueriesEcommerceRefundStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             61,
		PaymentOrderID: 91,
		OutRefundNo:    "RFD_STUCK_ECOM_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:          91,
		PaymentType: "profit_sharing",
		OrderID:     pgtype.Int8{Int64: 501, Valid: true},
	}

	store.EXPECT().
		ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().
		ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).
		Return([]db.ListStuckProcessingRefundOrdersRow{{
			ID:          stuckRefund.ID,
			OutRefundNo: stuckRefund.OutRefundNo,
			Status:      stuckRefund.Status,
			CreatedAt:   time.Now().Add(-20 * time.Minute),
			PaymentType: paymentOrder.PaymentType,
		}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(501)).Return(db.Order{ID: 501, MerchantID: 7001}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(7001)).Return(db.MerchantPaymentConfig{MerchantID: 7001, SubMchID: "sub_mch_7001"}, nil)
	ecommerceClient.EXPECT().
		QueryEcommerceRefund(gomock.Any(), "sub_mch_7001", stuckRefund.OutRefundNo).
		Return(&wechat.EcommerceRefundResponse{RefundID: "wx_refund_ecom_001", Status: wechat.RefundStatusAbnormal}, nil)
	distributor.EXPECT().
		DistributeTaskProcessRefundResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.RefundResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, payload *worker.RefundResultPayload, _ ...asynq.Option) error {
			if payload.OutRefundNo != stuckRefund.OutRefundNo || payload.RefundStatus != wechat.RefundStatusAbnormal || payload.RefundID != "wx_refund_ecom_001" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			return nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceQueriesEcommerceRefundStatusByReservation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             62,
		PaymentOrderID: 92,
		OutRefundNo:    "RFD_STUCK_ECOM_RES_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:            92,
		PaymentType:   "profit_sharing",
		ReservationID: pgtype.Int8{Int64: 601, Valid: true},
	}

	store.EXPECT().
		ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().
		ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).
		Return([]db.ListStuckProcessingRefundOrdersRow{{
			ID:          stuckRefund.ID,
			OutRefundNo: stuckRefund.OutRefundNo,
			Status:      stuckRefund.Status,
			CreatedAt:   time.Now().Add(-20 * time.Minute),
			PaymentType: paymentOrder.PaymentType,
		}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), int64(601)).Return(db.TableReservation{ID: 601, MerchantID: 8001}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(8001)).Return(db.MerchantPaymentConfig{MerchantID: 8001, SubMchID: "sub_mch_8001"}, nil)
	ecommerceClient.EXPECT().
		QueryEcommerceRefund(gomock.Any(), "sub_mch_8001", stuckRefund.OutRefundNo).
		Return(&wechat.EcommerceRefundResponse{RefundID: "wx_refund_ecom_res_001", Status: wechat.RefundStatusClosed}, nil)
	distributor.EXPECT().
		DistributeTaskProcessRefundResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.RefundResultPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, payload *worker.RefundResultPayload, _ ...asynq.Option) error {
			if payload.OutRefundNo != stuckRefund.OutRefundNo || payload.RefundStatus != wechat.RefundStatusClosed || payload.RefundID != "wx_refund_ecom_res_001" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			return nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceKeepsWaitingWhenEcommerceRefundStillProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             63,
		PaymentOrderID: 93,
		OutRefundNo:    "RFD_STUCK_ECOM_WAIT_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:          93,
		PaymentType: "profit_sharing",
		OrderID:     pgtype.Int8{Int64: 502, Valid: true},
	}

	store.EXPECT().
		ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).
		Return([]db.PaymentOrder{}, nil)
	store.EXPECT().
		ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).
		Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().
		ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).
		Return([]db.ListStuckProcessingRefundOrdersRow{{
			ID:          stuckRefund.ID,
			OutRefundNo: stuckRefund.OutRefundNo,
			Status:      stuckRefund.Status,
			CreatedAt:   time.Now().Add(-20 * time.Minute),
			PaymentType: paymentOrder.PaymentType,
		}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(502)).Return(db.Order{ID: 502, MerchantID: 7002}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(7002)).Return(db.MerchantPaymentConfig{MerchantID: 7002, SubMchID: "sub_mch_7002"}, nil)
	ecommerceClient.EXPECT().
		QueryEcommerceRefund(gomock.Any(), "sub_mch_7002", stuckRefund.OutRefundNo).
		Return(&wechat.EcommerceRefundResponse{RefundID: "wx_refund_ecom_wait_001", Status: wechat.RefundStatusProcessing}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
}
