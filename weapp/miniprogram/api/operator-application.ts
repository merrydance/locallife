/**
 * 运营商入驻申请接口
 * 基于swagger.json完全重构，包含OCR识别和数据回填功能
 */

import { request, API_BASE } from '../utils/request'
import { getToken } from '../utils/auth'

// ==================== 数据类型定义 ====================

// 运营商身份证正面OCR识别数据类型
export interface OperatorIDCardOCRData {
    address?: string                    // 地址
    gender?: string                     // 性别
    id_number?: string                  // 身份证号
    name?: string                       // 姓名
    nation?: string                     // 民族
    ocr_at?: string                     // OCR识别时间
}

// 运营商身份证背面OCR识别数据类型
export interface OperatorIDCardBackOCR {
    ocr_at?: string                     // OCR识别时间
    valid_end?: string                  // 有效期截止
    valid_start?: string                // 有效期起始
}

// 营业执照OCR识别数据类型（企业运营商使用）- 对齐 api.BusinessLicenseOCRData
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

// 运营商申请响应数据类型 - 完全对齐swagger定义
export interface OperatorApplicationResponse {
    id: number                                    // 申请ID
    user_id: number                              // 用户ID
    name?: string                                // 运营商名称
    contact_name?: string                        // 联系人姓名
    contact_phone?: string                       // 联系人电话
    legal_person_name?: string                   // 法人姓名
    legal_person_id_number?: string              // 法人身份证号
    region_id?: number                           // 申请区域ID
    region_name?: string                         // 区域名称
    requested_contract_years?: number            // 申请合同年限
    status: string                               // 申请状态: pending/reviewing/approved/rejected
    id_card_front_url?: string                   // 身份证正面图片URL
    id_card_back_url?: string                    // 身份证背面图片URL
    business_license_url?: string                // 营业执照图片URL（企业运营商）
    business_license_number?: string             // 营业执照号码
    id_card_front_ocr?: OperatorIDCardOCRData    // 身份证正面OCR数据
    id_card_back_ocr?: OperatorIDCardBackOCR     // 身份证背面OCR数据
    business_license_ocr?: BusinessLicenseOCRData // 营业执照OCR数据
    submitted_at?: string                        // 提交时间
    reviewed_at?: string                         // 审核时间
    reject_reason?: string                       // 拒绝原因
    created_at: string                           // 创建时间
    updated_at: string                           // 更新时间
}

// 创建运营商申请请求
export interface CreateOperatorApplicationRequest extends Record<string, unknown> {
    region_id: number                            // 申请区域ID（必填）
}

// 更新运营商申请基本信息请求
export interface UpdateOperatorApplicationBasicRequest extends Record<string, unknown> {
    name?: string                                // 运营商名称 (2-50字符)
    contact_name?: string                        // 联系人姓名 (2-20字符)
    contact_phone?: string                       // 联系人电话
    requested_contract_years?: number            // 申请合同年限 (1-10年)
}

// 更新运营商申请区域请求
export interface UpdateOperatorApplicationRegionRequest extends Record<string, unknown> {
    region_id: number                            // 区域ID（必填）
}

// 运营商银行绑定请求
export interface OperatorBindBankRequest extends Record<string, unknown> {
    account_bank: string                         // 开户银行（必填，最多128字符）
    account_name: string                         // 开户名称（必填，最多128字符）
    account_number: string                       // 银行账号（必填）
    account_type: 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'  // 账户类型（必填）
    bank_address_code: string                    // 开户银行省市编码（必填）
    contact_phone: string                        // 联系电话（必填）
    bank_name?: string                           // 开户银行全称（支行）
    contact_email?: string                       // 联系邮箱（可选）
}

// 运营商银行绑定响应
export interface OperatorBindBankResponse {
    applyment_id?: number                        // 微信申请单号
    status?: string                              // 状态
    message?: string                             // 消息
}

// 运营商申请状态响应 - 对齐 api.operatorApplymentStatusResponse
export interface OperatorApplymentStatusResponse {
    applyment_id?: number                        // 微信进件ID
    created_at?: string                          // 创建时间
    reject_reason?: string                       // 拒绝原因
    sign_url?: string                            // 签约链接
    status?: string                              // 状态
    status_desc?: string                         // 状态描述
    sub_mch_id?: string                          // 二级商户号（开户成功后返回）
    updated_at?: string                          // 更新时间
}

