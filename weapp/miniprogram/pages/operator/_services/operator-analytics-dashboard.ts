import { operatorAnalyticsService } from '../_api/operator-analytics'
import { operatorMerchantManagementService } from '../_api/operator-merchant-management'
import { operatorRiderManagementService } from '../_api/operator-rider-management'
import { formatPrice, formatPriceNoSymbol } from '../../../utils/util'

type OperatorTimeDimension = 'day' | 'week' | 'month'

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

interface TrendLike {
  date?: string
  total_gmv?: number
  order_count?: number
  operator_income?: number
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
      value: formatPrice(currentSummary.totalGmv),
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
      change: '已激活',
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
    gmv: formatPriceNoSymbol(Number(item.total_sales || item.total_gmv || 0)),
    orders: Number(item.order_count || 0),
    commission: formatPriceNoSymbol(Number(item.total_commission || 0))
  }))

  const topRiders = riderRankingList.slice(0, 5).map((item, index) => ({
    rank: index + 1,
    name: String(item.rider_name || '-'),
    deliveries: Number(item.delivery_count || 0),
    completionRate: `${Number(item.completion_rate || 0).toFixed(1)}%`,
    earnings: formatPriceNoSymbol(Number(item.total_earnings || 0))
  }))

  const regionSummary: OperatorAnalyticsRegionSummary = regionStats
    ? {
        regionName: String(regionStats.region_name || params.selectedRegionName || '当前区域'),
        merchantText: `${regionStats.merchant_stats.active_merchants}/${regionStats.merchant_stats.total_merchants}`,
        riderText: `${regionStats.rider_stats.online_riders}/${regionStats.rider_stats.active_riders}`,
        completionRate: `${Number(regionStats.order_stats.completion_rate || 0).toFixed(1)}%`,
        commission: formatPrice(regionStats.financial_stats.total_commission || 0)
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
