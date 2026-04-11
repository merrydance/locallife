# Push Notification Standards

> 作用：定义 LocalLife 商户端的消息投递可靠性标准，包括三通道架构、去重、接单确认闭环和语音播报规范。

## 1. 三通道消息架构

"不漏单"要求消息通过三条独立通道同时投递，任一通道到达即可触发处理：

| 通道 | 特性 | 延迟 | 可靠性 |
|---|---|---|---|
| ① WebSocket | App 前台时实时推送 | <100ms | 依赖网络连接，App 被杀后不可用 |
| ② 厂商推送 (JPush) | 系统级推送，穿透进程生死 | 1-3s | App 被杀也能收到（依赖厂商通道） |
| ③ 轮询兜底 | 每 30s 拉取未处理订单列表 | ≤30s | 最终一致，不依赖任何推送通道 |

### 后端投递流程

```
新订单/支付成功
  ├── ① wsHub.SendToMerchant(merchantID, msg)      // 已有
  ├── ② pushGateway.PushToMerchant(merchantID, msg) // 新增
  └── ③ 写入 notifications 表 (is_pushed 字段)       // 已有
```

### App 侧接收流程

```
任一通道消息到达
  → MessageDeduplicator.tryAccept(messageId)
  → 是新消息？
      → YES: 语音播报 + 全屏弹窗 + 本地通知
      → NO:  静默丢弃
```

## 2. 消息去重规则

### 2.1 去重键

使用后端生成的 `message_id`（UUID）作为去重键。同一个 `message_id` 无论从哪个通道到达，只处理一次。

### 2.2 双层缓存

- **内存层**: LRU Map，容量 500，满了淘汰最旧的。适用于短时间内的重复。
- **持久层**: sqflite 表，保留 24 小时。适用于 App 重启后的重复。

### 2.3 时序保证

去重器必须是同步的（单线程调用），不能出现两个通道同时通过去重检查的竞态。Dart 的单线程事件循环天然保证了这一点，但要确保不在 Isolate 中分别持有去重器实例。

## 3. 接单确认闭环

### 3.1 商户侧

1. 新订单到达 → 去重 → 全屏弹窗展示订单信息
2. 弹窗中显示倒计时（60 秒）
3. 商户点击"接单" → `POST /v1/merchant/orders/:id/accept`
4. 接单成功 → 触发打印 + 关闭弹窗 + 语音提示"接单成功"

### 3.2 后端侧（超时处理）

1. 定时任务每 30 秒扫描 `status = 'paid' AND created_at < now() - 60s` 的订单
2. 对超时未接单的订单：
   - 再次推送（三通道）
   - 写入 `platform_alert_events` 表
   - WebSocket 推送给平台运营（`ClientTypePlatform`）
3. 超时 5 分钟仍未接单 → 标记为需人工介入

### 3.3 幂等性

- `POST /v1/merchant/orders/:id/accept` 必须是幂等的。重复调用返回成功，不产生副作用。
- 后端使用条件更新：`UPDATE orders SET status = 'accepted' WHERE id = ? AND status = 'paid'`

## 4. 语音播报规范

### 4.1 预录音频（高优先级）

用于固定台词，音质和音量有保证：

| 文件 | 内容 | 时机 |
|---|---|---|
| `new_order.mp3` | "您有新的乐客来福订单，请及时处理" | 新订单到达 |
| `order_timeout.mp3` | "订单即将超时，请尽快处理" | 倒计时 <15 秒 |
| `network_error.mp3` | "网络连接异常，请检查网络" | WebSocket 断线 >10 秒 |

### 4.2 TTS 动态播报（预录音频播完后）

使用 `flutter_tts` 朗读动态内容：

```
"订单 {order_number} 号，金额 {amount} 元"
```

### 4.3 音频策略

- 使用 Android 的 `STREAM_ALARM` 音频流，确保在静音模式下也能播放。
- 播报时临时提升系统音量到 80%，播完恢复。
- 如果正在播报时又有新订单，加入队列依次播报，不中断当前播报。

## 5. 厂商推送 (JPush) 集成规范

### 5.1 Token 注册

- App 启动时获取 JPush Registration ID
- 调用 `POST /v1/merchant/device/register` 上报给后端
- Token 变化时（JPush 回调）立即重新上报

### 5.2 消息处理

JPush 消息分两类：

| 类型 | 说明 | 处理方式 |
|---|---|---|
| 通知消息 (Notification) | 系统通知栏展示 | 用户点击后进入 App，App 解析 `extras` 中的 `message_id` 和 `order_id` |
| 自定义消息 (Custom Message) | App 在前台时直接收到 | 直接走去重 → 播报流程 |

### 5.3 后端推送规则

- 使用 JPush 的 `registration_id` 精准推送，不用广播。
- 推送 priority 设为 HIGH。
- 必须在 `extras` 中携带 `message_id`、`order_id`、`type`。
- 启用所有厂商的第三方通道（`third_party_channel`）。

## 6. Monitor & Alerting

### 6.1 App 侧监控

- 连接状态变化写入本地日志（`sqflite`）
- 定期上报心跳：`PUT /v1/merchant/device/heartbeat`
- 心跳包含：WebSocket 连接状态、推送 Token 是否有效、最近接收的订单 ID

### 6.2 后端侧监控

- 商户设备 `last_active` 超过 5 分钟未更新 → 标记为离线
- 离线商户有新订单 → 额外通过短信/电话提醒（后续迭代）
