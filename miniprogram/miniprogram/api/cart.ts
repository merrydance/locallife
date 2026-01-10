/**
 * 购物车相关API接口
 * 严格对齐 swagger.json 中的购物车管理接口
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 购物车响应 - 对齐 api.cartResponse */
export interface CartResponse {
    id: number
    merchant_id: number
    order_type: string
    table_id?: number
    reservation_id?: number
    items: CartItemResponse[]
    subtotal: number
    total_count: number
}

/** 购物车商品项 - 对齐 api.cartItemResponse */
export interface CartItemResponse {
    id: number
    dish_id?: number
    combo_id?: number
    name: string              // 商品名称
    image_url: string         // 商品图片
    quantity: number
    unit_price: number
    subtotal: number          // 小计金额
    member_price?: number     // 会员价
    is_available: boolean
    customizations?: Record<string, unknown>  // 定制选项，object类型
}

/** 购物车摘要 - 对齐 api.cartSummaryResponse */
export interface CartSummaryResponse {
    cart_count: number        // 购物车数量（商户数）
    total_items: number       // 商品总数
    total_amount: number      // 商品总金额（分）
}

/** 添加商品到购物车请求 - 对齐 api.addCartItemRequest */
export interface AddCartItemRequest extends Record<string, unknown> {
    merchant_id: number
    order_type?: string       // 订单类型：takeout, dine_in, reservation
    table_id?: number
    reservation_id?: number
    dish_id?: number          // dish_id和combo_id二选一
    combo_id?: number         // dish_id和combo_id二选一
    quantity: number          // 数量，范围：1-99
    customizations?: Record<string, unknown>  // 定制选项
}

/** 更新购物车商品请求 - 对齐 api.updateCartItemRequest */
export interface UpdateCartItemRequest extends Record<string, unknown> {
    quantity?: number         // 数量，范围：1-99
    customizations?: Record<string, unknown>  // 定制选项
}

/** 计算购物车请求 - 对齐 api.calculateCartRequest */
export interface CalculateCartRequest extends Record<string, unknown> {
    merchant_id: number
    order_type?: string
    table_id?: number
    reservation_id?: number
    address_id?: number       // 配送地址ID，用于计算配送费
    latitude?: number         // 用户当前位置纬度（address_id的fallback）
    longitude?: number        // 用户当前位置经度（address_id的fallback）
    voucher_id?: number       // 优惠券ID，用于计算优惠
}

/** 清空购物车请求 - 对齐 api.clearCartRequest */
export interface ClearCartRequest extends Record<string, unknown> {
    merchant_id: number       // 商户ID (必填)
    order_type?: string
    table_id?: number
    reservation_id?: number
}

/** 商户购物车响应 - 对齐 api.merchantCartResponse */
export interface MerchantCartResponse {
    all_available?: boolean   // 所有商品是否可购买
    cart_id?: number          // 购物车ID
    item_count?: number       // 商品数量
    merchant_id?: number      // 商户ID
    merchant_logo?: string    // 商户Logo URL
    merchant_name?: string    // 商户名称
    subtotal?: number         // 商品小计（分）
    order_type?: string       // 订单类型
    table_id?: number         // 桌台ID
    reservation_id?: number   // 预约ID
}

/** 用户所有购物车响应 - 对齐 api.userCartsResponse */
export interface UserCartsResponse {
    carts: MerchantCartResponse[]  // 各商户购物车列表
    summary: CartSummaryResponse   // 汇总统计
}

/** 添加菜品项 - 对齐 api.addDishItem */
export interface AddDishItem {
    dish_id?: number          // 菜品ID
    combo_id?: number         // 套餐ID
    quantity: number          // 数量（1-99）
}

/** 批量添加菜品请求 - 对齐 api.addDishesRequest */
export interface AddDishesRequest extends Record<string, unknown> {
    items: AddDishItem[]      // 商品列表（1-50个）
}

/** 合单结算商品项 - 对齐 api.combinedCheckoutItem */
export interface CombinedCheckoutItem {
    merchant_id: number       // 商户ID
    merchant_name: string     // 商户名称
    order_type: string        // 订单类型
    subtotal: number          // 商品小计（分）
    delivery_fee: number      // 配送费（分）
    total_amount: number      // 小计+配送费（分）
}

/** 合单结算请求 - 对齐 api.combinedCheckoutRequest */
export interface CombinedCheckoutRequest extends Record<string, unknown> {
    cart_ids: number[]        // 要结算的购物车ID列表（1-10个）
    address_id?: number       // 配送地址ID（外卖时必填）
}

/** 合单结算响应 - 对齐 api.combinedCheckoutResponse */
export interface CombinedCheckoutResponse {
    items: CombinedCheckoutItem[]  // 各商户订单
    total_subtotal: number         // 商品合计（分）
    total_delivery_fee: number     // 配送费合计（分）
    total_amount: number           // 支付总额（分）
    can_combine_pay: boolean       // 是否可以合单支付
    message?: string               // 提示信息
}

