import {
  getMerchantAccountBalance,
  listMerchantWithdrawals,
  type MerchantAccountBalanceResponse
} from '../../../../api/merchant-finance'
import {
  buildAccountBalanceView,
  buildWithdrawalView,
  getMerchantFinanceUserMessage,
  type MerchantAccountBalanceView,
  type MerchantWithdrawalView
} from '../../../../services/merchant-finance-workflow'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

const CREATE_PAGE_PATH = '/pages/merchant/finance/withdrawals/create/index'
const DETAIL_PAGE_PATH = '/pages/merchant/finance/withdrawals/detail/index'

const EMPTY_BALANCE: MerchantAccountBalanceResponse = {
  sub_mch_id: '',
  available_amount: 0,
  pending_amount: 0,
  withdrawable_amount: 0,
  account_status: 'not_configured',
  status_desc: '尚未开通收付通账户'
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
    loadingWithdrawals: false,
    balanceView: buildAccountBalanceView(EMPTY_BALANCE) as MerchantAccountBalanceView,
    withdrawals: [] as MerchantWithdrawalView[],
    total: 0,
    page: 1,
    limit: 20,
    totalPages: 0,
    hasMore: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadWithdrawals({ silent: true, page: 1 })
  },

  onReachBottom() { void this.onLoadMore() },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingWithdrawals) {
      return
    }
    await this.loadWithdrawals({ silent: true, page: this.data.page + 1, append: true })
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

    this.setData({ accessReady: true, accessDenied: false, accessErrorMessage: '' })
    await this.loadWithdrawals({ page: 1 })
  },

  async loadWithdrawals(options: { silent?: boolean, page?: number, append?: boolean } = {}) {
    if (this.data.loadingWithdrawals) {
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false, page = 1, append = false } = options
    const hasTrustedData = this.data.withdrawals.length > 0 || this.data.total > 0
    this.setData({
      loadingWithdrawals: true,
      ...(silent || hasTrustedData
        ? { refreshErrorMessage: '' }
        : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    })

    try {
      const [balance, response] = await Promise.all([
        getMerchantAccountBalance(),
        listMerchantWithdrawals(page, this.data.limit)
      ])
      const withdrawals = response.withdrawals.map(buildWithdrawalView)
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingWithdrawals: false,
        balanceView: buildAccountBalanceView(balance),
        withdrawals: append ? this.data.withdrawals.concat(withdrawals) : withdrawals,
        total: response.total,
        page: response.page,
        limit: response.limit,
        totalPages: response.total_pages,
        hasMore: response.page < response.total_pages
      })
    } catch (error) {
      logger.warn('Merchant withdrawals load failed', error)
      const message = getMerchantFinanceUserMessage(error, '提现记录加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingWithdrawals: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.loadWithdrawals({ page: 1 })
  },

  onCreateWithdrawal() {
    wx.navigateTo({ url: CREATE_PAGE_PATH })
  },

  onTapWithdrawal(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.id)
    if (!Number.isFinite(id) || id <= 0) {
      return
    }
    wx.navigateTo({ url: `${DETAIL_PAGE_PATH}?id=${id}` })
  }
})