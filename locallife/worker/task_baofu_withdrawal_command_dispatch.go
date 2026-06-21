package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessBaofuWithdrawalCommandDispatch = "baofu:process_withdrawal_command_dispatch"
	baofuWithdrawalCommandDispatchUniqueTTL   = 30 * time.Minute

	baofuWithdrawalCommandDispatchStartedCode    = "baofu_withdraw_dispatch_started"
	baofuWithdrawalCommandDispatchStartedMessage = "宝付提现派发已开始，结果将通过回调或查询确认"
	baofuWithdrawalCommandAcceptedCode           = "baofu_withdraw_accepted"
	baofuWithdrawalCommandRejectedCode           = "baofu_acceptance_rejected"
	baofuWithdrawalCommandUnknownCode            = "create_withdraw_unknown"
	baofuWithdrawalCommandUnknownMessage         = "宝付提现请求结果暂不确定，系统将通过查询恢复"
)

type BaofuWithdrawalCommandDispatchConfig struct {
	PayoutMerchantID  string
	PayoutTerminalID  string
	WithdrawNotifyURL string
}

type BaofuWithdrawalCommandDispatchPayload struct {
	CommandID int64 `json:"command_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessBaofuWithdrawalCommandDispatch(ctx context.Context, payload *BaofuWithdrawalCommandDispatchPayload, opts ...asynq.Option) error {
	if payload == nil || payload.CommandID <= 0 {
		return fmt.Errorf("baofu withdrawal command id is required")
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal baofu withdrawal command dispatch payload: %w", err)
	}
	if len(opts) == 0 {
		opts = append(opts, asynq.Queue(QueueCritical), asynq.MaxRetry(5), asynq.Unique(baofuWithdrawalCommandDispatchUniqueTTL))
	}
	task := asynq.NewTask(TaskProcessBaofuWithdrawalCommandDispatch, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			log.Info().
				Str("type", task.Type()).
				Int64("command_id", payload.CommandID).
				Msg("duplicate baofu withdrawal command dispatch task enqueue suppressed")
			return nil
		}
		return fmt.Errorf("enqueue baofu withdrawal command dispatch task: %w", err)
	}
	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("command_id", payload.CommandID).
		Msg("enqueued baofu withdrawal command dispatch task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskBaofuWithdrawalCommandDispatch(ctx context.Context, task *asynq.Task) error {
	var payload BaofuWithdrawalCommandDispatchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal baofu withdrawal command dispatch payload: %w", asynq.SkipRetry)
	}
	if payload.CommandID <= 0 {
		return fmt.Errorf("baofu withdrawal command id is required: %w", asynq.SkipRetry)
	}
	if processor.baofuWithdrawClient == nil {
		return fmt.Errorf("baofu withdraw client not configured: %w", asynq.SkipRetry)
	}
	cfg := processor.baofuWithdrawalConfig.normalized()
	if cfg.PayoutMerchantID == "" || cfg.PayoutTerminalID == "" || cfg.WithdrawNotifyURL == "" {
		return fmt.Errorf("baofu withdrawal dispatch config not configured: %w", asynq.SkipRetry)
	}

	command, err := processor.store.GetExternalPaymentCommand(ctx, payload.CommandID)
	if err != nil {
		return fmt.Errorf("get baofu withdrawal command: %w", err)
	}
	if err := validateBaofuWithdrawalCommand(command); err != nil {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	if command.CommandStatus != db.ExternalPaymentCommandStatusSubmitted &&
		command.CommandStatus != db.ExternalPaymentCommandStatusUnknown {
		return nil
	}
	withdrawalOrderID := command.BusinessObjectID.Int64
	if !command.BusinessObjectID.Valid || withdrawalOrderID <= 0 {
		return fmt.Errorf("baofu withdrawal command missing withdrawal order id: %w", asynq.SkipRetry)
	}
	withdrawal, err := processor.store.GetBaofuWithdrawalOrder(ctx, withdrawalOrderID)
	if err != nil {
		return fmt.Errorf("get baofu withdrawal order: %w", err)
	}
	if strings.TrimSpace(withdrawal.OutRequestNo) == "" || strings.TrimSpace(withdrawal.OutRequestNo) != strings.TrimSpace(command.ExternalObjectKey) {
		return fmt.Errorf("baofu withdrawal command and order external key mismatch: %w", asynq.SkipRetry)
	}
	if command.CommandStatus == db.ExternalPaymentCommandStatusUnknown {
		return repairClaimedBaofuWithdrawalCommandOutcome(ctx, processor.store, command, withdrawal)
	}
	if isTerminalBaofuWithdrawalStatus(withdrawal.Status) {
		return nil
	}
	binding, err := processor.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: withdrawal.OwnerType,
		OwnerID:   withdrawal.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("get baofu withdrawal account binding: %w", err)
	}
	req, err := buildBaofuWithdrawalDispatchRequest(cfg, withdrawal, binding)
	if err != nil {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}

	claimed, err := processor.store.ClaimSubmittedExternalPaymentCommandForDispatch(ctx, db.ClaimSubmittedExternalPaymentCommandForDispatchParams{
		ID:               command.ID,
		CommandStatus:    db.ExternalPaymentCommandStatusUnknown,
		LastErrorCode:    pgtype.Text{String: baofuWithdrawalCommandDispatchStartedCode, Valid: true},
		LastErrorMessage: pgtype.Text{String: baofuWithdrawalCommandDispatchStartedMessage, Valid: true},
		ResponseSnapshot: baofuWithdrawalDispatchStartedSnapshot(command, withdrawal),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("claim baofu withdrawal command: %w", err)
	}

	upstream, err := processor.baofuWithdrawClient.CreateWithdraw(ctx, req)
	if err != nil {
		if auditErr := recordBaofuWithdrawalCommandOutcome(ctx, processor.store, claimed.ID, db.ExternalPaymentCommandStatusUnknown, nil, baofuWithdrawalCommandErrorCode(baofuWithdrawalCommandUnknownCode, err), baofuWithdrawalCommandErrorMessage("", err), err); auditErr != nil {
			return fmt.Errorf("record baofu withdrawal command unknown outcome: %w", auditErr)
		}
		log.Error().
			Err(err).
			Int64("baofu_withdrawal_order_id", withdrawal.ID).
			Int64("external_payment_command_id", claimed.ID).
			Str("out_request_no", strings.TrimSpace(withdrawal.OutRequestNo)).
			Msg("baofu withdrawal create result unknown after command dispatch")
		return nil
	}
	if upstream == nil {
		if auditErr := recordBaofuWithdrawalCommandOutcome(ctx, processor.store, claimed.ID, db.ExternalPaymentCommandStatusUnknown, nil, baofuWithdrawalCommandUnknownCode, baofuWithdrawalCommandUnknownMessage, nil); auditErr != nil {
			return fmt.Errorf("record baofu withdrawal empty outcome: %w", auditErr)
		}
		return nil
	}
	if upstream.Status == "" {
		upstream.Status = baofucontracts.WithdrawAcceptanceStatusFromUpstream(upstream.UpstreamState)
	}
	raw := baofuWithdrawalRawSnapshot(upstream.Raw)
	switch upstream.Status {
	case db.BaofuWithdrawalStatusProcessing:
		if _, err := processor.store.UpdateBaofuWithdrawalOrderToProcessing(ctx, db.UpdateBaofuWithdrawalOrderToProcessingParams{
			ID:              withdrawal.ID,
			BaofuWithdrawNo: baofuWithdrawalText(strings.TrimSpace(upstream.BaofuWithdrawNo)),
			RawSnapshot:     raw,
		}); err != nil {
			return fmt.Errorf("update baofu withdrawal accepted reference: %w", err)
		}
		if auditErr := recordBaofuWithdrawalCommandOutcome(ctx, processor.store, claimed.ID, db.ExternalPaymentCommandStatusAccepted, upstream, "", "", nil); auditErr != nil {
			return fmt.Errorf("record baofu withdrawal accepted outcome: %w", auditErr)
		}
		log.Info().
			Int64("baofu_withdrawal_order_id", withdrawal.ID).
			Int64("external_payment_command_id", claimed.ID).
			Str("out_request_no", strings.TrimSpace(withdrawal.OutRequestNo)).
			Str("baofu_withdraw_no", strings.TrimSpace(upstream.BaofuWithdrawNo)).
			Msg("baofu withdrawal create accepted")
		return nil
	case db.BaofuWithdrawalStatusFailed:
		if _, err := processor.store.ApplyBaofuWithdrawalTerminalStatusTx(ctx, db.ApplyBaofuWithdrawalTerminalStatusTxParams{
			WithdrawalOrderID: withdrawal.ID,
			Status:            db.BaofuWithdrawalStatusFailed,
			BaofuWithdrawNo:   baofuWithdrawalText(strings.TrimSpace(upstream.BaofuWithdrawNo)),
			RawSnapshot:       raw,
			ReleaseReason:     pgtype.Text{String: db.BaofuWithdrawalReservationReleaseReasonRejected, Valid: true},
		}); err != nil {
			return fmt.Errorf("mark baofu withdrawal create rejected: %w", err)
		}
		if auditErr := recordBaofuWithdrawalCommandOutcome(ctx, processor.store, claimed.ID, db.ExternalPaymentCommandStatusRejected, upstream, baofuWithdrawalCommandRejectedCode, strings.TrimSpace(upstream.Remark), nil); auditErr != nil {
			return fmt.Errorf("record baofu withdrawal rejected outcome: %w", auditErr)
		}
		log.Warn().
			Int64("baofu_withdrawal_order_id", withdrawal.ID).
			Int64("external_payment_command_id", claimed.ID).
			Str("out_request_no", strings.TrimSpace(withdrawal.OutRequestNo)).
			Str("upstream_state", strings.TrimSpace(upstream.UpstreamState)).
			Msg("baofu withdrawal create rejected by provider")
		return nil
	default:
		if auditErr := recordBaofuWithdrawalCommandOutcome(ctx, processor.store, claimed.ID, db.ExternalPaymentCommandStatusUnknown, upstream, baofuWithdrawalCommandUnknownCode, baofuWithdrawalCommandUnknownMessage, nil); auditErr != nil {
			return fmt.Errorf("record baofu withdrawal unsupported acceptance outcome: %w", auditErr)
		}
		return nil
	}
}

func (processor *RedisTaskProcessor) SetBaofuWithdrawalConfig(config BaofuWithdrawalCommandDispatchConfig) {
	processor.baofuWithdrawalConfig = config.normalized()
}

func validateBaofuWithdrawalCommand(command db.ExternalPaymentCommand) error {
	if command.Provider != db.ExternalPaymentProviderBaofu ||
		command.Channel != db.PaymentChannelBaofuAggregate ||
		command.Capability != db.ExternalPaymentCapabilityBaofuWithdraw ||
		command.CommandType != db.ExternalPaymentCommandTypeCreateBaofuWithdraw ||
		command.ExternalObjectType != db.ExternalPaymentObjectWithdraw {
		return fmt.Errorf("external payment command %d is not baofu withdrawal create", command.ID)
	}
	var snapshot struct {
		DispatchMode string `json:"dispatch_mode"`
	}
	if err := json.Unmarshal(command.ResponseSnapshot, &snapshot); err != nil {
		return fmt.Errorf("external payment command %d has invalid snapshot", command.ID)
	}
	if strings.TrimSpace(snapshot.DispatchMode) != "async_worker" {
		return fmt.Errorf("external payment command %d is not async-worker dispatch", command.ID)
	}
	return nil
}

func buildBaofuWithdrawalDispatchRequest(cfg BaofuWithdrawalCommandDispatchConfig, withdrawal db.BaofuWithdrawalOrder, binding db.BaofuAccountBinding) (baofucontracts.WithdrawRequest, error) {
	if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
		return baofucontracts.WithdrawRequest{}, fmt.Errorf("baofu withdrawal account binding is not active")
	}
	contractNo := strings.TrimSpace(binding.ContractNo.String)
	if contractNo == "" {
		return baofucontracts.WithdrawRequest{}, fmt.Errorf("baofu withdrawal account binding missing contract no")
	}
	feeMemberID := strings.TrimSpace(binding.SharingMerID.String)
	if feeMemberID == "" {
		return baofucontracts.WithdrawRequest{}, fmt.Errorf("baofu withdrawal account binding missing fee member id")
	}
	if strings.TrimSpace(withdrawal.OutRequestNo) == "" {
		return baofucontracts.WithdrawRequest{}, fmt.Errorf("baofu withdrawal out request no is required")
	}
	if withdrawal.Amount <= 0 {
		return baofucontracts.WithdrawRequest{}, fmt.Errorf("baofu withdrawal amount must be positive")
	}
	return baofucontracts.WithdrawRequest{
		MerchantID:    cfg.PayoutMerchantID,
		TerminalID:    cfg.PayoutTerminalID,
		ContractNo:    contractNo,
		TransSerialNo: strings.TrimSpace(withdrawal.OutRequestNo),
		AmountFen:     withdrawal.Amount,
		FeeMemberID:   feeMemberID,
		NotifyURL:     cfg.WithdrawNotifyURL,
	}, nil
}

func repairClaimedBaofuWithdrawalCommandOutcome(ctx context.Context, store db.Store, command db.ExternalPaymentCommand, withdrawal db.BaofuWithdrawalOrder) error {
	upstream, status, errorCode, errorMessage, ok := baofuWithdrawalCommandOutcomeFromOrder(withdrawal)
	if !ok {
		return nil
	}
	if err := recordBaofuWithdrawalCommandOutcome(ctx, store, command.ID, status, upstream, errorCode, errorMessage, nil); err != nil {
		return fmt.Errorf("repair claimed baofu withdrawal command outcome: %w", err)
	}
	log.Info().
		Int64("baofu_withdrawal_order_id", withdrawal.ID).
		Int64("external_payment_command_id", command.ID).
		Str("command_status", status).
		Str("out_request_no", strings.TrimSpace(withdrawal.OutRequestNo)).
		Msg("repaired claimed baofu withdrawal command outcome from order state")
	return nil
}

func baofuWithdrawalCommandOutcomeFromOrder(withdrawal db.BaofuWithdrawalOrder) (*baofucontracts.WithdrawResult, string, string, string, bool) {
	upstreamState, remark := baofuWithdrawalOrderRawStateAndRemark(withdrawal.RawSnapshot)
	withdrawNo := ""
	if withdrawal.BaofuWithdrawNo.Valid {
		withdrawNo = strings.TrimSpace(withdrawal.BaofuWithdrawNo.String)
	}
	if upstreamState == "2" {
		upstream := &baofucontracts.WithdrawResult{
			TransSerialNo:   strings.TrimSpace(withdrawal.OutRequestNo),
			BaofuWithdrawNo: withdrawNo,
			UpstreamState:   upstreamState,
			Status:          db.BaofuWithdrawalStatusFailed,
			AmountFen:       withdrawal.Amount,
			Remark:          remark,
			Raw:             baofuWithdrawalRawSnapshot(withdrawal.RawSnapshot),
		}
		return upstream, db.ExternalPaymentCommandStatusRejected, baofuWithdrawalCommandRejectedCode, remark, true
	}
	if withdrawNo == "" {
		return nil, "", "", "", false
	}
	upstream := &baofucontracts.WithdrawResult{
		TransSerialNo:   strings.TrimSpace(withdrawal.OutRequestNo),
		BaofuWithdrawNo: withdrawNo,
		UpstreamState:   upstreamState,
		Status:          db.BaofuWithdrawalStatusProcessing,
		AmountFen:       withdrawal.Amount,
		Remark:          remark,
		Raw:             baofuWithdrawalRawSnapshot(withdrawal.RawSnapshot),
	}
	return upstream, db.ExternalPaymentCommandStatusAccepted, "", "", true
}

func baofuWithdrawalOrderRawStateAndRemark(raw []byte) (string, string) {
	if len(raw) == 0 || !json.Valid(raw) {
		return "", ""
	}
	var payload struct {
		State       json.RawMessage `json:"state"`
		TransRemark json.RawMessage `json:"transRemark"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	return baofuWithdrawalRawScalarString(payload.State), baofuWithdrawalRawScalarString(payload.TransRemark)
}

