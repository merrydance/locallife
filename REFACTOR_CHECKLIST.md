# LocalLife 交互架构规范化重构清单 (UI Refactor Checklist)

依据 `DESIGN_SYSTEM.md` 的交互架构规范，对所有页面和组件进行重构，确保实现：

1. **App Shell (应用外壳)**：结构不依赖数据加载。
2. **Skeleton Screen (骨架屏)**：列表加载时使用呼吸感占位。
3. **Four States (四态完备)**：加载中、正常、空数据、异常。

---

## 🏗️ 核心组件 (Components)

- [ ] `components/auth-image`
- [ ] `components/card-skeleton`
- [ ] `components/cart-bar`
- [ ] `components/category-tabs`
- [ ] `components/custom-navbar`
- [ ] `components/delivery-card`
- [ ] `components/delivery-map`
- [ ] `components/delivery-task-card`
- [ ] `components/dish-card`
- [ ] `components/dish-skeleton`
- [ ] `components/document-uploader`
- [ ] `components/info-row`
- [ ] `components/list-skeleton`
- [ ] `components/ll-stats-card`
- [ ] `components/map-view`
- [ ] `components/merchant-promos`
- [ ] `components/package-card`
- [ ] `components/recharge-promo`
- [ ] `components/remark-input`
- [ ] `components/restaurant-card`
- [ ] `components/review-card`
- [ ] `components/room-card`
- [ ] `components/search-bar`
- [ ] `components/search-filter`
- [ ] `components/virtual-list`

---

## 📱 业务页面 (Pages)

### 🛵 骑手端 (Rider)

- [x] `pages/rider/dashboard/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/rider/task-detail/index` (已完成 App Shell 与四态完备性重构)
- [ ] `pages/rider/exception/index`
- [ ] `pages/rider/claims/index`
- [ ] `pages/rider/credit/index`
- [ ] `pages/rider/deposit/index`
- [ ] `pages/rider/tasks/index`
- [ ] `pages/rider/delivery/manage/manage`
- [x] `pages/register/rider/index` (已完成 App Shell)

### 🍔 外卖业务 (Takeout)

- [x] `pages/takeout/index` (已完成 App Shell、骨架屏与四态完备重构)
- [x] `pages/takeout/restaurant-detail/index` (已完成 App
      Shell、侧边栏骨架屏与错误重试重构)
- [x] `pages/takeout/dish-detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/takeout/combo-detail/index` (已完成 App Shell 与拼图骨架屏重构)
- [x] `pages/takeout/cart/index` (已完成分组列表骨架屏重构)
- [x] `pages/takeout/order-confirm/index` (已完成地址与商户卡片骨架屏重构)
- [x] `pages/takeout/merchant-info/index` (已完成资质信息骨架屏重构)

### 🛋️ 堂食与预订 (Dining & Reservation)

- [x] `pages/dining/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/dine-in/scan-entry/scan-entry` (已完成 App Shell 与骨架屏重构)
- [x] `pages/dine-in/menu/menu` (已完成 App Shell 与骨架屏重构)
- [x] `pages/dine-in/checkout/checkout` (已完成 App Shell 与骨架屏重构)
- [x] `pages/dine-in/payment-success/payment-success` (已完成 App Shell 重构)
- [x] `pages/reservation/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/reservation/list/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/reservation/create/index` (已完成 App Shell 重构)
- [x] `pages/reservation/detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/reservation/modify/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/reservation/room-detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/reservation/confirm/index` (已完成 App Shell 与骨架屏重构)

### 📦 订单管理 (Orders)

- [x] `pages/orders/list/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/orders/detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/orders/tracking/index` (已完成订单追踪地图骨架屏重构)

### 👤 个人中心与通用 (User & General)

- [x] `pages/user_center/index` (已完成 App Shell、骨架屏重构及工作台样式优化)
- [x] `pages/user_center/wallet/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/addresses/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/addresses/edit/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/coupons/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/favorites/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/membership/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/payment-detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/refund-detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/reservations/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/reviews/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user_center/reviews/create/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/user/bind-merchant/index` (已完成 App Shell 与 TDesign 重构)
- [x] `pages/favorite/index` (已移除，映射至 `user_center/favorites`)
- [x] `pages/message/center/index` (已移除，暂无消息中心，采用系统通知/弹窗)
- [x] `pages/coupon/center/index` (已移除，映射至 `user_center/coupons`)
- [x] `pages/coupon/my/index` (已移除，映射至 `user_center/coupons`)
- [x] `pages/support/index` (已移除，当前使用一键拨号客服)

### 🏢 商家、运营与平台 (B-Side & Admin)

- [x] `pages/operator/dashboard/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/dashboard/dashboard` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/analytics/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/appeal/list/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/appeal/detail/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/automation/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/claims/index` (已移除，功能合并至 appeal)
- [x] `pages/operator/delivery-fee/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/merchants/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/merchants/list/list` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/region/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/region/config` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/rules/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/operator/stats/index` (已移除，非独立页面)
- [x] `pages/register/merchant/index`
- [x] `pages/register/merchant/group/index` (已完成 App Shell)
- [x] `pages/register/merchant/join-group/index` (已完成 App Shell 与骨架屏重构)
- [x] `pages/register/merchant/store/index` (已完成 App Shell)
- [x] `pages/register/operator/index` (已完成 App Shell)
- [x] `pages/platform/dashboard/dashboard` (已完成 App Shell 与骨架屏重构)

---

_注：已剔除 templates 子目录等非独立页面内容，勾选表示已完成代码重构。_
