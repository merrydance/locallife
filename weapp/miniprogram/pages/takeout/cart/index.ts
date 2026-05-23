import * as CartAPI from '@/api/cart'
import { logger } from '@/utils/logger'
import {
  buildCartSummary,
  buildEmptyCartSummary,
  buildMerchantGroup,
  buildRecalculatedGroup,
  buildUpdatedGroupWithDeliveryFee,
  getCheckoutTotal,
  getTotalCount,
  isAbortLikeError,
  type MerchantCartGroup
} from '@/utils/takeout-cart-view'

let _loadAllCartsPromise: Promise<void> | null = null
let _lastLoadAllCartsAt = 0
const SINGLE_MERCHANT_CHECKOUT_NOTICE = '暂不支持多商户一起支付，请选择一家商户的商品结算。'

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
    checkoutTotalDisplay: '¥0.00',
    singleMerchantCheckoutNotice: '',
    removingUnavailableItemIds: {} as Record<string, boolean>
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
    const now = Date.now()
    if (_loadAllCartsPromise) {
      return _loadAllCartsPromise
    }
    if (now - _lastLoadAllCartsAt < 800) {
      logger.debug('skip duplicated loadAllCarts in short window', { since: now - _lastLoadAllCartsAt }, 'cart.loadAllCarts')
      return
    }

    _loadAllCartsPromise = (async () => {
      try {
        this.setData({ loading: true })

        // 获取用户所有购物车汇总
        const userCarts = await CartAPI.getUserCarts('takeout')

        if (!userCarts.carts || userCarts.carts.length === 0) {
          this.setData({
            loading: false,
            merchantGroups: [],
            summary: buildEmptyCartSummary()
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
            const group = buildMerchantGroup(merchantCart, cartDetail)
            merchantGroups.push(group)
          } catch (error) {
            logger.warn('Failed to load cart for merchant', { merchantId: merchantCart.merchant_id }, 'cart.loadAllCarts')
          }
        }

        // 预先计算费用并校验商户状态（在显示前完成）
        merchantGroups = await this.calculateDeliveryFees(merchantGroups, true)

        // 当前支付设计只支持单商户结算，默认选中第一家可用商户。
        const availableGroups = merchantGroups.filter((g) => !g.errorStatus)
        const selectedCartIds = availableGroups.slice(0, 1).map((g) => g.cartId)
        merchantGroups = merchantGroups.map((group) => ({
          ...group,
          selected: selectedCartIds.includes(group.cartId)
        }))

        // 设置数据并显示
        this.setData({
          loading: false,
          merchantGroups,
          selectedCartIds,
          summary: buildCartSummary(userCarts.summary, merchantGroups.length)
        })

        this.calculateCheckoutTotal()

        // 同步到全局状态，让其他页面（如外卖首页）能感知购物车变化
        this.syncToGlobalStore()
      } catch (error) {
        logger.error('Failed to load carts', error, 'cart.loadAllCarts')
        this.setData({ loading: false })
        if (isAbortLikeError(error)) {
          return
        }
        wx.showToast({ title: '加载购物车失败', icon: 'none' })
      }
    })().finally(() => {
      _loadAllCartsPromise = null
      _lastLoadAllCartsAt = Date.now()
    })

    return _loadAllCartsPromise
  },

  /**
   * 同步购物车状态到全局存储
   */
  syncToGlobalStore() {
    const { globalStore } = require('@/utils/global-store')
    const { summary, merchantGroups } = this.data

    const totalCount = getTotalCount(merchantGroups)

    globalStore.set('cart', {
      items: [],  // 多商户模式下不使用单一 items 列表
      totalCount,
      totalPrice: summary.totalAmount,
      totalPriceDisplay: summary.totalAmountDisplay
    })

    logger.debug('Cart synced to globalStore', { totalCount }, 'cart.syncToGlobalStore')
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
        updatedGroups[i] = buildUpdatedGroupWithDeliveryFee(group, result.delivery_fee || 0)
      } catch (error: unknown) {
        const userMessage =
          typeof error === 'object' && error !== null && 'userMessage' in error
            ? String((error as { userMessage?: string }).userMessage || '')
            : ''
        logger.warn('Failed to calculate delivery fee', { merchantId: group.merchantId }, 'cart.calculateDeliveryFees')
        
        // 捕获错误并显示给用户（如商户打烊、超出范围）
        updatedGroups[i] = {
          ...group,
          errorStatus: userMessage || '暂不支持配送',
          selected: false // 有错误时自动取消选中
        }
      }
    }))

    // 如果是基于 this.data 进行的更新，则需要 setData
    if (!groups) {
      this.setData({ merchantGroups: updatedGroups })
      // 检查 selectedCartIds 是否需要更新
      const { selectedCartIds } = this.data
      const newSelectedIds = updatedGroups.filter((g) => g.selected && selectedCartIds.includes(g.cartId)).map((g) => g.cartId)
      // 如果有原来选中的现在因为错误变为了不选中
      if (newSelectedIds.length !== selectedCartIds.length) {
         // 注意：这里的简单比较可能不够，但通常足够处理 "选中->不选中" 的情况
         // 更严谨：newSelectedIds 应该是 (OldSelected intersect ValidGroups)
         // 上面的 filter 已经做了这件事：g.selected 被置为 false 了如果出错
         const validSelectedIds = selectedCartIds.filter((id) => {
           const g = updatedGroups.find((group) => group.cartId === id)
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
    const total = getCheckoutTotal(merchantGroups, selectedCartIds)

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

    const group = merchantGroups.find((g) => g.cartId === cartId)
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
    const updatedGroups = merchantGroups.map((group) => ({
      ...group,
      selected: selectedCartIds.includes(group.cartId)
    }))

    this.setData({
      selectedCartIds,
      merchantGroups: updatedGroups,
      singleMerchantCheckoutNotice: this.hasMultiMerchantSelection(selectedCartIds, updatedGroups)
        ? SINGLE_MERCHANT_CHECKOUT_NOTICE
        : ''
    })
    this.calculateCheckoutTotal()
  },

  getSelectedMerchantIds(selectedCartIds: number[], merchantGroups: MerchantCartGroup[]): number[] {
    const merchantIds = merchantGroups
      .filter((group) => selectedCartIds.includes(group.cartId))
      .map((group) => group.merchantId)
      .filter(Boolean)

    return Array.from(new Set(merchantIds))
  },

  hasMultiMerchantSelection(selectedCartIds: number[], merchantGroups: MerchantCartGroup[]): boolean {
    return this.getSelectedMerchantIds(selectedCartIds, merchantGroups).length > 1
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
   * 移除已下架商品
   */
  async onRemoveUnavailable(e: WechatMiniprogram.CustomEvent) {
    const rawItemId = e.currentTarget.dataset.itemId
    const itemId = Number(rawItemId)
    if (!Number.isFinite(itemId) || itemId <= 0) return

    const itemKey = String(itemId)
    if (this.data.removingUnavailableItemIds[itemKey]) return

    this.setRemovingUnavailableItem(itemKey, true)

    try {
      await CartAPI.removeFromCart(itemId, { loading: false })
      this.removeLocalItem(itemId)
    } catch (error) {
      logger.warn('Failed to remove unavailable cart item', { itemId }, 'cart.onRemoveUnavailable')
      wx.showToast({ title: '移除失败，请重试', icon: 'none' })
    } finally {
      this.setRemovingUnavailableItem(itemKey, false)
    }
  },

  setRemovingUnavailableItem(itemKey: string, removing: boolean) {
    const removingUnavailableItemIds = {
      ...this.data.removingUnavailableItemIds
    }

    if (removing) {
      removingUnavailableItemIds[itemKey] = true
    } else {
      delete removingUnavailableItemIds[itemKey]
    }

    this.setData({ removingUnavailableItemIds })
  },

  /**
   * 获取商品当前数量
   */
  getItemQuantity(itemId: number): number {
    for (const group of this.data.merchantGroups) {
      const item = group.items.find((i) => i.id === itemId)
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
      const itemIndex = merchantGroups[i].items.findIndex((item) => item.id === itemId)
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
      const group = buildRecalculatedGroup(merchantGroups[i])
      updates[`merchantGroups[${i}].subtotal`] = group.subtotal
      updates[`merchantGroups[${i}].subtotalDisplay`] = group.subtotalDisplay
      updates[`merchantGroups[${i}].itemCount`] = group.itemCount
      updates[`merchantGroups[${i}].totalAmount`] = group.totalAmount
      updates[`merchantGroups[${i}].totalAmountDisplay`] = group.totalAmountDisplay
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
      const itemIndex = merchantGroups[i].items.findIndex((item) => item.id === itemId)
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
    }).filter((group) => group.items.length > 0)

    const removedGroup = merchantGroups[targetGroupIndex]
    const nextSelectedCartIds = removedGroup && updatedGroups.every((group) => group.cartId !== removedGroup.cartId)
      ? selectedCartIds.filter((id) => id !== removedGroup.cartId)
      : selectedCartIds

    this.setData({
      merchantGroups: updatedGroups,
      selectedCartIds: nextSelectedCartIds,
      singleMerchantCheckoutNotice: this.hasMultiMerchantSelection(nextSelectedCartIds, updatedGroups)
        ? SINGLE_MERCHANT_CHECKOUT_NOTICE
        : ''
    }, () => {
      if (updatedGroups.length === 0) {
        this.setData({
          summary: buildEmptyCartSummary()
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
            const group = this.data.merchantGroups.find((g) => g.merchantId === merchantId)
            await CartAPI.clearCart({
              merchant_id: merchantId,
              order_type: group?.orderType || 'takeout',
              table_id: group?.tableId,
              reservation_id: group?.reservationId
            })

            // 本地移除该商户分组，避免重新加载整个页面
            const { merchantGroups, selectedCartIds } = this.data
            const groupIndex = merchantGroups.findIndex((g) => g.merchantId === merchantId)

            if (groupIndex !== -1) {
              const removedGroup = merchantGroups[groupIndex]
              const newGroups = merchantGroups.filter((_, i) => i !== groupIndex)
              const newSelectedIds = selectedCartIds.filter((id) => id !== removedGroup.cartId)

              this.setData({
                merchantGroups: newGroups,
                selectedCartIds: newSelectedIds,
                singleMerchantCheckoutNotice: this.hasMultiMerchantSelection(newSelectedIds, newGroups)
                  ? SINGLE_MERCHANT_CHECKOUT_NOTICE
                  : ''
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

    if (this.hasMultiMerchantSelection(selectedCartIds, merchantGroups)) {
      this.setData({ singleMerchantCheckoutNotice: SINGLE_MERCHANT_CHECKOUT_NOTICE })
      wx.showModal({
        title: '暂不支持多商户一起支付',
        content: '请只选择一家商户的商品结算。',
        showCancel: false,
        confirmText: '知道了'
      })
      return
    }

    // 检查是否有不可用商品
    for (const group of merchantGroups) {
      if (selectedCartIds.includes(group.cartId) && !group.allAvailable) {
        wx.showToast({ title: '部分商品已下架，请移除后再结算', icon: 'none' })
        return
      }
    }

    const checkoutCarts = merchantGroups
      .filter((group) => selectedCartIds.includes(group.cartId))
      .map((group) => ({
        cartId: group.cartId,
        merchantId: group.merchantId,
        merchantName: group.merchantName,
        orderType: group.orderType || 'takeout',
        tableId: group.tableId,
        reservationId: group.reservationId,
        subtotal: group.subtotal,
        totalCount: group.itemCount,
        items: group.items.map((item) => ({
          id: item.id,
          dishId: item.dishId,
          comboId: item.comboId,
          name: item.name,
          imageUrl: item.imageUrl,
          quantity: item.quantity,
          unitPrice: item.unitPrice,
          subtotal: item.subtotal,
          specText: item.specDisplay,
          customizations: item.customizations,
          dishImages: item.dishImages || []
        }))
      }))

    wx.navigateTo({
      url: `/pages/takeout/order-confirm/index?cart_ids=${selectedCartIds.join(',')}`,
      success: (res) => {
        res.eventChannel.emit('checkoutContext', {
          cartIds: selectedCartIds,
          carts: checkoutCarts
        })
      }
    })
  },

  /**
   * 返回外卖页
   */
  onBackToTakeout() {
    wx.navigateBack()
  }
})
