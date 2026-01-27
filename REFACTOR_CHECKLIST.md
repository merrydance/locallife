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
- [ ] `pages/register/rider/index`

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

- [ ] `pages/dining/index`
- [ ] `pages/dine-in/scan-entry/scan-entry`
- [ ] `pages/dine-in/menu/menu`
- [ ] `pages/dine-in/checkout/checkout`
- [ ] `pages/dine-in/payment-success/payment-success`
- [ ] `pages/reservation/index`
- [ ] `pages/reservation/list/index`
- [ ] `pages/reservation/create/index`
- [ ] `pages/reservation/detail/index`
- [ ] `pages/reservation/modify/index`
- [ ] `pages/reservation/room-detail/index`
- [ ] `pages/reservation/confirm/index`

### 📦 订单管理 (Orders)

- [ ] `pages/orders/list/index`
- [ ] `pages/orders/detail/index`
- [ ] `pages/orders/tracking/index`

### 👤 个人中心与通用 (User & General)

- [ ] `pages/user_center/index`
- [ ] `pages/user_center/wallet/index`
- [ ] `pages/user_center/addresses/index`
- [ ] `pages/user_center/addresses/edit/index`
- [ ] `pages/user_center/coupons/index`
- [ ] `pages/user_center/favorites/index`
- [ ] `pages/user_center/membership/index`
- [ ] `pages/user_center/payment-detail/index`
- [ ] `pages/user_center/refund-detail/index`
- [ ] `pages/user_center/reservations/index`
- [ ] `pages/user_center/reviews/index`
- [ ] `pages/user_center/reviews/create/index`
- [ ] `pages/user/bind-merchant/index`
- [ ] `pages/favorite/index`
- [ ] `pages/message/center/index`
- [ ] `pages/coupon/center/index`
- [ ] `pages/coupon/my/index`
- [ ] `pages/support/index`

### 🏢 商家、运营与平台 (B-Side & Admin)

- [ ] `pages/operator/dashboard/index`
- [ ] `pages/operator/dashboard/dashboard`
- [ ] `pages/operator/analytics/index`
- [ ] `pages/operator/appeal/list/index`
- [ ] `pages/operator/appeal/detail/index`
- [ ] `pages/operator/automation/index`
- [ ] `pages/operator/claims/index`
- [ ] `pages/operator/delivery-fee/index`
- [ ] `pages/operator/merchants/index`
- [ ] `pages/operator/merchants/list/list`
- [ ] `pages/operator/region/index`
- [ ] `pages/operator/region/config`
- [ ] `pages/operator/rules/index`
- [ ] `pages/operator/stats/index`
- [ ] `pages/register/merchant/index`
- [ ] `pages/register/merchant/group/index`
- [ ] `pages/register/merchant/join-group/index`
- [ ] `pages/register/merchant/store/index`
- [ ] `pages/register/operator/index`
- [ ] `pages/platform/dashboard/dashboard`

---

_注：已剔除 templates 子目录等非独立页面内容，勾选表示已完成代码重构。_
