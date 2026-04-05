# CARD-08 收付通进件页状态承接与阻塞任务流统一

状态：未开始

优先级：P1

所属批次：Batch 3

## 问题目标

在已完成基础状态对齐和刷新失败承接的前提下，把进件页做成一个真正可理解的任务流页面，而不是状态卡、阻塞说明、签约链接和资金动作的混合中转站。

## 影响范围

- [weapp/miniprogram/pages/merchant/settings/applyment/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/index.ts)
- [weapp/miniprogram/pages/merchant/settings/applyment/index.wxml](weapp/miniprogram/pages/merchant/settings/applyment/index.wxml)
- [weapp/miniprogram/pages/merchant/settings/applyment/completed/index.ts](weapp/miniprogram/pages/merchant/settings/applyment/completed/index.ts)
- [weapp/miniprogram/pages/merchant/settings/applyment/completed/index.wxml](weapp/miniprogram/pages/merchant/settings/applyment/completed/index.wxml)

## 当前前提

- 进件状态文案、真实状态消费、刷新失败页内承接、完成页自循环修复等止血项已在既有 merchant 配置中心任务中接受。
- 本卡聚焦的是任务流表达、阻塞态层级、下一步动作和跨页观感统一，而不是重复收口已接受问题。

## 已知问题

- 进件页混合了承载状态、阻塞说明、签约、银行资料重提和跳转动作，主任务不够清晰。
- 当前页面更像状态中转站，而不是一条明确的结算开通任务流。
- 已有状态承接虽然更正确，但视觉与交互层级仍然偏重、偏散。

## 任务内容

- [ ] 重构进件页的信息层级，区分当前状态、阻塞原因、建议动作和次要补充信息。
- [ ] 收口签约链接、重提银行资料、刷新状态、查看完成页等动作的主次顺序。
- [ ] 检查阻塞态、刷新失败、已完成但仍需后续动作等边界分支，避免用户把不同状态误读成同一阶段。
- [ ] 让 completed 页面与主进件页在视觉、动作和回流上成为同一任务流的一部分，而不是分裂的两个中转页。

## 完成定义

- [ ] 进件页的主状态、阻塞原因和下一步动作一眼可读。
- [ ] completed 页与主进件页回流自然，不再制造新的状态岔路。
- [ ] 进件任务流在视觉和交互上比当前更统一、更易理解。

## 验证要求

- [ ] 人工验证未申请、审核中、被驳回、待签约、已完成、刷新失败六类主要场景。
- [ ] 验证 completed 页返回主页、从主页进入完成页、重进页面后的状态表现。
- [ ] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查视觉层级、反馈与提示、状态恢复。

## 完成记录

- [ ] 进件状态流重梳理完成
- [ ] 主页与完成页结构重构完成
- [ ] 阻塞态与回流回归完成
- [ ] review 完成