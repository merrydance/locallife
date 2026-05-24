package worker_test

import (
	"context"
	"encoding/json"
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

func TestProcessTaskBaofuProfitSharingCreatesShareCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuProfitSharingClient{shareResult: &aggregatecontracts.ShareResult{
		TradeNo:    "BFSHARE_UP_3001",
		OutTradeNo: "BFSHARE_3001",
		TxnState:   aggregatecontracts.ShareStateProcessing,
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		ShareNotifyURL:    "https://api.example.com/v1/webhooks/baofu/share",
	})

	profitSharingOrder := db.ProfitSharingOrder{
		ID:                    3001,
		PaymentOrderID:        4001,
		OutOrderNo:            "BFSHARE_3001",
		Status:                db.ProfitSharingOrderStatusPending,
		Provider:              db.ExternalPaymentProviderBaofu,
		Channel:               db.PaymentChannelBaofuAggregate,
		MerchantAmount:        8970,
		RiderAmount:           500,
		OperatorCommission:    300,
		PlatformCommission:    200,
		SharingDetailSnapshot: []byte(`{"provider":"baofu","channel":"baofu_aggregate","receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":8970},{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":500},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":300},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":200}]}`),
	}
	paymentOrder := db.PaymentOrder{
		ID:             4001,
		OutTradeNo:     "PO_BAOFU_4001",
		TransactionID:  pgtype.Text{String: "BFPAY_UP_4001", Valid: true},
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		Status:         "paid",
	}

	store.EXPECT().GetProfitSharingOrder(gomock.Any(), profitSharingOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateProfitSharing, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner)
		require.Equal(t, "profit_sharing_order", arg.BusinessObjectType.String)
		require.Equal(t, profitSharingOrder.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
		require.Equal(t, profitSharingOrder.OutOrderNo, arg.ExternalObjectKey)
		require.Equal(t, paymentOrder.OutTradeNo, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentCommandStatusSubmitted, arg.CommandStatus)
		require.NotContains(t, string(arg.ResponseSnapshot), "MER_SHARE")
		return db.ExternalPaymentCommand{ID: 5001}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: pgtype.Text{String: "BFSHARE_UP_3001", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: profitSharingOrder.ID, Status: db.ProfitSharingOrderStatusProcessing}, nil)

	payloadBytes, err := json.Marshal(worker.BaofuProfitSharingPayload{ProfitSharingOrderID: profitSharingOrder.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuProfitSharing(context.Background(), asynq.NewTask(worker.TaskProcessBaofuProfitSharing, payloadBytes))

	require.NoError(t, err)
	require.True(t, client.called)
	require.Equal(t, "COLLECT_MER", client.lastShareRequest.MerchantID)
	require.Equal(t, "COLLECT_TER", client.lastShareRequest.TerminalID)
	require.Equal(t, "BFPAY_UP_4001", client.lastShareRequest.OriginTradeNo)
	require.Equal(t, "BFSHARE_3001", client.lastShareRequest.OutTradeNo)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/share", client.lastShareRequest.NotifyURL)
	require.Equal(t, []aggregatecontracts.SharingDetail{
		{SharingMerID: "MER_SHARE", SharingAmountFen: 8970},
		{SharingMerID: "RIDER_SHARE", SharingAmountFen: 500},
		{SharingMerID: "OP_SHARE", SharingAmountFen: 300},
		{SharingMerID: "PLATFORM_SHARE", SharingAmountFen: 200},
	}, client.lastShareRequest.SharingDetails)
}

func TestProcessTaskBaofuProfitSharingDoesNotPersistLocalOutOrderNoAsUpstreamShareID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuProfitSharingClient{shareResult: &aggregatecontracts.ShareResult{
		OutTradeNo: "BFSHARE_3003",
		TxnState:   aggregatecontracts.ShareStateProcessing,
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
	})

	profitSharingOrder := db.ProfitSharingOrder{
		ID:                    3003,
		PaymentOrderID:        4003,
		OutOrderNo:            "BFSHARE_3003",
		Status:                db.ProfitSharingOrderStatusPending,
		Provider:              db.ExternalPaymentProviderBaofu,
		Channel:               db.PaymentChannelBaofuAggregate,
		SharingDetailSnapshot: []byte(`{"receivers":[{"sharing_mer_id":"MER_SHARE","amount":1000}]}`),
	}
	paymentOrder := db.PaymentOrder{
		ID:             4003,
		OutTradeNo:     "PO_BAOFU_4003",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		Status:         "paid",
	}

	store.EXPECT().GetProfitSharingOrder(gomock.Any(), profitSharingOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: 5003}, nil)
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: pgtype.Text{},
	}).Return(db.ProfitSharingOrder{ID: profitSharingOrder.ID, Status: db.ProfitSharingOrderStatusProcessing}, nil)

	payloadBytes, err := json.Marshal(worker.BaofuProfitSharingPayload{ProfitSharingOrderID: profitSharingOrder.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuProfitSharing(context.Background(), asynq.NewTask(worker.TaskProcessBaofuProfitSharing, payloadBytes))

	require.NoError(t, err)
}

func TestProcessTaskBaofuProfitSharingSkipsWhenRefundAmountIsOccupied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuProfitSharingClient{shareResult: &aggregatecontracts.ShareResult{
		TradeNo:    "BFSHARE_UP_3002",
		OutTradeNo: "BFSHARE_3002",
		TxnState:   aggregatecontracts.ShareStateProcessing,
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAggregateClient(client, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		ShareNotifyURL:    "https://api.example.com/v1/webhooks/baofu/share",
	})

	profitSharingOrder := db.ProfitSharingOrder{
		ID:                    3002,
		PaymentOrderID:        4002,
		OutOrderNo:            "BFSHARE_3002",
		Status:                db.ProfitSharingOrderStatusFailed,
		Provider:              db.ExternalPaymentProviderBaofu,
		Channel:               db.PaymentChannelBaofuAggregate,
		SharingDetailSnapshot: []byte(`{"receivers":[{"sharing_mer_id":"MER_SHARE","amount":1000}]}`),
	}
	paymentOrder := db.PaymentOrder{
		ID:             4002,
		OutTradeNo:     "PO_BAOFU_4002",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		Status:         "paid",
	}

	store.EXPECT().GetProfitSharingOrder(gomock.Any(), profitSharingOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(300), nil)

	payloadBytes, err := json.Marshal(worker.BaofuProfitSharingPayload{ProfitSharingOrderID: profitSharingOrder.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuProfitSharing(context.Background(), asynq.NewTask(worker.TaskProcessBaofuProfitSharing, payloadBytes))

	require.Error(t, err)
	require.Contains(t, err.Error(), "active refund amount")
	require.False(t, client.called)
}

type fakeBaofuProfitSharingClient struct {
	called           bool
	lastShareRequest aggregatecontracts.ShareAfterPayRequest
	shareResult      *aggregatecontracts.ShareResult
	err              error
}

func (c *fakeBaofuProfitSharingClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeBaofuProfitSharingClient) QueryPayment(context.Context, aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeBaofuProfitSharingClient) CreateProfitSharing(_ context.Context, req aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	c.called = true
	c.lastShareRequest = req
	if c.err != nil {
		return nil, c.err
	}
	return c.shareResult, nil
}

func (c *fakeBaofuProfitSharingClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *fakeBaofuProfitSharingClient) CreateRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *fakeBaofuProfitSharingClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *fakeBaofuProfitSharingClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, nil
}