/** 购物车计算结果 - 对齐 api.calculateCartResponse */
export interface CalculateCartResponse {
    subtotal: number              // 商品小计（分）
    delivery_fee: number          // 配送费（分）
    delivery_fee_discount: number // 配送费满返减免（分）
    discount_amount: number       // 优惠券减免金额（分）
    discount_info?: string        // 优惠说明
    meets_min_order: boolean      // 是否满足起送金额
    min_order_amount: number      // 最小起送金额（分），0表示无限制
    total_amount: number          // 实付金额（分）
}

// ==================== API接口函数 ====================

/**
 * 获取指定商户的购物车
 * @param params 获取参数
 */
export async function getCart(params: {
    merchant_id: number,
    order_type?: string,
    table_id?: number,
    reservation_id?: number
}): Promise<CartResponse> {
    return request({
        url: '/v1/cart',
        method: 'GET',
        data: params
    })
}

/**
 * 获取购物车摘要（所有商户）
 * @param orderType 订单类型过滤
 */
export async function getCartSummary(orderType: string = 'takeout'): Promise<CartSummaryResponse> {
    return request({
        url: '/v1/rpc/get_cart_summary_v2',
        method: 'POST',
        data: { p_order_type: orderType }
    })
}

/**
 * 获取用户所有商户的购物车（完整信息）
 * @param orderType 订单类型过滤
 */
export async function getUserCarts(orderType: string = 'takeout'): Promise<UserCartsResponse> {
    // 这里暂时先返回 summary，如果需要完整 carts 信息，后续再扩展 RPC
    const summary = await getCartSummary(orderType);
    return {
        carts: [], // 暂空，首页主要展示 summary
        summary
    }
}

/**
 * 添加商品到购物车
 * @param item 商品信息
 */
export async function addToCart(item: AddCartItemRequest): Promise<CartResponse> {
    return request({
        url: '/v1/cart/items',
        method: 'POST',
        data: item
    })
}

/**
 * 更新购物车商品
 * @param itemId 商品项ID
 * @param updates 更新数据
 */
export async function updateCartItem(itemId: number, updates: UpdateCartItemRequest): Promise<CartResponse> {
    return request({
        url: `/v1/cart/items/${itemId}`,
        method: 'PATCH',  // Swagger 定义是 PATCH
        data: updates
    })
}

/**
 * 从购物车删除商品
 * @param itemId 商品项ID
 */
export async function removeFromCart(itemId: number): Promise<CartResponse> {
    return request({
        url: `/v1/cart/items/${itemId}`,
        method: 'DELETE'
    })
}

/**
 * 清空指定商户的购物车
 * @param merchantId 商户ID
 */
export async function clearCart(merchantId?: number): Promise<void> {
    return request({
        url: '/v1/cart/clear',
        method: 'POST',
        data: merchantId ? { merchant_id: merchantId } : undefined
    })
}

/**
 * 计算购物车金额
 * @param params 计算参数
 */
export async function calculateCart(params: CalculateCartRequest): Promise<CalculateCartResponse> {
    return request({
        url: '/v1/cart/calculate',
        method: 'POST',
        data: params
    })
}

/**
 * 预览合单结算
 * 多商户合单结算预览，返回各商户子单和合计金额
 * @param params 合单结算请求
 */
export async function previewCombinedCheckout(params: CombinedCheckoutRequest): Promise<CombinedCheckoutResponse> {
    return request({
        url: '/v1/cart/combined-checkout/preview',
        method: 'POST',
        data: params
    })
}

// ==================== 便捷方法 ====================

/**
 * 获取购物车商品总数
 * @param merchantId 商户ID（可选，不传则获取所有商户）
 */
export async function getCartItemCount(merchantId?: number): Promise<number> {
    if (merchantId) {
        const cart = await getCart({ merchant_id: merchantId })
        return cart.total_count
    } else {
        const summary = await getCartSummary()
        return summary.total_items
    }
}

/**
 * 检查购物车是否为空
 * @param merchantId 商户ID（可选）
 */
export async function isCartEmpty(merchantId?: number): Promise<boolean> {
    const count = await getCartItemCount(merchantId)
    return count === 0
}

/**
 * 快速添加商品（不含定制化）
 * @param merchantId 商户ID
 * @param dishId 菜品ID
 * @param quantity 数量
 */
export async function quickAddToCart(merchantId: number, dishId: number, quantity: number = 1): Promise<CartResponse> {
    return addToCart({
        merchant_id: merchantId,
        dish_id: dishId,
        quantity
    })
}

// ==================== 兼容性别名 ====================

/** @deprecated 使用 AddCartItemRequest 替代 */
export type AddToCartRequest = AddCartItemRequest

/** @deprecated 使用 CalculateCartResponse 替代 */
export type CartCalculationResponse = CalculateCartResponse

/** @deprecated 使用 getCart 替代 */
export const getCartItems = getCart

/** @deprecated 使用 CartResponse 替代 */
export type CartDTO = CartResponse

/** @deprecated 使用 addToCart 替代 */
export const addCartItem = addToCart
