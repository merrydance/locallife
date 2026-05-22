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
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessPaymentDomainOutbox          = "payment:process_domain_outbox"
	PaymentDomainOutboxEventDispatcherProbe = "payment_domain_outbox_dispatcher_probe"
	paymentDomainOutboxRetryDelay           = 5 * time.Minute
	reservationNoShowAlertDedupWindow       = 24 * time.Hour
)

type PaymentDomainOutboxTaskDistributor interface {
	DistributeTaskProcessPaymentDomainOutbox(ctx context.Context, payload *PaymentDomainOutboxPayload, opts ...asynq.Option) error
}

type PaymentDomainOutboxPayload struct {
	OutboxID int64 `json:"outbox_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessPaymentDomainOutbox(
	ctx context.Context,
	payload *PaymentDomainOutboxPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payment domain outbox payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessPaymentDomainOutbox, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue payment domain outbox task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_domain_outbox_id", payload.OutboxID).
		Msg("enqueued payment domain outbox task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskPaymentDomainOutbox(ctx context.Context, task *asynq.Task) error {
	var payload PaymentDomainOutboxPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payment domain outbox payload: %w", asynq.SkipRetry)
	}
	if payload.OutboxID == 0 {
		return fmt.Errorf("payment domain outbox id is required: %w", asynq.SkipRetry)
	}

	now := time.Now().UTC()
	outbox, err := processor.store.ClaimPaymentDomainOutbox(ctx, db.ClaimPaymentDomainOutboxParams{
		ID:    payload.OutboxID,
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if errors.Is(err, db.ErrRecordNotFound) {
		log.Info().Int64("payment_domain_outbox_id", payload.OutboxID).Msg("payment domain outbox already claimed or not retryable")
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim payment domain outbox: %w", err)
	}

	if err := processor.dispatchPaymentDomainOutbox(ctx, outbox); err != nil {
		return processor.markPaymentDomainOutboxFailed(ctx, outbox, err)
	}

	if _, err := processor.store.MarkPaymentDomainOutboxPublished(ctx, outbox.ID); err != nil {
		return processor.markPaymentDomainOutboxFailed(ctx, outbox, fmt.Errorf("mark payment domain outbox published: %w", err))
	}

	log.Info().
		Int64("payment_domain_outbox_id", outbox.ID).
		Str("event_type", outbox.EventType).
		Str("aggregate_type", outbox.AggregateType).
		Int64("aggregate_id", outbox.AggregateID).
		Msg("payment domain outbox published")
	return nil
}

func (processor *RedisTaskProcessor) dispatchPaymentDomainOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	switch outbox.EventType {
	case PaymentDomainOutboxEventDispatcherProbe:
		return nil
	case db.PaymentDomainOutboxEventOrderPaymentSucceeded:
		return processor.dispatchOrderPaymentSucceededOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventReservationPaymentSucceeded:
		return processor.dispatchReservationPaymentSucceededOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventProfitSharingResultReady:
		return processor.dispatchProfitSharingResultReadyOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventOrderRefundSucceeded:
		return processor.dispatchOrderRefundSucceededOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventOrderRefundAbnormal:
		return processor.dispatchOrderRefundAbnormalOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventReservationRefundAbnormal:
		return processor.dispatchReservationRefundAbnormalOutbox(ctx, outbox)
	case db.PaymentDomainOutboxEventRiderDepositRefundAbnormal:
		return processor.dispatchRiderDepositRefundAbnormalOutbox(ctx, outbox)
	default:
		return fmt.Errorf("unsupported payment domain outbox event type %q", outbox.EventType)
	}
}

func (processor *RedisTaskProcessor) dispatchOrderPaymentSucceededOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregatePaymentOrder {
		return fmt.Errorf("unsupported order payment outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		PaymentOrderID           int64  `json:"payment_order_id"`
		OrderID                  int64  `json:"order_id"`
		MerchantID               int64  `json:"merchant_id"`
		OrderNo                  string `json:"order_no"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal order payment outbox payload: %w", err)
	}
	if payload.PaymentOrderID == 0 {
		payload.PaymentOrderID = outbox.AggregateID
	}
	if payload.PaymentOrderID != outbox.AggregateID {
		return fmt.Errorf("order payment outbox aggregate id %d does not match payload payment order id %d", outbox.AggregateID, payload.PaymentOrderID)
	}

	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment order %d business type %q is not order payment", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if !paymentOrder.OrderID.Valid {
		return fmt.Errorf("payment order %d has no order id", paymentOrder.ID)
	}
	if payload.OrderID != 0 && payload.OrderID != paymentOrder.OrderID.Int64 {
		return fmt.Errorf("order payment outbox payload order id %d does not match payment order order id %d", payload.OrderID, paymentOrder.OrderID.Int64)
	}

	order, err := processor.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if payload.MerchantID != 0 && payload.MerchantID != order.MerchantID {
		return fmt.Errorf("order payment outbox payload merchant id %d does not match order merchant id %d", payload.MerchantID, order.MerchantID)
	}

	if processor.distributor == nil {
		return fmt.Errorf("task distributor not configured")
	}

	result, err := processor.loadOrderPaymentNotificationResult(ctx, order)
	if err != nil {
		return err
	}
	if err := processor.sendOrderPaidNotifications(ctx, result); err != nil {
		return err
	}
	return nil
}

