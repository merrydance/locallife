import { request } from '../utils/request'
import { uploadMedia, type MediaUploadResult } from '../utils/media'

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

export interface MerchantCancelWithdrawAccountInfo {
  out_account_type: string
  amount: number
}

export interface MerchantCancelWithdrawBlockReason {
  type: string
  description: string
}

export interface MerchantCancelWithdrawEligibilityItem {
  sub_mchid: string
  merchant_state: string
  validate_result: string
  account_info?: MerchantCancelWithdrawAccountInfo[]
  block_reasons?: MerchantCancelWithdrawBlockReason[]
}

export interface MerchantCancelWithdrawEligibilityResponse {
  account_status: string
  status_desc: string
  eligible: boolean
  eligibility?: MerchantCancelWithdrawEligibilityItem
}

export interface MerchantCancelWithdrawIdentityInfoRequest {
  id_doc_type?: string
  identification_name?: string
  identification_no?: string
}

export interface MerchantCancelWithdrawBankAccountInfoRequest {
  account_name?: string
  account_bank?: string
  bank_branch_id?: string
  bank_branch_name?: string
  account_number?: string
}

export interface MerchantCancelWithdrawPayeeInfoRequest {
  account_type?: 'ACCOUNT_TYPE_CORPORATE' | 'ACCOUNT_TYPE_PERSONAL' | string
  bank_account_info?: MerchantCancelWithdrawBankAccountInfoRequest
  identity_info?: MerchantCancelWithdrawIdentityInfoRequest
}

export interface CreateMerchantCancelWithdrawApplicationRequest {
  out_request_no?: string
  withdraw: 'NOT_APPLY_WITHDRAW' | 'APPLY_WITHDRAW' | string
  business_license_status_declaration?: 'ACTIVE' | 'CANCELED' | 'REVOKED' | string
  payee_info?: MerchantCancelWithdrawPayeeInfoRequest
  proof_media_asset_ids?: number[]
  additional_material_asset_ids?: number[]
  remark?: string
}

export interface MerchantCancelWithdrawAccountWithdrawResult {
  out_account_type: string
  pay_state: string
  state_description: string
}

export interface MerchantCancelWithdrawApplicationItem {
  id: number
  out_request_no: string
  applyment_id?: string
  sub_mchid: string
  withdraw: string
  business_license_status_declaration?: string
  remark?: string
  local_sync_state: 'created' | 'submit_succeeded' | 'submit_unknown' | 'sync_failed' | string
  cancel_state?: string
  cancel_state_description?: string
  withdraw_state?: string
  withdraw_state_description?: string
  confirm_cancel_url?: string
  account_info?: MerchantCancelWithdrawAccountInfo[]
  account_withdraw_result?: MerchantCancelWithdrawAccountWithdrawResult[]
  proof_media_asset_ids?: number[]
  additional_material_asset_ids?: number[]
  last_error?: string
  modify_time?: string
  submitted_at?: string
  last_query_at?: string
  created_at: string
  updated_at: string
}

export interface ListMerchantCancelWithdrawApplicationsResponse {
  applications: MerchantCancelWithdrawApplicationItem[]
  total: number
  page: number
  limit: number
  total_pages: number
  account_status: string
  status_desc: string
}

export interface CreateMerchantCancelWithdrawApplicationResponse {
  application: MerchantCancelWithdrawApplicationItem
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

export async function getMerchantCancelWithdrawEligibility(): Promise<MerchantCancelWithdrawEligibilityResponse> {
  return request({
    url: '/v1/merchant/finance/account/cancel-withdraw/eligibility',
    method: 'GET'
  })
}

export async function listMerchantCancelWithdrawApplications(
  page: number = 1,
  limit: number = 20
): Promise<ListMerchantCancelWithdrawApplicationsResponse> {
  return request({
    url: '/v1/merchant/finance/account/cancel-withdraw/applications',
    method: 'GET',
    data: { page, limit }
  })
}

export async function getMerchantCancelWithdrawApplication(id: number): Promise<MerchantCancelWithdrawApplicationItem> {
  return request({
    url: `/v1/merchant/finance/account/cancel-withdraw/applications/${id}`,
    method: 'GET'
  })
}

export async function createMerchantCancelWithdrawApplication(
  payload: CreateMerchantCancelWithdrawApplicationRequest
): Promise<CreateMerchantCancelWithdrawApplicationResponse> {
  return request({
    url: '/v1/merchant/finance/account/cancel-withdraw/applications',
    method: 'POST',
    data: payload
  })
}

export function uploadMerchantCancelWithdrawMaterial(filePath: string): Promise<MediaUploadResult> {
  return uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory: 'merchant_cancel_withdraw'
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

export type MerchantFinanceStatusTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export function getMerchantAccountStatusView(status?: string, statusDesc?: string) {
  const normalizedStatus = String(status || '').trim().toLowerCase()
  return {
    normalizedStatus,
    isActive: normalizedStatus === 'active',
    statusDesc: statusDesc || ''
  }
}

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

export function getMerchantWithdrawStatusView(status?: string) {
  switch (status) {
    case 'pending':
      return { text: '处理中', theme: 'warning' as MerchantFinanceStatusTheme }
    case 'success':
      return { text: '成功', theme: 'success' as MerchantFinanceStatusTheme }
    case 'failed':
      return { text: '失败', theme: 'danger' as MerchantFinanceStatusTheme }
    default:
      return { text: '待同步', theme: 'default' as MerchantFinanceStatusTheme }
  }
}

