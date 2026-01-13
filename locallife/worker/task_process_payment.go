package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	info, err := distributor.client.EnqueueContext(ctx, task)
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

	// 获取支付订单
	paymentOrder, err := processor.store.GetPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Error().Int64("payment_order_id", payload.PaymentOrderID).Msg("payment order not found")
			return fmt.Errorf("payment order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get payment order: %w", err)
	}

	// 检查是否已处理
	if paymentOrder.Status != "paid" {
		log.Warn().
			Int64("payment_order_id", payload.PaymentOrderID).
			Str("status", paymentOrder.Status).
			Msg("payment order not in paid status, skip")
		return nil
	}

	// 根据业务类型执行后续逻辑
	switch payload.BusinessType {
	case "rider_deposit":
		return processor.handleRiderDepositPaid(ctx, paymentOrder)

	case "reservation":
		return processor.handleReservationPaid(ctx, paymentOrder)

	case "reservation_addon":
		return processor.handleReservationAddonPaid(ctx, paymentOrder)

	case "membership_recharge":
		return processor.handleMembershipRechargePaid(ctx, paymentOrder)

	case "order":
		// 订单支付成功后，需要触发分账
		if err := processor.handleOrderPaid(ctx, paymentOrder); err != nil {
			return err
		}
		// 如果是收付通分账类型，触发分账任务
		if paymentOrder.PaymentType == "profit_sharing" && paymentOrder.OrderID.Valid {
			return processor.distributor.DistributeTaskProcessProfitSharing(ctx, &ProfitSharingPayload{
				PaymentOrderID: paymentOrder.ID,
				OrderID:        paymentOrder.OrderID.Int64,
			})
		}
		return nil

	default:
		log.Warn().
			Str("business_type", payload.BusinessType).
			Int64("payment_order_id", payload.PaymentOrderID).
			Msg("unknown business type")
		return nil
	}
}

// handleRiderDepositPaid 处理骑手押金支付成功
func (processor *RedisTaskProcessor) handleRiderDepositPaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	// 获取骑手信息
	rider, err := processor.store.GetRiderByUserID(ctx, paymentOrder.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Error().Int64("user_id", paymentOrder.UserID).Msg("rider not found")
			return fmt.Errorf("rider not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get rider: %w", err)
	}

	// 计算新余额
	newBalance := rider.DepositAmount + paymentOrder.Amount

	// 更新骑手押金
	_, err = processor.store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: newBalance,
		FrozenDeposit: rider.FrozenDeposit,
	})
	if err != nil {
		return fmt.Errorf("update rider deposit: %w", err)
	}

	// 创建押金流水
	_, err = processor.store.CreateRiderDeposit(ctx, db.CreateRiderDepositParams{
		RiderID:      rider.ID,
		Amount:       paymentOrder.Amount,
		Type:         "deposit",
		BalanceAfter: newBalance,
		Remark:       pgtype.Text{String: "微信支付充值", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create rider deposit record: %w", err)
	}

	log.Info().
		Int64("rider_id", rider.ID).
		Int64("amount", paymentOrder.Amount).
		Int64("new_balance", newBalance).
		Msg("rider deposit charged")

	return nil
}

// handleReservationPaid 处理预定支付成功
func (processor *RedisTaskProcessor) handleReservationPaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if !paymentOrder.ReservationID.Valid {
		return fmt.Errorf("reservation_id is required: %w", asynq.SkipRetry)
	}

	// 更新预定状态为已支付
	_, err := processor.store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     paymentOrder.ReservationID.Int64,
		Status: "paid",
	})
	if err != nil {
		return fmt.Errorf("update reservation status: %w", err)
	}

	log.Info().
		Int64("reservation_id", paymentOrder.ReservationID.Int64).
		Int64("amount", paymentOrder.Amount).
		Msg("reservation paid")

	// 🔔 需发送通知（待实现）
	log.Info().
		Int64("reservation_id", paymentOrder.ReservationID.Int64).
		Int64("user_id", paymentOrder.UserID).
		Str("action", "send_reservation_notification").
		Msg("[NOTIFICATION] reservation success - user and merchant notification pending")

	return nil
}

// handleReservationAddonPaid 处理全款预订追加菜品支付成功：仅累加预付金额，状态不变
func (processor *RedisTaskProcessor) handleReservationAddonPaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if !paymentOrder.ReservationID.Valid {
		return fmt.Errorf("reservation_id is required: %w", asynq.SkipRetry)
	}

	reservationID := paymentOrder.ReservationID.Int64

	updated, err := processor.store.AddReservationPrepaidAmount(ctx, db.AddReservationPrepaidAmountParams{
		ID:     reservationID,
		PrepaidAmount: paymentOrder.Amount,
	})
	if err != nil {
		return fmt.Errorf("add reservation prepaid amount: %w", err)
	}

	log.Info().
		Int64("reservation_id", reservationID).
		Int64("added_amount", paymentOrder.Amount).
		Int64("prepaid_total", updated.PrepaidAmount).
		Msg("reservation addon paid and prepaid amount updated")

	// 🔔 需发送通知（待实现）
	log.Info().
		Int64("reservation_id", reservationID).
		Int64("user_id", paymentOrder.UserID).
		Str("action", "send_reservation_addon_notification").
		Msg("[NOTIFICATION] reservation addon paid - notification pending")

	return nil
}

