/**
 * 支付和退款接口模块
 * 基于swagger.json完全重构，提供支付流程、退款处理和配送费计算
 */

import { request } from '../utils/request';

// ==================== 类型定义 ====================

// 支付相关类型
export interface CreatePaymentParams {
    order_id: number;
    payment_method: 'wechat_pay' | 'alipay' | 'balance' | 'credit';
    amount: number;
    description?: string;
    return_url?: string;
    notify_url?: string;
}

export interface PaymentListParams {
    status?: 'pending' | 'paid' | 'failed' | 'cancelled' | 'refunded';
    order_id?: number;
    start_date?: string;
    end_date?: string;
    page?: number;
    page_size?: number;
}

// 退款相关类型
export interface CreateRefundParams {
    payment_id: number;
    amount: number;
    reason: string;
    refund_type: 'full' | 'partial';
    operator_id?: number;
}

// 配送费计算类型
export interface DeliveryFeeCalculateParams {
    merchant_id: number;
    user_address_id: number;
    order_amount: number;
    delivery_distance?: number;
    peak_hour?: boolean;
    promotion_codes?: string[];
}

// 响应类型
export interface Payment {
    id: number;
    order_id: number;
    payment_method: string;
    amount: number;
    status: 'pending' | 'paid' | 'failed' | 'cancelled' | 'refunded';
    transaction_id?: string;
    payment_url?: string;
    qr_code?: string;
    description: string;
    created_at: string;
    paid_at?: string;
    expired_at?: string;
    refund_amount?: number;
    refund_status?: 'none' | 'partial' | 'full';
}

export interface PaymentResult {
    payment: Payment;
    payment_info?: {
        payment_url?: string;
        qr_code?: string;
        app_pay_params?: any;
        jsapi_params?: any;
    };
}

export interface Refund {
    id: number;
    payment_id: number;
    amount: number;
    reason: string;
    refund_type: 'full' | 'partial';
    status: 'pending' | 'processing' | 'success' | 'failed';
    refund_transaction_id?: string;
    operator_id?: number;
    created_at: string;
    processed_at?: string;
    failed_reason?: string;
}

/** 配送费计算结果 - 对齐 api.DeliveryFeeResult */
export interface DeliveryFeeResult {
    baseFee?: number;                            // 基础配送费
    deliverySuspended?: boolean;                 // 是否暂停配送
    distanceFee?: number;                        // 距离费
    finalFee?: number;                           // 最终费用
    peakHourCoefficient?: number;                // 高峰时段系数
    promotionDiscount?: number;                  // 促销折扣
    subtotalFee?: number;                        // 小计费用
    suspendReason?: string;                      // 暂停原因
    valueFee?: number;                           // 价值费
    weatherCoefficient?: number;                 // 天气系数
}

/** 计算配送费请求 - 对齐 api.calculateDeliveryFeeRequest */
export interface CalculateDeliveryFeeRequest extends Record<string, unknown> {
    distance: number;                            // 配送距离（米，必填）
    merchant_id: number;                         // 商户ID（必填）
    order_amount: number;                        // 订单金额（分，必填）
    region_id: number;                           // 区域ID（必填）
}

/** 创建支付订单请求 - 对齐 api.createPaymentOrderRequest */
export interface CreatePaymentOrderRequest extends Record<string, unknown> {
    order_id: number;                            // 订单ID（必填）
    business_type: 'order' | 'reservation';      // 业务类型（必填）
    payment_type: 'native' | 'miniprogram';      // 支付类型（必填）
}

/** 创建退款订单请求 - 对齐 api.createRefundOrderRequest */
export interface CreateRefundOrderRequest extends Record<string, unknown> {
    payment_order_id: number;                    // 支付订单ID（必填）
    refund_amount: number;                       // 退款金额（分，必填）
    refund_type: 'full' | 'partial';             // 退款类型（必填）
    refund_reason?: string;                      // 退款原因
}

