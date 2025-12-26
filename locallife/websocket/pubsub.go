package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

const (
	// Redis频道前缀
	channelPrefixRider    = "notification:rider:"    // notification:rider:{rider_id}
	channelPrefixMerchant = "notification:merchant:" // notification:merchant:{merchant_id}
	channelPlatformAlerts = "notification:platform:alerts" // 平台告警频道
)

// PubSubManager 管理Redis Pub/Sub，用于跨进程通知推送
type PubSubManager struct {
	redisClient *redis.Client
	hub         *Hub
	ctx         context.Context
	cancel      context.CancelFunc
}

// NotificationPushMessage WebSocket推送消息（通过Redis传输）
type NotificationPushMessage struct {
	EntityType string  `json:"entity_type"` // rider/merchant
	EntityID   int64   `json:"entity_id"`
	Message    Message `json:"message"`
}

// NewPubSubManager 创建PubSub管理器
func NewPubSubManager(redisAddr string, redisPassword string, hub *Hub) (*PubSubManager, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: redisPassword,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &PubSubManager{
		redisClient: client,
		hub:         hub,
		ctx:         ctx,
		cancel:      cancel,
	}

	return manager, nil
}

// Start 启动订阅（监听所有骑手、商户和平台告警的通知频道）
func (m *PubSubManager) Start() {
	// 订阅模式：notification:rider:* 和 notification:merchant:* 和 notification:platform:alerts
	pubsub := m.redisClient.PSubscribe(m.ctx, channelPrefixRider+"*", channelPrefixMerchant+"*", channelPlatformAlerts)

	go func() {
		defer pubsub.Close()

		log.Info().Msg("WebSocket PubSub started, listening for notification push requests")

		for {
			select {
			case <-m.ctx.Done():
				log.Info().Msg("WebSocket PubSub stopped")
				return
			default:
				msg, err := pubsub.ReceiveMessage(m.ctx)
				if err != nil {
					if m.ctx.Err() != nil {
						return
					}
					log.Error().Err(err).Msg("receive pubsub message failed")
					time.Sleep(time.Second)
					continue
				}

				m.handlePubSubMessage(msg.Channel, msg.Payload)
			}
		}
	}()
}

// Stop 停止订阅
func (m *PubSubManager) Stop() {
	m.cancel()
	m.redisClient.Close()
}

// handlePubSubMessage 处理接收到的消息
func (m *PubSubManager) handlePubSubMessage(channel string, payload string) {
	// 平台告警消息直接广播给所有平台运营人员
	if channel == channelPlatformAlerts {
		m.handleAlertMessage(payload)
		return
	}

	var pushMsg NotificationPushMessage
	if err := json.Unmarshal([]byte(payload), &pushMsg); err != nil {
		log.Error().Err(err).Str("payload", payload).Msg("unmarshal pubsub message failed")
		return
	}

	// 根据类型推送
	switch pushMsg.EntityType {
	case "rider":
		if m.hub.IsRiderOnline(pushMsg.EntityID) {
			m.hub.SendToRider(pushMsg.EntityID, pushMsg.Message)
			log.Debug().
				Int64("rider_id", pushMsg.EntityID).
				Str("type", pushMsg.Message.Type).
				Msg("pushed notification to rider via WebSocket")
		} else {
			log.Debug().
				Int64("rider_id", pushMsg.EntityID).
				Msg("rider offline, skip WebSocket push")
		}

	case "merchant":
		if m.hub.IsMerchantOnline(pushMsg.EntityID) {
			m.hub.SendToMerchant(pushMsg.EntityID, pushMsg.Message)
			log.Debug().
				Int64("merchant_id", pushMsg.EntityID).
				Str("type", pushMsg.Message.Type).
				Msg("pushed notification to merchant via WebSocket")
		} else {
			log.Debug().
				Int64("merchant_id", pushMsg.EntityID).
				Msg("merchant offline, skip WebSocket push")
		}

	default:
		log.Warn().Str("entity_type", pushMsg.EntityType).Msg("unknown entity type in pubsub message")
	}
}

// handleAlertMessage 处理告警消息并广播给平台运营人员
func (m *PubSubManager) handleAlertMessage(payload string) {
	var wsMessage struct {
		Type      string          `json:"type"`
		Data      json.RawMessage `json:"data"`
		Timestamp time.Time       `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(payload), &wsMessage); err != nil {
		log.Error().Err(err).Str("payload", payload).Msg("unmarshal alert message failed")
		return
	}

	msg := Message{
		Type:      wsMessage.Type,
		Data:      wsMessage.Data,
		Timestamp: wsMessage.Timestamp,
	}

	// 广播给所有在线的平台运营人员
	m.hub.Broadcast(BroadcastMessage{
		ClientType: ClientTypePlatform,
		EntityID:   0,
		Message:    msg,
	})

	log.Info().
		Str("type", wsMessage.Type).
		Int("platform_clients", m.hub.GetOnlinePlatformCount()).
		Msg("alert broadcasted to platform operators")
}

// PublishNotificationPush 发布通知推送请求（由worker调用）
func PublishNotificationPush(ctx context.Context, redisClient *redis.Client, entityType string, entityID int64, message Message) error {
	pushMsg := NotificationPushMessage{
		EntityType: entityType,
		EntityID:   entityID,
		Message:    message,
	}

	payload, err := json.Marshal(pushMsg)
	if err != nil {
		return err
	}

	var channel string
	switch entityType {
	case "rider":
		channel = fmt.Sprintf("%s%d", channelPrefixRider, entityID)
	case "merchant":
		channel = fmt.Sprintf("%s%d", channelPrefixMerchant, entityID)
	default:
		return nil
	}

	return redisClient.Publish(ctx, channel, payload).Err()
}
