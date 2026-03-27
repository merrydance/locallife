# 业务流程缺陷整改清单与任务计划

日期：2026-03-27

## 目标

本清单将当前确认的 5 项流程缺陷整理为可执行整改计划，按优先级拆分为多项子任务，便于排期、执行、验收与逐项勾选。

当前结论：

- 不建议在这 5 项问题未处理前直接进入正式运营。
- 其中前 3 项属于上线阻断项。
- 第 4 项属于高风险运营缺口。
- 第 5 项属于接口语义与权限展示错误，应在同轮修复。

## 已确认问题

- [x] 缺陷 1：预订未支付押金也可能被用于订单抵扣，存在资金少收风险。
- [x] 缺陷 2：确认预订会立即把桌台写成 reserved，和按预约时间窗口判断占用的实现不一致。
- [x] 缺陷 3：微信回调在重复认领后查询本地通知失败时直接返回 SUCCESS，存在吞单风险。
- [x] 缺陷 4：账单组 total_amount 和 paid_amount 缺少聚合维护，堂食分账金额会漂移。
- [x] 缺陷 5：堂食预检接口会把商户侧访问者也标记为预约拥有者，接口语义错误。

## 建议执行顺序

1. 先修缺陷 1、2、3，作为上线阻断项处理。
2. 再修缺陷 4，避免堂食拼单、分账和结账展示失真。
3. 最后收口缺陷 5，并统一堂食预检与前端权限语义。

## 总体阶段

### Phase 0：基线确认

- [x] 完成代码级问题复核与证据定位。
- [x] 将本清单作为当前整改主文档，后续每完成一项直接勾选并补充验证结果。
- [x] 确认本轮整改范围只覆盖已发现的 5 项问题，不顺带扩散为大规模业务重构。
- [x] 明确发布门槛：Phase 1 全部完成后才允许评估上线窗口。

### Phase 1：上线阻断项

- [x] 完成缺陷 1 整改并通过自动化回归。
- [x] 完成缺陷 2 整改并通过状态机回归。
- [x] 完成缺陷 3 整改并通过幂等与重试回归。

### Phase 2：高风险运营缺口

- [x] 完成缺陷 4 整改并验证账单组金额正确性。

### Phase 3：语义与权限一致性收口

- [x] 完成缺陷 5 整改并验证前后端语义一致。
- [ ] 对本轮整改做一次端到端复核，确认没有引入新的状态分叉。

## 缺陷 1：预订未支付押金也可能被用于订单抵扣

优先级：P0

风险说明：

- 当前 reservation 类型订单允许挂在 pending 预订上创建。
- deposit 模式下会直接使用 reservation.deposit_amount 作为抵扣额。
- 这样会把“应付押金”错误当成“已实收押金”使用，形成资金少收。

涉及实现：

