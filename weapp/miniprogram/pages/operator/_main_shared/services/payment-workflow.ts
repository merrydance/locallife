import {
  BusinessType,
  createPayment,
  getPaymentDetail,
  invokeWechatPay,
  isPaymentStatusFailed,
  isPaymentStatusSuccessful,
  PaymentCancelledError,
  PaymentOrderResponse,
  PAYMENT_STATUS_POLL_INTERVAL_MS,
  PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
  PaymentStatus,
  PaymentType,
  pollPaymentStatus
} from '../api/payment'
import { logger } from '../../../../utils/logger'

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
function showPaymentWorkflowToast(
  context: WechatMiniprogram.Page.TrivialInstance | undefined,
  message: string
) {
  void context
  wx.showLoading({ title: message, mask: true })
}

function hidePaymentWorkflowToast(context: WechatMiniprogram.Page.TrivialInstance | undefined) {
  void context
  wx.hideLoading()
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

async function queryPaymentWorkflowTerminalResult(
  paymentOrderId: number,
  finalStatus: PaymentStatus
): Promise<PaymentWorkflowResult> {
  try {
    const payment = await getPaymentDetail(paymentOrderId)
    return buildPaymentWorkflowResultFromPayment(payment, mapPaymentStatusToWorkflowStatus(finalStatus))
  } catch (error: unknown) {
    logPaymentWorkflowError('支付终态详情查询失败，使用支付轮询终态承接', error, {
      paymentId: paymentOrderId,
      finalStatus
    })
    return {
      kind: 'order',
      status: mapPaymentStatusToWorkflowStatus(finalStatus),
      paymentOrderId
    }
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
      const finalStatus = await pollPaymentStatus(
        payment.id,
        options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
        options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
      )
      return await queryPaymentWorkflowTerminalResult(payment.id, finalStatus)
    } catch (pollError: unknown) {
      logPaymentWorkflowError('调起支付失败后的状态轮询也未取到终态，回退为待确认状态', pollError, {
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

  try {
    showPaymentWorkflowToast(options.context, options.confirmingMessage || '支付结果确认中...')
    const finalStatus = await pollPaymentStatus(
      payment.id,
      options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
      options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
    )
    return await queryPaymentWorkflowTerminalResult(payment.id, finalStatus)
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
