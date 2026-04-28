# 骑手能力组整合计划与任务卡

Date: 2026-04-28
Risk class: `G2`
Status: draft for implementation planning

## 1. 目标

这份文档把骑手侧能力优化收敛成一组高内聚、低耦合的能力组与任务卡。

本计划不按接口数量拆页面，也不把后端子域边界揉进同一个大流程。目标是让骑手作为自主经营取送业务的个人承包商，在小程序里拥有清晰的日常经营闭环：接单、履约、收入分账、押金风险、追偿申诉、通知恢复。

## 2. 已确认业务边界

### 2.1 骑手经营模型

- 骑手不是平台雇佣员工，而是自主经营取送业务的个人承包商。
- 平台不做传统派单、排班、拒单处罚或过程型异常托管。
- 骑手侧能力应服务自主接单、自主履约、自助查看资金与风险事项。

### 2.2 收入与分账模型

- 骑手配送费通过微信分账到骑手个人 `openid`，不是平台钱包余额。
- 骑手侧不建设收入提现能力。
- 骑手侧需要的是微信分账结算账本：每单配送费、分账状态、到账时间、失败原因或平台处理中状态。

### 2.3 微信同城配送与分账边界

- 微信同城配送订单上报是否成功，属于微信同城配送/订单上报子域自己的职责。
- 分账不主动耦合同城配送上报状态、冻结期计算或微信确认收货过程。
- 分账域只消费支付/微信事实域已经产出的可分账事实或分账触发事件。
- 骑手侧页面只展示骑手能理解和行动的结算状态，不暴露同城配送上报内部状态。

### 2.4 消费者保护与索赔边界

- 消费者保护发生在确认收货、微信冻结、平台售后时限等前置窗口内。
- 分账完成后，普通消费者退款和普通索赔应视为资金终态后的关闭事项。
- 骑手侧只需要承接与自身相关的追偿、申诉、待处理风险，不扩展成平台客服工单。

## 3. 当前代码事实

### 3.1 后端事实

- 骑手基础资料、上线、押金、押金退款入口在 `locallife/api/rider.go`。
- 骑手配送历史与收入统计当前主要来自 delivery 维度，在 `locallife/api/delivery.go`。
- 骑手个人分账接收方使用 `PERSONAL_OPENID`，由 `locallife/logic/profit_sharing_receiver_sync_service.go` 维护。
- 骑手分账金额、订单号、商户名、日收入统计已经有 SQL 查询基础，见 `locallife/db/query/profit_sharing_order.sql`。
- 通知持久化与 WebSocket replay 基础已经存在，见 `locallife/api/notification.go`、`locallife/db/query/notification.sql`、`locallife/websocket/message_store_redis.go`。
- 骑手追偿与申诉链路已有 rider 视角查询和支付能力，见 `locallife/db/query/claim_recovery.sql`、`locallife/db/query/recovery_dispute.sql`、`locallife/logic/claim_recovery_payment.go`。

### 3.2 小程序事实

- 骑手页面组已有：`weapp/miniprogram/pages/rider/dashboard`、`tasks`、`task-detail`、`navigation`、`deposit`、`claims`。
- 押金页已经抽出服务 owner：`weapp/miniprogram/services/rider-deposit-payment.ts`、`rider-deposit-finance.ts`、`rider-deposit-withdrawal.ts`。
- 当前 rider 共享领域组件目录为空，后续若在 `weapp/miniprogram/components/` 下新增共享组件，必须同步提供 `component-policy.json`。
- 小程序交付必须按能力组合优先，不允许按“一个接口一个页面”机械落地。

## 4. 架构原则

### 4.1 能力组优先

能力组是交付单元。一个能力组应同时说明：

- 同步读模型。
- 异步事件或结果来源。
- 页面恢复方式。
- 领域组件边界。
- 页面或页面组承载。

### 4.2 聚合层只组合，不篡改业务规则

骑手工作台可以聚合订单、收入、押金、追偿和通知摘要，但不得把这些子域的业务规则复制到页面或聚合层。

工作台聚合层负责：

- 读取多个子域的当前状态。
- 转换为骑手首屏需要的摘要。
- 做局部降级和状态承接。

工作台聚合层不负责：

