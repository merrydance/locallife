# 异常订单顾客索赔目标流程与现系统实现差异分析

## 1. 目标

本文只做一件事：

把当前后端实现，与最新确认的异常订单顾客索赔目标流程做逐段对照。

目标流程以这份文档为准：

- [artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md](artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md)

本次对照重点只看四件事：

1. `lookback` 是否已经成为判责核心。
2. 现系统是否已经拆成“判责阶段”与“赔偿阶段”两段。
3. 判责完成后，用户拿到的是不是“判责结果”，再进入后续赔偿等待。
4. 责任不清晰、顾客高风险、限制名单三条关键分支，现系统是否与目标一致。

## 2. 结论先说

结论很明确：

当前实现还没有对齐目标流程。

最主要的偏差有五个：

1. `lookback` 不是当前判责核心，甚至没有真正接入生产主链。
2. 当前实现没有清晰拆成“先判责、后赔偿”两阶段，而是在提交索赔时就同步创建了赔付动作和追偿动作。
3. 当前顾客看到的是“自动判责 + 赔偿处理中”的合并结果，不是“判责完成，若成立则等待赔偿到账”的两段式结果。
4. 当前“责任不清晰”并不会默认落到商户或骑手，而是会被提升为平台兜底。
5. 当前高风险顾客路径是系统直接限制并平台兜底，不存在“先通知判责结果，再由顾客确认继续索赔”的独立确认阶段。

但也有两项基础能力已经存在：

1. 判责事实、行为快照、追偿单、赔付动作已经有正式持久化对象。
2. 限制名单与停服能力已经有数据库与下单拦截基础。

## 3. 按目标流程逐段对照

## 3.1 平台介入入口

目标要求：

1. 顾客提交平台介入申请。
2. 系统只校验订单归属、订单状态、索赔时效等基础条件。
3. 平台介入后进入判责阶段。

当前实现：

