package worker_test

import (
	"bytes"
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
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

type refundRecoveryOrdinaryClient struct {
	createRefundRequest               *ospcontracts.RefundCreateRequest
	createRefundResponse              *ospcontracts.RefundResponse
	createRefundErr                   error
	queryRefundRequest                *ospcontracts.RefundQueryRequest
	queryRefundResponse               *ospcontracts.RefundResponse
	addProfitSharingReceiverRequests  []ospcontracts.ProfitSharingReceiverAddRequest
	addProfitSharingReceiverErrs      []error
	createProfitSharingOrderRequest   *ospcontracts.ProfitSharingOrderRequest
	createProfitSharingOrderResponse  *ospcontracts.ProfitSharingOrderResponse
	createProfitSharingOrderErrs      []error
	createProfitSharingReturnRequest  *ospcontracts.ProfitSharingReturnRequest
	createProfitSharingReturnResponse *ospcontracts.ProfitSharingReturnResponse
	createProfitSharingReturnErr      error
	queryProfitSharingReturnRequest   *ospcontracts.ProfitSharingReturnQueryRequest
	queryProfitSharingReturnResponse  *ospcontracts.ProfitSharingReturnResponse
	queryProfitSharingReturnErr       error
}

func (c *refundRecoveryOrdinaryClient) ServiceProviderAppID() string   { return "wxsp_app" }
func (c *refundRecoveryOrdinaryClient) ServiceProviderMchID() string   { return "1900000109" }
func (c *refundRecoveryOrdinaryClient) ServiceProviderMchName() string { return "LocalLife" }
func (c *refundRecoveryOrdinaryClient) RefundNotifyURL() string {
	return "https://api.example.com/v1/webhooks/wechat-ordinary/refund-notify"
}
func (c *refundRecoveryOrdinaryClient) QueryPayment(context.Context, ospcontracts.PaymentQueryRequest) (*ospcontracts.PaymentQueryResponse, error) {
	return nil, nil
}
func (c *refundRecoveryOrdinaryClient) ClosePayment(context.Context, ospcontracts.PaymentCloseRequest) error {
	return nil
}
func (c *refundRecoveryOrdinaryClient) QueryCombinePayment(context.Context, ospcontracts.CombineQueryRequest) (*ospcontracts.CombineQueryResponse, error) {
	return nil, nil
}
func (c *refundRecoveryOrdinaryClient) CloseCombinePayment(context.Context, ospcontracts.CombineCloseRequest) error {
	return nil
}
func (c *refundRecoveryOrdinaryClient) CreateRefund(_ context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error) {
	c.createRefundRequest = &req
	if c.createRefundErr != nil {
		return nil, c.createRefundErr
	}
	if c.createRefundResponse != nil {
		return c.createRefundResponse, nil
	}
	return &ospcontracts.RefundResponse{RefundID: "refund-ordinary", OutRefundNo: req.OutRefundNo}, nil
}
func (c *refundRecoveryOrdinaryClient) QueryRefund(_ context.Context, req ospcontracts.RefundQueryRequest) (*ospcontracts.RefundResponse, error) {
	c.queryRefundRequest = &req
	return c.queryRefundResponse, nil
}
func (c *refundRecoveryOrdinaryClient) AddProfitSharingReceiver(_ context.Context, req ospcontracts.ProfitSharingReceiverAddRequest) (*ospcontracts.ProfitSharingReceiverResponse, error) {
	c.addProfitSharingReceiverRequests = append(c.addProfitSharingReceiverRequests, req)
	if len(c.addProfitSharingReceiverErrs) > 0 {
		err := c.addProfitSharingReceiverErrs[0]
		c.addProfitSharingReceiverErrs = c.addProfitSharingReceiverErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	return &ospcontracts.ProfitSharingReceiverResponse{SubMchID: req.SubMchID, Type: req.Type, Account: req.Account, Name: req.Name, RelationType: req.RelationType}, nil
}
func (c *refundRecoveryOrdinaryClient) DeleteProfitSharingReceiver(context.Context, ospcontracts.ProfitSharingReceiverDeleteRequest) (*ospcontracts.ProfitSharingReceiverResponse, error) {
	return nil, nil
}
func (c *refundRecoveryOrdinaryClient) CreateProfitSharingOrder(_ context.Context, req ospcontracts.ProfitSharingOrderRequest) (*ospcontracts.ProfitSharingOrderResponse, error) {
	c.createProfitSharingOrderRequest = &req
	if len(c.createProfitSharingOrderErrs) > 0 {
		err := c.createProfitSharingOrderErrs[0]
		c.createProfitSharingOrderErrs = c.createProfitSharingOrderErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if c.createProfitSharingOrderResponse != nil {
		return c.createProfitSharingOrderResponse, nil
	}
	return &ospcontracts.ProfitSharingOrderResponse{SubMchID: req.SubMchID, TransactionID: req.TransactionID, OutOrderNo: req.OutOrderNo, OrderID: "ps-ordinary", State: ospcontracts.ProfitSharingOrderStateProcessing}, nil
}
func (c *refundRecoveryOrdinaryClient) QueryProfitSharingOrder(context.Context, ospcontracts.ProfitSharingQueryRequest) (*ospcontracts.ProfitSharingOrderResponse, error) {
	return nil, nil
}
func (c *refundRecoveryOrdinaryClient) CreateProfitSharingReturn(_ context.Context, req ospcontracts.ProfitSharingReturnRequest) (*ospcontracts.ProfitSharingReturnResponse, error) {
	c.createProfitSharingReturnRequest = &req
	if c.createProfitSharingReturnErr != nil {
		return nil, c.createProfitSharingReturnErr
	}
	if c.createProfitSharingReturnResponse != nil {
		return c.createProfitSharingReturnResponse, nil
	}
	return &ospcontracts.ProfitSharingReturnResponse{SubMchID: req.SubMchID, OrderID: req.OrderID, OutOrderNo: req.OutOrderNo, OutReturnNo: req.OutReturnNo, ReturnMchID: req.ReturnMchID, Amount: req.Amount, State: ospcontracts.ProfitSharingReturnStateProcessing}, nil
}
func (c *refundRecoveryOrdinaryClient) QueryProfitSharingReturn(_ context.Context, req ospcontracts.ProfitSharingReturnQueryRequest) (*ospcontracts.ProfitSharingReturnResponse, error) {
	c.queryProfitSharingReturnRequest = &req
	if c.queryProfitSharingReturnErr != nil {
		return nil, c.queryProfitSharingReturnErr
	}
	return c.queryProfitSharingReturnResponse, nil
}
func (c *refundRecoveryOrdinaryClient) UnfreezeProfitSharing(context.Context, ospcontracts.ProfitSharingUnfreezeRequest) (*ospcontracts.ProfitSharingUnfreezeResponse, error) {
	return nil, nil
}
func (c *refundRecoveryOrdinaryClient) QueryProfitSharingRemainingAmount(context.Context, ospcontracts.ProfitSharingRemainingAmountRequest) (*ospcontracts.ProfitSharingRemainingAmountResponse, error) {
	return nil, nil
}

func (r *orderRefundFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

func TestProcessTaskProfitSharingLogsRetryReceiverEnsureFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &refundRecoveryOrdinaryClient{
		addProfitSharingReceiverErrs: []error{
			nil,
			errors.New("retry receiver ensure failed"),
		},
		createProfitSharingOrderErrs: []error{
			errors.New("receiver missing before retry"),
			nil,
		},
		createProfitSharingOrderResponse: &ospcontracts.ProfitSharingOrderResponse{
			SubMchID:      "sub_mch_retry_ordinary",
			TransactionID: "wx_txn_retry_ordinary",
			OutOrderNo:    "PS91779",
			OrderID:       "ps_retry_ordinary",
			State:         ospcontracts.ProfitSharingOrderStateProcessing,
		},
	}

	paymentOrder := db.PaymentOrder{
		ID:             917,
		Amount:         10000,
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		TransactionID:  pgtype.Text{String: "wx_txn_retry_ordinary", Valid: true},
	}
	order := db.Order{ID: 79, MerchantID: 17, TotalAmount: 10000, DeliveryFee: 0, OrderType: "takeout"}
	merchant := db.Merchant{ID: 17, RegionID: 14, Name: "商户重试"}
	operator := db.Operator{ID: 45, UserID: 450, Name: "区域运营商重试"}
	operatorUser := db.User{ID: operator.UserID, WechatOpenid: "operator_openid_retry", FullName: "区域运营商重试"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_retry_ordinary"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 20, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(operator, nil)
	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(operatorUser, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
		require.Equal(t, int64(2000), arg.OperatorCommission)
		require.Equal(t, int64(8000), arg.MerchantAmount)
		return db.ProfitSharingOrder{ID: 3017, OutOrderNo: arg.OutOrderNo}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             3017,
		SharingOrderID: pgtype.Text{String: "ps_retry_ordinary", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: 3017, Status: "processing"}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateProfitSharing, arg.CommandType)
		require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
		require.Equal(t, "PS91779", arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, "ps_retry_ordinary", arg.ExternalSecondaryKey.String)
		return db.ExternalPaymentCommand{ID: 9937}, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetOrdinaryServiceProviderClient(ordinaryClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: order.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskProfitSharing(context.Background(), asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes))
	require.NoError(t, err)
	require.Len(t, ordinaryClient.addProfitSharingReceiverRequests, 2)
	require.Contains(t, logs.String(), "re-ensure operator profit sharing receiver failed before retry")
	require.Contains(t, logs.String(), "retry receiver ensure failed")
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

func TestRefundRecoverySchedulerRunOnceQueriesOrdinaryRefundStatusByOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &orderRefundFactApplicationRecorder{}
	ordinaryClient := &refundRecoveryOrdinaryClient{queryRefundResponse: &ospcontracts.RefundResponse{RefundID: "wx_refund_ordinary_order_001", Status: ospcontracts.RefundStatusAbnormal}}

	stuckRefund := db.RefundOrder{
		ID:             163,
		PaymentOrderID: 193,
		OutRefundNo:    "RFD_STUCK_ORDINARY_ORDER_001",
		Status:         "processing",
		RefundAmount:   880,
	}
	paymentOrder := db.PaymentOrder{
		ID:             193,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 1501, Valid: true},
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
	store.EXPECT().GetOrder(gomock.Any(), int64(1501)).Return(db.Order{ID: 1501, MerchantID: 17001}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(17001)).Return(db.MerchantPaymentConfig{MerchantID: 17001, SubMchID: "sub_mch_ordinary_7001"}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityPartnerRefund, arg.Capability)
		require.Equal(t, stuckRefund.OutRefundNo, arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentTerminalStatusFailed, arg.TerminalStatus)
		require.Equal(t, stuckRefund.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, "wechat:query:ordinary_service_provider:refund:"+stuckRefund.OutRefundNo+":"+db.ExternalPaymentTerminalStatusFailed, arg.DedupeKey)
		return db.ExternalPaymentFact{ID: 1812, DedupeKey: arg.DedupeKey, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             1812,
		Consumer:           "order_domain",
		BusinessObjectType: "refund_order",
		BusinessObjectID:   stuckRefund.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 1912, FactID: 1812, Consumer: "order_domain", BusinessObjectType: "refund_order", BusinessObjectID: stuckRefund.ID, Status: db.ExternalPaymentFactApplicationStatusPending}, nil)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
	scheduler.SetOrdinaryServiceProviderClient(ordinaryClient)
	scheduler.RunOnce()
	require.Equal(t, []int64{1912}, distributor.applicationIDs)
	require.NotNil(t, ordinaryClient.queryRefundRequest)
	require.Equal(t, "sub_mch_ordinary_7001", ordinaryClient.queryRefundRequest.SubMchID)
	require.Equal(t, stuckRefund.OutRefundNo, ordinaryClient.queryRefundRequest.OutRefundNo)
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

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
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

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, nil, nil)
	scheduler.SetBaofuAggregateClient(baofuClient, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{2913}, distributor.applicationIDs)
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