func baofuWithdrawalRawScalarString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var integer int64
	if err := json.Unmarshal(raw, &integer); err == nil {
		return strconv.FormatInt(integer, 10)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return ""
}

func recordBaofuWithdrawalCommandOutcome(ctx context.Context, store db.Store, commandID int64, status string, upstream *baofucontracts.WithdrawResult, errorCode string, errorMessage string, cause error) error {
	input := logic.RecordExternalPaymentCommandOutcomeInput{
		CommandID:        commandID,
		CommandStatus:    status,
		ResponseSnapshot: baofuWithdrawalCommandOutcomeSnapshot(status, upstream, cause),
	}
	if errorCode != "" {
		input.LastErrorCode = &errorCode
	}
	if errorMessage != "" {
		input.LastErrorMessage = &errorMessage
	}
	_, err := logic.NewPaymentCommandService(store).RecordExternalPaymentCommandOutcome(ctx, input)
	return err
}

func baofuWithdrawalCommandOutcomeSnapshot(status string, upstream *baofucontracts.WithdrawResult, cause error) []byte {
	payload := map[string]any{
		"provider":  db.ExternalPaymentProviderBaofu,
		"operation": "create_baofu_withdraw",
		"outcome":   baofuWithdrawalCommandSnapshotOutcome(status),
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
		if v := strings.TrimSpace(providerErr.UpstreamCode); v != "" {
			payload["upstream_code"] = v
		}
		if v := baofu.SanitizeUpstreamMessageForRecord(providerErr.UpstreamMessage); v != "" {
			payload["upstream_message_sanitized"] = v
		}
	} else if cause != nil {
		payload["error_present"] = true
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"create_baofu_withdraw"}`)
	}
	return raw
}

func baofuWithdrawalDispatchStartedSnapshot(command db.ExternalPaymentCommand, order db.BaofuWithdrawalOrder) []byte {
	raw, err := json.Marshal(map[string]any{
		"provider":                    db.ExternalPaymentProviderBaofu,
		"operation":                   "create_baofu_withdraw",
		"dispatch_state":              "started",
		"baofu_withdrawal_order_id":   order.ID,
		"external_payment_command_id": command.ID,
		"out_request_no":              strings.TrimSpace(order.OutRequestNo),
	})
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"create_baofu_withdraw","dispatch_state":"started"}`)
	}
	return raw
}

