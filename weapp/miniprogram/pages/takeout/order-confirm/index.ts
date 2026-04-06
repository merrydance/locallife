import * as CartAPI from '../../../api/cart'
import { CartItemResponse } from '../../../api/cart'
import { logger } from '../../../utils/logger'
import AddressService, { Address } from '../../../api/address'
import { createOrder, CreateOrderRequest, OrderItemRequest, OrderType } from '../../../api/order'
import { createOrderPayment, createCombinedPaymentOrder, invokeWechatPay } from '../../../api/payment'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'
import { getMyMemberships, MembershipResponse } from '../../../api/personal'
import Navigation from '../../../utils/navigation'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { globalStore } from '../../../utils/global-store'

interface CartItemView {
  id: number
  dishId?: number
  comboId?: number
  name: string
  imageUrl: string
  quantity: number
  unitPrice: number
  priceDisplay: string
  subtotal: number
  subtotalDisplay: string
  specText?: string
  customizations?: Record<string, unknown>
  dishImages?: string[]
}

interface MerchantCartView {
  merchantId: number
  merchantName: string
  orderType: OrderType
  tableId?: number
  reservationId?: number
  items: CartItemView[]
  totalCount: number
  subtotal: number
  subtotalDisplay: string
  deliveryFee: number
  deliveryFeeDisplay: string
  deliveryFeeDiscount: number
  deliveryDistance: number
  deliveryEtaMinutes: number
  deliveryEtaDisplay: string
  orderTotal: number
  orderTotalDisplay: string
  originalTotalDisplay: string // 原价（不含优惠）
  hasDiscount: boolean         // 是否有优惠
  appliedPromotions: Array<{ title: string, amount: number, amountDisplay: string, type: string }>
  ladderPromotions: Array<{ name: string, thresholdDisplay: string, discountDisplay: string, currentHit: boolean, missingNeedDisplay: string }>
  voucherTrials: Array<{ voucherName: string, amountDisplay: string, trialPayableDisplay: string }>
  paymentHint: string
}

