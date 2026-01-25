import { logger } from '../../../utils/logger'
import CartService from '../../../services/cart'
import { getOrderDetail, confirmOrder, cancelOrder, urgeOrder, OrderResponse, OrderType } from '../../../api/order'
import { processPayment } from '../../../api/payment'
import { OrderAdapter } from '../../../adapters/order'
import { OrderDetail } from '../../../models/order'
import { generateOrderTimeline } from '../../../utils/timeline'
import { ReservationService, ReservationResponse } from '../../../api/reservation'

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
    orderId: '',
    order: null as OrderDetail | null,
    orderDTO: null as OrderResponse | null,
    reservationInfo: null as ReservationResponse | null,
    navBarHeight: 88,
    loading: false,
    
    // UI Flags
    showTrackingButton: false,
    showConfirmButton: false,
    showCancelButton: false,
    showPayButton: false,
    showUrgeButton: false,
    showContactButton: true, // 总是显示联系客服/商家
    
    lastUrgeTime: 0,  // 上次催单时间
    urgeCountdown: 0  // 催单倒计时（秒）
  },

  onLoad(options: { id?: string }) {
    if (options.id) {
      this.setData({ orderId: options.id })
      this.loadOrderDetail()
    }
  },

  onShow() {
     // 返回页面时刷新
     if (this.data.orderId && this.data.order) {
        this.loadOrderDetail()
     }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadOrderDetail() {
    // 首次加载显示loading，后续静默刷新
    if (!this.data.order) {
       this.setData({ loading: true })
    }

    try {
      const orderDTO = await getOrderDetail(parseInt(this.data.orderId))
      const order = OrderAdapter.toDetailViewModel(orderDTO)

      let reservationInfo: ReservationResponse | null = null
      if (orderDTO.order_type === 'reservation' && orderDTO.reservation_id) {
        try {
          reservationInfo = await ReservationService.getReservationDetail(orderDTO.reservation_id)
        } catch (e) {
          logger.warn('Fetch reservation detail failed', e)
        }
      }

      const actions = orderDTO.actions || []

      // 按钮显示逻辑
      const showTrackingButton = orderDTO.order_type === 'takeout' &&
        ['delivering', 'rider_delivered', 'picked'].includes(orderDTO.status)

      const showConfirmButton = actions.includes('confirm')
      const showPayButton = actions.includes('pay')
      const showCancelButton = actions.includes('cancel')
      const showUrgeButton = actions.includes('urge')

      // 生成订单时间线 (如果需要展示详细Timeline)
      const timeline = generateOrderTimeline(orderDTO)

      this.setData({
        order: { ...order, timeline },
        orderDTO,
        reservationInfo,
        loading: false,
        showTrackingButton,
        showConfirmButton,
        showCancelButton,
        showPayButton,
        showUrgeButton
      })

      // 检查催单冷却时间
      this.checkUrgeCooldown()
    } catch (error) {
      logger.error('Load order detail failed:', error, 'Detail')
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  checkUrgeCooldown() {
    const { lastUrgeTime } = this.data
    if (!lastUrgeTime) return

    const elapsed = Date.now() - lastUrgeTime
    const cooldownMs = 5 * 60 * 1000 // 5分钟冷却
    if (elapsed < cooldownMs) {
      const remaining = Math.ceil((cooldownMs - elapsed) / 1000)
      this.setData({ urgeCountdown: remaining })
      this.startUrgeCountdown()
    }
  },

  startUrgeCountdown() {
    // Clear existing timer if any to avoid duplicates
    if ((this as any)._urgeTimer) clearInterval((this as any)._urgeTimer);

    (this as any)._urgeTimer = setInterval(() => {
      const { urgeCountdown } = this.data
      if (urgeCountdown <= 1) {
        clearInterval((this as any)._urgeTimer)
        this.setData({ urgeCountdown: 0 })
      } else {
        this.setData({ urgeCountdown: urgeCountdown - 1 })
      }
    }, 1000)
  },

  onUnload() {
     if ((this as any)._urgeTimer) clearInterval((this as any)._urgeTimer);
  },

  // 复制订单号
  onCopy(e: WechatMiniprogram.BaseEvent) {
    const text = e.currentTarget.dataset.text
    if (!text) return
    wx.setClipboardData({
      data: text,
      success: () => wx.showToast({ title: '已复制', icon: 'none' })
    })
  },

  // 进入商家详情
  onEnterMerchant(e: WechatMiniprogram.BaseEvent) {
     const merchantId = e.currentTarget.dataset.id
     if (!merchantId) return
     // 假设外卖商家详情页
     wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${merchantId}` })
  },

  // 联系商家
  onCallMerchant() {
    const phone = this.data.order?.merchantPhone
    if (phone) {
      wx.makePhoneCall({ phoneNumber: phone })
    } else {
      wx.showToast({ title: '暂无商家电话', icon: 'none' })
    }
  },

  // 联系客服
  onContact() {
     wx.showModal({
        title: '联系客服',
        content: '即将拨打平台客服电话 400-888-8888',
        success: (res) => {
           if (res.confirm) {
              wx.makePhoneCall({ phoneNumber: '4008888888' })
           }
        }
     })
  },

  onCancelOrder() {
    wx.showActionSheet({
      itemList: CANCEL_REASONS,
      success: async (res) => {
        const reason = CANCEL_REASONS[res.tapIndex]
        await this.doCancelOrder(reason)
      }
    })
  },

  async doCancelOrder(reason: string) {
    wx.showLoading({ title: '取消中...' })
    try {
      await cancelOrder(parseInt(this.data.orderId), { reason })
      wx.hideLoading()
      wx.showToast({ title: '已取消', icon: 'success' })
      this.loadOrderDetail()
    } catch (error) {
      wx.hideLoading()
      logger.error('取消订单失败', error, 'Detail.doCancelOrder')
      wx.showToast({ title: '取消失败', icon: 'error' })
    }
  },

  async onUrgeOrder() {
    const { urgeCountdown } = this.data
    if (urgeCountdown > 0) return

    wx.showLoading({ title: '催单中...' })
    try {
      await urgeOrder(parseInt(this.data.orderId), { message: '请尽快处理' })
      wx.hideLoading()
      wx.showToast({ title: '催单成功', icon: 'success' })

      this.setData({
        lastUrgeTime: Date.now(),
        urgeCountdown: 300
      })
      this.startUrgeCountdown()
    } catch (error) {
      wx.hideLoading()
      logger.error('催单失败', error, 'Detail.onUrgeOrder')
      wx.showToast({ title: '催单失败', icon: 'error' })
    }
  },

  async onReorder() {
    const { order } = this.data
    if (!order) return

    wx.showLoading({ title: '再次购买中...' })
    try {
      // 构造购物车上下文
      const orderType: OrderType = order.type || "takeout"
      const cartContext: any = { orderType }
      
      if (orderType === 'dine_in' && order.tableId) {
          cartContext.tableId = order.tableId
      }
      if (orderType === 'reservation' && order.reservationId) {
          cartContext.reservationId = order.reservationId
      }

      await CartService.loadCart(order.merchantId, cartContext)

      const addResults = await Promise.all(
        order.items.map((item) =>
          CartService.addItem({
            merchantId: order.merchantId,
            dishId: item.dishId,
            comboId: item.comboId,
            quantity: item.quantity
          })
        )
      )

      if (addResults.some((ok) => !ok)) {
        wx.showToast({ title: '部分商品可能已下架', icon: 'none' })
      }

      wx.showToast({ title: '已加入购物车', icon: 'success' })
      setTimeout(() => {
        wx.navigateTo({ url: '/pages/takeout/cart/index' })
      }, 300)
    } catch (error) {
      logger.error('再次购买失败', error, 'Detail.onReorder')
      wx.showToast({ title: '操作失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  },

  onViewTracking() {
    wx.navigateTo({
      url: `/pages/orders/tracking/index?orderId=${this.data.orderId}`
    })
  },

  async onPayOrder() {
    const { orderId } = this.data
    if (!orderId) return

    wx.showLoading({ title: '拉起支付...' })
    try {
      await processPayment(parseInt(orderId), 'order')
      wx.showToast({ title: '支付成功', icon: 'success' })
      this.loadOrderDetail()
    } catch (error) {
      logger.error('支付失败', error, 'Detail.onPayOrder')
      wx.showToast({ title: '支付未完成', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onConfirmReceipt() {
    wx.showModal({
      title: '确认收货',
      content: '确认已收到订单？',
      success: async (res) => {
        if (res.confirm) {
          try {
            await confirmOrder(parseInt(this.data.orderId))
            wx.showToast({ title: '确认成功', icon: 'success' })
            this.loadOrderDetail()
          } catch (error) {
            logger.error('确认收货失败', error, 'Detail.onConfirmReceipt')
            wx.showToast({ title: '确认失败', icon: 'error' })
          }
        }
      }
    })
  },
  
  onReview() {
     // TODO: 评价功能
     wx.showToast({ title: '评价功能开发中', icon: 'none' })
  }
})
