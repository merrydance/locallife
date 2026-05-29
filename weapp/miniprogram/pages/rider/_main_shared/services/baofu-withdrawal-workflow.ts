import type {
  BaofuWithdrawalBalanceResponse,
  BaofuWithdrawalItem,
  BaofuWithdrawalStatus
} from '../api/baofu-withdrawal'

export type BaofuWithdrawalStatusTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export interface YuanInputParseResult {
  amount: number
  errorMessage: string
}

export interface BaofuWithdrawalStatusView {
  text: string
  theme: BaofuWithdrawalStatusTheme
  syncState: string
  syncMessage: string
  isProcessing: boolean
  isSucceeded: boolean
  isFailed: boolean
  isReturned: boolean
  isTerminal: boolean
}

export interface BaofuWithdrawalBalanceView {
  availableAmount: number
  pendingAmount: number
  ledgerAmount: number
  frozenAmount: number
  minWithdrawAmount: number
  maxWithdrawAmount: number
  availableAmountText: string
  pendingAmountText: string
  ledgerAmountText: string
  frozenAmountText: string
  minWithdrawAmountText: string
  maxWithdrawAmountText: string
  canSubmit: boolean
  disabledReason: string
  statusDesc: string
}

export interface BaofuWithdrawalSubmitCheck {
  amount: number
  canSubmit: boolean
  errorMessage: string
}

export interface BaofuWithdrawalItemView extends BaofuWithdrawalItem {
  amountText: string
  statusView: BaofuWithdrawalStatusView
  createdAtText: string
  updatedAtText: string
}

export interface BaofuWithdrawalLoadedSummaryView {
  loadedCountText: string
  loadedAmountText: string
  succeededAmountText: string
  processingAmountText: string
  failedAmountText: string
  returnedAmountText: string
}

export type BaofuWithdrawalSettledResult<T> =
  | { status: 'fulfilled', value: T }
  | { status: 'rejected', reason: unknown }

export type BaofuWithdrawalFulfilledResult<T> = { status: 'fulfilled', value: T }
export type BaofuWithdrawalRejectedResult = { status: 'rejected', reason: unknown }

export function formatFenToYuanText(amount?: number): string {
  const normalized = Number.isFinite(amount) ? Math.max(Number(amount), 0) : 0
  return `¥${(normalized / 100).toFixed(2)}`
}

export function parseYuanInputToFen(input: string): YuanInputParseResult {
  const value = String(input || '').trim()
  if (!value) {
    return { amount: 0, errorMessage: '请输入提现金额' }
  }
  if (!/^\d+(\.\d{0,2})?$/.test(value)) {
    if (/^\d+\.\d{3,}$/.test(value)) {
      return { amount: 0, errorMessage: '金额最多保留两位小数' }
    }
    return { amount: 0, errorMessage: '请输入有效金额' }
  }

  const amount = Math.round(Number(value) * 100)
  if (!Number.isFinite(amount) || amount <= 0) {
    return { amount: 0, errorMessage: '请输入有效金额' }
  }

  return { amount, errorMessage: '' }
}

export function buildBaofuWithdrawalStatusView(
  status?: BaofuWithdrawalStatus,
  syncState?: string,
  syncMessage?: string
): BaofuWithdrawalStatusView {
  const normalized = String(status || '').trim()
  const state = String(syncState || normalized || 'unknown').trim()

  switch (normalized) {
    case 'processing':
      return {
        text: '提现处理中',
        theme: 'primary',
        syncState: state,
        syncMessage: syncMessage || '提现结果仍在确认中，请稍后刷新。',
        isProcessing: true,
        isSucceeded: false,
        isFailed: false,
        isReturned: false,
        isTerminal: false
      }
    case 'succeeded':
      return {
        text: '提现成功',
        theme: 'success',
        syncState: state,
        syncMessage: syncMessage || '提现已完成。',
        isProcessing: false,
        isSucceeded: true,
        isFailed: false,
        isReturned: false,
        isTerminal: true
      }
    case 'failed':
      return {
        text: '提现失败',
        theme: 'danger',
        syncState: state,
        syncMessage: syncMessage || '提现未完成，请刷新后再决定是否重新申请。',
        isProcessing: false,
        isSucceeded: false,
        isFailed: true,
        isReturned: false,
        isTerminal: true
      }
    case 'returned':
      return {
        text: '提现退票',
        theme: 'warning',
        syncState: state,
        syncMessage: syncMessage || '提现已退回，请以账户余额为准。',
        isProcessing: false,
        isSucceeded: false,
        isFailed: false,
        isReturned: true,
        isTerminal: true
      }
    default:
      return {
        text: '提现状态确认中',
        theme: 'default',
        syncState: state || 'unknown',
        syncMessage: syncMessage || '提现状态仍在同步，请稍后刷新。',
        isProcessing: false,
        isSucceeded: false,
        isFailed: false,
        isReturned: false,
        isTerminal: false
      }
  }
}

