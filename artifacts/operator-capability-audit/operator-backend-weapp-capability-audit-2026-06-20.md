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
| 已确认前后端漂移 | 7 | `OPA-001` 至 `OPA-007` 已落证据链 |
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
| 食安案件 | `GET /v1/operator/food-safety/cases` | `listOperatorFoodSafetyCases` | `safety/report/index` | 已调用，待多区域深挖 |
| 食安案件 | `GET /v1/operator/food-safety/cases/:id` | `getOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，待多区域深挖 |
| 食安案件 | `POST /v1/operator/food-safety/cases/:id/investigate` | `investigateOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，待多区域深挖 |
| 食安案件 | `POST /v1/operator/food-safety/cases/:id/resolve` | `resolveOperatorFoodSafetyCase` | `safety/detail/index` | 已调用，待多区域深挖 |
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
| 佣金账单 | `GET /v1/operators/me/commission` | `getOperatorCommission` | finance withdraw/bills | 已调用，待多区域深挖 |
| 结算账户 | `GET /v1/operators/me/settlement-account` | `getOperatorBaofuSettlementAccount` | finance settlement-account | 已调用 |
| 结算账户 | `POST /v1/operators/me/settlement-account` | `createOperatorBaofuSettlementAccount` | finance settlement-account submit | 已调用 |
| 分账配置 | `GET /v1/operators/me/profit-sharing/configs` | `listOperatorProfitSharingConfigs` | 未发现 | 后端已实现，小程序无入口 |
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
| OPA-001 | 前后端契约漂移 | 区域列表状态 | 已确认 |
| OPA-002 | 小程序承诺但后端不支持 | 商户搜索 keyword | 已确认 |
| OPA-003 | 小程序承诺但后端不支持 | 骑手搜索 keyword、API 类型 online_status | 已确认 |
| OPA-004 | 前后端响应 DTO 漂移 | 分析页区域体检 | 已确认 |
| OPA-005 | 后端权限边界缺口 | 峰时时段列表跨区域读取 | 已确认 |
| OPA-006 | 后端状态枚举漂移 | 实时统计待审骑手 | 已确认 |
| OPA-007 | 错误路径漂移 | 代取费配置 PATCH 失败无差别 POST | 已确认 |
| OP-NOUI-001/002 | 后端已实现但无页面入口 | 追偿争议/追偿单 | 当前确认 |
| OP-NOUI-003 | 后端已实现但无页面入口 | 分账规则配置 | 当前确认 |
| OP-NOUI-004 | 后端已实现但无页面入口 | 规则引擎代理 | 当前确认 |
| OP-CONTRACT-002/005 | 历史候选 | 商户/骑手 summary `region_id` | 当前代码已支持或无页面调用，不计当前缺陷 |

## Findings

### OPA-001: 运营商区域管理页把已生效区域误显示为“待审核”

- 状态：已确认
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

- 状态：已确认
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

### OPA-003: 骑手列表搜索与在线筛选类型没有后端支持

- 状态：已确认
- 风险级别：G1
- 类型：小程序能力承诺漂移、状态语义混用
- 影响页面：`weapp/miniprogram/pages/operator/riders/index`
- 影响接口：`GET /v1/operator/riders`

#### 前端证据

- `riders/index.wxml` 展示搜索框，placeholder 是“搜索骑手姓名、电话”。
- `riders/index.ts` 的 `onSearchChange()` 写入 `searchKeyword` 并触发 `loadRiders(true)`。
- `_services/operator-rider-management.ts` 的 `loadOperatorRiderListPageData()` 把 `keyword` 放入 `RiderQueryParams`。
- `_api/operator-rider-management.ts` 的 `RiderQueryParams` 声明 `keyword?: string` 和 `online_status?: RiderOnlineStatus`。
- 同一文件的 `RiderStatus` 还包含 `offline`，但页面状态 tab 使用的是生命周期状态。

#### 后端证据

- `locallife/api/operator_merchant_rider.go` 中 `listOperatorRidersRequest` 只绑定 `status/region_id/page/limit`。
- `listOperatorRiders()` 只按 `req.Status` 和一个目标区域查询 `ListRidersByRegion` 或 `ListRidersByRegionWithStatus`。
- 后端 status binding 允许 `approved/active/suspended/pending_approval/rejected`，不接受 `offline`；也没有绑定 `online_status` 或 `keyword`。

#### 根因

