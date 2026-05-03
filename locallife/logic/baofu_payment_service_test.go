package logic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBaofuPaymentServiceCreateWechatJSAPIOrderRecordsCommandBeforeClientCall(t *testing.T) {
	store := &fakeBaofuPaymentStore{}
	client := &fakeBaofuAggregatePaymentClient{
		unifiedResult: &aggregatecontracts.UnifiedOrderResult{
			TradeNo: "BFPAY202605030001",
			ChannelReturn: aggregatecontracts.ChannelReturn{
				WechatPayData: json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx123","signType":"RSA","paySign":"signature"}`),
			},
		},
	}
	now := time.Date(2026, 5, 3, 10, 11, 12, 0, time.UTC)
	service := NewBaofuPaymentService(store, client, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp123",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})
	service.now = func() time.Time { return now }

	result, err := service.CreateWechatJSAPIOrder(context.Background(), CreateBaofuWechatJSAPIOrderInput{
		PaymentOrder: db.PaymentOrder{
			ID:         88,
			UserID:     7001,
			Amount:     12345,
			OutTradeNo: "PO202605030001",
			Attach:     pgtype.Text{String: "order:42", Valid: true},
		},
		MerchantWechatSubMchID: "wx-sub-mch-001",
		PayerOpenID:            "payer-openid-secret",
		Body:                   "LocalLife订单",
		ClientIP:               "203.0.113.9",
	})

	require.NoError(t, err)
	require.True(t, store.commandCreatedBeforeClientCall)
	require.True(t, client.called)
	require.Equal(t, json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"nonce","package":"prepay_id=wx123","signType":"RSA","paySign":"signature"}`), result.WechatPayData)
	require.Equal(t, "BFPAY202605030001", result.BaofuTradeNo)

	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastCommand.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastCommand.Channel)
	require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, store.lastCommand.Capability)
	require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, store.lastCommand.CommandType)
	require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, store.lastCommand.BusinessOwner)
	require.Equal(t, "payment_order", store.lastCommand.BusinessObjectType.String)
	require.Equal(t, int64(88), store.lastCommand.BusinessObjectID.Int64)
	require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, store.lastCommand.ExternalObjectType)
	require.Equal(t, "PO202605030001", store.lastCommand.ExternalObjectKey)
	require.Equal(t, db.ExternalPaymentCommandStatusSubmitted, store.lastCommand.CommandStatus)
	require.Equal(t, now, store.lastCommand.SubmittedAt)
	require.NotContains(t, string(store.lastCommand.ResponseSnapshot), "payer-openid-secret")

	require.Equal(t, "COLLECT_MER", client.lastRequest.MerchantID)
	require.Equal(t, "COLLECT_TER", client.lastRequest.TerminalID)
	require.Equal(t, "PO202605030001", client.lastRequest.OutTradeNo)
	require.Equal(t, int64(12345), client.lastRequest.TransactionAmt)
	require.Equal(t, int64(12345), client.lastRequest.TotalAmt)
	require.Equal(t, aggregatecontracts.ProductTypeSharing, client.lastRequest.ProductType)
	require.Equal(t, aggregatecontracts.BaoCaiTongOrderTypeSharing, client.lastRequest.OrderType)
	require.Equal(t, aggregatecontracts.PayCodeWechatJSAPI, client.lastRequest.PayCode)
	require.Equal(t, "wxapp123", client.lastRequest.PayExtend.SubAppID)
	require.Equal(t, "payer-openid-secret", client.lastRequest.PayExtend.SubOpenID)
	require.Equal(t, "wx-sub-mch-001", client.lastRequest.SubMchID)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/payment", client.lastRequest.NotifyURL)
	require.Equal(t, "203.0.113.9", client.lastRequest.RiskInfo.ClientIP)
	require.Equal(t, "20260503101112", client.lastRequest.TransactionTime)
}

func TestBaofuPaymentServiceCreateWechatJSAPIOrderRejectsMissingWechatPayData(t *testing.T) {
	store := &fakeBaofuPaymentStore{}
	client := &fakeBaofuAggregatePaymentClient{unifiedResult: &aggregatecontracts.UnifiedOrderResult{}}
	service := NewBaofuPaymentService(store, client, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp123",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	_, err := service.CreateWechatJSAPIOrder(context.Background(), CreateBaofuWechatJSAPIOrderInput{
		PaymentOrder:           db.PaymentOrder{ID: 88, Amount: 12345, OutTradeNo: "PO202605030001"},
		MerchantWechatSubMchID: "wx-sub-mch-001",
		PayerOpenID:            "payer-openid-secret",
		Body:                   "LocalLife订单",
	})

	require.ErrorIs(t, err, ErrBaofuPaymentWechatPayDataRequired)
}

func TestPaymentCommandServiceRecordExternalPaymentCommand_AcceptsBaofuPaymentCommand(t *testing.T) {
	store := &fakeBaofuPaymentCommandStore{}
	now := time.Date(2026, 5, 3, 10, 30, 0, 0, time.UTC)
	businessObjectType := "payment_order"
	businessObjectID := int64(99)
	input := RecordExternalPaymentCommandInput{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuPayment,
		CommandType:        db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      db.ExternalPaymentBusinessOwnerOrder,
		BusinessObjectType: &businessObjectType,
		BusinessObjectID:   &businessObjectID,
		ExternalObjectType: db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:  "PO202605030001",
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		ResponseSnapshot:   []byte(`{"provider":"baofu"}`),
	}
	service := NewPaymentCommandService(store)
	service.now = func() time.Time { return now }

	result, err := service.RecordExternalPaymentCommand(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(707), result.Command.ID)
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.last.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.last.Channel)
	require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, store.last.ExternalObjectType)
	require.Equal(t, now, store.last.SubmittedAt)
}

type fakeBaofuPaymentStore struct {
	lastCommand                    db.CreateExternalPaymentCommandParams
	commandCreatedBeforeClientCall bool
}

func (s *fakeBaofuPaymentStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.lastCommand = arg
	s.commandCreatedBeforeClientCall = true
	return db.ExternalPaymentCommand{ID: 99, ExternalObjectKey: arg.ExternalObjectKey, CommandStatus: arg.CommandStatus}, nil
}

type fakeBaofuAggregatePaymentClient struct {
	called        bool
	lastRequest   aggregatecontracts.UnifiedOrderRequest
	unifiedResult *aggregatecontracts.UnifiedOrderResult
	err           error
}

func (c *fakeBaofuAggregatePaymentClient) CreateUnifiedOrder(ctx context.Context, req aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.called = true
	c.lastRequest = req
	if c.err != nil {
		return nil, c.err
	}
	if c.unifiedResult == nil {
		return nil, errors.New("missing result")
	}
	return c.unifiedResult, nil
}

type fakeBaofuPaymentCommandStore struct {
	last db.CreateExternalPaymentCommandParams
}

func (s *fakeBaofuPaymentCommandStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.last = arg
	return db.ExternalPaymentCommand{ID: 707, Provider: arg.Provider, Channel: arg.Channel, ExternalObjectType: arg.ExternalObjectType}, nil
}