// ==================== 文件上传辅助函数 ====================

/**
 * 运营商证照文件上传（multipart/form-data）
 * 符合后端 certificate_upload_guide.md 规范
 * @param url API路径
 * @param filePath 本地文件路径
 * @param formData 附加表单数据（如 side 参数）
 */
function uploadOperatorFile(url: string, filePath: string, formData: Record<string, string> = {}): Promise<OperatorApplicationResponse> {
    return new Promise((resolve, reject) => {
        const token = getToken()

        if (!filePath) {
            reject(new Error('文件路径不能为空'))
            return
        }

        wx.uploadFile({
            url: `${API_BASE}${url}`,
            filePath: filePath,
            name: 'image',  // 关键：字段名必须是 image
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
                            resolve(data as OperatorApplicationResponse)
                        }
                    } catch (e) {
                        reject(new Error('解析响应失败'))
                    }
                } else {
                    try {
                        const errData = JSON.parse(res.data)
                        reject(new Error(errData.message || `HTTP ${res.statusCode}`))
                    } catch {
                        reject(new Error(`HTTP ${res.statusCode}`))
                    }
                }
            },
            fail: (err) => {
                reject(err)
            }
        })
    })
}

// ==================== 接口服务类 ====================

export class OperatorApplicationService {

    /**
     * 获取或创建运营商申请
     * GET/POST /v1/operator/application
     */
    static async getOrCreateApplication(regionId?: number): Promise<OperatorApplicationResponse> {
        if (regionId) {
            // 创建新申请
            const request_data: CreateOperatorApplicationRequest = { region_id: regionId }
            return await request({
                url: '/v1/operator/application',
                method: 'POST',
                data: request_data
            })
        } else {
            // 获取现有申请
            return await request({
                url: '/v1/operator/application',
                method: 'GET'
            })
        }
    }

    /**
     * 更新运营商申请基本信息
     * PUT /v1/operator/application/basic
     */
    static async updateBasicInfo(data: UpdateOperatorApplicationBasicRequest): Promise<OperatorApplicationResponse> {
        return await request({
            url: '/v1/operator/application/basic',
            method: 'PUT',
            data
        })
    }

    /**
     * 更新运营商申请区域
     * PUT /v1/operator/application/region
     */
    static async updateRegion(data: UpdateOperatorApplicationRegionRequest): Promise<OperatorApplicationResponse> {
        return await request({
            url: '/v1/operator/application/region',
            method: 'PUT',
            data
        })
    }

    /**
     * 身份证OCR识别（正面或背面）
     * POST /v1/operator/application/idcard/ocr
     * 使用 multipart/form-data 上传图片文件
     * @param filePath 本地文件路径（wxfile:// 或 http://tmp/...）
     * @param side 正面 "Front" 或背面 "Back"
     */
    static async recognizeIDCard(filePath: string, side: 'Front' | 'Back'): Promise<OperatorApplicationResponse> {
        return uploadOperatorFile('/v1/operator/application/idcard/ocr', filePath, { side })
    }

    /**
     * 营业执照OCR识别（企业运营商）
     * POST /v1/operator/application/license/ocr
     * 使用 multipart/form-data 上传图片文件
     * @param filePath 本地文件路径
     */
    static async recognizeBusinessLicense(filePath: string): Promise<OperatorApplicationResponse> {
        return uploadOperatorFile('/v1/operator/application/license/ocr', filePath)
    }


    /**
     * 提交运营商申请
     * POST /v1/operator/application/submit
     */
    static async submitApplication(): Promise<OperatorApplicationResponse> {
        return await request({
            url: '/v1/operator/application/submit',
            method: 'POST'
        })
    }

    /**
     * 重置运营商申请
     * POST /v1/operator/application/reset
     */
    static async resetApplication(): Promise<OperatorApplicationResponse> {
        return await request({
            url: '/v1/operator/application/reset',
            method: 'POST'
        })
    }

    /**
     * 绑定银行账户
     * POST /v1/operator/applyment/bindbank
     */
    static async bindBank(data: OperatorBindBankRequest): Promise<OperatorBindBankResponse> {
        return await request({
            url: '/v1/operator/applyment/bindbank',
            method: 'POST',
            data
        })
    }

