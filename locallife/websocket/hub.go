package websocket

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	gorilla_websocket "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Message 通过WebSocket发送的消息结构
type Message struct {
	ID        string          `json:"id,omitempty"`       // 消息唯一ID
	Sequence  uint64          `json:"sequence,omitempty"` // 客户端内序号
	Type      string          `json:"type"`               // 消息类型：notification/ping/pong
	Data      json.RawMessage `json:"data"`               // 消息数据
	Timestamp time.Time       `json:"timestamp"`          // 消息时间戳
}

// Client 客户端连接类型
type ClientType string

const (
	ClientTypeRider    ClientType = "rider"    // 骑手
	ClientTypeMerchant ClientType = "merchant" // 商户
	ClientTypePlatform ClientType = "platform" // 平台运营（数据大盘，接收告警）
)

// ClientInfo 客户端信息
type ClientInfo struct {
	UserID     int64      // 用户ID
	ClientType ClientType // 客户端类型
	EntityID   int64      // 实体ID（骑手ID或商户ID）
}

// Client 表示一个WebSocket客户端连接
type Client struct {
	info       ClientInfo
	hub        *Hub
	send       chan Message
	ctx        context.Context
	done       chan struct{}
	conn       *gorilla_websocket.Conn // gorilla websocket连接
	closeOnce  sync.Once               // 确保 send channel 只关闭一次
	doneOnce   sync.Once               // 确保 done channel 只关闭一次
	sendMu     sync.RWMutex            // protects sends racing with channel close
	sendClosed bool
	seq        uint64 // per-client sequence counter
}

type clientSendResult int

const (
	clientSendResultSent clientSendResult = iota
	clientSendResultFull
	clientSendResultClosed
)

func (c *Client) closeDone() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

func (c *Client) closeSend() {
	c.closeOnce.Do(func() {
		c.sendMu.Lock()
		defer c.sendMu.Unlock()
		c.sendClosed = true
		close(c.send)
	})
}

func (c *Client) trySend(message Message) clientSendResult {
	select {
	case <-c.done:
		return clientSendResultClosed
	default:
	}

	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	select {
	case <-c.done:
		return clientSendResultClosed
	default:
	}
	if c.sendClosed {
		return clientSendResultClosed
	}
	select {
	case c.send <- message:
		return clientSendResultSent
	default:
		return clientSendResultFull
	}
}

// Hub 管理所有WebSocket连接
type Hub struct {
	// 注册的客户端，按类型和实体ID索引
	riders    map[int64]*Client              // key: rider_id
	merchants map[int64]map[*Client]struct{} // key: merchant_id, value: active connections
	platforms map[int64]*Client              // key: user_id（平台运营人员）

	// 注册/注销通道
	register   chan *Client
	unregister chan *Client

	// 广播消息
	broadcast chan BroadcastMessage

	ackStore     AckStore
	messageStore MessageStore
	queueStore   MessageQueue
	idGenerator  func() string
	metrics      MetricsRecorder
	reliableGate func(ClientInfo) bool
	// replayFilter 是可选的断线重连消息回放过滤器。
	// 返回 false 表示该消息不应回放给此客户端（例如骑手场景中订单已被他人接走）。
	replayFilter ReplayFilter

	retryQueue  chan retryItem
	retryConfig RetryConfig
	retryCounts map[string]int
	retryMu     sync.Mutex
	sentTimes   map[string]time.Time

	queueConfig      QueueConfig
	alertConfig      AlertConfig
	alertDrops       int64
	alertRetries     int64
	alertDisconnects int64

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	ClientType ClientType // 目标客户端类型
	EntityID   int64      // 目标实体ID，0表示广播给所有该类型客户端
	Message    Message    // 消息内容
}

// ReplayFilter 是断线重连消息回放的可选过滤函数。
// 返回 true 表示该消息应当回放；返回 false 跳过此消息。
// 典型用途：骑手回放时过滤掉已被他人接走的代取池订单。
type ReplayFilter func(ctx context.Context, info ClientInfo, msg Message) bool

