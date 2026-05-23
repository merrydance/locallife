package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskOrderPaymentTimeout_DelegatesPendingBaofuPaymentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &paymentTimeoutBaofuAggregateClient{
		paymentResult: &aggregatecontracts.UnifiedOrderResult{
			MerchantID:       "COLLECT_MER",
			TerminalID:       "COLLECT_TER",
			OutTradeNo:       "BF_LEGACY_TIMEOUT_1",
			TradeNo:          "BFTX_LEGACY_1",
			TxnState:         aggregatecontracts.PaymentStateWaitPaying,
			SuccessAmountFen: 4567,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		},
		closeResult: &aggregatecontracts.OrderCloseResult{
			MerchantID: "COLLECT_MER",
			TerminalID: "COLLECT_TER",
			OutTradeNo: "BF_LEGACY_TIMEOUT_1",
			ResultCode: aggregatecontracts.BusinessResultCodeSuccess,
		},
	}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClientForTest(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	order := db.Order{
		ID:        96001,
		OrderNo:   "ORD_LEGACY_TIMEOUT_1",
		UserID:    97001,
		Status:    db.OrderStatusPending,
		CreatedAt: time.Now().Add(-worker.OrderPaymentTimeoutMinutes*time.Minute - time.Minute),
	}
	paymentOrder := db.PaymentOrder{
		ID:             96002,
		OrderID:        pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:         order.UserID,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
		OutTradeNo:     "BF_LEGACY_TIMEOUT_1",
		TransactionID:  pgtype.Text{String: "BFTX_LEGACY_1", Valid: true},
		Amount:         4567,
		Status:         "pending",
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}
	closedPaymentOrder := paymentOrder
	closedPaymentOrder.Status = "closed"

	gomock.InOrder(
		store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil),
		store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).Return(paymentOrder, nil),
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
			return db.ExternalPaymentCommand{ID: 97002}, nil
		}),
		store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(closedPaymentOrder, nil),
		store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil),
		store.EXPECT().CancelOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CancelOrderTxParams) (db.CancelOrderTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, "支付超时未完成", arg.CancelReason)
			return db.CancelOrderTxResult{Order: db.Order{ID: order.ID, Status: db.OrderStatusCancelled}}, nil
		}),
	)

	task := asynq.NewTask(worker.TaskOrderPaymentTimeout, mustMarshalJSON(t, worker.PayloadOrderPaymentTimeout{OrderID: order.ID}))
	err := processor.ProcessTaskOrderPaymentTimeout(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, aggregatecontracts.PaymentQueryRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		TradeNo:    "BFTX_LEGACY_1",
	}, client.lastPaymentQuery)
	require.Equal(t, aggregatecontracts.OrderCloseRequest{
		MerchantID: "COLLECT_MER",
		TerminalID: "COLLECT_TER",
		OutTradeNo: "BF_LEGACY_TIMEOUT_1",
	}, client.lastCloseRequest)
}
