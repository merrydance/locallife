import * as CartAPI from '../../../api/cart'
import { logger } from '../../../utils/logger'
import { createOrder, OrderType } from '../../../api/order'
import {
  createCombinedPaymentOrder,
  createOrderPayment
} from '../../../api/payment'
import {
  completeCombinedPaymentWorkflow,
  completePaymentWorkflow,
  isCombinedPaymentWorkflowCancelled,
  isCombinedPaymentWorkflowPaid
} from '../../../services/payment-workflow'
import Navigation from '../../../utils/navigation'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  getCheckoutAddressDetail,
  getDefaultCheckoutAddress,
  loadCheckoutPaymentCapabilities,
  loadTakeoutMembershipState,
  type CheckoutAddress
} from '../../../services/takeout-checkout'
import {
  buildAddressSyncKey,
  buildCheckoutSnapshotPatch,
  buildOrderConfirmCartViews,
  buildPricingKey,
  buildPricingSuccessPatch,
  buildTakeoutCreateOrderRequest,
  buildTodaySlots,
  CheckoutSnapshotPayload,
  getCombinedPaymentPageMessage,
  mapWithConcurrency,
  MerchantCartView,
  navigateToCombinedPaymentSuccess,
  ORDER_CONFIRM_CONCURRENCY,
  syncTakeoutCartSummary
} from '../../../utils/takeout-order-confirm-support'

let _loadPaymentCapabilitiesPromise: Promise<void> | null = null

