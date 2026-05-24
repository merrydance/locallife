# 宝付宝财通多角色提现后端与小程序接入任务文档

**日期**：2026-05-19  
**范围**：`locallife/` 宝付宝财通提现 HTTP 契约、共享提现服务接线、角色级权限解析、回调/恢复一致性；`weapp/` 商户、平台、运营商、骑手收入提现入口与页面组。  
**风险等级**：G3。原因：涉及多角色资金提现、宝付宝财通二级户、外部支付命令、回调/轮询恢复、重复提交、对象级授权、提现账户归属、前端资金结果承接。  
**当前结论**：后端宝付宝财通提现 HTTP 契约已经落地，商户、平台、运营商、骑手收入小程序均已接入真实宝付提现列表、发起和详情回读；骑手保证金退款仍保持独立微信退款路径。
**执行原则**：先补后端真实契约和测试，再接小程序。前端不得伪造可提现余额、提现成功、提现终态或跨角色 owner 身份。

**2026-05-19 执行进度更新**：

- 已提交后端文档批次：`0df8f47a docs: plan multi-role baofu withdrawal integration`。
- 已提交后端契约批次：`e11784cb feat(api): expose shared baofu withdrawal contract`。
- 已新增小程序共享 API：`weapp/miniprogram/api/baofu-withdrawal.ts`。
- 已新增小程序共享 workflow：`weapp/miniprogram/services/baofu-withdrawal-workflow.ts`。
- 已接入商户提现页面组：
  - `weapp/miniprogram/pages/merchant/finance/withdrawals/index.*`
  - `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.*`
  - `weapp/miniprogram/pages/merchant/finance/withdrawals/detail/index.*`
- 已新增并挂入质量门禁：`weapp/scripts/check-baofu-withdrawal-workflow.js`。
- 本轮小程序验证已通过：`cd weapp && npm run quality:check`。
- 已接入运营商提现页面组：
  - `weapp/miniprogram/pages/operator/finance/withdrawals/index.*`
  - `weapp/miniprogram/pages/operator/finance/withdrawals/create/index.*`
  - `weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.*`
- 已整理运营商财务概览入口：
  - `结算账户`
  - `提现`
- 已接入平台提现页面组：
  - `weapp/miniprogram/pages/platform/finance/withdrawals/index.*`
  - `weapp/miniprogram/pages/platform/finance/withdrawals/create/index.*`
  - `weapp/miniprogram/pages/platform/finance/withdrawals/detail/index.*`
- 已整理平台 dashboard 入口：
  - `结算账户`
  - `提现`
- 已接入骑手收入提现页面组：
  - `weapp/miniprogram/pages/rider/income/withdrawals/index.*`
  - `weapp/miniprogram/pages/rider/income/withdrawals/create/index.*`
  - `weapp/miniprogram/pages/rider/income/withdrawals/detail/index.*`
- 已将骑手收入页提现入口改为由 `weapp/miniprogram/services/rider-income.ts` 组合后端 `getBaofuWithdrawalBalance('rider')`，不使用收入账本累计值推断可提现余额。

---

## 1. 当前审查结论

### 1.1 已存在的后端能力

后端已有宝付提现底层能力：

- `locallife/logic/baofu_withdraw_service.go`
  - `NewBaofuWithdrawService`
  - `QueryBalance`
  - `CreateWithdrawal`
  - `BaofuWithdrawClient.QueryWithdraw`
- `locallife/db/query/baofu_withdrawal_order.sql`
  - `CreateBaofuWithdrawalOrder`
  - `GetBaofuWithdrawalOrder`
  - `GetBaofuWithdrawalOrderByOutRequestNo`
  - `ListBaofuWithdrawalOrdersByOwner`
  - `ListProcessingBaofuWithdrawalOrdersForRecovery`
  - `UpdateBaofuWithdrawalOrderToProcessing`
  - `UpdateBaofuWithdrawalOrderStatus`
- `locallife/api/baofu_callback.go`
  - `/v1/webhooks/baofu/withdraw` 回调入口
- `locallife/worker/baofu_withdrawal_recovery_scheduler.go`
  - 宝付提现处理中订单轮询恢复
- `locallife/worker/task_baofu_withdrawal_fact_application.go`
  - 宝付提现事实应用，防止终态回退

`baofu_withdrawal_orders` 已经按 `owner_type` / `owner_id` 建模，`db/sqlc/constants.go` 也已有四类 owner：

- `merchant`
- `platform`
- `operator`
- `rider`

这说明提现底层链路可以跨角色复用；缺口主要在产品化 HTTP 边界、角色权限解析、前端 workflow 和入口。

### 1.2 后端 HTTP 契约当前状态

后端已新增宝付提现 HTTP 契约，并保留历史微信平台收付通商户提现路径作为旧 provider path：

- 新增商户宝付提现路由：
  - `GET /v1/merchant/finance/baofu-withdrawal/balance`
  - `GET /v1/merchant/finance/baofu-withdrawal/withdrawals`
  - `GET /v1/merchant/finance/baofu-withdrawal/withdrawals/:id`
  - `POST /v1/merchant/finance/baofu-withdrawal/withdraw`
- 新增平台宝付提现路由：
  - `GET /v1/platform/finance/baofu-withdrawal/balance`
  - `GET /v1/platform/finance/baofu-withdrawal/withdrawals`
  - `GET /v1/platform/finance/baofu-withdrawal/withdrawals/:id`
  - `POST /v1/platform/finance/baofu-withdrawal/withdraw`
- 新增运营商宝付提现路由：
  - `GET /v1/operators/me/finance/baofu-withdrawal/balance`
  - `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals`
  - `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals/:id`
  - `POST /v1/operators/me/finance/baofu-withdrawal/withdraw`
- 新增骑手收入宝付提现路由：
  - `GET /v1/rider/income/baofu-withdrawal/balance`
  - `GET /v1/rider/income/baofu-withdrawal/withdrawals`
  - `GET /v1/rider/income/baofu-withdrawal/withdrawals/:id`
  - `POST /v1/rider/income/baofu-withdrawal/withdraw`

保留但不能作为宝付提现复用的旧路由：

