"use strict";
/**
 * 支付和退款接口模块
 * 基于swagger.json完全重构，提供支付流程、退款处理和配送费计算
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
exports.PaymentStatusManager = exports.DeliveryFeeUtils = exports.RefundUtils = exports.PaymentUtils = exports.DeliveryFeeAdapter = exports.RefundAdapter = exports.PaymentAdapter = exports.calculateDeliveryFee = exports.getRefundById = exports.createRefund = exports.getPaymentRefunds = exports.closePayment = exports.getPaymentById = exports.getPayments = exports.createPayment = void 0;
const request_1 = require("../utils/request");
// ==================== API 接口函数 ====================
/**
 * 创建支付订单
 */
const createPayment = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/payments',
        method: 'POST',
        data: params
    });
});
exports.createPayment = createPayment;
/**
 * 获取支付列表
 */
const getPayments = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/payments',
        method: 'GET',
        data: params
    });
});
exports.getPayments = getPayments;
/**
 * 获取支付详情
 */
const getPaymentById = (id) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/payments/${id}`,
        method: 'GET'
    });
});
exports.getPaymentById = getPaymentById;
/**
 * 关闭支付订单
 */
const closePayment = (id) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/payments/${id}/close`,
        method: 'POST'
    });
});
exports.closePayment = closePayment;
/**
 * 获取支付的退款记录
 */
const getPaymentRefunds = (paymentId) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'GET'
    });
});
exports.getPaymentRefunds = getPaymentRefunds;
/**
 * 创建退款申请
 */
const createRefund = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/refunds',
        method: 'POST',
        data: params
    });
});
exports.createRefund = createRefund;
/**
 * 获取退款详情
 */
