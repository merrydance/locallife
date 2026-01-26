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
	// TaskOrderPaymentTimeout 订单支付超时任务
	TaskOrderPaymentTimeout = "order:payment_timeout"

	// OrderPaymentTimeoutMinutes 订单支付超时时间（分钟）
	OrderPaymentTimeoutMinutes = 30
)

// PayloadOrderPaymentTimeout 订单支付超时任务载荷
type PayloadOrderPaymentTimeout struct {
	OrderID int64 `json:"order_id"`
}

// DistributeTaskOrderPaymentTimeout 分发订单支付超时任务
func (d *RedisTaskDistributor) DistributeTaskOrderPaymentTimeout(
	ctx context.Context,
	payload *PayloadOrderPaymentTimeout,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskOrderPaymentTimeout, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("order_id", payload.OrderID).
		Msg("enqueued order payment timeout task")

	return nil
}

// ProcessTaskOrderPaymentTimeout 处理订单支付超时任务
func (p *RedisTaskProcessor) ProcessTaskOrderPaymentTimeout(ctx context.Context, task *asynq.Task) error {
	var payload PayloadOrderPaymentTimeout
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("type", task.Type()).
		Int64("order_id", payload.OrderID).
		Msg("processing order payment timeout task")

	// 获取订单（加锁）
	order, err := p.store.GetOrderForUpdate(ctx, payload.OrderID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Warn().Int64("order_id", payload.OrderID).Msg("order not found, skip timeout processing")
			return nil
		}
		return fmt.Errorf("get order: %w", err)
	}

	// 只处理待支付状态的订单
	if order.Status != db.OrderStatusPending {
		log.Info().
			Int64("order_id", payload.OrderID).
			Str("status", order.Status).
			Msg("order is not pending, skip timeout processing")
		return nil
	}

	// 检查订单是否确实已超时（创建时间 + 超时时间 < 当前时间）
	timeoutAt := order.CreatedAt.Add(OrderPaymentTimeoutMinutes * time.Minute)
	if time.Now().Before(timeoutAt) {
		log.Info().
			Int64("order_id", payload.OrderID).
			Time("timeout_at", timeoutAt).
			Msg("order not yet timed out")
		return nil
	}

	// 取消订单
	_, err = p.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
		OrderID:      order.ID,
		OldStatus:    order.Status,
		CancelReason: "支付超时自动取消",
		OperatorID:   order.UserID,
		OperatorType: "system",
	})
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	log.Info().
		Int64("order_id", payload.OrderID).
		Str("order_no", order.OrderNo).
		Msg("order payment timeout, cancelled successfully")

	return nil
}
