# 运营商后端能力闭环审计

日期：2026-04-29

## 审计目标

本文件把已识别的运营商能力域落入后端审计基线，判断每个能力是否已经形成可用闭环，以及是否接入主业务流程。审计重点不是接口数量，而是能力是否从路由、权限、业务逻辑、持久化、异步/回调、前端承接和验证证据形成完整链路。

## 判定口径

- 完整：路由、handler、权限边界、store/sqlc 或 logic、状态语义、主流程消费点和测试证据均能在源码中对应。
- 部分：路由和基础读写存在，但缺少主流程消费、失败恢复、测试覆盖、前端任务承接或权限/状态闭环。
- 缺失：后端无对应生产入口，或仅有未接入的代码壳。
- 风险等级：按工程治理口径粗分 G0-G3。支付、分账、补差、回调、争议处置、食安恢复、跨区域权限默认按 G2/G3 视角复核。

## 运营商能力边界校正

以下能力不再作为运营商侧能力承接，也不作为 operator weapp 缺口：

| 能力 | 边界结论 | 后续处理 |
| --- | --- | --- |
| 商户/骑手管理 | 当前由系统自动管理，运营商不处理商户和骑手；后期若新增商户巡检，可单独定义“证照与实地巡查核对”任务域 | 从 operator 能力矩阵移除；现有 weapp 商户/骑手入口应收口或改为只读诊断，不再作为待办能力 |
| 分账金额/删除接收方 | 分账金额计算和 receiver 生命周期由系统自动完成，不是运营商应具备的操作能力 | 从 operator 能力矩阵和 weapp 缺口移除；保留为系统资金域内部审计对象 |
| 补差创建/退回/取消 | 补差用于平台级代金券发放；退回和取消应在用户消费后退款链路中自动完成，不属于运营商能力 | 从 operator 能力矩阵和任务卡移除；另归入平台券/退款自动化资金域审计 |
| 追偿争议/追偿详情 | 商户/骑手发起后由后端自动评议和 worker 后处理，运营商不做人工裁决 | 从 operator weapp 待办缺口移除；如需展示，仅作为只读可观察/审计信息 |
| 投诉 | 来源是微信支付投诉通知/同步记录，核心处理是商户响应和微信侧状态同步；运营商不应默认承担客服处理工作 | 不作为默认 operator weapp 缺口；如需展示，应定义为只读监管/异常关注视图 |
| 分账配置 | 平台级配置，运营商只需要可见，不是运营商配置能力 | 不按配置工作台承接；只读展示可选 |
| operators/me 规则代理 | 通用规则引擎的区域代理发布入口，和当前运费/时段等轻量运营规则不是同一层级 | 不作为默认 operator weapp 缺口；若要开放需先确认产品边界和规则治理模式 |
| Admin 运营商申请/状态/区域申请 | 平台后台治理运营商生命周期，不是运营商自己在 weapp 里的能力 | 保留为后台治理闭环审计，不进入 operator weapp 承接范围 |

## 总览结论

