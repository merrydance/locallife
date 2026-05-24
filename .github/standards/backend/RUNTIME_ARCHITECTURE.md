# Backend Runtime Architecture

> 作用：记录 LocalLife 后端当前仓库级真实运行结构，帮助实现、接手和高风险审查建立“真实系统形状”，而不是只停留在抽象三层架构描述。

## 1. Service Shape

主服务入口：`locallife/main.go`

核心运行边界：

- HTTP transport: `locallife/api/`
- Business orchestration: `locallife/logic/`
- Persistence and transactions: `locallife/db/sqlc/`
- Async jobs: `locallife/worker/`
- Periodic jobs: `locallife/scheduler/`, 以及 `autotag/`, `session/`, `weather/`
- External integrations: `locallife/wechat/`, `locallife/maps/`, `locallife/media/`, `locallife/cloudprint/`, `locallife/ocr/`, `locallife/websocket/`

这是一个单体 Go 服务，包含多个 bounded domains；不要按微服务假设去寻找不存在的跨仓远端边界。

## 2. Startup Sequence

`locallife/main.go` 当前负责：

1. 从 `locallife/util/config.go` 加载配置
2. 执行生产环境保护检查
3. 构建 PostgreSQL 连接并运行 migration
4. 创建 `db.Store`
5. 在 Redis 可用时初始化相关能力
6. 启动 worker processor 和 schedulers
7. 构建并启动 Gin server

当前已知的生产防线包括：

- 生产环境要求显式 `ALLOWED_ORIGINS`
- 生产环境拒绝 wildcard CORS
- 金融任务队列在生产环境要求 Redis 可用
- 生产环境要求 `DATA_ENCRYPTION_KEY`

## 3. Composition Roots And Cross-Cutting Behavior

### `locallife/api/server.go`

这是后端 HTTP 组合根，集中承载：

- 路由注册
- 中间件装配
- 认证与 RBAC 入口
- 多类 handler wiring

行为变化不只来自 handler；也可能来自这里挂的 middleware。

### Middleware implications

当前全局链路会涉及：

- CORS
- security headers
- HSTS（生产环境）
- request tracing and logging
- Prometheus metrics
- rate limiting（非 `test` 环境）
- 30-second timeout middleware
- `/v1` 响应信封（webhook / websocket 例外）

所以某些“接口行为变了”的原因可能来自 middleware，而不是单个 handler。

## 4. Layer-Specific Runtime Notes

### `locallife/api/`

- 拥有 request binding、auth extraction、route entry、webhook entrypoint、response mapping。
- `api/server.go` 和对应 handler 文件一起决定外部可见行为。

### `locallife/logic/`

- 拥有业务流程与跨依赖编排。
- 常组合 `db.Store`、task distributor、地图/支付/通知客户端。
- 很多看似“只是 handler 改动”的事情，真实行为其实由 logic 决定。

### `locallife/db/sqlc/`

- 同时包含生成查询代码与手写事务文件。
- `db/sqlc/store.go` 是核心接口边界。
- 订单、支付、退款、代取等关键状态推进通常落在 `tx_*.go` 中。

### `locallife/worker/`

- 处理 timeout task、支付/退款处理、OCR 后续、打印、通知重试和恢复循环。
- 如果改动影响 post-commit 行为，只看同步请求路径通常不够。

### `locallife/scheduler/`

- 运行周期性 cleanup 和 recovery job。
- 许多 stale-state 收敛依赖 scheduler，而不是某次请求。

## 5. Core Domain Chains

### Orders and fulfillment

主订单创建链路通常要串看：

- `locallife/api/order.go`
- `locallife/logic/order_service.go`
- `locallife/db/sqlc/tx_create_order.go`

代取与履约状态常会联动：抢单、取货、代取中、送达、完成，以及押金、库存、通知、结算语义。

### Payments and funds

至少关注：

- `locallife/logic/payment_order_service.go`
- `locallife/logic/combined_payment_service.go`
- `locallife/db/sqlc/tx_payment_success.go`
- `locallife/logic/refund_service.go`
- `locallife/db/sqlc/tx_refund.go`

微信直连支付和微信电商支付并存；不要把它们概念上误合并。

### Merchant / operator platform flows

高风险且经常跨层的链路包括：

- 商户进件与 applyment
- 运营侧财务、区域治理
- 投诉、申诉、赔付与补偿

### Media and OCR

- 媒体在开发与生产环境有不同存储边界
- OCR 包含 task API、retry / dead-letter 与阿里云配置依赖

## 6. Generated Artifact Touchpoints

这些文件和流程是运行链路的一部分，不要只盯业务代码：

- SQL 查询源：`locallife/db/query/`
- sqlc 生成物与事务：`locallife/db/sqlc/`
- Swagger 输出：`locallife/docs/`
- mock 生成物：`locallife/db/mock/`, `locallife/worker/mock/`, `locallife/wechat/mock/`

原则：不要孤立审查或手改生成文件；优先追到驱动它们的手写源文件。

## 7. Practical Use

当任务是以下任一类型时，应把本文件作为热路径：

- backend takeover / onboarding
- 高风险 bugfix
- payment / refund / delivery / reservation / callback / worker / scheduler 审查
- 需要判断真实入口、真实写边界或恢复链路的实现任务

