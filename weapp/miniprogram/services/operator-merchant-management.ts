import {
  getMerchantStatusDisplay,
  operatorMerchantManagementService,
  parseMerchantStatusFilter,
  type MerchantQueryParams,
  type MerchantStatsResponse,
  type MerchantStatus,
  type OperatorMerchantDetailResponse,
  type OperatorMerchantItem
} from '../api/operator-merchant-management'
import { formatPriceNoSymbol } from '../utils/util'

export type OperatorMerchantFilterStatus = MerchantStatus | ''

export interface OperatorMerchantListView extends OperatorMerchantItem {
  status_label: string
  status_theme: 'success' | 'warning' | 'default'
  business_state_label: string
  business_state_theme: 'success' | 'default'
}

export interface OperatorMerchantListPageData {
  merchants: OperatorMerchantListView[]
  total: number
  nextPage: number
  hasMore: boolean
}

export interface OperatorMerchantDetailView {
  id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  address: string
  status: MerchantStatus | string
  status_theme: 'success' | 'warning' | 'default'
  is_open: boolean
  business_state_label: string
  business_state_theme: 'success' | 'default'
  region_id: number
  latitude: number
  longitude: number
  created_at: string
  updated_at: string
  status_label: string
}

export type OperatorMerchantStatsView = MerchantStatsResponse & {
  total_sales_display: string
  total_commission_display: string
  avg_daily_sales_display: string
  repurchase_rate_display: string
  avg_orders_per_user_display: string
  top_dishes_with_revenue: Array<{ dish_name: string, total_sold: number, total_revenue_display: string }>
}

function adaptMerchantItem(item: OperatorMerchantItem): OperatorMerchantListView {
  const statusDisplay = getMerchantStatusDisplay(item.status)
  return {
    ...item,
    status: statusDisplay.normalizedStatus,
    status_label: statusDisplay.label,
    status_theme: statusDisplay.theme,
    business_state_label: item.is_open ? '营业中' : '未营业',
    business_state_theme: item.is_open ? 'success' : 'default'
  }
}

function adaptMerchantDetail(detail: OperatorMerchantDetailResponse & Record<string, unknown>): OperatorMerchantDetailView {
  const status = String(detail.status || 'pending') as MerchantStatus | string
  const statusDisplay = getMerchantStatusDisplay(status)

  return {
    id: Number(detail.id || 0),
    name: String(detail.name || '未命名商户'),
    description: detail.description ? String(detail.description) : '',
    logo_url: detail.logo_url ? String(detail.logo_url) : '',
    phone: String(detail.phone || '-'),
    address: String(detail.address || '-'),
    status: statusDisplay.normalizedStatus,
    status_theme: statusDisplay.theme,
    is_open: Boolean(detail.is_open),
    business_state_label: detail.is_open ? '营业中' : '未营业',
    business_state_theme: detail.is_open ? 'success' : 'default',
    region_id: Number(detail.region_id || 0),
    latitude: Number(detail.latitude || 0),
    longitude: Number(detail.longitude || 0),
    created_at: String(detail.created_at || ''),
    updated_at: String(detail.updated_at || ''),
    status_label: statusDisplay.label
  }
}

export function parseOperatorMerchantStatusFilter(status?: string): OperatorMerchantFilterStatus {
  return parseMerchantStatusFilter(status)
}

export async function loadOperatorMerchantListPageData(params: {
  pageId: number
  pageSize: number
  regionId?: number
  statusFilter?: OperatorMerchantFilterStatus
  searchKeyword?: string
}): Promise<OperatorMerchantListPageData> {
  const query: MerchantQueryParams = {
    page: params.pageId,
    limit: params.pageSize,
    keyword: params.searchKeyword || undefined,
    status: params.statusFilter || undefined,
    sort_by: 'created_at',
    sort_order: 'desc',
    ...(params.regionId ? { region_id: params.regionId } : {})
  }

  const result = await operatorMerchantManagementService.getMerchantList(query)
  const merchants = (result.merchants || []).map(adaptMerchantItem)
  const total = Number(result.total || merchants.length)

  return {
    merchants,
    total,
    nextPage: params.pageId + 1,
    hasMore: merchants.length < total
  }
}

export async function loadOperatorMerchantDetailView(id: number): Promise<OperatorMerchantDetailView> {
  const raw = await operatorMerchantManagementService.getMerchantDetail(id)
  return adaptMerchantDetail(raw as OperatorMerchantDetailResponse & Record<string, unknown>)
}

export async function loadOperatorMerchantStatsView(id: number, days = 30): Promise<OperatorMerchantStatsView> {
  const stats = await operatorMerchantManagementService.getMerchantStats(id, days)
  return {
    ...stats,
    total_sales_display: formatPriceNoSymbol(stats.total_sales),
    total_commission_display: formatPriceNoSymbol(stats.total_commission),
    avg_daily_sales_display: formatPriceNoSymbol(stats.avg_daily_sales),
    repurchase_rate_display: (stats.repurchase_rate_basis_points / 100).toFixed(1),
    avg_orders_per_user_display: (stats.avg_orders_per_user_cents / 100).toFixed(2),
    top_dishes_with_revenue: (stats.top_dishes ?? []).map((dish) => ({
      dish_name: dish.dish_name,
      total_sold: dish.total_sold,
      total_revenue_display: formatPriceNoSymbol(dish.total_revenue)
    }))
  }
}