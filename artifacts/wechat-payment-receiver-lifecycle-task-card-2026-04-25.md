# TASK-PAY-005 分账接收方生命周期独立化任务卡

## 1. 目标

把微信分账接收方 membership 从支付成功、押金退款、运营商审核、运营商状态切换等主业务链路中拆出来，形成可重试、可观测、可人工恢复的 ReceiverLifecycle 边界。

本任务不处理分账 create/finish/return 的资金终态，不迁移 payment/refund/profit sharing fact application，也不改变现有运营商、骑手、订单的业务状态语义。

## 2. 当前审计结论

当前真实调用面：

- `logic/profit_sharing_receiver_sync_service.go` 封装了 `EnsureOperatorReceiver`、`DeleteOperatorReceiver`、`EnsureRiderReceiver`、`DeleteRiderReceiver`、`EnsurePersonalOpenIDReceiver`、`DeletePersonalOpenIDReceiver`。
- `worker/task_process_payment.go` 对 rider deposit payment success 已改为写 rider receiver `present` target intent；intent 写入失败只记录结构化日志，不再让已提交的支付成功处理返回错误。
- `logic/rider_deposit_refund_service.go` 在押金退款成功并押金归零后已改为写 rider receiver `absent` target intent；intent 写入失败只记录日志，不阻断退款终态处理。
- `logic/operator_status_service.go` 已在 005C-1 改为写 operator receiver target intent，不再同步 ensure/delete receiver。
- `api/operator_application_admin.go` 已在 005C-1 移除审核事务前同步 ensure receiver 和事务失败后的 background rollback delete，改为本地审核状态完成后写 operator receiver target intent。
- `api/profit_sharing_capability.go` 存在按支付单上下文删除 receiver 的手工能力接口，但它不是 lifecycle owner。

已有进展：

- operator 手动状态切换与合同到期失活已经统一复用 `OperatorStatusService`。
- add/delete receiver 的 already-exists / not-exists 错误已有窄范围幂等忽略。
- operator/rider 的 receiver target intent、worker/scheduler、owner-context repair、target/detail/attempts 查询 API 已全部落地；当前只剩长期失败告警交付仍主要停留在结构化日志与查询定位。

## 3. 风险分级

G3。

理由：receiver membership 直接影响分账资金接收方，且当前同步调用散落在支付成功、押金退款、运营商审核和状态切换路径中。任一同步失败都可能造成主业务状态回滚、重复重试、微信侧 receiver 与本地生命周期漂移，或资金链路后续分账失败。

## 4. 边界原则

- receiver membership 是 operator/rider 生命周期事实，不是某一笔 `payment_order` 的能力边界。
- 主业务 durable state 必须先提交；receiver ensure/delete 作为后置副作用进入可重试边界。
- receiver 同步失败不应回滚押金入账、押金退款终态、operator 审核结果或 operator 状态切换，除非当前请求明确是“手工同步 receiver”能力。
- `AddProfitSharingReceiver` / `DeleteProfitSharingReceiver` 不纳入 TASK-PAY-003 command 表；它们属于 TASK-PAY-005 receiver lifecycle command/fact/attempt 边界。
- 不能把 payment command accepted 当成 receiver 已准备好；两者是独立外部对象。

## 5. 分段计划

### TASK-PAY-005A Receiver lifecycle audit matrix

范围：

- 列出 operator/rider receiver 的所有 ensure/delete 调用点、触发条件、是否阻塞主业务、失败后的用户可见语义、重试入口。
- 明确 receiver membership 的 owner：operator lifecycle、rider deposit lifecycle、manual repair。
- 明确哪些现有入口只保留为 manual repair，不再作为主链路 owner。

验收：

- 每个调用点都有 owner、event、external key、failure behavior、下一步迁移目标。
- 不修改业务代码。

产出：`artifacts/wechat-payment-receiver-lifecycle-matrix-2026-04-25.md`。

### TASK-PAY-005B Receiver intent / attempt persistence design

范围：

- 设计 receiver lifecycle 的持久化锚点，例如 receiver intent / sync attempt / outbox event。
- 定义 receiver target key：provider、channel、receiver_type、appid、account/openid、owner_type、owner_id、desired_state。
- 定义状态：pending、processing、synced、failed、skipped，以及 retry/last_error 字段。
- 明确敏感字段边界：不保存明文 receiver name、openid 以外的实名材料、加密前姓名或完整微信 payload。

