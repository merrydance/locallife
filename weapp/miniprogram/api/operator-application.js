"use strict";
/**
 * 运营商入驻申请接口
 * 基于swagger.json完全重构，包含OCR识别和数据回填功能
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.OperatorApplicationFlow = exports.RegionService = exports.OperatorApplicationAdapter = exports.OperatorApplicationService = void 0;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== 文件上传辅助函数 ====================
/**
 * 运营商证照文件上传（multipart/form-data）
 * 符合后端 certificate_upload_guide.md 规范
 * @param url API路径
 * @param filePath 本地文件路径
 * @param formData 附加表单数据（如 side 参数）
 */
function uploadOperatorFile(url, filePath, formData = {}) {
    return new Promise((resolve, reject) => {
        const token = (0, auth_1.getToken)();
        if (!filePath) {
            reject(new Error('文件路径不能为空'));
            return;
        }
        wx.uploadFile({
            url: `${request_1.API_BASE}${url}`,
            filePath: filePath,
            name: 'image', // 关键：字段名必须是 image
            header: {
                'Authorization': `Bearer ${token}`
            },
            formData: formData,
            success: (res) => {
                if (res.statusCode === 200) {
                    try {
                        const data = JSON.parse(res.data);
                        if (data.code === 0 && data.data) {
                            resolve(data.data);
                        }
                        else if (data.data) {
                            resolve(data.data);
                        }
                        else {
                            resolve(data);
                        }
                    }
                    catch (e) {
                        reject(new Error('解析响应失败'));
                    }
                }
                else {
                    try {
                        const errData = JSON.parse(res.data);
                        reject(new Error(errData.message || `HTTP ${res.statusCode}`));
                    }
                    catch (_a) {
                        reject(new Error(`HTTP ${res.statusCode}`));
                    }
                }
            },
            fail: (err) => {
                reject(err);
            }
        });
    });
}
// ==================== 接口服务类 ====================
class OperatorApplicationService {
    /**
     * 获取或创建运营商申请
     * GET/POST /v1/operator/application
     */
    static async getOrCreateApplication(regionId) {
        if (regionId) {
            // 创建新申请
            const request_data = { region_id: regionId };
            return await (0, request_1.request)({
                url: '/v1/operator/application',
                method: 'POST',
                data: request_data
            });
        }
        else {
            // 获取现有申请
            return await (0, request_1.request)({
                url: '/v1/operator/application',
                method: 'GET'
            });
        }
    }
    /**
     * 更新运营商申请基本信息
     * PUT /v1/operator/application/basic
     */
    static async updateBasicInfo(data) {
        return await (0, request_1.request)({
            url: '/v1/operator/application/basic',
            method: 'PUT',
            data
        });
    }
    /**
     * 更新运营商申请区域
     * PUT /v1/operator/application/region
     */
    static async updateRegion(data) {
        return await (0, request_1.request)({
            url: '/v1/operator/application/region',
            method: 'PUT',
            data
        });
    }
    /**
     * 身份证OCR识别（正面或背面）
     * POST /v1/operator/application/idcard/ocr
     * 使用 multipart/form-data 上传图片文件
     * @param filePath 本地文件路径（wxfile:// 或 http://tmp/...）
     * @param side 正面 "Front" 或背面 "Back"
     */
    static async recognizeIDCard(filePath, side) {
        return uploadOperatorFile('/v1/operator/application/idcard/ocr', filePath, { side });
    }
    /**
     * 营业执照OCR识别（企业运营商）
     * POST /v1/operator/application/license/ocr
     * 使用 multipart/form-data 上传图片文件
     * @param filePath 本地文件路径
     */
    static async recognizeBusinessLicense(filePath) {
        return uploadOperatorFile('/v1/operator/application/license/ocr', filePath);
    }
    /**
     * 提交运营商申请
     * POST /v1/operator/application/submit
     */
    static async submitApplication() {
        return await (0, request_1.request)({
            url: '/v1/operator/application/submit',
            method: 'POST'
        });
    }
    /**
     * 重置运营商申请
     * POST /v1/operator/application/reset
     */
    static async resetApplication() {
        return await (0, request_1.request)({
            url: '/v1/operator/application/reset',
            method: 'POST'
        });
    }
    /**
     * 绑定银行账户
     * POST /v1/operator/applyment/bindbank
     */
    static async bindBank(data) {
        return await (0, request_1.request)({
            url: '/v1/operator/applyment/bindbank',
            method: 'POST',
            data
        });
    }
    /**
     * 获取申请状态
     * GET /v1/operator/applyment/status
     */
    static async getApplymentStatus() {
        return await (0, request_1.request)({
            url: '/v1/operator/applyment/status',
            method: 'GET'
        });
    }
}
exports.OperatorApplicationService = OperatorApplicationService;
// ==================== 数据适配器 ====================
/**
 * 运营商申请数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
class OperatorApplicationAdapter {
    /**
     * 将身份证OCR数据自动回填到表单
     */
    static fillIDCardData(ocrData) {
        return {
            legal_person_name: ocrData.name,
            legal_person_id_number: ocrData.id_number,
            contact_name: ocrData.name // 默认联系人为法人
        };
    }
    /**
     * 将营业执照OCR数据自动回填到表单
     */
    static fillBusinessLicenseData(ocrData) {
        return {
            name: ocrData.enterprise_name,
            legal_person_name: ocrData.legal_representative,
            business_license_number: ocrData.credit_code || ocrData.reg_num
        };
    }
    /**
     * 格式化申请状态显示文本
     */
    static formatStatus(status) {
        const statusMap = {
            'pending': '待提交',
            'reviewing': '审核中',
            'approved': '已通过',
            'rejected': '已拒绝'
        };
        return statusMap[status] || status;
    }
    /**
     * 格式化账户类型显示文本
     */
    static formatAccountType(accountType) {
        const typeMap = {
            'ACCOUNT_TYPE_BUSINESS': '对公账户',
            'ACCOUNT_TYPE_PRIVATE': '个人账户'
        };
        return typeMap[accountType] || accountType;
    }
    /**
     * 验证申请数据完整性
     */
    static validateApplicationData(data) {
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
        ];
        const missingFields = [];
        requiredFields.forEach(field => {
            if (!data[field.key]) {
                missingFields.push(field.label);
            }
        });
        return {
            isValid: missingFields.length === 0,
            missingFields
        };
    }
}
exports.OperatorApplicationAdapter = OperatorApplicationAdapter;
/**
 * 区域管理服务类
 * 为运营商申请提供区域选择功能
 */
