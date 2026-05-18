import type { ClaimRecoveryPaymentResponse } from '../api/appeals-customer-service'
import {
  isPaymentStatusFailed,
  type PaymentOrderResponse,
  type PaymentStatus
} from '../api/payment'
import {
  buildPaymentWorkflowResultFromPayment,
  completePaymentWorkflow,
  mapPaymentStatusToWorkflowStatus,
  type PaymentWorkflowStatus
} from './payment-workflow'

export interface ClaimRecoveryPaymentWorkflowResult {
  shouldSync: boolean
  paymentStatus: PaymentWorkflowStatus
  pendingConfirmation: boolean
}

export function isClaimRecoveryPaymentIncomplete(status: string): boolean {
  return isPaymentStatusFailed(status) || status === 'closed' || status === 'cancelled'
}

function normalizeClaimRecoveryPaymentStatus(status?: string): PaymentStatus {
  switch (status) {
    case 'paid':
    case 'refunded':
    case 'closed':
    case 'failed':
      return status
    default:
      return 'pending'
  }
}

function toClaimRecoveryPaymentOrder(paymentResult: ClaimRecoveryPaymentResponse): PaymentOrderResponse {
  return {
    ...(paymentResult as unknown as PaymentOrderResponse),
    id: paymentResult.payment_order_id,
    user_id: 0,
    order_id: 0,
    out_trade_no: paymentResult.out_trade_no || '',
    amount: paymentResult.amount || 0,
    status: normalizeClaimRecoveryPaymentStatus(paymentResult.status),
    payment_type: 'miniprogram',
    business_type: 'claim_recovery',
    pay_params: paymentResult.pay_params,
    created_at: ''
  }
}

export async function completeClaimRecoveryPayment(
  paymentResult: ClaimRecoveryPaymentResponse,
  _logContext: string,
  options: { context?: WechatMiniprogram.Page.TrivialInstance } = {}
): Promise<ClaimRecoveryPaymentWorkflowResult> {
  if (paymentResult.pay_params) {
    const workflowResult = await completePaymentWorkflow(toClaimRecoveryPaymentOrder(paymentResult), {
      context: options.context,
      paymentMessage: '正在调起微信支付...',
      confirmingMessage: '支付结果确认中...'
    })
    return {
      shouldSync: workflowResult.status !== 'cancelled',
      paymentStatus: workflowResult.status,
      pendingConfirmation: workflowResult.status === 'pending_confirmation'
    }
  }

  const workflowStatus = buildPaymentWorkflowResultFromPayment(
    toClaimRecoveryPaymentOrder(paymentResult),
    mapPaymentStatusToWorkflowStatus(paymentResult.status)
  ).status
  return {
    shouldSync: true,
    paymentStatus: workflowStatus,
    pendingConfirmation: workflowStatus === 'pending_confirmation'
  }
}
