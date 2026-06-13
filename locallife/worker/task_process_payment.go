package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// Redis 告警频道
const AlertChannel = "notification:platform:alerts"

var errMerchantOrderProfitSharingBillMissing = errors.New("merchant order profit sharing bill missing")

// AlertType 告警类型
type AlertType string

const (
	AlertTypePaymentTimeout              AlertType = "PAYMENT_TIMEOUT"
	AlertTypeTaskEnqueueFailure          AlertType = "TASK_ENQUEUE_FAILURE"
	AlertTypeProfitSharingFailed         AlertType = "PROFIT_SHARING_FAILED"
	AlertTypeProfitSharingReceiverFailed AlertType = "PROFIT_SHARING_RECEIVER_FAILED"
	AlertTypeRefundFailed                AlertType = "REFUND_FAILED"
	AlertTypeWithdrawFailed              AlertType = "WITHDRAW_FAILED"
	AlertTypeSystemError                 AlertType = "SYSTEM_ERROR"
	AlertTypeBillMismatch                AlertType = "BILL_MISMATCH"
	AlertTypeRiderDepositExpiry          AlertType = "RIDER_DEPOSIT_EXPIRY"
	AlertTypeCredentialExpiry            AlertType = "CREDENTIAL_EXPIRY"
	AlertTypeOCRJobFailed                AlertType = "OCR_JOB_FAILED"
	AlertTypeOCRRetryExhausted           AlertType = "OCR_RETRY_EXHAUSTED"
	AlertTypePrintAnomalyTimeout         AlertType = "PRINT_ANOMALY_TIMEOUT"
)

const (
	profitSharingEnqueueDedupWindow             = 12 * time.Minute
	profitSharingResultNotificationDedupWindow  = 24 * time.Hour
	profitSharingResultNotificationExpireWindow = 7 * 24 * time.Hour
	riderNotificationEntityType                 = "rider"
	riderNotificationChannelPrefix              = "notification:rider:"
	riderDeliveryPoolUpdateMessageType          = websocket.MessageTypeDeliveryPoolNew
	riderNewDeliveryOrderPayloadType            = "new_delivery_order"
	riderHighValueDeliveryFeeThreshold          = int64(1000)
	riderDeliverySearchStartDistanceM           = 100.0
	riderDeliverySearchStepDistanceM            = 100.0
	riderDeliverySearchMaxDistanceM             = 5000.0
	riderDeliverySearchMinNotifyRiderCount      = 3
)

type riderDeliveryOrderNotificationPayload struct {
	Type               string    `json:"type"`
	OrderID            int64     `json:"order_id"`
	DeliveryID         int64     `json:"delivery_id"`
	MerchantID         int64     `json:"merchant_id"`
	MerchantName       string    `json:"merchant_name"`
	PickupAddress      string    `json:"pickup_address"`
	PickupLongitude    float64   `json:"pickup_longitude"`
	PickupLatitude     float64   `json:"pickup_latitude"`
	DeliveryAddress    string    `json:"delivery_address"`
	DeliveryLongitude  float64   `json:"delivery_longitude"`
	DeliveryLatitude   float64   `json:"delivery_latitude"`
	DeliveryFee        int64     `json:"delivery_fee"`
	Distance           int32     `json:"distance"`
	Priority           int32     `json:"priority"`
	ExpectedPickupAt   time.Time `json:"expected_pickup_at"`
	ExpectedDeliveryAt time.Time `json:"expected_delivery_at"`
	IsHighValue        bool      `json:"is_high_value"`
	CreatedAt          time.Time `json:"created_at"`
}

func shouldDispatchOrderProfitSharing(order db.Order) bool {
	if order.ReservationID.Valid {
		return true
	}

	switch order.OrderType {
	case "takeout", "dine_in", "takeaway":
		return false
	default:
		return true
	}
}

func withProfitSharingEnqueueDedup(opts ...asynq.Option) []asynq.Option {
	merged := make([]asynq.Option, 0, len(opts)+1)
	merged = append(merged, opts...)
	merged = append(merged, asynq.Unique(profitSharingEnqueueDedupWindow))
	return merged
}

func profitSharingResultNotificationExpiresAt(order db.ProfitSharingOrder) time.Time {
	if order.FinishedAt.Valid {
		return order.FinishedAt.Time.Add(profitSharingResultNotificationExpireWindow)
	}
	if order.CreatedAt.IsZero() {
		return time.Now().Add(profitSharingResultNotificationExpireWindow)
	}
	return order.CreatedAt.Add(profitSharingResultNotificationExpireWindow)
}

func merchantVisiblePaymentChannelFee(order db.ProfitSharingOrder) int64 {
	if order.CalculationVersion == logic.BaofuSettlementCalculationVersionV2 {
		return order.MerchantPaymentFee
	}
	return order.PaymentFee
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelInfo     AlertLevel = "info"
)