验收：

- 有 schema/query 任务卡，但本段不直接改 schema。
- 能回答重复 ensure/delete、已存在/不存在、PARAM_ERROR、缺 openid 的处理语义。

产出：`artifacts/wechat-payment-receiver-lifecycle-persistence-design-2026-04-25.md`。

### TASK-PAY-005B-1 Receiver target/attempt schema and queries

范围：

- 新增 receiver target 与 attempt migration。
- 新增 sqlc query：upsert target、claim pending/failed target、create attempt、mark synced/failed/skipped、按 owner 查询。
- 新增 sqlc focused tests 验证去重、desired_state 覆盖、claim 顺序与 retry 条件。

边界：

- 不改 operator/rider 现有同步调用。
- 不新增 worker。
- 不调用微信。
- 不保存 receiver name 明文、原始微信 request/response 或额外实名材料。

验收：

- target unique key 能表达 owner + receiver target 的单一当前期望。
- attempts 能记录每次执行，但不会成为业务终态来源。
- SQL/query 符合 `SQL_STANDARDS.md`，生成物和 mocks 同步。

产出：

- `locallife/db/migration/000218_create_profit_sharing_receiver_lifecycle.up.sql`
- `locallife/db/migration/000218_create_profit_sharing_receiver_lifecycle.down.sql`
- `locallife/db/query/profit_sharing_receiver_lifecycle.sql`
- `locallife/db/sqlc/querier.go`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle.sql.go`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle_test.go`

验证：

- `make sqlc`
- `make migratedown1 && make migrateup1`
- `go test ./db/sqlc -run 'Test.*ProfitSharingReceiver' -count=1`

### TASK-PAY-005C Operator receiver lifecycle migration

范围：

- operator 审核通过、active/suspended、合同到期失活只写 durable state 和 receiver lifecycle intent。
- receiver ensure/delete 由 worker/scheduler 消费 intent。
- 保留平台手工 repair 入口。

005C-1 已完成（2026-04-25）：

- 新增 `ProfitSharingReceiverLifecycleService`，将 operator owner、appid、openid hash、display name hash 和 desired state 写入 `profit_sharing_receiver_targets`。
- `OperatorStatusService.UpdateStatus` 不再同步调用 `EnsureOperatorReceiver` / `DeleteOperatorReceiver`；active/suspended 本地状态与 role 更新后写 `present` / `absent` target intent。
- `approveOperatorApplicationAdmin` 不再在事务前调用微信 `AddProfitSharingReceiver`，也不再用 background rollback delete；审核事务或 fallback 本地状态完成后写 operator receiver `present` intent。
- 本段不新增 worker，不触碰 rider 押金链路，不改手工 receiver repair 接口。

005C-1 产出：

- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/logic/operator_status_service.go`
- `locallife/api/operator_application_admin.go`
- `locallife/api/logic_adapters.go`
- `locallife/logic/operator_status_service_test.go`
- `locallife/api/operator_application_admin_test.go`

005C-1 验证：

- `go test ./logic -run 'TestOperatorStatusService|TestPaymentCommandService' -count=1`
- `go test ./api -run 'TestApproveOperatorApplicationAdmin|TestUpdateOperatorStatusAdmin|TestBatchUpdateOperatorStatusAdmin' -count=1`

005C-1 残余风险：

- receiver worker/scheduler 尚未实现，target intent 只形成可恢复锚点，尚不会实际调用微信 Add/Delete。
- 当前 intent 写入失败会在本地 operator 状态已提交后返回稳定内部错误；后续应通过事务化 intent 或 recovery/alert/manual retry 降低局部漂移窗口。

005C-2 已完成（2026-04-25）：

- 新增 `ListRetryableProfitSharingReceiverTargetsByOwnerType`，scheduler 只按 `owner_type='operator'` 列出 due target，不直接 claim，避免 Redis 入队失败后 target 卡在 `processing`。
- 新增 `ProcessOperatorReceiverTarget`，由 worker 按 target id claim 后执行 operator receiver ensure/delete，并记录 attempt succeeded/failed/skipped；already exists / not exists 作为幂等成功写入 attempt。
- `ProfitSharingReceiverLifecycleService` 处理失败时只持久化错误码与脱敏摘要，不保存 openid、receiver name、raw WeChat request/response 或 provider raw detail。
- 新增 asynq task `profit_sharing_receiver:process_target` 与 `ProfitSharingReceiverLifecycleScheduler`，只扫描 operator owner target，使用 DB claim 条件保证重复入队幂等。
- `main.go` 注册 `profit-sharing-receiver-lifecycle` scheduler，合同到期失活测试改为验证本地 suspend + absent receiver target intent，不再期待同步微信 delete receiver。
- 本段不触碰 rider 押金 receiver lifecycle，不改手工 receiver repair 接口，不迁移分账 create/return/finish 资金终态。

005C-2 产出：

- `locallife/db/query/profit_sharing_receiver_lifecycle.sql`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle.sql.go`
- `locallife/db/mock/store.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/logic/profit_sharing_receiver_sync_service.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service_test.go`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle_test.go`
- `locallife/worker/task_profit_sharing_receiver_lifecycle.go`
- `locallife/worker/profit_sharing_receiver_lifecycle_scheduler.go`
- `locallife/worker/profit_sharing_receiver_lifecycle_scheduler_test.go`
- `locallife/worker/distributor.go`
- `locallife/worker/noop_distributor.go`
- `locallife/worker/processor.go`
- `locallife/worker/mock/distributor.go`
- `locallife/main.go`
- `locallife/scheduler/operator_contract_expiry_scheduler_test.go`

005C-2 验证：

- `make sqlc`
- `go test ./db/sqlc -run 'Test.*ProfitSharingReceiver' -count=1`
- `go test ./logic -run 'Test.*ProfitSharingReceiver|TestOperatorStatusService' -count=1`
- `go test ./worker -run 'Test.*ProfitSharingReceiver|TestProcessTaskAutomaticRecoveryDisputeResolution' -count=1`
- `go test ./db/mock ./db/sqlc -run 'Test.*ProfitSharingReceiver' -count=1 && go test ./logic -run 'Test.*ProfitSharingReceiver|TestOperatorStatusService|TestPaymentCommandService' -count=1 && go test ./worker -count=1`
- `go test . ./api ./scheduler -run 'TestApproveOperatorApplicationAdmin|TestUpdateOperatorStatusAdmin|TestBatchUpdateOperatorStatusAdmin|TestDataCleanupScheduler_MarkExpiredOperators' -count=1`

005C-2 残余风险：

- 当前 worker/scheduler 只消费 operator owner；rider receiver ensure/delete 仍留给 005D。
- 手工 repair 入口仍以旧 capability 存在，owner-context repair、长期 failed 告警与人工重放留给 005E。
- scheduler 入队依赖 asynq/Redis；入队失败不会提前 claim target，target 会保持 pending/failed 供后续扫描恢复。
- intent 写入失败发生在 operator 本地状态提交后的局部漂移窗口仍存在，后续应由 recovery/alert/manual retry 或事务化 intent 策略继续收口。

验收：

- receiver 同步失败不回滚 operator 审核或状态切换。
- 合同到期失活不会因为微信 delete 失败反复阻塞 scheduler。
- operator active 前的区域冲突校验仍同步执行，不被 receiver 异步化削弱。

### TASK-PAY-005D Rider receiver lifecycle migration

范围：

- 骑手押金支付成功后只触发 rider receiver ensure intent。
- 押金退款终态、余额归零后只触发 rider receiver delete intent。
- receiver 同步失败不回滚押金入账或退款终态。

验收：

- 押金支付成功任务不因 receiver ensure 失败反复失败。
- 押金归零删除 receiver 失败进入 retry/alert，不影响退款成功事实消费。
- stale credit reconciliation 不再隐式承担 receiver lifecycle 修复。

005D 已完成（2026-04-25）：

- `ProfitSharingReceiverLifecycleService` 增加 rider owner target intent：`RequestRiderReceiverPresent` / `RequestRiderReceiverAbsent`，并支持 worker 按 `owner_type='rider'` 处理 Add/Delete receiver。
- `ProcessTaskPaymentSuccess` 的 rider deposit success 后置副作用从同步 `EnsureRiderReceiver` 改为写 rider receiver `present` target intent；获取 rider 或写 intent 失败只记录结构化日志，不再让已提交的支付成功处理任务失败。
- `RiderDepositRefundService.ResolveRefund` 在押金与冻结押金归零后从同步 `DeleteRiderReceiver` 改为写 rider receiver `absent` target intent；intent 失败只记录日志，不阻断退款终态结算。
- `ProfitSharingReceiverLifecycleScheduler` 从 operator-only 扩展为扫描 operator 与 rider due target；worker 调用通用 `ProcessReceiverTarget`，仍由 DB claim 保证重复入队幂等。
- 本段不改 receiver target/attempt schema，不改手工 repair 入口，不迁移分账 create/finish/return 资金终态。

005D 产出：

- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/logic/rider_deposit_refund_service.go`
- `locallife/worker/task_process_payment.go`
- `locallife/worker/task_profit_sharing_receiver_lifecycle.go`
- `locallife/worker/profit_sharing_receiver_lifecycle_scheduler.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service_test.go`
- `locallife/logic/rider_deposit_refund_service_test.go`
- `locallife/worker/task_process_payment_test.go`
- `locallife/worker/profit_sharing_receiver_lifecycle_scheduler_test.go`

