import { request } from '../utils/request'
import type { ApplymentContactDocType, ApplymentContactType } from './applyment-bank'
import type { ApplymentAccountValidationResponse } from './merchant-finance'

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
  contact_type?: ApplymentContactType
  contact_name?: string
  contact_id_doc_type?: ApplymentContactDocType
  contact_id_card_number?: string
  contact_id_doc_copy_asset_id?: number
  contact_id_doc_copy_back_asset_id?: number
  contact_id_doc_period_begin?: string
  contact_id_doc_period_end?: string
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
  sign_state?: string
  legal_validation_url?: string
  account_validation?: ApplymentAccountValidationResponse
  reject_reason?: string
  created_at: string
  updated_at: string
}

export type OperatorApplymentNormalizedStatus =
  | 'pending'
  | 'ready'
  | 'in_review'
  | 'needs_sign'
  | 'opened'
  | 'rejected'
  | 'frozen'
  | 'cancelled'
  | 'unknown'

export type OperatorApplymentGuideTheme = 'primary' | 'warning'

export interface OperatorApplymentAccountValidationView {
  accountName: string
  accountNo: string
  payAmount: number
  payAmountText: string
  destinationAccountNumber: string
  destinationAccountName: string
  destinationAccountBank: string
  city: string
  remark: string
  deadline: string
}

export interface OperatorApplymentStatusView {
  statusCode: string
  normalizedStatus: OperatorApplymentNormalizedStatus
  statusDesc: string
  tagText: string
  tagTheme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
  applymentId: string
  subMchId: string
  rejectReason: string
  blockReason: string
  signURL: string
  signStateText: string
  legalValidationURL: string
  hasExistingApplyment: boolean
  isOpened: boolean
  canSubmitOpenInfo: boolean
  isInReview: boolean
  needsSign: boolean
  needsAccountValidation: boolean
  needsConfirmation: boolean
  needsLegalValidation: boolean
  hasPendingActions: boolean
  showRejectReason: boolean
  guideTheme: OperatorApplymentGuideTheme
  guideText: string
  accountValidation: OperatorApplymentAccountValidationView | null
}

export const DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW: OperatorApplymentStatusView = {
  statusCode: 'pending',
  normalizedStatus: 'pending',
  statusDesc: '未提交',
  tagText: '尚未提交进件资料',
  tagTheme: 'warning',
  applymentId: '-',
  subMchId: '-',
  rejectReason: '-',
  blockReason: '',
  signURL: '',
  signStateText: '',
  legalValidationURL: '',
  hasExistingApplyment: false,
  isOpened: false,
  canSubmitOpenInfo: true,
  isInReview: false,
  needsSign: false,
  needsAccountValidation: false,
  needsConfirmation: false,
  needsLegalValidation: false,
  hasPendingActions: false,
  showRejectReason: false,
  guideTheme: 'primary',
  guideText: '当前尚未开通微信支付商户，请提交必要信息完成开户。',
  accountValidation: null
}

const APPLYMENT_IN_REVIEW_STATUSES = new Set([
  'bindbank_submitted',
  'submitted',
  'checking',
  'auditing',
  'account_need_verify',
  'to_be_confirmed',
  'to_be_signed',
  'signing',
  'need_sign'
])

const APPLYMENT_NEEDS_SIGN_STATUSES = new Set(['to_be_signed', 'signing', 'need_sign'])
const APPLYMENT_OPENED_STATUSES = new Set(['finish'])
const APPLYMENT_REJECTED_STATUSES = new Set(['rejected', 'rejected_sign'])
const APPLYMENT_FROZEN_STATUSES = new Set(['frozen'])
const APPLYMENT_CANCELLED_STATUSES = new Set(['canceled', 'cancelled'])

export function normalizeOperatorApplymentStatus(status?: string): OperatorApplymentNormalizedStatus {
  const normalized = String(status || '').trim().toLowerCase()

  if (!normalized || normalized === 'pending' || normalized === 'not_applied') {
    return 'pending'
  }

  if (normalized === 'active') {
    return 'ready'
  }

  if (APPLYMENT_IN_REVIEW_STATUSES.has(normalized)) {
    return 'in_review'
  }

  if (APPLYMENT_NEEDS_SIGN_STATUSES.has(normalized)) {
    return 'needs_sign'
  }

  if (APPLYMENT_OPENED_STATUSES.has(normalized)) {
    return 'opened'
  }

  if (APPLYMENT_REJECTED_STATUSES.has(normalized)) {
    return 'rejected'
  }

  if (APPLYMENT_FROZEN_STATUSES.has(normalized)) {
    return 'frozen'
  }

  if (APPLYMENT_CANCELLED_STATUSES.has(normalized)) {
    return 'cancelled'
  }

  return 'unknown'
}

function getDefaultOperatorApplymentStatusDesc(statusCode: string): string {
  const statusDescMap: Record<string, string> = {
    pending: '未提交开户信息',
    active: '可提交开户信息',
    bindbank_submitted: '开户信息已提交',
    submitted: '微信审核中',
    auditing: '微信审核中',
    checking: '资料校验中',
    account_need_verify: '账户验证中',
    to_be_confirmed: '待确认',
    to_be_signed: '待签约确认',
    signing: '签约处理中',
    need_sign: '待签约确认',
    finish: '开户完成',
    frozen: '已冻结',
    rejected: '开户被拒绝',
    rejected_sign: '签约失败',
    canceled: '开户已取消',
    cancelled: '开户已取消'
  }
  return statusDescMap[statusCode] || statusCode || '未提交'
}

