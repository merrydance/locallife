import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import {
  MerchantCategoryStatRow,
  MerchantDailyStatRow,
  MerchantHourlyStatRow,
  MerchantOrderSourceStatRow,
  MerchantOverviewResponse,
  MerchantRepurchaseRateResponse,
  MerchantStatsService,
  TopSellingDishRow
} from '../_api/merchant-stats'
import { MerchantOrderManagementService, OrderStatsResponse } from '../_api/order-management'
import { ReservationService, ReservationStats } from '../_main_shared/api/reservation'
import { logger } from '../../../utils/logger'
import { isSettledFulfilled, isSettledRejected, settleAll, type SettledResult } from '../../../utils/promise'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type StatsRangeKey = '7d' | '30d'

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

interface DailyStatsViewModel extends MerchantDailyStatRow {
  dateLabel: string
  totalSalesText: string
  commissionText: string
  orderCountLabel: string
  orderMixLabel: string
  barWidth: string
}

interface HourlyStatsViewModel extends MerchantHourlyStatRow {
  hourLabel: string
  orderCountLabel: string
  avgOrderAmountText: string
  barWidth: string
}

interface SourceStatsViewModel extends MerchantOrderSourceStatRow {
  orderTypeLabel: string
  totalSalesText: string
  orderCountLabel: string
  shareText: string
  barWidth: string
}

interface RepurchaseSummaryViewModel {
  totalUsers: number
  repeatUsers: number
  repurchaseRateText: string
  avgOrdersPerUserText: string
}

interface CategoryStatsViewModel extends MerchantCategoryStatRow {
  totalSalesText: string
  orderCountLabel: string
  totalQuantityLabel: string
  shareText: string
  barWidth: string
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
  avg_daily_sales: 0,
  print_anomalies_count: 0
}

