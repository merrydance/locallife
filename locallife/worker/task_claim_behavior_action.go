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

type claimRecoveryBlockActionDetail struct {
	Action          string    `json:"action"`
	ClaimID         int64     `json:"claim_id"`
	RecoveryID      int64     `json:"recovery_id"`
	TargetEntity    string    `json:"target_entity"`
	TargetID        int64     `json:"target_id"`
	SuspendReason   string    `json:"suspend_reason,omitempty"`
	SuspendUntil    time.Time `json:"suspend_until"`
	Remark          string    `json:"remark"`
	LastError       string    `json:"last_error,omitempty"`
	TerminalFailure bool      `json:"terminal_failure,omitempty"`
}

type claimRecoveryOpenActionDetail struct {
	Action          string    `json:"action"`
	ClaimID         int64     `json:"claim_id"`
	RecoveryID      int64     `json:"recovery_id"`
	TargetEntity    string    `json:"target_entity"`
	TargetID        int64     `json:"target_id,omitempty"`
	RecoveryBasis   string    `json:"recovery_basis,omitempty"`
	RecoveryAmount  int64     `json:"recovery_amount"`
	DueAt           time.Time `json:"due_at"`
	Remark          string    `json:"remark"`
	LastError       string    `json:"last_error,omitempty"`
	TerminalFailure bool      `json:"terminal_failure,omitempty"`
}

type claimRecoveryReleaseActionDetail struct {
	Action          string `json:"action"`
	ClaimID         int64  `json:"claim_id"`
	RecoveryID      int64  `json:"recovery_id"`
	OrderID         int64  `json:"order_id"`
	TargetEntity    string `json:"target_entity"`
	Remark          string `json:"remark"`
	LastError       string `json:"last_error,omitempty"`
	TerminalFailure bool   `json:"terminal_failure,omitempty"`
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
		switch action.TargetEntity {
		case "user":
			return processor.executeClaimRestrictionAction(ctx, action)
		case "merchant", "rider":
			return processor.executeClaimRecoveryBlockAction(ctx, action)
		default:
			return nil
		}
	case "recovery":
		switch action.TargetEntity {
		case "merchant", "rider":
			return processor.executeClaimRecoveryOpenAction(ctx, action)
		default:
			return nil
		}
	case "release":
		switch action.TargetEntity {
		case "merchant", "rider":
			return executeClaimRecoveryReleaseAction(ctx, processor.store, action)
		default:
			return nil
		}
	case "notify":
		return processor.executeClaimNotificationAction(ctx, action)
	default:
		return nil
	}
}

func ExecuteClaimReleaseAction(ctx context.Context, store db.Store, actionID int64) error {
	action, err := store.GetBehaviorAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get behavior action: %w", err)
	}
	return executeClaimRecoveryReleaseActionStrict(ctx, store, action)
}

