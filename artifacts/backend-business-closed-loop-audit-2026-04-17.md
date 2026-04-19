# 后端业务闭环能力存在性与稳健性审查

## 1. 目标

这份文档只做一件事：

围绕 [artifacts/backend-business-closed-loop-map-2026-04-17.md](artifacts/backend-business-closed-loop-map-2026-04-17.md) 里的主链和五个侧环，确认：

1. 能力是否真实存在于代码里
2. 关键实现是否足够稳健
3. 是否看到明显的降级或漂移
4. 关键错误是否有日志落点

它不是新一轮规划文档，也不是控制面规格文档。

## 2. 审查结论

结论先说：

- 闭环图里的核心能力都能在当前后端代码中找到真实落点，不是空概念。
- 主链没有发现“关键节点根本不存在”的断裂。
- 稳健性最好的是支付成功处理、退款恢复、配送状态推进和微信回调校验这几条高风险链路。
- 最需要持续盯住的不是“能力缺失”，而是两类风险：
  - 某些能力的错误日志主要落在 API / worker 边界，而不是逻辑函数内部
  - 区域治理、供给治理这类组合能力更容易发生字段语义漂移，需要用这张闭环图反查，而不是继续追加新规格文档

整体判断：

- 能力存在性：通过
- 主链稳健性：基本通过
- 高风险链错误日志：基本通过
- 组合能力漂移风险：仍需靠后续 review 持续压住

## 3. 审查口径

每个能力按四个维度看：

- 存在性：是否有真实 handler / logic / tx / query 落点
- 稳健性：是否有幂等、事务、并发保护、状态冲突保护、所有权校验或恢复逻辑
- 漂移风险：是否容易出现“图上一个能力，代码里拆成几段后语义跑偏”
- 日志：关键失败是否在 API、worker、callback、recovery 边界有结构化日志

## 4. 主链审查

## 4.1 消费意图形成

### 存在性

存在。

主要落点：

- [locallife/logic/order_service.go](locallife/logic/order_service.go)
- [locallife/api/cart.go](locallife/api/cart.go)
- [locallife/api/payment_order.go](locallife/api/payment_order.go)

### 稳健性

基本够用。

在 [locallife/logic/order_service.go](locallife/logic/order_service.go) 里，下单前有商户状态、堂食桌台、预订状态、会员余额、优惠券、配送费、外卖限制等前置校验，说明“消费意图 -> 可创建交易”不是裸写订单。

### 漂移风险

中等。

消费意图相关对象分散在地址、购物车、会员、优惠券、预订等多个域里，最容易出现的不是链路缺失，而是某一处校验条件后移或重复实现。

### 日志

部分充足。

这一段更多依赖请求错误返回和 API 层统一错误包装；逻辑层自身日志不算重，但 API 层存在统一错误落点 [locallife/api/server.go](locallife/api/server.go)。

## 4.2 订单创建

### 存在性

存在。

主要落点：

- [locallife/logic/order_service.go](locallife/logic/order_service.go)
- [locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go)

### 稳健性

通过。

CreateOrder 路径已经不是简单写 order，包含订单金额计算、配送费、营销规则、会员余额和预订联动。事务层承担真实写边界，说明“交易锚点成立”这一步是被认真建模过的。

### 漂移风险

中等。

因为下单前置校验多，任何一段新规则如果绕开 OrderService 都可能造成语义漂移。这一段后续 review 时要继续盯“是否复用统一下单编排”。

### 日志

部分充足。

这一段主要通过统一请求错误和上层 API 错误落日志，不属于当前最依赖内部日志的风险段。

## 4.3 支付入账

### 存在性

存在，而且是明显的核心主链能力。

主要落点：

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go)
- [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)

### 稳健性

强。

能看到的稳健性证据包括：

