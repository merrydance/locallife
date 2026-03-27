# CARD-02 统一已实收押金抵扣模型

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

把“可抵扣金额”统一为已实收押金，而不是预订配置金额或应付押金金额。

## 影响范围

- [locallife/logic/order_service.go](locallife/logic/order_service.go#L196)
- [locallife/logic/order_payment.go](locallife/logic/order_payment.go#L39)
- [locallife/logic/reservation.go](locallife/logic/reservation.go)

## 任务内容

- [x] 盘点当前预订记录中哪些字段代表“应收押金”、哪些字段代表“已收金额”。
- [x] 提取统一的 deposit deduction 计算入口，避免散落在 order_service 中硬编码。
- [x] 确认 dine_in 与 reservation 是否共用抵扣逻辑，并统一到同一个 helper。
- [x] 如果现有字段不足以表达“已实收押金”，先定义保守判定规则并写入任务备注。

## 完成定义

- [x] 抵扣金额不再直接取 reservation.deposit_amount。
- [x] 抵扣逻辑能清晰解释资金来源。
- [x] 代码中不再出现多个彼此独立的押金抵扣判断。

## 验证要求

- [x] 增加单测覆盖已收押金小于订单金额。
- [x] 增加单测覆盖已收押金大于订单金额时的封顶行为。
- [x] 回归 full payment 预订不应误入押金抵扣路径。

## 依赖与风险

- 最好在 CARD-01 后执行，避免先修金额来源但入口仍放行 pending。
- 如需新增字段或调整预订支付语义，应单独记录为后续结构化改造项。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成

补充说明：

- 已将押金抵扣来源统一为最近一笔 reservation business_type 支付单的实付金额 `payment_orders.amount`，不再直接读取 `table_reservations.deposit_amount`。
- 当前代码库没有单独聚合“已实收押金”字段，本次采用保守规则：仅接受最近一笔状态为 `paid` 的预约支付单作为可抵扣来源；缺失或未结算时直接拒绝抵扣。