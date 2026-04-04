import { getMerchantApplymentStatus } from '../../../../../api/merchant-finance'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

function isCompletedStatus(status?: string) {
  return status === 'finish' || status === 'active'
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    error: '',
    completed: false,
    subMchId: '',
    statusDesc: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.loadStatus()
  },

  async loadStatus() {
    this.setData({ loading: true, error: '' })
    try {
      const status = await getMerchantApplymentStatus()
      this.setData({
        completed: isCompletedStatus(status.status),
        subMchId: status.sub_mch_id || '',
        statusDesc: status.status_desc || '收付通已开通'
      })
    } catch (error: unknown) {
      this.setData({
        error: getErrorUserMessage(error, '加载开通结果失败，请稍后重试')
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onRetry() {
    void this.loadStatus()
  },

  onGoFinance() {
    wx.redirectTo({ url: '/pages/merchant/finance/index' })
  },

  onBackToApplyment() {
    wx.redirectTo({ url: '/pages/merchant/settings/applyment/index' })
  }
})
