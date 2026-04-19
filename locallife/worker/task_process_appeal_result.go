package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessAppealResult = "appeal:process_result"
)

// ProcessAppealResultPayload 处理申诉审核结果的任务载荷
type ProcessAppealResultPayload struct {
	AppealID             int64  `json:"appeal_id"`
	ClaimID              int64  `json:"claim_id"`
	CompensationActionID int64  `json:"compensation_action_id"`
	Status               string `json:"status"`              // approved / rejected
	AppellantType        string `json:"appellant_type"`      // merchant / rider
	AppellantID          int64  `json:"appellant_id"`        // 商户ID或骑手ID
	ClaimantUserID       int64  `json:"claimant_user_id"`    // 索赔用户ID
	ClaimType            string `json:"claim_type"`          // 索赔类型
	ClaimAmount          int64  `json:"claim_amount"`        // 索赔金额
	CompensationAmount   int64  `json:"compensation_amount"` // 补偿金额（申诉成功时）
	OrderNo              string `json:"order_no"`            // 订单号
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
	info, err := distributor.enqueueTask(ctx, task)
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

	return processor.processAppealResult(ctx, payload)
}

func (processor *RedisTaskProcessor) processAppealResult(ctx context.Context, payload ProcessAppealResultPayload) error {

	log.Info().
		Int64("appeal_id", payload.AppealID).
		Str("status", payload.Status).
		Str("appellant_type", payload.AppellantType).
		Msg("processing appeal result")

	// 申诉成功的后续处理
	switch payload.Status {
	case "approved":
		if err := processor.rollbackClaimRecovery(ctx, payload); err != nil {
			log.Error().Err(err).Msg("failed to rollback claim recovery")
		}
		// 1. 降低索赔用户（恶意投诉）的信用分
		if err := processor.penalizeClaimant(ctx, payload); err != nil {
			log.Error().Err(err).Msg("failed to penalize claimant trust score")
			// 不中断，继续处理通知
		}
		if err := processor.executeAppealCompensation(ctx, payload); err != nil {
			log.Error().Err(err).Int64("appeal_id", payload.AppealID).Int64("behavior_action_id", payload.CompensationActionID).Msg("failed to execute appeal compensation action")
		}
	case "rejected":
		if err := processor.resumeClaimRecovery(ctx, payload); err != nil {
			log.Error().Err(err).Msg("failed to resume claim recovery")
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
	userID := payload.ClaimantUserID
	if userID == 0 && payload.ClaimID != 0 {
		if claim, err := processor.store.GetClaim(ctx, payload.ClaimID); err == nil {
			userID = claim.UserID
		}
	}
	if userID == 0 {
		return nil
	}

	if _, err := processor.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	}); err == nil {
		return nil
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return err
	}

	blockDays := int64(14)
	if days := processor.getRejectServiceCooldownDays(ctx); days > 0 {
		blockDays = days
	}
	blockUntil := time.Now().AddDate(0, 0, int(blockDays))

	if _, err := processor.store.CreateBehaviorBlocklist(ctx, db.CreateBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
		ReasonCode: "malicious-claim",
		BlockUntil: pgtype.Timestamptz{Time: blockUntil, Valid: true},
		Status:     "active",
	}); err != nil {
		return err
	}

	if _, err := processor.store.GetUserClaimWarningStatus(ctx, userID); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			_, _ = processor.store.CreateUserClaimWarning(ctx, db.CreateUserClaimWarningParams{
				UserID:            userID,
				LastWarningReason: pgtype.Text{String: "appeal approved: malicious claim", Valid: true},
				RequiresEvidence:  false,
			})
			return nil
		}
		return err
	}

	_ = processor.store.IncrementUserClaimWarning(ctx, db.IncrementUserClaimWarningParams{
		UserID:            userID,
		LastWarningReason: pgtype.Text{String: "appeal approved: malicious claim", Valid: true},
		RequiresEvidence:  false,
	})
	return nil
}

