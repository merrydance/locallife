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
}

export interface PaymentTerminalWaitOptions {
  maxAttempts?: number
  interval?: number
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
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

export async function completePaymentWorkflow(
  payment: PaymentOrderResponse,
  options: { maxAttempts?: number, interval?: number } = {}
): Promise<PaymentWorkflowResult> {
  if (!payment.pay_params) {
    const terminalStatus = mapPaymentStatusToWorkflowStatus(payment.status)
    if (terminalStatus === 'paid' || terminalStatus === 'failed' || terminalStatus === 'closed') {
      return buildPaymentWorkflowResultFromPayment(payment, terminalStatus)
    }

    return buildPaymentWorkflowResultFromPayment(payment, 'pay_params_missing')
  }

  try {
    await invokeWechatPay(payment.pay_params)
  } catch (error: unknown) {
    if (error instanceof PaymentCancelledError) {
      return buildPaymentWorkflowResultFromPayment(payment, 'cancelled')
    }

    const wxError = error as { errMsg?: string }
    if (wxError?.errMsg?.includes('cancel')) {
      return buildPaymentWorkflowResultFromPayment(payment, 'cancelled')
    }

    return buildPaymentWorkflowResultFromPayment(payment, 'failed')
  }

  try {
    const finalStatus = await pollPaymentStatus(
      payment.id,
      options.maxAttempts ?? PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
      options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
    )
    return buildPaymentWorkflowResultFromPayment(payment, mapPaymentStatusToWorkflowStatus(finalStatus))
  } catch {
    return buildPaymentWorkflowResultFromPayment(payment, 'pending_confirmation')
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
      interval: params.interval
    })
  } catch {
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
  combinedPayment: CombinedPaymentOrderResponse
): Promise<CombinedPaymentWorkflowResult> {
  if (!combinedPayment.pay_params) {
    if (isCombinedPaymentSuccessful(combinedPayment) || shouldRecreateCombinedPayment(combinedPayment)) {
      return buildCombinedPaymentWorkflowResult(combinedPayment)
    }

    return buildCombinedPaymentWorkflowResult(combinedPayment, 'pay_params_missing')
  }

  try {
    await invokeWechatPay(combinedPayment.pay_params)
  } catch (error: unknown) {
    if (isWechatPaymentCancelled(error)) {
      return buildCombinedPaymentWorkflowResult(combinedPayment, 'cancelled')
    }

    const recoveredPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
    return buildCombinedPaymentWorkflowResult(recoveredPayment)
  }

  try {
    return await waitForCombinedPaymentWorkflowResult(combinedPayment.id)
  } catch {
    const recoveredPayment = await recoverCombinedPaymentOrder(combinedPayment.id)
    return buildCombinedPaymentWorkflowResult(recoveredPayment, 'pending_confirmation')
  }
}