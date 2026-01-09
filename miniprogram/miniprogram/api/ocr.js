"use strict";
/**
 * 证照上传与OCR识别接口
 * 符合 docs/certificate_upload_guide.md 规范
 * 支持商户、骑手、运营商三种角色
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.ocrBusinessLicense = ocrBusinessLicense;
exports.ocrIdCard = ocrIdCard;
exports.ocrFoodLicense = ocrFoodLicense;
exports.ocrRiderIdCard = ocrRiderIdCard;
exports.ocrRiderHealthCert = ocrRiderHealthCert;
exports.ocrOperatorBusinessLicense = ocrOperatorBusinessLicense;
exports.ocrOperatorIdCard = ocrOperatorIdCard;
const request_1 = require("../utils/request");
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
};
// ==================== 证照上传功能 ====================
/**
 * 通用文件上传（multipart/form-data）
 * @param url API路径
 * @param filePath 本地文件路径
 * @param formData 附加表单数据（如 side 参数）
 */
function uploadOCR(url, filePath, formData = {}) {
    return (0, request_1.uploadFile)(filePath, url, 'image', formData);
}
// ==================== 商户 OCR 接口 ====================
/**
 * 商户营业执照OCR
 * POST /v1/merchant/application/license/ocr
 */
function ocrBusinessLicense(filePath) {
    return uploadOCR(API_ENDPOINTS.merchant.license, filePath)
        .then((res) => {
        console.log('[OCR] Merchant Business License Response:', JSON.stringify(res));
        return {
            applicationData: res,
            ocrData: res.business_license_ocr || res
        };
    });
}
/**
 * 商户身份证OCR
 * POST /v1/merchant/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
function ocrIdCard(filePath, side = 'front') {
    // 商户接口使用 "Front"/"Back" 首字母大写
    const capitalizedSide = side === 'front' ? 'Front' : 'Back';
    return uploadOCR(API_ENDPOINTS.merchant.idcard, filePath, { side: capitalizedSide })
        .then((res) => {
        console.log(`[OCR] Merchant ID Card ${side} Response:`, JSON.stringify(res));
        const ocrData = side === 'front'
            ? (res.id_card_front_ocr || res)
            : (res.id_card_back_ocr || res);
        return {
            applicationData: res,
            ocrData
        };
    });
}
/**
 * 商户食品经营许可证OCR
 * POST /v1/merchant/application/foodpermit/ocr
 */
function ocrFoodLicense(filePath) {
    return uploadOCR(API_ENDPOINTS.merchant.foodpermit, filePath)
        .then((res) => {
        console.log('[OCR] Merchant Food License Response:', JSON.stringify(res));
        return {
            applicationData: res,
            ocrData: res.food_permit_ocr || res
        };
    });
}
// ==================== 骑手 OCR 接口 ====================
/**
 * 骑手身份证OCR
 * POST /v1/rider/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面) 或 'back' (背面)
 */
function ocrRiderIdCard(filePath, side = 'front') {
    // 骑手接口使用小写 "front"/"back"
    return uploadOCR(API_ENDPOINTS.rider.idcard, filePath, { side })
        .then((res) => {
        console.log(`[OCR] Rider ID Card ${side} Response:`, JSON.stringify(res));
        const ocrData = res.id_card_ocr || res;
        return {
            applicationData: res,
            ocrData
        };
    });
}
/**
 * 骑手健康证上传
 * POST /v1/rider/application/healthcert
 */
function ocrRiderHealthCert(filePath) {
    return uploadOCR(API_ENDPOINTS.rider.healthcert, filePath)
        .then((res) => {
        console.log('[OCR] Rider Health Cert Response:', JSON.stringify(res));
        return {
            applicationData: res,
            ocrData: res.health_cert_ocr || res
        };
    });
}
// ==================== 运营商 OCR 接口 ====================
/**
 * 运营商营业执照OCR
 * POST /v1/operator/application/license/ocr
 */
function ocrOperatorBusinessLicense(filePath) {
    return uploadOCR(API_ENDPOINTS.operator.license, filePath)
        .then((res) => {
        console.log('[OCR] Operator Business License Response:', JSON.stringify(res));
        return {
            applicationData: res,
            ocrData: res.business_license_ocr || res
        };
    });
}
/**
 * 运营商身份证OCR
 * POST /v1/operator/application/idcard/ocr
 * @param filePath 本地文件路径
 * @param side 'front' (正面/Front) 或 'back' (背面/Back)
 */
function ocrOperatorIdCard(filePath, side = 'front') {
    // 运营商接口使用 "Front"/"Back" 首字母大写
    const capitalizedSide = side === 'front' ? 'Front' : 'Back';
    return uploadOCR(API_ENDPOINTS.operator.idcard, filePath, { side: capitalizedSide })
        .then((res) => {
        console.log(`[OCR] Operator ID Card ${side} Response:`, JSON.stringify(res));
        const ocrData = side === 'front'
            ? (res.id_card_front_ocr || res)
            : (res.id_card_back_ocr || res);
        return {
            applicationData: res,
            ocrData
        };
    });
}
