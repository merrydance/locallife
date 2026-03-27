# CARD-02 扩展 behavior_trace_snapshots 结构化快照

状态：未开始

优先级：P0

所属阶段：Phase 1

## 问题目标

把 behavior_trace_snapshots 从浅窗口计数表升级为三方结构化快照表，为后续主判解释和翻盘回放提供正式快照层。

## 影响范围

- 新增后续 migration 文件
- [locallife/db/query/behavior_trace.sql](locallife/db/query/behavior_trace.sql)

## 任务内容

- [ ] 增加 actor_type、actor_id、window_key、stats_scope
- [ ] 增加 metric_payload、association_payload、snapshot_version
- [ ] 保留旧列 window_days、abnormal_count、total_count、abnormal_rate、association_hits 做兼容
- [ ] 明确 user、merchant、rider 三类最小 metric_payload 键集合

## 完成定义

- [ ] snapshots 已能表达三方结构化快照
- [ ] 新旧列可以并存
- [ ] 后续 tx 双写不需要再改表意图

## 验证要求

- [ ] migration 可执行
- [ ] query 草案可支撑 create 和 list 场景

## 依赖与风险

- 如果结构化 payload 不先冻结，后续 CARD-06 容易反复改双写逻辑

## 完成记录

- [ ] migration 完成
- [ ] query 对齐完成
- [ ] 评审完成