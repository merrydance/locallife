# 后端代码审查任务清单（2026-01-18）

> 目标：从总体架构 -> 接口分组 -> 每个接口链路（handler -> store/db/sqlc）逐一审查。
> 勾选规则：完成一个接口链路审查后打勾。
> 注意：已移除接口不纳入清单（见 docs/deprecations.md）。

## 0. 总体架构与基础设施
- [x] 入口与启动流程（main.go、依赖注入、配置、任务/调度）
- [x] HTTP/中间件链（api/server.go 与中间件）
- [x] 数据层组织（db/sqlc、事务封装、错误处理）

## 1. 认证与用户体系
- [x] POST /v1/auth/wechat-login
- [x] POST /v1/auth/refresh
- [x] GET /v1/role-access
- [x] GET /v1/users/me
- [x] PATCH /v1/users/me
- [x] POST /v1/auth/bind-phone

## 2. 用户地址与位置服务
- [x] POST /v1/addresses
- [x] GET /v1/addresses
- [x] GET /v1/addresses/:id
- [x] PATCH /v1/addresses/:id
- [x] PATCH /v1/addresses/:id/default
- [x] DELETE /v1/addresses/:id
- [x] GET /v1/location/reverse-geocode
- [x] GET /v1/location/direction/bicycling

## 3. 地区与地理
- [x] GET /v1/regions/available
- [x] GET /v1/regions/:id/check
- [x] GET /v1/regions/:id
- [x] GET /v1/regions
- [x] GET /v1/regions/:id/children
- [x] GET /v1/regions/search

## 4. 搜索与扫码
- [x] GET /v1/search/dishes
- [x] GET /v1/search/merchants
- [x] GET /v1/search/combos
- [x] GET /v1/search/rooms
- [x] GET /v1/scan/table

## 5. 公共信息（用户侧）
- [x] GET /v1/public/dishes/:id
- [x] GET /v1/public/merchants/:id
- [x] GET /v1/public/merchants/:id/dishes
- [x] GET /v1/public/merchants/:id/combos
- [x] GET /v1/public/merchants/:id/rooms

## 6. 商户体系
### 6.1 商户基础信息
- [x] POST /v1/merchants/images/upload
- [x] GET /v1/merchants/me
- [x] GET /v1/merchants/my
- [x] PATCH /v1/merchants/me
- [x] GET /v1/merchants/me/status
- [x] PATCH /v1/merchants/me/status
- [x] GET /v1/merchants/me/business-hours
- [x] PUT /v1/merchants/me/business-hours
- [x] GET /v1/merchants/me/membership-settings
- [x] PUT /v1/merchants/me/membership-settings
- [x] GET /v1/merchants/:id/promotions

### 6.2 商户入驻申请（新版）
- [x] GET /v1/merchant/application
- [x] PUT /v1/merchant/application/basic
- [x] PUT /v1/merchant/application/images
- [x] POST /v1/merchant/application/license/ocr
- [x] POST /v1/merchant/application/foodpermit/ocr
- [x] POST /v1/merchant/application/idcard/ocr
- [x] POST /v1/merchant/application/submit
- [x] POST /v1/merchant/application/reset

### 6.3 商户进件（微信二级商户）
- [x] POST /v1/merchant/applyment/bindbank
- [x] GET /v1/merchant/applyment/status

### 6.4 员工与角色
- [x] POST /v1/bind-merchant
- [x] GET /v1/merchant/staff
- [x] POST /v1/merchant/staff/invite-code
- [x] POST /v1/merchant/staff
- [x] PATCH /v1/merchant/staff/:id/role
- [x] DELETE /v1/merchant/staff/:id

### 6.5 店主与认领
- [x] POST /v1/claim-boss
- [x] GET /v1/boss/merchants
- [x] POST /v1/merchant/boss-bind-code
- [x] GET /v1/merchant/bosses
- [x] DELETE /v1/merchant/bosses/:id

## 7. 标签、菜品、套餐、库存
### 7.1 标签
- [x] GET /v1/tags
- [x] POST /v1/tags

