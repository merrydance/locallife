# 运营侧后端能力与小程序实现全量审计（2026-06-20）

> 本文档是运营侧专项审计的主审计文档，工作区为 `audit/operator-backend-weapp-capability-audit`，从 `main` 分支 `fb82393c` 开出。当前阶段先落入已确认的“区域管理待审核假状态”审计结论；后续继续在同一文档中补齐后端能力全量盘点、小程序运营商页面全量盘点、实现缺口与契约漂移统计。

## 审计目标

- 穷尽式盘点运营侧后端能力：路由、handler、logic、SQL/sqlc、事务、worker、scheduler、权限、状态枚举、响应 DTO、Swagger 契约。
- 穷尽式盘点运营商小程序页面：一级、二级、三级、四级及更深页面，含页面、组件、service、api、adapter、状态映射、跳转入口。
- 对齐小程序运营商页面与真实后端能力，统计：
  - 后端已实现但小程序未实现的能力。
  - 小程序已实现但后端没有真实支持的能力。
  - 字段、枚举、状态、权限、分页、错误语义、异步结果等前后端漂移。
  - 假真值、缺失 ViewState、弱网/重入/重复提交风险。
- 将每个 finding 落入可复核证据链，避免只凭截图或页面观感判断。

## 审计口径

- 角色范围：运营商小程序和运营侧后端能力，不包含普通消费者、商户端、骑手端，除非其能力被运营侧页面直接调用或依赖。
- 后端范围：`locallife/api/**`、`locallife/logic/**`、`locallife/db/query/**`、`locallife/db/sqlc/**`、`locallife/worker/**`、`locallife/scheduler/**` 中与 operator/operator regions/operator finance/operator merchant/rider/dispatch/rules/safety/notification/onboarding/withdrawal 等相关链路。
- 小程序范围：`weapp/miniprogram/pages/operator/**`、`weapp/miniprogram/pages/register/operator/**`、`weapp/miniprogram/pages/platform/operators/**` 中影响运营商申请、管理、平台审核或运营角色入口的页面；同时检查被这些页面调用的 `_api`、`_services`、`_main_shared`、shared utils 和全局角色入口。
- 风险分级：按 `.github/instructions/backend-locallife.instructions.md` 和 `.github/instructions/weapp-mini-program.instructions.md` 使用 G0/G1/G2/G3。状态语义、权限、资金、提现、审核、异步确认、重复提交、弱网恢复向上分级。
- 证据要求：每个确认 finding 至少包含后端入口、前端入口、契约或生产数据证据之一；涉及生产状态时只做只读查询。

## 已读规则与参考

- `AGENTS.md`
- `.github/copilot-instructions.md`
- `.github/README.md`
- `.github/instructions/backend-locallife.instructions.md`
- `.github/instructions/weapp-mini-program.instructions.md`
- `.github/prompts/backend-review-closure.prompt.md`
- `.github/prompts/weapp-review.prompt.md`
- `.github/standards/backend/README.md`
- `.github/standards/engineering/README.md`
- `.agents/skills/locallife-human-centered-ui/references/review-rubric.md`
- `.agents/skills/locallife-human-centered-ui/references/weapp-mobile.md`

## 工作区

- 原工作区：`/home/sam/locallife`
- 专项 worktree：`/home/sam/.config/superpowers/worktrees/locallife/operator-backend-weapp-capability-audit`
- 分支：`audit/operator-backend-weapp-capability-audit`
- 基线：`main` at `fb82393c fix: backfill small catering food permit OCR`

## 当前统计

| 类别 | 数量 | 说明 |
| --- | ---: | --- |
| 已确认前后端漂移 | 12 | `OPA-001` 至 `OPA-012` 已落证据链；`OPA-011`、`OPA-012` 来自 2026-06-21 按新增不变量矩阵复核发现 |
| 已确认小程序缺失后端能力 | 3 | 追偿争议/追偿单、分账规则配置、规则引擎代理均有后端路由但无当前运营商小程序入口 |
| 已确认小程序实现但后端不支持 | 2 | 商户搜索、骑手搜索是用户可见承诺但后端不绑定查询条件 |
| 已确认生产数据异常 | 0 | `OPA-001` 已排除生产业务数据异常 |
| 待盘点后端能力 | 55 条路由已建账 | 尚需继续下钻 SQL/sqlc、worker/scheduler、Swagger/schema 和测试覆盖 |
| 待盘点小程序页面 | 24 个 operator 页面已建账 | 尚需继续逐页核 WXML ViewState、弱网恢复、重复提交和端到端截图 |

## 审计执行清单

- [x] 后端运营侧 route group 全量索引。
- [ ] 后端运营侧 DTO/response/status/error/pagination 契约表。
- [ ] 后端运营侧 SQL/sqlc/transaction/worker/scheduler 能力表。
- [x] 小程序运营商页面树全量索引，含一级、二级、三级、四级及更深页面。
- [ ] 小程序页面到 service/api/adapter/backend endpoint 的调用矩阵。
- [ ] 后端能力 vs 小程序实现覆盖矩阵。
- [ ] 小程序字段/状态/权限/错误语义漂移矩阵。
- [ ] findings 分级、修复建议、验证建议和剩余风险。
- [x] 已确认 findings 的逐项修复任务计划。
- [x] 2026-06-21 按新增业务不变量矩阵完成一轮运营侧二轮 review。

## 阶段一后端能力全量索引

索引来源是当前代码 `locallife/api/server.go` 中 `/v1/operator`、`/v1/operators/me` 和 `/v1/operators/me/rules` 的实际注册路由。当前阶段已枚举 55 条运营侧后端路由；本表只标记当前 operator 小程序是否有直接调用，后续还要继续追 handler -> logic -> SQL/sqlc -> worker/scheduler -> test 的闭环。

