import {
  formatOnlineStatus,
  getRiderStatusDisplay,
  operatorRiderManagementService,
  parseRiderStatusFilter,
  type OperatorRiderDetailResponse,
  type OperatorRiderItem,
  type RiderQueryParams,
  type RiderStatsResponse,
  type RiderStatus
} from '../_api/operator-rider-management'
import { formatPrice, formatPriceNoSymbol } from '../../../utils/util'

export type OperatorRiderFilterStatus = RiderStatus | ''

export interface OperatorRiderListView {
  id: number
  name: string
  phone: string
  status: string
  status_label: string
  status_theme: 'success' | 'warning' | 'danger' | 'default'
  is_online: boolean
  online_status_label: string
  region_id: number
  delivery_count: number
  total_earnings_display: string
}

export interface OperatorRiderListPageData {
  riders: OperatorRiderListView[]
  total: number
  nextPage: number
  hasMore: boolean
}

export interface OperatorRiderDetailView {
  id: number
  user_id: number
  real_name: string
  phone: string
  id_card_no?: string
  status: RiderStatus | string
  status_theme: 'success' | 'warning' | 'danger' | 'default'
  is_online: boolean
  online_status_label: string
  online_status_theme: 'success' | 'default'
  region_id: number
  deposit_amount: number
  frozen_deposit: number
  total_orders: number
  total_earnings: number
  current_latitude?: number
  current_longitude?: number
  location_updated_at?: string
  credit_score: number
  created_at: string
  updated_at: string
  deposit_amount_display: string
  frozen_deposit_display: string
  total_earnings_display: string
  status_label: string
}

export type OperatorRiderStatsView = RiderStatsResponse & {
  period_earnings_display: string
  completion_rate_display: string
  avg_delivery_min: string
}

function adaptOperatorRider(item: Partial<OperatorRiderItem> & Record<string, unknown>): OperatorRiderListView {
  const name = String(item.real_name || '未命名骑手')
  const onlineStatus = String(item.online_status || ((item.is_online as boolean) ? 'online' : 'offline'))
  const isOnline = onlineStatus === 'online' || Boolean(item.is_online)
  const deliveryCount = Number(item.total_orders || 0)
  const statusDisplay = getRiderStatusDisplay(String(item.status || ''))

  return {
    id: Number(item.id || 0),
    name,
    phone: String(item.phone || '-'),
    status: statusDisplay.normalizedStatus,
    status_label: statusDisplay.label,
    status_theme: statusDisplay.theme,
    is_online: isOnline,
    online_status_label: formatOnlineStatus(isOnline ? 'online' : 'offline'),
    region_id: Number(item.region_id || 0),
    delivery_count: deliveryCount,
    total_earnings_display: formatPrice(Number(item.total_earnings || 0))
  }
}

export function parseOperatorRiderStatusFilter(status?: string): OperatorRiderFilterStatus {
  return parseRiderStatusFilter(status)
}

export async function loadOperatorRiderListPageData(params: {
  pageId: number
  pageSize: number
  regionId?: number
  statusFilter?: OperatorRiderFilterStatus
  searchKeyword?: string
}): Promise<OperatorRiderListPageData> {
  const keyword = params.searchKeyword?.trim() || undefined
  const query: RiderQueryParams = {
    page: params.pageId,
    limit: params.pageSize,
    keyword,
    status: params.statusFilter || undefined,
    ...(params.regionId ? { region_id: params.regionId } : {})
  }

  const result = await operatorRiderManagementService.getRiderList(query)
  const riders = (result.riders || []).map((item) => adaptOperatorRider(item as Partial<OperatorRiderItem> & Record<string, unknown>))
  const total = Number(result.total || riders.length)

  return {
    riders,
    total,
    nextPage: params.pageId + 1,
    hasMore: riders.length < total
  }
}

export async function loadOperatorRiderDetailView(id: number): Promise<OperatorRiderDetailView> {
  const detail = await operatorRiderManagementService.getRiderDetail(id)
  return adaptOperatorRiderDetail(detail as OperatorRiderDetailResponse & Record<string, unknown>)
}

export async function loadOperatorRiderStatsView(id: number, days = 30): Promise<OperatorRiderStatsView> {
  const stats = await operatorRiderManagementService.getRiderStats(id, days)
  return {
    ...stats,
    period_earnings_display: formatPriceNoSymbol(stats.period_earnings),
    completion_rate_display: (stats.completion_rate_basis_points / 100).toFixed(1),
    avg_delivery_min: (stats.avg_delivery_seconds / 60).toFixed(1)
  }
}

function adaptOperatorRiderDetail(detail: OperatorRiderDetailResponse & Record<string, unknown>): OperatorRiderDetailView {
  const status = String(detail.status || '') as RiderStatus | string
  const onlineStatus = detail.is_online ? 'online' : 'offline'
  const statusDisplay = getRiderStatusDisplay(status as RiderStatus)
  const totalOrders = Number(detail.total_orders || 0)
  const totalEarnings = Number(detail.total_earnings || 0)
  const depositAmount = Number(detail.deposit_amount || 0)
  const frozenDeposit = Number(detail.frozen_deposit || 0)
  const currentLatitude = Number(detail.current_latitude || 0)
  const currentLongitude = Number(detail.current_longitude || 0)
  const locationUpdatedAt = String(detail.location_updated_at || '')

  return {
    id: Number(detail.id || 0),
    user_id: Number(detail.user_id || 0),
    real_name: String(detail.real_name || '未命名骑手'),
    phone: String(detail.phone || '-'),
    id_card_no: String(detail.id_card_no || ''),
    status: statusDisplay.normalizedStatus,
    status_theme: statusDisplay.theme,
    is_online: onlineStatus === 'online',
    online_status_label: formatOnlineStatus(onlineStatus),
    online_status_theme: onlineStatus === 'online' ? 'success' : 'default',
    region_id: Number(detail.region_id || 0),
    deposit_amount: depositAmount,
    frozen_deposit: frozenDeposit,
    total_orders: totalOrders,
    total_earnings: totalEarnings,
    current_latitude: currentLatitude || undefined,
    current_longitude: currentLongitude || undefined,
    location_updated_at: locationUpdatedAt,
    credit_score: Number(detail.credit_score || 0),
    created_at: String(detail.created_at || ''),
    updated_at: String(detail.updated_at || ''),
    deposit_amount_display: formatPriceNoSymbol(depositAmount),
    frozen_deposit_display: formatPriceNoSymbol(frozenDeposit),
    total_earnings_display: formatPriceNoSymbol(totalEarnings),
    status_label: statusDisplay.label
  }
}
