# Backend SQL Standards

本文件定义 LocalLife 后端在 PostgreSQL、migration、sqlc query source 与 `db/sqlc/` 持久层上的长期有效规范。

适用范围：

- `locallife/db/migration/`
- `locallife/db/query/`
- `locallife/db/sqlc/`
- 所有直接依赖 SQL 结构、query 签名或持久化语义的 `logic/`、`worker/`、`scheduler/` 与测试

与其他后端标准的关系：

- 架构与分层边界：看 `.github/standards/backend/SYSTEM_PROMPT.md`
- 后端硬约束：看 `.github/standards/backend/AGENT.md`
- API 合同与错误语义：看 `.github/standards/backend/API_CONTRACT_STANDARDS.md`

## 1. 基本原则

- 使用 `pgx` + `sqlc`，禁止引入全功能 ORM。
- SQL 源定义以 `locallife/db/query/` 为准；生成代码以 `locallife/db/sqlc/` 为准；不要手改 `*.sql.go`。
- 持久层只负责数据读写、事务编排与持久化常量，不承载 transport DTO 或 handler 级别响应塑形。
- 任何 SQL、schema、query signature 变更都必须继续传播到生成代码、调用方和测试，不能停在 SQL 层。

## 2. 目录职责

### 2.1 `db/migration/`

- 只放 schema 演进、索引、约束、默认值、数据修复所需的 migration。
- migration 是 forward-only 的默认路径；除非是刚上线且未承载真实业务数据的短窗口事故恢复，不以 down migration 作为常规修复手段。
- 新 migration 使用 `make new_migration name=<name>` 生成，避免手写文件名和顺序号。

### 2.2 `db/query/`

- 只放供 sqlc 消费的 SQL query 定义。
- 领域相关的 query 应放入已有领域 SQL 文件，除非出现明确的新边界。
- query 名称必须稳定、可读、领域化，避免让生成方法出现含糊或一次性命名。

### 2.3 `db/sqlc/`

- 存放 sqlc 生成代码、`Store` 组合接口、`tx_*.go` 事务胶水与 `constants.go`。
- 事务编排统一走 `execTx` 模式，不写裸 `Begin/Commit`。
- 业务状态与持久化相关枚举统一收口到 `constants.go`，其他层引用这里而不是各自复制字符串。

## 3. Query 编写规范

### 3.1 命名与结构

- 保持 sqlc 标准命名头：`-- name: Xxx :one|:many|:exec|:execrows|:execresult`。
- query 名称描述业务含义，不描述暂时性实现细节。
- 保持单个 query 关注单一读写目标，避免把多段互不相关的业务塞进一个巨大 SQL。
- 当已有文件内已存在同风格的可选过滤、分页或更新模式时，复用原模式，不在一个领域文件里再造一种新风格。

### 3.2 选择列与返回面

- 默认显式列出需要的列，避免在生产 query 中使用 `SELECT *`、`alias.*`、`table.*` 或 `cte.*`。
- 返回列应与真实调用面匹配，不为了“以后可能会用”而扩大返回面。
- 当结果顺序会影响业务、分页或测试稳定性时，必须显式 `ORDER BY`。

### 3.3 过滤与租户边界

- 所有读写 query 都要显式表达作用域边界，例如 `user_id`、`merchant_id`、`rider_id`、状态字段或软删除条件；不要把权限边界默认留给调用方想当然处理。
- 不要把 handler 或页面层的字段命名、展示语义直接编码进 query。
- 对可空过滤、分页游标、状态筛选等常见模式，优先保持当前代码库已有写法，避免同类问题出现多种 SQL 方言。

### 3.4 更新与删除

- `UPDATE` 和 `DELETE` 必须有明确 `WHERE` 边界，避免无作用域写入。
- 优先显式写出要更新的列，不依赖隐式行为。
- 状态迁移相关写入要体现当前状态前置条件时，应把前置条件写入 `WHERE` 或由上层事务语义保证，避免并发下的静默覆盖。
- 是否物理删除、软删除或归档，必须跟随当前领域既有语义，不在单个 query 中私自切换模型。
- 对被 CI 轻量 guard 命中的变更 query，默认会拦截无 `WHERE` 的 `UPDATE` 或 `DELETE`；如果确有合理的全表写入场景，必须在 query block 内用 `sqlguard: allow-unscoped-write` 注释说明。

