import dayjs from 'dayjs'
import { MerchantStatsService, MerchantOverviewResponse, TopSellingDishRow } from '../../../api/merchant-stats'
import { MerchantOrderManagementService, OrderStatsResponse } from '../../../api/order-management'
import { ReservationService, ReservationStats } from '../../../api/reservation'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import { getStableBarHeights } from '../../../utils/responsive'

type StatsRangeKey = '7d' | '30d'

type SettledResult<T> =
  | { status: 'fulfilled', value: T }
  | { status: 'rejected', reason: unknown }

interface RangeOption {
  key: StatsRangeKey
  label: string
  days: number
}

interface MerchantStatsSummary {
  totalOrders: number
  totalSalesText: string
  avgOrderValueText: string
  avgDailySalesText: string
  totalCommissionText: string
  completionRateText: string
  cancelledOrders: number
  completedOrders: number
  dateRangeLabel: string
}

interface TopDishViewModel extends TopSellingDishRow {
  rank: number
  revenueText: string
  priceText: string
  soldLabel: string
}

const RANGE_OPTIONS: RangeOption[] = [
  { key: '7d', label: '近7天', days: 7 },
  { key: '30d', label: '近30天', days: 30 }
]

const EMPTY_OVERVIEW: MerchantOverviewResponse = {
  total_days: 0,
  total_orders: 0,
  total_sales: 0,
  total_commission: 0,
  avg_daily_sales: 0
}

const EMPTY_ORDER_STATS: OrderStatsResponse = {
  total_orders: 0,
  total_revenue: 0,
  avg_order_value: 0,
  completed_orders: 0,
  cancelled_orders: 0,
  completion_rate: 0
}

const EMPTY_RESERVATION_STATS: ReservationStats = {
  pending_count: 0,
  paid_count: 0,
  confirmed_count: 0,
  checked_in_count: 0,
  completed_count: 0,
  cancelled_count: 0,
  expired_count: 0,
  no_show_count: 0
}

function formatAmount(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return `${value.toFixed(1)}%`
}

function buildRange(rangeKey: StatsRangeKey) {
  const option = RANGE_OPTIONS.find((item) => item.key === rangeKey) || RANGE_OPTIONS[0]
  const end = dayjs()
  const start = end.subtract(option.days - 1, 'day')
  return {
    start,
    end,
    params: {
      start_date: start.format('YYYY-MM-DD'),
      end_date: end.format('YYYY-MM-DD')
    },
    label: `${start.format('MM.DD')} - ${end.format('MM.DD')}`
  }
}

function buildSummary(
  overview: MerchantOverviewResponse,
  orderStats: OrderStatsResponse,
  dateRangeLabel: string
): MerchantStatsSummary {
  const totalOrders = orderStats.total_orders || overview.total_orders || 0
  const totalSales = orderStats.total_revenue || overview.total_sales || 0
  return {
    totalOrders,
    totalSalesText: formatAmount(totalSales),
    avgOrderValueText: formatAmount(orderStats.avg_order_value || 0),
    avgDailySalesText: formatAmount(overview.avg_daily_sales || 0),
    totalCommissionText: formatAmount(overview.total_commission || 0),
    completionRateText: formatPercent(orderStats.completion_rate || 0),
    cancelledOrders: orderStats.cancelled_orders || 0,
    completedOrders: orderStats.completed_orders || 0,
    dateRangeLabel
  }
}

function buildTopDishRows(rows: TopSellingDishRow[]): TopDishViewModel[] {
  return rows.map((item, index) => ({
    ...item,
    rank: index + 1,
    revenueText: formatAmount(item.total_revenue || 0),
    priceText: formatAmount(item.dish_price || 0),
    soldLabel: `${item.total_sold || 0} 份`
  }))
}

function getErrorMessage(err: unknown, fallback: string) {
  if (typeof err === 'object' && err !== null && 'userMessage' in err) {
    const userMessage = (err as { userMessage?: unknown }).userMessage
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }
  return fallback
}

function getSummaryErrorMessage(
  overviewResult: SettledResult<MerchantOverviewResponse>,
  orderStatsResult: SettledResult<OrderStatsResponse>
) {
  if (overviewResult.status === 'rejected' && orderStatsResult.status === 'rejected') {
    return getErrorMessage(overviewResult.reason, '经营概览加载失败，请稍后重试')
  }
  if (overviewResult.status === 'rejected') {
    return getErrorMessage(overviewResult.reason, '经营概览同步失败，请稍后重试')
  }
  if (orderStatsResult.status === 'rejected') {
    return getErrorMessage(orderStatsResult.reason, '订单统计同步失败，请稍后重试')
  }
  return ''
}

