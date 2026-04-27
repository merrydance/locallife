# TASK-PAY-005B Receiver Intent / Attempt Persistence Design

日期：2026-04-25

## 1. 目标

为微信分账接收方 lifecycle 建立持久化锚点设计，使 operator/rider receiver ensure/delete 可以从主业务链路中拆出，进入可重试、可观测、可人工恢复的异步边界。

本设计不直接改 schema，不改现有同步调用，不新增 worker。本阶段输出供后续 TASK-PAY-005B-implementation 或 TASK-PAY-005C/D 使用。

## 2. 为什么不复用现有表

不复用 `external_payment_commands`：

- `external_payment_commands` 记录支付、退款、分账、提现、补差等外部命令同步返回。
- receiver membership 是身份/生命周期副作用，不是某个 payment command 的同步返回。
- receiver target 的去重键是 appid + receiver_type + account + owner，不是 out_trade_no/out_refund_no/out_order_no。

不直接复用 `payment_domain_outbox`：

- outbox 适合表达“有一个事件待发布/处理”，但 receiver lifecycle 还需要表达 desired_state、target key、当前微信同步状态、幂等成功、可人工跳过、最后错误分类等长期状态。
- 可以在实现时通过 outbox 触发 worker，但 receiver 自身仍需要单独 intent/attempt 真值表。

## 3. 建议 schema

### 3.1 `profit_sharing_receiver_targets`

表示本地认为某个 owner 对某个微信 receiver target 的期望状态。

建议字段：

| 字段 | 类型 | 语义 |
| --- | --- | --- |
| `id` | BIGSERIAL PK | target id |
| `provider` | TEXT NOT NULL | 固定 `wechat` |
| `channel` | TEXT NOT NULL | 固定 `ecommerce` |
| `owner_type` | TEXT NOT NULL | `rider` / `operator` / `manual` |
| `owner_id` | BIGINT NOT NULL | rider id / operator id / manual request id |
| `receiver_type` | TEXT NOT NULL | `PERSONAL_OPENID` / `MERCHANT_ID` |
| `appid` | TEXT NOT NULL | 微信 appid，当前为 sp appid |
| `account_hash` | TEXT NOT NULL | receiver account 的哈希，用于索引和日志 |
| `account_ciphertext` | TEXT | 可选，若后续需要重放且不能从 owner 重新解析 account 才考虑；默认不建议保存 |
| `display_name_hash` | TEXT | 可选，仅用于变更检测，不保存明文 name |
| `desired_state` | TEXT NOT NULL | `present` / `absent` |
| `sync_status` | TEXT NOT NULL | `pending` / `processing` / `synced` / `failed` / `skipped` |
| `attempt_count` | INT NOT NULL DEFAULT 0 | 执行次数 |
| `next_retry_at` | TIMESTAMPTZ | 下次重试时间 |
| `last_error_code` | TEXT | 微信或本地分类错误码 |
| `last_error_message` | TEXT | 脱敏错误摘要 |
| `last_attempt_at` | TIMESTAMPTZ | 最近尝试时间 |
| `synced_at` | TIMESTAMPTZ | 达到 desired_state 的时间 |
| `skipped_at` | TIMESTAMPTZ | 被跳过时间 |
| `created_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | 创建时间 |
| `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | 更新时间 |

建议约束：

- `channel IN ('ecommerce')`
- `owner_type IN ('rider', 'operator', 'manual')`
- `receiver_type IN ('PERSONAL_OPENID', 'MERCHANT_ID')`
- `desired_state IN ('present', 'absent')`
- `sync_status IN ('pending', 'processing', 'synced', 'failed', 'skipped')`
- `attempt_count >= 0`
- `length(trim(account_hash)) > 0`
- unique：`(provider, channel, owner_type, owner_id, receiver_type, appid, account_hash)`

说明：

- 默认不保存 openid 明文；worker 可从 owner 表重新解析 user -> wechat_openid，再调用微信。
- 如后续发现 owner 数据可能变化导致重放不稳定，再单独设计加密存储，不在第一版扩大敏感面。

### 3.2 `profit_sharing_receiver_attempts`

表示每次对微信 Add/DeleteReceiver 的执行记录，便于审计和失败排查。

建议字段：

| 字段 | 类型 | 语义 |
| --- | --- | --- |
| `id` | BIGSERIAL PK | attempt id |
| `target_id` | BIGINT NOT NULL REFERENCES `profit_sharing_receiver_targets(id)` | receiver target |
| `action` | TEXT NOT NULL | `ensure` / `delete` |
| `status` | TEXT NOT NULL | `processing` / `succeeded` / `failed` / `skipped` |
| `idempotent_success` | BOOLEAN NOT NULL DEFAULT false | already exists / not exists 是否视为成功 |
| `error_code` | TEXT | 错误码 |
| `error_message` | TEXT | 脱敏错误摘要 |
| `started_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | 开始时间 |
| `finished_at` | TIMESTAMPTZ | 结束时间 |
| `created_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | 创建时间 |