- 商户旧提现路由仍是历史微信平台收付通路径：
  - `GET /v1/merchant/finance/account/withdrawals`
  - `GET /v1/merchant/finance/account/withdrawals/:id`
  - `POST /v1/merchant/finance/account/withdraw`
- 这些路由被 `gateEcommerceFundManagementWhenOrdinaryActive(...)` 包裹，并调用 `locallife/api/merchant_finance.go` 中的历史收付通提现实现。
- `merchant_finance.go` 的提现创建调用 `server.ecommerceClient.CreateEcommerceWithdraw(...)`，不是宝付 `BaofuWithdrawService.CreateWithdrawal(...)`。

当前仍需后续确认或加强：

- 宝付提现创建的 `client_request_id` 幂等语义尚未产品化；当前创建接口按后端生成唯一 `out_request_no` 并重新查询余额。
- 平台 owner ID 当前使用后端单一常量，若未来出现多平台开户主体，必须改为配置或主体表解析。
- 骑手收入提现虽然已有后端 route，但前端开放仍要以收入可提现余额口径为准，不能用累计收入推断。

### 1.3 小程序现状

商户提现页面已经从迁移占位改为真实宝付提现：

- `weapp/miniprogram/pages/merchant/finance/withdrawals/index.*`
  - 调用 `getBaofuWithdrawalBalance('merchant')`
  - 调用 `listBaofuWithdrawals('merchant', { page, limit })`
  - 展示可提现余额、处理中金额、结算账户入口、提现记录、加载更多、首屏失败和保留可信数据后的局部刷新失败。
- `weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.*`
  - 进入页重新调用 `getBaofuWithdrawalBalance('merchant')`
  - 提交前用共享 workflow 校验金额、小数位、最低提现金额、可提现余额和禁用原因。
  - 调用 `createBaofuWithdrawal('merchant', { amount })`，成功后进入详情页，不展示到账成功。
- `weapp/miniprogram/pages/merchant/finance/withdrawals/detail/index.*`
  - 调用 `getBaofuWithdrawal('merchant', id)`
  - 展示金额、状态、申请单号、申请时间、更新时间、状态说明。
  - 支持刷新状态；刷新失败保留上次可信记录。

小程序共享层已新增：

- `weapp/miniprogram/api/baofu-withdrawal.ts`
  - 只接受 `role`，不接受 `owner_type` / `owner_id`。
  - 角色 endpoint resolver 固定到后端新增宝付提现 routes。
- `weapp/miniprogram/services/baofu-withdrawal-workflow.ts`
  - 金额格式化和金额输入解析。
  - 余额 view model。
  - 状态 view model。
  - 提交校验。

运营商页面路径名叫 `withdraw`，但实际是财务概览，不是真实提现：

- `weapp/miniprogram/pages/operator/finance/withdraw/index.ts`
- `weapp/miniprogram/pages/operator/finance/withdraw/index.wxml`
- 该页展示收入概览和佣金明细，并有 `宝付结算账户` cell 跳转到 `/pages/operator/finance/settlement-account/index`。

平台小程序已有宝付结算账户页面，但没有平台提现页面组：

- `weapp/miniprogram/pages/platform/finance/settlement-account/index.ts`
- `weapp/miniprogram/pages/platform/finance/settlement-account/submit/index.ts`

骑手有两个容易混淆的资金入口：

- `weapp/miniprogram/pages/rider/deposit/index.ts`
  - 已实现的是骑手保证金退款/退回微信零钱语义。
  - 后端对应 `POST /v1/rider/withdraw` 和 `GET /v1/rider/withdrawals/status`。
  - 这不是宝付收入提现，不能混入本任务。
- `weapp/miniprogram/pages/rider/income/index.ts`
  - 当前是代取费收入账本和结算账户提示。
  - 没有收入提现入口，也没有宝付提现 API。

小程序已有宝付结算账户跨角色复用模式，可作为提现前端复用参考：

- `weapp/miniprogram/api/baofu-account.ts`
  - `BaofuAccountOwnerRole = 'rider' | 'merchant' | 'operator' | 'platform'`
  - `baofuAccountEndpoint(role)` 按角色解析 endpoint。
- `weapp/miniprogram/services/baofu-account-onboarding.ts`
  - 共享开户 workflow，页面按角色传入配置。

---

## 2. 跨角色复用结论

### 2.1 可以复用的部分

后端可以复用：

- `logic.BaofuWithdrawService`
  - `QueryBalance`
  - `CreateWithdrawal`
  - 后续补 `ListWithdrawals` / `GetWithdrawal` 或共享 query helper
- `baofu_withdrawal_orders`
  - 继续用 `owner_type` / `owner_id` 隔离记录。
- 宝付 provider client、签名、DTO、提现回调 parser。
- `/v1/webhooks/baofu/withdraw` 回调入口。
- `baofu_withdrawal_recovery_scheduler` 轮询恢复。
- `task_baofu_withdrawal_fact_application` 事实应用。
- API response item builder、状态文案、安全错误映射。

小程序可以复用：

- 新增 `weapp/miniprogram/api/baofu-withdrawal.ts`
  - 复用 `BaofuAccountOwnerRole`。
  - 用 role endpoint resolver 生成各角色路径。
- 新增 `weapp/miniprogram/services/baofu-withdrawal-workflow.ts`
  - 金额格式化。
  - `status` / `sync_state` 到中文文案和 TDesign theme 的映射。
  - 可提现、禁用原因、提交中、防重复、刷新失败 view model。
- 列表、创建、详情页的主要状态机：
  - loading
  - first-screen error
  - empty
  - stale refresh
  - submitting
  - processing
  - terminal success / failure / returned

### 2.2 不能复用或不能混用的部分

必须按角色隔离：

- HTTP route 和 middleware。
- 当前登录态到 owner 的解析。
- 创建权限。
- 可提现资产口径。
- 页面入口和产品文案。
- 审计日志里的业务 owner。

不能复用成同一能力：

- 历史商户微信收付通 `/v1/merchant/finance/account/withdraw*`。
- 骑手保证金退款 `/v1/rider/withdraw`。
- 任何前端传入的 `owner_type`、`owner_id`、`merchant_id`、`operator_id`、`rider_id`。

### 2.3 推荐架构

