# Weapp Payment And Refund Terminal Flow Fix Plan

日期: 2026-05-23

范围: `weapp/` 小程序顾客外卖购物车、确认单、订单列表、订单详情、支付结果页、退款详情页，以及商户订单详情中的退款发起体验。仅在小程序无法独立收口历史异常状态时，才触及 `locallife/` 后端防线。

风险等级: G2/G3。支付、退款、异步结果、跨页恢复和资金状态展示属于高风险路径；若触及宝付聚合支付/退款编排或数据库状态迁移，按 G3 处理。

## 背景

这份计划承接一次小程序支付/退款流程审查。审查重点不是“接口能不能调通”，而是用户在外卖购物车、订单支付、支付结果、退款提交和退款详情里看到的结果，是否符合真实业务边界与资金终态。

当前后端已经明确不支持合单支付:

- `locallife/logic/combined_payment_service.go:120` 的 `CreateCombinedPaymentOrder` 返回 unsupported。
- `locallife/logic/combined_payment_service.go:138` 的 `QueryCombinedPaymentOrder` 返回 unsupported。
- `locallife/api/payment_capabilities.go:13` 仍暴露 `combined_payment_supported` 与 `split_checkout_required` 能力字段。

当前小程序仍残留合单支付入口和恢复逻辑:

- `weapp/miniprogram/pages/takeout/order-confirm/index.ts:548` 处理 `paymentResult.combinedPayment`。
- `weapp/miniprogram/pages/orders/detail/index.ts:390` 读取 `payment_context.combined_payment_id` 并尝试恢复合单。
- `weapp/miniprogram/pages/orders/list/index.ts:449` 保留 `resumeCombinedPayment`。
- `weapp/miniprogram/pages/orders/list/index.ts:474` 保留 `onBatchPay`。
- `weapp/miniprogram/services/payment-workflow.ts:325` 之后仍保留合单支付 workflow。

当前支付和退款的结果承接也存在用户心智风险:

- `weapp/miniprogram/services/payment-workflow.ts:231` 的单笔支付 workflow 已经会调用 `wx.requestPayment` 后轮询，但轮询失败会返回 `pending_confirmation`。
- `weapp/miniprogram/pages/payment/result/index.ts:74` 和 `:105` 让支付结果页继续承接处理中状态，用户可能停在半终态页面。
- `weapp/miniprogram/pages/user_center/refund-detail/index.ts:101` 起会对非终态退款无限固定间隔轮询。
- `weapp/miniprogram/pages/merchant/orders/detail/index.ts:491` 提交退款后只关闭弹层、刷新详情，并提示后续更新。
- `weapp/miniprogram/pages/merchant/orders/detail/index.wxml:314` 和 `index.ts:501` 使用“微信侧”文案，和当前主业务宝付聚合退款边界不一致。
- `weapp/miniprogram/pages/user_center/payment-detail/index.ts:269` 过滤掉非终态退款，用户在处理中阶段看不到退款记录。

## 用户确认的产品决策

- 当前设计不支持合单支付，合单支付应从小程序支付业务中移除。
- 外卖购物车要直接提示不支持多商户一起支付，只能选择一个商户的订单支付。
- 支付拉起微信小程序支付后，页面保持旋转等待，直到获取到后端支付终态，再跳转支付终态页面。
- 退款提交或进入退款详情后，也应保持旋转等待，直到获取到后端退款终态；若指数退避轮询结束仍无终态，跳转到终态承接页并明确给出“结果待确认/稍后查看”等结果。
- 前端不得把 `wx.requestPayment` 成功回调、提交退款接口成功、轮询中的中间态当成资金终态。

## 边界

本次要做:

- 删除或禁用小程序合单支付入口、恢复逻辑、结果文案和相关 UI。
- 外卖购物车和确认单按“单商户支付”建模，能力接口加载失败时也不得放行多商户支付。
- 单笔支付保持后端支付单状态为唯一真值；支付后等待终态再进入结果页。
- 退款保持后端退款单状态为唯一真值；退款后等待终态，或指数退避结束后进入明确的结果承接页。
- 清理用户可见的 provider/技术文案，尤其是商户侧退款中的“微信侧”。
- 让支付详情能展示处理中退款，不再只展示 terminal refund。

