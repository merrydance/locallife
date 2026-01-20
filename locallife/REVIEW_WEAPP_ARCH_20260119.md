# 小程序（消费侧）架构与页面接口审查

## 范围与结论
- 消费侧根包页面数量：36（不含 merchant/rider/operator/register/platform 子包）
- 入口与导航：TabBar 为 外卖/预定/我的。其余为功能页面与流程页。

## 技术栈
- 运行环境：微信小程序。
- 语言：TypeScript（源码在 miniprogram/ 下，编译为 JS）。
- UI 组件：tdesign-miniprogram。
- 其它依赖：dayjs（时间处理）、echarts/echarts-for-weixin（图表）。

## 架构分层（消费侧相关）
- 应用层：App 初始化、登录、定位、全局错误处理、网络监控与主题管理。
  - 关键文件：[weapp/miniprogram/app.ts](weapp/miniprogram/app.ts)
- API 层：miniprogram/api 下按领域拆分（订单、支付、预约、商家、地址、用户等），统一走 request 封装。
  - request 封装包含：Token 自动刷新、缓存、重试、网络状态检查、性能监控与统一响应信封。
  - 关键文件：[weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts)
- Service 层：对 API 再封装/组合（如购物车、优惠券、地图），统一状态与业务逻辑。
  - 示例：[weapp/miniprogram/services/cart.ts](weapp/miniprogram/services/cart.ts)
- 业务页面层：miniprogram/pages 下按业务域分模块。

## 业务域概览（消费侧）
- 外卖：商家列表/详情、菜品详情、购物车、下单与支付。
- 预定：房间/商家搜索、预定确认、预定详情与修改。
- 堂食：扫码进桌、点餐、结账与支付成功页。
- 订单：订单列表、订单详情、订单配送轨迹。
- 用户中心：地址、优惠券、会员、收藏、评价、钱包/支付/退款详情。
- 消息与支持：消息中心、客服支持与信用页。

## 消费侧页面与接口使用清单
> 说明：以下为页面中直接引用的 API/Service 符号；若页面包含 request() 调用会标注。

### pages/takeout/index
- 页面文件：[weapp/miniprogram/pages/takeout/index.ts](weapp/miniprogram/pages/takeout/index.ts)
- API 模块与符号：
  - ../../api/dish: searchDishes、DishSummary、getTags、ComboSummary
  - ../../api/cart: getUserCarts
  - ../../api/merchant: （仅引用模块，无显式符号）
  - ../../api/combo: （仅引用模块，无显式符号）
- Service 模块与符号：
  - ../../services/cart: CartService
- 直接 request 调用：否

### pages/takeout/cart/index
| pages/dining/index | [x] | [x] | [x] | [x] |  |  |
- API 模块与符号：
  - @/api/cart: CartAPI、UserCartsResponse、MerchantCartResponse、CartResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/reservation/index
- 页面文件：[weapp/miniprogram/pages/reservation/index.ts](weapp/miniprogram/pages/reservation/index.ts)
- API 模块与符号：
  - ../../api/search: searchRooms、getRecommendedMerchants、RoomSearchResult
  - ../../api/merchant: MerchantSummary
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/index
- 页面文件：[weapp/miniprogram/pages/user_center/index.ts](weapp/miniprogram/pages/user_center/index.ts)
- API 模块与符号：
  - ../../api/auth: updateUserInfo、getUserInfo
  - ../../api/upload: UploadService
- Service 模块与符号：无
- 直接 request 调用：否

### pages/takeout/dish-detail/index
- 页面文件：[weapp/miniprogram/pages/takeout/dish-detail/index.ts](weapp/miniprogram/pages/takeout/dish-detail/index.ts)
- API 模块与符号：
  - ../../../api/dish: DishManagementService、DishResponse
  - ../../../api/personal: getMerchantReviews
- Service 模块与符号：
  - ../../../services/cart: （仅引用模块，无显式符号）
- 直接 request 调用：否

### pages/dining/index
- 页面文件：[weapp/miniprogram/pages/dining/index.ts](weapp/miniprogram/pages/dining/index.ts)
- API 模块与符号：
  - ../../api/reservation: precheckDiningSession、openDiningSession、DiningSessionDTO、BillingGroupDTO
  - ../../api/dining-session: createDiningOrder
  - ../../api/billing-group: createBillingGroup、listBillingGroupOrders
  - ../../api/order: getOrderDetail
  - ../../api/merchant: getMerchantDishes、DishDTO
- Service 模块与符号：
  - ../../services/cart: CartService
- 直接 request 调用：否

