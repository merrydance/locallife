# 微信支付调用点矩阵（TASK-PAY-001）

日期：2026-04-25

## 1. 盘点结论

当前系统已经把直连支付和平台收付通客户端分开，这是正确边界。但“外部支付事实”和“业务状态推进”仍然混在多个 handler、worker、service、scheduler 中。

最需要优先收口的不是重写微信客户端，而是统一三件事：

- 提交接口返回只代表 command accepted 或同步拒绝。
- 回调、查询、恢复调度都应归一到同一个 terminal fact 入口。
- 业务域只消费 terminal fact，不直接依赖微信提交返回做不可逆状态变更。

## 2. 高风险优先项

### P0-1 单笔支付超时缺少远端查单兜底

`worker/task_payment_timeout.go` 的 `ProcessTaskPaymentOrderTimeout` 对普通 `payment_orders` 到期后直接把本地 pending 单关闭，并继续取消业务订单；它不像 `worker/task_combined_payment_timeout.go` 那样先 `QueryCombineOrder` 核对微信远端状态。

影响：如果用户实际已支付但回调延迟或丢失，本地可能先关闭业务订单，后续回调只能走异常退款/补偿路径。收付通合单已经有远端查询兜底，普通收付通单笔支付、直连押金充值、直连追偿支付也应补齐同类查询策略。

归属任务：TASK-PAY-009，影响 TASK-PAY-004、TASK-PAY-006。

### P0-2 骑手押金退款仍在提交路径处理终态

`logic/rider_deposit_refund_service.go` 的 `SubmitWithdrawal` 在 `CreateRefund` 返回 `SUCCESS` 时立即 `ResolveRefund`，在 `PROCESSING` 时标记 processing。虽然当前微信 create 返回 PROCESSING 已修过契约校验，但这条链路仍把提交响应和终态处理放在一个业务 service 中。

影响：押金域样板重构时应保留当前 stale credit reconciliation 能力，但把退款创建和退款终态结算拆开，让回调/query/manual reconciliation 都进入同一个押金退款 terminal handler。

归属任务：TASK-PAY-004，依赖 TASK-PAY-002。

### P0-3 支付成功总处理器承担过多业务状态和副作用

`worker/task_process_payment.go` 的 `ProcessTaskPaymentSuccess` 是多个业务类型的支付成功入口，同时还串接押金、订单、预订、追偿、分账触发、接收方同步等后续动作。

影响：它目前是事实、业务状态、跨域副作用的汇合点。后续应拆成 payment fact terminalizer + domain handler + outbox/worker 副作用。

归属任务：TASK-PAY-002、TASK-PAY-004、TASK-PAY-005、TASK-PAY-006。

## 3. 调用矩阵