- 判定微信同城配送是否上报成功。
- 判定订单是否可分账。
- 执行分账或退款。
- 改写押金冻结、追偿、申诉状态机。

### 4.3 小程序自组件到页面

小程序实现顺序必须是：

1. 盘点真实后端字段、状态、动作和异步结果。
2. 按骑手日常任务分成能力组。
3. 为能力组定义领域 view model 和组件边界。
4. 判断组件落在已有页面、独立页面还是页面组。
5. 最后选择 TDesign 组件与页面结构。

不得先按接口生成页面，也不得把所有接口堆进 rider dashboard。

### 4.4 非顾客侧视觉标准

骑手侧属于非顾客侧工具页面，默认采用克制、稳定、TDesign-first 的工具型视觉语言。

实现时必须避免：

- 顶部解释性大卡片。
- 全宽长期 notice bar。
- 按钮墙、入口墙、字段面板。
- 顾客侧品牌视觉向骑手工具页渗透。
- 为每个小块额外包本地视觉壳。

## 5. 能力组设计

## 5.1 能力组 A：骑手经营工作台

目标：让骑手一进来就知道能不能接单、手上有没有任务、今天经营状态如何、是否有待处理风险。

同步能力：

- 骑手基础状态。
- 在线状态与上线阻塞原因。
- 当前进行中配送。
- 附近可抢订单数量或推荐订单摘要。
- 今日完成单数。
- 今日配送费摘要，按分账账本口径优先。
- 押金可用/冻结摘要。
- 待处理追偿或申诉数量。
- 未读关键通知数量。

异步能力：

- 新订单实时提醒。
- 订单被抢走、取消或状态变更。
- 定位同步状态。
- 分账到账或失败通知。
- 押金冻结/释放通知。
- 追偿待处理通知。

小程序承载：

- 页面 owner：`weapp/miniprogram/pages/rider/dashboard`。
- 组件候选：经营摘要、当前任务、抢单大厅、风险待处理、收入摘要。
- dashboard 只做首屏经营入口，不扩张成全部历史明细页。

## 5.2 能力组 B：接单与履约

目标：支持骑手自主发现订单、抢单、履约、导航和查看历史任务。

同步能力：

- 推荐订单/附近订单。
- 当前任务详情。
- 履约状态流转动作。
- 历史配送列表。

异步能力：

- 新单进入订单池。
- 订单从订单池消失。
- 手上任务状态变化。
- 定位上报结果。

小程序承载：

- `dashboard` 承接抢单大厅与当前任务摘要。
- `task-detail` 承接单任务履约动作。
- `navigation` 承接导航与定位辅助。
- `tasks` 承接历史任务，不承担收入分账账本职责。

边界：

- 不建设平台派单。
- 不把配送历史等同于收入账本。
- 不在履约页面处理分账规则。

## 5.3 能力组 C：配送费分账账本

目标：让骑手看到配送费如何从完成任务走到微信分账到账。

同步能力：

- 收入摘要：今日已到账、待结算、分账失败或处理中数量。
- 分账明细：订单号、商户、配送费、骑手分账金额、状态、完成时间、到账时间。
- 日收入趋势。

异步能力：

- 分账成功通知。
- 分账失败通知。
- 需平台处理通知。

小程序承载：

- 新增 rider income 页面组或在 `tasks` 页面内增加收入 tab，需要先完成页面边界评估。
- dashboard 只展示收入摘要和异常入口。
- 历史任务仍展示履约结果，不负责解释微信分账状态。

边界：

- 不做平台余额。
- 不做骑手收入提现。
- 不展示微信同城配送上报内部状态。
- 不在前端猜测分账完成时间。

## 5.4 能力组 D：押金与风险保障

目标：让骑手理解押金可用、冻结、提现处理中和释放条件。

同步能力：

- 押金余额。
- 配送冻结金额。
- 提现处理中金额。
- 押金流水。
- 充值和押金提现工作流。

异步能力：

- 充值支付结果。
- 押金提现/退款结果。
- 冻结释放。

小程序承载：

- 页面 owner：`weapp/miniprogram/pages/rider/deposit`。
- 现有 workflow owner 继续保留。
- dashboard 只展示押金摘要和阻塞原因入口。

边界：

- 押金提现不是配送收入提现。
- 不把押金页升级成骑手资金全账户。

