# CARD-02 骑手任务详情与状态流转闭环

状态：未开始

优先级：P0

所属阶段：Phase 1

## 问题目标

修复 task-detail/index 使用错误权限接口的问题，并把任务详情、状态流转、回流刷新做成真实闭环。

## 影响范围

- weapp/miniprogram/pages/rider/task-detail/index.ts
- weapp/miniprogram/pages/rider/task-detail/index.wxml
- weapp/miniprogram/pages/rider/task-detail/index.json
- weapp/miniprogram/pages/rider/dashboard/index.ts
- weapp/miniprogram/pages/rider/tasks/index.ts
- locallife/api/delivery.go
- locallife/logic/delivery_access.go

## 任务内容

- [ ] 明确骑手任务详情的唯一数据源，不能再调用 owner-only 的 /v1/delivery/order/:order_id。
- [ ] 如果沿用现有后端接口不足，则补骑手可访问的详情接口或改用活跃/历史列表缓存回填。
- [ ] 收口 deadline_desc、步骤映射、备注、订单号等展示字段，全部基于真实响应计算。
- [ ] 修复页面 JSON 组件注册缺失和详情失败壳。
- [ ] 验证状态流转后对 dashboard/tasks/task-detail 三页的刷新回流。

## 完成定义

- [ ] 骑手从首页或历史页进入任务详情可稳定成功。
- [ ] 详情页上的取餐、确认取餐、开始配送、确认送达都能回流到列表页。
- [ ] 无权限或不存在场景有明确页内反馈。

## 验证要求

- [ ] 人工验证 assigned、picking、picked、delivering 四种状态分支。
- [ ] 人工验证详情页返回大厅和返回上一页行为。
- [ ] 执行最小相关质量检查或后端定向测试。

## 完成记录

- [ ] 代码完成
- [ ] 详情链路验证完成
- [ ] 回归完成