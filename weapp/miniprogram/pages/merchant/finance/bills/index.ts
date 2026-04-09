import {
  getMerchantDailyFinance,
  getMerchantFinanceOverview,
  getMerchantPromotionExpenses,
  getMerchantServiceFees,
  listMerchantSettlementTimeline,
  listMerchantSettlements,
  type MerchantDailyFinanceItem,
  type MerchantFinanceOverviewResponse,
  type MerchantPromotionExpensesResponse,
  type MerchantServiceFeeSummaryResponse,
  type MerchantSettlementTimelineItem,
  type MerchantSettlementsResponse
} from '../../../../api/merchant-finance-bills'
import {
  buildFinanceRange,
  FINANCE_RANGE_OPTIONS,
  formatAmountText,
  formatDateTime
} from '../analysis-shared'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { isSettledFulfilled, isSettledRejected, settleAll } from '../../../../utils/promise'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type FinanceRangeKey = '7d' | '30d'

type OverviewTile = {
  id: string
  label: string
  value: string
  icon: {
    name: string
    color: string
    size: string
  }
}

type AnalysisEntry = {
  id: string
  title: string
  value: string
  url: string
  icon: {
    name: string
    color: string
    size: string
  }
}

const EMPTY_FINANCE_OVERVIEW: MerchantFinanceOverviewResponse = {
  completed_orders: 0,
  pending_orders: 0,
  total_gmv: 0,
  total_income: 0,
  total_platform_fee: 0,
  total_operator_fee: 0,
  total_service_fee: 0,
  pending_income: 0,
  promotion_orders: 0,
  total_promotion_exp: 0,
  net_income: 0
}

const EMPTY_SERVICE_FEES: MerchantServiceFeeSummaryResponse = {
  details: [],
  total_platform_fee: 0,
  total_operator_fee: 0,
  total_service_fee: 0
}

const EMPTY_PROMOTIONS: MerchantPromotionExpensesResponse = {
  orders: [],
  total: 0,
  page: 1,
  limit: 5,
  total_pages: 0,
  total_promo_orders: 0,
  total_promo_amount: 0
}

const EMPTY_SETTLEMENTS: MerchantSettlementsResponse = {
  settlements: [],
  total: 0,
  page: 1,
  limit: 5,
  total_pages: 0,
  total_amount: 0,
  total_merchant_amount: 0,
  total_platform_fee: 0,
  total_operator_fee: 0
}

function buildRefreshErrorMessage(messages: string[]) {
  const normalized = messages.filter((message) => typeof message === 'string' && message.trim())
  if (!normalized.length) return ''
  return Array.from(new Set(normalized)).join('；')
}

function createGridIcon(name: string, color = 'var(--td-brand-color)') {
  return {
    name,
    color,
    size: '40rpx'
  }
}

function buildOverviewTiles(overview: MerchantFinanceOverviewResponse): OverviewTile[] {
  return [
    {
      id: 'net-income',
      label: '净收入',
      value: formatAmountText(overview.net_income),
      icon: createGridIcon('wallet')
    },
    {
      id: 'total-gmv',
      label: '总交易额',
      value: formatAmountText(overview.total_gmv),
      icon: createGridIcon('chart-bar')
    },
    {
      id: 'pending-income',
      label: '待结算',
      value: formatAmountText(overview.pending_income),
      icon: createGridIcon('time')
    },
    {
      id: 'total-service-fee',
      label: '总服务费',
      value: formatAmountText(overview.total_service_fee),
      icon: createGridIcon('creditcard')
    }
  ]
}

