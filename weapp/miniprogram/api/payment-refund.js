"use strict";
/**
 * 支付和退款接口模块
 * 基于swagger.json完全重构，提供支付流程、退款处理和配送费计算
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.PaymentStatusManager = exports.DeliveryFeeUtils = exports.RefundUtils = exports.PaymentUtils = exports.DeliveryFeeAdapter = exports.RefundAdapter = exports.PaymentAdapter = exports.getRefundById = exports.calculateDeliveryFee = exports.createRefund = exports.createPayment = exports.getPaymentRefunds = exports.closePayment = exports.getPaymentById = exports.getPayments = void 0;
const request_1 = require("../utils/request");
// ==================== API 接口函数 ====================
/**
 * 获取支付订单列表
 */
const getPayments = async (params) => {
    return (0, request_1.request)({
        url: '/v1/payments',
        method: 'GET',
        data: params
    });
};
exports.getPayments = getPayments;
/**
 * 获取支付详情
 */
const getPaymentById = async (id) => {
    return (0, request_1.request)({
        url: `/v1/payments/${id}`,
        method: 'GET'
    });
};
exports.getPaymentById = getPaymentById;
/**
 * 关闭支付订单
 */
const closePayment = async (id) => {
    return (0, request_1.request)({
        url: `/v1/payments/${id}/close`,
        method: 'POST'
    });
};
exports.closePayment = closePayment;
/**
 * 获取支付的退款记录
 */
const getPaymentRefunds = async (paymentId) => {
    return (0, request_1.request)({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'GET'
    });
};
exports.getPaymentRefunds = getPaymentRefunds;
/**
 * 创建支付订单
 */
const createPayment = async (params) => {
    return (0, request_1.request)({
        url: '/v1/payments',
        method: 'POST',
        data: params
    });
};
exports.createPayment = createPayment;
/**
 * 创建退款订单
 */
const createRefund = async (params) => {
    const data = 'payment_id' in params ? {
        payment_order_id: params.payment_id,
        refund_amount: params.amount,
        refund_reason: params.reason,
        refund_type: params.refund_type
    } : params;
    return (0, request_1.request)({
        url: '/v1/refunds',
        method: 'POST',
        data
    });
};
exports.createRefund = createRefund;
/**
 * 计算配送费
 */
const calculateDeliveryFee = async (params) => {
    return (0, request_1.request)({
        url: '/v1/delivery-fee/calculate',
        method: 'POST',
        data: params
    });
};
exports.calculateDeliveryFee = calculateDeliveryFee;
/**
 * 获取退款详情
 */
const getRefundById = async (id) => {
    return (0, request_1.request)({
        url: `/v1/refunds/${id}`,
        method: 'GET'
    });
};
exports.getRefundById = getRefundById;
// ==================== 数据适配器 ====================
/**
 * 支付数据适配器
 */
