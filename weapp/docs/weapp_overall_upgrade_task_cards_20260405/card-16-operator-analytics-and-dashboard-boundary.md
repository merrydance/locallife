# CARD-16 运营分析页与首页关系重构

状态：未开始

优先级：P1

所属批次：Batch 6

## 问题目标

明确运营首页与分析页的职责边界，让 dashboard 负责当前待办和决策，analytics 负责深度观察与对比分析，避免两页互相复制摘要又都不够专注。

## 影响范围

- [weapp/miniprogram/pages/operator/dashboard/index.ts](weapp/miniprogram/pages/operator/dashboard/index.ts)
- [weapp/miniprogram/pages/operator/dashboard/index.wxml](weapp/miniprogram/pages/operator/dashboard/index.wxml)
- [weapp/miniprogram/pages/operator/analytics/index.ts](weapp/miniprogram/pages/operator/analytics/index.ts)
- [weapp/miniprogram/pages/operator/analytics/index.wxml](weapp/miniprogram/pages/operator/analytics/index.wxml)

## 当前前提

- analytics 的基础聚合和 dashboard 的部分入口关系已在既有 operator 任务卡中收口。
- 本卡聚焦的是两页之间的分工、跨页导航和系统化观感，不重复追踪已接受的基础统计修复。

## 已知问题

- 首页和分析页的内容边界仍然偏模糊，存在重复摘要和角色漂移。
- 用户容易在首页看到大量分析信息，在分析页又看不到足够聚焦的深度路径。
- 如果 dashboard 和 analytics 继续混用职责，运营控制台会长期缺乏稳定的信息架构。

## 任务内容

- [ ] 梳理 dashboard 与 analytics 的职责边界，明确哪些信息必须留在首页，哪些应该进入分析页。
- [ ] 统一两页的入口关系、返回关系和说明文案，避免重复摘要和重复解释。
- [ ] 检查 analytics 的筛选、排行、区域分析与首页快捷入口之间的衔接，让用户从首页进入分析页后能自然继续任务。
- [ ] 收口首页与分析页的组件组合、卡片层级和状态承接方式，减少“两个页面像两套系统”的感觉。

## 完成定义

- [ ] 首页与分析页职责清楚，不再互相复制摘要。
- [ ] 用户能从首页顺滑进入分析页继续深挖，而不是重新理解一遍页面逻辑。
- [ ] 两页的视觉层级、文案和入口关系明显更像同一套运营控制台。

## 验证要求

- [ ] 人工验证首页进入分析页、切换筛选、返回首页三类主要路径。
- [ ] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查跨页一致性、首屏优先级和信息架构边界。

## 完成记录

- [ ] dashboard / analytics 职责梳理完成
- [ ] 跨页关系重构完成
- [ ] 主路径回归完成
- [ ] review 完成