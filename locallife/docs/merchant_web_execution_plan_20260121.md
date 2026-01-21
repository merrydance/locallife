# 商户侧 Web 全量替换执行计划（逐项勾选清单）

> 目标：对齐小程序商户侧现有页面与功能，并覆盖新增后端能力；同步补齐 Web 登录扫码与集团/品牌相关流程。
> 说明：本清单基于小程序目录结构 + 后端路由定义整理，字段与类型需以 swagger 为准。
> 参考：
> - [locallife/docs/swagger.yaml](locallife/docs/swagger.yaml)
> - [locallife/api/server.go](locallife/api/server.go)
> - [docs/ai_coding_prompt.md](docs/ai_coding_prompt.md)
> - [.agent/rules/project-rules.md](.agent/rules/project-rules.md)
> - [docs/merchant_web_catalog_draft_20260121.md](docs/merchant_web_catalog_draft_20260121.md)

## 统一规范与约定（新实现必须遵守）
- 以最新后端/Swagger 为准；前端仅做适配，避免反向约束后端。
- 统一响应包裹：默认返回 `{code,message,data}`，见 [locallife/api/response_envelope.go](locallife/api/response_envelope.go)。
- 路由注册对齐现有分组与中间件（鉴权、权限、限流、Tracing、Prometheus）。
- 数据访问统一走 sqlc Store；新增查询放 db/query，迁移放 db/migration。
- 配置统一来自 `util.Config`；日志使用 zerolog，避免敏感数据。
- 上传/下载遵循签名/上传路径约定，避免直接暴露私有资源。

---

## A. 小程序商户侧现存页面/功能清单（需 Web 等价实现）

> 以小程序目录为模块单位；每项包含“页面/功能 → 主要后端接口”。

### A0. 商户侧页面全量清单（含二级/三级页面）
> 来自小程序目录 `weapp/miniprogram/pages/merchant/**` 的页面配置文件（*.json）。
- [ ] [weapp/miniprogram/pages/merchant/admin/index.json](weapp/miniprogram/pages/merchant/admin/index.json)
- [ ] [weapp/miniprogram/pages/merchant/analytics/index.json](weapp/miniprogram/pages/merchant/analytics/index.json)
- [ ] [weapp/miniprogram/pages/merchant/analytics/dashboard/dashboard.json](weapp/miniprogram/pages/merchant/analytics/dashboard/dashboard.json)
- [ ] [weapp/miniprogram/pages/merchant/analytics/dishes/index.json](weapp/miniprogram/pages/merchant/analytics/dishes/index.json)
- [ ] [weapp/miniprogram/pages/merchant/analytics/enhanced/index.json](weapp/miniprogram/pages/merchant/analytics/enhanced/index.json)
- [ ] [weapp/miniprogram/pages/merchant/analytics/sales/index.json](weapp/miniprogram/pages/merchant/analytics/sales/index.json)
- [ ] [weapp/miniprogram/pages/merchant/combos/index.json](weapp/miniprogram/pages/merchant/combos/index.json)
- [ ] [weapp/miniprogram/pages/merchant/dashboard/index.json](weapp/miniprogram/pages/merchant/dashboard/index.json)
- [ ] [weapp/miniprogram/pages/merchant/delivery-settings/index.json](weapp/miniprogram/pages/merchant/delivery-settings/index.json)
- [ ] [weapp/miniprogram/pages/merchant/dinein/index.json](weapp/miniprogram/pages/merchant/dinein/index.json)
- [ ] [weapp/miniprogram/pages/merchant/discounts/index.json](weapp/miniprogram/pages/merchant/discounts/index.json)
- [ ] [weapp/miniprogram/pages/merchant/dishes/index.json](weapp/miniprogram/pages/merchant/dishes/index.json)
- [ ] [weapp/miniprogram/pages/merchant/dishes/edit/index.json](weapp/miniprogram/pages/merchant/dishes/edit/index.json)
- [ ] [weapp/miniprogram/pages/merchant/dishes/manage/manage.json](weapp/miniprogram/pages/merchant/dishes/manage/manage.json)
- [ ] [weapp/miniprogram/pages/merchant/finance/index.json](weapp/miniprogram/pages/merchant/finance/index.json)
- [ ] [weapp/miniprogram/pages/merchant/health/index.json](weapp/miniprogram/pages/merchant/health/index.json)
- [ ] [weapp/miniprogram/pages/merchant/inventory/index.json](weapp/miniprogram/pages/merchant/inventory/index.json)
- [ ] [weapp/miniprogram/pages/merchant/kds/index.json](weapp/miniprogram/pages/merchant/kds/index.json)
- [ ] [weapp/miniprogram/pages/merchant/membership-settings/index.json](weapp/miniprogram/pages/merchant/membership-settings/index.json)
- [ ] [weapp/miniprogram/pages/merchant/members/index.json](weapp/miniprogram/pages/merchant/members/index.json)
- [ ] [weapp/miniprogram/pages/merchant/navigation/index.json](weapp/miniprogram/pages/merchant/navigation/index.json)
- [ ] [weapp/miniprogram/pages/merchant/orders/index.json](weapp/miniprogram/pages/merchant/orders/index.json)
- [ ] [weapp/miniprogram/pages/merchant/reservations/index.json](weapp/miniprogram/pages/merchant/reservations/index.json)
- [ ] [weapp/miniprogram/pages/merchant/review/manage/index.json](weapp/miniprogram/pages/merchant/review/manage/index.json)
- [ ] [weapp/miniprogram/pages/merchant/settings/index.json](weapp/miniprogram/pages/merchant/settings/index.json)
- [ ] [weapp/miniprogram/pages/merchant/staff/index.json](weapp/miniprogram/pages/merchant/staff/index.json)
- [ ] [weapp/miniprogram/pages/merchant/tables/index.json](weapp/miniprogram/pages/merchant/tables/index.json)
- [ ] [weapp/miniprogram/pages/merchant/tables/manage/manage.json](weapp/miniprogram/pages/merchant/tables/manage/manage.json)
- [ ] [weapp/miniprogram/pages/merchant/vouchers/index.json](weapp/miniprogram/pages/merchant/vouchers/index.json)

