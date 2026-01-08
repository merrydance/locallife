import { logger } from '../../../utils/logger'
import CartService from '../../../services/cart'
import { getOrderDetail, confirmOrder, cancelOrder, urgeOrder, OrderResponse } from '../../../api/order'
import { OrderAdapter } from '../../../adapters/order'
import { OrderDetail } from '../../../models/order'
import { generateOrderTimeline } from '../../../utils/timeline'

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
    navBarHeight: 88,
    loading: false,
    showTrackingButton: false,
    showConfirmButton: false,
    showCancelButton: false,
    showUrgeButton: false,
    lastUrgeTime: 0,  // 上次催单时间
    urgeCountdown: 0  // 催单倒计时（秒）
  },

  onLoad(options: { id?: string }) {
    if (options.id) {
      this.setData({ orderId: options.id })
      this.loadOrderDetail()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadOrderDetail() {
    this.setData({ loading: true })

    try {
      const orderDTO = await getOrderDetail(parseInt(this.data.orderId))
      const order = OrderAdapter.toDetailViewModel(orderDTO)

      // 判断是否显示物流追踪按钮（外卖订单且状态为配送中）
      const showTrackingButton = orderDTO.order_type === 'takeout' &&
        orderDTO.status === 'delivering'

      // 判断是否显示确认收货按钮（配送中或待取餐）
      const showConfirmButton = (orderDTO.order_type === 'takeout' && orderDTO.status === 'delivering') ||
        (orderDTO.order_type === 'takeaway' && orderDTO.status === 'ready')

      // 判断是否显示取消按钮（待支付、已支付、制作中可取消）
      const showCancelButton = ['pending', 'paid', 'preparing'].includes(orderDTO.status)

      // 判断是否显示催单按钮（已支付、制作中、配送中可催单）
      const showUrgeButton = ['paid', 'preparing', 'delivering'].includes(orderDTO.status)

      // 生成订单时间线
      const timeline = generateOrderTimeline(orderDTO)

      this.setData({
        order: { ...order, timeline },
        orderDTO,
        loading: false,
        showTrackingButton,
        showConfirmButton,
        showCancelButton,
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

  // 检查催单冷却时间
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

  // 开始催单倒计时
  startUrgeCountdown() {
    const timer = setInterval(() => {
      const { urgeCountdown } = this.data
      if (urgeCountdown <= 1) {
        clearInterval(timer)
        this.setData({ urgeCountdown: 0 })
      } else {
        this.setData({ urgeCountdown: urgeCountdown - 1 })
      }
    }, 1000)
  },

  onCallMerchant() {
    wx.showToast({ title: '暂无商家电话', icon: 'none' })
  },

  onCancelOrder() {
    // 显示取消原因选择
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
      setTimeout(() => this.loadOrderDetail(), 1500)
    } catch (error) {
      wx.hideLoading()
      logger.error('取消订单失败', error, 'Detail.doCancelOrder')
      wx.showToast({ title: '取消失败', icon: 'error' })
    }
  },

  // 催单功能
  async onUrgeOrder() {
    const { urgeCountdown } = this.data
    if (urgeCountdown > 0) {
      wx.showToast({ title: `${urgeCountdown}秒后可再次催单`, icon: 'none' })
      return
    }

    wx.showLoading({ title: '催单中...' })
    try {
      await urgeOrder(parseInt(this.data.orderId), { message: '请尽快处理' })
      wx.hideLoading()
      wx.showToast({ title: '催单成功', icon: 'success' })

      // 设置5分钟冷却
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

  onReview() {
    const { orderDTO } = this.data
    if (orderDTO) {
      wx.navigateTo({
        url: `/pages/user_center/reviews/create/index?orderId=${orderDTO.id}&merchantId=${orderDTO.merchant_id}`
      })
    }
  },

  async onReorder() {
    const { order } = this.data
    if (!order) return

    CartService.clear()

    // 使用更新后的字段名，转换ID为string（CartItem使用string类型）
    const addPromises = order.items.map((item) =>
      CartService.addItem({
        merchantId: String(order.merchantId),
        dishId: String(item.dishId || 0),
        dishName: item.name,
        shopName: order.merchantName,
        imageUrl: item.imageUrl,
        price: item.unitPrice,
        priceDisplay: item.unitPriceDisplay,
        quantity: item.quantity
      })
    )

    const results = await Promise.all(addPromises)
    if (results.some((success) => !success)) {
      return
    }

    wx.showToast({ title: '已加入购物车', icon: 'success' })
    setTimeout(() => {
      wx.navigateTo({ url: '/pages/takeout/order-confirm/index' })
    }, 500)
  },

  onViewTracking() {
    wx.navigateTo({
      url: `/pages/orders/tracking/index?orderId=${this.data.orderId}`
    })
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
            setTimeout(() => this.loadOrderDetail(), 1500)
          } catch (error) {
            logger.error('确认收货失败', error, 'Detail.onConfirmReceipt')
            wx.showToast({ title: '确认失败', icon: 'error' })
          }
        }
      }
    })
  },

  onContactRider() {
    wx.showToast({ title: '联系骑手功能开发中', icon: 'none' })
  },

  onViewPayment() {
    wx.navigateTo({
      url: `/pages/user_center/payment-detail/index?orderId=${this.data.orderId}`
    })
  }
})
