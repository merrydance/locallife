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
  static toRestaurantDetail(merchantId: string) {
    wx.navigateTo({
      url: `/pages/takeout/restaurant-detail/index?id=${merchantId}`
    })
  }

  /**
     * 跳转到购物车页
     */
  static toCart() {
    wx.navigateTo({
      url: '/pages/takeout/cart/index'
    })
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
     * 跳转到订单列表页
     */
  static toOrderList(tab?: 'all' | 'pending' | 'completed') {
    const url = tab
      ? `/pages/orders/list/index?tab=${tab}`
      : '/pages/orders/list/index'
    wx.navigateTo({ url })
  }

  /**
     * 跳转到评价页
     * TODO: 页面 pages/orders/review/index 不存在
     */
  static toReview(orderId: string) {
    wx.navigateTo({
      url: `/pages/orders/review/index?order_id=${orderId}`
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
     * 跳转到积分中心
     */
  static toPoints(merchantId?: string) {
    const url = merchantId
      ? `/pages/user_center/points/index?merchant_id=${merchantId}`
      : '/pages/user_center/points/index'
    wx.navigateTo({ url })
  }

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
  static toMembership() {
    wx.navigateTo({
      url: '/pages/user_center/membership/index'
    })
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
     * 跳转到信用分页
     */
  static toCredit() {
    wx.navigateTo({
      url: '/pages/user_center/credit/index'
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
