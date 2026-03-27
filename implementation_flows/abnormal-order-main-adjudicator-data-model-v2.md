# 异常订单主判数据模型 V2

日期：2026-03-27

## 1. 文档定位

本文不是规则表，也不是代码任务单，而是异常订单全自动主判在数据库层的正式落库方案。

它只回答四个问题：

1. 现有表结构为什么不足以支撑主判 V2。
2. 哪些表应该扩展，哪些表应该新增，而不是继续把语义塞进旧字段。
3. 平台兜底、用户限制、申诉翻盘、画像回滚分别应落到哪里。
4. 迁移时如何最小化对现有 API、worker、sqlc 的破坏。

配套上游文档：

1. [implementation_flows/abnormal-order-main-adjudicator-redesign.md](implementation_flows/abnormal-order-main-adjudicator-redesign.md)
2. [implementation_flows/abnormal-order-main-adjudicator-rules-matrix.md](implementation_flows/abnormal-order-main-adjudicator-rules-matrix.md)

后续 migration、query、sqlc、logic、worker 改造应优先以本文为数据合同。

## 2. 现状缺口

### 2.1 behavior_decisions 只够表达一版简单判责

当前 [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql) 和 [locallife/db/query/behavior_trace.sql](locallife/db/query/behavior_trace.sql) 中的 `behavior_decisions` 只能表达：

1. 一个责任方 `responsible_party`。
2. 一个赔付来源 `compensation_source`。
3. 一个粗粒度状态 `decision_status`。
4. 一段 `trace_summary` 文本。

它缺少主判 V2 必需的正式字段：

1. `decision_mode`，无法区分 `merchant_recovery`、`rider_recovery`、`platform_fallback`、`user_restricted`。
2. `claim_id`，无法把判决作为索赔的强主键关联对象。
3. `confidence_score/U/M/R` 四类分数，无法解释系统为什么这样判。
4. `fallback_reason` 和 `restriction_reason`，无法解释为什么兜底或限制。
5. `supersedes/overturned` 关系，无法表达再裁决和回滚链。

### 2.2 behavior_trace_snapshots 只存了两档浅快照

当前 [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go) 只会写 7d 和 30d 两条快照，而且只有：

1. `window_days`
2. `abnormal_count`
3. `total_count`
4. `abnormal_rate`
5. 永远为空的 `association_hits`

这不足以支撑：

1. 用户、商户、骑手三方对称快照。
2. 原始计数和净有效计数分离。
3. 图谱命中、关系强度、独立用户数等结构化事实。
4. 申诉翻盘后的可复盘回看。

### 2.3 claim_recoveries 没有判决主键和事件历史

当前 [locallife/db/migration/000119_add_claim_recoveries_and_remove_evidence.up.sql](locallife/db/migration/000119_add_claim_recoveries_and_remove_evidence.up.sql) 的 `claim_recoveries` 已经能表达追偿单，但还不够：

1. 没有 `decision_id`，只能靠 `decision_snapshot` 间接保存来源。
2. 没有事件表，`waive`、`resume`、`paid`、`overturned` 都只能隐含在状态变化里。
3. 无法清晰区分“运营核销”和“申诉翻盘回滚”。

### 2.4 画像和窗口统计仍然偏 raw claim 口径

当前 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql) 与 [locallife/db/query/abnormal_stats.sql](locallife/db/query/abnormal_stats.sql) 主要还是围绕：

1. 裸 claim 次数。
2. 总订单数。
3. 简单 abnormal rate。

它没有显式区分：

1. `claim_attempts`
2. `effective_claims`
3. `platform_fallback_claims`
4. `overturned_claims`
5. `malicious_confirmed_claims`

这会直接导致用户、商户、骑手画像继续被旧口径污染。

## 3. V2 设计原则

### 3.1 一笔 claim 只能有一个当前生效主判

可以有历史判决、再裁决、翻盘记录，但同一时间只能有一个 effective decision。

### 3.2 主判结果、动作执行、画像净值必须拆开

至少拆成三层：

1. 判决层：系统做了什么判断。
2. 动作层：系统执行了什么赔付、追偿、限制、通知动作。
3. 画像层：这个判决对用户、商户、骑手净值造成了什么影响。

否则申诉翻盘时只能“猜着回滚”。

### 3.3 保留旧字段做兼容别名，不在第一阶段强拆

第一阶段应采用 additive migration：

1. 先新增 V2 字段和 V2 表。
2. 双写旧语义和新语义。
3. 待 logic 和查询完全切换后再考虑下线旧字段依赖。

