# 骑手侧后端接口与 Weapp 映射矩阵

日期：2026-04-07

## 适用范围

- 后端骑手侧合同：`locallife/api/rider.go`、`locallife/api/rider_application.go`、`locallife/api/delivery.go`、`locallife/api/appeal.go`、`locallife/api/claim_recovery.go`、`locallife/api/server.go`
- 小程序骑手页：`weapp/miniprogram/pages/rider/**`
- 小程序骑手注册页：`weapp/miniprogram/pages/register/rider/index.ts`
- 小程序骑手 API 层：`weapp/miniprogram/api/rider.ts`、`weapp/miniprogram/api/rider-application.ts`、`weapp/miniprogram/api/delivery.ts`、`weapp/miniprogram/api/appeals-customer-service.ts`

## 目的

本文件回答三件事：

1. 后端骑手侧当前实际暴露了哪些接口。
2. 这些接口分别承载了哪些骑手能力。
3. Weapp 小程序骑手侧哪些页面已经承接，哪些还没有承接，哪些存在合同漂移。

本文件以 2026-04-07 当前代码事实为准。

## 单一事实来源

优先级从高到低如下：

1. `locallife/api/server.go` 中的真实路由注册。
2. `locallife/api/*.go` 中 handler 的请求、响应和权限语义。
3. `weapp/miniprogram/api/*.ts` 中前端 API 封装的真实 URL。
4. `weapp/miniprogram/pages/rider/**` 与 `weapp/miniprogram/pages/register/rider/index.ts` 中页面对 API 的实际消费。

## 审计结论摘要

- Weapp 当前骑手侧共有 7 个已注册工作台页面，另有 1 个骑手入驻页，已覆盖入驻、上线接单、抢单、履约、导航、押金账户、索赔与申诉主链路。
- 后端骑手合同并不只在 `/v1/rider/**` 下，真实骑手能力还分散在 `/v1/delivery/**`，并依赖统一媒体与 OCR 基础设施完成证照上传和识别。
- 当前主链路大体已经形成“后端接口 -> weapp API 封装 -> 页面消费”的闭环，尤其是工作台、任务流转、押金充值提现、索赔申诉详情等路径都已落地。
- 主要问题不是大面积缺页，而是合同漂移：骑手申请状态机已被后端收缩为 `draft/submitted/approved`，提交接口同步返回 `approved` 或回退后的 `draft`，但 Weapp 注册页和公共类型仍保留 `rejected`、等待审核等旧语义；同时还保留一套未被页面消费、且状态枚举已过时的 `rider-basic-management.ts` 封装。

## 后端能力域矩阵

