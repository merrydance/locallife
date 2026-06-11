package logic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

var (
	ErrBaofuWithdrawServiceNotConfigured = errors.New("baofu withdraw service is not configured")
	ErrBaofuWithdrawAccountNotReady      = errors.New("宝付结算账户未开通，暂不能提现")
	ErrBaofuWithdrawContractNoRequired   = errors.New("宝付结算账户缺少提现账户标识，暂不能提现")
	ErrBaofuWithdrawFeeMemberIDRequired  = errors.New("宝付结算账户缺少提现手续费承担方标识，暂不能提现")
	ErrBaofuWithdrawBalanceUnavailable   = errors.New("baofu withdraw balance unavailable")
	ErrBaofuWithdrawInsufficientBalance  = errors.New("可提现金额不足")
	ErrBaofuWithdrawCreateRejected       = errors.New("baofu withdraw create rejected")
	ErrBaofuWithdrawCreateResultUnknown  = errors.New("baofu withdraw create result unknown")
)

type baofuWithdrawStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	GetBaofuWithdrawalOrderByIdempotency(ctx context.Context, arg db.GetBaofuWithdrawalOrderByIdempotencyParams) (db.BaofuWithdrawalOrder, error)
	CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx context.Context, arg db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error)
	UpdateBaofuWithdrawalOrderToProcessing(ctx context.Context, arg db.UpdateBaofuWithdrawalOrderToProcessingParams) (db.BaofuWithdrawalOrder, error)
	UpdateBaofuWithdrawalOrderStatus(ctx context.Context, arg db.UpdateBaofuWithdrawalOrderStatusParams) (db.BaofuWithdrawalOrder, error)
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
	upstream, err := s.client.CreateWithdraw(ctx, baofucontracts.WithdrawRequest{
		MerchantID:    cfg.PayoutMerchantID,
		TerminalID:    cfg.PayoutTerminalID,
		ContractNo:    strings.TrimSpace(binding.ContractNo.String),
		TransSerialNo: outRequestNo,
		AmountFen:     input.AmountFen,
		FeeMemberID:   feeMemberID,
		NotifyURL:     cfg.WithdrawNotifyURL,
	})
	if err != nil {
		result.SyncState = "unknown"
		result.UserMessage = "提现申请已提交，结果正在确认，请勿重复提交"
		s.logBaofuWithdrawCreateUnknown(err, input, result.WithdrawalOrder)
		s.recordBaofuWithdrawCommand(ctx, input.OwnerType, result.WithdrawalOrder, nil, db.ExternalPaymentCommandStatusUnknown, "create_withdraw_unknown", "provider create result unknown; recovery will query by out_request_no", err)
		return result, fmt.Errorf("%w: %w", ErrBaofuWithdrawCreateResultUnknown, err)
	}
	if upstream == nil {
		result.SyncState = "unknown"
		result.UserMessage = "提现申请已提交，结果正在确认，请勿重复提交"
		emptyErr := errors.New("baofu withdraw returned empty result")
		s.logBaofuWithdrawCreateUnknown(emptyErr, input, result.WithdrawalOrder)
		s.recordBaofuWithdrawCommand(ctx, input.OwnerType, result.WithdrawalOrder, nil, db.ExternalPaymentCommandStatusUnknown, "create_withdraw_unknown", "provider create result unknown; recovery will query by out_request_no", emptyErr)
		return result, fmt.Errorf("%w: empty provider result", ErrBaofuWithdrawCreateResultUnknown)
	}
	raw := []byte(upstream.Raw)
	if len(raw) == 0 || !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	if upstream.Status == "" {
		upstream.Status = baofucontracts.WithdrawAcceptanceStatusFromUpstream(upstream.UpstreamState)
	}
	if upstream.Status == db.BaofuWithdrawalStatusFailed {
		updated, updateErr := s.store.UpdateBaofuWithdrawalOrderStatus(ctx, db.UpdateBaofuWithdrawalOrderStatusParams{
			ID:     result.WithdrawalOrder.ID,
			Status: db.BaofuWithdrawalStatusFailed,
			BaofuWithdrawNo: pgtype.Text{
				String: strings.TrimSpace(upstream.BaofuWithdrawNo),
				Valid:  strings.TrimSpace(upstream.BaofuWithdrawNo) != "",
			},
			RawSnapshot: raw,
		})
		if updateErr != nil {
			log.Error().
				Err(updateErr).
				Str("owner_type", strings.TrimSpace(input.OwnerType)).
				Int64("owner_id", input.OwnerID).
				Int64("baofu_withdrawal_order_id", result.WithdrawalOrder.ID).
				Str("out_request_no", outRequestNo).
				Str("upstream_state", strings.TrimSpace(upstream.UpstreamState)).
				Msg("mark baofu withdrawal create rejected failed")
			return result, fmt.Errorf("mark baofu withdrawal create rejected: %w", updateErr)
		}
		result.WithdrawalOrder = updated
		result.SyncState = "rejected"
		result.UserMessage = "提现申请未被受理，请刷新余额后重试"
		log.Warn().
			Str("owner_type", strings.TrimSpace(input.OwnerType)).
			Int64("owner_id", input.OwnerID).
			Int64("baofu_withdrawal_order_id", result.WithdrawalOrder.ID).
			Str("out_request_no", outRequestNo).
			Str("upstream_state", strings.TrimSpace(upstream.UpstreamState)).
			Str("provider_error_code", "baofu_acceptance_rejected").
			Str("provider_error_message", strings.TrimSpace(upstream.Remark)).
			Msg("baofu withdrawal create rejected by provider")
		s.recordBaofuWithdrawCommand(ctx, input.OwnerType, updated, upstream, db.ExternalPaymentCommandStatusRejected, "baofu_acceptance_rejected", strings.TrimSpace(upstream.Remark), nil)
		return result, ErrBaofuWithdrawCreateRejected
	}
	if upstream.Status != db.BaofuWithdrawalStatusProcessing {
		return result, fmt.Errorf("unsupported baofu withdraw acceptance status %q", upstream.Status)
	}
	updated, err := s.store.UpdateBaofuWithdrawalOrderToProcessing(ctx, db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID: result.WithdrawalOrder.ID,
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
	result.SyncState = "accepted"
	s.recordBaofuWithdrawCommand(ctx, input.OwnerType, updated, upstream, db.ExternalPaymentCommandStatusAccepted, "", "", nil)
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

func baofuWithdrawCommandSnapshot(order db.BaofuWithdrawalOrder, upstream *baofucontracts.WithdrawResult, cause error) []byte {
	payload := map[string]any{
		"baofu_withdrawal_order_id": order.ID,
		"provider":                  db.ExternalPaymentProviderBaofu,
		"operation":                 "create_baofu_withdraw",
		"owner_type":                order.OwnerType,
		"owner_id":                  order.OwnerID,
		"out_request_no":            strings.TrimSpace(order.OutRequestNo),
		"amount":                    order.Amount,
		"status":                    order.Status,
	}
	if upstream != nil {
		if v := strings.TrimSpace(upstream.BaofuWithdrawNo); v != "" {
			payload["baofu_withdraw_no"] = v
		}
		if v := strings.TrimSpace(upstream.UpstreamState); v != "" {
			payload["upstream_state"] = v
		}
		if v := strings.TrimSpace(upstream.Status); v != "" {
			payload["acceptance_status"] = v
		}
		if v := baofu.SanitizeUpstreamMessageForRecord(upstream.Remark); v != "" {
			payload["remark_sanitized"] = v
		}
	}
	var providerErr *baofu.ProviderError
	if errors.As(cause, &providerErr) && providerErr != nil {
		payload["error_present"] = true
		if v := strings.TrimSpace(providerErr.Operation); v != "" {
			payload["provider_operation"] = v
		}
		if v := strings.TrimSpace(providerErr.Capability); v != "" {
			payload["provider_capability"] = v
		}
		if providerErr.StatusCode != 0 {
			payload["http_status"] = providerErr.StatusCode
		}
		if v := strings.TrimSpace(providerErr.RequestID); v != "" {
			payload["request_id"] = v
		}
		if v := strings.TrimSpace(providerErr.UpstreamCode); v != "" {
			payload["upstream_code"] = v
		}
		if v := baofu.SanitizeUpstreamMessageForRecord(providerErr.UpstreamMessage); v != "" {
			payload["upstream_message_sanitized"] = v
		}
		if len(providerErr.DiagnosticSnapshot) > 0 && json.Valid(providerErr.DiagnosticSnapshot) {
			payload["provider_error_snapshot"] = json.RawMessage(providerErr.DiagnosticSnapshot)
		}
	} else if cause != nil {
		payload["error_present"] = true
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func (s *BaofuWithdrawService) recordBaofuWithdrawCommand(ctx context.Context, ownerType string, order db.BaofuWithdrawalOrder, upstream *baofucontracts.WithdrawResult, status string, errorCode string, errorMessage string, cause error) {
	now := s.now().UTC()
	var secondary pgtype.Text
	if upstream != nil {
		if withdrawNo := strings.TrimSpace(upstream.BaofuWithdrawNo); withdrawNo != "" {
			secondary = pgtype.Text{String: withdrawNo, Valid: true}
		}
	}
	arg := db.CreateExternalPaymentCommandParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:          db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
		BusinessOwner:        businessOwnerForBaofuWithdrawal(ownerType),
		BusinessObjectType:   pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: order.ID, Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    strings.TrimSpace(order.OutRequestNo),
		ExternalSecondaryKey: secondary,
		CommandStatus:        status,
		SubmittedAt:          now,
		ResponseSnapshot:     baofuWithdrawCommandSnapshot(order, upstream, cause),
	}
	if status == db.ExternalPaymentCommandStatusAccepted {
		arg.AcceptedAt = pgtype.Timestamptz{Time: now, Valid: true}
	}
	if status == db.ExternalPaymentCommandStatusRejected {
		arg.RejectedAt = pgtype.Timestamptz{Time: now, Valid: true}
	}
	if trimmed := baofuWithdrawCommandErrorCode(errorCode, cause); trimmed != "" {
		arg.LastErrorCode = pgtype.Text{String: trimmed, Valid: true}
	}
	if trimmed := baofuWithdrawCommandErrorMessage(errorMessage, cause); trimmed != "" {
		arg.LastErrorMessage = pgtype.Text{String: trimmed, Valid: true}
	}
	if _, err := s.store.CreateExternalPaymentCommand(ctx, arg); err != nil {
		log.Error().
			Err(err).
			Int64("baofu_withdrawal_order_id", order.ID).
			Str("out_request_no", strings.TrimSpace(order.OutRequestNo)).
			Str("command_status", status).
			Msg("record baofu withdrawal command failed")
	}
}

func baofuWithdrawCommandErrorCode(defaultCode string, cause error) string {
	var providerErr *baofu.ProviderError
	if errors.As(cause, &providerErr) && providerErr != nil {
		if code := strings.TrimSpace(providerErr.UpstreamCode); code != "" {
			return code
		}
	}
	return strings.TrimSpace(defaultCode)
}

func baofuWithdrawCommandErrorMessage(defaultMessage string, cause error) string {
	var providerErr *baofu.ProviderError
	if errors.As(cause, &providerErr) && providerErr != nil {
		if strings.TrimSpace(providerErr.UpstreamCode) != "" || strings.TrimSpace(providerErr.UpstreamMessage) != "" {
			return strings.TrimSpace(baofu.BaofuCommandMessage(providerErr.UpstreamCode, providerErr.UpstreamMessage))
		}
	}
	if message := strings.TrimSpace(defaultMessage); message != "" {
		return baofu.SanitizeUpstreamMessageForRecord(message)
	}
	if cause != nil {
		return baofu.SanitizeUpstreamMessageForRecord(cause.Error())
	}
	return ""
}

func (s *BaofuWithdrawService) logBaofuWithdrawCreateUnknown(err error, input BaofuCreateWithdrawalInput, order db.BaofuWithdrawalOrder) {
	log.Error().
		Err(err).
		Str("owner_type", strings.TrimSpace(input.OwnerType)).
		Int64("owner_id", input.OwnerID).
		Int64("baofu_withdrawal_order_id", order.ID).
		Str("out_request_no", strings.TrimSpace(order.OutRequestNo)).
		Int64("amount_fen", input.AmountFen).
		Str("sync_state", "unknown").
		Str("provider", db.ExternalPaymentProviderBaofu).
		Str("capability", db.ExternalPaymentCapabilityBaofuWithdraw).
		Msg("baofu withdrawal create result unknown after local order persisted")
}
