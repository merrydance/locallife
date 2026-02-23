import { AgreementService, AgreementDetail } from '../../../../api/agreement'

Page({
  data: {
    agreement: {} as AgreementDetail,
    loading: true,
    title: '协议详情',
    navBarHeight: 0,
    scrollViewHeight: 0
  },

  onLoad(options: { type: string, title?: string }) {
    if (options.title) {
      let title = decodeURIComponent(options.title)
      // Simplify common title suffixes
      title = title.replace(/(协议|政策|说明|规范)$/, '')
      this.setData({ title })
    }
    
    if (options.type) {
      this.fetchAgreementDetail(options.type)
    } else {
      wx.showToast({ title: '参数错误', icon: 'none' })
      setTimeout(() => wx.navigateBack(), 1500)
    }
  },

  onNavHeight(e: any) {
    const { navBarHeight } = e.detail
    const windowInfo = wx.getWindowInfo()
    this.setData({
      navBarHeight,
      scrollViewHeight: windowInfo.windowHeight - navBarHeight
    })
  },

  async fetchAgreementDetail(type: string) {
    this.setData({ loading: true })
    try {
      const res = await AgreementService.getAgreement(type)
      
      // SQL Date might need formatting if it's not a simple string
      if (res.published_on && typeof res.published_on === 'string') {
        res.published_on = res.published_on.split('T')[0]
      }

      this.setData({ agreement: res })
    } catch (err) {
      console.error('Failed to fetch agreement detail', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  }
})