- 微信回调体大小限制、签名校验、事件类型过滤、重复认领保护、stale claim 释放重试，见 [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- 支付成功处理走 [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go) 的事务与 processed_at 幂等保护
- 不同 business_type 明确分支处理，不是支付成功后一把梭
- 丢失关键关联对象时会显式报错并保持可见，而不是静默吞掉

### 漂移风险

低到中等。

当前 payment_order.business_type 是强约束锚点，能压住大部分语义漂移。但后续如果新增 business_type 分支，必须同步补交易后处理与监控，否则容易出现“支付已到账但业务未推进”的新漂移点。

### 日志

通过。

这一段是当前日志最完整的链之一：

- 回调验签失败、解析失败、重复处理、状态异常都有日志
- worker 告警、退款状态推进、部分退款判定也有结构化日志，见 [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- 统一服务端错误在 [locallife/api/server.go](locallife/api/server.go) 会落日志

## 4.4 履约交付

### 存在性

存在。

主要落点：

- [locallife/api/delivery.go](locallife/api/delivery.go)
- [locallife/db/sqlc/tx_delivery.go](locallife/db/sqlc/tx_delivery.go)
- [locallife/db/query/delivery_pool.sql](locallife/db/query/delivery_pool.sql)
- [locallife/db/query/delivery.sql](locallife/db/query/delivery.sql)

### 稳健性

强。

能看到的稳健性证据包括：

- 抢单事务里对 rider、delivery_pool、押金冻结做事务内检查，防并发抢单，见 [locallife/db/sqlc/tx_delivery.go](locallife/db/sqlc/tx_delivery.go)
- 配送状态推进不是单表更新，而是 delivery 和 order 的同步推进事务
- delivery_pool 有显式池子对象，不是拿订单状态硬猜“是否待接单”

### 漂移风险

中等。

这条链最容易发生的漂移不是主状态机断裂，而是“运营看见的阶段”和“事务真实推进的阶段”不一致。所以后续如果补调度聚合接口，必须严格复用现有 order / delivery / delivery_pool 三者语义。

### 日志

基本通过。

在 [locallife/api/delivery.go](locallife/api/delivery.go) 能看到估时失败、异步发货上报入队失败等关键节点日志；事务函数本身不重日志，但高风险错误主要在 API / worker 边界可见。

## 4.5 资金收口

### 存在性

存在。

主要落点：

- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)
- [locallife/worker/refund_recovery_scheduler.go](locallife/worker/refund_recovery_scheduler.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- [locallife/api/platform_stats.go](locallife/api/platform_stats.go)

### 稳健性

强。

能看到的稳健性证据包括：

- 退款创建使用事务防并发超退，见 [locallife/logic/refund_service.go](locallife/logic/refund_service.go)
- 部分退款不会错误终结 payment_order，逻辑里有累计退款额判断
- recovery scheduler 明确扫 stuck processing refunds 和未退款订单

### 漂移风险

中等。

这一段的风险主要不是缺实现，而是资金结果分散在 payment_order、refund_order、profit sharing、subsidy 等对象中，后续如果没有统一复盘口径，容易在运营视角上出现“财务真相”和“业务视角”漂移。

### 日志

通过。

退款、分账、补差、对账相关失败路径普遍能看到 log.Error 或 loggedServerError，尤其资金外部调用与 recovery 路径日志比较充分。

## 5. 五个侧环审查

## 5.1 商家供给闭环

### 存在性

存在。

主要落点：

- [locallife/api/merchant_application.go](locallife/api/merchant_application.go)
- [locallife/api/group.go](locallife/api/group.go)
- [locallife/api/merchant.go](locallife/api/merchant.go)

### 稳健性

基本通过。

商户申请链路已经包含 OCR、媒体、地区匹配、重复校验、审核事务和组织关系对象，不是一个只存草稿的空壳申请流。

### 漂移风险

中等偏高。

供给闭环天然跨 merchant_application、merchant、group、payment config、media、ocr 多条链，最容易因字段语义变动而漂移。闭环图保留下来是有价值的，因为它能反查“供给能力是否仍然一起工作”。

### 日志

通过。

商户申请链路的绑定错误、OCR 修复、重复校验、审核事务失败在 [locallife/api/merchant_application.go](locallife/api/merchant_application.go) 都有较多日志落点。

## 5.2 骑手运力闭环

### 存在性

存在。

主要落点：

- [locallife/db/sqlc/tx_delivery.go](locallife/db/sqlc/tx_delivery.go)
- [locallife/api/rider.go](locallife/api/rider.go)
- [locallife/api/delivery.go](locallife/api/delivery.go)

### 稳健性

基本通过。

运力闭环不是只有“骑手资料”，而是把押金、冻结、抢单资格、配送推进和收益释放串在一起。事务里可见押金冻结与订单状态同步，说明这条链是真闭环。

### 漂移风险

中等。

最容易漂移的是“骑手可接单资格”的判定来源。如果未来出现多个地方各自判断 can_grab_order，就会很快失真。

### 日志

基本通过。

API 边界、支付充值、异步任务失败存在日志，但“资格派生”本身更多靠状态一致性而不是单点日志。

## 5.3 平台资金闭环

### 存在性

存在。

主要落点：

- [locallife/api/platform_finance.go](locallife/api/platform_finance.go)
- [locallife/api/platform_stats.go](locallife/api/platform_stats.go)
- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)

### 稳健性

通过。

微信资金查询、退款、分账处理、平台汇总统计都能找到实代码，不是分析时虚构的“应该有”。

### 漂移风险

中等。

风险在于平台汇总视图和逐单资金对象天然分层，如果后续统计口径变化没有和底层对象一起 review，就会出现管理面漂移。

### 日志

通过。

[locallife/api/platform_finance.go](locallife/api/platform_finance.go) 采用 loggedServerError 处理上游错误；worker 和资金外部交互也普遍有结构化日志。

## 5.4 售后风控闭环

### 存在性

存在。

主要落点：

- [locallife/api/appeal.go](locallife/api/appeal.go)
- [locallife/api/claim_recovery.go](locallife/api/claim_recovery.go)
- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)

