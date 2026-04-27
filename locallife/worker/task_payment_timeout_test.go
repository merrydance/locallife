package worker_test

import (
	"context"
	"testing"
	"time"

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

func TestProcessTaskPaymentOrderTimeout_EcommerceRemotePaidRecordsFactInsteadOfClosing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	distributor := &paymentTimeoutFactApplicationRecorder{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)

	paymentOrder := db.PaymentOrder{
		ID:             9001,
		OutTradeNo:     "PO_TIMEOUT_REMOTE_PAID_1",
		Amount:         5800,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 9101, Valid: true},
		Attach:         pgtype.Text{String: "sub_mchid:1900000109", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	paidPaymentOrder := paymentOrder
	paidPaymentOrder.Status = "paid"
	paidPaymentOrder.TransactionID = pgtype.Text{String: "wx_tx_timeout_paid_1", Valid: true}

	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil)
	ecommerceClient.EXPECT().QueryPartnerOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo, "1900000109").Return(&wechatcontracts.PartnerOrderQueryResponse{
		OutTradeNo:    paymentOrder.OutTradeNo,
		TransactionID: "wx_tx_timeout_paid_1",
		TradeState:    "SUCCESS",
		SuccessTime:   "2026-04-26T10:00:00+08:00",
		Amount: wechatcontracts.PartnerOrderQueryAmount{
			Total:         paymentOrder.Amount,
			PayerTotal:    paymentOrder.Amount,
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	}, nil)
	store.EXPECT().UpdatePaymentOrderToPaid(gomock.Any(), db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "wx_tx_timeout_paid_1", Valid: true},
	}).Return(paidPaymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
		require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalObjectKey)
		require.Equal(t, "wx_tx_timeout_paid_1", arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		return db.ExternalPaymentFact{ID: 9201, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(9201), arg.FactID)
		require.Equal(t, "order_domain", arg.Consumer)
		require.Equal(t, "payment_order", arg.BusinessObjectType)
		require.Equal(t, paymentOrder.ID, arg.BusinessObjectID)
		return db.ExternalPaymentFactApplication{ID: 9301, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID}, nil
	})
	distributor.processPaymentFactApplication = func(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(9301), payload.ApplicationID)
		return nil
	}

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskPaymentOrderTimeout_EcommerceNotPayClosesRemoteBeforeLocalCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)

	paymentOrder := db.PaymentOrder{
		ID:             9002,
		OutTradeNo:     "PO_TIMEOUT_NOTPAY_1",
		Amount:         4500,
		Status:         "pending",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OrderID:        pgtype.Int8{Int64: 9102, Valid: true},
		Attach:         pgtype.Text{String: "sub_mchid:1900000110", Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"
	businessOrder := db.Order{ID: paymentOrder.OrderID.Int64, UserID: 9202, Status: db.OrderStatusPending}

	gomock.InOrder(
		store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo).Return(paymentOrder, nil),
		ecommerceClient.EXPECT().QueryPartnerOrderByOutTradeNo(gomock.Any(), paymentOrder.OutTradeNo, "1900000110").Return(&wechatcontracts.PartnerOrderQueryResponse{
			OutTradeNo: paymentOrder.OutTradeNo,
			TradeState: "NOTPAY",
			Amount: wechatcontracts.PartnerOrderQueryAmount{
				Total:         paymentOrder.Amount,
				PayerTotal:    paymentOrder.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil),
		ecommerceClient.EXPECT().ClosePartnerOrder(gomock.Any(), paymentOrder.OutTradeNo, "1900000110").Return(nil),
		store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil),
		store.EXPECT().GetOrderForUpdate(gomock.Any(), paymentOrder.OrderID.Int64).Return(businessOrder, nil),
		store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CancelOrderTxParams) (db.CancelOrderTxResult, error) {
			require.Equal(t, businessOrder.ID, arg.OrderID)
			require.Equal(t, db.OrderStatusPending, arg.OldStatus)
			require.Equal(t, "支付超时未完成", arg.CancelReason)
			require.Equal(t, businessOrder.UserID, arg.OperatorID)
			require.Equal(t, "system", arg.OperatorType)
			return db.CancelOrderTxResult{Order: db.Order{ID: businessOrder.ID, Status: db.OrderStatusCancelled}}, nil
		}),
	)

	task := asynq.NewTask(worker.TaskPaymentOrderTimeout, mustMarshalJSON(t, worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo}))
	err := processor.ProcessTaskPaymentOrderTimeout(context.Background(), task)
	require.NoError(t, err)
}

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
