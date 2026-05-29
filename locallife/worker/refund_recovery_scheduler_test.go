package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
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
	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil)
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
		ID:             81,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
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
			require.Contains(t, arg.Message, "查询支付通道已进入")
			require.NotContains(t, arg.Message, "查询微信侧")
			return db.PlatformAlertEvent{ID: 701, AlertType: arg.AlertType}, nil
		})
	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient)
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
		ID:             83,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
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
	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient)
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
		ID:             84,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
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
	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient)
	scheduler.RunOnce()
}
func TestRefundRecoverySchedulerRunOnceQueriesBaofuRefundStatusByOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &orderRefundFactApplicationRecorder{}
	baofuClient := &baofuRecoveryAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:       "BFRFD_STUCK_ORDER_001",
			TradeNo:          "BFREFUND_UP_ORDER_001",
			RefundState:      aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 880,
			Raw:              json.RawMessage(`{"refundState":"SUCCESS"}`),
		},
	}

	stuckRefund := db.RefundOrder{
		ID:             263,
		PaymentOrderID: 293,
		OutRefundNo:    "BFRFD_STUCK_ORDER_001",
		Status:         "processing",
		RefundAmount:   880,
	}
	paymentOrder := db.PaymentOrder{
		ID:             293,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 2501, Valid: true},
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, stuckRefund.OutRefundNo, arg.ExternalObjectKey)
		require.Equal(t, "BFREFUND_UP_ORDER_001", arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, stuckRefund.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, "baofu:query:refund:"+stuckRefund.OutRefundNo+":"+db.ExternalPaymentTerminalStatusSuccess, arg.DedupeKey)
		return db.ExternalPaymentFact{ID: 2812, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             2812,
		Consumer:           "order_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 2912, FactID: 2812, Consumer: "order_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil)
	scheduler.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{2912}, distributor.applicationIDs)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRefundQuery.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastRefundQuery.TerminalID)
	require.Equal(t, stuckRefund.OutRefundNo, baofuClient.lastRefundQuery.OutTradeNo)
}

func TestRefundRecoverySchedulerRunOnceRecordsBaofuReservationAddonRefundQueryFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &reservationRefundFactApplicationRecorder{}
	baofuClient := &baofuRecoveryAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:       "BFRFD_STUCK_RESERVATION_ADDON_001",
			TradeNo:          "BFREFUND_UP_RESERVATION_ADDON_001",
			RefundState:      aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 660,
			Raw:              json.RawMessage(`{"refundState":"SUCCESS"}`),
		},
	}

	stuckRefund := db.RefundOrder{
		ID:             265,
		PaymentOrderID: 295,
		OutRefundNo:    "BFRFD_STUCK_RESERVATION_ADDON_001",
		Status:         "processing",
		RefundAmount:   660,
	}
	paymentOrder := db.PaymentOrder{
		ID:             295,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "reservation_addon",
		ReservationID:  pgtype.Int8{Int64: 3501, Valid: true},
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, stuckRefund.OutRefundNo, arg.ExternalObjectKey)
		require.Equal(t, "BFREFUND_UP_RESERVATION_ADDON_001", arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, stuckRefund.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
		require.Equal(t, "baofu:query:refund:"+stuckRefund.OutRefundNo+":"+db.ExternalPaymentTerminalStatusSuccess, arg.DedupeKey)
		return db.ExternalPaymentFact{ID: 2814, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             2814,
		Consumer:           "reservation_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 2914, FactID: 2814, Consumer: "reservation_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil)
	scheduler.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{2914}, distributor.applicationIDs)
	require.Equal(t, "COLLECT_MER", baofuClient.lastRefundQuery.MerchantID)
	require.Equal(t, "COLLECT_TER", baofuClient.lastRefundQuery.TerminalID)
	require.Equal(t, stuckRefund.OutRefundNo, baofuClient.lastRefundQuery.OutTradeNo)
}

func TestRefundRecoverySchedulerRunOnceUsesBaofuRefundResultCodeWhenStateAbsent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &orderRefundFactApplicationRecorder{}
	baofuClient := &baofuRecoveryAggregateClient{
		refundResult: &aggregatecontracts.RefundResult{
			OutTradeNo:       "BFRFD_RESULT_ONLY_ORDER_001",
			TradeNo:          "BFREFUND_RESULT_ONLY_ORDER_001",
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
			SuccessAmountFen: 880,
			Raw:              json.RawMessage(`{"resultCode":"SUCCESS","succAmt":880}`),
		},
	}

	stuckRefund := db.RefundOrder{
		ID:             264,
		PaymentOrderID: 294,
		OutRefundNo:    "BFRFD_RESULT_ONLY_ORDER_001",
		Status:         "processing",
		RefundAmount:   880,
	}
	paymentOrder := db.PaymentOrder{
		ID:             294,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 2502, Valid: true},
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
		require.Equal(t, stuckRefund.OutRefundNo, arg.ExternalObjectKey)
		require.Equal(t, "BFREFUND_RESULT_ONLY_ORDER_001", arg.ExternalSecondaryKey.String)
		require.Equal(t, aggregatecontracts.BusinessResultCodeSuccess, arg.UpstreamState)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, "baofu:query:refund:"+stuckRefund.OutRefundNo+":"+db.ExternalPaymentTerminalStatusSuccess, arg.DedupeKey)
		return db.ExternalPaymentFact{ID: 2813, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             2813,
		Consumer:           "order_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 2913, FactID: 2813, Consumer: "order_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil)
	scheduler.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{2913}, distributor.applicationIDs)
}
