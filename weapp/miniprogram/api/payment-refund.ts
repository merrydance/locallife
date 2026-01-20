/**
 * 支付和退款接口模块
 * 基于swagger.json完全重构，提供支付流程、退款处理和配送费计算
 */

import { request } from '../utils/request';

// ==================== 类型定义 ====================

export type PaymentStatus = 'pending' | 'paid' | 'refunded' | 'closed' | 'failed' | 'cancelled'
export type PaymentType = 'native' | 'miniprogram'
export type BusinessType = 'order' | 'reservation'

export interface MiniProgramPayParams {
    timeStamp: string
    nonceStr: string
    package: string
    signType: string
    paySign: string
}

export interface PaymentOrder {
    id: number
    order_id?: number
    user_id: number
    payment_type: PaymentType
    business_type: BusinessType
    amount: number
    out_trade_no: string
    status: PaymentStatus
    prepay_id?: string
    pay_params?: MiniProgramPayParams
    paid_at?: string
    created_at: string
}

export interface Payment extends PaymentOrder {
    refund_amount?: number
    expired_at?: string
}

export interface RefundOrder {
    id: number
    payment_order_id: number
    refund_type: 'full' | 'partial'
    refund_amount: number
    refund_reason?: string
    out_refund_no: string
    status: string
    refunded_at?: string
    created_at: string
    payment_id?: number
    amount?: number
    operator_id?: number
    processed_at?: string
}

export type Refund = RefundOrder

export interface PaymentResult {
    payment: Payment
    payment_info?: MiniProgramPayParams
}

export interface PaginatedResponse<T> {
    data: T[]
    total: number
    page: number
    page_size: number
}

export interface CreatePaymentParams {
    order_id: number
    payment_method: 'wechat_pay' | 'alipay' | 'balance' | 'credit'
    amount: number
    description?: string
}

export interface CreateRefundParams {
    payment_id: number
    amount: number
    reason: string
    refund_type: 'full' | 'partial'
    operator_id?: number
}

export interface DeliveryFeeCalculateParams extends CalculateDeliveryFeeRequest {}

export interface DeliveryFeeBreakdown {
    base_fee: number
    distance_fee: number
    peak_hour_fee: number
    total_before_discount: number
    promotion_discount: number
    final_fee: number
}

export interface DeliveryPromotionApplied {
    code: string
    discount_amount: number
    description?: string
}

/** 配送费计算结果 - 对齐 api.DeliveryFeeResult */
export interface DeliveryFeeResult {
    base_fee?: number
    distance_fee?: number
    peak_hour_fee?: number
    promotion_discount?: number
    final_fee?: number
    delivery_distance?: number
    delivery_suspended?: boolean
    suspend_reason?: string
    breakdown?: DeliveryFeeBreakdown
    promotions_applied?: DeliveryPromotionApplied[]
}

