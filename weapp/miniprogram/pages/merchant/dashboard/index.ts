import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService } from '../../../api/order-management'
import {
  MerchantHourlyStatRow,
  MerchantOrderSourceStatRow,
  MerchantRepurchaseRateResponse,
  MerchantStatsService
} from '../../../api/merchant-stats'
import { getMerchantComplaintSummary } from '../../../api/merchant-complaints'
import { ReservationService } from '../../../api/reservation'
import { getMyMerchantOpenStatus, getMyMerchantProfile, updateMyMerchantOpenStatus } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { getConsoleDashboardErrorState } from '../../../utils/console-dashboard'
import dayjs from 'dayjs'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { playNewOrderAlert, destroyAudioAlert } from '../../../utils/audio-alert'

type WsUnsubscribe = () => void

const DASHBOARD_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const DASHBOARD_INSIGHT_REFRESH_WINDOW_MS = 5 * 60 * 1000

interface DashboardTodoItem {
  id: string
  title: string
  path: string
  count: number
  accent: 'urgent' | 'warning' | 'neutral'
}

interface DashboardInsightItem {
  id: string
  title: string
  value: string
  desc: string
}

interface DashboardInsightSource {
  hourlyStats: MerchantHourlyStatRow[]
  sourceStats: MerchantOrderSourceStatRow[]
  repurchase: MerchantRepurchaseRateResponse | null
}

interface DashboardCollaborationItem {
  id: string
  title: string
  desc: string
  path: string
}

const DASHBOARD_COLLABORATION_ITEMS: DashboardCollaborationItem[] = [
  {
    id: 'reviews',
    title: '评价管理',
    desc: '查看顾客评价并及时处理回复。',
    path: '/pages/merchant/reviews/index'
  }
]

const ORDER_TYPE_LABELS: Record<string, string> = {
  takeout: '外卖',
  dine_in: '堂食',
  takeaway: '自取',
  reservation: '预订'
}

function buildTodoItems(params: {
  pendingPaidOrders: number
  pendingReservations: number
  pendingComplaints: number
  printAnomalies: number
}): DashboardTodoItem[] {
  const items: DashboardTodoItem[] = [
    {
      id: 'paidOrders',
      title: '待接单',
      path: '/pages/merchant/orders/list/index?status=paid',
      count: params.pendingPaidOrders,
      accent: 'urgent'
    },
    {
      id: 'reservations',
      title: '待确认预订',
      path: '/pages/merchant/reservations/index',
      count: params.pendingReservations,
      accent: 'warning'
    },
    {
      id: 'complaints',
      title: '待回复投诉',
      path: '/pages/merchant/complaints/index',
      count: params.pendingComplaints,
      accent: 'warning'
    },
    {
      id: 'printAnomalies',
      title: '打印异常',
      path: '/pages/merchant/orders/print-anomalies/index',
      count: params.printAnomalies,
      accent: 'neutral'
    }
  ]

  return items.filter((item) => item.count > 0)
}

function getTodoCount(items: DashboardTodoItem[], id: string) {
  const matched = items.find((item) => item.id === id)
  return typeof matched?.count === 'number' ? matched.count : 0
}

function formatAmount(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return `${value.toFixed(1)}%`
}

