import { request } from '../utils/request'
import { AppError, ErrorType } from '../utils/error-handler'

const OCR_ENQUEUE_RETRY_DELAY_MS = 1500
const OCR_ENQUEUE_MAX_ATTEMPTS = 3
const IMAGE_MODERATION_PENDING_MARKERS = ['image moderation is pending', '内容审核中']
const OCR_MODERATION_PENDING_MESSAGE = '图片已上传，系统处理中'

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
    return '图片识别失败，请重新上传清晰图片'
  }

  if (/[\u4e00-\u9fff]/.test(errorMessage)) {
    return errorMessage
  }

  const normalized = errorMessage.toLowerCase()
  if (
    normalized.includes('timeout') ||
    normalized.includes('timed out')
  ) {
    return '系统识别时间较长，请稍后查看结果'
  }

  if (
    normalized.includes('moderation') ||
    normalized.includes('rejected') ||
    normalized.includes('blocked')
  ) {
    return '图片审核未通过，请更换图片后重试'
  }

  return '图片识别失败，请重新上传清晰图片'
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
          userMessage: OCR_MODERATION_PENDING_MESSAGE
        }, error)
      }

      await delay(OCR_ENQUEUE_RETRY_DELAY_MS)
    }
  }

  throw new AppError({
    type: ErrorType.BUSINESS,
    message: 'OCR enqueue blocked by media moderation',
    userMessage: OCR_MODERATION_PENDING_MESSAGE
  })
}

export function getOCRJob(jobId: number) {
  return request<OCRJobStatusResponse>({
    url: `/v1/ocr/jobs/${jobId}`,
    method: 'GET'
  })
}

export async function enqueueOCRJobAndRefresh<T>(
  data: CreateOCRJobInput,
  fetchLatest: () => Promise<T>,
  options?: { maxAttempts?: number, intervalMs?: number }
) {
  const created = await createOCRJobWithRetry(data)
  const maxAttempts = options?.maxAttempts ?? 12
  const intervalMs = options?.intervalMs ?? 1000

  let latestJob = created
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    latestJob = await getOCRJob(created.ocr_job_id)
    if (TERMINAL_STATUSES.has(latestJob.status)) {
      break
    }
    if (attempt < maxAttempts - 1) {
      await delay(intervalMs)
    }
  }

  if (latestJob.status === 'failed' || latestJob.status === 'cancelled') {
    throw new AppError({
      type: ErrorType.BUSINESS,
      message: latestJob.error_message || 'OCR job failed',
      userMessage: normalizeOCRJobFailureMessage(latestJob.error_message)
    })
  }

  return fetchLatest()
}