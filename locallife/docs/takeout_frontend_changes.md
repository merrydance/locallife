# 外卖前端改动清单（小程序/商户/骑手）

目标：对齐新增状态与字段，前端按 `actions` 控制按钮，完成联调与验收。

## 用户端（小程序）
- 订单列表/详情展示新增字段：`status_hint`, `badges`, `pickup_code_masked`。
- 状态文案更新：展示 `courier_accepted/picked/rider_delivered/user_delivered`。
- 按 `actions` 渲染按钮：`pay/cancel/urge/confirm/complain`。
- 确认收货：支持 `delivering`/`rider_delivered` -> `user_delivered`。
- 催单：覆盖 `courier_accepted/picked/delivering/rider_delivered`。

### 完成情况（小程序）
- 已完成 TS/JS 同步适配与渲染逻辑更新。
- 关键实现位置：
  - [weapp/miniprogram/api/order.ts](weapp/miniprogram/api/order.ts)
  - [weapp/miniprogram/adapters/order.ts](weapp/miniprogram/adapters/order.ts)
  - [weapp/miniprogram/adapters/order-card.ts](weapp/miniprogram/adapters/order-card.ts)
  - [weapp/miniprogram/pages/orders/list/index.ts](weapp/miniprogram/pages/orders/list/index.ts)
  - [weapp/miniprogram/pages/orders/detail/index.ts](weapp/miniprogram/pages/orders/detail/index.ts)
  - JS 同步文件：
    - [weapp/miniprogram/api/order.js](weapp/miniprogram/api/order.js)
    - [weapp/miniprogram/adapters/order.js](weapp/miniprogram/adapters/order.js)
    - [weapp/miniprogram/adapters/order-card.js](weapp/miniprogram/adapters/order-card.js)
    - [weapp/miniprogram/pages/orders/list/index.js](weapp/miniprogram/pages/orders/list/index.js)
    - [weapp/miniprogram/pages/orders/detail/index.js](weapp/miniprogram/pages/orders/detail/index.js)

## 商户端
- 订单列表/详情兼容新状态与 `status_hint/badges`。
- 出餐/备餐节点与状态展示一致。

## 骑手端
- 状态节点：`assigned -> picking -> picked -> delivering -> rider_delivered`。
- 位置上报：可选传 `delivery_id` 与 `source`。
- 围栏事件默认不自动推进；若启用开关，展示提示/状态变更。

## 联调与验收
- 回归用例：
  - 正常流程：ready -> courier_accepted -> picked -> delivering -> rider_delivered -> user_delivered。
  - 异常流程：取消闸门、异常通道、催单。
  - 围栏触发：到店/到达收货点事件记录（开关关闭与开启）。
- 验收产物：截图/录屏 + 接口响应对照。

### 小程序联调步骤（建议）
1) 列表与详情校验：
  - 确认 `status_hint/badges/actions` 正常显示，按钮仅按 `actions` 出现。
2) 状态流校验（外卖）：
  - `courier_accepted -> picked -> delivering -> rider_delivered -> user_delivered` 文案与卡片排序正确。
3) 用户动作校验：
  - `pay/cancel/urge/confirm` 入口可用；确认收货成功后刷新详情。
