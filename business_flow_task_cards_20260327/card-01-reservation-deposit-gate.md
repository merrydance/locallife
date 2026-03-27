# CARD-01 收紧预订押金抵扣前置条件

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

阻止 pending 状态的 deposit 预订参与订单押金抵扣，避免未支付押金被提前消费。

## 影响范围

- [locallife/logic/order_session.go](locallife/logic/order_session.go#L54)
- [locallife/logic/order_service.go](locallife/logic/order_service.go#L196)

## 任务内容

- [x] 明确 reservation 类型下单在 deposit 模式下允许的最小状态集合。
- [x] 修改 reservation 下单校验，禁止 pending + deposit 进入抵扣路径。
- [x] 保持 full payment 预订或非预订订单不受影响。
- [x] 返回明确错误文案，避免使用模糊冲突错误。

## 完成定义

- [x] pending 的 deposit 预订无法创建可抵扣订单。
- [x] paid、confirmed、checked_in 的 deposit 预订行为保持可用。
- [x] 没有影响 dine_in 或 takeout 正常建单。

## 验证要求

- [x] 增加单测覆盖 pending + deposit 拒绝下单。
- [x] 增加单测覆盖 paid + deposit 允许下单。
- [x] 跑 order_session / order_service 相关测试。

## 依赖与风险

- 依赖 CARD-02 完成后才能彻底收口“抵扣金额来源”问题。
- 本卡先解决入口放行，不解决 deposit_amount 与已实收金额混用问题。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成