// AlertData 告警数据结构
type AlertData struct {
	AlertType   AlertType              `json:"alert_type"`
	Level       AlertLevel             `json:"level"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	RelatedID   int64                  `json:"related_id"`
	RelatedType string                 `json:"related_type"`
	Extra       map[string]interface{} `json:"extra"`
	Timestamp   time.Time              `json:"timestamp"`
}

// publishAlert 通过 Redis Pub/Sub 发布告警
func (processor *RedisTaskProcessor) publishAlert(ctx context.Context, alert AlertData) {
	alert.Timestamp = time.Now()
	if err := SavePlatformAlertEvent(
		ctx,
		processor.store,
		string(alert.AlertType),
		string(alert.Level),
		alert.Title,
		alert.Message,
		alert.RelatedID,
		alert.RelatedType,
		alert.Extra,
		alert.Timestamp,
	); err != nil {
		log.Warn().Err(err).Str("alert_type", string(alert.AlertType)).Msg("persist platform alert event failed before pubsub publish")
	}

	if processor.pubSubPublisher == nil {
		log.Warn().Msg("pubsub publisher not configured, cannot publish alert")
		return
	}

	wsMessage := map[string]any{
		"type":      "alert",
		"data":      alert,
		"timestamp": alert.Timestamp,
	}
	wsMessageJSON, err := json.Marshal(wsMessage)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal alert message")
		return
	}

	if err := processor.pubSubPublisher.Publish(ctx, AlertChannel, wsMessageJSON); err != nil {
		log.Error().Err(err).Str("alert_type", string(alert.AlertType)).Msg("failed to publish alert to redis")
	} else {
		log.Info().
			Str("alert_type", string(alert.AlertType)).
			Str("level", string(alert.Level)).
			Str("title", alert.Title).
			Msg("alert published to Redis")
	}
}

// maybeMarkPaymentOrderRefunded 仅在累计退款额 >= 支付金额时才将支付单标记为 refunded，
// 避免部分退款错误终结支付单。
func (processor *RedisTaskProcessor) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) {
	totalSuccessfulRefunded, err := processor.store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg("failed to get total successful refunded amount")
		return
	}
	if totalSuccessfulRefunded >= paymentAmount {
		if _, dbErr := processor.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrderID).Msg("failed to mark payment order as refunded")
		}
	} else {
		log.Info().
			Int64("payment_order_id", paymentOrderID).
			Int64("total_successful_refunded", totalSuccessfulRefunded).
			Int64("payment_amount", paymentAmount).
			Msg("partial refund: payment order not yet fully refunded")
	}
}

func (processor *RedisTaskProcessor) markRefundOrderFailed(ctx context.Context, refundOrderID int64) error {
	_, err := processor.store.UpdateRefundOrderToFailed(ctx, refundOrderID)
	return err
}

func (processor *RedisTaskProcessor) markRefundOrderSuccess(ctx context.Context, refundOrderID int64) error {
	_, err := processor.store.UpdateRefundOrderToSuccess(ctx, refundOrderID)
	return err
}

func (processor *RedisTaskProcessor) markRefundOrderProcessing(ctx context.Context, params db.UpdateRefundOrderToProcessingParams) error {
	_, err := processor.store.UpdateRefundOrderToProcessing(ctx, params)
	return err
}

func refundRequestTotalAmount(paymentAmount, refundAmount int64) int64 {
	if refundAmount > paymentAmount {
		return refundAmount
	}
	return paymentAmount
}

func (processor *RedisTaskProcessor) publishWSMessage(ctx context.Context, channel string, payload []byte) {
	if processor.pubSubPublisher == nil {
		log.Warn().Str("channel", channel).Msg("pubsub publisher not configured, skip ws publish")
		return
	}

	if err := processor.pubSubPublisher.Publish(ctx, channel, payload); err != nil {
		log.Error().Err(err).Str("channel", channel).Msg("publish ws message failed")
	}
}

// 任务类型常量
const (
	TaskProcessRefund        = "payment:initiate_refund"
	TaskProcessRefundResult  = "payment:process_refund"
	TaskProcessAnomalyRefund = "payment:process_anomaly_refund" // 已关闭订单异常退款
)

// PayloadProcessRefund 发起退款任务载荷
type PayloadProcessRefund struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	OrderID        int64  `json:"order_id"`
	ReservationID  int64  `json:"reservation_id,omitempty"` // 预定退款时使用
	RefundAmount   int64  `json:"refund_amount"`
	Reason         string `json:"reason"`
	OutRefundNo    string `json:"out_refund_no,omitempty"`
}

// PayloadProcessAnomalyRefund 已关闭/失败订单异常退款任务载荷
type PayloadProcessAnomalyRefund struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	TransactionID  string `json:"transaction_id"` // 微信交易号，直接用于发起退款
	RefundAmount   int64  `json:"refund_amount"`
	OutRefundNo    string `json:"out_refund_no"` // 幂等键（"CRF{paymentOrderID}"）
}

// RefundResultPayload 退款结果任务载荷
type RefundResultPayload struct {
	OutRefundNo  string `json:"out_refund_no"`
	RefundStatus string `json:"refund_status"` // SUCCESS/ABNORMAL/CLOSED
	RefundID     string `json:"refund_id"`
}

// ProfitSharingResultPayload 分账结果 outbox 载荷
type ProfitSharingResultPayload struct {
	ProfitSharingOrderID int64  `json:"profit_sharing_order_id"` // 分账订单ID
	OutOrderNo           string `json:"out_order_no"`            // 商户分账单号
	Result               string `json:"result"`                  // 分账结果：SUCCESS/CLOSED/FAILED
	FailReason           string `json:"fail_reason"`             // 失败原因
	MerchantID           int64  `json:"merchant_id"`             // 商户ID
}

// DistributeTaskProcessRefund 分发发起退款任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessRefund(
	ctx context.Context,
	payload *PayloadProcessRefund,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessRefund, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("order_id", payload.OrderID).
		Int64("refund_amount", payload.RefundAmount).
		Msg("enqueued refund task")

	return nil
}

// DistributeTaskProcessAnomalyRefund 分发已关闭/失败订单异常退款任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessAnomalyRefund(
	ctx context.Context,
	payload *PayloadProcessAnomalyRefund,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessAnomalyRefund, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_order_id", payload.PaymentOrderID).
		Str("transaction_id", payload.TransactionID).
		Int64("refund_amount", payload.RefundAmount).
		Str("out_refund_no", payload.OutRefundNo).
		Msg("enqueued anomaly refund task")

	return nil
}

// DistributeTaskProcessRefundResult 分发退款结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessRefundResult(
	ctx context.Context,
	payload *RefundResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessRefundResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Str("out_refund_no", payload.OutRefundNo).
		Msg("enqueued task")

	return nil
}

// sendOrderPaidNotifications 发送订单支付成功的实时通知
func (processor *RedisTaskProcessor) sendOrderPaidNotifications(ctx context.Context, result db.ProcessOrderPaymentTxResult) error {
	// 1. 通知商户：有新订单
	if err := processor.notifyMerchantNewOrder(ctx, result.Order); err != nil {
		return fmt.Errorf("notify merchant new order: %w", err)
	}

	// 2. 如果是外卖订单，通知区域内骑手：订单池有新单
	if result.Delivery != nil && result.PoolItem != nil {
		processor.notifyRidersNewDelivery(ctx, result.Order, result.Delivery, result.PoolItem)
	}

	return nil
}

func (processor *RedisTaskProcessor) distributeTaskSendNotificationWithLog(ctx context.Context, payload *SendNotificationPayload, message string, opts ...asynq.Option) {
	if processor.distributor == nil {
		return
	}
	if err := processor.distributor.DistributeTaskSendNotification(ctx, payload, opts...); err != nil {
		log.Error().Err(err).
			Int64("user_id", payload.UserID).
			Str("notification_type", payload.Type).
			Str("related_type", payload.RelatedType).
			Int64("related_id", payload.RelatedID).
			Str("title", payload.Title).
			Msg(message)
	}
}

// notifyMerchantNewOrder 通知商户有新订单
func (processor *RedisTaskProcessor) notifyMerchantNewOrder(ctx context.Context, order db.Order) error {
	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant for notification: %w", err)
	}
	merchantPayload := logic.BuildMerchantNewOrderNotification(order, merchant.Name)

	feeBreakdown, err := processor.loadMerchantOrderFeeBreakdown(ctx, order)
	if err != nil {
		if isTerminalOrderStatus(order.Status) && errors.Is(err, errMerchantOrderProfitSharingBillMissing) {
			log.Warn().Err(err).
				Int64("order_id", order.ID).
				Int64("merchant_id", order.MerchantID).
				Str("order_no", order.OrderNo).
				Str("status", order.Status).
				Msg("skip merchant new order notification for terminal legacy order without fee breakdown")
			return nil
		}
		log.Error().Err(err).
			Int64("order_id", order.ID).
			Int64("merchant_id", order.MerchantID).
			Str("order_no", order.OrderNo).
			Str("status", order.Status).
			Msg("merchant new order fee breakdown unavailable")
		return fmt.Errorf("build merchant new order fee breakdown: %w", err)
	}

	items, err := processor.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return fmt.Errorf("load order items for merchant new order snapshot: %w", err)
	}
	itemViews, err := logic.BuildOrderItemViews(items)
	if err != nil {
		return fmt.Errorf("build order items for merchant new order snapshot: %w", err)
	}

	// 通过异步任务发送通知给商户
	expiresAt := time.Now().Add(24 * time.Hour)
	err = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:      merchant.OwnerUserID,
		Type:        "order",
		Title:       "🆕 新订单",
		Content:     merchantPayload.Content,
		RelatedType: "order",
		RelatedID:   order.ID,
		ExtraData: map[string]any{
			"message_id":    merchantPayload.MessageID,
			"order_no":      order.OrderNo,
			"order_type":    order.OrderType,
			"total_amount":  order.TotalAmount,
			"shop_name":     merchantPayload.ShopName,
			"fee_breakdown": feeBreakdown,
		},
		ExpiresAt: &expiresAt,
	}, asynq.Queue(QueueDefault))

	if err != nil {
		return fmt.Errorf("distribute merchant notification task: %w", err)
	} else {
		log.Info().
			Int64("order_id", order.ID).
			Int64("merchant_id", merchant.ID).
			Str("order_no", order.OrderNo).
			Msg("✅ merchant new order notification task distributed")
	}

	orderSnapshot := buildOrderSnapshotPayload(order, itemViews)
	orderSnapshot["message_id"] = merchantPayload.MessageID
	orderSnapshot["event"] = merchantPayload.Event
	orderSnapshot["order_id"] = merchantPayload.OrderID
	orderSnapshot["title"] = merchantPayload.Title
	orderSnapshot["content"] = merchantPayload.Content
	orderSnapshot["amount"] = merchantPayload.Amount
	orderSnapshot["shop_name"] = merchantPayload.ShopName
	orderSnapshot["fee_breakdown"] = feeBreakdown
	payload, err := json.Marshal(orderSnapshot)
	if err != nil {
		log.Error().Err(err).
			Int64("order_id", order.ID).
			Int64("merchant_id", merchant.ID).
			Msg("marshal merchant new order websocket payload failed")
		return fmt.Errorf("marshal merchant new order websocket payload: %w", err)
	}
	wsMessage := websocket.Message{
		ID:        merchantPayload.MessageID,
		Type:      "new_order",
		Data:      json.RawMessage(payload),
		Timestamp: time.Now(),
	}
	pushMsg := websocket.NotificationPushMessage{
		EntityType: "merchant",
		EntityID:   merchant.ID,
		Message:    wsMessage,
	}
	wsMessageJSON, err := json.Marshal(pushMsg)
	if err != nil {
		log.Error().Err(err).
			Int64("order_id", order.ID).
			Int64("merchant_id", merchant.ID).
			Msg("marshal merchant websocket push message failed")
		return fmt.Errorf("marshal merchant websocket push message: %w", err)
	}
	channel := fmt.Sprintf("notification:merchant:%d", merchant.ID)
	processor.publishWSMessage(ctx, channel, wsMessageJSON)
	return nil
}

func isTerminalOrderStatus(status string) bool {
	return status == db.OrderStatusCancelled || status == db.OrderStatusCompleted
}

func (processor *RedisTaskProcessor) loadMerchantOrderFeeBreakdown(ctx context.Context, order db.Order) (logic.MerchantOrderFeeBreakdown, error) {
	paymentOrder, err := processor.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	})
	if err != nil {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("get latest payment order by order: %w", err)
	}
	if paymentOrder.Status != "paid" {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("merchant new order notification requires paid payment order: order_id=%d payment_order_id=%d status=%s", order.ID, paymentOrder.ID, paymentOrder.Status)
	}
	if !paymentOrder.OrderID.Valid || paymentOrder.OrderID.Int64 != order.ID || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: payment order mismatch for order_id=%d payment_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID)
	}
	if paymentOrder.Amount != order.TotalAmount {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: payment amount mismatch for order_id=%d payment_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID)
	}
	profitSharingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: get profit sharing order by payment order: %w", errMerchantOrderProfitSharingBillMissing, err)
		}
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("get profit sharing order by payment order: %w", err)
	}
	if profitSharingOrder.PaymentOrderID != paymentOrder.ID ||
		profitSharingOrder.MerchantID != order.MerchantID ||
		profitSharingOrder.TotalAmount != order.TotalAmount {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: profit sharing bill mismatch for order_id=%d payment_order_id=%d profit_sharing_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID, profitSharingOrder.ID)
	}
	breakdown, err := logic.BuildMerchantOrderFeeBreakdown(logic.BuildMerchantOrderFeeBreakdownInput{
		Order:              order,
		ProfitSharingOrder: &profitSharingOrder,
	})
	if err != nil {
		return logic.MerchantOrderFeeBreakdown{}, err
	}
	return breakdown, nil
}

type orderItemSnapshot struct {
	ID             int64                          `json:"id"`
	Name           string                         `json:"name"`
	UnitPrice      int64                          `json:"unit_price"`
	Quantity       int16                          `json:"quantity"`
	Subtotal       int64                          `json:"subtotal"`
	SpecsText      string                         `json:"specs_text"`
	DishID         *int64                         `json:"dish_id,omitempty"`
	ComboID        *int64                         `json:"combo_id,omitempty"`
	Customizations []logic.OrderItemCustomization `json:"customizations,omitempty"`
}

func buildOrderSnapshotPayload(order db.Order, items []logic.OrderItemView) map[string]any {
	payload := map[string]any{
		"id":                    order.ID,
		"order_no":              order.OrderNo,
		"user_id":               order.UserID,
		"merchant_id":           order.MerchantID,
		"order_type":            order.OrderType,
		"delivery_fee":          order.DeliveryFee,
		"subtotal":              order.Subtotal,
		"discount_amount":       order.DiscountAmount,
		"delivery_fee_discount": order.DeliveryFeeDiscount,
		"total_amount":          order.TotalAmount,
		"status":                order.Status,
		"fulfillment_status":    order.FulfillmentStatus,
		"created_at":            order.CreatedAt,
	}

	if order.AddressID.Valid {
		payload["address_id"] = order.AddressID.Int64
	}
	if order.DeliveryDistance.Valid {
		payload["delivery_distance"] = order.DeliveryDistance.Int32
	}
	if order.TableID.Valid {
		payload["table_id"] = order.TableID.Int64
	}
	if order.ReservationID.Valid {
		payload["reservation_id"] = order.ReservationID.Int64
	}
	if order.PaymentMethod.Valid {
		payload["payment_method"] = order.PaymentMethod.String
	}
	if order.Notes.Valid {
		payload["notes"] = order.Notes.String
	}
	if order.PaidAt.Valid {
		payload["paid_at"] = order.PaidAt.Time
	}
	if order.CompletedAt.Valid {
		payload["completed_at"] = order.CompletedAt.Time
	}
	if order.CancelledAt.Valid {
		payload["cancelled_at"] = order.CancelledAt.Time
	}
	if order.CancelReason.Valid {
		payload["cancel_reason"] = order.CancelReason.String
	}
	if order.UpdatedAt.Valid {
		payload["updated_at"] = order.UpdatedAt.Time
	}
	if len(items) > 0 {
		respItems := make([]orderItemSnapshot, len(items))
		for index, item := range items {
			respItems[index] = orderItemSnapshot{
				ID:             item.ID,
				Name:           item.Name,
				UnitPrice:      item.UnitPrice,
				Quantity:       item.Quantity,
				Subtotal:       item.Subtotal,
				SpecsText:      item.SpecsText,
				DishID:         item.DishID,
				ComboID:        item.ComboID,
				Customizations: item.Customizations,
			}
		}
		payload["items"] = respItems
	}

	return payload
}

// notifyRidersNewDelivery 通知附近骑手有新代取订单
// 推送策略：从100米开始按100米递增，超过1000米则改为全区县推送
func (processor *RedisTaskProcessor) notifyRidersNewDelivery(ctx context.Context, order db.Order, delivery *db.Delivery, poolItem *db.DeliveryPool) {
	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", order.MerchantID).Msg("get merchant for rider notification failed")
		return
	}

	// 获取取餐坐标（作为中心点）
	pickupLng, _ := poolItem.PickupLongitude.Float64Value()
	pickupLat, _ := poolItem.PickupLatitude.Float64Value()
	deliveryLng, _ := poolItem.DeliveryLongitude.Float64Value()
	deliveryLat, _ := poolItem.DeliveryLatitude.Float64Value()

	if processor.deliveryBroadcast != nil {
		if err := processor.deliveryBroadcast.BroadcastNewOrderNotification(ctx, *poolItem, merchant.Name); err != nil {
			log.Warn().Err(err).
				Int64("order_id", order.ID).
				Int64("delivery_id", delivery.ID).
				Msg("broadcast new delivery order failed")
		}
	} else {
		var ridersToNotify []int64
		seenRiders := make(map[int64]struct{}, riderDeliverySearchMinNotifyRiderCount)
		var usedDistance float64

		for distance := riderDeliverySearchStartDistanceM; distance <= riderDeliverySearchMaxDistanceM; distance += riderDeliverySearchStepDistanceM {
			riders, err := processor.store.ListNearbyRiders(ctx, db.ListNearbyRidersParams{
				CenterLat:   pickupLat.Float64,
				CenterLng:   pickupLng.Float64,
				MaxDistance: distance,
				LimitCount:  1000,
			})
			if err != nil {
				log.Error().Err(err).Float64("distance", distance).Msg("list nearby riders failed")
				continue
			}

			usedDistance = distance
			for _, rider := range riders {
				if _, ok := seenRiders[rider.ID]; ok {
					continue
				}
				seenRiders[rider.ID] = struct{}{}
				ridersToNotify = append(ridersToNotify, rider.ID)
			}

			if len(ridersToNotify) >= riderDeliverySearchMinNotifyRiderCount {
				break
			}
		}

		if len(ridersToNotify) == 0 {
			log.Warn().
				Int64("order_id", order.ID).
				Float64("pickup_lat", pickupLat.Float64).
				Float64("pickup_lng", pickupLng.Float64).
				Msg("no nearby online riders, order waiting in pool")
			return
		}

		notificationData := riderDeliveryOrderNotificationPayload{
			Type:               riderNewDeliveryOrderPayloadType,
			OrderID:            order.ID,
			DeliveryID:         delivery.ID,
			MerchantID:         merchant.ID,
			MerchantName:       merchant.Name,
			PickupAddress:      delivery.PickupAddress,
			PickupLongitude:    pickupLng.Float64,
			PickupLatitude:     pickupLat.Float64,
			DeliveryAddress:    delivery.DeliveryAddress,
			DeliveryLongitude:  deliveryLng.Float64,
			DeliveryLatitude:   deliveryLat.Float64,
			DeliveryFee:        order.DeliveryFee,
			Distance:           poolItem.Distance,
			Priority:           poolItem.Priority,
			ExpectedPickupAt:   poolItem.ExpectedPickupAt,
			ExpectedDeliveryAt: delivery.EstimatedDeliveryAt.Time,
			IsHighValue:        order.DeliveryFee >= riderHighValueDeliveryFeeThreshold,
			CreatedAt:          poolItem.CreatedAt,
		}
		msgData, err := json.Marshal(notificationData)
		if err != nil {
			log.Error().Err(err).
				Int64("order_id", order.ID).
				Int64("delivery_id", delivery.ID).
				Msg("marshal rider delivery notification payload failed")
			return
		}
		messageTimestamp := time.Now()

		for _, riderID := range ridersToNotify {
			pushMsg := websocket.NotificationPushMessage{
				EntityType: riderNotificationEntityType,
				EntityID:   riderID,
				Message: websocket.Message{
					Type:      riderDeliveryPoolUpdateMessageType,
					Data:      json.RawMessage(msgData),
					Timestamp: messageTimestamp,
				},
			}
			wsMessageJSON, err := json.Marshal(pushMsg)
			if err != nil {
				log.Error().Err(err).
					Int64("order_id", order.ID).
					Int64("delivery_id", delivery.ID).
					Int64("rider_id", riderID).
					Msg("marshal rider websocket push message failed")
				continue
			}
			channel := fmt.Sprintf("%s%d", riderNotificationChannelPrefix, riderID)
			processor.publishWSMessage(ctx, channel, wsMessageJSON)
		}

		log.Info().
			Int64("order_id", order.ID).
			Int64("delivery_id", delivery.ID).
			Float64("search_radius_m", usedDistance).
			Int64("delivery_fee", order.DeliveryFee).
			Bool("is_high_value", order.DeliveryFee >= riderHighValueDeliveryFeeThreshold).
			Msg("new delivery order pushed to nearby riders via worker pubsub path")
	}
}

// ProcessTaskRefundResult 处理退款结果任务
func (processor *RedisTaskProcessor) ProcessTaskRefundResult(ctx context.Context, task *asynq.Task) error {
	var payload RefundResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("out_refund_no", payload.OutRefundNo).
		Str("refund_status", payload.RefundStatus).
		Msg("processing refund result")

	// 查询退款订单
	refundOrder, err := processor.store.GetRefundOrderByOutRefundNo(ctx, payload.OutRefundNo)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("refund order %s not found: %w", payload.OutRefundNo, asynq.SkipRetry)
		}
		return fmt.Errorf("get refund order: %w", err)
	}

	if payload.RefundStatus == "SUCCESS" && refundOrder.Status == "success" {
		log.Info().Str("out_refund_no", payload.OutRefundNo).Msg("refund already succeeded, skip duplicate callback")
		return nil
	}
	if payload.RefundStatus == "ABNORMAL" && refundOrder.Status == "failed" {
		log.Info().Str("out_refund_no", payload.OutRefundNo).Msg("refund already failed, skip duplicate callback")
		return nil
	}
	if payload.RefundStatus == "CLOSED" && refundOrder.Status == "closed" {
		log.Info().Str("out_refund_no", payload.OutRefundNo).Msg("refund already closed, skip duplicate callback")
		return nil
	}

	paymentOrder, paymentErr := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if paymentErr != nil {
		return fmt.Errorf("get payment order for refund result routing: %w", paymentErr)
	}
	isRiderDepositRefund := paymentOrder.BusinessType == "rider_deposit"
	if isRiderDepositRefund {
		return fmt.Errorf("rider deposit refund results must be applied via payment fact application: %w", asynq.SkipRetry)
	}
	if isReservationRefundPayment(paymentOrder) {
		return fmt.Errorf("reservation refund results must be applied via payment fact application: %w", asynq.SkipRetry)
	}
	// 根据退款状态更新
	switch payload.RefundStatus {
	case "SUCCESS":
		if isReservationRefundPayment(paymentOrder) {
			if err := processor.markReservationRefundSuccess(ctx, refundOrder, paymentOrder); err != nil {
				return fmt.Errorf("mark reservation refund success: %w", err)
			}
		} else {
			_, err = processor.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
			if err != nil {
				return fmt.Errorf("update refund order to success: %w", err)
			}
		}

		if !isReservationRefundPayment(paymentOrder) {
			processor.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
			if processor.distributor != nil {
				expiresAt := time.Now().Add(7 * 24 * time.Hour)
				processor.distributeTaskSendNotificationWithLog(ctx, &SendNotificationPayload{
					UserID:      paymentOrder.UserID,
					Type:        "refund",
					Title:       "退款成功",
					Content:     fmt.Sprintf("您的订单退款已完成，退款金额%.2f元", float64(refundOrder.RefundAmount)/100),
					RelatedType: "refund",
					RelatedID:   refundOrder.ID,
					ExtraData: map[string]any{
						"out_refund_no": payload.OutRefundNo,
						"refund_id":     payload.RefundID,
						"amount":        refundOrder.RefundAmount,
					},
					ExpiresAt: &expiresAt,
				}, "send refund success notification failed", asynq.Queue(QueueDefault))
			}
		}

	case "ABNORMAL":
		merchantID := int64(0)
		resolvedMerchantID, resolveErr := processor.resolveMerchantIDByPaymentOrder(ctx, paymentOrder)
		if resolveErr != nil {
			log.Warn().Err(resolveErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to resolve merchant for refund alert context")
		} else {
			merchantID = resolvedMerchantID
		}
		_, err = processor.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to failed: %w", err)
		}
		log.Warn().Str("out_refund_no", payload.OutRefundNo).Msg("refund abnormal")

		alertExtra := refundOrderAlertExtra(paymentOrder, refundOrder, merchantID, map[string]interface{}{
			"refund_id": payload.RefundID,
		})

		// R-07 修复：通过 Redis Pub/Sub + WebSocket 告警运营团队
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeRefundFailed,
			Level:       AlertLevelCritical,
			Title:       "退款异常 - 需人工介入",
			Message:     fmt.Sprintf("退款单 %s 状态异常(ABNORMAL)，微信退款ID: %s，请及时处理", payload.OutRefundNo, payload.RefundID),
			RelatedID:   refundOrder.ID,
			RelatedType: "refund_order",
			Extra:       alertExtra,
		})

	case "CLOSED":
		_, err = processor.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to closed: %w", err)
		}
		log.Info().Str("out_refund_no", payload.OutRefundNo).Msg("refund closed")
	}

	return nil
}

// ProcessTaskInitiateRefund 处理发起退款任务
func (processor *RedisTaskProcessor) ProcessTaskInitiateRefund(ctx context.Context, task *asynq.Task) error {
	var payload PayloadProcessRefund
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// 预定退款走独立简化流程（宝付主营业务）。
	if payload.ReservationID > 0 {
		return processor.processReservationRefund(ctx, payload)
	}

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("order_id", payload.OrderID).
		Int64("refund_amount", payload.RefundAmount).
		Str("reason", payload.Reason).
		Msg("processing refund task")

	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	if paymentOrder.Status != "paid" {
		log.Warn().
			Int64("payment_order_id", payload.PaymentOrderID).
			Str("status", paymentOrder.Status).
			Msg("payment order not in paid status, skip refund")
		return nil
	}

	if paymentOrder.BusinessType == "rider_deposit" {
		return processor.processRiderDepositMismatchRefund(ctx, paymentOrder, payload)
	}

	orderID := payload.OrderID
	if orderID == 0 && paymentOrder.OrderID.Valid {
		orderID = paymentOrder.OrderID.Int64
	}
	if orderID == 0 {
		return fmt.Errorf("payment order %d missing order context for refund", paymentOrder.ID)
	}

	if _, err := processor.store.GetOrder(ctx, orderID); err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	outRefundNo := strings.TrimSpace(payload.OutRefundNo)
	if outRefundNo == "" {
		outRefundNo = fmt.Sprintf("RF%d_%d", payload.PaymentOrderID, orderID)
	}

	var refundOrder db.RefundOrder
	existingRefund, findErr := processor.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if findErr == nil {
		refundOrder = existingRefund
		if err := validateRefundOrderMatchesPayload(refundOrder, payload); err != nil {
			return err
		}
		switch refundOrder.Status {
		case "success":
			log.Info().Str("out_refund_no", outRefundNo).Msg("refund already succeeded, skip")
			return nil
		case "processing":
			log.Info().Str("out_refund_no", outRefundNo).Msg("refund already processing, skip")
			return nil
		case "failed", "closed":
			log.Info().
				Str("out_refund_no", outRefundNo).
				Int64("refund_order_id", refundOrder.ID).
				Str("status", refundOrder.Status).
				Msg("refund already terminal, skip")
			return nil
		default:
			log.Info().
				Str("out_refund_no", outRefundNo).
				Int64("refund_order_id", refundOrder.ID).
				Str("status", refundOrder.Status).
				Msg("refund order already exists, retrying")
		}
	} else if !errors.Is(findErr, db.ErrRecordNotFound) {
		return fmt.Errorf("check existing refund order: %w", findErr)
	} else {
		txResult, createErr := processor.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: payload.PaymentOrderID,
			RefundType:     "user_cancel",
			RefundAmount:   payload.RefundAmount,
			RefundReason:   payload.Reason,
			OutRefundNo:    outRefundNo,
		})
		if createErr != nil {
			if statusCode, ok := db.IsRefundRequestError(createErr); ok {
				log.Warn().Err(createErr).Int("status", statusCode).Msg("refund business validation failed, skip")
				return nil
			}
			return fmt.Errorf("create refund order tx: %w", createErr)
		}
		refundOrder = txResult.RefundOrder
	}

	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		return processor.processBaofuAggregateRefund(ctx, paymentOrder, refundOrder, payload, outRefundNo)
	}

	return processor.processDirectRefund(ctx, paymentOrder, refundOrder, payload, outRefundNo)
}

func (processor *RedisTaskProcessor) processDirectRefund(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, payload PayloadProcessRefund, outRefundNo string) error {
	wxRefund, err := createDirectRefundContract(ctx, processor.directPaymentClient, &wechatcontracts.DirectRefundRequest{
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      payload.Reason,
		Amount: &wechatcontracts.DirectRefundRequestAmount{
			Refund:   payload.RefundAmount,
			Total:    refundRequestTotalAmount(paymentOrder.Amount, payload.RefundAmount),
			Currency: wechatcontracts.DirectRefundCurrencyCNY,
		},
	})
	if err != nil {
		logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(fmt.Errorf("call wechat refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		return fmt.Errorf("call wechat refund API: %w", err)
	}

	switch wxRefund.Status {
	case wechatcontracts.DirectRefundStatusSuccess:
		if dbErr := processor.markRefundOrderSuccess(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as success: %w", dbErr)
		}
		processor.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
	case wechatcontracts.DirectRefundStatusProcessing:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: wxRefund.RefundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
	default:
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as failed: %w", dbErr)
		}
	}

	log.Info().
		Int64("refund_order_id", refundOrder.ID).
		Str("out_refund_no", outRefundNo).
		Str("status", wxRefund.Status).
		Msg("direct refund request processed")
	return nil
}

func (processor *RedisTaskProcessor) processBaofuAggregateRefund(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	payload PayloadProcessRefund,
	outRefundNo string,
) error {
	if processor.baofuAggregateClient == nil {
		err := errors.New("baofu aggregate client not configured for refund")
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(err, fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	cfg := processor.baofuProfitSharingConfig.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		err := errors.New("baofu collect merchant config not configured for refund")
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(err, fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}

	req := aggregatecontracts.RefundBeforeShareRequest{
		MerchantID:      cfg.CollectMerchantID,
		TerminalID:      cfg.CollectTerminalID,
		OutTradeNo:      strings.TrimSpace(outRefundNo),
		NotifyURL:       cfg.RefundNotifyURL,
		RefundAmountFen: payload.RefundAmount,
		TotalAmountFen:  payload.RefundAmount,
		TransactionTime: time.Now().UTC().Format("20060102150405"),
		RefundReason:    strings.TrimSpace(payload.Reason),
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.OriginTradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OriginOutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}

	refundResp, err := processor.baofuAggregateClient.CreateRefund(ctx, req)
	if err != nil {
		logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
		disposition := logic.ClassifyBaofuRefundCreateFailure(err)
		if dbErr := processor.applyBaofuRefundCreateFailureDisposition(ctx, refundOrder, "", disposition); dbErr != nil {
			return errors.Join(fmt.Errorf("call baofu refund API: %w", err), dbErr)
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, disposition.CommandStatus, err)
		if disposition.RetryCreate {
			return fmt.Errorf("call baofu refund API: %w", err)
		}
		return nil
	}
	if refundResp == nil {
		err := errors.New("baofu refund returned empty result")
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(err, fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return err
	}

	refundID := strings.TrimSpace(refundResp.TradeNo)
	if refundID == "" {
		refundID = strings.TrimSpace(refundResp.OutTradeNo)
	}
	switch strings.ToUpper(strings.TrimSpace(refundResp.ResultCode)) {
	case aggregatecontracts.BusinessResultCodeSuccess:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark baofu refund order as processing: %w", dbErr)
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusAccepted, nil)
	case aggregatecontracts.BusinessResultCodeFail:
		providerErr := logic.BaofuRefundCreateProviderResultError(refundResp)
		disposition := logic.ClassifyBaofuRefundCreateFailure(providerErr)
		if dbErr := processor.applyBaofuRefundCreateFailureDisposition(ctx, refundOrder, refundID, disposition); dbErr != nil {
			return dbErr
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, disposition.CommandStatus, nil)
		if disposition.RetryCreate {
			return fmt.Errorf("baofu refund request retryable")
		}
		return nil
	default:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark baofu refund order as processing: %w", dbErr)
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusUnknown, nil)
	}

	log.Info().
		Int64("refund_order_id", refundOrder.ID).
		Str("out_refund_no", outRefundNo).
		Str("baofu_refund_state", strings.TrimSpace(refundResp.RefundState)).
		Str("result_code", strings.TrimSpace(refundResp.ResultCode)).
		Msg("baofu refund request processed")
	return nil
}

func (processor *RedisTaskProcessor) applyBaofuRefundCreateFailureDisposition(ctx context.Context, refundOrder db.RefundOrder, refundID string, disposition logic.BaofuRefundCreateFailureDisposition) error {
	switch {
	case disposition.MarkProcessing:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: strings.TrimSpace(refundID), Valid: strings.TrimSpace(refundID) != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark baofu refund order as processing after create uncertainty: %w", dbErr)
		}
	case disposition.MarkFailed:
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as failed: %w", dbErr)
		}
	}
	return nil
}

func (processor *RedisTaskProcessor) processRiderDepositMismatchRefund(ctx context.Context, paymentOrder db.PaymentOrder, payload PayloadProcessRefund) error {
	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("refund_amount", payload.RefundAmount).
		Str("reason", payload.Reason).
		Msg("processing rider deposit mismatch refund task")

	if processor.directPaymentClient == nil {
		return fmt.Errorf("payment client not configured, cannot process rider deposit refund")
	}

	outRefundNo := fmt.Sprintf("RFM%d_D", payload.PaymentOrderID)
	var refundOrder db.RefundOrder
	existingRefund, findErr := processor.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if findErr == nil {
		refundOrder = existingRefund
		if refundOrder.Status == "success" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("rider deposit refund already succeeded")
			return nil
		}
		if refundOrder.Status == "processing" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("rider deposit refund already processing")
			return nil
		}
	} else if !errors.Is(findErr, db.ErrRecordNotFound) {
		return fmt.Errorf("check existing rider deposit refund order: %w", findErr)
	} else {
		createdRefundOrder, createErr := processor.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     "amount_mismatch",
			RefundAmount:   payload.RefundAmount,
			RefundReason:   pgtype.Text{String: payload.Reason, Valid: payload.Reason != ""},
			OutRefundNo:    outRefundNo,
			Status:         "pending",
		})
		if createErr != nil {
			if db.ErrorCode(createErr) == db.UniqueViolation {
				lookupRefundOrder, lookupErr := processor.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
				if lookupErr != nil {
					return fmt.Errorf("lookup rider deposit refund order after conflict: %w", lookupErr)
				}
				refundOrder = lookupRefundOrder
			} else {
				return fmt.Errorf("create rider deposit refund order: %w", createErr)
			}
		} else {
			refundOrder = createdRefundOrder
		}
	}

	wxRefund, err := createDirectRefundContract(ctx, processor.directPaymentClient, &wechatcontracts.DirectRefundRequest{
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      payload.Reason,
		Amount: &wechatcontracts.DirectRefundRequestAmount{
			Refund:   payload.RefundAmount,
			Total:    refundRequestTotalAmount(paymentOrder.Amount, payload.RefundAmount),
			Currency: wechatcontracts.DirectRefundCurrencyCNY,
		},
	})
	if err != nil {
		logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(fmt.Errorf("call wechat rider deposit refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		return fmt.Errorf("call wechat rider deposit refund API: %w", err)
	}

	switch wxRefund.Status {
	case wechatcontracts.DirectRefundStatusSuccess:
		if dbErr := processor.markRefundOrderSuccess(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as success: %w", dbErr)
		}
		processor.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
	case wechatcontracts.DirectRefundStatusProcessing:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: wxRefund.RefundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
	default:
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as failed: %w", dbErr)
		}
	}

	return nil
}

// processReservationRefund 处理预定退款。
func (processor *RedisTaskProcessor) processReservationRefund(ctx context.Context, payload PayloadProcessRefund) error {
	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("reservation_id", payload.ReservationID).
		Int64("refund_amount", payload.RefundAmount).
		Str("reason", payload.Reason).
		Msg("processing reservation refund task")

	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	if paymentOrder.Status != "paid" {
		log.Warn().
			Int64("payment_order_id", payload.PaymentOrderID).
			Str("status", paymentOrder.Status).
			Msg("payment order not in paid status, skip reservation refund")
		return nil
	}
	if paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate {
		return mainBusinessRefundChannelDriftError(paymentOrder, "process reservation refund")
	}

	reservation, err := processor.store.GetTableReservation(ctx, payload.ReservationID)
	if err != nil {
		return fmt.Errorf("get reservation: %w", err)
	}
	if !paymentOrder.ReservationID.Valid || paymentOrder.ReservationID.Int64 != reservation.ID {
		return fmt.Errorf("payment order %d is not linked to reservation %d", paymentOrder.ID, reservation.ID)
	}

	// 生成退款单号。新链路优先复用调用方传入的幂等键，旧链路保持兼容。
	outRefundNo := payload.OutRefundNo
	if outRefundNo == "" {
		outRefundNo = fmt.Sprintf("RF%d_R%d", payload.PaymentOrderID, payload.ReservationID)
	}

	// 幂等检查：退款单号是否已存在
	var refundOrder db.RefundOrder
	existingRefund, findErr := processor.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if findErr == nil {
		refundOrder = existingRefund
		if err := validateRefundOrderMatchesPayload(refundOrder, payload); err != nil {
			return err
		}
		if refundOrder.Status == "success" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("reservation refund already succeeded")
			return nil
		}
		if refundOrder.Status == "processing" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("reservation refund already processing")
			return nil
		}
		log.Info().Str("out_refund_no", outRefundNo).Str("status", refundOrder.Status).Msg("reservation refund order exists, retrying")
	} else if !errors.Is(findErr, db.ErrRecordNotFound) {
		return fmt.Errorf("check existing refund order: %w", findErr)
	} else {
		refundType := refundTypeForPaymentOrder(paymentOrder)
		txResult, createErr := processor.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: payload.PaymentOrderID,
			RefundType:     refundType,
			RefundAmount:   payload.RefundAmount,
			RefundReason:   payload.Reason,
			OutRefundNo:    outRefundNo,
		})
		if createErr != nil {
			if _, ok := db.IsRefundRequestError(createErr); ok {
				log.Warn().Err(createErr).Msg("reservation refund business validation failed, skip")
				return nil
			}
			return fmt.Errorf("create refund order tx: %w", createErr)
		}
		refundOrder = txResult.RefundOrder
	}

	return processor.processBaofuAggregateRefund(ctx, paymentOrder, refundOrder, payload, outRefundNo)
}

func validateRefundOrderMatchesPayload(refundOrder db.RefundOrder, payload PayloadProcessRefund) error {
	if refundOrder.PaymentOrderID != payload.PaymentOrderID {
		return fmt.Errorf("refund order %d payment order mismatch: got %d want %d", refundOrder.ID, refundOrder.PaymentOrderID, payload.PaymentOrderID)
	}
	if refundOrder.RefundAmount != payload.RefundAmount {
		return fmt.Errorf("refund order %d amount mismatch: got %d want %d", refundOrder.ID, refundOrder.RefundAmount, payload.RefundAmount)
	}
	return nil
}

func isReservationRefundPayment(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.ReservationID.Valid &&
		(paymentOrder.BusinessType == "reservation" || paymentOrder.BusinessType == "reservation_addon")
}

func (processor *RedisTaskProcessor) markReservationRefundSuccess(ctx context.Context, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder) error {
	updatedRefundOrder, err := processor.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	processor.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
	if !paymentOrder.ReservationID.Valid || updatedRefundOrder.RefundAmount <= 0 {
		return nil
	}

	if _, err := processor.store.AddReservationPrepaidAmount(ctx, db.AddReservationPrepaidAmountParams{
		ID:            paymentOrder.ReservationID.Int64,
		PrepaidAmount: -updatedRefundOrder.RefundAmount,
	}); err != nil {
		return fmt.Errorf("update reservation prepaid amount: %w", err)
	}

	return nil
}

// ==================== 已关闭/失败订单异常退款 ====================

// ProcessTaskAnomalyRefund 处理已关闭/失败状态支付单收到付款后的自动退款任务。
func (processor *RedisTaskProcessor) ProcessTaskAnomalyRefund(ctx context.Context, task *asynq.Task) error {
	var payload PayloadProcessAnomalyRefund
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Str("transaction_id", payload.TransactionID).
		Int64("refund_amount", payload.RefundAmount).
		Str("out_refund_no", payload.OutRefundNo).
		Msg("processing anomaly refund task")

	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.Status != "closed" && paymentOrder.Status != "failed" {
		log.Warn().
			Int64("payment_order_id", payload.PaymentOrderID).
			Str("status", paymentOrder.Status).
			Msg("payment order no longer in closed/failed status, skip anomaly refund")
		return nil
	}

	refundOrder, err := processor.store.CreateAnomalyRefundRecord(ctx, db.CreateAnomalyRefundRecordParams{
		PaymentOrderID: payload.PaymentOrderID,
		RefundAmount:   payload.RefundAmount,
		OutRefundNo:    payload.OutRefundNo,
	})
	if err != nil {
		return fmt.Errorf("create anomaly refund record: %w", err)
	}
	if refundOrder.Status == "success" || refundOrder.Status == "processing" {
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("status", refundOrder.Status).
			Msg("anomaly refund already processed, skip")
		return nil
	}

	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		return processor.processBaofuAggregateRefund(ctx, paymentOrder, refundOrder, PayloadProcessRefund{
			PaymentOrderID: payload.PaymentOrderID,
			RefundAmount:   payload.RefundAmount,
			Reason:         "已关闭订单异常到账，系统自动退款",
			OutRefundNo:    payload.OutRefundNo,
		}, payload.OutRefundNo)
	}

	merchantID := int64(0)
	if resolvedMerchantID, resolveErr := processor.resolveMerchantIDByPaymentOrder(ctx, paymentOrder); resolveErr == nil {
		merchantID = resolvedMerchantID
	} else {
		log.Warn().Err(resolveErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to resolve merchant for anomaly refund alert context")
	}

	wxRefund, err := createDirectRefundContract(ctx, processor.directPaymentClient, &wechatcontracts.DirectRefundRequest{
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: payload.OutRefundNo,
		Reason:      "已关闭订单异常到账，系统自动退款",
		Amount: &wechatcontracts.DirectRefundRequestAmount{
			Refund:   payload.RefundAmount,
			Total:    paymentOrder.Amount,
			Currency: wechatcontracts.DirectRefundCurrencyCNY,
		},
	})
	if err != nil {
		logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, payload.OutRefundNo, paymentOrder.PaymentType, err)
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(fmt.Errorf("call wechat refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		return fmt.Errorf("call wechat refund API: %w", err)
	}

	switch wxRefund.Status {
	case wechatcontracts.DirectRefundStatusSuccess:
		if dbErr := processor.markRefundOrderSuccess(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as success: %w", dbErr)
		}
		processor.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", payload.OutRefundNo).
			Msg("anomaly direct refund completed successfully")
	case wechatcontracts.DirectRefundStatusProcessing:
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: wxRefund.RefundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("refund_id", wxRefund.RefundID).
			Msg("anomaly direct refund in processing, will be updated via refund callback")
	default:
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark refund order as failed: %w", dbErr)
		}
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeRefundFailed,
			Level:       AlertLevelCritical,
			Title:       "异常退款接口返回非预期状态",
			Message:     fmt.Sprintf("退款单 %d（支付单 %d）收到微信退款状态 %q，请核查", refundOrder.ID, payload.PaymentOrderID, wxRefund.Status),
			RelatedID:   refundOrder.ID,
			RelatedType: "refund_order",
			Extra: refundOrderAlertExtra(paymentOrder, refundOrder, merchantID, map[string]interface{}{
				"transaction_id": payload.TransactionID,
				"refund_id":      wxRefund.RefundID,
				"wechat_status":  wxRefund.Status,
			}),
		})
	}

	return nil
}

func recordWorkerBaofuRefundCommand(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundResp *aggregatecontracts.RefundResult, status string, commandErr error) {
	refundID := ""
	resultCode := ""
	refundState := ""
	errorCode := ""
	errorMessage := ""
	if refundResp != nil {
		refundID = strings.TrimSpace(refundResp.TradeNo)
		if refundID == "" {
			refundID = strings.TrimSpace(refundResp.OutTradeNo)
		}
		resultCode = strings.TrimSpace(refundResp.ResultCode)
		refundState = strings.TrimSpace(refundResp.RefundState)
		errorCode = strings.TrimSpace(refundResp.ErrorCode)
		if errorCode != "" || strings.TrimSpace(refundResp.ErrorMessage) != "" || strings.EqualFold(resultCode, aggregatecontracts.BusinessResultCodeFail) {
			errorMessage = strings.TrimSpace(baofu.BaofuCommandMessage(refundResp.ErrorCode, refundResp.ErrorMessage))
		}
	}
	if commandErr != nil && errorMessage == "" {
		var providerErr *baofu.ProviderError
		if errors.As(commandErr, &providerErr) {
			errorCode = strings.TrimSpace(providerErr.UpstreamCode)
			errorMessage = strings.TrimSpace(baofu.BaofuCommandMessage(providerErr.UpstreamCode, providerErr.UpstreamMessage))
		} else {
			errorMessage = strings.TrimSpace(commandErr.Error())
		}
	}

	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerBaofuRefundCommandInput(
		paymentOrder,
		refundOrder,
		status,
		workerStringPtrIfNotEmpty(refundID),
		workerStringPtrIfNotEmpty(errorCode),
		workerStringPtrIfNotEmpty(errorMessage),
		workerBaofuRefundCommandSnapshot(map[string]string{
			"out_refund_no":           refundOrder.OutRefundNo,
			"refund_id":               refundID,
			"result_code":             resultCode,
			"refund_state":            refundState,
			"error_code":              errorCode,
			"error_message":           errorMessage,
			"error_message_sanitized": baofu.SanitizeUpstreamMessageForRecord(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("record worker baofu refund command failed")
	}
}

func workerStringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func workerPaymentCommandErrorFields(err error) (*string, *string) {
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		return workerStringPtrIfNotEmpty(wxErr.Code), workerStringPtrIfNotEmpty(wxErr.Message)
	}
	var baofuErr *baofu.ProviderError
	if errors.As(err, &baofuErr) {
		return workerStringPtrIfNotEmpty(baofuErr.UpstreamCode), workerStringPtrIfNotEmpty(strings.TrimSpace(baofu.BaofuCommandMessage(baofuErr.UpstreamCode, baofuErr.UpstreamMessage)))
	}
	if err == nil {
		return nil, nil
	}
	return nil, workerStringPtrIfNotEmpty(err.Error())
}

func dbWorkerBaofuRefundCommandInput(
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) logic.RecordExternalPaymentCommandInput {
	businessObjectType := "refund_order"
	businessObjectID := refundOrder.ID
	return logic.RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        workerBaofuRefundBusinessOwner(paymentOrder),
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    strings.TrimSpace(refundOrder.OutRefundNo),
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func workerBaofuRefundBusinessOwner(paymentOrder db.PaymentOrder) string {
	if paymentOrder.BusinessType == "reservation" || paymentOrder.BusinessType == "reservation_addon" || paymentOrder.ReservationID.Valid {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return db.ExternalPaymentBusinessOwnerOrder
}

func workerBaofuRefundCommandSnapshot(values map[string]string) []byte {
	snapshot := map[string]string{
		"provider":  db.ExternalPaymentProviderBaofu,
		"operation": "order_refund",
	}
	for key, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			snapshot[key] = trimmed
		}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"order_refund"}`)
	}
	return raw
}