| 能力域 | 后端闭环 | 主流程接入 | weapp 承接 | 风险 | 结论 |
| --- | --- | --- | --- | --- | --- |
| 运营商入驻申请 | 完整 | 已接入审批、角色、区域、receiver intent | 已承接注册页 | G2 | 可用，需持续验证审批后角色/分账接收方链路 |
| 区域扩展 | 完整 | 已接入 admin 审批与 operator_regions | 已承接 | G1 | 可用 |
| 区域、统计、实时、趋势、排行 | 完整 | 查询真实区域/订单/商户/骑手事实 | 已承接 dashboard/analytics | G1 | 可用 |
| 待接单派单监控 | 完整 | 接入 deliveries/orders/merchants 和超时提醒 worker | 已承接 dispatch hall | G2 | 可用，需保留超时告警验证 |
| 运营商通知 | 完整 | 已被派单超时等 worker 写入 | 已承接 | G1 | 可用 |
| 规则、运费、高峰时段 | 完整 | 运费/天气/高峰/骑手押金规则有消费点 | 已承接 rules/delivery-fee/timeslot | G2 | 可用，区域押金和运费类配置已验证主链消费；天气规则更新会失效区域天气缓存 |
| 食安案件 | 完整 | 上报、熔断、调查、恢复链路有集成测试 | 已承接 | G3 | 可用，高风险路径需持续回归 |
| 追偿争议/追偿详情 | 完整 | 接入商户/骑手发起、自动评议、worker 后处理 | 不默认承接；只读可观察可选 | G2 | 后端自动裁决，不需要运营商人工介入 |
| 投诉 | 完整 | 微信支付投诉回调/同步、商户响应、状态完结 | 不默认承接；只读监管可选 | G3 | 微信支付投诉记录，不是默认运营商处理任务 |
| 财务概览/佣金 | 完整 | 查询区域经营与分账统计事实 | 部分承接 finance | G1 | 可用 |
| 分账配置 | 完整 | 查询平台级配置事实 | 可只读查看 | G1/G2 | 不是运营商配置能力，只读可见即可 |
| operators/me 规则代理 | 完整 | 版本、发布、回滚、禁用、命中查询具备区域约束 | 不进任何前端，未来评估是否保留后端能力 | G2 | 通用规则工作台能力，不等于当前轻量运营规则配置 |
| Admin 运营商申请/状态/区域申请 | 完整 | 接入 operator 状态、user_role、receiver sync | web/admin 范畴 | G2 | 平台后台治理链路，不属于 operator weapp |

## 能力域审计明细

### 1. 运营商入驻申请

源码入口：`locallife/api/server.go`、`locallife/api/operator_application.go`、`locallife/api/operator_application_admin.go`、`locallife/db/query/operator_application.sql`、`locallife/db/sqlc/tx_operator_application.go`。

接口：

- `POST /v1/operator/application`
- `GET /v1/operator/application`
- `PUT /v1/operator/application/region`
- `PUT /v1/operator/application/basic`
- `DELETE /v1/operator/application/documents/:document_type`
- `POST /v1/operator/application/submit`
- `POST /v1/operator/application/reset`
- Admin 审批、拒绝、状态流转接口。

闭环判断：完整。草稿、基础资料、区域、材料、提交、重置、admin 审批链路齐全。审批后会创建/更新 operator、operator_regions、user_role，并记录分账 receiver 意图。

主流程接入：已接入。该能力不是孤立申请表，而是接入登录角色、区域权限和后续分账接收方生命周期。

剩余风险：这是 G2 链路，审批后角色同步、receiver intent 后续处理和失败恢复应作为回归验证重点。

### 2. 区域扩展

源码入口：`locallife/api/operator_region_expansion.go`、`locallife/db/query/operator_region_application.sql`。

接口：

- `POST /v1/operator/region-expansion`
- `GET /v1/operator/region-expansion`
- Admin 区域申请审批/拒绝接口。

闭环判断：完整。申请、列表、审批后写入 operator region 关系具备源码路径。

主流程接入：已接入 operator_regions，后续统计、规则、商户/骑手、派单监控均依赖区域权限。

### 3. 区域、统计、实时、趋势、排行

源码入口：`locallife/api/operator_stats.go`、`locallife/api/operator_realtime.go`、`locallife/db/query/operator_stats.sql`、`locallife/db/query/operator_finance.sql`。

接口：

- `GET /v1/operator/regions`
- `GET /v1/operator/regions/:region_id/stats`
- `GET /v1/operator/stats/realtime`
- `GET /v1/operator/merchants/ranking`
- `GET /v1/operator/riders/ranking`
- `GET /v1/operator/trend/daily`

闭环判断：完整。查询基于区域、订单、商户、骑手和分账/经营事实。

