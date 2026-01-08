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
      const userCarts = await CartAPI.getUserCarts('takeout')

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
        // 重要：空购物车也要同步到全局状态
        this.syncToGlobalStore()
        return
      }

      // 为每个商户获取详细购物车内容
      const merchantGroups: MerchantCartGroup[] = []

      for (const merchantCart of userCarts.carts) {
        if (!merchantCart.merchant_id) continue

        try {
          const cartDetail = await CartAPI.getCart({
            merchant_id: merchantCart.merchant_id,
            order_type: merchantCart.order_type,
            table_id: merchantCart.table_id || 0,
            reservation_id: merchantCart.reservation_id || 0
          })
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

      // 同步到全局状态，让其他页面（如外卖首页）能感知购物车变化
      this.syncToGlobalStore()
    } catch (error) {
      logger.error('Failed to load carts', error, 'cart.loadAllCarts')
      this.setData({ loading: false })
      wx.showToast({ title: '加载购物车失败', icon: 'none' })
    }
  },

  /**
   * 同步购物车状态到全局存储
   */
  syncToGlobalStore() {
    const { globalStore } = require('@/utils/global-store')
    const { summary, merchantGroups } = this.data

    // 计算总数量
    const totalCount = merchantGroups.reduce((sum, group) => sum + group.itemCount, 0)

    globalStore.set('cart', {
      items: [],  // 多商户模式下不使用单一 items 列表
      totalCount: totalCount,
      totalPrice: summary.totalAmount,
      totalPriceDisplay: summary.totalAmountDisplay
    })

    logger.debug('Cart synced to globalStore', { totalCount }, 'cart.syncToGlobalStore')
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

    // 获取用户地址或当前位置用于计算代取费
    const app = getApp()
    const addressId = app.globalData?.selectedAddressId || app.globalData?.defaultAddressId
    const latitude = app.globalData?.latitude
    const longitude = app.globalData?.longitude

    const updatedGroups = [...merchantGroups]

    for (let i = 0; i < updatedGroups.length; i++) {
      const group = updatedGroups[i]
      try {
        // 优先使用 address_id，fallback 到当前位置坐标
        const result = await CartAPI.calculateCart({
          merchant_id: group.merchantId,
          address_id: addressId || undefined,
          latitude: !addressId && latitude ? latitude : undefined,
          longitude: !addressId && longitude ? longitude : undefined
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
    const { selectedCartIds, merchantGroups } = this.data

    const index = selectedCartIds.indexOf(cartId)
    if (index > -1) {
      selectedCartIds.splice(index, 1)
    } else {
      selectedCartIds.push(cartId)
    }

    // 同时更新 merchantGroups 中对应项的 selected 状态
    const updatedGroups = merchantGroups.map(group => ({
      ...group,
      selected: selectedCartIds.includes(group.cartId)
    }))

    this.setData({
      selectedCartIds,
      merchantGroups: updatedGroups
    })
    this.calculateCheckoutTotal()
  },

  /**
   * 增加商品数量
   */
  async onIncrease(e: WechatMiniprogram.CustomEvent) {
    const { itemId } = e.currentTarget.dataset
    const currentQuantity = this.getItemQuantity(itemId)
    const newQuantity = currentQuantity + 1

    // 先更新本地状态（乐观更新）
    this.updateLocalQuantity(itemId, newQuantity)

    try {
      await CartAPI.updateCartItem(itemId, { quantity: newQuantity })
      // 更新小计和总价
      this.recalculateSubtotals()
      // 重新计算代取费（后端根据订单金额计算）
      this.calculateDeliveryFees()
    } catch (error) {
      // 回滚本地状态
      this.updateLocalQuantity(itemId, currentQuantity)
      wx.showToast({ title: '更新失败', icon: 'none' })
    }
  },

  /**
   * 减少商品数量
   */
  async onDecrease(e: WechatMiniprogram.CustomEvent) {
    const { itemId } = e.currentTarget.dataset
    const currentQuantity = this.getItemQuantity(itemId)

    if (currentQuantity <= 1) {
      // 删除商品需要重新加载列表
      try {
        await CartAPI.removeFromCart(itemId)
        await this.loadAllCarts()
      } catch (error) {
        wx.showToast({ title: '删除失败', icon: 'none' })
      }
      return
    }

    const newQuantity = currentQuantity - 1

    // 乐观更新
    this.updateLocalQuantity(itemId, newQuantity)

    try {
      await CartAPI.updateCartItem(itemId, { quantity: newQuantity })
      this.recalculateSubtotals()
      // 重新计算代取费
      this.calculateDeliveryFees()
    } catch (error) {
      // 回滚
      this.updateLocalQuantity(itemId, currentQuantity)
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
   * 本地更新商品数量（乐观更新）
   */
  updateLocalQuantity(itemId: number, newQuantity: number) {
    const { merchantGroups } = this.data

    for (let i = 0; i < merchantGroups.length; i++) {
      const itemIndex = merchantGroups[i].items.findIndex(item => item.id === itemId)
      if (itemIndex !== -1) {
        // 使用路径更新避免重新渲染整个列表
        this.setData({
          [`merchantGroups[${i}].items[${itemIndex}].quantity`]: newQuantity
        })
        return
      }
    }
  },

  /**
   * 重新计算各商户小计和总计
   */
  recalculateSubtotals() {
    const { merchantGroups } = this.data

    // 批量更新对象
    const updates: Record<string, unknown> = {}

    for (let i = 0; i < merchantGroups.length; i++) {
      const group = merchantGroups[i]
      const subtotal = group.items.reduce((sum, item) => {
        return sum + (item.unitPrice * item.quantity)
      }, 0)

      updates[`merchantGroups[${i}].subtotal`] = subtotal
      updates[`merchantGroups[${i}].subtotalDisplay`] = `¥${(subtotal / 100).toFixed(2)}`
    }

    // 一次性更新所有值
    this.setData(updates)

    // 重新计算结算总价
    this.calculateCheckoutTotal()

    // 同步到全局 store
    this.syncToGlobalStore()
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

            // 本地移除该商户分组，避免重新加载整个页面
            const { merchantGroups, selectedCartIds } = this.data
            const groupIndex = merchantGroups.findIndex(g => g.merchantId === merchantId)

            if (groupIndex !== -1) {
              const removedGroup = merchantGroups[groupIndex]
              const newGroups = merchantGroups.filter((_, i) => i !== groupIndex)
              const newSelectedIds = selectedCartIds.filter(id => id !== removedGroup.cartId)

              this.setData({
                merchantGroups: newGroups,
                selectedCartIds: newSelectedIds
              })

              // 重新计算总价并同步
              this.calculateCheckoutTotal()
              this.syncToGlobalStore()
            }
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
