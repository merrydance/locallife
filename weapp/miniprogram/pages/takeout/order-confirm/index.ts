import * as CartAPI from '../../../api/cart'
import { logger } from '../../../utils/logger'
import { createOrder, isPaidOrderStatus, OrderResponse, OrderType } from './_main_shared/api/order'
import { createOrderPayment } from './_main_shared/api/payment'
import { completePaymentWorkflow } from './_main_shared/services/payment-workflow'
import Navigation from '../../../utils/navigation'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  getCheckoutAddressDetail,
  getDefaultCheckoutAddress,
  loadTakeoutMembershipState,
  type CheckoutAddress
} from './_services/takeout-checkout'
import {
  buildTakeoutOrderCreateRequestSignature,
  clearTakeoutOrderCreateIdempotency,
  ensureTakeoutOrderCreateIdempotencyKey
} from './_services/takeout-order-create-idempotency'
import {
  buildAddressSyncKey,
  buildCheckoutSnapshotPatch,
  buildCheckoutPaymentMethods,
  checkoutRequiresAddress,
  buildOrderConfirmCartViews,
  buildPricingKey,
  buildPricingSuccessPatch,
  buildTakeoutCreateOrderRequest,
  CheckoutSnapshotPayload,
  mapWithConcurrency,
  MerchantCartView,
  ORDER_CONFIRM_CONCURRENCY,
  resolveSelectedPaymentMethod,
  syncTakeoutCartSummary
} from './_utils/takeout-order-confirm-support'
import { getTakeoutPaymentCreateFailedContent } from './_utils/takeout-payment-error-copy'

