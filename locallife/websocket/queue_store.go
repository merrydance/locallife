package websocket

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MessageQueue defines an interface for queueing outbound messages.
type MessageQueue interface {
	Enqueue(ctx context.Context, client ClientInfo, msg Message) error
	Dequeue(ctx context.Context, client ClientInfo, limit int) ([]Message, error)
}

// QueueBacklogProvider provides total backlog size when supported.
type QueueBacklogProvider interface {
	TotalSize(ctx context.Context) int
}

type queueItem struct {
	msg       Message
	expiresAt time.Time
}

// MemoryQueueStore buffers messages in memory with TTL and size limits.
type MemoryQueueStore struct {
	mu           sync.RWMutex
	entries      map[string][]queueItem
	ttl          time.Duration
	maxPerClient int
	now          func() time.Time
}

// NewMemoryQueueStore creates a memory queue store.
func NewMemoryQueueStore(ttl time.Duration, maxPerClient int, now func() time.Time) *MemoryQueueStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if maxPerClient <= 0 {
		maxPerClient = 200
	}
	if now == nil {
		now = time.Now
	}
	return &MemoryQueueStore{
		entries:      make(map[string][]queueItem),
		ttl:          ttl,
		maxPerClient: maxPerClient,
		now:          now,
	}
}

// Enqueue adds a message to the queue.
func (s *MemoryQueueStore) Enqueue(ctx context.Context, client ClientInfo, msg Message) error {
	key := s.key(client)
	now := s.now()
	expiresAt := now.Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = append(s.entries[key], queueItem{msg: msg, expiresAt: expiresAt})
	s.pruneLocked(key, now)
	return nil
}

// Dequeue removes up to limit messages from the queue.
func (s *MemoryQueueStore) Dequeue(ctx context.Context, client ClientInfo, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = s.maxPerClient
	}

	key := s.key(client)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(key, now)
	items := s.entries[key]
	if len(items) == 0 {
		return nil, nil
	}

	if limit > len(items) {
		limit = len(items)
	}

	result := make([]Message, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, items[i].msg)
	}

	s.entries[key] = items[limit:]
	return result, nil
}

// TotalSize returns the total queued message count.
func (s *MemoryQueueStore) TotalSize(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, items := range s.entries {
		count += len(items)
	}
	return count
}

func (s *MemoryQueueStore) key(client ClientInfo) string {
	return string(client.ClientType) + ":" + strconv.FormatInt(client.EntityID, 10)
}

func (s *MemoryQueueStore) pruneLocked(key string, now time.Time) {
	items := s.entries[key]
	if len(items) == 0 {
		return
	}

	filtered := items[:0]
	for _, item := range items {
		if now.Before(item.expiresAt) {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) > s.maxPerClient {
		filtered = filtered[len(filtered)-s.maxPerClient:]
	}

	s.entries[key] = filtered
}
