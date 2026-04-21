import { operatorBasicManagementService, type RegionResponse } from '../api/operator-basic-management'
import { operatorAnalyticsService } from '../api/operator-analytics'
import { operatorMerchantManagementService } from '../api/operator-merchant-management'
import { operatorRiderManagementService } from '../api/operator-rider-management'
import { formatPriceNoSymbol } from '../utils/util'

export type OperatorTimeDimension = 'day' | 'week' | 'month'

export interface ConsolePickerOption {
  label: string
  value: string
}

export interface ConsoleRegionOption {
  id: number
  name: string
}

export interface ConsoleRegionPickerState {
  regions: ConsoleRegionOption[]
  regionPickerOptions: ConsolePickerOption[]
  regionPickerVisible: boolean
  selectedRegionIdx: number
  selectedRegionId: number
  selectedRegionValue: string
}

export interface OperatorAnalyticsMetric {
  label: string
  value: string
  change: string
  trend: 'up' | 'down'
}

export interface OperatorAnalyticsRegionSummary {
  regionName: string
  merchantText: string
  riderText: string
  completionRate: string
  commission: string
}

export interface OperatorMerchantRankingView {
  rank: number
  name: string
  gmv: string
  orders: number
  commission: string
}

export interface OperatorRiderRankingView {
  rank: number
  name: string
  deliveries: number
  completionRate: string
  earnings: string
}

export interface OperatorAnalyticsPageData {
  metrics: OperatorAnalyticsMetric[]
  regionSummary: OperatorAnalyticsRegionSummary
  topMerchants: OperatorMerchantRankingView[]
  topRiders: OperatorRiderRankingView[]
}

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
  date?: string
  total_gmv?: number
  order_count?: number
  operator_income?: number
}

function formatCurrencyFen(amount: number): string {
  return `¥${(Number(amount || 0) / 100).toFixed(2)}`
}

function getPeriodDays(dimension: OperatorTimeDimension): number {
  if (dimension === 'day') return 1
  if (dimension === 'week') return 7
  return 30
}

function formatDate(date: Date): string {
  return date.toISOString().split('T')[0]
}

function getRange(dimension: OperatorTimeDimension, offset = 0) {
  const days = getPeriodDays(dimension)
  const end = new Date()
  end.setHours(0, 0, 0, 0)
  end.setDate(end.getDate() - (days * offset))

  const start = new Date(end)
  start.setDate(end.getDate() - (days - 1))

  return {
    startDate: formatDate(start),
    endDate: formatDate(end)
  }
}

function getPeriodLabel(dimension: OperatorTimeDimension): string {
  if (dimension === 'day') return '今日'
  if (dimension === 'week') return '近7天'
  return '近30天'
}

