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
		MerchantSubMchID: "wx-sub-mch-001",
		PayerOpenID:      "payer-openid-secret",
		Body:             "LocalLife订单",
		ClientIP:         "203.0.113.9",
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
		PaymentOrder:     db.PaymentOrder{ID: 88, Amount: 12345, OutTradeNo: "PO202605030001"},
		MerchantSubMchID: "wx-sub-mch-001",
		PayerOpenID:      "payer-openid-secret",
		Body:             "LocalLife订单",
		ClientIP:         "203.0.113.9",
	})

	require.ErrorIs(t, err, ErrBaofuPaymentWechatPayDataRequired)
}

func TestBaofuPaymentServiceCreateWechatJSAPIOrderRejectsMissingClientIP(t *testing.T) {
	store := &fakeBaofuPaymentStore{}
	client := &fakeBaofuAggregatePaymentClient{}
	service := NewBaofuPaymentService(store, client, BaofuPaymentServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		MiniProgramAppID:  "wxapp123",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
	})

	_, err := service.CreateWechatJSAPIOrder(context.Background(), CreateBaofuWechatJSAPIOrderInput{
		PaymentOrder:     db.PaymentOrder{ID: 88, Amount: 12345, OutTradeNo: "PO202605030001"},
		MerchantSubMchID: "wx-sub-mch-001",
		PayerOpenID:      "payer-openid-secret",
		Body:             "LocalLife订单",
	})

	require.ErrorIs(t, err, ErrBaofuPaymentRiskInfoClientIPRequired)
	require.False(t, store.commandCreatedBeforeClientCall)
	require.False(t, client.called)
}

func TestBaofuPaymentServiceRecordPaymentCallbackFactCreatesTerminalApplication(t *testing.T) {
	store := &fakeBaofuPaymentStore{}
	now := time.Date(2026, 5, 3, 10, 20, 0, 0, time.UTC)
	occurredAt := time.Date(2026, 5, 3, 10, 15, 0, 0, time.UTC)
	service := NewBaofuPaymentService(store, nil, BaofuPaymentServiceConfig{})
	service.now = func() time.Time { return now }

	result, err := service.RecordPaymentFact(context.Background(), RecordBaofuPaymentFactInput{
		PaymentOrder: db.PaymentOrder{
			ID:         88,
			Amount:     12345,
			OutTradeNo: "PO202605030001",
		},
		FactSource:      db.ExternalPaymentFactSourceCallback,
		SourceEventID:   "BFN202605030001",
		SourceEventType: "PAYMENT",
		OccurredAt:      occurredAt,
		Fact: aggregatecontracts.PaymentFact{
			OutTradeNo:       "PO202605030001",
			TradeNo:          "BFPAY202605030001",
			TransactionState: aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 12345,
			FeeAmountFen:     37,
			Raw:              json.RawMessage(`{"outTradeNo":"PO202605030001","tradeNo":"BFPAY202605030001","txnState":"SUCCESS","sub_openid":"payer-openid-secret"}`),
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(501), result.Fact.ID)
	require.NotNil(t, result.Application)
	require.Equal(t, int64(601), result.Application.ID)
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastFact.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastFact.Channel)
	require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, store.lastFact.Capability)
	require.Equal(t, db.ExternalPaymentFactSourceCallback, store.lastFact.FactSource)
	require.Equal(t, "BFN202605030001", store.lastFact.SourceEventID.String)
	require.Equal(t, "PAYMENT", store.lastFact.SourceEventType.String)
	require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, store.lastFact.ExternalObjectType)
	require.Equal(t, "PO202605030001", store.lastFact.ExternalObjectKey)
	require.Equal(t, "BFPAY202605030001", store.lastFact.ExternalSecondaryKey.String)
	require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, store.lastFact.BusinessOwner.String)
	require.Equal(t, "payment_order", store.lastFact.BusinessObjectType.String)
	require.Equal(t, int64(88), store.lastFact.BusinessObjectID.Int64)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, store.lastFact.UpstreamState)
	require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, store.lastFact.TerminalStatus)
	require.True(t, store.lastFact.IsTerminal)
	require.Equal(t, int64(12345), store.lastFact.Amount.Int64)
	require.Equal(t, "CNY", store.lastFact.Currency)
	require.Equal(t, occurredAt, store.lastFact.OccurredAt.Time)
	require.Equal(t, now, store.lastFact.ObservedAt)
	require.Equal(t, "baofu:callback:payment:PO202605030001:BFN202605030001", store.lastFact.DedupeKey)
	require.NotContains(t, string(store.lastFact.RawResource), "sharingMerId")
	require.True(t, store.providerFeeActualUpserted)
	require.Equal(t, db.OrderPaymentFeeTypeProviderPaymentFee, store.lastProviderFeeActual.FeeType)
	require.Equal(t, db.OrderPaymentFeePayerTypePlatform, store.lastProviderFeeActual.PayerType)
	require.Equal(t, db.OrderPaymentFeePayeeTypeBaofu, store.lastProviderFeeActual.PayeeType)
	require.Equal(t, int64(88), store.lastProviderFeeActual.PaymentOrderID)
	require.Equal(t, int64(12345), store.lastProviderFeeActual.BaseAmount)
	require.Equal(t, int64(37), store.lastProviderFeeActual.Amount)
	require.Equal(t, db.OrderPaymentFeeAmountSourceActualCallback, store.lastProviderFeeActual.AmountSource)
	require.Equal(t, int64(501), store.lastProviderFeeActual.ExternalPaymentFactID.Int64)
	require.Equal(t, int64(501), store.lastApplication.FactID)
	require.Equal(t, "order_domain", store.lastApplication.Consumer)
	require.Equal(t, "payment_order", store.lastApplication.BusinessObjectType)
	require.Equal(t, int64(88), store.lastApplication.BusinessObjectID)
}

