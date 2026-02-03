package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// Redis 告警频道
const AlertChannel = "notification:platform:alerts"

// AlertType 告警类型
type AlertType string

const (
	AlertTypePaymentTimeout      AlertType = "PAYMENT_TIMEOUT"
	AlertTypeTaskEnqueueFailure  AlertType = "TASK_ENQUEUE_FAILURE"
	AlertTypeProfitSharingFailed AlertType = "PROFIT_SHARING_FAILED"
	AlertTypeRefundFailed        AlertType = "REFUND_FAILED"
	AlertTypeSystemError         AlertType = "SYSTEM_ERROR"
)

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
	if processor.redisClient == nil {
		log.Warn().Msg("redis client not configured, cannot publish alert")
		return
	}

	alert.Timestamp = time.Now()

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

	if err := processor.redisClient.Publish(ctx, AlertChannel, wsMessageJSON).Err(); err != nil {
		log.Error().Err(err).Str("alert_type", string(alert.AlertType)).Msg("failed to publish alert to redis")
	} else {
		log.Info().
			Str("alert_type", string(alert.AlertType)).
			Str("level", string(alert.Level)).
			Str("title", alert.Title).
			Msg("alert published to Redis")
	}
}

// 任务类型常量
const (
	TaskProcessPaymentSuccess      = "payment:process_success"
	TaskProcessRefund              = "payment:initiate_refund"
	TaskProcessRefundResult        = "payment:process_refund"
	TaskProcessProfitSharing       = "payment:process_profit_sharing"
	TaskProcessApplymentResult     = "payment:process_applyment_result"      // 进件结果处理
	TaskProcessProfitSharingResult = "payment:process_profit_sharing_result" // 分账结果处理
)

// PaymentSuccessPayload 支付成功任务载荷
type PaymentSuccessPayload struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	TransactionID  string `json:"transaction_id"`
	BusinessType   string `json:"business_type"`
}

// PayloadProcessRefund 发起退款任务载荷
type PayloadProcessRefund struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	OrderID        int64  `json:"order_id"`
	RefundAmount   int64  `json:"refund_amount"`
	Reason         string `json:"reason"`
}

// RefundResultPayload 退款结果任务载荷
type RefundResultPayload struct {
	OutRefundNo  string `json:"out_refund_no"`
	RefundStatus string `json:"refund_status"` // SUCCESS/ABNORMAL/CLOSED
	RefundID     string `json:"refund_id"`
}

// ProfitSharingPayload 分账任务载荷
type ProfitSharingPayload struct {
	PaymentOrderID int64 `json:"payment_order_id"`
	OrderID        int64 `json:"order_id"`
}

// ApplymentResultPayload 进件结果处理任务载荷
type ApplymentResultPayload struct {
	ApplymentID    int64  `json:"applyment_id"`    // 进件记录ID
	OutRequestNo   string `json:"out_request_no"`  // 业务申请编号
	ApplymentState string `json:"applyment_state"` // 进件状态
	SubMchID       string `json:"sub_mch_id"`      // 二级商户号（开户成功时返回）
	SubjectType    string `json:"subject_type"`    // 主体类型：merchant/rider
	SubjectID      int64  `json:"subject_id"`      // 主体ID（商户ID或骑手ID）
}

// ProfitSharingResultPayload 分账结果处理任务载荷
type ProfitSharingResultPayload struct {
	ProfitSharingOrderID int64  `json:"profit_sharing_order_id"` // 分账订单ID
	OutOrderNo           string `json:"out_order_no"`            // 商户分账单号
	Result               string `json:"result"`                  // 分账结果：SUCCESS/CLOSED/FAILED
	FailReason           string `json:"fail_reason"`             // 失败原因
	MerchantID           int64  `json:"merchant_id"`             // 商户ID
}

// DistributeTaskProcessPaymentSuccess 分发支付成功处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessPaymentSuccess(
	ctx context.Context,
	payload *PaymentSuccessPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessPaymentSuccess, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_order_id", payload.PaymentOrderID).
		Str("business_type", payload.BusinessType).
		Msg("enqueued task")

	return nil
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

// DistributeTaskProcessProfitSharing 分发分账处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessProfitSharing(
	ctx context.Context,
	payload *ProfitSharingPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessProfitSharing, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("order_id", payload.OrderID).
		Msg("enqueued task")

	return nil
}

// DistributeTaskProcessApplymentResult 分发进件结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessApplymentResult(
	ctx context.Context,
	payload *ApplymentResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessApplymentResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("applyment_id", payload.ApplymentID).
		Str("applyment_state", payload.ApplymentState).
		Msg("enqueued applyment result task")

	return nil
}

