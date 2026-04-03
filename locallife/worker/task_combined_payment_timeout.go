package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
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

	closeSubs := make([]wechat.SubOrderClose, 0, len(subOrders))
	for _, sub := range subOrders {
		if sub.SubMchID == "" || sub.OutTradeNo == "" {
			continue
		}
		closeSubs = append(closeSubs, wechat.SubOrderClose{
			SubMchID:   sub.SubMchID,
			OutTradeNo: sub.OutTradeNo,
		})
	}
	if len(closeSubs) == 0 {
		return fmt.Errorf("no sub orders to close")
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
