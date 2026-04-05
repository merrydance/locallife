# LocalLife 后端工程一致性指南

> Go 代码组织、`context`、并发、接口与测试实践见 `.github/standards/backend/GO_PRACTICES.md`。
> SQL 编写、migration、索引与 sqlc 约定见 `.github/standards/backend/SQL_STANDARDS.md`。

## 架构概述

本项目采用 **HTTP 三层架构**，Go 1.25，Gin 框架：

```
Handler (api/)  →  Logic (logic/)  →  Store (db/sqlc/)  →  PostgreSQL
         ↘  Worker (worker/)  →  Redis (asynq)
         ↘  Scheduler (scheduler/)  →  Cron
```

- **Handler 层** (`api/`): HTTP 请求/响应、参数绑定与验证、认证鉴权、响应 DTO
  转换。禁止包含业务逻辑。
- **Logic 层** (`logic/`): 纯业务逻辑。接收显式 Input 结构体，返回 Result
  结构体或 `RequestError`。不依赖 Gin Context。
- **Store 层** (`db/sqlc/`): SQLC 自动生成的类型安全查询 + 手写事务方法
  (`tx_*.go`)。通过 `Store` 接口暴露，便于 mock。
- **Worker 层** (`worker/`): 基于 asynq 的异步任务。分发通过 `TaskDistributor`
  接口，处理通过 `TaskProcessor` 接口。
- **Scheduler 层** (`scheduler/`): 基于 cron 的定时任务。通过
  `RunnableScheduler` 接口注册到 `Manager`。

---

## I. 分层职责与边界

### Handler 层 (`api/`)

Handler 只做五件事：

1. **参数绑定** — `ctx.ShouldBindJSON/URI/Query`
2. **认证提取** — `ctx.MustGet(authorizationPayloadKey).(*token.Payload)`
3. **调用 Logic/Store** — 业务逻辑委托给 `logic.*` 函数或 `server.store.*` 方法
4. **错误映射** — `writeLogicRequestError` 处理 logic 层返回的
   `RequestError`；`internalError` 处理 500
5. **响应转换** — 通过 `newXxxResponse()` 将 DB 模型转为 API DTO

```go
func (server *Server) createPaymentOrder(ctx *gin.Context) {
    var req createPaymentOrderRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {
        ctx.JSON(http.StatusBadRequest, errorResponse(err))
        return
    }

    authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

    service := logic.NewPaymentOrderService(server.store, server.paymentClient)
    result, err := service.CreatePaymentOrder(ctx, logic.CreatePaymentOrderInput{...})
    if err != nil {
        if writeLogicRequestError(ctx, err) {
            return
        }
        ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
        return
    }

    ctx.JSON(http.StatusOK, newPaymentOrderResponse(result.PaymentOrder))
}
```

### Logic 层 (`logic/`)

Logic 函数的统一签名模式：

```go
// 纯函数风格（大多数情况）
func FunctionName(ctx context.Context, store db.Store, input InputStruct) (ResultStruct, error)

// 或服务风格（需要多个依赖时）
type XxxService struct { store db.Store; client SomeInterface }
func NewXxxService(store db.Store, client SomeInterface) *XxxService
func (s *XxxService) Method(ctx context.Context, input Input) (Result, error)
```

- 所有业务错误通过 `logic.NewRequestError(httpStatus, err)` 返回
- 不直接操作 `gin.Context`，只接收标准 `context.Context`
- Input/Result 结构体在同文件中定义

### Store 层 (`db/sqlc/`)

```
db/
├── migration/     # SQL 迁移文件（make new_migration name=xxx 生成）
├── query/         # SQLC 查询定义（.sql 文件）
├── sqlc/          # SQLC 生成代码 + 手写事务（tx_*.go, constants.go, store.go）
└── mock/          # mockgen 生成的 mock
```

- **Store 接口** (`store.go`): 组合 `Querier`（SQLC 自动生成）+ 手写事务方法
- **事务方法** 命名为 `tx_*.go`，使用 `execTx` 模式
- **常量 SSOT** (`constants.go`): 状态枚举的唯一真实来源，其他层引用此处

---

## II. 错误处理

### 三级错误体系

