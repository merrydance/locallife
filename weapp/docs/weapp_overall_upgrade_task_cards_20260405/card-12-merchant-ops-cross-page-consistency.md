# CARD-12 订单与预订运营页跨页一致性收口

状态：未开始

优先级：P1

所属批次：Batch 4

## 问题目标

把商户订单列表页和预订页收口成同一套运营系统，让状态筛选、空态语义、动作强调、失败恢复和跨页回流保持一致，不再像两个各自演化的页面域。

## 影响范围

- [weapp/miniprogram/pages/merchant/orders/list/index.ts](weapp/miniprogram/pages/merchant/orders/list/index.ts)
- [weapp/miniprogram/pages/merchant/orders/list/index.wxml](weapp/miniprogram/pages/merchant/orders/list/index.wxml)
- [weapp/miniprogram/pages/merchant/reservations/index.ts](weapp/miniprogram/pages/merchant/reservations/index.ts)
- [weapp/miniprogram/pages/merchant/reservations/index.wxml](weapp/miniprogram/pages/merchant/reservations/index.wxml)

## 已知问题

- 订单与预订都属于高频运营页，但两页在状态、动作区、筛选、空态和任务表达上还缺少统一系统感。
- 即便单页能用，跨页切换时仍容易让用户感觉像两套不同的后台界面。
- 运营域如果缺少跨页一致性，会持续放大培训成本和认知负担。

## 任务内容

- [ ] 梳理订单页和预订页的共同运营模式，统一主任务表达、状态筛选、空态说明和失败恢复口径。
- [ ] 收口两页的主次按钮、危险动作、弹层动作区和状态卡表现，减少局部自定义样式漂移。
- [ ] 检查从 dashboard 或详情回到列表页时的上下文保留和回流逻辑，避免每次都重新从默认筛选和默认位置开始。
- [ ] 形成一套可复用的运营页布局与动作模式，为后续 kitchen、complaints、claims 等页面提供参照。

## 完成定义

- [ ] 订单页和预订页在视觉和交互上明显读起来像同一套系统。
- [ ] 状态筛选、失败恢复、空态语义和主次动作规则不再各搞一套。
- [ ] 跨页回流更稳，用户返回列表后不会总是丢失上下文。

## 验证要求

- [ ] 人工验证从 dashboard 进入订单页、预订页，再返回列表或切页后的上下文保留。
- [ ] 验证两页在空态、失败态、筛选切换和危险动作确认上的一致性。
- [ ] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查跨页一致性、视觉系统感和状态恢复。

## 完成记录

- [ ] 共同运营模式梳理完成
- [ ] 跨页一致性改造完成
- [ ] 列表回流与筛选回归完成
- [ ] review 完成