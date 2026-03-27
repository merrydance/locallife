# CARD-08 实现账单组金额维护链路

状态：已完成

优先级：P1

所属阶段：Phase 2

## 问题目标

让账单组金额在建单、支付、取消、替换订单等主链路中保持正确。

## 影响范围

- [locallife/db/sqlc/tx_dining_session.go](locallife/db/sqlc/tx_dining_session.go#L74)
- [locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go#L111)
- [locallife/logic/replace_order.go](locallife/logic/replace_order.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)

## 任务内容

- [x] 按 CARD-07 方案实现账单组金额更新或运行时聚合。
- [x] 覆盖建单、支付成功、取消订单、替换订单、关台结算等主要更新触点。
- [x] 明确 billing_group_orders.status 与金额汇总的关系。

## 完成定义

- [x] billing group 金额不会长期停留在 0 或旧值。
- [x] 订单生命周期变化后，账单组金额能同步更新或正确重算。

## 验证要求

- [x] 增加单测覆盖建单与支付。
- [x] 增加单测覆盖取消与替换订单。

## 依赖与风险

- 依赖 CARD-07 先确定单一事实来源。

## 实施记录

- 在 `billing_group.sql` 增加 `GetBillingGroupAmounts` 聚合查询，按关联订单实时状态和 `orders.total_amount` 计算金额。
- 修正 `CreateOrderTx` 的账单组关联金额来源，统一使用订单 `total_amount`，不再错误地写入 0。
- 修正 `ReplaceOrderTx`：旧订单被替换后，新的 replacement order 会自动继承原账单组关联。
- 当前 `billing_group_orders.status` 仍保留为关联记录状态；金额聚合以订单真实状态和订单当前金额为准，不再依赖该字段或历史快照金额维护支付口径。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [x] 评审完成