| 层级             | 函数                                                            | 用途                                |
| ---------------- | --------------------------------------------------------------- | ----------------------------------- |
| 客户端错误 (4xx) | `errorResponse(err)`                                            | 返回实际错误信息给客户端            |
| 业务错误 (4xx)   | `logic.NewRequestError(status, err)` → `writeLogicRequestError` | Logic 层→Handler 层的结构化错误传递 |
| 服务端错误 (5xx) | `internalError(ctx, err)`                                       | 日志记录实际错误，返回安全泛化信息  |

```go
// Logic 层抛出业务错误
func ValidateMerchantForOrder(ctx context.Context, ...) {
    return logic.NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
}

// Handler 层统一接收
if writeLogicRequestError(ctx, err) {
    return  // 已写入响应
}
ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
```

### 统一响应信封

所有 `/v1/` 下的 JSON 响应通过 `ResponseEnvelopeMiddleware` 自动包装为：

```json
// 成功 (2xx)
{"code": 0, "message": "ok", "data": {...}}

// 客户端错误 (4xx)
{"code": 40000, "message": "error description", "data": {...}}

// 服务端错误 (5xx) — 不泄露内部信息
{"code": 50000, "message": "internal server error"}
```

错误码映射：`40000=BadRequest`, `40100=Unauthorized`, `40300=Forbidden`,
`40400=NotFound`, `40900=Conflict`, `50000=InternalError`。

跳过包装：WebSocket 升级、`/v1/webhooks/`、`X-Response-Envelope: 0`。

---

## III. 数据建模规范

### 命名规范

- **表名**: 复数小写 `orders`, `users`, `merchants`
- **字段名**: snake_case `created_at`, `user_id`
- **主键**: `id bigserial PRIMARY KEY`
- **外键**: `{表名单数}_id` 格式
- **时间戳**: 统一使用 `timestamptz`

### 字段类型映射

| 用途      | PostgreSQL     | Go 类型                       | JSON               |
| --------- | -------------- | ----------------------------- | ------------------ |
| 主键      | `bigserial`    | `int64`                       | `"id": 123`        |
| 金额      | `bigint`（分） | `int64`                       | `"amount": 2880`   |
| 状态枚举  | `varchar(20)`  | `string`                      | `"status": "paid"` |
| JSON 数据 | `jsonb`        | `json.RawMessage` / `[]byte`  | 内嵌对象           |
| 时间      | `timestamptz`  | `time.Time`                   | ISO 8601           |
| 可选文本  | `varchar NULL` | `pgtype.Text` → API `*string` | `omitempty`        |
| 可选整数  | `bigint NULL`  | `pgtype.Int8` → API `*int64`  | `omitempty`        |

### 必备字段

```sql
CREATE TABLE xxx (
    id bigserial PRIMARY KEY,
    -- 业务字段...
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

### pgtype ↔ 指针转换模式

```go
// DB → API：pgtype → 指针
if o.AddressID.Valid {
    resp.AddressID = &o.AddressID.Int64
}

// API → DB：指针 → pgtype
params.AddressID = pgtype.Int8{Int64: *req.AddressID, Valid: req.AddressID != nil}
```

---

## IV. SQLC 查询规范

```sql
-- name: GetUser :one
SELECT id, username, email, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;

-- name: ListUsers :many
SELECT id, username, email, created_at, updated_at
FROM users
ORDER BY id
LIMIT $1 OFFSET $2;

-- name: CreateUser :one
INSERT INTO users (username, email) VALUES ($1, $2) RETURNING *;

-- name: UpdateUser :one
UPDATE users SET username = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
```

### 可选参数处理

```sql
-- name: UpdateUserOptional :one
UPDATE users SET
    username = COALESCE(sqlc.narg(username), username),
    email = COALESCE(sqlc.narg(email), email),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

### 事务模式 (`tx_*.go`)

```go
func (store *SQLStore) XxxTx(ctx context.Context, arg XxxTxParams) (XxxTxResult, error) {
    var result XxxTxResult
    err := store.execTx(ctx, func(q *Queries) error {
        var err error
        result.Item, err = q.SomeOperation(ctx, ...)
        if err != nil { return err }
        // 更多操作...
        return nil
    })
    return result, err
}
```

---

## V. 常量管理

### SSOT 层级

