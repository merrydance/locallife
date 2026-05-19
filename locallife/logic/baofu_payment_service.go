package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuPaymentServiceNotConfigured     = errors.New("baofu payment service is not configured")
	ErrBaofuPaymentInvalidInput             = errors.New("baofu payment input is invalid")
	ErrBaofuPaymentWechatPayDataRequired    = errors.New("baofu payment missing wechat pay data")
	ErrBaofuPaymentMerchantSubMchIDRequired = errors.New("baofu payment merchant report sub mch id is required")
	ErrBaofuPaymentRiskInfoClientIPRequired = aggregatecontracts.ErrUnifiedOrderRiskInfoClientIPRequired
)

type baofuPaymentStore interface {
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
}

type baofuPaymentFactStore interface {
	CreateExternalPaymentFact(ctx context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error)
	CreateExternalPaymentFactApplication(ctx context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error)
}

type baofuPaymentFeeActualStore interface {
	UpsertOrderPaymentFeeLedgerActual(ctx context.Context, arg db.UpsertOrderPaymentFeeLedgerActualParams) (db.OrderPaymentFeeLedger, error)
}

const (
	baofuOrderPaymentFactConsumerDomain       = "order_domain"
	baofuReservationPaymentFactConsumerDomain = "reservation_domain"
	baofuPaymentFactBusinessObjectOrder       = "payment_order"
)

type BaofuPaymentServiceConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	MiniProgramAppID  string
	PaymentNotifyURL  string
	TimeExpireMinutes int
}

type BaofuPaymentService struct {
	store  baofuPaymentStore
	client aggregatepay.Client
	config BaofuPaymentServiceConfig
	now    func() time.Time
}

func NewBaofuPaymentService(store baofuPaymentStore, client aggregatepay.Client, config BaofuPaymentServiceConfig) *BaofuPaymentService {
	return &BaofuPaymentService{
		store:  store,
		client: client,
		config: config.normalized(),
		now:    time.Now,
	}
}

type CreateBaofuWechatJSAPIOrderInput struct {
	PaymentOrder     db.PaymentOrder
	MerchantSubMchID string
	PayerOpenID      string
	Body             string
	PageURL          string
	ClientIP         string
	BusinessOwner    string
}

type CreateBaofuWechatJSAPIOrderResult struct {
	PaymentOrder  db.PaymentOrder
	BaofuTradeNo  string
	WechatPayData json.RawMessage
}

type CloseBaofuOrderInput struct {
	PaymentOrder  db.PaymentOrder
	BusinessOwner string
}

type QueryBaofuOrderInput struct {
	PaymentOrder db.PaymentOrder
}

type RecordBaofuPaymentFactInput struct {
	PaymentOrder    db.PaymentOrder
	Fact            aggregatecontracts.PaymentFact
	FactSource      string
	SourceEventID   string
	SourceEventType string
	OccurredAt      time.Time
	ObservedAt      time.Time
}

func (s *BaofuPaymentService) CreateWechatJSAPIOrder(ctx context.Context, input CreateBaofuWechatJSAPIOrderInput) (CreateBaofuWechatJSAPIOrderResult, error) {
	var result CreateBaofuWechatJSAPIOrderResult
	if s == nil || s.store == nil || s.client == nil {
		return result, ErrBaofuPaymentServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.MiniProgramAppID == "" || cfg.PaymentNotifyURL == "" {
		return result, ErrBaofuPaymentServiceNotConfigured
	}
	if err := validateCreateBaofuWechatJSAPIOrderInput(input); err != nil {
		return result, err
	}

	paymentOrder := input.PaymentOrder
	businessOwner := strings.TrimSpace(input.BusinessOwner)
	if businessOwner == "" {
		businessOwner = db.ExternalPaymentBusinessOwnerOrder
	}
	submittedAt := s.now().UTC()
	if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuPayment,
		CommandType:        db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:      businessOwner,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
		ExternalObjectType: db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:  strings.TrimSpace(paymentOrder.OutTradeNo),
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        submittedAt,
		ResponseSnapshot:   buildBaofuUnifiedOrderCommandSnapshot(paymentOrder),
	}); err != nil {
		return result, err
	}

	req := aggregatecontracts.NewWechatJSAPISharingUnifiedOrderRequest(aggregatecontracts.UnifiedOrderInput{
		MerchantID: strings.TrimSpace(cfg.CollectMerchantID),
		TerminalID: strings.TrimSpace(cfg.CollectTerminalID),
		OutTradeNo: strings.TrimSpace(paymentOrder.OutTradeNo),
		AmountFen:  paymentOrder.Amount,
		TxnTime:    submittedAt.Format("20060102150405"),
		TimeExpire: cfg.TimeExpireMinutes,
		SubMchID:   strings.TrimSpace(input.MerchantSubMchID),
		SubAppID:   strings.TrimSpace(cfg.MiniProgramAppID),
		SubOpenID:  strings.TrimSpace(input.PayerOpenID),
		Body:       strings.TrimSpace(input.Body),
		NotifyURL:  strings.TrimSpace(cfg.PaymentNotifyURL),
		PageURL:    strings.TrimSpace(input.PageURL),
		ClientIP:   strings.TrimSpace(input.ClientIP),
		Attach:     strings.TrimSpace(paymentOrder.Attach.String),
	})
	if err := req.Validate(); err != nil {
		return result, err
	}
	upstreamResult, err := s.client.CreateUnifiedOrder(ctx, req)
	if err != nil {
		return result, err
	}
	if upstreamResult == nil {
		return result, ErrBaofuPaymentWechatPayDataRequired
	}
	wechatPayData, err := upstreamResult.WechatPayData()
	if err != nil {
		return result, ErrBaofuPaymentWechatPayDataRequired
	}
	result.PaymentOrder = paymentOrder
	result.BaofuTradeNo = strings.TrimSpace(upstreamResult.TradeNo)
	result.WechatPayData = wechatPayData
	return result, nil
}

