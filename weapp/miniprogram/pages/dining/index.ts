Page({
  data: {
    navBarHeight: 88
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onLoad(options: { table_id?: string, scene?: string }) {
    if (options.scene) {
      wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?scene=${encodeURIComponent(options.scene)}` })
      return
    }

    if (options.table_id) {
      wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${options.table_id}` })
      return
    }

    wx.redirectTo({ url: '/pages/user_center/index' })
  }
})