- [locallife/api/risk_management.go](locallife/api/risk_management.go#L280-L330)

现状判断：

1. 当前入口已经没有“前置协商记录校验”，这一点和最新目标一致。
2. 入口会校验订单存在、订单归属、订单已完成。
3. 入口还会先检查顾客是否已经命中 `BehaviorBlocklist`，如果已被限制，则直接拒绝继续提交索赔。

这一段的结论是：

入口层已经具备“基础订单条件校验”的形态，但它不是“进入判责阶段”的纯入口，因为后续并不是只做判责，而是直接把赔偿链也推进了。

## 3.2 `lookback` 是否是判责核心

目标要求：

`lookback` 应该是最核心的判责依据。平台应围绕最近若干笔订单、相似异常、对手方历史、时间集中度等回溯事实来决定本次责任归属。

当前实现里与 `lookback` 有关的代码：

- [locallife/algorithm/lookback_checker.go](locallife/algorithm/lookback_checker.go)
- [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L98)
- [locallife/algorithm/claim_types.go](locallife/algorithm/claim_types.go#L84)

现状判断：

1. `LookbackChecker` 的 `PerformLookback`、`CheckClaimCorrelation`、`GetUserRecentClaimCount` 都定义好了。
2. `ClaimAutoApproval` 在构造时确实注入了 `lookbackChecker`，见 [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L98)。
3. 但生产代码里没有看到 `PerformLookback` 或 `CheckClaimCorrelation` 被真正调用；全文搜索命中只有定义与注入，没有主链执行点。
4. `Decision` 虽然预留了 `LookbackData` 字段，`CreateClaimWithDecisionAndEvidence` 也会序列化 `decision.LookbackData`，但实际没有代码给它赋值，见 [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L457-L459)。

当前实际判责靠什么：

1. API 层的 `EvaluateClaim` 主要用的是聚合统计 `GetUserBehaviorStats`，按近 90 天订单数、索赔数、警告数、平台兜底数来给出 `Normal/Warned/PlatformFallback/UserRestricted`，见 [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L222-L290)。
2. 真正的正式判责写边界在 [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L260-L470)，它读取的是行为 effect summary、设备/地址关联、delivery 责任事实，再做一套评分桥接。

这一段的结论是：

当前系统里，`lookback` 不是判责核心，只是一个已定义但未接线的候选组件。当前生产主链真正依赖的是“用户行为聚合统计 + 行为 effect summary + 评分桥接”。

## 3.3 是否已经拆成判责阶段与赔偿阶段

目标要求：

系统要先完成判责，再进入理赔。

推荐阶段边界应是：

1. 提交平台介入申请。
2. 生成判责结果。
3. 通知顾客判责结果。
4. 若需要理赔，再进入赔偿处理中。

当前实现：

- [locallife/api/risk_management.go](locallife/api/risk_management.go#L495-L706)
- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L313-L595)

现状判断：

1. `SubmitClaim` 在一次请求里就先调用 `EvaluateClaim`，随后直接调用 `CreateClaimWithDecisionAndEvidence`。
2. `CreateClaimWithBehaviorTx` 在一个事务里同时创建：
   - `claim`
   - `behavior_decision`
   - `behavior_trace_snapshots`
   - `behavior_action(payout)`
   - `behavior_action(recovery)`
   - `behavior_action(block)`
   - `behavior_action(notify)`
   - `claim_recovery`
3. 也就是说，当前不是“判责结束，再启动赔偿”，而是“判责记录和赔偿动作一起落库”。

这一段的结论是：

当前实现没有真正拆成“判责阶段/赔偿阶段”两段。它更接近“提交索赔时同时完成判责落库，并立即创建后续理赔 outbox”。

## 3.4 顾客收到的是否是判责结果

目标要求：

判责完成后，应先通知顾客判责结果；如果决定理赔，则让顾客等待赔偿到账。

当前实现：

- [locallife/api/risk_management.go](locallife/api/risk_management.go#L652-L666)
- [locallife/api/risk_management.go](locallife/api/risk_management.go#L181-L183)
- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L603-L669)

现状判断：

1. 提交成功后，API 直接返回 `DecisionStatus=auto-adjudicated` 与 `PayoutStatus=processing`。
2. 这意味着顾客拿到的不是单独的“判责完成结果”，而是“已经判责，而且赔付处理中”的合并结果。
3. `createBehaviorNotificationAction` 当前只会给三类对象建通知动作：
   - 商户责任成立时通知商户
   - 骑手责任成立时通知骑手
   - 用户被限制时通知用户
4. 对于普通顾客的“判责成立，赔偿稍后到账”，当前没有单独的用户通知动作。

这一段的结论是：

当前系统没有把“判责结果通知顾客”和“赔偿处理中通知顾客”拆开。顾客主要靠同步接口返回值知道结果，而不是靠正式的“判责完成通知”对象。

## 3.5 责任不清晰时落到谁头上

目标要求：

责任不清晰时，倾向由销售/服务方承担责任：

1. 商品质量类默认商户承担。
2. 配送履约类默认骑手承担。

当前实现：

- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L1118-L1159)
- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L1319-L1336)

现状判断：

1. 当责任事实缺失、商户责任分不足、骑手责任分不足、或整体置信度不足时，系统会调用 `promoteBehaviorPlatformFallback`。
2. 这个函数会把 `ResponsibleParty` 改成 `platform_fallback`，`CompensationSource` 改成 `platform`，并关闭 recovery。
3. 也就是说，当前“不清晰/证据弱”的默认归宿不是商户或骑手，而是平台兜底。

这一段的结论是：

当前实现与目标完全不一致。目标是销售/服务方兜责，当前实现是平台兜底。

## 3.6 顾客高风险路径是否先判责、再让用户确认继续索赔

目标要求：

1. 平台先把判责原因通知顾客。
2. 顾客可以撤回。
3. 顾客若坚持继续，则赔付并将其纳入限制名单。

当前实现：

- [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L171-L214)
- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go#L1150-L1350)
- [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L575-L631)

现状判断：

1. API 层 `EvaluateClaim` 在命中高风险用户时，会直接把决策切成 `PlatformFallback` 或 `UserRestricted`。
2. 事务层也会根据用户风险分、历史恶意记录、设备/地址关联，把决策提升为 `UserRestricted`。
3. 一旦进入 `UserRestricted`，系统会在同一条主链里同时：
   - 保留本次平台赔付
   - 创建用户 block action
   - 创建用户限制通知动作
4. 系统里没有看到一个“顾客收到判责结果后，再确认是否继续索赔”的正式 API 或状态。

这一段的结论是：

当前系统已经有“赔付 + 拉黑/停服”的技术基础，但它是系统直接执行，不是目标里的“先判责通知，再让顾客决定继续”的两步式路径。

## 3.7 限制名单与停服能力是否已经存在

目标要求：

顾客在高风险提醒后仍坚持索赔，应被纳入限制名单，平台后续不再继续提供服务。

当前实现：

- [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go#L324-L363)
- [locallife/logic/order_validation.go](locallife/logic/order_validation.go#L99-L119)
- [locallife/logic/order_service.go](locallife/logic/order_service.go#L252-L260)
- [locallife/api/risk_management.go](locallife/api/risk_management.go#L306-L314)
- [locallife/api/behavior_trace.go](locallife/api/behavior_trace.go#L18-L73)

现状判断：

1. 当前已经有正式的 `BehaviorBlocklist` 数据对象。
2. `applyUserRestrictionBlock` 会把用户写入黑名单，reason code 为 `malicious-claims`。
3. 下单链路会检查 `CheckTakeoutBlocklist`，命中时拒绝外卖服务。
4. 顾客再次提交索赔时，API 入口也会先拦截。
5. 商户后台已经有查询顾客风险提示的接口。

这一段的结论是：

目标中的“限制名单/停服”不是从零开始，当前系统已经具备相当明确的基础能力。

## 4. 当前最接近目标的现有能力

虽然整体主链未对齐，但现系统里有三块能力已经很接近目标，可作为后续改造支点：

1. 判责事实对象已经存在：`behavior_decision`、`trace_snapshots`、`graph_hits`、`fact_snapshot`。
2. 理赔阶段已经异步化了一部分：赔付动作通过 `behavior_action(payout)` 持久化，后续由 scheduler 与支付回调推进，不是纯同步转账。
3. 限制名单已经正式落库，并已接入下单与风险查询链路。

## 5. 改造优先级建议

如果要把当前实现改到目标流程，优先级建议如下：

### 第一优先级：把判责与赔偿拆成两段

先把业务边界改正：

1. `SubmitClaim` 只创建索赔单与判责任务。
2. 判责完成后生成明确的判责结果对象与用户通知。
3. 只有在判责结果要求赔付时，才创建 payout/recovery action。

### 第二优先级：让 `lookback` 真正成为判责核心

1. 把 `PerformLookback` 与 `CheckClaimCorrelation` 接入主链。
2. 让 `Decision.LookbackData` 成为正式判责输入，而不是空字段。
3. 用最近若干笔订单与对手方对照事实，替代当前仅靠聚合统计的轻量预判。

### 第三优先级：把“责任不清晰”从平台兜底改成销售/服务方兜责

1. 去掉当前 `promoteBehaviorPlatformFallback` 的默认兜底语义。
2. 按异常类型和履约事实，把不清晰责任优先落给商户或骑手。
3. 平台只保留极端异常情况下的内部补偿或调账能力，不再把它当成主默认路径。

### 第四优先级：补顾客确认继续索赔的显式阶段

1. 把当前“系统直接 restricted + compensate”改成“先通知判责结果”。
2. 顾客显式确认继续后，才执行赔付并落限制名单。

## 6. 一句话结论

当前系统已经有“行为决策持久化、异步赔付、追偿单、限制名单”的技术底座，但它还不是你要的那条流程。

最大的偏差是：

当前主链不是“`lookback` 核心判责 -> 判责完成通知用户 -> 再进入理赔”，而是“轻量预判 + 评分桥接判责 -> 在同一提交链路里把赔付/追偿/限制动作一并建好 -> 直接返回赔付处理中”。