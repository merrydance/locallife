import {
  BusinessType,
  CombinedPaymentOrderResponse,
  createPayment,
  getPaymentDetail,
  invokeWechatPay,
  isCombinedPaymentSuccessful,
  isPaymentStatusFailed,
  isPaymentStatusSuccessful,
  PaymentCancelledError,
  PaymentOrderResponse,
  PAYMENT_STATUS_POLL_INTERVAL_MS,
  PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
  PaymentStatus,
  PaymentType,
  recoverCombinedPaymentOrder,
  shouldRecreateCombinedPayment,
  pollPaymentStatus
} from '../api/payment'
import Toast, { hideToast } from '../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../utils/logger'

const TOAST_SELECTOR = '#t-toast'

// New Mini Program payment flows must enter here so requestPayment never becomes the UI terminal truth.
export type PaymentWorkflowStatus =
  | 'paid'
  | 'failed'
  | 'cancelled'
  | 'pending_confirmation'
  | 'create_failed'
  | 'pay_params_missing'
  | 'closed'

export type PaymentWorkflowKind =
  | 'order'
  | 'combined_order'
  | 'reservation'
  | 'reservation_addon'
  | 'rider_deposit'
  | 'baofu_account_verify_fee'
  | 'claim_recovery'

export interface PaymentWorkflowResult {
  kind: PaymentWorkflowKind
  status: PaymentWorkflowStatus
  paymentOrderId?: number
  businessId?: number
  businessType?: BusinessType | string
  amountFen?: number
  outTradeNo?: string
  payment?: PaymentOrderResponse
}

export type CombinedPaymentWorkflowStatus =
  | 'paid'
  | 'cancelled'
  | 'pending_confirmation'
  | 'recreate_required'
  | 'pay_params_missing'

export interface CombinedPaymentWorkflowResult {
  kind: 'combined_order'
  status: CombinedPaymentWorkflowStatus
  combinedPaymentId: number
  amountFen: number
  outTradeNo: string
  combinedPayment: CombinedPaymentOrderResponse
}

export interface StartPaymentOrderWorkflowParams {
  orderId: number
  businessType: BusinessType
  paymentType?: PaymentType
  maxAttempts?: number
  interval?: number
  context?: WechatMiniprogram.Page.TrivialInstance
}

export interface PaymentTerminalWaitOptions {
  maxAttempts?: number
  interval?: number
  context?: WechatMiniprogram.Page.TrivialInstance
  paymentMessage?: string
  confirmingMessage?: string
}

type PaymentWorkflowOptions = PaymentTerminalWaitOptions

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function showPaymentWorkflowToast(
  context: WechatMiniprogram.Page.TrivialInstance | undefined,
  message: string
) {
  if (!context) {
    return
  }

  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme: 'loading',
    direction: 'column',
    duration: 0,
    preventScrollThrough: true
  })
}

function hidePaymentWorkflowToast(context: WechatMiniprogram.Page.TrivialInstance | undefined) {
  if (!context) {
    return
  }

  hideToast({ context, selector: TOAST_SELECTOR })
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value)
  } catch (_error) {
    return '"[unserializable]"'
  }
}

function logPaymentWorkflowError(message: string, error: unknown, details: Record<string, unknown>) {
  logger.error(`${message} ${safeStringify(details)}`, error, 'payment-workflow')
}

function toPaymentWorkflowKind(businessType?: BusinessType | string): PaymentWorkflowKind {
  switch (businessType) {
    case 'reservation':
      return 'reservation'
    case 'reservation_addon':
      return 'reservation_addon'
    case 'rider_deposit':
      return 'rider_deposit'
    case 'claim_recovery':
      return 'claim_recovery'
    case 'baofu_account_verify_fee':
      return 'baofu_account_verify_fee'
    default:
      return 'order'
  }
}

export function mapPaymentStatusToWorkflowStatus(status?: PaymentStatus | string): PaymentWorkflowStatus {
  if (isPaymentStatusSuccessful(status)) {
    return 'paid'
  }

  if (status === 'closed') {
    return 'closed'
  }

  if (isPaymentStatusFailed(status)) {
    return 'failed'
  }

  return 'pending_confirmation'
}

export function isPaymentWorkflowPaid(status?: PaymentWorkflowStatus | string): boolean {
  return status === 'paid'
}

export function isCombinedPaymentWorkflowPaid(status?: CombinedPaymentWorkflowStatus | string): boolean {
  return status === 'paid'
}

export function isCombinedPaymentWorkflowCancelled(status?: CombinedPaymentWorkflowStatus | string): boolean {
  return status === 'cancelled'
}

export function shouldRecreateCombinedPaymentWorkflow(status?: CombinedPaymentWorkflowStatus | string): boolean {
  return status === 'recreate_required'
}

