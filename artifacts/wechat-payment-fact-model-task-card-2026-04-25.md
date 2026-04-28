# TASK-PAY-002 统一支付事实模型任务卡

日期：2026-04-25

## 1. 目标

建立一层统一的 external payment facts，让微信回调、主动查询、人工对账都进入同一个事实入口，再由业务域幂等消费事实。

本任务只定义模型和迁移边界，不迁移业务状态，也不重写微信客户端。

## 2. 设计原则

### 2.0 幂等治理分类

本模型属于 `.github/standards/backend/IDEMPOTENCY_STANDARDS.md` 中的“异步重复投递 + 外部契约键”治理，不是 request-level guard。

- `external_payment_facts.dedupe_key` 只治理 callback、query、manual reconciliation 得到的外部事实去重。
- `external_payment_commands` 只记录本系统向微信提交过的命令，不替代 `idempotency_records` 这类请求级幂等登记能力。
- `out_trade_no`、`out_refund_no`、`out_order_no`、`out_return_no` 等外部契约键继续保留在领域表中，事实层只引用这些键做归属、查询和审计。
- 后续支付创建、退款创建等入口如果需要 request-level guard，应另按幂等标准定义 operation scope、request hash 和 replay/conflict 语义。

### 2.1 保留现有业务状态表

现有表仍然是业务状态主表：

- `payment_orders`
- `combined_payment_orders`
- `refund_orders`
- `profit_sharing_orders`
- `profit_sharing_returns`
- `ecommerce_applyments`
- `withdrawal_records`
- `merchant_cancel_withdraw_applications`
- `rider_deposit_credits`
- claim payout 使用的 `behavior_actions`

新增事实层不替换这些表，而是先做旁路事实记录和统一幂等入口。业务迁移完成前，旧状态表继续对 API 和业务查询负责。

### 2.2 事实不可直接等于业务完成

`external_payment_facts` 只记录微信侧或外部通道侧发生了什么。业务完成必须由 domain handler 写入业务表。

例子：

- 微信退款 `SUCCESS` fact 只是外部退款成功；骑手押金域还要结算 credit、frozen deposit、流水和 stale credit reconciliation。
- 微信支付 `SUCCESS` fact 只是外部支付成功；订单域还要推进订单、预订或追偿状态。
- 微信进件 `FINISH` fact 只是进件外部终态；商户域还要决定商户是否可收款、是否还要验结算账户。

### 2.3 先统一终态入口，再逐步迁移业务域

第一阶段允许事实写入后继续调用现有 worker/service，以降低风险。目标是先让 callback/query/manual 都能形成同一种 fact，再逐步把 `ProcessTaskPaymentSuccess`、`ProcessTaskRefundResult` 等业务推进逻辑拆到 domain handler。

## 3. 建议对象模型

### 3.1 `external_payment_commands`

记录本系统向微信提交过的外部命令。它不是终态表，只表达 command submitted/accepted/rejected/unknown。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigserial | 主键 |
| `provider` | text | `wechat` |
| `channel` | text | `direct` / `ecommerce` |
| `capability` | text | `direct_jsapi_payment`, `direct_refund`, `partner_jsapi_payment`, `combine_payment`, `ecommerce_refund`, `profit_sharing`, `applyment`, `withdraw`, `cancel_withdraw`, `merchant_transfer` |
| `command_type` | text | `create_payment`, `close_payment`, `create_refund`, `create_profit_sharing`, `finish_profit_sharing`, `create_applyment`, `create_withdraw`, `create_transfer` |
| `business_owner` | text | `rider_deposit`, `order`, `reservation`, `claim_recovery`, `profit_sharing`, `applyment`, `merchant_finance` |
| `business_object_type` | text | 本地业务对象类型，例如 `payment_order`, `refund_order`, `ecommerce_applyment` |
| `business_object_id` | bigint | 本地业务对象 ID，可为空用于合单或人工命令 |
| `external_object_type` | text | 外部对象类型，例如 `payment`, `refund`, `profit_sharing`, `withdraw`, `transfer` |
| `external_object_key` | text | 主幂等键，例如 `out_trade_no`, `out_refund_no`, `out_order_no`, `out_request_no`, `out_bill_no` |
| `external_secondary_key` | text | 微信返回键，例如 `transaction_id`, `refund_id`, `applyment_id`, `withdraw_id`, `transfer_bill_no` |
| `command_status` | text | `submitted`, `accepted`, `rejected`, `unknown` |
| `submitted_at` | timestamptz | 本地提交时间 |
| `accepted_at` | timestamptz | 微信明确受理时间，可空 |
| `rejected_at` | timestamptz | 同步拒绝时间，可空 |
| `last_error_code` | text | 微信错误码或本地错误分类，可空 |
| `last_error_message` | text | 面向运维的错误摘要，可空 |
| `request_fingerprint` | text | 请求字段摘要，不存敏感明文 |
| `response_snapshot` | jsonb | 脱敏后的同步响应，禁止存证件号、银行卡号、密钥、原始签名 |
| `created_at` / `updated_at` | timestamptz | 时间戳 |