后端采用“共享服务 + 角色 wrapper”：

- 共享服务负责宝付提现业务不变量：
  - 查询绑定账户。
  - 查询宝付余额。
  - 创建 `baofu_withdrawal_orders`。
  - 调用宝付提现。
  - 记录 external payment command。
  - 统一状态和安全错误语义。
- 各角色 handler 只负责：
  - 认证和权限。
  - 从登录态解析 owner scope。
  - 选择 owner type / owner id。
  - 选择角色路径和创建权限。
  - 调用共享 helper 构造 response。

小程序采用“共享 API/workflow + 角色页面壳”：

- 共享 API 不暴露 owner 参数，只接收 role。
- 共享 workflow 只处理通用提现状态，不决定角色业务资产。
- 角色页面负责放在哪个财务入口、展示什么资产名称、是否允许当前阶段启用提现。

---

## 3. 角色范围矩阵

| 角色 | owner_type | owner_id 来源 | 当前前端入口 | 推荐后端 routes | 创建权限 | 本阶段策略 |
| --- | --- | --- | --- | --- | --- | --- |
| 商户 | `merchant` | 当前商户 ID，由 `MerchantStaffMiddleware` / 登录态解析 | `pages/merchant/finance/withdrawals/*` 占位页 | `/v1/merchant/finance/baofu-withdrawal/*` | 商户 `owner`；`manager` 只读 | 第一批接入真实提现 |
| 平台 | `platform` | 平台 owner ID，沿用现有平台开户/分账使用的固定 owner；当前代码里平台分账测试使用 `platform:0`，实现前需确认生产常量 | 无提现页，仅结算账户页 | `/v1/platform/finance/baofu-withdrawal/*` | 平台财务/管理员，按现有 platform finance middleware | 后端 routes 与共享前端能力同批补齐；页面可第二批开放 |
| 运营商 | `operator` | 当前运营商 ID，由 `/v1/operators/me/*` 登录态解析 | `pages/operator/finance/withdraw/index` 实为财务概览 | `/v1/operators/me/finance/baofu-withdrawal/*` | 运营商 owner/财务权限；只读角色不能创建 | 与平台同批补后端；前端把“财务概览”和“提现”拆清 |
| 骑手收入 | `rider` | 当前骑手 ID，由 `RiderMiddleware` 解析 | `pages/rider/income/index` 只读收入账本 | `/v1/rider/income/baofu-withdrawal/*` 或 `/v1/rider/finance/baofu-withdrawal/*` | 骑手本人 | 先写契约，是否开放创建要等收入账本可提现余额定义清楚 |
| 骑手保证金 | 不使用本表 | 当前骑手 ID | `pages/rider/deposit/index` | 现有 `/v1/rider/withdraw` | 骑手本人 | 非宝付收入提现，保持独立，不纳入本任务 |

平台 owner ID 注意事项：

- 现有宝付账户/分账相关代码里平台 owner 可见 `db.BaofuAccountOwnerTypePlatform`，测试中有 `platform:0`。
- 实现前必须确认平台 owner ID 的生产来源，并写成单一 helper/常量，不能在多个 handler 手写 `0`。
- 如果平台实际有多平台开户主体，则不能用固定 `0`，必须从平台主体表或配置解析。

骑手收入注意事项：

- 当前 rider income 页面展示代取费结算账本，但没有“可提现余额”合同。
- 不能用 `summary.totalRiderIncome` 当作可提现余额，已到账收入可能已提现或被冲正。
- 骑手收入提现上线前，后端必须先给出收入钱包/账本的 `withdrawable_amount`、`pending_withdraw_amount`、`total_withdrawn_amount` 或等价字段。

---

## 4. 不可降级的不变量

1. 小程序不得在后端宝付提现 HTTP 契约缺失时展示真实提现表单、提现按钮或提现成功态。
2. 所有角色的 owner 归属、提现账户归属必须由后端从登录态和持久化关系解析，不得信任前端传 `merchant_id`、`operator_id`、`rider_id`、`owner_id`、`owner_type`。
3. 提现请求必须防重复：同一业务提交不得因为重复点击、重试或弱网重入创建多笔宝付提现。
4. 同步创建返回不等于到账成功；小程序必须展示“已受理/处理中/成功/失败/退票/结果未知”。
5. 提现状态以 `baofu_withdrawal_orders` 和宝付回调/查询后的后端事实为准，不以前端本地状态或余额变化猜测。
6. Provider 原始错误、合同号、银行卡、身份证、手机号、宝付原始报文、签名、商户号、终端号等不得进入小程序响应或用户可见文案。
7. 回调、恢复任务和手动详情刷新不能让终态回退为处理中。
8. 后端缺配置、宝付查询失败、状态未知时必须返回稳定中文语义，且记录结构化日志。
9. 任何角色创建提现前，都必须后端实时查询宝付余额或后端可信可提现资产；余额查询失败时不允许前端按旧值提交。
10. 骑手保证金退款和骑手收入提现必须在 API、页面、文案、日志、测试中保持分离。

---

## 5. 目标后端契约

后端补齐后，小程序只接宝付提现新路径；不要复用历史微信收付通提现路径，也不要复用骑手保证金退款路径。

### 5.1 统一响应结构

查询宝付提现账户余额响应：

```json
{
  "account_status": "active",
  "status_desc": "结算账户已开通",
  "available_amount": 120000,
  "pending_amount": 3000,
  "ledger_amount": 123000,
  "frozen_amount": 0,
  "min_withdraw_amount": 100,
  "max_withdraw_amount": 500000000,
  "can_withdraw": true,
  "disabled_reason": ""
}
```

发起提现请求：

```json
{
  "amount": 120000,
  "client_request_id": "optional-client-idempotency-key",
  "remark": "提现"
}
```

发起提现成功响应：

```json
{
  "withdrawal": {
    "id": 91,
    "out_request_no": "MBW100120260519000001",
    "amount": 120000,
    "status": "processing",
    "status_text": "提现处理中",
    "sync_state": "processing",
    "sync_message": "提现已受理，到账结果确认后会同步更新。",
    "created_at": "2026-05-19T10:20:30Z",
    "updated_at": "2026-05-19T10:20:30Z"
  }
}
```

提现记录列表响应：

