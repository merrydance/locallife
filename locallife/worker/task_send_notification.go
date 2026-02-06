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
	// 是否忽略用户通知偏好（用于关键通知）
	IgnorePreferences bool `json:"ignore_preferences,omitempty"`
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
	info, err := distributor.enqueueTask(ctx, task)
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

	// 检查用户通知偏好设置（除非指定忽略）
	shouldPush := true
	if !payload.IgnorePreferences {
		prefs, err := processor.store.GetUserNotificationPreferences(ctx, payload.UserID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			log.Error().Err(err).Int64("user_id", payload.UserID).Msg("failed to get notification preferences")
		} else if err == nil {
			if !processor.isNotificationEnabled(prefs, payload.Type) {
				log.Debug().
					Int64("user_id", payload.UserID).
					Str("type", payload.Type).
					Msg("notification disabled by user preference")
				return nil
			}
			if processor.isInDoNotDisturbPeriod(prefs) {
				shouldPush = false
				log.Debug().
					Int64("user_id", payload.UserID).
					Msg("user is in do-not-disturb period, notification will be created but not pushed")
			}
		}
	}

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
	if shouldPush {
		if err := processor.tryWebSocketPush(ctx, payload.UserID, notification); err != nil {
			log.Error().Err(err).Int64("notification_id", notification.ID).Msg("WebSocket push failed (non-critical)")
			// 推送失败不影响主流程，通知已经存入数据库
		}
	}

	return nil
}

func (processor *RedisTaskProcessor) isNotificationEnabled(prefs db.UserNotificationPreference, notifType string) bool {
	switch notifType {
	case "order":
		return prefs.EnableOrderNotifications
	case "payment":
		return prefs.EnablePaymentNotifications
	case "delivery":
		return prefs.EnableDeliveryNotifications
	case "system":
		return prefs.EnableSystemNotifications
	case "food_safety":
		return prefs.EnableFoodSafetyNotifications
	default:
		return true
	}
}

func (processor *RedisTaskProcessor) isInDoNotDisturbPeriod(prefs db.UserNotificationPreference) bool {
	if !prefs.DoNotDisturbStart.Valid || !prefs.DoNotDisturbEnd.Valid {
		return false
	}

	now := time.Now()
	currentMicroseconds := int64(now.Hour()*3600+now.Minute()*60+now.Second()) * 1000000

	startMicroseconds := prefs.DoNotDisturbStart.Microseconds
	endMicroseconds := prefs.DoNotDisturbEnd.Microseconds

	if startMicroseconds > endMicroseconds {
		return currentMicroseconds >= startMicroseconds || currentMicroseconds < endMicroseconds
	}

	return currentMicroseconds >= startMicroseconds && currentMicroseconds < endMicroseconds
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

	wsMessage := websocket.Message{
		Type:      "notification",
		Data:      json.RawMessage(notificationData),
		Timestamp: time.Now(),
	}

	pushed := false

	// 检查是否是骑手
	for _, role := range roles {
		if role.Role == "rider" && role.RelatedEntityID.Valid {
			riderID := role.RelatedEntityID.Int64

			// 通过Redis Pub/Sub发布推送请求
			pushMsg := websocket.NotificationPushMessage{
				EntityType: "rider",
				EntityID:   riderID,
				Message:    wsMessage,
			}
			payload, _ := json.Marshal(pushMsg)
			channel := fmt.Sprintf("notification:rider:%d", riderID)
			processor.publishWSMessage(ctx, channel, payload)
			pushed = true
			log.Debug().Int64("rider_id", riderID).Msg("published notification push request to Redis")
			break
		}
	}

	// 检查是否是商户
	for _, role := range roles {
		if role.Role == "merchant" && role.RelatedEntityID.Valid {
			merchantID := role.RelatedEntityID.Int64

			// 通过Redis Pub/Sub发布推送请求
			pushMsg := websocket.NotificationPushMessage{
				EntityType: "merchant",
				EntityID:   merchantID,
				Message:    wsMessage,
			}
			payload, _ := json.Marshal(pushMsg)
			channel := fmt.Sprintf("notification:merchant:%d", merchantID)
			processor.publishWSMessage(ctx, channel, payload)
			pushed = true
			log.Debug().Int64("merchant_id", merchantID).Msg("published notification push request to Redis")
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