> 说明：`merchant/components/**` 为组件而非页面，不纳入页面替换清单。

### A0.1 页面级展开（功能 + 接口 + 字段对齐）
> 以最新后端/Swagger 为准；小程序实现若不一致会标注“需适配/需补齐”。

#### [weapp/miniprogram/pages/merchant/admin/index.ts](weapp/miniprogram/pages/merchant/admin/index.ts)
- 功能点：模块导航入口（订单/菜品/桌台/库存/堂食/后厨/分析/财务/营销/评价/预订/店铺/健康）
- 调用接口：无
- 字段对齐：N/A
- 备注：入口指向未落地页面：/pages/merchant/printers/index、/pages/merchant/finance/settlement、/pages/merchant/marketing/index、/pages/merchant/review/index、/pages/merchant/profile/index

#### [weapp/miniprogram/pages/merchant/analytics/index.ts](weapp/miniprogram/pages/merchant/analytics/index.ts)
- 功能点：统计概览、日报、热销菜品、时段分析、来源、复购率、分类销售、客户排行
- 调用接口：
  - GET /v1/merchant/stats/overview (start_date, end_date)
  - GET /v1/merchant/stats/daily (start_date, end_date)
  - GET /v1/merchant/stats/dishes/top (start_date, end_date, limit)
  - GET /v1/merchant/stats/hourly (date)
  - GET /v1/merchant/stats/sources (start_date, end_date)
  - GET /v1/merchant/stats/repurchase (start_date, end_date)
  - GET /v1/merchant/stats/categories (start_date, end_date)
  - GET /v1/merchant/stats/customers (start_date, end_date, page_id, page_size)
- 字段对齐：
  - 以 swagger 为准；优先采用 [weapp/miniprogram/api/merchant-analytics.ts](weapp/miniprogram/api/merchant-analytics.ts) 定义
- 适配点：当前页面内 StatsService 字段与 swagger 不一致（需以最新后端字段替换）

#### [weapp/miniprogram/pages/merchant/analytics/dashboard/dashboard.ts](weapp/miniprogram/pages/merchant/analytics/dashboard/dashboard.ts)
- 功能点：经营概览、销售分析、财务概览、客户分析
- 调用接口：
  - GET /v1/merchant/stats/overview (start_date, end_date)
  - GET /v1/merchant/stats/daily (start_date, end_date)
  - GET /v1/merchant/stats/dishes/top (start_date, end_date, limit)
  - GET /v1/merchant/stats/categories (start_date, end_date)
  - GET /v1/merchant/stats/customers (start_date, end_date, page_id, page_size)
  - GET /v1/merchant/stats/repurchase (start_date, end_date)
  - GET /v1/merchant/finance/overview (start_date, end_date)
- 字段对齐：
  - statsOverview: total_orders, total_revenue, total_customers, avg_order_value, completion_rate, growth_rate
  - dailyStats: date, orders, revenue, customers, avg_order_value
  - topDishes: dish_id, dish_name, sales_count, revenue, rank
  - categoryStats: category_id, category_name, sales_count, revenue, percentage
  - customerStats: user_id, username, total_orders, total_spent, avg_order_value, last_order_date
  - repurchaseStats: total_customers, repurchase_customers, repurchase_rate, avg_repurchase_interval
  - financeOverview: completed_orders, net_income, pending_income, pending_orders, promotion_orders, total_gmv, total_income, total_operator_fee, total_platform_fee, total_promotion_exp, total_service_fee

#### [weapp/miniprogram/pages/merchant/analytics/dishes/index.ts](weapp/miniprogram/pages/merchant/analytics/dishes/index.ts)
- 功能点：菜品分类占比、热销菜品榜
- 调用接口：当前为 mock（需补齐）
  - 建议对接 GET /v1/merchant/stats/dishes/top (start_date, end_date, limit)
  - 建议对接 GET /v1/merchant/stats/categories (start_date, end_date)
