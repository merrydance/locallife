# CARD-05 补 query 与 sqlc 生成链路

状态：未开始

优先级：P0

所属阶段：Phase 1

## 问题目标

把前四张 schema 卡落到 query 和生成代码层，确保后续 tx 和 logic 可以安全使用 V2 结构。

## 影响范围

- [locallife/db/query/behavior_trace.sql](locallife/db/query/behavior_trace.sql)
- 相关 recovery query 文件
- 新增 ledger query 文件
- sqlc 生成结果

## 任务内容

- [ ] 更新 behavior_decisions create/get/list/update query
- [ ] 更新 behavior_trace_snapshots create/list query
- [ ] 更新 claim_recoveries 相关 query
- [ ] 新增 claim_recovery_events query
- [ ] 新增 behavior_decision_effects query
- [ ] 执行 make sqlc

## 完成定义

- [ ] 所有新字段已有正式 query 支撑
- [ ] sqlc 生成成功
- [ ] 没有遗留手写 SQL 与生成代码漂移

## 验证要求

- [ ] make sqlc 成功
- [ ] 生成代码可编译

## 依赖与风险

- 依赖 CARD-01 到 CARD-04 的 schema 基本冻结

## 完成记录

- [ ] query 完成
- [ ] sqlc 生成完成
- [ ] 评审完成