# API接口重构说明

## 重构概述

本次重构将现有的API接口完全对齐swagger.json文档，解决了以下关键问题：

### 主要问题修复

1. **ID类型不匹配**: 所有ID字段从`string`改为`number`类型
2. **字段名不匹配**: 如`image_url` vs `logo_url`等字段名对齐
3. **接口路径错误**: 使用swagger中定义的正确API路径
4. **数据结构缺失**: 添加后端定义的所有必要字段

### 已完成的重构

#### 1. 认证接口 (`miniprogram/api/auth.ts`)
- ✅ 重构微信登录：`/v1/auth/wechat-login`
- ✅ 新增刷新令牌：`/v1/auth/renew-access`
- ✅ 重构用户信息：`/v1/users/me`
- ✅ 数据模型完全对齐swagger定义

**关键变更**:
```typescript
// 旧版本
interface LoginRequest { code: string }
interface LoginData { user_id: string, roles: string[] }

// 新版本 - 对齐swagger
interface WechatLoginRequest {
  code: string
  device_id: string
  device_type: 'ios' | 'android' | 'miniprogram' | 'h5'
}
interface WechatLoginResponse {
  access_token: string
  refresh_token: string
  user: UserResponse  // 包含完整用户信息
}
```

#### 2. 地址管理接口 (`miniprogram/api/address.ts`)
- ✅ 新增完整的地址CRUD操作
- ✅ 基于`/v1/addresses`系列接口
- ✅ 支持默认地址设置

**新增功能**:
```typescript
// 完整的地址管理功能
getUserAddresses()           // 获取地址列表
createUserAddress(data)      // 创建地址
updateUserAddress(id, data)  // 更新地址
deleteUserAddress(id)        // 删除地址
setDefaultAddress(id)        // 设置默认地址
```

#### 3. 商户接口重构 (`miniprogram/api/merchant.ts`)
- ✅ 重构商户数据模型，对齐`api.merchantSummary`
- ✅ 新增搜索和推荐接口
- ✅ ID类型从string改为number

**关键变更**:
```typescript
// 旧版本
interface MerchantSimpleDTO {
  id: string
  image_url: string
  rating: number  // 后端不存在此字段
}

// 新版本 - 对齐swagger
interface MerchantSummary {
  id: number
  logo_url: string
  monthly_sales: number
  estimated_delivery_fee: number
  tags: string[]
}
```

#### 4. 行为追踪接口 (`miniprogram/api/behavior.ts`)
- ✅ 新增用户行为埋点功能
- ✅ 基于`/v1/behaviors/track`接口
- ✅ 支持浏览、详情、加购、购买等行为类型

#### 5. 数据模型重构 (`miniprogram/models/dish.ts`)
- ✅ 菜品模型完全对齐swagger定义
- ✅ 新增定制化选项支持
- ✅ 支持会员价、制作时间等新字段

**关键变更**:
```typescript
// 旧版本
interface Dish {
  id: string
  merchantId: string
  spec_groups?: DishSpecGroup[]
}

// 新版本 - 对齐swagger
interface Dish {
  id: number
  merchantId: number
  customization_groups?: CustomizationGroup[]
  member_price?: number
  prepare_time?: number
}
```

#### 6. 适配器更新 (`miniprogram/adapters/dish.ts`)
- ✅ 更新适配器以支持新的数据结构
- ✅ 新增`fromSummaryDTO`方法处理Feed流数据
- ✅ 保持向后兼容性

### 兼容性处理

为了确保现有代码不受影响，我们保留了旧接口的别名：

```typescript
// 兼容性别名
export const login = wechatLogin
export const getMerchants = searchMerchants
export type LoginRequest = WechatLoginRequest
export type DishDTO = DishResponse
```

#### 7. 菜品接口重构 (`miniprogram/api/dish.ts`)
- ✅ 重构菜品详情：`/v1/dishes/{id}`
- ✅ 重构菜品定制化：`/v1/dishes/{id}/customizations`
- ✅ 重构菜品搜索：`/v1/search/dishes`
- ✅ 重构菜品推荐：`/v1/recommendations/dishes`
- ✅ 重构菜品分类：`/v1/dishes/categories`
- ✅ 新增商户端菜品管理功能

