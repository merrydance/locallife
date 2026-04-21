import { isLargeScreen } from '@/utils/responsive'
import {
  loadOperatorRegions,
  type ConsoleRegionOption
} from '../../../services/operator-regions'
import {
  loadOperatorAnalyticsPageData,
  type OperatorAnalyticsRegionSummary,
  type OperatorMerchantRankingView,
  type OperatorRiderRankingView
} from '../../../services/operator-analytics-dashboard'

type TimeDimension = 'day' | 'week' | 'month'
type RankingType = 'merchant' | 'rider'

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