状态枚举等业务常量的唯一真实来源在 `db/sqlc/constants.go`：

```go
// db/sqlc/constants.go — SSOT
const (
    OrderStatusPending   = "pending"
    OrderStatusPaid      = "paid"
    OrderStatusCompleted = "completed"
    // ...
)
```

其他层引用而非重定义：

```go
// api/order.go — 引用 SSOT
const (
    OrderStatusPending = db.OrderStatusPending
    OrderStatusPaid    = db.OrderStatusPaid
)
```

### 常量分类

| 文件                        | 内容                                                 |
| --------------------------- | ---------------------------------------------------- |
| `db/sqlc/constants.go`      | 订单状态、履约状态（与数据库 CHECK 约束对齐）        |
| `api/constants.go`          | WebSocket 事件类型、预约状态、桌台状态、业务阈值常量 |
| `api/order.go` 等各文件顶部 | 该模块专用的类型/状态常量                            |
| `worker/processor.go`       | 任务队列名称（`QueueCritical`, `QueueDefault`）      |

---

## VI. 中间件栈

路由在 `api/server.go` 的 `setupRouter()` 中注册，中间件执行顺序：

```
CORS → SecurityHeaders → HSTS (prod) → RequestTracing → RequestLogging
→ Prometheus → RateLimit (非 test) → Timeout(30s) → ResponseEnvelope (v1/)
→ Auth (认证组) → RBAC/MerchantStaff (特定路由)
```

### 关键中间件

| 中间件                             | 文件                       | 职责                                                 |
| ---------------------------------- | -------------------------- | ---------------------------------------------------- |
| `authMiddleware`                   | `middleware.go`            | Bearer Token 验证，支持 WebSocket `?token=` 查询参数 |
| `CasbinRoleMiddleware`             | `rbac_middleware.go`       | 基于 Casbin 的 RBAC 角色校验                         |
| `MerchantStaffMiddleware`          | `rbac_middleware.go`       | 商户员工角色校验（owner/manager/chef/cashier）       |
| `LoadOperatorMiddleware`           | `rbac_middleware.go`       | 加载运营商信息到 Context                             |
| `ValidateOperatorRegionMiddleware` | `rbac_middleware.go`       | 验证运营商对区域的管辖权                             |
| `ResponseEnvelopeMiddleware`       | `response_envelope.go`     | 统一 `{code, message, data}` 信封                    |
| `TimeoutMiddleware`                | `middleware.go`            | 全局 30s 超时，SSE 跳过                              |
| `RateLimiter`                      | `middleware_ratelimit.go`  | 全局 + 敏感接口独立限流                              |
| `PrometheusMiddleware`             | `middleware_prometheus.go` | 请求指标采集                                         |
| `RequestTracingMiddleware`         | `middleware_tracing.go`    | 生成 `X-Request-ID`                                  |

---

## VII. 认证与权限

### Token 体系

- **PASETO v2** 对称加密令牌
- Access Token + Refresh Token 双令牌
- 微信小程序登录：`/v1/auth/wechat-login` 返回双令牌
- Web 扫码登录：`/v1/auth/web-login/*` 系列接口

### 权限检查模式

```go
// 1. 认证提取
authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

// 2. 所有权验证
if resource.UserID != authPayload.UserID {
    ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
    return
}

// 3. RBAC 中间件（路由组级别）
group.Use(server.CasbinRoleMiddleware(RoleOperator))

// 4. 商户员工中间件
group.Use(server.MerchantStaffMiddleware("owner", "manager"))
```

---

## VIII. 异步任务 (asynq)

### 分发与处理

```go
// 分发：通过 TaskDistributor 接口
_ = server.taskDistributor.DistributeTaskProcessPaymentSuccess(
    ctx,
    &worker.PaymentSuccessPayload{OutTradeNo: outTradeNo},
    asynq.MaxRetry(3),
    asynq.Queue(worker.QueueCritical),
)

// 处理：每个任务一个文件 task_*.go
func (processor *RedisTaskProcessor) ProcessTaskPaymentSuccess(
    ctx context.Context, task *asynq.Task,
) error {
    var payload PaymentSuccessPayload
    if err := json.Unmarshal(task.Payload(), &payload); err != nil {
        return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
    }
    // 处理逻辑...
    return nil
}
```