// DistributeTaskProcessProfitSharingResult 分发分账结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessProfitSharingResult(
	ctx context.Context,
	payload *ProfitSharingResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessProfitSharingResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("profit_sharing_order_id", payload.ProfitSharingOrderID).
		Str("result", payload.Result).
		Msg("enqueued profit sharing result task")

	return nil
}

// ProcessTaskPaymentSuccess 处理支付成功任务
func (processor *RedisTaskProcessor) ProcessTaskPaymentSuccess(ctx context.Context, task *asynq.Task) error {
	var payload PaymentSuccessPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Str("business_type", payload.BusinessType).
		Msg("processing payment success")

	result, err := processor.store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payload.PaymentOrderID,
		RiderAverageSpeed:  processor.config.RiderAverageSpeed,
		DefaultPrepareTime: processor.config.DefaultPrepareTime,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Error().Int64("payment_order_id", payload.PaymentOrderID).Msg("payment order not found")
			return fmt.Errorf("payment order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("process payment success: %w", err)
	}

	if !result.Processed {
		log.Info().
			Int64("payment_order_id", payload.PaymentOrderID).
			Msg("payment order already processed or not eligible, skip")
		return nil
	}

	paymentOrder := result.PaymentOrder

	// 订单支付成功后，需要触发分账与通知
	if paymentOrder.BusinessType == "order" && result.OrderResult != nil {
		processor.sendOrderPaidNotifications(ctx, *result.OrderResult)
		if paymentOrder.PaymentType == "profit_sharing" && paymentOrder.OrderID.Valid {
			return processor.distributor.DistributeTaskProcessProfitSharing(ctx, &ProfitSharingPayload{
				PaymentOrderID: paymentOrder.ID,
				OrderID:        paymentOrder.OrderID.Int64,
			})
		}
	}

	// 预定支付成功后，创建未到店提醒任务 (预定时间后30分钟)
	if paymentOrder.BusinessType == "reservation" && paymentOrder.ReservationID.Valid {
		res, err := processor.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err == nil {
			// 计算提醒时间：预定日期 + 预定时间
			hours := res.ReservationTime.Microseconds / 1000000 / 3600
			minutes := (res.ReservationTime.Microseconds / 1000000 % 3600) / 60
			alertTime := time.Date(
				res.ReservationDate.Time.Year(), res.ReservationDate.Time.Month(), res.ReservationDate.Time.Day(),
				int(hours), int(minutes), 0, 0, time.Local,
			)

			_ = processor.distributor.DistributeTaskReservationNoShowAlert(
				ctx,
				&PayloadReservationNoShowAlert{ReservationID: res.ID},
				asynq.ProcessAt(alertTime),
			)
		}
	}

	return nil
}

// sendOrderPaidNotifications 发送订单支付成功的实时通知
func (processor *RedisTaskProcessor) sendOrderPaidNotifications(ctx context.Context, result db.ProcessOrderPaymentTxResult) {
	// 1. 通知商户：有新订单
	processor.notifyMerchantNewOrder(ctx, result.Order)

	// 2. 如果是外卖订单，通知区域内骑手：订单池有新单
	if result.Delivery != nil && result.PoolItem != nil {
		processor.notifyRidersNewDelivery(ctx, result.Order, result.Delivery, result.PoolItem)
	}
}

// notifyMerchantNewOrder 通知商户有新订单
func (processor *RedisTaskProcessor) notifyMerchantNewOrder(ctx context.Context, order db.Order) {
	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", order.MerchantID).Msg("get merchant for notification failed")
		return
	}

	// 通过异步任务发送通知给商户
	expiresAt := time.Now().Add(24 * time.Hour)
	err = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:      merchant.OwnerUserID,
		Type:        "order",
		Title:       "🆕 新订单",
		Content:     fmt.Sprintf("您有一笔新订单 %s，请及时处理", order.OrderNo),
		RelatedType: "order",
		RelatedID:   order.ID,
		ExtraData: map[string]any{
			"order_no":     order.OrderNo,
			"order_type":   order.OrderType,
			"total_amount": order.TotalAmount,
		},
		ExpiresAt: &expiresAt,
	}, asynq.Queue(QueueDefault))

	if err != nil {
		log.Error().Err(err).Int64("order_id", order.ID).Msg("distribute merchant notification task failed")
	} else {
		log.Info().
			Int64("order_id", order.ID).
			Int64("merchant_id", merchant.ID).
			Str("order_no", order.OrderNo).
			Msg("✅ merchant new order notification task distributed")
	}

	items, itemsErr := processor.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if itemsErr != nil {
		log.Warn().Err(itemsErr).Int64("order_id", order.ID).Msg("load order items for ws snapshot failed")
	}
	orderSnapshot := buildOrderSnapshotPayload(order, items)
	payload, _ := json.Marshal(orderSnapshot)
	wsMessage := websocket.Message{
		Type:      "new_order",
		Data:      json.RawMessage(payload),
		Timestamp: time.Now(),
	}
	pushMsg := websocket.NotificationPushMessage{
		EntityType: "merchant",
		EntityID:   merchant.ID,
		Message:    wsMessage,
	}
	wsMessageJSON, _ := json.Marshal(pushMsg)
	channel := fmt.Sprintf("notification:merchant:%d", merchant.ID)
	if err := processor.redisClient.Publish(ctx, channel, wsMessageJSON).Err(); err != nil {
		log.Error().Err(err).Int64("merchant_id", merchant.ID).Msg("publish new order ws message failed")
	}
}

