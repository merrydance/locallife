import { request } from '../utils/request'

export interface MerchantFinanceRangeParams {
  start_date: string
  end_date: string
}

export interface MerchantFinanceOverviewResponse {
  completed_orders: number
  pending_orders: number
  total_gmv: number
  total_merchant_receivable_amount: number
  total_platform_service_fee_amount: number
  total_payment_channel_fee_amount: number
  total_deduction_fee_amount: number
  pending_merchant_receivable_amount: number
  promotion_orders: number
  total_promotion_exp: number
  net_income: number
}

export interface MerchantFinanceOrderItem {
  id: number
  payment_order_id: number
  order_id?: number
  order_source: string
  total_amount: number
  platform_service_fee_amount: number
  payment_channel_fee_amount: number
  merchant_receivable_amount: number
  status: string
  created_at: string
  finished_at?: string
}

export interface MerchantFinanceOrdersResponse {
  orders: MerchantFinanceOrderItem[]
  total: number
  page: number
  limit: number
  total_pages: number
}

export interface MerchantServiceFeeItem {
  date: string
  order_source: string
  order_count: number
  total_amount: number
  platform_fee: number
  operator_fee: number
  total_fee: number
}

export interface MerchantServiceFeeSummaryResponse {
  details: MerchantServiceFeeItem[]
  total_platform_fee: number
  total_operator_fee: number
  total_service_fee: number
}

export interface MerchantPromotionExpenseItem {
  id: number
  order_no: string
  order_type: string
  subtotal: number
  delivery_fee: number
  delivery_fee_discount: number
  total_amount: number
  created_at: string
  completed_at?: string
}

export interface MerchantPromotionExpensesResponse {
  orders: MerchantPromotionExpenseItem[]
  total: number
  page: number
  limit: number
  total_pages: number
  total_promo_orders: number
  total_promo_amount: number
}

export interface MerchantDailyFinanceItem {
  date: string
  order_count: number
  total_gmv: number
  merchant_income: number
  total_fee: number
}

export interface MerchantDailyFinanceSummaryResponse {
  daily_stats: MerchantDailyFinanceItem[]
}

export interface MerchantSettlementItem {
  id: number
  payment_order_id: number
  order_source?: string
  total_amount: number
  platform_service_fee_amount: number
  payment_channel_fee_amount: number
  merchant_receivable_amount: number
  out_order_no: string
  sharing_order_id?: string
  status: string
  created_at: string
  finished_at?: string
}

export interface MerchantSettlementsResponse {
  settlements: MerchantSettlementItem[]
  total: number
  page: number
  limit: number
  total_pages: number
  total_amount: number
  total_merchant_receivable_amount: number
  total_platform_service_fee_amount: number
  total_payment_channel_fee_amount: number
}

export interface MerchantSettlementTimelineItem {
  record_type: string
  id: number
  payment_order_id: number
  order_source?: string
  total_amount: number
  platform_service_fee_amount: number
  payment_channel_fee_amount: number
  merchant_receivable_amount: number
  out_order_no: string
  sharing_order_id?: string
  status: string
  created_at: string
  finished_at?: string
  adjustment_type?: string
  related_type?: string
  related_id?: number
}

export interface MerchantSettlementTimelineResponse {
  timeline: MerchantSettlementTimelineItem[]
  total: number
  page: number
  limit: number
  total_pages: number
}

export async function getMerchantFinanceOverview(params: MerchantFinanceRangeParams): Promise<MerchantFinanceOverviewResponse> {
  return request({
    url: '/v1/merchant/finance/overview',
    method: 'GET',
    data: params
  })
}

export async function listMerchantFinanceOrders(
  params: MerchantFinanceRangeParams & { page?: number, limit?: number }
): Promise<MerchantFinanceOrdersResponse> {
  return request({
    url: '/v1/merchant/finance/orders',
    method: 'GET',
    data: params
  })
}

export async function getMerchantServiceFees(params: MerchantFinanceRangeParams): Promise<MerchantServiceFeeSummaryResponse> {
  return request({
    url: '/v1/merchant/finance/service-fees',
    method: 'GET',
    data: params
  })
}

export async function getMerchantPromotionExpenses(
  params: MerchantFinanceRangeParams & { page?: number, limit?: number }
): Promise<MerchantPromotionExpensesResponse> {
  return request({
    url: '/v1/merchant/finance/promotions',
    method: 'GET',
    data: params
  })
}

export async function getMerchantDailyFinance(params: MerchantFinanceRangeParams): Promise<MerchantDailyFinanceSummaryResponse> {
  return request({
    url: '/v1/merchant/finance/daily',
    method: 'GET',
    data: params
  })
}

export async function listMerchantSettlements(
  params: MerchantFinanceRangeParams & { status?: string, page?: number, limit?: number }
): Promise<MerchantSettlementsResponse> {
  return request({
    url: '/v1/merchant/finance/settlements',
    method: 'GET',
    data: params
  })
}

export async function listMerchantSettlementTimeline(
  params: MerchantFinanceRangeParams & { page?: number, limit?: number }
): Promise<MerchantSettlementTimelineResponse> {
  return request({
    url: '/v1/merchant/finance/settlement-timeline',
    method: 'GET',
    data: params
  })
}

export type MerchantFinanceStatusTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export function getMerchantFinanceOrderStatusView(status?: string) {
  switch (status) {
    case 'finished':
      return { text: '已完成', theme: 'success' as MerchantFinanceStatusTheme }
    case 'processing':
      return { text: '处理中', theme: 'primary' as MerchantFinanceStatusTheme }
    case 'pending':
      return { text: '待结算', theme: 'warning' as MerchantFinanceStatusTheme }
    case 'cancelled':
      return { text: '已取消', theme: 'danger' as MerchantFinanceStatusTheme }
    case 'failed':
      return { text: '失败', theme: 'danger' as MerchantFinanceStatusTheme }
    default:
      return { text: '待同步', theme: 'default' as MerchantFinanceStatusTheme }
  }
}
