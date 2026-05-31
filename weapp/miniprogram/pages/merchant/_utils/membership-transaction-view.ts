import { buildStatusTagView, type StatusTagTheme } from '../_main_shared/utils/status-tag'

export interface MembershipTransactionTagView {
  label: string
  theme: StatusTagTheme
}

export function getMembershipTransactionTagView(type: string): MembershipTransactionTagView {
  switch (type) {
    case 'recharge':
      return buildStatusTagView('充值', 'success')
    case 'consume':
      return buildStatusTagView('消费', 'warning')
    case 'refund':
      return buildStatusTagView('退款', 'success')
    case 'adjust':
    case 'adjustment_credit':
      return buildStatusTagView('人工调整', 'info')
    case 'adjustment_debit':
      return buildStatusTagView('人工扣减', 'warning')
    default:
      return buildStatusTagView(type || '交易', 'neutral')
  }
}
