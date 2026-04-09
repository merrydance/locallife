import {
  getMerchantFinanceOrderStatusView,
  listMerchantFinanceOrders,
  type MerchantFinanceOrderItem
} from '../../../../api/merchant-finance-bills'
import {
  buildFinanceRange,
  FINANCE_RANGE_OPTIONS,
  formatAmountText,
  formatDateTime,
  getOrderSourceText
} from '../analysis-shared'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type FinanceRangeKey = '7d' | '30d'

function resolveRangeKey(range?: string): FinanceRangeKey {
  return range === '30d' ? '30d' : '7d'
}

Page({
  data: {
    navBarHeight: 0,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    currentRange: '7d' as FinanceRangeKey,
    rangeOptions: FINANCE_RANGE_OPTIONS,
    dateRangeLabel: '--',
    updatedAtLabel: '--',
    orders: [] as MerchantFinanceOrderItem[],
    totalMerchantAmount: 0,
    totalAmount: 0,
    totalPlatformCommission: 0,
    totalOperatorCommission: 0
  },

  async onLoad(options?: { range?: string }) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight, currentRange: resolveRangeKey(options?.range) })
    await this.bootstrapPage()
  },

  async onPullDownRefresh() {
    if (this.data.accessReady && !this.data.accessDenied) {
      await this.loadData({ force: true })
    }
    wx.stopPullDownRefresh()
  },

  async bootstrapPage() {
    const access = await ensureMerchantConsoleAccess()

    if (access.status !== 'granted') {
      this.setData({
        accessReady: true,
        accessDenied: true,
        accessErrorMessage: access.message || '当前账号暂时不能查看订单收入',
        initialLoading: false
      })
      return
    }

    this.setData({ accessReady: true, accessDenied: false, accessErrorMessage: '' })
    await this.loadData({ force: true })
  },

  async loadData(options?: { force?: boolean }) {
    const { force = false } = options || {}
    if (!force && this.data.loading) {
      return
    }

    const { params, label } = buildFinanceRange(this.data.currentRange)
    const hasData = this.data.orders.length > 0

    this.setData({
      loading: true,
      initialLoading: !hasData,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      dateRangeLabel: label
    })

    try {
      const response = await listMerchantFinanceOrders({ ...params, page: 1, limit: 20 })
      const orders = response.orders || []
      const summary = orders.reduce(
        (acc, item) => ({
          totalMerchantAmount: acc.totalMerchantAmount + (item.merchant_amount || 0),
          totalAmount: acc.totalAmount + (item.total_amount || 0),
          totalPlatformCommission: acc.totalPlatformCommission + (item.platform_commission || 0),
          totalOperatorCommission: acc.totalOperatorCommission + (item.operator_commission || 0)
        }),
        {
          totalMerchantAmount: 0,
          totalAmount: 0,
          totalPlatformCommission: 0,
          totalOperatorCommission: 0
        }
      )

      this.setData({
        orders,
        ...summary,
        loading: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        updatedAtLabel: formatDateTime(new Date().toISOString()).slice(11, 16)
      })
    } catch (error) {
      logger.error('Load merchant finance orders failed', error, 'merchant-finance-orders')
      const message = getErrorUserMessage(error, '订单收入加载失败，请稍后重试')
      this.setData({
        loading: false,
        initialLoading: false,
        initialError: !hasData,
        initialErrorMessage: !hasData ? message : '',
        refreshErrorMessage: hasData ? `${message}，当前已保留上次同步结果` : ''
      })
    }
  },

  onSelectRange(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: FinanceRangeKey }
    if (!key || key === this.data.currentRange) {
      return
    }

    this.setData({ currentRange: key }, () => {
      void this.loadData({ force: true })
    })
  },

  onRetry() {
    void this.loadData({ force: true })
  },

  onBack() {
    wx.navigateBack({ delta: 1 })
  },

  formatAmountText,
  formatDateTime,
  getOrderSourceText,

  getFinanceOrderStatusText(status?: string) {
    return getMerchantFinanceOrderStatusView(status).text
  },

  getFinanceOrderStatusTheme(status?: string) {
    return getMerchantFinanceOrderStatusView(status).theme
  }
})