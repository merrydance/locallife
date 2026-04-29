# 运营商能力闭环任务卡

日期：2026-04-29

## 使用方式

这些任务卡来自 `operator-capability-backend-audit-2026-04-29.md`。每张卡都要求先验证后端能力是否完整可用，再决定 weapp 是否承接。不要直接按接口平铺页面；所有前端承接都必须先定义运营商任务、ViewState、确认/失败/恢复状态。

执行标准：后续涉及 weapp 或 web 承接时，先按 `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md` 识别用户任务、任务域和 ViewState，再按 `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`、`.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md` 和 `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md` 落地。运营商小程序属于非顾客侧工具型页面，默认 TDesign-first、page shell 稳定、无顶部解释性大卡片、无全局提示条堆叠、无 API 字段平铺。

## 已确认产品边界

日期：2026-04-29

| 能力 | 已确认边界 | 前端承接口径 |
| --- | --- | --- |
| 微信支付投诉 | 只读监管/异常关注 | 可选只读视图，不做客服待办，不提供处理工作台 |
| 追偿争议 | 只读审计 | 可选只读审计视图，不做人工裁决入口 |
| 商户/骑手 | 只读 | 收口管理/审批/处置语义，只展示后端真实字段和诊断事实 |
| 分账配置 | 只读事实 | 可在 finance 中展示只读事实，不做配置入口 |
| `operators/me/rules` 通用规则代理 | 不进任何前端，未来评估是否保留 | 不进入 operator weapp/web；后续单独评估后端保留价值 |
| 补差/分账金额/receiver 删除 | 后端自动执行 | 不进前端；另归系统资金域后端审计 |
| 规则、运费、高峰时段 | 先做 OP-CAP-07 剩余后端验证 | 后端验证完成前不推进 weapp 页面调整 |
| Admin 运营商治理 | 未来专项 | 不纳入本轮 operator weapp；后续单独做 admin 专项 |

## 当前执行顺序

1. OP-CAP-07A/07B/07C/07D：先完成规则、运费、天气、高峰时段剩余后端验证和消费矩阵。
2. OP-CAP-05A/05B 与 OP-CAP-06：在后端规则验证后，收口 operator weapp 中商户/骑手只读边界和死契约。
3. OP-CAP-01A/01B、OP-CAP-02A/02B、OP-CAP-03A：按只读监管/只读审计/只读事实准备，不进入主导航或待办语义。
4. OP-CAP-04A、OP-CAP-09A-E、OP-CAP-10A-D：作为后续专项或后台治理任务，不抢占当前执行队列。

## P0 / G2-G3 高风险回归与边界确认

### OP-CAP-01A 微信支付投诉后端回归

风险：G3，外部微信回调 + 用户投诉。

目标：验证投诉域作为微信支付投诉通知/同步记录的后端闭环，不把它升级成运营商客服处理能力。

后端范围：

- `locallife/api/complaint.go`
- `locallife/db/query/wechat_complaint.sql`
- 投诉同步 worker。

已知事实：

- 投诉 webhook 已做微信签名校验，不应重复作为缺陷。
- 后端 operator 有查看待处理和完结入口，但产品边界不能直接等同为 operator weapp 缺口。

验收标准：

- 后端测试覆盖 webhook 签名失败、重复通知、未知投诉入队同步、operator 完结权限。
- 文档记录投诉状态枚举与前端状态映射。

2026-04-29 关闭进展：

- 投诉 webhook 成功处理后写入 `wechat_notifications`，使 `CheckNotificationExists` 幂等检查有真实落点；重复通知会在签名验证和幂等检查后直接成功返回。
- 未知投诉状态通知会入队 `payment:sync_complaints` 拉取完整微信投诉记录；同步任务分发失败时返回 `FAIL` 让微信侧可重试，避免本地没有记录也吞掉通知。
- 已补充 `api/complaint_test.go` 覆盖签名失败、重复通知短路、未知投诉入队同步并记录通知，以及既有运营商完结权限。

### OP-CAP-01B 微信支付投诉只读监管视图预案

风险：G2/G3，只读暴露外部投诉记录。

目标：仅在需要时准备 operator 只读监管/异常关注视图，不进入待办、客服或处理工作台。