### 3.5 禁止写法与推荐写法

以下清单用于把高频 SQL 坏味道写成显式规则。能自动门禁的，会在后文说明；其余至少应作为 review 必查项。

| 场景 | 禁止写法 | 推荐写法 |
| --- | --- | --- |
| 读取列面 | `SELECT * FROM orders WHERE id = $1;` | `SELECT id, user_id, status, total_amount, created_at FROM orders WHERE id = $1;` |
| 读取列面 | `SELECT orders.* FROM orders WHERE id = $1;` | `SELECT id, user_id, status, total_amount, created_at FROM orders WHERE id = $1;` |
| 分页稳定性 | `SELECT id, status FROM orders WHERE user_id = $1 LIMIT $2 OFFSET $3;` | `SELECT id, status FROM orders WHERE user_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3;` |
| 无作用域写入 | `UPDATE orders SET status = 'closed';` | `UPDATE orders SET status = 'closed' WHERE id = $1 AND status = 'pending';` |
| 无列名插入 | `INSERT INTO orders VALUES ($1, $2, $3);` | `INSERT INTO orders (user_id, merchant_id, status) VALUES ($1, $2, $3);` |
| 事务外兜底状态条件 | `UPDATE refunds SET status = 'success' WHERE id = $1;` | `UPDATE refunds SET status = 'success' WHERE id = $1 AND status = 'processing';` |

额外说明：

- `SELECT *` 与 `alias.*` / `table.*` / `cte.*` 不是“偷懒”，而是会扩大返回面、放大 schema 漂移风险，并让 review 难以判断调用面真实依赖。
- `LIMIT` / `OFFSET` 没有 `ORDER BY`，在业务、分页和测试上都不稳定。
- `UPDATE` / `DELETE` 没有 `WHERE`，默认按事故级危险写法处理。
- `INSERT INTO ... VALUES (...)` 不写列名，会让 schema 演进时的风险被隐式放大。
- 状态迁移若依赖当前状态成立，就不要把前置条件藏在事务外假设里；应落实在 `WHERE` 或明确事务语义里。

## 4. Schema 与 Migration 规范

### 4.1 设计原则

- schema 变更优先采用兼容性更好的增量方式：先加字段/索引/新表，再逐步切换调用方，最后再考虑清理旧结构。
- 新增查询热点、唯一性要求或外键语义时，同时评估是否需要索引、唯一约束或检查约束，避免只改功能不补数据约束。
- 金额、状态、时间、外部单号、幂等键等关键字段的类型选择必须与现有领域模型保持一致，避免同类数据出现多种表示方式。

### 4.2 高风险变更

- 会影响大表扫描、批量回填、索引重建、锁竞争、金额状态语义或多租户隔离的 schema 变更，按高风险路径处理，不能按普通字段增删看待。
- 破坏性 schema 操作默认需要明确的兼容说明、验证方案和恢复思路；如果没有这些信息，不应把变更当作 routine patch。

### 4.3 数据修复与回填

- 一次性数据修复脚本、回填逻辑或审计工具若需要落在代码库，应明确边界、幂等性、目标数据范围与执行顺序。
- 不要把临时修复逻辑偷偷塞进常驻 handler、logic 或 query 中掩盖 schema 问题。

## 5. 事务、并发与锁

- 多表写入、余额/库存/状态迁移等需要原子性的路径，统一放在 `db/sqlc/tx_*.go` 或明确的 store 事务方法里。
- 不要把跨多步写入拆散到 handler 或多个无事务保护的 logic 调用中。
- 当正确性依赖“读后写”顺序时，要明确事务边界、当前状态条件或唯一约束支撑，避免并发请求互相覆盖。
- 新增锁敏感 query 或热点写路径时，要说明为什么当前语义在重复请求、重试或并发提交下仍然成立。

### 5.1 状态机与条件更新

- 订单、支付、退款、配送、预订、库存、delivery pool 等状态迁移写入，默认要把“当前状态前置条件”落实到 `WHERE` 或同一事务的锁语义里，而不是只依赖事务外检查。
- 对依赖状态前置条件的 `UPDATE ... RETURNING`，调用方必须检查“是否真的命中 1 行”；如果命中 0 行，默认视为并发冲突、状态漂移或重复执行，而不是静默成功。
- 禁止把终态/前态切换写成“只按 id 更新”的宽条件写法，尤其是 `pending -> paid`、`ready -> courier_accepted`、`processing -> succeeded`、`reserved -> released` 之类高风险状态迁移。

