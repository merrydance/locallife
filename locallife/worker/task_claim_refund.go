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
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const (
	// TaskClaimPayout 平台赔付任务。
	TaskClaimPayout = "task:claim_payout"
)

// ClaimPayoutPayload 平台赔付任务载荷。
type ClaimPayoutPayload struct {
	ActionID int64 `json:"action_id"`
}

type claimPayoutActionDetail struct {
	ClaimID          int64      `json:"claim_id"`
	AppealID         int64      `json:"appeal_id"`
	UserID           int64      `json:"user_id"`
	Amount           int64      `json:"amount"`
	SourceType       string     `json:"source_type"`
	SourceID         int64      `json:"source_id"`
	Remark           string     `json:"remark"`
	OutBillNo        string     `json:"out_bill_no,omitempty"`
	TransferBillNo   string     `json:"transfer_bill_no,omitempty"`
	TransferState    string     `json:"transfer_state,omitempty"`
	TransferCreateAt string     `json:"transfer_create_at,omitempty"`
	TransferUpdateAt string     `json:"transfer_update_at,omitempty"`
	FailReason       string     `json:"fail_reason,omitempty"`
	PackageInfo      string     `json:"package_info,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
	LastQueriedAt    *time.Time `json:"last_queried_at,omitempty"`
	TerminalFailure  bool       `json:"terminal_failure,omitempty"`
}

const claimPayoutTransferTimeout = 30 * time.Second

// NewClaimPayoutTask 创建平台赔付任务。
func NewClaimPayoutTask(payload *ClaimPayoutPayload) (*asynq.Task, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskClaimPayout, jsonPayload), nil
}

// ProcessTaskClaimPayout 处理平台赔付任务。
func (processor *RedisTaskProcessor) ProcessTaskClaimPayout(ctx context.Context, task *asynq.Task) error {
	var payload ClaimPayoutPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal claim payout payload: %w", asynq.SkipRetry)
	}

	if err := ExecuteClaimPayoutAction(ctx, processor.store, processor.distributor, processor.transferClient, payload.ActionID); err != nil {
		return fmt.Errorf("failed to execute claim payout action: %w", err)
	}

	return nil
}

// ExecuteClaimPayoutAction 执行索赔赔付动作，behavior_action 作为持久化 outbox。
func ExecuteClaimPayoutAction(ctx context.Context, store db.Store, distributor TaskDistributor, transferClient wechat.TransferClientInterface, actionID int64) error {
	action, err := store.GetBehaviorAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get behavior action: %w", err)
	}
	if action.ActionType != "payout" || action.TargetEntity != "user" {
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, claimPayoutActionDetail{TerminalFailure: true, LastError: "invalid payout action type"}, true)
		return fmt.Errorf("behavior action %d is not a user payout action", action.ID)
	}
	if action.Status == "success" {
		return nil
	}

	var detail claimPayoutActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, claimPayoutActionDetail{TerminalFailure: true, LastError: err.Error()}, true)
		return fmt.Errorf("unmarshal behavior action detail: %w", err)
	}
	if detail.UserID <= 0 || detail.Amount <= 0 || (detail.ClaimID <= 0 && detail.AppealID <= 0) {
		detail.LastError = "invalid behavior action detail for claim payout"
		detail.TerminalFailure = true
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, true)
		return fmt.Errorf("invalid behavior action detail for claim payout")
	}
	if detail.ClaimID > 0 {
		claim, err := store.GetClaim(ctx, detail.ClaimID)
		if err != nil {
			return fmt.Errorf("get claim for payout action: %w", err)
		}
		if claim.Status == db.ClaimStatusWithdrawn || claim.Status == db.ClaimStatusRejected {
			detail.LastError = fmt.Sprintf("claim %d is not eligible for payout execution in status %s", detail.ClaimID, claim.Status)
			detail.TerminalFailure = true
			_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, true)
			return nil
		}
	}
	if transferClient == nil {
		detail.LastError = "transfer client is not configured for claim payout"
		detail.TerminalFailure = false
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, false)
		return fmt.Errorf("transfer client is not configured for claim payout")
	}
	if action.Status == "failed" && detail.TerminalFailure {
		return nil
	}
	if action.Status == "running" && detail.OutBillNo != "" {
		resolved, err := reconcileClaimPayoutTransfer(ctx, store, distributor, transferClient, action.ID, &detail)
		if err != nil {
			return err
		}
		if resolved {
			return nil
		}
	}

	user, err := store.GetUser(ctx, detail.UserID)
	if err != nil {
		detail.LastError = err.Error()
		detail.TerminalFailure = true
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, true)
		return fmt.Errorf("get payout user: %w", err)
	}
	if user.WechatOpenid == "" {
		detail.LastError = fmt.Sprintf("payout user %d missing wechat openid", detail.UserID)
		detail.TerminalFailure = true
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, true)
		return fmt.Errorf("payout user %d missing wechat openid", detail.UserID)
	}
	if strings.TrimSpace(user.FullName) == "" {
		detail.LastError = fmt.Sprintf("payout user %d missing full name", detail.UserID)
		detail.TerminalFailure = true
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, true)
		return fmt.Errorf("payout user %d missing full name", detail.UserID)
	}

	transferReq := buildClaimPayoutTransferRequest(action.ID, detail, user, transferClient)
	detail.OutBillNo = transferReq.OutBillNo
	detail.TerminalFailure = false
	detail.LastError = ""
	if err := updateClaimPayoutAction(ctx, store, action.ID, "running", detail, pgtype.Timestamptz{}); err != nil {
		return fmt.Errorf("mark behavior action running: %w", err)
	}

	transferCtx, cancel := context.WithTimeout(ctx, claimPayoutTransferTimeout)
	defer cancel()

	transferResp, err := transferClient.CreateTransfer(transferCtx, transferReq)
	if err != nil && !isDuplicateClaimTransferError(err) {
		detail.LastError = err.Error()
		detail.TerminalFailure = false
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, false)
		return fmt.Errorf("create claim payout transfer: %w", err)
	}
	if transferResp != nil {
		applyTransferCreateResponse(&detail, transferResp)
	}

	resolved, err := reconcileClaimPayoutTransfer(ctx, store, distributor, transferClient, action.ID, &detail)
	if err != nil {
		return err
	}
	if resolved {
		return nil
	}
	return nil
}

// HandleClaimPayoutTransferNotification 根据商家转账终态通知推进平台赔付动作。
func HandleClaimPayoutTransferNotification(ctx context.Context, store db.Store, distributor TaskDistributor, actionID int64, resource *wechatcontracts.DirectMerchantTransferNotificationResource) error {
	if resource == nil {
		return fmt.Errorf("claim payout transfer notification resource is nil")
	}
	action, err := store.GetBehaviorAction(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get behavior action: %w", err)
	}
	if action.ActionType != "payout" || action.TargetEntity != "user" {
		return fmt.Errorf("behavior action %d is not a user payout action", action.ID)
	}
	if action.Status == "success" {
		return nil
	}

	var detail claimPayoutActionDetail
	if err := json.Unmarshal(action.Detail, &detail); err != nil {
		return fmt.Errorf("unmarshal behavior action detail: %w", err)
	}
	if strings.TrimSpace(detail.OutBillNo) != "" && !strings.EqualFold(strings.TrimSpace(detail.OutBillNo), strings.TrimSpace(resource.OutBillNo)) {
		return fmt.Errorf("claim payout transfer out_bill_no mismatch: stored=%s notified=%s", detail.OutBillNo, resource.OutBillNo)
	}

	applyTransferNotificationResource(&detail, resource)
	switch classifyClaimPayoutTransferState(detail.TransferState) {
	case claimPayoutTransferSucceeded:
		if err := markClaimPayoutOutcome(ctx, store, distributor, detail); err != nil {
			return fmt.Errorf("mark claim payout outcome: %w", err)
		}
		executedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return updateClaimPayoutAction(ctx, store, actionID, "success", detail, executedAt)
	case claimPayoutTransferTerminalFailure:
		detail.TerminalFailure = true
		if strings.TrimSpace(detail.FailReason) != "" {
			detail.LastError = fmt.Sprintf("transfer failed: %s", detail.FailReason)
		} else {
			detail.LastError = fmt.Sprintf("transfer reached terminal failure state: %s", detail.TransferState)
		}
		return updateClaimPayoutAction(ctx, store, actionID, "failed", detail, pgtype.Timestamptz{})
	default:
		detail.TerminalFailure = false
		return updateClaimPayoutAction(ctx, store, actionID, "running", detail, pgtype.Timestamptz{})
	}
}

func updateClaimPayoutAction(ctx context.Context, store db.Store, actionID int64, status string, detail claimPayoutActionDetail, executedAt pgtype.Timestamptz) error {
	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal claim payout action detail: %w", err)
	}
	return store.UpdateBehaviorActionExecution(ctx, db.UpdateBehaviorActionExecutionParams{
		ID:         actionID,
		Status:     status,
		Detail:     detailBytes,
		ExecutedAt: executedAt,
	})
}

func markClaimPayoutActionFailure(ctx context.Context, store db.Store, actionID int64, detail claimPayoutActionDetail, terminal bool) error {
	detail.TerminalFailure = terminal
	status := "failed"
	if !terminal {
		status = "failed"
	}
	return updateClaimPayoutAction(ctx, store, actionID, status, detail, pgtype.Timestamptz{})
}

func buildClaimPayoutTransferRequest(actionID int64, detail claimPayoutActionDetail, user db.User, transferClient wechat.TransferClientInterface) *wechatcontracts.DirectMerchantTransferCreateRequest {
	reason := "异常索赔平台赔付"
	transferRemark := detail.Remark
	if detail.AppealID > 0 {
		reason = "异常索赔申诉补偿"
	}
	if strings.TrimSpace(transferRemark) == "" {
		transferRemark = reason
	}

	return &wechatcontracts.DirectMerchantTransferCreateRequest{
		AppID:              transferClient.GetAppID(),
		OutBillNo:          claimPayoutOutBillNo(actionID),
		TransferSceneID:    wechatcontracts.DirectMerchantTransferSceneEnterpriseCompensation,
		OpenID:             user.WechatOpenid,
		UserName:           strings.TrimSpace(user.FullName),
		TransferAmount:     detail.Amount,
		TransferRemark:     transferRemark,
		UserRecvPerception: wechatcontracts.DirectMerchantTransferUserRecvPerceptionMerchantCompensation,
		TransferSceneReportInfos: []wechatcontracts.DirectMerchantTransferSceneReportInfo{{
			InfoType:    wechatcontracts.DirectMerchantTransferReportInfoTypeCompensationReason,
			InfoContent: transferRemark,
		}},
	}
}

func claimPayoutOutBillNo(actionID int64) string {
	return fmt.Sprintf("claimpayout%d", actionID)
}

func applyTransferCreateResponse(detail *claimPayoutActionDetail, resp *wechatcontracts.DirectMerchantTransferCreateResponse) {
	if resp == nil {
		return
	}
	detail.OutBillNo = resp.OutBillNo
	detail.TransferBillNo = resp.TransferBillNo
	detail.TransferState = resp.State
	detail.TransferCreateAt = resp.CreateTime
	detail.PackageInfo = resp.PackageInfo
}

func applyTransferQueryResponse(detail *claimPayoutActionDetail, resp *wechatcontracts.DirectMerchantTransferQueryResponse) {
	if resp == nil {
		return
	}
	now := time.Now()
	detail.OutBillNo = resp.OutBillNo
	detail.TransferBillNo = resp.TransferBillNo
	detail.TransferState = resp.State
	detail.TransferCreateAt = resp.CreateTime
	detail.TransferUpdateAt = resp.UpdateTime
	detail.FailReason = resp.FailReason
	detail.LastQueriedAt = &now
	detail.LastError = ""
}

func applyTransferNotificationResource(detail *claimPayoutActionDetail, resource *wechatcontracts.DirectMerchantTransferNotificationResource) {
	if resource == nil {
		return
	}
	now := time.Now()
	detail.OutBillNo = resource.OutBillNo
	detail.TransferBillNo = resource.TransferBillNo
	detail.TransferState = resource.State
	detail.TransferCreateAt = resource.CreateTime
	detail.TransferUpdateAt = resource.UpdateTime
	detail.FailReason = resource.FailReason
	detail.LastQueriedAt = &now
	detail.LastError = ""
}

func reconcileClaimPayoutTransfer(ctx context.Context, store db.Store, distributor TaskDistributor, transferClient wechat.TransferClientInterface, actionID int64, detail *claimPayoutActionDetail) (bool, error) {
	if detail.OutBillNo == "" {
		return false, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, claimPayoutTransferTimeout)
	defer cancel()

	transferResp, err := transferClient.QueryTransferByOutBillNo(queryCtx, detail.OutBillNo)
	if err != nil {
		if isTransferNotFoundError(err) {
			detail.TerminalFailure = false
			if updateErr := updateClaimPayoutAction(ctx, store, actionID, "running", *detail, pgtype.Timestamptz{}); updateErr != nil {
				return true, fmt.Errorf("persist unresolved claim payout transfer: %w", updateErr)
			}
			return true, nil
		}
		detail.LastError = err.Error()
		detail.TerminalFailure = false
		if updateErr := updateClaimPayoutAction(ctx, store, actionID, "running", *detail, pgtype.Timestamptz{}); updateErr != nil {
			return true, fmt.Errorf("query claim payout transfer: %w (persist detail: %v)", err, updateErr)
		}
		return true, fmt.Errorf("query claim payout transfer: %w", err)
	}

	applyTransferQueryResponse(detail, transferResp)
	switch classifyClaimPayoutTransferState(detail.TransferState) {
	case claimPayoutTransferSucceeded:
		if err := markClaimPayoutOutcome(ctx, store, distributor, *detail); err != nil {
			detail.LastError = err.Error()
			if updateErr := updateClaimPayoutAction(ctx, store, actionID, "failed", *detail, pgtype.Timestamptz{}); updateErr != nil {
				return true, fmt.Errorf("mark claim payout outcome: %w (persist detail: %v)", err, updateErr)
			}
			return true, fmt.Errorf("mark claim payout outcome: %w", err)
		}
		executedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		if err := updateClaimPayoutAction(ctx, store, actionID, "success", *detail, executedAt); err != nil {
			return true, fmt.Errorf("mark behavior action success: %w", err)
		}
		return true, nil
	case claimPayoutTransferTerminalFailure:
		detail.TerminalFailure = true
		if strings.TrimSpace(detail.FailReason) != "" {
			detail.LastError = fmt.Sprintf("transfer failed: %s", detail.FailReason)
		} else {
			detail.LastError = fmt.Sprintf("transfer reached terminal failure state: %s", detail.TransferState)
		}
		if err := updateClaimPayoutAction(ctx, store, actionID, "failed", *detail, pgtype.Timestamptz{}); err != nil {
			return true, fmt.Errorf("persist failed claim payout transfer: %w", err)
		}
		return true, nil
	default:
		detail.TerminalFailure = false
		if err := updateClaimPayoutAction(ctx, store, actionID, "running", *detail, pgtype.Timestamptz{}); err != nil {
			return true, fmt.Errorf("persist running claim payout transfer: %w", err)
		}
		return true, nil
	}
}

func markClaimPayoutOutcome(ctx context.Context, store db.Store, distributor TaskDistributor, detail claimPayoutActionDetail) error {
	switch {
	case detail.AppealID > 0:
		return store.MarkAppealCompensated(ctx, db.MarkAppealCompensatedParams{
			ID:            detail.AppealID,
			CompensatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	case detail.ClaimID > 0:
		paidAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		if err := store.MarkClaimPaid(ctx, db.MarkClaimPaidParams{
			ID:     detail.ClaimID,
			PaidAt: paidAt,
		}); err != nil {
			return err
		}
		result, err := store.FinalizeClaimCompensationAfterPayoutTx(ctx, db.FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: detail.ClaimID})
		if err != nil {
			return err
		}
		if err := enqueueClaimPostPayoutActions(ctx, distributor, result); err != nil {
			log.Warn().Err(err).Int64("claim_id", detail.ClaimID).Msg("enqueue claim post-payout actions failed; recovery scheduler will retry")
		}
		return nil
	default:
		return fmt.Errorf("unsupported behavior action detail")
	}
}

func enqueueClaimPostPayoutActions(ctx context.Context, distributor TaskDistributor, result db.FinalizeClaimCompensationAfterPayoutTxResult) error {
	if distributor == nil {
		return nil
	}
	if result.RestrictionAction != nil {
		if err := distributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&ClaimBehaviorActionPayload{ActionID: result.RestrictionAction.ID},
			asynq.Queue(QueueCritical),
			asynq.MaxRetry(10),
		); err != nil {
			return fmt.Errorf("enqueue claim restriction action %d: %w", result.RestrictionAction.ID, err)
		}
	}
	if result.NotificationAction != nil {
		if err := distributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&ClaimBehaviorActionPayload{ActionID: result.NotificationAction.ID},
			asynq.Queue(QueueDefault),
			asynq.MaxRetry(5),
		); err != nil {
			return fmt.Errorf("enqueue claim notification action %d: %w", result.NotificationAction.ID, err)
		}
	}
	return nil
}

type claimPayoutTransferResolution int

const (
	claimPayoutTransferPending claimPayoutTransferResolution = iota
	claimPayoutTransferSucceeded
	claimPayoutTransferTerminalFailure
)

func classifyClaimPayoutTransferState(transferState string) claimPayoutTransferResolution {
	switch strings.ToUpper(strings.TrimSpace(transferState)) {
	case wechatcontracts.DirectMerchantTransferStateSuccess:
		return claimPayoutTransferSucceeded
	case wechatcontracts.DirectMerchantTransferStateFail, wechatcontracts.DirectMerchantTransferStateCancelled:
		return claimPayoutTransferTerminalFailure
	default:
		return claimPayoutTransferPending
	}
}

func isTransferNotFoundError(err error) bool {
	var payErr *wechat.WechatPayError
	if !errors.As(err, &payErr) {
		return false
	}
	code := strings.ToUpper(payErr.Code)
	return payErr.StatusCode == 404 || strings.Contains(code, "NOT_FOUND") || strings.Contains(code, "RESOURCE_NOT_EXISTS")
}

func isDuplicateClaimTransferError(err error) bool {
	var payErr *wechat.WechatPayError
	if !errors.As(err, &payErr) {
		return false
	}
	code := strings.ToUpper(payErr.Code)
	return strings.Contains(code, "ALREADY_EXISTS") || strings.Contains(code, "DUPLICATE")
}

// DistributeTaskClaimPayout 分发平台赔付任务。
func (distributor *RedisTaskDistributor) DistributeTaskClaimPayout(
	ctx context.Context,
	payload *ClaimPayoutPayload,
	opts ...asynq.Option,
) error {
	task, err := NewClaimPayoutTask(payload)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
