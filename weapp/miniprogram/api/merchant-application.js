"use strict";
/**
 * 商户入驻申请接口
 * 基于swagger.json完全重构，包含OCR识别和数据回填功能
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.MerchantApplicationFlow = void 0;
exports.getMerchantApplicationDraft = getMerchantApplicationDraft;
exports.updateMerchantApplicationBasic = updateMerchantApplicationBasic;
exports.submitMerchantApplication = submitMerchantApplication;
exports.resetMerchantApplication = resetMerchantApplication;
exports.getMyMerchantApplication = getMyMerchantApplication;
exports.createMerchantApplication = createMerchantApplication;
exports.recognizeIDCardFront = recognizeIDCardFront;
exports.recognizeBusinessLicense = recognizeBusinessLicense;
exports.recognizeFoodPermit = recognizeFoodPermit;
exports.bindMerchantBank = bindMerchantBank;
exports.getMerchantApplymentStatus = getMerchantApplymentStatus;
exports.createMerchantApplicationFlow = createMerchantApplicationFlow;
exports.checkMerchantApplicationStatus = checkMerchantApplicationStatus;
exports.getApplicationStatusDescription = getApplicationStatusDescription;
const request_1 = require("../utils/request");
// ==================== 商户申请管理接口 ====================
/**
 * 获取或创建商户申请草稿
 * 如果不存在则自动创建新的草稿
 */
function getMerchantApplicationDraft() {
    return (0, request_1.request)({
        url: '/v1/merchant/application',
        method: 'GET'
    });
}
/**
 * 更新商户申请基本信息
 */
function updateMerchantApplicationBasic(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/basic',
        method: 'PUT',
        data
    });
}
/**
 * 提交商户申请
 */
function submitMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchant/application/submit',
        method: 'POST'
    });
}
/**
 * 重置商户申请
 */
function resetMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchant/application/reset',
        method: 'POST'
    });
}
/**
 * 获取我的商户申请状态
 */
function getMyMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchants/applications/me',
        method: 'GET'
    });
}
/**
 * 创建商户申请（正式提交）
 */
function createMerchantApplication(data) {
    return (0, request_1.request)({
        url: '/v1/merchants/applications',
        method: 'POST',
        data
    });
}
// ==================== OCR识别接口 ====================
/**
 * 身份证正面OCR识别
 * 自动识别姓名、身份证号、地址、性别等信息并回填到表单
 */
function recognizeIDCardFront(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/idcard/ocr',
        method: 'POST',
        data
    });
}
/**
 * 营业执照OCR识别
 * 自动识别企业名称、统一社会信用代码、法定代表人、经营范围、地址等信息并回填到表单
 */
function recognizeBusinessLicense(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/license/ocr',
        method: 'POST',
        data
    });
}
/**
 * 食品经营许可证OCR识别
 * 自动识别许可证编号、企业名称、有效期等信息并回填到表单
 */
function recognizeFoodPermit(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/foodpermit/ocr',
        method: 'POST',
        data
    });
}
// ==================== 银行绑定接口 ====================
/**
 * 绑定银行账户
 */
function bindMerchantBank(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/bindbank',
        method: 'POST',
        data
    });
}
/**
 * 获取申请状态
 */
function getMerchantApplymentStatus() {
    return (0, request_1.request)({
        url: '/v1/merchant/applyment/status',
        method: 'GET'
    });
}
// ==================== 便捷方法 ====================
/**
 * 完整的商户入驻流程
 * 1. 获取或创建草稿
 * 2. 上传并识别证件
 * 3. 填写基本信息
 * 4. 提交申请
 */
