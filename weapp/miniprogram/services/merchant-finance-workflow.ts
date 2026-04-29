import {
  createMerchantCancelWithdrawApplication,
  createMerchantWithdraw,
  getMerchantCancelWithdrawApplication,
  getMerchantWithdrawal,
  type CreateMerchantCancelWithdrawApplicationRequest,
  type MerchantAccountBalanceResponse,
  type MerchantCancelWithdrawApplicationItem,
  type MerchantCancelWithdrawEligibilityResponse,
  type MerchantFinanceOverviewResponse,
  type MerchantFinanceStatusTheme,
  type MerchantWithdrawItem,
  type MerchantWithdrawRequest
} from '../api/merchant-finance'
import { getErrorUserMessage } from '../utils/user-facing'

const WITHDRAW_WAIT_MAX_ATTEMPTS = 20
const WITHDRAW_WAIT_INTERVAL_MS = 1500
const CANCEL_WITHDRAW_WAIT_MAX_ATTEMPTS = 20
const CANCEL_WITHDRAW_WAIT_INTERVAL_MS = 1500

export interface MerchantFinanceAmountView {
  amount: number
  text: string
}

export interface MerchantFinanceStatusView {
  text: string
  theme: MerchantFinanceStatusTheme
  isTerminal: boolean
  isSuccess: boolean
  isFailure: boolean
  canRefresh: boolean
}

export interface MerchantFinanceOverviewView {
  completedOrders: number
  totalGmv: MerchantFinanceAmountView
  totalIncome: MerchantFinanceAmountView
  netIncome: MerchantFinanceAmountView
  totalServiceFee: MerchantFinanceAmountView
  pendingIncome: MerchantFinanceAmountView
}

export interface MerchantAccountBalanceView {
  accountStatus: string
  statusDesc: string
  isActive: boolean
  availableAmount: MerchantFinanceAmountView
  pendingAmount: MerchantFinanceAmountView
  withdrawableAmount: MerchantFinanceAmountView
}

export interface MerchantWithdrawalView {
  id: number
  amount: MerchantFinanceAmountView
  status: MerchantFinanceStatusView
  channelText: string
  reasonText: string
  createdAtText: string
  updatedAtText: string
  raw: MerchantWithdrawItem
}

export interface MerchantCancelWithdrawApplicationView {
  id: number
  status: MerchantFinanceStatusView
  withdrawStatus: MerchantFinanceStatusView
  outRequestNo: string
  applymentId: string
  modeText: string
  description: string
  lastErrorText: string
  businessLicenseStatusText: string
  confirmCancelUrl: string
  proofMediaCountText: string
  additionalMaterialCountText: string
  accountInfoText: string
  accountWithdrawResultText: string
  createdAtText: string
  updatedAtText: string
  canRefresh: boolean
  raw: MerchantCancelWithdrawApplicationItem
}

export interface MerchantCancelWithdrawEligibilityView {
  accountStatus: string
  statusDesc: string
  eligible: boolean
  blockReasonText: string
  accountAmountText: string
  raw: MerchantCancelWithdrawEligibilityResponse
}

export interface MerchantWithdrawalWaitResult {
  withdrawal: MerchantWithdrawalView
  timedOut: boolean
}

export interface MerchantCancelWithdrawWaitResult {
  application: MerchantCancelWithdrawApplicationView
  timedOut: boolean
}

export interface WaitForMerchantFinanceTerminalOptions {
  maxAttempts?: number
  intervalMs?: number
}

export type MerchantCancelWithdrawMode = 'NOT_APPLY_WITHDRAW' | 'APPLY_WITHDRAW'
export type MerchantCancelWithdrawAccountType = 'ACCOUNT_TYPE_CORPORATE' | 'ACCOUNT_TYPE_PERSONAL'
export type MerchantCancelWithdrawLicenseStatus = '' | 'ACTIVE' | 'CANCELED' | 'REVOKED'

export interface MerchantCancelWithdrawSubmitDraft {
  withdraw: MerchantCancelWithdrawMode
  businessLicenseStatusDeclaration: MerchantCancelWithdrawLicenseStatus
  accountType: MerchantCancelWithdrawAccountType
  accountName: string
  accountBank: string
  bankBranchId: string
  bankBranchName: string
  accountNumber: string
  idDocType: string
  identificationName: string
  identificationNo: string
  proofMediaAssetIds: number[]
  additionalMaterialAssetIds: number[]
  remark: string
}

