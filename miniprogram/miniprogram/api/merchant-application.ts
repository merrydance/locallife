/**
 * 商户入驻申请接口
 * 基于swagger.json完全重构，包含OCR识别和数据回填功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

// OCR识别数据类型
export interface BusinessLicenseOCRData {
    address?: string                    // 地址
    business_scope?: string             // 经营范围
    credit_code?: string                // 统一社会信用代码
    enterprise_name?: string            // 企业名称
    legal_representative?: string       // 法定代表人
    ocr_at?: string                     // OCR识别时间
    reg_num?: string                    // 注册号
    registered_capital?: string         // 注册资本
    type_of_enterprise?: string         // 类型
    valid_period?: string               // 营业期限
}

export interface MerchantIDCardOCRData {
    address?: string                    // 地址
    gender?: string                     // 性别
    id_number?: string                  // 身份证号
    name?: string                       // 姓名
    nation?: string                     // 民族
    ocr_at?: string                     // OCR识别时间
    valid_date?: string                 // 有效期（背面）
}

export interface FoodPermitOCRData {
    company_name?: string               // 企业名称
    ocr_at?: string                     // OCR识别时间
    permit_no?: string                  // 许可证编号
    raw_text?: string                   // 原始OCR文本
    valid_from?: string                 // 有效期起
    valid_to?: string                   // 有效期止
}

// 商户申请草稿数据类型
export interface MerchantApplicationDraftResponse {
    id?: string
    user_id?: string
    merchant_name?: string
    business_address?: string
    business_license_number?: string
    business_license_image_url?: string
    business_scope?: string
    contact_phone?: string
    legal_person_name?: string
    legal_person_id_number?: string
    legal_person_id_front_url?: string
    legal_person_id_back_url?: string
    food_permit_url?: string
    latitude?: string
    longitude?: string
    region_id?: number
    status?: string
    reject_reason?: string | null
    created_at?: string
    updated_at?: string
    // OCR识别结果
    business_license_ocr?: BusinessLicenseOCRData
    id_card_front_ocr?: MerchantIDCardOCRData
    id_card_back_ocr?: MerchantIDCardOCRData
    food_permit_ocr?: FoodPermitOCRData
}

// 商户申请正式数据类型
/** 商户申请响应 - 对齐 api.merchantApplicationResponse */
export interface MerchantApplicationResponse {
    business_address?: string
    business_license_image_url?: string
    business_license_number?: string
    business_scope?: string
    contact_phone?: string
    created_at?: string
    id?: string
    legal_person_id_back_url?: string
    legal_person_id_front_url?: string
    legal_person_id_number?: string
    legal_person_name?: string
    merchant_name?: string
    reject_reason?: string | null
    reviewed_at?: string                         // 审核时间
    reviewed_by?: string                         // 审核人ID
    status?: string
    updated_at?: string
    user_id?: string                             // 用户ID
}

// 创建商户申请请求类型
export interface CreateMerchantApplicationRequest {
    merchant_name: string               // 商户名称（自定义）
    business_address: string            // 经营地址
    business_license_number: string     // 统一社会信用代码或注册号
    business_license_image_url: string  // 营业执照图片URL
    business_scope?: string             // 经营范围
    contact_phone: string               // 联系电话
    legal_person_name: string           // 法定代表人姓名
    legal_person_id_number: string      // 法定代表人身份证号
    legal_person_id_front_url: string   // 身份证正面图片URL
    legal_person_id_back_url: string    // 身份证背面图片URL
    latitude: string                    // 纬度
    longitude: string                   // 经度
    region_id: number                   // 区域ID
}

// 更新基本信息请求类型
export interface UpdateMerchantApplicationBasicRequest {
    merchant_name?: string
    business_address?: string
    business_license_number?: string
    business_license_image_url?: string
    business_scope?: string
    contact_phone?: string
    legal_person_name?: string
    legal_person_id_number?: string
    legal_person_id_front_url?: string
    legal_person_id_back_url?: string
    food_permit_url?: string
    latitude?: string
    longitude?: string
    region_id?: number
}

// OCR上传请求类型
export interface OCRUploadRequest {
    image_url: string                   // 图片URL
}