本次不做:

- 不重新启用后端合单支付。
- 不新增合单支付后端接口、兼容层或新订单聚合模型。
- 不在小程序里按 provider 做支付/退款分支，前端只消费后端返回的支付参数和状态。
- 不把宝付、微信、聚合支付等 provider 原文暴露给用户。
- 不改造完整订单中心信息架构；仅收口支付、退款、异步终态相关路径。
- 不处理 `web/`、`merchant_app/`，除非后续发现同一资金状态契约必须跨端同步。

## 目标不变式

- 多商户外卖购物车不能进入一次支付链路；用户必须收敛到一个商户后再支付。
- 小程序支付业务里不存在合单支付入口、合单恢复、合单支付结果页跳转或合单 toast。
- `wx.requestPayment` 只表示微信收银台交互结束，不代表业务支付成功。
- 支付结果页只承接终态或结果待确认态，不长期承担轮询页面职责。
- 退款提交成功只表示退款单已受理，不代表退款成功。
- 等待页和结果页只能展示产品文案，不展示 provider、网关、接口字段或内部诊断。
- 处理中退款在支付详情和退款详情中可见，用户不会因为过滤逻辑误以为退款没有发起。

## 阶段任务清单

### [x] 阶段 0: 基线确认与影响面冻结

目标: 让后续执行者先确认真实入口、状态枚举、验证命令和当前漂移，避免边改边猜。

任务:

- 读取小程序交付基线: `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`。
- 读取宝付主业务边界: `.github/standards/domains/baofu-payment/README.md`。
- 确认后端合单支付仍为 unsupported，不为本次恢复合单。
- 列出小程序合单支付引用点，至少覆盖:
  - `weapp/miniprogram/pages/takeout/order-confirm/index.ts`
  - `weapp/miniprogram/pages/orders/detail/index.ts`
  - `weapp/miniprogram/pages/orders/list/index.ts`
  - `weapp/miniprogram/services/payment-workflow.ts`
  - `weapp/miniprogram/utils/orders-list-view.ts`
  - `weapp/miniprogram/utils/takeout-order-confirm-support.ts`
- 确认支付状态枚举来自 `weapp/miniprogram/api/payment.ts`，退款状态枚举也来自同一契约。

验收:

- 已确认所有待删除或改造入口，没有新增未知支付入口。
- 已记录执行前验证命令是否可用: `cd weapp && npm run compile`、`npm run lint`、必要时 `npm run quality:check`。

### [x] 阶段 1: 移除合单支付业务入口

目标: 小程序支付业务不再尝试创建、恢复、展示或跳转合单支付。

任务:

- 在 `weapp/miniprogram/pages/takeout/order-confirm/index.ts` 删除 `paymentResult.combinedPayment` 分支，只保留单笔/单商户订单支付路径。
- 在 `weapp/miniprogram/pages/orders/detail/index.ts` 删除基于 `payment_context.combined_payment_id` 的合单恢复路径；历史订单若仍携带该字段，应改为产品化提示或单笔支付可恢复路径。
- 在 `weapp/miniprogram/pages/orders/list/index.ts` 删除 `resumeCombinedPayment`、`onBatchPay` 和批量合单支付按钮状态。
- 删除或收口 `weapp/miniprogram/services/payment-workflow.ts` 中只服务合单支付的 workflow、类型和等待逻辑；保留单笔支付 workflow。
- 清理 `weapp/miniprogram/utils/orders-list-view.ts` 与 `weapp/miniprogram/utils/takeout-order-confirm-support.ts` 中的合单支付成功、失败、toast 文案。
- 若有 WXML 仍展示“批量支付”“合并支付”“合单支付”，改为单商户支付相关入口或移除。

验收:

- `rg -n "combinedPayment|combined_payment|合单|合并支付|BatchPay|onBatchPay|resumeCombinedPayment" weapp/miniprogram` 不再命中小程序支付业务运行路径。
- 订单列表、订单详情、确认单无法触发合单支付接口。
- 历史 `combined_payment_id` 不会导致页面调用 unsupported 后端接口。