func (s *BaofuPaymentService) RecordPaymentFact(ctx context.Context, input RecordBaofuPaymentFactInput) (RecordExternalPaymentFactResult, error) {
	var result RecordExternalPaymentFactResult
	if s == nil || s.store == nil {
		return result, ErrBaofuPaymentServiceNotConfigured
	}
	factStore, ok := s.store.(baofuPaymentFactStore)
	if !ok {
		return result, ErrBaofuPaymentServiceNotConfigured
	}
	if err := validateRecordBaofuPaymentFactInput(input); err != nil {
		return result, err
	}

	paymentOrder := input.PaymentOrder
	paymentFact := input.Fact
	outTradeNo := strings.TrimSpace(paymentFact.OutTradeNo)
	if outTradeNo == "" {
		outTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}
	upstreamState := strings.TrimSpace(paymentFact.TransactionState)
	terminalStatus := aggregatecontracts.NormalizePaymentTerminalStatus(upstreamState)
	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = s.now().UTC()
	}
	occurredAt := input.OccurredAt
	occurredAtParam := pgtype.Timestamptz{}
	if !occurredAt.IsZero() {
		occurredAtParam = pgtype.Timestamptz{Time: occurredAt.UTC(), Valid: true}
	}
	amount := paymentFact.SuccessAmountFen
	sourceEventID := strings.TrimSpace(input.SourceEventID)
	sourceEventType := strings.TrimSpace(input.SourceEventType)
	rawResource := paymentFact.Raw
	if len(rawResource) == 0 {
		rawResource = []byte(`{}`)
	}
	businessOwner := strings.TrimSpace(paymentOrder.BusinessType)
	if businessOwner == "" {
		businessOwner = db.ExternalPaymentBusinessOwnerOrder
	}
	consumer := baofuPaymentFactConsumer(businessOwner)

	fact, err := factStore.CreateExternalPaymentFact(ctx, db.CreateExternalPaymentFactParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuPayment,
		FactSource:           strings.TrimSpace(input.FactSource),
		SourceEventID:        pgtype.Text{String: sourceEventID, Valid: sourceEventID != ""},
		SourceEventType:      pgtype.Text{String: sourceEventType, Valid: sourceEventType != ""},
		ExternalObjectType:   db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:    outTradeNo,
		ExternalSecondaryKey: pgtype.Text{String: strings.TrimSpace(paymentFact.TradeNo), Valid: strings.TrimSpace(paymentFact.TradeNo) != ""},
		BusinessOwner:        pgtype.Text{String: businessOwner, Valid: businessOwner != ""},
		BusinessObjectType:   pgtype.Text{String: baofuPaymentFactBusinessObjectOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           isExternalPaymentTerminalStatus(terminalStatus),
		Amount:               pgtype.Int8{Int64: amount, Valid: amount > 0},
		Currency:             "CNY",
		OccurredAt:           occurredAtParam,
		ObservedAt:           observedAt.UTC(),
		RawResource:          rawResource,
		DedupeKey:            baofuPaymentFactDedupeKey(input, outTradeNo, upstreamState),
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	})
	if err != nil {
		return result, err
	}
	result.Fact = fact
	if paymentFact.FeeAmountFen > 0 {
		feeStore, ok := s.store.(baofuPaymentFeeActualStore)
		if !ok {
			return result, ErrBaofuPaymentServiceNotConfigured
		}
		if _, err := feeStore.UpsertOrderPaymentFeeLedgerActual(ctx, db.UpsertOrderPaymentFeeLedgerActualParams{
			Provider:              db.ExternalPaymentProviderBaofu,
			Channel:               db.PaymentChannelBaofuAggregate,
			PaymentOrderID:        paymentOrder.ID,
			FeeType:               db.OrderPaymentFeeTypeProviderPaymentFee,
			PayerType:             db.OrderPaymentFeePayerTypePlatform,
			PayeeType:             db.OrderPaymentFeePayeeTypeBaofu,
			BaseAmount:            amount,
			RateBps:               DefaultBaofuProviderPaymentFeeRateBps,
			Amount:                paymentFact.FeeAmountFen,
			AmountSource:          baofuPaymentFeeAmountSource(input.FactSource),
			ExternalPaymentFactID: pgtype.Int8{Int64: fact.ID, Valid: fact.ID > 0},
			Status:                db.OrderPaymentFeeStatusRecorded,
			CalculationVersion:    BaofuSettlementCalculationVersionV2,
		}); err != nil {
			return result, err
		}
	}
	if !fact.IsTerminal {
		return result, nil
	}
	application, err := factStore.CreateExternalPaymentFactApplication(ctx, db.CreateExternalPaymentFactApplicationParams{
		FactID:             fact.ID,
		Consumer:           consumer,
		BusinessObjectType: baofuPaymentFactBusinessObjectOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	})
	if err != nil {
		return result, err
	}
	result.Application = &application
	return result, nil
}

