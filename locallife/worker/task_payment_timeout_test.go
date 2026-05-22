package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskPaymentOrderTimeout_DirectRemotePaidRecordsFactInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	distributor := &paymentTimeoutFactApplicationRecorder{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil, directClient)

	paymentOrder := db.PaymentOrder{
		ID:             9003,
		OutTradeNo:     "PO_DIRECT_TIMEOUT_PAID_1",
		Amount:         30000,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	paidPaymentOrder := paymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_direct_timeout_paid_1", Valid: true}

	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	directClient.EXPECT().QueryOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(&wechatcontracts.DirectOrderQueryResponse{
		OutTradeNo:    paymentOrder.OutTradeNo,
		TransactionID: "wx_direct_timeout_paid_1",
		TradeState:    wechatcontracts.DirectTradeStateSuccess,
		SuccessTime:   "2026-04-26T10:03:00+08:00",
		Amount: wechatcontracts.DirectOrderQueryAmount{
			Total:         paymentOrder.Amount,
			PayerTotal:    paymentOrder.Amount,
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	}, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "wx_direct_timeout_paid_1", Valid: true},
	}).Return(paidPaymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelDirect, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentBusinessOwnerRiderDeposit, arg.BusinessOwner.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 9203, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(9203), arg.FactID)
		require.Equal(t, "rider_deposit_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 9303, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9303), payload.ApplicationID)
		return nil
	}

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskPaymentOrderTimeout_BaofuVerifyFeeDirectRemotePaidRecordsFactInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	distributor := &paymentTimeoutFactApplicationRecorder{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil, directClient)

	paymentOrder := db.PaymentOrder{
		ID:             9004,
		OutTradeNo:     "BFVF_DIRECT_TIMEOUT_PAID_1",
		Amount:         200,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	paidPaymentOrder := paymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_baofu_verify_fee_timeout_paid_1", Valid: true}

	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	directClient.EXPECT().QueryOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(&wechatcontracts.DirectOrderQueryResponse{
		OutTradeNo:    paymentOrder.OutTradeNo,
		TransactionID: "wx_baofu_verify_fee_timeout_paid_1",
		TradeState:    wechatcontracts.DirectTradeStateSuccess,
		SuccessTime:   "2026-05-08T10:03:00+08:00",
		Amount: wechatcontracts.DirectOrderQueryAmount{
			Total:         paymentOrder.Amount,
			PayerTotal:    paymentOrder.Amount,
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	}, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "wx_baofu_verify_fee_timeout_paid_1", Valid: true},
	}).Return(paidPaymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelDirect, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityDirectJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentBusinessOwnerBaofuVerifyFee, arg.BusinessOwner.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
		return db.ExternalPaymentFact{ID: 9204, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(9204), arg.FactID)
		require.Equal(t, "baofu_account_verify_fee_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 9304, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9304), payload.ApplicationID)
		return nil
	}

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskPaymentOrderTimeout_BaofuWaitPayingClosesRemoteBeforeLocalCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &paymentTimeoutBaofuAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			MerchantID:       "COLLECT_MER",
			TerminalID:       "COLLECT_TER",
			OutTradeNo:       "BF_TIMEOUT_WAIT_1",
			TradeNo:          "BFTX_9010",
			TxnState:         aggregatecontracts.PaymentStateWaitPaying,
			SuccessAmountFen: 12345,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		},
		closeResult: &aggregatecontracts.OrderCloseResult{
			MerchantID: "COLLECT_MER",
			TerminalID: "COLLECT_TER",
			OutTradeNo: "BF_TIMEOUT_WAIT_1",
			ResultCode: aggregatecontracts.BusinessResultCodeSuccess,
		},
	}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClientForTest(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	paymentOrder := db.PaymentOrder{
		ID:             9010,
		OutTradeNo:     "BF_TIMEOUT_WAIT_1",
		Amount:         12345,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 91010, Valid: true},
		TransactionID:  pgtype.Text{String: "BFTX_9010", Valid: true},
		Attach:         pgtype.Text{String: "sub_mchid:1900000112", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	businessOrder := db.Order{ID: paymentOrder.OrderID.Int64, UserID: 92010, Status: db.OrderStatusPending}

	gomock.InOrder(
		store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil),
		store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeClosePayment, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner)
			require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
			return db.ExternalPaymentCommand{ID: 93010}, nil
		}),
		store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil),
		store.EXPECT().GetOrderForUpdate(gomock.Any(), paymentOrder.OrderID.Int64).Return(businessOrder, nil),
		store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CancelOrderTxParams) (db.CancelOrderTxResult, error) {
			require.Equal(t, businessOrder.ID, arg.OrderID)
			return db.CancelOrderTxResult{Order: db.Order{ID: businessOrder.ID, Status: db.OrderStatusCancelled}}, nil
		}),
	)

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		TradeNo:    "BFTX_9010",
	}, client.lastPaymentQuery)
	require.Equal(t, aggregatecontracts.OrderCloseRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_TIMEOUT_WAIT_1",
	}, client.lastCloseRequest)
}