// handleMembershipRechargePaid 处理会员充值支付成功
func (processor *RedisTaskProcessor) handleMembershipRechargePaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	// 从attach字段解析充值参数
	if !paymentOrder.Attach.Valid || paymentOrder.Attach.String == "" {
		return fmt.Errorf("attach data is missing: %w", asynq.SkipRetry)
	}

	var attachData struct {
		MembershipID   int64  `json:"membership_id"`
		BonusAmount    int64  `json:"bonus_amount"`
		RechargeRuleID *int64 `json:"recharge_rule_id"`
	}

	if err := json.Unmarshal([]byte(paymentOrder.Attach.String), &attachData); err != nil {
		return fmt.Errorf("parse attach data: %w", asynq.SkipRetry)
	}

	// 执行充值事务
	result, err := processor.store.RechargeTx(ctx, db.RechargeTxParams{
		MembershipID:   attachData.MembershipID,
		RechargeAmount: paymentOrder.Amount,
		BonusAmount:    attachData.BonusAmount,
		RechargeRuleID: attachData.RechargeRuleID,
		Notes:          fmt.Sprintf("微信支付充值，订单号：%s", paymentOrder.OutTradeNo),
	})
	if err != nil {
		return fmt.Errorf("recharge membership: %w", err)
	}

	log.Info().
		Int64("membership_id", attachData.MembershipID).
		Int64("recharge_amount", paymentOrder.Amount).
		Int64("bonus_amount", attachData.BonusAmount).
		Int64("new_balance", result.Membership.Balance).
		Msg("membership recharged")

	// 🔔 需发送通知（待实现）
	log.Info().
		Int64("user_id", paymentOrder.UserID).
		Int64("membership_id", attachData.MembershipID).
		Str("action", "send_recharge_notification").
		Msg("[NOTIFICATION] recharge success notification pending")

	return nil
}