func baofuWithdrawalCommandSnapshotOutcome(status string) string {
	switch status {
	case db.ExternalPaymentCommandStatusAccepted:
		return "accepted"
	case db.ExternalPaymentCommandStatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

func baofuWithdrawalRawSnapshot(raw []byte) []byte {
	if len(raw) == 0 || !json.Valid(raw) {
		return []byte(`{}`)
	}
	return raw
}

func baofuWithdrawalText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func baofuWithdrawalCommandErrorCode(defaultCode string, cause error) string {
	var providerErr *baofu.ProviderError
	if errors.As(cause, &providerErr) && providerErr != nil {
		if code := strings.TrimSpace(providerErr.UpstreamCode); code != "" {
			return code
		}
	}
	return strings.TrimSpace(defaultCode)
}

func baofuWithdrawalCommandErrorMessage(defaultMessage string, cause error) string {
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

func (c BaofuWithdrawalCommandDispatchConfig) normalized() BaofuWithdrawalCommandDispatchConfig {
	c.PayoutMerchantID = strings.TrimSpace(c.PayoutMerchantID)
	c.PayoutTerminalID = strings.TrimSpace(c.PayoutTerminalID)
	c.WithdrawNotifyURL = strings.TrimSpace(c.WithdrawNotifyURL)
	return c
}
