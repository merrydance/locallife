import { request } from '../utils/request'
import { uploadMedia, postFormData, MediaUploadResult } from '../utils/media'
import type { AgreementConsentPayload } from './agreement-consent'

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
  business_license_media_asset_id?: number | null
  business_license_number: string
  business_scope: string | null
  business_license_ocr: BusinessLicenseOCRData | null
  food_permit_media_asset_id?: number | null
  food_permit_ocr: FoodPermitOCRData | null
  legal_person_name: string
  legal_person_id_number: string
  id_card_front_media_asset_id?: number | null
  id_card_back_media_asset_id?: number | null
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

// 兼容当前商户入驻基础信息更新请求定义。
export interface UpdateMerchantBasicInfoRequest {
  merchant_name?: string
  contact_phone?: string
  business_address?: string
  longitude?: string
  latitude?: string
  region_id?: number
  business_license_number?: string
  business_scope?: string
  legal_person_name?: string
  legal_person_id_number?: string
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
 * 若传 filePath：先上传到媒体服务，再个 media_asset_id 调用 OCR
 */
export async function ocrBusinessLicense(filePath?: string) {
  if (filePath) {
    const { mediaId } = await uploadMedia(filePath, {
      businessType: 'merchant',
      mediaCategory: 'business_license'
    })
    return postFormData<MerchantApplicationDraftResponse>(
      '/v1/merchant/application/license/ocr',
      { media_asset_id: mediaId }
    )
  }
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/license/ocr',
    method: 'POST'
  })
}

/**
 * 食品经营许可证 OCR（异步）
 * POST /v1/merchant/application/foodpermit/ocr
 */
export async function ocrFoodPermit(filePath?: string) {
  if (filePath) {
    const { mediaId } = await uploadMedia(filePath, {
      businessType: 'merchant',
      mediaCategory: 'food_permit'
    })
    return postFormData<MerchantApplicationDraftResponse>(
      '/v1/merchant/application/foodpermit/ocr',
      { media_asset_id: mediaId }
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
export async function ocrIdCard(filePath: string | undefined, side: 'Front' | 'Back') {
  if (filePath) {
    const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
    const { mediaId } = await uploadMedia(filePath, {
      businessType: 'merchant',
      mediaCategory
    })
    return postFormData<MerchantApplicationDraftResponse>(
      '/v1/merchant/application/idcard/ocr',
      { media_asset_id: mediaId, side }
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
export function submitMerchantApplication(data?: AgreementConsentPayload) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/submit',
    method: 'POST',
    data
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
 * 上传商户图片文件（Logo、门头照、环境照）
 * 媒体服务三步流程
 * @param filePath 本地文件路径
 * @param category 'logo' | 'storefront' | 'environment'
 * @returns { mediaId, displayUrl, urls }
 */
export function uploadMerchantImage(
  filePath: string,
  category: 'logo' | 'storefront' | 'environment'
): Promise<MediaUploadResult> {
  const mediaCategory =
    category === 'logo' ? 'logo'
    : category === 'storefront' ? 'storefront'
    : 'environment'
  return uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory
  })
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

// 更新商户店铺图片请求（已入驻商户使用）
export interface UpdateShopImagesRequest {
  storefront_images?: string[]
  environment_images?: string[]
}

// 更新商户店铺图片响应
export interface UpdateShopImagesResponse {
  storefront_images: string[] | null
  environment_images: string[] | null
}

/**
 * 更新已入驻商户的门头照/环境照
 * PATCH /v1/merchants/me/shop-images
 */
export function updateShopImages(data: UpdateShopImagesRequest) {
  return request<UpdateShopImagesResponse>({
    url: '/v1/merchants/me/shop-images',
    method: 'PATCH',
    data
  })
}

// ==================== Rider & Other Types (Preserved) ====================

export interface ApplyRiderRequest {
  id_card_no: string
  phone: string
  real_name: string
  vehicle_type?: string
  address?: string
  gender?: string
  id_card_front_images?: string[]
  id_card_back_images?: string[]
  health_certificate_images?: string[]
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

