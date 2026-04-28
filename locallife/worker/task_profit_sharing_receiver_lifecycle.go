package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const TaskProcessProfitSharingReceiverTarget = "profit_sharing_receiver:process_target"

const profitSharingReceiverFailureAlertMessageLimit = 200

type ProfitSharingReceiverTargetPayload struct {
	TargetID int64 `json:"target_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessProfitSharingReceiverTarget(
	ctx context.Context,
	payload *ProfitSharingReceiverTargetPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessProfitSharingReceiverTarget, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("target_id", payload.TargetID).
		Msg("enqueued profit sharing receiver target task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskProfitSharingReceiverTarget(ctx context.Context, task *asynq.Task) error {
	var payload ProfitSharingReceiverTargetPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.TargetID <= 0 {
		return fmt.Errorf("invalid profit sharing receiver target payload: %w", asynq.SkipRetry)
	}

	service := logic.NewProfitSharingReceiverLifecycleService(processor.store, processor.ecommerceClient)
	result, err := service.ProcessReceiverTarget(ctx, payload.TargetID, time.Now())
	if err != nil {
		return err
	}

	event := log.Info()
	if result.Status == "failed" || result.Status == "skipped" {
		event = log.Warn()
	}
	event = event.
		Int64("target_id", payload.TargetID).
		Str("owner_type", result.OwnerType).
		Int64("owner_id", result.OwnerID).
		Int32("attempt_count", result.AttemptCount).
		Str("status", result.Status).
		Str("action", result.Action)
	if result.ErrorCode != "" {
		event = event.Str("error_code", result.ErrorCode)
	}
	event.Msg("processed profit sharing receiver target")
	processor.publishProfitSharingReceiverFailureAlert(ctx, result)

	return nil
}

func (processor *RedisTaskProcessor) publishProfitSharingReceiverFailureAlert(ctx context.Context, result logic.ProfitSharingReceiverTargetProcessResult) {
	if processor == nil || processor.store == nil {
		return
	}
	if result.Status != db.ProfitSharingReceiverSyncStatusFailed || result.AttemptCount < 3 {
		return
	}

	target, err := processor.store.GetProfitSharingReceiverTarget(ctx, result.TargetID)
	if err != nil {
		log.Warn().Err(err).Int64("target_id", result.TargetID).Msg("load failed profit sharing receiver target for alert failed")
		return
	}

	processor.publishAlert(ctx, AlertData{
		AlertType:   AlertTypeProfitSharingReceiverFailed,
		Level:       AlertLevelCritical,
		Title:       "分账接收方同步连续失败",
		Message:     fmt.Sprintf("分账接收方目标 %d 已连续失败 %d 次，当前仍未达到期望状态 %s，请人工排查。", target.ID, target.AttemptCount, target.DesiredState),
		RelatedID:   target.ID,
		RelatedType: "profit_sharing_receiver_target",
		Extra:       profitSharingReceiverFailureAlertExtra(target),
	})
}

func profitSharingReceiverFailureAlertExtra(target db.ProfitSharingReceiverTarget) map[string]any {
	extra := map[string]any{
		"owner_type":    target.OwnerType,
		"owner_id":      target.OwnerID,
		"desired_state": target.DesiredState,
		"sync_status":   target.SyncStatus,
		"attempt_count": target.AttemptCount,
	}
	if target.LastErrorCode.Valid && strings.TrimSpace(target.LastErrorCode.String) != "" {
		extra["last_error_code"] = strings.TrimSpace(target.LastErrorCode.String)
	}
	if message := sanitizeProfitSharingReceiverAlertMessage(target.LastErrorMessage.String); message != "" {
		extra["last_error_message"] = message
	}
	if target.LastAttemptAt.Valid {
		extra["last_attempt_at"] = target.LastAttemptAt.Time
	}
	if target.NextRetryAt.Valid {
		extra["next_retry_at"] = target.NextRetryAt.Time
	}
	if !target.UpdatedAt.IsZero() {
		extra["updated_at"] = target.UpdatedAt
	}
	return extra
}

func sanitizeProfitSharingReceiverAlertMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= profitSharingReceiverFailureAlertMessageLimit {
		return message
	}
	return strings.TrimSpace(message[:profitSharingReceiverFailureAlertMessageLimit])
}
