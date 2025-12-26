package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

// NotificationHelper 通知发送辅助函数集合
type NotificationHelper struct {
	store db.Store
	wsHub *websocket.Hub
}

// SendNotificationParams 发送通知的参数
type SendNotificationParams struct {
	UserID      int64
	Type        string // order/payment/delivery/system/food_safety
	Title       string
	Content     string
	RelatedType string // order/payment/delivery/merchant/rider
	RelatedID   int64
	ExtraData   map[string]any
	ExpiresAt   *time.Time

	// WebSocket推送相关（可选）
	PushToRider    bool  // 是否推送给骑手
	PushToMerchant bool  // 是否推送给商户
	RiderID        int64 // 骑手ID（当PushToRider=true时必填）
	MerchantID     int64 // 商户ID（当PushToMerchant=true时必填）

	// 是否忽略用户偏好设置（用于关键通知，如支付成功、配送完成等）
	IgnorePreferences bool
}

// isNotificationEnabled 检查用户是否启用了该类型的通知
func (server *Server) isNotificationEnabled(prefs db.UserNotificationPreference, notifType string) bool {
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
		return true // 未知类型默认启用
	}
}

// isInDoNotDisturbPeriod 检查当前时间是否在免打扰时段内
func (server *Server) isInDoNotDisturbPeriod(prefs db.UserNotificationPreference) bool {
	// 如果免打扰时段未设置，则不在免打扰时段
	if !prefs.DoNotDisturbStart.Valid || !prefs.DoNotDisturbEnd.Valid {
		return false
	}

	now := time.Now()
	// 将当前时间转为微秒数（从0点开始）
	currentMicroseconds := int64(now.Hour()*3600+now.Minute()*60+now.Second()) * 1000000

	startMicroseconds := prefs.DoNotDisturbStart.Microseconds
	endMicroseconds := prefs.DoNotDisturbEnd.Microseconds

	// 处理跨午夜的情况（如 22:00 - 08:00）
	if startMicroseconds > endMicroseconds {
		// 跨午夜：当前时间在开始之后或结束之前
		return currentMicroseconds >= startMicroseconds || currentMicroseconds < endMicroseconds
	}

	// 正常情况（如 23:00 - 07:00 实际是 23:00 - 次日 07:00，但如果是 01:00 - 06:00）
	return currentMicroseconds >= startMicroseconds && currentMicroseconds < endMicroseconds
}

// SendNotification 创建通知并根据需要通过WebSocket推送
func (server *Server) SendNotification(ctx context.Context, params SendNotificationParams) error {
	// 测试环境跳过WebSocket推送，但仍创建通知记录（如果store不为nil）
	skipWebSocket := server.wsHub == nil

	// 检查用户通知偏好设置（除非指定忽略）
	var shouldPush bool = true
	if !params.IgnorePreferences {
		prefs, err := server.store.GetUserNotificationPreferences(ctx, params.UserID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Int64("user_id", params.UserID).Msg("failed to get notification preferences")
			// 获取偏好失败时不阻止通知创建，但记录日志
		} else if err == nil {
			// 检查该类型通知是否启用
			if !server.isNotificationEnabled(prefs, params.Type) {
				log.Debug().
					Int64("user_id", params.UserID).
					Str("type", params.Type).
					Msg("notification disabled by user preference")
				return nil // 用户禁用了该类型通知，不创建
			}

			// 检查是否在免打扰时段（只影响推送，不影响通知创建）
			if server.isInDoNotDisturbPeriod(prefs) {
				shouldPush = false
				log.Debug().
					Int64("user_id", params.UserID).
					Msg("user is in do-not-disturb period, notification will be created but not pushed")
			}
		}
	}

	// 构建extra_data
	var extraDataJSON []byte
	if params.ExtraData != nil {
		var err error
		extraDataJSON, err = json.Marshal(params.ExtraData)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal extra_data")
			return err
		}
	}

	// 创建通知记录
	createParams := db.CreateNotificationParams{
		UserID:  params.UserID,
		Type:    params.Type,
		Title:   params.Title,
		Content: params.Content,
	}

	if params.RelatedType != "" {
		createParams.RelatedType = pgtype.Text{String: params.RelatedType, Valid: true}
	}

	if params.RelatedID > 0 {
		createParams.RelatedID = pgtype.Int8{Int64: params.RelatedID, Valid: true}
	}

	if len(extraDataJSON) > 0 {
		createParams.ExtraData = extraDataJSON
	}

	if params.ExpiresAt != nil {
		createParams.ExpiresAt = pgtype.Timestamptz{Time: *params.ExpiresAt, Valid: true}
	}

	notification, err := server.store.CreateNotification(ctx, createParams)
	if err != nil {
		log.Error().Err(err).Msg("failed to create notification")
		return err
	}

	// 如果跳过WebSocket或不应推送（免打扰时段），直接返回
	if skipWebSocket || !shouldPush {
		return nil
	}

	// 通过WebSocket推送（如果骑手或商户在线）
	pushed := false

	if params.PushToRider && params.RiderID > 0 {
		if server.wsHub.IsRiderOnline(params.RiderID) {
			msgData, _ := json.Marshal(newNotificationResponse(notification))
			server.wsHub.SendToRider(params.RiderID, websocket.Message{
				Type:      "notification",
				Data:      msgData,
				Timestamp: time.Now(),
			})
			pushed = true
		}
	}

	if params.PushToMerchant && params.MerchantID > 0 {
		if server.wsHub.IsMerchantOnline(params.MerchantID) {
			msgData, _ := json.Marshal(newNotificationResponse(notification))
			server.wsHub.SendToMerchant(params.MerchantID, websocket.Message{
				Type:      "notification",
				Data:      msgData,
				Timestamp: time.Now(),
			})
			pushed = true
		}
	}

	// 标记已推送
	if pushed {
		_ = server.store.MarkNotificationAsPushed(ctx, notification.ID)
	}

	return nil
}