005D 验证：

- `get_errors` on modified Go files: no errors。
- `git diff --check -- logic/profit_sharing_receiver_lifecycle_service.go logic/profit_sharing_receiver_lifecycle_service_test.go logic/rider_deposit_refund_service.go logic/rider_deposit_refund_service_test.go worker/profit_sharing_receiver_lifecycle_scheduler.go worker/profit_sharing_receiver_lifecycle_scheduler_test.go worker/task_profit_sharing_receiver_lifecycle.go worker/task_process_payment.go worker/task_process_payment_test.go`
- `go test ./logic -run 'Test.*ProfitSharingReceiver|TestRiderDepositRefundService_ResolveRefund' -count=1`
- `go test ./worker -run 'TestProcessTaskPaymentSuccess_RiderDeposit|TestProcessTaskRefundResult_RiderDeposit|Test.*ProfitSharingReceiver' -count=1`
- `go test ./worker -count=1`

005D 残余风险：

- rider receiver intent 写入失败发生在押金支付/退款终态提交后的局部漂移窗口，当前按 non-blocking 记录日志；长期失败告警、人工重放和 owner-context repair 留给 005E。
- scheduler 仍依赖 asynq/Redis 入队；入队失败不 claim target，target 保持 pending/failed 供后续扫描恢复。
- rider Add/Delete receiver 的微信 PARAM_ERROR、缺 openid、高风险 receiver 等长期失败已有 target/attempt 错误记录，但还没有运营后台人工处理入口。

### TASK-PAY-005E Recovery, alerting, and manual repair

范围：

- 完善 receiver lifecycle retry scheduler / worker 的告警、长期失败观察和手工触发能力。
- 增加长期 failed receiver attempt 的结构化日志和告警字段。
- 增加手工重放入口或复用现有平台能力入口，但必须以 receiver owner 为上下文，不以单笔 payment_order 为 owner。

验收：

- already exists / not exists 幂等成功。
- PARAM_ERROR、缺 openid、加密失败、微信 5xx 分流清晰。
- 有 focused tests 覆盖重复投递、失败重试、手工恢复。

005E 已完成（2026-04-25）：

- 新增 platform admin owner-context repair 入口：`POST /v1/platform/profit-sharing/receiver-lifecycle/repair`，只接受 `owner_type=operator|rider`、`owner_id`、`desired_state=present|absent`，不接受 payment_order 上下文。
- repair handler 只写 receiver target intent 并立即入队 `profit_sharing_receiver:process_target`，不在 API 请求内同步调用微信 Add/Delete receiver。
- repair response 只返回 target id、owner、desired state、sync status、attempt count 和时间，不返回 openid、receiver name、account hash、display name hash 或微信原始响应。
- 005E review 补齐手工 repair 审计：成功入队写 `enqueued=true`，入队失败也会对已写入的 target intent 记录 `enqueued=false`，审计 metadata 只包含 owner、desired state、target status 与 attempt count，不包含 openid、receiver name、hash 或微信原始 payload。
- worker 处理 receiver target 时补充 `owner_type`、`owner_id`、`attempt_count` 结构化日志字段，failed/skipped 结果以 warn 级别输出。
- scheduler 对 `failed` 且 `attempt_count >= 3` 的 receiver target 输出结构化 error 日志，作为长期失败告警信号；仍保持 scheduler 只 list/enqueue，不 claim。
- 本段不新增 SQL/schema，不改变 operator/rider 主业务终态，不改旧 payment_order-based 手工 delete 接口。

