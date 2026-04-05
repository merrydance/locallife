# CARD-14 骑手工作台与历史任务回流一致性收口

状态：未开始

优先级：P1

所属批次：Batch 5

## 问题目标

把骑手首页和历史任务页收口成同一套工作台体验，统一入口、上下文回流、列表语义和失败恢复，而不是“首页一个世界、历史任务另一个世界”。

## 影响范围

- [weapp/miniprogram/pages/rider/dashboard/index.ts](weapp/miniprogram/pages/rider/dashboard/index.ts)
- [weapp/miniprogram/pages/rider/dashboard/index.wxml](weapp/miniprogram/pages/rider/dashboard/index.wxml)
- [weapp/miniprogram/pages/rider/tasks/index.ts](weapp/miniprogram/pages/rider/tasks/index.ts)
- [weapp/miniprogram/pages/rider/tasks/index.wxml](weapp/miniprogram/pages/rider/tasks/index.wxml)

## 当前前提

- 历史任务页真实分页 contract、基础错误壳和主入口跳转已在既有骑手任务卡中处理或纳入处理范围。
- 本卡聚焦首页与工具页之间的系统一致性，不重复追踪底层 contract 止血。

## 已知问题

- 首页与历史任务页虽然服务同一骑手工作流，但读起来仍像两个独立页面域。
- 入口关系、返回路径和列表语义仍可能让骑手在回看历史与返回工作台时丢上下文。
- 如果首页和历史页不统一，培训成本和日常认知负担会持续偏高。

## 任务内容

- [ ] 梳理骑手首页到历史任务页的入口、返回、筛选和状态语义，明确两页的职责边界。
- [ ] 统一任务卡布局、状态标签、空态/失败态文案和列表回流体验。
- [ ] 检查从历史列表进入详情、再回到列表或首页时的上下文保留。
- [ ] 形成一套可复用的骑手工作台页模式，为后续 claim、deposit 等页提供统一参照。

## 完成定义

- [ ] 首页和历史页在视觉、动作和状态语义上明显属于同一套系统。
- [ ] 返回路径和列表回流稳定，不再频繁丢失任务上下文。
- [ ] 历史页不再只是“能翻页的列表”，而是骑手工作台的一部分。

## 验证要求

- [ ] 人工验证首页进入历史、历史进入详情、详情返回历史、返回首页四类主要路径。
- [ ] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查跨页一致性、状态恢复和列表高频使用体验。

## 完成记录

- [ ] 工作台与历史页边界梳理完成
- [ ] 跨页一致性改造完成
- [ ] 回流与上下文回归完成
- [ ] review 完成