# 异常订单顾客索赔新 lookback 与 adjudicator 契约草案

## 1. 目标

本文只定义两件事：

1. 新的当前订单中心三方 lookback 应该长什么样。
2. 新的正式 adjudicator 输入/输出契约应该长什么样。

它的作用是把后续代码实现需要的结构、字段、原因码和状态转移固定下来。

本文不是数据库设计稿，也不是最终 Go 代码提交稿，但会尽量贴近即将落地的实现结构。

## 2. 设计边界

## 2.1 lookback 的边界

lookback 只负责提供“当前订单中心三方回溯事实”。

lookback 不负责：

1. 最终责任判定。
2. payout/recovery/block action 创建。
3. 把顾客风险直接提升成正式限制动作。

## 2.2 adjudicator 的边界

adjudicator 是唯一正式判责入口。

adjudicator 负责：

1. 读取证据输入。
2. 形成责任归属。
3. 形成赔偿/追偿/限制的业务决议。
4. 给出用户可见解释和内部原因码。

adjudicator 不负责：

1. 落库。
2. 调度 worker。
3. 直接发通知。

## 3. 建议文件落点

建议新增：

1. `locallife/algorithm/claim_adjudication_types.go`
2. `locallife/algorithm/claim_adjudicator.go`
3. `locallife/algorithm/lookback_tri_party.go`

建议保留但降级：

1. `locallife/algorithm/claim_auto_approval.go`
2. `locallife/algorithm/lookback_checker.go`

## 4. 新 lookback 契约

## 4.1 输入结构

建议 Go 结构：

```go
type TriPartyLookbackRequest struct {
	OrderID      int64
	UserID       int64
	MerchantID   int64
	RiderID      *int64
	ClaimType    string
	WindowDays   int32
	ReferenceAt  time.Time
	AddressID    *int64
	DeviceID     string
	DeviceFingerprint string
}
```

字段说明：

1. `OrderID` 是当前索赔订单锚点。
2. `UserID`、`MerchantID`、`RiderID` 用于三方同窗比较。
3. `ClaimType` 用于筛选相似异常。
4. `WindowDays` 第一阶段建议只支持 `7`、`30`。
5. `ReferenceAt` 用于固定统计窗口截止时间，避免事务内多次 `now()` 漂移。
6. `AddressID`、`DeviceFingerprint` 不属于核心 lookback 统计，但可作为辅助关联输入一并挂载。

## 4.2 输出结构

建议 Go 结构：

```go
type TriPartyLookbackResult struct {
	OrderAnchor            ClaimOrderAnchor
	CustomerSummary        PartyWindowSummary
	MerchantSummary        PartyWindowSummary
	RiderSummary           *PartyWindowSummary
	SharedSignals          SharedSignalSummary
	ComparisonConclusion   ComparisonConclusion
	ReasonCodes            []string
	UserFacingExplanation  string
	InternalSummary        string
}
```

建议辅助结构：

```go
type ClaimOrderAnchor struct {
	OrderID            int64
	ClaimType          string
	OrderType          string
	OrderStatus        string
	FulfillmentStatus  string
	MerchantID         int64
	RiderID            *int64
	DeliveredAt        *time.Time
	CompletedAt        *time.Time
}

type PartyWindowSummary struct {
	ActorType                  string
	ActorID                    int64
	WindowDays                 int32
	SimilarClaimCount          int32
	DistinctCounterpartyCount  int32
	ConfirmedLiabilityCount    int32
	PlatformFallbackCount      int32
	RecentSampleOrderIDs       []int64
	AbnormalRate               float64
	Narrative                  string
}

type SharedSignalSummary struct {
	SharedDeviceOtherUsers  int32
	SharedAddressOtherUsers int32
	HasSharedDeviceRisk     bool
	HasSharedAddressRisk    bool
	HitCodes                []string
}

type ComparisonConclusion struct {
	PrimaryObservation   string
	SuggestedLiability   string
	RequiresFallbackRule bool
	ConfidenceHint       string
}
```

## 4.3 lookback 输出原则

### 4.3.1 顾客摘要

顾客摘要表达的是：

1. 该顾客在同一时间窗口内，对同类异常发起过多少次索赔。
2. 这些索赔是否分散在不同商户/骑手。
3. 是否存在共享设备、共享地址等辅助风险信号。

它不应表达成：

1. 顾客是否在集中针对当前商户。
2. 顾客是否在集中针对当前骑手。

因为那会把当前订单中心模型重新拉回旧的“顾客怀疑器”。

### 4.3.2 商户摘要

商户摘要表达的是：

1. 当前商户在同一窗口内，被不同顾客因同类异常索赔的次数。
2. 当前商户是否存在同类异常重复暴露。
3. 商户侧历史是否足以构成默认销售责任的支撑证据。

