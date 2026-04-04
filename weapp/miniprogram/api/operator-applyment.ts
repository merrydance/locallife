import { request } from '../utils/request'
import { getOperatorApplication } from './operator-application'

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
  contact_phone: string
  contact_email?: string
}

export interface OperatorBindBankResponse {
  applyment_id: number
  status: string
  message: string
}

export interface OperatorApplymentStatusResponse {
  status: string
  status_desc: string
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
  try {
    return await request<OperatorApplymentStatusResponse>({
      url: '/v1/operator/applyment/status',
      method: 'GET'
    })
  } catch (error: unknown) {
    if (!isNotFoundError(error)) {
      throw error
    }

    const application = await getOperatorApplication()
    return {
      status: application.status,
      status_desc: mapApplicationStatusDesc(application.status),
      reject_reason: application.reject_reason,
      created_at: application.created_at,
      updated_at: application.updated_at
    }
  }
}

function isNotFoundError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false
  }
  return error.message.includes('(404)') || error.message.includes('服务未找到')
}

function mapApplicationStatusDesc(status: string): string {
  switch (status) {
  case 'bindbank_submitted':
    return '开户审核中'
  case 'active':
    return '可选择开户'
  default:
    return status || '未提交'
  }
}
