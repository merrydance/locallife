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

interface MerchantOrderDetailOptions {
  id?: string
}

interface MerchantOrderDetailView extends OrderResponse {
  status_label: string
  status_color: string
  status_icon: string
  status_desc: string
  order_type_label: string
  payment_method_label: string
  created_at_fmt: string
  paid_at_fmt: string
  completed_at_fmt: string
  status_hint_label: string
  step_current: number
  timeline_steps: Array<{ title: string, content: string }>
  location_label: string
  location_primary: string
  location_secondary: string
  contact_name: string
  contact_phone: string
  can_accept: boolean
  can_reject: boolean
  can_mark_ready: boolean
  can_complete: boolean
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    orderId: 0,
    order: null as MerchantOrderDetailView | null,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: true,
    submitting: false,
    isIPhoneX: false
  },

  onLoad(options: MerchantOrderDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const { model } = wx.getSystemInfoSync()
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')

    this.setData({
      navBarHeight,
      isIPhoneX,
      orderId: parseInt(options.id || '0')
    })

    if (!this.data.orderId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少订单编号，无法查看详情'
      })
      return
    }

    this.loadDetail()
  },

  onPullDownRefresh() {
    this.loadDetail(false)
  },

  onRetry() {
    this.loadDetail(true)
  },

  onRetryRefresh() {
    this.loadDetail(false)
  },

  async loadDetail(showLoading = true) {
    const canPreserveDetail = !showLoading && Boolean(this.data.order)
    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : canPreserveDetail
          ? { refreshErrorMessage: '' }
          : {})
    })
    try {
      const res = await MerchantOrderManagementService.getOrderDetail(this.data.orderId)
      this.setData({
        order: this.formatDetail(res),
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Merchant load order detail failed', err)
      const message = getErrorMessage(err, '订单详情加载失败，请稍后重试')
      if (canPreserveDetail) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          initialError: true,
          initialErrorMessage: message
        })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  formatDetail(order: OrderResponse): MerchantOrderDetailView {
    const timeline = this.buildTimeline(order)
    const scene = this.buildSceneInfo(order)

    return {
      ...order,
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      status_icon: this.getStatusIcon(order.status),
      status_desc: this.getStatusDesc(order),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      payment_method_label: OrderManagementAdapter.formatPaymentMethod(order.payment_method || 'wechat'),
      created_at_fmt: dayjs(order.created_at).format('YYYY-MM-DD HH:mm'),
      paid_at_fmt: this.formatTime(order.paid_at),
      completed_at_fmt: this.formatTime(order.completed_at),
      status_hint_label: order.status_hint || this.getFallbackStatusHint(order),
      step_current: timeline.current,
      timeline_steps: timeline.steps,
      location_label: scene.label,
      location_primary: scene.primary,
      location_secondary: scene.secondary,
      contact_name: order.delivery_contact_name || '',
      contact_phone: order.delivery_contact_phone || '',
      can_accept: OrderManagementAdapter.canAcceptOrder(order),
      can_reject: OrderManagementAdapter.canRejectOrder(order),
      can_mark_ready: OrderManagementAdapter.canMarkReady(order),
      can_complete: OrderManagementAdapter.canCompleteOrder(order)
    }
  },

  buildSceneInfo(order: OrderResponse) {
    if (order.order_type === 'takeout') {
      return {
        label: '配送地址',
        primary: order.delivery_address || '待同步配送地址',
        secondary: [order.delivery_contact_name, order.delivery_contact_phone].filter(Boolean).join(' ')
      }
    }

    if (order.order_type === 'dine_in') {
      return {
        label: '就餐位置',
        primary: order.table_id ? `${order.table_id} 号桌` : '堂食就餐',
        secondary: order.reservation_id ? `预订 #${order.reservation_id}` : '到店就餐'
      }
    }

    if (order.order_type === 'takeaway') {
      return {
        label: '取餐方式',
        primary: order.pickup_code_masked ? `取餐码 ${order.pickup_code_masked}` : '到店自取',
        secondary: order.pickup_code ? `原始取餐码 ${order.pickup_code}` : '顾客到店后核销'
      }
    }

    return {
      label: '预订信息',
      primary: order.reservation_id ? `预订 #${order.reservation_id}` : '预订点菜',
      secondary: order.table_id ? `${order.table_id} 号桌` : '到店后履约'
    }
  },

  buildTimeline(order: OrderResponse) {
    if (order.order_type === 'takeout') {
      const steps = [
        { title: '订单提交', content: this.formatTime(order.created_at, 'YYYY-MM-DD HH:mm') },
        { title: '已支付', content: this.formatTime(order.paid_at) },
        { title: '商户处理', content: this.formatTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
        { title: '出餐完成', content: this.formatTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
        { title: '骑手接单', content: this.formatTimelineValue(order.courier_accept_at, order.status === 'ready' ? '待分配骑手' : '--') },
        { title: '骑手取餐', content: this.formatTimelineValue(order.picked_at, order.status === 'courier_accepted' ? '骑手前往取餐' : '--') },
        { title: '送达完成', content: this.formatTimelineValue(order.user_delivered_at || order.auto_user_delivered_at || order.rider_delivered_at || order.completed_at, order.status === 'delivering' ? '配送途中' : '--') }
      ]

      const currentMap: Record<string, number> = {
        pending: 0,
        paid: 1,
        preparing: 2,
        ready: 3,
        courier_accepted: 4,
        picked: 5,
        delivering: 5,
        rider_delivered: 6,
        user_delivered: 6,
        completed: 6,
        cancelled: 0
      }

      return { steps, current: currentMap[order.status] ?? 0 }
    }

    const steps = [
      { title: '订单提交', content: this.formatTime(order.created_at, 'YYYY-MM-DD HH:mm') },
      { title: '已支付', content: this.formatTime(order.paid_at) },
      { title: '商户处理', content: this.formatTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
      { title: order.order_type === 'dine_in' ? '备餐完成' : '待取餐', content: this.formatTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
      { title: '履约完成', content: this.formatTimelineValue(order.completed_at, order.status === 'ready' ? '待商户确认' : '--') }
    ]

    const currentMap: Record<string, number> = {
      pending: 0,
      paid: 1,
      preparing: 2,
      ready: 3,
      completed: 4,
      cancelled: 0
    }

    return { steps, current: currentMap[order.status] ?? 0 }
  },

  formatTime(value?: string, pattern = 'HH:mm') {
    return value ? dayjs(value).format(pattern) : '--'
  },

  formatTimelineValue(value?: string, fallback = '--') {
    return value ? this.formatTime(value) : fallback
  },

  getStatusIcon(status: string) {
    const icons: Record<string, string> = {
      paid: 'notification',
      preparing: 'loading',
      ready: 'check-circle',
      courier_accepted: 'assignment-user',
      picked: 'shop',
      delivering: 'undertake-deliver',
      rider_delivered: 'check-circle-filled',
      user_delivered: 'check-circle-filled',
      completed: 'check-circle',
      cancelled: 'close-circle'
    }
    return icons[status] || 'info-circle'
  },

  getStatusDesc(order: OrderResponse) {
    const descs: Record<string, string> = {
      paid: '顾客已付款，请尽快接单或拒单处理',
      preparing: '订单已进入制作阶段',
      ready: order.order_type === 'takeout' ? '餐品已备妥，等待骑手接力' : '餐品已备妥，等待顾客取餐或到店核销',
      courier_accepted: '骑手已接单，正在前往门店',
      picked: '骑手已取餐，订单即将配送',
      delivering: '骑手配送中，请留意异常与超时',
      rider_delivered: '骑手已送达，等待顾客确认',
      user_delivered: '顾客已确认收货，订单即将完结',
      completed: '订单已成功履约完成',
      cancelled: order.cancel_reason || '订单已被系统或商户取消'
    }
    return descs[order.status] || ''
  },

  getFallbackStatusHint(order: OrderResponse) {
    if (order.cancel_reason) {
      return `取消原因：${order.cancel_reason}`
    }
    if (order.overtime) {
      return '当前订单已超时，请优先关注'
    }
    return ''
  },

  async onAccept() {
    await this.performAction(() => MerchantOrderManagementService.acceptOrder(this.data.orderId), '接单成功')
  },

  async onReject() {
    try {
      const result = await wx.showActionSheet({
        itemList: [...MERCHANT_REJECT_REASON_OPTIONS],
        alertText: '请选择拒单原因，系统将按后端契约发起退款'
      })
      const reason = MERCHANT_REJECT_REASON_OPTIONS[result.tapIndex]
      if (!reason) return

      await this.performAction(
        () => MerchantOrderManagementService.rejectOrder(this.data.orderId, { reason }),
        '已拒单并发起退款'
      )
    } catch (error) {
      const err = error as { errMsg?: string }
      if (err?.errMsg?.includes('cancel')) return
      logger.error('Select reject reason failed', error)
      wx.showToast({ title: '选择拒单原因失败', icon: 'none' })
    }
  },

  async onMarkReady() {
    await this.performAction(() => MerchantOrderManagementService.markOrderReady(this.data.orderId), '制作完成')
  },

  async onComplete() {
    await this.performAction(() => MerchantOrderManagementService.completeOrder(this.data.orderId), '订单已核销')
  },

  async performAction(request: () => Promise<unknown>, _successText: string) {
    this.setData({ submitting: true })
    try {
      const updatedOrder = await request() as OrderResponse
      this.setData({
        order: this.formatDetail(updatedOrder),
        refreshErrorMessage: ''
      })
      await this.loadDetail(false)

      const pages = getCurrentPages()
      const listPage = pages[pages.length - 2] as { loadOrders?: (reset?: boolean, showLoading?: boolean) => void } | undefined
      if (listPage?.loadOrders) {
        listPage.loadOrders(true, false)
      }
    } catch (err) {
      logger.error('Action failed', err)
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onCallCustomer() {
    if (this.data.order?.contact_phone) {
      wx.makePhoneCall({ phoneNumber: this.data.order.contact_phone })
    }
  }
})
