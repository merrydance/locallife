import { request } from '../../../../utils/request'
import { uploadMedia } from '../../../../utils/media'
import { enqueueOCRJobAndRefresh } from './ocr-jobs'
import { ApplicationStatus } from './onboarding'
import type { AgreementConsentPayload } from './agreement-consent'

export interface GroupBusinessLicenseOCR {
  status?: 'pending' | 'processing' | 'done' | 'failed'
  error?: string
  queued_at?: string
  started_at?: string
  ocr_at?: string
  ocr_job_id?: number
  credit_code?: string
  reg_num?: string
  enterprise_name?: string
}

export interface GroupIDCardOCR {
  status?: 'pending' | 'processing' | 'done' | 'failed'
  error?: string
  queued_at?: string
  started_at?: string
  ocr_at?: string
  ocr_job_id?: number
  name?: string
  id_number?: string
  gender?: string
  nation?: string
  address?: string
  valid_date?: string
}

export interface GroupApplicationResponse {
  id: number
  applicant_user_id: number
  group_name: string
  contact_phone: string
  license_number?: string
  license_image_asset_id?: number
  business_license_ocr?: GroupBusinessLicenseOCR
  legal_person_name?: string
  legal_person_id_number?: string
  id_card_front_asset_id?: number
  id_card_back_asset_id?: number
  id_card_front_ocr?: GroupIDCardOCR
  id_card_back_ocr?: GroupIDCardOCR
  address?: string
  region_id?: number
  status: ApplicationStatus
  reject_reason?: string
  reviewed_by?: number
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface UpdateGroupApplicationBasicRequest {
  group_name?: string
  contact_phone?: string
  license_number?: string
  license_image_asset_id?: number
  address?: string
  region_id?: number
}

export interface GroupJoinRequestRequest {
  reason?: string
}

export interface GroupJoinRequestResponse {
  id: number
  group_id: number
  merchant_id: number
  applicant_user_id: number
  status: 'pending' | 'approved' | 'rejected' | 'cancelled'
  reason?: string
  reviewed_by?: number
  reviewed_at?: string
  created_at: string
}

function hasGroupText(value?: string) {
  return typeof value === 'string' && value.trim().length > 0
}

function checkGroupBusinessLicenseWriteback(latest: GroupApplicationResponse) {
  const status = latest.business_license_ocr?.status || ''
  const error = latest.business_license_ocr?.error || ''
  return {
    ready: status === 'done'
      || hasGroupText(latest.license_number)
      || hasGroupText(latest.business_license_ocr?.credit_code)
      || hasGroupText(latest.business_license_ocr?.reg_num)
      || hasGroupText(latest.business_license_ocr?.enterprise_name),
    failed: status === 'failed',
    errorMessage: error
  }
}

function checkGroupIDCardWriteback(latest: GroupApplicationResponse, side: 'Front' | 'Back') {
  const payload = side === 'Front' ? latest.id_card_front_ocr : latest.id_card_back_ocr
  const status = payload?.status || ''
  const error = payload?.error || ''

  if (side === 'Front') {
    return {
      ready: status === 'done'
        || hasGroupText(latest.legal_person_name)
        || hasGroupText(latest.legal_person_id_number)
        || hasGroupText(payload?.name)
        || hasGroupText(payload?.id_number),
      failed: status === 'failed',
      errorMessage: error
    }
  }

  return {
    ready: status === 'done' || hasGroupText(payload?.valid_date),
    failed: status === 'failed',
    errorMessage: error
  }
}

/**
 * 获取或创建集团入驻草稿
 */
export function getOrCreateGroupApplication() {
  return request<GroupApplicationResponse>({
    url: '/v1/groups/applications/me',
    method: 'GET'
  })
}

/**
 * 更新集团基础信息
 */
export function updateGroupApplicationBasic(data: UpdateGroupApplicationBasicRequest) {
  return request<GroupApplicationResponse>({
    url: '/v1/groups/applications/basic',
    method: 'PUT',
    data
  })
}

export function deleteGroupApplicationDocument(
  documentType: 'business_license' | 'id_card_front' | 'id_card_back'
) {
  return request<GroupApplicationResponse>({
    url: `/v1/groups/applications/documents/${documentType}`,
    method: 'DELETE'
  })
}

/**
 * 上传集团营业执照并通过统一 OCR job 识别
 */
export async function ocrGroupBusinessLicense(filePath: string) {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'group',
    mediaCategory: 'business_license'
  })
  const draft = await getOrCreateGroupApplication()
  return enqueueOCRJobAndRefresh(
    {
      document_type: 'business_license',
      media_asset_id: mediaId,
      owner_type: 'group_application',
      owner_id: draft.id
    },
    getOrCreateGroupApplication,
    {
      verifyResult: checkGroupBusinessLicenseWriteback,
      maxAttempts: 20,
      intervalMs: 1000
    }
  )
}

/**
 * 上传集团负责人身份证并通过统一 OCR job 识别
 */
export async function ocrGroupIdCard(filePath: string, side: 'Front' | 'Back') {
  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'group',
    mediaCategory
  })
  const draft = await getOrCreateGroupApplication()
  return enqueueOCRJobAndRefresh(
    {
      document_type: 'id_card',
      media_asset_id: mediaId,
      owner_type: 'group_application',
      owner_id: draft.id,
      side: side === 'Front' ? 'front' : 'back'
    },
    getOrCreateGroupApplication,
    {
      verifyResult: (latest) => checkGroupIDCardWriteback(latest, side),
      maxAttempts: 20,
      intervalMs: 1000
    }
  )
}

/**
 * 提交集团入驻申请
 */
export function submitGroupApplication(data?: AgreementConsentPayload) {
  return request<GroupApplicationResponse>({
    url: '/v1/groups/applications/submit',
    method: 'POST',
    data
  })
}

/**
 * 搜索集团
 */
export function searchGroups(keyword: string) {
  return request<Array<Record<string, unknown>>>({
    url: '/v1/groups',
    method: 'GET',
    data: { keyword }
  })
}

/**
 * 申请加入集团
 */
export function applyToJoinGroup(groupID: number, data: GroupJoinRequestRequest) {
  return request<GroupJoinRequestResponse>({
    url: `/v1/groups/${groupID}/join-requests`,
    method: 'POST',
    data
  })
}
