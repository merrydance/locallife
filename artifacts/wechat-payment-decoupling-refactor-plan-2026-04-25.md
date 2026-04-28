# 微信支付解耦重构计划

日期：2026-04-25

## 1. 结论

需要做全局支付架构收口，但不应一次性重构整个系统。

正确策略是：全局盘点、统一原则、分域迁移、先拿骑手押金做样板。两套微信支付能力继续分开：直连支付仍负责骑手押金、追偿等平台直连能力；平台收付通仍负责商户交易、进件、分账、资金管理、投诉等服务商能力。

本次重构的核心不是把所有支付调用合成一个大服务，而是把“支付请求返回”“支付异步事实”“业务域状态”拆开。

## 1.1 当前执行账本（2026-04-26）

### 已彻底完成

- `TASK-PAY-004`：骑手押金支付与退款 callback 终态都已切到 `payment fact -> rider_deposit_domain application` 单源；旧 `ProcessTaskPaymentSuccess` 与 `PaymentRecoveryScheduler` 不再拥有押金充值 payment success 路径，接收方 present intent 改由 fact application 成功后写入。
- `TASK-PAY-003`：command 返回语义已与业务终态拆开，closeout 已完成；剩余工作已转入后续任务组。
- `TASK-PAY-005`：接收方生命周期主链与长期失败告警 closeout 已全部完成，作为独立异步能力运行。
- `TASK-PAY-007`：分账结果与分账回退主链已完成收口。callback/query/合法 command response 统一进入 `payment fact -> profit_sharing_domain application -> payment_domain_outbox`；旧 callback/query/legacy result worker 不再拥有生产终态推进权。补差能力继续保持 deferred/non-MVP，不纳入本轮 closeout。

- `TASK-PAY-006`：订单/预订支付成功与退款 terminal 主链已完成收口。order/reservation paid callback、combine replay、query recovery 与 ecommerce refund callback/query recovery 已统一进入各自 `payment fact -> domain application -> payment_domain_outbox` owner 路径；旧 `ProcessTaskPaymentSuccess`、`ProcessTaskRefundResult` 对商户交易只保留显式 skip/protect，不再拥有生产终态推进权。

### 尚未开始主实现

- `TASK-PAY-008`：进件、结算账户、商户提现、注销提现等长流程迁移。
- `TASK-PAY-009`：统一恢复、查询调度、人工对账入口和恢复节奏整合。
- `TASK-PAY-010`：投诉/违规/结算事件、观测面、发布门禁与收口校验。

### 当前推进顺序

1. 已完成分账结果与分账回退主链收口，后续不再回退到兼容层。
2. `TASK-PAY-004` 已整体收口完成；文档与调用矩阵只做同步，不再回开实现任务。
3. `TASK-PAY-005` 已整体收口完成；后续不再回开 receiver lifecycle 主链或 closeout 任务。
4. `TASK-PAY-007` 已整体收口完成；补差保持 deferred/non-MVP，只做文档同步与缺陷修正。
5. `TASK-PAY-006` 已整体收口完成；文档与调用矩阵已同步，后续不再回开 merchant transaction legacy handler 主链。
6. 下一阶段进入 `TASK-PAY-008` / `TASK-PAY-009`，优先处理长流程状态 owner 与恢复调度统一化的下一条高风险切片。

## 2. 核心架构原则

### 2.1 提交返回不是业务终态

微信支付相关接口大多是异步流程。除本地参数校验失败、微信同步拒绝、查询接口返回明确终态等场景外，提交接口的返回只能表示“请求已提交或已被受理”，不能直接作为业务成功终态。

必须区分三类事实：

- Command accepted：请求已被微信接受，例如支付下单成功、退款申请进入 PROCESSING、分账请求已受理。
- External terminal fact：微信通过回调或主动查询确认终态，例如支付成功、退款成功、退款关闭、转账失败、进件完成。
- Business state transition：本系统业务状态变化，例如订单变为 paid、押金入账、押金提现结清、商户进件完成。

提交返回可以推进“请求记录状态”，但不能直接推进不可逆业务终态。后续处理必须由回调、查询调度、恢复任务统一写入支付事实，再由业务域消费该事实。

### 2.2 支付事实不等于业务状态

支付事实层只记录微信侧发生了什么。业务域自己决定该事实如何影响业务状态。

