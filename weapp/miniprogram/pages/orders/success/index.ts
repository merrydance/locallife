import { getStableBarHeights } from '../../../utils/responsive'

Page({
  data: {
    orderId: '',
    orderNo: '',
    amount: '0.00',
    isCombined: false,
    orderCount: 0,
    successTitle: '支付成功',
    successDescription: '您的订单已支付完成，商家正在处理中',
    primaryButtonText: '查看订单',
    navBarHeight: 88
  },

  onLoad(options: { orderId: string, orderNo: string, amount: string, combined?: string, orderCount?: string }) {
    const { navBarHeight } = getStableBarHeights()
    const isCombined = options.combined === '1'
    const orderCount = Number(options.orderCount || '0') || 0
    this.setData({
      navBarHeight,
      orderId: options.orderId || '',
      orderNo: options.orderNo || '',
      amount: options.amount || '0.00',
      isCombined,
      orderCount,
      successTitle: isCombined ? '合并支付成功' : '支付成功',
      successDescription: isCombined
        ? `已完成${orderCount > 0 ? `${orderCount}笔` : ''}订单合并支付，商家正在处理中`
        : '您的订单已支付完成，商家正在处理中',
      primaryButtonText: isCombined ? '查看订单列表' : '查看订单'
    })
  },

  onViewOrder() {
    if (this.data.isCombined) {
      wx.redirectTo({
        url: '/pages/orders/list/index'
      })
      return
    }

    wx.redirectTo({
      url: `/pages/orders/detail/index?id=${this.data.orderId}`
    })
  },

  onGoHome() {
    wx.switchTab({
      url: '/pages/takeout/index'
    })
  }
})
