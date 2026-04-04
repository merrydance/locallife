import { request } from '../utils/request'

export interface OperatorBindBankRequest {
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

export interface OperatorBindBankResponse {
  applyment_id: number
  status: string
  message: string
}

export interface OperatorApplymentStatusResponse {
  status: string
  status_desc: string
  can_submit?: boolean
  block_reason?: string
  applyment_id?: number
  sub_mch_id?: string
  sign_url?: string
  reject_reason?: string
  created_at: string
  updated_at: string
}

export const operatorBindBank = (data: OperatorBindBankRequest) => {
  return request<OperatorBindBankResponse>({
    url: '/v1/operator/applyment/bindbank',
    method: 'POST',
    data
  })
}

export const getOperatorApplymentStatus = async (): Promise<OperatorApplymentStatusResponse> => {
  return request<OperatorApplymentStatusResponse>({
    url: '/v1/operator/applyment/status',
    method: 'GET'
  })
}