/** 计算配送费请求 - 对齐 api.calculateDeliveryFeeRequest */
export interface CalculateDeliveryFeeRequest extends Record<string, unknown> {
    merchant_id: number
    user_address_id: number
    order_amount: number
    delivery_distance?: number
    peak_hour?: boolean
    promotion_codes?: string[]
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

export interface ListPaymentOrdersParams {
    page?: number
    page_size?: number
    order_id?: number
    status?: PaymentStatus
}

export interface ListPaymentOrdersResponse {
    payment_orders: PaymentOrder[]
    total_count: number
    total: number
    page_id: number
    page_size: number
}

export interface ListRefundOrdersByPaymentResponse {
    refund_orders: RefundOrder[]
    total_count: number
    total: number
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

// ==================== API 接口函数 ====================

/**
 * 获取支付订单列表
 */
export const getPayments = async (params?: ListPaymentOrdersParams): Promise<ListPaymentOrdersResponse> => {
    return request({
        url: '/v1/payments',
        method: 'GET',
        data: params
    })
}

/**
 * 获取支付详情
 */
export const getPaymentById = async (id: number): Promise<PaymentOrder> => {
    return request({
        url: `/v1/payments/${id}`,
        method: 'GET'
    })
}

/**
 * 关闭支付订单
 */
export const closePayment = async (id: number): Promise<PaymentOrder> => {
    return request({
        url: `/v1/payments/${id}/close`,
        method: 'POST'
    })
}

/**
 * 获取支付的退款记录
 */
export const getPaymentRefunds = async (paymentId: number): Promise<ListRefundOrdersByPaymentResponse> => {
    return request({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'GET'
    })
}

/**
 * 创建支付订单
 */
export const createPayment = async (params: CreatePaymentOrderRequest | CreatePaymentParams): Promise<PaymentOrder> => {
    return request({
        url: '/v1/payments',
        method: 'POST',
        data: params
    })
}

/**
 * 创建退款订单
 */
export const createRefund = async (params: CreateRefundOrderRequest | CreateRefundParams): Promise<RefundOrder> => {
    const data = 'payment_id' in params ? {
        payment_order_id: params.payment_id,
        refund_amount: params.amount,
        refund_reason: params.reason,
        refund_type: params.refund_type
    } : params
    return request({
        url: '/v1/refunds',
        method: 'POST',
        data
    })
}

/**
 * 计算配送费
 */
export const calculateDeliveryFee = async (params: CalculateDeliveryFeeRequest): Promise<DeliveryFeeResult> => {
    return request({
        url: '/v1/delivery-fee/calculate',
        method: 'POST',
        data: params
    })
}

/**
 * 获取退款详情
 */
export const getRefundById = async (id: number): Promise<RefundOrder> => {
    return request({
        url: `/v1/refunds/${id}`,
        method: 'GET'
    })
}

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
        const paymentId = refund.payment_id ?? refund.payment_order_id
        const amount = refund.amount ?? refund.refund_amount
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
        const breakdown = result.breakdown || {
            base_fee: 0,
            distance_fee: 0,
            peak_hour_fee: 0,
            total_before_discount: 0,
            promotion_discount: 0,
            final_fee: 0
        }
        const promotionsApplied = result.promotions_applied || []
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
        return PaymentAdapter.adaptPaymentResult({ payment: result });
    }

    /**
     * 创建支付宝支付
     */
    static async createAlipayPayment(orderId: number, amount: number, description?: string): Promise<PaymentResult> {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'alipay', amount, description);
        const result = await createPayment(params);
        return PaymentAdapter.adaptPaymentResult({ payment: result });
    }

    /**
     * 余额支付
     */
    static async createBalancePayment(orderId: number, amount: number, description?: string): Promise<PaymentResult> {
        const params = PaymentAdapter.buildPaymentParams(orderId, 'balance', amount, description);
        const result = await createPayment(params);
        return PaymentAdapter.adaptPaymentResult({ payment: result });
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
            status: status as PaymentStatus | undefined,
            page,
            page_size: pageSize
        });

        return {
            data: result.payment_orders.map(payment => PaymentAdapter.adaptPayment(payment as Payment)),
            total: result.total,
            page: result.page_id || page,
            page_size: result.page_size || pageSize
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
        return refunds.refund_orders.map(refund => RefundAdapter.adaptRefund(refund));
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
            promotionCodes: promotionCodes
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
            peakHour: true,
            deliveryDistance: deliveryDistance
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

        const breakdownDetail = result.breakdown || {
            base_fee: 0,
            distance_fee: 0,
            peak_hour_fee: 0,
            total_before_discount: 0,
            promotion_discount: 0,
            final_fee: 0
        }

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

        const finalFee = result.final_fee ?? 0
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

// ==================== 支付状态管理 ====================

/**
 * 支付状态管理器
 */
export class PaymentStatusManager {
    private static paymentPollingMap = new Map<number, ReturnType<typeof setTimeout>>();

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