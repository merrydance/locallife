# 商户侧后端接口与 Weapp 映射矩阵

日期：2026-04-06
更新进度：2026-04-29

## 适用范围

- 后端商户侧合同：`locallife/api/**` 与 `locallife/api/server.go`
- 小程序商户页：`weapp/miniprogram/pages/merchant/**`
- 小程序商户 API 层：`weapp/miniprogram/api/**`

## 目的

本文件回答三件事：

1. 后端商户侧当前实际暴露了哪些接口。
2. 这些接口分别承载了哪些商户能力。
3. Weapp 小程序商户侧哪些页面已经承接，哪些还没有承接，哪些需要产品边界确认。

历史版本可参考：`weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md`。

本文件最初以 2026-04-06 代码事实为准；2026-04-29 已按当前代码做进度更新。完整审计底稿见 `artifacts/merchant-capability-backend-weapp-audit-2026-04-29.md`。

## 单一事实来源

优先级从高到低如下：

1. `locallife/api/server.go` 中的真实路由注册。
2. `locallife/api/*.go` 中 handler 的请求、响应和权限语义。
3. `weapp/miniprogram/app.json` 中真实注册的商户页面。
4. `weapp/miniprogram/api/*.ts` 中前端 API 封装的真实 URL。
5. `weapp/miniprogram/pages/merchant/**` 中页面对 API 的实际消费。

## 2026-04-29 更新进度摘要

- Weapp 商户分包当前注册 58 个页面入口。旧矩阵中的 45 个页面入口已过期。
- `结算账户资料` 已从“未承接”更新为“已闭环”：当前存在 `pages/merchant/settings/applyment/settlement-account/index`，并调用 `merchant-settlement-account.ts` 的查询、修改、申请查询接口。
- `财务总览与提现` 已完成新一轮承接：当前存在 `pages/merchant/finance/**`，并通过 `merchant-finance.ts` 与 `merchant-finance-workflow.ts` 承接总览、余额、提现、流水与结算。
- `取消提现` 已完成商户侧承接：提现详情中提供资格检查、`NOT_APPLY_WITHDRAW`、`APPLY_WITHDRAW`、收款资料、材料上传、提交后回读和申请详情页；它不按超长耗时任务设计，提交后保持 loading/同步态并等待查询后端结果真值。
- `商户多门店列表` 不是所有商户都需要的默认能力，它主要服务集团/品牌或多门店主体；当前应标为“边界待确认”，不是普通单店商户缺口。
- `包装策略` 旧矩阵记录为后端接口与页面均已闭环，但当前未发现 `/v1/merchants/me/packaging-policy` 路由和商户页。代码里只看到订单创建时的包装策略校验逻辑，不能继续作为“商户页已闭环”记录。
- 旧矩阵中的 Swagger 漂移已收口：当前投诉详情/回复/完结、员工邀请码、满减规则相关 `@Router` 注解已与真实路由对齐。

## 后端能力域矩阵

