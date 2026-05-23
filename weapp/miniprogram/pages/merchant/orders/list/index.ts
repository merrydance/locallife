import { getStableBarHeights } from '../../../../utils/responsive'
import {
  MerchantOrderManagementService,
  OrderResponse,
  OrderManagementAdapter,
  MERCHANT_REJECT_REASON_OPTIONS,
  MerchantOrderStatusFilter,
  normalizeMerchantVisibleOrderStatusFilter
} from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import dayjs from 'dayjs'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { buildMerchantOrderFeeBreakdownView } from '../../../../utils/merchant-order-detail-view'
import { wsManager, WSMessageType } from '../../../../utils/websocket'

type OrderStatusFilter = MerchantOrderStatusFilter
type OrderTypeFilter = '' | OrderResponse['order_type']
type WsUnsubscribe = () => void

interface OrderTypeOption {
  label: string
  value: OrderTypeFilter
}

interface MerchantOrderListItem extends OrderResponse {
  order_no_short: string
  order_type_label: string
  status_label: string
  status_color: string
  time_label: string
  scene_label: string
  scene_value: string
  status_hint_label: string
  customer_payable_text: string
  merchant_receivable_text: string
  fee_breakdown_available: boolean
  submitting: boolean
  can_accept: boolean
  can_reject: boolean
  can_mark_ready: boolean
  can_complete: boolean
  show_passive_state: boolean
}

interface OrdersPageOptions {
  status?: OrderStatusFilter
  order_type?: OrderTypeFilter
}

const ORDER_TYPE_OPTIONS: OrderTypeOption[] = [
  { label: '全部类型', value: '' },
  { label: '外卖', value: 'takeout' },
  { label: '堂食', value: 'dine_in' },
  { label: '自取', value: 'takeaway' },
  { label: '预订', value: 'reservation' }
]

const getErrorMessage = getErrorUserMessage

interface LoadOrdersOptions {
  showLoading?: boolean
  preserveCurrent?: boolean
}

interface MerchantNewOrderPayload {
  id?: number | string
  message_id?: string
  order_id?: number | string
  event?: string
  type?: string
}

