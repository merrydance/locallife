import dayjs from 'dayjs'
import { MerchantStatsService, MerchantOverviewResponse, TopSellingDishRow } from '../../../api/merchant-stats'
import { MerchantOrderManagementService, OrderStatsResponse } from '../../../api/order-management'
import { ReservationService, ReservationStats } from '../../../api/reservation'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

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

Page({
  data: {
    navBarHeight: 88,
    rangeOptions: RANGE_OPTIONS,
    currentRange: '7d' as StatsRangeKey,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loading: false,
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
      this.loadStats(false)
    }
  },

  onPullDownRefresh() {
    this.loadStats(false)
  },

  async loadStats(showLoading = true) {
    if (this.data.loading) return

    const { params, label } = buildRange(this.data.currentRange)
    this.setData({
      loading: true,
      ...(showLoading ? { initialError: false, initialErrorMessage: '' } : {})
    })

    try {
      const [overviewResult, orderStatsResult, topDishesResult, reservationStatsResult] = await Promise.allSettled([
        MerchantStatsService.getOverview(params),
        MerchantOrderManagementService.getOrderStats(params),
        MerchantStatsService.getTopSellingDishes({ ...params, limit: 5 }),
        ReservationService.getReservationStats()
      ])

      const hasSuccessfulResult = [overviewResult, orderStatsResult, topDishesResult, reservationStatsResult]
        .some((result) => result.status === 'fulfilled')

      if (!hasSuccessfulResult) {
        throw new Error('all_stats_requests_failed')
      }

      const overview = overviewResult.status === 'fulfilled' ? overviewResult.value : EMPTY_OVERVIEW
      const orderStats = orderStatsResult.status === 'fulfilled' ? orderStatsResult.value : EMPTY_ORDER_STATS
      const topDishes = topDishesResult.status === 'fulfilled' ? topDishesResult.value : []
      const reservationStats = reservationStatsResult.status === 'fulfilled' ? reservationStatsResult.value : EMPTY_RESERVATION_STATS

      this.setData({
        summary: buildSummary(overview, orderStats, label),
        topDishes: buildTopDishRows(topDishes || []),
        reservationStats,
        updatedAtLabel: dayjs().format('HH:mm'),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant stats failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '经营统计加载失败，请重试'
        : '经营统计加载失败，请重试'

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
  }
})