| 能力组 | 业务 owner | 提交/查询调用点 | 当前同步返回用途 | 当前终态来源 | 幂等键/外部键 | 主要缺口 | 后续任务 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 直连小程序支付：骑手押金充值 | 骑手押金域 | `api/rider.go` `rechargeRiderDeposit` 调 `directPaymentClient.CreateJSAPIOrder` | 生成 prepay/pay params，本地 payment_order 仍 pending | `api/payment_callback.go` `handlePaymentNotify` 记录 direct payment fact，并由 `PaymentFactApplication` 调用 `ProcessPaymentSuccessTx`；`PaymentFactApplicationScheduler` 负责恢复，旧 `ProcessTaskPaymentSuccess` / `PaymentRecoveryScheduler` 已切出 rider_deposit | `out_trade_no`, `payment_order_id`, `business_type=rider_deposit` | 充值终态主链已完成样板切换；timeout 前仍未先查微信远端，manual reconciliation / query 入口仍待统一进 fact 恢复矩阵 | `TASK-PAY-004` 已完成；剩余 `TASK-PAY-005`, `TASK-PAY-009` |
| 直连小程序支付：商户/骑手追偿付款 | 追偿域 | `logic/claim_recovery_payment.go` `CreateMerchantClaimRecoveryPayment` / `CreateRiderClaimRecoveryPayment` 调 `CreateJSAPIOrder` | 生成 pay params，本地 payment_order pending | `handlePaymentNotify` 记录 success fact 并立即触发 `claim_recovery_domain` application；legacy `ProcessTaskPaymentSuccess` 对 claim recovery 已 SkipRetry | `out_trade_no`, attach 中 claim/recovery 信息, `business_type=claim_recovery` | 支付成功 owner path 已切到 fact/application；剩余是 timeout 前查单与更广义恢复矩阵 | TASK-PAY-006 已完成；剩余 TASK-PAY-009 |
| 直连退款：骑手押金提现 | 骑手押金域 | `logic/rider_deposit_refund_service.go` `SubmitWithdrawal` 调 `CreateRefund` | create 返回只表示 accepted/processing；同步错误会失败/对账 | `handleRefundNotify` / `RefundRecoveryScheduler.queryRefundStatus` 统一写 direct refund fact，再由 `PaymentFactApplication` 推进 `ResolveRefund`；manual reconciliation 当前仍在 already-refunded 错误分支 | `out_refund_no`, `refund_id`, source `out_trade_no`, `rider_deposit_credit` | 支付/退款终态单源已收口；manual reconciliation 尚未并入同一 fact 入口，receiver lifecycle 已拆出主链但仍需独立 closeout 告警尾项 | `TASK-PAY-004` 已完成；剩余 `TASK-PAY-005` |
| 直连商家转账：平台赔付到零钱 | 追偿/赔付域 | `worker/task_claim_refund.go` `ExecuteClaimPayoutAction` 调 `transferClient.CreateTransfer`，随后 `QueryTransferByOutBillNo` | create 返回 accepted 后立即 query；query 结果若终态则推进赔付 action | `handleMerchantTransferNotify`; `ClaimPayoutRecoveryScheduler`; `reconcileClaimPayoutTransfer` 主动 query | `out_bill_no=claimpayout<action_id>`, `transfer_bill_no`, behavior_action_id | 状态处理做得相对接近事实模型，但仍是局部实现，未纳入统一 terminal fact/outbox | TASK-PAY-002, TASK-PAY-010 |
| 收付通普通支付：订单 | 订单域 | `logic/payment_order_service.go` `createOrderEcommercePayment` 调 `CreatePartnerJSAPIOrder` | 生成 prepay/pay params，本地 payment_order pending | `handleEcommercePaymentNotify` / `handlePartnerPaymentNotification` / `handleCombinePaymentNotify` 与 `PaymentRecoveryScheduler` 统一写 order paid fact，并由 `order_domain` application 推进支付成功后的订单激活与分账 outbox | `out_trade_no`, `transaction_id`, `sub_mchid`, `business_type=order` | merchant transaction paid 主链已收口；普通单 timeout 前主动查单策略与人工 reconciliation 仍待统一到恢复矩阵 | `TASK-PAY-006` 已完成；剩余 `TASK-PAY-009` |
| 收付通普通支付：预订 | 预订域 | `logic/payment_order_service.go` `createReservationEcommercePayment` 调 `CreatePartnerJSAPIOrder` | 生成 prepay/pay params，本地 payment_order pending | 单笔/合单 paid callback 与 `PaymentRecoveryScheduler` 统一写 reservation paid fact，并由 `reservation_domain` application 推进预订状态、payment_mode 分支与 payment outbox | `out_trade_no`, `reservation_id`, `payment_mode`, `business_type=reservation` | 预订 paid 主链已收口；manual reconciliation 与更广义恢复节奏仍待纳入统一恢复矩阵 | `TASK-PAY-006` 已完成；剩余 `TASK-PAY-009` |
| 收付通合单支付 | 订单域 | `logic/combined_payment_service.go` `CreateCombinedPaymentOrder` 调 `CreateCombineOrder` | 生成 combine prepay/pay params，本地 combined/payment orders pending | `handleCombinePaymentNotify` 与 `ProcessTaskCombinedPaymentOrderTimeout` query replay 命中 paid 后统一写 order/reservation paid fact，再由各自 domain application 推进 | `combine_out_trade_no`, 子单 `out_trade_no`, `transaction_id` | 合单 paid replay 已完成 owner cutover；timeout 前更早的远端查单策略仍待统一 | `TASK-PAY-006` 已完成；剩余 `TASK-PAY-009` |
| 收付通退款：订单/预订/异常退款 | 订单/预订售后域 | `logic/refund_service.go` `CreateRefundOrder` / `worker.ProcessTaskInitiateRefund` / `ProcessTaskAnomalyRefund` 调 `CreateEcommerceRefund` | create 成功后本地 refund_order processing；同步错误会映射失败 | `handleEcommerceRefundNotify` 与 `RefundRecoveryScheduler.queryRefundStatus` 统一写 ecommerce refund fact，再由 `order_domain` / `reservation_domain` application 单源推进退款成功、关闭、异常 outbox；`ProcessTaskRefundResult` 对商户交易只保留显式保护 | `out_refund_no`, `refund_id`, `payment_order_id`, `sub_mchid` | refund terminal 主链已收口；退款前分账回退链路仍与退款 create 过程耦合，manual reconciliation 仍待统一 | `TASK-PAY-006` 已完成；剩余 `TASK-PAY-007`, `TASK-PAY-009` |
| 分账请求 | 分账域 | `worker/task_process_payment.go` `ProcessTaskProfitSharing` 调 `CreateProfitSharing`; `finishProfitSharingOrder` 调 `FinishProfitSharing` | create/finish 返回后标记分账 order processing 或结果 | `handleProfitSharingNotify`; `ProcessTaskProfitSharingResult`; `ProfitSharingRecoveryScheduler` query/retry | `out_order_no`, `transaction_id`, `profit_sharing_order_id`, `sharing_order_id` | 请求、结果、解冻、失败重试都在 worker 中，尚未统一成 payment facts；接收方生命周期和业务支付完成有耦合 | TASK-PAY-007, TASK-PAY-005 |
| 分账回退 | 分账/退款域 | `logic/refund_service.go` 分账回退前置处理；请求链路中保留的 `ProcessTaskProfitSharingReturnResult` 查询入口 | 回退处理中延迟等待；终态以 query/callback fact application 为准 | `ProfitSharingRecoveryScheduler` stuck processing 直接 `QueryProfitSharingReturn` 并写 `profit_sharing_domain/profit_sharing_return` fact application，不再投递 legacy `ProfitSharingReturnResultPayload` recovery fallback | `out_return_no`, `out_order_no`, `refund_order_id` | stuck recovery 已切到 fact application；请求链路遗留 result worker producer 与 manual reconciliation 仍需后续独立切片清理 | TASK-PAY-007, TASK-PAY-009, TASK-PAY-010 |
| 分账金额查询 | 分账/运营能力 | `api/profit_sharing_capability.go` 调 `QueryProfitSharingAmounts` | 查询型接口，返回当前微信可分账金额 | 无业务终态推进 | `transaction_id` | 查询接口不应推进业务状态；后续可纳入 observability，不是迁移首批 | TASK-PAY-010 |
| 商户进件提交 | 商户进件域 | `logic/ecommerce_applyment_submission.go` `SubmitEcommerceApplyment` 调 `CreateEcommerceApplyment` | 提交创建本地 applyment，微信返回申请单信息/受理状态 | `handleApplymentStateNotify` 与 `ApplymentRecoveryScheduler` 在 merchant applyment 命中 `FINISH + sub_mchid` 时统一写 applyment success fact，再由 `applyment_domain` application 执行激活并通过 `applyment_activated` outbox 发送通知；命中 `REJECTED` / `FROZEN` / `CANCELED` 时统一写 applyment terminal fact，再由 `applyment_domain` application 更新 owner status 并通过 `applyment_terminal_state_ready` outbox 发送通知；命中 `ACCOUNT_NEED_VERIFY` / `APPLYMENT_STATE_TO_BE_CONFIRMED` / `APPLYMENT_STATE_TO_BE_SIGNED` 时统一写 processing fact，再由 `applyment_domain` application 更新 guidance 字段并通过 `applyment_pending_state_ready` outbox 发送待处理通知 | `out_request_no`, `applyment_id`, `sub_mchid` | 进件是长流程，提交成功绝不等于可收款；merchant applyment 的 success / terminal / pending guidance owner path 已收口到 fact/application/outbox，剩余是 settlement / withdraw 与更广义恢复矩阵 | `TASK-PAY-008` applyment 三块切片已完成；剩余 `TASK-PAY-008`, `TASK-PAY-009` |
| 结算账户查询/修改 | 商户进件/结算账户域 | `api/settlement_account.go` 调 `QuerySubMerchantSettlement`, `ModifySubMerchantSettlement`, `QuerySubMerchantSettlementApplication` | 查询/提交修改申请 | `ModifySubMerchantSettlement` 的 accepted / rejected 同步返回已写入 `external_payment_commands`；`QuerySubMerchantSettlement` 与 `ApplymentSettlementVerificationScheduler` 现在统一记录 settlement query fact，并由 `settlement_domain` application 单写 `ecommerce_applyments.settlement_verify_*`；`QuerySubMerchantSettlementApplication` 现在也统一记录 settlement application fact，并由 `settlement_domain` application 单写 `merchant_payment_configs.latest_settlement_application_no / latest_settlement_application_submitted_at` | `sub_mchid`, `application_no` | settlement verify truth 与 settlement application audit tracking 都已收口到 fact/application；剩余是提现、注销提现，以及更广义 recovery matrix | `TASK-PAY-008` settlement submit command slice、settlement account current-status owner path、settlement application audit owner path 已完成；剩余 `TASK-PAY-008`, `TASK-PAY-009` |
| 收付通商户提现 | 商户资金域 | `api/merchant_finance.go` `createMerchantAccountWithdraw` 调 `CreateEcommerceWithdraw` | 提交预约提现，返回 out_request_no/withdraw_id 后本地 record processing | `handleEcommerceWithdrawNotify`、withdraw detail live query、create-error 后的即时 query backfill 与 `ProcessTaskMerchantWithdrawResult` 现在统一写 withdraw fact，并由 `merchant_funds_domain` application 单写 `withdrawal_records.status / reason`；`withdraw_id` 继续单独回填到 `withdrawal_records.account_info`；recovery scheduler 只扫描 `updated_at` 早于 5 分钟前的 pending record，result worker 对 pending 查询尝试触碰 `updated_at` 作为节流标记 | `out_request_no`, `withdraw_id`, `sub_mchid`, `withdrawal_record_id` | merchant withdraw owner path 与恢复查询频率矩阵已收口到 fact/application；剩余仅是更广义 manual reconciliation 是否产品化 | `TASK-PAY-008` merchant withdraw owner path 已完成；`TASK-PAY-009-F` 已完成 |
| 收付通商户注销提现 | 商户注销/资金域 | `api/merchant_cancel_withdraw.go` 调 `ValidateEcommerceCancelWithdraw`, `CreateEcommerceCancelWithdraw` | 资格校验是同步判断；创建申请只确认 submitted/processing 与 command accepted | create 后即时 query、详情页 live query 与 `ProcessTaskMerchantCancelWithdrawResult` 现在统一写 cancel withdraw fact，并由 `merchant_funds_domain` application 单写 `merchant_cancel_withdraw_applications.cancel_state / withdraw_state / latest_query_response` 等 query truth；create handler 仅保留 `submitted_at` 标记与 accepted command 记录；recovery scheduler 只扫描 `COALESCE(last_query_at, created_at)` 早于 5 分钟前且尚未终态的申请 | `out_request_no`, `applyment_id`, `sub_mchid`, `merchant_cancel_withdraw_application_id` | merchant cancel withdraw owner path 与恢复查询频率矩阵已收口到 fact/application；剩余仅是更广义 manual reconciliation 是否产品化 | `TASK-PAY-008` 已完成；`TASK-PAY-009-G` 已完成 |
| 消费者投诉 2.0 | 客诉/合规域 | `wechat/complaint.go` `ListComplaints`, `GetComplaintDetail`, `RespondComplaint`, `CompleteComplaint`; `api/complaint.go` 调用 | 查询/回复/完结动作，部分为同步 accepted | `handleComplaintNotify`; 另有主动查询列表/详情 | `complaint_id`, `transaction_id`, `out_trade_no`, `sub_mch_id` | 不是资金终态，但会影响退款/商户治理；应作为 operational fact，避免和支付终态混用 | TASK-PAY-010 |
| 商户违规通知 | 合规域 | `api/violation.go` 创建/更新通知配置；`handleViolationNotify` 解密并持久化违规事件 | 配置接口同步返回；通知为异步事实 | `handleViolationNotify` | `notification_id`, `record_id`, `sub_mchid` | 不属于支付资金链路，但与微信收付通回调验签/幂等基础设施共用，应纳入观测门禁 | TASK-PAY-010 |
| 收付通结算事件通知 | 订单/分账结算域 | `api/payment_callback.go` `handleOrderSettlementNotify` 解密 settlement notification | 无提交，纯异步通知 | settlement webhook | `transaction_id`, `out_trade_no`, settlement event fields | 需要确认是否推进任何业务状态，若只是记录，应归为 operational fact | TASK-PAY-010 |