| 能力域 | 后端接口 | 主要后端文件 | Weapp 承接页面 | Weapp API 模块 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| 工作台聚合 | `GET /v1/merchant/orders/summary` `GET /v1/merchant/orders/stats` `GET /v1/merchant/stats/overview` `GET /v1/merchant/stats/hourly` `GET /v1/merchant/stats/sources` `GET /v1/merchant/stats/repurchase` `GET /v1/merchant/complaints/summary` `GET /v1/reservations/merchant/stats` `GET /v1/merchants/me` `GET/PATCH /v1/merchants/me/status` | `api/order.go` `api/merchant_stats.go` `api/complaint.go` `api/table_reservation.go` `api/merchant.go` | `merchant/dashboard` | `order-management.ts` `merchant-stats.ts` `merchant-complaints.ts` `reservation.ts` `merchant.ts` | 已闭环 |
| 商户 App 绑定 | `POST /v1/auth/app-bind/code`；Flutter App 运行时另有 `POST /v1/merchant/device/register` `PUT /v1/merchant/device/heartbeat` `DELETE /v1/merchant/device/{device_id}` | `api/auth.go` `api/merchant_app_device.go` | `merchant/dashboard` 生成绑定码 | `auth.ts` `merchant-app-bind.ts` | Weapp 只承接绑定码；设备注册/心跳属于 Flutter 商户 App |
| 商户订单 | `GET /v1/merchant/orders` `GET /v1/merchant/orders/{id}` `POST /v1/merchant/orders/{id}/accept` `POST /v1/merchant/orders/{id}/reject` `POST /v1/merchant/orders/{id}/ready` `POST /v1/merchant/orders/{id}/complete` | `api/order.go` | `merchant/orders/list` `merchant/orders/detail` | `order-management.ts` | 已闭环 |
| 打印任务与异常 | `GET /v1/merchant/orders/{id}/print-jobs` `POST /v1/merchant/orders/{id}/print-jobs` `POST /v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry` `GET /v1/merchant/orders/{id}/print-jobs/{print_log_id}/status` `GET /v1/merchant/orders/print-anomalies` | `api/order.go` | `merchant/orders/detail` `merchant/orders/print-anomalies` | `order-management.ts` | 已闭环 |
| 后厨 KDS | `GET /v1/kitchen/orders` `GET /v1/kitchen/orders/{id}` `POST /v1/kitchen/orders/{id}/preparing` `POST /v1/kitchen/orders/{id}/ready` | `api/kitchen.go` | `merchant/kitchen` `merchant/kitchen/detail` | `order-management.ts` | 已闭环 |
| 经营统计 | `GET /v1/merchant/stats/daily` `GET /v1/merchant/stats/overview` `GET /v1/merchant/stats/dishes/top` `GET /v1/merchant/stats/customers` `GET /v1/merchant/stats/customers/{user_id}` `GET /v1/merchant/stats/hourly` `GET /v1/merchant/stats/sources` `GET /v1/merchant/stats/repurchase` `GET /v1/merchant/stats/categories` | `api/merchant_stats.go` | `merchant/stats` `merchant/stats/customers` `merchant/stats/customers/detail` | `merchant-stats.ts` | 已闭环 |
| 财务总览与提现 | `GET /v1/merchant/finance/overview` `GET /v1/merchant/finance/orders` `GET /v1/merchant/finance/service-fees` `GET /v1/merchant/finance/promotions` `GET /v1/merchant/finance/daily` `GET /v1/merchant/finance/settlements` `GET /v1/merchant/finance/settlement-timeline` `GET /v1/merchant/finance/account/balance` `GET /v1/merchant/finance/account/withdrawals` `GET /v1/merchant/finance/account/withdrawals/{id}` `POST /v1/merchant/finance/account/withdraw` | `api/merchant_finance.go` | `merchant/finance` `merchant/finance/withdrawals` `merchant/finance/withdrawals/create` `merchant/finance/withdrawals/detail` `merchant/finance/bills` `merchant/finance/settlements` | `merchant-finance.ts` | 已闭环 |
| 取消提现 | `GET /v1/merchant/finance/account/cancel-withdraw/eligibility` `GET /v1/merchant/finance/account/cancel-withdraw/applications` `GET /v1/merchant/finance/account/cancel-withdraw/applications/{id}` `POST /v1/merchant/finance/account/cancel-withdraw/applications` | `api/merchant_cancel_withdraw.go` | `merchant/finance/withdrawals/detail` `merchant/finance/cancel-withdraw/detail` | `merchant-finance.ts` | 已闭环；`APPLY_WITHDRAW` 上游材料审查和商户确认链接需联调 |
| 收付通进件与绑卡 | `GET /v1/merchant/applyment/status` `POST /v1/merchant/applyment/bindbank` `GET /v1/merchant/applyment/banks` `GET /v1/merchant/applyment/banks/search-by-bank-account` `GET /v1/merchant/applyment/areas/provinces` `GET /v1/merchant/applyment/areas/provinces/{province_code}/cities` `GET /v1/merchant/applyment/banks/{bank_alias_code}/branches` | `api/ecommerce_applyment.go` `api/applyment_bank_catalog.go` | `merchant/settings/applyment` `merchant/settings/applyment/action` `merchant/settings/applyment/submit` | `merchant-applyment.ts` `applyment-bank.ts` | 已闭环 |
| 结算账户资料 | `GET /v1/merchant/finance/account/settlement-account` `POST /v1/merchant/finance/account/settlement-account` `GET /v1/merchant/finance/account/settlement-account/applications/{application_no}` | `api/settlement_account.go` | `merchant/settings/applyment` `merchant/settings/applyment/settlement-account` | `merchant-settlement-account.ts` | 已闭环；旧矩阵未承接结论已过期 |
| 门店基本资料 | `GET /v1/merchants/me` `PATCH /v1/merchants/me` `PATCH /v1/merchants/me/shop-images` | `api/merchant.go` | `merchant/settings/profile` `merchant/profile-images` | `merchant.ts` `onboarding.ts` | 已闭环 |
| 营业状态与营业时间 | `GET/PATCH /v1/merchants/me/status` `GET/PUT /v1/merchants/me/business-hours` | `api/merchant.go` | `merchant/dashboard` `merchant/settings/profile` `merchant/settings/business-hours` | `merchant.ts` | 已闭环 |
| 多门店列表 | `GET /v1/merchants/my` | `api/merchant.go` | 未发现商户页直接承接 | `merchant.ts` 已有 `listMyMerchants()` | 给集团/品牌或多门店主体使用，并非所有商户都需要；待讨论 |
| 商户申请草稿与主体资料 | `GET /v1/merchant/application` `PUT /v1/merchant/application/basic` `PUT /v1/merchant/application/images` `DELETE /v1/merchant/application/documents/{document_type}` `POST /v1/merchant/application/submit` `POST /v1/merchant/application/reset` `POST /v1/ocr/jobs` | `api/merchant_application.go` `api/ocr.go` | `merchant/settings/application` | `onboarding.ts` `ocr-jobs.ts` | 已闭环 |
| OCR 与媒体 | `POST /v1/ocr/jobs` `GET /v1/ocr/jobs/{id}` `GET /v1/ocr/jobs/{id}/result` `POST /v1/ocr/jobs/{id}/retry` `POST /v1/ocr/jobs/batch-query` `GET /v1/ocr/jobs/dead-letter` `POST /v1/media/upload-sessions` `POST /v1/media/complete` `DELETE /v1/media/{id}` | `api/ocr.go` `api/media.go` | 申请、集团申请、门店图片等页面使用单任务 OCR/媒体 | `ocr-jobs.ts` `media.ts` | 主链已闭环；`batch-query` 与 `dead-letter` 未见商户页承接，待讨论是否需要 |
| 菜品分类与菜品 | `POST /v1/dishes/categories` `GET /v1/dishes/categories` `GET /v1/dishes/categories/global` `PATCH /v1/dishes/categories/{id}` `DELETE /v1/dishes/categories/{id}` `POST /v1/dishes` `GET /v1/dishes` `GET /v1/dishes/{id}` `PUT /v1/dishes/{id}` `DELETE /v1/dishes/{id}` `PATCH /v1/dishes/{id}/status` `PATCH /v1/dishes/batch/status` `PUT /v1/dishes/{id}/customizations` `GET /v1/dishes/{id}/customizations` `PUT /v1/dishes/{id}/featured-tags` | `api/dish.go` | `merchant/dishes` `merchant/dishes/edit` `merchant/dishes/categories` | `dish.ts` | 已闭环 |
| 套餐 | `POST /v1/combos` `GET /v1/combos` `GET /v1/combos/{id}` `PUT /v1/combos/{id}` `DELETE /v1/combos/{id}` `PUT /v1/combos/{id}/online` `POST /v1/combos/{id}/dishes` `DELETE /v1/combos/{id}/dishes/{dish_id}` | `api/combo.go` | `merchant/combos` `merchant/combos/edit` | `dish.ts` | 已闭环 |
| 库存 | `POST /v1/inventory` `GET /v1/inventory` `PUT /v1/inventory` `PATCH /v1/inventory/{dish_id}` `POST /v1/inventory/check` `GET /v1/inventory/stats` | `api/inventory.go` | `merchant/inventory` | `dish.ts` | 管理主链已闭环；`check` 偏订单内部校验 |
| 桌台与桌台资产 | `POST /v1/tables` `GET /v1/tables` `GET /v1/tables/{id}` `PATCH /v1/tables/{id}` `PATCH /v1/tables/{id}/status` `DELETE /v1/tables/{id}` `GET /v1/tables/{id}/qrcode` `POST /v1/tables/{id}/images` `GET /v1/tables/{id}/images` `PUT /v1/tables/{id}/images/{image_id}/primary` `DELETE /v1/tables/{id}/images/{image_id}` `POST /v1/tables/{id}/tags` `GET /v1/tables/{id}/tags` `DELETE /v1/tables/{id}/tags/{tag_id}` | `api/table.go` `api/scan.go` | `merchant/tables` `merchant/tables/edit` | `table-device-management.ts` | 已闭环；页面以 `table_type=room` 统一承接包间管理 |
| 房间查询 | `GET /v1/merchants/{id}/rooms` `GET /v1/merchants/{id}/rooms/all` `GET /v1/rooms/{id}` `GET /v1/rooms/{id}/availability` | `api/table.go` | 商户页未直接消费，桌台页统一管理包间 | `room.ts` | 更偏顾客/预订可用性读取；是否需要商户独立房间页待讨论 |
| 预订 | `GET /v1/reservations/merchant` `GET /v1/reservations/merchant/workbench` `GET /v1/reservations/merchant/today` `GET /v1/reservations/merchant/stats` `GET /v1/reservations/merchant/dishes` `POST /v1/reservations/merchant/create` `GET /v1/reservations/{id}` `PUT /v1/reservations/{id}/update` `POST /v1/reservations/{id}/confirm` `POST /v1/reservations/{id}/complete` `POST /v1/reservations/{id}/cancel` `POST /v1/reservations/{id}/no-show` `POST /v1/reservations/{id}/checkin` `POST /v1/reservations/{id}/start-cooking` `POST /v1/reservations/{id}/add-dishes` `POST /v1/reservations/{id}/modify-dishes` | `api/table_reservation.go` `api/table_reservation_workbench.go` | `merchant/reservations` `merchant/reservations/edit` | `reservation.ts` | 主链已闭环；`workbench` API 已封装但页面未见消费，待讨论是否替换列表数据源 |
| 打印机与设备同步恢复 | `POST /v1/merchant/devices` `GET /v1/merchant/devices` `GET /v1/merchant/devices/access` `GET /v1/merchant/devices/{id}` `GET /v1/merchant/devices/{id}/status` `PUT /v1/merchant/devices/{id}` `DELETE /v1/merchant/devices/{id}` `POST /v1/merchant/devices/{id}/test` `GET /v1/merchant/devices/reconciliation-jobs` `POST /v1/merchant/devices/reconciliation-jobs/{id}/retry` | `api/device.go` `api/device_reconciliation.go` | `merchant/printers` `merchant/printers/edit` | `table-device-management.ts` | 已闭环 |
| 展示配置 | `GET /v1/merchant/display-config` `PUT /v1/merchant/display-config` | `api/device.go` | `merchant/settings/display-config` | `table-device-management.ts` | 已闭环 |
| 员工管理 | `GET /v1/merchant/staff` `POST /v1/merchant/staff` `PATCH /v1/merchant/staff/{id}/role` `DELETE /v1/merchant/staff/{id}` `POST /v1/merchant/staff/invite-code` | `api/staff.go` `api/server.go` | `merchant/staff` | `merchant-staff.ts` | 页面采用邀请码流；直接新增员工未承接，待讨论是否保留为后端备用 |
| 投诉处理 | `GET /v1/merchant/complaints` `GET /v1/merchant/complaints/summary` `GET /v1/merchant/complaints/{id}` `POST /v1/merchant/complaints/{id}/response` `POST /v1/merchant/complaints/{id}/complete` | `api/complaint.go` | `merchant/complaints` `merchant/complaints/detail` | `merchant-complaints.ts` | 已闭环 |
| 索赔、申诉、追偿 | `GET /v1/merchant/claims` `GET /v1/merchant/claims/summary` `GET /v1/merchant/claims/{id}` `GET /v1/merchant/claims/{id}/decision` `GET /v1/merchant/claims/{id}/recovery` `GET /v1/merchant/claims/behavior-summary` `POST /v1/merchant/claims/{id}/recovery/pay` `POST /v1/merchant/recovery-disputes` `GET /v1/merchant/recovery-disputes` `GET /v1/merchant/recovery-disputes/summary` `GET /v1/merchant/recovery-disputes/{id}` `POST /v1/merchant/appeals` `GET /v1/merchant/appeals` `GET /v1/merchant/appeals/summary` `GET /v1/merchant/appeals/{id}` | `api/appeal.go` `api/claim_recovery.go` `api/recovery_dispute.go` `api/behavior_trace.go` | `merchant/claims` `merchant/claims/detail` `merchant/appeals` `merchant/appeals/detail` | `appeals-customer-service.ts` `payment.ts` | 索赔/申诉/追偿支付已闭环；`recovery-disputes*` 未见 Weapp API/页面承接，待讨论 |
| 评价 | `GET /v1/reviews/merchants/{id}/all` `GET /v1/reviews/{id}` `POST /v1/reviews/{id}/reply` | `api/review.go` | `merchant/reviews` | `review.ts` `merchant.ts` | 已闭环 |
| 代金券 | `POST /v1/merchants/{id}/vouchers` `GET /v1/merchants/{id}/vouchers` `GET /v1/merchants/{id}/vouchers/active` `PATCH /v1/merchants/{id}/vouchers/{voucher_id}` `DELETE /v1/merchants/{id}/vouchers/{voucher_id}` | `api/voucher.go` | `merchant/vouchers` `merchant/vouchers/edit` | `coupon.ts` | 管理主链已闭环；`active` 读口未见商户页消费 |
| 配送促销 | `GET /v1/delivery-fee/merchants/{merchant_id}/promotions` `POST /v1/delivery-fee/merchants/{merchant_id}/promotions` `PATCH /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` `DELETE /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` | `api/delivery_fee.go` | `merchant/delivery-promotions` `merchant/delivery-promotions/edit` | `delivery-fee.ts` | 已闭环 |
| 满减规则 | `POST /v1/merchants/{id}/discounts` `GET /v1/merchants/{id}/discounts` `GET /v1/merchants/{id}/discounts/{id}` `GET /v1/merchants/{id}/discounts/active` `GET /v1/merchants/{id}/discounts/applicable` `GET /v1/merchants/{id}/discounts/best` `PATCH /v1/merchants/{id}/discounts/{id}` `DELETE /v1/merchants/{id}/discounts/{id}` | `api/discount.go` | `merchant/discount-rules` `merchant/discount-rules/edit` | `merchant.ts` | 管理主链已闭环；`active/applicable/best` 未见商户页消费，待讨论 |
| 商户标签 | `GET /v1/merchants/me/tags` `PUT /v1/merchants/me/tags` `GET /v1/tags` | `api/tag.go` `api/dish.go` | `merchant/merchant-categories` | `merchant.ts` | 已闭环 |
| 会员设置 | `GET /v1/merchants/me/membership-settings` `PUT /v1/merchants/me/membership-settings` | `api/membership.go` | `merchant/settings/membership` | `merchant.ts` | 已闭环 |
| 会员列表与人工调账 | `GET /v1/merchants/{id}/members` `GET /v1/merchants/{id}/members/{user_id}` `POST /v1/merchants/{id}/members/{user_id}/balance` `POST /v1/merchants/{id}/members/{user_id}/recharge` | `api/membership.go` | `merchant/settings/members` | `merchant.ts` | 列表、详情、余额调整已闭环；线下代录充值未见 Weapp API/页面承接，待讨论 |
| 充值规则 | `POST /v1/merchants/{id}/recharge-rules` `GET /v1/merchants/{id}/recharge-rules` `GET /v1/merchants/{id}/recharge-rules/active` `PATCH /v1/merchants/{id}/recharge-rules/{rule_id}` `DELETE /v1/merchants/{id}/recharge-rules/{rule_id}` | `api/membership.go` | `merchant/settings/recharge-rules` `merchant/settings/recharge-rules/edit` | `merchant.ts` | 管理主链已闭环；`active` 读口未见商户页消费，待讨论 |
| 包装策略 | 当前未见 `/v1/merchants/me/packaging-policy` 路由；后端只发现订单创建时的包装策略校验 | `logic/packaging_policy.go` | 未发现页面 | 未发现 API 封装 | 不是当前已暴露商户配置接口；从已闭环项移除 |
| 集团申请与加入集团 | `GET /v1/groups/applications/me` `PUT /v1/groups/applications/basic` `DELETE /v1/groups/applications/documents/{document_type}` `POST /v1/groups/applications/submit` `GET /v1/groups` `POST /v1/groups/{id}/join-requests` `POST /v1/ocr/jobs` | `api/group.go` `api/ocr.go` | `merchant/group/application` `merchant/group/join` | `group-application.ts` `ocr-jobs.ts` | 已闭环 |
| 集团/品牌治理 | `GET /v1/groups/{id}` `PATCH /v1/groups/{id}` `GET /v1/groups/{id}/merchants` `GET /v1/groups/{id}/brands` `POST /v1/groups/{id}/brands` `GET/PUT /v1/groups/{id}/policies` `POST /v1/groups/{id}/menu-templates` `GET /v1/brands/{id}` `POST /v1/brands/{id}/menu-templates` | `api/group.go` | 未发现商户页承接 | 未发现 Weapp 商户 API 封装 | 更像集团/品牌负责人能力，不是所有商户默认能力；待讨论 |

