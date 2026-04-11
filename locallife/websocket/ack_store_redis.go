package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisAckStore 用带 TTL 的 Redis key 存储客户端确认记录。
//
// 数据结构：
//
//	Key : ws:ack:{clientType}:{entityID}:{messageID}  (String with TTL)
//	Value: "1"
//
// 相比 Hash，每条 ack 独立持有 TTL，自动过期更精确，无需后台清理。
type RedisAckStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisAckStore 创建 Redis ACK 存储。
func NewRedisAckStore(client *redis.Client, ttl time.Duration) *RedisAckStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &RedisAckStore{
		client: client,
		ttl:    ttl,
	}
}

// RecordAck 记录客户端对某条消息的确认。
func (s *RedisAckStore) RecordAck(ctx context.Context, client ClientInfo, ack Ack) {
	if ack.MessageID == "" {
		return
	}
	s.client.Set(ctx, s.key(client, ack.MessageID), "1", s.ttl)
}

// HasAck 检查该消息是否已被客户端确认（且记录未过期）。
func (s *RedisAckStore) HasAck(ctx context.Context, client ClientInfo, messageID string) bool {
	if messageID == "" {
		return false
	}
	n, err := s.client.Exists(ctx, s.key(client, messageID)).Result()
	return err == nil && n > 0
}

func (s *RedisAckStore) key(client ClientInfo, messageID string) string {
	return fmt.Sprintf("ws:ack:%s:%d:%s", client.ClientType, client.EntityID, messageID)
}
