package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
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
	info, err := d.client.EnqueueContext(ctx, task)
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

	// 只处理未支付状态的订单
	if paymentOrder.Status != "pending" {
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Str("status", paymentOrder.Status).
			Msg("payment order is not pending, skip timeout processing")
		return nil
	}

	// 检查是否已超时
	if paymentOrder.ExpiresAt.Valid && time.Now().Before(paymentOrder.ExpiresAt.Time) {
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Time("expire_time", paymentOrder.ExpiresAt.Time).
			Msg("payment order not expired yet")
		return nil
	}

	// 更新支付订单状态为超时关闭
	_, err = p.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return fmt.Errorf("update payment order status: %w", err)
	}

	// 如果有微信支付 prepay_id，需要调用微信关单 API
	// 注意：这里需要注入 PaymentClient，或者通过其他方式调用
	// 由于关单操作可以延后，这里只更新数据库状态
	// 微信支付订单会在超时后自动关闭

	log.Info().
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("payment order timeout processed successfully")

	return nil
}
