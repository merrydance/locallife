import { request } from '../utils/request'
import type { ApplymentContactDocType, ApplymentContactType } from './applyment-bank'
import type { StatusTagTheme } from '../utils/status-tag'

export interface ApplymentAccountValidationResponse {
  account_name?: string
  account_no?: string
  pay_amount?: number
  destination_account_number?: string
  destination_account_name?: string
  destination_account_bank?: string
  city?: string
  remark?: string
  deadline?: string
}

export interface ApplymentStatusResponse {
  status: string
  status_desc: string
  can_submit?: boolean
  block_reason?: string
  account_authorize_state?: string
  action_hint?: string
  sign_url?: string
  sign_state?: string
  legal_validation_url?: string
  account_validation?: ApplymentAccountValidationResponse
  sub_mch_id?: string
  reject_reason?: string
}

export interface MerchantBindBankRequest {
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
  contact_email: string
  contact_type?: ApplymentContactType
  contact_name?: string
  contact_id_doc_type?: ApplymentContactDocType
  contact_id_card_number?: string
  contact_id_doc_copy_asset_id?: number
  contact_id_doc_copy_back_asset_id?: number
  contact_id_doc_period_begin?: string
  contact_id_doc_period_end?: string
}

export interface MerchantBindBankResponse {
  applyment_id: number
  status: string
  status_desc?: string
  message: string
  sign_url?: string
  sign_state?: string
  legal_validation_url?: string
  account_validation?: ApplymentAccountValidationResponse
  sub_mch_id?: string
  reject_reason?: string
}

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
export type MerchantApplymentFlowStepState = 'done' | 'current' | 'pending'

export interface MerchantApplymentFlowStepView {
  key: string
  title: string
  description: string
  state: MerchantApplymentFlowStepState
  stateText: string
}

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
  headline: string
  summaryText: string
  flowCurrent: number
  statusDesc: string
  tagText: string
  tagTheme: StatusTagTheme
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
  needsLegalValidation: boolean
  hasPendingActions: boolean
  actionHint: string
  showRejectReason: boolean
  hasApplyment: boolean
  guideTheme: MerchantApplymentGuideTheme
  guideTitle: string
  guideText: string
  guideDescription: string
  primaryActionText: string
  submitActionLabel: string
  flowSteps: MerchantApplymentFlowStepView[]
  accountValidation: MerchantApplymentAccountValidationView | null
}