```json
{
  "withdrawals": [
    {
      "id": 91,
      "out_request_no": "MBW100120260519000001",
      "amount": 120000,
      "status": "processing",
      "status_text": "提现处理中",
      "created_at": "2026-05-19T10:20:30Z",
      "updated_at": "2026-05-19T10:20:30Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 20,
  "total_pages": 1
}
```

单笔提现详情响应：

```json
{
  "id": 91,
  "out_request_no": "MBW100120260519000001",
  "amount": 120000,
  "status": "processing",
  "status_text": "提现处理中",
  "sync_state": "processing",
  "sync_message": "提现结果仍在确认中，请稍后刷新。",
  "created_at": "2026-05-19T10:20:30Z",
  "updated_at": "2026-05-19T10:20:30Z"
}
```

错误响应要求：

- 宝付服务未配置：`503`，用户文案 `提现服务暂不可用，请稍后再试`
- 结算账户未开通：`409`，用户文案 `结算账户未开通，暂不能提现`
- 账户缺少合同号：`409`，用户文案 `结算账户状态异常，请联系平台处理`
- 宝付余额查询失败：`502`，用户文案 `提现账户余额暂不可确认，请稍后刷新`
- 金额低于下限：`400`，用户文案 `提现金额低于最低提现金额`
- 金额超过可提现余额：`409`，用户文案 `可提现余额不足`
- 访问其他 owner 记录：`404` 或 `403`，不得泄露记录存在性

### 5.2 商户 routes

```text
GET  /v1/merchant/finance/baofu-withdrawal/balance
GET  /v1/merchant/finance/baofu-withdrawal/withdrawals?page=1&limit=20
GET  /v1/merchant/finance/baofu-withdrawal/withdrawals/:id
POST /v1/merchant/finance/baofu-withdrawal/withdraw
```

权限：

- `GET`：`MerchantStaffMiddleware("owner", "manager")`
- `POST`：`MerchantStaffMiddleware("owner")`

owner scope：

- `OwnerType: db.BaofuAccountOwnerTypeMerchant`
- `OwnerID: 当前商户 ID`

### 5.3 平台 routes

```text
GET  /v1/platform/finance/baofu-withdrawal/balance
GET  /v1/platform/finance/baofu-withdrawal/withdrawals?page=1&limit=20
GET  /v1/platform/finance/baofu-withdrawal/withdrawals/:id
POST /v1/platform/finance/baofu-withdrawal/withdraw
```

权限：

- 沿用现有 `/v1/platform/finance/*` 管理权限。
- 创建提现必须限制在平台财务或平台管理员，不允许普通运营查看者创建。

owner scope：

- `OwnerType: db.BaofuAccountOwnerTypePlatform`
- `OwnerID: 平台宝付账户 owner ID`
- 实现前先抽出 `resolvePlatformBaofuOwnerID()` 或等价 helper，禁止在 handler 中散落魔法数字。

### 5.4 运营商 routes

```text
GET  /v1/operators/me/finance/baofu-withdrawal/balance
GET  /v1/operators/me/finance/baofu-withdrawal/withdrawals?page=1&limit=20
GET  /v1/operators/me/finance/baofu-withdrawal/withdrawals/:id
POST /v1/operators/me/finance/baofu-withdrawal/withdraw
```

权限：

- 使用运营商当前登录态和 operator middleware。
- 创建提现必须限制在运营商主体 owner 或明确财务角色；如果当前权限模型没有细分，第一版只允许主体 owner。

owner scope：

- `OwnerType: db.BaofuAccountOwnerTypeOperator`
- `OwnerID: 当前运营商 ID`

### 5.5 骑手收入 routes

推荐路径二选一，实施时只能选一个并全链路一致：

```text
GET  /v1/rider/income/baofu-withdrawal/balance
GET  /v1/rider/income/baofu-withdrawal/withdrawals?page=1&limit=20
GET  /v1/rider/income/baofu-withdrawal/withdrawals/:id
POST /v1/rider/income/baofu-withdrawal/withdraw
```

或：

```text
GET  /v1/rider/finance/baofu-withdrawal/balance
GET  /v1/rider/finance/baofu-withdrawal/withdrawals?page=1&limit=20
GET  /v1/rider/finance/baofu-withdrawal/withdrawals/:id
POST /v1/rider/finance/baofu-withdrawal/withdraw
```

推荐使用 `/v1/rider/income/baofu-withdrawal/*`，因为它与 `pages/rider/income/index` 的收入账本语义一致，并能避开现有 `/v1/rider/withdraw` 保证金退款。

权限：

- `RiderMiddleware()`。
- 只能骑手本人操作。

owner scope：

- `OwnerType: db.BaofuAccountOwnerTypeRider`
- `OwnerID: 当前骑手 ID`

开放条件：

- 创建接口可以先不对小程序开放，直到后端收入账本给出明确可提现余额。
- 如果创建接口先落后端，必须在 balance 响应里返回 `can_withdraw: false` 和明确 `disabled_reason`，不能让前端自行猜测。

---

## 6. 后端执行任务卡

### 任务 1：接线 `BaofuWithdrawService` 到 API Server

**Files：**

- 修改：`locallife/api/server.go`
- 修改：`locallife/util/config.go`
- 可选修改：`locallife/main.go`
- 测试：`locallife/api/baofu_withdrawal_contract_test.go`

**步骤：**

1. 在 `api.Server` 增加字段：
   - `baofuWithdrawService *logic.BaofuWithdrawService`
2. 在 `NewServer` 中，当 `config.HasBaofuRuntimeConfig()` 且 `baofuAccountClient != nil` 时构造：
   - `logic.NewBaofuWithdrawService(store, baofuAccountClient, logic.BaofuWithdrawServiceConfig{...})`
3. 配置映射：
   - `CollectMerchantID: config.BaofuCollectMerchantID`
   - `CollectTerminalID: config.BaofuCollectTerminalID`
   - `PayoutMerchantID: config.BaofuPayoutMerchantID`
   - `PayoutTerminalID: config.BaofuPayoutTerminalID`
   - `WithdrawNotifyURL: config.EffectiveBaofuWithdrawNotifyURL()`