## 5.5 能力组 E：追偿与申诉自助处理

目标：让骑手能自助处理与自己相关的索赔追偿和申诉，不扩展为平台客服工单。

同步能力：

- 待处理追偿列表。
- 追偿详情。
- 支付追偿。
- 提交申诉。
- 查看申诉结果。

异步能力：

- 新追偿通知。
- 申诉审核结果通知。
- 逾期状态变化通知。

小程序承载：

- 页面 owner：`weapp/miniprogram/pages/rider/claims`。
- dashboard 展示待处理摘要和入口。
- claim detail 只承接当前追偿/申诉，不跳成运营处理页。

边界：

- 不建设平台介入式异常管理。
- 不让骑手页面承担消费者索赔受理时限规则。

## 5.6 能力组 F：关键通知与状态恢复

目标：让骑手在弱网、断线重连、后台返回时仍能恢复到可信状态。

同步能力：

- 通知列表。
- 未读数量。
- 关键通知分类。
- 当前业务状态回读。

异步能力：

- WebSocket 新单、订单状态、资金和风险事件。
- Redis replay。
- DB notification inbox。

小程序承载：

- dashboard 展示关键未读摘要。
- 可单独建设 rider notification center，也可复用已有通用通知页，但必须保持骑手业务分类。
- 关键财务和追偿结果必须有持久通知或可回读账本兜底。

边界：

- 实时消息只做提醒，不作为唯一事实源。
- 页面恢复以当前状态接口和持久通知为准。

## 6. 页面与组件承载建议

### 6.1 页面组

| 页面组 | 主任务 | 能力组 | 备注 |
| :--- | :--- | :--- | :--- |
| `rider/dashboard` | 日常经营入口 | A/B/C/D/E/F 摘要 | 只承接摘要和高频动作入口 |
| `rider/task-detail` | 单任务履约 | B | 不承接分账规则 |
| `rider/navigation` | 导航与定位辅助 | B | 不承担订单状态解释 |
| `rider/tasks` | 历史履约记录 | B | 不再作为收入真值页 |
| `rider/income` | 配送费分账账本 | C | 建议新增，或经评估后作为 `tasks` 的独立 tab |
| `rider/deposit` | 押金账户与押金退款 | D | 不做配送收入提现 |
| `rider/claims` | 追偿与申诉 | E | 自助处理，不扩展工单 |
| `rider/notifications` | 关键通知中心 | F | 可复用通用通知能力，但需骑手分类 |

### 6.2 组件候选

组件不按接口拆，按能力组和局部状态拆。

| 组件候选 | 所属能力组 | 复用范围 | TDesign 组别 |
| :--- | :--- | :--- | :--- |
| `rider-workbench-summary` | A | dashboard | data |
| `rider-current-delivery-card` | A/B | dashboard、task detail 可复用摘要 | data |
| `rider-order-pool-list` | B | dashboard | data |
| `rider-income-snapshot` | C | dashboard、income | data |
| `rider-settlement-ledger-list` | C | income | data |
| `rider-deposit-risk-summary` | D | dashboard、deposit | data |
| `rider-claim-action-summary` | E | dashboard、claims | data |
| `rider-critical-notice-list` | F | dashboard、notifications | feedback/data |

若组件放入 `weapp/miniprogram/components/`，必须为每个共享组件增加 `component-policy.json`，说明：

- `purpose`
- `tdesignGroup`
- `tdesignCandidates`
- `decision`
- `rationale`

若组件只服务单页，优先放在页面组内部或保留为页面局部区域，避免过早抽共享。

## 7. 实施阶段

## Stage 0：契约盘点与边界冻结

目标：冻结骑手能力组、后端真值和小程序承载边界。

输出：

- 骑手能力组清单。
- 后端字段/状态/动作矩阵。
- 小程序页面组与组件边界草图。
- 明确哪些后端缺口必须补，哪些只做前端整合。

## Stage 1：骑手收入分账账本

目标：补齐骑手侧最关键的经营信息缺口，不引入钱包和提现。

输出：

- 后端 rider income 读模型。
- 小程序收入摘要组件。
- 小程序收入账本页面或明确的独立 tab。
- 分账成功/失败状态承接。

## Stage 2：骑手工作台能力组重排

目标：把 dashboard 从接口集合重排成经营工作台。

输出：