### [x] 阶段 2: 外卖购物车单商户支付约束

目标: 外卖购物车从用户选择阶段就收住多商户支付，而不是等后端或能力接口兜底。

任务:

- 在 `weapp/miniprogram/pages/takeout/cart/index.ts` 以商户维度计算已选购物车项，若选中跨多个商户，直接展示产品文案: “暂不支持多商户一起支付，请选择一个商户的商品结算”。
- 多商户选中时禁用结算主按钮或在点击时阻断，保持同一个主反馈通道，避免 toast、弹窗、页内提示重复出现。
- 能力接口加载失败时默认按“不支持合单/必须拆单”处理，不得因为 `splitCheckoutRequired` 未加载而放行多购物车支付。
- 在 `weapp/miniprogram/pages/takeout/order-confirm/index.ts` 做第二道防线: 如果路由参数或恢复状态带入多商户/多购物车，阻断提交并引导返回购物车重新选择。
- 文案只表达用户任务，不出现 `split_checkout_required`、`combined_payment_supported`、合单能力等接口术语。

验收:

- 单商户多商品可继续进入确认单并支付。
- 跨商户选择不能进入支付链路。
- 支付能力接口失败时，跨商户仍不能进入支付链路。
- 用户能理解下一步是“只选一个商户结算”，不是“系统异常”。

### [x] 阶段 3: 支付后等待终态再跳结果页

目标: `wx.requestPayment` 返回后进入等待态，直到后端支付单为终态，再跳转结果页；轮询耗尽时进入“结果待确认”承接，不停留在无限处理中页面。

任务:

- 明确支付 workflow owner，优先收口在 `weapp/miniprogram/services/payment-workflow.ts`，Page 只负责触发、渲染等待态和跳转。
- 调整 `completePaymentWorkflow`:
  - 拉起 `wx.requestPayment` 后立即进入等待态。
  - 使用后端 `pollPaymentStatus` 查询支付单。
  - `paid`、`failed`、`closed`、`refunded` 等终态返回后才跳转结果页。
  - 用户取消微信收银台时按取消/未支付结果承接，不伪装成失败或成功。
  - 轮询异常或耗尽时返回 `pending_confirmation`，但该状态只用于结果待确认承接，不继续无限轮询。
- 调整 `weapp/miniprogram/pages/payment/result/index.ts`:
  - 移除结果页中的长期轮询职责。
  - 只展示支付成功、支付失败/关闭、已退款、结果待确认等可解释状态。
  - 结果待确认页提供“刷新结果/查看订单”一类恢复入口，不自动无限等待。
- 调整 `weapp/miniprogram/pages/payment/result/index.wxml`，等待态只用于短暂进入或由独立等待页承接；终态页必须有清晰主操作。
- 防重入: 支付中、等待后端终态中、跳转结果页前，主支付按钮不可重复触发。

验收:

- 支付成功回调后不会立即展示“支付成功”，必须以后端支付单终态为准。
- 后端状态延迟时，用户看到旋转等待，而不是可操作的半终态结果页。
- 轮询耗尽后不会无限转圈，会进入“结果待确认”结果页，并有查看订单/刷新路径。
- 重复点击支付不会创建并发支付流程。

### [x] 阶段 4: 退款提交后等待终态或指数退避结束

目标: 退款流程与支付一样以后端退款状态为真值；处理中可见，终态明确，轮询有上限。

任务:

- 新增或收口退款等待 workflow，建议放在 `weapp/miniprogram/services/` 下的独立 refund workflow 文件，避免把退款轮询散落在商户详情和用户退款详情页。
- 指数退避策略:
  - 起始间隔建议 1s 或 2s。
  - 每次失败或仍为非终态后递增，设置最大间隔，例如 8s。
  - 设置最大尝试次数或最大等待时长，例如 30s 到 60s。
  - token/cancel 机制保证页面卸载、重新进入、重复提交时不会并发轮询同一退款。