func (processor *RedisTaskProcessor) getRejectServiceCooldownDays(ctx context.Context) int64 {
	config, err := processor.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "behavior_trace.reject_service_cooldown_days",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		return 0
	}

	var payload struct {
		Days int64 `json:"days"`
	}
	if jsonErr := json.Unmarshal(config.ConfigValue, &payload); jsonErr == nil && payload.Days > 0 {
		return payload.Days
	}
	return 0
}

func (processor *RedisTaskProcessor) executeAppealCompensation(ctx context.Context, payload ProcessAppealResultPayload) error {
	if payload.Status != "approved" || payload.CompensationActionID == 0 {
		return nil
	}
	return ExecuteClaimPayoutAction(ctx, processor.store, processor.transferClient, payload.CompensationActionID)
}

func (processor *RedisTaskProcessor) rollbackClaimRecovery(ctx context.Context, payload ProcessAppealResultPayload) error {
	if payload.ClaimID == 0 {
		return nil
	}

	recovery, err := processor.store.GetClaimRecoveryByClaimID(ctx, payload.ClaimID)
	if err != nil {
		return nil
	}
	if recovery.Status != "appealed" {
		if recovery.Status == "waived" {
			return nil
		}
		log.Warn().
			Int64("appeal_id", payload.AppealID).
			Int64("claim_id", payload.ClaimID).
			Int64("recovery_id", recovery.ID).
			Str("recovery_status", recovery.Status).
			Msg("skip claim recovery rollback because recovery is not in appealed status")
		return nil
	}

	if _, err := processor.store.MarkClaimRecoveryWaived(ctx, recovery.ID); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "merchant" {
		order, orderErr := processor.store.GetOrder(ctx, recovery.OrderID)
		if orderErr != nil {
			return orderErr
		}
		if err := processor.store.UnsuspendMerchantTakeout(ctx, order.MerchantID); err != nil {
			return err
		}
	}

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "rider" {
		delivery, deliveryErr := processor.store.GetDeliveryByOrderID(ctx, recovery.OrderID)
		if deliveryErr != nil {
			return deliveryErr
		}
		if delivery.RiderID.Valid {
			if err := processor.store.UnsuspendRider(ctx, delivery.RiderID.Int64); err != nil {
				return err
			}
		}
	}

	return nil
}

func (processor *RedisTaskProcessor) resumeClaimRecovery(ctx context.Context, payload ProcessAppealResultPayload) error {
	if payload.ClaimID == 0 {
		return nil
	}

	recovery, err := processor.store.GetClaimRecoveryByClaimID(ctx, payload.ClaimID)
	if err != nil {
		return nil
	}
	if recovery.Status != "appealed" {
		if recovery.Status == "pending" || recovery.Status == "overdue" {
			return nil
		}
		log.Warn().
			Int64("appeal_id", payload.AppealID).
			Int64("claim_id", payload.ClaimID).
			Int64("recovery_id", recovery.ID).
			Str("recovery_status", recovery.Status).
			Msg("skip claim recovery resume because recovery is not in appealed status")
		return nil
	}

	_, err = processor.store.ResumeClaimRecoveryAfterAppeal(ctx, recovery.ID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return err
	}
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
		content = fmt.Sprintf("您针对订单%s的申诉已通过审核，平台已撤销本次判责与追偿。", payload.OrderNo)
		if payload.CompensationAmount > 0 {
			content += fmt.Sprintf(" 已核定补偿金额%.2f元。", float64(payload.CompensationAmount)/100)
		}
	} else {
		title = "申诉结果通知"
		content = fmt.Sprintf("您针对订单%s的申诉未通过审核，原判责与追偿继续有效。", payload.OrderNo)
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
		content = fmt.Sprintf("您在订单%s中的%s索赔经审核认定为不当，平台已撤销对责任方的追责与追偿安排，已发放赔付不再向您追回。请合理使用售后服务。",
			payload.OrderNo, getClaimTypeLabel(payload.ClaimType))
	} else {
		title = "索赔申诉结果通知"
		content = fmt.Sprintf("商家/骑手针对您订单%s的申诉未通过审核，原索赔与平台判责继续有效。", payload.OrderNo)
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
