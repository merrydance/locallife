import { isLargeScreen } from '@/utils/responsive'
import {
  loadOperatorAnalyticsPageData,
  loadOperatorRegions,
  type ConsoleRegionOption,
  type OperatorAnalyticsRegionSummary,
  type OperatorMerchantRankingView,
  type OperatorRiderRankingView
} from '../../../services/operator-console'

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
    regions: [] as ConsoleRegionOption[],
    regionPickerOptions: [] as Array<{ label: string, value: string }>,
    regionPickerVisible: false,
    selectedRegionIdx: 0,
    selectedRegionId: 0,
    selectedRegionValue: '',
    metrics: [] as Array<{ label: string, value: string, change: string, trend: 'up' | 'down' }>,
    regionSummary: {
      regionName: '',
      merchantText: '-',
      riderText: '-',
      completionRate: '-',
      commission: '-'
    } as OperatorAnalyticsRegionSummary,
    topMerchants: [] as OperatorMerchantRankingView[],
    topRiders: [] as OperatorRiderRankingView[]
  },

  async onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    await this.loadRegions()
    this.loadData()
  },

  async loadRegions() {
    const regionState = await loadOperatorRegions()
    this.setData(regionState)
  },

  async loadData() {
    this.setData({ loading: true, error: null })
    try {
      const nextView = await loadOperatorAnalyticsPageData({
        timeDimension: this.data.timeDimension,
        selectedRegionId: this.data.selectedRegionId,
        selectedRegionName: this.data.regions[this.data.selectedRegionIdx]?.name
      })

      this.setData({
        ...nextView,
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

  onOpenRegionPicker() {
    if (!this.data.regions.length) {
      return
    }

    this.setData({ regionPickerVisible: true })
  },

  onCloseRegionPicker() {
    this.setData({ regionPickerVisible: false })
  },

  onRegionConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedValue = String(values[0] || '')
    const idx = this.data.regionPickerOptions.findIndex((item) => item.value === selectedValue)
    const region = idx >= 0 ? this.data.regions[idx] : null

    this.setData({
      regionPickerVisible: false,
      selectedRegionIdx: idx >= 0 ? idx : this.data.selectedRegionIdx,
      selectedRegionId: region?.id || this.data.selectedRegionId,
      selectedRegionValue: selectedValue || this.data.selectedRegionValue
    }, () => {
      if (region?.id) {
        this.loadData()
      }
    })
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