前端把骑手生命周期状态、在线状态和搜索条件放在同一个 API 类型中，但后端列表接口只实现生命周期状态过滤和分页。搜索关键词和在线状态没有进入 handler/SQL，`offline` 也不是后端生命周期状态。

#### 用户影响

- 运营商搜索骑手姓名或手机号时，列表不按搜索词过滤。
- 后续如果页面接入 `online_status` 或 `offline` status，用户会看到无效筛选或 400 参数错误。
- 多区域运营商从 dashboard 带 `region_id` 进入时可限定区域；直接进入骑手页时仍默认主区域，这一点需继续和产品口径确认。

#### 修复方向

- 拆分前端类型：生命周期状态使用后端 `status` 枚举，在线状态使用独立 `online_status`，不要把 `offline` 放进生命周期 `RiderStatus`。
- 后端如支持搜索/在线筛选，应显式绑定 `keyword/online_status` 并新增 SQL/sqlc 查询。
- 如果暂不支持，应删除页面搜索承诺或在页面上改为本地已加载数据内搜索，并清楚标注作用范围；更推荐后端支持。

#### 建议验证

- 后端：补 `GET /v1/operator/riders?keyword=...`、`online_status=online/offline`、非法 status 的测试。
- 小程序：搜索姓名/手机号、切状态 tab、返回重入后验证筛选状态和列表结果一致。

### OPA-004: 分析页把扁平 `regionStatsResponse` 当成嵌套统计 DTO 使用

- 状态：已确认
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

小程序分析模块使用了一个比后端更丰富的虚构 DTO。当前 service 只对整个 `getRegionStats()` 调用做 `.catch(() => null)`，没有保护成功响应中的嵌套字段缺失；因此后端正常返回 200 时，页面仍可能因读取 `undefined.active_merchants` 抛错。

#### 用户影响

- 选择具体区域后，分析页“区域体检”可能无法渲染，或者整个页面加载失败。
- 如果运行时错误被页面 catch 为通用加载失败，用户会误以为后端不可用，而真实原因是 DTO 漂移。

#### 修复方向

- 二选一收口契约：
  - 后端扩展 `/regions/:id/stats`，按小程序需要返回嵌套 DTO，并更新 Swagger/测试。
  - 或前端改用后端现有扁平字段，只展示真实可得指标；缺失的活跃商户、在线骑手、履约率等从其他已实现接口组合获取。
- 短期前端至少要用安全 adapter，避免对可选嵌套字段直接解引用。

#### 建议验证

- 小程序：在有区域的运营账号进入 `analytics/index`，切换区域，确认“区域体检”不崩溃且字段来自真实后端响应。
- 后端：如果扩 DTO，补 handler/schema 测试并运行 `make swagger`。

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

### OPA-006: 实时统计的“待审骑手”使用了错误状态值

- 状态：已确认
- 风险级别：G2
- 类型：后端状态枚举漂移、运营指标失真
- 影响页面：`dashboard/index`、`analytics/index`
- 影响接口：`GET /v1/operator/stats/realtime`

#### 前端证据

- `_services/operator-workbench.ts` 和 `_services/operator-analytics-dashboard.ts` 读取 `pending_rider_count` 并展示为待审骑手指标。
- `_api/operator-rider-management.ts` 把前端 `pending` 规范化成后端 `pending_approval`，骑手列表 tab “待入驻”实际请求 `pending_approval`。

#### 后端证据

- `operator_merchant_rider.go` 的 `listOperatorRidersRequest` 允许状态包含 `pending_approval`，骑手汇总也使用 `countStatus("pending_approval")`。
- `operator_realtime.go` 的 `getOperatorRealtimeStats()` 统计待审骑手时调用 `CountRidersByRegionWithStatus(... Status: "pending")`。
- `db/sqlc/constants.go` 当前只定义 `RiderStatusApproved/Active/Suspended`，没有统一常量覆盖 `pending_approval`，导致魔法字符串分散。

#### 根因

实时统计和骑手管理列表/汇总使用了不同的待审状态语义：一个用 `pending`，一个用 `pending_approval`。当前小程序页面已经按 `pending_approval` 对齐列表，而实时统计仍使用旧值。

#### 用户影响

- 工作台/分析页待审骑手数量可能偏低或为 0。
- 运营商会误判当前没有待处理骑手，影响审核和调度准备。

#### 修复方向