**关键变更**:
```typescript
// 新增完整的菜品管理功能
getDishDetail(id)              // 获取菜品详情
getDishCustomizations(id)      // 获取定制化选项
searchDishes(params)           // 搜索菜品
getRecommendedDishes(params)   // 获取推荐菜品
getDishCategories()            // 获取分类列表
// 商户端管理功能
createDish(data)               // 创建菜品
updateDish(id, data)           // 更新菜品
updateDishStatus(id, status)   // 更新状态
batchUpdateDishStatus(updates) // 批量更新状态
```

#### 8. 搜索接口重构 (`miniprogram/api/search.ts`)
- ✅ 重构商户搜索：`/v1/search/merchants`
- ✅ 重构商户推荐：`/v1/recommendations/merchants`
- ✅ 重构包间搜索：`/v1/search/rooms`
- ✅ 新增搜索建议和历史功能
- ✅ 新增综合搜索功能

**新增功能**:
```typescript
// 完整的搜索功能
searchMerchants(params)        // 搜索商户
searchRooms(params)            // 搜索包间
getSearchSuggestions(keyword)  // 获取搜索建议
getPopularKeywords()           // 获取热门关键词
getSearchHistory()             // 获取搜索历史
unifiedSearch(keyword)         // 综合搜索（菜品+商户）
```

#### 9. 套餐接口重构 (`miniprogram/api/combo.ts`)
- ✅ 重构套餐列表：`/v1/combos`
- ✅ 重构套餐详情：`/v1/combos/{id}`
- ✅ 重构套餐推荐：`/v1/recommendations/combos`
- ✅ 新增商户端套餐管理功能

**关键变更**:
```typescript
// 套餐管理功能
getComboSetDetail(id)          // 获取套餐详情
getRecommendedCombos(params)   // 获取推荐套餐
// 商户端管理功能
createComboSet(data)           // 创建套餐
updateComboSet(id, data)       // 更新套餐
updateComboSetOnlineStatus()   // 上下架套餐
addDishToCombo()               // 添加菜品到套餐
removeDishFromCombo()          // 从套餐移除菜品
```

#### 10. 购物车接口重构 (`miniprogram/api/cart.ts`)
- ✅ 重构购物车管理：`/v1/cart`, `/v1/cart/items`
- ✅ 重构购物车计算：`/v1/cart/calculate`, `/v1/cart/summary`
- ✅ 新增合并结账功能：`/v1/cart/combined-checkout/preview`
- ✅ 新增便捷操作方法

**新增功能**:
```typescript
// 完整的购物车功能
getCart(merchantId)            // 获取购物车
getCartSummary()               // 获取购物车摘要
addToCart(item)                // 添加商品
updateCartItem(id, updates)    // 更新商品
removeFromCart(id)             // 删除商品
clearCart(merchantId)          // 清空购物车
calculateCart(params)          // 计算金额
previewCombinedCheckout()      // 合并结账预览
```

#### 11. 订单接口重构 (`miniprogram/api/order.ts`)
- ✅ 重构订单管理：`/v1/orders`, `/v1/orders/{id}`
- ✅ 重构订单计算：`/v1/orders/calculate`
- ✅ 重构订单操作：`/v1/orders/{id}/cancel`, `/v1/orders/{id}/confirm`, `/v1/orders/{id}/urge`
- ✅ 数据模型完全对齐swagger定义

**关键变更**:
```typescript
// 旧版本
interface OrderDTO {
  id: string
  merchant_id: string
  status: string
}

// 新版本 - 对齐swagger
interface OrderResponse {
  id: number
  order_no: string
  merchant_id: number
  status: OrderStatus  // 严格的枚举类型
  order_type: OrderType
  items: OrderItemResponse[]
  // 完整的金额字段
  subtotal: number
  delivery_fee: number
  delivery_fee_discount: number
  discount_amount: number
  payable_amount: number
}
```

