import { request } from '../../../../utils/request'
import { uploadMedia } from '../../../../utils/media'
import { enqueueOCRJobAndRefresh } from '../../_main_shared/api/ocr-jobs'
import type { ActiveCredentialSummary, ApplicationStatus, OnboardingReviewSummary } from '../../_main_shared/api/onboarding'
import type { AgreementConsentPayload } from '../../_main_shared/api/agreement-consent'
import { AppError, ErrorType } from '../../../../utils/error-handler'

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
    error_code?: string
    alert_emitted_at?: string
    queued_at?: string
    started_at?: string
    ocr_job_id?: number
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
    error_code?: string
    alert_emitted_at?: string
    queued_at?: string
    started_at?: string
    ocr_job_id?: number
    name?: string
    id_number?: string
    cert_number?: string
    valid_start?: string
    valid_end?: string
    ocr_at?: string
  }
  status: ApplicationStatus
  reject_reason?: string
  review_summary?: OnboardingReviewSummary | null
  active_credentials?: ActiveCredentialSummary[] | null
  created_at: string
  updated_at?: string
  submitted_at?: string
}

export interface RiderApplicationStatusView {
  statusCode: string
  isDraft: boolean
  isSubmitted: boolean
  isApproved: boolean
  isRejected: boolean
  canEdit: boolean
  canSubmit: boolean
}

export function buildRiderApplicationStatusView(status?: ApplicationStatus | string): RiderApplicationStatusView {
  const normalizedStatus = String(status || 'draft').trim().toLowerCase() || 'draft'

  switch (normalizedStatus) {
    case 'submitted':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: true,
        isApproved: false,
        isRejected: false,
        canEdit: false,
        canSubmit: false
      }
    case 'approved':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: false,
        isApproved: true,
        isRejected: false,
        canEdit: false,
        canSubmit: false
      }
    case 'rejected':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: false,
        isApproved: false,
        isRejected: true,
        canEdit: false,
        canSubmit: false
      }
    default:
      return {
        statusCode: 'draft',
        isDraft: true,
        isSubmitted: false,
        isApproved: false,
        isRejected: false,
        canEdit: true,
        canSubmit: true
      }
  }
}

function hasRiderText(value?: string) {
  return typeof value === 'string' && value.trim().length > 0
}

function hasRiderHealthCertKeyFields(latest: RiderApplicationResponse) {
  return hasRiderText(latest.health_cert_ocr?.valid_end)
    && hasRiderText(latest.health_cert_ocr?.name)
}

function buildRiderApplicationReadonlyMessage(status: ApplicationStatus) {
  switch (status) {
    case 'submitted':
      return '申请已提交，暂时不能修改资料'
    case 'approved':
      return '入驻已通过，无需重复上传资料'
    case 'rejected':
      return '申请已驳回，请先重置后再修改资料'
    default:
      return '当前申请状态暂不支持修改资料'
  }
}

function assertRiderApplicationEditable(application: RiderApplicationResponse) {
  if (application.status === 'draft') {
    return
  }

  throw new AppError({
    type: ErrorType.BUSINESS,
    message: `rider application is not editable in status ${application.status}`,
    userMessage: buildRiderApplicationReadonlyMessage(application.status)
  })
}

function checkRiderIDCardWriteback(latest: RiderApplicationResponse, side: 'Front' | 'Back') {
  const status = latest.id_card_ocr?.status || ''
  const error = latest.id_card_ocr?.error || ''

  if (side === 'Front') {
    return {
      ready: hasRiderText(latest.id_card_ocr?.id_number),
      failed: status === 'failed',
      errorMessage: error
    }
  }

  return {
    ready: hasRiderText(latest.id_card_ocr?.valid_end),
    failed: status === 'failed',
    errorMessage: error
  }
}

function checkRiderHealthCertWriteback(latest: RiderApplicationResponse) {
  const status = latest.health_cert_ocr?.status || ''
  const error = latest.health_cert_ocr?.error || ''
  const hasKeyFields = hasRiderHealthCertKeyFields(latest)
  return {
    ready: hasKeyFields,
    failed: status === 'failed' || (status === 'done' && !hasKeyFields),
    errorMessage: error || (status === 'done' && !hasKeyFields
      ? '健康证关键字段未识别，请重新上传清晰、无遮挡的健康证照片'
      : '')
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
  const draft = await getOrCreateRiderApplication()
  assertRiderApplicationEditable(draft)

  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory
  })
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
  const draft = await getOrCreateRiderApplication()
  assertRiderApplicationEditable(draft)

  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory: 'health_cert'
  })
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