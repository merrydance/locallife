import { getMerchantApplymentStatus } from '../../../../../api/merchant-finance'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../../utils/console-access'

function isCompletedStatus(status?: string) {
  return status === 'finish' || status === 'active'
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    loading: true,
    error: '',
    completed: false,
    subMchId: '',
    statusDesc: ''
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ loading: false })
      return
    }

    void this.loadStatus()
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      loading: true,
      error: ''
    })
    void this.onLoad()
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
    wx.redirectTo({ url: '/pages/merchant/settings/applyment/index?allowCompletedView=1' })
  }
})
