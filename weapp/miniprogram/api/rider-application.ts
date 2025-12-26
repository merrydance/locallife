/**
 * 骑手入驻申请接口
 * 基于swagger.json完全重构，包含OCR识别和数据回填功能
 */

import { request, API_BASE } from '../utils/request'
import { getToken } from '../utils/auth'

// ==================== 数据类型定义 ====================

// 身份证OCR识别数据类型（复用通用类型）
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
    cert_number?: string                // 证书编号
    ocr_at?: string                     // OCR识别时间
    valid_end?: string                  // 有效期截止
    valid_start?: string                // 有效期起始
}

// 骑手申请数据类型
export interface RiderApplicationResponse {
    id?: number
    user_id?: number
    real_name?: string                  // 真实姓名
    phone?: string                      // 手机号
    id_card_front_url?: string          // 身份证正面图片URL
    id_card_back_url?: string           // 身份证背面图片URL
    health_cert_url?: string            // 健康证图片URL
    status?: string                     // 申请状态
    reject_reason?: string              // 拒绝原因
    created_at?: string                 // 创建时间
    updated_at?: string                 // 更新时间
    submitted_at?: string               // 提交时间
    // OCR识别结果
    id_card_ocr?: IDCardOCRData         // 身份证OCR识别结果
    health_cert_ocr?: HealthCertOCRData // 健康证OCR识别结果
}

// 更新基本信息请求类型
export interface UpdateRiderApplicationBasicRequest {
    real_name?: string                  // 真实姓名
    phone?: string                      // 手机号
}

// OCR上传请求类型
export interface OCRUploadRequest {
    image_url: string                   // 图片URL
}

// 健康证上传请求类型
export interface HealthCertUploadRequest {
    image_url: string                   // 健康证图片URL
}

// 银行绑定相关类型
/** 骑手银行绑定请求 - 对齐 api.riderBindBankRequest */
export interface RiderBindBankRequest {
    account_bank: string                // 开户银行
    account_name: string                // 账户名称
    account_number: string              // 银行账号
    account_type: 'ACCOUNT_TYPE_PRIVATE' // 账户类型（骑手都是个人，使用对私账户）
    bank_address_code: string           // 开户银行省市编码
    bank_name?: string                  // 开户银行全称
    contact_phone: string               // 联系手机号
}

/** 骑手银行绑定响应 - 对齐 api.riderBindBankResponse */
export interface RiderBindBankResponse {
    applyment_id?: number               // 申请ID
    message?: string                    // 消息
    status?: string                     // 状态
}

export interface RiderApplymentStatusResponse {
    status: 'pending' | 'approved' | 'rejected' | 'processing'
    status_desc?: string                // 状态描述
    applyment_id?: number               // 微信进件ID
    sub_mch_id?: string                 // 二级商户号（开户成功后返回）
    reject_reason?: string              // 拒绝原因
    created_at?: string
    updated_at?: string
}

// ==================== 骑手申请管理接口 ====================

/**
 * 获取或创建骑手申请草稿
 * 如果不存在则自动创建新的草稿
 */
export function getRiderApplicationDraft(): Promise<RiderApplicationResponse> {
    return request({
        url: '/v1/rider/application',
        method: 'GET'
    })
}

/**
 * 更新骑手申请基本信息
 */
export function updateRiderApplicationBasic(data: UpdateRiderApplicationBasicRequest): Promise<RiderApplicationResponse> {
    return request({
        url: '/v1/rider/application/basic',
        method: 'PUT',
        data
    })
}

/**
 * 提交骑手申请
 */
export function submitRiderApplication(): Promise<RiderApplicationResponse> {
    return request({
        url: '/v1/rider/application/submit',
        method: 'POST'
    })
}

/**
 * 重置骑手申请
 */
export function resetRiderApplication(): Promise<RiderApplicationResponse> {
    return request({
        url: '/v1/rider/application/reset',
        method: 'POST'
    })
}

// ==================== OCR识别接口 ====================

// Helper for multipart upload
function uploadRiderFile(url: string, filePath: string, formData: any = {}): Promise<RiderApplicationResponse> {
    return new Promise((resolve, reject) => {
        const token = getToken() // Assume getToken exists or imported from auth/request

        if (!filePath) {
            reject(new Error('File path is empty'))
            return
        }

        wx.uploadFile({
            url: `${API_BASE}${url}`,
            filePath: filePath,
            name: 'image',
            header: {
                'Authorization': `Bearer ${token}`
            },
            formData: formData,
            success: (res) => {
                if (res.statusCode === 200) {
                    try {
                        const data = JSON.parse(res.data)
                        if (data.code === 0 && data.data) {
                            resolve(data.data)
                        } else if (data.data) {
                            resolve(data.data)
                        } else {
                            // Support direct return if any
                            resolve(data as RiderApplicationResponse)
                        }
                    } catch (e) {
                        reject(new Error('Parse response failed'))
                    }
                } else {
                    reject(new Error(`HTTP ${res.statusCode}`))
                }
            },
            fail: (err) => {
                reject(err)
            }
        })
    })
}