### 7.2 菜品
- [x] POST /v1/dishes/images/upload
- [x] POST /v1/dishes/categories
- [x] GET /v1/dishes/categories
- [x] PATCH /v1/dishes/categories/:id
- [x] DELETE /v1/dishes/categories/:id
- [x] POST /v1/dishes
- [x] GET /v1/dishes
- [x] GET /v1/dishes/:id
- [x] PUT /v1/dishes/:id
- [x] DELETE /v1/dishes/:id
- [x] PATCH /v1/dishes/:id/status
- [x] PATCH /v1/dishes/batch/status
- [x] GET /v1/dishes/:id/customizations
- [x] PUT /v1/dishes/:id/customizations
- [x] PUT /v1/dishes/:id/specs

### 7.3 套餐
- [x] POST /v1/combos
- [x] GET /v1/combos
- [x] GET /v1/combos/:id
- [x] PUT /v1/combos/:id
- [x] PUT /v1/combos/:id/online
- [x] DELETE /v1/combos/:id
- [x] POST /v1/combos/:id/dishes
- [x] DELETE /v1/combos/:id/dishes/:dish_id

### 7.4 库存
- [x] POST /v1/inventory
- [x] GET /v1/inventory
- [x] PUT /v1/inventory
- [x] PATCH /v1/inventory/:dish_id
- [x] POST /v1/inventory/check
- [x] GET /v1/inventory/stats

## 8. 桌台/包间/预订
### 8.1 桌台管理
- [x] POST /v1/tables/images/upload
- [x] POST /v1/tables
- [x] GET /v1/tables/:id
- [x] GET /v1/tables
- [x] PATCH /v1/tables/:id
- [x] PATCH /v1/tables/:id/status
- [x] DELETE /v1/tables/:id
- [x] POST /v1/tables/:id/tags
- [x] DELETE /v1/tables/:id/tags/:tag_id
- [x] GET /v1/tables/:id/tags
- [x] POST /v1/tables/:id/images
- [x] GET /v1/tables/:id/images
- [x] PUT /v1/tables/:id/images/:image_id/primary
- [x] DELETE /v1/tables/:id/images/:image_id
- [x] GET /v1/tables/:id/qrcode

### 8.2 包间与可用性
- [x] GET /v1/merchants/:id/rooms
- [x] GET /v1/merchants/:id/rooms/all
- [x] GET /v1/rooms/:id
- [x] GET /v1/rooms/:id/availability

### 8.3 预订
- [x] POST /v1/reservations
- [x] GET /v1/reservations/me
- [x] GET /v1/reservations/:id
- [x] POST /v1/reservations/:id/cancel
- [x] POST /v1/reservations/:id/add-dishes
- [x] POST /v1/reservations/:id/modify-dishes
- [x] POST /v1/reservations/:id/checkin
- [x] POST /v1/reservations/:id/start-cooking
- [x] GET /v1/reservations/merchant
- [x] GET /v1/reservations/merchant/today
- [x] GET /v1/reservations/merchant/stats
- [x] POST /v1/reservations/merchant/create
- [x] PUT /v1/reservations/:id/update
- [x] POST /v1/reservations/:id/confirm
- [x] POST /v1/reservations/:id/complete
- [x] POST /v1/reservations/:id/no-show

### 8.4 用餐会话
- [x] GET /v1/dining-sessions/precheck
- [x] POST /v1/dining-sessions/open

### 8.5 账单组
- [x] POST /v1/billing-groups
- [x] GET /v1/billing-groups
- [x] POST /v1/billing-groups/:id/join
- [x] GET /v1/billing-groups/:id/orders

## 9. 订单与支付
### 9.1 用户订单
- [x] GET /v1/orders/calculate
- [x] POST /v1/orders
- [x] GET /v1/orders
- [x] GET /v1/orders/:id
- [x] POST /v1/orders/:id/cancel
- [x] POST /v1/orders/:id/replace
- [x] POST /v1/orders/:id/urge
- [x] POST /v1/orders/:id/confirm