type orderItemSnapshot struct {
	ID             int64       `json:"id"`
	Name           string      `json:"name"`
	UnitPrice      int64       `json:"unit_price"`
	Quantity       int16       `json:"quantity"`
	Subtotal       int64       `json:"subtotal"`
	DishID         *int64      `json:"dish_id,omitempty"`
	ComboID        *int64      `json:"combo_id,omitempty"`
	ImageURL       *string     `json:"image_url,omitempty"`
	Customizations interface{} `json:"customizations,omitempty"`
}

func buildOrderSnapshotPayload(order db.Order, items []db.ListOrderItemsWithDishByOrderRow) map[string]any {
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
		for i, item := range items {
			respItems[i] = orderItemSnapshot{
				ID:        item.ID,
				Name:      item.Name,
				UnitPrice: item.UnitPrice,
				Quantity:  item.Quantity,
				Subtotal:  item.Subtotal,
			}
			if item.DishID.Valid {
				respItems[i].DishID = &item.DishID.Int64
			}
			if item.ComboID.Valid {
				respItems[i].ComboID = &item.ComboID.Int64
			}
			if item.DishImageUrl.Valid {
				img := item.DishImageUrl.String
				respItems[i].ImageURL = &img
			}
			if item.Customizations != nil {
				var customizations interface{}
				_ = json.Unmarshal(item.Customizations, &customizations)
				respItems[i].Customizations = customizations
			}
		}
		payload["items"] = respItems
	}

	return payload
}

