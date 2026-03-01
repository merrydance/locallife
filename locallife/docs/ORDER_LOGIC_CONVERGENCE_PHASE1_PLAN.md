# Order 逻辑收敛第一阶段实施预案

更新时间：2026-02-28

## 1. 目标与边界

### 1.1 目标

- 将订单主链路中的业务编排从 `api/order.go`、`api/payment_order.go` 收敛到 `logic`。
- 在不改动路由行为和响应结构前提下，先建立稳定的 service interface 边界。
- 将“下单/支付/退款/状态流转/异步任务调度”解耦为可测试的独立单元。

### 1.2 非目标（第一阶段不做）

- 不改 URL、鉴权中间件、响应 envelope 结构。
- 不进行数据库 schema 变更。
- 不一次性删除现有大测试文件，仅做迁移起点与拆分路线定义。

---

## 2. 现状复杂度与风险点

- `api/order.go`：2544 行（多职责聚合：参数校验、业务编排、状态机、通知、审计、任务调度）。
- `api/payment_order.go`：1483 行（支付/退款/分账回退分支复杂）。
- `api/order_test.go`：4316 行（大量流程性 mock 串联）。
- `api/payment_order_test.go`：1187 行（支付退款混合测试）。

高风险点：

- 同一状态流转在 Handler、Logic、Worker 三处出现，容易语义漂移。
- 退款与分账回退逻辑仍在 Handler 中，外部依赖分支过多。
- 异步任务分发存在重复调用路径（例如订单超时任务在下单处有重复调度代码块）。

---

## 3. 第一阶段接口草案（建议落在 `logic/interfaces.go`）

说明：接口仅定义边界，不要求一次性替换全部调用点。优先替换 `api/order.go` 与 `api/payment_order.go`。

### 3.1 OrderCommandService

```go
type OrderCommandService interface {
    CreateOrder(ctx context.Context, input CreateOrderCommandInput) (CreateOrderCommandResult, error)
    CancelOrder(ctx context.Context, input CancelOrderCommandInput) (CancelOrderCommandResult, error)
    UrgeOrder(ctx context.Context, input UrgeOrderCommandInput) (UrgeOrderCommandResult, error)
    ReplaceOrder(ctx context.Context, input ReplaceOrderCommandInput) (ReplaceOrderCommandResult, error)
    ConfirmOrder(ctx context.Context, input ConfirmOrderCommandInput) (ConfirmOrderCommandResult, error)

    AcceptMerchantOrder(ctx context.Context, input MerchantOrderCommandInput) (MerchantOrderCommandResult, error)
    RejectMerchantOrder(ctx context.Context, input MerchantRejectOrderCommandInput) (MerchantOrderCommandResult, error)
    MarkMerchantOrderReady(ctx context.Context, input MerchantOrderCommandInput) (MerchantOrderCommandResult, error)
    CompleteMerchantOrder(ctx context.Context, input MerchantOrderCommandInput) (MerchantOrderCommandResult, error)
}
```

### 3.2 OrderQueryService

```go
type OrderQueryService interface {
    GetUserOrder(ctx context.Context, input GetUserOrderQueryInput) (GetUserOrderQueryResult, error)
    ListUserOrders(ctx context.Context, input ListUserOrdersQueryInput) (ListUserOrdersQueryResult, error)
    GetMerchantOrder(ctx context.Context, input GetMerchantOrderQueryInput) (GetMerchantOrderQueryResult, error)
    ListMerchantOrders(ctx context.Context, input ListMerchantOrdersQueryInput) (ListMerchantOrdersQueryResult, error)
    GetMerchantOrderStats(ctx context.Context, input GetMerchantOrderStatsQueryInput) (GetMerchantOrderStatsQueryResult, error)
    CalculateOrderPreview(ctx context.Context, input CalculateOrderPreviewInput) (CalculateOrderPreviewResult, error)
}
```

### 3.3 PaymentOrchestrator

```go
type PaymentOrchestrator interface {
    CreatePaymentOrder(ctx context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error)
    CreateCombinedPaymentOrder(ctx context.Context, input CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error)
    ClosePaymentOrder(ctx context.Context, input ClosePaymentOrderInput) (ClosePaymentOrderResult, error)
    CloseCombinedPaymentOrder(ctx context.Context, input CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error)
}
```

### 3.4 RefundOrchestrator

```go
type RefundOrchestrator interface {
    CreateRefundOrder(ctx context.Context, input CreateRefundOrderInput) (CreateRefundOrderResult, error)
    GetRefundOrder(ctx context.Context, input GetRefundOrderInput) (GetRefundOrderResult, error)
    ListRefundOrdersByPayment(ctx context.Context, input ListRefundOrdersByPaymentInput) (ListRefundOrdersByPaymentResult, error)
    ListProfitSharingReturnsByRefund(ctx context.Context, input ListProfitSharingReturnsByRefundInput) (ListProfitSharingReturnsByRefundResult, error)
}
```

### 3.5 横切接口（解耦外部副作用）

```go
type NotificationPublisher interface {
    Send(ctx context.Context, input NotificationInput) error
}

type AuditLogger interface {
    Write(ctx context.Context, input AuditLogInput) error
}

type OrderEventPublisher interface {
    PublishMerchantOrderSnapshot(ctx context.Context, merchantID int64, order db.Order, messageType string)
}

type TaskScheduler interface {
    ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error
    SchedulePaymentOrderTimeout(ctx context.Context, outTradeNo string, at time.Time) error
    ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error
    ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error
    ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error
}

type Clock interface {
    Now() time.Time
}

type NumberGenerator interface {
    OrderNo() string
    OutTradeNo(prefix string) string
    OutRefundNo() string
}
```

