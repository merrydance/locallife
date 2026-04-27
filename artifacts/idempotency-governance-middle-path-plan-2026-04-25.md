# 幂等治理中间方案计划

日期：2026-04-25

## 1. 结论

本系统不应把所有幂等键统一迁入一个中心业务实体。

更稳妥的目标是：领域唯一性分散，入口幂等治理统一。

- 支付、退款、分账、补差、OCR、媒体上传等领域对象继续保留自己的唯一约束和自然幂等键。
- 客户端或上游请求级的 `Idempotency-Key` 统一进入 request-level 幂等登记能力。
- 中心幂等登记表只作为写入口门闩、请求重放识别、冲突判断和审计索引，不替代业务表最终唯一约束。

这不是通用架构的理论最优解，而是当前 LocalLife 这种高外部契约、高异步恢复、高资金状态系统的更优折中。它保留领域语义，同时补齐统一治理、观测、测试和 review 门禁。

## 1.1 当前前提

当前系统尚未上线运行，没有生产历史数据。本计划中的“改造”和“接入”默认都指代码路径、架构边界、SQL/schema、测试和治理规则的改造，不指生产历史数据迁移。文中出现的 `migration` 仅指数据库 schema migration。

因此本计划不需要处理以下生产兼容问题：

- 历史幂等记录 backfill。
- 线上未完成 payment、refund、worker、scheduler 状态的兼容接管。
- 旧版本客户端请求 key 的长期兼容。
- 双写观察期和灰度数据对账。
- 历史冲突记录清洗。

如果本地开发或测试环境已有临时数据，可以按 migration 重建、清库或用测试 fixture 重新生成，不作为生产迁移约束。

## 1.2 当前执行范围

当前本计划只执行阶段 0/1，也就是幂等语义盘点和分类标准沉淀。不实现 `idempotency_records` 中心表，不改造业务入口。

## 2. 当前问题

当前幂等能力分布在多个语义层次：

- 外部支付契约键：`payment_orders.out_trade_no`、`combined_payment_orders.combine_out_trade_no`、`combined_payment_sub_orders.out_trade_no`，见 [locallife/db/migration/000011_add_payment_orders.up.sql](../locallife/db/migration/000011_add_payment_orders.up.sql) 和 [locallife/db/migration/000040_upgrade_profit_sharing_combined_payment.up.sql](../locallife/db/migration/000040_upgrade_profit_sharing_combined_payment.up.sql)。
- 补差、分账、退款类外部单号：`out_subsidy_no`、`out_return_no`，见 [locallife/db/migration/000161_create_subsidy_orders.up.sql](../locallife/db/migration/000161_create_subsidy_orders.up.sql) 和 [locallife/db/migration/000114_add_profit_sharing_returns.up.sql](../locallife/db/migration/000114_add_profit_sharing_returns.up.sql)。
- 请求级幂等键：会员代充值 `Idempotency-Key`，见 [locallife/db/migration/000204_add_membership_recharge_idempotency.up.sql](../locallife/db/migration/000204_add_membership_recharge_idempotency.up.sql)。
- 任务自然键：OCR job 的 `idempotency_key`，见 [locallife/db/migration/000166_create_ocr_jobs.up.sql](../locallife/db/migration/000166_create_ocr_jobs.up.sql)。
- 自然去重查询：媒体上传 pending session 使用 `user_id + media_category + checksum_sha256 + status`，见 [locallife/db/query/media.sql](../locallife/db/query/media.sql)。
- 状态机幂等：worker、回调、恢复任务通过状态判断实现重复执行退出。

问题不在于这些键分散，而在于缺少统一分类、统一入口 helper、统一冲突语义、统一指标和统一测试模板。

## 3. 非目标

本计划明确不做以下事情：

- 不把 `out_trade_no`、`combine_out_trade_no`、`out_subsidy_no`、`out_return_no` 从领域表迁走。
- 不用中心表替代 payment、refund、OCR、media、claim、membership 等业务表的唯一约束。
- 不要求所有重复执行语义都变成 HTTP response replay。
- 不把回调、worker retry、scheduler recovery 的状态机幂等降级成普通请求幂等。
- 不一次性改造所有既有代码路径。

## 4. 目标架构

### 4.1 两层幂等模型

