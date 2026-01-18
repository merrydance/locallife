# 后端代码审查任务清单（从架构到接口链路）

> 目标：从总体架构 -> 接口分组 -> 每个接口链路（handler -> store/db/sqlc）逐一审查。
> 勾选规则：完成一个模块或一组接口的链路审查后打勾。

## 0. 总体架构与基础设施
- [x] 入口与启动流程（main.go、依赖注入、配置、任务/调度）
- [x] HTTP/中间件链（api/server.go 与中间件）
- [x] 数据层组织（db/sqlc、事务封装、错误处理）

## 1. 认证与用户体系
- [x] 认证与登录（/v1/auth、token、wechat）
- [x] 用户资料与绑定（/v1/users、/v1/auth/bind-phone）
- [x] 用户地址（/v1/addresses）

## 2. 基础能力
- [x] 地区与地理（/v1/regions、/v1/location）
- [x] 搜索与扫码（/v1/search、/v1/scan）
- [x] 公共信息（/v1/public/*）

## 3. 商户体系
- [x] 商户基础信息（/v1/merchants）
- [x] 商户入驻申请（/v1/merchant/application）
- [x] 商户进件（/v1/merchant/applyment）
- [x] 员工与角色（/v1/merchant/staff、/bind-merchant）
- [x] 店主与认领（/claim-boss、/boss、/merchant/bosses）
- [x] 管理员审核（/admin/merchants/*）

## 4. 商品与库存
- [x] 标签（/v1/tags）
- [x] 菜品与分类（/v1/dishes）
- [x] 套餐（/v1/combos）
- [x] 库存（/v1/inventory）

## 5. 桌台/包间/预订
- [x] 桌台管理（/v1/tables）
- [x] 包间与可用性（/v1/rooms、/v1/merchants/:id/rooms）
- [x] 预订与用餐会话（/v1/reservations、/v1/dining-sessions）
- [x] 账单组（/v1/billing-groups）

## 6. 订单与支付
- [x] 订单（/v1/orders、/v1/merchant/orders、/v1/kitchen）
- [x] 支付与退款（/v1/payments、/v1/refunds、/v1/webhooks）

## 7. 配送与骑手
- [x] 骑手申请/押金/状态（/v1/rider）
- [x] 配送链路（/v1/delivery、/v1/admin/riders）
- [x] 配送费配置与计算（/v1/delivery-fee）

## 8. 用户侧功能
- [x] 购物车（/v1/cart）
- [x] 收藏与浏览（/v1/favorites、/v1/history）
- [x] 评价（/v1/reviews）
- [x] 申诉/索赔（/v1/claims、/v1/appeals）

## 9. 会员/营销/推荐
- [x] 会员（/v1/memberships）
- [x] 充值规则（/v1/merchants/:id/recharge-rules）
- [x] 优惠券（/v1/merchants/:id/vouchers、/v1/vouchers）
- [x] 折扣（/v1/merchants/:id/discounts）
- [x] 推荐与埋点（/v1/behaviors、/v1/recommendations）
- [x] 推荐配置（/v1/regions/:id/recommendation-config）

## 10. 运营与统计
- [x] 通知与WebSocket（/v1/notifications、/v1/ws）
- [x] 商户统计/财务/设备/展示（/v1/merchant/*）
- [x] 运营商统计与财务（/v1/operator、/v1/operators/me）
- [x] 平台统计（/v1/platform/stats）
- [x] 行为追溯风控（/v1/claims、/v1/food-safety、/v1/fraud）

## 11. 汇总审查文档
- [ ] 产出总体审查报告（用途、状态机、健壮性、完成度、矛盾/冗余/缺失）