- 字段对齐：
  - top dishes: dish_name, total_sold 或 sales_count, total_revenue 或 revenue
  - categories: category_name, total_sales 或 revenue, order_count 或 total_quantity

#### [weapp/miniprogram/pages/merchant/analytics/enhanced/index.ts](weapp/miniprogram/pages/merchant/analytics/enhanced/index.ts)
- 功能点：轻量经营概览（今日GMV/订单/待处理/近7日趋势）
- 调用接口：
  - GET /v1/merchants/me/dashboard
- 字段对齐：
  - today_sales, today_orders, pending_orders, seven_days_sales[{ date, amount }]
- 备注：时段分布饼图当前为 mock，需补齐时段分布接口

#### [weapp/miniprogram/pages/merchant/analytics/sales/index.ts](weapp/miniprogram/pages/merchant/analytics/sales/index.ts)
- 功能点：销售额与订单量趋势图、日维度明细
- 调用接口：当前为 mock（需补齐）
  - 建议对接 GET /v1/merchant/stats/daily (start_date, end_date)
- 字段对齐：
  - daily: date, total_sales 或 revenue, order_count 或 orders, avg_order_value

#### [weapp/miniprogram/pages/merchant/combos/index.ts](weapp/miniprogram/pages/merchant/combos/index.ts)
- 功能点：套餐列表/详情/新增/编辑/上下架/关联菜品/标签
- 调用接口：
  - GET /v1/combos (page_id, page_size)
  - GET /v1/combos/:id
  - POST /v1/combos
  - PUT /v1/combos/:id
  - DELETE /v1/combos/:id
  - PUT /v1/combos/:id/online
  - POST /v1/combos/:id/dishes
  - DELETE /v1/combos/:id/dishes/:dish_id
  - GET /v1/dishes (page_id, page_size)
  - GET /v1/tags (type=dish)
- 字段对齐：
  - combo: id, name, description, combo_price, is_online
  - combo details: dishes[{ dish_id, dish_name, dish_price, quantity }], tags[{ id, name }]
  - create/update: name, description, combo_price, is_online, dishes[{ dish_id, quantity }], tag_ids

#### [weapp/miniprogram/pages/merchant/dashboard/index.ts](weapp/miniprogram/pages/merchant/dashboard/index.ts)
- 功能点：工作台概览、待处理订单、桌台状态、WebSocket实时更新
- 调用接口：
  - GET /v1/merchants/me
  - GET /v1/merchant/stats/overview (start_date, end_date=今日)
  - GET /v1/merchant/orders (page_id, page_size)
  - GET /v1/tables
  - PATCH /v1/tables/:id/status (status)
  - WebSocket /v1/ws?token=...（实时订单与通知）
- 字段对齐：
  - merchant: id, name, is_open
  - stats: total_revenue, total_orders
  - order list: id, order_no, status, order_type, total_amount, items[{ name }], table_no, created_at
  - tables: id, table_no, table_type, status, capacity, current_reservation_id

#### [weapp/miniprogram/pages/merchant/delivery-settings/index.ts](weapp/miniprogram/pages/merchant/delivery-settings/index.ts)
- 功能点：运费减免规则列表/新增/删除
- 调用接口：
  - GET /v1/delivery-fee/merchants/:merchant_id/promotions
  - POST /v1/delivery-fee/merchants/:merchant_id/promotions
  - DELETE /v1/delivery-fee/merchants/:merchant_id/promotions/:id
- 字段对齐：
  - promotion: id, name, promotion_type, discount_value, min_order_amount, start_time, end_time, is_active
  - create: name, promotion_type, discount_value, min_order_amount, start_time, end_time, is_active

#### [weapp/miniprogram/pages/merchant/dinein/index.ts](weapp/miniprogram/pages/merchant/dinein/index.ts)
- 功能点：堂食桌台状态、开台/结台
- 调用接口：
  - GET /v1/tables (table_type=table)
  - PATCH /v1/tables/:id/status (status)
- 字段对齐：
  - table: id, table_no, status, capacity, description, minimum_spend, current_reservation_id
  - update status: status

#### [weapp/miniprogram/pages/merchant/discounts/index.ts](weapp/miniprogram/pages/merchant/discounts/index.ts)
- 功能点：满减规则列表/创建/编辑/删除
- 调用接口：
  - GET /v1/merchants/:id/discounts (page_id, page_size)
  - POST /v1/merchants/:id/discounts
  - PATCH /v1/merchants/:id/discounts/:id
  - DELETE /v1/merchants/:id/discounts/:id
- 字段对齐：
  - rule: id, name, description, min_order_amount, discount_amount, can_stack_with_voucher, can_stack_with_membership, valid_from, valid_until, is_active, created_at
  - create/update: name, description, min_order_amount, discount_amount, can_stack_with_voucher, can_stack_with_membership, valid_from, valid_until

#### [weapp/miniprogram/pages/merchant/dishes/index.ts](weapp/miniprogram/pages/merchant/dishes/index.ts)
- 功能点：菜品列表/筛选/上下架/批量操作/标签
- 调用接口：
  - GET /v1/dishes (page_id, page_size, category_id, is_online, is_available)
  - PUT /v1/dishes/:id
  - PUT /v1/dishes/batch/status
  - GET /v1/dishes/categories
  - GET /v1/tags (type=dish)