/**
 * 身份证OCR识别
 * 自动识别姓名、身份证号、地址、性别、有效期等信息并回填到表单
 */
export function recognizeRiderIDCard(data: OCRUploadRequest & { side?: 'front' | 'back' }): Promise<RiderApplicationResponse> {
    // data.image_url is actually the filePath for upload here
    // But existing interface calls it image_url.
    // If the caller passes a local temp path (wxfile://), we upload it.
    // We should probably change the interface key to filePath to be clear, 
    // but to avoid breaking changes if it was used elsewhere, we treat image_url as path.
    const filePath = data.image_url
    const formData = { side: data.side || 'front' }
    return uploadRiderFile('/v1/rider/application/idcard/ocr', filePath, formData)
}

/**
 * 上传健康证
 * 上传健康证图片，后端会进行OCR识别
 */
export function uploadHealthCert(data: HealthCertUploadRequest): Promise<RiderApplicationResponse> {
    const filePath = data.image_url
    return uploadRiderFile('/v1/rider/application/healthcert', filePath)
}

// ==================== 银行绑定接口 ====================

/**
 * 绑定银行账户
 */
export function bindRiderBank(data: RiderBindBankRequest): Promise<RiderBindBankResponse> {
    return request({
        url: '/v1/rider/applyment/bindbank',
        method: 'POST',
        data
    })
}

/**
 * 获取申请状态
 */
export function getRiderApplymentStatus(): Promise<RiderApplymentStatusResponse> {
    return request({
        url: '/v1/rider/applyment/status',
        method: 'GET'
    })
}

// ==================== 便捷方法 ====================

/**
 * 完整的骑手入驻流程
 * 1. 获取或创建草稿
 * 2. 上传并识别证件
 * 3. 填写基本信息
 * 4. 提交申请
 */
export class RiderApplicationFlow {
    private draft: RiderApplicationResponse | null = null

    /**
     * 初始化申请流程
     */
    async initialize(): Promise<RiderApplicationResponse> {
        this.draft = await getRiderApplicationDraft()
        return this.draft
    }

    /**
     * 上传并识别身份证
     */
    async uploadAndRecognizeIDCard(imageUrl: string): Promise<RiderApplicationResponse> {
        const result = await recognizeRiderIDCard({ image_url: imageUrl })
        this.draft = result

        // 自动回填识别到的信息
        if (result.id_card_ocr) {
            const ocrData = result.id_card_ocr
            const updateData: UpdateRiderApplicationBasicRequest = {}

            if (ocrData.name) updateData.real_name = ocrData.name

            // 如果有识别到的信息，自动更新
            if (Object.keys(updateData).length > 0) {
                this.draft = await updateRiderApplicationBasic(updateData)
            }
        }

        return this.draft
    }

    /**
     * 上传并识别健康证
     */
    async uploadAndRecognizeHealthCert(imageUrl: string): Promise<RiderApplicationResponse> {
        const result = await uploadHealthCert({ image_url: imageUrl })
        this.draft = result
        return this.draft
    }

    /**
     * 更新基本信息
     */
    async updateBasicInfo(data: UpdateRiderApplicationBasicRequest): Promise<RiderApplicationResponse> {
        this.draft = await updateRiderApplicationBasic(data)
        return this.draft
    }

    /**
     * 提交申请
     */
    async submit(): Promise<RiderApplicationResponse> {
        const result = await submitRiderApplication()
        this.draft = result
        return result
    }

    /**
     * 获取当前草稿
     */
    getCurrentDraft(): RiderApplicationResponse | null {
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
            { field: 'real_name', name: '真实姓名' },
            { field: 'phone', name: '手机号' },
            { field: 'id_card_front_url', name: '身份证正面图片' },
            { field: 'id_card_back_url', name: '身份证背面图片' },
            { field: 'health_cert_url', name: '健康证图片' }
        ]

        const missingFields: string[] = []

        for (const { field, name } of requiredFields) {
            if (!this.draft[field as keyof RiderApplicationResponse]) {
                missingFields.push(name)
            }
        }