例如：

- 直连退款成功事实不直接等于押金提现完成；押金域需要检查 refund_order、credit、frozen_deposit、幂等状态后再结算。
- 支付成功事实不直接等于订单可履约；订单域还要决定库存、配送、预订等后续状态。
- 进件申请提交成功不等于商户可收款；商户进件域要等最终申请状态。

### 2.3 微信只作为外部适配器

直连支付 adapter 和收付通 adapter 只做：请求构造、签名、响应契约校验、错误码归一化、回调解密、查询外部状态。

它们不应该决定：

- 订单状态怎么变。
- 押金余额怎么记。
- 分账任务是否完成。
- 商户进件业务流程是否结束。

### 2.4 业务域拥有自己的状态机

每个业务域必须拥有自己的状态 owner：

- 订单域拥有订单支付、退款、履约状态。
- 预订域拥有预订支付、补款、退款状态。
- 骑手押金域拥有押金账户、可退款 credit、冻结、提现结算。
- 追偿域拥有追偿支付、释放、恢复状态。
- 分账域拥有接收方、分账、回退、解冻状态。
- 商户进件域拥有申请资料、申请状态、结算账户状态。

支付域只向这些业务域提供外部支付事实。

### 2.5 回调和查询调度进入同一个终态入口

回调不是唯一可靠来源，查询调度也不是补丁。两者应该写入同一个 Payment Fact Terminalizer。

推荐链路：

1. webhook 解密并校验签名。
2. query scheduler 主动查询超时或缺回调的外部单据。
3. 两者都归一化成同一类 external fact。
4. external fact 进入幂等终态处理器。
5. 业务域消费终态事实。

## 3. 目标分层

### 3.1 Payment Adapter Layer

职责：对接微信官方能力组。

- DirectPaymentAdapter：小程序直连支付、直连退款、直连查单、关单、商家转账。
- EcommercePaymentAdapter：平台收付通下单、退款、分账、进件、资金管理、投诉。
- Contract/ErrorCode packages：只沉淀官方请求、响应、错误码、枚举。

禁止：在 adapter 中写业务状态。

### 3.2 Payment Fact Layer

职责：记录外部支付事实和终态。

建议概念：

- payment_commands：本系统发出的外部请求。
- payment_facts：微信回调或查询得到的外部事实。
- external_object_type：payment_order、refund_order、profit_sharing_order、applyment、transfer_bill、withdraw_application。
- terminal_status：SUCCESS、FAILED、CLOSED、EXPIRED、UNKNOWN、PROCESSING。
- fact_source：callback、query、manual_reconciliation。

### 3.3 Business Domain Layer

职责：消费 payment facts 并推进业务状态。

示例：

- RiderDepositDomain.HandleDepositPaid(fact)
- RiderDepositDomain.HandleRefundSucceeded(fact)
- OrderPaymentDomain.HandlePaymentPaid(fact)
- ProfitSharingDomain.HandleSharingResult(fact)
- ApplymentDomain.HandleApplymentStatus(fact)

### 3.4 Orchestration Layer

职责：worker、scheduler、outbox、重试、告警。

禁止：把跨域副作用串进核心事务。

例如骑手押金支付成功后同步分账接收方，应从主链路拆出：

- 押金域确认余额入账。
- 写出 RiderDepositActivated 事件。
- ReceiverSync worker 异步确保骑手接收方存在。
- 接收方同步失败只重试和告警，不回滚押金余额。

## 4. 任务卡拆解

### TASK-PAY-001 支付调用点全量盘点

目标：建立全系统支付调用地图。

范围：

- 直连支付：骑手押金充值、押金提现、追偿付款、商家转账。
- 平台收付通：订单支付、预订支付、退款、分账、进件、资金管理、投诉、商户注销提现。
- worker、scheduler、api、logic 中所有直接调用微信客户端的位置。

交付物：

- 一张调用矩阵：调用点、微信能力组、业务 owner、同步返回用途、终态来源、幂等键、当前风险。

验收标准：

- 每个微信调用都有明确 owner。
- 每个调用都标记“提交返回是否被错误当作终态”。
- 每个异步流程都有 callback/query/manual recovery 三类路径说明。

验证：

- `rg "DirectPaymentClient|EcommerceClient|CreateRefund|QueryRefund|ProfitSharing|Applyment|Transfer" locallife`

