import {
  riderIncomeApi,
  RiderIncomeDailyItem,
  RiderIncomeDailyResponse,
  RiderIncomeLedgerItem,
  RiderIncomeLedgerResponse,
  RiderIncomeStatus,
  RiderIncomeStatusSummary,
  RiderIncomeSummaryResponse
} from '../api/rider-income'
import {
  getBaofuWithdrawalBalance,
  type BaofuWithdrawalBalanceResponse
} from '../api/baofu-withdrawal'
import RiderService from '../api/rider'
import {
  buildBaofuWithdrawalBalanceView,
  type BaofuWithdrawalBalanceView
} from './baofu-withdrawal-workflow'
import { logger } from '../utils/logger'

export type RiderIncomeStatusFilter = 'all' | RiderIncomeStatus
export type RiderIncomeTagTheme = 'default' | 'primary' | 'success' | 'warning' | 'danger'

export interface RiderIncomeDateRange {
  start_date: string
  end_date: string
}

export interface RiderIncomeStatusSummaryView {
  status: RiderIncomeStatus
  label: string
  theme: RiderIncomeTagTheme
  orderCount: number
  riderAmountDisplay: string
  deliveryFeeDisplay: string
  riderPaymentFeeDisplay: string
}

export interface RiderIncomeSummaryView {
  totalDeliveries: number
  totalRiderIncomeDisplay: string
  totalDeliveryFeeDisplay: string
  settlingAmountDisplay: string
  settlingCount: number
  failedCount: number
  statusSummary: RiderIncomeStatusSummaryView[]
}

export interface RiderIncomeLedgerItemView {
  id: number
  orderTitle: string
  merchantName: string
  amountDisplay: string
  deliveryFeeDisplay: string
  riderPaymentFeeDisplay: string
  riderAmountDisplay: string
  statusLabel: string
  statusTheme: RiderIncomeTagTheme
  createdAtLabel: string
  finishedAtLabel: string
  orderMeta: string
}

export interface RiderIncomeDailyItemView {
  dateLabel: string
  deliveryCountText: string
  dailyIncomeDisplay: string
}

export interface RiderIncomeLedgerPageView {
  items: RiderIncomeLedgerItemView[]
  pageId: number
  pageSize: number
  total: number
  hasMore: boolean
}

export interface RiderIncomePageDataView {
  settlementNotice: string
  withdrawalEntryNote: string
  withdrawalBalanceView: BaofuWithdrawalBalanceView
  summary: RiderIncomeSummaryView
  dailyItems: RiderIncomeDailyItemView[]
  ledgerPage: RiderIncomeLedgerPageView
}

export const RIDER_INCOME_PAGE_SIZE = 20

const statusOrder: RiderIncomeStatus[] = ['pending', 'processing', 'finished', 'failed']

const statusMetaMap: Record<RiderIncomeStatus, { label: string, theme: RiderIncomeTagTheme }> = {
  pending: { label: '待结算', theme: 'warning' },
  processing: { label: '分账中', theme: 'primary' },
  finished: { label: '已到账', theme: 'success' },
  failed: { label: '待处理', theme: 'danger' }
}

export function getRiderIncomeStatusMeta(status: RiderIncomeStatus) {
  return statusMetaMap[status] || { label: '未知状态', theme: 'default' as RiderIncomeTagTheme }
}

export function formatRiderIncomeFen(amount: number): string {
  if (!Number.isFinite(amount)) {
    return '0.00'
  }
  return (Math.max(amount, 0) / 100).toFixed(2)
}

function formatLocalDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildDefaultRiderIncomeDateRange(referenceDate = new Date()): RiderIncomeDateRange {
  const endDate = new Date(referenceDate.getFullYear(), referenceDate.getMonth(), referenceDate.getDate())
  const startDate = new Date(endDate)
  startDate.setDate(startDate.getDate() - 30)
  return {
    start_date: formatLocalDate(startDate),
    end_date: formatLocalDate(endDate)
  }
}

export function buildRiderIncomeDateRangeLabel(range: RiderIncomeDateRange): string {
  return `${range.start_date} 至 ${range.end_date}`
}

