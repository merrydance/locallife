Page({
  data: {
    navBarHeight: 88,
    applymentId: '',
    message: '开户申请已提交，微信支付通常会在1-3个工作日内完成审核。',
    tips: [
      '审核期间无需重复提交开户信息。',
      '如需查看最新进度，可返回开户页下拉刷新。'
    ] as string[]
  },

  onLoad(options: { applymentId?: string, message?: string }) {
    const message = options.message ? decodeURIComponent(options.message) : this.data.message
    const applymentId = options.applymentId ? decodeURIComponent(options.applymentId) : ''

    this.setData({
      message,
      applymentId
    })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onBackToApplyment() {
    wx.redirectTo({
      url: '/pages/operator/applyment/index'
    })
  },

  onBackToDashboard() {
    wx.redirectTo({
      url: '/pages/operator/dashboard/index'
    })
  }
})