### TASK-PAY-002 定义统一支付事实模型

目标：统一回调和查询调度进入同一个外部事实层。

细化任务卡：`artifacts/wechat-payment-fact-model-task-card-2026-04-25.md`。

范围：

- 设计 payment_facts 或等价表。
- 设计 fact dedupe key。
- 设计 terminalizer 的幂等规则。

边界：

- 不迁移业务状态。
- 不重写所有微信客户端。

验收标准：

- 同一微信事件重复回调不会重复推进业务。
- callback 和 query 写入相同事实模型。
- UNKNOWN/PROCESSING 不推进业务终态，只触发下一次查询或告警。

### TASK-PAY-003 拆分支付命令返回与业务终态

目标：修正“提交返回即成功”的调用习惯。

细化任务卡：`artifacts/wechat-payment-command-return-task-card-2026-04-25.md`。

范围：

- 支付下单返回：只表示可调起支付。
- 退款申请返回：只表示退款请求已受理或同步拒绝。
- 分账请求返回：只表示分账请求已受理或同步拒绝。
- 进件提交返回：只表示申请单已创建或同步拒绝。

验收标准：

- 所有提交接口返回状态命名避免 `success` 误导，使用 `accepted`、`processing`、`submitted`。
- 业务终态只由 terminal fact handler 推进。
- API 响应对前端明确区分“已提交处理”和“已完成”。

### TASK-PAY-004 骑手押金域样板重构

目标：以骑手押金作为首个纵向样板，验证新边界。

范围：

- 从支付成功总事务中拆出 RiderDepositPaymentSettled handler。
- 押金域独立拥有 riders.deposit_amount、frozen_deposit、rider_deposit_credits、rider_deposits 流水。
- 直连退款创建只生成 refund command accepted，不直接作为业务终态。
- 退款成功、关闭、异常由统一 terminal fact 触发押金域结算或回滚。

必须保留：

- 当前已修复的“原单已全额退款”stale credit reconciliation 逻辑。
- 当前 API 对小程序的兼容响应。

验收标准：

- 微信退款创建 PROCESSING 后，小程序看到的是处理中，不是成功终态。
- 退款 SUCCESS 回调或查询终态后，押金余额才最终扣减。
- 接收方同步失败不影响押金入账成功。
- stale credit 不再回到可提现余额。

2026-04-26 当前状态：

- 已完成：`handleRefundNotify`、`RefundRecoveryScheduler` 统一写 direct refund fact，并通过 `rider_deposit_domain` fact application 单源推进押金退款终态。
- 已完成：骑手押金退款 `ABNORMAL` 副作用通过 payment domain outbox 统一告警，旧 `ProcessTaskRefundResult` 已不再处理 rider deposit refund。
- 已完成：骑手押金支付成功已由 payment fact application 单源消费；旧 `ProcessTaskPaymentSuccess` 与 `PaymentRecoveryScheduler` 对 rider deposit 路径仅保留显式跳过与保护，不再拥有生产推进权。

验证：

- `go test ./logic ./db/sqlc -run 'Test(RiderDepositRefundService_|PrepareRiderDepositRefundTx_|ResolveRiderDepositRefundTx_)'`
- 增加 API 级回归：旧 credit 漂移后再次提现只对账旧 credit，不错误增加可提现余额。

### TASK-PAY-005 接收方生命周期独立化

目标：把分账接收方同步从订单、押金等主链路拆出来。

范围：

- 新建 ReceiverLifecycleService 或 ProfitSharingReceiverMembershipService。
- 接收 RiderDepositActivated、RiderDepositCleared、OperatorActivated、MerchantApplymentReady 等事件。
- 异步 ensure/delete receiver。

验收标准：

- 接收方同步失败不会回滚押金、订单或进件业务主状态。
- 接收方同步有重试、告警、可手工恢复入口。
- receiver_name 过长等微信 PARAM_ERROR 进入可观测失败，不导致支付成功任务反复失败。

### TASK-PAY-006 订单与预订支付迁移

目标：把商户交易支付成功从总支付流程迁移到订单/预订域处理器。

范围：

- order payment paid fact。
- reservation payment paid fact。
- order refund terminal fact。
- reservation refund terminal fact。

验收标准：

