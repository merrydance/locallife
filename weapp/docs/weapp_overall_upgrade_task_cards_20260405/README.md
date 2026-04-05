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

## 自动闭环执行建议

- 这套任务卡可以直接作为 [.github/prompts/general-task-loop.prompt.md](.github/prompts/general-task-loop.prompt.md) 的输入来源，借用 Delivery Loop Orchestrator 做全自动闭环执行。
- 推荐一次只跑一个 batch，一个 card 对应一个完整闭环单元。
- 每张 card 通过 review 和 doc-sync 决策后，再自动切到下一张 card。
- 不建议直接把 16 张卡一次性塞进同一轮自动执行，这会显著放大 UI 回归面和 review 噪音。

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

## Batch 4：商户预订运营簇

- [ ] [CARD-10 商户预订页日视图、汇总与主任务重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-10-merchant-reservations-day-view-and-task-focus.md)
- [ ] [CARD-11 商户订单列表任务化与动作层级收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-11-merchant-orders-list-task-surface.md)
- [ ] [CARD-12 订单与预订运营页跨页一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-12-merchant-ops-cross-page-consistency.md)

## Batch 5：骑手首页实时工作台簇

- [ ] [CARD-13 骑手首页实时区与状态语义重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-13-rider-dashboard-realtime-and-state-semantics.md)
- [ ] [CARD-14 骑手工作台与历史任务回流一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-14-rider-workbench-history-consistency.md)

## Batch 6：运营首页聚合控制台簇

- [ ] [CARD-15 运营首页首屏优先级与待办工作台收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-15-operator-dashboard-priority-and-workbench.md)
- [ ] [CARD-16 运营分析页与首页关系重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-16-operator-analytics-and-dashboard-boundary.md)

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
10. 然后做 CARD-10，把预订页从多任务堆叠页收口成清晰的日视图任务页。
11. 再做 CARD-11，把订单列表从“有动作的列表”收口成真正的任务页。
12. 最后做 CARD-12，统一订单页和预订页在状态、筛选、动作和空态上的系统感。
13. 再做 CARD-13，把骑手首页的实时区、空态和失败语义拉回可信工作台。
14. 然后做 CARD-14，统一骑手首页与历史任务页的入口和回流体验。
15. 接着做 CARD-15，把运营首页首屏收口成真正的待办与决策工作台。
16. 最后做 CARD-16，明确运营首页与分析页的边界和信息层级。

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

## Batch 4 完成标准

- [ ] 商户预订页和订单列表都能一眼看出当前最重要的待处理任务，而不是让用户在一屏里自己找重点。
- [ ] 订单与预订页的动作层级、按钮强调、状态筛选和空态语义明显更统一。
- [ ] 两个页面都具备更稳定的日常高频使用体验，包括筛选、刷新、失败恢复和跨页回流。
- [ ] Batch 4 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。

## Batch 5 完成标准

- [ ] 骑手能分清“真的没单”“加载失败”“订阅未恢复”“历史列表失败”等不同状态，不再被模糊空态误导。
- [ ] 骑手首页和历史任务页读起来像同一套工作台，而不是主入口和工具页各自演化。
- [ ] Batch 5 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。

## Batch 6 完成标准

- [ ] 运营首页首屏能快速表达当前最重要的待办和决策信息，不再首屏过载。
- [ ] dashboard 与 analytics 的职责边界清楚，首页不再和分析页互相复制摘要。
- [ ] Batch 6 的实现与 review 全部使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。