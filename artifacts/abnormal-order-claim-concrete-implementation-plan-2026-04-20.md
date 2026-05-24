# 异常订单顾客索赔代码级实施计划

## 1. 目标

本文把已经确认的目标业务链，进一步下沉到代码实施层。

目标不是再讨论“应该怎么设计”，而是明确：

1. 先改哪些文件。
2. 每个文件的职责怎么迁移。
3. 哪些函数保留但降级。
4. 哪些事务边界要拆开。
5. 如何分阶段改，才能在不打爆现有 payout/recovery 底座的前提下完成重构。

配套参考：

- [artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md](artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md)
- [artifacts/abnormal-order-claim-target-vs-current-gap-analysis-2026-04-20.md](artifacts/abnormal-order-claim-target-vs-current-gap-analysis-2026-04-20.md)
- [artifacts/abnormal-order-claim-redesign-intent-and-refactor-plan-2026-04-20.md](artifacts/abnormal-order-claim-redesign-intent-and-refactor-plan-2026-04-20.md)

## 2. 实施原则

## 2.1 先拆业务边界，再换判责模型

当前最大的业务错误，不是某一条规则算错，而是：

1. `SubmitClaim` 把判责和赔偿压成一次返回。
2. `CreateClaimWithBehaviorTx` 在一个事务里同时落判责和 payout/recovery/block/notify action。
3. `EvaluateClaim` 和事务层都在做正式判责，形成双判责结构。

因此实施顺序必须是：

1. 先把判责与赔偿边界拆开。
2. 再把旧评分桥接从主判责位置退下去。
3. 最后清理旧 lookback 和旧 API 语义。

## 2.2 优先复用现有持久化底座

本次不建议新起一整套 claim 表族。

优先复用并调整现有对象：

1. `claims`
2. `behavior_decision`
3. `behavior_trace_snapshots`
4. `behavior_decision_effects`
5. `behavior_actions`
6. `claim_recovery`

也就是说，重构重点应是“职责和时序重组”，不是“表重命名”。

## 2.3 第一阶段允许同步判责，暂不强制异步化

为了降低改造风险，第一阶段不需要把判责变成异步 worker。

可以接受：

1. `SubmitClaim` 仍然同步完成正式判责。
2. 但它只能返回判责结果，不能再同时创建赔偿动作。

这样可以先把核心边界改对，再决定后续是否把判责也异步化。

## 3. 目标链路映射到代码后的形态

## 3.1 提交阶段

提交阶段只做四件事：

1. 校验订单归属、状态、时效、黑名单。
2. 采集证据上下文。
3. 计算可赔金额上限。
4. 调用正式判责器，落 claim + behavior decision + trace snapshots。

提交阶段不做：

1. 创建 payout action。
2. 创建 recovery action。
3. 创建 restriction action。
4. 直接把外部状态返回为 `processing`。

## 3.2 判责阶段

判责阶段由一个正式 adjudicator 统一完成。

它统一消费：

1. 当前订单中心三方 lookback 结果。
2. 设备/地址关联证据。
3. 履约责任事实。
4. 必要的 effect summary，但只能作为辅助证据，不能继续当主决策器。

它统一产出：

1. 责任方。
2. 赔偿是否成立。
3. 是否需要用户确认继续索赔。
4. 是否需要后续限制服务。
5. 用户可见说明。
6. 平台内部原因码。

## 3.3 赔偿阶段

赔偿阶段只消费“已经完成的判责结果”。

它负责：

1. 创建 payout action。
2. 创建 recovery action。
3. 创建 restriction action。
4. 创建 notify action。

worker、scheduler、callback 底座保持不变，只是触发时机后移。

## 4. 文件级改造清单

## 4.1 API 层

主文件：

- [locallife/api/risk_management.go](locallife/api/risk_management.go)

需要做的事：

1. 重写 `SubmitClaimResponse`，把当前 `accepted + auto-adjudicated + processing` 模型拆开。
2. `SubmitClaim` 改成只返回判责阶段状态。
3. 新增“顾客确认继续索赔”入口。
4. 新增“触发赔偿阶段”调用点，但不要在 `SubmitClaim` 里直接写 payout/recovery/block action。

建议新增或调整的响应语义：

1. `status`
2. `decision_status`
3. `compensation_status`
4. `customer_action_required`
5. `customer_action`
6. `reason`
7. `warning`

建议第一阶段使用的外部状态：

1. `pending_platform_review`
2. `adjudicated`
3. `awaiting_compensation`
4. `compensating`
5. `compensated`
6. `warned_waiting_customer_confirmation`
7. `restricted_compensated`
8. `closed`

新增 API：

1. `POST /claims/:id/confirm-continue`
2. 用于高风险顾客在收到判责说明后确认继续。

第一阶段不建议新增：