## Weapp 页面映射矩阵

| 页面组 | 页面路径 | 对应 API 模块 | 对应后端接口 | 页面能力 |
| --- | --- | --- | --- | --- |
| 工作台 | `pages/merchant/dashboard/index` | `order-management.ts` `merchant-stats.ts` `merchant-complaints.ts` `reservation.ts` `merchant.ts` `merchant-app-bind.ts` | 订单摘要、经营概览、投诉汇总、预订统计、门店营业状态、商户 App 绑定码 | 工作台聚合入口与待办看板 |
| 订单列表 | `pages/merchant/orders/list/index` | `order-management.ts` | `/v1/merchant/orders` `/v1/merchant/orders/summary` `/v1/merchant/orders/{id}/accept` `/reject` `/ready` `/complete` | 列表筛选、接单、拒单、出餐、完结 |
| 订单详情 | `pages/merchant/orders/detail/index` | `order-management.ts` `payment.ts` | `/v1/merchant/orders/{id}` `/v1/merchant/orders/{id}/print-jobs*` `/v1/payments/{paymentId}/refunds` `/v1/refunds/{refundId}/returns` | 订单详情、打印任务、退款链路辅助信息 |
| 打印异常 | `pages/merchant/orders/print-anomalies/index` | `order-management.ts` | `/v1/merchant/orders/print-anomalies` `/v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry` | 打印异常查看与重试 |
| 后厨列表与详情 | `pages/merchant/kitchen/index` `pages/merchant/kitchen/detail/index` | `order-management.ts` `merchant.ts` | `/v1/kitchen/orders*` `/v1/merchants/me/status` | KDS 列表、开始制作、标记出餐 |
| 经营统计 | `pages/merchant/stats/index` `pages/merchant/stats/customers/index` `pages/merchant/stats/customers/detail/index` | `merchant-stats.ts` `order-management.ts` `reservation.ts` | `/v1/merchant/stats/**` `/v1/merchant/orders/stats` `/v1/reservations/merchant/stats` | 经营分析、客群分析、订单与预订统计 |
| 投诉 | `pages/merchant/complaints/index` `pages/merchant/complaints/detail/index` | `merchant-complaints.ts` | `/v1/merchant/complaints` `/summary` `/{id}` `/{id}/response` `/{id}/complete` | 投诉列表、详情、回复、完结 |
| 索赔与申诉 | `pages/merchant/claims/index` `pages/merchant/claims/detail/index` `pages/merchant/appeals/index` `pages/merchant/appeals/detail/index` | `appeals-customer-service.ts` `payment.ts` | `/v1/merchant/claims*` `/v1/merchant/appeals*` `/v1/merchant/claims/{id}/recovery/pay` `/v1/merchant/claims/behavior-summary` | 索赔详情、判责、行为摘要、追偿支付、申诉提交与查询 |
| 菜品 | `pages/merchant/dishes/index` `pages/merchant/dishes/edit/index` `pages/merchant/dishes/categories/index` | `dish.ts` | `/v1/dishes*` `/v1/dishes/categories*` `/v1/tags` | 菜品 CRUD、上下架、定制、分类管理 |
| 套餐 | `pages/merchant/combos/index` `pages/merchant/combos/edit/index` | `dish.ts` | `/v1/combos*` | 套餐 CRUD、上下架、关联菜品 |
| 库存 | `pages/merchant/inventory/index` | `dish.ts` | `/v1/inventory` `/v1/inventory/stats` | 查询和更新库存 |
| 桌台 | `pages/merchant/tables/index` `pages/merchant/tables/edit/index` | `table-device-management.ts` `dish.ts` | `/v1/tables*` `/v1/tables/{id}/qrcode` `/v1/tables/{id}/images*` `/v1/tables/{id}/tags*` | 桌台/包间、二维码、图片、标签 |
| 预订 | `pages/merchant/reservations/index` `pages/merchant/reservations/edit/index` | `reservation.ts` `table-device-management.ts` | `/v1/reservations/merchant*` `/v1/reservations/{id}/*` `/v1/tables` | 预订列表、确认、改菜、爽约、代客创建 |
| 打印机 | `pages/merchant/printers/index` `pages/merchant/printers/edit/index` | `table-device-management.ts` | `/v1/merchant/devices*` `/v1/merchant/devices/access` `/v1/merchant/devices/reconciliation-jobs*` | 打印机 CRUD、测试、状态、同步恢复 |
| 展示配置 | `pages/merchant/settings/display-config/index` | `table-device-management.ts` | `/v1/merchant/display-config` | 打印与语音协同配置 |
| 门店资料 | `pages/merchant/settings/profile/index` `pages/merchant/profile-images/index` | `merchant.ts` `onboarding.ts` | `/v1/merchants/me` `/v1/merchants/me/shop-images` `/v1/merchant/application` `/v1/media/{id}` | 基础资料、Logo 与门头图资产 |
| 营业时间 | `pages/merchant/settings/business-hours/index` | `merchant.ts` | `/v1/merchants/me/business-hours` | 营业时间维护 |
| 收付通开户 | `pages/merchant/settings/applyment/index` `pages/merchant/settings/applyment/action/index` `pages/merchant/settings/applyment/submit/index` | `merchant-applyment.ts` `applyment-bank.ts` | `/v1/merchant/applyment/status` `/v1/merchant/applyment/bindbank` `/v1/merchant/applyment/banks*` `/v1/merchant/applyment/areas/provinces*` | 开户状态、绑卡、银行目录查询 |
| 结算账户 | `pages/merchant/settings/applyment/settlement-account/index` | `merchant-settlement-account.ts` | `/v1/merchant/finance/account/settlement-account` `/applications/{application_no}` | 结算账户查询、修改申请、申请状态查询 |
| 商户主体申请 | `pages/merchant/settings/application/index` | `onboarding.ts` `ocr-jobs.ts` | `/v1/merchant/application*` `/v1/ocr/jobs` | 商户主体资料、OCR、提交、重置 |
| 员工 | `pages/merchant/staff/index` | `merchant-staff.ts` `auth.ts` | `/v1/merchant/staff` `/v1/merchant/staff/invite-code` `/v1/users/me` | 员工列表、角色修改、移除、邀请码 |
| 评价 | `pages/merchant/reviews/index` | `review.ts` `merchant.ts` | `/v1/reviews/merchants/{id}/all` `/v1/reviews/{id}` `/v1/reviews/{id}/reply` | 查看和回复评价 |
| 代金券 | `pages/merchant/vouchers/index` `pages/merchant/vouchers/edit/index` | `coupon.ts` `merchant.ts` | `/v1/merchants/{id}/vouchers*` | 代金券管理 |
| 配送促销 | `pages/merchant/delivery-promotions/index` `pages/merchant/delivery-promotions/edit/index` | `delivery-fee.ts` `merchant.ts` | `/v1/delivery-fee/merchants/{id}/promotions*` | 配送促销规则 |
| 满减规则 | `pages/merchant/discount-rules/index` `pages/merchant/discount-rules/edit/index` | `merchant.ts` | `/v1/merchants/{id}/discounts*` | 满减规则列表、创建、查看、启停、删除 |
| 商户标签 | `pages/merchant/merchant-categories/index` | `merchant.ts` | `/v1/merchants/me/tags` `/v1/tags` | 商户标签选择与保存 |
| 会员设置 | `pages/merchant/settings/membership/index` | `merchant.ts` | `/v1/merchants/me/membership-settings` | 会员可用场景与叠加规则 |
| 会员列表与调账 | `pages/merchant/settings/members/index` | `merchant.ts` | `/v1/merchants/{id}/members*` `/balance` | 会员列表、交易详情、人工调账 |
| 充值规则 | `pages/merchant/settings/recharge-rules/index` `pages/merchant/settings/recharge-rules/edit/index` | `merchant.ts` | `/v1/merchants/{id}/recharge-rules*` | 充值送规则管理 |
| 集团申请与入群 | `pages/merchant/group/application/index` `pages/merchant/group/join/index` | `group-application.ts` `ocr-jobs.ts` | `/v1/groups/applications*` `/v1/groups` `/v1/groups/{id}/join-requests` `/v1/ocr/jobs` | 集团申请、OCR、搜索集团、加入集团 |
| 配置中转页 | `pages/merchant/config/index` | 无直接后端 API | 无 | 商户控制台导航与路由分发 |

