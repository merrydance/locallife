# 运营商能力闭环任务卡

日期：2026-04-29

## 使用方式

这些任务卡来自 `operator-capability-backend-audit-2026-04-29.md`。每张卡都要求先验证后端能力是否完整可用，再决定 weapp 是否承接。不要直接按接口平铺页面；所有前端承接都必须先定义运营商任务、ViewState、确认/失败/恢复状态。

## P0 / G2-G3 高风险回归与边界确认

### OP-CAP-01 微信支付投诉记录边界确认

风险：G3，外部微信回调 + 用户投诉。

目标：确认投诉域是微信支付投诉通知/同步记录，不默认变成运营商客服处理能力；如果要给运营商看，只做只读监管或异常关注视图。

后端范围：

- `locallife/api/complaint.go`
- `locallife/db/query/wechat_complaint.sql`
- 投诉同步 worker。

weapp 范围：

- 默认不新增 operator 待办入口。
- 若承接，只能围绕“微信支付投诉记录、商户响应状态、同步状态、异常关注”做只读 ViewState。

已知事实：

- 投诉 webhook 已做微信签名校验，不应重复作为缺陷。
- 后端 operator 有查看待处理和完结入口，但产品边界不能直接等同为 operator weapp 缺口。

验收标准：

- 明确是否需要 operator 只读查看；不需要则从 weapp 差距中移除。
- 如需查看，weapp 只能展示投诉记录、同步状态、商户处理状态和异常关注，不做默认客服待办。
- 后端测试覆盖 webhook 签名失败、重复通知、未知投诉入队同步、operator 完结权限。
- 文档记录投诉状态枚举与前端状态映射。

### OP-CAP-02 追偿争议自动裁决可观察性确认

风险：G2，追偿/补偿/处罚后处理。

目标：确认追偿争议由系统自动裁决和 worker 后处理，不需要运营商人工介入；如需展示，仅作为只读审计/可观察视图。

后端范围：

- `locallife/api/recovery_dispute.go`
- `locallife/logic/recovery_dispute.go`
- `locallife/logic/recovery_dispute_query.go`
- `locallife/worker/task_automatic_recovery_dispute_resolution.go`
- `locallife/worker/task_process_recovery_dispute_result.go`
- `locallife/api/claim_recovery.go`

weapp 范围：

- 默认不新增 operator 待办入口。
- 若承接，只能做争议历史、自动裁决结果、后处理状态的只读视图。

已知事实：

- 创建端已校验商户/骑手与 claim 归属，先前“权限缺失”判断经复核不成立。
- 后端在提交后调用 `autoResolveRecoveryDisputeBestEffort`，失败时入队自动裁决重试任务。
- 2026-04-29 已清理顾客索赔提交后的旧商户异物/骑手餐损异步风控派发；legacy worker 入口保留消费能力但不再执行商户外卖暂停、骑手暂停或站内信副作用，判责与后处理以行为追溯主链为准。

验收标准：

- 明确 operator 不做人工裁决；不需要展示则从 weapp 差距中移除。
- 如需只读展示，列表能区分自动裁决状态和 worker 后处理状态。
- 详情页展示 claim、order、recovery、appellant、reason、review_notes、自动裁决来源和后处理状态。

## P1 / G2 中优先级

### OP-CAP-03 平台级分账配置只读边界

风险：G1/G2，分账配置只读信息。

目标：明确分账配置是平台级配置，运营商只需要可见，不拥有配置能力。分账金额、receiver 删除不属于运营商能力，不进入 operator weapp。

后端范围：

- `locallife/api/operator_profit_sharing_config.go`

weapp 范围：

- operator finance 的只读配置说明，或不承接。

验收标准：

- 不提供分账金额手工查询、receiver 删除、补差等操作入口。
- 若展示分账配置，必须是只读事实，不暗示运营商可修改资金链路。
- 若不展示，在能力文档中标注为系统自动资金域，不作为 weapp 缺口。