005E 产出：

- `locallife/api/profit_sharing_receiver_lifecycle.go`
- `locallife/api/profit_sharing_receiver_lifecycle_test.go`
- `locallife/api/server.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/worker/task_profit_sharing_receiver_lifecycle.go`
- `locallife/worker/profit_sharing_receiver_lifecycle_scheduler.go`
- `locallife/docs/docs.go`
- `locallife/docs/swagger.json`
- `locallife/docs/swagger.yaml`

005E 验证：

- `get_errors` on modified Go files: no errors。
- `git diff --check -- api/profit_sharing_receiver_lifecycle.go api/profit_sharing_receiver_lifecycle_test.go api/server.go logic/profit_sharing_receiver_lifecycle_service.go worker/task_profit_sharing_receiver_lifecycle.go worker/profit_sharing_receiver_lifecycle_scheduler.go`
- `go test ./api -run 'TestRepairProfitSharingReceiverLifecycleAPI' -count=1`
- `go test ./logic -run 'Test.*ProfitSharingReceiver' -count=1`
- `go test ./worker -run 'Test.*ProfitSharingReceiver' -count=1`
- `make swagger`

005E review 验证（2026-04-25）：

- `get_errors` on 005E modified Go files: no errors。
- `go -C /home/sam/locallife/locallife test ./api -run 'TestRepairProfitSharingReceiverLifecycleAPI' -count=1`
- `go -C /home/sam/locallife/locallife test ./logic -run 'Test.*ProfitSharingReceiver' -count=1`
- `go -C /home/sam/locallife/locallife test ./worker -run 'Test.*ProfitSharingReceiver' -count=1`
- `git diff --check -- locallife/api/profit_sharing_receiver_lifecycle.go locallife/api/profit_sharing_receiver_lifecycle_test.go locallife/api/server.go locallife/logic/profit_sharing_receiver_lifecycle_service.go locallife/worker/task_profit_sharing_receiver_lifecycle.go locallife/worker/profit_sharing_receiver_lifecycle_scheduler.go`
- `rg -n '[ \\t]+$' /home/sam/locallife/locallife/api/profit_sharing_receiver_lifecycle.go /home/sam/locallife/locallife/api/profit_sharing_receiver_lifecycle_test.go` returned no matches.

005E 残余风险：

- 手工 repair 已能按 owner-context 写 intent 和入队，但尚未提供 platform 列表/详情页或 attempts 查询 API；运维仍需通过日志和 DB target/attempt 表定位具体失败。
- API 入队失败会返回 500，但 target intent 已写为 pending；后续仍可由 scheduler 扫描恢复，调用方需要根据错误重试或等待调度。
- 长期失败当前是结构化 error 日志信号，还未接入独立告警通知表或告警分发渠道。

### TASK-PAY-005F Receiver lifecycle operations visibility

范围：

- 增加 platform admin 只读查询入口，用于定位 receiver lifecycle target 与 attempt，不触发微信调用，不改变 target 状态。
- target 列表支持按 `owner_type`、`owner_id`、`sync_status` 过滤，返回 `total/page/limit/total_pages/has_more`。
- target 详情按 target id 查询；attempt 列表按 target id 查询并分页。
- 所有响应只返回 owner、provider/channel/appid、desired state、sync status、attempt count、脱敏错误摘要与时间字段，不返回 openid、receiver name、account hash、display name hash 或微信原始 payload。
- 读取操作写入 best-effort platform audit log，便于追踪高风险运维查询。

005F 已完成（2026-04-25）：

- 新增 `GET /v1/platform/profit-sharing/receiver-lifecycle/targets`。
- 新增 `GET /v1/platform/profit-sharing/receiver-lifecycle/targets/{id}`。
- 新增 `GET /v1/platform/profit-sharing/receiver-lifecycle/targets/{id}/attempts`。
- 新增 target list/count SQL 与 attempts paginated list/count SQL，并同步 sqlc 与 mocks。
- 新增 focused API tests 覆盖过滤分页、响应脱敏、读取审计、详情查询、attempts 分页与 not found。
- 新增 focused SQLC tests 覆盖 target 过滤/count 与 attempts 分页/count。

005F 产出：

