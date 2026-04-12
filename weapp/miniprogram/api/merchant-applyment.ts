import {
  type ApplymentStatusResponse,
  type MerchantBindBankRequest,
  type MerchantBindBankResponse,
  getMerchantApplymentStatus as requestMerchantApplymentStatus,
  merchantBindBank as requestMerchantBindBank
} from './merchant-finance'

export type MerchantApplymentNormalizedStatus =
  | 'pending'
  | 'in_review'
  | 'needs_sign'
  | 'opened'
  | 'rejected'
  | 'frozen'
  | 'unknown'

export type MerchantApplymentGuideTheme = 'primary' | 'warning' | 'success' | 'danger'

export interface MerchantApplymentStatusView {
  statusCode: string
  normalizedStatus: MerchantApplymentNormalizedStatus
  statusDesc: string
  tagText: string
  tagTheme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
  blockReason: string
  signURL: string
  subMchId: string
  rejectReason: string
  canSubmitOpenInfo: boolean
  isOpened: boolean
  isInReview: boolean
  needsSign: boolean
  showRejectReason: boolean
  hasApplyment: boolean
  guideTheme: MerchantApplymentGuideTheme
  guideText: string
  submitActionLabel: string
}

export const DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW: MerchantApplymentStatusView = {
  statusCode: 'not_applied',
  normalizedStatus: 'pending',
  statusDesc: '尚未提交开户申请',
  tagText: '未提交',
  tagTheme: 'warning',
  blockReason: '',
  signURL: '',
  subMchId: '-',
  rejectReason: '-',
  canSubmitOpenInfo: true,
  isOpened: false,
  isInReview: false,
  needsSign: false,
  showRejectReason: false,
  hasApplyment: false,
  guideTheme: 'primary',
  guideText: '主体审核通过后，可先完善菜品、桌台、套餐和门店配置；准备收款前再填写结算账户并提交收付通进件。',
  submitActionLabel: '填写进件资料'
}

const APPLYMENT_IN_REVIEW_STATUSES = new Set([
  'bindbank_submitted',
  'submitted',
  'auditing',
  'checking',
  'account_need_verify'
])

const APPLYMENT_NEEDS_SIGN_STATUSES = new Set(['to_be_signed', 'signing', 'need_sign'])
const APPLYMENT_OPENED_STATUSES = new Set(['finish', 'active'])
const APPLYMENT_REJECTED_STATUSES = new Set(['rejected', 'rejected_sign'])
const APPLYMENT_FROZEN_STATUSES = new Set(['frozen'])

