import {
  getMerchantFinanceOrderStatusView,
  listMerchantFinanceOrders,
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

interface BillRowView {
  id: string
  title: string
  note: string
  amountText: string
  statusText: string
  statusTheme: MerchantFinanceStatusTheme
}

interface BillFetchResult {
  rows: BillRowView[]
  page: number
  totalPages: number
}

function getOrderSourceText(source?: string): string {
  switch (source) {
    case 'takeout':
      return '外卖'
    case 'dine_in':
      return '堂食'
    case 'reservation':
      return '预订'
    case 'takeaway':
      return '自提'
    default:
      return '订单'
  }
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingBills: false,
    rows: [] as BillRowView[],
    page: 1,
    totalPages: 0,
    hasMore: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadBills()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadBills({ silent: true, page: 1 })
  },

  onReachBottom() { void this.onLoadMore() },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingBills) {
      return
    }
    await this.loadBills({ silent: true, page: this.data.page + 1, append: true })
  },

  async loadBills(options: { silent?: boolean, page?: number, append?: boolean } = {}) {
    if (this.data.loadingBills) {
      wx.stopPullDownRefresh()
      return
    }
    const { silent = false, page = 1, append = false } = options
    const hasTrustedData = this.data.rows.length > 0
    this.setData({
      loadingBills: true,
      ...(silent || hasTrustedData ? { refreshErrorMessage: '' } : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    })

    try {
      const range = buildDefaultFinanceRange(30)
      const result = await this.fetchRows(range, page)
      const rows = append ? this.data.rows.concat(result.rows) : result.rows
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingBills: false,
        rows,
        page: result.page,
        totalPages: result.totalPages,
        hasMore: result.page < result.totalPages
      })
    } catch (error) {
      logger.warn('Merchant finance bills load failed', error)
      const message = getMerchantFinanceUserMessage(error, '订单流水加载失败，请稍后重试')
      this.setData({ initialLoading: false, initialError: !hasTrustedData, initialErrorMessage: hasTrustedData ? '' : message, refreshErrorMessage: hasTrustedData ? message : '', loadingBills: false })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  async fetchRows(range: { start_date: string, end_date: string }, page: number): Promise<BillFetchResult> {
    const response = await listMerchantFinanceOrders({ ...range, page, limit: 20 })
    return { rows: response.orders.map((item) => {
      const status = getMerchantFinanceOrderStatusView(item.status)
      return {
        id: `${item.id}`,
        title: `${getOrderSourceText(item.order_source)}入账`,
        note: formatDateTimeText(item.finished_at || item.created_at),
        amountText: formatFenToYuan(item.merchant_amount || 0),
        statusText: status.text,
        statusTheme: status.theme
      }
    }), page: response.page, totalPages: response.total_pages }
  },

  onRetry() { void this.loadBills({ page: 1 }) }
})