"use strict";
/**
 * 购物车相关API接口
 * 严格对齐 swagger.json 中的购物车管理接口
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.addCartItem = exports.getCartItems = void 0;
exports.getCart = getCart;
exports.getCartSummary = getCartSummary;
exports.addToCart = addToCart;
exports.updateCartItem = updateCartItem;
exports.removeFromCart = removeFromCart;
exports.clearCart = clearCart;
exports.calculateCart = calculateCart;
exports.getCartItemCount = getCartItemCount;
exports.isCartEmpty = isCartEmpty;
exports.quickAddToCart = quickAddToCart;
const request_1 = require("../utils/request");
// ==================== API接口函数 ====================
/**
 * 获取指定商户的购物车
 * @param merchantId 商户ID
 */
function getCart(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/cart',
            method: 'GET',
            data: { merchant_id: merchantId }
        });
    });
}
/**
 * 获取购物车摘要（所有商户）
 */
function getCartSummary() {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/cart/summary',
            method: 'GET'
        });
    });
}
/**
 * 添加商品到购物车
 * @param item 商品信息
 */
function addToCart(item) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/cart/items',
            method: 'POST',
            data: item
        });
    });
}
/**
 * 更新购物车商品
 * @param itemId 商品项ID
 * @param updates 更新数据
 */
function updateCartItem(itemId, updates) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/cart/items/${itemId}`,
            method: 'PATCH', // Swagger 定义是 PATCH
            data: updates
        });
    });
}
/**
 * 从购物车删除商品
 * @param itemId 商品项ID
 */
function removeFromCart(itemId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/cart/items/${itemId}`,
            method: 'DELETE'
        });
    });
}
/**
 * 清空指定商户的购物车
 * @param merchantId 商户ID
 */
function clearCart(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/cart/clear',
            method: 'POST',
            data: merchantId ? { merchant_id: merchantId } : undefined
        });
    });
}
/**
 * 计算购物车金额
 * @param params 计算参数
 */
function calculateCart(params) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/cart/calculate',
            method: 'POST',
            data: params
        });
    });
}
// ==================== 便捷方法 ====================
/**
 * 获取购物车商品总数
 * @param merchantId 商户ID（可选，不传则获取所有商户）
 */
function getCartItemCount(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        if (merchantId) {
            const cart = yield getCart(merchantId);
            return cart.total_count;
        }
        else {
            const summary = yield getCartSummary();
            return summary.total_items;
        }
    });
}
/**
 * 检查购物车是否为空
 * @param merchantId 商户ID（可选）
 */
function isCartEmpty(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        const count = yield getCartItemCount(merchantId);
        return count === 0;
    });
}
/**
 * 快速添加商品（不含定制化）
 * @param merchantId 商户ID
 * @param dishId 菜品ID
 * @param quantity 数量
 */
function quickAddToCart(merchantId_1, dishId_1) {
    return __awaiter(this, arguments, void 0, function* (merchantId, dishId, quantity = 1) {
        return addToCart({
            merchant_id: merchantId,
            dish_id: dishId,
            quantity
        });
    });
}
/** @deprecated 使用 getCart 替代 */
exports.getCartItems = getCart;
/** @deprecated 使用 addToCart 替代 */
exports.addCartItem = addToCart;
