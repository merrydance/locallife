# CARD-06 接通 tx_claim_behavior V2 双写

状态：已完成

优先级：P0

所属阶段：Phase 1

## 问题目标

在不切旧主判逻辑的前提下，让 claim 事务开始写入 V2 字段、结构化快照、recovery 关联和净值账本。

## 影响范围

- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go)

## 任务内容

- [x] 继续保留旧字段写入，保证兼容
- [x] 同步写入 behavior_decisions V2 字段
- [x] 同步写入三方结构化 snapshots
- [x] 创建 recovery 时补 decision_id 和 created event
- [x] 写入 behavior_decision_effects 的最小 applied 记录

## 完成定义

- [x] claim 事务已具备 V2 双写能力
- [x] 现有 claim 提交流程未被破坏
- [x] 后续 Phase 2 无需再回头补 Phase 1 数据结构写入

## 验证要求

- [x] 相关 db/sqlc 或 logic 最小测试通过
- [x] 新老字段已在双写路径中同时写入

## 依赖与风险

- 依赖 CARD-05 完成 query 和 sqlc 生成

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [x] 评审完成