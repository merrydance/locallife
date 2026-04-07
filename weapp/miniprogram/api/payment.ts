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
export type CombinedPaymentResolution = 'success' | 'recreate' | 'syncing'
export type PaymentViewTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

const SUCCESS_PAYMENT_STATUSES = new Set<PaymentStatus>(['paid', 'refunded'])
const FAILED_PAYMENT_STATUSES = new Set<PaymentStatus>(['closed', 'failed'])
const SYNCING_COMBINED_PAYMENT_STATES = new Set(['partial', 'mixed', 'unknown'])

export interface PaymentProcessResult {
  paymentId: number
  status: PaymentProcessStatus
  payment?: PaymentOrderResponse
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
  wechat_query?: CombinedPaymentWechatQueryResponse
  sub_orders: CombinedPaymentSubOrderResponse[]
}

export interface CombinedPaymentWechatAmountResponse {
  total_amount: number
  payer_amount: number
  currency: string
  payer_currency?: string
}

export interface CombinedPaymentWechatSubOrderResponse {
  mchid: string
  sub_mchid?: string
  sub_appid?: string
  sub_openid?: string
  out_trade_no: string
  transaction_id?: string
  trade_type?: string
  trade_state: string
  bank_type?: string
  attach?: string
  success_time?: string
  amount: CombinedPaymentWechatAmountResponse
}

