/**
 * 页面导航工具类
 * 统一管理小程序页面跳转逻辑
 */

export class Navigation {
  // ==================== 外卖相关 ====================

  /**
     * 跳转到菜品详情页
     */
  static toDishDetail(dishId: string, extraInfo?: {
    shopName?: string
    monthSales?: number
    distance?: number
    estimatedDeliveryTime?: number
  }) {
    let url = `/pages/takeout/dish-detail/index?id=${dishId}`
    if (extraInfo) {
      if (extraInfo.shopName) url += `&shop_name=${encodeURIComponent(extraInfo.shopName)}`
      if (extraInfo.monthSales) url += `&month_sales=${extraInfo.monthSales}`
      if (extraInfo.distance) url += `&distance=${extraInfo.distance}`
      if (extraInfo.estimatedDeliveryTime) url += `&estimated_delivery_time=${extraInfo.estimatedDeliveryTime}`
    }
    wx.navigateTo({ url })
  }

  /**
     * 跳转到商户详情页
     */
  static toRestaurantDetail(merchantId: string | number, options?: { activeTab?: string }) {
    let url = `/pages/takeout/restaurant-detail/index?id=${merchantId}`
    if (options?.activeTab) {
      url += `&activeTab=${encodeURIComponent(options.activeTab)}`
    }
    wx.navigateTo({ url })
  }

  /**
     * 跳转到套餐详情页
     */
  static toComboDetail(comboId: string, extraInfo?: {
    shopName?: string
    monthSales?: number
    distance?: number
    estimatedDeliveryTime?: number
  }) {
    let url = `/pages/takeout/combo-detail/index?id=${comboId}`
    if (extraInfo) {
      if (extraInfo.shopName) url += `&shop_name=${encodeURIComponent(extraInfo.shopName)}`
      if (extraInfo.monthSales) url += `&month_sales=${extraInfo.monthSales}`
      if (extraInfo.distance) url += `&distance=${extraInfo.distance}`
      if (extraInfo.estimatedDeliveryTime) url += `&estimated_delivery_time=${extraInfo.estimatedDeliveryTime}`
    }
    wx.navigateTo({ url })
  }

  /**
     * 跳转到购物车页
     */
  static toCart() {
    wx.navigateTo({
      url: '/pages/takeout/cart/index'
    })
  }

  static toTakeoutHome() {
    wx.switchTab({ url: '/pages/takeout/index' })
  }

  static toUserCenterHome() {
    wx.switchTab({ url: '/pages/user_center/index' })
  }

  /**
     * 跳转到订单确认页
     */
  static toOrderConfirm(cartData?: Record<string, unknown>) {
    const url = cartData
      ? `/pages/takeout/order-confirm/index?data=${encodeURIComponent(JSON.stringify(cartData))}`
      : '/pages/takeout/order-confirm/index'
    wx.navigateTo({ url })
  }

  // ==================== 订单相关 ====================

  /**
     * 跳转到订单详情页
     */
  static toOrderDetail(orderId: string) {
    wx.navigateTo({
      url: `/pages/orders/detail/index?id=${orderId}`
    })
  }

  /**
   * 跳转到支付成功页
   */
  static toPaymentSuccess(params: { orderId: string, orderNo: string, amount: string, isCombined?: boolean, orderCount?: number }) {
    const combinedQuery = params.isCombined ? `&combined=1&orderCount=${params.orderCount || 0}` : ''
    wx.redirectTo({
      url: `/pages/orders/success/index?orderId=${params.orderId}&orderNo=${params.orderNo}&amount=${params.amount}${combinedQuery}`
    })
  }

  static toDineInPaymentSuccess(params: { orderId: string, amount: string, merchantName?: string, tableNumber?: string }) {
    let url = `/pages/dine-in/payment-success/payment-success?order_id=${params.orderId}&amount=${params.amount}`
    if (params.merchantName) {
      url += `&merchant_name=${encodeURIComponent(params.merchantName)}`
    }
    if (params.tableNumber) {
      url += `&table_number=${encodeURIComponent(params.tableNumber)}`
    }
    wx.redirectTo({ url })
  }

