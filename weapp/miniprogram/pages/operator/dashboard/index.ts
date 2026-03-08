import { responsiveBehavior } from '@/utils/responsive'
import { formatPriceNoSymbol } from '@/utils/util'
import { operatorBasicManagementService, RegionResponse } from '../../../api/operator-basic-management'
import { operatorAnalyticsService, OperatorAppealService } from '../../../api/operator-analytics'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'
import { operatorRiderManagementService } from '../../../api/operator-rider-management'

interface PendingApprovalItem {
  id: number
  type: 'MERCHANT' | 'RIDER' | 'APPEAL'
  name: string
  time: string
}

interface RiderRankingDisplayItem {
  completion_rate: string
  [key: string]: unknown
}

const appealService = new OperatorAppealService()

Page({
  behaviors: [responsiveBehavior],
  data: {
    // 基础统计
    stats: {
      total_gmv_display: '0.00',
      total_orders: 0,
      active_merchants: 0,
      active_riders: 0,
      today_gmv_display: '0.00',
      today_orders: 0,
      today_income_display: '0.00'
    },
    
    // 财务概览
    finance: {
      balance_display: '0.00',
      total_income_display: '0.00',
      current_month_income_display: '0.00'
    },

    // 筛选维度: day | week | month
    timeDimension: 'day',
    
    // 待办事项
    pending_approvals: [] as PendingApprovalItem[],
    pending_count: 0,
    
    // 排行榜
    merchantRankings: [] as Array<Record<string, unknown>>,
    riderRankings: [] as RiderRankingDisplayItem[],
    rankingType: 'merchant', // merchant | rider

    // 区域切换
    regions: [] as RegionResponse[],
    selectedRegionIdx: 0,
    selectedRegionId: 0,

    loading: false,
    initialLoading: true,
    error: null as string | null,
    showApplymentEntry: false,
    navBarHeight: 88
  },

  onLoad() {
    this.initDashboard()
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.refreshData()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async initDashboard() {
    this.setData({ initialLoading: true, error: null })
    // 先加载区域列表，再加载看板数据
    try {
      const regionsResult = await operatorBasicManagementService.getOperatorRegions({ limit: 100 })
      const regions = regionsResult.regions || []
      this.setData({
        regions,
        selectedRegionIdx: 0,
        selectedRegionId: regions[0]?.id || 0
      })
    } catch {
      // 区域加载失败不阻断页面
    }
    await this.loadDashboardData()
    this.setData({ initialLoading: false })
  },

  async refreshData() {
    await this.loadDashboardData()
  },

  /**
   * 按时间维度加载指标和排行
   */
  async loadDashboardData() {
    if (this.data.loading) return
    this.setData({ loading: true })
    
    try {
      const { timeDimension, selectedRegionId } = this.data
      const { startDate, endDate } = this.getDateRange(timeDimension)
      const regionId = selectedRegionId || undefined
      
      // 1. 并行获取各项数据（财务概览失败不阻断运营中心）
      const [
        financeOverview,
        realtimeStats,
        merchantsPending,
        ridersPending,
        merchantRanking,
        riderRanking,
        dailyTrends,
        appeals
      ] = await Promise.all([
        operatorBasicManagementService.getFinanceOverview(undefined, undefined, regionId).catch(() => null),
        operatorAnalyticsService.getRealtimeStats(regionId),
        operatorMerchantManagementService.getMerchantList({ page: 1, limit: 10, status: 'pending', region_id: regionId })
          .catch(() => ({ merchants: [] as Array<{ id: number, name: string, created_at: string }> })),
        operatorRiderManagementService.getRiderList({ page: 1, limit: 10, status: 'pending', region_id: regionId })
          .catch(() => ({ riders: [] as Array<{ id: number, name: string, created_at: string }> })),
        operatorMerchantManagementService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
          .catch(() => []),
        operatorRiderManagementService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
          .catch(() => []),
        operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
          .catch(() => []),
        appealService.getAppealList({ page: 1, limit: 5, status: 'pending', region_id: regionId })
          .catch(() => ({ appeals: [] as Array<{ id: number, reason: string, created_at: string }> }))
      ])

      // 格式化处理各维度数据，确保兼容性
      const today = new Date().toISOString().split('T')[0]
      const trends = Array.isArray(dailyTrends) ? dailyTrends : []
      // 后端现在返回 operator_income 字段，直接使用无需前端计算
      const todayTrend = trends.find((t) => t.date === today) || { total_gmv: 0, order_count: 0, operator_income: 0 }
      
      const merchantRankList = Array.isArray(merchantRanking) ? merchantRanking : []
      const riderRankList = (Array.isArray(riderRanking) ? riderRanking : []).map((r: Record<string, unknown>) => ({
        ...r,
        completion_rate:
          typeof r.completion_rate === 'number'
            ? r.completion_rate.toFixed(1)
            : '0.0'
      }))

      // 待办事项组合
      const pendingItems = [
        ...(merchantsPending.merchants || []).map((m: { id: number, name: string, created_at: string }) => ({ id: m.id, type: 'MERCHANT', name: m.name, time: m.created_at })),
        ...(ridersPending.riders || []).map((r: { id: number, name: string, created_at: string }) => ({ id: r.id, type: 'RIDER', name: r.name, time: r.created_at })),
        ...(appeals.appeals || []).map((a: { id: number, reason?: string, created_at: string }) => ({ id: a.id, type: 'APPEAL', name: `客诉: ${a.reason || ('#' + a.id)}`, time: a.created_at }))
      ] as PendingApprovalItem[]

      pendingItems.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())

      this.setData({
        stats: {
          total_gmv_display: formatPriceNoSymbol(financeOverview?.total?.total_gmv ?? 0),
          total_orders: financeOverview?.current_month?.total_orders ?? 0,
          active_merchants: realtimeStats.active_merchant_count,
          active_riders: realtimeStats.active_rider_count,
          today_gmv_display: formatPriceNoSymbol(todayTrend.total_gmv),
          today_orders: todayTrend.order_count,
          // 使用后端计算的运营商可得金额，不再前端硬编码分成比例
          today_income_display: formatPriceNoSymbol(todayTrend.operator_income ?? 0)
        },
        finance: {
          // 使用后端返回的 operator_income 字段，遵循 SSOT 原则
          balance_display: formatPriceNoSymbol(financeOverview?.total?.operator_income ?? 0),
          total_income_display: formatPriceNoSymbol(financeOverview?.total?.operator_income ?? 0),
          current_month_income_display: formatPriceNoSymbol(financeOverview?.current_month?.operator_income ?? 0)
        },
        merchantRankings: merchantRankList,
        riderRankings: riderRankList,
        pending_approvals: pendingItems.slice(0, 5),
        pending_count: pendingItems.length,
        loading: false,
        error: null
      })

      if (!financeOverview) {
        console.warn('运营中心财务概览降级：finance overview unavailable')
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '数据加载失败，请重试'
      console.error('加载运营仪表盘失败:', error)
      const normalized = message.toLowerCase()
      const showApplymentEntry =
        normalized.includes('operator account is not active') ||
        normalized.includes('operator is not active') ||
        normalized.includes('开户') ||
        normalized.includes('bindbank')
      this.setData({ 
        loading: false,
        error: message,
        showApplymentEntry
      })
    }
  },

  /**
   * 获取日期范围
   */
  getDateRange(dimension: string) {
    const end = new Date()
    const start = new Date()
    
    if (dimension === 'day') {
      start.setHours(0, 0, 0, 0)
    } else if (dimension === 'week') {
      start.setDate(end.getDate() - 7)
    } else if (dimension === 'month') {
      start.setMonth(end.getMonth() - 1)
    }
    
    return {
      startDate: start.toISOString().split('T')[0],
      endDate: end.toISOString().split('T')[0]
    }
  },

  /**
   * 切换区域
   */
  onRegionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const idx = parseInt(e.detail.value)
    const regionId = this.data.regions[idx]?.id || 0
    this.setData({ selectedRegionIdx: idx, selectedRegionId: regionId }, () => {
      this.loadDashboardData()
    })
  },

  /**
   * 切换时间维度
   */
  onTimeDimensionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const dimension = e.detail.value
    this.setData({ timeDimension: dimension }, () => {
      this.loadDashboardData()
    })
  },

  /**
   * 切换排行榜类型
   */
  onRankingTypeChange(e: WechatMiniprogram.CustomEvent<{ value: 'merchant' | 'rider' }>) {
    this.setData({ rankingType: e.detail.value })
  },

  onRetry() {
    this.initDashboard()
  },

  /**
   * 处理待办点击
   */
  onPendingTap(e: WechatMiniprogram.TouchEvent) {
    const { id, type } = e.currentTarget.dataset as { id?: number, type?: PendingApprovalItem['type'] }
    let url = ''
    if (type === 'MERCHANT') url = `/pages/operator/merchants/detail/index?id=${id}`
    else if (type === 'RIDER') url = `/pages/operator/riders/detail/index?id=${id}`
    else if (type === 'APPEAL') url = `/pages/operator/appeal/detail/index?id=${id}`
    
    if (url) wx.navigateTo({ url })
  },

  onPendingViewAll() {
    const { selectedRegionId } = this.data
    const query = selectedRegionId ? `?region_id=${selectedRegionId}` : ''
    wx.navigateTo({ url: `/pages/operator/appeal/list/index${query}` })
  },

  onOpenApplyment() {
    wx.navigateTo({ url: '/pages/operator/applyment/index' })
  }
})
