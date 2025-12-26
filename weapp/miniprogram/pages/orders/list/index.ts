import { getOrders, cancelOrder, OrderStatus } from '../../../api/order'
import { logger } from '../../../utils/logger'
import { OrderCardAdapter } from '../../../adapters/order-card'
import type { OrderCardViewModel } from '../../../adapters/order-card'

// 状态筛选选项
const STATUS_TABS = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' },
  { label: '待接单', value: 'paid' },
  { label: '制作中', value: 'preparing' },
  { label: '配送中', value: 'delivering' },
  { label: '已完成', value: 'completed' }
]

// 取消原因选项
const CANCEL_REASONS = [
  '不想要了',
  '信息填写错误',
  '商品价格较贵',
  '配送时间太长',
  '其他原因'
]

Page({
  data: {
    orders: [] as OrderCardViewModel[],
    navBarHeight: 88,
    loading: false,
    page: 1,
    pageSize: 10,
    hasMore: true,
    statusTabs: STATUS_TABS,
    currentStatus: '' as OrderStatus | ''
  },

  onLoad() {
    this.loadOrders(true)
  },

  onShow() {
    // 返回时刷新列表
    if (this.data.orders.length > 0) {
      this.loadOrders(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.setData({ page: this.data.page + 1 })
      this.loadOrders(false)
    }
  },

  async loadOrders(reset = false) {
    if (this.data.loading) return
    this.setData({ loading: true })

    if (reset) {
      this.setData({ page: 1, orders: [], hasMore: true })
    }

    try {
      const { currentStatus } = this.data
      // API Call with status filter
      const params = currentStatus
        ? { status: currentStatus as OrderStatus, page_id: 1, page_size: 20 }
        : { page_id: 1, page_size: 20 }
      const orderDTOs = await getOrders(params)

      // Adapter conversion with enhanced card view
      const newOrders = orderDTOs.map(OrderCardAdapter.toCardViewModel)

      // Sort by priority (preparing > delivering > completed)
      const sortedOrders = OrderCardAdapter.sortByPriority(newOrders)

      const orders = reset ? sortedOrders : [...this.data.orders, ...sortedOrders]

      this.setData({
        orders,
        loading: false,
        hasMore: false // Assuming API returns all at once for now
      })
    } catch (error) {
      logger.error('Load orders failed:', error, 'List')
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  // 状态筛选切换
  onStatusChange(e: WechatMiniprogram.CustomEvent) {
    const status = e.detail.value || ''
    if (status === this.data.currentStatus) return
    this.setData({ currentStatus: status })
    this.loadOrders(true)
  },

  onViewOrder(e: WechatMiniprogram.BaseEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` })
  },

  // 快速取消订单
  onCancelOrder(e: WechatMiniprogram.BaseEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    wx.showActionSheet({
      itemList: CANCEL_REASONS,
      success: async (res) => {
        const reason = CANCEL_REASONS[res.tapIndex]
        await this.doCancelOrder(Number(id), reason)
      }
    })
  },

  async doCancelOrder(orderId: number, reason: string) {
    wx.showLoading({ title: '取消中...' })
    try {
      await cancelOrder(orderId, { reason })
      wx.hideLoading()
      wx.showToast({ title: '已取消', icon: 'success' })
      setTimeout(() => this.loadOrders(true), 1500)
    } catch (error) {
      wx.hideLoading()
      logger.error('取消订单失败', error, 'List.doCancelOrder')
      wx.showToast({ title: '取消失败', icon: 'error' })
    }
  },

  // 去支付
  onPayOrder(e: WechatMiniprogram.BaseEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` })
  },

  onReorder(e: WechatMiniprogram.BaseEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` })
  }
})