/** 退款订单响应 - 对齐 api.refundOrderResponse */
export interface RefundOrderResponse {
    id: number;                                  // 退款订单ID
    payment_order_id: number;                    // 支付订单ID
    out_refund_no: string;                       // 退款单号
    refund_amount: number;                       // 退款金额（分）
    refund_type: string;                         // 退款类型
    refund_reason?: string;                      // 退款原因
    status: string;                              // 退款状态
    refunded_at?: string;                        // 退款时间
    created_at: string;                          // 创建时间
}

/** 创建配送促销请求 - 对齐 api.createDeliveryPromotionRequest */
export interface CreateDeliveryPromotionRequest extends Record<string, unknown> {
    name: string;                                // 促销名称（1-50字符，必填）
    min_order_amount: number;                    // 最低订单金额（分，0-100000000，必填）
    discount_amount: number;                     // 折扣金额（分，最大10000000，必填）
    valid_from: string;                          // 有效期开始（必填）
    valid_until: string;                         // 有效期结束（必填）
}

/** 配送促销响应 - 对齐 api.deliveryPromotionResponse */
export interface DeliveryPromotionResponse {
    id: number;                                  // 促销ID
    merchant_id: number;                         // 商户ID
    name: string;                                // 促销名称
    min_order_amount: number;                    // 最低订单金额（分）
    discount_amount: number;                     // 折扣金额（分）
    valid_from: string;                          // 有效期开始
    valid_until: string;                         // 有效期结束
    is_active: boolean;                          // 是否激活
    created_at: string;                          // 创建时间
    updated_at: string;                          // 更新时间
}

export interface PaginatedResponse<T> {
    data: T[];
    pagination: {
        page: number;
        page_size: number;
        total: number;
        total_pages: number;
    };
}

// ==================== API 接口函数 ====================

/**
 * 创建支付订单
 */
export const createPayment = async (params: CreatePaymentParams): Promise<PaymentResult> => {
    return request({
        url: '/v1/payments',
        method: 'POST',
        data: params
    });
};

/**
 * 获取支付列表
 */
export const getPayments = async (params?: PaymentListParams): Promise<PaginatedResponse<Payment>> => {
    return request({
        url: '/v1/payments',
        method: 'GET',
        data: params
    });
};

/**
 * 获取支付详情
 */
export const getPaymentById = async (id: number): Promise<Payment> => {
    return request({
        url: `/v1/payments/${id}`,
        method: 'GET'
    });
};

/**
 * 关闭支付订单
 */
export const closePayment = async (id: number): Promise<{ success: boolean; message: string }> => {
    return request({
        url: `/v1/payments/${id}/close`,
        method: 'POST'
    });
};

/**
 * 获取支付的退款记录
 */
export const getPaymentRefunds = async (paymentId: number): Promise<Refund[]> => {
    return request({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'GET'
    });
};

/**
 * 创建退款申请
 */
export const createRefund = async (params: CreateRefundParams): Promise<Refund> => {
    return request({
        url: '/v1/refunds',
        method: 'POST',
        data: params
    });
};

/**
 * 获取退款详情
 */
export const getRefundById = async (id: number): Promise<Refund> => {
    return request({
        url: `/v1/refunds/${id}`,
        method: 'GET'
    });
};

/**
 * 计算配送费
 */
export const calculateDeliveryFee = async (params: DeliveryFeeCalculateParams): Promise<DeliveryFeeResult> => {
    return request({
        url: '/delivery-fee/calculate',
        method: 'POST',
        data: params
    });
};

// ==================== 数据适配器 ====================

/**
 * 支付数据适配器
 */
export class PaymentAdapter {
    /**
     * 适配支付数据
     */
    static adaptPayment(payment: Payment): Payment {
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
    static adaptPaymentResult(result: PaymentResult): PaymentResult {
        return {
            payment: this.adaptPayment(result.payment),
            payment_info: result.payment_info
        };
    }

    /**
     * 构建支付参数
     */
    static buildPaymentParams(
        orderId: number,
        paymentMethod: 'wechat_pay' | 'alipay' | 'balance' | 'credit',
        amount: number,
        description?: string
    ): CreatePaymentParams {
        return {
            order_id: orderId,
            payment_method: paymentMethod,
            amount,
            description: description || `订单 ${orderId} 支付`
        };
    }
}

/**
 * 退款数据适配器
 */
export class RefundAdapter {
    /**
     * 适配退款数据
     */
    static adaptRefund(refund: Refund): Refund {
        return {
            ...refund,
            id: Number(refund.id),
            payment_id: Number(refund.payment_id),
            amount: Number(refund.amount),
            operator_id: refund.operator_id ? Number(refund.operator_id) : undefined,
            created_at: refund.created_at,
            processed_at: refund.processed_at || undefined
        };
    }