func executeClaimRecoveryReleaseActionStrict(ctx context.Context, store db.Store, action db.BehaviorAction) error {
	return executeClaimRecoveryReleaseActionWithMode(ctx, store, action, true)
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
	if days := getRejectServiceCooldownDays(ctx, processor.store); days > 0 {
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

func (processor *RedisTaskProcessor) executeClaimRecoveryBlockAction(ctx context.Context, action db.BehaviorAction) error {
	var detail claimRecoveryBlockActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, claimRecoveryBlockActionDetail{LastError: err.Error(), TerminalFailure: true})
		return nil
	}
	if action.ActionType != "block" {
		detail.LastError = "invalid claim recovery block action type"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if detail.RecoveryID <= 0 || detail.TargetID <= 0 || detail.TargetEntity == "" {
		detail.LastError = "invalid claim recovery block action detail"
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

	suspendUntil := pgtype.Timestamptz{Time: detail.SuspendUntil, Valid: !detail.SuspendUntil.IsZero()}
	switch action.TargetEntity {
	case "merchant":
		if err := processor.store.SuspendMerchantTakeout(ctx, db.SuspendMerchantTakeoutParams{
			MerchantID:           detail.TargetID,
			TakeoutSuspendReason: pgtype.Text{String: detail.SuspendReason, Valid: detail.SuspendReason != ""},
			TakeoutSuspendUntil:  suspendUntil,
		}); err != nil {
			detail.LastError = err.Error()
			_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
			return fmt.Errorf("suspend merchant for claim recovery: %w", err)
		}
	case "rider":
		if err := processor.store.SuspendRider(ctx, db.SuspendRiderParams{
			RiderID:       detail.TargetID,
			SuspendReason: pgtype.Text{String: detail.SuspendReason, Valid: detail.SuspendReason != ""},
			SuspendUntil:  suspendUntil,
		}); err != nil {
			detail.LastError = err.Error()
			_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
			return fmt.Errorf("suspend rider for claim recovery: %w", err)
		}
	default:
		detail.LastError = "invalid claim recovery block target"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}

	return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
}

func (processor *RedisTaskProcessor) executeClaimRecoveryOpenAction(ctx context.Context, action db.BehaviorAction) error {
	var detail claimRecoveryOpenActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, claimRecoveryOpenActionDetail{LastError: err.Error(), TerminalFailure: true})
		return nil
	}
	if action.ActionType != "recovery" {
		detail.LastError = "invalid claim recovery open action type"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if detail.ClaimID <= 0 || detail.RecoveryID <= 0 || detail.TargetEntity == "" || action.TargetEntity != detail.TargetEntity || detail.RecoveryAmount <= 0 {
		detail.LastError = "invalid claim recovery open action detail"
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

	recovery, err := processor.store.GetClaimRecoveryByID(ctx, detail.RecoveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			detail.LastError = fmt.Sprintf("claim recovery for claim %d not found", detail.ClaimID)
			detail.TerminalFailure = true
			_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
			return nil
		}
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("get claim recovery for open action: %w", err)
	}
	if err := validateClaimRecoveryOpenAction(detail, recovery); err != nil {
		detail.LastError = err.Error()
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return nil
	}
	if err := ensureClaimRecoveryOpenEvents(ctx, processor.store, recovery, detail); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, processor.store, action.ID, detail)
		return fmt.Errorf("ensure claim recovery open events: %w", err)
	}

	return markClaimBehaviorActionSuccess(ctx, processor.store, action.ID, detail)
}