export const DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW: MerchantApplymentStatusView = {
  statusCode: 'not_applied',
  normalizedStatus: 'pending',
  headline: '先开通微信支付',
  summaryText: '填写结算账户资料后提交开户申请。',
  flowCurrent: 0,
  statusDesc: '尚未提交开户申请',
  tagText: '未提交',
  tagTheme: 'default',
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
  needsLegalValidation: false,
  hasPendingActions: false,
  actionHint: '',
  showRejectReason: false,
  hasApplyment: false,
  guideTheme: 'primary',
  guideTitle: '先开通微信支付',
  guideText: '填写结算账户资料后提交开户申请。',
  guideDescription: '填写结算账户资料后提交开户申请。',
  primaryActionText: '填写开户资料',
  submitActionLabel: '填写开户资料',
  flowSteps: [
    {
      key: 'submit',
      title: '填写结算账户',
      description: '准备结算银行卡资料。',
      state: 'current',
      stateText: '当前步骤'
    },
    {
      key: 'verify',
      title: '微信待办',
      description: '根据微信返回结果完成待办。',
      state: 'pending',
      stateText: '待开始'
    },
    {
      key: 'review',
      title: '微信审核',
      description: '微信支付会并行校验资料、签约和账户状态。',
      state: 'pending',
      stateText: '待开始'
    },
    {
      key: 'opened',
      title: '开通收款',
      description: '开通后即可正常收款。',
      state: 'pending',
      stateText: '待开始'
    }
  ],
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

export function normalizeMerchantApplymentStatus(status?: string): MerchantApplymentNormalizedStatus {
  const normalized = String(status || '').trim().toLowerCase()

  if (!normalized || normalized === 'not_applied' || normalized === 'pending') {
    return 'pending'
  }

  if (normalized === 'active') {
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

function replaceMerchantApplymentTerm(text: string, term: string, replacement: string): string {
  return text.split(term).join(replacement)
}

function normalizeMerchantApplymentActionText(text?: string): string {
  const ordinaryServiceProvider = '普通' + '服务商'
  const ecommerceAccount = '收' + '付通'
  const applymentTerm = '进' + '件'
  const specialMerchant = '特约' + '商户'
  const subMerchantNo = '子' + '商户号'
  const guardedMerchant = '商户' + '管控'

  let normalized = String(text || '')
  normalized = replaceMerchantApplymentTerm(normalized, `微信支付${specialMerchant}账户`, '微信支付账户')
  normalized = replaceMerchantApplymentTerm(normalized, `${ordinaryServiceProvider}账户`, '微信支付账户')
  normalized = replaceMerchantApplymentTerm(normalized, ordinaryServiceProvider, '微信支付')
  normalized = replaceMerchantApplymentTerm(normalized, ecommerceAccount, '微信支付')
  normalized = replaceMerchantApplymentTerm(normalized, specialMerchant, '商户')
  normalized = replaceMerchantApplymentTerm(normalized, subMerchantNo, '微信支付账户')
  normalized = replaceMerchantApplymentTerm(normalized, guardedMerchant, '微信支付账户状态')
  normalized = replaceMerchantApplymentTerm(normalized, applymentTerm, '开户申请')
  normalized = replaceMerchantApplymentTerm(normalized, '完成后返回本页刷新状态', '完成后回到本页，系统会自动刷新')
  normalized = replaceMerchantApplymentTerm(normalized, '完成后返回本页刷新', '完成后回到本页，系统会自动刷新')
  normalized = replaceMerchantApplymentTerm(normalized, '完成后' + '刷新状态', '完成后回到本页，系统会自动刷新')
  return normalized.trim()
}

function getDefaultMerchantApplymentStatusDesc(statusCode: string): string {
  const statusDescMap: Record<string, string> = {
    not_applied: '尚未提交开户申请',
    pending: '待提交',
    submitted: '已提交，请查看微信待办进度',
    bindbank_submitted: '已提交，请查看微信待办进度',
    checking: '资料校验中',
    auditing: '审核中',
    account_need_verify: '待账户验证',
    to_be_confirmed: '待账户验证',
    to_be_signed: '待处理，请按微信提示完成',
    signing: '处理中',
    need_sign: '待处理，请按微信提示完成',
    finish: '开户成功',
    active: '待提交开户资料',
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
    submitted: '待处理',
    bindbank_submitted: '待处理',
    checking: '校验中',
    auditing: '审核中',
    account_need_verify: '待验证',
    to_be_confirmed: '待验证',
    to_be_signed: '待处理',
    signing: '处理中',
    need_sign: '待处理',
    finish: '已开通',
    active: '待提交',
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

function shouldMerchantApplymentNeedSign(
  signState: MerchantApplymentSignState,
  statusCode: string
): boolean {
  if (signState === 'unsigned') {
    return true
  }

  if (signState === 'signed' || signState === 'not_signable') {
    return false
  }

  return APPLYMENT_NEEDS_SIGN_STATUSES.has(statusCode)
}

function getDefaultMerchantApplymentBlockReason(
  normalizedStatus: MerchantApplymentNormalizedStatus,
  statusDesc: string,
  statusCode: string,
  options: {
    needsAccountValidation: boolean
    needsSign: boolean
  }
) {
  if (options.needsAccountValidation) {
    return '当前申请待账户验证，完成后回到本页会自动刷新。'
  }
  if (options.needsSign) {
    return '当前申请有微信待办，请按微信提示完成。'
  }
  switch (normalizedStatus) {
    case 'in_review':
      return '当前资料正在审核中，暂不支持重复提交。'
    case 'opened':
      return '当前账户已开通，无需重复提交开户资料。'
    case 'frozen':
      return statusDesc || '当前开户状态不可用，暂不支持提交资料。'
    case 'cancelled':
      return '当前申请已作废，可重新提交资料。'
    case 'unknown':
      return '当前状态暂不支持提交资料。'
    default:
      return ''
  }
}

function getDefaultMerchantSubmitActionLabel(normalizedStatus: MerchantApplymentNormalizedStatus) {
  if (normalizedStatus === 'rejected' || normalizedStatus === 'cancelled') {
    return '重新提交资料'
  }
  return '填写开户资料'
}

function getMerchantApplymentGuideTitle(options: {
  headline: string
  canSubmitOpenInfo: boolean
  needsAccountValidation: boolean
  needsSign: boolean
  isInReview: boolean
  isOpened: boolean
}): string {
  if (options.isOpened) {
    return '收款能力已开通'
  }

  if (options.canSubmitOpenInfo) {
    return '提交开户资料'
  }

  if (options.needsAccountValidation && options.needsSign) {
    return '完成微信待办'
  }

  if (options.needsAccountValidation) {
    return '完成账户验证'
  }

  if (options.needsSign) {
    return '完成微信待办'
  }

  if (options.isInReview) {
    return '等待微信审核'
  }

  return options.headline
}

function getMerchantApplymentGuideDescription(options: {
  guideText: string
  summaryText: string
  blockReason: string
}): string {
  if (options.guideText) {
    return options.guideText
  }

  if (options.blockReason) {
    return options.blockReason
  }

  return options.summaryText
}

function getMerchantApplymentFlowStepStateText(state: MerchantApplymentFlowStepState): string {
  switch (state) {
    case 'done':
      return '已完成'
    case 'current':
      return '当前步骤'
    default:
      return '待开始'
  }
}

function buildMerchantApplymentHeadline(options: {
  normalizedStatus: MerchantApplymentNormalizedStatus
  canSubmitOpenInfo: boolean
  needsAccountValidation: boolean
  needsSign: boolean
  isInReview: boolean
  isOpened: boolean
}): string {
  if (options.isOpened) {
    return '微信支付已开通'
  }

  if (options.canSubmitOpenInfo) {
    return options.normalizedStatus === 'rejected' || options.normalizedStatus === 'cancelled'
      ? '需要重新提交资料'
      : '先开通微信支付'
  }

  if (options.needsAccountValidation && options.needsSign) {
    return '完成微信待办'
  }

  if (options.needsAccountValidation) {
    return '先完成账户验证'
  }

  if (options.needsSign) {
    return '完成微信待办'
  }

  if (options.isInReview) {
    return '微信正在审核资料'
  }

  if (options.normalizedStatus === 'frozen') {
    return '当前账户不可用'
  }

  if (options.normalizedStatus === 'cancelled') {
    return '当前申请已作废'
  }

  return '查看开户状态'
}

function buildMerchantApplymentSummaryText(options: {
  headline: string
  guideText: string
  isOpened: boolean
}): string {
  if (options.isOpened) {
    return '可以继续查看结算账户，并在财务页跟进订单流水与结算情况；余额和提现请前往微信支付商户平台/商家助手处理。'
  }

  if (options.guideText) {
    return options.guideText
  }

  return options.headline
}

function buildMerchantApplymentPrimaryActionText(options: {
  canSubmitOpenInfo: boolean
  submitActionLabel: string
  needsAccountValidation: boolean
  needsSign: boolean
  isInReview: boolean
  isOpened: boolean
}): string {
  if (options.isOpened) {
    return '查看结算账户'
  }

  if (options.canSubmitOpenInfo) {
    return options.submitActionLabel
  }

  if (options.needsAccountValidation || options.needsSign) {
    return '处理当前待办'
  }

  if (options.isInReview) {
    return '刷新最新状态'
  }

  return '查看开户进度'
}

function buildMerchantApplymentFlowSteps(options: {
  canSubmitOpenInfo: boolean
  hasApplyment: boolean
  needsAccountValidation: boolean
  needsSign: boolean
  isInReview: boolean
  isOpened: boolean
}): MerchantApplymentFlowStepView[] {
  const isActionStage = options.needsAccountValidation || options.needsSign

  const submitState: MerchantApplymentFlowStepState = options.hasApplyment && !options.canSubmitOpenInfo
    ? 'done'
    : 'current'
  const verifyState: MerchantApplymentFlowStepState = options.isOpened
    ? 'done'
    : options.isInReview
      ? 'done'
      : isActionStage
        ? 'current'
        : options.hasApplyment
          ? 'pending'
          : 'pending'
  const reviewState: MerchantApplymentFlowStepState = options.isOpened
    ? 'done'
    : options.isInReview
      ? 'current'
      : options.hasApplyment
        ? 'pending'
        : 'pending'
  const openedState: MerchantApplymentFlowStepState = options.isOpened ? 'current' : 'pending'

  return [
    {
      key: 'submit',
      title: '提交资料',
      description: '准备结算银行卡资料并提交开户申请。',
      state: submitState,
      stateText: getMerchantApplymentFlowStepStateText(submitState)
    },
    {
      key: 'verify',
      title: '微信待办',
      description: '根据微信返回结果完成账户验证或签约。',
      state: verifyState,
      stateText: getMerchantApplymentFlowStepStateText(verifyState)
    },
    {
      key: 'review',
      title: '微信审核',
      description: '微信支付会继续校验开户资料和账户状态。',
      state: reviewState,
      stateText: getMerchantApplymentFlowStepStateText(reviewState)
    },
    {
      key: 'opened',
      title: '开通收款',
      description: '开通后即可正常收款。',
      state: openedState,
      stateText: getMerchantApplymentFlowStepStateText(openedState)
    }
  ]
}

function getMerchantApplymentFlowCurrent(steps: MerchantApplymentFlowStepView[]): number {
  if (steps.length > 0 && steps.every((step) => step.state === 'done')) {
    return steps.length
  }

  const currentIndex = steps.findIndex((step) => step.state === 'current')
  if (currentIndex >= 0) {
    return currentIndex
  }

  const lastDoneIndex = steps.reduce((index, step, stepIndex) => {
    return step.state === 'done' ? stepIndex : index
  }, -1)

  return lastDoneIndex >= 0 ? lastDoneIndex : 0
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

function resolveMerchantApplymentStatusDesc(
  rawStatusDesc: string,
  statusCode: string,
  options: {
    needsAccountValidation: boolean
    needsSign: boolean
    needsLegalValidation: boolean
  }
): string {
  if (options.needsAccountValidation && options.needsSign) {
    return '当前申请有微信待办，请按微信提示完成。'
  }

  if (options.needsAccountValidation) {
    return options.needsLegalValidation
      ? '当前申请待账户验证，可优先使用法人扫码验证。'
      : '当前申请待账户验证，请按汇款指引完成验证。'
  }

  if (statusCode === 'to_be_confirmed') {
    return rawStatusDesc || '当前申请待账户验证，请按微信提示完成。'
  }

  if (options.needsSign) {
    return '当前申请有微信待办，完成后回到本页会自动刷新。'
  }

  return rawStatusDesc || getDefaultMerchantApplymentStatusDesc(statusCode)
}

export function buildMerchantApplymentStatusView(
  status: ApplymentStatusResponse | null
): MerchantApplymentStatusView {
  if (!status) {
    return { ...DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW }
  }

  const statusCode = String(status.status || 'not_applied').trim().toLowerCase() || 'not_applied'
  const normalizedStatus = normalizeMerchantApplymentStatus(statusCode)
  const signState = normalizeMerchantApplymentSignState(status.sign_state)
  const signStateText = getMerchantApplymentSignStateText(signState)
  const accountValidation = buildMerchantApplymentAccountValidationView(status.account_validation)
  const signURL = status.sign_url || ''
  const legalValidationURL = status.legal_validation_url || ''
  const actionHint = normalizeMerchantApplymentActionText(status.action_hint)
  const needsAccountValidation = Boolean(accountValidation) || Boolean(legalValidationURL) || statusCode === 'to_be_confirmed' || statusCode === 'account_need_verify'
  const needsLegalValidation = Boolean(legalValidationURL)
  const needsSign = Boolean(signURL) && shouldMerchantApplymentNeedSign(signState, statusCode)
  const statusDesc = normalizeMerchantApplymentActionText(resolveMerchantApplymentStatusDesc(status.status_desc || '', statusCode, {
    needsAccountValidation,
    needsSign,
    needsLegalValidation
  }))
  const hasPendingActions = needsAccountValidation || needsSign
  const canSubmitOpenInfo = typeof status.can_submit === 'boolean'
    ? status.can_submit
    : normalizedStatus === 'pending' || normalizedStatus === 'rejected' || normalizedStatus === 'cancelled'
  const rejectReason = normalizeMerchantApplymentActionText(status.reject_reason) || '-'
  const blockReason = normalizeMerchantApplymentActionText(status.block_reason) || getDefaultMerchantApplymentBlockReason(normalizedStatus, statusDesc, statusCode, {
    needsAccountValidation,
    needsSign
  })
  const subMchId = status.sub_mch_id || '-'
  const isOpened = normalizedStatus === 'opened'
  const isInReview = normalizedStatus === 'in_review' && !hasPendingActions
  const showRejectReason = normalizedStatus === 'rejected' && rejectReason !== '-'
  const hasApplyment = normalizedStatus !== 'pending'

  let guideText = DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW.guideText
  let guideTheme: MerchantApplymentGuideTheme = 'primary'
  let tagTheme: MerchantApplymentStatusView['tagTheme'] = 'default'
  let tagText = getDefaultMerchantApplymentTagText(statusCode)
  const submitActionLabel = getDefaultMerchantSubmitActionLabel(normalizedStatus)

  if (normalizedStatus === 'rejected') {
    guideText = showRejectReason
      ? '开户资料已被驳回，请根据拒绝原因修改后重新提交。'
      : '开户资料需要重新提交，请核对信息后再试。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (needsAccountValidation && needsSign) {
    guideText = '当前申请有微信待办，请按微信提示完成。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待处理'
  } else if (needsAccountValidation) {
    guideText = needsLegalValidation
      ? '当前申请待账户验证，可优先使用法人扫码验证；若无法扫码，请按汇款指引完成验证。'
      : '当前申请待账户验证，请按汇款指引完成验证。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待验证'
  } else if (needsSign) {
    guideText = '当前申请有微信待办，完成后回到本页会自动刷新。'
    guideTheme = 'warning'
    tagTheme = 'primary'
    tagText = '待处理'
  } else if (normalizedStatus === 'in_review') {
    guideText = '微信支付正在审核开户资料，审核期间无需重复提交。'
    guideTheme = 'warning'
    tagTheme = 'warning'
  } else if (isOpened) {
    guideText = '微信支付已开通，可正常收款。'
    guideTheme = 'success'
    tagTheme = 'success'
  } else if (normalizedStatus === 'frozen') {
    guideText = blockReason || '当前开户状态不可用，暂不支持提交资料。'
    guideTheme = 'danger'
    tagTheme = 'danger'
  } else if (normalizedStatus === 'cancelled') {
    guideText = blockReason || '当前申请已作废，可重新提交开户资料。'
    guideTheme = 'warning'
    tagTheme = 'default'
  } else if (!canSubmitOpenInfo && blockReason) {
    guideText = blockReason
    guideTheme = 'warning'
    tagTheme = 'default'
  }

  const headline = buildMerchantApplymentHeadline({
    normalizedStatus,
    canSubmitOpenInfo,
    needsAccountValidation,
    needsSign,
    isInReview,
    isOpened
  })
  const summaryText = buildMerchantApplymentSummaryText({
    headline,
    guideText,
    isOpened
  })
  const primaryActionText = buildMerchantApplymentPrimaryActionText({
    canSubmitOpenInfo,
    submitActionLabel,
    needsAccountValidation,
    needsSign,
    isInReview,
    isOpened
  })
  const guideTitle = getMerchantApplymentGuideTitle({
    headline,
    canSubmitOpenInfo,
    needsAccountValidation,
    needsSign,
    isInReview,
    isOpened
  })
  const guideDescription = getMerchantApplymentGuideDescription({
    guideText,
    summaryText,
    blockReason
  })
  const flowSteps = buildMerchantApplymentFlowSteps({
    canSubmitOpenInfo,
    hasApplyment,
    needsAccountValidation,
    needsSign,
    isInReview,
    isOpened
  })
  const flowCurrent = getMerchantApplymentFlowCurrent(flowSteps)

  return {
    statusCode,
    normalizedStatus,
    headline,
    summaryText,
    flowCurrent,
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
    needsLegalValidation,
    hasPendingActions,
    actionHint,
    showRejectReason,
    hasApplyment,
    guideTheme,
    guideTitle,
    guideText,
    guideDescription,
    primaryActionText,
    submitActionLabel,
    flowSteps,
    accountValidation
  }
}

export async function getMerchantApplymentStatus(): Promise<ApplymentStatusResponse> {
  return request({
    url: '/v1/merchant/applyment/status',
    method: 'GET'
  })
}

export async function merchantBindBank(payload: MerchantBindBankRequest): Promise<MerchantBindBankResponse> {
  return request({
    url: '/v1/merchant/applyment/bindbank',
    method: 'POST',
    data: payload
  })
}
