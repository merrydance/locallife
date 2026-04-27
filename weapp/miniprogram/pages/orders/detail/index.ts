import Navigation from '../../../utils/navigation'
import { logger } from '../../../utils/logger'
import CartService from '../../../services/cart'
import {
  confirmOrder,
  cancelOrder,
  urgeOrder,
  isCancelledOrderStatus,
  isDeliveringOrderStatus,
  OrderResponse,
  OrderType,
  isCompletedOrderStatus,
  isPendingOrderStatus,
  isReadyOrderStatus,
  isTrackableOrderStatus
} from '../../../api/order'
import {
  createOrderPayment,
  getCombinedPaymentFollowupMessage,
  getPaymentProcessOutcomeMessage,
  isCombinedPaymentSuccessful,
  isPaymentProcessSuccessful,
  PaymentCancelledError,
  processCreatedPayment,
  recoverCombinedPaymentOrder,
  shouldRecreateCombinedPayment,
  invokeWechatPay,
  processPayment
} from '../../../api/payment'
import { OrderAdapter } from '../../../adapters/order'
import { OrderDetail } from '../../../models/order'
import { generateOrderTimeline } from '../../../utils/timeline'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { loadOrderDetailBundle, getOrderReview, type OrderDetailReservation } from '../../../services/order-detail'

// 取消原因选项
const CANCEL_REASONS = [
  '不想要了',
  '信息填写错误',
  '商品价格较贵',
  '配送时间太长',
  '其他原因'
]

type PageWithUrgeTimer = {
  _urgeTimer?: ReturnType<typeof setInterval>
}