建议唯一约束：

- `(provider, channel, capability, command_type, external_object_type, external_object_key)` 唯一。
- 如果同一外部对象允许多类命令，例如分账和解冻，用 `command_type` 区分。

第一阶段可以不强制所有提交路径都先接入 command 表，但骑手押金样板应接入。

### 3.2 `external_payment_facts`

记录回调、主动查询或人工对账得到的外部事实。事实可以是中间态，也可以是终态。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigserial | 主键 |
| `provider` | text | `wechat` |
| `channel` | text | `direct` / `ecommerce` |
| `capability` | text | 能力组，枚举同 command |
| `fact_source` | text | `callback`, `query`, `manual_reconciliation` |
| `source_event_id` | text | callback 使用微信 notification id；query/manual 可空 |
| `source_event_type` | text | 微信 event_type 或本地事件类型 |
| `external_object_type` | text | `payment`, `combined_payment`, `refund`, `profit_sharing`, `profit_sharing_return`, `applyment`, `withdraw`, `cancel_withdraw`, `merchant_transfer`, `complaint`, `violation`, `settlement` |
| `external_object_key` | text | `out_trade_no`, `combine_out_trade_no`, `out_refund_no`, `out_order_no`, `out_return_no`, `out_request_no`, `out_bill_no`, `complaint_id` |
| `external_secondary_key` | text | 微信侧 ID，例如 `transaction_id`, `refund_id`, `sharing_order_id`, `applyment_id`, `withdraw_id`, `transfer_bill_no` |
| `business_owner` | text | 事实建议消费方，例如 `rider_deposit`, `order`, `reservation`, `claim_recovery` |
| `business_object_type` | text | 本地对象类型，可空 |
| `business_object_id` | bigint | 本地对象 ID，可空，无法立刻解析时允许后续补齐 |
| `upstream_state` | text | 微信原始状态，例如 `SUCCESS`, `PROCESSING`, `CLOSED`, `FINISH`, `REJECTED` |
| `terminal_status` | text | 归一状态：`success`, `failed`, `closed`, `expired`, `processing`, `unknown` |
| `is_terminal` | boolean | `success/failed/closed/expired` 为 true，`processing/unknown` 为 false |
| `amount` | bigint | 相关金额，单位分，可空 |
| `currency` | text | 默认 `CNY` |
| `occurred_at` | timestamptz | 微信事件实际发生时间，可空 |
| `upstream_updated_at` | timestamptz | 微信状态更新时间，可空 |
| `observed_at` | timestamptz | 本系统观察到该事实的时间 |
| `raw_resource` | jsonb | 脱敏后的关键响应，禁止存敏感明文 |
| `dedupe_key` | text | 归一后的事实幂等键 |
| `processing_status` | text | `received`, `terminalized`, `ignored`, `failed` |
| `processing_error` | text | terminalizer 处理失败摘要 |
| `processed_at` | timestamptz | terminalizer 处理完成时间，可空 |
| `created_at` / `updated_at` | timestamptz | 时间戳 |

建议唯一约束：