    /**
     * 构建退款参数
     */
    static buildRefundParams(
        paymentId: number,
        amount: number,
        reason: string,
        refundType: 'full' | 'partial' = 'full',
        operatorId?: number
    ): CreateRefundParams {
        return {
            payment_id: paymentId,
            amount,
            reason,
            refund_type: refundType,
            operator_id: operatorId
        };
    }
}

/**
 * 配送费适配器
 */
export class DeliveryFeeAdapter {
    /**
     * 适配配送费结果
     */
    static adaptDeliveryFeeResult(result: DeliveryFeeResult): DeliveryFeeResult {
        return {
            ...result,
            base_fee: Number(result.base_fee),
            distance_fee: Number(result.distance_fee),
            peak_hour_fee: Number(result.peak_hour_fee),
            promotion_discount: Number(result.promotion_discount),
            final_fee: Number(result.final_fee),
            breakdown: {
                base_fee: Number(result.breakdown.base_fee),
                distance_fee: Number(result.breakdown.distance_fee),
                peak_hour_fee: Number(result.breakdown.peak_hour_fee),
                total_before_discount: Number(result.breakdown.total_before_discount),
                promotion_discount: Number(result.breakdown.promotion_discount),
                final_fee: Number(result.breakdown.final_fee)
            },
            promotions_applied: result.promotions_applied.map(promo => ({
                ...promo,
                discount_amount: Number(promo.discount_amount)
            }))
        };
    }

    /**
     * 构建配送费计算参数
     */
    static buildCalculateParams(
        merchantId: number,
        userAddressId: number,
        orderAmount: number,
        options?: {
            deliveryDistance?: number;
            peakHour?: boolean;
            promotionCodes?: string[];
        }
    ): DeliveryFeeCalculateParams {
        return {
            merchant_id: merchantId,
            user_address_id: userAddressId,
            order_amount: orderAmount,
            delivery_distance: options?.deliveryDistance,
            peak_hour: options?.peakHour,
            promotion_codes: options?.promotionCodes
        };
    }
}

// ==================== 便捷函数 ====================

/**
 * 支付便捷函数
 */
export class PaymentUtils {
    /**
     * 创建微信支付
     */
    static async createWechatPayment(orderId: number, amount: number, description?: string): Promise<PaymentResult> {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'wechat_pay', amount, description);
        const result = await createPayment(params);
        return PaymentAdapter.adaptPaymentResult(result);
    }

    /**
     * 创建支付宝支付
     */
    static async createAlipayPayment(orderId: number, amount: number, description?: string): Promise<PaymentResult> {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'alipay', amount, description);
        const result = await createPayment(params);
        return PaymentAdapter.adaptPaymentResult(result);
    }

    /**
     * 余额支付
     */
    static async createBalancePayment(orderId: number, amount: number, description?: string): Promise<PaymentResult> {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'balance', amount, description);
        const result = await createPayment(params);
        return PaymentAdapter.adaptPaymentResult(result);
    }

    /**
     * 检查支付状态
     */
    static async checkPaymentStatus(paymentId: number): Promise<Payment> {
        const payment = await getPaymentById(paymentId);
        return PaymentAdapter.adaptPayment(payment);
    }

    /**
     * 获取用户支付历史
     */
    static async getUserPaymentHistory(
        status?: 'pending' | 'paid' | 'failed' | 'cancelled' | 'refunded',
        page: number = 1,
        pageSize: number = 20
    ): Promise<PaginatedResponse<Payment>> {
        const result = await getPayments({
            status,
            page,
            page_size: pageSize
        });

        return {
            ...result,
            data: result.data.map(payment => PaymentAdapter.adaptPayment(payment))
        };
    }
}