weapp 范围：

- 不进主待办入口。
- 若承接，只能围绕“投诉记录、商户响应状态、微信同步状态、异常关注”做只读 ViewState。
- 不提供回复、完结、客服处理或状态修改动作。

设计验收：

- 页面先定义任务域为“监管异常关注”，不是“处理投诉”。
- 首屏只加载异常关注所需字段，不按投诉接口字段平铺。
- loading、empty、first-screen error、refresh error、只读详情均有明确 ViewState。
- 不使用顶部解释性大卡片或全局 notice bar 解释只读边界；只读边界用状态标签、字段 note 或动作缺席表达。

### OP-CAP-02A 追偿争议自动裁决后端回归

风险：G2，追偿/补偿/处罚后处理。

目标：确认追偿争议由系统自动裁决和 worker 后处理，不需要运营商人工介入。

后端范围：

- `locallife/api/recovery_dispute.go`
- `locallife/logic/recovery_dispute.go`
- `locallife/logic/recovery_dispute_query.go`
- `locallife/worker/task_automatic_recovery_dispute_resolution.go`
- `locallife/worker/task_process_recovery_dispute_result.go`
- `locallife/api/claim_recovery.go`

已知事实：

- 创建端已校验商户/骑手与 claim 归属，先前“权限缺失”判断经复核不成立。
- 后端在提交后调用 `autoResolveRecoveryDisputeBestEffort`，失败时入队自动裁决重试任务。
- 2026-04-29 已清理顾客索赔提交后的旧商户异物/骑手餐损异步风控派发；legacy worker 入口保留消费能力但不再执行商户外卖暂停、骑手暂停或站内信副作用，判责与后处理以行为追溯主链为准。

验收标准：

- 明确 operator 不做人工裁决；不需要展示则从 weapp 差距中移除。
- 后端测试覆盖自动裁决触发、worker 后处理、重复任务、失败恢复和状态不倒退。

2026-04-29 关闭进展：

- 复核现有测试已覆盖商户/骑手创建争议后的自动裁决 best-effort、失败时后台任务兜底、已 resolved 争议重放后处理、释放/继续追偿/补偿/处罚失败传播，以及追偿状态不倒退保护。
- 聚焦回归 `go test ./api ./logic ./worker -run 'RecoveryDispute|ProcessRecoveryDispute|AutomaticRecoveryDispute' -count=1` 已通过；未发现需要新增 operator 人工入口。

### OP-CAP-02B 追偿争议只读审计视图预案

风险：G2，只读暴露追偿争议与后处理状态。

目标：仅在需要时准备 operator 只读审计视图，不提供人工裁决、释放、继续追偿、补偿或处罚入口。

weapp 范围：

- 不进入待办。
- 列表只读展示争议历史、自动裁决状态和 worker 后处理状态。
- 详情页只读展示 claim、order、recovery、appellant、reason、review_notes、自动裁决来源和后处理状态。

设计验收：

- 任务域命名为“审计/观察”，不是“处理/裁决”。
- 所有状态来自后端契约，不用本地推断自动裁决结果。
- 失败态保留上次可信数据并提供重试，不用 Toast-only 承接核心数据失败。

## P1 / G2 中优先级

### OP-CAP-03A 平台级分账配置只读事实

风险：G1/G2，分账配置只读信息。

目标：明确分账配置是平台级配置，运营商只需要可见，不拥有配置能力。分账金额、receiver 删除不属于运营商能力，不进入 operator weapp。

后端范围：

- `locallife/api/operator_profit_sharing_config.go`

weapp 范围：

- operator finance 可展示只读配置事实，或不承接。

验收标准：

- 不提供分账金额手工查询、receiver 删除、补差等操作入口。
- 若展示分账配置，必须是只读事实，不暗示运营商可修改资金链路；不得渲染成配置表单、开关或编辑入口。
- 若不展示，在能力文档中标注为系统自动资金域，不作为 weapp 缺口。

2026-04-29 关闭进展：