Page({
  data: {
    navBarHeight: 88,
    rangeOptions: RANGE_OPTIONS,
    currentRange: '7d' as StatsRangeKey,
    initialLoading: true,
    hasLoadedData: false,
    initialError: false,
    initialErrorMessage: '',
    loading: false,
    summaryAvailable: false,
    summaryError: false,
    summaryErrorMessage: '',
    topDishesAvailable: false,
    topDishesError: false,
    topDishesErrorMessage: '',
    reservationStatsAvailable: false,
    reservationStatsError: false,
    reservationStatsErrorMessage: '',
    updatedAtLabel: '--',
    summary: buildSummary(EMPTY_OVERVIEW, EMPTY_ORDER_STATS, '--'),
    topDishes: [] as TopDishViewModel[],
    reservationStats: EMPTY_RESERVATION_STATS as ReservationStats
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadStats()
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
    }
  },

  onPullDownRefresh() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  async loadStats(options: { showLoading?: boolean, silent?: boolean } = {}) {
    const { showLoading = true, silent = false } = options
    if (this.data.loading) return

    const { params, label } = buildRange(this.data.currentRange)
    this.setData({
      loading: true,
      ...(showLoading
        ? {
            initialError: false,
            initialErrorMessage: '',
            summaryError: false,
            summaryErrorMessage: '',
            topDishesError: false,
            topDishesErrorMessage: '',
            reservationStatsError: false,
            reservationStatsErrorMessage: ''
          }
        : {})
    })

    try {
      const [overviewResult, orderStatsResult, topDishesResult, reservationStatsResult] = await settleAll([
        MerchantStatsService.getOverview(params),
        MerchantOrderManagementService.getOrderStats(params),
        MerchantStatsService.getTopSellingDishes({ ...params, limit: 5 }),
        ReservationService.getReservationStats()
      ] as const)

      const summarySuccess = overviewResult.status === 'fulfilled' && orderStatsResult.status === 'fulfilled'
      const topDishesSuccess = topDishesResult.status === 'fulfilled'
      const reservationStatsSuccess = reservationStatsResult.status === 'fulfilled'
      const hasSuccessfulResult = summarySuccess || topDishesSuccess || reservationStatsSuccess
      const canPreserveExisting = silent && this.data.hasLoadedData

      if (!hasSuccessfulResult && !canPreserveExisting) {
        throw new Error('all_stats_requests_failed')
      }

      const nextState: Record<string, unknown> = {
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        hasLoadedData: this.data.hasLoadedData || hasSuccessfulResult,
        updatedAtLabel: hasSuccessfulResult ? dayjs().format('HH:mm') : this.data.updatedAtLabel
      }

      if (summarySuccess) {
        nextState.summary = buildSummary(overviewResult.value, orderStatsResult.value, label)
        nextState.summaryAvailable = true
        nextState.summaryError = false
        nextState.summaryErrorMessage = ''
      } else if (canPreserveExisting && this.data.summaryAvailable) {
        nextState.summaryError = true
        nextState.summaryErrorMessage = `${getSummaryErrorMessage(overviewResult, orderStatsResult)}，当前已保留上次统计结果`
      } else {
        nextState.summary = buildSummary(EMPTY_OVERVIEW, EMPTY_ORDER_STATS, label)
        nextState.summaryAvailable = false
        nextState.summaryError = true
        nextState.summaryErrorMessage = getSummaryErrorMessage(overviewResult, orderStatsResult)
      }

      if (topDishesSuccess) {
        nextState.topDishes = buildTopDishRows(topDishesResult.value || [])
        nextState.topDishesAvailable = true
        nextState.topDishesError = false
        nextState.topDishesErrorMessage = ''
      } else if (canPreserveExisting && this.data.topDishesAvailable) {
        nextState.topDishesError = true
        nextState.topDishesErrorMessage = `${getErrorMessage(topDishesResult.reason, '热销菜同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.topDishes = []
        nextState.topDishesAvailable = false
        nextState.topDishesError = true
        nextState.topDishesErrorMessage = getErrorMessage(topDishesResult.reason, '热销菜加载失败，请稍后重试')
      }

      if (reservationStatsSuccess) {
        nextState.reservationStats = reservationStatsResult.value
        nextState.reservationStatsAvailable = true
        nextState.reservationStatsError = false
        nextState.reservationStatsErrorMessage = ''
      } else if (canPreserveExisting && this.data.reservationStatsAvailable) {
        nextState.reservationStatsError = true
        nextState.reservationStatsErrorMessage = `${getErrorMessage(reservationStatsResult.reason, '预订池同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.reservationStats = EMPTY_RESERVATION_STATS
        nextState.reservationStatsAvailable = false
        nextState.reservationStatsError = true
        nextState.reservationStatsErrorMessage = getErrorMessage(reservationStatsResult.reason, '预订池加载失败，请稍后重试')
      }

      this.setData(nextState)
    } catch (err: unknown) {
      logger.error('Load merchant stats failed', err)
      const message = getErrorMessage(err, '经营统计加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onSelectRange(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: StatsRangeKey }
    if (!key || key === this.data.currentRange) return
    this.setData({ currentRange: key })
    this.loadStats()
  },

  onRetry() {
    this.loadStats()
  },

  onRetrySummary() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryTopDishes() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryReservationStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  }
})