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
import { formatPrice, formatPriceNoSymbol } from '../utils/util'

export type OperatorMerchantFilterStatus = MerchantStatus | ''

export interface OperatorMerchantListView extends OperatorMerchantItem {
  status_label: string
  status_theme: 'success' | 'warning' | 'default'
  rating_display: string
  order_count_display: number
  total_gmv_display: string
  commission_amount_display: string
  region_name_display: string
  category_display: string
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
  contact_person?: string
  contact_phone?: string
  address: string
  status: MerchantStatus | string
  status_theme: 'success' | 'warning' | 'default'
  category: string
  business_hours: string
  is_open: boolean
  business_state_label: string
  business_state_theme: 'success' | 'default'
  region_id: number
  region_name: string
  latitude: number
  longitude: number
  commission_rate_display: string
  created_at: string
  updated_at: string
  last_active_at?: string
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
    rating_display: Number(item.rating || 0).toFixed(1),
    order_count_display: Number(item.order_count || 0),
    total_gmv_display: formatPrice(Number(item.total_gmv || 0)),
    commission_amount_display: formatPrice(Number(item.commission_amount || 0)),
    region_name_display: item.region_name || `区域 ${item.region_id}`,
    category_display: item.category || '未分类'
  }
}

function adaptMerchantDetail(detail: OperatorMerchantDetailResponse & Record<string, unknown>): OperatorMerchantDetailView {
  const status = String(detail.status || 'pending') as MerchantStatus | string
  const statusDisplay = getMerchantStatusDisplay(status)

  return {
    id: Number(detail.id || 0),
    name: String(detail.name || '未命名商户'),
    description: detail.description ? String(detail.description) : '',
    logo_url: Array.isArray(detail.images) && detail.images.length > 0 ? String(detail.images[0]) : '',
    phone: String(detail.phone || detail.contact_phone || '-'),
    contact_person: detail.contact_person ? String(detail.contact_person) : '',
    contact_phone: detail.contact_phone ? String(detail.contact_phone) : '',
    address: String(detail.address || '-'),
    status: statusDisplay.normalizedStatus,
    status_theme: statusDisplay.theme,
    category: String(detail.category || '未分类'),
    business_hours: String(detail.business_hours || '-'),
    is_open: statusDisplay.isOpen,
    business_state_label: statusDisplay.businessStateLabel,
    business_state_theme: statusDisplay.businessStateTheme,
    region_id: Number(detail.region_id || 0),
    region_name: String(detail.region_name || `区域 ${Number(detail.region_id || 0)}`),
    latitude: Number(detail.latitude || 0),
    longitude: Number(detail.longitude || 0),
    commission_rate_display: `${(Number(detail.commission_rate || 0) / 100).toFixed(2)}%`,
    created_at: String(detail.created_at || ''),
    updated_at: String(detail.updated_at || ''),
    last_active_at: detail.last_active_at ? String(detail.last_active_at) : '',
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