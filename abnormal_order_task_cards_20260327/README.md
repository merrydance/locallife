# 异常订单主判开发任务卡索引

日期：2026-03-27

本目录用于把异常订单主判改造拆成可独立领取、交付、验证、勾选完成的任务卡。

配套上游文档：

1. [implementation_flows/abnormal-order-main-adjudicator-master-plan.md](implementation_flows/abnormal-order-main-adjudicator-master-plan.md)
2. [implementation_flows/abnormal-order-main-adjudicator-rules-matrix.md](implementation_flows/abnormal-order-main-adjudicator-rules-matrix.md)
3. [implementation_flows/abnormal-order-main-adjudicator-data-model-v2.md](implementation_flows/abnormal-order-main-adjudicator-data-model-v2.md)

## 使用方式

- 每张任务卡都以“单阶段内可独立交付”为目标。
- 任务卡只在代码、验证、结论都完成后才能勾选。
- 不允许跳过前置卡直接做后置卡，除非阶段图明确允许并行。
- 当前尚未开工，因此所有卡默认未开始。
- 真正开始实施时，优先看 [abnormal_order_task_cards_20260327/phase-1-delivery-map.md](abnormal_order_task_cards_20260327/phase-1-delivery-map.md)。

## 任务卡列表

### Phase 1：V2 落库骨架

- [ ] [CARD-01 扩展 behavior_decisions V2 主字段](abnormal_order_task_cards_20260327/card-01-behavior-decisions-v2.md)
- [ ] [CARD-02 扩展 behavior_trace_snapshots 结构化快照](abnormal_order_task_cards_20260327/card-02-trace-snapshots-v2.md)
- [ ] [CARD-03 补 claim_recoveries 与 recovery events 主判关联](abnormal_order_task_cards_20260327/card-03-recovery-link-and-events.md)
- [ ] [CARD-04 新增 behavior_decision_effects 画像净值账本](abnormal_order_task_cards_20260327/card-04-decision-effects-ledger.md)
- [ ] [CARD-05 补 query 与 sqlc 生成链路](abnormal_order_task_cards_20260327/card-05-query-and-sqlc-regeneration.md)
- [ ] [CARD-06 接通 tx_claim_behavior V2 双写](abnormal_order_task_cards_20260327/card-06-tx-claim-behavior-dual-write.md)
- [ ] [CARD-07 Phase 1 验证与回归收口](abnormal_order_task_cards_20260327/card-07-phase-1-validation.md)

### Phase 2：在线事实读取

- [ ] [CARD-08 接通净有效画像摘要读取](abnormal_order_task_cards_20260327/card-08-profile-summary-read-path.md)
- [ ] [CARD-09 接通图谱与关联摘要读取](abnormal_order_task_cards_20260327/card-09-graph-summary-read-path.md)
- [ ] [CARD-10 接通责任域关键事实读取与兜底降级](abnormal_order_task_cards_20260327/card-10-fact-assembler-and-fallback.md)

### Phase 3：正式主判引擎

- [ ] [CARD-11 定义 DecisionV2 与四分模型输出](abnormal_order_task_cards_20260327/card-11-decision-v2-structure.md)
- [ ] [CARD-12 实现规则矩阵裁决与 decision_mode 落库](abnormal_order_task_cards_20260327/card-12-decision-matrix-and-persistence.md)

### Phase 4：动作与回滚链路

- [ ] [CARD-13 接通 recovery fallback restrict 动作编排](abnormal_order_task_cards_20260327/card-13-action-orchestrator.md)
- [ ] [CARD-14 接通申诉再裁决与净值回滚](abnormal_order_task_cards_20260327/card-14-appeal-redecision-and-rollback.md)

### Phase 5：验证与切换

- [ ] [CARD-15 实现 shadow run 与主判差异观测](abnormal_order_task_cards_20260327/card-15-shadow-run-and-observability.md)
- [ ] [CARD-16 灰度切换与全量主判收口](abnormal_order_task_cards_20260327/card-16-cutover-and-cleanup.md)

## 阶段执行图

- [ ] [Phase 1 开发顺序图与依赖图](abnormal_order_task_cards_20260327/phase-1-delivery-map.md)

后续规则：

1. 只有当 Phase 1 接近开工或完成后，才继续补 Phase 2 delivery map。
2. 不提前把所有后续阶段拆得过细，避免计划快速失效。

## 推荐执行顺序

1. 先完成 CARD-01 到 CARD-04，把 V2 表结构和账本骨架建起来。
2. 再做 CARD-05 和 CARD-06，把 query、sqlc、事务双写接通。
3. 然后用 CARD-07 做 Phase 1 验证和收口。
4. 后续 Phase 2 到 Phase 5 再按总计划继续推进。

## 完成记录

- [ ] Phase 1 完成
- [ ] Phase 2 完成
- [ ] Phase 3 完成
- [ ] Phase 4 完成
- [ ] Phase 5 完成
- [ ] 异常订单主判改造完成，可进入正式切换评估