# CARD-04 清理预订与桌台状态机联动

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

统一 reservation、table、dining session 在预约场景下的占用判断口径，避免一部分看 reserved，一部分看时间窗口。

## 影响范围

- [locallife/logic/dining_session.go](locallife/logic/dining_session.go#L115)
- [locallife/db/sqlc/tx_dining_session_transfer.go](locallife/db/sqlc/tx_dining_session_transfer.go#L117)
- [locallife/db/sqlc/tx_dining_session_transfer.go](locallife/db/sqlc/tx_dining_session_transfer.go#L134)
- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go)

## 任务内容

- [x] 梳理“桌台被预约占用”的唯一判断口径。
- [x] 统一预检、开台、转台对预约占用的判断逻辑。
- [x] 检查 current_reservation_id 的写入与清理时机，避免留下脏值。
- [x] 补充注释或 helper 命名，明确 reserved 是“当前占用”还是“未来预留”。

## 完成定义

- [x] 预检、开台、转台对同一桌台同一时刻给出一致结论。
- [x] current_reservation_id 不再被未来预约错误长期占用。
- [x] 预约到店、换桌、关台后不会出现状态撕裂。

## 验证要求

- [x] 增加单测覆盖 future reservation + transfer。
- [x] 增加单测覆盖 check-in 后转台与关台。
- [x] 跑 dining_session / dining_session_precheck / tx_dining_session_transfer 相关测试。

## 依赖与风险

- 依赖 CARD-03 的确认占桌修复方案。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成

补充说明：

- 预检与开台原本已通过 `FindActiveReservationForTable` 按当前时间窗判断冲突预约，本次将转台事务也补齐为同口径校验。
- `tx_dining_session_transfer.go` 不再依赖 `table.status=reserved` 或 `current_reservation_id` 作为未来预约冲突的唯一来源。
- 已验证转台成功、future reservation 冲突、开台关台相关测试通过。