- 经营摘要。
- 当前任务摘要。
- 抢单大厅。
- 收入摘要。
- 押金/追偿风险摘要。
- 关键通知摘要。

## Stage 3：押金、追偿、通知联动收口

目标：不扩展新边界，只把已有押金、追偿和通知变成骑手能自助处理的闭环。

输出：

- 押金风险摘要接入 dashboard。
- 追偿待处理摘要接入 dashboard。
- 骑手关键通知分类与恢复策略。

## Stage 4：验证、回归与文档同步

目标：完成高风险路径验证和交付证据。

输出：

- 后端 focused tests。
- 小程序 `npm run quality:check`。
- 弱网、重入、重复点击、WebSocket 重连、账本回读验证记录。

## 8. 任务卡

## S0-01 能力契约矩阵

目标：建立 rider 侧能力组到后端真值的矩阵。

Owner：architecture / backend / weapp jointly

范围：

- 盘点 rider dashboard、tasks、deposit、claims 当前使用的字段和接口。
- 盘点后端已有 rider income SQL、claim recovery、notification、deposit 能力。
- 标注每个能力组的同步读模型、异步事件、恢复方式。

不做：

- 不新增接口。
- 不改页面。
- 不把微信同城配送上报纳入分账或 rider 页面契约。

验收：

- 每个能力组都有唯一 owner。
- 每个页面只承接明确任务域。
- 未确认后端缺口单独列出，不由小程序猜测补齐。

## BE-01 骑手收入分账读模型 API

目标：提供骑手配送费分账账本读模型。

Owner：backend rider income read model

建议接口：

- `GET /v1/rider/income/summary`
- `GET /v1/rider/income/ledger`
- `GET /v1/rider/income/daily`

输入：

- 时间范围。
- 分页参数。
- 可选分账状态筛选。

输出：

- 今日/区间已到账配送费。
- 待结算或处理中金额。
- 分账失败数量。
- 明细：订单号、商户名、配送费、骑手分账金额、状态、完成时间、到账时间。

不做：

- 不新增骑手钱包余额。
- 不新增骑手收入提现。
- 不读取或判断微信同城配送上报状态。
- 不把 delivery history 统计直接当作分账到账统计。

验收：

- 使用服务端认证 rider 身份，不信任客户端 rider_id。
- 数据来源以分账订单读模型为主。
- 空状态返回稳定空列表和 0 统计。
- 分页语义稳定。
- Swagger、sqlc 和测试按后端规则同步。

验证：

- `make sqlc`，如新增或修改 SQL。
- `make swagger`，如新增接口注解。
- `go test ./api -run 'Test.*RiderIncome'`
- `go test ./logic -run 'Test.*RiderIncome'`

## BE-02 骑手工作台摘要读模型

目标：给小程序 dashboard 提供高频首屏摘要，避免前端首屏请求爆炸。

Owner：backend rider workbench read model

建议接口：

- `GET /v1/rider/workbench/summary`

输出候选：

- 骑手在线状态和上线阻塞原因。
- 当前任务摘要。
- 可抢订单数量或推荐订单摘要。
- 今日完成单数。
- 今日配送费分账摘要。
- 押金可用/冻结摘要。
- 待处理追偿数量。
- 关键未读通知数量。

边界：

- 聚合层只组合各子域读模型，不复制子域业务规则。
- 子域失败允许局部降级，响应需能表达哪个区块不可用。
- 不把分账准入、同城配送上报、押金冻结规则写进聚合层。

验收：

- 首屏请求数量下降，dashboard 不再靠多个低价值请求拼首屏。
- 单个子域失败不会让整页白屏。
- 响应字段直接服务骑手首屏，不暴露内部 provider 字段。

验证：

- `go test ./api -run 'Test.*RiderWorkbench'`
- `go test ./logic -run 'Test.*RiderWorkbench'`

## BE-03 骑手关键通知分类补齐

目标：把骑手必须感知的资金、押金、追偿事件纳入可持久恢复的通知分类。

Owner：backend notification producer / rider notification read model

事件范围：

- 分账成功。
- 分账失败或需平台处理。
- 押金冻结或释放。
- 押金提现/退款结果。
- 新追偿待处理。
- 申诉结果。

边界：