- `dedupe_key` 全局唯一。
- callback dedupe key：`wechat:callback:<callback_type>:<notification_id>`。
- query dedupe key：`wechat:query:<channel>:<external_object_type>:<external_object_key>:<terminal_status>:<upstream_updated_at_or_observed_bucket>`。
- manual dedupe key：`wechat:manual:<external_object_type>:<external_object_key>:<terminal_status>:<operator_id>:<manual_batch_id>`。

说明：query 来源可能会重复看到同一个 PROCESSING。为了避免大量重复事实，处理中事实可以按时间桶去重，例如 10 分钟或 30 分钟；终态事实必须按外部对象和终态唯一化。

### 3.3 `external_payment_fact_applications`

记录某个 fact 被哪个业务域消费以及消费结果。它解决“事实已经记录，但业务推进失败后怎么重试”的问题。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigserial | 主键 |
| `fact_id` | bigint | 外键到 `external_payment_facts` |
| `consumer` | text | `rider_deposit_domain`, `order_payment_domain`, `reservation_domain`, `profit_sharing_domain`, `applyment_domain`, `merchant_finance_domain` |
| `business_object_type` | text | 目标业务对象 |
| `business_object_id` | bigint | 目标业务对象 ID |
| `status` | text | `pending`, `processing`, `applied`, `skipped`, `failed` |
| `attempt_count` | int | 重试次数 |
| `last_error` | text | 最近失败摘要 |
| `next_retry_at` | timestamptz | 失败重试时间 |
| `applied_at` | timestamptz | 消费成功时间 |
| `created_at` / `updated_at` | timestamptz | 时间戳 |

建议唯一约束：

- `(fact_id, consumer, business_object_type, business_object_id)` 唯一。

### 3.4 `payment_domain_outbox`

用于把业务域消费 fact 后产生的跨域副作用异步化。第一批尤其用于接收方生命周期。

建议字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigserial | 主键 |
| `event_type` | text | `rider_deposit_activated`, `rider_deposit_cleared`, `payment_order_paid`, `refund_succeeded` |
| `aggregate_type` | text | 业务聚合类型 |
| `aggregate_id` | bigint | 聚合 ID |
| `payload` | jsonb | 事件载荷，禁止敏感明文 |
| `status` | text | `pending`, `processing`, `published`, `failed` |
| `attempt_count` | int | 重试次数 |
| `next_retry_at` | timestamptz | 下次重试时间 |
| `last_error` | text | 错误摘要 |
| `created_at` / `updated_at` | timestamptz | 时间戳 |

## 4. 归一状态语义

### 4.1 `terminal_status`

| 归一状态 | 是否终态 | 说明 | 可推进业务终态 |
| --- | --- | --- | --- |
| `success` | 是 | 微信明确成功，例如支付成功、退款成功、转账成功、进件完成 | 可以，但必须由业务域判断幂等和前置条件 |
| `failed` | 是 | 微信明确失败，例如退款异常、转账失败、进件驳回 | 可以推进失败/回滚/待处理状态 |
| `closed` | 是 | 外部对象关闭，例如退款关闭、支付关闭、订单关闭 | 可以推进关闭或补偿状态 |
| `expired` | 是 | 外部对象超时失效 | 可以推进超时状态 |
| `processing` | 否 | 微信仍在处理中，或者提交已受理 | 不推进业务终态，只更新观测和下次查询 |
| `unknown` | 否 | 查询失败、状态不支持、合同漂移或无法确认 | 不推进业务终态，触发重试/告警 |

### 4.2 各能力状态映射

