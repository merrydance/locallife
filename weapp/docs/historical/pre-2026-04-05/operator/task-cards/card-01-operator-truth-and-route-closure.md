# CARD-01 运营侧真值与路由收口

状态：进行中

优先级：P0

所属阶段：Phase 0

## 问题目标

锁定运营侧唯一真相，停止统计、DTO、路由和页面能力继续漂移。

## 影响范围

1. weapp/miniprogram/pages/operator/dashboard/index
2. weapp/miniprogram/pages/operator/analytics/index
3. weapp/miniprogram/pages/operator/dashboard/dashboard
4. weapp/miniprogram/pages/operator/merchants/list/list
5. weapp/miniprogram/app.json

## 任务内容

- [x] 修正 dashboard/index 的统计窗口和标签语义。
- [x] 修正 analytics/index 的近 7 天指标计算，不再使用最后一天趋势点冒充周期值。
- [ ] 建立注册页与孤儿页对照表，明确每个孤儿页的去向：迁移、合并或删除。
- [x] 把线上商户管理应保留的能力从 merchants/list/list 迁移回注册页，或明确反向切路由。
- [x] 清理 dashboard/dashboard 对孤儿页的依赖。

## 完成定义

- [ ] 首页、分析页的统计标签和统计口径完全一致。
- [ ] operator 子包不再存在承载真实能力的孤儿页。
- [ ] 注册页与仓库中的实际能力一致。

## 验证要求

- [x] dashboard/index 和 analytics/index 的编辑器诊断通过。
- [x] 运行 npm run quality:check。
- [ ] 人工验证今日、近 7 天、近 30 天三种口径切换。

## 完成记录

- [x] 统计真值修复完成
- [ ] 孤儿页去向确认完成
- [x] 路由收口完成