type RetryConfig struct {
	Timeout    time.Duration
	MaxRetries int
}

type QueueConfig struct {
	FlushInterval time.Duration
	FlushBatch    int
}

type AlertConfig struct {
	Interval            time.Duration
	DropThreshold       int64
	RetryThreshold      int64
	DisconnectThreshold int64
	QueueThreshold      int
}

// NewHub 创建新的Hub
type HubOption func(*Hub)

// WithAckStore injects an acknowledgement store.
func WithAckStore(store AckStore) HubOption {
	return func(h *Hub) {
		h.ackStore = store
	}
}

// WithIDGenerator injects a message ID generator.
func WithIDGenerator(fn func() string) HubOption {
	return func(h *Hub) {
		if fn != nil {
			h.idGenerator = fn
		}
	}
}

// WithMessageStore injects a message store for replay.
func WithMessageStore(store MessageStore) HubOption {
	return func(h *Hub) {
		h.messageStore = store
	}
}

// WithQueueStore injects a queue store for backpressure buffering.
func WithQueueStore(store MessageQueue) HubOption {
	return func(h *Hub) {
		h.queueStore = store
	}
}

// WithRetryConfig overrides retry behavior.
func WithRetryConfig(cfg RetryConfig) HubOption {
	return func(h *Hub) {
		if cfg.Timeout > 0 {
			h.retryConfig.Timeout = cfg.Timeout
		}
		if cfg.MaxRetries > 0 {
			h.retryConfig.MaxRetries = cfg.MaxRetries
		}
	}
}

// WithQueueConfig overrides queue flush behavior.
func WithQueueConfig(cfg QueueConfig) HubOption {
	return func(h *Hub) {
		if cfg.FlushInterval > 0 {
			h.queueConfig.FlushInterval = cfg.FlushInterval
		}
		if cfg.FlushBatch > 0 {
			h.queueConfig.FlushBatch = cfg.FlushBatch
		}
	}
}

// WithAlertConfig overrides alert thresholds.
func WithAlertConfig(cfg AlertConfig) HubOption {
	return func(h *Hub) {
		if cfg.Interval > 0 {
			h.alertConfig.Interval = cfg.Interval
		}
		if cfg.DropThreshold > 0 {
			h.alertConfig.DropThreshold = cfg.DropThreshold
		}
		if cfg.RetryThreshold > 0 {
			h.alertConfig.RetryThreshold = cfg.RetryThreshold
		}
		if cfg.DisconnectThreshold > 0 {
			h.alertConfig.DisconnectThreshold = cfg.DisconnectThreshold
		}
		if cfg.QueueThreshold > 0 {
			h.alertConfig.QueueThreshold = cfg.QueueThreshold
		}
	}
}

// WithMetricsRecorder injects a metrics recorder.
func WithMetricsRecorder(recorder MetricsRecorder) HubOption {
	return func(h *Hub) {
		if recorder != nil {
			h.metrics = recorder
		}
	}
}

// WithReliableGate injects a per-client reliability gate.
func WithReliableGate(gate func(ClientInfo) bool) HubOption {
	return func(h *Hub) {
		if gate != nil {
			h.reliableGate = gate
		}
	}
}

// WithReplayFilter injects an optional per-message filter applied during
// disconnect-reconnect replay. If the filter returns false the message is
// skipped. Pass nil to disable filtering (default: replay everything).
func WithReplayFilter(f ReplayFilter) HubOption {
	return func(h *Hub) {
		h.replayFilter = f
	}
}

