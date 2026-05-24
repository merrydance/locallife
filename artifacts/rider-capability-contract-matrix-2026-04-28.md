# 骑手能力契约矩阵

Date: 2026-04-28
Stage: `S0-01`
Risk class: `G2`
Status: reviewed

## 1. 目标

本矩阵冻结骑手侧能力组、后端真值、小程序承载和实施缺口，作为后续 `BE-*` 与 `WEAPP-*` 任务卡的边界基线。

矩阵遵循高内聚低耦合原则：能力组按骑手日常经营任务组织，不按接口数量拆分页面；同步读模型、异步事件、页面恢复和组件承载必须成组设计。

## 2. 不变量

- 骑手是自主经营取送业务的个人承包商，不建设平台派单、排班或过程托管。
- 骑手代取费通过微信分账到个人 `openid`，不建设骑手收入提现钱包。
- 微信同城代取订单上报、冻结期和解冻事件属于微信订单/支付事实域，不耦合到骑手页面或 rider income 读模型。
- 骑手端只展示骑手可理解、可行动、可回读的经营状态。
- 实时消息只做提醒，最终状态必须能由业务接口或持久通知回读。

## 3. 能力组矩阵

| 能力组 | 骑手任务 | 后端同步真值 | 异步/恢复来源 | 小程序 owner | 当前状态 | 缺口 | 后续任务卡 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| A. 经营工作台 | 判断能否接单、手上任务、今日经营、待处理风险 | `/v1/rider/me`、`/v1/rider/status`、`/v1/delivery/active`、`/v1/delivery/recommend`、押金余额、追偿 summary、通知 unread | WebSocket 新单与订单状态，DB 通知，页面 onShow 回读 | `weapp/miniprogram/pages/rider/dashboard` | 页面已存在但依赖多源拼装，收入/押金/追偿摘要未形成统一经营视图 | 缺少 rider workbench summary 聚合读模型；dashboard 需要按能力组件重排 | `BE-02`、`WEAPP-01`、`WEAPP-03` |
| B. 接单与履约 | 自主发现订单、抢单、履约、导航、历史任务 | `/v1/delivery/recommend`、`/v1/delivery/grab/:order_id`、`/v1/delivery/active`、`/v1/delivery/:delivery_id/*`、`/v1/delivery/history` | WebSocket 订单池消息、replay filter、定位上报回读 | `dashboard`、`task-detail`、`navigation`、`tasks` | 主链路已具备，历史任务仍展示 `rider_earnings` 估算/履约口径 | 需要弱网/重入恢复收口；历史任务不能承担分账到账解释 | `WEAPP-04` |
| C. 代取费分账账本 | 查看代取费是否到账、每单状态、失败原因 | `profit_sharing_orders` 已有 `GetRiderProfitSharingStats`、`ListRiderProfitSharingOrders`、`GetRiderDailyIncome` | 分账结果通知、账本回读 | 建议新增 `weapp/miniprogram/pages/rider/income`；dashboard 只承接 snapshot | SQL 基础存在，骑手自助 API 和小程序页面缺失 | 需要 rider income API；现有 SQL 缺少 count/status summary/filter 的完整账本支持 | `BE-01`、`WEAPP-02` |
| D. 押金与风险保障 | 查看可用押金、冻结、提现处理中、充值/押金退款 | `/v1/rider/deposit`、`/v1/rider/deposits`、`/v1/rider/deposit` POST、`/v1/rider/withdraw`、`/v1/rider/withdrawals/status` | 支付单回查、退款状态回查、页面重入恢复 | `weapp/miniprogram/pages/rider/deposit`；服务 owner 已有 `rider-deposit-*` | 押金 workflow 已收口，dashboard 仅有阻塞提醒 | 需要把押金风险摘要接入工作台；保持押金与代取收入边界 | `WEAPP-05` |
| E. 追偿与申诉 | 查看待处理追偿、支付追偿、提交申诉、查看结果 | `/v1/rider/claims`、`/v1/rider/claims/summary`、`/v1/rider/recoveries/:id`、`/v1/rider/recovery-disputes` | 追偿/申诉通知，onShow 回读 | `weapp/miniprogram/pages/rider/claims` | 列表、详情、支付、申诉链路已有页面承载 | dashboard 缺少待处理追偿摘要；需保证返回后状态恢复 | `WEAPP-06` |
| F. 关键通知与恢复 | 弱网、断线、后台返回后知道关键财务/风险事件 | `/v1/notifications`、`/v1/notifications/unread/count`、业务接口回读 | WebSocket Redis replay、DB notification inbox | 通用通知页或新增 rider notification center；dashboard 承接未读摘要 | 通知和 replay 基础存在 | 骑手分类和关键财务/风险通知契约需补齐；新单实时提醒不应污染长通知 | `BE-03`、`WEAPP-07` |

