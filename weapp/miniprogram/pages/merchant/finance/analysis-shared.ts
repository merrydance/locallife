import dayjs from 'dayjs'

export type FinanceRangeKey = '7d' | '30d'
export type FinanceRangeOption = {
  key: FinanceRangeKey
  label: string
  days: number
}

export const FINANCE_RANGE_OPTIONS: FinanceRangeOption[] = [
  { key: '7d', label: '近7天', days: 7 },
  { key: '30d', label: '近30天', days: 30 }
]

export function buildFinanceRange(rangeKey: FinanceRangeKey) {
  const option = FINANCE_RANGE_OPTIONS.find((item) => item.key === rangeKey) || FINANCE_RANGE_OPTIONS[0]
  const end = dayjs()
  const start = end.subtract(option.days - 1, 'day')
  return {
    label: `${start.format('MM.DD')} - ${end.format('MM.DD')}`,
    params: {
      start_date: start.format('YYYY-MM-DD'),
      end_date: end.format('YYYY-MM-DD')
    }
  }
}

export function formatAmount(fen: number) {
  return (Number(fen || 0) / 100).toFixed(2)
}

export function formatAmountText(fen: number) {
  return `¥${formatAmount(fen)}`
}

export function formatDateTime(value?: string) {
  if (!value) return '暂无'
  return value.replace('T', ' ').slice(0, 16)
}

export function getOrderSourceText(source?: string) {
  switch (source) {
    case 'takeout':
      return '外卖订单'
    case 'dine_in':
      return '堂食订单'
    case 'reservation':
      return '预订订单'
    default:
      return source || '订单'
  }
}

export function getTimelineRecordTypeText(recordType?: string) {
  switch (recordType) {
    case 'profit_sharing':
      return '分账结算'
    case 'adjustment':
      return '结算调整'
    default:
      return recordType || '流水'
  }
}

export function getAdjustmentTypeText(adjustmentType?: string) {
  switch (adjustmentType) {
    case 'claim_recovery_charge':
      return '索赔追偿扣款'
    case 'claim_recovery_reversal':
      return '索赔追偿回补'
    default:
      return adjustmentType || '结算调整'
  }
}