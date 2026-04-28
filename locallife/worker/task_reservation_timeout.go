package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

const (
	TaskReservationPaymentTimeout  = "reservation:payment_timeout"
	TaskReservationNoShowAlert     = "reservation:no_show_alert"
	TaskReservationFoodSafetyAlert = "reservation:food_safety_alert"
)

// PayloadReservationNoShowAlert 预定未到店提醒任务载荷
type PayloadReservationNoShowAlert struct {
	ReservationID int64 `json:"reservation_id"`
}

// PayloadReservationPaymentTimeout 预定支付超时任务载荷
type PayloadReservationPaymentTimeout struct {
	ReservationID int64 `json:"reservation_id"`
}

// PayloadReservationFoodSafetyAlert 食安停业预订提醒任务载荷
type PayloadReservationFoodSafetyAlert struct {
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
	info, err := d.enqueueTask(ctx, task)
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

// DistributeTaskReservationNoShowAlert 分发预定未到店提醒任务
func (d *RedisTaskDistributor) DistributeTaskReservationNoShowAlert(
	ctx context.Context,
	payload *PayloadReservationNoShowAlert,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskReservationNoShowAlert, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("reservation_id", payload.ReservationID).
		Msg("enqueued reservation no-show alert task")

	return nil
}

// DistributeTaskReservationFoodSafetyAlert 分发食安停业预订提醒任务
func (d *RedisTaskDistributor) DistributeTaskReservationFoodSafetyAlert(
	ctx context.Context,
	payload *PayloadReservationFoodSafetyAlert,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskReservationFoodSafetyAlert, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("reservation_id", payload.ReservationID).
		Msg("enqueued reservation food safety alert task")

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

	if err := p.store.ReleaseReservationInventoryTx(ctx, db.ReleaseReservationInventoryTxParams{
		ReservationID: reservation.ID,
	}); err != nil {
		return fmt.Errorf("release reservation inventory: %w", err)
	}

	log.Info().
		Int64("reservation_id", payload.ReservationID).
		Msg("reservation payment timeout processed successfully")

	return nil
}

// ProcessTaskReservationNoShowAlert 处理预定未到店提醒任务
func (p *RedisTaskProcessor) ProcessTaskReservationNoShowAlert(ctx context.Context, task *asynq.Task) error {
	var payload PayloadReservationNoShowAlert
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	// 1. 获取预定详情
	reservation, err := p.store.GetTableReservationWithTable(ctx, payload.ReservationID)
	if err != nil {
		return fmt.Errorf("get reservation: %w", err)
	}

	// 2. 状态检查：只有“已支付”或“商户已确认”状态，且未签到的预订才需要发送提醒
	if reservation.Status != "paid" && reservation.Status != "confirmed" {
		log.Info().
			Int64("reservation_id", payload.ReservationID).
			Str("status", reservation.Status).
			Msg("reservation status changed or already checked in, skip no-show alert")
		return nil
	}

	// 3. 构建通知内容
	// 格式化时间为 HH:MM
	var arrivalTimeStr string
	if reservation.ReservationTime.Valid {
		hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
		arrivalTimeStr = fmt.Sprintf("%02d:%02d", hours, minutes)
	}

	data, _ := json.Marshal(map[string]any{
		"reservation_id":   reservation.ID,
		"table_no":         reservation.TableNo,
		"arrival_time":     arrivalTimeStr,
		"reservation_date": reservation.ReservationDate.Time.Format("2006-01-02"),
		"contact_name":     reservation.ContactName,
		"contact_phone":    reservation.ContactPhone,
	})

	// 4. 发送 WebSocket 推送给商户（通过 Pub/Sub 发布器）
	pushMsg := websocket.NotificationPushMessage{
		EntityType: "merchant",
		EntityID:   reservation.MerchantID,
		Message: websocket.Message{
			Type:      "reservation_no_show_alert",
			Data:      json.RawMessage(data),
			Timestamp: time.Now(),
		},
	}
	payloadBytes, _ := json.Marshal(pushMsg)
	channel := fmt.Sprintf("notification:merchant:%d", reservation.MerchantID)
	p.publishWSMessage(ctx, channel, payloadBytes)
	log.Info().Int64("reservation_id", payload.ReservationID).Msg("reservation no-show alert publish attempted")
	return nil
}

// ProcessTaskReservationFoodSafetyAlert 处理食安停业预订提醒任务
func (p *RedisTaskProcessor) ProcessTaskReservationFoodSafetyAlert(ctx context.Context, task *asynq.Task) error {
	var payload PayloadReservationFoodSafetyAlert
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	reservation, err := p.store.GetTableReservation(ctx, payload.ReservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("get reservation: %w", err)
	}

	if reservation.Status != "pending" && reservation.Status != "paid" && reservation.Status != "confirmed" {
		log.Info().
			Int64("reservation_id", reservation.ID).
			Str("status", reservation.Status).
			Msg("reservation status changed, skip food safety alert")
		return nil
	}

	merchantProfile, err := p.store.GetMerchantProfile(ctx, reservation.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant profile: %w", err)
	}
	if !merchantProfile.IsSuspended || !db.IsFoodSafetySuspendReason(merchantProfile.SuspendReason.String) {
		log.Info().
			Int64("reservation_id", reservation.ID).
			Int64("merchant_id", reservation.MerchantID).
			Msg("merchant is no longer food safety suspended, skip reservation alert")
		return nil
	}

	merchant, err := p.store.GetMerchant(ctx, reservation.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant: %w", err)
	}

	reservationTime := time.Date(
		reservation.ReservationDate.Time.Year(), reservation.ReservationDate.Time.Month(), reservation.ReservationDate.Time.Day(),
		int(reservation.ReservationTime.Microseconds/1000000/3600), int((reservation.ReservationTime.Microseconds/1000000%3600)/60), 0, 0, time.Local,
	)

	if err := p.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:            reservation.UserID,
		Type:              "food_safety",
		Title:             "预订提醒：商户仍处于食安停业状态",
		Content:           fmt.Sprintf("您在%s的预订商户“%s”目前仍处于食安停业状态。如不再等待恢复营业，请主动退订并按规则申请退款，平台不会代您自动退款。", reservationTime.Format("2006-01-02 15:04"), merchant.Name),
		RelatedType:       "reservation",
		RelatedID:         reservation.ID,
		IgnorePreferences: true,
		ExtraData: map[string]any{
			"reservation_id":   reservation.ID,
			"merchant_id":      reservation.MerchantID,
			"merchant_name":    merchant.Name,
			"reservation_time": reservationTime.Format(time.RFC3339),
			"scene":            "food_safety_suspension_reminder",
		},
	}); err != nil {
		return fmt.Errorf("enqueue reservation food safety notification: %w", err)
	}

	log.Info().Int64("reservation_id", reservation.ID).Msg("reservation food safety alert enqueued")
	return nil
}
