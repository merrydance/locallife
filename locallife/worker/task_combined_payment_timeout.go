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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const (
	TaskCombinedPaymentOrderTimeout = "combined_payment_order:timeout"
)

// PayloadCombinedPaymentOrderTimeout 合单支付超时任务载荷
type PayloadCombinedPaymentOrderTimeout struct {
	CombineOutTradeNo string `json:"combine_out_trade_no"`
}

// DistributeTaskCombinedPaymentOrderTimeout 分发合单支付超时任务
func (d *RedisTaskDistributor) DistributeTaskCombinedPaymentOrderTimeout(
	ctx context.Context,
	payload *PayloadCombinedPaymentOrderTimeout,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskCombinedPaymentOrderTimeout, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Str("combine_out_trade_no", payload.CombineOutTradeNo).
		Msg("enqueued combined payment order timeout task")

	return nil
}

// ProcessTaskCombinedPaymentOrderTimeout 处理合单支付超时任务
func (p *RedisTaskProcessor) ProcessTaskCombinedPaymentOrderTimeout(ctx context.Context, task *asynq.Task) error {
	var payload PayloadCombinedPaymentOrderTimeout
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("type", task.Type()).
		Str("combine_out_trade_no", payload.CombineOutTradeNo).
		Msg("processing combined payment order timeout task")

	if p.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured")
	}

	combined, err := p.store.GetCombinedPaymentOrderByOutTradeNo(ctx, payload.CombineOutTradeNo)
	if err != nil {
		return fmt.Errorf("get combined payment order: %w", err)
	}

	// 检查是否已超时（在关闭之前先检查）
	if combined.ExpiresAt.Valid && time.Now().Before(combined.ExpiresAt.Time) {
		log.Info().
			Str("combine_out_trade_no", payload.CombineOutTradeNo).
			Time("expire_time", combined.ExpiresAt.Time).
			Msg("combined payment order not expired yet")
		return nil
	}

	// 状态机：
	// - pending → 调用微信关单，关闭合单和子单
	// - closed  → 合单已关闭（上次可能只完成了部分），继续关闭待关的子单
	// - 其他    → 已由其他流程处理，幂等退出
	switch combined.Status {
	case "pending":
		// 继续向下执行微信关单 + 本地关闭
	case "closed":
		log.Info().
			Str("combine_out_trade_no", payload.CombineOutTradeNo).
			Msg("combined payment order already closed, checking sub-order states")
	default:
		log.Info().
			Str("combine_out_trade_no", payload.CombineOutTradeNo).
			Str("status", combined.Status).
			Msg("combined payment order in terminal state, skip timeout processing")
		return nil
	}

	combinedRow, err := p.store.GetCombinedPaymentOrderWithSubOrders(ctx, combined.ID)
	if err != nil {
		return fmt.Errorf("get combined payment sub orders: %w", err)
	}

	var subOrders []struct {
		SubMchID   string `json:"sub_mchid"`
		OutTradeNo string `json:"out_trade_no"`
	}
	if err := json.Unmarshal(combinedRow.SubOrders, &subOrders); err != nil {
		return fmt.Errorf("unmarshal combined sub orders: %w", err)
	}

	closeSubs := make([]wechatcontracts.SubOrderClose, 0, len(subOrders))
	for _, sub := range subOrders {
		if sub.SubMchID == "" || sub.OutTradeNo == "" {
			continue
		}
		closeSubs = append(closeSubs, wechatcontracts.SubOrderClose{
			SubMchID:   sub.SubMchID,
			OutTradeNo: sub.OutTradeNo,
		})
	}
	if len(closeSubs) == 0 {
		return fmt.Errorf("no sub orders to close")
	}

	queryResp, err := p.ecommerceClient.QueryCombineOrder(ctx, combined.CombineOutTradeNo)
	if err != nil {
		return fmt.Errorf("query combine order before timeout close: %w", err)
	}

	queryState := classifyCombinedPaymentQueryState(queryResp)
	switch queryState {
	case combinedPaymentQueryStatePaid:
		return p.reconcileRemotePaidCombinedPayment(ctx, combined, queryResp)
	case combinedPaymentQueryStatePartialPaid:
		return p.reconcileRemotePartialPaidCombinedPayment(ctx, combined, queryResp)
	case combinedPaymentQueryStatePending, combinedPaymentQueryStateClosed, combinedPaymentQueryStateFailed:
		// 继续走超时关单流程。
	case combinedPaymentQueryStateRefunded, combinedPaymentQueryStateMixed, combinedPaymentQueryStateUnknown:
		p.publishAlert(ctx, AlertData{
			AlertType:   AlertTypePaymentTimeout,
			Level:       AlertLevelCritical,
			Title:       "合单超时扫描遇到异常远端状态",
			Message:     fmt.Sprintf("合单 %s 超时扫描发现微信侧状态为 %s，系统已停止自动关单，请人工核对。", combined.CombineOutTradeNo, queryState),
			RelatedID:   combined.ID,
			RelatedType: "combined_payment_order",
			Extra: map[string]interface{}{
				"combine_out_trade_no": combined.CombineOutTradeNo,
				"remote_state":         queryState,
			},
		})
		return nil
	}

	// 只有 pending 状态才需要调用微信关单 API（closed 说明已经调用过了）
	if combined.Status == "pending" {
		if err := p.ecommerceClient.CloseCombineOrder(ctx, combined.CombineOutTradeNo, closeSubs); err != nil {
			return fmt.Errorf("close combine order: %w", err)
		}

		if _, err := p.store.UpdateCombinedPaymentOrderToClosed(ctx, combined.ID); err != nil {
			return fmt.Errorf("update combined payment order: %w", err)
		}
	}

	// 逐个关闭仍处于 pending 状态的子支付单（重试安全：已 closed 的子单会被跳过）
	for _, sub := range closeSubs {
		paymentOrder, err := p.store.GetPaymentOrderByOutTradeNo(ctx, sub.OutTradeNo)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				continue
			}
			return fmt.Errorf("get payment order: %w", err)
		}
		if paymentOrder.Status == "pending" {
			if _, err := p.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil {
				return fmt.Errorf("update payment order to closed: %w", err)
			}
		}
		// 取消关联的业务订单（与单笔支付超时逻辑保持一致）
		if paymentOrder.OrderID.Valid {
			order, err := p.store.GetOrderForUpdate(ctx, paymentOrder.OrderID.Int64)
			if err != nil {
				return fmt.Errorf("get order for combine timeout cancel: %w", err)
			}
			if order.Status == db.OrderStatusPending {
				if _, err = p.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
					OrderID:      order.ID,
					OldStatus:    order.Status,
					CancelReason: "支付超时未完成",
					OperatorID:   order.UserID,
					OperatorType: "system",
				}); err != nil {
					return fmt.Errorf("cancel order after combined payment timeout: %w", err)
				}
			}
		}
	}

	log.Info().
		Str("combine_out_trade_no", payload.CombineOutTradeNo).
		Msg("combined payment order timeout processed successfully")

	return nil
}