func baofuPaymentFactConsumer(businessOwner string) string {
	switch strings.TrimSpace(businessOwner) {
	case db.ExternalPaymentBusinessOwnerReservation, reservationAddonBusiness:
		return baofuReservationPaymentFactConsumerDomain
	default:
		return baofuOrderPaymentFactConsumerDomain
	}
}

func (s *BaofuPaymentService) CloseOrder(ctx context.Context, input CloseBaofuOrderInput) (*aggregatecontracts.OrderCloseResult, error) {
	if s == nil || s.store == nil || s.client == nil {
		return nil, ErrBaofuPaymentServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		return nil, ErrBaofuPaymentServiceNotConfigured
	}
	paymentOrder := input.PaymentOrder
	if paymentOrder.ID == 0 || strings.TrimSpace(paymentOrder.OutTradeNo) == "" {
		return nil, ErrBaofuPaymentInvalidInput
	}
	businessOwner := strings.TrimSpace(input.BusinessOwner)
	if businessOwner == "" {
		businessOwner = db.ExternalPaymentBusinessOwnerOrder
	}
	if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuPayment,
		CommandType:        db.ExternalPaymentCommandTypeClosePayment,
		BusinessOwner:      businessOwner,
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
		ExternalObjectType: db.ExternalPaymentObjectBaofuPaymentOrder,
		ExternalObjectKey:  strings.TrimSpace(paymentOrder.OutTradeNo),
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   buildBaofuOrderCloseCommandSnapshot(paymentOrder),
	}); err != nil {
		return nil, err
	}
	return s.client.CloseOrder(ctx, aggregatecontracts.OrderCloseRequest{
		MerchantID: strings.TrimSpace(cfg.CollectMerchantID),
		TerminalID: strings.TrimSpace(cfg.CollectTerminalID),
		OutTradeNo: strings.TrimSpace(paymentOrder.OutTradeNo),
	})
}

func (s *BaofuPaymentService) QueryOrder(ctx context.Context, input QueryBaofuOrderInput) (*aggregatecontracts.UnifiedOrderResult, error) {
	if s == nil || s.client == nil {
		return nil, ErrBaofuPaymentServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		return nil, ErrBaofuPaymentServiceNotConfigured
	}
	paymentOrder := input.PaymentOrder
	if paymentOrder.ID == 0 || strings.TrimSpace(paymentOrder.OutTradeNo) == "" {
		return nil, ErrBaofuPaymentInvalidInput
	}
	req := aggregatecontracts.PaymentQueryRequest{
		MerchantID: strings.TrimSpace(cfg.CollectMerchantID),
		TerminalID: strings.TrimSpace(cfg.CollectTerminalID),
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.TradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}
	return s.client.QueryPayment(ctx, req)
}

