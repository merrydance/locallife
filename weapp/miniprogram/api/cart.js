"use strict";
/**
 * 购物车相关API接口
 * 严格对齐 swagger.json 中的购物车管理接口
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.addCartItem = exports.getCartItems = void 0;
exports.getCart = getCart;
exports.getCartSummary = getCartSummary;
exports.getUserCarts = getUserCarts;
exports.addToCart = addToCart;
exports.updateCartItem = updateCartItem;
exports.removeFromCart = removeFromCart;
exports.clearCart = clearCart;
exports.calculateCart = calculateCart;
exports.previewCombinedCheckout = previewCombinedCheckout;
exports.getCartItemCount = getCartItemCount;
exports.isCartEmpty = isCartEmpty;
exports.quickAddToCart = quickAddToCart;
const request_1 = require("../utils/request");
// ==================== API接口函数 ====================
/**
 * 获取指定商户的购物车
 * @param params 获取参数
 */
async function getCart(params, options) {
    return (0, request_1.request)({
        url: '/v1/cart',
        method: 'GET',
        data: params,
        ...(options || {})
    });
}
/**
 * 获取购物车摘要（所有商户）
 * @param orderType 订单类型过滤
 */
async function getCartSummary(orderType) {
    const response = await (0, request_1.request)({
        url: '/v1/cart/summary',
        method: 'GET',
        data: orderType ? { order_type: orderType } : undefined
    });
    return response.summary;
}
/**
 * 获取用户所有商户的购物车（完整信息）
 * @param orderType 订单类型过滤
 */
async function getUserCarts(orderType, options) {
    return (0, request_1.request)({
        url: '/v1/cart/summary',
        method: 'GET',
        data: orderType ? { order_type: orderType } : undefined,
        ...(options || {})
    });
}
/**
 * 添加商品到购物车
 * @param item 商品信息
 */
async function addToCart(item, options) {
    return (0, request_1.request)({
        url: '/v1/cart/items',
        method: 'POST',
        data: item,
        ...(options || {})
    });
}
/**
 * 更新购物车商品
 * @param itemId 商品项ID
 * @param updates 更新数据
 */
async function updateCartItem(itemId, updates, options) {
    return (0, request_1.request)({
        url: `/v1/cart/items/${itemId}`,
        method: 'PATCH', // Swagger 定义是 PATCH
        data: updates,
        ...(options || {})
    });
}
/**
 * 从购物车删除商品
 * @param itemId 商品项ID
 */
async function removeFromCart(itemId, options) {
    return (0, request_1.request)({
        url: `/v1/cart/items/${itemId}`,
        method: 'DELETE',
        ...(options || {})
    });
}
/**
 * 清空指定商户的购物车
 * @param merchantId 商户ID
 */
async function clearCart(params) {
    return (0, request_1.request)({
        url: '/v1/cart/clear',
        method: 'POST',
        data: params
    });
}
/**
 * 计算购物车金额
 * @param params 计算参数
 */
async function calculateCart(params, options) {
    return (0, request_1.request)({
        url: '/v1/cart/calculate',
        method: 'POST',
        data: params,
        ...(options || {})
    });
}
/**
 * 预览合单结算
 * 多商户合单结算预览，返回各商户子单和合计金额
 * @param params 合单结算请求
 */
async function previewCombinedCheckout(params) {
    return (0, request_1.request)({
        url: '/v1/cart/combined-checkout/preview',
        method: 'POST',
        data: params
    });
}
// ==================== 便捷方法 ====================
/**
 * 获取购物车商品总数
 * @param merchantId 商户ID（可选，不传则获取所有商户）
 */
async function getCartItemCount(merchantId) {
    if (merchantId) {
        const cart = await getCart({ merchant_id: merchantId });
        return cart.total_count;
    }
    else {
        const summary = await getCartSummary();
        return summary.total_items;
    }
}
/**
 * 检查购物车是否为空
 * @param merchantId 商户ID（可选）
 */
async function isCartEmpty(merchantId) {
    const count = await getCartItemCount(merchantId);
    return count === 0;
}
/**
 * 快速添加商品（不含定制化）
 * @param merchantId 商户ID
 * @param dishId 菜品ID
 * @param quantity 数量
 */
async function quickAddToCart(merchantId, dishId, quantity = 1) {
    return addToCart({
        merchant_id: merchantId,
        dish_id: dishId,
        quantity
    });
}
/** @deprecated 使用 getCart 替代 */
exports.getCartItems = getCart;
/** @deprecated 使用 addToCart 替代 */
exports.addCartItem = addToCart;
