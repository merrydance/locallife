Page({
  data: {
    navBarHeight: 88
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSelectStore() {
    wx.navigateTo({ url: './store/index' })
  },

  onSelectGroup() {
    wx.navigateTo({ url: './group/index' })
  },

  onJoinGroup() {
    wx.navigateTo({ url: './join-group/index' })
  }
})
