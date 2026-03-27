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
	OutBatchNo       string     `json:"out_batch_no,omitempty"`
	BatchID          string     `json:"batch_id,omitempty"`
	BatchStatus      string     `json:"batch_status,omitempty"`
	TransferCreateAt string     `json:"transfer_create_at,omitempty"`
	TransferUpdateAt string     `json:"transfer_update_at,omitempty"`
	CloseReason      string     `json:"close_reason,omitempty"`
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

	if err := ExecuteClaimPayoutAction(ctx, processor.store, processor.paymentClient, payload.ActionID); err != nil {
		return fmt.Errorf("failed to execute claim payout action: %w", err)
	}

	return nil
}

// ExecuteClaimPayoutAction 执行索赔赔付动作，behavior_action 作为持久化 outbox。
func ExecuteClaimPayoutAction(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, actionID int64) error {
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
	if paymentClient == nil {
		detail.LastError = "payment client is not configured for claim payout"
		detail.TerminalFailure = false
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, false)
		return fmt.Errorf("payment client is not configured for claim payout")
	}
	if action.Status == "failed" && detail.TerminalFailure {
		return nil
	}
	if action.Status == "running" && detail.OutBatchNo != "" {
		resolved, err := reconcileClaimPayoutTransfer(ctx, store, paymentClient, action.ID, &detail)
		if resolved || err != nil {
			return err
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

	transferReq := buildClaimPayoutTransferRequest(action.ID, detail, user)
	detail.OutBatchNo = transferReq.OutBatchNo
	detail.TerminalFailure = false
	detail.LastError = ""
	if err := updateClaimPayoutAction(ctx, store, action.ID, "running", detail, pgtype.Timestamptz{}); err != nil {
		return fmt.Errorf("mark behavior action running: %w", err)
	}

	transferCtx, cancel := context.WithTimeout(ctx, claimPayoutTransferTimeout)
	defer cancel()

	transferResp, err := paymentClient.CreateTransfer(transferCtx, transferReq)
	if err != nil && !isDuplicateClaimTransferError(err) {
		detail.LastError = err.Error()
		detail.TerminalFailure = false
		_ = markClaimPayoutActionFailure(ctx, store, action.ID, detail, false)
		return fmt.Errorf("create claim payout transfer: %w", err)
	}
	if transferResp != nil {
		applyTransferCreateResponse(&detail, transferResp)
	}

	resolved, err := reconcileClaimPayoutTransfer(ctx, store, paymentClient, action.ID, &detail)
	if resolved || err != nil {
		return err
	}
	return fmt.Errorf("claim payout transfer reconciliation returned without final state")
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

func buildClaimPayoutTransferRequest(actionID int64, detail claimPayoutActionDetail, user db.User) *wechat.TransferRequest {
	batchName := "异常索赔平台赔付"
	batchRemark := "claim payout"
	transferRemark := detail.Remark
	if detail.AppealID > 0 {
		batchName = "异常索赔申诉补偿"
		batchRemark = "appeal compensation"
	}
	if strings.TrimSpace(transferRemark) == "" {
		transferRemark = batchRemark
	}

	return &wechat.TransferRequest{
		OutBatchNo:     claimPayoutOutBatchNo(actionID),
		BatchName:      batchName,
		BatchRemark:    batchRemark,
		TransferAmount: detail.Amount,
		OpenID:         user.WechatOpenid,
		UserName:       strings.TrimSpace(user.FullName),
		TransferRemark: transferRemark,
	}
}

func claimPayoutOutBatchNo(actionID int64) string {
	return fmt.Sprintf("claimpayout%d", actionID)
}

func applyTransferCreateResponse(detail *claimPayoutActionDetail, resp *wechat.TransferResponse) {
	if resp == nil {
		return
	}
	detail.OutBatchNo = resp.OutBatchNo
	detail.BatchID = resp.BatchID
	detail.BatchStatus = resp.BatchStatus
	detail.TransferCreateAt = resp.CreateTime
}

func applyTransferQueryResponse(detail *claimPayoutActionDetail, resp *wechat.TransferQueryResponse) {
	if resp == nil {
		return
	}
	now := time.Now()
	detail.OutBatchNo = resp.OutBatchNo
	detail.BatchID = resp.BatchID
	detail.BatchStatus = resp.BatchStatus
	detail.TransferCreateAt = resp.CreateTime
	detail.TransferUpdateAt = resp.UpdateTime
	detail.CloseReason = resp.CloseReason
	detail.LastQueriedAt = &now
	detail.LastError = ""
}

func reconcileClaimPayoutTransfer(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, actionID int64, detail *claimPayoutActionDetail) (bool, error) {
	if detail.OutBatchNo == "" {
		return false, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, claimPayoutTransferTimeout)
	defer cancel()

	transferResp, err := paymentClient.QueryTransfer(queryCtx, detail.OutBatchNo)
	if err != nil {
		if isTransferNotFoundError(err) {
			return false, nil
		}
		detail.LastError = err.Error()
		detail.TerminalFailure = false
		if updateErr := updateClaimPayoutAction(ctx, store, actionID, "running", *detail, pgtype.Timestamptz{}); updateErr != nil {
			return true, fmt.Errorf("query claim payout transfer: %w (persist detail: %v)", err, updateErr)
		}
		return true, fmt.Errorf("query claim payout transfer: %w", err)
	}

	applyTransferQueryResponse(detail, transferResp)
	switch classifyClaimPayoutTransferStatus(detail.BatchStatus) {
	case claimPayoutTransferSucceeded:
		if err := markClaimPayoutOutcome(ctx, store, *detail); err != nil {
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
	case claimPayoutTransferClosed:
		detail.TerminalFailure = true
		if detail.CloseReason != "" {
			detail.LastError = fmt.Sprintf("transfer closed: %s", detail.CloseReason)
		} else {
			detail.LastError = "transfer closed"
		}
		if err := updateClaimPayoutAction(ctx, store, actionID, "failed", *detail, pgtype.Timestamptz{}); err != nil {
			return true, fmt.Errorf("persist closed claim payout transfer: %w", err)
		}
		return true, fmt.Errorf("claim payout transfer closed: %s", detail.CloseReason)
	default:
		detail.TerminalFailure = false
		if err := updateClaimPayoutAction(ctx, store, actionID, "running", *detail, pgtype.Timestamptz{}); err != nil {
			return true, fmt.Errorf("persist running claim payout transfer: %w", err)
		}
		return true, fmt.Errorf("claim payout transfer still processing: %s", detail.BatchStatus)
	}
}

func markClaimPayoutOutcome(ctx context.Context, store db.Store, detail claimPayoutActionDetail) error {
	switch {
	case detail.AppealID > 0:
		return store.MarkAppealCompensated(ctx, db.MarkAppealCompensatedParams{
			ID:            detail.AppealID,
			CompensatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	case detail.ClaimID > 0:
		return store.MarkClaimPaid(ctx, db.MarkClaimPaidParams{
			ID:     detail.ClaimID,
			PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	default:
		return fmt.Errorf("unsupported behavior action detail")
	}
}

type claimPayoutTransferResolution int

const (
	claimPayoutTransferPending claimPayoutTransferResolution = iota
	claimPayoutTransferSucceeded
	claimPayoutTransferClosed
)

func classifyClaimPayoutTransferStatus(batchStatus string) claimPayoutTransferResolution {
	switch strings.ToUpper(strings.TrimSpace(batchStatus)) {
	case "FINISHED", "SUCCESS":
		return claimPayoutTransferSucceeded
	case "CLOSED", "FAILED":
		return claimPayoutTransferClosed
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
	return strings.Contains(code, "OUT_BATCH_NO") || strings.Contains(code, "BATCH_ALREADY") || strings.Contains(code, "DUPLICATE")
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