// NewHub creates a new Hub.
func NewHub(ctx context.Context, opts ...HubOption) *Hub {
	ctx, cancel := context.WithCancel(ctx)
	h := &Hub{
		riders:       make(map[int64]*Client),
		merchants:    make(map[int64]map[*Client]struct{}),
		platforms:    make(map[int64]*Client),
		register:     make(chan *Client, 10),
		unregister:   make(chan *Client, 10),
		broadcast:    make(chan BroadcastMessage, 100),
		ctx:          ctx,
		cancel:       cancel,
		ackStore:     NewMemoryAckStore(30*time.Minute, time.Now),
		messageStore: NewMemoryMessageStore(30*time.Minute, 200, time.Now),
		idGenerator:  func() string { return uuid.NewString() },
		retryQueue:   make(chan retryItem, 1000),
		retryConfig:  RetryConfig{Timeout: 10 * time.Second, MaxRetries: 3},
		retryCounts:  make(map[string]int),
		queueConfig:  QueueConfig{FlushInterval: 200 * time.Millisecond, FlushBatch: 10},
		alertConfig:  AlertConfig{Interval: time.Minute, DropThreshold: 100, RetryThreshold: 200, DisconnectThreshold: 200, QueueThreshold: 1000},
		metrics:      noopMetricsRecorder{},
		reliableGate: func(ClientInfo) bool { return true },
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Run 启动Hub，处理注册、注销和广播
func (h *Hub) Run() {
	log.Info().Msg("WebSocket Hub started")
	defer log.Info().Msg("WebSocket Hub stopped")

	go h.retryWorker()
	go h.alertWorker()

	for {
		select {
		case <-h.ctx.Done():
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case msg := <-h.broadcast:
			h.broadcastMessage(msg)
		}
	}
}

// registerClient 注册客户端
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch client.info.ClientType {
	case ClientTypeRider:
		if old, exists := h.riders[client.info.EntityID]; exists {
			// 关闭旧连接
			old.closeDone()
		}
		h.riders[client.info.EntityID] = client
		h.updateConnectionMetricsLocked()
		log.Info().
			Int64("rider_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Rider connected via WebSocket")
		go h.flushQueue(client, "rider")

	case ClientTypeMerchant:
		if _, exists := h.merchants[client.info.EntityID]; !exists {
			h.merchants[client.info.EntityID] = make(map[*Client]struct{})
		}
		h.merchants[client.info.EntityID][client] = struct{}{}
		h.updateConnectionMetricsLocked()
		log.Info().
			Int64("merchant_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Merchant connected via WebSocket")
		go h.flushQueue(client, "merchant")

	case ClientTypePlatform:
		if old, exists := h.platforms[client.info.EntityID]; exists {
			// 关闭旧连接
			old.closeDone()
		}
		h.platforms[client.info.EntityID] = client
		h.updateConnectionMetricsLocked()
		log.Info().
			Int64("platform_user_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Platform operator connected via WebSocket")
		go h.flushQueue(client, "platform")
	}
}

// unregisterClient 注销客户端
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch client.info.ClientType {
	case ClientTypeRider:
		// ✅ 修复：只有当 map 中的 client 就是当前要注销的 client 时才删除
		// 避免新连接替换旧连接后，旧连接注销时删除了新连接
		if existing, exists := h.riders[client.info.EntityID]; exists && existing == client {
			delete(h.riders, client.info.EntityID)
			h.updateConnectionMetricsLocked()
			atomic.AddInt64(&h.alertDisconnects, 1)
			client.closeDone()
			client.closeSend()
			log.Info().
				Int64("rider_id", client.info.EntityID).
				Msg("Rider disconnected from WebSocket")
		}

	case ClientTypeMerchant:
		if connections, exists := h.merchants[client.info.EntityID]; exists {
			if _, exists := connections[client]; !exists {
				return
			}
			delete(connections, client)
			if len(connections) == 0 {
				delete(h.merchants, client.info.EntityID)
			}
			h.updateConnectionMetricsLocked()
			atomic.AddInt64(&h.alertDisconnects, 1)
			client.closeDone()
			client.closeSend()
			log.Info().
				Int64("merchant_id", client.info.EntityID).
				Msg("Merchant disconnected from WebSocket")
		}

	case ClientTypePlatform:
		if existing, exists := h.platforms[client.info.EntityID]; exists && existing == client {
			delete(h.platforms, client.info.EntityID)
			h.updateConnectionMetricsLocked()
			atomic.AddInt64(&h.alertDisconnects, 1)
			client.closeDone()
			client.closeSend()
			log.Info().
				Int64("platform_user_id", client.info.EntityID).
				Msg("Platform operator disconnected from WebSocket")
		}
	}
}

