package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	TaskPaymentOrderTimeout = "payment_order:timeout"
)

// PayloadPaymentOrderTimeout 支付订单超时任务载荷
type PayloadPaymentOrderTimeout struct {
	PaymentOrderNo string `json:"payment_order_no"`
}

// DistributeTaskPaymentOrderTimeout 分发支付订单超时任务
func (d *RedisTaskDistributor) DistributeTaskPaymentOrderTimeout(
	ctx context.Context,
	payload *PayloadPaymentOrderTimeout,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskPaymentOrderTimeout, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("enqueued payment order timeout task")

	return nil
}

// ProcessTaskPaymentOrderTimeout 处理支付订单超时任务
func (p *RedisTaskProcessor) ProcessTaskPaymentOrderTimeout(ctx context.Context, task *asynq.Task) error {
	var payload PayloadPaymentOrderTimeout
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("type", task.Type()).
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("processing payment order timeout task")

	// 获取支付订单
	paymentOrder, err := p.store.GetPaymentOrderByOutTradeNo(ctx, payload.PaymentOrderNo)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	// 检查是否已超时（在关闭之前先检查，避免尚未到期就被错误触发）
	if paymentOrder.ExpiresAt.Valid && time.Now().Before(paymentOrder.ExpiresAt.Time) {
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Time("expire_time", paymentOrder.ExpiresAt.Time).
			Msg("payment order not expired yet")
		return nil
	}

	// 状态机：
	// - pending  → 关闭支付单，然后取消业务订单
	// - closed   → 支付单已关闭（可能是上次失败后重试），直接检查业务订单并继续
	// - 其他状态 → 已由其他流程处理，幂等退出
	switch paymentOrder.Status {
	case "pending":
		if _, err := p.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil {
			return fmt.Errorf("update payment order to closed: %w", err)
		}
	case "closed":
		// 支付单已经关闭，可能是上次任务执行到一半后失败重试进来的
		// 继续向下检查业务订单是否也已取消
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Msg("payment order already closed, checking business order state")
	default:
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Str("status", paymentOrder.Status).
			Msg("payment order in terminal state, skip timeout processing")
		return nil
	}

	// 取消关联的业务订单（无论是本次刚关闭还是上次已关闭）
	if paymentOrder.OrderID.Valid {
		order, err := p.store.GetOrderForUpdate(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return fmt.Errorf("get order for timeout cancel: %w", err)
		}

		if order.Status == db.OrderStatusPending {
			_, err = p.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
				OrderID:      order.ID,
				OldStatus:    order.Status,
				CancelReason: "支付超时未完成",
				OperatorID:   order.UserID,
				OperatorType: "system",
			})
			if err != nil {
				return fmt.Errorf("cancel order after payment timeout: %w", err)
			}
		} else {
			log.Info().
				Int64("order_id", order.ID).
				Str("status", order.Status).
				Msg("order already moved past pending, skip timeout cancel")
		}
	}

	log.Info().
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("payment order timeout processed successfully")

	return nil
}