function getOperatorSignStateText(signState?: string) {
  switch (String(signState || '').trim().toUpperCase()) {
    case 'UNSIGNED':
      return '未签约'
    case 'SIGNED':
      return '已签约'
    case 'NOT_SIGNABLE':
      return '当前不可签约'
    default:
      return ''
  }
}

function buildOperatorApplymentAccountValidationView(
  validation?: ApplymentAccountValidationResponse | null
): OperatorApplymentAccountValidationView | null {
  if (!validation) {
    return null
  }

  const payAmount = typeof validation.pay_amount === 'number' ? validation.pay_amount : 0
  return {
    accountName: validation.account_name || '-',
    accountNo: validation.account_no || '-',
    payAmount,
    payAmountText: payAmount > 0 ? `${(payAmount / 100).toFixed(2)}元` : '-',
    destinationAccountNumber: validation.destination_account_number || '-',
    destinationAccountName: validation.destination_account_name || '-',
    destinationAccountBank: validation.destination_account_bank || '-',
    city: validation.city || '-',
    remark: validation.remark || '-',
    deadline: validation.deadline || '-'
  }
}

function getDefaultCanSubmitOpenInfo(
  normalizedStatus: OperatorApplymentNormalizedStatus,
  rawStatusCode: string
): boolean {
  return normalizedStatus === 'pending'
    || normalizedStatus === 'ready'
    || normalizedStatus === 'rejected'
    || normalizedStatus === 'cancelled'
    || rawStatusCode === 'active'
}

export function buildOperatorApplymentStatusView(
  status: OperatorApplymentStatusResponse | null
): OperatorApplymentStatusView {
  if (!status) {
    return { ...DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW }
  }

  const statusCode = String(status.status || 'pending').trim().toLowerCase() || 'pending'
  const normalizedStatus = normalizeOperatorApplymentStatus(statusCode)
  const statusDesc = status.status_desc || getDefaultOperatorApplymentStatusDesc(statusCode)
  const tagText = getDefaultOperatorApplymentStatusDesc(statusCode)
  const rejectReason = status.reject_reason || '-'
  const blockReason = status.block_reason || ''
  const signURL = status.sign_url || ''
  const signStateText = getOperatorSignStateText(status.sign_state)
  const accountValidation = buildOperatorApplymentAccountValidationView(status.account_validation)
  const legalValidationURL = status.legal_validation_url || ''
  const isOpened = normalizedStatus === 'opened'
  const needsSign = String(status.sign_state || '').trim().toUpperCase() === 'UNSIGNED' || APPLYMENT_NEEDS_SIGN_STATUSES.has(statusCode)
  const needsAccountValidation = statusCode === 'account_need_verify' || Boolean(accountValidation)
  const needsConfirmation = statusCode === 'to_be_confirmed'
  const needsLegalValidation = Boolean(legalValidationURL)
  const hasPendingActions = needsAccountValidation || needsConfirmation || needsSign
  const isInReview = normalizedStatus === 'in_review' && !hasPendingActions
  const canSubmitOpenInfo = typeof status.can_submit === 'boolean'
    ? status.can_submit
    : getDefaultCanSubmitOpenInfo(normalizedStatus, statusCode)

  let guideText = DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW.guideText
  let tagTheme: OperatorApplymentStatusView['tagTheme'] = 'warning'
  if (normalizedStatus === 'rejected') {
    guideText = '开户被拒，请根据拒绝原因修改信息后重新提交。'
    tagTheme = 'danger'
  } else if (needsAccountValidation && needsSign) {
    guideText = needsLegalValidation
      ? '当前申请同时存在账户验证和签约待处理，可优先使用法人扫码验证并完成签约。'
      : '当前申请同时存在账户验证和签约待处理，请按页面指引逐项完成。'
    tagTheme = 'primary'
  } else if (needsAccountValidation) {
    guideText = needsLegalValidation
      ? '当前申请待账户验证，可优先使用法人扫码验证；若无法扫码，请按汇款指引完成验证。'
      : '当前申请待账户验证，请先按汇款指引完成验证。'
    tagTheme = 'primary'
  } else if (needsConfirmation) {
    guideText = '当前申请待确认，请先按微信支付指引完成确认。'
    tagTheme = 'primary'
  } else if (needsSign) {
    guideText = '微信支付已进入签约阶段，请尽快完成签约确认。'
    tagTheme = 'primary'
  } else if (normalizedStatus === 'in_review') {
    guideText = '微信支付正在审核开户信息，审核期间无需重复提交。'
    tagTheme = 'warning'
  } else if (normalizedStatus === 'frozen') {
    guideText = blockReason || statusDesc || '当前账号状态不可用，暂不支持提交微信支付开户。'
    tagTheme = 'danger'
  } else if (normalizedStatus === 'cancelled') {
    guideText = blockReason || '当前开户申请已取消，可重新提交开户信息。'
  } else if (isOpened) {
    guideText = '微信支付商户已开通，可正常经营与提现。'
    tagTheme = 'success'
  } else if (!canSubmitOpenInfo && blockReason) {
    guideText = blockReason
  }

  return {
    statusCode,
    normalizedStatus,
    statusDesc,
    tagText,
    tagTheme,
    applymentId: status.applyment_id ? String(status.applyment_id) : '-',
    subMchId: status.sub_mch_id || '-',
    rejectReason,
    blockReason,
    signURL,
    signStateText,
    legalValidationURL,
    hasExistingApplyment: normalizedStatus !== 'pending',
    isOpened,
    canSubmitOpenInfo,
    isInReview,
    needsSign,
    needsAccountValidation,
    needsConfirmation,
    needsLegalValidation,
    hasPendingActions,
    showRejectReason: canSubmitOpenInfo && rejectReason !== '-',
    guideTheme: needsSign ? 'warning' : 'primary',
    guideText,
    accountValidation
  }
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
