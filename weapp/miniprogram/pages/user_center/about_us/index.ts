Page({
  data: {
    navBarHeight: 0,
    scrollViewHeight: 0
  },

  onLoad() {
    // Initialization logic if needed
  },

  onNavHeight(e: any) {
    const { navBarHeight } = e.detail
    const windowInfo = wx.getWindowInfo()
    this.setData({
      navBarHeight,
      scrollViewHeight: windowInfo.windowHeight - navBarHeight
    })
  }
})