export interface MerchantCancelWithdrawPayloadBuildResult {
  payload?: CreateMerchantCancelWithdrawApplicationRequest
  errorMessage?: string
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export function formatFenToYuan(amount?: number): string {
  const normalized = Number.isFinite(amount) ? Math.max(Number(amount), 0) : 0
  return `¥${(normalized / 100).toFixed(2)}`
}

export function buildAmountView(amount?: number): MerchantFinanceAmountView {
  const normalized = Number.isFinite(amount) ? Number(amount) : 0
  return {
    amount: normalized,
    text: formatFenToYuan(normalized)
  }
}

export function formatDateTimeText(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hour = `${date.getHours()}`.padStart(2, '0')
  const minute = `${date.getMinutes()}`.padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

export function buildDefaultFinanceRange(days = 30): { start_date: string, end_date: string } {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))

  return {
    start_date: formatDateParam(start),
    end_date: formatDateParam(end)
  }
}

function formatDateParam(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildAccountBalanceView(balance: MerchantAccountBalanceResponse): MerchantAccountBalanceView {
  const normalizedStatus = String(balance.account_status || '').trim().toLowerCase()
  return {
    accountStatus: normalizedStatus,
    statusDesc: balance.status_desc || '',
    isActive: normalizedStatus === 'active',
    availableAmount: buildAmountView(balance.available_amount),
    pendingAmount: buildAmountView(balance.pending_amount),
    withdrawableAmount: buildAmountView(balance.withdrawable_amount)
  }
}

export function buildFinanceOverviewView(overview: MerchantFinanceOverviewResponse): MerchantFinanceOverviewView {
  return {
    completedOrders: overview.completed_orders || 0,
    totalGmv: buildAmountView(overview.total_gmv),
    totalIncome: buildAmountView(overview.total_income),
    netIncome: buildAmountView(overview.net_income),
    totalServiceFee: buildAmountView(overview.total_service_fee),
    pendingIncome: buildAmountView(overview.pending_income)
  }
}

export function getMerchantWithdrawalStatusView(status?: string): MerchantFinanceStatusView {
  switch (String(status || '').trim()) {
    case 'success':
      return buildStatusView('成功', 'success', true, true, false)
    case 'failed':
      return buildStatusView('失败', 'danger', true, false, true)
    case 'pending':
      return buildStatusView('处理中', 'warning', false, false, false)
    default:
      return buildStatusView('待同步', 'default', false, false, false)
  }
}

export function getMerchantCancelWithdrawStatusView(status?: string, localSyncState?: string): MerchantFinanceStatusView {
  const normalizedStatus = String(status || '').trim()
  const normalizedSyncState = String(localSyncState || '').trim()

  if (normalizedSyncState === 'submit_unknown') {
    return buildStatusView('结果待同步', 'warning', false, false, false)
  }
  if (normalizedSyncState === 'sync_failed') {
    return buildStatusView('提交失败', 'danger', true, false, true)
  }

  switch (normalizedStatus) {
    case 'FINISH':
      return buildStatusView('已完成', 'success', true, true, false)
    case 'REJECTED':
      return buildStatusView('已驳回', 'danger', true, false, true)
    case 'REVOKED':
      return buildStatusView('已撤销', 'danger', true, false, true)
    case 'CANCELED':
      return buildStatusView('已取消', 'danger', true, false, true)
    case 'ACCEPTED':
      return buildStatusView('已受理', 'primary', false, false, false)
    case 'REVIEWING':
      return buildStatusView('审核中', 'primary', false, false, false)
    case 'WAITING_MERCHANT_CONFIRM':
      return buildStatusView('待商户确认', 'warning', false, false, false)
    case 'SYSTEM_PROCESSING':
    case 'FUND_PROCESSING':
      return buildStatusView('处理中', 'warning', false, false, false)
    default:
      return buildStatusView('待同步', 'default', false, false, false)
  }
}

export function getMerchantCancelWithdrawWithdrawStatusView(status?: string, withdraw?: string): MerchantFinanceStatusView {
  switch (String(status || '').trim()) {
    case 'WITHDRAW_SUCCEED':
      return buildStatusView('提现成功', 'success', true, true, false)
    case 'WITHDRAW_EXCEPTION':
      return buildStatusView('提现异常', 'danger', true, false, true)
    case 'WITHDRAW_PROCESSING':
      return buildStatusView('提现处理中', 'warning', false, false, false)
    default:
      if (withdraw === 'APPLY_WITHDRAW') {
        return buildStatusView('提现待同步', 'warning', false, false, false)
      }
      return buildStatusView('无需提现', 'default', true, false, false)
  }
}

export function buildMerchantCancelWithdrawPayload(
  draft: MerchantCancelWithdrawSubmitDraft
): MerchantCancelWithdrawPayloadBuildResult {
  const remark = normalizeShortText(draft.remark)
  if (remark && !/^[0-9A-Za-z\u4e00-\u9fa5]+$/.test(remark)) {
    return { errorMessage: '备注仅支持中文、数字或字母' }
  }

  if (draft.withdraw === 'NOT_APPLY_WITHDRAW') {
    return {
      payload: {
        withdraw: 'NOT_APPLY_WITHDRAW',
        ...(remark ? { remark } : {})
      }
    }
  }

  const accountName = normalizeShortText(draft.accountName)
  const accountBank = normalizeShortText(draft.accountBank)
  const accountNumber = normalizeShortText(draft.accountNumber)
  if (!accountName || !accountBank || !accountNumber) {
    return { errorMessage: '请填写收款户名、开户银行和银行账号' }
  }

  const accountType = draft.accountType === 'ACCOUNT_TYPE_PERSONAL' ? 'ACCOUNT_TYPE_PERSONAL' : 'ACCOUNT_TYPE_CORPORATE'
  const payload: CreateMerchantCancelWithdrawApplicationRequest = {
    withdraw: 'APPLY_WITHDRAW',
    payee_info: {
      account_type: accountType,
      bank_account_info: {
        account_name: accountName,
        account_bank: accountBank,
        account_number: accountNumber,
        bank_branch_id: normalizeShortText(draft.bankBranchId),
        bank_branch_name: normalizeShortText(draft.bankBranchName)
      }
    },
    proof_media_asset_ids: draft.proofMediaAssetIds.filter((assetId) => assetId > 0),
    additional_material_asset_ids: draft.additionalMaterialAssetIds.filter((assetId) => assetId > 0),
    ...(remark ? { remark } : {})
  }

  if (draft.businessLicenseStatusDeclaration) {
    payload.business_license_status_declaration = draft.businessLicenseStatusDeclaration
  }
  if ((draft.businessLicenseStatusDeclaration === 'CANCELED' || draft.businessLicenseStatusDeclaration === 'REVOKED') && !payload.proof_media_asset_ids?.length) {
    return { errorMessage: '营业执照已注销或吊销时，请上传注销提现证明材料' }
  }
  if ((payload.proof_media_asset_ids || []).length > 1) {
    return { errorMessage: '注销提现证明材料最多上传 1 张' }
  }
  if ((payload.additional_material_asset_ids || []).length > 10) {
    return { errorMessage: '补充材料最多上传 10 张' }
  }

  if (accountType === 'ACCOUNT_TYPE_PERSONAL') {
    const idDocType = normalizeShortText(draft.idDocType)
    const identificationName = normalizeShortText(draft.identificationName)
    const identificationNo = normalizeShortText(draft.identificationNo)
    if (!idDocType || !identificationName || !identificationNo) {
      return { errorMessage: '个人账户请填写证件类型、证件姓名和证件号码' }
    }
    payload.payee_info = {
      ...payload.payee_info,
      identity_info: {
        id_doc_type: idDocType,
        identification_name: identificationName,
        identification_no: identificationNo
      }
    }
  }

  return { payload }
}

function normalizeShortText(value?: string): string {
  return String(value || '').trim()
}

function buildStatusView(
  text: string,
  theme: MerchantFinanceStatusTheme,
  isTerminal: boolean,
  isSuccess: boolean,
  isFailure: boolean
): MerchantFinanceStatusView {
  return {
    text,
    theme,
    isTerminal,
    isSuccess,
    isFailure,
    canRefresh: !isTerminal
  }
}

export function buildWithdrawalView(withdrawal: MerchantWithdrawItem): MerchantWithdrawalView {
  return {
    id: withdrawal.id,
    amount: buildAmountView(withdrawal.amount),
    status: getMerchantWithdrawalStatusView(withdrawal.status),
    channelText: withdrawal.channel || '微信支付',
    reasonText: withdrawal.reason || '',
    createdAtText: formatDateTimeText(withdrawal.created_at),
    updatedAtText: formatDateTimeText(withdrawal.updated_at),
    raw: withdrawal
  }
}

export function buildCancelWithdrawApplicationView(
  application: MerchantCancelWithdrawApplicationItem
): MerchantCancelWithdrawApplicationView {
  const status = getMerchantCancelWithdrawStatusView(application.cancel_state, application.local_sync_state)
  const withdrawStatus = getMerchantCancelWithdrawWithdrawStatusView(application.withdraw_state, application.withdraw)
  const description = application.cancel_state_description || application.withdraw_state_description || ''

  return {
    id: application.id,
    status,
    withdrawStatus,
    outRequestNo: application.out_request_no || '',
    applymentId: application.applyment_id || '',
    modeText: application.withdraw === 'APPLY_WITHDRAW' ? '提现后注销' : '不提现注销',
    description,
    lastErrorText: application.last_error
      ? getErrorUserMessage(application.last_error, '状态同步失败，请刷新后再试')
      : '',
    businessLicenseStatusText: getBusinessLicenseStatusText(application.business_license_status_declaration),
    confirmCancelUrl: application.confirm_cancel_url || '',
    proofMediaCountText: `${(application.proof_media_asset_ids || []).length} 张`,
    additionalMaterialCountText: `${(application.additional_material_asset_ids || []).length} 张`,
    accountInfoText: (application.account_info || []).map((item) => `${getCancelWithdrawAccountTypeText(item.out_account_type)} ${formatFenToYuan(item.amount)}`).join('，') || '无账户余额',
    accountWithdrawResultText: (application.account_withdraw_result || []).map((item) => `${getCancelWithdrawAccountTypeText(item.out_account_type)} ${getCancelWithdrawPayStateText(item.pay_state)}`).join('，') || '无提现结果',
    createdAtText: formatDateTimeText(application.created_at),
    updatedAtText: formatDateTimeText(application.updated_at),
    canRefresh: status.canRefresh,
    raw: application
  }
}

function getBusinessLicenseStatusText(status?: string): string {
  switch (String(status || '').trim()) {
    case 'ACTIVE':
      return '正常'
    case 'CANCELED':
      return '已注销'
    case 'REVOKED':
      return '已吊销'
    default:
      return '未声明'
  }
}

function getCancelWithdrawPayStateText(status?: string): string {
  switch (String(status || '').trim()) {
    case 'PAY_SUCCEED':
      return '提现成功'
    case 'PAY_FAIL':
      return '提现失败'
    case 'BANK_REFUNDED':
      return '银行退票'
    case 'PAY_PROCESSING':
      return '提现处理中'
    default:
      return '待同步'
  }
}

export function buildCancelWithdrawEligibilityView(
  eligibility: MerchantCancelWithdrawEligibilityResponse
): MerchantCancelWithdrawEligibilityView {
  const accountInfo = eligibility.eligibility?.account_info || []
  const blockReasons = eligibility.eligibility?.block_reasons || []
  return {
    accountStatus: eligibility.account_status || '',
    statusDesc: eligibility.status_desc || '',
    eligible: !!eligibility.eligible,
    blockReasonText: blockReasons.map(buildCancelWithdrawBlockReasonText).filter(Boolean).join('；'),
    accountAmountText: accountInfo.map((item) => `${getCancelWithdrawAccountTypeText(item.out_account_type)}: ${formatFenToYuan(item.amount)}`).join('，'),
    raw: eligibility
  }
}

function buildCancelWithdrawBlockReasonText(reason: { type?: string, description?: string }): string {
  if (reason.description) {
    return reason.description
  }

  switch (reason.type) {
    case 'CONSUMER_COMPLAINT_UNPROCESSED':
      return '有消费者投诉待处理'
    case 'HAS_BLOCKING_CONTROL':
      return '账户存在管控限制'
    case 'FUNDS_PENDING_PROCESSING':
      return '有资金仍在处理中'
    case 'OTHER_REASON':
      return '暂不满足注销提现条件'
    default:
      return '暂不满足注销提现条件'
  }
}

function getCancelWithdrawAccountTypeText(accountType: string): string {
  switch (accountType) {
    case 'BASIC_ACCOUNT':
      return '基本账户'
    case 'OPERATE_ACCOUNT':
      return '运营账户'
    case 'MARGIN_ACCOUNT':
      return '保证金账户'
    case 'TRADE_FEE_ACCOUNT':
      return '交易手续费账户'
    default:
      return '账户余额'
  }
}

export async function submitMerchantWithdrawAndWait(
  payload: MerchantWithdrawRequest,
  options: WaitForMerchantFinanceTerminalOptions = {}
): Promise<MerchantWithdrawalWaitResult> {
  const response = await createMerchantWithdraw(payload)
  const withdrawal = response.withdrawal
  return waitForMerchantWithdrawalTerminalStatus(withdrawal.id, withdrawal, options)
}

export async function waitForMerchantWithdrawalTerminalStatus(
  withdrawalId: number,
  fallbackWithdrawal?: MerchantWithdrawItem,
  options: WaitForMerchantFinanceTerminalOptions = {}
): Promise<MerchantWithdrawalWaitResult> {
  const maxAttempts = Math.max(1, options.maxAttempts || WITHDRAW_WAIT_MAX_ATTEMPTS)
  const intervalMs = Math.max(500, options.intervalMs || WITHDRAW_WAIT_INTERVAL_MS)
  let latest = fallbackWithdrawal ? buildWithdrawalView(fallbackWithdrawal) : null

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    try {
      latest = buildWithdrawalView(await getMerchantWithdrawal(withdrawalId))
      if (latest.status.isTerminal) {
        return { withdrawal: latest, timedOut: false }
      }
    } catch (_error: unknown) {
      if (!latest) {
        throw _error
      }
    }

    if (attempt < maxAttempts - 1) {
      await wait(intervalMs)
    }
  }

  if (!latest) {
    latest = buildWithdrawalView(await getMerchantWithdrawal(withdrawalId))
  }
  return { withdrawal: latest, timedOut: true }
}

