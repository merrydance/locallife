package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessMerchantWithdrawResult = "payment:process_merchant_withdraw_result"
	merchantWithdrawMaxRetry          = 5
	merchantWithdrawChannel           = "wechat_ecommerce_fund"
	operatorWithdrawChannel           = "wechat_ecommerce_fund_operator"
)

// MerchantWithdrawResultPayload 商户提现状态轮询任务载荷
type MerchantWithdrawResultPayload struct {
	WithdrawalRecordID int64 `json:"withdrawal_record_id"`
	RetryCount         int   `json:"retry_count"`
}

type merchantWithdrawAccountInfo struct {
	MerchantID   int64  `json:"merchant_id,omitempty"`
	OperatorID   int64  `json:"operator_id,omitempty"`
	SubMchID     string `json:"sub_mch_id"`
	OutRequestNo string `json:"out_request_no"`
	WithdrawID   string `json:"withdraw_id,omitempty"`
	Remark       string `json:"remark,omitempty"`
}

func mapWechatWithdrawStatus(status string) string {
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return "success"
	case "FAILED", "CLOSED", "ABNORMAL", "CANCELLED":
		return "failed"
	default:
		return "pending"
	}
}

// DistributeTaskProcessMerchantWithdrawResult 分发商户提现状态轮询任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessMerchantWithdrawResult(
	ctx context.Context,
	payload *MerchantWithdrawResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessMerchantWithdrawResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("withdrawal_record_id", payload.WithdrawalRecordID).
		Int("retry_count", payload.RetryCount).
		Msg("enqueued merchant withdraw result task")

	return nil
}

// ProcessTaskMerchantWithdrawResult 处理商户提现状态轮询任务
func (processor *RedisTaskProcessor) ProcessTaskMerchantWithdrawResult(ctx context.Context, task *asynq.Task) error {
	if processor.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured: %w", asynq.SkipRetry)
	}

	var payload MerchantWithdrawResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	record, err := processor.store.GetWithdrawalRecord(ctx, payload.WithdrawalRecordID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return fmt.Errorf("withdrawal record not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get withdrawal record: %w", err)
	}

	if record.Channel != merchantWithdrawChannel && record.Channel != operatorWithdrawChannel {
		return fmt.Errorf("withdrawal channel mismatch: %w", asynq.SkipRetry)
	}

	if record.Status != "pending" {
		return nil
	}

	var accountInfo merchantWithdrawAccountInfo
	if err := json.Unmarshal(record.AccountInfo, &accountInfo); err != nil {
		return fmt.Errorf("unmarshal account info: %w", asynq.SkipRetry)
	}

	if accountInfo.SubMchID == "" || accountInfo.OutRequestNo == "" {
		return fmt.Errorf("invalid account info for withdrawal record: %w", asynq.SkipRetry)
	}

	resp, err := processor.ecommerceClient.QueryEcommerceWithdrawByOutRequestNo(ctx, accountInfo.SubMchID, accountInfo.OutRequestNo)
	if err != nil {
		if payload.RetryCount >= merchantWithdrawMaxRetry && isWechatWithdrawRequestNotFound(err) {
			_, _ = processor.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
				ID:     record.ID,
				Status: "failed",
				Reason: pgtype.Text{String: fmt.Sprintf("withdraw request not found in wechat after retries: %v", err), Valid: true},
			})
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeWithdrawFailed,
				Level:       AlertLevelCritical,
				Title:       "商户提现提交状态不明",
				Message:     fmt.Sprintf("提现记录 %d 多次查询仍未在微信侧找到对应申请单，已收敛为 failed，请人工确认是否为本地僵尸提现单。", record.ID),
				RelatedID:   record.ID,
				RelatedType: "withdrawal_record",
				Extra: withdrawalAlertExtra(record, accountInfo, map[string]interface{}{
					"retry_count": payload.RetryCount,
					"fail_reason": err.Error(),
					"result":      "withdraw_request_not_found",
				}),
			})
			return fmt.Errorf("withdraw request not found after retries: %w", asynq.SkipRetry)
		}

		if payload.RetryCount >= merchantWithdrawMaxRetry {
			_, _ = processor.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
				ID:     record.ID,
				Status: "pending",
				Reason: pgtype.Text{String: fmt.Sprintf("query withdraw result failed: %v", err), Valid: true},
			})
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeWithdrawFailed,
				Level:       AlertLevelCritical,
				Title:       "商户提现结果查询失败",
				Message:     fmt.Sprintf("提现记录 %d 连续查询失败，状态保持 pending，等待恢复调度继续核对，请人工关注微信提现结果。", record.ID),
				RelatedID:   record.ID,
				RelatedType: "withdrawal_record",
				Extra: withdrawalAlertExtra(record, accountInfo, map[string]interface{}{
					"retry_count": payload.RetryCount,
					"fail_reason": err.Error(),
				}),
			})
			return fmt.Errorf("query withdraw result failed after retries: %w", asynq.SkipRetry)
		}

		if processor.distributor != nil {
			_ = processor.distributor.DistributeTaskProcessMerchantWithdrawResult(
				ctx,
				&MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: payload.RetryCount + 1},
				asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
				asynq.Queue(QueueDefault),
			)
		}

		return nil
	}

	newStatus := mapWechatWithdrawStatus(resp.Status)
	reason := pgtype.Text{}
	if resp.FailReason != "" {
		reason = pgtype.Text{String: resp.FailReason, Valid: true}
	}

	if newStatus != record.Status || reason.Valid {
		_, err = processor.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
			ID:     record.ID,
			Status: newStatus,
			Reason: reason,
		})
		if err != nil {
			return fmt.Errorf("update withdrawal status: %w", err)
		}
	}

	if newStatus == "failed" {
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeWithdrawFailed,
			Level:       AlertLevelCritical,
			Title:       "商户提现失败",
			Message:     fmt.Sprintf("提现记录 %d 状态变为 failed，微信提现单号 %s，请人工介入处理。", record.ID, accountInfo.OutRequestNo),
			RelatedID:   record.ID,
			RelatedType: "withdrawal_record",
			Extra: withdrawalAlertExtra(record, accountInfo, map[string]interface{}{
				"wechat_status": resp.Status,
				"fail_reason":   resp.FailReason,
			}),
		})
	}

	if newStatus == "pending" && payload.RetryCount < merchantWithdrawMaxRetry && processor.distributor != nil {
		_ = processor.distributor.DistributeTaskProcessMerchantWithdrawResult(
			ctx,
			&MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: payload.RetryCount + 1},
			asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
			asynq.Queue(QueueDefault),
		)
	}

	return nil
}

func isWechatWithdrawRequestNotFound(err error) bool {
	var payErr *wechat.WechatPayError
	if !errors.As(err, &payErr) {
		return false
	}

	code := strings.ToUpper(payErr.Code)
	return payErr.StatusCode == 404 || strings.Contains(code, "NOT_FOUND") || strings.Contains(code, "RESOURCE_NOT_EXISTS")
}