// broadcastMessage 广播消息
func (h *Hub) broadcastMessage(msg BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch msg.ClientType {
	case ClientTypeRider:
		if msg.EntityID == 0 {
			// 广播给所有骑手
			for _, client := range h.riders {
				h.sendToClient(client, msg.Message, "rider")
			}
		} else {
			// 发送给特定骑手
			if client, exists := h.riders[msg.EntityID]; exists {
				h.sendToClient(client, msg.Message, "rider")
			}
		}

	case ClientTypeMerchant:
		if msg.EntityID == 0 {
			// 广播给所有商户
			for _, connections := range h.merchants {
				for client := range connections {
					h.sendToClient(client, msg.Message, "merchant")
				}
			}
		} else {
			// 发送给特定商户
			if connections, exists := h.merchants[msg.EntityID]; exists {
				for client := range connections {
					h.sendToClient(client, msg.Message, "merchant")
				}
			}
		}

	case ClientTypePlatform:
		// 平台告警消息，广播给所有在线的平台运营人员
		for _, client := range h.platforms {
			h.sendToClient(client, msg.Message, "platform")
		}
	}
}

func (h *Hub) sendToClient(client *Client, msg Message, label string) {
	if msg.ID != "" && h.ackStore != nil {
		if h.ackStore.HasAck(h.ctx, client.info, msg.ID) {
			log.Debug().
				Str("message_id", msg.ID).
				Int64("entity_id", client.info.EntityID).
				Str("client_type", string(client.info.ClientType)).
				Msg("Skip sending acked message")
			h.metrics.RecordSend(client.info.ClientType, "skipped")
			return
		}
	}

	message := msg
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	if message.ID == "" {
		message.ID = h.idGenerator()
	}
	// 为客户端生成单调递增序号，用于断线重连后的消息回放。
	message.Sequence = client.nextSequence()
	if h.messageStore != nil {
		h.messageStore.Save(h.ctx, client.info, message)
	}
	h.storeSentAt(client.info, message)

	// 非心跳消息加入重试调度，用于“有效恰好一次”（至少一次 + 客户端幂等）。
	if message.Type != "ping" && message.Type != "pong" {
		h.scheduleRetry(client.info, message)
	}

	switch client.trySend(message) {
	case clientSendResultSent:
		h.metrics.RecordSend(client.info.ClientType, "sent")
	case clientSendResultClosed:
		h.metrics.RecordSend(client.info.ClientType, "dropped")
	case clientSendResultFull:
		// 连接侧背压：缓冲满时落入队列（若启用可靠投递）。
		if h.queueStore != nil && h.reliableGate(client.info) {
			if err := h.queueStore.Enqueue(h.ctx, client.info, message); err != nil {
				h.logDrop(label, client.info)
				h.metrics.RecordSend(client.info.ClientType, "dropped")
				atomic.AddInt64(&h.alertDrops, 1)
			} else {
				h.metrics.RecordSend(client.info.ClientType, "queued")
				log.Warn().
					Str("message_id", message.ID).
					Str("client_type", string(client.info.ClientType)).
					Int64("entity_id", client.info.EntityID).
					Msg("Send buffer full, queued message")
			}
			return
		}
		h.logDrop(label, client.info)
		h.metrics.RecordSend(client.info.ClientType, "dropped")
		atomic.AddInt64(&h.alertDrops, 1)
	}
}

func (h *Hub) logDrop(label string, info ClientInfo) {
	switch label {
	case "rider":
		log.Warn().Int64("rider_id", info.EntityID).Msg("Rider send buffer full, dropping message")
	case "merchant":
		log.Warn().Int64("merchant_id", info.EntityID).Msg("Merchant send buffer full, dropping message")
	default:
		log.Warn().Int64("platform_user_id", info.EntityID).Msg("Platform operator send buffer full, dropping message")
	}
}