主流程接入：读侧能力，接入真实业务表，不需要额外状态机。

weapp 承接：dashboard、analytics 已承接；dashboard 中围绕商户/骑手的“待办审批”语义应移除，因为商户/骑手当前由系统自动管理，不属于运营商待处理能力。

### 4. 待接单派单监控

源码入口：`locallife/api/operator_dispatch_monitor.go`、`locallife/db/query/operator_dispatch_monitor.sql`、`locallife/scheduler/operator_dispatch_alert.go`、`locallife/worker/task_operator_pending_dispatch_alert.go`。

接口：

- `GET /v1/operator/regions/:region_id/delivery-pool/summary`
- `GET /v1/operator/regions/:region_id/delivery-pool`

闭环判断：完整。读侧从 `deliveries`、`orders`、`merchants` 聚合待接单配送单；调度器和 worker 会基于超时扫描写入运营商通知。

主流程接入：已接入配送单状态和通知链路。

验证重点：需要保留 scheduler/worker 的定时扫描、重复告警、区域收件人过滤测试。

### 5. 运营商通知

源码入口：`locallife/api/operator_notification.go`、通知相关 query、派单告警 worker。

接口：

- `GET /v1/operators/me/notifications`
- `GET /v1/operators/me/notifications/summary`
- `GET /v1/operators/me/notifications/:id`
- `POST /v1/operators/me/notifications/:id/read`
- `POST /v1/operators/me/notifications/read-all`

闭环判断：完整。列表、摘要、详情、已读状态齐全。

主流程接入：已被待接单超时等后台链路写入，不只是手工消息表。

### 6. 规则、运费、高峰时段

源码入口：`locallife/api/operator_rules.go`、`locallife/api/delivery_fee.go`、`locallife/db/query/region_rule_config.sql`、`locallife/db/query/delivery_fee_config.sql`、`locallife/weather/coefficient.go`。

接口：

- `GET /v1/operator/rules`
- `PATCH /v1/operator/rules/:key`
- `POST /v1/operator/regions/:region_id/peak-hours`
- `GET /v1/operator/regions/:region_id/peak-hours`
- `DELETE /v1/operator/peak-hours/:id`
- delivery-fee operator group 下区域运费配置读写接口。

闭环判断：完整。规则读写、运费配置、高峰期配置存在区域权限边界，并有运费、天气系数、骑手押金消费点。

主流程接入：已接入运费计算、天气系数、高峰时段和骑手押金门槛。骑手押金运行时读取顺序已收敛为 `region_rule_configs.rider_deposit` -> 全局平台配置 `rider_deposit_fen` -> 系统默认 `DefaultRiderDepositThresholdFen`；旧 `operator.rider_deposit` 和运营商 scope 平台配置不再作为主链读取来源。

关闭证据：2026-04-29 已更新 `locallife/db/sqlc/rider_status_helpers.go`、`locallife/api/operator_rules.go`、`locallife/logic/rider_workbench.go` 及对应测试。随后补齐 `calculateDeliveryFeeInternal` 的区域运费配置、天气最终系数、高峰时段系数主链回归，并在天气规则更新后失效对应区域天气缓存，避免 Redis 旧系数继续影响价格；本次未改 SQL 源，不需要 `make sqlc`。

验证命令：`go test ./api -run 'TestCalculateDeliveryFeeInternal_ConsumesRegionWeatherAndPeakRules|TestUpdateOperatorRule_WeatherCoefficientInvalidatesWeatherCache' -count=1`。

### 7. 商户/骑手管理与商户能力（已移出运营商能力）

源码入口：`locallife/api/operator_merchant_rider.go`、`locallife/db/query/operator_core.sql`、`locallife/db/query/operator_rider_stats.sql`、`locallife/db/query/merchant_system_label.sql`、`locallife/db/sqlc/tx_merchant_capabilities.go`。

接口：

