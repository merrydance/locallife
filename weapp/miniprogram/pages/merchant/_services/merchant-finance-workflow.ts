import {
  getMerchantFinanceOrderStatusView,
  getMerchantFinanceOverview,
  listMerchantFinanceOrders,
  listMerchantSettlements,
  type MerchantFinanceOrderItem,
  type MerchantFinanceOrdersResponse,
  type MerchantFinanceOverviewResponse,
  type MerchantFinanceStatusTheme,
  type MerchantSettlementItem,
  type MerchantSettlementsResponse
} from '../_main_shared/api/merchant-finance'
import { validateFinanceDateRange } from '../_main_shared/utils/finance-date-range'
import { getErrorUserMessage } from '../../../utils/user-facing'

const DEFAULT_MERCHANT_FINANCE_DAYS = 30
export const MERCHANT_FINANCE_PAGE_SIZE = 20
export const MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS = 90
export const MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS = 365

export interface MerchantFinanceRange extends Record<string, unknown> {
  start_date: string
  end_date: string
}

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

export interface MerchantFinanceBillSummaryView {
  rangeLabel: string
  totalIncomeText: string
  totalGmvText: string
  totalOrdersText: string
  pendingIncomeText: string
}

export interface MerchantFinanceBillRowView {
  id: string
  title: string
  note: string
  amountText: string
  statusText: string
  statusTheme: MerchantFinanceStatusTheme
}

export interface MerchantFinanceBillPageView {
  rows: MerchantFinanceBillRowView[]
  summary: MerchantFinanceBillSummaryView
  summaryErrorMessage: string
  page: number
  totalPages: number
  hasMore: boolean
  totalCount: number
}

export interface MerchantFinanceSettlementSummaryView {
  rangeLabel: string
  settlementAmountText: string
  totalOrderAmountText: string
  deductionAmountText: string
  totalCountText: string
}

export interface MerchantFinanceSettlementRowView {
  id: string
  title: string
  note: string
  amountText: string
  statusText: string
  statusTheme: MerchantFinanceStatusTheme
}

export interface MerchantFinanceSettlementPageView {
  rows: MerchantFinanceSettlementRowView[]
  summary: MerchantFinanceSettlementSummaryView
  page: number
  totalPages: number
  hasMore: boolean
  totalCount: number
}

export type MerchantFinanceSettledResult<T> =
  | { status: 'fulfilled', value: T }
  | { status: 'rejected', reason: unknown }

export type MerchantFinanceFulfilledResult<T> = { status: 'fulfilled', value: T }
export type MerchantFinanceRejectedResult = { status: 'rejected', reason: unknown }

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

export function buildDefaultFinanceRange(days = DEFAULT_MERCHANT_FINANCE_DAYS): MerchantFinanceRange {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))

  return {
    start_date: formatDateParam(start),
    end_date: formatDateParam(end)
  }
}

export function buildMerchantFinanceMonthRange(): MerchantFinanceRange {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth(), 1)
  return {
    start_date: formatDateParam(start),
    end_date: formatDateParam(end)
  }
}

export function buildMerchantFinanceRangeLabel(range: MerchantFinanceRange): string {
  return `${range.start_date} 至 ${range.end_date}`
}

export function validateMerchantFinanceRange(
  range: MerchantFinanceRange,
  maxDays: number,
  label: string
) {
  return validateFinanceDateRange(range, maxDays, label)
}

