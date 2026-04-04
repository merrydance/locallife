import { getOperatorApplymentStatus } from '../../../../api/operator-applyment'
import { getErrorUserMessage } from '../../../../utils/user-facing'

function isCompletedStatus(status?: string) {
  return status === 'finish'
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    error: '',
    completed: false,
    applymentId: '',
    subMchId: '',
    statusDesc: ''
  },

  onLoad() {
    void this.loadStatus()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadStatus() {
    this.setData({ loading: true, error: '' })
    try {
      const status = await getOperatorApplymentStatus()
      this.setData({
        completed: isCompletedStatus(status.status),
        applymentId: status.applyment_id ? String(status.applyment_id) : '',
        subMchId: status.sub_mch_id || '',
        statusDesc: status.status_desc || '微信支付商户已开通'
      })
    } catch (error: unknown) {
      this.setData({
        error: getErrorUserMessage(error, '加载开户完成信息失败，请稍后重试')
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onRetry() {
    void this.loadStatus()
  },

  onBackToApplyment() {
    wx.redirectTo({ url: '/pages/operator/applyment/index' })
  },

  onBackToDashboard() {
    wx.redirectTo({ url: '/pages/operator/dashboard/index' })
  }
})