export function buildPaymentWorkflowResultFromPayment(
  payment: PaymentOrderResponse,
  status: PaymentWorkflowStatus = mapPaymentStatusToWorkflowStatus(payment.status)
): PaymentWorkflowResult {
  return {
    kind: toPaymentWorkflowKind(payment.business_type),
    status,
    paymentOrderId: payment.id,
    businessId: payment.order_id,
    businessType: payment.business_type,
    amountFen: payment.amount,
    outTradeNo: payment.out_trade_no,
    payment
  }
}

export async function queryPaymentWorkflowResult(paymentOrderId: number): Promise<PaymentWorkflowResult> {
  const payment = await getPaymentDetail(paymentOrderId)
  return buildPaymentWorkflowResultFromPayment(payment)
}

export async function waitForPaymentWorkflowTerminalResult(
  paymentOrderId: number,
  options: PaymentTerminalWaitOptions = {}
): Promise<PaymentWorkflowResult> {
  const finalStatus = await pollPaymentStatus(
    paymentOrderId,
    options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
    options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
  )
  const payment = await getPaymentDetail(paymentOrderId)
  return buildPaymentWorkflowResultFromPayment(payment, mapPaymentStatusToWorkflowStatus(finalStatus))
}

async function queryPaymentWorkflowResultOrPending(payment: PaymentOrderResponse): Promise<PaymentWorkflowResult> {
  try {
    return await queryPaymentWorkflowResult(payment.id)
  } catch (error: unknown) {
    logPaymentWorkflowError('支付单查询失败，回退为待确认状态', error, {
      paymentId: payment.id,
      orderId: payment.order_id,
      businessType: payment.business_type,
      amount: payment.amount,
      outTradeNo: payment.out_trade_no
    })
    return buildPaymentWorkflowResultFromPayment(payment, 'pending_confirmation')
  }
}

export async function completePaymentWorkflow(
  payment: PaymentOrderResponse,
  options: PaymentWorkflowOptions = {}
): Promise<PaymentWorkflowResult> {
  if (!payment.pay_params) {
    const terminalStatus = mapPaymentStatusToWorkflowStatus(payment.status)
    if (terminalStatus === 'paid' || terminalStatus === 'failed' || terminalStatus === 'closed') {
      return buildPaymentWorkflowResultFromPayment(payment, terminalStatus)
    }

    return buildPaymentWorkflowResultFromPayment(payment, 'pay_params_missing')
  }

  try {
    showPaymentWorkflowToast(options.context, options.paymentMessage || '正在调起微信支付...')
    await invokeWechatPay(payment.pay_params)
  } catch (error: unknown) {
    if (error instanceof PaymentCancelledError) {
      hidePaymentWorkflowToast(options.context)
      return buildPaymentWorkflowResultFromPayment(payment, 'cancelled')
    }

    const wxError = error as { errMsg?: string }
    if (wxError?.errMsg?.includes('cancel')) {
      hidePaymentWorkflowToast(options.context)
      return buildPaymentWorkflowResultFromPayment(payment, 'cancelled')
    }

    logPaymentWorkflowError('调起微信支付失败，改为查询支付状态', error, {
      paymentId: payment.id,
      orderId: payment.order_id,
      businessType: payment.business_type,
      amount: payment.amount,
      outTradeNo: payment.out_trade_no
    })
    showPaymentWorkflowToast(options.context, options.confirmingMessage || '支付结果确认中...')
    try {
      return await queryPaymentWorkflowResultOrPending(payment)
    } finally {
      hidePaymentWorkflowToast(options.context)
    }
  }

  try {
    showPaymentWorkflowToast(options.context, options.confirmingMessage || '支付结果确认中...')
    const finalStatus = await pollPaymentStatus(
      payment.id,
      options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
      options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
    )
    return buildPaymentWorkflowResultFromPayment(payment, mapPaymentStatusToWorkflowStatus(finalStatus))
  } catch (error: unknown) {
    logPaymentWorkflowError('支付状态轮询失败，回退为待确认状态', error, {
      paymentId: payment.id,
      orderId: payment.order_id,
      businessType: payment.business_type,
      amount: payment.amount,
      outTradeNo: payment.out_trade_no
    })
    return buildPaymentWorkflowResultFromPayment(payment, 'pending_confirmation')
  } finally {
    hidePaymentWorkflowToast(options.context)
  }
}

export async function startPaymentOrderWorkflow(params: StartPaymentOrderWorkflowParams): Promise<PaymentWorkflowResult> {
  try {
    const payment = await createPayment({
      order_id: params.orderId,
      payment_type: params.paymentType || 'miniprogram',
      business_type: params.businessType
    })

    return completePaymentWorkflow(payment, {
      maxAttempts: params.maxAttempts,
      interval: params.interval,
      context: params.context
    })
  } catch (error: unknown) {
    logPaymentWorkflowError('创建支付单失败，返回 create_failed', error, {
      orderId: params.orderId,
      businessType: params.businessType,
      paymentType: params.paymentType || 'miniprogram'
    })
    return {
      kind: toPaymentWorkflowKind(params.businessType),
      status: 'create_failed',
      businessId: params.orderId,
      businessType: params.businessType
    }
  }
}

