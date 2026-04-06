/**
 * 支付与退款统一 API 模块
 */

import { request } from '../utils/request'

export type PaymentStatus = 'pending' | 'paid' | 'refunded' | 'closed' | 'failed'
export type RefundStatus = 'pending' | 'processing' | 'success' | 'failed' | 'closed'
export type PaymentType = 'native' | 'miniprogram'
export type BusinessType = 'order' | 'reservation' | 'reservation_addon' | 'membership_recharge' | 'rider_deposit' | 'claim_recovery'
export type PaymentLedgerEntryType = 'payment' | 'refund'
export type PaymentProcessStatus = 'paid' | 'failed' | 'unknown'

export interface PaymentProcessResult {
  paymentId: number
  status: PaymentProcessStatus
}

export interface MiniProgramPayParams {
  timeStamp: string
  nonceStr: string
  package: string
  signType?: 'MD5' | 'HMAC-SHA256' | 'RSA'
  paySign: string
}

export interface PaymentOrder {
  id: number
  user_id: number
  order_id?: number
  out_trade_no: string
  prepay_id?: string
  amount: number
  status: PaymentStatus
  payment_type: PaymentType | string
  business_type: BusinessType | string
  pay_params?: MiniProgramPayParams
  created_at: string
  paid_at?: string
}

export type PaymentOrderResponse = PaymentOrder
export type PaymentDTO = PaymentOrder

export interface CombinedPaymentSubOrderResponse {
  order_id: number
  merchant_id: number
  sub_mch_id: string
  amount: number
  out_trade_no: string
  description: string
  profit_sharing_status?: string
  merchant_name?: string
  merchant_logo?: string
  order_no?: string
}

export interface CombinedPaymentOrderResponse {
  id: number
  combine_out_trade_no: string
  total_amount: number
  status: PaymentStatus | string
  prepay_id?: string
  pay_params?: MiniProgramPayParams
  expires_at?: string
  sub_orders: CombinedPaymentSubOrderResponse[]
}

export interface CreateCombinedPaymentRequest {
  order_ids: number[]
}

export interface CreatePaymentRequest {
  order_id: number
  payment_type?: PaymentType
  business_type: BusinessType
}

export interface RefundOrder {
  id: number
  payment_order_id: number
  refund_type: 'full' | 'partial'
  refund_amount: number
  refund_reason?: string
  out_refund_no: string
  status: RefundStatus | string
  refunded_at?: string
  created_at: string
}

export type RefundResponse = RefundOrder

export interface CreateRefundOrderRequest {
  payment_order_id: number
  refund_amount: number
  refund_type: 'full' | 'partial'
  refund_reason?: string
}

export interface LegacyRefundRequest {
  payment_id: number
  amount: number
  reason: string
  refund_type: 'full' | 'partial'
  operator_id?: number
}

export interface CreateRefundRequest {
  refund_amount: number
  refund_reason: string
  refund_type?: 'full' | 'partial'
}

export interface ListPaymentsParams {
  page_id?: number
  page_size?: number
  order_id?: number
}

export interface ListPaymentsResponse {
  payment_orders: PaymentOrder[]
  total: number
  page_id: number
  page_size: number
}

export interface ListRefundOrdersByPaymentResponse {
  refund_orders: RefundOrder[]
  total: number
}