const (
	combinedPaymentQueryStatePending     = "pending"
	combinedPaymentQueryStatePaid        = "paid"
	combinedPaymentQueryStatePartialPaid = "partial"
	combinedPaymentQueryStateClosed      = "closed"
	combinedPaymentQueryStateFailed      = "failed"
	combinedPaymentQueryStateRefunded    = "refunded"
	combinedPaymentQueryStateMixed       = "mixed"
	combinedPaymentQueryStateUnknown     = "unknown"
)

func classifyCombinedPaymentQueryState(resp *wechatcontracts.CombineQueryResponse) string {
	if resp == nil || len(resp.SubOrders) == 0 {
		return combinedPaymentQueryStateUnknown
	}

	allSuccess := true
	allClosed := true
	allFailed := true
	allRefunded := true
	hasSuccess := false
	hasNotPay := false

	for _, subOrder := range resp.SubOrders {
		switch strings.ToUpper(strings.TrimSpace(subOrder.TradeState)) {
		case "SUCCESS":
			hasSuccess = true
			allClosed = false
			allFailed = false
			allRefunded = false
		case "NOTPAY":
			hasNotPay = true
			allSuccess = false
			allClosed = false
			allFailed = false
			allRefunded = false
		case "CLOSED":
			allSuccess = false
			allFailed = false
			allRefunded = false
		case "PAYERROR":
			allSuccess = false
			allClosed = false
			allRefunded = false
		case "REFUND":
			allSuccess = false
			allClosed = false
			allFailed = false
		default:
			allSuccess = false
			allClosed = false
			allFailed = false
			allRefunded = false
		}
	}

	switch {
	case allSuccess:
		return combinedPaymentQueryStatePaid
	case allClosed:
		return combinedPaymentQueryStateClosed
	case allFailed:
		return combinedPaymentQueryStateFailed
	case allRefunded:
		return combinedPaymentQueryStateRefunded
	case hasSuccess:
		return combinedPaymentQueryStatePartialPaid
	case hasNotPay:
		return combinedPaymentQueryStatePending
	default:
		return combinedPaymentQueryStateMixed
	}
}

