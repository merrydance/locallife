package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// eventually 是对 require.Eventually 的薄封装，统一超时与轮询间隔。
// 用于替代测试中的 time.Sleep + 断言，消除依赖固定等待时间的竞态条件。
func eventually(t *testing.T, condition func() bool, msgAndArgs ...interface{}) {
	t.Helper()
	require.Eventually(t, condition, time.Second, time.Millisecond, msgAndArgs...)
}

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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online after register")
	require.Equal(t, 1, hub.GetOnlineRiderCount())

	// 注销
	hub.Unregister(client)
	eventually(t, func() bool { return !hub.IsRiderOnline(100) }, "rider should be offline after unregister")
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
	eventually(t, func() bool { return hub.IsMerchantOnline(200) }, "merchant should be online after register")
	require.Equal(t, 1, hub.GetOnlineMerchantCount())

	hub.Unregister(client)
	eventually(t, func() bool { return !hub.IsMerchantOnline(200) }, "merchant should be offline after unregister")
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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online after first register")

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
	// 旧连接应该被关闭
	select {
	case <-oldClient.done:
		// 预期行为
	case <-time.After(time.Second):
		t.Error("old client's done channel should be closed after new connection")
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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online after first register")

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
	// 等待新连接处理完成（旧连接被踢出后新连接才算完全注册）
	eventually(t, func() bool {
		select {
		case <-oldClient.done:
			return true
		default:
			return false
		}
	}, "old client should be evicted after new connection")

	// 尝试注销旧连接（应该不起作用，因为 map 中现在是新连接）
	hub.Unregister(oldClient)
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "new client should still be online after old client unregister")
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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online before sending message")

	// 发送消息
	testMsg := Message{
		Type:      "notification",
		Data:      json.RawMessage(`{"test": "data"}`),
		Timestamp: time.Now(),
	}

	hub.SendToRider(100, testMsg)

	// 验证消息接收
	select {
	case received := <-client.send:
		require.Equal(t, "notification", received.Type)
	case <-time.After(time.Second):
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
	eventually(t, func() bool { return hub.IsMerchantOnline(200) }, "merchant should be online before sending message")

	testMsg := Message{
		Type:      "new_order",
		Data:      json.RawMessage(`{"order_id": 123}`),
		Timestamp: time.Now(),
	}

	hub.SendToMerchant(200, testMsg)

	select {
	case received := <-client.send:
		require.Equal(t, "new_order", received.Type)
	case <-time.After(time.Second):
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
	eventually(t, func() bool { return hub.GetOnlineRiderCount() == 3 }, "all 3 riders should be online")

	// 广播消息
	testMsg := Message{
		Type:      "broadcast",
		Data:      json.RawMessage(`{"message": "hello all"}`),
		Timestamp: time.Now(),
	}

	hub.BroadcastToAllRiders(testMsg)

	// 验证所有骑手都收到消息
	for i, client := range clients {
		select {
		case received := <-client.send:
			require.Equal(t, "broadcast", received.Type, "client %d should receive broadcast", i)
		case <-time.After(time.Second):
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
	eventually(t, func() bool { return hub.GetOnlineRiderCount() == 3 }, "all 3 riders should be online")

	// 只广播给骑手100和200
	testMsg := Message{
		Type:      "targeted_broadcast",
		Data:      json.RawMessage(`{"region": 1}`),
		Timestamp: time.Now(),
	}

	hub.BroadcastToRiders([]int64{100, 200}, testMsg)

	// 骑手100和200应该收到消息
	for i := 0; i < 2; i++ {
		select {
		case received := <-clients[i].send:
			require.Equal(t, "targeted_broadcast", received.Type)
		case <-time.After(time.Second):
			t.Errorf("client %d should receive targeted broadcast", i)
		}
	}

	// 骑手300不应该收到消息
	select {
	case <-clients[2].send:
		t.Error("client 2 (rider 300) should NOT receive targeted broadcast")
	case <-time.After(200 * time.Millisecond):
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
	eventually(t, func() bool { return hub.GetOnlineRiderCount() == 3 }, "all 3 riders should be online")

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
			// 并发检查在线状态（不等待处理完成，测试 map 的并发安全性）
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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online before sending")

	// 发送多条消息，应该不会阻塞
	for i := 0; i < 10; i++ {
		hub.SendToRider(100, Message{
			Type:      "test",
			Timestamp: time.Now(),
		})
	}
	// 等待广播消息被 run loop 处理完：channel 满后再发送一条并等该条被投递
	// 用 Eventually 探测 send channel 中至少有 1 条消息（buffer=1，所以就是满）
	eventually(t, func() bool { return len(client.send) == 1 }, "send buffer should be full")

	// 只有一条消息被接收，其余被丢弃
	require.Len(t, client.send, 1)
}

func TestHub_AckDedup(t *testing.T) {
	ctx := context.Background()
	ackStore := NewMemoryAckStore(5*time.Minute, time.Now)
	hub := NewHub(ctx, WithAckStore(ackStore))

	go hub.Run()
	defer hub.Shutdown()

	client := &Client{
		info: ClientInfo{
			UserID:     1,
			ClientType: ClientTypeRider,
			EntityID:   100,
		},
		hub:  hub,
		send: make(chan Message, 10),
		done: make(chan struct{}),
	}

	hub.Register(client)
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online before ack test")

	msgID := "msg-ack-1"
	hub.SendToRider(100, Message{
		ID:        msgID,
		Type:      "notification",
		Timestamp: time.Now(),
	})

	var received Message
	select {
	case received = <-client.send:
		require.Equal(t, msgID, received.ID)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected to receive first message")
	}

	hub.RecordAck(client.info, Ack{MessageID: msgID, Sequence: received.Sequence, Timestamp: time.Now()})

	hub.SendToRider(100, Message{
		ID:        msgID,
		Type:      "notification",
		Timestamp: time.Now(),
	})

	select {
	case <-client.send:
		t.Fatal("expected dedup to skip sending acked message")
	case <-time.After(100 * time.Millisecond):
		// expected
	}
}

func TestHub_Replay(t *testing.T) {
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
		send: make(chan Message, 10),
		done: make(chan struct{}),
	}

	hub.Register(client)
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online before replay test")

	msgID := "msg-replay-1"
	hub.SendToRider(100, Message{
		ID:        msgID,
		Type:      "notification",
		Timestamp: time.Now(),
	})

	select {
	case received := <-client.send:
		require.Equal(t, msgID, received.ID)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected to receive initial message")
	}

	hub.ReplayToClient(client.info, 0, 1)

	select {
	case replayed := <-client.send:
		require.Equal(t, msgID, replayed.ID)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected replayed message")
	}
}

func BenchmarkHubBroadcastToRiders(b *testing.B) {
	ctx := context.Background()
	hub := NewHub(ctx)

	go hub.Run()
	defer hub.Shutdown()

	const riderCount = 200
	for i := int64(1); i <= riderCount; i++ {
		client := &Client{
			info: ClientInfo{
				UserID:     i,
				ClientType: ClientTypeRider,
				EntityID:   i,
			},
			hub:  hub,
			send: make(chan Message, 256),
			done: make(chan struct{}),
		}
		hub.Register(client)
	}
	// ensure registration processing: spin until all riders are registered
	for hub.GetOnlineRiderCount() < riderCount {
		time.Sleep(time.Millisecond)
	}

	msg := Message{Type: "benchmark", Data: json.RawMessage(`{"benchmark":true}`)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastToAllRiders(msg)
	}
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
	eventually(t, func() bool { return hub.IsRiderOnline(100) }, "rider should be online before shutdown")

	// 关闭 Hub
	hub.Shutdown()

	// send channel 应该被关闭（Shutdown 会 close 所有 client.send）
	select {
	case _, ok := <-client.send:
		require.False(t, ok, "send channel should be closed after shutdown")
	case <-time.After(time.Second):
		t.Fatal("send channel was not closed after shutdown")
	}
}