  /**
     * 跳转到订单列表页
     */
  static toOrderList(tabOrOptions?: 'all' | 'pending' | 'completed' | { orderType?: 'takeout' | 'reservation' | 'dine_in' | 'takeaway' }) {
    let url = '/pages/orders/list/index'

    if (typeof tabOrOptions === 'object' && tabOrOptions?.orderType) {
      url = `/pages/orders/list/index?order_type=${tabOrOptions.orderType}`
    } else if (typeof tabOrOptions === 'string' && tabOrOptions !== 'all') {
      url = `/pages/orders/list/index?tab=${tabOrOptions}`
    }

    wx.navigateTo({ url })
  }

  static redirectToOrderList(options?: { orderType?: 'takeout' | 'reservation' | 'dine_in' | 'takeaway' }) {
    const url = options?.orderType
      ? `/pages/orders/list/index?order_type=${options.orderType}`
      : '/pages/orders/list/index'
    wx.redirectTo({ url })
  }

  /**
     * 跳转到评价页
     * TODO: 页面 pages/orders/review/index 不存在
     */
  /**
     * 跳转到评价页
     */
  static toReview(orderId: string) {
    wx.navigateTo({
      url: `/pages/user_center/reviews/create/index?orderId=${orderId}`
    })
  }

  // ==================== 地址相关 ====================

  /**
     * 跳转到地址列表页
     * @param from 来源页面标识，用于返回时处理
     */
  static toAddressList(from?: string) {
    const url = from
      ? `/pages/user_center/addresses/index?from=${from}`
      : '/pages/user_center/addresses/index'
    wx.navigateTo({ url })
  }

  static toAddressSelector() {
    wx.navigateTo({ url: '/pages/user_center/addresses/index?select=true' })
  }

  /**
     * 跳转到地址编辑页
     * TODO: 页面 pages/user_center/address-edit/index 不存在
     */
  static toAddressEdit(addressId?: string) {
    const url = addressId
      ? `/pages/user_center/address-edit/index?id=${addressId}`
      : '/pages/user_center/address-edit/index'
    wx.navigateTo({ url })
  }

  // ==================== 预订相关 ====================

  /**
     * 跳转到包间详情页
     */
  static toRoomDetail(roomId: string) {
    wx.navigateTo({
      url: `/pages/reservation/room-detail/index?id=${roomId}`
    })
  }

  /**
     * 跳转到预订确认页
     */
  static toReservationConfirm(params: {
    roomId: string
    date: string
    time: string
  }) {
    wx.navigateTo({
      url: `/pages/reservation/confirm/index?room_id=${params.roomId}&date=${params.date}&time=${params.time}`
    })
  }

  /**
     * 跳转到预订详情页
     * TODO: 页面 pages/reservation/detail/index 不存在
     */
  static toReservationDetail(reservationId: string) {
    wx.navigateTo({
      url: `/pages/reservation/detail/index?id=${reservationId}`
    })
  }

  static toReservationCreate(params: { merchantId: string | number, merchantName?: string }) {
    let url = `/pages/reservation/create/index?merchantId=${params.merchantId}`
    if (params.merchantName) {
      url += `&merchantName=${encodeURIComponent(params.merchantName)}`
    }
    wx.navigateTo({ url })
  }

  static toReservationList() {
    wx.navigateTo({ url: '/pages/reservation/list/index' })
  }

  static redirectToReservationList() {
    wx.redirectTo({ url: '/pages/reservation/list/index' })
  }

  static toUserReservations() {
    wx.navigateTo({ url: '/pages/user_center/reservations/index' })
  }