func validateClaimRecoveryOpenAction(detail claimRecoveryOpenActionDetail, recovery db.ClaimRecovery) error {
	if recovery.ID != detail.RecoveryID {
		return fmt.Errorf("claim recovery %d does not match open action recovery %d", recovery.ID, detail.RecoveryID)
	}
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != detail.TargetEntity {
		return fmt.Errorf("claim recovery %d target %q does not match open action target %q", recovery.ID, recovery.RecoveryTarget.String, detail.TargetEntity)
	}
	if recovery.RecoveryAmount != detail.RecoveryAmount {
		return fmt.Errorf("claim recovery %d amount %d does not match open action amount %d", recovery.ID, recovery.RecoveryAmount, detail.RecoveryAmount)
	}
	if detail.RecoveryBasis != "" && (!recovery.RecoveryBasis.Valid || recovery.RecoveryBasis.String != detail.RecoveryBasis) {
		return fmt.Errorf("claim recovery %d basis %q does not match open action basis %q", recovery.ID, recovery.RecoveryBasis.String, detail.RecoveryBasis)
	}
	if !detail.DueAt.IsZero() && !recovery.DueAt.UTC().Truncate(time.Second).Equal(detail.DueAt.UTC().Truncate(time.Second)) {
		return fmt.Errorf("claim recovery %d due_at %s does not match open action due_at %s", recovery.ID, recovery.DueAt.UTC().Format(time.RFC3339), detail.DueAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func ensureClaimRecoveryOpenEvents(ctx context.Context, store db.Store, recovery db.ClaimRecovery, detail claimRecoveryOpenActionDetail) error {
	events, err := store.ListClaimRecoveryEventsByRecovery(ctx, recovery.ID)
	if err != nil {
		return fmt.Errorf("list claim recovery events: %w", err)
	}

	hasCreated := false
	hasPayable := false
	for _, event := range events {
		switch event.EventType {
		case db.ClaimRecoveryEventTypeCreated:
			hasCreated = true
		case db.ClaimRecoveryEventTypePayable:
			hasPayable = true
		}
	}

	payload := map[string]any{
		"recovery_target": detail.TargetEntity,
		"recovery_basis":  detail.RecoveryBasis,
		"recovery_amount": detail.RecoveryAmount,
	}
	if !hasCreated {
		if err := db.WriteClaimRecoveryEvent(ctx, store, recovery, db.ClaimRecoveryEventTypeCreated, payload); err != nil {
			return err
		}
	}
	if !hasPayable {
		if err := db.WriteClaimRecoveryEvent(ctx, store, recovery, db.ClaimRecoveryEventTypePayable, payload); err != nil {
			return err
		}
	}

	return nil
}

func executeClaimRecoveryReleaseAction(ctx context.Context, store db.Store, action db.BehaviorAction) error {
	return executeClaimRecoveryReleaseActionWithMode(ctx, store, action, false)
}

func executeClaimRecoveryReleaseActionWithMode(ctx context.Context, store db.Store, action db.BehaviorAction, strict bool) error {
	var detail claimRecoveryReleaseActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, claimRecoveryReleaseActionDetail{LastError: err.Error(), TerminalFailure: true})
		if strict {
			return fmt.Errorf("unmarshal claim recovery release action detail: %w", err)
		}
		return nil
	}
	if action.ActionType != "release" {
		detail.LastError = "invalid claim recovery release action type"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		if strict {
			return errors.New(detail.LastError)
		}
		return nil
	}
	if detail.RecoveryID <= 0 || detail.OrderID <= 0 || detail.TargetEntity == "" || action.TargetEntity != detail.TargetEntity {
		detail.LastError = "invalid claim recovery release action detail"
		detail.TerminalFailure = true
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		if strict {
			return errors.New(detail.LastError)
		}
		return nil
	}
	if action.Status == "success" {
		return nil
	}
	if action.Status == "failed" && detail.TerminalFailure {
		if strict {
			if detail.LastError != "" {
				return fmt.Errorf("claim recovery release action %d already failed terminally: %s", action.ID, detail.LastError)
			}
			return fmt.Errorf("claim recovery release action %d already failed terminally", action.ID)
		}
		return nil
	}

	if action.Status != "running" {
		claimed, err := claimBehaviorActionMarkRunning(ctx, store, action, detail)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
	}

	recovery, err := store.GetClaimRecoveryByID(ctx, detail.RecoveryID)
	if err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		return fmt.Errorf("get claim recovery for release action: %w", err)
	}
	if err := validateClaimRecoveryReleaseAction(detail, recovery); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		if strict {
			return errors.New(detail.LastError)
		}
		return nil
	}

	if err := db.ReleaseClaimRecoverySuspensionIfClear(ctx, store, recovery); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		return fmt.Errorf("release claim recovery suspension: %w", err)
	}

	if err := db.WriteClaimRecoveryClosedEventIfAbsent(ctx, store, recovery, map[string]any{
		"claim_id":          recovery.ClaimID,
		"recovery_target":   recovery.RecoveryTarget.String,
		"recovery_amount":   recovery.RecoveryAmount,
		"status":            recovery.Status,
		"release_action_id": action.ID,
	}); err != nil {
		detail.LastError = err.Error()
		_ = markClaimBehaviorActionFailure(ctx, store, action.ID, detail)
		return fmt.Errorf("write claim recovery closed event: %w", err)
	}

	return markClaimBehaviorActionSuccess(ctx, store, action.ID, detail)
}

func validateClaimRecoveryReleaseAction(detail claimRecoveryReleaseActionDetail, recovery db.ClaimRecovery) error {
	if recovery.ID != detail.RecoveryID {
		return fmt.Errorf("claim recovery %d does not match release action recovery %d", recovery.ID, detail.RecoveryID)
	}
	if recovery.ClaimID != detail.ClaimID {
		return fmt.Errorf("claim recovery %d claim %d does not match release action claim %d", recovery.ID, recovery.ClaimID, detail.ClaimID)
	}
	if recovery.OrderID != detail.OrderID {
		return fmt.Errorf("claim recovery %d order %d does not match release action order %d", recovery.ID, recovery.OrderID, detail.OrderID)
	}
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != detail.TargetEntity {
		return fmt.Errorf("claim recovery %d target %q does not match release action target %q", recovery.ID, recovery.RecoveryTarget.String, detail.TargetEntity)
	}
	return nil
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
