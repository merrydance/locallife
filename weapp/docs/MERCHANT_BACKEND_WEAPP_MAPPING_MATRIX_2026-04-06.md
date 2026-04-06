# 商户侧后端接口与 Weapp 映射矩阵

日期：2026-04-06

## 适用范围

- 后端商户侧合同：`locallife/api/**` 与 `locallife/api/server.go`
- 小程序商户页：`weapp/miniprogram/pages/merchant/**`
- 小程序商户 API 层：`weapp/miniprogram/api/**`

## 目的

本文件回答三件事：

1. 后端商户侧当前实际暴露了哪些接口。
2. 这些接口分别承载了哪些商户能力。
3. Weapp 小程序商户侧哪些页面已经承接，哪些还没有承接，哪些存在合同漂移。

历史版本可参考：`weapp/docs/historical/pre-2026-04-05/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md`。
本文件以 2026-04-06 当前代码事实为准。

## 单一事实来源

优先级从高到低如下：

1. `locallife/api/server.go` 中的真实路由注册。
2. `locallife/api/*.go` 中 handler 的请求、响应和权限语义。
3. `weapp/miniprogram/api/*.ts` 中前端 API 封装的真实 URL。
4. `weapp/miniprogram/pages/merchant/**` 中页面对 API 的实际消费。

## 审计结论摘要

- Weapp 商户侧当前共有 45 个页面入口，已覆盖订单、后厨、菜品、套餐、库存、桌台、预订、打印机、财务、投诉、申诉、索赔、评价、员工、门店资料、会员、营销、申请与集团申请等主能力域。
- 后端商户合同并不只在 `/v1/merchant/**` 下，真实商户能力还分散在 `/v1/merchants/me/**`、`/v1/merchants/{id}/**`、`/v1/kitchen/**`、`/v1/dishes/**`、`/v1/combos/**`、`/v1/tables/**`、`/v1/reservations/**`、`/v1/reviews/**`、`/v1/groups/**` 和 `/v1/ocr/jobs`。
- 当前主链路大多已形成“后端接口 -> weapp API 封装 -> 商户页面”的闭环，但仍有一批后台已暴露、尚未被小程序商户侧完整承接的接口，主要集中在结算账户、多门店切换、充值规则 active 读口、满减规则附加读口，以及房间能力缺少独立商户控制台入口等。
- 存在少量合同漂移：部分能力真实可用，但 Swagger 或 handler 注解未完整表达，例如投诉详情/回复/完结路由，以及员工邀请码注解路径与真实注册路径不一致。

## 后端能力域矩阵

