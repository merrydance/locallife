import { request } from '../utils/request'
import { uploadMedia, postFormData } from '../utils/media'
import { ApplicationStatus } from './onboarding'
import type { AgreementConsentPayload } from './agreement-consent'

export interface RiderApplicationResponse {
  id: number
  user_id: number
  real_name?: string
  phone?: string
  id_card_front_url?: string
  id_card_back_url?: string
  id_card_ocr?: {
    name?: string
    id_number?: string
    gender?: string
    nation?: string
    address?: string
    valid_start?: string
    valid_end?: string
    ocr_at?: string
  }
  health_cert_url?: string
  health_cert_ocr?: {
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
 * 上传并在识别身份证照片
 */
export async function ocrRiderIdCard(filePath: string, side: 'Front' | 'Back') {
  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory
  })
  return postFormData<RiderApplicationResponse>(
    '/v1/rider/application/idcard/ocr',
    { media_asset_id: mediaId, side }
  )
}

/**
 * 上传并在识别健康证照片
 */
export async function ocrRiderHealthCert(filePath: string) {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory: 'health_cert'
  })
  return postFormData<RiderApplicationResponse>(
    '/v1/rider/application/healthcert',
    { media_asset_id: mediaId }
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