// notifyRidersNewDelivery 通知附近骑手有新配送订单
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

	// 推送策略：100m起步，每次+100m，超过1000m改为全区县推送
	const (
		startDistance   = 100.0  // 起始距离100米
		stepDistance    = 100.0  // 每次扩大100米
		regionThreshold = 1000.0 // 超过1000米改为全区县推送
		minRiderCount   = 3      // 最少通知骑手数
	)

	var ridersToNotify []int64
	var usedDistance float64
	var isRegionBroadcast bool

	// 阶段1：按距离递增查找附近骑手（100m -> 200m -> ... -> 1000m）
	for distance := startDistance; distance <= regionThreshold; distance += stepDistance {
		riders, err := processor.store.ListNearbyRiders(ctx, db.ListNearbyRidersParams{
			CenterLat:   pickupLat.Float64,
			CenterLng:   pickupLng.Float64,
			MaxDistance: distance,
			LimitCount:  1000, // 不限制数量
		})
		if err != nil {
			log.Error().Err(err).Float64("distance", distance).Msg("list nearby riders failed")
			continue
		}

		usedDistance = distance
		for _, r := range riders {
			ridersToNotify = append(ridersToNotify, r.ID)
		}

		// 如果找到足够骑手，停止扩大范围
		if len(riders) >= minRiderCount {
			break
		}
	}

	// 阶段2：如果1000米内骑手不足，改为全区县推送
	if len(ridersToNotify) < minRiderCount {
		regionRiders, err := processor.store.ListOnlineRidersByRegion(ctx, pgtype.Int8{Int64: merchant.RegionID, Valid: true})
		if err != nil {
			log.Error().Err(err).Int64("region_id", merchant.RegionID).Msg("list region riders failed")
		} else {
			ridersToNotify = nil // 清空，使用区域骑手
			for _, r := range regionRiders {
				ridersToNotify = append(ridersToNotify, r.ID)
			}
			isRegionBroadcast = true
		}
	}

	if len(ridersToNotify) == 0 {
		log.Warn().
			Int64("order_id", order.ID).
			Int64("region_id", merchant.RegionID).
			Msg("no online riders in region, order waiting in pool")
		return
	}

	// 构建完整的新订单池消息数据（使用标准类型常量）
	if processor.deliveryBroadcast != nil {
		_ = processor.deliveryBroadcast.BroadcastNewOrderNotification(ctx, merchant.RegionID, *poolItem, merchant.Name)
	} else {
		// 回退方案（保持兼容）
		newOrderData := map[string]any{
			"type":                 "new_delivery_order",
			"order_id":             order.ID,
			"delivery_id":          delivery.ID,
			"merchant_id":          merchant.ID,
			"merchant_name":        merchant.Name,
			"pickup_address":       delivery.PickupAddress,
			"pickup_longitude":     pickupLng.Float64,
			"pickup_latitude":      pickupLat.Float64,
			"delivery_address":     delivery.DeliveryAddress,
			"delivery_longitude":   deliveryLng.Float64,
			"delivery_latitude":    deliveryLat.Float64,
			"delivery_fee":         order.DeliveryFee,
			"distance":             poolItem.Distance,
			"priority":             poolItem.Priority,
			"expected_pickup_at":   poolItem.ExpectedPickupAt,
			"expected_delivery_at": delivery.EstimatedDeliveryAt.Time,
			"is_high_value":        order.DeliveryFee >= 1000,
			"created_at":           poolItem.CreatedAt,
		}
		msgData, _ := json.Marshal(newOrderData)

		for _, riderID := range ridersToNotify {
			pushMsg := websocket.NotificationPushMessage{
				EntityType: "rider",
				EntityID:   riderID,
				Message: websocket.Message{
					Type:      "delivery_pool_update",
					Data:      json.RawMessage(msgData),
					Timestamp: time.Now(),
				},
			}
			wsMessageJSON, _ := json.Marshal(pushMsg)
			channel := fmt.Sprintf("notification:rider:%d", riderID)
			_ = processor.redisClient.Publish(ctx, channel, wsMessageJSON).Err()
		}
	}

	log.Info().
		Int64("order_id", order.ID).
		Int64("delivery_id", delivery.ID).
		Float64("search_radius_m", usedDistance).
		Bool("region_broadcast", isRegionBroadcast).
		Int64("region_id", merchant.RegionID).
		Int64("delivery_fee", order.DeliveryFee).
		Bool("is_high_value", order.DeliveryFee >= 1000).
		Msg("✅ new delivery order pushed to riders")
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
			log.Error().Str("out_refund_no", payload.OutRefundNo).Msg("refund order not found")
			return fmt.Errorf("refund order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get refund order: %w", err)
	}

	// 根据退款状态更新
	switch payload.RefundStatus {
	case "SUCCESS":
		_, err = processor.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to success: %w", err)
		}

		// ✅ P1-3: 同步更新支付订单状态为已退款
		_, err = processor.store.UpdatePaymentOrderToRefunded(ctx, refundOrder.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("update payment order to refunded: %w", err)
		}

		log.Info().
			Str("out_refund_no", payload.OutRefundNo).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Msg("refund success and payment order status synced")

		// 🔔 发送退款成功通知
		if processor.distributor != nil {
			paymentOrder, err := processor.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
			if err == nil {
				expiresAt := time.Now().Add(7 * 24 * time.Hour)
				_ = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
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
				}, asynq.Queue(QueueDefault))
			}
		}

	case "ABNORMAL":
		_, err = processor.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to failed: %w", err)
		}
		log.Warn().Str("out_refund_no", payload.OutRefundNo).Msg("refund abnormal")

		// ⚠️ 需通知运营人工介入（待实现）
		log.Error().
			Str("out_refund_no", payload.OutRefundNo).
			Str("refund_id", payload.RefundID).
			Str("status", "ABNORMAL").
			Str("action", "notify_operations_team").
			Msg("[ALERT] refund abnormal - operations team intervention required")

	case "CLOSED":
		_, err = processor.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to closed: %w", err)
		}
		log.Info().Str("out_refund_no", payload.OutRefundNo).Msg("refund closed")
	}

	return nil
}