| 能力域 | 后端接口 | 主要后端文件 | Weapp 承接页面 | Weapp API 模块 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| 工作台聚合 | `GET /v1/merchant/orders/summary` `GET /v1/merchant/orders/stats` `GET /v1/merchant/stats/overview` `GET /v1/merchant/stats/hourly` `GET /v1/merchant/stats/sources` `GET /v1/merchant/stats/repurchase` `GET /v1/merchant/complaints/summary` `GET /v1/reservations/merchant/stats` `GET /v1/merchants/me` `GET/PATCH /v1/merchants/me/status` | `api/order.go` `api/merchant_stats.go` `api/complaint.go` `api/table_reservation.go` `api/merchant.go` | `merchant/dashboard` | `order-management.ts` `merchant-stats.ts` `merchant-complaints.ts` `reservation.ts` `merchant.ts` | 已闭环 |
| 商户订单 | `GET /v1/merchant/orders` `GET /v1/merchant/orders/{id}` `POST /v1/merchant/orders/{id}/accept` `POST /v1/merchant/orders/{id}/reject` `POST /v1/merchant/orders/{id}/ready` `POST /v1/merchant/orders/{id}/complete` | `api/order.go` | `merchant/orders/list` `merchant/orders/detail` | `order-management.ts` | 已闭环 |
| 打印任务与异常 | `GET /v1/merchant/orders/{id}/print-jobs` `POST /v1/merchant/orders/{id}/print-jobs` `POST /v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry` `GET /v1/merchant/orders/{id}/print-jobs/{print_log_id}/status` `GET /v1/merchant/orders/print-anomalies` | `api/order.go` | `merchant/orders/detail` `merchant/orders/print-anomalies` | `order-management.ts` | 已闭环 |
| 后厨 KDS | `GET /v1/kitchen/orders` `GET /v1/kitchen/orders/{id}` `POST /v1/kitchen/orders/{id}/preparing` `POST /v1/kitchen/orders/{id}/ready` | `api/kitchen.go` | `merchant/kitchen` `merchant/kitchen/detail` | `order-management.ts` | 已闭环 |
| 经营统计 | `GET /v1/merchant/stats/daily` `GET /v1/merchant/stats/overview` `GET /v1/merchant/stats/dishes/top` `GET /v1/merchant/stats/customers` `GET /v1/merchant/stats/customers/{user_id}` `GET /v1/merchant/stats/hourly` `GET /v1/merchant/stats/sources` `GET /v1/merchant/stats/repurchase` `GET /v1/merchant/stats/categories` | `api/merchant_stats.go` | `merchant/stats` `merchant/stats/customers` `merchant/stats/customers/detail` | `merchant-stats.ts` | 已闭环 |
| 财务总览与提现 | `GET /v1/merchant/finance/overview` `GET /v1/merchant/finance/orders` `GET /v1/merchant/finance/service-fees` `GET /v1/merchant/finance/promotions` `GET /v1/merchant/finance/daily` `GET /v1/merchant/finance/settlements` `GET /v1/merchant/finance/settlement-timeline` `GET /v1/merchant/finance/account/balance` `GET /v1/merchant/finance/account/withdrawals` `POST /v1/merchant/finance/account/withdraw` | `api/merchant_finance.go` | `merchant/finance` | `merchant-finance.ts` | 已闭环 |
| 收付通进件与绑卡 | `GET /v1/merchant/applyment/status` `POST /v1/merchant/applyment/bindbank` `GET /v1/merchant/applyment/banks` `GET /v1/merchant/applyment/banks/search-by-bank-account` `GET /v1/merchant/applyment/areas/provinces` `GET /v1/merchant/applyment/areas/provinces/{province_code}/cities` `GET /v1/merchant/applyment/banks/{bank_alias_code}/branches` | `api/ecommerce_applyment.go` `api/applyment_bank_catalog.go` | `merchant/settings/applyment` `merchant/settings/applyment/completed` | `merchant-finance.ts` `applyment-bank.ts` | 已闭环 |
| 结算账户资料 | `GET /v1/merchant/finance/account/settlement-account` `POST /v1/merchant/finance/account/settlement-account` `GET /v1/merchant/finance/account/settlement-account/applications/{application_no}` | `api/settlement_account.go` | 未发现商户页承接 | 未发现商户 API 封装 | 后端已暴露，Weapp 未承接 |
| 门店基本资料 | `GET /v1/merchants/me` `PATCH /v1/merchants/me` `PATCH /v1/merchants/me/shop-images` | `api/merchant.go` | `merchant/settings/profile` `merchant/profile-images` | `merchant.ts` `onboarding.ts` | 已闭环 |
| 营业状态与营业时间 | `GET/PATCH /v1/merchants/me/status` `GET/PUT /v1/merchants/me/business-hours` | `api/merchant.go` | `merchant/dashboard` `merchant/settings/profile` `merchant/settings/business-hours` | `merchant.ts` | 已闭环 |
| 商户申请草稿与主体资料 | `GET /v1/merchant/application` `PUT /v1/merchant/application/basic` `PUT /v1/merchant/application/images` `DELETE /v1/merchant/application/documents/{document_type}` `POST /v1/merchant/application/submit` `POST /v1/merchant/application/reset` `POST /v1/ocr/jobs` | `api/merchant_application.go` `api/ocr_jobs.go` | `merchant/settings/application` | `onboarding.ts` `ocr-jobs.ts` | 已闭环 |
| 菜品分类与菜品 | `POST /v1/dishes/categories` `GET /v1/dishes/categories` `GET /v1/dishes/categories/global` `PATCH /v1/dishes/categories/{id}` `DELETE /v1/dishes/categories/{id}` `POST /v1/dishes` `GET /v1/dishes` `GET /v1/dishes/{id}` `PUT /v1/dishes/{id}` `DELETE /v1/dishes/{id}` `PATCH /v1/dishes/{id}/status` `PATCH /v1/dishes/batch/status` `PUT /v1/dishes/{id}/customizations` `GET /v1/dishes/{id}/customizations` `PUT /v1/dishes/{id}/featured-tags` | `api/dish.go` | `merchant/dishes` `merchant/dishes/edit` `merchant/dishes/categories` | `dish.ts` | 已闭环 |
| 套餐 | `POST /v1/combos` `GET /v1/combos` `GET /v1/combos/{id}` `PUT /v1/combos/{id}` `DELETE /v1/combos/{id}` `PUT /v1/combos/{id}/online` `POST /v1/combos/{id}/dishes` `DELETE /v1/combos/{id}/dishes/{dish_id}` | `api/combo.go` | `merchant/combos` `merchant/combos/edit` | `dish.ts` | 已闭环 |
| 库存 | `GET /v1/inventory` `PUT /v1/inventory` `GET /v1/inventory/stats` | `api/inventory.go` | `merchant/inventory` | `dish.ts` | 已闭环；`POST /v1/inventory` 与 `POST /v1/inventory/check` 不作为独立商户页入口 |
| 桌台与桌台资产 | `POST /v1/tables` `GET /v1/tables` `GET /v1/tables/{id}` `PATCH /v1/tables/{id}` `PATCH /v1/tables/{id}/status` `DELETE /v1/tables/{id}` `GET /v1/tables/{id}/qrcode` `POST /v1/tables/{id}/images` `GET /v1/tables/{id}/images` `PUT /v1/tables/{id}/images/{image_id}/primary` `DELETE /v1/tables/{id}/images/{image_id}` `POST /v1/tables/{id}/tags` `GET /v1/tables/{id}/tags` `DELETE /v1/tables/{id}/tags/{tag_id}` | `api/table.go` `api/scan.go` | `merchant/tables` | `table-device-management.ts` | 已闭环 |
| 预订 | `GET /v1/reservations/merchant` `GET /v1/reservations/merchant/today` `GET /v1/reservations/merchant/stats` `GET /v1/reservations/merchant/dishes` `POST /v1/reservations/merchant/create` `GET /v1/reservations/{id}` `PUT /v1/reservations/{id}/update` `POST /v1/reservations/{id}/confirm` `POST /v1/reservations/{id}/complete` `POST /v1/reservations/{id}/cancel` `POST /v1/reservations/{id}/no-show` `POST /v1/reservations/{id}/checkin` `POST /v1/reservations/{id}/start-cooking` `POST /v1/reservations/{id}/add-dishes` `POST /v1/reservations/{id}/modify-dishes` | `api/table_reservation.go` | `merchant/reservations` | `reservation.ts` | 已闭环 |
| 打印机与设备同步恢复 | `POST /v1/merchant/devices` `GET /v1/merchant/devices` `GET /v1/merchant/devices/{id}` `GET /v1/merchant/devices/{id}/status` `PUT /v1/merchant/devices/{id}` `DELETE /v1/merchant/devices/{id}` `POST /v1/merchant/devices/{id}/test` `GET /v1/merchant/devices/reconciliation-jobs` `POST /v1/merchant/devices/reconciliation-jobs/{id}/retry` | `api/device.go` `api/device_reconciliation.go` | `merchant/printers` | `table-device-management.ts` | 已闭环 |
| 展示配置 | `GET /v1/merchant/display-config` `PUT /v1/merchant/display-config` | `api/device.go` | `merchant/settings/display-config` | `table-device-management.ts` | 已闭环 |
| 员工管理 | `GET /v1/merchant/staff` `PATCH /v1/merchant/staff/{id}/role` `DELETE /v1/merchant/staff/{id}` `POST /v1/merchant/staff/invite-code` | `api/staff.go` `api/server.go` | `merchant/staff` | `merchant-staff.ts` | 已闭环；页面当前采用邀请码流，未直接消费 `POST /v1/merchant/staff` |
| 投诉处理 | `GET /v1/merchant/complaints` `GET /v1/merchant/complaints/summary` `GET /v1/merchant/complaints/{id}` `POST /v1/merchant/complaints/{id}/response` `POST /v1/merchant/complaints/{id}/complete` | `api/complaint.go` | `merchant/complaints` `merchant/complaints/detail` | `merchant-complaints.ts` | 已闭环 |
| 索赔、申诉与追偿 | `GET /v1/merchant/claims` `GET /v1/merchant/claims/summary` `GET /v1/merchant/claims/{id}` `GET /v1/merchant/claims/{id}/decision` `GET /v1/merchant/claims/{id}/recovery` `POST /v1/merchant/claims/{id}/recovery/pay` `GET /v1/merchant/risk/users/{id}` `POST /v1/merchant/appeals` `GET /v1/merchant/appeals` `GET /v1/merchant/appeals/summary` `GET /v1/merchant/appeals/{id}` | `api/appeal.go` `api/claim_recovery.go` `api/behavior_trace.go` | `merchant/claims` `merchant/claims/detail` `merchant/appeals` `merchant/appeals/detail` | `appeals-customer-service.ts` `payment.ts` | 已闭环 |
| 评价 | `GET /v1/reviews/merchants/{id}/all` `GET /v1/reviews/{id}` `POST /v1/reviews/{id}/reply` | `api/review.go` | `merchant/reviews` | `review.ts` `merchant.ts` | 已闭环 |
| 代金券 | `POST /v1/merchants/{id}/vouchers` `GET /v1/merchants/{id}/vouchers` `PATCH /v1/merchants/{id}/vouchers/{voucher_id}` `DELETE /v1/merchants/{id}/vouchers/{voucher_id}` | `api/voucher.go` | `merchant/vouchers` | `coupon.ts` | 已闭环 |
| 配送促销 | `GET /v1/delivery-fee/merchants/{merchant_id}/promotions` `POST /v1/delivery-fee/merchants/{merchant_id}/promotions` `PATCH /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` `DELETE /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` | `api/delivery_fee.go` | `merchant/delivery-promotions` | `delivery-fee.ts` | 已闭环 |
| 满减规则 | `POST /v1/merchants/{id}/discounts` `GET /v1/merchants/{id}/discounts` `PATCH /v1/merchants/{id}/discounts/{id}` `DELETE /v1/merchants/{id}/discounts/{id}` | `api/discount.go` `api/server.go` | `merchant/discount-rules` | `merchant.ts` | 已闭环 |
| 商户标签 | `GET /v1/merchants/me/tags` `PUT /v1/merchants/me/tags` `GET /v1/tags` | `api/tag.go` `api/dish.go` | `merchant/merchant-categories` | `merchant.ts` | 已闭环 |
| 会员设置 | `GET /v1/merchants/me/membership-settings` `PUT /v1/merchants/me/membership-settings` | `api/membership.go` | `merchant/settings/membership` | `merchant.ts` | 已闭环 |
| 会员列表与人工调账 | `GET /v1/merchants/{id}/members` `GET /v1/merchants/{id}/members/{user_id}` `POST /v1/merchants/{id}/members/{user_id}/balance` | `api/membership.go` | `merchant/settings/members` | `merchant.ts` | 已闭环 |
| 充值规则 | `POST /v1/merchants/{id}/recharge-rules` `GET /v1/merchants/{id}/recharge-rules` `PATCH /v1/merchants/{id}/recharge-rules/{rule_id}` `DELETE /v1/merchants/{id}/recharge-rules/{rule_id}` | `api/membership.go` | `merchant/settings/recharge-rules` | `merchant.ts` | 已闭环 |
| 包装策略 | `GET /v1/merchants/me/packaging-policy` `PUT /v1/merchants/me/packaging-policy` | `api/packaging_policy.go` | `merchant/settings/packaging-policy` | `merchant.ts` | 已闭环 |
| 集团申请与加入集团 | `GET /v1/groups/applications/me` `PUT /v1/groups/applications/basic` `DELETE /v1/groups/applications/documents/{document_type}` `POST /v1/groups/applications/submit` `GET /v1/groups` `POST /v1/groups/{id}/join-requests` `POST /v1/ocr/jobs` | `api/group.go` `api/ocr_jobs.go` | `merchant/group/application` `merchant/group/join` | `group-application.ts` `ocr-jobs.ts` | 已闭环 |
| 房间查询 | `GET /v1/merchants/{id}/rooms` `GET /v1/merchants/{id}/rooms/all` `GET /v1/rooms/{id}` `GET /v1/rooms/{id}/availability` | `api/table.go` | `merchant/reservations` 间接消费包间详情；未发现独立房间页 | `room.ts` `reservation.ts` | API 已有，商户控制台无独立房间入口 |
| 商户身份切换 | `GET /v1/merchants/my` | `api/merchant.go` | 未发现商户页直接承接 | `merchant.ts` 未见对应消费 | 后端已暴露，Weapp 未承接 |

