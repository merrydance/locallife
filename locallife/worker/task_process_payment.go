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
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// Redis 告警频道
const AlertChannel = "notification:platform:alerts"

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
	riderDeliveryPoolUpdateMessageType          = "delivery_pool_update"
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

func (processor *RedisTaskProcessor) ensurePersonalProfitSharingReceiver(ctx context.Context, openid, realName string) error {
	return processor.profitSharingReceiverSyncService().EnsurePersonalOpenIDReceiver(ctx, openid, realName)
}

func (processor *RedisTaskProcessor) ensurePersonalProfitSharingReceiverForPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder, subMchID, openid, realName string) error {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return fmt.Errorf("ordinary service provider client not configured for profit sharing receiver")
		}
		_, err := processor.ordinarySPClient.AddProfitSharingReceiver(ctx, ospcontracts.ProfitSharingReceiverAddRequest{
			SubMchID:     subMchID,
			AppID:        processor.ordinarySPClient.ServiceProviderAppID(),
			Type:         ospcontracts.ReceiverTypePersonalOpenID,
			Account:      strings.TrimSpace(openid),
			Name:         strings.TrimSpace(realName),
			RelationType: ospcontracts.ProfitSharingRelationStaff,
		})
		return err
	}
	return processor.ensurePersonalProfitSharingReceiver(ctx, openid, realName)
}

func (processor *RedisTaskProcessor) profitSharingReceiverSyncService() *logic.ProfitSharingReceiverSyncService {
	return logic.NewProfitSharingReceiverService(processor.store, processor.ecommerceClient)
}

type operatorProfitSharingReceiverTarget struct {
	ReceiverType string
	Account      string
	ReceiverName string
	RelationType string
	IsPersonal   bool
}

func resolveOperatorReceiverName(operator db.Operator) string {
	if name := strings.TrimSpace(operator.ContactName); name != "" {
		return name
	}
	return strings.TrimSpace(operator.Name)
}

func (processor *RedisTaskProcessor) resolveOperatorProfitSharingReceiver(ctx context.Context, operator db.Operator) (*operatorProfitSharingReceiverTarget, error) {
	user, err := processor.store.GetUser(ctx, operator.UserID)
	if err != nil {
		return nil, fmt.Errorf("get operator user: %w", err)
	}
	if strings.TrimSpace(user.WechatOpenid) == "" {
		return nil, fmt.Errorf("operator wechat openid not configured")
	}

	return &operatorProfitSharingReceiverTarget{
		ReceiverType: wechatcontracts.ReceiverTypePersonal,
		Account:      strings.TrimSpace(user.WechatOpenid),
		ReceiverName: resolveOperatorReceiverName(operator),
		RelationType: wechatcontracts.RelationOthers,
		IsPersonal:   true,
	}, nil
}

func (processor *RedisTaskProcessor) finishProfitSharingOrder(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	profitSharingOrder db.ProfitSharingOrder,
	subMchID string,
	description string,
) error {
	resp, err := processor.finishWechatProfitSharing(ctx, paymentOrder, subMchID, paymentOrder.TransactionID.String, profitSharingOrder.OutOrderNo, description)
	if err != nil {
		return fmt.Errorf("finish profit sharing: %w", err)
	}

	sharingOrderID := pgtype.Text{}
	if resp != nil && resp.OrderID != "" {
		sharingOrderID = pgtype.Text{String: resp.OrderID, Valid: true}
	}

	if _, err := processor.store.UpdateProfitSharingOrderToProcessing(ctx, db.UpdateProfitSharingOrderToProcessingParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: sharingOrderID,
	}); err != nil {
		return fmt.Errorf("update profit sharing order to processing: %w", err)
	}
	responseStatus := ""
	if resp != nil {
		responseStatus = resp.Status
	}
	recordProfitSharingCommandAccepted(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), profitSharingOrder, db.ExternalPaymentCommandTypeFinishProfitSharing, sharingOrderID.String, responseStatus)
	if logic.NormalizeProfitSharingTerminalStatus(responseStatus) == db.ExternalPaymentTerminalStatusSuccess {
		application, factErr := recordProfitSharingCommandResponseFact(ctx, processor.store, paymentOrder, profitSharingOrder, resp, db.ExternalPaymentCommandTypeFinishProfitSharing, nil)
		if factErr != nil {
			return fmt.Errorf("record finish profit sharing command response fact: %w", factErr)
		}
		enqueueProfitSharingPaymentFactApplication(ctx, processor.distributor, application)
	}

	log.Info().
		Int64("profit_sharing_order_id", profitSharingOrder.ID).
		Str("out_order_no", profitSharingOrder.OutOrderNo).
		Str("wechat_order_id", sharingOrderID.String).
		Msg("profit sharing finish order sent")

	return nil
}

