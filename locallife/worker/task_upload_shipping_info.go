package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	// TaskUploadShippingInfo 微信发货信息上报任务（合规要求）
	TaskUploadShippingInfo = "task:upload_shipping_info"
)

// UploadShippingInfoPayload 发货信息上报任务载荷
type UploadShippingInfoPayload struct {
	OrderID int64 `json:"order_id"`
	UserID  int64 `json:"user_id"`
}

// NewUploadShippingInfoTask 创建发货信息上报任务
func NewUploadShippingInfoTask(payload *UploadShippingInfoPayload) (*asynq.Task, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskUploadShippingInfo, jsonPayload), nil
}

// ProcessTaskUploadShippingInfo 处理发货信息上报任务
func (processor *RedisTaskProcessor) ProcessTaskUploadShippingInfo(ctx context.Context, task *asynq.Task) error {
	var payload UploadShippingInfoPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal upload_shipping_info payload: %w", asynq.SkipRetry)
	}

	if processor.wechatClient == nil {
		// 微信客户端未配置，无法上报，跳过重试
		return fmt.Errorf("wechat client not configured: %w", asynq.SkipRetry)
	}

	// 1. 查询支付订单
	po, err := processor.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: payload.OrderID, Valid: true},
		BusinessType: "order",
	})
	if err != nil {
		// DB 错误可重试
		return fmt.Errorf("shipping upload: get payment order failed: %w", err)
	}
	if po.Status != "paid" {
		log.Debug().Int64("order_id", payload.OrderID).Str("status", po.Status).Msg("shipping upload: payment order not paid, skip")
		return nil
	}

	// 2. 获取用户 openid
	user, err := processor.store.GetUser(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("shipping upload: get user failed: %w", err)
	}
	if user.WechatOpenid == "" {
		log.Warn().Int64("user_id", payload.UserID).Msg("shipping upload: user has no openid, skip")
		return nil
	}

	now := time.Now()

	switch po.PaymentChannel {
	case db.PaymentChannelDirect, db.PaymentChannelBaofuAggregate:
		transactionID := shippingUploadTransactionID(po)
		if transactionID == "" && po.OutTradeNo == "" {
			log.Warn().Int64("payment_order_id", po.ID).Msg("shipping upload: no transaction_id or out_trade_no, skip")
			return nil
		}
		mchID := shippingUploadMchID(po)
		if transactionID == "" && mchID == "" {
			log.Warn().Int64("payment_order_id", po.ID).Msg("shipping upload: merchant order key missing mchid, skip")
			return nil
		}
		itemDesc, err := processor.shippingUploadItemDesc(ctx, payload.OrderID)
		if err != nil {
			return err
		}
		if itemDesc == "" {
			log.Warn().Int64("order_id", payload.OrderID).Msg("shipping upload: item desc empty, skip")
			return nil
		}

		if err := processor.wechatClient.UploadShippingInfo(ctx, &wechat.UploadShippingInfoRequest{
			TransactionID: transactionID,
			OutTradeNo:    po.OutTradeNo,
			MchID:         mchID,
			PayerOpenID:   user.WechatOpenid,
			ItemDesc:      itemDesc,
			UploadTime:    now,
		}); err != nil {
			return fmt.Errorf("upload_shipping_info failed: %w", err)
		}
		log.Info().Int64("order_id", payload.OrderID).Msg("upload_shipping_info ok")

	default:
		log.Debug().
			Int64("order_id", payload.OrderID).
			Str("payment_channel", po.PaymentChannel).
			Msg("shipping upload: unsupported payment channel, skip")
	}

	return nil
}

func (processor *RedisTaskProcessor) shippingUploadItemDesc(ctx context.Context, orderID int64) (string, error) {
	items, err := processor.store.ListOrderItemsByOrder(ctx, orderID)
	if err != nil {
		return "", fmt.Errorf("shipping upload: list order items failed: %w", err)
	}
	return buildShippingItemDesc(items), nil
}

func buildShippingItemDesc(items []db.OrderItem) string {
	const maxShippingItemDescRunes = 120

	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		parts = append(parts, fmt.Sprintf("%sx%d", name, quantity))
	}
	desc := strings.Join(parts, "、")
	if len([]rune(desc)) <= maxShippingItemDescRunes {
		return desc
	}
	runes := []rune(desc)
	return string(runes[:maxShippingItemDescRunes])
}

func shippingUploadTransactionID(paymentOrder db.PaymentOrder) string {
	if paymentOrder.PaymentChannel != db.PaymentChannelDirect {
		return ""
	}
	if !paymentOrder.TransactionID.Valid {
		return ""
	}
	return strings.TrimSpace(paymentOrder.TransactionID.String)
}

func shippingUploadMchID(paymentOrder db.PaymentOrder) string {
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		return ""
	}
	return shippingUploadSubMchIDFromAttach(paymentOrder.Attach.String, paymentOrder.Attach.Valid)
}

func shippingUploadSubMchIDFromAttach(attach string, valid bool) string {
	if !valid {
		return ""
	}
	for _, segment := range strings.Split(strings.TrimSpace(attach), ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(segment), ":")
		if !ok || strings.TrimSpace(key) != "sub_mchid" {
			continue
		}
		return strings.TrimSpace(value)
	}
	return ""
}

// DistributeTaskUploadShippingInfo 分发发货信息上报任务
func (distributor *RedisTaskDistributor) DistributeTaskUploadShippingInfo(
	ctx context.Context,
	payload *UploadShippingInfoPayload,
	opts ...asynq.Option,
) error {
	task, err := NewUploadShippingInfoTask(payload)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, err = distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	return nil
}
