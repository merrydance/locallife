# CARD-04 新增 behavior_decision_effects 画像净值账本

状态：已完成

优先级：P0

所属阶段：Phase 1

## 问题目标

建立正式的画像净值账本，避免未来申诉翻盘时继续靠代码猜测应该减回哪些计数。

## 影响范围

- 新增后续 migration 文件
- 新增相关 query 文件

## 任务内容

- [x] 新增 behavior_decision_effects 表
- [x] 定义 entity_type、entity_id、metric_key、delta_value、status、reverted_by_decision_id 等字段
- [x] 为 applied 和 reverted 场景设计最小约束与索引

## 完成定义

- [x] 主判净值影响已有正式账本承接
- [x] rollback 不再必须依赖业务代码猜测原始计数

## 验证要求

- [x] migration 已编写完成
- [x] query 草案可支撑 insert 和 revert 场景

## 依赖与风险

- 这是后续画像汇总和申诉回滚的基座卡

## 完成记录

- [x] migration 完成
- [x] query 对齐完成
- [x] 评审完成