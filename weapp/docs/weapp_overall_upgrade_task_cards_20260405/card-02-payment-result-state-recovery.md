# CARD-02 支付结果页与未知状态承接收口

状态：代码修复完成，待人工回归

优先级：P0

所属批次：Batch 1

## 问题目标

把支付后的结果承接从静态成功页升级成真实状态面，明确区分成功、失败、取消和未知结果，并支持回查和重入恢复。

## 影响范围

- [weapp/miniprogram/pages/orders/success/index.ts](weapp/miniprogram/pages/orders/success/index.ts)
- [weapp/miniprogram/pages/orders/success/index.wxml](weapp/miniprogram/pages/orders/success/index.wxml)
- 相关支付结果查询、订单状态查询和回跳承接逻辑

## 已知问题

- 当前结果页主要依赖路由参数拼接成功文案，不是按真实结果状态驱动。
- 微信回跳、弱网、重新进入、未知结果等高风险状态没有被单独承接。
- 结果页可信度不足，会放大用户对支付链路的不信任。

## 任务内容

- [x] 核对支付结果与订单状态的真实 contract，确认页面应承接的状态集合。
- [x] 把结果页拆成成功、失败、取消、未知结果等关键状态分支，而不是单一成功态。
- [x] 为未知结果提供回查入口或自动回查策略，不再把未知结果伪装成成功或失败。
- [x] 优化结果页的信息架构，移除与主任务无关的干扰信息，让结果说明、下一步动作和恢复入口更聚焦。
- [x] 处理用户重新进入、回前台、重复进入结果页时的状态恢复。

## 完成定义

- [x] 结果页不再是静态成功壳，而是按真实支付/订单状态承接。
- [x] 未知结果有明确回查路径。
- [x] 用户从微信回跳或弱网重入后不会被错误地落在成功页。

## 验证要求

- [ ] 人工验证成功、失败、取消、未知结果四类主要场景。
- [ ] 验证微信回跳、回前台、重新打开结果页时的恢复表现。
- [x] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查反馈与提示、状态恢复和契约闭环。

## 完成记录

- [x] 状态集合与 contract 核对完成
- [x] 结果页状态面重构完成
- [x] 2026-04-06 补做共享支付状态语义修复，明确区分 `paid`、`failed` 和 `unknown`，不再把 `failed`/`closed` 承接为 `unknown`
- [ ] 回查与重入恢复验证完成
- [x] review 完成

PR 链接、验证结果和残余风险见 [weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md](weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md)。