### 4.3.3 骑手摘要

骑手摘要表达的是：

1. 当前骑手在同一窗口内，被不同顾客因配送类异常索赔的次数。
2. 是否存在配送履约链异常集中。
3. 骑手侧历史是否足以构成默认服务责任的支撑证据。

## 4.4 lookback 原因码

建议原因码前缀统一使用 `lb_`。

第一阶段建议支持：

1. `lb_customer_repeat_similar_claims`
2. `lb_customer_dispersed_counterparties`
3. `lb_merchant_repeat_similar_incidents`
4. `lb_rider_repeat_delivery_incidents`
5. `lb_shared_device_detected`
6. `lb_shared_address_detected`
7. `lb_merchant_pattern_stronger_than_customer`
8. `lb_rider_pattern_stronger_than_customer`
9. `lb_customer_pattern_stronger_than_counterparties`
10. `lb_insufficient_tri_party_history`

## 5. adjudicator 契约

## 5.1 输入结构

建议 Go 结构：

```go
type ClaimAdjudicationInput struct {
	OrderContext          ClaimOrderContext
	CompensationContext   ClaimCompensationContext
	EvidenceContext       ClaimEvidenceContext
	RiskHint              *ClaimBehaviorHint
	Lookback              TriPartyLookbackResult
	Association           ClaimAssociationEvidence
	ResponsibilityFacts   ClaimResponsibilityFacts
	EffectSummary         ClaimEffectSummary
}
```

建议配套结构：

```go
type ClaimOrderContext struct {
	OrderID             int64
	UserID              int64
	MerchantID          int64
	RiderID             *int64
	OrderType           string
	OrderStatus         string
	FulfillmentStatus   string
	ClaimType           string
	ClaimReason         string
}

type ClaimBehaviorHint struct {
	Status         string
	ShouldWarn     bool
	WarningCount   int
	PlatformPayCount int
	Message        string
}

type ClaimAssociationEvidence struct {
	DistinctDevices          int32
	DistinctAddresses        int32
	SharedDeviceOtherUsers   int32
	SharedAddressOtherUsers  int32
	HitCodes                 []string
}

type ClaimResponsibilityFacts struct {
	DeliveryExists        bool
	RiderAssigned         bool
	PickupConfirmed       bool
	DeliveryCompleted     bool
	MissingCriticalFacts  []string
}

type ClaimEffectSummary struct {
	UserPlatformFallbackClaims    int64
	UserMaliciousConfirmedClaims  int64
	MerchantEffectiveLiability    int64
	RiderEffectiveLiability       int64
}
```

## 5.2 输出结构

建议 Go 结构：

```go
type ClaimAdjudicationResult struct {
	DecisionMode                    string
	ResponsibleParty                string
	ResponsibilityDomain            string
	CompensationSource              string
	ApprovedAmount                  int64
	ClaimLifecycleStatus            string
	DecisionLifecycleStatus         string
	RequiresCustomerConfirmation    bool
	CustomerAction                  string
	ShouldCompensate                bool
	ShouldCreateRecovery            bool
	RecoveryTarget                  string
	RecoveryAmount                  int64
	ShouldRestrictAfterCompensation bool
	ReasonCodes                     []string
	TraceSummary                    string
	UserFacingReason                string
	Warning                         string
	LookbackSnapshot                TriPartyLookbackResult
}
```

字段语义：

1. `DecisionMode` 是正式责任模式真值。
2. `ClaimLifecycleStatus` 是对 claim 主对象的外部状态写入建议。
3. `DecisionLifecycleStatus` 是对 `behavior_decision` 的内部状态写入建议。
4. `RequiresCustomerConfirmation` 只在高风险顾客坚持索赔场景下为真。
5. `ShouldCompensate` 表示判责结果是否支持继续赔偿。
6. `ShouldCreateRecovery` 与 `RecoveryTarget` 一起驱动后续 compensation tx。
7. `ShouldRestrictAfterCompensation` 只表示后续动作资格，不表示提交阶段立即执行。

## 5.3 adjudicator 决策优先级

建议按以下顺序做正式判责：

1. 先按 claim type 确定责任域。
2. 再看三方 lookback 哪一侧模式更强。
3. 再看履约事实是否支持 rider 责任。
4. 再看商户/骑手历史责任是否支撑默认归责。
5. 最后才看顾客风险提示和共享关联信号。

关键原则：

1. 顾客风险信号不能继续直接推导“平台兜底”。
2. 责任不清晰时，按责任域默认落商户或骑手。
3. 高风险顾客路径应该输出“要求确认继续”，而不是立即执行限制。

## 5.4 正式责任模式

建议保留现有四种主模式，但重新定义触发语义：

1. `merchant_recovery`
2. `rider_recovery`
3. `platform_fallback`
4. `user_restricted`

