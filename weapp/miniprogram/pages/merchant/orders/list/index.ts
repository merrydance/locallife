import { getStableBarHeights } from '../../../../utils/responsive'
import {
  MerchantOrderManagementService,
  OrderResponse,
  OrderManagementAdapter,
  MERCHANT_REJECT_REASON_OPTIONS
} from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import dayjs from 'dayjs'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type OrderStatus = OrderResponse['status']
type OrderStatusFilter = '' | OrderStatus
type OrderTypeFilter = '' | OrderResponse['order_type']

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
  submitting: boolean
  can_accept: boolean
  can_reject: boolean
  can_mark_ready: boolean
  can_complete: boolean
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

Page({
  data: {
    navBarHeight: 88,
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
    hasMore: true
  },

  onLoad(options: OrdersPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      currentStatus: options.status || 'paid',
      orderTypeFilter: options.order_type || '',
      pageTitle: '订单中心'
    })
    this.loadOrders(true)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading) {
      this.loadOrders(true, false)
    }
  },

  async loadOrders(reset = false, showLoading = true) {
    if (this.data.loading) return
    if (!reset && !this.data.hasMore) return

    const hasExistingOrders = this.data.orders.length > 0
    const isSilentRefresh = reset && !showLoading && hasExistingOrders

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
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
    } catch (err) {
      logger.error('Merchant load orders failed', err)
      const message = getErrorMessage(err, '订单加载失败，请稍后重试')
      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  formatOrder(order: OrderResponse): MerchantOrderListItem {
    const scene = this.buildSceneSummary(order)
    return {
      ...order,
      order_no_short: order.order_no.slice(-6).toUpperCase(),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      time_label: dayjs(order.created_at).format('HH:mm'),
      scene_label: scene.label,
      scene_value: scene.value,
      status_hint_label: order.status_hint || this.getStatusHint(order),
      submitting: false,
      can_accept: OrderManagementAdapter.canAcceptOrder(order),
      can_reject: OrderManagementAdapter.canRejectOrder(order),
      can_mark_ready: OrderManagementAdapter.canMarkReady(order),
      can_complete: OrderManagementAdapter.canCompleteOrder(order)
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

  getStatusHint(order: OrderResponse) {
    switch (order.status) {
      case 'paid':
        return '顾客已支付，建议尽快接单或拒单处理'
      case 'preparing':
        return '商户正在制作中，可在出餐后标记完成'
      case 'ready':
        return order.order_type === 'takeout' ? '等待骑手取餐或系统分配送力' : '等待顾客取餐或到店核销'
      case 'courier_accepted':
        return '骑手已接单，正在到店取餐'
      case 'picked':
        return '骑手已取餐，订单即将配送'
      case 'delivering':
        return '配送途中，请关注异常和超时情况'
      case 'rider_delivered':
        return '骑手已送达，等待顾客确认'
      case 'user_delivered':
        return '顾客已确认收货，系统即将完成订单'
      case 'completed':
        return '订单已完成履约'
      case 'cancelled':
        return order.cancel_reason || '订单已取消'
      default:
        return ''
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatusFilter }>) {
    this.setData({
      currentStatus: e.detail.value,
      orders: [],
      page: 1,
      hasMore: true,
      refreshErrorMessage: ''
    }, () => {
      this.loadOrders(true)
    })
  },

  onOrderTypeChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: OrderTypeFilter }
    this.setData({
      orderTypeFilter: value || '',
      orders: [],
      page: 1,
      hasMore: true,
      refreshErrorMessage: ''
    }, () => {
      this.loadOrders(true)
    })
  },

  onPullDownRefresh() {
    this.loadOrders(true, false)
  },

  onReachBottom() {
    this.loadOrders()
  },

  onLoadMore() {
    this.loadOrders()
  },

  onRetry() {
    this.loadOrders(true)
  },

  onRetryRefresh() {
    this.loadOrders(true, false)
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
      const updatedOrder = await request() as OrderResponse
      this.syncOrderAfterAction(updatedOrder)
    } catch (err) {
      logger.error('Order action failed', err)
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ [`orders[${index}].submitting`]: false })
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