- 字段对齐：
  - dish: id, name, price, member_price, image_url, category_id, category_name, is_online, is_available, prepare_time, sort_order, tags
  - batch status: dish_ids[], is_online

#### [weapp/miniprogram/pages/merchant/dishes/edit/index.ts](weapp/miniprogram/pages/merchant/dishes/edit/index.ts)
- 功能点：菜品创建/编辑、图片上传、规格/定制、标签
- 调用接口：
  - POST /v1/dishes
  - PUT /v1/dishes/:id
  - GET /v1/dishes/categories
  - GET /v1/tags (type=dish, type=customization)
  - POST /v1/tags
  - DELETE /v1/tags/:id
  - POST /v1/dishes/images/upload
- 字段对齐：
  - create/update: name, description, price, member_price, image_url, category_id, prepare_time, sort_order, is_online, is_available, tag_ids, customization_groups
  - customization_groups: id, name, is_required, sort_order, options[{ tag_id, tag_name, extra_price, sort_order }]

#### [weapp/miniprogram/pages/merchant/dishes/manage/manage.ts](weapp/miniprogram/pages/merchant/dishes/manage/manage.ts)
- 功能点：菜品快速维护（列表/编辑/上下架/删除）
- 调用接口：
  - GET /v1/dishes
  - PUT /v1/dishes/:id
  - DELETE /v1/dishes/:id
  - PUT /v1/dishes/batch/status
- 字段对齐：
  - dish: id, name, price, category_id, is_online, is_available, image_url

#### [weapp/miniprogram/pages/merchant/finance/index.ts](weapp/miniprogram/pages/merchant/finance/index.ts)
- 功能点：财务概览/日结/订单明细/服务费/促销/结算
- 调用接口：
  - GET /v1/merchant/finance/overview (start_date, end_date)
  - GET /v1/merchant/finance/daily (start_date, end_date)
  - GET /v1/merchant/finance/orders (start_date, end_date, page, limit)
  - GET /v1/merchant/finance/service-fees (start_date, end_date)
  - GET /v1/merchant/finance/promotions (start_date, end_date, page, limit)
  - GET /v1/merchant/finance/settlements (start_date, end_date, status?, page, limit)
- 字段对齐：
  - overview: completed_orders, pending_orders, total_gmv, total_income, total_platform_fee, total_operator_fee, total_service_fee, pending_income, promotion_orders, total_promotion_exp, net_income
  - daily: date, order_count, total_gmv, merchant_income, total_fee
  - orders: id, payment_order_id, order_id, order_source, total_amount, platform_fee, operator_fee, merchant_amount, status, created_at, finished_at
  - service fees: date, order_source, order_count, total_amount, platform_fee, operator_fee, total_fee
  - promotions: id, order_no, order_type, subtotal, delivery_fee, delivery_fee_discount, total_amount, created_at, completed_at
  - settlements: id, payment_order_id, order_source, total_amount, platform_fee, operator_fee, merchant_amount, sharing_order_id, status, created_at, finished_at

#### [weapp/miniprogram/pages/merchant/health/index.ts](weapp/miniprogram/pages/merchant/health/index.ts)
- 功能点：经营健康/信用分（当前已下线）
- 调用接口：无（页面直接提示下线）
- 字段对齐：N/A
- 备注：存在跳转 /pages/merchant/appeals/index（需确认是否保留）

#### [weapp/miniprogram/pages/merchant/inventory/index.ts](weapp/miniprogram/pages/merchant/inventory/index.ts)
- 功能点：菜品库存列表/分类筛选/批量保存
- 调用接口：
  - GET /v1/dishes (page_id, page_size)
  - GET /v1/dishes/categories
  - GET /v1/inventory?date=YYYY-MM-DD
  - GET /v1/inventory/stats?date=YYYY-MM-DD
  - POST /v1/inventory
  - PATCH /v1/inventory/:dish_id
- 字段对齐：
  - dish: id, name, price, image_url, category_id, category_name, is_online
  - inventory: dish_id, date, total_quantity, sold_quantity, available
  - stats: total_dishes, unlimited_dishes, sold_out_dishes, available_dishes
  - create/update: dish_id, date, total_quantity

#### [weapp/miniprogram/pages/merchant/kds/index.ts](weapp/miniprogram/pages/merchant/kds/index.ts)
- 功能点：后厨看板、实时订单、语音提醒
- 调用接口：
  - GET /v1/kitchen/orders
  - POST /v1/kitchen/orders/:id/preparing
  - POST /v1/kitchen/orders/:id/ready
  - WebSocket /v1/ws?token=...（订单实时推送）
- 字段对齐：
  - kitchen orders: new_orders/preparing_orders/ready_orders, stats
  - order: id, order_no, order_type, status, items[{ name, quantity, prepare_time, customizations }], table_no, paid_at, created_at, waiting_minutes
  - stats: new_count, preparing_count, ready_count, completed_today_count, avg_prepare_time

