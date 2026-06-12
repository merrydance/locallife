package logic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
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
	ErrBaofuWithdrawFeeMemberIDRequired  = errors.New("宝付结算账户缺少提现手续费承担方标识，暂不能提现")
	ErrBaofuWithdrawBalanceUnavailable   = errors.New("baofu withdraw balance unavailable")
	ErrBaofuWithdrawInsufficientBalance  = errors.New("可提现金额不足")
)

type baofuWithdrawStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	GetBaofuWithdrawalOrderByIdempotency(ctx context.Context, arg db.GetBaofuWithdrawalOrderByIdempotencyParams) (db.BaofuWithdrawalOrder, error)
	CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx context.Context, arg db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error)
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
	OwnerType      string
	OwnerID        int64
	AmountFen      int64
	OutRequestNo   string
	IdempotencyKey string
}

type BaofuCreateWithdrawalResult struct {
	WithdrawalOrder     db.BaofuWithdrawalOrder
	SyncState           string
	UserMessage         string
	IdempotencyReplayed bool
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
		MerchantID:  cfg.CollectMerchantID,
		TerminalID:  cfg.CollectTerminalID,
		ContractNo:  strings.TrimSpace(binding.ContractNo.String),
		AccountType: strings.TrimSpace(binding.AccountType),
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
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("idempotency key is required"))
	}
	requestHash := baofuWithdrawalCreateRequestHash(input.OwnerType, input.OwnerID, input.AmountFen)
	replay, replayed, err := s.replayBaofuWithdrawalByIdempotency(ctx, input, idempotencyKey, requestHash)
	if err != nil {
		return result, err
	}
	if replayed {
		return replay, nil
	}
	outRequestNo := strings.TrimSpace(input.OutRequestNo)
	if outRequestNo == "" {
		return result, errors.New("宝付提现单号不能为空")
	}
	binding, err := s.readyBinding(ctx, input.OwnerType, input.OwnerID)
	if err != nil {
		return result, err
	}
	feeMemberID := strings.TrimSpace(binding.SharingMerID.String)
	if feeMemberID == "" {
		return result, ErrBaofuWithdrawFeeMemberIDRequired
	}
	balance, err := s.client.QueryBalance(ctx, baofucontracts.BalanceQueryRequest{
		MerchantID:  cfg.CollectMerchantID,
		TerminalID:  cfg.CollectTerminalID,
		ContractNo:  strings.TrimSpace(binding.ContractNo.String),
		AccountType: strings.TrimSpace(binding.AccountType),
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
	submittedAt := s.now().UTC()
	submittedTxResult, err := s.store.CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx, db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams{
		WithdrawalOrder: db.CreateBaofuWithdrawalOrderParams{
			OwnerType:        strings.TrimSpace(input.OwnerType),
			OwnerID:          input.OwnerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     outRequestNo,
			Amount:           input.AmountFen,
			Status:           db.BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: idempotencyKey,
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: requestHash,
				Valid:  true,
			},
		},
		BusinessOwner: businessOwnerForBaofuWithdrawal(input.OwnerType),
		SubmittedAt:   submittedAt,
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			return s.replayBaofuWithdrawalByUniqueConflict(ctx, input, idempotencyKey, requestHash, err)
		}
		return result, err
	}
	result.WithdrawalOrder = submittedTxResult.WithdrawalOrder
	result.SyncState = "unknown"
	result.UserMessage = "提现申请已提交，结果正在确认，请勿重复提交"
	return result, nil
}

func (s *BaofuWithdrawService) replayBaofuWithdrawalByIdempotency(ctx context.Context, input BaofuCreateWithdrawalInput, idempotencyKey string, requestHash string) (BaofuCreateWithdrawalResult, bool, error) {
	var result BaofuCreateWithdrawalResult
	existing, err := s.store.GetBaofuWithdrawalOrderByIdempotency(ctx, db.GetBaofuWithdrawalOrderByIdempotencyParams{
		OwnerType: strings.TrimSpace(input.OwnerType),
		OwnerID:   input.OwnerID,
		IdempotencyKey: pgtype.Text{
			String: idempotencyKey,
			Valid:  true,
		},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, false, nil
		}
		return result, false, fmt.Errorf("get baofu withdrawal idempotency: %w", err)
	}
	if strings.TrimSpace(existing.IdempotencyRequestHash.String) != requestHash {
		return result, false, NewRequestError(http.StatusConflict, errors.New("idempotency key already used by a different withdrawal request"))
	}
	result.WithdrawalOrder = existing
	result.IdempotencyReplayed = true
	return result, true, nil
}

func (s *BaofuWithdrawService) replayBaofuWithdrawalByUniqueConflict(ctx context.Context, input BaofuCreateWithdrawalInput, idempotencyKey string, requestHash string, createErr error) (BaofuCreateWithdrawalResult, error) {
	result, replayed, err := s.replayBaofuWithdrawalByIdempotency(ctx, input, idempotencyKey, requestHash)
	if err != nil {
		return result, err
	}
	if replayed {
		return result, nil
	}
	return result, fmt.Errorf("create baofu withdrawal order: %w", createErr)
}

func baofuWithdrawalCreateRequestHash(ownerType string, ownerID int64, amountFen int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("owner_type=%s\nowner_id=%d\namount_fen=%d", strings.TrimSpace(ownerType), ownerID, amountFen)))
	return fmt.Sprintf("sha256:%x", sum)
}

func businessOwnerForBaofuWithdrawal(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		return db.ExternalPaymentBusinessOwnerMerchantFunds
	case db.BaofuAccountOwnerTypeRider:
		return db.ExternalPaymentBusinessOwnerRiderIncome
	case db.BaofuAccountOwnerTypeOperator:
		return db.ExternalPaymentBusinessOwnerOperatorFunds
	case db.BaofuAccountOwnerTypePlatform:
		return db.ExternalPaymentBusinessOwnerPlatformFunds
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