func (h *Hub) sendStoredToClient(client *Client, msg Message, label string) {
	if msg.ID != "" && h.ackStore != nil {
		if h.ackStore.HasAck(h.ctx, client.info, msg.ID) {
			log.Debug().
				Str("message_id", msg.ID).
				Int64("entity_id", client.info.EntityID).
				Str("client_type", string(client.info.ClientType)).
				Msg("Skip replaying acked message")
			h.metrics.RecordSend(client.info.ClientType, "skipped")
			return
		}
	}

	message := msg
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	switch client.trySend(message) {
	case clientSendResultSent:
		h.metrics.RecordReplay(client.info.ClientType)
	case clientSendResultClosed:
		h.metrics.RecordSend(client.info.ClientType, "dropped")
	case clientSendResultFull:
		switch label {
		case "rider":
			log.Warn().Int64("rider_id", client.info.EntityID).Msg("Rider send buffer full, dropping message")
		case "merchant":
			log.Warn().Int64("merchant_id", client.info.EntityID).Msg("Merchant send buffer full, dropping message")
		default:
			log.Warn().Int64("platform_user_id", client.info.EntityID).Msg("Platform operator send buffer full, dropping message")
		}
		h.metrics.RecordSend(client.info.ClientType, "dropped")
	}
}

func (h *Hub) updateConnectionMetricsLocked() {
	if h.metrics == nil {
		return
	}
	h.metrics.RecordConnections(len(h.riders), h.onlineMerchantConnectionCountLocked(), len(h.platforms))
}

func (h *Hub) onlineMerchantConnectionCountLocked() int {
	count := 0
	for _, connections := range h.merchants {
		count += len(connections)
	}
	return count
}

// Register 注册客户端到Hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister 从Hub注销客户端
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg BroadcastMessage) {
	select {
	case h.broadcast <- msg:
	default:
		log.Warn().Msg("Broadcast channel full, dropping message")
	}
}

// SendToRider 发送消息给特定骑手
func (h *Hub) SendToRider(riderID int64, msg Message) {
	h.Broadcast(BroadcastMessage{
		ClientType: ClientTypeRider,
		EntityID:   riderID,
		Message:    msg,
	})
}

// SendToMerchant 发送消息给特定商户
func (h *Hub) SendToMerchant(merchantID int64, msg Message) {
	h.Broadcast(BroadcastMessage{
		ClientType: ClientTypeMerchant,
		EntityID:   merchantID,
		Message:    msg,
	})
}

// BroadcastToAllRiders 广播消息给所有在线骑手
func (h *Hub) BroadcastToAllRiders(msg Message) {
	h.Broadcast(BroadcastMessage{
		ClientType: ClientTypeRider,
		EntityID:   0,
		Message:    msg,
	})
}

// BroadcastToAllMerchants 广播消息给所有在线商户
func (h *Hub) BroadcastToAllMerchants(msg Message) {
	h.Broadcast(BroadcastMessage{
		ClientType: ClientTypeMerchant,
		EntityID:   0,
		Message:    msg,
	})
}

// BroadcastToRiders 广播消息给指定的骑手列表（用于按区域推送新订单）
func (h *Hub) BroadcastToRiders(riderIDs []int64, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, riderID := range riderIDs {
		if client, exists := h.riders[riderID]; exists {
			h.sendToClient(client, msg, "rider")
		}
	}
}

// RecordAck records client acknowledgements if an AckStore is configured.
func (h *Hub) RecordAck(client ClientInfo, ack Ack) {
	if h.ackStore == nil {
		return
	}
	h.ackStore.RecordAck(h.ctx, client, ack)
	h.metrics.RecordAck(client.ClientType)
	if sentAt, ok := h.popSentAt(client, ack.MessageID); ok {
		latency := time.Since(sentAt).Seconds()
		h.metrics.RecordLatency(client.ClientType, latency)
	}
	h.clearRetry(client, ack.MessageID)
}

// ReplayToClient replays messages after the provided sequence for a connected client.
func (h *Hub) ReplayToClient(info ClientInfo, afterSequence uint64, limit int) {
	if h.messageStore == nil {
		return
	}

	client := h.getClient(info)
	if client == nil {
		return
	}

	h.replayToClientConnection(client, afterSequence, limit)
}