func (processor *RedisTaskProcessor) processProfitSharingResultPayload(ctx context.Context, payload ProfitSharingResultPayload, requireEnqueueSuccess bool) error {
	log.Info().
		Int64("profit_sharing_order_id", payload.ProfitSharingOrderID).
		Str("result", payload.Result).
		Msg("processing profit sharing result")

	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, payload.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Error().Int64("merchant_id", payload.MerchantID).Msg("merchant not found")
			return nil // 不重试
		}
		return fmt.Errorf("get merchant: %w", err)
	}

	// 获取分账订单信息
	profitSharingOrder, err := processor.store.GetProfitSharingOrderByOutOrderNo(ctx, payload.OutOrderNo)
	if err != nil {
		log.Error().Err(err).Str("out_order_no", payload.OutOrderNo).Msg("get profit sharing order")
		return nil
	}

	switch payload.Result {
	case "SUCCESS":
		// 分账成功，通知商户
		if processor.distributor == nil {
			if requireEnqueueSuccess {
				return fmt.Errorf("task distributor not configured for profit sharing success notification")
			}
			return nil
		}

		expiresAt := profitSharingResultNotificationExpiresAt(profitSharingOrder)
		err := processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
			UserID:      merchant.OwnerUserID,
			Type:        "finance",
			Title:       "订单收入已到账",
			Content:     fmt.Sprintf("您有一笔订单收入%.2f元已到账", float64(profitSharingOrder.MerchantAmount)/100),
			RelatedType: "profit_sharing",
			RelatedID:   payload.ProfitSharingOrderID,
			ExtraData: map[string]any{
				"merchant_receivable_amount":  profitSharingOrder.MerchantAmount,
				"platform_service_fee_amount": profitSharingOrder.PlatformCommission + profitSharingOrder.OperatorCommission,
				"payment_channel_fee_amount":  merchantVisiblePaymentChannelFee(profitSharingOrder),
			},
			ExpiresAt: &expiresAt,
		}, asynq.Unique(profitSharingResultNotificationDedupWindow))
		if err != nil && requireEnqueueSuccess {
			return fmt.Errorf("enqueue profit sharing success notification: %w", err)
		}
		if err := processor.distributeRiderProfitSharingResultNotification(ctx, profitSharingOrder, "SUCCESS", expiresAt); err != nil {
			if requireEnqueueSuccess {
				return err
			}
			log.Warn().Err(err).Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("distribute rider profit sharing success notification failed")
		}

	case "CLOSED", "FAILED":
		if processor.distributor == nil && requireEnqueueSuccess {
			return fmt.Errorf("task distributor not configured for profit sharing retry")
		}

		// 分账失败，通知运营人员
		log.Error().
			Int64("profit_sharing_order_id", payload.ProfitSharingOrderID).
			Str("out_order_no", payload.OutOrderNo).
			Str("fail_reason", payload.FailReason).
			Int64("merchant_id", payload.MerchantID).
			Str("alert_type", "PROFIT_SHARING_FAILED").
			Msg("⚠️ ALERT: profit sharing failed - manual review required")

		// 发送告警给平台运营人员
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeProfitSharingFailed,
			Level:       AlertLevelCritical,
			Title:       "分账失败",
			Message:     fmt.Sprintf("分账单 %s 分账失败，原因：%s，需要人工介入处理", payload.OutOrderNo, payload.FailReason),
			RelatedID:   payload.ProfitSharingOrderID,
			RelatedType: "profit_sharing_order",
			Extra: profitSharingOrderAlertExtra(profitSharingOrder, map[string]interface{}{
				"fail_reason": payload.FailReason,
				"result":      payload.Result,
			}),
		})
		var riderNotificationErr error
		if processor.distributor != nil {
			expiresAt := profitSharingResultNotificationExpiresAt(profitSharingOrder)
			if err := processor.distributeRiderProfitSharingResultNotification(ctx, profitSharingOrder, payload.Result, expiresAt); err != nil {
				riderNotificationErr = err
				log.Warn().Err(err).Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("distribute rider profit sharing failure notification failed")
			}
		}

		if riderNotificationErr != nil && requireEnqueueSuccess {
			return riderNotificationErr
		}

	default:
		if !requireEnqueueSuccess {
			log.Warn().
				Int64("profit_sharing_order_id", payload.ProfitSharingOrderID).
				Str("result", payload.Result).
				Msg("unsupported profit sharing result, skip legacy result task")
			return nil
		}
		return fmt.Errorf("unsupported profit sharing result %q", payload.Result)
	}

	return nil
}

