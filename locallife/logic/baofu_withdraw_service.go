package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuWithdrawServiceNotConfigured = errors.New("baofu withdraw service is not configured")
	ErrBaofuWithdrawAccountNotReady      = errors.New("宝付结算账户未开通，暂不能提现")
	ErrBaofuWithdrawContractNoRequired   = errors.New("宝付结算账户缺少提现账户标识，暂不能提现")
	ErrBaofuWithdrawBalanceUnavailable   = errors.New("baofu withdraw balance unavailable")
	ErrBaofuWithdrawInsufficientBalance  = errors.New("可提现金额不足")
)

type baofuWithdrawStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	CreateBaofuWithdrawalOrder(ctx context.Context, arg db.CreateBaofuWithdrawalOrderParams) (db.BaofuWithdrawalOrder, error)
	UpdateBaofuWithdrawalOrderToProcessing(ctx context.Context, arg db.UpdateBaofuWithdrawalOrderToProcessingParams) (db.BaofuWithdrawalOrder, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
}

type BaofuWithdrawClient interface {
	QueryBalance(ctx context.Context, req baofucontracts.BalanceQueryRequest) (*baofucontracts.BalanceResult, error)
	CreateWithdraw(ctx context.Context, req baofucontracts.WithdrawRequest) (*baofucontracts.WithdrawResult, error)
	QueryWithdraw(ctx context.Context, req baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error)
}

type BaofuWithdrawServiceConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	PayoutMerchantID  string
	PayoutTerminalID  string
	WithdrawNotifyURL string
}

type BaofuWithdrawService struct {
	store  baofuWithdrawStore
	client BaofuWithdrawClient
	config BaofuWithdrawServiceConfig
	now    func() time.Time
}

func NewBaofuWithdrawService(store baofuWithdrawStore, client BaofuWithdrawClient, config BaofuWithdrawServiceConfig) *BaofuWithdrawService {
	return &BaofuWithdrawService{
		store:  store,
		client: client,
		config: config.normalized(),
		now:    time.Now,
	}
}

type BaofuBalanceQueryInput struct {
	OwnerType string
	OwnerID   int64
}

type BaofuBalanceQueryResult struct {
	Binding            db.BaofuAccountBinding
	AvailableAmountFen int64
	PendingAmountFen   int64
	LedgerAmountFen    int64
	FrozenAmountFen    int64
}

type BaofuCreateWithdrawalInput struct {
	OwnerType    string
	OwnerID      int64
	AmountFen    int64
	OutRequestNo string
}

type BaofuCreateWithdrawalResult struct {
	WithdrawalOrder db.BaofuWithdrawalOrder
}

func (s *BaofuWithdrawService) QueryBalance(ctx context.Context, input BaofuBalanceQueryInput) (BaofuBalanceQueryResult, error) {
	var result BaofuBalanceQueryResult
	if s == nil || s.store == nil || s.client == nil {
		return result, ErrBaofuWithdrawServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		return result, ErrBaofuWithdrawServiceNotConfigured
	}
	binding, err := s.readyBinding(ctx, input.OwnerType, input.OwnerID)
	if err != nil {
		return result, err
	}
	result.Binding = binding

	upstream, err := s.client.QueryBalance(ctx, baofucontracts.BalanceQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
		ContractNo: strings.TrimSpace(binding.ContractNo.String),
	})
	if err != nil {
		return result, err
	}
	if upstream == nil {
		return result, errors.New("baofu balance query returned empty result")
	}
	result.AvailableAmountFen = upstream.AvailableAmountFen
	result.PendingAmountFen = upstream.PendingAmountFen
	result.LedgerAmountFen = upstream.LedgerAmountFen
	result.FrozenAmountFen = upstream.FrozenAmountFen
	return result, nil
}