export interface ProfitSharingReturn {
  id: number
  refund_order_id: number
  out_order_no: string
  out_return_no: string
  return_mchid: string
  amount: number
  status: string
  return_id?: string
  fail_reason?: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export interface PaymentLedgerEntry {
  id: number
  entry_type: PaymentLedgerEntryType
  payment_order_id: number
  refund_order_id?: number
  order_id?: number
  business_type: BusinessType | string
  amount: number
  status: string
  occurred_at: string
  created_at: string
}

export interface ListPaymentLedgerParams {
  page_id?: number
  page_size?: number
}

export interface ListPaymentLedgerResponse {
  entries: PaymentLedgerEntry[]
  total: number
  page_id: number
  page_size: number
}

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

export interface CalculateDeliveryFeeRequest extends Record<string, unknown> {
  merchant_id: number
  user_address_id: number
  order_amount: number
  delivery_distance?: number
  peak_hour?: boolean
  promotion_codes?: string[]
}

function normalizeRefundPayload(
  paymentIdOrParams: number | CreateRefundOrderRequest | LegacyRefundRequest,
  refundData?: CreateRefundRequest
): CreateRefundOrderRequest {
  if (typeof paymentIdOrParams === 'number') {
    if (!refundData) {
      throw new Error('refundData is required')
    }
    return {
      payment_order_id: paymentIdOrParams,
      refund_amount: refundData.refund_amount,
      refund_reason: refundData.refund_reason,
      refund_type: refundData.refund_type || 'full'
    }
  }

  if ('payment_id' in paymentIdOrParams) {
    return {
      payment_order_id: paymentIdOrParams.payment_id,
      refund_amount: paymentIdOrParams.amount,
      refund_reason: paymentIdOrParams.reason,
      refund_type: paymentIdOrParams.refund_type
    }
  }

  return paymentIdOrParams
}

export async function getPaymentList(params: ListPaymentsParams = {}): Promise<ListPaymentsResponse> {
  return request({
    url: '/v1/payments',
    method: 'GET',
    data: params
  })
}

export const getPayments = getPaymentList

export async function getPaymentLedger(params: ListPaymentLedgerParams = {}): Promise<ListPaymentLedgerResponse> {
  return request({
    url: '/v1/payments/ledger',
    method: 'GET',
    data: params
  })
}

export async function getPaymentDetail(paymentId: number): Promise<PaymentOrderResponse> {
  return request({
    url: `/v1/payments/${paymentId}`,
    method: 'GET'
  })
}

export const getPaymentById = getPaymentDetail

export async function createPayment(paymentData: CreatePaymentRequest): Promise<PaymentOrderResponse> {
  return request({
    url: '/v1/payments',
    method: 'POST',
    data: paymentData
  })
}

export const pay = createPayment

export async function createCombinedPaymentOrder(payload: CreateCombinedPaymentRequest): Promise<CombinedPaymentOrderResponse> {
  return request({
    url: '/v1/payments/combined',
    method: 'POST',
    data: payload
  })
}

export async function getCombinedPaymentOrder(combinedPaymentId: number): Promise<CombinedPaymentOrderResponse> {
  return request({
    url: `/v1/payments/combined/${combinedPaymentId}`,
    method: 'GET'
  })
}

export async function closeCombinedPaymentOrder(combinedPaymentId: number): Promise<CombinedPaymentOrderResponse> {
  return request({
    url: `/v1/payments/combined/${combinedPaymentId}/close`,
    method: 'POST'
  })
}

export async function closePayment(paymentId: number): Promise<PaymentOrderResponse> {
  return request({
    url: `/v1/payments/${paymentId}/close`,
    method: 'POST'
  })
}

export async function getPaymentRefunds(paymentId: number): Promise<ListRefundOrdersByPaymentResponse> {
  return request({
    url: `/v1/payments/${paymentId}/refunds`,
    method: 'GET'
  })
}

export async function createRefund(
  paymentIdOrParams: number | CreateRefundOrderRequest | LegacyRefundRequest,
  refundData?: CreateRefundRequest
): Promise<RefundOrder> {
  return request({
    url: '/v1/refunds',
    method: 'POST',
    data: normalizeRefundPayload(paymentIdOrParams, refundData)
  })
}

export async function getRefundById(id: number): Promise<RefundOrder> {
  return request({
    url: `/v1/refunds/${id}`,
    method: 'GET'
  })
}

export async function getRefundReturns(refundId: number): Promise<ProfitSharingReturn[]> {
  return request({
    url: `/v1/refunds/${refundId}/returns`,
    method: 'GET'
  })
}

export async function calculateDeliveryFee(params: CalculateDeliveryFeeRequest): Promise<DeliveryFeeResult> {
  return request({
    url: '/v1/delivery-fee/calculate',
    method: 'POST',
    data: params
  })
}

export async function createOrderPayment(orderId: number): Promise<PaymentOrderResponse> {
  return createPayment({
    order_id: orderId,
    business_type: 'order'
  })
}

export async function createReservationPayment(reservationId: number): Promise<PaymentOrderResponse> {
  return createPayment({
    order_id: reservationId,
    business_type: 'reservation'
  })
}

export async function invokeWechatPay(paymentParams: MiniProgramPayParams): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.requestPayment({
      ...paymentParams,
      success: () => resolve(),
      fail: (error) => reject(error)
    })
  })
}

export async function processPayment(orderId: number, businessType: BusinessType = 'order'): Promise<PaymentProcessResult> {
  let payment: PaymentOrderResponse

  try {
    payment = await createPayment({
      order_id: orderId,
      business_type: businessType
    })
  } catch (error: unknown) {
    console.warn('[payment] 创建支付单异常，按 unknown 承接', error)
    return {
      paymentId: 0,
      status: 'unknown'
    }
  }

  if (!payment.pay_params) {
    if (payment.status === 'paid' || payment.status === 'refunded') {
      return {
        paymentId: payment.id,
        status: 'paid'
      }
    }

    if (payment.status === 'failed' || payment.status === 'closed') {
      return {
        paymentId: payment.id,
        status: 'failed'
      }
    }

    console.warn('[payment] 支付参数缺失', {
      paymentId: payment.id,
      paymentStatus: payment.status,
      businessType
    })
    return {
      paymentId: payment.id,
      status: 'failed'
    }
  }

  try {
    await invokeWechatPay(payment.pay_params)
  } catch (error: unknown) {
    const wxError = error as { errMsg?: string }
    if (wxError?.errMsg?.includes('cancel')) {
      throw new PaymentCancelledError()
    }

    console.warn('[payment] 拉起支付失败', error)
    return {
      paymentId: payment.id,
      status: 'failed'
    }
  }

  try {
    const finalStatus = await pollPaymentStatus(payment.id, 5, 2000)

    if (finalStatus === 'paid' || finalStatus === 'refunded') {
      return {
        paymentId: payment.id,
        status: 'paid'
      }
    }

    return {
      paymentId: payment.id,
      status: 'failed'
    }
  } catch (error: unknown) {
    console.warn('[payment] 支付结果暂未同步，按 unknown 承接', error)
    return {
      paymentId: payment.id,
      status: 'unknown'
    }
  }
}

export async function checkPaymentStatus(paymentId: number): Promise<PaymentStatus> {
  const payment = await getPaymentDetail(paymentId)
  return payment.status
}

export async function pollPaymentStatus(paymentId: number, maxAttempts: number = 30, interval: number = 2000): Promise<PaymentStatus> {
  for (let i = 0; i < maxAttempts; i++) {
    const status = await checkPaymentStatus(paymentId)

    if (status === 'paid' || status === 'refunded' || status === 'closed' || status === 'failed') {
      return status
    }

    if (i < maxAttempts - 1) {
      await new Promise((resolve) => setTimeout(resolve, interval))
    }
  }

  throw new Error('支付状态检查超时')
}

export class PaymentCancelledError extends Error {
  constructor() {
    super('用户取消支付')
    this.name = 'PaymentCancelledError'
  }
}