### pages/orders/detail/index
- 页面文件：[weapp/miniprogram/pages/orders/detail/index.ts](weapp/miniprogram/pages/orders/detail/index.ts)
- API 模块与符号：
  - ../../../api/order: getOrderDetail、confirmOrder、cancelOrder、urgeOrder、OrderResponse
  - ../../../api/payment: processPayment
  - ../../../api/reservation: ReservationService、ReservationResponse
- Service 模块与符号：
  - ../../../services/cart: CartService
- 直接 request 调用：否

### pages/orders/list/index
- 页面文件：[weapp/miniprogram/pages/orders/list/index.ts](weapp/miniprogram/pages/orders/list/index.ts)
- API 模块与符号：
  - ../../../api/order: getOrders、cancelOrder、OrderStatus、getOrderDetail
- Service 模块与符号：
  - ../../../services/cart: CartService
- 直接 request 调用：否

### pages/reservation/confirm/index
- 页面文件：[weapp/miniprogram/pages/reservation/confirm/index.ts](weapp/miniprogram/pages/reservation/confirm/index.ts)
- API 模块与符号：
  - ../../../api/reservation: createReservation、CreateReservationRequest
  - ../../../api/room: checkRoomAvailability
- Service 模块与符号：无
- 直接 request 调用：否

### pages/reservation/detail/index
- 页面文件：[weapp/miniprogram/pages/reservation/detail/index.ts](weapp/miniprogram/pages/reservation/detail/index.ts)
- API 模块与符号：
  - ../../../api/reservation: ReservationItem、ReservationService、ReservationResponse、ReservationStatus
  - ../../../api/payment: processPayment
- Service 模块与符号：无
- 直接 request 调用：否

### pages/reservation/modify/index
- 页面文件：[weapp/miniprogram/pages/reservation/modify/index.ts](weapp/miniprogram/pages/reservation/modify/index.ts)
- API 模块与符号：
  - ../../../api/reservation: ReservationItem、ReservationResponse、ReservationService
  - ../../../api/merchant: getMerchantDishes
- Service 模块与符号：无
- 直接 request 调用：否

### pages/reservation/room-detail/index
- 页面文件：[weapp/miniprogram/pages/reservation/room-detail/index.ts](weapp/miniprogram/pages/reservation/room-detail/index.ts)
- API 模块与符号：
  - ../../../api/reservation: getRoomDetail、Room
  - ../../../api/room: checkRoomAvailability、RoomAvailabilityResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/takeout/order-confirm/index
- 页面文件：[weapp/miniprogram/pages/takeout/order-confirm/index.ts](weapp/miniprogram/pages/takeout/order-confirm/index.ts)
- API 模块与符号：
  - ../../../api/cart: CartAPI、CartItemResponse
  - ../../../api/address: AddressService、Address
  - ../../../api/order: createOrder、CreateOrderRequest
- Service 模块与符号：无
- 直接 request 调用：否

### pages/takeout/restaurant-detail/index
- 页面文件：[weapp/miniprogram/pages/takeout/restaurant-detail/index.ts](weapp/miniprogram/pages/takeout/restaurant-detail/index.ts)
- API 模块与符号：
  - ../../../api/merchant: getPublicMerchantDetail、getPublicMerchantDishes、getPublicMerchantCombos、PublicMerchantDetail、PublicDishCategory
  - ../../../api/room: getPublicMerchantRooms、PublicRoom
  - ../../../api/cart: getUserCarts
- Service 模块与符号：
  - ../../../services/cart: （仅引用模块，无显式符号）
- 直接 request 调用：否

### pages/takeout/merchant-info/index
- 页面文件：[weapp/miniprogram/pages/takeout/merchant-info/index.ts](weapp/miniprogram/pages/takeout/merchant-info/index.ts)
- API 模块与符号：
  - ../../../api/merchant: getPublicMerchantDetail、PublicMerchantDetail
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/addresses/index
- 页面文件：[weapp/miniprogram/pages/user_center/addresses/index.ts](weapp/miniprogram/pages/user_center/addresses/index.ts)
- API 模块与符号：
  - ../../../api/address: AddressService、Address
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/addresses/edit/index
- 页面文件：[weapp/miniprogram/pages/user_center/addresses/edit/index.ts](weapp/miniprogram/pages/user_center/addresses/edit/index.ts)
- API 模块与符号：
  - ../../../../api/address: AddressService、Address、CreateAddressRequest、UpdateAddressRequest
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/coupons/index
- 页面文件：[weapp/miniprogram/pages/user_center/coupons/index.ts](weapp/miniprogram/pages/user_center/coupons/index.ts)
- API 模块与符号：
  - ../../../api/personal: getMyVouchers、getMyAvailableVouchers、claimVoucher、UserVoucherResponse、VoucherResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/credit/index
