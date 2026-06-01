import RiderService, {
  type RiderWithdrawResponse,
  type RiderWithdrawalStatusResponse
} from '../_main_shared/api/rider'

const STORAGE_KEY = 'riderDepositPendingWithdrawal'
const TERMINAL_WAIT_MAX_ATTEMPTS = 45
const TERMINAL_WAIT_INTERVAL_MS = 2000

export interface RiderDepositPendingWithdrawalContext {
  refundOrderIds: number[]
  acceptedAmount: number
  updatedAt: string
}

export interface RiderDepositWithdrawalStatusView {
  title: string
  description: string
  amountDisplay: string
  statusText: string
  tagTheme: 'default' | 'primary' | 'warning' | 'danger' | 'success'
  panelTheme: 'success' | 'warning' | 'error'
  feedbackMessage: string
  feedbackTheme: 'success' | 'warning'
  isTerminal: boolean
  shouldRefreshFinance: boolean
  canRefresh: boolean
}

export interface RiderDepositWithdrawalRecoveryResult {
  view: RiderDepositWithdrawalStatusView
  isTerminal: boolean
  shouldRefreshFinance: boolean
}

export interface RiderDepositWithdrawalTerminalWaitResult extends RiderDepositWithdrawalRecoveryResult {
  timedOut: boolean
}

export interface RiderDepositWithdrawalTerminalWaitOptions {
  maxAttempts?: number
  intervalMs?: number
}

export interface RecoverStoredRiderDepositWithdrawalOptions extends RiderDepositWithdrawalTerminalWaitOptions {
  waitForTerminal?: boolean
}

export interface RiderDepositWithdrawalStatusData {
  hasPendingWithdrawal: boolean
  pendingWithdrawalTitle: string
  pendingWithdrawalDescription: string
  pendingWithdrawalAmountDisplay: string
  pendingWithdrawalStatusText: string
  pendingWithdrawalTagTheme: 'default' | 'primary' | 'warning' | 'danger' | 'success'
  pendingWithdrawalPanelTheme: 'success' | 'warning' | 'error'
  pendingWithdrawalCanRefresh: boolean
}

function formatFenToYuan(amount: number): string {
  return `¥${(Math.max(amount, 0) / 100).toFixed(2)}`
}

function uniquePositiveIds(ids: number[]): number[] {
  const seen = new Set<number>()
  const result: number[] = []

  ids.forEach((id) => {
    if (!Number.isFinite(id) || id <= 0 || seen.has(id)) {
      return
    }
    seen.add(id)
    result.push(id)
  })

  return result
}

