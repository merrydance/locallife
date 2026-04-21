# LocalLife 后端工程一致性指南

> Go 代码组织、`context`、并发、接口与测试实践见 `.github/standards/backend/GO_PRACTICES.md`。
> SQL 编写、migration、索引与 sqlc 约定见 `.github/standards/backend/SQL_STANDARDS.md`。
> 本文件应作为“深实现契约参考”按需打开，而不是每次 backend 任务都默认通读的热路径文档。

## 架构概述

LocalLife 后端仍采用 HTTP 三层主线：`api/ -> logic/ -> db/sqlc/`，并通过 `worker/` 与 `scheduler/` 承载异步与恢复边界。

这一层的详细仓库级真实运行结构已经由以下文档正式拥有：

- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/backend/AGENT.md`
- `locallife/AGENTS.md`

本文件只保留那些在实现细节上仍然需要更深约束的内容。

---

## I. 分层职责与边界

这里仅保留实现层真正需要记住的边界摘要：

- `api/` 负责 transport：参数绑定、认证提取、错误映射、响应转换；不要放业务规则。
- `logic/` 负责业务流程和依赖编排，只接收标准 `context.Context`，不依赖 `gin.Context`。
- `db/sqlc/` 负责 SQL source、生成查询、事务方法与持久化常量；事务通过 `execTx` 模式统一管理。
- 模块应围绕业务能力高内聚组织，关键状态机、校验和写模型必须有明确拥有者；不要让多个包同时推进同一核心状态。
- 模块间依赖保持单向，优先通过最小能力接口协作；不要为了“复用”提前沉淀 `common/shared/helper` 业务抽象。
- 入口 DTO、sqlc 行模型、第三方 SDK 类型必须在边界处翻译，不能一路透传到业务层把边界污染掉。

这些规则的更完整版本由以下文档拥有：

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/backend/GO_PRACTICES.md`
- `.github/standards/backend/SQL_STANDARDS.md`

---

## II. 错误处理

### 三级错误体系

| 层级             | 函数                                                            | 用途                                |
| ---------------- | --------------------------------------------------------------- | ----------------------------------- |
| 客户端错误 (4xx) | `errorResponse(err)`                                            | 返回实际错误信息给客户端            |
| 业务错误 (4xx)   | `logic.NewRequestError(status, err)` → `writeLogicRequestError` | Logic 层→Handler 层的结构化错误传递 |
| 服务端错误 (5xx) | `internalError(ctx, err)`                                       | 日志记录实际错误，返回安全泛化信息  |

```text
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

```text
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
数据建模、字段类型、命名、migration 与 pgtype 约定由以下文档正式拥有：

- `.github/standards/backend/SQL_STANDARDS.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

本文件只在后文保留 LocalLife 特有、值得长期记忆的转换 helper 约束，而不再重复通用数据建模教程。

---

## IV. SQLC 查询规范
SQL source、query 风格、sqlc、事务模式、并发写语义和 SQL guard 全部以 `.github/standards/backend/SQL_STANDARDS.md` 为准。

本文件不再重复一份通用 SQLC 教程，以避免和 SQL 标准形成双份正文。

---

## V. 常量管理

### SSOT 层级

状态枚举等业务常量的唯一真实来源在 `db/sqlc/constants.go`：

```text
// db/sqlc/constants.go — SSOT
const (
    OrderStatusPending   = "pending"
    OrderStatusPaid      = "paid"
    OrderStatusCompleted = "completed"
    // ...
)
```

其他层应引用而非重定义。常量管理的日常执行规则已由 `.github/standards/backend/AGENT.md` 和 `.github/instructions/backend-locallife.instructions.md` 拥有；本节只保留 SSOT 位置本身这一深实现约束。

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

```text
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

### 关键约束

- 任务必须**幂等**
- Payload 保持最小化，只存标识符（如 `out_trade_no`），不存大对象
- 队列优先级：`critical=10`, `default=5`
- 支持 `NoopTaskDistributor`（Redis 不可用时的降级）

### 定时调度器

通过 `scheduler.Manager` 统一管理：

```text
schedulerManager.Register("weather", weather.NewScheduler(...))
schedulerManager.Register("payment-recovery", worker.NewPaymentRecoveryScheduler(...))
schedulerManager.StartAll(ctx, waitGroup)
```

---

## IX. 配置管理

所有配置通过 `util.Config` 结构体统一管理，来源：`app.env` 文件 + 环境变量覆盖。

```text
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
依赖注入的通用规则由 `.github/standards/backend/AGENT.md` 和 `.github/standards/backend/GO_PRACTICES.md` 正式拥有。

本文件只保留和当前 `api.Server` 演进历史直接相关的内容，见第 XVI 节。

---

## XI. 日志规范
结构化日志、敏感字段约束和 Go guard 已由 `.github/standards/backend/AGENT.md` 与 `.github/standards/backend/GO_PRACTICES.md` 正式拥有。本文件不再重复日志风格示例。

---

## XII. 测试规范
测试实践、mock 生成和验证矩阵已经由以下文档正式拥有：

- `.github/standards/backend/GO_PRACTICES.md`
- `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`

本文件不再重复测试教程与命令说明。

---

## XIII. 构建与变更流程
构建、生成、验证和交付流程由 `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`、`.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md` 与 `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md` 正式拥有。

---

## XIV. 严格禁止
“严格禁止”类底线现在以 `.github/standards/backend/AGENT.md` 为唯一权威来源。本文件不再维护第二份禁止事项正文，避免和硬约束文档形成双份真相。

---

## XV. pgtype 转换 Helper

所有 pgtype → 指针的转换使用 `api/order_response.go` 中的统一 helper：

```text
pgtypeInt8Ptr(v pgtype.Int8) *int64
pgtypeInt4Ptr(v pgtype.Int4) *int32
pgtypeTextPtr(v pgtype.Text) *string
pgtypeTimestamptzPtr(v pgtype.Timestamptz) *time.Time
```

对于订单响应转换，使用 `orderNullableFields` 统一映射可选字段：

```text
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

```text
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

```text
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