func (s *BaofuWithdrawService) CreateWithdrawal(ctx context.Context, input BaofuCreateWithdrawalInput) (BaofuCreateWithdrawalResult, error) {
	var result BaofuCreateWithdrawalResult
	if s == nil || s.store == nil || s.client == nil {
		return result, ErrBaofuWithdrawServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.PayoutMerchantID == "" || cfg.PayoutTerminalID == "" {
		return result, ErrBaofuWithdrawServiceNotConfigured
	}
	if input.AmountFen <= 0 {
		return result, errors.New("提现金额必须大于0")
	}
	outRequestNo := strings.TrimSpace(input.OutRequestNo)
	if outRequestNo == "" {
		return result, errors.New("宝付提现单号不能为空")
	}
	binding, err := s.readyBinding(ctx, input.OwnerType, input.OwnerID)
	if err != nil {
		return result, err
	}
	balance, err := s.client.QueryBalance(ctx, baofucontracts.BalanceQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
		ContractNo: strings.TrimSpace(binding.ContractNo.String),
	})
	if err != nil {
		return result, fmt.Errorf("%w: %w", ErrBaofuWithdrawBalanceUnavailable, err)
	}
	if balance == nil {
		return result, fmt.Errorf("%w: empty result", ErrBaofuWithdrawBalanceUnavailable)
	}
	if input.AmountFen > balance.AvailableAmountFen {
		return result, ErrBaofuWithdrawInsufficientBalance
	}
	submittedSnapshot := []byte(`{"state":"submitted"}`)
	withdrawalOrder, err := s.store.CreateBaofuWithdrawalOrder(ctx, db.CreateBaofuWithdrawalOrderParams{
		OwnerType:        strings.TrimSpace(input.OwnerType),
		OwnerID:          input.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     outRequestNo,
		Amount:           input.AmountFen,
		Status:           db.BaofuWithdrawalStatusProcessing,
		RawSnapshot:      submittedSnapshot,
	})
	if err != nil {
		return result, err
	}
	if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
		BusinessOwner:      businessOwnerForBaofuWithdrawal(input.OwnerType),
		BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: withdrawalOrder.ID, Valid: true},
		ExternalObjectType: db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  outRequestNo,
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   baofuWithdrawCommandSnapshot(withdrawalOrder),
	}); err != nil {
		return result, err
	}
	upstream, err := s.client.CreateWithdraw(ctx, baofucontracts.WithdrawRequest{
		MerchantID:    cfg.PayoutMerchantID,
		TerminalID:    cfg.PayoutTerminalID,
		ContractNo:    strings.TrimSpace(binding.ContractNo.String),
		TransSerialNo: outRequestNo,
		AmountFen:     input.AmountFen,
		NotifyURL:     cfg.WithdrawNotifyURL,
	})
	if err != nil {
		return result, err
	}
	if upstream == nil {
		return result, errors.New("baofu withdraw returned empty result")
	}
	raw := []byte(upstream.Raw)
	if len(raw) == 0 || !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	updated, err := s.store.UpdateBaofuWithdrawalOrderToProcessing(ctx, db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID: withdrawalOrder.ID,
		BaofuWithdrawNo: pgtype.Text{
			String: strings.TrimSpace(upstream.BaofuWithdrawNo),
			Valid:  strings.TrimSpace(upstream.BaofuWithdrawNo) != "",
		},
		RawSnapshot: raw,
	})
	if err != nil {
		return result, err
	}
	result.WithdrawalOrder = updated
	return result, nil
}

func businessOwnerForBaofuWithdrawal(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	case db.BaofuAccountOwnerTypeRider:
		return "rider"
	case db.BaofuAccountOwnerTypeOperator:
		return "operator"
	case db.BaofuAccountOwnerTypePlatform:
		return "platform"
	default:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	}
}

func (s *BaofuWithdrawService) readyBinding(ctx context.Context, ownerType string, ownerID int64) (db.BaofuAccountBinding, error) {
	binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: strings.TrimSpace(ownerType),
		OwnerID:   ownerID,
	})
	if err != nil {
		return db.BaofuAccountBinding{}, err
	}
	if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
		return db.BaofuAccountBinding{}, ErrBaofuWithdrawAccountNotReady
	}
	if strings.TrimSpace(binding.ContractNo.String) == "" {
		return db.BaofuAccountBinding{}, ErrBaofuWithdrawContractNoRequired
	}
	return binding, nil
}

func (c BaofuWithdrawServiceConfig) normalized() BaofuWithdrawServiceConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.PayoutMerchantID = strings.TrimSpace(c.PayoutMerchantID)
	c.PayoutTerminalID = strings.TrimSpace(c.PayoutTerminalID)
	c.WithdrawNotifyURL = strings.TrimSpace(c.WithdrawNotifyURL)
	return c
}

func baofuWithdrawCommandSnapshot(order db.BaofuWithdrawalOrder) []byte {
	payload := map[string]any{
		"baofu_withdrawal_order_id": order.ID,
		"owner_type":                order.OwnerType,
		"owner_id":                  order.OwnerID,
		"amount":                    order.Amount,
		"status":                    order.Status,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}
