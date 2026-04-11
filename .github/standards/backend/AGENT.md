# Go 后端硬约束

> 本文件定义 **不可违反的工程约束**。项目架构、分层、错误处理等实现细节见
> `.github/standards/backend/SYSTEM_PROMPT.md`。
> SQL 编写、migration、索引与 sqlc 约定见 `.github/standards/backend/SQL_STANDARDS.md`。
> Go 代码组织、`context`、并发、接口与测试实践见 `.github/standards/backend/GO_PRACTICES.md`。
> 后端目录总入口见 `.github/standards/backend/README.md`。
> 仓库级高风险链路与已知失效模式见 `.github/standards/backend/BACKEND_RISK_MAP.md`。
> 当前仓库真实运行结构与生成触点见 `.github/standards/backend/RUNTIME_ARCHITECTURE.md`。
> 常用命令、再生成规则和验证工作流见 `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`。
> 交付与评审收口清单见 `.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md` 与 `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`。
> 正式审查如何形成 durable knowledge 见 `.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`。

## 1. 依赖注入

- 所有依赖（Database, Redis, 外部客户端）通过 **构造函数** 显式注入。
- 模块间依赖通过 **Interface** 定义，禁止依赖具体实现结构体。
- **禁止** 使用包级全局变量存储配置、DB 连接、Logger 等运行时状态。

## 2. 数据访问

- 使用 **pgx** (驱动) + **sqlc** (类型安全代码生成)。禁止全功能 ORM（如 GORM）。
- 事务通过 `execTx` 闭包模式统一管理，禁止裸 `Begin/Commit`。
- 数据库连接、Redis 地址等通过 `util.Config` 结构体加载，禁止硬编码。

## 3. API 规范

- 所有 API 路由包含版本号前缀：`/v1/`。
- 使用 Swagger/OpenAPI 注释，通过 `swag init` 自动生成文档（仅开发环境挂载）。
- 请求体使用 `binding` tag (`go-playground/validator`) 进行字段级校验。

## 4. 安全中间件

- **CORS**：配置显式白名单，禁止设置为 `*`。
- **Recovery**：全局 Panic 捕获。
- **Rate Limiting**：基于令牌桶的限流，敏感接口独立限流。
- **Graceful Shutdown**：监听 SIGTERM/SIGINT，通过 `errgroup` 等待所有组件完成。
- 暴露 `/health` (Liveness) 和 `/ready` (Readiness) 端点。

## 5. 类型安全

- **禁止** 使用 `interface{}` / `any` 类型，必须明确类型定义。
- 请求/响应必须有明确的结构体，禁止 `map[string]any`。
- 状态枚举使用 `db/sqlc/constants.go` 中定义的常量，禁止魔法字符串。

## 6. 函数签名

- 所有核心函数（Store, Logic 层）的 **第一个参数** 必须是
  `ctx context.Context`。
- 公共函数/方法必须编写符合 GoDoc 规范的注释。

## 7. 日志

- 使用 `zerolog` 结构化日志。
- **禁止** `fmt.Println`、`log.Print` 等非结构化输出。
- 关键路径日志必须携带 `request_id`、`user_id` 等结构化字段。

## 8. 异步任务

- 后台任务使用 `asynq`，通过 `TaskDistributor` 接口分发。
- 任务必须幂等，Payload 保持最小化（只存标识符）。

## 9. 依赖管理

- 使用 Go Modules，`go.mod` 锁定明确版本，提交 `go.sum`。

## 10. 变更完整性检查

提交后端变更前，必须检查是否形成完整执行路径，而不是只改到一半：

- 改 API 行为时，同时检查路由、请求绑定、Logic 入参、响应 DTO、Swagger、测试。
- 改 Logic 时，同时检查调用入口、Store 接口、事务路径、Worker/Scheduler 入口、测试。
- 改 `db/query/` 或依赖 SQL 结构时，同时检查 `make sqlc`、生成代码调用方、Logic、Handler、测试。
- 新增状态或枚举时，优先落到 `db/sqlc/constants.go`，并检查所有比较与映射点是否同步。
- 新增异步任务时，检查分发、处理、重试/幂等、观测日志是否完整。

## 11. 禁止出现的半成品信号

- SQL 已新增，但没有任何 Logic、Handler、Worker、Scheduler 或测试使用它。
- Logic 已计算出结果，但结果没有持久化、没有返回给调用方、也没有影响后续行为。
- Handler 新增了请求字段或返回字段，但未贯穿到 Logic、Store、DTO 或 Swagger。
- 为了排查问题保留 `fmt.Println`、`panic`、临时 `return`、硬编码测试值、注释掉的生产分支。

## 12. 最低验收门槛

- 说明本次改动是否需要执行 `make sqlc`、`make mock`、`make swagger`。
- 至少运行最小相关验证命令，并在交付时说明执行了什么。
- 若未补测试，必须明确说明残余风险落在哪个分支或失败路径。
- 高风险实现交付前，至少过一遍 `.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md`。
- 正式 backend 审查或 subsystem audit 收口时，至少过一遍 `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`。