建议约束：

- `action IN ('ensure', 'delete')`
- `status IN ('processing', 'succeeded', 'failed', 'skipped')`

说明：

- attempts 是审计日志，不作为业务是否可分账的唯一判断；target 的 `sync_status` 才是当前状态。
- attempt 不保存完整 request/response payload，不保存 receiver_name 明文。

## 4. 建议 sqlc query

第一批 query：

- `UpsertProfitSharingReceiverTarget`
  - 以 unique key upsert target，更新 desired_state、sync_status=`pending`、next_retry_at=now()。
  - 当相同 target 已 `processing` 时允许更新 desired_state，但 worker claim 时应重新读取最新 desired_state。
- `ClaimPendingProfitSharingReceiverTargets`
  - 按 `sync_status IN ('pending', 'failed') AND (next_retry_at IS NULL OR next_retry_at <= now())` 扫描。
  - `ORDER BY next_retry_at NULLS FIRST, id ASC LIMIT $1 FOR UPDATE SKIP LOCKED`。
  - claim 时更新为 `processing`、`attempt_count = attempt_count + 1`、`last_attempt_at=now()`。
- `CreateProfitSharingReceiverAttempt`
- `MarkProfitSharingReceiverTargetSynced`
- `MarkProfitSharingReceiverTargetFailed`
- `MarkProfitSharingReceiverTargetSkipped`
- `ListProfitSharingReceiverTargetsByOwner`
- `GetProfitSharingReceiverTarget`

## 5. 事件到 target 的映射

| 事件 | owner_type | desired_state | target source | 后续阶段 |
| --- | --- | --- | --- | --- |
| operator approved / active | `operator` | `present` | operator.user_id -> users.wechat_openid, operator contact/name | TASK-PAY-005C |
| operator suspended / expired | `operator` | `absent` | operator.user_id -> users.wechat_openid | TASK-PAY-005C |
| rider deposit paid | `rider` | `present` | rider.user_id -> users.wechat_openid, rider real_name | TASK-PAY-005D |
| rider deposit refunded to zero | `rider` | `absent` | rider.user_id -> users.wechat_openid | TASK-PAY-005D |
| manual repair | `manual` or original owner | `present` / `absent` | explicit owner context | TASK-PAY-005E |

## 6. Worker 语义

Worker claim target 后：

1. 重新加载 owner 和 user/openid。
2. 如果 owner 已不存在或已不需要 receiver，标记 `skipped`。
3. 如果缺 openid，标记 `failed` 或 `skipped`，取决于 owner 是否仍 active：
   - active owner 缺 openid：`failed`，需要人工补资料。
   - inactive/absent owner 缺 openid：`skipped`，因为无需确保 receiver 存在。
4. desired_state=`present` 调 `AddProfitSharingReceiver`。
5. desired_state=`absent` 调 `DeleteProfitSharingReceiver`。
6. already exists / not exists 视为 `synced`，attempt 标记 `idempotent_success=true`。
7. 微信 5xx、timeout、rate limit：`failed` 并设置退避重试。
8. PARAM_ERROR、NO_AUTH、receiver high risk、实名错误：`failed`，需要可见告警和人工介入。

## 7. API / 手工入口边界

后续手工入口不应再以 payment_order 为主 owner。

建议：

- `POST /v1/admin/profit-sharing-receivers/:target_id/retry`
- `POST /v1/admin/operators/:operator_id/profit-sharing-receiver/sync`
- `POST /v1/admin/riders/:rider_id/profit-sharing-receiver/sync`

这些入口只写 target pending 或触发 worker，不直接在 handler 中调用微信。

## 8. 验证计划

005B implementation 最低测试：

- sqlc：target upsert 去重、desired_state 覆盖、claim 顺序和 `FOR UPDATE SKIP LOCKED`。
- logic：operator/rider target 构造不保存明文 receiver name；missing openid 分流。
- worker：already exists / not exists 幂等成功；5xx 失败进入 retry；PARAM_ERROR 进入 failed 且保留脱敏摘要。

## 9. Review 结论

本设计保持了 TASK-PAY-005 的边界：只处理 receiver lifecycle，不处理 payment command、payment fact、profit sharing result、subsidy 或 complaint/violation operational fact。

下一段若进入实现，应先创建 migration/query/sqlc 的 TASK-PAY-005B-1，小步落地 target/attempt 表和最小 query，不触碰 operator/rider 现有同步调用。