- `GET /v1/operator/merchants`
- `GET /v1/operator/merchants/summary`
- `GET /v1/operator/merchants/:id`
- `GET /v1/operator/merchants/:id/stats`
- `GET /v1/operator/merchants/:id/capabilities`
- `PATCH /v1/operator/merchants/:id/capabilities`
- `GET /v1/operator/riders`
- `GET /v1/operator/riders/summary`
- `GET /v1/operator/riders/:id`
- `GET /v1/operator/riders/:id/stats`

边界结论：当前不作为运营商能力。商户和骑手由系统自动管理，运营商不处理商户/骑手审批、暂停、恢复、能力标签等动作。后续若新增商户巡检，应另起“证照与实地巡查核对”能力域，重新定义巡检对象、证据、状态机和后端主流程。

闭环判断：作为运营商能力已移除；作为系统内部管理能力仍需由后端对应业务域维护。

weapp 处理方向：operator merchant/rider 页面不再作为能力承接缺口推进。现有入口应收口、隐藏或改为极窄只读诊断；不得继续展示后端未返回字段，也不得暗示运营商可以审批或处置商户/骑手。

2026-04-29 关闭证据：operator weapp 已将商户/骑手入口和页面收口为“档案”只读语义，移除 dashboard “待办审批”聚合；列表/详情展示字段已对齐后端 list/detail 契约和独立 stats endpoint，删除评分、车辆、紧急联系人、GMV/佣金、区域名等伪字段。验证：`npm run compile`、`npm run lint`、`npm run quality:check`。

### 8. 食安案件

源码入口：`locallife/api/operator_food_safety_cases.go`、`locallife/api/risk_management.go`、`locallife/db/sqlc/tx_food_safety.go`、`locallife/integration/food_safety_case_integration_test.go`。

接口：

- `GET /v1/operator/food-safety/cases`
- `GET /v1/operator/food-safety/cases/:id`
- `POST /v1/operator/food-safety/cases/:id/investigate`
- `POST /v1/operator/food-safety/cases/:id/resolve`

闭环判断：完整。用户上报、达到阈值后创建案件、暂停商户/外卖/订单、运营商调查、结案恢复商户与订单暂停状态，均有集成测试覆盖。

主流程接入：已接入风险上报、订单暂停和商户状态恢复链路。

风险校正：曾怀疑食安解除可能误恢复非食安暂停，但已有 `TestFoodSafetyCaseResolutionDoesNotClearNonFoodSafetySuspension` 一类集成测试覆盖，应作为既有防线保留。

2026-04-29 关闭证据：`UpdateFoodSafetyCaseInvestigation` 已增加 `status <> 'resolved'` 条件，避免并发结案后又写回调查中；API 单测覆盖详情、investigate、resolve 跨区域拒绝、重复结案稳定 400、并发结案后 investigate 拒绝；集成测试覆盖重复 resolve 不重复恢复，并扩展商户级、外卖级、订单级非食安暂停保护。验证：`go test ./api -run 'Test.*OperatorFoodSafetyCase' -count=1`、`go test ./integration -run 'TestFoodSafetyCase' -count=1`。

### 9. 追偿争议/追偿详情

源码入口：`locallife/api/recovery_dispute.go`、`locallife/logic/recovery_dispute.go`、`locallife/logic/recovery_dispute_query.go`、`locallife/worker/task_automatic_recovery_dispute_resolution.go`、`locallife/worker/task_process_recovery_dispute_result.go`。

接口：

- `POST /v1/merchant/recovery-disputes`
- `GET /v1/merchant/recovery-disputes`
- `GET /v1/merchant/recovery-disputes/:id`
- `POST /v1/rider/recovery-disputes`
- `GET /v1/rider/recovery-disputes`
- `GET /v1/rider/recovery-disputes/:id`
- `GET /v1/operator/recovery-disputes`
- `GET /v1/operator/recovery-disputes/summary`
- `GET /v1/operator/recovery-disputes/:id`
- `GET /v1/operator/recoveries/:id`