export async function submitMerchantCancelWithdrawAndWait(
  payload: CreateMerchantCancelWithdrawApplicationRequest,
  options: WaitForMerchantFinanceTerminalOptions = {}
): Promise<MerchantCancelWithdrawWaitResult> {
  const response = await createMerchantCancelWithdrawApplication(payload)
  return waitForMerchantCancelWithdrawTerminalStatus(response.application.id, response.application, options)
}

export async function waitForMerchantCancelWithdrawTerminalStatus(
  applicationId: number,
  fallbackApplication?: MerchantCancelWithdrawApplicationItem,
  options: WaitForMerchantFinanceTerminalOptions = {}
): Promise<MerchantCancelWithdrawWaitResult> {
  const maxAttempts = Math.max(1, options.maxAttempts || CANCEL_WITHDRAW_WAIT_MAX_ATTEMPTS)
  const intervalMs = Math.max(500, options.intervalMs || CANCEL_WITHDRAW_WAIT_INTERVAL_MS)
  let latest = fallbackApplication ? buildCancelWithdrawApplicationView(fallbackApplication) : null

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    try {
      latest = buildCancelWithdrawApplicationView(await getMerchantCancelWithdrawApplication(applicationId))
      if (latest.status.isTerminal) {
        return { application: latest, timedOut: false }
      }
    } catch (_error: unknown) {
      if (!latest) {
        throw _error
      }
    }

    if (attempt < maxAttempts - 1) {
      await wait(intervalMs)
    }
  }

  if (!latest) {
    latest = buildCancelWithdrawApplicationView(await getMerchantCancelWithdrawApplication(applicationId))
  }
  return { application: latest, timedOut: true }
}

export function getMerchantFinanceUserMessage(error: unknown, fallback: string): string {
  return getErrorUserMessage(error, fallback)
}