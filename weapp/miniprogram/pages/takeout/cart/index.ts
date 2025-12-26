import CartService from '@/services/cart'
import { CartItem } from '@/models/cart'

Page({
  data: {
    items: [] as CartItem[],
    totalCount: 0,
    totalPrice: 0,
    totalPriceDisplay: '$0.00',
    navBarHeight: 88
  },

  onLoad() {
    this.loadCart()
  },

  onShow() {
    this.loadCart()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  loadCart() {
    const cart = CartService.getCart()
    this.setData({
      items: cart.items,
      totalCount: cart.totalCount,
      totalPrice: cart.totalPrice,
      totalPriceDisplay: cart.totalPriceDisplay
    })
  },

  onIncrease(e: WechatMiniprogram.CustomEvent) {
    const { dishId } = e.currentTarget.dataset
    const item = this.data.items.find((i) => i.dishId === dishId)
    if (item) {
      CartService.updateQuantity(dishId, item.quantity + 1)
      this.loadCart()
    }
  },

  onDecrease(e: WechatMiniprogram.CustomEvent) {
    const { dishId } = e.currentTarget.dataset
    const item = this.data.items.find((i) => i.dishId === dishId)
    if (item) {
      CartService.updateQuantity(dishId, item.quantity - 1)
      this.loadCart()
    }
  },

  onClearAll() {
    wx.showModal({
      title: '清空购物车',
      content: '确定要清空购物车吗?',
      success: (res) => {
        if (res.confirm) {
          CartService.clear()
          this.loadCart()
        }
      }
    })
  },

  onCheckout() {
    if (this.data.totalCount === 0) {
      wx.showToast({ title: '购物车为空', icon: 'none' })
      return
    }

    wx.navigateTo({ url: '/pages/takeout/order-confirm/index' })
  },

  onBackToTakeout() {
    wx.navigateBack()
  }
})
