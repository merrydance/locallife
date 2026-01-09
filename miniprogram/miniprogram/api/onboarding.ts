import { request, uploadFile } from '../utils/request'

// ==================== OCR Status Types ====================

export type OCRStatus = 'pending' | 'processing' | 'done' | 'failed'

export interface BaseOCRData {
  status: OCRStatus
  error?: string
  queued_at?: string
  started_at?: string
  ocr_at?: string
}

export interface BusinessLicenseOCRData extends BaseOCRData {
  reg_num?: string
  enterprise_name?: string
  legal_representative?: string
  type_of_enterprise?: string
  address?: string
  business_scope?: string
  registered_capital?: string
  valid_period?: string
  credit_code?: string
}

export interface FoodPermitOCRData extends BaseOCRData {
  raw_text?: string
  permit_no?: string
  company_name?: string
  valid_from?: string
  valid_to?: string
}

export interface IDCardOCRData extends BaseOCRData {
  // Front
  name?: string
  id_number?: string
  gender?: string
  nation?: string
  address?: string
  // Back
  valid_date?: string
}

// ==================== Application Response Types ====================

export type ApplicationStatus = 'draft' | 'submitted' | 'approved' | 'rejected'

export interface MerchantApplicationDraftResponse {
  id: number
  user_id: number
  merchant_name: string
  contact_phone: string
  business_address: string
  longitude: string | null
  latitude: string | null
  region_id: number | null
  business_license_image_url: string
  business_license_number: string
  business_scope: string | null
  business_license_ocr: BusinessLicenseOCRData | null
  food_permit_url: string | null
  food_permit_ocr: FoodPermitOCRData | null
  legal_person_name: string
  legal_person_id_number: string
  legal_person_id_front_url: string
  legal_person_id_back_url: string
  id_card_front_ocr: IDCardOCRData | null
  id_card_back_ocr: IDCardOCRData | null
  storefront_images?: string[] | null  // 门头照 URL 数组，最多3张
  environment_images?: string[] | null // 环境照 URL 数组，最多5张
  status: ApplicationStatus
  reject_reason: string | null
  created_at: string
  updated_at: string
}

// 图片上传响应
export interface UploadImageResponse {
  image_url: string
}

// 更新图片请求
export interface UpdateMerchantImagesRequest {
  storefront_images?: string[]
  environment_images?: string[]
}

// ==================== Request Types ====================

// 对齐 api/merchant-application.ts 中的定义，支持更完整的字段更新
export interface UpdateMerchantBasicInfoRequest {
  merchant_name?: string
  contact_phone?: string
  business_address?: string
  longitude?: string
  latitude?: string
  region_id?: number
  business_license_number?: string
  business_license_image_url?: string
  business_scope?: string
  legal_person_name?: string
  legal_person_id_number?: string
  legal_person_id_front_url?: string
  legal_person_id_back_url?: string
  food_permit_url?: string
  storefront_images?: string[] // 虽然有单独接口，但API也可能支持
  environment_images?: string[]
}

// ==================== API Methods ====================

/**
 * 获取或创建商户入驻申请草稿
 * GET /v1/merchant/application
 * - 200: 返回现有草稿
 * - 201: 创建新草稿并返回
 * - 409: 已存在 submitted/approved
 */
export function getMerchantApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application',
    method: 'GET'
  })
}

/**
 * 更新基础信息（草稿可编辑）
 * PUT /v1/merchant/application/basic
 */
export function updateMerchantBasicInfo(data: UpdateMerchantBasicInfoRequest) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/basic',
    method: 'PUT',
    data
  })
}

/**
 * 营业执照 OCR（异步）
 * POST /v1/merchant/application/license/ocr
 * @param filePath 本地文件路径，若不传则复用已上传的图片
 */
export function ocrBusinessLicense(filePath?: string) {
  if (filePath) {
    return uploadFile<MerchantApplicationDraftResponse>(
      filePath,
      '/v1/merchant/application/license/ocr',
      'image'
    )
  }
  // 不传文件时，触发复用已有图片的 OCR
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/license/ocr',
    method: 'POST'
  })
}

/**
 * 食品经营许可证 OCR（异步）
 * POST /v1/merchant/application/foodpermit/ocr
 */
export function ocrFoodPermit(filePath?: string) {
  if (filePath) {
    return uploadFile<MerchantApplicationDraftResponse>(
      filePath,
      '/v1/merchant/application/foodpermit/ocr',
      'image'
    )
  }
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/foodpermit/ocr',
    method: 'POST'
  })
}

/**
 * 身份证 OCR（异步）
 * POST /v1/merchant/application/idcard/ocr
 * @param side 'Front' 或 'Back'
 */
export function ocrIdCard(filePath: string | undefined, side: 'Front' | 'Back') {
  if (filePath) {
    return uploadFile<MerchantApplicationDraftResponse>(
      filePath,
      '/v1/merchant/application/idcard/ocr',
      'image',
      { side }
    )
  }
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/idcard/ocr',
    method: 'POST',
    data: { side }
  })
}

/**
 * 提交申请（自动审核）
 * POST /v1/merchant/application/submit
 * 无请求体，返回 approved 或 rejected
 */
export function submitMerchantApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/submit',
    method: 'POST'
  })
}

/**
 * 获取当前用户最新申请（用于 submitted 后轮询）
 * GET /v1/merchants/applications/me
 */
export function getMyApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchants/applications/me',
    method: 'GET'
  })
}

/**
 * 重置被拒绝申请为草稿
 * POST /v1/merchant/application/reset
 */
export function resetMerchantApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/reset',
    method: 'POST'
  })
}

/**
 * 上传门头照/环境照图片文件
 * POST /v1/merchants/images/upload
 * @param filePath 本地文件路径
 * @param category 'storefront' 或 'environment'
 */
export function uploadMerchantImage(filePath: string, category: 'storefront' | 'environment') {
  return uploadFile<UploadImageResponse>(
    filePath,
    '/v1/merchants/images/upload',
    'image',
    { category }
  )
}

/**
 * 保存门头照/环境照 URL 到草稿
 * PUT /v1/merchant/application/images
 */
export function updateMerchantImages(data: UpdateMerchantImagesRequest) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/images',
    method: 'PUT',
    data
  })
}

// ==================== Rider & Other Types (Preserved) ====================

export interface ApplyRiderRequest {
  id_card_no: string
  phone: string
  real_name: string
}

export function submitRiderApplication(data: ApplyRiderRequest) {
  return request<void>({
    url: '/onboarding/rider',
    method: 'POST',
    data
  })
}

export interface DepositRequest extends Record<string, unknown> {
  amount: number
  remark?: string
}

export interface DepositResponse {
  amount?: number
  balance_after?: number
  created_at?: string
  id?: number
  remark?: string
  rider_id?: number
  type?: string
}

