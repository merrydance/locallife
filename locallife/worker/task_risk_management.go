package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	// TypeCheckMerchantForeignObject 检查商户异物索赔历史任务
	TypeCheckMerchantForeignObject = "risk:check_merchant_foreign_object"

	// TypeCheckRiderDamage 检查骑手餐损历史任务
	TypeCheckRiderDamage = "risk:check_rider_damage"
)

// CheckMerchantForeignObjectPayload 商户异物索赔检查任务载荷
type CheckMerchantForeignObjectPayload struct {
	MerchantID int64 `json:"merchant_id"`
}

// NewCheckMerchantForeignObjectTask 创建商户异物索赔检查任务
func NewCheckMerchantForeignObjectTask(merchantID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(CheckMerchantForeignObjectPayload{
		MerchantID: merchantID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCheckMerchantForeignObject, payload), nil
}

// HandleCheckMerchantForeignObject 保留 legacy task 消费能力；当前索赔判责由行为追溯主链处理。
func (processor *RedisTaskProcessor) HandleCheckMerchantForeignObject(ctx context.Context, task *asynq.Task) error {
	var payload CheckMerchantForeignObjectPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w", asynq.SkipRetry)
	}

	log.Info().Int64("merchant_id", payload.MerchantID).Msg("legacy merchant foreign object risk task ignored; claim behavior trace handles adjudication")

	return nil
}

// CheckRiderDamagePayload 骑手餐损检查任务载荷
type CheckRiderDamagePayload struct {
	RiderID int64 `json:"rider_id"`
}

// NewCheckRiderDamageTask 创建骑手餐损检查任务
func NewCheckRiderDamageTask(riderID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(CheckRiderDamagePayload{
		RiderID: riderID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCheckRiderDamage, payload), nil
}

// HandleCheckRiderDamage 保留 legacy task 消费能力；当前索赔判责由行为追溯主链处理。
func (processor *RedisTaskProcessor) HandleCheckRiderDamage(ctx context.Context, task *asynq.Task) error {
	var payload CheckRiderDamagePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w", asynq.SkipRetry)
	}

	log.Info().Int64("rider_id", payload.RiderID).Msg("legacy rider damage risk task ignored; claim behavior trace handles adjudication")

	return nil
}

// DistributeTaskCheckMerchantForeignObject 分发商户异物索赔检查任务
func (distributor *RedisTaskDistributor) DistributeTaskCheckMerchantForeignObject(
	ctx context.Context,
	merchantID int64,
	opts ...asynq.Option,
) error {
	task, err := NewCheckMerchantForeignObjectTask(merchantID)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}

// DistributeTaskCheckRiderDamage 分发骑手餐损检查任务
func (distributor *RedisTaskDistributor) DistributeTaskCheckRiderDamage(
	ctx context.Context,
	riderID int64,
	opts ...asynq.Option,
) error {
	task, err := NewCheckRiderDamageTask(riderID)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