#### 12. 支付接口重构 (`miniprogram/api/payment.ts`)
- ✅ 重构支付管理：`/v1/payments`, `/v1/payments/{id}`
- ✅ 重构退款功能：`/v1/payments/{id}/refunds`
- ✅ 新增微信支付集成和状态轮询
- ✅ 新增完整支付流程封装

**新增功能**:
```typescript
// 完整的支付功能
createPayment(data)            // 创建支付订单
getPaymentDetail(id)           // 获取支付详情
createRefund(id, data)         // 创建退款
invokeWechatPay(params)        // 调起微信支付
processPayment(orderId)        // 完整支付流程
pollPaymentStatus(id)          // 轮询支付状态
```

#### 13. 预定接口重构 (`miniprogram/api/reservation.ts`)
- ✅ 重构预定管理：`/v1/reservations`, `/v1/reservations/me`
- ✅ 重构预定操作：`/v1/reservations/{id}/cancel`, `/v1/reservations/{id}/add-dishes`
- ✅ 支持定金模式和全款模式预定
- ✅ **业务逻辑**: 支付成功 = 预定成功，无需商户确认

**关键功能**:
```typescript
// 顾客端预定功能
createReservation(data)        // 创建预定
getMyReservations(params)      // 获取我的预定
cancelReservation(id, reason)  // 取消预定
addDishesToReservation()       // 为预定添加菜品
// 便捷方法
createDepositReservation()     // 创建定金模式预定
createFullPaymentReservation() // 创建全款模式预定
// 商户端查看功能
getMerchantReservations()      // 商户查看预定列表
getMerchantReservationStats()  // 商户查看预定统计
```

#### 14. 扫码点餐和桌台接口重构 (`miniprogram/api/table.ts`)
- ✅ 重构扫码点餐：`/v1/scan/table` - **顾客端功能**
- ✅ 重构桌台管理：`/v1/tables`, `/v1/tables/{id}` - 商户端管理
- ✅ 重构二维码管理：`/v1/tables/{id}/qrcode` - 商户端管理
- ✅ 新增桌台图片和标签管理 - 商户端管理

**核心功能**:
```typescript
// 顾客端扫码点餐功能
scanTable(merchantId, tableNo) // 顾客扫码获取菜单
parseQRCodeUrl(url)            // 解析二维码URL
// 商户端桌台管理功能
getTableDetail(id)             // 获取桌台详情
updateTableStatus()            // 更新桌台状态
getTableQRCode(id)             // 获取桌台二维码
generateQRCodeUrl()            // 生成二维码URL
```

#### 15. 包间接口重构 (`miniprogram/api/room.ts`)
- ✅ 重构包间浏览：`/v1/merchants/{id}/rooms`, `/v1/rooms/{id}` - **顾客端功能**
- ✅ 重构可用性检查：`/v1/rooms/{id}/availability` - **顾客端功能**
- ✅ 新增包间筛选和费用计算功能 - **顾客端功能**

**特色功能**:
```typescript
// 顾客端包间浏览功能
getMerchantAvailableRooms()    // 顾客浏览可用包间
checkRoomAvailability()        // 顾客检查包间可用性
getRoomsByCapacity()           // 按容量筛选包间
getRoomsByPrice()              // 按价格筛选包间
calculateRoomCost()            // 计算包间费用
getAvailableRoomsForTimeSlot() // 获取时间段内可用包间
```

### 重要架构调整

根据用户反馈，我们重新组织了接口重构的结构：

**入驻申请接口归类调整**：
- 商户入驻申请 → 移至顾客端 1.6
- 骑手入驻申请 → 移至顾客端 1.7  
- 运营商入驻申请 → 移至顾客端 1.8

这样的调整更符合实际业务逻辑，因为入驻申请本质上是用户侧的功能。

**OCR智能识别功能**：
入驻申请过程中包含重要的OCR（光学字符识别）功能：
- **身份证OCR**: 自动识别姓名、身份证号、地址、性别、有效期等信息
- **营业执照OCR**: 自动识别企业名称、统一社会信用代码、法定代表人、经营范围、注册资本等
- **食品经营许可证OCR**: 自动识别许可证编号、企业名称、有效期等
- **健康证OCR**: 自动识别证书编号、有效期等

