import {
  buildMerchantVoucherStatusView,
  type MerchantVoucher
} from '../api/coupon'

type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

export interface VoucherView extends MerchantVoucher {
  amount_yuan: string
  min_order_amount_yuan: string
  usage_condition_text: string
  valid_range_text: string
  remaining_quantity_text: string
  order_type_labels: string[]
  statusPending: boolean
  deletePending: boolean
}

export interface VoucherPageChunk {
  vouchers: VoucherView[]
  pageId: number
  pageSize: number
  total?: number
  hasMore?: boolean
}

export interface VoucherPageProbeResult {
  hasMore: boolean
  total: number
  nextPageCache: VoucherPageChunk | null
}

export const VOUCHERS_PAGE_SIZE = 50
export const VOUCHERS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const ALL_ORDER_TYPES: OrderType[] = ['takeout', 'dine_in', 'takeaway', 'reservation']

const ORDER_TYPE_LABEL_MAP: Record<OrderType, string> = {
  takeout: '外卖配送',
  dine_in: '堂食',
  takeaway: '到店自取',
  reservation: '预订'
}

function formatAmount(amount: number) {
  return (amount / 100).toFixed(2)
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function getOrderTypeLabels(orderTypes: string[]) {
  const normalized = (orderTypes.length ? orderTypes : ALL_ORDER_TYPES) as OrderType[]
  return normalized.map((item) => ORDER_TYPE_LABEL_MAP[item] || item)
}

export function buildVoucherView(voucher: MerchantVoucher): VoucherView {
  const statusView = buildMerchantVoucherStatusView(voucher)
  const minOrderAmountYuan = formatAmount(voucher.min_order_amount)
  const remainingQuantity = Math.max(voucher.total_quantity - voucher.claimed_quantity, 0)

  return {
    ...voucher,
    status_code: statusView.code,
    status_label: statusView.label,
    status_theme: statusView.theme,
    amount_yuan: formatAmount(voucher.amount),
    min_order_amount_yuan: minOrderAmountYuan,
    usage_condition_text: voucher.min_order_amount > 0 ? `满 ¥${minOrderAmountYuan} 可用` : '不限门槛可用',
    valid_range_text: `${formatDate(voucher.valid_from)} 至 ${formatDate(voucher.valid_until)}`,
    remaining_quantity_text: `${remainingQuantity} / ${voucher.total_quantity}`,
    order_type_labels: getOrderTypeLabels(voucher.allowed_order_types),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(visibleCount: number) {
  return `当前已加载 ${visibleCount} 张代金券`
}

export function buildVoucherPresentationUpdate(vouchers: VoucherView[]) {
  return {
    vouchers,
    resultSummaryText: buildResultSummaryText(vouchers.length),
    emptyDescription: '当前还没有代金券，先新增一个'
  }
}

export function appendVoucherViews(existingVouchers: VoucherView[], incomingVouchers: VoucherView[]) {
  if (!incomingVouchers.length) {
    return existingVouchers
  }

  const merged = [...existingVouchers]
  const seen = new Set(existingVouchers.map((item) => item.id))

  incomingVouchers.forEach((voucher) => {
    if (seen.has(voucher.id)) {
      return
    }

    seen.add(voucher.id)
    merged.push(voucher)
  })

  return merged
}

export function upsertVoucherView(vouchers: VoucherView[], voucher: MerchantVoucher) {
  const nextVoucher = buildVoucherView(voucher)
  const index = vouchers.findIndex((item) => item.id === nextVoucher.id)

  if (index === -1) {
    return [nextVoucher, ...vouchers]
  }

  const nextVouchers = [...vouchers]
  nextVouchers[index] = nextVoucher
  return nextVouchers
}

export function removeVoucherView(vouchers: VoucherView[], voucherId: number) {
  return vouchers.filter((item) => item.id !== voucherId)
}

export function hasReliableTotal(total: number | undefined, loadedCount: number) {
  return typeof total === 'number' && total > loadedCount
}

export function normalizeTotal(total: number | undefined, fallback: number) {
  return typeof total === 'number' && total >= 0 ? total : fallback
}

export function resolveTargetPageCount(pageId: number, pageSize: number, visibleCount: number) {
  const normalizedPageSize = pageSize > 0 ? pageSize : VOUCHERS_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

export function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

export function buildVoucherEditPageUrl(voucherId?: number) {
  if (voucherId && voucherId > 0) {
    return `/pages/merchant/vouchers/edit/index?id=${voucherId}`
  }

  return '/pages/merchant/vouchers/edit/index'
}