Page({
  data: {
    carts: [] as MerchantCartView[],
    cartIds: [] as number[],
    orderType: 'takeout' as 'takeout' | 'takeaway',
    address: null as CheckoutAddress | null,
    remarks: {} as Record<number, string>,
    selectedPaymentMethod: 'wechat_pay' as 'wechat_pay' | 'balance',
    paymentMethods: buildCheckoutPaymentMethods(null, {}),
    requiresAddress: true,
    balanceInsufficient: false,
    navBarHeight: 88,
    initLoading: true, // 页面初始化加载标志（用于骨架屏）
    loading: false,    // 按钮提交加载标志
    loadError: '',
    pricingError: '',
    orderTotalDisplay: '0.00',
    summarySubtotalDisplay: '0.00',
    summaryDeliveryLabel: '代取总费',
    summaryDeliveryDisplay: '待计算',
    singleMerchantCheckoutNotice: '',
    
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

  onLoad(options: { cart_ids?: string, data?: string, order_type?: 'takeout' | 'takeaway' }) {
    if (options.order_type === 'takeaway') {
      this.setData({ orderType: 'takeaway' })
    }

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

    if (this.data.orderType === 'takeout') {
      this.loadDefaultAddress()
    }
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

    const patch = buildCheckoutSnapshotPatch(payload, this.data.cartIds)
    const nextCarts = patch.carts || []
    const requiresAddress = checkoutRequiresAddress(nextCarts)
    this.setData({
      ...patch,
      orderType: nextCarts[0]?.orderType === 'takeaway' ? 'takeaway' : this.data.orderType,
      requiresAddress
    })
    this.updatePaymentState()

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

      const userCarts = await CartAPI.getUserCarts(this.data.orderType)
      
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

      this.setData({
        carts: cartViews,
        initLoading: false,
        requiresAddress: checkoutRequiresAddress(cartViews)
      })
      this.updatePaymentState()

      // 根据地址计算每个商户代取费。通过 key 去重，避免 onLoad 并发加载购物车与默认地址时重复试算。
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
      this.updatePaymentState()
    } catch (error) {
      logger.error('Load memberships failed', error, 'Order-confirm')
    }
  },

  async loadDefaultAddress() {
    if (this.data.orderType === 'takeaway' || !this.data.requiresAddress || this._defaultAddressLoaded) {
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
    if (!this.data.requiresAddress) {
      return
    }
    Navigation.toAddressSelector()
  },

  onRemarkInput(e: WechatMiniprogram.CustomEvent) {
    const { merchantId } = e.currentTarget.dataset as { merchantId?: number | string }
    const merchantIdNum = Number(merchantId)
    if (!merchantIdNum) return
    const remarks = { ...(this.data.remarks || {}), [merchantIdNum]: e.detail.value }
    this.setData({ remarks })
  },

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
   * 计算代取费并更新应付总额
   */
  async calculateDeliveryFee(_silent: boolean = false) {
    const { address, requiresAddress } = this.data
    const currentCarts = this.data.carts
    if (!currentCarts || currentCarts.length === 0) {
      this._pricingInFlight = false
      this._activePricingKey = ''
      return
    }

    const requestVersion = ++this._pricingRequestVersion
    const pricingKey = buildPricingKey(address, currentCarts)
    this._pricingInFlight = !!pricingKey

    if (requiresAddress && !address) {
      if (requestVersion !== this._pricingRequestVersion) return
      this.setData({
        pricingError: '',
        summaryDeliveryDisplay: '待选择地址',
        orderTotalDisplay: (currentCarts.reduce((sum, cart) => sum + (cart.subtotal || 0), 0) / 100).toFixed(2)
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
          address_id: requiresAddress ? address?.id : undefined,
          latitude: requiresAddress && address?.latitude ? Number(address.latitude) : undefined,
          longitude: requiresAddress && address?.longitude ? Number(address.longitude) : undefined
        }, { loading: false })
        return { cart, result }
      })

      if (requestVersion !== this._pricingRequestVersion) {
        return
      }

      this.setData(buildPricingSuccessPatch(calcResults))
      this.updatePaymentState()
    } catch (error) {
      logger.error('Calculate delivery fee failed', error, 'Order-confirm')
      if (requestVersion !== this._pricingRequestVersion) {
        return
      }
      const pricingError = getErrorUserMessage(error, '代取费计算失败，请重试')
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

  getPrimaryCart(): MerchantCartView | null {
    return this.data.carts && this.data.carts.length === 1 ? this.data.carts[0] : null
  },

  updatePaymentState() {
    const cart = this.getPrimaryCart()
    const selectedPaymentMethod = resolveSelectedPaymentMethod(
      cart,
      this.data.memberBalances,
      this.data.selectedPaymentMethod
    )
    const memberBalance = cart ? (this.data.memberBalances[cart.merchantId] || 0) : 0
    this.setData({
      paymentMethods: buildCheckoutPaymentMethods(cart, this.data.memberBalances),
      selectedPaymentMethod,
      balanceInsufficient: !!cart && memberBalance > 0 && memberBalance < (cart.orderTotal || 0)
    })
  },

  onPaymentMethodChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ selectedPaymentMethod: e.detail.value === 'balance' ? 'balance' : 'wechat_pay' })
    this.updatePaymentState()
  },

  getSelectedMerchantIds(carts: MerchantCartView[]): number[] {
    return Array.from(new Set((carts || []).map((cart) => cart.merchantId).filter(Boolean)))
  },

  hasMultiMerchantSelection(carts: MerchantCartView[]): boolean {
    return this.getSelectedMerchantIds(carts).length !== 1 || carts.length !== 1
  },

  showSingleMerchantRequired() {
    wx.showModal({
      title: '暂不支持多商户一起支付',
      content: this.data.singleMerchantCheckoutNotice || '请返回购物车，只选择一家商户的商品结算。',
      showCancel: false,
      confirmText: '返回购物车',
      success: () => wx.navigateBack()
    })
  },

  async onSubmitOrder() {
    if (this.data.loading) {
      return
    }

    const { carts, address, remarks, loadError, pricingError, requiresAddress, selectedPaymentMethod } = this.data

    if (loadError) {
      wx.showToast({ title: '请先重试加载购物车', icon: 'none' })
      return
    }

    if (pricingError) {
      wx.showToast({ title: '请先重试代取费计算', icon: 'none' })
      return
    }

    if (requiresAddress && (!address || !address.id)) {
      wx.showToast({ title: '请选择收货地址', icon: 'none' })
      return
    }

    if (!carts || carts.length === 0) {
      wx.showToast({ title: '购物车为空', icon: 'none' })
      return
    }

    if (this.hasMultiMerchantSelection(carts)) {
      this.setData({ singleMerchantCheckoutNotice: '暂不支持多商户一起支付，请只选择一家商户的商品结算。' })
      this.showSingleMerchantRequired()
      return
    }

    this.setData({ loading: true })
    const ordersCreated: number[] = []
    let createdOrder: OrderResponse | null = null

    try {
      for (const cart of carts) {
        const orderRequest = buildTakeoutCreateOrderRequest({
          cart,
          addressId: address?.id,
          note: remarks[cart.merchantId] || '',
          useBalance: selectedPaymentMethod === 'balance'
        })
        const orderRequestSignature = buildTakeoutOrderCreateRequestSignature(orderRequest)
        const idempotencyKey = ensureTakeoutOrderCreateIdempotencyKey(orderRequestSignature)
        const order = await createOrder(orderRequest, { idempotencyKey })
        clearTakeoutOrderCreateIdempotency(idempotencyKey)
        createdOrder = order
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

      if (!createdOrder) {
        throw new Error('订单创建结果缺失')
      }
      await this.handlePayment(createdOrder)
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
      success: () => Navigation.redirectToOrderList({ orderType: carts[0]?.orderType || 'takeout' })
    })
  },

  async handlePayment(order: OrderResponse) {
    const orderId = order.id
    const paidBalanceOrder = isPaidOrderStatus(order.status) && order.payment_method === 'balance'
    try {
      if (paidBalanceOrder) {
        Navigation.toPaymentResult({
          status: 'paid',
          businessId: orderId,
          businessType: 'order',
          orderNo: String(orderId),
          amount: this.data.orderTotalDisplay
        })
        return
      }

      const paymentResult = await completePaymentWorkflow(await createOrderPayment(orderId), { context: this })
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
      this.showPaymentCreateFailed(orderId, paymentError)
    } finally {
      this.setData({ loading: false })
    }
  },

  showPaymentCreateFailed(orderId: number, error?: unknown) {
    this.setData({ loading: false })
    wx.showModal({
      title: '订单已创建',
      content: getTakeoutPaymentCreateFailedContent(error),
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
