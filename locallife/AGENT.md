# Go 后端硬约束

> 本文件定义 **不可违反的工程约束**。项目架构、分层、错误处理等实现细节见
> `SYSTEM_PROMPT.md`。

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