| 能力 | 微信/当前状态 | 归一状态 |
| --- | --- | --- |
| 直连/收付通支付 | `SUCCESS` | `success` |
| 直连/收付通支付 | `NOTPAY`, `USERPAYING`, `ACCEPT` | `processing` |
| 直连/收付通支付 | `CLOSED` | `closed` |
| 直连/收付通支付 | `PAYERROR` | `failed` |
| 直连/收付通退款 | `SUCCESS` | `success` |
| 直连/收付通退款 | `PROCESSING` | `processing` |
| 直连/收付通退款 | `CLOSED` | `closed` |
| 直连/收付通退款 | `ABNORMAL` | `failed` |
| 商家转账 | `ACCEPTED`, `PROCESSING`, `WAIT_USER_CONFIRM` | `processing` |
| 商家转账 | `SUCCESS` | `success` |
| 商家转账 | `FAIL`, `CANCELING`, `CANCELLED` | `failed` 或 `closed`，按微信语义细分 |
| 分账 | `PROCESSING` | `processing` |
| 分账 | `FINISHED` | `success` |
| 分账 | `FAILED` | `failed` |
| 分账回退 | `PROCESSING` | `processing` |
| 分账回退 | `SUCCESS` | `success` |
| 分账回退 | `FAILED` | `failed` |
| 进件 | `APPLYMENT_STATE_EDITTING`, `AUDITING`, `TO_BE_SIGNED` | `processing` |
| 进件 | `APPLYMENT_STATE_FINISHED` | `success` |
| 进件 | `APPLYMENT_STATE_REJECTED`, unsupported legacy terminal state | `failed` 或 `unknown`，由 contract validation 决定 |
| 商户提现 | 预约/处理中状态 | `processing` |
| 商户提现 | 成功到账状态 | `success` |
| 商户提现 | 失败/退票状态 | `failed` |
| 商户注销提现 | 注销/提现处理中 | `processing` |
| 商户注销提现 | 注销完成且提现完成 | `success` |
| 商户注销提现 | 驳回/失败 | `failed` |

状态映射应由 payment fact normalizer 集中维护，不能散落在 handler 或业务 service 中重复实现。

## 5. Terminalizer 规则

### 5.1 输入来源

Terminalizer 接收三类输入：

- callback：来自微信通知，已验签和解密。
- query：来自 recovery scheduler、timeout scanner 或手工查询。
- manual_reconciliation：来自运维后台或一次性对账任务。

### 5.2 处理流程

1. 校验 provider/channel/capability 和归属边界。
2. 从外部 payload 解析 `external_object_type`、`external_object_key`、`upstream_state`、金额和微信 ID。
3. 归一为 `terminal_status` 和 `is_terminal`。
4. 构造 `dedupe_key`。
5. 插入 `external_payment_facts`，重复时返回已有 fact。
6. 如果 `is_terminal=false`，只记录事实并更新下一次查询建议，不推进业务。
7. 如果 `is_terminal=true`，创建或 claim `external_payment_fact_applications`。
8. 调用对应 domain handler。
9. domain handler 成功后标记 application `applied`，失败则记录 `failed` 和 `next_retry_at`。
10. fact 的 `processing_status` 仅表示 terminalizer 是否完成分发，不代表业务一定成功。

### 5.3 幂等要求

- 同一微信 notification id 重复投递，只能产生一个 fact。
- callback 和 query 得到同一个外部终态时，可以产生不同来源的事实，但同一个 domain handler 对同一外部对象的同一终态只能应用一次。
- domain handler 必须用业务表状态前置条件或事务锁保证幂等，不能只依赖 fact 唯一约束。
- 对金额路径，application 标记为 `applied` 必须和业务状态写入同事务完成。

## 6. 和现有 `wechat_notifications` 的关系

`wechat_notifications` 当前用于 callback claim 和幂等记录，仍可保留。

建议迁移方式：

1. 第一阶段继续使用 `wechat_notifications` 做 notification claim。
2. claim 成功后写入 `external_payment_facts`。
3. `wechat_notifications.processed_at` 表示 webhook 已处理完成；`external_payment_facts.processing_status` 表示事实是否完成 terminalizer 分发。
4. 后续稳定后，可以把 `wechat_notifications` 视为 callback inbox，保留其表名以降低迁移风险。

不能把 `wechat_notifications` 直接当作 facts 表，因为它缺少 channel/capability、终态归一、业务 owner、query/manual 来源和 application 重试语义。

## 7. 首批落地切片

### TASK-PAY-002A schema 草案与 sqlc source

范围：

