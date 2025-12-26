package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	gorilla_websocket "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Message 通过WebSocket发送的消息结构
type Message struct {
	Type      string          `json:"type"`      // 消息类型：notification/ping/pong
	Data      json.RawMessage `json:"data"`      // 消息数据
	Timestamp time.Time       `json:"timestamp"` // 消息时间戳
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
	info     ClientInfo
	hub      *Hub
	send     chan Message
	ctx      context.Context
	done     chan struct{}
	conn     *gorilla_websocket.Conn // gorilla websocket连接
	closeOnce sync.Once               // 确保 send channel 只关闭一次
}

// Hub 管理所有WebSocket连接
type Hub struct {
	// 注册的客户端，按类型和实体ID索引
	riders    map[int64]*Client // key: rider_id
	merchants map[int64]*Client // key: merchant_id
	platforms map[int64]*Client // key: user_id（平台运营人员）

	// 注册/注销通道
	register   chan *Client
	unregister chan *Client

	// 广播消息
	broadcast chan BroadcastMessage

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

// NewHub 创建新的Hub
func NewHub(ctx context.Context) *Hub {
	ctx, cancel := context.WithCancel(ctx)
	return &Hub{
		riders:     make(map[int64]*Client),
		merchants:  make(map[int64]*Client),
		platforms:  make(map[int64]*Client),
		register:   make(chan *Client, 10),
		unregister: make(chan *Client, 10),
		broadcast:  make(chan BroadcastMessage, 100),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run 启动Hub，处理注册、注销和广播
func (h *Hub) Run() {
	log.Info().Msg("WebSocket Hub started")
	defer log.Info().Msg("WebSocket Hub stopped")

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
			close(old.done)
		}
		h.riders[client.info.EntityID] = client
		log.Info().
			Int64("rider_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Rider connected via WebSocket")

	case ClientTypeMerchant:
		if old, exists := h.merchants[client.info.EntityID]; exists {
			// 关闭旧连接
			close(old.done)
		}
		h.merchants[client.info.EntityID] = client
		log.Info().
			Int64("merchant_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Merchant connected via WebSocket")

	case ClientTypePlatform:
		if old, exists := h.platforms[client.info.EntityID]; exists {
			// 关闭旧连接
			close(old.done)
		}
		h.platforms[client.info.EntityID] = client
		log.Info().
			Int64("platform_user_id", client.info.EntityID).
			Int64("user_id", client.info.UserID).
			Msg("Platform operator connected via WebSocket")
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
			client.closeOnce.Do(func() {
				close(client.send)
			})
			log.Info().
				Int64("rider_id", client.info.EntityID).
				Msg("Rider disconnected from WebSocket")
		}

	case ClientTypeMerchant:
		// ✅ 修复：只有当 map 中的 client 就是当前要注销的 client 时才删除
		if existing, exists := h.merchants[client.info.EntityID]; exists && existing == client {
			delete(h.merchants, client.info.EntityID)
			client.closeOnce.Do(func() {
				close(client.send)
			})
			log.Info().
				Int64("merchant_id", client.info.EntityID).
				Msg("Merchant disconnected from WebSocket")
		}

	case ClientTypePlatform:
		if existing, exists := h.platforms[client.info.EntityID]; exists && existing == client {
			delete(h.platforms, client.info.EntityID)
			client.closeOnce.Do(func() {
				close(client.send)
			})
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
				select {
				case client.send <- msg.Message:
				default:
					log.Warn().
						Int64("rider_id", client.info.EntityID).
						Msg("Rider send buffer full, dropping message")
				}
			}
		} else {
			// 发送给特定骑手
			if client, exists := h.riders[msg.EntityID]; exists {
				select {
				case client.send <- msg.Message:
				default:
					log.Warn().
						Int64("rider_id", msg.EntityID).
						Msg("Rider send buffer full, dropping message")
				}
			}
		}

	case ClientTypeMerchant:
		if msg.EntityID == 0 {
			// 广播给所有商户
			for _, client := range h.merchants {
				select {
				case client.send <- msg.Message:
				default:
					log.Warn().
						Int64("merchant_id", client.info.EntityID).
						Msg("Merchant send buffer full, dropping message")
				}
			}
		} else {
			// 发送给特定商户
			if client, exists := h.merchants[msg.EntityID]; exists {
				select {
				case client.send <- msg.Message:
				default:
					log.Warn().
						Int64("merchant_id", msg.EntityID).
						Msg("Merchant send buffer full, dropping message")
				}
			}
		}

	case ClientTypePlatform:
		// 平台告警消息，广播给所有在线的平台运营人员
		for _, client := range h.platforms {
			select {
			case client.send <- msg.Message:
			default:
				log.Warn().
					Int64("platform_user_id", client.info.EntityID).
					Msg("Platform operator send buffer full, dropping message")
			}
		}
	}
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
			select {
			case client.send <- msg:
			default:
				log.Warn().
					Int64("rider_id", riderID).
					Msg("Rider send buffer full, dropping message")
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
	_, exists := h.merchants[merchantID]
	return exists
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

// GetOnlinePlatformCount 获取在线平台运营人员数量
func (h *Hub) GetOnlinePlatformCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.platforms)
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypePaymentTimeout      AlertType = "PAYMENT_TIMEOUT"       // 支付超时
	AlertTypeTaskEnqueueFailure  AlertType = "TASK_ENQUEUE_FAILURE"  // 任务入队失败
	AlertTypeProfitSharingFailed AlertType = "PROFIT_SHARING_FAILED" // 分账失败
	AlertTypeRefundFailed        AlertType = "REFUND_FAILED"         // 退款失败
	AlertTypeSystemError         AlertType = "SYSTEM_ERROR"          // 系统错误
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
	alert.Timestamp = time.Now()
	
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
		client.closeOnce.Do(func() {
			close(client.send)
		})
	}

	// 关闭所有商户连接
	for _, client := range h.merchants {
		client.closeOnce.Do(func() {
			close(client.send)
		})
	}

	// 关闭所有平台运营人员连接
	for _, client := range h.platforms {
		client.closeOnce.Do(func() {
			close(client.send)
		})
	}
}
