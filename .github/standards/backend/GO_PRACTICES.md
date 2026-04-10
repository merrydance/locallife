# Backend Go Practices

本文件定义 LocalLife Go 后端在代码组织、接口抽象、`context`、并发、错误包装、测试与基础质量门禁上的长期实践约定。

适用范围：

- `locallife/api/`
- `locallife/logic/`
- `locallife/db/sqlc/`
- `locallife/worker/`
- `locallife/scheduler/`
- `locallife/cmd/`

与其他后端标准的关系：

- 不可违反的底线约束：看 `.github/standards/backend/AGENT.md`
- 分层职责与主路径实现：看 `.github/standards/backend/SYSTEM_PROMPT.md`
- HTTP 错误与日志语义：看 `.github/standards/backend/ERROR_HANDLING.md`
- SQL、migration 与 sqlc 规则：看 `.github/standards/backend/SQL_STANDARDS.md`

## 1. 代码质量基线

- 所有 Go 代码默认满足 `gofmt` 与 import 有序整理要求，不保留手工对齐、无用 import 或非标准格式。
- 不把“能编译”当作质量门槛。新增或修改 Go 代码时，至少保证最小相关测试可运行；并发敏感路径要额外考虑 race 风险。
- 不依赖 reviewer 肉眼兜底基础问题。格式、静态错误、无效代码、明显阴影变量和无用分支应在本地先清理，而不是等评审指出。

## 2. Package 与接口设计

### 2.1 package 约定

- package 名保持简短、稳定、语义明确，避免 `util2`、`commonhelper`、`misc` 这一类无边界名称。
- 不为一次性复用提前抽“公共包”；先看当前领域内是否已有稳定边界。
- 避免把领域逻辑塞进泛化 helper 包里，导致真正依赖方向变得不透明。

### 2.2 接口设计

- 接口用于表达真实边界，不用于机械性地把每个实现都包一层抽象。
- 优先最小接口；不要为了 mock 方便把一个实现体的全部方法抬升成大接口。
- 接口应尽量由消费方需要来驱动，而不是让提供方暴露一整面能力再让所有调用方一起耦合进去。
- 构造函数默认返回具体类型；只有跨 package 边界确实需要隐藏实现或收窄依赖面时，再返回接口。
- 不要把 `context.Context`、logger、配置对象或 request-scope 数据存进长期存活的接口实现里当隐式全局状态。

## 3. `context` 规范

- `context.Context` 作为核心函数第一个参数传入，不传 `nil`。
- 不把 `context.Context` 存到 struct 字段中；它属于调用链，而不是对象长期状态。
- 不在库函数内部擅自改用 `context.Background()` 覆盖上游取消语义；只有进程入口、测试或非常明确的脱链场景才使用 `Background()`。
- 创建 `context.WithTimeout`、`context.WithCancel` 或 `context.WithDeadline` 的代码，负责 `defer cancel()`，除非控制流明确把取消责任交给调用方。
- 超时策略应靠近外部边界，例如 DB、Redis、HTTP client、第三方 SDK、worker 处理超时，而不是在业务深处随意套新的 timeout。

## 4. 并发与 goroutine 生命周期

- 多个相关 goroutine 的生命周期统一由调用方显式管理，优先用 `errgroup` 或清晰的 owner 模式，而不是散落 `go func()`。
- request path 中禁止无说明的 fire-and-forget goroutine。若后台执行是业务需要，应该显式落到 worker、scheduler、outbox 或其他可观测边界。
- channel 的关闭责任归发送方或 owner，不能由不拥有生命周期的一侧随意 close。
- 共享可变状态必须有明确同步策略；如果正确性依赖并发访问顺序，就不要把它藏在偶然的调用时序里。
- 涉及 goroutine、channel、锁、重试或重复投递语义的变更，至少做一次 focused review 或测试说明，不把并发正确性当默认成立。

## 5. 错误处理与返回值