| 能力域 | 后端接口 | 主要后端文件 | Weapp 承接页面 | Weapp API 模块 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| 骑手入驻草稿、资料与自动审核 | `GET /v1/rider/application` `PUT /v1/rider/application/basic` `DELETE /v1/rider/application/documents/:document_type` `DELETE /v1/rider/application/health-cert` `POST /v1/rider/application/submit` `POST /v1/rider/application/reset` | `api/rider_application.go` `api/server.go` | `register/rider` | `rider-application.ts` | 已闭环，但状态机存在漂移 |
| 骑手证照上传与 OCR 写回 | 骑手申请页依赖统一媒体上传和 OCR job，页面通过共享媒体/OCR基础设施完成身份证、健康证上传与识别写回 | `api/rider_application.go` `api/server.go` | `register/rider` | `rider-application.ts` `ocr-jobs.ts` `utils/media.ts` | 已闭环 |
| 骑手身份概览与运营状态 | `GET /v1/rider/me` `GET /v1/rider/status` `POST /v1/rider/online` `POST /v1/rider/offline` | `api/rider.go` `api/server.go` | `rider/dashboard` | `rider.ts` | 已闭环 |
| 骑手位置上报 | `POST /v1/rider/location` | `api/rider.go` `api/server.go` | `rider/dashboard` `rider/task-detail` `rider/navigation` | `rider.ts` `utils/rider-location.ts` `utils/rider-live-location.ts` | 已闭环 |
| 押金账户与账单 | `GET /v1/rider/deposit` `POST /v1/rider/deposit` `POST /v1/rider/withdraw` `GET /v1/rider/deposits` | `api/rider.go` `api/server.go` | `rider/deposit` `rider/dashboard` 间接跳转 | `rider.ts` `payment.ts` | 已闭环 |
| 抢单大厅 | `GET /v1/delivery/recommend` `POST /v1/delivery/grab/:order_id` | `api/delivery.go` `api/server.go` | `rider/dashboard` | `delivery.ts` | 已闭环 |
| 我的进行中任务 | `GET /v1/delivery/active` | `api/delivery.go` `api/server.go` | `rider/dashboard` | 页面直接请求；辅以 `delivery.ts` 类型 | 已闭环 |
| 履约状态流转 | `POST /v1/delivery/:delivery_id/start-pickup` `POST /v1/delivery/:delivery_id/confirm-pickup` `POST /v1/delivery/:delivery_id/start-delivery` `POST /v1/delivery/:delivery_id/confirm-delivery` | `api/delivery.go` `api/server.go` | `rider/dashboard` `rider/task-detail` | `delivery.ts` | 已闭环 |
| 任务详情与历史任务 | `GET /v1/delivery/order/:order_id` `GET /v1/delivery/history` | `api/delivery.go` `api/server.go` | `rider/task-detail` `rider/tasks` | `delivery.ts`；历史页直接请求 | 已闭环 |
| 配送轨迹与导航 | `GET /v1/delivery/:delivery_id/track` `GET /v1/delivery/:delivery_id/rider-location` | `api/delivery.go` `api/server.go` | `rider/task-detail` `rider/navigation` | `delivery.ts` | 已闭环 |
| 骑手索赔与追偿 | `GET /v1/rider/claims` `GET /v1/rider/claims/summary` `GET /v1/rider/claims/:id` `GET /v1/rider/claims/:id/decision` `GET /v1/rider/claims/behavior-summary` `GET /v1/rider/claims/:id/recovery` `POST /v1/rider/claims/:id/recovery/pay` | `api/appeal.go` `api/claim_recovery.go` `api/server.go` | `rider/claims` `rider/claims/detail` | `appeals-customer-service.ts` `payment.ts` | 已闭环 |
| 骑手申诉 | `POST /v1/rider/appeals` `GET /v1/rider/appeals` `GET /v1/rider/appeals/:id` | `api/appeal.go` `api/server.go` | `rider/claims` `rider/claims/detail` | `appeals-customer-service.ts` | 已闭环 |

## Weapp 页面映射矩阵

| 页面组 | 页面路径 | 对应 API 模块 | 对应后端接口 | 页面能力 |
| --- | --- | --- | --- | --- |
| 骑手注册 | `pages/register/rider/index` | `rider-application.ts` `ocr-jobs.ts` `utils/media.ts` | `/v1/rider/application*` 以及共享媒体/OCR基础设施 | 协议确认、证照上传、OCR 自动回填、基础资料保存、自动审核提交 |
| 骑手工作台 | `pages/rider/dashboard/index` | `rider.ts` `delivery.ts` `utils/rider-location.ts` `utils/rider-live-location.ts` | `/v1/rider/me` `/v1/rider/status` `/v1/rider/online` `/v1/rider/offline` `/v1/delivery/active` `/v1/delivery/recommend` `/v1/delivery/grab/:order_id` `/v1/delivery/:delivery_id/*` `/v1/rider/location` | 上下线、抢单大厅、我的任务、位置连续上报、履约动作入口、押金不足拦截、跳转钱包和索赔页 |
| 押金与账单 | `pages/rider/deposit/index` | `rider.ts` `payment.ts` | `/v1/rider/deposit` `/v1/rider/deposits` `/v1/rider/withdraw` | 押金余额、配送冻结/提现处理中拆分展示、充值支付、提现、账单分页 |
| 历史任务 | `pages/rider/tasks/index` | 页面直接请求 `request` | `/v1/delivery/history` | 历史配送分页、累计收益、跳转详情与地图 |
| 任务详情 | `pages/rider/task-detail/index` | `delivery.ts` `utils/rider-live-location.ts` | `/v1/delivery/order/:order_id` `/v1/delivery/:delivery_id/track` `/v1/delivery/:delivery_id/rider-location` `/v1/delivery/:delivery_id/start-pickup` `/confirm-pickup` `/start-delivery` `/confirm-delivery` | 任务详情、轨迹地图、履约状态推进、定位补发状态展示 |
| 导航页 | `pages/rider/navigation/index` | `delivery.ts` `utils/rider-live-location.ts` | `/v1/delivery/order/:order_id` `/v1/delivery/:delivery_id/track` `/v1/delivery/:delivery_id/rider-location` | 下一站导航、轨迹渲染、连续定位状态提示 |
| 索赔与申诉列表 | `pages/rider/claims/index` | `appeals-customer-service.ts` | `/v1/rider/claims` `/v1/rider/claims/summary` `/v1/rider/appeals` | 索赔记录、待处理分桶、申诉记录、分页加载 |
| 索赔详情 | `pages/rider/claims/detail/index` | `appeals-customer-service.ts` `payment.ts` | `/v1/rider/claims/:id` `/v1/rider/claims/:id/decision` `/v1/rider/claims/behavior-summary` `/v1/rider/claims/:id/recovery` `/v1/rider/claims/:id/recovery/pay` `/v1/rider/appeals` `/v1/rider/appeals/:id` | 判责详情、行为摘要、提交申诉、支付追偿、回读申诉详情 |

