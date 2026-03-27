# 支付流程整改执行计划

日期：2026-03-26

## 背景

本计划针对支付流程审查中发现的 5 个高优先级缺口，目标是把整改工作拆成可独立推进、可逐项勾选、可验证回归的执行清单。

当前高优先级问题如下：

1. 合单子单进入异常退款或金额异常路径后，主合单仍可能被错误标记为 paid，导致账务语义错误。
2. 金额异常自动退款当前主要依赖 OrderID，导致 membership_recharge、rider_deposit 这类无 OrderID 业务无法自动闭环。
3. 微信回调在本地查不到支付单时直接返回 SUCCESS，会吞掉外部唯一支付确认，形成不可自动恢复的资金风险。
4. 收付通回调缺少子商户归属校验（combine_mchid/sub_mchid 与本地商户配置对齐），存在跨租户状态污染风险。
5. 通知认领后若 release 失败，后续重试可能被误判为“已处理成功”并提前 ACK，导致真实回调被吞。

推荐执行顺序如下：

1. 先补齐回调归属校验和认领释放失败兜底，先堵住越权与吞单。
2. 再修正金额异常与恢复调度的联动语义，避免“异常到账被正常履约推进”。
3. 然后补齐无 OrderID 业务（membership_recharge、rider_deposit）的自动退款闭环。
4. 最后统一 not found 重试策略和观测告警，收口失败路径。

## 执行原则

- 保持现有签名校验、通知认领幂等、正常支付成功链路不退化。
- 保持 handler -> worker -> tx 的职责边界，避免在 handler 做跨业务类型的复杂状态决策。
- 所有回调都必须经过“签名可信 + 归属可信 + 数据一致”三重校验后才可更新本地业务状态。
- 优先修复资金语义和可恢复性问题，不在本轮扩展到普通退款流程重构。
- 每个审查发现都必须落到至少一项代码修改或测试补强。
- API、worker、db 三层职责边界保持清晰，不把业务修复散落到错误的层级。

## 任务清单

### Phase 1：范围确认与回归边界

- [ ] 复核单笔支付回调、合单支付回调、退款任务、退款恢复调度器的当前契约，确认本次只覆盖支付成功回调与异常退款链路，不扩展到普通退款业务重构。
- [ ] 记录需要保持不变的现有行为：签名校验、通知认领幂等、正常 paid 流程、已有退款结果回调处理、已有支付成功异步任务分发。
- [ ] 列出必须回归的支付业务类型：order、reservation、membership_recharge、rider_deposit。

### Phase 2：补齐回调归属校验与认领释放兜底

- [ ] 在单笔、合单、退款、分账回调中统一增加归属校验：回调资源中的商户字段（如 combine_mchid、mchid、sub_mchid、sp_mchid）必须与本地 payment_order/merchant_payment_config 对齐。
- [ ] 对归属不匹配的回调统一返回 FAIL（触发重试）并输出高优先级安全告警，避免直接 SUCCESS 吞掉线索。
- [ ] 直连回调补充可比对字段（mchid/appid）解析与校验能力，避免仅凭 out_trade_no 推进状态。
- [ ] 明确 TryClaimWechatNotification 成功后所有失败分支的 release 行为；release 失败时不得让后续重试被“已处理”短路。
- [ ] 增加兜底扫描任务或补偿标记，覆盖“claim 已占位但业务未完成”场景。
- [ ] 补测试：归属不匹配、release 失败、重试回调再次进入、避免重复告警。

### Phase 3：修复合单主单被错误标记为 paid

- [ ] 重新定义合单回调中的“子单成功”判定：只有真实完成支付且无需异常退款、无需金额异常退款的子单才计入成功。
- [ ] 调整 combine payment 回调分支，区分三类结果：真实支付成功、异常已转退款、处理失败待重试。
- [ ] 修改合单主单状态更新条件，只有全部子单都处于真实支付成功状态时才调用主单 paid 更新。
- [ ] 如果存在子单进入异常退款或金额异常退款流程，确保主单保持 pending 或进入可识别的异常态；如果当前表结构无法表达异常态，至少不要标记为 paid，并通过告警暴露该合单。
- [ ] 为上述行为补测试：closed/failed 子单收到回调、金额异常子单收到回调、混合成功与异常子单共存、所有子单都正常成功。

