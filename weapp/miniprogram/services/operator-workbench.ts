import { operatorBasicManagementService } from '../api/operator-basic-management'
import { operatorAnalyticsService } from '../api/operator-analytics'
import { operatorMerchantManagementService } from '../api/operator-merchant-management'
import { operatorRiderManagementService } from '../api/operator-rider-management'
import { formatPriceNoSymbol } from '../utils/util'

type OperatorTimeDimension = 'day' | 'week' | 'month'

export interface OperatorPendingSummary {
  merchants: number
  riders: number
}

export interface OperatorPendingApprovalItem {
  id: number
  type: 'MERCHANT' | 'RIDER'
  name: string
  time: string
}

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
  pending_approvals: OperatorPendingApprovalItem[]
  pending_count: number
  pendingSummary: OperatorPendingSummary
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
    merchantSummary,
    riderSummary,
    merchantsPending,
    ridersPending,
    merchantRanking,
    riderRanking,
    dailyTrends
  ] = await Promise.all([
    operatorBasicManagementService.getFinanceOverview(undefined, undefined, regionId).catch(() => null),
    operatorAnalyticsService.getRealtimeStats(regionId),
    operatorMerchantManagementService.getMerchantSummary(regionId)
      .catch(() => ({ total: 0, pending: 0, approved: 0, rejected: 0, suspended: 0 })),
    operatorRiderManagementService.getRiderSummary(regionId)
      .catch(() => ({ total: 0, pending_approval: 0, active: 0, rejected: 0, suspended: 0, online: 0 })),
    operatorMerchantManagementService.getMerchantList({ page: 1, limit: 5, status: 'pending', region_id: regionId })
      .catch(() => ({ merchants: [] as Array<{ id: number, name: string, created_at: string }>, total: 0 })),
    operatorRiderManagementService.getRiderList({ page: 1, limit: 5, status: 'pending_approval', region_id: regionId })
      .catch(() => ({ riders: [] as Array<{ id: number, name: string, created_at: string }>, total: 0 })),
    operatorMerchantManagementService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
      .catch(() => ({ rankings: [] })),
    operatorRiderManagementService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
      .catch(() => ({ rankings: [] })),
    operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
      .catch(() => [])
  ])

  const trends = Array.isArray(dailyTrends) ? dailyTrends as TrendLike[] : []
  const currentPeriodSummary = sumTrendValues(trends)
  const merchantRankings = normalizeRankingRows(merchantRanking)
  const riderRankings = normalizeRankingRows(riderRanking).map((item) => ({
    ...item,
    completion_rate: typeof item.completion_rate === 'number' ? item.completion_rate.toFixed(1) : '0.0'
  }))

  const pendingSummary: OperatorPendingSummary = {
    merchants: Number(merchantSummary.pending || 0),
    riders: Number((riderSummary as { pending_approval?: number }).pending_approval || 0)
  }

  const pendingItems: OperatorPendingApprovalItem[] = [
    ...(merchantsPending.merchants || []).map((item: { id: number, name: string, created_at: string }) => ({ id: item.id, type: 'MERCHANT' as const, name: item.name, time: item.created_at })),
    ...(ridersPending.riders || []).map((item: { id: number, name: string, created_at: string }) => ({ id: item.id, type: 'RIDER' as const, name: item.name, time: item.created_at }))
  ]

  pendingItems.sort((left, right) => new Date(right.time).getTime() - new Date(left.time).getTime())

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
    pending_approvals: pendingItems.slice(0, 5),
    pending_count: pendingSummary.merchants + pendingSummary.riders,
    pendingSummary
  }
}