Page({
  data: {
    orderId: '',
    order: null as OrderDetail | null,
    orderDTO: null as OrderResponse | null,
    reservationInfo: null as OrderDetailReservation | null,
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    refreshErrorMessage: '',
    
    // UI Flags
    showTrackingButton: false,
    showConfirmButton: false,
    showCancelButton: false,
    showPayButton: false,
    showUrgeButton: false,
    showContactButton: true, // 总是显示联系客服/商家
    showReviewButton: false,
    showPickupConfirmButton: false,
    showReorderButton: false,
    isReviewed: false,
    paying: false,
    statusHeaderDesc: '',
    statusHeaderIcon: 'time',
    
    lastUrgeTime: 0,  // 上次催单时间
    urgeCountdown: 0  // 催单倒计时（秒）
  },

  onLoad(options: { id?: string }) {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

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
    this.setData({ isError: false, refreshErrorMessage: '' })
    if (!this.data.order) {
       this.setData({ loading: true })
    }

    try {
      const { orderDTO, reservationInfo } = await loadOrderDetailBundle(parseInt(this.data.orderId))
      const order = OrderAdapter.toDetailViewModel(orderDTO)

      const actions = orderDTO.actions || []

      // 按钮显示逻辑
      const showTrackingButton = orderDTO.order_type === 'takeout' && isTrackableOrderStatus(orderDTO.status)

      const showConfirmButton = actions.includes('confirm')
      const showPayButton = actions.includes('pay')
      const showCancelButton = actions.includes('cancel')
      const showUrgeButton = actions.includes('urge')
      const showPickupConfirmButton = orderDTO.order_type === 'takeaway' && isReadyOrderStatus(orderDTO.status)
      const showReorderButton = isCompletedOrderStatus(orderDTO.status)

      let statusHeaderDesc = `订单编号: ${order.orderNo}`
      if (order.expectDeliverTime) {
        statusHeaderDesc = `预计 ${order.expectDeliverTime} 送达`
      } else if (isCompletedOrderStatus(orderDTO.status)) {
        statusHeaderDesc = `感谢您对${order.merchantName}的信任`
      } else if (isCancelledOrderStatus(orderDTO.status)) {
        statusHeaderDesc = orderDTO.cancel_reason || '订单已取消'
      }

      let statusHeaderIcon = 'time'
      if (isCompletedOrderStatus(orderDTO.status)) {
        statusHeaderIcon = 'check-circle'
      } else if (isCancelledOrderStatus(orderDTO.status)) {
        statusHeaderIcon = 'close-circle'
      } else if (isDeliveringOrderStatus(orderDTO.status)) {
        statusHeaderIcon = 'cart'
      } else if (isPendingOrderStatus(orderDTO.status)) {
        statusHeaderIcon = 'timer'
      }

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
        showUrgeButton,
        showReviewButton: isCompletedOrderStatus(orderDTO.status),
        showPickupConfirmButton,
        showReorderButton,
        statusHeaderDesc,
        statusHeaderIcon
      })

      // 检查评价状态
      if (isCompletedOrderStatus(orderDTO.status)) {
        this.checkReviewStatus()
      }

      // 检查催单冷却时间
      this.checkUrgeCooldown()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载订单详情失败，请稍后重试')
      logger.error('Load order detail failed:', error, 'Detail')
      if (!this.data.order) {
        this.setData({ 
          loading: false, 
          isError: true, 
          errorMsg: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({
          loading: false,
          refreshErrorMessage: `${getErrorUserMessage(error, '刷新失败，请稍后重试')}，当前已保留上次结果`
        })
      }
    }
  },

  onRetryRefresh() {
    this.loadOrderDetail()
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
    const page = this as unknown as PageWithUrgeTimer
    if (page._urgeTimer) clearInterval(page._urgeTimer)

    page._urgeTimer = setInterval(() => {
      const { urgeCountdown } = this.data
      if (urgeCountdown <= 1) {
        if (page._urgeTimer) clearInterval(page._urgeTimer)
        this.setData({ urgeCountdown: 0 })
      } else {
        this.setData({ urgeCountdown: urgeCountdown - 1 })
      }
    }, 1000)
  },

  onUnload() {
     const page = this as unknown as PageWithUrgeTimer
     if (page._urgeTimer) clearInterval(page._urgeTimer)
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
      Navigation.toRestaurantDetail(merchantId)
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
    const orderId = this.data.orderId
    wx.navigateTo({
      url: `/pages/user_center/service_center/index${orderId ? '?orderId=' + orderId : ''}`
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
    const { order, orderDTO } = this.data
    if (!order || !orderDTO) return

    // 针对不同类型的再来一单路由
    const orderType = orderDTO.order_type || 'takeout'

    if (orderType === 'dine_in') {
        const tableId = orderDTO.table_id || order.tableId
        if (!tableId) {
          wx.showToast({ title: '请重新扫码进入堂食点餐', icon: 'none' })
          return
        }
        wx.navigateTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableId}` })
        return
    }

    if (orderType === 'reservation') {
      Navigation.toReservationCreate({
        merchantId: order.merchantId,
        merchantName: order.merchantName || ''
      })
        return
    }

    // takeout 和 takeaway 继续走购物车逻辑
    wx.showLoading({ title: '再次购买中...' })
    try {
      // 构造购物车上下文
      const cartContext: {
        orderType: OrderType
      } = { orderType }

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

      Navigation.toCart()
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
    const { orderId, paying, orderDTO } = this.data
    if (!orderId || paying) return

    this.setData({ paying: true })
    try {
      const combinedPaymentID = orderDTO?.payment_context?.combined_payment_id
      if (combinedPaymentID) {
        let combinedPayment = await recoverCombinedPaymentOrder(combinedPaymentID)

        if (combinedPayment.pay_params) {
          try {
            await invokeWechatPay(combinedPayment.pay_params)
            combinedPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
            if (isCombinedPaymentSuccessful(combinedPayment)) {
              Navigation.toPaymentSuccess({
                orderId,
                orderNo: combinedPayment.combine_out_trade_no || this.data.order?.orderNo || orderId,
                amount: (combinedPayment.total_amount / 100).toFixed(2),
                isCombined: true,
                orderCount: combinedPayment.sub_orders.length
              })
              return
            }

            if (!shouldRecreateCombinedPayment(combinedPayment)) {
              wx.showToast({ title: `${getCombinedPaymentFollowupMessage(combinedPayment)}，订单详情稍后会自动同步。`, icon: 'none' })
              return
            }
          } catch (error) {
            if (error instanceof PaymentCancelledError) {
              wx.showToast({ title: '已取消支付，可继续完成原合单支付', icon: 'none' })
              return
            }

            const wxError = error as { errMsg?: string }
            if (wxError?.errMsg?.includes('cancel')) {
              wx.showToast({ title: '已取消支付，可继续完成原合单支付', icon: 'none' })
              return
            }

            combinedPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
            if (isCombinedPaymentSuccessful(combinedPayment)) {
              Navigation.toPaymentSuccess({
                orderId,
                orderNo: combinedPayment.combine_out_trade_no || this.data.order?.orderNo || orderId,
                amount: (combinedPayment.total_amount / 100).toFixed(2),
                isCombined: true,
                orderCount: combinedPayment.sub_orders.length
              })
              return
            }

            if (!shouldRecreateCombinedPayment(combinedPayment)) {
              wx.showToast({ title: `${getCombinedPaymentFollowupMessage(combinedPayment)}，订单详情稍后会自动同步。`, icon: 'none' })
              return
            }
          }
        } else if (isCombinedPaymentSuccessful(combinedPayment)) {
          Navigation.toPaymentSuccess({
            orderId,
            orderNo: combinedPayment.combine_out_trade_no || this.data.order?.orderNo || orderId,
            amount: (combinedPayment.total_amount / 100).toFixed(2),
            isCombined: true,
            orderCount: combinedPayment.sub_orders.length
          })
          return
        } else {
          if (!shouldRecreateCombinedPayment(combinedPayment)) {
            wx.showToast({ title: `${getCombinedPaymentFollowupMessage(combinedPayment)}，订单详情稍后会自动同步。`, icon: 'none' })
            return
          }
        }

        const fallbackResult = await processCreatedPayment(await createOrderPayment(parseInt(orderId, 10)))
        if (!isPaymentProcessSuccessful(fallbackResult)) {
          await this.loadOrderDetail()
          wx.showToast({
            title: getPaymentProcessOutcomeMessage(fallbackResult, {
              failed: '支付未完成，请稍后重试',
              unknown: '支付结果确认中，请稍后刷新订单详情'
            }),
            icon: 'none'
          })
          return
        }

        const fallbackPayment = fallbackResult.payment
        if (!fallbackPayment) {
          await this.loadOrderDetail()
          wx.showToast({ title: '支付结果确认中，请稍后刷新订单详情', icon: 'none' })
          return
        }

        Navigation.toPaymentSuccess({
          orderId,
          orderNo: fallbackPayment.out_trade_no || this.data.order?.orderNo || orderId,
          amount: (fallbackPayment.amount / 100).toFixed(2)
        })
        return
      }

      const paymentResult = await processPayment(parseInt(orderId, 10), 'order')

      if (!isPaymentProcessSuccessful(paymentResult)) {
        await this.loadOrderDetail()
        wx.showToast({
          title: getPaymentProcessOutcomeMessage(paymentResult, {
            failed: '支付未完成，请重新发起',
            unknown: '支付结果确认中，请稍后刷新'
          }),
          icon: 'none'
        })
        return
      }
      
      const { order } = this.data
      if (order) {
        Navigation.toPaymentSuccess({
          orderId,
          orderNo: order.orderNo,
          amount: (order.payableAmount / 100).toFixed(2)
        })
      } else {
        await this.loadOrderDetail()
      }
    } catch (error) {
      if (error instanceof PaymentCancelledError) {
        wx.showToast({ title: '已取消支付', icon: 'none' })
      } else {
        logger.error('支付失败', error, 'Detail.onPayOrder')
        await this.loadOrderDetail()
        wx.showToast({ title: '支付结果确认中，请稍后刷新', icon: 'none' })
      }
    } finally {
      this.setData({ paying: false })
    }
  },

  async onConfirmReceipt() {
    const { orderDTO } = this.data
    if (!orderDTO) return

    const transactionId = orderDTO.wechat_transaction_id
    if (!transactionId) {
      // 无微信支付交易号（如余额支付），直接走本地确认
      wx.showModal({
        title: '确认收货',
        content: '确认已收到订单？',
        success: async (res) => {
          if (res.confirm) {
            wx.showLoading({ title: '处理中...' })
            try {
              await confirmOrder(parseInt(this.data.orderId))
              wx.hideLoading()
              await this.loadOrderDetail()
            } catch (error) {
              wx.hideLoading()
              logger.error('确认收货失败', error, 'Detail.onConfirmReceipt')
              wx.showToast({ title: '确认失败', icon: 'error' })
            }
          }
        }
      })
      return
    }

    // 有微信支付交易号：通过微信官方确认收货组件
    const app = getApp<IAppOption>()
    app.globalData.pendingConfirmOrderId = parseInt(this.data.orderId)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    if ((wx as any).openBusinessView) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (wx as any).openBusinessView({
        businessType: 'weappOrderConfirm',
        extraData: { transaction_id: transactionId },
        fail() {
          logger.error('打开确认收货组件失败', undefined, 'Detail.onConfirmReceipt')
          wx.showToast({ title: '打开失败，请重试', icon: 'none' })
        }
      })
    } else {
      wx.showToast({ title: '请升级微信后重试', icon: 'none' })
    }
  },
  
  async checkReviewStatus() {
    try {
      const review = await getOrderReview(parseInt(this.data.orderId))
      if (review && review.id) {
        this.setData({ isReviewed: true })
      }
    } catch (error) {
       // 404 is normal here, means not reviewed
       this.setData({ isReviewed: false })
    }
  },

  onReview() {
    const { orderId, isReviewed } = this.data
    if (isReviewed) {
        wx.navigateTo({ url: '/pages/user_center/reviews/index' })
        return
    }
    wx.navigateTo({
      url: `/pages/user_center/reviews/create/index?orderId=${orderId}`
    })
  },

  onShareAppMessage() {
    const { order, orderId } = this.data
    return {
      title: order?.merchantName ? `${order.merchantName} 订单详情` : '订单详情',
      path: `/pages/orders/detail/index?id=${orderId}`
    }
  },

  onShareTimeline() {
    const { order } = this.data
    return {
      title: order?.merchantName ? `${order.merchantName} 订单记录` : '订单记录'
    }
  }
})