- 业务失败与基础设施失败分开表达：业务失败走 `logic.NewRequestError(...)`，异常失败走 `fmt.Errorf("context: %w", err)` 并由上层统一记录。
- 对需要分支判断的错误使用 `errors.Is` / `errors.As`，不要靠字符串匹配错误文本。
- 不要返回信息不足的裸错误；包装时给出能定位失败层级的上下文。
- 不在正常请求路径里用 `panic` 代替错误传播；panic 只用于真正的不可恢复程序错误。
- 避免“先 log 再 return 同一个 error”造成重复日志，具体模式遵循 `.github/standards/backend/ERROR_HANDLING.md`。

## 6. 禁止写法与推荐写法

以下清单把高频 Go 反模式与推荐模式显式写出来。能自动门禁的，会在本节末尾说明；其余至少应作为实现与 review 的固定检查项。

| 场景 | 禁止写法 | 推荐写法 |
| --- | --- | --- |
| 非结构化日志 | `fmt.Println("payment callback failed", err)` | `log.Error().Err(err).Str("out_trade_no", outTradeNo).Msg("payment callback failed")` |
| 覆盖上游 context | `store.GetOrder(context.Background(), orderID)` | `store.GetOrder(ctx, orderID)` |
| 把 context 存进 struct | `type Service struct { ctx context.Context }` | `type Service struct { store db.Store } // ctx 由每次调用传入` |
| request path fire-and-forget | `go func() { _ = doAsyncWork(ctx, orderID) }()` | `err = distributor.DistributeTaskProcessPaymentSuccess(ctx, payload, ...)` |
| 业务错误用 panic | `if order == nil { panic("order missing") }` | `if order == nil { return logic.NewRequestError(http.StatusNotFound, errors.New("order not found")) }` |
| 字符串判断错误 | `if strings.Contains(err.Error(), "not found") { ... }` | `if errors.Is(err, db.ErrRecordNotFound) { ... }` |

额外说明：

- `fmt.Print*` / `log.Print*` 在后端请求和任务路径里会破坏结构化日志、过滤与审计字段。
- 普通调用链中的 `context.Background()` 会切断取消、超时和 tracing 语义；只有进程入口、明确脱链任务或特殊测试场景才允许。
- 把 `context.Context` 挂到 struct 上，会把调用链状态变成隐式全局，破坏可推理性。
- request path 中直接起 goroutine，等于把生命周期、幂等、失败观测和重试语义藏掉。
- 业务分支用 `panic` 会把可恢复错误错误升级成进程级异常。
- 字符串判断错误文本会把控制流绑定到不稳定文案，失去类型和哨兵错误的可维护性。

### 6.1 当前轻量 Go Guard 范围

- 当前 changed-file Go guard 只检查本次 diff 新增的非测试 Go 代码，不重扫历史债务。
- 当前 guard 只拦截三类高信噪比问题：
  - 新增 `fmt.Print*` 或 `log.Print*` 非结构化输出。
  - 新增 `context.Background()`。
  - 新增 `panic(...)`。
- 如果确有合理例外，允许在同一行内显式说明：
  - `goguard: allow-unstructured-log`
  - `goguard: allow-background-context`
  - `goguard: allow-panic`
- `goguard:` 例外注释必须在同一行内写出具体理由，至少说明：为什么默认规则在这里不适用、为什么当前边界仍然安全、以及为什么这不是可以复制到普通业务路径的通用模式。
- 禁止只写裸标记，例如 `// goguard: allow-panic` 却没有任何解释；这种写法应视为门禁绕过尝试。
- 在支付、退款、分账、callback、worker/recovery、订单/履约/预订/库存状态机等高风险路径里，`goguard:` 例外默认按高风险 review hotspot 处理。
- `context` 存 struct、无管理 goroutine、字符串判断错误等仍主要依赖实现和 review 依据本标准收口，因为它们更依赖上下文、误报成本更高。

## 7. 测试实践

