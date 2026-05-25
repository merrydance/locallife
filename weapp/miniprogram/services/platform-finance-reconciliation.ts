import {
  platformDashboardService,
  type PlatformBaofuDailyReconciliationRow,
  type PlatformProfitSharingDetailRow,
  type PlatformProfitSharingDetailsResponse,
  type PlatformProfitSharingReconciliationRow,
  type PlatformProfitSharingSlaResponse
} from '../api/platform-dashboard'
import { getBaofuWithdrawalBalance } from '../api/baofu-withdrawal'
import {
  buildBaofuWithdrawalBalanceView,
  isBaofuWithdrawalRequestFulfilled,
  settleBaofuWithdrawalRequest,
  withdrawalBalanceUnavailableView,
  type BaofuWithdrawalBalanceView
} from './baofu-withdrawal-workflow'

const DEFAULT_RECONCILIATION_DAYS = 30
const DEFAULT_DETAILS_PAGE_SIZE = 20

export interface PlatformFinanceReconciliationRange extends Record<string, unknown> {
  start_date: string
  end_date: string
}

export interface PlatformFinanceMetricView {
  label: string
  value: string
}

export type PlatformFinanceReconciliationDetailTarget = 'profitSharingDetails' | 'dailyDetails'

export interface PlatformFinanceSummaryCardView {
  key: string
  label: string
  value: string
  detailTarget: PlatformFinanceReconciliationDetailTarget
}

export interface PlatformFinanceReconciliationSummaryView {
  totalOrdersText: string
  totalProfitSharingAmountText: string
  merchantFlowText: string
  riderFlowText: string
  platformCommissionText: string
  operatorCommissionText: string
  merchantShareText: string
  riderShareText: string
  paidAmountText: string
  merchantAmountText: string
  riderAmountText: string
  withdrawSucceededText: string
  withdrawProcessingText: string
  exceptionCountText: string
  currentAvailableAmountText: string
  currentPendingAmountText: string
  currentLedgerAmountText: string
  currentFrozenAmountText: string
  balanceStatusText: string
  balanceUnavailable: boolean
}

