# CARD-03 补 claim_recoveries 与 recovery events 主判关联

状态：已完成

优先级：P0

所属阶段：Phase 1

## 问题目标

让 claim_recoveries 不再只靠 decision_snapshot 间接追溯来源，并建立 recovery 事件链，为后续 rollback 做准备。

## 影响范围

- [locallife/db/migration/000119_add_claim_recoveries_and_remove_evidence.up.sql](locallife/db/migration/000119_add_claim_recoveries_and_remove_evidence.up.sql)
- 新增后续 migration 文件
- 相关 recovery query 文件

## 任务内容

- [x] 给 claim_recoveries 增加 decision_id
- [x] 增加 recovery_basis 等正式来源字段
- [x] 新增 claim_recovery_events 表
- [x] 预留 created、paid、waived、resumed、overturned 事件类型

## 完成定义

- [x] recovery 可直接追溯到 decision_id
- [x] recovery 事件历史已有正式表承接

## 验证要求

- [x] migration 已编写完成
- [x] 表关系与约束合理

## 依赖与风险

- 后续 CARD-13、CARD-14 依赖这张表，不应拖到后面再补

## 完成记录

- [x] migration 完成
- [x] query 对齐完成
- [x] 评审完成