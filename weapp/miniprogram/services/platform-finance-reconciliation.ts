import {
  platformDashboardService,
  type PlatformProfitSharingDetailRow,
  type PlatformProfitSharingDetailsResponse,
  type PlatformProfitSharingReconciliationRow
} from '../api/platform-dashboard'

const DEFAULT_RECONCILIATION_DAYS = 30
const DEFAULT_DETAILS_PAGE_SIZE = 20

export interface PlatformFinanceReconciliationRange extends Record<string, unknown> {
  start_date: string
  end_date: string
}

export interface PlatformFinanceSummaryCardView {
  key: string
  label: string
  value: string
}

export interface PlatformFinanceReconciliationSummaryView {
  merchantFlowText: string
  riderFlowText: string
  platformCommissionText: string
  operatorCommissionText: string
  merchantShareText: string
  riderShareText: string
}

export interface PlatformProfitSharingDetailView {
  id: string
  title: string
  statusLabel: string
  statusTheme: 'success' | 'warning' | 'danger' | 'default'
  reconciliationDate: string
  totalAmountText: string
  merchantFlowText: string
  riderFlowText: string
  platformCommissionText: string
  operatorCommissionText: string
  merchantAmountText: string
  riderAmountText: string
  finishedAtText: string
}

export interface PlatformProfitSharingDetailsPageView {
  detailRows: PlatformProfitSharingDetailView[]
  detailsTotal: number
  detailsTotalText: string
  detailsPageId: number
  detailsPageSize: number
  detailsHasMore: boolean
}

export interface PlatformFinanceReconciliationPageView {
  rangeLabel: string
  summary: PlatformFinanceReconciliationSummaryView
  summaryCards: PlatformFinanceSummaryCardView[]
  detailRows: PlatformProfitSharingDetailView[]
  detailsTotal: number
  detailsTotalText: string
  detailsPageId: number
  detailsPageSize: number
  detailsHasMore: boolean
}

export function formatPlatformFinanceFen(fen?: number): string {
  const normalized = Number.isFinite(fen) ? Number(fen) : 0
  return `¥${(normalized / 100).toFixed(2)}`
}

function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildPlatformReconciliationRange(days = DEFAULT_RECONCILIATION_DAYS): PlatformFinanceReconciliationRange {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))
  return {
    start_date: formatDate(start),
    end_date: formatDate(end)
  }
}

function buildRangeLabel(range: PlatformFinanceReconciliationRange): string {
  return `${range.start_date} 至 ${range.end_date}`
}

function getStatusView(status: string): { label: string, theme: PlatformProfitSharingDetailView['statusTheme'] } {
  switch (status) {
    case 'finished':
      return { label: '已完成', theme: 'success' }
    case 'success':
    case 'succeeded':
      return { label: '成功', theme: 'success' }
    case 'processing':
      return { label: '分账中', theme: 'warning' }
    case 'pending':
      return { label: '待分账', theme: 'warning' }
    case 'failed':
      return { label: '失败', theme: 'danger' }
    default:
      return { label: '未知', theme: 'default' }
  }
}

