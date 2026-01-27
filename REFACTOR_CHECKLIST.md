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
- [ ] `pages/register/rider/index`

### 🍔 外卖业务 (Takeout)

- [ ] `pages/takeout/index`
- [ ] `pages/takeout/restaurant-detail/index`
- [ ] `pages/takeout/dish-detail/index`
- [ ] `pages/takeout/combo-detail/index`
- [ ] `pages/takeout/cart/index`
- [ ] `pages/takeout/order-confirm/index`
- [ ] `pages/takeout/merchant-info/index`

### 🛋️ 堂食与预订 (Dining & Reservation)

- [ ] `pages/dining/index`
- [ ] `pages/dine-in/scan-entry/scan-entry`
- [ ] `pages/dine-in/menu/menu`
- [ ] `pages/dine-in/checkout/checkout`
- [ ] `pages/reservation/index`
- [ ] `pages/reservation/list/index`
- [ ] `pages/reservation/create/index`
- [ ] `pages/reservation/detail/index`
- [ ] `pages/reservation/room-detail/index`

### 📦 订单管理 (Orders)

- [ ] `pages/orders/list/index`
- [ ] `pages/orders/detail/index`
- [ ] `pages/orders/tracking/index`

### 👤 个人中心 (User Center)

- [ ] `pages/user_center/index`
- [ ] `pages/user_center/wallet/index`
- [ ] `pages/user_center/addresses/index`
- [ ] `pages/user_center/coupons/index`
- [ ] `pages/user_center/reviews/index`
- [ ] `pages/user_center/membership/index`

### 🏢 商家与运营 (Merchant & Operator)

- [ ] `pages/operator/dashboard/index`
- [ ] `pages/operator/merchants/index`
- [ ] `pages/operator/analytics/index`
- [ ] `pages/register/merchant/index`

---

_注：勾选表示已按照最新规范完成代码重构与验证。_