- 页面文件：[weapp/miniprogram/pages/user_center/credit/index.ts](weapp/miniprogram/pages/user_center/credit/index.ts)
- API 模块与符号：无
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/favorites/index
- 页面文件：[weapp/miniprogram/pages/user_center/favorites/index.ts](weapp/miniprogram/pages/user_center/favorites/index.ts)
- API 模块与符号：
  - ../../../api/personal: getFavoriteDishes、getFavoriteMerchants、removeDishFromFavorites、removeMerchantFromFavorites、FavoriteDishResponse、FavoriteMerchantResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/membership/index
- 页面文件：[weapp/miniprogram/pages/user_center/membership/index.ts](weapp/miniprogram/pages/user_center/membership/index.ts)
- API 模块与符号：
  - ../../../api/personal: getMyMemberships、MembershipResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/reviews/index
- 页面文件：[weapp/miniprogram/pages/user_center/reviews/index.ts](weapp/miniprogram/pages/user_center/reviews/index.ts)
- API 模块与符号：
  - ../../../api/personal: getMyReviews、ReviewResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/wallet/index
- 页面文件：[weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts)
- API 模块与符号：
  - ../../../api/personal: getMyMemberships
  - ../../../api/payment-refund: getPayments、Payment
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/payment-detail/index
  - ../../../api/payment-refund: getPaymentById、closePayment、getPaymentRefunds、getPayments、Payment、Refund
- Service 模块与符号：无
- 直接 request 调用：否

| pages/user_center/payment-detail/index | [x] | [x] | [x] | [x] |  | 支付终态为 paid/refunded/closed（轮询终止条件） |
- 页面文件：[weapp/miniprogram/pages/user_center/refund-detail/index.ts](weapp/miniprogram/pages/user_center/refund-detail/index.ts)
- Service 模块与符号：无
### pages/user_center/reservations/index
- 页面文件：[weapp/miniprogram/pages/user_center/reservations/index.ts](weapp/miniprogram/pages/user_center/reservations/index.ts)
- API 模块与符号：
  - ../../../api/reservation: ReservationService、ReservationResponse、ReservationStatus
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user_center/reviews/create/index
- 页面文件：[weapp/miniprogram/pages/user_center/reviews/create/index.ts](weapp/miniprogram/pages/user_center/reviews/create/index.ts)
- API 模块与符号：
  - ../../../../api/personal: createReview、CreateReviewRequest
  - ../../../../api/order: getOrderDetail
  - ../../../../api/review: ReviewService.uploadReviewImage
- Service 模块与符号：无
- 直接 request 调用：否

### pages/dine-in/scan-entry/scan-entry
- 页面文件：[weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts](weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts)
- API 模块与符号：
  - ../../../api/table: scanTable、getTableDetail
  - ../../../api/dining-session: transferDiningSessionTable
- Service 模块与符号：无
- 直接 request 调用：否

### pages/dine-in/menu/menu
- 页面文件：[weapp/miniprogram/pages/dine-in/menu/menu.ts](weapp/miniprogram/pages/dine-in/menu/menu.ts)
- API 模块与符号：
  - ../../../api/table: scanTable、ScanTableResponse、ScanTableCategoryInfo
  - ../../../api/cart: getCart、addToCart、updateCartItem、removeFromCart、calculateCart、CartResponse、CartItemResponse
  - ../../../api/reservation: getReservationDetail
  - ../../../api/merchant: getMerchantDishes
  - ../../../api/dish: DishResponse
- Service 模块与符号：无
- 直接 request 调用：否

### pages/dine-in/checkout/checkout
- 页面文件：[weapp/miniprogram/pages/dine-in/checkout/checkout.ts](weapp/miniprogram/pages/dine-in/checkout/checkout.ts)
- API 模块与符号：
  - ../../../api/cart: getCart、calculateCart
  - ../../../api/order: createOrder
  - ../../../api/payment: createPayment、invokeWechatPay
  - ../../../api/table: getTableDetail
  - ../../../api/reservation: getReservationDetail
  - ../../../api/coupon: CouponService
  - ../../../api/personal: getMyMemberships
  - ../../../api/merchant: getPublicMerchantDetail
- Service 模块与符号：无
- 直接 request 调用：否

### pages/dine-in/payment-success/payment-success
- 页面文件：[weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts](weapp/miniprogram/pages/dine-in/payment-success/payment-success.ts)
- API 模块与符号：无
- Service 模块与符号：无
- 直接 request 调用：否