// ReplayToClientConnection replays messages to the specific connected client.
func (h *Hub) ReplayToClientConnection(client *Client, afterSequence uint64, limit int) {
	if h.messageStore == nil || client == nil {
		return
	}

	h.replayToClientConnection(client, afterSequence, limit)
}

func (h *Hub) replayToClientConnection(client *Client, afterSequence uint64, limit int) {
	messages := h.messageStore.Replay(h.ctx, client.info, afterSequence, limit)
	if len(messages) == 0 {
		return
	}

	label := "platform"
	switch client.info.ClientType {
	case ClientTypeRider:
		label = "rider"
	case ClientTypeMerchant:
		label = "merchant"
	}

	for _, msg := range messages {
		// 若注入了业务过滤器（如骑手场景剔除已被抢走的订单），先过滤再投递
		if h.replayFilter != nil && !h.replayFilter(h.ctx, client.info, msg) {
			continue
		}
		h.sendStoredToClient(client, msg, label)
	}
}

func (h *Hub) getClient(info ClientInfo) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch info.ClientType {
	case ClientTypeRider:
		return h.riders[info.EntityID]
	case ClientTypeMerchant:
		for client := range h.merchants[info.EntityID] {
			return client
		}
		return nil
	case ClientTypePlatform:
		return h.platforms[info.EntityID]
	default:
		return nil
	}
}

type retryItem struct {
	client  ClientInfo
	message Message
	attempt int
}

func (h *Hub) scheduleRetry(client ClientInfo, msg Message) {
	if h.ackStore == nil || msg.ID == "" {
		return
	}
	if !h.reliableGate(client) {
		return
	}
	if h.retryConfig.MaxRetries <= 0 {
		return
	}

	key := h.retryKey(client, msg.ID)
	count := h.retryCount(key)
	if count >= h.retryConfig.MaxRetries {
		return
	}

	attempt := count + 1
	time.AfterFunc(h.retryConfig.Timeout, func() {
		select {
		case h.retryQueue <- retryItem{client: client, message: msg, attempt: attempt}:
		default:
			log.Warn().Str("message_id", msg.ID).Msg("Retry queue full, dropping retry")
		}
	})
}

func (h *Hub) storeSentAt(client ClientInfo, msg Message) {
	if msg.ID == "" {
		return
	}

	h.retryMu.Lock()
	if h.sentTimes == nil {
		h.sentTimes = make(map[string]time.Time)
	}
	h.sentTimes[h.retryKey(client, msg.ID)] = time.Now()
	h.retryMu.Unlock()
}

func (h *Hub) popSentAt(client ClientInfo, messageID string) (time.Time, bool) {
	if messageID == "" {
		return time.Time{}, false
	}

	key := h.retryKey(client, messageID)
	h.retryMu.Lock()
	sentAt, ok := h.sentTimes[key]
	if ok {
		delete(h.sentTimes, key)
	}
	h.retryMu.Unlock()
	return sentAt, ok
}

func (h *Hub) retryWorker() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case item := <-h.retryQueue:
			if h.ackStore != nil && h.ackStore.HasAck(h.ctx, item.client, item.message.ID) {
				h.clearRetry(item.client, item.message.ID)
				continue
			}

			if item.attempt > h.retryConfig.MaxRetries {
				continue
			}
			if !h.reliableGate(item.client) {
				continue
			}

			h.setRetryCount(item.client, item.message.ID, item.attempt)
			h.metrics.RecordRetry(item.client.ClientType)
			atomic.AddInt64(&h.alertRetries, 1)
			h.storeSentAt(item.client, item.message)
			if h.queueStore != nil {
				if err := h.queueStore.Enqueue(h.ctx, item.client, item.message); err != nil {
					log.Warn().Err(err).Str("message_id", item.message.ID).Msg("Failed to enqueue retry")
				}
			}
		}
	}
}