#### [weapp/miniprogram/pages/merchant/membership-settings/index.ts](weapp/miniprogram/pages/merchant/membership-settings/index.ts)
- 功能点：会员余额规则/充值规则
- 调用接口：
  - GET /v1/merchants/me/membership-settings
  - PUT /v1/merchants/me/membership-settings
  - GET /v1/merchants/:id/recharge-rules
  - POST /v1/merchants/:id/recharge-rules
  - PATCH /v1/merchants/:id/recharge-rules/:rule_id
  - DELETE /v1/merchants/:id/recharge-rules/:rule_id
- 字段对齐：
  - membership settings: balance_usable_scenes, bonus_usable_scenes, max_deduction_percent, allow_with_discount, allow_with_voucher
  - recharge rule: id, recharge_amount, bonus_amount, valid_from, valid_until, is_active
  - create/update: recharge_amount, bonus_amount, valid_from, valid_until, is_active

#### [weapp/miniprogram/pages/merchant/members/index.ts](weapp/miniprogram/pages/merchant/members/index.ts)
- 功能点：会员列表/详情/余额调整/交易记录
- 调用接口：
  - GET /v1/merchants/:id/members (page_id, page_size)
  - GET /v1/merchants/:id/members/:user_id
  - POST /v1/merchants/:id/members/:user_id/balance
- 字段对齐：
  - member: user_id, full_name, phone, avatar_url, membership_id, balance, total_recharged, total_consumed, created_at
  - transactions: id, membership_id, type, amount, balance_after, related_order_id, notes, created_at
  - adjust balance: amount, notes

#### [weapp/miniprogram/pages/merchant/navigation/index.ts](weapp/miniprogram/pages/merchant/navigation/index.ts)
- 功能点：模块导航入口
- 调用接口：无
- 字段对齐：N/A

#### [weapp/miniprogram/pages/merchant/orders/index.ts](weapp/miniprogram/pages/merchant/orders/index.ts)
- 功能点：订单列表/筛选/批量操作/详情/状态流转
- 调用接口：
  - GET /v1/merchant/orders (page_id, page_size, status?)
  - GET /v1/merchant/orders/:id
  - POST /v1/merchant/orders/:id/accept
  - POST /v1/merchant/orders/:id/reject (reason)
  - POST /v1/merchant/orders/:id/ready
  - POST /v1/merchant/orders/:id/complete
  - GET /v1/merchant/orders/stats (start_date, end_date)
- 字段对齐：
  - order: id, order_no, order_type, status, user_id, items[{ name, quantity, unit_price, subtotal, customizations }], subtotal, delivery_fee, delivery_fee_discount, discount_amount, total_amount, payment_method, notes, created_at, paid_at, completed_at
  - reject: reason
  - stats: total_orders, total_revenue, avg_order_value, completed_orders, cancelled_orders, completion_rate

#### [weapp/miniprogram/pages/merchant/reservations/index.ts](weapp/miniprogram/pages/merchant/reservations/index.ts)
- 功能点：预订日历/列表/统计、代客创建、确认/完成/爽约
- 调用接口：
  - GET /v1/reservations/merchant (page_id, page_size, status?, date?)
  - GET /v1/reservations/merchant/today
  - GET /v1/reservations/merchant/stats
  - POST /v1/reservations/merchant/create
  - PUT /v1/reservations/:id/update
  - POST /v1/reservations/:id/confirm
  - POST /v1/reservations/:id/complete
  - POST /v1/reservations/:id/no-show
  - POST /v1/reservations/:id/checkin
  - POST /v1/reservations/:id/start-cooking
  - GET /v1/tables
- 字段对齐：
  - reservation: id, table_id, table_no, table_type, reservation_date, reservation_time, guest_count, contact_name, contact_phone, payment_mode, deposit_amount, prepaid_amount, status, notes, created_at
  - stats: pending_count, paid_count, confirmed_count, checked_in_count, completed_count, cancelled_count, expired_count, no_show_count
  - create/update: table_id, date, time, guest_count, contact_name, contact_phone, source, notes

#### [weapp/miniprogram/pages/merchant/review/manage/index.ts](weapp/miniprogram/pages/merchant/review/manage/index.ts)
- 功能点：评价列表/筛选/回复
- 调用接口：
  - GET /v1/reviews (page_id, page_size, has_reply?)
  - POST /v1/merchant/reviews/:id/reply (content)
- 字段对齐：
  - review: id, order_id, user_name, user_avatar, rating, content, images, reply, reply_at, created_at, is_anonymous
  - reply: content

#### [weapp/miniprogram/pages/merchant/settings/index.ts](weapp/miniprogram/pages/merchant/settings/index.ts)
- 功能点：商户信息维护、Logo上传、打印机与显示配置
- 调用接口：
  - GET /v1/merchants/me
  - PATCH /v1/merchants/me
  - POST /v1/merchants/images/upload
  - GET /v1/merchant/devices
  - POST /v1/merchant/devices
  - PATCH /v1/merchant/devices/:id
  - DELETE /v1/merchant/devices/:id
  - GET /v1/merchant/display-config
  - PUT /v1/merchant/display-config
