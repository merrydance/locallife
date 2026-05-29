import RiderService from '../api/rider'
import type { PaymentWorkflowStatus } from './payment-workflow'
import {
  completePaymentWorkflow
} from './payment-workflow'
import {
  getPaymentDetail,
  isPaymentStatusFailed,
  isPaymentStatusSuccessful,
  mapWechatTradeStateToPaymentStatus,
  PAYMENT_STATUS_POLL_INTERVAL_MS,
  PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
  queryPaymentOrder,
  type PaymentOrderQueryResponse,
  type PaymentOrderResponse,
  type PaymentStatus
} from '../api/payment'
import { logger } from '../../../../utils/logger'

const STORAGE_KEY = 'riderDepositPendingRecharge'

export type RiderDepositRechargeWorkflowStatus =
  | 'idle'
  | 'creating'
  | 'paying'
  | PaymentWorkflowStatus

export interface RiderDepositPendingRechargeContext {
  paymentOrderId: number
  amount: number
  outTradeNo?: string
  expiresAt?: string
  updatedAt: string
}

export interface RiderDepositRechargeWorkflowResult {
  status: RiderDepositRechargeWorkflowStatus
  paymentOrderId: number
  amount: number
  paymentStatus?: PaymentStatus | string
  shouldRefresh: boolean
  pendingContext: RiderDepositPendingRechargeContext | null
}

export interface RiderDepositRechargeWorkflowOptions {
  context?: WechatMiniprogram.Page.TrivialInstance
}

export interface RiderDepositRechargeWorkflowStatusView {
  isPaid: boolean
  isCancelled: boolean
  isPendingConfirmation: boolean
  isFailed: boolean
  feedbackMessage: string
  feedbackTheme: 'success' | 'warning'
}

function buildPendingContext(params: {
  paymentOrderId: number
  amount: number
  outTradeNo?: string
  expiresAt?: string
}): RiderDepositPendingRechargeContext {
  return {
    paymentOrderId: params.paymentOrderId,
    amount: params.amount,
    outTradeNo: params.outTradeNo,
    expiresAt: params.expiresAt,
    updatedAt: new Date().toISOString()
  }
}

function isValidPendingRechargeContext(value: unknown): value is RiderDepositPendingRechargeContext {
  if (!value || typeof value !== 'object') {
    return false
  }

  const candidate = value as Partial<RiderDepositPendingRechargeContext>
  return Number.isFinite(candidate.paymentOrderId)
    && Number.isFinite(candidate.amount)
    && typeof candidate.updatedAt === 'string'
}

export function savePendingRiderDepositRecharge(context: RiderDepositPendingRechargeContext) {
  try {
    wx.setStorageSync(STORAGE_KEY, context)
  } catch (error: unknown) {
    logger.error('保存骑手押金待确认支付上下文失败', error, 'rider-deposit-payment')
    const recoverableError = new Error('支付进度暂时无法保存，请稍后再试')
    ;(recoverableError as Error & { userMessage?: string }).userMessage = '支付进度暂时无法保存，请稍后再试'
    throw recoverableError
  }
}

export function getPendingRiderDepositRecharge(): RiderDepositPendingRechargeContext | null {
  try {
    const stored = wx.getStorageSync(STORAGE_KEY) as unknown
    if (!isValidPendingRechargeContext(stored)) {
      return null
    }
    return stored
  } catch (error: unknown) {
    logger.error('读取骑手押金待确认支付上下文失败', error, 'rider-deposit-payment')
    return null
  }
}

export function clearPendingRiderDepositRecharge() {
  try {
    wx.removeStorageSync(STORAGE_KEY)
  } catch (error: unknown) {
    logger.error('清除骑手押金待确认支付上下文失败', error, 'rider-deposit-payment')
    return
  }
}