---

## 4. 建议文件拆分清单（第一阶段）

目标：先做到单文件不超过 500 行；拆分后保持原路由与 handler 函数名不变。

### 4.1 `api/order.go` 拆分

- `api/order_create.go`
  - `createOrder`
  - 下单请求结构体与参数校验（仅 HTTP 层）
- `api/order_user_commands.go`
  - `cancelOrder`
  - `urgeOrder`
  - `replaceOrder`
  - `confirmOrder`
- `api/order_user_queries.go`
  - `getOrder`
  - `listOrders`
  - `calculateOrder`
- `api/order_merchant_commands.go`
  - `acceptOrder`
  - `rejectOrder`
  - `markOrderReady`
  - `completeOrder`
- `api/order_merchant_queries.go`
  - `listMerchantOrders`
  - `getMerchantOrder`
  - `getOrderStats`
- `api/order_response.go`
  - response 组装与 pgtype helper（已存在，可继续扩展）

### 4.2 `api/payment_order.go` 拆分

- `api/payment_order_commands.go`
  - 创建/关闭支付单与合单
- `api/payment_order_queries.go`
  - 查询支付单、合单、退款单
- `api/refund_commands.go`
  - `createRefundOrder`（整段迁移前先独立文件）
- `api/refund_queries.go`
  - `getRefundOrder`
  - `listRefundOrdersByPayment`
  - `listProfitSharingReturnsByRefund`

### 4.3 `logic` 新文件建议

- `logic/interfaces.go`：放 service interface 与横切 interface。
- `logic/order_service.go`：OrderCommandService 默认实现。
- `logic/order_query.go`：OrderQueryService 默认实现。
- `logic/refund_service.go`：RefundOrchestrator 默认实现。
- `logic/payment_facade.go`：PaymentOrchestrator 默认实现（组合现有 payment/combined service）。

---

## 5. 影响范围矩阵（必须同步评估）

### 5.1 API 层

- `api/server.go`：路由不变，仅引用的 handler 所在文件变化。
- `api/payment_callback.go`：支付成功/退款回调后续可能改调 orchestrator，但第一阶段可先保持。
- `api/membership.go`、`api/rider.go`：存在 `CreatePaymentOrder` 直连 store 的逻辑，后续应纳入 PaymentOrchestrator。

### 5.2 Worker 层

- `worker/distributor.go`：TaskDistributor 已是 interface，可复用为 TaskScheduler 的底层实现。
- `worker/task_order_timeout.go` 与 `worker/task_payment_timeout.go`：状态常量与取消逻辑应与 logic 统一来源，避免硬编码散落。
- `worker/task_process_payment.go`：支付成功、退款、分账相关任务处理与新 orchestrator 的边界要明确。

### 5.3 Store / Mock 层

- `db/sqlc/store.go`：已有聚合事务接口，足够支撑第一阶段。
- `db/mock/store.go`、`worker/mock/distributor.go`、`wechat/mock/*`：接口新增后需 `make mock` 更新。

---

## 6. 测试策略（删除/重写/保留）

### 6.1 原则

- 不做“一次性删除全部大测试”。
- 先补 Service 契约测试，再迁移 Handler 流程测试。
- 删除条件：被新测试覆盖且只验证旧实现细节的冗余用例。

### 6.2 分类动作

- 保留（短期）
  - `api/order_test.go`
  - `api/payment_order_test.go`
  - 作为回归护栏，避免收敛初期行为回归。

- 重写（第一阶段开始）
  - 新增 `logic/order_service_test.go`
  - 新增 `logic/order_query_test.go`
  - 新增 `logic/refund_service_test.go`
  - 重点覆盖状态机、鉴权边界、金额与任务调度决策。

- 逐步删除（第二阶段后）
  - 将 `api/order_test.go` 拆成：
    - `api/order_create_test.go`
    - `api/order_user_commands_test.go`
    - `api/order_merchant_commands_test.go`
    - `api/order_queries_test.go`
  - 将 `api/payment_order_test.go` 拆成：
    - `api/payment_commands_test.go`
    - `api/payment_queries_test.go`
    - `api/refund_commands_test.go`
    - `api/refund_queries_test.go`

### 6.3 建议删除的“硬编码模式”（不是立刻删文件）

- 直接断言底层 SQL 调用顺序但不关心业务结果的用例。
- 复制粘贴的大量近似 case（仅字段变体差异）。
- 与未来 interface 边界冲突的 handler 内部细节断言。

---

## 7. 第一阶段实施步骤（可执行）

1. 新增 `logic/interfaces.go`，定义本文件第 3 节接口。
2. 引入默认实现骨架：`order_service.go`、`order_query.go`、`refund_service.go`。
3. 在 `api/order.go` 中优先替换 3 条命令链路：`cancelOrder`、`urgeOrder`、`confirmOrder`。
4. 在 `api/payment_order.go` 中优先替换 `createRefundOrder` 到 `RefundOrchestrator`。
5. 添加 service 层表驱动测试，覆盖 P0 主流程。
6. 保持路由与响应不变，通过现有 API 测试回归。

---

## 8. 验收标准（第一阶段）

- `api/order.go`、`api/payment_order.go` 各自关键复杂 handler 的业务分支下沉到 `logic`。
- 新增接口与实现后，handler 不再直接编排退款/分账回退细节。
- P0 链路的 service 测试通过，旧 API 回归测试不退化。
- 无行为变更（状态码、错误语义、返回结构保持兼容）。