### OP-CAP-04 operators/me 规则代理产品边界

风险：G2，规则发布影响业务策略。

目标：解释并确认 `operators/me/rules` 的产品边界。它是通用规则引擎的区域代理发布入口，不等同于当前 `/operator/rules`、运费、高峰时段等轻量配置。

后端范围：

- `locallife/api/rules_operator_proxy.go`
- `locallife/api/rules_engine_db.go`
- `locallife/db/query/rules.sql`
- `locallife/db/query/rules_read.sql`
- `locallife/db/query/rule_hits.sql`
- 规则 query、规则审计表。

已知事实：

- 发布/回滚已校验版本归属和 `ruleVersionMatchesRegion`，先前“发布未校验区域”判断经复核不成立。
- 这套代理读写 `rules`、`rule_versions`、`rule_audits`、`rule_hits`，不是 `region_rule_configs`。
- 主流程接入受 `RulesEngineEnabled` 控制；当前订单、索赔/风控、预约风险提醒、骑手申请审核后审计会调用通用规则引擎或记录命中。索赔路径的非 allow 阻断只是源码层能力，按当前有效索赔应赔付的业务设计，不能默认作为“规则判不赔”的产品语义，需要单独审计是否应移除或收窄。
- `/operator/rules` 才是当前轻量区域运营规则配置，主要承接运费、天气系数、高峰时段、骑手押金等明确运营参数。

验收标准：

- 默认不承接到 operator weapp。
- 若未来开放给运营商，必须先定义规则治理模式、版本草稿、发布确认、回滚确认、禁用影响提示、命中记录查看。
- 若不开放，应在能力文档标注后端保留给 web/operator 或 admin，并从 weapp 差距中移出产品缺口。

### OP-CAP-05 收口商户/骑手管理入口

风险：G1，能力边界漂移。

目标：商户/骑手当前由系统自动管理，不属于运营商处理能力。收口或隐藏 operator weapp 中相关入口；若保留，只能作为极窄只读诊断。

weapp 范围：

- `weapp/miniprogram/api/operator-merchant-management.ts`
- `weapp/miniprogram/services/operator-merchant-management.ts`
- `weapp/miniprogram/pages/operator/merchants/index.wxml`
- `weapp/miniprogram/pages/operator/merchants/detail/index.wxml`
- `weapp/miniprogram/api/operator-rider-management.ts`
- `weapp/miniprogram/services/operator-rider-management.ts`
- `weapp/miniprogram/pages/operator/riders/index.wxml`
- `weapp/miniprogram/pages/operator/riders/detail/index.wxml`

已知问题：

- 商户列表/详情展示评分、分类、GMV/佣金、区域名等默认字段，后端 list/detail 并不返回。
- 骑手列表/详情展示评分、车辆、紧急联系人等默认字段，后端 operator detail 并不返回。
- dashboard 存在“待办审批”语义，但当前后端和产品边界都不要求运营商处理商户/骑手。

验收标准：

- 移除或隐藏 operator 商户/骑手管理入口和 dashboard 待办审批入口。
- 如保留只读诊断，页面只展示后端真实返回或明确从 stats endpoint 获取的字段。
- adapter 不再用 `0`、`--`、固定文案伪造能力事实。
- 不提供审批、暂停、恢复、能力标签处理等运营商动作。

## P2 / G1-G2 结构完善

### OP-CAP-06 清理死契约 `/v1/operators/me` root wrapper

风险：G1，前端契约漂移。

目标：删除或替换 weapp 中后端不存在的 `GET/PATCH /v1/operators/me` wrapper。

weapp 范围：

- `weapp/miniprogram/api/operator-basic-management.ts`

验收标准：

- 删除未被页面使用的 `getOperatorInfo`、`updateOperatorInfo` 或改为真实后端契约。
- 如果 `getOperatorDashboard` 依赖它，应同步删除或改造。
- `npm run compile` 和相关 lint 不出现类型缺口。