function calcChange(current: number, previous: number): string {
  if (!previous) return '+0%'
  const rate = ((current - previous) / previous) * 100
  return rate >= 0 ? `+${rate.toFixed(1)}%` : `${rate.toFixed(1)}%`
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

function buildRegionPickerState(regions: ConsoleRegionOption[]): ConsoleRegionPickerState {
  return {
    regions,
    regionPickerOptions: regions.map((item) => ({ label: item.name, value: String(item.id) })),
    regionPickerVisible: false,
    selectedRegionIdx: 0,
    selectedRegionId: regions[0]?.id || 0,
    selectedRegionValue: String(regions[0]?.id || '')
  }
}

function mapRegions(source: RegionResponse[]): ConsoleRegionOption[] {
  return (source || []).map((item) => ({ id: item.id, name: item.name }))
}

export async function loadOperatorRegions(): Promise<ConsoleRegionPickerState> {
  try {
    const response = await operatorBasicManagementService.getOperatorRegions({ page: 1, limit: 100 })
    return buildRegionPickerState(mapRegions(response.regions || []))
  } catch (_error) {
    return buildRegionPickerState([])
  }
}

export async function loadOperatorAnalyticsPageData(params: {
  timeDimension: OperatorTimeDimension
  selectedRegionId?: number
  selectedRegionName?: string
}): Promise<OperatorAnalyticsPageData> {
  const currentRange = getRange(params.timeDimension, 0)
  const previousRange = getRange(params.timeDimension, 1)
  const regionId = params.selectedRegionId || undefined
  const periodLabel = getPeriodLabel(params.timeDimension)

  const [realtime, trends, regionStats, merchantRanking, riderRanking] = await Promise.all([
    operatorAnalyticsService.getRealtimeStats(regionId),
    operatorAnalyticsService.getDailyTrend(regionId, previousRange.startDate, currentRange.endDate),
    regionId
      ? operatorAnalyticsService.getRegionStats(regionId, currentRange.startDate, currentRange.endDate).catch(() => null)
      : Promise.resolve(null),
    operatorMerchantManagementService.getMerchantRanking({
      region_id: regionId,
      start_date: currentRange.startDate,
      end_date: currentRange.endDate,
      limit: 5
    }),
    operatorRiderManagementService.getRiderRanking({
      region_id: regionId,
      start_date: currentRange.startDate,
      end_date: currentRange.endDate,
      limit: 5
    })
  ])

  const trendList = Array.isArray(trends) ? trends as TrendLike[] : []
  const currentPeriod = trendList.filter((item) => item.date && item.date >= currentRange.startDate && item.date <= currentRange.endDate)
  const previousPeriod = trendList.filter((item) => item.date && item.date >= previousRange.startDate && item.date <= previousRange.endDate)
  const currentSummary = sumTrendValues(currentPeriod)
  const previousSummary = sumTrendValues(previousPeriod)
  const gmvChange = calcChange(currentSummary.totalGmv, previousSummary.totalGmv)
  const ordersChange = calcChange(currentSummary.totalOrders, previousSummary.totalOrders)

  const metrics: OperatorAnalyticsMetric[] = [
    {
      label: `${periodLabel}GMV`,
      value: formatCurrencyFen(currentSummary.totalGmv),
      change: gmvChange,
      trend: gmvChange.startsWith('-') ? 'down' : 'up'
    },
    {
      label: '活跃商户',
      value: String(realtime.active_merchant_count ?? 0),
      change: `待审 ${realtime.pending_merchant_count ?? 0}`,
      trend: 'up'
    },
    {
      label: '活跃骑手',
      value: String(realtime.active_rider_count ?? 0),
      change: `待审 ${realtime.pending_rider_count ?? 0}`,
      trend: 'up'
    },
    {
      label: `${periodLabel}订单`,
      value: String(currentSummary.totalOrders),
      change: ordersChange,
      trend: ordersChange.startsWith('-') ? 'down' : 'up'
    }
  ]

  const merchantRankingList = normalizeRankingRows(merchantRanking)
  const riderRankingList = normalizeRankingRows(riderRanking)

  const topMerchants = merchantRankingList.slice(0, 5).map((item, index) => ({
    rank: index + 1,
    name: String(item.merchant_name || '-'),
    gmv: (Number(item.total_sales || item.total_gmv || 0) / 100).toFixed(2),
    orders: Number(item.order_count || 0),
    commission: (Number(item.total_commission || 0) / 100).toFixed(2)
  }))

  const topRiders = riderRankingList.slice(0, 5).map((item, index) => ({
    rank: index + 1,
    name: String(item.rider_name || '-'),
    deliveries: Number(item.delivery_count || 0),
    completionRate: `${Number(item.completion_rate || 0).toFixed(1)}%`,
    earnings: (Number(item.total_earnings || 0) / 100).toFixed(2)
  }))

  const regionSummary: OperatorAnalyticsRegionSummary = regionStats
    ? {
        regionName: String(regionStats.region_name || params.selectedRegionName || '当前区域'),
        merchantText: `${regionStats.merchant_stats.active_merchants}/${regionStats.merchant_stats.total_merchants}`,
        riderText: `${regionStats.rider_stats.online_riders}/${regionStats.rider_stats.active_riders}`,
        completionRate: `${Number(regionStats.order_stats.completion_rate || 0).toFixed(1)}%`,
        commission: formatCurrencyFen(regionStats.financial_stats.total_commission || 0)
      }
    : {
        regionName: params.selectedRegionName || '全部区域',
        merchantText: '-',
        riderText: '-',
        completionRate: '-',
        commission: '-'
      }

  return {
    metrics,
    regionSummary,
    topMerchants,
    topRiders
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