新的语义：

### `merchant_recovery`

适用：

1. 商品质量类异常。
2. 商户同类异常模式强于顾客侧风险。
3. 或责任不清晰但属于销售侧默认责任。

### `rider_recovery`

适用：

1. 配送履约类异常。
2. 骑手履约事实和同类异常模式支持骑手责任。
3. 或责任不清晰但属于服务侧默认责任。

### `platform_fallback`

适用范围必须大幅收缩。

只建议用于：

1. 当前订单缺失关键上下文，无法安全归到商户或骑手。
2. 订单本身数据损坏或跨链缺失，且短期内无法补偿责任认定。

它不再用于：

1. 低置信度默认兜底。
2. 低责任分默认兜底。
3. 顾客风险高但系统仍想赶快结案时兜底。

### `user_restricted`

新的语义不是“提交即拉黑”。

新的语义应是：

1. 本次顾客风险高。
2. 平台先展示判责原因。
3. 只有顾客确认继续，才进入赔偿。
4. 赔偿完成后再执行 restriction。

## 6. 状态转移契约

## 6.1 claim 生命周期

建议：

1. 提交后判责完成且无需确认：`adjudicated`
2. 判责完成且等待顾客确认：`warned_waiting_customer_confirmation`
3. 已进入赔偿创建阶段：`awaiting_compensation`
4. worker 已开始处理：`compensating`
5. 赔付完成：`compensated`
6. 高风险赔付后限制完成：`restricted_compensated`
7. 顾客撤回或不继续：`closed`

## 6.2 decision 生命周期

建议：

1. 提交同步判责后：`decided`
2. 高风险等待顾客确认：`waiting_customer_confirmation`
3. 已允许进入赔偿：`compensation_ready`
4. 已创建 payout action：`compensation_started`
5. 赔偿闭环完成：`compensation_finished`
6. 撤回关闭：`closed`

## 7. 顾客确认继续索赔契约

## 7.1 输入

建议 API 语义：

```go
type ConfirmClaimContinueRequest struct {
	ClaimID int64 `json:"claim_id"`
}
```

## 7.2 输出

建议返回：

```go
type ConfirmClaimContinueResponse struct {
	ClaimID             int64   `json:"claim_id"`
	Status              string  `json:"status"`
	DecisionStatus      string  `json:"decision_status"`
	CompensationStatus  string  `json:"compensation_status"`
	Reason              string  `json:"reason"`
	Warning             *string `json:"warning,omitempty"`
}
```

## 7.3 前置条件

只有满足下面条件才允许确认继续：

1. claim 当前状态是 `warned_waiting_customer_confirmation`
2. decision 当前状态是 `waiting_customer_confirmation`
3. claim 属于当前顾客
4. 没有已存在的 compensation action

## 8. 原因码建议

建议分三类原因码：

### 8.1 lookback 原因码

前缀：`lb_`

### 8.2 责任事实原因码

前缀：`rf_`

建议：

1. `rf_delivery_chain_missing`
2. `rf_rider_assignment_missing`
3. `rf_pickup_confirmation_missing`
4. `rf_delivery_history_supports_rider`
5. `rf_merchant_history_supports_merchant`
6. `rf_customer_risk_requires_confirmation`
7. `rf_domain_default_to_merchant`
8. `rf_domain_default_to_rider`

### 8.3 决策结果原因码

前缀：`dm_`

建议：

1. `dm_merchant_recovery`
2. `dm_rider_recovery`
3. `dm_platform_fallback_exception_only`
4. `dm_user_confirmation_required`
5. `dm_restrict_after_compensation`

## 9. 对现有类型的兼容建议

## 9.1 `Decision`

当前 [locallife/algorithm/claim_types.go](locallife/algorithm/claim_types.go) 的 `Decision` 已经不够表达新的状态边界。

建议：

1. 不继续给 `Decision` 打补丁。
2. 新增 `ClaimAdjudicationResult` 作为正式判责结果。
3. `Decision` 只在旧兼容链路存在期间保留。

## 9.2 `ClaimBehaviorResult`

建议保留，但重命名语义为 hint，而不是正式判责结果。

更合适的新名字：

1. `ClaimBehaviorHint`

## 9.3 `LookbackResult`

建议旧类型保留在兼容层，不再扩展。

新主类型应改为：

1. `TriPartyLookbackResult`

## 10. 一句话结论

新的实现契约应该是：lookback 只产出当前订单中心三方窗口事实，adjudicator 作为唯一正式判责入口消费这些事实并输出 `ClaimAdjudicationResult`，高风险顾客路径通过 `RequiresCustomerConfirmation` 和后续 compensation tx 实现“先判责说明、再确认继续、再赔付并限制”的闭环，而不是继续复用当前 `Decision` 和旧顾客中心 lookback 直接拼业务动作。