### 3.4 平台兜底和用户限制必须是一等持久化语义

不能继续只靠：

1. `responsible_party = platform_fallback`
2. `compensation_source = platform`
3. `reason_codes`

来拼凑出正式含义。V2 需要显式字段承载。

## 4. 核心表改造

## 4.1 behavior_decisions 升级为主判主表

建议保留现有表名 `behavior_decisions`，直接升级为主判主表，而不是新建平行表。

原因：

1. 当前 claim 交易事务已经接入它。
2. 已有 action、snapshot、appeal 等关联表围绕它建立。
3. 改名或重建会带来不必要的 query/sqlc 破坏面。

### 建议新增字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `claim_id` | `BIGINT` | 主判对应的 claim，异常订单主判场景应强关联 |
| `decision_mode` | `TEXT` | 正式模式：`merchant_recovery`、`rider_recovery`、`platform_fallback`、`user_restricted`，保留 `split` 作为扩展值 |
| `responsibility_domain` | `TEXT` | `merchant_domain`、`rider_domain`、`user_domain`、`unknown` |
| `payout_mode` | `TEXT` | `instant_paid`、`limited_paid`、`rejected` |
| `effective_status` | `TEXT` | `effective`、`overturned`、`superseded`、`archived` |
| `confidence_score` | `INT` | `C` 分数，0-100 |
| `user_risk_score` | `INT` | `U` 分数，0-100 |
| `merchant_liability_score` | `INT` | `M` 分数，0-100 |
| `rider_liability_score` | `INT` | `R` 分数，0-100 |
| `fallback_reason` | `TEXT` | 平台兜底原因码 |
| `restriction_reason` | `TEXT` | 用户限制原因码 |
| `liability_shares` | `JSONB` | 责任份额，当前版本通常为空对象，给未来扩展留接口 |
| `score_breakdown` | `JSONB` | 分项打分明细 |
| `graph_hits` | `JSONB` | 图谱命中详情 |
| `fact_snapshot` | `JSONB` | 订单、画像、关系、状态链的关键事实快照 |
| `supersedes_decision_id` | `BIGINT` | 再裁决时指向被替代判决 |
| `overturned_by_decision_id` | `BIGINT` | 被哪一条新判决推翻 |
| `profile_effect_applied` | `BOOLEAN` | 画像净值是否已入账 |

### 建议保留的旧字段

| 字段 | 去留建议 | 原因 |
| --- | --- | --- |
| `responsible_party` | 保留 | 作为兼容别名，旧 worker 和 recovery 逻辑仍会读取 |
| `compensation_source` | 保留 | 作为兼容别名，旧响应结构仍可能使用 |
| `decision_status` | 保留但弱化 | 逐步由 `effective_status` 和 action 状态替代 |
| `trace_summary` | 保留 | 作为人工可读摘要，不替代结构化快照 |

### 推荐约束

1. `claim_id` 对异常订单主判应非空。
2. 同一 `claim_id` 只能有一条 `effective_status = 'effective'` 的记录。
3. `decision_mode = 'platform_fallback'` 时，`fallback_reason` 必填。
4. `decision_mode = 'user_restricted'` 时，`restriction_reason` 必填。
5. `effective_status = 'superseded'` 时，`supersedes_decision_id` 必填。

### 推荐索引

1. `(claim_id)`
2. `(claim_id) WHERE effective_status = 'effective'`
3. `(decision_mode, effective_status, created_at DESC)`
4. `(user_id, created_at DESC)`
5. `(merchant_id, created_at DESC)`
6. `(rider_id, created_at DESC)`

## 4.2 behavior_trace_snapshots 升级为三方结构化快照表

建议继续保留 `behavior_trace_snapshots`，但把它从“简单窗口计数表”升级为“结构化快照表”。

### 建议新增字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `actor_type` | `TEXT` | `user`、`merchant`、`rider` |
| `actor_id` | `BIGINT` | 对应主体 ID |
| `window_key` | `TEXT` | `7d`、`30d`、`90d`、`lifetime` |
| `stats_scope` | `TEXT` | `raw`、`net_effective` |
| `metric_payload` | `JSONB` | 结构化指标快照 |
| `association_payload` | `JSONB` | 结构化关系命中，不再只用 `TEXT[]` |
| `snapshot_version` | `TEXT` | 快照版本号 |

### metric_payload 最低应包含的键

#### user

1. `claim_attempts`
2. `effective_claims`
3. `platform_fallback_claims`
4. `overturned_claims`
5. `malicious_confirmed_claims`
6. `paid_claim_amount`
7. `distinct_devices`
8. `distinct_addresses`