class MerchantApplicationFlow {
    constructor() {
        this.draft = null;
    }
    /**
     * 初始化申请流程
     */
    initialize() {
        return __awaiter(this, void 0, void 0, function* () {
            this.draft = yield getMerchantApplicationDraft();
            return this.draft;
        });
    }
    /**
     * 上传并识别身份证正面
     */
    uploadAndRecognizeIDCard(imageUrl) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield recognizeIDCardFront({ image_url: imageUrl });
            this.draft = result;
            // 自动回填识别到的信息
            if (result.id_card_front_ocr) {
                const ocrData = result.id_card_front_ocr;
                const updateData = {};
                if (ocrData.name)
                    updateData.legal_person_name = ocrData.name;
                if (ocrData.id_number)
                    updateData.legal_person_id_number = ocrData.id_number;
                if (ocrData.address)
                    updateData.business_address = ocrData.address;
                // 如果有识别到的信息，自动更新
                if (Object.keys(updateData).length > 0) {
                    this.draft = yield updateMerchantApplicationBasic(updateData);
                }
            }
            return this.draft;
        });
    }
    /**
     * 上传并识别营业执照
     */
    uploadAndRecognizeBusinessLicense(imageUrl) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield recognizeBusinessLicense({ image_url: imageUrl });
            this.draft = result;
            // 自动回填识别到的信息
            if (result.business_license_ocr) {
                const ocrData = result.business_license_ocr;
                const updateData = {};
                if (ocrData.enterprise_name) {
                    // 营业执照上的企业名称作为默认商户名称，用户可以自定义修改
                    updateData.merchant_name = ocrData.enterprise_name;
                }
                if (ocrData.credit_code)
                    updateData.business_license_number = ocrData.credit_code;
                if (ocrData.business_scope)
                    updateData.business_scope = ocrData.business_scope;
                if (ocrData.legal_representative)
                    updateData.legal_person_name = ocrData.legal_representative;
                if (ocrData.address)
                    updateData.business_address = ocrData.address;
                // 如果有识别到的信息，自动更新
                if (Object.keys(updateData).length > 0) {
                    this.draft = yield updateMerchantApplicationBasic(updateData);
                }
            }
            return this.draft;
        });
    }
    /**
     * 上传并识别食品经营许可证
     */
    uploadAndRecognizeFoodPermit(imageUrl) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield recognizeFoodPermit({ image_url: imageUrl });
            this.draft = result;
            return this.draft;
        });
    }
    /**
     * 更新基本信息
     */
    updateBasicInfo(data) {
        return __awaiter(this, void 0, void 0, function* () {
            this.draft = yield updateMerchantApplicationBasic(data);
            return this.draft;
        });
    }
    /**
     * 提交申请
     */
    submit() {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield submitMerchantApplication();
            this.draft = result;
            return result;
        });
    }
    /**
     * 获取当前草稿
     */
    getCurrentDraft() {
        return this.draft;
    }
    /**
     * 验证申请信息是否完整
     */
    validateApplication() {
        if (!this.draft) {
            return { isValid: false, missingFields: ['申请草稿未初始化'] };
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
        ];
        const missingFields = [];
        for (const { field, name } of requiredFields) {
            if (!this.draft[field]) {
                missingFields.push(name);
            }
        }
        return {
            isValid: missingFields.length === 0,
            missingFields
        };
    }
}
exports.MerchantApplicationFlow = MerchantApplicationFlow;
/**
 * 创建商户申请流程实例
 */
function createMerchantApplicationFlow() {
    return new MerchantApplicationFlow();
}
/**
 * 快速检查申请状态
 */
function checkMerchantApplicationStatus() {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const application = yield getMyMerchantApplication();
            return {
                hasApplication: true,
                status: application.status,
                canApply: application.status === 'rejected' // 只有被拒绝后才能重新申请
            };
        }
        catch (error) {
            // 如果没有申请记录，返回可以申请
            return {
                hasApplication: false,
                canApply: true
            };
        }
    });
}
/**
 * 获取申请进度描述
 */
function getApplicationStatusDescription(status) {
    const statusMap = {
        'draft': '草稿中',
        'pending': '审核中',
        'approved': '审核通过',
        'rejected': '审核拒绝',
        'processing': '处理中'
    };
    return statusMap[status] || '未知状态';
}
// 兼容性导出
exports.default = {
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
};