## Weapp 页面映射矩阵

| 页面组 | 页面路径 | 对应 API 模块 | 对应后端接口 | 页面能力 |
| --- | --- | --- | --- | --- |
| 工作台 | `pages/merchant/dashboard/index` | `order-management.ts` `merchant-stats.ts` `merchant-complaints.ts` `reservation.ts` `merchant.ts` | 订单摘要、经营概览、时段分析、来源分析、复购率、投诉汇总、预订统计、门店营业状态 | 工作台聚合入口与待办看板 |
| 订单列表 | `pages/merchant/orders/list/index` | `order-management.ts` | `/v1/merchant/orders` `/v1/merchant/orders/summary` `/v1/merchant/orders/{id}/accept` `/v1/merchant/orders/{id}/reject` `/v1/merchant/orders/{id}/ready` `/v1/merchant/orders/{id}/complete` | 列表筛选、接单、拒单、出餐、完结 |
| 订单详情 | `pages/merchant/orders/detail/index` | `order-management.ts` `payment.ts` | `/v1/merchant/orders/{id}` `/v1/merchant/orders/{id}/print-jobs*` `/v1/payments/{paymentId}/refunds` `/v1/refunds/{refundId}/returns` | 订单详情、打印任务、退款链路辅助信息 |
| 打印异常 | `pages/merchant/orders/print-anomalies/index` | `order-management.ts` | `/v1/merchant/orders/print-anomalies` `/v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry` | 打印异常查看与重试 |
| 后厨列表与详情 | `pages/merchant/kitchen/index` `pages/merchant/kitchen/detail/index` | `order-management.ts` `merchant.ts` | `/v1/kitchen/orders*` `/v1/merchants/me/status` | KDS 列表、开始制作、标记出餐 |
| 财务中心 | `pages/merchant/finance/index` | `merchant-finance.ts` | `/v1/merchant/finance/**` `/v1/merchant/finance/account/balance` `/v1/merchant/finance/account/withdrawals` `/v1/merchant/finance/account/withdraw` | 财务概览、流水、结算、提现 |
| 经营统计 | `pages/merchant/stats/index` `pages/merchant/stats/customers/index` `pages/merchant/stats/customers/detail/index` | `merchant-stats.ts` `order-management.ts` `reservation.ts` | `/v1/merchant/stats/**` `/v1/merchant/orders/stats` `/v1/reservations/merchant/stats` | 经营分析、客群分析、订单与预订统计 |
| 投诉 | `pages/merchant/complaints/index` `pages/merchant/complaints/detail/index` | `merchant-complaints.ts` | `/v1/merchant/complaints` `/v1/merchant/complaints/summary` `/v1/merchant/complaints/{id}` `/response` `/complete` | 投诉列表、详情、回复、完结 |
| 索赔与申诉 | `pages/merchant/claims/index` `pages/merchant/claims/detail/index` `pages/merchant/appeals/index` `pages/merchant/appeals/detail/index` | `appeals-customer-service.ts` `payment.ts` | `/v1/merchant/claims*` `/v1/merchant/appeals*` `/v1/merchant/risk/users/{id}` `/v1/merchant/claims/{id}/recovery/pay` | 索赔详情、判责、追偿支付、申诉提交与查询 |
| 菜品 | `pages/merchant/dishes/index` `pages/merchant/dishes/edit/index` `pages/merchant/dishes/categories/index` | `dish.ts` | `/v1/dishes*` `/v1/dishes/categories*` `/v1/tags` | 菜品 CRUD、上下架、定制、分类管理 |
| 套餐 | `pages/merchant/combos/index` `pages/merchant/combos/edit/index` | `dish.ts` | `/v1/combos*` | 套餐 CRUD、上下架、关联菜品 |
| 库存 | `pages/merchant/inventory/index` | `dish.ts` | `/v1/inventory` `/v1/inventory/stats` | 查询和更新库存 |
| 桌台 | `pages/merchant/tables/index` | `table-device-management.ts` `dish.ts` | `/v1/tables*` `/v1/tables/{id}/qrcode` `/v1/tables/{id}/images*` `/v1/tables/{id}/tags*` | 桌台、二维码、图片、标签 |
| 预订 | `pages/merchant/reservations/index` | `reservation.ts` `table-device-management.ts` | `/v1/reservations/merchant*` `/v1/reservations/{id}/*` `/v1/tables` | 预订列表、确认、改菜、爽约、代客创建 |
| 打印机 | `pages/merchant/printers/index` | `table-device-management.ts` | `/v1/merchant/devices*` `/v1/merchant/devices/reconciliation-jobs*` | 打印机 CRUD、测试、状态、同步恢复 |
| 展示配置 | `pages/merchant/settings/display-config/index` | `table-device-management.ts` | `/v1/merchant/display-config` | 打印、语音、KDS 展示配置 |
| 门店资料 | `pages/merchant/settings/profile/index` `pages/merchant/profile-images/index` | `merchant.ts` `onboarding.ts` | `/v1/merchants/me` `/v1/merchants/me/shop-images` `/v1/merchant/application` `/v1/media/{id}` | 基础资料、Logo 与门头图资产 |
| 营业时间 | `pages/merchant/settings/business-hours/index` | `merchant.ts` | `/v1/merchants/me/business-hours` | 营业时间维护 |
| 收付通开户 | `pages/merchant/settings/applyment/index` `pages/merchant/settings/applyment/completed/index` | `merchant-finance.ts` `applyment-bank.ts` | `/v1/merchant/applyment/status` `/v1/merchant/applyment/bindbank` `/v1/merchant/applyment/banks*` `/v1/merchant/applyment/areas/provinces*` | 开户状态、绑卡、银行目录查询 |
| 商户主体申请 | `pages/merchant/settings/application/index` | `onboarding.ts` `ocr-jobs.ts` | `/v1/merchant/application*` `/v1/ocr/jobs` | 商户主体资料、OCR、提交、重置 |
| 员工 | `pages/merchant/staff/index` | `merchant-staff.ts` `auth.ts` | `/v1/merchant/staff*` `/v1/merchant/staff/invite-code` `/v1/users/me` | 员工列表、角色修改、移除、邀请码 |
| 评价 | `pages/merchant/reviews/index` | `review.ts` `merchant.ts` | `/v1/reviews/merchants/{id}/all` `/v1/reviews/{id}` `/v1/reviews/{id}/reply` | 查看和回复评价 |
| 代金券 | `pages/merchant/vouchers/index` | `coupon.ts` `merchant.ts` | `/v1/merchants/{id}/vouchers*` | 代金券管理 |
| 配送促销 | `pages/merchant/delivery-promotions/index` | `delivery-fee.ts` `merchant.ts` | `/v1/delivery-fee/merchants/{id}/promotions*` | 配送促销规则 |
| 满减规则 | `pages/merchant/discount-rules/index` | `merchant.ts` | `/v1/merchants/{id}/discounts*` | 满减规则列表、创建、启停、删除 |
| 商户标签 | `pages/merchant/merchant-categories/index` | `merchant.ts` | `/v1/merchants/me/tags` `/v1/tags` | 商户标签选择与保存 |
| 会员设置 | `pages/merchant/settings/membership/index` | `merchant.ts` | `/v1/merchants/me/membership-settings` | 会员可用场景与叠加规则 |
| 会员列表与调账 | `pages/merchant/settings/members/index` | `merchant.ts` | `/v1/merchants/{id}/members*` | 会员列表、交易详情、人工调账 |
| 充值规则 | `pages/merchant/settings/recharge-rules/index` | `merchant.ts` | `/v1/merchants/{id}/recharge-rules*` | 充值送规则管理 |
| 包装策略 | `pages/merchant/settings/packaging-policy/index` | `merchant.ts` `dish.ts` | `/v1/merchants/me/packaging-policy` `/v1/dishes` | 包装菜品和适用订单类型管理 |
| 集团申请与入群 | `pages/merchant/group/application/index` `pages/merchant/group/join/index` | `group-application.ts` `ocr-jobs.ts` | `/v1/groups/applications*` `/v1/groups` `/v1/groups/{id}/join-requests` `/v1/ocr/jobs` | 集团申请、OCR、搜索集团、加入集团 |
| 配置中转页 | `pages/merchant/config/index` | 无直接后端 API | 无 | 商户控制台导航与路由分发 |