## 4. 回调与恢复入口清单

### 回调入口

| 入口 | 能力 | 当前处理方式 | 迁移目标 |
| --- | --- | --- | --- |
| `api/payment_callback.go` `handlePaymentNotify` | 直连支付成功 | 解密、幂等 claim、查 payment_order、入队 `ProcessTaskPaymentSuccess` | direct payment fact terminalizer |
| `api/payment_callback.go` `handleEcommercePaymentNotify` / `handlePartnerPaymentNotification` | 收付通单笔支付成功 | 解密、归属校验、查 payment_order、记录 order/reservation paid fact、入队 payment fact application；closed/failed 异常到账会触发异常退款 | 已切到 ecommerce payment fact terminalizer |
| `api/payment_callback.go` `handleCombinePaymentNotify` | 收付通合单支付成功 | 解密、归属校验、对 paid 子单记录 order/reservation paid fact、入队 payment fact application，并只在全部正常时标记合单主单 paid | 已切到 combine payment fact terminalizer |
| `api/payment_callback.go` `handleRefundNotify` | 直连退款结果 | 解密、归属校验、入队 `ProcessTaskRefundResult` | direct refund fact terminalizer |
| `api/payment_callback.go` `handleEcommerceRefundNotify` | 收付通退款结果 | 解密、归属校验、查 refund/payment objects、记录 order/reservation refund fact、入队 payment fact application；非商户交易路径才回退 legacy refund worker | 已切到 ecommerce refund fact terminalizer |
| `api/payment_callback.go` `handleProfitSharingNotify` | 分账动账通知 | 解密、归属校验、入队分账结果 | profit sharing fact terminalizer |
| `api/payment_callback.go` `handleApplymentStateNotify` | 进件状态通知 | 解密、更新/入队申请状态处理 | applyment fact terminalizer |
| `api/payment_callback.go` `handleEcommerceWithdrawNotify` | 商户提现状态通知 | 解密、同步 withdrawal_record 状态 | withdraw fact terminalizer |
| `api/payment_callback.go` `handleMerchantTransferNotify` | 商家转账通知 | 解密、解析 out_bill_no，调用赔付 action 处理 | transfer fact terminalizer |
| `api/complaint.go` `handleComplaintNotify` | 消费者投诉通知 | 解密并同步投诉记录 | operational fact |
| `api/violation.go` `handleViolationNotify` | 商户违规通知 | 解密并同步违规记录 | operational fact |