export function buildBaofuWithdrawalBalanceView(
  balance?: BaofuWithdrawalBalanceResponse | null
): BaofuWithdrawalBalanceView {
  const availableAmount = normalizeAmount(balance?.available_amount)
  const pendingAmount = normalizeAmount(balance?.pending_amount)
  const ledgerAmount = normalizeAmount(balance?.ledger_amount)
  const frozenAmount = normalizeAmount(balance?.frozen_amount)
  const minWithdrawAmount = normalizeAmount(balance?.min_withdraw_amount || 100)
  const maxWithdrawAmount = normalizeAmount(balance?.max_withdraw_amount || 500000000)
  const canSubmit = Boolean(balance?.can_withdraw) && availableAmount >= minWithdrawAmount
  const disabledReason = String(balance?.disabled_reason || '').trim() ||
    (canSubmit ? '' : '可提现金额不足')

  return {
    availableAmount,
    pendingAmount,
    ledgerAmount,
    frozenAmount,
    minWithdrawAmount,
    maxWithdrawAmount,
    availableAmountText: formatFenToYuanText(availableAmount),
    pendingAmountText: formatFenToYuanText(pendingAmount),
    ledgerAmountText: formatFenToYuanText(ledgerAmount),
    frozenAmountText: formatFenToYuanText(frozenAmount),
    minWithdrawAmountText: formatFenToYuanText(minWithdrawAmount),
    maxWithdrawAmountText: formatFenToYuanText(maxWithdrawAmount),
    canSubmit,
    disabledReason,
    statusDesc: String(balance?.status_desc || '').trim()
  }
}

export function withdrawalBalanceUnavailableView(message?: string): BaofuWithdrawalBalanceView {
  const fallback = String(message || '').trim() || '可提现余额暂不可确认'
  const minWithdrawAmount = 100
  const maxWithdrawAmount = 500000000
  return {
    availableAmount: 0,
    pendingAmount: 0,
    ledgerAmount: 0,
    frozenAmount: 0,
    minWithdrawAmount,
    maxWithdrawAmount,
    availableAmountText: '--',
    pendingAmountText: '--',
    ledgerAmountText: '--',
    frozenAmountText: '--',
    minWithdrawAmountText: formatFenToYuanText(minWithdrawAmount),
    maxWithdrawAmountText: formatFenToYuanText(maxWithdrawAmount),
    canSubmit: false,
    disabledReason: fallback,
    statusDesc: '余额暂不可确认'
  }
}

export function settleBaofuWithdrawalRequest<T>(
  request: Promise<T>
): Promise<BaofuWithdrawalSettledResult<T>> {
  return request.then(
    (value) => ({ status: 'fulfilled', value }),
    (reason) => ({ status: 'rejected', reason })
  )
}

export function isBaofuWithdrawalRequestFulfilled<T>(
  result: BaofuWithdrawalSettledResult<T>
): result is BaofuWithdrawalFulfilledResult<T> {
  return result.status === 'fulfilled'
}

export function isBaofuWithdrawalRequestRejected<T>(
  result: BaofuWithdrawalSettledResult<T>
): result is BaofuWithdrawalRejectedResult {
  return result.status === 'rejected'
}

export function buildBaofuWithdrawalSubmitCheck(
  input: string,
  balanceView: BaofuWithdrawalBalanceView
): BaofuWithdrawalSubmitCheck {
  const parsed = parseYuanInputToFen(input)
  if (parsed.errorMessage) {
    return { amount: 0, canSubmit: false, errorMessage: parsed.errorMessage }
  }
  if (parsed.amount < balanceView.minWithdrawAmount) {
    return {
      amount: parsed.amount,
      canSubmit: false,
      errorMessage: `提现金额至少 ${balanceView.minWithdrawAmountText}`
    }
  }
  if (parsed.amount > balanceView.maxWithdrawAmount) {
    return {
      amount: parsed.amount,
      canSubmit: false,
      errorMessage: `提现金额最多 ${balanceView.maxWithdrawAmountText}`
    }
  }
  if (parsed.amount > balanceView.availableAmount) {
    return { amount: parsed.amount, canSubmit: false, errorMessage: '超过可提现余额' }
  }
  if (!balanceView.canSubmit) {
    return {
      amount: parsed.amount,
      canSubmit: false,
      errorMessage: balanceView.disabledReason || '当前暂不能提现'
    }
  }
  return { amount: parsed.amount, canSubmit: true, errorMessage: '' }
}

export function buildBaofuWithdrawalItemView(item: BaofuWithdrawalItem): BaofuWithdrawalItemView {
  return {
    ...item,
    amountText: formatFenToYuanText(item.amount),
    statusView: buildBaofuWithdrawalStatusView(item.status, item.sync_state, item.sync_message),
    createdAtText: formatDateTimeText(item.created_at),
    updatedAtText: formatDateTimeText(item.updated_at)
  }
}

export function buildBaofuWithdrawalLoadedSummaryView(
  rows: BaofuWithdrawalItemView[]
): BaofuWithdrawalLoadedSummaryView {
  const totalAmount = rows.reduce((total, row) => total + normalizeAmount(row.amount), 0)
  const succeededAmount = rows
    .filter((row) => row.statusView.isSucceeded)
    .reduce((total, row) => total + normalizeAmount(row.amount), 0)
  const processingAmount = rows
    .filter((row) => row.statusView.isProcessing)
    .reduce((total, row) => total + normalizeAmount(row.amount), 0)
  const failedAmount = rows
    .filter((row) => row.statusView.isFailed)
    .reduce((total, row) => total + normalizeAmount(row.amount), 0)
  const returnedAmount = rows
    .filter((row) => row.statusView.isReturned)
    .reduce((total, row) => total + normalizeAmount(row.amount), 0)

  return {
    loadedCountText: `${rows.length} 笔`,
    loadedAmountText: formatFenToYuanText(totalAmount),
    succeededAmountText: formatFenToYuanText(succeededAmount),
    processingAmountText: formatFenToYuanText(processingAmount),
    failedAmountText: formatFenToYuanText(failedAmount),
    returnedAmountText: formatFenToYuanText(returnedAmount)
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

function normalizeAmount(value?: number): number {
  return Number.isFinite(value) ? Math.max(Number(value), 0) : 0
}
