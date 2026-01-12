# AI 编程提示词约定（Go / 小程序 / 后端）

## 使用方式
- 在开始实现前先阅读此文档，再结合 `.agent/rules/project-rules.md` 与已有代码风格。
- 编写提示词时，将下列约定作为检查清单，确保生成代码不跑偏。

## 总体原则
- 先找现有实现：同类功能（handler、service、repo、task、util）有无可复用或对齐的模式。
- 小步快跑：拆成小函数/小文件，清晰输入输出；依赖通过接口注入，避免全局状态。
- 以类型约束/接口组合聚合：定义 `Request/Response`、`DTO`、`Service` 接口，减少隐式约定。
- 显式失败与日志：错误要包装上下文；关键路径使用结构化日志，避免静默失败。
- 可测试性优先：纯函数优先，其次接口+mock；保持 determinism（无随机/时间依赖时需注入时钟）。

## Go 代码约定
- 遵循 go.mod 的 Go 版本（目前写明 1.24），不要使用项目未采用的试验特性；保持与现有代码风格一致。
- `context.Context` 贯穿入口，禁止在库函数内创建新 context。
- 入参校验：基础校验（空值/范围），跨层复用 validator/已有校验器，返回可读错误。
- 并发：使用 `errgroup` / channel 时确保超时和取消；避免共享可变状态，必要时用锁或单线程约束。
- 错误处理：区分业务错误与系统错误；对外返回统一 envelope；内部使用 `errors.Wrap` 或自带上下文。
- 日志：使用 zerolog，带关键字段（user_id, merchant_id, order_id, task_id）。避免敏感信息泄露（密钥、证照号码、手机号）。
- 配置：全部来自 `util.Config`，新增项要加 mapstructure tag 与 .env 示例，并处理可选值。
- I/O：文件/网络操作必须超时，避免阻塞；大对象避免全量读入内存。
- 安全：
  - token/签名严格校验；上传/下载遵守签名 URL 策略。
  - 禁止把密钥、API 响应敏感字段写日志或返回给前端。
  - 遵守内容安全（图片/文本检查）调用路径。

## HTTP Handler 约定（Gin）
- 路由注册对齐现有分组/中间件（鉴权、速率限制、响应包裹、Tracing、Prometheus）。
- Request/Response 使用明确 DTO，保持字段命名与前端约定一致；响应统一 `{code,message,data}`。
- 认证与权限：重用现有 middleware（auth、Casbin、MerchantStaffMiddleware 等）。
- 上传/下载：仅允许通过签名 URL；大文件限制 `MaxMultipartMemory`；返回的路径遵循 `/uploads/...` 约定。

## 数据库 & Redis
- Postgres 访问通过 sqlc 生成的 Store；禁止手写拼接 SQL。新增查询放在 db/query，表结构变更放在 db/migration，然后重新生成 sqlc（并按需 mockgen）。
- 事务：优先复用已有 Tx 方法；如需新事务，遵循 db/sqlc 里的 `execTx` 模式，避免在事务内做外部调用（HTTP/Redis）。
- 乐观/悲观锁：遵循现有模式（如 `FOR UPDATE` 查询）；确保幂等更新。
- Redis：区分 KV、队列、Pub/Sub；Redis 地址/密码来自配置；序列化格式对齐现有（JSON/MsgPack）。

## 异步任务（asynq）
- 任务类型、Payload 结构放在 worker 包统一定义；分发通过 `TaskDistributor`，处理在 `TaskProcessor`。
- 任务必须是幂等的；需要重试策略时设置合适的 `MaxRetry`、队列优先级。
- 跨任务调用保持最小化 payload，避免放置大对象/敏感数据。

## 微信相关
- 小程序登录/支付/分账/退款：遵循 wechat 包接口，严控签名、解密、证书路径。
- 内容安全：上传前图片/文本必须走 `ImgSecCheck` / `MsgSecCheck`；OCR 使用专用接口（营业执照/身份证/食品许可）。
- 通知回调：校验签名后再处理；处理逻辑幂等，避免重复推送。

## 前后端联调
- 保持字段、状态机、错误码与前端文档一致；必要时在注释中标明前端依赖点。
- 如果变更 API 行为/字段，更新 Swagger（`make swagger`）与前端对齐。

## 测试与验证
- 单测首选表驱动；使用 mock（在 `~/go/bin/mockgen`）对外部依赖打桩。
- 对时间/随机依赖注入时钟/种子，避免 flaky。
- 如改动查询或状态机，补充相应单测或至少标注 TODO。

## 提示词模板（示例）
> 目标：实现 XXX（简述功能与上下文）。
> 约束：
> - 使用现有路由/中间件模式（参考 api/server.go）。
> - 数据访问走 sqlc Store，新增查询放 db/query，表变更放 db/migration，重生成 sqlc。
> - 事务遵循 db/sqlc 的 execTx 模式，避免事务内外部调用；能复用现有 Tx 方法则复用。
> - 任务通过 TaskDistributor/TaskProcessor，保持幂等。
> - 配置从 util.Config 注入，不写死常量。
> - 日志使用 zerolog，勿泄露敏感信息。
> - 需要内容安全/支付/签名的路径遵循 wechat 包接口。
> 输出：列出修改的文件与主要函数，给出关键实现。

## 说明
- “编写提示词”指在用 AI 辅助编码前，先把需求拆解成清晰约束与输出要求（如上模板），防止模型产出偏离项目既有模式。
- 此文档与 `.agent/rules/project-rules.md` 一起使用，任何新增能力前先比对现有代码实现，尽量复用。
