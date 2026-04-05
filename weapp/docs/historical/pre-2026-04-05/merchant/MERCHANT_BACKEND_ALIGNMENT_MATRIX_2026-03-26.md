# 商户侧后端能力对齐矩阵

日期：2026-03-26

> 历史快照说明
>
> 本文是 2026-03-26 阶段的商户侧后端能力对齐矩阵快照，保留的是当时的能力盘点、合同判断和治理约束，不代表当前商户侧所有页面和接口都仍处于文中所述状态。
>
> 涉及当前实现时，请优先以实际代码、后端真实路由和后续收口文档为准；本文更适合作为历史对齐背景和设计约束参考。

适用范围：
- 小程序商户侧 `weapp/miniprogram/pages/merchant/**`
- 小程序商户 API 层 `weapp/miniprogram/api/**`
- 后端商户能力合同 `locallife/api/**`

## 目标

本文件用于定义“小程序商户侧必须严格对齐后端能力”的单一事实来源，避免继续出现以下漂移：

1. 后端已开放商户能力，但小程序没有入口或没有页面承接。
2. 小程序页面存在，但只承接了后端能力的一部分动作或一部分状态。
3. 小程序页面自行定义状态、动作或文案，与后端真实合同不一致。
4. API 层已有封装但页面没有消费，形成死能力。

## 单一事实来源

优先级从高到低如下：

1. 路由注册与权限边界：`locallife/api/server.go`
2. handler 的请求/响应合同：`locallife/api/*.go`
3. 业务状态常量：`locallife/db/sqlc/constants.go`
4. 小程序 API 封装：`weapp/miniprogram/api/*.ts`
5. 小程序页面：`weapp/miniprogram/pages/merchant/**`

## 状态与权限约束

### 订单状态

以 `locallife/db/sqlc/constants.go` 为准：

| 常量 | 值 | 前端要求 |
| --- | --- | --- |
| `OrderStatusPending` | `pending` | 仅用户态可见，不应出现在商户待处理主流中 |
| `OrderStatusPaid` | `paid` | 商户待接单主入口 |
| `OrderStatusPreparing` | `preparing` | 商户制作中主入口 |
| `OrderStatusReady` | `ready` | 商户待取餐/待核销/待送达主入口 |
| `OrderStatusCourierAccepted` | `courier_accepted` | 外卖履约追踪态，商户应可见 |
| `OrderStatusPicked` | `picked` | 外卖履约追踪态，商户应可见 |
| `OrderStatusDelivering` | `delivering` | 外卖履约追踪态，商户应可见 |
| `OrderStatusRiderDelivered` | `rider_delivered` | 履约追踪态，商户详情应可见 |
| `OrderStatusUserDelivered` | `user_delivered` | 履约追踪态，商户详情应可见 |
| `OrderStatusCompleted` | `completed` | 已完成归档态 |
| `OrderStatusCancelled` | `cancelled` | 已取消归档态 |

### 商户角色权限

以 `MerchantStaffMiddleware` 为准：

| 能力域 | 后端角色约束 | 前端要求 |
| --- | --- | --- |
| 商户订单 | `owner` `manager` `cashier` | 页面可见性和按钮权限必须收口到这三类 |
| 后厨 KDS | `owner` `manager` `chef` | KDS 页面不能对 `cashier` 开放主动作 |
| 菜品/套餐/库存 | `owner` `manager` `chef` | 菜单经营类页面不能默认对 `cashier` 显示编辑入口 |
| 配送优惠 | `owner` `manager` | 活动配置页不能对 `chef`/`cashier` 暴露编辑入口 |
| 员工管理 | `owner` 或 `owner+manager` | 角色编辑/删除必须细分 owner 专属动作 |
| 投诉处理 | `owner` `manager` | 投诉页必须角色感知 |

## 能力域总表