/**
 * 退款便捷函数
 */
export class RefundUtils {
    /**
     * 申请全额退款
     */
    static async requestFullRefund(paymentId: number, reason: string): Promise<Refund> {
        // 先获取支付信息确定退款金额
        const payment = await getPaymentById(paymentId);
        const params = RefundAdapter.buildRefundParams(paymentId, payment.amount, reason, 'full');
        const refund = await createRefund(params);
        return RefundAdapter.adaptRefund(refund);
    }

    /**
     * 申请部分退款
     */
    static async requestPartialRefund(paymentId: number, amount: number, reason: string): Promise<Refund> {
        const params = RefundAdapter.buildRefundParams(paymentId, amount, reason, 'partial');
        const refund = await createRefund(params);
        return RefundAdapter.adaptRefund(refund);
    }

    /**
     * 获取支付的退款记录
     */
    static async getRefundHistory(paymentId: number): Promise<Refund[]> {
        const refunds = await getPaymentRefunds(paymentId);
        return refunds.map(refund => RefundAdapter.adaptRefund(refund));
    }

    /**
     * 检查退款状态
     */
    static async checkRefundStatus(refundId: number): Promise<Refund> {
        const refund = await getRefundById(refundId);
        return RefundAdapter.adaptRefund(refund);
    }
}

/**
 * 配送费便捷函数
 */
export class DeliveryFeeUtils {
    /**
     * 快速计算配送费
     */
    static async quickCalculate(
        merchantId: number,
        userAddressId: number,
        orderAmount: number
    ): Promise<DeliveryFeeResult> {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount);
        const result = await calculateDeliveryFee(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }

    /**
     * 计算带促销的配送费
     */
    static async calculateWithPromotions(
        merchantId: number,
        userAddressId: number,
        orderAmount: number,
        promotionCodes: string[]
    ): Promise<DeliveryFeeResult> {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
            promotion_codes: promotionCodes
        });
        const result = await calculateDeliveryFee(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }

    /**
     * 计算高峰时段配送费
     */
    static async calculatePeakHourFee(
        merchantId: number,
        userAddressId: number,
        orderAmount: number,
        deliveryDistance?: number
    ): Promise<DeliveryFeeResult> {
        const params = DeliveryFeeAdapter.buildCalculateParams(merchantId, userAddressId, orderAmount, {
            peak_hour: true,
            delivery_distance: deliveryDistance
        });
        const result = await calculateDeliveryFee(params);
        return DeliveryFeeAdapter.adaptDeliveryFeeResult(result);
    }

    /**
     * 格式化配送费显示
     */
    static formatDeliveryFee(result: DeliveryFeeResult): {
        displayText: string;
        breakdown: string[];
        hasDiscount: boolean;
    } {
        const breakdown: string[] = [];

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

// ==================== 支付状态管理 ====================

/**
 * 支付状态管理器
 */
export class PaymentStatusManager {
    private static paymentPollingMap = new Map<number, NodeJS.Timeout>();

    /**
     * 开始轮询支付状态
     */
    static startPaymentPolling(
        paymentId: number,
        onStatusChange: (payment: Payment) => void,
        interval: number = 3000,
        maxAttempts: number = 60
    ): void {
        let attempts = 0;

        const poll = async () => {
            try {
                attempts++;
                const payment = await PaymentUtils.checkPaymentStatus(paymentId);
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

            } catch (error) {
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
    static stopPaymentPolling(paymentId: number): void {
        const timeoutId = this.paymentPollingMap.get(paymentId);
        if (timeoutId) {
            clearTimeout(timeoutId);
            this.paymentPollingMap.delete(paymentId);
        }
    }

    /**
     * 停止所有轮询
     */
    static stopAllPolling(): void {
        this.paymentPollingMap.forEach((timeoutId) => {
            clearTimeout(timeoutId);
        });
        this.paymentPollingMap.clear();
    }
}

export default {
    // 支付接口
    createPayment,
    getPayments,
    getPaymentById,
    closePayment,
    getPaymentRefunds,

    // 退款接口
    createRefund,
    getRefundById,

    // 配送费接口
    calculateDeliveryFee,

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