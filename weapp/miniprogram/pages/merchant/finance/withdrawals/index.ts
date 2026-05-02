import { getStableBarHeights } from '../../../../utils/responsive'

const FINANCE_PAGE_PATH = '/pages/merchant/finance/index'

Page({
  data: {
    navBarHeight: 88
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onBackToFinance() {
    wx.redirectTo({ url: FINANCE_PAGE_PATH })
  }
})