闭环判断：完整。创建端会校验 claim 归属和争议窗口；提交后会调用 `autoResolveRecoveryDisputeBestEffort`，由系统根据行为判责快照自动评议，并通过 worker 执行释放、继续追偿、补偿或处罚等后处理。

主流程接入：已接入索赔追偿、争议、释放/继续追偿、补偿/处罚后处理。2026-04-29 已清理 `SubmitClaim` 中旧商户异物/骑手餐损异步风险任务派发，legacy worker 入口仅消费并记录忽略日志，不再绕过行为追溯主链触发商户外卖暂停或骑手暂停。

关闭证据：已更新 `locallife/api/risk_management.go`、`locallife/worker/task_risk_management.go`、`locallife/algorithm/claim_auto_approval.go` 及相关测试；`tx_claim_behavior_test.go` 补充顾客与责任方原始行为快照断言，确保判责输入继续来自行为追溯主链。

weapp 承接：不作为运营商人工处理缺口。若产品需要让运营商看到争议历史，应定义为只读可观察/审计视图，而不是待处理工作台。

### 10. 投诉

源码入口：`locallife/api/complaint.go`、`locallife/db/query/wechat_complaint.sql`、微信投诉同步 worker。

接口：

- `GET /v1/merchant/complaints`
- `GET /v1/merchant/complaints/summary`
- `GET /v1/merchant/complaints/:id`
- `POST /v1/merchant/complaints/:id/response`
- `POST /v1/merchant/complaints/:id/complete`
- `GET /v1/operators/me/complaints`
- `POST /v1/operators/me/complaints/:id/complete`
- `POST /v1/webhooks/wechat-ecommerce/complaint-notify`

闭环判断：完整。该域来源是微信支付投诉通知和同步记录：投诉回调读取 body 后做 `VerifyNotificationSignature`，再解密投诉通知；商户可回复/完结，系统同步微信侧状态。后端保留了运营商查看待处理与完结入口，但产品上不应直接等同为运营商客服处理任务。

主流程接入：已接入微信投诉回调和同步任务。

weapp 承接：不作为默认 operator weapp 缺口。若要给运营商看，应先定义为微信支付投诉的只读监管/异常关注视图，避免做成待办客服台。

2026-04-29 关闭证据：投诉 webhook 成功处理后会记录 `wechat_notifications`，重复通知可基于通知 ID 短路；未知投诉会入队同步任务拉取完整投诉详情，同步入队失败时返回 `FAIL` 触发微信重试。验证：`go test ./api -run 'TestHandleComplaintNotify|TestCompleteComplaintAPI' -count=1`。

### 11. 财务概览、佣金、分账配置

源码入口：`locallife/api/operator_stats.go`、`locallife/api/operator_profit_sharing_config.go`、`locallife/db/query/operator_finance.sql`、`locallife/db/query/profit_sharing_config.sql`。

接口：

- `GET /v1/operators/me/finance/overview`
- `GET /v1/operators/me/commission`
- `GET /v1/operators/me/profit-sharing/configs`

闭环判断：完整。读侧查询经营、佣金和平台级分账配置事实。

主流程接入：查询主业务事实，不是写侧能力。分账配置是平台级配置，不是运营商配置项；运营商只需要看见适用于本区域/全局的配置事实。

weapp 承接：finance 页面承接 overview/commission；profit-sharing configs 只读展示可选，不应作为配置工作台缺口。

2026-04-29 关闭证据：`GET /v1/operators/me/profit-sharing/configs` 仅返回本区域和全局分账配置事实，operator 侧无写入口；weapp operator 未提供配置表单、receiver 删除、补差或手工分账金额查询入口。验证：`go test ./api -run 'TestListOperatorProfitSharingConfigsAPI|TestListProfitSharingConfigs' -count=1`。

### 12. 分账金额、接收方删除、补差（已移出运营商能力）

