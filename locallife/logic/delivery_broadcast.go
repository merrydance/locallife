package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

const (
	broadcastStartDistanceMeters = 100.0
	broadcastStepDistanceMeters  = 100.0
	broadcastMaxDistanceMeters   = 5000.0
	broadcastMinRiderCount       = 3
	broadcastRiderLimitCount     = 1000
)

// DeliveryBroadcastLogic 处理代取相关的实时广播逻辑
type DeliveryBroadcastLogic struct {
	store       db.Store
	redisClient *redis.Client
}

// NewDeliveryBroadcastLogic 创建代取广播逻辑实例
func NewDeliveryBroadcastLogic(store db.Store, redisClient *redis.Client) *DeliveryBroadcastLogic {
	return &DeliveryBroadcastLogic{
		store:       store,
		redisClient: redisClient,
	}
}

// BroadcastOrderGone 通知取餐点附近的在线骑手，某个订单已从代取池中消失（被抢走或取消）
func (l *DeliveryBroadcastLogic) BroadcastOrderGone(ctx context.Context, orderID int64, pickupLat float64, pickupLng float64) error {
	riders, err := l.listNearbyBroadcastRiders(ctx, pickupLat, pickupLng)
	if err != nil {
		return err
	}

	if len(riders) == 0 {
		return nil
	}

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

	return l.pushToRiders(ctx, riders, wsMsg)
}

// BroadcastNewOrderNotification 通知取餐点附近的骑手有新订单加入（差量更新）
func (l *DeliveryBroadcastLogic) BroadcastNewOrderNotification(ctx context.Context, poolItem db.DeliveryPool, merchantName string) error {
	if !poolItem.PickupLongitude.Valid || !poolItem.PickupLatitude.Valid {
		log.Warn().Int64("order_id", poolItem.OrderID).Msg("skip new order broadcast without pickup coordinates")
		return nil
	}

	lng, err := poolItem.PickupLongitude.Float64Value()
	if err != nil {
		return err
	}
	lat, err := poolItem.PickupLatitude.Float64Value()
	if err != nil {
		return err
	}

	riders, err := l.listNearbyBroadcastRiders(ctx, lat.Float64, lng.Float64)
	if err != nil {
		return err
	}

	if len(riders) == 0 {
		return nil
	}

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

func (l *DeliveryBroadcastLogic) listNearbyBroadcastRiders(ctx context.Context, centerLat float64, centerLng float64) ([]db.Rider, error) {
	riders := make([]db.Rider, 0, broadcastMinRiderCount)
	seen := make(map[int64]struct{}, broadcastMinRiderCount)
	var lastErr error

	for distance := broadcastStartDistanceMeters; distance <= broadcastMaxDistanceMeters; distance += broadcastStepDistanceMeters {
		nearbyRiders, err := l.store.ListNearbyRiders(ctx, db.ListNearbyRidersParams{
			CenterLat:   centerLat,
			CenterLng:   centerLng,
			MaxDistance: distance,
			LimitCount:  broadcastRiderLimitCount,
		})
		if err != nil {
			lastErr = err
			continue
		}

		for _, rider := range nearbyRiders {
			if _, ok := seen[rider.ID]; ok {
				continue
			}
			seen[rider.ID] = struct{}{}
			riders = append(riders, db.Rider{
				ID:                rider.ID,
				UserID:            rider.UserID,
				RealName:          rider.RealName,
				IDCardNo:          rider.IDCardNo,
				Phone:             rider.Phone,
				DepositAmount:     rider.DepositAmount,
				FrozenDeposit:     rider.FrozenDeposit,
				Status:            rider.Status,
				IsOnline:          rider.IsOnline,
				CreditScore:       rider.CreditScore,
				CurrentLongitude:  rider.CurrentLongitude,
				CurrentLatitude:   rider.CurrentLatitude,
				LocationUpdatedAt: rider.LocationUpdatedAt,
				TotalOrders:       rider.TotalOrders,
				TotalEarnings:     rider.TotalEarnings,
				OnlineDuration:    rider.OnlineDuration,
				CreatedAt:         rider.CreatedAt,
				UpdatedAt:         rider.UpdatedAt,
				RegionID:          rider.RegionID,
				ApplicationID:     rider.ApplicationID,
			})
		}

		if len(riders) >= broadcastMinRiderCount {
			break
		}
	}

	if len(riders) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return riders, nil
}

// pushToRiders 内部辅助函数：批量推送消息到骑手频道
func (l *DeliveryBroadcastLogic) pushToRiders(ctx context.Context, riders []db.Rider, msg websocket.Message) error {
	if l.redisClient == nil {
		return nil
	}

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
