# CARD-09 补齐账单组金额回归与接口校验

状态：进行中

优先级：P1

所属阶段：Phase 2

## 问题目标

验证账单组金额在列表、详情、关台和堂食相关接口中的表现一致。

## 影响范围

- [locallife/api/billing_group.go](locallife/api/billing_group.go)
- [locallife/api/dining_session.go](locallife/api/dining_session.go)

## 任务内容

- [x] 对账单组列表、详情、堂食预检或关台相关接口做金额回归。
- [x] 检查是否仍有接口直接返回未维护的原始字段。
- [x] 必要时补充 API 层集成测试或 handler 单测。

## 完成定义

- [x] 同一账单组在多个接口中金额一致。
- [x] 不再出现“列表是 0、详情是有值”或相反情况。

## 验证要求

- [x] 增加 handler 层回归测试。
- [ ] 按 [business_flow_task_cards_20260327/manual-regression-checklist.md](business_flow_task_cards_20260327/manual-regression-checklist.md) 手工回归拼桌、部分支付、关台三个场景。

## 依赖与风险

- 依赖 CARD-08 实现金额维护链路。

## 当前结论

- 已补 `billing_group` handler 回归测试，确认列表和创建返回值走聚合金额，而非主表旧值。
- 已补 `joinBillingGroup` handler 回归测试，确认拼桌/加入账单组返回值同样走聚合金额口径。
- 已统一 `billing_group.go` 与 `dining_session.go` 的返回路径，不再直接透出 `billing_groups` 主表金额字段。
- 已整理手工回归清单，便于在真实多人拼桌、部分支付、关台链路上收口端到端风险。
- 仍缺少手工场景回归，因此本卡保留为“进行中”。

## 完成记录

- [x] 测试完成
- [ ] 手工回归完成
- [ ] 评审完成