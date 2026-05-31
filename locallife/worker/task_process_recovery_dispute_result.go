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
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessRecoveryDisputeResult = "recovery_dispute:process_result"
)

// ProcessRecoveryDisputeResultPayload 处理追偿争议审核结果的任务载荷
type ProcessRecoveryDisputeResultPayload struct {
	RecoveryDisputeID    int64  `json:"recovery_dispute_id"`
	ClaimID              int64  `json:"claim_id"`
	RecoveryTarget       string `json:"recovery_target"`
	CompensationActionID int64  `json:"compensation_action_id"`
	ReleaseActionID      int64  `json:"release_action_id"`
	Status               string `json:"status"`              // approved / rejected
	AppellantType        string `json:"appellant_type"`      // merchant / rider
	AppellantID          int64  `json:"appellant_id"`        // 商户ID或骑手ID
	ClaimantUserID       int64  `json:"claimant_user_id"`    // 索赔用户ID
	ClaimType            string `json:"claim_type"`          // 索赔类型
	ClaimAmount          int64  `json:"claim_amount"`        // 索赔金额
	CompensationAmount   int64  `json:"compensation_amount"` // 补偿金额（追偿争议通过时）
	OrderNo              string `json:"order_no"`            // 订单号
}

// DistributeTaskProcessRecoveryDisputeResult 分发追偿争议审核结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessRecoveryDisputeResult(
	ctx context.Context,
	payload *ProcessRecoveryDisputeResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessRecoveryDisputeResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("recovery_dispute_id", payload.RecoveryDisputeID).
		Str("status", payload.Status).
		Msg("enqueued recovery dispute result task")

	return nil
}

// ProcessTaskRecoveryDisputeResult 处理追偿争议审核结果
func (processor *RedisTaskProcessor) ProcessTaskRecoveryDisputeResult(ctx context.Context, task *asynq.Task) error {
	var payload ProcessRecoveryDisputeResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	return processor.processRecoveryDisputeResult(ctx, payload)
}

func (processor *RedisTaskProcessor) processRecoveryDisputeResult(ctx context.Context, payload ProcessRecoveryDisputeResultPayload) error {
	log.Info().
		Int64("recovery_dispute_id", payload.RecoveryDisputeID).
		Str("status", payload.Status).
		Str("appellant_type", payload.AppellantType).
		Msg("processing recovery dispute result")

	if err := ExecuteRecoveryDisputeResultEffects(ctx, processor.store, processor.distributor, processor.transferClient, payload); err != nil {
		return err
	}

	// 2. 发送通知给追偿争议发起人（商户或骑手对应的用户）
	if err := processor.sendAppellantNotification(ctx, payload); err != nil {
		log.Error().Err(err).Msg("failed to send appellant notification")
	}

	// 3. 发送通知给索赔用户
	if err := processor.sendClaimantNotification(ctx, payload); err != nil {
		log.Error().Err(err).Msg("failed to send claimant notification")
	}

	log.Info().
		Int64("recovery_dispute_id", payload.RecoveryDisputeID).
		Str("status", payload.Status).
		Msg("recovery dispute result processed successfully")

	return nil
}

func ExecuteRecoveryDisputeResultEffects(ctx context.Context, store db.Store, distributor TaskDistributor, transferClient wechat.TransferClientInterface, payload ProcessRecoveryDisputeResultPayload) error {
	switch payload.Status {
	case "approved":
		if payload.ReleaseActionID > 0 {
			if err := ExecuteClaimReleaseAction(ctx, store, payload.ReleaseActionID); err != nil {
				return fmt.Errorf("execute claim recovery release action for recovery dispute %d: %w", payload.RecoveryDisputeID, err)
			}
		} else if payload.ClaimID != 0 {
			recovery, err := store.GetClaimRecoveryByClaimIDAndTarget(ctx, db.GetClaimRecoveryByClaimIDAndTargetParams{
				ClaimID:        payload.ClaimID,
				RecoveryTarget: pgtype.Text{String: payloadRecoveryTarget(payload), Valid: payloadRecoveryTarget(payload) != ""},
			})
			if err == nil {
				return fmt.Errorf("approved recovery dispute result missing release action id for claim recovery %d", recovery.ID)
			}
			if !errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("get claim recovery for approved recovery dispute result: %w", err)
			}
		}
		if err := penalizeRecoveryDisputeClaimant(ctx, store, payload); err != nil {
			return fmt.Errorf("penalize claimant for approved recovery dispute %d: %w", payload.RecoveryDisputeID, err)
		}
		if err := executeRecoveryDisputeCompensation(ctx, store, distributor, transferClient, payload); err != nil {
			return fmt.Errorf("execute recovery dispute compensation action for recovery dispute %d: %w", payload.RecoveryDisputeID, err)
		}
	case "rejected":
		if err := resumeClaimRecoveryAfterRecoveryDispute(ctx, store, payload); err != nil {
			return fmt.Errorf("resume claim recovery after rejected recovery dispute %d: %w", payload.RecoveryDisputeID, err)
		}
	}

	return nil
}