// 银行绑定相关类型
export interface MerchantBankBindRequest {
    account_name: string                // 账户名
    account_number: string              // 账户号
    bank_name: string                   // 银行名称
    bank_code?: string                  // 银行代码
}

/** 商户申请状态响应 - 对齐 api.merchantApplymentStatusResponse */
export interface MerchantApplymentStatusResponse {
    reject_reason?: string                       // 拒绝原因
    sign_url?: string                            // 签约链接
    status?: string                              // 状态
    status_desc?: string                         // 状态描述
    sub_mch_id?: string                          // 二级商户号
}

import { supabase, supabaseRequest } from '../services/supabase'
import { uploadFile } from '../utils/request'

// ... (types 保持不变) ...

// ==================== 商户申请管理接口 ====================

/**
 * 获取或创建商户申请草稿
 */
export async function getMerchantApplicationDraft(): Promise<MerchantApplicationDraftResponse> {
    const { data, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications').select('*')
    if (error) throw error
    if (data && data.length > 0) return data[0]

    // 如果不存在，则创建
    const { data: newData, error: createError } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications').insert({})
    if (createError) throw createError
    return newData![0]
}

/**
 * 更新商户申请基本信息
 */
export async function updateMerchantApplicationBasic(data: UpdateMerchantApplicationBasicRequest): Promise<MerchantApplicationDraftResponse> {
    const draft = await getMerchantApplicationDraft()
    const { data: updatedData, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
        .update(data)
        .eq('id', draft.id!)
    
    if (error) throw error
    return updatedData![0]
}

/**
 * 提交商户申请
 */
export async function submitMerchantApplication(): Promise<MerchantApplicationDraftResponse> {
    const draft = await getMerchantApplicationDraft()
    const { data, error } = await supabase.rpc<MerchantApplicationDraftResponse>('submit_merchant_application', {
        app_id: draft.id
    })
    
    if (error) throw error
    return data!
}

/**
 * 重置商户申请
 */
export async function resetMerchantApplication(): Promise<MerchantApplicationDraftResponse> {
    const draft = await getMerchantApplicationDraft()
    const { data: updatedData, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
        .update({ status: 'draft', reject_reason: null })
        .eq('id', draft.id!)
    
    if (error) throw error
    return updatedData![0]
}

/**
 * 获取我的商户申请状态
 */
export async function getMyMerchantApplication(): Promise<MerchantApplicationResponse> {
    const { data, error } = await supabase.from<MerchantApplicationResponse>('merchant_applications').select('*').single()
    if (error) throw error
    return data!
}

/**
 * 创建商户申请（正式提交）
 */
export async function createMerchantApplication(data: CreateMerchantApplicationRequest): Promise<MerchantApplicationResponse> {
    // 逻辑类似于 submit，但这里支持传全量数据创建
    const { data: res, error } = await supabase.from<MerchantApplicationResponse>('merchant_applications').insert(data)
    if (error) throw error
    
    // 插入后尝试执行一次审核
    return await submitMerchantApplication() as any
}

// ==================== OCR识别辅助 ====================

async function performOCR(filePath: string, type: string, side?: string) {
    // 1. 上传图片至 bucket: identity
    const uploadRes = await uploadFile(filePath, '', 'file', { category: 'identity' })
    const imageUrl = (uploadRes as any).url
    if (!imageUrl) throw new Error('Image upload failed')

    // 2. 获取当前申请单 ID
    const draft = await getMerchantApplicationDraft()

    // 3. 更新图片 URL
    const urlFieldMap: Record<string, string> = {
        'business_license': 'business_license_image_url',
        'food_permit': 'food_permit_url',
        'id_card_Front': 'legal_person_id_front_url',
        'id_card_Back': 'legal_person_id_back_url'
    }
    const fieldKey = type === 'id_card' ? `${type}_${side}` : type
    const targetField = urlFieldMap[fieldKey]
    if (targetField) {
        await supabase.from('merchant_applications').update({ [targetField]: imageUrl }).eq('id', draft.id!)
    }

    // 4. 调用 ocr-service
    await supabaseRequest<any>({
        url: '/functions/v1/ocr-service',
        method: 'POST',
        data: {
            application_id: draft.id,
            image_url: imageUrl,
            type,
            side,
            target_table: 'merchant_applications'
        }
    })

    return await getMerchantApplicationDraft()
}

// ==================== OCR识别接口 ====================

/**
 * 身份证正面OCR识别
 */
export async function recognizeIDCardFront(data: OCRUploadRequest): Promise<MerchantApplicationDraftResponse> {
    return await performOCR(data.image_url, 'id_card', 'Front')
}

/**
 * 营业执照OCR识别
 */
export async function recognizeBusinessLicense(data: OCRUploadRequest): Promise<MerchantApplicationDraftResponse> {
    return await performOCR(data.image_url, 'business_license')
}

/**
 * 食品经营许可证OCR识别
 */
export async function recognizeFoodPermit(data: OCRUploadRequest): Promise<MerchantApplicationDraftResponse> {
    return await performOCR(data.image_url, 'food_permit')
}

// ==================== 银行绑定接口 ====================

/**
 * 绑定银行账户
 */
export function bindMerchantBank(data: MerchantBankBindRequest): Promise<any> {
    // 后续迁移，暂时留空
    return Promise.resolve({})
}

/**
 * 获取申请状态
 */
export async function getMerchantApplymentStatus(): Promise<MerchantApplymentStatusResponse> {
    const draft = await getMerchantApplicationDraft()
    return { status: draft.status, status_desc: draft.status } as any
}

// ==================== 便捷方法 ====================

/**
 * 完整的商户入驻流程
 * 1. 获取或创建草稿
 * 2. 上传并识别证件
 * 3. 填写基本信息
 * 4. 提交申请
 */
export class MerchantApplicationFlow {
    private draft: MerchantApplicationDraftResponse | null = null

    /**
     * 初始化申请流程
     */
    async initialize(): Promise<MerchantApplicationDraftResponse> {
        this.draft = await getMerchantApplicationDraft()
        return this.draft
    }

    /**
     * 上传并识别身份证正面
     */
    async uploadAndRecognizeIDCard(imageUrl: string): Promise<MerchantApplicationDraftResponse> {
        const result = await recognizeIDCardFront({ image_url: imageUrl })
        this.draft = result

        // 自动回填识别到的信息
        if (result.id_card_front_ocr) {
            const ocrData = result.id_card_front_ocr
            const updateData: UpdateMerchantApplicationBasicRequest = {}

            if (ocrData.name) updateData.legal_person_name = ocrData.name
            if (ocrData.id_number) updateData.legal_person_id_number = ocrData.id_number
            if (ocrData.address) updateData.business_address = ocrData.address

            // 如果有识别到的信息，自动更新
            if (Object.keys(updateData).length > 0) {
                this.draft = await updateMerchantApplicationBasic(updateData)
            }
        }

        return this.draft
    }

    /**
     * 上传并识别营业执照
     */
    async uploadAndRecognizeBusinessLicense(imageUrl: string): Promise<MerchantApplicationDraftResponse> {
        const result = await recognizeBusinessLicense({ image_url: imageUrl })
        this.draft = result

        // 自动回填识别到的信息
        if (result.business_license_ocr) {
            const ocrData = result.business_license_ocr
            const updateData: UpdateMerchantApplicationBasicRequest = {}

            if (ocrData.enterprise_name) {
                // 营业执照上的企业名称作为默认商户名称，用户可以自定义修改
                updateData.merchant_name = ocrData.enterprise_name
            }
            if (ocrData.credit_code) updateData.business_license_number = ocrData.credit_code
            if (ocrData.business_scope) updateData.business_scope = ocrData.business_scope
            if (ocrData.legal_representative) updateData.legal_person_name = ocrData.legal_representative
            if (ocrData.address) updateData.business_address = ocrData.address

            // 如果有识别到的信息，自动更新
            if (Object.keys(updateData).length > 0) {
                this.draft = await updateMerchantApplicationBasic(updateData)
            }
        }

        return this.draft
    }

    /**
     * 上传并识别食品经营许可证
     */
    async uploadAndRecognizeFoodPermit(imageUrl: string): Promise<MerchantApplicationDraftResponse> {
        const result = await recognizeFoodPermit({ image_url: imageUrl })
        this.draft = result
        return this.draft
    }

    /**
     * 更新基本信息
     */
    async updateBasicInfo(data: UpdateMerchantApplicationBasicRequest): Promise<MerchantApplicationDraftResponse> {
        this.draft = await updateMerchantApplicationBasic(data)
        return this.draft
    }

    /**
     * 提交申请
     */
    async submit(): Promise<MerchantApplicationDraftResponse> {
        const result = await submitMerchantApplication()
        this.draft = result
        return result
    }

    /**
     * 获取当前草稿
     */
    getCurrentDraft(): MerchantApplicationDraftResponse | null {
        return this.draft
    }

    /**
     * 验证申请信息是否完整
     */
    validateApplication(): { isValid: boolean; missingFields: string[] } {
        if (!this.draft) {
            return { isValid: false, missingFields: ['申请草稿未初始化'] }
        }

        const requiredFields = [
            { field: 'merchant_name', name: '商户名称' },
            { field: 'business_address', name: '经营地址' },
            { field: 'business_license_number', name: '营业执照号码' },
            { field: 'business_license_image_url', name: '营业执照图片' },
            { field: 'contact_phone', name: '联系电话' },
            { field: 'legal_person_name', name: '法定代表人姓名' },
            { field: 'legal_person_id_number', name: '法定代表人身份证号' },
            { field: 'legal_person_id_front_url', name: '身份证正面图片' },
            { field: 'legal_person_id_back_url', name: '身份证背面图片' },
            { field: 'latitude', name: '纬度' },
            { field: 'longitude', name: '经度' },
            { field: 'region_id', name: '区域ID' }
        ]

        const missingFields: string[] = []

        for (const { field, name } of requiredFields) {
            if (!this.draft[field as keyof MerchantApplicationDraftResponse]) {
                missingFields.push(name)
            }
        }

        return {
            isValid: missingFields.length === 0,
            missingFields
        }
    }
}

/**
 * 创建商户申请流程实例
 */
export function createMerchantApplicationFlow(): MerchantApplicationFlow {
    return new MerchantApplicationFlow()
}

/**
 * 快速检查申请状态
 */
export async function checkMerchantApplicationStatus(): Promise<{
    hasApplication: boolean
    status?: string
    canApply: boolean
}> {
    try {
        const application = await getMyMerchantApplication()
        return {
            hasApplication: true,
            status: application.status,
            canApply: application.status === 'rejected' // 只有被拒绝后才能重新申请
        }
    } catch (error) {
        // 如果没有申请记录，返回可以申请
        return {
            hasApplication: false,
            canApply: true
        }
    }
}

/**
 * 获取申请进度描述
 */
export function getApplicationStatusDescription(status: string): string {
    const statusMap: Record<string, string> = {
        'draft': '草稿中',
        'pending': '审核中',
        'approved': '审核通过',
        'rejected': '审核拒绝',
        'processing': '处理中'
    }
    return statusMap[status] || '未知状态'
}

// ==================== 商户申诉相关类型定义 ====================

/**
 * 创建商户申诉请求 - 对齐 api.createMerchantAppealRequest
 */
export interface CreateMerchantAppealRequest extends Record<string, unknown> {
    claim_id: number                             // 索赔ID（必填，最小值1）
    evidence_urls?: string[]                     // 证据图片URL列表（最多10个）
    reason: string                               // 申诉原因（必填，10-1000字符）
}

// 兼容性导出
export default {
    // 申请管理
    getMerchantApplicationDraft,
    updateMerchantApplicationBasic,
    submitMerchantApplication,
    resetMerchantApplication,
    getMyMerchantApplication,
    createMerchantApplication,

    // OCR识别
    recognizeIDCardFront,
    recognizeBusinessLicense,
    recognizeFoodPermit,

    // 银行绑定
    bindMerchantBank,
    getMerchantApplymentStatus,

    // 便捷方法
    createMerchantApplicationFlow,
    checkMerchantApplicationStatus,
    getApplicationStatusDescription
}