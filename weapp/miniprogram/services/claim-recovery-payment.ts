import type { ClaimRecoveryPaymentResponse } from '../api/appeals-customer-service'
import { invokeWechatPay, isPaymentStatusSuccessful, pollPaymentStatus } from '../api/payment'
import { logger } from '../utils/logger'

export interface ClaimRecoveryPaymentWorkflowResult {
  shouldSync: boolean
  paymentStatus: string
  pendingConfirmation: boolean
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
          paymentStatus: paymentResult.status,
          pendingConfirmation: false
        }
      }
      throw error
    }

    try {
      const finalStatus = await pollPaymentStatus(paymentResult.payment_order_id, 5, 1500)
      return {
        shouldSync: true,
        paymentStatus: finalStatus,
        pendingConfirmation: !isPaymentStatusSuccessful(finalStatus)
      }
    } catch (error) {
      logger.error(logContext, error)
      return {
        shouldSync: true,
        paymentStatus: paymentResult.status,
        pendingConfirmation: true
      }
    }
  }

  return {
    shouldSync: true,
    paymentStatus: paymentResult.status,
    pendingConfirmation: !isPaymentStatusSuccessful(paymentResult.status)
  }
}