// penalizeRecoveryDisputeClaimant 惩罚恶意索赔用户的信用分。
func penalizeRecoveryDisputeClaimant(ctx context.Context, store db.Store, payload ProcessRecoveryDisputeResultPayload) error {
	userID := payload.ClaimantUserID
	if userID == 0 && payload.ClaimID != 0 {
		if claim, err := store.GetClaim(ctx, payload.ClaimID); err == nil {
			userID = claim.UserID
		}
	}
	if userID == 0 {
		return nil
	}

	if _, err := store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	}); err == nil {
		return nil
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return err
	}

	blockDays := int64(14)
	if days := getRejectServiceCooldownDays(ctx, store); days > 0 {
		blockDays = days
	}
	blockUntil := time.Now().AddDate(0, 0, int(blockDays))

	if _, err := store.CreateBehaviorBlocklist(ctx, db.CreateBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
		ReasonCode: "malicious-claim",
		BlockUntil: pgtype.Timestamptz{Time: blockUntil, Valid: true},
		Status:     "active",
	}); err != nil {
		return err
	}

	recordRecoveryDisputeClaimWarning(ctx, store, userID)
	return nil
}

func recordRecoveryDisputeClaimWarning(ctx context.Context, store db.Store, userID int64) {
	if _, err := store.GetUserClaimWarningStatus(ctx, userID); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			if _, createErr := store.CreateUserClaimWarning(ctx, db.CreateUserClaimWarningParams{
				UserID:            userID,
				LastWarningReason: pgtype.Text{String: "recovery dispute approved: malicious claim", Valid: true},
				RequiresEvidence:  false,
			}); createErr != nil {
				log.Warn().Err(createErr).Int64("user_id", userID).Msg("failed to create recovery dispute claimant warning")
			}
			return
		}
		log.Warn().Err(err).Int64("user_id", userID).Msg("failed to query recovery dispute claimant warning status")
		return
	}

	if err := store.IncrementUserClaimWarning(ctx, db.IncrementUserClaimWarningParams{
		UserID:            userID,
		LastWarningReason: pgtype.Text{String: "recovery dispute approved: malicious claim", Valid: true},
		RequiresEvidence:  false,
	}); err != nil {
		log.Warn().Err(err).Int64("user_id", userID).Msg("failed to increment recovery dispute claimant warning")
	}
}

func getRejectServiceCooldownDays(ctx context.Context, store db.Store) int64 {
	config, err := store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
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

func executeRecoveryDisputeCompensation(ctx context.Context, store db.Store, distributor TaskDistributor, transferClient wechat.TransferClientInterface, payload ProcessRecoveryDisputeResultPayload) error {
	if payload.Status != "approved" || payload.CompensationActionID == 0 {
		return nil
	}
	return ExecuteClaimPayoutAction(ctx, store, distributor, transferClient, payload.CompensationActionID)
}

func resumeClaimRecoveryAfterRecoveryDispute(ctx context.Context, store db.Store, payload ProcessRecoveryDisputeResultPayload) error {
	if payload.ClaimID == 0 {
		return nil
	}

	recovery, err := store.GetClaimRecoveryByClaimIDAndTarget(ctx, db.GetClaimRecoveryByClaimIDAndTargetParams{
		ClaimID:        payload.ClaimID,
		RecoveryTarget: pgtype.Text{String: payloadRecoveryTarget(payload), Valid: payloadRecoveryTarget(payload) != ""},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("get claim recovery by claim id: %w", err)
	}
	if recovery.Status != "disputed" {
		if recovery.Status == "pending" || recovery.Status == "overdue" {
			return nil
		}
		log.Warn().
			Int64("recovery_dispute_id", payload.RecoveryDisputeID).
			Int64("claim_id", payload.ClaimID).
			Int64("recovery_id", recovery.ID).
			Str("recovery_status", recovery.Status).
			Msg("skip claim recovery resume because recovery is not in disputed status")
		return nil
	}

	_, err = store.ResumeClaimRecoveryAfterDispute(ctx, recovery.ID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return err
	}
	return nil
}

func payloadRecoveryTarget(payload ProcessRecoveryDisputeResultPayload) string {
	if payload.RecoveryTarget != "" {
		return payload.RecoveryTarget
	}
	return payload.AppellantType
}

// sendAppellantNotification 发送通知给追偿争议发起人
func (processor *RedisTaskProcessor) sendAppellantNotification(ctx context.Context, payload ProcessRecoveryDisputeResultPayload) error {
	// 获取追偿争议发起人对应的用户ID
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
		Type:        "recovery_dispute",
		Title:       title,
		Content:     content,
		RelatedType: "recovery_dispute",
		RelatedID:   payload.RecoveryDisputeID,
		ExtraData: map[string]any{
			"recovery_dispute_id": payload.RecoveryDisputeID,
			"status":              payload.Status,
			"appellant_type":      payload.AppellantType,
		},
	}

	err = processor.distributor.DistributeTaskSendNotification(ctx, notificationPayload)
	if err != nil {
		return fmt.Errorf("distribute notification: %w", err)
	}

	return nil
}

// sendClaimantNotification 发送通知给索赔用户
func (processor *RedisTaskProcessor) sendClaimantNotification(ctx context.Context, payload ProcessRecoveryDisputeResultPayload) error {
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
		Type:        "recovery_dispute",
		Title:       title,
		Content:     content,
		RelatedType: "recovery_dispute",
		RelatedID:   payload.RecoveryDisputeID,
		ExtraData: map[string]any{
			"recovery_dispute_id": payload.RecoveryDisputeID,
			"status":              payload.Status,
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