1. 独立判责 worker API
2. 手工运营调查 API

## 4.2 algorithm 包

主文件：

- [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go)
- [locallife/algorithm/lookback_checker.go](locallife/algorithm/lookback_checker.go)
- [locallife/algorithm/claim_types.go](locallife/algorithm/claim_types.go)

建议新增文件：

1. `locallife/algorithm/claim_adjudicator.go`
2. `locallife/algorithm/claim_adjudication_types.go`
3. `locallife/algorithm/lookback_tri_party.go`

### 4.2.1 `claim_auto_approval.go`

当前问题：

1. `EvaluateClaim` 仍在做正式判责。
2. `CheckUserClaimBehavior` 仍在用 90 天聚合统计驱动正式结果。
3. `CreateClaimWithDecisionAndEvidence` 直接调用 `CreateClaimWithBehaviorTx`，把正式判责和赔偿动作绑定在一起。

改造方向：

1. `EvaluateClaim` 降级为轻量预校验/金额裁剪 helper，或直接重命名为 `PrepareClaimSubmission`。
2. `CheckUserClaimBehavior` 降级为“顾客风险辅助提示”，不再输出最终责任模式。
3. `CreateClaimWithDecisionAndEvidence` 拆成两个调用：
   - `CreateClaimAdjudicationTx`
   - `CreateClaimCompensationTx`
4. `applyPersistedDecisionSideEffects` 不再在提交阶段执行限制或平台兜底计数副作用，改为消费正式 compensation 阶段结果。
5. `executePersistedBehaviorActions` 只保留给 compensation 阶段结果消费，提交阶段不再调用。

### 4.2.2 `lookback_checker.go`

当前问题：

1. 以顾客历史索赔集合为锚点。
2. 只会推出“同一商户集中”“同一骑手集中”等顾客导向的可疑模式。
3. 没有被主链调用。

改造方向：

1. 不在原文件上继续修补。
2. 新建三方 lookback 实现，旧文件先保留但退出主链。

新的 lookback 输入应至少包含：

1. `OrderID`
2. `UserID`
3. `MerchantID`
4. `RiderID`
5. `ClaimType`
6. `WindowDays`

新的 lookback 输出应至少包含：

1. `CustomerWindowSummary`
2. `MerchantWindowSummary`
3. `RiderWindowSummary`
4. `ComparisonConclusion`
5. `ReasonCodes`
6. `UserFacingExplanation`

### 4.2.3 新正式判责器

新增 `claim_adjudicator.go`，作为正式判责唯一入口。

建议暴露：

1. `AdjudicateClaim(ctx, input) (result, error)`

建议输入结构：

1. 订单上下文
2. claim 基础信息
3. lookback 结果
4. 设备/地址关联证据
5. 履约责任事实
6. effect summary 辅助视图

建议输出结构：

1. `DecisionMode`
2. `ResponsibleParty`
3. `CompensationSource`
4. `ApprovedAmount`
5. `RequiresCustomerConfirmation`
6. `ShouldRestrictAfterCompensation`
7. `CreateRecovery`
8. `RecoveryTarget`
9. `RecoveryAmount`
10. `TraceSummary`
11. `ReasonCodes`
12. `LookbackSnapshot`

## 4.3 事务与存储层

主文件：

- [locallife/db/sqlc/tx_claim_behavior.go](locallife/db/sqlc/tx_claim_behavior.go)
- [locallife/db/query](locallife/db/query)
- [locallife/db/sqlc/constants.go](locallife/db/sqlc/constants.go)

### 4.3.1 事务拆分

当前问题：

`CreateClaimWithBehaviorTx` 同时做了三类职责：

1. claim 与 decision 落库
2. trace snapshot 与 effect 写入
3. payout/recovery/block/notify action 创建

应拆成两个事务入口：

1. `CreateClaimAdjudicationTx`
2. `CreateClaimCompensationTx`

#### `CreateClaimAdjudicationTx`

职责：

1. 创建 claim
2. 创建 behavior_decision
3. 创建 trace snapshots
4. 创建 decision effects
5. 更新 claim 为判责完成态
6. 如顾客需二次确认，则只落等待确认态，不创建赔偿动作

禁止在这里做：

1. payout action
2. recovery action
3. restriction action
4. notify responsible party action

#### `CreateClaimCompensationTx`

职责：

1. 读取既有 adjudication result
2. 校验 claim 当前状态允许进入赔偿
3. 创建 payout action
4. 需要时创建 recovery action
5. 需要时创建 restriction action
6. 创建相关 notify action
7. 更新 claim/decision 为 compensation 阶段状态

### 4.3.2 `tx_claim_behavior.go` 中需要退出主链的函数

以下函数不一定删除，但要退出“正式决策主流程”的硬编码位置：

