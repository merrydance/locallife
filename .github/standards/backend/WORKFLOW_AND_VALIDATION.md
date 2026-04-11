# Backend Workflow And Validation

> 作用：把 LocalLife 后端当前仓库的常用命令、生成链路、DB-backed 测试行为和最小充分验证方式沉淀为固定工作语言。

## 1. Working Directory

以下命令默认从 `locallife/` 目录执行。

## 2. Core Commands

### Core development commands

- `make server`
- `make test`
- `make test-unit`
- `make test-integration`
- `make test-safety`
- `make check-generated`
- `make migrateup`
- `make sqlc`
- `make swagger`
- `make lint-filesize`

### Useful targeted commands

- `go test ./api`
- `go test ./logic`
- `go test ./worker`
- `go test ./db/sqlc`
- `go test ./logic -run 'TestCreateOrder'`
- `./scripts/test_safety.sh`
- `./scripts/check_generated.sh`

## 3. DB-Backed Test Behavior

`locallife/db/sqlc/main_test.go` 当前会：

- 读取 `TEST_DB_SOURCE`，否则退回 `DB_SOURCE`
- 默认使用 `postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql`
- 在测试前运行 migration `Up()`

含义：

- `db/sqlc` 下的测试不应被误认为纯单元测试
- SQL、事务与资金链路相关测试通常要求真实可用的 Postgres

## 4. Regeneration Rules

- 改动 `locallife/db/query/` 或影响生成代码的事务/接口边界后，运行 `make sqlc`
- 改动 Swagger 注释、路由或公开 API 契约后，运行 `make swagger`
- 只要改了 SQL 或 Swagger 源文件，优先再运行 `make check-generated`
- 若只改 handler / logic 且未触及生成源，通常不需要 regeneration，但要明确说明你做了这个判断

## 5. Validation Strategy

对非平凡 backend 任务，默认按这个顺序思考：

1. 识别真实入口和真实写边界
2. 读上游调用方与下游 async / recovery 路径
3. 在能真正落实不变量的最低层修改
4. 先跑最接近风险边界的 targeted tests
5. 共享行为受影响时，再扩到包级测试或更广的命令

## 6. Repo-Specific Heuristics

- 优先通过 `db.Store` 访问持久化与事务边界，不要绕开已有 transaction helpers
- 优先复用现有 service constructor 和 interface，而不是新增 ad hoc helper
- 对“logic 看起来对了”的改动，默认追问 worker / scheduler / recovery 是否仍然一致
- 触及已审查过的生产链路时，优先回看 `locallife/docs/production-robustness-*.md`
- 除非任务明确要求，否则避免对 `api/`、`logic/`、`worker/` 做广泛重构

## 7. High-Risk Validation Defaults

下列路径优先考虑 `make test-safety` 或等价 focused regressions：

- order creation
- payment creation and success processing
- refund and profit-sharing transitions
- delivery state transitions
- reservation uniqueness / exclusivity

当前根目录 workflow `/.github/workflows/backend-safety.yml` 还会检查：

- focused high-risk regressions
- generated artifact consistency
- file-size guardrails

当前 `/.github/workflows/backend-focused-gosec.yml` 还会对变更涉及的高风险 backend Go 包做 focused SAST；默认覆盖 `api`、`logic`、`worker`、`media`、`wechat` 与 `util` 范围，并在发现问题或扫描失败时阻断 workflow，而不是只做提醒。

## 8. Handoff Expectation

交付 backend 任务时，至少说明：

1. 风险等级及依据
2. 运行了哪些命令
3. 哪些层已验证：`api` / `logic` / `db/sqlc` / `worker` / `scheduler` / callback
4. 哪些路径未验证
5. 是否存在 generation、发布、回滚或观测缺口

如果这些问题说不清，该交付不能算生产级闭环。