### Phase 4：补齐金额异常自动退款覆盖面

- [ ] 盘点当前金额异常退款依赖的定位条件，替换“仅 OrderID 有效才可退款”的限制为“PaymentOrder 能定位原支付渠道与业务主体即可退款”。
- [ ] 为 reservation 支付单补齐金额异常自动退款路径，优先复用 ReservationID 和 merchant payment config 的现有查找逻辑。
- [ ] 为 membership_recharge 支付单补齐金额异常自动退款路径，基于 payment_order 的 combined 或 payment 配置和商户信息定位退款参数。
- [ ] 为 rider_deposit 支付单补齐金额异常自动退款路径，确认其走直连退款接口，并且不依赖 OrderID。
- [ ] 审查并统一单笔回调与合单回调在金额异常时的分支策略，避免一边能自动退款、一边只能人工处理。
- [ ] 修正 payment recovery scheduler 的筛选条件，防止金额异常但待退款的 paid 订单进入正常支付成功推进。
- [ ] 明确“异常到账待退款”状态表达（字段或标记位）；在无法新增状态时至少从恢复扫描 SQL 中排除。
- [ ] 评估是否需要新增一个面向无 OrderID 业务的通用退款任务载荷，或者直接扩展现有 PayloadProcessRefund 字段以显式携带 ReservationID、BusinessType、MerchantID。
- [ ] 为每种无 OrderID 业务补测试：金额异常后应记录 paid、成功创建或复用 refund_order、成功入队正确退款任务或触发明确可恢复告警。

### Phase 5：修复本地查不到支付单却返回 SUCCESS

- [ ] 重新定义该场景的处理策略，优先采用“返回 FAIL 让微信重试，并发送高优先级告警”的方案。
- [ ] 分别检查单笔支付回调与合单支付回调的 not found 分支，统一成同一策略，避免一个可重试、一个直接吞单。
- [ ] 设计重试期间的幂等语义，确保不会因重复通知导致多次告警或多次异常退款入队。
- [ ] 如果决定返回 FAIL，需要确认 tryClaimNotification 的释放时机，保证下次重试还能重新进入主逻辑。
- [ ] 为本地支付单不存在场景补测试，覆盖单笔回调和合单子单回调两条路径。

### Phase 6：统一异常退款与恢复链路的可观测性

- [ ] 检查异常退款、金额异常退款、人工处理告警三类路径的告警标题、级别、关键字段是否一致，至少包含 payment_order_id、out_trade_no、transaction_id、refund_amount、business_type。
- [ ] 增加“归属校验失败”“回调认领释放失败”“异常到账误推进拦截”三类告警模板，确保能被值班快速分流。
- [ ] 审视 refund recovery scheduler 是否需要扩展到 membership_recharge、rider_deposit；如果本次不扩展，明确记为本轮范围外，并保证金额异常场景已在回调时闭环处理。
- [ ] 复核 refund_order 创建幂等性，确保回调重试、任务重试、恢复调度不会造成重复退款申请。

### Phase 7：回归测试与发布检查

- [ ] 运行支付回调相关单元测试，重点覆盖 payment_callback、task_process_payment、payment_order_service、combined payment 相关测试。
- [ ] 增加或更新失败路径测试，确保每个审查发现都对应至少一个自动化测试。
- [ ] 增加安全边界测试：错误 sub_mchid/combine_mchid/sp_mchid 通知不得推进本地状态。
- [ ] 增加可靠性边界测试：release 失败后微信重试应可再次进入处理（或可被补偿任务恢复），不能直接 ACK 成功。
- [ ] 做一次端到端代码复核，确认 API 层只做回调分支编排，退款或状态推进仍落在 logic、worker、db 既有职责边界内。
- [ ] 检查是否需要补充运维说明：哪些告警代表必须人工退款，哪些告警代表系统已自动退款但需关注结果。
- [ ] 输出最终变更说明，逐条对应本次审查发现，确认每条问题都已有代码、测试、告警或范围说明。

