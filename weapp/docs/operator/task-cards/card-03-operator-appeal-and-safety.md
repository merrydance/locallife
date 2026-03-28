# CARD-03 运营申诉与食安链路补齐

状态：进行中

优先级：P0

所属阶段：Phase 1

## 问题目标

把申诉审核和食安事件从“能看部分信息”补到“审核可信、信息完整、动作可追踪”的状态。

## 影响范围

1. weapp/miniprogram/pages/operator/appeal/list/index
2. weapp/miniprogram/pages/operator/appeal/detail/index
3. weapp/miniprogram/pages/operator/safety/report/index
4. weapp/miniprogram/pages/operator/safety/detail/index
5. weapp/miniprogram/api/appeals-customer-service.ts
6. weapp/miniprogram/api/operator-analytics.ts
7. weapp/miniprogram/api/operator-basic-management.ts

## 任务内容

- [x] appeal/list/index 接完整分页合同和真实状态集合。
- [x] appeal/detail/index 对齐 operator appeal detail 真实字段，补齐 timeline、evidence_files、related_order。
- [x] 审核完成后的返回、刷新和提示统一收口。
- [x] safety/report/index 补 submitSafetyReport 的页面入口，或明确为只读页并下掉无效预期。
- [x] safety/detail/index 补错误壳、已处理态只读和恢复商户结果回流。

## 完成定义

- [ ] 申诉列表与详情完全对齐后端字段和状态。
- [ ] 食安事件至少具备查看、提交、处理三类清晰能力。
- [ ] 审核与处置动作具备成功、失败和回流表达。

## 验证要求

- [x] appeal 与 safety 相关页面的编辑器诊断通过。
- [x] 运行 npm run quality:check。
- [ ] 人工验证分页、审核通过、审核驳回、食安处置和恢复商户流程。

## 完成记录

- [x] 申诉链完成
- [x] 食安链完成
- [x] 回流链完成