- 支付成功回调重复投递不重复激活订单。
- 查询调度补到支付成功也走相同入口。
- 退款申请 accepted 不提前关闭订单，退款终态才改变业务状态。

当前拆分：

- `006-order-payment-paid-handler`：已完成。普通单 paid callback、combine replay、query recovery 已统一进入 `order_domain` paid fact/application，legacy payment success worker 对 order 只保留显式保护。
- `006-reservation-payment-paid-handler`：已完成。预订 paid callback、combine replay、query recovery 已统一进入 `reservation_domain` paid fact/application，legacy payment success worker 对 reservation / reservation_addon 只保留显式保护。
- `006-order-refund-terminal-handler`：已完成。订单 ecommerce refund callback/query 已统一进入 `order_domain` refund fact/application，成功通知与异常告警通过 owner/outbox 单源执行。
- `006-reservation-refund-terminal-handler`：已完成。预订 ecommerce refund callback/query 已统一进入 `reservation_domain` refund fact/application，prepaid amount 调整只在 terminal success owner handler 内发生。
- `006-legacy-cleanup`：已完成。`ProcessTaskPaymentSuccess`、`ProcessTaskRefundResult` 对商户交易只保留显式 skip/protect，recovery/replay 与 owner handler 归属已对齐。

### TASK-PAY-007 分账剩余异步链路收口

目标：把分账剩余异步链路统一纳入 payment fact layer。补差保持 deferred/non-MVP，不作为当前 TASK-PAY-007 的实现重点。

范围：

- 请求分账。
- 查询分账结果。
- 分账动账通知。
- 分账回退。
- 解冻剩余资金。

验收标准：

- 分账请求返回不作为最终分账成功。
- 分账结果 callback/query 才推进分账状态。
- 分账回退 query/result 终态不再由 legacy worker 直接写业务状态，而是先落 terminal fact，再由 application 推进 `profit_sharing_returns` 与退款后续动作。
- 分账失败有明确重试或人工处理策略。

当前拆分：

- `007-return-query-application`：已完成。`QueryProfitSharingReturn` 终态统一记录为 `external_payment_facts`，`profit_sharing_domain` application 单源推进 `profit_sharing_returns`，并在全部回退成功后继续发起退款。
- `007-return-command-response-fact-source`：已完成。`CreateProfitSharingReturn` 的合法同步终态与同步拒绝都已进入 `command_response` fact source，请求路径不再保留 direct terminal write。
- `007-return-legacy-cleanup`：已完成。分账回退请求路径 direct terminal write 与多余 legacy 分支已清理；分账与分账回退终态统一由 application owner 推进。

当前不纳入：

- 补差 create/return/cancel 的能力组闭环实现。
- 补差对象级授权整改本体。
- 因“补差与分账组”历史并列而把补差重新拉回当前 MVP。

### TASK-PAY-008 商户进件与资金管理迁移

目标：把进件、结算账户、提现等长流程全部纳入异步事实模型。

范围：

- 提交申请单。
- 查询申请状态。
- 修改/查询结算账户。
- 商户提现申请和状态查询。

验收标准：

- 提交申请只标记 submitted。
- 查询到 FINISH/REJECTED 等终态后才更新业务 ready 状态。
- 微信 NO_AUTH/PARAM_ERROR 等同步拒绝只进入申请失败原因或待处理状态，不误判业务完成。

当前阶段进展：