const getRefundById = (id) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/refunds/${id}`,
        method: 'GET'
    });
});
exports.getRefundById = getRefundById;
/**
 * 计算配送费
 */
const calculateDeliveryFee = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/delivery-fee/calculate',
        method: 'POST',
        data: params
    });
});
exports.calculateDeliveryFee = calculateDeliveryFee;
// ==================== 数据适配器 ====================
/**
 * 支付数据适配器
 */
class PaymentAdapter {
    /**
     * 适配支付数据
     */
    static adaptPayment(payment) {
        return Object.assign(Object.assign({}, payment), { id: Number(payment.id), order_id: Number(payment.order_id), amount: Number(payment.amount), refund_amount: payment.refund_amount ? Number(payment.refund_amount) : undefined, created_at: payment.created_at, paid_at: payment.paid_at || undefined, expired_at: payment.expired_at || undefined });
    }
    /**
     * 适配支付结果
     */
    static adaptPaymentResult(result) {
        return {
            payment: this.adaptPayment(result.payment),
            payment_info: result.payment_info
        };
    }
    /**
     * 构建支付参数
     */
    static buildPaymentParams(orderId, paymentMethod, amount, description) {
        return {
            order_id: orderId,
            payment_method: paymentMethod,
            amount,
            description: description || `订单 ${orderId} 支付`
        };
    }
}
exports.PaymentAdapter = PaymentAdapter;
/**
 * 退款数据适配器
 */
class RefundAdapter {
    /**
     * 适配退款数据
     */
    static adaptRefund(refund) {
        return Object.assign(Object.assign({}, refund), { id: Number(refund.id), payment_id: Number(refund.payment_id), amount: Number(refund.amount), operator_id: refund.operator_id ? Number(refund.operator_id) : undefined, created_at: refund.created_at, processed_at: refund.processed_at || undefined });
    }
    /**
     * 构建退款参数
     */
    static buildRefundParams(paymentId, amount, reason, refundType = 'full', operatorId) {
        return {
            payment_id: paymentId,
            amount,
            reason,
            refund_type: refundType,
            operator_id: operatorId
        };
    }
}
exports.RefundAdapter = RefundAdapter;
/**
 * 配送费适配器
 */
class DeliveryFeeAdapter {
    /**
     * 适配配送费结果
     */
    static adaptDeliveryFeeResult(result) {
        return Object.assign(Object.assign({}, result), { base_fee: Number(result.base_fee), distance_fee: Number(result.distance_fee), peak_hour_fee: Number(result.peak_hour_fee), promotion_discount: Number(result.promotion_discount), final_fee: Number(result.final_fee), breakdown: {
                base_fee: Number(result.breakdown.base_fee),
                distance_fee: Number(result.breakdown.distance_fee),
                peak_hour_fee: Number(result.breakdown.peak_hour_fee),
                total_before_discount: Number(result.breakdown.total_before_discount),
                promotion_discount: Number(result.breakdown.promotion_discount),
                final_fee: Number(result.breakdown.final_fee)
            }, promotions_applied: result.promotions_applied.map(promo => (Object.assign(Object.assign({}, promo), { discount_amount: Number(promo.discount_amount) }))) });
    }
    /**
     * 构建配送费计算参数
     */
    static buildCalculateParams(merchantId, userAddressId, orderAmount, options) {
        return {
            merchant_id: merchantId,
            user_address_id: userAddressId,
            order_amount: orderAmount,
            delivery_distance: options === null || options === void 0 ? void 0 : options.deliveryDistance,
            peak_hour: options === null || options === void 0 ? void 0 : options.peakHour,
            promotion_codes: options === null || options === void 0 ? void 0 : options.promotionCodes
        };
    }
}
exports.DeliveryFeeAdapter = DeliveryFeeAdapter;
// ==================== 便捷函数 ====================
/**
 * 支付便捷函数
 */
class PaymentUtils {
    /**
     * 创建微信支付
     */
    static createWechatPayment(orderId, amount, description) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = PaymentAdapter.buildPaymentParams(orderId, 'wechat_pay', amount, description);
            const result = yield (0, exports.createPayment)(params);
            return PaymentAdapter.adaptPaymentResult(result);
        });
    }
    /**
     * 创建支付宝支付
     */
    static createAlipayPayment(orderId, amount, description) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = PaymentAdapter.buildPaymentParams(orderId, 'alipay', amount, description);
            const result = yield (0, exports.createPayment)(params);
            return PaymentAdapter.adaptPaymentResult(result);
        });
    }
    /**
     * 余额支付
     */
    static createBalancePayment(orderId, amount, description) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = PaymentAdapter.buildPaymentParams(orderId, 'balance', amount, description);
            const result = yield (0, exports.createPayment)(params);
            return PaymentAdapter.adaptPaymentResult(result);
        });
    }
    /**
     * 检查支付状态
     */
    static checkPaymentStatus(paymentId) {
        return __awaiter(this, void 0, void 0, function* () {
            const payment = yield (0, exports.getPaymentById)(paymentId);
            return PaymentAdapter.adaptPayment(payment);
        });
    }
    /**
     * 获取用户支付历史
     */
    static getUserPaymentHistory(status_1) {
        return __awaiter(this, arguments, void 0, function* (status, page = 1, pageSize = 20) {
            const result = yield (0, exports.getPayments)({
                status,
                page,
                page_size: pageSize
            });
            return Object.assign(Object.assign({}, result), { data: result.data.map(payment => PaymentAdapter.adaptPayment(payment)) });
        });
    }
}
exports.PaymentUtils = PaymentUtils;
/**
 * 退款便捷函数
 */
class RefundUtils {
    /**
     * 申请全额退款
     */
    static requestFullRefund(paymentId, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            // 先获取支付信息确定退款金额
            const payment = yield (0, exports.getPaymentById)(paymentId);
            const params = RefundAdapter.buildRefundParams(paymentId, payment.amount, reason, 'full');
            const refund = yield (0, exports.createRefund)(params);
            return RefundAdapter.adaptRefund(refund);
        });
    }
    /**
     * 申请部分退款
     */
    static requestPartialRefund(paymentId, amount, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = RefundAdapter.buildRefundParams(paymentId, amount, reason, 'partial');
            const refund = yield (0, exports.createRefund)(params);
            return RefundAdapter.adaptRefund(refund);
        });
    }
    /**
     * 获取支付的退款记录
     */
    static getRefundHistory(paymentId) {
        return __awaiter(this, void 0, void 0, function* () {
            const refunds = yield (0, exports.getPaymentRefunds)(paymentId);
            return refunds.map(refund => RefundAdapter.adaptRefund(refund));
        });
    }
    /**
     * 检查退款状态
     */
    static checkRefundStatus(refundId) {
        return __awaiter(this, void 0, void 0, function* () {
            const refund = yield (0, exports.getRefundById)(refundId);
            return RefundAdapter.adaptRefund(refund);
        });
    }
}
exports.RefundUtils = RefundUtils;
/**
 * 配送费便捷函数
 */
class DeliveryFeeUtils {
    /**
     * 快速计算配送费
     */
    static quickCalculate(merchantId, userAddressId, orderAmount) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount);
            const result = yield (0, exports.calculateDeliveryFee)(params);
            return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
        });
    }
    /**
     * 计算带促销的配送费
     */
    static calculateWithPromotions(merchantId, userAddressId, orderAmount, promotionCodes) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
                promotion_codes: promotionCodes
            });
            const result = yield (0, exports.calculateDeliveryFee)(params);
            return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
        });
    }
    /**
     * 计算高峰时段配送费
     */
    static calculatePeakHourFee(merchantId, userAddressId, orderAmount, deliveryDistance) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
                peak_hour: true,
                delivery_distance: deliveryDistance
            });
            const result = yield (0, exports.calculateDeliveryFee)(params);
            return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
        });
    }
    /**
     * 格式化配送费显示
     */
    static formatDeliveryFee(result) {
        const breakdown = [];
        if (result.breakdown.base_fee > 0) {
            breakdown.push(`起送费: ¥${result.breakdown.base_fee.toFixed(2)}`);
        }
        if (result.breakdown.distance_fee > 0) {
            breakdown.push(`距离费: ¥${result.breakdown.distance_fee.toFixed(2)}`);
        }
        if (result.breakdown.peak_hour_fee > 0) {
            breakdown.push(`高峰费: ¥${result.breakdown.peak_hour_fee.toFixed(2)}`);
        }
        const hasDiscount = result.breakdown.promotion_discount > 0;
        if (hasDiscount) {
            breakdown.push(`优惠减免: -¥${result.breakdown.promotion_discount.toFixed(2)}`);
        }
        const displayText = result.final_fee === 0
            ? '免配送费'
            : `配送费 ¥${result.final_fee.toFixed(2)}`;
        return {
            displayText,
            breakdown,
            hasDiscount
        };
    }
}
exports.DeliveryFeeUtils = DeliveryFeeUtils;
// ==================== 支付状态管理 ====================
/**
 * 支付状态管理器
 */
class PaymentStatusManager {
    /**
     * 开始轮询支付状态
     */
    static startPaymentPolling(paymentId, onStatusChange, interval = 3000, maxAttempts = 60) {
        let attempts = 0;
        const poll = () => __awaiter(this, void 0, void 0, function* () {
            try {
                attempts++;
                const payment = yield PaymentUtils.checkPaymentStatus(paymentId);
                onStatusChange(payment);
                // 如果支付完成或失败，停止轮询
                if (payment.status === 'paid' || payment.status === 'failed' || payment.status === 'cancelled') {
                    this.stopPaymentPolling(paymentId);
                    return;
                }
                // 达到最大尝试次数，停止轮询
                if (attempts >= maxAttempts) {
                    this.stopPaymentPolling(paymentId);
                    return;
                }
                // 继续轮询
                const timeoutId = setTimeout(poll, interval);
                this.paymentPollingMap.set(paymentId, timeoutId);
            }
            catch (error) {
                console.error('轮询支付状态失败:', error);
                this.stopPaymentPolling(paymentId);
            }
        });
        // 开始轮询
        poll();
    }
    /**
     * 停止轮询支付状态
     */
    static stopPaymentPolling(paymentId) {
        const timeoutId = this.paymentPollingMap.get(paymentId);
        if (timeoutId) {
            clearTimeout(timeoutId);
            this.paymentPollingMap.delete(paymentId);
        }
    }
    /**
     * 停止所有轮询
     */
    static stopAllPolling() {
        this.paymentPollingMap.forEach((timeoutId) => {
            clearTimeout(timeoutId);
        });
        this.paymentPollingMap.clear();
    }
}
exports.PaymentStatusManager = PaymentStatusManager;
PaymentStatusManager.paymentPollingMap = new Map();
exports.default = {
    // 支付接口
    createPayment: exports.createPayment,
    getPayments: exports.getPayments,
    getPaymentById: exports.getPaymentById,
    closePayment: exports.closePayment,
    getPaymentRefunds: exports.getPaymentRefunds,
    // 退款接口
    createRefund: exports.createRefund,
    getRefundById: exports.getRefundById,
    // 配送费接口
    calculateDeliveryFee: exports.calculateDeliveryFee,
    // 适配器
    PaymentAdapter,
    RefundAdapter,
    DeliveryFeeAdapter,
    // 便捷函数
    PaymentUtils,
    RefundUtils,
    DeliveryFeeUtils,
    // 状态管理
    PaymentStatusManager
};
