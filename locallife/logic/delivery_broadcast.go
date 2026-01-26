package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

// DeliveryBroadcastLogic 处理配送相关的实时广播逻辑
type DeliveryBroadcastLogic struct {
	store       db.Store
	redisClient *redis.Client
}

// NewDeliveryBroadcastLogic 创建配送广播逻辑实例
func NewDeliveryBroadcastLogic(store db.Store, redisClient *redis.Client) *DeliveryBroadcastLogic {
	return &DeliveryBroadcastLogic{
		store:       store,
		redisClient: redisClient,
	}
}

// BroadcastOrderGone 通知区域内的所有骑手，某个订单已从配送池中消失（被抢走或取消）
func (l *DeliveryBroadcastLogic) BroadcastOrderGone(ctx context.Context, regionID int64, orderID int64) error {
	// 1. 获取该区域内的所有在线骑手
	riders, err := l.store.ListOnlineRidersByRegion(ctx, pgtype.Int8{Int64: regionID, Valid: true})
	if err != nil {
		log.Error().Err(err).Int64("region_id", regionID).Msg("failed to list online riders for broadcast")
		return err
	}

	if len(riders) == 0 {
		return nil
	}

	// 2. 构造消息
	msgData, _ := json.Marshal(map[string]any{
		"order_id":  orderID,
		"event":     "gone",
		"timestamp": time.Now(),
	})

	wsMsg := websocket.Message{
		Type:      websocket.MessageTypeDeliveryPoolGone,
		Data:      json.RawMessage(msgData),
		Timestamp: time.Now(),
	}

	// 3. 通过 Redis Pub/Sub 推送给每个骑手
	return l.pushToRiders(ctx, riders, wsMsg)
}

// BroadcastNewOrderNotification 通知区域内的骑手有新订单加入（差量更新）
func (l *DeliveryBroadcastLogic) BroadcastNewOrderNotification(ctx context.Context, regionID int64, poolItem db.DeliveryPool, merchantName string) error {
	riders, err := l.store.ListOnlineRidersByRegion(ctx, pgtype.Int8{Int64: regionID, Valid: true})
	if err != nil {
		log.Error().Err(err).Int64("region_id", regionID).Msg("failed to list online riders for new order broadcast")
		return err
	}

	if len(riders) == 0 {
		return nil
	}

	lng, _ := poolItem.PickupLongitude.Float64Value()
	lat, _ := poolItem.PickupLatitude.Float64Value()

	msgData, _ := json.Marshal(map[string]any{
		"order_id":         poolItem.OrderID,
		"merchant_name":    merchantName,
		"pickup_longitude": lng.Float64,
		"pickup_latitude":  lat.Float64,
		"delivery_fee":     poolItem.DeliveryFee,
		"distance":         poolItem.Distance,
		"event":            "new",
		"timestamp":        time.Now(),
	})

	wsMsg := websocket.Message{
		Type:      websocket.MessageTypeDeliveryPoolNew,
		Data:      json.RawMessage(msgData),
		Timestamp: time.Now(),
	}

	return l.pushToRiders(ctx, riders, wsMsg)
}

// pushToRiders 内部辅助函数：批量推送消息到骑手频道
func (l *DeliveryBroadcastLogic) pushToRiders(ctx context.Context, riders []db.Rider, msg websocket.Message) error {
	for _, rider := range riders {
		pushMsg := websocket.NotificationPushMessage{
			EntityType: websocket.EntityRider,
			EntityID:   rider.ID,
			Message:    msg,
		}

		payload, _ := json.Marshal(pushMsg)
		channel := fmt.Sprintf("notification:rider:%d", rider.ID)

		if err := l.redisClient.Publish(ctx, channel, payload).Err(); err != nil {
			log.Warn().Err(err).Int64("rider_id", rider.ID).Msg("failed to publish ws message to redis channel")
		}
	}
	return nil
}
