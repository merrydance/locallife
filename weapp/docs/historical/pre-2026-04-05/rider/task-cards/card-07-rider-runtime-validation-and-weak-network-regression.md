# CARD-07 骑手侧运行时验证与弱网回归

状态：代码级验证完成，待真机回归

优先级：P0

所属阶段：Phase 4

## 问题目标

在主链和领域页完成收口后，对骑手侧做统一运行时验证，避免再次出现“类型无错、真实接口失配”的交付问题。

## 影响范围

- weapp/miniprogram/pages/rider/**
- weapp/miniprogram/api/rider.ts
- weapp/miniprogram/api/delivery.ts
- weapp/miniprogram/api/rider-basic-management.ts
- weapp/miniprogram/api/appeals-customer-service.ts
- weapp/miniprogram/utils/request.ts
- weapp/miniprogram/utils/network-monitor.ts
- weapp/miniprogram/utils/websocket.ts
- locallife/api/notification.go
- locallife/logic/delivery_broadcast.go
- locallife/worker/task_send_notification.go
- locallife/websocket/**

## 任务内容

- [x] 对 dashboard、task-detail、tasks、deposit、claims、claims/detail 做 loading、success、empty、error、retry 五态验证。
- [x] 对抢单、取餐、确认取餐、开始配送、确认送达做代码级主链验证，并核对失败后 reconcile 回读路径。
- [x] 对申诉、追偿支付、充值、提现做代码级弱网与失败恢复验证。
- [x] 对注册页和入口做最终一致性检查。

## 完成定义

- [x] rider 子包完成统一代码级运行时验证。
- [x] 剩余风险已收束为明确记录的非阻断项。

## 验证要求

- [x] 执行最小相关质量检查。
- [ ] 真机或开发者工具完成主链和弱网回归。
- [ ] 最终重新评分达到 95 分以上。

## 完成记录

- [x] 质量检查完成
- [x] 代码级链路核对完成
- [ ] 真机主链回归完成
- [ ] 95 分验收完成

## 本次实现说明

- dashboard 已补初始化失败页内错误态、网络恢复后的自动重刷、在线态重连兜底，以及关键动作失败后的真实状态回读。
- tasks 已补分页弱网失败后的页内重试，避免“首屏正常、翻页只 toast 无恢复入口”的退化。
- deposit 与 claims/detail 均已接入支付后的状态轮询或详情刷新，不再只依赖微信支付 success callback。
- rider websocket 客户端已补 ACK，上游后端的 ACK、回放、背压队列和 retry 机制与当前前端消费链闭合。
- 自动验证已完成：`npm run quality:check`、`go test ./websocket`、`go test ./logic -run 'TestListNearbyBroadcastRiders|TestBroadcastNewOrderNotification'` 通过。
- 当前真值流程图与后端实时支撑核对结果见：`weapp/docs/historical/pre-2026-04-05/rider/RIDER_RUNTIME_REALTIME_FLOW_MAP.md`。
- 仍待真机或开发者工具执行主链、支付链和弱网回归，因此暂不标记 95 分验收完成。

## 剩余风险

- 当前环境未执行开发者工具弱网面板或真机回归，因此无法实证覆盖支付回查时延、后台切前台后的 websocket 稳定性和微信支付取消后的真实用户体验。
- rider 工作台当前只消费配送池新单与消失两类 websocket 消息，通知类 websocket 消息尚未在 rider UI 形成直接可见收益。
- 定位类实时能力仍停留在后端可用、前端未承接阶段，不纳入本卡通过范围。