- 后端 `GET /v1/operators/me/profit-sharing/configs` 只读查询 `ListProfitSharingConfigsForRegion`，返回本区域和全局配置事实，不提供 operator 写入口。
- weapp 运营商侧没有对应配置表单、receiver 删除、补差或手工分账金额查询入口；保持不承接为默认缺口。
- 聚焦回归 `go test ./api -run 'TestListOperatorProfitSharingConfigsAPI|TestListProfitSharingConfigs' -count=1` 已通过。

### OP-CAP-04A operators/me 规则代理后端保留评估

风险：G2，规则发布影响业务策略。

目标：解释并确认 `operators/me/rules` 的产品边界。它是通用规则引擎的区域代理发布入口，不等同于当前 `/operator/rules`、运费、高峰时段等轻量配置；当前不进入任何前端，未来单独评估是否保留后端能力。

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

- 不承接到 operator weapp、web/operator 或其他前端入口。
- 从当前前端差距和任务队列中移出。
- 后续若评估保留，需要单独回答：真实生产调用点、规则治理 owner、发布风险、是否仍需要运营商区域代理入口。

2026-04-29 关闭进展：

- 后端 `operators/me/rules` 仍是通用规则引擎区域代理能力，读写 `rules`、`rule_versions`、`rule_audits`、`rule_hits`，不是轻量区域规则 `/v1/operator/rules`。
- 当前 weapp 仅承接 `/v1/operator/rules` 轻量配置；未发现 `operators/me/rules` 前端入口，符合“不进当前前端队列”的边界。
- 聚焦回归 `go test ./api -run 'TestListOperatorRulesProxyAPI|TestCreateOperatorRuleVersionProxyAPI|TestListOperatorRuleHitsProxyAPI|TestDBRulesEngineEvaluate' -count=1` 已通过。

### OP-CAP-05A 收口商户/骑手管理语义

风险：G1，能力边界漂移。

目标：商户/骑手当前由系统自动管理，不属于运营商处理能力。operator weapp 保留时只能是只读事实/诊断，不得继续呈现管理、审批、暂停、恢复、能力配置或待办语义。

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

- 不提供审批、暂停、恢复、能力标签处理等运营商动作。
- dashboard 移除“待办审批”语义，不再把商户/骑手展示为运营商待处理事项。
- 页面标题、入口名称、空态和按钮文案不得出现“管理/审批/处置”暗示。

2026-04-29 关闭进展：

- 已将 operator dashboard 的商户/骑手入口收口为“商户档案”“骑手档案”，移除“待办审批”聚合和商户/骑手待审跳转。
- 商户/骑手页面标题收口为档案/档案详情，只保留只读查看语义。
- 提交：`811ba1e1 fix operator weapp readonly merchant rider views`。

### OP-CAP-05B 商户/骑手只读字段契约收口

风险：G1，能力边界漂移 + 前端伪字段。

目标：保留只读页面时，只展示后端真实返回或明确从 stats endpoint 获取的字段。

weapp 范围：

- `weapp/miniprogram/api/operator-merchant-management.ts`
- `weapp/miniprogram/services/operator-merchant-management.ts`
- `weapp/miniprogram/pages/operator/merchants/index.wxml`
- `weapp/miniprogram/pages/operator/merchants/detail/index.wxml`
- `weapp/miniprogram/api/operator-rider-management.ts`
- `weapp/miniprogram/services/operator-rider-management.ts`
- `weapp/miniprogram/pages/operator/riders/index.wxml`
- `weapp/miniprogram/pages/operator/riders/detail/index.wxml`

验收标准：

- 清点列表和详情页所有展示字段，逐项标注后端来源。
- 删除评分、车辆、紧急联系人、GMV/佣金、区域名等后端未返回且未由 stats endpoint 提供的伪字段。
- adapter 不再用 `0`、`--`、固定文案伪造能力事实。
- 保留 loading、empty、first-screen error、refresh error、只读详情状态。
- 视觉上遵守非顾客侧 TDesign-first 和 page shell 规则，不使用顶部解释性大卡片说明“只读”。

2026-04-29 关闭进展：

- 已将 weapp operator merchant/rider API 与 service view model 收口到后端 list/detail 真实字段；经营统计仍仅来自独立 stats endpoint。
- 已删除商户评分、分类、GMV/佣金、区域名、联系人/营业时间/佣金费率，以及骑手评分、车辆、紧急联系人等伪字段展示。
- `npm run compile`、`npm run lint`、`npm run quality:check` 已通过。

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

