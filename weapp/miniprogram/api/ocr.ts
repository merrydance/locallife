/**
 * 证照上传与OCR识别接口
 * 符合 docs/certificate_upload_guide.md 规范
 * 支持商户、骑手、运营商三种角色
 */

import { uploadMedia } from '../utils/media'
import { request } from '../utils/request'
import { enqueueOCRJobAndRefresh } from './ocr-jobs'

// ==================== 类型定义 ====================

// 角色类型
export type OCRRole = 'merchant' | 'rider' | 'operator'

// Generic OCR Response interface - adjust based on actual returns
export interface OCRResponse {
  // Business License OCR fields (营业执照)
  name?: string
  enterprise_name?: string      // 企业名称
  reg_num?: string              // 注册号/统一社会信用代码
  address?: string              // 地址
  person?: string
  legal_representative?: string // 法定代表人
  valid_period?: string         // 营业期限
  business?: string
  business_scope?: string       // 经营范围
  type_of_enterprise?: string   // 企业类型
  registered_capital?: string   // 注册资本
  credit_code?: string

  // ID Card OCR fields (身份证)
  id?: string
  id_num?: string
  id_number?: string            // 身份证号码（后端实际字段名）
  gender?: string               // 性别
  nation?: string               // 民族
  valid_date?: string           // 有效期
  valid_end?: string            // 有效期截止
  valid_start?: string          // 有效期起始

  // Food License OCR fields (食品经营许可证)
  license_name?: string
  legal_person?: string
  validity?: string
  valid_from?: string           // 有效期起
  valid_to?: string             // 有效期止
  permit_no?: string            // 许可证编号

  // Health Cert fields (健康证 - 骑手)
  cert_number?: string          // 证书编号

  // Deprecated / Legacy fields for backward compatibility
  addr?: string
}

// OCR上传返回结果，包含完整申请数据和OCR结果
export interface OCRUploadResult {
  applicationData: Record<string, unknown> // 完整的申请数据（用于刷新页面）
  ocrData: OCRResponse          // OCR识别结果
}

type OCRPayload = Record<string, unknown>

interface OCRNestedPayload extends OCRPayload {
  business_license_ocr?: OCRResponse
  id_card_front_ocr?: OCRResponse
  id_card_back_ocr?: OCRResponse
  food_permit_ocr?: OCRResponse
  id_card_ocr?: OCRResponse
  health_cert_ocr?: OCRResponse
}

function toOCRResponse(payload: unknown): OCRResponse {
  return payload as OCRResponse
}

// ==================== 角色端点映射 ====================

const API_ENDPOINTS = {
  merchant: {
    application: '/v1/merchant/application'
  },
  rider: {
    application: '/v1/rider/application'
  },
  operator: {
    application: '/v1/operator/application'
  }
}

interface OCRJobResponse {
  ocr_job_id: number
  status: string
}

async function uploadMerchantApplicationOCR(
  filePath: string,
  mediaCategory: 'business_license' | 'food_permit' | 'id_card_front' | 'id_card_back',
  documentType: 'business_license' | 'food_permit' | 'id_card',
  side?: 'Front' | 'Back'
): Promise<OCRPayload> {
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory
  })

  const draft = await request<{ id: number }>({
    url: API_ENDPOINTS.merchant.application,
    method: 'GET'
  })

  await request<OCRJobResponse>({
    url: '/v1/ocr/jobs',
    method: 'POST',
    data: {
      document_type: documentType,
      media_asset_id: mediaId,
      owner_type: 'merchant_application',
      owner_id: draft.id,
      side: side ? side.toLowerCase() : undefined
    }
  })

  return request<OCRPayload>({
    url: API_ENDPOINTS.merchant.application,
    method: 'GET'
  })
}

// ==================== 商户 OCR 接口 ====================

/**
 * 商户营业执照OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=business_license
 */
export function ocrBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadMerchantApplicationOCR(filePath, 'business_license', 'business_license')
    .then((res: OCRNestedPayload) => {
      return {
        applicationData: res,
        ocrData: toOCRResponse(res.business_license_ocr || res)
      }
    })
}

/**
 * 商户身份证OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=id_card
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
export function ocrIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return uploadMerchantApplicationOCR(
    filePath,
    side === 'front' ? 'id_card_front' : 'id_card_back',
    'id_card',
    capitalizedSide
  )
    .then((res: OCRNestedPayload) => {
      const ocrData = side === 'front'
        ? toOCRResponse(res.id_card_front_ocr || res)
        : toOCRResponse(res.id_card_back_ocr || res)
      return {
        applicationData: res,
        ocrData
      }
    })
}

/**
 * 商户食品经营许可证OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=food_permit
 */
