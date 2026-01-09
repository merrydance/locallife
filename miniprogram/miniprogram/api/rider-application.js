"use strict";
/**
 * 骑手入驻申请接口
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
exports.RiderApplicationFlow = void 0;
exports.getRiderApplicationDraft = getRiderApplicationDraft;
exports.updateRiderApplicationBasic = updateRiderApplicationBasic;
exports.submitRiderApplication = submitRiderApplication;
exports.resetRiderApplication = resetRiderApplication;
exports.recognizeRiderIDCard = recognizeRiderIDCard;
exports.uploadHealthCert = uploadHealthCert;
exports.bindRiderBank = bindRiderBank;
exports.getRiderApplymentStatus = getRiderApplymentStatus;
exports.createRiderApplicationFlow = createRiderApplicationFlow;
exports.checkRiderApplicationStatus = checkRiderApplicationStatus;
exports.getApplicationStatusDescription = getApplicationStatusDescription;
exports.getBankBindingStatusDescription = getBankBindingStatusDescription;
exports.validatePhoneNumber = validatePhoneNumber;
exports.validateIDCardNumber = validateIDCardNumber;
exports.validateRealName = validateRealName;
exports.validateRiderApplicationForm = validateRiderApplicationForm;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== 骑手申请管理接口 ====================
/**
 * 获取或创建骑手申请草稿
 * 如果不存在则自动创建新的草稿
 */
function getRiderApplicationDraft() {
    return (0, request_1.request)({
        url: '/v1/rider/application',
        method: 'GET'
    });
}
/**
 * 更新骑手申请基本信息
 */
function updateRiderApplicationBasic(data) {
    return (0, request_1.request)({
        url: '/v1/rider/application/basic',
        method: 'PUT',
        data
    });
}
/**
 * 提交骑手申请
 */
function submitRiderApplication() {
    return (0, request_1.request)({
        url: '/v1/rider/application/submit',
        method: 'POST'
    });
}
/**
 * 重置骑手申请
 */
function resetRiderApplication() {
    return (0, request_1.request)({
        url: '/v1/rider/application/reset',
        method: 'POST'
    });
}
// ==================== OCR识别接口 ====================
// Helper for multipart upload
function uploadRiderFile(url, filePath, formData = {}) {
    return new Promise((resolve, reject) => {
        const token = (0, auth_1.getToken)(); // Assume getToken exists or imported from auth/request
        if (!filePath) {
            reject(new Error('File path is empty'));
            return;
        }
        wx.uploadFile({
            url: `${request_1.API_BASE}${url}`,
            filePath: filePath,
            name: 'image',
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
                            // Support direct return if any
                            resolve(data);
                        }
                    }
                    catch (e) {
                        reject(new Error('Parse response failed'));
                    }
                }
                else {
                    reject(new Error(`HTTP ${res.statusCode}`));
                }
            },
            fail: (err) => {
                reject(err);
            }
        });
    });
}
/**
 * 身份证OCR识别
 * 自动识别姓名、身份证号、地址、性别、有效期等信息并回填到表单
 */
function recognizeRiderIDCard(data) {
    // data.image_url is actually the filePath for upload here
    // But existing interface calls it image_url.
    // If the caller passes a local temp path (wxfile://), we upload it.
    // We should probably change the interface key to filePath to be clear, 
    // but to avoid breaking changes if it was used elsewhere, we treat image_url as path.
    const filePath = data.image_url;
    const formData = { side: data.side || 'front' };
    return uploadRiderFile('/v1/rider/application/idcard/ocr', filePath, formData);
}
/**
 * 上传健康证
 * 上传健康证图片，后端会进行OCR识别
 */
function uploadHealthCert(data) {
    const filePath = data.image_url;
    return uploadRiderFile('/v1/rider/application/healthcert', filePath);
}
// ==================== 银行绑定接口 ====================
/**
 * 绑定银行账户
 */
function bindRiderBank(data) {
    return (0, request_1.request)({
        url: '/v1/rider/applyment/bindbank',
        method: 'POST',
        data
    });
}
/**
 * 获取申请状态
 */
function getRiderApplymentStatus() {
    return (0, request_1.request)({
        url: '/v1/rider/applyment/status',
        method: 'GET'
    });
}
// ==================== 便捷方法 ====================
/**
 * 完整的骑手入驻流程
 * 1. 获取或创建草稿
 * 2. 上传并识别证件
 * 3. 填写基本信息
 * 4. 提交申请
 */