- 实时 WebSocket 只做提醒。
- 持久通知或业务账本才是恢复真值。
- 不把新单实时提醒全部沉淀成长通知，避免通知中心被订单池事件污染。

验收：

- 关键财务与风险事件有持久可查路径。
- dashboard 未读摘要和通知中心能读取同一分类。
- 用户偏好不能屏蔽法律、资金、风险必须送达的通知，若已有偏好系统需显式例外。

验证：

- `go test ./worker -run 'Test.*Rider.*Notification'`
- `go test ./api -run 'Test.*Notification'`

## WEAPP-01 骑手页面组能力边界重排

目标：按能力组重排 rider 页面组，不按接口铺页面。

Owner：weapp rider page group

范围：

- 为 dashboard、tasks、task-detail、navigation、income、deposit、claims、notifications 定义主任务。
- 明确每个页面的首屏关键数据、失败承接、回读策略。
- 决定 income 是独立页面还是 tasks 内独立 tab。

不做：

- 不改视觉细节。
- 不新建本地假字段。
- 不把低频所有能力塞入 dashboard。

验收：

- 每个页面只有一个主任务。
- dashboard 只展示摘要、当前任务和高频入口。
- 历史任务不再承担分账到账解释。

## WEAPP-02 骑手收入领域组件与页面

目标：承接骑手配送费分账账本。

Owner：weapp rider income capability

组件候选：

- `rider-income-snapshot`
- `rider-settlement-ledger-list`
- `rider-settlement-status-filter`

页面候选：

- `weapp/miniprogram/pages/rider/income/index`

状态：

- 首屏加载。
- 首屏空态。
- 首屏失败。
- 局部刷新失败保留上次可信数据。
- 加载更多失败。

文案边界：

- 使用“配送费结算”“微信分账”“已到账”。
- 不使用“余额”“可提现”“钱包”。
- 不暴露 provider 原始错误。

验收：

- 页面字段全部来自 BE-01。
- dashboard 只使用 snapshot，不复制账本列表逻辑。
- 若新增共享组件，补齐 `component-policy.json`。
- 不新增顶部解释性大卡片或全宽长期 notice bar。

验证：

- `npm run compile`
- `npm run quality:check`

## WEAPP-03 骑手工作台组件化改造

目标：把 dashboard 从接口和大块 WXML 堆叠，收敛为能力组组件组合。

Owner：weapp rider dashboard

组件候选：

- `rider-workbench-summary`
- `rider-current-delivery-card`
- `rider-order-pool-list`
- `rider-income-snapshot`
- `rider-deposit-risk-summary`
- `rider-claim-action-summary`

实现规则：

- 页面负责加载、刷新、导航、动作转发和状态承接。
- 组件负责展示局部 view model 与发出语义事件。
- 组件不得直接请求接口、跳路由或持有整页编排。
- 组件优先 TDesign 表达，不新增本地视觉壳。

验收：

- dashboard 首屏信息密度降低但关键状态更清楚。
- 新订单实时提醒不污染持久通知中心。
- 当前任务优先级高于抢单大厅。
- 收入、押金、追偿只展示摘要和入口。
- 页面 TS/WXML/WXSS 不继续膨胀到门禁风险区。

验证：

- `npm run compile`
- `npm run quality:check`

## WEAPP-04 接单与履约状态恢复

目标：强化自主接单与履约页面在弱网、断线、重入时的可信恢复。

Owner：weapp rider task workflow

范围：

- dashboard 订单池刷新和 WebSocket replay 后的回读策略。
- task-detail 状态动作防重入。
- navigation 定位状态提示与重试。
- history tasks 分页与刷新失败保留可信数据。

不做：

- 不新增派单。
- 不新增平台调度操作。
- 不在前端本地决定订单池真实状态。

验收：

- 新单、订单消失、当前任务变化都能通过回读恢复。
- 重复点击抢单或履约动作不会重复提交。
- 首屏失败和局部刷新失败分开承接。

验证：

- `npm run compile`
- `npm run quality:check`
- 手工验证断线重连、后台返回、重复点击。

## WEAPP-05 押金风险摘要接入工作台

目标：让 dashboard 展示押金是否影响上线和接单，不把押金页变成收入钱包。

Owner：weapp rider deposit summary

范围：