1. `promoteBehaviorPlatformFallback`
2. `promoteBehaviorUserRestricted`
3. `buildBehaviorDecisionScores`

处理方式：

1. 第一阶段保留它们作为辅助证据计算函数。
2. 不再允许它们直接改写最终责任方。
3. 最终责任方改由新的 adjudicator 输出。

### 4.3.3 新查询需求

需要在 `db/query` 增加当前订单中心三方 lookback 查询。

至少需要这些查询：

1. 按用户 + claim_type + 时间窗统计相似异常索赔。
2. 按商户 + claim_type + 时间窗统计被不同顾客索赔次数。
3. 按骑手 + claim_type + 时间窗统计被不同顾客索赔次数。
4. 获取当前订单的 merchant/rider/delivery 履约上下文。
5. 获取同窗口内责任相关订单样本或摘要。

建议策略：

1. 先做 summary query，优先满足正式判责。
2. 只有在用户说明文案需要时，再补样本级明细查询。

### 4.3.4 状态常量

需要把 claim 外部生命周期与内部 decision lifecycle 正式常量化。

建议新增到常量层，而不是在 API 或测试里写 magic string。

claim lifecycle 建议：

1. `pending_platform_review`
2. `adjudicated`
3. `awaiting_compensation`
4. `compensating`
5. `compensated`
6. `warned_waiting_customer_confirmation`
7. `restricted_compensated`
8. `closed`

decision lifecycle 建议：

1. `decided`
2. `waiting_customer_confirmation`
3. `compensation_ready`
4. `compensation_started`
5. `compensation_finished`
6. `closed`

## 4.4 worker / scheduler / callback

主文件：

- [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go)
- [locallife/worker/claim_refund_recovery_scheduler.go](locallife/worker/claim_refund_recovery_scheduler.go)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)

改造方向：

1. worker 不需要重写核心执行逻辑。
2. scheduler 不需要改变“扫描 created action”这一基本模型。
3. callback 不需要改变“按 action 推进终态”的基本模型。

真正要改的是上游触发时机：

1. `SubmitClaim` 不再直接制造可执行 payout action。
2. 只有 `CreateClaimCompensationTx` 才会创建这些 action。

因此这部分主要是：

1. 适配新的 claim/decision 状态。
2. 确认 scheduler 只扫描 compensation 阶段创建出的 action。
3. 确认 callback 更新的是 compensation 生命周期，而不是旧的“提交即 processing”状态。

## 4.5 服务限制链路

主文件：

- [locallife/logic/order_validation.go](locallife/logic/order_validation.go)
- [locallife/logic/order_service.go](locallife/logic/order_service.go)

改造方向：

1. `BehaviorBlocklist` 读链路保留。
2. 下单拦截逻辑保留。
3. 只调整 restriction 触发时机，从“提交即高风险命中后直接执行”改成“顾客确认继续索赔且赔付完成后执行”。

## 5. 推荐的分阶段实施顺序

## 阶段 1：先把写边界拆开

目标：不碰 worker 支付执行底座，先把“提交即赔偿处理中”纠正掉。

动作：

1. 新增 `CreateClaimAdjudicationTx`。
2. 从 `CreateClaimWithBehaviorTx` 抽出 payout/recovery/block/notify action 创建逻辑到 `CreateClaimCompensationTx`。
3. `SubmitClaim` 改为只走 adjudication tx。
4. API 响应改成返回 adjudication 语义，不再返回 `payout_status=processing`。

阶段 1 完成标志：

1. 提交索赔后数据库中只有 claim + decision + snapshots + effects。
2. 没有 payout/recovery/restriction/notify action 被立即创建。
3. 现有 worker 测试仍能跑，但要靠新的 compensation 入口来创建 action。

## 阶段 2：引入当前订单中心三方 lookback

目标：先把新判责最重要的事实输入接起来。

动作：

1. 新增三方 lookback query。
2. 新增 `lookback_tri_party.go`。
3. 在 adjudicator 中接入 lookback 输出。
4. 把旧 `LookbackChecker` 从主链彻底移除。

阶段 2 完成标志：

1. `Decision.LookbackData` 不再为空壳。
2. `behavior_trace_snapshots` 的来源事实能反映顾客/商户/骑手同窗口摘要。
3. 主链不再依赖“顾客集中针对某商户/骑手”的旧相关性判断。

## 阶段 3：引入单一正式 adjudicator

目标：消灭双判责。

动作：

1. 新增 `AdjudicateClaim`。
2. `EvaluateClaim` 降级为预处理 helper，或仅负责金额裁剪和 claim type 基础映射。
3. `CheckUserClaimBehavior` 降级为风险提示输入，不再输出正式责任模式。
4. `promoteBehaviorPlatformFallback`、`promoteBehaviorUserRestricted` 退出最终改写责任方的主路径。