### pages/orders/tracking/index
- 页面文件：[weapp/miniprogram/pages/orders/tracking/index.ts](weapp/miniprogram/pages/orders/tracking/index.ts)
- API 模块与符号：
  - ../../../api/order: getOrderDetail
  - ../../../api/delivery: getDeliveryByOrder、getRiderLocation、DeliveryResponse
  - ../../../api/location: getBicyclingDirection
- Service 模块与符号：无
- 直接 request 调用：否

### pages/support/index
- 页面文件：[weapp/miniprogram/pages/support/index.ts](weapp/miniprogram/pages/support/index.ts)
- API 模块与符号：无
- Service 模块与符号：无
- 直接 request 调用：否

### pages/message/center/index
- 页面文件：[weapp/miniprogram/pages/message/center/index.ts](weapp/miniprogram/pages/message/center/index.ts)
- API 模块与符号：
  - ../../../api/notification: notificationService、Notification
- Service 模块与符号：无
- 直接 request 调用：否

### pages/credit/index
- 页面文件：[weapp/miniprogram/pages/credit/index.ts](weapp/miniprogram/pages/credit/index.ts)
- API 模块与符号：无
- Service 模块与符号：无
- 直接 request 调用：否

### pages/user/bind-merchant/index
- 页面文件：[weapp/miniprogram/pages/user/bind-merchant/index.ts](weapp/miniprogram/pages/user/bind-merchant/index.ts)
- API 模块与符号：
  - ../../../api/personal: bindMerchant、BindMerchantResponse
- Service 模块与符号：无
- 直接 request 调用：否

## 备注
- 页面以 API 模块/Service 作为调用边界，便于统一鉴权、缓存与错误处理。

## 后端最新实现核对（v1 路由）
> 说明：以下为消费侧相关接口在后端的最新路由注册位置，统一以 server.go 作为入口核对。

