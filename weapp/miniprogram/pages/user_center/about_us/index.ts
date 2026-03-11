import { APP_VERSION } from '../../../config/index'

Page({
  data: {
    navBarHeight: 0,
    scrollViewHeight: 0,
    version: APP_VERSION
  },

  onLoad() {},


  onNavHeight(e: any) {
    const { navBarHeight } = e.detail
    const windowInfo = wx.getWindowInfo()
    this.setData({
      navBarHeight,
      scrollViewHeight: windowInfo.windowHeight - navBarHeight
    })
  }
})