- `locallife/db/query/profit_sharing_receiver_lifecycle.sql`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle.sql.go`
- `locallife/db/sqlc/profit_sharing_receiver_lifecycle_test.go`
- `locallife/db/mock/store.go`
- `locallife/api/profit_sharing_receiver_lifecycle.go`
- `locallife/api/profit_sharing_receiver_lifecycle_read.go`
- `locallife/api/profit_sharing_receiver_lifecycle_test.go`
- `locallife/api/server.go`
- `locallife/docs/docs.go`
- `locallife/docs/swagger.json`
- `locallife/docs/swagger.yaml`

005F 验证：

- `make -C /home/sam/locallife/locallife sqlc`
- `go -C /home/sam/locallife/locallife test ./db/sqlc -run 'Test.*ProfitSharingReceiver' -count=1`
- `go -C /home/sam/locallife/locallife test ./api -run 'Test(List|Get|Repair)ProfitSharingReceiverLifecycle' -count=1`
- `go -C /home/sam/locallife/locallife test ./logic -run 'Test.*ProfitSharingReceiver' -count=1`
- `go -C /home/sam/locallife/locallife test ./worker -run 'Test.*ProfitSharingReceiver' -count=1`
- `make -C /home/sam/locallife/locallife swagger`
- `get_errors` on 005F modified/generated files: no errors。
- `git diff --check -- locallife/api/profit_sharing_receiver_lifecycle.go locallife/api/profit_sharing_receiver_lifecycle_test.go locallife/api/server.go locallife/db/query/profit_sharing_receiver_lifecycle.sql locallife/db/sqlc/profit_sharing_receiver_lifecycle.sql.go locallife/db/sqlc/profit_sharing_receiver_lifecycle_test.go locallife/db/mock/store.go locallife/docs/docs.go locallife/docs/swagger.json locallife/docs/swagger.yaml`
- split 后 `locallife/api/profit_sharing_receiver_lifecycle.go` 为 152 行，`locallife/api/profit_sharing_receiver_lifecycle_read.go` 为 375 行；本段新增非测试 API 文件均低于 500 行 guardrail。全局 `make lint-filesize` 当前仍受既有 65 个超限文件阻塞，本段未尝试收敛这些无关历史文件。

005F 残余风险：

- 005F 只提供 API 查询能力，不包含 web/operator console 页面。
- 长期失败仍依赖结构化 error 日志与 target/attempt 查询定位，尚未接入独立告警通知表或告警分发渠道。

### TASK-PAY-005G Receiver lifecycle long-failed alert closeout

005G 已完成（2026-04-26）：

- `worker/task_profit_sharing_receiver_lifecycle.go` 在 receiver target 处理结果为 `failed` 且 `attempt_count >= 3` 时，复用现有平台告警通道发布 `PROFIT_SHARING_RECEIVER_FAILED` 告警，不再只停留在结构化日志。
- 告警 payload 只包含 `owner_type`、`owner_id`、`desired_state`、`sync_status`、`attempt_count`、脱敏错误码/摘要与时间字段，不回传 openid、receiver name、account hash 或微信原始 payload。
- 告警发布仍走现有非阻塞路径；告警持久化或 pubsub 失败不会回滚 receiver target/attempt 真值。

005G 产出：

- `locallife/worker/task_profit_sharing_receiver_lifecycle.go`
- `locallife/worker/task_profit_sharing_receiver_lifecycle_test.go`
- `locallife/worker/task_process_payment.go`

005G 验证：

- `go test ./worker -run 'TestProcessTaskProfitSharingReceiverTarget_(PublishesAlertAfterRepeatedFailure|DoesNotPublishAlertBelowRepeatedFailureThreshold)|TestProfitSharingReceiverLifecycleSchedulerRunOnce' -count=1`

完成后判定：

- TASK-PAY-005 backend 主链与 closeout 已全部完成；是否增加前端页面属于后续运维体验增强，不再阻塞 007。

## 6. 暂不纳入

- 分账 create/finish/return 的 command/fact 迁移。
- 补差 create/return/cancel fact 化。
- 订单/预订支付成功 domain handler 拆分。
- 微信投诉、违规通知、结算账户修改 operational fact。

## 7. 首段建议

TASK-PAY-005A 至 005G 已全部完成。下一步直接进入 TASK-PAY-007 的分账剩余异步链路拆分。补差不属于当前 MVP，实现顺序上不应被拉回 005 之后的首批任务。