## 4. 后端契约清单

### 4.1 已有骑手基础与履约接口

- `GET /v1/rider/me`：骑手资料、押金、累计单量、累计收入、在线状态。
- `GET /v1/rider/status`：在线状态、当前活跃代取、上线/下线能力和阻塞原因。
- `POST /v1/rider/online`、`POST /v1/rider/offline`：上线和下线。
- `POST /v1/rider/location`：骑手定位上报。
- `GET /v1/delivery/recommend`：推荐/可抢订单。
- `POST /v1/delivery/grab/:order_id`：抢单。
- `GET /v1/delivery/active`：当前活跃代取。
- `GET /v1/delivery/history`：历史代取。
- `POST /v1/delivery/:delivery_id/start-pickup`、`confirm-pickup`、`start-delivery`、`confirm-delivery`：履约状态动作。

### 4.2 已有押金接口

- `GET /v1/rider/deposit`：押金总额、代取冻结、提现处理中、可用押金。
- `GET /v1/rider/deposits`：押金流水。
- `POST /v1/rider/deposit`：押金充值支付单。
- `POST /v1/rider/withdraw`：押金提现/退款申请，可能返回 `202 processing`。
- `GET /v1/rider/withdrawals/status`：押金退款状态回查。

### 4.3 已有追偿与申诉接口

- `GET /v1/rider/claims`：骑手相关索赔/追偿列表。
- `GET /v1/rider/claims/summary`：骑手追偿摘要。
- `GET /v1/rider/claims/:id`：索赔详情。
- `GET /v1/rider/claims/:id/decision`：责任判定依据。
- `GET /v1/rider/claims/behavior-summary`：行为回溯摘要。
- `GET /v1/rider/recoveries/:id`：追偿单详情。
- `POST /v1/rider/recoveries/:id/pay`：支付追偿。
- `POST /v1/rider/recovery-disputes`：提交争议。
- `GET /v1/rider/recovery-disputes`、`GET /v1/rider/recovery-disputes/:id`：争议列表与详情。

当前契约漂移：

- 后端真实追偿详情/支付路由使用 `recovery_id`：`/v1/rider/recoveries/:id`、`/v1/rider/recoveries/:id/pay`。
- 小程序 `appeals-customer-service.ts` 当前以 `claim_id` 调用 `/v1/rider/claims/:id/recovery` 和 `/v1/rider/claims/:id/recovery/pay`。
- `ClaimResponse` 当前只暴露 `recovery_status`，未暴露 `recovery_id`。
- 后续修复不能在小程序里猜测 `claim_id == recovery_id`；应由后端读模型暴露 `recovery_id`，或由后端提供明确的 claim-scoped recovery read model。
- 该漂移归入 `BE-02`/`WEAPP-06` 前置修复，避免追偿详情和支付入口继续不可达。

### 4.4 收入分账读模型缺口

已有 SQL 基础：

- `GetRiderProfitSharingStats`
- `ListRiderProfitSharingOrders`
- `GetRiderDailyIncome`

当前缺口：

- 缺少 rider self-facing API。
- 缺少 rider ledger 总数查询。
- 缺少按分账状态筛选的 rider ledger 查询。
- 缺少按状态聚合的 rider summary，例如待结算、处理中、失败、已到账。
- 现有历史代取接口的 `rider_earnings` 不能作为到账真值。

### 4.5 工作台聚合读模型缺口

当前 dashboard 需要组合骑手状态、订单池、当前任务、押金、追偿、通知和收入摘要。若全部由小程序首屏多接口并发拼装，会扩大首屏请求预算和局部失败复杂度。

建议新增 `GET /v1/rider/workbench/summary`，但该聚合层只组合子域结果，不复制子域业务规则。

## 5. 小程序承载清单