用户上传证件照片后，后端OCR服务会自动识别内容并回传给前端，自动回填到申请表单中，大大提升用户体验和申请效率。

#### 16. 个人中心功能接口重构 (`miniprogram/api/personal.ts`)
- ✅ 重构收藏功能：`/v1/favorites/dishes`, `/v1/favorites/merchants`
- ✅ 重构浏览历史：`/v1/history/browse`
- ✅ 重构评价系统：`/v1/reviews`, `/v1/reviews/me`, `/v1/reviews/{id}`
- ✅ 重构会员系统：`/v1/memberships`, `/v1/memberships/recharge`, `/v1/vouchers/me`
- ✅ 重构通知系统：`/v1/notifications`, `/v1/notifications/preferences`
- ✅ 重构申诉功能：`/v1/claims`, `/v1/claims/{id}`

**核心功能**:
```typescript
// 收藏管理功能
getFavoriteDishes()            // 获取收藏菜品
addDishToFavorites(dishId)     // 添加菜品收藏
getFavoriteMerchants()         // 获取收藏商户
toggleDishFavorite(dishId)     // 切换菜品收藏状态
// 浏览历史功能
getBrowseHistory(params)       // 获取浏览历史
clearBrowseHistory()           // 清空浏览历史
// 评价系统功能
createReview(data)             // 创建评价
getMyReviews(params)           // 获取我的评价
getMerchantReviews(id, params) // 获取商户评价
replyToReview(id, data)        // 商户回复评价
// 会员系统功能
getMyMemberships()             // 获取会员卡列表
rechargeMembership(data)       // 会员卡充值
getMembershipTransactions()   // 获取交易记录
// 通知系统功能
getNotifications(params)       // 获取通知列表
getUnreadNotificationCount()   // 获取未读数量
markAllNotificationsAsRead()   // 全部标记已读
updateNotificationPreferences() // 更新通知设置
// 申诉功能
createClaim(data)              // 创建申诉
getMyClaims(params)            // 获取申诉列表
getClaimDetail(id)             // 获取申诉详情
// 优惠券功能
getMyVouchers(params)          // 获取我的优惠券
getMyAvailableVouchers()       // 获取可用优惠券
claimVoucher(voucherId)        // 领取优惠券
// 便捷方法
getPersonalCenterOverview()    // 获取个人中心概览
```

#### 17. 商户入驻申请接口重构 (`miniprogram/api/merchant-application.ts`)
- ✅ 重构商户申请：`/v1/merchants/applications`, `/v1/merchants/applications/me`
- ✅ 重构申请流程：`/v1/merchant/application/*`, `/v1/merchant/application/submit`
- ✅ 重构OCR识别和数据回填：
  * 身份证OCR：`/v1/merchant/application/idcard/ocr` → 自动回填姓名、身份证号、地址等
  * 营业执照OCR：`/v1/merchant/application/license/ocr` → 自动回填企业名称、统一社会信用代码、法定代表人、经营范围、地址等
  * 食品经营许可证OCR：`/v1/merchant/application/foodpermit/ocr` → 自动回填许可证编号、有效期等
- ✅ 重构银行绑定：`/v1/merchant/bindbank`, `/v1/merchant/applyment/status`

**核心功能**:
```typescript
// 申请管理功能
getMerchantApplicationDraft()      // 获取或创建申请草稿
updateMerchantApplicationBasic()   // 更新基本信息
submitMerchantApplication()        // 提交申请
getMyMerchantApplication()         // 获取申请状态
// OCR智能识别功能
recognizeIDCardFront(imageUrl)     // 身份证正面OCR识别
recognizeBusinessLicense(imageUrl) // 营业执照OCR识别
recognizeFoodPermit(imageUrl)      // 食品许可证OCR识别
// 银行绑定功能
bindMerchantBank(data)             // 绑定银行账户
getMerchantApplymentStatus()       // 获取申请状态
// 完整流程封装
const flow = createMerchantApplicationFlow()
await flow.initialize()           // 初始化申请
await flow.uploadAndRecognizeIDCard(url)      // 上传身份证并自动回填
await flow.uploadAndRecognizeBusinessLicense(url) // 上传营业执照并自动回填
await flow.updateBasicInfo(data)  // 更新基本信息
await flow.submit()               // 提交申请
// 便捷方法
checkMerchantApplicationStatus()   // 检查申请状态
getApplicationStatusDescription()  // 获取状态描述
```