4. `util.Config` 当前有 `BaofuNotifyBaseURL`，但没有独立 `BaofuWithdrawNotifyURL` 和 `EffectiveBaofuWithdrawNotifyURL()`。补派生规则：
   - 优先使用 `BAOFU_WITHDRAW_NOTIFY_URL`
   - 为空时，如果 `BAOFU_NOTIFY_BASE_URL` 有值，则使用 `{base}/withdraw`
   - 生产环境开启宝付主业务时必须能得到绝对 URL
5. 不改历史微信收付通 `/account/withdraw*` 路由，也不改 `/v1/rider/withdraw` 保证金退款路由；新增宝付路径，避免 provider 和业务语义混淆。

**验收：**

- `server.baofuWithdrawService` 在宝付配置完整时非空。
- 宝付配置缺失时，新宝付提现 API 返回 `503`，不 panic。
- `BAOFU_WITHDRAW_NOTIFY_URL` 和 `BAOFU_NOTIFY_BASE_URL + /withdraw` 的派生规则都有单元测试或配置测试。

### 任务 2：新增共享 HTTP helper 和 role scope 解析

**Files：**

- 新增：`locallife/api/baofu_withdrawal_handlers.go`
- 新增或修改：`locallife/api/baofu_withdrawal_scope.go`
- 测试：`locallife/api/baofu_withdrawal_contract_test.go`

**步骤：**

1. 定义内部 scope：
   - `type baofuWithdrawalOwnerScope struct { Role string; OwnerType string; OwnerID int64; CanCreate bool }`
2. 定义共享 DTO：
   - `baofuWithdrawalBalanceResponse`
   - `createBaofuWithdrawalRequest`
   - `baofuWithdrawalItem`
   - `baofuWithdrawalsResponse`
3. 定义共享 handler helper：
   - `handleGetBaofuWithdrawalBalance(ctx, scope)`
   - `handleCreateBaofuWithdrawal(ctx, scope)`
   - `handleListBaofuWithdrawals(ctx, scope)`
   - `handleGetBaofuWithdrawal(ctx, scope)`
4. helper 只能接收后端解析出的 scope，不允许从 query/body 读取 owner。
5. `POST` helper 先检查 `scope.CanCreate`，再校验金额和余额。
6. response builder 统一状态：
   - `processing` -> `提现处理中`
   - `succeeded` -> `提现成功`
   - `failed` -> `提现失败`
   - `returned` -> `提现退票`
   - 未识别状态 -> `提现状态确认中`
7. 错误映射必须把 provider/internal error 转成稳定中文文案，不能直接输出 `err.Error()`。

**验收：**

- helper 单测覆盖不同 `owner_type` 不会串记录。
- helper 对缺服务、缺账户、缺合同号、余额查询失败有稳定响应。
- helper 不接受任何前端 owner 字段。

### 任务 3：新增四类角色 routes

**Files：**

- 修改：`locallife/api/server.go`
- 新增或修改：`locallife/api/merchant_baofu_withdrawal.go`
- 新增或修改：`locallife/api/platform_baofu_withdrawal.go`
- 新增或修改：`locallife/api/operator_baofu_withdrawal.go`
- 新增或修改：`locallife/api/rider_baofu_income_withdrawal.go`
- 测试：`locallife/api/baofu_withdrawal_contract_test.go`
- 测试：`locallife/api/casbin_enforcer_test.go` 或对应权限测试

**步骤：**

1. 商户注册：
   - `GET /v1/merchant/finance/baofu-withdrawal/balance`
   - `GET /v1/merchant/finance/baofu-withdrawal/withdrawals`
   - `GET /v1/merchant/finance/baofu-withdrawal/withdrawals/:id`
   - `POST /v1/merchant/finance/baofu-withdrawal/withdraw`
2. 平台注册：
   - `GET /v1/platform/finance/baofu-withdrawal/balance`
   - `GET /v1/platform/finance/baofu-withdrawal/withdrawals`
   - `GET /v1/platform/finance/baofu-withdrawal/withdrawals/:id`
   - `POST /v1/platform/finance/baofu-withdrawal/withdraw`
3. 运营商注册：
   - `GET /v1/operators/me/finance/baofu-withdrawal/balance`
   - `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals`
   - `GET /v1/operators/me/finance/baofu-withdrawal/withdrawals/:id`
   - `POST /v1/operators/me/finance/baofu-withdrawal/withdraw`
4. 骑手收入注册：
   - `GET /v1/rider/income/baofu-withdrawal/balance`
   - `GET /v1/rider/income/baofu-withdrawal/withdrawals`
   - `GET /v1/rider/income/baofu-withdrawal/withdrawals/:id`
   - `POST /v1/rider/income/baofu-withdrawal/withdraw`
5. 每个 route wrapper 只做 scope 解析，然后调用共享 helper。
6. route wrapper 内不得复制提现业务逻辑。

**测试要求：**

- 商户 `owner` 可创建，`manager` 只读不可创建。
- 平台普通非财务角色不可创建。
- 运营商非主体 owner 不可创建。
- 骑手只能访问自己的收入提现记录。
- 任一角色访问另一个 owner 的记录返回 `403` 或 `404`。
- 旧 `/v1/merchant/finance/account/withdraw*` 行为不变。
- 旧 `/v1/rider/withdraw` 保证金退款行为不变。

### 任务 4：补齐 SQL 分页和幂等边界

**Files：**

- 修改：`locallife/db/query/baofu_withdrawal_order.sql`
- 生成：`locallife/db/sqlc/baofu_withdrawal_order.sql.go`
- 生成：`locallife/db/sqlc/querier.go`
- 测试：`locallife/db/sqlc/baofu_withdrawal_order_test.go`

**步骤：**

1. 新增：
   - `CountBaofuWithdrawalOrdersByOwner`
2. `ListBaofuWithdrawalOrdersByOwner` 与 `CountBaofuWithdrawalOrdersByOwner` 必须使用完全一致的 owner 过滤条件。
3. 如采用 `client_request_id`，新增持久化幂等设计；不要只靠前端防重复。
4. 如果短期不落 `client_request_id`，创建接口仍必须：
   - 按后端生成唯一 out request no。
   - 每次创建前重新查询余额。
   - 明确不承诺“同一 client request id 安全重试”。