Page({
  data: {
    carts: [] as MerchantCartView[],
    cartIds: [] as number[],
    address: null as CheckoutAddress | null,
    remarks: {} as Record<number, string>,
    navBarHeight: 88,
    initLoading: true, // 页面初始化加载标志（用于骨架屏）
    loading: false,    // 按钮提交加载标志
    loadError: '',
    pricingError: '',
    orderTotalDisplay: '0.00',
    summarySubtotalDisplay: '0.00',
    summaryDeliveryDisplay: '待计算',
    splitCheckoutRequired: false,
    splitCheckoutNotice: '',
    
    // 会员优惠相关
    memberBalances: {} as Record<number, number>, // merchantId -> balance
    membershipIds: {} as Record<number, number>
  },

  _pricingRequestVersion: 0,
  _pricingInFlight: false,
  _activePricingKey: '',
  _pricingRefreshPending: false,
  _pendingPricingSilent: true,
  _defaultAddressLoaded: false,
  _snapshotFallbackTimer: 0 as number | 0,

  onLoad(options: { cart_ids?: string, data?: string }) {
    // 解析URL中的cart_ids参数
    if (options.cart_ids) {
      const cartIds = options.cart_ids.split(',').map(Number).filter((id) => !isNaN(id))
      this.setData({ cartIds })
    }

    let hasSnapshot = false
    const openerEventChannel = this.getOpenerEventChannel()
    if (openerEventChannel?.on) {
      openerEventChannel.on('checkoutContext', (payload: CheckoutSnapshotPayload) => {
        if (this.applyCheckoutSnapshot(payload)) {
          hasSnapshot = true
        }
      })
    }

    if (options.data) {
      try {
        const payload = JSON.parse(decodeURIComponent(options.data)) as CheckoutSnapshotPayload
        hasSnapshot = this.applyCheckoutSnapshot(payload) || hasSnapshot
      } catch (error) {
        logger.warn('Parse checkout snapshot failed', error, 'Order-confirm')
      }
    }

    if (!hasSnapshot) {
      this._snapshotFallbackTimer = setTimeout(() => {
        if (!this.data.carts.length && !this.data.loadError) {
          this.loadCart()
        }
        this._snapshotFallbackTimer = 0
      }, 120)
    }

    this.loadDefaultAddress()
    void this.loadPaymentCapabilities()
  },

  onUnload() {
    if (this._snapshotFallbackTimer) {
      clearTimeout(this._snapshotFallbackTimer)
      this._snapshotFallbackTimer = 0
    }
  },

  onShow() {
    // If returning from address selection, we might have a selectedAddressId
    const pages = getCurrentPages()
    const currPage = pages[pages.length - 1] as WechatMiniprogram.Page.Instance<WechatMiniprogram.IAnyObject, WechatMiniprogram.IAnyObject> & {
      data: { selectedAddressId?: number | string | null }
    }
    if (currPage.data?.selectedAddressId) {
      this.loadAddressById(currPage.data.selectedAddressId)
      currPage.setData({ selectedAddressId: null })
    }
  },

  updateAddress(address: CheckoutAddress | null) {
    const currentAddressKey = buildAddressSyncKey(this.data.address)
    const nextAddressKey = buildAddressSyncKey(address)

    if (currentAddressKey === nextAddressKey) {
      return
    }

    this.setData({ address })
    this.requestPricingRefresh()
  },

  applyCheckoutSnapshot(payload?: CheckoutSnapshotPayload | null) {
    if (!payload?.carts || payload.carts.length === 0) {
      return false
    }

    if (this._snapshotFallbackTimer) {
      clearTimeout(this._snapshotFallbackTimer)
      this._snapshotFallbackTimer = 0
    }

    this.setData(buildCheckoutSnapshotPatch(payload, this.data.cartIds))

    this.requestPricingRefresh(true)
    void this.loadMemberships()
    return true
  },

  syncGlobalCartSummary(carts: MerchantCartView[]) {
    syncTakeoutCartSummary(carts)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadCart() {
    try {
      this.setData({ initLoading: true, loadError: '', pricingError: '' })

      // Step 1: 获取外卖类型的购物车列表
      const userCarts = await CartAPI.getUserCarts('takeout')
      
      if (!userCarts.carts || userCarts.carts.length === 0) {
        wx.showToast({ title: '购物车为空', icon: 'none' })
        setTimeout(() => wx.navigateBack(), 1500)
        return
      }

      const { cartIds } = this.data
      let selectedCarts = userCarts.carts
      if (cartIds.length > 0) {
        selectedCarts = userCarts.carts.filter((c) => cartIds.includes(c.cart_id || 0))
      }
      if (selectedCarts.length === 0) selectedCarts = [userCarts.carts[0]]

      const merchantCarts = selectedCarts.filter((mc) => !!mc.merchant_id)
      const rawResults = await mapWithConcurrency(merchantCarts, ORDER_CONFIRM_CONCURRENCY, async (merchantCart) => {
        const merchantId = merchantCart.merchant_id as number
        const cartDetail = await CartAPI.getCart({
          merchant_id: merchantId,
          order_type: (merchantCart.order_type || 'takeout') as OrderType,
          table_id: merchantCart.table_id || undefined,
          reservation_id: merchantCart.reservation_id || undefined
        })
        return { merchantCart, merchantId, cartDetail }
      })

      const cartViews = buildOrderConfirmCartViews(rawResults)

      this.setData({ carts: cartViews, initLoading: false })

      // 根据地址计算每个商户配送费。通过 key 去重，避免 onLoad 并发加载购物车与默认地址时重复试算。
      this.requestPricingRefresh(true)
      
      // 加载会员信息
      await this.loadMemberships()
    } catch (error) {
      logger.error('Load cart failed', error, 'Order-confirm')
      this.setData({
        initLoading: false,
        loadError: getErrorUserMessage(error, '购物车加载失败，请重试')
      })
    }
  },

  onRetryLoad() {
    this.loadCart()
  },

  async loadMemberships() {
    const { carts } = this.data
    if (!carts || carts.length === 0) return

    try {
      this.setData(await loadTakeoutMembershipState(carts.map((cart) => cart.merchantId)))
    } catch (error) {
      logger.error('Load memberships failed', error, 'Order-confirm')
    }
  },

  async loadDefaultAddress() {
    if (this._defaultAddressLoaded) {
      return
    }

    try {
      this._defaultAddressLoaded = true
      const defaultAddr = await getDefaultCheckoutAddress()
      if (defaultAddr) {
        this.updateAddress(defaultAddr)
      }
    } catch (error) {
      this._defaultAddressLoaded = false
      logger.error('Load address failed', error, 'Order-confirm')
    }
  },

  async loadAddressById(id: number | string) {
    const nextAddressId = Number(id)
    if (!nextAddressId) {
      return
    }

    try {
      const addr = await getCheckoutAddressDetail(nextAddressId)
      this.updateAddress(addr)
    } catch (error) {
      logger.error('Load address failed', error, 'Order-confirm')
    }
  },

  onSelectAddress() {
    Navigation.toAddressSelector()
  },

  onRemarkInput(e: WechatMiniprogram.CustomEvent) {
    const { merchantId } = e.currentTarget.dataset as { merchantId?: number | string }
    const merchantIdNum = Number(merchantId)
    if (!merchantIdNum) return
    const remarks = { ...(this.data.remarks || {}), [merchantIdNum]: e.detail.value }
    this.setData({ remarks })
  },

  onDeliveryTimeChange() {},

  /**
   * 选择预约时间（仅当天，默认营业时段 10:00-22:00，30 分钟粒度）
   */
  async chooseScheduleSlot() {
    const slots = buildTodaySlots(10, 22, 30)
    if (slots.length === 0) {
      wx.showToast({ title: '今日无可选时间', icon: 'none' })
      this.setData({ deliveryTime: 'ASAP', scheduleSlot: '' })
      return
    }

    try {
      const res = await wx.showActionSheet({ itemList: slots })
      const picked = slots[res.tapIndex]
      this.setData({ deliveryTime: 'SCHEDULED', scheduleSlot: picked })
    } catch (err) {
      // 取消选择则回退到尽快送达
      this.setData({ deliveryTime: 'ASAP', scheduleSlot: '' })
    }
  },

  /**
   * 构建当天可选时间段
   */
  requestPricingRefresh(silent: boolean = false) {
    const pricingKey = buildPricingKey(this.data.address, this.data.carts)
    if (pricingKey && this._pricingInFlight && pricingKey === this._activePricingKey) {
      if (!this._pricingRefreshPending) {
        this._pendingPricingSilent = silent
      } else {
        this._pendingPricingSilent = this._pendingPricingSilent && silent
      }
      this._pricingRefreshPending = true
      return
    }

    if (pricingKey) {
      this._activePricingKey = pricingKey
    }

    void this.calculateDeliveryFee(silent)
  },

  /**
   * 计算配送费并更新应付总额
   */
  async calculateDeliveryFee(_silent: boolean = false) {
    const { address } = this.data
    const currentCarts = this.data.carts
    if (!currentCarts || currentCarts.length === 0) {
      this._pricingInFlight = false
      this._activePricingKey = ''
      return
    }

    const requestVersion = ++this._pricingRequestVersion
    const pricingKey = buildPricingKey(address, currentCarts)
    this._pricingInFlight = !!pricingKey

    if (!address) {
      if (requestVersion !== this._pricingRequestVersion) return
      this.setData({
        pricingError: '',
        summaryDeliveryDisplay: '待选择地址',
        orderTotalDisplay: currentCarts.reduce((sum, cart) => sum + (cart.subtotal || 0), 0).toString()
      })
      this._pricingInFlight = false
      this._activePricingKey = ''
      return
    }

    try {
      const calcResults = await mapWithConcurrency(currentCarts, ORDER_CONFIRM_CONCURRENCY, async (cart) => {
        const result = await CartAPI.calculateCart({
          merchant_id: cart.merchantId,
          order_type: cart.orderType,
          address_id: address.id,
          latitude: address.latitude ? Number(address.latitude) : undefined,
          longitude: address.longitude ? Number(address.longitude) : undefined
        }, { loading: false })
        return { cart, result }
      })

      if (requestVersion !== this._pricingRequestVersion) {
        return
      }

      this.setData(buildPricingSuccessPatch(calcResults))
    } catch (error) {
      logger.error('Calculate delivery fee failed', error, 'Order-confirm')
      if (requestVersion !== this._pricingRequestVersion) {
        return
      }
      const pricingError = getErrorUserMessage(error, '配送费计算失败，请重试')
      this.setData({ pricingError })
    } finally {
      if (requestVersion === this._pricingRequestVersion) {
        this._pricingInFlight = false
        this._activePricingKey = buildPricingKey(this.data.address, this.data.carts)
        if (this._pricingRefreshPending) {
          const rerunSilent = this._pendingPricingSilent
          this._pricingRefreshPending = false
          this._pendingPricingSilent = true
          this.requestPricingRefresh(rerunSilent)
        }
      }
    }
  },

  onRetryPricing() {
    this.requestPricingRefresh()
  },

  async loadPaymentCapabilities() {
    if (_loadPaymentCapabilitiesPromise) {
      return _loadPaymentCapabilitiesPromise
    }

    _loadPaymentCapabilitiesPromise = (async () => {
      try {
        const capabilities = await loadCheckoutPaymentCapabilities()
        const splitCheckoutRequired = capabilities.splitCheckoutRequired
        this.setData({
          splitCheckoutRequired,
          splitCheckoutNotice: capabilities.splitCheckoutNotice
        })
      } catch (error) {
        logger.warn('Failed to load payment capabilities', error, 'Order-confirm')
      }
    })().finally(() => {
      _loadPaymentCapabilitiesPromise = null
    })

    return _loadPaymentCapabilitiesPromise
  },

  showSplitCheckoutRequired() {
    wx.showModal({
      title: '请分开支付',
      content: this.data.splitCheckoutNotice || '当前支付通道需按商户分别下单支付，请返回购物车一次选择一家商户。',
      showCancel: false,
      confirmText: '返回购物车',
      success: () => wx.navigateBack()
    })
  },

  async onSubmitOrder() {
    if (this.data.loading) {
      return
    }

    const { carts, address, remarks, loadError, pricingError } = this.data

    if (loadError) {
      wx.showToast({ title: '请先重试加载购物车', icon: 'none' })
      return
    }

    if (pricingError) {
      wx.showToast({ title: '请先重试配送费计算', icon: 'none' })
      return
    }

    if (!address || !address.id) {
      wx.showToast({ title: '请选择收货地址', icon: 'none' })
      return
    }

    if (!carts || carts.length === 0) {
      wx.showToast({ title: '购物车为空', icon: 'none' })
      return
    }

    await this.loadPaymentCapabilities()
    if (this.data.splitCheckoutRequired && carts.length > 1) {
      this.showSplitCheckoutRequired()
      return
    }

    this.setData({ loading: true })
    const ordersCreated: number[] = []

    try {
      for (const cart of carts) {
        const order = await createOrder(buildTakeoutCreateOrderRequest({
          cart,
          addressId: address.id,
          note: remarks[cart.merchantId] || ''
        }))
        ordersCreated.push(order.id)

        // 按商户清空购物车，避免下单后按商品逐个删除导致请求数线性放大。
        try {
          await CartAPI.clearCart({
            merchant_id: cart.merchantId,
            order_type: cart.orderType,
            table_id: cart.tableId,
            reservation_id: cart.reservationId
          })
        } catch (clearErr) {
          logger.error('Clear merchant cart after order failed', clearErr, 'Order-confirm')
          // 清理失败不阻断支付流程，仅记录日志
        }
      }

      // 无论购物车项清理是否成功，都立即使 globalStore 缓存失效，
      // 确保外卖首页 / 购物车页的下次 onShow 拿到空购物车，而不是脏缓存。
      this.syncGlobalCartSummary([])

      if (ordersCreated.length === 1) {
        await this.handlePayment(ordersCreated[0])
      } else {
        await this.handleCombinedPayment(ordersCreated)
      }
    } catch (error) {
      logger.error('Create order failed:', error, 'Order-confirm')

      if (ordersCreated.length > 0) {
        this.handlePartialOrderCreationFailure(carts, ordersCreated)
        return
      }

      wx.showToast({ title: getErrorUserMessage(error, '下单失败，请稍后重试'), icon: 'none' })
      this.setData({ loading: false })
    }
  },

  handlePartialOrderCreationFailure(carts: MerchantCartView[], orderIds: number[]) {
    this.setData({ loading: false })

    const remainingCarts = carts.slice(orderIds.length)
    this.syncGlobalCartSummary(remainingCarts)

    const createdCount = orderIds.length
    const remainingCount = remainingCarts.length
    const content = remainingCount > 0
      ? `已有${createdCount}笔订单创建成功，剩余${remainingCount}笔未创建。请先到订单列表查看已创建订单并继续支付，未创建部分再返回重试。`
      : `已有${createdCount}笔订单创建成功，请先到订单列表继续支付。`

    wx.showModal({
      title: '部分订单已创建',
      content,
      showCancel: false,
      confirmText: '查看订单',
      success: () => Navigation.redirectToOrderList({ orderType: 'takeout' })
    })
  },

  async handlePayment(orderId: number) {
    try {
      const paymentResult = await completePaymentWorkflow(await createOrderPayment(orderId))
      Navigation.toPaymentResult({
        status: paymentResult.status,
        paymentOrderId: paymentResult.paymentOrderId,
        businessId: orderId,
        businessType: paymentResult.businessType || 'order',
        orderNo: paymentResult.outTradeNo || String(orderId),
        amount: paymentResult.amountFen ? (paymentResult.amountFen / 100).toFixed(2) : undefined
      })
    } catch (paymentError) {
      logger.error('Payment creation failed', paymentError, 'Order-confirm')
      this.showPaymentCreateFailed(orderId)
    } finally {
      this.setData({ loading: false })
    }
  },

  async handleCombinedPayment(orderIds: number[]) {
    try {
      const paymentResult = await completeCombinedPaymentWorkflow(await createCombinedPaymentOrder({ order_ids: orderIds }))
      const combinedPayment = paymentResult.combinedPayment

      if (isCombinedPaymentWorkflowPaid(paymentResult.status)) {
        navigateToCombinedPaymentSuccess(combinedPayment, orderIds)
        return
      }

      if (isCombinedPaymentWorkflowCancelled(paymentResult.status)) {
        wx.showModal({
          title: '支付未完成',
          content: '订单已创建，可在订单列表继续完成合单支付。',
          showCancel: false,
          confirmText: '查看订单',
          success: () => Navigation.redirectToOrderList({ orderType: 'takeout' })
        })
        return
      }

      wx.showModal({
        title: '订单已创建',
        content: getCombinedPaymentPageMessage(combinedPayment),
        showCancel: false,
        success: () => Navigation.redirectToOrderList({ orderType: 'takeout' })
      })
    } catch (paymentError) {
      logger.error('Combined payment creation failed', paymentError, 'Order-confirm')
      const paymentMessage = getErrorUserMessage(paymentError, '')
      const splitRequired = paymentMessage.includes('分开支付') || paymentMessage.includes('合单支付暂未开通')
      wx.showModal({
        title: '订单已创建',
        content: splitRequired
          ? '当前支付通道需按商户分别支付。订单已创建，请在订单列表逐笔支付。'
          : '支付创建失败，请在订单列表继续完成合单支付。',
        showCancel: false,
        success: () => Navigation.redirectToOrderList({ orderType: 'takeout' })
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  showPaymentCreateFailed(orderId: number) {
    this.setData({ loading: false })
    wx.showModal({
      title: '订单已创建',
      content: '支付创建失败，请在订单详情页重新发起支付。',
      showCancel: false,
      confirmText: '查看订单',
      success: () => {
        wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })
      }
    })
  },

  /**
   * 充值成功回调
   */
  onRecharged() {
    this.loadMemberships() // 重新加载余额
  },

  /**
   * 领券成功回调
   */
  onVoucherClaimed() {
    wx.showToast({ title: '领券完成', icon: 'success' })
    // 重要：领券后立即重新计算优惠金额，让用户看到变化
    this.requestPricingRefresh()
  }

})