源码入口：`locallife/api/profit_sharing_capability.go`、`locallife/api/subsidy.go`、`locallife/api/payment_callback_profit_sharing_fact.go`、`locallife/logic/profit_sharing_receiver_lifecycle_service.go`、`locallife/db/query/subsidy_order.sql`、`locallife/db/query/profit_sharing_order.sql`。

接口：

- `GET /v1/operators/me/payment-orders/:id/profit-sharing/amounts`
- `POST /v1/operators/me/payment-orders/:id/profit-sharing/receivers/delete`
- `POST /v1/operator/payment-orders/:id/subsidies`
- `POST /v1/operator/payment-orders/:id/subsidies/return`
- `POST /v1/operator/payment-orders/:id/subsidies/cancel`

边界结论：不作为运营商能力。分账金额计算、receiver 生命周期由系统自动完成；补差用于平台级代金券发放，退回和取消应在用户消费后退款链路自动完成。

闭环判断：作为运营商能力已移除；作为系统资金域内部能力仍需单独审计。

已证实缺口：

- 补差入口未复用 `paymentOrderSupportsProfitSharingCapability` 之类能力预检，非电商分账订单可能到微信侧才失败。
- 补差成功只记录 external payment command，未形成与分账回调事实同等级的 external payment fact 链路。是否设计为 command-only 需要产品/对账契约明确，否则不能称为资金事实闭环。
- `cancelSubsidy` 的微信失败和非 SUCCESS 分支不写本地失败状态，只返回错误；可观测性和重试恢复弱于 create/return。
- `returnSubsidy` 使用 `SR-{payment_order_id}`，当前 create 又限定同一支付单同一商户一笔补差，因此现有模型下不构成多补差冲突；若未来放开多补差，需要先改幂等键。

weapp 处理方向：operator weapp 不承接分账金额、receiver 删除、补差创建/退回/取消；这些能力按后端自动执行和系统资金域后端审计处理，不进入前端。

### 13. operators/me 规则代理

源码入口：`locallife/api/rules_operator_proxy.go`、`locallife/api/rules_engine_db.go`、`locallife/db/query/rules.sql`、`locallife/db/query/rules_read.sql`、`locallife/db/query/rule_hits.sql`、`locallife/db/migration/000110_add_rules_engine.up.sql`、`locallife/db/migration/000111_add_rule_hits.up.sql`。

接口：

- `GET /v1/operators/me/rules`
- `GET /v1/operators/me/rules/:id`
- `POST /v1/operators/me/rules`
- `POST /v1/operators/me/rules/:id/versions`
- `POST /v1/operators/me/rules/:id/publish`
- `POST /v1/operators/me/rules/:id/rollback`
- `POST /v1/operators/me/rules/:id/disable`
- `GET /v1/operators/me/rules/hits`

闭环判断：完整。规则、版本、发布、回滚、禁用、命中查询均存在；发布/回滚会校验 rule/version 归属和 `ruleVersionMatchesRegion`。

和区域规则的关系：它和 `/v1/operator/rules` 是并行的两套规则机制，不是同一张表。

- `/v1/operator/rules` 读写 `region_rule_configs` 和 operator 自身的轻量运营字段，面向运费、天气系数、高峰时段、骑手押金等明确区域运营配置。
- `/v1/operators/me/rules` 读写通用规则引擎表：`rules`、`rule_versions`、`rule_audits`、`rule_hits`。规则内容以 JSON 存在 `scope`、`condition`、`action`、`gray_config` 中，通过 `region_id` scope/gray 限定运营商可见和可发布范围。

主流程接入：有接入，但受 `RulesEngineEnabled` 开关控制。服务启动时若启用会使用 `NewDBRulesEngine(store)`；引擎通过 `ListActiveRuleVersions` 读取 `rules.status='active'` 且 `rule_versions.status='published'` 的版本，并按 `scope`、`condition`、`gray_config` 匹配。当前可见调用点包括：

