import { AgreementService, AgreementDetail } from '../../../../api/agreement'

function stripAgreementHtmlHeader(html: string): string {
  if (!html) return html

  // Remove the first document title inside the HTML to avoid duplicate titles
  // (navbar title + page title + <h1> in rich-text).
  let out = html.replace(/<h1\b[^>]*>[\s\S]*?<\/h1>/i, '')

  // Remove the publish-date paragraph if present; the page already shows published_on.
  out = out.replace(/<p\b[^>]*class=["']publish-date["'][^>]*>[\s\S]*?<\/p>/i, '')

  return out
}

Page({
  data: {
    agreement: {} as AgreementDetail,
    loading: true,
    title: '协议详情',
    navBarHeight: 0,
    scrollViewHeight: 0
  },

  onLoad(options: { type: string, title?: string }) {
    // Keep navbar title short to avoid wrapping; show the real title in the page body.
    this.setData({ title: '协议详情' })

    if (options.type) {
      this.fetchAgreementDetail(options.type)
    } else {
      wx.showToast({ title: '参数错误', icon: 'none' })
      setTimeout(() => wx.navigateBack(), 1500)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
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

      if (res.content && typeof res.content === 'string') {
        res.content = stripAgreementHtmlHeader(res.content)
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