### 关键约束

- 任务必须**幂等**
- Payload 保持最小化，只存标识符（如 `out_trade_no`），不存大对象
- 队列优先级：`critical=10`, `default=5`
- 支持 `NoopTaskDistributor`（Redis 不可用时的降级）

### 定时调度器

通过 `scheduler.Manager` 统一管理：

```go
schedulerManager.Register("weather", weather.NewScheduler(...))
schedulerManager.Register("payment-recovery", worker.NewPaymentRecoveryScheduler(...))
schedulerManager.StartAll(ctx, waitGroup)
```

---

## IX. 配置管理

所有配置通过 `util.Config` 结构体统一管理，来源：`app.env` 文件 + 环境变量覆盖。

```go
type Config struct {
    Environment string        `mapstructure:"ENVIRONMENT"`
    DBSource    string        `mapstructure:"DB_SOURCE"`
    // ...每个字段必须有 mapstructure tag
}
```

新增配置项必须：

1. 在 `Config` 中添加字段 + `mapstructure` tag
2. 在 `LoadConfig()` 中设置 `viper.SetDefault()` 默认值
3. 在 `app.env.example` 中添加示例
4. 处理配置缺失的降级逻辑（参考 Redis/微信支付客户端的可选初始化）

---

## X. 依赖注入

### Server 结构体

`api.Server` 持有所有运行时依赖，通过构造函数 `NewServer(...)` 注入：

```go
type Server struct {
    config          util.Config
    store           db.Store              // 接口类型，支持 mock
    tokenMaker      token.Maker           // 接口类型
    wechatClient    wechat.WechatClient   // 接口类型
    paymentClient   wechat.PaymentClientInterface
    ecommerceClient wechat.EcommerceClientInterface
    mapClient       maps.TencentMapClientInterface
    taskDistributor worker.TaskDistributor // 接口类型
    wsHub           *websocket.Hub
    rulesEngine     rules.Engine
    // ...
}
```

### 关键设计

- 外部依赖均通过**接口类型**注入
- 可选依赖允许 `nil`（如 `paymentClient`, `ecommerceClient`），使用前需判空
- 测试使用 `Set*ForTest()` 方法注入 mock

---

## XI. 日志规范

使用 `zerolog`，结构化日志：

```go
log.Info().
    Int64("order_id", order.ID).
    Int64("user_id", userID).
    Str("status", order.Status).
    Msg("order created")

log.Error().
    Err(err).
    Str("request_id", GetRequestID(ctx)).
    Str("path", ctx.Request.URL.Path).
    Msg("internal error")
```

### 强制约束

- 关键路径必须带结构化字段：`user_id`, `merchant_id`, `order_id`, `request_id`
- **禁止**记录敏感信息：密钥、证照号码、手机号、API 密钥
- 开发环境：`ConsoleWriter`（彩色输出）
- 生产环境：JSON 格式

---

## XII. 测试规范

### Logic 层测试（主力）

使用 mock store + 表驱动测试：

```go
func TestCancelOrder_NotFound(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    store := mockdb.NewMockStore(ctrl)
    store.EXPECT().
        GetOrderForUpdate(gomock.Any(), int64(10)).
        Return(db.Order{}, db.ErrRecordNotFound)

    _, err := CancelOrder(context.Background(), store, CancelOrderInput{UserID: 1, OrderID: 10})
    reqErr := assertRequestError(t, err)
    require.Equal(t, 404, reqErr.Status)
}
```

### 生成 Mock

```bash
make sqlc   # 同时重新生成 SQLC + 所有 mock
make mock   # 仅重新生成 mock
```

Mock 生成目标：

- `db/mock/store.go` — Store 接口
- `worker/mock/distributor.go` — TaskDistributor 接口
- `wechat/mock/wechat_client.go` — WechatClient
- `wechat/mock/payment_client.go` — PaymentClientInterface,
  EcommerceClientInterface

---

## XIII. 构建与变更流程

### 新增 API

```
☐ 定义请求/响应结构体（含 JSON tag + binding tag + Swagger 注释）
☐ 在 Handler 中绑定参数、调用 Logic、转换响应
☐ 在 setupRouter() 注册路由
☐ 编写 Logic 层函数（如有业务逻辑）
☐ 编写 Logic 层单元测试
☐ 运行 make swagger 更新文档
```

