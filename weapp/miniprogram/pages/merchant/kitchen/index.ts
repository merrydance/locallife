import dayjs from 'dayjs'
import { getMyMerchantOpenStatus } from '../../../api/merchant'
import { KitchenDisplayService, KitchenOrderResponse, KitchenOrdersResponse, OrderManagementAdapter } from '../../../api/order-management'
import { isPreparingOrderStatus, isReadyOrderStatus } from '../../../api/order'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { resolveStatusTagTheme, type StatusTagTheme } from '../../../utils/status-tag'
import { wsManager, WSMessageType } from '../../../utils/websocket'

type WsUnsubscribe = () => void

type KitchenActionType = '' | 'preparing' | 'ready'
type KitchenBoardFilter = 'all' | 'new' | 'preparing' | 'ready'

interface KitchenBoardFilterOption {
  label: string
  value: KitchenBoardFilter
}

interface MerchantStatusChangePayload {
  merchant_id?: number
  is_open?: boolean
  auto_close_at?: string
  source?: string
}

interface KitchenBoardOrder extends KitchenOrderResponse {
  order_no_short: string
  order_type_label: string
  waiting_label: string
  remaining_label: string
  remaining_tag_text: string
  remaining_tag_theme: StatusTagTheme
  preparation_label: string
  seat_or_pickup_label: string
  created_time_label: string
  estimated_ready_label: string
  is_overdue: boolean
}

interface KitchenBoardStats {
  newCount: number
  preparingCount: number
  readyCount: number
  totalPending: number
  avgPreparationTime: number
  behindScheduleCount: number
}

const BOARD_FILTER_OPTIONS: KitchenBoardFilterOption[] = [
  { label: '全部待处理', value: 'all' },
  { label: '新订单', value: 'new' },
  { label: '制作中', value: 'preparing' },
  { label: '待取餐', value: 'ready' }
]

function formatKitchenOrder(order: KitchenOrderResponse): KitchenBoardOrder {
  const remainingMinutes = Math.round(OrderManagementAdapter.getRemainingTime(order))
  const preparationMinutes = OrderManagementAdapter.calculatePreparationTime(order)
  const seatOrPickupLabel = order.table_number || order.table_no
    ? `${order.table_number || order.table_no}号桌`
    : order.pickup_number
      ? `取餐号 ${order.pickup_number}`
      : order.customer_name || '现场订单'

  return {
    ...order,
    order_no_short: order.order_no.slice(-6).toUpperCase(),
    order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
    waiting_label: `${order.waiting_minutes || 0}分钟`,
    remaining_label: remainingMinutes > 0 ? `剩余${remainingMinutes}分钟` : '请尽快处理',
    remaining_tag_text: OrderManagementAdapter.isOrderOverdue(order)
      ? '已超时'
      : (remainingMinutes > 0 ? `剩余${remainingMinutes}分钟` : '请尽快处理'),
    remaining_tag_theme: OrderManagementAdapter.isOrderOverdue(order)
      ? resolveStatusTagTheme('danger')
      : resolveStatusTagTheme('success'),
    preparation_label: preparationMinutes === null ? '--' : `${preparationMinutes}分钟`,
    seat_or_pickup_label: seatOrPickupLabel,
    created_time_label: dayjs(order.created_at).format('HH:mm'),
    estimated_ready_label: order.estimated_ready_at ? dayjs(order.estimated_ready_at).format('HH:mm') : '--',
    is_overdue: OrderManagementAdapter.isOrderOverdue(order)
  }
}

function buildKitchenStats(response: KitchenOrdersResponse): KitchenBoardStats {
  const stats = response.stats
  return {
    newCount: stats?.new_count ?? response.new_orders.length,
    preparingCount: stats?.preparing_count ?? response.preparing_orders.length,
    readyCount: stats?.ready_count ?? response.ready_orders.length,
    totalPending: stats?.total_pending ?? (response.new_orders.length + response.preparing_orders.length + response.ready_orders.length),
    avgPreparationTime: stats?.avg_prepare_time ?? stats?.avg_preparation_time ?? 0,
    behindScheduleCount: stats?.orders_behind_schedule ?? response.preparing_orders.filter((order) => OrderManagementAdapter.isOrderOverdue(order)).length
  }
}

function buildKitchenStatsFromLists(
  newOrders: KitchenBoardOrder[],
  preparingOrders: KitchenBoardOrder[],
  readyOrders: KitchenBoardOrder[]
): KitchenBoardStats {
  const overduePreparing = preparingOrders.filter((order) => order.is_overdue).length
  const preparationTimes = readyOrders
    .map((order) => Number.parseInt(order.preparation_label, 10))
    .filter((value) => Number.isFinite(value) && value >= 0)

  return {
    newCount: newOrders.length,
    preparingCount: preparingOrders.length,
    readyCount: readyOrders.length,
    totalPending: newOrders.length + preparingOrders.length + readyOrders.length,
    avgPreparationTime: preparationTimes.length
      ? Math.round(preparationTimes.reduce((sum, value) => sum + value, 0) / preparationTimes.length)
      : 0,
    behindScheduleCount: overduePreparing
  }
}

