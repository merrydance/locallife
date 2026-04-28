# 小程序微信支付状态对齐审查与修复计划

日期：2026-04-27

## 1. 背景

后端已经完成一轮微信支付解耦重构，并把支付、退款、提现、分账回退等资金事件逐步收口到 command、fact、fact application 和业务域终态推进链路。小程序端还存在若干旧式交互：页面直接把 `wx.requestPayment` 成功、创建退款成功、创建提现成功，理解成业务终态成功。

本文件用于沉淀本轮小程序端审查发现，并制定第一稿修复计划。目标不是把每个页面各自补一个判断，而是把支付结果确认、状态解释、跳转与刷新策略收敛到高内聚、低耦合的前端工作流中。

本计划按 `G2/G3` 资金链路风险处理。

补充判断：骑手押金不应继续被当作“小程序支付体验优化”的子问题处理。它同时涉及充值入账、可退款凭证、提现预留、微信退款回调、余额刷新和运营状态恢复，风险级别应按 `G3` 资金账务链路单独收敛。

## 2. 总目标

1. 小程序端所有微信支付相关页面以后端真值为准，不再把本地微信支付拉起成功当成业务成功终态。
2. 用户能清楚知道当前处境：支付未发起、已取消、提交后确认中、已成功、失败可重试、退款处理中、提现处理中、状态同步失败。
3. 交互动作明确：该等待时展示确认中和自动刷新，该跳转时跳到详情或列表，该刷新时提供清晰按钮，该失败时给出中文原因和下一步。
4. 页面内不再散写支付状态推断。创建、拉起、轮询、恢复、结果解释统一进入小程序服务层和视图模型层。
5. 修复会员充值、退款详情字段漂移、危险动作文案等已经发现的契约与交互问题。
6. 将骑手押金充值、提现、退款结果作为独立账务闭环审计，确保小程序显示的总押金、提现处理中、可用押金和流水都来自同一后端真值。

## 3. 设计原则

### 3.1 终态真值原则

- `wx.requestPayment` 成功只表示用户完成了微信支付交互，不表示本地支付单已经 `paid`。
- `create payment` 返回只表示本地支付单和微信预支付已创建，不表示业务订单已支付。
- `create refund` 返回只表示退款单已创建或微信已受理，不表示退款已到账。
- `withdraw` 返回 `202` 或 `processing` 只表示提现/退款请求已受理，不表示钱已经到账。
- 业务成功页只能由后端明确终态驱动：支付单 `paid`，合单有效状态成功，预订/订单业务状态已经同步为成功类状态，或后端返回已完成对账的提现成功。

### 3.2 状态分层原则

小程序端需要区分三类状态，不能混用。

1. 命令状态：支付单创建、退款创建、提现提交是否被受理。
2. 支付事实状态：微信支付/退款/提现远端是否成功、失败、处理中。
3. 业务状态：订单、预订、追偿、押金、会员资产是否已经被业务域确认并展示。

页面只能展示业务可理解的状态，不能把命令状态包装成业务终态。

### 3.3 终态展示原则

真实终态不一定都需要独立页面，但必须都有一个独立的展示模型。支付、退款、提现这类资金动作至少需要统一的结果组件或结果页承接这些状态：

- 成功终态：后端已经确认业务域完成，例如订单已支付、押金已入账、退款已成功、提现已对账完成。
- 失败终态：后端确认失败、关闭、撤销、退回，且用户知道是否可以重试。
- 中间态：后端已受理或微信交互已提交，但业务终态尚未确认。
- 未发起态：创建支付单、退款单或提现申请失败，不能展示为确认中。

页面选择规则：

- 跨页面离开的动作，例如订单支付、堂食支付、合单支付，应优先进入通用结果页。
- 留在同一资产页的动作，例如骑手押金充值、提现，可以用页面内独立状态组件承接，但这个组件必须有终态、处理中、失败、刷新和重试动作。
- Toast 只能作为瞬时反馈，不能承载资金动作终态。
- 弹窗只适合确认危险动作或短暂阻塞，不适合长期展示退款、提现、支付确认中状态。

### 3.4 轮询和等待原则

提交资金动作后，页面应该主动查询后端终态，但轮询不能无限旋转。

推荐策略：

1. 提交命令成功后立即查询一次后端状态。
2. 若仍是处理中，用短轮询承接首个同步窗口，例如 1.5 秒、3 秒、6 秒。
3. 短轮询未拿到终态时停止旋转，展示“确认中”状态和“刷新状态”按钮。
4. 用户重新进入页面、下拉刷新、点击刷新时再次查询后端。
5. 对提现和退款，前端查询对象不能只看余额接口；应能查询本次 refund/withdraw 的明确处理状态，至少展示“已受理、处理中、成功、失败/退回”。

等待期间的视觉组件：

- 短等待可使用 TDesign `Loading` 或 `Toast` 的 loading 态，但文案必须是“正在确认支付结果”或“正在同步提现结果”，不是“支付成功”。
- 超过短窗口后应切换为页面内状态块或结果页状态，不再持续 loading。
- 倒计时只适合“稍后自动刷新一次”或“即将返回详情”，不能暗示倒计时结束就一定成功。
- 资金动作的确认中状态必须提供手动刷新入口。

### 3.5 高内聚低耦合原则

支付状态编排集中到服务层，页面只做三件事：触发动作、渲染视图、执行跳转。

建议新增或收敛这些模块：

- [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)：继续保留底层 API、支付状态枚举、`invokeWechatPay`、查询和轮询函数。
- `weapp/miniprogram/services/payment-workflow.ts`：新增统一支付工作流服务，负责 create、requestPayment、poll、query、结果归类。
- `weapp/miniprogram/utils/payment-result-view.ts`：新增统一中文文案和按钮动作视图模型。
- `weapp/miniprogram/utils/payment-navigation.ts`：可选，用于集中成功、取消、确认中、失败的跳转目标。
- 页面只消费 `PaymentWorkflowResult`，不直接判断 `pay_params` 和 `wx.requestPayment` 的成功含义。

## 4. 本轮审查发现

### 4.1 外卖下单确认页存在假成功风险

涉及文件：

- [weapp/miniprogram/pages/takeout/order-confirm/index.ts](weapp/miniprogram/pages/takeout/order-confirm/index.ts)
- [weapp/miniprogram/utils/takeout-order-confirm-support.ts](weapp/miniprogram/utils/takeout-order-confirm-support.ts)

当前问题：

- 单笔支付在 `invokeWechatPay(paymentResult.pay_params)` 成功后直接进入支付成功页。
- 合单支付在 `invokeWechatPay(combinedPayment.pay_params)` 成功后直接进入合并支付成功页。
- 成功页文案是业务终态成功，但没有等待后端支付单或合单状态确认。

风险：

- 用户看见支付成功，但订单详情或商户端仍可能是待支付或同步中。
- 合单中部分子单、远端延迟、callback 延迟时，前端会提前宣布成功。

### 4.2 堂食结账和堂食成功页存在假成功风险

涉及文件：

- [weapp/miniprogram/pages/dine-in/checkout/checkout.ts](weapp/miniprogram/pages/dine-in/checkout/checkout.ts)
- [weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts](weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts)
- [weapp/miniprogram/pages/dine-in/payment-success/payment-success.wxml](weapp/miniprogram/pages/dine-in/payment-success/payment-success.wxml)

当前问题：

- 堂食支付拉起微信后直接跳转成功页。
- 成功页不回查后端状态，直接显示“支付成功”。
- 成功页还自动倒计时跳订单详情，用户无法判断这是已确认成功还是正在同步。

风险：

- 堂食订单处于支付同步空窗时，用户被推进成功体验，但商户侧可能还未收到可履约状态。

### 4.3 支付详情页继续支付绕过统一终态确认

涉及文件：

- [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts)
- [weapp/miniprogram/pages/user_center/payment-detail/index.wxml](weapp/miniprogram/pages/user_center/payment-detail/index.wxml)

当前问题：

- 非押金支付继续支付时重新创建支付单，拉起微信后直接跳支付成功页。
- 没有轮询 `GET /v1/payments/:id`，也没有回查订单或预订业务状态。
- “关闭支付详情”按钮实际关闭支付单，文案和动作不匹配。

风险：

- 补救入口反而制造新的状态漂移。
- 用户可能以为只是关闭页面，实际关闭了待支付单。

### 4.4 共享 `processPayment` 错误归类不准确

涉及文件：

