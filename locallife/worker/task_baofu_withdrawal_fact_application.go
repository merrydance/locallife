package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const TaskProcessBaofuWithdrawalFactApplication = "baofu:process_withdrawal_fact_application"

type BaofuWithdrawalFactApplicationPayload struct {
	WithdrawalOrderID int64  `json:"withdrawal_order_id"`
	UpstreamState     string `json:"upstream_state"`
	BaofuWithdrawNo   string `json:"baofu_withdraw_no,omitempty"`
	RawSnapshot       []byte `json:"raw_snapshot,omitempty"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessBaofuWithdrawalFactApplication(ctx context.Context, payload *BaofuWithdrawalFactApplicationPayload, opts ...asynq.Option) error {
	if payload == nil || payload.WithdrawalOrderID <= 0 {
		return fmt.Errorf("baofu withdrawal order id is required")
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal baofu withdrawal fact application payload: %w", err)
	}
	task := asynq.NewTask(TaskProcessBaofuWithdrawalFactApplication, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue baofu withdrawal fact application task: %w", err)
	}
	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("baofu_withdrawal_order_id", payload.WithdrawalOrderID).
		Msg("enqueued baofu withdrawal fact application task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskBaofuWithdrawalFactApplication(ctx context.Context, task *asynq.Task) error {
	var payload BaofuWithdrawalFactApplicationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal baofu withdrawal fact application payload: %w", err)
	}
	if payload.WithdrawalOrderID <= 0 {
		return fmt.Errorf("baofu withdrawal order id is required")
	}
	withdrawalOrder, err := processor.store.GetBaofuWithdrawalOrder(ctx, payload.WithdrawalOrderID)
	if err != nil {
		return fmt.Errorf("get baofu withdrawal order: %w", err)
	}
	status := baofucontracts.WithdrawStatusFromUpstream(payload.UpstreamState)
	raw := payload.RawSnapshot
	if len(raw) == 0 || !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	baofuWithdrawNo := strings.TrimSpace(payload.BaofuWithdrawNo)
	if baofuWithdrawNo == "" && withdrawalOrder.BaofuWithdrawNo.Valid {
		baofuWithdrawNo = withdrawalOrder.BaofuWithdrawNo.String
	}
	baofuWithdrawNoText := pgtype.Text{
		String: baofuWithdrawNo,
		Valid:  baofuWithdrawNo != "",
	}
	if status == db.BaofuWithdrawalStatusProcessing {
		if _, err := processor.store.UpdateBaofuWithdrawalOrderToProcessing(ctx, db.UpdateBaofuWithdrawalOrderToProcessingParams{
			ID:              withdrawalOrder.ID,
			BaofuWithdrawNo: baofuWithdrawNoText,
			RawSnapshot:     raw,
		}); err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				current, getErr := processor.store.GetBaofuWithdrawalOrder(ctx, withdrawalOrder.ID)
				if getErr == nil && isTerminalBaofuWithdrawalStatus(current.Status) {
					return nil
				}
				if getErr != nil {
					return fmt.Errorf("get baofu withdrawal order after processing update conflict: %w", getErr)
				}
			}
			return fmt.Errorf("update baofu withdrawal processing reference: %w", err)
		}
		return nil
	}
	if !isTerminalBaofuWithdrawalStatus(status) {
		return fmt.Errorf("unsupported baofu withdrawal fact status %q", status)
	}
	if _, err := processor.store.ApplyBaofuWithdrawalTerminalStatusTx(ctx, db.ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: withdrawalOrder.ID,
		Status:            status,
		BaofuWithdrawNo:   baofuWithdrawNoText,
		RawSnapshot:       raw,
	}); err != nil {
		return fmt.Errorf("apply baofu withdrawal terminal status: %w", err)
	}
	return nil
}

func isTerminalBaofuWithdrawalStatus(status string) bool {
	switch status {
	case db.BaofuWithdrawalStatusSucceeded, db.BaofuWithdrawalStatusFailed, db.BaofuWithdrawalStatusReturned:
		return true
	default:
		return false
	}
}
