import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService } from '../../../api/order-management'
import {
  MerchantHourlyStatRow,
  MerchantOrderSourceStatRow,
  MerchantRepurchaseRateResponse,
  MerchantStatsService
} from '../../../api/merchant-stats'
import { listMerchantComplaints } from '../../../api/merchant-complaints'
import { ReservationService } from '../../../api/reservation'
import { getUserInfo } from '../../../api/auth'
import { getMyMerchantOpenStatus, getMyMerchantProfile, updateMyMerchantOpenStatus } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import { shouldBypassConsoleRoleValidation } from '../../../utils/console-access'
import { getConsoleDashboardErrorState } from '../../../utils/console-dashboard'
import dayjs from 'dayjs'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { playNewOrderAlert, destroyAudioAlert } from '../../../utils/audio-alert'

type WsUnsubscribe = () => void

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
      path: '/pages/merchant/printers/index',
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

Page({
  data: {
    navBarHeight: 88,
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
    insightItems: buildInsightItems({}) as DashboardInsightItem[],
    businessStatusText: '营业中，当前经营平稳',
    loading: false,
    businessStatusSubmitting: false,
    accessDenied: false,
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

    const hasAccess = await this.ensureMerchantAccess()
    if (!hasAccess) {
      this.setData({ accessDenied: true, initialLoading: false })
      return
    }

    this.initWebSocket()
  },

  onShow() {
    if (this.data.accessDenied) return
    this.refreshData()
    // 页面重新可见时重新注册 WS 监听（onHide 已清除旧监听，底层连接仍在）
    this.initWebSocket()
  },

  onHide() {
    this.cleanupWebSocket()
  },

  onUnload() {
    this.cleanupWebSocket()
    destroyAudioAlert()
  },

  async ensureMerchantAccess() {
    if (shouldBypassConsoleRoleValidation()) {
      return true
    }

    try {
      const user = await getUserInfo()
      const normalizedRoles = (user.roles || []).map((role) => String(role).toLowerCase())
      const isMerchant = normalizedRoles.some((role) =>
        ['merchant', 'merchant_owner', 'merchant_boss', 'merchant_staff'].includes(role)
      )

      if (!isMerchant) {
        wx.showToast({ title: '当前账号无商户权限', icon: 'none' })
      }

      return isMerchant
    } catch (err) {
      logger.error('Check merchant access failed', err)
      wx.showToast({ title: '无法校验商户权限', icon: 'none' })
      return false
    }
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
        this.refreshData()
      }
    })

    this.data._wsListeners = [sub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub) => unsub())
      this.data._wsListeners = []
    }
  },

  async refreshData() {
    if (this.data.loading) return
    const isFirstLoad = this.data.initialLoading
    this.setData({
      loading: true,
      ...(isFirstLoad
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '', refreshErrorCanRetry: true }
        : { refreshErrorMessage: '', refreshErrorCanRetry: true })
    })

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

        try {
          const currentMerchant = wx.getStorageSync('current_merchant') || {}
          wx.setStorageSync('current_merchant', {
            ...currentMerchant,
            id: merchantProfile.id,
            merchant_id: merchantProfile.id,
            name: merchantProfile.name,
            is_open: merchantOpenStatus?.is_open ?? merchantProfile.is_open
          })
        } catch (storageErr) {
          logger.warn('Sync current merchant cache failed', storageErr)
        }
      } else {
        logger.error('Failed to fetch merchant runtime status', merchantProfileResult.reason)
        addRefreshError(merchantProfileResult.reason, '工作台基础信息加载失败，请重试')
      }

      const recentThirtyDays = dayjs().subtract(29, 'day').format('YYYY-MM-DD')
      const [overview, reservationStats, complaintResult, printAnomaliesResult, hourlyResult, sourceResult, repurchaseResult] = await settleAll([
        MerchantStatsService.getOverview({
          start_date: today,
          end_date: today
        }),
        ReservationService.getReservationStats(),
        listMerchantComplaints({ state: 'PENDING_RESPONSE', page: 1, limit: 100 }),
        MerchantOrderManagementService.listPrintAnomalies({ page_id: 1, page_size: 1, status: 'failed' }),
        MerchantStatsService.getHourlyStats({ start_date: today, end_date: today }),
        MerchantStatsService.getOrderSources({ start_date: today, end_date: today }),
        MerchantStatsService.getRepurchaseRate({ start_date: recentThirtyDays, end_date: today })
      ] as const)

      const currentTodoItems = this.data.todayTodos
      const pendingReservations = reservationStats.status === 'fulfilled'
        ? reservationStats.value.paid_count || 0
        : getTodoCount(currentTodoItems, 'reservations')
      const pendingComplaints = complaintResult.status === 'fulfilled'
        ? complaintResult.value.complaints.length
        : getTodoCount(currentTodoItems, 'complaints')
      const printAnomalies = printAnomaliesResult.status === 'fulfilled'
        ? printAnomaliesResult.value.total || printAnomaliesResult.value.items.length
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
      }

      if (overview.status === 'rejected') {
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

      if (printAnomaliesResult.status === 'rejected') {
        logger.error('Failed to fetch printer anomaly reminders', printAnomaliesResult.reason)
        addRefreshError(printAnomaliesResult.reason, '打印异常同步失败，请稍后重试')
      }

      if (hourlyResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard hourly insights', hourlyResult.reason)
        addRefreshError(hourlyResult.reason, '高峰时段同步失败，请稍后重试')
      }

      if (sourceResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard source insights', sourceResult.reason)
        addRefreshError(sourceResult.reason, '订单结构同步失败，请稍后重试')
      }

      if (repurchaseResult.status === 'rejected') {
        logger.error('Failed to fetch dashboard repurchase insights', repurchaseResult.reason)
        addRefreshError(repurchaseResult.reason, '复购数据同步失败，请稍后重试')
      }

      let pendingPaidOrders = getTodoCount(currentTodoItems, 'paidOrders')

      try {
        const paidOrderResult = await MerchantOrderManagementService.getOrderList({
          page_id: 1,
          page_size: 1,
          status: 'paid'
        })
        pendingPaidOrders = paidOrderResult.total || 0

        const todayTodos = buildTodoItems({
          pendingPaidOrders,
          pendingReservations,
          pendingComplaints,
          printAnomalies
        })
        const todoCount = todayTodos.reduce((sum, item) => sum + item.count, 0)
        const nextInsights = buildInsightItems({
          hourlyStats: hourlyResult.status === 'fulfilled' ? hourlyResult.value : this.data.insightItems[0] ? undefined : [],
          sourceStats: sourceResult.status === 'fulfilled' ? sourceResult.value : this.data.insightItems[1] ? undefined : [],
          repurchase: repurchaseResult.status === 'fulfilled' ? repurchaseResult.value : undefined
        })

        this.setData({
          todayTodos,
          insightItems: nextInsights,
          businessStatusText: buildBusinessStatusText(resolvedIsOpen, todoCount)
        })
      } catch (error) {
        const errorState = getConsoleDashboardErrorState('merchant', error, '待接单数量同步失败，请稍后重试')
        const message = errorState.message
        const todayTodos = buildTodoItems({
          pendingPaidOrders,
          pendingReservations,
          pendingComplaints,
          printAnomalies
        })
        const todoCount = todayTodos.reduce((sum, item) => sum + item.count, 0)
        logger.error('Load dashboard order flow failed', error)
        this.setData({
          todayTodos,
          businessStatusText: buildBusinessStatusText(resolvedIsOpen, todoCount)
        })
        refreshErrors.push(message)
        refreshErrorStates.push({ message, canRetry: errorState.canRetry })
      }

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
          refreshErrorCanRetry: refreshErrorStates.every((state) => state.canRetry)
        })
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
  },

  onPullDownRefresh() {
    this.refreshData()
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
    const { path } = e.currentTarget.dataset as { path?: string }
    if (!path) return
    wx.navigateTo({ url: path })
  },

  onRetry() {
    this.refreshData()
  },

  onRetryRefresh() {
    this.refreshData()
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
      this.refreshData().catch((err) => logger.error('Refresh dashboard after toggle failed', err))
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
