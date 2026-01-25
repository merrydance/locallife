import { request, uploadFile } from '../utils/request'
import { ApplicationStatus } from './onboarding'

export interface OperatorApplicationResponse {
  id: number
  user_id: number
  region_id: number
  region_name?: string
  name?: string
  contact_name?: string
  contact_phone?: string
  business_license_url?: string
  business_license_number?: string
  legal_person_name?: string
  legal_person_id_number?: string
  id_card_front_url?: string
  id_card_back_url?: string
  requested_contract_years: number
  status: ApplicationStatus
  reject_reason?: string
  created_at: string
  updated_at: string
  submitted_at?: string
  reviewed_at?: string
  business_license_ocr?: any
  id_card_front_ocr?: any
  id_card_back_ocr?: any
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
export function ocrOperatorBusinessLicense(filePath: string) {
  return uploadFile<OperatorApplicationResponse>(
    filePath,
    '/v1/operator/application/license/ocr',
    'image'
  )
}

/**
 * 上传身份证并识别
 */
export function ocrOperatorIdCard(filePath: string, side: 'Front' | 'Back') {
  return uploadFile<OperatorApplicationResponse>(
    filePath,
    '/v1/operator/application/idcard/ocr',
    'image',
    { side }
  )
}

/**
 * 获取可申请的区域列表
 */
export function listAvailableRegions(params: { page_id: number, page_size: number, level?: number }) {
  return request<{ regions: any[], totalCount: number }>({
    url: '/v1/regions/available',
    method: 'GET',
    data: params
  })
}

/**
 * 提交申请
 */
export function submitOperatorApplication() {
  return request<OperatorApplicationResponse>({
    url: '/v1/operator/application/submit',
    method: 'POST'
  })
}