- 第一小阶段已完成：merchant applyment `FINISH + sub_mchid` owner path 从 callback / recovery 直接激活切到 `external_payment_facts` -> `external_payment_fact_applications` -> `payment_domain_outbox`。
- 第二小阶段第一块切片已完成：merchant applyment `REJECTED` / `FROZEN` / `CANCELED` 终态也已从 callback / recovery legacy 直写切到 `external_payment_facts` -> `external_payment_fact_applications` -> `payment_domain_outbox`。
- 第二小阶段第二块切片已完成：merchant applyment `ACCOUNT_NEED_VERIFY` / `APPLYMENT_STATE_TO_BE_CONFIRMED` / `APPLYMENT_STATE_TO_BE_SIGNED` 已从 callback / recovery legacy follow-up 切到 `external_payment_facts` -> `external_payment_fact_applications` -> `payment_domain_outbox`。
- 第三小阶段第一块切片已完成：merchant settlement modify submit 在 accepted / rejected 微信返回后已写入 `external_payment_commands`，不再只有 `latest_settlement_application_no` tracking 字段可追。
- 第三小阶段第二块切片已完成：merchant settlement current-status query 与 `ApplymentSettlementVerificationScheduler` 已统一记录 settlement query fact，并由 `settlement_domain` application 单写 `ecommerce_applyments.settlement_verify_*`；商户主动查询与巡检查询不再分别直写同一组状态字段。
- 第三小阶段第三块切片已完成：merchant `QuerySubMerchantSettlementApplication` 已从 handler 直接写 tracking 切到 `external_payment_facts` -> `external_payment_fact_applications`；`settlement_domain` application 现在单写 `merchant_payment_configs.latest_settlement_application_no / latest_settlement_application_submitted_at`，不再把 settlement application audit tracking 混入 applyment owner。
- 第四小阶段第一块切片已完成：merchant withdraw live query、callback 与 recovery worker 已统一记录 withdraw fact，并附加 `merchant_funds_domain -> withdrawal_record` application target；withdraw owner status 现在只由 fact application 单写 `withdrawal_records.status / reason`，不再由 handler、callback helper 或 result worker 各自直写。
- 第四小阶段第二块切片已完成：merchant cancel withdraw create 后即时 query、详情 live query 与 recovery worker 已统一记录 cancel withdraw fact，并附加 `merchant_funds_domain -> merchant_cancel_withdraw_application` application target；cancel withdraw owner 状态现在只由 fact application 单写 `merchant_cancel_withdraw_applications.cancel_state / withdraw_state / latest_query_response` 等 query truth 字段，create handler 只保留 `submitted_at` 标记这类 command accepted 窄写入。
- 当前已收口的执行链：`handleApplymentStateNotify` 与 `ApplymentRecoveryScheduler` 在 merchant applyment 命中 `finish + sub_mchid` 时记录 success fact，由 `applyment_domain` application 调 `ApplymentSubMchActivationTx`，再由 `applyment_activated` outbox 发送开户成功通知并标记 result processed；命中 `rejected` / `frozen` / `canceled` 时记录 terminal fact，由 `applyment_domain` application 单源更新 applyment 状态，再由 `applyment_terminal_state_ready` outbox 发送终态通知并标记 result processed；命中 `account_need_verify` / `to_be_confirmed` / `to_be_signed` 时记录 processing fact，由 `applyment_domain` application 单源更新 guidance 字段，再由 `applyment_pending_state_ready` outbox 发送待处理通知并标记 result processed。
- 当前已收口的 settlement verify 执行链：merchant `QuerySubMerchantSettlement` 与 `ApplymentSettlementVerificationScheduler` 统一写入 `external_payment_facts`，命中 `VERIFY_SUCCESS` / `VERIFY_FAIL` / `VERIFYING` 后由 `settlement_domain` application 同步更新 `ecommerce_applyments.settlement_verify_status`、`settlement_verify_fail_reason`、以及巡检来源携带的 `first_trade_at` / `last_checked_at` / `check_count`；scheduler 只在 owner 状态持久化成功后再发送失败通知并标记 `settlement_verify_failed_notified_at`。
- 当前已收口的 settlement application audit 执行链：merchant `QuerySubMerchantSettlementApplication` 在命中 `AUDIT_SUCCESS` / `AUDIT_FAIL` / `AUDITING` 后统一写入 settlement fact，并附加 `settlement_domain -> merchant_payment_config` application target；handler 同步 apply 后单写 `merchant_payment_configs.latest_settlement_application_no / latest_settlement_application_submitted_at`，不再保留独立的 handler 直写 owner 路径。
- 当前已收口的 merchant withdraw 执行链：withdraw detail live query、create 异常后的即时 query backfill、提现回调与 `ProcessTaskMerchantWithdrawResult` 查询恢复，都会统一写入 withdraw fact；`merchant_funds_domain` application 再单写 `withdrawal_records.status / reason`，同时账户侧 `withdraw_id` 继续单独持久化到 `withdrawal_records.account_info` 供列表、详情与告警解析复用。
- 当前已收口的 merchant cancel withdraw 执行链：create 成功后的即时 query、详情页 live query 与 `ProcessTaskMerchantCancelWithdrawResult` 查询恢复，都会统一写入 cancel withdraw fact；`merchant_funds_domain` application 再单写 `merchant_cancel_withdraw_applications` 的 cancel/withdraw query truth 字段，create handler 仅保留 `submitted_at` 和 accepted command 记录，不再由 API/worker 继续直写 query owner 状态。
- 当前仍未收口的 008 范围：更广义 manual reconciliation / recovery matrix；下一锚点顺序进入 TASK-PAY-009 的统一恢复与对账调度。

