package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// TaskClaimBehaviorAction 执行索赔补偿阶段的 notify/block 行为动作。
	TaskClaimBehaviorAction = "task:claim_behavior_action"
)

// ClaimBehaviorActionPayload 索赔行为动作任务载荷。
type ClaimBehaviorActionPayload struct {
	ActionID int64 `json:"action_id"`
}

type claimRestrictionActionDetail struct {
	Action            string `json:"action"`
	ClaimID           int64  `json:"claim_id"`
	UserID            int64  `json:"user_id"`
	DecisionMode      string `json:"decision_mode"`
	RestrictionReason string `json:"restriction_reason,omitempty"`
	Remark            string `json:"remark"`
	LastError         string `json:"last_error,omitempty"`
	TerminalFailure   bool   `json:"terminal_failure,omitempty"`
}

type claimNotifyActionDetail struct {
	Action           string `json:"action"`
	ClaimID          int64  `json:"claim_id"`
	TargetEntity     string `json:"target_entity"`
	TargetID         int64  `json:"target_id,omitempty"`
	RecipientUserID  int64  `json:"recipient_user_id,omitempty"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	RelatedType      string `json:"related_type"`
	RelatedID        int64  `json:"related_id"`
	Remark           string `json:"remark"`
	LastError        string `json:"last_error,omitempty"`
	TerminalFailure  bool   `json:"terminal_failure,omitempty"`
}

// NewClaimBehaviorActionTask 创建索赔行为动作任务。
func NewClaimBehaviorActionTask(payload *ClaimBehaviorActionPayload) (*asynq.Task, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskClaimBehaviorAction, jsonPayload), nil
}

// ProcessTaskClaimBehaviorAction 处理索赔补偿阶段的 block/notify 行为动作。
func (processor *RedisTaskProcessor) ProcessTaskClaimBehaviorAction(ctx context.Context, task *asynq.Task) error {
	var payload ClaimBehaviorActionPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal claim behavior action payload: %w", asynq.SkipRetry)
	}

	if err := processor.executeClaimBehaviorAction(ctx, payload.ActionID); err != nil {
		return fmt.Errorf("failed to execute claim behavior action: %w", err)
	}

	return nil
}

func (processor *RedisTaskProcessor) executeClaimBehaviorAction(ctx context.Context, actionID int64) error {
	action, err := processor.store.GetBehaviorAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get behavior action: %w", err)
	}

	switch action.ActionType {
	case "block":
		return processor.executeClaimRestrictionAction(ctx, action)
	case "notify":
		return processor.executeClaimNotificationAction(ctx, action)
	default:
		return nil
	}
}

func (processor *RedisTaskProcessor) executeClaimRestrictionAction(ctx context.Context, action db.BehaviorAction) error {
	var detail claimRestrictionActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, claimRestrictionActionDetail{LastError: err.Error(), TerminalFailure: true})
		return nil
	}
	if action.ActionType != "block" || action.TargetEntity != "user" {
		detail.LastError = "invalid claim restriction action type"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if detail.UserID <= 0 || detail.ClaimID <= 0 {
		detail.LastError = "invalid claim restriction action detail"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if action.Status == "success" || (action.Status == "failed" && detail.TerminalFailure) {
		return nil
	}

	if action.Status != "running" {
		claimed, err := claimBehaviorActionMarkRunning(ctx, processor.store, action, detail)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
	}

	claim, err := processor.store.GetClaim(ctx, detail.ClaimID)
	if err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("get claim for restriction action: %w", err)
	}
	if !claim.PaidAt.Valid {
		if claim.Status == db.ClaimStatusRejected || claim.Status == db.ClaimStatusWithdrawn {
			detail.LastError = fmt.Sprintf("claim %d is not eligible for post-payout restriction in status %s", detail.ClaimID, claim.Status)
			detail.TerminalFailure = true
			_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
			return nil
		}
		return resetClaimBehaviorActionToCreated(ctx, processor.store, action.ID, detail)
	}

	if _, err := processor.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   detail.UserID,
	}); err == nil {
		return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("get active behavior blocklist: %w", err)
	}

	blockDays := int64(14)
	if days := processor.getRejectServiceCooldownDays(ctx); days > 0 {
		blockDays = days
	}
	blockUntil := time.Now().AddDate(0, 0, int(blockDays))

	if _, err := processor.store.CreateBehaviorBlocklist(ctx, db.CreateBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   detail.UserID,
		ReasonCode: "malicious-claims",
		BlockUntil: pgtype.Timestamptz{Time: blockUntil, Valid: true},
		Status:     "active",
	}); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("create behavior blocklist: %w", err)
	}

	return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
}

func (processor *RedisTaskProcessor) executeClaimNotificationAction(ctx context.Context, action db.BehaviorAction) error {
	var detail claimNotifyActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, claimNotifyActionDetail{LastError: err.Error(), TerminalFailure: true})
		return nil
	}
	if action.ActionType != "notify" {
		detail.LastError = "invalid claim notify action type"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if detail.ClaimID <= 0 || detail.RecipientUserID <= 0 || detail.Title == "" || detail.Content == "" {
		detail.LastError = "invalid claim notify action detail"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if action.Status == "success" || (action.Status == "failed" && detail.TerminalFailure) {
		return nil
	}

	if action.Status != "running" {
		claimed, err := claimBehaviorActionMarkRunning(ctx, processor.store, action, detail)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
	}

	claim, err := processor.store.GetClaim(ctx, detail.ClaimID)
	if err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("get claim for notify action: %w", err)
	}
	if !claim.PaidAt.Valid {
		if claim.Status == db.ClaimStatusRejected || claim.Status == db.ClaimStatusWithdrawn {
			detail.LastError = fmt.Sprintf("claim %d is not eligible for post-payout notification in status %s", detail.ClaimID, claim.Status)
			detail.TerminalFailure = true
			_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
			return nil
		}
		return resetClaimBehaviorActionToCreated(ctx, processor.store, action.ID, detail)
	}

	existingNotification, err := processor.findExistingClaimNotification(ctx, detail)
	if err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("find existing claim notification: %w", err)
	}
	if existingNotification != nil {
		if !existingNotification.IsPushed {
			if err := processor.tryWebSocketPush(ctx, detail.RecipientUserID, *existingNotification); err != nil {
				detail.LastError = err.Error()
				_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
				return fmt.Errorf("push existing claim notification: %w", err)
			}
		}
		return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
	}

	if err := processor.executeSendNotificationPayload(ctx, SendNotificationPayload{
		UserID:            detail.RecipientUserID,
		Type:              detail.NotificationType,
		Title:             detail.Title,
		Content:           detail.Content,
		RelatedType:       detail.RelatedType,
		RelatedID:         detail.RelatedID,
		IgnorePreferences: true,
	}); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("execute send notification payload: %w", err)
	}

	return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
}

func (processor *RedisTaskProcessor) findExistingClaimNotification(ctx context.Context, detail claimNotifyActionDetail) (*db.Notification, error) {
	notifications, err := processor.store.GetNotificationsByRelated(ctx, db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: detail.RelatedType, Valid: detail.RelatedType != ""},
		RelatedID:   pgtype.Int8{Int64: detail.RelatedID, Valid: detail.RelatedID > 0},
	})
	if err != nil {
		return nil, err
	}
	for _, notification := range notifications {
		if notification.UserID != detail.RecipientUserID {
			continue
		}
		if notification.Type != detail.NotificationType {
			continue
		}
		if strings.TrimSpace(notification.Title) != detail.Title {
			continue
		}
		if strings.TrimSpace(notification.Content) != detail.Content {
			continue
		}
		copied := notification
		return &copied, nil
	}
	return nil, nil
}

func claimBehaviorActionMarkRunning(ctx context.Context, store db.Store, action db.BehaviorAction, detail any) (bool, error) {
	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return false, fmt.Errorf("marshal claim behavior action detail: %w", err)
	}

	_, err = store.UpdateBehaviorActionExecutionIfCurrent(ctx, db.UpdateBehaviorActionExecutionIfCurrentParams{
		ID:         action.ID,
		Status:     action.Status,
		Status_2:   "running",
		Detail:     detailBytes,
		ExecutedAt: pgtype.Timestamptz{},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("mark behavior action running: %w", err)
	}

	return true, nil
}

func markClaimBehaviorActionSuccess(ctx context.Context, store db.Store, actionID int64, detail any) error {
	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal claim behavior action detail: %w", err)
	}
	return store.UpdateBehaviorActionExecution(ctx, db.UpdateBehaviorActionExecutionParams{
		ID:         actionID,
		Status:     "success",
		Detail:     detailBytes,
		ExecutedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
}

func markClaimBehaviorActionFailure(ctx context.Context, store db.Store, actionID int64, detail any) error {
	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal claim behavior action detail: %w", err)
	}
	return store.UpdateBehaviorActionExecution(ctx, db.UpdateBehaviorActionExecutionParams{
		ID:         actionID,
		Status:     "failed",
		Detail:     detailBytes,
		ExecutedAt: pgtype.Timestamptz{},
	})
}

func resetClaimBehaviorActionToCreated(ctx context.Context, store db.Store, actionID int64, detail any) error {
	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal claim behavior action detail: %w", err)
	}
	return store.UpdateBehaviorActionExecution(ctx, db.UpdateBehaviorActionExecutionParams{
		ID:         actionID,
		Status:     "created",
		Detail:     detailBytes,
		ExecutedAt: pgtype.Timestamptz{},
	})
}

// DistributeTaskClaimBehaviorAction 分发索赔行为动作执行任务。
func (distributor *RedisTaskDistributor) DistributeTaskClaimBehaviorAction(
	ctx context.Context,
	payload *ClaimBehaviorActionPayload,
	opts ...asynq.Option,
) error {
	task, err := NewClaimBehaviorActionTask(payload)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
