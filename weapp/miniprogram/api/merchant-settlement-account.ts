import { request } from '../utils/request'

export interface MerchantSettlementAccountInfo {
  account_type: string
  account_bank: string
  bank_name?: string
  bank_branch_id?: string
  account_number: string
  verify_result: string
  verify_fail_reason?: string
}

export interface MerchantSettlementAccountResponse {
  account_status: string
  status_desc?: string
  account?: MerchantSettlementAccountInfo
}

export interface ModifyMerchantSettlementAccountRequest {
  account_type: 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'
  account_bank: string
  bank_name?: string
  bank_branch_id?: string
  account_number: string
  account_name?: string
}

export interface ModifyMerchantSettlementAccountResponse {
  application_no: string
}

export interface SettlementApplicationResponse {
  account_name: string
  account_type: string
  account_bank: string
  bank_name?: string
  bank_branch_id?: string
  account_number: string
  verify_result: string
  verify_fail_reason?: string
  verify_finish_time?: string
}

export function getMerchantSettlementAccount(): Promise<MerchantSettlementAccountResponse> {
  return request({
    url: '/v1/merchant/finance/account/settlement-account',
    method: 'GET'
  })
}

export function modifyMerchantSettlementAccount(
  payload: ModifyMerchantSettlementAccountRequest
): Promise<ModifyMerchantSettlementAccountResponse> {
  return request({
    url: '/v1/merchant/finance/account/settlement-account',
    method: 'POST',
    data: payload
  })
}

export function getMerchantSettlementApplication(
  applicationNo: string
): Promise<SettlementApplicationResponse> {
  return request({
    url: `/v1/merchant/finance/account/settlement-account/applications/${applicationNo}`,
    method: 'GET'
  })
}

export function getSettlementAccountStatusView(status?: string, statusDesc?: string) {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  return {
    normalizedStatus,
    isActive: normalizedStatus === 'active',
    isNotConfigured: normalizedStatus === 'not_configured',
    statusDesc: statusDesc || ''
  }
}