### TASK-PAY-009 统一恢复与对账调度

目标：补齐所有异步链路的 query scheduler。

范围：

- 支付单超时未回调查询。
- 退款单处理中查询。
- 分账处理中查询。
- 进件申请处理中查询。
- 商户提现处理中查询。

验收标准：

- 所有 PROCESSING 超过阈值的外部对象都有查询恢复任务。
- 查询结果进入同一 terminalizer。
- 查询频率遵守微信限制并有退避策略。

2026-04-26 `TASK-PAY-009-A` 盘点冻结：

- `payment_fact_application_scheduler` 已在 `main.go` 注册并作为统一 retry 扫描入口；本轮补齐 target 覆盖：`profit_sharing_domain/profit_sharing_return`、`settlement_domain/ecommerce_applyment`、`settlement_domain/merchant_payment_config`、`merchant_funds_domain/withdrawal_record`、`merchant_funds_domain/merchant_cancel_withdraw_application`、`claim_recovery_domain/payment_order`，避免 008 已收口 owner path 的 application 失败后只能靠同步调用重试。
- `payment_domain_outbox_scheduler` 已在 `main.go` 注册，覆盖当前支付域 outbox 事件；它负责 owner 持久化后的副作用发布，不承担微信 query recovery。
- 普通 `payment_order:timeout` 仍是第一条未收口恢复链：本地 pending 超时会直接 `UpdatePaymentOrderToClosed` 并取消业务订单，缺少关单前 `QueryOrder` / `QueryPartnerOrder` 保护；下一切片先做这一刀。
- 合单 `combined_payment_order:timeout` 已具备 `QueryCombineOrder` 保护，并在远端 paid / partial paid 时写入 order/reservation payment fact application；可作为普通单 timeout 改造模板。
- `payment_recovery_scheduler` 已将 paid-unprocessed 的收付通 order/reservation payment 改成 manual reconciliation fact application；它仍不负责发现“微信已支付但本地 pending”的远端查单。
- `refund_recovery_scheduler` 已将 stuck processing 的 direct rider deposit refund、ecommerce order refund、ecommerce reservation refund 查询结果写入同一 refund fact application；pending/cancelled refund 补偿仍走 refund command 创建路径，legacy result payload 只应保留给尚未建模的退款类型。
- `profit_sharing_recovery_scheduler` 仍以重入分账处理和分账回退结果轮询为主；分账请求、分账回退的 query/command-response fact helper 已存在，下一步需要把 processing query 结果和旧 result worker 分支压到同一 application terminalizer。
- `applyment_recovery_scheduler` 对 finish / rejected / frozen / canceled / account_need_verify / to_be_confirmed / to_be_signed 已写 applyment fact application；未知状态仍保留直接状态更新作为 review hotspot，后续只允许在明确 contract 后收窄。
- `merchant_withdraw_recovery_scheduler` 与 `merchant_cancel_withdraw_recovery_scheduler` 继续负责扫描 pending 对象并入队 result query worker；result worker 已统一写 `merchant_funds_domain` fact application，后续只清理 manual reconciliation/频率矩阵，不重新打开 008 owner path。
- `claim_refund_recovery_scheduler` 仍是行为动作恢复，不在 009 首批资金 query terminalizer 内；商家转账/追偿支付 fact 化进入后续 transfer fact 切片。

2026-04-26 `TASK-PAY-009-B` 普通支付 timeout-before-close query 收口：

