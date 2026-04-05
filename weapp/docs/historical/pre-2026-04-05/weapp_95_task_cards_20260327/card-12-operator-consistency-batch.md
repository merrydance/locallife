# CARD-12 运营商侧一致性补齐批次

状态：已拆分待执行

优先级：P1

所属阶段：Phase 3

## 问题目标

统一运营商侧列表、详情、审核、提现、统计的权限表达、分页合同和状态反馈。

详细审计与拆分见：

- `weapp/docs/historical/pre-2026-04-05/operator/OPERATOR_95_FULL_AUDIT_EXECUTION_PLAN_2026-03-28.md`
- `weapp/docs/historical/pre-2026-04-05/operator/OPERATOR_PHASE0_BASELINE_LEDGER_2026-03-28.md`
- `weapp/docs/historical/pre-2026-04-05/operator/task-cards/README.md`

## 影响范围

- `weapp/miniprogram/pages/operator/**`

## 任务内容

- [ ] 建立运营商区域权限矩阵。
- [ ] 收口 merchant、rider、appeal、finance 四类页面的分页与筛选合同。
- [ ] 补齐审核完成后的刷新、提示和回流。

## 完成定义

- [ ] 运营商页权限表达与后端一致。
- [ ] 核心列表与详情页状态完整。

## 验证要求

- [ ] 角色与区域权限回归。
- [ ] 审核与提现主流程回归。

## 完成记录

- [ ] 权限矩阵完成
- [ ] 页面收口完成
- [ ] 回归完成

## 当前拆分状态

- [x] 已完成运营侧注册页与孤儿页全量审计
- [x] 已形成 phase 修复计划
- [x] 已拆分 5 张可执行任务卡
- [ ] 进入逐卡修复阶段