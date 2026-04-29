import { operatorBasicManagementService } from '../api/operator-basic-management'
import { operatorAnalyticsService } from '../api/operator-analytics'
import { operatorMerchantManagementService } from '../api/operator-merchant-management'
import { operatorRiderManagementService } from '../api/operator-rider-management'
import {
  loadOperatorNotificationSummaryCard,
  type OperatorNotificationSummaryCard
} from './operator-notification-center'
import { formatPriceNoSymbol } from '../utils/util'

type OperatorTimeDimension = 'day' | 'week' | 'month'

export interface OperatorCenterStats {
  total_gmv_display: string
  total_orders: number
  active_merchants: number
  active_riders: number
  today_gmv_display: string
  today_orders: number
  today_income_display: string
}

export interface OperatorCenterFinance {
  balance_display: string
  total_income_display: string
  current_month_income_display: string
}

export interface OperatorCenterRiderRankingItem extends Record<string, unknown> {
  completion_rate: string
}

export interface OperatorCenterPageData {
  stats: OperatorCenterStats
  finance: OperatorCenterFinance
  merchantRankings: Array<Record<string, unknown>>
  riderRankings: OperatorCenterRiderRankingItem[]
  notificationSummary: OperatorNotificationSummaryCard
}

interface TrendLike {
  total_gmv?: number
  order_count?: number
  operator_income?: number
}

function sumTrendValues(trends: TrendLike[]) {
  return trends.reduce(
    (summary, item) => ({
      totalGmv: summary.totalGmv + Number(item.total_gmv || 0),
      totalOrders: summary.totalOrders + Number(item.order_count || 0),
      totalIncome: summary.totalIncome + Number(item.operator_income || 0)
    }),
    { totalGmv: 0, totalOrders: 0, totalIncome: 0 }
  )
}

function normalizeRankingRows(source: unknown): Array<Record<string, unknown>> {
  if (Array.isArray(source)) {
    return source as Array<Record<string, unknown>>
  }

  if (source && typeof source === 'object' && Array.isArray((source as { rankings?: unknown[] }).rankings)) {
    return (source as { rankings: Array<Record<string, unknown>> }).rankings
  }

  return []
}

function getCenterDateRange(dimension: OperatorTimeDimension) {
  const end = new Date()
  const start = new Date()

  if (dimension === 'day') {
    start.setHours(0, 0, 0, 0)
  } else if (dimension === 'week') {
    start.setDate(end.getDate() - 7)
  } else {
    start.setMonth(end.getMonth() - 1)
  }

  return {
    startDate: start.toISOString().split('T')[0],
    endDate: end.toISOString().split('T')[0]
  }
}

export async function loadOperatorCenterPageData(params: {
  timeDimension: OperatorTimeDimension
  selectedRegionId?: number
}): Promise<OperatorCenterPageData> {
  const { startDate, endDate } = getCenterDateRange(params.timeDimension)
  const regionId = params.selectedRegionId || undefined

  const [
    financeOverview,
    realtimeStats,
    merchantRanking,
    riderRanking,
    dailyTrends,
    notificationSummary
  ] = await Promise.all([
    operatorBasicManagementService.getFinanceOverview(undefined, undefined, regionId).catch(() => null),
    operatorAnalyticsService.getRealtimeStats(regionId),
    operatorMerchantManagementService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
      .catch(() => ({ rankings: [] })),
    operatorRiderManagementService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
      .catch(() => ({ rankings: [] })),
    operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
      .catch(() => []),
    loadOperatorNotificationSummaryCard().catch(() => ({
      unreadCount: 0,
      latestTitle: '暂无待接单提醒',
      latestSummary: '当前没有新的待接单提醒。',
      latestCreatedAt: '',
      empty: true
    }))
  ])

  const trends = Array.isArray(dailyTrends) ? dailyTrends as TrendLike[] : []
  const currentPeriodSummary = sumTrendValues(trends)
  const merchantRankings = normalizeRankingRows(merchantRanking)
  const riderRankings = normalizeRankingRows(riderRanking).map((item) => ({
    ...item,
    completion_rate: typeof item.completion_rate === 'number' ? item.completion_rate.toFixed(1) : '0.0'
  }))

  return {
    stats: {
      total_gmv_display: formatPriceNoSymbol(currentPeriodSummary.totalGmv),
      total_orders: currentPeriodSummary.totalOrders,
      active_merchants: realtimeStats.active_merchant_count,
      active_riders: realtimeStats.active_rider_count,
      today_gmv_display: formatPriceNoSymbol(currentPeriodSummary.totalGmv),
      today_orders: currentPeriodSummary.totalOrders,
      today_income_display: formatPriceNoSymbol(currentPeriodSummary.totalIncome)
    },
    finance: {
      balance_display: formatPriceNoSymbol(financeOverview?.total?.operator_income ?? 0),
      total_income_display: formatPriceNoSymbol(financeOverview?.total?.operator_income ?? 0),
      current_month_income_display: formatPriceNoSymbol(financeOverview?.current_month?.operator_income ?? 0)
    },
    merchantRankings,
    riderRankings,
    notificationSummary
  }
}
