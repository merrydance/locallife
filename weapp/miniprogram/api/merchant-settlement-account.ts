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
  latest_application_no?: string
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

export function getSettlementVerifyResultText(result?: string) {
  switch (String(result || '').trim().toUpperCase()) {
    case 'AUDIT_SUCCESS':
      return '审核通过'
    case 'AUDIT_FAIL':
      return '审核失败'
    case 'AUDITING':
      return '审核中'
    case 'VERIFY_SUCCESS':
      return '校验通过'
    case 'VERIFY_FAIL':
      return '校验失败'
    case 'VERIFYING':
      return '校验中'
    default:
      return '处理中'
  }
}

export function getSettlementVerifyResultTheme(result?: string) {
  switch (String(result || '').trim().toUpperCase()) {
    case 'AUDIT_SUCCESS':
      return 'success'
    case 'AUDIT_FAIL':
      return 'danger'
    case 'AUDITING':
      return 'warning'
    case 'VERIFY_SUCCESS':
      return 'success'
    case 'VERIFY_FAIL':
      return 'danger'
    case 'VERIFYING':
      return 'warning'
    default:
      return 'default'
  }
}

export function getSettlementAccountStatusView(response?: MerchantSettlementAccountResponse | null) {
  const normalizedStatus = String(response?.account_status || '').trim().toLowerCase()
  const normalizedVerifyResult = String(response?.account?.verify_result || '').trim().toUpperCase()
  const isActiveAccount = normalizedStatus === 'active'
  const isVerificationFailed = normalizedVerifyResult === 'VERIFY_FAIL'
  const isVerificationPending = normalizedVerifyResult === 'VERIFYING'
  const isVerificationReady = normalizedVerifyResult === '' || normalizedVerifyResult === 'VERIFY_SUCCESS'
  const hasUnknownVerificationState = isActiveAccount && !isVerificationFailed && !isVerificationPending && !isVerificationReady
  const latestApplicationNo = String(response?.latest_application_no || '').trim()

  let statusDesc = String(response?.status_desc || '').trim()
  if (!statusDesc && hasUnknownVerificationState) {
    statusDesc = '微信提现卡状态同步中，请稍后再试。'
  }

  return {
    normalizedStatus,
    normalizedVerifyResult,
    latestApplicationNo,
    isActiveAccount,
    isNotConfigured: normalizedStatus === 'not_configured',
    isVerificationFailed,
    isVerificationPending,
    hasUnknownVerificationState,
    canOpenWithdraw: isActiveAccount && isVerificationReady,
    canViewSettlementAccount: isActiveAccount,
    canEditSettlementAccount: isActiveAccount && (isVerificationReady || isVerificationFailed),
    statusDesc
  }
}