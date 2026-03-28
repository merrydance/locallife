import { request } from '../utils/request'

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
  const created = await createOCRJob(data)
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
    throw new Error(latestJob.error_message || 'OCR 识别失败，请稍后重试')
  }

  return fetchLatest()
}