| 域 | 方法与路径 | Handler | 当前运营商小程序调用 | 阶段结论 |
| --- | --- | --- | --- | --- |
| 区域扩展 | `POST /v1/operator/region-expansion` | `applyOperatorRegionExpansion` | `region-expansion/index` | 已调用 |
| 区域扩展 | `GET /v1/operator/region-expansion` | `listOperatorRegionApplications` | `region-expansion/index` | 已调用 |
| 区域列表 | `GET /v1/operator/regions` | `listOperatorRegions` | dashboard/analytics/region/rules 等区域选择 | 已调用，`OPA-001` |
| 区域统计 | `GET /v1/operator/regions/:region_id/stats` | `getRegionStats` | `analytics/index`、基础管理 API | 已调用，`OPA-004` |
| 调度池 | `GET /v1/operator/regions/:region_id/delivery-pool/summary` | `getOperatorPendingDispatchSummary` | `dispatch-hall/index` | 已调用 |
| 调度池 | `GET /v1/operator/regions/:region_id/delivery-pool` | `listOperatorPendingDispatches` | `dispatch-hall/index` | 已调用 |
| 峰时时段 | `POST /v1/operator/regions/:region_id/peak-hours` | `createPeakHourConfig` | `timeslot/index` | 已调用 |
| 峰时时段 | `GET /v1/operator/regions/:region_id/peak-hours` | `listPeakHourConfigs` | `region/config`、`timeslot/index` | 已调用，`OPA-005` |
| 峰时时段 | `DELETE /v1/operator/peak-hours/:id` | `deletePeakHourConfig` | `timeslot/index` | 已调用 |
| 实时统计 | `GET /v1/operator/stats/realtime` | `getOperatorRealtimeStats` | dashboard/analytics | 已调用，`OPA-006` |
| 排行分析 | `GET /v1/operator/merchants/ranking` | `getOperatorMerchantRanking` | dashboard/analytics | 已调用 |
| 排行分析 | `GET /v1/operator/riders/ranking` | `getOperatorRiderRanking` | dashboard/analytics | 已调用 |
| 趋势分析 | `GET /v1/operator/trend/daily` | `getRegionDailyTrend` | dashboard/analytics | 已调用 |
| 商户管理 | `GET /v1/operator/merchants` | `listOperatorMerchants` | `merchants/index` | 已调用，`OPA-002` |
| 商户管理 | `GET /v1/operator/merchants/summary` | `getOperatorMerchantSummary` | 暂未发现页面调用 | API-only/预留 |
| 商户管理 | `GET /v1/operator/merchants/:id` | `getOperatorMerchant` | `merchants/detail/index` | 已调用 |
| 商户管理 | `GET /v1/operator/merchants/:id/capabilities` | `getOperatorMerchantCapabilities` | `merchants/detail/index` | 已调用 |
| 商户管理 | `PATCH /v1/operator/merchants/:id/capabilities` | `updateOperatorMerchantCapabilities` | `merchants/detail/index` | 已调用 |
| 商户管理 | `GET /v1/operator/merchants/:id/stats` | `getOperatorMerchantStats` | `merchants/detail/index` | 已调用 |
| 骑手管理 | `GET /v1/operator/riders` | `listOperatorRiders` | `riders/index` | 已调用，`OPA-003` |
| 骑手管理 | `GET /v1/operator/riders/summary` | `getOperatorRiderSummary` | 暂未发现页面调用 | API-only/预留 |
| 骑手管理 | `GET /v1/operator/riders/:id` | `getOperatorRider` | `riders/detail/index` | 已调用 |
| 骑手管理 | `GET /v1/operator/riders/:id/stats` | `getOperatorRiderStats` | `riders/detail/index` | 已调用 |
| 食安案件 | `GET /v1/operator/food-safety/cases` | `listOperatorFoodSafetyCases` | `safety/report/index` | 已调用，`OPA-011` |
| 食安案件 | `GET /v1/operator/food-safety/cases/:id` | `getOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，`OPA-011` |
| 食安案件 | `POST /v1/operator/food-safety/cases/:id/investigate` | `investigateOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，`OPA-011` |
| 食安案件 | `POST /v1/operator/food-safety/cases/:id/resolve` | `resolveOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，`OPA-011` |
| 追偿 | `GET /v1/operator/recovery-disputes` | `listOperatorRecoveryDisputes` | 未发现 | 后端已实现，小程序无入口 |
| 追偿 | `GET /v1/operator/recovery-disputes/summary` | `listOperatorRecoveryDisputesSummary` | 未发现 | 后端已实现，小程序无入口 |
| 追偿 | `GET /v1/operator/recovery-disputes/:id` | `getOperatorRecoveryDisputeDetail` | 未发现 | 后端已实现，小程序无入口 |
| 追偿 | `GET /v1/operator/recoveries/:id` | `getOperatorClaimRecovery` | 未发现 | 后端已实现，小程序无入口 |
| 区域规则 | `GET /v1/operator/rules` | `listOperatorRules` | `rules/index` | 已调用 |
| 区域规则 | `PATCH /v1/operator/rules/:key` | `updateOperatorRule` | `rules/index` | 已调用 |
| 财务概览 | `GET /v1/operators/me/finance/overview` | `getOperatorFinanceOverview` | dashboard/finance withdraw | 已调用 |
| 宝付提现 | `GET /v1/operators/me/finance/baofu-withdrawal/balance` | `getOperatorBaofuWithdrawalBalance` | finance withdrawals/create | 已调用 |
| 宝付提现 | `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals` | `listOperatorBaofuWithdrawals` | finance withdrawals list | 已调用 |
| 宝付提现 | `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals/:id` | `getOperatorBaofuWithdrawal` | finance withdrawals detail | 已调用 |
| 宝付提现 | `POST /v1/operators/me/finance/baofu-withdrawal/withdraw` | `createOperatorBaofuWithdrawal` | finance withdrawals create | 已调用 |
| 佣金账单 | `GET /v1/operators/me/commission` | `getOperatorCommission` | finance withdraw/bills | 已调用，`OPA-012` |
| 结算账户 | `GET /v1/operators/me/settlement-account` | `getOperatorBaofuSettlementAccount` | finance settlement-account | 已调用 |
| 结算账户 | `POST /v1/operators/me/settlement-account` | `createOperatorBaofuSettlementAccount` | finance settlement-account submit | 已调用 |
| 分账配置 | `GET /v1/operators/me/profit-sharing/configs` | `listOperatorProfitSharingConfigs` | 未发现 | 后端已实现，小程序无入口；多区域默认口径已加固 |
| 通知中心 | `GET /v1/operators/me/notifications` | `listOperatorNotifications` | notifications/index | 已调用 |
| 通知中心 | `GET /v1/operators/me/notifications/summary` | `getOperatorNotificationSummary` | dashboard/notifications | 已调用 |
| 通知中心 | `GET /v1/operators/me/notifications/:id` | `getOperatorNotification` | notifications/detail | 已调用 |
| 通知中心 | `PUT /v1/operators/me/notifications/:id/read` | `markOperatorNotificationAsRead` | notifications/detail/list | 已调用 |
| 通知中心 | `PUT /v1/operators/me/notifications/read-all` | `markAllOperatorNotificationsAsRead` | notifications/index | 已调用 |
| 规则引擎代理 | `GET /v1/operators/me/rules` | `listOperatorRulesProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `GET /v1/operators/me/rules/hits` | `listOperatorRuleHitsProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `GET /v1/operators/me/rules/:id` | `getOperatorRuleProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `POST /v1/operators/me/rules` | `createOperatorRuleProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `POST /v1/operators/me/rules/:id/versions` | `createOperatorRuleVersionProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `POST /v1/operators/me/rules/:id/publish` | `publishOperatorRuleProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `POST /v1/operators/me/rules/:id/rollback` | `rollbackOperatorRuleProxy` | 未发现 | 后端已实现，小程序无入口 |
| 规则引擎代理 | `POST /v1/operators/me/rules/:id/disable` | `disableOperatorRuleProxy` | 未发现 | 后端已实现，小程序无入口 |

## 阶段一小程序页面树

页面树来源是 `weapp/miniprogram/app.json` 的 `pages/operator` 分包，当前共 24 个页面。深度按 `pages/operator` 下业务路径层级记录。

| 深度 | 页面 | 主要任务 | 主要后端调用 |
| ---: | --- | --- | --- |
| 1 | `analytics/index` | 数据分析、排行、区域体检 | regions、realtime、trend、region stats、merchant/rider ranking |
| 1 | `dashboard/index` | 运营工作台 | regions、finance overview、realtime、rankings、trend、notifications summary |
| 1 | `dispatch-hall/index` | 待接单监控 | delivery-pool summary/list |
| 1 | `notifications/index` | 通知列表、标记已读 | notifications list/read/read-all |
| 2 | `notifications/detail/index` | 通知详情、跳转调度大厅 | notification detail/read |
| 1 | `merchants/index` | 商户列表、搜索、状态过滤 | merchants list |
| 2 | `merchants/detail/index` | 商户详情、能力标签、经营统计 | merchant detail/capabilities/stats |
| 1 | `riders/index` | 骑手列表、搜索、状态过滤 | riders list |
| 2 | `riders/detail/index` | 骑手详情、代取统计 | rider detail/stats |
| 1 | `rules/index` | 区域规则查看与更新 | operator rules list/update |
| 1 | `region/index` | 运营区域列表、选择配置区县 | operator regions |
| 2 | `region/config` | 区域运费/峰时配置入口 | delivery fee config、peak-hours |
| 1 | `timeslot/index` | 峰时时段列表、新增、删除 | peak-hours list/create/delete |
| 1 | `delivery-fee/index` | 区域代取费配置 | delivery-fee region config GET/PATCH/POST |
| 2 | `safety/report/index` | 食安案件列表 | food-safety cases list |
| 2 | `safety/detail/index` | 食安调查与结案 | food-safety detail/investigate/resolve |
| 2 | `finance/withdraw/index` | 收入概览、账单/提现入口 | finance overview、commission |
| 2 | `finance/bills/index` | 佣金账单 | commission |
| 2 | `finance/withdrawals/index` | 提现列表与余额 | baofu balance、withdrawals list |
| 3 | `finance/withdrawals/create/index` | 发起提现 | baofu balance、withdraw |
| 3 | `finance/withdrawals/detail/index` | 提现详情与轮询 | baofu withdrawal detail |
| 2 | `finance/settlement-account/index` | 宝付结算账户状态 | settlement-account GET |
| 3 | `finance/settlement-account/submit/index` | 提交/继续宝付开户 | settlement-account POST + payment workflow |
| 1 | `region-expansion/index` | 申请扩展运营区域 | regions/available、region-expansion GET/POST |

## 已确认缺口与漂移摘要

| 编号 | 类型 | 影响面 | 状态 |
| --- | --- | --- | --- |
| OPA-001 | 前后端契约漂移 | 区域列表状态 | 已修复：`5b642fef` |
| OPA-002 | 小程序承诺但后端不支持 | 商户搜索 keyword | 已修复：`369b33e3` |
| OPA-003 | 小程序承诺但后端不支持 | 骑手搜索 keyword、API 类型 online_status | 已修复：`90e87ac7` |
| OPA-004 | 前后端响应 DTO 漂移 | 分析页区域体检 | 已修复：本次专项修复 |
| OPA-005 | 后端权限边界缺口 | 峰时时段列表跨区域读取 | 已修复：`36c91afa` |
| OPA-006 | 后端状态模型漂移 | 实时统计与骑手生命周期 | 已修复：`35df01de` |
| OPA-007 | 错误路径漂移 | 代取费配置 PATCH 失败无差别 POST | 已修复：`e50b8503`、`6bf2fef9` |
| OPA-008 | 小程序 ViewState/布局漂移 | 商户列表成功态空白 | 已修复：`ee292a34` |
| OPA-009 | 后端区域授权真值漂移 | 调度大厅提示无权限 | 已修复：`6182ade7` |
| OPA-010 | 小程序运行时展示适配漂移 | 财务概览金额只显示 `¥` | 已修复：`6182ade7` |
| OPA-011 | 后端默认区域/多区域漂移 | 食安案件列表、详情、调查、结案 | 已修复：`9e8bef7f`、`632921a8`、本次小程序契约 gate 收口 |
| OPA-012 | 后端默认区域/多区域漂移 | 佣金明细、佣金账单、财务页最近佣金 | 已修复：佣金明细默认聚合全部可管区域 |
| OP-NOUI-001/002 | 后端已实现但无页面入口 | 追偿争议/追偿单 | 当前确认 |
| OP-NOUI-003 | 后端已实现但无页面入口 | 分账规则配置 | 当前确认 |
| OP-NOUI-004 | 后端已实现但无页面入口 | 规则引擎代理 | 当前确认 |
| OP-RISK-001 | 后端-only 残余风险 | 分账配置默认单区域口径 | 已修复：默认聚合全部可管区域，merchant-scoped 配置不跨区泄漏 |
| OP-RISK-002 | API 直接调用残余风险 | `/v1/operator/rules` 无参 legacy 兜底 | 待加固：小程序页面路径已显式传 `region_id` |
| OP-CONTRACT-002/005 | 历史候选 | 商户/骑手 summary `region_id` | 当前代码已支持或无页面调用，不计当前缺陷 |

## 二轮漂移复盘与方法修正

2026-06-21 用户回归继续暴露 `OPA-008` 至 `OPA-010`，说明前几轮专项修复虽然逐项关闭了已知 finding，但审计方法仍然偏“接口/页面/症状逐项核对”，没有把运营商侧抽象成跨后端、小程序运行时、页面 ViewState 的业务不变量矩阵。

这不是单个补丁遗漏，而是审计模型不够强：运营商小程序的可用性依赖 `运营商身份 -> 区域授权 -> 能力入口 -> 后端权限 -> 数据聚合 -> 小程序 ViewModel -> WXML 实际渲染` 同时成立。前几轮 review 已覆盖 route、handler、SQL、service 和页面绑定，但没有把“同一能力的所有入口路径”和“小程序真实运行约束”都机器化成 gate。

### 漂移继续出现的机制性原因

1. 审计粒度偏能力清单和页面切片，而不是业务不变量。`OPA-009` 说明 `/v1/operator/regions` 能展示区域，不等于调度接口的区域授权口径一致。
2. 新旧模型并存但没有统一读取口。`operators.region_id` 和 `operator_regions` 同时存在时，列表、显式区域校验、默认聚合各自实现，容易出现一个路径修复、另一个路径漂移。
3. 同一用户任务有多条技术路径。区域列表、调度大厅、财务聚合、商户列表分别走不同 handler/service/page 状态机，单页回归不会自然覆盖同域同类路径。
4. Review 过度相信编译、接口返回和静态页面绑定。`OPA-010` 的后端数据正确，小程序 WXML 却不能可靠调用 Page formatter，属于“契约正确但运行时适配漂移”。
5. 已知症状修复后，没有立即把同类路径收进自动化矩阵。`OPA-008` 是页面成功态布局/ViewState 问题，和后端无关；如果只按接口真值审计，就会漏掉“数据存在但页面不可见”。

### 运营商能力不变量矩阵

| 不变量 | 必须同时成立的路径 | 已暴露漂移 | 已落地防线 | 后续规则 |
| --- | --- | --- | --- | --- |
| 已审核且有有效区域的运营商应看到可运营入口 | `operators`、`operator_regions`、`GET /v1/operator/regions`、dashboard/region picker | `OPA-001` 已审核区域显示待审核 | `check:operator-region-status-contract` | 禁止小程序用缺失字段兜底成审核态；状态必须来自后端关系真值 |
| 区域权限读取口径应一致 | 显式 `region_id`、默认聚合、legacy primary region、suspended relation | `OPA-009` 调度大厅无权限；`OPA-011` 食安多区域不可见；`OPA-012` 佣金明细与财务概览口径不一致；历史 `OPA-005` 峰时段越权 | 后端 dispatch/finance focused tests | 修改 operator region auth 时必须同时测显式区域、默认聚合、legacy fallback、suspended deny |
| 默认视图语义必须一致 | 列表/总览/明细/最近记录在未选区时是“全部区域”还是“必须选区” | `OPA-011`、`OPA-012` 仍使用 `getOperatorRegionID()`，多区域无参会 403 或落到单一区域 | 本轮新增人工矩阵，待机器化 | 默认列表/总览应优先用 `resolveOperatorRegionSelection()`；必须单区配置/动作则页面必须显式选区且后端 fail closed |
| 页面成功态必须可见 | WXML 分支、scroll 容器高度、空态、加载态、分页态 | `OPA-008` 商户列表成功态空白 | `check:operator-merchant-list-viewstate` | 列表页修复不能只看接口成功，必须验证 success/empty/load-more 都有可见高度 |
| 资金/比例展示由 ViewModel 拥有 | 后端金额 fen、service adapter、Page data、WXML 绑定 | `OPA-010` 财务概览只显示 `¥` | `check:operator-finance-overview-display`、`check:finance-bill-pages` | WXML 不调用金额/比例 formatter；关键展示字符串在 service/view model 中生成 |
| 搜索/筛选/分页必须绑定同一查询条件 | keyword/status/online_status/region/page/count | `OPA-002`、`OPA-003` | 商户/骑手搜索契约脚本和 SQLC/API tests | 列表承诺的筛选项必须有后端绑定、SQL 条件和 stale response guard |
| 写路径错误语义不能 catch-all fallback | PATCH/POST、404、403、400、500、network unknown | `OPA-007` PATCH 任意失败降级 POST | `check:operator-delivery-fee-fallback` | 只有稳定业务错误可触发 fallback；权限失败和网络未知必须停止并保留草稿 |

### 审计方法调整

- 后续不再以“页面是否调用接口”作为完成标准，而以“业务不变量在所有入口路径成立”作为完成标准。
- 每个运营商区域相关修复都要同步检查：区域列表展示、显式 `region_id` 权限、默认聚合、legacy primary region、suspended relation、页面空态。
- 每个小程序页面修复都要同步检查：成功态可见、空态可见、错误态可恢复、旧请求不会覆盖新状态、WXML 不承担关键格式化。
- 文档 finding 只是第一层防线；能机器化的必须进入 `weapp/scripts/check-*` 或后端 focused test，避免靠人工记忆复盘。小程序侧运营商专项脚本入口统一为 `npm run check:operator-capability-audit`。

## Findings

### OPA-001: 运营商区域管理页把已生效区域误显示为“待审核”

- 状态：已修复：`5b642fef`
- 风险级别：G2
- 类型：前后端契约漂移、假业务状态、前端假真值
- 影响页面：`weapp/miniprogram/pages/operator/region/index`
- 影响接口：`GET /v1/operator/regions`
- 生产样本：宁晋县，行政区划代码 `130528`

#### 用户可见现象

运营商小程序“区域管理”页展示：

- 区域：宁晋县
- 描述：`3级区域 | 代码: 130528`
- 状态标签：`待审核`

但该运营商与区域在生产库中均为已审核、已生效状态。

#### 前端证据

页面入口：

- `weapp/miniprogram/pages/operator/region/index.ts`
  - `loadRegions()` 调用 `loadOperatorRegionListItems()`。
- `weapp/miniprogram/pages/operator/region/index.wxml`
  - 列表右侧状态标签渲染 `item.status_label`。

service/adapter 链路：

- `weapp/miniprogram/pages/operator/_services/operator-regions.ts`
  - `loadOperatorRegionListItems()` 调用 `operatorBasicManagementService.getOperatorRegions({ page: 1, limit: 100 })`。
  - 返回值逐项进入 `OperatorBasicManagementAdapter.adaptRegionResponse(item)`。
- `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts`
  - `getOperatorRegions()` 调用 `/v1/operator/regions`。
  - `RegionResponse` 类型声明了可选 `status?: RegionStatus`。
  - `adaptRegionResponse()` 使用 `const status = data.status ?? 'pending'`。
  - `getRegionStatusDisplay()` 将非 `active`/`inactive` 的状态统一显示为 `待审核`。

结论：只要后端不返回 `status`，小程序就会把区域默认为 `pending` 并显示“待审核”。

#### 后端证据

接口入口：

- `locallife/api/server.go`
  - `operatorStatsGroup.GET("/regions", server.listOperatorRegions)`
- `locallife/api/operator_stats.go`
  - `listOperatorRegions()` 优先读取 `operator_regions` 关系表，随后对每个 `rel.RegionID` 再调用 `server.store.GetRegion(ctx, rel.RegionID)`。
  - 响应使用 `newRegionResponse(region)`。
- `locallife/api/region.go`
  - `regionResponse` 字段只有 `id/code/name/level/parent_id/longitude/latitude`。
  - `newRegionResponse()` 没有写入 `status`，也没有写入 `operator_region_status` 或 `operator_id`。

Swagger 证据：

- `locallife/docs/swagger.yaml`
  - `api.regionResponse` 不包含 `status`。
  - `/v1/operator/regions` 返回 schema 是泛化 object，并未声明运营关系状态字段。

SQL 证据：

- `locallife/db/query/operator_region.sql`
  - `ListOperatorRegions` 返回 `or_t.status`，但 handler 丢弃了该 relation status，重新用 `GetRegion` 得到通用行政区 DTO。
- `locallife/db/migration/000033_operator_multi_region.up.sql`
  - `operator_regions.status` 合法值是 `active/suspended`。
- `locallife/db/migration/000106_add_operator_finance_safety.up.sql`
  - `regions.status` 默认值是 `active`，这是行政区可用状态，不是运营商区域审批状态。

结论：后端当前接口没有返回前端所需的运营商区域状态；前端类型假设了 `status`，并把缺失状态降级成审核中。

#### 生产库只读核对

通过 `ssh -p 22333 sam@aliyun`，使用后端同一 `DB_SOURCE` 对生产库只读查询。

宁晋县 `130528` 核对结果：

| 表 | 关键字段 | 结果 |
| --- | --- | --- |
| `regions` | `code=130528`, `name=宁晋县`, `status` | `active` |
| `operators` | `operator_id=1`, `region_id=596`, `status` | `active` |
| `operator_regions` | `operator_id=1`, `region_id=596`, `status` | `active` |
| `operator_applications` | `region_id=596`, `status` | `approved` |
| `operator_region_applications` | `region_id=596` | 无记录 |

综合查询结果：

| operator_id | operator_status | code | region_name | region_catalog_status | operator_region_status | application_status | pending_expansion_count |
| ---: | --- | --- | --- | --- | --- | --- | ---: |
| 1 | active | 130528 | 宁晋县 | active | active | approved | 0 |

结论：生产业务状态正确，不存在真实“待审核”；截图是前端契约兜底造成的假状态。

#### 根因

`/v1/operator/regions` 的实际响应是通用行政区 DTO，不包含运营商区域关系状态。小程序运营商区域管理页却把该接口当作运营商区域关系 DTO 使用，并将缺失的 `status` 默认成 `pending`。这是典型 API/DTO flattening 与后端真值漂移：页面需要的是“当前运营商对该区域的管理关系状态”，但拿到的是“行政区基础信息”。

#### 用户影响

- 已审核运营商看到“待审核”，会误判账号或区域尚未开通。
- 运营区域状态展示不可信，后续区域配置、规则配置、运费、骑手/商户管理入口的状态判断可能被误导。
- 若后续后端返回 `operator_regions.status='suspended'`，当前前端 `RegionStatus` 不认识 `suspended`，仍可能显示为“待审核”，扩大漂移范围。

#### 修复方向

优先后端收口契约：

- 为 `/v1/operator/regions` 定义运营商区域专用 response，例如 `operatorRegionResponse`。
- 明确返回 `region_id/code/name/level/parent_id` 以及 `operator_region_status` 或页面真正需要的 `status`。
- 状态枚举应以 `operator_regions.status` 为源，至少覆盖 `active/suspended`；如果业务需要展示申请状态，则应另有明确字段来自 `operator_region_applications.status`，不能混用行政区状态。
- 更新 Swagger 并补 handler 层测试，锁住状态字段。

前端同步修复：

- `RegionResponse` 不应假设通用行政区接口有审核状态。
- 移除 `data.status ?? 'pending'` 这类假真值兜底；缺失状态应作为契约错误或展示为不影响审核语义的安全降级。
- `RegionStatus` 应对齐后端真实枚举，例如 `active/suspended`，不要使用未被后端支持的 `inactive/pending` 作为运营区域关系状态。

#### 建议验证

- 后端：
  - `go test ./api -run TestListOperatorRegions`
  - 如新增/修改 Swagger 注释，运行 `make swagger`
  - 如改 SQL 查询源，运行 `make sqlc && make check-generated`
- 小程序：
  - `npm run compile`
  - `npm run lint`
  - 对运营商区域页做真机或开发者工具回归：已审核区域显示“运营中”；暂停区域显示明确暂停态；缺失状态不显示“待审核”。

#### 本 finding 已验证与未验证

已验证：

- 前端页面链路：页面 -> service -> api -> adapter -> 状态标签。
- 后端接口链路：route -> handler -> `ListOperatorRegions` -> `GetRegion` -> `newRegionResponse`。
- Swagger 中 `api.regionResponse` 不包含 `status`。
- 生产库宁晋县相关业务状态均为 active/approved，不存在待审核数据。

未验证：

- 未调用线上 HTTP API 抓包确认响应体；当前结论基于当前代码、Swagger 和生产库只读数据。
- 未修改代码，因此未运行后端或小程序测试。
- 未盘点其它运营区域是否同样显示假“待审核”；根据代码路径推断该问题影响所有 `/v1/operator/regions` 返回且没有 `status` 字段的区域。

### OPA-002: 商户列表提供“搜索商户名称、电话”，但后端不绑定 `keyword`

- 状态：已修复：`369b33e3`
- 风险级别：G1
- 类型：小程序能力承诺漂移、查询参数未闭环
- 影响页面：`weapp/miniprogram/pages/operator/merchants/index`
- 影响接口：`GET /v1/operator/merchants`

#### 前端证据

- `merchants/index.wxml` 展示搜索框，placeholder 是“搜索商户名称、电话”。
- `merchants/index.ts` 的 `onSearchChange()` 写入 `searchKeyword` 并触发 `loadMerchants(true)`。
- `_services/operator-merchant-management.ts` 的 `loadOperatorMerchantListPageData()` 把 `keyword: params.searchKeyword || undefined` 放进 `MerchantQueryParams`。
- `_api/operator-merchant-management.ts` 的 `MerchantQueryParams` 声明 `keyword?: string`，`getMerchantList()` 原样发送到 `/v1/operator/merchants`。

#### 后端证据

- `locallife/api/operator_merchant_rider.go` 中 `listOperatorMerchantsRequest` 只绑定 `status/region_id/page/limit`。
- `listOperatorMerchants()` 只使用 `req.Status` 和区域选择结果，查询分支只调用 `ListMerchantsByRegion` 或 `ListMerchantsByRegionWithStatus`。
- 当前后端没有绑定、校验、传递或 SQL 过滤 `keyword`，也没有使用 `sort_by/sort_order/start_date/end_date`。

#### 根因

小程序把搜索作为页面主工具暴露给运营商，但后端列表接口的请求结构和 SQL 查询没有 keyword 条件。前端发送的 `keyword` 被 Gin bind 忽略，最终查询仍返回未搜索的分页列表。

#### 用户影响

- 运营商按商户名称或手机号搜索时，结果实际不按关键词过滤。
- 如果当前第一页恰好没有目标商户，用户会误以为商户不存在或不在管辖范围。
- 分页和搜索组合下尤其容易误判，因为后端返回的是原分页数据。

#### 修复方向

- 后端增加明确的 `keyword` 查询契约，定义可搜索字段、模糊规则、大小写/空白处理和分页计数语义。
- SQL/sqlc 增加按区域集合 + 状态 + keyword 的列表和 count 查询；多区域聚合路径要保持分页顺序稳定。
- 如果短期不做后端搜索，应移除或禁用搜索框，并用不承诺搜索能力的筛选文案替代。

#### 建议验证

- 后端：补 `GET /v1/operator/merchants?keyword=...` 的 handler 测试，覆盖命中、未命中、跨区域不可见、状态组合和分页 count。
- 小程序：搜索商户名称、手机号后，确认返回列表和总数均被过滤；弱网重试不保留错误搜索结果。

#### 本 finding 已验证与未验证

已验证：
- 后端新增 `keyword` 查询契约，trim 后最长 50 字符，匹配商户名称和手机号。
- SQL/sqlc 新增 `ListOperatorMerchants` / `CountOperatorMerchants`，由数据库按 `region_ids + statuses + keyword` 同一条件完成列表与 count。
- handler 不再按区域循环后内存合并分页，分页顺序和 total 由同一查询条件保证。
- 小程序 service 发送 trim 后 keyword；页面新增 `merchantListRequestSeq`，弱网或快速切换搜索条件时旧响应/旧错误不会覆盖新结果。

未验证：
- 未在微信开发者工具或真机手工模拟慢网输入、清空和翻页；自动化已覆盖契约、编译与 lint。

### OPA-003: 骑手列表搜索与在线筛选类型没有后端支持

- 状态：已修复：`90e87ac7`
- 风险级别：G1
- 类型：小程序能力承诺漂移、状态语义混用
- 影响页面：`weapp/miniprogram/pages/operator/riders/index`
- 影响接口：`GET /v1/operator/riders`

#### 前端证据

- `riders/index.wxml` 展示搜索框，placeholder 是“搜索骑手姓名、电话”。
- `riders/index.ts` 的 `onSearchChange()` 写入 `searchKeyword` 并触发 `loadRiders(true)`。
- `_services/operator-rider-management.ts` 的 `loadOperatorRiderListPageData()` 把 `keyword` 放入 `RiderQueryParams`。
- `_api/operator-rider-management.ts` 的 `RiderQueryParams` 声明 `keyword?: string` 和 `online_status?: RiderOnlineStatus`。
- 原始审计时同一文件的 `RiderStatus` 还包含 `offline`，但该生命周期污染已在 OPA-006 中移除；OPA-003 剩余范围聚焦 `keyword` 和 `online_status` 能力承诺。

#### 后端证据

- `locallife/api/operator_merchant_rider.go` 中 `listOperatorRidersRequest` 只绑定 `status/region_id/page/limit`。
- `listOperatorRiders()` 只按 `req.Status` 和一个目标区域查询 `ListRidersByRegion` 或 `ListRidersByRegionWithStatus`。
- OPA-006 后，后端 status binding 已收敛为 `approved/active/suspended`；当前仍没有绑定 `online_status` 或 `keyword`。

#### 根因

前端 service 把搜索关键词和在线状态放进查询参数模型，但后端列表接口只实现生命周期状态过滤和分页。搜索关键词没有进入 handler/SQL，在线状态也没有可信来源与时效语义。

#### 用户影响

- 运营商搜索骑手姓名或手机号时，列表不按搜索词过滤。
- 后续如果页面继续接入 `online_status`，用户会看到无效筛选或误以为在线状态是强实时真值。
- 多区域运营商从 dashboard 带 `region_id` 进入时可限定区域；直接进入骑手页时仍默认主区域，这一点需继续和产品口径确认。

#### 修复方向

- 生命周期状态已由 OPA-006 收敛为 `approved/active/suspended`；OPA-003 不再重新引入申请态或 `offline` 生命周期。
- 后端如支持搜索/在线筛选，应显式绑定 `keyword/online_status` 并新增 SQL/sqlc 查询。
- 如果暂不支持，应删除页面搜索承诺或在页面上改为本地已加载数据内搜索，并清楚标注作用范围；更推荐后端支持。

#### 建议验证

- 后端：补 `GET /v1/operator/riders?keyword=...`、`online_status=online/offline`、非法 status 的测试。
- 小程序：搜索姓名/手机号、切状态 tab、返回重入后验证筛选状态和列表结果一致。

#### 本 finding 已验证与未验证

已验证：
- 后端新增 `keyword` 查询契约，trim 后最长 50 字符，匹配骑手姓名和手机号。
- 后端新增 `online_status=online/offline` 查询契约，明确映射 `riders.is_online` 当前存储值；没有把 `offline` 重新混入生命周期 status。
- SQL/sqlc 新增 `ListOperatorRiders` / `CountOperatorRiders`，由数据库按 `region_ids + statuses + keyword + online_status` 同一条件完成列表与 count。
- `GET /v1/operator/riders` 不传 `region_id` 时改为聚合当前运营商全部可管区域；指定 `region_id` 时仍先校验 operator 管辖权。
- 小程序 service 发送 trim 后 keyword，并移除后端不支持的 `sort_by/sort_order` 参数；`RiderOnlineStatus` 类型收敛为 `online/offline`。
- 小程序页面新增 `riderListRequestSeq`，弱网或快速切换搜索条件时旧响应/旧错误不会覆盖新结果。

未验证：
- 未在微信开发者工具或真机手工模拟慢网输入、清空和翻页；自动化已覆盖契约、编译与 lint。

### OPA-004: 分析页把扁平 `regionStatsResponse` 当成嵌套统计 DTO 使用

- 状态：已修复：本次专项修复
- 风险级别：G2
- 类型：前后端响应 DTO 漂移、页面运行时风险
- 影响页面：`weapp/miniprogram/pages/operator/analytics/index`
- 影响接口：`GET /v1/operator/regions/:region_id/stats`

#### 前端证据

- `_api/operator-analytics.ts` 定义的 `OperatorRegionStatsResponse` 包含 `merchant_stats/rider_stats/order_stats/financial_stats/growth_stats` 等嵌套对象。
- `_services/operator-analytics-dashboard.ts` 中 `loadOperatorAnalyticsPageData()` 调用 `operatorAnalyticsService.getRegionStats()`。
- 同一函数随后读取 `regionStats.merchant_stats.active_merchants`、`regionStats.rider_stats.online_riders`、`regionStats.order_stats.completion_rate`、`regionStats.financial_stats.total_commission`。
- `analytics/index.wxml` 将这些值展示在“区域体检”中。

#### 后端证据

- `locallife/api/operator_stats.go` 的 `regionStatsResponse` 只有 `region_id/region_name/merchant_count/total_orders/total_gmv/total_commission` 六个扁平字段。
- `getRegionStats()` 返回的正是该扁平 response，没有嵌套统计对象。

#### 根因

小程序分析模块曾使用一个比后端更丰富的虚构 DTO。`getRegionStats()` 正常返回 200 时，页面却按嵌套 `merchant_stats/rider_stats/order_stats/financial_stats` 读取字段，因此成功响应也会因 `undefined` 解引用而崩溃。

#### 用户影响

- 选择具体区域后，分析页“区域体检”可能无法渲染，或者整个页面加载失败。
- 如果运行时错误被页面 catch 为通用加载失败，用户会误以为后端不可用，而真实原因是 DTO 漂移。

#### 修复方式

- 前端收口到后端真实契约，不再扩展虚构的嵌套分析 DTO。
- `OperatorRegionStatsResponse` 仅保留 `region_id/region_name/merchant_count/total_orders/total_gmv/total_commission`。
- `loadOperatorAnalyticsPageData()` 新增 `buildOperatorAnalyticsRegionSummary()`，把 `regionStats`、实时统计和页面 ViewModel 做显式 adapter。
- 页面“区域体检”改为展示后端已返回的活跃商户、活跃骑手、订单数和区域佣金。
- `analytics/index.ts` 增加 `analyticsRequestSeq`，连续切区或切时间维度时只接收最后一次请求结果，避免旧响应覆盖新选择。

#### 建议验证

- 小程序：在有区域的运营账号进入 `analytics/index`，切换区域，确认“区域体检”不崩溃且字段来自真实后端响应。
- 后端：本次未扩 DTO，不需要补 Swagger 或 handler 变更。

### OPA-005: 峰时时段列表读取没有校验运营商是否管理该区域

- 状态：已确认
- 风险级别：G3
- 类型：后端授权边界缺口、跨区域数据读取风险
- 影响页面：`weapp/miniprogram/pages/operator/region/config`、`weapp/miniprogram/pages/operator/timeslot/index`
- 影响接口：`GET /v1/operator/regions/:region_id/peak-hours`

#### 前端证据

- `_main_shared/api/delivery-fee.ts` 的 `getPeakConfigs(regionId)` 调用 `/v1/operator/regions/${regionId}/peak-hours`。
- `_services/operator-region-config.ts` 的 `loadOperatorPeakHourViews()` 和 `loadOperatorRegionConfigOverview()` 都会读取该接口。
- `timeslot/index.ts` 从页面参数读取 `region_id` 后调用 `loadPeakConfigs(selectedRegionId)`。

#### 后端证据

- `createPeakHourConfig()` 使用 `checkOperatorManagesRegion(ctx, req.RegionID)` 校验写权限。
- `deletePeakHourConfig()` 先读取配置，再对 `config.RegionID` 调 `checkOperatorManagesRegion()`。
- `listPeakHourConfigs()` 只绑定 path `region_id`，随后直接调用 `ListPeakHourConfigsByRegion(ctx, uri.RegionID)`，没有调用 `checkOperatorManagesRegion()`。

#### 根因

同一资源的写/删路径有区域权限校验，读路径遗漏了相同的区域管理校验。由于路由只要求 operator 角色，任何运营商只要知道其他区域 ID，就可能读取该区域峰时时段配置。

#### 用户影响

- 跨区域运营配置可能被非管理运营商读取。
- 虽然不能直接写入或删除非管理区域配置，但读取本身已经越过租户/区域边界。

#### 修复方向

- 在 `listPeakHourConfigs()` 中与 create/delete 一致调用 `checkOperatorManagesRegion(ctx, uri.RegionID)`。
- 补充 API 测试：同运营商管理区域返回 200，非管理区域返回 403。
- 检查其它 path-region operator GET 是否都做了同等权限校验。

#### 建议验证

- 后端：`go test ./api -run TestListPeakHourConfigs` 或新增 focused test。
- 小程序：区域配置页和峰时配置页在合法区域仍能读取；非法跳转参数展示无权限错误。

### OPA-006: 实时统计与骑手管理混用了“申请状态”和“骑手生命周期状态”

- 状态：已修复：`35df01de`；错误修复 `b1ebb921` 已通过 `51737559` 回滚
- 风险级别：G2
- 类型：状态模型漂移、运营指标失真、区域指标越权风险
- 影响页面：`dashboard/index`、`analytics/index`、`riders/index`
- 影响接口：`GET /v1/operator/stats/realtime`、`GET /v1/operator/riders`、`GET /v1/operator/riders/summary`

#### 前端证据

- `_services/operator-workbench.ts` 和 `_services/operator-analytics-dashboard.ts` 读取 `pending_rider_count`，旧页面文案把它解释为待审骑手。
- `_api/operator-rider-management.ts` 曾把前端 `pending` 规范化成后端 `pending_approval`，`riders/index.wxml` 曾展示“待入驻”tab。
- 这些前端承诺都假设 `riders.status` 存在待审态，但真实骑手生命周期不包含 `pending` 或 `pending_approval`。

#### 后端与生产库证据

- 修复前 `operator_realtime.go` 的 `getOperatorRealtimeStats()` 用 `riders.status = 'pending'` 统计待审骑手，但生产约束不允许该值。
- 修复前 `operator_merchant_rider.go` 的列表和汇总允许 `pending_approval/rejected`，但这些也不是 `riders.status` 的合法生命周期值。
- 生产只读聚合显示：`riders.status` 当前为 `active=2`、`approved=2`；`rider_applications.status` 当前为 `approved=4`、`draft=96`。
- 生产约束显示：`riders_status_check` 只允许 `approved/active/suspended`；`rider_applications_status_check` 只允许 `draft/submitted/approved`。
- `rider_applications` 当前没有 `region_id`，提交态申请不能安全归属到运营商区域统计。

#### 根因

原 finding 把“待审骑手”误归因为 `pending` 与 `pending_approval` 的枚举命名不一致；深入核对后确认这是更上层的状态模型混用：骑手申请表表达申请流程，骑手表表达已形成骑手后的生命周期。运营商区域统计如果直接读取未按区域归属的申请表，会把全局申请状态泄漏进某个运营商的区域指标。

#### 用户影响

- 工作台/分析页旧 `pending_rider_count` 不是可解释的区域指标，可能长期为 0 或诱导运营商理解为“没有待处理申请”。
- 骑手列表旧“待入驻”tab 承诺了后端生命周期不存在的状态，用户筛选后只能得到空结果或错误结果。
- 如果短期用 `rider_applications.submitted` 补数，会产生跨区域/全局申请数量泄漏风险。

#### 已采纳修复方向

- `pending_rider_count` 保留为兼容字段，但明确返回 `0`，直到存在可按运营商区域归属的骑手申请模型。
- 骑手列表、汇总和小程序 tab 全部收敛到真实 `riders.status` 生命周期：`approved/active/suspended`；页面文案从“待入驻”改为“待激活”。
- 小程序 analytics 指标不再展示假的“待审 N”，改为围绕真实 active rider 口径表达。
- 后端测试锁定 realtime 不再查询 `riders.pending`、`riders.pending_approval` 或全局 `rider_applications.submitted`。

#### 后续设计前置条件

如果产品需要运营商审核骑手申请，必须先在申请模型或分配流程中建立区域归属、运营商授权、重复提交和审批时序，再新增 operator rider application review API 与小程序入口；不能在当前区域统计接口里临时拼接全局申请表。

### OPA-007: 代取费配置 PATCH 任意失败都会降级 POST

- 状态：已修复：`e50b8503`、`6bf2fef9`
- 风险级别：G2
- 类型：错误语义漂移、失败路径掩盖
- 影响页面：`weapp/miniprogram/pages/operator/delivery-fee/index`
- 影响接口：`PATCH/POST /v1/delivery-fee/regions/:region_id/config`

#### 前端证据

- `_main_shared/api/delivery-fee.ts` 的 `updateRegionConfig()` 先 PATCH `/v1/delivery-fee/regions/${regionId}/config`。
- `catch (_e)` 捕获任何 PATCH 失败后，不判断错误类型，直接 POST 同一路径创建配置。
- `_services/operator-region-config.ts` 的 `saveOperatorDeliveryFeeConfig()` 调用该方法。

#### 后端证据

- `server.go` 同时注册了 operator 可用的 delivery-fee config POST/PATCH。
- `delivery_fee.go` 中 POST 是创建配置，PATCH 是更新配置；两者是不同业务语义。
- `updateDeliveryFeeConfig()` 在配置不存在时返回 HTTP 404；小程序 `request()` 会把 HTTP status 写入 `AppError.statusCode`，因此前端可以稳定按 `statusCode === 404` 判定是否允许 POST。

#### 根因

前端为了兼容“配置不存在时创建”的便利路径，把 PATCH 的所有失败都当成“缺配置”处理。权限失败、参数错误、后端 500、网络异常等都会触发 POST，从而掩盖原始错误，并可能制造第二个错误或错误写入尝试。

#### 用户影响

- 真实更新失败原因被二次请求覆盖，页面错误提示不稳定。
- 在弱网或后端异常时，用户可能以为系统尝试了正确的创建/更新流程，但真实状态不明。
- 若后端错误分支存在非 404 但 POST 可成功的情况，可能把原本应失败的更新变成创建路径。

#### 修复方向

- 只在明确的 404/配置不存在业务错误上降级 POST。
- 其它错误直接向页面抛出原始语义，由页面显示可恢复错误。
- 页面保存中禁用重复提交，成功后用后端返回值回填表单，失败时保留当前草稿。
- 后端 POST 创建配置支持显式 `is_active=false`，避免首次保存停用状态时被固定写成启用。
- 更推荐后端提供单一 upsert 语义接口，或前端先 GET 状态后选择 POST/PATCH，并保留并发冲突处理。

#### 建议验证

- 小程序：`npm run check:operator-delivery-fee-fallback`，覆盖 404 才 POST、非 404 不 POST、保存按钮防重入和后端返回值回填。
- 小程序：`npm run compile && npm run lint`。
- 后端：`go test ./api -run 'TestCreateDeliveryFeeConfigAPI_AllowsInactiveConfig|Test(Create|Update|Get)DeliveryFeeConfigAPI' -count=1`，确认 POST/PATCH/GET 配置契约。
- 生成物：如修改后端 Swagger 契约，运行 `make swagger && make check-generated`。

### OPA-008: 商户列表接口有数据但小程序成功态不可见

- 状态：已修复：`ee292a34`
- 风险级别：G1
- 类型：小程序 ViewState/布局漂移、成功态不可见
- 影响页面：`weapp/miniprogram/pages/operator/merchants/index`
- 影响脚本：`weapp/scripts/check-operator-merchant-list-viewstate.test.js`

#### 用户可见现象

运营商进入商户列表时，后端已经能按区域和搜索条件返回商户数据，但页面主体看起来是空的。这个现象容易被误判成“没有商户”或“后端没合并”，实际是成功态列表容器没有稳定可见高度。

#### 前端证据

- `merchants/index.wxml` 的成功态列表分支和空态/加载更多分支依赖 scroll 容器承载。
- `merchants/index.wxss` 中内容容器和 scroll-view 高度约束不完整时，成功态可渲染但不可见。
- 该问题和 `GET /v1/operator/merchants` 的后端搜索/分页修复不是同一个问题：接口成功不代表页面成功态可见。

#### 根因

前几轮审计按“service 是否调用后端、接口是否返回、字段是否匹配”收口，但没有把页面成功态、空态和滚动容器高度作为运营商列表页不变量。结果是数据链路已经闭合，ViewState/布局层仍能把可用数据隐藏。

#### 修复方式

- 调整商户列表 WXML 分支，让 `scroll-view` 直接承接成功态列表、空态和加载更多。
- 调整页面内容容器和 `.merchants-scroll` 高度，避免 flex 子节点塌陷。
- 新增 `check:operator-merchant-list-viewstate`，锁定成功态列表和空态必须有可见 scroll 高度。

#### 建议验证

- 小程序：`npm run check:operator-merchant-list-viewstate`
- 小程序：`npm run compile && npm run lint`
- 人工回归：有商户、无商户、搜索无结果、分页加载更多四种状态均需要可见。

### OPA-009: 调度大厅按新区域关系判定无权限，漏兼容已审核运营商主区域

- 状态：已修复：`6182ade7`
- 风险级别：G2
- 类型：后端区域授权真值漂移、legacy/new model 并存漂移
- 影响页面：`weapp/miniprogram/pages/operator/dispatch-hall/index`
- 影响接口：`GET /v1/operator/regions/:region_id/delivery-pool/summary`、`GET /v1/operator/regions/:region_id/delivery-pool`
- 影响后端：`locallife/api/operator_dispatch_monitor.go`、`locallife/api/delivery_fee.go`

#### 用户可见现象

已审核运营商在小程序区域列表或入口中能看到自己的运营区域，但进入调度大厅后提示“无权限”。这说明“区域可见”和“区域可操作”的后端权限读取口径发生漂移。

#### 后端证据

- `/v1/operator/regions` 的展示链路可以暴露运营商主区域。
- 调度大厅接口使用区域授权校验，只认可 active `operator_regions` 关系时，会漏掉仅存在 legacy `operators.region_id` 的已审核主区域。
- 默认聚合 helper 如果无差别纳入 legacy 主区域，也可能把 suspended relation 或历史关系带进统计，因此不能简单全局放开。

#### 根因

`operators.region_id` 与 `operator_regions` 新旧模型并存，但没有统一的“当前运营商可操作区域”读取口。区域列表、调度显式 `region_id`、默认财务/统计聚合各自实现，导致某些路径只看新表，某些路径仍能看旧主区域。

#### 修复方式

- 在区域授权 helper 中增加 legacy primary-region fallback，但仅在该 operator 没有 `operator_regions` 关系时生效。
- suspended relation 仍然拒绝，不能因为 legacy `operators.region_id` 重新获得权限。
- 修正默认聚合 helper，不把 suspended legacy primary region 纳入 finance/stat 聚合。
- 增加 dispatch 和 stats focused tests，覆盖 legacy fallback、active relation、suspended relation、显式区域和默认聚合。

#### 建议验证

- 后端：`go test ./api -run 'Test(GetOperatorPendingDispatch|ListOperatorPendingDispatch|GetOperatorFinanceOverview|GetOperatorCommission)' -count=1`
- 小程序：进入调度大厅，已审核主区域可读；暂停区域或非管理区域显示无权限。
- 复核：所有 operator region auth 修改必须同时覆盖显式 `region_id` 与默认聚合路径。

### OPA-010: 财务概览后端有金额但小程序只显示 `¥`

- 状态：已修复：`6182ade7`
- 风险级别：G2
- 类型：小程序运行时展示适配漂移、资金展示 ViewModel 漂移
- 影响页面：`weapp/miniprogram/pages/operator/finance/withdraw/index`
- 影响接口：`GET /v1/operators/me/finance/overview`、`GET /v1/operators/me/commission`
- 影响脚本：`weapp/scripts/check-operator-finance-overview-display.test.js`

#### 用户可见现象

运营商财务页的概览金额区域只显示货币符号 `¥`，没有显示具体金额。后端并非无数据，问题发生在小程序模板渲染层。

#### 前端证据

- 财务页 WXML 曾在模板中调用 Page formatter，例如金额和比例格式化方法。
- WeChat WXML 对这种 Page 方法调用不可靠，导致表达式不能按普通 TypeScript runtime 的预期执行。
- 资金类展示如果依赖 WXML 临时格式化，编译和 TypeScript lint 均可能通过，但真机/开发者工具显示仍然漂移。

#### 根因

前几轮 review 把“后端返回金额字段、页面有绑定表达式”视为足够，但没有把“小程序运行时是否能实际渲染关键金额字符串”作为 gate。金额/比例属于资金域关键展示，格式化所有权应在 service/view model，而不是 WXML。

#### 修复方式

- `operator-finance.ts` 在 view model 中生成 `totalIncomeDisplay`、`currentMonthIncomeDisplay`、`operatorShareRatioDisplay`、佣金行金额展示等字符串。
- 财务页 WXML 只绑定已格式化 display 字段，不再调用金额/比例 formatter。
- 页面 TS 移除 template-only formatter 方法，避免形成看似可用但运行时不可靠的接口。
- 新增 `check:operator-finance-overview-display`，锁定运营商财务概览和佣金行必须绑定 display 字符串。

#### 建议验证

- 小程序：`npm run check:operator-finance-overview-display`
- 小程序：`npm run check:finance-bill-pages`
- 小程序：`npm run compile && npm run lint`
- 人工回归：财务概览、佣金账单、提现入口均显示完整金额和比例，不出现裸 `¥`。

### OPA-011: 食安案件仍使用单区域默认口径，多区域运营商无法稳定查看和处置

- 状态：待修复
- 风险级别：G3
- 类型：后端默认区域/多区域漂移、状态转移授权边界、页面能力缺口
- 影响页面：`weapp/miniprogram/pages/operator/safety/report/index`、`weapp/miniprogram/pages/operator/safety/detail/index`
- 影响接口：`GET /v1/operator/food-safety/cases`、`GET /v1/operator/food-safety/cases/:id`、`POST /v1/operator/food-safety/cases/:id/investigate`、`POST /v1/operator/food-safety/cases/:id/resolve`

#### 用户可见现象

多区域运营商进入食安案件列表时，小程序不传 `region_id`。后端使用 `getOperatorRegionID()` 解析单一区域：多区域无默认区域时会返回 403；存在 legacy 主区域时只可能看到一个区域的案件。详情、调查、结案同样依赖单一区域比较，运营商可能无法处理自己其它有效区域的食安案件。

#### 后端证据

- `locallife/api/operator_food_safety_cases.go`
  - `listOperatorFoodSafetyCases()` 使用 `server.getOperatorRegionID(ctx)` 后只查询 `ListFoodSafetyCasesByRegion` 或 `ListFoodSafetyCasesByRegionAndStatus`。
  - `getOperatorFoodSafetyCase()`、`investigateOperatorFoodSafetyCase()`、`resolveOperatorFoodSafetyCase()` 都先取单个 `regionID`，再用 `caseRecord.RegionID != regionID` 判定无权限。
  - `resolveOperatorFoodSafetyCase()` 把单一区域 `regionID` 传入 `ResolveFoodSafetyCaseTx`；如果当前 operator 管理案件真实区域但默认解析到另一区域，会被错误拒绝或错误使用区域条件。
- `locallife/api/operator_food_safety_cases_test.go`
  - 现有测试集中在单个 `operator.RegionID`，跨区域测试只验证“案件区域不等于 operator 主区域时拒绝”，没有覆盖多 active `operator_regions` 默认列表、详情、调查、结案。

#### 小程序证据

- `weapp/miniprogram/pages/operator/_services/operator-safety.ts`
  - `loadOperatorFoodSafetyCaseListPageData()` 只传 `page/limit/status`。
  - `loadOperatorFoodSafetyDetailPageData()`、`saveOperatorFoodSafetyInvestigation()`、`saveOperatorFoodSafetyResolution()` 都只按案件 ID 调用。
- `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts`
  - `getFoodSafetyCases()` 类型和请求参数没有 `region_id`。
  - 详情、调查、结案 API 也没有显式区域参数。
- `weapp/miniprogram/pages/operator/safety/report/index.ts`
  - 页面没有区域选择，也没有说明“必须先选单一区域”；默认用户理解应是自己运营范围内的食安案件。

#### 根因

食安案件是在“单运营商单区域”时期接入的业务链路，后端默认使用 `getOperatorRegionID()`；后来商户、骑手、财务概览等默认视图已迁移到 `resolveOperatorRegionSelection()`，但食安没有同步迁移。前端也没有区域选择入口，因此默认语义在产品上是“我的运营范围”，技术实现却仍是“必须解析出一个区域”。

#### 修复方向

- 列表默认使用 `resolveOperatorRegionSelection()`：未传 `region_id` 时聚合全部 active/legacy 可管理区域；传 `region_id` 时仍显式校验。
- 详情、调查、结案先按案件 ID 读取案件，再对 `caseRecord.RegionID` 调用 `checkOperatorManagesRegion()`；不要用“案件区域等于默认区域”代替授权。
- 结案事务使用案件真实区域作为 `ResolveFoodSafetyCaseTxParams.RegionID`，并保持已结案/并发结案的幂等拒绝语义。
- 小程序可先不增加区域筛选；若后续新增筛选，应复用运营区域 picker，且默认“全部区域”与后端聚合一致。

#### 建议验证

- 后端新增 focused tests：
  - 多 active 区域 operator 无 `region_id` 时能看到多个区域案件，分页和 total 一致。
  - 显式 `region_id` 非管理区域、suspended relation 均返回 403。
  - 多区域 operator 可查看、调查、结案任一自己管理区域案件；非管理区域案件仍 403。
  - 结案重复提交、已结案案件和调查/结案竞态保持现有错误语义。
- 小程序：`npm run compile`；如增加区域筛选，补运营侧专项脚本覆盖默认全部区域、显式切区、错误态。

### OPA-012: 佣金明细仍使用单区域默认口径，和财务概览“全部区域”漂移

- 状态：已修复（2026-06-21，本次 OPA-012 批次）
- 风险级别：G2/G3
- 类型：后端默认区域/多区域漂移、资金统计口径漂移、小程序财务链路不一致
- 影响页面：`weapp/miniprogram/pages/operator/finance/withdraw/index`、`weapp/miniprogram/pages/operator/finance/bills/index`
- 影响接口：`GET /v1/operators/me/commission`

#### 用户可见现象

运营商财务页概览已经按 `resolveOperatorRegionSelection()` 聚合全部可管区域；同一页面的最近佣金明细和佣金账单却调用 `/v1/operators/me/commission`，后端仍使用 `getOperatorRegionID()`。多区域运营商会看到“总收入/本月收入是全部区域，但明细为空、报错或只属于某一个区域”的对账漂移。

#### 后端证据

- `locallife/api/operator_stats.go`
  - `getOperatorFinanceOverview()` 使用 `server.resolveOperatorRegionSelection(ctx)`，并遍历 `selection.RegionIDs` 汇总当月和累计统计。
  - `getOperatorCommission()` 使用 `server.getOperatorRegionID(ctx)`，随后只调用一次 `GetRegionDailyTrend`。
- `locallife/api/operator_stats_test.go`
  - `TestGetOperatorCommissionAPI` 只覆盖单区域 `operator.RegionID`，没有覆盖多 active 区域默认聚合，也没有覆盖显式 `region_id` 与 suspended deny。

#### 小程序证据

- `weapp/miniprogram/pages/operator/_services/operator-finance.ts`
  - `loadOperatorFinancePageData()` 并发调用 `getFinanceOverview()` 和 `getCommissionList({ page: 1, limit: 10 })`，没有传 `region_id`。
  - `loadOperatorCommissionBillPage()` 只传日期、页码、limit，也没有区域参数。
- `weapp/miniprogram/pages/operator/finance/withdraw/index.ts`
  - 页面把 overview 和 commission rows 放在同一财务任务中，用户会自然理解二者属于同一统计范围。
- `weapp/miniprogram/pages/operator/finance/bills/index.ts`
  - 账单页只有日期范围和分页，没有区域选择或“当前仅单一区域”的说明。

#### 根因

财务概览已按多区域 operator 的默认聚合语义重构，但佣金明细沿用旧的单区域 helper。小程序侧也没有把财务概览、最近佣金、账单页的区域选择建成同一个 ViewModel，因此同一财务任务内出现“总览全部区域、明细单区域/403”的漂移。

#### 修复方向

- 后端 `getOperatorCommission()` 使用 `resolveOperatorRegionSelection()`：
  - 显式 `region_id` 时校验单区域；
  - 默认时聚合全部可管区域。
- 多区域聚合按日期合并 `GetRegionDailyTrend` 结果，`order_count/total_gmv/commission` 求和，佣金率用合并后的 `commission / total_gmv` 计算。
- 分页应在合并、排序后执行，`total_count` 代表合并后的日期行数；summary 代表当前查询范围全部日期和全部授权区域。
- 小程序如果暂不做区域选择，财务概览、最近佣金、账单页都保持默认“全部区域”；如新增区域筛选，overview 与 commission 必须共用同一 `selectedRegionId`。

#### 建议验证

- 后端新增 focused tests：
  - 多 active 区域默认合并同一天佣金趋势，summary/total_count/page 均正确。
  - 显式 `region_id` 只返回该区域且校验权限。
  - suspended relation、非管理区域、无区域 operator 均 fail closed。
- 小程序：`npm run check:operator-finance-overview-display`、`npm run check:finance-bill-pages`、`npm run compile`。
- 人工回归：财务概览总额与最近佣金/账单日期范围口径一致，不出现总览有钱但明细加载失败或只显示单区域。

#### 修复记录（2026-06-21）

- 后端 `getOperatorCommission()` 已改用 `resolveOperatorRegionSelection()`：无 `region_id` 默认聚合全部 active/legacy 可管区域，显式 `region_id` 仍走单区域授权校验。
- 佣金明细先按日期合并授权区域趋势，再计算 `summary/total/total_count` 并分页，避免“按单区域分页后拼接”或 summary/items 来源不一致。
- suspended legacy primary region 不再进入默认聚合；新增 focused test 锁定多区域默认汇总和 suspended legacy 排除。
- 小程序财务页和账单页继续不注入默认 `region_id`，由后端承担默认全部区域语义；专项 gate 增加断言，避免后续把 overview 和 commission 拆成不同默认区域口径。
- Swagger 生成物已同步 `/v1/operators/me/commission` 的可选 `region_id` 查询参数说明。

### 2026-06-21 按新增方法二轮 review 记录

本轮已按“业务不变量矩阵”重新 review 运营侧后端 helper 与小程序页面调用，而不是只看已经报错的页面。重点核查了：

- `getOperatorRegionID()`、`resolveOperatorRegionSelection()`、`checkOperatorManagesRegion()` 在运营侧后端 handler 中的使用差异。
- 小程序 `pages/operator/**` 全量页面入口、service/API 参数传递、WXML 关键显示和 ViewState。
- 已修复的商户/骑手/调度/财务概览链路是否继续符合显式区域、默认聚合、legacy fallback、suspended deny 的不变量。

结论：

- `OPA-011`、`OPA-012` 是新增方法发现的真实生产路径漂移，均影响当前运营商小程序默认路径，不能视为文档债。
- `OP-RISK-001` 分账配置属于 backend-only 加固项；后续已从 `getOperatorRegionID()` 切换为多区域选择器，默认聚合全部 active 可管区域，merchant-scoped 配置仍按商户所属区域过滤。
- `OP-RISK-002` `/v1/operator/rules` 后端无参时会先返回 `operators.region_id`，没有校验 active/suspended 关系；小程序规则页当前强制从区域页进入并传 `region_id`，页面路径未复现，但 API 直接调用口径应后续统一。
- 本轮暂未把商户/骑手详情页展示原始 ID、坐标等体验问题升为前后端漂移；它们属于后续运营端信息架构升级机会，不影响当前后端真值或权限边界。

## 修复任务计划

本节是后续修复执行的任务蓝图，不是一次性大改清单。执行时按“一个子任务一组最小相关文件、一组 focused 验证、一次 review、一次提交”的节奏推进；每个大问题完成后，再做一次设计目标复核，确认没有把漂移转移到其它页面、接口或状态口径上。

### 总体执行顺序

1. 已完成的历史批次维持原复核记录：`OPA-001` 至 `OPA-010` 以及 `OP-NOUI` 裁决不重新混改。
2. 新增待修优先级：先修 G3 `OPA-011` 食安案件多区域授权与处置链路，因为它涉及商户暂停/恢复和案件结案状态转移。
3. 再修 G2/G3 `OPA-012` 佣金明细默认聚合，因为它会让财务概览和账单明细对账口径不一致。
4. 最后处理 `OP-RISK-001/002`：分账配置和规则无参兜底属于无当前小程序默认路径或 API 直接调用残余风险，应在生产可见缺陷关闭后做统一 helper 加固。

### 全局修复原则

- 契约先行：后端 response、status、error code、pagination、empty state 是页面真值源；小程序不得用 `?? 'pending'`、本地状态枚举或 catch-all fallback 制造业务真值。
- 权限 fail closed：凡 path/query/body 中出现 `region_id`、`merchant_id`、`rider_id`、规则 ID、提现 ID、追偿 ID，都必须在后端以当前登录运营商身份重建授权边界，不能信任页面传参。
- 幂等显式化：保存、创建、提现、规则发布、追偿处理等写路径必须说明重复提交、弱网重试、超时后重入、并发更新的结果；读路径必须说明分页和筛选条件是否稳定。
- 时序可恢复：小程序保存后不能只依赖乐观 `setData`；必须能从后端重新拉取并得到同一状态。轮询、弱网、返回重入都要有明确 ViewState。
- 小步提交：每个子任务只修改自己列出的路径；提交前检查 `git diff --stat`，不带入其它人的工作树改动。
- 大问题完成复核：每个 OPA 完成后，重新按 route -> handler -> logic/sqlc -> service/api/adapter -> page/ViewState 走一遍，确认设计目标达成且不影响其它正常工作。

### 合并与提交门槛

- 单个子任务：focused 测试或编译通过，review 通过后可独立提交到 `audit/operator-backend-weapp-capability-audit`。
- 单个大问题：所有子任务完成后，补一条复核记录，确认契约、权限、幂等、时序、页面状态均闭环。
- 全部问题：只在本专项所有修复、回归、文档复核完成后，才合并回 `main` 并推送；合并前必须确认工作区只包含本专项提交，不处理、不整理、不回滚其它人的修改。

### 新增待修计划（2026-06-21 二轮 review）

本节只覆盖本轮按新增方法确认的待修项。历史已修复项保留后文复核记录，不在本轮重新展开。

### OPA-011 修复计划：食安案件多区域默认视图与处置授权重构

背景：食安案件列表、详情、调查、结案仍使用 `getOperatorRegionID()`。多区域运营商在小程序默认路径不传 `region_id` 时，可能 403、只看到 legacy 主区域案件，或无法处理自己其它 active 区域的案件。结案会恢复商户并关闭食安事件，属于 G3 状态转移和授权边界。

设计目标：食安案件列表默认展示当前运营商全部可管理区域；详情和写操作按案件真实 `region_id` 校验当前运营商是否管理该区域；结案事务使用案件真实区域，不能使用默认区域替代授权事实。

边界与非目标：本项不重新设计食安事件触发规则，不新增平台审核流，不改变已结案案件的业务含义；小程序是否增加区域筛选属于可选增强，不能成为后端修复的前置条件。

时序、幂等、越权关注：
- 列表读路径必须先解析授权区域集合，再查询案件，避免先读后过滤。
- 详情/调查/结案必须先读案件，再用案件区域做 `checkOperatorManagesRegion()`；页面传入案件 ID 不能隐含授权。
- 调查和结案写路径要保留已有 resolved case、并发结案、调查报告缺失等稳定错误语义。
- 结案恢复商户和关闭事件仍必须保持事务边界，不能拆成多个不可恢复的页面动作。

子任务：

1. `OPA-011-A` 后端多区域失败测试先行
   - 文件：`locallife/api/operator_food_safety_cases_test.go`
   - 内容：新增多 active 区域默认列表、显式区域、suspended deny、详情/调查/结案可处理任一管理区域案件、非管理区域 403 的测试。
   - 验证：`cd locallife && PATH=/usr/local/go/bin:$PATH go test ./api -run Test.*OperatorFoodSafetyCase -count=1`
   - 可提交范围：测试和必要 mock 期望。

2. `OPA-011-B` 列表默认聚合实现
   - 文件：`locallife/api/operator_food_safety_cases.go`，必要时 `locallife/db/query/*food_safety*` 和 sqlc 生成物。
   - 内容：`listOperatorFoodSafetyCases()` 改用 `resolveOperatorRegionSelection()`；若现有 SQL 只能单区域，先做安全的多区域合并和统一排序/分页，或新增 region_ids 查询以保证 total/page 一致。
   - 验证：重复运行 `OPA-011-A` 列表相关 tests；如改 SQL，运行 `make sqlc`。
   - 可提交范围：后端列表实现、测试、生成物。

3. `OPA-011-C` 详情与处置授权重构
   - 文件：`locallife/api/operator_food_safety_cases.go`
   - 内容：详情、调查、结案从“默认 regionID 等于案件 regionID”改为“读取案件后校验案件 regionID 是否由当前 operator 管理”；结案事务传入案件真实 `RegionID`。
   - 验证：重复运行 `OPA-011-A` 写路径 tests，并覆盖已结案、调查报告缺失、并发更新错误语义。
   - 可提交范围：后端详情/写路径最小改动和测试。

4. `OPA-011-D` 小程序契约与可选区域筛选收口
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-basic-management.ts`、`_services/operator-safety.ts`、`safety/report/index.*`、`safety/detail/index.*`
   - 内容：若后端默认聚合全部区域，页面可先保持无区域筛选；若新增筛选，必须传显式 `region_id` 并复用 active 运营区域 picker。错误态不得把 403 渲染成空列表。
   - 验证：`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run compile`；如新增脚本，纳入 `check:operator-capability-audit`。
   - 可提交范围：小程序食安 service/page 和专项脚本。

5. `OPA-011-E` 完成复核
   - 文件：本文档。
   - 内容：按完成复核模板记录提交、设计目标、时序、幂等、越权、回归、非目标和剩余风险。
   - 验证：文档 review。
   - 可提交范围：文档。

大问题复核：
- 多区域 operator 默认能看到全部 active/legacy 可管区域食安案件，且分页/total 不漂移。
- 详情、调查、结案只按案件真实区域授权；非管理区域和 suspended relation fail closed。
- 结案事务仍能原子恢复商户和关闭事件，重复提交和并发竞态语义明确。

### OPA-012 修复计划：佣金明细默认聚合与财务 ViewModel 口径统一

背景：`getOperatorFinanceOverview()` 已默认聚合全部可管理区域，但 `getOperatorCommission()` 仍使用 `getOperatorRegionID()`。小程序财务页和账单页不传 `region_id`，导致总览和明细在多区域运营商下可能 403、为空或只反映一个区域。

设计目标：财务概览、最近佣金、佣金账单在未选区时共享“全部区域”语义；显式选区时共享单区域校验；金额和比例展示继续由 ViewModel 输出。

边界与非目标：本项不改变分账订单来源、不重算历史分账、不新增提现能力；不把分润配置页面塞入财务页。

时序、幂等、越权关注：
- 佣金明细为读路径，无写入幂等；但多区域合并必须先完成授权区域解析。
- 多区域同一天趋势要先合并再分页，不能各区域分页后拼接。
- summary、total、total_count、items 必须来自同一日期范围和同一区域集合。
- 显式 `region_id` 403 不得被小程序吞成空账单。

子任务：

1. `OPA-012-A` 后端聚合失败测试先行
   - 文件：`locallife/api/operator_stats_test.go`
   - 内容：新增多 active 区域默认合并、显式区域、suspended deny、summary/total_count/page 的 focused tests。
   - 验证：`cd locallife && PATH=/usr/local/go/bin:$PATH go test ./api -run TestGetOperatorCommissionAPI -count=1`
   - 可提交范围：测试和 mock 期望。

2. `OPA-012-B` 后端佣金聚合实现
   - 文件：`locallife/api/operator_stats.go`，必要时新增小 helper。
   - 内容：`getOperatorCommission()` 改用 `resolveOperatorRegionSelection()`；按日期合并多个区域 `GetRegionDailyTrend` 结果；合并后排序、分页、计算 summary 和佣金率。
   - 验证：重复运行 `OPA-012-A`；同时跑 `TestGetOperatorFinanceOverviewAPI` 确认总览未回退。
   - 可提交范围：后端实现和测试。

3. `OPA-012-C` 小程序财务口径复核
   - 文件：`weapp/miniprogram/pages/operator/_services/operator-finance.ts`、`finance/withdraw/index.*`、`finance/bills/index.*`
   - 内容：保持默认“全部区域”或新增共用区域筛选；overview 与 commission 必须使用同一 selectedRegionId；错误态明确显示佣金明细加载失败，不用空列表掩盖 403。
   - 验证：`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run check:operator-finance-overview-display && PATH="$HOME/.local/bin:$PATH" npm run check:finance-bill-pages && PATH="$HOME/.local/bin:$PATH" npm run compile`
   - 可提交范围：财务 service/page 和脚本。

4. `OPA-012-D` 专项 gate 加固
   - 文件：`weapp/scripts/check-operator-capability-audit*.js` 或现有运营侧专项脚本。
   - 内容：锁定财务页不能把 overview 默认全部区域和 commission 默认单区域拆开；如新增区域筛选，检查两个请求使用同一参数源。
   - 验证：`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run check:operator-capability-audit`
   - 可提交范围：小程序脚本和 package 命令。

5. `OPA-012-E` 完成复核
   - 文件：本文档。
   - 内容：补充完成复核，说明资金统计口径、分页、权限、非目标和剩余风险。
   - 验证：文档 review。
   - 可提交范围：文档。

大问题复核：
- 默认财务概览、最近佣金、佣金账单均为全部可管区域。
- 显式区域、非管理区域、suspended relation、legacy fallback 均有 focused test。
- summary、items、total_count、分页来源一致，不再出现“总览有收入但明细无数据/报错”的默认路径漂移。

### OP-RISK 加固计划：backend-only 和 API 直接调用残余风险

背景：本轮还发现两个不直接破坏当前小程序默认路径的残余风险：`listOperatorProfitSharingConfigs()` 使用 `getOperatorRegionID()`，多区域 operator 无默认区域时不具备聚合语义；`resolveOperatorRuleRegionID()` 无参时优先返回 `operators.region_id`，没有像显式 `region_id` 一样校验 active/suspended 关系。它们不应和 `OPA-011/012` 混同，但应进入后续加固队列。

设计目标：所有 operator 后端能力对“无参默认、显式 region_id、legacy fallback、suspended deny”的解释一致；backend-only 能力不因为暂时无小程序入口而保留弱授权口径。

子任务：

1. `OP-RISK-001-A` 分账配置默认区域语义裁决
   - 文件：`locallife/api/operator_profit_sharing_config.go`、本文档。
   - 内容：已裁决为默认聚合全部 active 可管区域；显式 `region_id` 仍只查单区域并 fail closed。
   - 验证：focused API tests。

2. `OP-RISK-001-B` 分账配置权限和分页加固
   - 文件：`locallife/db/query/profit_sharing_config.sql`、`locallife/api/operator_profit_sharing_config.go`
   - 内容：新增 `ListProfitSharingConfigsForRegions`，默认查询按 `region_ids` 过滤区域默认配置，同时通过商户所属区域限制 merchant-scoped 配置不跨授权区域泄漏。
   - 验证：`make sqlc`、`go test ./api -run TestListOperatorProfitSharingConfigsAPI -count=1`、必要 SQLC 临时库测试。

3. `OP-RISK-002-A` 规则无参兜底加固
   - 文件：`locallife/api/operator_rules.go`
   - 内容：`resolveOperatorRuleRegionID()` 无参时不直接信 `operator.RegionID`；改为 `getOperatorRegionID()` 或显式校验 active/suspended 关系。由于小程序规则页已传 `region_id`，本项主要锁 API 直接调用。
   - 验证：新增 suspended relation 无参 API tests；现有 `rules/index` 小程序路径不受影响。

4. `OP-RISK-002-B` 运营侧 helper 使用矩阵脚本化
   - 文件：可新增 `weapp/scripts/check-operator-capability-audit` 补充项或后端轻量脚本。
   - 内容：扫描 operator handler 中 `getOperatorRegionID()` 使用点，要求每个使用点在文档中分类为“必须单区”或“待迁移聚合”。
   - 验证：专项脚本通过。

### OPA-005 修复计划：峰时时段列表跨区域读取权限

背景：`GET /v1/operator/regions/:region_id/peak-hours` 与同资源 create/delete 不一致，读路径没有校验当前运营商是否管理该区域。这是 G3 授权边界问题，应优先修。

设计目标：读、写、删三条峰时时段路径共享同一运营商区域授权语义；非法区域返回稳定的无权限错误；合法区域原行为不变。

边界与非目标：本项只处理峰时时段配置读取授权，不重构代取费全部权限体系，不调整峰时段业务模型，不改变合法区域的 response DTO。

时序、幂等、越权关注：
- 读路径本身无写入幂等问题，但必须在读取 SQL 前完成授权校验，避免先读后拒绝造成侧信道。
- 非法区域、已暂停区域、历史解绑区域要按业务授权口径明确返回 403 或同等业务错误。
- 页面传入的 `region_id` 只能作为待校验目标，不能作为权限事实。

子任务：

1. `OPA-005-A` 后端回归测试先行
   - 文件：`locallife/api/delivery_fee_test.go` 或同域现有测试文件。
   - 内容：新增合法区域返回 200、非管理区域返回 403 的测试；覆盖 operator 身份、path region、空列表合法返回。
   - 验证：`cd locallife && go test ./api -run 'Test.*PeakHour.*List|TestListPeakHourConfigs'`
   - 可提交范围：只提交测试和必要测试夹具。

2. `OPA-005-B` 后端授权修复
   - 文件：`locallife/api/delivery_fee.go`
   - 内容：在 `listPeakHourConfigs()` 绑定 path 后、查询配置前调用 `checkOperatorManagesRegion(ctx, uri.RegionID)`；错误映射沿用 create/delete。
   - 验证：重复运行 `OPA-005-A` 的 focused test。
   - 可提交范围：handler 最小改动和测试期望更新。

3. `OPA-005-C` 同类 path-region GET 审查
   - 文件：`locallife/api/server.go`、`locallife/api/*operator*go`、`locallife/api/delivery_fee.go`
   - 内容：列出所有 operator path/query `region_id` GET，确认是否调用 `checkOperatorManagesRegion()` 或等价的 `resolveOperatorRegionSelection()`；发现新缺口另开 finding，不混入本修复提交。
   - 验证：更新本文档“后续审计工作记录”。
   - 可提交范围：审计文档，不和业务代码混合。

大问题复核：
- 非管理区域在 handler 层被拒绝，SQL 不读取目标配置。
- 合法区域页面 `region/config` 和 `timeslot/index` 仍能加载空列表和已有列表。
- 没有新增“前端隐藏按钮等于权限控制”的假权限。

### OPA-001 修复计划：运营商区域状态契约重构

背景：`/v1/operator/regions` 返回通用行政区 DTO，丢弃 `operator_regions.status`；小程序把缺失 `status` 默认成 `pending`，导致已审核区域显示“待审核”。

设计目标：建立运营商区域专用契约，清楚区分行政区状态、运营商区域关系状态、申请状态；页面只展示后端真实返回的业务状态。

边界与非目标：本项不改变运营商申请审批流，不迁移历史区域数据，不把区域扩展申请状态混入已管理区域列表。

时序、幂等、越权关注：
- `GET /v1/operator/regions` 是读路径，无写入幂等问题；但多区域列表必须由当前 operator 身份解析，不能通过前端传 operator_id。
- 状态来源必须稳定：`operator_regions.status` 表示关系状态，申请表状态只用于申请记录，不用于已管理区域标签。
- 如果后端需要兼容旧字段名 `status`，必须同时说明 `status` 的含义，不允许再让页面猜测。

子任务：

1. `OPA-001-A` 契约设计与 Swagger 形态确认
   - 文件：`locallife/api/operator_stats.go`、`locallife/api/region.go`、`locallife/docs/swagger.yaml` 或 swagger 源注释。
   - 内容：定义 `operatorRegionResponse` 字段：`region_id/id/code/name/level/parent_id/longitude/latitude/status`；其中 `status` 明确为 `operator_regions.status`，枚举 `active/suspended`。若需要行政区状态，单独命名为 `region_catalog_status`。
   - 验证：文档 review 通过后再进入实现。
   - 可提交范围：可先只提交 API 契约说明和测试草案。

2. `OPA-001-B` 后端 response 与测试
   - 文件：`locallife/api/operator_stats.go`、必要时 `locallife/db/sqlc/operator_region.sql.go` 只读使用既有字段。
   - 内容：`listOperatorRegions()` 直接从 `ListOperatorRegions` 的关系记录构造运营商区域 DTO，不再通过 `newRegionResponse(region)` 丢弃关系状态；保留区域基础信息。
   - 验证：`cd locallife && go test ./api -run TestListOperatorRegions`；如修改 Swagger 注释，运行 `make swagger`。
   - 可提交范围：后端 DTO、handler、测试、swagger 生成物。

3. `OPA-001-C` 小程序类型与 adapter 收口
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-basic-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-regions.ts`、`weapp/miniprogram/pages/operator/region/index.*`
   - 内容：移除 `data.status ?? 'pending'`；`RegionStatus` 对齐 `active/suspended`；缺失状态展示为“状态未知”或进入错误态，不展示“待审核”。
   - 验证：`cd weapp && npm run compile && npm run lint`。
   - 可提交范围：小程序 API 类型、adapter、区域页显示。

4. `OPA-001-D` 状态文案和入口行为复核
   - 文件：`weapp/miniprogram/pages/operator/dashboard/index.*`、`analytics/index.*`、`region/config`、`rules/index` 中使用区域选择的页面。
   - 内容：确认这些页面没有继续把运营区域状态理解成申请状态；暂停区域的配置入口是否禁用按产品口径单独确认。
   - 验证：开发者工具或真机回归：已审核区域显示运营中，暂停区域显示暂停，缺字段不显示待审核。
   - 可提交范围：只提交必要页面状态映射，不做视觉重构。

大问题复核：
- 生产样本宁晋县 `130528` 的后端 response 能返回 `active`。
- 小程序没有任何 `status ?? 'pending'` 兜底影响运营区域。
- Swagger、TypeScript 类型、页面文案对同一状态含义一致。

### OPA-004 修复计划：区域体检统计 DTO 重构

背景：小程序分析页读取嵌套 `merchant_stats/rider_stats/order_stats/financial_stats`，后端 `/regions/:id/stats` 实际只返回扁平六字段，正常 200 也可能触发运行时解引用错误。

设计目标：以“运营商查看区域体检”的用户任务重新定义统计契约；页面只展示后端真实支持且可解释的指标，缺失指标不得通过虚构 DTO 填充。

边界与非目标：本项不重建全量 BI 系统，不新增耗时聚合 worker，不改变排行榜和趋势接口；如需要新增指标，只在当前区域体检场景内最小扩展。

时序、幂等、越权关注：
- 统计接口必须先校验 operator 管理目标区域，再聚合数据。
- 多指标聚合需要同一查询时刻或可接受的近实时口径说明，避免页面把不同时间窗口数据当成强一致。
- 读路径无写入幂等问题，但弱网重试不能把旧区域统计展示到新选中区域。

子任务：

1. `OPA-004-A` 统计契约决策
   - 文件：本文档、`locallife/api/operator_stats.go`、`weapp/miniprogram/pages/operator/_api/operator-analytics.ts`
   - 内容：在“后端扩展嵌套 DTO”和“前端降级到扁平 DTO + 组合已有接口”之间做明确选择。推荐后端提供页面所需的 `operatorRegionStatsResponse`，但只包含当前 SQL 可稳定支持的指标。
   - 验证：契约 review，确认每个字段来源、时间窗口、空值语义。
   - 可提交范围：文档和测试草案。

2. `OPA-004-B` 后端统计 response 与权限测试
   - 文件：`locallife/api/operator_stats.go`、相关 sqlc 查询或既有统计查询。
   - 内容：实现选定 DTO；对非法区域返回无权限；对无数据区域返回 0 值而非缺字段；Swagger 更新。
   - 验证：`cd locallife && go test ./api -run TestGetRegionStats`；如改 Swagger，`make swagger`；如改 SQL，`make sqlc`。
   - 可提交范围：后端统计契约、测试、生成物。

3. `OPA-004-C` 小程序 analytics adapter 重写
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-analytics.ts`、`_services/operator-analytics-dashboard.ts`、`analytics/index.*`
   - 内容：所有后端统计先进入 adapter，adapter 输出页面 ViewModel；禁止页面或 service 直接解引用可选嵌套字段。
   - 验证：`cd weapp && npm run compile && npm run lint`。
   - 可提交范围：analytics API 类型、service adapter、页面绑定。

4. `OPA-004-D` 弱网和切区时序复核
   - 文件：`analytics/index.ts`
   - 内容：切换区域时取消或丢弃旧请求结果；错误态保留当前选区，不把旧区域数据误显示为新区域数据。
   - 验证：开发者工具模拟慢网，连续切换区域，确认最终展示属于最后一次选择。
   - 可提交范围：页面状态机最小修复。

大问题复核：
- 区域体检首屏不再依赖虚构字段。
- 统计接口的区域授权、空数据、错误语义、时间窗口均可解释。
- dashboard 与 analytics 如果共用指标，口径一致或差异被明确命名。

### OPA-006 修复计划：骑手申请状态与生命周期状态分离

背景：实时统计旧代码用 `riders.status = pending` 统计“待审骑手”，骑手列表旧代码又允许 `pending_approval/rejected`。审计过程中确认这不是简单枚举拼写问题：`riders` 表只有已形成骑手后的生命周期状态，待审属于 `rider_applications` 申请流程；且申请表当前没有区域归属，不能直接进入运营商区域指标。

设计目标：运营商实时统计和骑手管理只表达真实可归属、可授权的骑手生命周期；小程序不再展示后端不支持的“待入驻/待审骑手”承诺；未来如要做运营商审核骑手申请，先补区域归属和授权模型。

边界与非目标：本项不新增骑手申请审核入口，不迁移 `rider_applications` schema，不把申请表的全局数量拼入区域实时统计，不解决 OPA-003 的搜索和在线筛选缺口。

时序、幂等、越权关注：
- 当前修复只影响读路径和页面展示，无新增写入幂等问题。
- 申请审批时序仍由申请流负责；只有申请批准并形成 `riders` 记录后，才进入运营商骑手生命周期统计。
- `rider_applications` 未区域化前，任何 submitted/pending 申请数量都不能作为某个运营商区域的指标暴露。
- 多区域实时统计继续沿用当前 operator 区域选择和权限解析，不接受前端传 `operator_id`。

子任务：

1. `OPA-006-A` 状态模型源审计
   - 文件：`locallife/db/sqlc/constants.go`、`locallife/api/operator_merchant_rider.go`、`locallife/api/operator_realtime.go`、骑手申请/审核相关文件。
   - 内容：列出 `riders.status` 与 `rider_applications.status` 的合法枚举、写入点和生产只读聚合；确认二者不能互相替代。
   - 验证：`rg -n '"pending"|"pending_approval"|RiderStatus|rider_applications' locallife`；生产只读聚合只记录枚举数量，不输出 PII。
   - 可提交范围：文档或 focused 测试，不混入业务修复。

2. `OPA-006-B` 错误修复回滚
   - 文件：`locallife/api/operator_merchant_rider.go`、`locallife/api/operator_realtime.go`、`locallife/api/operator_realtime_test.go`、`locallife/db/sqlc/constants.go`、`weapp/miniprogram/pages/operator/riders/index.wxml`。
   - 内容：回滚把 `pending` 改成 `pending_approval` 的错误修复，避免把申请态继续伪装成骑手生命周期。
   - 验证：回滚后重新跑 OPA-006 focused 测试，确认没有保留错误待审口径。
   - 可提交范围：独立 revert 提交 `51737559`，便于 review 看清纠偏动作。

3. `OPA-006-C` 后端生命周期契约修复
   - 文件：`locallife/api/operator_realtime.go`、`locallife/api/operator_merchant_rider.go`、`locallife/api/operator_realtime_test.go`、`locallife/api/operator_merchant_rider_test.go`、Swagger 生成物。
   - 内容：`pending_rider_count` 作为兼容字段固定返回 `0` 并注释说明原因；列表 status 只接受 `approved/active/suspended`；summary 返回 `approved/active/suspended/online`；测试禁止查询 `riders.pending`、`riders.pending_approval` 或全局 `rider_applications.submitted`。
   - 验证：`cd locallife && PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestGetOperatorRealtimeStatsAPI_DoesNotUseApplicationStatusesForRegionRiders|TestGetOperatorRiderSummaryAPI|TestListOperatorRidersAPI' -count=1`；`PATH=/usr/local/go/bin:$PATH make check-generated`。
   - 可提交范围：后端 API、测试、Swagger 生成物。

4. `OPA-006-D` 小程序骑手生命周期显示收口
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts`、`weapp/miniprogram/pages/operator/riders/index.wxml`。
   - 内容：移除 `pending/pending_approval/rejected/offline` 生命周期承诺；`approved` 显示为待激活；analytics 不再展示假的待审计数。
   - 验证：`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run compile && PATH="$HOME/.local/bin:$PATH" npm run lint`。
   - 可提交范围：小程序类型、service adapter、页面 tab 文案。

5. `OPA-006-E` 未来骑手申请区域化设计前置
   - 文件：本文档或后续骑手申请设计文档。
   - 内容：如果要让运营商处理骑手申请，先设计申请区域归属、运营商可见性、审批幂等、重复提交、审核冲突和历史数据迁移，再新增后端和小程序入口。
   - 验证：架构 review；不在 OPA-006 当前修复里暗改 schema 或页面。
   - 可提交范围：文档或独立设计任务。

大问题复核：
- `riders.status` 生命周期只剩 `approved/active/suspended`，页面和后端契约一致。
- `pending_rider_count` 不再假装能表达区域待审申请；兼容字段为 0 且有注释和测试锁定。
- 申请态和骑手生命周期不再在 API 类型、summary、realtime、tab 文案中混用。
- 未来待审申请能力被明确拆成新设计任务，不在当前指标接口中补假数。

### OPA-007 修复计划：代取费保存错误语义与幂等收口

背景：小程序 `updateRegionConfig()` 捕获 PATCH 任意失败后都 POST，权限失败、参数错误、500、网络错误都会被误认为“配置不存在”。

设计目标：保存代取费配置时，创建、更新、缺配置、权限失败、参数错误、网络未知结果各有明确语义；重复点击和弱网重试不会制造错误写入。

边界与非目标：本项不重做整个代取费页面视觉，不改变计费模型；是否新增后端 upsert 接口作为第二阶段设计决策，不能和前端错误修复混为一体。

时序、幂等、越权关注：
- PATCH 只有在后端明确返回配置不存在时才可转 POST。
- 403/401 必须直接停止，不能二次 POST 探测权限。
- 400 必须保留参数错误，500/网络失败进入未知结果或可重试错误，不做创建写入。
- 保存按钮需要提交中禁用；超时后重入应以 GET 后端状态为准。

子任务：

1. `OPA-007-A` 后端错误契约确认
   - 文件：`locallife/api/delivery_fee.go`、`locallife/docs/swagger.yaml` 或 Swagger 注释。
   - 内容：确认 PATCH 缺配置返回的 HTTP status 和业务 code；若当前没有稳定 code，新增或记录稳定错误分类。
   - 验证：`cd locallife && go test ./api -run 'Test.*DeliveryFee.*Config'`。
   - 可提交范围：错误契约测试和必要注释。

2. `OPA-007-B` 小程序 fallback 判定修复
   - 文件：`weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts`
   - 内容：只对 404 或明确业务 code 执行 POST；其它错误原样抛出。错误解析使用现有 request 封装，不用 `err.message` 字符串猜测。
   - 验证：新增或补充 service 测试：PATCH 404 才 POST；PATCH 400/403/500/network 不 POST。
   - 可提交范围：API 方法和测试。

3. `OPA-007-C` 页面提交状态与未知结果处理
   - 文件：`weapp/miniprogram/pages/operator/delivery-fee/index.ts`、相关 service。
   - 内容：保存中禁用重复提交；保存失败不乐观覆盖本地配置；网络未知时提示用户刷新或重试，并保留草稿。
   - 验证：`cd weapp && npm run compile && npm run lint`；开发者工具模拟超时和重复点击。
   - 可提交范围：页面状态机和文案。

4. `OPA-007-D` 后端 upsert 方案评审
   - 文件：本文档或后续设计文档。
   - 内容：评估是否需要 `PUT /v1/delivery-fee/regions/:region_id/config` 作为显式幂等 upsert；若需要，另开 G2/G3 后端任务，包含唯一约束、条件更新、审计日志和重复提交测试。
   - 验证：架构 review，不在当前小修中暗改接口语义。
   - 可提交范围：设计记录。

大问题复核：
- 所有非 404 PATCH 失败都不会触发 POST。
- 重复点击、弱网超时、返回重入均能从后端恢复真实状态。
- 权限失败不会被页面二次请求掩盖。

### OPA-002 修复计划：商户列表搜索契约闭环

背景：小程序商户列表提供“搜索商户名称、电话”，但后端 `GET /v1/operator/merchants` 不绑定 `keyword`，搜索承诺无效。

设计目标：商户搜索是后端支持的、可分页、可计数、受区域权限约束的能力；页面搜索结果和总数都来自同一查询条件。

边界与非目标：本项只支持当前页面承诺的名称和电话搜索；不引入复杂全文检索、不改变商户审核状态流、不新增跨角色搜索。

时序、幂等、越权关注：
- 搜索是读路径，无写入幂等问题；但分页必须绑定 keyword，翻页时不能丢失搜索条件。
- 查询必须始终限定当前 operator 可管理区域，keyword 不能扩大可见范围。
- 弱网乱序返回时，旧关键词结果不能覆盖新关键词页面。

子任务：

1. `OPA-002-A` 后端搜索契约定义
   - 文件：`locallife/api/operator_merchant_rider.go`、`locallife/db/query/*merchant*.sql`
   - 内容：定义 `keyword` trim 后的最大长度、空字符串语义、匹配字段、手机号模糊规则、排序字段和分页 count 口径。
   - 验证：契约 review。
   - 可提交范围：文档或测试草案。

2. `OPA-002-B` SQL/sqlc 查询实现
   - 文件：`locallife/db/query/*merchant*.sql`、生成的 `locallife/db/sqlc/*`。
   - 内容：新增按区域集合、状态、keyword 查询和 count；避免字符串拼接 SQL。
   - 验证：`cd locallife && make sqlc`；SQL review 检查索引和模糊查询成本。
   - 可提交范围：SQL、生成物、查询单测或集成测试。

3. `OPA-002-C` Handler 参数绑定与测试
   - 文件：`locallife/api/operator_merchant_rider.go`
   - 内容：`listOperatorMerchantsRequest` 绑定 `keyword`；校验长度；传入查询层；非法参数返回稳定 400。
   - 验证：`cd locallife && go test ./api -run TestListOperatorMerchants`。
   - 可提交范围：handler、测试。

4. `OPA-002-D` 小程序搜索状态机复核
   - 文件：`weapp/miniprogram/pages/operator/merchants/index.ts`、`_services/operator-merchant-management.ts`
   - 内容：确认搜索、清空、翻页、返回重入都保留同一条件；旧请求不覆盖新关键词。
   - 验证：`cd weapp && npm run compile && npm run lint`；开发者工具输入关键词、清空、分页回归。
   - 可提交范围：必要页面/service 修复。

大问题复核：
- 搜索命中、未命中、跨区域不可见、状态 + keyword 组合、分页 count 都有测试或手工证据。
- 页面不再承诺后端不支持的搜索字段。

### OPA-003 修复计划：骑手搜索与在线状态模型拆分

背景：小程序骑手 API 类型包含 `keyword`、`online_status`，但后端只支持 `approved/active/suspended` 生命周期状态过滤和分页，不支持 keyword/online_status。原先混入生命周期的 `offline/pending_approval/rejected` 已由 OPA-006 收口，不能在 OPA-003 中重新引入。

设计目标：骑手生命周期、在线状态、搜索关键词分层清楚；后端明确支持哪些查询条件，页面只暴露真实能力。

边界与非目标：本项不改变骑手上下线机制，不新增实时定位或在线心跳系统；若后端没有可靠在线状态源，本轮不做在线筛选 UI。

时序、幂等、越权关注：
- 搜索和筛选是读路径，分页与 keyword/status/online_status 必须绑定。
- 在线状态如果来自实时/缓存数据，必须说明时效和过期语义，不能显示为强实时真值。
- 所有骑手查询必须限定当前 operator 管理区域。

子任务：

1. `OPA-003-A` 骑手状态模型裁决
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-rider-management.ts`、`locallife/api/operator_merchant_rider.go`
   - 内容：确认生命周期枚举维持 OPA-006 的 `approved/active/suspended`；不得重新引入 `pending_approval/rejected/offline`。确认是否有后端在线状态源；没有则删除 `online_status` 类型承诺。
   - 验证：契约 review。
   - 可提交范围：类型清理或文档。

2. `OPA-003-B` 后端 keyword 搜索实现
   - 文件：`locallife/db/query/*rider*.sql`、`locallife/api/operator_merchant_rider.go`
   - 内容：按姓名、电话实现区域内 keyword 搜索；状态组合和分页 count 与商户搜索一致。
   - 验证：`cd locallife && make sqlc && go test ./api -run TestListOperatorRiders`。
   - 可提交范围：SQL/sqlc、handler、测试。

3. `OPA-003-C` 在线状态能力处理
   - 文件：如存在在线状态后端来源，涉及对应 logic/cache 查询；否则只改小程序类型。
   - 内容：若后端有可靠在线状态，显式绑定 `online_status=online/offline` 并测试；若没有，移除页面和 API 类型中的 `online_status` 暴露。
   - 验证：按所选路径运行后端或小程序 focused 验证。
   - 可提交范围：不要同时做“假在线筛选”和搜索修复。

4. `OPA-003-D` 小程序骑手列表 ViewState
   - 文件：`weapp/miniprogram/pages/operator/riders/index.ts`、`_services/operator-rider-management.ts`
   - 内容：搜索、状态 tab、分页、返回重入状态一致；非法 status 不在页面生成；弱网旧请求不覆盖新筛选。
   - 验证：`cd weapp && npm run compile && npm run lint`；开发者工具回归姓名、电话、状态 tab。
   - 可提交范围：页面/service 类型与状态机修复。

大问题复核：
- 生命周期 status 和在线 status 在代码、类型、文案中不再混用。
- 若保留在线筛选，有可靠后端来源和时效说明；若不保留，页面不再暴露该承诺。
- 跨区域不可见路径被后端测试覆盖。

### OP-NOUI 修复计划：后端已有能力但运营商小程序无入口

背景：当前发现追偿争议/追偿单、分账规则配置、规则引擎代理已有后端路由，但运营商小程序无直接入口。此类问题不能简单按“缺页面”处理，因为有些能力可能是平台端、内部端或预留能力。

设计目标：对每个 backend-only 能力先裁决产品归属、角色权限、操作频率和风险等级，再决定新增小程序入口、保留 API-only、迁移到平台端，或退役/隐藏。

边界与非目标：本项不在未裁决前新增页面，不把规则引擎这类复杂能力塞进运营商移动端，不因为接口存在就扩大运营商权限。

时序、幂等、越权关注：
- 追偿、分账、规则发布都可能影响资金、规则命中或争议处理，默认按 G3 或至少 G2 评估。
- 写路径必须有幂等键、重复提交语义、审计日志和权限边界；小程序只能展示适合移动端处理的高频任务。
- 列表读路径必须确认 operator 只能看到自己区域、自己分账配置或被授权规则。

子任务：

1. `OP-NOUI-A` 能力归属裁决表
   - 文件：本文档。
   - 内容：为 `recovery-disputes/recoveries`、`profit-sharing/configs`、`operators/me/rules/**` 分别记录目标角色、是否移动端可操作、是否只读、风险等级、上线条件。
   - 验证：产品/后端/前端 review。
   - 可提交范围：文档。

2. `OP-NOUI-B` 后端权限与审计复核
   - 文件：`locallife/api/operator_recovery*`、`locallife/api/operator_profit*`、`locallife/api/operator_rule*` 或实际 handler 文件。
   - 内容：确认每条 API 是否从登录 operator 重建权限，是否可被传参越权，写路径是否有幂等和审计。
   - 验证：focused backend tests 或新增 finding。
   - 可提交范围：每个能力域单独提交，避免混改资金与规则。

3. `OP-NOUI-C` 移动端任务 IA 设计
   - 文件：如决定新增页面，落到 `weapp/miniprogram/pages/operator/<domain>/...` 设计记录；暂不直接编码。
   - 内容：从用户任务出发决定页面组：追偿更像“异常处理队列”，分账配置更像“只读配置确认”，规则引擎可能不适合手机端复杂编辑。
   - 验证：Human-Centered UI review，确认首屏任务、低频能力、失败恢复。
   - 可提交范围：设计文档或页面任务说明。

4. `OP-NOUI-D` 分域实现或退役
   - 文件：按裁决结果另开实现任务。
   - 内容：新增入口时先接只读列表/详情，再评估写操作；退役时移除或隐藏未使用路由需有兼容和监控计划。
   - 验证：后端 focused tests、`cd weapp && npm run quality:check`、必要的权限回归。
   - 可提交范围：每个能力域独立 PR/提交。

大问题复核：
- 每个 backend-only 能力都有明确结论：新增入口、API-only、迁移平台端、延后或退役。
- 没有因为补 UI 而扩大运营商权限。
- 高风险写操作具备幂等、审计、重复提交和未知结果恢复策略。

#### OP-NOUI 能力归属裁决表（2026-06-21）

| 能力 | 当前后端入口 | 目标角色 | 移动端裁决 | 后端安全裁决 | 风险等级 | 上线条件 |
| --- | --- | --- | --- | --- | --- | --- |
| 追偿争议/追偿单 | `GET /v1/operator/recovery-disputes`、`/summary`、`/:id`、`GET /v1/operator/recoveries/:id` | 运营商异常处理/风控观察 | 暂不新增小程序入口；后续如产品需要，按“异常处理队列”另开 IA 和只读列表/详情任务 | 保留 read-only API；列表走 `resolveOperatorRegionSelection`，详情/追偿单按 operator 管理区域校验 | G2/G3 | 明确运营商是否需要移动端处理追偿异常；若新增入口，先只读观察，再单独评估任何处理动作 |
| 分润配置可见性 | `GET /v1/operators/me/profit-sharing/configs` | 运营商财务确认/配置观察 | 暂不新增小程序入口；可在财务页后续增加只读“分润配置确认” | 保留 read-only API；默认聚合全部 active 可管区域；已修复 merchant-scoped 全局配置跨区域泄漏，商户专属配置必须归属当前 operator region | G3 | 需要产品确认展示字段、解释文案和是否放入财务；不得提供移动端写配置 |
| 规则引擎代理只读 | `GET /v1/operators/me/rules`、`/:id`、`/hits` | 运营商只读观察/诊断 | 不新增复杂规则引擎页面；现有小程序规则配置继续使用 `/v1/operator/rules` 简化区域规则入口 | 保留只读代理；按 rule version scope/gray_config 或 hit region 过滤 | G3 | 若需要运营商规则命中观察页，先定义只读任务和字段裁剪，不暴露编辑/发布能力 |
| 规则引擎代理写入 | `POST /v1/operators/me/rules`、`/:id/versions`、`/:id/publish`、`/:id/rollback`、`/:id/disable` | 平台规则治理 | 明确不适合运营商小程序 | 已关闭，统一返回 403；规则创建、版本、发布、回滚、禁用必须走平台治理入口 | G3 | 若未来重新开放，必须先设计区域独占/共享规则所有权、幂等键、审计失败处理、重复提交语义和跨区域回归测试 |

### 修复批次建议

| 批次 | 范围 | 目标 | 合并前门槛 |
| --- | --- | --- | --- |
| History Batch 1 | `OPA-005` | 先关闭跨区域读取 | 已完成：后端权限测试通过，非法区域 fail closed |
| History Batch 2 | `OPA-001`、`OPA-006` | 统一运营区域和骑手状态真值 | 已完成：后端 DTO/常量、小程序状态映射、生产样本复核 |
| History Batch 3 | `OPA-007` | 收口保存错误语义和幂等 | 已完成：PATCH 非 404 不 POST，弱网/重复点击可恢复 |
| History Batch 4 | `OPA-004` | 重构区域体检统计契约 | 已完成：后端统计 DTO 与页面 ViewModel 一致，切区时序正确 |
| History Batch 5 | `OPA-002`、`OPA-003` | 列表搜索/筛选能力闭环 | 已完成：keyword/status/分页/count/跨区域测试覆盖 |
| History Batch 6 | `OP-NOUI-*` | 后端-only 能力归属裁决 | 已完成：先裁决再实现，不用接口反推页面 |
| History Batch 7 | `OPA-008`、`OPA-009`、`OPA-010` | 用户回归暴露的二轮漂移收口 | 已完成：页面成功态可见、调度区域授权一致、财务关键展示由 ViewModel 输出 |
| Next Batch 1 | `OPA-011` | 食安案件多区域默认视图与处置授权 | 多区域列表、详情、调查、结案测试通过；非管理/suspended fail closed |
| Next Batch 2 | `OPA-012` | 佣金明细默认聚合与财务口径统一 | overview、recent commission、bill items 同一区域集合；summary/page/count 一致 |
| Next Batch 3 | `OP-RISK-001/002` | backend-only 和 API 直接调用残余风险加固 | 分账配置已完成多区域默认聚合；规则无参兜底仍需完成显式语义裁决与 focused tests |

### 每个大问题完成后的复核模板

完成任一 OPA 后，在本节下追加复核记录：

| 字段 | 记录要求 |
| --- | --- |
| 完成范围 | 列出本次完成的子任务 ID、提交 hash、涉及文件 |
| 设计目标 | 逐条确认是否达成，未达成项说明原因和后续任务 |
| 时序 | 弱网、乱序返回、重复点击、重入、轮询或统计窗口是否已验证 |
| 幂等 | 写路径重复提交、超时重试、冲突更新的行为是否明确 |
| 越权 | region/merchant/rider/rule/finance/recovery 等边界是否后端校验 |
| 回归 | focused test、编译、lint、swagger/sqlc/mock 生成步骤 |
| 非目标 | 明确没有处理的相邻问题，避免 review 误以为已覆盖 |
| 剩余风险 | 只写具体风险，不写泛泛“需继续观察” |

### 已完成复核记录

#### OPA-011 完成复核：食安案件默认多区域视图与处置授权

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-011-B/C/D/E`；业务提交 `9e8bef7f fix: authorize operator food safety cases by case region`、`632921a8 fix: aggregate operator food safety cases by managed regions`，以及本次小程序契约 gate 收口；涉及后端食安 handler/test、`trust_score.sql`、sqlc/mock 生成物、`weapp/scripts/check-operator-food-safety-contract.test.js`、`weapp/package.json`、本文档 |
| 设计目标 | 已达成：食安列表默认无参展示当前 operator 全部 active 授权区域；显式 `region_id` 仍 fail closed；详情/调查/结案按案件真实 `region_id` 授权；结案事务传入案件真实区域并保留事务内二次校验；小程序食安页面不传默认 `region_id`，错误不吞成空态，调查/结案后回读详情 |
| 时序 | 列表先解析授权区域集合再查 SQL；SQL 统一做跨区域排序、分页和 total，避免 handler 合并造成乱序分页；详情/写路径先读案件再按案件区域授权；调查/结案后小程序回读详情而不是只靠本地乐观状态 |
| 幂等 | 列表为只读；调查仍由 `UpdateFoodSafetyCaseInvestigation` 的 `status <> 'resolved'` 条件保护；结案仍由 `ResolveFoodSafetyCaseTx` 行锁和已结案检查防重复；小程序提交按钮保留 submitting loading，后端仍是最终真值 |
| 越权 | 默认列表不再信任 legacy `operators.region_id`；legacy 主区域 suspended 时只使用 active `operator_regions` 集合；详情/调查/结案授权事实来自 `operator_id + case.region_id`；非管理区域测试覆盖 403，事务内也拒绝案件区域不匹配 |
| 小程序复核 | `operator-safety.ts` 不本地过滤区域、不吞错成空列表；`safety/report` 区分 loading/error/success-empty/success-list；状态 tab 只传后端支持的 `status` 并重置分页；`safety/detail` 区分 active action 与 resolved read-only；新增 `check:operator-food-safety-contract` 并纳入 `check:operator-capability-audit` |
| 回归 | 红灯：列表多区域测试在修复前因缺少 `region_ids` SQLC 契约和旧默认区域逻辑失败；详情/写路径测试在修复前返回 403 且未检查案件区域；绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'Test.*OperatorFoodSafetyCase' -count=1`、`PATH=/usr/local/go/bin:$PATH make sqlc`、`PATH=/usr/local/go/bin:$PATH make check-generated`、`PATH="$HOME/.local/bin:$PATH" npm run check:operator-food-safety-contract`、`PATH="$HOME/.local/bin:$PATH" npm run check:operator-capability-audit` 均通过 |
| 非目标 | 未新增小程序区域筛选 UI；未改变食安事件触发规则、商户暂停/恢复事务、案件状态枚举；未处理 `OPA-012` 佣金口径和 `OP-RISK-001/002` |
| 剩余风险 | 未做真机截图或弱网手工演练；当前机器化 gate 覆盖契约、错误态和回读路径，但未验证 TDesign 组件在所有机型上的视觉表现 |

#### OPA-011-B 子任务复核：食安案件列表默认多区域聚合

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-011-B`；涉及 `locallife/api/operator_food_safety_cases.go`、`locallife/api/operator_food_safety_cases_test.go`、`locallife/db/query/trust_score.sql`、`locallife/db/sqlc/querier.go`、`locallife/db/sqlc/trust_score.sql.go`、`locallife/db/mock/store.go` |
| 设计目标 | 已达成当前子任务目标：食安案件列表从 `getOperatorRegionID()` 切换为 `resolveOperatorRegionSelection()`；默认无参使用全部 active 授权区域；显式 `region_id` 仍由同一 resolver 做 active 授权；列表、status 过滤、total、has_more 统一按 `region_ids` 查询 |
| 时序 | 读路径先解析后端授权区域集合，再进入 SQL 查询；数据库负责跨区域全局 `created_at DESC, id DESC` 排序和分页，避免 handler 先读多区再内存拼接导致分页/total 不一致 |
| 幂等 | 本步只改只读列表，不新增写入和重试副作用；重复请求在同一授权集合和同一分页参数下保持稳定排序 |
| 越权 | 默认列表不再依赖 legacy `operators.region_id`；新增回归覆盖 legacy 主区域 suspended 时只使用 `ListOperatorRegions` 返回的 active 区域集合；显式非授权区域仍由 `resolveOperatorRegionSelection()` fail closed |
| 回归 | 红灯：列表测试在修复前因缺少 `region_ids` SQLC 契约和旧默认区域逻辑失败；绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestListOperatorFoodSafetyCases_(UsesManagedRegionSelection\|DefaultAggregatesManagedRegions\|DefaultExcludesSuspendedLegacyPrimaryRegion)' -count=1` 通过；扩展绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'Test.*OperatorFoodSafetyCase' -count=1` 通过；生成检查：`PATH=/usr/local/go/bin:$PATH make sqlc`、`PATH=/usr/local/go/bin:$PATH make check-generated` 通过 |
| 非目标 | 尚未给小程序食安列表新增区域筛选；未改变食安事件触发、商户暂停/恢复事务、案件状态枚举；未处理 `OPA-012` 佣金和 `OP-RISK-001/002` |
| 剩余风险 | 当前 `OPA-011` 还需做小程序食安页面契约复核，确认后端默认聚合后页面不会把 403/空态误渲染为“无案件”；该复核不阻塞后端列表不变量，但属于 `OPA-011-D/E` 收口范围 |

#### OPA-011-C 子任务复核：食安案件详情与处置按案件真实区域授权

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-011-C` 第一小步；涉及 `locallife/api/operator_food_safety_cases.go`、`locallife/api/operator_food_safety_cases_test.go` |
| 设计目标 | 已达成当前子任务目标：详情、调查、结案不再先解析运营商默认区域；统一先读取案件，再以 `caseRecord.RegionID` 调用 `checkOperatorManagesRegion()`；结案事务入参 `RegionID` 改为案件真实区域，事务内仍保留区域二次校验 |
| 时序 | 读案件后再做授权，避免页面只凭案件 ID 获得操作权；调查/结案仍在授权通过后进入原有状态判断和事务/条件更新，未改变 resolved case、调查报告缺失、并发更新的顺序 |
| 幂等 | 本步未新增写路径；调查仍由 `UpdateFoodSafetyCaseInvestigation` 的 `status <> 'resolved'` 条件防止已结案覆盖；结案仍由 `ResolveFoodSafetyCaseTx` 在事务内锁定案件并拒绝重复结案 |
| 越权 | 已新增回归覆盖：运营商默认区域与案件区域不同但同时管理案件区域时允许详情/调查/结案；案件区域不归属当前运营商时 403；授权事实来自后端 `operator_id + case.region_id`，不依赖前端传参或 legacy 默认区 |
| 回归 | 红灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestOperatorFoodSafetyCaseDetailAndActions_AuthorizeByCaseRegion\|TestOperatorFoodSafetyCaseDetailAndActions_ForbidCrossRegion' -count=1` 在修复前返回 403 且缺少案件区域授权调用；绿灯：同命令通过；扩展绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'Test.*OperatorFoodSafetyCase' -count=1` 通过；`git diff --check` 通过 |
| 非目标 | 尚未完成 `OPA-011-B` 列表默认多区域聚合；尚未改小程序食安列表区域筛选；尚未处理 `OPA-012` 佣金和 `OP-RISK-001/002` |
| 剩余风险 | 当前小程序食安列表无参默认仍走旧的单区域/default 口径，需下一小任务改为全部 active 授权区域并补多区域列表分页/total 回归 |

#### OPA-001 完成复核：运营商区域状态契约重构

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-001-A/B/C/D`；业务提交 `5b642fef fix: return operator region relation status`；涉及 `locallife/api/operator_stats.go`、`locallife/db/query/operator_region.sql`、`locallife/db/sqlc/*operator_region*`、`locallife/db/mock/store.go`、Swagger 生成物、`weapp/miniprogram/pages/operator/_api/operator-basic-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-regions.ts`、`web/src/app/operator/**` 相关运营区域入口 |
| 设计目标 | 已达成：`GET /v1/operator/regions` 返回运营商区域专用 DTO，`status` 明确为 `operator_regions.status`；小程序和 Web 不再把缺失关系状态默认成 `pending/待审核`；管理页展示全部 active/suspended 关系，操作入口只选择 active 关系 |
| 设计修正 | 实现时没有复用 active-only 的 `ListOperatorRegions` 作为展示契约，而是新增 display-only 的 `ListOperatorRegionRelations`；原 `ListOperatorRegions` 保持 active-only，继续适合作为权限/可操作区域语义，避免展示状态和授权状态互相污染 |
| 时序 | 读路径无写入时序；状态来源顺序固定为当前登录 operator -> `operator_regions` 关系 -> 区域基础信息补齐；前端操作入口先拿到足量关系再筛 active，避免第一页刚好是 suspended 时误判无可运营区域 |
| 幂等 | 本项不新增写路径；区域扩展申请、区域暂停/恢复、峰时配置写入均未改变，重复请求语义不受影响 |
| 越权 | `/v1/operator/regions` 仍从服务端登录态解析 operator，不接受前端传入 `operator_id`；具体统计、规则、商户、骑手、峰时等操作接口继续按 active 关系独立校验，不把“能展示 suspended 关系”升级成“能操作 suspended 区域” |
| 小程序复核 | `RegionStatus` 收敛为 `active/suspended`，新增 `unknown` 仅作显示态；移除 `data.status ?? 'pending'`；区域管理列表保留全部关系，区域 picker 只返回 active；新增 `weapp/scripts/check-operator-region-status-contract.test.js` 锁定不能再把缺失状态展示为待审核 |
| Web 兼容复核 | Web `OperatorRegionResponse` 增加关系状态；`rules`、`merchants/manage`、`riders/manage`、`peak-hours`、`regions/stats` 操作入口统一用 `getActiveOperatorRegions`；`regions` 管理页显示 `运营中/已暂停/状态未知`；新增 `web/scripts/check-operator-region-status-contract.test.js` 锁定契约 |
| 回归 | 红灯：后端 focused API 测试和小程序契约脚本在修复前曾按预期失败；绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestListOperatorRegionsAPI_ReturnsOperatorRegionStatus\|Test(GetRegionStatsAPI\|GetOperatorFinanceOverviewAPI\|GetOperatorCommissionAPI)' -count=1` 通过；`PATH=/usr/local/go/bin:$PATH make check-generated` 通过；`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run check:operator-region-status-contract && PATH="$HOME/.local/bin:$PATH" npm run compile && PATH="$HOME/.local/bin:$PATH" npm run lint` 通过；`cd web && node scripts/check-operator-region-status-contract.test.js && PATH="$HOME/.local/bin:$PATH" npm run lint` 通过；`git diff --check` 通过 |
| 非目标 | 未改运营商申请审批状态机；未迁移历史区域数据；未把申请记录状态混入已管理区域列表；未重构全局 operator 区域选择组件；未解决 OPA-004/006/007 等其它状态或错误语义问题 |
| 剩余风险 | `PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListOperatorRegionRelationsIncludesSuspendedForDisplay -count=1` 在执行新增 DB-backed 测试前被本地测试库 migration 状态阻断：`no migration found for version 276: read down for version 276 .: file does not exist`；当前 worktree 迁移文件到 `000274`，该问题属于本地 `locallife_test` schema 版本漂移，需清理测试库或补齐迁移历史后再跑该 persistence 测试 |

#### OPA-005 完成复核：峰时时段列表跨区域读取权限

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-005-A/B/C`；业务提交 `36c91afa fix: enforce operator region auth for peak hour list`；涉及 `locallife/api/delivery_fee.go`、`locallife/api/delivery_fee_test.go`、`locallife/docs/docs.go`、`locallife/docs/swagger.json`、`locallife/docs/swagger.yaml` |
| 设计目标 | 已达成：`GET /v1/operator/regions/:region_id/peak-hours` 在读取 `ListPeakHourConfigsByRegion` 前调用 `checkOperatorManagesRegion`；合法区域继续返回列表/空列表；非管理区域返回 403 |
| 时序 | 读路径无写入时序；关键顺序已通过 mock 约束：非管理区域不会触发 `ListPeakHourConfigsByRegion`，避免先读后拒绝 |
| 幂等 | 读路径无写入幂等问题；create/delete 原有写路径未改动，仍由既有 handler 内授权和审计日志负责 |
| 越权 | 已在 handler 层用当前登录 operator 身份重建 `operator_id + region_id` 授权；页面 path `region_id` 只作为待校验目标，不作为权限事实 |
| 同类审查 | 已复核当前 operator route group 中 path-region GET：`/regions/:region_id/stats`、`/regions/:region_id/delivery-pool/summary`、`/regions/:region_id/delivery-pool` 均已有 `checkOperatorManagesRegion`；排行/趋势/列表/汇总类 query `region_id` 走 `resolveOperatorRegionSelection` 或专用 resolver；本次未发现新的 path-region GET 同类缺口 |
| 回归 | 红灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run TestListPeakHourConfigsAPI -count=1` 曾因缺少 `CheckOperatorManagesRegion` 调用失败；绿灯：同命令通过；扩展回归：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'Test(Create\|List\|Delete)PeakHourConfigAPI' -count=1` 通过；生成检查：`PATH=/usr/local/go/bin:$PATH make swagger`、`PATH=/usr/local/go/bin:$PATH make check-generated` 通过 |
| 非目标 | 未重构全局 operator 区域授权中间件；未改代取费配置 POST/PATCH 的错误语义；未处理食安、追偿等 resource-id 型授权深挖 |
| 剩余风险 | `swag` 生成时仍输出 Go runtime const evaluation warning，但命令退出 0 且 `make check-generated` 报告 generated artifacts are in sync；resource-id 型 operator GET 仍按后续全量审计继续深挖 |

#### OPA-006 完成复核：骑手申请状态与生命周期状态分离

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-006-A/B/C/D`；错误业务提交 `b1ebb921 fix: align operator realtime pending rider status`，纠偏回滚 `51737559 Revert "fix: align operator realtime pending rider status"`，正确业务提交 `35df01de fix: align operator rider lifecycle status semantics`；涉及 `locallife/api/operator_realtime.go`、`locallife/api/operator_merchant_rider.go`、`locallife/api/operator_realtime_test.go`、`locallife/api/operator_merchant_rider_test.go`、Swagger 生成物、`weapp/miniprogram/pages/operator/_api/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts`、`weapp/miniprogram/pages/operator/riders/index.wxml` |
| 设计目标 | 已达成修正后的设计目标：实时统计和骑手管理只使用 `riders.status` 生命周期；小程序不再展示“待入驻/待审骑手”这类后端无真实生命周期支撑的承诺；`pending_rider_count` 作为兼容字段保留但返回 0，避免页面或外部调用方误解为真实区域待审申请数 |
| 设计修正 | 原 finding 把问题判断为 `pending` 与 `pending_approval` 枚举不一致；生产约束和代码链路复核后确认这是状态模型边界错误。`rider_applications.status` 表达申请流程，`riders.status` 表达骑手生命周期，且申请表没有 `region_id`，不能进入运营商区域实时统计 |
| 生产只读证据 | 生产聚合：`rider_applications.status` 为 `approved=4`、`draft=96`；`riders.status` 为 `active=2`、`approved=2`；约束：`rider_applications_status_check` 允许 `draft/submitted/approved`，`riders_status_check` 允许 `approved/active/suspended`；查询未输出姓名、电话、证件等 PII |
| 时序 | 本次只改读路径和展示口径；申请审批时序仍是申请流先推进，批准后形成 `riders` 记录，再进入运营商骑手生命周期统计。没有把未区域化的 submitted application 拼入区域实时统计，也没有改变审核写路径 |
| 幂等 | 不新增写路径；列表、summary、realtime 都是读路径，重复请求只重新读取相同生命周期口径；没有增加重复提交、超时重试或并发写入风险 |
| 越权 | 避免把全局 `rider_applications.submitted` 泄漏给某个运营商区域；现有 rider list/detail/stats 仍沿用当前 operator 区域解析和 rider 归属校验；页面传入 status 只作为筛选条件，不能扩大可见范围 |
| 小程序复核 | `RiderStatus` 收敛为 `approved/active/suspended`；`approved` 显示为“待激活”；移除 `pending/pending_approval/rejected/offline` 生命周期承诺；analytics active rider 指标不再显示假的“待审 N”；缺失/未知状态展示为“状态未知”而不是降级成 pending |
| 回归 | 后端：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestGetOperatorRealtimeStatsAPI_DoesNotUseApplicationStatusesForRegionRiders\|TestGetOperatorRiderSummaryAPI\|TestListOperatorRidersAPI' -count=1` 通过；生成检查：`PATH=/usr/local/go/bin:$PATH make check-generated` 通过；小程序：`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run compile` 通过，`cd weapp && PATH="$HOME/.local/bin:$PATH" npm run lint` 通过；提交前 `git diff --check` 通过 |
| 非目标 | 未新增骑手申请区域归属 schema；未实现运营商骑手申请审核 API 或小程序入口；未迁移申请历史数据；未解决 OPA-003 的骑手 keyword 搜索和在线状态筛选；未调整商户状态模型 |
| 剩余风险 | `pending_rider_count` 兼容字段仍存在，虽然已注释并由测试锁定为 0；如果未来要展示运营商待审骑手申请，必须先完成区域归属、授权边界、审批幂等和历史数据迁移设计，否则不能复用该字段补假数 |

#### OPA-007 完成复核：代取费保存错误语义与幂等收口

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-007-A/B/C`；业务提交 `e50b8503 fix: constrain operator delivery fee fallback`、`6bf2fef9 fix: preserve delivery fee active state on create`；涉及 `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts`、`weapp/miniprogram/pages/operator/_services/operator-region-config.ts`、`weapp/miniprogram/pages/operator/delivery-fee/index.ts`、`weapp/miniprogram/pages/operator/delivery-fee/index.wxml`、`weapp/scripts/check-operator-delivery-fee-fallback.test.js`、`weapp/package.json`、`locallife/api/delivery_fee.go`、`locallife/api/delivery_fee_test.go`、Swagger 生成物 |
| 设计目标 | 已达成：PATCH 只有在 `AppError.statusCode === 404` 时才降级 POST；400/401/403/5xx/网络错误不再触发二次 POST；页面保存中禁用重复提交；成功后用后端返回值回填表单；创建配置时可显式保存 `is_active=false` |
| Human-Centered UI Check | 角色是运营商，当前任务是在区域配置页保存一次代取费规则；高频路径是修改数值后一次提交；首屏优先保留表单和保存按钮；刷新/返回时以 GET 后端配置为真值；失败时保留草稿并展示可重试错误；低频的 upsert 接口重构不进入当前页面改动 |
| 时序 | 保存链路为 PATCH -> 仅 404 时 POST -> 使用写接口返回值回填；非 404 错误直接停止并保留用户当前输入。弱网或服务端异常不会再因为 catch-all 进入创建路径；页面重入仍通过原有 `loadConfig()` 从后端恢复真实配置 |
| 幂等 | 前端 `saving` 状态和按钮 loading/disabled 防止重复点击；本项未引入自动重试；POST 仍保留后端唯一约束和 409 语义；未把 PATCH/POST 暗改成无条件 upsert |
| 越权 | 403/401 不再二次 POST 探测；后端 create/update 继续由 operator role、LoadOperator、ValidateOperatorRegionMiddleware 校验 path `region_id`；页面传参只作为待校验目标，不作为权限事实 |
| 后端契约复核 | `updateDeliveryFeeConfig()` 缺配置返回 404，作为唯一前端 fallback 入口；`createDeliveryFeeConfig()` 新增可选 `is_active`，未传时默认 true，显式 false 时创建停用配置并写入审计日志 metadata |
| 回归 | 红灯：`PATH="$HOME/.local/bin:$PATH" npm run check:operator-delivery-fee-fallback` 修复前因缺少 404 predicate 失败；红灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run TestCreateDeliveryFeeConfigAPI_AllowsInactiveConfig -count=1` 修复前因 `arg.IsActive=true` 失败。绿灯：`npm run check:operator-delivery-fee-fallback` 通过；`npm run compile` 通过；`npm run lint` 通过；`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestCreateDeliveryFeeConfigAPI_AllowsInactiveConfig\|Test(Create\|Update\|Get)DeliveryFeeConfigAPI' -count=1` 通过；`PATH=/usr/local/go/bin:$PATH make swagger && PATH=/usr/local/go/bin:$PATH make check-generated` 通过 |
| 扩展验证 | `PATH="$HOME/.local/bin:$PATH" npm run quality:check` 已执行，前置大量 check、lint、compile 通过；最后在既有 full-scan `gate:page-complexity` 阻断，超限文件为 `weapp/miniprogram/pages/merchant/group/application/index.ts`、`weapp/miniprogram/pages/register/merchant/group`、`weapp/miniprogram/pages/rider/deposit/index.ts`，均非本次变更路径 |
| 非目标 | 未新增 `PUT /v1/delivery-fee/regions/:region_id/config` upsert；未重做代取费页面视觉；未改变计费模型、唯一约束或 409 冲突语义；未处理峰时时段编辑能力 |
| 剩余风险 | 未在开发者工具里手工模拟网络超时和连续点击；当前自动化覆盖代码契约、编译和 lint。后续若要把创建/更新收敛为单一幂等 upsert，需要另开后端设计任务，补条件更新、审计日志、重复提交和冲突测试 |

#### OPA-004 完成复核：区域体检统计 DTO 重构

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-004-C/D`；业务提交 `ef9f363f fix: align operator analytics region stats contract`；涉及 `weapp/miniprogram/pages/operator/_api/operator-analytics.ts`、`weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts`、`weapp/miniprogram/pages/operator/analytics/index.ts`、`weapp/miniprogram/pages/operator/analytics/index.wxml`、`weapp/scripts/check-operator-analytics-region-stats-contract.test.js`、`weapp/package.json` |
| 设计目标 | 已达成：`GET /v1/operator/regions/:region_id/stats` 只作为扁平统计真值源，不再把虚构嵌套 DTO 补进页面；区域体检改为展示后端真实返回的 `merchant_count/total_orders/total_gmv/total_commission` 并结合实时统计得到可解释摘要；切区/切时间的旧响应不会覆盖新请求 |
| 设计修正 | 不扩后端虚构分析 DTO，不新增 BI 式深度统计；页面 “区域体检” 只承接真实可解释字段，`merchantText/riderText/orderText/commission` 由显式 adapter 组装，避免页面或 service 直接解引用不存在的 `.merchant_stats/.rider_stats/.order_stats/.financial_stats` |
| 时序 | `analyticsRequestSeq` 作为页面级请求序号，切区或切时间时仅接收最后一次请求结果；旧请求完成后若序号不匹配则丢弃，不把旧区域统计回写到新选区。页面首屏仍按地区加载 regions -> analytics data 的既有顺序执行 |
| 幂等 | 读路径不新增写入；重复进入、重复切区只会重新读取同一后端真值，不会产生副作用。测试脚本与页面逻辑只负责展示和丢弃过期响应 |
| 越权 | 不引入新的区域查询参数或跨区域读取入口；区域统计仍受后端 `checkOperatorManagesRegion` 和现有 `getRegionStats` 契约约束，小程序只消费已授权返回值 |
| 小程序复核 | `OperatorRegionStatsResponse` 收敛为 backend truth；`buildOperatorAnalyticsRegionSummary()` 成为唯一页面适配入口；页面文案从“在线骑手 / 活跃骑手”“履约完成率”收口为“活跃骑手”“订单数”；新增 `weapp/scripts/check-operator-analytics-region-stats-contract.test.js` 锁定契约、渲染和时序保护 |
| 回归 | 红灯：`PATH=\"$HOME/.local/bin:$PATH\" npm run check:operator-analytics-region-stats-contract` 在修复前按预期失败；绿灯：该脚本通过；`PATH=\"$HOME/.local/bin:$PATH\" npm run compile` 通过；`PATH=\"$HOME/.local/bin:$PATH\" npm run lint` 通过；`PATH=/usr/local/go/bin:$PATH go test ./api -run TestGetRegionStatsAPI -count=1` 通过；`PATH=\"$HOME/.local/bin:$PATH\" npm run gate:weapp` 运行到 `gate:page-complexity` 处因仓库既有超限页面失败，失败文件为 `weapp/miniprogram/pages/merchant/group/application/index.ts`、`weapp/miniprogram/pages/register/merchant/group`、`weapp/miniprogram/pages/rider/deposit/index.ts`，与本次变更无关；`git diff --check` 通过 |
| 非目标 | 未扩展后端区域统计 DTO；未新增趋势/绩效/健康度分析；未重构排行榜或实时统计接口；未处理其它 operator 页面中的搜索/筛选漂移 |
| 剩余风险 | 区域体检仍由实时统计 + region stats 组合得出，两个读接口不是同一时刻快照；如果未来要展示强一致 BI 指标，需另行设计统一统计口径或后端聚合接口。当前页面已通过请求序号控制避免旧响应覆盖新选区，但仍属于近实时而非事务一致视图 |

#### OPA-002 完成复核：商户列表搜索契约闭环

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-002-A/B/C/D`；业务提交 `369b33e3 fix: support operator merchant keyword search`；涉及 `locallife/api/operator_merchant_rider.go`、`locallife/api/operator_merchant_rider_test.go`、`locallife/db/query/merchant.sql`、`locallife/db/sqlc/merchant.sql.go`、`locallife/db/sqlc/merchant_test.go`、`locallife/db/sqlc/querier.go`、`locallife/db/mock/store.go`、Swagger 生成物、`weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts`、`weapp/miniprogram/pages/operator/merchants/index.ts`、`weapp/scripts/check-operator-merchant-search-contract.test.js`、`weapp/package.json` |
| 设计目标 | 已达成：`GET /v1/operator/merchants` 明确支持 `keyword`；搜索字段限定为商户名称和手机号；keyword 前后空白会被后端和小程序 service trim；列表与 total 使用同一组 `region_ids/statuses/keyword` 条件；`status=approved` 仍包含 `approved + active`；小程序不再承诺后端不存在的搜索能力 |
| 设计修正 | 实现时没有继续沿用“按区域/状态循环查询后内存合并分页”的旧路径，而是新增 `ListOperatorMerchants` / `CountOperatorMerchants` 统一 SQLC 查询。这样数据库负责跨区域排序、分页和 count，减少多区域分页和 keyword/count 条件漂移 |
| 时序 | 小程序页面新增 `merchantListRequestSeq`；刷新搜索允许启动新请求，不再被旧 `loading=true` 阻塞；旧搜索响应或旧错误返回时若序号过期则丢弃，避免慢网下旧关键词覆盖新关键词结果 |
| 幂等 | 本项为读路径，不新增写入和自动重试；重复搜索只重新读取同一后端真值，不产生副作用；分页请求保持 keyword/status/region 条件一致 |
| 越权 | 后端仍先通过当前登录 operator 解析可管理区域或校验指定 `region_id`，再把授权后的 `RegionIDs` 传入 SQL；keyword 只在已授权区域集合内过滤，不能扩大可见商户范围；跨区域不可见由 SQLC 临时库测试和 API mock 参数共同覆盖 |
| 小程序复核 | `loadOperatorMerchantListPageData()` 发送 trim 后 keyword；页面搜索、清空、状态切换和刷新共用 `loadMerchants()`，请求序号统一收住弱网乱序；新增 `weapp/scripts/check-operator-merchant-search-contract.test.js` 锁定 service trim 和 stale response guard |
| 回归 | 红灯：`PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListOperatorMerchants_SearchesByKeywordWithinManagedRegions -count=1` 修复前因查询方法不存在失败；`PATH="$HOME/.local/bin:$PATH" node scripts/check-operator-merchant-search-contract.test.js` 修复前因 service 未 trim 失败。绿灯：临时库 `TEST_DB_SOURCE="postgresql:///<tmp>?sslmode=disable&host=/var/run/postgresql" PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListOperatorMerchants_SearchesByKeywordWithinManagedRegions -count=1` 通过；`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestListOperatorMerchantsAPI\|TestGetOperatorMerchantSummaryAPI\|TestGetOperatorMerchantAPI\|TestGetOperatorMerchantCapabilitiesAPI\|TestUpdateOperatorMerchantCapabilitiesAPI' -count=1` 通过；`PATH="$HOME/.local/bin:$PATH" npm run check:operator-merchant-search-contract` 通过；`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过；`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过；`PATH=/usr/local/go/bin:$PATH make check-generated` 通过；`git diff --check` 通过 |
| 非目标 | 未支持 `sort_by/sort_order/start_date/end_date`；未引入全文检索或索引迁移；未改变商户审核状态流；未改普通消费者搜索；未处理 OPA-003 骑手 keyword/online_status 漂移 |
| 剩余风险 | 默认本地 `locallife_test` 测试库当前停在不存在于本 worktree 的 migration version 276，直接跑 `go test ./db/sqlc` 会被 `no migration found for version 276: read down for version 276 .: file does not exist` 阻断；本项已用全新临时库跑通 SQLC focused。未做微信开发者工具手工慢网回归，后续可在 OPA-003 后统一做运营商列表页真机回归 |

#### OPA-003 完成复核：骑手列表搜索与在线状态契约闭环

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-003-A/B/C/D`；业务提交 `90e87ac7 fix: support operator rider search filters`；涉及 `locallife/api/operator_merchant_rider.go`、`locallife/api/operator_merchant_rider_test.go`、`locallife/db/query/rider.sql`、`locallife/db/sqlc/rider.sql.go`、`locallife/db/sqlc/rider_test.go`、`locallife/db/sqlc/querier.go`、`locallife/db/mock/store.go`、Swagger 生成物、`weapp/miniprogram/pages/operator/_api/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/_services/operator-rider-management.ts`、`weapp/miniprogram/pages/operator/riders/index.ts`、`weapp/scripts/check-operator-rider-search-contract.test.js`、`weapp/package.json` |
| 设计目标 | 已达成：`GET /v1/operator/riders` 明确支持 `keyword` 和 `online_status=online/offline`；生命周期状态继续限定为 `approved/active/suspended`；搜索字段限定为骑手姓名和手机号；keyword 前后空白会被后端和小程序 service trim；列表与 total 使用同一组 `region_ids/statuses/keyword/online_status` 条件；小程序不再声明后端不支持的 `busy/break` 在线状态或排序参数 |
| 设计修正 | 新增 `ListOperatorRiders` / `CountOperatorRiders` 统一 SQLC 查询，替代 handler 中“按单一区域、是否有 status 分两套查询”的旧路径。这样数据库负责多区域排序、分页和 count，避免 list/count 或 region/status/keyword/online_status 条件漂移；不传 `region_id` 时与商户列表和骑手汇总保持一致，聚合全部可管区域 |
| 时序 | 小程序页面新增 `riderListRequestSeq`；刷新搜索允许启动新请求，不再被旧 `loading=true` 阻塞；旧搜索响应或旧错误返回时若序号过期则丢弃，避免慢网下旧关键词或旧状态覆盖新筛选结果 |
| 幂等 | 本项为读路径，不新增写入和自动重试；重复搜索只重新读取同一后端真值，不产生副作用；分页请求保持 keyword/status/region 条件一致。`online_status` 只读取 `riders.is_online` 当前存储值，不改变骑手上下线状态 |
| 越权 | 后端仍先通过当前登录 operator 解析可管理区域或校验指定 `region_id`，再把授权后的 `RegionIDs` 传入 SQL；keyword 和 online_status 只在已授权区域集合内过滤，不能扩大可见骑手范围；跨区域不可见由 SQLC 临时库测试和 API mock 参数共同覆盖 |
| 小程序复核 | `loadOperatorRiderListPageData()` 发送 trim 后 keyword；移除后端不支持的 `sort_by/sort_order`；`RiderOnlineStatus` 收敛为 `online/offline`；页面搜索、清空、状态切换和刷新共用 `loadRiders()`，请求序号统一收住弱网乱序；新增 `weapp/scripts/check-operator-rider-search-contract.test.js` 锁定 service trim、类型收敛和 stale response guard |
| 回归 | 红灯：`PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListOperatorRiders_SearchesByKeywordAndOnlineStatusWithinManagedRegions -count=1` 修复前因查询方法不存在失败；`PATH=/usr/local/go/bin:$PATH go test ./api -run TestListOperatorRidersAPI -count=1` 修复前因 handler 仍调用 `ListRidersByRegion/ListRidersByRegionWithStatus` 失败；`PATH="$HOME/.local/bin:$PATH" npm run check:operator-rider-search-contract` 修复前因 service 未 trim keyword 失败。绿灯：临时库 `TEST_DB_SOURCE="postgresql:///<tmp>?sslmode=disable&host=/var/run/postgresql" PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListOperatorRiders_SearchesByKeywordAndOnlineStatusWithinManagedRegions -count=1` 通过；`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestListOperatorRidersAPI\|TestGetOperatorRiderAPI\|TestGetOperatorRiderSummaryAPI\|TestGetOperatorRiderStatsAPI\|TestListOperatorMerchantsAPI\|TestGetOperatorMerchantSummaryAPI' -count=1` 通过；`PATH="$HOME/.local/bin:$PATH" npm run check:operator-rider-search-contract` 通过；`PATH="$HOME/.local/bin:$PATH" npm run check:operator-merchant-search-contract` 通过；`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过；`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过；`PATH=/usr/local/go/bin:$PATH make swagger && PATH=/usr/local/go/bin:$PATH make check-generated` 通过；`git diff --check` 通过 |
| 非目标 | 未改变骑手上下线机制、在线心跳或位置更新；未新增在线状态筛选 UI；未把 `pending_approval/rejected/offline` 重新放入生命周期 status；未实现 `start_date/end_date/sort_by/sort_order`；未新增全文检索或索引迁移；未新增骑手申请审核入口 |
| 剩余风险 | 默认本地 `locallife_test` 测试库当前停在不存在于本 worktree 的 migration version 276，直接跑 `go test ./db/sqlc` 会被 `no migration found for version 276: read down for version 276 .: file does not exist` 阻断；本项已用全新临时库跑通 SQLC focused。未做微信开发者工具手工慢网回归，`online_status` 语义为 `riders.is_online` 当前存储值，不承诺强实时心跳状态 |

#### OP-NOUI 完成复核：后端-only 能力归属裁决与权限硬化

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OP-NOUI-A/B`；业务提交 `ae280503 fix: harden operator backend-only capabilities`；涉及 `locallife/api/rules_operator_proxy.go`、`locallife/api/rules_operator_proxy_test.go`、`locallife/db/query/profit_sharing_config.sql`、`locallife/db/sqlc/profit_sharing_config.sql.go`、`locallife/db/sqlc/profit_sharing_config_test.go`、Swagger 生成物 |
| 设计目标 | 已达成：没有因为后端存在接口就反推小程序页面；追偿争议/追偿单裁决为 read-only API-only，待产品确认异常队列任务后再做移动端 IA；分润配置裁决为 read-only 财务观察能力，暂不新增小程序入口；规则引擎代理写路径关闭，保留只读观察入口，复杂规则治理归平台端 |
| 设计修正 | 原计划把 `operators/me/rules/**` 视为“后端已有但小程序无入口”的页面缺口；复核后确认它首先是生产权限缺口：运营商 token 能触达 create/version/publish/rollback/disable，而规则主体没有区域独占所有权模型。修复选择 fail-closed，而不是补移动端页面或继续在 handler 内做局部区域注入 |
| 时序 | 规则写代理关闭后没有发布/回滚/禁用时序；运营商端重复 POST 均稳定返回 403，不会进入规则版本查询或状态更新。分润配置为读路径；SQL 查询在数据库层一次性限定 region 与 merchant 归属，避免先读后过滤造成分页/count 漂移 |
| 幂等 | 规则代理写路径不再执行副作用，因此重复提交、超时重试不会改变规则状态；真正的规则写入仍由平台治理入口承担后续幂等和审计要求。分润配置、追偿争议本轮只读，不新增写幂等问题 |
| 越权 | 规则代理写路径在 handler 前置 403，测试锁定不会调用 `CreateRule`、`CreateRuleVersion`、`GetRuleVersion`、`GetRule`、`ListRuleVersionsByRule` 或 `UpdateRuleStatus`。分润配置 `ListProfitSharingConfigsForRegion` 改为 LEFT JOIN `merchants`，`merchant_id IS NOT NULL` 时必须满足 `merchants.region_id = operator region`，阻断跨区域 merchant-scoped 全局配置泄漏。追偿争议列表/汇总继续走 `resolveOperatorRegionSelection`，详情和追偿单由现有测试覆盖区域授权 |
| 回归 | 红灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run TestOperatorRulesProxyWriteAPIsAreDisabled -count=1` 修复前因写路径继续访问 `CreateRule/CreateRuleVersion/GetRuleVersion/GetRule/ListRuleVersionsByRule` 失败；临时库 `go test ./db/sqlc -run TestListProfitSharingConfigsForRegionExcludesMerchantOverridesOutsideRegion -count=1` 修复前因其他区域商户专属配置可见失败。绿灯：临时库 `TEST_DB_SOURCE="postgresql:///<tmp>?sslmode=disable&host=/var/run/postgresql" PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run TestListProfitSharingConfigsForRegionExcludesMerchantOverridesOutsideRegion -count=1` 通过；`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestOperatorRulesProxyWriteAPIsAreDisabled\|TestListOperatorRulesProxyAPI\|TestListOperatorRuleHitsProxyAPI\|TestListOperatorProfitSharingConfigsAPI\|TestListOperatorRecoveryDisputesAPI\|TestListOperatorRecoveryDisputesSummaryAPI\|TestGetOperatorRecoveryDisputeDetailAPI\|TestGetOperatorClaimRecoveryAPI' -count=1` 通过；`PATH=/usr/local/go/bin:$PATH make sqlc`、`PATH=/usr/local/go/bin:$PATH make swagger`、`PATH=/usr/local/go/bin:$PATH make check-generated`、`git diff --check` 通过 |
| 非目标 | 未新增运营商小程序追偿争议、分润配置或规则代理页面；未移除平台规则治理入口；未重新设计规则引擎的区域独占/共享所有权模型；未改变 `/v1/operator/rules` 现有区域规则配置入口；未新增追偿争议处理写操作 |
| 剩余风险 | `GET /v1/operators/me/rules` 只读代理仍用 rule version JSON 中的 `scope.region_id` / `gray_config.region_id` 判断可见性，适合诊断观察，不适合作为规则编辑所有权模型；如果未来要开放运营商规则命中或规则详情页面，需要先做字段裁剪和移动端 IA review。分润配置列表现在阻断跨区域商户配置，但若未来支持多区域 operator 一次聚合分润配置，需要新增 region 集合查询而不是复用单 region API |

#### OPA-008 完成复核：商户列表成功态可见性

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-008`；业务提交 `ee292a34 fix: restore operator merchant list visibility`；涉及 `weapp/miniprogram/pages/operator/merchants/index.wxml`、`weapp/miniprogram/pages/operator/merchants/index.wxss`、`weapp/scripts/check-operator-merchant-list-viewstate.test.js`、`weapp/package.json` |
| 设计目标 | 已达成：商户列表在成功态直接渲染有稳定高度的 `scroll-view`；无数据空态、列表、加载更多都在同一可见滚动容器内；接口有数据时不再因为布局塌陷而表现为空白 |
| 设计修正 | 该问题不是后端商户搜索/分页契约问题，而是小程序 ViewState 与布局可见性问题；因此修复聚焦页面成功态和 scroll 容器，不扩大后端接口或搜索语义 |
| 时序 | 本项不新增请求时序；既有搜索 stale response guard 继续负责旧请求丢弃；页面渲染层只保证成功态数据到达后有可见承载 |
| 幂等 | 不新增写路径；重复进入或刷新只重新读取列表并渲染，不产生副作用 |
| 越权 | 不新增权限入口；商户可见范围仍由 `GET /v1/operator/merchants` 后端区域授权和查询条件控制 |
| 小程序复核 | 新增 `check:operator-merchant-list-viewstate`，锁定 WXML 成功态结构、空态描述、`.content { min-height: 0; }` 和 `.merchants-scroll { height: 100%; }` |
| 回归 | `PATH="$HOME/.local/bin:$PATH" npm run check:operator-merchant-list-viewstate`、`npm run compile`、`npm run lint` 在业务修复时已通过；本次文档/gate 收口会通过 `npm run check:operator-capability-audit` 复跑小程序专项脚本 |
| 非目标 | 未改商户搜索 SQL、状态筛选、详情页能力标签或商户管理权限；这些已分别由 `OPA-002` 和商户能力脚本覆盖 |
| 剩余风险 | 未记录微信开发者工具或真机截图证据；自动化脚本能锁住结构和高度约束，但不能替代真实设备视觉 smoke |

#### OPA-009 完成复核：调度大厅区域授权口径收敛

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-009`；业务提交 `6182ade7 fix: restore operator dispatch and finance views`；涉及 `locallife/api/delivery_fee.go`、`locallife/api/operator_dispatch_monitor.go`、`locallife/api/operator_dispatch_monitor_test.go`、`locallife/api/operator_stats_test.go` |
| 设计目标 | 已达成：仅有 legacy `operators.region_id` 且没有 `operator_regions` 行的已审核运营商，可以访问自己主区域的调度大厅；已有 active `operator_regions` 的运营商仍按关系表授权；suspended relation 不会被 legacy 主区域绕过 |
| 设计修正 | 不把 legacy 主区域无条件加入所有授权集合，而是在无新关系行时作为兼容 fallback；默认聚合 helper 同步避免把 suspended legacy primary region 纳入财务/统计聚合 |
| 时序 | 本项为读路径授权和聚合口径修复；没有改变调度 alert、delivery pool 或通知状态机；读取前先完成授权判断，不做先读后拒绝 |
| 幂等 | 不新增写路径；重复读取 summary/list 只返回当前授权范围内的调度池视图 |
| 越权 | 重点复核 legacy/new model 并存边界：非管理区域继续拒绝，suspended relation 继续拒绝，legacy fallback 只在无 `operator_regions` 关系时生效 |
| 后端复核 | `operator_dispatch_monitor_test.go` 覆盖 legacy primary-region fallback、active relation、suspended deny；`operator_stats_test.go` 覆盖默认聚合不纳入 suspended legacy primary region |
| 回归 | 业务修复时 `go test ./api -count=1` 通过；focused dispatch/finance tests 通过；本次文档/gate 收口会复跑 `go test ./api -run 'Test(GetOperatorPendingDispatch|ListOperatorPendingDispatch|GetOperatorFinanceOverview|GetOperatorCommission)' -count=1` |
| 非目标 | 未重构全局 operator region resolver；未迁移生产历史数据；未改变调度池业务状态、派单策略或通知 worker |
| 剩余风险 | 生产历史 operator 数据如果同时存在异常 legacy 主区域和多条关系，需要继续用只读 SQL 做样本审计；当前代码测试覆盖了预期兼容和暂停拒绝语义 |

#### OPA-010 完成复核：运营商财务金额展示 ViewModel 收口

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-010`；业务提交 `6182ade7 fix: restore operator dispatch and finance views`；涉及 `weapp/miniprogram/pages/operator/_services/operator-finance.ts`、`weapp/miniprogram/pages/operator/finance/withdraw/index.ts`、`weapp/miniprogram/pages/operator/finance/withdraw/index.wxml`、`weapp/scripts/check-operator-finance-overview-display.test.js`、`weapp/package.json` |
| 设计目标 | 已达成：财务概览金额、比例和佣金行金额均由 service/view model 生成 display 字符串；WXML 只绑定字符串，不再调用 Page formatter；后端有金额时不会因为模板运行时限制只显示裸 `¥` |
| 设计修正 | 不把问题归因于后端 finance overview 数据缺失，而是明确为小程序运行时展示适配漂移；资金类展示格式化所有权上移到 view model |
| 时序 | 本项不改变 finance overview 或 commission 的读取顺序；刷新、返回重入仍以重新拉取后端数据并重建 view model 为准 |
| 幂等 | 不新增写路径；提现创建、账单读取、账户状态均不在本项改动范围内 |
| 越权 | 不新增资金接口或参数；财务数据可见范围仍由后端当前 operator 身份和区域聚合控制 |
| 小程序复核 | 新增 `check:operator-finance-overview-display`，锁定 WXML 不调用 `formatFen/formatShareRatio`，且必须绑定 `totalIncomeDisplay/currentMonthIncomeDisplay/operatorShareRatioDisplay/currentMonthGmvDisplay/currentMonthCommissionDisplay` 等 display 字段 |
| 回归 | 业务修复时 `npm run check:operator-finance-overview-display`、`npm run check:finance-bill-pages`、`npm run compile`、`npm run lint` 均通过；本次文档/gate 收口会通过 `npm run check:operator-capability-audit` 复跑小程序专项脚本 |
| 非目标 | 未改后端财务计算公式、提现流程、宝付账户开户或佣金账单分页；未给 WXML 通用表达式安全 gate 增加全仓 formatter 禁令 |
| 剩余风险 | 自动化脚本能锁住当前财务页 display 字段，但不能替代微信开发者工具或真机 smoke；后续新增资金展示页面必须复用 ViewModel display 字段规则 |

#### OPA-012 完成复核：佣金明细默认聚合与财务口径统一

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OPA-012-A/B/C/D/E`；本次提交涉及 `locallife/api/operator_stats.go`、`locallife/api/operator_stats_test.go`、Swagger 生成物、`weapp/scripts/check-operator-finance-overview-display.test.js`、本文档 |
| 设计目标 | 已达成：`GET /v1/operators/me/commission` 与 `/finance/overview` 共用 `resolveOperatorRegionSelection()`；无 `region_id` 默认聚合全部 active/legacy 可管区域；显式 `region_id` 仍单区域授权；summary、total、total_count、items 都来自同一日期范围和同一区域集合 |
| 设计修正 | 不新增小程序区域筛选 UI。当前财务页和账单页没有区域筛选，因此正确实现是保持两处请求都不传默认 `region_id`，让后端成为默认“全部区域”真值源；小程序 gate 锁定这一点，防止后续又在前端注入单一区域 |
| 时序 | 佣金明细先解析授权区域集合，再读取趋势；多区域先按日期合并、排序，再分页和计算汇总，避免按区域分页后拼接导致 count/page 漂移。财务页 overview 与最近佣金仍并发加载，单项失败显示各自错误态 |
| 幂等 | 本项只改读路径，不新增写入、重试或重复提交副作用；重复请求只重新读取相同授权集合的趋势数据 |
| 越权 | 默认路径不再信任 legacy `operators.region_id` 单区；suspended legacy primary region 被排除；显式 `region_id` 仍调用 `checkOperatorManagesRegion()` fail closed；非授权区域不进入趋势查询 |
| 小程序复核 | `operator-finance.ts` 的 overview、最近佣金、账单页均不注入默认 `region_id`；commission failure 保持显式 `commissionError`，不会吞成正常空列表；`check:operator-finance-overview-display` 新增默认聚合和错误态断言，并已纳入 `check:operator-capability-audit` |
| 回归 | 红灯：新增 `DefaultAggregatesManagedRegions`、`DefaultAggregationExcludesSuspendedLegacyPrimaryRegion` 在旧实现上因调用 `getOperatorRegionID()` 和 `CheckOperatorManagesRegion(operator.region_id)` 失败；绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'Test(GetOperatorFinanceOverviewAPI\|GetOperatorCommissionAPI)' -count=1` 通过；`PATH=/usr/local/go/bin:$PATH make swagger`、`PATH=/usr/local/go/bin:$PATH make check-generated` 通过；`PATH="$HOME/.local/bin:$PATH" npm run check:operator-finance-overview-display`、`PATH="$HOME/.local/bin:$PATH" npm run check:operator-capability-audit` 通过 |
| 非目标 | 未改变财务计算公式、分账订单 SQL、提现/结算账户流程、账单日期范围口径；未处理 `operator_stats.go` 中商户/骑手排行仍需单区域的旧 helper；未处理 `OP-RISK-001/002` |
| 剩余风险 | 未做微信开发者工具或真机 smoke；`parseDateRange()` 仍按既有 UTC 日期边界返回，本次未把日期日末语义纳入修复，若业务要求“包含整天”需另开时间口径任务 |

#### OP-RISK-001 完成复核：分账配置默认多区域聚合与商户范围隔离

| 字段 | 记录 |
| --- | --- |
| 完成范围 | `OP-RISK-001-A/B`；本次提交涉及 `locallife/api/operator_profit_sharing_config.go`、`locallife/api/operator_profit_sharing_config_test.go`、`locallife/db/query/profit_sharing_config.sql`、sqlc/mock 生成物、Swagger 生成物、`locallife/db/sqlc/profit_sharing_config_test.go`、本文档 |
| 设计目标 | 已达成：`GET /v1/operators/me/profit-sharing/configs` 从默认单区域 helper 切换为 `resolveOperatorRegionSelection()`；无 `region_id` 默认聚合全部 active/legacy 可管区域；显式 `region_id` 仍只查询单授权区域并 fail closed；merchant-scoped 配置必须归属授权区域内商户 |
| 设计修正 | 本项不新增小程序入口。分账配置当前仍是 backend-only read-only 能力，后续若进入财务页，应先做只读信息架构和字段解释，不在移动端开放写配置 |
| 时序 | 读路径先解析后端授权区域集合，再进入 SQL；SQL 使用同一 `region_ids` 同时过滤区域默认配置和商户所属区域，避免 handler 层二次拼接造成分页/排序漂移 |
| 幂等 | 本项只改只读列表，不新增写路径、重复提交或外部副作用；重复请求在相同筛选和分页参数下保持稳定 `priority ASC, id DESC` 排序 |
| 越权 | 默认路径不再信任 legacy `operators.region_id` 单区；suspended legacy primary region 被排除；显式 `region_id` 继续经 `checkOperatorManagesRegion()` 校验；SQL 回归覆盖授权区域 A/B 可见、区域 C 和区域 C 商户覆盖不可见 |
| 回归 | 红灯：新增 API 测试在旧实现上会因调用 `getOperatorRegionID()`/单区查询而失败；绿灯：`PATH=/usr/local/go/bin:$PATH go test ./api -run 'TestListOperatorProfitSharingConfigsAPI' -count=1` 通过；`TEST_DB_SOURCE=postgresql:///<tmp>?sslmode=disable&host=/var/run/postgresql PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run 'TestListProfitSharingConfigsForRegionsAggregatesManagedRegionsWithoutMerchantLeakage' -count=1` 通过；`PATH=/usr/local/go/bin:$PATH make sqlc`、`PATH=/usr/local/go/bin:$PATH make swagger`、`PATH=/usr/local/go/bin:$PATH make check-generated` 通过 |
| 非目标 | 未改变平台侧分账配置创建/更新/停用接口；未改变实际订单分账计算、分账订单 SQL、支付/退款/提现流程；未新增运营商小程序页面 |
| 剩余风险 | 默认 `locallife_test` 本地库当前记录到 migration version 277，但本分支 migration 目录到 276，直接跑 `go test ./db/sqlc` 会被 migrate 初始化阻断；本次已用干净临时库验证 SQLC 行为，后续仍建议清理默认测试库漂移 |

## 后续审计工作记录

后续继续在本节追加全量盘点结果。每个能力或页面至少记录：

| 编号 | 域 | 后端能力/页面 | 后端证据 | 小程序证据 | 状态 | Finding |
| --- | --- | --- | --- | --- | --- | --- |
| OPA-001 | 区域管理 | `/v1/operator/regions` 区域列表状态 | `operator_stats.go`, `operator_region.sql`, `operator_region_test.go` | `operator/region/index`, `operator-regions.ts`, `operator-basic-management.ts` | 已修复：`5b642fef` | 已完成复核 |
| OPA-005 | 峰时时段 | `/v1/operator/regions/:region_id/peak-hours` 列表授权 | `delivery_fee.go`, `delivery_fee_test.go` | `operator/region/config`, `operator/timeslot/index`, `delivery-fee.ts` | 已修复：`36c91afa` | 已完成复核 |
| OPA-006 | 骑手管理/实时统计 | `/v1/operator/stats/realtime`、`/v1/operator/riders` 生命周期口径 | `operator_realtime.go`, `operator_merchant_rider.go`, `operator_realtime_test.go`, `operator_merchant_rider_test.go` | `operator/riders/index`, `operator-rider-management.ts`, `operator-analytics-dashboard.ts` | 已修复：`35df01de` | 已完成复核 |
| OPA-007 | 代取费配置 | `/v1/delivery-fee/regions/:region_id/config` PATCH/POST 保存语义 | `delivery_fee.go`, `delivery_fee_test.go`, Swagger 生成物 | `operator/delivery-fee/index`, `delivery-fee.ts`, `operator-region-config.ts` | 已修复：`e50b8503`、`6bf2fef9` | 已完成复核 |
| OPA-002 | 商户管理 | `/v1/operator/merchants` keyword 搜索 | `operator_merchant_rider.go`, `merchant.sql`, `merchant_test.go`, Swagger 生成物 | `operator/merchants/index`, `operator-merchant-management.ts`, `check-operator-merchant-search-contract.test.js` | 已修复：`369b33e3` | 已完成复核 |
| OPA-003 | 骑手管理 | `/v1/operator/riders` keyword/online_status 搜索筛选 | `operator_merchant_rider.go`, `rider.sql`, `rider_test.go`, Swagger 生成物 | `operator/riders/index`, `operator-rider-management.ts`, `check-operator-rider-search-contract.test.js` | 已修复：`90e87ac7` | 已完成复核 |
| OP-NOUI | 后端-only 能力 | 追偿争议/追偿单、分润配置、`/v1/operators/me/rules/**` | `recovery_dispute.go`, `claim_recovery.go`, `operator_profit_sharing_config.go`, `rules_operator_proxy.go`, `profit_sharing_config.sql` | 当前运营商小程序无 backend-only 入口；现有规则配置使用 `/v1/operator/rules` | 已修复/裁决：`ae280503` | 已完成复核 |
| OPA-008 | 商户管理 | 商户列表成功态可见性 | 后端商户列表契约沿用 `OPA-002` | `operator/merchants/index`, `check-operator-merchant-list-viewstate.test.js` | 已修复：`ee292a34` | 已完成复核 |
| OPA-009 | 调度大厅 | `/v1/operator/regions/:region_id/delivery-pool/summary`、`/delivery-pool` 区域授权 | `operator_dispatch_monitor.go`, `delivery_fee.go`, `operator_dispatch_monitor_test.go`, `operator_stats_test.go` | `operator/dispatch-hall/index` | 已修复：`6182ade7` | 已完成复核 |
| OPA-010 | 财务概览 | `/v1/operators/me/finance/overview`、`/commission` 金额展示适配 | `operator_stats_test.go` 覆盖默认聚合边界 | `operator/finance/withdraw/index`, `operator-finance.ts`, `check-operator-finance-overview-display.test.js` | 已修复：`6182ade7` | 已完成复核 |
| OPA-011 | 食安案件 | `/v1/operator/food-safety/cases`、`/:id`、`/investigate`、`/resolve` 默认单区域口径 | `operator_food_safety_cases.go`, `operator_food_safety_cases_test.go`, `trust_score.sql`, sqlc/mock 生成物 | `operator/safety/report`, `operator/safety/detail`, `operator-safety.ts`, `operator-basic-management.ts`, `check-operator-food-safety-contract.test.js` | 已修复：`9e8bef7f`、`632921a8`、本次 gate 收口 | 已完成复核 |
| OPA-012 | 财务佣金 | `/v1/operators/me/commission` 与 `/finance/overview` 默认区域口径不一致 | `operator_stats.go`, `operator_stats_test.go`, Swagger 生成物 | `operator/finance/withdraw`, `operator/finance/bills`, `operator-finance.ts`, `check-operator-finance-overview-display.test.js` | 已修复：本次 OPA-012 提交 | 已完成复核 |
| OP-RISK-001 | 分账配置 | `/v1/operators/me/profit-sharing/configs` 默认单区域口径 | `operator_profit_sharing_config.go`, `profit_sharing_config.sql`, `profit_sharing_config_test.go` | 当前运营商小程序无入口 | 已修复 | 已完成复核 |
| OP-RISK-002 | 区域规则 | `/v1/operator/rules` 无参 legacy 兜底 | `operator_rules.go` | `operator/rules/index` 当前显式传 `region_id` | 待加固 | API 直接调用残余风险 |