// ProcessTaskProfitSharing 处理分账任务
func (processor *RedisTaskProcessor) ProcessTaskProfitSharing(ctx context.Context, task *asynq.Task) error {
	var payload ProfitSharingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("order_id", payload.OrderID).
		Msg("processing profit sharing")

	// 获取支付订单
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("payment order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get payment order: %w", err)
	}

	// 必须有微信交易号才能分账
	if !paymentOrder.TransactionID.Valid || paymentOrder.TransactionID.String == "" {
		return fmt.Errorf("transaction_id is required for profit sharing: %w", asynq.SkipRetry)
	}

	// 获取订单信息
	order, err := processor.store.GetOrder(ctx, payload.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get order: %w", err)
	}

	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant: %w", err)
	}

	// 获取商户支付配置（从新表 merchant_payment_configs）
	paymentConfig, err := processor.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("merchant_id", order.MerchantID).Msg("merchant payment config not found, skip profit sharing")
			return nil // 商户未配置微信支付，跳过分账
		}
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	// 获取运营商信息（根据配送地址所在区域）
	var operator db.Operator
	var hasOperator bool
	regionID := merchant.RegionID
	var operatorCommission int64
	var platformCommission int64
	merchantAmount := order.TotalAmount

	// 获取配送地址的区域ID（优先使用配送地址，否则回退商户区域）
	if order.AddressID.Valid {
		address, err := processor.store.GetUserAddress(ctx, order.AddressID.Int64)
		if err == nil && address.RegionID > 0 {
			regionID = address.RegionID
		}
	}

	config, err := processor.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: order.MerchantID, Valid: true},
		RegionID:    pgtype.Int8{Int64: regionID, Valid: regionID > 0},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("profit sharing config not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get profit sharing config: %w", err)
	}

	platformRate := config.PlatformRate
	operatorRate := config.OperatorRate
	riderEnabled := config.RiderEnabled

	// 查找运营商
	if regionID > 0 {
		op, err := processor.store.GetOperatorByRegion(ctx, regionID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("get operator: %w", err)
		}

		if err == nil {
			operator = op
			hasOperator = true
		}
	}

	// 计算分账金额（单位：分）
	platformCommission = order.TotalAmount * int64(platformRate) / 100
	if hasOperator {
		operatorCommission = order.TotalAmount * int64(operatorRate) / 100
	}
	merchantAmount = order.TotalAmount - platformCommission - operatorCommission
	needProfitSharing := platformCommission > 0 || operatorCommission > 0

	// 若已有分账记录，复用并尝试重试
	existingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return fmt.Errorf("get profit sharing order by payment order: %w", err)
	}

	var profitSharingOrder db.ProfitSharingOrder
	var outOrderNo string
	if err == nil {
		if existingOrder.Status == "finished" || existingOrder.Status == "processing" {
			log.Info().
				Int64("profit_sharing_order_id", existingOrder.ID).
				Str("status", existingOrder.Status).
				Msg("profit sharing already processed, skip")
			return nil
		}

		profitSharingOrder = existingOrder
		outOrderNo = existingOrder.OutOrderNo
		platformCommission = existingOrder.PlatformCommission
		operatorCommission = existingOrder.OperatorCommission
		merchantAmount = existingOrder.MerchantAmount

		if existingOrder.OperatorID.Valid {
			op, err := processor.store.GetOperator(ctx, existingOrder.OperatorID.Int64)
			if err == nil {
				operator = op
				hasOperator = true
			}
		}
	} else {
		// 创建分账订单记录
		outOrderNo = fmt.Sprintf("PS%d%d", payload.PaymentOrderID, payload.OrderID)
		var operatorID pgtype.Int8
		if hasOperator {
			operatorID = pgtype.Int8{Int64: operator.ID, Valid: true}
		}

		riderAmount := int64(0)
		if riderEnabled && order.DeliveryFee > 0 {
			riderAmount = order.DeliveryFee
		}
		profitSharingOrder, err = processor.store.CreateProfitSharingOrder(ctx, db.CreateProfitSharingOrderParams{
			PaymentOrderID:      payload.PaymentOrderID,
			MerchantID:          order.MerchantID,
			OperatorID:          operatorID,
			OrderSource:         order.OrderType,
			TotalAmount:         order.TotalAmount,
			DeliveryFee:         order.DeliveryFee,
			RiderID:             pgtype.Int8{},
			RiderAmount:         riderAmount,
			DistributableAmount: order.TotalAmount,
			PlatformRate:        platformRate,
			OperatorRate:        operatorRate,
			PlatformCommission:  platformCommission,
			OperatorCommission:  operatorCommission,
			MerchantAmount:      merchantAmount,
			OutOrderNo:          outOrderNo,
			Status:              "pending",
		})
		if err != nil {
			return fmt.Errorf("create profit sharing order: %w", err)
		}
	}

	log.Info().
		Int64("order_id", payload.OrderID).
		Str("merchant_name", merchant.Name).
		Int64("total_amount", order.TotalAmount).
		Int64("platform_commission", platformCommission).
		Int64("operator_commission", operatorCommission).
		Int64("merchant_amount", merchantAmount).
		Int32("platform_rate", platformRate).
		Int32("operator_rate", operatorRate).
		Bool("need_profit_sharing", needProfitSharing).
		Msg("profit sharing order created")

	// 如果不需要分账（堂食/打包），直接完结分账
	if !needProfitSharing || (platformCommission == 0 && operatorCommission == 0) {
		_, err = processor.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrder.ID)
		if err != nil {
			return fmt.Errorf("update profit sharing order to finished: %w", err)
		}
		log.Info().Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("no profit sharing needed, marked as finished")
		return nil
	}

	// 检查是否配置了平台收付通客户端
	if processor.ecommerceClient == nil {
		log.Error().Msg("ecommerce client not configured, cannot process profit sharing")
		return fmt.Errorf("ecommerce client not configured: %w", asynq.SkipRetry)
	}

	// 构建分账接收方列表
	var receivers []wechat.ProfitSharingReceiver

	// 平台佣金（进入服务商账户）
	if platformCommission > 0 {
		receivers = append(receivers, wechat.ProfitSharingReceiver{
			Type:            "MERCHANT_ID",
			ReceiverAccount: processor.ecommerceClient.GetSpMchID(), // 服务商商户号
			Amount:          platformCommission,
			Description:     "平台服务费",
		})
	}

	// 运营商佣金
	if hasOperator && operatorCommission > 0 && operator.WechatMchID.Valid {
		receivers = append(receivers, wechat.ProfitSharingReceiver{
			Type:            "MERCHANT_ID",
			ReceiverAccount: operator.WechatMchID.String,
			Amount:          operatorCommission,
			Description:     "运营商服务费",
		})
	}

	// 如果没有接收方，直接完结
	if len(receivers) == 0 {
		_, err = processor.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrder.ID)
		if err != nil {
			return fmt.Errorf("update profit sharing order to finished: %w", err)
		}
		log.Info().Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("no receivers, marked as finished")
		return nil
	}

	// 调用微信分账 API
	resp, err := processor.ecommerceClient.CreateProfitSharing(ctx, &wechat.ProfitSharingRequest{
		SubMchID:      paymentConfig.SubMchID, // 商户二级商户号
		TransactionID: paymentOrder.TransactionID.String,
		OutOrderNo:    outOrderNo,
		Receivers:     receivers,
		Finish:        true, // 分账完成后剩余资金解冻给商户
	})
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", outOrderNo).
			Msg("call wechat profit sharing API failed")
		return fmt.Errorf("create profit sharing: %w", err)
	}

	// 更新分账订单状态为处理中
	_, err = processor.store.UpdateProfitSharingOrderToProcessing(ctx, db.UpdateProfitSharingOrderToProcessingParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: pgtype.Text{String: resp.OrderID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update profit sharing order to processing: %w", err)
	}

	log.Info().
		Int64("profit_sharing_order_id", profitSharingOrder.ID).
		Str("wechat_order_id", resp.OrderID).
		Str("status", resp.Status).
		Msg("profit sharing request sent")

	return nil
}