func (h *Hub) retryKey(client ClientInfo, messageID string) string {
	return string(client.ClientType) + ":" + messageID + ":" + strconv.FormatInt(client.EntityID, 10)
}

func (h *Hub) retryCount(key string) int {
	h.retryMu.Lock()
	defer h.retryMu.Unlock()
	return h.retryCounts[key]
}

func (h *Hub) setRetryCount(client ClientInfo, messageID string, count int) {
	h.retryMu.Lock()
	defer h.retryMu.Unlock()
	h.retryCounts[h.retryKey(client, messageID)] = count
}

func (h *Hub) clearRetry(client ClientInfo, messageID string) {
	if messageID == "" {
		return
	}
	h.retryMu.Lock()
	defer h.retryMu.Unlock()
	delete(h.retryCounts, h.retryKey(client, messageID))
}

func (h *Hub) flushQueue(client *Client, label string) {
	if h.queueStore == nil {
		return
	}

	ticker := time.NewTicker(h.queueConfig.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-client.done:
			return
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			messages, err := h.queueStore.Dequeue(h.ctx, client.info, h.queueConfig.FlushBatch)
			if err != nil {
				log.Warn().Err(err).Msg("flush queue failed")
				return
			}
			// 队列暂时为空时继续等待下次 tick，不退出 goroutine。
			// 背压场景下新消息可能在任意时刻再次入队。
			for _, msg := range messages {
				h.sendStoredToClient(client, msg, label)
			}
		}
	}
}

func (h *Hub) alertWorker() {
	if h.alertConfig.Interval <= 0 {
		return
	}

	ticker := time.NewTicker(h.alertConfig.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			drops := atomic.SwapInt64(&h.alertDrops, 0)
			retries := atomic.SwapInt64(&h.alertRetries, 0)
			disconnects := atomic.SwapInt64(&h.alertDisconnects, 0)

			if drops >= h.alertConfig.DropThreshold {
				h.SendAlert(AlertData{
					AlertType:   AlertTypeSystemError,
					Level:       AlertLevelWarning,
					Title:       "WebSocket 推送丢弃告警",
					Message:     "推送丢弃量超过阈值",
					RelatedType: "websocket",
					Extra: map[string]interface{}{
						"drops":     drops,
						"interval":  h.alertConfig.Interval.String(),
						"threshold": h.alertConfig.DropThreshold,
					},
				})
			}

			if retries >= h.alertConfig.RetryThreshold {
				h.SendAlert(AlertData{
					AlertType:   AlertTypeSystemError,
					Level:       AlertLevelWarning,
					Title:       "WebSocket 推送重试告警",
					Message:     "推送重试量超过阈值",
					RelatedType: "websocket",
					Extra: map[string]interface{}{
						"retries":   retries,
						"interval":  h.alertConfig.Interval.String(),
						"threshold": h.alertConfig.RetryThreshold,
					},
				})
			}

			if disconnects >= h.alertConfig.DisconnectThreshold {
				h.SendAlert(AlertData{
					AlertType:   AlertTypeSystemError,
					Level:       AlertLevelWarning,
					Title:       "WebSocket 连接抖动告警",
					Message:     "连接断开数量超过阈值",
					RelatedType: "websocket",
					Extra: map[string]interface{}{
						"disconnects": disconnects,
						"interval":    h.alertConfig.Interval.String(),
						"threshold":   h.alertConfig.DisconnectThreshold,
					},
				})
			}

			// 连接侧背压观测：队列堆积超过阈值时触发告警
			if backlogProvider, ok := h.queueStore.(QueueBacklogProvider); ok {
				backlog := backlogProvider.TotalSize(h.ctx)
				if backlog >= h.alertConfig.QueueThreshold {
					h.SendAlert(AlertData{
						AlertType:   AlertTypeSystemError,
						Level:       AlertLevelWarning,
						Title:       "WebSocket 队列堆积告警",
						Message:     "推送队列堆积超过阈值",
						RelatedType: "websocket",
						Extra: map[string]interface{}{
							"backlog":   backlog,
							"threshold": h.alertConfig.QueueThreshold,
						},
					})
				}
			}
		}
	}
}