function buildCombinedPaymentWorkflowResult(
  combinedPayment: CombinedPaymentOrderResponse,
  status?: CombinedPaymentWorkflowStatus
): CombinedPaymentWorkflowResult {
  const resolvedStatus: CombinedPaymentWorkflowStatus = status || (
    isCombinedPaymentSuccessful(combinedPayment)
      ? 'paid'
      : shouldRecreateCombinedPayment(combinedPayment)
        ? 'recreate_required'
        : 'pending_confirmation'
  )

  return {
    kind: 'combined_order',
    status: resolvedStatus,
    combinedPaymentId: combinedPayment.id,
    amountFen: combinedPayment.total_amount,
    outTradeNo: combinedPayment.combine_out_trade_no,
    combinedPayment
  }
}

async function waitForCombinedPaymentWorkflowResult(
  combinedPaymentId: number,
  options: PaymentTerminalWaitOptions = {}
): Promise<CombinedPaymentWorkflowResult> {
  const maxAttempts = options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS
  const interval = options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    const recoveredPayment = await recoverCombinedPaymentOrder(combinedPaymentId)
    if (isCombinedPaymentSuccessful(recoveredPayment) || shouldRecreateCombinedPayment(recoveredPayment)) {
      return buildCombinedPaymentWorkflowResult(recoveredPayment)
    }

    if (attempt < maxAttempts - 1) {
      await delay(interval)
    }
  }

  throw new Error('合单支付状态检查超时')
}

function isWechatPaymentCancelled(error: unknown): boolean {
  if (error instanceof PaymentCancelledError) {
    return true
  }

  const wxError = error as { errMsg?: string }
  return !!wxError?.errMsg?.includes('cancel')
}

export async function completeCombinedPaymentWorkflow(
  combinedPayment: CombinedPaymentOrderResponse,
  options: PaymentWorkflowOptions = {}
): Promise<CombinedPaymentWorkflowResult> {
  if (!combinedPayment.pay_params) {
    if (isCombinedPaymentSuccessful(combinedPayment) || shouldRecreateCombinedPayment(combinedPayment)) {
      return buildCombinedPaymentWorkflowResult(combinedPayment)
    }

    return buildCombinedPaymentWorkflowResult(combinedPayment, 'pay_params_missing')
  }

  try {
    showPaymentWorkflowToast(options.context, options.paymentMessage || '正在调起微信支付...')
    await invokeWechatPay(combinedPayment.pay_params)
  } catch (error: unknown) {
    if (isWechatPaymentCancelled(error)) {
      hidePaymentWorkflowToast(options.context)
      return buildCombinedPaymentWorkflowResult(combinedPayment, 'cancelled')
    }

    logPaymentWorkflowError('合单支付调起失败，改为查询合单状态', error, {
      combinedPaymentId: combinedPayment.id,
      totalAmount: combinedPayment.total_amount,
      combineOutTradeNo: combinedPayment.combine_out_trade_no,
      subOrderCount: combinedPayment.sub_orders?.length || 0
    })
    showPaymentWorkflowToast(options.context, options.confirmingMessage || '支付结果确认中...')
    try {
      const recoveredPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
      return buildCombinedPaymentWorkflowResult(recoveredPayment)
    } catch (recoverError: unknown) {
      logPaymentWorkflowError('合单支付查询失败，回退为待确认状态', recoverError, {
        combinedPaymentId: combinedPayment.id,
        totalAmount: combinedPayment.total_amount,
        combineOutTradeNo: combinedPayment.combine_out_trade_no,
        subOrderCount: combinedPayment.sub_orders?.length || 0
      })
      return buildCombinedPaymentWorkflowResult(combinedPayment, 'pending_confirmation')
    } finally {
      hidePaymentWorkflowToast(options.context)
    }
  }

  try {
    showPaymentWorkflowToast(options.context, options.confirmingMessage || '支付结果确认中...')
    return await waitForCombinedPaymentWorkflowResult(combinedPayment.id)
  } catch (error: unknown) {
    logPaymentWorkflowError('合单支付轮询失败，回退为待确认状态', error, {
      combinedPaymentId: combinedPayment.id,
      totalAmount: combinedPayment.total_amount,
      combineOutTradeNo: combinedPayment.combine_out_trade_no,
      subOrderCount: combinedPayment.sub_orders?.length || 0
    })
    try {
      const recoveredPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
      return buildCombinedPaymentWorkflowResult(recoveredPayment, 'pending_confirmation')
    } catch (recoverError: unknown) {
      logPaymentWorkflowError('合单支付查询失败，仍回退为待确认状态', recoverError, {
        combinedPaymentId: combinedPayment.id,
        totalAmount: combinedPayment.total_amount,
        combineOutTradeNo: combinedPayment.combine_out_trade_no,
        subOrderCount: combinedPayment.sub_orders?.length || 0
      })
      return buildCombinedPaymentWorkflowResult(combinedPayment, 'pending_confirmation')
    }
  } finally {
    hidePaymentWorkflowToast(options.context)
  }
}
