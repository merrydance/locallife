import Toast, { hideToast } from '../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../utils/logger'
import {
  BaofuAccountOwnerRole,
  BaofuAccountProfile,
  BaofuSettlementAccountPayment,
  BaofuSettlementAccountResponse,
  getBaofuAccountPayment,
  getBaofuAccountNextActionText,
  getBaofuSettlementAccount,
  getBaofuAccountStatusText,
  isBaofuSettlementAfterPaymentTerminalStatus,
  isBaofuSettlementOpeningProcessingStatus,
  isBaofuSettlementPaymentRequiredStatus,
  isBaofuSettlementTerminalStatus,
  submitBaofuSettlementAccountProfile
} from '../api/baofu-account'
import { PAYMENT_STATUS_POLL_INTERVAL_MS, type PaymentOrderResponse } from '../api/payment'
import { completePaymentWorkflow } from './payment-workflow'

const TOAST_SELECTOR = '#t-toast'
const BAOFU_STATUS_POLL_MAX_ATTEMPTS = 45
const PENDING_STORAGE_PREFIX = 'baofuSettlementAccountPendingWorkflow:'
const GENERIC_RETRY_MESSAGE = '开户进度暂时无法同步，请稍后刷新'

export type BaofuOnboardingWorkflowStatus =
  | 'ready'
  | 'failed'
  | 'profile_pending'
  | 'verify_fee_pending'
  | 'closed'
  | 'processing'
  | 'cancelled'
  | 'pending_confirmation'
  | 'pay_params_missing'

export interface BaofuOnboardingWorkflowResult {
  status: BaofuOnboardingWorkflowStatus
  account: BaofuSettlementAccountResponse
  shouldRefreshWorkbench: boolean
}

interface WorkflowOptions {
  role?: BaofuAccountOwnerRole
  context?: WechatMiniprogram.Page.TrivialInstance
  loadingMessage?: string
  maxAttempts?: number
  interval?: number
}

export interface PendingWorkflowContext {
  role: BaofuAccountOwnerRole
  paymentOrderId: number
  amount: number
  outTradeNo?: string
  updatedAt: string
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function showProgressToast(context: WechatMiniprogram.Page.TrivialInstance | undefined, message: string) {
  if (!context) {
    return
  }

  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme: 'loading',
    direction: 'column',
    duration: 0,
    preventScrollThrough: true
  })
}

function hideProgressToast(context: WechatMiniprogram.Page.TrivialInstance | undefined) {
  if (!context) {
    return
  }

  hideToast({ context, selector: TOAST_SELECTOR })
}

function storageKey(role: BaofuAccountOwnerRole): string {
  return `${PENDING_STORAGE_PREFIX}${role}`
}

function withUserMessage(error: unknown, fallback: string): unknown {
  if (error && typeof error === 'object') {
    const candidate = error as { userMessage?: unknown }
    if (typeof candidate.userMessage !== 'string' || !candidate.userMessage.trim()) {
      candidate.userMessage = fallback
    }
    return error
  }

  const wrapped = new Error(fallback)
  ;(wrapped as Error & { userMessage?: string, originalError?: unknown }).userMessage = fallback
  ;(wrapped as Error & { userMessage?: string, originalError?: unknown }).originalError = error
  return wrapped
}

function logAndThrowWorkflowError(
  action: string,
  role: BaofuAccountOwnerRole,
  error: unknown,
  fallback = GENERIC_RETRY_MESSAGE
): never {
  logger.error(`宝付开户流程异常 action=${action} role=${role}`, error, 'baofu-account-onboarding')
  throw withUserMessage(error, fallback)
}

function isPendingWorkflowContext(value: unknown): value is PendingWorkflowContext {
  if (!value || typeof value !== 'object') {
    return false
  }

  const candidate = value as Partial<PendingWorkflowContext>
  return (
    candidate.role === 'merchant' ||
    candidate.role === 'rider' ||
    candidate.role === 'operator'
  ) && Number.isFinite(candidate.paymentOrderId) && Number.isFinite(candidate.amount) && typeof candidate.updatedAt === 'string'
}

function savePendingWorkflowContext(context: PendingWorkflowContext) {
  try {
    wx.setStorageSync(storageKey(context.role), context)
  } catch (error: unknown) {
    logger.error(`保存宝付开户待确认支付上下文失败 role=${context.role}`, error, 'baofu-account-onboarding')
    const recoverableError = new Error('开户进度暂时无法保存，请稍后再试')
    ;(recoverableError as Error & { userMessage?: string }).userMessage = '开户进度暂时无法保存，请稍后再试'
    throw recoverableError
  }
}