- `payment_order:timeout` 的 pending 分支已改为先按 `payment_channel` 查询微信远端状态：收付通普通单走 `QueryPartnerOrderByTransactionID` / `QueryPartnerOrderByOutTradeNo`，直连普通单走 `QueryOrderByOutTradeNo`。
- 查询发现远端 `SUCCESS` 时，不再本地关单或取消业务订单；改为写入 `external_payment_facts`，创建对应 `external_payment_fact_applications` 并入队 `payment:process_fact_application`，由既有 order/reservation/rider deposit/claim recovery terminalizer 收敛业务状态。
- 查询发现远端 `NOTPAY` / `CLOSED` / `PAYERROR` / `REVOKED` 时，先调用微信远端关单接口，再关闭本地支付单并执行原有业务订单取消链路。
- 查询发现 `USERPAYING` 时返回可重试错误；发现未知/退款等异常远端状态或远端金额与本地金额不一致时发布 payment timeout 告警并停止自动关单。
- 本切片不扩展退款、分账、进件、商户提现语义；`closed` 本地旧状态重试发现远端已支付后的异常退款/人工恢复仍留给后续专门切片。

2026-04-26 `TASK-PAY-009-C` 退款 recovery legacy fallback 收口：

- `refund_recovery_scheduler` 的 stuck processing 查询结果继续按 owner 写入 fact application：直连 rider deposit refund、收付通 order refund、收付通 reservation refund 都进入对应 `refund_order` application target。
- 查询结果已进入终态但没有建模 fact application target 的退款，不再回退到 `DistributeTaskProcessRefundResult` legacy result worker；系统改为持久化 `REFUND_FAILED` critical 平台告警，要求人工核对并补建 owner terminalizer。
- 查询仍为 `PROCESSING` 时继续等待，不写终态 fact，也不触发 legacy worker。
- cancelled/pending refund 补偿仍只负责创建/重入退款 command，不在本切片中改变 create path 语义。

2026-04-26 `TASK-PAY-009-D` 分账回退 recovery legacy fallback 收口：

- `profit_sharing_recovery_scheduler` 的 stuck processing 分账回退单不再重新投递 `ProfitSharingReturnResultPayload` 旧结果轮询任务；scheduler 现在直接调用 `QueryProfitSharingReturn` 获取微信远端事实。
- 查询结果为 `SUCCESS` / `FAILED` 时写入 `external_payment_facts`，创建 `profit_sharing_domain/profit_sharing_return` application，并入队 `payment:process_fact_application`，由既有分账回退 terminalizer 单源推进 `profit_sharing_returns` 与退款后续动作。
- 查询结果仍为 `PROCESSING` 时只刷新本地分账回退 processing 记录和 return_id，不写终态 fact，也不触发 legacy result worker。
- 普通分账创建 retry、缺失分账补偿和分账请求主流程不在本切片扩大；`ProcessTaskProfitSharingReturnResult` 仍保留给请求链路中已存在的轮询入口，后续清理必须另起切片并确认所有 producer 已切换。

2026-04-27 `TASK-PAY-009-E` 进件 recovery unsupported 状态 fallback 收口：

- `applyment_recovery_scheduler` 对微信查询返回的未知/unsupported `applyment_state` 不再借用本地旧状态继续 `UpdateEcommerceApplymentStatus`，避免把未建模的上游状态静默写成本地处理中状态。
- 未知状态现在持久化 `SYSTEM_ERROR` critical 平台告警，保留本地状态不变，要求人工核对并补齐状态映射或 owner terminalizer；已知 `FINISH` / `REJECTED` / `FROZEN` / `CANCELED` / `ACCOUNT_NEED_VERIFY` / `TO_BE_CONFIRMED` / `TO_BE_SIGNED` 仍走 applyment fact application。
- `AUDITING` / `CHECKING` / `PROCESSING` / `SIGNING` 等已知非终态仍只刷新查询字段和 follow-up 需要的本地状态，不扩大为新 terminal fact 语义。

2026-04-27 `TASK-PAY-009-F` 商户提现 recovery 频率矩阵收口：

- `merchant_withdraw_recovery_scheduler` 不再每 3 分钟无条件扫描所有 pending merchant withdraw；`ListPendingWithdrawalRecordsByChannel` 增加 `updated_before` cutoff，只选择 `updated_at` 早于 5 分钟前的 pending 记录入队查询。
- `ProcessTaskMerchantWithdrawResult` 查询到仍 pending 或查询失败准备 retry 时会触碰 pending withdrawal 的 `updated_at`，让 scheduler 以最近查询尝试作为节流标记；终态仍由 `merchant_funds_domain/withdrawal_record` fact application 单写 `withdrawal_records.status / reason`。
- 本切片不新增兼容层，也不重新打开 008 owner path；`withdrawal_records` 当前没有 `last_query_at` 字段，因此采用已有 `updated_at` 作为恢复扫描节流标记。