2026-04-29 关闭进展：

- 已删除 weapp `operator-basic-management.ts` 中不存在的 `GET/PATCH /v1/operators/me` wrapper、`OperatorResponse`/`UpdateOperatorRequest`、以及依赖该死契约的 `getOperatorDashboard` helper。
- `npm run compile`、`npm run lint`、`npm run quality:check` 已通过。

### OP-CAP-07A 运费配置消费链路回归

风险：G2，价格和配送体验。

目标：确认 operator 运费配置变更能实际影响运费预览和下单结算，不停留在配置读写层。

后端范围：

- `locallife/api/operator_rules.go`
- `locallife/api/delivery_fee.go`
- `locallife/db/query/region_rule_config.sql`
- `locallife/db/query/delivery_fee_config.sql`

2026-04-29 关闭进展：

- `RIDER_DEPOSIT` 已从旧运营商字段读取收敛为区域规则主链消费：`region_rule_configs.rider_deposit` -> 全局平台配置 `rider_deposit_fen` -> 系统默认 `DefaultRiderDepositThresholdFen`。
- `/v1/operator/rules` 更新 `RIDER_DEPOSIT` 时继续记录兼容性的运营商 scope 配置，同时写入 `region_rule_configs` 并触发区域骑手运营状态重算；旧 `operator.rider_deposit` 和运营商 scope 平台配置不再参与主链阈值解析。
- 已覆盖的验证：区域优先、平台兜底、系统默认、区域更新后骑手降级/升级重算、骑手上线/状态/工作台读取阈值、API 支付查询测试契约对齐。
- 运费金额计算已通过 `calculateDeliveryFeeInternal` 验证区域运费配置、天气最终系数和高峰时段系数共同影响最终运费；订单预览、下单、购物车预览和搜索估价均通过同一个 `server.calculateDeliveryFeeInternal` adapter 消费该结果。

验收标准：

- 测试覆盖区域权限、配置更新、非法 key 拒绝。
- 测试覆盖运费预览或下单结算读取区域运费配置。
- 文档列出运费相关 rule key 的消费点和验证命令。

### OP-CAP-07B 天气系数消费链路回归

风险：G2，价格和配送体验。

目标：确认 `region_rule_configs` 中天气系数被实际运费计算消费，且缺配置时按平台/系统口径兜底。

后端范围：

- `locallife/api/operator_rules.go`
- `locallife/weather/coefficient.go`
- `locallife/db/query/region_rule_config.sql`

2026-04-29 关闭进展：

- `weather.CalculateCoefficient` 已验证按 `region_rule_configs` 优先、城市平台配置兜底、系统默认兜底计算轻/中/重/极端天气基础系数。
- `/v1/operator/rules/:key` 更新天气倍数后会失效该区域 Redis 天气缓存，避免旧缓存系数在 TTL 内继续参与价格计算。
- `calculateDeliveryFeeInternal` 在无缓存命中时消费 `weather_coefficients.final_coefficient`；天气调度器写入该字段时使用 `weather.CalculateCoefficient` 的区域规则结果。

验收标准：

- 测试覆盖轻/中/大/极端天气系数读取和兜底。
- 测试覆盖区域配置缺失、系数为零或非法输入时的行为。
- 文档列出天气系数消费点和验证命令。

### OP-CAP-07C 高峰时段消费链路回归

风险：G2，价格和配送体验。

目标：确认 operator 高峰时段配置被实际配送费或调度相关计算消费，且区域权限和删除行为正确。

后端范围：

- `locallife/api/operator_rules.go`
- `locallife/api/delivery_fee.go`
- 高峰时段相关 query 和消费点。

2026-04-29 关闭进展：

- `calculateDeliveryFeeInternal` 读取 `peak_hour_configs`，按当前星期和时间匹配活动配置，支持跨日时段，并取最大命中系数参与最终运费。
- 已补主链测试覆盖高峰时段系数对最终运费的影响；高峰时段新增、列表、权限和非法时段已有 API 测试继续保护。

