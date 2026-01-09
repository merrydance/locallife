import { CartResponse, CartItemResponse, AddCartItemRequest, UpdateCartItemRequest } from '../api/cart'
import * as CartAPI from '../api/cart'
import { logger } from '../utils/logger'
import { globalStore } from '../utils/global-store'

/**
 * CartService - Backend Synchronized Version
 * Manages cart state by communicating with the backend API.
 * Acts as a centralized store for the current merchant's cart.
 */
class CartService {
  private static instance: CartService

  // Cache the current cart to avoid excessive network requests
  private currentCart: CartResponse | null = null
  private currentMerchantId: number | null = null

  static getInstance(): CartService {
    if (!CartService.instance) {
      CartService.instance = new CartService()
    }
    return CartService.instance
  }

  /**
   * Get the current cached cart.
   * Note: This might be stale. Use refreshCart() to ensure latest data.
   */
  getCart(): CartResponse | null {
    return this.currentCart
  }

  /**
   * Get the current merchant ID being operated on
   */
  getMerchantId(): number | null {
    return this.currentMerchantId
  }

  /**
   * Initialize or switch to a specific merchant's cart
   */
  async loadCart(merchantId: number): Promise<CartResponse> {
    this.currentMerchantId = merchantId
    return this.refreshCart()
  }

  /**
   * Refresh the cart data from backend
   */
  async refreshCart(): Promise<CartResponse> {
    if (!this.currentMerchantId) {
      throw new Error('No merchant selected for CartService')
    }

    try {
      logger.debug('Refreshing cart from backend', { merchantId: this.currentMerchantId }, 'CartService.refreshCart')
      const cart = await CartAPI.getCart({ merchant_id: this.currentMerchantId })
      this.currentCart = cart

      this.notifyListeners()
      return cart
    } catch (error) {
      logger.error('Failed to refresh cart', error, 'CartService.refreshCart')
      throw error
    }
  }

  /**
   * Add item to backend cart
   */
  async addItem(item: {
    merchantId: string | number,
    dishId?: string | number,
    comboId?: string | number,
    quantity?: number,
    customizations?: Record<string, unknown>
  }): Promise<boolean> {
    try {
      const merchantId = Number(item.merchantId)
      const quantity = item.quantity || 1

      const req: AddCartItemRequest = {
        merchant_id: merchantId,
        dish_id: item.dishId ? Number(item.dishId) : undefined,
        combo_id: item.comboId ? Number(item.comboId) : undefined,
        quantity: quantity,
        customizations: item.customizations
      }

      logger.info('Adding item to backend cart', req, 'CartService.addItem')
      const updatedCart = await CartAPI.addToCart(req)

      // Update local state
      this.currentMerchantId = merchantId
      this.currentCart = updatedCart
      this.notifyListeners()

      return true
    } catch (error) {
      logger.error('Failed to add item to cart', error, 'CartService.addItem')

      // Handle simple error reporting
      wx.showToast({
        title: '添加失败，请重试',
        icon: 'none'
      })
      return false
    }
  }

  /**
   * Update item quantity or specs
   */
  async updateItem(itemId: number, updates: UpdateCartItemRequest): Promise<boolean> {
    try {
      const updatedCart = await CartAPI.updateCartItem(itemId, updates)
      this.currentCart = updatedCart
      this.notifyListeners()
      return true
    } catch (error) {
      logger.error('Failed to update cart item', error, 'CartService.updateItem')
      return false
    }
  }

  /**
   * Remove item from cart
   */
  async removeItem(itemId: number): Promise<boolean> {
    try {
      const updatedCart = await CartAPI.removeFromCart(itemId)
      this.currentCart = updatedCart
      this.notifyListeners()
      return true
    } catch (error) {
      logger.error('Failed to remove item', error, 'CartService.removeItem')
      return false
    }
  }

  /**
   * Update quantity helper
   */
  async updateQuantity(itemId: number, quantity: number): Promise<boolean> {
    if (quantity <= 0) {
      return this.removeItem(itemId)
    }
    return this.updateItem(itemId, { quantity })
  }

  /**
   * Clear current merchant's cart
   */
  async clear(): Promise<boolean> {
    if (!this.currentMerchantId) return false

    try {
      await CartAPI.clearCart(this.currentMerchantId)

      // Reset local state to empty structure manually or refetch
      // Refetching is safer to ensure backend state
      return this.refreshCart().then(() => true)
    } catch (error) {
      logger.error('Failed to clear cart', error, 'CartService.clear')
      return false
    }
  }

  /**
   * Notify global store or event system about cart changes
   * This adapts the new API structure to the old global store format if necessary
   */
  private notifyListeners() {
    if (!this.currentCart) return

    // Adapt to the format expected by the frontend
    // The previous frontend might expect { totalCount, totalPrice }
    // We map the backend response to that structure
    const cartSummary = {
      items: this.currentCart.items || [],
      totalCount: this.currentCart.total_count,
      totalPrice: this.currentCart.subtotal,
      totalPriceDisplay: `¥${(this.currentCart.subtotal / 100).toFixed(2)}`
    }

    // You can use a dedicated event emitter or the global store
    // For now, we update the global store entry 'cart'
    globalStore.set('cart', cartSummary)
  }
}

export default CartService.getInstance()
