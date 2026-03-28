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
- 当前状态：Phase 1、Phase 2 已完成；Phase 3 正在收口；Phase 4 以后默认未开始。
- 真正继续实施时，优先看当前阶段对应的 delivery map，而不是只看总计划。

## 任务卡列表

### Phase 1：V2 落库骨架

- [x] [CARD-01 扩展 behavior_decisions V2 主字段](abnormal_order_task_cards_20260327/card-01-behavior-decisions-v2.md)
- [x] [CARD-02 扩展 behavior_trace_snapshots 结构化快照](abnormal_order_task_cards_20260327/card-02-trace-snapshots-v2.md)
- [x] [CARD-03 补 claim_recoveries 与 recovery events 主判关联](abnormal_order_task_cards_20260327/card-03-recovery-link-and-events.md)
- [x] [CARD-04 新增 behavior_decision_effects 画像净值账本](abnormal_order_task_cards_20260327/card-04-decision-effects-ledger.md)
- [x] [CARD-05 补 query 与 sqlc 生成链路](abnormal_order_task_cards_20260327/card-05-query-and-sqlc-regeneration.md)
- [x] [CARD-06 接通 tx_claim_behavior V2 双写](abnormal_order_task_cards_20260327/card-06-tx-claim-behavior-dual-write.md)
- [x] [CARD-07 Phase 1 验证与回归收口](abnormal_order_task_cards_20260327/card-07-phase-1-validation.md)

### Phase 2：在线事实读取

- [x] [CARD-08 接通净有效画像摘要读取](abnormal_order_task_cards_20260327/card-08-profile-summary-read-path.md)
- [x] [CARD-09 接通图谱与关联摘要读取](abnormal_order_task_cards_20260327/card-09-graph-summary-read-path.md)
- [x] [CARD-10 接通责任域关键事实读取与兜底降级](abnormal_order_task_cards_20260327/card-10-fact-assembler-and-fallback.md)

### Phase 3：正式主判引擎

- [ ] [CARD-11 定义 DecisionV2 与四分模型输出](abnormal_order_task_cards_20260327/card-11-decision-v2-structure.md)
- [ ] [CARD-12 实现规则矩阵裁决与 decision_mode 落库](abnormal_order_task_cards_20260327/card-12-decision-matrix-and-persistence.md)

### Phase 4：动作与回滚链路

- [ ] [CARD-13 接通 recovery fallback restrict 动作编排](abnormal_order_task_cards_20260327/card-13-action-orchestrator.md)
- [ ] [CARD-14 接通申诉再裁决与净值回滚](abnormal_order_task_cards_20260327/card-14-appeal-redecision-and-rollback.md)

### Phase 5：预上线验收与直接切换

- [ ] [CARD-15 预上线验收与主判差异复核](abnormal_order_task_cards_20260327/card-15-prelaunch-validation-and-observability.md)
- [ ] [CARD-16 直接切主判与旧分支下线](abnormal_order_task_cards_20260327/card-16-direct-cutover-and-cleanup.md)

## 阶段执行图

- [x] [Phase 1 开发顺序图与依赖图](abnormal_order_task_cards_20260327/phase-1-delivery-map.md)
- [x] [Phase 4 开发顺序图与依赖图](abnormal_order_task_cards_20260327/phase-4-delivery-map.md)
- [x] [Phase 5 开发顺序图与依赖图](abnormal_order_task_cards_20260327/phase-5-delivery-map.md)

后续规则：

1. 已完成阶段允许事后补卡，但必须基于已经验证过的代码结果回填，而不是凭印象勾选。
2. 当前优先按 Phase 4 delivery map 推进；Phase 5 只保留直接切主判前所需的最小验收与收口资产。

## 推荐执行顺序

1. 先完成 CARD-01 到 CARD-04，把 V2 表结构和账本骨架建起来。
2. 再做 CARD-05 和 CARD-06，把 query、sqlc、事务双写接通。
3. 然后用 CARD-07 做 Phase 1 验证和收口。
4. 当前继续按 Phase 4 delivery map 推进回滚与动作闭环。

## 完成记录

- [x] Phase 1 完成
- [x] Phase 2 完成
- [ ] Phase 3 完成
- [ ] Phase 4 完成
- [ ] Phase 5 完成
- [ ] 异常订单主判改造完成，可直接切换正式主判

## 当前阶段结论

1. Phase 1 和 Phase 2 已完成，且不需要再回头补骨架或事实读取。
2. Phase 3 已完成 formal mode 收口和 user_restricted 事务晋升，但入口旧预判仍待进一步下收为 hint。
3. 当前主线焦点已经切到 Phase 4：把 recovery、restrict、appeal re-decision、净值回滚和 overturn 审计链补成闭环。