function loadPendingWorkflowContext(role: BaofuAccountOwnerRole): PendingWorkflowContext | null {
  try {
    const stored = wx.getStorageSync(storageKey(role)) as unknown
    if (!isPendingWorkflowContext(stored)) {
      return null
    }
    return stored
  } catch (error: unknown) {
    logger.error(`读取宝付开户待确认支付上下文失败 role=${role}`, error, 'baofu-account-onboarding')
    const recoverableError = new Error('开户进度暂时无法读取，请稍后刷新')
    ;(recoverableError as Error & { userMessage?: string }).userMessage = '开户进度暂时无法读取，请稍后刷新'
    throw recoverableError
  }
}

function clearPendingWorkflowContext(role: BaofuAccountOwnerRole) {
  try {
    wx.removeStorageSync(storageKey(role))
  } catch (error: unknown) {
    logger.error(`清除宝付开户待确认支付上下文失败 role=${role}`, error, 'baofu-account-onboarding')
  }
}

export function getPendingBaofuAccountOnboardingContext(role: BaofuAccountOwnerRole): PendingWorkflowContext | null {
  return loadPendingWorkflowContext(role)
}

export function clearPendingBaofuAccountOnboardingContext(role: BaofuAccountOwnerRole) {
  clearPendingWorkflowContext(role)
}

function buildPendingWorkflowContext(
  role: BaofuAccountOwnerRole,
  payment: BaofuSettlementAccountPayment | null | undefined,
  fallbackAmount: number
): PendingWorkflowContext | null {
  const paymentOrderId = Number(payment?.payment_order_id || 0)
  if (!paymentOrderId) {
    return null
  }

  return {
    role,
    paymentOrderId,
    amount: Number(payment?.amount || fallbackAmount || 200),
    outTradeNo: payment?.out_trade_no,
    updatedAt: new Date().toISOString()
  }
}

function mapAccountStatus(status?: string): BaofuOnboardingWorkflowStatus {
  const normalized = String(status || '').trim().toLowerCase()
  switch (normalized) {
    case 'ready':
      return 'ready'
    case 'failed':
      return 'failed'
    case 'profile_pending':
      return 'profile_pending'
    case 'verify_fee_pending':
      return 'verify_fee_pending'
    case 'closed':
      return 'closed'
    case 'opening_processing':
    case 'verify_fee_processing':
    case 'merchant_report_processing':
    case 'applet_auth_pending':
      return 'processing'
    default:
      return 'processing'
  }
}

function buildResult(account: BaofuSettlementAccountResponse, status = mapAccountStatus(account.status)): BaofuOnboardingWorkflowResult {
  return {
    status,
    account,
    shouldRefreshWorkbench: status === 'ready' || status === 'failed' || status === 'profile_pending' || status === 'closed'
  }
}

async function startOrResumeBaofuSettlementAccount(role: BaofuAccountOwnerRole): Promise<BaofuSettlementAccountResponse> {
  return submitBaofuSettlementAccountProfile(role, { profile: {} })
}

export function startOrResumeMerchantSettlementAccount(
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  return startOrResumeBaofuAccountOnboarding('merchant', options)
}

export function startOrResumeRiderSettlementAccount(
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  return startOrResumeBaofuAccountOnboarding('rider', options)
}

export function startOrResumeOperatorSettlementAccount(
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  return startOrResumeBaofuAccountOnboarding('operator', options)
}

export async function pollBaofuSettlementAccountStatus(
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  const role = options.role || 'rider'
  const maxAttempts = options.maxAttempts ?? BAOFU_STATUS_POLL_MAX_ATTEMPTS
  const interval = options.interval ?? PAYMENT_STATUS_POLL_INTERVAL_MS
  const context = options.context

  showProgressToast(context, options.loadingMessage || '开户状态同步中...')

  try {
    for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
      const account = await getBaofuSettlementAccount(role)
      if (isBaofuSettlementAfterPaymentTerminalStatus(account.status)) {
        return buildResult(account)
      }

      if (attempt < maxAttempts - 1) {
        await delay(interval)
      }
    }

    const account = await getBaofuSettlementAccount(role)
    const terminalStatus = isBaofuSettlementAfterPaymentTerminalStatus(account.status)
      ? mapAccountStatus(account.status)
      : 'pending_confirmation'
    return buildResult(account, terminalStatus)
  } catch (error: unknown) {
    logAndThrowWorkflowError('poll_status', role, error)
  } finally {
    hideProgressToast(context)
  }
}

