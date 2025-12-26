package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewHub(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	require.NotNil(t, hub)
	require.NotNil(t, hub.riders)
	require.NotNil(t, hub.merchants)
	require.NotNil(t, hub.register)
	require.NotNil(t, hub.unregister)
	require.NotNil(t, hub.broadcast)
}

func TestHub_RegisterAndUnregisterRider(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	// 启动 Hub
	go hub.Run()
	defer hub.Shutdown()

	// 创建骑手客户端（无实际连接，用于测试）
	client := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	// 注册
	hub.Register(client)
	time.Sleep(50 * time.Millisecond) // 等待处理

	require.True(t, hub.IsRiderOnline(100))
	require.Equal(t, 1, hub.GetOnlineRiderCount())

	// 注销
	hub.Unregister(client)
	time.Sleep(50 * time.Millisecond)

	require.False(t, hub.IsRiderOnline(100))
	require.Equal(t, 0, hub.GetOnlineRiderCount())
}

func TestHub_RegisterAndUnregisterMerchant(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	client := &Client{
		info: ClientInfo{
			UserID:     2,
			ClientType: ClientTypeMerchant,
			EntityID:   200,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	require.True(t, hub.IsMerchantOnline(200))
	require.Equal(t, 1, hub.GetOnlineMerchantCount())

	hub.Unregister(client)
	time.Sleep(50 * time.Millisecond)

	require.False(t, hub.IsMerchantOnline(200))
	require.Equal(t, 0, hub.GetOnlineMerchantCount())
}

func TestHub_ReplaceOldConnection(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 创建旧连接
	oldClient := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(oldClient)
	time.Sleep(50 * time.Millisecond)

	require.True(t, hub.IsRiderOnline(100))

	// 创建新连接（同一骑手ID）
	newClient := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(newClient)
	time.Sleep(50 * time.Millisecond)

	// 旧连接应该被关闭
	select {
	case <-oldClient.done:
		// 预期行为
	default:
		t.Error("old client's done channel should be closed")
	}

	// 新连接应该在线
	require.True(t, hub.IsRiderOnline(100))
	require.Equal(t, 1, hub.GetOnlineRiderCount())
}

func TestHub_UnregisterWrongClient(t *testing.T) {
	// 测试：旧连接注销时不应删除新连接
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 创建并注册旧连接
	oldClient := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(oldClient)
	time.Sleep(50 * time.Millisecond)

	// 创建并注册新连接（替换旧连接）
	newClient := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(newClient)
	time.Sleep(50 * time.Millisecond)

	// 尝试注销旧连接（应该不起作用，因为 map 中现在是新连接）
	hub.Unregister(oldClient)
	time.Sleep(50 * time.Millisecond)

	// 新连接应该仍然在线
	require.True(t, hub.IsRiderOnline(100))
	require.Equal(t, 1, hub.GetOnlineRiderCount())
}

func TestHub_SendToRider(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	client := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	// 发送消息
	testMsg := Message{
		Type:      "notification",
		Data:      json.RawMessage(`{"test": "data"}`),
		Timestamp: time.Now(),
	}

	hub.SendToRider(100, testMsg)
	time.Sleep(50 * time.Millisecond)

	// 验证消息接收
	select {
	case received := <-client.send:
		require.Equal(t, "notification", received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive message")
	}
}

func TestHub_SendToMerchant(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	client := &Client{
		info: ClientInfo{
			UserID:     2,
			ClientType: ClientTypeMerchant,
			EntityID:   200,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	testMsg := Message{
		Type:      "new_order",
		Data:      json.RawMessage(`{"order_id": 123}`),
		Timestamp: time.Now(),
	}

	hub.SendToMerchant(200, testMsg)
	time.Sleep(50 * time.Millisecond)

	select {
	case received := <-client.send:
		require.Equal(t, "new_order", received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive message")
	}
}

func TestHub_BroadcastToAllRiders(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 创建多个骑手
	var clients []*Client
	for i := int64(1); i <= 3; i++ {
		client := &Client{
			info: ClientInfo{
				UserID:     i,
				ClientType: ClientTypeRider,
				EntityID:   i * 100,
			},
			hub:  hub,
			send: make(chan Message, 256),
			done: make(chan struct{}),
		}
		clients = append(clients, client)
		hub.Register(client)
	}
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, 3, hub.GetOnlineRiderCount())

	// 广播消息
	testMsg := Message{
		Type:      "broadcast",
		Data:      json.RawMessage(`{"message": "hello all"}`),
		Timestamp: time.Now(),
	}

	hub.BroadcastToAllRiders(testMsg)
	time.Sleep(50 * time.Millisecond)

	// 验证所有骑手都收到消息
	for i, client := range clients {
		select {
		case received := <-client.send:
			require.Equal(t, "broadcast", received.Type, "client %d should receive broadcast", i)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d did not receive broadcast", i)
		}
	}
}

func TestHub_BroadcastToRiders(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 创建3个骑手
	var clients []*Client
	for i := int64(1); i <= 3; i++ {
		client := &Client{
			info: ClientInfo{
				UserID:     i,
				ClientType: ClientTypeRider,
				EntityID:   i * 100,
			},
			hub:  hub,
			send: make(chan Message, 256),
			done: make(chan struct{}),
		}
		clients = append(clients, client)
		hub.Register(client)
	}
	time.Sleep(50 * time.Millisecond)

	// 只广播给骑手100和200
	testMsg := Message{
		Type:      "targeted_broadcast",
		Data:      json.RawMessage(`{"region": 1}`),
		Timestamp: time.Now(),
	}

	hub.BroadcastToRiders([]int64{100, 200}, testMsg)
	time.Sleep(50 * time.Millisecond)

	// 骑手100和200应该收到消息
	for i := 0; i < 2; i++ {
		select {
		case received := <-clients[i].send:
			require.Equal(t, "targeted_broadcast", received.Type)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d should receive targeted broadcast", i)
		}
	}

	// 骑手300不应该收到消息
	select {
	case <-clients[2].send:
		t.Error("client 2 (rider 300) should NOT receive targeted broadcast")
	case <-time.After(50 * time.Millisecond):
		// 预期行为
	}
}

func TestHub_GetOnlineRiderIDs(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 注册几个骑手
	for i := int64(1); i <= 3; i++ {
		client := &Client{
			info: ClientInfo{
				UserID:     i,
				ClientType: ClientTypeRider,
				EntityID:   i * 100,
			},
			hub:  hub,
			send: make(chan Message, 256),
			done: make(chan struct{}),
		}
		hub.Register(client)
	}
	time.Sleep(50 * time.Millisecond)

	ids := hub.GetOnlineRiderIDs()
	require.Len(t, ids, 3)

	// 验证所有ID都在
	idSet := make(map[int64]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	require.True(t, idSet[100])
	require.True(t, idSet[200])
	require.True(t, idSet[300])
}

func TestHub_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// 并发注册和注销
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()

			client := &Client{
				info: ClientInfo{
					UserID:     id,
					ClientType: ClientTypeRider,
					EntityID:   id,
				},
				hub:  hub,
				send: make(chan Message, 256),
				done: make(chan struct{}),
			}

			hub.Register(client)
			time.Sleep(10 * time.Millisecond)

			// 并发检查在线状态
			_ = hub.IsRiderOnline(id)
			_ = hub.GetOnlineRiderCount()

			hub.Unregister(client)
		}(int64(i))
	}

	wg.Wait()
}

func TestHub_SendBufferFull(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	// 创建一个 send buffer 很小的客户端
	client := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 1), // 只有1个buffer
		done: make(chan struct{}),
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	// 发送多条消息，应该不会阻塞
	for i := 0; i < 10; i++ {
		hub.SendToRider(100, Message{
			Type:      "test",
			Timestamp: time.Now(),
		})
	}
	time.Sleep(50 * time.Millisecond)

	// 只有一条消息被接收，其余被丢弃
	require.Len(t, client.send, 1)
}

func TestHub_Shutdown(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()

	// 注册客户端
	client := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 256),
		done: make(chan struct{}),
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	// 关闭 Hub
	hub.Shutdown()
	time.Sleep(50 * time.Millisecond)

	// send channel 应该被关闭
	_, ok := <-client.send
	require.False(t, ok, "send channel should be closed after shutdown")
}
