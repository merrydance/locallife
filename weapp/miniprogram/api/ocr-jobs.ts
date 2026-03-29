import { request } from '../utils/request'
import { AppError, ErrorType } from '../utils/error-handler'

const OCR_ENQUEUE_RETRY_DELAY_MS = 1500
const OCR_ENQUEUE_MAX_ATTEMPTS = 3
const IMAGE_MODERATION_PENDING_MARKERS = [
  'image moderation is pending',
  '内容审核中',
  '多媒体内容安全审查',
  '安全审查系统拦截'
]
const OCR_BLOCKED_MESSAGE = '图片被微信多媒体内容安全审查系统拦截，请更换图片再试'
const OCR_FAILURE_MESSAGE = '识别失败，请提供更清晰更规整的图片重试'
const OCR_TIMEOUT_MESSAGE = '识别超时，请稍后重试'

export interface CreateOCRJobInput {
  document_type: 'business_license' | 'food_permit' | 'id_card' | 'health_cert'
  media_asset_id: number
  owner_type: 'merchant_application' | 'operator_application' | 'rider_application' | 'group_application'
  owner_id: number
  side?: 'front' | 'back'
}

export interface OCRJobStatusResponse {
  ocr_job_id: number
  status: 'pending' | 'processing' | 'succeeded' | 'failed' | 'cancelled'
  error_code?: string
  error_message?: string
}

export interface OCRWritebackCheckResult {
  ready: boolean
  failed?: boolean
  errorMessage?: string
}

export interface OCRRefreshOptions<T> {
  maxAttempts?: number
  intervalMs?: number
  verifyResult?: (latest: T) => OCRWritebackCheckResult
  pendingMessage?: string
}

const TERMINAL_STATUSES = new Set<OCRJobStatusResponse['status']>(['succeeded', 'failed', 'cancelled'])

function delay(ms: number) {
  return new Promise<void>((resolve) => {
    setTimeout(() => resolve(), ms)
  })
}

export function createOCRJob(data: CreateOCRJobInput) {
  return request<OCRJobStatusResponse>({
    url: '/v1/ocr/jobs',
    method: 'POST',
    data
  })
}

function extractErrorMessage(error: unknown) {
  if (!error || typeof error !== 'object') return ''

  const maybeError = error as {
    userMessage?: string
    message?: string
    originalError?: { message?: string }
  }

  return String(maybeError.userMessage || maybeError.message || maybeError.originalError?.message || '')
}

function isImageModerationPendingError(error: unknown) {
  const message = extractErrorMessage(error).toLowerCase()
  return IMAGE_MODERATION_PENDING_MARKERS.some((marker) => message.includes(marker))
}

function normalizeOCRJobFailureMessage(errorMessage?: string) {
  if (!errorMessage) {
    return OCR_FAILURE_MESSAGE
  }

  if (errorMessage.includes('多媒体内容安全审查') || errorMessage.includes('安全审查系统拦截')) {
    return OCR_BLOCKED_MESSAGE
  }

  if (errorMessage.includes('超时')) {
    return OCR_TIMEOUT_MESSAGE
  }

  if (/[\u4e00-\u9fff]/.test(errorMessage)) {
    return errorMessage
  }

  const normalized = errorMessage.toLowerCase()
  if (
    normalized.includes('timeout') ||
    normalized.includes('timed out')
  ) {
    return OCR_TIMEOUT_MESSAGE
  }

  if (
    normalized.includes('moderation') ||
    normalized.includes('rejected') ||
    normalized.includes('blocked')
  ) {
    return OCR_BLOCKED_MESSAGE
  }

  return OCR_FAILURE_MESSAGE
}

function buildOCRPendingError(userMessage?: string) {
  return new AppError({
    type: ErrorType.BUSINESS,
    message: 'OCR result not ready yet',
    userMessage: userMessage || OCR_TIMEOUT_MESSAGE
  })
}

async function createOCRJobWithRetry(data: CreateOCRJobInput) {
  for (let attempt = 0; attempt < OCR_ENQUEUE_MAX_ATTEMPTS; attempt += 1) {
    try {
      return await createOCRJob(data)
    } catch (error) {
      if (!isImageModerationPendingError(error)) {
        throw error
      }

      if (attempt >= OCR_ENQUEUE_MAX_ATTEMPTS - 1) {
        throw new AppError({
          type: ErrorType.BUSINESS,
          message: 'OCR enqueue blocked by media moderation',
          userMessage: OCR_BLOCKED_MESSAGE
        }, error)
      }

      await delay(OCR_ENQUEUE_RETRY_DELAY_MS)
    }
  }

  throw new AppError({
    type: ErrorType.BUSINESS,
    message: 'OCR enqueue blocked by media moderation',
    userMessage: OCR_BLOCKED_MESSAGE
  })
}

export function getOCRJob(jobId: number) {
  return request<OCRJobStatusResponse>({
    url: `/v1/ocr/jobs/${jobId}`,
    method: 'GET'
  })
}

export async function waitForOCRJobTerminal(
  jobId: number,
  options?: { maxAttempts?: number, intervalMs?: number, pendingMessage?: string }
) {
  const maxAttempts = options?.maxAttempts ?? 12
  const intervalMs = options?.intervalMs ?? 1000

  let latestJob: OCRJobStatusResponse | null = null
  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    latestJob = await getOCRJob(jobId)
    if (TERMINAL_STATUSES.has(latestJob.status)) {
      return latestJob
    }
    if (attempt < maxAttempts - 1) {
      await delay(intervalMs)
    }
  }

  throw buildOCRPendingError(options?.pendingMessage)
}

export async function waitForOCRWriteback<T>(
  fetchLatest: () => Promise<T>,
  verifyResult: (latest: T) => OCRWritebackCheckResult,
  options?: { maxAttempts?: number, intervalMs?: number, pendingMessage?: string }
) {
  const maxAttempts = options?.maxAttempts ?? 15
  const intervalMs = options?.intervalMs ?? 1000

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    const latest = await fetchLatest()
    const check = verifyResult(latest)

    if (check.failed) {
      throw new AppError({
        type: ErrorType.BUSINESS,
        message: check.errorMessage || 'OCR writeback failed',
        userMessage: normalizeOCRJobFailureMessage(check.errorMessage)
      })
    }

    if (check.ready) {
      return latest
    }

    if (attempt < maxAttempts - 1) {
      await delay(intervalMs)
    }
  }

  throw buildOCRPendingError(options?.pendingMessage)
}

export async function enqueueOCRJobAndRefresh<T>(
  data: CreateOCRJobInput,
  fetchLatest: () => Promise<T>,
  options?: OCRRefreshOptions<T>
) {
  const created = await createOCRJobWithRetry(data)
  const latestJob = await waitForOCRJobTerminal(created.ocr_job_id, {
    maxAttempts: options?.maxAttempts,
    intervalMs: options?.intervalMs,
    pendingMessage: options?.pendingMessage
  })

  if (latestJob.status === 'failed' || latestJob.status === 'cancelled') {
    throw new AppError({
      type: ErrorType.BUSINESS,
      message: latestJob.error_message || 'OCR job failed',
      userMessage: normalizeOCRJobFailureMessage(latestJob.error_message)
    })
  }

  if (!options?.verifyResult) {
    return fetchLatest()
  }

  return waitForOCRWriteback(fetchLatest, options.verifyResult, {
    maxAttempts: options?.maxAttempts,
    intervalMs: options?.intervalMs,
    pendingMessage: options?.pendingMessage
  })
}