let orderListRequestPending = false
let realtimeRefreshPending = false
let realtimeRefreshTimer: ReturnType<typeof setTimeout> | null = null
const realtimeMessageIds = new Set<string>()

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    orders: [] as MerchantOrderListItem[],
    currentStatus: 'paid' as OrderStatusFilter,
    orderTypeFilter: '' as OrderTypeFilter,
    orderTypeOptions: ORDER_TYPE_OPTIONS,
    pageTitle: '订单中心',
    page: 1,
    pageSize: 10,
    hasMore: true,
    _wsListeners: [] as WsUnsubscribe[]
  },

  async onLoad(options: OrdersPageOptions) {
    orderListRequestPending = false
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      currentStatus: normalizeMerchantVisibleOrderStatusFilter(options.status),
      orderTypeFilter: options.order_type || '',
      pageTitle: '订单中心'
    })
    await this.initializePage()
  },

  async onShow() {
    if (this.data.initialLoading || this.data.loading) {
      return
    }

    const hasAccess = await this.syncAccessState()
    if (!hasAccess) {
      return
    }

    this.initWebSocket()

    await this.loadOrders(true, {
      showLoading: false,
      preserveCurrent: this.data.orders.length > 0
    })
  },

  onHide() {
    this.stopRealtimeRuntime()
  },

  onUnload() {
    this.stopRealtimeRuntime()
  },

  async initializePage() {
    const hasAccess = await this.syncAccessState()
    if (!hasAccess) {
      wx.stopPullDownRefresh()
      return false
    }

    const loaded = await this.loadOrders(true)
    this.initWebSocket()
    return loaded
  },

  initWebSocket() {
    this.cleanupWebSocket()
    wsManager.connect()

    const notificationSub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      const notification = typeof data === 'object' && data !== null
        ? (data as MerchantNewOrderPayload)
        : {}
      if (notification.event === 'new_order' || notification.type === 'order') {
        this.handleRealtimeNewOrder(notification)
      }
    })

    const blockedSub = wsManager.on(WSMessageType.CONNECTION_BLOCKED, (payload) => {
      const message = typeof payload === 'object' && payload !== null && 'message' in payload
        ? String((payload as { message?: unknown }).message || '')
        : ''
      if (!message) {
        return
      }
      this.setData({ refreshErrorMessage: message })
    })

    this.data._wsListeners = [notificationSub, blockedSub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners?.length) {
      this.data._wsListeners.forEach((unsubscribe) => unsubscribe())
      this.data._wsListeners = []
    }
  },

  stopRealtimeRuntime() {
    this.cleanupWebSocket()
    if (realtimeRefreshTimer) {
      clearTimeout(realtimeRefreshTimer)
      realtimeRefreshTimer = null
    }
    realtimeRefreshPending = false
    wsManager.disconnect()
  },

  handleRealtimeNewOrder(data: unknown) {
    const payload = typeof data === 'object' && data !== null
      ? (data as MerchantNewOrderPayload)
      : {}
    const messageId = typeof payload.message_id === 'string'
      ? payload.message_id
      : ''
    if (messageId) {
      if (realtimeMessageIds.has(messageId)) {
        return
      }
      realtimeMessageIds.add(messageId)
      if (realtimeMessageIds.size > 100) {
        const oldest = realtimeMessageIds.values().next().value
        if (oldest) realtimeMessageIds.delete(oldest)
      }
    }

    this.refreshOrdersFromRealtime()
  },

  refreshOrdersFromRealtime() {
    if (realtimeRefreshPending) {
      return
    }
    realtimeRefreshPending = true

    const refresh = async () => {
      try {
        await this.loadOrders(true, {
          showLoading: false,
          preserveCurrent: this.data.orders.length > 0
        })
      } finally {
        realtimeRefreshPending = false
      }
    }

    if (orderListRequestPending) {
      realtimeRefreshTimer = setTimeout(() => {
        realtimeRefreshTimer = null
        void refresh()
      }, 500)
      return
    }

    void refresh()
  },

  async syncAccessState() {
    const accessResult = await ensureMerchantConsoleAccess()
    const accessGranted = isMerchantConsoleAccessGranted(accessResult)

    this.setData({
      accessReady: true,
      accessDenied: isMerchantConsoleAccessDenied(accessResult),
      accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
      ...(accessGranted
        ? {}
        : {
            loading: false,
            orders: [],
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: '',
            page: 1,
            hasMore: true
          })
    })

    return accessGranted
  },

  async loadOrders(reset = false, options?: LoadOrdersOptions) {
    if (orderListRequestPending) return false
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return false
    if (!reset && !this.data.hasMore) return false

    const hasExistingOrders = this.data.orders.length > 0
    const preserveCurrent = !!options?.preserveCurrent && reset && hasExistingOrders
    const showLoading = options?.showLoading !== false
    const usePageLoading = reset && showLoading && !hasExistingOrders
    const useListLoading = !reset && showLoading
    const shouldToggleLoading = usePageLoading || useListLoading

    orderListRequestPending = true

    this.setData({
      ...(shouldToggleLoading ? { loading: true } : {}),
      ...(usePageLoading
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : showLoading
          ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
          : preserveCurrent
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const page = reset ? 1 : this.data.page
      const res = await MerchantOrderManagementService.getOrderList({
        page_id: page,
        page_size: this.data.pageSize,
        status: this.data.currentStatus || undefined,
        order_type: this.data.orderTypeFilter || undefined
      })

      const formattedOrders = (res.orders || []).map((order) => this.formatOrder(order))

      this.setData({
        orders: reset ? formattedOrders : [...this.data.orders, ...formattedOrders],
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        page: page + 1,
        hasMore: page * this.data.pageSize < (res.total || 0)
      })
      return true
    } catch (err) {
      logger.error('Merchant load orders failed', err)
      const message = getErrorMessage(err, '订单加载失败，请稍后重试')
      if (this.data.initialLoading || (!hasExistingOrders && reset)) {
        this.setData({
          orders: [],
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (preserveCurrent || hasExistingOrders) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
      return false
    } finally {
      orderListRequestPending = false
      if (shouldToggleLoading && this.data.loading) {
        this.setData({ loading: false })
      }
      wx.stopPullDownRefresh()
    }
  },

  formatOrder(order: OrderResponse): MerchantOrderListItem {
    const scene = this.buildSceneSummary(order)
    const feeBreakdownView = buildMerchantOrderFeeBreakdownView(order)
    return {
      ...order,
      order_no_short: order.order_no.slice(-6).toUpperCase(),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      time_label: dayjs(order.created_at).format('HH:mm'),
      scene_label: scene.label,
      scene_value: scene.value,
      status_hint_label: order.status_hint || OrderManagementAdapter.getMerchantOrderStatusHint(order),
      customer_payable_text: feeBreakdownView.customer_payable_text,
      merchant_receivable_text: feeBreakdownView.merchant_receivable_text,
      fee_breakdown_available: feeBreakdownView.available,
      submitting: false,
      can_accept: OrderManagementAdapter.canAcceptOrder(order),
      can_reject: OrderManagementAdapter.canRejectOrder(order),
      can_mark_ready: OrderManagementAdapter.canMarkReady(order),
      can_complete: OrderManagementAdapter.canCompleteOrder(order),
      show_passive_state: OrderManagementAdapter.shouldShowPassiveState(order)
    }
  },

  buildSceneSummary(order: OrderResponse) {
    if (order.order_type === 'takeout') {
      return {
        label: '配送地址',
        value: order.delivery_address || order.delivery_contact_name || '外卖配送'
      }
    }

    if (order.order_type === 'dine_in') {
      return {
        label: '就餐位置',
        value: order.table_id ? `${order.table_id} 号桌` : '堂食就餐'
      }
    }

    if (order.order_type === 'takeaway') {
      return {
        label: '取餐方式',
        value: order.pickup_code_masked ? `取餐码 ${order.pickup_code_masked}` : '到店自取'
      }
    }

    return {
      label: '预订单',
      value: order.reservation_id ? `预订 #${order.reservation_id}` : '预订点菜'
    }
  },

  async applyFilters(nextStatus: OrderStatusFilter, nextOrderType: OrderTypeFilter) {
    if (this.data.loading) {
      return
    }

    const previousPage = this.data.page
    const previousHasMore = this.data.hasMore
    const previousStatus = this.data.currentStatus
    const previousOrderType = this.data.orderTypeFilter
    const preserveCurrent = this.data.orders.length > 0

    this.setData({
      currentStatus: nextStatus,
      orderTypeFilter: nextOrderType,
      page: 1,
      hasMore: true,
      refreshErrorMessage: ''
    })

    const success = await this.loadOrders(true, {
      showLoading: false,
      preserveCurrent
    })

    if (!success) {
      this.setData({
        currentStatus: previousStatus,
        orderTypeFilter: previousOrderType,
        page: previousPage,
        hasMore: previousHasMore
      })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatusFilter }>) {
    const nextStatus = e.detail.value
    if (nextStatus === this.data.currentStatus) {
      return
    }

    void this.applyFilters(nextStatus, this.data.orderTypeFilter)
  },

  onOrderTypeChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: OrderTypeFilter }
    const nextOrderType = value || ''
    if (nextOrderType === this.data.orderTypeFilter) {
      return
    }

    void this.applyFilters(this.data.currentStatus, nextOrderType)
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.loadOrders(true, {
      showLoading: false,
      preserveCurrent: this.data.orders.length > 0
    })
  },

  onReachBottom() {
    void this.loadOrders()
  },

  onLoadMore() {
    void this.loadOrders()
  },

  onRetry() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.loadOrders(true)
  },

  onRetryRefresh() {
    void this.loadOrders(true, {
      showLoading: false,
      preserveCurrent: this.data.orders.length > 0
    })
  },

  async onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      loading: false,
      orders: [],
      page: 1,
      hasMore: true
    })

    await this.initializePage()
  },

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `../detail/index?id=${id}` })
  },

  async onAcceptOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, () => MerchantOrderManagementService.acceptOrder(id), '接单成功')
  },

  async onRejectOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    try {
      const result = await wx.showActionSheet({
        itemList: [...MERCHANT_REJECT_REASON_OPTIONS],
        alertText: '请选择拒单原因，系统将按后端契约发起退款'
      })
      const reason = MERCHANT_REJECT_REASON_OPTIONS[result.tapIndex]
      if (!reason) return

      await this.performAction(
        id,
        () => MerchantOrderManagementService.rejectOrder(id, { reason }),
        '已拒单并发起退款'
      )
    } catch (error) {
      const err = error as { errMsg?: string }
      if (err?.errMsg?.includes('cancel')) return
      logger.error('Select reject reason failed', error)
      wx.showToast({ title: '选择拒单原因失败', icon: 'none' })
    }
  },

  async onMarkReady(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, () => MerchantOrderManagementService.markOrderReady(id), '制作已完成')
  },

  async onCompleteOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, () => MerchantOrderManagementService.completeOrder(id), '订单已核销')
  },

  async performAction(id: number, request: () => Promise<unknown>, _successMsg: string) {
    const index = this.data.orders.findIndex((order) => order.id === id)
    if (index === -1) return

    this.setData({ [`orders[${index}].submitting`]: true })
    try {
      await request()
      await this.loadOrders(true, {
        showLoading: false,
        preserveCurrent: this.data.orders.length > 0
      })
    } catch (err) {
      logger.error('Order action failed', err)
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      const nextIndex = this.data.orders.findIndex((order) => order.id === id)
      if (nextIndex !== -1) {
        this.setData({ [`orders[${nextIndex}].submitting`]: false })
      }
    }
  },

  syncOrderAfterAction(updatedOrder: OrderResponse) {
    const currentOrders = [...this.data.orders]
    const index = currentOrders.findIndex((order) => order.id === updatedOrder.id)
    if (index === -1) return

    const formatted = this.formatOrder(updatedOrder)
    const matchesCurrentFilter = !this.data.currentStatus || updatedOrder.status === this.data.currentStatus

    if (matchesCurrentFilter) {
      currentOrders[index] = formatted
    } else {
      currentOrders.splice(index, 1)
    }

    this.setData({
      orders: currentOrders,
      refreshErrorMessage: ''
    })
  },

  preventBubble() {}
})