Page({
  data: {
    carts: [] as MerchantCartView[],
    cartIds: [] as number[],
    address: null as Address | null,
    remarks: {} as Record<number, string>,
    navBarHeight: 88,
    initLoading: true, // 页面初始化加载标志（用于骨架屏）
    loading: false,    // 按钮提交加载标志
    loadError: '',
    pricingError: '',
    orderTotalDisplay: '0.00',
    summarySubtotalDisplay: '0.00',
    summaryDeliveryDisplay: '待计算',
    
    // 支付及会员相关
    selectedPaymentMethod: 'wechat', // 'wechat' | 'balance'
    memberBalances: {} as Record<number, number>, // merchantId -> balance
    memberBalanceDisplays: {} as Record<number, string>,
    membershipIds: {} as Record<number, number>
  },

  onLoad(options: { cart_ids?: string }) {
    // 解析URL中的cart_ids参数
    if (options.cart_ids) {
      const cartIds = options.cart_ids.split(',').map(Number).filter((id) => !isNaN(id))
      this.setData({ cartIds })
    }
    this.loadCart()
    this.loadDefaultAddress()
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

      // 并行拉取各商户购物车详情
      const rawResults = await Promise.all(
        selectedCarts
          .filter((mc) => !!mc.merchant_id)
          .map(async (merchantCart) => {
            const merchantId = merchantCart.merchant_id as number
            const cartDetail = await CartAPI.getCart({
              merchant_id: merchantId,
              order_type: (merchantCart.order_type || 'takeout') as OrderType,
              table_id: merchantCart.table_id || undefined,
              reservation_id: merchantCart.reservation_id || undefined
            })
            return { merchantCart, merchantId, cartDetail }
          })
      )

      const cartViews: MerchantCartView[] = rawResults
        .filter(({ cartDetail }) => cartDetail.items && cartDetail.items.length > 0)
        .map(({ merchantCart, merchantId, cartDetail }) => {
          const items: CartItemView[] = cartDetail.items.map((item: CartItemResponse) => ({
            id: item.id,
            dishId: item.dish_id,
            comboId: item.combo_id,
            name: item.name,
            imageUrl: getPublicImageUrl(item.image_url || ''),
            quantity: item.quantity,
            unitPrice: item.unit_price,
            priceDisplay: formatPriceNoSymbol(item.unit_price),
            subtotal: item.subtotal,
            subtotalDisplay: formatPriceNoSymbol(item.subtotal),
            specText: item.spec_text || '',
            customizations: item.customizations || undefined,
            dishImages: (item.combo_member_images || []).map((url: string) => getPublicImageUrl(url))
          }))
          return {
            merchantId,
            merchantName: merchantCart.merchant_name || '商家',
            orderType: (merchantCart.order_type || 'takeout') as OrderType,
            tableId: merchantCart.table_id || undefined,
            reservationId: merchantCart.reservation_id || undefined,
            items,
            totalCount: items.reduce((sum, item) => sum + item.quantity, 0),
            subtotal: cartDetail.subtotal,
            subtotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
            deliveryFee: 0,
            deliveryFeeDisplay: '待计算',
            deliveryFeeDiscount: 0,
            deliveryDistance: 0,
            deliveryEtaMinutes: 0,
            deliveryEtaDisplay: '',
            orderTotal: cartDetail.subtotal,
            orderTotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
            originalTotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
            hasDiscount: false,
            appliedPromotions: [],
            ladderPromotions: [],
            voucherTrials: [],
            paymentHint: ''
          }
        })

      this.setData({ carts: cartViews, initLoading: false })
      
      // 根据地址计算每个商户配送费 (silent)
      await this.calculateDeliveryFee(true)
      
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
      const result = await getMyMemberships()
      const memberBalances: Record<number, number> = {}
      const memberBalanceDisplays: Record<number, string> = {}
      const membershipIds: Record<number, number> = {}

      carts.forEach((cart) => {
        const membership = result.memberships?.find(
          (m: MembershipResponse) => m.merchant_id === cart.merchantId
        )
        if (membership) {
          memberBalances[cart.merchantId] = membership.balance
          memberBalanceDisplays[cart.merchantId] = formatPriceNoSymbol(membership.balance)
          membershipIds[cart.merchantId] = membership.id
        }
      })

      this.setData({
        memberBalances,
        memberBalanceDisplays,
        membershipIds
      })
    } catch (error) {
      logger.error('Load memberships failed', error, 'Order-confirm')
    }
  },

  async loadDefaultAddress() {
    try {
      const addresses = await AddressService.getAddresses()
      if (addresses && addresses.length > 0) {
        const defaultAddr = addresses.find((a: Address) => a.is_default) || addresses[0]
        this.setData({ address: defaultAddr })
        await this.calculateDeliveryFee()
      }
    } catch (error) {
      logger.error('Load address failed', error, 'Order-confirm')
    }
  },

  async loadAddressById(id: number | string) {
    try {
      const addresses = await AddressService.getAddresses()
      const addr = addresses.find((a: Address) => String(a.id) === String(id))
      if (addr) {
        this.setData({ address: addr })
        await this.calculateDeliveryFee()
      }
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
    const slots = this.buildTodaySlots(10, 22, 30)
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
  buildTodaySlots(startHour: number, endHour: number, stepMinutes: number): string[] {
    const now = new Date()
    const slots: string[] = []
    for (let h = startHour; h < endHour; h++) {
      for (let m = 0; m < 60; m += stepMinutes) {
        const slot = new Date(now)
        slot.setHours(h, m, 0, 0)
        if (slot.getTime() > now.getTime()) {
          const hh = String(slot.getHours()).padStart(2, '0')
          const mm = String(slot.getMinutes()).padStart(2, '0')
          slots.push(`${hh}:${mm}`)
        }
      }
    }
    return slots
  },

  /**
   * 计算配送费并更新应付总额
   */
  async calculateDeliveryFee(_silent: boolean = false) {
    const { address } = this.data
    const currentCarts = this.data.carts
    if (!currentCarts || currentCarts.length === 0) return

    if (!address) {
      this.setData({
        pricingError: '',
        summaryDeliveryDisplay: '待选择地址',
        orderTotalDisplay: formatPriceNoSymbol(currentCarts.reduce((s, c) => s + (c.subtotal || 0), 0))
      })
      return
    }

    try {
      // 并行计算各商户配送费
      const calcResults = await Promise.all(
        currentCarts.map(async (cart) => {
          const result = await CartAPI.calculateCart({
            merchant_id: cart.merchantId,
            order_type: cart.orderType,
            address_id: address.id,
            latitude: address.latitude ? Number(address.latitude) : undefined,
            longitude: address.longitude ? Number(address.longitude) : undefined
          }, { loading: false })
          return { cart, result }
        })
      )

      const updated = calcResults
        .filter(({ result }) => !!result)
        .map(({ cart, result }) => {
          const deliveryFee = result.delivery_fee || 0
          const deliveryFeeDiscount = result.delivery_fee_discount || 0
          const finalDeliveryFee = Math.max(0, deliveryFee - deliveryFeeDiscount)
          const deliveryDistance = result.delivery_distance || 0
          const orderTotal = result.total_amount || 0
          const originalTotal = (cart.subtotal || 0) + deliveryFee
          const hasDiscount = orderTotal < originalTotal

          const deliveryEtaMinutes = result.delivery_eta_minutes || 0
          const deliveryEtaDisplay = this.formatEtaWindow(deliveryEtaMinutes)
          const appliedPromotions = (result.applied_promotions || []).map((p) => ({
            title: p.title || '优惠',
            amount: p.amount || 0,
            amountDisplay: formatPriceNoSymbol(p.amount || 0),
            type: p.type || 'merchant'
          }))
          const ladderPromotions = (result.ladder_promotions || []).map((rule) => ({
            name: rule.name || '满减活动',
            thresholdDisplay: formatPriceNoSymbol(rule.threshold || 0),
            discountDisplay: formatPriceNoSymbol(rule.discount || 0),
            currentHit: !!rule.current_hit,
            missingNeedDisplay: formatPriceNoSymbol(rule.missing_need || 0)
          }))
          const voucherTrials = (result.voucher_trials || []).map((trial) => ({
            voucherName: trial.voucher_name || '优惠券',
            amountDisplay: formatPriceNoSymbol(trial.amount || 0),
            trialPayableDisplay: formatPriceNoSymbol(trial.trial_payable || 0)
          }))
          return {
            ...cart,
            deliveryFee,
            deliveryFeeDisplay: finalDeliveryFee > 0 ? '¥' + formatPriceNoSymbol(finalDeliveryFee) : '免代取费',
            deliveryFeeDiscount,
            deliveryDistance,
            orderTotal,
            orderTotalDisplay: formatPriceNoSymbol(orderTotal),
            originalTotalDisplay: formatPriceNoSymbol(originalTotal),
            hasDiscount,
            deliveryEtaMinutes,
            deliveryEtaDisplay,
            appliedPromotions: appliedPromotions || [],
            ladderPromotions,
            voucherTrials,
            paymentHint: result.payment_assessment?.payment_hint || ''
          }
        })

      const summarySubtotal = updated.reduce((sum, c) => {
        const merchDiscount = (c.appliedPromotions || [])
          .filter((p) => p.type === 'merchant' || p.type === 'voucher')
          .reduce((s, p) => s + (p.amount || 0), 0)
        return sum + (c.subtotal || 0) - merchDiscount
      }, 0)
      const summaryDelivery = updated.reduce((sum, c) => sum + Math.max(0, (c.deliveryFee || 0) - (c.deliveryFeeDiscount || 0)), 0)
      const totalOrderAmount = updated.reduce((sum, c) => sum + (c.orderTotal || 0), 0)

      this.setData({
        carts: updated,
        pricingError: '',
        summarySubtotalDisplay: formatPriceNoSymbol(summarySubtotal),
        summaryDeliveryDisplay: summaryDelivery > 0 ? '¥' + formatPriceNoSymbol(summaryDelivery) : '免代取费',
        orderTotalDisplay: formatPriceNoSymbol(totalOrderAmount)
      })
    } catch (error) {
      logger.error('Calculate delivery fee failed', error, 'Order-confirm')
      const pricingError = getErrorUserMessage(error, '配送费计算失败，请重试')
      this.setData({ pricingError })
    }
  },

  onRetryPricing() {
    void this.calculateDeliveryFee()
  },

  formatEtaWindow(etaMinutes: number): string {
    if (!etaMinutes || etaMinutes <= 0) return ''
    const padding = 5
    const now = new Date()
    const start = new Date(now.getTime() + Math.max(etaMinutes - padding, 0) * 60 * 1000)
    const end = new Date(now.getTime() + (etaMinutes + padding) * 60 * 1000)
    return `${this.formatTime(start)}-${this.formatTime(end)}`
  },

  formatTime(date: Date): string {
    const hh = String(date.getHours()).padStart(2, '0')
    const mm = String(date.getMinutes()).padStart(2, '0')
    return `${hh}:${mm}`
  },

  normalizeCustomizations(customizations: Record<string, unknown>): Record<string, number | string> {
    const normalized: Record<string, number | string> = {}
    Object.entries(customizations).forEach(([key, value]) => {
      if (typeof value === 'number' || typeof value === 'string') {
        normalized[key] = value
      } else if (value !== null && value !== undefined) {
        normalized[key] = String(value)
      }
    })
    return normalized
  },

  async onSubmitOrder() {
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

    this.setData({ loading: true })

    try {
      const ordersCreated: number[] = []

      for (const cart of carts) {
        const requestData: CreateOrderRequest = {
          merchant_id: cart.merchantId,
          items: cart.items.map((item) => {
            const orderItem: OrderItemRequest = {
              quantity: item.quantity
            }
            if (item.dishId) orderItem.dish_id = item.dishId
            if (item.comboId) orderItem.combo_id = item.comboId
            if (item.customizations) {
              orderItem.customizations = this.normalizeCustomizations(item.customizations as Record<string, unknown>)
            }
            return orderItem
          }),
          order_type: cart.orderType,
          address_id: address.id,
          notes: remarks[cart.merchantId] || '',
          delivery_fee: cart.deliveryFee,
          delivery_fee_discount: cart.deliveryFeeDiscount,
          delivery_distance: cart.deliveryDistance,
          // 仅在单商户下单时启用余额支付选择
          use_balance: carts.length === 1 && this.data.selectedPaymentMethod === 'balance'
        }

        const order = await createOrder(requestData)
        ordersCreated.push(order.id)

        // 清理当前商户购物车项
        const cartItemIds = cart.items.map((item) => item.id).filter(Boolean)
        if (cartItemIds.length > 0) {
          try {
            await Promise.all(cartItemIds.map((id) => CartAPI.removeFromCart(id)))
          } catch (clearErr) {
            logger.error('Remove cart items after order failed', clearErr, 'Order-confirm')
            // 清理失败不阻断支付流程，仅记录日志
          }
        }
      }

      // 无论购物车项清理是否成功，都立即使 globalStore 缓存失效，
      // 确保外卖首页 / 购物车页的下次 onShow 拿到空购物车，而不是脏缓存。
      globalStore.set('cart', { items: [], totalCount: 0, totalPrice: 0, totalPriceDisplay: '0.00' })

      if (ordersCreated.length === 1) {
        await this.handlePayment(ordersCreated[0])
      } else {
        await this.handleCombinedPayment(ordersCreated)
      }
    } catch (error) {
      logger.error('Create order failed:', error, 'Order-confirm')
      wx.showToast({ title: getErrorUserMessage(error, '下单失败，请稍后重试'), icon: 'none' })
      this.setData({ loading: false })
    }
  },

  async handlePayment(orderId: number) {
    try {
      const paymentResult = await createOrderPayment(orderId)
      const amount = (paymentResult.amount / 100).toFixed(2)
      const orderNo = paymentResult.out_trade_no || String(orderId)

      if (paymentResult.pay_params) {
        try {
          await invokeWechatPay(paymentResult.pay_params)
          Navigation.toPaymentSuccess({
            orderId: String(orderId),
            orderNo,
            amount
          })
        } catch {
          // 用户取消支付或唤起失败：订单已创建，引导用户前往订单详情重新支付
          wx.showModal({
            title: '支付未完成',
            content: '订单已创建，可在订单详情页重新发起支付。',
            showCancel: false,
            confirmText: '查看订单',
            success: () => wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })
          })
        }
      } else if (paymentResult.status === 'paid') {
        Navigation.toPaymentSuccess({
          orderId: String(orderId),
          orderNo,
          amount
        })
      } else {
        this.showPaymentCreateFailed(orderId)
      }
    } catch (paymentError) {
      logger.error('Payment creation failed', paymentError, 'Order-confirm')
      this.showPaymentCreateFailed(orderId)
    } finally {
      this.setData({ loading: false })
    }
  },

  async handleCombinedPayment(orderIds: number[]) {
    try {
      const combinedPayment = await createCombinedPaymentOrder({ order_ids: orderIds })
      const firstOrderId = combinedPayment.sub_orders?.[0]?.order_id || orderIds[0]
      const amount = (combinedPayment.total_amount / 100).toFixed(2)
      const orderNo = combinedPayment.combine_out_trade_no || String(firstOrderId)

      if (combinedPayment.pay_params) {
        try {
          await invokeWechatPay(combinedPayment.pay_params)
          Navigation.toPaymentSuccess({
            orderId: String(firstOrderId),
            orderNo,
            amount,
            isCombined: true,
            orderCount: orderIds.length
          })
        } catch {
          // 合单支付取消：引导用户前往订单列表逐单支付
          wx.showModal({
            title: '支付未完成',
            content: '订单已创建，可在订单列表逐单重新支付。',
            showCancel: false,
            confirmText: '查看订单',
            success: () => Navigation.redirectToOrderList()
          })
        }
      } else if (combinedPayment.status === 'paid') {
        Navigation.toPaymentSuccess({
          orderId: String(firstOrderId),
          orderNo,
          amount,
          isCombined: true,
          orderCount: orderIds.length
        })
      } else {
        wx.showModal({
          title: '订单已创建',
          content: '支付创建失败，请在订单列表逐单支付。',
          showCancel: false,
          success: () => Navigation.redirectToOrderList()
        })
      }
    } catch (paymentError) {
      logger.error('Combined payment creation failed', paymentError, 'Order-confirm')
      wx.showModal({
        title: '订单已创建',
        content: '支付创建失败，请在订单列表逐单支付。',
        showCancel: false,
        success: () => Navigation.redirectToOrderList()
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
   * 切换支付方式
   */
  onPaymentMethodChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ selectedPaymentMethod: e.detail.value })
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
    this.calculateDeliveryFee()
  }

})