- [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
- [weapp/miniprogram/pages/reservation/confirm/index.ts](weapp/miniprogram/pages/reservation/confirm/index.ts)
- [weapp/miniprogram/pages/reservation/detail/index.ts](weapp/miniprogram/pages/reservation/detail/index.ts)
- [weapp/miniprogram/pages/user_center/reservations/index.ts](weapp/miniprogram/pages/user_center/reservations/index.ts)

当前问题：

- `processPayment` 在创建支付单失败时返回 `status: 'unknown'`。
- `unknown` 会被预订结果页解释为“支付结果确认中”。

风险：

- 创建支付单失败并没有真实支付待确认。用户会等待一个不存在的支付结果。

### 4.5 会员充值入口与后端契约漂移

涉及文件：

- [locallife/api/membership.go](locallife/api/membership.go)
- [weapp/miniprogram/pages/user_center/membership/index.ts](weapp/miniprogram/pages/user_center/membership/index.ts)
- [weapp/miniprogram/pages/user_center/membership/index.wxml](weapp/miniprogram/pages/user_center/membership/index.wxml)
- [weapp/miniprogram/api/personal.ts](weapp/miniprogram/api/personal.ts)

当前问题：

- 后端会员线上充值已暂停，固定返回 `503`。
- 小程序仍显示“立即充值”，仍让用户选择充值规则并进入发起支付流程。

风险：

- 用户被引导进入不可完成流程，最后只得到错误。
- 页面没有提前说明“请联系商户线下充值后入账”。

### 4.6 退款详情页字段漂移与原始状态泄露

涉及文件：

- [locallife/api/payment_order.go](locallife/api/payment_order.go)
- [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
- [weapp/miniprogram/pages/user_center/refund-detail/index.ts](weapp/miniprogram/pages/user_center/refund-detail/index.ts)
- [weapp/miniprogram/pages/user_center/refund-detail/index.wxml](weapp/miniprogram/pages/user_center/refund-detail/index.wxml)

当前问题：

- 后端字段是 `refund_reason`、`out_refund_no`、`refunded_at`。
- 页面读取 `refund.reason`、`refund.processed_at`、`refund.refund_transaction_id`。
- 分账回退直接显示 `item.status`，未映射成中文状态。

风险：

- 退款原因、处理时间、退款单号可能长期空白。
- 用户看见英文或内部状态，不能理解下一步。

### 4.7 钱包账单状态兜底过度乐观

涉及文件：

- [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts)
- [weapp/miniprogram/utils/payment-ledger-view.ts](weapp/miniprogram/utils/payment-ledger-view.ts)

当前问题：

- 支付账单未知状态默认显示“已支付”。
- 退款账单未知状态默认显示“退款成功”。

风险：

- 后端新增状态或同步状态时，小程序会错误展示成功。

### 4.8 追偿支付整体方向正确，但结果承接还可更清晰

涉及文件：

- [weapp/miniprogram/pages/rider/claims/detail/index.ts](weapp/miniprogram/pages/rider/claims/detail/index.ts)
- [weapp/miniprogram/pages/merchant/claims/detail/index.ts](weapp/miniprogram/pages/merchant/claims/detail/index.ts)

当前状态：

- 追偿支付已在微信支付后轮询支付单状态。
- 轮询超时后会继续刷新详情，不直接宣布成功。

待优化：

- 轮询超时时应明确提示“追偿支付已提交，系统正在确认到账结果”，并提供“刷新追偿状态”。
- 不应只静默写日志后继续刷新。

### 4.9 骑手押金充值恢复有样板价值，但提现闭环不足

涉及文件：

- [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts)
- [weapp/miniprogram/services/rider-deposit-payment.ts](weapp/miniprogram/services/rider-deposit-payment.ts)
- [weapp/miniprogram/services/rider-deposit-finance.ts](weapp/miniprogram/services/rider-deposit-finance.ts)
- [locallife/api/rider.go](locallife/api/rider.go)

当前可复用经验：

- 已有待确认充值上下文、支付单回查、支付轮询、提现处理中金额、自动刷新。
- 后端押金充值按用户、业务类型、金额复用未过期 pending 支付单。

待优化：

- “继续支付”实现上仍通过金额再次调用充值接口，依赖后端复用 pending 单。可在文案和代码中显式称为“恢复待支付单”。
- 未来统一工作流落地后，应让押金服务复用通用支付结果类型，保留押金域自己的余额刷新逻辑。
- 提现链路不能只靠余额延时刷新，需要在 4.11 和阶段 8 中按资金账务闭环单独处理。

### 4.10 预订支付结果页是当前正向样板

涉及文件：

- [weapp/miniprogram/pages/orders/success/index.ts](weapp/miniprogram/pages/orders/success/index.ts)
- [weapp/miniprogram/utils/reservation-payment-result-view.ts](weapp/miniprogram/utils/reservation-payment-result-view.ts)

当前状态：

- 预订结果页会回查预订详情。
- 页面根据预订业务状态重新推导成功、取消、失败、确认中。

应复用的经验：

- 结果页要能展示 unknown，并提供重新查询。
- 成功页标题不能只依赖前一页传参。
- 后续状态以详情页为准。

### 4.11 骑手押金存在前后端账务闭环风险

涉及文件：

- [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts)
- [weapp/miniprogram/services/rider-deposit-payment.ts](weapp/miniprogram/services/rider-deposit-payment.ts)
- [weapp/miniprogram/services/rider-deposit-finance.ts](weapp/miniprogram/services/rider-deposit-finance.ts)
- [locallife/api/rider.go](locallife/api/rider.go)
- [locallife/logic/rider_deposit_refund_service.go](locallife/logic/rider_deposit_refund_service.go)
- [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go)
- [locallife/db/sqlc/tx_rider_refund.go](locallife/db/sqlc/tx_rider_refund.go)

当前问题：

- 小程序提现提交后只刷新余额和流水，并用两次延时刷新兜底；没有围绕本次 refund order 或 withdrawal result 的终态承接。
- 微信已经通知用户提现成功时，小程序仍可能只拿到旧余额或旧流水，表现为持续 loading、报错、可用余额不变。
- 押金页的可用余额完全信任 `/v1/rider/deposit` 返回值；如果后端 `riders.deposit_amount`、`riders.frozen_deposit`、`rider_deposit_credits`、`refund_orders` 之间漂移，前端无法自证。
- 后端提现前的第一层可用余额判断仍使用 `rider.DepositAmount - rider.FrozenDeposit`，没有先把 pending rider deposit refund 作为独立扣减口径纳入判断；事务层虽会冻结，但入口判断和余额展示口径不完全一致。
- 押金充值入账依赖 `payment_orders.processed_at`、`rider_deposits(type=deposit,payment_order_id)`、`rider_deposit_credits(payment_order_id)` 多个幂等点。若历史数据已有缺失或旧路径残留，可能出现主余额、流水、可退款 credit 不一致。
- “充值 100 元后后端记成 200 元，但实际只能提现 100 元”这类现象说明主余额和可退款 credit 已经不是同一真值，不能靠小程序刷新修复。

风险：

- 用户资金认知被破坏：微信显示到账或退款成功，但小程序余额不变或报错。
- 后端账务可能出现主余额虚高、可退款凭证不足、提现入口误判、流水与余额不一致。
- 该问题属于 `G3` 资金账务问题，应先后端对账和幂等闭环，再做 UI 收敛。

建议结论：

- 骑手押金需要单独的“押金账务真值与提现终态”修复任务，不应只归入通用支付结果页改造。
- 后端应暴露或补齐本次提现/退款的状态查询能力，让小程序能根据 refund order/withdraw action 查询终态，而不是只看余额是否变化。
- 小程序押金页应保留页内组件承接，不一定跳独立结果页，但必须有独立的押金动作状态组件，展示充值待确认、提现处理中、提现成功、提现失败退回、同步失败和手动刷新。

## 5. 目标架构

### 5.1 支付工作流服务

新增 `weapp/miniprogram/services/payment-workflow.ts`，作为小程序支付入口的统一编排层。

建议类型：

```ts
export type PaymentWorkflowKind =
  | 'order'
  | 'combined_order'
  | 'reservation'
  | 'reservation_addon'
  | 'rider_deposit'
  | 'claim_recovery'

export type PaymentWorkflowStatus =
  | 'paid'
  | 'failed'
  | 'cancelled'
  | 'pending_confirmation'
  | 'create_failed'
  | 'pay_params_missing'
  | 'closed'

export interface PaymentWorkflowResult {
  kind: PaymentWorkflowKind
  status: PaymentWorkflowStatus
  paymentOrderId?: number
  combinedPaymentId?: number
  businessId?: number
  amountFen?: number
  outTradeNo?: string
  message: string
  nextAction: 'success_page' | 'detail_page' | 'list_page' | 'stay_and_retry' | 'refresh_status'
  shouldRefresh: boolean
}
```

服务职责：

- 创建或恢复支付单。
- 拉起微信支付。
- 区分用户取消、拉起失败、创建失败、支付后待确认、明确成功、明确失败。
- 对普通支付轮询 `GET /v1/payments/:id`。
- 对合单支付轮询或查询 `GET /v1/payments/combined/:id/query`。
- 返回统一 `PaymentWorkflowResult`，不直接操作页面状态。

不做的事情：

- 不渲染 Toast、Modal、页面字段。
- 不直接跳转页面。
- 不关心具体页面按钮布局。

### 5.2 结果视图模型

新增 `weapp/miniprogram/utils/payment-result-view.ts`。

职责：

- 把 `PaymentWorkflowResult` 转成中文标题、说明、主按钮、次按钮、标签主题。
- 所有文案明确告诉用户下一步。
- 未知状态不展示为失败或成功，只展示“正在确认”。

建议文案矩阵：

| 状态 | 标题 | 说明 | 主动作 |
| --- | --- | --- | --- |
| `paid` | 支付已确认 | 系统已确认支付成功，订单状态会继续更新 | 查看详情 |
| `pending_confirmation` | 支付结果确认中 | 支付已提交，系统正在同步微信支付结果，请稍后刷新 | 刷新状态 |
| `cancelled` | 支付已取消 | 本次支付未完成，订单已保留，可稍后继续支付 | 查看订单 |
| `create_failed` | 支付未发起 | 支付单创建失败，未产生待确认支付，请稍后重试 | 返回重试 |
| `pay_params_missing` | 暂不能支付 | 支付参数暂未准备好，请回到详情页重新发起 | 查看详情 |
| `failed` | 支付未完成 | 系统未确认支付成功，请重新发起或查看详情 | 查看详情 |
| `closed` | 支付已关闭 | 当前支付单已关闭，如仍需支付请重新发起 | 查看详情 |

### 5.3 通用结果页

现有 [weapp/miniprogram/pages/orders/success/index.ts](weapp/miniprogram/pages/orders/success/index.ts) 已承担订单和预订支付结果，但普通订单分支仍是纯成功页。建议二选一：

方案 A：扩展现有 `orders/success` 为通用支付结果页。

- 优点：少增页面，兼容现有跳转。
- 缺点：文件语义“success”不再准确。

方案 B：新增 `pages/payment/result/index`。

- 优点：语义清楚，可承载支付成功、取消、确认中、失败。
- 缺点：需要调整 app.json 和现有跳转。

建议采用方案 B。支付结果页不应叫 success。旧 `Navigation.toPaymentSuccess` 可保留兼容，但内部逐步改为 `Navigation.toPaymentResult`。

结果页职责：

- 根据 `paymentOrderId`、`combinedPaymentId`、`businessType`、`businessId` 回查当前状态。
- 首屏如传入 `pending_confirmation`，展示“正在确认支付结果”，并自动刷新一次。
- 用户点击“刷新状态”时重新查询。
- 明确成功后再展示成功标题。
- 明确失败、关闭、取消时提供回详情或重新支付入口。

### 5.4 页面接入方式

页面侧统一写法：

```ts
const result = await PaymentWorkflow.payOrder(orderId)
const action = buildPaymentResultAction(result)
await applyPaymentAction(action)
```

页面不得再出现这些模式：

```ts
await invokeWechatPay(payment.pay_params)
Navigation.toPaymentSuccess(...)
```

例外：底层 `payment-workflow.ts` 和已证明需要自定义域编排的押金服务可直接调用 `invokeWechatPay`，但必须返回统一结果。

## 6. 交互规范

### 6.1 创建支付单失败

场景：接口错误、配置缺失、会员充值 503、订单状态不允许支付。

页面行为：

- 不进入确认中页面。
- 不进入成功页。
- 保留当前页面或跳订单详情。
- 中文提示必须说明没有发起成功。

建议文案：

- `支付未发起，请稍后重试。`
- `当前订单暂不能支付，请返回订单详情查看最新状态。`
- `会员线上充值已暂停，请联系商户线下充值后入账。`

### 6.2 用户取消微信支付

页面行为：

- 不报“支付失败”。
- 告诉用户订单已保留，下一步去详情或列表继续支付。

建议文案：

- `已取消支付，订单已保留，可在订单详情继续支付。`
- `已取消合并支付，可在订单列表继续完成合单支付。`

### 6.3 微信支付交互成功，后端未确认

页面行为：

- 展示 loading：`正在确认支付结果...`
- 轮询支付单或合单状态。
- 超时后进入确认中状态，不进入成功页。
- 提供“刷新状态”和“查看详情”。

建议文案：

- `支付已提交，系统正在同步微信支付结果。若暂未更新，请稍后刷新或查看订单详情。`

### 6.4 后端明确支付成功

页面行为：

- 可以进入支付成功结果页。
- 成功页仍建议提供“查看订单/预订/追偿详情”，不要只做营销导流。

建议文案：

- `支付已确认，商家正在处理中。`

### 6.5 后端明确支付失败或关闭

页面行为：

- 不留在“确认中”。
- 给出重新支付或查看详情。

建议文案：

- `支付未完成，请返回订单详情重新发起。`
- `支付单已关闭，如仍需支付请重新发起。`

### 6.6 退款处理中

页面行为：

- 显示退款申请、处理中、成功、失败、关闭的中文状态。
- 分账回退也显示中文状态，并展示失败原因。
- 不展示原始 `pending`、`processing`、`closed`。

建议文案：

- `退款申请已提交，微信侧正在处理，到账结果会自动同步。`
- `分账回退处理中，退款到账会在回退完成后继续推进。`

### 6.7 提现处理中

页面行为：

- `202` 或 `processing` 提示为受理，不提示到账。
- 刷新余额和流水。
- 安排短延迟刷新，但用户可手动刷新。

建议文案：

- `提现请求已受理，到账结果会在微信退款结果确认后同步到账单列表。`

## 7. 分阶段修复计划

## 阶段 0：冻结旧错误模式

### 目标

先阻止新代码继续把 `invokeWechatPay` 成功直接跳成功页。

### 任务

1. 在 `.github/standards/weapp` 或现有小程序支付规范中补一条：页面不得直接在 `invokeWechatPay` 后跳成功页，必须经过统一工作流或后端状态回查。
2. 在本轮涉及文件改造前，先用 grep 建立基线：`invokeWechatPay(`、`processPayment(`、`Navigation.toPaymentSuccess(`。
3. 把允许直接调用 `invokeWechatPay` 的位置限定到 `api/payment.ts`、`services/payment-workflow.ts`、`services/rider-deposit-payment.ts`、追偿迁移前临时入口。

### 验收

- 能列出所有直接调用微信支付的页面。
- 新页面开发有明确准入规则。

## 阶段 1：统一支付工作流和结果模型

### 目标

建立高内聚服务层，后续页面只接结果模型。

### 任务

1. 新增 `services/payment-workflow.ts`。
2. 新增 `utils/payment-result-view.ts`。
3. 把 [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts) 中 `processPayment` 的创建失败语义从 `unknown` 改为明确失败类状态，或废弃页面直接使用它。
4. 增加合单 workflow：创建、恢复、拉起、查询、归类。
5. 增加普通订单 workflow：创建、拉起、轮询、归类。
6. 增加预订 workflow：可保留现有业务详情回查，但统一结果枚举。
7. 增加追偿 workflow：保留支付单轮询，补充超时结果模型。

### 验收

- 服务层可覆盖普通订单、合单、预订、追偿、押金恢复的支付结果分类。
- 页面不再自己拼 `paid/failed/unknown`。
- 创建支付单失败不会返回 `unknown`。

## 阶段 2：新增或改造支付结果页

### 目标

让成功、取消、失败、确认中有统一可理解的承接页面。

### 任务

1. 新增 `pages/payment/result/index`，或重命名/兼容扩展现有成功页。
2. 页面支持入参：`businessType`、`businessId`、`paymentOrderId`、`combinedPaymentId`、`initialStatus`、`amount`、`source`。
3. 首屏根据 `initialStatus` 展示对应文案。
4. 如为确认中，自动查询一次，失败时展示“稍后刷新”。
5. 主按钮根据状态变化：成功看详情，确认中刷新，取消看详情，失败重新发起或看详情。

### 验收

- 普通订单不会因为传参直接显示成功。
- 合单支付确认中能显示合单语义。
- 结果页所有状态都有中文说明和下一步按钮。

## 阶段 3：改造订单支付入口

### 目标

修复最强假成功路径。

### 任务

1. 改造 [weapp/miniprogram/pages/takeout/order-confirm/index.ts](weapp/miniprogram/pages/takeout/order-confirm/index.ts)：
   - 单笔支付使用 `PaymentWorkflow.payOrder`。
   - 合单支付使用 `PaymentWorkflow.payCombinedOrder`。
   - `paid` 才跳成功结果页。
   - `pending_confirmation` 跳支付结果页确认中，或跳订单列表并带状态提示。
   - `cancelled` 跳订单详情/列表，不弹“失败”。
2. 改造 [weapp/miniprogram/pages/orders/detail/index.ts](weapp/miniprogram/pages/orders/detail/index.ts)：
   - 保留当前较谨慎的流程，但接入统一结果模型。
   - 合单支付成功后主动 query，而不是只在异常时 query。
3. 改造 [weapp/miniprogram/pages/orders/list/index.ts](weapp/miniprogram/pages/orders/list/index.ts)：
   - 批量合单支付在 `requestPayment` 成功后立即 query。
   - 如果仍同步中，提示“支付结果确认中，请稍后刷新订单列表”。
4. 改造 [weapp/miniprogram/pages/dine-in/checkout/checkout.ts](weapp/miniprogram/pages/dine-in/checkout/checkout.ts)：
   - 堂食支付必须轮询支付单。
   - 不再直接进入 [weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts](weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts)。
5. 改造堂食支付成功页：
   - 如保留，必须只接受已确认成功状态。
   - 去掉确认前倒计时跳转。

### 验收

- grep 不再出现页面级 `invokeWechatPay(...); Navigation.toPaymentSuccess(...)` 连用。
- 支付成功页只从明确 `paid` 或业务成功状态进入。
- 确认中状态不会被展示成成功。

## 阶段 4：修复预订、追偿、押金的状态模型一致性

### 目标

保留已有好路径，消除语义边角。

### 任务

1. 预订支付：
   - 保留回查预订详情的设计。
   - 修复 `processPayment` 创建失败归类，创建失败时显示“支付未发起”。
2. 追偿支付：
   - 抽出共用追偿支付 helper，商户和骑手页面复用。
   - 轮询超时时返回 `pending_confirmation`，页面展示明确中文反馈。
   - 追偿详情刷新失败时保留当前状态并提示可重试。
3. 骑手押金：
   - 保留 `rider-deposit-payment.ts` 的 pending context 设计，但“继续支付”应优先按原 `paymentOrderId` 恢复或查询，不应只按金额重新 submit。
   - 将押金充值结果枚举与通用 `PaymentWorkflowStatus` 对齐。
   - “继续支付”文案调整为“继续处理待支付单”或“继续支付”，说明它会恢复同一笔待确认充值。
   - 保留押金页面的余额、冻结、提现处理中展示。
   - 提现提交后不再只依赖余额延时刷新，应有本次提现/退款的处理中状态承接和刷新动作。

### 验收

- 预订创建支付单失败不再进入“确认中”。
- 追偿支付超时有用户可见提示。
- 押金充值/提现仍能清晰区分处理中和成功。
- 提现处理中不会无限 loading；短轮询结束后展示明确状态和刷新入口。

## 阶段 5：修复会员充值入口

### 目标

让小程序与后端“线上会员充值暂停”契约一致。

### 任务

1. 在 [weapp/miniprogram/pages/user_center/membership/index.ts](weapp/miniprogram/pages/user_center/membership/index.ts) 禁用线上充值动作。
2. [weapp/miniprogram/pages/user_center/membership/index.wxml](weapp/miniprogram/pages/user_center/membership/index.wxml) 中把按钮文案从“立即充值”改为“联系商户充值”或隐藏充值按钮。
3. 若保留入口，点击后展示中文说明：`会员线上充值已暂停，请联系商户线下充值后入账。`
4. [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts) 的快速充值入口也要同步，不再跳到不可完成流程。
5. [weapp/miniprogram/api/personal.ts](weapp/miniprogram/api/personal.ts) 中 `RechargeResponse` 可保留兼容，但页面不得依赖它拉起支付。

### 验收

- 用户不会进入选择金额后再失败的流程。
- 所有会员充值入口都给出一致中文说明。

## 阶段 6：修复退款和账单展示

### 目标

退款、分账回退、账单流水展示后端真实字段和中文状态。

### 任务

1. 修复 [weapp/miniprogram/pages/user_center/refund-detail/index.wxml](weapp/miniprogram/pages/user_center/refund-detail/index.wxml)：
   - `refund.reason` 改为 `refund.refund_reason`。
   - `refund.refund_transaction_id` 改为 `refund.out_refund_no`。
   - 移除或替换不存在的 `processed_at`。
2. 在 [weapp/miniprogram/pages/user_center/refund-detail/index.ts](weapp/miniprogram/pages/user_center/refund-detail/index.ts) 中构建 `ProfitSharingReturnView`，映射中文状态、主题、失败原因、完成时间。
3. 复用 [weapp/miniprogram/utils/merchant-order-detail-view.ts](weapp/miniprogram/utils/merchant-order-detail-view.ts) 中退款和分账回退状态映射，或抽到共享 util。
4. 修复 [weapp/miniprogram/pages/user_center/payment-detail/index.wxml](weapp/miniprogram/pages/user_center/payment-detail/index.wxml) 金额双 `¥` 问题。
5. 修复 [weapp/miniprogram/utils/payment-ledger-view.ts](weapp/miniprogram/utils/payment-ledger-view.ts)：未知支付状态不得默认已支付，未知退款状态不得默认退款成功。

### 验收

- 退款详情能正确展示退款原因、退款单号、退款状态、分账回退状态。
- 未知状态显示“状态同步中”，不是成功。
- 账单列表不会把未知状态染成成功。

## 阶段 7：修复危险动作文案和按钮层级

### 目标

让用户明确知道自己将关闭支付单，而不是关闭详情页。

### 任务

1. [weapp/miniprogram/pages/user_center/payment-detail/index.wxml](weapp/miniprogram/pages/user_center/payment-detail/index.wxml) 按钮文案改为“关闭支付单”。
2. 关闭按钮使用非 primary 的危险或次级样式。
3. 弹窗文案保留明确后果：`关闭后该支付单无法继续支付，如仍需付款需要重新发起。`
4. 支付中禁用关闭动作。

### 验收

- 按钮入口、弹窗标题、弹窗内容语义一致。
- 用户不会误以为只是关闭页面。

## 阶段 8：骑手押金账务闭环专项

### 目标

把骑手押金从“页面刷新问题”升级为资金账务闭环问题处理，确认充值入账、提现预留、微信退款成功、余额扣减、可退款 credit、流水展示都来自一致的后端真值。

### 后端任务

1. 建立押金账务不变量清单：
   - `riders.deposit_amount` 必须等于已入账押金减已成功提现和扣减后的主余额。
   - `riders.frozen_deposit` 必须能拆成配送冻结和提现处理中。
   - `rider_deposit_credits.refundable_amount` 合计不能大于可提现押金，且应与可退款窗口内的支付单一致。
   - `refund_orders(status IN pending, processing)` 必须和提现处理中金额一致。
2. 增加对账查询或内部审计脚本，找出这些漂移：
   - 主余额大于可退款 credit 合计。
   - 单个 `payment_order_id` 有重复押金入账流水。
   - `payment_orders.status='paid'` 且 `processed_at` 为空但已有押金流水或 credit。
   - `refund_orders` 已 success 但 `riders.deposit_amount` 或 `frozen_deposit` 未落账。
3. 检查 [locallife/api/rider.go](locallife/api/rider.go) 的提现入口余额判断，统一使用押金域可用余额口径，不只用 `DepositAmount - FrozenDeposit`。
4. 检查 [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go) 的押金充值幂等，补覆盖历史半处理状态的测试：已有 deposit log 无 credit、已有 credit 无 deposit log、processed_at 失败重试。
5. 检查 [locallife/db/sqlc/tx_rider_refund.go](locallife/db/sqlc/tx_rider_refund.go) 的提现成功和失败回滚幂等，确保重复回调、查询恢复、微信已全额退款异常不会重复扣减或重复恢复。
6. 为骑手提现增加面向小程序的查询能力，返回本次提现涉及的 refund orders、accepted amount、processing amount、success amount、failed amount 和用户可读状态。

### 小程序任务

1. 押金页新增或收敛 `RiderDepositActionStatus` 视图模型，覆盖充值待确认、充值成功、提现已受理、提现处理中、提现成功、提现失败退回、同步失败。
2. 提现提交成功后保存本次提现上下文，包括 refund order ids 或后端返回的提现跟踪字段。
3. 提交后立即刷新余额和流水，再短轮询提现状态；短轮询结束仍未终态时停止 loading，显示“提现处理中，到账结果会同步到账单，可稍后刷新”。
4. `onShow`、下拉刷新、点击刷新都查询本次提现状态和押金余额，终态后清理本地 pending context。
5. 当微信已通知成功但本地余额仍未变化时，展示“微信已返回成功，系统正在同步账务，请稍后刷新”，同时保留问题可见，不显示成普通失败。

### 验收

- 充值 100 元不会出现主余额 200 元、可提现 credit 100 元的账务漂移。
- 微信退款/提现成功回调后，押金页可在刷新后看到可用余额、提现处理中金额和流水同步变化。
- 提现处理中不会阻塞用户在 loading 态；页面有明确状态和刷新入口。
- 后端重复回调、恢复调度、查询事实应用不会重复入账或重复扣账。

## 8. 页面级目标行为矩阵

| 页面 | 当前问题 | 修复后行为 |
| --- | --- | --- |
| 外卖确认页 | JSAPI 成功直接成功页 | 支付后轮询，`paid` 成功，超时确认中，取消去订单详情/列表 |
| 堂食结账页 | JSAPI 成功直接成功页 | 支付后轮询，确认后才成功，否则去结果页或订单详情 |
| 订单详情页 | 部分合单成功后未立即 query | 合单支付后无论是否异常都 query 一次 |
| 订单列表页 | 新建合单支付后成功判断偏弱 | 支付后 query 合单状态，确认中提示刷新 |
| 支付详情页 | 继续支付直接成功页 | 接统一 workflow，确认后才成功 |
| 预订确认/详情/列表 | 创建失败被归为 unknown | 创建失败显示支付未发起，支付后空窗才 unknown |
| 预订结果页 | 当前较好 | 保留回查业务详情模型，可迁移到通用结果页 |
| 追偿详情页 | 轮询超时提示不清晰 | 超时展示确认中，并提供刷新追偿状态 |
| 骑手押金页 | 提现缺少本次终态承接，后端账务可能漂移 | 单独做 G3 押金闭环；页内组件承接终态，余额、credit、refund 状态一致 |
| 会员页/钱包页 | 线上充值已暂停但入口仍在线 | 禁用或改为联系商户充值 |
| 退款详情页 | 字段漂移、原始状态 | 修字段，中文状态，失败原因和下一步 |
| 钱包账单 | 未知状态默认成功 | 未知状态显示同步中 |

## 9. 建议实现顺序

优先级按用户误导风险排序。

1. 阶段 1 和阶段 2：统一 workflow 与结果页。没有这一层，后续页面会继续重复修。
2. 阶段 3：外卖、堂食、支付详情页假成功修复。
3. 阶段 5：会员充值入口契约漂移修复，范围小但用户体感强。
4. 阶段 6：退款详情与账单状态修复。
5. 阶段 8：骑手押金账务闭环专项，优先级应高于普通体验收尾。
6. 阶段 4：预订、追偿一致性收敛，押金只做与专项衔接。
7. 阶段 7：危险动作文案和层级。

也可以拆成三个 PR：

- PR 1：支付 workflow、结果页、外卖/堂食/支付详情假成功修复。
- PR 2：会员充值、退款详情、账单、追偿一致性和文案修复。
- PR 3：骑手押金账务闭环专项，包含后端对账、接口和小程序状态承接。

如果希望降低单次发布风险，建议拆成四个 PR：

- PR A：基础设施和一个样板页面，建议选支付详情页或堂食结账页。
- PR B：外卖单笔、合单、订单列表/详情全面接入。
- PR C：会员、退款、账单、追偿收尾。
- PR D：骑手押金账务闭环专项，单独评审和验证。

## 10. 可立即落地的任务切片

当前阶段拆解已经能指导方向，但如果要马上开始修复，还需要按“一个任务只解决一个闭环”继续切小。建议以下切片作为实际落地顺序。

### 切片 0：冻结旧支付成功模式

目标：先防止新代码继续扩散假成功模式。

范围：文档、grep 基线，不改业务行为。

交付：

1. 在小程序标准中补充规则：页面不得在 `invokeWechatPay` 成功后直接跳业务成功页。
2. 记录当前直接调用清单：`invokeWechatPay(`、`Navigation.toPaymentSuccess`、`processPayment(`。
3. 标记迁移期允许例外：底层支付 API、统一 workflow、押金充值恢复、追偿临时入口。

验收：

- 能从 grep 清单看出每个旧入口后续落到哪个任务切片。
- 新增页面没有理由继续复制旧模式。

建议优先级：最高，先做。

### 切片 1：支付结果类型和文案模型

目标：先把状态语言统一，但不接任何真实页面。

范围：

- 新增 `weapp/miniprogram/services/payment-workflow.ts` 的类型、状态归类 helper。
- 新增 `weapp/miniprogram/utils/payment-result-view.ts`。

不包含：

- 不新增页面。
- 不改订单、堂食、押金、追偿入口。
- 不调用微信支付。

验收：

- `paid`、`pending_confirmation`、`cancelled`、`create_failed`、`pay_params_missing`、`failed`、`closed` 都能生成中文标题、说明、主按钮和次按钮。
- 创建失败不再与支付后 unknown 混用。

建议优先级：切片 0 后立即做。

### 切片 2：通用支付结果页骨架

目标：提供统一承接页面，但先只支持传入状态展示和手动刷新占位。

范围：

- 新增 `weapp/miniprogram/pages/payment/result/*`。
- 更新 `app.json` 页面注册。
- 新增或扩展导航 helper：`Navigation.toPaymentResult`。

不包含：

- 不改外卖、堂食、订单列表入口。
- 不做复杂合单子单展示。
- 不处理押金提现。

验收：

- 传入 `initialStatus=paid` 展示成功。
- 传入 `initialStatus=pending_confirmation` 展示确认中和刷新按钮。
- 传入 `initialStatus=cancelled/create_failed/failed/closed` 展示不同中文处置。
- 页面无营销成功假象，确认中不会显示成成功。

建议优先级：切片 1 后做。

### 切片 3：支付详情页继续支付样板

目标：选一个入口先完整跑通“创建/恢复、拉起、查询、结果页”的样板，降低后续页面改造风险。

范围：

- [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts)
- [weapp/miniprogram/pages/user_center/payment-detail/index.wxml](weapp/miniprogram/pages/user_center/payment-detail/index.wxml)
- 必要的 workflow 查询普通支付单能力。

不包含：

- 不改外卖下单。
- 不改合单。
- 不改堂食。

验收：

- 继续支付后只有后端确认 `paid` 才展示成功。
- 微信支付取消展示“已取消支付，可在详情页继续支付”。
- 轮询超时进入确认中，不报失败。
- “关闭支付详情”改成“关闭支付单”，危险动作语义清楚。

建议优先级：作为第一个业务样板。

### 切片 4：外卖单笔支付接入样板

目标：修掉最常见的单笔订单假成功。

范围：

- [weapp/miniprogram/pages/takeout/order-confirm/index.ts](weapp/miniprogram/pages/takeout/order-confirm/index.ts) 单商户单笔分支。
- [weapp/miniprogram/utils/takeout-order-confirm-support.ts](weapp/miniprogram/utils/takeout-order-confirm-support.ts) 必要参数和结果处理。

不包含：

- 不改合单。
- 不改订单列表批量支付。
- 不改堂食。

验收：

- 单笔支付后必须查询支付单或业务订单状态。
- 微信成功但后端未确认时进入确认中。
- 用户取消不提示支付失败。

建议优先级：支付详情样板稳定后做。

### 切片 5：合单支付状态接入

目标：把合单从“微信完成即成功”改成“合单 query 明确成功才成功”。

范围：

- 外卖确认页多商户合单分支。
- [weapp/miniprogram/pages/orders/detail/index.ts](weapp/miniprogram/pages/orders/detail/index.ts) 合单继续支付分支。
- [weapp/miniprogram/pages/orders/list/index.ts](weapp/miniprogram/pages/orders/list/index.ts) 批量合单分支。

不包含：

- 不改普通单笔支付已完成部分。
- 不做合单子订单详情页重构。

验收：

- 合单支付后无论微信返回成功还是查询异常，都不会直接展示业务成功。
- query 成功才显示成功。
- query 同步中展示确认中，并允许回订单列表刷新。

建议优先级：切片 4 后做。

### 切片 6：堂食支付接入

目标：堂食成功页只承接已确认成功。

范围：

- [weapp/miniprogram/pages/dine-in/checkout/checkout.ts](weapp/miniprogram/pages/dine-in/checkout/checkout.ts)
- [weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts](weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts)
- [weapp/miniprogram/pages/dine-in/payment-success/payment-success.wxml](weapp/miniprogram/pages/dine-in/payment-success/payment-success.wxml)

不包含：

- 不改外卖。
- 不新增堂食营销功能。

验收：

- 堂食支付确认中不进入成功页。
- 成功页不再承担确认前倒计时。
- 取消支付后回到订单详情或结账页，并提示可继续支付。

建议优先级：切片 5 后做。

### 切片 7：会员充值入口停用

目标：快速修复与后端 `503` 契约不一致的问题。

范围：

- [weapp/miniprogram/pages/user_center/membership/index.ts](weapp/miniprogram/pages/user_center/membership/index.ts)
- [weapp/miniprogram/pages/user_center/membership/index.wxml](weapp/miniprogram/pages/user_center/membership/index.wxml)
- [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts)

不包含：

- 不删除 API 类型。
- 不做线下充值新流程。

验收：

- 用户不会进入线上会员充值支付流程。
- 所有入口文案一致：线上充值已暂停，请联系商户线下充值后入账。

建议优先级：可与切片 1 到 3 并行。

### 切片 8：退款详情和账单状态字段修复

目标：修复明确字段漂移和未知状态误判成功。

范围：

- [weapp/miniprogram/pages/user_center/refund-detail/index.ts](weapp/miniprogram/pages/user_center/refund-detail/index.ts)
- [weapp/miniprogram/pages/user_center/refund-detail/index.wxml](weapp/miniprogram/pages/user_center/refund-detail/index.wxml)
- [weapp/miniprogram/utils/payment-ledger-view.ts](weapp/miniprogram/utils/payment-ledger-view.ts)

不包含：

- 不改退款创建流程。
- 不改后端退款接口。

验收：

- `refund_reason`、`out_refund_no`、`refunded_at` 展示正确。
- 分账回退状态展示中文。
- 未知支付状态不显示已支付，未知退款状态不显示退款成功。

建议优先级：可与支付 workflow 并行。

### 切片 9：追偿支付确认中文化

目标：保留现有轮询方向，补齐确认中和刷新文案。

范围：

- [weapp/miniprogram/pages/rider/claims/detail/index.ts](weapp/miniprogram/pages/rider/claims/detail/index.ts)
- [weapp/miniprogram/pages/merchant/claims/detail/index.ts](weapp/miniprogram/pages/merchant/claims/detail/index.ts)

不包含：

- 不改追偿后端状态机。
- 不改赔付结算逻辑。

验收：

- 轮询超时展示“追偿支付已提交，系统正在确认到账结果”。
- 页面提供刷新追偿状态动作。
- 刷新失败保留当前状态，不误报成功或失败。

建议优先级：切片 7、8 后做。

### 切片 10：骑手押金后端账务审计

目标：先证明或定位押金主余额、冻结、credit、refund order、流水漂移。

范围：

- 后端对账查询、测试或内部审计脚本。
- [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go)
- [locallife/db/sqlc/tx_rider_refund.go](locallife/db/sqlc/tx_rider_refund.go)
- [locallife/api/rider.go](locallife/api/rider.go)

不包含：

- 不先改小程序 UI。
- 不先改变提现页面交互。

验收：

- 能发现主余额大于可退款 credit、重复押金入账、refund success 未落账等异常。
- 充值、提现、重复回调、恢复查询有测试覆盖。
- 提现入口和余额接口使用同一可用余额口径。

建议优先级：作为押金专项第一步，优先级高于普通体验收尾。

### 切片 11：骑手提现状态查询接口

目标：给小程序一个本次提现维度的真值查询对象。

范围：

- 后端新增或扩展骑手提现查询能力。
- 返回 accepted amount、processing amount、success amount、failed amount、refund orders 和用户可读状态。

不包含：

- 不改小程序押金页 UI。
- 不改微信退款底层 client。

验收：

- 小程序不需要只靠 `/v1/rider/deposit` 判断本次提现是否完成。
- 查询接口能表达已受理、处理中、成功、失败/退回。

建议优先级：切片 10 后做。

### 切片 12：骑手押金页状态组件

目标：用页内组件承接充值和提现终态，不让用户卡在 loading 或错误 toast。

范围：

- [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts)
- [weapp/miniprogram/services/rider-deposit-payment.ts](weapp/miniprogram/services/rider-deposit-payment.ts)
- [weapp/miniprogram/services/rider-deposit-finance.ts](weapp/miniprogram/services/rider-deposit-finance.ts)

不包含：

- 不在后端状态接口完成前硬编码前端推断。
- 不新增独立提现进度页。

验收：

- 提现提交后短轮询，超时后停止 loading。
- 页面展示“提现处理中，到账结果会同步到账单，可稍后刷新”。
- `onShow`、下拉刷新、点击刷新都能恢复本次提现状态。
- 微信已提示成功但本地未同步时，展示同步中，不显示普通失败。

建议优先级：切片 10、11 后做。

### 推荐第一轮开工包

如果现在就开始落地，建议第一轮只拿这些切片：

1. 切片 0：冻结旧支付成功模式。
2. 切片 1：支付结果类型和文案模型。
3. 切片 2：通用支付结果页骨架。
4. 切片 3：支付详情页继续支付样板。
5. 切片 7：会员充值入口停用。
6. 切片 10：骑手押金后端账务审计。

这组任务边界清楚、风险可控，而且能同时推进两条主线：一条修支付假成功，一条定位押金账务真相。

## 11. 验证计划

### 11.1 静态验证

运行范围：`weapp/`。

建议命令：

```bash
npm run lint
npm run compile
```

静态 grep 验收：

```bash
rg "invokeWechatPay\(" weapp/miniprogram/pages weapp/miniprogram/services
rg "Navigation\.toPaymentSuccess" weapp/miniprogram
rg "status: 'unknown'" weapp/miniprogram/api weapp/miniprogram/services
```

期望：

- 页面级直接 `invokeWechatPay` 调用显著减少，只保留迁移前已说明例外。
- 不再出现 `invokeWechatPay` 后无查询直接成功页的模式。
- 创建支付失败不再被包装为 `unknown`。

### 11.2 手工场景验证

普通订单支付：

1. 正常支付成功：进入成功结果页，订单详情显示已支付或处理中。
2. 微信支付取消：提示已取消，跳订单详情或列表，可继续支付。
3. 微信支付成功但轮询超时：进入确认中页，点击刷新可继续查询。
4. 创建支付单失败：提示支付未发起，不进入确认中。

合单支付：

1. 合单支付成功并 query 成功：进入合单成功结果。
2. 合单支付后 query 仍同步中：提示确认中，订单列表可刷新。
3. 合单取消：提示可继续完成原合单支付。
4. 合单失效：提示重新发起合单支付。

堂食支付：

1. 支付确认成功才展示堂食成功。
2. 确认中不展示营销成功页。
3. 支付取消后跳订单详情并提示可继续支付。

预订支付：

1. 创建支付单失败显示支付未发起。
2. 支付后回查预订详情成功，结果页展示业务状态。
3. 回查失败展示确认中和刷新按钮。

会员充值：

1. 我的会员页点击充值不再进入微信支付。
2. 钱包快速充值入口不再跳不可完成流程。
3. 文案明确联系商户线下充值后入账。

退款详情：

1. 展示退款原因 `refund_reason`。
2. 展示退款单号 `out_refund_no`。
3. 分账回退状态显示中文。
4. 退款处理中、成功、失败、关闭都有明确状态。

骑手押金：

1. 充值成功、取消、确认中仍符合现有体验。
2. 待确认充值可恢复。
3. 提现处理中金额刷新正确。
4. 微信退款成功回调后，刷新页面能看到余额和流水变化。
5. 后端对账脚本或测试覆盖主余额、冻结金额、credit、refund order 的一致性。

### 11.3 后端契约验证

不要求本轮修改后端，但需要对齐这些已知契约：

- 普通支付详情：`GET /v1/payments/:id`。
- 合单支付查询：`GET /v1/payments/combined/:id/query`。
- 退款详情：`GET /v1/refunds/:id` 返回 `refund_reason`、`out_refund_no`、`refunded_at`。
- 会员充值：`POST /v1/memberships/recharge` 当前返回 `503`。
- 骑手提现：`POST /v1/rider/withdraw` 可能返回 `200 success` 或 `202 processing`。
- 骑手押金余额：`GET /v1/rider/deposit` 当前返回总押金、冻结押金、配送冻结、提现处理中和可用押金，但还缺少本次提现维度的终态查询。

## 12. 验收标准

本轮修复完成后，需要满足以下标准：

1. 小程序任一支付入口都不会在仅 `wx.requestPayment` 成功后展示业务成功。
2. 创建支付单失败、用户取消、支付后确认中、明确成功、明确失败、支付关闭都有不同中文文案。
3. 普通订单、合单、堂食、支付详情页走统一支付工作流。
4. 会员充值入口与后端暂停契约一致。
5. 退款详情不再读取不存在字段，不再展示原始英文状态。
6. 钱包账单未知状态不再默认成功。
7. 危险动作“关闭支付单”文案和按钮层级清晰。
8. 骑手押金充值、提现、退款不会出现主余额、可用余额、可退款 credit、流水互相矛盾。
9. `npm run compile` 和 `npm run lint` 通过，或明确记录已有无关失败。

## 13. 开放问题

1. 通用支付结果页是新增 `pages/payment/result`，还是兼容扩展现有 `pages/orders/success`？建议新增。
2. 普通订单支付后确认中，默认跳结果页还是订单详情页？建议结果页承接首次确认中，详情页作为次按钮。
3. 合单支付确认中是否需要展示子订单列表？建议第一版只展示总金额和订单数，详情交给订单列表。
4. 会员线上充值暂停是长期策略还是临时策略？若长期暂停，应从 API 类型和入口文案上彻底移除“微信支付充值”心智。
5. 商户收付通提现 API 当前小程序没有实际页面入口。若后续开放，需要单独按 `202 pending_confirmation` 和 `sync_state stale` 设计页面。
6. 骑手押金是否新增独立“提现进度”页面？建议第一版先做押金页内状态组件，只有多笔提现或需要客服凭证时再拆页面。
7. 后端是否允许同一骑手同时存在多笔提现处理中？建议第一版保持单笔处理中限制，降低账务和用户理解复杂度。

## 14. 第一稿结论

当前最大风险不是某个页面少了一次刷新，而是支付结果语义散落在页面中，导致多个入口把“支付交互完成”误当成“业务终态成功”。修复应先建立统一支付工作流和结果页语义，再逐页接入。

第一版建议以“普通订单、合单、堂食、支付详情”作为支付结果语义主战场，因为这些页面直接产生假成功体验；同时快速修会员充值入口和退款字段漂移，消除明确契约错误。骑手押金需要单独升级为资金账务闭环专项：先确认后端主余额、冻结金额、可退款 credit、refund order 和流水的一致性，再做小程序页内状态组件和刷新承接。预订链路已有较好样板，应保留其回查和 pending 承接思路，并逐步并入统一结果模型。

## 15. 落地进度

### 2026-04-28 阶段 0：冻结旧支付成功模式

状态：已落地，待阶段复审。

变更：

1. `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` 已收紧支付、退款、提现结果展示规则：资金结果页不得把中间态当成可停留的页面级结果，等待终态时保持 loading 或页内动作状态，明细列表不得展示处理中流水。
2. `weapp/scripts/check-payment-workflow-boundary.js` 已新增支付工作流边界门禁，阻止页面/组件直接调用 `invokeWechatPay`，阻止页面/组件继续使用 legacy `processPayment`，阻止旧 `Navigation.toPaymentSuccess` 或 `pages/orders/success` 路由回流。
3. `weapp/package.json` 已把 `gate:payment-workflow-boundary` 接入 `gate:weapp` 和 `quality:check`。

当前 grep 基线：

- `invokeWechatPay(` 只剩 `api/payment.ts` 定义，以及 `services/payment-workflow.ts`、`services/rider-deposit-payment.ts`、`services/claim-recovery-payment.ts` 三个允许的服务边界调用。
- 主小程序页面和组件中未发现直接 `invokeWechatPay(` 调用。
- 主小程序中未发现 `Navigation.toPaymentSuccess` 或 `pages/orders/success` 旧支付成功路由。

已验证：

- `npm run gate:payment-workflow-boundary` 通过。

阶段 0 剩余风险：

- `services/claim-recovery-payment.ts` 仍作为迁移期允许例外保留，后续阶段 4 需要决定迁入通用 payment workflow，或在标准中保留更窄的正式例外说明。

### 2026-04-28 阶段 1：统一支付工作流和结果模型

状态：已落地，待阶段复审。

变更：

1. `weapp/miniprogram/services/payment-workflow.ts` 已作为普通支付和合单支付的统一 workflow owner，覆盖创建、拉起、轮询、查询和结果归类。
2. `weapp/miniprogram/utils/payment-result-view.ts` 已作为支付结果中文文案和动作视图模型 owner，覆盖 `paid`、`cancelled`、`pending_confirmation`、`create_failed`、`pay_params_missing`、`closed`、`failed`。
3. `weapp/miniprogram/services/rider-deposit-payment.ts` 的押金充值状态已从专属 `submitted_pending_confirmation` / `unknown` 收敛为通用 `pending_confirmation`，保留押金域自己的余额刷新和 pending context。

当前 grep 基线：

- 主小程序 `weapp/miniprogram` 中未发现 `processPayment(` 调用。
- 主小程序 `weapp/miniprogram` 中未发现支付流程结果 `status: 'unknown'`。
- 主小程序 `weapp/miniprogram` 中未发现 `submitted_pending_confirmation`。

已验证：

- `npm run compile` 通过。

阶段 1 剩余风险：

- 押金充值仍是独立 domain workflow，尚未复用普通支付结果页；这是保留的领域边界，后续阶段 4 和阶段 8 再决定是否进一步并入通用 workflow 或保留正式例外。

### 2026-04-28 阶段 2：通用支付结果页

状态：已复核通过，当前实现满足最新规则。

当前实现：

1. `weapp/miniprogram/pages/payment/result/index.ts` 已作为通用支付结果页 owner，支持 `status`、`paymentOrderId`、`businessType`、`businessId`、`orderNo`、`amount` 和 `returnStatus` 入参。
2. `pending_confirmation` 不再渲染页面级结果卡；有 `paymentOrderId` 时进入 `waitingForTerminal` loading 并持续等待后端终态，没有 `paymentOrderId` 时降级为 `pay_params_missing`，避免把无法查询的支付展示成确认中。
3. `weapp/miniprogram/pages/payment/result/index.wxml` 只在 `waitingForTerminal=false` 时渲染 `t-result` 和结果操作按钮。
4. 合单支付当前不把非终态送入结果页停留；合单成功后才使用通用结果页，非终态由订单列表/详情刷新承接。

已验证：

- 代码复核确认 `pending_confirmation` 分支只展示 TDesign loading，不展示 `t-result` 中间态。
- 阶段 2 未改业务代码；沿用阶段 1 的 `npm run quality:check` 通过结果作为当前基线。

阶段 2 剩余风险：

- 堂食仍保留 `pages/dine-in/payment-success` 作为已确认成功后的专属成功页；它已有 `confirmed=1` 防护，不属于当前假成功缺陷，但后续阶段 3 可继续清理为统一结果页，减少 success 语义残留。

### 2026-04-28 阶段 3：订单支付入口

状态：已落地，待阶段复审。

变更：

1. 外卖确认页、订单详情页、订单列表页已经通过 `services/payment-workflow.ts` 完成普通支付和合单支付终态确认；页面层不直接调用 `invokeWechatPay`。
2. 堂食结账页 `pages/dine-in/checkout/checkout.ts` 已改为支付后统一跳转 `pages/payment/result`，不再进入专属 success 页面。
3. 旧堂食成功页 `pages/dine-in/payment-success/*` 已删除，`app.json` 路由和 `Navigation.toDineInPaymentSuccess` helper 已清理。

当前 grep 基线：

- 主小程序中未发现 `toDineInPaymentSuccess`。
- 主小程序中未发现 `payment-success/payment-success` 或 `pages/dine-in/payment-success` 路由引用。

已验证：

- `npm run compile` 通过。

阶段 3 剩余风险：

- 合单非终态当前回订单列表/详情刷新承接，不进入支付结果页；这符合“不中间态页面级停留”的最新规则，但真实微信设备上的合单延迟回写仍需要手工场景验证。

### 2026-04-28 阶段 4：预订、追偿、押金状态模型

状态：已落地，待阶段复审。

变更：

1. 预订确认页 `pages/reservation/confirm/index.ts` 的支付异常兜底从 `pending_confirmation` 改为 `create_failed`，避免创建/拉起异常进入确认中停留语义。
2. 追偿支付 helper `services/claim-recovery-payment.ts` 已返回通用 `PaymentWorkflowStatus`；轮询超时统一返回 `pending_confirmation`，明确由详情页动作提示承接。
3. 商户和骑手追偿详情页现在把 `closed`、`cancelled` 与失败状态同样视为“追偿支付未完成，可刷新后重试”，避免关闭类状态落入不可操作的确认中提示。
4. 押金充值状态枚举已在阶段 1 对齐通用 `PaymentWorkflowStatus`；提现提交后已在前序修复中等待终态并保留页内刷新入口，本阶段未重复改动押金代码。

已验证：

- `npm run compile` 通过。

阶段 4 剩余风险：

- 追偿支付轮询超时后的业务域状态刷新依赖详情页 `loadDetail(true, true)` 和用户手动刷新；真实延迟到账场景仍需要微信设备和后端回调联调验证。
- 押金后端账务不变量仍归入阶段 8 的 G3 专项，不能用本阶段前端状态收敛替代。

### 2026-04-28 阶段 5：会员充值入口

状态：已复核通过，无新增代码改动。

当前实现：

1. `pages/user_center/membership/index.wxml` 的会员卡按钮已显示“联系商户充值”，不再展示“立即充值”。
2. `pages/user_center/membership/index.ts` 的手动充值和 `autoRecharge=1` 入口均调用 `showMembershipRechargePausedMessage()`，不进入选择金额或微信支付流程。
3. `pages/user_center/wallet/index.ts` 的快捷充值入口统一展示 `MEMBERSHIP_RECHARGE_PAUSED_MESSAGE`，并在有会员卡上下文时提供查看商户动作。
4. `api/personal.ts` 的 `rechargeMembership` 只作为兼容 API 保留，页面层没有依赖它拉起支付。

已验证：

- `rg "立即充值|startRecharge|createRecharge|rechargeMembership|autoRecharge|MEMBERSHIP_RECHARGE_PAUSED_MESSAGE|会员线上充值已暂停" weapp/miniprogram --glob '*.{ts,wxml}'` 确认会员充值入口均落到暂停提示，未发现页面继续发起会员线上充值支付。
- 阶段 5 未改业务代码；沿用阶段 4 的 `npm run quality:check` 通过结果作为当前基线。

阶段 5 剩余风险：无新增残留。若未来恢复会员线上充值，需要重新定义后端终态真值和页面等待策略后再开启入口。

### 2026-04-28 阶段 6：退款和账单展示

状态：已落地，待阶段复审。

变更：

1. `pages/user_center/refund-detail/index.ts` 已从 `refund_reason`、`out_refund_no`、`refunded_at` 构建展示字段，WXML 不再直接读取旧字段或不存在字段。
2. 退款详情页在退款非终态时只展示 `t-loading` 等待终态，不把处理中展示成页面级结果。
3. 分账回退已通过 `utils/profit-sharing-return-view.ts` 映射中文状态、主题、失败原因和完成时间，并且只展示终态回退记录。
4. `utils/payment-ledger-view.ts` 已把未知支付/退款状态映射为“状态同步中”，不会默认染成成功。
5. `pages/user_center/payment-detail/index.ts` 的退款列表补齐 `_reasonText`，统一使用 `refund.refund_reason`；WXML 不再读取旧 `reason` 字段。

已验证：

- `npm run compile` 通过。
- `rg "reason \\|\\||refund.reason|refund_transaction_id|processed_at" weapp/miniprogram/pages/user_center --glob '*.{ts,wxml}'` 复核与支付退款相关的旧字段残留；仅服务中心工单自己的 `reason/processed_at` 仍存在，不属于支付退款字段。

阶段 6 剩余风险：真实退款处理中长时间未回写时，页面会持续等待；这是符合“不中间态停留”的规则，但需要后端退款终态回查可用性保障用户最终能看到结果。

### 2026-04-28 阶段 7：危险动作文案和按钮层级

状态：已落地，待阶段复审。

变更：

1. `pages/user_center/payment-detail/index.wxml` 的关闭动作按钮文案为“关闭支付单”，使用 `theme="danger" variant="outline"`，与“继续支付”主按钮区分。
2. 弹窗标题为“关闭支付单”，内容明确说明“关闭后该支付单无法继续支付，如仍需付款需要重新发起。”
3. 关闭按钮在 `paying` 时禁用；`onClosePayment()` 也补充 `paying` guard，避免事件重复触发时关闭正在拉起的支付单。
4. 关闭入口仅在 `statusView.isPending` 时出现，不对已支付、已关闭、失败等终态展示危险动作。

已验证：

- 定向 grep 复核按钮文案、弹窗文案、`showCloseButton` 和 `disabled="{{paying}}"`。

阶段 7 剩余风险：无新增残留。

### 2026-04-28 阶段 8：骑手押金账务闭环专项

状态：已落地，待阶段复审。

已确认闭环：

1. `/v1/rider/deposit` 已使用 `GetPendingRiderDepositRefundAmountByUserID` 和 `db.CalculateRiderDepositAvailability()`，将配送冻结与提现处理中金额拆开计算，可用押金不会只依赖 `riders.frozen_deposit`。
2. `SubmitWithdrawal()` 和 `PrepareRiderDepositRefundTx()` 已使用同一押金可用余额口径；事务层会锁 rider、扣减可退款 credit、创建 refund order 和隐藏型 freeze 流水。
3. `ResolveRiderDepositRefundTx()` 已覆盖提现成功扣减、失败/关闭恢复、重复终态回调幂等和微信已全额退款后的 stale credit drain。
4. `/v1/rider/withdrawals/status` 已返回本次提现 refund orders、accepted/processing/success/failed 汇总和用户可读状态，小程序能等待终态或展示刷新入口。
5. `ListRiderDepositLedgerAnomalies` 已作为内部押金账务审计查询，覆盖主余额与 credit 漂移、冻结小于提现处理中、重复押金入账流水、成功退款未落账。

本阶段新增：

1. `db/query/rider_deposit.sql` 的 `ListRiderDepositLedgerAnomalies` 新增 `paid_unprocessed_has_artifacts` 异常，识别 `payment_orders.status='paid' AND processed_at IS NULL` 但已经存在押金流水或 credit 的半处理漂移。
2. `db/sqlc/tx_rider_refund_test.go` 的 `TestListRiderDepositLedgerAnomaliesDetectsDrift` 新增半处理样例，确保内部审计能看见这类已产生账务痕迹但支付单未标记 processed 的高风险状态。

已验证：

- `make sqlc` 已运行，生成 `db/sqlc/rider_deposit.sql.go`。
- `go test -run 'TestProcessPaymentSuccessTx_RiderDeposit|TestPrepareRiderDepositRefundTx|TestListRiderDepositLedgerAnomaliesDetectsDrift|TestListRiderDepositWithdrawalRefundOrdersByIDs|TestResolveRiderDepositRefundTx' ./db/sqlc` 通过。
- `make check-generated` 通过，生成物一致。

阶段 8 剩余风险：

- 本阶段补齐的是内部审计可见性和既有事务链路复核；未连接真实微信退款回调或生产对账任务跑一遍全链路。因此真实设备提现、微信退款回调延迟、重复通知和人工对账执行仍需要集成环境或沙箱环境验证。
