import {
  getMerchantFinanceOrderStatusView,
  listMerchantSettlements,
  type MerchantSettlementItem,
  type MerchantSettlementsResponse
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
type SettlementStatusFilter = 'all' | 'pending' | 'processing' | 'finished' | 'failed'

const STATUS_OPTIONS = [
  { key: 'all', label: '全部' },
  { key: 'pending', label: '待结算' },
  { key: 'processing', label: '处理中' },
  { key: 'finished', label: '已完成' },
  { key: 'failed', label: '失败' }
] as const

const EMPTY_SUMMARY: MerchantSettlementsResponse = {
  settlements: [],
  total: 0,
  page: 1,
  limit: 20,
  total_pages: 0,
  total_amount: 0,
  total_merchant_amount: 0,
  total_operator_fee: 0,
  total_platform_fee: 0
}

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
    currentStatus: 'all' as SettlementStatusFilter,
    rangeOptions: FINANCE_RANGE_OPTIONS,
    statusOptions: STATUS_OPTIONS,
    dateRangeLabel: '--',
    updatedAtLabel: '--',
    summary: EMPTY_SUMMARY as MerchantSettlementsResponse,
    settlements: [] as MerchantSettlementItem[]
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
        accessErrorMessage: access.message || '当前账号暂时不能查看结算记录',
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
    const hasData = this.data.settlements.length > 0

    this.setData({
      loading: true,
      initialLoading: !hasData,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      dateRangeLabel: label
    })

    try {
      const response = await listMerchantSettlements({
        ...params,
        page: 1,
        limit: 20,
        ...(this.data.currentStatus !== 'all' ? { status: this.data.currentStatus } : {})
      })

      this.setData({
        summary: response,
        settlements: response.settlements || [],
        loading: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        updatedAtLabel: formatDateTime(new Date().toISOString()).slice(11, 16)
      })
    } catch (error) {
      logger.error('Load merchant finance settlements failed', error, 'merchant-finance-settlements')
      const message = getErrorUserMessage(error, '结算记录加载失败，请稍后重试')
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

  onSelectStatus(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: SettlementStatusFilter }
    if (!key || key === this.data.currentStatus) {
      return
    }

    this.setData({ currentStatus: key }, () => {
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