export function normalizeMerchantApplymentStatus(status?: string): MerchantApplymentNormalizedStatus {
  const normalized = String(status || '').trim().toLowerCase()

  if (!normalized || normalized === 'not_applied' || normalized === 'pending') {
    return 'pending'
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

  return 'unknown'
}

function getDefaultMerchantApplymentStatusDesc(statusCode: string): string {
  const statusDescMap: Record<string, string> = {
    not_applied: '尚未提交开户申请',
    pending: '待提交',
    submitted: '已提交，等待审核',
    bindbank_submitted: '已提交，等待审核',
    auditing: '审核中',
    checking: '审核中',
    account_need_verify: '审核中',
    to_be_signed: '待签约，请点击签约链接完成签约',
    signing: '签约中',
    need_sign: '待签约，请点击签约链接完成签约',
    finish: '开户成功',
    active: '账户已开通',
    rejected: '审核被拒绝',
    rejected_sign: '签约失败',
    frozen: '已冻结'
  }

  return statusDescMap[statusCode] || '状态更新中'
}

function getDefaultMerchantApplymentTagText(statusCode: string): string {
  const tagTextMap: Record<string, string> = {
    not_applied: '未提交',
    pending: '待提交',
    submitted: '审核中',
    bindbank_submitted: '审核中',
    auditing: '审核中',
    checking: '审核中',
    account_need_verify: '审核中',
    to_be_signed: '待签约',
    signing: '签约中',
    need_sign: '待签约',
    finish: '已开通',
    active: '已开通',
    rejected: '已拒绝',
    rejected_sign: '签约失败',
    frozen: '已冻结'
  }

  return tagTextMap[statusCode] || '状态更新中'
}

function getDefaultMerchantApplymentBlockReason(
  normalizedStatus: MerchantApplymentNormalizedStatus,
  statusDesc: string
) {
  switch (normalizedStatus) {
    case 'in_review':
      return '当前资料正在审核中，暂不支持重复提交。'
    case 'needs_sign':
      return '当前已进入微信签约环节，请先完成签约。'
    case 'opened':
      return '当前账户已开通，无需重复提交进件资料。'
    case 'frozen':
      return statusDesc || '当前进件状态不可用，暂不支持提交收付通进件。'
    case 'unknown':
      return '当前状态暂不支持提交收付通进件。'
    default:
      return ''
  }
}

function getDefaultMerchantSubmitActionLabel(normalizedStatus: MerchantApplymentNormalizedStatus) {
  if (normalizedStatus === 'rejected') {
    return '重新提交资料'
  }
  return '填写进件资料'
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
  const canSubmitOpenInfo = typeof status.can_submit === 'boolean'
    ? status.can_submit
    : normalizedStatus === 'pending' || normalizedStatus === 'rejected'
  const rejectReason = status.reject_reason || '-'
  const blockReason = status.block_reason || getDefaultMerchantApplymentBlockReason(normalizedStatus, statusDesc)
  const signURL = status.sign_url || ''
  const subMchId = status.sub_mch_id || '-'
  const isOpened = normalizedStatus === 'opened'
  const needsSign = normalizedStatus === 'needs_sign'
  const isInReview = normalizedStatus === 'in_review' || needsSign
  const showRejectReason = normalizedStatus === 'rejected' && rejectReason !== '-'
  const hasApplyment = normalizedStatus !== 'pending'

  let guideText = DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW.guideText
  let guideTheme: MerchantApplymentGuideTheme = 'primary'
  let tagTheme: MerchantApplymentStatusView['tagTheme'] = 'warning'

  if (normalizedStatus === 'rejected') {
    guideText = showRejectReason
      ? '进件资料已被驳回，请根据拒绝原因修改后重新提交。'
      : '进件资料需要重新提交，请核对信息后再试。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (normalizedStatus === 'in_review') {
    guideText = '微信支付正在审核进件资料，审核期间无需重复提交。'
    guideTheme = 'warning'
    tagTheme = 'warning'
  } else if (needsSign) {
    guideText = '当前已进入微信签约环节，请先完成签约，再回到本页刷新状态。'
    guideTheme = 'warning'
    tagTheme = 'primary'
  } else if (isOpened) {
    guideText = '收付通已开通，可正常收款、结算与提现。'
    guideTheme = 'success'
    tagTheme = 'success'
  } else if (normalizedStatus === 'frozen') {
    guideText = blockReason || '当前进件状态不可用，暂不支持提交收付通进件。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (!canSubmitOpenInfo && blockReason) {
    guideText = blockReason
    guideTheme = 'warning'
    tagTheme = 'default'
  }

  return {
    statusCode,
    normalizedStatus,
    statusDesc,
    tagText: getDefaultMerchantApplymentTagText(statusCode),
    tagTheme,
    blockReason,
    signURL,
    subMchId,
    rejectReason,
    canSubmitOpenInfo,
    isOpened,
    isInReview,
    needsSign,
    showRejectReason,
    hasApplyment,
    guideTheme,
    guideText,
    submitActionLabel: getDefaultMerchantSubmitActionLabel(normalizedStatus)
  }
}

export const getMerchantApplymentStatus = async () => requestMerchantApplymentStatus()

export const merchantBindBank = async (payload: MerchantBindBankRequest): Promise<MerchantBindBankResponse> => {
  return requestMerchantBindBank(payload)
}

export type { ApplymentStatusResponse, MerchantBindBankRequest, MerchantBindBankResponse }