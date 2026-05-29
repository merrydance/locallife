import { getPaymentRefunds, getPayments, type PaymentOrder, type RefundOrder } from '../_main_shared/api/payment'

export type OrderRefundProgressResult = {
  payment: PaymentOrder | null
  refund: RefundOrder | null
}

export function selectRefundablePayment(payments: PaymentOrder[]): PaymentOrder | null {
  if (!Array.isArray(payments) || payments.length === 0) {
    return null
  }

  const sorted = [...payments].sort((left, right) => {
    const rightCreated = new Date(right.created_at).getTime()
    const leftCreated = new Date(left.created_at).getTime()
    if (rightCreated !== leftCreated) {
      return rightCreated - leftCreated
    }
    return right.id - left.id
  })

  return sorted.find((payment) => payment.status === 'paid' || payment.status === 'refunded')
    || sorted.find((payment) => payment.status === 'pending')
    || sorted[0]
    || null
}

function sortRefundsByNewest(refunds: RefundOrder[]): RefundOrder[] {
  return [...refunds].sort((left, right) => {
    const rightCreated = new Date(right.created_at).getTime()
    const leftCreated = new Date(left.created_at).getTime()
    if (rightCreated !== leftCreated) {
      return rightCreated - leftCreated
    }

    return right.id - left.id
  })
}

export async function findLatestOrderRefund(orderId: number): Promise<OrderRefundProgressResult> {
  const paymentsResponse = await getPayments({ order_id: orderId, page_id: 1, page_size: 10 })
  const payment = selectRefundablePayment(paymentsResponse.payment_orders || [])
  if (!payment) {
    return { payment: null, refund: null }
  }

  const refund = await findLatestRefundByPayment(payment.id)
  return {
    payment,
    refund
  }
}

export async function findLatestRefundByPayment(paymentId: number): Promise<RefundOrder | null> {
  if (!paymentId) {
    return null
  }

  const refundsResponse = await getPaymentRefunds(paymentId)
  const refunds = sortRefundsByNewest(refundsResponse.refund_orders || [])
  return refunds[0] || null
}
