import {
  getMerchantAccountBalance,
  type MerchantAccountBalanceResponse
} from '../../../api/merchant-finance'
import {
  buildAccountBalanceView,
  getMerchantFinanceUserMessage
} from '../../../services/merchant-finance-workflow'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

const FINANCE_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const WITHDRAW_CREATE_PAGE_PATH = '/pages/merchant/finance/withdrawals/create/index'
const WITHDRAWALS_PAGE_PATH = '/pages/merchant/finance/withdrawals/index'
const BILLS_PAGE_PATH = '/pages/merchant/finance/bills/index'
const SETTLEMENTS_PAGE_PATH = '/pages/merchant/finance/settlements/index'
const SETTLEMENT_ACCOUNT_PAGE_PATH = '/pages/merchant/settings/applyment/settlement-account/index'

const EMPTY_BALANCE: MerchantAccountBalanceResponse = {
  sub_mch_id: '',
  available_amount: 0,
  pending_amount: 0,
  withdrawable_amount: 0,
  account_status: 'not_configured',
  status_desc: '尚未开通收付通账户'
}

interface FinanceEntryView {
  id: string
  title: string
  icon: string
  path: string
}

function buildFinanceEntries(): FinanceEntryView[] {
  return [
    {
      id: 'withdrawals',
      title: '提现记录',
      icon: 'money',
      path: WITHDRAWALS_PAGE_PATH
    },
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
      title: '结算账户',
      icon: 'creditcard',
      path: SETTLEMENT_ACCOUNT_PAGE_PATH
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
    balanceView: buildAccountBalanceView(EMPTY_BALANCE),
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
      const balance = await getMerchantAccountBalance()
      const balanceView = buildAccountBalanceView(balance)

      const loadedAt = Date.now()
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingFinance: false,
        lastLoadedAt: loadedAt,
        balanceView,
        entries: buildFinanceEntries()
      })
    } catch (error) {
      logger.warn('Merchant finance account balance load failed', error)
      const message = getMerchantFinanceUserMessage(error, '财务信息加载失败，请稍后重试')
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
  },

  onCreateWithdrawal() {
    wx.navigateTo({ url: WITHDRAW_CREATE_PAGE_PATH })
  }
})