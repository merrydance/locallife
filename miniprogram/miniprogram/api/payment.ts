/**
 * 支付相关API接口
 * 基于swagger.json中的支付管理接口
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 支付状态枚举 */
export type PaymentStatus =
    | 'pending'   // 待支付
    | 'paid'      // 已支付
    | 'failed'    // 支付失败
    | 'cancelled' // 已取消
    | 'refunded'  // 已退款

/** 支付类型枚举 */
export type PaymentType =
    | 'native'      // 扫码支付
    | 'miniprogram' // 小程序支付

/** 业务类型枚举 */
export type BusinessType =
    | 'order'       // 订单支付
    | 'reservation' // 预定押金

/** 小程序支付参数 */
export interface MiniProgramPayParams {
    timeStamp: string
    nonceStr: string
    package: string
    signType: string
    paySign: string
}

/** 支付订单响应 */
export interface PaymentOrderResponse {
    id: number
    user_id: number
    order_id: number
    out_trade_no: string
    prepay_id: string
    amount: number
    status: PaymentStatus
    payment_type: PaymentType
    business_type: BusinessType
    pay_params?: MiniProgramPayParams
    created_at: string
    paid_at?: string
}

/** 创建支付请求 */
export interface CreatePaymentRequest {
    order_id: number
    payment_type: PaymentType
    business_type: BusinessType
    return_url?: string
}

/** 退款响应 */
export interface RefundResponse {
    id: number
    payment_id: number
    refund_amount: number
    refund_reason: string
    status: 'pending' | 'success' | 'failed'
    refund_no: string
    created_at: string
    refunded_at?: string
}

/** 创建退款请求 */
export interface CreateRefundRequest {
    refund_amount: number
    refund_reason: string
}

/** 支付列表查询参数 */
export interface ListPaymentsParams {
    page_id: number
    page_size: number
    status?: PaymentStatus
    business_type?: BusinessType
}

// ==================== API接口函数 ====================

/**
 * 获取支付订单列表
 * @param params 查询参数
 */
export async function getPaymentList(params: ListPaymentsParams): Promise<PaymentOrderResponse[]> {
    return request({
        url: '/v1/payments',
        method: 'GET',
        data: params
    })
}

/**
 * 获取支付订单详情
 * @param paymentId 支付订单ID
 */
export async function getPaymentDetail(paymentId: number): Promise<PaymentOrderResponse> {
    return request({
        url: `/v1/payments/${paymentId}`,
        method: 'GET'
    })
}

/**
 * 创建支付订单
 * @param paymentData 支付数据
 */
export async function createPayment(paymentData: CreatePaymentRequest): Promise<PaymentOrderResponse> {
    return request({
        url: '/v1/payments',
        method: 'POST',
        data: paymentData
    })
}

/**
 * 关闭支付订单
 * @param paymentId 支付订单ID
 */
export async function closePayment(paymentId: number): Promise<void> {
    return request({
        url: `/v1/payments/${paymentId}/close`,
        method: 'POST'
    })
}

/**
 * 获取支付订单的退款列表
 * @param paymentId 支付订单ID
 */
export async function getPaymentRefunds(paymentId: number): Promise<RefundResponse[]> {
    return request({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'GET'
    })
}

/**
 * 创建退款
 * @param paymentId 支付订单ID
 * @param refundData 退款数据
 */
export async function createRefund(paymentId: number, refundData: CreateRefundRequest): Promise<RefundResponse> {
    return request({
        url: `/v1/payments/${paymentId}/refunds`,
        method: 'POST',
        data: refundData
    })
}

// ==================== 便捷方法 ====================

/**
 * 为订单创建小程序支付
 * @param orderId 订单ID
 */
export async function createOrderPayment(orderId: number): Promise<PaymentOrderResponse> {
    return createPayment({
        order_id: orderId,
        payment_type: 'miniprogram',
        business_type: 'order'
    })
}

/**
 * 为预定创建小程序支付
 * @param reservationId 预定ID
 */
export async function createReservationPayment(reservationId: number): Promise<PaymentOrderResponse> {
    return createPayment({
        order_id: reservationId,
        payment_type: 'miniprogram',
        business_type: 'reservation'
    })
}

/**
 * 调起微信支付
 * @param paymentParams 支付参数
 */
export async function invokeWechatPay(paymentParams: MiniProgramPayParams): Promise<void> {
    return new Promise((resolve, reject) => {
        wx.requestPayment({
            ...paymentParams,
            success: () => resolve(),
            fail: (error) => reject(error)
        })
    })
}

/**
 * 完整的支付流程
 * @param orderId 订单ID
 * @param businessType 业务类型
 */
export async function processPayment(orderId: number, businessType: BusinessType = 'order'): Promise<void> {
    try {
        // 1. 创建支付订单
        const payment = await createPayment({
            order_id: orderId,
            payment_type: 'miniprogram',
            business_type: businessType
        })

        // 2. 调起微信支付
        if (payment.pay_params) {
            await invokeWechatPay(payment.pay_params)
        } else {
            throw new Error('支付参数缺失')
        }
    } catch (error) {
        console.error('支付失败:', error)
        throw error
    }
}

/**
 * 检查支付状态
 * @param paymentId 支付订单ID
 */
export async function checkPaymentStatus(paymentId: number): Promise<PaymentStatus> {
    const payment = await getPaymentDetail(paymentId)
    return payment.status
}

/**
 * 轮询支付状态直到完成
 * @param paymentId 支付订单ID
 * @param maxAttempts 最大尝试次数
 * @param interval 轮询间隔（毫秒）
 */
export async function pollPaymentStatus(
    paymentId: number,
    maxAttempts: number = 30,
    interval: number = 2000
): Promise<PaymentStatus> {
    for (let i = 0; i < maxAttempts; i++) {
        const status = await checkPaymentStatus(paymentId)

        if (status === 'paid' || status === 'failed' || status === 'cancelled') {
            return status
        }

        if (i < maxAttempts - 1) {
            await new Promise(resolve => setTimeout(resolve, interval))
        }
    }

    throw new Error('支付状态检查超时')
}

// ==================== 兼容性别名 ====================

/** @deprecated 使用 getPaymentList 替代 */
export const getPayments = getPaymentList

/** @deprecated 使用 PaymentOrderResponse 替代 */
export type PaymentDTO = PaymentOrderResponse

/** @deprecated 使用 createPayment 替代 */
export const pay = createPayment