### 稳健性

基本通过。

claim、appeal、recovery 不是单表状态，而是多角色查询、支付、waive、审核串起来的闭环，说明“异常和损失变成可追偿对象”这条能力真实存在。

### 漂移风险

中等。

这条链最容易漂移的是角色口径和责任口径。如果 merchant、rider、operator 看的是不同语义切片，而不是同一 claim 真相，会逐步失真。

### 日志

基本通过。

关键失败在 API 边界可见，但相比支付回调和退款恢复，这一段的日志密度略低，更依赖请求错误和审计轨迹。

## 5.5 区域治理闭环

### 存在性

存在。

主要落点：

- [locallife/api/operator_rules.go](locallife/api/operator_rules.go)
- [locallife/api/operator_realtime.go](locallife/api/operator_realtime.go)
- [locallife/api/operator_stats.go](locallife/api/operator_stats.go)

### 稳健性

基本通过。

区域治理不是一个空“后台角色”，而是区域规则、实时统计、区域扩张、商户和骑手经营面共同构成的治理能力。

### 漂移风险

高于前几项。

原因不是实现弱，而是它天然是组合能力：规则、统计、供给、履约、财务都可能汇入区域视角。只要继续追加派生文档而不回到闭环图，就最容易把“区域治理”讲偏成单一后台模块。

### 日志

基本通过。

[locallife/api/operator_realtime.go](locallife/api/operator_realtime.go) 对并发子查询失败有显式日志；规则变更类接口也有 audit 落点，但这条链更依赖审计与统计一致性，而不只是错误日志。

## 6. 重点结论

## 6.1 这张闭环图值得保留

值得。

原因不是它“概念完整”，而是它能压住两个最容易反复出现的问题：

- 能力已经存在，但后续讨论把它讲散了
- 局部增强文档越来越多，最后反过来污染了对系统真相的理解

## 6.2 当前没有发现“能力根本不在”的大洞

这次按闭环图回看，主链和五个侧环都能找到真实代码落点。

所以当前更重要的不是继续发明能力，而是围绕这张图做针对性的 review。

## 6.3 最值得持续审查的三类问题

1. 新增改动是否绕开现有统一编排入口，例如 OrderService、payment success tx、delivery tx。
2. 组合能力是否出现语义漂移，尤其是区域治理、供给治理、售后责任口径。
3. 关键失败是否仍然在 API / worker / callback / recovery 边界落到结构化日志。

## 7. 后续用法

后续如果继续审代码，建议只围绕这张图做两种工作：

1. 按节点审查：逐个能力 review 稳健性与日志。
2. 按箭头审查：逐个回流点检查是否有降级、漂移或监控缺口。

不再继续扩写新的派生规划文档，否则很容易再次偏离系统真相。

## 8. 本轮正式审查 findings

以下 findings 基于对闭环图相关代码路径的进一步 review，按严重度排序。

### Finding 1：区域治理闭环已经支持多区域权限，但多组经营统计接口仍然退化为单区域或隐式 region_id 语义，存在明显能力漂移

严重度：高

问题说明：