### A. 工作台与经营总览

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 今日订单/收入统计 | `GET /v1/merchant/orders/stats` | `locallife/api/order.go` | `merchant/dashboard` 已部分承接 | 保留并升级 |
| 经营概览 | `GET /v1/merchant/stats/overview` | `locallife/api/merchant_stats.go` | API 有封装，首页未完整承接 | 必须补齐 |
| 热销菜 | `GET /v1/merchant/stats/dishes/top` | `locallife/api/merchant_stats.go` | API 有封装，页面未见完整承接 | 必须补齐 |
| 日统计 | `GET /v1/merchant/stats/daily` | `locallife/api/merchant_stats.go` | 未承接 | 新增 |
| 小时统计 | `GET /v1/merchant/stats/hourly` | `locallife/api/merchant_stats.go` | 未承接 | 新增 |
| 复购率 | `GET /v1/merchant/stats/repurchase` | `locallife/api/merchant_stats.go` | 未承接 | 新增 |
| 类目销售分析 | `GET /v1/merchant/stats/categories` | `locallife/api/merchant_stats.go` | 未承接 | 新增 |
| 客户分析 | `GET /v1/merchant/stats/customers` `GET /v1/merchant/stats/customers/{user_id}` | `locallife/api/merchant_stats.go` | 未承接 | 新增二级页 |

### B. 商户订单主链路

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 订单列表 | `GET /v1/merchant/orders` | `locallife/api/order.go` | `merchant/orders/list` 已承接 | 重构并补齐状态 |
| 订单详情 | `GET /v1/merchant/orders/{id}` | `locallife/api/order.go` | `merchant/orders/detail` 已承接 | 重构 |
| 接单 | `POST /v1/merchant/orders/{id}/accept` | `locallife/api/order.go` | 已承接 | 保留 |
| 拒单 | `POST /v1/merchant/orders/{id}/reject` | `locallife/api/order.go` | 页面未完整承接 | 必须补齐 |
| 标记制作完成 | `POST /v1/merchant/orders/{id}/ready` | `locallife/api/order.go` | 已承接 | 保留 |
| 完成订单 | `POST /v1/merchant/orders/{id}/complete` | `locallife/api/order.go` | 已承接 | 保留 |
| 订单统计 | `GET /v1/merchant/orders/stats` | `locallife/api/order.go` | 首页部分使用 | 扩展到工作台/统计页 |

### C. 后厨 KDS

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 后厨订单分组列表 | `GET /v1/kitchen/orders` | `locallife/api/kitchen.go` | API 已封装，页面缺失 | 必须新增 |
| 后厨订单详情 | `GET /v1/kitchen/orders/{id}` | `locallife/api/kitchen.go` | API 已封装，页面缺失 | 必须新增 |
| 开始制作 | `POST /v1/kitchen/orders/{id}/preparing` | `locallife/api/kitchen.go` | API 已封装，页面缺失 | 必须新增 |
| 标记出餐 | `POST /v1/kitchen/orders/{id}/ready` | `locallife/api/kitchen.go` | API 已封装，页面缺失 | 必须新增 |

### D. 菜品管理

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 创建菜品 | `POST /v1/dishes` | `locallife/api/dish.go` | `merchant/dishes/edit` 已承接 | 保留 |
| 菜品列表 | `GET /v1/dishes` | `locallife/api/dish.go` | `merchant/dishes` 已承接 | 重构 |
| 菜品详情 | `GET /v1/dishes/{id}` | `locallife/api/dish.go` | 编辑页已承接 | 保留 |
| 更新菜品 | `PUT /v1/dishes/{id}` | `locallife/api/dish.go` | 已承接 | 保留 |
| 删除菜品 | `DELETE /v1/dishes/{id}` | `locallife/api/dish.go` | 已承接 | 保留 |
| 单品上下架 | `PATCH /v1/dishes/{id}/status` | `locallife/api/dish.go` | 已承接 | 保留 |
| 批量上下架 | `PATCH /v1/dishes/batch/status` | `locallife/api/dish.go` | 未承接 | 必须补齐 |
| 定制项/规格获取 | `GET /v1/dishes/{id}/customizations` | `locallife/api/dish.go` | 承接不完整 | 必须补齐 |
| 定制项/规格设置 | `PUT /v1/dishes/{id}/customizations` `PUT /v1/dishes/{id}/specs` | `locallife/api/dish.go` | 承接不完整 | 必须补齐 |
| 热卖/推荐标签 | `PUT /v1/dishes/{id}/featured-tags` | `locallife/api/dish.go` | 未承接 | 必须补齐 |

