import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService, OrderManagementAdapter, OrderResponse } from '../../../api/order-management'
import { MerchantStatsService } from '../../../api/merchant-stats'
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
type OrderStatusTab = 'paid' | 'preparing' | 'ready' | 'completed'

interface DashboardOrderItem extends OrderResponse {
  order_no_short: string
  order_type_label: string
  status_label: string
  status_color: string
  time_label: string
}

interface DashboardShortcutItem {
  id: string
  title: string
  desc: string
  path: string
  badge?: number
}

function buildShortcutItems(pendingReservations: number, pendingComplaints: number): DashboardShortcutItem[] {
  return [
    {
      id: 'reservations',
      title: '预订管理',
      desc: '处理当日预订',
      path: '/pages/merchant/reservations/index',
      badge: pendingReservations > 0 ? pendingReservations : undefined
    },
    {
      id: 'complaints',
      title: '投诉处理',
      desc: '及时回复投诉',
      path: '/pages/merchant/complaints/index',
      badge: pendingComplaints > 0 ? pendingComplaints : undefined
    },
    {
      id: 'staff',
      title: '员工管理',
      desc: '分配角色与邀请',
      path: '/pages/merchant/staff/index'
    },
    {
      id: 'config',
      title: '配置中心',
      desc: '统一维护店铺设置',
      path: '/pages/merchant/config/index'
    }
  ]
}

function getShortcutBadge(items: DashboardShortcutItem[], id: string) {
  const matched = items.find((item) => item.id === id)
  return typeof matched?.badge === 'number' ? matched.badge : 0
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
    shortcutItems: buildShortcutItems(0, 0) as DashboardShortcutItem[],
    currentOrderTab: 'paid' as OrderStatusTab,
    orderFlowLoading: false,
    orderFlowError: false,
    orderFlowErrorMessage: '',
    orderFlow: [] as DashboardOrderItem[],
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
        this.setData({
          merchantInfo: {
            name: merchantProfile.name,
            merchant_id: merchantProfile.id
          },
          isOpen: merchantOpenStatus?.is_open ?? merchantProfile.is_open
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

      const [overview, reservationStats, complaintResult] = await settleAll([
        MerchantStatsService.getOverview({
          start_date: today,
          end_date: today
        }),
        ReservationService.getReservationStats(),
        listMerchantComplaints({ state: 'PENDING_RESPONSE', page: 1, limit: 100 })
      ] as const)

      const currentShortcutItems = this.data.shortcutItems
      const pendingReservations = reservationStats.status === 'fulfilled'
        ? reservationStats.value.paid_count || 0
        : getShortcutBadge(currentShortcutItems, 'reservations')
      const pendingComplaints = complaintResult.status === 'fulfilled'
        ? complaintResult.value.complaints.length
        : getShortcutBadge(currentShortcutItems, 'complaints')

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
          },
          shortcutItems: buildShortcutItems(pendingReservations, pendingComplaints)
        })
      } else {
        this.setData({
          shortcutItems: buildShortcutItems(pendingReservations, pendingComplaints)
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

      try {
        const orderFlow = await this.fetchOrderFlow(this.data.currentOrderTab)
        this.setData({
          orderFlow,
          orderFlowError: false,
          orderFlowErrorMessage: ''
        })
      } catch (error) {
        const errorState = getConsoleDashboardErrorState('merchant', error, '订单流加载失败，请稍后重试')
        const message = errorState.message
        logger.error('Load dashboard order flow failed', error)
        this.setData({
          orderFlowError: true,
          orderFlowErrorMessage: message,
          ...(isFirstLoad ? { orderFlow: [] } : {})
        })
        refreshErrors.push(message)
        refreshErrorStates.push({ message, canRetry: errorState.canRetry })
      }

      const orderFlowLoaded = !this.data.orderFlowError

      if (isFirstLoad && (!runtimeLoaded || !summaryLoaded || !orderFlowLoaded)) {
        const initialErrorState = refreshErrorStates[0] || getConsoleDashboardErrorState(
          'merchant',
          refreshErrors[0] || this.data.orderFlowErrorMessage,
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

  async fetchOrderFlow(status: OrderStatusTab): Promise<DashboardOrderItem[]> {
    const result = await MerchantOrderManagementService.getOrderList({
      page_id: 1,
      page_size: 10,
      status
    })
    const orderList = result.orders || []
    return orderList.map((order) => ({
      ...order,
      order_no_short: order.order_no.slice(-6).toUpperCase(),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      time_label: dayjs(order.created_at).format('HH:mm')
    }))
  },

  async loadOrderFlow(status: OrderStatusTab) {
    this.setData({
      orderFlowLoading: true,
      orderFlowError: false,
      orderFlowErrorMessage: '',
      refreshErrorMessage: '',
      refreshErrorCanRetry: true,
      orderFlow: []
    })
    try {
      const orderFlow = await this.fetchOrderFlow(status)
      this.setData({
        orderFlow,
        orderFlowError: false,
        orderFlowErrorMessage: '',
        refreshErrorMessage: '',
        refreshErrorCanRetry: true
      })
    } catch (err) {
      const errorState = getConsoleDashboardErrorState('merchant', err, '订单流加载失败，请稍后重试')
      const message = errorState.message
      logger.error('Load dashboard order flow failed', err)
      this.setData({
        orderFlow: [],
        orderFlowError: true,
        orderFlowErrorMessage: message,
        refreshErrorMessage: errorState.canRetry ? `${message}，当前已保留上次同步结果` : message,
        refreshErrorCanRetry: errorState.canRetry
      })
    } finally {
      this.setData({ orderFlowLoading: false })
    }
  },

  onOrderTabChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatusTab }>) {
    const value = e.detail.value
    this.setData({ currentOrderTab: value })
    this.loadOrderFlow(value)
  },

  onGoOrderList() {
    wx.navigateTo({ url: `/pages/merchant/orders/list/index?status=${this.data.currentOrderTab}` })
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

  onOrderTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/merchant/orders/detail/index?id=${id}` })
  },

  onRetry() {
    this.refreshData()
  },

  onRetryRefresh() {
    this.refreshData()
  },

  onRetryOrderFlow() {
    this.loadOrderFlow(this.data.currentOrderTab)
  },

  async onToggleBusiness() {
    if (this.data.businessStatusSubmitting) return

    const targetOpen = !this.data.isOpen
    this.setData({ businessStatusSubmitting: true })

    try {
      const response = await updateMyMerchantOpenStatus(targetOpen)
      this.setData({ isOpen: response.is_open })
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