function buildInsightItems(params: {
  hourlyStats?: MerchantHourlyStatRow[]
  sourceStats?: MerchantOrderSourceStatRow[]
  repurchase?: MerchantRepurchaseRateResponse
}): DashboardInsightItem[] {
  const insights: DashboardInsightItem[] = []

  const peakHour = (params.hourlyStats || []).reduce<MerchantHourlyStatRow | null>((best, current) => {
    if (!best) return current
    return (current.order_count || 0) > (best.order_count || 0) ? current : best
  }, null)

  insights.push({
    id: 'peak-hour',
    title: '今日高峰',
    value: peakHour ? `${String(peakHour.hour).padStart(2, '0')}:00` : '--',
    desc: peakHour ? `${peakHour.order_count || 0} 单，客单 ${formatAmount(peakHour.avg_order_amount || 0)}` : '今日暂未形成明显高峰'
  })

  const sourceRows = params.sourceStats || []
  const totalSourceOrders = sourceRows.reduce((sum, item) => sum + (item.order_count || 0), 0)
  const dominantSource = sourceRows.reduce<MerchantOrderSourceStatRow | null>((best, current) => {
    if (!best) return current
    return (current.order_count || 0) > (best.order_count || 0) ? current : best
  }, null)

  insights.push({
    id: 'order-structure',
    title: '订单结构',
    value: dominantSource ? (ORDER_TYPE_LABELS[dominantSource.order_type] || dominantSource.order_type || '其他') : '--',
    desc: dominantSource && totalSourceOrders > 0
      ? `占比 ${(((dominantSource.order_count || 0) / totalSourceOrders) * 100).toFixed(1)}%，共 ${dominantSource.order_count || 0} 单`
      : '今日暂无来源结构数据'
  })

  insights.push({
    id: 'repurchase',
    title: '近30天复购',
    value: params.repurchase ? formatPercent(params.repurchase.repurchase_rate || 0) : '--',
    desc: params.repurchase
      ? `${params.repurchase.repeat_users || 0} 位复购用户，人均 ${Number.isFinite(params.repurchase.avg_orders_per_user) ? params.repurchase.avg_orders_per_user.toFixed(2) : '--'} 单`
      : '近30天复购数据暂未同步'
  })

  return insights
}

function buildBusinessStatusText(isOpen: boolean, todoCount: number) {
  if (isOpen) {
    return todoCount > 0 ? `营业中，当前有 ${todoCount} 项待跟进` : '营业中，当前经营平稳'
  }

  return todoCount > 0 ? `已暂停接单，仍有 ${todoCount} 项待处理` : '已暂停接单，可优先处理配置与复盘'
}