// GetOnlineRiderIDs 获取所有在线骑手的ID列表
func (h *Hub) GetOnlineRiderIDs() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]int64, 0, len(h.riders))
	for id := range h.riders {
		ids = append(ids, id)
	}
	return ids
}

// IsRiderOnline 检查骑手是否在线
func (h *Hub) IsRiderOnline(riderID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.riders[riderID]
	return exists
}

// IsMerchantOnline 检查商户是否在线
func (h *Hub) IsMerchantOnline(merchantID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.merchants[merchantID]) > 0
}

// GetOnlineRiderCount 获取在线骑手数量
func (h *Hub) GetOnlineRiderCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.riders)
}

// GetOnlineMerchantCount 获取在线商户数量
func (h *Hub) GetOnlineMerchantCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.merchants)
}

// GetOnlineMerchantConnectionCount 获取在线商户 WebSocket 连接数量
func (h *Hub) GetOnlineMerchantConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.onlineMerchantConnectionCountLocked()
}

// GetOnlinePlatformCount 获取在线平台运营人员数量
func (h *Hub) GetOnlinePlatformCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.platforms)
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypePaymentTimeout        AlertType = "PAYMENT_TIMEOUT"         // 支付超时
	AlertTypeTaskEnqueueFailure    AlertType = "TASK_ENQUEUE_FAILURE"    // 任务入队失败
	AlertTypeProfitSharingFailed   AlertType = "PROFIT_SHARING_FAILED"   // 分账失败
	AlertTypeRefundFailed          AlertType = "REFUND_FAILED"           // 退款失败
	AlertTypeSystemError           AlertType = "SYSTEM_ERROR"            // 系统错误
	AlertTypePaymentAmountMismatch AlertType = "PAYMENT_AMOUNT_MISMATCH" // 支付金额不符（疑似攻击）
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelCritical AlertLevel = "critical" // 严重
	AlertLevelWarning  AlertLevel = "warning"  // 警告
	AlertLevelInfo     AlertLevel = "info"     // 信息
)

// AlertData 告警数据结构
type AlertData struct {
	AlertType   AlertType              `json:"alert_type"`   // 告警类型
	Level       AlertLevel             `json:"level"`        // 告警级别
	Title       string                 `json:"title"`        // 告警标题
	Message     string                 `json:"message"`      // 告警详情
	RelatedID   int64                  `json:"related_id"`   // 相关实体ID（订单ID、支付单ID等）
	RelatedType string                 `json:"related_type"` // 相关实体类型
	Extra       map[string]interface{} `json:"extra"`        // 额外信息
	Timestamp   time.Time              `json:"timestamp"`    // 告警时间
}

// SendAlert 发送告警给所有在线的平台运营人员
func (h *Hub) SendAlert(alert AlertData) {
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	data, err := json.Marshal(alert)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal alert data")
		return
	}

	msg := Message{
		Type:      "alert",
		Data:      data,
		Timestamp: alert.Timestamp,
	}

	h.Broadcast(BroadcastMessage{
		ClientType: ClientTypePlatform,
		EntityID:   0, // 广播给所有平台运营人员
		Message:    msg,
	})

	log.Info().
		Str("alert_type", string(alert.AlertType)).
		Str("level", string(alert.Level)).
		Str("title", alert.Title).
		Int64("related_id", alert.RelatedID).
		Int("platform_clients", h.GetOnlinePlatformCount()).
		Msg("Alert sent to platform operators")
}

// Shutdown 关闭Hub
func (h *Hub) Shutdown() {
	log.Info().Msg("Shutting down WebSocket Hub")
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 关闭所有骑手连接
	for _, client := range h.riders {
		client.closeDone()
		client.closeSend()
	}

	// 关闭所有商户连接
	for _, connections := range h.merchants {
		for client := range connections {
			client.closeDone()
			client.closeSend()
		}
	}

	// 关闭所有平台运营人员连接
	for _, client := range h.platforms {
		client.closeDone()
		client.closeSend()
	}
}
