import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

const FINANCE_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const BILLS_PAGE_PATH = '/pages/merchant/finance/bills/index'
const SETTLEMENTS_PAGE_PATH = '/pages/merchant/finance/settlements/index'
const BAOFU_SETTLEMENT_ACCOUNT_PAGE_PATH = '/pages/merchant/finance/settlement-account/index'

interface FinanceEntryView {
  id: string
  title: string
  icon: string
  path: string
}

function buildFinanceEntries(): FinanceEntryView[] {
  return [
    {
      id: 'bills',
      title: '订单流水',
      icon: 'chart-bar',
      path: BILLS_PAGE_PATH
    },
    {
      id: 'settlements',
      title: '结算',
      icon: 'time',
      path: SETTLEMENTS_PAGE_PATH
    },
    {
      id: 'settlement-account',
      title: '宝付结算账户',
      icon: 'creditcard',
      path: BAOFU_SETTLEMENT_ACCOUNT_PAGE_PATH
    }
  ]
}

function shouldAutoRefresh(lastLoadedAt: number): boolean {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= FINANCE_AUTO_REFRESH_WINDOW_MS
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingFinance: false,
    lastLoadedAt: 0,
    entries: buildFinanceEntries()
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.initialLoading) {
      return
    }

    if (!shouldAutoRefresh(this.data.lastLoadedAt)) {
      return
    }

    await this.loadFinance({ silent: true })
  },

  onPullDownRefresh() {
    void this.loadFinance({ silent: true, force: true })
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })

    const accessResult = await ensureMerchantConsoleAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessErrorMessage: ''
    })

    await this.loadFinance({ force: true })
  },

  async loadFinance(options: { silent?: boolean, force?: boolean } = {}) {
    const { silent = false, force = false } = options
    if (this.data.loadingFinance) {
      wx.stopPullDownRefresh()
      return
    }

    const hasTrustedData = this.data.lastLoadedAt > 0
    if (!force && hasTrustedData && !shouldAutoRefresh(this.data.lastLoadedAt)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loadingFinance: true,
      ...(silent || hasTrustedData
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const loadedAt = Date.now()
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingFinance: false,
        lastLoadedAt: loadedAt,
        entries: buildFinanceEntries()
      })
    } catch (error) {
      logger.warn('Merchant finance guidance load failed', error)
      const message = '财务信息加载失败，请稍后重试'
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingFinance: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.loadFinance({ force: true })
  },

  onManualRefresh() {
    void this.loadFinance({ silent: this.data.lastLoadedAt > 0, force: true })
  }
})
