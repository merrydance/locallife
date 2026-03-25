import { request } from '../utils/request'
import { uploadMedia, postFormData } from '../utils/media'
import { ApplicationStatus } from './onboarding'
import type { AgreementConsentPayload } from './agreement-consent'

type OCRResult = Record<string, unknown>

export interface AvailableRegion {
  id: number
  name: string
  level?: number
  parent_id?: number
  parent_name?: string
}

export interface RegionItem {
  id: number
  code: string
  name: string
  level: number
  parent_id?: number
}

export interface OperatorApplicationResponse {
  id: number
  user_id: number
  region_id: number
  region_name?: string
  name?: string
  contact_name?: string
  contact_phone?: string
  business_license_asset_id?: number
  business_license_number?: string
  legal_person_name?: string
  legal_person_id_number?: string
  id_card_front_asset_id?: number
  id_card_back_asset_id?: number
  requested_contract_years: number
  status: ApplicationStatus
  reject_reason?: string
  created_at: string
  updated_at: string
  submitted_at?: string
  reviewed_at?: string
  business_license_ocr?: OCRResult
  id_card_front_ocr?: OCRResult
  id_card_back_ocr?: OCRResult
  /** 申请已通过且运营商账号已建立（即用户已是正式运营商）*/
  is_operator?: boolean
}

export interface CreateOperatorDraftRequest {
  region_id: number
}

export interface UpdateOperatorRegionRequest {
  region_id: number
}

export interface UpdateOperatorBasicRequest {
  name?: string
  contact_name?: string
  contact_phone?: string
  requested_contract_years?: number
}

/**
 * 获取或创建运营商入驻申请草稿
 */
export function getOrCreateOperatorApplication(data?: CreateOperatorDraftRequest) {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application',
    method: 'POST',
    data
  })
}

/**
 * 获取当前申请状态
 */
export function getOperatorApplication() {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application',
    method: 'GET'
  })
}

/**
 * 更新申请区域
 */
export function updateOperatorRegion(data: UpdateOperatorRegionRequest) {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application/region',
    method: 'PUT',
    data
  })
}

/**
 * 更新基础信息
 */
export function updateOperatorBasic(data: UpdateOperatorBasicRequest) {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application/basic',
    method: 'PUT',
    data
  })
}

/**
 * 上传营业执照并识别
 */
export async function ocrOperatorBusinessLicense(filePath: string) {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'operator',
    mediaCategory: 'business_license'
  })
  return postFormData<OperatorApplicationResponse>(
    '/v1/operator/application/license/ocr',
    { media_asset_id: mediaId }
  )
}

/**
 * 上传身份证并识别
 */
export async function ocrOperatorIdCard(filePath: string, side: 'Front' | 'Back') {
  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'operator',
    mediaCategory
  })
  return postFormData<OperatorApplicationResponse>(
    '/v1/operator/application/idcard/ocr',
    { media_asset_id: mediaId, side }
  )
}

/**
 * 获取可申请的区域列表
 */
export function listAvailableRegions(params: { page_id: number, page_size: number, level?: number, parent_id?: number, keyword?: string }) {
  return request<{ regions: AvailableRegion[], totalCount: number }>({
    url: '/v1/regions/available',
    method: 'GET',
    data: params
  })
}

/**
 * 获取区域列表（用于分级选择）
 */
export function listRegions(params: { page_id: number, page_size: number, level?: number, parent_id?: number }) {
  return request<RegionItem[]>({
    url: '/v1/regions',
    method: 'GET',
    data: params
  })
}

/**
 * 提交申请
 */
export function submitOperatorApplication(data?: AgreementConsentPayload) {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application/submit',
    method: 'POST',
    data
  })
}

// ─── 运营商区域扩展申请 ───

export interface RegionExpansionApplication {
  id: number
  operator_id: number
  region_id: number
  region_name: string
  status: 'pending' | 'approved' | 'rejected'
  reject_reason?: string
  created_at: string
}

/**
 * 申请运营更多区域
 */
export function applyRegionExpansion(regionId: number) {
  return request<RegionExpansionApplication>({
    url: '/v1/operator/region-expansion',
    method: 'POST',
    data: { region_id: regionId }
  })
}

/**
 * 获取自己的区域扩展申请列表
 */
export function listRegionExpansionApplications() {
  return request<{ applications: RegionExpansionApplication[] }>({
    url: '/v1/operator/region-expansion',
    method: 'GET'
  })
}