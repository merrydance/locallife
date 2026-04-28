package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	TaskOperatorPendingDispatchAlert = "operator:pending_dispatch_alert"
)

type OperatorPendingDispatchAlertPayload struct {
	DeliveryID       int64  `json:"delivery_id"`
	AlertKey         string `json:"alert_key"`
	ThresholdMinutes int32  `json:"threshold_minutes"`
}

func (distributor *RedisTaskDistributor) DistributeTaskOperatorPendingDispatchAlert(
	ctx context.Context,
	payload *OperatorPendingDispatchAlertPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskOperatorPendingDispatchAlert, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("delivery_id", payload.DeliveryID).
		Str("alert_key", payload.AlertKey).
		Msg("enqueued operator pending dispatch alert task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskOperatorPendingDispatchAlert(ctx context.Context, task *asynq.Task) error {
	var payload OperatorPendingDispatchAlertPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	delivery, err := processor.store.GetDelivery(ctx, payload.DeliveryID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Info().Int64("delivery_id", payload.DeliveryID).Msg("skip operator pending dispatch alert: delivery not found")
			return nil
		}
		return fmt.Errorf("get delivery: %w", err)
	}

	if delivery.Status != "pending" {
		log.Info().Int64("delivery_id", delivery.ID).Str("status", delivery.Status).Msg("skip operator pending dispatch alert: delivery no longer pending")
		return nil
	}

	threshold := time.Duration(payload.ThresholdMinutes) * time.Minute
	if threshold <= 0 {
		threshold = 3 * time.Minute
	}
	if time.Since(delivery.CreatedAt) < threshold {
		log.Debug().Int64("delivery_id", delivery.ID).Msg("skip operator pending dispatch alert: threshold not reached")
		return nil
	}

	order, err := processor.store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Info().Int64("delivery_id", delivery.ID).Int64("order_id", delivery.OrderID).Msg("skip operator pending dispatch alert: order not found")
			return nil
		}
		return fmt.Errorf("get order: %w", err)
	}

	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Info().Int64("delivery_id", delivery.ID).Int64("merchant_id", order.MerchantID).Msg("skip operator pending dispatch alert: merchant not found")
			return nil
		}
		return fmt.Errorf("get merchant: %w", err)
	}

	recipients, err := processor.store.ListActiveOperatorNotificationRecipientsByRegion(ctx, merchant.RegionID)
	if err != nil {
		return fmt.Errorf("list active operator notification recipients by region: %w", err)
	}
	if len(recipients) == 0 {
		log.Info().Int64("delivery_id", delivery.ID).Int64("region_id", merchant.RegionID).Msg("skip operator pending dispatch alert: no active operator recipient")
		return nil
	}

	waitMinutes := int32(time.Since(delivery.CreatedAt).Minutes())
	if waitMinutes < payload.ThresholdMinutes {
		waitMinutes = payload.ThresholdMinutes
	}

	for _, recipient := range recipients {
		notificationPayload := SendNotificationPayload{
			UserID:      recipient.UserID,
			Type:        "delivery",
			Title:       "待接单提醒",
			Content:     "区域内有订单超过3分钟未被骑手接单，请尽快提醒骑手接单。",
			RelatedType: "delivery",
			RelatedID:   delivery.ID,
			ExtraData: map[string]any{
				"audience":          "operator",
				"category":          "dispatch_timeout",
				"level":             "warning",
				"summary":           "有订单超过3分钟仍未被骑手接单",
				"region_id":         recipient.RegionID,
				"region_name":       recipient.RegionName,
				"threshold_minutes": payload.ThresholdMinutes,
				"wait_minutes":      waitMinutes,
				"alert_key":         payload.AlertKey,
			},
			IgnorePreferences: true,
		}

		if err := processor.executeSendNotificationPayload(ctx, notificationPayload); err != nil {
			return fmt.Errorf("send operator pending dispatch notification to user %d: %w", recipient.UserID, err)
		}
	}

	log.Info().
		Int64("delivery_id", delivery.ID).
		Int64("region_id", merchant.RegionID).
		Int("recipient_count", len(recipients)).
		Msg("processed operator pending dispatch alert")

	return nil
}