2026-04-27 `TASK-PAY-009-G` 商户注销提现 recovery 频率矩阵收口：

- `merchant_cancel_withdraw_recovery_scheduler` 改为传入 `query_before` cutoff；`ListPendingMerchantCancelWithdrawApplications` 只扫描 `COALESCE(last_query_at, created_at)` 早于 5 分钟前、且尚未进入 `REJECTED` / `REVOKED` / `CANCELED` / `FINISH` 的申请。
- 结果查询 worker 已在 query path 更新 `last_query_at`，因此 recovery scheduler 不再对刚查询过的注销提现申请重复入队；终态仍由 `merchant_funds_domain/merchant_cancel_withdraw_application` fact application 单写 query truth 字段。
- 本轮同时 regenerated SQLC 和 Store mocks，是因为两条 recovery query 的签名都从单一 limit 扩展为显式参数结构；业务记录仍按 merchant withdraw 与 merchant cancel withdraw 分开。

2026-04-27 `TASK-PAY-009-H` paid-unprocessed 直连支付 recovery legacy fallback 收口：

- `payment_recovery_scheduler` 对 paid 但未 processed 的直连 rider deposit / claim recovery payment order 不再投递 legacy `ProcessTaskPaymentSuccess`；scheduler 现在写 `manual_reconciliation` direct payment fact，并入队 `payment:process_fact_application`。
- fact application target 使用既有 `rider_deposit_domain/payment_order` 与 `claim_recovery_domain/payment_order` terminalizer，由 owner path 单写 `ProcessPaymentSuccessTx` 及后续押金 receiver outbox。
- 未配置 fact application target 的 paid-unprocessed payment order 只记录 warning 并跳过，不再用 legacy success worker 兜底扩大状态写路径。

009 后续执行顺序冻结：普通支付 timeout-before-close query、paid-unprocessed direct recovery fallback、退款 recovery fallback、分账回退 stuck recovery、applyment unsupported 状态 fallback、merchant withdraw 频率矩阵、merchant cancel withdraw 频率矩阵已完成；剩余 009 收口点只保留更广义 manual reconciliation 入口审计与是否需要产品化的决策。

### TASK-PAY-010 支付域观测与发布门禁

目标：让支付异步链路可观测、可回滚、可审计。

范围：

- 标准化日志字段：external_object_type、external_id、out_trade_no、out_refund_no、fact_source、terminal_status、business_owner。
- 告警：终态不一致、长期 PROCESSING、回调验签失败、幂等冲突、业务状态推进失败。
- 发布检查：高风险链路必须有回调、查询、幂等、重复投递测试。

验收标准：

- 能按支付单号或退款单号查完整链路。
- 能识别“微信已终态、本地未推进”的卡点。
- 能人工重放 terminal fact。

## 5. 建议执行顺序

第一批只做设计和盘点：TASK-PAY-001、TASK-PAY-002、TASK-PAY-003。

第二批做样板：TASK-PAY-004、TASK-PAY-005。先拿骑手押金和接收方同步拆分验证架构。

第三批迁移商户交易：TASK-PAY-006、TASK-PAY-007。

第四批迁移长流程能力：TASK-PAY-008、TASK-PAY-009、TASK-PAY-010。

## 6. 不建议做的事

- 不要把直连支付和平台收付通合成一个巨大客户端。
- 不要一次性改完所有支付调用点。
- 不要让 handler 直接理解微信错误码和状态机。
- 不要让提交接口返回的 PROCESSING/SUCCESS 文案直接驱动业务完成状态。
- 不要把 receiver sync、通知、告警等副作用放在核心账本事务里。

## 7. 首个落地切片

建议从骑手押金开始，目标是最小但完整地证明新模型：

1. 支付成功只写 payment fact。
2. 押金域消费 paid fact 入账。
3. 押金提现提交只创建 refund command 并进入 processing。
4. 退款 callback/query 终态触发押金结算。
5. 接收方同步改为监听押金域事件异步执行。
6. API 与小程序现有响应保持兼容。

这个切片完成后，再按同一模式迁移订单、预订、分账和进件。