# CARD-04 实时订阅生命周期模板收口

状态：未开始

优先级：P0

所属阶段：Phase 0

## 问题目标

统一五端实时页面的冷启动、重连、前后台切换和解绑模式。

## 影响范围

- [weapp/miniprogram/utils/websocket.ts](weapp/miniprogram/utils/websocket.ts)
- `weapp/miniprogram/pages/**/dashboard/**/*.ts`
- `weapp/miniprogram/pages/notification/**/*.ts`

## 任务内容

- [ ] 提炼实时页面标准模板：何时 connect、何时 subscribe、何时 cleanup。
- [ ] 明确“异步身份或状态加载完成后必须补建订阅”的规则。
- [ ] 梳理前后台切换后的恢复策略。
- [ ] 输出可复用的页面实现规范。

## 完成定义

- [ ] 冷启动在线状态的页面可直接收到实时消息。
- [ ] 前后台切换后不需要手工操作即可恢复。

## 验证要求

- [ ] 至少用骑手侧和商户侧各验证一个页面。

## 完成记录

- [ ] 模板完成
- [ ] 双端验证完成
- [ ] 规范发布完成