## 重点缺口清单

### 后端已暴露，但 Weapp 商户页未见直接承接

| 能力 | 路由 | 说明 |
| --- | --- | --- |
| 结算账户资料 | `/v1/merchant/finance/account/settlement-account` `POST /v1/merchant/finance/account/settlement-account` `GET /v1/merchant/finance/account/settlement-account/applications/{application_no}` | 后端已实现，当前小程序商户页未发现对应资金账户资料页 |
| 商户多门店列表 | `GET /v1/merchants/my` | 后端已支持“我的商户列表”，当前小程序商户页仍主要依赖 `current_merchant` 缓存和 `GET /v1/merchants/me` |
| 房间查询 | `GET /v1/merchants/{id}/rooms` `GET /v1/merchants/{id}/rooms/all` `GET /v1/rooms/{id}` `GET /v1/rooms/{id}/availability` | `room.ts` 与 `reservation.ts` 已有封装，当前商户控制台仅在预订链路间接读取包间详情，未提供独立房间管理入口 |
| 直接新增员工 | `POST /v1/merchant/staff` | 后端保留了直接创建员工接口，但当前商户页主流程是生成邀请码，由员工自行绑定，不直接调用该写口 |
| 充值规则 active 读口 | `GET /v1/merchants/{id}/recharge-rules/active` | 现有页面使用列表接口，并未单独消费 active 读口 |
| 满减规则附加读口 | `GET /v1/merchants/{id}/discounts/{id}` `GET /v1/merchants/{id}/discounts/active` `GET /v1/merchants/{id}/discounts/applicable` `GET /v1/merchants/{id}/discounts/best` | 现有小程序商户页仅承接列表、创建、更新、删除 |
| 提现详情 | `GET /v1/merchant/finance/account/withdrawals/{id}` | 财务页使用列表与提现申请，未见单笔详情页 |

