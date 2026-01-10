/**
 * 证照上传与OCR识别接口
 * 符合 docs/certificate_upload_guide.md 规范
 * 适配 Supabase 架构
 */

import { supabaseRequest } from '../services/supabase'
import { uploadFile } from '../utils/request'

// ==================== 类型定义 ====================

export type OCRRole = 'merchant' | 'rider' | 'operator'

export interface OCRResponse {
  name?: string
  enterprise_name?: string
  reg_num?: string
  address?: string
  legal_representative?: string
  valid_period?: string
  business_scope?: string
  type_of_enterprise?: string
  registered_capital?: string
  credit_code?: string
  id_number?: string
  gender?: string
  nation?: string
  valid_end?: string
  valid_start?: string
  permit_no?: string
  cert_number?: string
  [key: string]: any
}

export interface OCRUploadResult {
  applicationData: any
  ocrData: OCRResponse
}

// ==================== 通用 OCR 助手 ====================

/**
 * 通用 Supabase OCR 链路
 */
async function performSupabaseOCR(
    filePath: string, 
    type: string, 
    role: OCRRole, 
    side?: string
): Promise<OCRUploadResult> {
    // 1. 上传图片 (assets 桶或 identity 桶由 image-service 根据 category 自动决定)
    const bucketCategory = (type === 'id_card' || type === 'health_cert') ? 'identity' : 'assets'
    const uploadRes = await uploadFile(filePath, '', 'file', { category: bucketCategory })
    const imageUrl = (uploadRes as any).url
    if (!imageUrl) throw new Error('Upload failed')

    // 2. 映射目标表
    const tableMap: Record<OCRRole, string> = {
        merchant: 'merchant_applications',
        rider: 'rider_applications',
        operator: 'operator_applications'
    }
    const targetTable = tableMap[role]

    // 3. 获取申请单 (获取当前用户的草稿)
    // 这里我们直接通过 ocr-service 更新，ocr-service 内部会处理 application_id 的查找（如果需要）
    // 但我们的 ocr-service 需要 application_id。
    // 所以我们需要先查到 application_id。
    const { data: appData } = await supabaseRequest<any[]>({
        url: `/rest/v1/${targetTable}?select=id`,
        method: 'GET'
    })
    const appId = appData?.[0]?.id
    if (!appId) throw new Error(`No active ${role} application found`)

    // 4. 调用 OCR Edge Function
    const { data: ocrRes, error } = await supabaseRequest<any>({
        url: '/functions/v1/ocr-service',
        method: 'POST',
        data: {
            application_id: appId,
            image_url: imageUrl,
            type,
            side,
            target_table: targetTable
        }
    })

    if (error) throw error

    // 5. 获取更新后的完整申请数据
    const { data: updatedApp } = await supabaseRequest<any[]>({
        url: `/rest/v1/${targetTable}?id=eq.${appId}`,
        method: 'GET'
    })

    return {
        applicationData: updatedApp?.[0],
        ocrData: ocrRes.ocr_result || ocrRes
    }
}

// ==================== 商户 OCR 接口 ====================

export function ocrBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return performSupabaseOCR(filePath, 'business_license', 'merchant')
}

export function ocrIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return performSupabaseOCR(filePath, 'id_card', 'merchant', capitalizedSide)
}

export function ocrFoodLicense(filePath: string): Promise<OCRUploadResult> {
  return performSupabaseOCR(filePath, 'food_permit', 'merchant')
}

// ==================== 骑手 OCR 接口 ====================

export function ocrRiderIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return performSupabaseOCR(filePath, 'id_card', 'rider', capitalizedSide)
}

export function ocrRiderHealthCert(filePath: string): Promise<OCRUploadResult> {
  return performSupabaseOCR(filePath, 'health_cert', 'rider')
}

// ==================== 运营商 OCR 接口 ====================

export function ocrOperatorBusinessLicense(filePath: string): Promise<OCRUploadResult> {
  return performSupabaseOCR(filePath, 'business_license', 'operator')
}

export function ocrOperatorIdCard(filePath: string, side: 'front' | 'back' = 'front'): Promise<OCRUploadResult> {
  const capitalizedSide = side === 'front' ? 'Front' : 'Back'
  return performSupabaseOCR(filePath, 'id_card', 'operator', capitalizedSide)
}

export default {
    ocrBusinessLicense,
    ocrIdCard,
    ocrFoodLicense,
    ocrRiderIdCard,
    ocrRiderHealthCert,
    ocrOperatorBusinessLicense,
    ocrOperatorIdCard
}
