import { request } from '../utils/request'
import { uploadMedia, postFormData } from '../utils/media'
import { ApplicationStatus } from './onboarding'
import type { AgreementConsentPayload } from './agreement-consent'

export interface GroupApplicationResponse {
  id: number
  applicant_user_id: number
  group_name: string
  contact_phone: string
  license_number?: string
  license_image_url?: string
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
  license_image_url?: string
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

/**
 * 上传并识别集团营业执照
 */
export async function ocrGroupBusinessLicense(filePath: string) {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'group',
    mediaCategory: 'business_license',
  })
  return postFormData<GroupApplicationResponse>(
    '/v1/groups/applications/license/ocr',
    { media_asset_id: mediaId }
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