### 新增数据库表

```
☐ make new_migration name=add_xxx_table （创建空迁移文件）
☐ 编写 UP/DOWN SQL
☐ 在 db/query/ 编写 SQLC 查询
☐ make sqlc（重新生成代码 + mock）
☐ 如需事务方法，在 db/sqlc/ 添加 tx_xxx.go
☐ 更新 Store 接口（在 store.go 中）
☐ 如有新的状态枚举，添加到 db/sqlc/constants.go
```

### 部署检查

```
☐ 环境变量已配置（参照 app.env.example）
☐ 数据库迁移已执行（或 AUTO_MIGRATE=true）
☐ 所有测试通过（make test）
☐ 日志级别为 INFO
☐ 敏感信息不泄露（密钥、证照等）
```

---

## XIV. 严格禁止

1. **硬编码业务数据** — 不要写死 ID、金额、状态值，使用常量
2. **空实现/TODO** — 不允许 `return nil` 占位或 `// TODO: implement`
3. **全局变量** — 使用依赖注入
4. **忽略错误** — 必须处理所有 `err != nil`
5. **直接返回 DB 模型** — 使用响应 DTO 结构体
6. **裸 SQL 字符串** — 使用 SQLC 生成的类型安全方法
7. **Handler 中写业务逻辑** — 业务逻辑必须在 `logic/` 层
8. **在事务内做外部调用** — 禁止事务中调用 HTTP/Redis/微信 API
9. **`any` 类型** — 使用 SQLC 自动生成的类型，请求/响应必须有明确结构
10. **魔法字符串** — 状态值使用 `db/sqlc/constants.go` 中定义的常量
11. **重复 pgtype 转换** — 使用 `order_response.go` 中的 helper，不要手写
    `.Valid { resp.X = &o.X }`

---

## XV. pgtype 转换 Helper

所有 pgtype → 指针的转换使用 `api/order_response.go` 中的统一 helper：

```go
pgtypeInt8Ptr(v pgtype.Int8) *int64
pgtypeInt4Ptr(v pgtype.Int4) *int32
pgtypeTextPtr(v pgtype.Text) *string
pgtypeTimestamptzPtr(v pgtype.Timestamptz) *time.Time
```

对于订单响应转换，使用 `orderNullableFields` 统一映射可选字段：

```go
orderNullableFields{
    AddressID: o.AddressID, DeliveryDistance: o.DeliveryDistance,
    // ... 所有可选字段
}.applyTo(&resp)
```

新增类似域（如 reservation、delivery 的响应转换）时，复用 `pgtypeXxxPtr`
helper， 如果字段超过 5 个且存在多变体，创建类似的 `xxxNullableFields` 结构体。

---

## XVI. Server 结构体演进

### 现状（已知技术债）

`Server` 持有 18 个字段，所有 Handler 是 `Server` 的方法，任何 Handler
都能访问所有依赖。 这是渐进开发中的自然结果，**现有代码不做大规模重构**。

### 新模块准则

对于新增的、独立性强的业务模块，**推荐**采用 Handler Group 模式：

```go
// 新文件 api/xxx_handlers.go
type XxxHandlers struct {
    store           db.Store
    taskDistributor worker.TaskDistributor
    // 仅注入该组需要的依赖
}

func NewXxxHandlers(store db.Store, td worker.TaskDistributor) *XxxHandlers {
    return &XxxHandlers{store: store, taskDistributor: td}
}

func (h *XxxHandlers) Create(ctx *gin.Context) { ... }
func (h *XxxHandlers) List(ctx *gin.Context)   { ... }
```

在 `setupRouter()` 中注册：

```go
xxxHandlers := NewXxxHandlers(server.store, server.taskDistributor)
xxxGroup := authGroup.Group("/xxx")
{
    xxxGroup.POST("", xxxHandlers.Create)
    xxxGroup.GET("", xxxHandlers.List)
}
```

### Handler 文件大小约束

单个 Handler 文件不超过 **500 行**。超过时按职责拆分：

- `xxx_create.go` — 创建相关
- `xxx_query.go` — 查询/列表
- `xxx_merchant.go` — 商户端操作
