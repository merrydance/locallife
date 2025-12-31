import * as CartAPI from '@/api/cart'
import { UserCartsResponse, MerchantCartResponse, CartResponse } from '@/api/cart'
import { logger } from '@/utils/logger'
import { getPublicImageUrl } from '@/utils/image'

interface MerchantCartGroup {
  cartId: number
  merchantId: number
  merchantName: string
  merchantLogo: string
  items: CartItemView[]
  subtotal: number
  subtotalDisplay: string
  deliveryFee: number
  deliveryFeeDisplay: string
  totalAmount: number
  totalAmountDisplay: string
  itemCount: number
  allAvailable: boolean
  selected: boolean
}

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
  isAvailable: boolean
}

Page({
  data: {
    loading: true,
    merchantGroups: [] as MerchantCartGroup[],
    summary: {
      cartCount: 0,
      totalItems: 0,
      totalAmount: 0,
      totalAmountDisplay: '¥0.00'
    },
    navBarHeight: 88,

    // 结算相关
    selectedCartIds: [] as number[],
    checkoutTotal: 0,
    checkoutTotalDisplay: '¥0.00'
  },

  onLoad() {
    this.loadAllCarts()
  },

  onShow() {
    this.loadAllCarts()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  /**
   * 加载所有商户的购物车
   */
  async loadAllCarts() {
    try {
      this.setData({ loading: true })

      // 获取用户所有购物车汇总
      const userCarts = await CartAPI.getUserCarts()

      if (!userCarts.carts || userCarts.carts.length === 0) {
        this.setData({
          loading: false,
          merchantGroups: [],
          summary: {
            cartCount: 0,
            totalItems: 0,
            totalAmount: 0,
            totalAmountDisplay: '¥0.00'
          }
        })
        return
      }

      // 为每个商户获取详细购物车内容
      const merchantGroups: MerchantCartGroup[] = []

      for (const merchantCart of userCarts.carts) {
        if (!merchantCart.merchant_id) continue

        try {
          const cartDetail = await CartAPI.getCart(merchantCart.merchant_id)
          const group = this.buildMerchantGroup(merchantCart, cartDetail)
          merchantGroups.push(group)
        } catch (error) {
          logger.warn('Failed to load cart for merchant', { merchantId: merchantCart.merchant_id }, 'cart.loadAllCarts')
        }
      }

      // 默认全选
      const selectedCartIds = merchantGroups.map(g => g.cartId)

      this.setData({
        loading: false,
        merchantGroups,
        selectedCartIds,
        summary: {
          cartCount: userCarts.summary?.cart_count || merchantGroups.length,
          totalItems: userCarts.summary?.total_items || 0,
          totalAmount: userCarts.summary?.total_amount || 0,
          totalAmountDisplay: `¥${((userCarts.summary?.total_amount || 0) / 100).toFixed(2)}`
        }
      })

      // 计算各商户代取费
      await this.calculateDeliveryFees()
      this.calculateCheckoutTotal()
    } catch (error) {
      logger.error('Failed to load carts', error, 'cart.loadAllCarts')
      this.setData({ loading: false })
      wx.showToast({ title: '加载购物车失败', icon: 'none' })
    }
  },

  /**
   * 构建商户购物车组
   */
  buildMerchantGroup(merchantCart: MerchantCartResponse, cartDetail: CartResponse): MerchantCartGroup {
    const items: CartItemView[] = (cartDetail.items || []).map(item => ({
      id: item.id,
      dishId: item.dish_id,
      comboId: item.combo_id,
      name: item.name,
      imageUrl: getPublicImageUrl(item.image_url),
      quantity: item.quantity,
      unitPrice: item.unit_price,
      priceDisplay: `¥${(item.unit_price / 100).toFixed(2)}`,
      subtotal: item.subtotal,
      subtotalDisplay: `¥${(item.subtotal / 100).toFixed(2)}`,
      isAvailable: item.is_available
    }))

    const subtotal = cartDetail.subtotal || 0

    return {
      cartId: cartDetail.id,
      merchantId: merchantCart.merchant_id || 0,
      merchantName: merchantCart.merchant_name || '未知商户',
      merchantLogo: getPublicImageUrl(merchantCart.merchant_logo || ''),
      items,
      subtotal,
      subtotalDisplay: `¥${(subtotal / 100).toFixed(2)}`,
      deliveryFee: 0,  // 需要通过 calculateCart 获取
      deliveryFeeDisplay: '待计算',
      totalAmount: subtotal,
      totalAmountDisplay: `¥${(subtotal / 100).toFixed(2)}`,
      itemCount: items.reduce((sum, item) => sum + item.quantity, 0),
      allAvailable: items.every(item => item.isAvailable),
      selected: true
    }
  },

  /**
   * 计算各商户代取费
   * 获取用户当前地址并计算每个商户的代取费
   */
  async calculateDeliveryFees() {
    const { merchantGroups } = this.data
    if (merchantGroups.length === 0) return

    // 获取用户默认地址用于计算代取费
    const app = getApp()
    const addressId = app.globalData?.selectedAddressId || app.globalData?.defaultAddressId

    const updatedGroups = [...merchantGroups]

    for (let i = 0; i < updatedGroups.length; i++) {
      const group = updatedGroups[i]
      try {
        const result = await CartAPI.calculateCart({
          merchant_id: group.merchantId,
          address_id: addressId || undefined
        })

        // 更新代取费信息
        updatedGroups[i] = {
          ...group,
          deliveryFee: result.delivery_fee || 0,
          deliveryFeeDisplay: result.delivery_fee > 0
            ? `¥${(result.delivery_fee / 100).toFixed(2)}`
            : '免代取费',
          totalAmount: group.subtotal + (result.delivery_fee || 0),
          totalAmountDisplay: `¥${((group.subtotal + (result.delivery_fee || 0)) / 100).toFixed(2)}`
        }
      } catch (error) {
        logger.warn('Failed to calculate delivery fee', { merchantId: group.merchantId }, 'cart.calculateDeliveryFees')
        // 保持原有值，显示"待计算"
      }
    }

    this.setData({ merchantGroups: updatedGroups })
  },

  /**
   * 计算结算总价
   */
  calculateCheckoutTotal() {
    const { merchantGroups, selectedCartIds } = this.data

    let total = 0
    for (const group of merchantGroups) {
      if (selectedCartIds.includes(group.cartId)) {
        total += group.totalAmount
      }
    }

    this.setData({
      checkoutTotal: total,
      checkoutTotalDisplay: `¥${(total / 100).toFixed(2)}`
    })
  },

  /**
   * 切换商户选中状态
   */
  onToggleMerchant(e: WechatMiniprogram.CustomEvent) {
    const { cartId } = e.currentTarget.dataset
    const { selectedCartIds } = this.data

    const index = selectedCartIds.indexOf(cartId)
    if (index > -1) {
      selectedCartIds.splice(index, 1)
    } else {
      selectedCartIds.push(cartId)
    }

    this.setData({ selectedCartIds })
    this.calculateCheckoutTotal()
  },

  /**
   * 增加商品数量
   */
  async onIncrease(e: WechatMiniprogram.CustomEvent) {
    const { itemId } = e.currentTarget.dataset
    try {
      await CartAPI.updateCartItem(itemId, { quantity: this.getItemQuantity(itemId) + 1 })
      await this.loadAllCarts()
    } catch (error) {
      wx.showToast({ title: '更新失败', icon: 'none' })
    }
  },

  /**
   * 减少商品数量
   */
  async onDecrease(e: WechatMiniprogram.CustomEvent) {
    const { itemId } = e.currentTarget.dataset
    const quantity = this.getItemQuantity(itemId)

    try {
      if (quantity <= 1) {
        await CartAPI.removeFromCart(itemId)
      } else {
        await CartAPI.updateCartItem(itemId, { quantity: quantity - 1 })
      }
      await this.loadAllCarts()
    } catch (error) {
      wx.showToast({ title: '更新失败', icon: 'none' })
    }
  },

  /**
   * 获取商品当前数量
   */
  getItemQuantity(itemId: number): number {
    for (const group of this.data.merchantGroups) {
      const item = group.items.find(i => i.id === itemId)
      if (item) return item.quantity
    }
    return 1
  },

  /**
   * 清空某个商户的购物车
   */
  async onClearMerchant(e: WechatMiniprogram.CustomEvent) {
    const { merchantId } = e.currentTarget.dataset

    wx.showModal({
      title: '清空购物车',
      content: '确定要清空该商户的购物车吗?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await CartAPI.clearCart(merchantId)
            await this.loadAllCarts()
          } catch (error) {
            wx.showToast({ title: '清空失败', icon: 'none' })
          }
        }
      }
    })
  },

  /**
   * 去结算
   */
  onCheckout() {
    const { selectedCartIds, merchantGroups } = this.data

    if (selectedCartIds.length === 0) {
      wx.showToast({ title: '请选择要结算的商品', icon: 'none' })
      return
    }

    // 检查是否有不可用商品
    for (const group of merchantGroups) {
      if (selectedCartIds.includes(group.cartId) && !group.allAvailable) {
        wx.showToast({ title: '部分商品已下架，请移除后再结算', icon: 'none' })
        return
      }
    }

    // 跳转到订单确认页，传递选中的 cart_ids
    wx.navigateTo({
      url: `/pages/takeout/order-confirm/index?cart_ids=${selectedCartIds.join(',')}`
    })
  },

  /**
   * 返回外卖页
   */
  onBackToTakeout() {
    wx.navigateBack()
  }
})