### E. 菜品分类与商户经营类目

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 分类列表 | `GET /v1/dishes/categories` | `locallife/api/dish.go` | 已承接 | 保留 |
| 创建分类 | `POST /v1/dishes/categories` | `locallife/api/dish.go` | 已承接 | 保留 |
| 更新分类 | `PATCH /v1/dishes/categories/{id}` | `locallife/api/dish.go` | 已承接 | 保留 |
| 删除分类 | `DELETE /v1/dishes/categories/{id}` | `locallife/api/dish.go` | 已承接 | 保留 |
| 全局类目库 | `GET /v1/dishes/categories/global` | `locallife/api/dish.go` | 未见完整承接 | 补齐 |
| 商户经营标签读取 | `GET /v1/merchants/me/tags` | `locallife/api/merchant.go` | 页面存在但配置链路分散 | 重构进设置域 |
| 商户经营标签写入 | `PUT /v1/merchants/me/tags` | `locallife/api/merchant.go` | 页面存在但与配置中心未统一 | 重构进设置域 |

### F. 套餐管理

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 创建套餐 | `POST /v1/combos` | `locallife/api/combo.go` | 已承接 | 保留 |
| 套餐列表 | `GET /v1/combos` | `locallife/api/combo.go` | 已承接 | 重构 |
| 套餐详情 | `GET /v1/combos/{id}` | `locallife/api/combo.go` | 已承接 | 保留 |
| 更新套餐 | `PUT /v1/combos/{id}` | `locallife/api/combo.go` | 已承接 | 保留 |
| 套餐上下架 | `PUT /v1/combos/{id}/online` | `locallife/api/combo.go` | 已承接 | 保留 |
| 删除套餐 | `DELETE /v1/combos/{id}` | `locallife/api/combo.go` | 已承接 | 保留 |
| 关联菜品 | `POST /v1/combos/{id}/dishes` | `locallife/api/combo.go` | 编辑页承接不完整 | 必须补齐 |
| 移除菜品 | `DELETE /v1/combos/{id}/dishes/{dish_id}` | `locallife/api/combo.go` | 编辑页承接不完整 | 必须补齐 |

### G. 库存管理

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 创建日库存 | `POST /v1/inventory` | `locallife/api/inventory.go` | 页面承接有限 | 补齐 |
| 查询库存 | `GET /v1/inventory` | `locallife/api/inventory.go` | 已承接 | 重构 |
| 更新日库存 | `PUT /v1/inventory` | `locallife/api/inventory.go` | 已承接 | 保留 |
| 更新单品库存 | `PATCH /v1/inventory/{dish_id}` | `locallife/api/inventory.go` | 已承接 | 保留 |
| 库存检查扣减 | `POST /v1/inventory/check` | `locallife/api/inventory.go` | 商户页无直接入口 | 无需前台入口，但需在经营页呈现结果语义 |
| 库存统计 | `GET /v1/inventory/stats` | `locallife/api/inventory.go` | 未见前端承接 | 必须补齐 |

### H. 桌台与预订

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 桌台列表 | `GET /v1/tables` | `locallife/api/table.go` | 已承接 | 重构 |
| 创建桌台 | `POST /v1/tables` | `locallife/api/table.go` | 已承接 | 保留 |
| 更新桌台 | `PATCH /v1/tables/{id}` | `locallife/api/table.go` | 已承接 | 保留 |
| 更新桌台状态 | `PATCH /v1/tables/{id}/status` | `locallife/api/table.go` | 已承接 | 保留 |
| 删除桌台 | `DELETE /v1/tables/{id}` | `locallife/api/table.go` | 已承接 | 保留 |
| 桌台标签 CRUD | `POST/DELETE/GET /v1/tables/{id}/tags` | `locallife/api/table.go` | API 有基础，页面未完整承接 | 补齐 |
| 桌台图片 CRUD | `POST/GET/PUT/DELETE /v1/tables/{id}/images*` | `locallife/api/table.go` | API 有基础，页面未完整承接 | 补齐 |
| 桌台二维码 | `GET /v1/tables/{id}/qrcode` | `locallife/api/table.go` | 页面未完整承接 | 补齐 |
| 商户预订列表 | `GET /v1/reservations/merchant` | `locallife/api/table_reservation.go` | 已承接 | 重构 |
| 商户预订菜品摘要 | `GET /v1/reservations/merchant/dishes` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 今日预订 | `GET /v1/reservations/merchant/today` | `locallife/api/table_reservation.go` | 未承接 | 补齐 |
| 预订统计 | `GET /v1/reservations/merchant/stats` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 商户代客创建预订 | `POST /v1/reservations/merchant/create` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 商户修改预订 | `PUT /v1/reservations/{id}/update` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 确认预订 | `POST /v1/reservations/{id}/confirm` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 完成预订 | `POST /v1/reservations/{id}/complete` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |
| 标记爽约 | `POST /v1/reservations/{id}/no-show` | `locallife/api/table_reservation.go` | 页面未完整承接 | 补齐 |

