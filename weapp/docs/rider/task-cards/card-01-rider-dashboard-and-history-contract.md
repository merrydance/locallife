# CARD-01 骑手首页与历史任务合同收口

状态：未开始

优先级：P0

所属阶段：Phase 1

## 问题目标

把 dashboard/index 和 tasks/index 收口到后端真实推荐单、活跃任务、历史任务合同，解决主入口“列表能看、历史失真”的问题。

## 影响范围

- weapp/miniprogram/pages/rider/dashboard/index.ts
- weapp/miniprogram/pages/rider/dashboard/index.wxml
- weapp/miniprogram/pages/rider/tasks/index.ts
- weapp/miniprogram/pages/rider/tasks/index.wxml
- weapp/miniprogram/api/delivery.ts

## 任务内容

- [ ] 修正 dashboard 推荐单展示字段，按真实返回解释 real_distance、estimated_minutes、distance_to_pickup。
- [ ] 为 dashboard 补页内 error/retry，避免 refresh 失败后直接伪装成空列表。
- [ ] 收口 dashboard 到 tasks 的历史入口，明确用户如何进入历史任务页。
- [ ] 把 tasks/index 的请求参数改为 page 和 limit，按真实 total 或 hasMore 计算翻页。
- [ ] 为 tasks/index 补 loading、error、empty 三态，不再只打日志。

## 完成定义

- [ ] 骑手首页的推荐单、活跃单和历史入口都按真实接口运行。
- [ ] 历史任务翻页不再重复第一页。
- [ ] 首页和历史页都具备可感知的失败恢复面。

## 验证要求

- [ ] 人工验证首页冷启动、下拉刷新、抢单后的列表回流。
- [ ] 人工验证历史任务翻页至少两页。
- [ ] 执行最小相关质量检查。

## 完成记录

- [ ] 代码完成
- [ ] 历史翻页验证完成
- [ ] 回归完成