// 任务类型常量
const (
	TaskProcessRefund                    = "payment:initiate_refund"
	TaskProcessRefundResult              = "payment:process_refund"
	TaskProcessProfitSharing             = "payment:process_profit_sharing"
	TaskProcessApplymentResult           = "payment:process_applyment_result"             // 进件结果处理
	TaskProcessProfitSharingReturnResult = "payment:process_profit_sharing_return_result" // 分账回退结果处理
	TaskProcessAnomalyRefund             = "payment:process_anomaly_refund"               // 已关闭订单异常退款
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

// ProfitSharingPayload 分账任务载荷
type ProfitSharingPayload struct {
	PaymentOrderID int64  `json:"payment_order_id"`
	OrderID        int64  `json:"order_id,omitempty"`
	ReservationID  int64  `json:"reservation_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// ApplymentResultPayload 进件结果处理任务载荷
type ApplymentResultPayload struct {
	ApplymentID     int64  `json:"applyment_id"`     // 进件记录ID
	OutRequestNo    string `json:"out_request_no"`   // 业务申请编号
	ApplymentState  string `json:"applyment_state"`  // 进件状态
	ApplymentStatus string `json:"applyment_status"` // 本地映射状态
	SignState       string `json:"sign_state"`       // 签约状态
	SubMchID        string `json:"sub_mch_id"`       // 二级商户号（开户成功时返回）
	SubjectType     string `json:"subject_type"`     // 主体类型：merchant/operator
	SubjectID       int64  `json:"subject_id"`       // 主体ID
}

// ProfitSharingResultPayload 分账结果 outbox 载荷
type ProfitSharingResultPayload struct {
	ProfitSharingOrderID int64  `json:"profit_sharing_order_id"` // 分账订单ID
	OutOrderNo           string `json:"out_order_no"`            // 商户分账单号
	Result               string `json:"result"`                  // 分账结果：SUCCESS/CLOSED/FAILED
	FailReason           string `json:"fail_reason"`             // 失败原因
	MerchantID           int64  `json:"merchant_id"`             // 商户ID
}

// ProfitSharingReturnResultPayload 分账回退结果处理任务载荷
type ProfitSharingReturnResultPayload struct {
	ProfitSharingReturnID int64  `json:"profit_sharing_return_id"`
	OutReturnNo           string `json:"out_return_no"`
	OutOrderNo            string `json:"out_order_no"`
	SubMchID              string `json:"sub_mchid"`
	RefundOrderID         int64  `json:"refund_order_id"`
	RetryCount            int    `json:"retry_count"`
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

// DistributeTaskProcessProfitSharing 分发分账处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessProfitSharing(
	ctx context.Context,
	payload *ProfitSharingPayload,
	opts ...asynq.Option,
) error {
	normalizedPayload := normalizeProfitSharingPayload(payload)
	jsonPayload, err := json.Marshal(normalizedPayload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessProfitSharing, jsonPayload, withProfitSharingEnqueueDedup(opts...)...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	event := log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_order_id", normalizedPayload.PaymentOrderID).
		Str("idempotency_key", normalizedPayload.IdempotencyKey)

	if normalizedPayload.OrderID > 0 {
		event.Int64("order_id", normalizedPayload.OrderID)
	}
	if normalizedPayload.ReservationID > 0 {
		event.Int64("reservation_id", normalizedPayload.ReservationID)
	}

	event.Msg("enqueued task")

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
		Str("sign_state", payload.SignState).
		Msg("enqueued applyment result task")

	return nil
}

// DistributeTaskProcessProfitSharingReturnResult 分发分账回退结果处理任务
func (distributor *RedisTaskDistributor) DistributeTaskProcessProfitSharingReturnResult(
	ctx context.Context,
	payload *ProfitSharingReturnResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessProfitSharingReturnResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Str("out_return_no", payload.OutReturnNo).
		Msg("enqueued profit sharing return result task")

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
	if order.Status != db.OrderStatusPaid {
		log.Error().
			Int64("order_id", order.ID).
			Int64("merchant_id", order.MerchantID).
			Str("order_no", order.OrderNo).
			Str("status", order.Status).
			Str("payment_method", order.PaymentMethod.String).
			Msg("merchant new order notification attempted for unpaid order")
		return fmt.Errorf("merchant new order notification requires paid order: order_id=%d status=%s", order.ID, order.Status)
	}

	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant for notification: %w", err)
	}
	merchantPayload := logic.BuildMerchantNewOrderNotification(order, merchant.Name)

	items, err := processor.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return fmt.Errorf("load order items for merchant new order snapshot: %w", err)
	}
	itemViews, err := logic.BuildOrderItemViews(items)
	if err != nil {
		return fmt.Errorf("build order items for merchant new order snapshot: %w", err)
	}
	feeBreakdown, err := processor.loadMerchantOrderFeeBreakdown(ctx, order)
	if err != nil {
		log.Error().Err(err).
			Int64("order_id", order.ID).
			Int64("merchant_id", order.MerchantID).
			Str("order_no", order.OrderNo).
			Str("status", order.Status).
			Msg("merchant new order fee breakdown unavailable")
		return fmt.Errorf("build merchant new order fee breakdown: %w", err)
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

func (processor *RedisTaskProcessor) loadMerchantOrderFeeBreakdown(ctx context.Context, order db.Order) (logic.MerchantOrderFeeBreakdown, error) {
	paymentOrder, err := processor.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	})
	if err != nil {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("get latest payment order by order: %w", err)
	}
	profitSharingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		return logic.MerchantOrderFeeBreakdown{}, fmt.Errorf("get profit sharing order by payment order: %w", err)
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
	if paymentOrderUsesEcommerceChannel(paymentOrder) && paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder {
		return fmt.Errorf("order refund results must be applied via payment fact application: %w", asynq.SkipRetry)
	}
	if isReservationRefundPayment(paymentOrder) {
		return fmt.Errorf("reservation refund results must be applied via payment fact application: %w", asynq.SkipRetry)
	}
	merchantID := int64(0)
	resolvedMerchantID, resolveErr := processor.resolveMerchantIDByPaymentOrder(ctx, paymentOrder)
	if resolveErr != nil {
		log.Warn().Err(resolveErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to resolve merchant for refund alert context")
	} else {
		merchantID = resolvedMerchantID
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
		_, err = processor.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		if err != nil {
			return fmt.Errorf("update refund order to failed: %w", err)
		}
		log.Warn().Str("out_refund_no", payload.OutRefundNo).Msg("refund abnormal")

		alertExtra := refundOrderAlertExtra(paymentOrder, refundOrder, merchantID, map[string]interface{}{
			"refund_id": payload.RefundID,
		})
		alertExtra = mergeAlertExtra(alertExtra, abnormalRefundActionExtra(paymentOrder, refundOrder))

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

// ProcessTaskProfitSharing 处理分账任务
func (processor *RedisTaskProcessor) ProcessTaskProfitSharing(ctx context.Context, task *asynq.Task) error {
	var payload ProfitSharingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	event := log.Info().
		Int64("payment_order_id", payload.PaymentOrderID)

	if payload.OrderID > 0 {
		event.Int64("order_id", payload.OrderID)
	}
	if payload.ReservationID > 0 {
		event.Int64("reservation_id", payload.ReservationID)
	}

	event.Msg("processing profit sharing")

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

	// 初始化通用参数
	var merchantID int64
	var totalAmount int64
	var deliveryFee int64
	var orderSource string
	var outOrderNoSuffix string

	if payload.OrderID > 0 {
		// 获取订单信息
		order, err := processor.store.GetOrder(ctx, payload.OrderID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("order not found: %w", asynq.SkipRetry)
			}
			return fmt.Errorf("get order: %w", err)
		}
		merchantID = order.MerchantID
		// 分账基数使用 paymentOrder.Amount（微信实际冻结金额）而非 order.TotalAmount。
		// 当用户使用会员余额部分支付时，order.TotalAmount = WeChat支付额 + 余额支付额，
		// 而 paymentOrder.Amount = order.TotalAmount - order.BalancePaid = 微信冻结额。
		// 若以 order.TotalAmount 为基数，接收方总额可能超过微信冻结额，导致分账 API 失败。
		// 注意：满减、优惠券、配送费折扣、押金抵扣均已在 order.TotalAmount 中扣除，无需额外处理。
		totalAmount = paymentOrder.Amount
		deliveryFee = order.DeliveryFee
		orderSource = order.OrderType
		if order.ReservationID.Valid && order.OrderType == "dine_in" {
			// 预订到店后的替换/补差价订单会落在 dine_in 订单类型上，
			// 但资金语义仍属于预订链路，后续分账必须继续按 reservation 配置执行。
			orderSource = db.OrderTypeReservation
		}
		outOrderNoSuffix = fmt.Sprintf("%d", payload.OrderID)
	} else if payload.ReservationID > 0 {
		// 获取预订信息
		res, err := processor.store.GetTableReservation(ctx, payload.ReservationID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("reservation not found: %w", asynq.SkipRetry)
			}
			return fmt.Errorf("get reservation: %w", err)
		}
		merchantID = res.MerchantID
		// 预订交易使用支付订单金额
		totalAmount = paymentOrder.Amount
		deliveryFee = 0
		orderSource = "reservation"
		outOrderNoSuffix = fmt.Sprintf("R%d", payload.ReservationID)
	} else {
		return fmt.Errorf("neither order_id nor reservation_id provided: %w", asynq.SkipRetry)
	}

	// 获取商户信息
	merchant, err := processor.store.GetMerchant(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("get merchant: %w", err)
	}

	// 获取商户支付配置（从新表 merchant_payment_configs）
	paymentConfig, err := processor.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("merchant_id", merchantID).Msg("merchant payment config not found, skip profit sharing")
			return nil // 商户未配置微信支付，跳过分账
		}
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	// 获取运营商信息（根据商户所属区域）
	var operator db.Operator
	var hasOperator bool
	regionID := merchant.RegionID
	var operatorCommission int64
	var platformCommission int64
	var operatorCommissionRedirectedToPlatform bool
	merchantAmount := totalAmount

	config, err := processor.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: orderSource,
		MerchantID:  pgtype.Int8{Int64: merchantID, Valid: true},
		RegionID:    pgtype.Int8{Int64: regionID, Valid: regionID > 0},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			// 如果没有配置分账规则，假设无分账，仅记录
			config = db.ProfitSharingConfig{
				PlatformRate: 0,
				OperatorRate: 0,
				RiderEnabled: false,
			}
		} else {
			return fmt.Errorf("get profit sharing config: %w", err)
		}
	}

	platformRate := config.PlatformRate
	operatorRate := config.OperatorRate
	riderEnabled := config.RiderEnabled
	var rider db.Rider
	var hasRider bool
	var riderAmount int64
	var riderUserOpenID string

	// 预订业务强制禁用骑手分账
	if orderSource == "reservation" {
		riderEnabled = false
	}

	// 堂食/打包自提强制禁用平台与运营商分账（防御性校验，避免错误 DB 配置产生误收）
	if orderSource == "dine_in" || orderSource == "takeaway" {
		platformRate = 0
		operatorRate = 0
		riderEnabled = false
	}

	if payload.OrderID > 0 && riderEnabled && deliveryFee > 0 {
		delivery, err := processor.store.GetDeliveryByOrderID(ctx, payload.OrderID)
		if err != nil {
			if !errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("get delivery by order id: %w", err)
			}
		} else if delivery.RiderID.Valid {
			rider, err = processor.store.GetRider(ctx, delivery.RiderID.Int64)
			if err != nil {
				return fmt.Errorf("get rider: %w", err)
			}
			user, err := processor.store.GetUser(ctx, rider.UserID)
			if err != nil {
				return fmt.Errorf("get rider user: %w", err)
			}
			if user.WechatOpenid == "" {
				log.Warn().
					Int64("order_id", payload.OrderID).
					Int64("rider_id", rider.ID).
					Msg("rider wechat openid is empty, skip rider direct profit sharing")
			} else {
				hasRider = true
				riderAmount = deliveryFee
				// 当用户使用会员余额大额支付时，微信冻结额可能小于配送费。
				// 将骑手分账金额上限对齐至实际微信冻结额，避免接收方总额超出冻结额。
				if riderAmount > totalAmount {
					riderAmount = totalAmount
				}
				riderUserOpenID = user.WechatOpenid
			}
		}
	}

	// 查找运营商
	if regionID > 0 {
		op, err := processor.store.GetActiveOperatorByRegion(ctx, regionID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("get operator: %w", err)
		}

		if err == nil {
			operator = op
			hasOperator = true
		}
	}

	distributableAmount := totalAmount - riderAmount
	if distributableAmount < 0 {
		distributableAmount = 0
	}

	// 计算分账金额（单位：分）
	platformCommission = distributableAmount * int64(platformRate) / 100
	operatorCommission = distributableAmount * int64(operatorRate) / 100
	if !hasOperator && operatorCommission > 0 {
		platformCommission += operatorCommission
		operatorCommission = 0
		operatorCommissionRedirectedToPlatform = true
	}
	merchantAmount = distributableAmount - platformCommission - operatorCommission
	if merchantAmount < 0 {
		log.Error().
			Int64("payment_order_id", payload.PaymentOrderID).
			Int64("total_amount", totalAmount).
			Int64("distributable_amount", distributableAmount).
			Int64("platform_commission", platformCommission).
			Int64("operator_commission", operatorCommission).
			Int64("rider_amount", riderAmount).
			Msg("merchant amount computed negative: platform+operator rates exceed 100%, check profit sharing config")
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeSystemError,
			Level:       AlertLevelCritical,
			Title:       "分账配置错误",
			Message:     fmt.Sprintf("支付单 %d 分账计算商户金额为负（平台+运营商比例之和超过100%%），请检查分账配置", payload.PaymentOrderID),
			RelatedID:   payload.PaymentOrderID,
			RelatedType: "payment_order",
		})
		merchantAmount = 0
	}
	// needProfitSharing 在加载/创建分账订单（可能覆盖 commission 值）之后再求值，
	// 以确保重试路径中复用的是存储值，而不是本次重新计算的值。
	var needProfitSharing bool

	// 若已有分账记录，复用并尝试重试
	existingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, payload.PaymentOrderID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return fmt.Errorf("get profit sharing order by payment order: %w", err)
	}

	var profitSharingOrder db.ProfitSharingOrder
	var outOrderNo string
	if err == nil {
		if existingOrder.Status == "finished" {
			log.Info().
				Int64("profit_sharing_order_id", existingOrder.ID).
				Str("status", existingOrder.Status).
				Msg("profit sharing already processed, skip")
			return nil
		}
		if existingOrder.Status == "processing" {
			return processor.reconcileProcessingProfitSharing(ctx, paymentOrder, paymentConfig.SubMchID, existingOrder)
		}

		profitSharingOrder = existingOrder
		outOrderNo = existingOrder.OutOrderNo
		platformCommission = existingOrder.PlatformCommission
		operatorCommission = existingOrder.OperatorCommission
		merchantAmount = existingOrder.MerchantAmount
		riderAmount = existingOrder.RiderAmount

		if existingOrder.OperatorID.Valid {
			op, err := processor.store.GetOperator(ctx, existingOrder.OperatorID.Int64)
			if err == nil {
				operator = op
				hasOperator = true
			}
		}

		if existingOrder.RiderID.Valid && riderAmount > 0 {
			r, getRiderErr := processor.store.GetRider(ctx, existingOrder.RiderID.Int64)
			if getRiderErr != nil {
				return fmt.Errorf("get rider for existing profit sharing order: %w", getRiderErr)
			}
			user, getUserErr := processor.store.GetUser(ctx, r.UserID)
			if getUserErr != nil {
				return fmt.Errorf("get rider user for existing profit sharing order: %w", getUserErr)
			}
			if user.WechatOpenid != "" {
				rider = r
				hasRider = true
				riderUserOpenID = user.WechatOpenid
			}
		}
	} else {
		// 创建分账订单记录
		outOrderNo = fmt.Sprintf("PS%d%s", payload.PaymentOrderID, outOrderNoSuffix)
		var operatorID pgtype.Int8
		var riderID pgtype.Int8
		if hasOperator {
			operatorID = pgtype.Int8{Int64: operator.ID, Valid: true}
		}
		if hasRider {
			riderID = pgtype.Int8{Int64: rider.ID, Valid: true}
		}

		profitSharingOrder, err = processor.store.CreateProfitSharingOrder(ctx, db.CreateProfitSharingOrderParams{
			PaymentOrderID:      payload.PaymentOrderID,
			MerchantID:          merchantID,
			OperatorID:          operatorID,
			OrderSource:         orderSource,
			TotalAmount:         totalAmount,
			DeliveryFee:         deliveryFee,
			RiderID:             riderID,
			RiderAmount:         riderAmount,
			DistributableAmount: distributableAmount,
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

	// 在加载/创建分账订单后求值，确保使用最终存储的 commission 值（而非本次重新计算值）
	needProfitSharing = platformCommission > 0 || operatorCommission > 0 || riderAmount > 0

	log.Info().
		Int64("payment_order_id", payload.PaymentOrderID).
		Str("merchant_name", merchant.Name).
		Int64("total_amount", totalAmount).
		Int64("platform_commission", platformCommission).
		Int64("operator_commission", operatorCommission).
		Int64("rider_amount", riderAmount).
		Int64("merchant_amount", merchantAmount).
		Int32("platform_rate", platformRate).
		Int32("operator_rate", operatorRate).
		Bool("operator_commission_redirected_to_platform", operatorCommissionRedirectedToPlatform).
		Bool("need_profit_sharing", needProfitSharing).
		Msg("profit sharing order created")

	// 如果不需要分账（堂食/打包），直接完结分账
	if !needProfitSharing {
		err := processor.finishProfitSharingOrder(ctx, paymentOrder, profitSharingOrder, paymentConfig.SubMchID, "无需分账，解冻剩余资金")
		if err != nil {
			return err
		}
		log.Info().Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("no profit sharing needed, finish order requested")
		return nil
	}

	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return fmt.Errorf("ordinary service provider client not configured for profit sharing: %w", asynq.SkipRetry)
		}
	} else if processor.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured for profit sharing: %w", asynq.SkipRetry)
	}

	var operatorTarget *operatorProfitSharingReceiverTarget
	if hasOperator && operatorCommission > 0 {
		operatorTarget, err = processor.resolveOperatorProfitSharingReceiver(ctx, operator)
		if err != nil {
			return fmt.Errorf("resolve operator receiver: %w", err)
		}
	}

	// 构建分账接收方列表
	var receivers []wechatcontracts.ProfitSharingReceiver

	// 平台佣金（进入服务商账户）
	if platformCommission > 0 {
		serviceProviderMchID := ""
		serviceProviderMchName := ""
		if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
			serviceProviderMchID = processor.ordinarySPClient.ServiceProviderMchID()
			serviceProviderMchName = processor.ordinarySPClient.ServiceProviderMchName()
		} else {
			serviceProviderMchID = processor.ecommerceClient.GetSpMchID()
			serviceProviderMchName = processor.ecommerceClient.GetSpMchName()
		}
		receivers = append(receivers, wechatcontracts.ProfitSharingReceiver{
			Type:            "MERCHANT_ID",
			ReceiverAccount: serviceProviderMchID,
			ReceiverName:    strings.TrimSpace(serviceProviderMchName),
			Amount:          platformCommission,
			Description:     "平台服务费",
		})
	}

	// 运营商佣金
	if operatorTarget != nil {
		receivers = append(receivers, wechatcontracts.ProfitSharingReceiver{
			Type:            operatorTarget.ReceiverType,
			ReceiverAccount: operatorTarget.Account,
			Amount:          operatorCommission,
			Description:     "运营商服务费",
		})
	}

	if hasRider && riderAmount > 0 && riderUserOpenID != "" {
		receivers = append(receivers, wechatcontracts.ProfitSharingReceiver{
			Type:            wechatcontracts.ReceiverTypePersonal,
			ReceiverAccount: riderUserOpenID,
			Amount:          riderAmount,
			Description:     "骑手配送费",
		})
	}

	// 如果没有接收方，直接完结
	if len(receivers) == 0 {
		err = processor.finishProfitSharingOrder(ctx, paymentOrder, profitSharingOrder, paymentConfig.SubMchID, "无可用分账接收方，解冻剩余资金")
		if err != nil {
			return err
		}
		log.Info().Int64("profit_sharing_order_id", profitSharingOrder.ID).Msg("no receivers, finish order requested")
		return nil
	}

	if operatorTarget != nil {
		ensureErr := processor.ensurePersonalProfitSharingReceiverForPaymentOrder(ctx, paymentOrder, paymentConfig.SubMchID, operatorTarget.Account, operatorTarget.ReceiverName)
		if ensureErr != nil {
			log.Error().Err(ensureErr).
				Int64("profit_sharing_order_id", profitSharingOrder.ID).
				Str("operator_receiver_type", operatorTarget.ReceiverType).
				Str("operator_receiver_account", operatorTarget.Account).
				Msg("ensure operator profit sharing receiver failed")
			return fmt.Errorf("ensure operator receiver: %w", ensureErr)
		}
	}

	if hasRider && riderAmount > 0 && riderUserOpenID != "" {
		if err := processor.ensurePersonalProfitSharingReceiverForPaymentOrder(ctx, paymentOrder, paymentConfig.SubMchID, riderUserOpenID, rider.RealName); err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", profitSharingOrder.ID).
				Int64("rider_id", rider.ID).
				Msg("ensure rider personal profit sharing receiver failed")
			return fmt.Errorf("ensure rider receiver: %w", err)
		}
	}

	// 调用微信分账 API
	reqProfitSharing := &wechatcontracts.ProfitSharingRequest{
		SubMchID:      paymentConfig.SubMchID, // 商户二级商户号
		TransactionID: paymentOrder.TransactionID.String,
		OutOrderNo:    outOrderNo,
		Receivers:     receivers,
		Finish:        true, // 分账完成后剩余资金解冻给商户
	}
	resp, err := processor.createWechatProfitSharing(ctx, paymentOrder, reqProfitSharing)
	if err != nil {
		log.Warn().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", outOrderNo).
			Msg("create profit sharing failed, retry once after re-ensuring receivers")

		if operatorTarget != nil {
			if retryEnsureErr := processor.ensurePersonalProfitSharingReceiverForPaymentOrder(ctx, paymentOrder, paymentConfig.SubMchID, operatorTarget.Account, operatorTarget.ReceiverName); retryEnsureErr != nil {
				log.Warn().Err(retryEnsureErr).
					Int64("profit_sharing_order_id", profitSharingOrder.ID).
					Int64("payment_order_id", paymentOrder.ID).
					Str("receiver_role", "operator").
					Msg("re-ensure operator profit sharing receiver failed before retry")
			}
		}
		if hasRider && riderAmount > 0 && riderUserOpenID != "" {
			if retryEnsureErr := processor.ensurePersonalProfitSharingReceiverForPaymentOrder(ctx, paymentOrder, paymentConfig.SubMchID, riderUserOpenID, rider.RealName); retryEnsureErr != nil {
				log.Warn().Err(retryEnsureErr).
					Int64("profit_sharing_order_id", profitSharingOrder.ID).
					Int64("payment_order_id", paymentOrder.ID).
					Str("receiver_role", "rider").
					Msg("re-ensure rider profit sharing receiver failed before retry")
			}
		}

		resp, err = processor.createWechatProfitSharing(ctx, paymentOrder, reqProfitSharing)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", profitSharingOrder.ID).
				Str("out_order_no", outOrderNo).
				Msg("call wechat profit sharing API failed after retry")
			return fmt.Errorf("create profit sharing: %w", err)
		}
	}

	// 更新分账订单状态为处理中
	_, err = processor.store.UpdateProfitSharingOrderToProcessing(ctx, db.UpdateProfitSharingOrderToProcessingParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: pgtype.Text{String: resp.OrderID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update profit sharing order to processing: %w", err)
	}
	recordProfitSharingCommandAccepted(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), profitSharingOrder, db.ExternalPaymentCommandTypeCreateProfitSharing, resp.OrderID, resp.Status)
	if logic.NormalizeProfitSharingTerminalStatus(resp.Status) == db.ExternalPaymentTerminalStatusSuccess {
		application, factErr := recordProfitSharingCommandResponseFact(ctx, processor.store, paymentOrder, profitSharingOrder, resp, db.ExternalPaymentCommandTypeCreateProfitSharing, profitSharingCommandResponseAmount(profitSharingOrder))
		if factErr != nil {
			return fmt.Errorf("record profit sharing command response fact: %w", factErr)
		}
		enqueueProfitSharingPaymentFactApplication(ctx, processor.distributor, application)
	}

	log.Info().
		Int64("profit_sharing_order_id", profitSharingOrder.ID).
		Str("wechat_order_id", resp.OrderID).
		Str("status", resp.Status).
		Msg("profit sharing request sent")

	return nil
}

func (processor *RedisTaskProcessor) reconcileProcessingProfitSharing(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	subMchID string,
	profitSharingOrder db.ProfitSharingOrder,
) error {
	queryResp, err := processor.queryWechatProfitSharing(ctx, paymentOrder, subMchID, paymentOrder.TransactionID.String, profitSharingOrder.OutOrderNo)
	if err != nil {
		return fmt.Errorf("query profit sharing: %w", err)
	}

	finalResult, failReason := logic.ResolveProfitSharingQueryFinalResult(queryResp)
	paymentFactApplication, err := recordProfitSharingQueryFact(ctx, processor.store, paymentOrder, profitSharingOrder, queryResp, finalResult, failReason)
	if err != nil {
		return fmt.Errorf("record profit sharing query fact: %w", err)
	}
	switch finalResult {
	case "PROCESSING":
		log.Info().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", profitSharingOrder.OutOrderNo).
			Msg("profit sharing still processing after query")
		return nil
	case "SUCCESS":
	case "FAILED":
	}
	enqueueProfitSharingPaymentFactApplication(ctx, processor.distributor, paymentFactApplication)

	return nil
}

// ProcessTaskInitiateRefund 处理发起退款任务
func (processor *RedisTaskProcessor) ProcessTaskInitiateRefund(ctx context.Context, task *asynq.Task) error {
	var payload PayloadProcessRefund
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// 预定退款走独立简化流程（直连支付，无分账）
	if payload.ReservationID > 0 {
		return processor.processReservationRefund(ctx, payload)
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

	switch paymentOrder.BusinessType {
	case "rider_deposit":
		return processor.processRiderDepositMismatchRefund(ctx, paymentOrder, payload)
	}

	orderID := payload.OrderID
	if orderID == 0 && paymentOrder.OrderID.Valid {
		orderID = paymentOrder.OrderID.Int64
	}
	if orderID == 0 {
		return fmt.Errorf("payment order %d missing order context for refund", paymentOrder.ID)
	}

	// 获取订单以获取商户信息
	order, err := processor.store.GetOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	// 生成退款单号（下划线分隔符确保不同 ID 组合不产生相同字符串）
	// 例: RF1_23 ≠ RF12_3，而 RF123 则无法区分
	outRefundNo := fmt.Sprintf("RF%d_%d", payload.PaymentOrderID, orderID)

	// 幂等检查：该退款单号是否已存在
	var refundOrder db.RefundOrder
	existingRefund, findErr := processor.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if findErr == nil {
		// 已存在：如果已成功或处理中，直接短路返回，避免重复请求微信退款 API
		refundOrder = existingRefund
		if refundOrder.Status == "success" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("refund already succeeded, skip")
			return nil
		}
		if refundOrder.Status == "processing" {
			log.Info().Str("out_refund_no", outRefundNo).Msg("refund already processing, skip")
			return nil
		}
		if refundOrder.Status == "failed" || refundOrder.Status == "closed" {
			log.Info().
				Str("out_refund_no", outRefundNo).
				Int64("refund_order_id", refundOrder.ID).
				Str("status", refundOrder.Status).
				Msg("refund already terminal, skip")
			return nil
		}
		log.Info().
			Str("out_refund_no", outRefundNo).
			Int64("refund_order_id", refundOrder.ID).
			Str("status", refundOrder.Status).
			Msg("refund order already exists, retrying")
	} else if !errors.Is(findErr, db.ErrRecordNotFound) {
		return fmt.Errorf("check existing refund order: %w", findErr)
	} else {
		// 不存在：通过事务原子性地校验累计退款额并创建退款单，防止并发超退（与人工退款链路对齐）
		txResult, createErr := processor.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: payload.PaymentOrderID,
			RefundType:     "user_cancel",
			RefundAmount:   payload.RefundAmount,
			RefundReason:   payload.Reason,
			OutRefundNo:    outRefundNo,
		})
		if createErr != nil {
			if statusCode, ok := db.IsRefundRequestError(createErr); ok {
				// 业务校验失败（超退、已退等）：不重试
				log.Warn().Err(createErr).Int("status", statusCode).Msg("refund business validation failed, skip")
				return nil
			}
			return fmt.Errorf("create refund order tx: %w", createErr)
		}
		refundOrder = txResult.RefundOrder
	}
	if requiresEcommerceRefund(paymentOrder) && !paymentOrderUsesMainBusinessRefundChannel(paymentOrder) {
		refundErr := mainBusinessRefundChannelDriftError(paymentOrder, "initiate refund")
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(refundErr, fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		return fmt.Errorf("%w: %w", refundErr, asynq.SkipRetry)
	}

	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		return processor.processBaofuAggregateRefund(ctx, paymentOrder, refundOrder, payload, outRefundNo)
	}

	var paymentConfig db.MerchantPaymentConfig

	if paymentOrderUsesMainBusinessRefundChannel(paymentOrder) {
		if paymentOrderUsesEcommerceChannel(paymentOrder) && processor.ecommerceClient == nil {
			return fmt.Errorf("ecommerce client not configured for refund")
		}
		if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) && processor.ordinarySPClient == nil {
			return fmt.Errorf("ordinary service provider client not configured for refund")
		}

		paymentConfig, err = processor.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("merchant payment config not found")
			}
			return fmt.Errorf("get merchant payment config: %w", err)
		}
	}

	if paymentOrderUsesMainBusinessRefundChannel(paymentOrder) {
		profitSharingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
		if err != nil {
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(fmt.Errorf("profit sharing order not found"), fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			return fmt.Errorf("profit sharing order not found")
		}
		if profitSharingOrder.RiderAmount > 0 {
			blockingErr := errors.New("订单包含个人分账，当前不支持自动退款，请联系平台处理")
			log.Error().
				Int64("refund_order_id", refundOrder.ID).
				Int64("profit_sharing_order_id", profitSharingOrder.ID).
				Int64("rider_amount", profitSharingOrder.RiderAmount).
				Msg("refund blocked because rider personal profit sharing return is unsupported")
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(blockingErr, fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeRefundFailed,
				Level:       AlertLevelCritical,
				Title:       "退款被阻断：存在个人分账",
				Message:     fmt.Sprintf("退款单 %d 包含骑手个人分账金额 %.2f 元，微信暂不支持个人接收方分账回退，已自动阻断并标记失败，请平台人工处理。", refundOrder.ID, float64(profitSharingOrder.RiderAmount)/100),
				RelatedID:   refundOrder.ID,
				RelatedType: "refund_order",
				Extra: mergeAlertExtra(
					refundOrderAlertExtra(paymentOrder, refundOrder, profitSharingOrder.MerchantID, nil),
					profitSharingOrderAlertExtra(profitSharingOrder, nil),
				),
			})
			return fmt.Errorf("%w: %w", blockingErr, asynq.SkipRetry)
		}
		if !profitSharingOrder.SharingOrderID.Valid || profitSharingOrder.SharingOrderID.String == "" {
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return fmt.Errorf("mark refund order as failed: %w", dbErr)
			}
			return fmt.Errorf("profit sharing order id missing")
		}

		var operatorTarget *operatorProfitSharingReceiverTarget
		if profitSharingOrder.OperatorCommission > 0 {
			if !profitSharingOrder.OperatorID.Valid {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return fmt.Errorf("mark refund order as failed: %w", dbErr)
				}
				return fmt.Errorf("operator not found for profit sharing")
			}
			op, err := processor.store.GetOperator(ctx, profitSharingOrder.OperatorID.Int64)
			if err != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get operator: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get operator: %w", err)
			}
			target, targetErr := processor.resolveOperatorProfitSharingReceiver(ctx, op)
			if targetErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("resolve operator receiver: %w", targetErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("resolve operator receiver: %w", targetErr)
			}
			operatorTarget = target
			if operatorTarget.IsPersonal {
				blockingErr := fmt.Errorf("订单包含个人分账，当前不支持自动退款")
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(blockingErr, fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				processor.publishAlert(ctx, AlertData{
					AlertType:   AlertTypeRefundFailed,
					Level:       AlertLevelCritical,
					Title:       "退款被阻断：存在个人运营商分账",
					Message:     fmt.Sprintf("退款单 %d 包含个人运营商分账金额 %.2f 元，微信暂不支持个人接收方分账回退，已自动阻断并标记失败，请平台人工处理。", refundOrder.ID, float64(profitSharingOrder.OperatorCommission)/100),
					RelatedID:   refundOrder.ID,
					RelatedType: "refund_order",
					Extra: mergeAlertExtra(
						refundOrderAlertExtra(paymentOrder, refundOrder, profitSharingOrder.MerchantID, nil),
						profitSharingOrderAlertExtra(profitSharingOrder, nil),
					),
				})
				return fmt.Errorf("%w: %w", blockingErr, asynq.SkipRetry)
			}
		}

		riderOpenID := ""
		if profitSharingOrder.RiderAmount > 0 && profitSharingOrder.RiderID.Valid {
			rider, getRiderErr := processor.store.GetRider(ctx, profitSharingOrder.RiderID.Int64)
			if getRiderErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get rider: %w", getRiderErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get rider: %w", getRiderErr)
			}
			user, getUserErr := processor.store.GetUser(ctx, rider.UserID)
			if getUserErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get rider user: %w", getUserErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get rider user: %w", getUserErr)
			}
			if user.WechatOpenid == "" {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return fmt.Errorf("mark refund order as failed: %w", dbErr)
				}
				return fmt.Errorf("rider wechat openid not configured")
			}
			riderOpenID = user.WechatOpenid
		}

		hasProcessing := false
		pendingReturnFactApplications := make([]*db.ExternalPaymentFactApplication, 0, 2)
		profitSharingReturnTerminalFactQueuedErr := errors.New("profit sharing return terminal fact queued")
		enqueuePendingProfitSharingReturnFactApplications := func() {
			for _, application := range pendingReturnFactApplications {
				enqueueProfitSharingReturnPaymentFactApplication(ctx, processor.distributor, application)
			}
		}
		processReturn := func(outReturnNo, returnAccount, description string, amount int64) error {
			// 幂等检查：如果该 outReturnNo 已有记录，且状态为 success/processing，直接跳过
			// 这让 ProcessTaskInitiateRefund 在被重试时能从失败点继续，而非重新全量执行
			existingReturn, lookupErr := processor.store.GetProfitSharingReturnByOutReturnNo(ctx, outReturnNo)
			if lookupErr == nil {
				switch existingReturn.Status {
				case "success":
					return nil // 已完成，跳过
				case "processing":
					hasProcessing = true
					return nil // 已在进行中，等待 recovery 跟踪
					// pending/failed 状态：继续向下重试
				}
			}

			returnRecord, err := processor.store.CreateProfitSharingReturn(ctx, db.CreateProfitSharingReturnParams{
				RefundOrderID:        refundOrder.ID,
				ProfitSharingOrderID: profitSharingOrder.ID,
				PaymentOrderID:       paymentOrder.ID,
				SubMchid:             paymentConfig.SubMchID,
				OutOrderNo:           profitSharingOrder.OutOrderNo,
				OutReturnNo:          outReturnNo,
				ReturnMchid:          returnAccount,
				Amount:               amount,
				Status:               "pending",
			})
			if err != nil {
				return err
			}

			returnResp, err := processor.createWechatProfitSharingReturn(ctx, paymentOrder, &wechatcontracts.ProfitSharingReturnRequest{
				SubMchID:      paymentConfig.SubMchID,
				OrderID:       profitSharingOrder.SharingOrderID.String,
				TransactionID: paymentOrder.TransactionID.String,
				OutOrderNo:    profitSharingOrder.OutOrderNo,
				OutReturnNo:   outReturnNo,
				ReturnMchID:   returnAccount,
				Amount:        amount,
				Description:   description,
			})
			if err != nil {
				if isProfitSharingReturnProcessingCommandError(err) {
					log.Warn().
						Err(err).
						Int64("profit_sharing_return_id", returnRecord.ID).
						Str("out_return_no", returnRecord.OutReturnNo).
						Msg("profit sharing return request reported ambiguous state, fallback to polling")

					if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
						ID:       returnRecord.ID,
						ReturnID: pgtype.Text{},
					}); dbErr != nil {
						log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
					} else {
						recordWorkerProfitSharingReturnCommandUnknown(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, err)
					}
					if processor.distributor != nil {
						if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
							ctx,
							&ProfitSharingReturnResultPayload{
								ProfitSharingReturnID: returnRecord.ID,
								OutReturnNo:           returnRecord.OutReturnNo,
								OutOrderNo:            returnRecord.OutOrderNo,
								SubMchID:              returnRecord.SubMchid,
								RefundOrderID:         returnRecord.RefundOrderID,
								RetryCount:            0,
							},
							asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
						); enqErr != nil {
							log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
						}
					}
					hasProcessing = true
					return nil
				}

				recordWorkerProfitSharingReturnCommandRejected(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, err)
				if _, factErr := recordProfitSharingReturnCommandErrorFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, err); factErr != nil {
					return factErr
				}
				return fmt.Errorf("profit sharing return command rejected: %w", err)
			}

			switch returnResp.Result {
			case "SUCCESS":
				if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				} else {
					recordWorkerProfitSharingReturnCommandAccepted(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, returnResp)
				}
				if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, returnResp); factErr != nil {
					return fmt.Errorf("record profit sharing return success fact: %w", factErr)
				}
				if processor.distributor != nil {
					if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
						ctx,
						&ProfitSharingReturnResultPayload{
							ProfitSharingReturnID: returnRecord.ID,
							OutReturnNo:           returnRecord.OutReturnNo,
							OutOrderNo:            returnRecord.OutOrderNo,
							SubMchID:              returnRecord.SubMchid,
							RefundOrderID:         returnRecord.RefundOrderID,
							RetryCount:            0,
						},
						asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
					); enqErr != nil {
						log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
					}
				}
				hasProcessing = true
			case "PROCESSING":
				if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				} else {
					recordWorkerProfitSharingReturnCommandAccepted(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, returnResp)
				}
				if processor.distributor != nil {
					if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
						ctx,
						&ProfitSharingReturnResultPayload{
							ProfitSharingReturnID: returnRecord.ID,
							OutReturnNo:           returnRecord.OutReturnNo,
							OutOrderNo:            returnRecord.OutOrderNo,
							SubMchID:              returnRecord.SubMchid,
							RefundOrderID:         returnRecord.RefundOrderID,
							RetryCount:            0,
						},
						asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
					); enqErr != nil {
						log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
					}
				}
				hasProcessing = true
			case "FAILED":
				failedErr := errors.New("profit sharing return failed")
				if returnResp.FailReason != "" {
					failedErr = errors.New(returnResp.FailReason)
				}
				recordWorkerProfitSharingReturnCommandRejected(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, failedErr)
				if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				}
				if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, returnResp); factErr != nil {
					return fmt.Errorf("record profit sharing return failed fact: %w", factErr)
				}
				if processor.distributor != nil {
					if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
						ctx,
						&ProfitSharingReturnResultPayload{
							ProfitSharingReturnID: returnRecord.ID,
							OutReturnNo:           returnRecord.OutReturnNo,
							OutOrderNo:            returnRecord.OutOrderNo,
							SubMchID:              returnRecord.SubMchid,
							RefundOrderID:         returnRecord.RefundOrderID,
							RetryCount:            0,
						},
						asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
					); enqErr != nil {
						log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
					}
				}
				hasProcessing = true
			default:
				unknownResultErr := fmt.Errorf("unknown return result: %s", returnResp.Result)
				log.Warn().
					Err(unknownResultErr).
					Int64("profit_sharing_return_id", returnRecord.ID).
					Str("out_return_no", returnRecord.OutReturnNo).
					Str("result", returnResp.Result).
					Msg("profit sharing return request returned unknown result, fallback to polling")
				if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				} else {
					recordWorkerProfitSharingReturnCommandUnknown(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, unknownResultErr)
					if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, returnResp); factErr != nil {
						return fmt.Errorf("record profit sharing return unknown fact: %w", factErr)
					}
				}
				if processor.distributor != nil {
					if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
						ctx,
						&ProfitSharingReturnResultPayload{
							ProfitSharingReturnID: returnRecord.ID,
							OutReturnNo:           returnRecord.OutReturnNo,
							OutOrderNo:            returnRecord.OutOrderNo,
							SubMchID:              returnRecord.SubMchid,
							RefundOrderID:         returnRecord.RefundOrderID,
							RetryCount:            0,
						},
						asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
					); enqErr != nil {
						log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
					}
				}
				hasProcessing = true
				return nil
			}

			return nil
		}

		if profitSharingOrder.PlatformCommission > 0 {
			platformReturnMchID := ""
			if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
				if processor.ordinarySPClient == nil {
					return fmt.Errorf("ordinary service provider client not configured for profit sharing return")
				}
				platformReturnMchID = processor.ordinarySPClient.ServiceProviderMchID()
			} else {
				if processor.ecommerceClient == nil {
					return fmt.Errorf("ecommerce client not configured for profit sharing return")
				}
				platformReturnMchID = processor.ecommerceClient.GetSpMchID()
			}
			outReturnNo := fmt.Sprintf("PR%dPL", refundOrder.ID)
			if err := processReturn(outReturnNo, platformReturnMchID, "平台分账回退", profitSharingOrder.PlatformCommission); err != nil {
				enqueuePendingProfitSharingReturnFactApplications()
				if errors.Is(err, profitSharingReturnTerminalFactQueuedErr) {
					return nil
				}
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("profit sharing return failed"), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("profit sharing return failed")
			}
		}
		if profitSharingOrder.OperatorCommission > 0 {
			outReturnNo := fmt.Sprintf("PR%dOP", refundOrder.ID)
			if err := processReturn(outReturnNo, operatorTarget.Account, "运营商分账回退", profitSharingOrder.OperatorCommission); err != nil {
				enqueuePendingProfitSharingReturnFactApplications()
				if errors.Is(err, profitSharingReturnTerminalFactQueuedErr) {
					return nil
				}
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("profit sharing return failed"), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("profit sharing return failed")
			}
		}
		if profitSharingOrder.RiderAmount > 0 {
			outReturnNo := fmt.Sprintf("PR%dRD", refundOrder.ID)
			if err := processReturn(outReturnNo, riderOpenID, "骑手分账回退", profitSharingOrder.RiderAmount); err != nil {
				enqueuePendingProfitSharingReturnFactApplications()
				if errors.Is(err, profitSharingReturnTerminalFactQueuedErr) {
					return nil
				}
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("profit sharing return failed"), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("profit sharing return failed")
			}
		}
		enqueuePendingProfitSharingReturnFactApplications()
		if hasProcessing {
			return nil
		}
		if len(pendingReturnFactApplications) > 0 {
			return nil
		}
	}

	// 根据支付渠道选择退款 API，与同步退款服务（refund_service.go）保持一致：
	// - profit_sharing（收付通）→ CreateEcommerceRefund，需携带 SubMchID
	// - miniprogram/native 等直连支付 → CreateRefund，直连退款 API
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		refundResp, err := createEcommerceRefundContract(ctx, processor.ecommerceClient, &wechatcontracts.EcommerceRefundRequest{
			SubMchID:    paymentConfig.SubMchID,
			OutTradeNo:  paymentOrder.OutTradeNo,
			OutRefundNo: outRefundNo,
			Reason:      payload.Reason,
			Amount: &wechatcontracts.EcommerceRefundRequestAmount{
				Refund:   payload.RefundAmount,
				Total:    refundRequestTotalAmount(paymentOrder.Amount, payload.RefundAmount),
				Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
			},
		})
		if err != nil {
			logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(fmt.Errorf("call wechat ecommerce refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			return fmt.Errorf("call wechat ecommerce refund API: %w", err)
		}
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundResp.RefundID, Valid: refundResp.RefundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
		recordWorkerEcommerceRefundCommandAccepted(ctx, processor.store, paymentOrder, refundOrder, refundResp.RefundID)
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("status", wechatcontracts.EcommerceRefundStatusProcessing).
			Msg("ecommerce refund request processed")
	} else if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		refundResp, err := processor.ordinarySPClient.CreateRefund(ctx, ospcontracts.RefundCreateRequest{
			SubMchID:    paymentConfig.SubMchID,
			OutTradeNo:  paymentOrder.OutTradeNo,
			OutRefundNo: outRefundNo,
			Reason:      payload.Reason,
			NotifyURL:   processor.ordinarySPClient.RefundNotifyURL(),
			Amount: ospcontracts.RefundAmountRequest{
				Refund:   payload.RefundAmount,
				Total:    refundRequestTotalAmount(paymentOrder.Amount, payload.RefundAmount),
				Currency: ospcontracts.CurrencyCNY,
			},
		})
		if err != nil {
			logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(fmt.Errorf("call wechat ordinary service provider refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			recordWorkerEcommerceRefundCommandRejected(ctx, processor.store, paymentOrder, refundOrder, err)
			return fmt.Errorf("call wechat ordinary service provider refund API: %w", err)
		}
		refundID := ""
		if refundResp != nil {
			refundID = refundResp.RefundID
		}
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
		recordWorkerEcommerceRefundCommandAccepted(ctx, processor.store, paymentOrder, refundOrder, refundID)
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("status", wechatcontracts.EcommerceRefundStatusProcessing).
			Msg("ordinary service provider refund request processed")
	} else {
		// 直连支付退款（miniprogram/native 等）
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
	}

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
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(fmt.Errorf("call baofu refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusRejected, err)
		return fmt.Errorf("call baofu refund API: %w", err)
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
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("mark baofu refund order as failed: %w", dbErr)
		}
		recordWorkerBaofuRefundCommand(ctx, processor.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusRejected, nil)
		return fmt.Errorf("baofu refund request rejected: %w", asynq.SkipRetry)
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
// 预定支付已切到收付通，退款必须走电商退款接口，并携带子商户号。
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

	if processor.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured, cannot process reservation refund")
	}
	if !paymentOrderUsesEcommerceChannel(paymentOrder) {
		return mainBusinessRefundChannelDriftError(paymentOrder, "process reservation refund")
	}

	reservation, err := processor.store.GetTableReservation(ctx, payload.ReservationID)
	if err != nil {
		return fmt.Errorf("get reservation: %w", err)
	}
	paymentConfig, err := processor.store.GetMerchantPaymentConfig(ctx, reservation.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
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

	refundResp, err := createEcommerceRefundContract(ctx, processor.ecommerceClient, &wechatcontracts.EcommerceRefundRequest{
		SubMchID:    paymentConfig.SubMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      payload.Reason,
		Amount: &wechatcontracts.EcommerceRefundRequestAmount{
			Refund:   payload.RefundAmount,
			Total:    refundRequestTotalAmount(paymentOrder.Amount, payload.RefundAmount),
			Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
		},
	})
	if err != nil {
		// 保持 pending 状态，由恢复调度器重试
		logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, outRefundNo, paymentOrder.PaymentType, err)
		return fmt.Errorf("reservation refund order %d: call wechat ecommerce refund API: %w", refundOrder.ID, err)
	}

	if err := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: refundResp.RefundID, Valid: refundResp.RefundID != ""},
	}); err != nil {
		return fmt.Errorf("mark reservation refund as processing: %w", err)
	}
	recordWorkerEcommerceRefundCommandAccepted(ctx, processor.store, paymentOrder, refundOrder, refundResp.RefundID)

	log.Info().
		Int64("refund_order_id", refundOrder.ID).
		Str("out_refund_no", outRefundNo).
		Str("status", wechatcontracts.EcommerceRefundStatusProcessing).
		Msg("reservation refund request processed")

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

// ProcessTaskAnomalyRefund 处理已关闭/失败状态支付单收到微信付款的自动退款任务。
// 调用路径：支付回调竞态检测 → 入队 → 此处理器 → CreateEcommerceRefund（TransactionID）
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

	// 确认支付单仍处于 closed/failed（防止并发改状态后重复退款）
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

	// 幂等检查 + 创建退款记录
	refundOrder, err := processor.store.CreateAnomalyRefundRecord(ctx, db.CreateAnomalyRefundRecordParams{
		PaymentOrderID: payload.PaymentOrderID,
		RefundAmount:   payload.RefundAmount,
		OutRefundNo:    payload.OutRefundNo,
	})
	if err != nil {
		return fmt.Errorf("create anomaly refund record: %w", err)
	}

	// 跳过已完成的退款（幂等复用）
	if refundOrder.Status == "success" || refundOrder.Status == "processing" {
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("status", refundOrder.Status).
			Msg("anomaly refund already processed, skip")
		return nil
	}

	var subMchID string
	merchantID := int64(0)
	if requiresEcommerceRefund(paymentOrder) && !paymentOrderUsesEcommerceChannel(paymentOrder) {
		refundErr := mainBusinessRefundChannelDriftError(paymentOrder, "process anomaly refund")
		if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
			return errors.Join(refundErr, fmt.Errorf("mark refund order as failed: %w", dbErr))
		}
		return fmt.Errorf("%w: %w", refundErr, asynq.SkipRetry)
	}

	// 根据支付渠道选择退款 API：
	// - profit_sharing（收付通）→ CreateEcommerceRefund，使用 TransactionID 绕过本地状态约束
	// - miniprogram/native 等直连支付 → CreateRefund，使用 OutTradeNo
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		if processor.ecommerceClient == nil {
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(fmt.Errorf("ecommerce client not configured"), fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			return fmt.Errorf("ecommerce client not configured")
		}

		if paymentOrder.OrderID.Valid {
			order, orderErr := processor.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
			if orderErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get order for merchant lookup: %w", orderErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get order for merchant lookup: %w", orderErr)
			}
			merchantID = order.MerchantID
			cfg, cfgErr := processor.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
			if cfgErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get merchant payment config: %w", cfgErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get merchant payment config: %w", cfgErr)
			}
			subMchID = cfg.SubMchID
		} else if paymentOrder.ReservationID.Valid {
			reservation, resErr := processor.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
			if resErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get reservation for merchant lookup: %w", resErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get reservation for merchant lookup: %w", resErr)
			}
			merchantID = reservation.MerchantID
			cfg, cfgErr := processor.store.GetMerchantPaymentConfig(ctx, reservation.MerchantID)
			if cfgErr != nil {
				if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
					return errors.Join(fmt.Errorf("get merchant payment config: %w", cfgErr), fmt.Errorf("mark refund order as failed: %w", dbErr))
				}
				return fmt.Errorf("get merchant payment config: %w", cfgErr)
			}
			subMchID = cfg.SubMchID
		} else {
			// 无法确定商户，标记失败并告警（不重试）
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return fmt.Errorf("mark refund order as failed: %w", dbErr)
			}
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeRefundFailed,
				Level:       AlertLevelCritical,
				Title:       "⚠️ 异常退款无法确定商户",
				Message:     fmt.Sprintf("支付单 %d 的异常退款无法确定 SubMchID（OrderID 和 ReservationID 均为空），请人工处理", payload.PaymentOrderID),
				RelatedID:   payload.PaymentOrderID,
				RelatedType: "payment_order",
				Extra: mergeAlertExtra(paymentOrderAlertExtra(paymentOrder, 0), map[string]interface{}{
					"transaction_id": payload.TransactionID,
					"refund_amount":  payload.RefundAmount,
					"out_refund_no":  payload.OutRefundNo,
				}),
			})
			// 不可重试：返回 nil 防止 asynq 无限重试
			return nil
		}

		refundResp, err := createEcommerceRefundContract(ctx, processor.ecommerceClient, &wechatcontracts.EcommerceRefundRequest{
			SubMchID:      subMchID,
			TransactionID: payload.TransactionID,
			OutRefundNo:   payload.OutRefundNo,
			Reason:        "已关闭订单异常到账，系统自动退款",
			Amount: &wechatcontracts.EcommerceRefundRequestAmount{
				Refund:   payload.RefundAmount,
				Total:    paymentOrder.Amount,
				Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
			},
		})
		if err != nil {
			logRefundRequestFailure(refundOrder.ID, paymentOrder.ID, payload.OutRefundNo, paymentOrder.PaymentType, err)
			if dbErr := processor.markRefundOrderFailed(ctx, refundOrder.ID); dbErr != nil {
				return errors.Join(fmt.Errorf("call wechat ecommerce refund API: %w", err), fmt.Errorf("mark refund order as failed: %w", dbErr))
			}
			return fmt.Errorf("call wechat ecommerce refund API: %w", err)
		}
		if dbErr := processor.markRefundOrderProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundResp.RefundID, Valid: refundResp.RefundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("mark refund order as processing: %w", dbErr)
		}
		recordWorkerEcommerceRefundCommandAccepted(ctx, processor.store, paymentOrder, refundOrder, refundResp.RefundID)
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("refund_id", refundResp.RefundID).
			Msg("anomaly ecommerce refund accepted, waiting for callback confirmation")
	} else {
		// 直连支付退款（miniprogram/native 等），使用 OutTradeNo
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
				Title:       "⚠️ 异常退款接口返回非预期状态",
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
	}

	return nil
}

func recordWorkerEcommerceRefundCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundID string) {
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerEcommerceRefundCommandInput(
		paymentOrder,
		refundOrder,
		db.ExternalPaymentCommandStatusAccepted,
		workerStringPtrIfNotEmpty(refundID),
		nil,
		nil,
		workerEcommerceRefundCommandSnapshot(map[string]string{
			"out_refund_no": refundOrder.OutRefundNo,
			"refund_id":     refundID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("record worker ecommerce refund command accepted failed")
	}
}

func recordWorkerEcommerceRefundCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, commandErr error) {
	errorCode, errorMessage := workerPaymentCommandErrorFields(commandErr)
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerEcommerceRefundCommandInput(
		paymentOrder,
		refundOrder,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		workerEcommerceRefundCommandSnapshot(map[string]string{
			"out_refund_no": refundOrder.OutRefundNo,
			"error_code":    workerStringValue(errorCode),
			"error_message": workerStringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("record worker ecommerce refund command rejected failed")
	}
}

func recordProfitSharingCommandAccepted(ctx context.Context, store db.Store, channel string, profitSharingOrder db.ProfitSharingOrder, commandType string, sharingOrderID string, status string) {
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbProfitSharingCommandInput(
		channel,
		profitSharingOrder,
		commandType,
		workerStringPtrIfNotEmpty(sharingOrderID),
		workerProfitSharingCommandSnapshot(map[string]string{
			"out_order_no": profitSharingOrder.OutOrderNo,
			"order_id":     sharingOrderID,
			"status":       status,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", profitSharingOrder.OutOrderNo).
			Str("command_type", commandType).
			Msg("record profit sharing command accepted failed")
	}
}

func recordWorkerProfitSharingReturnCommandAccepted(ctx context.Context, store db.Store, channel string, returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) {
	returnID := ""
	result := ""
	if returnResp != nil {
		returnID = returnResp.ReturnID
		result = returnResp.Result
	}
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerProfitSharingReturnCommandInput(
		channel,
		returnRecord,
		db.ExternalPaymentCommandStatusAccepted,
		workerStringPtrIfNotEmpty(returnID),
		nil,
		nil,
		workerProfitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"return_id":     returnID,
			"result":        result,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record worker profit sharing return command accepted failed")
	}
}

func recordWorkerProfitSharingReturnCommandUnknown(ctx context.Context, store db.Store, channel string, returnRecord db.ProfitSharingReturn, commandErr error) {
	errorCode, errorMessage := workerPaymentCommandErrorFields(commandErr)
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerProfitSharingReturnCommandInput(
		channel,
		returnRecord,
		db.ExternalPaymentCommandStatusUnknown,
		nil,
		errorCode,
		errorMessage,
		workerProfitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"error_code":    workerStringValue(errorCode),
			"error_message": workerStringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record worker profit sharing return command unknown failed")
	}
}

func recordWorkerProfitSharingReturnCommandRejected(ctx context.Context, store db.Store, channel string, returnRecord db.ProfitSharingReturn, commandErr error) {
	errorCode, errorMessage := workerPaymentCommandErrorFields(commandErr)
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbWorkerProfitSharingReturnCommandInput(
		channel,
		returnRecord,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		workerProfitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"error_code":    workerStringValue(errorCode),
			"error_message": workerStringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record worker profit sharing return command rejected failed")
	}
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
			"out_refund_no": refundOrder.OutRefundNo,
			"refund_id":     refundID,
			"result_code":   resultCode,
			"refund_state":  refundState,
			"error_code":    errorCode,
			"error_message": errorMessage,
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

func workerStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
	var ordinaryErr *ordinaryserviceprovider.ProviderError
	if errors.As(err, &ordinaryErr) {
		return workerStringPtrIfNotEmpty(ordinaryErr.ProviderCode), workerStringPtrIfNotEmpty(workerOrdinaryProviderFrontendCommandMessage(ordinaryErr.Frontend))
	}
	if err == nil {
		return nil, nil
	}
	return nil, workerStringPtrIfNotEmpty(err.Error())
}

func workerOrdinaryProviderFrontendCommandMessage(frontend ordinaryserviceprovider.FrontendGuidance) string {
	message := strings.TrimSpace(frontend.Message)
	if action := strings.TrimSpace(frontend.Action); action != "" {
		message = strings.TrimSpace(message + "，" + action)
	}
	return message
}

func dbWorkerEcommerceRefundCommandInput(
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
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentOrder.PaymentChannel,
		Capability:           workerRefundCommandCapability(paymentOrder.PaymentChannel),
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        workerEcommerceRefundBusinessOwner(paymentOrder),
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    refundOrder.OutRefundNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
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

func workerRefundCommandCapability(channel string) string {
	if channel == db.PaymentChannelOrdinaryServiceProvider {
		return db.ExternalPaymentCapabilityPartnerRefund
	}
	return db.ExternalPaymentCapabilityEcommerceRefund
}

func dbProfitSharingCommandInput(
	channel string,
	profitSharingOrder db.ProfitSharingOrder,
	commandType string,
	externalSecondaryKey *string,
	responseSnapshot []byte,
) logic.RecordExternalPaymentCommandInput {
	businessObjectType := "profit_sharing_order"
	businessObjectID := profitSharingOrder.ID
	return logic.RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              channel,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		CommandType:          commandType,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    profitSharingOrder.OutOrderNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        db.ExternalPaymentCommandStatusAccepted,
		ResponseSnapshot:     responseSnapshot,
	}
}

func dbWorkerProfitSharingReturnCommandInput(
	channel string,
	returnRecord db.ProfitSharingReturn,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) logic.RecordExternalPaymentCommandInput {
	businessObjectType := "profit_sharing_return"
	businessObjectID := returnRecord.ID
	return logic.RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              channel,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		CommandType:          db.ExternalPaymentCommandTypeCreateProfitSharingReturn,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func workerEcommerceRefundBusinessOwner(paymentOrder db.PaymentOrder) string {
	if paymentOrder.BusinessType == "reservation" || paymentOrder.BusinessType == "reservation_addon" || paymentOrder.ReservationID.Valid {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return db.ExternalPaymentBusinessOwnerOrder
}

func workerEcommerceRefundCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func workerProfitSharingCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func workerProfitSharingReturnCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
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
		Str("applyment_status", payload.ApplymentStatus).
		Str("sign_state", payload.SignState).
		Str("sub_mch_id", payload.SubMchID).
		Msg("processing applyment result")

	if payload.SubjectType != "merchant" {
		log.Warn().
			Int64("applyment_id", payload.ApplymentID).
			Str("subject_type", payload.SubjectType).
			Msg("skip non-merchant applyment subject type")
		return nil
	}

	status := resolveApplymentResultStatus(payload)

	// 根据进件状态处理
	switch status {
	case "finish":
		// 进件成功，需要：
		// 1. 发送成功通知
		// 2. 添加为分账接收方
		if err := processor.handleApplymentSuccess(ctx, &payload); err != nil {
			return err
		}

	case "rejected":
		// 进件被驳回，发送通知
		if err := processor.handleApplymentRejected(ctx, &payload); err != nil {
			return err
		}

	case "account_need_verify", "to_be_confirmed", "to_be_signed":
		// 待账户验证/待确认/待签约，发送提醒通知
		if err := processor.handleApplymentPending(ctx, &payload); err != nil {
			return err
		}

	case "frozen", "canceled":
		// 已冻结/已作废，发送终态通知
		if err := processor.handleApplymentTerminalState(ctx, &payload); err != nil {
			return err
		}

	default:
		log.Info().
			Str("state", status).
			Msg("applyment state does not require async processing")
		return nil
	}

	if processor.store != nil {
		if err := processor.store.MarkEcommerceApplymentResultProcessed(ctx, db.MarkEcommerceApplymentResultProcessedParams{
			ID:                       payload.ApplymentID,
			ResultTaskProcessedState: pgtype.Text{String: status, Valid: status != ""},
		}); err != nil {
			return fmt.Errorf("mark applyment result processed: %w", err)
		}
	}

	return nil
}

// handleApplymentSuccess 处理进件成功
func (processor *RedisTaskProcessor) handleApplymentSuccess(ctx context.Context, payload *ApplymentResultPayload) error {
	var merchant db.Merchant
	if payload.SubjectType == "merchant" {
		loadedMerchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
		if err != nil {
			log.Error().Err(err).Int64("merchant_id", payload.SubjectID).Msg("get merchant for applyment success")
			return nil
		}
		merchant = loadedMerchant
	}

	// 发送进件完成通知。当前主链下，商户自身是分账出资方，不在这里追加为分账接收方。
	userID := merchant.OwnerUserID
	notifyContent := fmt.Sprintf("您的商户「%s」已完成微信支付开户，可以开始接单收款了！", merchant.Name)

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		processor.distributeTaskSendNotificationWithLog(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户成功",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		}, "send applyment success notification failed")
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

	rejectReason := "请登录后台查看详情"
	if applyment.RejectReason.Valid {
		rejectReason = applyment.RejectReason.String
	}
	merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", payload.SubjectID).Msg("get merchant")
		return nil
	}
	userID := merchant.OwnerUserID
	notifyContent := fmt.Sprintf("您的商户「%s」微信支付开户申请被驳回，原因：%s", merchant.Name, rejectReason)

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		processor.distributeTaskSendNotificationWithLog(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户被驳回",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		}, "send applyment rejected notification failed")
	}

	return nil
}

// handleApplymentPending 处理待账户验证/待确认/待签约
func (processor *RedisTaskProcessor) handleApplymentPending(ctx context.Context, payload *ApplymentResultPayload) error {
	resolvedStatus := resolveApplymentResultStatus(*payload)
	merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
	if err != nil {
		return nil
	}
	userID := merchant.OwnerUserID
	var notifyContent string
	switch resolvedStatus {
	case "account_need_verify":
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户需要完成账户验证，请按页面指引完成汇款验证或法人扫码验证", merchant.Name)
	case "to_be_confirmed":
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户需要确认，请登录微信支付商户平台完成确认", merchant.Name)
	default:
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户需要签约，请登录微信支付商户平台完成签约", merchant.Name)
	}

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(3 * 24 * time.Hour)
		processor.distributeTaskSendNotificationWithLog(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       "微信支付开户待处理",
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		}, "send applyment pending notification failed")
	}

	return nil
}

func (processor *RedisTaskProcessor) handleApplymentTerminalState(ctx context.Context, payload *ApplymentResultPayload) error {
	resolvedStatus := resolveApplymentResultStatus(*payload)
	merchant, err := processor.store.GetMerchant(ctx, payload.SubjectID)
	if err != nil {
		return nil
	}

	userID := merchant.OwnerUserID
	var title string
	var notifyContent string
	switch resolvedStatus {
	case "frozen":
		title = "微信支付开户已冻结"
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户状态已被冻结，请登录后台查看详情并联系平台处理", merchant.Name)
	default:
		title = "微信支付开户已作废"
		notifyContent = fmt.Sprintf("您的商户「%s」微信支付开户申请已作废，请检查资料后重新发起申请", merchant.Name)
	}

	if userID > 0 && processor.distributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		processor.distributeTaskSendNotificationWithLog(ctx, &SendNotificationPayload{
			UserID:      userID,
			Type:        "system",
			Title:       title,
			Content:     notifyContent,
			RelatedType: "applyment",
			RelatedID:   payload.ApplymentID,
			ExpiresAt:   &expiresAt,
		}, "send applyment terminal notification failed")
	}

	return nil
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

		// 自动重试队列（指数退避延迟，避免微信端短暂异常导致永久失败）
		if processor.distributor != nil {
			paymentOrder, err := processor.store.GetPaymentOrder(ctx, profitSharingOrder.PaymentOrderID)
			if err != nil {
				if requireEnqueueSuccess {
					return fmt.Errorf("get payment order for profit sharing retry: %w", err)
				}
				return nil
			}
			retryPayload, ok := buildProfitSharingPayloadFromPaymentOrder(paymentOrder)
			if !ok {
				log.Warn().
					Int64("payment_order_id", profitSharingOrder.PaymentOrderID).
					Msg("payment order missing order_id and reservation_id, skip profit sharing retry enqueue")
				return nil
			}
			// 首次从回调失败进入 → 5min 延迟；Unique 防止重复入队
			err = processor.distributor.DistributeTaskProcessProfitSharing(
				ctx,
				&retryPayload,
				asynq.Queue(QueueCritical),
				asynq.ProcessIn(5*time.Minute),
				asynq.MaxRetry(5),
				asynq.Unique(6*time.Minute),
			)
			if err != nil && requireEnqueueSuccess {
				return fmt.Errorf("enqueue profit sharing retry after result failure: %w", err)
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

	title := "配送费已到账"
	content := fmt.Sprintf("本单配送费%.2f元已通过微信分账到账，可在收入账本查看。", float64(profitSharingOrder.RiderAmount)/100)
	if result != "SUCCESS" {
		title = "配送费结算处理中"
		content = "本单配送费结算暂未完成，平台正在核对处理，可在收入账本查看状态。"
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

// ==================== 分账回退结果处理 ====================

// ProcessTaskProfitSharingReturnResult 处理分账回退结果任务
func (processor *RedisTaskProcessor) ProcessTaskProfitSharingReturnResult(ctx context.Context, task *asynq.Task) error {
	var payload ProfitSharingReturnResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	returnRecord, err := processor.store.GetProfitSharingReturnByOutReturnNo(ctx, payload.OutReturnNo)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("profit sharing return not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get profit sharing return: %w", err)
	}
	paymentOrder := db.PaymentOrder{ID: returnRecord.PaymentOrderID, PaymentChannel: db.PaymentChannelEcommerce}
	if processor.ordinarySPClient != nil {
		paymentOrder, err = processor.store.GetPaymentOrder(ctx, returnRecord.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get payment order for profit sharing return: %w", err)
		}
	}
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if processor.ordinarySPClient == nil {
			return fmt.Errorf("ordinary service provider client not configured")
		}
	} else if processor.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured")
	}

	resp, err := processor.queryWechatProfitSharingReturn(ctx, paymentOrder, returnRecord)
	if err != nil {
		return fmt.Errorf("query profit sharing return: %w", err)
	}

	switch resp.Result {
	case "PROCESSING":
		if _, dbErr := processor.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
			ID:       returnRecord.ID,
			ReturnID: pgtype.Text{String: resp.ReturnID, Valid: resp.ReturnID != ""},
		}); dbErr != nil {
			log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
		}
		if payload.RetryCount+1 > processor.config.ProfitSharingReturnMaxRetries {
			log.Warn().
				Int64("profit_sharing_return_id", returnRecord.ID).
				Int64("refund_order_id", returnRecord.RefundOrderID).
				Int("retry_count", payload.RetryCount+1).
				Msg("profit sharing return result poll reached max retries; keep processing for recovery scheduler")
			return nil
		}
		if processor.distributor != nil {
			if enqErr := processor.distributor.DistributeTaskProcessProfitSharingReturnResult(
				ctx,
				&ProfitSharingReturnResultPayload{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            payload.RetryCount + 1,
				},
				asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
			); enqErr != nil {
				log.Error().Err(enqErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to re-enqueue profit sharing return result task")
			}
		}
		return nil

	case "SUCCESS":
		application, factErr := recordProfitSharingReturnQueryFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, resp)
		if factErr != nil {
			return fmt.Errorf("record profit sharing return success fact: %w", factErr)
		}
		enqueueProfitSharingReturnPaymentFactApplication(ctx, processor.distributor, application)
		return nil

	case "FAILED":
		application, factErr := recordProfitSharingReturnQueryFact(ctx, processor.store, profitSharingPaymentChannel(paymentOrder), returnRecord, resp)
		if factErr != nil {
			return fmt.Errorf("record profit sharing return failed fact: %w", factErr)
		}
		enqueueProfitSharingReturnPaymentFactApplication(ctx, processor.distributor, application)
		return nil
	default:
		log.Error().
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Str("result", resp.Result).
			Msg("profit sharing return query returned unsupported result")
		return fmt.Errorf("unknown profit sharing return result: %s: %w", resp.Result, asynq.SkipRetry)
	}
}

func normalizeProfitSharingPayload(payload *ProfitSharingPayload) ProfitSharingPayload {
	if payload == nil {
		return ProfitSharingPayload{}
	}

	normalized := *payload
	if normalized.IdempotencyKey == "" {
		normalized.IdempotencyKey = profitSharingTaskIdempotencyKey(normalized)
	}
	return normalized
}

func buildProfitSharingPayloadFromPaymentOrder(paymentOrder db.PaymentOrder) (ProfitSharingPayload, bool) {
	payload := ProfitSharingPayload{PaymentOrderID: paymentOrder.ID}
	if paymentOrder.OrderID.Valid {
		payload.OrderID = paymentOrder.OrderID.Int64
		return payload, true
	}
	if paymentOrder.ReservationID.Valid {
		payload.ReservationID = paymentOrder.ReservationID.Int64
		return payload, true
	}
	return ProfitSharingPayload{}, false
}

func profitSharingTaskIdempotencyKey(payload ProfitSharingPayload) string {
	if payload.PaymentOrderID > 0 {
		return fmt.Sprintf("profit_sharing:payment_order:%d", payload.PaymentOrderID)
	}
	if payload.OrderID > 0 {
		return fmt.Sprintf("profit_sharing:order:%d", payload.OrderID)
	}
	if payload.ReservationID > 0 {
		return fmt.Sprintf("profit_sharing:reservation:%d", payload.ReservationID)
	}
	return "profit_sharing:unknown"
}