function buildRefreshErrorMessage(messages: string[]) {
  const normalized = messages.filter((message) => typeof message === 'string' && message.trim())
  if (!normalized.length) return ''
  const unique = Array.from(new Set(normalized))
  return `${unique.join('；')}，当前已保留上次同步结果`
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    refreshErrorCanRetry: true,
    isOpen: true,
    merchantInfo: {
      name: '示例餐厅',
      merchant_id: 0
    },
    todayStats: {
      revenue: 0,
      orderCount: 0,
      avgOrderPrice: 0
    },
    todayTodos: [] as DashboardTodoItem[],
    insightSource: {
      hourlyStats: [],
      sourceStats: [],
      repurchase: null
    } as DashboardInsightSource,
    insightItems: buildInsightItems({}) as DashboardInsightItem[],
    insightLoading: false,
    insightErrorMessage: '',
    collaborationItems: DASHBOARD_COLLABORATION_ITEMS,
    businessStatusText: '营业中，当前经营平稳',
    loading: false,
    lastPrimaryRefreshAt: 0,
    lastInsightRefreshAt: 0,
    businessStatusSubmitting: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialErrorCanRetry: true,
    _wsListeners: [] as WsUnsubscribe[]
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    // 从全局或存储中获取商户信息
    const currentMerchant = wx.getStorageSync('current_merchant')
    if (currentMerchant) {
      this.setData({ merchantInfo: currentMerchant })
    }

    const accessResult = await ensureMerchantConsoleAccess()
    if (accessResult.status !== 'granted') {
      this.setData({
        accessReady: true,
        accessDenied: accessResult.status === 'denied',
        accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
        initialLoading: false
      })
      return
    }

    this.setData({ accessReady: true, accessDenied: false, accessErrorMessage: '' })
    this.refreshData({ force: true }).catch((err) => logger.error('Merchant dashboard initial refresh failed', err))
  },

  onShow() {
    if (this.data.accessDenied || this.data.accessErrorMessage || !this.data.accessReady) return

    if (shouldAutoRefresh(this.data.lastPrimaryRefreshAt, DASHBOARD_AUTO_REFRESH_WINDOW_MS)) {
      this.refreshData().catch((err) => logger.error('Merchant dashboard onShow refresh failed', err))
      return
    }

    this.syncOpenStatusOnShow()

    if (shouldAutoRefresh(this.data.lastInsightRefreshAt, DASHBOARD_INSIGHT_REFRESH_WINDOW_MS)) {
      this.refreshInsights().catch((err) => logger.error('Merchant dashboard onShow insights refresh failed', err))
    }
  },

  onHide() {
    this.stopRealtimeRuntime()
  },

  onUnload() {
    this.stopRealtimeRuntime()
    destroyAudioAlert()
  },

  updateCurrentMerchantCache(params: {
    merchantId?: number
    merchantName?: string
    isOpen?: boolean
  } = {}) {
    try {
      const currentMerchant = wx.getStorageSync('current_merchant') || {}
      const merchantId = params.merchantId ?? currentMerchant.id ?? currentMerchant.merchant_id ?? this.data.merchantInfo.merchant_id
      const merchantName = params.merchantName ?? currentMerchant.name ?? this.data.merchantInfo.name
      wx.setStorageSync('current_merchant', {
        ...currentMerchant,
        ...(merchantId
          ? {
              id: merchantId,
              merchant_id: merchantId
            }
          : {}),
        ...(merchantName ? { name: merchantName } : {}),
        ...(typeof params.isOpen === 'boolean' ? { is_open: params.isOpen } : {})
      })
    } catch (storageErr) {
      logger.warn('Sync current merchant cache failed', storageErr)
    }
  },

  async syncOpenStatusOnShow() {
    try {
      const merchantOpenStatus = await getMyMerchantOpenStatus()
      const todoCount = this.data.todayTodos.reduce((sum, item) => sum + item.count, 0)
      this.setData({
        isOpen: merchantOpenStatus.is_open,
        businessStatusText: buildBusinessStatusText(merchantOpenStatus.is_open, todoCount)
      })
      this.updateCurrentMerchantCache({ isOpen: merchantOpenStatus.is_open })
      this.syncRealtimeRuntime(merchantOpenStatus.is_open)
    } catch (err) {
      logger.warn('Merchant dashboard onShow open status sync failed', err)
    }
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '', initialLoading: true })
    this.onLoad()
  },

  initWebSocket() {
    // 先清除旧监听，再发起连接（保证不重复注册）
    this.cleanupWebSocket()
    wsManager.connect()

    const sub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      logger.info('Merchant received notification', data)
      const notification =
        typeof data === 'object' && data !== null
          ? (data as { type?: string })
          : {}
      // 检查是否是订单通知 (后端 params.Type = "order")
      if (notification.type === 'order') {
        wx.vibrateLong()
        playNewOrderAlert()
        wx.showModal({
          title: '新订单提醒',
          content: '您有新的订单需要处理',
          confirmText: '去处理',
          success: (res) => {
            if (res.confirm) {
              this.onGoOrderList()
            }
          }
        })
        this.refreshData({
          showLoading: false,
          force: true,
          refreshInsights: false
        }).catch((err) => logger.error('Merchant dashboard realtime order refresh failed', err))
      }
    })

    this.data._wsListeners = [sub]
  },

  syncRealtimeRuntime(isOpen: boolean) {
    if (!isOpen) {
      this.stopRealtimeRuntime()
      return
    }

    this.initWebSocket()
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub) => unsub())
      this.data._wsListeners = []
    }
  },

  stopRealtimeRuntime() {
    this.cleanupWebSocket()
    wsManager.disconnect()
  },

  async refreshData(options: { showLoading?: boolean, force?: boolean, refreshInsights?: boolean } = {}) {
    if (this.data.loading) return

    const isFirstLoad = this.data.initialLoading
    const showLoading = options.showLoading ?? isFirstLoad
    const force = options.force === true
    const refreshInsights = options.refreshInsights !== false
    const hasExistingData = !isFirstLoad
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && !shouldAutoRefresh(this.data.lastPrimaryRefreshAt, DASHBOARD_AUTO_REFRESH_WINDOW_MS)) {
      if (refreshInsights && shouldAutoRefresh(this.data.lastInsightRefreshAt, DASHBOARD_INSIGHT_REFRESH_WINDOW_MS)) {
        this.refreshInsights().catch((err) => logger.error('Merchant dashboard deferred insights refresh failed', err))
      }
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '', refreshErrorCanRetry: true }
        : isSilentRefresh
          ? { refreshErrorMessage: '', refreshErrorCanRetry: true }
          : {})
    })

    let shouldKickOffInsights = false

    try {
      const today = dayjs().format('YYYY-MM-DD')
      let runtimeLoaded = false
      let summaryLoaded = false
      let resolvedIsOpen = this.data.isOpen
      const refreshErrors: string[] = []
      const refreshErrorStates: Array<{ message: string, canRetry: boolean }> = []
      const addRefreshError = (error: unknown, fallback: string) => {
        const errorState = getConsoleDashboardErrorState('merchant', error, fallback)
        refreshErrors.push(errorState.message)
        refreshErrorStates.push({ message: errorState.message, canRetry: errorState.canRetry })
      }

      const [merchantProfileResult, merchantOpenStatusResult] = await settleAll([
        getMyMerchantProfile(),
        getMyMerchantOpenStatus()
      ] as const)

      if (merchantProfileResult.status === 'fulfilled') {
        const merchantProfile = merchantProfileResult.value
        const merchantOpenStatus = merchantOpenStatusResult.status === 'fulfilled'
          ? merchantOpenStatusResult.value
          : null

        runtimeLoaded = true
        resolvedIsOpen = merchantOpenStatus?.is_open ?? merchantProfile.is_open
        this.setData({
          merchantInfo: {
            name: merchantProfile.name,
            merchant_id: merchantProfile.id
          },
          isOpen: resolvedIsOpen
        })
        this.syncRealtimeRuntime(resolvedIsOpen)
        this.updateCurrentMerchantCache({
          merchantId: merchantProfile.id,
          merchantName: merchantProfile.name,
          isOpen: merchantOpenStatus?.is_open ?? merchantProfile.is_open
        })
      } else {
        logger.error('Failed to fetch merchant runtime status', merchantProfileResult.reason)
        this.stopRealtimeRuntime()
        addRefreshError(merchantProfileResult.reason, '工作台基础信息加载失败，请重试')
      }

      const [overview, reservationStats, complaintResult, paidOrderSummary] = await settleAll([
        MerchantStatsService.getOverview({
          start_date: today,
          end_date: today
        }),
        ReservationService.getReservationStats(),
        getMerchantComplaintSummary(),
        MerchantOrderManagementService.getOrderSummary()
      ] as const)

      const currentTodoItems = this.data.todayTodos
      const pendingReservations = reservationStats.status === 'fulfilled'
        ? reservationStats.value.paid_count || 0
        : getTodoCount(currentTodoItems, 'reservations')
      const pendingComplaints = complaintResult.status === 'fulfilled'
        ? complaintResult.value.pending_response || 0
        : getTodoCount(currentTodoItems, 'complaints')
      const printAnomalies = overview.status === 'fulfilled'
        ? overview.value.print_anomalies_count || 0
        : getTodoCount(currentTodoItems, 'printAnomalies')

      if (overview.status === 'fulfilled') {
        summaryLoaded = true
        const overviewData = overview.value
        const orderCount = overviewData.total_orders || 0
        const revenue = overviewData.total_sales || 0

        this.setData({
          todayStats: {
            revenue,
            orderCount,
            avgOrderPrice: orderCount > 0 ? Math.round(revenue / orderCount) : 0
          }
        })
      } else {
        logger.error('Failed to fetch merchant overview', overview.reason)
        addRefreshError(overview.reason, '经营概览加载失败，请稍后重试')
      }

      if (reservationStats.status === 'rejected') {
        logger.error('Failed to fetch reservation reminders', reservationStats.reason)
        addRefreshError(reservationStats.reason, '预订待办同步失败，请稍后重试')
      }

      if (complaintResult.status === 'rejected') {
        logger.error('Failed to fetch complaint reminders', complaintResult.reason)
        addRefreshError(complaintResult.reason, '投诉待办同步失败，请稍后重试')
      }

      const pendingPaidOrders = paidOrderSummary.status === 'fulfilled'
        ? paidOrderSummary.value.paid_count || 0
        : getTodoCount(currentTodoItems, 'paidOrders')

      if (paidOrderSummary.status === 'rejected') {
        logger.error('Load dashboard order flow failed', paidOrderSummary.reason)
        addRefreshError(paidOrderSummary.reason, '待接单数量同步失败，请稍后重试')
      }

      const todayTodos = buildTodoItems({
        pendingPaidOrders,
        pendingReservations,
        pendingComplaints,
        printAnomalies
      })
      const todoCount = todayTodos.reduce((sum, item) => sum + item.count, 0)

      this.setData({
        todayTodos,
        businessStatusText: buildBusinessStatusText(resolvedIsOpen, todoCount)
      })

      if (isFirstLoad && (!runtimeLoaded || !summaryLoaded)) {
        const initialErrorState = refreshErrorStates[0] || getConsoleDashboardErrorState(
          'merchant',
          refreshErrors[0],
          '商户工作台暂时无法加载，请稍后重试。'
        )
        this.setData({
          initialError: true,
          initialErrorMessage: initialErrorState.message,
          initialErrorCanRetry: initialErrorState.canRetry
        })
      } else {
        this.setData({
          initialError: false,
          initialErrorMessage: '',
          initialErrorCanRetry: true,
          refreshErrorMessage: buildRefreshErrorMessage(refreshErrors),
          refreshErrorCanRetry: refreshErrorStates.every((state) => state.canRetry),
          lastPrimaryRefreshAt: Date.now()
        })
        shouldKickOffInsights = refreshInsights
      }
    } catch (err) {
      logger.error('Merchant dashboard refresh failed', err)
      const errorState = getConsoleDashboardErrorState('merchant', err, '商户工作台暂时无法加载，请稍后重试。')
      if (isFirstLoad) {
        this.setData({
          initialError: true,
          initialErrorMessage: errorState.message,
          initialErrorCanRetry: errorState.canRetry
        })
      } else {
        this.setData({
          refreshErrorMessage: errorState.canRetry ? `${errorState.message}，当前已保留上次同步结果` : errorState.message,
          refreshErrorCanRetry: errorState.canRetry
        })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false })
      wx.stopPullDownRefresh()
    }

    if (shouldKickOffInsights && (force || shouldAutoRefresh(this.data.lastInsightRefreshAt, DASHBOARD_INSIGHT_REFRESH_WINDOW_MS))) {
      this.refreshInsights({ force }).catch((err) => logger.error('Merchant dashboard insights refresh failed', err))
    }
  },

  async refreshInsights(options: { force?: boolean } = {}) {
    if (this.data.insightLoading) return

    const force = options.force === true
    const hasExistingInsights = this.data.lastInsightRefreshAt > 0

    if (!force && !shouldAutoRefresh(this.data.lastInsightRefreshAt, DASHBOARD_INSIGHT_REFRESH_WINDOW_MS)) {
      return
    }

    this.setData({ insightLoading: true, insightErrorMessage: '' })

    try {
      const today = dayjs().format('YYYY-MM-DD')
      const recentThirtyDays = dayjs().subtract(29, 'day').format('YYYY-MM-DD')
      const [hourlyResult, sourceResult, repurchaseResult] = await settleAll([
        MerchantStatsService.getHourlyStats({ start_date: today, end_date: today }),
        MerchantStatsService.getOrderSources({ start_date: today, end_date: today }),
        MerchantStatsService.getRepurchaseRate({ start_date: recentThirtyDays, end_date: today })
      ] as const)

      const insightErrors: string[] = []
      const nextInsightSource: DashboardInsightSource = {
        hourlyStats: hourlyResult.status === 'fulfilled' ? hourlyResult.value : this.data.insightSource.hourlyStats,
        sourceStats: sourceResult.status === 'fulfilled' ? sourceResult.value : this.data.insightSource.sourceStats,
        repurchase: repurchaseResult.status === 'fulfilled' ? repurchaseResult.value : this.data.insightSource.repurchase
      }

      if (hourlyResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard hourly insights', hourlyResult.reason)
        insightErrors.push(getConsoleDashboardErrorState('merchant', hourlyResult.reason, '高峰时段同步失败，请稍后重试').message)
      }

      if (sourceResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard source insights', sourceResult.reason)
        insightErrors.push(getConsoleDashboardErrorState('merchant', sourceResult.reason, '订单结构同步失败，请稍后重试').message)
      }

      if (repurchaseResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard repurchase insights', repurchaseResult.reason)
        insightErrors.push(getConsoleDashboardErrorState('merchant', repurchaseResult.reason, '复购数据同步失败，请稍后重试').message)
      }

      this.setData({
        insightSource: nextInsightSource,
        insightItems: buildInsightItems({
          hourlyStats: nextInsightSource.hourlyStats,
          sourceStats: nextInsightSource.sourceStats,
          repurchase: nextInsightSource.repurchase || undefined
        }),
        insightErrorMessage: insightErrors.length
          ? hasExistingInsights
            ? `${buildRefreshErrorMessage(insightErrors)}`
            : buildRefreshErrorMessage(insightErrors).replace('，当前已保留上次同步结果', '')
          : '',
        lastInsightRefreshAt: Date.now()
      })
    } catch (err) {
      logger.error('Merchant dashboard insights refresh failed', err)
      const errorState = getConsoleDashboardErrorState('merchant', err, '经营洞察同步失败，请稍后重试')
      this.setData({
        insightErrorMessage: hasExistingInsights ? `${errorState.message}，当前已保留上次同步结果` : errorState.message
      })
    } finally {
      this.setData({ insightLoading: false })
    }
  },

  onPullDownRefresh() {
    this.refreshData({ showLoading: false, force: true, refreshInsights: true })
  },

  onGoOrderList() {
    wx.navigateTo({ url: '/pages/merchant/orders/list/index' })
  },

  onGoKitchen() {
    wx.navigateTo({ url: '/pages/merchant/kitchen/index' })
  },

  onGoStats() {
    wx.navigateTo({ url: '/pages/merchant/stats/index' })
  },

  onGoShortcut(e: WechatMiniprogram.TouchEvent) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    const { path } = e.currentTarget.dataset as { path?: string }
    if (!path) return
    wx.navigateTo({ url: path })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    this.refreshData({ force: true, refreshInsights: true })
  },

  onRetryRefresh() {
    this.refreshData({ showLoading: false, force: true, refreshInsights: true })
  },

  async onToggleBusiness() {
    if (this.data.businessStatusSubmitting) return

    const targetOpen = !this.data.isOpen
    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: targetOpen ? '确认开始营业' : '确认暂停接单',
        content: targetOpen
          ? '恢复营业后，顾客可以重新下单，工作台会继续接收新订单提醒。'
          : '暂停接单后，顾客暂时无法提交新订单，已生成订单仍需继续处理。',
        confirmText: targetOpen ? '确认开门' : '确认打烊',
        cancelText: '取消',
        success: (res) => resolve(!!res.confirm),
        fail: () => resolve(false)
      })
    })

    if (!confirmed) return

    this.setData({ businessStatusSubmitting: true })

    try {
      const response = await updateMyMerchantOpenStatus(targetOpen)
      const todoCount = this.data.todayTodos.reduce((sum, item) => sum + item.count, 0)
      this.setData({
        isOpen: response.is_open,
        businessStatusText: buildBusinessStatusText(response.is_open, todoCount)
      })
      this.refreshData({ force: true }).catch((err) => logger.error('Refresh dashboard after toggle failed', err))
    } catch (err: unknown) {
      logger.error('Update merchant open status failed', err)
      let message = '营业状态更新失败'
      if (typeof err === 'object' && err !== null && 'userMessage' in err) {
        const userMessage = (err as { userMessage?: unknown }).userMessage
        if (typeof userMessage === 'string' && userMessage.trim()) {
          message = userMessage
        }
      }
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ businessStatusSubmitting: false })
    }
  },

  onGoToSettings() {
    wx.navigateTo({ url: '/pages/merchant/config/index' })
  }
})
