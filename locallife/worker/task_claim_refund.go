package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// TaskClaimRefund 索赔退款任务
	TaskClaimRefund = "task:claim_refund"
)

// ClaimRefundPayload 索赔退款任务载荷
type ClaimRefundPayload struct {
	ClaimID    int64  `json:"claim_id"`
	UserID     int64  `json:"user_id"`
	Amount     int64  `json:"amount"`
	SourceType string `json:"source_type"`
	SourceID   int64  `json:"source_id"`
	Remark     string `json:"remark"`
}

// NewClaimRefundTask 创建索赔退款任务
func NewClaimRefundTask(payload *ClaimRefundPayload) (*asynq.Task, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskClaimRefund, jsonPayload), nil
}

// ProcessTaskClaimRefund 处理索赔退款任务
func (processor *RedisTaskProcessor) ProcessTaskClaimRefund(ctx context.Context, task *asynq.Task) error {
	var payload ClaimRefundPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal claim refund payload: %w", asynq.SkipRetry)
	}

	_, err := processor.store.ClaimRefundTx(ctx, db.ClaimRefundTxParams{
		ClaimID:    payload.ClaimID,
		UserID:     payload.UserID,
		Amount:     payload.Amount,
		SourceType: payload.SourceType,
		SourceID:   payload.SourceID,
		Remark:     payload.Remark,
	})
	if err != nil {
		return fmt.Errorf("failed to execute claim refund tx: %w", err)
	}

	return nil
}

// DistributeTaskClaimRefund 分发索赔退款任务
func (distributor *RedisTaskDistributor) DistributeTaskClaimRefund(
	ctx context.Context,
	payload *ClaimRefundPayload,
	opts ...asynq.Option,
) error {
	task, err := NewClaimRefundTask(payload)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