        return {
            isValid: missingFields.length === 0,
            missingFields
        }
    }

    /**
     * 获取OCR识别的信息摘要
     */
    getOCRSummary(): {
        idCard?: {
            name?: string
            idNumber?: string
            address?: string
            gender?: string
            validPeriod?: string
        }
        healthCert?: {
            certNumber?: string
            validPeriod?: string
        }
    } {
        if (!this.draft) return {}

        const summary: any = {}

        // 身份证信息摘要
        if (this.draft.id_card_ocr) {
            const ocr = this.draft.id_card_ocr
            summary.idCard = {
                name: ocr.name,
                idNumber: ocr.id_number,
                address: ocr.address,
                gender: ocr.gender,
                validPeriod: ocr.valid_start && ocr.valid_end
                    ? `${ocr.valid_start} 至 ${ocr.valid_end}`
                    : undefined
            }
        }

        // 健康证信息摘要
        if (this.draft.health_cert_ocr) {
            const ocr = this.draft.health_cert_ocr
            summary.healthCert = {
                certNumber: ocr.cert_number,
                validPeriod: ocr.valid_start && ocr.valid_end
                    ? `${ocr.valid_start} 至 ${ocr.valid_end}`
                    : undefined
            }
        }

        return summary
    }
}

/**
 * 创建骑手申请流程实例
 */
export function createRiderApplicationFlow(): RiderApplicationFlow {
    return new RiderApplicationFlow()
}

/**
 * 快速检查申请状态
 */
export async function checkRiderApplicationStatus(): Promise<{
    hasApplication: boolean
    status?: string
    canApply: boolean
}> {
    try {
        const application = await getRiderApplicationDraft()
        return {
            hasApplication: !!application.id,
            status: application.status,
            canApply: !application.status || application.status === 'rejected' // 没有申请或被拒绝后才能申请
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

/**
 * 获取银行绑定状态描述
 */
export function getBankBindingStatusDescription(status: string): string {
    const statusMap: Record<string, string> = {
        'pending': '待审核',
        'approved': '已通过',
        'rejected': '已拒绝',
        'processing': '处理中'
    }
    return statusMap[status] || '未知状态'
}

// ==================== 骑手申诉相关类型定义 ====================

/**
 * 创建骑手申诉请求 - 对齐 api.createRiderAppealRequest
 */
export interface CreateRiderAppealRequest extends Record<string, unknown> {
    claim_id: number                             // 索赔ID（必填，最小值1）
    evidence_urls?: string[]                     // 证据图片URL列表（最多10个）
    reason: string                               // 申诉原因（必填，10-1000字符）
}

/**
 * 验证手机号格式
 */
export function validatePhoneNumber(phone: string): boolean {
    const phoneRegex = /^1[3-9]\d{9}$/
    return phoneRegex.test(phone)
}

/**
 * 验证身份证号格式
 */
export function validateIDCardNumber(idCard: string): boolean {
    const idCardRegex = /(^\d{15}$)|(^\d{18}$)|(^\d{17}(\d|X|x)$)/
    return idCardRegex.test(idCard)
}

/**
 * 验证真实姓名格式
 */
export function validateRealName(name: string): boolean {
    if (!name || name.length < 2 || name.length > 50) {
        return false
    }
    // 中文姓名正则
    const chineseNameRegex = /^[\u4e00-\u9fa5·]{2,50}$/
    return chineseNameRegex.test(name)
}

/**
 * 完整的表单验证
 */
export function validateRiderApplicationForm(data: {
    real_name?: string
    phone?: string
    id_card_front_url?: string
    id_card_back_url?: string
    health_cert_url?: string
}): { isValid: boolean; errors: string[] } {
    const errors: string[] = []

    if (!data.real_name) {
        errors.push('请填写真实姓名')
    } else if (!validateRealName(data.real_name)) {
        errors.push('真实姓名格式不正确')
    }

    if (!data.phone) {
        errors.push('请填写手机号')
    } else if (!validatePhoneNumber(data.phone)) {
        errors.push('手机号格式不正确')
    }

    if (!data.id_card_front_url) {
        errors.push('请上传身份证正面照片')
    }

    if (!data.id_card_back_url) {
        errors.push('请上传身份证背面照片')
    }

    if (!data.health_cert_url) {
        errors.push('请上传健康证照片')
    }

    return {
        isValid: errors.length === 0,
        errors
    }
}

// 兼容性导出
export default {
    // 申请管理
    getRiderApplicationDraft,
    updateRiderApplicationBasic,
    submitRiderApplication,
    resetRiderApplication,

    // OCR识别
    recognizeRiderIDCard,
    uploadHealthCert,

    // 银行绑定
    bindRiderBank,
    getRiderApplymentStatus,

    // 便捷方法
    createRiderApplicationFlow,
    checkRiderApplicationStatus,
    getApplicationStatusDescription,
    getBankBindingStatusDescription,

    // 验证方法
    validatePhoneNumber,
    validateIDCardNumber,
    validateRealName,
    validateRiderApplicationForm
}