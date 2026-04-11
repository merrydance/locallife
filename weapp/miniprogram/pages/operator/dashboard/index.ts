import { responsiveBehavior } from '@/utils/responsive'
import { formatPriceNoSymbol } from '@/utils/util'
import { operatorBasicManagementService, RegionResponse } from '../../../api/operator-basic-management'
import { operatorAnalyticsService, OperatorAppealService } from '../../../api/operator-analytics'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'
import { operatorRiderManagementService } from '../../../api/operator-rider-management'
import { getConsoleDashboardErrorState, shouldShowOperatorApplymentEntry } from '../../../utils/console-dashboard'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { platformAlertsService } from '../../../api/platform-alerts'
import {
  buildAbnormalRefundClipboardText,
  formatPlatformAlertTime,
  toActionableAbnormalRefundAlert,
  type ActionableAbnormalRefundAlert
} from '../../../utils/platform-alerts'

type TimeDimension = 'day' | 'week' | 'month'

interface PendingSummary {
  merchants: number
  riders: number
  appeals: number
}

interface TrendSummary {
  totalGmv: number
  totalOrders: number
  totalIncome: number
}

interface DailyTrendItemLike {
  date?: string
  total_gmv?: number
  order_count?: number
  operator_income?: number
}

function summarizeTrends(trends: DailyTrendItemLike[]): TrendSummary {
  return trends.reduce<TrendSummary>((summary, item) => ({
    totalGmv: summary.totalGmv + Number(item.total_gmv || 0),
    totalOrders: summary.totalOrders + Number(item.order_count || 0),
    totalIncome: summary.totalIncome + Number(item.operator_income || 0)
  }), {
    totalGmv: 0,
    totalOrders: 0,
    totalIncome: 0
  })
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

interface AlertFeedItem extends ActionableAbnormalRefundAlert {
  timeDisplay: string
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
    timeDimension: 'day' as TimeDimension,
    
    // 待办事项
    pending_approvals: [] as PendingApprovalItem[],
    pending_count: 0,
    pendingSummary: {
      merchants: 0,
      riders: 0,
      appeals: 0
    } as PendingSummary,
    
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
    errorCanRetry: true,
    showApplymentEntry: false,
    navBarHeight: 88,
    abnormalRefundAlerts: [] as AlertFeedItem[],
    alertFeedReady: false,
    _alertListeners: [] as Array<() => void>
  },

  onLoad() {
    this.initDashboard()
    this.initAlertFeed()
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.refreshData()
    }
    if (this.data._alertListeners.length === 0) {
      this.startAlertFeed()
    }
  },

  onHide() {
    this.stopAlertFeed()
  },

  onUnload() {
    this.stopAlertFeed()
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
        merchantSummary,
        riderSummary,
        appealSummary,
        merchantsPending,
        ridersPending,
        merchantRanking,
        riderRanking,
        dailyTrends,
        appeals
      ] = await Promise.all([
        operatorBasicManagementService.getFinanceOverview(undefined, undefined, regionId).catch(() => null),
        operatorAnalyticsService.getRealtimeStats(regionId),
        operatorMerchantManagementService.getMerchantSummary(regionId)
          .catch(() => ({ total: 0, pending: 0, approved: 0, rejected: 0, suspended: 0 })),
        operatorRiderManagementService.getRiderSummary(regionId)
          .catch(() => ({ total: 0, pending_approval: 0, active: 0, rejected: 0, suspended: 0, online: 0 })),
        appealService.getAppealSummary(regionId)
          .catch(() => ({ total: 0, pending: 0, approved: 0, rejected: 0 })),
        operatorMerchantManagementService.getMerchantList({ page: 1, limit: 5, status: 'pending', region_id: regionId })
          .catch(() => ({ merchants: [] as Array<{ id: number, name: string, created_at: string }>, total: 0 })),
        operatorRiderManagementService.getRiderList({ page: 1, limit: 5, status: 'pending_approval', region_id: regionId })
          .catch(() => ({ riders: [] as Array<{ id: number, name: string, created_at: string }>, total: 0 })),
        operatorMerchantManagementService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
          .catch(() => ({ rankings: [] })),
        operatorRiderManagementService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 5, region_id: regionId })
          .catch(() => ({ rankings: [] })),
        operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
          .catch(() => []),
        appealService.getAppealList({ page: 1, limit: 5, status: 'pending', region_id: regionId })
          .catch(() => ({ appeals: [] as Array<{ id: number, reason: string, created_at: string }>, total: 0 }))
      ])

      const trends = Array.isArray(dailyTrends) ? dailyTrends as DailyTrendItemLike[] : []
      const currentPeriodSummary = summarizeTrends(trends)

      const merchantRankList = normalizeRankingRows(merchantRanking)
      const riderRankList = normalizeRankingRows(riderRanking).map((r: Record<string, unknown>) => ({
        ...r,
        completion_rate:
          typeof r.completion_rate === 'number'
            ? r.completion_rate.toFixed(1)
            : '0.0'
      }))

      const pendingSummary: PendingSummary = {
        merchants: Number(merchantSummary.pending || 0),
        riders: Number(riderSummary.pending_approval || 0),
        appeals: Number(appealSummary.pending || 0)
      }

      // 待办事项组合
      const pendingItems = [
        ...(merchantsPending.merchants || []).map((m: { id: number, name: string, created_at: string }) => ({ id: m.id, type: 'MERCHANT', name: m.name, time: m.created_at })),
        ...(ridersPending.riders || []).map((r: { id: number, name: string, created_at: string }) => ({ id: r.id, type: 'RIDER', name: r.name, time: r.created_at })),
        ...(appeals.appeals || []).map((a: { id: number, reason?: string, created_at: string }) => ({ id: a.id, type: 'APPEAL', name: `客诉: ${a.reason || ('#' + a.id)}`, time: a.created_at }))
      ] as PendingApprovalItem[]

      pendingItems.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())

      this.setData({
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
        merchantRankings: merchantRankList,
        riderRankings: riderRankList,
        pending_approvals: pendingItems.slice(0, 5),
        pending_count: pendingSummary.merchants + pendingSummary.riders + pendingSummary.appeals,
        pendingSummary,
        loading: false,
        error: null
      })

      if (!financeOverview) {
        console.warn('运营中心财务概览降级：finance overview unavailable')
      }
    } catch (error: unknown) {
      const errorState = getConsoleDashboardErrorState('operator', error, '运营中心数据加载失败，请稍后重试。')
      console.error('加载运营仪表盘失败:', error)
      const showApplymentEntry = shouldShowOperatorApplymentEntry(error)
      this.setData({ 
        loading: false,
        error: errorState.message,
        errorCanRetry: errorState.canRetry,
        showApplymentEntry
      })
    }
  },

  /**
   * 获取日期范围
   */
  getDateRange(dimension: TimeDimension) {
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
  onTimeDimensionChange(e: WechatMiniprogram.CustomEvent<{ value: TimeDimension }>) {
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

  startAlertFeed() {
    this.stopAlertFeed()
    wsManager.connect('/v1/platform/ws')

    const alertSub = wsManager.on(WSMessageType.ALERT, (data) => {
      const nextAlert = toActionableAbnormalRefundAlert(data)
      if (!nextAlert) {
        return
      }

      const nextItems = [
        {
          ...nextAlert,
          timeDisplay: formatPlatformAlertTime(nextAlert.timestamp)
        },
        ...this.data.abnormalRefundAlerts.filter((item) => item.refundOrderId !== nextAlert.refundOrderId)
      ].slice(0, 5)

      this.setData({
        abnormalRefundAlerts: nextItems,
        alertFeedReady: true
      })
    })

    this.data._alertListeners = [alertSub]
  },

  async initAlertFeed() {
    try {
      await this.loadRecentPlatformAlerts()
    } finally {
      this.startAlertFeed()
    }
  },

  async loadRecentPlatformAlerts() {
    try {
      const response = await platformAlertsService.listPlatformAlerts({ page_id: 1, page_size: 10 })
      const alerts = (response.alerts || [])
        .map((item) => toActionableAbnormalRefundAlert(item))
        .filter((item): item is ActionableAbnormalRefundAlert => !!item)
        .map((item) => ({
          ...item,
          timeDisplay: formatPlatformAlertTime(item.timestamp)
        }))
        .slice(0, 5)

      this.setData({
        abnormalRefundAlerts: alerts,
        alertFeedReady: true
      })
    } catch (_error) {
      this.setData({ alertFeedReady: true })
    }
  },

  stopAlertFeed() {
    if (this.data._alertListeners.length > 0) {
      this.data._alertListeners.forEach((unsubscribe) => unsubscribe())
      this.data._alertListeners = []
    }
    wsManager.disconnect()
  },

  onAbnormalRefundAlertTap(e: WechatMiniprogram.TouchEvent) {
    const { index } = e.currentTarget.dataset as { index?: number }
    const alert = typeof index === 'number' ? this.data.abnormalRefundAlerts[index] : undefined
    if (!alert) {
      return
    }

    wx.showActionSheet({
      itemList: ['查看处理说明', '复制处理参数'],
      success: ({ tapIndex }) => {
        if (tapIndex === 0) {
          this.showAbnormalRefundGuide(alert)
          return
        }
        this.copyAbnormalRefundAlert(alert)
      }
    })
  },

  showAbnormalRefundGuide(alert: AlertFeedItem) {
    const extraFields = alert.userBankCardRequiredFields.length > 0
      ? `\n收款到用户银行卡需补充：${alert.userBankCardRequiredFields.join('、')}`
      : ''

    wx.showModal({
      title: '异常退款处理说明',
      content:
        `该退款已进入微信异常退款人工处理流程。\n\n` +
        `接口：${alert.method} ${alert.path}\n` +
        `权限：平台管理员\n` +
        `退款单ID：${alert.refundOrderId}\n` +
        `支付单ID：${alert.paymentOrderId || '-'}\n` +
        `微信退款ID：${alert.refundId}\n` +
        `默认退款去向：${alert.defaultType}\n` +
        `支持退款去向：${alert.supportedTypes.join(' / ') || alert.defaultType}` +
        extraFields +
        `\n\n${alert.message}`,
      cancelText: '关闭',
      confirmText: '复制参数',
      success: (result) => {
        if (result.confirm) {
          this.copyAbnormalRefundAlert(alert)
        }
      }
    })
  },

  copyAbnormalRefundAlert(alert: AlertFeedItem) {
    wx.setClipboardData({
      data: buildAbnormalRefundClipboardText(alert),
      success: () => {
        wx.showToast({ title: '处理参数已复制', icon: 'success' })
      }
    })
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
    const query = selectedRegionId ? `region_id=${selectedRegionId}` : ''
    const actions = [
      {
        label: `商户待审 (${this.data.pendingSummary.merchants})`,
        enabled: this.data.pendingSummary.merchants > 0,
        url: `/pages/operator/merchants/index?${query}${query ? '&' : ''}status=pending`
      },
      {
        label: `骑手待审 (${this.data.pendingSummary.riders})`,
        enabled: this.data.pendingSummary.riders > 0,
        url: `/pages/operator/riders/index?${query}${query ? '&' : ''}status=pending_approval`
      },
      {
        label: `待处理申诉 (${this.data.pendingSummary.appeals})`,
        enabled: this.data.pendingSummary.appeals > 0,
        url: `/pages/operator/appeal/list/index${query ? `?${query}` : ''}`
      }
    ].filter((item) => item.enabled)

    if (actions.length === 0) {
      return
    }

    if (actions.length === 1) {
      wx.navigateTo({ url: actions[0].url })
      return
    }

    wx.showActionSheet({
      itemList: actions.map((item) => item.label),
      success: ({ tapIndex }) => {
        const target = actions[tapIndex]
        if (!target) return
        wx.navigateTo({ url: target.url })
      }
    })
  },

  onOpenApplyment() {
    wx.navigateTo({ url: '/pages/operator/applyment/index' })
  }
})
