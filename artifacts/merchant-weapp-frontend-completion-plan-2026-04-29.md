# 商户 Weapp 前端完善实施计划

日期：2026-04-29

## 目标

按前端架构基线完成商户侧未承接能力的前端完善：先从用户任务和后端真值组织能力，再决定页面组、ViewState、组件边界和 TDesign 表达，避免接口平铺、入口墙和解释性大卡片。

## 设计约束

- 适用角色：商户侧小程序，属于非顾客侧工具型页面。
- 视觉标准：`.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`。
- 页面交付标准：`.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`。
- 架构标准：`.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`。
- 后端事实来源：`locallife/api/server.go` 与 `weapp/miniprogram/api/**`。
- TDesign 组件优先：tabs、cell、tag、button、input、picker、dialog、popup、loading、empty、result、fab 等官方组件。

## 任务域划分

| 任务域 | Owner | 页面策略 | 状态 |
| --- | --- | --- | --- |
| 财务总览、流水、结算、提现、取消提现 | `pages/merchant/finance/**` 页面组 + `services/merchant-finance-workflow.ts` | 新建财务页面组 | 已完成 |
| 结算账户 | `pages/merchant/settings/applyment/settlement-account` | 保持既有页面，财务页只跳转 | 已闭环，不改 |
| 多门店列表 | 后续集团/品牌主体页面 | 本轮不进普通商户入口 | 待讨论 |
| 预订 workbench | `pages/merchant/reservations/index` | 用现有预订页吸收 | 待执行 |
| 追偿争议 | `pages/merchant/claims/detail` | 放在索赔详情上下文 | 待讨论/后续 |
| 会员代录充值 | `pages/merchant/settings/members` | 放在会员动作上下文 | 待讨论/后续 |
| active/applicable/best 读口 | 对应营销/顾客任务域 | 不新增商户入口 | 不作为本轮页面 |
| OCR 批量/死信、Flutter App 运行时 | 诊断/Flutter App owner | 不进 Weapp 商户页 | 不作为本轮页面 |

## 任务卡

### MCW-FE-01 计划固化

风险：G0，文档与任务拆分。

范围：落地本计划，明确任务卡边界、质量门禁和 review 口径。

验收：计划文件存在，后续任务可逐卡更新状态。

质量门禁：文档改动不跑 `npm run quality:check`，执行人工 review。

状态：已完成。

### MCW-FE-02 财务领域基础

风险：G2，资金域、提现、异步结果回读。

范围：

- 新增取消提现 API 封装。
- 新增财务 workflow/service，集中处理金额格式、日期范围、状态映射、提现提交后回读、结果未知和重复提交保护。
- 不在页面中直接解释后端状态。

非目标：不实现 UI 页面。

验收：API 与 workflow 编译通过；状态 ViewModel 不暴露原始错误文案。

质量门禁：`npm run quality:check`。

review：检查资金状态是否以后端真值为准，是否没有乐观成功。

状态：已完成。

### MCW-FE-03 财务首页

风险：G1/G2，资金摘要只读 + 提现入口。

范围：

- 注册 `pages/merchant/finance/index`。
- 首屏只展示账户余额、账户状态和提现主动作。
- 提供提现记录、流水、结算、结算账户入口。
- 使用 page shell + TDesign，不做顶部解释大卡片。

非目标：不在首页堆完整流水和所有账单接口。

验收：弱网首屏 loading/error/empty/refresh-error 可见；入口可达。

质量门禁：`npm run quality:check`。

review：检查首页是否仍只有一个主任务：判断资金状态并进入后续动作。

状态：已完成。

### MCW-FE-04 提现闭环

风险：G2，资金动作、重复点击、未知结果。

范围：

- 注册提现列表、发起提现、提现详情页面。
- 发起提现后 loading/同步态等待查询后端真值。
- 提现详情承接单笔状态、失败原因、刷新结果。
- 取消提现从提现详情进入，承接资格检查、`NOT_APPLY_WITHDRAW`、`APPLY_WITHDRAW`、收款账户、个人证件信息、执照状态声明、材料上传、提交后回读和申请详情。

