import { getStableBarHeights } from '../../../../utils/responsive'
import { MerchantOrderManagementService, OrderResponse, OrderManagementAdapter } from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import dayjs from 'dayjs'

type OrderStatus = OrderResponse['status']

interface MerchantOrderListItem extends OrderResponse {
  order_no_short: string
  order_type_label: string
  status_label: string
  status_color: string
  time_label: string
  address_summary: string
  submitting: boolean
}

interface OrdersPageOptions {
  status?: OrderStatus
  order_type?: OrderResponse['order_type']
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    orders: [] as MerchantOrderListItem[],
    currentStatus: 'paid' as OrderStatus,
    orderTypeFilter: '' as '' | OrderResponse['order_type'],
    pageTitle: '订单管理',
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
      pageTitle: options.order_type === 'takeout'
        ? '外卖订单'
        : options.order_type === 'dine_in'
          ? '堂食订单'
          : '订单管理'
    })
    this.loadOrders(true)
  },

  async loadOrders(reset = false) {
    if (this.data.loading) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true })
    try {
      const page = reset ? 1 : this.data.page
      const res = await MerchantOrderManagementService.getOrderList({
        page_id: page,
        page_size: this.data.pageSize,
        status: this.data.currentStatus || undefined
      })

      const sourceOrders = this.data.orderTypeFilter
        ? (res || []).filter((order) => order.order_type === this.data.orderTypeFilter)
        : (res || [])

      const formattedOrders = sourceOrders.map((o) => this.formatOrder(o))
      
      this.setData({
        orders: reset ? formattedOrders : [...this.data.orders, ...formattedOrders],
        page: page + 1,
        hasMore: res.length === this.data.pageSize
      })
    } catch (err) {
      logger.error('Merchant load orders failed', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  formatOrder(o: OrderResponse) {
    return {
      ...o,
      order_no_short: o.order_no.slice(-6).toUpperCase(),
      order_type_label: OrderManagementAdapter.formatOrderType(o.order_type),
      status_label: OrderManagementAdapter.formatOrderStatus(o.status),
      status_color: OrderManagementAdapter.getStatusColor(o.status),
      time_label: dayjs(o.created_at).format('HH:mm'),
      address_summary: o.address_id ? '外卖订单' : '', // 简略处理，详情页显示完整
      submitting: false
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatus }>) {
    this.setData({ 
      currentStatus: e.detail.value,
      orders: [],
      page: 1,
      hasMore: true 
    }, () => {
      this.loadOrders(true)
    })
  },

  onPullDownRefresh() {
    this.loadOrders(true)
  },

  onReachBottom() {
    this.loadOrders()
  },

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `../detail/index?id=${id}` })
  },

  // ==================== 快捷操作 ====================

  async onAcceptOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, MerchantOrderManagementService.acceptOrder(id), '接单成功')
  },

  async onMarkReady(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, MerchantOrderManagementService.markOrderReady(id), '制作已完成')
  },

  async onCompleteOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.performAction(id, MerchantOrderManagementService.completeOrder(id), '订单已核销')
  },

  async performAction(id: number, apiPromise: Promise<unknown>, successMsg: string) {
    const index = this.data.orders.findIndex((o) => o.id === id)
    if (index === -1) return

    this.setData({ [`orders[${index}].submitting`]: true })
    try {
      await apiPromise
      wx.showToast({ title: successMsg, icon: 'success' })
      // 这里的策略是接单后状态改变，如果是基于状态过滤的列表，可能需要移除，或者直接刷新
      setTimeout(() => this.loadOrders(true), 500)
    } catch (err) {
      logger.error('Order action failed', err)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      this.setData({ [`orders[${index}].submitting`]: false })
    }
  },

  preventBubble() {}
})
