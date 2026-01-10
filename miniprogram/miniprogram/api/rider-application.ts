/**
 * 骑手入驻申请接口
 * 适配 Supabase 架构，支持 OCR 即时识别与回填
 */

import { supabase, supabaseRequest } from '../services/supabase'
import { uploadFile } from '../utils/request'

// ==================== 数据类型定义 ====================

// 身份证OCR识别数据类型
export interface IDCardOCRData {
    address?: string                    // 地址
    gender?: string                     // 性别
    id_number?: string                  // 身份证号
    name?: string                       // 姓名
    nation?: string                     // 民族
    ocr_at?: string                     // OCR识别时间
    valid_end?: string                  // 有效期截止
    valid_start?: string                // 有效期起始
}

// 健康证OCR识别数据类型
export interface HealthCertOCRData {
    name?: string
    id_number?: string
    cert_number?: string                // 证书编号
    ocr_at?: string                     // OCR识别时间
    valid_end?: string                  // 有效期截止
    valid_start?: string                // 有效期起始
}

// 骑手申请数据类型
export interface RiderApplicationResponse {
    id: string
    user_id: string
    real_name?: string                  // 真实姓名
    phone?: string                      // 手机号
    id_card_front_url?: string          // 身份证正面图片URL
    id_card_back_url?: string           // 身份证背面图片URL
    health_cert_url?: string            // 健康证图片URL
    status: string                      // 申请状态
    reject_reason?: string | null       // 拒绝原因
    created_at: string                  // 创建时间
    updated_at?: string                 // 更新时间
    submitted_at?: string               // 提交时间
    
    // 扩展信息
    gender?: string
    hometown?: string
    current_address?: string
    id_card_number?: string
    id_card_validity?: string
    address?: string
    address_detail?: string
    latitude?: number
    longitude?: number
    vehicle_type?: string
    available_time?: string

    // OCR识别结果
    id_card_ocr?: IDCardOCRData         // 身份证OCR识别结果
    health_cert_ocr?: HealthCertOCRData // 健康证OCR识别结果
}

// 更新基本信息请求类型
export interface UpdateRiderApplicationBasicRequest {
    real_name?: string
    phone?: string
    gender?: string
    hometown?: string
    current_address?: string
    id_card_number?: string
    id_card_validity?: string
    address?: string
    address_detail?: string
    latitude?: number
    longitude?: number
    vehicle_type?: string
    available_time?: string
}

// OCR上传请求类型
export interface OCRUploadRequest {
    image_url: string                   // 图片本地路径 (filePath)
}

// 健康证上传请求类型
export interface HealthCertUploadRequest {
    image_url: string                   // 健康证图片本地路径
}

// 银行绑定相关类型
export interface RiderBindBankRequest {
    account_bank: string
    account_name: string
    account_number: string
    account_type: 'ACCOUNT_TYPE_PRIVATE'
    bank_address_code: string
    bank_name?: string
    contact_phone: string
}

export interface RiderBindBankResponse {
    applyment_id?: number
    message?: string
    status?: string
}

export interface RiderApplymentStatusResponse {
    status: 'pending' | 'approved' | 'rejected' | 'processing'
    status_desc?: string
    applyment_id?: number
    sub_mch_id?: string
    reject_reason?: string
    created_at?: string
    updated_at?: string
}

// ==================== 骑手申请管理接口 ====================

/**
 * 获取或创建骑手申请草稿
 */
export async function getRiderApplicationDraft(): Promise<RiderApplicationResponse> {
    const { data, error } = await supabase.from<RiderApplicationResponse>('rider_applications').select('*')
    if (error) throw error
    if (data && data.length > 0) return data[0]

    // 如果不存在，则创建
    const { data: newData, error: createError } = await supabase.from<RiderApplicationResponse>('rider_applications').insert({})
    if (createError) throw createError
    return newData![0]
}

/**
 * 更新骑手申请基本信息
 */