// ProcessTaskInitiateRefund 处理发起退款任务
func (processor *RedisTaskProcessor) ProcessTaskInitiateRefund(ctx context.Context, task *asynq.Task) error {
	var payload PayloadProcessRefund
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Int64("order_id", payload.OrderID).
		Int64("refund_amount", payload.RefundAmount).
		Str("reason", payload.Reason).
		Msg("processing refund task")

	// 获取支付订单
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	// 检查支付订单状态
	if paymentOrder.Status != "paid" {
		log.Warn().
			Int64("payment_order_id", payload.PaymentOrderID).
			Str("status", paymentOrder.Status).
			Msg("payment order not in paid status, skip refund")
		return nil
	}

	// 获取订单以获取商户信息
	order, err := processor.store.GetOrder(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	// 获取商户支付配置
	paymentConfig, err := processor.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("merchant payment config not found")
		}
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	// 生成退款单号
	outRefundNo := fmt.Sprintf("RF%d%d", payload.PaymentOrderID, payload.OrderID)

	// 创建退款记录
	refundOrder, err := processor.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: payload.PaymentOrderID,
		RefundType:     "user_cancel",
		RefundAmount:   payload.RefundAmount,
		RefundReason:   pgtype.Text{String: payload.Reason, Valid: true},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		return fmt.Errorf("create refund order: %w", err)
	}

	// 检查是否有微信支付客户端
	if processor.ecommerceClient == nil {
		log.Error().Msg("ecommerce client not configured, cannot process refund")
		return fmt.Errorf("ecommerce client not configured")
	}

	// 调用微信退款 API
	refundResp, err := processor.ecommerceClient.CreateEcommerceRefund(ctx, &wechat.EcommerceRefundRequest{
		SubMchID:     paymentConfig.SubMchID,
		OutTradeNo:   paymentOrder.OutTradeNo,
		OutRefundNo:  outRefundNo,
		Reason:       payload.Reason,
		RefundAmount: payload.RefundAmount,
		TotalAmount:  paymentOrder.Amount,
	})
	if err != nil {
		// 更新退款状态为失败
		processor.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return fmt.Errorf("call wechat refund API: %w", err)
	}

	// 根据微信返回状态更新退款订单
	switch refundResp.Status {
	case wechat.RefundStatusSuccess:
		processor.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
		// 更新支付订单状态为已退款
		processor.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
	case wechat.RefundStatusProcessing:
		// 更新退款单为处理中
		processor.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundResp.RefundID, Valid: true},
		})
	default:
		// 其他状态标记为失败
		processor.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
	}

	log.Info().
		Int64("refund_order_id", refundOrder.ID).
		Str("out_refund_no", outRefundNo).
		Str("status", string(refundResp.Status)).
		Msg("refund request processed")

	return nil
}