export function formatDateParam(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildFinanceOverviewView(overview: MerchantFinanceOverviewResponse): MerchantFinanceOverviewView {
  return {
    completedOrders: overview.completed_orders || 0,
    totalGmv: buildAmountView(overview.total_gmv),
    totalIncome: buildAmountView(overview.total_merchant_receivable_amount),
    netIncome: buildAmountView(overview.net_income),
    totalServiceFee: buildAmountView(overview.total_deduction_fee_amount),
    pendingIncome: buildAmountView(overview.pending_merchant_receivable_amount)
  }
}

export function buildMerchantFinanceBillPageView(
  overview: MerchantFinanceOverviewResponse,
  response: MerchantFinanceOrdersResponse,
  range: MerchantFinanceRange,
  summaryErrorMessage = ''
): MerchantFinanceBillPageView {
  const overviewView = buildFinanceOverviewView(overview)
  const page = Number(response.page || 1)
  const totalPages = Number(response.total_pages || 0)

  return {
    rows: (response.orders || []).map(buildMerchantFinanceBillRowView),
    summary: {
      rangeLabel: buildMerchantFinanceRangeLabel(range),
      totalIncomeText: overviewView.totalIncome.text,
      totalGmvText: overviewView.totalGmv.text,
      totalOrdersText: `${overviewView.completedOrders} 单`,
      pendingIncomeText: overviewView.pendingIncome.text
    },
    summaryErrorMessage,
    page,
    totalPages,
    hasMore: page < totalPages,
    totalCount: Number(response.total || 0)
  }
}

export function buildMerchantFinanceSettlementPageView(
  response: MerchantSettlementsResponse,
  range: MerchantFinanceRange
): MerchantFinanceSettlementPageView {
  const page = Number(response.page || 1)
  const totalPages = Number(response.total_pages || 0)
  const deductionAmount = Number(response.total_platform_service_fee_amount || 0) +
    Number(response.total_payment_channel_fee_amount || 0)

  return {
    rows: (response.settlements || []).map(buildMerchantFinanceSettlementRowView),
    summary: {
      rangeLabel: buildMerchantFinanceRangeLabel(range),
      settlementAmountText: formatFenToYuan(response.total_merchant_receivable_amount),
      totalOrderAmountText: formatFenToYuan(response.total_amount),
      deductionAmountText: formatFenToYuan(deductionAmount),
      totalCountText: `${Number(response.total || 0)} 笔`
    },
    page,
    totalPages,
    hasMore: page < totalPages,
    totalCount: Number(response.total || 0)
  }
}

export function merchantFinanceSummaryUnavailableView(range: MerchantFinanceRange): MerchantFinanceBillSummaryView {
  return {
    rangeLabel: buildMerchantFinanceRangeLabel(range),
    totalIncomeText: '--',
    totalGmvText: '--',
    totalOrdersText: '--',
    pendingIncomeText: '--'
  }
}

export function buildMerchantFinanceBillPageViewWithUnavailableSummary(
  response: MerchantFinanceOrdersResponse,
  range: MerchantFinanceRange,
  summaryErrorMessage: string
): MerchantFinanceBillPageView {
  const page = Number(response.page || 1)
  const totalPages = Number(response.total_pages || 0)

  return {
    rows: (response.orders || []).map(buildMerchantFinanceBillRowView),
    summary: merchantFinanceSummaryUnavailableView(range),
    summaryErrorMessage,
    page,
    totalPages,
    hasMore: page < totalPages,
    totalCount: Number(response.total || 0)
  }
}

export function settleMerchantFinanceRequest<T>(
  request: Promise<T>
): Promise<MerchantFinanceSettledResult<T>> {
  return request.then(
    (value) => ({ status: 'fulfilled', value }),
    (reason) => ({ status: 'rejected', reason })
  )
}

export function isMerchantFinanceRequestFulfilled<T>(
  result: MerchantFinanceSettledResult<T>
): result is MerchantFinanceFulfilledResult<T> {
  return result.status === 'fulfilled'
}

export function isMerchantFinanceRequestRejected<T>(
  result: MerchantFinanceSettledResult<T>
): result is MerchantFinanceRejectedResult {
  return result.status === 'rejected'
}

export async function loadMerchantFinanceBillPage(input: {
  range?: MerchantFinanceRange
  page?: number
  limit?: number
} = {}): Promise<MerchantFinanceBillPageView> {
  const range = input.range || buildDefaultFinanceRange()
  const page = input.page || 1
  const limit = input.limit || MERCHANT_FINANCE_PAGE_SIZE

  const [overviewResult, ordersResult] = await Promise.all([
    settleMerchantFinanceRequest(getMerchantFinanceOverview(range)),
    settleMerchantFinanceRequest(listMerchantFinanceOrders({ ...range, page, limit }))
  ])

  if (isMerchantFinanceRequestRejected(ordersResult)) {
    throw ordersResult.reason
  }

  if (isMerchantFinanceRequestRejected(overviewResult)) {
    return buildMerchantFinanceBillPageViewWithUnavailableSummary(
      ordersResult.value,
      range,
      getErrorUserMessage(overviewResult.reason, '汇总同步失败，订单流水可继续查看')
    )
  }

  return buildMerchantFinanceBillPageView(overviewResult.value, ordersResult.value, range)
}

export async function loadMerchantFinanceSettlementPage(input: {
  range?: MerchantFinanceRange
  page?: number
  limit?: number
} = {}): Promise<MerchantFinanceSettlementPageView> {
  const range = input.range || buildDefaultFinanceRange()
  const page = input.page || 1
  const limit = input.limit || MERCHANT_FINANCE_PAGE_SIZE
  const response = await listMerchantSettlements({ ...range, page, limit })
  return buildMerchantFinanceSettlementPageView(response, range)
}

export function getMerchantFinanceUserMessage(error: unknown, fallback: string): string {
  return getErrorUserMessage(error, fallback)
}

function buildMerchantFinanceBillRowView(item: MerchantFinanceOrderItem): MerchantFinanceBillRowView {
  const status = getMerchantFinanceOrderStatusView(item.status)
  return {
    id: `${item.id}`,
    title: `${getOrderSourceText(item.order_source)}入账`,
    note: formatDateTimeText(item.finished_at || item.created_at),
    amountText: formatFenToYuan(item.merchant_receivable_amount || 0),
    statusText: status.text,
    statusTheme: status.theme
  }
}

function buildMerchantFinanceSettlementRowView(item: MerchantSettlementItem): MerchantFinanceSettlementRowView {
  const status = getMerchantFinanceOrderStatusView(item.status)
  return {
    id: `${item.id}`,
    title: item.out_order_no || '结算单',
    note: formatDateTimeText(item.finished_at || item.created_at),
    amountText: formatFenToYuan(item.merchant_receivable_amount || 0),
    statusText: status.text,
    statusTheme: status.theme
  }
}

function getOrderSourceText(source?: string): string {
  switch (source) {
    case 'takeout':
      return '外卖'
    case 'dine_in':
      return '堂食'
    case 'reservation':
      return '预订'
    case 'takeaway':
      return '自提'
    default:
      return '订单'
  }
}