  static toReservationHome() {
    wx.switchTab({ url: '/pages/reservation/index' })
  }

  // ==================== 堂食相关 ====================

  /**
     * 跳转到堂食菜单页
     */
  static toDiningMenu(tableId: string) {
    wx.navigateTo({
      url: `/pages/dining/index?table_id=${tableId}`
    })
  }

  // ==================== 个人中心相关 ====================

  /**
     * 跳转到优惠券页
     */
  static toCoupons() {
    wx.navigateTo({
      url: '/pages/user_center/coupons/index'
    })
  }

  /**
     * 跳转到收藏夹
     */
  static toFavorites() {
    wx.navigateTo({
      url: '/pages/user_center/favorites/index'
    })
  }

  /**
     * 跳转到会员中心
     */
  static toMembership(options?: { membershipId?: string | number, autoRecharge?: boolean }) {
    let url = '/pages/user_center/membership/index'
    const params: string[] = []

    if (options?.membershipId !== undefined) {
      params.push(`membershipId=${encodeURIComponent(String(options.membershipId))}`)
    }

    if (options?.autoRecharge) {
      params.push('autoRecharge=1')
    }

    if (params.length > 0) {
      url = `${url}?${params.join('&')}`
    }

    wx.navigateTo({ url })
  }

  /**
     * 跳转到我的评价
     */
  static toMyReviews() {
    wx.navigateTo({
      url: '/pages/user_center/reviews/index'
    })
  }

  /**
     * 跳转到钱包
     */
  static toWallet() {
    wx.navigateTo({
      url: '/pages/user_center/wallet/index'
    })
  }

  /**
   * 跳转到协议中心
   */
  static toAgreementCenter() {
    wx.navigateTo({
      url: '/pages/user_center/agreements/index'
    })
  }

  static toAgreementDetail(type: string, title?: string) {
    let url = `/pages/user_center/agreements/detail/index?type=${type}`
    if (title) url += `&title=${encodeURIComponent(title)}`
    wx.navigateTo({ url })
  }

  /**
   * 跳转到关于我们
   */
  static toAboutUs() {
    wx.navigateTo({
      url: '/pages/user_center/about_us/index'
    })
  }

  /**
   * 跳转到客服中心
   */
  static toServiceCenter() {
    wx.navigateTo({
      url: '/pages/user_center/service_center/index'
    })
  }

  /**
   * 跳转到索赔提交页
   */
  static toSubmitClaim(claimType: string, orderId?: string) {
    let url = `/pages/user_center/service_center/submit/index?claimType=${claimType}`
    if (orderId) url += `&orderId=${orderId}`
    wx.navigateTo({ url })
  }

  /**
   * 跳转到索赔详情页
   */
  static toClaimDetail(claimId: number) {
    wx.navigateTo({
      url: `/pages/user_center/service_center/detail/index?id=${claimId}`
    })
  }


  // ==================== 骑手相关 ====================

  /**
     * 跳转到骑手任务列表
     */
  static toRiderTasks() {
    wx.navigateTo({
      url: '/pages/rider/tasks/index'
    })
  }

  /**
     * 跳转到骑手任务详情
     */
  static toRiderTaskDetail(taskId: string) {
    wx.navigateTo({
      url: `/pages/rider/task-detail/index?id=${taskId}`
    })
  }

  // ==================== 工具方法 ====================

  /**
     * 返回上一页
     */
  static back(delta: number = 1) {
    wx.navigateBack({ delta })
  }

  /**
     * 重定向到指定页面（不可返回）
     */
  static redirectTo(url: string) {
    wx.redirectTo({ url })
  }

  /**
     * 切换到 tabBar 页面
     */
  static switchTab(url: string) {
    wx.switchTab({ url })
  }

  /**
     * 关闭所有页面，打开到应用内的某个页面
     */
  static reLaunch(url: string) {
    wx.reLaunch({ url })
  }
}

export default Navigation
