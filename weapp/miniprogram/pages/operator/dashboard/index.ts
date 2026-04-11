import { responsiveBehavior } from '@/utils/responsive'
import { getConsoleDashboardErrorState, shouldShowOperatorApplymentEntry } from '../../../utils/console-dashboard'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import {
  buildAbnormalRefundClipboardText,
  formatPlatformAlertTime,
  toActionableAbnormalRefundAlert,
  type ActionableAbnormalRefundAlert
} from '../../../utils/platform-alerts'
import {
  loadOperatorAlertFeed,
  loadOperatorCenterPageData,
  loadOperatorRegions,
  type ConsoleRegionOption
} from '../../../services/operator-console'

type TimeDimension = 'day' | 'week' | 'month'

interface PendingSummary {
  merchants: number
  riders: number
  appeals: number
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
    regions: [] as ConsoleRegionOption[],
    regionPickerOptions: [] as Array<{ label: string, value: string }>,
    regionPickerVisible: false,
    selectedRegionIdx: 0,
    selectedRegionId: 0,
    selectedRegionValue: '',

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
    const regionState = await loadOperatorRegions()
    this.setData(regionState)
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
      const nextView = await loadOperatorCenterPageData({
        timeDimension: this.data.timeDimension,
        selectedRegionId: this.data.selectedRegionId
      })

      this.setData({
        ...nextView,
        loading: false,
        error: null
      })
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

  onOpenRegionPicker() {
    if (this.data.regions.length <= 1) {
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
        this.loadDashboardData()
      }
    })
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
      this.setData({
        abnormalRefundAlerts: await loadOperatorAlertFeed(),
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
