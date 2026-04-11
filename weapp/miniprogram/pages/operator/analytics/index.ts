import { isLargeScreen } from '@/utils/responsive'
import { operatorBasicManagementService, RegionResponse } from '../../../api/operator-basic-management'
import { operatorAnalyticsService } from '../../../api/operator-analytics'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'
import { operatorRiderManagementService } from '../../../api/operator-rider-management'

type TimeDimension = 'day' | 'week' | 'month'
type RankingType = 'merchant' | 'rider'

interface TrendLike {
  date?: string
  total_gmv?: number
  order_count?: number
}

interface RegionSummaryView {
  regionName: string
  merchantText: string
  riderText: string
  completionRate: string
  commission: string
}

interface MerchantRankingView {
  rank: number
  name: string
  gmv: string
  orders: number
  commission: string
}

interface RiderRankingView {
  rank: number
  name: string
  deliveries: number
  completionRate: string
  earnings: string
}

function formatCurrencyFen(amount: number): string {
  return `¥${(Number(amount || 0) / 100).toFixed(2)}`
}

function getPeriodDays(dimension: TimeDimension): number {
  if (dimension === 'day') return 1
  if (dimension === 'week') return 7
  return 30
}

function formatDate(date: Date): string {
  return date.toISOString().split('T')[0]
}

function getRange(dimension: TimeDimension, offset = 0) {
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

function getPeriodLabel(dimension: TimeDimension): string {
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
      totalOrders: summary.totalOrders + Number(item.order_count || 0)
    }),
    { totalGmv: 0, totalOrders: 0 }
  )
}

Page({
  data: {
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    timeDimension: 'week' as TimeDimension,
    rankingType: 'merchant' as RankingType,
    regions: [] as RegionResponse[],
    selectedRegionIdx: 0,
    selectedRegionId: 0,
    metrics: [] as Array<{ label: string, value: string, change: string, trend: 'up' | 'down' }>,
    regionSummary: {
      regionName: '',
      merchantText: '-',
      riderText: '-',
      completionRate: '-',
      commission: '-'
    } as RegionSummaryView,
    topMerchants: [] as MerchantRankingView[],
    topRiders: [] as RiderRankingView[]
  },

  async onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    await this.loadRegions()
    this.loadData()
  },

  async loadRegions() {
    try {
      const response = await operatorBasicManagementService.getOperatorRegions({ page: 1, limit: 100 })
      const regions = response.regions || []
      this.setData({
        regions,
        selectedRegionIdx: 0,
        selectedRegionId: regions[0]?.id || 0
      })
    } catch (_error) {
      this.setData({
        regions: [],
        selectedRegionIdx: 0,
        selectedRegionId: 0
      })
    }
  },

  async loadData() {
    this.setData({ loading: true, error: null })
    try {
      const { timeDimension, selectedRegionId } = this.data
      const currentRange = getRange(timeDimension, 0)
      const previousRange = getRange(timeDimension, 1)
      const regionId = selectedRegionId || undefined
      const periodLabel = getPeriodLabel(timeDimension)

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
      const gmvTrend: 'up' | 'down' = gmvChange.startsWith('-') ? 'down' : 'up'
      const ordersTrend: 'up' | 'down' = ordersChange.startsWith('-') ? 'down' : 'up'

      const metrics = [
        {
          label: `${periodLabel}GMV`,
          value: formatCurrencyFen(currentSummary.totalGmv),
          change: gmvChange,
          trend: gmvTrend
        },
        {
          label: '活跃商户',
          value: String(realtime.active_merchant_count ?? 0),
          change: `待审 ${realtime.pending_merchant_count ?? 0}`,
          trend: 'up' as const
        },
        {
          label: '活跃骑手',
          value: String(realtime.active_rider_count ?? 0),
          change: `待审 ${realtime.pending_rider_count ?? 0}`,
          trend: 'up' as const
        },
        {
          label: `${periodLabel}订单`,
          value: String(currentSummary.totalOrders),
          change: ordersChange,
          trend: ordersTrend
        }
      ]

      const merchantRankingList = (Array.isArray((merchantRanking as unknown as { rankings?: unknown[] }).rankings)
        ? (merchantRanking as unknown as { rankings: Array<Record<string, unknown>> }).rankings
        : (Array.isArray(merchantRanking) ? merchantRanking : [])) as Array<Record<string, unknown>>

      const riderRankingList = (Array.isArray((riderRanking as unknown as { rankings?: unknown[] }).rankings)
        ? (riderRanking as unknown as { rankings: Array<Record<string, unknown>> }).rankings
        : (Array.isArray(riderRanking) ? riderRanking : [])) as Array<Record<string, unknown>>

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

      const regionSummary: RegionSummaryView = regionStats
        ? {
            regionName: String(regionStats.region_name || this.data.regions[this.data.selectedRegionIdx]?.name || '当前区域'),
            merchantText: `${regionStats.merchant_stats.active_merchants}/${regionStats.merchant_stats.total_merchants}`,
            riderText: `${regionStats.rider_stats.online_riders}/${regionStats.rider_stats.active_riders}`,
            completionRate: `${Number(regionStats.order_stats.completion_rate || 0).toFixed(1)}%`,
            commission: formatCurrencyFen(regionStats.financial_stats.total_commission || 0)
          }
        : {
            regionName: this.data.regions[this.data.selectedRegionIdx]?.name || '全部区域',
            merchantText: '-',
            riderText: '-',
            completionRate: '-',
            commission: '-'
          }
      
      this.setData({
        metrics,
        regionSummary,
        topMerchants,
        topRiders,
        initialLoading: false,
        loading: false
      })
    } catch (error) {
      console.error('加载分析数据失败:', error)
      this.setData({
        initialLoading: false,
        loading: false,
        error: '加载分析数据失败'
      })
    }
  },

  onRetry() {
    this.loadData()
  },

  onRegionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const idx = parseInt(e.detail.value, 10)
    const regionId = this.data.regions[idx]?.id || 0
    this.setData({ selectedRegionIdx: idx, selectedRegionId: regionId }, () => {
      this.loadData()
    })
  },

  onTimeDimensionChange(e: WechatMiniprogram.CustomEvent<{ value: TimeDimension }>) {
    this.setData({ timeDimension: e.detail.value }, () => {
      this.loadData()
    })
  },

  onRankingTypeChange(e: WechatMiniprogram.CustomEvent<{ value: RankingType }>) {
    this.setData({ rankingType: e.detail.value })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  }
})