export interface PlatformProfitSharingStatusView {
  id: string
  label: string
  theme: 'success' | 'warning' | 'danger' | 'default'
  totalOrdersText: string
  totalAmountText: string
  platformCommissionText: string
  operatorCommissionText: string
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
  providerText: string
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

export interface PlatformBaofuDailyReconciliationView {
  id: string
  date: string
  paidAmountText: string
  merchantAmountText: string
  riderAmountText: string
  platformCommissionText: string
  operatorCommissionText: string
  withdrawSucceededText: string
  withdrawProcessingText: string
  exceptionCountText: string
}

export interface PlatformFinanceReconciliationPageView {
  rangeLabel: string
  summary: PlatformFinanceReconciliationSummaryView
  summaryCards: PlatformFinanceSummaryCardView[]
  metrics: PlatformFinanceMetricView[]
  statusRows: PlatformProfitSharingStatusView[]
  detailRows: PlatformProfitSharingDetailView[]
  detailsTotal: number
  detailsTotalText: string
  detailsPageId: number
  detailsPageSize: number
  detailsHasMore: boolean
  dailyRows: PlatformBaofuDailyReconciliationView[]
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

function formatDuration(seconds?: number): string {
  const normalized = Number.isFinite(seconds) ? Math.max(Number(seconds), 0) : 0
  if (normalized < 60) {
    return `${Math.round(normalized)} 秒`
  }
  if (normalized < 3600) {
    return `${Math.round(normalized / 60)} 分钟`
  }
  return `${(normalized / 3600).toFixed(1)} 小时`
}

function getStatusView(status: string): { label: string, theme: PlatformProfitSharingStatusView['theme'] } {
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

function buildStatusRows(rows: PlatformProfitSharingReconciliationRow[]): PlatformProfitSharingStatusView[] {
  return rows.map((row) => {
    const status = getStatusView(row.status)
    return {
      id: row.status || status.label,
      label: status.label,
      theme: status.theme,
      totalOrdersText: `${Number(row.total_orders || 0)} 单`,
      totalAmountText: formatPlatformFinanceFen(row.total_amount),
      platformCommissionText: formatPlatformFinanceFen(row.total_platform_commission),
      operatorCommissionText: formatPlatformFinanceFen(row.total_operator_commission)
    }
  })
}

function buildProfitSharingDetailRows(rows: PlatformProfitSharingDetailRow[]): PlatformProfitSharingDetailView[] {
  return rows.map((row) => {
    const status = getStatusView(row.status)
    const outOrderNo = row.out_order_no || `分账单 ${row.id}`
    const provider = row.provider ? row.provider.toUpperCase() : '分账'
    const channel = row.channel || '默认通道'

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
      providerText: `${provider} · ${channel}`,
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

function buildSlaMetrics(sla: PlatformProfitSharingSlaResponse): PlatformFinanceMetricView[] {
  return [
    { label: '分账单', value: `${Number(sla.total_orders || 0)} 单` },
    { label: '已完成', value: `${Number(sla.finished_orders || 0)} 单` },
    { label: '待处理', value: `${Number(sla.pending_orders || 0)} 单` },
    { label: '失败', value: `${Number(sla.failed_orders || 0)} 单` },
    { label: '平均完成', value: formatDuration(sla.avg_finish_seconds) },
    { label: 'P95 完成', value: formatDuration(sla.p95_finish_seconds) }
  ]
}

function buildDailyRows(rows: PlatformBaofuDailyReconciliationRow[]): PlatformBaofuDailyReconciliationView[] {
  return rows.map((row) => {
    const exceptionCount = Number(row.unapplied_fact_count || 0) + Number(row.unknown_command_count || 0) + Number(row.fee_ledger_mismatch_count || 0)
    return {
      id: `${row.date}-${row.provider}-${row.channel}`,
      date: row.date,
      paidAmountText: formatPlatformFinanceFen(row.paid_amount),
      merchantAmountText: formatPlatformFinanceFen(row.merchant_amount),
      riderAmountText: formatPlatformFinanceFen(row.rider_amount),
      platformCommissionText: formatPlatformFinanceFen(row.platform_commission),
      operatorCommissionText: formatPlatformFinanceFen(row.operator_commission),
      withdrawSucceededText: formatPlatformFinanceFen(row.withdraw_succeeded_amount),
      withdrawProcessingText: formatPlatformFinanceFen(row.withdraw_processing_amount),
      exceptionCountText: `${exceptionCount} 项`
    }
  })
}

function sumValues<T>(rows: T[], selector: (row: T) => number | undefined): number {
  return rows.reduce((total, row) => {
    const value = selector(row)
    return total + (Number.isFinite(value) ? Number(value) : 0)
  }, 0)
}

function buildSummary(input: {
  reconciliationRows: PlatformProfitSharingReconciliationRow[]
  dailyRows: PlatformBaofuDailyReconciliationRow[]
  balanceView: BaofuWithdrawalBalanceView
}): PlatformFinanceReconciliationSummaryView {
  const exceptionCount = sumValues(input.dailyRows, (row) => row.unapplied_fact_count) +
    sumValues(input.dailyRows, (row) => row.unknown_command_count) +
    sumValues(input.dailyRows, (row) => row.fee_ledger_mismatch_count)
  const balanceUnavailable = input.balanceView.availableAmountText === '--'
  const balanceStatusText = input.balanceView.statusDesc ||
    (balanceUnavailable ? '当前余额暂不可确认' : '') ||
    (input.balanceView.canSubmit ? '结算账户可提现' : input.balanceView.disabledReason)

  return {
    totalOrdersText: `${sumValues(input.reconciliationRows, (row) => row.total_orders)} 单`,
    totalProfitSharingAmountText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_amount)),
    merchantFlowText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_merchant_flow)),
    riderFlowText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_rider_flow)),
    platformCommissionText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_platform_commission)),
    operatorCommissionText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_operator_commission)),
    merchantShareText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_merchant_amount)),
    riderShareText: formatPlatformFinanceFen(sumValues(input.reconciliationRows, (row) => row.total_rider_amount)),
    paidAmountText: formatPlatformFinanceFen(sumValues(input.dailyRows, (row) => row.paid_amount)),
    merchantAmountText: formatPlatformFinanceFen(sumValues(input.dailyRows, (row) => row.merchant_amount)),
    riderAmountText: formatPlatformFinanceFen(sumValues(input.dailyRows, (row) => row.rider_amount)),
    withdrawSucceededText: formatPlatformFinanceFen(sumValues(input.dailyRows, (row) => row.withdraw_succeeded_amount)),
    withdrawProcessingText: formatPlatformFinanceFen(sumValues(input.dailyRows, (row) => row.withdraw_processing_amount)),
    exceptionCountText: `${exceptionCount} 项`,
    currentAvailableAmountText: input.balanceView.availableAmountText,
    currentPendingAmountText: input.balanceView.pendingAmountText,
    currentLedgerAmountText: input.balanceView.ledgerAmountText,
    currentFrozenAmountText: input.balanceView.frozenAmountText,
    balanceStatusText,
    balanceUnavailable
  }
}

