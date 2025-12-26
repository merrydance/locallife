package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	TaskReservationPaymentTimeout = "reservation:payment_timeout"
)

// PayloadReservationPaymentTimeout 预定支付超时任务载荷
type PayloadReservationPaymentTimeout struct {
	ReservationID int64 `json:"reservation_id"`
}

// DistributeTaskReservationPaymentTimeout 分发预定支付超时任务
func (d *RedisTaskDistributor) DistributeTaskReservationPaymentTimeout(
	ctx context.Context,
	payload *PayloadReservationPaymentTimeout,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskReservationPaymentTimeout, jsonPayload, opts...)
	info, err := d.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Int64("reservation_id", payload.ReservationID).
		Msg("enqueued reservation payment timeout task")

	return nil
}

// ProcessTaskReservationPaymentTimeout 处理预定支付超时任务
func (p *RedisTaskProcessor) ProcessTaskReservationPaymentTimeout(ctx context.Context, task *asynq.Task) error {
	var payload PayloadReservationPaymentTimeout
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("type", task.Type()).
		Int64("reservation_id", payload.ReservationID).
		Msg("processing reservation payment timeout task")

	// 获取预定信息
	reservation, err := p.store.GetTableReservation(ctx, payload.ReservationID)
	if err != nil {
		return fmt.Errorf("get reservation: %w", err)
	}

	// 只处理待支付状态的预定（pending 为创建后待支付）
	if reservation.Status != "pending" {
		log.Info().
			Int64("reservation_id", payload.ReservationID).
			Str("status", reservation.Status).
			Msg("reservation is not pending payment, skip timeout processing")
		return nil
	}

	// 检查是否已超时（支付截止时间）
	if time.Now().Before(reservation.PaymentDeadline) {
		log.Info().
			Int64("reservation_id", payload.ReservationID).
			Time("payment_deadline", reservation.PaymentDeadline).
			Msg("reservation payment not expired yet")
		return nil
	}

	// 更新预定状态为已取消（超时未支付）
	_, err = p.store.UpdateReservationToCancelled(ctx, db.UpdateReservationToCancelledParams{
		ID:           reservation.ID,
		CancelReason: pgtype.Text{String: "payment timeout", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update reservation status: %w", err)
	}

	log.Info().
		Int64("reservation_id", payload.ReservationID).
		Msg("reservation payment timeout processed successfully")

	return nil
}