// ==================== 进件结果处理 ====================

// ProcessTaskApplymentResult 处理进件结果任务
// 在进件回调后异步执行：1.发送通知 2.添加分账接收方
func (processor *RedisTaskProcessor) ProcessTaskApplymentResult(ctx context.Context, task *asynq.Task) error {
	var payload ApplymentResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Int64("applyment_id", payload.ApplymentID).
		Str("applyment_state", payload.ApplymentState).
		Str("sub_mch_id", payload.SubMchID).
		Msg("processing applyment result")

	// 根据进件状态处理
	switch payload.ApplymentState {
	case "APPLYMENT_STATE_FINISHED":
		// 进件成功，需要：
		// 1. 发送成功通知
		// 2. 添加为分账接收方
		return processor.handleApplymentSuccess(ctx, &payload)

	case "APPLYMENT_STATE_REJECTED":
		// 进件被驳回，发送通知
		return processor.handleApplymentRejected(ctx, &payload)

	case "APPLYMENT_STATE_TO_BE_CONFIRMED", "APPLYMENT_STATE_TO_BE_SIGNED":
		// 待确认/待签约，发送提醒通知
		return processor.handleApplymentPending(ctx, &payload)

	default:
		log.Info().
			Str("state", payload.ApplymentState).
			Msg("applyment state does not require async processing")
		return nil
	}
}

// handleApplymentSuccess 处理进件成功
func (processor *RedisTaskProcessor) handleApplymentSuccess(ctx context.Context, payload *ApplymentResultPayload) error {
	// 1. 添加为分账接收方（关键步骤！）
	if processor.ecommerceClient != nil && payload.SubMchID != "" {
		_, err := processor.ecommerceClient.AddProfitSharingReceiver(ctx, &wechat.AddReceiverRequest{
			AppID:        processor.ecommerceClient.GetSpAppID(),
			Type:         wechat.ReceiverTypeMerchant,
			Account:      payload.SubMchID,
			RelationType: wechat.RelationStore, // 门店关系
		})
		if err != nil {
			// 添加失败不影响流程，但需要记录告警
			log.Error().Err(err).
				Str("sub_mch_id", payload.SubMchID).
				Str("alert_type", "ADD_RECEIVER_FAILED").
				Msg("⚠️ ALERT: failed to add profit sharing receiver - manual intervention required")
			// 不返回错误，允许继续发送通知
		} else {
			log.Info().
				Str("sub_mch_id", payload.SubMchID).
				Msg("successfully added profit sharing receiver")
		}
	}

	// 2. 发送成功通知
	var userID int64
	var notifyContent string

	switch payload.SubjectType {
	case "merchant":
		merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
		if err != nil {
			log.Error().Err(err).Int64("merchant_id", payload.SubjectID).Msg("get merchant for notification")
			return nil // 不重试
		}
		userID = merchant.OwnerUserID
		notifyContent = fmt.Sprintf("您的商户「%s」已完成微信支付开户，可以开始接单收款了！", merchant.Name)

	case "rider":
		rider, err := processor.store.GetRider(ctx, payload.SubjectID)
		if err != nil {
			log.Error().Err(err).Int64("rider_id", payload.SubjectID).Msg("get rider for notification")
			return nil
		}
		userID = rider.UserID
		notifyContent = "您的骑手账户已完成微信支付开户，可以开始接单了！"
	}

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		_ = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户成功",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		})
	}

	return nil
}