- Logic 层默认采用表驱动测试，覆盖成功分支、失败分支、状态迁移与边界条件。
- 单元测试优先 deterministic：时间、随机数、外部依赖、ID 生成等如果会影响断言，应通过注入或固定输入控制。
- 不在 unit test 中依赖长时间 `sleep` 等待异步结果；优先通过可控同步点、mock 或显式轮询条件完成断言。
- mock 只覆盖当前用例真正依赖的交互，不把整条执行链的每个方法都机械性 expect 一遍。
- 并发敏感改动、重试逻辑、状态机推进和 callback/worker 路径，除了正常路径，还要补 failure path 或 duplicate path 的断言说明。

## 8. 评审关注点

评审 Go 后端变更时，优先检查以下问题：

- 是否为了 mock 或抽象便利引入了过大的接口面。
- 是否有 `context.Background()`、缺失 `cancel()`、或把 `context` 存进 struct 的错误用法。
- 是否出现无生命周期管理的 goroutine、模糊的 channel close 责任或未解释的共享状态。
- 是否用字符串判断错误、重复日志、或把内部错误语义错误地下沉到业务分支。
- 测试是否真的覆盖到新增分支，而不是只把主路径跑通。

## 9. 高并发业务路径规则

以下规则针对 LocalLife 这类高并发外卖/餐饮管理后端，优先级不低于通用 Go 风格规则。

### 9.1 事务拥有权与外部 I/O

- 事务负责 durable state transition；禁止在 `db/sqlc/tx_*.go`、`execTx` 闭包或其直接持锁路径里做 HTTP、WeChat、地图、OSS、OCR、WebSocket、Redis 副作用等外部 I/O。
- 如果一个业务动作既要更新本地状态，又要触发外部副作用，默认顺序是：先写 durable anchor / 主记录状态，再在事务外 enqueue、emit、notify 或调用第三方。
- 不要把“事务外补一刀”当成并发正确性的默认兜底；如果核心不变量只能靠事务外代码维持，优先重构到事务或数据库约束层。

### 9.2 真值来源与进程内状态

- 不要用进程内 `map`、`sync.Mutex`、本地布尔标记或单实例调用顺序来保证跨请求、跨 worker、跨 callback 的业务正确性。
- Redis、WebSocket 推送、本地缓存和内存投影只能做性能或体验优化，不能作为订单、支付、退款、配送、预订、库存等主状态真值来源。
- 当一条规则需要在多实例下成立时，默认把真值落在数据库状态、唯一约束、条件更新、lease 或幂等键上。

### 9.3 重试、重复执行与补偿

- worker、scheduler、callback、payment/refund integration 的重试必须显式定义：可重试错误、不可重试错误、最大重试次数、退避策略以及重试前提下的幂等保证。
- 不要把所有错误都机械重试，也不要在没有 durable anchor 或幂等前提时重复执行可能产生资金、副作用或状态推进的逻辑。
- 若一个路径依赖补偿、recovery 或人工介入成立，交付时必须明确主路径失败后如何收敛，而不是把补偿链路当隐式背景噪音。

### 9.4 Bounded fan-out

- request path 中不要无界地按订单项、商户、骑手、店铺或列表行逐个打 DB / 第三方请求；高并发热路径优先考虑批量查询、预聚合、异步投影或受控并发。
- 若一个 handler / logic 方法必须 fan-out，必须明确并发上限、超时、部分失败策略以及对下游的背压影响。
- 不要把“现在数据量不大”当作热路径 fan-out 的长期许可。

### 9.5 测试与冲突表达

- 订单、支付、退款、配送、预订、库存等高风险逻辑新增分支时，除了 happy path，至少补一个 conflict / duplicate / stale-state / retry 方向的断言说明。
- 当条件更新、claim 或状态迁移命中 0 行时，优先返回 typed conflict / request error，而不是当成成功 no-op 吞掉。

## 10. 本地实践默认值

- 修改 Go 文件后，先做格式化与 import 整理，再运行最小相关验证。
- 纯逻辑改动优先跑 focused unit tests；数据库行为、并发语义或 worker 路径改动再补充更高层验证。
- 当变更明显涉及并发正确性时，补充说明是否做过 focused race 检查或为什么当前无需额外 race 验证。