"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getMerchantApplication = getMerchantApplication;
exports.updateMerchantBasicInfo = updateMerchantBasicInfo;
exports.ocrBusinessLicense = ocrBusinessLicense;
exports.ocrFoodPermit = ocrFoodPermit;
exports.ocrIdCard = ocrIdCard;
exports.submitMerchantApplication = submitMerchantApplication;
exports.getMyApplication = getMyApplication;
exports.resetMerchantApplication = resetMerchantApplication;
exports.uploadMerchantImage = uploadMerchantImage;
exports.updateMerchantImages = updateMerchantImages;
exports.submitRiderApplication = submitRiderApplication;
const request_1 = require("../utils/request");
// ==================== API Methods ====================
/**
 * 获取或创建商户入驻申请草稿
 * GET /v1/merchant/application
 * - 200: 返回现有草稿
 * - 201: 创建新草稿并返回
 * - 409: 已存在 submitted/approved
 */
function getMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchant/application',
        method: 'GET'
    });
}
/**
 * 更新基础信息（草稿可编辑）
 * PUT /v1/merchant/application/basic
 */
function updateMerchantBasicInfo(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/basic',
        method: 'PUT',
        data
    });
}
/**
 * 营业执照 OCR（异步）
 * POST /v1/merchant/application/license/ocr
 * @param filePath 本地文件路径，若不传则复用已上传的图片
 */
function ocrBusinessLicense(filePath) {
    if (filePath) {
        return (0, request_1.uploadFile)(filePath, '/v1/merchant/application/license/ocr', 'image');
    }
    // 不传文件时，触发复用已有图片的 OCR
    return (0, request_1.request)({
        url: '/v1/merchant/application/license/ocr',
        method: 'POST'
    });
}
/**
 * 食品经营许可证 OCR（异步）
 * POST /v1/merchant/application/foodpermit/ocr
 */
function ocrFoodPermit(filePath) {
    if (filePath) {
        return (0, request_1.uploadFile)(filePath, '/v1/merchant/application/foodpermit/ocr', 'image');
    }
    return (0, request_1.request)({
        url: '/v1/merchant/application/foodpermit/ocr',
        method: 'POST'
    });
}
/**
 * 身份证 OCR（异步）
 * POST /v1/merchant/application/idcard/ocr
 * @param side 'Front' 或 'Back'
 */
function ocrIdCard(filePath, side) {
    if (filePath) {
        return (0, request_1.uploadFile)(filePath, '/v1/merchant/application/idcard/ocr', 'image', { side });
    }
    return (0, request_1.request)({
        url: '/v1/merchant/application/idcard/ocr',
        method: 'POST',
        data: { side }
    });
}
/**
 * 提交申请（自动审核）
 * POST /v1/merchant/application/submit
 * 无请求体，返回 approved 或 rejected
 */
function submitMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchant/application/submit',
        method: 'POST'
    });
}
/**
 * 获取当前用户最新申请（用于 submitted 后轮询）
 * GET /v1/merchants/applications/me
 */
function getMyApplication() {
    return (0, request_1.request)({
        url: '/v1/merchants/applications/me',
        method: 'GET'
    });
}
/**
 * 重置被拒绝申请为草稿
 * POST /v1/merchant/application/reset
 */
function resetMerchantApplication() {
    return (0, request_1.request)({
        url: '/v1/merchant/application/reset',
        method: 'POST'
    });
}
/**
 * 上传门头照/环境照图片文件
 * POST /v1/merchants/images/upload
 * @param filePath 本地文件路径
 * @param category 'storefront' 或 'environment'
 */
function uploadMerchantImage(filePath, category) {
    return (0, request_1.uploadFile)(filePath, '/v1/merchants/images/upload', 'image', { category });
}
/**
 * 保存门头照/环境照 URL 到草稿
 * PUT /v1/merchant/application/images
 */
function updateMerchantImages(data) {
    return (0, request_1.request)({
        url: '/v1/merchant/application/images',
        method: 'PUT',
        data
    });
}
function submitRiderApplication(data) {
    return (0, request_1.request)({
        url: '/onboarding/rider',
        method: 'POST',
        data
    });
}