- 复用 `rider-deposit-finance` view model。
- dashboard 展示可用押金、冻结、提现处理中和上线阻塞入口。
- 点击进入 `rider/deposit` 处理充值或押金提现。

不做：

- 不重复实现押金支付 workflow。
- 不展示配送收入提现入口。

验收：

- 押金阻塞上线时，入口和原因清晰。
- 非阻塞时不占据首屏主位。
- 文案不混淆押金和配送费收入。

验证：

- `npm run compile`
- `npm run quality:check`

## WEAPP-06 追偿与申诉摘要接入工作台

目标：把骑手待处理追偿和申诉结果变成可发现的自助事项。

Owner：weapp rider claim action summary

范围：

- dashboard 展示待处理追偿数量和最近一条行动提示。
- `claims` 页面继续承接列表、详情、支付追偿和申诉。
- 追偿支付或申诉结果后返回 dashboard 能刷新摘要。

不做：

- 不新增平台客服工单。
- 不把消费者索赔受理规则放到骑手页面。

验收：

- 待处理事项不埋在历史列表里。
- 已处理、争议中、关闭状态文案清楚。
- 页面返回后状态可恢复。

验证：

- `npm run compile`
- `npm run quality:check`

## WEAPP-07 骑手关键通知中心

目标：为骑手资金与风险通知提供统一可恢复入口。

Owner：weapp rider notification center

范围：

- 评估复用现有通知 API 和页面的方式。
- 增加骑手分类筛选：订单、收入、押金、追偿。
- dashboard 展示未读关键通知摘要。

边界：

- 新单实时提醒仍以 dashboard 订单池为主，不默认进入长通知列表。
- 财务和追偿事件必须能从通知或业务账本恢复。

验收：

- 用户重进页面后能看到未读关键通知。
- 通知点击能跳到对应业务页。
- 原始后端错误或 provider 文本不外露。

验证：

- `npm run compile`
- `npm run quality:check`

## QA-01 骑手主链路回归

目标：覆盖骑手日常经营闭环。

手工验证：

- 上线和下线。
- 抢单大厅刷新。
- 抢单成功和抢单失败。
- 当前任务进入详情。
- 履约状态流转。
- 导航与定位权限恢复。
- 历史任务分页。
- 收入账本空态、列表、筛选、加载更多。
- 押金充值、押金提现处理中、押金不足阻塞上线。
- 追偿待处理、支付、申诉、申诉结果。
- 通知未读、已读、跳转。

自动验证：

- 后端 focused tests。
- 小程序 `npm run quality:check`。

## QA-02 弱网与重入验证

目标：验证高风险状态恢复。

场景：

- dashboard 首屏部分区块失败。
- WebSocket 断线后重连。
- 抢单后立即后台再返回。
- 履约动作提交后网络超时。
- 支付追偿后结果未知。
- 收入账本加载更多失败。
- 押金提现返回处理中。

验收：

- 不出现 Toast-only 首屏失败。
- 不用空数组或 0 伪装真实结果。
- 用户能继续回查或重试。
- 页面不重复提交高风险动作。

## 9. 验证命令基线

后端：

- `make sqlc`
- `make swagger`
- `make check-generated`
- `go test ./api -run 'Test.*RiderIncome|Test.*RiderWorkbench'`
- `go test ./logic -run 'Test.*RiderIncome|Test.*RiderWorkbench'`

小程序：

- `npm run compile`
- `npm run quality:check`

## 10. 明确不做

- 不做平台派单。
- 不做骑手排班。
- 不做骑手收入提现钱包。
- 不把微信同城配送上报状态耦合到 profit sharing 或 rider 页面。
- 不在骑手页面实现消费者索赔受理时限判断。
- 不把 dashboard 扩张成所有 rider 能力的大而全页面。
- 不新增顶部解释性大卡片、全宽长期提示条或说明型入口墙。

## 11. 开始实施前的阻塞项

- 需要确认 `profit_sharing_orders` 当前 rider 状态枚举是否足够表达前端账本状态。
- 需要确认是否已有通用通知页面能按骑手分类复用。
- 需要确认 dashboard 首屏是否新增 `workbench summary` 聚合接口，还是先用前端现有多接口组合过渡。
- 需要确认 income 独立页面是否加入 rider 分包路由。
- 需要确认所有新增共享组件的 `component-policy.json` 维护责任。

