# Weapp 整体升级执行计划

日期：2026-04-05

> 当前执行说明
>
> 本文基于 2026-04-05 的 weapp 整体升级型只读审查结论形成，目标是把“哪些页面最不统一、最不友好、最值得先升级”收口成可执行批次。
>
> 本文不是新的长期标准。涉及设计、交互、性能和后端真值时，仍以当前权威文档为准：
>
> - `.github/standards/weapp/README.md`
> - `.github/standards/weapp/REVIEW_CHECKLIST.md`
> - `.github/standards/weapp/DESIGN_SYSTEM.md`
> - `.github/standards/weapp/INTERACTION_STANDARDS.md`
> - `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
> - `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
> - `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
> - `weapp/docs/miniprogram-prompt-system.md`

## 1. 目标

本计划用于把当前 weapp 从“功能已覆盖较多，但页面质量层次不一、主链路可信度不足、跨页统一性不够”的状态，提升到可持续按统一标准实现与 review 的状态。

本轮目标不是零散修 bug，而是完成三件事：

1. 优先修复最伤害用户信任和主任务完成率的页面簇。
2. 把视觉、交互、性能三部分统一纳入页面实现和 review 热路径。
3. 让高频主链路和高风险流程先达到“可信、清晰、可恢复、统一”的基线，再做细节抛光。

## 2. 本轮关键判断

基于 2026-04-05 的整体审查，当前 weapp 的主要问题不是“有没有页面”，而是以下四类问题在关键页面持续重复：

1. 假控件、假成功、静态结果页等契约闭环问题仍在主链路里出现。
2. 高流量页面仍有首屏 fan-out、过度水合、重入全量重拉等性能预算失控问题。
3. 商户、运营、骑手等聚合页信息块过多，主次任务和视觉节奏不清。
4. 高风险资料流和结算流被做成超长单页，状态说明多、任务分段弱、恢复路径不清。

结论：下一阶段最划算的方式不是平均改造所有页面，而是按页面簇推进，把最影响主链路可信度和整端统一性的批次优先打透。

## 3. 默认执行方法

每个批次都必须同时从以下三个视角推进：

1. 视觉：页面骨架、组件组合、边距与间距、视觉层级、安全区、空态与结果区观感。
2. 交互：主任务、状态、反馈、弱网恢复、重入恢复、返回上下文、危险操作确认。
3. 性能：首屏请求预算、预加载边界、`onLoad` / `onShow` 重拉量、长列表和复杂表单负担。

补充底线：

1. 后端 contract 不是第四种设计意见，而是上述三部分都必须共同遵守的硬边界。
2. 所有批次都必须使用 `.github/standards/weapp/REVIEW_CHECKLIST.md` 做 review。
3. 做整体升级 review 时，应额外使用历史蓝图文档识别“仍在延续的旧问题模式”。

## 3.1 闭环执行建议

本计划可以直接接入 [.github/prompts/general-task-loop.prompt.md](.github/prompts/general-task-loop.prompt.md) 对应的 Delivery Loop Orchestrator 模式执行，但建议按批次推进，而不是一次把全部卡片塞进同一轮自动闭环。

推荐方式：

1. 以一个 batch 作为一次自动闭环的任务集合。
2. 以一张 task card 作为一个 implement -> review -> fix -> review -> doc-sync 单元。
3. 当前 batch 通过 review 后，再进入下一批，避免把视觉、交互、性能和跨页一致性问题混成不可控的大回归面。

## 4. 优先级总表

### P0：必须最先启动

1. 预订支付与结果承接簇
2. 外卖结算与首页性能簇

### P1：紧随其后

1. 商户结算配置簇
2. 商户预订运营簇

### P2：统一性与工作台体验提升

1. 骑手首页实时工作台簇
2. 运营首页聚合控制台簇
3. 外卖搜索结果态簇

## 5. 批次拆分

### Batch 1：预订支付与结果承接簇

目标：先消除假支付、假成功和结果承接不可信。

执行拆分：

- [x] [Batch 1 任务卡索引](weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md)
- [x] [CARD-01 预订确认页任务边界与支付真实化](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-01-reservation-confirm-contract-and-task-flow.md)
- [x] [CARD-02 支付结果页与未知状态承接收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-02-payment-result-state-recovery.md)
- [x] [CARD-03 预订详情与返回重入恢复链路收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-03-reservation-detail-reentry-and-context.md)

页面范围：

1. `weapp/miniprogram/pages/reservation/confirm/index.ts`
2. `weapp/miniprogram/pages/reservation/confirm/index.wxml`
3. `weapp/miniprogram/pages/reservation/detail/index.ts`
4. `weapp/miniprogram/pages/reservation/detail/index.wxml`
5. `weapp/miniprogram/pages/orders/success/index.ts`
6. `weapp/miniprogram/pages/orders/success/index.wxml`
7. `weapp/miniprogram/pages/user_center/reservations/index.ts`
8. `weapp/miniprogram/pages/user_center/reservations/index.wxml`

已知核心问题：

1. 预订确认页存在支付方式展示与真实流程不一致的问题。
2. 支付后的结果承接过于静态，没有回查、未知结果或重入恢复。
3. 预订链中的任务边界不清，用户容易误判当前处于付款、锁桌还是后续点餐步骤。

本批次必须完成：

1. 清除不被真实流程消费的假控件和伪任务分支。
2. 把支付结果、未知结果、取消结果、弱网回跳结果做成真实状态面，而不是静态成功页。
3. 明确预订确认页与结果页的职责边界，让用户知道每一步到底完成了什么。
4. 对返回页面、微信回跳、重新进入和支付重试做状态恢复设计。

验收标准：

1. 用户不会再在预订链路中看到“展示了支付选项但实际上没用”的假交互。
2. 结果页能表达成功、失败、取消、未知等关键状态，而不只是一张成功静态页。
3. 支付后重进、回前台、弱网回跳不会让用户丢失关键上下文。

风险等级：G3

状态回填：Batch 1 任务卡与回填摘要已更新；当前无需新增标准文档或提示文档。

### Batch 2：外卖结算与首页性能簇

目标：把高频消费链路从“可用但重、慢、不够可信”提升到“快速、稳定、可信”。

执行拆分：

- [ ] [CARD-04 外卖首页首屏预算与渐进水合收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-04-takeout-home-budget-and-hydration.md)
- [ ] [CARD-05 外卖确认页 contract、试算与支付承接真实化](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-05-order-confirm-contract-and-result-flow.md)
- [ ] [CARD-06 外卖搜索结果态、建议态与失败恢复收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-06-search-result-state-and-retry-semantics.md)

页面范围：

1. `weapp/miniprogram/pages/takeout/order-confirm/index.ts`
2. `weapp/miniprogram/pages/takeout/order-confirm/index.wxml`
3. `weapp/miniprogram/pages/takeout/index.ts`
4. `weapp/miniprogram/pages/takeout/index.wxml`
5. `weapp/miniprogram/pages/takeout/search/index.ts`
6. `weapp/miniprogram/pages/takeout/search/index.wxml`

已知核心问题：

1. 下单确认页存在按商户线性放大的初始化、试算和下单流程。
2. 外卖首页仍有明显 per-item 水合和逐店详情 fan-out。
3. 搜索页和结果页还存在失败伪装成空态的情况。

本批次必须完成：

1. 给首页和下单确认页建立明确的首屏请求预算与重入刷新边界。
2. 优化商户级串行调用和 per-item fan-out，优先改成批量接口、延迟水合或更受控的增量加载。
3. 清理“支付功能开发中”之类占位承接，把支付未知结果纳入真实状态流。
4. 区分搜索无结果、建议加载失败、列表加载失败和普通空态。

验收标准：

1. 首页首屏和下单确认页的请求放大效应明显下降。
2. 高峰期和弱网下，用户仍能稳定完成下单与支付结果确认。
3. 搜索结果页不再用空态伪装失败态。

风险等级：G2-G3

### Batch 3：商户结算配置簇

目标：把高风险资料流和结算开通流从长单页堆叠改造成清晰可恢复的任务流。

执行拆分：

- [ ] [CARD-07 主体申请页信息架构与分段任务流重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-07-merchant-application-information-architecture.md)
- [ ] [CARD-08 收付通进件页状态承接与阻塞任务流统一](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-08-merchant-applyment-state-and-blocking-flow.md)
- [ ] [CARD-09 结算配置跨页回流与入口一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-09-merchant-settlement-cross-page-consistency.md)

页面范围：

1. `weapp/miniprogram/pages/merchant/settings/application/index.ts`
2. `weapp/miniprogram/pages/merchant/settings/application/index.wxml`
3. `weapp/miniprogram/pages/merchant/settings/applyment/index.ts`
4. `weapp/miniprogram/pages/merchant/settings/applyment/index.wxml`
5. `weapp/miniprogram/pages/merchant/settings/applyment/completed/index.ts`
6. `weapp/miniprogram/pages/merchant/settings/applyment/completed/index.wxml`
7. `weapp/miniprogram/pages/merchant/finance/index.ts`
8. `weapp/miniprogram/pages/merchant/finance/index.wxml`

已知核心问题：

1. 主体申请页承载信息过多，长滚动链导致可读性和恢复性都下降。
2. 进件页同时承担状态卡、阻塞说明、签约、账户跳转、刷新状态等多种任务。
3. 资料上传、OCR、保存、状态刷新混在同一页面状态机里，不利于任务分段。

本批次必须完成：

1. 拆清“资料录入”“结果确认”“阻塞说明”“后续动作”几个任务段。
2. 统一上传、OCR 回填、保存中、回读失败、阻塞状态的页内承接方式。
3. 让关键动作从长页面中变得清楚，不再把所有动作放在同一滚动链里竞争注意力。
4. 让页面在弱网、回退、重新进入后能回到可信状态，而不是回到半完成页面。

验收标准：

1. 用户能一眼分清当前是在填写资料、等待审核、去签约还是去补资料。
2. 页面不再以超长表单堆叠承担所有任务。
3. 高风险资料流具备稳定的失败恢复和重入恢复。

风险等级：G2-G3

### Batch 4：商户预订运营簇

目标：把重运营页面从“功能很多但不好用”提升到“任务清楚、动作顺手、视觉统一”。

执行拆分：

- [ ] [CARD-10 商户预订页日视图、汇总与主任务重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-10-merchant-reservations-day-view-and-task-focus.md)
- [ ] [CARD-11 商户订单列表任务化与动作层级收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-11-merchant-orders-list-task-surface.md)
- [ ] [CARD-12 订单与预订运营页跨页一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-12-merchant-ops-cross-page-consistency.md)

页面范围：

1. `weapp/miniprogram/pages/merchant/reservations/index.ts`
2. `weapp/miniprogram/pages/merchant/reservations/index.wxml`
3. `weapp/miniprogram/pages/merchant/orders/list/index.ts`
4. `weapp/miniprogram/pages/merchant/orders/list/index.wxml`

已知核心问题：

1. 日汇总、备菜、列表、代客建单、状态流转挤在同一页中，主任务不清。
2. 页面动作多但强调弱，存在 outline 小按钮和密集操作区。
3. 日期切换和预订列表读取仍偏重，影响日常高频使用。

本批次必须完成：

1. 给商户预订运营页明确一屏主任务和次任务层级。
2. 清理弱强调小按钮承担主流程的问题，统一底部弹层和动作区模式。
3. 优化日期切换、日汇总与列表加载的请求边界。
4. 把列表页与预订页的状态、筛选、分页和运营动作统一成一个系统。

验收标准：

1. 页面首屏能清楚区分“今日经营概览”和“当前待处理动作”。
2. 用户不再需要在一屏里扫描大量同级模块才能找到当前最该点的动作。
3. 预订运营和订单运营页面的布局、动作样式和空态语义明显更统一。

风险等级：G2

### Batch 5：骑手首页实时工作台簇

目标：把骑手首页从超大聚合页收口成可信的实时工作台。

执行拆分：

- [ ] [CARD-13 骑手首页实时区与状态语义重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-13-rider-dashboard-realtime-and-state-semantics.md)
- [ ] [CARD-14 骑手工作台与历史任务回流一致性收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-14-rider-workbench-history-consistency.md)

页面范围：

1. `weapp/miniprogram/pages/rider/dashboard/index.ts`
2. `weapp/miniprogram/pages/rider/dashboard/index.wxml`
3. `weapp/miniprogram/pages/rider/tasks/index.ts`
4. `weapp/miniprogram/pages/rider/tasks/index.wxml`

已知核心问题：

1. 大厅、任务、概览、实时定位和导航入口堆在同一页中。
2. 子区块失败容易被伪装成“暂未展示”一类模糊空态。
3. 冷启动、重连、回前台的恢复质量会直接影响抢单和履约。

本批次必须完成：

1. 把实时任务区和普通概览区做出更清楚的信息架构边界。
2. 区分真实空态、加载失败、订阅失败、定位失败和暂时无任务。
3. 优化冷启动、重连、前后台切换时的恢复反馈。

验收标准：

1. 用户能分清“真的没单”“暂时加载失败”“订阅未恢复”。
2. 抢单大厅和任务区在弱网与重入场景下仍能维持可信状态。

风险等级：G2

### Batch 6：运营首页聚合控制台簇

目标：把运营首页从首屏过载的聚合页改造成有明确优先级的控制台。

执行拆分：

- [ ] [CARD-15 运营首页首屏优先级与待办工作台收口](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-15-operator-dashboard-priority-and-workbench.md)
- [ ] [CARD-16 运营分析页与首页关系重构](weapp/docs/weapp_overall_upgrade_task_cards_20260405/card-16-operator-analytics-and-dashboard-boundary.md)

页面范围：

1. `weapp/miniprogram/pages/operator/dashboard/index.ts`
2. `weapp/miniprogram/pages/operator/dashboard/index.wxml`
3. `weapp/miniprogram/pages/operator/analytics/index.ts`
4. `weapp/miniprogram/pages/operator/analytics/index.wxml`

已知核心问题：

1. 首屏同时承载太多模块，主任务不聚焦。
2. 待办、异常、排行、矩阵都占据首屏注意力，没有形成层级。
3. 聚合请求过多，弱网下首页显得又重又散。

本批次必须完成：

1. 给运营首页建立首屏优先级，只保留真正的当前待办和高频决策信息。
2. 让二级统计和补充信息后置，避免首屏过载。
3. 对聚合请求做预算控制，减少一次进页全量拉取。

验收标准：

1. 用户在首屏能快速知道“现在最该处理什么”。
2. 首页弱网体验明显改善，不再通过堆模块来制造“信息很全”的假繁忙感。

风险等级：G1-G2

## 6. 推荐执行顺序

1. 先做 Batch 1，止血预订支付和结果可信度问题。
2. 并行启动 Batch 2 的首页预算审查和结算流 contract 审查。
3. Batch 1 完成后推进 Batch 2 的实现收口。
4. 再执行 Batch 3，把商户高风险资料流与结算流统一提升。
5. 随后执行 Batch 4、Batch 5、Batch 6，分别治理商户运营、骑手实时和运营控制台的一致性问题。

## 7. 每批次统一验收要求

每个批次都必须至少完成以下验收：

1. 视觉：布局骨架、组件组合、边距与间距、安全区、动作区主次关系符合当前 design system。
2. 交互：主任务明确，状态完整，反馈单一，弱网与重入恢复清楚。
3. 性能：首屏请求、预加载和重拉量受控，不靠默认全量重拉维持正确性。
4. contract：真实后端字段、状态、分页、鉴权和异步结果语义已核对。
5. review：必须使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`。

## 8. 建议交付形式

建议后续把每个 batch 继续拆成单独任务卡，每张任务卡满足以下要求：

1. 单人或单小组可在一轮迭代内完成。
2. 明确页面范围、主问题、验收标准、验证命令和人工回归清单。
3. 明确是 baseline violation 修复还是 upgrade opportunity 收口。
4. 完成后回填 PR、验证结果和残余风险。

## 9. 当前最值得先做的三组

1. 预订支付与结果承接簇：最直接消除假支付、假成功和主链路不可信。
2. 外卖结算与首页性能簇：最直接改善高频消费链路的稳定性和速度。
3. 商户结算配置簇：最直接提升高风险资料流和结算开通流的可读性与恢复性。