### 查询/恢复入口

| 入口 | 当前覆盖 | 缺口 |
| --- | --- | --- |
| `worker/payment_fact_application_scheduler.go` | 每分钟扫描 retryable fact applications 并入队 `payment:process_fact_application`；2026-04-26 已补齐 profit sharing return、settlement、merchant funds、claim recovery targets | 只负责 application retry，不做微信远端查询；后续新增 consumer 时必须同步 target list 和测试 |
| `worker/payment_domain_outbox_scheduler.go` | 每分钟扫描 pending payment domain outbox 并入队 `payment:process_domain_outbox`，覆盖支付成功、退款异常/成功、分账结果、进件状态等副作用 | 只负责 owner 持久化后的副作用发布，不承担 query recovery |
| `worker/payment_recovery_scheduler.go` | 扫描 paid 但未 processed 的支付单；收付通 order/reservation 与直连 rider_deposit/claim_recovery 均写 manual reconciliation fact application；未配置 target 的 owner 只告警跳过，不再走 legacy payment success worker | 不查询微信 pending 远端状态，不能发现“微信已支付但本地仍 pending”；更广义 manual reconciliation 若产品化，应另起人工入口写 manual fact |
| `worker/task_payment_timeout.go` | 普通 `payment_order:timeout` 已先按 `payment_channel` 执行 QueryOrder / QueryPartnerOrder；远端已支付时写 query fact application 并停止本地关单，远端未支付/已关闭/失败时先远端关单再本地关闭和取消业务订单 | 本切片覆盖 pending 本地单；本地已 closed 后才发现远端已支付的异常退款/人工恢复仍是后续 review hotspot |
| `worker/task_combined_payment_timeout.go` | 合单超时前 QueryCombineOrder，远端 paid/partial_paid 时写 order/reservation payment fact application 并停止自动关单 | 合单逻辑相对完整，可作为普通单 timeout 改造参考 |
| `worker/refund_recovery_scheduler.go` | stuck processing 退款查询直连 `QueryRefund` 或收付通 `QueryEcommerceRefund`；rider deposit/order/reservation 已写 refund fact application；未建模 owner 终态查询结果已停止 legacy `RefundResultPayload` fallback，改为持久化 `REFUND_FAILED` critical 告警 | pending/cancelled refund 补偿仍是创建退款 command 路径；未建模 owner 需要补建 terminalizer 后才能自动收敛 |
| `worker/profit_sharing_recovery_scheduler.go` | 分账创建 retry、缺失分账补偿；stuck processing 分账回退直接 `QueryProfitSharingReturn`，terminal 结果写 `profit_sharing_domain/profit_sharing_return` fact application，仍 processing 则刷新本地记录 | stuck 分账回退 recovery 已停止 legacy result payload fallback；分账请求链路中的旧轮询 producer 和 manual reconciliation 仍需后续独立清理 |
| `worker/applyment_recovery_scheduler.go` | 长时间审核/异常进件主动 query；已知 finish/terminal/pending guidance 状态写 applyment fact application；未知/unsupported 状态已停止直接 `UpdateEcommerceApplymentStatus` fallback，改为持久化 critical 平台告警并保持本地状态不变 | 已知非终态仍刷新本地查询字段；后续若微信新增状态，必须先补 contract mapping 或 owner terminalizer |
| `worker/merchant_withdraw_recovery_scheduler.go` | 商户提现 pending 扫描入队 result query worker；扫描 SQL 已用 `updated_at < now()-5m` 控制频率，result worker 对 pending 查询尝试触碰 `updated_at`；终态仍写 `merchant_funds_domain/withdrawal_record` fact application | manual reconciliation 若要产品化，应另起入口并写 manual fact，不再走 scheduler 高频兜底 |
| `worker/merchant_cancel_withdraw_recovery_scheduler.go` | 商户注销提现 pending 扫描入队 result query worker；扫描 SQL 已用 `COALESCE(last_query_at, created_at) < now()-5m` 控制频率，result worker 查询路径维护 `last_query_at`；终态仍写 `merchant_funds_domain/merchant_cancel_withdraw_application` fact application | manual reconciliation 若要产品化，应另起入口并写 manual fact，不再走 scheduler 高频兜底 |
| `worker/claim_refund_recovery_scheduler.go` | 平台赔付商家转账行为动作恢复，跳过 terminal failure action | 应统一到 merchant_transfer / transfer fact，但不进入 009 首批 payment/refund/profit-sharing recovery 切片 |