推荐示例：

```text
-- bad: 并发下会把任何当前状态直接改成 ready
UPDATE orders
SET status = 'ready', updated_at = now()
WHERE id = $1
RETURNING *;

-- good: 只有 preparing 状态才能推进到 ready
UPDATE orders
SET status = 'ready', updated_at = now()
WHERE id = $1
  AND status = 'preparing'
RETURNING *;
```

### 5.2 排他性、唯一性与 claim 语义

- “一个预订只允许一个活跃订单”“一个配送池项目只允许一个成功抢单者”“同一库存单元不能被重复占用”等约束，不能只靠事务外 `SELECT` 判断。
- 当业务正确性依赖排他性时，优先考虑：唯一约束、部分唯一索引、`SELECT ... FOR UPDATE`、带状态条件的 claim 更新、或显式 lease 字段。
- callback、worker、scheduler、delivery pool、recovery scanner 相关 query 必须定义重复领取和重复执行语义，不允许默认“应该只跑一次”。

推荐示例：

```text
-- bad: 只读检查后在别处再写，两个并发请求都可能通过
SELECT COUNT(*)
FROM table_reservations
WHERE table_id = $1
  AND reservation_date = $2
  AND status IN ('pending', 'paid', 'confirmed');

-- good: 在事务里锁定目标记录，或通过条件更新 claim 目标状态
SELECT *
FROM delivery_pool
WHERE order_id = $1
FOR UPDATE;
```

### 5.3 热路径扫描、分页与聚合

- 商户订单列表、用户订单列表、配送池、退款/恢复扫描、运营筛选页等热路径，默认不能把大 `OFFSET`、无界 `COUNT(*)` 或全表排序当作免费操作。
- 对高基数、持续增长且用户高频访问的列表，默认优先评估 keyset / seek pagination，而不是只延续 `LIMIT ... OFFSET ...` 习惯。
- `COUNT(*)`、热状态扫描、恢复任务扫描若进入请求热路径，必须明确说明索引支撑、访问频率和可接受代价；否则应优先考虑 `has_more`、summary/projection、异步统计或受控扫描。
- 新增 recovery / pool / timeout scanner 时，默认按 `status + next_retry_at/expires_at + tenant/scope` 设计过滤与索引，而不是依赖“数据量还不大”的假设。

推荐示例：

```text
-- bad: 热路径高 offset 分页
SELECT id, status, created_at
FROM orders
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- better: 高基数热路径优先考虑 keyset
SELECT id, status, created_at
FROM orders
WHERE merchant_id = $1
  AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;
```

## 6. 生成、传播与验证

- 修改 `db/query/` 或依赖 SQL 结构时，运行 `make sqlc`。
- 如果生成接口变化影响 mocks，运行 `make mock` 或对应生成步骤。
- SQL 变更的最低闭环检查包括：
  - 生成代码是否更新。
  - `Store`/Logic/Handler/Worker/Scheduler/Test 是否继续可达并语义一致。
  - 新字段、新状态、新 query 是否真正被调用，而不是留成孤儿实现。
- 默认优先运行最小相关测试；若变更触及真实数据库行为、事务分支、索引依赖或 migration 语义，再补足更高层验证。

### 6.1 当前 CI SQL Guard 范围

- 当前 guard 对 `SELECT *` 与 qualified-star 用法采用仓库级扫描：
  - 当前工作树中所有 `locallife/db/query/*.sql` 都会被扫描。
  - `SELECT *` 的历史债务统一记录在 `.github/sqlguard/select_star_baseline.txt`。
  - 新增 `SELECT *` 若不在 baseline 中，会直接失败，即使它不属于旧债文件。
  - qualified-star 用法当前无 baseline；任何 `alias.*` / `table.*` / `cte.*` 都会直接失败，除非写了带理由的 `sqlguard: allow-qualified-star`。