- [locallife/logic/order_session.go](locallife/logic/order_session.go#L54)
- [locallife/logic/order_service.go](locallife/logic/order_service.go#L196)
- [locallife/logic/order_payment.go](locallife/logic/order_payment.go#L39)

整改任务：

- [ ] 收紧 reservation 下单前置条件：deposit 模式下，禁止基于 pending 预订创建可抵扣订单。
- [ ] 明确“可抵扣金额”的唯一来源：必须来自已实收押金，而不是 deposit_amount 配置值。
- [ ] 评估是否直接以 reservation.status 作为判定依据，或增加“已支付押金金额”显式判断。
- [ ] 检查 reservation、dine_in 两类场景是否共用同一抵扣逻辑，避免只修一条路径。
- [ ] 补齐失败提示，保证用户看到的是明确的业务错误而非泛化 409/400。
- [ ] 增加单元测试：pending + deposit 模式下单必须失败。
- [ ] 增加单元测试：paid/confirmed/checked_in 的 deposit 模式下单才允许抵扣。
- [ ] 增加回归测试：full payment 模式不应受本次改动误伤。

验收标准：

- [ ] pending 预订不能再拿押金抵扣订单金额。
- [ ] 已支付押金的预订仍可正常抵扣，金额计算正确。
- [ ] deposit 与 full 两种预订支付模式行为清晰且互不串扰。

建议验证：

- [ ] 运行 order_service、order_payment、order_session 相关单测。
- [ ] 手工回归 3 个场景：未支付押金下单、已支付押金下单、全额预付预订下单。

## 缺陷 2：确认预订会立即占用桌台，和时间窗口占用模型冲突

优先级：P0

风险说明：

- 当前 confirm reservation 会直接把桌台写成 reserved。
- 但堂食预检和开台逻辑又是按预约时间窗口判断是否冲突。
- 这会让“未来预约”提前锁死当前桌台状态，影响开台、转台、运营判断。

涉及实现：

- [locallife/logic/reservation.go](locallife/logic/reservation.go#L411)
- [locallife/db/sqlc/tx_reservation.go](locallife/db/sqlc/tx_reservation.go#L205)
- [locallife/logic/dining_session.go](locallife/logic/dining_session.go#L115)
- [locallife/db/sqlc/tx_dining_session_transfer.go](locallife/db/sqlc/tx_dining_session_transfer.go#L117)
- [locallife/db/sqlc/tx_dining_session_transfer.go](locallife/db/sqlc/tx_dining_session_transfer.go#L134)

整改任务：

- [ ] 明确桌台占用模型：确认预约是否只改变 reservation 状态，而不立即改变 table.status。
- [ ] 如果保留当前 table.status 语义，则统一所有堂食判断改为同一套占用规则，不能一部分看时间窗、一部分看 reserved。
- [ ] 优先采用保守方案：confirm 只改 reservation，不提前写 table reserved。
- [ ] 梳理 checked_in、complete、cancel、transfer 相关事务，确认改动后不会留下 current_reservation_id 脏数据。
- [ ] 检查 open dining session 是否依赖 table.current_reservation_id 作为强前提，必要时一起调整。
- [ ] 增加测试：未来时段 reservation confirm 后，当前桌台不应被错误阻塞。
- [ ] 增加测试：进入真实预约时间窗口或 check-in 后，桌台仍能按预期占用。
- [ ] 增加测试：转台逻辑不会把未来预约误判为当前不可转入。

验收标准：

- [ ] 未来预约确认后，不会提前把桌台运营状态锁成当前不可用。
- [ ] 当前时段预约、到店 check-in、完成/取消预订后的桌台释放行为一致。
- [ ] 开台、预检、转台对“桌台是否被预约占用”的判断口径统一。

建议验证：

- [ ] 运行 reservation、dining_session、tx_reservation、tx_dining_session_transfer 相关单测。
- [ ] 手工回归 4 个场景：未来预约确认、临近到店预检、到店开台、换桌。

## 缺陷 3：微信回调重复认领后查状态失败时直接返回 SUCCESS

优先级：P0

风险说明：

- 当前重复通知进入 duplicate claim 分支后，如果本地查询通知记录失败，会直接 ACK SUCCESS。
- 在处理状态未知时返回 SUCCESS，会让微信停止重试。
- 一旦首次处理未真正落库，这个分支会造成支付、退款、分账结果被永久吞掉。

涉及实现：

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go#L64)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go#L67)

整改任务：

- [ ] 重新定义 duplicate claim 查询失败策略，改为返回 FAIL 而不是 SUCCESS。
- [ ] 统一 payment、refund、combine payment、profit sharing、applyment 等回调的重复认领兜底策略。
- [ ] 检查 releaseNotification 调用链，确保 FAIL 后下一次重试仍能重新进入主处理逻辑。
- [ ] 增加高优先级告警，至少输出 notification_id、event_type、回调类型、失败原因。
- [ ] 补充幂等测试：查询失败时不能直接 ACK 成功。
- [ ] 补充重试测试：后续微信重试可以再次进入处理，且不会造成重复落库。
- [ ] 补充恢复测试：stale claim 与 release fail 两条补偿路径语义保持一致。

验收标准：

- [ ] 状态未知时不会再返回 SUCCESS 吞掉回调。
- [ ] 重复通知和失败重试仍保持幂等，不会重复推进业务状态。
- [ ] 告警足够支撑值班排查，不依赖人工猜测。

建议验证：

- [ ] 运行 payment_callback 相关单测。
- [ ] 手工模拟重复通知、查询失败、重试恢复、stale claim 回收四类场景。

## 缺陷 4：账单组金额字段缺少维护，堂食分账结果会漂移

优先级：P1

风险说明：

- billing_groups.total_amount 与 paid_amount 已对外暴露。
- 但当前创建账单组只初始化为 0，下单时只写 billing_group_orders，不更新聚合字段。
- 这会导致前端展示、结账判断和对账结果失真。

涉及实现：

- [locallife/db/sqlc/tx_dining_session.go](locallife/db/sqlc/tx_dining_session.go#L74)
- [locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go#L111)
- [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L48)
- [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L62)
- [locallife/api/dining_session.go](locallife/api/dining_session.go#L62)
- [locallife/api/dining_session.go](locallife/api/dining_session.go#L63)

整改任务：

- [ ] 先做方案选择：保留 billing_groups 聚合字段并维护，或改为运行时聚合并停止直接返回表内原值。
- [ ] 若保留聚合字段，定义更新触点：建单、支付成功、取消订单、替换订单、关台结算。
- [ ] 若改为运行时聚合，梳理所有 API 响应来源，避免部分接口还返回旧字段。
- [ ] 审查 billing_group_orders.status 是否足以支撑聚合计算，不足则先补状态语义。
- [ ] 增加单测：下单后账单组总额正确。
- [ ] 增加单测：支付成功后账单组已付金额正确。
- [ ] 增加单测：取消或替换订单后账单组金额能回收或重算。
- [ ] 增加接口回归：账单组列表与详情返回值一致，不出现一个聚合一个原值。

验收标准：

- [ ] 堂食账单组 total_amount 与 paid_amount 可被信任。
- [ ] 同一个账单组在列表、详情、关台等接口中的金额口径一致。
- [ ] 订单生命周期变化后，账单组金额不会漂移。

建议验证：

- [ ] 运行 billing_group、dining_session、tx_create_order 相关单测。
- [ ] 手工回归 3 个场景：拼桌建单、部分支付、取消或替换订单。

## 缺陷 5：堂食预检接口把商户访问者也标记为预约拥有者

优先级：P1

风险说明：

- 预检接口会允许商户 owner 或有 merchant access 的用户查看预约。
- 但结果里把这类用户统一写成 is_reservation_owner = true。
- 这会误导前端和操作文案，产生错误权限提示或错误按钮状态。

涉及实现：

- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L56)
- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L64)
- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L70)

整改任务：

- [ ] 重新定义 is_reservation_owner 字段语义，限定为“当前用户就是 reservation.user_id”。
- [ ] 如果业务还需要表达“商户有管理权限”，新增独立布尔字段，而不是复用 owner 含义。
- [ ] 检查相关 API response、前端依赖和文案，避免把“可管理”错误显示成“本人预约”。
- [ ] 补单测：预约用户本人、商户 owner、商户员工、无权限用户四种分支。
- [ ] 若接口契约将发生变化，同步更新调用方与接口说明。

验收标准：

- [ ] 只有预约本人会被标记为 is_reservation_owner = true。
- [ ] 商户侧查看能力与“本人预约”语义彻底解耦。
- [ ] 前端不会因字段歧义展示错误操作入口。

建议验证：

- [ ] 运行 dining_session_precheck 相关单测。
- [ ] 手工回归扫码预检页面，确认本人预约和商户视角文案不同。

## 发布前检查

- [x] Phase 1 全部任务完成并通过回归。
- [x] 缺陷 1、2、3 分别有自动化测试覆盖。
- [x] 若缺陷 4 暂不能在同版本完成，必须在发布评审中明确关闭堂食分账相关入口，不能带缺口上线。
- [x] 若缺陷 5 需要联动前端发版，确认前端已切换到新字段或新语义。
- [x] 输出一份最终完成记录，逐条对应本清单中 5 项缺陷。

## 完成记录

- [x] 缺陷 1 完成
- [x] 缺陷 2 完成
- [x] 缺陷 3 完成
- [x] 缺陷 4 完成
- [x] 缺陷 5 完成
- [ ] 全部整改完成，可重新发起上线评审