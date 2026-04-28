package worker_test

import (
	"context"
	"errors"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type riderDepositRefundFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
}

func (r *riderDepositRefundFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

type reservationRefundFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
}

func (r *reservationRefundFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

type orderRefundFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
}

func (r *orderRefundFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

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

func TestRefundRecoverySchedulerRunOncePersistsAlertForUnsupportedDirectRefundFactTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

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
	store.EXPECT().
		CreatePlatformAlertEvent(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePlatformAlertEventParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
			require.Equal(t, string(worker.AlertTypeRefundFailed), arg.AlertType)
			require.Equal(t, string(worker.AlertLevelCritical), arg.Level)
			require.Equal(t, stuckRefund.ID, arg.RelatedID)
			require.Equal(t, "refund_order", arg.RelatedType)
			return db.PlatformAlertEvent{ID: 701, AlertType: arg.AlertType}, nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceRecordsRiderDepositDirectRefundQueryFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &riderDepositRefundFactApplicationRecorder{}
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             53,
		PaymentOrderID: 83,
		OutRefundNo:    "RFD_STUCK_RIDER_001",
		Status:         "processing",
		RefundAmount:   20000,
	}
	paymentOrder := db.PaymentOrder{
		ID:           83,
		PaymentType:  "miniprogram",
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
	}

	store.EXPECT().ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).Return([]db.ListStuckProcessingRefundOrdersRow{{
		ID:          stuckRefund.ID,
		OutRefundNo: stuckRefund.OutRefundNo,
		Status:      stuckRefund.Status,
		CreatedAt:   time.Now().Add(-20 * time.Minute),
		PaymentType: paymentOrder.PaymentType,
	}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	paymentClient.EXPECT().QueryRefund(gomock.Any(), stuckRefund.OutRefundNo).Return(&wechat.RefundResponse{RefundID: "wx_refund_rider_001", Status: wechat.RefundStatusSuccess}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			if arg.FactSource != db.ExternalPaymentFactSourceQuery || arg.ExternalObjectKey != stuckRefund.OutRefundNo || arg.TerminalStatus != db.ExternalPaymentTerminalStatusSuccess {
				t.Fatalf("unexpected fact params: %+v", arg)
			}
			if arg.DedupeKey != "wechat:query:direct:refund:"+stuckRefund.OutRefundNo+":"+db.ExternalPaymentTerminalStatusSuccess {
				t.Fatalf("unexpected dedupe key: %s", arg.DedupeKey)
			}
			return db.ExternalPaymentFact{ID: 201, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             201,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 301,
		FactID:             201,
		Consumer:           "rider_deposit_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient, nil)
	scheduler.RunOnce()
	if len(distributor.applicationIDs) != 1 || distributor.applicationIDs[0] != 301 {
		t.Fatalf("unexpected application ids: %+v", distributor.applicationIDs)
	}
}

func TestRefundRecoverySchedulerRunOnceSkipsRiderDepositRefundResultWhenFactWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             54,
		PaymentOrderID: 84,
		OutRefundNo:    "RFD_STUCK_RIDER_FACT_FAIL_001",
		Status:         "processing",
		RefundAmount:   20000,
	}
	paymentOrder := db.PaymentOrder{
		ID:           84,
		PaymentType:  "miniprogram",
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
	}

	store.EXPECT().ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).Return([]db.ListStuckProcessingRefundOrdersRow{{
		ID:          stuckRefund.ID,
		OutRefundNo: stuckRefund.OutRefundNo,
		Status:      stuckRefund.Status,
		CreatedAt:   time.Now().Add(-20 * time.Minute),
		PaymentType: paymentOrder.PaymentType,
	}}, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), stuckRefund.ID).Return(stuckRefund, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	paymentClient.EXPECT().QueryRefund(gomock.Any(), stuckRefund.OutRefundNo).Return(&wechat.RefundResponse{RefundID: "wx_refund_rider_fail_001", Status: wechat.RefundStatusSuccess}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{}, errors.New("insert fact failed"))

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceSkipsDirectRefundStatusWithoutPaymentClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             52,
		PaymentOrderID: 82,
		OutRefundNo:    "RFD_STUCK_DIRECT_SKIP_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:          82,
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

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOncePersistsAlertForUnsupportedEcommerceRefundFactTarget(t *testing.T) {
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
		ID:             91,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 501, Valid: true},
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
	store.EXPECT().
		CreatePlatformAlertEvent(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePlatformAlertEventParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
			require.Equal(t, string(worker.AlertTypeRefundFailed), arg.AlertType)
			require.Equal(t, string(worker.AlertLevelCritical), arg.Level)
			require.Equal(t, stuckRefund.ID, arg.RelatedID)
			require.Equal(t, "refund_order", arg.RelatedType)
			return db.PlatformAlertEvent{ID: 702, AlertType: arg.AlertType}, nil
		})

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceSkipsEcommerceRefundStatusWithoutEcommerceClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             64,
		PaymentOrderID: 94,
		OutRefundNo:    "RFD_STUCK_ECOM_SKIP_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             94,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 503, Valid: true},
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

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
	scheduler.RunOnce()
}

func TestRefundRecoverySchedulerRunOnceQueriesEcommerceRefundStatusByReservation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &reservationRefundFactApplicationRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             62,
		PaymentOrderID: 92,
		OutRefundNo:    "RFD_STUCK_ECOM_RES_001",
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             92,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   db.ExternalPaymentBusinessOwnerReservation,
		ReservationID:  pgtype.Int8{Int64: 601, Valid: true},
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		if arg.ExternalObjectKey != stuckRefund.OutRefundNo || arg.TerminalStatus != db.ExternalPaymentTerminalStatusClosed || arg.BusinessObjectID.Int64 != stuckRefund.ID {
			t.Fatalf("unexpected fact params: %+v", arg)
		}
		return db.ExternalPaymentFact{ID: 811, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             811,
		Consumer:           "reservation_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 911, FactID: 811, Consumer: "reservation_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
	require.Equal(t, []int64{911}, distributor.applicationIDs)
}

func TestRefundRecoverySchedulerRunOnceQueriesEcommerceRefundStatusByOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &orderRefundFactApplicationRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	stuckRefund := db.RefundOrder{
		ID:             63,
		PaymentOrderID: 93,
		OutRefundNo:    "RFD_STUCK_ECOM_ORDER_001",
		Status:         "processing",
		RefundAmount:   880,
	}
	paymentOrder := db.PaymentOrder{
		ID:             93,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 501, Valid: true},
	}

	store.EXPECT().ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).Return([]db.ListStuckProcessingRefundOrdersRow{{
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
	ecommerceClient.EXPECT().QueryEcommerceRefund(gomock.Any(), "sub_mch_7001", stuckRefund.OutRefundNo).Return(&wechat.EcommerceRefundResponse{RefundID: "wx_refund_ecom_order_001", Status: wechat.RefundStatusAbnormal}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		if arg.ExternalObjectKey != stuckRefund.OutRefundNo || arg.TerminalStatus != db.ExternalPaymentTerminalStatusFailed || arg.BusinessObjectID.Int64 != stuckRefund.ID || arg.BusinessOwner.String != db.ExternalPaymentBusinessOwnerOrder {
			t.Fatalf("unexpected fact params: %+v", arg)
		}
		return db.ExternalPaymentFact{ID: 812, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             812,
		Consumer:           "order_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 912, FactID: 812, Consumer: "order_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, ecommerceClient)
	scheduler.RunOnce()
	require.Equal(t, []int64{912}, distributor.applicationIDs)
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
		ID:             93,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		OrderID:        pgtype.Int8{Int64: 502, Valid: true},
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
