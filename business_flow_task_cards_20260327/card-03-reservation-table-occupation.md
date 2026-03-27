# CARD-03 修正预订确认占桌时机

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

让 confirm reservation 不再提前把未来时段的桌台写成当前 reserved。

## 影响范围

- [locallife/logic/reservation.go](locallife/logic/reservation.go#L411)
- [locallife/db/sqlc/tx_reservation.go](locallife/db/sqlc/tx_reservation.go#L205)

## 任务内容

- [x] 明确 confirm reservation 的职责边界，只处理 reservation 状态，还是同时处理 table 状态。
- [x] 优先采用保守修复：confirm 只改 reservation，不提前写 table.status。
- [x] 检查 reservation complete / cancel 是否依赖 confirm 时写入的 current_reservation_id。
- [ ] 如果必须保留 table 变更，至少补足预约时间窗口判断与当前占用校验。

## 完成定义

- [x] 未来时段 reservation confirm 后不会直接阻塞当前桌台。
- [x] 当前预订确认链路仍可顺利进入后续 check-in。
- [x] 不会因移除 reserved 写入而导致后续事务报错。

## 验证要求

- [x] 增加单测覆盖未来预约 confirm。
- [x] 增加单测覆盖临近预约时段 confirm。
- [x] 跑 reservation / tx_reservation 相关测试。

## 依赖与风险

- 本卡完成后需要 CARD-04 继续清理开台、转台、关台等后续状态链。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成

补充说明：

- `ConfirmReservationTx` 已调整为仅推进 reservation 到 `confirmed`，不再写入 `table.status=reserved` 和 `current_reservation_id`。
- `tx_reservation_test.go` 已同步改为“确认不占桌、后续开台/到店才占桌”的事务语义，并通过相关测试。