async function completePaymentThenPoll(
  account: BaofuSettlementAccountResponse,
  options: WorkflowOptions
): Promise<BaofuOnboardingWorkflowResult> {
  const context = options.context
  const payment = getBaofuAccountPayment(account)
  const normalizedAccountStatus = String(account.status || '').trim().toLowerCase()
  const role = options.role || 'rider'
  const pendingWorkflowContext = buildPendingWorkflowContext(role, payment, payment?.amount || account.verify_fee_amount || 200)

  if (!payment?.payment_order_id || !payment.pay_params) {
    if (normalizedAccountStatus === 'failed' || normalizedAccountStatus === 'ready' || normalizedAccountStatus === 'profile_pending') {
      return buildResult(account)
    }

    if (isBaofuSettlementPaymentRequiredStatus(normalizedAccountStatus)) {
      return buildResult(account)
    }

    return buildResult(account, 'pay_params_missing')
  }

  if (pendingWorkflowContext) {
    savePendingWorkflowContext(pendingWorkflowContext)
  }

  const paymentResult = await completePaymentWorkflow({
    id: payment.payment_order_id,
    user_id: 0,
    order_id: 0,
    out_trade_no: payment.out_trade_no || '',
    amount: payment.amount || 200,
    status: 'pending',
    payment_type: 'miniprogram',
    business_type: 'baofu_account_verify_fee',
    pay_params: payment.pay_params,
    created_at: ''
  } as PaymentOrderResponse, {
    maxAttempts: options.maxAttempts,
    interval: options.interval,
    context,
    paymentMessage: '正在调起微信支付...',
    confirmingMessage: '支付结果确认中...'
  })

  if (paymentResult.status === 'cancelled') {
    const refreshed = await getBaofuSettlementAccount(role)
    return buildResult(refreshed, isBaofuSettlementPaymentRequiredStatus(refreshed.status) ? 'verify_fee_pending' : mapAccountStatus(refreshed.status))
  }

  if (paymentResult.status === 'pay_params_missing') {
    if (pendingWorkflowContext) {
      savePendingWorkflowContext(pendingWorkflowContext)
    }
    return buildResult(account, 'pay_params_missing')
  }

  if (paymentResult.status === 'closed') {
    clearPendingWorkflowContext(role)
    return buildResult(account, 'closed')
  }

  if (paymentResult.status === 'failed') {
    clearPendingWorkflowContext(role)
    return buildResult(account, 'failed')
  }

  if (paymentResult.status === 'pending_confirmation') {
    if (pendingWorkflowContext) {
      savePendingWorkflowContext(pendingWorkflowContext)
    }
    return buildResult(account, 'pending_confirmation')
  }

  if (paymentResult.status !== 'paid') {
    if (pendingWorkflowContext) {
      savePendingWorkflowContext(pendingWorkflowContext)
    }
    return buildResult(account, 'pending_confirmation')
  }

  await startOrResumeBaofuSettlementAccount(role)
  clearPendingWorkflowContext(role)

  return pollBaofuSettlementAccountStatus({
    ...options,
    role,
    loadingMessage: '支付结果确认中...'
  })
}

async function startOrResumeBaofuAccountOnboarding(
  role: BaofuAccountOwnerRole,
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  const context = options.context
  const pendingWorkflowContext = loadPendingWorkflowContext(role)

  try {
    showProgressToast(context, options.loadingMessage || '正在恢复开户进度...')
    if (pendingWorkflowContext) {
      const account = await getBaofuSettlementAccount(role)
      const payment = getBaofuAccountPayment(account)

      if (!payment?.pay_params) {
        if (isBaofuSettlementTerminalStatus(account.status) || isBaofuSettlementPaymentRequiredStatus(account.status)) {
          clearPendingWorkflowContext(role)
          return buildResult(account)
        }
        if (isBaofuSettlementOpeningProcessingStatus(account.status)) {
          return pollBaofuSettlementAccountStatus({
            ...options,
            role,
            loadingMessage: '开户状态同步中...'
          })
        }
      } else {
        return completePaymentThenPoll(account, { ...options, role })
      }
    }

    const account = await startOrResumeBaofuSettlementAccount(role)
    if (getBaofuAccountPayment(account)?.pay_params) {
      return completePaymentThenPoll(account, { ...options, role })
    }
    if (isBaofuSettlementOpeningProcessingStatus(account.status)) {
      return pollBaofuSettlementAccountStatus({ ...options, role })
    }
    return buildResult(account)
  } catch (error: unknown) {
    logAndThrowWorkflowError('start_or_resume', role, error)
  } finally {
    hideProgressToast(context)
  }
}