第一层：领域最终约束。

领域表必须继续表达自己的资源唯一性、外部平台单号和状态机恢复锚点。例如支付订单、退款单、补差单、OCR job、上传 session、赔付 action。

第二层：请求级幂等登记。

新增 `idempotency_records` 这类中心能力，用于客户端或上游重复提交同一个写请求的场景。它不拥有业务状态，只记录请求身份、请求指纹、执行状态、最终资源绑定和可选响应快照。

### 4.2 建议数据模型

字段草案：

- `id`: bigserial primary key
- `operation_scope`: text not null，例如 `merchant_membership.recharge`、`payment.create_partner_order`
- `actor_type`: text not null，例如 `user`、`merchant`、`operator`、`system`
- `actor_id`: bigint not null
- `idempotency_key`: text not null
- `request_hash`: text not null
- `status`: text not null，枚举 `processing`、`succeeded`、`failed_retryable`、`failed_terminal`
- `resource_type`: text
- `resource_id`: bigint
- `response_snapshot`: jsonb
- `error_code`: text
- `expires_at`: timestamptz not null
- `created_at`: timestamptz not null default now()
- `updated_at`: timestamptz not null default now()

核心唯一约束：

```sql
UNIQUE (operation_scope, actor_type, actor_id, idempotency_key)
```

必要索引：

```sql
CREATE INDEX idempotency_records_resource_idx
    ON idempotency_records(resource_type, resource_id)
    WHERE resource_type IS NOT NULL AND resource_id IS NOT NULL;

CREATE INDEX idempotency_records_expires_at_idx
    ON idempotency_records(expires_at);
```

### 4.3 状态语义

- `processing`: 请求已被接管但未绑定终态资源；重复请求返回 409 retryable 或按具体 API 契约返回处理中。
- `succeeded`: 请求已成功绑定业务资源；同 hash 重放返回既有资源或安全重建响应。
- `failed_retryable`: 请求未产生不可逆业务效果，允许同 key 同 hash 重试。
- `failed_terminal`: 请求已确定失败；同 key 同 hash 返回稳定失败语义，不重复触发外部副作用。

## 5. 防止坏处出现的机制

### 5.1 防止两套心智模型混乱

治理规则：每个幂等场景必须先分类，再落实现。

分类矩阵：

| 类型 | 判断标准 | 落点 | 示例 |
| --- | --- | --- | --- |
| 外部契约键 | 第三方 API、回调、查询、对账依赖该字段 | 领域表唯一约束 | `out_trade_no`、`combine_out_trade_no`、`out_subsidy_no` |
| 领域资源自然键 | 键本身定义一个业务资源或任务 | 领域表唯一约束 | OCR job `media_asset_id + document_type + owner + side` |
| 请求重放键 | 客户端或上游重试同一个写请求 | `idempotency_records` + 业务表兜底 | 会员代充值、创建支付意图 |
| 异步重复投递 | webhook、worker、scheduler 重复触发 | 状态机条件更新 + 事件/任务记录 | 微信回调、退款恢复、超时关闭 |
| 自然去重查询 | 无显式 key，但业务条件定义同一未完成资源 | 领域查询和唯一/条件索引 | 媒体 pending upload session |

硬规则：新增 G2/G3 写路径时，设计说明必须写明属于上表哪一类。未分类不得进入实现。

### 5.2 防止全局可观测性缺失

中心表不覆盖所有幂等键，因此必须补指标而不是假装一张表天然可观测。

统一指标建议：

- `idempotency_request_total{operation_scope,result}`
- `idempotency_conflict_total{operation_scope,reason}`
- `idempotency_replay_total{operation_scope,status}`
- `domain_unique_conflict_total{domain,constraint}`
- `async_duplicate_delivery_total{source,handler}`

统一日志字段：

- `operation_scope`
- `actor_type`
- `actor_id`
- `idempotency_key_hash`，不得直接高频输出原始 key
- `request_hash`
- `resource_type`
- `resource_id`
- `idempotency_result`

验收门禁：接入 request-level 幂等登记的入口必须同时接入指标和结构化日志字段。

### 5.3 防止公共能力被绕过

新增 backend helper，而不是让业务代码手写表访问。

建议接口：

