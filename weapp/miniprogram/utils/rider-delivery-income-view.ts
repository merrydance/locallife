import { Delivery } from '../api/delivery'

export interface RiderDeliveryIncomeView {
  grossText: string
  feeText: string
  netText: string
  summaryText: string
  hasBill: boolean
  statusText: string
}

function formatMoney(cents?: number): string {
  if (typeof cents !== 'number' || !Number.isFinite(cents)) {
    return '--'
  }
  return `¥${(cents / 100).toFixed(2)}`
}

function hasAmount(value?: number): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

export function buildRiderDeliveryIncomeView(delivery: Pick<
  Delivery,
  | 'delivery_fee'
  | 'rider_earnings'
  | 'rider_gross_amount'
  | 'rider_payment_fee'
  | 'rider_net_earnings'
  | 'profit_sharing_order_id'
  | 'profit_sharing_status'
>): RiderDeliveryIncomeView {
  const hasBill = !!delivery.profit_sharing_order_id && (
    hasAmount(delivery.rider_payment_fee) ||
    hasAmount(delivery.rider_net_earnings) ||
    hasAmount(delivery.rider_gross_amount)
  )

  const grossAmount = hasAmount(delivery.rider_gross_amount)
    ? delivery.rider_gross_amount
    : delivery.delivery_fee
  const netAmount = hasAmount(delivery.rider_net_earnings)
    ? delivery.rider_net_earnings
    : (hasBill ? delivery.rider_earnings : undefined)

  return {
    grossText: formatMoney(grossAmount),
    feeText: hasBill ? formatMoney(delivery.rider_payment_fee || 0) : '--',
    netText: hasBill ? formatMoney(netAmount) : '账单同步中',
    summaryText: hasBill ? formatMoney(netAmount) : '账单同步中',
    hasBill,
    statusText: delivery.profit_sharing_status || ''
  }
}
