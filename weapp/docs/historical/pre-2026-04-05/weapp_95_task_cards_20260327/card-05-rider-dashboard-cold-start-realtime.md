# CARD-05 修复骑手首页冷启动实时链路

状态：进行中（代码完成，待人工回归）

优先级：P0

所属阶段：Phase 1

## 问题目标

让骑手在冷启动进入首页且已在线时，也能立即收到实时新单和配送池变更。

## 影响范围

- [weapp/miniprogram/pages/rider/dashboard/index.ts](weapp/miniprogram/pages/rider/dashboard/index.ts)
- [weapp/miniprogram/utils/websocket.ts](weapp/miniprogram/utils/websocket.ts)

## 任务内容

- [x] 调整首页初始化时机，在在线状态异步加载完成后补建 websocket 监听。
- [x] 统一 `onLoad`、`onShow`、手动上下线后的订阅行为。
- [x] 防止重复注册监听或冷启动漏订阅。

## 完成定义

- [ ] 冷启动在线骑手可直接收到新单事件。
- [ ] 不需要切后台或手动上下线才能恢复。

## 验证要求

- [ ] 人工验证冷启动在线场景。
- [ ] 人工验证上下线切换与前后台切换。
- [x] 编辑器诊断与定向 lint 校验通过。

## 完成记录

- [x] 代码完成
- [ ] 冷启动验证完成
- [ ] 回归完成

补充说明：

- 已在骑手首页增加统一的在线运行时入口 `enterOnlineRuntime`，用于收口冷启动在线、前台恢复和手动上线后的刷新与 websocket 订阅。
- 当前已完成编辑器诊断、定向 eslint 校验；仍需在真机或开发者工具中验证冷启动在线和前后台切换场景。