5. 运行 `make sqlc`。

**验收：**

- `CountBaofuWithdrawalOrdersByOwner` 已生成到 sqlc。
- 分页 total、total_pages 不靠当前数组长度推断。
- sqlc 生成物已提交。

### 任务 5：验证宝付回调、恢复任务和 API 状态一致

**Files：**

- 修改或确认：`locallife/api/baofu_callback.go`
- 修改或确认：`locallife/worker/baofu_withdrawal_recovery_scheduler.go`
- 修改或确认：`locallife/worker/task_baofu_withdrawal_fact_application.go`
- 测试：已有相关测试，按需补 `api` 和 `worker` 回归。

**步骤：**

1. 确认回调签名、宝付商户号/终端号身份校验已经覆盖提现回调。
2. 确认回调入队前能找到 `baofu_withdrawal_orders`。
3. 确认 recovery scheduler 使用 payout merchant 配置查询提现。
4. 确认 `UpdateBaofuWithdrawalOrderStatus` 只允许 `processing` 进入终态，不能让终态回退。
5. API 详情展示必须复用这些状态，不另造前端状态枚举。
6. 如果详情页触发即时查询，只允许查询非终态记录；查询失败返回本地缓存并标记 `sync_state: "stale"`。

**验收命令：**

```bash
cd locallife
go test ./api -run 'TestBaofuWithdrawal|TestBaofuWithdrawCallback|TestCasbin'
go test ./logic -run 'TestBaofuWithdrawService'
go test ./worker -run 'TestBaofuWithdrawal'
make check-baofu-contract
make check-generated
```

---

## 7. 小程序执行任务卡

小程序任务必须等后端任务 1-5 通过后再做。不要提前把占位页改成表单。

### 任务 6：新增跨角色宝付提现 API 封装

**Files：**

- 新增：`weapp/miniprogram/api/baofu-withdrawal.ts`
- 修改：`weapp/miniprogram/api/merchant-finance-account.ts`，只做兼容 re-export 如有必要
- 测试或检查：相关 TypeScript 编译和 lint

**步骤：**

1. 复用 `BaofuAccountOwnerRole`：
   - `merchant`
   - `platform`
   - `operator`
   - `rider`
2. 新增类型：
   - `BaofuWithdrawalBalanceResponse`
   - `BaofuWithdrawalItem`
   - `BaofuWithdrawalsResponse`
   - `CreateBaofuWithdrawalRequest`
3. 新增 endpoint resolver：

```ts
function baofuWithdrawalEndpoint(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '/v1/merchant/finance/baofu-withdrawal'
    case 'platform':
      return '/v1/platform/finance/baofu-withdrawal'
    case 'operator':
      return '/v1/operators/me/finance/baofu-withdrawal'
    default:
      return '/v1/rider/income/baofu-withdrawal'
  }
}
```

4. 新增函数：
   - `getBaofuWithdrawalBalance(role)`
   - `createBaofuWithdrawal(role, data)`
   - `listBaofuWithdrawals(role, params)`
   - `getBaofuWithdrawal(role, id)`
5. API 封装不得接受 `owner_type` 或 `owner_id`。
6. 错误通过 `getErrorUserMessage(...)` 或角色财务错误映射处理，不展示后端英文或 provider 文案。

**验收：**

- 商户封装只调用 `/v1/merchant/finance/baofu-withdrawal/*`。
- 平台封装只调用 `/v1/platform/finance/baofu-withdrawal/*`。
- 运营商封装只调用 `/v1/operators/me/finance/baofu-withdrawal/*`。
- 骑手收入封装只调用 `/v1/rider/income/baofu-withdrawal/*`。
- 没有任何 API 方法允许页面传 owner。

### 任务 7：新增共享提现 workflow

**Files：**

- 新增：`weapp/miniprogram/services/baofu-withdrawal-workflow.ts`
- 修改：`weapp/miniprogram/services/merchant-finance-workflow.ts`，只保留商户财务概览专属逻辑，必要时 re-export 通用 formatter

**步骤：**

1. 新增金额 formatter：
   - `formatFenToYuanText(fen)`
   - `parseYuanInputToFen(input)`
2. 新增状态 view：
   - `buildBaofuWithdrawalStatusView(status, syncState?)`
3. 新增余额 view：
   - `buildBaofuWithdrawalBalanceView(balance)`
   - 输出 `amountDisplay`、`canSubmit`、`disabledReason`。
4. 新增提交 view：
   - 校验两位小数。
   - 校验 `min_withdraw_amount`。
   - 校验 `available_amount`。
5. 不在 workflow 内决定角色入口、页面跳转或 owner。

**验收：**

- 处理中、成功、失败、退票、未知、缓存结果都有中文 view。
- 金额解析覆盖空值、非法数字、三位小数、低于下限、超过余额。

### 任务 8：商户提现页面接真实记录

**Files：**

- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.ts`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.json`
- 修改：对应 `index.wxss`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.*`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/detail/index.*`
- 修改：`weapp/miniprogram/pages/merchant/finance/index.*`

**步骤：**

1. 列表页首屏加载：
   - `getBaofuWithdrawalBalance('merchant')`
   - `listBaofuWithdrawals('merchant', { page: 1, limit: 20 })`
2. 页面状态显式区分：
   - 首屏 loading
   - 首屏失败 + 重试
   - 空记录
   - 有记录
   - 局部刷新失败但保留上次可信数据
3. 展示：
   - 可提现余额
   - 提现处理中金额或宝付 pending amount
   - 最近提现记录
   - 主按钮 `申请提现`
4. 发起页只保留核心提现金额；备注如无业务必要不进主表单。
5. 提交时禁用按钮，避免重复点击。
6. 返回 `processing` 时展示 `提现已受理，到账结果确认后会同步更新。`，不能显示到账成功。
7. 详情页展示金额、状态、申请时间、更新时间、安全失败说明和刷新按钮。
8. 财务页入口名称用 `提现`，不要叫 `宝付提现`。
9. 旧“提现功能已迁移”文案只保留给旧 provider fallback 或深链兜底，不作为新主流程。

**验收：**