- 新增 migration：`external_payment_commands`、`external_payment_facts`、`external_payment_fact_applications`、`payment_domain_outbox`。
- 新增 sqlc query：insert/get/claim/update fact application。
- 新增 constants：provider、channel、source、terminal_status、processing_status、capability。

验收：

- migration 可执行。
- `make sqlc` 后生成代码。
- 新 query 都有稳定 `ORDER BY` 和明确状态条件。

验证：

- `make sqlc`
- `go test ./db/sqlc -run 'TestExternalPaymentFact|TestPaymentFactApplication'`

已落地文件：

- `locallife/db/migration/000217_external_payment_facts.up.sql`
- `locallife/db/migration/000217_external_payment_facts.down.sql`
- `locallife/db/query/external_payment_fact.sql`
- `locallife/db/sqlc/external_payment_fact.sql.go`
- `locallife/db/sqlc/external_payment_fact_test.go`
- `locallife/db/sqlc/constants.go`

### TASK-PAY-002B fact normalizer 与 terminalizer skeleton

范围：

- 新增 `logic/payment_fact_service.go` 或 `logic/paymentfact/` 小包。
- 提供 `RecordExternalPaymentFact(ctx, input)`。
- 支持 callback/query/manual 三种 source。
- 不直接迁移业务 handler，只允许返回 fact 和 application claim 结果。

验收：

- 重复 dedupe key 返回已有 fact。
- `processing/unknown` 不创建业务 application。
- `success/failed/closed/expired` 创建 application，但 handler 可以先用 no-op consumer。

验证：

- `go test ./logic -run TestPaymentFactService`

### TASK-PAY-002C 接入骑手押金只读事实写入

范围：

- `handlePaymentNotify` 中 rider_deposit 支付成功写 direct payment fact。
- `handleRefundNotify` / `RefundRecoveryScheduler` 中 rider_deposit refund 终态写 direct refund fact。
- 事实写入失败时按高风险处理：callback 返回 FAIL 等待微信重试；query 记录错误并下次重试。
- 不改变当前押金入账/退款结算行为。

验收：

- 当前业务行为不变。
- 同一 callback 重复投递只产生一条 fact。
- query 和 callback 对同一退款终态不会重复结算押金。

验证：

- 现有 rider deposit refund 测试继续通过。
- 新增 callback/query fact 写入幂等测试。

### TASK-PAY-002D application retry worker

范围：

- 扫描 `external_payment_fact_applications(status='failed', next_retry_at<=now())`。
- 重新调用 domain handler。
- 超过重试阈值发平台告警。

验收：

- domain handler 临时失败不会丢 fact。
- 可以按 fact id 重放业务应用。

验证：

- `go test ./worker -run TestPaymentFactApplicationRecovery`

## 8. TASK-PAY-003 的输入

TASK-PAY-002 完成后，TASK-PAY-003 应基于 command/fact 语义重命名提交接口返回：

- 支付下单：`payment_status=submitted` 或 `pay_params_ready`，不叫 paid/success。
- 退款提交：`refund_status=processing` 或 `accepted`，不叫 refunded/success。
- 分账提交：`sharing_status=processing`，不叫 finished。
- 进件提交：`applyment_status=submitted`，不叫 completed。

前端只展示“已提交处理/待微信确认/处理中”，不把提交成功文案做成终态成功。

## 9. 风险与发布门禁

### 风险

- 事实表写入失败会影响 callback 返回值，必须明确是否等待微信重试。
- raw payload 脱敏必须严格，禁止存身份证号、银行卡号、密钥、签名原文、证件图片地址等敏感明文。
- 新增唯一约束要考虑 query 重复 PROCESSING 的高频写入，避免造成恢复任务噪音。
- domain handler 和 application 状态必须同事务提交，否则可能出现业务已推进但 application 仍 pending 的漂移。

### 发布门禁

- 每个接入能力必须有 callback duplicate 测试。
- 每个 query recovery 接入必须有“query 和 callback 到同一终态入口”的测试。
- 金额状态变更必须有重复执行测试。
- `unknown` 和 `processing` 不得推进业务终态。
- 新 schema/query 变更必须 `make sqlc`，涉及 mocks 时补 `make mock`。