- 字段对齐：
  - merchant: id, name, description, logo_url, phone, address, latitude, longitude, is_open, version
  - update merchant: name, description, logo_url, phone, address, latitude, longitude, version
  - printer/device: id, printer_name, printer_type, printer_sn, printer_key, print_takeout, print_dine_in, print_reservation, is_active
  - display config: enable_print, print_takeout, print_dine_in, print_reservation, enable_voice, voice_takeout, voice_dine_in, enable_kds, kds_url

#### [weapp/miniprogram/pages/merchant/staff/index.ts](weapp/miniprogram/pages/merchant/staff/index.ts)
- 功能点：员工列表/新增/改角色/删除/邀请码
- 调用接口：
  - GET /v1/merchant/staff
  - POST /v1/merchant/staff (user_id, role)
  - PATCH /v1/merchant/staff/:id/role (role)
  - DELETE /v1/merchant/staff/:id
  - POST /v1/merchant/staff/invite-code
  - POST /v1/bind-merchant (扫码入职)
- 字段对齐：
  - staff: id, merchant_id, user_id, role, status, full_name, avatar_url, created_at
  - invite: invite_code, expires_at

#### [weapp/miniprogram/pages/merchant/tables/index.ts](weapp/miniprogram/pages/merchant/tables/index.ts)
- 功能点：桌台列表/创建/编辑/二维码/图片/标签
- 调用接口：
  - GET /v1/tables
  - GET /v1/tables/:id
  - POST /v1/tables
  - PATCH /v1/tables/:id
  - DELETE /v1/tables/:id
  - GET /v1/tables/:id/qrcode
  - GET /v1/tables/:id/images
  - POST /v1/tables/:id/images
  - POST /v1/tables/images/upload
  - GET /v1/tags (type=table)
  - POST /v1/tags
  - DELETE /v1/tags/:id
- 字段对齐：
  - table: id, table_no, table_type, capacity, description, minimum_spend, status, tags[{ id, name }], qr_code_url
  - image: id, image_url, is_primary, sort_order
  - create/update: table_no, table_type, capacity, description, minimum_spend, status, tag_ids

#### [weapp/miniprogram/pages/merchant/tables/manage/manage.ts](weapp/miniprogram/pages/merchant/tables/manage/manage.ts)
- 功能点：桌台快速维护（列表/新增/编辑/删/状态/二维码）
- 调用接口：
  - GET /v1/tables (page, page_size)
  - POST /v1/tables
  - PATCH /v1/tables/:id
  - DELETE /v1/tables/:id
  - PATCH /v1/tables/:id/status
  - GET /v1/tables/:id/qrcode
- 字段对齐：
  - table: id, table_no, table_type, capacity, minimum_spend, status, description, qr_code_url

#### [weapp/miniprogram/pages/merchant/vouchers/index.ts](weapp/miniprogram/pages/merchant/vouchers/index.ts)
- 功能点：代金券列表/创建/编辑/启停/删除
- 调用接口：
  - GET /v1/merchants/:id/vouchers (page_id, page_size)
  - POST /v1/merchants/:id/vouchers
  - PATCH /v1/merchants/:id/vouchers/:voucher_id
  - DELETE /v1/merchants/:id/vouchers/:voucher_id
- 字段对齐：
  - voucher: id, merchant_id, name, code, description, amount, min_order_amount, total_quantity, claimed_quantity, used_quantity, is_active, valid_from, valid_until, allowed_order_types, created_at
  - create/update: name, code?, description, amount, min_order_amount, total_quantity, valid_from, valid_until, allowed_order_types, is_active

### A1. Dashboard / Analytics（经营概览）
- [x] 经营概览面板 → `/v1/merchant/stats/overview`
- [x] 日报/趋势 → `/v1/merchant/stats/daily`, `/v1/merchant/stats/hourly`, `/v1/merchant/stats/sources`, `/v1/merchant/stats/repurchase`
- [x] 热销菜品/分类分析 → `/v1/merchant/stats/dishes/top`, `/v1/merchant/stats/categories`
- [x] 客户分析 → `/v1/merchant/stats/customers`, `/v1/merchant/stats/customers/:user_id`

### A2. Orders（订单管理）
- [x] 订单列表/筛选 → `/v1/merchant/orders`
- [x] 订单详情 → `/v1/merchant/orders/:id`
- [x] 接单/拒单/出餐/完成 → `/v1/merchant/orders/:id/accept|reject|ready|complete`
- [x] 订单统计 → `/v1/merchant/orders/stats`

### A3. Kitchen / KDS（后厨）
- [x] 厨房订单列表 → `/v1/kitchen/orders`
- [x] 厨房订单详情 → `/v1/kitchen/orders/:id`
- [x] 开始制作/出餐 → `/v1/kitchen/orders/:id/preparing|ready`