- 除 `SELECT *` 与 qualified-star 之外，其余轻量规则仍只检查被本次 diff 实质性触碰到的 query block。
- 当前 diff-based guard 会拦截三类高信噪比问题：
  - `:many` query 中使用 `LIMIT` 或 `OFFSET` 但没有 `ORDER BY`。
  - `UPDATE` 或 `DELETE` 语句缺少 `WHERE`。
  - `INSERT INTO <table> VALUES (...)` 这类未显式声明列名的插入语句。
- 当前 guard **不会** 自动拦截所有 SQL 坏味道；例如状态迁移条件过宽、热路径 `COUNT(*)` 或查询列面过大但不是 `SELECT *`，仍主要依赖实现者与 reviewer 按本标准收口。
- 仅注释改动不会重新触发同一 query block 的 diff-based 历史坏 SQL 检查，但 query 名称变更仍视为实质性变更。
- 允许的例外必须在对应 query block 内显式说明：
  - `sqlguard: allow-select-star`
  - `sqlguard: allow-qualified-star`
  - `sqlguard: allow-unordered-limit`
  - `sqlguard: allow-unscoped-write`
  - `sqlguard: allow-implicit-insert-columns`
- `sqlguard:` 例外注释不是空白豁免。每条例外至少要在同一注释行里写出：
  - 为什么默认规则在这里不适用。
  - 为什么该例外在当前 query block 内仍然安全。
  - 例外边界是什么，避免后续调用方误把它当通用模式复制。
- 禁止只写裸标记，例如只写 `-- sqlguard: allow-select-star` 却不解释原因；这种写法应视为门禁绕过尝试。
- 对支付、退款、分账、回调、worker/recovery、订单/履约/预订/库存状态机等 `G2` / `G3` 路径，`sqlguard:` 例外默认按高风险审查点处理，而不是普通注释。

### 6.2 本地校验命令

- 日常本地自测当前 guard：`bash .github/scripts/test_backend_sql_guard.sh`
- 对当前工作树执行全仓 `SELECT *` baseline 校验并叠加 diff-based 规则：`bash .github/scripts/backend_sql_guard.sh <base_sha> HEAD`
- 如需维护历史债务台账，同步更新 `.github/sqlguard/select_star_baseline.txt`，确保只保留仍存在的 legacy `SELECT *` query block。
- CI 中会先执行 guard 自测，再对本次变更运行 guard，最后执行 sqlc、mock、swagger 生成物一致性检查。

## 7. 性能与可运维性

- 新增复杂筛选、聚合、分页、排序、批量更新或热点查询时，至少判断一次索引是否支撑目标访问路径。
- 对非平凡 query，不要把“应该很快”当作证据；必要时使用 `EXPLAIN` 或 `EXPLAIN ANALYZE` 验证执行计划。
- 避免在业务热路径中引入明显的 N+1 SQL 模式；若必须分步查询，应说明原因并控制调用规模。
- query 日志、错误与排障信息不能把原始 SQL、驱动细节或敏感参数直接暴露给用户侧响应。
- CI 允许做轻量 SQL guardrail，但 guardrail 只能覆盖高信噪比规则；如果确有合理例外，必须用明确注释说明原因，例如 `sqlguard: allow-select-star`、`sqlguard: allow-unordered-limit`、`sqlguard: allow-unscoped-write`、`sqlguard: allow-implicit-insert-columns`，而不是绕过 source-of-truth 或直接改生成文件。

## 8. Review 检查单

评审 SQL 相关改动时，至少检查以下问题：

- 这是 source-of-truth 层改动，还是误改了生成文件。
- 新 query 是否有明确调用方和测试覆盖面。
- schema、query signature 或返回字段变化是否一路传播到上层。
- 是否引入了不带作用域的写入、遗漏租户边界或遗漏状态前置条件。
- 新访问路径是否需要索引、约束或额外事务保护。
- migration 是否保持前向兼容，是否说明了上线顺序与恢复方式。

## 9. 禁止事项

- 手改 sqlc 生成文件。
- 在 SQL 层编码 transport DTO、前端展示字段或 handler 特有响应结构。
- 使用没有明确边界的 `UPDATE` / `DELETE`。
- 新增 query 后不接调用方、不补测试、也不说明为什么是预留实现。
- 为了临时排障把一次性数据修复、硬编码条件或危险 SQL 留在长期代码路径中。