## 有接口但未见页面承接的能力清单

这些能力先列入待讨论池，不直接等同于必须新增页面。

| 能力 | 路由 | Weapp 现状 | 讨论点 |
| --- | --- | --- | --- |
| 多门店列表 | `/v1/merchants/my` | `merchant.ts::listMyMerchants()` 已封装，未发现页面消费 | 给集团/品牌或多门店主体使用，并非所有商户默认能力 |
| 预订工作台数据源 | `/v1/reservations/merchant/workbench` | `reservation.ts` 已封装，未发现页面消费 | 是否替换预订页当前列表/聚合数据源 |
| 直接新增员工 | `POST /v1/merchant/staff` | 页面采用邀请码，未发现 API/页面直接新增员工 | 是否保留为后端备用，还是未来给 owner 加直接新增入口 |
| 商户追偿争议 | `POST /v1/merchant/recovery-disputes` `GET /v1/merchant/recovery-disputes` `GET /v1/merchant/recovery-disputes/summary` `GET /v1/merchant/recovery-disputes/{id}` | 未发现 Weapp API/页面承接 | 是否需要商户发起/查看追偿争议；若需要，应放在索赔详情任务域而非新入口墙 |
| 线下代录会员充值 | `POST /v1/merchants/{id}/members/{user_id}/recharge` | 会员页已承接余额调整，未承接代录充值 | 是否仍需要线下收款后入账这个商户动作 |
| 充值规则 active 读口 | `GET /v1/merchants/{id}/recharge-rules/active` | 未发现 Weapp 商户 API/页面消费 | 管理页是否需要，还是只给顾客充值/展示链路使用 |
| 代金券 active 读口 | `GET /v1/merchants/{id}/vouchers/active` | `coupon.ts` 有封装，未发现商户页消费 | 管理页不一定需要；可留给顾客领券/下单链路 |
| 满减 active/applicable/best 读口 | `GET /v1/merchants/{id}/discounts/active` `GET /v1/merchants/{id}/discounts/applicable` `GET /v1/merchants/{id}/discounts/best` | 商户管理页使用列表和详情，未消费这三类读口 | 更偏下单/可用规则计算；是否进入商户页待讨论 |
| 房间专用查询 | `/v1/merchants/{id}/rooms` `/rooms/all` `/v1/rooms/{id}` `/availability` | `room.ts` 有封装，商户页通过桌台模型管理包间 | 是否需要独立房间页；当前不是“包间完全未承接” |
| OCR 批量/死信 | `POST /v1/ocr/jobs/batch-query` `GET /v1/ocr/jobs/dead-letter` | 主流程使用单任务 OCR，未见页面消费批量/死信 | 更偏诊断/运维，是否对商户暴露待讨论 |
| 集团/品牌治理 | `/v1/groups/{id}` `/merchants` `/brands` `/policies` `/menu-templates` `/v1/brands/{id}` `/menu-templates` | 商户页只承接集团申请和加入集团 | 属于集团/品牌负责人能力，不是所有商户默认能力 |
| Flutter 商户 App 设备运行时 | `/v1/merchant/device/register` `/heartbeat` `DELETE /{device_id}` | Weapp 只承接绑定码，运行时接口由 Flutter App 使用 | 不是 Weapp 页面缺口 |