func TestProcessTaskPaymentOrderTimeout_BaofuQueryErrorStopsLocalClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &paymentTimeoutBaofuAggregateClient{paymentErr: errors.New("baofu payment query failed")}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClientForTest(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	paymentOrder := db.PaymentOrder{
		ID:             9011,
		OutTradeNo:     "BF_TIMEOUT_QUERY_ERR_1",
		Amount:         22345,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 91011, Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}

	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)

	require.Error(t, err)
	require.Contains(t, err.Error(), "query baofu payment order before timeout close")
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_TIMEOUT_QUERY_ERR_1",
	}, client.lastPaymentQuery)
}

func TestProcessTaskPaymentOrderTimeout_BaofuCloseErrorStopsLocalClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &paymentTimeoutBaofuAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			MerchantID:       "COLLECT_MER",
			TerminalID:       "COLLECT_TER",
			OutTradeNo:       "BF_TIMEOUT_CLOSE_ERR_1",
			TxnState:         aggregatecontracts.PaymentStateWaitPaying,
			SuccessAmountFen: 12345,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		},
		closeErr: errors.New("baofu close failed"),
	}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClientForTest(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	paymentOrder := db.PaymentOrder{
		ID:             9013,
		OutTradeNo:     "BF_TIMEOUT_CLOSE_ERR_1",
		Amount:         12345,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 91013, Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}

	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 93013}, nil)

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)

	require.Error(t, err)
	require.Contains(t, err.Error(), "close baofu payment order before local timeout close")
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_TIMEOUT_CLOSE_ERR_1",
	}, client.lastPaymentQuery)
	require.Equal(t, aggregatecontracts.OrderCloseRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_TIMEOUT_CLOSE_ERR_1",
	}, client.lastCloseRequest)
}

func TestProcessTaskPaymentOrderTimeout_BaofuSuccessRecordsFactInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &paymentTimeoutBaofuAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			MerchantID:       "COLLECT_MER",
			TerminalID:       "COLLECT_TER",
			OutTradeNo:       "BF_TIMEOUT_SUCCESS_1",
			TradeNo:          "BFTX_SUCCESS_9012",
			TxnState:         aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 32345,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		},
	}
	distributor := &paymentTimeoutFactApplicationRecorder{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	processor.SetBaofuAggregateClientForTest(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	paymentOrder := db.PaymentOrder{
		ID:             9012,
		OutTradeNo:     "BF_TIMEOUT_SUCCESS_1",
		Amount:         32345,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 91012, Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
		require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, "BFTX_SUCCESS_9012", arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		require.Equal(t, paymentOrder.Amount, arg.Amount.Int64)
		return db.ExternalPaymentFact{ID: 92012, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(92012), arg.FactID)
		require.Equal(t, "order_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 93012, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(93012), payload.ApplicationID)
		return nil
	}

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_TIMEOUT_SUCCESS_1",
	}, client.lastPaymentQuery)
	require.Empty(t, client.lastCloseRequest)
}

type paymentTimeoutFactApplicationRecorder struct {
	worker.NoopTaskDistributor
	processPaymentFactApplication func(context.Context, *worker.PaymentFactApplicationPayload, ...asynq.Option) error
}

func (d *paymentTimeoutFactApplicationRecorder) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	if d.processPaymentFactApplication == nil {
		return nil
	}
	return d.processPaymentFactApplication(ctx, payload, opts...)
}

type paymentTimeoutBaofuAggregateClient struct {
	paymentResult    *aggregatecontracts.UnifiedOrderResult
	paymentErr       error
	closeResult      *aggregatecontracts.OrderCloseResult
	closeErr         error
	lastPaymentQuery aggregatecontracts.PaymentQueryRequest
	lastCloseRequest aggregatecontracts.OrderCloseRequest
}

func (c *paymentTimeoutBaofuAggregateClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *paymentTimeoutBaofuAggregateClient) QueryPayment(_ context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastPaymentQuery = req
	if c.paymentErr != nil {
		return nil, c.paymentErr
	}
	return c.paymentResult, nil
}

func (c *paymentTimeoutBaofuAggregateClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *paymentTimeoutBaofuAggregateClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *paymentTimeoutBaofuAggregateClient) CreateRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *paymentTimeoutBaofuAggregateClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *paymentTimeoutBaofuAggregateClient) CloseOrder(_ context.Context, req aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	c.lastCloseRequest = req
	if c.closeErr != nil {
		return nil, c.closeErr
	}
	return c.closeResult, nil
}