class RegionService {
    /**
     * 获取区域列表
     * GET /v1/regions
     */
    static async getRegions(params) {
        return await (0, request_1.request)({
            url: '/v1/regions',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取可申请的区县列表（未被运营商绑定的区域）
     * GET /v1/regions/available
     */
    static async getAvailableRegions(params) {
        return await (0, request_1.request)({
            url: '/v1/regions/available',
            method: 'GET',
            data: params
        });
    }
    /**
     * 搜索区域
     * GET /v1/regions/search
     */
    static async searchRegions(params) {
        return await (0, request_1.request)({
            url: '/v1/regions/search',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取区域详情
     * GET /v1/regions/{id}
     */
    static async getRegionById(id) {
        return await (0, request_1.request)({
            url: `/v1/regions/${id}`,
            method: 'GET'
        });
    }
    /**
     * 检查区域是否可申请
     * GET /v1/regions/{id}/check
     */
    static async checkRegionAvailable(id) {
        return await (0, request_1.request)({
            url: `/v1/regions/${id}/check`,
            method: 'GET'
        });
    }
    /**
     * 获取区域的下级区域列表
     * GET /v1/regions/{id}/children
     */
    static async getRegionChildren(id) {
        return await (0, request_1.request)({
            url: `/v1/regions/${id}/children`,
            method: 'GET'
        });
    }
}
exports.RegionService = RegionService;
// ==================== 运营商申请流程辅助类 ====================
/**
 * 运营商申请流程管理器
 * 提供完整的申请流程支持
 */
class OperatorApplicationFlow {
    /**
     * 获取省份列表
     */
    static async getProvinces() {
        return await RegionService.getRegions({
            level: 1,
            page_id: 1,
            page_size: 50
        });
    }
    /**
     * 获取城市列表
     */
    static async getCities(provinceId) {
        return await RegionService.getRegions({
            level: 2,
            parent_id: provinceId,
            page_id: 1,
            page_size: 100
        });
    }
    /**
     * 获取可申请的区县列表
     */
    static async getAvailableDistricts(cityId) {
        const params = {
            level: 3,
            page_id: 1,
            page_size: 100
        };
        if (cityId) {
            params.parent_id = cityId;
        }
        return await RegionService.getAvailableRegions(params);
    }
    /**
     * 完整的运营商申请流程
     * 1. 选择区域并创建申请
     * 2. 上传并识别证件
     * 3. 填写基本信息
     * 4. 提交申请
     */
    static async createApplicationWithRegion(regionId) {
        // 1. 检查区域是否可申请
        const checkResult = await RegionService.checkRegionAvailable(regionId);
        if (!checkResult.available) {
            throw new Error(checkResult.message || '该区域不可申请');
        }
        // 2. 创建申请
        return await OperatorApplicationService.getOrCreateApplication(regionId);
    }
    /**
     * 自动填充OCR识别的数据
     * 注意：参数现在是本地文件路径，而非URL
     * @param idCardFilePath 身份证正面本地文件路径
     * @param businessLicenseFilePath 营业执照本地文件路径（可选）
     */
    static async autoFillFromOCR(idCardFilePath, businessLicenseFilePath) {
        const results = {};
        // 识别身份证正面
        if (idCardFilePath) {
            const idCardResult = await OperatorApplicationService.recognizeIDCard(idCardFilePath, 'Front');
            results.idCardData = idCardResult.id_card_front_ocr;
        }
        // 识别营业执照（如果是企业运营商）
        if (businessLicenseFilePath) {
            const licenseResult = await OperatorApplicationService.recognizeBusinessLicense(businessLicenseFilePath);
            results.businessLicenseData = licenseResult.business_license_ocr;
        }
        // 生成建议的表单数据
        const suggestedFormData = {};
        if (results.idCardData) {
            const idCardFill = OperatorApplicationAdapter.fillIDCardData(results.idCardData);
            Object.assign(suggestedFormData, idCardFill);
        }
        if (results.businessLicenseData) {
            const licenseFill = OperatorApplicationAdapter.fillBusinessLicenseData(results.businessLicenseData);
            Object.assign(suggestedFormData, licenseFill);
        }
        return {
            ...results,
            suggestedFormData
        };
    }
}
exports.OperatorApplicationFlow = OperatorApplicationFlow;
// ==================== 导出默认服务 ====================
exports.default = OperatorApplicationService;
