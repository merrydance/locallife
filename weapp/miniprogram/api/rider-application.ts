import { request } from '../utils/request'
import { uploadMedia } from '../utils/media'
import { enqueueOCRJobAndRefresh } from './ocr-jobs'
import { ApplicationStatus } from './onboarding'
import type { AgreementConsentPayload } from './agreement-consent'

export interface RiderApplicationResponse {
  id: number
  user_id: number
  real_name?: string
  phone?: string
  id_card_front_asset_id?: number
  id_card_back_asset_id?: number
  id_card_ocr?: {
    status?: 'pending' | 'processing' | 'done' | 'failed'
    error?: string
    name?: string
    id_number?: string
    gender?: string
    nation?: string
    address?: string
    valid_start?: string
    valid_end?: string
    ocr_at?: string
  }
  health_cert_asset_id?: number
  health_cert_ocr?: {
    status?: 'pending' | 'processing' | 'done' | 'failed'
    error?: string
    name?: string
    id_number?: string
    cert_number?: string
    valid_start?: string
    valid_end?: string
    ocr_at?: string
  }
  status: ApplicationStatus
  reject_reason?: string
  created_at: string
  updated_at?: string
  submitted_at?: string
}

function hasRiderText(value?: string) {
  return typeof value === 'string' && value.trim().length > 0
}

function checkRiderIDCardWriteback(latest: RiderApplicationResponse, side: 'Front' | 'Back') {
  const status = latest.id_card_ocr?.status || ''
  const error = latest.id_card_ocr?.error || ''

  if (side === 'Front') {
    return {
      ready: status === 'done' || hasRiderText(latest.id_card_ocr?.name) || hasRiderText(latest.id_card_ocr?.id_number),
      failed: status === 'failed',
      errorMessage: error
    }
  }

  return {
    ready: status === 'done' || hasRiderText(latest.id_card_ocr?.valid_end),
    failed: status === 'failed',
    errorMessage: error
  }
}

function checkRiderHealthCertWriteback(latest: RiderApplicationResponse) {
  const status = latest.health_cert_ocr?.status || ''
  const error = latest.health_cert_ocr?.error || ''
  return {
    ready: status === 'done'
      || hasRiderText(latest.health_cert_ocr?.cert_number)
      || hasRiderText(latest.health_cert_ocr?.valid_end)
      || hasRiderText(latest.health_cert_ocr?.name),
    failed: status === 'failed',
    errorMessage: error
  }
}

export interface UpdateRiderBasicRequest {
  real_name?: string
  phone?: string
}

/**
 * 获取或创建骑手入驻草稿
 */
export function getOrCreateRiderApplication() {
  return request<RiderApplicationResponse>({
    url: '/v1/rider/application',
    method: 'GET'
  })
}

/**
 * 更新骑手基础信息
 */
export function updateRiderApplicationBasic(data: UpdateRiderBasicRequest) {
  return request<RiderApplicationResponse>({
    url: '/v1/rider/application/basic',
    method: 'PUT',
    data
  })
}

/**
 * 上传身份证并通过统一 OCR job 识别
 */
export async function ocrRiderIdCard(filePath: string, side: 'Front' | 'Back') {
  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory
  })
  const draft = await getOrCreateRiderApplication()
  return enqueueOCRJobAndRefresh(
    {
      document_type: 'id_card',
      media_asset_id: mediaId,
      owner_type: 'rider_application',
      owner_id: draft.id,
      side: side === 'Front' ? 'front' : 'back'
    },
    getOrCreateRiderApplication,
    {
      verifyResult: (latest) => checkRiderIDCardWriteback(latest, side),
      maxAttempts: 20,
      intervalMs: 1000
    }
  )
}

/**
 * 上传健康证并通过统一 OCR job 识别
 */
export async function ocrRiderHealthCert(filePath: string) {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory: 'health_cert'
  })
  const draft = await getOrCreateRiderApplication()
  return enqueueOCRJobAndRefresh(
    {
      document_type: 'health_cert',
      media_asset_id: mediaId,
      owner_type: 'rider_application',
      owner_id: draft.id
    },
    getOrCreateRiderApplication,
    {
      verifyResult: checkRiderHealthCertWriteback,
      maxAttempts: 20,
      intervalMs: 1000
    }
  )
}

/**
 * 提交骑手入驻申请
 */
export function submitRiderApplication(data?: AgreementConsentPayload) {
  return request<RiderApplicationResponse>({
    url: '/v1/rider/application/submit',
    method: 'POST',
    data
  })
}

/**
 * 重置被拒绝的申请为草稿
 */
export function resetRiderApplication() {
  return request<RiderApplicationResponse>({
    url: '/v1/rider/application/reset',
    method: 'POST'
  })
}

export function deleteRiderApplicationHealthCert() {
  return request<RiderApplicationResponse>({
    url: '/v1/rider/application/health-cert',
    method: 'DELETE'
  })
}

export function deleteRiderApplicationDocument(
  documentType: 'id_card_front' | 'id_card_back' | 'health_cert'
) {
  return request<RiderApplicationResponse>({
    url: `/v1/rider/application/documents/${documentType}`,
    method: 'DELETE'
  })
}