验收标准：

- 测试覆盖新增、列表、删除、跨区域拒绝和非法时段拒绝。
- 测试覆盖高峰时段对下游价格或调度计算的影响。
- 文档列出高峰时段消费点和验证命令。

### OP-CAP-07D 轻量规则 key 消费矩阵

风险：G1/G2，配置项漂移。

目标：把 `/v1/operator/rules` 每个 rule key 的后端来源、消费点、兜底和验证证据列成矩阵；没有消费点的 key 必须标记为只读/待接入，不能继续伪装成可生效配置。

后端范围：

- `locallife/api/operator_rules.go`
- `locallife/api/delivery_fee.go`
- `locallife/weather/coefficient.go`
- `locallife/db/query/region_rule_config.sql`
- `locallife/db/query/delivery_fee_config.sql`

验收标准：

- 矩阵覆盖 `RIDER_DEPOSIT`、运费参数、天气系数、高峰时段等现有 key。
- 每个 key 至少记录：读写表、更新入口、主流程消费点、兜底顺序、测试文件、验证命令、剩余风险。
- 无消费点 key 从前端可编辑配置中下线或标记为待接入。

2026-04-29 消费矩阵：

| rule key | 读写表/事实源 | 更新入口 | 主流程消费点 | 兜底顺序 | 测试证据 | 剩余风险 |
| --- | --- | --- | --- | --- | --- | --- |
| `RIDER_DEPOSIT` | `region_rule_configs.rider_deposit` | `PATCH /v1/operator/rules/RIDER_DEPOSIT` | `db.GetEffectiveRiderDepositThreshold`，骑手上线/状态/工作台 | 区域配置 -> 全局平台配置 -> 系统默认 | `api/operator_deposit_rules_test.go`、`api/rider_test.go`、`logic/rider_workbench_test.go` | 无已知主链缺口 |
| `BASE_DELIVERY_FEE`、`BASE_DISTANCE`、`EXTRA_FEE_PER_KM`、`MIN_DELIVERY_FEE`、`MAX_DELIVERY_FEE`、`DELIVERY_VALUE_RATIO` | `delivery_fee_configs` | `PATCH /v1/operator/rules/:key` | `server.calculateDeliveryFeeInternal`，被订单预览/下单/购物车/搜索估价复用 | 区域配置 -> 全局平台默认 -> 系统默认 | `api/delivery_fee_test.go` | 仍可补更高层订单 handler 回归，但核心 adapter 已同源 |
| `WEATHER_COEFF_EXTREME`、`WEATHER_COEFF_HEAVY`、`WEATHER_COEFF_MODERATE`、`WEATHER_COEFF_LIGHT` | `region_rule_configs`，天气调度写 `weather_coefficients.final_coefficient`，Redis weather cache | `PATCH /v1/operator/rules/:key` | `weather.CalculateCoefficient` -> `weather_coefficients.final_coefficient` -> `server.calculateDeliveryFeeInternal` | 区域配置 -> 城市平台配置 -> 系统默认；价格计算优先 Redis，未命中读 DB | `weather/coefficient_test.go`、`api/delivery_fee_test.go`、`api/operator_deposit_rules_test.go` | 天气调度外部和风 API 未在本轮跑集成 |
| `PEAK_HOUR_COEFFICIENTS` | `peak_hour_configs` | `POST /v1/operator/regions/:region_id/peak-hours`、`DELETE /v1/operator/peak-hours/:id` | `server.calculateDeliveryFeeInternal` 当前时段匹配后参与最终运费 | 无命中 -> `1.0` | `api/delivery_fee_test.go` | 依赖系统当前时间，已用全天配置覆盖主链 |
| `PLATFORM_COMMISSION` | `profit_sharing_configs` | 不可由运营商更新 | 分账配置事实只读展示；不属于轻量规则写入口 | 无活动配置 -> 只读展示 `0` | `api/operator_deposit_rules_test.go` | 只读事实，不作为 OP-CAP-07 可编辑配置 |
| `WEATHER_COEFFICIENT` | `weather_coefficients.final_coefficient` | 只读，系统天气调度更新 | `server.calculateDeliveryFeeInternal` 当前天气系数 | 无天气记录 -> `1.0` | `api/operator_deposit_rules_test.go`、`api/delivery_fee_test.go` | 只读事实，外部天气抓取未在本轮跑集成 |