func TestBaofuPaymentServiceRecordPaymentQueryFactSkipsProcessingApplication(t *testing.T) {
	store := &fakeBaofuPaymentStore{}
	now := time.Date(2026, 5, 3, 10, 25, 0, 0, time.UTC)
	service := NewBaofuPaymentService(store, nil, BaofuPaymentServiceConfig{})
	service.now = func() time.Time { return now }

	result, err := service.RecordPaymentFact(context.Background(), RecordBaofuPaymentFactInput{
		PaymentOrder: db.PaymentOrder{
			ID:         88,
			Amount:     12345,
			OutTradeNo: "PO202605030001",
		},
		FactSource: db.ExternalPaymentFactSourceQuery,
		Fact: aggregatecontracts.PaymentFact{
			OutTradeNo:       "PO202605030001",
			TransactionState: aggregatecontracts.PaymentStateWaitPaying,
			Raw:              json.RawMessage(`{"outTradeNo":"PO202605030001","txnState":"WAIT_PAYING"}`),
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(501), result.Fact.ID)
	require.Nil(t, result.Application)
	require.Equal(t, db.ExternalPaymentTerminalStatusProcessing, store.lastFact.TerminalStatus)
	require.False(t, store.lastFact.IsTerminal)
	require.Equal(t, "baofu:query:payment:PO202605030001:WAIT_PAYING", store.lastFact.DedupeKey)
	require.False(t, store.applicationCreated)
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
	lastFact                       db.CreateExternalPaymentFactParams
	lastApplication                db.CreateExternalPaymentFactApplicationParams
	lastProviderFeeActual          db.UpsertOrderPaymentFeeLedgerActualParams
	commandCreatedBeforeClientCall bool
	applicationCreated             bool
	providerFeeActualUpserted      bool
}

func (s *fakeBaofuPaymentStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.lastCommand = arg
	s.commandCreatedBeforeClientCall = true
	return db.ExternalPaymentCommand{ID: 99, ExternalObjectKey: arg.ExternalObjectKey, CommandStatus: arg.CommandStatus}, nil
}

func (s *fakeBaofuPaymentStore) CreateExternalPaymentFact(ctx context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
	s.lastFact = arg
	return db.ExternalPaymentFact{
		ID:                 501,
		Provider:           arg.Provider,
		Channel:            arg.Channel,
		Capability:         arg.Capability,
		FactSource:         arg.FactSource,
		ExternalObjectType: arg.ExternalObjectType,
		ExternalObjectKey:  arg.ExternalObjectKey,
		UpstreamState:      arg.UpstreamState,
		TerminalStatus:     arg.TerminalStatus,
		IsTerminal:         arg.IsTerminal,
		DedupeKey:          arg.DedupeKey,
		ProcessingStatus:   arg.ProcessingStatus,
	}, nil
}

func (s *fakeBaofuPaymentStore) CreateExternalPaymentFactApplication(ctx context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
	s.lastApplication = arg
	s.applicationCreated = true
	return db.ExternalPaymentFactApplication{
		ID:                 601,
		FactID:             arg.FactID,
		Consumer:           arg.Consumer,
		BusinessObjectType: arg.BusinessObjectType,
		BusinessObjectID:   arg.BusinessObjectID,
		Status:             arg.Status,
	}, nil
}

func (s *fakeBaofuPaymentStore) UpsertOrderPaymentFeeLedgerActual(ctx context.Context, arg db.UpsertOrderPaymentFeeLedgerActualParams) (db.OrderPaymentFeeLedger, error) {
	s.lastProviderFeeActual = arg
	s.providerFeeActualUpserted = true
	return db.OrderPaymentFeeLedger{ID: 701, Amount: arg.Amount, AmountSource: arg.AmountSource}, nil
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

func (c *fakeBaofuAggregatePaymentClient) CreateProfitSharing(ctx context.Context, req aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

func (c *fakeBaofuAggregatePaymentClient) QueryPayment(ctx context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

func (c *fakeBaofuAggregatePaymentClient) QueryProfitSharing(ctx context.Context, req aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

func (c *fakeBaofuAggregatePaymentClient) CreateRefund(ctx context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

func (c *fakeBaofuAggregatePaymentClient) QueryRefund(ctx context.Context, req aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

func (c *fakeBaofuAggregatePaymentClient) CloseOrder(ctx context.Context, req aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, errors.New("not implemented in payment tests")
}

type fakeBaofuPaymentCommandStore struct {
	last db.CreateExternalPaymentCommandParams
}

func (s *fakeBaofuPaymentCommandStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.last = arg
	return db.ExternalPaymentCommand{ID: 707, Provider: arg.Provider, Channel: arg.Channel, ExternalObjectType: arg.ExternalObjectType}, nil
}