// handleApplymentRejected 处理进件驳回
func (processor *RedisTaskProcessor) handleApplymentRejected(ctx context.Context, payload *ApplymentResultPayload) error {
	// 获取驳回原因
	applyment, err := processor.store.GetEcommerceApplyment(ctx, payload.ApplymentID)
	if err != nil {
		log.Error().Err(err).Int64("applyment_id", payload.ApplymentID).Msg("get applyment")
		return nil
	}

	var userID int64
	var notifyContent string
	rejectReason := "请登录后台查看详情"
	if applyment.RejectReason.Valid {
		rejectReason = applyment.RejectReason.String
	}

	switch payload.SubjectType {
	case "merchant":
		merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
		if err != nil {
			log.Error().Err(err).Int64("merchant_id", payload.SubjectID).Msg("get merchant")
			return nil
		}
		userID = merchant.OwnerUserID
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户申请被驳回，原因：%s", merchant.Name, rejectReason)

	case "rider":
		rider, err := processor.store.GetRider(ctx, payload.SubjectID)
		if err != nil {
			log.Error().Err(err).Int64("rider_id", payload.SubjectID).Msg("get rider")
			return nil
		}
		userID = rider.UserID
		notifyContent = fmt.Sprintf("您的骑手微信支付开户申请被驳回，原因：%s", rejectReason)
	}

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		_ = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户被驳回",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		})
	}

	return nil
}

// handleApplymentPending 处理待确认/待签约
func (processor *RedisTaskProcessor) handleApplymentPending(ctx context.Context, payload *ApplymentResultPayload) error {
	var userID int64
	var notifyContent string

	switch payload.SubjectType {
	case "merchant":
		merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
		if err != nil {
			return nil
		}
		userID = merchant.OwnerUserID
		if payload.ApplymentState == "APPLYMENT_STATE_TO_BE_CONFIRMED" {
			notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户需要确认，请登录微信支付商户平台完成确认", merchant.Name)
		} else {
			notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户需要签约，请登录微信支付商户平台完成签约", merchant.Name)
		}

	case "rider":
		rider, err := processor.store.GetRider(ctx, payload.SubjectID)
		if err != nil {
			return nil
		}
		userID = rider.UserID
		if payload.ApplymentState == "APPLYMENT_STATE_TO_BE_CONFIRMED" {
			notifyContent = "您的骑手微信支付开户需要确认，请登录微信支付商户平台完成确认"
		} else {
			notifyContent = "您的骑手微信支付开户需要签约，请登录微信支付商户平台完成签约"
		}
	}

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(3 * 24 * time.Hour)
		_ = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户待处理",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		})
	}

	return nil
}

// ==================== 分账结果处理 ====================

// ProcessTaskProfitSharingResult 处理分账结果任务
// 在分账回调后异步执行：发送通知
func (processor *RedisTaskProcessor) ProcessTaskProfitSharingResult(ctx context.Context, task *asynq.Task) error {
	var payload ProfitSharingResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

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
		if processor.distributor != nil {
			expiresAt := time.Now().Add(7 * 24 * time.Hour)
			_ = processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
				UserID:      merchant.OwnerUserID,
				Type:        "finance",
				Title:       "订单收入已到账",
				Content:     fmt.Sprintf("您有一笔订单收入%.2f元已到账", float64(profitSharingOrder.MerchantAmount)/100),
				RelatedType: "profit_sharing",
				RelatedID:   payload.ProfitSharingOrderID,
				ExtraData: map[string]any{
					"merchant_amount":     profitSharingOrder.MerchantAmount,
					"platform_commission": profitSharingOrder.PlatformCommission,
					"operator_commission": profitSharingOrder.OperatorCommission,
				},
				ExpiresAt: &expiresAt,
			})
		}

	case "CLOSED", "FAILED":
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
			Extra: map[string]interface{}{
				"out_order_no": payload.OutOrderNo,
				"merchant_id":  payload.MerchantID,
				"fail_reason":  payload.FailReason,
			},
		})

		// 自动重试队列（延迟执行，避免微信端短暂异常导致永久失败）
		if processor.distributor != nil {
			paymentOrder, err := processor.store.GetPaymentOrder(ctx, profitSharingOrder.PaymentOrderID)
			if err == nil && paymentOrder.OrderID.Valid {
				_ = processor.distributor.DistributeTaskProcessProfitSharing(
					ctx,
					&ProfitSharingPayload{
						PaymentOrderID: profitSharingOrder.PaymentOrderID,
						OrderID:        paymentOrder.OrderID.Int64,
					},
					asynq.Queue(QueueCritical),
					asynq.ProcessIn(30*time.Minute),
					asynq.MaxRetry(5),
					asynq.Unique(6*time.Hour),
				)
			}
		}
	}

	return nil
}