```go
type IdempotencyGuard interface {
    Begin(ctx context.Context, input BeginIdempotencyInput) (BeginIdempotencyResult, error)
    Succeed(ctx context.Context, input CompleteIdempotencyInput) error
    FailRetryable(ctx context.Context, input FailIdempotencyInput) error
    FailTerminal(ctx context.Context, input FailIdempotencyInput) error
}
```

实现约束：

- `api/` 只读取 header、绑定 actor、构造 operation scope，不直接操作幂等表。
- `logic/` 决定请求 hash、资源绑定、冲突语义和业务错误。
- `db/query/` 只暴露 guard 需要的最小 SQL，不让每个业务域随意拼自己的幂等查询。
- 所有高风险入口禁止复制粘贴旧会员充值式的手写幂等流程，除非该流程被明确列为改造前临时保留路径。

review 门禁：凡是新增 `Idempotency-Key`、`idempotency_key`、`out_*_no`、`notification_id`、重复执行退出逻辑，都必须检查是否命中本计划分类矩阵。

### 5.4 防止 request hash 设计过度或错误

每个 `operation_scope` 必须定义 canonical request fields。

参与 hash 的字段：

- 金额、币种、目标用户/会员/商户、业务资源 ID、外部 channel、关键备注、source。
- 会改变资金、权限、状态或业务效果的所有字段。

不得参与 hash 的字段：

- trace id、request id、客户端当前时间、nonce、展示用冗余字段。
- 服务端可从 actor 或资源重新推导且不改变业务效果的字段。

验收门禁：每个新 scope 必须有一条“同 key 不同 hash 返回 409”的测试，以及一条“同 key 同 hash 返回既有资源”的测试。

### 5.5 防止 response snapshot 变成敏感数据源

默认不存完整响应。

优先级：

1. 存 `resource_type/resource_id`，重放时从业务表重建响应。
2. 只在响应无法稳定重建、且字段安全时存最小 `response_snapshot`。
3. 严禁存储 token、签名私钥、微信原始敏感 payload、身份证号、银行卡号、手机号全量、raw provider payload。

验收门禁：引入 `response_snapshot` 的 scope 必须在设计说明里列出字段 allowlist 和保留周期。

### 5.6 防止跨资源创建仍然不一致

中心幂等表只保护入口重复提交，不负责替代事务和外部副作用恢复。

高风险写路径仍必须满足：

- 本地 durable anchor 先落库。
- 外部调用不放在数据库事务内部。
- 外部调用成功但本地后续失败时，有查询、回调或 recovery 能收敛。
- 业务终态由领域 owner 写入，不由支付/微信 adapter 直接推进。
- 领域表唯一约束保留最终兜底。

验收门禁：支付、退款、赔付、充值这类 G3/G2 路径，计划或 PR 必须说明重复提交、处理中重试、外部成功本地失败、worker 重试四种路径。

### 5.7 防止 TTL 语义混乱

按 operation scope 配置 TTL，不做全局固定值。

建议默认：

| scope 类型 | TTL 建议 | 原因 |
| --- | --- | --- |
| 普通非资金写请求 | 24h-72h | 覆盖客户端重试窗口 |
| 支付/退款/充值/赔付入口 | 7d-30d | 覆盖外部回调、查询和人工重试窗口 |
| 高审计资金路径 | 90d 或长期归档 | 便于追责和对账 |
| response snapshot | 尽量短于 record TTL | 降低敏感数据和响应漂移风险 |

过期不应删除领域唯一约束。过期只表示 request replay guard 不再提供旧响应，不表示业务资源可重复创建。

## 6. 实施阶段

### 阶段 0：幂等语义盘点

目标：建立完整现状矩阵。

任务：

- 扫描 backend 中所有 `Idempotency-Key`、`idempotency_key`、`out_trade_no`、`combine_out_trade_no`、`out_subsidy_no`、`out_return_no`、`notification_id` 和中文“幂等”注释。
- 按第 5.1 分类矩阵归类。
- 标记每个场景的风险等级、现有唯一约束、重放语义、冲突语义、测试覆盖和是否需要中心登记。

产出：`artifacts/idempotency-scope-inventory-2026-04-25.md`。

验收：能明确回答每个幂等键是否应该接入 request-level guard；不能回答的场景不得进入代码改造。