- 下单：`createOrder` 把 `server.rulesEngine` 传入订单命令服务，规则可影响订单创建决策并写 `rule_hits`。
- 索赔/风控：`risk_management.go` 在 claim 相关上下文中调用规则引擎，源码层面支持非 allow 阻断；但按当前顾客索赔设计，有效索赔不应被规则直接判成“不赔”，因此该阻断能力应视为高风险边界/历史漂移，不能作为默认产品语义推广。
- 预约：`table_reservation.go` 在预订成功后用于风险提醒，命中 `alert` 时通知商户。
- 骑手申请：`rider_application.go` 在提交审核后记录规则命中，用于审计。

因此它不是空壳，但它是“通用规则引擎入口”，不是当前 operator 小程序里的轻量区域规则配置。

weapp 承接：不作为默认 operator weapp 缺口。除非产品明确要让运营商在小程序里创建、发版、回滚通用规则，否则应保留在 web/operator 或后台治理界面。

2026-04-29 关闭证据：复核 weapp 仅接入 `/v1/operator/rules` 轻量区域配置，未接入 `operators/me/rules` 通用规则代理；后端代理列表、版本创建、命中查询和 DB rules engine 聚焦回归通过。验证：`go test ./api -run 'TestListOperatorRulesProxyAPI|TestCreateOperatorRuleVersionProxyAPI|TestListOperatorRuleHitsProxyAPI|TestDBRulesEngineEvaluate' -count=1`。

## 已确认前端承接口径

日期：2026-04-29

- 微信支付投诉：只读监管/异常关注，可选只读视图，不做客服待办。
- 追偿争议：只读审计，可选只读视图，不做人工裁决。
- 商户/骑手：只读事实/诊断，收口管理、审批、暂停、恢复和待办语义。
- 分账配置：只读事实，可选展示，不做配置入口。
- `operators/me/rules` 通用规则代理：不进任何前端，未来单独评估是否保留后端能力。
- 补差、分账金额、receiver 删除：后端自动执行，不进前端，另归系统资金域后端审计。
- 规则、运费、高峰时段：先完成 OP-CAP-07 剩余后端验证，再推进 weapp 页面调整。
- Admin 运营商治理：未来另做 admin 专项，不纳入本轮 operator weapp。

## weapp 承接缺口

已承接：工作台、数据分析、派单监控、通知、区域/规则/运费/时段、食安案件、财务概览/佣金、区域扩展、运营商入驻申请。

未承接或后续可选只读：

- 追偿争议与追偿详情：系统自动裁决，不是运营商待办；如承接，只能做只读审计。
- 微信支付投诉记录：来源于微信支付投诉通知/同步，不是默认运营商客服处理；如承接，只能做只读监管/异常关注。
- 分账配置：平台级配置，只读事实展示可选，不做运营商配置入口。
- operators/me 规则代理：通用规则工作台能力，已确认不进任何前端，未来单独评估是否保留后端能力。

承接质量问题：

- 商户/骑手页面已不属于当前运营商处理能力；现有入口应收口为只读事实/诊断，不得展示后端未返回字段，也不得出现管理、审批、暂停、恢复或待办语义。
- dashboard “待办审批”实际只能跳转商户/骑手列表/详情，且商户/骑手处理不属于当前运营商能力，应移除该待办语义。
- `weapp/miniprogram/api/operator-basic-management.ts` 保留后端不存在的 `GET/PATCH /v1/operators/me` wrapper，当前未见页面调用，应清理或改成真实契约。

## 后续验证建议

- 后端：优先补 G2/G3 任务卡的单元测试和集成测试，不用大跑全量。
- weapp：新增高风险操作页前，先完成 ViewState 和失败态设计，避免按 API 平铺按钮。
- 文档：每个任务完成后把“源码闭环证据 + 验证命令 + 残余风险”回填到本文件。