**OCR智能识别特色**:
- **身份证OCR**: 自动识别姓名、身份证号、地址、性别、有效期等信息
- **营业执照OCR**: 自动识别企业名称、统一社会信用代码、法定代表人、经营范围、注册资本、地址等
- **食品经营许可证OCR**: 自动识别许可证编号、企业名称、有效期等
- **智能回填**: OCR识别后自动回填到申请表单，大大提升用户体验和申请效率
- **商户名称自定义**: 营业执照识别的企业名称作为默认值，商户可以设置自定义的店铺名称

#### 18. 骑手入驻申请接口重构 (`miniprogram/api/rider-application.ts`)
- ✅ 重构骑手申请：`/v1/rider/apply`, `/v1/rider/application/*`
- ✅ 重构申请流程：`/v1/rider/application/basic`, `/v1/rider/application/submit`
- ✅ 重构OCR识别和数据回填：
  * 身份证OCR：`/v1/rider/application/idcard/ocr` → 自动回填姓名、身份证号、地址、性别等
  * 健康证OCR：`/v1/rider/application/healthcert` → 自动回填证书编号、有效期等
- ✅ 重构银行绑定：`/v1/rider/applyment/bindbank`, `/v1/rider/applyment/status`

**核心功能**:
```typescript
// 申请管理功能
getRiderApplicationDraft()         // 获取或创建申请草稿
updateRiderApplicationBasic()      // 更新基本信息
submitRiderApplication()           // 提交申请
resetRiderApplication()            // 重置申请
// OCR智能识别功能
recognizeRiderIDCard(imageUrl)     // 身份证OCR识别
uploadHealthCert(imageUrl)         // 健康证上传和OCR识别
// 银行绑定功能
bindRiderBank(data)                // 绑定银行账户
getRiderApplymentStatus()          // 获取申请状态
// 完整流程封装
const flow = createRiderApplicationFlow()
await flow.initialize()           // 初始化申请
await flow.uploadAndRecognizeIDCard(url)      // 上传身份证并自动回填
await flow.uploadAndRecognizeHealthCert(url)  // 上传健康证并自动回填
await flow.updateBasicInfo(data)  // 更新基本信息
await flow.submit()               // 提交申请
// 便捷方法和验证
checkRiderApplicationStatus()      // 检查申请状态
validateRiderApplicationForm()     // 表单验证
validatePhoneNumber()              // 手机号验证
validateRealName()                 // 姓名验证
```

**OCR智能识别特色**:
- **身份证OCR**: 自动识别姓名、身份证号、地址、性别、民族、有效期等信息
- **健康证OCR**: 自动识别证书编号、有效期起止时间等
- **智能回填**: OCR识别后自动回填到申请表单，提升申请效率
- **表单验证**: 提供完整的表单验证功能，包括手机号、身份证号、姓名格式验证

### 下一步计划

1. **继续重构顾客端接口**：
   - 运营商入驻申请接口（1.8）

2. **继续重构管理端接口**：
   - 商户端管理接口（已入驻后的管理功能）
   - 骑手端管理接口（已入驻后的配送管理）
   - 运营商管理接口（区域和数据管理）
   - 超管平台接口

2. **更新现有页面代码**：
   - 更新页面中的API调用
   - 修复数据类型不匹配问题
   - 测试所有功能的正常运行

3. **与后端联调测试**：
   - 验证所有接口的正确性
   - 确保数据格式完全匹配
   - 处理可能的兼容性问题

### 注意事项

⚠️ **重要提醒**：
- 所有ID字段已改为number类型，需要更新相关的页面代码
- 接口路径已更新，确保后端部署了对应的API
- 新增的字段可能需要在UI中进行展示
- 建议在开发环境中充分测试后再部署到生产环境