function buildBoardPresentation(
  filter: KitchenBoardFilter,
  newOrders: KitchenBoardOrder[],
  preparingOrders: KitchenBoardOrder[],
  readyOrders: KitchenBoardOrder[]
) {
  const visibleNewOrders = filter === 'all' || filter === 'new' ? newOrders : []
  const visiblePreparingOrders = filter === 'all' || filter === 'preparing' ? preparingOrders : []
  const visibleReadyOrders = filter === 'all' || filter === 'ready' ? readyOrders : []
  const visibleCount = visibleNewOrders.length + visiblePreparingOrders.length + visibleReadyOrders.length
  const labelMap: Record<KitchenBoardFilter, string> = {
    all: '待处理订单',
    new: '新订单',
    preparing: '制作中订单',
    ready: '待取餐订单'
  }

  return {
    visibleNewOrders,
    visiblePreparingOrders,
    visibleReadyOrders,
    boardResultSummaryText: filter === 'all'
      ? `当前共 ${visibleCount} 单待处理订单`
      : `${labelMap[filter]}共 ${visibleCount} 单`,
    boardEmptyDescription: filter === 'all'
      ? '当前后厨没有待处理订单'
      : `当前没有${labelMap[filter]}`
  }
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    boardFilterOptions: BOARD_FILTER_OPTIONS,
    boardFilter: 'all' as KitchenBoardFilter,
    boardInitialLoading: true,
    boardInitialError: false,
    boardInitialErrorMessage: '',
    boardRefreshErrorMessage: '',
    boardLoading: false,
    isMerchantOpen: true,
    actionOrderId: 0,
    actionType: '' as KitchenActionType,
    stats: {
      newCount: 0,
      preparingCount: 0,
      readyCount: 0,
      totalPending: 0,
      avgPreparationTime: 0,
      behindScheduleCount: 0
    } as KitchenBoardStats,
    newOrders: [] as KitchenBoardOrder[],
    preparingOrders: [] as KitchenBoardOrder[],
    readyOrders: [] as KitchenBoardOrder[],
    visibleNewOrders: [] as KitchenBoardOrder[],
    visiblePreparingOrders: [] as KitchenBoardOrder[],
    visibleReadyOrders: [] as KitchenBoardOrder[],
    boardResultSummaryText: '当前共 0 单待处理订单',
    boardEmptyDescription: '当前后厨没有待处理订单',
    _wsListeners: [] as WsUnsubscribe[]
  },

  async onLoad() {
    await this.bootstrap()
  },

  async bootstrap() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: ''
    })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })

    if (accessResult.status !== 'granted') {
      this.setData({ boardInitialLoading: false })
      return
    }

    this.refreshRealtimeRuntime()
    await this.loadKitchenOrders(true)
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    this.loadKitchenOrders(false).catch(() => undefined)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    this.refreshRealtimeRuntime()
    if (!this.data.boardInitialLoading) {
      this.loadKitchenOrders(false)
    }
  },

  onHide() {
    this.stopRealtimeRuntime({ disconnect: true })
  },

  onUnload() {
    this.stopRealtimeRuntime({ disconnect: true })
  },

  initWebSocket() {
    this.cleanupWebSocket()
    wsManager.connect()

    const statusChangeSub = wsManager.on(WSMessageType.MERCHANT_STATUS_CHANGE, (data) => {
      const payload = typeof data === 'object' && data !== null
        ? (data as MerchantStatusChangePayload)
        : {}

      if (payload.merchant_id && payload.merchant_id > 0) {
        this.applyMerchantOpenStatus(Boolean(payload.is_open))
      }
    })

    const sub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      const notification = typeof data === 'object' && data !== null
        ? (data as { type?: string })
        : {}

      if (this.data.isMerchantOpen && notification.type === 'order') {
        this.loadKitchenOrders(false)
      }
    })

    const blockedSub = wsManager.on(WSMessageType.CONNECTION_BLOCKED, (payload) => {
      const message = typeof payload === 'object' && payload !== null && 'message' in payload
        ? String((payload as { message?: unknown }).message || '')
        : ''
      if (!message) {
        return
      }

      this.setData({ boardRefreshErrorMessage: message })
      wx.showToast({ title: message, icon: 'none' })
    })

    this.data._wsListeners = [statusChangeSub, sub, blockedSub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners?.length) {
      this.data._wsListeners.forEach((unsubscribe) => unsubscribe())
      this.data._wsListeners = []
    }
  },

  stopRealtimeRuntime(options: { disconnect?: boolean } = {}) {
    this.cleanupWebSocket()
    if (options.disconnect) {
      wsManager.disconnect()
    }
  },

  syncRealtimeRuntime(isOpen: boolean) {
    this.applyMerchantOpenStatus(isOpen)
    this.initWebSocket()
  },

  applyMerchantOpenStatus(isOpen: boolean) {
    this.setData({
      isMerchantOpen: isOpen,
      boardRefreshErrorMessage: isOpen ? '' : '当前门店已打烊，后厨实时订单已暂停'
    })
  },

  async refreshRealtimeRuntime() {
    try {
      const status = await getMyMerchantOpenStatus()
      this.syncRealtimeRuntime(Boolean(status?.is_open))
    } catch (err) {
      logger.warn('Load merchant open status for kitchen realtime failed', err)
      this.stopRealtimeRuntime({ disconnect: true })
    }
  },

  async loadKitchenOrders(showLoading = true) {
    if (this.data.boardLoading) return

    const hasExistingOrders = Boolean(this.data.newOrders.length || this.data.preparingOrders.length || this.data.readyOrders.length)
    const isSilentRefresh = !showLoading && hasExistingOrders

    this.setData({
      boardLoading: true,
      ...(showLoading
        ? { boardInitialError: false, boardInitialErrorMessage: '', boardRefreshErrorMessage: '' }
        : isSilentRefresh
          ? { boardRefreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await KitchenDisplayService.getKitchenOrders()
      this.setData({
        stats: buildKitchenStats(response),
        newOrders: (response.new_orders || []).map(formatKitchenOrder),
        preparingOrders: (response.preparing_orders || []).map(formatKitchenOrder),
        readyOrders: (response.ready_orders || []).map(formatKitchenOrder),
        ...buildBoardPresentation(
          this.data.boardFilter,
          (response.new_orders || []).map(formatKitchenOrder),
          (response.preparing_orders || []).map(formatKitchenOrder),
          (response.ready_orders || []).map(formatKitchenOrder)
        ),
        boardInitialLoading: false,
        boardInitialError: false,
        boardInitialErrorMessage: '',
        boardRefreshErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load kitchen orders failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '后厨数据加载失败，请重试'
        : '后厨数据加载失败，请重试'

      if (this.data.boardInitialLoading || (!this.data.newOrders.length && !this.data.preparingOrders.length && !this.data.readyOrders.length)) {
        this.setData({
          boardInitialLoading: false,
          boardInitialError: true,
          boardInitialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ boardRefreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ boardLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetryBoard() {
    this.loadKitchenOrders()
  },

  onRetryAccess() {
    this.bootstrap()
  },

  onBoardFilterChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset as { value?: KitchenBoardFilter }
    if (!value) {
      return
    }

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean' ? detail : !!detail?.checked
    if (!nextChecked || value === this.data.boardFilter) {
      return
    }

    this.setData({
      boardFilter: value,
      ...buildBoardPresentation(value, this.data.newOrders, this.data.preparingOrders, this.data.readyOrders)
    })
  },

  applyKitchenLists(newOrders: KitchenBoardOrder[], preparingOrders: KitchenBoardOrder[], readyOrders: KitchenBoardOrder[]) {
    this.setData({
      newOrders,
      preparingOrders,
      readyOrders,
      stats: buildKitchenStatsFromLists(newOrders, preparingOrders, readyOrders),
      ...buildBoardPresentation(this.data.boardFilter, newOrders, preparingOrders, readyOrders)
    })
  },

  syncKitchenOrder(order: KitchenOrderResponse) {
    const formattedOrder = formatKitchenOrder(order)
    const newOrders = this.data.newOrders.filter((item) => item.id !== order.id)
    const preparingOrders = this.data.preparingOrders.filter((item) => item.id !== order.id)
    const readyOrders = this.data.readyOrders.filter((item) => item.id !== order.id)

    if (isPreparingOrderStatus(order.status)) {
      preparingOrders.unshift(formattedOrder)
    } else if (isReadyOrderStatus(order.status)) {
      readyOrders.unshift(formattedOrder)
    } else {
      newOrders.unshift(formattedOrder)
    }

    this.applyKitchenLists(newOrders, preparingOrders, readyOrders)
  },

  async onStartPreparing(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.runKitchenAction(id, 'preparing', KitchenDisplayService.startPreparing(id), '已开始制作')
  },

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/merchant/kitchen/detail/index?id=${id}` })
  },

  async onMarkReady(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.runKitchenAction(id, 'ready', KitchenDisplayService.markKitchenOrderReady(id), '已标记出餐')
  },

  async runKitchenAction(orderId: number, actionType: KitchenActionType, requestPromise: Promise<KitchenOrderResponse>, _successMessage: string) {
    if (this.data.actionOrderId === orderId && this.data.actionType === actionType) return

    this.setData({ actionOrderId: orderId, actionType })
    try {
      const updatedOrder = await requestPromise
      this.syncKitchenOrder(updatedOrder)
      this.setData({ boardRefreshErrorMessage: '' })
      await this.loadKitchenOrders(false)
    } catch (err: unknown) {
      logger.error('Kitchen action failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '操作失败，请稍后重试'
        : '操作失败，请稍后重试'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ actionOrderId: 0, actionType: '' })
    }
  }
})
