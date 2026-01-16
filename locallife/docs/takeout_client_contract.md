# 外卖客户端字段/状态契约（小程序/商户/骑手）

目标：提供统一的状态/字段语义与按钮行为约束，确保多端一致。

## 适用范围
- 用户端（小程序）订单列表/详情
- 商户端订单看板
- 骑手端配送流程

## 状态流（订单）
```
ready -> courier_accepted -> picked -> delivering -> rider_delivered -> user_delivered
```
- `courier_accepted`: 骑手已接单，尚未到店取餐。
- `picked`: 骑手已取餐。
- `delivering`: 配送中。
- `rider_delivered`: 骑手已送达，等待用户确认。
- `user_delivered`: 用户确认收货（或自动确认）。

## 响应字段（订单）
- `status`: 订单状态（见上）。
- `status_hint`: 短文案提示（可为空）。
- `badges`: jsonb 数组，建议结构 `{text, type, locale?}`。
- `pickup_code_masked`: 取餐码脱敏展示。
- `actions`: 用户端可执行操作数组。
- 关键时间戳：`prep_start_at`, `ready_at`, `courier_accept_at`, `picked_at`, `rider_delivered_at`, `user_delivered_at`。

## 用户端 actions 对照（仅用户端）
- `pending`: `pay`, `cancel`
- `paid`: `cancel`, `urge`
- `preparing`/`ready`: `urge`
- `courier_accepted`/`picked`/`delivering`: `urge`
- `rider_delivered`: `confirm`, `complain`
- `user_delivered`/`completed`: `complain`

> 说明：`actions` 为服务端判定结果，前端仅需按 `actions` 渲染按钮。

## 骑手端流程约束
- 取餐：`assigned -> picking -> picked`
- 配送：`picked -> delivering`
- 送达：`delivering -> rider_delivered`
- 围栏事件默认不自动推进状态（除非开关启用）。

## 商户端建议
- 以 `status` 为准展示流转节点；`status_hint` 与 `badges` 作为次级展示。

## 兼容性要求
- 旧客户端可忽略新增字段（`status_hint`, `badges`, `pickup_code_masked`）。
- 新客户端优先使用 `status` + `actions` 判断按钮可用性。

