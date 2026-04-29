import { getMerchantCancelWithdrawApplication } from '../../../../../api/merchant-finance'
import {
  buildCancelWithdrawApplicationView,
  getMerchantFinanceUserMessage,
  type MerchantCancelWithdrawApplicationView
} from '../../../../../services/merchant-finance-workflow'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    id: 0,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingApplication: false,
    application: null as MerchantCancelWithdrawApplicationView | null
  },

  async onLoad(query: { id?: string }) {
    const { navBarHeight } = getStableBarHeights()
    const id = Number(query.id)
    this.setData({ navBarHeight, id: Number.isFinite(id) ? id : 0 })
    await this.loadApplication()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadApplication({ silent: true })
  },

  async loadApplication(options: { silent?: boolean } = {}) {
    if (this.data.loadingApplication) {
      wx.stopPullDownRefresh()
      return
    }
    if (!this.data.id) {
      this.setData({ initialLoading: false, initialError: true, initialErrorMessage: '注销提现申请不存在' })
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false } = options
    const hasTrustedData = !!this.data.application
    this.setData({
      loadingApplication: true,
      ...(silent || hasTrustedData ? { refreshErrorMessage: '' } : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    })

    try {
      const application = buildCancelWithdrawApplicationView(await getMerchantCancelWithdrawApplication(this.data.id))
      this.setData({
        application,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingApplication: false
      })
    } catch (error) {
      logger.warn('Merchant cancel withdraw application load failed', error)
      const message = getMerchantFinanceUserMessage(error, '注销提现申请加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingApplication: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() { void this.loadApplication() },

  onCopyConfirmUrl() {
    const url = this.data.application?.confirmCancelUrl || ''
    if (!url) {
      return
    }
    wx.setClipboardData({ data: url })
  }
})