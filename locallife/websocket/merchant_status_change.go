package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// MerchantStatusChangePublisher 发布商户营业状态变更事件。
type MerchantStatusChangePublisher interface {
	PublishMerchantStatusChange(ctx context.Context, merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) error
}

// RedisMerchantStatusChangePublisher 通过 Redis Pub/Sub 发布商户营业状态变更事件。
type RedisMerchantStatusChangePublisher struct {
	publisher PubSubPublisher
}

func NewRedisMerchantStatusChangePublisher(publisher PubSubPublisher) *RedisMerchantStatusChangePublisher {
	return &RedisMerchantStatusChangePublisher{publisher: publisher}
}

func (p *RedisMerchantStatusChangePublisher) PublishMerchantStatusChange(ctx context.Context, merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) error {
	if p == nil {
		return nil
	}
	return PublishMerchantStatusChange(ctx, p.publisher, merchantID, isOpen, autoCloseAt, source)
}

// LocalMerchantStatusChangePublisher 直接向本地 Hub 发送商户营业状态变更事件。
type LocalMerchantStatusChangePublisher struct {
	Hub *Hub
}

func NewMerchantStatusChangeLocalPublisher(hub *Hub) *LocalMerchantStatusChangePublisher {
	return &LocalMerchantStatusChangePublisher{Hub: hub}
}

func (p *LocalMerchantStatusChangePublisher) PublishMerchantStatusChange(_ context.Context, merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) error {
	if p == nil || p.Hub == nil {
		return nil
	}

	msg, err := NewMerchantStatusChangeMessage(merchantID, isOpen, autoCloseAt, source)
	if err != nil {
		return err
	}

	p.Hub.SendToMerchant(merchantID, msg)
	return nil
}

// MerchantStatusChangeData 是商户营业状态变更事件的推送载荷。
type MerchantStatusChangeData struct {
	MerchantID  int64      `json:"merchant_id"`
	IsOpen      bool       `json:"is_open"`
	AutoCloseAt *time.Time `json:"auto_close_at,omitempty"`
	Source      string     `json:"source"`
}

// NewMerchantStatusChangeMessage 构造商户营业状态变更 WebSocket 消息。
func NewMerchantStatusChangeMessage(merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) (Message, error) {
	if source == "" {
		source = "unknown"
	}

	data, err := json.Marshal(MerchantStatusChangeData{
		MerchantID:  merchantID,
		IsOpen:      isOpen,
		AutoCloseAt: autoCloseAt,
		Source:      source,
	})
	if err != nil {
		return Message{}, fmt.Errorf("marshal merchant status change payload: %w", err)
	}

	return Message{
		Type:      MessageTypeMerchantStatusChange,
		Data:      json.RawMessage(data),
		Timestamp: time.Now(),
	}, nil
}

// PublishMerchantStatusChange 通过 Redis Pub/Sub 发布商户营业状态变更事件。
func PublishMerchantStatusChange(ctx context.Context, publisher PubSubPublisher, merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) error {
	if publisher == nil {
		return nil
	}

	msg, err := NewMerchantStatusChangeMessage(merchantID, isOpen, autoCloseAt, source)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(NotificationPushMessage{
		EntityType: EntityMerchant,
		EntityID:   merchantID,
		Message:    msg,
	})
	if err != nil {
		return err
	}

	channel := fmt.Sprintf("%s%d", channelPrefixMerchant, merchantID)
	return publisher.Publish(ctx, channel, payload)
}