#### merchant

1. `completed_orders`
2. `effective_liability_claims`
3. `platform_fallback_claims`
4. `overturned_liability_claims`
5. `distinct_claim_users`
6. `foreign_object_claims`

#### rider

1. `completed_orders`
2. `effective_liability_claims`
3. `platform_fallback_claims`
4. `overturned_liability_claims`
5. `distinct_claim_users`
6. `damage_claims`
7. `timeout_claims`

### 兼容策略

第一阶段不删除以下旧列：

1. `window_days`
2. `abnormal_count`
3. `total_count`
4. `abnormal_rate`
5. `association_hits`

但新代码不应再把它们当作主判唯一事实来源。

## 4.3 behavior_actions 继续做动作执行表

`behavior_actions` 当前方向是对的，应继续作为动作执行表，不建议重建。

但建议补两点：

1. `action_type` 扩到能表达 `payout`、`recovery_create`、`restrict`、`warn`、`notify`。
2. `detail` 里统一带 `decision_mode`、`claim_id`、`recovery_id`、`reason_code`，避免动作和判决脱钩。

这里不承担画像净值回滚语义，画像净值应落到新表。

## 5. 新增表

## 5.1 behavior_decision_effects

这是 V2 最关键的新表，用于把“判决对画像造成的净影响”显式落库。

如果没有这张表，申诉翻盘后仍然只能靠业务代码猜测该减回哪些计数。

### 建议字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `BIGSERIAL` | 主键 |
| `decision_id` | `BIGINT` | 来源判决 |
| `entity_type` | `TEXT` | `user`、`merchant`、`rider` |
| `entity_id` | `BIGINT` | 主体 ID |
| `metric_key` | `TEXT` | 例如 `effective_claims`、`merchant_recovery_claims` |
| `delta_value` | `INT` | 增量，可正可负 |
| `status` | `TEXT` | `applied`、`reverted` |
| `applied_at` | `TIMESTAMPTZ` | 入账时间 |
| `reverted_at` | `TIMESTAMPTZ` | 回滚时间 |
| `reverted_by_decision_id` | `BIGINT` | 哪条再裁决导致回滚 |
| `note` | `TEXT` | 备注 |

### 作用

1. 所有画像净值更新都先写 effect，再汇总入画像表。
2. 翻盘时只需要把对应 effect 标记为 reverted，再做反向汇总。
3. 这张表本身就是净值审计账本。

## 5.2 claim_recovery_events

当前 `claim_recoveries` 只有结果状态，没有过程事件。V2 建议新增事件表。

### 建议字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `BIGSERIAL` | 主键 |
| `recovery_id` | `BIGINT` | 追偿单 |
| `decision_id` | `BIGINT` | 触发它的判决 |
| `event_type` | `TEXT` | `created`、`paid`、`waived`、`resumed`、`overturned` |
| `payload` | `JSONB` | 事件细节 |
| `created_at` | `TIMESTAMPTZ` | 事件时间 |

### 作用

1. 区分运营核销与申诉翻盘。
2. 让恢复追偿、撤销追偿、重新生效都有审计链。
3. 为后续 recovery 状态机重构留底。

## 5.3 主判画像表

建议不要继续把 V2 主判画像完全塞进现有 `merchant_profiles` 和 `rider_profiles`。

更稳妥的做法是新增三张主判画像表：

1. `user_claim_profiles`
2. `merchant_claim_profiles`
3. `rider_claim_profiles`

### 最低字段集合

| 字段 | 说明 |
| --- | --- |
| `entity_id` | 主体 ID |
| `claim_attempts_total` | 提交索赔总数 |
| `effective_claims_total` | 成立索赔总数 |
| `platform_fallback_total` | 平台兜底总数 |
| `overturned_total` | 被翻盘总数 |
| `malicious_confirmed_total` | 确认恶意总数，仅 user 需要强使用 |
| `merchant_recovery_total` | 最终追商户总数 |
| `rider_recovery_total` | 最终追骑手总数 |
| `last_decision_at` | 最近一次判决时间 |
| `updated_at` | 更新时间 |

### 说明

1. 画像表承载生命周期累计值。
2. 窗口统计仍交给日汇总表或快照表。
3. 画像表数据来源必须来自 `behavior_decision_effects` 汇总，而不是散落在 handler/logic 中手改。

## 6. 现有表的补强

## 6.1 claim_recoveries 增加 decision_id

建议在 `claim_recoveries` 上新增：

1. `decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id)`
2. `recovery_basis TEXT`，值可为 `merchant_recovery`、`rider_recovery`

