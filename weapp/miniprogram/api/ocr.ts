/**
 * 证照上传与OCR识别接口
 * 符合 docs/certificate_upload_guide.md 规范
 * 支持商户、骑手、运营商三种角色
 */

import { uploadFile } from '../utils/request'

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
  applicationData: any          // 完整的申请数据（用于刷新页面）
  ocrData: OCRResponse          // OCR识别结果
}

// ==================== 角色端点映射 ====================

const API_ENDPOINTS = {
  merchant: {
    license: '/v1/merchant/application/license/ocr',
    idcard: '/v1/merchant/application/idcard/ocr',
    foodpermit: '/v1/merchant/application/foodpermit/ocr'
  },
  rider: {
    idcard: '/v1/rider/application/idcard/ocr',
    healthcert: '/v1/rider/application/healthcert'
  },
  operator: {
    license: '/v1/operator/application/license/ocr',
    idcard: '/v1/operator/application/idcard/ocr'
  }
}

// ==================== 证照上传功能 ====================

/**
 * 通用文件上传（multipart/form-data）
 * @param url API路径
 * @param filePath 本地文件路径
 * @param formData 附加表单数据（如 side 参数）
 */
function uploadOCR(url: string, filePath: string, formData: any = {}): Promise<any> {
  return uploadFile(filePath, url, 'image', formData)
}

// ==================== 商户 OCR 接口 ====================

/**
 * 商户营业执照OCR
 * POST /v1/merchant/application/license/ocr
 */
export function ocrBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadOCR(API_ENDPOINTS.merchant.license, filePath)
    .then((res: any) => {
      console.log('[OCR] Merchant Business License Response:', JSON.stringify(res))
      return {
        applicationData: res,
        ocrData: res.business_license_ocr || res
      }
    })
}

/**
 * 商户身份证OCR
 * POST /v1/merchant/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
export function ocrIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  // 商户接口使用 "Front"/"Back" 首字母大写
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return uploadOCR(API_ENDPOINTS.merchant.idcard, filePath, { side: capitalizedSide })
    .then((res: any) => {
      console.log(`[OCR] Merchant ID Card ${side} Response:`, JSON.stringify(res))
      const ocrData = side === 'front'
        ? (res.id_card_front_ocr || res)
        : (res.id_card_back_ocr || res)
      return {
        applicationData: res,
        ocrData
      }
    })
}

/**
 * 商户食品经营许可证OCR
 * POST /v1/merchant/application/foodpermit/ocr
 */
export function ocrFoodLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadOCR(API_ENDPOINTS.merchant.foodpermit, filePath)
    .then((res: any) => {
      console.log('[OCR] Merchant Food License Response:', JSON.stringify(res))
      return {
        applicationData: res,
        ocrData: res.food_permit_ocr || res
      }
    })
}

// ==================== 骑手 OCR 接口 ====================

/**
 * 骑手身份证OCR
 * POST /v1/rider/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面) 或 'back' (背面)
 */
export function ocrRiderIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  // 骑手接口使用小写 "front"/"back"
  return uploadOCR(API_ENDPOINTS.rider.idcard, filePath, { side })
    .then((res: any) => {
      console.log(`[OCR] Rider ID Card ${side} Response:`, JSON.stringify(res))
      const ocrData = res.id_card_ocr || res
      return {
        applicationData: res,
        ocrData
      }
    })
}

/**
 * 骑手健康证上传
 * POST /v1/rider/application/healthcert
 */
export function ocrRiderHealthCert(filePath: string): Promise<OCRUploadResult> {
  return uploadOCR(API_ENDPOINTS.rider.healthcert, filePath)
    .then((res: any) => {
      console.log('[OCR] Rider Health Cert Response:', JSON.stringify(res))
      return {
        applicationData: res,
        ocrData: res.health_cert_ocr || res
      }
    })
}

// ==================== 运营商 OCR 接口 ====================

/**
 * 运营商营业执照OCR
 * POST /v1/operator/application/license/ocr
 */
export function ocrOperatorBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return uploadOCR(API_ENDPOINTS.operator.license, filePath)
    .then((res: any) => {
      console.log('[OCR] Operator Business License Response:', JSON.stringify(res))
      return {
        applicationData: res,
        ocrData: res.business_license_ocr || res
      }
    })
}

/**
 * 运营商身份证OCR
 * POST /v1/operator/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
export function ocrOperatorIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  // 运营商接口使用 "Front"/"Back" 首字母大写
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return uploadOCR(API_ENDPOINTS.operator.idcard, filePath, { side: capitalizedSide })
    .then((res: any) => {
      console.log(`[OCR] Operator ID Card ${side} Response:`, JSON.stringify(res))
      const ocrData = side === 'front'
        ? (res.id_card_front_ocr || res)
        : (res.id_card_back_ocr || res)
      return {
        applicationData: res,
        ocrData
      }
    })
}
