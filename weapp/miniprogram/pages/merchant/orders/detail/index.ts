import { getStableBarHeights } from '../../../../utils/responsive'
import { MerchantOrderManagementService, OrderResponse, OrderManagementAdapter } from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import dayjs from 'dayjs'

Page({
  data: {
    navBarHeight: 88,
    orderId: 0,
    order: null as any,
    loading: true,
    submitting: false,
    isIPhoneX: false
  },

  onLoad(options: any) {
    const { navBarHeight } = getStableBarHeights()
    const { model } = wx.getSystemInfoSync()
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')
    
    this.setData({ 
      navBarHeight, 
      isIPhoneX,
      orderId: parseInt(options.id)
    })
    this.loadDetail()
  },

  async loadDetail() {
    this.setData({ loading: true })
    try {
      const res = await MerchantOrderManagementService.getOrderDetail(this.data.orderId)
      this.setData({ order: this.formatDetail(res) })
    } catch (err) {
      logger.error('Merchant load order detail failed', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  formatDetail(o: OrderResponse) {
    const statusLabel = OrderManagementAdapter.formatOrderStatus(o.status)
    const statusColor = OrderManagementAdapter.getStatusColor(o.status)
    
    // 步骤条位置
    const stepMap: Record<string, number> = {
      'pending': 0,
      'paid': 1,
      'preparing': 2,
      'ready': 3,
      'delivering': 4,
      'completed': 4,
      'cancelled': -1
    }

    return {
      ...o,
      status_label: statusLabel,
      status_color: statusColor,
      status_icon: this.getStatusIcon(o.status),
      status_desc: this.getStatusDesc(o.status),
      order_type_label: OrderManagementAdapter.formatOrderType(o.order_type),
      payment_method_label: OrderManagementAdapter.formatPaymentMethod(o.payment_method),
      created_at_fmt: dayjs(o.created_at).format('YYYY-MM-DD HH:mm'),
      paid_at_fmt: o.paid_at ? dayjs(o.paid_at).format('HH:mm') : '--',
      accept_at_fmt: o.status === 'preparing' || o.status === 'ready' || o.status === 'completed' ? '已接单' : '',
      ready_at_fmt: o.status === 'ready' || o.status === 'completed' ? '制作完成' : '',
      completed_at_fmt: o.completed_at ? dayjs(o.completed_at).format('HH:mm') : '',
      step_current: stepMap[o.status] || 0
    }
  },

  getStatusIcon(status: string) {
    const icons: Record<string, string> = {
      'paid': 'notification',
      'preparing': 'loading',
      'ready': 'check-circle',
      'completed': 'check-circle',
      'cancelled': 'close-circle'
    }
    return icons[status] || 'info-circle'
  },

  getStatusDesc(status: string) {
    const descs: Record<string, string> = {
      'paid': '顾客已付款，请尽快接单制作',
      'preparing': '正在烹饪制作中...',
      'ready': '制作完成，等待后续操作',
      'completed': '订单已成功履行',
      'cancelled': '订单已被系统或手动取消'
    }
    return descs[status] || ''
  },

  // ==================== 操作 ====================

  async onAccept() {
    await this.performAction(MerchantOrderManagementService.acceptOrder(this.data.orderId))
  },

  async onMarkReady() {
    await this.performAction(MerchantOrderManagementService.markOrderReady(this.data.orderId))
  },

  async onComplete() {
    await this.performAction(MerchantOrderManagementService.completeOrder(this.data.orderId))
  },

  async performAction(apiPromise: Promise<any>) {
    this.setData({ submitting: true })
    try {
      await apiPromise
      wx.showToast({ title: '操作成功', icon: 'success' })
      setTimeout(() => this.loadDetail(), 500)
      
      // 通知列表页刷新
      const pages = getCurrentPages()
      const listPage = pages[pages.length - 2] as any
      if (listPage && listPage.loadOrders) {
        listPage.loadOrders(true)
      }
    } catch (err) {
      logger.error('Action failed', err)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onCallCustomer() {
    if (this.data.order?.customer_phone) {
      wx.makePhoneCall({ phoneNumber: this.data.order.customer_phone })
    }
  }
})
