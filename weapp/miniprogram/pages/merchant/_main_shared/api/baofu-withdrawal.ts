import { request } from '../../../../utils/request'
import type { BaofuAccountOwnerRole } from './baofu-account'

export interface BaofuWithdrawalBalanceResponse {
  account_status?: string
  status_desc?: string
  available_amount: number
  pending_amount: number
  ledger_amount: number
  frozen_amount: number
  min_withdraw_amount: number
  max_withdraw_amount: number
  can_withdraw: boolean
  disabled_reason?: string
}

export type BaofuWithdrawalStatus =
  | 'processing'
  | 'succeeded'
  | 'failed'
  | 'returned'
  | string

export interface BaofuWithdrawalItem {
  id: number
  out_request_no: string
  amount: number
  status: BaofuWithdrawalStatus
  status_text?: string
  sync_state?: string
  sync_message?: string
  created_at: string
  updated_at: string
}

export interface BaofuWithdrawalsResponse {
  withdrawals: BaofuWithdrawalItem[]
  total: number
  page: number
  limit: number
  total_pages: number
}

export interface CreateBaofuWithdrawalRequest {
  amount: number
  remark?: string
}

export interface CreateBaofuWithdrawalResponse {
  withdrawal: BaofuWithdrawalItem
  message?: string
}

export interface CreateBaofuWithdrawalOptions {
  idempotencyKey: string
}

export interface ListBaofuWithdrawalsParams {
  page?: number
  limit?: number
}

export function baofuWithdrawalEndpoint(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '/v1/merchant/finance/baofu-withdrawal'
    case 'platform':
      return '/v1/platform/finance/baofu-withdrawal'
    case 'operator':
      return '/v1/operators/me/finance/baofu-withdrawal'
    default:
      return '/v1/rider/income/baofu-withdrawal'
  }
}

export function getBaofuWithdrawalBalance(role: BaofuAccountOwnerRole): Promise<BaofuWithdrawalBalanceResponse> {
  return request({
    url: `${baofuWithdrawalEndpoint(role)}/balance`,
    method: 'GET'
  })
}

export function listBaofuWithdrawals(
  role: BaofuAccountOwnerRole,
  params: ListBaofuWithdrawalsParams = {}
): Promise<BaofuWithdrawalsResponse> {
  return request({
    url: `${baofuWithdrawalEndpoint(role)}/withdrawals`,
    method: 'GET',
    data: params
  })
}

export function getBaofuWithdrawal(role: BaofuAccountOwnerRole, id: number): Promise<CreateBaofuWithdrawalResponse> {
  return request({
    url: `${baofuWithdrawalEndpoint(role)}/withdrawals/${id}`,
    method: 'GET'
  })
}

export function createBaofuWithdrawal(
  role: BaofuAccountOwnerRole,
  payload: CreateBaofuWithdrawalRequest,
  options: CreateBaofuWithdrawalOptions
): Promise<CreateBaofuWithdrawalResponse> {
  if (!options?.idempotencyKey) {
    throw new Error('缺少提现请求幂等键')
  }

  return request({
    url: `${baofuWithdrawalEndpoint(role)}/withdraw`,
    method: 'POST',
    data: payload,
    header: {
      'Idempotency-Key': options.idempotencyKey
    }
  })
}