function buildSummaryCards(summary: PlatformFinanceReconciliationSummaryView): PlatformFinanceSummaryCardView[] {
  return [
    { key: 'merchant_flow', label: '商户流水', value: summary.merchantFlowText, detailTarget: 'profitSharingDetails' },
    { key: 'rider_flow', label: '骑手流水', value: summary.riderFlowText, detailTarget: 'profitSharingDetails' },
    { key: 'platform_share', label: '平台分账', value: summary.platformCommissionText, detailTarget: 'profitSharingDetails' },
    { key: 'merchant_share', label: '商户分账', value: summary.merchantShareText, detailTarget: 'profitSharingDetails' },
    { key: 'rider_share', label: '骑手分账', value: summary.riderShareText, detailTarget: 'profitSharingDetails' },
    { key: 'operator_share', label: '运营商分账', value: summary.operatorCommissionText, detailTarget: 'profitSharingDetails' }
  ]
}

export function buildPlatformFinanceReconciliationPageView(input: {
  range: PlatformFinanceReconciliationRange
  reconciliationRows: PlatformProfitSharingReconciliationRow[]
  sla: PlatformProfitSharingSlaResponse
  dailyRows: PlatformBaofuDailyReconciliationRow[]
  detailsResponse?: PlatformProfitSharingDetailsResponse
  balanceView?: BaofuWithdrawalBalanceView
}): PlatformFinanceReconciliationPageView {
  const balanceView = input.balanceView || withdrawalBalanceUnavailableView('当前可提现余额暂不可确认')
  const detailsPage = buildProfitSharingDetailsPageView(input.detailsResponse, 1)
  const summary = buildSummary({
    reconciliationRows: input.reconciliationRows,
    dailyRows: input.dailyRows,
    balanceView
  })
  return {
    rangeLabel: buildRangeLabel(input.range),
    summary,
    summaryCards: buildSummaryCards(summary),
    metrics: buildSlaMetrics(input.sla),
    statusRows: buildStatusRows(input.reconciliationRows),
    detailRows: detailsPage.detailRows,
    detailsTotal: detailsPage.detailsTotal,
    detailsTotalText: detailsPage.detailsTotalText,
    detailsPageId: detailsPage.detailsPageId,
    detailsPageSize: detailsPage.detailsPageSize,
    detailsHasMore: detailsPage.detailsHasMore,
    dailyRows: buildDailyRows(input.dailyRows)
  }
}

export async function loadPlatformFinanceReconciliationPage(range = buildPlatformReconciliationRange()): Promise<PlatformFinanceReconciliationPageView> {
  const [reconciliationRows, sla, dailyRows, balanceResult] = await Promise.all([
    platformDashboardService.getProfitSharingReconciliation(range),
    platformDashboardService.getProfitSharingSla(range),
    platformDashboardService.getBaofuDailyReconciliation(range),
    settleBaofuWithdrawalRequest(getBaofuWithdrawalBalance('platform'))
  ])

  return buildPlatformFinanceReconciliationPageView({
    range,
    reconciliationRows,
    sla,
    dailyRows,
    balanceView: isBaofuWithdrawalRequestFulfilled(balanceResult)
      ? buildBaofuWithdrawalBalanceView(balanceResult.value)
      : withdrawalBalanceUnavailableView('当前可提现余额暂不可确认')
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
