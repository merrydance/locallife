import * as CartAPI from '../../../api/cart'
import { CartItemResponse } from '../../../api/cart'
import { logger } from '../../../utils/logger'
import AddressService, { Address } from '../../../api/address'
import { createOrder, CreateOrderRequest, OrderType } from '../../../api/order'
import { createOrderPayment, invokeWechatPay } from '../../../api/payment'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'

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
  customizations?: Record<string, unknown>
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
}

Page({
  data: {
    carts: [] as MerchantCartView[],
    cartIds: [] as number[],
    address: null as Address | null,
    remarks: {} as Record<number, string>,
    navBarHeight: 88,
    loading: false,
    orderTotalDisplay: '0.00',
    summarySubtotalDisplay: '0.00',
    summaryDeliveryDisplay: '待计算'
  },

  onLoad(options: { cart_ids?: string }) {
    // 解析URL中的cart_ids参数
    if (options.cart_ids) {
      const cartIds = options.cart_ids.split(',').map(Number).filter(id => !isNaN(id))
      this.setData({ cartIds })
    }
    this.loadCart()
    this.loadDefaultAddress()
  },

  onShow() {
    // If returning from address selection, we might have a selectedAddressId
    const pages = getCurrentPages()
    const currPage = pages[pages.length - 1] as WechatMiniprogram.Page.Instance & {
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
      this.setData({ loading: true })
      console.log('[Order-confirm] Loading takeout cart...')

      // Step 1: 获取外卖类型的购物车列表
      const userCarts = await CartAPI.getUserCarts('takeout')
      console.log('[Order-confirm] userCarts:', JSON.stringify(userCarts))

      if (!userCarts.carts || userCarts.carts.length === 0) {
        console.log('[Order-confirm] No carts found, navigating back')
        wx.showToast({ title: '购物车为空', icon: 'none' })
        setTimeout(() => wx.navigateBack(), 1500)
        return
      }

      // 如果有指定的cart_ids，只使用这些购物车
      const { cartIds } = this.data
      let selectedCarts = userCarts.carts
      if (cartIds.length > 0) {
        selectedCarts = userCarts.carts.filter(c => cartIds.includes(c.cart_id || 0))
      }

      if (selectedCarts.length === 0) {
        // 如果没有匹配的cart_ids，使用第一个购物车
        selectedCarts = [userCarts.carts[0]]
      }

      // 逐商户拉取购物车详情
      const cartViews: MerchantCartView[] = []
      for (const merchantCart of selectedCarts) {
        const merchantId = merchantCart.merchant_id
        const orderType = (merchantCart.order_type || 'takeout') as OrderType

        if (!merchantId) {
          wx.showToast({ title: '商户信息缺失', icon: 'none' })
          setTimeout(() => wx.navigateBack(), 1500)
          return
        }

        const cartDetail = await CartAPI.getCart({
          merchant_id: merchantId,
          order_type: orderType,
          table_id: merchantCart.table_id || undefined,
          reservation_id: merchantCart.reservation_id || undefined
        })

        if (!cartDetail.items || cartDetail.items.length === 0) {
          continue
        }

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
          customizations: item.customizations || undefined
        }))

        const totalCount = items.reduce((sum, item) => sum + item.quantity, 0)
        const subtotal = cartDetail.subtotal

        cartViews.push({
          merchantId,
          merchantName: merchantCart.merchant_name || '商家',
          orderType,
          tableId: merchantCart.table_id || undefined,
          reservationId: merchantCart.reservation_id || undefined,
          items,
          totalCount,
          subtotal,
          subtotalDisplay: formatPriceNoSymbol(subtotal),
          deliveryFee: 0,
          deliveryFeeDisplay: '待计算',
          deliveryFeeDiscount: 0,
          deliveryDistance: 0,
          deliveryEtaMinutes: 0,
          deliveryEtaDisplay: '',
          orderTotal: subtotal,
          orderTotalDisplay: formatPriceNoSymbol(subtotal)
        })
      }

      if (cartViews.length === 0) {
        wx.showToast({ title: '购物车为空', icon: 'none' })
        setTimeout(() => wx.navigateBack(), 1500)
        return
      }

      this.setData({ carts: cartViews, loading: false })

      // 根据地址计算每个商户配送费
      await this.calculateDeliveryFee()
    } catch (error) {
      logger.error('Load cart failed', error, 'Order-confirm')
      wx.showToast({ title: '加载购物车失败', icon: 'error' })
      this.setData({ loading: false })
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
    wx.navigateTo({ url: '/pages/user_center/addresses/index?select=true' })
  },

  onRemarkInput(e: WechatMiniprogram.CustomEvent) {
    const { merchantId } = e.currentTarget.dataset as { merchantId?: number | string }
    const merchantIdNum = Number(merchantId)
    if (!merchantIdNum) return
    const remarks = { ...this.data.remarks, [merchantIdNum]: e.detail.value }
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
  async calculateDeliveryFee() {
    const { carts, address } = this.data
    if (!carts || carts.length === 0) return

    if (!address) {
      this.setData({
        summaryDeliveryDisplay: '待选择地址',
        orderTotalDisplay: formatPriceNoSymbol(carts.reduce((s, c) => s + (c.subtotal || 0), 0))
      })
      return
    }

    try {
      const updated = [] as MerchantCartView[]

      for (const cart of carts) {
        const result = await CartAPI.calculateCart({
          merchant_id: cart.merchantId,
          order_type: cart.orderType,
          address_id: address.id,
          latitude: address.latitude ? Number(address.latitude) : undefined,
          longitude: address.longitude ? Number(address.longitude) : undefined
        })

        const deliveryFee = result.delivery_fee || 0
        const deliveryFeeDiscount = result.delivery_fee_discount || 0
        const deliveryDistance = result.delivery_distance || 0
        const orderTotal = (cart.subtotal || 0) + deliveryFee
        const deliveryEtaMinutes = result.delivery_eta_minutes || 0
        const deliveryEtaDisplay = this.formatEtaWindow(deliveryEtaMinutes)

        updated.push({
          ...cart,
          deliveryFee,
          deliveryFeeDisplay: deliveryFee > 0 ? '¥' + formatPriceNoSymbol(deliveryFee) : '免配送费',
          deliveryFeeDiscount,
          deliveryDistance,
          orderTotal,
          orderTotalDisplay: formatPriceNoSymbol(orderTotal),
          deliveryEtaMinutes,
          deliveryEtaDisplay
        })
      }

      const summarySubtotal = updated.reduce((sum, c) => sum + (c.subtotal || 0), 0)
      const summaryDelivery = updated.reduce((sum, c) => sum + (c.deliveryFee || 0), 0)
      const orderTotal = summarySubtotal + summaryDelivery

      this.setData({
        carts: updated,
        summarySubtotalDisplay: formatPriceNoSymbol(summarySubtotal),
        summaryDeliveryDisplay: summaryDelivery > 0 ? '¥' + formatPriceNoSymbol(summaryDelivery) : '免配送费',
        orderTotalDisplay: formatPriceNoSymbol(orderTotal)
      })
    } catch (error) {
      logger.error('Calculate delivery fee failed', error, 'Order-confirm')
      const errMessage = error instanceof Error ? error.message : String(error)
      wx.showModal({
        title: '调试',
        content: `计算运费失败: ${errMessage || '未知错误'}`,
        showCancel: false
      })
      // 保留现有金额显示，不打断流程
    }
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

  async onSubmitOrder() {
    const { carts, address, remarks } = this.data

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
            const orderItem: { dish_id?: number; combo_id?: number; quantity: number; customizations?: Record<string, unknown> } = {
              quantity: item.quantity
            }
            if (item.dishId) orderItem.dish_id = item.dishId
            if (item.comboId) orderItem.combo_id = item.comboId
            if (item.customizations) orderItem.customizations = item.customizations
            return orderItem
          }),
          order_type: cart.orderType,
          address_id: address.id,
          notes: remarks[cart.merchantId] || '',
          delivery_fee: cart.deliveryFee,
          delivery_fee_discount: cart.deliveryFeeDiscount,
          delivery_distance: cart.deliveryDistance
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
            showDebugModal('清理购物车失败（已创建订单）', clearErr)
          }
        }
      }

      if (ordersCreated.length === 1) {
        await this.handlePayment(ordersCreated[0])
      } else {
        this.setData({ loading: false })
        wx.showModal({
          title: '订单已创建',
          content: `已创建 ${ordersCreated.length} 个订单，当前不支持合单支付，请在订单列表逐单支付。`,
          showCancel: false,
          success: () => wx.redirectTo({ url: '/pages/orders/list/index' })
        })
      }
    } catch (error) {
      logger.error('Create order failed:', error, 'Order-confirm')
      wx.showToast({ title: '下单失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  async handlePayment(orderId: number) {
    try {
      const paymentResult = await createOrderPayment(orderId)

      if (paymentResult.pay_params) {
        try {
          await invokeWechatPay(paymentResult.pay_params)
          wx.showToast({ title: '支付成功', icon: 'success' })
        } catch (err) {
          console.log('[Order-confirm] Payment cancelled or failed:', err)
          wx.showToast({ title: '支付取消', icon: 'none' })
        } finally {
          setTimeout(() => {
            wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })
          }, 1500)
        }
      } else {
        this.showPaymentDevModal(orderId)
      }
    } catch (paymentError) {
      console.error('[Order-confirm] Payment creation failed:', paymentError)
      this.showPaymentDevModal(orderId)
    }
  },

  showPaymentDevModal(orderId: number) {
    this.setData({ loading: false })
    wx.showModal({
      title: '支付功能开发中',
      content: '微信支付功能正在开发中，订单已创建成功。',
      showCancel: false,
      confirmText: '查看订单',
      success: () => {
        wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })
      }
    })
  }
})
