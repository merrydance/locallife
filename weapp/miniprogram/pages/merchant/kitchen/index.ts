import dayjs from 'dayjs'
import { KitchenDisplayService, KitchenOrderResponse, KitchenOrdersResponse, OrderManagementAdapter } from '../../../api/order-management'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

type KitchenActionType = '' | 'preparing' | 'ready'

interface KitchenBoardOrder extends KitchenOrderResponse {
  order_no_short: string
  order_type_label: string
  waiting_label: string
  remaining_label: string
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

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loading: false,
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
    readyOrders: [] as KitchenBoardOrder[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadKitchenOrders()
  },

  onPullDownRefresh() {
    this.loadKitchenOrders()
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadKitchenOrders(false)
    }
  },

  async loadKitchenOrders(showLoading = true) {
    if (this.data.loading) return

    this.setData({
      loading: true,
      ...(showLoading ? { initialError: false, initialErrorMessage: '' } : {})
    })

    try {
      const response = await KitchenDisplayService.getKitchenOrders()
      this.setData({
        stats: buildKitchenStats(response),
        newOrders: (response.new_orders || []).map(formatKitchenOrder),
        preparingOrders: (response.preparing_orders || []).map(formatKitchenOrder),
        readyOrders: (response.ready_orders || []).map(formatKitchenOrder),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load kitchen orders failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '后厨数据加载失败，请重试'
        : '后厨数据加载失败，请重试'

      if (this.data.initialLoading || (!this.data.newOrders.length && !this.data.preparingOrders.length && !this.data.readyOrders.length)) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadKitchenOrders()
  },

  async onStartPreparing(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.runKitchenAction(id, 'preparing', KitchenDisplayService.startPreparing(id), '已开始制作')
  },

  async onMarkReady(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.runKitchenAction(id, 'ready', KitchenDisplayService.markKitchenOrderReady(id), '已标记出餐')
  },

  async runKitchenAction(orderId: number, actionType: KitchenActionType, requestPromise: Promise<KitchenOrderResponse>, successMessage: string) {
    if (this.data.actionOrderId === orderId && this.data.actionType === actionType) return

    this.setData({ actionOrderId: orderId, actionType })
    try {
      await requestPromise
      wx.showToast({ title: successMessage, icon: 'success' })
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