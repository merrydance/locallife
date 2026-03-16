package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	// TypeHandleSuspiciousPattern 处理可疑索赔模式任务
	TypeHandleSuspiciousPattern = "risk:handle_suspicious_pattern"

	// TypeCheckMerchantForeignObject 检查商户异物索赔历史任务
	TypeCheckMerchantForeignObject = "risk:check_merchant_foreign_object"

	// TypeCheckRiderDamage 检查骑手餐损历史任务
	TypeCheckRiderDamage = "risk:check_rider_damage"
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

	_ = payload
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

	// 如果需要通知整改，暂停外卖并发送站内信给商户负责人
	if result.ShouldNotify {
		// 异物高发：触发外卖熔断（不影响堂食），暂停 24 小时
		reason := fmt.Sprintf("foreign object claims high: %d in %d days", result.ForeignObjectNum, result.WindowDays)
		if err := processor.store.SuspendMerchantTakeout(ctx, db.SuspendMerchantTakeoutParams{
			MerchantID:           payload.MerchantID,
			TakeoutSuspendReason: pgtype.Text{String: reason, Valid: true},
			TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		}); err != nil {
			log.Error().Err(err).Int64("merchant_id", payload.MerchantID).Msg("failed to suspend merchant takeout")
			return fmt.Errorf("suspend merchant takeout: %w", err)
		}

		// 获取商户负责人，向其发送站内信通知
		merchant, err := processor.store.GetMerchant(ctx, payload.MerchantID)
		if err != nil {
			// 通知失败不影响熔断结果，记录日志后继续
			log.Error().Err(err).Int64("merchant_id", payload.MerchantID).Msg("failed to get merchant for foreign object notification")
		} else {
			title := "您的外卖服务已被暂停整改"
			content := fmt.Sprintf(
				"您的店铺近 %d 天内收到 %d 次异物投诉，外卖服务已暂停 24 小时，请尽快自查整改。",
				result.WindowDays, result.ForeignObjectNum,
			)
			if err := processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
				UserID:            merchant.OwnerUserID,
				Type:              "system",
				Title:             title,
				Content:           content,
				RelatedType:       "merchant",
				RelatedID:         payload.MerchantID,
				IgnorePreferences: true, // 食安/风控类关键通知，不受免打扰设置限制
			}); err != nil {
				log.Error().Err(err).Int64("merchant_id", payload.MerchantID).Msg("failed to enqueue foreign object notification")
			}
		}
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