### I. 打印机与展示配置

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 打印机列表 | `GET /v1/merchant/devices` | `locallife/api/device.go` | 已承接 | 重构 |
| 添加打印机 | `POST /v1/merchant/devices` | `locallife/api/device.go` | 已承接 | 保留 |
| 查看打印机 | `GET /v1/merchant/devices/{id}` | `locallife/api/device.go` | 页面承接有限 | 补齐 |
| 更新打印机 | `PUT /v1/merchant/devices/{id}` | `locallife/api/device.go` | 已承接 | 保留 |
| 删除打印机 | `DELETE /v1/merchant/devices/{id}` | `locallife/api/device.go` | 已承接 | 保留 |
| 测试打印 | `POST /v1/merchant/devices/{id}/test` | `locallife/api/device.go` | 已承接 | 保留 |
| 展示配置读取 | `GET /v1/merchant/display-config` | `locallife/api/device.go` | API 存在，页面未完整承接 | 必须补齐 |
| 展示配置写入 | `PUT /v1/merchant/display-config` | `locallife/api/device.go` | API 存在，页面未完整承接 | 必须补齐 |

### J. 配送优惠与营销

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 创建配送优惠 | `POST /v1/delivery-fee/merchants/{merchant_id}/promotions` | `locallife/api/delivery_fee.go` | 已承接 | 保留 |
| 优惠列表 | `GET /v1/delivery-fee/merchants/{merchant_id}/promotions` | `locallife/api/delivery_fee.go` | 已承接 | 重构 |
| 更新优惠 | `PATCH /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` | `locallife/api/delivery_fee.go` | 已承接 | 保留 |
| 删除优惠 | `DELETE /v1/delivery-fee/merchants/{merchant_id}/promotions/{id}` | `locallife/api/delivery_fee.go` | 已承接 | 保留 |
| 营销支出明细 | `GET /v1/merchant/finance/promotions` | `locallife/api/merchant_finance.go` | 未承接 | 并入财务域 |

### K. 商户资料与配置

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 获取当前商户资料 | `GET /v1/merchants/me` | `locallife/api/merchant.go` | 页面分散承接 | 重构进设置域 |
| 获取我的商户列表 | `GET /v1/merchants/my` | `locallife/api/merchant.go` | 前端未完整承接 | 补齐身份切换 |
| 更新商户资料 | `PATCH /v1/merchants/me` | `locallife/api/merchant.go` | 页面分散承接 | 重构 |
| 更新门店图资 | `PATCH /v1/merchants/me/shop-images` | `locallife/api/merchant.go` | `profile-images` 部分承接 | 重构 |
| 获取营业状态 | `GET /v1/merchants/me/status` | `locallife/api/merchant.go` | 首页部分承接 | 统一到设置与首页 |
| 更新营业状态 | `PATCH /v1/merchants/me/status` | `locallife/api/merchant.go` | 首页已承接 | 保留 |
| 获取营业时间 | `GET /v1/merchants/me/business-hours` | `locallife/api/merchant.go` | 配置中心未系统承接 | 新增设置页 |
| 设置营业时间 | `PUT /v1/merchants/me/business-hours` | `locallife/api/merchant.go` | 配置中心未系统承接 | 新增设置页 |
| 获取会员设置 | `GET /v1/merchants/me/membership-settings` | `locallife/api/merchant.go` | 未承接 | 新增设置页 |
| 更新会员设置 | `PUT /v1/merchants/me/membership-settings` | `locallife/api/merchant.go` | 未承接 | 新增设置页 |