export async function startBaofuAccountOnboarding(
  profile: BaofuAccountProfile,
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  const role = options.role || 'rider'
  const context = options.context

  try {
    showProgressToast(context, options.loadingMessage || '正在提交开户资料...')
    const account = await submitBaofuSettlementAccountProfile(role, { profile })
    return completePaymentThenPoll(account, { ...options, role })
  } catch (error: unknown) {
    logAndThrowWorkflowError('submit_profile', role, error, '开户资料提交失败，请稍后重试')
  } finally {
    hideProgressToast(context)
  }
}

export async function submitBaofuAccountProfile(
  profile: BaofuAccountProfile,
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  return startBaofuAccountOnboarding(profile, options)
}

export async function continueBaofuAccountPayment(
  options: WorkflowOptions = {}
): Promise<BaofuOnboardingWorkflowResult> {
  const role = options.role || 'rider'
  const context = options.context

  try {
    showProgressToast(context, options.loadingMessage || '正在恢复支付进度...')
    const account = await getBaofuSettlementAccount(role)
    const payment = getBaofuAccountPayment(account)

    if (payment?.pay_params) {
      const pendingWorkflowContext = buildPendingWorkflowContext(role, payment, payment.amount || account.verify_fee_amount || 200)
      if (pendingWorkflowContext) {
        savePendingWorkflowContext(pendingWorkflowContext)
      }
      return completePaymentThenPoll(account, { ...options, role })
    }

    if (isBaofuSettlementTerminalStatus(account.status) || isBaofuSettlementPaymentRequiredStatus(account.status)) {
      clearPendingWorkflowContext(role)
      return buildResult(account)
    }

    if (isBaofuSettlementOpeningProcessingStatus(account.status)) {
      return pollBaofuSettlementAccountStatus({ ...options, role })
    }

    return buildResult(account, 'pay_params_missing')
  } catch (error: unknown) {
    logAndThrowWorkflowError('continue_payment', role, error, '支付进度恢复失败，请稍后重试')
  } finally {
    hideProgressToast(context)
  }
}

export function getBaofuOnboardingFeedbackMessage(result: BaofuOnboardingWorkflowResult): string {
  switch (result.status) {
    case 'ready':
      return '结算账户已开通'
    case 'failed':
      return result.account.status_desc ||
        getBaofuAccountNextActionText(result.account.status, result.account.verify_fee_amount) ||
        '开户未通过，请核对资料后重试；如持续失败请联系平台处理'
    case 'profile_pending':
      return '请补全开户资料'
    case 'verify_fee_pending':
      return '核验费待支付，可继续支付'
    case 'closed':
      return '支付已关闭，请重新发起支付'
    case 'cancelled':
      return '支付未完成，可稍后继续'
    case 'pay_params_missing':
      return '支付信息暂未就绪，请刷新后重试'
    case 'pending_confirmation':
      return '支付结果仍在同步，请稍后刷新'
    default:
      return result.account.status_desc ||
        getBaofuAccountNextActionText(result.account.status, result.account.verify_fee_amount) ||
        getBaofuAccountStatusText(result.account.status) ||
        '开户进度正在同步'
  }
}

export function getBaofuOnboardingFeedbackTheme(result: BaofuOnboardingWorkflowResult): 'success' | 'warning' | 'error' {
  switch (result.status) {
    case 'ready':
      return 'success'
    case 'failed':
      return 'error'
    default:
      return 'warning'
  }
}

export function shouldClearPendingBaofuAccountOnboardingContext(result: BaofuOnboardingWorkflowResult): boolean {
  switch (result.status) {
    case 'ready':
    case 'failed':
    case 'profile_pending':
    case 'closed':
      return true
    default:
      return false
  }
}

export function buildPendingBaofuAccountOnboardingContext(
  role: BaofuAccountOwnerRole,
  account: BaofuSettlementAccountResponse
): PendingWorkflowContext | null {
  const payment = getBaofuAccountPayment(account)
  return buildPendingWorkflowContext(role, payment, payment?.amount || account.verify_fee_amount || 200)
}
