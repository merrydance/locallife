# Flutter App Architecture

> 作用：定义 LocalLife Flutter 移动端的分层架构、状态管理、依赖注入和目录组织约定。

跨前端的任务驱动、ViewState、API 平铺反模式和 Repository / Use Case / Presentation / UI 边界见 `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`。本文件负责 Flutter 端的具体落地方式。

## 1. 技术栈

| 层 | 技术 | 说明 |
|---|---|---|
| 框架 | Flutter 3.x | Android only（当前阶段） |
| 状态管理 | Riverpod 2.x | `StreamProvider` 适配 WebSocket 流 |
| HTTP | Dio | 拦截器、重试、Token 自动刷新 |
| WebSocket | web_socket_channel | Flutter 官方维护 |
| 路由 | GoRouter | 声明式路由，支持 deep link |
| 本地存储 | shared_preferences + sqflite | 配置 + 离线缓存 |
| 推送 | jpush_flutter | 统一厂商通道（华为/小米/OPPO/vivo） |

## 2. 目录结构

```
lib/
├── main.dart                     # 入口
├── app.dart                      # MaterialApp + GoRouter 配置
├── config/                       # 环境配置、主题
├── core/                         # 跨 feature 共享的基础设施
│   ├── network/                  # API client, WebSocket, 网络状态
│   ├── push/                     # 推送管理
│   ├── audio/                    # TTS + 音频播放
│   ├── print/                    # 打印服务
│   └── service/                  # 前台服务、轮询、消息去重
├── features/                     # 按功能模块组织
│   ├── auth/                     # 登录
│   ├── order/                    # 订单（列表、详情、接单弹窗）
│   ├── printer/                  # 打印机设置
│   ├── settings/                 # 设置、权限引导
│   └── update/                   # OTA 自更新
├── models/                       # 数据模型（跨 feature 共享）
└── widgets/                      # 通用 UI 组件
```

## 3. 分层规则

### 3.1 Feature 内部分层

每个 feature 目录内按职责分离：

```
features/order/
├── order_list_page.dart          # UI 层：纯展示 + 用户交互
├── order_detail_page.dart
├── order_alert_page.dart         # 全屏接单弹窗
├── order_provider.dart           # 逻辑层：Riverpod providers
└── order_repository.dart         # 数据层：API 调用封装（可选）
```

实现顺序必须先从商户当前任务和 ViewState 开始，再决定 provider、repository 与页面结构。不要把后端订单 entity、接口返回块或字段组直接铺成 Widget 树。

### 3.2 core/ 是共享基础设施

- `core/` 不包含业务逻辑，只提供技术能力（网络、推送、音频、打印）。
- Feature 通过 Riverpod provider 依赖 core 服务。
- core 模块之间不应互相依赖。

### 3.3 禁止项

- 不要在 Widget 中直接调用 Dio 或操作数据库。
- 不要在 provider 中直接操作 UI（如 showDialog）。
- 不要创建全局单例（除 Riverpod ProviderContainer 外）。
- 不要在 `core/` 中引入 feature 模块。

## 4. 状态管理约定

```dart
// WebSocket 消息流 — 使用 StreamProvider
final wsMessagesProvider = StreamProvider<WsMessage>((ref) {
  final wsClient = ref.watch(wsClientProvider);
  return wsClient.messageStream;
});

// API 数据 — 使用 FutureProvider
final pendingOrdersProvider = FutureProvider<List<Order>>((ref) {
  final api = ref.watch(apiClientProvider);
  return api.getPendingOrders();
});

// 可变 UI 状态 — 使用 StateNotifierProvider
final orderFilterProvider = StateNotifierProvider<OrderFilter, OrderFilterState>(
  (ref) => OrderFilter(),
);
```

## 5. 错误处理

- API 错误必须映射为中文业务提示，不得直接暴露英文错误或 HTTP status code。
- 网络异常统一在 Dio 拦截器中处理，转换为自定义 `AppException`。
- 使用 `AsyncValue` 的 `.when()` 方法处理 loading/error/data 三态。

## 6. 与后端契约

- 后端 API 契约以 Go 后端的 Swagger 文档为唯一权威来源。
- 不要在 Flutter 侧创造后端不存在的字段或状态。
- WebSocket 消息格式与 `locallife/websocket/hub.go` 中的 `Message` 结构体对齐。