### A4. Reservations（预订/包间）
- [ ] 商户预订列表/今日预订 → `/v1/reservations/merchant`, `/v1/reservations/merchant/today`
- [ ] 预订统计 → `/v1/reservations/merchant/stats`
- [ ] 商户代客创建/修改 → `/v1/reservations/merchant/create`, `/v1/reservations/:id/update`
- [ ] 确认/完成/爽约 → `/v1/reservations/:id/confirm|complete|no-show`
- [ ] 到店签到/起菜通知 → `/v1/reservations/:id/checkin|start-cooking`

### A5. Tables / Dine-in（桌台/堂食）
- [ ] 桌台列表/详情/增删改 → `/v1/tables`
- [ ] 桌台状态 → `/v1/tables/:id/status`
- [ ] 桌台标签 → `/v1/tables/:id/tags`
- [ ] 桌台图片 → `/v1/tables/:id/images`
- [ ] 桌台二维码生成 → `/v1/tables/:id/qrcode`
- [ ] 用餐会话（开台/预检/转台） → `/v1/dining-sessions/precheck`, `/v1/dining-sessions/open`, `/v1/dining-sessions/:id/transfer-table`

### A6. Dishes / Combos（菜品/套餐）
- [ ] 菜品 CRUD → `/v1/dishes`
- [ ] 菜品分类 → `/v1/dishes/categories`
- [ ] 菜品上下架/批量 → `/v1/dishes/:id/status`, `/v1/dishes/batch/status`
- [ ] 菜品规格/定制 → `/v1/dishes/:id/customizations|specs`
- [ ] 套餐 CRUD → `/v1/combos`
- [ ] 套餐关联菜品 → `/v1/combos/:id/dishes`

### A7. Inventory（库存）
- [ ] 日库存 CRUD → `/v1/inventory`
- [ ] 单品库存 → `/v1/inventory/:dish_id`
- [ ] 库存统计 → `/v1/inventory/stats`

### A8. Discounts / Vouchers / Marketing（优惠/营销）
- [ ] 折扣规则 → `/v1/merchants/:id/discounts`
- [ ] 优惠券管理 → `/v1/merchants/:id/vouchers`
- [ ] 充值规则 → `/v1/merchants/:id/recharge-rules`
- [ ] 会员促销费用统计 → `/v1/merchant/finance/promotions`
- [ ] 配送满返/满减 → `/v1/delivery-fee/merchants/:merchant_id/promotions`

### A9. Membership / Members（会员）
- [ ] 会员设置 → `/v1/merchants/me/membership-settings`
- [ ] 商户会员列表/详情 → `/v1/merchants/:id/members`, `/v1/merchants/:id/members/:user_id`
- [ ] 会员余额调整 → `/v1/merchants/:id/members/:user_id/balance`

### A10. Reviews（评价）
- [ ] 评价列表（可见/全量） → `/v1/reviews/merchants/:id`, `/v1/reviews/merchants/:id/all`
- [ ] 评价回复 → `/v1/reviews/:id/reply`

### A11. Staff（员工）
- [ ] 员工列表 → `/v1/merchant/staff`
- [ ] 邀请码生成 → `/v1/merchant/staff/invite-code`
- [ ] 员工添加/角色/移除 → `/v1/merchant/staff` + `/v1/merchant/staff/:id/role` + `/v1/merchant/staff/:id`
- [ ] 员工扫码入职 → `/v1/bind-merchant`

### A12. Finance（财务）
- [ ] 财务概览 → `/v1/merchant/finance/overview`
- [ ] 订单明细 → `/v1/merchant/finance/orders`
- [ ] 服务费/结算 → `/v1/merchant/finance/service-fees`, `/v1/merchant/finance/settlements`
- [ ] 日结 → `/v1/merchant/finance/daily`

### A13. Devices / Settings / Navigation（设备/设置）
- [ ] 打印机/设备管理 → `/v1/merchant/devices`
- [ ] 订单展示配置 → `/v1/merchant/display-config`
- [ ] 商户基础信息 → `/v1/merchants/me`
- [ ] 营业状态/营业时间 → `/v1/merchants/me/status`, `/v1/merchants/me/business-hours`

### A14. Appeals / Risk（申诉/风控）
- [ ] 商户索赔与申诉 → `/v1/merchant/claims`, `/v1/merchant/appeals`
- [ ] 用户风控视图 → `/v1/merchant/risk/users/:id`

---

## B. 小程序商户侧缺失或待新增页面/功能（与新后端对齐）

### B1. 集团/品牌体系
- [ ] 集团入驻申请（草稿/提交/进度） → `/v1/groups/applications/*`
- [ ] 集团搜索与详情 → `/v1/groups`, `/v1/groups/:id`
- [ ] 集团门店列表 → `/v1/groups/:id/merchants`
- [ ] 品牌管理与详情 → `/v1/groups/:id/brands`, `/v1/brands/:id`
- [ ] 门店申请加入集团 → `/v1/groups/:id/join-requests`
- [ ] 集团审核门店加入 → `/v1/groups/:id/join-requests/:request_id/approve|reject|cancel`
- [ ] 集团策略设置 → `/v1/groups/:id/policies`
- [ ] 集团/品牌菜单模板 → `/v1/groups/:id/menu-templates`, `/v1/brands/:id/menu-templates`

