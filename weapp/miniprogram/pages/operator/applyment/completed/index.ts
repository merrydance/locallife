import {
  buildOperatorApplymentStatusView,
  getOperatorApplymentStatus
} from '../../../../api/operator-applyment'
import { getOperatorAccountBalance } from '../../../../api/operator-finance'
import { getErrorUserMessage } from '../../../../utils/user-facing'

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
      const statusView = buildOperatorApplymentStatusView(status)
      let subMchId = status.sub_mch_id || ''

      if (statusView.isOpened && !subMchId) {
        try {
          const balance = await getOperatorAccountBalance()
          subMchId = balance.sub_mch_id || ''
        } catch {
          // 资金账户接口只做兜底，不阻断完成页主流程。
        }
      }

      this.setData({
        completed: statusView.isOpened,
        applymentId: status.applyment_id ? String(status.applyment_id) : '',
        subMchId,
        statusDesc: statusView.statusDesc || '微信支付商户已开通'
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