这样 recovery 就不再只靠 `decision_snapshot` 追溯来源。

## 6.2 abnormal_stats_daily 从 raw claims 进化为 outcome 窗口表

当前 [locallife/db/query/abnormal_stats.sql](locallife/db/query/abnormal_stats.sql) 里的 `abnormal_stats_daily` 仍以 `abnormal_claims` 为核心，这会继续放大旧口径。

建议扩展为：

1. `claim_attempts`
2. `effective_claims`
3. `platform_fallback_claims`
4. `overturned_claims`
5. `malicious_confirmed_claims`
6. `merchant_recovery_claims`
7. `rider_recovery_claims`
8. `distinct_claim_users`

兼容期内：

1. `abnormal_claims` 可先映射为 `effective_claims + platform_fallback_claims` 或继续保留旧值。
2. 新主判逻辑不应再直接只读 `abnormal_claims`。

## 7. 兼容映射

为了减少第一阶段改造面，建议明确以下兼容映射：

| `decision_mode` | `responsible_party` 兼容值 | `compensation_source` 兼容值 |
| --- | --- | --- |
| `merchant_recovery` | `merchant` | `platform` |
| `rider_recovery` | `rider` | `platform` |
| `platform_fallback` | `platform_fallback` | `platform` |
| `user_restricted` | `user` | `unknown` 或按限赔策略写 `platform` |

说明：

1. `responsible_party` 不再等价于正式主判模式，只作为兼容字段。
2. 业务代码应逐步改读 `decision_mode`，不要继续拿 `responsible_party` 推导全部语义。

## 8. 最小迁移顺序

## 8.1 Phase A：纯加法 migration

先做不破坏旧逻辑的 schema 变更：

1. 给 `behavior_decisions` 加 V2 字段。
2. 给 `behavior_trace_snapshots` 加 V2 字段。
3. 给 `claim_recoveries` 加 `decision_id` 和 `recovery_basis`。
4. 新增 `behavior_decision_effects`。
5. 新增 `claim_recovery_events`。
6. 新增三张主判画像表。

这一阶段完成后需要：

1. 更新 `db/query/*.sql`
2. 执行 `make sqlc`

## 8.2 Phase B：事务层双写

在 [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go) 里实现双写：

1. 继续写旧字段。
2. 同时写 `decision_mode/U/M/R/C/fallback_reason/graph_hits/score_breakdown`。
3. 同时写三方结构化快照。
4. 同时写 `behavior_decision_effects`。
5. 创建 recovery 时写 `claim_recoveries.decision_id` 和 `claim_recovery_events.created`。

## 8.3 Phase C：读取层切换

在 API、logic、worker 层逐步完成：

1. 主判读取 `decision_mode` 而不是 `responsible_party`。
2. 画像统计读取净有效口径而不是裸 claim 口径。
3. 申诉翻盘直接按 `behavior_decision_effects` 做回滚。

## 8.4 Phase D：回填和清理

仅在新链路稳定后再做：

1. 回填历史 decision 的 V2 字段能回多少回多少。
2. 清理旧口径 query 对主判的影响。
3. 评估是否下线 `decision_status` 的核心语义。

## 9. 当前最推荐的落库切口

如果下一步只做第一批真正可交付的代码改造，优先级建议如下：

1. `behavior_decisions` 增加 `claim_id`、`decision_mode`、`U/M/R/C`、`fallback_reason`、`restriction_reason`、`score_breakdown`、`graph_hits`。
2. 新增 `behavior_decision_effects`，先把净值账本建立起来。
3. `claim_recoveries` 增加 `decision_id`。
4. `behavior_trace_snapshots` 增加 `actor_type/window_key/stats_scope/metric_payload`。

这四步完成后，主判 V2 就已经有了：

1. 可解释判决主记录。
2. 可回滚净值账本。
3. 可追溯追偿来源。
4. 可复盘的三方快照。

后面的画像表和日汇总扩展，可以作为第二批改造继续推进。

## 10. 结论

异常订单主判 V2 的数据库核心，不是再往 `behavior_decisions` 塞几个 reason code，而是把四层语义彻底拆清：

1. `behavior_decisions` 负责正式裁决结果。
2. `behavior_actions` 负责动作执行。
3. `behavior_decision_effects` 负责画像净值和回滚账本。
4. `behavior_trace_snapshots` 负责判决时点的三方结构化快照。

只有这四层拆开，平台兜底、用户限制、申诉翻盘、画像回滚才会变成稳定的数据库能力，而不是散落在代码里的临时规则。