阶段 3 完成标志：

1. 责任方真值只来自 adjudicator 输出。
2. API 层与事务层不再分别改写责任结论。
3. effect summary 与 score bridge 只保留辅助证据角色。

## 阶段 4：接入高风险顾客确认继续索赔路径

目标：补上当前缺失的显式业务节点。

动作：

1. 增加 claim 状态 `warned_waiting_customer_confirmation`。
2. 新增顾客确认继续 API。
3. 只有确认继续后才调用 `CreateClaimCompensationTx`。
4. compensation 完成后再创建 restriction action 并触发 blocklist。

阶段 4 完成标志：

1. 高风险顾客不会在提交时被立即限制。
2. 顾客可以撤回。
3. 坚持继续时才发生赔付 + 限制服务。

## 阶段 5：清理旧规则和兼容代码

目标：收掉旧链路残留。

动作：

1. 删除旧 lookback 主逻辑调用。
2. 删除旧 API `processing` 语义依赖。
3. 删除测试中对 `CreateClaimWithBehaviorTx` 一次性创建所有 action 的假设。
4. 视情况废弃 `CreateClaimWithBehaviorTx`，保留兼容 wrapper 一段时间后再删。

## 6. 测试改造计划

## 6.1 API 测试

主文件：

- [locallife/api/risk_management_test.go](locallife/api/risk_management_test.go)

要改的断言：

1. 不能再断言 `DecisionStatus=auto-adjudicated + PayoutStatus=processing`。
2. 改成断言 adjudication status、customer action required、warning 等判责阶段字段。
3. 新增高风险顾客确认继续索赔 API 测试。

## 6.2 algorithm 测试

主文件：

- [locallife/algorithm/claim_auto_approval_test.go](locallife/algorithm/claim_auto_approval_test.go)

要改的断言：

1. `EvaluateClaim` 不再直接输出平台兜底或限制服务的最终业务动作。
2. 新增 adjudicator 测试，重点覆盖：
   - 商品质量类责任不清晰默认商户
   - 代取履约类责任不清晰默认骑手
   - 顾客高风险要求确认继续
   - 三方 lookback 对责任归属的影响

## 6.3 存储层测试

主文件：

- [locallife/db/sqlc](locallife/db/sqlc)

要新增的测试重点：

1. `CreateClaimAdjudicationTx` 不创建 compensation actions。
2. `CreateClaimCompensationTx` 只能在合法前置状态下创建 actions。
3. 高风险顾客路径在未确认前不能创建 block/restriction action。
4. recovery 只在正式责任方是 merchant/rider 时创建。

## 6.4 worker 测试

保留现有 payout/recovery/callback 测试主轴。

新增校验：

1. action 创建时机后移后，worker 仍能完整推进终态。
2. compensation 完成后 restricted 路径是否能正确收口。

## 7. 数据库与生成物步骤

执行顺序建议：

1. 先改 `db/query` 源 SQL。
2. 再补 transaction 代码。
3. 执行 `make sqlc`。
4. 如果 mock store 接口变了，再执行 `make mock`。

如果新增或调整 swagger 响应：

1. 最后执行 `make swagger`。

## 8. 验证顺序

最小验证建议按这个顺序跑：

1. `go test ./algorithm -run 'Test(EvaluateClaim|Adjudicate|Lookback)'`
2. `go test ./db/sqlc -run 'Test(CreateClaimAdjudicationTx|CreateClaimCompensationTx)'`
3. `go test ./api -run 'TestSubmitClaimAPI|TestConfirmClaimContinueAPI'`
4. `go test ./worker -run 'TestProcessTaskClaimPayout|TestClaimRefundRecovery'`

阶段 1 至阶段 3 完成后，再补一轮跨层组合验证。

## 9. 建议的实际开工顺序

如果马上开始改代码，建议严格按下面顺序做：

1. 先在 `db/sqlc/tx_claim_behavior.go` 抽出 `CreateClaimAdjudicationTx` 和 `CreateClaimCompensationTx`。
2. 再改 `api/risk_management.go` 的响应模型和 `SubmitClaim` 主链。
3. 然后补 `algorithm/claim_adjudicator.go` 和新 lookback。
4. 再加高风险顾客确认继续索赔 API。
5. 最后清理旧评分桥接和旧 lookback 调用。

## 10. 一句话结论

最稳的改法不是直接重写整个索赔系统，而是先把现有 `CreateClaimWithBehaviorTx` 这条“判责 + 赔偿动作一起落库”的写边界拆开，再用当前订单中心三方 lookback 和单一 adjudicator 替换掉旧的顾客聚合预判与评分桥接主导逻辑；赔付 worker、scheduler、callback、blocklist 这些底座都可以保留，只需要把触发时机后移到正式判责之后。