验证命令：

- `go test ./api -run 'TestCalculateDeliveryFeeInternal_ConsumesRegionWeatherAndPeakRules|TestUpdateOperatorRule_WeatherCoefficientInvalidatesWeatherCache' -count=1`

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

### OP-CAP-09A 食安重复 resolve 回归

风险：G3，商户暂停/恢复。

目标：验证食安案件重复 resolve 不会重复恢复、重复通知或造成状态倒退。

后端范围：

- `locallife/api/operator_food_safety_cases.go`
- `locallife/api/risk_management.go`
- `locallife/db/sqlc/tx_food_safety.go`
- `locallife/integration/food_safety_case_integration_test.go`

验收标准：

- 重复 resolve 有稳定业务响应或幂等结果。
- 不重复清理非本案件拥有的暂停原因。

### OP-CAP-09B 食安跨区域权限回归

风险：G3，商户暂停/恢复 + 区域权限。

目标：验证跨区域 operator 不能调查或恢复不属于自己区域的食安案件。

验收标准：

- 测试覆盖列表、详情、investigate、resolve 的跨区域拒绝。

### OP-CAP-09C 食安调查材料缺失回归

风险：G3，恢复门槛。

目标：验证缺调查报告、缺整改说明或缺必要证据时不能错误恢复商户/订单。

验收标准：

- 测试覆盖缺关键材料时的业务错误和状态不变。

### OP-CAP-09D 食安非食安暂停保护回归

风险：G3，状态恢复误伤。

目标：保留并扩展“解除食安不清除非食安暂停”的集成保护。

验收标准：

- 保留既有测试。
- 覆盖商户、外卖、订单维度的非食安暂停不被误恢复。

### OP-CAP-09E 食安只读/状态页预案

风险：G2/G3，weapp 承接高风险状态。

目标：若未来继续承接 operator 食安案件页面，详情页必须区分处理中、失败、已恢复、不可恢复状态。

验收标准：

- 不用 Toast-only 承接核心数据失败。
- 有重复提交保护和回读路径。
- 不用解释性大卡片堆说明。

### OP-CAP-10A Admin 运营商审批与角色同步

风险：G2，角色/区域/receiver 同步。

目标：后续 admin 专项中验证运营商申请审批能同步 operator、user_role 和 operator_regions。

后端范围：

- `locallife/api/operator_application_admin.go`
- `locallife/logic/operator_status_service.go`
- `locallife/logic/profit_sharing_receiver_lifecycle_service.go`
- `locallife/db/sqlc/tx_operator_application.go`

验收标准：

- 审批通过、拒绝、重复审批、失败回滚都有测试证据。

### OP-CAP-10B Admin 运营商状态变更回归

风险：G2，角色/区域/状态同步。

目标：后续 admin 专项中验证暂停、恢复、合同到期失活不会造成角色或区域权限漂移。

验收标准：

- 暂停、恢复、失活和重复操作都有测试证据。

### OP-CAP-10C Admin 区域扩展审批回归

风险：G2，跨区域权限。

目标：后续 admin 专项中验证区域扩展审批能同步 operator_regions，并被后续统计、规则、派单监控读取。

验收标准：

- 区域申请审批、拒绝、重复审批和跨区域读取都有测试证据。

### OP-CAP-10D Admin receiver lifecycle 可追踪性

风险：G2/G3，分账接收方生命周期。

目标：后续 admin 专项中验证 receiver present/absent intent 与实际 worker 处理状态可追踪。

验收标准：

- receiver intent、worker 执行、失败重试和最终状态可查询。
- 不强塞到 operator weapp；web/admin 或后台治理界面另行承接。

## 关闭任务的统一要求

- 每张卡关闭时回填：变更文件、验证命令、剩余风险。
- G2/G3 任务没有测试证据不得标记完成。
- 前端承接不得用后端字段平铺页面；必须说明目标用户任务和 ViewState。
- 不修改与任务无关的用户地址页变更。