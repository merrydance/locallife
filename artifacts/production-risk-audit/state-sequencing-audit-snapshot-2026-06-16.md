# 系统状态时序隐患审计快照（2026-06-16）

> 目的：本文件是当前只读审计会话的防漂移快照。结论均为“候选时序风险/待进一步验证”，除非特别标注，不等同于已确认生产 bug。

## 审计口径

- 范围：运行中可能因状态时序、弱顺序事件、重复触发、回调/worker/scheduler 竞态导致异常的分支和逻辑链。
- 方法：先从状态常量、SQL 状态写入点、异步入口、回调、worker、scheduler、前端重复触发点反查链路。
- 当前阶段：只读盘点。未改业务代码，未跑测试。
- 风险分级口径：资金、回调、退款、分账、提现、订单/配送/预约状态机按 G2/G3 看待。

## 已读/已参考的项目规则

- `AGENTS.md`
- `.github/copilot-instructions.md`
- `.github/README.md`
- `locallife/AGENTS.md`
- `.github/instructions/backend-locallife.instructions.md`
- `.github/prompts/backend-bugfix.prompt.md`
- `.github/prompts/backend-payment-domain.prompt.md`
- `.github/standards/backend/README.md`
- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/domains/wechat-payment/README.md`
- `.github/standards/domains/baofu-payment/README.md`
- Skill 指引：`locallife-prompt-router`、`superpowers:systematic-debugging`、`superpowers:verification-before-completion`、`superpowers:dispatching-parallel-agents`

## 已重点读取的代码面

Backend:

- `locallife/db/sqlc/constants.go`
- `locallife/db/query/payment_order.sql`
- `locallife/db/query/refund_order.sql`
- `locallife/db/query/profit_sharing_order.sql`
- `locallife/db/query/profit_sharing_return.sql`
- `locallife/db/query/external_payment_fact.sql`
- `locallife/db/query/table_reservation.sql`
- `locallife/db/query/reservation_adjustment.sql`
- `locallife/db/query/baofu_withdrawal_order.sql`
- `locallife/db/query/order.sql`
- `locallife/db/query/onboarding_review.sql`
- `locallife/db/query/ocr_job.sql`
- `locallife/db/query/media.sql`
- `locallife/db/query/print_log.sql`
- `locallife/db/sqlc/tx_payment_success.go`
- `locallife/db/sqlc/tx_create_order.go`
- `locallife/db/sqlc/tx_order_status.go`
- `locallife/db/sqlc/tx_reservation_adjustment.go`
- `locallife/db/sqlc/tx_dining_session.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- `locallife/logic/payment_order_service.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/worker/task_payment_timeout.go`
- `locallife/worker/task_payment_timeout_baofu.go`
- `locallife/worker/task_process_payment.go`
- `locallife/worker/task_payment_fact_application.go`
- `locallife/worker/payment_recovery_scheduler.go`
- `locallife/worker/task_payment_domain_outbox.go`
- `locallife/worker/task_baofu_profit_sharing.go`
- `locallife/worker/task_baofu_withdrawal_command_dispatch.go`
- `locallife/worker/task_baofu_withdrawal_fact_application.go`
- `locallife/worker/baofu_withdrawal_recovery_scheduler.go`
- `locallife/worker/task_reservation_timeout.go`
- `locallife/worker/task_onboarding_review.go`
- `locallife/scheduler/data_cleanup.go`
- `locallife/scheduler/order_timeout.go`
- `locallife/scheduler/takeout_auto_complete.go`
- `locallife/scheduler/dine_in_checkout_recovery.go`

Frontend sampled:

- `weapp/miniprogram/pages/takeout/order-confirm/index.ts`
- `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/services/payment-workflow.ts`
- `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/api/payment.ts`
- `weapp/miniprogram/pages/orders/_services/order-cancel-refund-workflow.ts`
- `weapp/miniprogram/pages/orders/_services/order-receipt-confirmation.ts`

本轮追加读取/复核：

- `locallife/db/migration/000011_add_payment_orders.up.sql`
- `locallife/db/migration/000089_add_payment_orders_transaction_id_unique.up.sql`
- `locallife/db/migration/000170_add_claim_recovery_payment_order_uniqueness.up.sql`
- `locallife/db/migration/000233_add_baofu_account_opening_flows.up.sql`
- `locallife/api/payment_callback.go`
- `locallife/api/baofu_callback.go`
- `locallife/api/baofu_account_open_callback_fact.go`
- `locallife/api/feieyun_callback.go`
- `locallife/api/yilianyun_callback.go`
- `locallife/logic/baofu_payment_service.go`
- `locallife/logic/baofu_payment_fact_application.go`
- `locallife/logic/payment_fact_application_baofu_verify_fee.go`
- `locallife/worker/cloud_printer_status_poll_scheduler.go`
- `locallife/worker/task_print_order.go`
- `locallife/worker/task_print_attempt.go`
- `locallife/db/migration/000177_add_print_log_vendor_order_id.up.sql`
- `locallife/db/migration/000180_add_print_log_task_key.up.sql`
- `locallife/db/migration/000246_add_cloud_printer_multi_provider_foundation.up.sql`
- `locallife/db/migration/000247_add_print_log_provider_status_poll_metadata.up.sql`
- `locallife/worker/payment_fact_application_scheduler.go`
- `locallife/worker/baofu_account_opening_recovery_scheduler.go`（通过 rg 输出部分核对，尚需完整读）
- `locallife/db/query/baofu_account_opening_flow.sql`
- `locallife/db/query/baofu_merchant_report.sql`
- `locallife/db/migration/000229_add_baofu_merchant_reports.up.sql`
- `locallife/db/migration/000245_add_baofu_account_opening_mode.up.sql`
- `locallife/db/sqlc/tx_baofu_account_opening_ready.go`
- `locallife/api/baofu_settlement_account.go`
- `locallife/logic/baofu_account_onboarding_open.go`
- `locallife/logic/baofu_account_onboarding_prepare.go`
- `locallife/logic/baofu_account_onboarding_payment.go`
- `locallife/logic/baofu_account_onboarding_profile.go`
- `locallife/logic/baofu_account_onboarding_apply.go`
- `locallife/logic/baofu_account_onboarding_recovery.go`
- `locallife/logic/baofu_account_merchant_report_service.go`
- `locallife/logic/baofu_merchant_report_service.go`
- `locallife/worker/task_baofu_account_opening.go`
- `locallife/worker/baofu_merchant_report_recovery_scheduler.go`
- `locallife/logic/combined_payment_service.go`
- `locallife/db/query/combined_payment.sql`
- `locallife/db/sqlc/tx_close_combined_payment.go`
- `locallife/logic/membership_payment.go`
- `locallife/logic/membership_recharge.go`
- `locallife/logic/membership_balance_adjust.go`
- `locallife/db/query/membership.sql`
- `locallife/db/sqlc/tx_membership.go`
- `locallife/db/query/user_balance.sql`
- `locallife/api/membership.go`
- `locallife/logic/credential_governance_service.go`
- `locallife/db/query/credential_ledger.sql`
- `locallife/db/sqlc/tx_credential_ledger.go`
- `locallife/scheduler/data_cleanup.go`（资质到期提醒/暂停段）
- `locallife/db/query/trust_score.sql`
- `locallife/api/rider_application_submit.go`
- `locallife/worker/task_claim_refund.go`
- `locallife/worker/claim_refund_recovery_scheduler.go`
- `locallife/worker/task_claim_refund_test.go`
- `locallife/worker/claim_behavior_action_recovery_scheduler.go`
- `locallife/db/query/behavior_trace.sql`
- `locallife/db/query/claim_recovery.sql`
- `locallife/db/query/recovery_dispute.sql`
- `locallife/db/sqlc/tx_claim_behavior.go`
- `locallife/db/sqlc/claim_recovery_behavior_action.go`
- `weapp/miniprogram/pages/orders/detail/index.ts`
- `weapp/miniprogram/pages/orders/tracking/index.ts`
- `locallife/logic/rider_deposit_refund_service.go`
- `locallife/db/sqlc/tx_rider_refund.go`
- `locallife/db/query/rider_deposit.sql`
- `locallife/db/query/rider_deposit_credit.sql`
- `locallife/db/query/refund_order.sql`（rider deposit refund 相关查询/状态更新段）
- `locallife/worker/refund_recovery_scheduler.go`（stuck processing rider deposit refund 查询恢复段）
- `locallife/worker/task_process_payment.go`（旧 refund result worker 与 rider deposit amount mismatch refund 段）
- `locallife/api/payment_callback.go`（直连退款回调 rider_deposit 分流段）
- `locallife/logic/payment_fact_application_service.go`（rider deposit refund fact application 段）
- `locallife/worker/payment_fact_application_scheduler.go`
- `locallife/logic/rider_deposit_refund_service_test.go`
- `locallife/logic/payment_fact_application_service_test.go`
- `locallife/worker/task_process_payment_mismatch_test.go`
- `locallife/api/baofu_callback.go`（提现回调段）
- `locallife/api/baofu_withdrawal.go`
- `locallife/api/baofu_withdrawal_callback_fact.go`
- `locallife/logic/baofu_withdraw_service.go`
- `locallife/db/sqlc/tx_baofu_withdrawal.go`
- `locallife/db/query/baofu_withdrawal_order.sql`（复核提现 recovery 查询条件）
- `locallife/db/query/external_payment_fact.sql`（external command submitted/unknown/claim 查询）
- `locallife/db/migration/000260_add_baofu_withdrawal_idempotency.up.sql`
- `locallife/db/migration/000261_harden_baofu_withdrawal_idempotency_pair.up.sql`
- `locallife/worker/baofu_withdrawal_recovery_scheduler.go`
- `locallife/worker/task_baofu_withdrawal_command_dispatch.go`
- `locallife/worker/task_baofu_withdrawal_fact_application.go`
- `locallife/worker/task_baofu_withdrawal_command_dispatch_test.go`
- `locallife/worker/task_baofu_withdrawal_fact_application_test.go`
- `locallife/api/baofu_callback_test.go`（提现回调测试段）

## 已发现候选时序隐患

### 1. 支付创建：先查后建可能产生多个 pending 支付单

- 链路：`PaymentOrderService.CreatePaymentOrder` 先查 latest payment，再决定复用/关闭/新建。
- 证据点：`logic/payment_order_service.go:212-237`，`db/query/payment_order.sql:1-26`。
- 风险条件：同一业务对象并发发起支付创建，两边都在插入前看到没有可复用 pending 单。
- 现有保护：`out_trade_no` 方向有唯一性；前端确认页有 `loading` 与创建订单幂等键。
- 候选原因：通用 `(order_id/reservation_id, business_type, active status)` 级别的数据库唯一保护未在已读 SQL 中看到。
- 待验证：实际迁移里是否有部分唯一索引；压测并发 `create payment`。

### 2. 多支付单成功：同一订单/预约可能被不同支付单重复支付

- 链路：多个 payment_order 分别回调/查询成功，各自进入 `ProcessPaymentSuccessTx`。
- 证据点：`db/sqlc/tx_payment_success.go:42-53` 锁支付单本身，`db/sqlc/tx_create_order.go:390-413` 锁订单并只允许 pending -> paid。
- 风险条件：候选 1 已产生多个有效支付单，随后两个 provider 成功事件先后到达。
- 现有保护：订单支付处理本身幂等，第二次不会重复激活订单。
- 候选原因：第二个支付单仍可能代表用户真实扣款；业务层是否自动补偿/退款需继续追。

### 3. 支付超时关闭与支付成功回调/查询竞态

- 链路：`payment_order:timeout` 查询远端，成功则记 paid/fact，否则关闭远端和本地；另有批量清理直接关闭过期 pending。
- 证据点：`worker/task_payment_timeout.go:86-123`，`worker/task_payment_timeout.go:205-227`，`worker/task_payment_timeout_baofu.go:81-147`，`db/query/payment_order.sql:153-174`，`db/query/payment_order.sql:218-222`。
- 风险条件：本地关闭、远端关闭、远端成功、回调事实到达顺序不一致。
- 现有保护：更新 paid/closed 均要求本地 `pending`；Baofu 超时查询遇到 success 会记录 fact 并跳过关闭。
- 候选原因：`CloseExpiredPaymentOrders` 批量关闭不查远端、不取消业务单，可能与回调/恢复路径产生状态冲突。

### 4. 外部支付事实 dedupe 是事实级，不是业务对象终态级

- 链路：callback/query/manual reconciliation 均可写 external_payment_facts，再创建 application。
- 证据点：`db/query/external_payment_fact.sql:156-225`，`db/query/external_payment_fact.sql:257-275`。
- 风险条件：同一外部对象产生多条不同 source/event/upstream 状态事实，多个 application 针对同一业务对象执行。
- 现有保护：dedupe_key 唯一；application 唯一键含 `fact_id`，同一事实不会重复应用。
- 候选原因：跨事实的同一业务对象终态冲突依赖各 domain 的幂等处理，需逐域核查。

### 5. 支付事实应用与 outbox/terminalized/application applied 不是同一事务

- 链路：claim application -> get fact -> mutate domain -> create outbox -> mark fact terminalized -> mark application applied。
- 证据点：`logic/payment_fact_application_service.go:190-249`。
- 风险条件：domain mutation 成功后，outbox 或 terminal mark 失败，retry 重入 domain。
- 现有保护：订单/预约支付应用有部分 processed 状态重建；outbox 有 `CreatePaymentDomainOutboxOnce`。
- 候选原因：各分支重入鲁棒性不完全一致，特别是退款/分账回退等分支仍需逐条验证。

### 6. 预约状态 SQL 多处无 expected-old-status

- 链路：支付成功、超时取消、商户确认/完成/签到/爽约等均可直接写 reservation status。
- 证据点：`db/query/table_reservation.sql:121-181`，`db/sqlc/tx_payment_success.go:163-168`，`worker/task_reservation_timeout.go`（已读）。
- 风险条件：预约超时取消与支付成功、商户操作与用户动作并发。
- 现有保护：部分事务调用前会读取状态；加菜调整会锁预约。
- 候选原因：SQL 层缺少旧状态条件，后到写入可能覆盖已取消/已完成等状态。

### 7. 预约支付成功可覆盖取消/过期

- 链路：reservation payment fact -> `ProcessPaymentSuccessTx` -> `UpdateReservationStatus(... paid)`。
- 证据点：`db/sqlc/tx_payment_success.go:142-168`，`db/query/table_reservation.sql:121-134`。
- 风险条件：预约 payment_deadline 到期被 scheduler 取消后，provider 成功回调/查询晚到。
- 现有保护：支付单本身要先变 paid；预约支付记录有 payment_order_id 幂等。
- 候选原因：预约本体更新没有 `WHERE status='pending'` 或 `FOR UPDATE` 后状态断言。

### 8. 预约加菜调整：支付超时/cleanup 关闭与支付成功应用竞态

- 链路：reservation_addon payment 成功应用调整；payment timeout 和 data cleanup 会 close/expire adjustment。
- 证据点：`db/sqlc/tx_reservation_adjustment.go:230-347`，`db/sqlc/tx_reservation_adjustment.go:349-394`，`scheduler/data_cleanup.go:1228-1266`，`worker/task_payment_timeout.go:158-166`。
- 风险条件：调整单处于 `pending_payment/applying`，支付成功与清理关闭交错。
- 现有保护：调整单和支付单会 `FOR UPDATE`；状态更新只允许 active -> terminal。
- 候选原因：具体锁顺序、payment_order 与 adjustment 的互斥覆盖范围需要进一步用事务层测试确认。

### 9. 退款终态 success/failed/closed 乱序

- 链路：退款请求 -> pending/processing；provider callback/query fact -> success/failed/closed。
- 证据点：`db/query/refund_order.sql:150-178`，`logic/payment_fact_application_service.go:523-737`。
- 风险条件：success 与 failed/closed 事实乱序到达，或 direct refund worker 与 fact application 并发写终态。
- 现有保护：终态更新只从 pending/processing 进入；closed/failed 分支有部分 terminal conflict reload。
- 候选原因：success 分支的冲突处理不如 failed/closed 分支完整；终态“先到先赢”的业务语义需确认。

### 10. 分账准备与退款创建竞态

- 链路：分账 worker list ready -> transaction recheck active refunds -> processing；同时用户/系统创建退款。
- 证据点：`db/query/profit_sharing_order.sql:227-260`，`db/sqlc/tx_baofu_profit_sharing.go`（已读）。
- 风险条件：分账刚进入 processing 后新退款创建。
- 现有保护：ready list 与 tx 内都会排除 active/success refund。
- 候选原因：分账已发起后退款需要走退分账/退款恢复链路，需确认所有入口都能正确接上。

### 11. 分账 command 与 fact/recovery 竞态

- 链路：prepare command -> 本地 processing -> 外部调用 -> 记录 command outcome -> 更新 sharing_id；callback/query fact 也可更新 finished/failed。
- 证据点：`db/query/profit_sharing_order.sql:291-319`，`logic/payment_fact_application_service.go:854-927`。
- 风险条件：外部已受理但本地 outcome/update 失败；成功/失败事实乱序。
- 现有保护：finished/failed 只从 processing 进入；terminal conflict reload。
- 候选原因：first terminal wins 是否符合 provider 事实语义，以及 command unknown/accepted 与 order terminal 的一致性需继续查。

### 12. 退分账回退与退款启动竞态

- 链路：profit_sharing_return fact -> return success/failed；全部 success 后 `tryInitiateRefundAfterProfitSharingReturns` 将 refund pending -> processing。
- 证据点：`db/query/profit_sharing_return.sql:37-63`，`logic/payment_fact_application_service.go:945-1110`。
- 风险条件：多个 return facts 并发；一个 failed 到达时另一个 success 分支正统计成功数并启动退款。
- 现有保护：refund update 只允许 pending -> processing/failed。
- 候选原因：return 状态 SQL 无 expected-old-status，success/failed 可互相覆盖；退款状态转移依赖后读计数。

### 13. Baofu 提现 dispatch/callback/recovery 竞态

- 链路：submitted command claimed unknown -> 外部提现 -> order processing/failed；callback/recovery fact 也可终态化 order。
- 证据点：`db/query/baofu_withdrawal_order.sql:59-88`，`worker/task_baofu_withdrawal_command_dispatch.go`，`worker/task_baofu_withdrawal_fact_application.go`，`worker/baofu_withdrawal_recovery_scheduler.go`。
- 风险条件：dispatch provider result、callback、query recovery 乱序。
- 现有保护：withdrawal order status update 要求当前 `processing`，终态先到后续跳过。
- 候选原因：command 可能停在 `unknown`，而 order 已 terminal；repair/观测链路需核对。

### 14. 订单支付超时取消与支付成功竞态

- 链路：`OrderTimeoutScheduler` list pending orders -> `CancelOrderTx`；支付成功路径 pending -> paid。
- 证据点：`scheduler/order_timeout.go:57-106`，`db/query/order.sql:256-265`，`db/sqlc/tx_create_order.go:390-413`。
- 风险条件：pending order 超时被取消时，provider 成功晚到或同时到。
- 现有保护：订单支付成功只接受 pending；取消也带 expected old status。
- 候选原因：如果取消先赢，支付成功 fact 会导致 paid payment_order 但订单不可激活，需要退款/补偿路径闭环。

### 15. Data cleanup 批量关闭 payment_orders 绕开远端查询和业务取消

- 链路：`cleanupExpiredPaymentOrders` -> `CloseExpiredPaymentOrders`。
- 证据点：`scheduler/data_cleanup.go:1197-1226`，`db/query/payment_order.sql:218-222`。
- 风险条件：pending 支付过期但远端其实成功或正在支付；业务订单/预约仍 pending。
- 现有保护：payment timeout task 有远端查询；但 data cleanup 是独立路径。
- 候选原因：该 bulk update 只看本地时间和 status。

### 16. 代取过期清理：二次取消 delivery 与支付状态字符串疑似错误

- 链路：stale delivery cleanup -> `CancelOrderTx` -> 再 `UpdateDeliveryToCancelled` -> 查支付单找成功支付并入退款任务。
- 证据点：`scheduler/data_cleanup.go:1311-1405`，`db/sqlc/tx_order_status.go:204-212`。
- 风险条件：`CancelOrderTx` 内已经取消 delivery，scheduler 外层再次取消同一 delivery。
- 现有保护：订单取消事务有 expected status 与 delivery 状态拦截。
- 候选原因：外层查支付成功使用 `p.Status == "success"`，而 payment_order 状态常量是 `paid/refunded/closed/failed/pending`，该分支很可能找不到成功支付；二次 delivery cancel 也可能让整体路径报错。

### 17. 手动配送/地理围栏自动配送状态推进竞态

- 链路：人工取货/送达、地理围栏自动取货/送达、delivery/order 状态事务并存。
- 证据点：`db/query/order.sql:278-351`，`db/sqlc/tx_delivery.go`（已读），`logic/delivery_status.go` 和 `logic/delivery_geofence.go`（已读）。
- 风险条件：同一 delivery/order 被自动与人工入口同时推进。
- 现有保护：核心 DB 更新多为条件更新，delivery/rider 有锁。
- 候选原因：日志、通知、响应构造存在事务外副作用，可能出现重复/滞后。

### 18. 外卖自动完成与用户确认/索赔创建竞态

- 链路：scheduler list delivered -> 查 claims -> auto complete -> log -> schedule profit sharing。
- 证据点：`scheduler/takeout_auto_complete.go:68-116`，`db/query/order.sql:324-351`。
- 风险条件：scheduler 查完无索赔后，用户马上创建索赔；或用户确认收货与 auto complete 并发。
- 现有保护：complete SQL 仅允许 `rider_delivered/user_delivered` -> completed。
- 候选原因：索赔检查和 completed 更新不是同一事务/同一条件。

### 19. 订单/履约状态允许 same-state idempotence，副作用需逐点检查

- 链路：courier accepted、picked、delivering、rider_delivered、user_delivered、complete。
- 证据点：`db/query/order.sql:278-351`。
- 风险条件：重复任务或重复点击导致 same-state update 返回成功。
- 现有保护：时间字段用 `COALESCE`，状态推进有 allowed list。
- 候选原因：调用者如果在事务外发通知/写日志，可能产生重复副作用。

### 20. 堂食结账恢复与客户端关闭会话竞态

- 链路：client checkout close 与 `DineInCheckoutRecoveryScheduler` 同时 `CloseDiningSessionTx`。
- 证据点：`scheduler/dine_in_checkout_recovery.go:79-115`，`db/sqlc/tx_dining_session.go:185-260`。
- 风险条件：paid open session 被恢复任务和前端/接口同时关闭。
- 现有保护：`closeDiningSessionInternal` 看到 `session.Status == "closed"` 会返回。
- 候选原因：读取 session 未见 `FOR UPDATE`；如果两个事务都读到 open，后续 close session/table/billing group/reservation complete 的重复副作用需确认。

### 21. 入驻自动审核 run：cancel 与 complete 乱序

- 链路：queued -> processing；应用编辑可 cancel active runs；worker 可 complete run。
- 证据点：`db/query/onboarding_review.sql:85-135`，`worker/task_onboarding_review.go:42-126`。
- 风险条件：用户编辑取消 processing run，同时 worker 正完成审核并写应用结果。
- 现有保护：SQL terminal update 要求 run_status queued/processing；worker 开头会 skip cancelled/completed。
- 候选原因：worker 在开始后如果应用被编辑取消，后续 service 内是否再次确认 run/current snapshot 需要继续查。

### 22. OCR job lease 与过期重试/旧素材写回竞态

- 链路：OCR job pending/processing lease；lease 过期可被另一 worker claim；provider 结果写回应用资料。
- 证据点：`db/query/ocr_job.sql:64-117`，`worker/ocr_writeback_guard.go`，各 `task_*_ocr.go`。
- 风险条件：旧 OCR 任务结果晚到，或 lease 过期后两个 worker 都拿到 provider 结果。
- 现有保护：`MarkOCRJobProcessing` 有 lease 条件；`CompleteOCRJob/FailOCRJob` 要求 processing；存在 writeback guard。
- 候选原因：guard 是否覆盖所有 owner/document/side，尤其旧媒体替换后 provider 返回的分支需继续核。

### 23. 媒体上传会话完成与过期清理竞态

- 链路：upload session pending -> completed；scheduler/API 可 expire pending；media asset confirm/moderation 与 session complete 分离。
- 证据点：`db/query/media.sql:35-69`，`db/query/media.sql:120-139`。
- 风险条件：客户端上传确认与过期任务同时执行。
- 现有保护：expire 只 pending；asset 查询排除 deleted。
- 候选原因：`CompleteUploadSession` 未要求 `status='pending'`，可能把 expired session 改为 completed；asset 状态更新也没有 expected-old-status。

### 24. 云打印 provider callback 与 status poll/过期处理竞态

- 链路：print_log pending；provider callback mark terminal；poll scheduler claim pending logs 后 query provider 并 terminal；old pending 可 expire/failed。
- 证据点：`db/query/print_log.sql`，`api/yilianyun_callback.go`，`worker/cloud_printer_status_poll_scheduler.go`。
- 风险条件：callback 成功与 poll 失败/过期同时写同一 print_log。
- 现有保护：callback 读取后若非 pending 会跳过；poll 有 claim checked_at/attempts。
- 候选原因：需确认 terminal update SQL 是否带 `status='pending'`，以及 failure/expired 是否可能覆盖 success。

### 25. Baofu 开户/报备/小程序授权多状态链路

- 链路：profile_pending -> verify_fee_pending/processing -> opening_processing -> merchant_report_processing/applet_auth_pending -> ready/failed/voided；同时有支付成功、开户查询、报备查询、回调/恢复。
- 证据点：`logic/baofu_account_onboarding_service.go`，`db/sqlc/constants.go:317-328`，相关 baofu account/report query 尚未完整逐行复核。
- 风险条件：验证费支付成功、开户结果、报备结果、用户重新编辑/重新发起乱序。
- 现有保护：存在 active flow 查询和 reusable verify-fee payment。
- 候选原因：这条链路是高风险资金/资质状态机，当前只粗扫到入口，需单独展开。

### 26. 前端重复触发放大器：确认页总体较好，但确认收货 helper 自身无锁

- 链路：小程序重复点击 -> create order/payment/cancel/confirm receipt。
- 证据点：订单确认页 `loading` 与幂等 key 在 `weapp/.../order-confirm/index.ts:461-501`；支付 workflow 不把 `requestPayment` 当终态在 `payment-workflow.ts:18-24`；取消订单有 `cancelSubmitting` 在 `order-cancel-refund-workflow.ts:237-243`；确认收货 helper 无内建 submitting guard 在 `order-receipt-confirmation.ts:39-65`。
- 风险条件：调用确认收货 helper 的页面未加锁，多次弹窗/多次请求。
- 现有保护：后端 complete SQL 条件更新会阻止重复完成。
- 候选原因：前端可能产生重复请求/重复 toast/重复刷新，后端日志或分账调度是否重复需继续查调用方。

### 27. 支付创建唯一性：迁移证据增强，未见通用业务对象 active 唯一约束

- 链路：同一 order/reservation/business_type 并发创建 payment_order。
- 证据点：`000011_add_payment_orders.up.sql` 只有 `out_trade_no` 唯一和普通 `order_id/reservation_id/status` 索引；`000089` 增加 `transaction_id` 唯一；`000170` 只针对 `claim_recovery` 的 `(business_type, attach)` active 唯一；`000233` 只针对 `baofu_account_verify_fee` 的 `(business_type, attach)` active 唯一。
- 风险条件：主业务 `order`、`reservation`、`reservation_addon` 在应用层先查后建期间出现并发窗口。
- 现有保护：前端部分入口有 loading/幂等键；`out_trade_no` 自身唯一。
- 候选原因：未看到类似 `(order_id, business_type) WHERE status IN active` 或 `(reservation_id, business_type) WHERE status IN active` 的数据库级不变量；需用真实 schema/并发测试确认。

### 28. 微信直连 closed/failed 后到账有异常退款分支，但宝付主业务成功晚到路径语义不同

- 链路：支付单本地 closed/failed 后，provider success callback/query fact 晚到。
- 证据点：微信直连在 `api/payment_callback.go:850-946` 对 closed/failed payment_order 直接触发 `TaskProcessAnomalyRefund`；宝付主业务回调在 `api/baofu_callback.go:202-262` 记录 fact 后进入 payment fact application，`logic/baofu_payment_fact_application.go:14-53` 的 success fact 只允许本地 pending -> paid，若当前不是 paid 会报错。
- 风险条件：宝付本地超时关闭或失败事实先到，随后宝付成功事实到达。
- 现有保护：宝付 success fact 不会静默把 closed/failed 改成 paid；应用会暴露冲突错误。
- 候选原因：宝付主业务未看到类似微信直连的自动 anomaly refund 分支；冲突 fact 是否由恢复/人工对账闭环处理需继续核查。

### 29. 宝付支付 success/closed/failed 终态乱序可能导致 fact application 失败并反复重试/告警

- 链路：Baofu payment callback/query 记录 terminal fact -> application -> 更新 payment_order。
- 证据点：`logic/baofu_payment_fact_application.go:105-132` 的 closed/failed fact 只从 pending 更新，若当前为 paid 会记录冲突并返回错误；`logic/baofu_payment_fact_application.go:14-53` 的 success fact 遇到 current status 非 paid 也返回错误。
- 风险条件：同一外部支付对象产生多个不同 terminal fact，且本地先应用其中一个。
- 现有保护：本地 payment_order 只会先到先写一个终态，不会被后到 fact 覆盖。
- 候选原因：first-terminal-wins 对宝付 provider 事实语义是否正确待验证；后到“更真实”的终态如何解除 failed application、如何补偿资金仍需单独查。

### 30. 宝付 `RecordPaymentFact` 的 fact、fee ledger、application 创建不在已见事务边界内

- 链路：宝付支付 callback/query -> `RecordPaymentFact` -> 创建 external fact -> 写支付手续费 actual ledger -> 创建 fact application。
- 证据点：`logic/baofu_payment_service.go:218-283` 顺序执行 `CreateExternalPaymentFact`、`UpsertOrderPaymentFeeLedgerActual`、`CreateExternalPaymentFactApplication`。
- 风险条件：fact 创建成功后，fee ledger 或 application 创建失败；callback 返回失败后重试。
- 现有保护：`CreateExternalPaymentFact` 在 `dedupe_key` 冲突且所有关键字段一致时 `DO UPDATE ... RETURNING *`，会返回既有 fact；`CreateExternalPaymentFactApplication` 在 `(fact_id, consumer, business_object_type, business_object_id)` 冲突时也返回既有 application；`PaymentFactApplicationScheduler` 每分钟扫描 pending/failed application 重新入队。
- 候选原因：重复 callback 无法补 application 的风险已降低；残余风险集中在 fact、手续费 ledger、application 非同一事务，若中间步骤产生部分副作用，重试语义、费用 ledger 与 fact application 一致性仍需验证。

### 31. 退款旧队列的终态冲突处理弱于 payment fact application

- 链路：微信直连非骑手押金、非预约退款 callback -> `DistributeTaskProcessRefundResult` -> `ProcessTaskRefundResult`。
- 证据点：`api/payment_callback.go:1157-1339` 中非 rider_deposit refund 进入旧队列；`worker/task_process_payment.go:755-848` 只跳过“同状态重复”，随后直接调用 `UpdateRefundOrderToSuccess/Failed/Closed`。
- 风险条件：refund_order 已被 failed/closed 终态写入后，success callback 晚到；或 success 之后 failed/closed 晚到。
- 现有保护：SQL 终态更新通常只允许 pending/processing，避免后到事实覆盖已终态。
- 候选原因：旧队列没有像 `payment_fact_application_service.go` 的 terminal conflict reload/分类逻辑；乱序终态可能变成任务错误、重试或人工告警，而不是明确落入“后到终态已被拒绝”的审计状态。

### 32. 云打印飞鹅云 callback 使用无旧状态条件的通用更新，可能覆盖已终态 print_log

- 链路：print_log pending -> provider callback/poll/expire/task retry 更新状态。
- 证据点：`db/query/print_log.sql:173-182` 的 `UpdatePrintLogStatus` 不带 `status='pending'`；`api/feieyun_callback.go:84-94` 飞鹅云成功 callback 直接调用该更新；相对地，`api/yilianyun_callback.go:127-159` 和 `worker/cloud_printer_status_poll_scheduler.go:180-186` 使用 `MarkProviderStatusPrintLogTerminal`，SQL 带 `AND status='pending'`。
- 风险条件：飞鹅云成功 callback 晚于本地失败/过期/取消状态到达，或重复 callback 与 worker 状态更新交错。
- 现有保护：飞鹅云只处理 success 状态并有签名验证；易联云和 provider poll 路径已有 pending guard。
- 候选原因：同一 print_log 的终态不变量没有统一落在 `UpdatePrintLogStatus` 层；需继续核查打印任务正常提交失败路径是否也可能后到覆盖 callback success。

### 33. 确认收货调用方也未见 submitting guard，重复点击可放大后端完成/分账调度竞态

- 链路：订单详情页/配送跟踪页 -> `confirmReceiptWithRecovery` -> backend confirm order。
- 证据点：helper 本身无锁：`order-receipt-confirmation.ts:39-65`；详情页调用前未见 `confirmSubmitting`：`orders/detail/index.ts:392-404`；跟踪页调用前未见 `confirmSubmitting`：`orders/tracking/index.ts:132-146`。
- 风险条件：用户快速多次点击或多个页面同时确认收货，发出多个 confirm 请求。
- 现有保护：后端订单完成 SQL 有状态条件，预计不会重复把订单完成。
- 候选原因：需要继续核查 backend confirm 是否会重复创建状态日志、重复分账 outbox、重复通知，尤其与自动完成 scheduler 并发时的副作用边界。

### 34. 宝付开户回调 fact 记录与开户流程状态应用在 handler 内串行完成，需单独验证重试幂等

- 链路：宝付开户 callback -> `recordBaofuAccountOpenCallbackFact` -> `applyBaofuAccountOpenCallbackState` -> 可能继续报备恢复。
- 证据点：`api/baofu_account_open_callback_fact.go:20-65` 创建 external fact 后立即调用 `applyBaofuAccountOpenCallbackState`；`api/baofu_account_open_callback_fact.go:190-232` 应用开户结果后尝试继续 merchant report，失败只 warn，交由 recovery。
- 风险条件：fact 创建成功但开户流程应用失败；或 apply 成功后 callback 重投；或开户 active/failed 与用户重新编辑、新 flow 创建交错。
- 现有保护：dedupe_key 使用外部流水/合约号和 upstream_state；未匹配本地 flow 会写平台告警；merchant report continuation 失败会日志提示 recovery。
- 候选原因：该链路没有走 generic fact application 表的 claim/apply/applied 状态；需要逐行核查 `ApplyAccountOpenCallbackResult` 的 expected state、active flow 唯一索引和重复 callback 语义。

### 35. External payment fact upsert 语义降低重复回调缺 application 风险，但不消除跨步骤时序风险

- 链路：provider callback/query/recovery -> `CreateExternalPaymentFact` -> `CreateExternalPaymentFactApplication` -> scheduler/task apply。
- 证据点：`db/query/external_payment_fact.sql` 中 `CreateExternalPaymentFact` 对 `dedupe_key` 冲突执行字段一致性受保护的 upsert 并 `RETURNING *`；`CreateExternalPaymentFactApplication` 对 `(fact_id, consumer, business_object_type, business_object_id)` 冲突也 upsert 返回既有行；`worker/payment_fact_application_scheduler.go` 周期性重新派发 pending/failed applications。
- 风险条件：fact 创建成功、application 创建失败、任务入队失败、或 application 已 failed 但语义冲突持续存在。
- 现有保护：重复同一事实能拿回既有 fact/application；failed application 有 scheduler 重试。
- 候选原因：这里的风险从“重复回调无法补 application”收敛为“多步非事务链路的部分成功、持续失败重试、以及跨事实终态冲突处理”。后续排查应按具体业务 consumer 验证。

### 36. 宝付开户 flow 的失败写入范围较宽，可能与报备/授权恢复链路乱序

- 链路：开户、报备、小程序授权、恢复任务都可能推进 `baofu_account_opening_flows`。
- 证据点：`db/query/baofu_account_opening_flow.sql` 中多数更新带 expected-state guard；但 `MarkBaofuAccountOpeningFlowFailed` 允许除 `ready/voided` 外的任意状态写为 failed；`MarkBaofuAccountOpeningFlowReady` 允许 `opening_processing/merchant_report_processing/applet_auth_pending/ready`，并允许特定 duplicate-failed (`BF00060`、`EXISTED_LOGIN_NO`) 转 ready；`RecoverFailedBaofuAccountOpeningFlowFromActiveBinding` 仅从 `failed/ready` 且匹配 active binding/open_trans/contract_no 时恢复。
- 风险条件：一个 late provider failure 在 flow 已进入 `merchant_report_processing` 或 `applet_auth_pending` 后到达；同时另一路 report/app auth recovery 可能正在把 flow 推向 ready。
- 现有保护：ready/voided 不会被 failed 覆盖；duplicate account 类失败存在恢复到 ready 的专门分支；recoverable list 覆盖 `opening_processing/merchant_report_processing/applet_auth_pending` 和最近 duplicate-failed。
- 候选原因：`failed` 作为 broad terminal 写入可能提前终止仍可恢复的链路，或制造“先 failed 后 ready 恢复”的震荡；需结合 callback/recovery 调用点确认 intended semantics。

### 37. 宝付报备记录 upsert/终态更新缺 expected-old-status，重试可能回退 report/app auth 状态

- 链路：开户成功后继续商户报备；报备结果、小程序授权结果、恢复任务可反复更新 `baofu_merchant_reports` 和 opening flow。
- 证据点：`db/query/baofu_merchant_report.sql` 中 `UpsertBaofuMerchantReportProcessing` 在冲突时重置 `report_state='processing'`、`applet_auth_state='pending'` 并清空失败字段；`MarkBaofuMerchantReportSucceeded/Failed/AppletAuthSucceeded/AppletAuthFailed` 未见 expected-old-status 条件。
- 风险条件：报备或授权已经 succeeded/failed 后，调用方重新提交 report processing；或 success/failure 回调、查询结果乱序到达。
- 现有保护：`logic/baofu_account_merchant_report_service.go` 的结果应用会根据 report/app auth 组合推进 opening flow：失败则 mark flow failed；双成功则事务内 upsert payment config、mark flow ready、activate merchant；report 成功但 app auth pending 会先 upsert config 并标记 `applet_auth_pending`。
- 候选原因：如果调用方没有更强 guard，报备记录可从 terminal 回到 processing/pending，或 late failed 覆盖 succeeded 的 report/app auth 字段；需继续核查 service 入口和 recovery scheduler 是否只在允许状态调用。

### 38. 宝付验证费支付成功后继续开户的链路可能被重复触发，需要核查 prepare/execute 分界

- 链路：verify fee payment fact application -> `ContinueAfterVerifyFeePaid` -> `openFromProfile`；另有可能存在 prepare/execute opening 的 worker/recovery 路径。
- 证据点：`logic/payment_fact_application_baofu_verify_fee.go` 调用 `ContinueAfterVerifyFeePaid`；`logic/baofu_account_onboarding_payment.go`、`logic/baofu_account_onboarding_service.go`、`logic/baofu_account_onboarding_open.go` 中存在验证费支付后继续开户/准备开户方法（尚未完整逐行复核）。
- 风险条件：同一验证费 success fact 重试、scheduler 重派 application、用户重新编辑资料或已有 active flow 时，开户外部调用被重复发起。
- 现有保护：已补读确认，线上路径实际是 `PrepareOpeningAfterVerifyFeePaid` 只把 flow 准备到 `opening_processing` 并返回 `ShouldEnqueueOpening`，`worker/task_baofu_account_opening.go` 再执行 `ExecutePreparedOpening`；任务带 `asynq.Unique(30m)`；flow 有 active owner 唯一索引和 open_trans 唯一索引；`ExecutePreparedOpening` 只有状态仍为 `opening_processing` 才外呼。
- 候选原因：风险从“支付 fact 直接重复外呼”收敛为“prepared flow 外呼已发生但本地 apply/任务失败后，任务重试或恢复再次用同一 open_trans 外呼”。代码中有 duplicate failure reconcile/query 兜底，但 provider 是否对同 open_trans 完全幂等、以及外呼成功后本地失败窗口仍需压测/沙箱验证。

### 39. 宝付开户 flow 有 owner active 唯一索引，但 failed/ready 不在 active 唯一范围，可能允许新旧 flow 与晚到回调交错

- 链路：用户重新提交资料/开户 -> 新建或复用 opening flow；provider callback 可通过 open_trans 或 contract_no 回找 flow。
- 证据点：`000233_add_baofu_account_opening_flows.up.sql:112-121` 对 `(owner_type, owner_id)` 只在 `profile_pending/verify_fee_pending/verify_fee_processing/opening_processing/merchant_report_processing/applet_auth_pending` 建唯一索引；`GetActiveBaofuAccountOpeningFlowByOwner` 同样只返回这些 active 状态；callback 若缺 out_request_no 会用 contract_no 找 binding，再取 owner 的 latest flow。
- 风险条件：旧 flow 已 failed，用户重新提交产生新 active flow；随后旧 provider callback 到达，尤其 callback 只有 contract_no 或被解析到 owner latest flow。
- 现有保护：有 `open_trans_serial_no` 唯一索引；正常有 out_request_no 时 callback 精确匹配 flow；`ensureAccountOpenResultSerialMatchesFlow` 会阻断 result out_request_no 与 flow 不一致并落平台告警。
- 候选原因：当 callback 缺 out_request_no、只靠 contract_no -> binding -> latest flow 匹配时，需验证是否可能把旧外部事实应用到新 flow；当前代码会做合约归属和流水匹配告警，但无 generic application 状态来表达“旧 callback 已收到但未应用”。

### 40. 宝付开户 prepared 外呼缺本地 command accepted 记录，恢复依赖 flow 状态/query 而非 command 状态机

- 链路：`PrepareOpening`/`PrepareOpeningAfterVerifyFeePaid` -> `MarkBaofuAccountOpeningFlowOpeningProcessing` -> enqueue `TaskProcessBaofuAccountOpening` -> `OpenAccount`。
- 证据点：`logic/baofu_account_onboarding_open.go:47-55` 持久化 open_trans/login_no/request snapshot；`worker/task_baofu_account_opening.go:67-91` 执行外呼；已读 `rg` 中旧 `logic/baofu_account_service.go` 有 external command，但 onboarding `openFromPreparedProfile` 未见 `CreateExternalPaymentCommand` 记录开户外呼。
- 风险条件：外呼请求已到 provider，但本地 worker 在调用后、应用结果前崩溃/超时；重试任务或 recovery scheduler 再次处理同一 `opening_processing` flow。
- 现有保护：flow 上持久化 `open_trans_serial_no` 与 request snapshot；recovery scheduler 每 5 分钟查询 `opening_processing`；duplicate failure 会触发 query reconcile；active binding 可恢复 failed flow。
- 候选原因：缺 command 状态意味着无法区分“已提交 provider 等待查询”和“还未真正外呼”，重复执行只能依赖 provider 幂等和后续 reconcile；需验证宝付开户接口对相同 out_request_no 的重复提交语义。

### 41. 宝付回调 fact 只记录 received，不创建 application；应用失败会导致 provider 重投/告警但本地缺可调度 apply backlog

- 链路：宝付账户 callback -> `CreateExternalPaymentFact` -> `ApplyAccountOpenCallbackResult` -> merchant report continuation。
- 证据点：`api/baofu_account_open_callback_fact.go:19-68` 创建 fact 后立即应用状态；未见为 `baofu_account_opening_flow` 创建 `external_payment_fact_applications`；`ApplyAccountOpenCallbackResult` 传 `continueMerchantReport=false`，handler 随后用 `continueBaofuMerchantReportAfterAccountCallback` 单独接报备。
- 风险条件：fact 创建成功后，flow 状态应用失败；或应用成功但报备 continuation 失败；provider 重投与 recovery 查询并行。
- 现有保护：callback handler 返回失败时 provider 可能重投；fact dedupe 可复用既有 fact；报备 continuation 失败会 warn 并交给 recovery；unmatched/mismatch 会写平台告警。
- 候选原因：账户 callback 与通用 payment fact application 的可观测/可重试模型不一致；如果 provider 不重投或重投被 dedupe 后应用仍失败，没有 application backlog 供 scheduler 统一追踪。

### 42. 宝付报备 recovery 只扫描 report 表，不回写 opening flow；双 recovery scheduler 可能形成状态展示滞后

- 链路：`BaofuMerchantReportRecoveryScheduler` 扫 `baofu_merchant_reports` 处理 report/app auth；`BaofuAccountOpeningRecoveryScheduler` 扫 opening flow 并调用 `RecoverMerchantReportFlow`，后者会 apply opening flow。
- 证据点：`worker/baofu_merchant_report_recovery_scheduler.go:108-123` 只调用 `BaofuMerchantReportService.RecoverWechatMerchantReport`，该 service 只更新 `baofu_merchant_reports`；`logic/baofu_account_merchant_report_service.go:113-319` 才根据 report 结果把 opening flow 推进 failed/applet_auth_pending/ready。
- 风险条件：独立 report recovery 已把 report/app auth 标成 succeeded/failed，但 opening flow 仍停在 `merchant_report_processing/applet_auth_pending`，直到另一条 account opening recovery 或 callback continuation 再应用。
- 现有保护：account opening recovery 每 5 分钟会扫 flow 并调用 `RecoverMerchantReportFlow`；前端读 active binding/report 时部分路径能展示 readiness。
- 候选原因：两条 scheduler 的写边界不一致，可能导致短期状态滞后；若 account opening recovery 配置缺失或异常，report 表已 terminal 但 flow/merchant activation/payment config 不一定同步推进。

### 43. 宝付 merchant report 提交命令先写 submitted，再 upsert 本地 report，再外呼，存在部分提交/重试语义窗口

- 链路：`SubmitWechatMerchantReport` -> `CreateExternalPaymentCommand(submitted)` -> `UpsertBaofuMerchantReportProcessing` -> `SubmitWechatReport` -> query/bind/apply。
- 证据点：`logic/baofu_merchant_report_service.go:98-125` 顺序执行 command、report upsert、provider submit；`UpsertBaofuMerchantReportProcessing` 冲突时会替换 report_no 并重置状态。
- 风险条件：command 写成功但 report upsert 失败；report upsert 成功但 provider submit 失败；调用方重试时生成新的 report_no 并覆盖旧 report_no。
- 现有保护：`RecoverMerchantReportFlow` 只有 report 不存在时才 `submitMerchantReport`；report 存在且 processing 时走 query，不会每次都 submit；`report_no` 自身唯一，owner/report_type 唯一。
- 候选原因：如果第一次外呼其实已被 provider 受理但本地返回错误，后续是否仍查询旧 report_no 取决于 upsert 是否成功；若 upsert 前失败则只有 command 记录而 report 缺失，后续可能用新 report_no 再提交。

### 44. 宝付 binding active 与 flow ready/merchant activation 有跨事务差异，merchant 侧 ready 事务不包含 binding 激活

- 链路：开户 active result -> `applyAccountOpenActive` 激活 binding；merchant flow 进入 report；report/app auth 双成功后 `MarkMerchantBaofuAccountOpeningReadyTx` 事务内 upsert payment config、mark flow ready、activate merchant。
- 证据点：`logic/baofu_account_onboarding_apply.go:343-409` 激活 binding；`logic/baofu_account_merchant_report_service.go:275-295` 调 ready tx；`db/sqlc/tx_baofu_account_opening_ready.go:19-47` 事务只覆盖 payment config、flow ready、merchant activate。
- 风险条件：binding active 已提交，但 flow/report/merchant activation 后续失败；或 ready tx 部分失败回滚后，binding 已 active 但 flow 仍 processing。
- 现有保护：后续 recovery 可从 active binding 恢复 failed flow；merchant report recovery/account opening recovery 可继续推进；ready tx 对 payment config/flow/merchant 是同事务。
- 候选原因：开户绑定与报备 ready 是分阶段事务，不是单一状态机事务；需验证各读路径是否能正确表达“binding active 但 merchant report 未 ready”的中间态，避免误以为可收款。

### 45. 打印任务 provider 返回与 callback 到达之间存在短窗口，可能出现 callback 暂时无法匹配本地 print_log

- 链路：`executePrintAttemptWithProvider` 先 `CreatePrintLog(pending, provider_origin_id)`，再调用 provider `Print`，最后 `UpdatePrintLogStatus` 写 vendor_order_id/status；provider callback 可能在本地最后一次 update 前到达。
- 证据点：`worker/task_print_attempt.go:40-63` 创建 print_log 后外呼；`worker/task_print_attempt.go:63-130` 才写 vendor_order_id/status；飞鹅云 callback 用 `GetPrintLogByVendorOrderID`；易联云 callback 用 `GetPrintLogByProviderAndOriginID`，但还会校验本地 `vendor_order_id` 与 callback order_id 一致。
- 风险条件：provider 极快 callback 早于本地写入 vendor_order_id；飞鹅云查不到 vendor_order_id，易联云因本地 vendor_order_id 未就绪而 mismatch。
- 现有保护：飞鹅云/易联云未知 callback 都返回 `FAIL`/409，注释与日志语义是要求 provider 重试；`provider_origin_id` 有唯一索引，`task_key, printer_id` 有唯一索引。
- 候选原因：该窗口不一定丢终态，但依赖 provider 重投；若 provider 不按预期重投或回调重试窗口短，print_log 可能长时间 pending/failed，需靠 poll/人工重试/异常告警收敛。

### 46. 飞鹅云 callback 无 pending guard，后到成功可覆盖本地 failed/cancelled 终态

- 链路：飞鹅云 callback -> `GetPrintLogByVendorOrderID` -> `UpdatePrintLogStatus(success)`。
- 证据点：`api/feieyun_callback.go:72-94` 直接调用 `UpdatePrintLogStatus`；`db/query/print_log.sql:173-182` 不带 `AND status='pending'`；相对地易联云 callback 在 `api/yilianyun_callback.go:127-159` 先判断非 pending 直接 ACK，并用 `MarkProviderStatusPrintLogTerminal` 的 pending guard。
- 风险条件：本地已因 provider error、人工取消、retry 记录或其他路径将同一 print_log 标记 failed/cancelled，飞鹅云成功 callback 晚到。
- 现有保护：飞鹅云只处理 status=1 的成功回调并做签名校验；如果后到 success 代表 provider 最终确实打印成功，覆盖 failed 可能是业务上可接受的纠偏。
- 候选原因：同一 print_log 终态“后到 success 能否覆盖 failure”未在 SQL 层统一表达，且与易联云/poll 的 first-terminal-wins 语义不一致；需确认产品希望“成功优先”还是“终态先到先赢”。

### 47. 打印 retry 会创建新 print_log，旧 provider callback 晚到可能改变旧记录但不改变最新异常视图

- 链路：商户重试 print job -> `retryPrintLog` 读取旧 log -> `executePrintAttemptWithProvider` 创建新 log；旧 provider callback 仍可能更新旧 log。
- 证据点：`worker/task_print_order.go:154-185` retry 使用旧内容创建新尝试；`print_log.sql:98-125` 异常列表按 `(order_id, printer_id)` 取最新 log；`GetPrintLogByVendorOrderID` 按 vendor_order_id 找最新匹配记录。
- 风险条件：旧打印任务 provider 成功 callback 晚于商户重试，旧 log 被标 success，但新 log 可能 failed/pending；或旧 success 与新 failed 同时存在。
- 现有保护：商户异常列表只看每个订单/打印机最新 log，不会被旧 log success 直接清掉新失败；retry 有 `task_key, printer_id` 唯一保护。
- 候选原因：历史记录语义可能让一个实际已打印成功的旧任务被新失败覆盖为“仍有异常”，也可能导致重复打印；需要从产品角度确认“最新尝试优先”是否符合预期。

### 48. 打印任务 task_key 幂等是按 task/printer，不覆盖手动多次触发或不同 trigger 间重复打印

- 链路：accepted/ready/manual/retry 多入口派发打印任务。
- 证据点：`worker/task_print_order.go:267-278` legacy key 为 `order-legacy:<order_id>:<trigger>` 或 retry key；`000180_add_print_log_task_key.up.sql` 只对 `(task_key, printer_id)` 建唯一索引；manual trigger 如果无 task_key 会返回空 key。
- 风险条件：同一订单不同 trigger 或多次 manual 触发打印，生成不同/空 task_key，都会创建新 print_log 并外呼 provider。
- 现有保护：这是可能的产品能力；retry/manual 需要允许主动重打。
- 候选原因：若上游 accepted 与 ready 触发配置/任务重放交错，可能放大重复打印；需确认所有自动触发的 task_key 都稳定且上游不会同时发 accepted/ready 两套任务。

### 49. 确认收货并发重复请求的失败分支可能不是业务幂等响应

- 链路：小程序订单详情/配送跟踪页 -> `confirmReceiptWithRecovery` -> `OrderService.ConfirmOrder` -> `ConfirmTakeoutOrder`。
- 证据点：`logic/confirm_order.go:26-53` 先 `GetOrder` 再条件更新；`db/query/order.sql:324-335` 只允许 `rider_delivered/user_delivered` -> `completed`；`api/order.go:1065-1075` 仅把 logic request error 映射成业务响应，其它错误走 500。
- 风险条件：两个确认请求几乎同时读取到 `rider_delivered/user_delivered`，第一个更新成功，第二个条件更新无行返回。
- 现有保护：订单完成 SQL 有旧状态条件，预计不会重复完成订单，也不会重复进入后续通知/分账调度。
- 候选原因：并发输掉的一方可能拿到 `ErrRecordNotFound` 类数据库错误并被 API 作为 500 返回，而不是重新读取后返回 already completed；需用并发测试或日志确认实际错误类型和用户表现。

### 50. 桌台二维码生成使用 `table_no` 直拼 scene，Mini Program 解析会截断合法含 `-` 桌号

- 链路：`generateTableQRCode` 拼出 `m_<merchantID>-t_<tableNo>`；Mini Program `scan-entry` 只用 `/t_([^-]+)/` 解析 `scene`。
- 证据点：`locallife/api/scan.go:528-535`；`weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:24-35`。
- 风险条件：合法 `table_no` 含 `-` 时，二维码生成仍会成功，但扫码后 `table_no` 会被截断成第一个 `-` 之前的前缀，进而落到错误桌台查询或 404。
- 现有保护：当 `scene` 超过 32 字符时会切换到 `tid_<tableID>`，但这不覆盖短串含 `-` 的桌号。
- 候选原因：这是后端生成与前端解析对 `scene` 语法理解不一致造成的确定性时序/协议错位，不是单纯的输入校验缺失。

### 51. 桌台二维码生成先落对象存储与 media_assets，再写 tables.qr_code_url，回写失败后重试不自愈

- 链路：`generateTableQRCode` 先调用微信生成码，再 `storeTableQRCode` 上传 OSS + 插入 `media_assets`，最后 `UpdateTable(... qr_code_url)`。
- 证据点：`locallife/api/scan.go:546-564`；`locallife/api/scan.go:552-564`；`locallife/db/migration/000140_create_media_assets.up.sql` 的 `object_key` 唯一索引。
- 风险条件：二维码文件和 `media_assets` 已成功写入，但 `UpdateTable` 在回写 `qr_code_url` 时失败。此后同一二维码重试会撞同一 `object_key`，当前实现没有 object-key 复用分支可把旧 asset 自愈回桌台。
- 现有保护：`table.QrCodeUrl` 已有当前版本时可直接返回，且 `UpdateTableTx` 会在改桌号时清空旧 QR；但这条 API 路径本身不是事务化的。
- 候选原因：持久化对象和桌台指针分离，失败后只留下可见资产，没有把“已生成但未回写”的半状态收敛进可恢复流程。

### 52. 桌台编辑页改桌号后事务分支会清空 QR，但直写更新入口仍存在，因此安全性取决于真实调用面

- 链路：`updateTable` 在带 `tag_ids` 或需要未来预订保护时走 `UpdateTableTx`；`UpdateTableTx` 遇到 `table_no` 变化会清空 `qr_code_url`。
- 证据点：`locallife/api/table.go:816-883`；`locallife/db/sqlc/tx_table.go:134-160`。
- 结论：当前商户编辑页在保存时总会带 `tag_ids`，因此改桌号路径会进入事务清 QR 分支；`updateTable` 的普通直写分支只在不带 `tag_ids` 且不触发未来预订保护时存在，但仓库内已扫到的编辑/生成入口没有绕过它去保留旧 QR 的实际调用。

### 53. 商户二维码海报与当前页表单状态只是参数拼接，不会自行混用旧 QR 图片

- 链路：`openTableQRCode` 取当前表单桌号和已加载二维码 URL 组成展示上下文；`saveTableQRCodePosterToAlbum` 只按入参绘制海报并保存。
- 证据点：`weapp/miniprogram/pages/merchant/tables/shared/table-edit-view.ts:300-304`；`weapp/miniprogram/pages/merchant/tables/edit/index.ts:488-539`；`weapp/miniprogram/pages/merchant/_utils/merchant-tables-shared.ts:605-613`。
- 结论：海报函数不是独立状态源，不会自己拉取旧桌号或旧二维码；如果页面上出现桌号/二维码错位，根因仍应回到上游 `qrCodeImageUrl`/`qrCodeTableNo` 的同步时机，而不是海报工具本身。

### 54. 用户确认收货与自动完成的核心状态互斥较好，但输掉方与副作用边界需确认

- 链路：用户确认 `CompleteTakeoutOrderByUser`；scheduler 自动完成 `AutoCompleteTakeoutOrder`；两边成功后都会写日志并尝试调度宝付分账。
- 证据点：`logic/confirm_order.go:51-72`，`scheduler/takeout_auto_complete.go:96-116`，`db/query/order.sql:324-351`。
- 风险条件：用户确认和自动完成同时处理同一外卖订单。
- 现有保护：两条 SQL 都只允许 `rider_delivered/user_delivered` 进入 `completed`，因此通常只有一个入口能成功执行通知/调度后续动作。
- 候选原因：失败方如何处理数据库无行返回仍需核查；自动完成的“无索赔”检查与完成更新不在同一事务内，仍保留候选 18 中的索赔创建竞态。

### 55. 订单完成状态日志在事务外且错误被忽略，可能造成状态与审计轨迹不一致

- 链路：确认收货或自动完成成功更新订单后，调用 `CreateOrderStatusLog` 写状态日志。
- 证据点：`logic/confirm_order.go:55-64` 使用 `_, _ = store.CreateOrderStatusLog(...)`；`scheduler/takeout_auto_complete.go:105-113` 同样忽略日志错误。
- 风险条件：订单更新成功后日志写入失败、超时、连接中断，或进程在状态更新后日志写入前退出。
- 现有保护：核心订单状态已经完成；日志失败不会阻塞用户确认或 scheduler。
- 候选原因：运行排查时可能出现订单已 `completed` 但缺少对应完成日志，影响审计、客服追踪和后续基于日志的自动化判断；需确认是否有日志补偿/对账。

### 56. 完成订单后的宝付分账调度依赖事务外 best-effort 入队，短期唯一性不足以表达持久交付

- 链路：`OrderService.ConfirmOrder` 成功后 `scheduleBaofuProfitSharingForCompletedOrder`；自动完成 scheduler 成功后也调度分账；`apiTaskScheduler.ScheduleProfitSharing` 入队 `baofu:process_profit_sharing`。
- 证据点：`logic/order_service.go:651`、`logic/order_service.go:855-869`；`scheduler/takeout_auto_complete.go:114`；`api/logic_adapters.go:166-177` 使用 `asynq.Unique(30*time.Second)`。
- 风险条件：订单已完成但分账任务入队失败，或 30 秒 unique 窗口内重复触发被去重后原任务失败/丢失；也可能在窗口外被重新调度。
- 现有保护：`ListBaofuProfitSharingOrdersReadyForCommand` 会扫描 completed 且 ready 的 pending/failed 分账订单；分账 worker 自身只接受 `pending/failed`，`finished` 直接返回。
- 候选原因：这里更像最终一致性窗口而非重复分账直接风险；需确认恢复 scheduler 的频率、失败分账是否能稳定回到 ready list，以及入队失败是否有监控。

### 57. 宝付分账 command 创建、外呼、accepted 记录和 sharing_id 更新不是一个原子状态机

- 链路：分账 worker 读取 pending/failed -> `PrepareBaofuProfitSharingCommandTx` 将分账单置 processing -> `CreateExternalPaymentCommand(submitted)` -> 外呼 `CreateProfitSharing` -> 记录 command outcome -> `UpdateProfitSharingOrderSharingID`。
- 证据点：`worker/task_baofu_profit_sharing.go:90-188`；`db/query/profit_sharing_order.sql:291-319`。
- 风险条件：本地已置 processing 但 command 创建失败；外呼已被 provider 受理但 outcome 记录或 sharing_id 更新失败；callback/query fact 先于本地 sharing_id 更新到达。
- 现有保护：prepare 在事务内排除 active/success refund；分账终态更新只允许 `processing` -> `finished/failed`；command outcome 有 `unknown/accepted/rejected` 区分。
- 候选原因：processing 之后的多步骤依赖 command/recovery/fact application 共同收敛，仍需验证“外呼成功但本地失败”的恢复路径是否能通过 out_order_no 查询并补齐 sharing_id/终态，避免 processing 长期悬挂或重复外呼。

### 58. 退款 fact application 的 success 分支对 late failed/closed 冲突处理不对称

- 链路：宝付退款 callback/query -> external payment fact -> `applyOrderRefundFact` / `applyReservationRefundFact`。
- 证据点：`logic/payment_fact_application_service.go:523-577`、`logic/payment_fact_application_service.go:636-702`；`db/query/refund_order.sql:158-178`。
- 风险条件：退款单已被 failed/closed 终态写入后，provider success fact 晚到；或 failed/closed fact 与 success fact 并发。
- 现有保护：SQL 只允许 `pending/processing` -> `success/failed/closed`；failed/closed 分支遇到已有其它终态会 reload 并返回 nil，避免覆盖。
- 候选原因：success 分支没有先判断“已有 failed/closed 终态则分类处理”，会直接调用 `UpdateRefundOrderToSuccess` 并把无行更新当错误返回；这可能让后到 success application 持续 failed/retry，而不是进入明确的终态冲突审计。

### 59. 预约退款 success 与预约预付金额回退不是同事务，失败后重试可能不补金额

- 链路：reservation refund fact success -> `UpdateRefundOrderToSuccess` -> `maybeMarkPaymentOrderRefunded` -> `AddReservationPrepaidAmount(-refund_amount)`。
- 证据点：`logic/payment_fact_application_service.go:636-664`。
- 风险条件：退款单成功更新后，`maybeMarkPaymentOrderRefunded` 或 `AddReservationPrepaidAmount` 失败，application 返回错误并重试。
- 现有保护：重复 success fact 看到 refund_order 已 `success` 时不会重复扣减预付金额，避免二次扣减。
- 候选原因：保护重复扣减的同时也带来补偿缺口：若第一次在 refund_order 已 success 后、预付金额扣减前失败，重试时 `transitionedToSuccess=false`，不会再执行 `AddReservationPrepaidAmount`；需验证是否有独立对账修复预约 prepaid_amount。

### 60. 旧微信直连退款 result worker 与新 fact application 的终态冲突语义不一致

- 链路：微信退款 callback 非骑手押金路径 -> `DistributeTaskProcessRefundResult` -> `ProcessTaskRefundResult`；骑手押金/宝付路径走 fact application。
- 证据点：`api/payment_callback.go:1287-1325`；`worker/task_process_payment.go:755-848`；`logic/payment_fact_application_service.go:523-737`。
- 风险条件：direct 非骑手押金退款 success/abnormal/closed 回调乱序，或同一 out_refund_no 被旧 worker 与恢复任务分别处理。
- 现有保护：旧 worker 会跳过“同状态重复”；SQL 条件更新避免后到终态覆盖已终态。
- 候选原因：旧 worker 对“不同终态后到”的处理是直接更新失败并由 asynq 重试/告警；新 fact application 对 failed/closed 后到已有其它终态会部分归类为 nil。两套语义不一致，排障时同类退款乱序会表现成不同状态。

### 61. refund recovery 对 stuck processing 只建 modeled fact，unsupported 类型只告警不落终态

- 链路：`RefundRecoveryScheduler.recoverStuckProcessingRefunds` 查询 provider 终态；骑手押金 direct 与宝付 order/reservation 建 fact application；其它业务类型落平台告警。
- 证据点：`worker/refund_recovery_scheduler.go:284-390`、`worker/refund_recovery_scheduler.go:394-424`。
- 风险条件：某类退款进入 processing，但没有 fact application target；provider 查询已 terminal。
- 现有保护：会写 critical 平台告警，避免静默吞掉。
- 候选原因：告警不是状态收敛；该退款单仍可能停在 processing，后续账务、用户展示、分账阻断会持续依赖人工处理。需盘点是否还有 direct 非骑手押金、历史业务或替换订单退款落在 unsupported 分支。

### 62. 宝付退款创建路径接受“已存在/处理中”语义，但 command/outcome 与 refund_order 状态可能分离

- 链路：商户拒单/商户退款/替换订单退款创建 `refund_order` 后外呼宝付 `CreateRefund`，再更新 refund_order processing 或 failed，并记录 external command。
- 证据点：`logic/merchant_reject_refund.go:185-231`；`logic/refund_service.go:56-147`、`logic/refund_service.go:318` 附近；`logic/replace_order.go:352-412`。
- 风险条件：外呼已被宝付受理但本地 `UpdateRefundOrderToProcessing` 或 command 记录失败；或 provider 返回“订单已存在/处理中”一类语义，本地需要转为 processing 等 query recovery。
- 现有保护：已有测试覆盖 `CreateRefundOrder_BaofuOrderExistMarksProcessingForQueryRecovery`、`RetryableProviderErrorDoesNotFailRefund` 等场景；stuck processing recovery 会查询宝付并建 fact。
- 候选原因：不同入口的 command 记录和失败处理不完全一致，特别是替换订单退款在 `processReplaceOrderRefundWithBaofu` 返回错误时才记录 rejected，而外呼成功后的 command accepted 记录在调用方分支；需确认所有入口都能在“外呼成功、本地失败”窗口通过同一个 out_refund_no 恢复。

### 63. 宝付分账 processing 恢复查询会补 fact，但 provider ORDER_NOT_EXIST 可直接把分账单转 failed

- 链路：`BaofuPaymentRecoveryScheduler` 扫描 processing 分账单 -> `QueryProfitSharing` -> 记录 share fact 或在特定不存在错误时标 failed。
- 证据点：`worker/baofu_payment_recovery_scheduler.go:339-389`；`worker/baofu_profit_sharing_recovery_query.go:15-27`；`db/query/profit_sharing_order.sql:621-630`。
- 风险条件：分账外呼已被 provider 接收，但查询接口在短窗口或用错 key 返回 `ORDER_NOT_EXIST`；或本地 `sharing_order_id` 未补齐时只能按 `out_order_no` 查。
- 现有保护：如果有非 out_order_no 的真实 `sharing_order_id`，`ORDER_NOT_EXIST` 不会直接转 failed；查询会在 tradeNo 失败时按 outTradeNo 重试；只有 processing 且满足保护条件才转 failed。
- 候选原因：该策略把部分“不存在”解释成可恢复失败，但仍需用宝付实际一致性窗口验证，避免 provider 最终成功但本地已 failed，后续 callback/fact 再变成 late success 冲突。

### 64. 分账成功 fact 有金额一致性保护，但金额缺失会导致 application 持续失败

- 链路：宝付 share callback/query -> `RecordShareFact` -> `applyProfitSharingSuccessFact`。
- 证据点：`logic/baofu_profit_sharing_service.go:200-244` 要求 success amount > 0；`logic/payment_fact_application_service.go:787-817` 校验 fact amount 等于本地 expected share amount。
- 风险条件：provider success callback/query 不带成功金额，或金额口径与本地账单口径不同；或者本地账单在 pending/failed 状态被刷新，外部 success 对应旧金额。
- 现有保护：金额不一致不会把分账单错误标 finished，会暴露为 application 错误/重试。
- 候选原因：这属于强一致保护带来的恢复压力；需确认 provider 成功事件金额字段稳定，以及金额不一致时是否有人工对账和 application 终止策略，避免无限重试噪声。

### 65. `profit_sharing_returns` 仍有 fact application 处理，但当前已读源码未见创建/外呼/恢复闭环

- 链路：历史分账回退记录表 -> profit sharing return fact application -> 全部回退成功后启动 refund processing。
- 证据点：`db/query/profit_sharing_return.sql` 仍定义 create/get/update/list；`logic/payment_fact_application_service.go:945-1110` 仍处理 `profit_sharing_return` fact；`api/logic_adapters.go:179-185` 明确跳过 legacy return result scheduling；当前 `rg` 未在非测试业务代码中发现 `CreateProfitSharingReturn` 调用。
- 风险条件：线上仍存在旧 `profit_sharing_returns` pending/processing 数据，或旧入口/数据迁移残留等待回退结果。
- 现有保护：若已经有 fact application 到达，应用层仍能更新 return/refund；API 还能列出 refund 下的 returns。
- 候选原因：这像是支付通道迁移后的残留状态机；如果有历史 processing return，当前源码中未见定时查询/重新派发闭环，可能长期悬挂，需要用生产数据确认是否还有非终态行。

### 66. 分账回退 return 状态 SQL 无旧状态条件，success/failed/processing 可互相覆盖

- 链路：profit sharing return fact application 根据 external fact 更新 return 状态，并可能更新 refund_order。
- 证据点：`db/query/profit_sharing_return.sql:37-63` 三个 update 都只按 `id` 更新；`logic/payment_fact_application_service.go:961-1003` 会先按 return_id 标 processing，再按 success/failed 写终态。
- 风险条件：同一 return 的 success/failed fact 乱序；或已 success 后又收到 failed/closed fact；或已 failed 后收到带 return_id 的非终态/终态 fact。
- 现有保护：应用层进入 processing 前有 `returnRecord.Status != "success" && returnRecord.Status != "failed"` 判断；success/failed 分支也避免同状态重复更新。
- 候选原因：保护在应用层、不是数据库不变量；并发或跨 worker 乱序仍可能让后到 fact 覆盖先到终态，且 refund_order 可能随 failed 分支被置 failed，需补并发测试或迁移判断该旧模型是否仍会运行。

### 67. 分账回退成功后启动退款与 return/refund 状态统计不是同事务，存在统计窗口

- 链路：return success fact -> `UpdateProfitSharingReturnToSuccess` -> `tryInitiateRefundAfterProfitSharingReturns` 统计 total/success/failed -> `UpdateRefundOrderToProcessing`。
- 证据点：`logic/payment_fact_application_service.go:974-980`、`logic/payment_fact_application_service.go:1049-1110`。
- 风险条件：多个 return facts 并发应用，一个 success 分支统计时另一个 failed 正在写入；或 refund_order 已被其它路径处理/failed。
- 现有保护：退款单只有 `pending` 时才尝试进入 processing；`UpdateRefundOrderToProcessing` SQL 只允许 pending -> processing。
- 候选原因：return 状态无 DB expected-old-status 与统计分离叠加，会让“全部 success 后启动退款”和“任一 failed 后退款 failed”的先后语义依赖执行顺序；虽然当前看像 legacy 模型，仍需生产数据确认是否还会触发。

### 68. 宝付开户核验费支付单写回 `prepay_id` 失败会留下可复用但不可重建签名参数的 pending 单

- 链路：`ensureVerifyFeePayment` 先建 `payment_order`，再调微信直连下单，最后 `UpdatePaymentOrderPrepayId`；`PrepareOpening` / `PrepareOpeningAfterVerifyFeePaid` 之后回到 `addPaymentFromFlow` 只在 `status='pending' && prepay_id` 存在时重建 `pay_params`。
- 证据点：`logic/baofu_account_onboarding_payment.go:23-31`，`logic/baofu_account_onboarding_payment.go:48-90`，`logic/baofu_account_onboarding_service.go:276-294`，`api/baofu_settlement_account_response.go:134-166`，`db/query/payment_order.sql:118-119`。
- 风险条件：微信下单已成功返回 `prepay_id`，但本地 `UpdatePaymentOrderPrepayId` 写回失败；随后用户刷新开户页或恢复流程时，`GetReusableBaofuVerifyFeePayment` 仍会命中这笔 pending 单，但 `prepay_id` 为空，无法重新生成支付参数。
- 现有保护：`UpdatePaymentOrderPrepayId` 要求 `status='pending' AND prepay_id IS NULL`，避免重复覆盖；宝付开户 flow recovery 会继续扫描 `verify_fee_pending/verify_fee_processing`。
- 候选原因：这条链路把“下单成功”和“写回可复用签名参数”拆成两个阶段，第二阶段失败后不会自动补写 `prepay_id`，而复用分支又把缺失 `prepay_id` 当成可直接返回的半状态，导致前端只能看到一笔 pending 记录但拿不到 `pay_params`。

### 69. 宝付提现创建后不立即派发 command，依赖 recovery scheduler 扫 submitted command

- 链路：提现 API -> `BaofuWithdrawService.CreateWithdrawal` -> `CreateBaofuWithdrawalOrderWithSubmittedCommandTx` -> recovery scheduler `enqueueSubmittedCommands` -> dispatch worker 外呼宝付。
- 证据点：`logic/baofu_withdraw_service.go:124-198`；`db/sqlc/tx_baofu_withdrawal.go:23-57`；`api/baofu_withdrawal.go:304-315` 未见立即派发；`worker/baofu_withdrawal_recovery_scheduler.go:125-143` 扫描 submitted command。
- 风险条件：scheduler 未运行、任务队列异常、或 submitted command 扫描延迟；用户已收到 202 “提现申请已提交”，但 provider 侧尚未收到提现请求。
- 现有保护：recovery scheduler 每 5 分钟运行，command 和 withdrawal_order 在本地同事务创建；submitted command 可重复扫描入队，task 有 30 分钟 unique。
- 候选原因：这是设计上的最终一致性窗口，但在运营语义上容易误解为“已提交宝付”；需确认监控是否覆盖 submitted command 滞留时间。

### 70. 宝付提现 command claim 后变 unknown，若入队任务丢失/耗尽重试，recovery 不再扫描 unknown command

- 链路：dispatch worker `ClaimSubmittedExternalPaymentCommandForDispatch` 把 command 从 submitted 改 unknown，再调用 `CreateWithdraw`。
- 证据点：`worker/task_baofu_withdrawal_command_dispatch.go:130-146`；`db/query/external_payment_fact.sql:114-125`；`worker/baofu_withdrawal_recovery_scheduler.go:125-143` 只列 `command_status='submitted'`。
- 风险条件：command 被 claim 成 unknown 后，worker 在外呼前崩溃、任务丢失、或重试耗尽；由于 withdrawal_order 已是 processing，后续 recovery 会查 provider，但 provider 可能根本没收到该 out_request_no。
- 现有保护：asynq 任务通常会重试；`ProcessTaskBaofuWithdrawalCommandDispatch` 支持处理 unknown command，并可根据 withdrawal_order 修复 command outcome；processing withdrawal recovery 会持续查询 provider。
- 候选原因：如果外呼实际上未发生，query recovery 只会报查不到并日志错误，unknown command 不会被 submitted-command scanner 重新派发；需验证是否有 unknown command 超时告警或人工重置机制。

### 71. 宝付提现创建前的余额校验不是本地冻结，并发不同幂等键可能超额提交

- 链路：`CreateWithdrawal` 查询宝付余额 -> 本地创建 processing withdrawal_order + submitted command。
- 证据点：`logic/baofu_withdraw_service.go:154-179`；`db/query/baofu_withdrawal_order.sql:1-18`。
- 风险条件：同一 owner 用不同 idempotency key 并发发起多笔提现，两边都在 provider 余额尚未冻结前看到同一 available amount。
- 现有保护：同一 idempotency key 会 replay；provider 在真正 `CreateWithdraw` 时预计会做余额校验/冻结，失败会把 withdrawal_order 标 failed 或保持可恢复状态。
- 候选原因：本地没有按 owner 维度的“active withdrawal amount”扣减或串行锁，可能造成多个本地 processing 单，最后依赖 provider 拒绝和回调/查询收敛；需确认产品展示是否能承受短期 processing 总额大于可提现余额。

### 72. 宝付提现 callback 记录 external fact，但状态应用走专用 task payload，没有 generic application backlog

- 链路：提现 callback -> `recordBaofuWithdrawCallbackFact` -> `DistributeTaskProcessBaofuWithdrawalFactApplication` -> `UpdateBaofuWithdrawalOrderStatus`。
- 证据点：`api/baofu_callback.go:115-186`；`api/baofu_withdrawal_callback_fact.go:15-54`；`worker/task_baofu_withdrawal_fact_application.go:43-88`。
- 风险条件：external fact 创建成功后，专用 task 入队失败；或者 task 重试耗尽。
- 现有保护：handler 在入队失败时返回 FAIL，期望 provider 重投；processing withdrawal recovery 每 5 分钟可查询并再次派发专用 fact application。
- 候选原因：与通用 `external_payment_fact_applications` 不同，这条链路没有 application 表来统一表达 pending/failed/applied；若 provider 不重投且 recovery 查询异常，fact 已 received 但本地状态可能长期 processing。

### 73. 宝付提现状态 first terminal wins，returned/failed/succeeded 后到终态不会覆盖，需确认 provider 语义

- 链路：callback/recovery fact application 只在 withdrawal_order 仍 processing 时写终态。
- 证据点：`db/query/baofu_withdrawal_order.sql:78-88`；`worker/task_baofu_withdrawal_fact_application.go:59-87`。
- 风险条件：provider 先通知 failed/returned，随后又查询/通知 succeeded；或先 succeeded 后 returned。
- 现有保护：SQL 带 `AND status='processing'`；worker 发现当前已 terminal 直接返回，后到终态不会覆盖。
- 候选原因：first-terminal-wins 是否符合宝付提现真实状态语义需确认，特别是 `returned` 可能代表成功后退回，和 `failed`/`succeeded` 的财务含义不同；目前本地不会记录后到冲突状态。

### 74. 宝付提现 command outcome 与 withdrawal_order 终态可暂时或永久不一致

- 链路：dispatch worker 外呼后分别更新 withdrawal_order 与 external command outcome；callback/recovery 只更新 withdrawal_order。
- 证据点：`worker/task_baofu_withdrawal_command_dispatch.go:147-226`；`worker/task_baofu_withdrawal_command_dispatch.go:282-306` 的 `repairClaimedBaofuWithdrawalCommandOutcome`；`worker/task_baofu_withdrawal_fact_application.go:43-88` 不更新 command。
- 风险条件：provider callback/recovery 已把 withdrawal_order 终态化，但 command 仍 submitted/unknown；或 accepted outcome 记录失败。
- 现有保护：unknown command 再被 dispatch task 处理时可从 withdrawal_order/raw_snapshot 修复 accepted/rejected；terminal withdrawal 会让 submitted command dispatch 跳过。
- 候选原因：如果 command 已 submitted 但 withdrawal_order 被 callback 终态化，submitted command dispatch 看到 terminal order 直接返回，未见同时修复 command outcome；对账/审计以 command 表为准时可能显示仍 submitted。

### 70. 预约主状态 SQL 多数只按 id 更新，DB 层缺 expected-old-status 防线

- 链路：预约确认、完成、取消、过期、未到店、签到、支付成功、桌台/堂食联动等入口 -> `table_reservations.status`。
- 证据点：`db/query/table_reservation.sql:121-181` 的 `UpdateReservationStatus`、`UpdateReservationToPaid`、`UpdateReservationToConfirmed`、`UpdateReservationToCompleted`、`UpdateReservationToCancelled`、`UpdateReservationToExpired`、`UpdateReservationToNoShow` 都是 `WHERE id = $1`；`db/query/table_reservation.sql:276` 的 `UpdateReservationToCheckedIn` 同属该类。
- 风险条件：业务层先读状态再写状态，但读写不在同一事务，或存在其它旁路写入口；两个入口弱顺序交错时，后写可能覆盖前写的终态/业务状态。
- 现有保护：`logic/reservation.go:490`、`:533`、`:583`、`:718`、`:764`、`:828` 等主要人工入口会先 `GetTableReservationForUpdate` 并校验状态；部分事务 helper 内部也会重新锁 reservation。
- 候选原因：状态合法性主要靠调用方约束，不是数据库不变量；一旦有 worker、支付事实、桌台、堂食入口绕开同一状态矩阵，仍可能产生覆盖型时序问题。需把所有写入口收敛成迁移矩阵后逐条验证。

### 71. 预约支付成功晚到可能把已取消/过期/完成的 reservation 覆盖为 paid

- 链路：支付回调/查询 -> `ProcessPaymentSuccessTx` -> reservation payment 记录 -> `UpdateReservationStatus(... paid)`。
- 证据点：`db/sqlc/tx_payment_success.go:145-170` 先按 `payment_order_id` 幂等创建 reservation payment，再调用 `UpdateReservationStatus`；该 SQL 在 `db/query/table_reservation.sql:121-126` 只按 id 更新。
- 风险条件：用户/商户取消、支付超时任务、桌台释放、堂食关闭等先把 reservation 置为 cancelled/expired/completed/no_show，之后支付成功 fact 晚到。
- 现有保护：`reservation_payments.payment_order_id` 的幂等检查可避免重复插入同一支付记录；支付成功链路处于事务内，库存同步也在同一事务内。
- 候选原因：支付成功应用未校验 reservation 当前状态是否仍允许进入 paid；如果支付真实成功但业务预约已终止，需要明确是恢复预约、自动退款、还是人工对账，目前这一段看不到该分支语义。

### 72. 预约支付超时任务先读 pending 再无条件 cancel，读写窗口内可覆盖支付成功/人工操作

- 链路：`ProcessTaskReservationPaymentTimeout` -> `GetTableReservation` -> 判断 pending 和 deadline -> `UpdateReservationToCancelled` -> `ReleaseReservationInventoryTx`。
- 证据点：`worker/task_reservation_timeout.go:130-156` 先普通读取并校验 status；`worker/task_reservation_timeout.go:154-162` 再调用取消和释放库存；取消 SQL 在 `db/query/table_reservation.sql:160-167` 只按 id 更新。
- 风险条件：timeout worker 读到 pending 后，支付成功或商户操作在 worker 写 cancel 前提交；worker 后写把 paid/confirmed/checked_in 等覆盖成 cancelled。
- 现有保护：任务会在 deadline 前跳过，且读到非 pending 会跳过。
- 候选原因：pending 判断不在 update 条件内，属于典型 read-check-write 时序窗口；需要用并发测试确认支付成功事务与 timeout worker 的实际交错结果。

### 73. 预约 no_show/cancelled 与库存释放分两段提交，状态和库存可能不一致

- 链路 A：商户标记 no_show/cancel -> 状态事务 -> `ReleaseReservationInventoryTx`。
- 链路 B：支付超时 cancel -> `UpdateReservationToCancelled` -> `ReleaseReservationInventoryTx`。
- 证据点：`logic/reservation.go:718-753` 的 no_show 入口在 `MarkNoShowTx` 后单独释放库存；`worker/task_reservation_timeout.go:154-162` 取消成功后单独释放库存。
- 风险条件：状态已成功变成 no_show/cancelled，但释放库存事务失败、进程崩溃或重试缺失；后续库存仍按占用计算，或再次释放遇到已变更数据。
- 现有保护：库存释放封装在独立事务中，调用失败会向上返回错误；部分 worker 失败可依赖任务重试。
- 候选原因：业务状态和库存副作用不是同一原子提交；人工入口如果返回错误但状态已变，用户重试时可能因状态不再允许原动作而无法自动补库存。

### 74. 桌台强制释放 available 时会直接把 current_reservation_id 对应预约置 completed

- 链路：桌台更新 API -> 请求状态为 available -> 如果 table 有 `CurrentReservationID` -> `UpdateReservationToCompleted` -> 更新桌台。
- 证据点：`api/table.go:997-999` 在 table 当前有 reservation 时直接调用 `UpdateReservationToCompleted`；对应 SQL 在 `db/query/table_reservation.sql:152-158` 只按 id 更新。
- 风险条件：桌台 current_reservation_id 指向 paid/confirmed/checked_in 以外的状态，或 reservation 已 cancelled/no_show/expired 但 table 字段尚未清理；桌台释放后覆盖 reservation 终态。
- 现有保护：入口是商户/管理类桌台更新路径，可能受权限和前端流程约束；`UpdateReservationToCompleted` 若 id 不存在会失败。
- 候选原因：桌台状态修复逻辑把“释放桌台”隐含成“完成预约”，未在 DB 更新条件中限定当前 reservation 状态；这是跨模型状态联动的高风险旁路。

### 75. 堂食开台携带 reservation_id 时直接 checked_in，需确认前置校验覆盖所有入口

- 链路：`OpenDiningSession` -> `OpenDiningSessionTx` -> 如果 `ReservationID.Valid` -> `UpdateReservationToCheckedIn` -> 更新桌台 current_reservation_id。
- 证据点：`db/sqlc/tx_dining_session.go:145-171` 开台事务内调用 `UpdateReservationToCheckedIn`；`logic/dining_session.go:189-351` 是已发现的主要前置入口，尚需完整复核其状态校验。
- 风险条件：开台请求携带 cancelled/expired/no_show/completed reservation_id，或前置校验与事务写入之间状态被其它入口改变。
- 现有保护：开台事务会锁 table，并把 table current reservation 与 session 建立在同一事务；主逻辑层预计有权限、桌台、reservation 归属校验。
- 候选原因：状态条件不在 `UpdateReservationToCheckedIn` SQL 中，若前置校验不完整或锁边界不一致，开台可以成为覆盖 reservation 状态的旁路。

### 76. 堂食关台会把任意非 completed 的 current reservation 置 completed

- 链路：`CloseDiningSessionTx` -> 关闭 dining session -> 更新 table/current_reservation_id -> 如果存在 reservation -> raw SQL 更新 reservation completed。
- 证据点：`db/sqlc/tx_dining_session.go:248-270` 选择 session/table 的 reservation id 后执行 `UPDATE table_reservations SET status = 'completed' ... WHERE id = $1 AND status != 'completed'`。
- 风险条件：table/session 关联的 reservation 已被取消、过期、no_show、或支付链路仍 pending/paid；关台后统一写 completed，覆盖其它终态或中间态。
- 现有保护：整个关台路径在事务内处理 table/session；如果 reservation 已 completed 会跳过。
- 候选原因：`status != 'completed'` 是宽条件，不是显式 allowed-from 状态集合；需要确认堂食关台业务上是否确实允许从所有非 completed 状态收敛到 completed。

### 77. 预约调整 adjustment 状态迁移有旧状态 guard，但 paid fact 与关闭/过期仍需并发验证

- 链路：reservation addon payment success -> `applyPaidReservationAdjustmentWithQueries` -> adjustment applying/applied/closed/failed/expired -> reservation items/prepaid amount/inventory。
- 证据点：`db/query/reservation_adjustment.sql:103-154` 多个 adjustment 状态更新带 `status = 'creating_payment'` 或 `status IN (...)`；`db/sqlc/tx_reservation_adjustment.go:230-270` 应用 paid adjustment 时锁 adjustment 和 reservation，并检查 reservation status/cooking 状态。
- 风险条件：支付成功 fact 与用户关闭 adjustment、过期扫描、库存释放/转换并发；或 reservation 在 paid fact 应用前已进入 completed/cancelled/no_show。
- 现有保护：adjustment 表比 reservation 主状态机有更强 old-status guard；`applyPaidReservationAdjustmentWithQueries` 会校验 reservation 仍处于 paid/confirmed/checked_in 且未开做。
- 候选原因：这条链路的局部保护较好，但仍跨 payment fact、adjustment、reservation、inventory 多对象；需要用并发/乱序测试确认 paid fact 晚到时不会产生“支付成功但 adjustment 关闭、库存/金额未补偿”的悬挂态。

### 78. OCR job 创建、owner pending 标记、任务入队分三步，approved 媒体下缺少统一恢复入口

- 链路：`createOCRJob` -> `UpsertOCRJob` -> `markOCRPending` 写 owner 表 OCR JSON/asset binding -> `enqueueOCRJob`。
- 证据点：`api/ocr.go:1000-1147` 创建 job 后依次标记 owner pending、按媒体审核状态决定是否入队；`db/query/ocr_job.sql:1-25` job upsert 独立于 owner 表；`api/ocr.go:273-298` 分 owner/document 派发 asynq task。
- 风险条件：`UpsertOCRJob` 成功后 `markOCRPending` 失败，或 owner pending 成功后入队失败；尤其媒体已 approved 时，如果用户不重试或补偿失败，job/owner/队列可能不同步。
- 现有保护：同一 idempotency key 重试会返回同一 pending job 并再次尝试 `markOCRPending`/入队；入队失败时 `markOCRJobDispatchFailed` 会尝试把 pending job 和 owner OCR 字段都置 failed。
- 候选原因：这是跨 `ocr_jobs`、owner application、asynq 的非事务三段提交；需要确认是否有扫描 pending approved OCR jobs 的恢复任务。目前已读代码只看到媒体审核决议触发 pending jobs 处理，未见针对 approved 媒体创建后入队失败/未入队的后台恢复。

### 79. OCR 入队失败补偿本身是 best-effort，可能留下 owner pending 或 job failed 的半状态

- 链路：`enqueueOCRJob` 失败 -> `markOCRJobDispatchFailed` -> `FailPendingOCRJob` -> `markOCRFailed` 写 owner failed JSON。
- 证据点：`api/ocr.go:553-581` 中 `FailPendingOCRJob` 或 `markOCRFailed` 失败只记录日志并返回；`db/query/ocr_job.sql:119-130` 只允许 pending job 置 failed。
- 风险条件：入队失败后 `FailPendingOCRJob` 成功但 owner failed 写回失败，或 `FailPendingOCRJob` 因 job 已被 worker/其它入口 claim 不再 pending 而失败；UI 侧仍看到 owner OCR JSON 为 pending，而 job 可能已经 failed/processing。
- 现有保护：`markOCRFailed` 内部会按当前 owner binding 判断 stale，避免写错被替换的媒体；API 会向调用方返回错误，用户重试可能创建/复用任务。
- 候选原因：补偿不是事务且失败不再向外传播为可恢复动作；需要用日志/数据确认是否存在 `ocr_enqueue_failed` job 与 owner OCR status 不一致。

### 80. OCR job 成功/失败终态与 owner application 写回分离，可能 job 已终态但业务表未更新

- 链路：worker `ExecuteJob` -> `CompleteOCRJob`/`FailOCRJob` -> 重新 guard 当前 binding -> 更新 merchant/operator/rider/group application OCR 字段。
- 证据点：`ocr/service.go:92-154` 在 provider 成功后先 `CompleteOCRJob`；`worker/task_merchant_application_ocr.go:165-212`、`:290-323`、`:395-442` 成功后再更新 application；失败分支 `worker/task_merchant_application_ocr.go:131-157`、`:255-282`、`:360-387` 也是 job fail 后再写 owner failed；operator/group/rider worker 同样是 job 和 owner 分步。
- 风险条件：job 已 succeeded/failed 后，owner 写回失败、进程崩溃、DB 短暂错误、或 application 已提交/状态变更导致写回被拒绝；job 重试时由于 `CompleteOCRJob` 只允许 processing，重复 task 未必能重新写回。
- 现有保护：写回前会重新检查当前 binding 和 application status，避免晚到 OCR 覆盖替换后的媒体或非 draft application；owner 更新失败会让 asynq task 返回错误并重试，但 job 已终态后的重试路径需单独验证。
- 候选原因：OCR job 表是事实源，owner OCR JSON 是投影；两者没有 application backlog 表。需要测试“CompleteOCRJob 成功后 owner update 失败再重试”是否会重新投影，还是卡在 job succeeded + owner pending。

### 81. Rider OCR 写回只检查 asset binding，不检查 OCR job id，旧 job 与新 job 同媒体可能乱序覆盖

- 链路：rider id card/health cert OCR worker -> `ExecuteJob` -> `GetRiderApplication` -> `riderIDCardAssetStillBound`/`riderHealthCertAssetStillBound` -> `UpdateRiderApplication...`。
- 证据点：`worker/task_rider_application_ocr.go:175-301`、`:349-438` 的 stale 检查只比较当前 media asset id；`db/query/rider_application.sql:35-68` 更新 SQL 仅带 `id` 和 `status='draft'`，`id_card_ocr` 采用 JSON merge。
- 风险条件：同一 rider application、同一 media asset 因重试或手动 retry 创建多个 OCR job，旧 job 后完成；asset binding 仍匹配，但 owner OCR JSON 的 `ocr_job_id` 已指向新 job。
- 现有保护：application 必须仍是 draft；媒体替换会阻止旧 job 写回；id_card front/back 数据 merge 保留另一面信息。
- 候选原因：merchant/operator/group guard 会校验 owner OCR JSON 里的 `ocr_job_id`，rider 这条链路未见同等 job id guard；需验证统一 OCR retry 对 rider 同媒体多 job 的乱序终态是否会旧结果覆盖新结果。

### 82. Group application OCR pending/结果写回 SQL 不带 `status='draft'`，依赖事务外 guard

- 链路：group application OCR 创建/worker -> `markGroupApplicationOCRPending`/group OCR worker -> `UpdateGroupApplicationLicense`。
- 证据点：`api/ocr.go:450-540` 和 `worker/task_group_application_ocr.go:82-148`、`:173-263` 在调用前检查 app.Status == draft；但 `db/query/group.sql:28-56` 的 `UpdateGroupApplicationBasic`/`UpdateGroupApplicationLicense` 只按 `id` 更新，无 status 条件。
- 风险条件：guard 读取 draft 后，申请被提交/审核状态变化，再由 OCR pending/结果写回更新 `application_data`、`license_number` 或法人字段。
- 现有保护：worker 有 initial guard 和写回前 current guard；clear 类 SQL 带 `status='draft'`；主入口通常按申请状态限制编辑。
- 候选原因：状态限制没有落到 DB update 条件，读写窗口仍存在；相较 merchant/operator/rider 的 `WHERE id=$1 AND status='draft'`，group OCR 写回更依赖调用方顺序。

### 83. `ResetStaleMerchantOCRStatus` 只修 merchant application OCR JSON，不处理 ocr_jobs 或其它 owner 类型

- 链路：stale merchant OCR status cleanup -> owner OCR JSON pending/processing 改 failed。
- 证据点：`db/query/merchant_application.sql:269-280` 的 `ResetStaleMerchantOCRStatus` 只更新 `merchant_applications` 四个 OCR JSON 字段；当前已读 `rg` 未发现 operator/rider/group 的同类 stale reset SQL，也未见该 SQL 同步更新 `ocr_jobs`。
- 风险条件：OCR worker/队列丢失，owner OCR JSON 长期 pending；或 cleanup 把 owner JSON 改 failed，但对应 `ocr_jobs` 仍 pending/processing/succeeded。
- 现有保护：OCR job 有 dead-letter 查询 `ListOCRDeadLetterJobs`，worker 有 retry/failed 机制；merchant cleanup 至少能避免 merchant UI 无限 pending。
- 候选原因：这是 owner 投影层的局部修复，不是 job 状态机统一恢复；需要确认 scheduler 调用范围、operator/rider/group 是否会无限 pending，以及 merchant owner failed/job pending 的不一致是否影响重试和审核。

### 84. upload session 完成 SQL 不带 pending/未过期条件，可能覆盖 expired

- 链路：`CompleteUpload` -> 读 session -> 检查未 expired -> 创建/确认 media_asset -> `CompleteUploadSession`。
- 证据点：`media/registry.go:171-244` 在业务层检查 session status/expire_at；`db/query/media.sql:120-126` 的 `CompleteUploadSession` 只 `WHERE id = $1`，无 `status='pending'` 或 `expire_at >= now()`。
- 风险条件：complete 读到 pending 后，`ExpireStaleUploadSessions` 或 `ExpireUploadSession` 在创建 asset/确认 asset 前后把 session 改 expired；随后 complete 用无条件 SQL 把 expired 改 completed。
- 现有保护：业务层在写前检查 `session.Status == "expired" || time.Now().After(session.ExpireAt)`；`ExpireUploadSession`/`ExpireStaleUploadSessions` 只改 pending。
- 候选原因：这是 read-check-write 窗口，过期约束没有落到最终 update；需要并发测试确认 completed 覆盖 expired 是否会发生，以及产品是否允许“刚过期但对象已上传”的完成语义。

### 85. media_asset 创建/确认与 upload session 完成不是同事务，且完成错误被忽略

- 链路：`CompleteUpload` -> `CreateMediaAsset` -> `ConfirmMediaAssetUploaded` -> `CompleteUploadSession`。
- 证据点：`media/registry.go:219-244` 创建 asset、确认 upload_status 后调用 `CompleteUploadSession`；该调用结果被 `_, _ =` 忽略。
- 风险条件：asset 已创建并 confirmed，但 session 完成更新失败、DB 短暂错误或行状态已被其它任务改写；API 仍返回成功。
- 现有保护：同 object_key 冲突会用 `GetMediaAssetByObjectKey` 视作幂等；已 completed 且有 media_asset_id 的 session 会直接返回 asset。
- 候选原因：如果 session 没有成功记录 media_asset_id，后续按 upload_id 重试可能因 session expired 返回过期，而 asset 已存在；同时 pending session 可能被 stale expiry 扫成 expired，产生 asset/session 半状态。

### 86. 完成上传后触发媒体审核是事务外动作，失败可能留下 pending moderation 且无已读恢复入口

- 链路：`completeMediaUpload` -> `mediaRegistry.CompleteUpload` 返回 confirmed asset -> `triggerMediaModeration` -> 自动通过/发起微信异步审核/写 trace id。
- 证据点：`api/media.go:142-177` 上传完成后单独调用 `triggerMediaModeration`；`api/media_moderation.go:65-218` 内部可能外呼微信、再写 moderation_trace_id；失败时 complete API 返回 502，但 asset/session 已经提交。
- 风险条件：微信异步审核请求失败、缺 uploader openid、生成 private URL 失败、或写 trace id 失败；用户侧看到上传完成接口失败，但 media_asset 可能已 confirmed 且 moderation_status 仍 pending。
- 现有保护：重复 complete 对已完成 session 会再次返回 asset，API 层会再次尝试 `triggerMediaModeration`；部分非审核类别会自动 approved。
- 候选原因：审核触发没有本地 outbox/backlog 表；如果用户不重试或后台没有补扫 pending moderation assets，可能长期 pending，进而阻塞公有媒体展示或 OCR 延迟派发。

### 87. 微信 media moderation 已受理但 trace_id 持久化失败时，callback 无法关联 asset

- 链路：`triggerMediaModeration` -> `WechatClient.MediaCheckAsync` 成功返回 trace_id -> `SetMediaAssetModerationTraceID`。
- 证据点：`api/media_moderation.go:166-206` 外部请求成功后才写本地 trace id；`db/query/media.sql:59-63` 只按 asset id 写 trace id。
- 风险条件：微信已接收审核任务并稍后 callback，但本地持久化 trace id 失败；callback 用 trace_id 查不到 asset。
- 现有保护：API 返回错误，用户重试 complete 时因为 asset 仍 pending 且无 trace_id，会再次发起审核。
- 候选原因：外部任务创建与本地 trace 绑定不是原子；重复重试还可能产生多个外部 trace，而本地只保存后一次 trace。需验证旧 trace callback 到达时是否被安全忽略，以及是否有 pending asset 的补偿扫描。

### 88. media moderation 状态更新只按 id/trace_id 写，late/conflicting callback 可覆盖终态

- 链路：自动审核/开发自动通过/微信 callback -> `SetMediaAssetModerationStatus` 或 `SetMediaAssetModerationStatusByTraceID`。
- 证据点：`db/query/media.sql:53-68` 两个 moderation status update 都没有 expected-old-status；`api/media_moderation.go:314-343` callback 根据 trace_id 直接写映射后的状态。
- 风险条件：同一 trace 的重复 callback 先后结果不同；多次发起审核产生多个 trace，旧 trace callback 晚到；自动 approved 与微信 callback/retry 交错。
- 现有保护：`triggerMediaModeration` 起始会跳过非 pending 或已有 trace_id 的 asset；callback 需要匹配 trace_id 才能更新。
- 候选原因：状态机缺少 first-terminal-wins 或 conflict recording；一旦 trace 绑定存在，后到 callback 可把 approved/rejected/quarantined 互相覆盖，需要确认微信 callback 是否保证单调且唯一。

### 89. media moderation callback 先更新 asset 状态，再处理 pending OCR jobs，OCR 联动失败会留下半应用状态

- 链路：微信 callback -> `SetMediaAssetModerationStatusByTraceID` -> `processPendingOCRJobsForMediaModeration` -> approved 时 enqueue OCR，blocked 时 `FailPendingOCRJob` + owner failed。
- 证据点：`api/media_moderation.go:314-353` 先写 moderation_status，随后处理 OCR；`api/ocr.go:918-964` 根据 moderation_status 派发或失败 pending OCR jobs。
- 风险条件：asset 状态已更新为 approved/rejected，但 OCR enqueue/fail 或 owner failed 写回失败；handler 返回 500 期待微信重试，但 asset 状态已变。
- 现有保护：微信重试 callback 时会再次执行 OCR 联动；`processPendingOCRJobsForMediaModeration` 对 blocked 状态会检查 current owner binding，approved 状态会列 pending jobs 再 enqueue。
- 候选原因：asset moderation 和 OCR job/owner 投影不是同事务；如果 provider 不重试、重试耗尽或 pending jobs 已被其它路径改写，可能出现 asset approved 但 OCR job 仍 pending 未入队，或 asset rejected 但 owner 仍 pending。

### 90. onboarding review run 终态与 application review_summary 投影分离，summary 可能丢失或被旧 run 覆盖

- 链路：`CompleteOnboardingReviewRun`/`CancelOnboardingReviewRun` -> `UpdateMerchantApplicationReviewSummary`/`UpdateRiderApplicationReviewSummary`。
- 证据点：`logic/onboarding_review_service.go:205-249`、`:260-303` 先完成 run 再更新 application summary；`db/query/merchant_application.sql:282-286` 和 `db/query/rider_application.sql:211-215` 的 summary update 只按 `id` 更新，不校验 application status 或 summary run_id。
- 风险条件：run 已 completed/cancelled，但 summary 写回失败；或多个 review run 并发/重试，较旧 run 的 summary 后写覆盖较新 run 的 summary。
- 现有保护：run 表自身状态更新带 guard，`MarkOnboardingReviewRunProcessing` 只允许 queued，`CompleteOnboardingReviewRun`/`CancelOnboardingReviewRun` 只允许 queued/processing；merchant reset 时会在事务内取消 active review runs 并写 superseded summary。
- 候选原因：review run 是事实表，application review_summary 是投影；投影更新没有 run_id 单调性保护，也没有和 run 终态放进同一通用事务，需要用并发/失败注入验证 UI 是否可能显示过期审核结果。

### 91. 商户自动审核先提交审批事务，随后才完成 review run 与资质 ledger，失败后依赖重试/repair 收敛

- 链路：merchant onboarding worker/API sync fallback -> `ApproveMerchantApplicationTx` -> `CompleteMerchantReviewRun`/`RecordMerchantReview` -> credential ledger activation -> restore governance -> notification。
- 证据点：`logic/merchant_onboarding_review_service.go:110-182` 先执行 `ApproveMerchantApplicationTx`，之后才持久化 review run，再激活 credential ledgers 和 restore；`db/sqlc/tx_merchant_application.go:52-247` 审批事务本身不包含 onboarding review run/credential ledger。
- 风险条件：商户申请已 approved、商户/角色/subject profile 已提交，但后续 complete review run、写 summary、activate credentials 或 restore 失败。
- 现有保护：worker 重试时如果 application 已 approved 且带 existing run，会走 `repairApprovedMerchantApplication`；`worker/task_onboarding_review.go:117-123` 对 completed merchant run 不直接跳过，允许修复资质；merchant API enqueue 失败会 fallback 同步处理。
- 候选原因：这是有 repair 设计的最终一致性链路，但不是一次事务；如果没有 existingRunID、queued run 创建失败后审批成功、或重试/repair 没跑通，可能出现“商户已通过但 review/credential/governance 未收敛”的半状态。

### 92. 骑手拒审路径先退回 draft，再完成 review run；失败后可能留下 draft 申请但 queued/processing run 未终态

- 链路：rider onboarding review -> decision needs_resubmit -> `ReturnRiderApplicationToDraft` -> `CompleteRiderReviewRun`/`RecordRiderReview` -> update summary。
- 证据点：`logic/rider_onboarding_review_service.go:80-175` 拒审分支先 `ReturnRiderApplicationToDraft`，随后才通过 `onboardingReviewService` 完成 review run；`db/query/rider_application.sql:173-185` 退回只要求 `status='submitted'`。
- 风险条件：申请已退回 draft，但 run complete 或 summary 写回失败；asynq 重试时 application 已不是 submitted，当前 `ProcessSubmittedApplication` 会返回 status not submitted。
- 现有保护：退回 draft 允许用户重新编辑；review run 表仍保留 queued/processing 记录，人工/后台可见。
- 候选原因：拒审业务状态和 review run 终态不是同事务，且不像 approved+credential 路径那样明显有 repair 分支；需要测试拒审后 complete run 失败的重试行为是否会卡住 active run。

### 93. 骑手通过路径在 credential governance 开启时事务较完整，但关闭该服务时不会记录 review run

- 链路：rider approved -> 如果 credential governance 可用，`ApproveRiderApplicationWithReviewTx`；否则 `ApproveRiderApplicationTx` 后直接返回。
- 证据点：`logic/rider_onboarding_review_service.go:93-159` credential governance 为 nil 时执行 `ApproveRiderApplicationTx` 后返回；`db/sqlc/tx_rider_application.go:63-108` 的 `ApproveRiderApplicationWithReviewTx` 才把 review run、申请审批、summary、credential ledgers 放进同一事务。
- 风险条件：生产配置或测试/降级环境中 credential governance service 未初始化；骑手申请会 approved，但没有 onboarding review run/summary/credential ledger。
- 现有保护：正常 processor/server 初始化会创建 credential governance service；无服务时至少申请和 rider 记录由 `ApproveRiderApplicationTx` 原子创建。
- 候选原因：这是配置依赖型状态链分叉；需要确认线上永远不会以 nil credential governance 运行，否则审计链路和资质治理链路会缺事实记录。

### 94. 商户提交复用 active review run 只处理 merchant，rider 提交失败后缺少已读到的复用/补派发入口

- 链路 A：merchant submit retry -> `GetLatestActiveMerchantOnboardingReviewRun` -> queued 复用/processing 直接返回。
- 链路 B：rider submit -> create rider review run -> enqueue -> fallback sync。
- 证据点：`api/merchant_application.go:974-1037` 对 submitted retry 会复用 merchant active run；`api/rider_application_submit.go:92-124` 每次从 draft submit 创建 run，`SubmitRiderApplication` SQL 只允许 draft -> submitted。
- 风险条件：rider submit 已把 application 改 submitted 并创建 queued run，但 enqueue 失败且 fallback sync 也失败；用户再次提交时不再满足 draft 条件，未见同类 active run 复用逻辑。
- 现有保护：同一次请求中 enqueue 失败会立刻 fallback sync；worker task 如果已经入队会继续处理。
- 候选原因：rider 提交链路的恢复入口看起来比 merchant 少；需要验证失败后是否只能靠原 task/人工处理，还是其它 API 会重启 queued run。

### 95. credential restore 通知在状态恢复之后发送，通知失败会让任务重试并重复执行 repair/restore 路径

- 链路：onboarding review worker -> credential ledger activation/restore governance -> `distribute*CredentialRestoreNotification`。
- 证据点：`worker/task_onboarding_review.go:67-88` 在 `ProcessApplication` 成功后发送 restore notification，通知失败会返回错误；`logic/credential_governance_service.go:92-134` restore 已在此前写入治理状态。
- 风险条件：恢复商户/骑手经营状态已提交，通知任务入队失败；onboarding review task 重试再次执行 repair/restore/通知逻辑。
- 现有保护：credential activation 会检查 active ledgers，restore tx 预计按 suspend reason/状态过滤，通知 payload 有 review_run_id 和 source 可帮助幂等消费。
- 候选原因：通知属于最终副作用，和状态恢复没有 outbox 原子绑定；需要确认重复 review task 不会重复 ledger/restore，也不会重复通知或把失败通知当成审核失败。

### 96. delivery 取消 SQL 只按 id 更新，可能由 late cleanup/manual cancel 覆盖已推进状态

- 链路：stale delivery cleanup/manual cancel/order cancel -> `UpdateDeliveryToCancelled`。
- 证据点：`db/query/delivery.sql` 中正常推进如 assign/pickup/picked/delivering/delivered/completed 多数带 old status/rider 条件；但 `UpdateDeliveryToCancelled` 只按 delivery id 更新为 cancelled。
- 风险条件：cleanup 取样 pending delivery 后，骑手抢单或配送状态已推进；后到取消入口仍调用无旧状态 guard 的 update，把 assigned/picking/picked/delivering/delivered/completed 覆盖为 cancelled。
- 现有保护：多数业务入口在调用前会读取 delivery/order 状态并做前置校验；部分事务会锁订单或 delivery。
- 候选原因：关键终态/半终态保护没有沉到 SQL 条件；需继续读取 `CancelOrderTx`、所有 `UpdateDeliveryToCancelled` 调用点，确认是否所有调用都在同一锁域内二次断言。

### 97. delivery damage/delayed/ETA 字段更新不校验配送状态，可晚到写入 terminal delivery

- 链路：配送异常标记/延迟标记/预计送达时间更新 -> `UpdateDeliveryDamage`/`UpdateDeliveryDelayed`/`UpdateDeliveryEstimatedTime`。
- 证据点：`db/query/delivery.sql` 中这些更新只按 delivery id 写副作用字段或 ETA，未见 `status NOT IN terminal` 或 expected status 条件。
- 风险条件：delivery 已 cancelled/completed/delivered 后，延迟任务、运营动作或骑手端晚到请求继续写 `is_damaged`、`is_delayed`、`estimated_delivery_time`。
- 现有保护：调用方可能有业务前置校验；这些字段本身不一定改变主状态。
- 候选原因：副作用字段可能参与赔付、告警、展示或 SLA 统计；若 terminal 后可改，需确认是否符合审计语义，以及是否需要 terminal 后冻结或记录冲突。

### 98. 骑手确认送达主事务较完整，但 API 前置读取与事务外状态日志仍有半状态窗口

- 链路：`ConfirmDelivery` -> 读 delivery/order 校验 -> `CompleteDeliveryTx` -> 写 delivery/order/押金/流水/stats -> 事务外创建状态日志。
- 证据点：`db/sqlc/tx_delivery.go` 的完成事务内包含 `UpdateDeliveryToDelivered`、`UpdateOrderToRiderDelivered`、解冻押金、押金流水和骑手统计；`logic/delivery_status.go` 外层先读校验，状态日志在事务外创建且错误被忽略。
- 风险条件：两个确认送达请求或自动补偿/取消并发；主事务依赖 SQL guard 阻止重复推进，但状态日志失败或并发输掉方错误映射可能导致审计日志缺失/接口表现不一致。
- 现有保护：主状态 update 带旧状态/rider 条件，押金/统计在同一事务中，降低重复资金副作用风险。
- 候选原因：状态事实与审计日志不原子；需验证并发失败时 API 返回是否可接受，以及是否有日志补偿。

### 99. StartPickup/ConfirmPickup/StartDelivery 的状态日志在事务外且错误忽略，可能出现状态已变但日志缺失

- 链路：骑手开始取货/确认取货/开始配送 -> 状态更新事务或 SQL -> `CreateOrderStatusLog`/delivery 日志。
- 证据点：`logic/delivery_status.go` 已读到这些状态推进后的日志创建位于事务外，且错误只记录或忽略。
- 风险条件：状态推进成功后日志写入失败、进程崩溃、DB 短暂错误；后续客服/运营依赖日志判断状态变化，看到订单已推进但缺关键节点。
- 现有保护：主 delivery/order 状态本身有 guard；日志不是主状态源。
- 候选原因：状态机事实和审计投影分离；若日志参与自动化判断或赔付取证，需要补偿扫描或将日志纳入同事务。

### 100. stale delivery cleanup 的取样-取消-更新配送-退款入队是多段提交，可能与抢单/状态推进交错

- 链路：`cleanupStaleDeliveries` -> `ListPendingDeliveriesBefore` -> `GetOrder` -> `CancelOrderTx` -> `UpdateDeliveryToCancelled` -> 查支付单 -> 入队退款任务。
- 证据点：`scheduler/data_cleanup.go` 的 stale delivery 清理先取样 pending delivery，再逐条取消订单、更新 delivery、选择成功支付单并入队退款；`UpdateDeliveryToCancelled` 本身无旧状态 guard。
- 风险条件：取样后 delivery 被抢单或状态推进；只要 `CancelOrderTx` 在某个 order 状态下成功，后续 delivery 仍可能被覆盖 cancelled。退款入队失败只 log，订单已取消。
- 现有保护：`CancelOrderTx` 预计会校验订单状态并在事务内处理订单侧取消；cleanup 只查询 pending delivery 样本。
- 候选原因：多对象清理没有单一事务/锁域覆盖 delivery/order/payment/task；需继续核查 `CancelOrderTx` 的 expected status 行为和实际调度窗口。

### 101. stale delivery 自动退款分支使用 `p.Status == "success"`，已确认与 payment_order 成功状态枚举不一致，可能漏退款

- 链路：stale delivery cleanup 取消订单成功 -> `GetPaymentOrdersByOrder` -> 找成功支付单 -> enqueue `ProcessRefundTask`。
- 证据点：`scheduler/data_cleanup.go:1372-1399` 外层查支付单时筛选 `p.Status == "success"`；`db/query/payment_order.sql` 明确 `UpdatePaymentOrderToPaid` 写入 `status='paid'`，ledger/list/recovery 查询也以 `paid/refunded/closed/failed/pending` 为 payment_order 主状态枚举。
- 风险条件：真实成功支付单状态是 `paid` 而不是 `success`；自动取消配送后不会入队退款，订单已 cancelled 但资金未进入退款链路。
- 现有保护：注释提到后续 `ProcessOrderPaymentTimeout` 或人工补偿；其它退款/支付恢复任务可能覆盖部分场景。
- 候选原因：这是已读代码层面的强信号，不再只是疑似枚举混用；需用生产数据/日志确认 stale delivery 自动取消后是否依赖其它补偿扫描兜底，以及是否存在 cancelled+paid+无 refund_order 的订单。

### 102. stale delivery 取消后退款任务入队失败只记录日志，订单/配送已终止但退款副作用可能丢失

- 链路：stale delivery cleanup -> order cancel/delivery cancel committed -> enqueue refund task。
- 证据点：`scheduler/data_cleanup.go` 中退款任务入队位于订单/配送状态更新之后；入队失败只记录日志，注释依赖后续 timeout/人工补偿。
- 风险条件：Redis/asynq 短暂故障或唯一键冲突；订单已经取消，delivery 已 cancelled，但用户支付成功资金未被自动退款。
- 现有保护：可能存在支付 timeout worker、人工补偿或退款恢复；订单状态可作为后续扫描依据。
- 候选原因：缺本地 outbox/ledger 记录“取消后待退款”事实；需要查是否有独立 scheduler 从 cancelled+paid 状态补扫。

### 103. 配送超时告警 ledger 先写、通知后发，入队失败删除 ledger 失败会永久抑制后续告警

- 链路：operator dispatch alert scheduler -> 创建 `delivery_timeout_alerts` ledger -> enqueue/operator notification -> 入队失败时尝试删除 ledger。
- 证据点：`scheduler/operator_dispatch_alert.go` 已读到先创建告警记录，再入队通知；失败后有删除 ledger 的补偿。
- 风险条件：通知入队失败且删除 ledger 也失败；后续扫描认为该 delivery 已告警，不再重发，但实际通知未送达。
- 现有保护：删除 ledger 补偿覆盖一部分失败；日志可用于人工发现。
- 候选原因：ledger 与通知任务不是 outbox 原子绑定；需确认是否有“告警 ledger 已创建但通知未入队”的巡检。

### 104. delivery_pool 列表不按 expires_at 过滤，过期池项可能仍可见或被抢

- 链路：用户/骑手查看配送池 -> `ListDeliveryPool` -> 抢单 `GrabOrderTx`。
- 证据点：`db/query/delivery_pool.sql` 中 `CountDeliveryPoolNearby` 会过滤 `expires_at >= now()`，但 `ListDeliveryPool` 未过滤；注释写外卖订单始终可见直到被接单或取消。
- 风险条件：池项已过期但未被 cleanup 移除；骑手端列表仍展示并发起抢单，若 `GrabOrderTx` 不检查 pool `expires_at`，可能接到业务上已过期的订单。
- 现有保护：抢单事务会锁 pool/order/delivery，并通过 `AssignDelivery` 的 rider null/status guard 控制主状态；过期订单可能另有 cleanup 取消。
- 候选原因：展示计数和列表过滤语义不一致；需继续核查 `GrabOrderTx` 是否校验 `delivery_pool.expires_at`，以及“过期仍可见”是否产品设计。

### 105. 抢单事务保护主状态，但与 pool 过期删除/自动取消之间仍需确认锁顺序

- 链路：`GrabOrderTx` -> `GetDeliveryPoolByOrderIDForUpdate` -> `AssignDelivery` -> `RemoveFromDeliveryPool` -> 冻结押金 -> order courier_accepted。
- 证据点：`db/sqlc/tx_delivery.go`/`delivery_pool.sql` 已读到抢单事务内锁定 pool，并把配送分配、移除池项、押金和订单状态放进同一事务；但 `AddToDeliveryPool`、过期删除、stale delivery cleanup 是其它入口。
- 风险条件：过期清理或订单取消与骑手抢单同时发生，锁的是不同对象或顺序不同；可能一个事务移除/取消，另一个事务已冻结押金/推进订单。
- 现有保护：主事务有 `AssignDelivery` rider null guard，pool row `FOR UPDATE` 可阻止同一 pool 重复抢。
- 候选原因：需要把 pool/order/delivery 的所有清理和抢单锁顺序画出来，尤其是 cleanup 先 order cancel 后 delivery cancel 的路径。

### 106. delivery ETA 可由晚到估算覆盖已完成/取消配送的最终展示或统计

- 链路：距离/时长估算 -> `UpdateDeliveryEstimatedTime` -> 用户端/运营展示 ETA。
- 证据点：`logic/delivery_estimate.go` 与 `db/query/delivery.sql` 已读到 ETA 更新只按 delivery id，不校验 status。
- 风险条件：配送已 delivered/completed/cancelled 后，异步估算或重试请求晚到，把最终 ETA 改成新的预计值。
- 现有保护：ETA 不是主状态；调用方可能只在活跃配送中触发。
- 候选原因：如果 SLA、超时告警、用户承诺时间或赔付用该字段，terminal 后可变会影响事实追溯。

### 107. delivery 取消入口和订单状态事务的错误映射/幂等表现尚未验证，可能把并发输掉方暴露成 500

- 链路：多个取消/抢单/完成入口并发 -> SQL guard 无行更新或事务返回错误 -> API/worker 处理。
- 证据点：delivery 正常推进 SQL 多数带 expected status/rider guard；取消和 cleanup 跨 order/delivery 多对象。已读摘要显示 `tx_order_status.go` 附近仍需查看 `UpdateDeliveryToCancelled` 调用上下文。
- 风险条件：一个入口成功推进状态，另一个入口随后因为 old status 不匹配或 delivery 已被取消而报数据库错误；如果没有 reload-and-classify，用户/worker 可能看到 500 或反复重试。
- 现有保护：部分逻辑会先读当前状态并返回业务错误；worker 重试可能最终收敛。
- 候选原因：这属于弱顺序事件常见表现问题；需要逐 API/worker 核查并发输掉方是否被映射成 idempotent success、409，还是内部错误。

### 108. `CancelOrderTx` 初始 delivery 状态校验后才锁 delivery，锁后不复核状态即可无条件取消

- 链路：订单取消 -> `CancelOrderTx` -> `GetDeliveryByOrderID` -> `deliveryBlocksOrderCancellation` -> `UpdateOrderToCancelled` -> `GetDeliveryForUpdate` -> `UpdateDeliveryToCancelled`。
- 证据点：`db/sqlc/tx_order_status.go:160-214` 初始读取 delivery 后，如果状态不是 picked/delivering/delivered/completed 就允许继续；后续锁住 `lockedDelivery` 后直接调用 `UpdateDeliveryToCancelled`，没有再次判断 `lockedDelivery.Status` 是否已变成 blocked/terminal；`db/query/delivery.sql:85-91` 的 `UpdateDeliveryToCancelled` 只按 id 更新。
- 风险条件：初始读取时 delivery 是 assigned/picking，事务等待期间骑手确认取餐/开始配送等路径把 delivery 推进到 picked/delivering；取消事务获得锁后仍把 delivery 改为 cancelled。
- 现有保护：订单取消先用 `UpdateOrderToCancelled` 按 expected order status 更新，能阻止订单状态已变化的部分交错；delivery 主推进 SQL 也带 old status/rider guard。
- 候选原因：delivery 是否阻断取消的判断没有和最终 cancel update 使用同一个 locked 状态；需要并发测试覆盖 assigned->picked 与 order cancel 的交错。

### 109. `CancelOrderTx` 可能只按初始 delivery 状态决定是否解冻押金，锁后状态变化会造成资金副作用错配

- 链路：订单取消 -> delivery cancel -> `deliveryRequiresRiderDepositUnfreeze(lockedDelivery.Status)` -> 解冻骑手押金/写押金流水。
- 证据点：`db/sqlc/tx_order_status.go:214-236` 用锁后的 delivery 状态决定是否解冻 assigned/picking 押金；但 delivery cancel 本身无旧状态 guard，且 cancel 是否允许主要来自锁前状态判断。
- 风险条件：delivery 在取消事务等待期间从 picking 推进到 picked/delivering；取消事务随后无条件 cancelled，但因 locked 状态不再是 assigned/picking 而不解冻押金，或与完成/赔付链路产生押金冻结状态错配。
- 现有保护：如果 order 状态已经被配送推进 SQL 同步改变，`UpdateOrderToCancelled` 的 expected status 可能失败并回滚；需要实测锁等待顺序。
- 候选原因：主状态覆盖和押金副作用条件不完全同源；需要在并发测试里同时检查 delivery.status、orders.status、rider.frozen_deposit 和 deposit ledger。

### 110. 抢单事务内锁住 delivery_pool 后不复核 `expires_at`，外层过期检查存在 read-check-write 窗口

- 链路：`GrabDeliveryOrder` -> `GetDeliveryPoolByOrderID` -> 检查 `ExpiresAt.Before(now)` -> `GrabOrderTx` -> `GetDeliveryPoolByOrderIDForUpdate` -> assign/freeze/order accepted。
- 证据点：`logic/delivery_grab.go:85-92` 在事务外检查 pool 过期；`db/sqlc/tx_delivery.go:217-226` 在事务内只锁 pool，不检查 locked pool 的 `expires_at`。
- 风险条件：pool 在外层检查通过后、事务拿锁前自然过期或被过期清理任务视为过期；抢单仍可能成功并冻结押金/推进订单。
- 现有保护：外层检查能挡住明显过期；pool row `FOR UPDATE` 和 unique order_id 能挡住重复抢单；stale delivery cleanup 会取消长期 pending delivery。
- 候选原因：过期约束没有落到事务内最终判断；需要确认产品是否允许临界过期抢单，以及与 `RemoveExpiredFromDeliveryPool`/cleanup 的交错。

### 111. `ListDeliveryPoolNearby` 也不按 `expires_at` 过滤，附近列表、计数和抢单校验语义不一致

- 链路：骑手附近推荐 -> `ListDeliveryPoolNearby` -> 前端展示/推送 -> 抢单。
- 证据点：`db/query/delivery_pool.sql:56-73` 的 `ListDeliveryPoolNearby` 只按距离过滤；`CountDeliveryPoolNearby` 才有 `expires_at >= now()`；`logic/delivery_recommendation.go` 调用 `ListDeliveryPoolNearby` 作为推荐列表。
- 风险条件：过期 pool item 未及时删除时，附近列表仍展示，工作台计数可能不包含它；用户点击抢单后被外层过期检查拒绝，或遇到候选 110 的事务窗口。
- 现有保护：抢单外层会检查 pool 过期；列表只是展示/推荐，不直接改变状态。
- 候选原因：同一业务概念“可接单”在 list/count/grab 三处条件不同，容易造成弱顺序下的错觉和临界抢单问题。

### 112. 抢单事务锁住 order 后未复核“可抢订单状态”，依赖 `UpdateOrderToCourierAccepted` 的宽 allowed-from 集合

- 链路：`GrabDeliveryOrder` -> 事务外 `GetOrder`/`IsOrderStatusAllowedForDeliveryAction(order.Status,"grab")` -> `GrabOrderTx` -> `GetOrderForUpdate` -> `UpdateOrderToCourierAccepted`。
- 证据点：`logic/delivery_grab.go:125-135` 事务外校验订单可抢；`db/sqlc/tx_delivery.go:207-208` 锁住 order 后未重新判断 `order.Status`；`db/query/order.sql:278-285` 的 `UpdateOrderToCourierAccepted` 允许 `status IN ('preparing','ready','courier_accepted')`。
- 风险条件：外层看到 ready 后，事务等待期间订单被其它入口改为 preparing 或 courier_accepted；抢单事务仍可能继续分配 delivery、冻结押金并把 order 置 courier_accepted。若业务上 only ready 可抢，则 preparing 是非法回退/覆盖。
- 现有保护：外层 action guard 会先判断；`UpdateOrderToCourierAccepted` 本身不允许 pending/paid/cancelled/completed 等终态；food safety guard 会重新检查暂停。
- 候选原因：事务内最终状态约束比业务 action guard 更宽；需对照 `IsOrderStatusAllowedForDeliveryAction("grab")` 的 allowed-from 集合确认 preparing 是否故意可抢，或只是复用 SQL 导致的宽条件。

### 113. 订单配送状态同步 SQL 接受幂等旧状态，但事务外 oldStatus 日志可能与实际锁内状态不一致

- 链路：配送动作外层读取 order oldStatus -> delivery/order 状态同步事务 -> 事务外/事务内写日志。
- 证据点：`logic/delivery_status.go` 的 confirm pickup/start delivery 使用事务外读取的 `oldStatus` 写日志；`db/query/order.sql` 中 `UpdateOrderToPicked` 允许 `courier_accepted/picked`，`UpdateOrderToDelivering` 允许 `picked/delivering`，`UpdateOrderToRiderDelivered` 允许 `delivering/rider_delivered`。
- 风险条件：外层读取 oldStatus 后，另一个请求已推进订单；本请求通过幂等 allowed-from 更新成功或失败，日志 from_status 仍可能使用旧快照，产生状态日志与真实迁移不一致。
- 现有保护：delivery 状态 SQL 更严格，通常只有一个请求能推进；部分日志在事务内，部分在事务外。
- 候选原因：order SQL 的幂等 allowed-from 与外层日志快照不完全绑定；需逐动作验证重复请求时状态日志是否重复或 from/to 不准确。

### 114. 食安暂停先按活跃订单快照取样，再无状态条件写 pause 投影，可能标记已终态订单

- 链路：用户食安上报 -> `ReportFoodSafetyIncidentTx` 熔断商户 -> `ListMerchantActiveTakeoutOrdersForFoodSafety` -> `UpdateOrderFoodSafetyPauseState`。
- 证据点：`db/query/order.sql:189-197` 按当前 status 取活跃外卖订单；`db/sqlc/tx_food_safety.go:204-220` 遍历快照写 pause；`db/query/order.sql:362-370` 的 `UpdateOrderFoodSafetyPauseState` 只按 id 更新 `exception_state/status_hint/claim_channel`，不校验 status 仍在活跃集合。
- 风险条件：订单在 list 之后、pause update 之前被取消、用户确认、自动完成或替换；后到 pause 投影仍写入已终态订单。
- 现有保护：食安事务整体会在同一 DB 事务内执行；主订单状态没有被改成 paused，只是 exception/status hint 投影。
- 候选原因：投影层 read-check-write，没有把“仍活跃外卖”条件沉到最终 update；需用并发测试验证 pause 是否可落到 completed/cancelled/order replaced 场景。

### 115. 食安暂停清理只覆盖部分活跃状态，暂停后推进到 rider_delivered/completed 的订单可能残留 pause 投影

- 链路：食安 case resolve -> `ClearMerchantFoodSafetyPausedOrders` -> 清空 `exception_state/status_hint/claim_channel`。
- 证据点：`db/query/order.sql:372-382` 清理条件要求 `exception_state='food_safety_paused'` 且 status 在 `paid/preparing/ready/courier_accepted/picked/delivering`；不包含 `rider_delivered/user_delivered/completed/cancelled`。
- 风险条件：订单被标记 food_safety_paused 后，通过不走食安 gate 的用户确认/自动完成/取消诉求等入口推进到 rider_delivered/user_delivered/completed/cancelled；case resolved 时清理 SQL 不再匹配，终态订单长期带食安暂停提示/索赔通道。
- 现有保护：配送推进和商户接单/出餐事务调用 `ensureFoodSafetyTakeoutProgressAllowed`，能阻止部分 paused 订单继续履约。
- 候选原因：pause 投影和主订单终态的清理矩阵不完整；需确认用户确认收货、自动完成、客服处理是否应该同步清掉 food_safety_paused。

### 116. 食安推进 gate 依赖商户当前 suspension reason，不直接校验订单 pause 投影，可能出现 gate 与投影不一致

- 链路：外卖接单/出餐/配送推进/抢单 -> `ensureFoodSafetyTakeoutProgressAllowed` -> merchant profile takeout suspension reason。
- 证据点：`db/sqlc/tx_takeout_order.go:142-159` 通过 `GetMerchantProfile` 判断 `IsTakeoutSuspended && IsFoodSafetySuspendReason`；该 helper 不读取订单自身 `exception_state='food_safety_paused'`。
- 风险条件：订单已经写入 food_safety_paused，但商户 takeout suspension 被其它流程提前解除、覆盖原因、或 case resolve 清理未覆盖该订单；后续推进 gate 可能放行，而订单投影仍显示暂停。
- 现有保护：case resolve 会尝试清理 merchant 下活跃 paused orders；暂停和商户 suspension 在同一事务内写入。
- 候选原因：主 gate 事实源是 merchant profile，订单 pause 是投影；两者没有单调版本或 case_id 绑定，弱顺序下容易出现“商户已恢复但订单仍 paused/或订单 paused 但 gate 放行”。

### 117. 食安暂停 follow-up 通知和预约提醒在事务外入队，失败只记录日志，暂停事实可能无人收到

- 链路：`ReportFoodSafetyIncidentTx` 成功 -> `dispatchFoodSafetySuspensionFollowUps` -> 商户/用户/骑手通知 + 未来预约提醒任务。
- 证据点：`api/risk_management.go:988-1129` 在事务提交后发送 websocket/通知/预约 alert；`enqueueFoodSafetyNotification` 和 `scheduleFoodSafetyReservationAlert` 入队失败只 log，不回滚已提交的 pause/suspension。
- 风险条件：asynq/Redis 故障、单条通知入队失败、查 delivery/rider 失败；订单已经暂停，商户/骑手/用户未收到对应提示，骑手可能继续尝试履约直到 API gate 拦截。
- 现有保护：API/状态接口会暴露 status_hint/exception_state；部分推进入口会被 gate 阻断；日志可人工追查。
- 候选原因：暂停事实与通知 outbox 不原子；需要确认是否有未通知的食安暂停补偿扫描或运营看板。

### 118. 用户取消晚期订单写 `cancel_requested` 投影时忽略错误，可能吞掉售后入口创建失败

- 链路：用户取消 preparing/ready/courier_accepted/picked/delivering/rider_delivered 订单 -> `UpdateOrderExceptionState(cancel_requested)` -> `CreateOrderStatusLog` -> 返回“已记录取消诉求”。
- 证据点：`logic/cancel_order.go:41-65` 对 late status 分支调用 `UpdateOrderExceptionState` 和 `CreateOrderStatusLog` 都使用 `_, _ =` 忽略错误，然后返回 400 文案表示已记录诉求。
- 风险条件：DB 更新失败、日志失败、并发状态变化导致 update 异常；用户收到“已记录”但订单没有 `exception_state=cancel_requested` 或缺日志。
- 现有保护：主订单状态不变；用户可再次联系客服或重试；后续索赔/售后可能还有其它入口。
- 候选原因：这是时序/副作用失败被吞的强信号；需验证 API 层是否把该 400 当成用户已成功提交取消诉求，以及运营侧是否完全依赖 exception_state/日志。

### 119. 骑手确认送达事务未走食安暂停 gate，可能把已暂停 delivering 订单推进到 rider_delivered

- 链路：食安熔断把 active takeout order 标记 `food_safety_paused` -> 骑手端确认送达 -> `CompleteDeliveryTx` -> `UpdateDeliveryToDelivered` + `UpdateOrderToRiderDelivered`。
- 证据点：`db/sqlc/tx_delivery.go:360-390` 的 `CompleteDeliveryTx` 未调用 `ensureFoodSafetyTakeoutProgressAllowed`；同文件抢单/取货/配送推进前有 gate。`db/query/order.sql:305-312` 的 `UpdateOrderToRiderDelivered` 只校验 order status 为 delivering/rider_delivered，不看 `exception_state`。
- 风险条件：食安暂停发生时订单已 delivering，骑手未收到通知或继续点击送达；系统仍可把 delivery/order 推进到 delivered/rider_delivered，并解冻押金、更新骑手统计。
- 现有保护：前序抢单、开始取餐、确认取餐、开始配送会被食安 gate 阻断；食安 follow-up 会尝试通知骑手。
- 候选原因：食安暂停不是主状态，且最后一步送达没有 gate；会与候选 115 的清理范围叠加，产生 rider_delivered/completed 订单仍带 pause 投影的窗口。

### 120. 用户确认收货与自动完成 SQL 不看 exception_state/claim_channel，可能完成 paused/cancel_requested 订单

- 链路 A：用户确认收货 -> `ConfirmTakeoutOrder` -> `CompleteTakeoutOrderByUser`。
- 链路 B：自动完成 scheduler -> `hasClaimForOrder` -> `AutoCompleteTakeoutOrder` -> 分账任务。
- 证据点：`logic/confirm_order.go:47-63` 只按 order status 判断 rider_delivered/user_delivered；`scheduler/takeout_auto_complete.go:86-115` 只检查 claim 后完成；`db/query/order.sql:324-351` 的 `CompleteTakeoutOrderByUser`/`AutoCompleteTakeoutOrder` 只按 order type + status 更新 completed，不过滤 `exception_state='food_safety_paused'` 或 `cancel_requested`。
- 风险条件：订单进入 rider_delivered/user_delivered 后，被食安暂停投影、用户取消诉求投影或其它售后投影标记；后续确认收货/自动完成仍能完成订单，且不会清理 exception/claim_channel。
- 现有保护：索赔存在时自动完成会跳过；用户确认要求订单已送达；主状态已接近履约完成。
- 候选原因：售后/暂停投影和完成状态机没有统一 guard；需确认产品是否允许“异常中订单”完成，以及完成后是否应清理或保留投影。

### 121. 索赔创建事务内未复核订单仍 completed，依赖 API 外层快照与唯一约束

- 链路：`SubmitClaim` -> 外层 `GetOrder` 检查 status completed/user/金额/已有索赔 -> `CreateClaimWithBehaviorTx` -> `CreateClaim` + behavior decision/effects/actions。
- 证据点：`api/risk_management.go:350-390` 在 API 层检查订单归属、status completed、已有索赔；`db/sqlc/tx_claim_behavior.go:218-304` 事务内只 `GetOrder` 并创建 claim/decision，未重新校验 order status 仍 completed、user 仍匹配或订单未被替换；`db/migration/000126_add_unique_constraint_to_claims.up.sql` 仅有 `claims_order_id_unique` 防重复。
- 风险条件：API 外层检查后、事务创建 claim 前，订单被后台替换、状态修正或客服回滚；claim 仍可按旧快照创建并驱动赔付/追偿。
- 现有保护：订单 completed 通常是终态；一个订单只能有一个 claim；重复提交由唯一约束兜底。
- 候选原因：索赔事实创建没有把“订单仍可索赔”作为 DB 事务内不变量；需验证是否存在 completed 后被替换/撤销/异常处理的合法路径。

### 122. 索赔补偿状态与 action 入队分离，入队失败后依赖 recovery scheduler 扫描 created/failed/running action

- 链路：`ConfirmContinueClaim` -> `CreateClaimCompensationTx` 创建/复用 payout/recovery/block/notify actions 并可能更新 claim approved -> `enqueueClaimCompensationActions` 入队。
- 证据点：`api/risk_management.go:1924-1944` 补偿事务成功后再入队，入队失败只设置 `enqueueDeferred` 并记录日志；`api/risk_management.go:1535-1585` 分别入队 payout/behavior action。`worker/claim_refund_recovery_scheduler.go:75-106` 和 `worker/claim_behavior_action_recovery_scheduler.go:70-118` 会扫描 action 状态恢复。
- 风险条件：补偿状态/action 已持久化，但 task distributor 不可用或部分 action 入队失败；用户看到已确认继续，赔付/追偿/通知延后，且依赖 scheduler 配置和扫描周期。
- 现有保护：有 ClaimPayoutRecoveryScheduler 和 ClaimBehaviorActionRecoveryScheduler；入队失败显式记录 `dispatch_deferred` 审计信息。
- 候选原因：这是有恢复设计的最终一致性链路，但不是 outbox 原子交付；需确认 scheduler 在线、覆盖所有 action type/status，并监控 deferred/action stale。

### 123. claim payout action 使用无 expected-status 更新，running/success/failed 后到结果可能覆盖 action 状态

- 链路：claim payout worker/recovery -> `ExecuteClaimPayoutAction` -> 微信转账创建/查询 -> `updateClaimPayoutAction` -> `UpdateBehaviorActionExecution`。
- 证据点：`worker/task_claim_refund.go:304-323` 的 `updateClaimPayoutAction` 调用 `UpdateBehaviorActionExecution`；`db/query/behavior_trace.sql:234-240` 的 `UpdateBehaviorActionExecution` 只按 action id 更新 status/detail/executed_at，没有 expected current status。相比 `worker/task_claim_behavior_action.go:642` 行为 action 开始时有 `UpdateBehaviorActionExecutionIfCurrent` claim created -> running。
- 风险条件：同一 payout action 被 asynq 重试和 recovery scheduler 同时处理，或微信查询结果先返回 running 后返回 success/failed；后到 running/failed 可能覆盖 success，或后到 success 覆盖 terminal failed。
- 现有保护：detail 中有 transfer state/terminal failure，部分 worker 逻辑会根据微信状态分类；payout recovery 会跳过 detail 标记 terminal failure 的 failed action。
- 候选原因：action 主状态缺少 first-terminal-wins/expected-old-status；需用并发任务和 provider 状态乱序测试确认不会覆盖已终态 action 或重复触发 `MarkClaimPaid`/post-payout action。

### 124. claim paid 与 post-payout recovery/notification action 创建分离，后续 action 入队失败仅告警依赖恢复

- 链路：claim payout success -> `MarkClaimPaid` -> `FinalizeClaimCompensationAfterPayoutTx` 创建 recovery/restriction/notification action -> `enqueueClaimPostPayoutActions`。
- 证据点：`worker/task_claim_refund.go:465-488` 先 `MarkClaimPaid`，再 finalize 创建后续 action；`worker/task_claim_refund.go:489-493` 入队失败只 warn，日志称 recovery scheduler will retry。`FinalizeClaimCompensationAfterPayoutTx` 在 `db/sqlc/tx_claim_behavior.go:555-620` 基于 claim paid_at 创建后续 artifacts。
- 风险条件：claim 已 paid_at，但 finalize 失败、进程崩溃、或 post-payout action 入队失败；用户赔付已完成，但责任方追偿/通知/限制动作滞后或缺失。
- 现有保护：claim action recovery scheduler 会扫 created/failed recovery/block/notify action；若 finalize 未成功创建 action，需确认是否有从 paid claim 反推 missing action 的补偿入口。
- 候选原因：赔付事实与追偿/通知动作不是单事务；需重点验证“MarkClaimPaid 成功后 FinalizeClaimCompensationAfterPayoutTx 失败”的重试能否补齐 artifacts。

### 125. 追偿争议创建先落 dispute，再尝试把 recovery 标记 disputed，可能形成 dispute 存在但 recovery 未暂停

- 链路：`SubmitRecoveryDispute` -> 外层读取 recovery context/窗口期/重复争议 -> `CreateRecoveryDisputeWithRecoveryTx` -> `CreateRecoveryDispute` -> `GetClaimRecoveryByClaimIDAndTarget` -> `MarkClaimRecoveryDisputed`。
- 证据点：`logic/recovery_dispute.go` 在事务外做 eligibility/window/existing dispute 检查；`db/sqlc/tx_create_recovery_dispute.go` 事务内先创建 recovery dispute，再读取 claim recovery 并尝试标记 disputed；`db/query/recovery_dispute.sql` 中 `MarkClaimRecoveryDisputed` 只允许 `pending/overdue -> disputed`。
- 风险条件：外层检查后、事务内标记前，claim recovery 被支付、豁免、取消或不存在；争议记录已创建，但 recovery 状态未进入 disputed，后续审核/展示可能同时看到“争议已提交”和“追偿仍继续”。
- 现有保护：事务会把 create dispute 与后续 recovery 更新包在一起；重复 dispute 有唯一约束和 UniqueViolation 处理；`MarkClaimRecoveryDisputed` 至少避免从 terminal 状态回退。
- 候选原因：事务内没有把“recovery 必须仍 eligible 且成功进入 disputed”作为 dispute 创建成功的硬条件；需确认 tx 在 recovery 不存在或状态不匹配时是否应返回冲突而不是成功保留 dispute。

### 126. 追偿争议资格和窗口期基于事务外快照，事务内未重新锁定/复核 claim recovery eligibility

- 链路：提交争议 API/logic -> `loadRecoveryContext`/窗口期判断/权限判断 -> `CreateRecoveryDisputeWithRecoveryTx`。
- 证据点：`logic/recovery_dispute.go` 外层先读 recovery context、期限和是否已有争议；`db/sqlc/tx_create_recovery_dispute.go` 事务内没有重新计算窗口期、appellant 权限和 recovery 当前可争议状态，只按 claim/recovery target 尝试状态迁移。
- 风险条件：争议窗口刚过期、责任方/申诉方状态变化、claim recovery 被另一路操作终止；旧快照仍可创建 dispute。
- 现有保护：重复争议由 `idx_recovery_disputes_claim_id_appellant_unique` 兜底；recovery 状态迁移只接受 `pending/overdue`。
- 候选原因：典型 read-check-write 弱顺序；需要用并发测试验证窗口边界、recovery terminal 与 dispute submit 同时发生时的最终状态。

### 127. 追偿争议审核 approved 时只在 recovery 仍 disputed 才 waive/release，可能出现 dispute approved 但追偿未释放

- 链路：审核争议 -> `ReviewRecoveryDisputeTx` -> `ReviewRecoveryDispute(status=approved)` -> 读取 claim recovery -> `MarkClaimRecoveryWaived` + 创建 release action。
- 证据点：`db/sqlc/tx_recovery_dispute_review.go` approved 分支仅在 recovery status 为 `disputed` 时执行 waive/release；`db/query/recovery_dispute.sql` 的 review SQL 只 guard dispute 自身 `status='submitted'`，不把 recovery expected-status 绑定到 review 成功。
- 风险条件：争议提交后，追偿被其它链路 paid/waived/overdue/pending 变更，或候选 125 形成 dispute 存在但 recovery 未 disputed；审核仍可把 dispute 标记 approved，但不一定创建 release action 或 waived recovery event。
- 现有保护：如果 recovery 已终态，避免强行回退；review SQL 防止同一 dispute 被重复审核覆盖。
- 候选原因：争议状态和 recovery 状态是两个事实源，approved 没有强制要求 recovery 同步进入业务期望终态；需核对产品语义是“批准争议必然免追偿”还是“只在仍 disputed 时免追偿”。

### 128. 追偿争议 rejected 在事务内 resume，worker 又尝试 resume，幂等依赖 SQL 当前状态判断

- 链路：审核 rejected -> `ReviewRecoveryDisputeTx` 内 `ResumeClaimRecoveryAfterDispute` -> 后续 `ProcessTaskRecoveryDisputeResult` -> `ExecuteRecoveryDisputeResultEffects` rejected 分支再次 resume。
- 证据点：`db/sqlc/tx_recovery_dispute_review.go` rejected 分支会把 disputed recovery 恢复 pending/overdue；`worker/task_process_recovery_dispute_result.go` rejected 分支再次调用恢复逻辑，若 recovery 已 pending/overdue 则直接 nil。
- 风险条件：审核事务成功但 result task 延迟；期间 recovery 由 overdue scanner、人工处理或支付链路改变；worker 后到 resume 可能遇到非预期状态并跳过或报错。
- 现有保护：resume helper 对当前状态做判断，pending/overdue 可视为已恢复；submitted review 只允许一次。
- 候选原因：这是双写兜底式幂等设计，需验证所有后到状态都只会 no-op，不会把 paid/waived/terminal 追偿恢复成 active。

### 129. 追偿争议审核后的赔付/release/处罚/通知 effects 与审核事务分离，入队或 worker 失败会留下半状态

- 链路：审核成功 -> 事务内创建 release/payout action 或标记争议结果 -> API/任务分发 -> `ProcessTaskRecoveryDisputeResult` -> 执行 release、claim payout、claimant penalty、通知。
- 证据点：`worker/task_process_recovery_dispute_result.go` 先执行 `ExecuteRecoveryDisputeResultEffects`，再给 appellant/claimant 通知；通知失败只 log。approved 分支若有 `ReleaseActionID` 执行 release；赔付复用 `ExecuteClaimPayoutAction`；处罚会创建 user behavior blocklist/warning。
- 风险条件：review 已 approved/rejected，但 result task 未入队、任务重试耗尽、部分 effect 成功后进程崩溃；争议状态终态与 release/payout/penalty/notification 落地不一致。
- 现有保护：effect worker 具备部分幂等判断；行为 action/赔付 action 有各自 recovery scheduler 覆盖部分半状态；通知失败不会阻断主链路。
- 候选原因：审核终态与后续副作用不是 outbox 原子交付；需继续核查 API 审核入口的 result task 入队失败处理，以及是否有从 terminal dispute 反扫 missing effects 的补偿。

### 130. approved dispute 缺 release action id 时 worker 会按 recovery 存在性报错，可能造成终态争议反复重试

- 链路：approved dispute result task -> `ExecuteRecoveryDisputeResultEffects` -> 无 `ReleaseActionID` -> 查询 recovery -> recovery 存在则返回 missing release action id。
- 证据点：`worker/task_process_recovery_dispute_result.go` approved 分支在没有 release action id 时会检查 recovery；若 recovery 存在则认为缺失 release action 并报错。
- 风险条件：候选 127 的 approved 但未创建 release action、或事务创建 action 失败/未写入 result detail；worker 对已终态 dispute 反复重试但无法自愈。
- 现有保护：如果 recovery 已不存在或无需释放，worker 可以跳过；action 创建通常在审核事务内完成。
- 候选原因：worker 恢复依赖 review tx 产物完整存在；需确认是否有 rebuild release action 的补偿逻辑，或 approved-without-release 是否应被视为合法终态。

### 131. recovery dispute compensated 标记不校验 dispute 当前状态，赔付后到可能落在非 approved 争议上

- 链路：approved dispute compensation payout -> `ExecuteClaimPayoutAction`/result effect -> `MarkRecoveryDisputeCompensated`。
- 证据点：`db/query/recovery_dispute.sql` 中 `MarkRecoveryDisputeCompensated` 只设置 `compensated_at = COALESCE(compensated_at,$2)`，未在 SQL 层限制 recovery_dispute status 必须为 approved。
- 风险条件：赔付 action 与人工审核/自动审核/状态修复乱序；一个本应 rejected 或已被更正的争议，仍被后到 compensation effect 标记 compensated。
- 现有保护：赔付 action 通常只在 approved 分支创建；`COALESCE` 保持首次 compensated_at。
- 候选原因：compensated 是跨 action 的后到投影，没有 expected dispute status guard；需验证是否存在 action 创建后 dispute 状态还能被纠正或回滚的路径。

### 132. recovery dispute 重复提交由唯一约束兜底，但 rider/merchant 冲突处理语义不一致

- 链路：提交争议 -> 外层 `CheckRecoveryDisputeExists` -> `CreateRecoveryDispute` -> 唯一约束冲突处理。
- 证据点：`logic/recovery_dispute.go` 在事务前检查是否已有 dispute；migration `000213_create_recovery_disputes.up.sql` 有 `idx_recovery_disputes_claim_id_appellant_unique`；UniqueViolation 后 rider 分支会尝试返回 same rider existing dispute，merchant 分支倾向直接 conflict。
- 风险条件：同一 appellant 并发提交；一个请求已创建但尚未提交完 recovery 状态迁移，另一个请求命中唯一约束；不同 appellant role 的返回体和状态推进观察可能不一致。
- 现有保护：DB 唯一约束防止重复 dispute 事实；外层已有 exists check 降低普通重复提交。
- 候选原因：唯一约束兜底是正确方向，但并发冲突后的 API 语义与事务内半状态处理仍需核对，避免客户端重试误判或重复触发后续审核/通知。

### 133. 追偿争议自动裁决先按事务外判责快照推导结果，review tx 只 guard dispute submitted

- 链路：自动裁决 task -> `EvaluateAutomaticRecoveryDisputeResolution` -> 读取 dispute context 和 order 下 behavior decisions -> `DeriveAutomaticRecoveryDisputeResolution` -> `ReviewRecoveryDisputeWithCompensationTx`。
- 证据点：`logic/recovery_dispute_auto_resolution.go` 在事务外读取 `ListBehaviorDecisionsByOrder` 并选择 claim 对应 decision；随后 review tx 参数只携带推导出的 status/decision_id/review_notes。`db/query/recovery_dispute.sql` 的 review SQL guard 是 dispute `status='submitted'`。
- 风险条件：自动裁决评估后、review tx 前，新的 behavior decision 生效、旧 decision 失效或责任方变化；自动裁决仍按旧快照 approved/rejected。
- 现有保护：review tx 只允许 submitted -> terminal，避免重复审核覆盖；自动裁决会记录 decision_id 便于追溯。
- 候选原因：裁决依据和写终态不在同一事务/同一版本条件下；需验证 behavior decision 是否存在单调生效版本，以及自动裁决是否应在 tx 内复核 latest decision。

### 134. 自动裁决 task 对已终态 dispute 会重建 result payload 并再次执行 effects，幂等依赖后续 action/status 判断

- 链路：`ProcessTaskAutomaticRecoveryDisputeResolution` -> 若 dispute 已非 submitted -> `GetRecoveryDisputeForPostProcess` -> 重新评估 resolution（失败只 warn）-> `buildProcessRecoveryDisputeResultPayload` -> `processRecoveryDisputeResult`。
- 证据点：`worker/task_automatic_recovery_dispute_resolution.go` 对 terminal dispute 不直接 no-op，而是重建 payload 再执行 result effects；重新评估 resolution 失败不会阻断，会继续按既有 dispute/postProcess 构建 payload。
- 风险条件：自动裁决 task 重试、延迟执行或与人工审核竞态；同一 approved/rejected dispute 的 release/payout/penalty/notification effects 可能被多次触发，实际幂等性落在各 action worker 和状态 SQL。
- 现有保护：dispute review 本身不会被重复写；result payload 会尝试从已存在 behavior actions 中找 release/payout action id；部分 action 执行有状态判断。
- 候选原因：终态 dispute 的 result effect 是可重复执行路径，不是单一 outbox 消费；需逐个 effect 验证重复调用是否 first-terminal-wins，尤其通知、处罚、release、claim payout。

### 135. 自动裁决 result payload 通过 decision/action 反查 release/payout，action detail 不匹配会造成 approved 后缺 effect id

- 链路：终态 dispute 自动裁决重试 -> `claimRecoveryForRecoveryDisputeResult` -> `recoveryDisputeResultDecisionIDs` -> `ListBehaviorActionsByDecision` -> 解析 action detail 匹配 claim/recovery/dispute。
- 证据点：`worker/task_automatic_recovery_dispute_resolution.go` 的 `buildProcessRecoveryDisputeResultPayload` 只从 recovery decision id 和重新评估 resolution decision id 相关 actions 中寻找 release/payout；release 还要求 detail 中 claim_id/recovery_id 匹配，payout 要求 detail.recovery_dispute_id 匹配。
- 风险条件：review tx 创建了 action 但 decision_id 与当前 recovery/resolution 不一致、action detail 缺 recovery_id/旧 recovery_id、重新评估失败或 latest decision 已变化；payload 会带 0 action id，后续 approved 分支可能报 missing release action 或跳过赔付。
- 现有保护：submitted 首次自动 review 后会直接使用 tx result 中的 action id；重试路径才需要反查。
- 候选原因：恢复路径依赖从当前状态重建历史 action 关联，缺少明确的 recovery_dispute_id -> action outbox 索引；需确认重试/恢复时能稳定找到首次 review 创建的 action。

### 136. 追偿解除暂停按“无 blocking recovery”整体判断，可能清理非追偿原因造成的暂停状态

- 链路：recovery release action 或追偿终止 -> `ReleaseClaimRecoverySuspensionIfClear` -> `HasBlockingClaimRecoveryForMerchant/Rider` -> `UnsuspendMerchantTakeout`/`UnsuspendRider`。
- 证据点：`db/sqlc/claim_recovery_suspension.go` 只判断同 merchant/rider 是否还有 blocking claim recovery；若没有就直接调用 unsuspend。当前已读文件中未见对暂停 reason/case_id 的二次校验。
- 风险条件：商户/骑手同时因为食安、风控、手工运营或其它原因被暂停；某个 claim recovery 被 waive/paid 后 release action 后到，可能把非追偿原因的暂停一并解除。
- 现有保护：是否误清理取决于 `UnsuspendMerchantTakeout`/`UnsuspendRider` SQL 是否自带 reason guard；需要继续读取 query。
- 候选原因：解除动作以“无追偿阻断”作为充分条件，可能没有绑定“当前暂停正由该追偿链路设置”；需核查 SQL 和行为 action 执行细节。

### 137. 自动裁决 best-effort：dispute 创建成功后自动审核/结果处理失败，API 仍返回持久化现状并依赖 retry 入队

- 链路：商户/骑手提交争议 -> `autoResolveRecoveryDisputeBestEffort` -> `autoResolveRecoveryDispute` 失败 -> 尝试 `DistributeTaskAutomaticRecoveryDisputeResolution` -> reload persisted dispute -> 返回客户端。
- 证据点：`api/recovery_dispute.go:683-715` 与 `:1184-1222` 创建后若 status=submitted 就 best-effort 自动裁决；`api/recovery_dispute.go:1767-1803` 自动裁决失败只记录错误，若 task distributor 可用则入队 retry，若不可用则 warn，最后返回 persisted dispute。
- 风险条件：review tx 已成功但 result effects 失败、audit log/dispatch 失败、task distributor 不可用、retry 入队也失败；客户端可能看到 submitted/approved/rejected 之一，但后续 release/payout/notification 是否执行取决于另一路 retry。
- 现有保护：自动裁决失败后会尽量入队 `TaskAutomaticRecoveryDisputeResolution`；若 review 已提交，retry task 对 terminal dispute 会尝试重建 result payload。
- 候选原因：dispute submit API 把自动裁决和 effects 作为 best-effort 旁路，不是强一致完成；需确认 submitted 长期滞留、approved effects 缺失是否有监控/补偿扫描。

### 138. recovery dispute result 入队失败会同步 inline 执行，但 inline 失败后外层仍可能走 best-effort 返回

- 链路：`autoResolveRecoveryDispute` -> `dispatchRecoveryDisputeResult` -> asynq enqueue 失败则 `processRecoveryDisputeResultInline` -> effects + notification。
- 证据点：`api/recovery_dispute.go:1805-1815` 当 taskDistributor 为空或入队失败会 inline 执行；`api/recovery_dispute.go:1817-1861` inline effects 失败直接返回错误，通知发送不检查返回值或只 log。上层 `autoResolveRecoveryDisputeBestEffort` 捕获错误后仍返回 persisted dispute。
- 风险条件：review tx 已把 dispute 写成 approved/rejected；inline effects 失败导致 API 不回滚 review，只通过 automatic retry 补偿。若 retry 入队也失败，终态 dispute 与 effects 长期不一致。
- 现有保护：有 inline fallback，避免单纯 Redis 故障导致完全不执行；terminal dispute retry 会再执行 result effects。
- 候选原因：review 事实与 result effects 没有 outbox 原子性；inline fallback 降低失败概率但不能保证最终交付。

### 139. 追偿解除暂停已确认使用无 reason guard SQL，可能覆盖其它来源暂停

- 链路：release action -> `ReleaseClaimRecoverySuspensionIfClear` -> `UnsuspendMerchantTakeout`/`UnsuspendRider`。
- 证据点：`db/sqlc/claim_recovery_suspension.go:19-53` 调用无 owner 参数的 unsuspend；`db/query/trust_score.sql` 中 `UnsuspendMerchantTakeout`/`UnsuspendRider` 只按 merchant_id/rider_id 清空暂停字段；同文件另有 `ReleaseMerchantTakeoutSuspensionIfOwned`/`ReleaseRiderSuspensionIfOwned` 这类 reason guard 版本，但当前 helper 未使用。
- 风险条件：追偿逾期 block 后，又被食安/运营/风控设置了新的暂停 reason；旧 release action 后到且判断无 blocking recovery 后，会清空新暂停原因。
- 现有保护：block action 创建的 suspend_reason 形如 `claim recovery overdue: claim_id=...`；库里已有 IfOwned SQL 可作为更精确模式，但当前链路未接上。
- 候选原因：这是跨原因暂停状态的典型后到覆盖风险；需用并发/顺序测试验证 old release after new suspend 是否发生，以及是否影响商户外卖/骑手接单 gate。

### 140. claim recovery overdue 扫描只列 pending，disputed 且过期的 recovery 不生成新的 block action

- 链路：逾期扫描 -> `ListDueClaimRecoveries` -> `MarkClaimRecoveryOverdueWithActionTx` -> block action。
- 证据点：`db/query/claim_recovery.sql:167-177` 只查询 `status='pending' AND due_at <= $1`；`HasBlockingClaimRecoveryForMerchant/Rider` 又把 `status='disputed' AND due_at <= NOW()` 视为 blocking。
- 风险条件：recovery 在 due 前进入 disputed，争议长期未处理并过期；系统认为它 blocking，但逾期 scanner 不会把它转 overdue 或创建 block action。若 dispute rejected 后 resume，会按 due_at 恢复 overdue，但在争议期间可能没有实际 suspend action。
- 现有保护：disputed 过期在 HasBlocking 中会阻止 release 清暂停；rejected resume 会把过期 disputed 恢复为 overdue。
- 候选原因：blocking 判断与 overdue action 生成条件不一致；需确认 disputed 期间是否应暂停追偿执行，还是过期即应限制服务。

### 141. claim behavior action 执行开始有 expected-status，成功/失败/reset 无 expected-status，后到结果可覆盖终态

- 链路：behavior action worker/recovery -> `claimBehaviorActionMarkRunning` -> 执行业务副作用 -> `markClaimBehaviorActionSuccess`/`markClaimBehaviorActionFailure`/`resetClaimBehaviorActionToCreated`。
- 证据点：`worker/task_claim_behavior_action.go:642-657` 开始执行使用 `UpdateBehaviorActionExecutionIfCurrent`，要求当前 status 等于旧状态；但 `markClaimBehaviorActionSuccess`、`markClaimBehaviorActionFailure`、`resetClaimBehaviorActionToCreated` 在 `:660-696` 都调用无 current guard 的 `UpdateBehaviorActionExecution`，其 SQL `db/query/behavior_trace.sql:234-240` 只按 id 更新。
- 风险条件：同一 action 被 asynq retry、manual inline、recovery scheduler 同时处理；一个执行分支成功后，另一个较早读到 running 的分支失败或 reset 到 created，后到写覆盖 success。
- 现有保护：worker 入口遇到 action.Status == success 会 no-op；开始 claim 对非 running 状态有 CAS；scheduler 只扫 created/failed。
- 候选原因：CAS 只保护“开始执行”，不保护“终态写入”；需用并发任务验证 success/failed/created 是否可能后到覆盖，特别是 release/block/notify 这类副作用已发生的 action。

### 142. claim behavior action recovery scheduler 不扫描 running，worker 崩溃可能导致 action 长期 running

- 链路：worker 把 action 从 created/failed 标记 running -> 进程崩溃/上下文取消/外部调用卡死 -> scheduler recovery。
- 证据点：`worker/claim_behavior_action_recovery_scheduler.go:80-119` 只对 `statuses := []string{"created", "failed"}` 扫描重投；不扫描 running，也未见 running stale timeout 条件。
- 风险条件：action 已 running 但业务副作用未执行或未写 success/failed；后续 recovery 不会重新入队，导致 payout 后追偿通知/release/block 等 action 悬挂。
- 现有保护：asynq 自身对任务失败会重试；如果进程崩溃导致任务重新投递，入口读取 action.Status == running 会直接继续执行而不是 claim。
- 候选原因：running 被视为可继续执行但没有后台 stale recovery；需确认 asynq 丢失/重试耗尽/进程终止时是否可能留下 running，并是否有运营告警。

### 143. 追偿 block action 使用无 owner guard 的 SuspendMerchantTakeout/SuspendRider，可能覆盖后到暂停 reason

- 链路：claim recovery overdue -> block action -> `executeClaimRecoveryBlockAction` -> `SuspendMerchantTakeout`/`SuspendRider`。
- 证据点：`worker/task_claim_behavior_action.go:278-291` 执行 block 时直接调用 suspend；`db/query/trust_score.sql:75-82` 和 `:183-190` 的 suspend SQL 只按 id 写 `is_*_suspended=true` 和 reason/until，不要求当前 reason 为空或等于本 action。仓库里另有 `ClaimMerchantTakeoutSuspensionIfAvailable`/`ClaimRiderSuspensionIfAvailable` 带 reason 可用性条件。
- 风险条件：商户/骑手已因食安、证照、运营等其它原因暂停；追偿 block action 后到会把暂停 reason 和 until 改成 claim recovery overdue，之后 release 又可能按候选 139 清掉该暂停。
- 现有保护：credential governance 等链路使用 claim-if-available 防覆盖；本 action detail 记录了 claim/recovery id 和 suspend_reason。
- 候选原因：block 与 release 两端都未绑定 owner/reason，形成“旧追偿 action 覆盖新暂停，再由旧 release 清掉”的完整时序风险链。

### 144. claim recovery open action 只校验 recovery 字段并补事件，不校验 recovery 当前状态仍可打开

- 链路：post-payout recovery action -> `executeClaimRecoveryOpenAction` -> `GetClaimRecoveryByID` -> `validateClaimRecoveryOpenAction` -> `ensureClaimRecoveryOpenEvents` -> mark success。
- 证据点：`worker/task_claim_behavior_action.go:307-365` 校验 id/target/amount/basis/due_at，但 `validateClaimRecoveryOpenAction` 未检查 `recovery.Status`；`ensureClaimRecoveryOpenEvents` 只按现有事件列表补 created/payable 事件。
- 风险条件：recovery action 延迟执行时，claim recovery 已被 paid/waived/disputed/overdue；open action 仍可能补 created/payable 事件并标记 success，造成终态 recovery 后出现“打开/可追偿”事件。
- 现有保护：action detail 与 recovery 的金额/target/due_at 必须匹配；事件补写会先查已有事件，避免重复 created/payable。
- 候选原因：action 表达的是历史打开动作，但执行时没有 expected recovery status/version；需确认事件语义是否允许在 terminal recovery 上补历史事件。

### 145. claim recovery release action 不校验 recovery 已 paid/waived/closed 类终态，任何状态都可触发解除暂停和 closed event

- 链路：dispute approved 或 recovery settle 后 release action -> `executeClaimRecoveryReleaseAction` -> validate 基础字段 -> `ReleaseClaimRecoverySuspensionIfClear` -> `WriteClaimRecoveryClosedEventIfAbsent` -> mark success。
- 证据点：`worker/task_claim_behavior_action.go:421-510` 的 release 校验只覆盖 recovery_id/claim/order/target，不检查 recovery status 是否 `paid/waived` 或 dispute approved；closed event payload 直接写当前 status。
- 风险条件：release action 延迟或误重建，执行时 recovery 仍 pending/overdue/disputed；如果当前没有其它 blocking recovery，仍会解除暂停并写 closed event。
- 现有保护：release action 通常由 review/settle tx 在业务状态变化时创建；`WriteClaimRecoveryClosedEventIfAbsent` 可避免重复 closed event。
- 候选原因：release effect 没有把“追偿已实际关闭”作为执行前置；需验证 action 创建路径是否绝不可能在 pending/overdue 上创建 release，或执行时应加状态 guard。

### 146. claim recovery events 无显式唯一约束，事件幂等依赖读后判断和 helper 约定

- 链路：open/release/overdue/review 等流程 -> `WriteClaimRecoveryEvent`/`WriteClaimRecoveryClosedEventIfAbsent` -> `claim_recovery_events`。
- 证据点：migration `000172_add_behavior_decision_v2.up.sql` 创建 `claim_recovery_events` 仅有 recovery_id/decision_id 普通索引；`000211_expand_claim_recovery_event_types.up.sql` 只扩展事件枚举，未加 `(recovery_id,event_type)` 唯一约束。`worker/task_claim_behavior_action.go:385-418` 通过先 list events 再判断是否补 created/payable；`db/sqlc/claim_recovery_event.go:36-47` 的 `WriteClaimRecoveryClosedEventIfAbsent` 也是先 list 后 insert。
- 风险条件：两个 worker 并发执行同一 open/release action，或 action retry 与 recovery scheduler 重叠；都在读事件时未看到对方即将写入，可能重复 created/payable/closed 类事件。
- 现有保护：helper 会在单线程重复执行时避免 closed 重复；普通 open events 会读取已有事件后再补写。
- 候选原因：事件幂等没有 DB 唯一约束兜底，属于可疑 read-check-write；需用并发测试确认是否可重复写审计事件。

### 147. recovery dispute result 重放可能重复处罚 claimant，处罚幂等只按“当前是否有 active blocklist”判断

- 链路：approved dispute result effects -> `penalizeRecoveryDisputeClaimant` -> `GetActiveBehaviorBlocklist` -> `CreateBehaviorBlocklist` -> `recordRecoveryDisputeClaimWarning`。
- 证据点：`worker/task_process_recovery_dispute_result.go:123-221` 每次 approved result effect 都会尝试处罚索赔用户；若当前已有 active blocklist 则 no-op，否则创建新的 `malicious-claim` blocklist 并创建/递增 warning。
- 风险条件：同一 approved dispute result task 重试/重放；第一次处罚 blocklist 已过期、被运营解除或因并发未查询到，后到 result effect 会再次创建 blocklist 并递增 warning。
- 现有保护：active blocklist 唯一索引可避免同时存在多个 active；如果 active 仍存在，重复 result effect 直接跳过。
- 候选原因：处罚幂等 key 是用户当前 active blocklist，而不是 recovery_dispute_id 或 claim_id；需确认同一争议终态重复处理是否允许再次处罚/递增 warning。

### 148. recovery dispute result 通知失败只记录日志，任务仍成功，通知没有独立恢复扫描

- 链路：result worker -> `ExecuteRecoveryDisputeResultEffects` 成功 -> `sendAppellantNotification`/`sendClaimantNotification` -> 失败 log -> 返回 nil。
- 证据点：`worker/task_process_recovery_dispute_result.go:82-99` 通知失败只 `log.Error`，不返回错误；`sendAppellantNotification`/`sendClaimantNotification` 在 `:273-365` 通过 distributor 入队通知，失败会返回给 caller 但 caller 吞掉。
- 风险条件：result effects 已执行，通知入队失败；任务被视为成功不会重试，用户/商户/骑手可能看不到审核结果通知。
- 现有保护：争议状态和详情接口可查询；API inline fallback 使用同步通知但也不把发送失败作为主链路失败。
- 候选原因：通知是非关键副作用且无 outbox/恢复扫描；需确认产品是否接受“状态已变更但通知缺失”，以及是否有通知失败监控。

### 149. recovery dispute 人工审核 tx 存在但当前 API 路由看似已移除，系统主要依赖提交后自动裁决

- 链路：`ReviewRecoveryDisputeWithCompensationTx` 能写 approved/rejected 和创建 actions；生产 API 文件中提交后调用 `autoResolveRecoveryDisputeBestEffort`。
- 证据点：全仓 `rg` 仅发现 `ReviewRecoveryDisputeWithCompensationTx` 在自动裁决逻辑、测试和 tx 内使用；`api/recovery_dispute_test.go` 有 `ReviewRouteRemoved` 用例；`api/recovery_dispute.go` 未见独立 operator review handler 调用该 tx。
- 风险条件：自动裁决失败后 dispute 长期 submitted；如果没有人工审核入口或补偿看板，submitted 只能靠 retry task 恢复，retry 入队失败时可能悬挂。
- 现有保护：submit API best-effort 失败会尝试入队 automatic retry；查询接口可看到 submitted 状态。
- 候选原因：这是可用性/恢复路径风险，不一定是 bug；需确认是否另有后台管理端、运维脚本或定时扫描处理 submitted recovery disputes。

### 150. 围栏 dwell 事件先落库再推进状态，状态推进失败后同类 dwell 唯一约束会阻断后续自动推进重试

- 链路：骑手位置上报 -> `processDeliveryLocationEvents` -> `createDeliveryLocationEvent(dwell_*)` -> `maybeAutoAdvancePickup`/`maybeAutoConfirmPickup`/`maybeAutoConfirmDelivery`。
- 证据点：`api/rider_location_events.go:59-76` 先创建 dwell event，只有 `created == true` 才触发自动推进；`db/query/delivery_location_event.sql` 对 `(delivery_id,event_type)` 使用 `ON CONFLICT DO NOTHING`；自动推进失败只 log，不删除 dwell event。
- 风险条件：dwell event 创建成功后，状态 tx 因订单状态变化、食安暂停、DB 瞬断、押金解冻失败、位置校验失败等返回错误；后续位置上报再次进入同一围栏时，dwell event 已存在，`created=false`，不会再次触发自动推进。
- 现有保护：手动配送状态 API 仍可推进；tx 内有状态 guard，避免错误推进主状态。
- 候选原因：事件 ledger 被用作触发去重，但创建位置在副作用之前；缺少“已触发且成功推进”状态或失败后可重试标记。

### 151. pickup dwell 在 assigned 状态下只会自动推进到 picking，无法在同一次/后续同类 dwell 中自动确认取餐

- 链路：delivery status assigned -> 到店 dwell_pickup -> `maybeAutoAdvancePickup` -> `maybeAutoConfirmPickup`。
- 证据点：`api/rider_location_events.go:69-72` 对同一个原始 `delivery` 快照先调用 `maybeAutoAdvancePickup` 再调用 `maybeAutoConfirmPickup`；`maybeAutoConfirmPickup` 在 `:259-264` 要求传入 delivery.Status == "picking"。但此处传入的仍是创建 dwell 前读取到的 assigned 快照；`dwell_pickup` 又受 `(delivery_id,event_type)` 唯一约束限制。
- 风险条件：系统期望骑手到店驻留后既可自动开始取餐、又可继续自动确认取餐；第一次 dwell 把 delivery 改为 picking 后，第二个函数因旧快照跳过。后续位置上报虽然读取到 picking，但 `dwell_pickup` 已存在，`created=false`，不会再触发 `maybeAutoConfirmPickup`。
- 现有保护：骑手可手动确认取餐；如果 delivery 一开始就是 picking，首次 dwell_pickup 可触发自动确认取餐。
- 候选原因：状态推进链使用旧快照和事件唯一键耦合，形成“一次 dwell 只能消费一次”的隐藏顺序依赖；需确认产品是否真的启用了 `GeofenceAutoPickupEnabled`，以及测试是否覆盖 assigned -> picking -> picked 自动双阶段。

### 152. 围栏事件只按 delivery_id/event_type 去重，不绑定 rider/status/位置窗口，可能让早期或错误事件占用后续合法触发机会

- 链路：位置上报 -> `createDeliveryLocationEvent` -> `ON CONFLICT (delivery_id,event_type) DO NOTHING`。
- 证据点：`api/rider_location_events.go:138-159` 事件参数包含 rider_id、经纬度、recorded_at、source；但 SQL 去重键只看 delivery_id + event_type。`processDeliveryLocationEvents` 在创建前只用当前 delivery 快照判断 rider/status/target。
- 风险条件：配送被改派、delivery 状态回退/重建、第一次事件位置精度边界异常、旧 recorded_at 乱序到达；早期 event 占用 `arrive_pickup/dwell_pickup/arrive_dropoff/dwell_dropoff`，后续真正满足条件的事件无法新建，也不会触发自动状态副作用。
- 现有保护：处理前校验 delivery 当前 rider_id 与上报 rider 一致；不同 delivery id 不互相影响。
- 候选原因：事件幂等键表达“一单每种事件只能发生一次”，但自动推进语义依赖“某状态窗口内的一次有效 dwell”；需验证改派/重试/乱序位置上报时是否会错过触发。

### 153. 围栏自动送达使用位置上报传入的 rider 快照做确认半径校验，需确认它一定包含刚写入的最新位置

- 链路：位置上报 -> 更新 rider/location -> `processDeliveryLocationEvents(ctx, rider, deliveryID, latest)` -> `AutoConfirmDelivery` -> `validateDeliveryConfirmRadius(rider, delivery, ...)`。
- 证据点：`logic/delivery_geofence.go:121-128` 自动送达校验调用 `validateDeliveryConfirmRadius`；该函数在 `logic/delivery_status.go:38-89` 读取的是 `rider.CurrentLongitude/CurrentLatitude/LocationUpdatedAt`，不是直接使用本次 `latest` 点。`api/rider_location_events.go` 传入的 rider 来源尚需继续读取位置上报入口确认。
- 风险条件：位置上报入口先读 rider、后写 location/current_position，再用旧 rider 调 geofence；或者 current_position 写入失败但 location point 已记录；自动送达可能因为 rider 快照缺失/过期失败，而 dwell_dropoff event 已创建，后续不再重试。
- 现有保护：已补读 `api/rider.go:1167-1188`，位置入口会先 `UpdateRiderLocation` 并把 `updatedRider` 传给围栏处理，因此“旧 rider 快照”风险降低；手动确认送达可重试。
- 候选原因：同一次上报的 `latest` 与校验使用的 rider profile 仍不是同一事务事实源；主要剩余风险转为候选 154/155 的半状态和时间戳语义。

### 154. 骑手位置轨迹写入、当前定位更新、区域同步、围栏事件处理非事务，任一步失败会留下半状态

- 链路：`updateRiderLocation` -> `BatchCreateRiderLocations` -> `UpdateRiderLocation` -> `syncRiderCurrentRegion` -> `processDeliveryLocationEvents`。
- 证据点：`api/rider.go:1160-1188` 顺序执行四段写/处理，未包事务；任一错误会提前返回，前面已成功的写入不会回滚。
- 风险条件：轨迹批量插入成功但 current location 更新失败；current location 更新成功但 region 同步失败；region 同步成功但围栏事件创建或自动状态推进失败。客户端收到 500/400 后重试，可能产生重复轨迹、当前位置已变化但围栏未触发，或围栏触发与返回结果不一致。
- 现有保护：rider_locations 是轨迹型数据，可重复记录；围栏事件自身有去重；后续手动状态 API 可补推进。
- 候选原因：位置上报链路混合了 telemetry、rider profile、region、delivery state side-effect，弱顺序下容易出现部分成功；需确认是否需要 outbox/异步围栏处理或把用户可见错误与已提交事实区分。

### 155. UpdateRiderLocation 使用服务端 now() 作为 location_updated_at，可能让接近 1 小时旧的 recorded_at 被视为新鲜定位

- 链路：位置上报时间校验 -> 取 batch 中 recorded_at 最新点 -> `UpdateRiderLocation` -> 自动/手动确认送达校验 `LocationUpdatedAt`。
- 证据点：`api/rider.go:1073-1084` 允许 1 小时内历史位置；`api/rider.go:1167-1171` 更新 current 经纬度时只传经纬度；`db/query/rider.sql:43-52` 的 `UpdateRiderLocation` 写 `location_updated_at = now()`，不使用该点的 `recorded_at`。`logic/delivery_status.go:51-69` 送达确认校验的是 `rider.LocationUpdatedAt` 与当前时间差。
- 风险条件：设备离线缓存了 30-60 分钟前的接近收货点位置，恢复网络后上报；系统把 current location 时间更新为服务端当前时间，送达确认半径/新鲜度校验可能通过。
- 现有保护：超过 1 小时的历史位置会被拒绝；围栏 dwell 需要多采样和 recorded_at 窗口。
- 候选原因：current location 的时间语义是“服务器收到时间”而不是“位置采集时间”；需确认送达确认的 `LocationMaxAgeSec` 应基于采集时间还是接收时间。

### 156. 位置上报只绑定第一个 active delivery，多活跃配送半状态下轨迹和围栏可能落到错误配送

- 链路：位置上报 -> `ListRiderActiveDeliveries` -> `activeDeliveryID = activeDeliveries[0].ID` -> 每个未显式 delivery_id 的点都绑定该 delivery -> 围栏处理该 delivery。
- 证据点：`api/rider.go:1103-1107` 取第一条 active delivery；`db/query/delivery.sql:136-138` active deliveries 按 `created_at ASC` 返回所有 assigned/picking/picked/delivering；未见此处强制只能有一条。
- 风险条件：由于取消/完成/抢单竞态留下同一骑手多条 active delivery，或旧 delivery 未终态；位置上报未带 delivery_id 时会绑定最早 active delivery，后续围栏自动推进/轨迹查询也围绕该 delivery。
- 现有保护：正常业务应通过抢单资格和配送状态控制避免多活跃；如果客户端显式传 delivery_id，API 会要求它等于当前选中的 activeDeliveryID，从而至少暴露冲突。
- 候选原因：位置入口把“第一条 active”当作唯一事实；需要验证是否有 DB/业务约束确保 rider 同时只能有一条 active delivery。

### 157. 配送状态变更后的用户通知错误被吞，状态已推进但用户可能未收到关键节点通知

- 链路：骑手抢单/开始取餐/确认取餐/开始配送/确认送达/围栏自动推进 -> 主状态事务成功 -> `sendDeliveryStatusNotification`。
- 证据点：`api/delivery.go:20-38` 中 `sendDeliveryStatusNotification` 调用 `server.SendNotification` 后用 `_ =` 忽略错误；`api/delivery.go:493/568/626/679/747` 和 `api/rider_location_events.go:248/276/318` 均在状态推进后调用。
- 风险条件：通知服务、Redis/asynq 或 DB 短暂失败；订单/配送状态已改变，用户未收到“已接单/取餐/配送/送达”等提醒，可能继续等待或误操作。
- 现有保护：用户主动查询订单/配送状态能看到主状态；通知不是主状态源。
- 候选原因：状态事实与通知副作用没有 outbox 原子绑定，且调用方无重试/告警；需确认通知缺失是否可接受，或是否有通知失败监控。

### 158. 20 分钟配送延迟标记先写 `is_delayed`，后续商户通知失败不会重试

- 链路：`cleanupStaleDeliveries` 轻微超时分支 -> `ListPendingDeliveriesBefore` -> `UpdateDeliveryDelayed` -> 查询订单/商户 boss -> 入队商户通知。
- 证据点：`scheduler/data_cleanup.go:1414-1497` 先把 delivery 标记 delayed，再发送商户通知；商户 boss 缺失、taskDistributor 为空或通知入队失败只记录日志。`db/query/delivery.sql:101-105` 的 `UpdateDeliveryDelayed` 只按 id 更新。
- 风险条件：标记 delayed 成功后通知失败；后续扫描因 `delivery.IsDelayed` 为 true 跳过，不再提醒商户，但系统已认为该配送延迟已处理。
- 现有保护：另有 operator pending dispatch alert 3 分钟提醒链路；日志可人工排查。
- 候选原因：延迟 ledger 和通知投递顺序反了，且 delayed 字段兼任“已提醒”去重；需验证商户通知失败后是否有补偿。

### 159. 20 分钟配送延迟标记最终 update 不复核 pending，可能晚到标记已被接单/取消的 delivery

- 链路：延迟扫描取样 pending delivery -> delivery 状态被抢单/取消/完成 -> `UpdateDeliveryDelayed` 后到。
- 证据点：`scheduler/data_cleanup.go:1414-1436` 取样后只在内存中看 `delivery.IsDelayed` 和 created_at；最终 `UpdateDeliveryDelayed` SQL 只按 delivery id，不要求 `status='pending'`。
- 风险条件：delivery 在取样后被骑手抢单或被 stale cleanup 取消；后到 delayed 标记仍可写入 assigned/cancelled 等状态，影响展示、SLA 或后续统计。
- 现有保护：扫描对象来自 pending 快照；这只是副作用字段，不改变主状态。
- 候选原因：read-check-write 条件未沉到最终 update；需并发验证 pending -> assigned/cancelled 与 delayed 标记的交错。

### 160. operator pending dispatch alert ledger 可能在任务实际跳过后仍保留，抑制后续同 key 提醒

- 链路：operator alert scheduler -> `CreateDeliveryTimeoutAlert` ledger -> 入队 worker -> worker 重新读取 delivery，若不再 pending/阈值未到/无 recipient 则跳过。
- 证据点：`scheduler/operator_dispatch_alert.go:41-72` 先创建 `delivery_timeout_alerts` 再入队；`worker/task_operator_pending_dispatch_alert.go:55-104` 对 delivery not found、status 非 pending、阈值未到、无 recipient 等情况直接 nil；未删除 ledger。`delivery_timeout_alerts` 对 `(delivery_id,alert_key)` 唯一。
- 风险条件：scheduler 与抢单并发，ledger 创建后 worker 看到 assigned 跳过；若后续同 delivery 因状态修复或异常回到 pending，或首次 worker 因 recipient 暂缺跳过，该 alert_key 已存在，后续扫描不会再提醒。
- 现有保护：正常配送不应从 assigned 回到 pending；recipient 暂缺可由运营配置监控发现。
- 候选原因：ledger 表示“已尝试提醒”而不是“已成功通知”；需确认这是预期去重语义，还是应区分 delivered/skipped/failed。

### 161. operator pending dispatch alert 对多个 recipient 串行发送，部分成功后失败会重试并可能重复通知前面 recipient

- 链路：alert worker -> `ListActiveOperatorNotificationRecipientsByRegion` -> for recipients -> `executeSendNotificationPayload`。
- 证据点：`worker/task_operator_pending_dispatch_alert.go:105-136` 逐个 recipient 同步发送；任一发送失败返回错误，asynq 会重试整任务。已发送成功的 recipient 没有 per-recipient ledger。
- 风险条件：第一个运营收到了提醒，第二个发送失败；任务重试后第一个再次收到同一 delivery alert，第二个可能才收到。
- 现有保护：通知实现可能按 related/type 做自身去重，需继续核查；alert ledger 可防止 scheduler 重复入队同一 delivery/key。
- 候选原因：任务级幂等和 recipient 级副作用粒度不一致；需验证通知表是否有唯一键或 find-existing 逻辑。

### 162. 手动配送状态日志仍在事务外且错误忽略，已确认具体覆盖 ConfirmPickup/StartDelivery/ConfirmDelivery

- 链路：`ConfirmPickup`/`StartDelivery`/`ConfirmDelivery` -> 状态事务成功 -> `CreateOrderStatusLog`。
- 证据点：`logic/delivery_status.go:232-244`、`:302-314`、`:373-385` 都在事务成功后用 `_, _ = store.CreateOrderStatusLog(...)`；日志 from_status 使用事务前读取的 `oldStatus`。
- 风险条件：状态推进成功后日志失败；或并发导致事务内实际 order old status 与事务外 `oldStatus` 不一致，日志记录的 from/to 与真实迁移不完全对应。
- 现有保护：主 delivery/order 状态在事务内更新；日志只用于审计/展示。
- 候选原因：这是候选 99/113 的具体落点；需验证状态日志是否参与 SLA、赔付、客服判断，若参与则需要强一致或补偿。

### 163. 通知表无业务幂等键，任务重试会重复创建同一 related 通知

- 链路：任意通知入队 -> `ProcessTaskSendNotification` -> `executeSendNotificationPayload` -> `CreateNotification`。
- 证据点：`worker/task_send_notification.go:94-134` 每次处理都会调用 `CreateNotification`；`db/query/notification.sql:1-12` 只是 insert，没有 `ON CONFLICT` 或 `(user_id,type,related_type,related_id,...)` 唯一约束。`GetNotificationsByRelated` 只被部分 claim notify action 用于自查，通用通知 worker 未用。
- 风险条件：asynq 重试、operator alert 多 recipient 部分失败重试、API fallback/worker 重复执行；同一用户可能收到多条同一 delivery/order/recovery_dispute 通知。
- 现有保护：WebSocket push 失败不让任务失败；部分业务 action 自己实现了 find-existing。
- 候选原因：通知系统默认是 at-least-once insert，不是 idempotent delivery；需逐高频通知确认是否接受重复，或需要业务幂等 key。

### 164. 非测试环境下 `SendNotification` 无 task distributor 时起 goroutine 后立即返回 nil，调用方无法感知持久化失败

- 链路：API 状态变更后调用 `SendNotification` -> taskDistributor nil -> goroutine `sendNotificationInternal`。
- 证据点：`api/notification_helper.go:90-115` 在 taskDistributor 为空且非 test 时启动后台 goroutine 后返回 nil；内部创建通知失败只 log，不返回给调用方。配送状态通知又在 `api/delivery.go:20-38` 忽略 `SendNotification` 返回值。
- 风险条件：队列未配置/降级运行、store 暂时失败、进程在 goroutine 执行前退出；主业务状态已提交，通知事实可能完全缺失。
- 现有保护：正常生产应配置 taskDistributor；关键路径也可调用 `SendNotificationSync`，但配送状态通知未使用。
- 候选原因：通知 fallback 是 best-effort，且和业务状态没有 outbox/确认；需确认生产不会以 nil distributor 跑核心 API。

### 165. 抢单后 ETA 更新在主事务外，慢地图调用后无状态 guard 写 delivery，可能晚到覆盖取消/完成后的 ETA

- 链路：`grabOrder` -> `GrabDeliveryOrder`/`GrabOrderTx` 成功 -> `UpdateDeliveryEstimatedTime` -> 外部 LBS distance matrix -> `UpdateDeliveryEstimatedTime` SQL。
- 证据点：`api/delivery.go:465-487` 抢单成功后再计算 ETA，失败只 log；`logic/delivery_estimate.go:43-115` 可能调用 map client 后才写 ETA；`db/query/delivery.sql:177-181` 的 `UpdateDeliveryEstimatedTime` 只按 delivery id 更新，不校验 status 或 rider。
- 风险条件：地图服务慢响应期间，订单被取消、配送被取消或状态推进；后到 ETA 仍写入 terminal/非预期状态 delivery，影响展示/SLA/告警。
- 现有保护：ETA 不是主状态；该更新通常紧跟抢单。
- 候选原因：外部调用与最终写库之间没有 expected delivery status/version；需验证 ETA 是否被 terminal 状态页面、超时告警或赔付逻辑消费。

### 166. 抢单主事务成功后 ETA、通知、广播是三段 best-effort，客户端成功不代表周边投影齐全

- 链路：抢单成功 -> ETA 更新 -> 商户通知 -> 附近骑手 order-gone 广播 -> 重新读取 delivery 返回。
- 证据点：`api/delivery.go:465-501` ETA 失败只 log；`sendDeliveryStatusNotification` 忽略通知错误；`deliveryBroadcast.BroadcastOrderGone` 返回值被 `_ =` 忽略；最后再 `GetDelivery` 返回。
- 风险条件：骑手已抢单并冻结押金/订单进入 courier_accepted，但 ETA 未更新、商户未通知、其它骑手大厅未收到移除广播；可能造成重复点击、展示滞后或商户误判。
- 现有保护：主状态和 delivery pool 移除在 `GrabOrderTx` 中完成；其它骑手再抢单会被 DB guard 拦截。
- 候选原因：主状态与体验投影/通知是弱一致；需确认这些投影缺失是否会引发用户重复操作或运营告警。

### 167. 订单支付超时 worker 的 `GetOrderForUpdate` 不与取消事务共事务，锁释放后仍存在支付成功/状态推进竞态

- 链路：`ProcessTaskOrderPaymentTimeout` -> `GetOrderForUpdate` 读取 pending -> 超时判断 -> `CancelOrderTx`。
- 证据点：`worker/task_order_timeout.go:60-103` 直接调用 store query 的 `GetOrderForUpdate`，随后再调用 `CancelOrderTx`；若没有显式事务，`FOR UPDATE` 锁在该语句结束后释放，不能保护后续取消判断。`scheduler/order_timeout.go:63-92` 则是先 list pending，再逐个 `CancelOrderTx`。
- 风险条件：timeout worker 读到 pending 后，支付成功回调或 payment fact application 在 `CancelOrderTx` 前把 order/payment 推进；取消事务和支付成功事务交错，可能表现为取消失败重试、支付后被取消、或 paid payment 与 cancelled order 的补偿链路。
- 现有保护：`CancelOrderTx` 自身用 `OldStatus`/order status guard，支付成功链路通常也有 expected-state；宝付 pending payment timeout 会委托 payment-order timeout。
- 候选原因：timeout 的锁意图没有覆盖完整决策-执行窗口；需用支付成功与 timeout 同时到达的并发测试确认最终状态和错误映射。

### 168. 订单 timeout scheduler 和 timeout task 是两条取消入口，可能并发处理同一 pending order

- 链路 A：`OrderTimeoutScheduler.cleanupTimeoutOrders` 每 5 分钟 list pending -> `CancelOrderTx`。
- 链路 B：asynq `TaskOrderPaymentTimeout` 到期 -> `ProcessTaskOrderPaymentTimeout` -> `CancelOrderTx` 或委托 payment timeout。
- 证据点：`scheduler/order_timeout.go:63-92` 和 `worker/task_order_timeout.go:60-118` 都会处理 pending 超时订单；两者都以当前 pending 快照为入口。
- 风险条件：同一 order 同时被 scheduler 和 task 处理；一个取消成功，另一个进入 `CancelOrderTx` 后遇到状态不匹配。若错误映射不幂等，可能产生告警/重试；若状态日志/通知在其中一个路径缺失，也会出现投影差异。
- 现有保护：`CancelOrderTx` 预期按 old status 阻止重复取消；scheduler 只 log 后继续。
- 候选原因：同一业务 timeout 有重复执行者，依赖底层状态 guard 收敛；需确认重复取消不会重复日志/退款/通知，也不会把正常支付成功误判为失败。

### 169. payment-order timeout 先外部查询/关闭，再本地 closed，再取消业务订单，多段提交可能留下 closed payment_order + pending order

- 链路：`ProcessTaskPaymentOrderTimeout` -> `preparePaymentOrderTimeoutClose` provider query -> `closeRemotePaymentOrderForTimeout` -> `UpdatePaymentOrderToClosed` -> `CancelOrderTx`。
- 证据点：`worker/task_payment_timeout.go:82-145` 在 payment_order pending 分支中外部查询/关闭成功后先把本地 payment_order 更新 closed，再读取并取消业务 order；两个本地状态更新不在同一事务。
- 风险条件：payment_order 已 closed，但随后 `GetOrderForUpdate`/`CancelOrderTx` 失败、进程崩溃、或 order 已被其它链路推进；本地支付单关闭事实与业务订单取消事实分离。
- 现有保护：timeout task 重试遇到 payment_order status `closed` 会继续检查业务 order 并尝试取消；如果 order 已非 pending 会跳过取消。
- 候选原因：这是有重试恢复设计的多段状态链，但依赖任务后续能重进；需验证重试耗尽/任务丢失时是否有 closed payment_order + pending order 补扫。

### 170. timeout 查询发现远端已支付时，支付 fact application 入队失败会让本地 paid 与业务订单应用分离

- 链路：payment-order timeout query -> 远端 SUCCESS -> direct `UpdatePaymentOrderToPaid` 或 baofu `RecordPaymentFact` -> 创建 external fact application -> enqueue application。
- 证据点：`worker/task_payment_timeout.go:205-232` direct SUCCESS 会更新 payment_order paid、记录 fact、入队 application；`worker/task_payment_timeout_baofu.go:81-124` Baofu SUCCESS 记录 fact/application 后入队；入队失败直接返回错误，依赖 asynq 重试。
- 风险条件：远端已支付，本地 payment_order/fact 已部分写入，但 application 入队失败或任务重试耗尽；order/reservation 等业务状态未及时应用支付成功。
- 现有保护：external payment fact/application 有去重和 scheduler/worker 恢复链路（前文已记录 payment fact 多步非事务候选）；timeout task 返回错误会触发重试。
- 候选原因：timeout 路径也参与支付成功事实创建，且同样不是 outbox 原子交付；需确认 payment fact application recovery 覆盖 timeout query fact。

### 171. payment-order timeout 对 USERPAYING/查询异常返回错误反复重试，期间 order 可能被其它 timeout 入口取消

- 链路：payment-order timeout -> provider query -> direct `USERPAYING` 或 Baofu query error/config error -> return error -> asynq retry；另有 order timeout scheduler/task 可能处理同一业务 order。
- 证据点：`worker/task_payment_timeout.go:237-239` direct USERPAYING blocks timeout close 并返回错误；`worker/task_payment_timeout_baofu.go:15-60` provider/client/config/query nil 等异常均返回错误。`worker/task_order_timeout.go` 可委托 pending Baofu payment-order timeout，但 `scheduler/order_timeout.go` 仍可直接 `CancelOrderTx` pending order。
- 风险条件：远端支付中，本地 payment-order timeout 因 USERPAYING 或查询异常暂不关闭；同时 order timeout scheduler 直接取消 pending order，随后远端支付成功事实晚到，形成 paid payment 与 cancelled order 的补偿场景。
- 现有保护：order timeout worker 会对 Baofu pending payment 委托 payment-order timeout；payment success fact application 应处理后到成功。
- 候选原因：多个 timeout 执行者对“远端支付中”的处理语义不同；需验证 direct/Baofu pending payment 的 order scheduler 是否也应委托，或至少不绕过 payment_order 查询。

### 172. payment-order timeout remote close 成功但本地 `UpdatePaymentOrderToClosed` 失败，重试会重复外部关闭或进入已关闭远端状态

- 链路：timeout pending -> provider close -> local `UpdatePaymentOrderToClosed`。
- 证据点：`worker/task_payment_timeout.go:93-99` 先 `closeRemotePaymentOrderForTimeout`，再 `UpdatePaymentOrderToClosed`；direct close 对 `ORDER_CLOSED` 幂等处理，Baofu close 通过 service `CloseOrder`，具体远端幂等已在支付事实链路中另行候选。
- 风险条件：外部支付单已关闭，本地 DB 更新失败；任务重试会再次 query/close，可能看到 closed/failed/unknown 后再走不同分支。
- 现有保护：direct close 忽略已关闭错误；Baofu query 会先识别 terminal closed/failed 后跳过 remote close。
- 候选原因：外部事实和本地状态没有事务，需验证 provider close 幂等以及 retry 后能稳定把本地 payment_order 收敛到 closed。

### 173. delivery damage 更新 SQL 存在但未找到明确业务写入口，若未来接入需补状态 guard

- 链路：潜在配送损坏/赔付入口 -> `UpdateDeliveryDamage`。
- 证据点：`db/query/delivery.sql:92-99` 的 `UpdateDeliveryDamage` 只按 delivery id 写 `is_damaged/damage_amount/damage_reason`；全仓 `rg` 当前只找到生成代码、字段读取和 SQL 定义，未见 API/worker 明确调用。
- 风险条件：若后续接入异常上报或赔付自动标记，晚到 damage 写入可能落到 cancelled/completed/delivered 等终态配送，影响骑手信用、赔付、SLA 统计。
- 现有保护：当前似乎没有生产写入口；因此风险是“潜伏分支/未来接入需注意”，不是已确认运行路径。
- 候选原因：字段更新 SQL 已具备无状态 guard 的形状；需在接入前要求 expected delivery status 或独立事件 ledger。

### 174. external payment fact application 进入 `processing` 后没有 stale reclaim，worker 崩溃可能永久悬挂

- 链路：payment fact/application 创建 -> `ProcessTaskPaymentFactApplication` -> `ApplyExternalPaymentFactApplication` -> `ClaimExternalPaymentFactApplication`。
- 证据点：`db/query/external_payment_fact.sql:289-317` 的 claim 只允许 `pending/failed -> processing`，applied/failed 也只允许从 `processing` 更新；`ListRetryableExternalPaymentFactApplicationsByTarget` 只扫 `pending/failed`（同文件 `327-335`）。`logic/payment_fact_application_service.go:196-200` claim 不到记录就 `Skipped=true`；`worker/task_payment_fact_application.go:105-111` 对 skipped 返回 nil。
- 风险条件：task claim 成功后进程崩溃、context 超时、DB 连接中断，或业务 side-effect 长时间卡住；asynq 后续重试同一 application 时因为状态已是 `processing` 被跳过，scheduler 也不再捞它。
- 现有保护：claim/mark 使用状态 CAS 能避免并发双处理；但未看到 `processing` 超时回收或 lease 字段。
- 候选原因：application 的 `processing` 更像永久状态而不是租约；需查生产是否存在长时间 `processing`，并确认是否需要 stale scanner 或 processing deadline。

### 175. payment fact application 的 domain side-effect、outbox、fact terminalized、application applied 是多段提交

- 链路：claim application -> get fact -> apply domain -> create domain outbox -> update fact processing_status terminalized -> mark application applied。
- 证据点：`logic/payment_fact_application_service.go:215-245` 按上述顺序逐步调用 store 方法；这些调用不在同一个显式 transaction 中。`db/query/external_payment_fact.sql:248-255` 的 fact processing_status 更新只按 fact id，不校验当前 processing_status。
- 风险条件：任一阶段成功后进程崩溃或下一阶段失败，会留下 domain 已变更但 outbox/fact/application 未同步、outbox 已存在但 application 未 applied、fact terminalized 但 application 仍 processing 等半状态。
- 现有保护：application mark 使用 `status='processing'`，outbox 部分使用 `CreatePaymentDomainOutboxOnce`；重试理论上可收敛一部分半状态。
- 候选原因：支付事实应用承载跨域副作用，但没有一个统一事务/状态机包住所有投影；需逐 consumer 验证每个半状态是否可被重试安全收敛。

### 176. domain side-effect 成功后 outbox 创建失败会把 application 标 failed，后续重试可能重复执行 domain

- 链路：`applyExternalPaymentFactToDomain` 成功 -> `createPaymentDomainOutboxForAppliedFact` 失败 -> `markExternalPaymentFactApplicationFailed` -> scheduler 重试。
- 证据点：`logic/payment_fact_application_service.go:215-227` 先应用 domain，再创建 outbox；outbox 失败会调用 `markExternalPaymentFactApplicationFailed`。`logic/payment_fact_application_failure.go:12-41` 将 application 标为 `failed` 并设置 `next_retry_at`。
- 风险条件：domain 更新已提交，但 outbox marshal/DB 写入/ensure bill 等失败；重试再次进入 domain apply。若 consumer 的 domain apply 不是严格幂等，可能重复确认订单、重复释放追偿、重复退款投影、重复分账终态副作用。
- 现有保护：部分 domain apply 预计有状态 guard 或幂等查询；outbox 使用 business aggregate 去重，但不能保护 outbox 之前的 domain 重放。
- 候选原因：失败标记覆盖的是 application 而非 domain side-effect ledger；需逐个 `apply*PaymentFact` 验证“side-effect 成功 + outbox 失败 + 重试”的语义。

### 177. outbox 创建成功后 fact/application mark 失败，重试可能再次应用 domain 并复用/冲突 outbox

- 链路：domain apply 成功 -> `CreatePaymentDomainOutboxOnce` 成功 -> `UpdateExternalPaymentFactProcessingStatus` 或 `MarkExternalPaymentFactApplicationApplied` 失败 -> mark failed/返回错误 -> 重试。
- 证据点：`logic/payment_fact_application_service.go:225-245` outbox 后才更新 fact/application；`db/query/external_payment_fact.sql:355-379` 的 `CreatePaymentDomainOutboxOnce` 以 `(event_type, aggregate_type, aggregate_id)` 去重，payload 相同或去掉 `external_payment_fact_id/payment_fact_application_id` 后相同才返回既有记录。
- 风险条件：outbox 已 pending/published，但 application 没有 applied；重试再次执行 domain apply，再创建同一 outbox。若 payload 因业务状态变化而不同，`ON CONFLICT ... WHERE` 不匹配可能返回 no row/错误；若 payload 被认为相同，则 outbox 复用但 domain side-effect 仍已重放。
- 现有保护：outbox 去重可降低重复消息；application mark CAS 能避免并发覆盖 applied。
- 候选原因：outbox 幂等键不能覆盖 domain side-effect，也可能因为 payload drift 暴露 conflict；需验证 sqlc 对 no-row conflict 的处理和 worker 重试后最终状态。

### 178. `CreateExternalPaymentFactApplication` 冲突时只返回旧行，不会唤醒卡住或长延迟 failed application

- 链路：外部事实重复到达/timeout recovery 重建 fact application -> `CreateExternalPaymentFactApplication`。
- 证据点：`db/query/external_payment_fact.sql:257-275` 对 `(fact_id, consumer, business_object_type, business_object_id)` 冲突仅 `DO UPDATE SET updated_at = external_payment_fact_applications.updated_at` 并返回旧状态，不重置 `status/next_retry_at/last_error`。
- 风险条件：旧 application 已卡在 `processing`，或 failed 的 `next_retry_at` 很久以后；重复回调、查询恢复、timeout 查询成功 fact 只能拿到旧 application，无法立即重新排队有效处理。
- 现有保护：对正常重复事实这是幂等；scheduler 会在 `next_retry_at` 到期后重扫 failed。
- 候选原因：创建幂等和“唤醒恢复”复用了同一个入口；需确认所有 recovery 路径是否另有强制 reset/requeue 能处理卡住状态。

### 179. `CreateExternalPaymentFact` 同 dedupe_key 但字段不完全一致时可能不返回记录，导致重复事实无法应用

- 链路：外部回调/查询/recovery 记录 payment fact -> `CreateExternalPaymentFact`。
- 证据点：`db/query/external_payment_fact.sql:156-225` 对 `dedupe_key` 冲突使用 `DO UPDATE ... WHERE`，要求 provider/channel/capability/source/business/upstream_state/terminal_status/amount/currency 等字段均与 excluded 一致才返回记录。
- 风险条件：同一外部对象同 dedupe_key 被不同来源以不同 raw/upstream_state/amount/business_object_id 记录，冲突 WHERE 不匹配时 SQL 可能返回 no row；调用方若把它当错误，后续 application 不创建或不入队。
- 现有保护：严格字段一致性能防止把不同事实误并为一个；dedupe_key 设计理论上应包含足够维度。
- 候选原因：dedupe_key 与事实字段一致性是隐含契约；需抽样所有 fact 创建者确认同一 dedupe_key 不会因回调/查询/恢复来源差异产生字段漂移。

### 180. payment fact scheduler 使用 `asynq.Unique(30s)` 且只扫固定 target，任务仍在队列/processing 时可能延迟恢复

- 链路：`PaymentFactApplicationScheduler.RunOnce` -> `ListRetryableExternalPaymentFactApplicationsByTarget` -> `DistributeTaskProcessPaymentFactApplication`。
- 证据点：`worker/payment_fact_application_scheduler.go:108-131` 每轮按固定 targets 扫 pending/failed，并以 `asynq.Unique(paymentFactApplicationTaskUnique)` 入队；常量为 30 秒。SQL 入口只筛 `pending/failed`。
- 风险条件：同一 application 任务仍在队列或刚被 unique 抑制，enqueue 失败只 log；若 worker claim 后变 `processing`，后续 scheduler 不会再看到它。对 failed retry 是延迟问题，对 processing 悬挂是丢恢复问题。
- 现有保护：scheduler 每分钟运行，pending/failed 通常会下一轮再捞；runMu 避免同实例并发 run。
- 候选原因：队列唯一性、DB 状态和 worker claim 没有统一租约；需验证 asynq unique 错误是否被误判为异常、以及 stuck processing 监控是否存在。

### 181. payment recovery scheduler 只覆盖 direct paid-unprocessed，Baofu 和其它支付渠道依赖外部专用路径

- 链路：`PaymentRecoveryScheduler` -> `ListPaidUnprocessedPaymentOrders` -> direct recovery fact/application 或 skip。
- 证据点：`worker/payment_recovery_scheduler.go:99-149` 对 `shouldRecordDirectPaymentRecoveryFact` 命中者记录 recovered direct payment fact 并 enqueue；`PaymentChannelBaofuAggregate` 直接 warn skip，注释为 dedicated baofu scheduler；其它 channel/business type 也 warn skip。
- 风险条件：Baofu 专用 scheduler 未运行、漏扫某 business type、或新增支付渠道没有配置 target；paid payment_order 可能长期未应用到订单/预约/押金/追偿等业务状态。
- 现有保护：Baofu 有专用恢复链路；direct 常见业务类型有恢复 fact。
- 候选原因：recovery 覆盖面由多个 scheduler 分摊，缺少统一“paid but business unapplied”闭环；需交叉查询所有 paid-unprocessed 分类。

### 182. payment fact application 创建/入队分离，入队失败依赖 scheduler；但 scheduler 不会补 processing 和未知 target

- 链路：各回调/timeout/recovery 路径创建 fact/application -> enqueue `TaskPaymentFactApplication`；enqueue 失败返回错误或只 log -> scheduler 后续补扫。
- 证据点：`worker/payment_recovery_scheduler.go:122-129` recovered direct enqueue 失败只 log；`worker/payment_fact_application_scheduler.go:108-131` 后补只按配置 target 和 pending/failed 扫描。
- 风险条件：创建 application 成功但入队失败；若 application 初始 pending 且 target 配置完整，可被补扫；若 consumer/business_object_type 未在 target 列表、或旧 application 已是 processing，则补偿缺口出现。
- 现有保护：有 payment fact application scheduler 作为 pending/failed 恢复；多数核心 target 预计已配置。
- 候选原因：可靠投递依赖“写 DB + 后台扫表”模式，但 target 白名单与状态过滤决定恢复边界；需列出 `paymentFactApplicationSchedulerTargets` 与所有 application 创建点对照。

### 183. claim recovery release action 在 payment fact application applied 后才异步派发，派发失败只 log

- 链路：追偿支付成功 fact application -> 生成 `ClaimRecoveryReleaseAction` -> application applied -> `ProcessTaskPaymentFactApplication` 派发 behavior action。
- 证据点：`worker/task_payment_fact_application.go:113-128` 如果 result 带 release action，distributor nil 或派发失败只 log，不让 payment fact application 失败重试；该 action 执行框架前文已有 action recovery 候选。
- 风险条件：追偿支付事实已 applied，application 不会再重试；release action 任务未派发或派发失败，只能依赖行为 action 自身 recovery scheduler。如果 action recovery 漏扫或 action 未落库成功后才返回，追偿暂停/解冻可能滞后或缺失。
- 现有保护：release action 本身已在 domain apply 中创建；若 action recovery scheduler 覆盖 pending/running，应能最终执行。
- 候选原因：支付 fact application 和后续 behavior action 投递是弱一致；需验证 action recovery scheduler 是否覆盖该 action 类型和所有半状态。

### 184. payment domain outbox 进入 `processing` 后也没有 stale reclaim，worker 崩溃可能永久悬挂

- 链路：payment fact application 创建 outbox -> outbox scheduler 入队 -> `ProcessTaskPaymentDomainOutbox` -> `ClaimPaymentDomainOutbox`。
- 证据点：`db/query/external_payment_fact.sql:389-406` list/claim 只处理 `pending/failed`，claim 成功后置 `processing`；`MarkPaymentDomainOutboxPublished/Failed` 也只允许从 `processing` 更新（同文件 `408-426`）。当前未看到针对 stale `processing` outbox 的回收 SQL 或 scheduler。
- 风险条件：outbox claim 后进程崩溃、dispatch 卡死、DB 连接在 mark published/failed 前中断；后续 scheduler 因状态为 `processing` 不会再入队。
- 现有保护：claim 的状态 CAS 避免并发双发；release readiness smoke 有 fixture claimability 检查，但不是运行时恢复。
- 候选原因：outbox processing 缺少租约/超时回收；需查生产是否存在长时间 processing，以及是否有人工 SQL 补偿流程。

### 185. outbox dispatch 成功后 `MarkPaymentDomainOutboxPublished` 失败，可能导致外部副作用已发但 outbox 卡住或后续重复发

- 链路：`ProcessTaskPaymentDomainOutbox` -> `dispatchPaymentDomainOutbox` 成功 -> `MarkPaymentDomainOutboxPublished`。
- 证据点：`worker/task_payment_domain_outbox.go:80-86` 先 dispatch，再 mark published；mark published 失败后调用 `markPaymentDomainOutboxFailed`。`markPaymentDomainOutboxFailed` 本身也只在 DB 层 `status='processing'` 时能成功（`db/query/external_payment_fact.sql:418-426`）。
- 风险条件：通知任务、打印任务、no-show alert、分账结果通知、退款告警等已派发/发布；随后 DB mark published 失败，若 mark failed 也失败则 outbox 留在 processing，若 mark failed 成功则 scheduler 后续会重放 dispatch。
- 现有保护：部分下游使用 `asynq.Unique` 或状态检查；但并非所有通知/告警都有业务幂等键。
- 候选原因：outbox 的“副作用成功”和“本地发布标记”不是原子提交；需按事件类型验证重复 dispatch 的业务后果。

### 186. payment fact application 创建 outbox 后没有立即派发 outbox task，主要依赖 outbox scheduler 后扫

- 链路：`ApplyExternalPaymentFactApplication` -> `createPaymentDomainOutboxForAppliedFact` -> application applied -> `ProcessTaskPaymentFactApplication` 返回。
- 证据点：`logic/payment_fact_application_service.go:225-245` 创建 outbox 后只 mark fact/application；`worker/task_payment_fact_application.go:105-139` 只额外派发 claim recovery release action，未使用 `result.Outbox` 派发 `TaskProcessPaymentDomainOutbox`。`payment_domain_outbox_scheduler.go:115-163` 每分钟扫描 pending/failed outbox。
- 风险条件：支付成功已经应用到订单/预约/退款/分账状态，但商户通知、自动接单打印、预约 no-show alert、退款通知/告警要等 scheduler；scheduler 未启动或 event type 未配置时投影长期缺失。
- 现有保护：outbox scheduler 每分钟启动时和 cron 都会扫；pending/failed 状态可补偿。
- 候选原因：可靠性完全依赖后扫而非写后派发；需确认生产 scheduler 必启、监控覆盖 pending backlog，并评估是否需要创建后立即 enqueue。

### 187. outbox scheduler 入队任务 `MaxRetry(0)`，mark failed 失败时没有 asynq 层兜底重试

- 链路：outbox scheduler -> `DistributeTaskProcessPaymentDomainOutbox(... asynq.MaxRetry(0) ...)` -> worker dispatch/mark。
- 证据点：`worker/payment_domain_outbox_scheduler.go:147-154` 入队 outbox 处理任务时设置 `MaxRetry(0)`；worker 出错时通常先 `MarkPaymentDomainOutboxFailed` 并返回 nil（`worker/task_payment_domain_outbox.go:80-86,720-735`），但如果 mark failed 本身失败会返回 error。
- 风险条件：dispatch 失败或 mark published 失败后，DB 又无法标 failed；asynq 不会重试该任务，outbox 可能停在 processing，scheduler 也不再捞。
- 现有保护：如果 mark failed 成功，scheduler 会在 5 分钟后重试；`MaxRetry(0)` 可避免队列级重复风暴。
- 候选原因：重试责任完全转移给 DB 状态；一旦状态回写失败，队列和 DB 两边都失去恢复入口。

### 188. `CreatePaymentDomainOutboxOnce` 的唯一键是事件+聚合，payload drift 会拒绝重复创建并反向拖住 application

- 链路：payment fact application 重试 -> `createPaymentDomainOutboxForAppliedFact` -> `CreatePaymentDomainOutboxOnce`。
- 证据点：`db/query/external_payment_fact.sql:355-379` 使用 `(event_type, aggregate_type, aggregate_id)` 唯一键；只有 payload 完全相同，或去掉 `external_payment_fact_id/payment_fact_application_id` 后相同，才返回既有 outbox。
- 风险条件：第一次 domain apply 后创建了 outbox，后续重试时订单金额、退款 id、fail reason、分账 fee breakdown、业务状态快照等 payload 字段出现漂移；`ON CONFLICT ... WHERE` 不匹配会让创建失败，application 被标 failed 后持续重试。
- 现有保护：payload 严格一致能发现同一聚合下不一致事件，避免静默覆盖。
- 候选原因：outbox 幂等键与 payload 不变性耦合；需验证所有 outbox payload 字段是否来自不可变事实，而不是当前可变业务状态。

### 189. 订单支付成功 outbox 将自动接单/打印放在商户通知之前，打印派发失败会延迟新订单通知

- 链路：`dispatchOrderPaymentSucceededOutbox` -> `autoAcceptPaidOrderForPrinting` -> `distributeAutoAcceptedOrderPrint` -> `sendOrderPaidNotifications`。
- 证据点：`worker/task_payment_domain_outbox.go:169-180` 先自动接单/打印，再发送商户和骑手通知；`autoAcceptPaidOrderForPrinting` 在 `AcceptMerchantOrder` 成功后，如果 `distributeAutoAcceptedOrderPrint` 失败会返回错误（同文件 `211-226`），导致 outbox 标 failed。
- 风险条件：订单已从 paid 自动推进到 preparing/courier_accepted，但打印任务入队失败；商户新订单通知和骑手新单广播不会执行，直到 outbox 后续重试并成功处理打印分支。
- 现有保护：重试时若订单已 preparing/courier_accepted，会走 `scheduleAutoAcceptedOrderPrint`，并通过 task key/print log/unique 降低重复打印（同文件 `232-310`）。
- 候选原因：订单状态推进、打印派发、通知派发串行绑定在同一个 outbox dispatch 中；需确认打印失败是否应阻断商户新订单通知。

### 190. 订单支付成功 outbox 重放可能重复创建商户通知，且骑手新单广播是 best-effort

- 链路：outbox dispatch 重试/重放 -> `sendOrderPaidNotifications` -> `notifyMerchantNewOrder` + `notifyRidersNewDelivery`。
- 证据点：商户通知通过 `DistributeTaskSendNotification` 入队（`worker/task_process_payment.go:411-433`），通知任务最终 `CreateNotification` 无业务唯一键（`worker/task_send_notification.go:112-139`，`db/query/notification.sql:1-13`）。骑手广播失败只 log/continue（`worker/task_process_payment.go:624-630,691-720`），不会让 outbox failed。
- 风险条件：商户通知任务已入队后 outbox mark published 失败，后续 outbox 重试再次入队通知；骑手广播/pubsub 失败则 outbox 仍可 published，骑手端可能完全没收到实时新单。
- 现有保护：通知表持久化后列表可见；部分实时广播只影响实时性，抢单状态由 DB guard 保护。
- 候选原因：通知任务是 at-least-once 且缺少业务去重键，骑手广播是 best-effort；需确认用户侧是否能容忍重复商户通知和漏实时广播。

### 191. 预约支付成功 outbox 只派发 no-show alert 任务，任务自身只做当前状态检查且 websocket 发布无确认

- 链路：`dispatchReservationPaymentSucceededOutbox` -> `DistributeTaskReservationNoShowAlert(ProcessAt(reservation time), Unique(24h))` -> `ProcessTaskReservationNoShowAlert`。
- 证据点：outbox dispatch 读取当前 reservation 后按 reservation date/time 入队 no-show alert（`worker/task_payment_domain_outbox.go:364-380`）；no-show task 仅在状态为 `paid/confirmed` 时发布 websocket，然后返回 nil（`worker/task_reservation_timeout.go:182-229`）。
- 风险条件：outbox 晚处理或重试时预约时间已过，alert 可能立即触发；websocket/pubsub 失败只算 attempted；如果 outbox 重放间隔超过 24h，unique 窗口也无法阻止同一 reservation 再次入队。
- 现有保护：task 执行时会跳过非 paid/confirmed 状态；asynq unique 24h 降低短期重复。
- 候选原因：预约支付成功后的提醒是弱一致任务，不是持久业务事件；需验证 reschedule/cancel/check-in 与已排 no-show task 的交叉语义。

### 192. 退款异常 outbox 的平台告警发布失败不影响 outbox published，可能丢人工介入信号

- 链路：refund abnormal fact application -> abnormal outbox -> `dispatch*RefundAbnormalOutbox` -> `publishAlert`。
- 证据点：订单/预约/骑手押金退款异常 outbox 都调用 `processor.publishAlert` 后直接返回 nil（`worker/task_payment_domain_outbox.go:487-496,558-567,708-717`）；`publishAlert` 持久化 alert 失败只 log，pubsub publisher nil 也只 log 后返回（`worker/task_process_payment.go:120-141`）。
- 风险条件：退款已经进入 ABNORMAL，需要人工介入；alert_event 写库失败或 pubsub 未配置，outbox 仍会被 mark published，后续不再重试告警。
- 现有保护：退款单本身终态仍在 DB，可由运营列表/对账查询发现；alert 只是提醒投影。
- 候选原因：异常告警不是 outbox 成功条件；需确认运营是否还有独立扫描 ABNORMAL refund_order 的入口。

### 193. payment domain outbox 存在 `applyment_*` 常量但 dispatcher/scheduler 默认不支持，若生产写入会永久 failed/不被扫

- 链路：潜在 applyment/account onboarding outbox -> payment domain outbox scheduler/dispatcher。
- 证据点：`db/sqlc/constants.go` 定义 `PaymentDomainOutboxEventApplymentActivated`、`ApplymentPendingStateReady`、`ApplymentTerminalStateReady`；但 `defaultPaymentDomainOutboxSchedulerEventTypes` 只包含 probe/order/reservation/profit-sharing/refund 事件（`worker/payment_domain_outbox_scheduler.go:21-30`），`dispatchPaymentDomainOutbox` switch 也未覆盖 applyment 事件（`worker/task_payment_domain_outbox.go:97-117`）。当前 `rg` 未找到这些事件的生产创建点。
- 风险条件：如果旧代码、迁移脚本、未来功能或手工补偿写入 applyment outbox，默认 scheduler 不会扫描；即使被手工派发，dispatcher 也会标 unsupported failed。
- 现有保护：当前未找到创建点，因此是潜伏风险/常量残留待确认。
- 候选原因：事件枚举和 dispatcher/scheduler 白名单漂移；需确认 applyment outbox 是否废弃，或补齐事件处理/删除常量。

### 194. 宝付支付 success fact 晚到时若本地 payment_order 已 closed/failed，会让 application 失败重试而非自动补偿

- 链路：Baofu payment SUCCESS fact -> `applyOrderPaymentFact`/`applyReservationPaymentFact` -> `markBaofuPaymentOrderPaid`。
- 证据点：`logic/payment_fact_application_service.go:338-345,406-413` 先对宝付 success fact 调 `markBaofuPaymentOrderPaid`；`logic/baofu_payment_fact_application.go:31-53` 只允许 `UpdatePaymentOrderToPaid` 从 `pending` 成功，冲突后只有当前状态已经 `paid` 才视为幂等，否则返回 `baofu payment order ... is not payable after success fact`。`db/query/payment_order.sql:153-160` 的 paid 更新要求 `status='pending'`。
- 风险条件：timeout/关闭链路先把 payment_order 置 closed/failed，之后 provider 查询/回调发现远端其实已支付；success fact application 会 failed 并重试，但业务 order/reservation 不会自动应用 paid。
- 现有保护：金额校验严格；冲突会 log/返回错误，不会静默覆盖 closed/failed。
- 候选原因：本地 first-terminal-wins 与远端 success-late 的补偿策略分离；需确认是否有 paid-after-closed 的自动退款/人工对账闭环。

### 195. 普通 reservation 支付成功分支直接 `UpdateReservationStatus('paid')`，没有当前 reservation 状态 guard 且不写 `paid_at`

- 链路：reservation payment_order paid -> `ProcessPaymentSuccessTx` business_type=`reservation` -> 创建 reservation_payment -> `UpdateReservationStatus`。
- 证据点：`db/sqlc/tx_payment_success.go:142-170` reservation 分支未 lock `table_reservations`，也未检查当前 reservation status，直接调用 `UpdateReservationStatus(Status: "paid")`；`db/query/table_reservation.sql:121-126` 只按 id 更新 status/updated_at，不带 expected status；`UpdateReservationToPaid` 虽会写 `paid_at`，但该分支未使用（同 SQL `128-134`）。
- 风险条件：预约已 expired/cancelled/no_show/completed，支付成功事实晚到仍可能把 reservation 改回 paid；同时 `paid_at` 不更新可能影响提醒、统计、对账或 UI。
- 现有保护：payment_order 本身需是 paid 且 processed_at 为空；reservation_payment 有唯一约束/先查防重。
- 候选原因：支付事实应用绕过了 reservation 状态机 expected-state；需验证所有 reservation 支付成功路径是否应只允许 pending -> paid，并补 paid_at 语义。

### 196. reservation_payment 已创建但 reservation 状态未更新的半状态，重试会直接标 payment_order processed

- 链路：`ProcessPaymentSuccessTx` reservation 分支 -> `CreateReservationPayment` -> `UpdateReservationStatus` -> `syncReservationInventory` -> `UpdatePaymentOrderProcessedAt`。
- 证据点：`db/sqlc/tx_payment_success.go:147-151` 如果 `GetReservationPaymentByPaymentOrderID` 已存在就 `break`；随后统一执行 `UpdatePaymentOrderProcessedAt`（同文件 `327-332`）。如果曾经出现“reservation_payment 已存在但 reservation/status 或库存同步未完成”的数据，重试不会再补 `UpdateReservationStatus`/inventory。
- 风险条件：历史版本、手工补数据、事务边界变更、或非同事务写入导致 reservation_payment 先存在；后续 payment fact application 会认为已处理并设置 processed_at，留下 paid payment_order + reservation 非 paid/库存未同步。
- 现有保护：当前代码中 `CreateReservationPayment`、`UpdateReservationStatus`、`syncReservationInventory` 与 processed_at 都在同一个 `execTx` 内，正常事务失败会整体回滚。
- 候选原因：幂等判据只看 reservation_payment，而不是完整的 reservation/payment/inventory 投影；需查生产是否存在此类半状态。

### 197. reservation_addon 无 adjustment 旧路径会绕过预约状态/cooking guard，直接加 prepaid_amount

- 链路：payment_order business_type=`reservation_addon` -> `ProcessPaymentSuccessTx` -> 找不到 `reservation_adjustment` -> 创建 reservation_payment -> `AddReservationPrepaidAmount`。
- 证据点：`db/sqlc/tx_payment_success.go:173-212` 在找不到 adjustment 时走旧 addon 路径，未 lock/check reservation status，也未检查 `cooking_started_at`；相对地 `applyPaidReservationAdjustmentWithQueries` 会 lock reservation，并要求 status 为 paid/confirmed/checked_in 且未 cooking started（`db/sqlc/tx_reservation_adjustment.go:264-274`）。`AddReservationPrepaidAmount` SQL 只按 id 累加（`db/query/table_reservation.sql:136-142`）。
- 风险条件：旧 addon payment_order 或缺失 adjustment 的新 payment_order 支付成功晚到，可能对 cancelled/expired/no_show/completed 或已开餐预约增加 prepaid_amount。
- 现有保护：新正向调整主路径有 adjustment guard；payment_order processed_at 防重复。
- 候选原因：兼容旧路径和新 adjustment 状态机并存，保护强度不一致；需确认生产是否还有无 adjustment 的 reservation_addon 支付单。

### 198. reservation_addon adjustment 被标 `applying` 后若后续失败，终态关闭路径会把 `applying` 当可关闭并释放 hold

- 链路：addon paid fact -> `applyPaidReservationAdjustmentWithQueries` -> `MarkReservationAdjustmentApplying` -> 后续失败；之后 closed/failed payment fact 或 recovery -> `CloseReservationAdjustmentForPaymentTx`。
- 证据点：`db/sqlc/tx_reservation_adjustment.go:260-274` 先把 adjustment 标为 applying，再检查 reservation 状态/cooking；`CloseReservationAdjustmentForPaymentTx` 对 applied/closed/failed/expired 直接返回，但对 applying 会释放 holds 并标 closed/failed/expired（同文件 `366-384`）。SQL 也允许 applying -> closed/failed/expired（`db/query/reservation_adjustment.sql:127-155`）。
- 风险条件：success fact 应用已进入 applying 后，因为 reservation 状态/cooking、库存、items 写入等失败而 application failed；如果随后又收到 close/failed fact 或清理任务，可能把同一 adjustment 从 applying 关闭，和已支付成功事实产生冲突。
- 现有保护：payment_order paid success fact 应是 terminal；close/failed fact 与 success fact 同时存在理论上不应常见。
- 候选原因：adjustment 的 applying 是“正在应用成功事实”的中间态，同时也被失败关闭路径视为可关闭；需验证 provider terminal fact 乱序/重复时的收敛。

### 199. full refund 成功后 `UpdatePaymentOrderToRefunded` 无状态 guard，可能覆盖 payment_order 非 paid 状态

- 链路：refund success fact -> `applyOrderRefundFact`/`applyReservationRefundFact` -> `maybeMarkPaymentOrderRefunded` -> `UpdatePaymentOrderToRefunded`。
- 证据点：`logic/payment_fact_application_service.go:550-552,667-669,805-818` 在累计成功退款金额达到 payment amount 后调用 `UpdatePaymentOrderToRefunded`；`db/query/payment_order.sql:176-181` 只按 id 把 payment_order status 改为 refunded，不要求当前 status 为 paid/refunded。
- 风险条件：payment_order 已被 closed/failed、或处于非预期状态但关联 refund_order success 晚到/误关联；full refund fact 会把它覆盖成 refunded。
- 现有保护：refund_order 通过 payment_order_id 关联；正常退款只应针对 paid payment_order。
- 候选原因：退款终态应用依赖上游业务不产生非法 refund_order，而非 DB expected-state；需查是否存在非 paid payment_order 下的 success refund_order。

### 200. reservation refund success 已把 refund_order 标 success 后，若扣减 prepaid_amount 失败，重试不会再扣减

- 链路：reservation refund success fact -> `UpdateRefundOrderToSuccess` -> `maybeMarkPaymentOrderRefunded` -> `AddReservationPrepaidAmount(-refund_amount)` -> 后续 outbox/fact/application mark。
- 证据点：`logic/payment_fact_application_service.go:656-677` 只有本次从非 success 转为 success 时 `transitionedToSuccess=true`，才扣减 reservation prepaid_amount；如果 `UpdateRefundOrderToSuccess` 成功后 `maybeMarkPaymentOrderRefunded` 或 `AddReservationPrepaidAmount` 失败，application 会 failed。重试时 refundOrder 已是 success，`transitionedToSuccess=false`，不再执行 prepaid 扣减。
- 风险条件：DB 暂时失败、reservation 被删除/锁冲突、上下文超时发生在 refund_order success 之后、prepaid_amount 扣减之前；最终可能 refund_order=success 但 reservation.prepaid_amount 未扣。
- 现有保护：如果整个失败发生在 SQL 单语句内部则不会更新；但这些步骤不是一个 transaction。
- 候选原因：用“本次是否发生终态迁移”控制后续补偿，遇到半成功重试会跳过必要投影；需用故障注入验证 reservation refund success 后的重试。

### 201. profit_sharing_return 终态 SQL 无 expected status，success/failed late fact 可互相覆盖

- 链路：profit sharing return fact -> `applyProfitSharingReturnFact` -> `UpdateProfitSharingReturnToSuccess/Failed`。
- 证据点：`logic/payment_fact_application_service.go:971-1003` success 时只要当前不是 success 就更新 success，failed/closed 时只要当前不是 failed 就更新 failed；`db/query/profit_sharing_return.sql:46-63` 的 success/failed 更新只按 id，不要求当前状态为 processing 或非终态。
- 风险条件：provider 先返回 failed/closed，后又返回 success，或反过来；本地 return 终态会被后到事实覆盖，影响退款继续发起或失败告警。
- 现有保护：fact dedupe_key 应避免同一事实重复；但不同 terminal fact 仍可能各自进入 application。
- 候选原因：退分账状态机未采用 first-terminal-wins 或版本时间比较；需确认宝付退分账是否可能出现 terminal 纠正。

### 202. profit_sharing_return failed fact 会继续把 refund_order 标 failed；若 refund_order 已 success/closed，application 可能长期失败

- 链路：profit sharing return failed/closed fact -> `UpdateProfitSharingReturnToFailed` -> `UpdateRefundOrderToFailed`。
- 证据点：`logic/payment_fact_application_service.go:991-1003` 无论 returnRecord 是否已经 failed，都会尝试 `UpdateRefundOrderToFailed`；`db/query/refund_order.sql:166-171` 只允许 pending/processing -> failed。若 refund_order 已 success/closed，返回 no row 会让 application failed。
- 风险条件：退分账 failed fact 晚到，但退款单已因其它 return success 或人工流程进入 success/closed；application 反复重试，external fact 可能无法 terminalized。
- 现有保护：refund_order SQL 不会把 success/closed 回退到 failed。
- 候选原因：return 终态和 refund_order 终态的冲突处理不对称；需验证 late failed 后是否应忽略、告警或人工处理。

### 203. profit_sharing_order success-after-failed 被视为错误，failed-after-finished 被忽略且不出 outbox，终态冲突语义需确认

- 链路：profit sharing result fact -> `applyProfitSharingFact` -> `applyProfitSharingSuccessFact`/`applyProfitSharingFailedFact` -> outbox。
- 证据点：`logic/payment_fact_application_service.go:854-883` success fact 只接受 processing/finished；当前 order 为 failed 时会返回错误。`applyProfitSharingFailedFact` 遇到 finished 返回空 order nil（同文件 `900-910`），后续 `createPaymentDomainOutboxForAppliedFact` 看到 `ProfitSharingOrder == nil` 会走 refund outbox fallback 并通常不创建分账 outbox。
- 风险条件：provider terminal fact 乱序或纠正时，success-after-failed 会让 application failed/retry；failed-after-finished 会被 application applied 但不产生分账失败通知/告警。
- 现有保护：finished 优先于 failed 可避免收入到账后被回退；金额校验保护 success fact。
- 候选原因：分账终态冲突采取非对称 first-terminal-wins；需与宝付语义和运营告警要求确认。

### 204. Baofu verify fee terminal failure 会重置开户 flow 到 verify_fee_pending，需确认不会覆盖已推进 flow

- 链路：direct payment fact for baofu verify fee -> `applyBaofuVerifyFeePaymentFact` -> terminal closed/failed/expired -> `applyBaofuVerifyFeePaymentTerminalFailure`。
- 证据点：`logic/payment_fact_application_baofu_verify_fee.go:62-132` terminal failure 先把 payment_order closed/failed，再 `GetBaofuAccountOpeningFlowByPaymentOrder`，随后调用 `MarkBaofuAccountOpeningFlowVerifyFeePending` 清空 `VerifyFeePaymentOrderID` 并写 retry guidance。
- 风险条件：verify fee payment_order terminal failure fact 晚到，但开户 flow 已因其它支付/人工恢复推进；需要确认不会错误清空当前支付单或覆盖更新后的 retry guidance。
- 现有保护：已补读 `db/query/baofu_account_opening_flow.sql:86-99`，`MarkBaofuAccountOpeningFlowVerifyFeePending` 带状态 guard，只允许 `profile_pending`/`verify_fee_pending`/`verify_fee_processing` 回到 `verify_fee_pending`，不会从 `opening_processing`/`ready`/`failed` 回退。
- 候选原因：风险已从“可能覆盖已推进 flow”收敛为“同一 verify_fee_processing 窗口内仍需验证 payment_order owner 是否匹配、旧失败事实是否可能清空新的 `verify_fee_payment_order_id`”；需继续查 flow 与 payment_order 绑定条件。

### 205. terminal external payment fact/application 分段创建本身不是永久孤儿窗，但要依赖来源侧重试/恢复，真正残余只剩“源头重试也失效”的窄窗

- 链路：callback/query/recovery -> `RecordExternalPaymentFact`/provider fact helper -> `CreateExternalPaymentFact` -> `CreateExternalPaymentFactApplication` -> scheduler 扫 application。
- 证据点：`logic/payment_fact_service.go:97-145`，`worker/payment_fact_application_scheduler.go:110`，`worker/payment_recovery_scheduler.go:77-122`，`worker/baofu_payment_recovery_scheduler.go:87-124, 201-278`，`worker/refund_recovery_scheduler.go:84-120, 460-560`，`worker/baofu_account_opening_recovery_scheduler.go:89-180`，以及 `api/payment_callback.go:967-1153` / `api/baofu_callback.go:202-262` / `worker/task_payment_timeout.go:205-233` / `worker/task_payment_timeout_baofu.go:81-124` 等入口。
- 风险条件：fact insert 成功后 application insert 失败，且对应来源后续既不会重试也不会被本域 recovery scheduler 再次扫到。
- 现有保护：callback 类入口会返回 FAIL 触发 provider 重试；timeout/query/recovery 类入口本身又有 paid-unprocessed、processing、flow-state 或 refund-state 的恢复扫描；同 dedupe_key fact 冲突时会复用既有 fact。
- 候选原因：application 表不是唯一恢复面，但只靠它确实不够；这里的真实窗更窄，主要取决于每个来源是否还能第二次进入同一 `RecordExternalPaymentFact` 路径。

### 206. 同一 direct payment 或部分 Baofu terminal 结果可由 callback/query/manual recovery 产生多个 fact/application，依赖下游幂等收敛

- 链路：provider callback、timeout query、manual reconciliation/recovery 都可写 external payment fact；随后各自创建 application 并进入同一业务消费逻辑。
- 证据点：`worker/direct_payment_fact.go:14-55` timeout query fact 的 dedupe key 为 `wechat:query:direct:payment:<objectKey>:<terminalStatus>`；`worker/direct_payment_fact.go:83-115` manual reconciliation fact 的 dedupe key 为 `wechat:manual_reconciliation:direct_payment:<businessOwner>:<outTradeNo>:success`；`logic/baofu_profit_sharing_service.go:300-309` Baofu profit sharing callback 使用 `source_event_id`，非 callback/query/recovery 使用 `baofu:<source>:profit_sharing:<outTradeNo>:<secondary-or-upstreamState>`。
- 风险条件：同一个远端终态先被 callback 消费，又被 query/recovery/manual reconciliation 写入不同 source/dedupe_key 的 fact；多个 application 可能重复调用 domain side-effect、outbox、通知、fee ledger 或状态推进。
- 现有保护：业务应用层多处有状态 guard、金额校验或已存在记录判断；application 层自身用 dedupe_key 防同源重复。
- 候选原因：跨来源重复事实是有意的 at-least-once 设计还是潜在重复消费窗口，需要逐 consumer 验证 `ProcessPaymentSuccessTx`、Baofu verify fee、refund/profit-sharing/fee ledger/outbox 的幂等边界。

### 207. 微信直连支付回调先把订单标 paid、再发通知、再记 fact，回调失败与用户可见副作用之间存在先后倒置

- 链路：`handlePaymentNotify` -> `UpdatePaymentOrderToPaid` -> `sendPaymentSuccessNotification` -> `recordDirectPaymentCallbackFact` -> `enqueueDirectPaymentFactApplication` -> `markNotificationProcessed`。
- 证据点：`api/payment_callback.go:1073-1153` 明确先更新 payment_order，再同步发送支付成功通知，然后才写事实并入队；任何后续失败都会 `releaseNotification` 并返回 FAIL，但通知和已改订单不会回滚。
- 风险条件：事实写入或 application 入队失败时，用户可能已经收到成功通知，而回调对微信返回 FAIL；微信重试后又可能进入“已支付订单”分支，造成通知/事实应用时序不一致。
- 现有保护：`UpdatePaymentOrderToPaid` 只允许 pending->paid；后续事实 dedupe/application dedupe 可压住部分重复。
- 候选原因：同步通知被放在事实持久化之前，依赖后续重试/恢复收敛；需验证该顺序是否有意接受“先通知后落事实”的窗口。

### 208. 微信直连支付超时查询在 remote paid 分支先把本地订单标 paid，再记 timeout fact，若后续入队失败会落入 recovery 而不是继续由 timeout task 重放

- 链路：`ProcessTaskPaymentOrderTimeout` -> `prepareDirectPaymentOrderTimeoutClose` -> `handleDirectPaymentTimeoutQueryResult(SUCCESS)` -> `UpdatePaymentOrderToPaid` -> `recordDirectPaymentTimeoutQueryFact` -> `enqueueOrderPaymentFactApplication`。
- 证据点：`worker/task_payment_timeout.go:205-233` 里 remote paid 分支先改本地状态再写事实和入队；`worker/direct_payment_fact.go:14-55` 仅负责 fact/application 创建，不会补回 timeout task；task 的 `default` 分支对 paid 状态直接跳过。
- 风险条件：`UpdatePaymentOrderToPaid` 成功后，fact 创建或 application 入队失败；任务重试再次进入时订单已 paid，超时任务会提前退出，后续恢复完全依赖 `payment_recovery_scheduler` 的 paid-unprocessed 扫描。
- 现有保护：`payment_recovery_scheduler` 会扫描 `status=paid && processed_at is null` 的直连订单并补记 recovered fact。
- 候选原因：超时任务把“状态推进”和“事实补写”拆成两个阶段，第二阶段失败后不会由同一任务继续收敛；需验证恢复扫描是否覆盖所有直连业务 owner 与最坏延迟。

### 209. Baofu 超时查询 remote paid 也先改本地订单，再记 fact 并入队；若入队失败，超时任务本身不会继续重放该事实

- 链路：`prepareBaofuPaymentOrderTimeoutClose` -> `handleBaofuPaymentTimeoutQueryResult(SUCCESS)` -> `RecordPaymentFact` -> `enqueueOrderPaymentFactApplication`。
- 证据点：`worker/task_payment_timeout_baofu.go:81-124` remote paid 分支先构造 fact 并申请 application，再仅在 `recorded.Application != nil` 时入队；任务之后直接返回 stop=true。retry 语义会被 paid/processed 状态和后续恢复扫描改变。
- 风险条件：本地已经推进到 paid，但 application 入队失败；timeout task 重试时不再走相同 query/record 分支，后续只能靠 `payment_recovery_scheduler` 或 fact application scheduler。
- 现有保护：Baofu 侧 `payment_recovery_scheduler` 会扫描 pending/paid 的相关订单并补 query fact；事实应用调度器会扫 pending application。
- 候选原因：Baofu timeout 远端成功与本地落事实之间不是单事务，失败后依赖两个不同 scheduler 补齐；需验证这两个 scheduler 的覆盖窗口是否足够。

### 210. 微信/宝付 query fact 分支会先改本地订单，再决定是否把 fact 应用直接执行，若应用被 defer 或 skipped，订单和事实投影可能短暂分离

- 链路：`recordAndApplyDirectPaymentQueryFact` / `recordAndApplyBaofuAggregatePaymentQueryFact` -> `UpdatePaymentOrderToPaid`（部分分支）-> `RecordExternalPaymentFact` -> `ApplyExternalPaymentFactApplication` 或 deferred/skip。
- 证据点：`logic/payment_order_query_wechat.go:304-389` 里 direct baofu verify fee success 先可能更新订单，再记录 fact；若 `shouldDeferDirectPaymentQueryFactApplication` 命中，`factResult.Application != nil` 也会直接返回不应用。Baofu aggregate query 则在 fact/applications 创建后立即应用，但 success amount mismatch 会提前跳过本地 paid 迁移。
- 风险条件：同一 query 请求里订单状态与事实 application 的推进不是原子步骤，某些业务（特别是 Baofu verify fee）会故意延后 application；若中间失败，订单可能已经 paid 但事实仍待应用。
- 现有保护：`payment_recovery_scheduler` 和 `payment_fact_application_scheduler` 分别覆盖 paid-unprocessed 与 retryable application。
- 候选原因：查询链路把“本地状态投影”和“业务应用”拆开以兼容 verify fee/onboarding 续链；需要确认延后应用的窗口不会让下游再次触发冲突状态。

### 211. 微信通知认领/释放采用删行而不是状态回退，应用成功后只写 `processed_at`，若处理前后失败与重复回调交错可能出现“已处理但又可重试”的观察窗口

- 链路：`tryClaimNotification` -> `handleDuplicateClaimedNotification` / `releaseNotificationWithReason` -> 业务处理 -> `markNotificationProcessed`。
- 证据点：`api/payment_callback.go:472-539` 认领用 `INSERT ... ON CONFLICT DO NOTHING`，释放直接 `DELETE FROM wechat_notifications`，成功只 `UPDATE processed_at`；`wechat_notification.sql` 只有 `processed_at IS NULL` 的 stale 扫描，没有单独的 claim 状态列。
- 风险条件：处理成功前后若回调进程崩溃、`markNotificationProcessed` 失败或 release/processed 交错，重复通知会表现为“已认领但未 processed”或“已删除后可重新认领”的窗口；重试和人工排查要依赖这几个动作的先后。
- 现有保护：重复认领会走已处理/在处理/过期释放路径；stale 扫描能把长期未 processed 的占位释放出来。
- 候选原因：通知幂等是靠行存在性和 processed_at 组合表达的，而不是独立状态机；需验证 `release` 与 `markProcessed` 的失败语义是否符合微信重试期望。

### 212. 宝付提现命令派发先把 command 认领成 unknown/accepted/rejected，再更新 withdrawal_order，若更新时间失败会留下命令和订单两边不同步的中间态

- 链路：`ProcessTaskBaofuWithdrawalCommandDispatch` -> `ClaimSubmittedExternalPaymentCommandForDispatch` -> 调宝付创建提现 -> `UpdateBaofuWithdrawalOrderToProcessing/Status` -> `recordBaofuWithdrawalCommandOutcome`。
- 证据点：`worker/task_baofu_withdrawal_command_dispatch.go:130-209` 先 claim command，再在 accepted/rejected/unknown 分支更新 withdrawal_order；`baofu_withdrawal_order.sql:67-88` 的状态更新仍要求 `status='processing'`。unknown outcome 还依赖 `repairClaimedBaofuWithdrawalCommandOutcome` 从 withdrawal order 回补命令结果。
- 风险条件：命令已 claim 成 unknown/accepted，但 withdrawal_order 更新失败；或反过来订单已更新、command outcome 写失败。恢复调度器会再扫 submitted command/processing order，但两个表可能在一段时间里显示不同步。
- 现有保护：`repairClaimedBaofuWithdrawalCommandOutcome` 会从订单状态回补 command outcome；恢复调度器同时扫 submitted 命令和 processing 订单。
- 候选原因：提现链路把“命令审计”和“订单状态”拆成两条投影，靠 recovery 线程做最终一致；需验证未知态/accepted 态是否会在长时间抖动下重复派发。

### 213. 宝付提现 recovery 同时扫 submitted command 和 processing order，可能在同一轮里对同一提现单形成双路径修复，依赖 command unique/claim 互斥

- 链路：`BaofuWithdrawalRecoveryScheduler.runOnce` -> `enqueueSubmittedCommands` -> `queryAndEnqueue`。
- 证据点：`worker/baofu_withdrawal_recovery_scheduler.go:117-182` 一轮里先补提交命令，再对 processing 订单查状态并入队；`task_baofu_withdrawal_command_dispatch.go:43-71` 有 30 分钟 unique TTL，但实际上命令调度/恢复/重试可能跨多轮。订单和命令分别由不同 task 处理。
- 风险条件：同一提现单在 command 仍 submitted、order 又已 processing/unknown 时，恢复器可能同时补命令派发与状态查询；若 command claim 和 order status 更新顺序反复抖动，可能重复触发 provider 查询或 outcome 修复。
- 现有保护：command claim、状态机 expected-state、以及 unique TTL 限制了一部分重复；处理函数对 terminal order/status 也会早退。
- 候选原因：提现恢复是 command-first + order-first 的双路径补偿，设计上允许短时间重复调度；需要验证 provider 侧和命令审计是否完全幂等。

### 214. 云打印任务/状态轮询对同一 `print_log` 只按 id/status 更新，终态晚到可能覆盖早到结果，且 provider status poll 与业务打印成功分支各自独立

- 链路：`ProcessTaskPrintOrder` -> `executePrintAttempt*` -> `UpdatePrintLogStatus`；另一路 `CloudPrinterStatusPollScheduler` -> `ClaimPendingProviderStatusPrintLogs` -> `MarkProviderStatusPrintLogTerminal` / `RecordProviderStatusPollError` / `ExpireProviderStatusPrintLogs`。
- 证据点：`db/query/print_log.sql:173-238` 的 `UpdatePrintLogStatus` 和 `MarkProviderStatusPrintLogTerminal` 只按 id 或 `status='pending'` 更新，没有额外 expected-state；`ExpireProviderStatusPrintLogs` 也会把仍 pending 的记录直接改成 failed。`worker/cloud_printer_status_poll_scheduler.go:189-257` 会把 provider 查询结果 success/failed/timeout/cancelled 直接写回。
- 风险条件：业务打印已成功但 provider 轮询晚到 failure/timeout，或反过来 pending 记录后到 success；多个打印重试记录还可能让“最新一条”与“历史一条”状态同时存在。
- 现有保护：`ListTimedOutPrintAnomalies`/`CountMerchantPrintAnomalies` 用最新记录视角，`GetPrintLogByTaskKeyAndPrinter` 能压住同任务的重复重试记录。
- 候选原因：打印日志是历史记录 + 最新记录双轨模型，而不是单一状态机；需要验证运营视图和恢复逻辑是否都按“最新有效记录”读取。

### 215. 云打印 provider status poll 用 `processed_at/created_at` 时间窗推进，可能把稍晚写入的 pending log 推到下一轮或直接过期

- 链路：`ClaimPendingProviderStatusPrintLogs` -> provider query -> `MarkProviderStatusPrintLogTerminal`/`RecordProviderStatusPollError`/`ExpireProviderStatusPrintLogs`。
- 证据点：`worker/cloud_printer_status_poll_scheduler.go:111-143` 按 `created_at <= ready_before`、`created_at >= maxAge`、`provider_status_checked_at <= now-interval` 选取；超龄先 expire，再 claim pending。轮询本身不锁业务打印线程。
- 风险条件：打印回调/状态写入稍晚，刚好跨过 `initialDelay` 或 `maxAge` 窗口；同一任务可能被轮询多次，或者直接被过期失败，和外部 provider 最终 success 顺序错位。
- 现有保护：多轮 claim + provider query at-least-once；过期只作用于仍 pending 的记录。
- 候选原因：轮询器依赖时间窗而非“事件已确认”标记，需要验证 `initialDelay/maxAge` 对不同 provider 的实际时延是否足够宽。

### 216. 媒体审核回调先改资产状态，再补派发 OCR；派发失败会留下资产与 OCR 任务半状态

- 链路：`handleMiniProgramMediaCheckNotify` 先 `SetMediaAssetModerationStatusByTraceID`，再 `processPendingOCRJobsForMediaModeration`；approved 分支只 enqueue pending OCR job，非 approved 分支则把 pending job 直接标 failed。
- 证据点：`api/media_moderation.go:314-343`，`api/ocr.go:918-934`，`api/ocr.go:1105-1138`，`worker/ocr_retry.go:14-21`。
- 风险条件：callback 已把 `media_assets.moderation_status` 推到终态，但后续 OCR job enqueue 失败或 callback 返回 500；approved 媒体下 pending OCR job 仍停在 pending，没有像 createOCRJob 主路径那样落 dispatch failed。
- 现有保护：`createOCRJob` 主路径在 enqueue 失败时会 `markOCRJobDispatchFailed`；`asynq.Unique(12m)` 只压住重复入队，不补状态。
- 候选原因：媒体审核状态和 OCR 任务派发不是同一事务，回调链路失败后只能依赖后续重复回调/人工补偿，OCR 任务与媒体状态可能长期不同步。

### 217. 商户主体资料投影是三段直写，且 API/worker 投影失败被吞掉

- 链路：`tryProjectMerchantSubjectProfile` / `syncMerchantSubjectProfile` -> `SaveApplicationProfile` -> `DetachMerchantSubjectProfileMerchantFromOtherApplications` -> `UpsertMerchantSubjectProfile` -> `CreateMerchantSubjectProfileVersion`。
- 证据点：`logic/merchant_subject_profile_service.go:292-348`，`api/merchant_application.go:1663-1679`，`worker/task_merchant_application_ocr.go:447-464`，`logic/merchant_onboarding_review_service.go:217-247`。
- 风险条件：detach 已提交但 upsert/version 失败；或 projection 调用本身失败后只记 warn。这里没有 service 级事务，detach/upsert/version 是分段写，可能出现 merchant 先被从旧 profile 脱挂，再卡在没有新版本快照的中间态。
- 现有保护：审批主交易里的 `ApproveMerchantApplicationTx` 自身是原子的，但 OCR 修正、信息编辑、审核修复这些投影入口都不在同一个事务边界。
- 候选原因：商户主真值、应用表、主体资料表、版本审计表是分离写入的，投影失败时没有独立补偿/重放边界，容易形成长期漂移。

### 218. 商户/骑手入驻审核的 approval tx 与后续 restore/通知分离

- 链路：merchant `ProcessSubmittedApplication` 先跑 `ApproveMerchantApplicationTx`，再做 review run summary、`activateMissingMerchantCredentials`、`RestoreMerchantIfEligible`；rider 侧 `ApproveRiderApplicationWithReviewTx` 先把审批、review run、credential activation 落库，再在事务外做 `RestoreRiderIfEligible`。
- 证据点：`logic/merchant_onboarding_review_service.go:143-196`，`logic/merchant_onboarding_review_service.go:199-255`，`logic/rider_onboarding_review_service.go:102-199`，`locallife/db/sqlc/tx_merchant_application.go:52-160`，`locallife/db/sqlc/tx_rider_application.go:63-105`，`logic/credential_governance_service.go:95-134`。
- 风险条件：approval 事务已经提交，但 review summary 更新、credential activation/restore，或后续通知失败。merchant/rider 都会留下“已批准但治理状态未完全恢复”的半状态；merchant 还额外可能出现 review summary 与实际 run 状态不同步。
- 现有保护：approval 核心状态写入在各自 tx 内；credential activation tx 也有自己的原子边界。
- 候选原因：审批、审计摘要、资质激活、资质恢复被拆成多个可失败阶段，失败后没有统一回滚，也没有单一补偿任务把它们收敛成同一终态。

### 219. 宝付商户报备初始提交先写 command 再 upsert report，report 写入失败只会留下审计残留（对应 NP-7）

- 链路：`SubmitWechatMerchantReport` 先 `CreateExternalPaymentCommand(submitted)`，再 `UpsertBaofuMerchantReportProcessing`，最后才外呼 `SubmitWechatReport`。
- 证据点：`logic/baofu_merchant_report_service.go:98-125`；`worker/baofu_merchant_report_recovery_scheduler.go:108-123` 只扫 `baofu_merchant_reports`；`db/query/external_payment_fact.sql:1-144` 只提供命令单行查询/回放，不存在商户报备 command 的独立 recovery 扫描。
- 风险条件：command 已写但 report upsert 失败、进程退出或上下文中断。
- 现有保护：provider 外呼尚未发生，report recovery 不依赖这条 command；因此这里只是审计残留，不构成业务外部副作用。
- 候选原因：已归入 NP-7，不再作为业务时序风险单独追踪。

### 220. 宝付开户回调缺 out_request_no 时会回落到 contract_no -> latest flow，旧回调晚到可能打到新 flow

- 链路：开户回调优先按 `out_request_no` 查 flow；缺失时按 `contract_no` 找 binding，再取该主体 `GetLatestBaofuAccountOpeningFlowByOwner`，最后把回调 fact / flow state 应用到这条 latest flow。
- 证据点：`api/baofu_account_open_callback_fact.go:120-155`；`db/sqlc/baofu_account_opening_flow.sql.go:86-108`；`logic/baofu_account_onboarding_apply.go:412-447`。`resolveBaofuAccountOpeningFlowForCallback` 在缺少流水号时没有按 flow 创建时间/当前 active 状态约束，只按 owner 最新 flow 兜底。
- 风险条件：旧 flow 的回调丢失 `out_request_no` 或 provider 仅返回 `contract_no`，而用户已重新发起新 flow；晚到的旧回调可能被匹配到最新 flow，尤其当新旧 flow 共享同一 contract/binding 语义时。
- 现有保护：`ensureAccountOpenResultSerialMatchesFlow` 会在有流水号时阻断 mismatch；`ensureAccountOpenContractMatchesFlow` 会校验绑定归属和 opening mode；`persistUnmatchedBaofuAccountCallbackAlert` 会记录未匹配告警。
- 候选原因：fallback 的目标是修复 provider 缺号回调，但它仍把“latest flow”当作归属锚点，而不是“与回调事实时间或绑定的那条 flow”；因此它是一个真实的时序窗，虽然有 contract/binding 校验，但仍可能把旧事实压到新 active flow 上。

### 221. 宝付商户报备恢复没有 per-owner claim，report 未落库时并发恢复可双重提交并互相覆盖 report_no

- 链路：商户开户回调续接 / 开户恢复 scheduler -> `RecoverMerchantReportFlow` -> `GetBaofuMerchantReportByOwner` 未命中 -> `submitMerchantReport` -> `SubmitWechatMerchantReport` -> `CreateExternalPaymentCommand(submitted)` -> `UpsertBaofuMerchantReportProcessing` -> `SubmitWechatReport`。
- 证据点：`logic/baofu_account_merchant_report_service.go:68-110,141-151`，`logic/baofu_merchant_report_service.go:79-125`，`worker/baofu_account_opening_recovery_scheduler.go:160-180`，`api/baofu_account_open_callback_fact.go:227-253`。
- 风险条件：同一 merchant flow 在 `baofu_merchant_reports` 还不存在时，被 callback continuation 和 recovery scheduler（或两个恢复调用方）几乎同时触发；双方都看到 `GetBaofuMerchantReportByOwner` 不存在，于是各自生成不同 `report_no` 并发起外呼。后写入的 upsert 会重置当前 report_no，而前一个外呼可能已经被 provider 受理。
- 现有保护：`baofu_merchant_reports` 对 `(owner_type, owner_id, report_type)` 有唯一行，但没有 claim/lock 保证只提交一次；scheduler 的 `runMu` 只能防同实例并发，挡不住 callback 与 scheduler 互撞。
- 候选原因：报备恢复把“是否需要提交”放在纯读判断里，submit 又会刷新 `report_no`，所以它是典型的 read-then-create race，不是单纯的幂等重试。

### 222. 宝付开户流程创建是 read-then-insert，active_owner 唯一索引能挡重复落库但挡不住并发提交的瞬时失败

- 链路：`openingFromInput` -> `GetActiveBaofuAccountOpeningFlowByOwner` 未命中 -> `getOrCreateFlowWithExisting` -> `createBaofuAccountOpeningFlow` -> `CreateBaofuAccountOpeningFlow`，而新 flow 初始状态 `profile_pending` 已被 active_owner 唯一索引覆盖。
- 证据点：`logic/baofu_account_onboarding_service.go:219-253`，`logic/baofu_account_onboarding_profile.go:168-191`，`logic/baofu_account_onboarding_profile.go:206-215`，`locallife/db/sqlc/baofu_account_opening_flow.sql.go:86-108`，`locallife/db/migration/000233_add_baofu_account_opening_flows.up.sql:112-121`。
- 风险条件：同一 owner 的两个 `PrepareOpening` / `StartOrRecoverOpening` 请求并发到达，双方都先读到“没有 active flow”，随后同时尝试插入新 flow。数据库唯一索引会让后到者失败，但代码没有把这个冲突显式收敛成“复用已有 flow”。
- 现有保护：active_owner 唯一索引可以防止双活 flow 真正落库；但它只是在数据库层拒绝，当前读后写路径没有乐观重试或冲突回读。
- 候选原因：这是开户入口上典型的读写竞态，用户快速重复提交或 API 重试时可能表现为偶发 500/冲突，而不是平滑复用同一条 flow。

### 223. 宝付开户 `opening_processing` 没有 claim 字段，两个执行者可能并发打出同一笔 `OpenAccount`

- 链路：`PrepareOpeningAfterVerifyFeePaid` / `ExecutePreparedOpening` / `ProcessTaskBaofuAccountOpening` / `RecoverOpeningFlow` 都会在 `state='opening_processing'` 时进入 `openFromPreparedProfile`，而 `openFromPreparedProfile` 直接调用 `accountClient.OpenAccount`，没有单独的 claimed/dispatching 记录。
- 证据点：`logic/baofu_account_onboarding_prepare.go:68-104`，`worker/task_baofu_account_opening.go:67-91`，`logic/baofu_account_onboarding_recovery.go:6-28`，`logic/baofu_account_onboarding_open.go:62-112`。`MarkBaofuAccountOpeningFlowOpeningProcessing` 只是把 flow 置为 opening_processing，并不区分“已准备待执行”与“已被某个 worker 认领执行”。
- 风险条件：一个 worker 已把 flow 置为 opening_processing，但在它真正写回 active binding / failed 之前，另一个 worker、重入任务或恢复调度器也读到同一状态并同步发起 `OpenAccount`。这是典型的 read-then-act 竞态，外部 provider 若非严格按 `open_trans` 幂等，会出现重复开户请求。
- 现有保护：`asynq.Unique(30m)` 只能压住同一队列的重复入队，挡不住 worker 与 recovery 的交错；`activeBindingForPreparedFlow` 能在“已激活绑定先落库”后早退，但挡不住两边同时还没看到绑定时的双外呼。
- 候选原因：这里缺的是一个持久化的 dispatch claim 或 provider-call-in-progress 标记，而不是 flow 状态本身。`opening_processing` 同时承担“待执行”和“正在执行”的语义，窗口就留在了两个执行者并发起跑的那一瞬间。

### 224. 会员余额取消回滚依赖订单取消状态 guard，核心资金事务较完整但历史重复流水仍需数据确认

- 链路：订单创建使用会员余额 -> `CreateOrderTx` 锁会员并扣余额/写 consume 流水；取消订单 -> `CancelOrderTx` 在同一事务里回滚券、会员余额、库存等副作用。
- 证据点：`db/sqlc/tx_create_order.go:148-261`，`db/sqlc/tx_create_order.go:268-281`，`db/sqlc/tx_order_status.go:292-347`，`db/query/membership.sql:50-56`。
- 风险条件：同一订单取消与其它状态推进并发时，输掉方由订单状态 guard 回滚；但如果历史上同一订单已有多条 membership consume 流水，当前回滚只取 `GetMembershipConsumeByOrder` 的最新一条来拆 principal/bonus。
- 现有保护：订单取消自身有 expected old status；会员行用 `FOR UPDATE` 串行化；余额回滚和退款流水与订单取消在同一事务中提交；新订单创建路径一般只会为一个 order 写一条 consume。
- 候选原因：当前主链路不是明显时序洞，更像历史数据质量风险；后续若排资金差异，可查同一 `(membership_id, related_order_id, type='consume')` 是否存在多行。

### 225. 资质过期暂停先 claim profile 再标 ledger suspended，失败会留下 profile/ledger 半状态

- 链路：`DataCleanupScheduler.enforceExpiredCredentials` 扫 expired active ledger -> `ClaimMerchantTakeoutSuspensionIfAvailable` / `ClaimRiderSuspensionIfAvailable` -> `MarkCredentialLedgerSuspended` -> 发通知/平台告警。
- 证据点：`scheduler/data_cleanup.go:960-1040`；`db/query/trust_score.sql:75-97`、`:183-205`；`db/query/credential_ledger.sql:145-154`。
- 风险条件：claim profile 成功后，`MarkCredentialLedgerSuspended` 失败、超时或进程退出；外卖/接单已暂停，但 `credential_ledgers.suspended_at` 未写。
- 现有保护：下一轮扫描仍会看到 expired active ledger；由于 profile suspend reason 仍是 `document_expired`，claim SQL 允许同 reason 再次 claim 并重试 mark ledger。restore 使用 reason-owned release，避免释放其它原因暂停。
- 候选原因：这不是永久性资金类洞，但会造成短期甚至多轮的状态展示/通知不一致；如果 mark ledger 长期失败，前端 active credential summary 可能看不到 suspended_at，而业务 profile 已暂停。

### 226. 资质到期提醒先发送通知再标 `last_reminded_at`，标记失败会重复提醒

- 链路：`remindExpiringCredentialsAt` 扫 7 天窗口 -> 发送通知任务或直接写通知 -> `MarkCredentialLedgerReminderSent`。
- 证据点：`scheduler/data_cleanup.go:880-930`；`db/query/credential_ledger.sql:136-143`。
- 风险条件：通知已经成功入队/落库后，更新 `last_reminded_at` 失败；下一轮仍会命中同一 ledger 并再次发送提醒。
- 现有保护：重复提醒影响有限，且比漏提醒更可接受；`shouldSkipCredentialReminder` 与 SQL 窗口会过滤已成功标记的 ledger。
- 候选原因：通知副作用和 ledger 标记不是同一 outbox/事务；需要运营上接受 at-least-once 提醒，或后续用通知业务幂等键约束。

### 227. 资质恢复释放与新 ledger 激活分阶段，恢复通知仍是 best-effort

- 链路：新申请审核通过 -> activate credential ledgers -> `RestoreMerchantIfEligible`/`RestoreRiderIfEligible` -> reason-owned release profile suspension -> mark active ledgers resumed -> 发送恢复通知。
- 证据点：`logic/credential_governance_service.go:95-147`；`db/sqlc/tx_credential_ledger.go:135-177`；`worker/task_onboarding_review.go:126-196`；`api/rider_application_submit.go:126-132`。
- 风险条件：新 ledger 已激活但 restore transaction 失败；或 restore 已释放但通知入队失败。用户可能短期仍被暂停，或已恢复但未收到通知。
- 现有保护：release 使用 `Release*SuspensionIfOwned`，只释放 `document_expired` 自己持有的暂停，不覆盖 food-safety/追偿等其它暂停；restore tx 内 profile release 和 ledger resumed 是同事务；通知失败不影响业务恢复。
- 候选原因：业务状态恢复与通知仍分离，属于可观测性/用户感知风险；若 restore tx 失败，需要确认提交复审后的同步/异步重试入口是否足够。

### 228. 索赔平台赔付先标 running/out_bill_no 再外呼，外呼前崩溃会进入 running+NOT_FOUND 悬挂

- 链路：`ExecuteClaimPayoutAction` 读取 payout behavior action -> 生成 `claimpayout{actionID}` -> `UpdateBehaviorActionExecution(status='running', detail.out_bill_no=...)` -> `CreateTransfer` -> `QueryTransferByOutBillNo`；恢复调度器每 5 分钟扫描 `created/failed/running` payout action。
- 证据点：`worker/task_claim_refund.go:121-183`、`:398-419`；`worker/claim_refund_recovery_scheduler.go:85-103`；`db/query/behavior_trace.sql:237-252`；测试 `TestProcessTaskClaimPayout_RunningActionQueryNotFoundKeepsActionRunning` 明确覆盖了 404 后保持 running。
- 风险条件：本地已把 action 标成 running 并写入 out_bill_no，但进程在真正调用微信 `CreateTransfer` 前退出，或请求未被微信创建但本地无法区分。下一轮恢复看到 running+out_bill_no 只会查询；微信返回 404/NOT_FOUND 时继续保持 running，不清空 out_bill_no，也不重新 create。
- 现有保护：out_bill_no 由 action id 确定；若微信已受理但本地返回异常，后续重复 create 的 duplicate 分支能转查询；running action 会被恢复调度器持续扫描。
- 候选原因：`running` 同时表示“本地准备外呼”和“provider 已有可查询单据”，而 NOT_FOUND 被当作“继续等待”，没有区分“尚未外呼成功创建”的窗口。

### 229. 索赔平台赔付 action 没有持久化 claim/lease，重复执行依赖微信 out_bill_no 幂等吸收

- 链路：API 或补偿 tx 创建 payout behavior action -> asynq 任务执行；恢复调度器也会扫 `created/failed/running` 并直接调用同一个 `ExecuteClaimPayoutAction`。
- 证据点：`worker/task_claim_refund.go:75-183` 使用普通 `UpdateBehaviorActionExecution` 标 running；`db/query/behavior_trace.sql:237-252` 同时存在 `UpdateBehaviorActionExecutionIfCurrent` 但 payout 未使用；`worker/claim_refund_recovery_scheduler.go:85-103` 会重入执行。
- 风险条件：同一 payout action 被 asynq 重复投递、人工重试、或恢复调度器与正在执行的任务交错；两个执行者都读到非 success 状态后，均可无条件写 running 并同时调用 `CreateTransfer`。
- 现有保护：商户单号固定为 `claimpayout{actionID}`；duplicate 错误会被识别并走查询；成功后 action success 会使后续早退。
- 候选原因：缺少类似普通 behavior action 的 `UpdateBehaviorActionExecutionIfCurrent` 抢占边界；当前正确性强依赖微信按 out_bill_no 严格幂等及 duplicate 错误映射稳定，仍应作为外部副作用并发窗口单独深挖。

### 230. 骑手押金提现的活跃配送校验在事务外，提现冻结后仍可能与新接单时序交错

- 链路：`SubmitWithdrawal` 先读 rider 与 pending refund，再在 `rider.FrozenDeposit == 0` 时调用 `ListRiderActiveDeliveries`；随后才进入 `PrepareRiderDepositRefundTx` 锁 rider/幂等请求/credit/source payment order 并冻结押金。
- 证据点：`logic/rider_deposit_refund_service.go:89-119`；`db/sqlc/tx_rider_refund.go:67-255`；接单事务 `db/sqlc/tx_delivery.go:203-261` 也锁 rider 并按 `DepositAmount - FrozenDeposit` 校验可用押金。
- 风险条件：预检时没有 active delivery，但提现事务完成后仍允许骑手基于剩余可用押金接单；或者历史半状态下 active delivery 未正确冻结押金时，事务内不会再复核 active delivery 集合。若业务规则是“发起/处理中提现期间不得产生活跃配送”，当前只有事务外快照与资金冻结间接约束，缺少持久化的 delivery-eligibility guard。
- 现有保护：提现准备事务与接单事务都锁 rider；接单会按最新 `FrozenDeposit` 计算可用押金，因此一般不会造成资金透支。若并发接单先完成并冻结押金，提现事务会在锁 rider 后因 `FrozenDeposit > 0` 拒绝。
- 候选原因：这是业务资格时序窗，不是已确认的资金超扣；核心待查是产品是否允许“部分提现处理中仍可接单”，以及接单路径是否应感知 pending withdrawal，而不是只感知余额。

### 231. 骑手押金退款创建返回终态时仍先落 `processing`，终态结算依赖回调或 15 分钟 stuck query

- 链路：提现准备后逐笔调用微信直连 `CreateRefund`；无论微信响应 `SUCCESS`、`CLOSED` 还是 `ABNORMAL`，当前分支都先 `UpdateRefundOrderToProcessing` 并记录 accepted command，没有立即执行 `ResolveRiderDepositRefundTx` 的成功结算或失败解冻。
- 证据点：`logic/rider_deposit_refund_service.go:166-248`；`db/query/refund_order.sql:150-178`；`worker/refund_recovery_scheduler.go:248-342` 只扫描超过 15 分钟的 `processing` refund 再查询 provider；测试 `TestRiderDepositRefundService_SubmitWithdrawal_CreateRefundTerminalResponseReturnsAccepted` 覆盖终态 create response 被视为 accepted/processing。
- 风险条件：provider create response 已给出终态，但回调延迟或丢失；本地押金冻结、credit 已消耗、refund_order 停在 processing，直到 stuck recovery 查询后才结算/解冻。期间用户看到的押金状态与外部退款真实状态不一致。
- 现有保护：直连退款回调会记录 rider_deposit refund fact 并通过 payment fact application 应用；stuck processing scheduler 会查询 provider 并补建 fact；`ResolveRiderDepositRefundTx` 对 success/failed/closed 有锁内资金结算/解冻。
- 候选原因：这是有恢复路径的短中期半状态，不一定是 bug；但它把 create response 的终态信息降级成 processing，恢复 SLA 依赖回调和 15 分钟查询窗口，适合后续按产品体验和资金展示要求确认。

### 232. 骑手押金退款回调解析本地对象失败时会降级到旧 refund result 队列，可能 ACK 后不产生 rider_deposit fact application

- 链路：退款回调验签/解密/归属校验后，`resolveRiderDepositRefundFactObjects` 先按 `out_refund_no` 查本地 refund_order/payment_order；若查询报错或未命中，返回 `shouldRecordFact=false,nil`。handler 随后把回调投到旧 `TaskProcessRefundResult` 队列并 ACK 微信。
- 证据点：`api/payment_callback.go:273-293`、`:1285-1340`；旧 worker 在识别到 `paymentOrder.BusinessType == "rider_deposit"` 时返回 `asynq.SkipRetry`，要求 rider deposit refund 必须走 payment fact application：`worker/task_process_payment.go:768-775`。
- 风险条件：回调到达时本地查询发生瞬时 DB 错误、读延迟、refund_order 尚未可见，或者本地对象短暂缺失；旧队列任务一旦成功入队，handler 会 `markNotificationProcessed` 并向微信返回成功。后续旧 worker 若能查到 rider_deposit refund，会 SkipRetry，不会补建 `rider_deposit_domain/refund_order` application。
- 现有保护：如果本地对象解析成功，会走新的 fact/application；如果 enqueue 旧队列失败，会 release notification 等待微信重试；`processing` refund 还有 stuck query 恢复。但 pending refund 或查询恢复未覆盖的状态仍可能失去这次回调事实。
- 候选原因：`resolveRiderDepositRefundFactObjects` 把“无法判断是不是 rider_deposit”当成“不是 rider_deposit”处理，导致新旧消费链路的 fallback 方向偏乐观；这是一条明确的弱顺序/降级隐患。

### 233. 骑手押金金额异常退款使用 `amount_mismatch`，但后续回调会按 payment_order business_type 进入押金提现结算器

- 链路：直连支付金额不匹配时，若 `paymentOrder.BusinessType == "rider_deposit"`，handler 创建 `refund_type='amount_mismatch'` 的 refund_order，并投递 `TaskProcessRefund`；worker 的 `processRiderDepositMismatchRefund` 发起微信退款。
- 证据点：`api/payment_callback.go:950-1069`、`:404-455`；`worker/task_process_payment.go:886-888`、`:1131-1218`；退款回调分流只看 `paymentOrder.BusinessType == rider_deposit`，不看 `refund_order.refund_type`：`api/payment_callback.go:273-293`、`:1295-1308`；fact application 最终调用 `ResolveRiderDepositRefundTx`：`logic/payment_fact_application_service.go:464-488`。
- 风险条件：金额异常押金退款 create 返回 `PROCESSING`，后续靠退款回调收敛；此时 refund_order 不是普通押金提现冻结单，通常没有对应 rider_deposit_credit/frozen deposit 语义。回调却会走 `ResolveRiderDepositRefundTx`，可能在 `GetRiderDepositCreditByPaymentOrderID` 或 frozen deposit 校验处失败，导致 fact application 反复失败。
- 现有保护：如果 create refund 直接返回 `SUCCESS`，旧 worker 会先把 refund_order 标 success 并尝试 `maybeMarkPaymentOrderRefunded`；后续回调如果再到达，`ResolveRiderDepositRefundTx` 在 refund_order 已 success 时会早退，风险收窄。测试 `TestProcessTaskInitiateRefund_RiderDepositMismatchRefund` 覆盖的是同步 success 路径。
- 候选原因：同属 rider_deposit payment_order 的两类退款语义混在同一回调路由上：押金提现退款需要解冻/扣减 rider deposit，金额异常退款只应退款 payment_order，不应套用提现结算事务。

### 234. 骑手押金退款成功后的 `payment_order.refunded` 写在结算事务外，普通路径失败会被吞掉

- 链路：`ResolveRiderDepositRefundTx` 先在事务内把 refund_order、rider deposit/frozen deposit、credit、押金流水结算；事务返回后 `resolveRefund` 再调用 `maybeMarkPaymentOrderRefunded` 更新 payment_order。
- 证据点：`db/sqlc/tx_rider_refund.go:328-520`；`logic/rider_deposit_refund_service.go:293-321`、`:472-482`；`db/query/payment_order.sql:176-181` 的 `UpdatePaymentOrderToRefunded` 无 status guard。
- 风险条件：事务内押金已经成功扣减/解冻并写成功流水，但事务外统计成功退款或更新 payment_order 失败。普通非 stale-credit 路径里 `maybeMarkPaymentOrderRefunded` 只打日志不返回错误，payment fact application 仍可能继续标 applied，后续没有同一 application 的必然重试。
- 现有保护：如果后续还有退款事实重放、stuck query 或人工触发，`maybeMarkPaymentOrderRefunded` 可能再次执行；`ListRiderDepositLedgerAnomalies` 已有部分押金 ledger 异常查询；stale-credit reconciliation 分支对 `UpdatePaymentOrderToRefunded` 会返回错误并促使 application retry。
- 候选原因：这是典型的 domain settlement 与 payment_order 投影分段提交；核心资金 ledger 较强，但支付单状态投影可能长期落后，且 `UpdatePaymentOrderToRefunded` 本身无旧状态条件会叠加既有候选 199 的覆盖风险。

### 235. 宝付提现 command 被 claim 成 `unknown` 后，外呼前崩溃或创建结果不确定会脱离 submitted 派发扫描

- 链路：提现创建只落 `baofu_withdrawal_orders(status='processing')` 与 `external_payment_commands(status='submitted')`；recovery 扫 submitted command 入队；dispatch worker 先把 command `submitted -> unknown`，再调用宝付 `CreateWithdraw`。
- 证据点：`logic/baofu_withdraw_service.go:175-206`；`db/sqlc/tx_baofu_withdrawal.go:23-59`；`worker/baofu_withdrawal_recovery_scheduler.go:123-142`；`db/query/external_payment_fact.sql:142-154` 只扫描 `command_status='submitted'`；`worker/task_baofu_withdrawal_command_dispatch.go:130-155` 先 claim unknown 再外呼；`:112-114` 遇到 unknown 只做 outcome repair。
- 风险条件：worker 在 claim 成 unknown 后、外呼前崩溃；或外呼返回系统错误/网络不确定，代码把 command outcome 继续记录为 unknown 且不改 withdrawal_order。之后 submitted scanner 不再覆盖该 command，同一个 task 重试时也进入 unknown repair 分支，不重新 `CreateWithdraw`。
- 现有保护：`baofu_withdrawal_orders(status='processing')` 会被 recovery 每 5 分钟查询 provider；如果 provider 实际已创建提现单，query 或 callback 仍可推进 withdrawal_order 终态。
- 候选原因：`unknown` 同时表示“本地已抢占准备外呼”和“provider 创建结果未知”。当 provider 根本没有创建单据或 query 返回不存在/错误时，本地没有“claim 超时后重新外呼”的路径，可能留下 processing withdrawal + unknown command。

### 236. 宝付提现 provider 已受理但本地 order 更新失败时，command outcome 修复依赖后续查询，可能长期 unknown

- 链路：dispatch worker 收到 provider processing/failed 响应后，先更新 `baofu_withdrawal_orders`，再写 external command accepted/rejected outcome。
- 证据点：`worker/task_baofu_withdrawal_command_dispatch.go:167-197`；测试 `TestProcessTaskBaofuWithdrawalCommandDispatchDoesNotAcceptCommandBeforeOrderUpdate` 与 `...DoesNotRejectCommandBeforeOrderUpdate` 明确要求 order 更新失败时不写 command outcome；unknown command repair 只从本地 order 的 `baofu_withdraw_no/raw_snapshot` 推断 outcome：`worker/task_baofu_withdrawal_command_dispatch.go:266-314`。
- 风险条件：provider 已经返回受理/拒绝，但 `UpdateBaofuWithdrawalOrderToProcessing` 或 `UpdateBaofuWithdrawalOrderStatus` 失败。command 留在 unknown；asynq 重试读取 unknown command 后只尝试从 order 修复，而 order 没有成功写入 provider 单号/状态时 repair no-op 并返回 nil。
- 现有保护：processing withdrawal recovery 可按 `out_request_no` 查询 provider，若查到结果可更新 withdrawal_order；若 recovery 后 order 带上 `baofu_withdraw_no/raw_snapshot`，人工或任务重放 command dispatch 才可能修复 command outcome。
- 候选原因：command outcome 与 withdrawal_order 不是一个原子状态机，且 unknown command 不被周期扫描；domain order 可能最终收敛，但 command 审计状态可能长期 unknown。

### 237. 宝付提现回调 fact 与状态应用是两套表外任务，应用成功后 fact 仍无 processed/application 标记

- 链路：提现回调解析后写 `external_payment_facts`，随后投递 `TaskProcessBaofuWithdrawalFactApplication`；task payload 只有 withdrawal_order_id/upstream_state/baofu_withdraw_no/raw_snapshot，worker 直接更新 `baofu_withdrawal_orders`。
- 证据点：`api/baofu_withdrawal_callback_fact.go:15-55`；`api/baofu_callback.go:155-186`；`worker/task_baofu_withdrawal_fact_application.go:47-91`。该 worker 不接收 fact id，也不更新 `external_payment_facts.processing_status`，没有 `external_payment_fact_applications` 行。
- 风险条件：状态已经应用到 withdrawal_order，但外部事实仍停留在 `received`；或回调 fact 已落库、task 失败/耗尽重试时，没有 generic payment fact application scheduler 可以根据 fact 表补派发，只能依赖 provider 重试或 withdrawal processing recovery。
- 现有保护：handler 在 fact 落库失败或任务入队失败时返回 FAIL，不 ACK；processing withdrawal recovery 仍可 query provider 并直接投递同类 application task。
- 候选原因：这是定制事实应用通道，不是统一 payment fact application 通道；它在业务状态上有 callback/recovery 双入口，但在审计/补偿状态上缺少 durable application 记录。

### 238. 宝付提现余额校验只看 provider 当前余额，不本地冻结 pending 提现金额

- 链路：`CreateWithdrawal` 查询宝付余额，若 `AmountFen <= AvailableAmountFen` 就创建本地 processing withdrawal 和 submitted command；同一 owner 的不同幂等键请求之间没有本地 pending withdrawal sum/freeze。
- 证据点：`logic/baofu_withdraw_service.go:160-176`；`db/query/baofu_withdrawal_order.sql:1-22` 创建 withdrawal_order 时只写 processing，不扣减本地余额；`db/migration/000260_add_baofu_withdrawal_idempotency.up.sql:21-23` 唯一键只覆盖同一 owner + idempotency_key。
- 风险条件：同一主体并发发起两笔不同幂等键提现，两次 QueryBalance 都看到相同 provider 可用余额，均创建 processing command；后续 provider 可能拒绝其中一笔，也可能出现用户侧“已提交金额”超过当前可用余额的展示/预期差。
- 现有保护：provider 最终会按真实余额受理或拒绝；本地 idempotency 能防同一 key 重复；提现接口返回“结果确认中”，不是同步成功。
- 候选原因：这里不是本地账扣错，而是没有本地 pending freeze/limit 约束，属于外部余额查询与异步 command 派发之间的并发窗口；对应旧候选 66 已确认仍成立。

### 239. 宝付分账 prepare 后、command 落库前失败依赖 best-effort 回退，回退失败会留下 processing 空命令态

- 链路：分账 worker 先 `PrepareBaofuProfitSharingCommandTx` 把 `profit_sharing_orders` 从 pending/failed 推到 processing，再构建请求并 `CreateExternalPaymentCommand`；请求构建失败或 command 落库失败时调用 `markBaofuProfitSharingCommandFailed`。
- 证据点：`db/sqlc/tx_baofu_profit_sharing.go:174-229`；`worker/task_baofu_profit_sharing.go:106-145`、`:261-268`；`db/query/profit_sharing_order.sql:291-319`。
- 风险条件：prepare 已提交，但后续请求构建、command 落库或 mark failed 过程中 DB/上下文失败；此时没有 external command 审计行，分账单可能停在 processing。
- 现有保护：`command_started_at` 会让 `ListBaofuProcessingProfitSharingOrdersForRecovery` 扫到 processing 单；recovery 可按 `out_order_no` 查询 provider，若 provider 明确 `ORDER_NOT_EXIST` 且本地没有真实 `sharing_order_id`，会把单据标 failed。
- 候选原因：这不是重复外呼风险，而是“本地已进入 processing 但外部副作用根本未发生”的半状态；最终收敛依赖 recovery 能运行且 provider 对不存在查询给出稳定语义，配置缺失或查询失败时会长期悬挂。

### 240. 宝付分账 provider 已受理后，accepted outcome 与 sharing_id 写入分离，审计和业务引用可长期不一致

- 链路：`CreateProfitSharing` 返回后，worker 先记录 external command accepted，再调用 `UpdateProfitSharingOrderSharingID` 写 provider `TradeNo`。
- 证据点：`worker/task_baofu_profit_sharing.go:147-177`；`db/query/profit_sharing_order.sql:300-304`；测试 `TestProcessTaskBaofuProfitSharingReturnsErrorWhenAcceptedOutcomeAuditFails` 覆盖 accepted outcome 写失败会返回错误。
- 风险条件：provider 已受理，command accepted 已写入，但 `UpdateProfitSharingOrderSharingID` 失败；或 accepted outcome 写失败但 provider 已受理。本地分账单保持 processing，`sharing_order_id` 可能为空。
- 现有保护：recovery 查询在没有真实 `sharing_order_id` 时会用 `out_order_no`；如果后续 callback/query 产生 success/failed fact，application 可把 order 推到终态。
- 候选原因：command 审计、provider 引用、业务分账单终态不是同一事务。业务可能最终按 `out_order_no` 收敛，但 command 与 order 的引用关系可能残缺，影响人工对账、异常恢复和“已受理但本地无 provider 单号”的排查。

### 241. 宝付分账 rejected/unknown command outcome 不会直接把分账单退回 failed，依赖查询恢复解释外部状态

- 链路：`CreateProfitSharing` provider business error 记录 command rejected，网络/系统错误记录 unknown；worker 返回错误，分账单仍是 processing。
- 证据点：`worker/task_baofu_profit_sharing.go:147-154`、`:194-205`；测试 `TestProcessTaskBaofuProfitSharingKeepsProcessingWhenProviderCreateResultUnknown` 和 `TestProcessTaskBaofuProfitSharingRecordsRejectedOutcomeForProviderBusinessError` 明确期望保持 processing。
- 风险条件：provider business error 实际是确定拒绝，但本地 order 不标 failed；后续 recovery 查询如果持续失败、配置缺失、或 provider 对 rejected 创建没有可查询单据，分账单会长期 processing。
- 现有保护：这种设计可避免把网络不确定误判失败；processing recovery 会按 `out_order_no` 查询，并在受限条件下把 `ORDER_NOT_EXIST` 转 failed。
- 候选原因：`processing` 同时表示“provider 已受理等待终态”和“创建被拒绝/不确定等待查询裁决”。这对资金安全偏保守，但对运营可见状态和 SLA 可能造成长时间悬挂；需用宝付 rejected/unknown 的真实查询语义确认。

### 242. 宝付分账 success fact 到达时若本地仍是 pending/failed 会 application failed，无法借 fact 反推 processing

- 链路：分账 callback/query -> `RecordShareFact` 创建 terminal application -> `applyProfitSharingSuccessFact`。
- 证据点：`logic/baofu_profit_sharing_service.go:225-268`；`logic/payment_fact_application_service.go:854-883`；测试 `TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingSuccessRejectsPendingOrder`。
- 风险条件：provider 已完成分账，但本地 prepare/processing 写入失败或被人工/恢复先标 failed；随后 success callback/query 事实到达，本地不是 processing/finished。
- 现有保护：正常 worker 顺序会先 prepare processing 再外呼；success amount 还会校验金额，避免错账直接 finished。
- 候选原因：fact application 不做“外部 success 纠偏本地非 processing”的恢复，只把 success-after-failed/pending 当错误。若 provider 成功而本地状态先失败，application 会持续失败，需要人工对账语义。

### 243. 宝付分账 recovery 入队 fact application 失败不是永久丢失，但会延长 processing 到下一轮扫描

- 链路：processing 分账单 recovery 查询 provider，`RecordShareFact` 生成 terminal application，再投递 `TaskProcessPaymentFactApplication`。
- 证据点：`worker/baofu_payment_recovery_scheduler.go:368-390`；`logic/baofu_profit_sharing_service.go:230-268`；payment fact application scheduler 白名单已包含 profit sharing domain（见 `worker/payment_fact_application_scheduler.go:23-37`）。
- 风险条件：fact/application 已落库，但 asynq 入队失败；或者 scheduler 暂停，application 还未执行，分账单继续显示 processing。
- 现有保护：统一 payment fact application scheduler 会补扫 pending application；下一轮 recovery 也可能因 fact dedupe/application dedupe 继续收敛。
- 候选原因：这不是永久孤儿 fact，但仍是弱顺序观察窗口。运营看到的分账状态取决于 application scheduler 是否健康，故排查时要同时看 fact/application/outbox，而不是只看 recovery 查询日志。

### 244. 宝付分账订单失败后可被重新 prepare 外呼，若旧请求后来成功，会形成新旧请求终态冲突

- 链路：`PrepareBaofuProfitSharingCommandTx` 允许 pending/failed -> processing；`ListBaofuProfitSharingOrdersReadyForCommand` 也扫描 pending/failed；worker 对 failed 单会再次发起 `CreateProfitSharing`，沿用同一个 `out_order_no`。
- 证据点：`db/sqlc/tx_baofu_profit_sharing.go:181-184`、`:219-223`；`db/query/profit_sharing_order.sql:227-260`；测试 `TestPrepareBaofuProfitSharingCommandTxRetriesFailedOrder`。
- 风险条件：第一次外呼状态不确定或被本地 recovery 标 failed，第二次 retry 又外呼；如果宝付按 `out_order_no` 幂等返回既有分账关系，可能收敛；如果 provider 对同号重复提交返回业务错误或旧请求晚到 success，本地会进入候选 203/242 的终态冲突路径。
- 现有保护：`out_order_no` 有 DB 唯一约束，重复 retry 不会换本地商户单号；外部是否幂等需看 provider 语义。
- 候选原因：这是 failed 重试与 provider 最终一致性的交叉窗，不是本地直接双账。需要确认宝付 share-after-pay 对同 `outTradeNo` 重复提交的幂等/错误语义，以及 late success after local failed 的运营处理。

### 245. `profit_sharing_returns` 旧退分账模型没有当前生产创建/查询闭环，但 fact consumer 仍可改变退款状态

- 链路：`profit_sharing_returns` 表仍存在；payment fact application 仍支持 `business_object_type='profit_sharing_return'`，success 后可能启动 refund processing，failed 后会把 refund_order 标 failed。
- 证据点：`db/query/profit_sharing_return.sql:1-70`；`logic/payment_fact_application_service.go:945-1003`、`:1040-1109`；`rg` 当前未在非测试业务代码中发现 `CreateProfitSharingReturn` 生产调用，`api/logic_adapters.go:179-185` 对 legacy return result scheduling 仅记录跳过。
- 风险条件：线上仍有历史 pending/processing return 或历史/手工 fact application 进入该 consumer；旧 return 的 late fact 会继续影响 refund_order。
- 现有保护：若没有历史非终态 return/fact，则这条旧状态机不参与当前宝付退款主链路；退款前分账回退的当前宝付链路似乎已转向 refund-before-share。
- 候选原因：这是“残留状态机仍可写核心退款状态”的边界。后续应以生产数据确认是否存在 `profit_sharing_returns.status in ('pending','processing')` 或未应用的 return fact；若不存在，可降为历史兼容面。

### 246. `profit_sharing_returns` success/failed 覆盖与 refund_order 推进仍未被 DB expected-status 保护

- 链路：return fact success/failed -> 更新 `profit_sharing_returns` -> 统计 return 状态 -> 推进或失败 refund_order。
- 证据点：`db/query/profit_sharing_return.sql:37-63`；`logic/payment_fact_application_service.go:960-1003`、`:1048-1107`。
- 风险条件：同一 return 的 success 与 failed/closed fact 并发或乱序；应用层先读到非终态后，另一个 worker 已写终态，本 worker 仍可按 id 覆盖状态；failed 分支还会尝试把 refund_order 标 failed。
- 现有保护：应用层会避免同状态重复，且 refund_order SQL 不会把 success/closed 回退为 failed。
- 候选原因：旧 return 表的终态 first-wins 不在数据库层表达，应用层读后写不足以覆盖并发乱序。该项是旧候选 62/201/202 的收窄版：前提是旧 return consumer 仍有生产数据或事实流。

### 247. 微信直连 closed/failed 后到账会 ACK 回调并永久保留通知占位，自动退款入队失败后只剩告警/人工

- 链路：直连支付回调发现本地 `payment_order.status in ('closed','failed')` -> 构造 `TaskProcessAnomalyRefund` -> 入队成功或失败后都 `markNotificationProcessed` 并返回微信 SUCCESS。
- 证据点：`api/payment_callback.go:850-947`。
- 风险条件：支付真实到账，但自动异常退款任务入队失败、taskDistributor 未配置、或后续退款任务失败/耗尽重试；微信不会再重投这条支付回调，因为 notification 已 processed 且已 ACK。
- 现有保护：入队失败会发 critical alert，入队成功会发 warning alert；异常退款 task 有自己的重试。
- 候选原因：这是刻意的事故流水去重策略，但状态收敛依赖任务队列或人工处理，不依赖 provider 重试。生产排查时需要把 `wechat_notifications.processed_at`、异常退款任务、告警和实际退款结果串起来，不能只看支付回调已成功。

### 248. 微信直连金额异常分支先把 payment_order 标 paid，再创建/派发退款；派发失败会把 refund_order 标 failed 并 ACK 回调

- 链路：直连支付金额不匹配 -> `ensureAmountMismatchRefundRecord` 创建 refund_order -> `UpdatePaymentOrderToPaid` -> 尝试入队 `TaskProcessRefund` -> 失败时 `markRefundOrderFailed`、发告警、`markNotificationProcessed`、ACK 微信。
- 证据点：`api/payment_callback.go:950-1070`；骑手押金金额异常退款后续语义另见候选 233。
- 风险条件：refund_order 已创建且 payment_order 已 paid，但退款任务入队失败或自动退款条件不足；后续微信不再重投原支付回调，refund_order 已 failed 可能阻断普通 paid-unprocessed recovery（`ListPaidUnprocessedPaymentOrders` 排除存在任意 refund_order）。
- 现有保护：创建 refund_order / 更新 payment_order 失败会 release notification 并要求微信重试；无法自动退款时会发 critical alert；若成功入队，退款 worker/recovery 负责后续。
- 候选原因：金额异常是“ACK + 告警/人工兜底”的路径，不是通用 payment fact application 路径。若队列失败时人工处理未闭环，payment_order=paid、refund_order=failed、业务 application 未执行可能长期并存。

### 249. 直连支付 query 成功对宝付 verify fee 会先标 paid 再 defer application，若后续 scheduler 不运行会停在 paid/unprocessed

- 链路：`PaymentOrderService.QueryPaymentOrder` 远端查询直连支付 -> `recordAndApplyDirectPaymentQueryFact`；对 `baofu_verify_fee` success 先 `markDirectBaofuVerifyFeeQueryPaymentPaid`，再创建 fact/application；`shouldDeferDirectPaymentQueryFactApplication` 命中后不立即 apply。
- 证据点：`logic/payment_order_query_wechat.go:304-389`；`logic/payment_order_query_baofu_verify_fee.go:14-45`。
- 风险条件：用户/系统查询发现 verify fee 已支付并把 payment_order 标 paid，但 application 只落 pending，未立即推进开户 flow；如果 payment fact application scheduler 停止或 target 配置缺失，开户续链会延迟。
- 现有保护：application 已落库，scheduler target 包含 `baofu_account_verify_fee_domain/payment_order`；`PaymentRecoveryScheduler` 也会扫描 paid/unprocessed direct verify fee 并补建 recovered fact。
- 候选原因：这不是固定丢状态，但 query 同步返回的 payment_order 可能已经 paid，而开户 flow 仍未推进；需要在运营/用户可见层接受这个短暂半状态，或用监控确认 scheduler 延迟。

### 250. paid-unprocessed recovery 排除已有 refund_order，金额异常/异常到账路径不会被通用支付成功恢复补业务

- 链路：`PaymentRecoveryScheduler` 扫 `ListPaidUnprocessedPaymentOrders`，只对无 refund_order 的 paid/unprocessed payment_order 补 direct payment fact application。
- 证据点：`db/query/payment_order.sql:198-209`；`worker/payment_recovery_scheduler.go:98-150`。
- 风险条件：直连金额异常分支或 closed/failed 异常到账分支已经创建 refund_order/异常退款意图，但业务 payment_order 仍 paid 且 processed_at 为空；通用 recovery 因 `NOT EXISTS refund_orders` 跳过。
- 现有保护：这些场景本来不应继续正常业务成功处理，而应走退款/人工闭环；排除 refund_order 能避免把异常到账误推进业务。
- 候选原因：这属于设计边界，不是单纯漏扫。排查 paid/unprocessed 时必须区分“正常支付成功待应用”和“异常支付待退款”，后者不会由 payment recovery 自动补业务状态。

### 251. 宝付支付 fact、实际手续费 ledger、application 是三段提交；ledger 失败会留下已收事实但未创建 application 的短半状态

- 链路：宝付支付 callback/query/recovery 调 `BaofuPaymentService.RecordPaymentFact` -> `CreateExternalPaymentFact` -> 若 `FeeAmountFen>0` 则 `UpsertOrderPaymentFeeLedgerActual` -> terminal fact 再 `CreateExternalPaymentFactApplication`。
- 证据点：`logic/baofu_payment_service.go:218-282`；`db/query/order_payment_fee_ledger.sql:36-80`；`api/baofu_callback.go:241-256`。
- 风险条件：fact 写入成功后，实际手续费 ledger upsert 失败；当前调用直接返回错误，terminal application 尚未创建。宝付 callback 入口会返回 FAIL，理论上靠 provider 重投或后续 query/recovery 再走同一 dedupe fact 来补 ledger/application；但在重投/恢复到来前，事实表已有 success，业务应用仍未发生。
- 现有保护：`CreateExternalPaymentFact` 对相同 dedupe/相同字段会返回既有 fact；callback fact 落库失败会返回 FAIL；宝付主业务 query success 会同步 `RecordPaymentFact` 并尝试应用；fee ledger upsert 本身幂等。
- 候选原因：这是事实记录与业务应用之间的短暂可恢复半状态，不是立即永久丢单。生产排查需要同时看 `external_payment_facts` 是否已有 success、`external_payment_fact_applications` 是否缺失、以及该 fact 是否带手续费且 ledger 写入失败日志。

### 252. 宝付支付 application 创建失败依赖源头重试/查询恢复补齐，application scheduler 只扫已存在的 application

- 链路：`RecordPaymentFact` 在 fact 与 fee ledger 成功后创建 application；若 `CreateExternalPaymentFactApplication` 失败，callback 返回 FAIL，query/recovery 返回错误。
- 证据点：`logic/baofu_payment_service.go:269-282`；`db/query/external_payment_fact.sql:257-275`；`worker/payment_fact_application_scheduler.go:23-37,108-131`。
- 风险条件：terminal fact 已存在，但 application 创建失败；此时 payment fact application scheduler 没有 application 记录可扫。只有宝付 provider 重投 callback、用户/系统再次 query、或宝付专用 recovery 再次调用 `RecordPaymentFact`，才能通过 dedupe fact 回放并创建 application。
- 现有保护：同 fact 的 `CreateExternalPaymentFactApplication` 冲突会返回既有 application；callback 创建 application 失败会向宝付返回 FAIL 而不是 ACK；宝付 recovery/query 也可补建。
- 候选原因：这与候选 205/174-182 的公共问题一致，但宝付支付入口的具体边界是“application 不存在时，scheduler 无法从 fact 反向补建”。需要确认宝付 callback 重投窗口、query/recovery 调度频率，以及是否需要 fact-without-application 监控。

### 253. 宝付支付实际手续费可被后到 callback/query 覆盖为另一种 actual source，未见按 observed_at/occurred_at 防旧事实覆盖新事实

- 链路：多来源宝付 payment fact 都可能携带 `FeeAmountFen`；`UpsertOrderPaymentFeeLedgerActual` 在唯一键冲突时直接覆盖 `base_amount/rate_bps/amount/amount_source/status/calculation_version`，仅 `external_payment_fact_id` 用 `COALESCE(EXCLUDED, existing)`。
- 证据点：`logic/baofu_payment_service.go:246-267`；`db/query/order_payment_fee_ledger.sql:69-80`；`db/query/external_payment_fact.sql:239-246` 可列出同外部对象多事实。
- 风险条件：callback 与 query/manual recovery 先后携带不同 fee amount 或不同 `amount_source`；后到写入会覆盖先前 actual 值，没有按 provider 更新时间、观测时间或来源优先级做旧值保护。
- 现有保护：calculated ledger 不会覆盖 actual（见 NP-37）；实际手续费理论上来自同一 provider 单据，正常情况下应一致；`external_payment_fact_id` 首次 actual fact 会保留。
- 候选原因：本地把“实际值”视为后写覆盖，而不是事件时间 first/last wins。需要用 provider 语义确认 callback/query fee 字段是否绝对稳定；若不稳定，利润分账计算可能依赖被后到事实改写后的手续费。

### 254. 宝付核验费成功 application 先标 payment_order processed，再续链开户；若续链持续失败，会形成 paid+processed 但 flow 未推进的观察窗口

- 链路：直连核验费支付成功 callback/query -> payment_order paid -> fact application -> `applyBaofuVerifyFeePaymentFact` -> `UpdatePaymentOrderProcessedAt` -> `ContinueAfterVerifyFeePaid` -> `PrepareOpeningAfterVerifyFeePaid` / enqueue `TaskProcessBaofuAccountOpening`。
- 证据点：`api/payment_callback.go:1073-1120`；`logic/payment_fact_application_baofu_verify_fee.go:37-58`；`db/query/payment_order.sql:211-216`；`worker/task_payment_fact_application.go:36-50`。
- 风险条件：`UpdatePaymentOrderProcessedAt` 已提交后，读取 flow/profile、`MarkBaofuAccountOpeningFlowOpeningProcessing`、或开户任务入队失败；application 会标 failed 待重试，但在重试成功前，支付单已经 `processed_at` 有值，开户 flow 可能仍停在 `verify_fee_processing` 或已到 `opening_processing` 但无任务执行。
- 现有保护：application 失败后 scheduler 会重试；重试时即使 payment_order 已 processed，代码会把 `ErrRecordNotFound + paymentOrder.ProcessedAt.Valid` 视为幂等继续；如果 flow 已经 `opening_processing`，`PrepareOpeningAfterVerifyFeePaid` 会返回 `ShouldEnqueueOpening=true` 再次尝试入队。
- 候选原因：这不是永久断链结论，而是“支付处理标记早于开户续链完成”的半状态。运营排查 paid+processed 核验费时，不能只看 payment_order，还要联查 application status、opening flow state 和开户任务队列。

### 255. 预约主支付成功仍可无条件把 reservation 写为 paid，可能覆盖 timeout/cancel/completed/no_show 等后到状态

- 链路：reservation payment fact success -> `applyReservationPaymentFact` -> `ProcessPaymentSuccessTx` -> business_type `reservation` -> `CreateReservationPayment` -> `UpdateReservationStatus(... paid)` -> `syncReservationInventoryWithQueries` -> `UpdatePaymentOrderProcessedAt`。
- 证据点：`logic/payment_fact_application_service.go:401-433`；`db/sqlc/tx_payment_success.go:142-170`；`db/query/table_reservation.sql:121-126`。
- 风险条件：预约支付单已 paid，但 application 晚于预约 timeout/cancel/no_show/completed 等状态写入；或者支付成功事实与预约取消事务并发。`UpdateReservationStatus` 只按 id 更新，没有 `WHERE status='pending'` 或锁内状态断言。
- 现有保护：`ProcessPaymentSuccessTx` 锁 payment_order 并用 `processed_at` 幂等；`reservation_payments.payment_order_id` 先查后插避免重复应用同一支付单；但这些保护不限制 reservation 当前状态。
- 候选原因：这是候选 6/7 的核心实例。payment_order 层保证同一支付单只处理一次，但 reservation 状态机没有把“只有 pending 可变 paid”表达在 DB/事务层，晚到成功可能回退或覆盖已经终态/后续态的预约。

### 256. 预约加菜旧 payment_order 分支只累加 prepaid，不校验 reservation 当前状态，若未命中新 adjustment 记录仍有旁路覆盖风险

- 链路：business_type `reservation_addon` 的 payment success -> `ProcessPaymentSuccessTx`；若 `GetReservationAdjustmentByPaymentOrderForUpdate` 找不到 adjustment，则走旧路径：`CreateReservationPayment(type='addon')` -> `AddReservationPrepaidAmount` -> `syncReservationInventoryWithQueries` -> mark processed。
- 证据点：`db/sqlc/tx_payment_success.go:173-212`；`db/query/table_reservation.sql:136-142`。
- 风险条件：历史/旁路创建的 reservation_addon payment_order 没有关联 `reservation_adjustments`，但对应 reservation 已 cancelled/expired/completed/no_show；支付成功 application 会直接增加 `prepaid_amount`，没有锁 reservation 后验证可加菜状态。
- 现有保护：新调整链路会优先锁 adjustment 并调用 `applyPaidReservationAdjustmentWithQueries`；该新链路检查 reservation 状态必须是 `paid/confirmed/checked_in` 且未开始烹饪（见 NP-43）。
- 候选原因：这是 legacy fallback 与新 adjustment 状态机共存的时序边界。需要确认生产是否仍会创建无 adjustment 的 `reservation_addon` payment_order；若没有，风险可降为历史兼容面。

### 257. 订单支付成功后的分账账单/outbox 在业务已 processed 后失败，会让 application 重试承担补齐，而 payment_order 已显示处理完成

- 链路：order payment fact success -> `ProcessPaymentSuccessTx` 激活订单并 `UpdatePaymentOrderProcessedAt` -> 对宝付且 `requires_profit_sharing` 时 `EnsureBaofuProfitSharingBillTx` -> `CreatePaymentDomainOutboxOnce` -> terminalize fact/application。
- 证据点：`logic/payment_fact_application_service.go:333-370`、`:1112-1180`（outbox/分账创建逻辑在同服务内）；测试 `TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPaymentBillFailureBlocksOutbox` 表示账单失败会使 application failed 但前面的 payment success tx 已返回 processed。
- 风险条件：订单状态和 payment_order.processed_at 已提交，但分账账单创建、success outbox 创建或后续 fact/application terminal mark 失败；运营侧看到支付单已 processed、订单已 paid，但支付事实 application 仍 failed/processing，通知/分账/outbox 未补齐。
- 现有保护：application 失败后 scheduler 可重试；`applyOrderPaymentFact` 对 `processed_at` 已有的 order payment 会回读 order 并重建 outbox payload；宝付分账账单用 `EnsureBaofuProfitSharingBillTx` 幂等创建/刷新。
- 候选原因：这是多段提交的可恢复半状态，不是订单重复激活风险。排查订单已 paid 但通知/分账缺失时，需要查 payment fact application，而不是只查 payment_order.processed_at。

### 258. refund success fact 晚于本地 failed/closed 时不会自动升格为 success，application 会失败并等待人工/重试语义确认

- 链路：退款 fact success -> `applyOrderRefundFact` / `applyReservationRefundFact` -> 若 refund_order 当前不是 success，则调用 `UpdateRefundOrderToSuccess`；SQL 只允许 `status IN ('pending','processing')`。
- 证据点：`logic/payment_fact_application_service.go:541-552`、`:656-677`；`db/query/refund_order.sql:158-164`。
- 风险条件：本地因发起退款失败、provider 返回异常、旧 worker 或人工流程先把 refund_order 标为 `failed/closed`，之后 callback/query 又得到 provider success fact。success 分支没有像 failed/closed 分支那样 reload terminal conflict 并按 success 幂等处理，而是把更新错误返回给 application。
- 现有保护：不会把 `failed/closed` 静默覆盖为 `success`；application 保持 failed 可见；provider success fact 留在 `external_payment_facts`。
- 候选原因：这是“本地先终态 vs provider late success”的业务语义窗。若 provider 已真实退款，本地 refund_order 仍 failed/closed 会影响 payment_order refunded、reservation prepaid 扣减、退款成功 outbox/通知；需要人工对账或定义 success-after-failed 的自动纠正策略。

### 259. reservation refund success 扣减 prepaid 与 refund_order success 是分段提交，扣减失败会导致 success refund_order 与 prepaid 未扣减并存

- 链路：reservation refund success fact -> `UpdateRefundOrderToSuccess` -> `maybeMarkPaymentOrderRefunded` -> `AddReservationPrepaidAmount(-refund_amount)` -> outbox/fact/application terminal mark。
- 证据点：`logic/payment_fact_application_service.go:656-677`；`db/query/table_reservation.sql:136-142`。
- 风险条件：refund_order 已从 pending/processing 更新为 success 后，扣减 reservation prepaid 失败；application 会失败重试，但重试时 `refundOrder.Status == success`，`transitionedToSuccess=false`，当前代码不会再次执行 `AddReservationPrepaidAmount(-refund_amount)`。
- 现有保护：同一次调用里若扣减成功，后续 outbox/terminal mark 失败重试不会重复扣减；这避免了重复扣减。
- 候选原因：这是“只在从非 success 转 success 的那次调用做 prepaid 扣减”的补偿缺口。需要确认 `AddReservationPrepaidAmount` 失败后是否有独立修复任务；否则可能出现退款成功但预约 prepaid_amount 仍未回退。

### 260. 普通直连订单退款仍走旧 `TaskProcessRefundResult`，与 payment fact application 的终态/通知语义不完全一致

- 链路：微信直连退款回调 -> 非骑手押金退款入队 `TaskProcessRefundResult`；旧 worker 对 order refund 直接 `UpdateRefundOrderToSuccess/Failed/Closed`、通知、maybe mark payment refunded。骑手押金直连退款和宝付订单/预约退款则走 payment fact application。
- 证据点：`api/payment_callback.go:1285-1340`；`worker/task_process_payment.go:734-851`；宝付退款 callback 走 `recordBaofuRefundCallbackFact`：`api/baofu_callback.go:515-588`。
- 风险条件：同一 refund_order 的状态可能来自旧 worker payload、payment fact callback/query、recovery query 或手工任务；不同入口对 terminal conflict、outbox、通知、reservation prepaid 的处理不完全相同。
- 现有保护：旧 worker 对 reservation/rider deposit refund result 会拒绝并要求走 payment fact application；宝付 refund callback 落 fact 失败会 FAIL。
- 候选原因：这是退款结果消费的双轨边界。排查退款乱序时必须先按 `payment_channel/business_type` 判断走旧 result task 还是 payment fact application，不能把两套语义混作一种幂等模型。

## 已核实的非问题

### NP-1. 宝付核验费成功续链不会直接外呼开户

- 链路：宝付核验费 payment fact application -> `BaofuVerifyFeeAsyncContinuation.ContinueAfterVerifyFeePaid` -> `PrepareOpeningAfterVerifyFeePaid`。
- 证据点：`logic/payment_fact_application_baofu_verify_fee.go:54-56`，`worker/task_payment_fact_application.go:36-46`，`logic/baofu_account_onboarding_prepare.go:16-65`。
- 结论：这条生产链路只做 flow 预处理并在需要时入队 `TaskProcessBaofuAccountOpening`，不直接调用 `OpenAccount`；因此“核验费成功回调一到就重复开户”不是这条链路上的问题。

### NP-2. 已推进到报备/ready/voided 的开户 flow 不会被重复核验费成功重开

- 证据点：`logic/baofu_account_onboarding_prepare.go:27-39`，`logic/baofu_account_onboarding_prepare.go:41-65`，`logic/baofu_account_onboarding_service.go:382-420`。
- 结论：`PrepareOpeningAfterVerifyFeePaid` 在 `merchant_report_processing` / `applet_auth_pending` / `ready` / `voided` 时只回传当前状态，不会再排队 `opening`；`ContinueAfterVerifyFeePaid` 在 `opening_processing` / `merchant_report_processing` / `applet_auth_pending` / `ready` 时直接 return nil。重复核验费成功本身不是重开窗。

### NP-3. 宝付开户结果在有 `out_request_no` 时是精确匹配，错流回写被阻断

- 证据点：`logic/baofu_account_onboarding_apply.go:201-233`。
- 结论：`ensureAccountOpenResultSerialMatchesFlow` 和 `ensureAccountOpenContractMatchesFlow` 会拦截流水号/合约号归属不一致并打平台告警；因此“带正确流水号的旧回调静默落到新 flow”不是问题，真正的高风险窗只剩 `out_request_no` 缺失时的 `contract_no -> latest flow` fallback（对应候选 220）。

### NP-4. 宝付商户报备重复 `bind_sub_config` 被刻意当成成功

- 证据点：`logic/baofu_merchant_report_service.go:267-309`，`logic/baofu_merchant_report_service_test.go:115-132`，`logic/baofu_account_onboarding_service_test.go:1753-1818`。
- 结论：当命令已记录且 provider 返回 `BIND_REPEAT_ERROR` 时，代码会直接标记 applet auth succeeded；这不是异常分支，不应再按重复绑定失败去追。

### NP-5. 宝付商户 ready 落点是单事务

- 证据点：`logic/baofu_account_merchant_report_service.go:196-206`，`db/sqlc/tx_baofu_account_opening_ready.go:19-46`。
- 结论：`payment_config`、`flow.ready`、`merchant.activate` 在同一个事务里提交；这里没有“ready 已写但商户激活丢失”的内部分段问题。真正的边界在更早的 account binding 激活阶段，而不是 ready tx 本身。

### NP-6. 宝付 verify fee 终态失败不会回退已推进 flow

- 证据点：`logic/payment_fact_application_baofu_verify_fee.go:62-118`，`db/query/baofu_account_opening_flow.sql:86-99`。
- 结论：终态失败分支会尝试把 flow 重置到 `verify_fee_pending`，但 SQL 只允许 `profile_pending` / `verify_fee_pending` / `verify_fee_processing`；`opening_processing` / `merchant_report_processing` / `applet_auth_pending` / `ready` 不会被回退。候选 204 的风险只存在于 `verify_fee_processing` 窗口内，不是更后面的状态回写问题。

### NP-7. 宝付商户报备先写 command 再 upsert report 的失败窗口只留下审计残留，不会形成业务外部副作用

- 证据点：`logic/baofu_merchant_report_service.go:98-127`，`logic/baofu_account_merchant_report_service.go:85-123`。
- 结论：如果 `UpsertBaofuMerchantReportProcessing` 在 command 落库后失败，provider 外呼根本不会发生，剩下的是一条孤立 command 审计记录；恢复链路只按 `baofu_merchant_reports` 扫描，不依赖这条 command。这个窗口不是业务状态时序 bug，对应候选 219。

### NP-8. 宝付开户回调没有单独 application backlog，但 handler 重试和 flow recovery 已覆盖

- 证据点：`api/baofu_account_open_callback_fact.go:20-65`，`logic/baofu_account_onboarding_recovery.go:6-28`，`worker/baofu_account_opening_recovery_scheduler.go:89-130`。
- 结论：回调 handler 先落 fact 再直接应用 flow，失败时会向 provider 返回错误；同一回调重试可通过 dedupe 重新进入应用，而 `opening_processing` / `failed` flow 也会被 recovery scheduler 扫到。这里没有独立 application backlog 只是架构选择，不是丢状态的洞。

### NP-9. 我已复核的 terminal fact/application 入口大多有源头重试或恢复，不是“写完 fact 就永久丢 application”的固定洞

- 证据点：`api/payment_callback.go:967-1153`，`api/baofu_callback.go:202-262`，`worker/task_payment_timeout.go:205-233`，`worker/task_payment_timeout_baofu.go:81-124`，`worker/payment_recovery_scheduler.go:77-122`，`worker/baofu_payment_recovery_scheduler.go:87-124, 201-278`，`worker/refund_recovery_scheduler.go:84-120, 460-560`，`worker/baofu_account_opening_recovery_scheduler.go:89-180`。
- 结论：这类路径的真正依赖点是“来源侧是否还能重入”，而不是 application scheduler 本身是否扫描 fact 表。当前没有看到一条确认会永久落成孤儿 fact 的主链路。

### NP-10. 合单支付当前业务入口 fail-closed，SQL/事务残留不是当前运行中主链路

- 证据点：`logic/combined_payment_service.go:82-93` 的 create/query/close 均返回 `combinedPaymentUnsupportedError()`；`combined_payment.sql` 与 `tx_close_combined_payment.go` 仍保留历史表和关闭事务；`scheduler/data_cleanup.go` 仍会清理过期 combined pending。
- 结论：合单支付的创建/支付/关闭时序暂不作为当前主业务风险继续展开；残留 pending combined order 更像历史数据/清理兼容面。

### NP-11. 会员线上充值已暂停，不再产生 `membership_recharge` 支付事实应用链

- 证据点：`api/membership.go:649-686` 仅做 membership/规则预校验，随后直接返回 503 “会员线上充值已停用”；`PaymentOrderService.CreatePaymentOrder` 只接受 `order` / `reservation` / `reservation_addon`，不接受 `membership_recharge`。
- 结论：当前未看到用户线上会员充值支付单创建和支付成功 fact consumer；这条旧业务类型不作为当前支付 fact 时序风险继续展开。

### NP-12. 商户代录会员充值/调整的资金变动与流水在同一事务内，且有幂等唯一索引

- 证据点：`logic/membership_recharge.go:88-156`、`db/sqlc/tx_membership.go:53-141`、`logic/membership_balance_adjust.go:20-70`、`db/sqlc/tx_membership.go:292-391`、`000204_add_membership_recharge_idempotency.up.sql`、`000241_add_membership_adjustment_idempotency.up.sql`。
- 结论：同一幂等键重试不会双入账；并发撞唯一索引时事务整体回滚并回读既有流水。当前未看到“余额已改、流水没写”的运行中窗口。

### NP-13. 微信商家转账回调入口不是无保护的任意 action 覆盖

- 证据点：`api/payment_callback.go:542-641` 先验签，再按 notification id `tryClaimNotification`，解密后校验 mchid 归属，最后从 `out_bill_no=claimpayout{actionID}` 解析 action；失败会释放 notification claim 等待微信重试。
- 结论：这条回调入口的信任边界和重复通知处理较完整；当前未把“伪造/重复回调直接覆盖任意 payout action”列为候选问题。真正残余风险在 action 执行侧的 running/not-found 和并发外呼窗口（对应 228/229）。

### NP-14. 普通 claim 赔付成功后 action success 写失败，多数可由重放收敛

- 证据点：`worker/task_claim_refund.go:424-477` 成功查询后先 `MarkClaimPaid` 再 `FinalizeClaimCompensationAfterPayoutTx`，最后标 action success；`db/query/trust_score.sql:340-344` 的 `MarkClaimPaid` 用 `COALESCE(paid_at, $2)`；`db/sqlc/tx_claim_behavior.go:551-615` 会锁 claim 后根据既有 recovery/action 补齐；`worker/claim_behavior_action_recovery_scheduler.go:85-116` 会补派发 created/failed 行为动作。
- 结论：如果 claim 已 paid、追偿/通知 action 已创建，但 payout action 标 success 失败，后续查询到微信 success 后重放通常不会双写 paid_at，也会复用已有 recovery/action。这里不作为独立“先改业务后标 action”问题追踪；仍需关注 228 的 NOT_FOUND 悬挂窗口。

### NP-15. 骑手押金提现准备事务的资金冻结主体较完整，不是同幂等键双扣 credit 的问题

- 证据点：`db/sqlc/tx_rider_refund.go:67-255` 在同一事务内锁 rider、锁 `rider_deposit_withdrawal_requests` 幂等请求、锁每条 deposit credit、锁源 payment_order，随后消耗 refundable credit、创建 refund_order、写 freeze log、更新 withdrawal request refund ids、增加 `riders.frozen_deposit`。
- 结论：同一幂等键不会重复创建多组 refund_order/credit 消耗；并发不同幂等键也会通过 rider row 与 credit row 锁串行化。当前候选 230 关注的是 active delivery eligibility 时序，不是押金 credit 主账双扣。

### NP-16. 骑手押金退款 fact application 入队失败不是固定永久丢失

- 证据点：退款回调成功解析本地对象后会创建 `consumer='rider_deposit_domain'`、`business_object_type='refund_order'` 的 application：`api/payment_callback.go:353-389`；scheduler 白名单包含该 target：`worker/payment_fact_application_scheduler.go:23-37`；stuck processing refund 查询也会补建 rider_deposit refund query fact：`worker/refund_recovery_scheduler.go:342-358`。
- 结论：在 fact/application 已经创建的前提下，单次 asynq 入队失败会被 scheduler 补扫；因此不把“创建 fact 后入队失败”单独列为 rider deposit refund 永久丢状态问题。真正风险是候选 232 的“解析失败时根本没有创建 fact/application”。

### NP-17. 微信直连退款回调入口具备签名、通知幂等和商户归属校验

- 证据点：`api/payment_callback.go:1157-1340` 先验签、解析事件类型、`tryClaimNotification` 原子占位、解密 resource、校验 mchid 归属；失败分支会 release notification 并向微信返回 FAIL。
- 结论：当前未把“伪造直连退款回调直接覆盖退款单”列为问题。剩余风险集中在回调业务对象解析/新旧消费链路分流、以及具体 refund_type 的结算语义。

### NP-18. 押金提现与接单并发一般不会造成押金透支

- 证据点：提现准备事务锁 rider 并增加 `FrozenDeposit`：`db/sqlc/tx_rider_refund.go:71-109`、`:242-249`；接单事务也锁 rider 并按最新 `DepositAmount - FrozenDeposit` 校验 `FreezeAmount`：`db/sqlc/tx_delivery.go:208-230`、`:256-261`。
- 结论：如果接单先完成，提现事务会看到 frozen deposit 并拒绝；如果提现先完成，接单只能使用提现后剩余可用押金。候选 230 不是资金透支结论，而是“提现处理中是否仍允许形成新 active delivery”的业务资格边界。

### NP-19. 宝付提现同一幂等键不会重复创建本地提现单或 command

- 证据点：`logic/baofu_withdraw_service.go:136-147`、`:209-241`；`db/migration/000260_add_baofu_withdrawal_idempotency.up.sql:21-23`；`db/migration/000261_harden_baofu_withdrawal_idempotency_pair.up.sql:31-45`；`db/sqlc/tx_baofu_withdrawal.go:23-59` 在一个事务里创建 withdrawal_order 与 submitted command。
- 结论：同一 owner + idempotency key 会按 request hash 回放或冲突，不会在本地生成多笔提现意图。候选 238 关注的是不同幂等键并发，不是同 key 重放。

### NP-20. 宝付提现创建接口不直接外呼 provider 是当前设计，不是“HTTP 请求半途双外呼”问题

- 证据点：`logic/baofu_withdraw_service.go:175-206` 只调用 `CreateBaofuWithdrawalOrderWithSubmittedCommandTx`；测试 `TestBaofuWithdrawServiceCreateWithdrawalRecordsSubmittedCommandWithoutProviderCall`、`TestBaofuWithdrawServiceCreateWithdrawalPersistsIntentWithoutProviderDispatch` 明确期望创建阶段不调用 `CreateWithdraw`。
- 结论：创建接口返回“提现申请已提交，结果正在确认”后，外呼由 command dispatch worker/recovery 负责。因此本组风险应聚焦 submitted/unknown command 派发与恢复，而不是同步接口重复外呼。

### NP-21. 宝付提现回调入口不是无保护 ACK

- 证据点：`api/baofu_callback.go:115-186` 会检查 parser、task distributor、payout merchant/terminal identity、out_request_no、本地 withdrawal_order；fact 落库失败或 task 入队失败都返回 FAIL；测试 `TestBaofuWithdrawCallbackPersistsFactBeforeEnqueueAndAck`、`TestBaofuWithdrawCallbackDoesNotEnqueueWhenFactPersistenceFails`、`TestBaofuWithdrawCallbackRejectsPayoutIdentityMismatch` 覆盖主要边界。
- 结论：当前不把“提现回调未验归属/落库失败仍 ACK”列为问题。剩余风险在 fact/application 状态不统一（候选 237）以及 command 派发恢复（候选 235/236）。

### NP-22. 宝付提现 late terminal callback/query 不会覆盖已终态 withdrawal_order

- 证据点：`worker/task_baofu_withdrawal_fact_application.go:55-61` 先读 order，已终态直接 return；`db/query/baofu_withdrawal_order.sql:78-88` 更新终态也要求当前 `status='processing'`；测试 `TestProcessTaskBaofuWithdrawalFactApplicationDoesNotRegressTerminalOrder` 覆盖 late result 不回退。
- 结论：普通的 succeeded/failed/returned 后到结果不会覆盖既有终态；旧候选 68 可以收窄为“provider 终态语义需确认”，不再按本地终态覆盖风险继续追。

### NP-23. 宝付提现 order 先成功写入 provider 单号后，command outcome 写失败有修复路径

- 证据点：unknown command dispatch 会调用 `repairClaimedBaofuWithdrawalCommandOutcome`，从 withdrawal_order 的 `baofu_withdraw_no/raw_snapshot/status` 推断 accepted/rejected：`worker/task_baofu_withdrawal_command_dispatch.go:112-114`、`:266-314`；测试 `TestProcessTaskBaofuWithdrawalCommandDispatchRepairsAcceptedOutcomeAfterOrderUpdate` 与 `...RepairsRejectedOutcomeAfterOrderUpdate` 覆盖该路径。
- 结论：如果 withdrawal_order 已经成功写入 provider 单号或失败 raw state，只是 command outcome 写失败，则重放同一个 dispatch task 可修复 command。候选 236 只保留 order 更新失败的更窄窗口。

### NP-24. 宝付分账账单创建/刷新与手续费 ledger 是同一事务

- 证据点：`db/sqlc/tx_baofu_profit_sharing.go:113-170`，`createBaofuProfitSharingOrderWithLedgers` / `refreshBaofuProfitSharingBillWithLedgers` 会在同一 `execTx` 内创建或刷新 `profit_sharing_orders`，并写 `baofu_fee_ledgers`、`order_payment_fee_ledgers`；测试覆盖返回既有账单、刷新 pending reservation bill、冲突拒绝等场景。
- 结论：当前宝付账单创建不是“分账单已建但 fee ledger 没写”的裸分段提交；候选风险应集中在账单进入 processing 后的 command/provider/fact/application 分段，而不是账单与手续费账本原子性。

### NP-25. 宝付分账发起前的 active refund 与成功退款净额会在事务内复核

- 证据点：`EnsureBaofuProfitSharingBillTx` 锁 `payment_order` 后检查 active refund、成功退款净额和账单金额：`db/sqlc/tx_baofu_profit_sharing.go:116-139`；`PrepareBaofuProfitSharingCommandTx` 再次锁 `profit_sharing_order` / `payment_order` 并复核：`:174-229`；测试 `TestPrepareBaofuProfitSharingCommandTxRejectsActiveRefund`、`...RejectsStaleBillAfterSuccessfulRefund`、`...AllowsReservationSuccessfulRefundWhenBillMatchesNetAmount`。
- 结论：旧候选 10“分账准备与退款创建竞态”需收窄：在 prepare 事务提交前出现的 active refund / net amount drift 会被挡住。真正残余是 prepare 后已经 processing，再出现退款请求时，业务需要走后续退款/退分账/人工对账语义。

### NP-26. 宝付分账 ready list 的退款过滤与 prepare 复核形成双层保护

- 证据点：`ListBaofuProfitSharingOrdersReadyForCommand` 过滤 paid/completed 且无 active refund，对 order 型成功退款也跳过：`db/query/profit_sharing_order.sql:227-260`；`PrepareBaofuProfitSharingCommandTx` 在事务内再次复核：`db/sqlc/tx_baofu_profit_sharing.go:197-217`。
- 结论：单靠 ready list 不是最终一致性保护，但它不是唯一保护；即使 list 与退款创建并发，prepare 仍会按最新锁内状态拒绝。因此不要把 ready list 的读视为本链路唯一时序防线。

### NP-27. 当前源码未见生产路径绕过 `EnsureBaofuProfitSharingBillTx` 直接创建宝付分账单

- 证据点：非生成、非测试业务代码中，`CreatePendingOrder` 调 `EnsureBaofuProfitSharingBillTx`：`logic/baofu_profit_sharing_service.go:92-189`；主要触发来自 reservation completion、payment fact application 和 baofu recovery：`logic/reservation_profit_sharing.go:46-72`、`logic/payment_fact_application_service.go:1247`、`worker/baofu_payment_recovery_scheduler.go:229,280`。`CreateProfitSharingOrderSimple` 未见生产调用。
- 结论：虽然 schema 上 `profit_sharing_orders.payment_order_id` 只有普通索引，当前宝付主链路并未发现直接绕过事务批量插入的生产入口；“同一 payment_order 双宝付分账账单”暂不作为已确认时序问题，后续若发现新写入口再重开。

### NP-28. 宝付分账单终态 SQL 对 finished/failed 互相覆盖有 processing guard

- 证据点：`UpdateProfitSharingOrderToFinished` 与 `UpdateProfitSharingOrderToFailed` 均要求 `status='processing'`：`db/query/profit_sharing_order.sql:306-319`；`applyProfitSharingFailedFact` 遇 finished 会返回 nil，不回退：`logic/payment_fact_application_service.go:900-927`；测试 `TestUpdateProfitSharingOrderToFailedDoesNotRegressFinished`、`TestPaymentFactServiceApplyExternalPaymentFactApplication_ProfitSharingFailedFactTreatsFinishedUpdateConflictAsIdempotent`。
- 结论：普通 late failed 不会把本地已 finished 的分账单覆盖为 failed。剩余风险不是终态覆盖，而是 success-after-failed 会持续 application failed、failed-after-finished 不出失败 outbox 的运营语义（候选 203/242）。

### NP-29. 宝付分账 provider `TradeNo` 查询失败时会回退按 `outOrderNo` 查询

- 证据点：`queryBaofuProfitSharing` 优先用真实 `sharing_order_id` 查询，但在 business error 或 invalid data content 时清空 `TradeNo` 改用 `OutTradeNo`：`worker/baofu_profit_sharing_recovery_query.go:29-67`。
- 结论：`sharing_order_id` 写入错误/失效并不必然导致 processing 永久无法查询；仍有 out_order_no fallback。候选 59/240 的残余重点是 provider `ORDER_NOT_EXIST` 一致性窗口和本地引用审计不一致，而不是“只要 sharing_id 缺失就完全不能恢复”。

### NP-30. 宝付分账 fact/application 入队失败已有统一 scheduler 补扫

- 证据点：分账 RecordShareFact 对 terminal fact 创建 `consumer='profit_sharing_domain'`、`business_object_type='profit_sharing_order'` 的 application：`logic/baofu_profit_sharing_service.go:262-268`；scheduler 白名单包含 profit sharing order/return：`worker/payment_fact_application_scheduler.go:23-37`；recovery 入队失败只记录错误不删除 application：`worker/baofu_payment_recovery_scheduler.go:384-390`。
- 结论：在 fact/application 已经落库后，单次 asynq 入队失败不按永久丢状态处理；真正要查的是 application 是否持续失败、scheduler 是否运行、以及源头是否没有创建 application。

### NP-31. `profit_sharing_returns` 当前未见生产创建和 provider 查询恢复入口，旧状态机应先用数据确认是否仍活跃

- 证据点：`rg` 非测试业务代码未发现 `CreateProfitSharingReturn` 调用；`api/logic_adapters.go:179-185` 对 `ScheduleProfitSharingReturnResult` 仅记录跳过 legacy scheduling；`ListStuckProcessingProfitSharingReturns` 仅剩 query/生成接口，当前未见 worker 调用。
- 结论：候选 245/246 不应扩张成“当前宝付退款必然走旧退分账链路”。它更像历史兼容/残留事实 consumer，下一步需要生产数据或迁移记录确认是否有非终态旧 return。

### NP-32. 微信直连支付回调中 direct payment fact application 入队失败会被 scheduler 补扫

- 证据点：`api/payment_callback.go:1094-1120` 记录 direct payment fact 后只是在 enqueue 失败时打 warn，不会回滚 fact/application；`api/direct_payment_fact_application.go:12-33` 只是回调侧轻量入队；`worker/payment_fact_application_scheduler.go:23-37,108-131` 会按 target 重新派发 pending/failed application。
- 结论：direct payment callback 的入队失败不是固定永久丢状态问题。真正残余更像“回调已 ACK、应用暂时滞后”，后续由 scheduler 或后续查询/recovery 收敛。

### NP-33. 宝付主业务支付 query 成功会立即走统一 fact application，而不是留给 callback 专有通道

- 证据点：`logic/payment_order_query_wechat.go:214-254` 对宝付 aggregate query 成功直接 `RecordPaymentFact` 后同步 `ApplyExternalPaymentFactApplication`；`logic/payment_order_query_wechat.go:234-249` 也在成功 fact 后立即取回更新后的 payment_order。
- 结论：宝付主业务的查询恢复不依赖 callback 再入队，也不依赖 `processed_at` 轮询。候选 250 只适用于通用 paid-unprocessed recovery 的排除面，不代表宝付 query 成功会丢 application。

### NP-34. `PaymentRecoveryScheduler` 刻意跳过 Baofu main-business recovery，因为宝付有专用恢复器

- 证据点：`worker/payment_recovery_scheduler.go:133-140` 对 `payment_channel='baofu_aggregate'` 只打印 skip；`worker/baofu_payment_recovery_scheduler.go:412-455` 负责宝付 pending payment recovery；`worker/baofu_payment_recovery_scheduler.go:342-390` 负责宝付 processing share recovery。
- 结论：通用 recovery 没扫到宝付单不是遗漏，而是分流。排查宝付支付/分账时应该看专用 scheduler 与其 target 是否在跑，而不是把 payment_recovery_scheduler 当主入口。

### NP-35. 直连支付 query success 对宝付 verify fee 的 defer 是为了开户续链，不是漏掉事实应用

- 证据点：`logic/payment_order_query_wechat.go:341-381` 在宝付核验费 success 时可先标 paid，再创建 fact/application，但 `shouldDeferDirectPaymentQueryFactApplication` 会有意不立即 apply；`logic/payment_order_query_baofu_verify_fee.go:14-45` 与 `logic/payment_fact_application_baofu_verify_fee.go:54-132` 表明后续还要进入开户 flow 的 verify fee continuation。
- 结论：这条路径的半状态是有意设计，目的在于让核验费支付成功先完成账户状态与续链准备，再由 application 驱动开户。它不是“query 写了事实却忘了应用”的固定洞。

### NP-36. 直连支付 closed/failed 后到账的异常退款路径是刻意 ACK + 告警 + 异步退款

- 证据点：`api/payment_callback.go:850-947` 在 closed/failed payment_order 收到到账时会直接入队 `TaskProcessAnomalyRefund` 或发人工告警，然后 `markNotificationProcessed` 并 ACK 微信。
- 结论：这条路径本来就不是走 payment fact application 的成功态。它需要队列、告警和人工退款协同，不能按“回调处理失败应重投直到成功”的普通支付成功模型来审。

### NP-37. 宝付分账账单的 estimated/calculated 手续费不会覆盖已记录的 actual provider 手续费

- 证据点：`UpsertOrderPaymentFeeLedgerCalculated` 在 existing `amount_source in ('actual_callback','actual_query')` 时保留原 `base_amount/rate_bps/amount/amount_source/status`：`db/query/order_payment_fee_ledger.sql:115-141`；测试 `TestCreateBaofuProfitSharingOrderTxKeepsActualProviderFeeLedger` 覆盖 actual callback 先写、分账账单后计算的场景：`db/sqlc/tx_baofu_profit_sharing_test.go:927-1017`。
- 结论：如果宝付支付 callback 已写 actual fee，后续创建/刷新分账账单的 calculated fee 不会把实际手续费改回估算值。候选 253 关注的是 actual callback/query 之间的后到覆盖，而不是 calculated 覆盖 actual。

### NP-38. 宝付支付 callback 在 fact/ledger/application 落库失败时不会 ACK

- 证据点：`handleBaofuPaymentNotify` 调 `RecordPaymentFact`，任何错误都会返回 `500` + `FAIL`，只有 application 创建成功后才尝试入队并最终返回 `OK`：`api/baofu_callback.go:241-262`。
- 结论：对于宝付支付 callback，fact/ledger/application 创建阶段失败不是“已 ACK 后只能人工处理”的路径；provider 理论上会重投。已落 application 后的 asynq 入队失败才会 ACK，但该类 pending/failed application 可被统一 scheduler 补扫。

### NP-39. 宝付核验费失败/关闭/过期不会把已进入开户后续阶段的 flow 拉回核验费待支付

- 证据点：`applyBaofuVerifyFeePaymentTerminalFailure` 会调用 `MarkBaofuAccountOpeningFlowVerifyFeePending`，SQL 只允许 `state IN ('profile_pending','verify_fee_pending','verify_fee_processing')`：`logic/payment_fact_application_baofu_verify_fee.go:87-119`；`db/query/baofu_account_opening_flow.sql:86-99`。
- 结论：late closed/failed/expired fact 不会把已经 `opening_processing`、`merchant_report_processing`、`applet_auth_pending` 或 `ready` 的开户 flow 回退到 `verify_fee_pending`。这条不是后到失败覆盖成功开户进度的问题。

### NP-40. 宝付核验费 application 入队开户任务失败后，可通过 application 重试继续补排

- 证据点：`BaofuVerifyFeeAsyncContinuation.ContinueAfterVerifyFeePaid` 在 `PrepareOpeningAfterVerifyFeePaid` 返回 `ShouldEnqueueOpening` 时才入队；若入队失败会使 application 标 failed：`worker/task_payment_fact_application.go:36-50`、`logic/payment_fact_application_service.go:215-218`；重试时 `PrepareOpeningAfterVerifyFeePaid` 对 `opening_processing` 返回 `ShouldEnqueueOpening=true`：`logic/baofu_account_onboarding_prepare.go:27-34`。
- 结论：开户任务单次入队失败不是固定断链。只要 payment fact application scheduler 正常扫描 failed application，后续会再次尝试排 `TaskProcessBaofuAccountOpening`。

### NP-41. 宝付核验费支付成功 application 本身不直接外呼开户 provider

- 证据点：`BaofuVerifyFeeAsyncContinuation` 只调用 `PrepareOpeningAfterVerifyFeePaid` 并入队 `TaskProcessBaofuAccountOpening`；测试 `TestProcessTaskPaymentFactApplication_BaofuVerifyFeeSuccessEnqueuesOpeningTask` 断言 `openCalls` 为 0：`worker/task_payment_fact_application.go:36-50`；`worker/task_payment_fact_application_test.go:125-222`。
- 结论：核验费成功事实重试不会在 payment fact application worker 中直接重复外呼宝付开户；外呼发生在独立的 `TaskProcessBaofuAccountOpening` / recovery 链路。

### NP-42. 普通订单支付成功不会覆盖 cancelled/refunded/completed 等非 pending 订单状态

- 证据点：`processOrderPaymentWithQueries` 先 `GetOrderForUpdate`，已 paid 直接幂等返回，非 pending 会报错：`db/sqlc/tx_create_order.go:390-413`；底层 `UpdateOrderStatus` 也要求 `expected_status`：`db/query/order.sql:232-240`。
- 结论：同一 payment_order 成功事实重试不会把已取消/退款/完成订单重新置 paid。若 provider late success 遇到本地 order 非 pending，application 会失败/重试或等待人工处理，不是静默覆盖订单状态。

### NP-43. 新预约加菜 adjustment 支付成功路径有 reservation/adjustment 锁和状态 guard

- 证据点：`applyPaidReservationAdjustmentWithQueries` 锁 payment_order、adjustment、reservation；只允许 reservation `paid/confirmed/checked_in` 且 `CookingStartedAt` 为空时应用，并把 adjustment 标 `applying/applied`：`db/sqlc/tx_reservation_adjustment.go:230-347`。
- 结论：当前新 adjustment 链路不是“支付成功后无条件改菜/加 prepaid”。候选 256 只保留在未命中 adjustment 的 legacy fallback 入口。

### NP-44. 订单/预约支付成功 outbox 重试已考虑 processed_at 后的幂等回放

- 证据点：订单分支在 `ProcessPaymentSuccessTx` 返回未 processed 但 payment_order 已有 `processed_at` 时，会回读 order 并设置 `Processed=true` 以重建 outbox：`logic/payment_fact_application_service.go:357-369`；预约分支同样把已 processed payment_order 视为可创建 reservation success outbox：`logic/payment_fact_application_service.go:423-429`；测试 `...OrderPaymentOutboxRetryAfterProcessed` 与 `...ReservationPaymentOutboxRetryAfterProcessed` 覆盖。
- 结论：domain mutation 成功、outbox/terminal mark 失败后的重试不是固定卡死；只要 application 仍可重试，outbox 可通过 `CreatePaymentDomainOutboxOnce` 幂等补建。残余风险仍是 processing 悬挂和 scheduler 健康（候选 174-182/257）。

### NP-45. refund failed/closed fact 不会覆盖已 success 的 refund_order

- 证据点：`UpdateRefundOrderToFailed/Closed` 只允许 pending/processing：`db/query/refund_order.sql:166-178`；consumer 在本地已 terminal 且不是当前 fact 目标状态时直接 return nil：`logic/payment_fact_application_service.go:553-592`、`:678-717`；测试 `...OrderRefundFailedFactDoesNotRegressSuccessfulRefund`、`...ReservationRefundFailedFactDoesNotRegressSuccessfulRefund` 覆盖。
- 结论：普通 late failed/closed 不会把本地已成功退款回退为失败/关闭。剩余风险是反方向 success-after-failed/closed（候选 258），以及异常 outbox/通知语义。

### NP-46. 部分退款不会把 payment_order 标成 refunded

- 证据点：`maybeMarkPaymentOrderRefunded` 只有累计成功退款额 `>= paymentOrder.Amount` 才 `UpdatePaymentOrderToRefunded`：`logic/payment_fact_application_service.go:805-817`；测试 `TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderRefundSuccessDoesNotMarkPartialRefundedPaymentAsRefunded` 覆盖。
- 结论：单笔部分退款 success fact 不会直接把原支付单改为 fully refunded；该项不是时序覆盖问题。

### NP-47. 宝付 refund callback 落事实失败不会 ACK，已落 application 后入队失败可由 scheduler 补扫

- 证据点：`handleBaofuRefundNotify` 在加载 refund/payment 或 `recordBaofuRefundCallbackFact` 失败时返回 `FAIL`；只有 fact/application 创建成功后才 ACK，入队只是 best-effort：`api/baofu_callback.go:500-535`。统一 scheduler target 已包含 order/reservation refund application。
- 结论：宝付退款 callback 的落库阶段不是 ACK 后永久丢失模型；残余风险集中在 application 处理失败、processing 悬挂、以及 success-after-failed/closed 语义。

## 本轮复核补记

- `219` 已并入 `NP-7`，它是审计残留，不是时序风险。
- `220` 仍保留为高风险窗：缺少 `out_request_no` 时会回落到 `contract_no -> latest flow`。
- `221`/`222`/`223` 仍需继续看并发和 provider 幂等语义，当前没有足够证据把它们降级。
- `205` 已进一步收窄：不是单靠 application-level recovery 的固定孤儿洞，真正残余只剩源头重试/恢复也失效的窄窗。
- `230` 已收窄为业务资格时序窗，不作为押金透支问题表述。
- `231` 有回调和 stuck query 恢复，不按永久丢账处理；后续要确认产品可接受的半状态时长。
- `232`/`233` 是 rider deposit refund 本轮更值得优先深挖的两个分流/语义错配点。
- `65` 已具体化为 `235`：unknown command 不被 submitted scanner 覆盖，关键窗口是 claim unknown 后外呼前/外呼不确定。
- `66` 已复核仍成立并具体化为 `238`：不同幂等键并发提现没有本地 pending freeze。
- `68` 已收窄到 `NP-22`：本地终态 first-wins 较明确，剩余只需确认 provider 对 returned/failed/succeeded 的业务语义。
- `69` 已具体化为 `236` 与 `NP-23`：order 已写入 provider 信息后 command 可修复，order 写入失败则 command 可能长期 unknown。
- `10` 已收窄到 `NP-25/NP-26`：分账发起前 active refund 与净额漂移会被 ready list + prepare 事务复核挡住；真正残余是 prepare 后 processing 与后续退款的业务顺序。
- `52` 仍作为“完成后分账任务交付窗口”保留，但宝付 recovery 会扫描 ready payment_order 并补建账单/任务，后续优先看 scheduler 健康与监控，不把它当直接重复分账风险。
- `53` 已具体化为 `239/240/241`：prepare 后到 provider/fact 的多段状态机仍存在 processing 空命令态、command/order 引用不一致、rejected/unknown 长期 processing 三类窗口。
- `59` 保留但已收窄：`sharing_order_id` 缺失不等于不能恢复，因 query 有 out_order_no fallback（`NP-29`）；残余是 `ORDER_NOT_EXIST` 一致性窗口。
- `60` 保留为金额口径/人工对账问题：success amount 缺失会在 `RecordShareFact` 被拒，金额不一致会在 application 拒绝，不会误标 finished。
- `61/62/201/202` 已收窄为 `245/246/NP-31`：`profit_sharing_returns` 更像旧状态机残留，是否仍是生产风险取决于是否有非终态旧数据或旧 fact 流。
- `203` 仍保留：宝付分账本地终态有 processing guard，不会普通互相覆盖；但 success-after-failed / failed-after-finished 的运营语义仍需和 provider 语义确认。
- `174-182` 公共 payment fact/application 底座问题仍保留；本轮新增的是入口差异：direct callback 入队失败可由 scheduler 补扫（`NP-32`），但 closed/failed 后到账和金额异常 ACK 后主要依赖异常退款任务/告警（`247/248/NP-36`）。
- `170/208/209/210` 已进一步收窄：timeout/query/callback 事实路径大多能落 application 并被 scheduler 补扫；剩余重点是“先改 payment_order paid，再事实应用滞后”的观察窗口，以及 abnormal/mismatch 路径不走正常业务 application。
- `181` 已收窄到 `NP-34`：通用 recovery 跳过 Baofu 是刻意分流；宝付支付/分账要看 `BaofuPaymentRecoveryScheduler`。
- `182` 仍保留：入队失败依赖 scheduler 只覆盖 pending/failed target，不覆盖 processing/stale 或未知 target；但本轮未发现 direct/Baofu 主支付已落 application 后入队失败会永久丢。
- `206` 保留为多来源事实去重/下游幂等问题；本轮确认 direct query、callback、manual recovery 的 dedupe key 形态不同，后续 consumer 侧需按多 fact 顺序验证。
- `30` 已收窄为 `251/252/NP-37/NP-38`：宝付支付 fact/fee/application 确实非事务，但 callback 落库阶段失败会 FAIL 并可重投；residual 风险集中在 fact 已存在但 application 缺失时 scheduler 无法反向补建、以及 actual fee 多事实后到覆盖。
- `249/NP-35` 进一步细化为 `254/NP-39-NP-41`：核验费 defer 是设计，但 payment_order processed 与 opening flow/task 续链之间仍有可观察半状态；后到 failure 不会回退后续 flow，开户外呼由独立任务/恢复链路承担。
- `6/7` 已具体化为 `255`：普通 reservation payment success 的 reservation 状态写入没有 expected-old-status；订单支付分支有锁和 pending guard（`NP-42`）。
- `8` 已收窄为 `256/NP-43`：新 reservation adjustment 链路有锁与状态 guard；残余主要是未命中 adjustment 的 legacy reservation_addon fallback。
- `5/174-182` 在订单/预约成功 consumer 侧具体化为 `257/NP-44`：domain mutation 与 outbox/fact/application terminal mark 分段，重试可补齐，但仍需要 application scheduler 与 processing 悬挂监控。
- `9/54/55/200` 在退款 consumer 侧具体化为 `258-260/NP-45-NP-47`：failed/closed 不覆盖 success 已有保护；真正残余是 success-after-failed/closed、reservation prepaid 扣减补偿、以及直连旧 worker 与 fact consumer 双轨语义。
- `50` 已确认：scene 解析对 `-` 的截断是确定性协议错位，不是单纯输入校验；`51` 已确认：二维码文件与桌台指针分离，回写失败后当前代码没有自愈分支；`52`/`53` 已确认非问题，普通编辑入口仍会进入事务清 QR，而海报函数只是纯参数拼接。
- `EnvVersion` 仍保留为证据不足项：当前已读本地代码和可抓到的官方页面片段都不足以把 `generateTableQRCode` 的 `develop` 常量判成明确 defect，因此先不升格，后续若补到官方文档/产品意图再决定是否单独开 finding。

## 当前最值得优先深挖的方向

1. 支付创建并发唯一性：用真实 schema 和并发测试确认主业务 active payment 是否缺少 DB 级唯一约束。
2. 宝付 payment_order 终态乱序：特别是 closed/failed 后 success fact 晚到是否有自动退款/人工对账闭环。
3. 支付关闭/成功乱序后的补偿：特别是 paid payment_order 但 order/reservation 已取消的处理。
4. 预约状态机：把 `table_reservations` 的所有写入口收拢成状态迁移矩阵，重点验证支付成功、timeout、桌台释放、堂食开关台这些旁路是否覆盖终态。
5. 退款/分账/退分账事实乱序：用 terminal fact 顺序表验证 first-terminal-wins 是否业务正确。
6. `scheduler/data_cleanup.go` 的 stale delivery refund 分支：`p.Status == "success"` 已确认与 payment_order 成功枚举 `paid` 不一致，优先查生产 cancelled+paid+无 refund_order。
7. cloud printer `UpdatePrintLogStatus` 调用点是否可能覆盖终态，尤其飞鹅云 callback 与打印任务失败路径。
8. media upload session `CompleteUploadSession` 是否能覆盖 expired。
9. Baofu 开户/报备状态机单独展开，验证 callback/recovery/edit/retry 的 expected-state 条件。
10. Payment fact 多步非事务链路：逐 consumer 验证 processing stale、fact/application/retry/domain side-effect/outbox 一致性。
11. 宝付开户/报备双 scheduler：验证 report 表 terminal 后 opening flow、payment config、merchant activation 的最终一致性和滞后窗口。
12. 云打印终态语义：统一确认飞鹅云 success 覆盖失败、易联云 first-terminal-wins、poll expiry 的业务期望是否一致。
13. 确认收货/自动完成：验证并发输掉方错误映射、状态日志补偿、分账任务恢复与 `asynq.Unique(30s)` 的实际交付语义。
14. 退款终态与业务联动：重点验证 late success after failed/closed、预约 prepaid_amount 补偿、旧 worker 与 fact application 的冲突语义差异。
15. 宝付分账/退分账：验证 `ORDER_NOT_EXIST` 转 failed 的 provider 一致性窗口、success amount 口径、以及 legacy `profit_sharing_returns` 是否还有非终态生产数据。
16. 宝付提现：盘点 submitted/unknown command 滞留、processing withdrawal 查询失败、以及 command outcome 与 withdrawal_order 终态不一致的生产数据。
17. 预约库存/调整链路：验证 no_show/cancel 状态与库存释放分段提交、addon paid fact 与 adjustment 关闭/过期的并发语义。
18. OCR 状态机：验证 job succeeded/failed 与 owner OCR JSON 投影分离、rider 同媒体多 job 乱序、group application DB 层缺 draft guard、以及 approved 媒体 OCR 入队恢复。
19. 媒体上传/审核：验证 upload session 完成与过期并发、asset/session 半状态、moderation pending 补偿、callback 乱序覆盖和 OCR 联动半状态。
20. 入驻审核：验证 merchant approved 后 review/credential repair、rider reject 后 run complete 失败、rider queued run 补派发、summary run_id 单调性和通知副作用幂等。
21. 配送状态机：优先验证 `UpdateDeliveryToCancelled` 无 old-status guard、`CancelOrderTx` 锁后不复核 delivery 状态、stale delivery cleanup 与抢单/完成并发、`p.Status == "success"` 自动退款筛选、delivery_pool 过期 list/count/grab 语义不一致，以及食安 pause/售后投影与主状态机分离。
22. 索赔/追偿动作链：验证 claim 创建事务内订单状态复核、claim payout action 终态覆盖、paid claim 后 recovery/action 补齐，以及 action recovery scheduler 是否覆盖所有半状态。
23. 追偿争议链：验证 dispute/recovery 双事实源在 submit/review/result-effect 三段之间的 expected-state 绑定、approved-without-release 合法性、result task 入队失败恢复，以及 compensation/release/action 后到覆盖。
24. 自动追偿争议裁决：验证自动裁决依据的 behavior decision 是否有版本 guard，terminal dispute result effect 重放是否完全幂等，以及 release/payout action 反查是否稳定。
25. 追偿暂停/release：优先验证无 reason guard release 后到覆盖其它暂停、pending/disputed overdue 扫描语义、以及 block/release action 重复执行是否完整幂等。
26. behavior action 执行框架：验证 running stale、终态无 expected-status 覆盖、block/release 对暂停字段 owner guard、open/release 对 recovery 状态 guard、以及 claim_recovery_events 并发重复写。
27. recovery dispute result effects：验证 approved result 重放处罚是否按 dispute/claim 幂等、通知失败是否需要 outbox，以及 submitted 自动裁决失败后的人工/定时补偿入口。
28. 配送围栏自动推进：重点验证 dwell event 与状态推进的事务边界、assigned -> picking -> picked 自动双阶段、event 去重键是否应绑定状态窗口，以及自动送达使用的 rider 位置快照是否最新。
29. 骑手位置上报：验证轨迹/current/region/geofence 半状态、current location 时间戳语义、以及 rider 多 active delivery 是否有强约束。
30. 配送告警/通知：验证 delayed 字段是否兼任通知去重、operator alert ledger 是否表达成功通知、recipient 级通知幂等，以及配送状态通知失败是否有补偿。
31. 通知/ETA/timeout 公共副作用：验证通知任务 at-least-once 是否会重复、无 distributor fallback 是否允许、抢单后 ETA/广播/通知半状态、以及订单 timeout 双入口与支付成功竞态。
32. payment-order timeout：验证 provider query/close/local close/business cancel 多段提交，远端 paid fact application 入队恢复，USERPAYING 与 order timeout scheduler 的交叉，以及 remote close 后本地失败的收敛。
33. payment fact application 恢复面：验证 scheduler target 白名单、processing 悬挂回收、outbox payload drift、以及 release action 派发失败后的补偿链路。
34. payment domain outbox：验证 processing 悬挂回收、dispatch 后 mark published 失败的重复副作用、订单支付成功 outbox 中打印/通知串行阻断、以及异常告警 best-effort 是否有独立补偿。
35. payment fact consumer：验证普通 reservation 支付覆盖终态、reservation_payment 半状态、reservation_addon 旧路径、reservation refund prepaid 扣减半状态、以及 profit_sharing_return 终态互相覆盖。

## 未完成/待继续读取

- Baofu account opening/report/query/recovery 对应 SQL 和 worker 已补读主体链路；仍需用测试/沙箱验证 provider 对同 open_trans/report_no 重复提交的真实语义，以及 merchant report 初始提交在 command-before-upsert 失败窗口是否会留下孤儿 command；开户回调缺 out_request_no 时的 contract_no -> latest flow fallback 已确认存在保护，但仍可能把旧回调落到新 flow，属于继续观察的高风险窗。报备恢复并发提交、开户 flow 创建的读后写竞态、以及 `opening_processing` 无 claim 字段导致的双外呼窗口也已确认存在，需要后续看能否用恢复/冲突回读进一步收敛。
- Baofu merchant report recovery 已确认有 report-level scheduler 与 opening-flow-level scheduler 两条路径；仍需验证 report terminal 后 flow/merchant activation 的最终一致性时延。
- Baofu profit sharing 主链路本轮已复核账单创建、ready list、prepare tx、worker 外呼、recovery query、fact application、旧 return consumer；后续只剩 provider 语义/生产数据确认：同 `out_order_no` 重复 share-after-pay 是否幂等、`ORDER_NOT_EXIST` 查询一致性窗口、success amount 字段稳定性，以及 `profit_sharing_returns` 是否仍有非终态历史数据。
- `task_process_payment.go` direct refund 与 Baofu refund 的 worker 更新分支已补读主要路径，但尚未对照 `refund_order.sql` 逐条验证 terminal conflict。
- `cloud_printer_status_poll_scheduler.go`、`print_log.sql`、`task_print_order.go`、`task_print_attempt.go` 使用 `UpdatePrintLogStatus` 的上下文已补读；仍需结合 provider 文档/生产日志验证 callback 重试语义和飞鹅云 success 覆盖 failed 是否符合产品意图。
- 确认收货 helper 的两个已知调用页面未见 submitting lock；仍需核查 backend confirm 是否重复副作用安全。
- `CreateExternalPaymentFact` 与 `CreateExternalPaymentFactApplication` SQL 冲突语义、application claim/mark、scheduler/recovery/outbox 主链路已补读并追加候选；payment domain outbox publisher/scheduler/通知/告警/no-show alert 主链路已补读并追加候选；payment fact consumer 第一批（订单/预约支付、预约 addon、退款、分账/退分账、宝付 verify fee 调用点）已补读并追加候选；direct/Baofu fact 创建入口已补充 `247-253` 与 `NP-32-NP-38`。仍需继续核查 Baofu verify fee flow SQL，以及 payment fact consumer 的订单/预约状态覆盖边界。
- 预约状态机已补充第一批候选；仍需完整复核 `logic/dining_session.go`、`tx_dining_session.go`、`api/table.go` 的前置校验和锁边界。
- OCR job/owner/队列链路已补读并追加第一批候选；仍需核查 media moderation 回调、upload session 状态机、以及 onboarding review 是否消费 owner OCR JSON 或 ocr_jobs 作为事实源。
- media upload session/media asset/media moderation 第一批候选已补充；仍需查是否存在 pending moderation/upload session 的 scheduler 或运营补偿入口。
- onboarding review/credential governance 第一批候选已补充；仍需进一步核查 credential ledger SQL 的唯一约束和 restore tx 的状态条件。
- delivery/食安 pause/claim compensation 候选已补充；`CancelOrderTx`、`payment_order.sql`、`logic/delivery_grab.go`、`delivery_pool.sql`、`tx_delivery.go`、`tx_food_safety.go`、`tx_takeout_order.go`、`logic/cancel_order.go`、`logic/confirm_order.go`、`scheduler/takeout_auto_complete.go`、`tx_claim_behavior.go`、claim payout/action recovery scheduler 已读关键路径。recovery dispute submit/review/result worker 与 automatic resolution 第一批候选已补充。claim recovery suspension/overdue/block/open/release action、claim recovery event helper、recovery dispute result effects 第一批候选已补充。delivery geofence event/auto status、位置上报入口、delivery API 通知/告警、通知幂等/ETA/订单 timeout、payment-order timeout 第一批候选已补充。delivery damage 当前未见明确写入口但 SQL 形状已记录。仍需继续读取剩余高风险 worker/scheduler，尤其支付 fact application 与订单/预约状态应用、提现/分账恢复残余、以及前端/客户端是否会重复触发关键 API。
- 现有测试覆盖：还未跑测试，也未用集成测试模拟弱顺序事件。

## 恢复上下文提示

如果会话压缩或切换，从这里继续：

1. 先读本文件。
2. 再读 `AGENTS.md`、`.github/copilot-instructions.md`、`.github/README.md`、`locallife/AGENTS.md`。
3. 继续从“当前最值得优先深挖的方向”开始，不要把上面的候选项当已确认 bug。
4. 保持只读，直到用户明确要求修复或深入验证某条。