class RiderApplicationFlow {
    constructor() {
        this.draft = null;
    }
    /**
     * 初始化申请流程
     */
    initialize() {
        return __awaiter(this, void 0, void 0, function* () {
            this.draft = yield getRiderApplicationDraft();
            return this.draft;
        });
    }
    /**
     * 上传并识别身份证
     */
    uploadAndRecognizeIDCard(imageUrl) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield recognizeRiderIDCard({ image_url: imageUrl });
            this.draft = result;
            // 自动回填识别到的信息
            if (result.id_card_ocr) {
                const ocrData = result.id_card_ocr;
                const updateData = {};
                if (ocrData.name)
                    updateData.real_name = ocrData.name;
                // 如果有识别到的信息，自动更新
                if (Object.keys(updateData).length > 0) {
                    this.draft = yield updateRiderApplicationBasic(updateData);
                }
            }
            return this.draft;
        });
    }
    /**
     * 上传并识别健康证
     */
    uploadAndRecognizeHealthCert(imageUrl) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield uploadHealthCert({ image_url: imageUrl });
            this.draft = result;
            return this.draft;
        });
    }
    /**
     * 更新基本信息
     */
    updateBasicInfo(data) {
        return __awaiter(this, void 0, void 0, function* () {
            this.draft = yield updateRiderApplicationBasic(data);
            return this.draft;
        });
    }
    /**
     * 提交申请
     */
    submit() {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield submitRiderApplication();
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
            { field: 'real_name', name: '真实姓名' },
            { field: 'phone', name: '手机号' },
            { field: 'id_card_front_url', name: '身份证正面图片' },
            { field: 'id_card_back_url', name: '身份证背面图片' },
            { field: 'health_cert_url', name: '健康证图片' }
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
    /**
     * 获取OCR识别的信息摘要
     */
    getOCRSummary() {
        if (!this.draft)
            return {};
        const summary = {};
        // 身份证信息摘要
        if (this.draft.id_card_ocr) {
            const ocr = this.draft.id_card_ocr;
            summary.idCard = {
                name: ocr.name,
                idNumber: ocr.id_number,
                address: ocr.address,
                gender: ocr.gender,
                validPeriod: ocr.valid_start && ocr.valid_end
                    ? `${ocr.valid_start} 至 ${ocr.valid_end}`
                    : undefined
            };
        }
        // 健康证信息摘要
        if (this.draft.health_cert_ocr) {
            const ocr = this.draft.health_cert_ocr;
            summary.healthCert = {
                certNumber: ocr.cert_number,
                validPeriod: ocr.valid_start && ocr.valid_end
                    ? `${ocr.valid_start} 至 ${ocr.valid_end}`
                    : undefined
            };
        }
        return summary;
    }
}
exports.RiderApplicationFlow = RiderApplicationFlow;
/**
 * 创建骑手申请流程实例
 */
function createRiderApplicationFlow() {
    return new RiderApplicationFlow();
}
/**
 * 快速检查申请状态
 */
function checkRiderApplicationStatus() {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const application = yield getRiderApplicationDraft();
            return {
                hasApplication: !!application.id,
                status: application.status,
                canApply: !application.status || application.status === 'rejected' // 没有申请或被拒绝后才能申请
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
/**
 * 获取银行绑定状态描述
 */
function getBankBindingStatusDescription(status) {
    const statusMap = {
        'pending': '待审核',
        'approved': '已通过',
        'rejected': '已拒绝',
        'processing': '处理中'
    };
    return statusMap[status] || '未知状态';
}
/**
 * 验证手机号格式
 */
function validatePhoneNumber(phone) {
    const phoneRegex = /^1[3-9]\d{9}$/;
    return phoneRegex.test(phone);
}
/**
 * 验证身份证号格式
 */
function validateIDCardNumber(idCard) {
    const idCardRegex = /(^\d{15}$)|(^\d{18}$)|(^\d{17}(\d|X|x)$)/;
    return idCardRegex.test(idCard);
}
/**
 * 验证真实姓名格式
 */
function validateRealName(name) {
    if (!name || name.length < 2 || name.length > 50) {
        return false;
    }
    // 中文姓名正则
    const chineseNameRegex = /^[\u4e00-\u9fa5·]{2,50}$/;
    return chineseNameRegex.test(name);
}
/**
 * 完整的表单验证
 */
function validateRiderApplicationForm(data) {
    const errors = [];
    if (!data.real_name) {
        errors.push('请填写真实姓名');
    }
    else if (!validateRealName(data.real_name)) {
        errors.push('真实姓名格式不正确');
    }
    if (!data.phone) {
        errors.push('请填写手机号');
    }
    else if (!validatePhoneNumber(data.phone)) {
        errors.push('手机号格式不正确');
    }
    if (!data.id_card_front_url) {
        errors.push('请上传身份证正面照片');
    }
    if (!data.id_card_back_url) {
        errors.push('请上传身份证背面照片');
    }
    if (!data.health_cert_url) {
        errors.push('请上传健康证照片');
    }
    return {
        isValid: errors.length === 0,
        errors
    };
}
// 兼容性导出
exports.default = {
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
};
