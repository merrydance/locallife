import { getRefundById, isRefundStatusTerminal, RefundOrder } from '../api/payment'
import { logger } from '../../../../../utils/logger'

export interface RefundTerminalWaitOptions {
  maxAttempts?: number
  initialIntervalMs?: number
  maxIntervalMs?: number
  backoffFactor?: number
  shouldContinue?: () => boolean
  onAttempt?: (refund: RefundOrder, attempt: number) => void
}

export interface RefundTerminalWaitResult {
  refund: RefundOrder
  terminal: boolean
  attempts: number
}

const DEFAULT_MAX_ATTEMPTS = 8
const DEFAULT_INITIAL_INTERVAL_MS = 1000
const DEFAULT_MAX_INTERVAL_MS = 8000
const DEFAULT_BACKOFF_FACTOR = 2

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function waitForRefundTerminalResult(
  refundId: number,
  options: RefundTerminalWaitOptions = {}
): Promise<RefundTerminalWaitResult> {
  const maxAttempts = options.maxAttempts ?? DEFAULT_MAX_ATTEMPTS
  const maxIntervalMs = options.maxIntervalMs ?? DEFAULT_MAX_INTERVAL_MS
  const backoffFactor = options.backoffFactor ?? DEFAULT_BACKOFF_FACTOR
  let nextDelayMs = options.initialIntervalMs ?? DEFAULT_INITIAL_INTERVAL_MS
  let lastRefund: RefundOrder | null = null

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    if (options.shouldContinue && !options.shouldContinue()) {
      throw new Error('退款结果确认已取消')
    }

    const refund = await getRefundById(refundId)
    lastRefund = refund
    options.onAttempt?.(refund, attempt)

    if (isRefundStatusTerminal(refund.status)) {
      return {
        refund,
        terminal: true,
        attempts: attempt
      }
    }

    if (attempt < maxAttempts) {
      await delay(nextDelayMs)
      nextDelayMs = Math.min(maxIntervalMs, Math.ceil(nextDelayMs * backoffFactor))
    }
  }

  if (!lastRefund) {
    logger.warn('Refund terminal wait ended without a refund payload', { refundId }, 'refund-workflow')
    lastRefund = await getRefundById(refundId)
  }

  return {
    refund: lastRefund,
    terminal: isRefundStatusTerminal(lastRefund.status),
    attempts: maxAttempts
  }
}