## 12. Stage 1 BE-01 Implementation Review

Date: 2026-04-28
Status: pass with unrelated full-api test failures recorded

Implemented:

- Added rider income read-model routes:
	- `GET /v1/rider/income/summary`
	- `GET /v1/rider/income/ledger`
	- `GET /v1/rider/income/daily`
- Extended `profit_sharing_orders` rider queries with optional status filter, ledger count, status summary, and stable ordering by `created_at DESC, id DESC`.
- Added `logic.RiderIncomeService` to keep handler logic limited to auth, query parsing, and response mapping.
- Added focused API tests for summary, ledger pagination/filtering, daily income, auth, invalid status/date, and missing rider profile.
- Regenerated sqlc, mocks, and Swagger artifacts.

Review checklist:

- [x] Current rider identity is resolved from the authenticated user; no client-supplied `rider_id` is trusted.
- [x] Data source remains `profit_sharing_orders`; delivery history is not used as settlement truth.
- [x] No rider wallet, income withdrawal, platform dispatch, or WeChat same-city reporting coupling was introduced.
- [x] SQL uses explicit columns, optional status filter, count query, status summary, and stable pagination ordering.
- [x] Empty status buckets return stable zero-count summaries for pending, processing, finished, and failed.
- [x] Swagger, sqlc, and mocks are synchronized.

Validation:

- `make sqlc`: pass
- `make swagger`: pass
- `make check-generated`: pass
- `go test ./logic ./api -run 'Test(GetRiderIncomeSummaryAPI|ListRiderIncomeLedgerAPI|GetRiderIncomeDailyAPI)' -count=1`: pass
- `go test ./api -count=1`: failed in pre-existing payment query tests unrelated to rider income:
	- `TestQueryPaymentOrderAPI_ServiceUnavailableWhenEcommerceClientMissing`
	- `TestQueryPaymentOrderAPI_DirectPaymentReturnsClearError`

No BE-01 blockers remain. Next implementation stage can proceed to `BE-02` or `WEAPP-02` depending on whether backend workbench aggregation or the income page should be prioritized first.

## 13. Stage 1 WEAPP-02 Implementation Review

Date: 2026-04-28
Status: pass

Implemented:

- Added rider income task-domain API owner: `weapp/miniprogram/api/rider-income.ts`.
- Added rider income view-model service: `weapp/miniprogram/services/rider-income.ts`.
- Added rider income page route under the rider subpackage: `pages/rider/income/index`.
- Built the rider income page with:
	- Date-range based summary from `GET /v1/rider/income/summary`.
	- Recent daily income preview from `GET /v1/rider/income/daily`.
	- Status-filtered, paginated settlement ledger from `GET /v1/rider/income/ledger`.
	- First-screen loading, first-screen error, empty state, stale-data refresh failure state, and load-more failure retry.
- Added a dashboard shortcut to the income page without adding income ledger logic to dashboard.

Review checklist:

- [x] Page owner is `pages/rider/income`; dashboard only provides navigation and does not duplicate ledger state or list logic.
- [x] Fields and status semantics come from BE-01; no frontend-only settlement truth was invented.
- [x] Page copy uses “配送费结算”, “分账”, and “已到账”; no rider income wallet or withdrawal surface was introduced.
- [x] No platform dispatch, WeChat same-city reporting, freeze/unfreeze, or provider-internal status was exposed in the page.
- [x] Existing trusted data is preserved on silent refresh failure; first-screen failure and load-more failure have visible retry paths.
- [x] TDesign components are used for tabs, status tags, loading, empty state, icon, and buttons; no shared component was added, so no `component-policy.json` is required.

Validation:

- `npm run compile`: pass
- `npm run quality:check`: pass
- VS Code diagnostics on touched weapp files: pass

No WEAPP-02 blockers remain. The next stage can proceed to `BE-02` for workbench summary aggregation or `WEAPP-03` for dashboard capability composition, depending on whether backend aggregation or visible dashboard consolidation should come first.

## 14. Stage 2 BE-02 Implementation Review

Date: 2026-04-28
Status: pass

Implemented:

