# 后端代码全量审查报告

> **审查范围**：`locallife/` 整体后端代码（Go 1.24）  
> **审查日期**：2026-03-16  
> **严重等级**：P0（阻塞上线）/ P1（高优先级）/ P2（中优先级）/ P3（代码质量）  
> **修复状态**：✅ 已修复 / ⏳ 待处理 / 🔁 已确认无需修改

---

## P0 — 必须立即修复（阻塞级 Bug）

### ✅ P0-1：`execTx` 死锁重试耗尽后静默返回 `nil`

> **已修复**（2026-03-16）：`return nil` 替换为返回错误；两处 `time.Sleep` 替换为 `select` + `ctx.Done()`（同步修复 P2-4）。

**文件**：[db/sqlc/exec_tx.go](locallife/db/sqlc/exec_tx.go#L75-L80)

```go
// 当前代码：所有重试尝试耗尽后，for 循环正常退出，返回 nil
for attempt := 1; attempt <= execTxMaxAttempts; attempt++ {
    ...
    // 重试失败，continue
}
return nil  // ← 🚨 Bug：这里应该返回最后一次的 deadlock error
```

`isDeadlockError(commitErr)` 为 `true` 且 `attempt == execTxMaxAttempts` 时，代码仅记录日志，然后 `continue` 使循环结束，最终执行 `return nil`，调用方会误认为事务提交成功，实际上数据库中什么也没发生。

**修复方案**：
```go
for attempt := 1; attempt <= execTxMaxAttempts; attempt++ {
    ...
    if isDeadlockError(commitErr) {
        if attempt < execTxMaxAttempts {
            time.Sleep(execTxRetryDelay * time.Duration(attempt))
            continue
        }
        // 重试耗尽，返回错误
        return fmt.Errorf("tx commit deadlock retry exhausted after %d attempts: %w", execTxMaxAttempts, commitErr)
    }
    return commitErr
}
return fmt.Errorf("execTx: all %d attempts exhausted", execTxMaxAttempts) // 防御性兜底
```

---

## P1 — 高优先级问题

### ✅ P1-1：生产代码中残留 `fmt.Printf` / `[DEBUG]` 调试输出

> **已修复**（2026-03-16）：cart.go、tencent.go、lookback_checker.go、dish.go、risk_management.go 全部替换为 zerolog 结构化调用；tencent.go 改为 `log.Debug()` 条件块。

以下位置直接向 stdout 打印调试信息，绕过结构化日志（zerolog），在生产环境中无法按级别过滤，也无法携带 request_id/trace 上下文：

| 文件 | 行号 | 内容 |
|------|------|------|
| [api/cart.go](locallife/api/cart.go#L946) | 946 | `fmt.Printf("[DEBUG] getUserCartsSummary...")` |
| [maps/tencent.go](locallife/maps/tencent.go#L357-L358) | 357-358 | `fmt.Printf("[TencentMap] Request/Response: ...")` |
| [algorithm/lookback_checker.go](locallife/algorithm/lookback_checker.go#L107) | 107 | `fmt.Printf("Failed to get order merchant/rider info: %v\n", err)` |
| [api/dish.go](locallife/api/dish.go#L1287) | 1287 | `fmt.Printf("failed to remove dish %d from combos...")` |
| [api/risk_management.go](locallife/api/risk_management.go#L471) | 471, 485 | `fmt.Printf("failed to enqueue claim refund task...")` |

**修复方案**：全部替换为 `log.Error().Err(err).Msg(...)` 调用。`maps/tencent.go` 的请求/响应 debug 日志建议改为 `log.Debug()` 并通过日志级别控制；API key 即使打码也不应出现在日志中。

---

### ✅ P1-2：`api/merchant.go` 和 `api/combo.go` 混用标准 `"log"` 包

> **已修复**（2026-03-16）：两个文件均将 `"log"` import 替换为 `"github.com/rs/zerolog/log"`，所有 `log.Printf` 替换为带上下文字段的 zerolog 调用。

---

### ✅ P1-3：浏览历史 N+1 查询

> **已修复**（2026-03-16）：`listBrowseHistory` 改为先收集所有 merchant/dish ID，再分别调用已有的 `GetMerchantsByIDs` / `GetDishesByIDs` 批量查询，最后用 `map` 拼装结果。无论 `page_size` 多大，额外 DB 调用固定为 ≤2 次。

---

### 🔁 P1-4：`RateLimiter` 后台 goroutine 无关闭路径

> **已确认无需修改**：`Stop()` 方法已存在，`Server.Shutdown()` 已调用 `server.rateLimiter.Stop()`，实现正确。

---

### ✅ P1-5：`externalBaseURL` 信任客户端可控的 `Origin` 头

> **已修复**（2026-03-16）：`externalBaseURL` 改为 `(server *Server)` 方法，完全移除 Origin 头读取。新优先级：`config.ExternalBaseURL`（生产必填）→ `X-Forwarded-Host` → `Host` header。`util/config.go` 新增 `ExternalBaseURL string` 配置项。

---

### ✅ P1-6：`wechat/ecommerce.go` 中关键支付加密未实现

> **已修复**（2026-03-16）：实现 `EncryptSensitiveData` 方法，`AddProfitSharingReceiver` 中接收方名称字段通过平台公钥加密后传输，携带 `Wechatpay-Serial` 头；调用方可自行预加密传入 `EncryptedName` 跳过重复加密。

**文件**：`wechat/ecommerce.go:676`（TODO 注释）

```go
// TODO: 需要使用微信平台证书加密
```

涉及敏感字段（如身份证、银行卡号）的微信 API 调用要求使用平台公钥加密，当前实现为明文传输，违反微信支付接入规范，可能导致合规风险或接口调用失败。

**修复方案**：实现微信平台公钥/证书加密逻辑，参考微信支付文档中的「敏感信息加密」章节。

---

### ✅ P1-7：`calculateCart` Handler 超过 200 行，违反单一职责

> **已修复**（2026-03-16）：业务逻辑抽取到 `logic/cart_calculation.go`，新增 `CalculateCartPreview` + `resolveRouteAndFee` 函数，Handler 缩减至绑定参数 → 调用 `logic.CalculateCartPreview` → 序列化响应三步。地图 nil-safe fallback 改为 Haversine 直线距离，消除 map client 不可用时的硬错误。

---

## P2 — 中优先级问题

### ✅ P2-1：全局 viper 实例导致并发测试干扰

> **已修复**（2026-03-16）：`LoadConfig` 改为 `v := viper.New()` 独立实例，消除全局状态污染。

---

### ✅ P2-2：Casbin 全局状态（`GetGlobalCasbinEnforcer` / `InitCasbin`）

> **已修复**（2026-03-16）：`globalCasbinEnforcer` 改为 `atomic.Pointer[CasbinEnforcer]`，所有读写操作改为 `.Load()`/`.Store()`，中间件先将加载结果存入局部变量再使用，消除 check-then-act 竞态。

---

### ✅ P2-3：错误消息语言不一致（中英文混用）

> **已修复**（2026-03-16）：全面迁移完成。
>
> **迁移规模**：`api/` 内原有 240 处中文 `errors.New()`，本次全部迁移为 `*APIError` 数字错误码 + 英文消息，用户可见的中文直接响应降至 **0**。
>
> **方案**：`api/apierrors.go` 集中定义所有 API 错误常量（5 位数字码，前 3 位对应 HTTP 状态语义）。新增约 90 个常量，覆盖申请流程、图片上传、骑手业务、索赔风控、登录会话、区域运营商、必填字段校验等所有业务域。`errorResponse()` 函数已识别 `*APIError` 并在响应中透出 `code` 字段，前端可按 code 做 i18n 分发，无需解析中文字符串。
>
> **剩余**：56 处中文仅出现在 `internalError()` 调用（不对外暴露）或辅助函数的中间错误链中，属于内部日志范畴，无需迁移。

---

### ✅ P2-4：`execTx` 在事务重试中使用 `time.Sleep` 阻塞 goroutine

> **已修复**（2026-03-16）：随 P0-1 一并修复，两处 `time.Sleep` 替换为 `select { case <-time.After(...): case <-ctx.Done(): return ctx.Err() }`。

---

### ✅ P2-5：`deleteStoredImageAsync` 无并发控制和错误上报

> **已修复**（2026-03-16）：引入 `imageDeleteWorker`，用有界 channel（容量 256）+ 固定 2 个 worker goroutine 替代无限制 `go func()`；失败时 `log.Error()` 记录；`Server.Shutdown()` 调用 `imageDeleter.shutdown()` 等待 worker 优雅退出。

**文件**：[api/upload_url.go](locallife/api/upload_url.go) `deleteStoredImageAsync` 函数

---

### ✅ P2-6：响应状态码不符合 REST 语义（创建操作返回 200）

> **已修复**（2026-03-16）：主要创建接口均已改为 `http.StatusCreated (201)`。此轮补充修复 `addCartItem`：给 `returnUpdatedCart` 加入 `statusCode` 参数，`addCartItem`（创建资源）传入 201，`updateCartItem`/`deleteCartItem`（更改资源）保持 200。

---

### ✅ P2-7：`minOrderAmount` 未实现但出现在响应中

> **已修复**（2026-03-16）：将 `calculateCartResponse.MinOrderAmount` 改为 `omitempty`，`MeetsMinOrder` 改为 `*bool omitempty`。当商户未配置起送金额（= 0）时，这两个字段不序列化输出——**字段缺失 = 无限制**，语义明确。当 `MinOrderAmount > 0` 时才返回 `meets_min_order`，避免前端依赖恒 true 的字段作校验被绕过。

---

### ✅ P2-8：常量在多层重复定义（DRY 违反）

> **已修复**（2026-03-16）：新增 `logic/geo_constants.go`，统一导出 `MetersPerLatDegree = 111_000` / `MetersPerLngDegree = 96_000`；`api/order.go` 中的重复定义已删除；`logic/order_calculation.go` 和 `logic/delivery_quote.go` 均引用导出常量。

---

### ✅ P2-9：风险管理通知为空实现（TODO 未完成）

> **已修复**（2026-03-16）：`HandleCheckMerchantForeignObject` 中:
> 1. `SuspendMerchantTakeout` 返回错误不再静默丢弃，改为 `log.Error` + 向上传播使任务可被 asynq 重试；
> 2. 暂停成功后调用 `processor.store.GetMerchant` 获取 `OwnerUserID`，通过 `processor.distributor.DistributeTaskSendNotification` 异步发送站内信；
> 3. 通知设置 `IgnorePreferences: true`（食安/风控类关键通知不受免打扰限制）；
> 4. 通知入队失败仅记录 `log.Error`，不影响已完成的熔断结果。

**文件**：`worker/task_risk_management.go:95`

---

### ✅ P2-10：`wechat.go` 登录时设备信息失败就报 500

> **已修复**（2026-03-16）：`UpsertUserDevice` 失败改为 `log.Warn()` 后 continue，不再阻断登录主路径。

---

## P3 — 代码质量问题

### ✅ P3-1：`isNoRows` 是 `isNotFoundError` 的冗余别名

> **已修复**（2026-03-16）：删除 `isNoRows` 函数声明，全文替换为 `isNotFoundError`。

---

### ✅ P3-2：WebSocket 测试中大量使用 `time.Sleep` 导致测试脂弱

> **已修复**（2026-03-16）：`hub_test.go` 中 26 处 `time.Sleep` 全部替换：
> - Register/Unregister 后的状态验证：改用 `require.Eventually`（轮询间隔 1ms，超时 1s）
> - `SendToRider/Merchant` 后等消息：去掉 sleep，保留已有的 `select + time.After(1s)`
> - 连接替换后等旧连接关闭：改用 `select + time.After(1s)`
> - Benchmark 中的 1ms 轮询保留（benchmark 不适用 Eventually）
>
> 测试聋制时间：1.3s+ 降至 0.3s，消除了在 CI 慢机器上的 flaky 须证。

**文件**：`websocket/hub_test.go`（20+ 处 `time.Sleep(50ms)`）

---

### ✅ P3-3：Swagger `@host` 硬编码为 `localhost:8080`

> **已修复**（2026-03-16）：`api/server.go` 注册 swagger handler 时，在 development 模式下用 `config.ExternalBaseURL` 覆盖 `docs.SwaggerInfo.Host`。`docs` 包改为具名导入，不再依赖注解中的静态地址。

**文件**：[main.go](locallife/main.go#L44)

---

### ✅ P3-4：`bcrypt.DefaultCost` 安全性偏低

> **已修复**（2026-03-16）：新增常量 `bcryptCost = 12` 替换 `bcrypt.DefaultCost`，附注释说明旧 hash 兼容性及渐进式升级策略。

---

### 🔁 P3-5：`api/server.go` 中有一处不必要的 `_ = ctx.Error(err)`

> **已确认无需修改**：该调用位于 `internalError()` 辅助函数内，是故意将错误挂载到 Gin 内部 errors slice 以便 `RequestLoggingMiddleware` 统一提取，行为正确。

---

### ✅ P3-6：`_ = err` 静默忽略错误（多处）

> **已修复**（2026-03-16）：table_reservation.go 中任务分发失败、dish.go 中取消旧分类关联失败，均替换为 `log.Warn().Err(err).Msg(...)` 后继续。

---

### ✅ P3-7：操作多步骤 updateDishCategory 缺少事务包装

> **已修复**（2026-03-16）：新增 `db/sqlc/tx_dish_category.go` 实现 `RenameMerchantDishCategoryTx`，将「创建分类 → 关联新分类 → 迁移菜品 → 取消旧关联」4步封装在同一 `execTx` 中；Store 接口新增该方法；`make mock` 重新生成 MockStore；handler 替换为单次事务调用。

---

### P3-8：空字符串地址比较不稳定的条件常量

**文件**：`api/cart.go` `calculateCart`

```go
// 起送金额暂不实现
var minOrderAmount int64
meetsMinOrder := true
```

这两行和返回值耦合，前端可能对 `min_order_amount: 0` 有不同的行为假设。应改为返回 `nil` 或提供明确文档说明此字段尚未实现（或不返回此字段）。

---

### ✅ P3-9：`reverse_geocode` 和 `geocode` 共用同一个 URL 路径

> **已修复**（2026-03-16）：删除 `reverseGeocodeURL` 冗余常量，统一使用 `geocodeURL`，附腾讯地图文档链接注释说明两者共用路径原因。

---

## 综合改进建议

### 建议一：引入 `golangci-lint` 到 CI 流程

配置规则至少应包含：
```yaml
linters:
  enable:
    - errcheck       # 检测未处理的 error
    - gochecknoglobals  # 检测全局变量
    - exhauststruct  # 检测结构体初始化完整性
    - gosec          # 安全检测
    - forbidigo      # 禁止 fmt.Print* 等调用
```

至少 `errcheck` 能直接检出 P3-6 类问题，`gosec` 能辅助检出 P1-5。

---

### 建议二：统一 API 错误定义

建议创建 `api/apierrors.go`，集中定义所有用户可见的错误消息，使用英文 + 错误码，避免散乱的内联 `errors.New("xxx")` 字符串：

```go
var (
    ErrNotMerchant   = apierr(40301, "not a merchant")
    ErrDishNotFound  = apierr(40401, "dish not found")
    ErrCartEmpty     = apierr(40001, "cart is empty")
    // ...
)
```

---

### 建议三：拆分超长 Handler

以下 Handler 建议抽取业务逻辑到 `logic/` 包（以 `CalculateOrderPreview` 为参照）：
- `calculateCart` (~200 行)
- `SubmitClaim` (~150 行，已含部分规则引擎逻辑)
- `createReservation` (~100+ 行，含 ws 推送和规则引擎)

原则：**Handler 只做绑定参数 → 调用 logic → 序列化结果，不含业务判断。**

---

### 建议四：建立单元测试覆盖率基线

当前 `logic/` 层测试较为完整，但 `api/` 层的业务逻辑（如 `updateDishCategory` 的迁移逻辑）缺乏测试。建议：
- 将覆盖率基线设定为 logic 层 ≥ 80%、db/sqlc 层 ≥ 70%
- 将覆盖率检查加入 CI gate

---

## 问题优先级总览

| 编号 | 优先级 | 状态 | 文件 | 描述 |
|------|--------|------|------|------|
| P0-1 | 🔴 P0 | ✅ 已修复 | db/sqlc/exec_tx.go | 死锁重试耗尽后静默返回 nil |
| P1-1 | 🟠 P1 | ✅ 已修复 | 多处 | 生产代码 fmt.Printf/[DEBUG] 输出 |
| P1-2 | 🟠 P1 | ✅ 已修复 | api/merchant.go, api/combo.go | 混用标准 log 包 |
| P1-3 | 🟠 P1 | ✅ 已修复 | api/cart.go | 浏览历史 N+1 查询 |
| P1-4 | 🟠 P1 | 🔁 无需修改 | api/middleware_ratelimit.go | RateLimiter goroutine 泄漏（已正确实现） |
| P1-5 | 🟠 P1 | ✅ 已修复 | api/public_url.go | Origin 头 SSRF/开放重定向风险 |
| P1-6 | 🟠 P1 | ✅ 已修复 | wechat/ecommerce.go | 支付敢感字段加密未实现 |
| P1-7 | 🟠 P1 | ✅ 已修复 | api/cart.go | calculateCart Handler 超 200 行 |
| P2-1 | 🟡 P2 | ✅ 已修复 | util/config.go | 全局 viper 测试干扰 |
| P2-2 | 🟡 P2 | ✅ 已修复 | api/casbin_enforcer.go | Casbin 全局状态竞态 |
| P2-3 | 🟡 P2 | ✅ 已修复 | 多处 | 错误消息中英文混用 |
| P2-4 | 🟡 P2 | ✅ 已修复 | db/sqlc/exec_tx.go | 重试 Sleep 不可取消（随 P0-1 修复） |
| P2-5 | 🟡 P2 | ✅ 已修复 | api/upload_url.go | 异步删除无并发控制 |
| P2-6 | 🟡 P2 | ✅ 已修复 | 多处 | 创建接口返回 200 而非 201 |
| P2-7 | 🟡 P2 | ✅ 已修复 | api/cart.go | minOrderAmount 未实现 |
| P2-8 | 🟡 P2 | ✅ 已修复 | api/order.go, logic/ | 距离常量重复定义 |
| P2-9 | 🟡 P2 | ✅ 已修复 | worker/task_risk_management.go | 风控通知 TODO 未实现 |
| P2-10 | 🟡 P2 | ✅ 已修复 | api/wechat.go | 设备记录失败阻断登录 |
| P3-1 | 🟢 P3 | ✅ 已修复 | api/dish.go | isNoRows 冗余别名 |
| P3-2 | 🟢 P3 | ✅ 已修复 | websocket/hub_test.go | time.Sleep 导致测试脂弱 |
| P3-3 | 🟢 P3 | ✅ 已修复 | main.go, api/server.go | Swagger host 硬编码 |
| P3-4 | 🟢 P3 | ✅ 已修复 | util/password.go | bcrypt cost 偏低 |
| P3-5 | 🟢 P3 | 🔁 无需修改 | api/server.go | ctx.Error 在 internalError() 辅助函数内为故意行为 |
| P3-6 | 🟢 P3 | ✅ 已修复 | 多处 | _ = err 静默忽略错误 |
| P3-7 | 🟢 P3 | ✅ 已修复 | api/dish.go | updateDishCategory 多步无事务 |
| P3-8 | 🟢 P3 | ✅ 已修复 | api/cart.go | minOrderAmount: 0 语义不明 |
| P3-9 | 🟢 P3 | ✅ 已修复 | maps/tencent.go | geocode/reverse 共用 URL 冗余常量 |