## 5. TASK-PAY-002 的输入约束

统一支付事实模型至少要覆盖这些外部对象：

- `payment_order`: 直连支付、收付通单笔支付、收付通合单子单。
- `combined_payment_order`: 收付通合单主单。
- `refund_order`: 直连退款、收付通退款、异常退款。
- `profit_sharing_order`: 分账请求、分账结果、解冻。
- `profit_sharing_return`: 分账回退。
- `ecommerce_applyment`: 进件申请状态。
- `withdrawal_record`: 商户提现。
- `merchant_cancel_withdraw_application`: 商户注销提现。
- `merchant_transfer`: 平台赔付到零钱。
- `wechat_complaint` / `wechat_violation`: operational facts，不应和资金终态混为一类，但可以复用回调幂等基础设施。

建议 fact dedupe key：

- 首选微信 notification id + callback type。
- 查询来源没有 notification id 时，用 `source=query` + `external_object_type` + 业务幂等键 + `terminal_status` + 微信更新时间。
- manual reconciliation 用 `source=manual` + operator/user + reason + 目标对象 id。

## 6. TASK-PAY-004 首个样板边界

骑手押金样板应只改一个纵向链路，避免一次性扩散：

1. 保留 `api/rider.go` 对小程序的响应契约。
2. `CreateJSAPIOrder` 只创建支付命令和 pay params。
3. `handlePaymentNotify` / timeout query / manual reconciliation 都写 direct payment fact。
4. `RiderDepositDomain.HandleDepositPaid` 消费 paid fact 入账。
5. `SubmitWithdrawal` 只创建 refund command accepted 或同步拒绝，不直接当业务终态。
6. `handleRefundNotify` / `RefundRecoveryScheduler` / manual reconciliation 都写 direct refund fact。
7. `RiderDepositDomain.HandleRefundTerminal` 负责 credit、frozen、流水、stale credit reconciliation。
8. 接收方 ensure/delete 从押金主链路移到异步 receiver lifecycle。

2026-04-26 进度：

- 已完成第 6-7 步的 callback/query 收口：`handleRefundNotify` 与 `RefundRecoveryScheduler` 已切到 direct refund fact + `rider_deposit_domain` application 单源。
- 已完成退款异常副作用收口：`ABNORMAL` 通过 payment domain outbox 统一告警，不再依赖旧 refund result worker。
- 已完成 callback 侧第 3-4 步：押金充值支付成功已切到 direct payment fact + `rider_deposit_domain` application 单源，旧 payment success worker 与 payment recovery scheduler 已从 rider_deposit 路径退出。
- timeout query / manual reconciliation 仍未并入同一恢复矩阵，继续记在 TASK-PAY-009。

## 7. 验证建议

TASK-PAY-001 本身是只读盘点，验证方式是源代码搜索和关键文件读取。

后续开始代码迁移时，最低验证门槛：

- 支付 callback 重复投递测试。
- query recovery 与 callback 到同一终态入口的测试。
- timeout 前远端已支付的查单保护测试。
- 提交返回 PROCESSING 不推进业务终态的测试。
- 已存在的 rider deposit stale credit reconciliation 回归测试必须保留。
