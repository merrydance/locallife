package worker_test

import (
	"context"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeWorkerBaofuRefundClient struct {
	called            bool
	lastRefundRequest aggregatecontracts.RefundBeforeShareRequest
	refundResult      *aggregatecontracts.RefundResult
	err               error
}

func (c *fakeWorkerBaofuRefundClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryPayment(context.Context, aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CreateRefund(ctx context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	c.called = true
	c.lastRefundRequest = req
	if c.err != nil {
		return nil, c.err
	}
	if c.refundResult != nil {
		return c.refundResult, nil
	}
	return &aggregatecontracts.RefundResult{OutTradeNo: req.OutTradeNo, TradeNo: "BFREFUND_TEST"}, nil
}

func (c *fakeWorkerBaofuRefundClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, nil
}

func (c *fakeWorkerBaofuRefundClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, nil
}

func expectWorkerExternalRefundCommand(t require.TestingT, store *mockdb.MockStore, provider string, channel string, capability string, refundOrderID int64, outRefundNo string, refundID string, businessOwner string, status string, errorCode string, commandID int64) {
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, provider, arg.Provider)
		require.Equal(t, channel, arg.Channel)
		require.Equal(t, capability, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
		require.Equal(t, businessOwner, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "refund_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, refundOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
		require.Equal(t, outRefundNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outRefundNo)
		if refundID != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, refundID, arg.ExternalSecondaryKey.String)
			require.Contains(t, string(arg.ResponseSnapshot), refundID)
		}
		if errorCode != "" {
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), errorCode)
		}
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}

func expectWorkerExternalRefundAcceptedCommand(t require.TestingT, store *mockdb.MockStore, channel string, capability string, refundOrderID int64, outRefundNo string, refundID string, businessOwner string, commandID int64) {
	expectWorkerExternalRefundCommand(t, store, db.ExternalPaymentProviderWechat, channel, capability, refundOrderID, outRefundNo, refundID, businessOwner, db.ExternalPaymentCommandStatusAccepted, "", commandID)
}