| 页面 | 当前职责 | 目标职责 | 边界 |
| :--- | :--- | :--- | :--- |
| `pages/rider/dashboard` | 工作台、在线开关、订单池、当前任务、快捷入口 | 经营工作台：摘要、当前任务、抢单大厅、高频风险入口 | 不承接账本列表，不扩张成所有能力大页 |
| `pages/rider/task-detail` | 单任务详情与履约动作 | 单任务履约 owner | 不处理分账、押金、追偿规则 |
| `pages/rider/navigation` | 导航与定位辅助 | 导航和定位恢复 owner | 不承担订单状态决策 |
| `pages/rider/tasks` | 历史任务列表 | 历史履约记录 | 不作为分账到账账本 |
| `pages/rider/income` | 不存在 | 代取费分账账本 | 不做余额和提现 |
| `pages/rider/deposit` | 押金余额、充值、押金退款 | 押金保障 owner | 不混入代取收入 |
| `pages/rider/claims` | 索赔/追偿/申诉 | 自助处理追偿与申诉 | 不扩展平台客服工单 |
| `pages/rider/notifications` | 不存在 | 骑手关键通知中心 | 新单实时提醒不默认变成长通知 |

## 6. 组件边界建议

| 组件 | 页面 | 输入 | 输出事件 | 是否共享 |
| :--- | :--- | :--- | :--- | :--- |
| `rider-workbench-summary` | dashboard | workbench summary view model | `tapIncome`、`tapDeposit`、`tapClaims` | 可共享，需 policy |
| `rider-current-delivery-card` | dashboard/task-detail | current delivery view model | `tapDetail`、`tapNavigate` | 可共享，需 policy |
| `rider-order-pool-list` | dashboard | recommended order list | `grab`、`openLocation`、`refresh` | 页面组内优先 |
| `rider-income-snapshot` | dashboard/income | income summary | `tapLedger` | 可共享，需 policy |
| `rider-settlement-ledger-list` | income | ledger rows, paging state | `loadMore`、`refresh` | income 页面内优先 |
| `rider-deposit-risk-summary` | dashboard/deposit | deposit summary | `tapDeposit` | 可共享，需 policy |
| `rider-claim-action-summary` | dashboard/claims | claim summary | `tapClaims` | 可共享，需 policy |
| `rider-critical-notice-list` | dashboard/notifications | notification summary | `tapNotice`、`markRead` | 可共享，需 policy |

组件规则：

- 展示组件不得直接请求接口、跳转路由或持有整页编排。
- 领域组件可以持有局部展示状态，但不吸收页面级 orchestration。
- 新增共享组件必须补 `component-policy.json`。
- 优先 TDesign Miniprogram，不新增 notice/card/panel 类本地视觉壳。

## 7. Stage 1 输入条件

Stage 0 完成后，Stage 1 可以从 `BE-01` 开始，先补齐骑手收入分账读模型。

进入条件：

- 能力组 owner 已冻结。
- income 不再与 delivery history 混用。
- income 不引入钱包或提现。
- profit sharing 不耦合微信同城代取上报状态。

Stage 1 后端预计需要：

- 扩展 `profit_sharing_order.sql` 的 rider 查询能力。
- 新增 logic 层 rider income read model。
- 新增 API handler 与 route。
- 运行 `make sqlc`、`make swagger` 和 focused tests。

## 8. Stage 0 Review Checklist

- [x] 能力组是否按骑手任务组织，而不是按接口拆页。
- [x] 分账、微信同城代取上报、押金、追偿边界是否保持低耦合。
- [x] 小程序页面 owner 是否唯一且不大而全。
- [x] 是否明确了收入账本不是代取历史，也不是钱包。
- [x] 是否明确实时消息不是唯一事实源。
- [x] 是否列出后续实现所需的真实后端缺口。

## 9. Stage 0 Review Result

Review result: pass after documenting one existing contract drift.

Finding recorded:

- 小程序骑手追偿详情/支付仍按 `claim_id` 调用旧路径，但后端、Casbin、Swagger 和集成测试的真实契约是 `recovery_id` 路径。Stage 0 不直接修复该路径，因为页面缺少 `recovery_id` 真值；后续实现必须先补后端读模型或明确 claim-scoped recovery 契约，再修小程序调用。

No Stage 0 document blockers remain.