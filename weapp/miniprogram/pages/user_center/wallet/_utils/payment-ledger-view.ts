import { getPaymentStatusView, getRefundStatusView, isPaymentStatusTerminal, isRefundStatusTerminal, PaymentLedgerEntry } from '../_main_shared/api/payment'

export type PaymentLedgerStatusTheme = 'primary' | 'success' | 'warning' | 'error' | 'default'

interface PaymentLedgerStatusView {
  statusName: string
  statusTheme: PaymentLedgerStatusTheme
}

export function isPaymentLedgerEntryTerminal(entry: Pick<PaymentLedgerEntry, 'entry_type' | 'status'>): boolean {
  return entry.entry_type === 'refund'
    ? isRefundStatusTerminal(entry.status)
    : isPaymentStatusTerminal(entry.status)
}

export function getPaymentLedgerStatusView(entry: Pick<PaymentLedgerEntry, 'entry_type' | 'status'>): PaymentLedgerStatusView {
  if (entry.entry_type === 'refund') {
    const refundStatusView = getRefundStatusView(entry.status)

    if (refundStatusView.normalizedStatus === 'success') {
      return { statusName: '退款成功', statusTheme: 'primary' }
    }

    if (refundStatusView.isProcessing) {
      return { statusName: '状态同步中', statusTheme: 'default' }
    }

    if (refundStatusView.isFailed) {
      return { statusName: '退款失败', statusTheme: 'error' }
    }

    if (refundStatusView.normalizedStatus === 'closed') {
      return { statusName: '已关闭', statusTheme: 'default' }
    }

    return { statusName: '状态同步中', statusTheme: 'default' }
  }

  const paymentStatusView = getPaymentStatusView(entry.status)

  if (paymentStatusView.normalizedStatus === 'paid') {
    return { statusName: '已支付', statusTheme: 'success' }
  }

  if (paymentStatusView.normalizedStatus === 'refunded') {
    return { statusName: '已退款', statusTheme: 'primary' }
  }

  if (paymentStatusView.normalizedStatus === 'pending') {
    return { statusName: '状态同步中', statusTheme: 'default' }
  }

  if (paymentStatusView.normalizedStatus === 'failed') {
    return { statusName: '支付失败', statusTheme: 'error' }
  }

  if (paymentStatusView.normalizedStatus === 'closed' || paymentStatusView.normalizedStatus === 'cancelled') {
    return { statusName: '已关闭', statusTheme: 'default' }
  }

  return { statusName: '状态同步中', statusTheme: 'default' }
}