class PaymentAdapter {
    /**
     * 适配支付数据
     */
    static adaptPayment(payment) {
        return {
            ...payment,
            id: Number(payment.id),
            order_id: Number(payment.order_id),
            amount: Number(payment.amount),
            refund_amount: payment.refund_amount ? Number(payment.refund_amount) : undefined,
            created_at: payment.created_at,
            paid_at: payment.paid_at || undefined,
            expired_at: payment.expired_at || undefined
        };
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
        var _a, _b;
        const paymentId = (_a = refund.payment_id) !== null && _a !== void 0 ? _a : refund.payment_order_id;
        const amount = (_b = refund.amount) !== null && _b !== void 0 ? _b : refund.refund_amount;
        return {
            ...refund,
            id: Number(refund.id),
            payment_id: paymentId ? Number(paymentId) : undefined,
            amount: amount ? Number(amount) : 0,
            operator_id: refund.operator_id ? Number(refund.operator_id) : undefined,
            created_at: refund.created_at,
            processed_at: refund.processed_at || refund.refunded_at || undefined
        };
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
        const breakdown = result.breakdown || {
            base_fee: 0,
            distance_fee: 0,
            peak_hour_fee: 0,
            total_before_discount: 0,
            promotion_discount: 0,
            final_fee: 0
        };
        const promotionsApplied = result.promotions_applied || [];
        return {
            ...result,
            base_fee: Number(result.base_fee || 0),
            distance_fee: Number(result.distance_fee || 0),
            peak_hour_fee: Number(result.peak_hour_fee || 0),
            promotion_discount: Number(result.promotion_discount || 0),
            final_fee: Number(result.final_fee || 0),
            breakdown: {
                base_fee: Number(breakdown.base_fee),
                distance_fee: Number(breakdown.distance_fee),
                peak_hour_fee: Number(breakdown.peak_hour_fee),
                total_before_discount: Number(breakdown.total_before_discount),
                promotion_discount: Number(breakdown.promotion_discount),
                final_fee: Number(breakdown.final_fee)
            },
            promotions_applied: promotionsApplied.map(promo => ({
                ...promo,
                discount_amount: Number(promo.discount_amount)
            }))
        };
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
    static async createWechatPayment(orderId, amount, description) {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'wechat_pay', amount, description);
        const result = await (0, exports.createPayment)(params);
        return PaymentAdapter.adaptPaymentResult({ payment: result });
    }
    /**
     * 创建支付宝支付
     */
    static async createAlipayPayment(orderId, amount, description) {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'alipay', amount, description);
        const result = await (0, exports.createPayment)(params);
        return PaymentAdapter.adaptPaymentResult({ payment: result });
    }
    /**
     * 余额支付
     */
    static async createBalancePayment(orderId, amount, description) {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'balance', amount, description);
        const result = await (0, exports.createPayment)(params);
        return PaymentAdapter.adaptPaymentResult({ payment: result });
    }
    /**
     * 检查支付状态
     */
    static async checkPaymentStatus(paymentId) {
        const payment = await (0, exports.getPaymentById)(paymentId);
        return PaymentAdapter.adaptPayment(payment);
    }
    /**
     * 获取用户支付历史
     */
    static async getUserPaymentHistory(status, page = 1, pageSize = 20) {
        const result = await (0, exports.getPayments)({
            status: status,
            page,
            page_size: pageSize
        });
        return {
            data: result.payment_orders.map(payment => PaymentAdapter.adaptPayment(payment)),
            total: result.total,
            page: result.page_id || page,
            page_size: result.page_size || pageSize
        };
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
    static async requestFullRefund(paymentId, reason) {
        // 先获取支付信息确定退款金额
        const payment = await (0, exports.getPaymentById)(paymentId);
        const params = RefundAdapter.buildRefundParams(paymentId, payment.amount, reason, 'full');
        const refund = await (0, exports.createRefund)(params);
        return RefundAdapter.adaptRefund(refund);
    }
    /**
     * 申请部分退款
     */
    static async requestPartialRefund(paymentId, amount, reason) {
        const params = RefundAdapter.buildRefundParams(paymentId, amount, reason, 'partial');
        const refund = await (0, exports.createRefund)(params);
        return RefundAdapter.adaptRefund(refund);
    }
    /**
     * 获取支付的退款记录
     */
    static async getRefundHistory(paymentId) {
        const refunds = await (0, exports.getPaymentRefunds)(paymentId);
        return refunds.refund_orders.map(refund => RefundAdapter.adaptRefund(refund));
    }
    /**
     * 检查退款状态
     */
    static async checkRefundStatus(refundId) {
        const refund = await (0, exports.getRefundById)(refundId);
        return RefundAdapter.adaptRefund(refund);
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
    static async quickCalculate(merchantId, userAddressId, orderAmount) {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount);
        const result = await (0, exports.calculateDeliveryFee)(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }
    /**
     * 计算带促销的配送费
     */
    static async calculateWithPromotions(merchantId, userAddressId, orderAmount, promotionCodes) {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
            promotionCodes: promotionCodes
        });
        const result = await (0, exports.calculateDeliveryFee)(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }
    /**
     * 计算高峰时段配送费
     */
    static async calculatePeakHourFee(merchantId, userAddressId, orderAmount, deliveryDistance) {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
            peakHour: true,
            deliveryDistance: deliveryDistance
        });
        const result = await (0, exports.calculateDeliveryFee)(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }
    /**
     * 格式化配送费显示
     */
    static formatDeliveryFee(result) {
        var _a;
        const breakdown = [];
        const breakdownDetail = result.breakdown || {
            base_fee: 0,
            distance_fee: 0,
            peak_hour_fee: 0,
            total_before_discount: 0,
            promotion_discount: 0,
            final_fee: 0
        };
        if (breakdownDetail.base_fee > 0) {
            breakdown.push(`起送费: ¥${breakdownDetail.base_fee.toFixed(2)}`);
        }
        if (breakdownDetail.distance_fee > 0) {
            breakdown.push(`距离费: ¥${breakdownDetail.distance_fee.toFixed(2)}`);
        }
        if (breakdownDetail.peak_hour_fee > 0) {
            breakdown.push(`高峰费: ¥${breakdownDetail.peak_hour_fee.toFixed(2)}`);
        }
        const hasDiscount = breakdownDetail.promotion_discount > 0;
        if (hasDiscount) {
            breakdown.push(`优惠减免: -¥${breakdownDetail.promotion_discount.toFixed(2)}`);
        }
        const finalFee = (_a = result.final_fee) !== null && _a !== void 0 ? _a : 0;
        const displayText = finalFee === 0
            ? '免配送费'
            : `配送费 ¥${finalFee.toFixed(2)}`;
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
        const poll = async () => {
            try {
                attempts++;
                const payment = await PaymentUtils.checkPaymentStatus(paymentId);
                onStatusChange(payment);
                // 如果支付完成或终态，停止轮询
                if (payment.status === 'paid' || payment.status === 'refunded' || payment.status === 'closed') {
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
        };
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