### L. 商户申请与进件开户

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 获取/创建商户申请草稿 | `GET /v1/merchant/application` | `locallife/api/merchant_application.go` | 小程序商户侧未系统承接 | 新增设置页 |
| 更新申请基础信息 | `PUT /v1/merchant/application/basic` | `locallife/api/merchant_application.go` | 未系统承接 | 新增设置页 |
| 更新申请图片 | `PUT /v1/merchant/application/images` | `locallife/api/merchant_application.go` | 未系统承接 | 新增设置页 |
| 提交申请 | `POST /v1/merchant/application/submit` | `locallife/api/merchant_application.go` | 未系统承接 | 新增设置页 |
| 重置申请 | `POST /v1/merchant/application/reset` | `locallife/api/merchant_application.go` | 未系统承接 | 新增设置页 |
| 获取进件状态 | `GET /v1/merchant/applyment/status` | `locallife/api/ecommerce_applyment.go` | `merchant/finance` 部分承接 | 重构 |
| 绑定银行账户 | `POST /v1/merchant/applyment/bindbank` | `locallife/api/ecommerce_applyment.go` | `merchant/finance` 部分承接 | 重构 |

### M. 财务与结算

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 账户余额 | `GET /v1/merchant/finance/account/balance` | `weapp/miniprogram/api/merchant-finance.ts` 对应后端 handler | 已承接 | 保留 |
| 提现申请 | `POST /v1/merchant/finance/account/withdraw` | 同上 | 已承接 | 保留 |
| 提现记录 | `GET /v1/merchant/finance/account/withdrawals` | 同上 | 已承接 | 保留 |
| 财务概览 | `GET /v1/merchant/finance/overview` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 财务订单明细 | `GET /v1/merchant/finance/orders` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 服务费明细 | `GET /v1/merchant/finance/service-fees` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 营销支出 | `GET /v1/merchant/finance/promotions` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 财务日报 | `GET /v1/merchant/finance/daily` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 结算记录 | `GET /v1/merchant/finance/settlements` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |
| 结算时间线 | `GET /v1/merchant/finance/settlement-timeline` | `locallife/api/merchant_finance.go` | 未承接 | 必须补齐 |

### N. 索赔、申诉与风控

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 商户索赔列表 | `GET /v1/merchant/claims` | `locallife/api/appeal.go`/相关 handler | 已承接 | 重构 |
| 索赔详情 | `GET /v1/merchant/claims/{id}` | 同上 | 已承接 | 重构 |
| 判责依据 | `GET /v1/merchant/claims/{id}/decision` | `locallife/api/appeal.go` | 已有商户端能力，页面承接不完整 | 必须补齐 |
| 用户行为摘要 | `GET /v1/merchant/claims/behavior-summary` | `locallife/api/appeal.go` | 未完整承接 | 必须补齐 |
| 追偿信息 | `GET /v1/merchant/claims/{id}/recovery` | `locallife/api/claim_recovery.go` | 已部分承接 | 补齐 |
| 追偿支付 | `POST /v1/merchant/claims/{id}/recovery/pay` | `locallife/api/claim_recovery.go` | 已部分承接 | 补齐 |
| 创建申诉 | `POST /v1/merchant/appeals` | `locallife/api/appeal.go` | 已承接 | 保留 |
| 申诉列表 | `GET /v1/merchant/appeals` | `locallife/api/appeal.go` | 已承接 | 重构 |
| 申诉详情 | `GET /v1/merchant/appeals/{id}` | `locallife/api/appeal.go` | 页面承接有限 | 补齐 |
| 用户风险信息 | `GET /v1/merchant/risk/users/{id}` | `locallife/api/appeal.go` / risk 相关 | 未承接 | 新增二级页或弹层 |

