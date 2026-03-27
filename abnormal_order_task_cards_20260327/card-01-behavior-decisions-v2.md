# CARD-01 扩展 behavior_decisions V2 主字段

状态：未开始

优先级：P0

所属阶段：Phase 1

## 问题目标

把 behavior_decisions 从旧版简单判责表升级为主判 V2 主表，能正式承载 decision_mode、U/M/R/C、fallback_reason、restriction_reason 等核心语义。

## 影响范围

- [locallife/db/migration/000094_add_behavior_trace_system.up.sql](locallife/db/migration/000094_add_behavior_trace_system.up.sql)
- 新增后续 migration 文件
- [locallife/db/query/behavior_trace.sql](locallife/db/query/behavior_trace.sql)

## 任务内容

- [ ] 为 behavior_decisions 增加 claim_id
- [ ] 增加 decision_mode、responsibility_domain、payout_mode、effective_status
- [ ] 增加 confidence_score、user_risk_score、merchant_liability_score、rider_liability_score
- [ ] 增加 fallback_reason、restriction_reason、liability_shares、score_breakdown、graph_hits、fact_snapshot
- [ ] 增加 supersedes_decision_id、overturned_by_decision_id、profile_effect_applied
- [ ] 为关键字段补约束和索引
- [ ] 保留 responsible_party、compensation_source、decision_status 做兼容

## 完成定义

- [ ] behavior_decisions 已具备正式主判 V2 字段
- [ ] 现有旧语义字段仍保留兼容
- [ ] 索引和约束能够支撑 claim 维度的有效判决读取

## 验证要求

- [ ] migration 可执行
- [ ] schema 审查通过

## 依赖与风险

- 这是 Phase 1 的基座卡，后续 CARD-05 和 CARD-06 都依赖它

## 完成记录

- [ ] migration 完成
- [ ] query 对齐完成
- [ ] 评审完成