### 9.2 商户订单
- [x] GET /v1/merchant/orders
- [x] GET /v1/merchant/orders/:id
- [x] POST /v1/merchant/orders/:id/accept
- [x] POST /v1/merchant/orders/:id/reject
- [x] POST /v1/merchant/orders/:id/ready
- [x] POST /v1/merchant/orders/:id/complete
- [x] GET /v1/merchant/orders/stats

### 9.3 厨房显示系统
- [x] GET /v1/kitchen/orders
- [x] GET /v1/kitchen/orders/:id
- [x] POST /v1/kitchen/orders/:id/preparing
- [x] POST /v1/kitchen/orders/:id/ready

### 9.4 支付订单
- [x] POST /v1/payments
- [x] GET /v1/payments
- [x] GET /v1/payments/:id
- [x] POST /v1/payments/:id/close
- [x] GET /v1/payments/:id/refunds

### 9.5 退款订单
- [x] POST /v1/refunds
- [x] GET /v1/refunds/:id

### 9.6 微信支付回调
- [x] POST /v1/webhooks/wechat-pay/notify
- [x] POST /v1/webhooks/wechat-pay/refund-notify
- [x] POST /v1/webhooks/wechat-ecommerce/notify
- [x] POST /v1/webhooks/wechat-ecommerce/refund-notify
- [x] POST /v1/webhooks/wechat-ecommerce/applyment-notify
- [x] POST /v1/webhooks/wechat-ecommerce/profit-sharing-notify

## 10. 配送与骑手
### 10.1 骑手申请与开户
- [x] GET /v1/rider/application
- [x] PUT /v1/rider/application/basic
- [x] POST /v1/rider/application/idcard/ocr
- [x] POST /v1/rider/application/healthcert
- [x] POST /v1/rider/application/submit
- [x] POST /v1/rider/application/reset
- [x] POST /v1/rider/applyment/bindbank
- [x] GET /v1/rider/applyment/status

### 10.2 骑手资料与押金
- [x] GET /v1/rider/me
- [x] GET /v1/rider/deposit
- [x] POST /v1/rider/deposit
- [x] POST /v1/rider/withdraw
- [x] GET /v1/rider/deposits

### 10.3 骑手状态与位置
- [x] GET /v1/rider/status
- [x] POST /v1/rider/online
- [x] POST /v1/rider/offline
- [x] POST /v1/rider/location

### 10.4 骑手订单操作
- [x] POST /v1/rider/orders/:id/delay
- [x] POST /v1/rider/orders/:id/exception

### 10.5 高值单资格积分
- [x] GET /v1/rider/score
- [x] GET /v1/rider/score/history

### 10.6 骑手索赔与申诉
- [x] GET /v1/rider/claims
- [x] GET /v1/rider/claims/:id
- [x] POST /v1/rider/appeals
- [x] GET /v1/rider/appeals
- [x] GET /v1/rider/appeals/:id

### 10.7 配送管理
- [x] GET /v1/delivery/recommend
- [x] POST /v1/delivery/grab/:order_id
- [x] GET /v1/delivery/active
- [x] GET /v1/delivery/history
- [x] POST /v1/delivery/:delivery_id/start-pickup
- [x] POST /v1/delivery/:delivery_id/confirm-pickup
- [x] POST /v1/delivery/:delivery_id/start-delivery
- [x] POST /v1/delivery/:delivery_id/confirm-delivery
- [x] GET /v1/delivery/order/:order_id
- [x] GET /v1/delivery/:delivery_id/track
- [x] GET /v1/delivery/:delivery_id/rider-location

### 10.8 骑手审核（运营商/管理员）
- [x] GET /v1/admin/riders
- [x] POST /v1/admin/riders/:rider_id/approve
- [x] POST /v1/admin/riders/:rider_id/reject

## 11. 用户侧功能
### 11.1 购物车
- [x] GET /v1/cart
- [x] GET /v1/cart/summary
- [x] POST /v1/cart/combined-checkout/preview
- [x] POST /v1/cart/items
- [x] PATCH /v1/cart/items/:id
- [x] DELETE /v1/cart/items/:id
- [x] POST /v1/cart/clear
- [x] POST /v1/cart/calculate

