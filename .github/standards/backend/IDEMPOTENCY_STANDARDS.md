# Backend Idempotency Standards

本文件定义 LocalLife 后端幂等治理的长期分类规则。适用于 `locallife/` 中涉及重复提交、外部单号、callback、worker、scheduler、recovery、OCR、媒体上传、支付、退款、分账、赔付等路径的设计、实现和 review。

## 1. 核心原则

不要把所有幂等键统一迁入一个中心业务实体。

LocalLife 的稳定目标是：领域唯一性分散，入口幂等治理统一。

- 领域表继续拥有外部契约键、自然资源键、状态机恢复锚点和最终唯一约束。
- request-level guard 只治理客户端或上游重复提交同一个写请求的场景。
- callback、worker、scheduler、recovery 的重复执行语义由事实记录、任务记录、状态条件、领域表唯一约束或条件更新保证，不降级成普通 HTTP `Idempotency-Key`。

## 2. 必须先分类

新增或修改任何幂等相关逻辑前，必须先把 key 或重复执行语义归入下表之一。

| 类型 | 判断标准 | 默认落点 | 例子 | request-level guard |
| --- | --- | --- | --- | --- |
| 外部契约键 | 第三方接口、回调、查询、对账依赖该字段 | 领域表唯一约束 | `out_trade_no`、`combine_out_trade_no`、`out_refund_no`、`out_order_no`、`out_return_no`、`out_subsidy_no` | 不迁入；最多记录请求到资源的绑定 |
| 领域资源自然键 | key 本身定义一个业务资源、任务或未完成资源 | 领域表唯一约束或条件唯一索引 | OCR job key、媒体 pending upload session | 通常不接入 |
| 请求重放键 | 客户端或上游重试同一个写请求 | request-level guard + 业务表最终兜底 | 会员代充值 `Idempotency-Key` | 候选 |
| 异步重复投递 | webhook、worker、scheduler、recovery 重复触发 | callback inbox、事实记录、任务记录、状态条件或领域表唯一约束 | 微信回调、退款恢复、timeout task | 不接入 |
| 自然去重查询 | 无显式 key，但业务条件定义同一未完成资源 | 领域查询 + 唯一/条件索引 | 同用户同类别同 checksum 的 pending upload session | 通常不接入 |

未分类的 G2/G3 写路径不得直接实现中心幂等表、`Idempotency-Key`、新的 `idempotency_key` 字段或重复执行分支。

## 3. 外部契约键规则

外部契约键不能从领域表迁走，也不能只存到 request-level guard 中。

必须保留在领域表中的字段包括但不限于：

- 支付：`out_trade_no`、`combine_out_trade_no`、合单子单 `out_trade_no`。
- 退款：`out_refund_no`。
- 分账：`out_order_no`、`out_return_no`。
- 补差：`out_subsidy_no`、`out_return_no`。
- 转账、提现、进件等外部通道的 `out_*` / `*_id` 键，按对应领域表定义。

这些字段必须继续支持外部查询、回调归属校验、对账、恢复和人工排查。

## 4. Request-Level Guard 候选规则

只有满足以下条件的入口，才是 request-level guard 候选：

- 调用方能稳定提供同一个请求重放 key。
- 同 key 同请求应该返回同一资源或等价响应。
- 同 key 不同请求必须返回冲突，而不是覆盖旧请求。
- 业务效果能由领域表唯一约束、状态条件或事务最终兜底。
- 响应重建不需要存储敏感完整响应，或可以定义最小 snapshot allowlist。

每个候选 scope 必须在设计中写清楚：

- `operation_scope`。
- actor 来源和权限边界。
- canonical request fields。
- 同 key 不同 hash 的冲突语义。
- `processing` 重放语义。
- TTL 和 snapshot 策略。
- 领域表最终唯一约束。

## 5. Request Hash 规则

参与 request hash 的字段必须覆盖会改变业务效果、资金、权限、状态或目标资源的字段。

通常应参与：金额、币种、目标用户/会员/商户、业务资源 ID、外部 channel、关键备注、source、会影响外部副作用的选项。

通常不应参与：trace id、request id、客户端当前时间、nonce、展示用冗余字段、服务端可从可信上下文重新推导且不改变业务效果的字段。

每个 request-level guard scope 后续实现时，必须至少有：

- 同 key 同 hash 重放测试。
- 同 key 不同 hash 冲突测试。
- 并发同 key 只有一个请求执行副作用的测试或明确验证说明。

## 6. 响应与敏感数据

默认不要存完整 response snapshot。

优先级：

1. 存 `resource_type/resource_id`，重放时从业务表重建响应。
2. 仅在响应无法稳定重建、且字段安全时存最小 snapshot。
3. 严禁存 token、签名私钥、微信原始敏感 payload、身份证号、银行卡号、手机号全量、证件图片地址、raw provider payload。

## 7. 异步重复执行规则

callback、worker、scheduler、timeout、recovery 不能只假设“只会执行一次”。

设计和 review 必须明确：

- 重复投递或重复领取的 dedupe key 或 claim 条件。
- 业务状态前置条件和 0-row conditional update 语义。
- 已终态、处理中、失败重试、未知状态分别如何处理。
- 外部成功但本地后续失败时如何由 query、callback、recovery 或人工入口收敛。

对支付、退款、分账、赔付、提现、进件等 G3 路径，入口 request-level guard 不能替代 callback/query/recovery 的幂等和终态收敛设计。

## 8. 当前阶段边界

当前阶段只要求完成 inventory、分类矩阵、标准和 review gate。不要从本标准直接推导出必须立刻创建 `idempotency_records` 或改造现有入口。

当前 inventory 产物：`artifacts/idempotency-scope-inventory-2026-04-25.md`。