### 页面存在，但本身不是后端能力页

| 页面 | 说明 |
| --- | --- |
| `pages/merchant/config/index` | 主要负责能力入口导航和控制台跳转，不直接绑定后端接口 |
| `pages/merchant/settings/applyment/completed/index` | 本质上是开户结果页，只读依赖 `/v1/merchant/applyment/status` |

### 真实可用，但合同表达存在漂移

| 主题 | 实际情况 | 影响 |
| --- | --- | --- |
| 投诉路由注解不完整 | `api/complaint.go` 中真实实现了 `/v1/merchant/complaints`、`/{id}`、`/{id}/response`、`/{id}/complete`，但完整 Swagger `@Router` 注解只显式覆盖了 `/summary` | 只看 Swagger 会误判投诉详情与动作接口缺失 |
| 员工邀请码注解路径漂移 | `api/server.go` 真实注册路径是 `POST /v1/merchant/staff/invite-code`，而 `api/staff.go` 中注解写成了 `POST /v1/merchant/invite-code` | 只看注解会与小程序 API 封装产生路径不一致判断 |
| 满减规则主要依赖路由注册 | `/v1/merchants/{id}/discounts*` 在 `api/server.go` 中真实注册，但 `api/discount.go` 对应 Swagger 暴露不完整 | 能力真实存在，但文档化程度不足 |

## 对齐建议

1. 若要继续补齐商户端能力，优先级最高的是结算账户资料页、多门店切换入口，以及房间独立管理入口；其中房间能力并非完全未接，而是尚未形成独立商户控制台页面闭环。
2. 若要收口合同一致性，优先修正投诉与员工邀请码的 Swagger 注解，再补齐满减规则路由的注解覆盖，避免后续审计再次误判。
3. 后续新增商户页时，不应只搜索 `/v1/merchant/**`，必须同时覆盖 `/v1/merchants/me/**`、`/v1/merchants/{id}/**`、`/v1/kitchen/**`、`/v1/reservations/**`、`/v1/dishes/**`、`/v1/combos/**`、`/v1/tables/**`、`/v1/reviews/**`、`/v1/groups/**` 与 `/v1/ocr/jobs`。