function isValidPendingWithdrawalContext(value: unknown): value is RiderDepositPendingWithdrawalContext {
  if (!value || typeof value !== 'object') {
    return false
  }

  const candidate = value as Partial<RiderDepositPendingWithdrawalContext>
  return Array.isArray(candidate.refundOrderIds)
    && uniquePositiveIds(candidate.refundOrderIds).length > 0
    && Number.isFinite(candidate.acceptedAmount)
    && typeof candidate.updatedAt === 'string'
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export function buildPendingRiderDepositWithdrawalContext(
  response: RiderWithdrawResponse
): RiderDepositPendingWithdrawalContext | null {
  const refundOrderIds = uniquePositiveIds((response.refunds || []).map((refund) => refund.refund_order_id))
  if (refundOrderIds.length === 0 || response.status === 'success') {
    return null
  }

  const acceptedAmount = Number.isFinite(response.accepted_amount)
    ? response.accepted_amount
    : (response.refunds || []).reduce((total, refund) => total + (refund.amount || 0), 0)

  return {
    refundOrderIds,
    acceptedAmount,
    updatedAt: new Date().toISOString()
  }
}

export function savePendingRiderDepositWithdrawal(context: RiderDepositPendingWithdrawalContext) {
  wx.setStorageSync(STORAGE_KEY, context)
}

export function getPendingRiderDepositWithdrawal(): RiderDepositPendingWithdrawalContext | null {
  try {
    const stored = wx.getStorageSync(STORAGE_KEY) as unknown
    if (!isValidPendingWithdrawalContext(stored)) {
      return null
    }

    return {
      ...stored,
      refundOrderIds: uniquePositiveIds(stored.refundOrderIds)
    }
  } catch (_error) {
    return null
  }
}

export function clearPendingRiderDepositWithdrawal() {
  try {
    wx.removeStorageSync(STORAGE_KEY)
  } catch (_error) {
    return
  }
}

export async function recoverPendingRiderDepositWithdrawal(
  context: RiderDepositPendingWithdrawalContext
): Promise<RiderWithdrawalStatusResponse> {
  return RiderService.getWithdrawalStatus(context.refundOrderIds)
}

export async function recoverStoredRiderDepositWithdrawalStatus(
  options: RecoverStoredRiderDepositWithdrawalOptions = {}
): Promise<RiderDepositWithdrawalRecoveryResult | null> {
  const pendingWithdrawal = getPendingRiderDepositWithdrawal()
  if (!pendingWithdrawal) {
    return null
  }

  if (options.waitForTerminal) {
    return waitForRiderDepositWithdrawalTerminalStatus(pendingWithdrawal, options)
  }

  const result = await recoverPendingRiderDepositWithdrawal(pendingWithdrawal)
  const view = buildRiderDepositWithdrawalStatusView(result, pendingWithdrawal.acceptedAmount)
  if (view.isTerminal) {
    clearPendingRiderDepositWithdrawal()
  }

  return {
    view,
    isTerminal: view.isTerminal,
    shouldRefreshFinance: view.shouldRefreshFinance
  }
}

export async function waitForSubmittedRiderDepositWithdrawalTerminalStatus(
  response: RiderWithdrawResponse,
  options: RiderDepositWithdrawalTerminalWaitOptions = {}
): Promise<RiderDepositWithdrawalTerminalWaitResult | null> {
  const context = buildPendingRiderDepositWithdrawalContext(response)
  if (!context) {
    return null
  }

  savePendingRiderDepositWithdrawal(context)
  return waitForRiderDepositWithdrawalTerminalStatus(context, options)
}

export async function waitForRiderDepositWithdrawalTerminalStatus(
  context: RiderDepositPendingWithdrawalContext,
  options: RiderDepositWithdrawalTerminalWaitOptions = {}
): Promise<RiderDepositWithdrawalTerminalWaitResult> {
  const maxAttempts = Math.max(1, options.maxAttempts || TERMINAL_WAIT_MAX_ATTEMPTS)
  const intervalMs = Math.max(500, options.intervalMs || TERMINAL_WAIT_INTERVAL_MS)
  let latestView: RiderDepositWithdrawalStatusView | null = null

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    try {
      const result = await recoverPendingRiderDepositWithdrawal(context)
      latestView = buildRiderDepositWithdrawalStatusView(result, context.acceptedAmount)

      if (latestView.isTerminal) {
        clearPendingRiderDepositWithdrawal()
        return {
          view: latestView,
          isTerminal: true,
          shouldRefreshFinance: latestView.shouldRefreshFinance,
          timedOut: false
        }
      }
    } catch (_error: unknown) {
      latestView = latestView || buildRiderDepositWithdrawalSyncFailedView(context)
    }

    if (attempt < maxAttempts - 1) {
      await wait(intervalMs)
    }
  }

  if (latestView) {
    return {
      view: latestView,
      isTerminal: false,
      shouldRefreshFinance: latestView.shouldRefreshFinance,
      timedOut: true
    }
  }

  return {
    view: buildRiderDepositWithdrawalSyncFailedView(context),
    isTerminal: false,
    shouldRefreshFinance: false,
    timedOut: true
  }
}

export function buildRiderDepositWithdrawalStatusData(
  view: RiderDepositWithdrawalStatusView | null
): RiderDepositWithdrawalStatusData {
  if (!view) {
    return {
      hasPendingWithdrawal: false,
      pendingWithdrawalTitle: '',
      pendingWithdrawalDescription: '',
      pendingWithdrawalAmountDisplay: '',
      pendingWithdrawalStatusText: '',
      pendingWithdrawalTagTheme: 'warning',
      pendingWithdrawalPanelTheme: 'warning',
      pendingWithdrawalCanRefresh: false
    }
  }

  return {
    hasPendingWithdrawal: true,
    pendingWithdrawalTitle: view.title,
    pendingWithdrawalDescription: view.description,
    pendingWithdrawalAmountDisplay: view.amountDisplay,
    pendingWithdrawalStatusText: view.statusText,
    pendingWithdrawalTagTheme: view.tagTheme,
    pendingWithdrawalPanelTheme: view.panelTheme,
    pendingWithdrawalCanRefresh: view.canRefresh
  }
}

export function buildStoredRiderDepositWithdrawalSyncFailedView(): RiderDepositWithdrawalStatusView | null {
  const pendingWithdrawal = getPendingRiderDepositWithdrawal()
  if (!pendingWithdrawal) {
    return null
  }

  return buildRiderDepositWithdrawalSyncFailedView(pendingWithdrawal)
}