function buildAnalysisEntries(params: {
  rangeKey: FinanceRangeKey
  overview: MerchantFinanceOverviewResponse
  serviceFees: MerchantServiceFeeSummaryResponse
  promotions: MerchantPromotionExpensesResponse
  dailyRows: MerchantDailyFinanceItem[]
  settlements: MerchantSettlementsResponse
  timeline: MerchantSettlementTimelineItem[]
}): AnalysisEntry[] {
  const rangeQuery = `?range=${params.rangeKey}`
  const latestDaily = params.dailyRows[0]
  const latestTimeline = params.timeline[0]

  return [
    {
      id: 'orders',
      title: '订单收入',
      value: formatAmountText(params.overview.total_income),
      url: `/pages/merchant/finance/orders/index${rangeQuery}`,
      icon: createGridIcon('cart')
    },
    {
      id: 'service-fees',
      title: '服务费',
      value: formatAmountText(params.serviceFees.total_service_fee),
      url: `/pages/merchant/finance/service-fees/index${rangeQuery}`,
      icon: createGridIcon('wallet')
    },
    {
      id: 'promotions',
      title: '营销支出',
      value: formatAmountText(params.promotions.total_promo_amount),
      url: `/pages/merchant/finance/promotions/index${rangeQuery}`,
      icon: createGridIcon('discount')
    },
    {
      id: 'daily',
      title: '财务日报',
      value: latestDaily ? formatAmountText(latestDaily.merchant_income) : '--',
      url: `/pages/merchant/finance/daily/index${rangeQuery}`,
      icon: createGridIcon('calendar-event')
    },
    {
      id: 'settlements',
      title: '结算记录',
      value: formatAmountText(params.settlements.total_merchant_amount),
      url: `/pages/merchant/finance/settlements/index${rangeQuery}`,
      icon: createGridIcon('creditcard')
    },
    {
      id: 'timeline',
      title: '结算流水',
      value: latestTimeline ? formatAmountText(latestTimeline.merchant_amount || latestTimeline.total_amount || 0) : '--',
      url: `/pages/merchant/finance/timeline/index${rangeQuery}`,
      icon: createGridIcon('time')
    }
  ]
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
    loading: false,
    currentRange: '7d' as FinanceRangeKey,
    rangeOptions: FINANCE_RANGE_OPTIONS,
    updatedAtLabel: '--',
    dateRangeLabel: '--',
    overviewTiles: buildOverviewTiles(EMPTY_FINANCE_OVERVIEW) as OverviewTile[],
    analysisEntries: [] as AnalysisEntry[]
  },

  async onLoad(options?: { range?: FinanceRangeKey }) {
    const { navBarHeight } = getStableBarHeights()
    const currentRange = options?.range === '30d' ? '30d' : '7d'
    this.setData({ navBarHeight, currentRange })
    await this.bootstrapPage()
  },

  onPullDownRefresh() {
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }
    void this.loadData({ force: true })
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
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })

    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    await this.loadData({ force: true })
  },

  hasAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
  },

  async loadData(options?: { force?: boolean }) {
    const { force = false } = options || {}
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    const { params, label } = buildFinanceRange(this.data.currentRange)

    if (!force && this.data.loading) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      dateRangeLabel: label
    })

    try {
      const [overviewResult, serviceFeeResult, promotionsResult, dailyResult, settlementsResult, timelineResult] = await settleAll([
        getMerchantFinanceOverview(params),
        getMerchantServiceFees(params),
        getMerchantPromotionExpenses({ ...params, page: 1, limit: 20 }),
        getMerchantDailyFinance(params),
        listMerchantSettlements({ ...params, page: 1, limit: 10 }),
        listMerchantSettlementTimeline({ ...params, page: 1, limit: 20 })
      ] as const)
      const staleMessages: string[] = []

      if (
        isSettledRejected(overviewResult) ||
        isSettledRejected(serviceFeeResult) ||
        isSettledRejected(promotionsResult) ||
        isSettledRejected(dailyResult) ||
        isSettledRejected(settlementsResult) ||
        isSettledRejected(timelineResult)
      ) {
        const failedResults = [
          overviewResult,
          serviceFeeResult,
          promotionsResult,
          dailyResult,
          settlementsResult,
          timelineResult
        ]

        failedResults.forEach((result) => {
          if (isSettledRejected(result)) {
            staleMessages.push(getErrorUserMessage(result.reason, '经营分析部分数据加载失败，请稍后重试'))
          }
        })
      }

      if (isSettledRejected(overviewResult)) {
        throw overviewResult.reason
      }

      const serviceFeeSummary = isSettledFulfilled(serviceFeeResult) ? serviceFeeResult.value : EMPTY_SERVICE_FEES
      const promotionSummary = isSettledFulfilled(promotionsResult) ? promotionsResult.value : EMPTY_PROMOTIONS
      const dailyFinanceRows = isSettledFulfilled(dailyResult) ? (dailyResult.value.daily_stats || []) : []
      const settlementsSummary = isSettledFulfilled(settlementsResult) ? settlementsResult.value : EMPTY_SETTLEMENTS
      const settlementTimeline = isSettledFulfilled(timelineResult) ? (timelineResult.value.timeline || []) : []
      const overviewTiles = buildOverviewTiles(overviewResult.value)
      const analysisEntries = buildAnalysisEntries({
        rangeKey: this.data.currentRange,
        overview: overviewResult.value,
        serviceFees: serviceFeeSummary,
        promotions: promotionSummary,
        dailyRows: dailyFinanceRows,
        settlements: settlementsSummary,
        timeline: settlementTimeline
      })

      this.setData({
        overviewTiles,
        analysisEntries,
        refreshErrorMessage: buildRefreshErrorMessage(staleMessages),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loading: false,
        updatedAtLabel: formatDateTime(new Date().toISOString()).slice(11, 16)
      })
    } catch (error) {
      logger.error('Load merchant finance bills failed', error, 'merchant-finance-bills')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '经营分析加载失败，请稍后重试',
        refreshErrorMessage: '',
        loading: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.loadData({ force: true })
  },

  onRangeChange(e: WechatMiniprogram.CustomEvent<{ value?: FinanceRangeKey }>) {
    const key = e.detail.value
    if (!key || key === this.data.currentRange) {
      return
    }

    this.setData({ currentRange: key }, () => {
      void this.loadData({ force: true })
    })
  }
})