func validateCreateBaofuWechatJSAPIOrderInput(input CreateBaofuWechatJSAPIOrderInput) error {
	paymentOrder := input.PaymentOrder
	if paymentOrder.ID == 0 || strings.TrimSpace(paymentOrder.OutTradeNo) == "" || paymentOrder.Amount <= 0 {
		return ErrBaofuPaymentInvalidInput
	}
	if strings.TrimSpace(input.MerchantSubMchID) == "" {
		return ErrBaofuPaymentMerchantSubMchIDRequired
	}
	if strings.TrimSpace(input.PayerOpenID) == "" || strings.TrimSpace(input.Body) == "" {
		return ErrBaofuPaymentInvalidInput
	}
	if strings.TrimSpace(input.ClientIP) == "" {
		return ErrBaofuPaymentRiskInfoClientIPRequired
	}
	return nil
}

func validateRecordBaofuPaymentFactInput(input RecordBaofuPaymentFactInput) error {
	if input.PaymentOrder.ID == 0 || strings.TrimSpace(input.PaymentOrder.OutTradeNo) == "" {
		return ErrBaofuPaymentInvalidInput
	}
	if !isExternalPaymentFactSource(input.FactSource) {
		return fmt.Errorf("unsupported fact source %q", input.FactSource)
	}
	outTradeNo := strings.TrimSpace(input.Fact.OutTradeNo)
	if outTradeNo != "" && outTradeNo != strings.TrimSpace(input.PaymentOrder.OutTradeNo) {
		return ErrBaofuPaymentInvalidInput
	}
	return nil
}

func (c BaofuPaymentServiceConfig) normalized() BaofuPaymentServiceConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.MiniProgramAppID = strings.TrimSpace(c.MiniProgramAppID)
	c.PaymentNotifyURL = strings.TrimSpace(c.PaymentNotifyURL)
	if c.TimeExpireMinutes == 0 {
		c.TimeExpireMinutes = 30
	}
	return c
}

func buildBaofuUnifiedOrderCommandSnapshot(paymentOrder db.PaymentOrder) []byte {
	snapshot := struct {
		Provider              string `json:"provider"`
		Operation             string `json:"operation"`
		OutTradeNo            string `json:"out_trade_no"`
		AmountFen             int64  `json:"amount_fen"`
		PayCode               string `json:"pay_code"`
		SubMchIDProvided      bool   `json:"sub_mch_id_provided"`
		PayerOpenIDRedacted   bool   `json:"payer_openid_redacted"`
		RequiresProfitSharing bool   `json:"requires_profit_sharing"`
	}{
		Provider:              db.ExternalPaymentProviderBaofu,
		Operation:             "unified_order",
		OutTradeNo:            strings.TrimSpace(paymentOrder.OutTradeNo),
		AmountFen:             paymentOrder.Amount,
		PayCode:               aggregatecontracts.PayCodeWechatJSAPI,
		SubMchIDProvided:      true,
		PayerOpenIDRedacted:   true,
		RequiresProfitSharing: true,
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"unified_order"}`)
	}
	return raw
}

func buildBaofuOrderCloseCommandSnapshot(paymentOrder db.PaymentOrder) []byte {
	snapshot := struct {
		Provider   string `json:"provider"`
		Operation  string `json:"operation"`
		OutTradeNo string `json:"out_trade_no"`
	}{
		Provider:   db.ExternalPaymentProviderBaofu,
		Operation:  "order_close",
		OutTradeNo: strings.TrimSpace(paymentOrder.OutTradeNo),
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"order_close"}`)
	}
	return raw
}

func baofuPaymentFactDedupeKey(input RecordBaofuPaymentFactInput, outTradeNo string, upstreamState string) string {
	source := strings.TrimSpace(input.FactSource)
	if source == db.ExternalPaymentFactSourceCallback && strings.TrimSpace(input.SourceEventID) != "" {
		return fmt.Sprintf("baofu:callback:payment:%s:%s", outTradeNo, strings.TrimSpace(input.SourceEventID))
	}
	secondary := strings.TrimSpace(input.Fact.TradeNo)
	if secondary == "" {
		secondary = strings.TrimSpace(upstreamState)
	}
	if secondary == "" {
		secondary = db.ExternalPaymentTerminalStatusUnknown
	}
	statusComponent := strings.TrimSpace(upstreamState)
	if statusComponent == "" {
		statusComponent = db.ExternalPaymentTerminalStatusUnknown
	}
	if secondary == statusComponent {
		return fmt.Sprintf("baofu:%s:payment:%s:%s", source, outTradeNo, secondary)
	}
	return fmt.Sprintf("baofu:%s:payment:%s:%s:%s", source, outTradeNo, secondary, statusComponent)
}

func baofuPaymentFeeAmountSource(factSource string) string {
	if strings.TrimSpace(factSource) == db.ExternalPaymentFactSourceCallback {
		return db.OrderPaymentFeeAmountSourceActualCallback
	}
	return db.OrderPaymentFeeAmountSourceActualQuery
}