### 11.2 收藏与浏览
- [x] POST /v1/favorites/merchants
- [x] GET /v1/favorites/merchants
- [x] DELETE /v1/favorites/merchants/:id
- [x] POST /v1/favorites/dishes
- [x] GET /v1/favorites/dishes
- [x] DELETE /v1/favorites/dishes/:id
- [x] GET /v1/history/browse

### 11.3 索赔（用户）
- [x] GET /v1/claims
- [x] GET /v1/claims/:id

## 12. 评价与通知
### 12.1 评价
- [x] POST /v1/reviews/images/upload
- [x] POST /v1/reviews
- [x] GET /v1/reviews/:id
- [x] GET /v1/reviews/me
- [x] GET /v1/reviews/merchants/:id
- [x] GET /v1/reviews/merchants/:id/all
- [x] POST /v1/reviews/:id/reply
- [x] DELETE /v1/reviews/:id

### 12.2 通知与 WebSocket
- [x] GET /v1/notifications
- [x] GET /v1/notifications/unread/count
- [x] PUT /v1/notifications/:id/read
- [x] PUT /v1/notifications/read-all
- [x] DELETE /v1/notifications/:id
- [x] GET /v1/notifications/preferences
- [x] PUT /v1/notifications/preferences
- [x] GET /v1/ws
- [x] GET /v1/platform/ws

## 13. 会员/营销/推荐
### 13.1 会员
- [x] POST /v1/memberships
- [x] POST /v1/memberships/recharge
- [x] GET /v1/memberships
- [x] GET /v1/memberships/:id
- [x] GET /v1/memberships/:id/transactions

### 13.2 充值规则
- [x] POST /v1/merchants/:id/recharge-rules
- [x] GET /v1/merchants/:id/recharge-rules
- [x] GET /v1/merchants/:id/recharge-rules/active
- [x] PATCH /v1/merchants/:id/recharge-rules/:rule_id
- [x] DELETE /v1/merchants/:id/recharge-rules/:rule_id

### 13.3 优惠券
- [x] POST /v1/merchants/:id/vouchers
- [x] GET /v1/merchants/:id/vouchers
- [x] GET /v1/merchants/:id/vouchers/active
- [x] PATCH /v1/merchants/:id/vouchers/:voucher_id
- [x] DELETE /v1/merchants/:id/vouchers/:voucher_id
- [x] POST /v1/vouchers/:voucher_id/claim
- [x] GET /v1/vouchers/me
- [x] GET /v1/vouchers/available/:merchant_id
- [x] GET /v1/vouchers/available

### 13.4 折扣
- [x] POST /v1/merchants/:id/discounts
- [x] GET /v1/merchants/:id/discounts
- [x] GET /v1/merchants/:id/discounts/active
- [x] GET /v1/merchants/:id/discounts/:id
- [x] PATCH /v1/merchants/:id/discounts/:id
- [x] DELETE /v1/merchants/:id/discounts/:id
- [x] GET /v1/merchants/:id/discounts/applicable
- [x] GET /v1/merchants/:id/discounts/best

### 13.5 推荐与埋点
- [x] POST /v1/behaviors/track
- [x] GET /v1/recommendations/dishes
- [x] GET /v1/recommendations/combos
- [x] GET /v1/recommendations/merchants
- [x] GET /v1/recommendations/rooms
- [x] PATCH /v1/regions/:id/recommendation-config
- [x] GET /v1/regions/:id/recommendation-config

