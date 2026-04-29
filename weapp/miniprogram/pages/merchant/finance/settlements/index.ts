import {
  getMerchantFinanceOrderStatusView,
  listMerchantSettlements,
  type MerchantFinanceStatusTheme
} from '../../../../api/merchant-finance'
import {
  buildDefaultFinanceRange,
  formatDateTimeText,
  formatFenToYuan,
  getMerchantFinanceUserMessage
} from '../../../../services/merchant-finance-workflow'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

interface SettlementRowView {
  id: string
  title: string
  note: string
  amountText: string
  statusText: string
  statusTheme: MerchantFinanceStatusTheme
}

interface SettlementFetchResult {
  rows: SettlementRowView[]
  summaryText: string
  page: number
  totalPages: number
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingSettlements: false,
    rows: [] as SettlementRowView[],
    summaryText: '',
    page: 1,
    totalPages: 0,
    hasMore: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadSettlements()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() { void this.loadSettlements({ silent: true, page: 1 }) },

  onReachBottom() { void this.onLoadMore() },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingSettlements) {
      return
    }
    await this.loadSettlements({ silent: true, page: this.data.page + 1, append: true })
  },

  async loadSettlements(options: { silent?: boolean, page?: number, append?: boolean } = {}) {
    if (this.data.loadingSettlements) {
      wx.stopPullDownRefresh()
      return
    }
    const { silent = false, page = 1, append = false } = options
    const hasTrustedData = this.data.rows.length > 0
    this.setData({ loadingSettlements: true, ...(silent || hasTrustedData ? { refreshErrorMessage: '' } : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }) })

    try {
      const range = buildDefaultFinanceRange(30)
      const result = await this.fetchRows(range, page)
      const rows = append ? this.data.rows.concat(result.rows) : result.rows
      this.setData({
        rows,
        summaryText: result.summaryText,
        page: result.page,
        totalPages: result.totalPages,
        hasMore: result.page < result.totalPages,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingSettlements: false
      })
    } catch (error) {
      logger.warn('Merchant settlements load failed', error)
      const message = getMerchantFinanceUserMessage(error, '结算记录加载失败，请稍后重试')
      this.setData({ initialLoading: false, initialError: !hasTrustedData, initialErrorMessage: hasTrustedData ? '' : message, refreshErrorMessage: hasTrustedData ? message : '', loadingSettlements: false })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  async fetchRows(range: { start_date: string, end_date: string }, page: number): Promise<SettlementFetchResult> {
    const response = await listMerchantSettlements({ ...range, page, limit: 20 })
    return {
      rows: response.settlements.map((item) => {
        const status = getMerchantFinanceOrderStatusView(item.status)
        return {
          id: `${item.id}`,
          title: item.out_order_no || '结算单',
          note: formatDateTimeText(item.finished_at || item.created_at),
          amountText: formatFenToYuan(item.merchant_amount || 0),
          statusText: status.text,
          statusTheme: status.theme
        }
      }),
      summaryText: `已结算入账 ${formatFenToYuan(response.total_merchant_amount || 0)}`,
      page: response.page,
      totalPages: response.total_pages
    }
  },

  onRetry() { void this.loadSettlements({ page: 1 }) }
})