# Weapp 整体升级任务卡索引（2026-04-05）

日期：2026-04-05

> 当前执行说明
>
> 本目录用于承接 [weapp/docs/WEAPP_OVERALL_UPGRADE_EXECUTION_PLAN_2026-04-05.md](weapp/docs/WEAPP_OVERALL_UPGRADE_EXECUTION_PLAN_2026-04-05.md) 的后续拆卡。
>
> 当前先拆出 Batch 1，目标是把预订支付与结果承接簇落成可直接执行、评审和验收的任务卡。

## 使用方式

- 每张卡尽量满足单人或单小组可在一轮迭代内完成。
- 每张卡都必须同时从视觉、交互、性能三部分审视，并以后端真实 contract 为硬边界。
- 实现和 review 时必须使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。
- 完成后回填状态、PR 链接、验证结果和残余风险。

## Batch 1：预订支付与结果承接簇

- [ ] [CARD-01 预订确认页任务边界与支付真实化](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-01-reservation-confirm-contract-and-task-flow.md)
- [ ] [CARD-02 支付结果页与未知状态承接收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-02-payment-result-state-recovery.md)
- [ ] [CARD-03 预订详情与返回重入恢复链路收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-03-reservation-detail-reentry-and-context.md)

## Batch 2：外卖结算与首页性能簇

- [ ] [CARD-04 外卖首页首屏预算与渐进水合收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-04-takeout-home-budget-and-hydration.md)
- [ ] [CARD-05 外卖确认页 contract、试算与支付承接真实化](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-05-order-confirm-contract-and-result-flow.md)
- [ ] [CARD-06 外卖搜索结果态、建议态与失败恢复收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-06-search-result-state-and-retry-semantics.md)

## Batch 3：商户结算配置簇

- [ ] [CARD-07 主体申请页信息架构与分段任务流重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-07-merchant-application-information-architecture.md)
- [ ] [CARD-08 收付通进件页状态承接与阻塞任务流统一](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-08-merchant-applyment-state-and-blocking-flow.md)
- [ ] [CARD-09 结算配置跨页回流与入口一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-09-merchant-settlement-cross-page-consistency.md)

## 推荐执行顺序

1. 先做 CARD-01，清除确认页的假控件和伪任务边界。
2. 再做 CARD-02，把支付成功、失败、取消、未知结果承接成真实状态面。
3. 最后做 CARD-03，把预订详情、返回页面、回前台和重入恢复统一收口。
4. 然后做 CARD-04，先把外卖首页的首屏预算和水合策略收口。
5. 再做 CARD-05，把确认页的试算、下单和支付承接统一成可信流程。
6. 最后做 CARD-06，统一搜索结果、建议、空态和失败恢复语义。
7. 接着做 CARD-07，把主体申请页从超长滚动表单收口成清晰分段任务流。
8. 再做 CARD-08，统一进件页的状态卡、阻塞说明、刷新失败和后续动作。
9. 最后做 CARD-09，把主体申请、进件、完成页和资金入口之间的回流与入口一致性收口。

## Batch 1 完成标准

- [ ] 预订确认页不存在展示了但不生效的支付交互。
- [ ] 支付结果页不再只是静态成功页，具备成功、失败、取消、未知结果等关键状态承接。
- [ ] 用户从确认页、详情页、微信回跳、前后台切换和重试路径中都能回到可信状态。
- [ ] Batch 1 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。

## Batch 2 完成标准

- [ ] 外卖首页首屏请求和 per-item 水合明显收口，不再默认放大到随商户数线性恶化。
- [ ] 外卖确认页不再存在支付承接占位、结果不可信或弱网下流程断裂的问题。
- [ ] 搜索页明确区分建议失败、结果失败、空结果和正常结果，不再把失败伪装成空。
- [ ] Batch 2 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。

## Batch 3 完成标准

- [ ] 主体申请页与进件页不再依赖超长单页堆叠承担所有任务，用户能明确分清“填写资料”“查看状态”“处理阻塞”“继续后续动作”。
- [ ] 已接受的闭环止血项保持成立，同时页面的信息架构、跨页回流和恢复路径继续提升到统一可用水平。
- [ ] 主体申请、进件、完成页、资金入口之间的跳转与返回上下文一致，不再像多个状态中转页拼接在一起。
- [ ] Batch 3 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。