import {
  getBaofuWithdrawalBalance,
  listBaofuWithdrawals,
  type BaofuWithdrawalBalanceResponse,
  type BaofuWithdrawalItem
} from '../../../../api/baofu-withdrawal'
import {
  buildBaofuWithdrawalBalanceView,
  buildBaofuWithdrawalItemView,
  isBaofuWithdrawalRequestFulfilled,
  isBaofuWithdrawalRequestRejected,
  settleBaofuWithdrawalRequest,
  withdrawalBalanceUnavailableView,
  type BaofuWithdrawalBalanceView,
  type BaofuWithdrawalItemView
} from '../../../../services/baofu-withdrawal-workflow'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const PAGE_SIZE = 20
const CREATE_PAGE_PATH = '/pages/rider/income/withdrawals/create/index'
const DETAIL_PAGE_PATH = '/pages/rider/income/withdrawals/detail/index'

interface RiderIncomeWithdrawalFetchResult {
  balance?: BaofuWithdrawalBalanceResponse
  balanceErrorMessage: string
  withdrawals: BaofuWithdrawalItem[]
  recordsErrorMessage: string
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
    balanceErrorMessage: '',
    recordsErrorMessage: '',
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
            refreshErrorMessage: '',
            balanceErrorMessage: '',
            recordsErrorMessage: ''
          })
    })

    try {
      const result = await this.fetchWithdrawals(page)
      const nextRows = result.withdrawals.map(buildBaofuWithdrawalItemView)
      const rows = append ? this.data.rows.concat(nextRows) : nextRows
      const balanceView = result.balance
        ? buildBaofuWithdrawalBalanceView(result.balance)
        : withdrawalBalanceUnavailableView(result.balanceErrorMessage)

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: result.recordsErrorMessage,
        balanceErrorMessage: result.balanceErrorMessage,
        recordsErrorMessage: result.recordsErrorMessage,
        loadingWithdrawals: false,
        balanceView,
        rows,
        page: result.page,
        totalPages: result.totalPages,
        hasMore: result.page < result.totalPages
      })
    } catch (error) {
      logger.warn('Rider income baofu withdrawals load failed', error)
      const message = getErrorUserMessage(error, '收入提现信息加载失败，请稍后重试')
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

  async fetchWithdrawals(page: number): Promise<RiderIncomeWithdrawalFetchResult> {
    const [balanceResult, recordsResult] = await Promise.all([
      settleBaofuWithdrawalRequest(getBaofuWithdrawalBalance('rider')),
      settleBaofuWithdrawalRequest(listBaofuWithdrawals('rider', { page, limit: PAGE_SIZE }))
    ])

    if (isBaofuWithdrawalRequestRejected(balanceResult)) {
      logger.warn('Rider income baofu withdrawal balance load failed', balanceResult.reason)
    }
    if (isBaofuWithdrawalRequestRejected(recordsResult)) {
      logger.warn('Rider income baofu withdrawals records load failed', recordsResult.reason)
    }
    if (isBaofuWithdrawalRequestRejected(balanceResult) && isBaofuWithdrawalRequestRejected(recordsResult)) {
      throw new Error('收入提现信息加载失败，请稍后重试')
    }

    const balance = isBaofuWithdrawalRequestFulfilled(balanceResult) ? balanceResult.value : undefined
    const records = isBaofuWithdrawalRequestFulfilled(recordsResult) ? recordsResult.value : undefined

    return {
      balance,
      balanceErrorMessage: balance ? '' : getErrorUserMessage(
        isBaofuWithdrawalRequestRejected(balanceResult) ? balanceResult.reason : undefined,
        '可提现余额暂不可确认，提现申请已暂停，提现记录可继续查看'
      ),
      withdrawals: records?.withdrawals || [],
      recordsErrorMessage: records ? '' : getErrorUserMessage(
        isBaofuWithdrawalRequestRejected(recordsResult) ? recordsResult.reason : undefined,
        '提现记录加载失败，请稍后刷新'
      ),
      page: records?.page || page,
      totalPages: records?.total_pages || 0
    }
  },

  onRetry() {
    void this.loadWithdrawals({ page: 1 })
  },

  onOpenCreate() {
    if (this.data.balanceErrorMessage) {
      wx.showToast({
        title: this.data.balanceErrorMessage,
        icon: 'none'
      })
      return
    }
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
