# CARD-07 骑手侧运行时验证与弱网回归

状态：部分实现，待真机回归

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

## 任务内容

- [x] 对 dashboard、task-detail、tasks、deposit、exception 做 loading、success、empty、error、retry 五态验证。
- [ ] 对抢单、取餐、确认取餐、开始配送、确认送达做主链验证。
- [ ] 对异常上报、申诉、追偿支付、充值、提现做弱网和失败恢复验证。
- [ ] 对注册页和入口做最终一致性检查。

## 完成定义

- [ ] rider 子包通过统一运行时验证。
- [ ] 剩余风险只保留明确记录的非阻断项。

## 验证要求

- [x] 执行最小相关质量检查。
- [ ] 真机或开发者工具完成主链和弱网回归。
- [ ] 最终重新评分达到 95 分以上。

## 完成记录

- [x] 质量检查完成
- [ ] 主链回归完成
- [ ] 95 分验收完成

## 本次实现说明

- dashboard 已补初始化失败页内错误态、网络恢复后的自动重刷与在线态重连兜底。
- tasks 已补分页弱网失败后的页内重试，避免“首屏正常、翻页只 toast 无恢复入口”的退化。
- exception、deposit、task-detail 的页内 error/retry 与主操作反馈已在前序批次补齐，本次统一纳入运行态卡验证。
- 自动验证已完成：`npm run compile` 与 `npm run lint:all` 通过。
- 仍待真机或开发者工具执行主链、支付链和弱网回归，因此暂不标记 95 分验收完成。