### O. 投诉处理

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 投诉列表 | `GET /v1/merchant/complaints` | `locallife/api/server.go` 注册，具体 handler 在投诉域文件 | 页面缺失 | 必须新增 |
| 投诉详情 | `GET /v1/merchant/complaints/{id}` | 同上 | 页面缺失 | 必须新增 |
| 商户回复投诉 | `POST /v1/merchant/complaints/{id}/response` | 同上 | 页面缺失 | 必须新增 |
| 完成投诉处理 | `POST /v1/merchant/complaints/{id}/complete` | 同上 | 页面缺失 | 必须新增 |

### P. 员工管理

| 能力 | 后端接口 | 后端文件 | 当前小程序承接 | 结论 |
| --- | --- | --- | --- | --- |
| 员工列表 | `GET /v1/merchant/staff` | `locallife/api/staff.go` | 页面缺失 | 必须新增 |
| 生成邀请码 | `POST /v1/merchant/staff/invite-code` | `locallife/api/staff.go` | 商户域无页面承接，用户域有绑定页 | 必须新增商户页入口 |
| 新增员工 | `POST /v1/merchant/staff` | `locallife/api/staff.go` | 页面缺失 | 必须新增 |
| 更新角色 | `PATCH /v1/merchant/staff/{id}/role` | `locallife/api/staff.go` | 页面缺失 | 必须新增 |
| 删除员工 | `DELETE /v1/merchant/staff/{id}` | `locallife/api/staff.go` | 页面缺失 | 必须新增 |
| 员工绑定商户 | `POST /v1/bind-merchant` | `locallife/api/server.go` | 用户中心已有入口 | 保持现状并与员工管理页串联 |

## 当前前端 API 层对齐情况

### 已有较好的基础

| 文件 | 当前价值 | 结论 |
| --- | --- | --- |
| `weapp/miniprogram/api/order-management.ts` | 已同时覆盖 merchant orders 和 kitchen DTO/服务封装 | 作为订单/KDS 合同层基础继续扩展 |
| `weapp/miniprogram/api/table-device-management.ts` | 已覆盖桌台、打印机、展示配置等基础接口 | 作为店务与设备域基础继续扩展 |
| `weapp/miniprogram/api/merchant-finance.ts` | 已覆盖账户余额、提现、进件状态、绑卡 | 必须扩展为完整 finance 域 |
| `weapp/miniprogram/api/merchant-stats.ts` | 已覆盖概览与热销菜 | 必须扩展为完整 stats 域 |

### 需要新建或大幅扩展的 API 域

1. `merchant-complaints.ts`
2. `merchant-staff.ts`
3. `merchant-settings.ts` 或按域拆分的 `merchant-profile.ts` `merchant-application.ts`
4. `merchant-risk.ts`
5. `merchant-analytics.ts` 或扩展现有 `merchant-stats.ts`

## 强制对齐要求

### 页面级要求

每个商户页面必须满足：

1. 至少一个后端能力域作为事实来源。
2. 明确列出依赖的接口、请求参数、响应字段、状态字段。
3. 覆盖 loading、success、empty、error 四态。
4. 覆盖 submitting、success、failure、refresh 四类动作反馈。
5. 对角色限制有显式 UI 表达，不允许依赖点击后 403 才告知用户。

### API 级要求

1. 页面不得直接拼接商户合同 URL。
2. 所有状态文案、颜色、图标映射必须走统一 adapter。
3. 不允许页面本地定义与后端冲突的 status union type。
4. 订单、申诉、投诉、财务、设备等域必须按 capability 分文件，不继续堆在页面里。

## 一次性大改落地顺序

1. 先完成本矩阵的“接口级对齐表”补充到页面任务中。
2. 先补共享基础设施与 API 合同层。
3. 再迁移核心交易链：dashboard、orders、kitchen、tables、reservations、printers。
4. 再迁移菜单经营链：dishes、combos、inventory、promotions。
5. 再迁移资金风控链：finance、claims、appeals、complaints、stats。
6. 最后迁移设置与人员链：config、profile、application、applyment、staff。

## 最终验收口径

以 `locallife/api/server.go` 中商户相关路由为准，验收必须回答两个问题：

1. 每条路由是否在小程序中有明确落点。
2. 每个落点是否具备完整操作闭环，而不是只有列表或只有入口。

只要有任一路由没有明确落点，这次商户侧改造就不能视为“严格对齐后端”。