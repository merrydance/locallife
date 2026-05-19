import {
  getBaofuWithdrawalBalance,
  listBaofuWithdrawals,
  type BaofuWithdrawalBalanceResponse,
  type BaofuWithdrawalItem
} from '../../../../api/baofu-withdrawal'
import {
  buildBaofuWithdrawalBalanceView,
  buildBaofuWithdrawalItemView,
  type BaofuWithdrawalBalanceView,
  type BaofuWithdrawalItemView
} from '../../../../services/baofu-withdrawal-workflow'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const PAGE_SIZE = 20
const CREATE_PAGE_PATH = '/pages/operator/finance/withdrawals/create/index'
const DETAIL_PAGE_PATH = '/pages/operator/finance/withdrawals/detail/index'

interface OperatorWithdrawalFetchResult {
  balance: BaofuWithdrawalBalanceResponse
  withdrawals: BaofuWithdrawalItem[]
  page: number
  totalPages: number
}

const EMPTY_BALANCE_VIEW = buildBaofuWithdrawalBalanceView(null)

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingWithdrawals: false,
    balanceView: EMPTY_BALANCE_VIEW as BaofuWithdrawalBalanceView,
    rows: [] as BaofuWithdrawalItemView[],
    page: 1,
    totalPages: 0,
    hasMore: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadWithdrawals()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadWithdrawals({ silent: true, page: 1 })
  },

  onReachBottom() {
    void this.onLoadMore()
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingWithdrawals) {
      return
    }
    await this.loadWithdrawals({ silent: true, append: true, page: this.data.page + 1 })
  },

  async loadWithdrawals(options: { silent?: boolean, append?: boolean, page?: number } = {}) {
    if (this.data.loadingWithdrawals) {
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false, append = false, page = 1 } = options
    const hasTrustedData = this.data.rows.length > 0 || this.data.balanceView.availableAmount > 0

    this.setData({
      loadingWithdrawals: true,
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
      const result = await this.fetchWithdrawals(page)
      const nextRows = result.withdrawals.map(buildBaofuWithdrawalItemView)
      const rows = append ? this.data.rows.concat(nextRows) : nextRows

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingWithdrawals: false,
        balanceView: buildBaofuWithdrawalBalanceView(result.balance),
        rows,
        page: result.page,
        totalPages: result.totalPages,
        hasMore: result.page < result.totalPages
      })
    } catch (error) {
      logger.warn('Operator baofu withdrawals load failed', error)
      const message = getErrorUserMessage(error, '提现信息加载失败，请稍后重试')
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

  async fetchWithdrawals(page: number): Promise<OperatorWithdrawalFetchResult> {
    const [balance, records] = await Promise.all([
      getBaofuWithdrawalBalance('operator'),
      listBaofuWithdrawals('operator', { page, limit: PAGE_SIZE })
    ])

    return {
      balance,
      withdrawals: records.withdrawals || [],
      page: records.page || page,
      totalPages: records.total_pages || 0
    }
  },

  onRetry() {
    void this.loadWithdrawals({ page: 1 })
  },

  onOpenCreate() {
    if (!this.data.balanceView.canSubmit) {
      wx.showToast({
        title: this.data.balanceView.disabledReason || '当前暂不能提现',
        icon: 'none'
      })
      return
    }
    wx.navigateTo({ url: CREATE_PAGE_PATH })
  },

  onOpenDetail(e: WechatMiniprogram.BaseEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) {
      return
    }
    wx.navigateTo({ url: `${DETAIL_PAGE_PATH}?id=${id}` })
  }
})