- 没有后端数据时不显示 0 元假余额。
- 余额查询失败不允许提交提现。
- 重复点击不会发出多次请求。
- 详情刷新失败时保留上次可信记录。

### 任务 9：运营商财务页拆清概览与提现

**Files：**

- 修改：`weapp/miniprogram/pages/operator/finance/withdraw/index.*`
- 新增：`weapp/miniprogram/pages/operator/finance/withdrawals/index.*`
- 新增：`weapp/miniprogram/pages/operator/finance/withdrawals/create/index.*`
- 新增：`weapp/miniprogram/pages/operator/finance/withdrawals/detail/index.*`
- 修改：`weapp/miniprogram/pages/operator/dashboard/index.*`
- 修改：`weapp/miniprogram/app.json`

**步骤：**

1. 当前 `pages/operator/finance/withdraw/index` 实际是财务概览。第一步先决定命名：
   - 保守做法：保留路径，页面标题改为 `财务概览`，在页面内新增 `提现` 入口。
   - 更干净做法：新增 `finance/overview`，旧路径重定向或保留兼容。
2. 把现有 cell 文案从 `宝付结算账户` 改为用户友好的 `结算账户`，note 可写 `用于接收分账和提现`。
3. 新增 `withdrawals` 页面组，调用 `role='operator'` 的共享 API。
4. 发起提现只展示运营商可提现余额，不展示平台、商户、骑手文案。
5. 如果后端返回 `can_withdraw: false`，主按钮禁用并展示后端安全中文原因。

**验收：**

- 用户看到的是 `财务概览`、`结算账户`、`提现`，不再把“宝付结算账户”当作主业务词。
- 运营商提现不走商户 API。
- 运营商提现页面不能访问或展示其他运营商记录。

### 任务 10：平台提现页面接入

**Files：**

- 新增：`weapp/miniprogram/pages/platform/finance/withdrawals/index.*`
- 新增：`weapp/miniprogram/pages/platform/finance/withdrawals/create/index.*`
- 新增：`weapp/miniprogram/pages/platform/finance/withdrawals/detail/index.*`
- 修改：`weapp/miniprogram/pages/platform/dashboard/dashboard.*`
- 修改：`weapp/miniprogram/app.json`

**步骤：**

1. 平台 dashboard 中已有 `宝付结算账户` 入口，应改成用户友好的 `结算账户`。
2. 新增 `提现` 入口，只有后端确认平台提现权限和 owner ID 后开放。
3. 页面调用 `role='platform'` 的共享 API。
4. 页面文案使用 `平台账户余额` / `平台提现`，不要暴露 `owner_type=platform`。
5. 创建提现前必须展示后端返回的可提现余额，不根据平台 GMV 或账本合计推断。

**验收：**

- 平台提现不走商户或运营商 API。
- 平台 owner ID 不由前端传。
- 无权限角色看不到或不能提交平台提现。

### 任务 11：骑手收入提现接入前置与页面策略

**Files：**

- 修改：`weapp/miniprogram/pages/rider/income/index.*`
- 新增：`weapp/miniprogram/pages/rider/income/withdrawals/index.*`
- 新增：`weapp/miniprogram/pages/rider/income/withdrawals/create/index.*`
- 新增：`weapp/miniprogram/pages/rider/income/withdrawals/detail/index.*`
- 不修改为宝付收入提现：`weapp/miniprogram/pages/rider/deposit/index.*`
- 修改：`weapp/miniprogram/app.json`

**步骤：**

1. 保持 `pages/rider/deposit/index` 的保证金退款逻辑独立：
   - 文案继续围绕保证金。
   - API 继续走 `RiderService.withdrawDeposit(...)` / `/v1/rider/withdraw`。
2. 在 `pages/rider/income/index` 仅当后端 balance 返回收入可提现能力时展示 `提现` 入口。
3. 页面调用 `role='rider'` 的共享 API。
4. 如果后端暂未定义收入可提现余额，则只展示收入账本和结算账户提示，不展示提现入口。
5. 骑手收入提现文案使用 `代取费提现` 或 `收入提现`，不得写成 `保证金提现`。

**验收：**

- 骑手保证金退款和代取费收入提现入口不会互相跳转。
- 骑手收入提现不使用 `summary.totalRiderIncome` 自行推断可提现余额。
- 骑手未开通结算账户时，收入页提示去开通结算账户，不展示可提交提现表单。

### 任务 12：小程序验证

**命令：**

```bash
cd weapp
npm run quality:check
git diff --check
```

**人工验证：**

1. 商户账户未开通：列表页不能提交提现，入口指向结算账户。
2. 商户余额查询失败：页面显示错误和重试，不显示假余额。
3. 商户余额充足：进入发起页，提交后进入处理中详情。
4. 运营商财务概览：`结算账户` 与 `提现` 是两个清晰入口。
5. 平台 dashboard：`结算账户` 与 `提现` 不再都叫宝付账户。
6. 骑手收入页：只有后端返回可提现能力时才出现收入提现。
7. 骑手保证金页：保证金退款不受宝付收入提现改造影响。
8. 任一角色重复点击提交：只发起一次请求。
9. 任一角色详情刷新失败：保留上次可信状态并提示同步失败。

---

## 8. 推荐提交批次

### 批次 1：任务文档

包含：

- 本文档。

提交信息建议：

```bash
git add artifacts/baofu-withdrawal-multi-role-backend-weapp-integration-plan-2026-05-19.md
git commit -m "docs: plan multi-role baofu withdrawal integration"
```

### 批次 2：后端宝付提现共享契约

包含：

- `api.Server` 接线 `BaofuWithdrawService`
- 共享 helper、role scope、统一 response builder
- 配置 `BAOFU_WITHDRAW_NOTIFY_URL`
- 后端测试

提交信息建议：

```bash
git commit -m "feat(api): expose shared baofu withdrawal contract"
```

### 批次 3：后端角色 routes、SQL 和恢复一致性

包含：

- 商户、平台、运营商、骑手收入 routes
- `CountBaofuWithdrawalOrdersByOwner`
- `make sqlc`
- 回调/恢复一致性测试补齐

提交信息建议：

```bash
git commit -m "feat(api): add role-scoped baofu withdrawal routes"
```

### 批次 4：小程序共享 API 和 workflow

包含：

