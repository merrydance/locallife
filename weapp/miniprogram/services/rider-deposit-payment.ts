import RiderService from '../api/rider'
import {
  PaymentCancelledError,
  getPaymentDetail,
  invokeWechatPay,
  isPaymentStatusFailed,
  isPaymentStatusSuccessful,
  pollPaymentStatus,
  type PaymentOrderResponse,
  type PaymentStatus
} from '../api/payment'

const STORAGE_KEY = 'riderDepositPendingRecharge'

export type RiderDepositRechargeWorkflowStatus =
  | 'idle'
  | 'creating'
  | 'paying'
  | 'submitted_pending_confirmation'
  | 'paid'
  | 'cancelled'
  | 'failed'
  | 'unknown'

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
  wx.setStorageSync(STORAGE_KEY, context)
}

export function getPendingRiderDepositRecharge(): RiderDepositPendingRechargeContext | null {
  try {
    const stored = wx.getStorageSync(STORAGE_KEY) as unknown
    if (!isValidPendingRechargeContext(stored)) {
      return null
    }
    return stored
  } catch (_error) {
    return null
  }
}

export function clearPendingRiderDepositRecharge() {
  try {
    wx.removeStorageSync(STORAGE_KEY)
  } catch (_error) {
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
    case 'submitted_pending_confirmation':
    case 'unknown':
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

export async function recoverPendingRiderDepositRecharge(
  context: RiderDepositPendingRechargeContext
): Promise<RiderDepositRechargeWorkflowResult> {
  const payment = await getPaymentDetail(context.paymentOrderId)

  if (isPaymentStatusSuccessful(payment.status)) {
    clearPendingRiderDepositRecharge()
    return {
      status: 'paid',
      paymentOrderId: context.paymentOrderId,
      amount: context.amount,
      paymentStatus: payment.status,
      shouldRefresh: true,
      pendingContext: null
    }
  }

  if (isPaymentStatusFailed(payment.status)) {
    clearPendingRiderDepositRecharge()
    return {
      status: 'failed',
      paymentOrderId: context.paymentOrderId,
      amount: context.amount,
      paymentStatus: payment.status,
      shouldRefresh: true,
      pendingContext: null
    }
  }

  const nextContext = buildPendingContext({
    paymentOrderId: payment.id,
    amount: payment.amount,
    outTradeNo: payment.out_trade_no
  })
  savePendingRiderDepositRecharge(nextContext)

  return {
    status: 'submitted_pending_confirmation',
    paymentOrderId: payment.id,
    amount: payment.amount,
    paymentStatus: payment.status,
    shouldRefresh: false,
    pendingContext: nextContext
  }
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

async function finalizeRechargeAfterPay(context: RiderDepositPendingRechargeContext): Promise<RiderDepositRechargeWorkflowResult> {
  try {
    const finalStatus = await pollPaymentStatus(context.paymentOrderId, 5, 1500)
    if (isPaymentStatusSuccessful(finalStatus)) {
      clearPendingRiderDepositRecharge()
      return {
        status: 'paid',
        paymentOrderId: context.paymentOrderId,
        amount: context.amount,
        paymentStatus: finalStatus,
        shouldRefresh: true,
        pendingContext: null
      }
    }

    clearPendingRiderDepositRecharge()
    return {
      status: 'failed',
      paymentOrderId: context.paymentOrderId,
      amount: context.amount,
      paymentStatus: finalStatus,
      shouldRefresh: true,
      pendingContext: null
    }
  } catch (_error) {
    return {
      status: 'unknown',
      paymentOrderId: context.paymentOrderId,
      amount: context.amount,
      shouldRefresh: false,
      pendingContext: context
    }
  }
}

export async function submitRiderDepositRecharge(amountFen: number): Promise<RiderDepositRechargeWorkflowResult> {
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

  try {
    await invokeWechatPay(rechargeResult.pay_params)
  } catch (error: unknown) {
    const errMsg = error instanceof Error ? error.message : ''
    const wxErrMsg = error && typeof error === 'object' && 'errMsg' in error ? String((error as { errMsg?: string }).errMsg || '') : ''
    if (error instanceof PaymentCancelledError || errMsg.includes('cancel') || wxErrMsg.includes('cancel')) {
      return {
        status: 'cancelled',
        paymentOrderId: context.paymentOrderId,
        amount: context.amount,
        shouldRefresh: false,
        pendingContext: context
      }
    }

    throw error
  }

  return finalizeRechargeAfterPay(context)
}

export async function continuePendingRiderDepositRecharge(
  pendingRecharge: RiderDepositPendingRechargeContext
): Promise<RiderDepositRechargeWorkflowResult> {
  return submitRiderDepositRecharge(pendingRecharge.amount)
}

export async function continueStoredRiderDepositRecharge(): Promise<RiderDepositRechargeWorkflowResult | null> {
  const pendingRecharge = getPendingRiderDepositRecharge()
  if (!pendingRecharge) {
    return null
  }

  return continuePendingRiderDepositRecharge(pendingRecharge)
}

export async function getRiderDepositPaymentDetail(paymentOrderId: number): Promise<PaymentOrderResponse> {
  return getPaymentDetail(paymentOrderId)
}