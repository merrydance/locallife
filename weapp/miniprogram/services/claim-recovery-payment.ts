import type { ClaimRecoveryPaymentResponse } from '../api/appeals-customer-service'
import {
  invokeWechatPay,
  isPaymentStatusFailed,
  PAYMENT_STATUS_POLL_INTERVAL_MS,
  PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
  pollPaymentStatus
} from '../api/payment'
import { mapPaymentStatusToWorkflowStatus, type PaymentWorkflowStatus } from './payment-workflow'
import { logger } from '../utils/logger'

export interface ClaimRecoveryPaymentWorkflowResult {
  shouldSync: boolean
  paymentStatus: PaymentWorkflowStatus
  pendingConfirmation: boolean
}

export function isClaimRecoveryPaymentIncomplete(status: string): boolean {
  return isPaymentStatusFailed(status) || status === 'closed' || status === 'cancelled'
}

export async function completeClaimRecoveryPayment(
  paymentResult: ClaimRecoveryPaymentResponse,
  logContext: string
): Promise<ClaimRecoveryPaymentWorkflowResult> {
  if (paymentResult.pay_params) {
    try {
      await invokeWechatPay(paymentResult.pay_params)
    } catch (error: unknown) {
      const wxError = error as { errMsg?: string }
      if (wxError?.errMsg?.includes('cancel')) {
        wx.showToast({ title: '已取消支付', icon: 'none' })
        return {
          shouldSync: false,
          paymentStatus: 'cancelled',
          pendingConfirmation: false
        }
      }
      throw error
    }

    try {
      const finalStatus = await pollPaymentStatus(
        paymentResult.payment_order_id,
        PAYMENT_STATUS_POLL_MAX_ATTEMPTS,
        PAYMENT_STATUS_POLL_INTERVAL_MS
      )
      const workflowStatus = mapPaymentStatusToWorkflowStatus(finalStatus)
      return {
        shouldSync: true,
        paymentStatus: workflowStatus,
        pendingConfirmation: workflowStatus === 'pending_confirmation'
      }
    } catch (error) {
      logger.error(logContext, error)
      return {
        shouldSync: true,
        paymentStatus: 'pending_confirmation',
        pendingConfirmation: true
      }
    }
  }

  const workflowStatus = mapPaymentStatusToWorkflowStatus(paymentResult.status)
  return {
    shouldSync: true,
    paymentStatus: workflowStatus,
    pendingConfirmation: workflowStatus === 'pending_confirmation'
  }
}