// handleOrderPaid 处理订单支付成功
func (processor *RedisTaskProcessor) handleOrderPaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if !paymentOrder.OrderID.Valid {
		return fmt.Errorf("order_id is required: %w", asynq.SkipRetry)
	}

	// ✅ 使用事务处理：更新订单状态 + 扣减库存 + 创建配送单 + 推入配送池 (原子操作)
	result, err := processor.store.ProcessOrderPaymentTx(ctx, db.ProcessOrderPaymentTxParams{
		OrderID: paymentOrder.OrderID.Int64,
	})
	if err != nil {
		// 如果是库存不足错误，记录详细日志
		if strings.Contains(err.Error(), "insufficient inventory") {
			log.Error().
				Err(err).
				Int64("order_id", paymentOrder.OrderID.Int64).
				Int64("payment_order_id", paymentOrder.ID).
				Int64("user_id", paymentOrder.UserID).
				Str("out_trade_no", paymentOrder.OutTradeNo).
				Msg("⚠️ order payment failed: insufficient inventory")

			// 🔔 需触发退款流程（待实现）
			log.Warn().
				Int64("payment_order_id", paymentOrder.ID).
				Int64("amount", paymentOrder.Amount).
				Str("action", "trigger_auto_refund").
				Msg("[REFUND] auto refund required due to insufficient inventory")

			return fmt.Errorf("insufficient inventory, refund required: %w", err)
		}
		return fmt.Errorf("process order payment: %w", err)
	}

	// 如果订单关联预订，支付成功后将预订状态推进到已签到，保持堂食与预订一致
	if result.Order.ReservationID.Valid {
		_, err := processor.store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
			ID:     result.Order.ReservationID.Int64,
			Status: "checked_in",
		})
		if err != nil {
			return fmt.Errorf("update reservation status after order payment: %w", err)
		}
	}

	// 根据订单类型记录日志
	if result.Delivery != nil {
		log.Info().
			Int64("order_id", paymentOrder.OrderID.Int64).
			Int64("delivery_id", result.Delivery.ID).
			Int64("amount", paymentOrder.Amount).
			Str("order_type", result.Order.OrderType).
			Msg("order paid, delivery created and added to pool")
	} else {
		log.Info().
			Int64("order_id", paymentOrder.OrderID.Int64).
			Int64("amount", paymentOrder.Amount).
			Str("order_type", result.Order.OrderType).
			Msg("order paid (non-delivery order)")
	}

	// ✅ 发送实时通知
	processor.sendOrderPaidNotifications(ctx, result)

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

	// 构建完整的新订单池消息数据（骑手App可直接显示）
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
		"distance":             poolItem.Distance,                 // 商家到顾客距离（米）
		"priority":             poolItem.Priority,                 // 优先级（高值单=2或3）
		"expected_pickup_at":   poolItem.ExpectedPickupAt,         // 预计出餐时间
		"expected_delivery_at": delivery.EstimatedDeliveryAt.Time, // 预计送达时间
		"is_high_value":        order.DeliveryFee >= 1000,         // 运费>=10元为高值单
		"created_at":           poolItem.CreatedAt,
	}
	msgData, _ := json.Marshal(newOrderData)

	// 通过 Redis Pub/Sub 推送给骑手
	notifiedCount := 0
	for _, riderID := range ridersToNotify {
		wsMessage := map[string]any{
			"type":      "delivery_pool_update",
			"data":      json.RawMessage(msgData),
			"timestamp": time.Now(),
		}
		wsMessageJSON, _ := json.Marshal(wsMessage)

		channel := fmt.Sprintf("notification:rider:%d", riderID)
		if err := processor.redisClient.Publish(ctx, channel, wsMessageJSON).Err(); err != nil {
			log.Error().Err(err).Int64("rider_id", riderID).Msg("publish new delivery to rider failed")
		} else {
			notifiedCount++
		}
	}

	log.Info().
		Int64("order_id", order.ID).
		Int64("delivery_id", delivery.ID).
		Float64("search_radius_m", usedDistance).
		Bool("region_broadcast", isRegionBroadcast).
		Int64("region_id", merchant.RegionID).
		Int("notified_count", notifiedCount).
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
		if errors.Is(err, pgx.ErrNoRows) {
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

		// 🔔 需发送退款通知（待实现）
		log.Info().
			Str("out_refund_no", payload.OutRefundNo).
			Str("refund_id", payload.RefundID).
			Str("action", "send_refund_notification").
			Msg("[NOTIFICATION] refund success - user notification pending")

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
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn().Int64("merchant_id", order.MerchantID).Msg("merchant payment config not found, skip profit sharing")
			return nil // 商户未配置微信支付，跳过分账
		}
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	// 获取运营商信息（根据配送地址所在区域）
	var operator db.Operator
	var hasOperator bool
	var operatorCommission int64 = 0
	var platformCommission int64 = 0
	var merchantAmount = order.TotalAmount

	// 只有外卖和预定才需要分账，堂食/打包商户全额收款
	needProfitSharing := order.OrderType == "takeout" || order.OrderType == "reservation"

	if needProfitSharing {
		// 获取配送地址的区域ID
		var regionID int64
		if order.AddressID.Valid {
			address, err := processor.store.GetUserAddress(ctx, order.AddressID.Int64)
			if err == nil {
				regionID = address.RegionID
			}
		}

		// 查找运营商
		if regionID > 0 {
			op, err := processor.store.GetOperatorByRegion(ctx, regionID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("get operator: %w", err)
			}

			if err == nil {
				operator = op
				hasOperator = true
				// 计算分账金额（单位：分）
				// 平台 2%, 运营商 3%, 商户 95%
				platformCommission = order.TotalAmount * 2 / 100
				operatorCommission = order.TotalAmount * 3 / 100
				merchantAmount = order.TotalAmount - platformCommission - operatorCommission
			}
		}
	}

	// 创建分账订单记录
	outOrderNo := fmt.Sprintf("PS%d%d", payload.PaymentOrderID, payload.OrderID)
	var operatorID pgtype.Int8
	if hasOperator {
		operatorID = pgtype.Int8{Int64: operator.ID, Valid: true}
	}

	profitSharingOrder, err := processor.store.CreateProfitSharingOrder(ctx, db.CreateProfitSharingOrderParams{
		PaymentOrderID:     payload.PaymentOrderID,
		MerchantID:         order.MerchantID,
		OperatorID:         operatorID,
		OrderSource:        order.OrderType,
		TotalAmount:        order.TotalAmount,
		PlatformCommission: platformCommission,
		OperatorCommission: operatorCommission,
		MerchantAmount:     merchantAmount,
		OutOrderNo:         outOrderNo,
	})
	if err != nil {
		return fmt.Errorf("create profit sharing order: %w", err)
	}

	log.Info().
		Int64("order_id", payload.OrderID).
		Str("merchant_name", merchant.Name).
		Int64("total_amount", order.TotalAmount).
		Int64("platform_commission", platformCommission).
		Int64("operator_commission", operatorCommission).
		Int64("merchant_amount", merchantAmount).
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
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	}

	return nil
}