- `weapp/miniprogram/api/baofu-withdrawal.ts`
- `weapp/miniprogram/services/baofu-withdrawal-workflow.ts`

提交信息建议：

```bash
git commit -m "feat(weapp): add shared baofu withdrawal workflow"
```

### 批次 5：商户提现页面

包含：

- 商户提现列表、创建、详情
- 商户财务入口切换

提交信息建议：

```bash
git commit -m "feat(weapp): wire merchant baofu withdrawal flow"
```

### 批次 6：运营商和平台提现页面

包含：

- 运营商财务概览文案整理
- 运营商提现页面组
- 平台提现页面组
- dashboard 入口文案整理

提交信息建议：

```bash
git commit -m "feat(weapp): wire operator and platform withdrawal flows"
```

### 批次 7：骑手收入提现页面

包含：

- 骑手收入提现入口和页面组
- 保证金退款隔离回归

提交信息建议：

```bash
git commit -m "feat(weapp): add rider income withdrawal flow"
```

---

## 9. 当前执行边界

### 9.1 已允许执行的范围

后端宝付提现 HTTP 契约已存在后，当前允许执行并已完成：

1. 小程序新增跨角色宝付提现 API 封装。
2. 小程序新增跨角色宝付提现 workflow。
3. 商户小程序提现列表、发起、详情接入真实宝付提现接口。

商户端真实提现接入必须继续遵守：

- 不调用历史 `/v1/merchant/finance/account/withdraw*` 微信平台收付通提现路径。
- 不调用现有 `/v1/rider/withdraw` 骑手保证金退款路径。
- 不在创建返回后展示到账成功，只展示已受理/处理中和后续详情回读。
- 不允许前端传 `owner_type`、`owner_id` 或其他 owner scope。
- 余额查询失败时不能提交提现。

### 9.2 仍不应马上开放的范围

平台、运营商、骑手收入的小程序真实提现已按后续批次接入；本次执行没有直接复制商户页面后上线，而是逐项关闭以下前置问题：

1. 运营商 `finance/withdraw` 仍保留为财务概览兼容路径，页面内已整理为 `财务概览`、`结算账户`、`提现` 三个清晰入口。
2. 平台 dashboard 的 `宝付结算账户` 已改成用户友好的 `结算账户`，并新增独立 `提现` 入口。
3. 骑手收入提现已与 `pages/rider/deposit/index` 保证金退款保持页面、API、文案隔离。
4. 骑手收入提现入口只根据后端 `getBaofuWithdrawalBalance('rider')` 返回展示，不用收入累计值推断。
5. 平台和运营商创建权限仍由后端 route/middleware 决定，页面只承接 `can_withdraw`、`disabled_reason` 和错误返回。
6. `weapp/scripts/check-baofu-withdrawal-workflow.js` 已扩展到四角色页面组，阻止错误 role、前端 owner 字段、旧商户微信提现路径、骑手保证金退款路径混入宝付收入提现。

---

## 10. 验收总清单

- [x] 后端新增宝付提现共享 service 接线，且不复用历史微信收付通提现 path。
- [x] 后端不改坏现有 `/v1/rider/withdraw` 保证金退款 path。
- [x] 商户、平台、运营商、骑手收入都有 role-scoped routes 或明确阶段性禁用策略。
- [x] 商户 owner 才能发起提现，manager 只能查看。
- [x] 平台提现创建只允许平台财务/管理员。
- [x] 运营商提现创建只允许运营商主体 owner 或明确财务角色。
- [x] 骑手收入提现只允许骑手本人。
- [x] 后端不信任前端 owner 字段。
- [x] 提现创建前查询宝付余额或后端可信可提现资产并校验金额。
- [x] 骑手收入不使用账本累计收入推断可提现余额。
- [x] 提现记录分页有 count query。
- [x] 回调、恢复任务、详情展示不会让终态回退。
- [x] 小程序 API 只调用 `/baofu-withdrawal/*` 新接口。
- [x] 小程序共享 workflow 不决定 owner 和角色权限。
- [x] 商户、平台、运营商、骑手收入页面不互相调用错误角色 API。
- [x] 运营商和平台入口展示 `结算账户` / `提现`，不把 `宝付结算账户` 当作用户主词。
- [x] 骑手保证金退款与骑手收入提现页面、API、文案保持分离。
- [x] 商户小程序提交中态、防重入、结果未知和刷新失败都有页面承接。
- [x] 平台、运营商、骑手收入小程序提交中态、防重入、结果未知和刷新失败都有页面承接。
- [ ] `cd locallife && go test ./api -run 'TestBaofuWithdrawal|TestBaofuWithdrawCallback|TestCasbin'` 通过。
- [ ] `cd locallife && go test ./logic -run 'TestBaofuWithdrawService'` 通过。
- [ ] `cd locallife && go test ./worker -run 'TestBaofuWithdrawal'` 通过。
- [ ] `cd locallife && make check-baofu-contract && make check-generated` 通过。
- [x] `cd weapp && npm run quality:check` 通过。
- [x] `git diff --check` 通过。

---

## 11. 剩余风险记录

后端契约完成且四角色小程序页面组接入后，剩余风险明确落在：

- 宝付提现创建的重复请求幂等策略尚未产品化。
- 宝付提现详情页是否触发即时查询仍需后端确定单写入口，避免 HTTP handler 与 worker 同时写状态。
- 平台 owner ID 当前以后端单一常量承接；如果未来有多平台开户主体，生产来源需要调整。
- 骑手收入可提现余额已由宝付提现 balance 契约承接；若后续收入钱包改为非宝付余额口径，需要同步调整后端契约和 `services/rider-income.ts`。
- 运营商当前 `finance/withdraw` 路径名历史上与页面内容不一致；本次保留兼容路径并在页面内整理为财务概览入口，未来如做路由清理可新增 `finance/overview` 并保留旧路径跳转。
- 本次只运行小程序静态/门禁验证，未在微信开发者工具或真机上人工点击三角色提现流程。

这些风险未关闭前，可以对外宣称“四角色小程序宝付提现页面已接入真实 HTTP 契约并通过小程序质量门禁”，但不能宣称“重复请求具备产品化幂等保障”或“已完成真机验收”。
