import {
  type ApplymentAccountValidationResponse,
  type ApplymentStatusResponse,
  type MerchantBindBankRequest,
  type MerchantBindBankResponse,
  getMerchantApplymentStatus as requestMerchantApplymentStatus,
  merchantBindBank as requestMerchantBindBank
} from './merchant-finance'

export type MerchantApplymentNormalizedStatus =
  | 'pending'
  | 'in_review'
  | 'opened'
  | 'rejected'
  | 'frozen'
  | 'cancelled'
  | 'unknown'

export type MerchantApplymentGuideTheme = 'primary' | 'warning' | 'success' | 'danger'
export type MerchantApplymentSignState = 'unsigned' | 'signed' | 'not_signable' | 'unknown'

export interface MerchantApplymentAccountValidationView {
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

export interface MerchantApplymentStatusView {
  statusCode: string
  normalizedStatus: MerchantApplymentNormalizedStatus
  statusDesc: string
  tagText: string
  tagTheme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
  blockReason: string
  signURL: string
  signState: MerchantApplymentSignState
  signStateText: string
  legalValidationURL: string
  subMchId: string
  rejectReason: string
  canSubmitOpenInfo: boolean
  isOpened: boolean
  isInReview: boolean
  needsSign: boolean
  needsAccountValidation: boolean
  needsConfirmation: boolean
  needsLegalValidation: boolean
  hasPendingActions: boolean
  showRejectReason: boolean
  hasApplyment: boolean
  guideTheme: MerchantApplymentGuideTheme
  guideText: string
  submitActionLabel: string
  accountValidation: MerchantApplymentAccountValidationView | null
}

export const DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW: MerchantApplymentStatusView = {
  statusCode: 'not_applied',
  normalizedStatus: 'pending',
  statusDesc: '尚未提交开户申请',
  tagText: '未提交',
  tagTheme: 'warning',
  blockReason: '',
  signURL: '',
  signState: 'unknown',
  signStateText: '',
  legalValidationURL: '',
  subMchId: '-',
  rejectReason: '-',
  canSubmitOpenInfo: true,
  isOpened: false,
  isInReview: false,
  needsSign: false,
  needsAccountValidation: false,
  needsConfirmation: false,
  needsLegalValidation: false,
  hasPendingActions: false,
  showRejectReason: false,
  hasApplyment: false,
  guideTheme: 'primary',
  guideText: '主体审核通过后，可先完善菜品、桌台、套餐和门店配置；准备收款前再填写结算账户并提交收付通进件。',
  submitActionLabel: '填写进件资料',
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
const APPLYMENT_OPENED_STATUSES = new Set(['finish', 'active'])
const APPLYMENT_REJECTED_STATUSES = new Set(['rejected', 'rejected_sign'])
const APPLYMENT_FROZEN_STATUSES = new Set(['frozen'])
const APPLYMENT_CANCELLED_STATUSES = new Set(['canceled', 'cancelled'])

export function normalizeMerchantApplymentStatus(status?: string): MerchantApplymentNormalizedStatus {
  const normalized = String(status || '').trim().toLowerCase()

  if (!normalized || normalized === 'not_applied' || normalized === 'pending') {
    return 'pending'
  }

  if (APPLYMENT_IN_REVIEW_STATUSES.has(normalized)) {
    return 'in_review'
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

export function normalizeMerchantApplymentSignState(signState?: string): MerchantApplymentSignState {
  const normalized = String(signState || '').trim().toUpperCase()
  switch (normalized) {
    case 'UNSIGNED':
      return 'unsigned'
    case 'SIGNED':
      return 'signed'
    case 'NOT_SIGNABLE':
      return 'not_signable'
    default:
      return 'unknown'
  }
}

function getDefaultMerchantApplymentStatusDesc(statusCode: string): string {
  const statusDescMap: Record<string, string> = {
    not_applied: '尚未提交开户申请',
    pending: '待提交',
    submitted: '已提交，等待审核',
    bindbank_submitted: '已提交，等待审核',
    checking: '资料校验中',
    auditing: '审核中',
    account_need_verify: '待账户验证',
    to_be_confirmed: '待确认',
    to_be_signed: '待签约，请点击签约链接完成签约',
    signing: '签约中',
    need_sign: '待签约，请点击签约链接完成签约',
    finish: '开户成功',
    active: '账户已开通',
    rejected: '审核被拒绝',
    rejected_sign: '签约失败',
    frozen: '已冻结',
    canceled: '已作废',
    cancelled: '已作废'
  }

  return statusDescMap[statusCode] || '状态更新中'
}

function getDefaultMerchantApplymentTagText(statusCode: string): string {
  const tagTextMap: Record<string, string> = {
    not_applied: '未提交',
    pending: '待提交',
    submitted: '审核中',
    bindbank_submitted: '审核中',
    checking: '校验中',
    auditing: '审核中',
    account_need_verify: '待验证',
    to_be_confirmed: '待确认',
    to_be_signed: '待签约',
    signing: '签约中',
    need_sign: '待签约',
    finish: '已开通',
    active: '已开通',
    rejected: '已拒绝',
    rejected_sign: '签约失败',
    frozen: '已冻结',
    canceled: '已作废',
    cancelled: '已作废'
  }

  return tagTextMap[statusCode] || '状态更新中'
}

function getMerchantApplymentSignStateText(signState: MerchantApplymentSignState): string {
  switch (signState) {
    case 'unsigned':
      return '未签约'
    case 'signed':
      return '已签约'
    case 'not_signable':
      return '当前不可签约'
    default:
      return ''
  }
}

function getDefaultMerchantApplymentBlockReason(
  normalizedStatus: MerchantApplymentNormalizedStatus,
  statusDesc: string,
  statusCode: string,
  options: {
    needsAccountValidation: boolean
    needsConfirmation: boolean
    needsSign: boolean
  }
) {
  if (options.needsAccountValidation) {
    return '当前申请待账户验证，请先完成验证后再刷新状态。'
  }
  if (options.needsConfirmation || statusCode === 'to_be_confirmed') {
    return '当前申请待确认，请先完成确认后再刷新状态。'
  }
  if (options.needsSign) {
    return '当前申请存在待签约事项，请先完成签约。'
  }
  switch (normalizedStatus) {
    case 'in_review':
      return '当前资料正在审核中，暂不支持重复提交。'
    case 'opened':
      return '当前账户已开通，无需重复提交进件资料。'
    case 'frozen':
      return statusDesc || '当前进件状态不可用，暂不支持提交收付通进件。'
    case 'cancelled':
      return '当前申请已作废，可重新提交资料。'
    case 'unknown':
      return '当前状态暂不支持提交收付通进件。'
    default:
      return ''
  }
}

function getDefaultMerchantSubmitActionLabel(normalizedStatus: MerchantApplymentNormalizedStatus) {
  if (normalizedStatus === 'rejected' || normalizedStatus === 'cancelled') {
    return '重新提交资料'
  }
  return '填写进件资料'
}

function buildMerchantApplymentAccountValidationView(
  validation?: ApplymentAccountValidationResponse | null
): MerchantApplymentAccountValidationView | null {
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

export function buildMerchantApplymentStatusView(
  status: ApplymentStatusResponse | null
): MerchantApplymentStatusView {
  if (!status) {
    return { ...DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW }
  }

  const statusCode = String(status.status || 'not_applied').trim().toLowerCase() || 'not_applied'
  const normalizedStatus = normalizeMerchantApplymentStatus(statusCode)
  const statusDesc = status.status_desc || getDefaultMerchantApplymentStatusDesc(statusCode)
  const signState = normalizeMerchantApplymentSignState(status.sign_state)
  const signStateText = getMerchantApplymentSignStateText(signState)
  const accountValidation = buildMerchantApplymentAccountValidationView(status.account_validation)
  const legalValidationURL = status.legal_validation_url || ''
  const needsAccountValidation = statusCode === 'account_need_verify' || Boolean(accountValidation)
  const needsConfirmation = statusCode === 'to_be_confirmed'
  const needsLegalValidation = Boolean(legalValidationURL)
  const needsSign = signState === 'unsigned' || APPLYMENT_NEEDS_SIGN_STATUSES.has(statusCode)
  const hasPendingActions = needsAccountValidation || needsConfirmation || needsSign
  const canSubmitOpenInfo = typeof status.can_submit === 'boolean'
    ? status.can_submit
    : normalizedStatus === 'pending' || normalizedStatus === 'rejected' || normalizedStatus === 'cancelled'
  const rejectReason = status.reject_reason || '-'
  const blockReason = status.block_reason || getDefaultMerchantApplymentBlockReason(normalizedStatus, statusDesc, statusCode, {
    needsAccountValidation,
    needsConfirmation,
    needsSign
  })
  const signURL = status.sign_url || ''
  const subMchId = status.sub_mch_id || '-'
  const isOpened = normalizedStatus === 'opened'
  const isInReview = normalizedStatus === 'in_review' && !hasPendingActions
  const showRejectReason = normalizedStatus === 'rejected' && rejectReason !== '-'
  const hasApplyment = normalizedStatus !== 'pending'

  let guideText = DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW.guideText
  let guideTheme: MerchantApplymentGuideTheme = 'primary'
  let tagTheme: MerchantApplymentStatusView['tagTheme'] = 'warning'
  let tagText = getDefaultMerchantApplymentTagText(statusCode)

  if (normalizedStatus === 'rejected') {
    guideText = showRejectReason
      ? '进件资料已被驳回，请根据拒绝原因修改后重新提交。'
      : '进件资料需要重新提交，请核对信息后再试。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (needsAccountValidation && needsSign) {
    guideText = needsLegalValidation
      ? '当前申请同时存在账户验证和签约待处理，可优先使用法人扫码验证，并完成签约后再刷新状态。'
      : '当前申请同时存在账户验证和签约待处理，请按页面指引逐项完成后再刷新状态。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待处理'
  } else if (needsAccountValidation) {
    guideText = needsLegalValidation
      ? '当前申请待账户验证，可优先使用法人扫码验证；若无法扫码，请按汇款指引完成验证。'
      : '当前申请待账户验证，请按汇款指引完成验证后再刷新状态。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待验证'
  } else if (needsConfirmation && needsSign) {
    guideText = '当前申请待确认且签约未完成，请先完成确认和签约，再回到本页刷新状态。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待处理'
  } else if (needsConfirmation) {
    guideText = '当前申请待确认，请先按微信支付指引完成确认。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待确认'
  } else if (needsSign) {
    guideText = '当前申请存在待签约事项，请先完成签约，再回到本页刷新状态。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待签约'
  } else if (normalizedStatus === 'in_review') {
    guideText = '微信支付正在审核进件资料，审核期间无需重复提交。'
    guideTheme = 'warning'
    tagTheme = 'warning'
  } else if (isOpened) {
    guideText = '收付通已开通，可正常收款、结算与提现。'
    guideTheme = 'success'
    tagTheme = 'success'
  } else if (normalizedStatus === 'frozen') {
    guideText = blockReason || '当前进件状态不可用，暂不支持提交收付通进件。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (normalizedStatus === 'cancelled') {
    guideText = blockReason || '当前申请已作废，可重新提交进件资料。'
    guideTheme = 'warning'
    tagTheme = 'default'
  } else if (!canSubmitOpenInfo && blockReason) {
    guideText = blockReason
    guideTheme = 'warning'
    tagTheme = 'default'
  }

  return {
    statusCode,
    normalizedStatus,
    statusDesc,
    tagText,
    tagTheme,
    blockReason,
    signURL,
    signState,
    signStateText,
    legalValidationURL,
    subMchId,
    rejectReason,
    canSubmitOpenInfo,
    isOpened,
    isInReview,
    needsSign,
    needsAccountValidation,
    needsConfirmation,
    needsLegalValidation,
    hasPendingActions,
    showRejectReason,
    hasApplyment,
    guideTheme,
    guideText,
    submitActionLabel: getDefaultMerchantSubmitActionLabel(normalizedStatus),
    accountValidation
  }
}

export const getMerchantApplymentStatus = async () => requestMerchantApplymentStatus()

export const merchantBindBank = async (payload: MerchantBindBankRequest): Promise<MerchantBindBankResponse> => {
  return requestMerchantBindBank(payload)
}

export type { ApplymentStatusResponse, MerchantBindBankRequest, MerchantBindBankResponse }