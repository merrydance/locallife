import { request } from '../utils/request'

export interface MerchantAccountBalanceResponse {
  sub_mch_id: string
  available_amount: number
  pending_amount: number
  withdrawable_amount: number
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
  total_count: number
  total: number
  page: number
  limit: number
  total_pages: number
}

export interface CreateMerchantWithdrawResponse {
  withdrawal: MerchantWithdrawItem
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

export async function listMerchantWithdrawals(page: number = 1, limit: number = 20): Promise<ListMerchantWithdrawalsResponse> {
  return request({
    url: '/v1/merchant/finance/account/withdrawals',
    method: 'GET',
    data: { page, limit }
  })
}