export function getRiderDepositRechargeWorkflowStatusView(
  status: RiderDepositRechargeWorkflowStatus
): RiderDepositRechargeWorkflowStatusView {
  switch (status) {
    case 'paid':
      return {
        isPaid: true,
        isCancelled: false,
        isPendingConfirmation: false,
        isFailed: false,
        feedbackMessage: '充值已完成，账户余额和账单已同步更新。',
        feedbackTheme: 'success'
      }
    case 'cancelled':
      return {
        isPaid: false,
        isCancelled: true,
        isPendingConfirmation: false,
        isFailed: false,
        feedbackMessage: '已取消支付，充值单仍保留，可继续支付或稍后确认。',
        feedbackTheme: 'warning'
      }
    case 'pending_confirmation':
      return {
        isPaid: false,
        isCancelled: false,
        isPendingConfirmation: true,
        isFailed: false,
        feedbackMessage: '支付已提交，状态待确认，可继续支付或查看支付进度。',
        feedbackTheme: 'warning'
      }
    default:
      return {
        isPaid: false,
        isCancelled: false,
        isPendingConfirmation: false,
        isFailed: true,
        feedbackMessage: '充值未完成，可重新发起支付。',
        feedbackTheme: 'warning'
      }
  }
}

function getPaymentEffectiveStatus(payment: PaymentOrderQueryResponse | PaymentOrderResponse): PaymentStatus | string {
  const queriedStatus = 'wechat_query' in payment
    ? mapWechatTradeStateToPaymentStatus(payment.wechat_query?.trade_state)
    : undefined
  return queriedStatus || payment.status
}

function buildRechargeResultFromPayment(
  status: RiderDepositRechargeWorkflowStatus,
  payment: PaymentOrderQueryResponse | PaymentOrderResponse,
  context: RiderDepositPendingRechargeContext | null,
  shouldRefresh: boolean
): RiderDepositRechargeWorkflowResult {
  return {
    status,
    paymentOrderId: payment.id,
    amount: payment.amount,
    paymentStatus: getPaymentEffectiveStatus(payment),
    shouldRefresh,
    pendingContext: context
  }
}

async function getRiderDepositRechargePaymentTruth(paymentOrderId: number): Promise<PaymentOrderQueryResponse | PaymentOrderResponse> {
  try {
    return await queryPaymentOrder(paymentOrderId)
  } catch (error: unknown) {
    logger.error('骑手押金支付远端查询失败，回退本地支付单查询', error, 'rider-deposit-payment')
    return getPaymentDetail(paymentOrderId)
  }
}

export async function recoverPendingRiderDepositRecharge(
  context: RiderDepositPendingRechargeContext
): Promise<RiderDepositRechargeWorkflowResult> {
  const payment = await getRiderDepositRechargePaymentTruth(context.paymentOrderId)
  const paymentStatus = getPaymentEffectiveStatus(payment)

  if (isPaymentStatusSuccessful(paymentStatus)) {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('paid', payment, null, true)
  }

  if (isPaymentStatusFailed(paymentStatus)) {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('failed', payment, null, true)
  }

  const nextContext = buildPendingContext({
    paymentOrderId: payment.id,
    amount: payment.amount,
    outTradeNo: payment.out_trade_no
  })
  savePendingRiderDepositRecharge(nextContext)

  return buildRechargeResultFromPayment('pending_confirmation', payment, nextContext, false)
}

export async function recoverStoredRiderDepositRecharge(): Promise<RiderDepositRechargeWorkflowResult | null> {
  const pendingRecharge = getPendingRiderDepositRecharge()
  if (!pendingRecharge) {
    return null
  }

  return recoverPendingRiderDepositRecharge(pendingRecharge)
}

async function createRechargePayment(amountFen: number) {
  const rechargeResult = await RiderService.rechargeDeposit({ amount: amountFen })
  const paymentOrderId = Number(rechargeResult.payment_order_id || 0)
  const context = paymentOrderId > 0
    ? buildPendingContext({
        paymentOrderId,
        amount: amountFen,
        outTradeNo: rechargeResult.out_trade_no,
        expiresAt: rechargeResult.expires_at
      })
    : null

  if (context) {
    savePendingRiderDepositRecharge(context)
  }

  return {
    rechargeResult,
    context
  }
}

function toPaymentOrderResponse(
  payment: PaymentOrderQueryResponse | PaymentOrderResponse,
  fallbackContext: RiderDepositPendingRechargeContext
): PaymentOrderResponse {
  return {
    ...(payment as PaymentOrderResponse),
    id: payment.id,
    user_id: (payment as PaymentOrderResponse).user_id || 0,
    order_id: (payment as PaymentOrderResponse).order_id || 0,
    out_trade_no: payment.out_trade_no || fallbackContext.outTradeNo || '',
    amount: payment.amount || fallbackContext.amount,
    status: getPaymentEffectiveStatus(payment) as PaymentStatus,
    payment_type: (payment as PaymentOrderResponse).payment_type || 'miniprogram',
    business_type: (payment as PaymentOrderResponse).business_type || 'rider_deposit',
    created_at: (payment as PaymentOrderResponse).created_at || ''
  }
}

