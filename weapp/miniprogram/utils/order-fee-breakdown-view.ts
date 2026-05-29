export interface OrderFeeBreakdown {
  food_amount: number
  merchant_discount_amount: number
  voucher_discount_amount: number
  food_payable_amount: number
  delivery_fee_amount: number
  delivery_fee_discount_amount: number
  delivery_payable_amount: number
  customer_payable_amount: number
  platform_service_fee_amount: number
  payment_channel_fee_amount: number
  merchant_receivable_amount: number
  rider_gross_amount?: number
  rider_payment_fee_amount?: number
  rider_net_earnings_amount?: number
}

export interface OrderFeeBreakdownRow {
  key: string
  label: string
  value: number
  value_text: string
  tone: 'default' | 'discount' | 'total' | 'income' | 'fee'
  visible: boolean
}

export interface OrderFeeSettlementGroup {
  key: string
  title: string
  total_text: string
  tone: 'merchant' | 'rider'
  visible: boolean
  rows: OrderFeeBreakdownRow[]
}

export interface CustomerOrderFeeBreakdownView {
  available: boolean
  settlement_groups: OrderFeeSettlementGroup[]
}

export function formatOrderFeeMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

export function formatSignedOrderFeeMoney(amount: number) {
  if (amount < 0) {
    return `-¥${(Math.abs(amount) / 100).toFixed(2)}`
  }
  return formatOrderFeeMoney(amount)
}

export function createOrderFeeBreakdownRow(
  key: string,
  label: string,
  value: number,
  tone: OrderFeeBreakdownRow['tone'],
  alwaysVisible = false
): OrderFeeBreakdownRow {
  return {
    key,
    label,
    value,
    value_text: formatSignedOrderFeeMoney(value),
    tone,
    visible: alwaysVisible || value !== 0
  }
}

function deductionLabel(label: string) {
  return `- ${label}`
}

export function buildOrderFeeSettlementGroups(breakdown: OrderFeeBreakdown): OrderFeeSettlementGroup[] {
  const merchantRows = [
    createOrderFeeBreakdownRow('merchant_food_payable_amount', '菜品合计', breakdown.food_payable_amount, 'default', true),
    createOrderFeeBreakdownRow('merchant_platform_service_fee_amount', deductionLabel('平台服务费'), -breakdown.platform_service_fee_amount, 'fee', true),
    createOrderFeeBreakdownRow('merchant_payment_channel_fee_amount', deductionLabel('支付通道费'), -breakdown.payment_channel_fee_amount, 'fee', true),
    createOrderFeeBreakdownRow('merchant_receivable_amount', '商户实收', breakdown.merchant_receivable_amount, 'income', true)
  ]
  const riderGrossAmount = breakdown.rider_gross_amount || 0
  const riderPaymentFeeAmount = breakdown.rider_payment_fee_amount || 0
  const riderNetEarningsAmount = breakdown.rider_net_earnings_amount || 0
  const riderRows = [
    createOrderFeeBreakdownRow('rider_gross_amount', '代取费', riderGrossAmount, 'default', true),
    createOrderFeeBreakdownRow('rider_payment_fee_amount', deductionLabel('支付通道费'), -riderPaymentFeeAmount, 'fee', true),
    createOrderFeeBreakdownRow('rider_net_earnings_amount', '骑手实收', riderNetEarningsAmount, 'income', true)
  ]

  return [
    {
      key: 'merchant',
      title: '商户账单',
      total_text: formatOrderFeeMoney(breakdown.merchant_receivable_amount),
      tone: 'merchant',
      visible: true,
      rows: merchantRows
    },
    {
      key: 'rider',
      title: '骑手账单',
      total_text: formatOrderFeeMoney(riderNetEarningsAmount),
      tone: 'rider',
      visible: riderGrossAmount !== 0 || riderPaymentFeeAmount !== 0 || riderNetEarningsAmount !== 0,
      rows: riderRows
    }
  ]
}

export function buildCustomerOrderFeeBreakdownView(breakdown?: OrderFeeBreakdown): CustomerOrderFeeBreakdownView {
  if (!breakdown) {
    return {
      available: false,
      settlement_groups: []
    }
  }

  return {
    available: true,
    settlement_groups: buildOrderFeeSettlementGroups(breakdown)
  }
}
