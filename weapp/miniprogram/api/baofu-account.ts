import { request } from '../utils/request'
import type { MiniProgramPayParams } from './payment'
import type { StatusTagTheme } from '../utils/status-tag'

export type BaofuAccountOwnerRole = 'rider' | 'merchant' | 'operator' | 'platform'

export type BaofuSettlementAccountStatus =
  | 'ready'
  | 'failed'
  | 'profile_pending'
  | 'verify_fee_pending'
  | 'verify_fee_processing'
  | 'opening_processing'
  | 'merchant_report_processing'
  | 'applet_auth_pending'
  | 'voided'

export type BaofuSettlementAccountProfileStatus =
  | 'complete'
  | 'incomplete'

export type BaofuSettlementAccountOpenState =
  | 'active'
  | 'processing'
  | 'failed'
  | 'abnormal'

export interface BaofuAccountProfile {
  legal_name?: string
  business_license_number?: string
  legal_person_name?: string
  legal_person_id_number?: string
  corporate_mobile?: string
  email?: string
  bank_account_no?: string
  account_name?: string
  real_name?: string
  certificate_no?: string
  id_card_number?: string
  bank_mobile?: string
  mobile?: string
  phone?: string
  bank_account_number?: string
  account_number?: string
  bank_name?: string
  deposit_bank_province?: string
  deposit_bank_city?: string
  deposit_bank_name?: string
  contact_name?: string
  contact_mobile?: string
  card_user_name?: string
  self_employed?: boolean
}

export interface BaofuSettlementAccountPayment {
  payment_order_id?: number
  amount?: number
  business_type?: 'baofu_account_verify_fee' | string
  out_trade_no?: string
  pay_params?: MiniProgramPayParams
  expires_at?: string
}

export interface BaofuSettlementAccountProfileDefaults {
  source?: 'wechat_applyment' | string
  legal_name?: string
  certificate_no_mask?: string
  business_license_number?: string
  legal_person_name?: string
  card_user_name?: string
  self_employed?: boolean
  legal_person_id_number_mask?: string
  corporate_mobile_mask?: string
  email_mask?: string
  bank_account_no_mask?: string
  bank_name?: string
  deposit_bank_province?: string
  deposit_bank_city?: string
  deposit_bank_name?: string
  bank_address_code?: string
  bank_branch_id?: string
  account_bank?: string
  account_bank_code?: number
  bank_alias?: string
  bank_alias_code?: string
  contact_name?: string
  contact_mobile_mask?: string
  has_legal_person_id_number?: boolean
  has_corporate_mobile?: boolean
  has_certificate_no?: boolean
  has_email?: boolean
  has_bank_account_no?: boolean
  has_contact_mobile?: boolean
  has_saved_sensitive_defaults?: boolean
}

export interface BaofuSettlementAccountResponse {
  owner_type: 'merchant' | 'platform' | 'rider' | 'operator'
  owner_id: number
  account_type: 'personal' | 'business'
  status: BaofuSettlementAccountStatus
  state: BaofuSettlementAccountStatus
  status_desc?: string
  label?: string
  payment_ready?: boolean
  open_state?: BaofuSettlementAccountOpenState
  profile_status?: BaofuSettlementAccountProfileStatus
  missing_fields?: string[]
  flow_id?: number
  flow_state?: BaofuSettlementAccountStatus
  verify_fee_amount?: number
  payment_order_id?: number
  amount?: number
  business_type?: 'baofu_account_verify_fee' | string
  out_trade_no?: string
  pay_params?: MiniProgramPayParams
  expires_at?: string
  payment?: BaofuSettlementAccountPayment
  bank_card_last4?: string
  bank_account_no_mask?: string
  bank_mobile_mask?: string
  contact_mobile_mask?: string
  email_mask?: string
  wechat_sub_mch_id_mask?: string
  profile_defaults?: BaofuSettlementAccountProfileDefaults
  submitted_at?: string
  updated_at?: string
}

export interface SubmitBaofuSettlementAccountRequest {
  profile: BaofuAccountProfile
}

export interface BaofuSettlementAccountView {
  normalizedStatus: string
  statusText: string
  statusDesc: string
  nextActionText: string
  tagTheme: StatusTagTheme
  verifyFeeAmount: number
  verifyFeeDisplay: string
  canSubmitProfile: boolean
  canStartPayment: boolean
  canRefresh: boolean
  isReady: boolean
  isFailed: boolean
  isProcessing: boolean
}

function normalizeBaofuSettlementAccountStatus(status?: string): string {
  const normalized = String(status || '').trim().toLowerCase()
  return normalized || 'unknown'
}

function baofuAccountEndpoint(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '/v1/merchant/settlement-account'
    case 'operator':
      return '/v1/operators/me/settlement-account'
    case 'platform':
      return '/v1/platform/finance/settlement-account'
    default:
      return '/v1/rider/settlement-account'
  }
}

export function getBaofuSettlementAccount(role: BaofuAccountOwnerRole): Promise<BaofuSettlementAccountResponse> {
  return request({
    url: baofuAccountEndpoint(role),
    method: 'GET'
  })
}

export function submitBaofuSettlementAccountProfile(
  role: BaofuAccountOwnerRole,
  payload: SubmitBaofuSettlementAccountRequest
): Promise<BaofuSettlementAccountResponse> {
  return request({
    url: baofuAccountEndpoint(role),
    method: 'POST',
    data: payload
  })
}