## 14. 运营与统计
### 14.1 商户统计与财务
- [x] GET /v1/merchant/stats/daily
- [x] GET /v1/merchant/stats/overview
- [x] GET /v1/merchant/stats/dishes/top
- [x] GET /v1/merchant/stats/customers
- [x] GET /v1/merchant/stats/customers/:user_id
- [x] GET /v1/merchant/stats/hourly
- [x] GET /v1/merchant/stats/sources
- [x] GET /v1/merchant/stats/repurchase
- [x] GET /v1/merchant/stats/categories
- [x] GET /v1/merchant/finance/overview
- [x] GET /v1/merchant/finance/orders
- [x] GET /v1/merchant/finance/service-fees
- [x] GET /v1/merchant/finance/promotions
- [x] GET /v1/merchant/finance/daily
- [x] GET /v1/merchant/finance/settlements
- [x] POST /v1/merchant/devices
- [x] GET /v1/merchant/devices
- [x] GET /v1/merchant/devices/:id
- [x] PUT /v1/merchant/devices/:id
- [x] DELETE /v1/merchant/devices/:id
- [x] POST /v1/merchant/devices/:id/test
- [x] GET /v1/merchant/display-config
- [x] PUT /v1/merchant/display-config

### 14.2 运营商统计与财务
- [x] GET /v1/operator/regions/:region_id/stats
- [x] POST /v1/operator/regions/:region_id/peak-hours
- [x] GET /v1/operator/regions/:region_id/peak-hours
- [x] GET /v1/operator/merchants/ranking
- [x] GET /v1/operator/riders/ranking
- [x] GET /v1/operator/trend/daily
- [x] DELETE /v1/operator/peak-hours/:id
- [x] GET /v1/operator/merchants
- [x] GET /v1/operator/merchants/:id
- [x] POST /v1/operator/merchants/:id/suspend
- [x] POST /v1/operator/merchants/:id/resume
- [x] GET /v1/operator/riders
- [x] GET /v1/operator/riders/:id
- [x] POST /v1/operator/riders/:id/suspend
- [x] POST /v1/operator/riders/:id/resume
- [x] GET /v1/operator/appeals
- [x] GET /v1/operator/appeals/:id
- [x] POST /v1/operator/appeals/:id/review
- [x] GET /v1/operators/me/finance/overview
- [x] GET /v1/operators/me/commission

### 14.3 平台统计
- [x] GET /v1/platform/stats/overview
- [x] GET /v1/platform/stats/daily
- [x] GET /v1/platform/stats/regions/compare
- [x] GET /v1/platform/stats/merchants/ranking
- [x] GET /v1/platform/stats/categories
- [x] GET /v1/platform/stats/growth/users
- [x] GET /v1/platform/stats/growth/merchants
- [x] GET /v1/platform/stats/riders/ranking
- [x] GET /v1/platform/stats/hourly
- [x] GET /v1/platform/stats/realtime

## 15. 风控与信任分
- [x] POST /v1/claims
- [x] PATCH /v1/claims/:id/review
- [x] POST /v1/food-safety/report
- [x] PATCH /v1/food-safety/merchants/:id/suspend
- [x] POST /v1/fraud/detect

## 16. 运营商入驻申请
- [x] POST /v1/operator/application
- [x] GET /v1/operator/application
- [x] PUT /v1/operator/application/region
- [x] PUT /v1/operator/application/basic
- [x] POST /v1/operator/application/license/ocr
- [x] POST /v1/operator/application/idcard/ocr
- [x] POST /v1/operator/application/submit
- [x] POST /v1/operator/application/reset

## 17. 运营商进件（微信二级商户）
- [x] POST /v1/operator/applyment/bindbank
- [x] GET /v1/operator/applyment/status

## 18. 配送费配置
- [x] POST /v1/delivery-fee/regions/:region_id/config
- [x] PATCH /v1/delivery-fee/regions/:region_id/config
- [x] GET /v1/delivery-fee/regions/:region_id/config
- [x] POST /v1/delivery-fee/merchants/:merchant_id/promotions
- [x] GET /v1/delivery-fee/merchants/:merchant_id/promotions
- [x] DELETE /v1/delivery-fee/merchants/:merchant_id/promotions/:id
- [x] POST /v1/delivery-fee/calculate

## 19. 文件与上传
- [x] GET /uploads/*filepath
- [x] POST /v1/uploads/sign

## 20. 健康与监控
- [x] GET /health
- [x] GET /ready
- [x] GET /metrics
