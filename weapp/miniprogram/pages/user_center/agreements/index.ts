import { AgreementService, AgreementBrief } from '../../../api/agreement'
import Navigation from '../../../utils/navigation'

Page({
  data: {
    agreements: [] as AgreementBrief[],
    loading: true,
    isRefreshing: false,
    navBarHeight: 0,
    scrollViewHeight: 0
  },

  onLoad() {
    this.fetchAgreements()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    const { navBarHeight } = e.detail
    const windowInfo = wx.getWindowInfo()
    this.setData({
      navBarHeight,
      scrollViewHeight: windowInfo.windowHeight
    })
  },

  async fetchAgreements() {
    if (!this.data.isRefreshing) {
      this.setData({ loading: true })
    }
    
    try {
      const res = await AgreementService.listActiveAgreements()
      this.setData({ agreements: res })
    } catch (err) {
      console.error('Failed to fetch agreements', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ 
        loading: false,
        isRefreshing: false
      })
    }
  },

  onRefresh() {
    this.setData({ isRefreshing: true })
    this.fetchAgreements()
  },

  onAboutUsTap() {
    Navigation.toAboutUs()
  },

  onAgreementTap(e: WechatMiniprogram.TouchEvent) {
    const { type, title } = e.currentTarget.dataset
    Navigation.toAgreementDetail(type, title)
  },

  onPullDownRefresh() {
    this.fetchAgreements().then(() => {
      wx.stopPullDownRefresh()
    })
  }
})
