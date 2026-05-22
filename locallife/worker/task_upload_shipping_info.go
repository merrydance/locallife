package worker

import (
	"context"
	"encoding/json"
	"fmt"
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

	notifyURL := processor.config.WechatShippingSettleNotifyURL

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

	switch {
	case po.PaymentType == "miniprogram":
		transactionID := ""
		if po.TransactionID.Valid {
			transactionID = po.TransactionID.String
		}
		if transactionID == "" && po.OutTradeNo == "" {
			log.Warn().Int64("payment_order_id", po.ID).Msg("shipping upload: no transaction_id or out_trade_no, skip")
			return nil
		}

		if err := processor.wechatClient.UploadShippingInfo(ctx, &wechat.UploadShippingInfoRequest{
			TransactionID: transactionID,
			OutTradeNo:    po.OutTradeNo,
			PayerOpenID:   user.WechatOpenid,
			NotifyURL:     notifyURL,
			UploadTime:    now,
		}); err != nil {
			return fmt.Errorf("upload_shipping_info failed: %w", err)
		}
		log.Info().Int64("order_id", payload.OrderID).Msg("upload_shipping_info ok")

	default:
		// 余额支付、宝付主业务等无需上报
	}

	return nil
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
