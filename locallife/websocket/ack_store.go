package websocket

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// Ack represents a client acknowledgement payload.
type Ack struct {
	MessageID string
	Sequence  uint64
	Timestamp time.Time
}

// AckStore defines an interface for storing and checking message acknowledgements.
type AckStore interface {
	RecordAck(ctx context.Context, client ClientInfo, ack Ack)
	HasAck(ctx context.Context, client ClientInfo, messageID string) bool
}

// MemoryAckStore is an in-memory ack store with TTL-based cleanup.
type MemoryAckStore struct {
	mu      sync.RWMutex
	entries map[string]time.Time
	ttl     time.Duration
	now     func() time.Time
}

// NewMemoryAckStore creates an in-memory ack store.
func NewMemoryAckStore(ttl time.Duration, now func() time.Time) *MemoryAckStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &MemoryAckStore{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		now:     now,
	}
}

// RecordAck stores an acknowledgement with TTL.
func (s *MemoryAckStore) RecordAck(ctx context.Context, client ClientInfo, ack Ack) {
	if ack.MessageID == "" {
		return
	}

	now := s.now()
	expiresAt := now.Add(s.ttl)

	s.mu.Lock()
	s.entries[s.key(client, ack.MessageID)] = expiresAt
	s.pruneLocked(now)
	s.mu.Unlock()
}

// HasAck returns whether the ack exists and is still valid.
func (s *MemoryAckStore) HasAck(ctx context.Context, client ClientInfo, messageID string) bool {
	if messageID == "" {
		return false
	}

	now := s.now()
	key := s.key(client, messageID)

	s.mu.RLock()
	expiresAt, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return false
	}

	if now.After(expiresAt) {
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return false
	}

	return true
}

func (s *MemoryAckStore) key(client ClientInfo, messageID string) string {
	return string(client.ClientType) + ":" + strconv.FormatInt(client.EntityID, 10) + ":" + messageID
}

func (s *MemoryAckStore) pruneLocked(now time.Time) {
	for key, expiresAt := range s.entries {
		if now.After(expiresAt) {
			delete(s.entries, key)
		}
	}
}

// strconv handles int64 conversion efficiently; keep helper simple.
