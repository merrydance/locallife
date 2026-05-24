import { operatorBasicManagementService } from '../api/operator-basic-management'

const DEFAULT_OPERATOR_BILL_DAYS = 30
export const OPERATOR_COMMISSION_BILL_PAGE_SIZE = 20

export interface OperatorCommissionBillRange extends Record<string, unknown> {
  start_date: string
  end_date: string
}

export interface OperatorCommissionRowView {
  date: string
  order_count: number
  total_gmv_fen: number
  total_commission_fen: number
}

interface CommissionListResponseLike {
  commissions?: Array<{
    items?: Array<{
      date: string
      order_count: number
      total_gmv: number
      commission: number
    }>
  }>
  items?: Array<{
    date: string
    order_count: number
    total_gmv: number
    commission: number
  }>
}

export interface OperatorFinancePageData {
  totalIncomeFen: number
  currentMonthIncomeFen: number
  currentMonthGmvFen: number
  currentMonthOrders: number
  currentMonthCommissionFen: number
  operatorShareRatio: number
  loadError: string
  commissionRows: OperatorCommissionRowView[]
  commissionError: string
}

export interface OperatorCommissionBillSummaryView {
  rangeLabel: string
  totalCommissionText: string
  totalGmvText: string
  totalOrdersText: string
}

export interface OperatorCommissionBillRowView {
  id: string
  date: string
  commissionText: string
  totalGmvText: string
  orderCountText: string
  commissionRateText: string
}

export interface OperatorCommissionBillPageView {
  rows: OperatorCommissionBillRowView[]
  summary: OperatorCommissionBillSummaryView
  page: number
  totalPages: number
  hasMore: boolean
  totalCount: number
}

interface CommissionPageResponseLike {
  items?: Array<{
    date: string
    order_count: number
    total_gmv: number
    commission: number
    commission_rate?: string
  }>
  summary?: {
    total_gmv?: number
    total_commission?: number
    total_orders?: number
  }
  total?: number
  total_count?: number
  page?: number
  limit?: number
}

function adaptCommissionRows(response: unknown): OperatorCommissionRowView[] {
  if (!response || typeof response !== 'object') {
    return []
  }

  const payload = response as CommissionListResponseLike
  const rawItems = Array.isArray(payload.items)
    ? payload.items
    : Array.isArray(payload.commissions)
      ? payload.commissions.reduce<Array<{ date: string, order_count: number, total_gmv: number, commission: number }>>((accumulator, item) => {
        if (Array.isArray(item.items)) {
          accumulator.push(...item.items)
        }
        return accumulator
      }, [])
      : []

  return rawItems.slice(0, 10).map((item) => ({
    date: item.date,
    order_count: Number(item.order_count || 0),
    total_gmv_fen: Number(item.total_gmv || 0),
    total_commission_fen: Number(item.commission || 0)
  }))
}

export function formatOperatorFinanceFen(fen?: number): string {
  const normalized = Number.isFinite(fen) ? Number(fen) : 0
  return `¥${(normalized / 100).toFixed(2)}`
}

function formatOperatorFinanceDate(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildOperatorCommissionBillRange(days = DEFAULT_OPERATOR_BILL_DAYS): OperatorCommissionBillRange {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))
  return {
    start_date: formatOperatorFinanceDate(start),
    end_date: formatOperatorFinanceDate(end)
  }
}

export function buildOperatorCommissionBillMonthRange(): OperatorCommissionBillRange {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth(), 1)
  return {
    start_date: formatOperatorFinanceDate(start),
    end_date: formatOperatorFinanceDate(end)
  }
}

function buildRangeLabel(range: OperatorCommissionBillRange): string {
  return `${range.start_date} 至 ${range.end_date}`
}

function formatCommissionRateText(rate?: string): string {
  if (!rate || rate === 'N/A') {
    return '无交易'
  }
  return rate
}

function getCommissionPageResponse(response: unknown): CommissionPageResponseLike {
  if (!response || typeof response !== 'object') {
    return {}
  }
  return response as CommissionPageResponseLike
}

export function buildOperatorCommissionBillPageView(
  response: unknown,
  range: OperatorCommissionBillRange,
  fallbackPage: number,
  fallbackLimit: number
): OperatorCommissionBillPageView {
  const payload = getCommissionPageResponse(response)
  const items = Array.isArray(payload.items) ? payload.items : []
  const page = Number(payload.page || fallbackPage || 1)
  const limit = Number(payload.limit || fallbackLimit || OPERATOR_COMMISSION_BILL_PAGE_SIZE)
  const totalCount = Number(payload.total_count ?? items.length)
  const totalPages = limit > 0 ? Math.ceil(totalCount / limit) : 0

  return {
    rows: items.map((item) => ({
      id: item.date,
      date: item.date,
      commissionText: formatOperatorFinanceFen(item.commission),
      totalGmvText: formatOperatorFinanceFen(item.total_gmv),
      orderCountText: `${Number(item.order_count || 0)} 单`,
      commissionRateText: formatCommissionRateText(item.commission_rate)
    })),
    summary: {
      rangeLabel: buildRangeLabel(range),
      totalCommissionText: formatOperatorFinanceFen(payload.summary?.total_commission),
      totalGmvText: formatOperatorFinanceFen(payload.summary?.total_gmv),
      totalOrdersText: `${Number(payload.summary?.total_orders || 0)} 单`
    },
    page,
    totalPages,
    hasMore: page < totalPages,
    totalCount
  }
}

export async function loadOperatorFinancePageData(): Promise<OperatorFinancePageData> {
  const [overview, commissionList] = await Promise.all([
    operatorBasicManagementService.getFinanceOverview().catch(() => null),
    operatorBasicManagementService.getCommissionList({ page: 1, limit: 10 }).catch(() => null)
  ])

  return {
    totalIncomeFen: overview?.total?.operator_income ?? 0,
    currentMonthIncomeFen: overview?.current_month?.operator_income ?? 0,
    currentMonthGmvFen: overview?.current_month?.total_gmv ?? 0,
    currentMonthOrders: overview?.current_month?.total_orders ?? 0,
    currentMonthCommissionFen: overview?.current_month?.total_commission ?? 0,
    operatorShareRatio: overview?.operator_share_ratio ?? 0,
    loadError: overview ? '' : '收入概览加载失败，请稍后重试',
    commissionRows: adaptCommissionRows(commissionList),
    commissionError: commissionList ? '' : '佣金明细加载失败，请稍后重试'
  }
}

export async function loadOperatorCommissionBillPage(params: {
  page?: number
  limit?: number
  range?: OperatorCommissionBillRange
} = {}): Promise<OperatorCommissionBillPageView> {
  const page = params.page || 1
  const limit = params.limit || OPERATOR_COMMISSION_BILL_PAGE_SIZE
  const range = params.range || buildOperatorCommissionBillRange()
  const response = await operatorBasicManagementService.getCommissionList({
    ...range,
    page,
    limit
  })
  return buildOperatorCommissionBillPageView(response, range, page, limit)
}