function buildRechargeResultFromWorkflow(
  workflowStatus: PaymentWorkflowStatus,
  payment: PaymentOrderQueryResponse | PaymentOrderResponse,
  context: RiderDepositPendingRechargeContext
): RiderDepositRechargeWorkflowResult {
  if (workflowStatus === 'paid') {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('paid', payment, null, true)
  }

  if (workflowStatus === 'failed' || workflowStatus === 'closed') {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('failed', payment, null, true)
  }

  savePendingRiderDepositRecharge(context)
  return buildRechargeResultFromPayment(workflowStatus, payment, context, false)
}

async function completeRiderDepositPayment(
  payment: PaymentOrderResponse,
  context: RiderDepositPendingRechargeContext,
  options: RiderDepositRechargeWorkflowOptions = {}
): Promise<RiderDepositRechargeWorkflowResult> {
  const workflowResult = await completePaymentWorkflow(payment, {
    maxAttempts: PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
    interval: PAYMENT_STATUS_POLL_INTERVAL_MS,
    context: options.context,
    paymentMessage: '正在调起微信支付...',
    confirmingMessage: '支付结果确认中...'
  })

  return buildRechargeResultFromWorkflow(
    workflowResult.status,
    workflowResult.payment || payment,
    context
  )
}

export async function submitRiderDepositRecharge(
  amountFen: number,
  options: RiderDepositRechargeWorkflowOptions = {}
): Promise<RiderDepositRechargeWorkflowResult> {
  const { rechargeResult, context } = await createRechargePayment(amountFen)

  if (!context) {
    return {
      status: 'failed',
      paymentOrderId: 0,
      amount: amountFen,
      shouldRefresh: false,
      pendingContext: null
    }
  }

  if (!rechargeResult.pay_params) {
    return recoverPendingRiderDepositRecharge(context)
  }

  return completeRiderDepositPayment({
    id: context.paymentOrderId,
    user_id: 0,
    order_id: 0,
    out_trade_no: rechargeResult.out_trade_no || context.outTradeNo || '',
    amount: context.amount,
    status: 'pending',
    payment_type: 'miniprogram',
    business_type: 'rider_deposit',
    pay_params: rechargeResult.pay_params,
    created_at: ''
  } as PaymentOrderResponse, context, options)
}

export async function continuePendingRiderDepositRecharge(
  pendingRecharge: RiderDepositPendingRechargeContext,
  options: RiderDepositRechargeWorkflowOptions = {}
): Promise<RiderDepositRechargeWorkflowResult> {
  const recoveryResult = await recoverPendingRiderDepositRecharge(pendingRecharge)
  if (recoveryResult.status !== 'pending_confirmation') {
    return recoveryResult
  }

  const payment = await getRiderDepositRechargePaymentTruth(recoveryResult.paymentOrderId)
  const paymentStatus = getPaymentEffectiveStatus(payment)
  if (isPaymentStatusSuccessful(paymentStatus)) {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('paid', payment, null, true)
  }
  if (isPaymentStatusFailed(paymentStatus)) {
    clearPendingRiderDepositRecharge()
    return buildRechargeResultFromPayment('failed', payment, null, true)
  }

  const nextContext = recoveryResult.pendingContext || buildPendingContext({
    paymentOrderId: payment.id,
    amount: payment.amount,
    outTradeNo: payment.out_trade_no
  })
  savePendingRiderDepositRecharge(nextContext)

  if (!payment.pay_params) {
    return buildRechargeResultFromPayment('pending_confirmation', payment, nextContext, false)
  }

  return completeRiderDepositPayment(toPaymentOrderResponse(payment, nextContext), nextContext, options)
}

export async function continueStoredRiderDepositRecharge(
  options: RiderDepositRechargeWorkflowOptions = {}
): Promise<RiderDepositRechargeWorkflowResult | null> {
  const pendingRecharge = getPendingRiderDepositRecharge()
  if (!pendingRecharge) {
    return null
  }

  return continuePendingRiderDepositRecharge(pendingRecharge, options)
}

export async function getRiderDepositPaymentDetail(paymentOrderId: number): Promise<PaymentOrderResponse> {
  return getPaymentDetail(paymentOrderId)
}
