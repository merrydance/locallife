package websocket

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MessageStore stores recent messages for replay.
type MessageStore interface {
	Save(ctx context.Context, client ClientInfo, msg Message)
	Replay(ctx context.Context, client ClientInfo, afterSequence uint64, limit int) []Message
}

type storedMessage struct {
	msg       Message
	expiresAt time.Time
}

// MemoryMessageStore keeps recent messages in memory with TTL and per-client size limit.
type MemoryMessageStore struct {
	mu           sync.RWMutex
	entries      map[string][]storedMessage
	ttl          time.Duration
	maxPerClient int
	now          func() time.Time
}

// NewMemoryMessageStore creates a new MemoryMessageStore.
func NewMemoryMessageStore(ttl time.Duration, maxPerClient int, now func() time.Time) *MemoryMessageStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if maxPerClient <= 0 {
		maxPerClient = 200
	}
	if now == nil {
		now = time.Now
	}
	return &MemoryMessageStore{
		entries:      make(map[string][]storedMessage),
		ttl:          ttl,
		maxPerClient: maxPerClient,
		now:          now,
	}
}

// Save stores the message for later replay.
func (s *MemoryMessageStore) Save(ctx context.Context, client ClientInfo, msg Message) {
	if msg.ID == "" || msg.Sequence == 0 {
		return
	}

	now := s.now()
	expiresAt := now.Add(s.ttl)
	key := s.key(client)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = append(s.entries[key], storedMessage{msg: msg, expiresAt: expiresAt})
	s.pruneLocked(key, now)
}

// Replay returns messages after the given sequence, up to the limit.
func (s *MemoryMessageStore) Replay(ctx context.Context, client ClientInfo, afterSequence uint64, limit int) []Message {
	if limit <= 0 {
		limit = s.maxPerClient
	}

	now := s.now()
	key := s.key(client)

	s.mu.Lock()
	s.pruneLocked(key, now)
	stored := append([]storedMessage(nil), s.entries[key]...)
	s.mu.Unlock()

	result := make([]Message, 0, len(stored))
	for _, item := range stored {
		if item.msg.Sequence <= afterSequence {
			continue
		}
		result = append(result, item.msg)
		if len(result) >= limit {
			break
		}
	}

	return result
}

func (s *MemoryMessageStore) key(client ClientInfo) string {
	return string(client.ClientType) + ":" + strconv.FormatInt(client.EntityID, 10)
}

func (s *MemoryMessageStore) pruneLocked(key string, now time.Time) {
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
