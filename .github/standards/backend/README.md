# Backend Standards Index

本目录是 LocalLife 后端在 `.github` 下的正式权威入口，用于收拢长期有效的后端工程规则、仓库级运行上下文、高风险链路和正式审查收口要求。

适用范围：

- `locallife/`
- 与后端实现、审查、生成物、验证和高风险链路直接相关的 `.github/` 资产

## 推荐阅读顺序

1. `.github/standards/engineering/README.md`
2. `.github/standards/backend/AGENT.md`
3. `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
4. `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`

然后按任务实际需要再打开最小相关深文档，而不是每次都把后端全栈指导全部读完：

- `API_CONTRACT_STANDARDS.md`: 路由、状态码、empty-state、契约语义
- `SYSTEM_PROMPT.md`: 分层、middleware、DTO、依赖注入等实现细则
- `GO_PRACTICES.md`: Go 代码组织、并发、context、测试与 Go guard 规则
- `SQL_STANDARDS.md`: SQL、migration、索引、并发写语义与 SQL guard 规则
- 对应 `domain README`: 支付、media、OCR 等高风险域入口
- `BACKEND_CHANGE_SAFETY_CHECKLIST.md`: 实现/修复交付前收口
- `BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`: review/audit 收口
- `FORMAL_REVIEW_DURABILITY.md`: findings、pattern、audit log 的 durable writeback

## 文档角色

- `AGENT.md`: 不可违反的后端硬约束。
- `SYSTEM_PROMPT.md`: 分层、错误模型、中间件、依赖注入、类型与实现一致性。
- `RUNTIME_ARCHITECTURE.md`: 当前仓库真实运行结构、启动顺序、核心域与异步边界。
- `WORKFLOW_AND_VALIDATION.md`: 本仓库的生成链路、测试链路、局部验证与常用命令。
- `BACKEND_CHANGE_SAFETY_CHECKLIST.md`: backend 实现/修复任务的交付前收口清单。
- `BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`: backend review/audit 结束前的收口清单。
- `FORMAL_REVIEW_DURABILITY.md`: 正式 backend review 如何把 findings、模式和审查范围沉淀为 durable knowledge。
- `API_CONTRACT_STANDARDS.md`: API 契约语义与状态码口径。
- `GO_PRACTICES.md`: Go 代码组织、接口、context、并发与测试实践。
- `SQL_STANDARDS.md`: SQL、migration、索引与 sqlc 规则。

## 使用规则

- 先看本目录和 engineering index，再下钻到更细的 area 或 domain 标准。
- 仓库级运行事实、命令和高风险入口优先写在本目录，并通过对应 domain README 继续下钻，不要只留在子目录私有 prompt 上下文里。
- 高频执行提醒继续镜像到 `.github/instructions/`。
- 高频任务入口继续镜像到 `.github/prompts/`。
- 如果某个后端规则既是长期基线，又和当前仓库的真实运行方式强相关，优先把基线落在本目录，再让 `locallife/AGENTS.md` 等入口文件引用它，而不是反过来。

