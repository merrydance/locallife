# CARD-03 骑手积分与高值单资格页重建

状态：未开始

优先级：P0

所属阶段：Phase 2

## 问题目标

把 credit/index 从错误 DTO 页面重建为真实高值单资格页面，正确消费 premium score 与日志合同。

## 影响范围

- weapp/miniprogram/pages/rider/credit/index.ts
- weapp/miniprogram/pages/rider/credit/index.wxml
- weapp/miniprogram/api/rider-basic-management.ts
- locallife/api/rider.go

## 任务内容

- [ ] 用真实字段 premium_score、can_accept_premium_order、logs 重建页面状态。
- [ ] 重新定义等级文案与展示规则，避免继续依赖不存在的 score_level。
- [ ] 正确映射积分历史中的 change_type、change_amount、remark、related_order_id。
- [ ] 明确 credit/index 是否需要正式入口；若需要则补入口，若不需要则下线路由。

## 完成定义

- [ ] 积分页在真实接口下可稳定展示。
- [ ] 用户能理解自己是否具备高值单资格，以及最近积分变更原因。
- [ ] 页面入口决策明确，不再处于注册但无入口状态。

## 验证要求

- [ ] 人工验证有日志和无日志两种场景。
- [ ] 验证正负积分变更展示。
- [ ] 执行最小相关质量检查。

## 完成记录

- [ ] 代码完成
- [ ] 页面入口决策完成
- [ ] 回归完成