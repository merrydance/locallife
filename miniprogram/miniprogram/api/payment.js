"use strict";
/**
 * 支付相关API接口
 * 基于swagger.json中的支付管理接口
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
exports.pay = exports.getPayments = void 0;
exports.getPaymentList = getPaymentList;
exports.getPaymentDetail = getPaymentDetail;
exports.createPayment = createPayment;
exports.closePayment = closePayment;
exports.getPaymentRefunds = getPaymentRefunds;
exports.createRefund = createRefund;
exports.createOrderPayment = createOrderPayment;
exports.createReservationPayment = createReservationPayment;
exports.invokeWechatPay = invokeWechatPay;
exports.processPayment = processPayment;
exports.checkPaymentStatus = checkPaymentStatus;
exports.pollPaymentStatus = pollPaymentStatus;
const request_1 = require("../utils/request");
// ==================== API接口函数 ====================
/**
 * 获取支付订单列表
 * @param params 查询参数
 */
function getPaymentList(params) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/payments',
            method: 'GET',
            data: params
        });
    });
}
/**
 * 获取支付订单详情
 * @param paymentId 支付订单ID
 */
function getPaymentDetail(paymentId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/payments/${paymentId}`,
            method: 'GET'
        });
    });
}
/**
 * 创建支付订单
 * @param paymentData 支付数据
 */
function createPayment(paymentData) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/payments',
            method: 'POST',
            data: paymentData
        });
    });
}
/**
 * 关闭支付订单
 * @param paymentId 支付订单ID
 */
function closePayment(paymentId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/payments/${paymentId}/close`,
            method: 'POST'
        });
    });
}
/**
 * 获取支付订单的退款列表
 * @param paymentId 支付订单ID
 */
function getPaymentRefunds(paymentId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/payments/${paymentId}/refunds`,
            method: 'GET'
        });
    });
}
/**
 * 创建退款
 * @param paymentId 支付订单ID
 * @param refundData 退款数据
 */
function createRefund(paymentId, refundData) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/payments/${paymentId}/refunds`,
            method: 'POST',
            data: refundData
        });
    });
}
// ==================== 便捷方法 ====================
/**
 * 为订单创建小程序支付
 * @param orderId 订单ID
 */
function createOrderPayment(orderId) {
    return __awaiter(this, void 0, void 0, function* () {
        return createPayment({
            order_id: orderId,
            payment_type: 'miniprogram',
            business_type: 'order'
        });
    });
}
/**
 * 为预定创建小程序支付
 * @param reservationId 预定ID
 */
function createReservationPayment(reservationId) {
    return __awaiter(this, void 0, void 0, function* () {
        return createPayment({
            order_id: reservationId,
            payment_type: 'miniprogram',
            business_type: 'reservation'
        });
    });
}
/**
 * 调起微信支付
 * @param paymentParams 支付参数
 */
function invokeWechatPay(paymentParams) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            wx.requestPayment(Object.assign(Object.assign({}, paymentParams), { success: () => resolve(), fail: (error) => reject(error) }));
        });
    });
}
/**
 * 完整的支付流程
 * @param orderId 订单ID
 * @param businessType 业务类型
 */
function processPayment(orderId_1) {
    return __awaiter(this, arguments, void 0, function* (orderId, businessType = 'order') {
        try {
            // 1. 创建支付订单
            const payment = yield createPayment({
                order_id: orderId,
                payment_type: 'miniprogram',
                business_type: businessType
            });
            // 2. 调起微信支付
            if (payment.pay_params) {
                yield invokeWechatPay(payment.pay_params);
            }
            else {
                throw new Error('支付参数缺失');
            }
        }
        catch (error) {
            console.error('支付失败:', error);
            throw error;
        }
    });
}
/**
 * 检查支付状态
 * @param paymentId 支付订单ID
 */
function checkPaymentStatus(paymentId) {
    return __awaiter(this, void 0, void 0, function* () {
        const payment = yield getPaymentDetail(paymentId);
        return payment.status;
    });
}
/**
 * 轮询支付状态直到完成
 * @param paymentId 支付订单ID
 * @param maxAttempts 最大尝试次数
 * @param interval 轮询间隔（毫秒）
 */
function pollPaymentStatus(paymentId_1) {
    return __awaiter(this, arguments, void 0, function* (paymentId, maxAttempts = 30, interval = 2000) {
        for (let i = 0; i < maxAttempts; i++) {
            const status = yield checkPaymentStatus(paymentId);
            if (status === 'paid' || status === 'failed' || status === 'cancelled') {
                return status;
            }
            if (i < maxAttempts - 1) {
                yield new Promise(resolve => setTimeout(resolve, interval));
            }
        }
        throw new Error('支付状态检查超时');
    });
}
// ==================== 兼容性别名 ====================
/** @deprecated 使用 getPaymentList 替代 */
exports.getPayments = getPaymentList;
/** @deprecated 使用 createPayment 替代 */
exports.pay = createPayment;
