import CartService from '../../../services/cart'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'
import { getAddressList, AddressDTO } from '../../../api/address'
import { createOrder, previewOrder, CreateOrderRequest } from '../../../api/order'
import { Cart } from '../../../models/cart'
import { formatPriceNoSymbol } from '../../../utils/util'

interface PreviewData {
  subtotal: number
  deliveryFee: number
  discount: number
  total: number
}

Page({
  data: {
    cart: null as Cart | null,
    address: null as AddressDTO | null,
    remark: '',
    deliveryTime: 'ASAP',
    navBarHeight: 88,
    loading: false,
    previewData: null as PreviewData | null,
    orderTotalDisplay: '0.00',
    deliveryFeeDisplay: '5.00'
  },

  onLoad() {
    this.loadCart()
    this.loadDefaultAddress()
  },

  onShow() {
    // If returning from address selection, we might have a selectedAddressId
    const pages = getCurrentPages()
    const currPage = pages[pages.length - 1]
    if (currPage.data.selectedAddressId) {
      this.loadAddressById(currPage.data.selectedAddressId)
      // clear it
      currPage.setData({ selectedAddressId: null })
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  loadCart() {
    const cart = CartService.getCart()
    // 计算订单总价（商品金额 + 配送费 500分）
    const orderTotal = (cart?.totalPrice || 0) + 500
    this.setData({
      cart,
      orderTotalDisplay: formatPriceNoSymbol(orderTotal)
    })
    this.updateOrderPreview()
  },

  async loadDefaultAddress() {
    try {
      const addresses = await getAddressList()
      if (addresses && addresses.length > 0) {
        const defaultAddr = addresses.find((a) => a.is_default) || addresses[0]
        this.setData({ address: defaultAddr })
        this.updateOrderPreview()
      }
    } catch (error) {
      logger.error('Load address failed', error, 'Order-confirm')
    }
  },

  async loadAddressById(id: string) {
    try {
      const addresses = await getAddressList() // Ideally use getAddressDetail or find from cached list
      const addr = addresses.find((a) => a.id === id)
      if (addr) {
        this.setData({ address: addr })
        this.updateOrderPreview()
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

  async updateOrderPreview() {
    const { cart, address } = this.data
    const CartService = require('../../../services/cart').default
    const merchantId = CartService.getMerchantId()

    if (!cart || cart.totalCount === 0 || !merchantId) return

    const requestData: CreateOrderRequest = {
      merchant_id: merchantId,
      items: cart.items.map((item: any) => ({
        dish_id: item.dishId,
        quantity: item.quantity,
        extra_options: []
      })),
      order_type: 'DELIVERY',
      address_id: address?.id
    }

    try {
      const preview = await previewOrder(requestData)
      this.setData({ previewData: preview })
    } catch (e) {
      logger.error('Preview failed', e, 'Order-confirm')
    }
  },

  async onSubmitOrder() {
    const { cart, address, remark } = this.data
    const CartService = require('../../../services/cart').default
    const merchantId = CartService.getMerchantId()

    if (!address) {
      wx.showToast({ title: '请选择收货地址', icon: 'none' })
      return
    }

    if (cart.totalCount === 0) {
      wx.showToast({ title: '购物车为空', icon: 'none' })
      return
    }

    if (!merchantId) {
      wx.showToast({ title: '商户信息丢失', icon: 'none' })
      return
    }

    this.setData({ loading: true })

    try {
      // Construct Request
      const requestData: CreateOrderRequest = {
        merchant_id: merchantId,
        items: cart.items.map((item: any) => ({
          dish_id: item.dishId,
          quantity: item.quantity,
          extra_options: []
        })),
        order_type: 'DELIVERY',
        address_id: address.id,
        comment: remark
      }

      const order = await createOrder(requestData)

      wx.showToast({ title: '下单成功', icon: 'success' })

      // 清空购物车
      CartService.clear()

      // 跳转到订单详情
      setTimeout(() => {
        wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` })
      }, 1500)
    } catch (error) {
      logger.error('Create order failed:', error, 'Order-confirm')
      wx.showToast({ title: '下单失败', icon: 'error' })
      this.setData({ loading: false })
    }
  }
})