    /**
     * 获取申请状态
     * GET /v1/operator/applyment/status
     */
    static async getApplymentStatus(): Promise<OperatorApplymentStatusResponse> {
        return await request({
            url: '/v1/operator/applyment/status',
            method: 'GET'
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 运营商申请数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
export class OperatorApplicationAdapter {

    /**
     * 将身份证OCR数据自动回填到表单
     */
    static fillIDCardData(ocrData: OperatorIDCardOCRData): {
        legal_person_name?: string
        legal_person_id_number?: string
        contact_name?: string
    } {
        return {
            legal_person_name: ocrData.name,
            legal_person_id_number: ocrData.id_number,
            contact_name: ocrData.name  // 默认联系人为法人
        }
    }

    /**
     * 将营业执照OCR数据自动回填到表单
     */
    static fillBusinessLicenseData(ocrData: BusinessLicenseOCRData): {
        name?: string
        legal_person_name?: string
        business_license_number?: string
    } {
        return {
            name: ocrData.enterprise_name,
            legal_person_name: ocrData.legal_representative,
            business_license_number: ocrData.credit_code || ocrData.reg_num
        }
    }

    /**
     * 格式化申请状态显示文本
     */
    static formatStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待提交',
            'reviewing': '审核中',
            'approved': '已通过',
            'rejected': '已拒绝'
        }
        return statusMap[status] || status
    }

    /**
     * 格式化账户类型显示文本
     */
    static formatAccountType(accountType: string): string {
        const typeMap: Record<string, string> = {
            'ACCOUNT_TYPE_BUSINESS': '对公账户',
            'ACCOUNT_TYPE_PRIVATE': '个人账户'
        }
        return typeMap[accountType] || accountType
    }

    /**
     * 验证申请数据完整性
     */
    static validateApplicationData(data: OperatorApplicationResponse): {
        isValid: boolean
        missingFields: string[]
    } {
        const requiredFields = [
            { key: 'name', label: '运营商名称' },
            { key: 'contact_name', label: '联系人姓名' },
            { key: 'contact_phone', label: '联系人电话' },
            { key: 'legal_person_name', label: '法人姓名' },
            { key: 'legal_person_id_number', label: '法人身份证号' },
            { key: 'region_id', label: '申请区域' },
            { key: 'requested_contract_years', label: '合同年限' },
            { key: 'id_card_front_url', label: '身份证正面' },
            { key: 'id_card_back_url', label: '身份证背面' }
        ]

        const missingFields: string[] = []

        requiredFields.forEach(field => {
            if (!data[field.key as keyof OperatorApplicationResponse]) {
                missingFields.push(field.label)
            }
        })

        return {
            isValid: missingFields.length === 0,
            missingFields
        }
    }
}

// ==================== 区域管理相关接口 ====================

// 区域响应数据类型
export interface RegionResponse {
    id: number                                   // 区域ID
    name: string                                 // 区域名称
    code?: string                                // 区域编码
    level: number                                // 区域层级（1=省 2=市 3=区县）
    parent_id?: number                           // 上级区域ID
    latitude?: string                            // 纬度
    longitude?: string                           // 经度
}

// 区域查询参数
export interface RegionQueryParams extends Record<string, unknown> {
    level?: number                               // 区域层级（1=省 2=市 3=区县）
    parent_id?: number                           // 上级区域ID
    page_id: number                              // 页码（必填）
    page_size: number                            // 每页数量（必填，5-100）
}

// 区域搜索参数
export interface RegionSearchParams extends Record<string, unknown> {
    name: string                                 // 搜索关键词（必填）
    level?: number                               // 区域层级
    page_id: number                              // 页码（必填）
    page_size: number                            // 每页数量（必填，5-100）
}

/**
 * 区域管理服务类
 * 为运营商申请提供区域选择功能
 */
export class RegionService {

    /**
     * 获取区域列表
     * GET /v1/regions
     */
    static async getRegions(params: RegionQueryParams): Promise<RegionResponse[]> {
        return await request({
            url: '/v1/regions',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取可申请的区县列表（未被运营商绑定的区域）
     * GET /v1/regions/available
     */
    static async getAvailableRegions(params: RegionQueryParams): Promise<RegionResponse[]> {
        return await request({
            url: '/v1/regions/available',
            method: 'GET',
            data: params
        })
    }

    /**
     * 搜索区域
     * GET /v1/regions/search
     */
    static async searchRegions(params: RegionSearchParams): Promise<RegionResponse[]> {
        return await request({
            url: '/v1/regions/search',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取区域详情
     * GET /v1/regions/{id}
     */
    static async getRegionById(id: number): Promise<RegionResponse> {
        return await request({
            url: `/v1/regions/${id}`,
            method: 'GET'
        })
    }

    /**
     * 检查区域是否可申请
     * GET /v1/regions/{id}/check
     */
    static async checkRegionAvailable(id: number): Promise<{ available: boolean; message?: string }> {
        return await request({
            url: `/v1/regions/${id}/check`,
            method: 'GET'
        })
    }

    /**
     * 获取区域的下级区域列表
     * GET /v1/regions/{id}/children
     */
    static async getRegionChildren(id: number): Promise<RegionResponse[]> {
        return await request({
            url: `/v1/regions/${id}/children`,
            method: 'GET'
        })
    }
}

// ==================== 运营商申请流程辅助类 ====================

/**
 * 运营商申请流程管理器
 * 提供完整的申请流程支持
 */
export class OperatorApplicationFlow {

    /**
     * 获取省份列表
     */
    static async getProvinces(): Promise<RegionResponse[]> {
        return await RegionService.getRegions({
            level: 1,
            page_id: 1,
            page_size: 50
        })
    }

    /**
     * 获取城市列表
     */
    static async getCities(provinceId: number): Promise<RegionResponse[]> {
        return await RegionService.getRegions({
            level: 2,
            parent_id: provinceId,
            page_id: 1,
            page_size: 100
        })
    }

    /**
     * 获取可申请的区县列表
     */
    static async getAvailableDistricts(cityId?: number): Promise<RegionResponse[]> {
        const params: RegionQueryParams = {
            level: 3,
            page_id: 1,
            page_size: 100
        }

        if (cityId) {
            params.parent_id = cityId
        }

        return await RegionService.getAvailableRegions(params)
    }

    /**
     * 完整的运营商申请流程
     * 1. 选择区域并创建申请
     * 2. 上传并识别证件
     * 3. 填写基本信息
     * 4. 提交申请
     */
    static async createApplicationWithRegion(regionId: number): Promise<OperatorApplicationResponse> {
        // 1. 检查区域是否可申请
        const checkResult = await RegionService.checkRegionAvailable(regionId)
        if (!checkResult.available) {
            throw new Error(checkResult.message || '该区域不可申请')
        }

        // 2. 创建申请
        return await OperatorApplicationService.getOrCreateApplication(regionId)
    }

    /**
     * 自动填充OCR识别的数据
     * 注意：参数现在是本地文件路径，而非URL
     * @param idCardFilePath 身份证正面本地文件路径
     * @param businessLicenseFilePath 营业执照本地文件路径（可选）
     */
    static async autoFillFromOCR(
        idCardFilePath: string,
        businessLicenseFilePath?: string
    ): Promise<{
        idCardData?: OperatorIDCardOCRData
        businessLicenseData?: BusinessLicenseOCRData
        suggestedFormData: Partial<UpdateOperatorApplicationBasicRequest>
    }> {
        const results: any = {}

        // 识别身份证正面
        if (idCardFilePath) {
            const idCardResult = await OperatorApplicationService.recognizeIDCard(idCardFilePath, 'Front')
            results.idCardData = idCardResult.id_card_front_ocr
        }

        // 识别营业执照（如果是企业运营商）
        if (businessLicenseFilePath) {
            const licenseResult = await OperatorApplicationService.recognizeBusinessLicense(businessLicenseFilePath)
            results.businessLicenseData = licenseResult.business_license_ocr
        }

        // 生成建议的表单数据
        const suggestedFormData: Partial<UpdateOperatorApplicationBasicRequest> = {}

        if (results.idCardData) {
            const idCardFill = OperatorApplicationAdapter.fillIDCardData(results.idCardData)
            Object.assign(suggestedFormData, idCardFill)
        }

        if (results.businessLicenseData) {
            const licenseFill = OperatorApplicationAdapter.fillBusinessLicenseData(results.businessLicenseData)
            Object.assign(suggestedFormData, licenseFill)
        }

        return {
            ...results,
            suggestedFormData
        }
    }
}

// ==================== 导出默认服务 ====================

export default OperatorApplicationService