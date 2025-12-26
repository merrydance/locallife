package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/algorithm"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessAppealResult = "appeal:process_result"

	// 申诉成功惩罚（简化设计）
	// 设计理念：申诉成功 = 人工审核确认恶意 = 比系统自动检测更可靠 = 相当于发现两次
	// 与 algorithm.ScoreEvidenceRequired (-10) 对齐，不区分金额和类型
	// 恶意就是恶意，不因金额大小而区别对待
	PenaltyAppealUpheld = int16(10)
)

// ProcessAppealResultPayload 处理申诉审核结果的任务载荷
type ProcessAppealResultPayload struct {
	AppealID           int64  `json:"appeal_id"`
	Status             string `json:"status"`              // approved / rejected
	AppellantType      string `json:"appellant_type"`      // merchant / rider
	AppellantID        int64  `json:"appellant_id"`        // 商户ID或骑手ID
	ClaimantUserID     int64  `json:"claimant_user_id"`    // 索赔用户ID
	ClaimType          string `json:"claim_type"`          // 索赔类型
	ClaimAmount        int64  `json:"claim_amount"`        // 索赔金额
	CompensationAmount int64  `json:"compensation_amount"` // 补偿金额（申诉成功时）
	OrderNo            string `json:"order_no"`            // 订单号
}

// DistributeTaskProcessAppealResult 分发申诉审核结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessAppealResult(
	ctx context.Context,
	payload *ProcessAppealResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessAppealResult, jsonPayload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("appeal_id", payload.AppealID).
		Str("status", payload.Status).
		Msg("enqueued appeal result task")

	return nil
}

// ProcessTaskProcessAppealResult 处理申诉审核结果
func (processor *RedisTaskProcessor) ProcessTaskProcessAppealResult(ctx context.Context, task *asynq.Task) error {
	var payload ProcessAppealResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Info().
		Int64("appeal_id", payload.AppealID).
		Str("status", payload.Status).
		Str("appellant_type", payload.AppellantType).
		Msg("processing appeal result")

	// 申诉成功的后续处理
	if payload.Status == "approved" {
		// 1. 降低索赔用户（恶意投诉）的信用分
		if err := processor.penalizeClaimant(ctx, payload); err != nil {
			log.Error().Err(err).Msg("failed to penalize claimant trust score")
			// 不中断，继续处理通知
		}
	}

	// 2. 发送通知给申诉人（商户或骑手对应的用户）
	if err := processor.sendAppellantNotification(ctx, payload); err != nil {
		log.Error().Err(err).Msg("failed to send appellant notification")
	}

	// 3. 发送通知给索赔用户
	if err := processor.sendClaimantNotification(ctx, payload); err != nil {
		log.Error().Err(err).Msg("failed to send claimant notification")
	}

	log.Info().
		Int64("appeal_id", payload.AppealID).
		Str("status", payload.Status).
		Msg("✅ appeal result processed successfully")

	return nil
}

// penalizeClaimant 惩罚恶意索赔用户的信用分
// 设计：申诉成功 = 确认恶意 = 固定扣10分（相当于系统发现两次）
func (processor *RedisTaskProcessor) penalizeClaimant(ctx context.Context, payload ProcessAppealResultPayload) error {
	relatedType := "appeal"
	relatedID := payload.AppealID

	// 创建信用分计算器并扣分
	tsc := algorithm.NewTrustScoreCalculator(processor.store, nil) // WebSocket推送由通知任务处理
	err := tsc.UpdateTrustScore(
		ctx,
		algorithm.EntityTypeCustomer,
		payload.ClaimantUserID,
		-PenaltyAppealUpheld, // 固定扣10分
		"appeal_upheld",
		fmt.Sprintf("申诉成功: 订单%s的%s索赔被确认为不当", payload.OrderNo, getClaimTypeLabel(payload.ClaimType)),
		&relatedType,
		&relatedID,
	)
	if err != nil {
		return fmt.Errorf("update trust score: %w", err)
	}

	log.Info().
		Int64("user_id", payload.ClaimantUserID).
		Int16("score_deduction", PenaltyAppealUpheld).
		Msg("penalized claimant trust score")

	return nil
}

// sendAppellantNotification 发送通知给申诉人
func (processor *RedisTaskProcessor) sendAppellantNotification(ctx context.Context, payload ProcessAppealResultPayload) error {
	// 获取申诉人对应的用户ID
	var userID int64
	var err error

	if payload.AppellantType == "merchant" {
		merchant, err := processor.store.GetMerchant(ctx, payload.AppellantID)
		if err != nil {
			return fmt.Errorf("get merchant: %w", err)
		}
		userID = merchant.OwnerUserID
	} else {
		rider, err := processor.store.GetRider(ctx, payload.AppellantID)
		if err != nil {
			return fmt.Errorf("get rider: %w", err)
		}
		userID = rider.UserID
	}

	var title, content string
	if payload.Status == "approved" {
		title = "申诉成功通知"
		content = fmt.Sprintf("您针对订单%s的申诉已通过审核，补偿金额%.2f元将在1-3个工作日内到账。",
			payload.OrderNo, float64(payload.CompensationAmount)/100)
	} else {
		title = "申诉结果通知"
		content = fmt.Sprintf("您针对订单%s的申诉未通过审核，如有疑问请联系客服。", payload.OrderNo)
	}

	// 分发通知任务
	notificationPayload := &SendNotificationPayload{
		UserID:      userID,
		Type:        "appeal",
		Title:       title,
		Content:     content,
		RelatedType: "appeal",
		RelatedID:   payload.AppealID,
		ExtraData: map[string]any{
			"appeal_id":      payload.AppealID,
			"status":         payload.Status,
			"appellant_type": payload.AppellantType,
		},
	}

	err = processor.distributor.DistributeTaskSendNotification(ctx, notificationPayload)
	if err != nil {
		return fmt.Errorf("distribute notification: %w", err)
	}

	return nil
}

// sendClaimantNotification 发送通知给索赔用户
func (processor *RedisTaskProcessor) sendClaimantNotification(ctx context.Context, payload ProcessAppealResultPayload) error {
	var title, content string

	if payload.Status == "approved" {
		title = "索赔申诉结果通知"
		content = fmt.Sprintf("您在订单%s中的%s索赔经审核认定为不当，相关赔付将被撤回。请合理使用售后服务。",
			payload.OrderNo, getClaimTypeLabel(payload.ClaimType))
	} else {
		title = "索赔申诉结果通知"
		content = fmt.Sprintf("商家/骑手针对您订单%s的申诉未通过审核，原索赔有效。", payload.OrderNo)
	}

	// 分发通知任务
	notificationPayload := &SendNotificationPayload{
		UserID:      payload.ClaimantUserID,
		Type:        "appeal",
		Title:       title,
		Content:     content,
		RelatedType: "appeal",
		RelatedID:   payload.AppealID,
		ExtraData: map[string]any{
			"appeal_id": payload.AppealID,
			"status":    payload.Status,
		},
	}

	err := processor.distributor.DistributeTaskSendNotification(ctx, notificationPayload)
	if err != nil {
		return fmt.Errorf("distribute notification: %w", err)
	}

	return nil
}

// getClaimTypeLabel 获取索赔类型的中文标签
func getClaimTypeLabel(claimType string) string {
	switch claimType {
	case "foreign-object":
		return "异物"
	case "damage":
		return "餐损"
	case "delay":
		return "延迟"
	case "quality":
		return "质量问题"
	case "missing-item":
		return "缺漏"
	default:
		return "其他"
	}
}