- 搜索 / 公共数据 / 扫码
  - /v1/search/dishes|merchants|combos|rooms：[locallife/api/server.go](locallife/api/server.go#L320-L335)
  - /v1/scan/table：[locallife/api/server.go](locallife/api/server.go#L341-L346)
  - /v1/public/dishes/:id、/v1/public/merchants/:id(/dishes|/combos|/rooms)：[locallife/api/server.go](locallife/api/server.go#L348-L354)

- 用户 / 地址 / 位置 / 绑定
  - /v1/users/me、/v1/auth/bind-phone：[locallife/api/server.go](locallife/api/server.go#L372-L378)
  - /v1/addresses（增删改查/默认）：[locallife/api/server.go](locallife/api/server.go#L380-L386)
  - /v1/location/reverse-geocode、/v1/location/direction/bicycling：[locallife/api/server.go](locallife/api/server.go#L388-L390)
  - /v1/bind-merchant：[locallife/api/server.go](locallife/api/server.go#L417-L425)

- 预订 / 包间 / 用餐会话 / 账单组
  - /v1/reservations（创建/查询/取消/改菜/签到等）：[locallife/api/server.go](locallife/api/server.go#L588-L638)
  - /v1/rooms、/v1/merchants/:id/rooms（C 端包间）：[locallife/api/server.go](locallife/api/server.go#L568-L586)
  - /v1/dining-sessions（预检查/开台/转桌）：[locallife/api/server.go](locallife/api/server.go#L640-L648)
  - /v1/billing-groups：[locallife/api/server.go](locallife/api/server.go#L650-L656)

- 订单 / 支付 / 退款 / 配送
  - /v1/orders（列表/详情/取消/催单/确认等）：[locallife/api/server.go](locallife/api/server.go#L658-L672)
  - /v1/payments（创建/查询/关闭/退款列表）：[locallife/api/server.go](locallife/api/server.go#L704-L712)
  - /v1/refunds（创建/详情）：[locallife/api/server.go](locallife/api/server.go#L714-L720)
  - /v1/delivery（订单配送/轨迹/骑手位置）：[locallife/api/server.go](locallife/api/server.go#L744-L777)

- 通知 / 购物车 / 收藏 / 会员 / 评价 / 优惠券
  - /v1/notifications：[locallife/api/server.go](locallife/api/server.go#L788-L802)
  - /v1/cart（items/clear/calculate/summary/combined-checkout）：[locallife/api/server.go](locallife/api/server.go#L936-L946)
  - /v1/favorites（merchants/dishes）：[locallife/api/server.go](locallife/api/server.go#L950-L960)
  - /v1/memberships（列表/详情/交易）：[locallife/api/server.go](locallife/api/server.go#L970-L986)
  - /v1/reviews（创建/我的评价/图片上传）：[locallife/api/server.go](locallife/api/server.go#L990-L1004)
  - /v1/vouchers（claim/me/available）：[locallife/api/server.go](locallife/api/server.go#L1084-L1097)

## 逐页审查任务计划（消费侧）
> 目标：逐页检查后端接口是否正确、字段/方法是否对齐，并消除前端 any 类型。以页面为单位勾选完成，并进行横向对比。

### 统一检查清单（每页都要做）
- 接口正确性：页面使用的 API 是否与业务场景匹配，是否存在临时/Mock 接口。
- 字段对齐：请求字段、响应字段、枚举值与后端定义一致（含可选字段与命名风格）。
- 方法对齐：HTTP 方法、路径、分页参数、状态流转一致。
- 类型治理：移除 any（含隐式 any），补齐类型定义、DTO/Response 类型，避免类型断言滥用。
- 复用与边界：优先使用 api/ 与 services/ 层封装，避免页面直连 request。

### 页面任务清单（含横向对比）
> 说明：每行对应一个页面。勾选“接口/字段/方法/any”完成情况；“对比结果”用于标注与同域页面的差异与发现。

| 页面 | 接口正确性 | 字段对齐 | 方法对齐 | any 清理 | 对比结果 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| pages/takeout/index | [x] | [x] | [x] | [x] |  | searchCombos 后端返回 original_price/combo_price 为分（Cents） |
| pages/takeout/cart/index | [x] | [x] | [x] | [x] |  |  |
| pages/takeout/dish-detail/index | [x] | [x] | [x] | [x] |  |  |
| pages/takeout/order-confirm/index | [x] | [x] | [x] | [x] |  |  |
| pages/takeout/restaurant-detail/index | [x] | [x] | [x] | [x] |  |  |
| pages/takeout/merchant-info/index | [x] | [x] | [x] | [x] |  |  |
| pages/reservation/index | [x] | [x] | [x] | [x] |  |  |
| pages/reservation/confirm/index | [x] | [x] | [x] | [x] |  |  |
| pages/reservation/detail/index | [x] | [x] | [x] | [x] |  |  |
| pages/reservation/modify/index | [x] | [x] | [x] | [x] |  |  |
| pages/reservation/room-detail/index | [x] | [x] | [x] | [x] |  |  |
| pages/dine-in/scan-entry/scan-entry | [x] | [x] | [x] | [x] |  |  |
| pages/dine-in/menu/menu | [x] | [x] | [x] | [x] |  |  |
| pages/dine-in/checkout/checkout | [x] | [x] | [x] | [x] |  |  |
| pages/dine-in/payment-success/payment-success | [x] | [x] | [x] | [x] |  |  |
| pages/dining/index | [x] | [x] | [x] | [x] |  |  |
| pages/orders/list/index | [x] | [x] | [x] | [x] |  | 列表响应为 {orders,total_count,page_id,page_size} |
| pages/orders/detail/index | [x] | [x] | [x] | [x] |  | 订单 items 可为空，适配空数组渲染 |
| pages/orders/tracking/index | [x] | [x] | [x] | [x] |  | 配送联系方式字段可选，骑手位置 accuracy/speed/heading 可选 |
| pages/user_center/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/addresses/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/addresses/edit/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/coupons/index | [x] | [x] | [x] | [x] |  | 代金券列表需 page_id/page_size（/v1/vouchers/me*） |
| pages/user_center/credit/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/favorites/index | [x] | [x] | [x] | [x] |  | 收藏商户仅返回 address/status；菜品字段为 image_url/description |
| pages/user_center/membership/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/reviews/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/reviews/create/index | [x] | [x] | [x] | [x] |  |  |
| pages/user_center/wallet/index | [x] | [x] | [x] | [x] |  | 支付列表返回 payment_orders（/v1/payments） |
| pages/user_center/payment-detail/index | [x] | [x] | [x] | [x] |  | 退款列表返回 refund_orders，支付终态 paid/refunded/closed |
| pages/user_center/refund-detail/index | [x] | [x] | [x] | [x] |  | 退款字段使用 refund_amount/refunded_at |
| pages/user_center/reservations/index | [x] | [x] | [x] | [x] |  | 列表响应含 total_count/page_id/page_size |
| pages/message/center/index | [x] | [x] | [x] | [x] |  | 通知分页使用 limit/offset；类型枚举 order/payment/delivery/system/food_safety |
| pages/support/index | [x] | [x] | [x] | [x] |  |  |
| pages/credit/index | [x] | [x] | [x] | [x] |  |  |
| pages/user/bind-merchant/index | [x] | [x] | [x] | [x] |  |  |
