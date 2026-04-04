import { request } from '../utils/request'

export interface MerchantAccountBalanceResponse {
  sub_mch_id: string
  available_amount: number
  pending_amount: number
  withdrawable_amount: number
  account_status: string
  status_desc: string
}

export interface MerchantWithdrawRequest {
  amount: number
  remark: string
  out_request_no?: string
}

export interface MerchantWithdrawItem {
  id: number
  amount: number
  status: 'pending' | 'success' | 'failed' | string
  channel: string
  out_request_no?: string
  withdraw_id?: string
  sub_mch_id?: string
  reason?: string
  created_at: string
  updated_at: string
}

export interface ListMerchantWithdrawalsResponse {
  withdrawals: MerchantWithdrawItem[]
  total: number
  page: number
  limit: number
  total_pages: number
  account_status: string
  status_desc: string
}

export interface CreateMerchantWithdrawResponse {
  withdrawal: MerchantWithdrawItem
  wechat?: {
    status?: string
    withdraw_id?: string
    out_request_no?: string
    fail_reason?: string
  }
}

export interface MerchantFinanceRangeParams {
  start_date: string
  end_date: string
}

export interface MerchantFinanceOverviewResponse {
  completed_orders: number
  pending_orders: number
  total_gmv: number
  total_income: number
  total_platform_fee: number
  total_operator_fee: number
  total_service_fee: number
  pending_income: number
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
  platform_commission: number
  operator_commission: number
  merchant_amount: number
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
  platform_commission: number
  operator_commission: number
  merchant_amount: number
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
  total_merchant_amount: number
  total_platform_fee: number
  total_operator_fee: number
}

export interface MerchantSettlementTimelineItem {
  record_type: string
  id: number
  payment_order_id: number
  order_source?: string
  total_amount: number
  platform_commission: number
  operator_commission: number
  merchant_amount: number
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

export async function getMerchantAccountBalance(): Promise<MerchantAccountBalanceResponse> {
  return request({
    url: '/v1/merchant/finance/account/balance',
    method: 'GET'
  })
}

export async function createMerchantWithdraw(payload: MerchantWithdrawRequest): Promise<CreateMerchantWithdrawResponse> {
  return request({
    url: '/v1/merchant/finance/account/withdraw',
    method: 'POST',
    data: payload
  })
}

export async function getMerchantWithdrawal(id: number): Promise<MerchantWithdrawItem> {
  return request({
    url: `/v1/merchant/finance/account/withdrawals/${id}`,
    method: 'GET'
  })
}

export async function listMerchantWithdrawals(page: number = 1, limit: number = 20): Promise<ListMerchantWithdrawalsResponse> {
  return request({
    url: '/v1/merchant/finance/account/withdrawals',
    method: 'GET',
    data: { page, limit }
  })
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

/* ─────────────────────────── 收付通进件 ─────────────────────────── */

export interface ApplymentStatusResponse {
  status: string
  status_desc: string
  can_submit?: boolean
  block_reason?: string
  sign_url?: string
  sub_mch_id?: string
  reject_reason?: string
}

export interface MerchantBindBankRequest {
  account_type: 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'
  account_bank: string
  account_bank_code?: number
  bank_alias?: string
  bank_alias_code?: string
  need_bank_branch?: boolean
  bank_address_code?: string
  bank_branch_id?: string
  bank_name?: string
  account_number: string
  account_name: string
}

export interface MerchantBindBankResponse {
  applyment_id: number
  status: string
  message: string
}

export async function getMerchantApplymentStatus(): Promise<ApplymentStatusResponse> {
  return request({
    url: '/v1/merchant/applyment/status',
    method: 'GET'
  })
}

export async function merchantBindBank(payload: MerchantBindBankRequest): Promise<MerchantBindBankResponse> {
  return request({
    url: '/v1/merchant/applyment/bindbank',
    method: 'POST',
    data: payload
  })
}
