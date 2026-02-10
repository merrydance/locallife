import { getStableBarHeights } from '../../../utils/responsive'

Page({
  data: {
    orderId: '',
    orderNo: '',
    amount: '0.00',
    navBarHeight: 88
  },

  onLoad(options: { orderId: string; orderNo: string; amount: string }) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      orderId: options.orderId || '',
      orderNo: options.orderNo || '',
      amount: options.amount || '0.00'
    })
  },

  onViewOrder() {
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
