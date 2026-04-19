import { buildStatusTagView, type StatusTagTheme } from './status-tag'

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
      return buildStatusTagView('人工调整', 'info')
    default:
      return buildStatusTagView(type || '交易', 'neutral')
  }
}