export interface CombinedPaymentWechatQueryResponse {
  combine_out_trade_no: string
  aggregate_trade_state: PaymentStatus | 'partial' | 'mixed' | 'unknown' | string
  sub_orders: CombinedPaymentWechatSubOrderResponse[]
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

export interface RefundProgressView {
  title: string
  time: string
  done: boolean
  active: boolean
}

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

export function getCombinedPaymentEffectiveState(payment: CombinedPaymentOrderResponse): string {
  return payment.wechat_query?.aggregate_trade_state || payment.status
}

export function isCombinedPaymentSuccessful(payment: CombinedPaymentOrderResponse): boolean {
  return isPaymentStatusSuccessful(getCombinedPaymentEffectiveState(payment))
}

export function isPaymentStatusSuccessful(status?: string): boolean {
  return !!status && SUCCESS_PAYMENT_STATUSES.has(status as PaymentStatus)
}

export function isPaymentStatusFailed(status?: string): boolean {
  return !!status && FAILED_PAYMENT_STATUSES.has(status as PaymentStatus)
}

export function getPaymentStatusView(status?: PaymentStatus | string) {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  switch (normalizedStatus) {
    case 'paid':
      return {
        normalizedStatus,
        text: '已支付',
        icon: 'check-circle-filled',
        className: 'paid',
        theme: 'success' as PaymentViewTheme,
        isPending: false,
        showPendingTip: false
      }
    case 'pending':
      return {
        normalizedStatus,
        text: '待支付',
        icon: 'time-filled',
        className: 'pending',
        theme: 'warning' as PaymentViewTheme,
        isPending: true,
        showPendingTip: true
      }
    case 'failed':
      return {
        normalizedStatus,
        text: '支付失败',
        icon: 'close-circle-filled',
        className: 'failed',
        theme: 'danger' as PaymentViewTheme,
        isPending: false,
        showPendingTip: false
      }
    case 'closed':
    case 'cancelled':
      return {
        normalizedStatus,
        text: '已关闭',
        icon: 'info-circle-filled',
        className: 'closed',
        theme: 'default' as PaymentViewTheme,
        isPending: false,
        showPendingTip: false
      }
    case 'refunded':
      return {
        normalizedStatus,
        text: '已退款',
        icon: 'check-circle-filled',
        className: 'refunded',
        theme: 'primary' as PaymentViewTheme,
        isPending: false,
        showPendingTip: false
      }
    default:
      return {
        normalizedStatus,
        text: normalizedStatus || '状态更新中',
        icon: 'info-circle-filled',
        className: 'default',
        theme: 'default' as PaymentViewTheme,
        isPending: false,
        showPendingTip: false
      }
  }
}

export function getRefundStatusView(status?: RefundStatus | string) {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  switch (normalizedStatus) {
    case 'success':
      return {
        normalizedStatus,
        text: '退款成功',
        icon: 'check-circle-filled',
        className: 'success',
        theme: 'success' as PaymentViewTheme,
        showPendingTip: false,
        isProcessing: false,
        isFailed: false
      }
    case 'pending':
    case 'processing':
      return {
        normalizedStatus,
        text: normalizedStatus === 'pending' ? '退款申请中' : '退款处理中',
        icon: 'time-filled',
        className: 'processing',
        theme: 'warning' as PaymentViewTheme,
        showPendingTip: true,
        isProcessing: true,
        isFailed: false
      }
    case 'failed':
      return {
        normalizedStatus,
        text: '退款失败',
        icon: 'close-circle-filled',
        className: 'failed',
        theme: 'danger' as PaymentViewTheme,
        showPendingTip: false,
        isProcessing: false,
        isFailed: true
      }
    case 'closed':
      return {
        normalizedStatus,
        text: '退款已关闭',
        icon: 'info-circle-filled',
        className: 'default',
        theme: 'default' as PaymentViewTheme,
        showPendingTip: false,
        isProcessing: false,
        isFailed: false
      }
    default:
      return {
        normalizedStatus,
        text: normalizedStatus || '状态更新中',
        icon: 'info-circle-filled',
        className: 'default',
        theme: 'default' as PaymentViewTheme,
        showPendingTip: false,
        isProcessing: false,
        isFailed: false
      }
  }
}

export function buildRefundProgress(refund: RefundOrder, formatTime: (timeStr: string) => string): RefundProgressView[] {
  const statusView = getRefundStatusView(refund.status)
  const isProcessing = statusView.isProcessing
  const isFinished = statusView.isFailed || statusView.normalizedStatus === 'success'

  return [
    {
      title: '提交申请',
      time: formatTime(refund.created_at),
      done: true,
      active: statusView.normalizedStatus === 'pending'
    },
    {
      title: '审核中',
      time: '',
      done: isProcessing || isFinished,
      active: statusView.normalizedStatus === 'processing'
    },
    {
      title: '退款处理',
      time: '',
      done: isFinished,
      active: false
    },
    {
      title: statusView.isFailed ? '退款失败' : '退款完成',
      time: refund.refunded_at ? formatTime(refund.refunded_at) : '',
      done: isFinished,
      active: isFinished
    }
  ]
}

export function isPaymentProcessSuccessful(resultOrStatus: PaymentProcessResult | PaymentProcessStatus): boolean {
  const status = typeof resultOrStatus === 'string' ? resultOrStatus : resultOrStatus.status
  return status === 'paid'
}

export function isPaymentProcessFailed(resultOrStatus: PaymentProcessResult | PaymentProcessStatus): boolean {
  const status = typeof resultOrStatus === 'string' ? resultOrStatus : resultOrStatus.status
  return status === 'failed'
}

export function getPaymentProcessOutcomeMessage(
  resultOrStatus: PaymentProcessResult | PaymentProcessStatus,
  messages?: { failed?: string, unknown?: string }
): string {
  return isPaymentProcessFailed(resultOrStatus)
    ? (messages?.failed || '支付未完成，请稍后重试')
    : (messages?.unknown || '支付结果确认中，请稍后刷新')
}

export function getCombinedPaymentResolution(paymentOrState: CombinedPaymentOrderResponse | string): CombinedPaymentResolution {
  const effectiveState = typeof paymentOrState === 'string'
    ? paymentOrState
    : getCombinedPaymentEffectiveState(paymentOrState)

  if (isPaymentStatusSuccessful(effectiveState)) {
    return 'success'
  }

  if (isPaymentStatusFailed(effectiveState)) {
    return 'recreate'
  }

  if (SYNCING_COMBINED_PAYMENT_STATES.has(effectiveState)) {
    return 'syncing'
  }

  return 'syncing'
}

export function shouldRecreateCombinedPayment(paymentOrState: CombinedPaymentOrderResponse | string): boolean {
  return getCombinedPaymentResolution(paymentOrState) === 'recreate'
}

export function isCombinedPaymentSyncing(paymentOrState: CombinedPaymentOrderResponse | string): boolean {
  return getCombinedPaymentResolution(paymentOrState) === 'syncing'
}

export function getCombinedPaymentFollowupMessage(paymentOrState: CombinedPaymentOrderResponse | string): string {
  const resolution = getCombinedPaymentResolution(paymentOrState)

  if (resolution === 'recreate') {
    return '原合单已失效，请重新发起支付'
  }

  if (resolution === 'syncing') {
    return '支付状态正在同步，请稍后刷新'
  }

  return '可继续完成原合单支付'
}

export async function recoverCombinedPaymentOrder(combinedPaymentId: number): Promise<CombinedPaymentOrderResponse> {
  return queryCombinedPaymentOrder(combinedPaymentId)
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

export async function queryCombinedPaymentOrder(combinedPaymentId: number): Promise<CombinedPaymentOrderResponse> {
  return request({
    url: `/v1/payments/combined/${combinedPaymentId}/query`,
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

function resolveCreatedPaymentStatus(payment: PaymentOrderResponse): PaymentProcessStatus {
  if (isPaymentStatusSuccessful(payment.status)) {
    return 'paid'
  }

  if (isPaymentStatusFailed(payment.status)) {
    return 'failed'
  }

  console.warn('[payment] 支付参数缺失', {
    paymentId: payment.id,
    paymentStatus: payment.status,
    businessType: payment.business_type
  })
  return 'failed'
}

export async function processCreatedPayment(payment: PaymentOrderResponse): Promise<PaymentProcessResult> {
  if (!payment.pay_params) {
    return {
      paymentId: payment.id,
      status: resolveCreatedPaymentStatus(payment),
      payment
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
      status: 'failed',
      payment
    }
  }

  try {
    const finalStatus = await pollPaymentStatus(payment.id, 5, 2000)

    return {
      paymentId: payment.id,
      status: isPaymentStatusSuccessful(finalStatus) ? 'paid' : 'failed',
      payment
    }
  } catch (error: unknown) {
    console.warn('[payment] 支付结果暂未同步，按 unknown 承接', error)
    return {
      paymentId: payment.id,
      status: 'unknown',
      payment
    }
  }
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

  return processCreatedPayment(payment)
}

export async function checkPaymentStatus(paymentId: number): Promise<PaymentStatus> {
  const payment = await getPaymentDetail(paymentId)
  return payment.status
}

export async function pollPaymentStatus(paymentId: number, maxAttempts: number = 30, interval: number = 2000): Promise<PaymentStatus> {
  for (let i = 0; i < maxAttempts; i++) {
    const status = await checkPaymentStatus(paymentId)

    if (isPaymentStatusSuccessful(status) || isPaymentStatusFailed(status)) {
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