#### B1.1 字段对齐（后端 API 结构）
- 集团入驻申请（response）
  - id, applicant_user_id, group_name, contact_phone, license_number, license_image_url, address, region_id, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at
- 入驻申请基础信息（PUT /v1/groups/applications/basic）
  - group_name, contact_phone, license_number, license_image_url, address, region_id
- 营业执照 OCR 上传（POST /v1/groups/applications/license/ocr）
  - multipart file: image|file
- 集团（response）
  - id, name, owner_user_id, status, contact_phone, license_number, license_image_url, address, region_id, created_at, updated_at
- 品牌（response）
  - id, group_id, name, logo_url, description, status, created_at, updated_at
- 集团门店（response）
  - id, name, logo_url, address, phone, status
- 加入申请（response）
  - id, group_id, merchant_id, applicant_user_id, status, reason, reviewed_by, reviewed_at, created_at
- 申请加入集团（POST /v1/groups/:id/join-requests）
  - reason?
- 审核通过（POST /v1/groups/:id/join-requests/:request_id/approve）
  - brand_id?
- 驳回（POST /v1/groups/:id/join-requests/:request_id/reject）
  - reason?
- 撤回（POST /v1/groups/:id/join-requests/:request_id/cancel）
  - 无 body
- 集团策略（PUT /v1/groups/:id/policies）
  - pricing_mode, menu_mode, inventory_mode, promotion_mode
- 菜单模板（POST /v1/groups/:id/menu-templates | /v1/brands/:id/menu-templates）
  - payload, version?, status?
  - response: id, group_id/brand_id, version, status, created_at, updated_at

### B2. 转台能力（新后端功能）
- [ ] 转台页面与流程 → `/v1/dining-sessions/:id/transfer-table`

#### B2.1 字段对齐（后端 API 结构）
- 转台请求（POST /v1/dining-sessions/:id/transfer-table）
  - to_table_id, table_code?, reason?
- 转台响应
  - session: id, merchant_id, table_id, reservation_id?, user_id, active_order_id?, status, opened_at, closed_at?, created_at, updated_at?
  - from_table / to_table: id, table_no, table_type, capacity, description?, minimum_spend?, status, tags?, current_reservation_id?

---

## C. Web 端新增页面/功能（替代小程序商户侧）

### C1. Web 登录扫码页
- [ ] 登录二维码展示（Web）
- [ ] 登录状态轮询/回调处理
- [ ] 登录成功后切换商户/角色选择

### C2. 员工邀请二维码（Web 生成）
- [ ] Web 端生成邀请码二维码（二维码内容为小程序页面路径）

---

## D. 后端需新增/调整接口清单

### D1. Web 登录扫码流程（新增）
> Web 端不走 `code -> openid`，由小程序扫码确认后签发 token。
- [x] 生成登录会话/登录码
  - `POST /v1/auth/web-login/sessions`
- [x] 查询登录会话状态
  - `GET /v1/auth/web-login/sessions/:code`
- [x] 小程序确认扫码登录（携带小程序已登录 token）
  - `POST /v1/auth/web-login/confirm`
- [x] Web 端用 `code` 兑换 token（一次性）
  - `POST /v1/auth/web-login/consume`
> 已落地：新增 web_login_sessions 表、sqlc 查询、API 与路由；配置 `WEB_LOGIN_SESSION_TTL`。

### D2. 数据库清理
- [x] 删除 `boss_bind_code` 及 `boss_bind_code_expires_at`（迁移 + 代码清理）
> 已落地：迁移 000102_drop_boss_bind_code + sqlc 查询清理。

---

## E. 执行顺序建议（可勾选）
1. [ ] 以 swagger 为准，逐项校验字段/类型（先对齐集团/品牌与转台）
2. [ ] 完成 Web 登录扫码接口设计与后端实现
3. [ ] 完成 Web 商户侧页面全量替换（按模块逐项迁移）
4. [ ] 清理废弃字段 `boss_bind_code`

---

## Z. 实施现状（需回顾与纠偏）
> 说明：以下为已生成的 Web 骨架与接口接入情况，尚未对齐小程序页面布局与字段类型，需回顾后逐项修正。

- [x] A1 Dashboard / Analytics：已按小程序布局重构（日期栏 + Tab + 分区），并按 swagger 类型对齐字段展示与金额格式。
- [x] A2 Orders：已按小程序桌面 SaaS 布局对齐（桌面提示/统计卡/筛选/表格/抽屉），字段与类型与后端对齐。
- [x] A3 Kitchen / KDS：已按小程序全屏三栏看板布局重构，字段与类型与后端对齐；语音/实时推送暂以开关占位。
- [ ] A4 Reservations：已生成列表/详情/操作并接入接口，但**代客创建/完整字段校验未对齐**，布局未对齐小程序。
- [ ] A5 Tables / Dine-in：已生成桌台/堂食页并接入接口，但**标签/图片/二维码等流程未完整实现**，布局未对齐小程序。

---

如需，我可以把每个模块再拆分到具体页面（路由级）与字段级对齐表。