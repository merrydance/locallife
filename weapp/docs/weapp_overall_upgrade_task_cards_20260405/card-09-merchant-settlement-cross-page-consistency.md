# CARD-09 结算配置跨页回流与入口一致性收口

状态：未开始

优先级：P1

所属批次：Batch 3

## 问题目标

把主体申请、进件、完成页和资金入口之间的跨页关系收口成一个统一系统，避免用户在多个页面之间来回跳转时失去上下文或误判当前阶段。

## 影响范围

- [weapp/miniprogram/pages/merchant/settings/application/index.ts](weapp/miniprogram/pages/merchant/settings/application/index.ts)
- [weapp/miniprogram/pages/merchant/settings/applyment/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/index.ts)
- [weapp/miniprogram/pages/merchant/settings/applyment/completed/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/completed/index.ts)
- [weapp/miniprogram/pages/merchant/finance/index.ts](weapp/miniprogram/pages/merchant/finance/index.ts)

## 当前前提

- finance 对真实后端状态的基础消费、进件 completed 页自循环修复等止血项已被接受。
- 本卡聚焦的是跨页入口、返回路径、当前阶段提示和系统级统一感，而不是重复处理已接受状态枚举问题。

## 已知问题

- 用户在主体申请、进件、完成页、资金入口之间切换时，容易把它们看成多个相邻但不统一的中转页。
- 即便单页局部已经更正确，跨页主任务和返回上下文仍然可能不够连续。
- 高风险流程如果缺少统一入口和回流逻辑，会持续制造“页面都对，但任务感是碎的”。

## 任务内容

- [ ] 梳理主体申请、进件、完成页和 finance 的入口矩阵，明确每个状态下用户应该从哪进、往哪去。
- [ ] 统一跨页返回、完成后回流、失败后回看、从 finance 进入进件或主体页的上下文规则。
- [ ] 收口跨页文案和状态提示，让用户知道自己当前处在“资料准备”“进件审核”“签约完成”“资金可用”中的哪一段。
- [ ] 检查这些页面在视觉骨架、动作按钮和状态卡上的一致性，避免同一任务流像多套页面系统拼接。

## 完成定义

- [ ] 用户在结算配置链中始终知道自己当前所处阶段和下一步动作。
- [ ] 各页面的进入和返回路径清楚，不再出现多条回流逻辑互相打架。
- [ ] 结算配置链在整体观感上像一个系统，而不是若干单页勉强接起来。

## 验证要求

- [ ] 人工验证从 config/finance 进入、主体申请后进入进件、进件完成后回看、被驳回后重提四类主要路径。
- [ ] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查跨页一致性、状态恢复和主任务清晰度。

## 完成记录

- [ ] 入口矩阵与回流路径梳理完成
- [ ] 跨页一致性改造完成
- [ ] 主路径人工回归完成
- [ ] review 完成