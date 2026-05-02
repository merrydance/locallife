import { type MerchantFinanceOverviewResponse } from '../api/merchant-finance'
import { getErrorUserMessage } from '../utils/user-facing'

export interface MerchantFinanceAmountView {
  amount: number
  text: string
}

export interface MerchantFinanceOverviewView {
  completedOrders: number
  totalGmv: MerchantFinanceAmountView
  totalIncome: MerchantFinanceAmountView
  netIncome: MerchantFinanceAmountView
  totalServiceFee: MerchantFinanceAmountView
  pendingIncome: MerchantFinanceAmountView
}

export function formatFenToYuan(amount?: number): string {
  const normalized = Number.isFinite(amount) ? Math.max(Number(amount), 0) : 0
  return `¥${(normalized / 100).toFixed(2)}`
}

export function buildAmountView(amount?: number): MerchantFinanceAmountView {
  const normalized = Number.isFinite(amount) ? Number(amount) : 0
  return {
    amount: normalized,
    text: formatFenToYuan(normalized)
  }
}

export function formatDateTimeText(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hour = `${date.getHours()}`.padStart(2, '0')
  const minute = `${date.getMinutes()}`.padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

export function buildDefaultFinanceRange(days = 30): { start_date: string, end_date: string } {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))

  return {
    start_date: formatDateParam(start),
    end_date: formatDateParam(end)
  }
}

function formatDateParam(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildFinanceOverviewView(overview: MerchantFinanceOverviewResponse): MerchantFinanceOverviewView {
  return {
    completedOrders: overview.completed_orders || 0,
    totalGmv: buildAmountView(overview.total_gmv),
    totalIncome: buildAmountView(overview.total_income),
    netIncome: buildAmountView(overview.net_income),
    totalServiceFee: buildAmountView(overview.total_service_fee),
    pendingIncome: buildAmountView(overview.pending_income)
  }
}

export function getMerchantFinanceUserMessage(error: unknown, fallback: string): string {
  return getErrorUserMessage(error, fallback)
}