function buildRiderDepositWithdrawalSyncFailedView(
  pendingWithdrawal: RiderDepositPendingWithdrawalContext
): RiderDepositWithdrawalStatusView {

  return {
    title: '提现状态同步失败',
    description: '本次提现已经提交，但当前无法确认最新结果。系统会继续同步，也可稍后查看账单明细。',
    amountDisplay: formatFenToYuan(pendingWithdrawal.acceptedAmount),
    statusText: '同步失败',
    tagTheme: 'danger',
    panelTheme: 'error',
    feedbackMessage: '提现状态同步失败，系统会继续同步，也可稍后查看账单明细。',
    feedbackTheme: 'warning',
    isTerminal: false,
    shouldRefreshFinance: false,
    canRefresh: true
  }
}

export function buildRiderDepositWithdrawalStatusView(
  response: RiderWithdrawalStatusResponse,
  fallbackAmount: number = 0
): RiderDepositWithdrawalStatusView {
  const status = response.status || 'syncing'
  const acceptedAmount = response.accepted_amount || fallbackAmount
  const amountDisplay = formatFenToYuan(acceptedAmount)
  const backendMessage = response.message || ''
  const statusText = response.status_text || '同步中'

  switch (status) {
    case 'success':
      return {
        title: '提现已到账',
        description: backendMessage || '本次提现已完成，押金余额和账单记录已经同步更新。',
        amountDisplay,
        statusText,
        tagTheme: 'success',
        panelTheme: 'success',
        feedbackMessage: '提现已到账，押金余额和账单记录已经同步更新。',
        feedbackTheme: 'success',
        isTerminal: true,
        shouldRefreshFinance: true,
        canRefresh: false
      }
    case 'failed':
      return {
        title: '提现未完成',
        description: backendMessage || '本次提现未成功，资金会回到可用押金，请稍后查看账户后再决定是否重新申请。',
        amountDisplay,
        statusText,
        tagTheme: 'danger',
        panelTheme: 'error',
        feedbackMessage: '提现未完成，资金会回到可用押金，请稍后查看账户后再重新申请。',
        feedbackTheme: 'warning',
        isTerminal: true,
        shouldRefreshFinance: true,
        canRefresh: false
      }
    case 'partial_failed':
      return {
        title: '部分提现未完成',
        description: backendMessage || '部分退款单未成功，请以账单明细和可用押金为准，必要时稍后重新申请。',
        amountDisplay,
        statusText,
        tagTheme: 'warning',
        panelTheme: 'warning',
        feedbackMessage: '部分提现未完成，请以账单明细和可用押金为准。',
        feedbackTheme: 'warning',
        isTerminal: true,
        shouldRefreshFinance: true,
        canRefresh: false
      }
    case 'processing':
      return {
        title: '微信提现处理中',
        description: backendMessage || '微信正在处理本次退款，到账结果确认后会同步到账单。',
        amountDisplay,
        statusText,
        tagTheme: 'primary',
        panelTheme: 'warning',
        feedbackMessage: '提现正在处理中，系统会自动同步到账结果。',
        feedbackTheme: 'warning',
        isTerminal: false,
        shouldRefreshFinance: false,
        canRefresh: true
      }
    case 'accepted':
      return {
        title: '提现已受理',
        description: backendMessage || '提现请求已提交到微信退款通道，到账前请勿重复申请。',
        amountDisplay,
        statusText,
        tagTheme: 'warning',
        panelTheme: 'warning',
        feedbackMessage: '提现已受理，到账结果确认后会同步到账单。',
        feedbackTheme: 'warning',
        isTerminal: false,
        shouldRefreshFinance: false,
        canRefresh: true
      }
    default:
      return {
        title: '提现状态同步中',
        description: backendMessage || '当前结果还在同步，未确认前请以账单明细为准。',
        amountDisplay,
        statusText,
        tagTheme: 'default',
        panelTheme: 'warning',
        feedbackMessage: '提现状态仍在同步，系统会自动同步结果。',
        feedbackTheme: 'warning',
        isTerminal: false,
        shouldRefreshFinance: false,
        canRefresh: true
      }
  }
}

export function buildSubmittedRiderDepositWithdrawalView(
  response: RiderWithdrawResponse
): RiderDepositWithdrawalStatusView | null {
  const context = buildPendingRiderDepositWithdrawalContext(response)
  if (!context) {
    return null
  }

  return {
    title: '提现已受理',
    description: '提现请求已提交到微信退款通道，到账结果确认后会同步到账单。',
    amountDisplay: formatFenToYuan(context.acceptedAmount),
    statusText: '已受理',
    tagTheme: 'warning',
    panelTheme: 'warning',
    feedbackMessage: '提现已受理，到账结果确认后会同步到账单。',
    feedbackTheme: 'warning',
    isTerminal: false,
    shouldRefreshFinance: false,
    canRefresh: true
  }
}