export const getRiderBaofuSettlementAccount = () => getBaofuSettlementAccount('rider')

export const submitRiderBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('rider', { profile })

export const getMerchantBaofuSettlementAccount = () => getBaofuSettlementAccount('merchant')

export const submitMerchantBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('merchant', { profile })

export const getOperatorBaofuSettlementAccount = () => getBaofuSettlementAccount('operator')

export const submitOperatorBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('operator', { profile })

export const getPlatformBaofuSettlementAccount = () => getBaofuSettlementAccount('platform')

export const submitPlatformBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('platform', { profile })

export function getBaofuAccountStatusText(status?: string): string {
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return '已开通'
    case 'profile_pending':
      return '待补充资料'
    case 'verify_fee_pending':
      return '待支付核验费'
    case 'verify_fee_processing':
      return '支付确认中'
    case 'opening_processing':
      return '开户处理中'
    case 'merchant_report_processing':
      return '商户报备中'
    case 'applet_auth_pending':
      return '授权目录绑定中'
    case 'failed':
      return '开通失败'
    case 'voided':
      return '已作废'
    default:
      return '同步中'
  }
}

export function getBaofuAccountStatusTheme(status?: string): StatusTagTheme {
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return 'success'
    case 'failed':
    case 'voided':
      return 'danger'
    case 'profile_pending':
    case 'verify_fee_pending':
    case 'verify_fee_processing':
      return 'warning'
    case 'opening_processing':
    case 'merchant_report_processing':
    case 'applet_auth_pending':
      return 'primary'
    default:
      return 'default'
  }
}

export function getBaofuAccountNextActionText(
  status?: string,
  verifyFeeAmount = 200
): string {
  const feeDisplay = (verifyFeeAmount / 100).toFixed(2)
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return '结算账户已可用'
    case 'profile_pending':
      return '请补全开户资料后提交'
    case 'verify_fee_pending':
      return `支付 ${feeDisplay} 元核验费后继续开户`
    case 'verify_fee_processing':
      return '核验费支付结果确认中，请稍后刷新'
    case 'opening_processing':
      return '宝付开户处理中，请稍后刷新'
    case 'merchant_report_processing':
      return '正在配置微信支付商户展示名称，请稍后刷新'
    case 'applet_auth_pending':
      return '正在绑定微信支付授权目录，请稍后刷新'
    case 'failed':
      return '开户未通过，请核对资料后重试；如持续失败请联系平台处理'
    case 'voided':
      return '开户流程已作废，请联系平台处理'
    default:
      return '开户状态同步中，请稍后刷新'
  }
}

export function isBaofuSettlementTerminalStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'ready' ||
    normalized === 'failed' ||
    normalized === 'profile_pending' ||
    normalized === 'voided'
}

export function isBaofuSettlementAfterPaymentTerminalStatus(status?: string): boolean {
  return isBaofuSettlementTerminalStatus(status)
}

export function isBaofuSettlementPaymentRequiredStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'verify_fee_pending'
}

export function isBaofuSettlementOpeningProcessingStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'opening_processing' ||
    normalized === 'verify_fee_processing' ||
    normalized === 'merchant_report_processing' ||
    normalized === 'applet_auth_pending'
}

export function getBaofuAccountPayment(response?: BaofuSettlementAccountResponse | null): BaofuSettlementAccountPayment | null {
  if (!response) {
    return null
  }

  const nested = response.payment
  const paymentOrderId = nested?.payment_order_id || response.payment_order_id || 0
  const payParams = nested?.pay_params || response.pay_params

  if (!paymentOrderId && !payParams) {
    return null
  }

  return {
    payment_order_id: paymentOrderId,
    amount: nested?.amount || response.amount || response.verify_fee_amount || 200,
    business_type: nested?.business_type || response.business_type || 'baofu_account_verify_fee',
    out_trade_no: nested?.out_trade_no || response.out_trade_no,
    pay_params: payParams,
    expires_at: nested?.expires_at || response.expires_at
  }
}

export function buildBaofuSettlementAccountView(
  response?: BaofuSettlementAccountResponse | null
): BaofuSettlementAccountView {
  const normalizedStatus = normalizeBaofuSettlementAccountStatus(response?.status || response?.state)
  const verifyFeeAmount = Number(response?.verify_fee_amount || response?.payment?.amount || response?.amount || 200)
  const nextActionText = getBaofuAccountNextActionText(normalizedStatus, verifyFeeAmount)
  const statusDesc = String(response?.status_desc || nextActionText).trim()

  return {
    normalizedStatus,
    statusText: response?.label || getBaofuAccountStatusText(normalizedStatus),
    statusDesc,
    nextActionText,
    tagTheme: getBaofuAccountStatusTheme(normalizedStatus),
    verifyFeeAmount,
    verifyFeeDisplay: (verifyFeeAmount / 100).toFixed(2),
    canSubmitProfile: normalizedStatus === 'profile_pending' || normalizedStatus === 'failed',
    canStartPayment: isBaofuSettlementPaymentRequiredStatus(normalizedStatus),
    canRefresh: normalizedStatus !== 'ready' && normalizedStatus !== 'voided',
    isReady: normalizedStatus === 'ready',
    isFailed: normalizedStatus === 'failed',
    isProcessing: isBaofuSettlementOpeningProcessingStatus(normalizedStatus)
  }
}
