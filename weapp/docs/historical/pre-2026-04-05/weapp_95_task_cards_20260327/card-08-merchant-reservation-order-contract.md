# CARD-08 修复商户预订订单过滤与分页错位

状态：进行中（代码完成，待人工回归）

优先级：P0

所属阶段：Phase 1

## 问题目标

让商户预订页跳转到订单流转列表时，分页、筛选和空态都遵循真实合同。

## 影响范围

- [weapp/miniprogram/pages/merchant/reservations/index.ts](weapp/miniprogram/pages/merchant/reservations/index.ts)
- [weapp/miniprogram/pages/merchant/orders/list/index.ts](weapp/miniprogram/pages/merchant/orders/list/index.ts)
- 相关订单 API 文件

## 任务内容

- [x] 将 `order_type=reservation` 下推到服务请求层。
- [x] 修正 `hasMore` 与空态逻辑，使其和过滤后的真实结果一致。
- [x] 清理客户端分页后再过滤的代码路径。

## 完成定义

- [ ] 预订单列表结果可信。
- [ ] 分页不再漂移。

## 验证要求

- [ ] 人工验证第一页、翻页、空态三类场景。
- [ ] 校验列表总量与详情进入链路。

## 完成记录

- [x] 合同改造完成
- [ ] 分页回归完成
- [ ] 空态回归完成

补充说明：

- 商户订单列表已改为直接消费带 `total` 的分页结果，并支持 `order_type` 请求参数，不再在前端拿到一页后再做本地业务过滤。
- 为保证分页真值，这次同时补齐了后端商户订单列表的 `order_type` 可选过滤合同。