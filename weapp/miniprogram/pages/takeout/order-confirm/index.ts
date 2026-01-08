import * as CartAPI from '../../../api/cart'
import { CartItemResponse } from '../../../api/cart'
import { logger } from '../../../utils/logger'
import AddressService, { Address } from '../../../api/address'
import { createOrder, CreateOrderRequest } from '../../../api/order'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'
import { request } from '../../../utils/request'

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
}

interface CartData {
  items: CartItemView[]
  merchantId: number
  merchantName: string
  totalCount: number
  totalPrice: number
  totalPriceDisplay: string
}

Page({
  data: {
    cart: null as CartData | null,
    cartIds: [] as number[],
    address: null as Address | null,
    remark: '',
    deliveryTime: 'ASAP',
    navBarHeight: 88,
    loading: false,
    orderTotalDisplay: '0.00',
    deliveryFee: 500,  // 配送费（分）
    deliveryFeeDisplay: '5.00'
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
    const currPage = pages[pages.length - 1]
    if ((currPage as any).data.selectedAddressId) {
      this.loadAddressById((currPage as any).data.selectedAddressId)
        ; (currPage as any).setData({ selectedAddressId: null })
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

      // 目前只支持单商户结算
      const merchantCart = selectedCarts[0]
      const merchantId = merchantCart.merchant_id

      if (!merchantId) {
        wx.showToast({ title: '商户信息缺失', icon: 'none' })
        setTimeout(() => wx.navigateBack(), 1500)
        return
      }

      // Step 2: 获取购物车商品详情
      const cartDetail = await CartAPI.getCart({
        merchant_id: merchantId,
        order_type: 'takeout'
      })

      if (!cartDetail.items || cartDetail.items.length === 0) {
        wx.showToast({ title: '购物车为空', icon: 'none' })
        setTimeout(() => wx.navigateBack(), 1500)
        return
      }

      // 转换为页面数据格式
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
        subtotalDisplay: formatPriceNoSymbol(item.subtotal)
      }))

      const totalCount = items.reduce((sum, item) => sum + item.quantity, 0)
      const totalPrice = cartDetail.subtotal

      const cart: CartData = {
        items,
        merchantId,
        merchantName: merchantCart.merchant_name || '商家',
        totalCount,
        totalPrice,
        totalPriceDisplay: formatPriceNoSymbol(totalPrice)
      }

      // 计算订单总价（商品金额 + 配送费）
      const { deliveryFee } = this.data
      const orderTotal = totalPrice + deliveryFee

      this.setData({
        cart,
        orderTotalDisplay: formatPriceNoSymbol(orderTotal),
        loading: false
      })
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
      }
    } catch (error) {
      logger.error('Load address failed', error, 'Order-confirm')
    }
  },

  onSelectAddress() {
    wx.navigateTo({ url: '/pages/user_center/addresses/index?select=true' })
  },

  onRemarkInput(e: WechatMiniprogram.CustomEvent) {
    this.setData({ remark: e.detail.value })
  },

  onDeliveryTimeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ deliveryTime: e.detail.value })
  },

  async onSubmitOrder() {
    const { cart, address, remark } = this.data

    if (!address || !address.id) {
      wx.showToast({ title: '请选择收货地址', icon: 'none' })
      return
    }

    if (!cart || cart.totalCount === 0) {
      wx.showToast({ title: '购物车为空', icon: 'none' })
      return
    }

    if (!cart.merchantId) {
      wx.showToast({ title: '商户信息丢失', icon: 'none' })
      return
    }

    this.setData({ loading: true })

    try {
      // Step 1: 创建订单
      const requestData: CreateOrderRequest = {
        merchant_id: cart.merchantId,
        items: cart.items.map((item) => {
          const orderItem: { dish_id?: number; combo_id?: number; quantity: number } = {
            quantity: item.quantity
          }
          if (item.dishId) {
            orderItem.dish_id = item.dishId
          }
          if (item.comboId) {
            orderItem.combo_id = item.comboId
          }
          return orderItem
        }),
        order_type: 'takeout',
        address_id: address.id,
        notes: remark
      }

      const order = await createOrder(requestData)
      console.log('[Order-confirm] Order created:', order.id)

      // Step 2: 创建支付订单
      try {
        // 调用后端 /v1/payments API (对应 createPaymentOrder)
        const paymentResult = await request({
          url: '/v1/payments',
          method: 'POST',
          data: {
            order_id: order.id,
            payment_type: 'miniprogram',  // 小程序支付
            business_type: 'order'         // 订单支付
          }
        }) as { pay_params?: { timeStamp: string; nonceStr: string; package: string; signType: string; paySign: string } }

        console.log('[Order-confirm] Payment created:', paymentResult)

        // Step 3: 检查是否返回了支付参数 (后端返回 pay_params)
        if (paymentResult.pay_params) {
          // 调用微信支付
          const params = paymentResult.pay_params
          wx.requestPayment({
            timeStamp: params.timeStamp,
            nonceStr: params.nonceStr,
            package: params.package,
            signType: (params.signType || 'RSA') as 'RSA' | 'MD5' | 'HMAC-SHA256',
            paySign: params.paySign,
            success: () => {
              wx.showToast({ title: '支付成功', icon: 'success' })
              setTimeout(() => {
                wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` })
              }, 1500)
            },
            fail: (err) => {
              console.log('[Order-confirm] Payment cancelled or failed:', err)
              wx.showToast({ title: '支付取消', icon: 'none' })
              // 支付取消/失败，跳转到订单详情（状态为待支付）
              setTimeout(() => {
                wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` })
              }, 1500)
            }
          })
        } else {
          // 支付参数未返回（可能是后端未配置微信支付）
          this.showPaymentDevModal(order.id)
        }
      } catch (paymentError) {
        console.error('[Order-confirm] Payment creation failed:', paymentError)
        // 支付订单创建失败，提示开发中
        this.showPaymentDevModal(order.id)
      }
    } catch (error) {
      logger.error('Create order failed:', error, 'Order-confirm')
      wx.showToast({ title: '下单失败', icon: 'error' })
      this.setData({ loading: false })
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
