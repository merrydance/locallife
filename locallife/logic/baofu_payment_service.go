package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuPaymentServiceNotConfigured  = errors.New("baofu payment service is not configured")
	ErrBaofuPaymentInvalidInput          = errors.New("baofu payment input is invalid")
	ErrBaofuPaymentWechatPayDataRequired = errors.New("baofu payment missing wechat pay data")
)

type baofuPaymentStore interface {
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
}

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
	PaymentOrder           db.PaymentOrder
	MerchantWechatSubMchID string
	PayerOpenID            string
	Body                   string
	PageURL                string
	ClientIP               string
	BusinessOwner          string
}

type CreateBaofuWechatJSAPIOrderResult struct {
	PaymentOrder  db.PaymentOrder
	BaofuTradeNo  string
	WechatPayData json.RawMessage
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
		SubMchID:   strings.TrimSpace(input.MerchantWechatSubMchID),
		SubAppID:   strings.TrimSpace(cfg.MiniProgramAppID),
		SubOpenID:  strings.TrimSpace(input.PayerOpenID),
		Body:       strings.TrimSpace(input.Body),
		NotifyURL:  strings.TrimSpace(cfg.PaymentNotifyURL),
		PageURL:    strings.TrimSpace(input.PageURL),
		ClientIP:   strings.TrimSpace(input.ClientIP),
		Attach:     strings.TrimSpace(paymentOrder.Attach.String),
	})
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

func validateCreateBaofuWechatJSAPIOrderInput(input CreateBaofuWechatJSAPIOrderInput) error {
	paymentOrder := input.PaymentOrder
	if paymentOrder.ID == 0 || strings.TrimSpace(paymentOrder.OutTradeNo) == "" || paymentOrder.Amount <= 0 {
		return ErrBaofuPaymentInvalidInput
	}
	if strings.TrimSpace(input.MerchantWechatSubMchID) == "" {
		return ErrBaofuAccountWechatSubMchRequired
	}
	if strings.TrimSpace(input.PayerOpenID) == "" || strings.TrimSpace(input.Body) == "" {
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