- 在 `weapp/miniprogram/pages/merchant/orders/detail/index.ts` 中，`createRefund` 返回退款单后进入等待态，直到 `success`、`failed`、`closed` 等终态，或退避轮询结束。
- 轮询结束仍非终态时，跳转退款结果/详情承接页，文案表达“退款已提交，结果待确认，请稍后查看”，不表达“退款成功”。
- 在 `weapp/miniprogram/pages/user_center/refund-detail/index.ts` 中替换当前无限固定间隔轮询；进入非终态退款详情时使用同一指数退避策略。
- 确认退款详情页能展示处理中、成功、失败、关闭、待确认，不因非终态隐藏核心信息。

验收:

- 商户提交退款后不会只 toast 或静默刷新，用户看到明确等待态。
- 退款成功/失败以后端状态为准。
- 长时间未取到终态时不会无限等待，会进入结果待确认承接。
- 页面卸载或重新进入不会留下旧轮询继续写 state。

### [x] 阶段 5: 支付详情和退款可见性修复

目标: 用户能在支付详情中看到已提交但尚未终态的退款，不误判为“没有退款”。

任务:

- 修改 `weapp/miniprogram/pages/user_center/payment-detail/index.ts:269` 附近逻辑，不再用 `isRefundStatusTerminal(refund.status)` 过滤退款。
- 对退款列表按状态映射展示:
  - `pending`: 退款申请中。
  - `processing`: 退款处理中。
  - `success`: 退款成功。
  - `failed`: 退款失败。
  - `closed`: 退款已关闭。
- 非终态退款点击后进入退款详情页，由阶段 4 的等待/待确认策略承接。
- 支付详情中的退款区块不展示 provider、网关或内部错误原文。

验收:

- 刚提交的退款能在支付详情中看到。
- 处理中退款和终态退款都能进入退款详情。
- 支付详情不会因为只有非终态退款而隐藏退款列表。

### [x] 阶段 6: UI 文案和用户心智清理

目标: 页面语言符合用户对支付/退款的理解，不暴露 provider 细节或技术边界。

任务:

- 将 `weapp/miniprogram/pages/merchant/orders/detail/index.wxml:314` 的“微信侧退款进度”改为业务文案，例如“提交后会继续同步退款处理进度”。
- 将 `weapp/miniprogram/pages/merchant/orders/detail/index.ts:501` 的“微信侧处理进度”改为同类业务文案。
- 全局搜索小程序用户可见文案中的 `微信侧`、`宝付`、`Baofu`、`provider`、`网关`、`combined_payment`、`split_checkout_required`，确认不会出现在顾客/商户普通用户界面。
- 支付等待、退款等待和待确认结果文案保持单一反馈通道:
  - 等待中: “正在确认支付结果”/“正在确认退款结果”。
  - 待确认: “结果还在同步中，请稍后查看订单/退款详情”。
  - 终态: 直接展示成功、失败、关闭等结果和下一步。
- 非顾客侧商户页面使用克制的 TDesign 反馈，不引入顾客侧装饰语言。

验收:

- 用户界面没有 provider 术语和接口字段术语。
- 同一支付/退款结果不会同时用 toast、弹窗、notice 和结果页重复表达。
- 等待页和结果页的主操作清晰，不让用户误以为可以重复发起资金动作。

### [x] 阶段 7: 可选后端防线与历史状态处理

目标: 只在小程序无法独立规避历史合单上下文时，补一层后端保护；不恢复合单支付能力。

触发条件:

- 阶段 1 后仍存在历史订单只能通过 `combined_payment_id` 找到待支付上下文，且没有单笔支付恢复路径。
- 小程序删除合单恢复后，会让历史未支付订单无法继续支付或关闭。

任务:

- 先确认真实后端订单/支付上下文是否已有单笔 payment order 可恢复。
- 若需要后端协助，只允许增加“关闭/废弃历史 unsupported 合单上下文并创建单笔支付”的显式路径，不能让合单支付重新进入生产流程。
- 后端返回给小程序的错误必须是业务可映射错误，不暴露内部 unsupported message。
- 若触及后端支付域，按 G3 执行，并读取:
  - `locallife/AGENTS.md`
  - `.github/prompts/backend-payment-domain.prompt.md`
  - `.github/standards/domains/baofu-payment/README.md`
  - `.github/standards/domains/wechat-payment/README.md`

