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

## 6. 测试实践

- Logic 层默认采用表驱动测试，覆盖成功分支、失败分支、状态迁移与边界条件。
- 单元测试优先 deterministic：时间、随机数、外部依赖、ID 生成等如果会影响断言，应通过注入或固定输入控制。
- 不在 unit test 中依赖长时间 `sleep` 等待异步结果；优先通过可控同步点、mock 或显式轮询条件完成断言。
- mock 只覆盖当前用例真正依赖的交互，不把整条执行链的每个方法都机械性 expect 一遍。
- 并发敏感改动、重试逻辑、状态机推进和 callback/worker 路径，除了正常路径，还要补 failure path 或 duplicate path 的断言说明。

## 7. 评审关注点

评审 Go 后端变更时，优先检查以下问题：

- 是否为了 mock 或抽象便利引入了过大的接口面。
- 是否有 `context.Background()`、缺失 `cancel()`、或把 `context` 存进 struct 的错误用法。
- 是否出现无生命周期管理的 goroutine、模糊的 channel close 责任或未解释的共享状态。
- 是否用字符串判断错误、重复日志、或把内部错误语义错误地下沉到业务分支。
- 测试是否真的覆盖到新增分支，而不是只把主路径跑通。

## 8. 本地实践默认值

- 修改 Go 文件后，先做格式化与 import 整理，再运行最小相关验证。
- 纯逻辑改动优先跑 focused unit tests；数据库行为、并发语义或 worker 路径改动再补充更高层验证。
- 当变更明显涉及并发正确性时，补充说明是否做过 focused race 检查或为什么当前无需额外 race 验证。