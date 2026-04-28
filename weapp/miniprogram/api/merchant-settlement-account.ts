import { request } from '../utils/request'
import type { StatusTagTheme } from '../utils/status-tag'

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
  need_bank_branch?: boolean
  bank_name?: string
  bank_branch_id?: string
  account_number: string
  account_name?: string
}

export interface SettlementDisplayItem {
  label: string
  value: string
}

export interface MerchantSettlementAccountView {
  normalizedStatus: string
  statusText: string
  statusDesc: string
  tagTheme: StatusTagTheme
  latestApplicationNo: string
  hasAccount: boolean
  canViewSettlementAccount: boolean
  canEditSettlementAccount: boolean
  items: SettlementDisplayItem[]
}

export interface MerchantSettlementApplicationView {
  applicationNo: string
  statusText: string
  tagTheme: StatusTagTheme
  items: SettlementDisplayItem[]
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

export function getSettlementAccountTypeText(type?: string) {
  switch (String(type || '').trim().toUpperCase()) {
    case 'ACCOUNT_TYPE_BUSINESS':
      return '对公账户'
    case 'ACCOUNT_TYPE_PRIVATE':
      return '对私账户'
    default:
      return '未知账户类型'
  }
}

export function getSettlementAccountStatusText(status?: string) {
  switch (String(status || '').trim().toLowerCase()) {
    case 'active':
      return '已开通'
    case 'not_configured':
      return '未配置'
    case 'inactive':
      return '未激活'
    default:
      return '同步中'
  }
}

export function getSettlementAccountStatusTheme(status?: string): StatusTagTheme {
  switch (String(status || '').trim().toLowerCase()) {
    case 'active':
      return 'success'
    case 'not_configured':
      return 'warning'
    case 'inactive':
      return 'danger'
    default:
      return 'default'
  }
}

function displayText(value?: string | null) {
  const normalized = String(value || '').trim()
  return normalized || '-'
}

function pushDisplayItem(items: SettlementDisplayItem[], label: string, value?: string | null) {
  const displayValue = displayText(value)
  if (displayValue === '-') {
    return
  }
  items.push({ label, value: displayValue })
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

export function buildMerchantSettlementAccountView(response?: MerchantSettlementAccountResponse | null): MerchantSettlementAccountView {
  const statusView = getSettlementAccountStatusView(response)
  const account = response?.account
  const items: SettlementDisplayItem[] = []

  if (account) {
    pushDisplayItem(items, '账户类型', getSettlementAccountTypeText(account.account_type))
    pushDisplayItem(items, '开户银行', account.account_bank)
    pushDisplayItem(items, '开户支行', account.bank_name)
    pushDisplayItem(items, '支行编号', account.bank_branch_id)
    pushDisplayItem(items, '银行账号', account.account_number)
    pushDisplayItem(items, '校验状态', getSettlementVerifyResultText(account.verify_result))
    pushDisplayItem(items, '失败原因', account.verify_fail_reason)
  }

  pushDisplayItem(items, '最新申请单', statusView.latestApplicationNo)

  return {
    normalizedStatus: statusView.normalizedStatus,
    statusText: getSettlementAccountStatusText(statusView.normalizedStatus),
    statusDesc: statusView.statusDesc,
    tagTheme: getSettlementAccountStatusTheme(statusView.normalizedStatus),
    latestApplicationNo: statusView.latestApplicationNo,
    hasAccount: Boolean(account),
    canViewSettlementAccount: statusView.canViewSettlementAccount,
    canEditSettlementAccount: statusView.canEditSettlementAccount,
    items
  }
}

export function buildMerchantSettlementApplicationView(
  response?: SettlementApplicationResponse | null,
  applicationNo: string = ''
): MerchantSettlementApplicationView {
  const normalizedApplicationNo = displayText(applicationNo)
  if (!response) {
    return {
      applicationNo: normalizedApplicationNo,
      statusText: '同步中',
      tagTheme: 'default',
      items: []
    }
  }

  const items: SettlementDisplayItem[] = []
  pushDisplayItem(items, '开户名称', response.account_name)
  pushDisplayItem(items, '账户类型', getSettlementAccountTypeText(response.account_type))
  pushDisplayItem(items, '开户银行', response.account_bank)
  pushDisplayItem(items, '开户支行', response.bank_name)
  pushDisplayItem(items, '支行编号', response.bank_branch_id)
  pushDisplayItem(items, '银行账号', response.account_number)
  pushDisplayItem(items, '审核状态', getSettlementVerifyResultText(response.verify_result))
  pushDisplayItem(items, '失败原因', response.verify_fail_reason)
  pushDisplayItem(items, '完成时间', response.verify_finish_time)

  return {
    applicationNo: normalizedApplicationNo,
    statusText: getSettlementVerifyResultText(response.verify_result),
    tagTheme: getSettlementVerifyResultTheme(response.verify_result),
    items
  }
}
