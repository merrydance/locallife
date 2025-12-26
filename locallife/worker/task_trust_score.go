package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/algorithm"
)

const (
	// TypeHandleSuspiciousPattern 处理可疑索赔模式任务
	TypeHandleSuspiciousPattern = "trust:handle_suspicious_pattern"

	// TypeCheckMerchantForeignObject 检查商户异物索赔历史任务
	TypeCheckMerchantForeignObject = "trust:check_merchant_foreign_object"

	// TypeCheckRiderDamage 检查骑手餐损历史任务
	TypeCheckRiderDamage = "trust:check_rider_damage"
)

// HandleSuspiciousPatternPayload 可疑模式处理任务载荷
type HandleSuspiciousPatternPayload struct {
	UserID       int64                     `json:"user_id"`
	ClaimID      int64                     `json:"claim_id"`
	ClaimType    string                    `json:"claim_type"`
	LookbackData *algorithm.LookbackResult `json:"lookback_data"`
}

// NewHandleSuspiciousPatternTask 创建可疑模式处理任务
func NewHandleSuspiciousPatternTask(userID, claimID int64, claimType string, lookback *algorithm.LookbackResult) (*asynq.Task, error) {
	payload, err := json.Marshal(HandleSuspiciousPatternPayload{
		UserID:       userID,
		ClaimID:      claimID,
		ClaimType:    claimType,
		LookbackData: lookback,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeHandleSuspiciousPattern, payload), nil
}

// HandleSuspiciousPatternProcessor 处理可疑模式任务
func (processor *RedisTaskProcessor) HandleSuspiciousPattern(ctx context.Context, task *asynq.Task) error {
	var payload HandleSuspiciousPatternPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w", asynq.SkipRetry)
	}

	// Worker后台任务不需要WebSocket实时通知，传nil
	calculator := algorithm.NewTrustScoreCalculator(processor.store, nil)

	// 根据索赔频率和模式扣分
	var scoreChange int16
	var reason string

	lookback := payload.LookbackData
	if lookback != nil && lookback.ClaimsFound >= 5 {
		// 高频索赔（5次以上）
		scoreChange = algorithm.ScoreThirdMaliciousClaim // -50
		reason = "高频索赔处罚"
	} else if lookback != nil && lookback.ClaimsFound >= 3 {
		// 频繁索赔（3次以上）
		scoreChange = algorithm.ScoreFirstMaliciousClaim // -30
		reason = "频繁索赔警告"
	} else {
		// 可疑但次数不多，轻微扣分
		scoreChange = -15
		reason = "可疑模式提醒"
	}

	relatedType := "claim"
	err := calculator.UpdateTrustScore(
		ctx,
		algorithm.EntityTypeCustomer,
		payload.UserID,
		scoreChange,
		reason,
		fmt.Sprintf("%s索赔模式异常（索赔ID: %d）", payload.ClaimType, payload.ClaimID),
		&relatedType,
		&payload.ClaimID,
	)

	if err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	return nil
}

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

// HandleCheckMerchantForeignObject 处理商户异物索赔检查任务
// 业务逻辑：异物索赔只通知整改，不扣分不停业（食安问题走食安熔断流程）
func (processor *RedisTaskProcessor) HandleCheckMerchantForeignObject(ctx context.Context, task *asynq.Task) error {
	var payload CheckMerchantForeignObjectPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w", asynq.SkipRetry)
	}

	// 使用 MerchantForeignObjectTracker 检查状态
	tracker := algorithm.NewMerchantForeignObjectTracker(processor.store)
	result, err := tracker.CheckMerchantForeignObjectStatus(ctx, payload.MerchantID)
	if err != nil {
		return fmt.Errorf("failed to check merchant foreign object status: %w", err)
	}

	// 如果需要通知整改，发送WebSocket通知
	if result.ShouldNotify {
		// 注意：Worker中wsHub为nil，通过其他方式发送通知
		// 实际生产中可以通过消息队列或数据库标记触发通知
		// 这里记录日志，实际通知由定时任务扫描处理
		// TODO: 可以改为调用通知服务发送站内信/短信
	}

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

// HandleCheckRiderDamage 处理骑手餐损检查任务
func (processor *RedisTaskProcessor) HandleCheckRiderDamage(ctx context.Context, task *asynq.Task) error {
	var payload CheckRiderDamagePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w", asynq.SkipRetry)
	}

	// Worker后台任务不需要WebSocket实时通知，传nil
	approver := algorithm.NewClaimAutoApproval(processor.store, nil)
	err := approver.CheckRiderDamageHistory(ctx, payload.RiderID)
	if err != nil {
		return fmt.Errorf("failed to check rider damage history: %w", err)
	}

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

	_, err = distributor.client.EnqueueContext(ctx, task, opts...)
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

	_, err = distributor.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