### OP-CAP-07 规则、运费、高峰时段回归验证

风险：G2，价格和配送体验。

目标：确认 operator 规则变更能实际影响运费、天气系数、高峰时段和押金相关流程。

后端范围：

- `locallife/api/operator_rules.go`
- `locallife/api/delivery_fee.go`
- `locallife/weather/coefficient.go`
- `locallife/db/query/region_rule_config.sql`
- `locallife/db/query/delivery_fee_config.sql`

2026-04-29 关闭进展：

- `RIDER_DEPOSIT` 已从旧运营商字段读取收敛为区域规则主链消费：`region_rule_configs.rider_deposit` -> 全局平台配置 `rider_deposit_fen` -> 系统默认 `DefaultRiderDepositThresholdFen`。
- `/v1/operator/rules` 更新 `RIDER_DEPOSIT` 时继续记录兼容性的运营商 scope 配置，同时写入 `region_rule_configs` 并触发区域骑手运营状态重算；旧 `operator.rider_deposit` 和运营商 scope 平台配置不再参与主链阈值解析。
- 已覆盖的验证：区域优先、平台兜底、系统默认、区域更新后骑手降级/升级重算、骑手上线/状态/工作台读取阈值、API 支付查询测试契约对齐。
- 未关闭范围：运费金额计算、天气系数、高峰时段对下单/预览的端到端影响仍按本卡验收标准继续回归。

验收标准：

- 测试覆盖区域权限、规则更新、运费计算消费、非法 key 拒绝。
- 文档列出每个规则 key 的消费点；没有消费点的规则标记为只读/待接入。

### OP-CAP-08 待接单监控与通知链路回归

风险：G2，配送履约。

目标：验证 pending dispatch monitor 不只是读表，超时提醒确实按区域写给运营商。

后端范围：

- `locallife/api/operator_dispatch_monitor.go`
- `locallife/scheduler/operator_dispatch_alert.go`
- `locallife/worker/task_operator_pending_dispatch_alert.go`
- `locallife/api/operator_notification.go`

验收标准：

- 测试覆盖区域过滤、超时阈值、无运营商收件人、重复扫描去重或可接受重复策略。
- weapp dispatch hall 能区分加载失败、空列表和真实无待接单。

### OP-CAP-09 食安案件持续回归防线

风险：G3，商户暂停/恢复。

目标：保留并扩展食安案件从上报到恢复的集成级保护。

后端范围：

- `locallife/api/operator_food_safety_cases.go`
- `locallife/api/risk_management.go`
- `locallife/db/sqlc/tx_food_safety.go`
- `locallife/integration/food_safety_case_integration_test.go`

验收标准：

- 保留“解除食安不清除非食安暂停”的测试。
- 补充弱网/重复 resolve/缺调查报告/跨区域 operator 的测试。
- weapp detail 页有处理中、失败、已恢复和不可恢复状态。

### OP-CAP-10 后台 admin 运营商治理链路回归

风险：G2，角色/区域/receiver 同步。

目标：确保 admin 对运营商申请、状态、区域申请的操作能同步 user_role、operator_regions、receiver lifecycle。

后端范围：

- `locallife/api/operator_application_admin.go`
- `locallife/logic/operator_status_service.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/db/sqlc/tx_operator_application.go`

验收标准：

- 审批、暂停、恢复、合同到期失活、区域扩展审批都有测试证据。
- receiver present/absent intent 和实际 worker 处理状态可追踪。
- web/admin 或 operator web 承接状态显示，不强塞到 weapp operator 侧。

## 关闭任务的统一要求

- 每张卡关闭时回填：变更文件、验证命令、剩余风险。
- G2/G3 任务没有测试证据不得标记完成。
- 前端承接不得用后端字段平铺页面；必须说明目标用户任务和 ViewState。
- 不修改与任务无关的用户地址页变更。