- Added rider workbench summary route: `GET /v1/rider/workbench/summary`.
- Added `logic.RiderWorkbenchService` as a read-only aggregation layer for rider status, active deliveries, order pool count, today's completed deliveries, today's profit sharing income, deposit availability, pending claims, and unread notifications.
- Added `CountRiderCompletedDeliveriesInRange` in `delivery.sql` and regenerated sqlc/mocks.
- Added section-level degradation so optional subdomain failures mark only the affected section unavailable while the workbench still returns available sections.
- Added focused logic and API tests for normal summary, rider-not-found terminal failure, and optional section degradation.
- Regenerated Swagger docs for the new response contract.

Review checklist:

- [x] Current rider identity is resolved from the authenticated user; no client-supplied `rider_id` is trusted.
- [x] Aggregation remains read-only and delegates business rules to existing domain read models/helpers.
- [x] Profit sharing remains the income truth; delivery history is used only for operational completed-count summary.
- [x] Deposit summary uses the existing deposit availability helper and exposes pending amount as `deposit_refund_processing_amount`, not rider income withdrawal.
- [x] No rider wallet, income withdrawal, platform dispatch, or WeChat same-city reporting/freeze/unfreeze coupling was introduced.
- [x] Single section failures degrade locally and do not white-screen the whole workbench response.
- [x] Handler stays transport-focused; business aggregation lives in `logic`.

Validation:

- `make sqlc`: pass
- `go test ./logic ./api -run 'Test.*RiderWorkbench' -count=1`: pass
- `make swagger`: pass
- `make check-generated`: pass
- VS Code diagnostics on touched BE-02 files: pass
- Boundary grep for workbench files: pass; remaining internal `withdrawal` reference is only the existing deposit-refund availability helper input.
- `git diff --check`: pass

No BE-02 blockers remain. The next stage can proceed to `WEAPP-03` to consume the workbench summary by capability group on the rider dashboard, while leaving the recorded `claim_id`/`recovery_id` drift for its own recovery contract task.

## 15. Stage 2 WEAPP-03 Implementation Review

Date: 2026-04-28
Status: pass after one review fix

Implemented:

- Added rider workbench API contract owner: `weapp/miniprogram/api/rider-workbench.ts`.
- Added rider workbench dashboard view-model builder: `weapp/miniprogram/services/rider-workbench.ts`.
- Updated `pages/rider/dashboard` to consume `GET /v1/rider/workbench/summary` for first-screen operating summary.
- Replaced the old top status card and full-width notice-bar pattern with:
	- first-screen loading/error states;
	- inline sync failure state;
	- compact operating metrics for today completed count, settled amount, and order-pool count;
	- risk snapshots for deposit, claims, pending/processing settlement, and unread notifications.
- Kept order grabbing, delivery actions, navigation, calls, WebSocket order-pool updates, and location sync on their existing detailed delivery APIs.
- Renamed the dashboard deposit entry from wallet wording to deposit wording and kept the income ledger as a separate page entry.

Review checklist:

- [x] Dashboard first-screen status comes from backend workbench summary; no client-side settlement, deposit, or claim truth was invented.
- [x] Summary remains read-only and does not introduce wallet, income withdrawal, platform dispatch, or WeChat same-city reporting/freeze/unfreeze coupling.
- [x] Current task action cards still rely on full active-delivery data; the workbench summary's compact delivery item is not used for navigation/contact actions.
- [x] Pull-to-refresh, retry, network restore, grab success, and delivery-action success now refresh workbench summary before refreshing task detail lists.
- [x] First-screen failure and local refresh failure are separated; trusted detailed task data is not replaced by incomplete summary data.
- [x] TDesign declarations were cleaned up after removing `t-notice-bar`, grid, and divider usage.
- [x] No shared component was added, so no `component-policy.json` is required in this stage.

Review fix:

- During review, the dashboard was found to temporarily map the workbench summary's compact current-delivery item into the old actionable current-task card. That summary item does not carry navigation coordinates or contact fields. The runtime now keeps actionable current-task cards sourced only from the existing active-delivery API, while the workbench summary contributes counts and risk/metric snapshots.

Validation:

- VS Code diagnostics on touched WEAPP-03 files: pass
- `npm run compile`: pass
- `npm run quality:check`: pass
- `git diff --check`: pass

No WEAPP-03 blockers remain. The next stage should stay within the documented task sequence and not start `BE-03` or `WEAPP-04` until explicitly selected.