export async function updateRiderApplicationBasic(data: UpdateRiderApplicationBasicRequest): Promise<RiderApplicationResponse> {
    const draft = await getRiderApplicationDraft()
    const { data: updatedData, error } = await supabase.from<RiderApplicationResponse>('rider_applications')
        .update(data)
        .eq('id', draft.id)
    
    if (error) throw error
    return updatedData![0]
}

/**
 * 提交骑手申请
 */
export async function submitRiderApplication(): Promise<RiderApplicationResponse> {
    const draft = await getRiderApplicationDraft()
    const { data, error } = await supabase.rpc<RiderApplicationResponse>('submit_rider_application', {
        app_id: draft.id
    })
    
    if (error) throw error
    return data!
}

/**
 * 重置骑手申请
 */
export async function resetRiderApplication(): Promise<RiderApplicationResponse> {
    const draft = await getRiderApplicationDraft()
    const { data: updatedData, error } = await supabase.from<RiderApplicationResponse>('rider_applications')
        .update({ status: 'draft', reject_reason: null })
        .eq('id', draft.id)
    
    if (error) throw error
    return updatedData![0]
}

// ==================== OCR识别接口 ====================

/**
 * 身份证OCR识别
 * 执行流程：1. 上传图片至 Supabase Storage；2. 调用 ocr-service；3. 返回识别结果用于回填
 */
export async function recognizeRiderIDCard(data: OCRUploadRequest & { side?: 'Front' | 'Back' }): Promise<RiderApplicationResponse> {
    const filePath = data.image_url
    const side = data.side || 'Front'
    
    // 1. 上传图片 (filePath, url, name, formData)
    const uploadRes = await uploadFile(filePath, '', 'file', { category: 'identity' })
    
    const imageUrl = (uploadRes as any).url
    if (!imageUrl) throw new Error('Upload failed')

    // 2. 获取当前申请
    const draft = await getRiderApplicationDraft()

    // 3. 更新 URL 到数据库
    const urlField = side === 'Front' ? 'id_card_front_url' : 'id_card_back_url'
    await supabase.from('rider_applications').update({ [urlField]: imageUrl }).eq('id', draft.id)

    // 4. 手动调用 ocr-service 获取即时结果
    const { data: ocrRes, error } = await supabaseRequest<any>({
        url: '/functions/v1/ocr-service',
        method: 'POST',
        data: {
            application_id: draft.id,
            image_url: imageUrl,
            type: 'id_card',
            side,
            target_table: 'rider_applications'
        }
    })

    if (error) throw error
    
    // 返回最新的申请数据
    return await getRiderApplicationDraft()
}

/**
 * 上传健康证并 OCR
 */
export async function uploadHealthCert(data: HealthCertUploadRequest): Promise<RiderApplicationResponse> {
    const filePath = data.image_url
    
    // 1. 上传图片
    const uploadRes = await uploadFile(filePath, '', 'file', { category: 'identity' })
    
    const imageUrl = (uploadRes as any).url
    if (!imageUrl) throw new Error('Upload failed')

    // 2. 获取当前申请
    const draft = await getRiderApplicationDraft()

    // 3. 更新 URL
    await supabase.from('rider_applications').update({ health_cert_url: imageUrl }).eq('id', draft.id)

    // 4. 调用 ocr-service
    const { error } = await supabaseRequest<any>({
        url: '/functions/v1/ocr-service',
        method: 'POST',
        data: {
            application_id: draft.id,
            image_url: imageUrl,
            type: 'health_cert',
            target_table: 'rider_applications'
        }
    })

    if (error) throw error
    
    return await getRiderApplicationDraft()
}

// ==================== 银行绑定接口 (待后续迁移) ====================

export function bindRiderBank(data: RiderBindBankRequest): Promise<RiderBindBankResponse> {
    return Promise.resolve({})
}