func (processor *RedisTaskProcessor) dispatchReservationPaymentSucceededOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregatePaymentOrder {
		return fmt.Errorf("unsupported reservation payment outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		PaymentOrderID           int64  `json:"payment_order_id"`
		ReservationID            int64  `json:"reservation_id"`
		BusinessType             string `json:"business_type"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal reservation payment outbox payload: %w", err)
	}
	if payload.PaymentOrderID == 0 {
		payload.PaymentOrderID = outbox.AggregateID
	}
	if payload.PaymentOrderID != outbox.AggregateID {
		return fmt.Errorf("reservation payment outbox aggregate id %d does not match payload payment order id %d", outbox.AggregateID, payload.PaymentOrderID)
	}

	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerReservation && paymentOrder.BusinessType != reservationPaymentAddonBusinessType {
		return fmt.Errorf("payment order %d business type %q is not reservation payment", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if !paymentOrder.ReservationID.Valid {
		return fmt.Errorf("payment order %d has no reservation id", paymentOrder.ID)
	}
	if payload.ReservationID != 0 && payload.ReservationID != paymentOrder.ReservationID.Int64 {
		return fmt.Errorf("reservation payment outbox payload reservation id %d does not match payment order reservation id %d", payload.ReservationID, paymentOrder.ReservationID.Int64)
	}
	if processor.distributor == nil {
		return fmt.Errorf("task distributor not configured")
	}

	reservation, err := processor.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
	if err != nil {
		return fmt.Errorf("get reservation: %w", err)
	}
	hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
	minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
	alertTime := time.Date(
		reservation.ReservationDate.Time.Year(), reservation.ReservationDate.Time.Month(), reservation.ReservationDate.Time.Day(),
		int(hours), int(minutes), 0, 0, time.Local,
	)
	if err := processor.distributor.DistributeTaskReservationNoShowAlert(
		ctx,
		&PayloadReservationNoShowAlert{ReservationID: reservation.ID},
		asynq.ProcessAt(alertTime),
		asynq.Unique(reservationNoShowAlertDedupWindow),
	); err != nil {
		return fmt.Errorf("enqueue reservation no-show alert after reservation payment outbox: %w", err)
	}
	return nil
}

func (processor *RedisTaskProcessor) loadOrderPaymentNotificationResult(ctx context.Context, order db.Order) (db.ProcessOrderPaymentTxResult, error) {
	result := db.ProcessOrderPaymentTxResult{Order: order}
	if order.OrderType != "takeout" {
		return result, nil
	}

	delivery, err := processor.store.GetDeliveryByOrderID(ctx, order.ID)
	if err == nil {
		result.Delivery = &delivery
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return result, fmt.Errorf("get delivery by order id: %w", err)
	}

	poolItem, err := processor.store.GetDeliveryPoolByOrderID(ctx, order.ID)
	if err == nil {
		result.PoolItem = &poolItem
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return result, fmt.Errorf("get delivery pool by order id: %w", err)
	}

	return result, nil
}

func (processor *RedisTaskProcessor) dispatchProfitSharingResultReadyOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregateProfitSharingOrder {
		return fmt.Errorf("unsupported profit sharing result outbox aggregate type %q", outbox.AggregateType)
	}

	var payload ProfitSharingResultPayload
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal profit sharing result outbox payload: %w", err)
	}
	if payload.ProfitSharingOrderID == 0 {
		payload.ProfitSharingOrderID = outbox.AggregateID
	}
	if payload.ProfitSharingOrderID != outbox.AggregateID {
		return fmt.Errorf("profit sharing result outbox aggregate id %d does not match payload order id %d", outbox.AggregateID, payload.ProfitSharingOrderID)
	}
	if payload.OutOrderNo == "" {
		return fmt.Errorf("profit sharing result outbox payload out_order_no is required")
	}
	if payload.Result == "" {
		return fmt.Errorf("profit sharing result outbox payload result is required")
	}
	if payload.MerchantID == 0 {
		return fmt.Errorf("profit sharing result outbox payload merchant_id is required")
	}

	return processor.processProfitSharingResultPayload(ctx, payload, true)
}

func (processor *RedisTaskProcessor) dispatchRiderDepositRefundAbnormalOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregateRefundOrder {
		return fmt.Errorf("unsupported rider deposit refund outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		RefundOrderID            int64  `json:"refund_order_id"`
		PaymentOrderID           int64  `json:"payment_order_id"`
		OutRefundNo              string `json:"out_refund_no"`
		RefundStatus             string `json:"refund_status"`
		RefundID                 string `json:"refund_id"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal rider deposit refund abnormal outbox payload: %w", err)
	}
	if payload.RefundOrderID == 0 {
		payload.RefundOrderID = outbox.AggregateID
	}
	if payload.RefundOrderID != outbox.AggregateID {
		return fmt.Errorf("rider deposit refund outbox aggregate id %d does not match payload refund order id %d", outbox.AggregateID, payload.RefundOrderID)
	}
	if payload.RefundStatus != "ABNORMAL" {
		return fmt.Errorf("rider deposit refund outbox payload refund_status %q is not abnormal", payload.RefundStatus)
	}

	refundOrder, err := processor.store.GetRefundOrder(ctx, payload.RefundOrderID)
	if err != nil {
		return fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerRiderDeposit {
		return fmt.Errorf("payment order %d business type %q is not rider deposit", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if payload.PaymentOrderID != 0 && payload.PaymentOrderID != paymentOrder.ID {
		return fmt.Errorf("rider deposit refund outbox payload payment order id %d does not match refund payment order id %d", payload.PaymentOrderID, paymentOrder.ID)
	}

	refundID := payload.RefundID
	if refundID == "" && refundOrder.RefundID.Valid {
		refundID = refundOrder.RefundID.String
	}
	alertExtra := refundOrderAlertExtra(paymentOrder, refundOrder, 0, map[string]interface{}{
		"refund_id":                   refundID,
		"external_payment_fact_id":    payload.ExternalPaymentFactID,
		"payment_fact_application_id": payload.PaymentFactApplicationID,
	})
	processor.publishAlert(ctx, AlertData{
		AlertType:   AlertTypeRefundFailed,
		Level:       AlertLevelCritical,
		Title:       "退款异常 - 需人工介入",
		Message:     fmt.Sprintf("骑手押金退款单 %s 状态异常(ABNORMAL)，微信退款ID: %s，请及时处理", refundOrder.OutRefundNo, refundID),
		RelatedID:   refundOrder.ID,
		RelatedType: "refund_order",
		Extra:       alertExtra,
	})
	return nil
}

func (processor *RedisTaskProcessor) dispatchReservationRefundAbnormalOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregateRefundOrder {
		return fmt.Errorf("unsupported reservation refund outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		RefundOrderID            int64  `json:"refund_order_id"`
		PaymentOrderID           int64  `json:"payment_order_id"`
		ReservationID            int64  `json:"reservation_id"`
		OutRefundNo              string `json:"out_refund_no"`
		RefundStatus             string `json:"refund_status"`
		RefundID                 string `json:"refund_id"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal reservation refund abnormal outbox payload: %w", err)
	}
	if payload.RefundOrderID == 0 {
		payload.RefundOrderID = outbox.AggregateID
	}
	if payload.RefundOrderID != outbox.AggregateID {
		return fmt.Errorf("reservation refund outbox aggregate id %d does not match payload refund order id %d", outbox.AggregateID, payload.RefundOrderID)
	}
	if payload.RefundStatus != "ABNORMAL" {
		return fmt.Errorf("reservation refund outbox payload refund_status %q is not abnormal", payload.RefundStatus)
	}

	refundOrder, err := processor.store.GetRefundOrder(ctx, payload.RefundOrderID)
	if err != nil {
		return fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if !isReservationRefundPayment(paymentOrder) {
		return fmt.Errorf("payment order %d business type %q is not reservation refund", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if payload.PaymentOrderID != 0 && payload.PaymentOrderID != paymentOrder.ID {
		return fmt.Errorf("reservation refund outbox payload payment order id %d does not match refund payment order id %d", payload.PaymentOrderID, paymentOrder.ID)
	}
	if payload.ReservationID != 0 && (!paymentOrder.ReservationID.Valid || payload.ReservationID != paymentOrder.ReservationID.Int64) {
		return fmt.Errorf("reservation refund outbox payload reservation id %d does not match payment order reservation id", payload.ReservationID)
	}

	merchantID, err := processor.resolveMerchantIDByPaymentOrder(ctx, paymentOrder)
	if err != nil {
		return fmt.Errorf("resolve reservation refund merchant: %w", err)
	}
	refundID := payload.RefundID
	if refundID == "" && refundOrder.RefundID.Valid {
		refundID = refundOrder.RefundID.String
	}
	alertExtra := refundOrderAlertExtra(paymentOrder, refundOrder, merchantID, map[string]interface{}{
		"refund_id":                   refundID,
		"external_payment_fact_id":    payload.ExternalPaymentFactID,
		"payment_fact_application_id": payload.PaymentFactApplicationID,
	})
	processor.publishAlert(ctx, AlertData{
		AlertType:   AlertTypeRefundFailed,
		Level:       AlertLevelCritical,
		Title:       "预订退款异常 - 需人工介入",
		Message:     fmt.Sprintf("预订退款单 %s 状态异常(ABNORMAL)，微信退款ID: %s，请及时处理", refundOrder.OutRefundNo, refundID),
		RelatedID:   refundOrder.ID,
		RelatedType: "refund_order",
		Extra:       alertExtra,
	})
	return nil
}

func (processor *RedisTaskProcessor) dispatchOrderRefundSucceededOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregateRefundOrder {
		return fmt.Errorf("unsupported order refund success outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		RefundOrderID            int64  `json:"refund_order_id"`
		PaymentOrderID           int64  `json:"payment_order_id"`
		OrderID                  int64  `json:"order_id"`
		UserID                   int64  `json:"user_id"`
		OutRefundNo              string `json:"out_refund_no"`
		RefundAmount             int64  `json:"refund_amount"`
		RefundStatus             string `json:"refund_status"`
		RefundID                 string `json:"refund_id"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal order refund success outbox payload: %w", err)
	}
	if payload.RefundOrderID == 0 {
		payload.RefundOrderID = outbox.AggregateID
	}
	if payload.RefundOrderID != outbox.AggregateID {
		return fmt.Errorf("order refund success outbox aggregate id %d does not match payload refund order id %d", outbox.AggregateID, payload.RefundOrderID)
	}
	if payload.RefundStatus != "SUCCESS" {
		return fmt.Errorf("order refund success outbox payload refund_status %q is not success", payload.RefundStatus)
	}

	refundOrder, err := processor.store.GetRefundOrder(ctx, payload.RefundOrderID)
	if err != nil {
		return fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if !paymentOrder.OrderID.Valid || paymentOrder.ReservationID.Valid || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment order %d business type %q is not order refund", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if payload.PaymentOrderID != 0 && payload.PaymentOrderID != paymentOrder.ID {
		return fmt.Errorf("order refund success outbox payload payment order id %d does not match refund payment order id %d", payload.PaymentOrderID, paymentOrder.ID)
	}
	if payload.OrderID != 0 && payload.OrderID != paymentOrder.OrderID.Int64 {
		return fmt.Errorf("order refund success outbox payload order id %d does not match payment order order id %d", payload.OrderID, paymentOrder.OrderID.Int64)
	}
	userID := payload.UserID
	if userID == 0 {
		userID = paymentOrder.UserID
	}
	if userID == 0 {
		return fmt.Errorf("order refund success outbox user_id is required")
	}
	if processor.distributor == nil {
		return fmt.Errorf("task distributor not configured")
	}
	refundID := payload.RefundID
	if refundID == "" && refundOrder.RefundID.Valid {
		refundID = refundOrder.RefundID.String
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	return processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:      userID,
		Type:        "refund",
		Title:       "退款成功",
		Content:     fmt.Sprintf("您的订单退款已完成，退款金额%.2f元", float64(refundOrder.RefundAmount)/100),
		RelatedType: "refund",
		RelatedID:   refundOrder.ID,
		ExtraData: map[string]any{
			"out_refund_no":               refundOrder.OutRefundNo,
			"refund_id":                   refundID,
			"amount":                      refundOrder.RefundAmount,
			"external_payment_fact_id":    payload.ExternalPaymentFactID,
			"payment_fact_application_id": payload.PaymentFactApplicationID,
		},
		ExpiresAt: &expiresAt,
	}, asynq.Queue(QueueDefault))
}

func (processor *RedisTaskProcessor) dispatchOrderRefundAbnormalOutbox(ctx context.Context, outbox db.PaymentDomainOutbox) error {
	if outbox.AggregateType != db.PaymentDomainOutboxAggregateRefundOrder {
		return fmt.Errorf("unsupported order refund abnormal outbox aggregate type %q", outbox.AggregateType)
	}

	var payload struct {
		RefundOrderID            int64  `json:"refund_order_id"`
		PaymentOrderID           int64  `json:"payment_order_id"`
		OrderID                  int64  `json:"order_id"`
		OutRefundNo              string `json:"out_refund_no"`
		RefundStatus             string `json:"refund_status"`
		RefundID                 string `json:"refund_id"`
		ExternalPaymentFactID    int64  `json:"external_payment_fact_id"`
		PaymentFactApplicationID int64  `json:"payment_fact_application_id"`
	}
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal order refund abnormal outbox payload: %w", err)
	}
	if payload.RefundOrderID == 0 {
		payload.RefundOrderID = outbox.AggregateID
	}
	if payload.RefundOrderID != outbox.AggregateID {
		return fmt.Errorf("order refund abnormal outbox aggregate id %d does not match payload refund order id %d", outbox.AggregateID, payload.RefundOrderID)
	}
	if payload.RefundStatus != "ABNORMAL" {
		return fmt.Errorf("order refund abnormal outbox payload refund_status %q is not abnormal", payload.RefundStatus)
	}

	refundOrder, err := processor.store.GetRefundOrder(ctx, payload.RefundOrderID)
	if err != nil {
		return fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if !paymentOrder.OrderID.Valid || paymentOrder.ReservationID.Valid || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("payment order %d business type %q is not order refund", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if payload.PaymentOrderID != 0 && payload.PaymentOrderID != paymentOrder.ID {
		return fmt.Errorf("order refund abnormal outbox payload payment order id %d does not match refund payment order id %d", payload.PaymentOrderID, paymentOrder.ID)
	}
	if payload.OrderID != 0 && payload.OrderID != paymentOrder.OrderID.Int64 {
		return fmt.Errorf("order refund abnormal outbox payload order id %d does not match payment order order id %d", payload.OrderID, paymentOrder.OrderID.Int64)
	}
	merchantID, err := processor.resolveMerchantIDByPaymentOrder(ctx, paymentOrder)
	if err != nil {
		return fmt.Errorf("resolve order refund merchant: %w", err)
	}
	refundID := payload.RefundID
	if refundID == "" && refundOrder.RefundID.Valid {
		refundID = refundOrder.RefundID.String
	}
	alertExtra := refundOrderAlertExtra(paymentOrder, refundOrder, merchantID, map[string]interface{}{
		"refund_id":                   refundID,
		"external_payment_fact_id":    payload.ExternalPaymentFactID,
		"payment_fact_application_id": payload.PaymentFactApplicationID,
	})
	processor.publishAlert(ctx, AlertData{
		AlertType:   AlertTypeRefundFailed,
		Level:       AlertLevelCritical,
		Title:       "订单退款异常 - 需人工介入",
		Message:     fmt.Sprintf("订单退款单 %s 状态异常(ABNORMAL)，微信退款ID: %s，请及时处理", refundOrder.OutRefundNo, refundID),
		RelatedID:   refundOrder.ID,
		RelatedType: "refund_order",
		Extra:       alertExtra,
	})
	return nil
}

func (processor *RedisTaskProcessor) markPaymentDomainOutboxFailed(ctx context.Context, outbox db.PaymentDomainOutbox, dispatchErr error) error {
	nextRetryAt := time.Now().UTC().Add(paymentDomainOutboxRetryDelay)
	_, markErr := processor.store.MarkPaymentDomainOutboxFailed(ctx, db.MarkPaymentDomainOutboxFailedParams{
		ID:          outbox.ID,
		LastError:   pgtype.Text{String: dispatchErr.Error(), Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
	})
	if markErr != nil {
		return fmt.Errorf("%w; mark payment domain outbox failed: %v", dispatchErr, markErr)
	}
	log.Warn().Err(dispatchErr).
		Int64("payment_domain_outbox_id", outbox.ID).
		Str("event_type", outbox.EventType).
		Msg("payment domain outbox dispatch failed; scheduled retry")
	return nil
}