## 关键文件

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)：单笔支付与合单支付回调主入口。
- [locallife/api/payment_callback_test.go](locallife/api/payment_callback_test.go)：回调幂等、异常到账、金额异常、任务入队失败等测试样例。
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)：退款任务、异常退款任务、退款单幂等、支付成功后任务处理集中在这里。
- [locallife/db/sqlc/tx_notification.go](locallife/db/sqlc/tx_notification.go)：通知认领与释放的幂等占位实现。
- [locallife/worker/payment_recovery_scheduler.go](locallife/worker/payment_recovery_scheduler.go)：paid 未处理恢复扫描，需避免误推进异常到账订单。
- [locallife/db/sqlc/tx_create_ecommerce_payment.go](locallife/db/sqlc/tx_create_ecommerce_payment.go)：无 OrderID 的收付通支付单创建路径，决定 membership 和 reservation 的本地数据形态。
- [locallife/db/sqlc/tx_create_combined_payment.go](locallife/db/sqlc/tx_create_combined_payment.go)：合单支付本地事务创建路径，影响子单与主单关系及后续状态语义。
- [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)：payment_orders 查询与恢复扫描 SQL，尤其是 paid 未退款扫描范围。
- [locallife/api/membership.go](locallife/api/membership.go)：membership_recharge 的支付创建与幂等返回模式。
- [locallife/api/rider.go](locallife/api/rider.go)：rider_deposit 的直连支付创建路径。

## 验证要求

1. 运行与支付回调直接相关的测试，至少覆盖 [locallife/api/payment_callback_test.go](locallife/api/payment_callback_test.go) 所在包的单测。
2. 运行与退款任务相关的测试，重点覆盖 [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go) 所在包的单测。
3. 若新增或修改了 sqlc 事务相关行为，补跑 [locallife/db/sqlc/tx_create_combined_payment.go](locallife/db/sqlc/tx_create_combined_payment.go) 和 [locallife/db/sqlc/tx_create_ecommerce_payment.go](locallife/db/sqlc/tx_create_ecommerce_payment.go) 相关测试。
4. 手工回归 4 条核心场景：正常支付成功、closed 或 failed 后迟到回调、金额异常自动退款、回调找不到本地支付单。
5. 复核告警内容，确认异常路径都能输出可排障的关键字段。
6. 手工回归安全场景：伪造或错租户 sub_mchid/combine_mchid 回调不得推进任何本地支付状态。
7. 手工回归可靠性场景：模拟 release 失败后重复回调，验证不会被误 ACK 且可被系统恢复。

## 范围说明

本轮纳入范围：

- 支付成功回调。
- 异常到账退款。
- 金额异常退款。
- 合单主单状态语义修正。
- 回调归属校验（商户/子商户）。
- 通知认领释放失败兜底与补偿。
- 相关自动化测试补强。

本轮默认不纳入范围：

- 普通用户主动退款的大规模重构。
- 支付表结构新增状态。
- 非支付领域的运营后台展示改造。

若在实现中发现现有表结构无法表达“异常已退款待核查”状态，可先采用“主单不置为 paid + 明确告警 + 测试覆盖”的保守方案。

## 后续考虑

1. 如果希望合单异常子单有独立状态展示，后续可以单开任务，为 combined_payment_orders 增加更明确的异常状态，而不是在本轮一起做。
2. 如果未来还会增加无 OrderID 的支付业务，建议后续把退款任务入参改造成以 payment_order_id 为中心的统一解析模型，避免继续在回调层拼装业务分支。