export function ocrFoodLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadMerchantApplicationOCR(filePath, 'food_permit', 'food_permit')
    .then((res: OCRNestedPayload) => {
      return {
        applicationData: res,
        ocrData: toOCRResponse(res.food_permit_ocr || res)
      }
    })
}

// ==================== 骑手 OCR 接口 ====================

/**
 * 骑手身份证OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=rider_application，document_type=id_card
 * @param filePath 本地文件路径
 * @param side 'front' (正面) 或 'back' (背面)
 */
export function ocrRiderIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  return uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory: side === 'front' ? 'id_card_front' : 'id_card_back'
  })
    .then(async ({ mediaId }) => {
      const draft = await request<{ id: number }>({
        url: API_ENDPOINTS.rider.application,
        method: 'GET'
      })
      return enqueueOCRJobAndRefresh(
        {
          document_type: 'id_card',
          media_asset_id: mediaId,
          owner_type: 'rider_application',
          owner_id: draft.id,
          side
        },
        () => request<OCRNestedPayload>({
          url: API_ENDPOINTS.rider.application,
          method: 'GET'
        })
      )
    })
    .then((res: OCRNestedPayload) => {
      const ocrData = res.id_card_ocr || res
      return {
        applicationData: res,
        ocrData: toOCRResponse(ocrData)
      }
    })
}

/**
 * 骑手健康证上传
 * 统一走 POST /v1/ocr/jobs，owner_type=rider_application，document_type=health_cert
 */
export function ocrRiderHealthCert(filePath: string): Promise<OCRUploadResult> {
  return uploadMedia(filePath, {
    businessType: 'rider',
    mediaCategory: 'health_cert'
  })
    .then(async ({ mediaId }) => {
      const draft = await request<{ id: number }>({
        url: API_ENDPOINTS.rider.application,
        method: 'GET'
      })
      return enqueueOCRJobAndRefresh(
        {
          document_type: 'health_cert',
          media_asset_id: mediaId,
          owner_type: 'rider_application',
          owner_id: draft.id
        },
        () => request<OCRNestedPayload>({
          url: API_ENDPOINTS.rider.application,
          method: 'GET'
        })
      )
    })
    .then((res: OCRNestedPayload) => {
      return {
        applicationData: res,
        ocrData: toOCRResponse(res.health_cert_ocr || res)
      }
    })
}

// ==================== 运营商 OCR 接口 ====================

/**
 * 运营商营业执照OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=operator_application，document_type=business_license
 */
export function ocrOperatorBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadMedia(filePath, {
    businessType: 'operator',
    mediaCategory: 'business_license'
  })
    .then(async ({ mediaId }) => {
      const draft = await request<{ id: number }>({
        url: API_ENDPOINTS.operator.application,
        method: 'GET'
      })
      return enqueueOCRJobAndRefresh(
        {
          document_type: 'business_license',
          media_asset_id: mediaId,
          owner_type: 'operator_application',
          owner_id: draft.id
        },
        () => request<OCRNestedPayload>({
          url: API_ENDPOINTS.operator.application,
          method: 'GET'
        })
      )
    })
    .then((res: OCRNestedPayload) => {
      return {
        applicationData: res,
        ocrData: toOCRResponse(res.business_license_ocr || res)
      }
    })
}

/**
 * 运营商身份证OCR
 * 统一走 POST /v1/ocr/jobs，owner_type=operator_application，document_type=id_card
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
export function ocrOperatorIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  return uploadMedia(filePath, {
    businessType: 'operator',
    mediaCategory: side === 'front' ? 'id_card_front' : 'id_card_back'
  })
    .then(async ({ mediaId }) => {
      const draft = await request<{ id: number }>({
        url: API_ENDPOINTS.operator.application,
        method: 'GET'
      })
      return enqueueOCRJobAndRefresh(
        {
          document_type: 'id_card',
          media_asset_id: mediaId,
          owner_type: 'operator_application',
          owner_id: draft.id,
          side
        },
        () => request<OCRNestedPayload>({
          url: API_ENDPOINTS.operator.application,
          method: 'GET'
        })
      )
    })
    .then((res: OCRNestedPayload) => {
      const ocrData = side === 'front'
        ? toOCRResponse(res.id_card_front_ocr || res)
        : toOCRResponse(res.id_card_back_ocr || res)
      return {
        applicationData: res,
        ocrData
      }
    })
}