- 系统底层已经通过 [locallife/api/delivery_fee.go](locallife/api/delivery_fee.go#L58) 的 region 解析逻辑和 `operator_regions` 关系表支持“一个 operator 管多个 region”。
- 但 [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L242)、[locallife/api/operator_stats.go](locallife/api/operator_stats.go#L326)、[locallife/api/operator_stats.go](locallife/api/operator_stats.go#L429) 这些经营统计接口，仍然直接调用 `getOperatorRegionID` 取单一区域。
- 对多区域 operator 而言，这意味着调用方要么传一个文档里没有声明的 `region_id` 才能工作，要么在多区域场景直接收到 “region_id is required when managing multiple regions”。
- 同类问题也出现在“region_id 标为可选”的经营汇总接口，例如 [locallife/api/operator_merchant_rider.go](locallife/api/operator_merchant_rider.go#L869) 默认仍以 `operator.RegionID` 作为后备，并在 [locallife/api/operator_merchant_rider.go](locallife/api/operator_merchant_rider.go#L882) 之后走到 “operator has no assigned region” 错误分支。

影响：

- 闭环图中的“区域治理闭环”在多区域经营场景下已经出现语义漂移。
- 这不是单纯的前端适配问题，而是后端接口契约和治理能力定义已经不一致。
- 一旦 operator 扩展到多个区域，排行、趋势、汇总这些经营接口会出现不可用或口径不明的退化行为。

### Finding 2：运营商趋势和财务概览在关键子查询失败时会静默回退为 0 值，既没有报错，也没有日志，直接违反“错误应可见”的审查目标

严重度：高

问题说明：

- [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L482) 中，`getRegionDailyTrend` 对 `GetOperatorProfitSharingStats` 的失败直接吞掉，`operator_income` 回落为 0。
- [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L583) 到 [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L630) 中，`getOperatorFinanceOverview` 对月度统计、累计统计、运营商收益统计的失败也都采取“如果 err == nil 才赋值，否则静默返回默认 0”的策略。
- 这些失败路径没有 `internalError`，也没有 `log.Error` / `log.Warn`。

影响：

- 财务和趋势面会把“查询失败”伪装成“真实为 0”，这比直接报错更危险。
- 这属于典型的静默降级，会直接污染经营判断、区域治理动作和财务对账认知。
- 从闭环审查角度，这意味着“错误都有落日志”当前不成立，至少在区域经营统计与财务概览这条链上不成立。

### Finding 3：运营商财务概览对非法 region_id 采用静默回退默认区域，而不是显式返回参数错误，存在错误区域数据被误读的风险

严重度：中

问题说明：

- [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L538) 的 `getOperatorFinanceOverview` 支持前端传 `region_id` 做区域切换。
- 但在 [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L549) 这里，只有 `fmt.Sscanf` 成功且值大于 0 才采用该参数；否则直接落回默认区域，不返回 400。

影响：

- 当前端传错 region_id、拼错参数或发生序列化问题时，接口不会失败，而是悄悄返回另一个区域的数据。
- 这类行为会造成最难排查的“页面看起来正常，但看错区域财务数据”的问题。
- 这也是治理视图的契约漂移，不应被视为可接受的容错策略。

## 9. 第二轮正式审查 findings（平台资金闭环 / 售后风控闭环）

以下 findings 基于对平台资金闭环和售后风控闭环相关代码路径的进一步 review，按严重度排序。

业务语义校正（2026-04-17）：

- 用户补充确认的目标模型是：售后/追偿、申诉属于平台侧闭环，不属于运营商职责；补差能力当前未启用。
- 因此，本节 findings 不应被理解为“这些 operator 路径符合预期，只是实现不稳健”，而应理解为“如果以当前确认的业务模型为准，那么这些代码路径本身已经体现了实现偏差或休眠能力漂移”。
- 其中，Finding 4 更准确地说是“未启用能力仍以可调用 operator 写接口形式存在的潜在风险”；Finding 5 和 Finding 6 更准确地说是“平台职责被实现成 operator 审批/核销链路的业务模型漂移”。

### Finding 4：补差接口缺少对象级授权与支付单-商户一致性校验，平台资金写操作当前只有角色边界，没有业务对象边界

严重度：高

状态备注（2026-04-18）：该项按当前排期暂不推进，本轮仅在文档与代码中保留 deferred 标记，不实施补差对象级授权整改本体。

问题说明：

- [locallife/api/server.go](locallife/api/server.go#L1345) 到 [locallife/api/server.go](locallife/api/server.go#L1351) 将补差创建、退回、取消挂在 operator 路由下，但处理函数内部没有继续做对象级权限校验。
- [locallife/api/subsidy.go](locallife/api/subsidy.go#L97) 的 `createSubsidy` 只校验支付单存在、已支付，以及请求里的 `merchant_id` 能取到支付配置；它没有校验这个 `merchant_id` 是否就是该支付单所属商户，也没有校验当前 operator 是否有权操作该支付单所在区域。
- 同样地，[locallife/api/subsidy.go](locallife/api/subsidy.go#L258) 的 `returnSubsidy` 和 [locallife/api/subsidy.go](locallife/api/subsidy.go#L402) 的 `cancelSubsidy` 只按 `payment_order_id` 取补差单，然后直接调用微信 API，也没有区域或商户归属校验。

影响：

- 任何具备 operator 角色的调用方，只要知道支付单 ID，就可能跨区域、跨商户触发补差、补差退回或补差取消。
- `createSubsidy` 还允许请求体传入任意 `merchant_id`，因此存在“支付交易来自 A 商户，但补差资金打到 B 商户 sub_mch_id”的对象绑定错误风险。
- 这属于平台资金闭环里的对象级授权缺口，影响面明显高于一般统计漂移问题。

### Finding 5：售后风控闭环的 operator 追偿/申诉路径仍然固化为单区域 operator 语义，多区域场景会直接漂移或拒绝合法操作

严重度：高

问题说明：

- [locallife/api/claim_recovery.go](locallife/api/claim_recovery.go#L196) 和 [locallife/api/claim_recovery.go](locallife/api/claim_recovery.go#L329) 在 operator 查看追偿单、核销追偿单时，都把 `operator.RegionID` 直接传入 logic。
- [locallife/logic/claim_recovery.go](locallife/logic/claim_recovery.go#L69) 到 [locallife/logic/claim_recovery.go](locallife/logic/claim_recovery.go#L74)、[locallife/logic/claim_recovery.go](locallife/logic/claim_recovery.go#L96) 到 [locallife/logic/claim_recovery.go](locallife/logic/claim_recovery.go#L104) 又继续用“claimInfo.RegionID 必须等于 input.RegionID”的单区域判断。
- 运营商申诉列表和汇总虽然引入了可选 `region_id`，但在未传参时仍回落到 [locallife/api/appeal.go](locallife/api/appeal.go#L1406) 和 [locallife/api/appeal.go](locallife/api/appeal.go#L1504) 的 `getOperatorRegionID` 语义，因此这条售后链和前一轮区域治理闭环暴露的是同一种漂移。

影响：

- 多区域 operator 在售后/追偿场景下会出现“能管理该区域，但接口因为默认单区域判断而拒绝操作”的情况。
- 这意味着售后风控闭环并没有真正跟上多区域治理模型，区域扩展后最容易在申诉审批、追偿查看、追偿核销这些高频治理动作上失真。

### Finding 6：申诉创建成功后，追偿状态切换为 appealed 的失败会被直接吞掉，导致申诉状态与追偿状态可能分裂且没有任何日志

严重度：高

问题说明：

- [locallife/logic/appeal.go](locallife/logic/appeal.go#L120) 和 [locallife/logic/appeal.go](locallife/logic/appeal.go#L239) 在创建商户申诉、骑手申诉成功后，都会尝试把对应追偿单标记为 `appealed`。
- 但这里使用的是 `_, _ = store.MarkClaimRecoveryAppealed(...)`，失败直接忽略，没有返回错误，也没有日志。
- [locallife/db/sqlc/claim_recovery.sql.go](locallife/db/sqlc/claim_recovery.sql.go#L229) 到 [locallife/db/sqlc/claim_recovery.sql.go](locallife/db/sqlc/claim_recovery.sql.go#L247) 显示这个状态变更并不是无条件成功，它只允许从 `pending` / `overdue` 切到 `appealed`，因此完全可能失败。

影响：

- 系统可能出现“申诉单已经创建成功，但追偿单仍停留在 pending / overdue”的状态分裂。
- 后续依赖 `recovery_status = appealed` 的列表筛选、审批语义和恢复逻辑会出现口径错乱，而操作员看不到任何错误信号。
- 这已经不是可接受的非关键附带更新，而是售后风控闭环中的状态传播断裂。

## 10. 职责校正后的链路审查（运营商只读，行为追溯系统自动处置）

用户补充确认后的目标模型：

- 运营商只能查看其关联 region 的赔付、申诉、追偿记录，不具备审核、核销、赔付等处置能力。
- 索赔赔付、平台垫付、追偿生成、追偿恢复、申诉复核与补偿，应由平台行为追溯系统自动处理，而不是运营商人工处理。

基于这一定义，当前代码链路可以拆成三段：

### 10.1 已经存在且符合目标方向的自动主链

- [locallife/api/risk_management.go](locallife/api/risk_management.go#L272) 的 `SubmitClaim` 已经明确走“规则评估 + 行为追溯 + 自动裁决”的正式主链。
- 该入口调用 [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L95) 的自动评估逻辑，并通过 [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L222) 的事务一次性持久化 claim、behavior_decision、behavior_action、claim_recovery 等正式产物。
- 这说明“索赔 -> 平台赔付/垫付 -> 追偿生成”这条主链，已经具备自动化主骨架，不依赖运营商人工审批。

### 10.2 当前实现的主要漂移：申诉主链没有复用行为追溯系统，而是落到了 operator review 模型

严重度：高

问题说明：

- 商户和骑手提交申诉时，当前入口 [locallife/api/appeal.go](locallife/api/appeal.go#L684) 和 [locallife/api/appeal.go](locallife/api/appeal.go#L1179) 调用的是 [locallife/logic/appeal.go](locallife/logic/appeal.go#L69) 与 [locallife/logic/appeal.go](locallife/logic/appeal.go#L173) 的普通 `appeals` 表写入逻辑。
- 该逻辑还会尝试把追偿单直接标记为 `appealed`，但并没有创建或触发 `behavior_appeals` 自动复核链。
- 仓库里虽然已经存在 [locallife/db/sqlc/behavior_trace.sql.go](locallife/db/sqlc/behavior_trace.sql.go#L106) 的 `CreateBehaviorAppeal`、以及 `ListBehaviorAppealsByEntity` / `UpdateBehaviorAppealStatus` 等行为申诉模型，但当前 API、logic、worker 主链都没有接上它。
- 真正被接上的，是 [locallife/api/appeal.go](locallife/api/appeal.go#L1675) 的 `reviewAppeal`，它通过 [locallife/db/sqlc/tx_appeal_review.go](locallife/db/sqlc/tx_appeal_review.go#L25) 把申诉状态、reviewer、review_notes、补偿动作写入数据库，再由 [locallife/worker/task_process_appeal_result.go](locallife/worker/task_process_appeal_result.go#L63) 做回滚追偿、通知和补偿执行。

影响：

- 现在的系统不是“行为追溯系统自动复核申诉，运营商只查看结果”，而是“运营商先决定申诉结果，worker 再做后处理”。
- 这与当前确认的业务模型存在架构级偏差，不是单点实现问题。

### 10.3 需要移除的运营商处置能力

以下能力不符合“运营商只读”边界，应在文档中标记为待移除或待下线：

- [locallife/api/server.go](locallife/api/server.go#L1323) 的 `POST /operators/me/stats/appeals/:id/review`，对应处理器 [locallife/api/appeal.go](locallife/api/appeal.go#L1675)。
- [locallife/api/server.go](locallife/api/server.go#L1325) 的 `POST /operators/me/stats/claims/:id/recovery/waive`，对应处理器 [locallife/api/claim_recovery.go](locallife/api/claim_recovery.go#L314)。
- 与上述写路径绑定的 operator 审计语义，例如 [locallife/api/appeal.go](locallife/api/appeal.go#L1735) 附近写入的 `operator_appeal_reviewed` 行为，也应一起移除或改成系统自动复核审计。

### 10.4 可以保留但需要语义收紧为只读的运营商能力

以下接口符合“运营商查看关联 region 记录”的方向，但仍需后续按多 region 语义收口：

- [locallife/api/server.go](locallife/api/server.go#L1320) 的申诉列表。
- [locallife/api/server.go](locallife/api/server.go#L1321) 的申诉汇总。
- [locallife/api/server.go](locallife/api/server.go#L1322) 的申诉详情。
- [locallife/api/server.go](locallife/api/server.go#L1324) 的追偿详情查看。

这些接口的职责应限定为“读取已持久化的 claim / appeal / recovery / behavior decision 真相”，不能再承担审批、核销或触发补偿动作的职责。

### 10.5 当前最准确的审查结论

- 索赔自动裁决主链：已存在，并且已经以行为追溯系统为核心。
- 申诉自动复核主链：数据模型有预留，但当前生产主链没有接通。
- 运营商能力：当前实现同时包含只读能力和不应存在的处置能力；其中写能力应标记为待移除。
- 因此，这条链路当前的核心问题不是“自动化完全缺失”，而是“自动化主链只覆盖了索赔裁决，没有覆盖申诉复核；缺口被 operator review 线路顶替了”。

## 11. 彻底整改任务计划（按确认后的业务设计落地）

本节目标不是“局部修两个 handler”，而是把售后/追偿/申诉链彻底拉回到以下业务模型：

- 索赔裁决、平台垫付、追偿生成、追偿恢复、申诉复核、申诉补偿都由平台行为追溯系统自动处理。
- 运营商只保留 region 范围内的只读查看能力。
- 商户和骑手看到的是同一套已持久化真相，而不是另一条人工审批语义。

### 11.1 目标状态

目标状态定义如下：

1. 用户提交索赔后，系统继续沿用现有自动裁决主链，自动生成赔付、垫付、追偿、限制服务等正式产物。
2. 商户或骑手提交申诉后，系统创建“待自动复核”的申诉记录，并触发行为追溯复核任务；不再存在人工 review 入口。
3. 申诉复核结果由系统自动写回用户可见申诉记录，并自动执行追偿回滚、追偿恢复、补偿动作、通知与审计。
4. 运营商只能查看关联 region 的 claim / appeal / recovery / behavior decision 结果，不能审批、核销、赔付或补偿。
5. 读模型以持久化后的 behavior decision、appeal、claim recovery 为唯一真相，不允许读取接口自行派生写副作用。

### 11.2 非目标

本轮不把以下内容混入本计划：

- 未启用的补差能力整改（2026-04-18 状态：deferred，本轮仅做显式标记，不推进对象级授权重构）。
- 区域经营统计的多 region 漂移整改。
- 用户索赔规则本身的阈值调参。

这些问题可以后续单独处理，但不应阻塞本条售后闭环整改。

### 11.3 任务分期

#### 第一阶段：先封住错误职责边界

目标：先把不符合业务模型的 operator 写能力从运行面收掉，避免继续扩大错误使用面。

任务：

1. 下线 [locallife/api/server.go](locallife/api/server.go#L1323) 的 operator review 路由。
2. 下线 [locallife/api/server.go](locallife/api/server.go#L1325) 的 operator waive 路由。
3. 清理与这两条写路径绑定的 Swagger 注释、路由注册、权限说明和 API 测试。
4. 保留 operator 只读接口，但在文档中明确为“只读 region 记录查询”。

完成标准：

- 运行面不再存在 operator 可触发的申诉审核、追偿核销入口。
- operator 角色仅能读取申诉列表、申诉详情、申诉汇总、追偿详情。

本轮实施切片：

1. [locallife/api/appeal.go](locallife/api/appeal.go) 将 merchant/rider 申诉创建后立即接入系统自动复核，并删除 operator review 写 handler。
2. [locallife/api/claim_recovery.go](locallife/api/claim_recovery.go) 删除 operator waive 写 handler，仅保留 operator 追偿只读查询。
3. [locallife/api/server.go](locallife/api/server.go) 和 [locallife/casbin/policy.csv](locallife/casbin/policy.csv) 移除 operator 写路由与权限。
4. [locallife/api/appeal_test.go](locallife/api/appeal_test.go) 把申诉创建测试迁移到“自动复核后返回最终状态”，并删除 operator review API 测试，补 route 不可达断言。
5. [locallife/docs/swagger.yaml](locallife/docs/swagger.yaml)、[locallife/docs/swagger.json](locallife/docs/swagger.json)、[locallife/docs/docs.go](locallife/docs/docs.go) 通过 swagger 重新生成去掉 operator 写接口暴露。

#### 第二阶段：定清新的自动复核写模型

目标：把“申诉复核由谁驱动、写哪些表、谁是用户可见真相”固定下来，避免继续在 `appeals` 和 `behavior_appeals` 之间摇摆。

建议定稿：

1. `appeals` 保留为用户可见申诉单，是商户/骑手的正式申诉记录。
2. `behavior_appeals` 作为系统内部复核工作项，承载行为追溯系统的自动复核排队、状态和重评时间。
3. `behavior_decisions` / `behavior_actions` 仍是自动裁决与自动执行的权威写模型。
4. `claim_recoveries` 保留为追偿执行状态模型，但状态推进改为系统任务驱动，不再由 operator handler 驱动。

任务：

1. 明确 `appeals` 的系统字段设计，例如增加 `resolution_source=system`、`resolved_decision_id`、`resolution_reason` 之类的系统归因字段，避免继续复用 `reviewer_id` / `review_notes` 的人工语义。
2. 明确 `behavior_appeals.status` 枚举，至少覆盖 `pending`, `processing`, `resolved`, `failed` 这类系统状态。
3. 设计“申诉创建 -> 生成 behavior_appeal -> 任务入队 -> 自动复核 -> 写回 appeal/recovery/action”这一条单向主链。

完成标准：

- 申诉自动复核链的权威写模型清晰，代码层不再需要 operator 参与才能推进状态。
- `appeals` 和 `behavior_appeals` 的边界在文档和代码中一致。

#### 第三阶段：重写申诉创建主链

目标：让商户/骑手提交申诉时直接进入系统自动复核，而不是先写一个待 operator 处理的工单。

任务：

1. 改造 [locallife/logic/appeal.go](locallife/logic/appeal.go#L69) 和 [locallife/logic/appeal.go](locallife/logic/appeal.go#L173) 的创建逻辑。
2. 申诉创建成功后，不再调用“人工审批语义”的 `ReviewAppealWithCompensationTx`。
3. 申诉创建时同时创建 `behavior_appeal` 记录，并入队自动复核任务。
4. 现在的 `MarkClaimRecoveryAppealed` 逻辑需要改成系统可见、可审计、不可静默失败的状态推进；如果继续保留 `appealed` 作为“复核中”状态，也必须由系统任务驱动并记录日志。

完成标准：

- 创建 merchant/rider appeal 后，系统无需任何 operator 写操作即可继续推进到最终结论。
- 申诉和追偿状态传播不再存在静默失败点。

#### 第四阶段：实现系统自动复核与自动处置任务

目标：把当前“operator 先决定，worker 再善后”的模式，改成“worker/系统任务自己完成复核与后处理”。

任务：

1. 新建或改造申诉复核任务，由任务读取 `behavior_appeal`、关联 `behavior_decision`、claim、recovery、证据快照，自动得出复核结论。
2. 复核通过时，自动回滚追偿、解除商户/骑手限制、执行申诉补偿动作、发送通知、记审计。
3. 复核不通过时，自动恢复追偿并发送通知。
4. 把 [locallife/worker/task_process_appeal_result.go](locallife/worker/task_process_appeal_result.go#L63) 从“消费 operator 决定结果”改成“消费系统自动复核结果”。
5. 审计字段改成系统行为，例如 `system_appeal_resolved`，不再记 `operator_appeal_reviewed`。

完成标准：

- appeal 的最终状态由系统任务写出，不再依赖人工 review 输入。
- 追偿回滚、恢复、补偿、通知和审计全部由系统任务闭环完成。

#### 第五阶段：收口只读视图和多角色查询

目标：让 merchant、rider、operator、平台看到的是同一条真相的不同只读切片。

任务：

1. merchant/rider 的申诉列表、详情继续以用户侧视角展示，但底层读取系统已持久化结果。
2. operator 侧只保留 region 范围内的只读聚合与详情。
3. operator 读路径中所有文案、注释、Swagger 描述改成“查看记录/查看结果”，去掉“审核”“核销”等处置性表述。
4. 对 operator 只读路径补多 region 语义校验，避免继续绑定 `operator.RegionID` 单区域假设。

完成标准：

- 系统不存在任何 operator 处置语义残留。
- 同一条 appeal / recovery 在不同角色视图里语义一致。

#### 第六阶段：历史数据迁移与兼容清理

目标：避免老数据把新模型继续污染下去。

任务：

1. 盘点已存在的 operator reviewed appeals 和 waived recoveries。
2. 设计历史数据兼容策略：保留历史字段但标记为 legacy，或做一次性回填，把老记录映射到新的系统 resolution_source。
3. 清理不再使用的 handler、事务、测试桩和审计事件名称。
4. 对外文档统一改成“平台自动处理，运营商只读查看”。

完成标准：

- 新老数据都能被正确解释，不会再把历史人工语义误认为当前业务模型。
- 代码库中不再残留 active 的 operator 处置入口。

### 11.4 验证计划

整改完成后，至少需要以下验证：

1. 用户提交索赔后，自动生成赔付/追偿/decision/action，且 operator 不参与。
2. 商户申诉创建后，系统自动进入复核任务并最终自动落定结果。
3. 骑手申诉创建后，系统自动进入复核任务并最终自动落定结果。
4. appeal 通过时，追偿自动回滚、限制自动解除、补偿自动执行、通知自动发送。
5. appeal 不通过时，追偿自动恢复，通知自动发送。
6. operator 角色调用旧 review/waive 路由时，应不可达。
7. operator 只读接口仍能查看关联 region 的赔付、申诉、追偿记录。
8. 失败路径必须有日志：复核任务失败、追偿状态推进失败、补偿动作失败、通知失败都要有结构化日志。

### 11.5 建议实施顺序

建议按以下顺序推进，避免边改边扩大不一致：

1. 先封运行面：下线 operator 写路由。
2. 再定写模型：固定 appeals / behavior_appeals / decisions / recoveries 的职责边界。
3. 再改创建主链：merchant/rider appeal 直接进入系统复核。
4. 再改 worker：自动输出复核结果并执行后处理。
5. 最后做只读视图清理、历史兼容和测试收口。

### 11.6 完成定义

这条链路可以视为“彻底解决”的标准是：

- 运营商不再拥有任何赔付、申诉、追偿处置能力。
- 索赔与申诉都由行为追溯系统自动裁决或自动复核。
- appeal、recovery、decision、action 之间不存在人工中断点。
- 关键失败都有日志，关键状态都能追溯，关键结果都能被多角色只读视图一致消费。