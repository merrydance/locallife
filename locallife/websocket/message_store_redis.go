package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisMessageStore 用 Redis Sorted Set 存储需要回放的消息。
//
// 数据结构：
//
//	Key  : ws:replay:{clientType}:{entityID}          (Sorted Set)
//	Score: message.Sequence                           (uint64 → float64)
//	Member: JSON-encoded Message
//
// Sorted Set 天然按分数有序，使得"从断点序号 N 起回放"可以用 ZRANGEBYSCORE 高效完成。
// 每次写入后：
//   - ZREMRANGEBYRANK 删除最旧条目，保持每个客户端最多 maxPerClient 条
//   - EXPIRE 刷新 TTL
type RedisMessageStore struct {
	client       *redis.Client
	ttl          time.Duration
	maxPerClient int64
}

// NewRedisMessageStore 创建 Redis 消息回放存储。
func NewRedisMessageStore(client *redis.Client, ttl time.Duration, maxPerClient int64) *RedisMessageStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if maxPerClient <= 0 {
		maxPerClient = 200
	}
	return &RedisMessageStore{
		client:       client,
		ttl:          ttl,
		maxPerClient: maxPerClient,
	}
}

// Save 存储消息以供断线重连时回放。
// 消息必须同时拥有 ID 和 Sequence 才会被存储。
func (s *RedisMessageStore) Save(ctx context.Context, client ClientInfo, msg Message) {
	if msg.ID == "" || msg.Sequence == 0 {
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	key := s.key(client)
	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  float64(msg.Sequence),
		Member: payload,
	})
	// 保留最新 maxPerClient 条，删除最旧的多余条目
	pipe.ZRemRangeByRank(ctx, key, 0, -s.maxPerClient-1)
	pipe.Expire(ctx, key, s.ttl)
	_, _ = pipe.Exec(ctx)
}

// Replay 返回 sequence > afterSequence 的消息，最多返回 limit 条。
func (s *RedisMessageStore) Replay(ctx context.Context, client ClientInfo, afterSequence uint64, limit int) []Message {
	if limit <= 0 {
		limit = int(s.maxPerClient)
	}

	key := s.key(client)
	// Sorted Set range: (afterSequence, +inf)，括号表示不含端点
	minScore := fmt.Sprintf("(%d", afterSequence)
	items, err := s.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    minScore,
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil
	}

	result := make([]Message, 0, len(items))
	for _, item := range items {
		var msg Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			continue
		}
		result = append(result, msg)
	}
	return result
}

func (s *RedisMessageStore) key(client ClientInfo) string {
	return fmt.Sprintf("ws:replay:%s:%d", client.ClientType, client.EntityID)
}