func (p *RedisTaskProcessor) reconcileRemotePaidCombinedPayment(ctx context.Context, combined db.CombinedPaymentOrder, queryResp *wechatcontracts.CombineQueryResponse) error {
	result, err := p.reconcileRemoteSuccessfulCombinedSubOrders(ctx, combined, queryResp)
	if err != nil {
		return err
	}
	if result.exceptionalCount > 0 {
		p.publishAlert(ctx, AlertData{
			AlertType:   AlertTypePaymentTimeout,
			Level:       AlertLevelCritical,
			Title:       "合单远端已支付但本地存在异常子单",
			Message:     fmt.Sprintf("合单 %s 在超时扫描时确认微信侧已支付，但本地存在 %d 笔异常子单，已停止自动关单。", combined.CombineOutTradeNo, result.exceptionalCount),
			RelatedID:   combined.ID,
			RelatedType: "combined_payment_order",
			Extra: map[string]interface{}{
				"combine_out_trade_no": combined.CombineOutTradeNo,
				"success_count":        result.successCount,
				"exceptional_count":    result.exceptionalCount,
			},
		})
		return nil
	}

	if combined.Status == "closed" {
		p.publishAlert(ctx, AlertData{
			AlertType:   AlertTypePaymentTimeout,
			Level:       AlertLevelWarning,
			Title:       "合单超时后从远端已支付状态自动修复",
			Message:     fmt.Sprintf("合单 %s 在超时扫描时确认微信侧已支付，且子单已成功收敛；系统已将本地主合单从 %s 自动修复为 paid。", combined.CombineOutTradeNo, combined.Status),
			RelatedID:   combined.ID,
			RelatedType: "combined_payment_order",
			Extra: map[string]interface{}{
				"combine_out_trade_no": combined.CombineOutTradeNo,
				"local_status":         combined.Status,
				"reconciled_to":        "paid",
			},
		})
	}

	if _, err := p.store.UpdateCombinedPaymentOrderToPaid(ctx, db.UpdateCombinedPaymentOrderToPaidParams{
		ID:            combined.ID,
		TransactionID: pgtype.Text{Valid: false},
	}); err != nil {
		return fmt.Errorf("mark combined payment order paid after remote confirmation: %w", err)
	}

	log.Info().
		Str("combine_out_trade_no", combined.CombineOutTradeNo).
		Int("sub_order_count", result.successCount).
		Msg("combined payment timeout reconciled to paid from remote state")

	return nil
}

func (p *RedisTaskProcessor) reconcileRemotePartialPaidCombinedPayment(ctx context.Context, combined db.CombinedPaymentOrder, queryResp *wechatcontracts.CombineQueryResponse) error {
	result, err := p.reconcileRemoteSuccessfulCombinedSubOrders(ctx, combined, queryResp)
	if err != nil {
		return err
	}

	p.publishAlert(ctx, AlertData{
		AlertType:   AlertTypePaymentTimeout,
		Level:       AlertLevelCritical,
		Title:       "合单超时扫描发现部分子单已支付",
		Message:     fmt.Sprintf("合单 %s 在超时扫描时发现微信侧部分子单已支付，系统已停止自动关单并收敛已支付子单，请人工核对剩余子单。", combined.CombineOutTradeNo),
		RelatedID:   combined.ID,
		RelatedType: "combined_payment_order",
		Extra: map[string]interface{}{
			"combine_out_trade_no": combined.CombineOutTradeNo,
			"success_count":        result.successCount,
			"exceptional_count":    result.exceptionalCount,
		},
	})

	return nil
}

type combinedPaymentRemoteReconcileResult struct {
	successCount     int
	exceptionalCount int
}

