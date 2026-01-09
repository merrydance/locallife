"use strict";
/**
 * 订单相关API接口
 * 基于swagger.json中的订单管理接口
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
exports.previewOrder = exports.getOrders = void 0;
exports.getPayableAmount = getPayableAmount;
exports.getOrderList = getOrderList;
exports.getOrderDetail = getOrderDetail;
exports.createOrder = createOrder;
exports.calculateOrder = calculateOrder;
exports.cancelOrder = cancelOrder;
exports.confirmOrder = confirmOrder;
exports.urgeOrder = urgeOrder;
exports.getOrdersByStatus = getOrdersByStatus;
exports.getPendingOrders = getPendingOrders;
exports.getActiveOrders = getActiveOrders;
exports.getHistoryOrders = getHistoryOrders;
exports.createOrderFromCart = createOrderFromCart;
const request_1 = require("../utils/request");
/** 计算后的应付金额（便捷属性，total_amount - discount_amount） */
function getPayableAmount(order) {
    return order.total_amount - order.discount_amount;
}
// ==================== API接口函数 ====================
/**
 * 获取订单列表
 * @param params 查询参数
 */
function getOrderList(params) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/orders',
            method: 'GET',
            data: params
        });
    });
}
/**
 * 获取订单详情
 * @param orderId 订单ID
 */
function getOrderDetail(orderId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/orders/${orderId}`,
            method: 'GET'
        });
    });
}
/**
 * 创建订单
 * @param orderData 订单数据
 */
function createOrder(orderData) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/orders',
            method: 'POST',
            data: orderData
        });
    });
}
/**
 * 计算订单金额
 * @param params 计算参数
 */
function calculateOrder(params) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/orders/calculate',
            method: 'GET',
            data: params
        });
    });
}
/**
 * 取消订单
 * @param orderId 订单ID
 * @param cancelData 取消原因
 */
function cancelOrder(orderId, cancelData) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/orders/${orderId}/cancel`,
            method: 'POST',
            data: cancelData
        });
    });
}
/**
 * 确认订单（用户确认收货）
 * @param orderId 订单ID
 */
function confirmOrder(orderId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/orders/${orderId}/confirm`,
            method: 'POST'
        });
    });
}
/**
 * 催单
 * @param orderId 订单ID
 * @param urgeData 催单信息
 */
function urgeOrder(orderId_1) {
    return __awaiter(this, arguments, void 0, function* (orderId, urgeData = {}) {
        return (0, request_1.request)({
            url: `/v1/orders/${orderId}/urge`,
            method: 'POST',
            data: urgeData
        });
    });
}
// ==================== 便捷方法 ====================
/**
 * 获取指定状态的订单
 * @param status 订单状态
 * @param pageSize 每页数量
 */
function getOrdersByStatus(status_1) {
    return __awaiter(this, arguments, void 0, function* (status, pageSize = 10) {
        return getOrderList({
            page_id: 1,
            page_size: pageSize,
            status
        });
    });
}
/**
 * 获取待支付订单
 */
function getPendingOrders() {
    return __awaiter(this, void 0, void 0, function* () {
        return getOrdersByStatus('pending');
    });
}
/**
 * 获取进行中的订单（已支付但未完成）
 */
function getActiveOrders() {
    return __awaiter(this, void 0, void 0, function* () {
        const statuses = ['paid', 'preparing', 'ready', 'delivering'];
        const results = yield Promise.all(statuses.map(status => getOrdersByStatus(status, 20)));
        return results.reduce((acc, curr) => acc.concat(curr), []);
    });
}
/**
 * 获取历史订单（已完成或已取消）
 */
function getHistoryOrders() {
    return __awaiter(this, void 0, void 0, function* () {
        const statuses = ['completed', 'cancelled'];
        const results = yield Promise.all(statuses.map(status => getOrdersByStatus(status, 20)));
        return results.reduce((acc, curr) => acc.concat(curr), []);
    });
}
/**
 * 从购物车创建订单
 * @param merchantId 商户ID
 * @param orderType 订单类型
 * @param options 其他选项
 */
function createOrderFromCart(merchantId_1, orderType_1) {
    return __awaiter(this, arguments, void 0, function* (merchantId, orderType, options = {}) {
        // 这里需要先获取购物车数据，然后转换为订单格式
        // 实际实现时需要调用购物车API
        console.log('Creating order from cart for merchant:', merchantId, 'type:', orderType, 'options:', options);
        throw new Error('需要先实现购物车到订单的转换逻辑');
    });
}
// ==================== 兼容性别名 ====================
/** @deprecated 使用 getOrderList 替代 */
exports.getOrders = getOrderList;
/** @deprecated 使用 calculateOrder 替代 */
exports.previewOrder = calculateOrder;