## 已通过其他页面或任务模型承接，但不是独立入口

| 能力 | 路由 | 说明 |
| --- | --- | --- |
| 包间管理 | `/v1/tables*` + `table_type=room` | 商户侧通过桌台页统一管理包间，不按 `/v1/rooms*` 另建控制台入口。 |
| 结算账户 | `/v1/merchant/finance/account/settlement-account*` | 已放在收付通开户页组下，而不是财务页下。 |
| 直接创建评价回复 | `/v1/reviews/{id}/reply` | 已由评价页承接，不需要单独页面。 |

## 当前不再成立的旧结论

| 旧结论 | 当前事实 | 处理 |
| --- | --- | --- |
| 结算账户资料未承接 | `settings/applyment/settlement-account/index` 已注册并消费查询、修改、申请查询接口 | 改为已闭环 |
| 财务中心未发现页面注册 | 当前已注册 `pages/merchant/finance/**` 页面组，并消费 `merchant-finance.ts` 与 `merchant-finance-workflow.ts` | 改为已闭环 |
| 包装策略已闭环 | 当前未发现商户配置路由和页面，只有订单逻辑校验 | 从已闭环能力移除 |
| 投诉 Swagger 注解不完整 | `api/complaint.go` 当前已有列表、摘要、详情、回复、完结 `@Router` | 不再作为漂移 |
| 员工邀请码注解路径漂移 | `api/staff.go` 当前注解为 `/v1/merchant/staff/invite-code` | 不再作为漂移 |
| 满减规则 Swagger 不完整 | `api/discount.go` 当前已有创建、列表、详情、active、applicable、best、更新、删除注解 | 不再作为漂移 |

## 对齐建议

1. 第一轮讨论建议从财务域开始：财务中心、提现、提现详情、取消提现属于同一任务域，不宜拆成多个孤立入口。
2. 取消提现和提现都不按超长任务处理；参考骑手押金提现，提交后保持 loading 或同步态，等待查询接口拿到后端结果真值，再更新为成功、失败或可刷新状态。
3. 多门店列表先按集团/品牌或多门店主体能力讨论，不作为所有商户默认入口。
4. 集团/品牌治理、房间专用查询、active/可用规则读口、OCR 诊断接口先保留在待讨论池，逐个确认是否需要商户页承接。
5. 后续新增商户页时，不应只搜索 `/v1/merchant/**`，必须同时覆盖 `/v1/merchants/me/**`、`/v1/merchants/{id}/**`、`/v1/kitchen/**`、`/v1/reservations/**`、`/v1/dishes/**`、`/v1/combos/**`、`/v1/tables/**`、`/v1/reviews/**`、`/v1/groups/**` 与 `/v1/ocr/jobs`。