func (p *RedisTaskProcessor) reconcileRemoteSuccessfulCombinedSubOrders(ctx context.Context, combined db.CombinedPaymentOrder, queryResp *wechatcontracts.CombineQueryResponse) (combinedPaymentRemoteReconcileResult, error) {
	var result combinedPaymentRemoteReconcileResult
	if queryResp == nil {
		return result, fmt.Errorf("query combine order response is nil")
	}

	for _, subOrder := range queryResp.SubOrders {
		if strings.ToUpper(strings.TrimSpace(subOrder.TradeState)) != "SUCCESS" {
			continue
		}

		paymentOrder, err := p.store.GetPaymentOrderByOutTradeNo(ctx, subOrder.OutTradeNo)
		if err != nil {
			return result, fmt.Errorf("get payment order by out_trade_no %s: %w", subOrder.OutTradeNo, err)
		}

		if subOrder.Amount.TotalAmount != paymentOrder.Amount {
			result.exceptionalCount++
			p.publishAlert(ctx, AlertData{
				AlertType:   AlertTypePaymentTimeout,
				Level:       AlertLevelCritical,
				Title:       "合单远端支付金额与本地不一致",
				Message:     fmt.Sprintf("合单 %s 子单 %s 在超时扫描时发现微信侧金额与本地不一致，已停止自动关单。", combined.CombineOutTradeNo, subOrder.OutTradeNo),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no": combined.CombineOutTradeNo,
					"out_trade_no":         subOrder.OutTradeNo,
					"expected_amount":      paymentOrder.Amount,
					"actual_amount":        subOrder.Amount.TotalAmount,
				},
			})
			continue
		}

		switch paymentOrder.Status {
		case "pending":
			paymentOrder, err = p.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
				ID:            paymentOrder.ID,
				TransactionID: pgtype.Text{String: subOrder.TransactionID, Valid: subOrder.TransactionID != ""},
			})
			if err != nil {
				return result, fmt.Errorf("mark payment order %d paid from remote timeout reconciliation: %w", paymentOrder.ID, err)
			}
			fallthrough
		case "paid":
			if !paymentOrder.ProcessedAt.Valid {
				if p.distributor == nil {
					return result, fmt.Errorf("task distributor not configured for remote paid combined payment reconciliation")
				}
				if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder {
					application, factErr := recordCombinedOrderPaymentQueryFact(ctx, p.store, combined, paymentOrder, subOrder)
					if factErr != nil {
						return result, fmt.Errorf("record combined payment fact for payment order %d: %w", paymentOrder.ID, factErr)
					}
					if err := enqueueOrderPaymentFactApplication(ctx, p.distributor, application); err != nil {
						return result, fmt.Errorf("enqueue order payment fact application for payment order %d: %w", paymentOrder.ID, err)
					}
				} else if shouldRecordReservationPaymentFactForOrder(paymentOrder) {
					application, factErr := recordCombinedReservationPaymentQueryFact(ctx, p.store, combined, paymentOrder, subOrder)
					if factErr != nil {
						return result, fmt.Errorf("record combined reservation payment fact for payment order %d: %w", paymentOrder.ID, factErr)
					}
					if err := enqueueReservationPaymentFactApplication(ctx, p.distributor, application); err != nil {
						return result, fmt.Errorf("enqueue reservation payment fact application for payment order %d: %w", paymentOrder.ID, err)
					}
				} else {
					return result, fmt.Errorf("payment order %d business type %q has no payment fact application target", paymentOrder.ID, paymentOrder.BusinessType)
				}
			}
			result.successCount++
		case "closed", "failed":
			result.exceptionalCount++
			if p.distributor == nil {
				return result, fmt.Errorf("task distributor not configured for anomaly refund reconciliation")
			}
			if err := p.distributor.DistributeTaskProcessAnomalyRefund(ctx, &PayloadProcessAnomalyRefund{
				PaymentOrderID: paymentOrder.ID,
				TransactionID:  subOrder.TransactionID,
				RefundAmount:   paymentOrder.Amount,
				OutRefundNo:    fmt.Sprintf("CRF%d", paymentOrder.ID),
			}, asynq.MaxRetry(5), asynq.Queue(QueueCritical)); err != nil {
				return result, fmt.Errorf("enqueue anomaly refund for payment order %d: %w", paymentOrder.ID, err)
			}
			p.publishAlert(ctx, AlertData{
				AlertType:   AlertTypePaymentTimeout,
				Level:       AlertLevelCritical,
				Title:       "合单远端已支付但本地子单已关闭",
				Message:     fmt.Sprintf("合单 %s 子单 %s 在超时扫描时发现微信侧已支付，但本地状态为 %s，已入队异常退款。", combined.CombineOutTradeNo, subOrder.OutTradeNo, paymentOrder.Status),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no": combined.CombineOutTradeNo,
					"out_trade_no":         subOrder.OutTradeNo,
					"local_status":         paymentOrder.Status,
					"transaction_id":       subOrder.TransactionID,
				},
			})
		default:
			result.exceptionalCount++
			p.publishAlert(ctx, AlertData{
				AlertType:   AlertTypePaymentTimeout,
				Level:       AlertLevelCritical,
				Title:       "合单远端已支付但本地子单状态异常",
				Message:     fmt.Sprintf("合单 %s 子单 %s 在超时扫描时发现微信侧已支付，但本地状态为 %s。", combined.CombineOutTradeNo, subOrder.OutTradeNo, paymentOrder.Status),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no": combined.CombineOutTradeNo,
					"out_trade_no":         subOrder.OutTradeNo,
					"local_status":         paymentOrder.Status,
				},
			})
		}
	}

	return result, nil
}
