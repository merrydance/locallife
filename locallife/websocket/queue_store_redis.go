package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisQueueStore stores queued messages in Redis lists.
type RedisQueueStore struct {
	client       *redis.Client
	ttl          time.Duration
	maxPerClient int64
}

// NewRedisQueueStore creates a Redis-backed queue store.
func NewRedisQueueStore(client *redis.Client, ttl time.Duration, maxPerClient int64) *RedisQueueStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if maxPerClient <= 0 {
		maxPerClient = 200
	}
	return &RedisQueueStore{
		client:       client,
		ttl:          ttl,
		maxPerClient: maxPerClient,
	}
}

func (s *RedisQueueStore) Enqueue(ctx context.Context, client ClientInfo, msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	key := s.key(client)
	pipe := s.client.Pipeline()
	pipe.RPush(ctx, key, payload)
	pipe.LTrim(ctx, key, -s.maxPerClient, -1)
	pipe.Expire(ctx, key, s.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisQueueStore) Dequeue(ctx context.Context, client ClientInfo, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 1
	}

	key := s.key(client)
	result := make([]Message, 0, limit)
	for i := 0; i < limit; i++ {
		item, err := s.client.LPop(ctx, key).Bytes()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return result, err
		}
		var msg Message
		if err := json.Unmarshal(item, &msg); err != nil {
			continue
		}
		result = append(result, msg)
	}

	return result, nil
}

func (s *RedisQueueStore) key(client ClientInfo) string {
	return fmt.Sprintf("ws:queue:%s:%d", client.ClientType, client.EntityID)
}