function parseDateTime(value?: string): Date | null {
  if (!value) {
    return null
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date
}

function formatDateTime(value?: string): string {
  const date = parseDateTime(value)
  if (!date) {
    return '--'
  }
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

function formatDate(value: string): string {
  const parts = value.split('-')
  if (parts.length !== 3) {
    return value || '--'
  }
  return `${parts[1]}-${parts[2]}`
}

function findStatusSummary(rows: RiderIncomeStatusSummary[], status: RiderIncomeStatus): RiderIncomeStatusSummary {
  return rows.find((row) => row.status === status) || {
    status,
    order_count: 0,
    rider_amount: 0,
    delivery_fee: 0,
    rider_gross_amount: 0,
    rider_payment_fee: 0
  }
}

export function buildRiderIncomeSummaryView(summary: RiderIncomeSummaryResponse): RiderIncomeSummaryView {
  const rows = summary.status_summary || []
  const pending = findStatusSummary(rows, 'pending')
  const processing = findStatusSummary(rows, 'processing')
  const failed = findStatusSummary(rows, 'failed')

  return {
    totalDeliveries: summary.total_deliveries || 0,
    totalRiderIncomeDisplay: formatRiderIncomeFen(summary.total_rider_income || 0),
    totalDeliveryFeeDisplay: formatRiderIncomeFen(summary.total_delivery_fee || 0),
    settlingAmountDisplay: formatRiderIncomeFen((pending.rider_amount || 0) + (processing.rider_amount || 0)),
    settlingCount: (pending.order_count || 0) + (processing.order_count || 0),
    failedCount: failed.order_count || 0,
    statusSummary: statusOrder.map((status) => {
      const item = findStatusSummary(rows, status)
      const meta = getRiderIncomeStatusMeta(status)
      return {
        status,
        label: meta.label,
        theme: meta.theme,
        orderCount: item.order_count || 0,
        riderAmountDisplay: formatRiderIncomeFen(item.rider_amount || 0),
        deliveryFeeDisplay: formatRiderIncomeFen(item.delivery_fee || 0),
        riderPaymentFeeDisplay: formatRiderIncomeFen(item.rider_payment_fee || 0)
      }
    })
  }
}

export function buildRiderIncomeLedgerItemView(item: RiderIncomeLedgerItem): RiderIncomeLedgerItemView {
  const meta = getRiderIncomeStatusMeta(item.status)
  return {
    id: item.id,
    orderTitle: item.order_no || `订单 #${item.order_id}`,
    merchantName: item.merchant_name || '商户',
    amountDisplay: `+¥${formatRiderIncomeFen(item.rider_amount || 0)}`,
    deliveryFeeDisplay: `¥${formatRiderIncomeFen(item.delivery_fee || 0)}`,
    riderPaymentFeeDisplay: `¥${formatRiderIncomeFen(item.rider_payment_fee || 0)}`,
    riderAmountDisplay: `¥${formatRiderIncomeFen(item.rider_amount || 0)}`,
    statusLabel: meta.label,
    statusTheme: meta.theme,
    createdAtLabel: formatDateTime(item.created_at),
    finishedAtLabel: item.finished_at ? formatDateTime(item.finished_at) : '待到账',
    orderMeta: `订单 ${item.order_no || item.order_id}`
  }
}

export function buildRiderIncomeDailyItemView(item: RiderIncomeDailyItem): RiderIncomeDailyItemView {
  return {
    dateLabel: formatDate(item.date),
    deliveryCountText: `${item.delivery_count || 0} 单`,
    dailyIncomeDisplay: `¥${formatRiderIncomeFen(item.daily_income || 0)}`
  }
}

export function buildEmptyRiderIncomeSummaryView(): RiderIncomeSummaryView {
  return buildRiderIncomeSummaryView({
    total_deliveries: 0,
    total_rider_income: 0,
    total_delivery_fee: 0,
    total_rider_gross_amount: 0,
    total_rider_payment_fee: 0,
    status_summary: []
  })
}

export function buildEmptyRiderIncomeWithdrawalBalanceView(): BaofuWithdrawalBalanceView {
  return buildBaofuWithdrawalBalanceView(null)
}

export function normalizeRiderIncomeLedgerPage(
  response: RiderIncomeLedgerResponse,
  fallbackPage: number
): RiderIncomeLedgerPageView {
  const pageSize = response.page_size || RIDER_INCOME_PAGE_SIZE
  const pageId = response.page_id || fallbackPage
  const total = typeof response.total === 'number' ? response.total : 0

  return {
    items: (response.items || []).map(buildRiderIncomeLedgerItemView),
    pageId,
    pageSize,
    total,
    hasMore: typeof response.has_more === 'boolean'
      ? response.has_more
      : pageId * pageSize < total
  }
}

export function statusFilterToParam(status: RiderIncomeStatusFilter): RiderIncomeStatus | undefined {
  return status === 'all' ? undefined : status
}

export async function loadRiderIncomePageData(input: {
  dateRange: RiderIncomeDateRange
  statusTab: RiderIncomeStatusFilter
  dailyPreviewLimit?: number
  pageSize?: number
}): Promise<RiderIncomePageDataView> {
  const pageSize = input.pageSize || RIDER_INCOME_PAGE_SIZE
  const [summary, daily, ledger, riderStatus, withdrawalBalance] = await Promise.all([
    riderIncomeApi.getSummary(input.dateRange),
    riderIncomeApi.getDaily(input.dateRange),
    riderIncomeApi.listLedger({
      ...input.dateRange,
      status: statusFilterToParam(input.statusTab),
      page_id: 1,
      page_size: pageSize
    }),
    RiderService.getStatus(),
    loadRiderIncomeWithdrawalBalance()
  ])

  return buildRiderIncomePageDataView({
    summary,
    daily,
    ledger,
    withdrawalBalance,
    settlementPaymentReady: riderStatus.settlement_account?.payment_ready,
    fallbackPage: 1,
    dailyPreviewLimit: input.dailyPreviewLimit,
    pageSize
  })
}

export async function loadRiderIncomeLedgerPage(input: {
  dateRange: RiderIncomeDateRange
  statusTab: RiderIncomeStatusFilter
  pageId: number
  pageSize?: number
}): Promise<RiderIncomeLedgerPageView> {
  const response = await riderIncomeApi.listLedger({
    ...input.dateRange,
    status: statusFilterToParam(input.statusTab),
    page_id: input.pageId,
    page_size: input.pageSize || RIDER_INCOME_PAGE_SIZE
  })
  return normalizeRiderIncomeLedgerPage(response, input.pageId)
}

function buildRiderIncomePageDataView(input: {
  summary: RiderIncomeSummaryResponse
  daily: RiderIncomeDailyResponse
  ledger: RiderIncomeLedgerResponse
  withdrawalBalance: BaofuWithdrawalBalanceResponse | null
  settlementPaymentReady?: boolean
  fallbackPage: number
  dailyPreviewLimit?: number
  pageSize: number
}): RiderIncomePageDataView {
  const withdrawalBalanceView = input.withdrawalBalance
    ? buildBaofuWithdrawalBalanceView(input.withdrawalBalance)
    : buildEmptyRiderIncomeWithdrawalBalanceView()

  return {
    settlementNotice: input.settlementPaymentReady === false
      ? '结算账户未开通，暂不能接收代取费分账订单'
      : '',
    withdrawalBalanceView,
    withdrawalEntryNote: buildRiderIncomeWithdrawalEntryNote(input.withdrawalBalance, withdrawalBalanceView),
    summary: buildRiderIncomeSummaryView(input.summary),
    dailyItems: (input.daily.items || [])
      .slice(0, input.dailyPreviewLimit || 7)
      .map(buildRiderIncomeDailyItemView),
    ledgerPage: normalizeRiderIncomeLedgerPage(input.ledger, input.fallbackPage)
  }
}

function buildRiderIncomeWithdrawalEntryNote(
  withdrawalBalance: BaofuWithdrawalBalanceResponse | null,
  withdrawalBalanceView: BaofuWithdrawalBalanceView
): string {
  if (!withdrawalBalance) {
    return '可提现余额暂不可确认'
  }
  if (withdrawalBalanceView.canSubmit) {
    return `可提现 ${withdrawalBalanceView.availableAmountText}`
  }
  return withdrawalBalanceView.disabledReason || withdrawalBalanceView.statusDesc || '暂不能提现'
}

async function loadRiderIncomeWithdrawalBalance(): Promise<BaofuWithdrawalBalanceResponse | null> {
  try {
    return await getBaofuWithdrawalBalance('rider')
  } catch (error) {
    logger.warn('Load rider income baofu withdrawal balance failed', error)
    return null
  }
}