验收:

- 后端仍明确不支持合单支付。
- 历史合单上下文不会造成小程序调用 unsupported 接口或卡死。
- 新增后端路径有针对性测试和支付域验证记录。

### [x] 阶段 8: 验证与交付收口

目标: 用最小但足够的验证证明支付、退款、单商户边界和文案清理已经闭环。

自动化验证:

- 从 `weapp/` 目录运行 `npm run compile`。
- 从 `weapp/` 目录运行 `npm run lint`。
- 若改动跨多个页面、workflow 或共享 service，运行 `npm run quality:check`。
- 若触及后端，进入 `locallife/` 并运行对应后端测试；支付域至少包含相关 service/API focused tests，必要时运行 `make check-baofu-contract`。

手工场景:

- 外卖购物车单商户多商品: 可进入确认单并支付。
- 外卖购物车跨商户选择: 被阻断，并提示只能选择一个商户结算。
- 支付能力接口失败: 跨商户仍被阻断。
- 微信收银台支付成功但后端状态延迟: 页面保持等待，终态返回后跳结果页。
- 微信收银台取消: 不展示支付成功，可回到订单继续支付或查看订单。
- 后端支付轮询耗尽: 进入结果待确认页，有查看订单/刷新入口。
- 商户提交退款: 进入退款结果等待，终态后展示成功/失败。
- 退款长时间处理中: 指数退避结束后进入待确认承接页。
- 用户支付详情: 能看到处理中退款。
- 退款详情前后台切换/返回重入: 不出现并发轮询和旧状态覆盖。

交付记录:

- 记录变更文件和删除的合单入口。
- 记录执行过的自动化命令和结果。
- 记录未覆盖的手工场景及剩余风险。
- 若保留任何历史合单字段或文案残留，必须说明为什么不可删除、运行时是否不可达、后续由谁清理。

## 执行记录

日期: 2026-05-23

已完成:

- 小程序支付运行路径已移除合单创建、合单恢复、合单 workflow 和合单结果文案。
- 外卖购物车默认只选中第一家可用商户；跨商户选择会阻断支付，并提示只选择一家商户结算。
- 确认单增加单商户防线，不再按支付能力接口加载结果决定是否放行多商户支付。
- 支付 workflow 在 `wx.requestPayment` 后等待后端支付终态；结果页不再无限轮询，只承接终态或“结果待确认”。
- 新增 `weapp/miniprogram/services/refund-workflow.ts`，退款等待使用指数退避和最大尝试次数。
- 商户发起退款后进入退款终态等待或待确认承接；等待失败不再误报“发起退款失败”。
- 用户退款详情复用同一退款等待策略，卸载/重试通过 token 防止旧轮询写 state。
- 用户支付详情不再过滤非终态退款，处理中退款可见。
- 商户退款文案已移除“微信侧”。
- 新增并接入 `npm run check:payment-refund-terminal-flow`；更新 `check:takeout-cart-split-checkout-ux` 为单商户支付心智。

验证:

- `cd weapp && npm run check:payment-refund-terminal-flow`: 通过。
- `cd weapp && npm run check:takeout-cart-split-checkout-ux`: 通过。
- `cd weapp && npm run compile`: 通过。
- `cd weapp && npm run lint`: 通过。
- `cd weapp && npm run quality:check`: 通过，包含 `gate:payment-workflow-boundary`。

未执行:

- 未做微信开发者工具真机/模拟器手工支付和退款链路验证。
- 未改动后端，因此未运行后端支付域测试。

剩余风险:

- 真实微信收银台返回、后端回写延迟、退款通道异步回调仍需在联调环境按阶段 8 手工场景验证。
- 后端接口类型中仍保留历史 `combined_payment_id` 字段和 `/v1/payments/combined` typed API；本次已从小程序运行时支付业务移除调用，没有恢复后端合单支付。