func (processor *RedisTaskProcessor) distributeRiderProfitSharingResultNotification(ctx context.Context, profitSharingOrder db.ProfitSharingOrder, result string, expiresAt time.Time) error {
	if processor.distributor == nil || !profitSharingOrder.RiderID.Valid || profitSharingOrder.RiderAmount <= 0 {
		return nil
	}

	rider, err := processor.store.GetRider(ctx, profitSharingOrder.RiderID.Int64)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("rider_id", profitSharingOrder.RiderID.Int64).Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("skip rider profit sharing notification: rider not found")
			return nil
		}
		return fmt.Errorf("get rider for rider profit sharing notification: %w", err)
	}
	if rider.UserID <= 0 {
		log.Warn().Int64("rider_id", rider.ID).Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("skip rider profit sharing notification: rider user id empty")
		return nil
	}

	title := "代取费已到账"
	content := fmt.Sprintf("本单代取费%.2f元已通过微信分账到账，可在收入账本查看。", float64(profitSharingOrder.RiderAmount)/100)
	if result != "SUCCESS" {
		title = "代取费结算处理中"
		content = "本单代取费结算暂未完成，平台正在核对处理，可在收入账本查看状态。"
	}

	err = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:            rider.UserID,
		Type:              "finance",
		Title:             title,
		Content:           content,
		RelatedType:       "profit_sharing_order",
		RelatedID:         profitSharingOrder.ID,
		IgnorePreferences: true,
		ExtraData: map[string]any{
			"profit_sharing_order_id": profitSharingOrder.ID,
			"rider_amount":            profitSharingOrder.RiderAmount,
			"result":                  result,
		},
		ExpiresAt: &expiresAt,
	}, asynq.Unique(profitSharingResultNotificationDedupWindow))
	if err != nil {
		return fmt.Errorf("enqueue rider profit sharing notification: %w", err)
	}
	return nil
}