const EMPTY_ORDER_STATS: OrderStatsResponse = {
  pending_count: 0,
  paid_count: 0,
  preparing_count: 0,
  ready_count: 0,
  delivering_count: 0,
  completed_count: 0,
  cancelled_count: 0
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

const EMPTY_REPURCHASE_SUMMARY: RepurchaseSummaryViewModel = {
  totalUsers: 0,
  repeatUsers: 0,
  repurchaseRateText: '--',
  avgOrdersPerUserText: '--'
}

const ORDER_TYPE_LABELS: Record<string, string> = {
  takeout: '外卖',
  dine_in: '堂食',
  takeaway: '自提',
  reservation: '预订'
}

function formatAmount(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return `${value.toFixed(1)}%`
}

function formatDecimal(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return value.toFixed(2)
}

function padHour(hour: number): string {
  return String(Math.max(0, Math.min(23, Math.round(hour)))).padStart(2, '0')
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
  const completedOrders = overview.total_orders || orderStats.completed_count || 0
  const totalOrders =
    (orderStats.pending_count || 0) +
    (orderStats.paid_count || 0) +
    (orderStats.preparing_count || 0) +
    (orderStats.ready_count || 0) +
    (orderStats.delivering_count || 0) +
    completedOrders +
    (orderStats.cancelled_count || 0)
  const totalSales = overview.total_sales || 0
  const avgOrderValue = completedOrders > 0 ? Math.round(totalSales / completedOrders) : 0
  const completionRate = totalOrders > 0 ? (completedOrders / totalOrders) * 100 : 0
  return {
    totalOrders,
    totalSalesText: formatAmount(totalSales),
    avgOrderValueText: formatAmount(avgOrderValue),
    avgDailySalesText: formatAmount(overview.avg_daily_sales || 0),
    totalCommissionText: formatAmount(overview.total_commission || 0),
    completionRateText: formatPercent(completionRate),
    cancelledOrders: orderStats.cancelled_count || 0,
    completedOrders,
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

function buildDailyRows(rows: MerchantDailyStatRow[]): DailyStatsViewModel[] {
  const maxSales = rows.reduce((max, item) => Math.max(max, item.total_sales || 0), 0)

  return rows.map((item) => {
    const widthPercent = maxSales > 0 ? Math.round(((item.total_sales || 0) / maxSales) * 100) : 0

    return {
      ...item,
      dateLabel: dayjs(item.date).isValid() ? dayjs(item.date).format('MM.DD') : item.date,
      totalSalesText: formatAmount(item.total_sales || 0),
      commissionText: formatAmount(item.commission || 0),
      orderCountLabel: `${item.order_count || 0} 单`,
      orderMixLabel: `外卖 ${item.takeout_orders || 0} / 堂食 ${item.dine_in_orders || 0}`,
      barWidth: `${Math.max(widthPercent, item.total_sales > 0 ? 18 : 0)}%`
    }
  })
}

function buildHourlyRows(rows: MerchantHourlyStatRow[]): HourlyStatsViewModel[] {
  const maxOrderCount = rows.reduce((max, item) => Math.max(max, item.order_count || 0), 0)

  return rows.map((item) => {
    const widthPercent = maxOrderCount > 0 ? Math.round(((item.order_count || 0) / maxOrderCount) * 100) : 0

    return {
      ...item,
      hourLabel: `${padHour(item.hour)}:00`,
      orderCountLabel: `${item.order_count || 0} 单`,
      avgOrderAmountText: formatAmount(item.avg_order_amount || 0),
      barWidth: `${Math.max(widthPercent, item.order_count > 0 ? 18 : 0)}%`
    }
  })
}

function buildSourceRows(rows: MerchantOrderSourceStatRow[]): SourceStatsViewModel[] {
  const totalOrders = rows.reduce((sum, item) => sum + (item.order_count || 0), 0)
  const maxOrderCount = rows.reduce((max, item) => Math.max(max, item.order_count || 0), 0)

  return rows.map((item) => {
    const widthPercent = maxOrderCount > 0 ? Math.round(((item.order_count || 0) / maxOrderCount) * 100) : 0

    return {
      ...item,
      orderTypeLabel: ORDER_TYPE_LABELS[item.order_type] || item.order_type || '其他',
      totalSalesText: formatAmount(item.total_sales || 0),
      orderCountLabel: `${item.order_count || 0} 单`,
      shareText: totalOrders > 0 ? `订单占比 ${(((item.order_count || 0) / totalOrders) * 100).toFixed(1)}%` : '订单占比 --',
      barWidth: `${Math.max(widthPercent, item.order_count > 0 ? 18 : 0)}%`
    }
  })
}

function buildRepurchaseSummary(data: MerchantRepurchaseRateResponse): RepurchaseSummaryViewModel {
  return {
    totalUsers: data.total_users || 0,
    repeatUsers: data.repeat_users || 0,
    repurchaseRateText: formatPercent(data.repurchase_rate || 0),
    avgOrdersPerUserText: `${formatDecimal(data.avg_orders_per_user || 0)} 单`
  }
}

function buildCategoryRows(rows: MerchantCategoryStatRow[]): CategoryStatsViewModel[] {
  const totalSales = rows.reduce((sum, item) => sum + (item.total_sales || 0), 0)
  const maxSales = rows.reduce((max, item) => Math.max(max, item.total_sales || 0), 0)

  return rows.map((item) => {
    const widthPercent = maxSales > 0 ? Math.round(((item.total_sales || 0) / maxSales) * 100) : 0

    return {
      ...item,
      totalSalesText: formatAmount(item.total_sales || 0),
      orderCountLabel: `${item.order_count || 0} 单`,
      totalQuantityLabel: `${item.total_quantity || 0} 份`,
      shareText: totalSales > 0 ? `小计占比 ${(((item.total_sales || 0) / totalSales) * 100).toFixed(1)}%` : '小计占比 --',
      barWidth: `${Math.max(widthPercent, item.total_sales > 0 ? 18 : 0)}%`
    }
  })
}

const getErrorMessage = getErrorUserMessage

function getSummaryErrorMessage(
  overviewResult: SettledResult<MerchantOverviewResponse>,
  orderStatsResult: SettledResult<OrderStatsResponse>
) {
  if (isSettledRejected(overviewResult) && isSettledRejected(orderStatsResult)) {
    return getErrorMessage(overviewResult.reason, '经营概览加载失败，请稍后重试')
  }
  if (isSettledRejected(overviewResult)) {
    return getErrorMessage(overviewResult.reason, '经营概览同步失败，请稍后重试')
  }
  if (isSettledRejected(orderStatsResult)) {
    return getErrorMessage(orderStatsResult.reason, '订单统计同步失败，请稍后重试')
  }
  return ''
}

function getSectionErrorMessage<T>(result: SettledResult<T>, fallback: string) {
  if (isSettledRejected(result)) {
    return getErrorMessage(result.reason, fallback)
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
    dailyStatsAvailable: false,
    dailyStatsError: false,
    dailyStatsErrorMessage: '',
    hourlyStatsAvailable: false,
    hourlyStatsError: false,
    hourlyStatsErrorMessage: '',
    sourceStatsAvailable: false,
    sourceStatsError: false,
    sourceStatsErrorMessage: '',
    repurchaseAvailable: false,
    repurchaseError: false,
    repurchaseErrorMessage: '',
    categoryStatsAvailable: false,
    categoryStatsError: false,
    categoryStatsErrorMessage: '',
    reservationStatsAvailable: false,
    reservationStatsError: false,
    reservationStatsErrorMessage: '',
    updatedAtLabel: '--',
    summary: buildSummary(EMPTY_OVERVIEW, EMPTY_ORDER_STATS, '--'),
    topDishes: [] as TopDishViewModel[],
    dailyStats: [] as DailyStatsViewModel[],
    hourlyStats: [] as HourlyStatsViewModel[],
    sourceStats: [] as SourceStatsViewModel[],
    repurchaseSummary: EMPTY_REPURCHASE_SUMMARY as RepurchaseSummaryViewModel,
    categoryStats: [] as CategoryStatsViewModel[],
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
            dailyStatsError: false,
            dailyStatsErrorMessage: '',
            hourlyStatsError: false,
            hourlyStatsErrorMessage: '',
            sourceStatsError: false,
            sourceStatsErrorMessage: '',
            repurchaseError: false,
            repurchaseErrorMessage: '',
            categoryStatsError: false,
            categoryStatsErrorMessage: '',
            reservationStatsError: false,
            reservationStatsErrorMessage: ''
          }
        : {})
    })

    try {
      const [overviewResult, orderStatsResult, topDishesResult, dailyStatsResult, hourlyStatsResult, sourceStatsResult, repurchaseResult, categoryStatsResult, reservationStatsResult] = await settleAll([
        MerchantStatsService.getOverview(params),
        MerchantOrderManagementService.getOrderStats(params),
        MerchantStatsService.getTopSellingDishes({ ...params, limit: 5 }),
        MerchantStatsService.getDailyStats(params),
        MerchantStatsService.getHourlyStats(params),
        MerchantStatsService.getOrderSources(params),
        MerchantStatsService.getRepurchaseRate(params),
        MerchantStatsService.getCategoryStats(params),
        ReservationService.getReservationStats()
      ] as const)

      const summarySuccess = isSettledFulfilled(overviewResult) && isSettledFulfilled(orderStatsResult)
      const topDishesSuccess = isSettledFulfilled(topDishesResult)
      const dailyStatsSuccess = isSettledFulfilled(dailyStatsResult)
      const hourlyStatsSuccess = isSettledFulfilled(hourlyStatsResult)
      const sourceStatsSuccess = isSettledFulfilled(sourceStatsResult)
      const repurchaseSuccess = isSettledFulfilled(repurchaseResult)
      const categoryStatsSuccess = isSettledFulfilled(categoryStatsResult)
      const reservationStatsSuccess = isSettledFulfilled(reservationStatsResult)
      const hasSuccessfulResult = summarySuccess || topDishesSuccess || dailyStatsSuccess || hourlyStatsSuccess || sourceStatsSuccess || repurchaseSuccess || categoryStatsSuccess || reservationStatsSuccess
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

      if (dailyStatsSuccess) {
        nextState.dailyStats = buildDailyRows(dailyStatsResult.value || [])
        nextState.dailyStatsAvailable = true
        nextState.dailyStatsError = false
        nextState.dailyStatsErrorMessage = ''
      } else if (canPreserveExisting && this.data.dailyStatsAvailable) {
        nextState.dailyStatsError = true
        nextState.dailyStatsErrorMessage = `${getSectionErrorMessage(dailyStatsResult, '日趋势同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.dailyStats = []
        nextState.dailyStatsAvailable = false
        nextState.dailyStatsError = true
        nextState.dailyStatsErrorMessage = getSectionErrorMessage(dailyStatsResult, '日趋势加载失败，请稍后重试')
      }

      if (hourlyStatsSuccess) {
        nextState.hourlyStats = buildHourlyRows(hourlyStatsResult.value || [])
        nextState.hourlyStatsAvailable = true
        nextState.hourlyStatsError = false
        nextState.hourlyStatsErrorMessage = ''
      } else if (canPreserveExisting && this.data.hourlyStatsAvailable) {
        nextState.hourlyStatsError = true
        nextState.hourlyStatsErrorMessage = `${getSectionErrorMessage(hourlyStatsResult, '时段分析同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.hourlyStats = []
        nextState.hourlyStatsAvailable = false
        nextState.hourlyStatsError = true
        nextState.hourlyStatsErrorMessage = getSectionErrorMessage(hourlyStatsResult, '时段分析加载失败，请稍后重试')
      }

      if (sourceStatsSuccess) {
        nextState.sourceStats = buildSourceRows(sourceStatsResult.value || [])
        nextState.sourceStatsAvailable = true
        nextState.sourceStatsError = false
        nextState.sourceStatsErrorMessage = ''
      } else if (canPreserveExisting && this.data.sourceStatsAvailable) {
        nextState.sourceStatsError = true
        nextState.sourceStatsErrorMessage = `${getSectionErrorMessage(sourceStatsResult, '订单来源同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.sourceStats = []
        nextState.sourceStatsAvailable = false
        nextState.sourceStatsError = true
        nextState.sourceStatsErrorMessage = getSectionErrorMessage(sourceStatsResult, '订单来源加载失败，请稍后重试')
      }

      if (repurchaseSuccess) {
        nextState.repurchaseSummary = buildRepurchaseSummary(repurchaseResult.value)
        nextState.repurchaseAvailable = true
        nextState.repurchaseError = false
        nextState.repurchaseErrorMessage = ''
      } else if (canPreserveExisting && this.data.repurchaseAvailable) {
        nextState.repurchaseError = true
        nextState.repurchaseErrorMessage = `${getSectionErrorMessage(repurchaseResult, '复购数据同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.repurchaseSummary = EMPTY_REPURCHASE_SUMMARY
        nextState.repurchaseAvailable = false
        nextState.repurchaseError = true
        nextState.repurchaseErrorMessage = getSectionErrorMessage(repurchaseResult, '复购数据加载失败，请稍后重试')
      }

      if (categoryStatsSuccess) {
        nextState.categoryStats = buildCategoryRows(categoryStatsResult.value || [])
        nextState.categoryStatsAvailable = true
        nextState.categoryStatsError = false
        nextState.categoryStatsErrorMessage = ''
      } else if (canPreserveExisting && this.data.categoryStatsAvailable) {
        nextState.categoryStatsError = true
        nextState.categoryStatsErrorMessage = `${getSectionErrorMessage(categoryStatsResult, '分类小计同步失败，请稍后重试')}，当前已保留上次结果`
      } else {
        nextState.categoryStats = []
        nextState.categoryStatsAvailable = false
        nextState.categoryStatsError = true
        nextState.categoryStatsErrorMessage = getSectionErrorMessage(categoryStatsResult, '分类小计加载失败，请稍后重试')
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

  onRetryDailyStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryHourlyStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetrySourceStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryRepurchase() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryCategoryStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onRetryReservationStats() {
    this.loadStats({ showLoading: false, silent: this.data.hasLoadedData })
  },

  onGoCustomerStats() {
    wx.navigateTo({ url: '/pages/merchant/stats/customers/index' })
  }
})