package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	// 写超时
	writeWait = 10 * time.Second

	// pong等待超时（客户端必须在此时间内发送pong）
	pongWait = 60 * time.Second

	// ping间隔（必须小于pongWait）
	pingPeriod = (pongWait * 9) / 10

	// 最大消息大小
	maxMessageSize = 512 * 1024 // 512KB
)

// NewClient 创建新的客户端连接
func NewClient(hub *Hub, conn *websocket.Conn, info ClientInfo) *Client {
	return &Client{
		info: info,
		hub:  hub,
		send: make(chan Message, 256),
		ctx:  context.Background(),
		done: make(chan struct{}),
		conn: conn,
	}
}

// ReadPump 从WebSocket读取消息
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.done:
			return
		default:
		}

		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).
					Int64("user_id", c.info.UserID).
					Str("client_type", string(c.info.ClientType)).
					Msg("WebSocket read error")
			}
			break
		}

		// 处理客户端消息（如心跳、确认等）
		c.handleMessage(msg)
	}
}

// WritePump 向WebSocket写入消息
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub关闭了通道
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(message)
			if err != nil {
				log.Error().Err(err).
					Int64("user_id", c.info.UserID).
					Str("client_type", string(c.info.ClientType)).
					Msg("WebSocket write error")
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端发送的消息
func (c *Client) handleMessage(msg Message) {
	switch msg.Type {
	case "pong":
		// 心跳响应，重置超时
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

	case "ack":
		// 消息确认（可用于追踪消息送达）
		var ackData struct {
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(msg.Data, &ackData); err == nil {
			log.Debug().
				Str("message_id", ackData.MessageID).
				Int64("user_id", c.info.UserID).
				Msg("Message acknowledged")
		}

	default:
		log.Warn().
			Str("type", msg.Type).
			Int64("user_id", c.info.UserID).
			Msg("Unknown message type from client")
	}
}