- 后端统一骑手状态常量，在 `db/sqlc/constants.go` 增补真实 rider status 常量，禁止 handler 魔法字符串。
- `getOperatorRealtimeStats()` 改为统计 `pending_approval`，并补测试与骑手 summary 对齐。
- 如果历史数据仍存在 `pending`，应明确是否迁移或兼容双状态，不能静默只算一个。

#### 建议验证

- 后端：构造 `pending_approval` 骑手，验证 realtime `pending_rider_count` 与 `getOperatorRiderSummary()` 一致。
- 小程序：dashboard/analytics 与骑手列表待入驻 tab 的数量口径一致。

### OPA-007: 代取费配置 PATCH 任意失败都会降级 POST

- 状态：已确认
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

#### 根因

前端为了兼容“配置不存在时创建”的便利路径，把 PATCH 的所有失败都当成“缺配置”处理。权限失败、参数错误、后端 500、网络异常等都会触发 POST，从而掩盖原始错误，并可能制造第二个错误或错误写入尝试。

#### 用户影响

- 真实更新失败原因被二次请求覆盖，页面错误提示不稳定。
- 在弱网或后端异常时，用户可能以为系统尝试了正确的创建/更新流程，但真实状态不明。
- 若后端错误分支存在非 404 但 POST 可成功的情况，可能把原本应失败的更新变成创建路径。

#### 修复方向

- 只在明确的 404/配置不存在业务错误上降级 POST。
- 其它错误直接向页面抛出原始语义，由页面显示可恢复错误。
- 更推荐后端提供单一 upsert 语义接口，或前端先 GET 状态后选择 POST/PATCH，并保留并发冲突处理。

#### 建议验证

- 小程序 service 单测或集成测试：PATCH 404 才 POST；PATCH 403/400/500/网络失败不 POST。
- 后端：确认 POST/PATCH 对权限和幂等语义都有测试覆盖。

## 修复任务计划

本节是后续修复执行的任务蓝图，不是一次性大改清单。执行时按“一个子任务一组最小相关文件、一组 focused 验证、一次 review、一次提交”的节奏推进；每个大问题完成后，再做一次设计目标复核，确认没有把漂移转移到其它页面、接口或状态口径上。

### 总体执行顺序

1. 先修 G3 越权问题：`OPA-005`。任何跨区域读取风险都应先 fail closed。
2. 再修 G2 状态和错误语义：`OPA-001`、`OPA-004`、`OPA-006`、`OPA-007`。这些问题会让运营商看到假状态、假指标或不确定保存结果。
3. 再修 G1 查询承诺漂移：`OPA-002`、`OPA-003`。先定后端搜索/筛选契约，再收口页面承诺。
4. 最后处理后端已实现但小程序无入口的能力：`OP-NOUI-001` 至 `OP-NOUI-004`。先做产品与权限归属裁决，不默认“有接口就加页面”。

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

### OPA-006 修复计划：待审骑手状态枚举统一

背景：实时统计用 `pending` 统计待审骑手，骑手列表和汇总使用 `pending_approval`，导致 dashboard/analytics 指标可能失真。

设计目标：骑手生命周期状态有单一枚举来源；实时统计、列表、汇总、小程序 tab 使用同一待审口径。

边界与非目标：本项不重构骑手全生命周期审批流程，不调整商户状态枚举，不新增在线状态筛选。

时序、幂等、越权关注：
- 读统计无写入幂等问题，但状态口径必须和审核写路径一致。
- 如果生产存在历史 `pending` 数据，需先只读统计数量，再决定迁移、兼容双读或废弃；不能静默改变业务口径。
- 多区域统计必须沿用当前 operator 区域选择逻辑。

子任务：

1. `OPA-006-A` 状态枚举源审计
   - 文件：`locallife/db/sqlc/constants.go`、`locallife/api/operator_merchant_rider.go`、`locallife/api/operator_realtime.go`、骑手申请/审核相关文件。
   - 内容：列出 rider status 全量枚举和写入点，确认真实待审值是 `pending_approval`。
   - 验证：`rg -n '"pending"|"pending_approval"|RiderStatus' locallife` 结果入审计记录。
   - 可提交范围：文档或常量测试，不混业务修复。