## 重点缺口清单

### 后端已暴露，但 Weapp 未直接承接或未形成独立入口

| 能力 | 路由 | 说明 |
| --- | --- | --- |
| 重置待处理申请 | `POST /v1/rider/application/reset` | 后端保留了 submitted -> draft 的重置能力，当前注册页未直接调用 |
| 单独删除健康证 | `DELETE /v1/rider/application/health-cert` | 页面统一走 `DELETE /v1/rider/application/documents/:document_type`，未单独消费该别名接口 |

### 主链路已接，但合同语义存在漂移

| 主题 | 实际情况 | 影响 |
| --- | --- | --- |
| 骑手申请状态机漂移 | 后端迁移后 `rider_applications` 仅允许 `draft/submitted/approved`；`POST /v1/rider/application/submit` 只会返回 `approved` 或退回后的 `draft`，并通过 `reject_reason` 表达失败原因 | Weapp `register/rider` 与 `api/onboarding.ts` 仍保留 `rejected`、提交后等待审核、驳回后重置等旧语义，后续再改注册流时容易继续按旧合同开发 |
| 未使用且过时的骑手基础 API 封装 | `weapp/miniprogram/api/rider-basic-management.ts` 未被当前骑手页面消费，且其 `RiderStatus` 枚举仍是 `pending/active/suspended/rejected`，与后端当前 `approved/active/suspended` 不一致 | 当前页面虽然主要使用 `rider.ts`，但该旧封装会误导后续开发或生成代码 |
| 任务域封装重复 | 页面同时存在 `delivery.ts`、`delivery-task-management.ts` 以及局部直接 `request('/v1/delivery/...')` 的三套接入方式 | 能力本身已闭环，但契约定义分散，后续接口调整时容易漏改 |

### 页面存在，但本身不是独立后端能力页

| 页面 | 说明 |
| --- | --- |
| `pages/rider/navigation/index` | 本质是任务详情的导航增强页，后端不提供独立导航合同 |
| `pages/rider/task-detail/index` | 同时承载详情、轨迹与状态动作，不是新的后端能力域 |

## 对齐建议

1. 先收口骑手申请状态机：统一 `weapp/miniprogram/api/onboarding.ts`、`weapp/miniprogram/api/rider-application.ts` 和 `pages/register/rider/index.ts` 的状态语义，去掉 `rejected` 幻觉，改为“提交即同步返回 approved 或 draft+reject_reason”。
2. 清理或合并未使用的骑手 API 封装，至少要处理 `rider-basic-management.ts` 和 `delivery-task-management.ts`，避免同一合同出现多套枚举与 URL 写法。
3. 后续若继续扩骑手端能力，不应只搜索 `/v1/rider/**`，必须同时覆盖 `/v1/delivery/**` 以及骑手申请依赖的共享媒体/OCR能力，否则很容易把主链路看成“不完整”。