非目标：不把取消提现做成独立入口页。

验收：提交中禁用、防重复点击、结果未知可刷新、返回重入可恢复。

质量门禁：`npm run quality:check`。

review：重点 review 后端真值、unknown result、重复点击、登录/重入残余风险。

状态：已完成。

### MCW-FE-05 流水与结算页

风险：G1/G2，只读资金账务。

范围：

- 注册流水页和结算页。
- 订单流水页承接订单入账列表，不把服务费、促销和日汇总接口平铺成页面 tabs。
- 结算页承接结算记录列表，不把结算时间线作为独立页面任务。
- 分页、筛选、空态、失败态都由 ViewState 承接。

非目标：不把财务首页扩成全量账单页。

验收：列表分页依据后端 total/total_pages，不用本地长度猜测。

质量门禁：`npm run quality:check`。

review：检查分页、刷新失败保留可信数据、状态文案。

状态：已完成。

### MCW-FE-06 现有页吸收项

风险：G1/G2，按具体页面判断。

范围：

- 预订页评估并接入 workbench 数据源，若现有页面已经满足任务则只记录不改。
- 多门店只在集团/品牌或多门店主体条件下规划，不进普通商户默认入口。
- 追偿争议、会员代录充值若后端契约与产品边界明确，再分别进入对应详情页任务卡。

非目标：不为每个接口新增独立页面。

验收：每个吸收项都有“接入/不接入”的明确结论和原因。

质量门禁：涉及代码则 `npm run quality:check`；仅文档则人工 review。

状态：已完成。

处理结论：

- 预订 workbench：现有预订页已经有独立列表、今日、统计和状态动作，`workbench` 可作为后续数据源收敛项，不在本轮强换，避免 G1 预订页大范围重写。
- 多门店列表：仍按集团/品牌或多门店主体能力处理，不进入普通商户默认入口。
- 追偿争议：应落在索赔详情上下文，但发起/查看的产品边界需逐个确认，本轮不新增独立入口。
- 会员代录充值：应落在会员详情或会员动作上下文，涉及线下收款入账，需确认财务凭证和操作权限后再做。

### MCW-FE-07 最终整体质量复核

风险：G2，跨页面资金域复核。

范围：

- 全量 `npm run quality:check`。
- 对资金域代码做 review。
- 回写本文档任务状态。

验收：门禁通过；未验证路径和剩余风险写清楚。

状态：已完成。

复核结果：

- 已执行最终全量 `npm run quality:check`，eslint、tsc 与全部 Weapp gate 均通过。
- Review 补齐了提现记录、订单流水、结算记录列表的 `total_pages` 分页加载路径，避免只加载第一页。
- 资金动作仍以后端回读结果为页面真值：提现与取消提现提交中禁用重复点击，不把提交成功等同于终态成功。
- 取消提现已完整承接 `NOT_APPLY_WITHDRAW` 与 `APPLY_WITHDRAW`：提现后注销支持营业执照状态声明、企业/个人收款账户、个人证件信息、证明材料、补充材料和申请详情回读；材料使用私有媒体分类 `merchant_cancel_withdraw`。
- 原始后端枚举和诊断文案已收口为中文 ViewModel 文案；`order_source`、`record_type`、未知财务状态、取消提现 `last_error` 不直接展示。
- 剩余风险：`APPLY_WITHDRAW` 的微信上游真实材料审查、商户确认链接打开方式和外部回调时序仍需联调；前端已按后端契约做本地必填和数量校验，但不替代微信官方证明材料判定。

## 执行规则

每张任务卡执行顺序固定：实现 -> `npm run quality:check` -> 代码 review -> 无问题后标记完成 -> 进入下一张。若门禁或 review 发现问题，先修复再继续。