2. `OPA-006-B` 常量补齐与魔法字符串替换
   - 文件：`locallife/db/sqlc/constants.go`、`locallife/api/operator_realtime.go`、必要的 handler/service 文件。
   - 内容：补 `RiderStatusPendingApproval`、`RiderStatusRejected` 等真实枚举常量；实时统计改用常量。
   - 验证：`cd locallife && go test ./api -run 'Test.*Realtime|Test.*RiderSummary'`。
   - 可提交范围：常量、引用替换、focused 测试。

3. `OPA-006-C` 历史数据口径决策
   - 文件：迁移脚本仅在确认存在历史 `pending` 且业务要求归并时新增；否则只更新审计文档。
   - 内容：只读查询生产/测试数据中 `pending` 与 `pending_approval` 数量；若需迁移，另开高风险数据修复任务，带回滚和抽样验证。
   - 验证：查询记录不包含敏感凭据；迁移任务必须单独 review。
   - 可提交范围：文档或独立 migration，不和 handler 修复混提。

4. `OPA-006-D` 小程序指标一致性回归
   - 文件：`weapp/miniprogram/pages/operator/_services/operator-workbench.ts`、`_services/operator-analytics-dashboard.ts`、`riders/index.*`
   - 内容：确认待审指标和骑手待入驻 tab 使用同一后端状态，不引入本地转换漂移。
   - 验证：`cd weapp && npm run compile && npm run lint`。
   - 可提交范围：只有发现前端漂移才提交代码，否则更新复核记录。

大问题复核：
- `pending_rider_count` 与骑手 summary/list 待审数量在同一数据集下可对齐。
- 代码中不再出现与待审骑手相关的裸 `"pending"`。
- 历史数据是否需要迁移有明确结论。

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

背景：小程序骑手 API 类型包含 `keyword`、`online_status` 和生命周期 status `offline`，后端只支持生命周期状态过滤，且不支持 keyword/online_status。

设计目标：骑手生命周期、在线状态、搜索关键词分层清楚；后端明确支持哪些查询条件，页面只暴露真实能力。

边界与非目标：本项不改变骑手上下线机制，不新增实时定位或在线心跳系统；若后端没有可靠在线状态源，本轮不做在线筛选 UI。

时序、幂等、越权关注：
- 搜索和筛选是读路径，分页与 keyword/status/online_status 必须绑定。
- 在线状态如果来自实时/缓存数据，必须说明时效和过期语义，不能显示为强实时真值。
- 所有骑手查询必须限定当前 operator 管理区域。

子任务：

1. `OPA-003-A` 骑手状态模型裁决
   - 文件：`weapp/miniprogram/pages/operator/_api/operator-rider-management.ts`、`locallife/api/operator_merchant_rider.go`
   - 内容：确认生命周期枚举：`approved/active/suspended/pending_approval/rejected`；`offline` 从生命周期 status 移除。确认是否有后端在线状态源；没有则删除 `online_status` 类型承诺。
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

### 修复批次建议

| 批次 | 范围 | 目标 | 合并前门槛 |
| --- | --- | --- | --- |
| Batch 1 | `OPA-005` | 先关闭跨区域读取 | 后端权限测试通过，非法区域 fail closed |
| Batch 2 | `OPA-001`、`OPA-006` | 统一运营区域和骑手状态真值 | 后端 DTO/常量、小程序状态映射、生产样本复核 |
| Batch 3 | `OPA-007` | 收口保存错误语义和幂等 | PATCH 非 404 不 POST，弱网/重复点击可恢复 |
| Batch 4 | `OPA-004` | 重构区域体检统计契约 | 后端统计 DTO 与页面 ViewModel 一致，切区时序正确 |
| Batch 5 | `OPA-002`、`OPA-003` | 列表搜索/筛选能力闭环 | keyword/status/分页/count/跨区域测试覆盖 |
| Batch 6 | `OP-NOUI-*` | 后端-only 能力归属裁决 | 先裁决再实现，不用接口反推页面 |

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

## 后续审计工作记录

后续继续在本节追加全量盘点结果。每个能力或页面至少记录：

| 编号 | 域 | 后端能力/页面 | 后端证据 | 小程序证据 | 状态 | Finding |
| --- | --- | --- | --- | --- | --- | --- |
| OPA-001 | 区域管理 | `/v1/operator/regions` 区域列表状态 | `operator_stats.go`, `region.go`, `operator_region.sql` | `operator/region/index`, `operator-regions.ts`, `operator-basic-management.ts` | 已确认漂移 | 已记录 |
