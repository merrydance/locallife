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
	TaskSendNotification = "notification:send"
)

// SendNotificationPayload 发送通知任务载荷
type SendNotificationPayload struct {
	UserID      int64          `json:"user_id"`
	Type        string         `json:"type"` // order/payment/delivery/system/food_safety
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	RelatedType string         `json:"related_type,omitempty"` // order/payment/delivery/merchant/rider
	RelatedID   int64          `json:"related_id,omitempty"`
	ExtraData   map[string]any `json:"extra_data,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
}

// DistributeTaskSendNotification 分发发送通知任务
func (distributor *RedisTaskDistributor) DistributeTaskSendNotification(
	ctx context.Context,
	payload *SendNotificationPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskSendNotification, jsonPayload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("user_id", payload.UserID).
		Str("notification_type", payload.Type).
		Msg("enqueued notification task")

	return nil
}

// ProcessTaskSendNotification 处理发送通知任务
func (processor *RedisTaskProcessor) ProcessTaskSendNotification(ctx context.Context, task *asynq.Task) error {
	var payload SendNotificationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Info().
		Int64("user_id", payload.UserID).
		Str("type", payload.Type).
		Str("title", payload.Title).
		Msg("processing send notification task")

	// 构建extra_data
	var extraDataJSON []byte
	if payload.ExtraData != nil {
		var err error
		extraDataJSON, err = json.Marshal(payload.ExtraData)
		if err != nil {
			return fmt.Errorf("marshal extra_data: %w", err)
		}
	}

	// 创建通知记录
	createParams := db.CreateNotificationParams{
		UserID:  payload.UserID,
		Type:    payload.Type,
		Title:   payload.Title,
		Content: payload.Content,
	}

	if payload.RelatedType != "" {
		createParams.RelatedType = pgtype.Text{String: payload.RelatedType, Valid: true}
	}

	if payload.RelatedID > 0 {
		createParams.RelatedID = pgtype.Int8{Int64: payload.RelatedID, Valid: true}
	}

	if len(extraDataJSON) > 0 {
		createParams.ExtraData = extraDataJSON
	}

	if payload.ExpiresAt != nil {
		createParams.ExpiresAt = pgtype.Timestamptz{Time: *payload.ExpiresAt, Valid: true}
	}

	notification, err := processor.store.CreateNotification(ctx, createParams)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	log.Info().
		Int64("notification_id", notification.ID).
		Int64("user_id", payload.UserID).
		Str("type", payload.Type).
		Msg("✅ notification created successfully")

	// 🔥 WebSocket实时推送：通过Redis Pub/Sub通知API服务器
	// 需要判断用户角色，确定推送给骑手还是商户
	if err := processor.tryWebSocketPush(ctx, payload.UserID, notification); err != nil {
		log.Error().Err(err).Int64("notification_id", notification.ID).Msg("WebSocket push failed (non-critical)")
		// 推送失败不影响主流程，通知已经存入数据库
	}

	return nil
}

// tryWebSocketPush 尝试通过WebSocket推送通知（如果用户是骑手或商户）
func (processor *RedisTaskProcessor) tryWebSocketPush(ctx context.Context, userID int64, notification db.Notification) error {
	// 查询用户角色
	roles, err := processor.store.ListUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("list user roles: %w", err)
	}

	// 构建WebSocket消息
	notificationData, _ := json.Marshal(map[string]any{
		"id":         notification.ID,
		"user_id":    notification.UserID,
		"type":       notification.Type,
		"title":      notification.Title,
		"content":    notification.Content,
		"is_read":    notification.IsRead,
		"created_at": notification.CreatedAt,
	})

	wsMessage := map[string]any{
		"type":      "notification",
		"data":      json.RawMessage(notificationData),
		"timestamp": time.Now(),
	}

	wsMessageJSON, _ := json.Marshal(wsMessage)

	pushed := false

	// 检查是否是骑手
	for _, role := range roles {
		if role.Role == "rider" && role.RelatedEntityID.Valid {
			riderID := role.RelatedEntityID.Int64

			// 通过Redis Pub/Sub发布推送请求
			channel := fmt.Sprintf("notification:rider:%d", riderID)
			if err := processor.redisClient.Publish(ctx, channel, wsMessageJSON).Err(); err != nil {
				log.Error().Err(err).Int64("rider_id", riderID).Msg("publish to redis failed")
			} else {
				pushed = true
				log.Debug().Int64("rider_id", riderID).Msg("published notification push request to Redis")
			}
			break
		}
	}

	// 检查是否是商户
	for _, role := range roles {
		if role.Role == "merchant" && role.RelatedEntityID.Valid {
			merchantID := role.RelatedEntityID.Int64

			// 通过Redis Pub/Sub发布推送请求
			channel := fmt.Sprintf("notification:merchant:%d", merchantID)
			if err := processor.redisClient.Publish(ctx, channel, wsMessageJSON).Err(); err != nil {
				log.Error().Err(err).Int64("merchant_id", merchantID).Msg("publish to redis failed")
			} else {
				pushed = true
				log.Debug().Int64("merchant_id", merchantID).Msg("published notification push request to Redis")
			}
			break
		}
	}

	// 标记已推送
	if pushed {
		if err := processor.store.MarkNotificationAsPushed(ctx, notification.ID); err != nil {
			log.Error().Err(err).Int64("notification_id", notification.ID).Msg("mark as pushed failed")
		}
	}

	return nil
}
