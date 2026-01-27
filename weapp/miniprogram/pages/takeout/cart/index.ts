import * as CartAPI from '@/api/cart'
import { UserCartsResponse, MerchantCartResponse, CartResponse } from '@/api/cart'
import { DishManagementService } from '@/api/dish'
import { logger } from '@/utils/logger'
import { getPublicImageUrl } from '@/utils/image'

// ... existing imports

// ... existing imports

interface MerchantCartGroup {
  cartId: number
  merchantId: number
  orderType?: string
  tableId?: number
  reservationId?: number
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
  errorStatus?: string // 商户错误状态：如“已打烊”、“无法配送”
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
  specDisplay?: string // 规格展示
  customizations?: Record<string, unknown> // 原始定制选项，用于前端解析
  dishImages?: string[] // 新增：套餐内的菜品图片
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
      let merchantGroups: MerchantCartGroup[] = []


      for (const merchantCart of userCarts.carts) {
        if (!merchantCart.merchant_id) continue

        try {
          const cartDetail = await CartAPI.getCart({
            merchant_id: merchantCart.merchant_id,
            order_type: merchantCart.order_type || 'takeout',
            table_id: merchantCart.table_id ?? undefined,
            reservation_id: merchantCart.reservation_id ?? undefined
          })
          const group = this.buildMerchantGroup(merchantCart, cartDetail)
          merchantGroups.push(group)
        } catch (error) {
          logger.warn('Failed to load cart for merchant', { merchantId: merchantCart.merchant_id }, 'cart.loadAllCarts')
        }
      }

      // 预先计算费用并校验商户状态（在显示前完成）
      merchantGroups = await this.calculateDeliveryFees(merchantGroups, true)

      // 默认全选（排除有错误的商户）
      const selectedCartIds = merchantGroups
        .filter(g => !g.errorStatus)
        .map(g => g.cartId)

      // 设置数据并显示
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
    const items: CartItemView[] = (cartDetail.items || []).map(item => {
      let specDisplay = item.spec_text || ''

      return {
        id: item.id,
        dishId: item.dish_id,
        comboId: item.combo_id,
        name: item.name,
        imageUrl: getPublicImageUrl(item.image_url || ''),
        quantity: item.quantity,
        unitPrice: item.unit_price,
        priceDisplay: `¥${(item.unit_price / 100).toFixed(2)}`,
        subtotal: item.subtotal,
        subtotalDisplay: `¥${(item.subtotal / 100).toFixed(2)}`,
        isAvailable: item.is_available,
        specDisplay,
        customizations: item.customizations,
        dishImages: item.combo_member_images?.map(url => getPublicImageUrl(url)) || []
      }
    })

    const subtotal = cartDetail.subtotal || 0
    const orderType = cartDetail.order_type || merchantCart.order_type || 'takeout'