export function getRiderApplymentStatus(): Promise<RiderApplymentStatusResponse> {
    return Promise.resolve({ status: 'processing' })
}

// ==================== 验证方法 ====================

export function validatePhoneNumber(phone: string): boolean {
    const phoneRegex = /^1[3-9]\d{9}$/
    return phoneRegex.test(phone)
}

export function validateIDCardNumber(idCard: string): boolean {
    const idCardRegex = /(^\d{15}$)|(^\d{18}$)|(^\d{17}(\d|X|x)$)/
    return idCardRegex.test(idCard)
}

export function validateRealName(name: string): boolean {
    if (!name || name.length < 2 || name.length > 50) return false
    const chineseNameRegex = /^[\u4e00-\u9fa5·]{2,50}$/
    return chineseNameRegex.test(name)
}

export function validateRiderApplicationForm(data: any): { isValid: boolean; errors: string[] } {
    const errors: string[] = []
    if (!data.real_name) errors.push('请填写真实姓名')
    else if (!validateRealName(data.real_name)) errors.push('真实姓名格式不正确')
    if (!data.phone) errors.push('请填写手机号')
    else if (!validatePhoneNumber(data.phone)) errors.push('手机号格式不正确')
    if (!data.id_card_front_url) errors.push('请上传身份证正面照片')
    if (!data.id_card_back_url) errors.push('请上传身份证背面照片')
    if (!data.health_cert_url) errors.push('请上传健康证照片')
    return { isValid: errors.length === 0, errors }
}

// ==================== 便捷流程管理类 ====================

export class RiderApplicationFlow {
    private draft: RiderApplicationResponse | null = null

    async initialize(): Promise<RiderApplicationResponse> {
        this.draft = await getRiderApplicationDraft()
        return this.draft
    }

    async uploadAndRecognizeIDCard(imageUrl: string, side: 'Front' | 'Back' = 'Front'): Promise<RiderApplicationResponse> {
        const result = await recognizeRiderIDCard({ image_url: imageUrl, side })
        this.draft = result

        if (result.id_card_ocr) {
            const ocrData = result.id_card_ocr
            const updateData: UpdateRiderApplicationBasicRequest = {}
            if (ocrData.name) updateData.real_name = ocrData.name
            if (Object.keys(updateData).length > 0) {
                this.draft = await updateRiderApplicationBasic(updateData)
            }
        }
        return this.draft
    }

    async uploadAndRecognizeHealthCert(imageUrl: string): Promise<RiderApplicationResponse> {
        this.draft = await uploadHealthCert({ image_url: imageUrl })
        return this.draft
    }

    async submit(): Promise<RiderApplicationResponse> {
        return await submitRiderApplication()
    }

    getCurrentDraft() { return this.draft }
}

export function createRiderApplicationFlow() { return new RiderApplicationFlow() }

export async function checkRiderApplicationStatus(): Promise<{ hasApplication: boolean; status?: string; canApply: boolean }> {
    try {
        const application = await getRiderApplicationDraft()
        return {
            hasApplication: !!application.id,
            status: application.status,
            canApply: !application.status || application.status === 'rejected'
        }
    } catch (error) {
        return { hasApplication: false, canApply: true }
    }
}

export function getApplicationStatusDescription(status: string): string {
    const statusMap: Record<string, string> = {
        'draft': '草稿中',
        'submitted': '审核中',
        'approved': '审核通过',
        'rejected': '审核拒绝'
    }
    return statusMap[status] || '未知状态'
}

export default {
    getRiderApplicationDraft,
    updateRiderApplicationBasic,
    submitRiderApplication,
    resetRiderApplication,
    recognizeRiderIDCard,
    uploadHealthCert,
    createRiderApplicationFlow,
    checkRiderApplicationStatus,
    getApplicationStatusDescription,
    validatePhoneNumber,
    validateIDCardNumber,
    validateRealName,
    validateRiderApplicationForm
}