### 阶段 1：标准和设计契约

目标：先把规则固化，防止实现阶段走偏。

任务：

- 在 backend 标准或 domain 标准中沉淀“领域唯一性分散，入口幂等治理统一”。
- 定义 operation scope 命名规范。
- 定义 request hash 字段选择规则。
- 定义 replay、conflict、processing、terminal failure 的 API 错误语义。
- 更新 review checklist，新增幂等分类检查项。

验收：新增 G2/G3 写入口在 review 时有明确 checklist 可查，不依赖口头判断。

## 7. 后续实现测试矩阵草案

以下只作为阶段 1 沉淀到 backend 标准或 review gate 的测试要求草案，不代表当前进入实现。

每个接入 request-level guard 的 scope 至少覆盖：

- 缺少 `Idempotency-Key` 的 API 行为。
- 首次请求成功创建资源。
- 同 key 同 hash 重放返回同一资源或等价响应。
- 同 key 不同 hash 返回 409。
- 并发同 key 只有一个请求执行副作用。
- `processing` 状态下重复提交不触发外部调用。
- `failed_retryable` 下同 key 同 hash 可重试。
- `failed_terminal` 下同 key 同 hash 返回稳定失败。
- 资源已创建但 guard 完成更新失败时，可通过领域唯一约束和恢复任务收敛。

对 G3 资金路径，额外覆盖：

- 外部调用成功、本地响应失败后的查询/回调恢复。
- 回调重复投递。
- scheduler 或 worker 重试。
- 领域表唯一约束冲突映射。

## 8. 后续实现交付门禁草案

以下只作为阶段 1 的门禁草案，不代表当前创建中心表或改造入口。

任何幂等治理实现 PR 必须说明：

- 风险等级：通常为 G2；涉及支付、退款、资金、回调、OCR、上传则为 G3。
- 幂等类型分类：外部契约键、领域自然键、请求重放键、异步重复投递或自然去重查询。
- 是否接入 `idempotency_records`，以及为什么。
- 领域表最终唯一约束是什么。
- request hash 的 canonical 字段。
- 同 key 不同请求的冲突语义。
- `processing` 重放语义。
- TTL 和 snapshot 策略。
- 已运行的 regeneration：`make sqlc`、`make mock`、`make swagger` 是否需要。
- 已运行的验证命令和未验证的残余风险。

## 9. 阶段 0/1 成功标准

本轮成功不是“所有幂等键都在一张表里”，也不是完成中心表实现，而是达到以下状态：

- 产出全量幂等 inventory。
- 每个已发现幂等键都有分类、风险等级、现有落点和是否候选接入 request-level guard 的判断。
- 分类矩阵和交付门禁草案已沉淀到 backend 标准或 review gate。
- 明确哪些键属于领域唯一约束，哪些键属于请求重放治理，哪些属于异步重复投递或自然去重。
- 明确本阶段不创建 `idempotency_records`，不实现 `IdempotencyGuard`，不改造会员充值、支付、退款、赔付等入口。
- review 能在设计阶段发现错误分类，而不是等到实现或线上事故后才发现。

## 10. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 中心表被误用为所有业务唯一性的替代品 | 在标准和 review gate 中明确它只处理 request-level guard |
| request hash 字段遗漏导致错误重放 | 每个 scope 固化 canonical 字段并测试同 key 不同请求 |
| snapshot 泄漏敏感数据 | 默认不用 snapshot，必须字段 allowlist |
| processing 状态卡住 | 增加 processing age 指标、恢复任务和人工排查查询 |
| 代码改造范围过大 | 只按 operation scope 小步接入，先会员充值样板 |
| 外部成功本地失败无法收敛 | 保留领域 durable anchor、查询、回调、worker recovery |

## 11. 本轮交付项

本轮只交付阶段 0 和阶段 1：

1. 产出全量幂等 inventory：`artifacts/idempotency-scope-inventory-2026-04-25.md`。
2. 把分类矩阵沉淀到 backend 标准：`.github/standards/backend/IDEMPOTENCY_STANDARDS.md`。
3. 把幂等分类检查补入 backend change safety checklist：`.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md`。

本阶段不创建中心表，不实现 helper，不修改业务代码路径。