function formatDateTime(value?: string): string {
  if (!value) {
    return '未完成'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hours = `${date.getHours()}`.padStart(2, '0')
  const minutes = `${date.getMinutes()}`.padStart(2, '0')
  return `${month}-${day} ${hours}:${minutes}`
}

function buildProfitSharingDetailRows(rows: PlatformProfitSharingDetailRow[]): PlatformProfitSharingDetailView[] {
  return rows.map((row) => {
    const status = getStatusView(row.status)
    const outOrderNo = row.out_order_no || `分账单 ${row.id}`

    return {
      id: String(row.id),
      title: outOrderNo,
      statusLabel: status.label,
      statusTheme: status.theme,
      reconciliationDate: row.reconciliation_date || '-',
      totalAmountText: formatPlatformFinanceFen(row.total_amount),
      merchantFlowText: formatPlatformFinanceFen(row.merchant_flow),
      riderFlowText: formatPlatformFinanceFen(row.rider_flow),
      platformCommissionText: formatPlatformFinanceFen(row.platform_commission),
      operatorCommissionText: formatPlatformFinanceFen(row.operator_commission),
      merchantAmountText: formatPlatformFinanceFen(row.merchant_amount),
      riderAmountText: formatPlatformFinanceFen(row.rider_amount),
      finishedAtText: formatDateTime(row.finished_at)
    }
  })
}

function buildProfitSharingDetailsPageView(
  response: PlatformProfitSharingDetailsResponse | undefined,
  fallbackPage = 1
): PlatformProfitSharingDetailsPageView {
  const pageSize = response?.page_size || DEFAULT_DETAILS_PAGE_SIZE
  const pageId = response?.page_id || fallbackPage
  const total = typeof response?.total === 'number' ? response.total : 0

  return {
    detailRows: buildProfitSharingDetailRows(response?.items || []),
    detailsTotal: total,
    detailsTotalText: `共 ${total} 条`,
    detailsPageId: pageId,
    detailsPageSize: pageSize,
    detailsHasMore: typeof response?.has_more === 'boolean'
      ? response.has_more
      : pageId * pageSize < total
  }
}

function sumValues<T>(rows: T[], selector: (row: T) => number | undefined): number {
  return rows.reduce((total, row) => {
    const value = selector(row)
    return total + (Number.isFinite(value) ? Number(value) : 0)
  }, 0)
}

function buildSummary(reconciliationRows: PlatformProfitSharingReconciliationRow[]): PlatformFinanceReconciliationSummaryView {
  return {
    merchantFlowText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_merchant_flow)),
    riderFlowText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_rider_flow)),
    platformCommissionText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_platform_commission)),
    operatorCommissionText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_operator_commission)),
    merchantShareText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_merchant_amount)),
    riderShareText: formatPlatformFinanceFen(sumValues(reconciliationRows, (row) => row.total_rider_amount))
  }
}

function buildSummaryCards(summary: PlatformFinanceReconciliationSummaryView): PlatformFinanceSummaryCardView[] {
  return [
    { key: 'merchant_flow', label: '商户流水', value: summary.merchantFlowText },
    { key: 'rider_flow', label: '骑手流水', value: summary.riderFlowText },
    { key: 'platform_share', label: '平台分账', value: summary.platformCommissionText },
    { key: 'merchant_share', label: '商户分账', value: summary.merchantShareText },
    { key: 'rider_share', label: '骑手分账', value: summary.riderShareText },
    { key: 'operator_share', label: '运营商分账', value: summary.operatorCommissionText }
  ]
}

export function buildPlatformFinanceReconciliationPageView(input: {
  range: PlatformFinanceReconciliationRange
  reconciliationRows: PlatformProfitSharingReconciliationRow[]
  detailsResponse?: PlatformProfitSharingDetailsResponse
}): PlatformFinanceReconciliationPageView {
  const detailsPage = buildProfitSharingDetailsPageView(input.detailsResponse, 1)
  const summary = buildSummary(input.reconciliationRows)
  return {
    rangeLabel: buildRangeLabel(input.range),
    summary,
    summaryCards: buildSummaryCards(summary),
    detailRows: detailsPage.detailRows,
    detailsTotal: detailsPage.detailsTotal,
    detailsTotalText: detailsPage.detailsTotalText,
    detailsPageId: detailsPage.detailsPageId,
    detailsPageSize: detailsPage.detailsPageSize,
    detailsHasMore: detailsPage.detailsHasMore
  }
}

export async function loadPlatformFinanceReconciliationPage(range = buildPlatformReconciliationRange()): Promise<PlatformFinanceReconciliationPageView> {
  const reconciliationRows = await platformDashboardService.getProfitSharingReconciliation(range)

  return buildPlatformFinanceReconciliationPageView({
    range,
    reconciliationRows
  })
}

export async function loadPlatformFinanceReconciliationDetailsPage(input: {
  range: PlatformFinanceReconciliationRange
  pageId: number
  pageSize?: number
}): Promise<PlatformProfitSharingDetailsPageView> {
  const detailsResponse = await platformDashboardService.getProfitSharingDetails({
    ...input.range,
    page_id: input.pageId,
    page_size: input.pageSize || DEFAULT_DETAILS_PAGE_SIZE
  })

  return buildProfitSharingDetailsPageView(detailsResponse, input.pageId)
}