    return {
      cartId: cartDetail.id,
      merchantId: merchantCart.merchant_id || 0,
      orderType,
      tableId: cartDetail.table_id ?? merchantCart.table_id,
      reservationId: cartDetail.reservation_id ?? merchantCart.reservation_id,
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
      selected: true // 初始设为true，calculateDeliveryFees 后可能会改为 false
    }
  },

  /**
   * 计算各商户代取费
   * 获取用户当前地址并计算每个商户的代取费
   * @param groups 可选，如果传入则计算该列表，否则使用 this.data.merchantGroups
   * @returns 更新后的 merchantGroups
   */
  async calculateDeliveryFees(groups?: MerchantCartGroup[], silent: boolean = false): Promise<MerchantCartGroup[]> {
    const merchantGroups = groups || this.data.merchantGroups
    if (merchantGroups.length === 0) return []

    // 获取用户地址或当前位置用于计算代取费
    const app = getApp()
    const addressId = app.globalData?.selectedAddressId || app.globalData?.defaultAddressId
    const latitude = app.globalData?.latitude
    const longitude = app.globalData?.longitude

    const updatedGroups = [...merchantGroups]
    let hasChanges = false

    // 并行计算以提高速度
    await Promise.all(updatedGroups.map(async (group, i) => {
      try {
        // 优先使用 address_id，fallback 到当前位置坐标
        const result = await CartAPI.calculateCart({
          merchant_id: group.merchantId,
          order_type: group.orderType,
          table_id: group.tableId,
          reservation_id: group.reservationId,
          address_id: addressId || undefined,
          latitude: !addressId && latitude ? latitude : undefined,
          longitude: !addressId && longitude ? longitude : undefined
        }, { loading: !silent })

        // 更新代取费信息
        updatedGroups[i] = {
          ...group,
          deliveryFee: result.delivery_fee || 0,
          deliveryFeeDisplay: result.delivery_fee > 0
            ? `¥${(result.delivery_fee / 100).toFixed(2)}`
            : '免代取费',
          totalAmount: group.subtotal + (result.delivery_fee || 0),
          totalAmountDisplay: `¥${((group.subtotal + (result.delivery_fee || 0)) / 100).toFixed(2)}`,
          errorStatus: '', // 清除错误状态
          // 既然计算成功，如果是之前因错误导致的selected=false，是否要恢复？
          // 保守起见，保持当前selected状态，除非它之前是错的但用户本意是选中
          // 这里简化：只有在出错时才强制selected=false
        }
      } catch (error: any) {
        logger.warn('Failed to calculate delivery fee', { merchantId: group.merchantId }, 'cart.calculateDeliveryFees')
        
        // 捕获错误并显示给用户（如商户打烊、超出范围）
        updatedGroups[i] = {
          ...group,
          errorStatus: error.userMessage || '暂不支持配送',
          selected: false // 有错误时自动取消选中
        }
        hasChanges = true
      }
    }))

    // 如果是基于 this.data 进行的更新，则需要 setData
    if (!groups) {
      this.setData({ merchantGroups: updatedGroups })
      
      // 检查 selectedCartIds 是否需要更新
      const { selectedCartIds } = this.data
      const newSelectedIds = updatedGroups.filter(g => g.selected && selectedCartIds.includes(g.cartId)).map(g => g.cartId)
      
      // 如果有原来选中的现在因为错误变为了不选中
      if (newSelectedIds.length !== selectedCartIds.length) {
         // 注意：这里的简单比较可能不够，但通常足够处理 "选中->不选中" 的情况
         // 更严谨：newSelectedIds 应该是 (OldSelected intersect ValidGroups)
         // 上面的 filter 已经做了这件事：g.selected 被置为 false 了如果出错
         const validSelectedIds = selectedCartIds.filter(id => {
           const g = updatedGroups.find(group => group.cartId === id)
           return g && !g.errorStatus
         })
         
         if (validSelectedIds.length !== selectedCartIds.length) {
            this.setData({ selectedCartIds: validSelectedIds })
            this.calculateCheckoutTotal()
         }
      }
    }

    return updatedGroups
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

    const group = merchantGroups.find(g => g.cartId === cartId)
    // 阻止有错误的商户被选中
    if (group?.errorStatus) {
      wx.showToast({ title: group.errorStatus, icon: 'none' })
      return
    }

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
    if (!itemId) return
    const currentQuantity = this.getItemQuantity(itemId)
    const newQuantity = currentQuantity + 1

    // 先更新本地状态（乐观更新）
    this.updateLocalQuantity(itemId, newQuantity)

    try {
      await CartAPI.updateCartItem(itemId, { quantity: newQuantity }, { loading: false })
      // 更新小计和总价
      this.recalculateSubtotals()
      // 重新计算代取费（后端根据订单金额计算）
      this.calculateDeliveryFees(undefined, true)
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
    if (!itemId) return
    const currentQuantity = this.getItemQuantity(itemId)

    if (currentQuantity <= 1) {
      // 删除商品，本地移除避免整页刷新
      try {
        await CartAPI.removeFromCart(itemId, { loading: false })
        this.removeLocalItem(itemId)
      } catch (error) {
        wx.showToast({ title: '删除失败', icon: 'none' })
      }
      return
    }

    const newQuantity = currentQuantity - 1

    // 乐观更新
    this.updateLocalQuantity(itemId, newQuantity)

    try {
      await CartAPI.updateCartItem(itemId, { quantity: newQuantity }, { loading: false })
      this.recalculateSubtotals()
      // 重新计算代取费
      this.calculateDeliveryFees(undefined, true)
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
      const itemCount = group.items.reduce((sum, item) => sum + item.quantity, 0)
      const deliveryFee = group.deliveryFee || 0
      const totalAmount = subtotal + deliveryFee

      updates[`merchantGroups[${i}].subtotal`] = subtotal
      updates[`merchantGroups[${i}].subtotalDisplay`] = `¥${(subtotal / 100).toFixed(2)}`
      updates[`merchantGroups[${i}].itemCount`] = itemCount
      updates[`merchantGroups[${i}].totalAmount`] = totalAmount
      updates[`merchantGroups[${i}].totalAmountDisplay`] = `¥${(totalAmount / 100).toFixed(2)}`
    }

    // 一次性更新所有值
    this.setData(updates)

    // 重新计算结算总价
    this.calculateCheckoutTotal()

    // 同步到全局 store
    this.syncToGlobalStore()
  },

  /**
   * 本地移除商品（避免整页刷新）
   */
  removeLocalItem(itemId: number) {
    const { merchantGroups, selectedCartIds } = this.data
    let targetGroupIndex = -1
    let targetItemIndex = -1

    for (let i = 0; i < merchantGroups.length; i++) {
      const itemIndex = merchantGroups[i].items.findIndex(item => item.id === itemId)
      if (itemIndex !== -1) {
        targetGroupIndex = i
        targetItemIndex = itemIndex
        break
      }
    }

    if (targetGroupIndex === -1 || targetItemIndex === -1) return

    const updatedGroups = merchantGroups.map((group, index) => {
      if (index !== targetGroupIndex) return group
      const nextItems = group.items.filter((_, itemIndex) => itemIndex !== targetItemIndex)
      return {
        ...group,
        items: nextItems
      }
    }).filter(group => group.items.length > 0)

    const removedGroup = merchantGroups[targetGroupIndex]
    const nextSelectedCartIds = removedGroup && updatedGroups.every(group => group.cartId !== removedGroup.cartId)
      ? selectedCartIds.filter(id => id !== removedGroup.cartId)
      : selectedCartIds

    this.setData({
      merchantGroups: updatedGroups,
      selectedCartIds: nextSelectedCartIds
    }, () => {
      if (updatedGroups.length === 0) {
        this.setData({
          summary: {
            cartCount: 0,
            totalItems: 0,
            totalAmount: 0,
            totalAmountDisplay: '¥0.00'
          }
        })
      } else {
        this.recalculateSubtotals()
        this.calculateDeliveryFees(undefined, true)
      }
      this.calculateCheckoutTotal()
      this.syncToGlobalStore()
    })
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
            const group = this.data.merchantGroups.find(g => g.merchantId === merchantId)
            await CartAPI.clearCart({
              merchant_id: merchantId,
              order_type: group?.orderType || 'takeout',
              table_id: group?.tableId,
              reservation_id: group?.reservationId
            })

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
