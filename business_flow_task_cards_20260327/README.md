# 业务流程缺陷开发任务卡索引

日期：2026-03-27

本目录基于 [business_flow_defects_remediation_checklist_20260327.md](business_flow_defects_remediation_checklist_20260327.md) 继续拆分，目标是把 5 项缺陷拆成可独立开发、评审、测试和勾选完成的任务卡。

## 使用方式

- 每张任务卡以“单人或单小组可独立交付”为目标。
- 任务卡完成后，直接回填状态、PR 链接、验证结果和剩余风险。
- Phase 1 任务卡全部完成前，不建议安排正式上线。
- Phase 1 的并行依赖和开发顺序，见 [business_flow_task_cards_20260327/phase-1-delivery-map.md](business_flow_task_cards_20260327/phase-1-delivery-map.md)。
- Phase 2 的方案决策与实施顺序，见 [business_flow_task_cards_20260327/phase-2-delivery-map.md](business_flow_task_cards_20260327/phase-2-delivery-map.md)。
- Phase 3 的接口语义与调用方联动顺序，见 [business_flow_task_cards_20260327/phase-3-delivery-map.md](business_flow_task_cards_20260327/phase-3-delivery-map.md)。

## 任务卡列表

### Phase 1：上线阻断项

- [ ] [CARD-01 收紧预订押金抵扣前置条件](business_flow_task_cards_20260327/card-01-reservation-deposit-gate.md)
- [ ] [CARD-02 统一已实收押金抵扣模型](business_flow_task_cards_20260327/card-02-deposit-deduction-source.md)
- [ ] [CARD-03 修正预订确认占桌时机](business_flow_task_cards_20260327/card-03-reservation-table-occupation.md)
- [ ] [CARD-04 清理预订与桌台状态机联动](business_flow_task_cards_20260327/card-04-reservation-table-state-followup.md)
- [ ] [CARD-05 修正重复认领未知状态时的回调 ACK 策略](business_flow_task_cards_20260327/card-05-payment-callback-ack-strategy.md)
- [ ] [CARD-06 统一回调重复认领兜底与告警](business_flow_task_cards_20260327/card-06-payment-callback-recovery.md)

### Phase 2：高风险运营缺口

- [ ] [CARD-07 确定账单组金额单一事实来源](business_flow_task_cards_20260327/card-07-billing-group-source-of-truth.md)
- [ ] [CARD-08 实现账单组金额维护链路](business_flow_task_cards_20260327/card-08-billing-group-aggregation.md)
- [ ] [CARD-09 补齐账单组金额回归与接口校验](business_flow_task_cards_20260327/card-09-billing-group-regression.md)

### Phase 3：语义与权限一致性

- [ ] [CARD-10 修正预检接口 reservation owner 语义](business_flow_task_cards_20260327/card-10-dining-precheck-owner-semantics.md)
- [ ] [CARD-11 联动调用方收口预检权限表达](business_flow_task_cards_20260327/card-11-dining-precheck-caller-alignment.md)

### Phase 4：发布与复核

- [ ] [CARD-12 发布前回归与上线评审复核](business_flow_task_cards_20260327/card-12-release-readiness.md)

## 推荐领取顺序

1. 先做 CARD-01、CARD-03、CARD-05，尽快把最危险的行为关掉。
2. 再做 CARD-02、CARD-04、CARD-06，补足模型一致性和失败路径。
3. 然后完成 CARD-07 到 CARD-09，收口堂食账单聚合问题。
4. 最后处理 CARD-10、CARD-11，并执行 CARD-12。

## 执行拆分

- [ ] [Phase 1 开发顺序图与并行依赖图](business_flow_task_cards_20260327/phase-1-delivery-map.md)
- [ ] [Phase 2 开发顺序图与依赖图](business_flow_task_cards_20260327/phase-2-delivery-map.md)
- [ ] [Phase 3 开发顺序图与依赖图](business_flow_task_cards_20260327/phase-3-delivery-map.md)

## 执行辅助

- [ ] [发布前手工回归清单](business_flow_task_cards_20260327/manual-regression-checklist.md)

## 完成记录

- [ ] Phase 1 完成
- [ ] Phase 2 完成
- [ ] Phase 3 完成
- [ ] Phase 4 完成
- [ ] 全部任务卡完成，可重新发起上线评审