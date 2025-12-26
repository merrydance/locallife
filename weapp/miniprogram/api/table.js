"use strict";
/**
 * 扫码点餐相关API接口 (顾客端功能)
 * 基于swagger.json中的扫码点餐接口
 * 商户端桌台管理功能已迁移到 table-device-management.ts
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
exports.getTable = exports.getScanTableInfo = void 0;
exports.scanTable = scanTable;
exports.getTableDetail = getTableDetail;
exports.parseQRCodeUrl = parseQRCodeUrl;
exports.generateQRCodeUrl = generateQRCodeUrl;
const request_1 = require("../utils/request");
// ==================== API接口函数 ====================
/**
 * 扫码点餐 - 顾客扫码获取商户和桌台信息
 * 顾客扫描桌台二维码后，获取完整的菜单信息进行点餐
 * @param merchantId 商户ID
 * @param tableNo 桌台编号
 */
function scanTable(merchantId, tableNo) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/scan/table',
            method: 'GET',
            data: { merchant_id: merchantId, table_no: tableNo }
        });
    });
}
/**
 * 获取桌台详情
 * @param tableId 桌台ID
 */
function getTableDetail(tableId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}`,
            method: 'GET'
        });
    });
}
// ==================== 注意 ====================
// 商户端桌台管理功能已迁移到 table-device-management.ts
// 包括：桌台CRUD、状态管理、二维码管理、图片管理、标签管理等
// ==================== 便捷方法 ====================
/**
 * 通过二维码URL解析商户和桌台信息
 * @param qrCodeUrl 二维码URL
 */
function parseQRCodeUrl(qrCodeUrl) {
    try {
        const url = new URL(qrCodeUrl);
        const merchantId = url.searchParams.get('merchant_id');
        const tableNo = url.searchParams.get('table_no');
        if (merchantId && tableNo) {
            return {
                merchantId: parseInt(merchantId),
                tableNo
            };
        }
    }
    catch (error) {
        console.error('Invalid QR code URL:', error);
    }
    return null;
}
/**
 * 生成桌台二维码URL
 * @param merchantId 商户ID
 * @param tableNo 桌台编号
 * @param baseUrl 基础URL
 */
function generateQRCodeUrl(merchantId, tableNo, baseUrl = 'https://api.example.com') {
    return `${baseUrl}/v1/scan/table?merchant_id=${merchantId}&table_no=${encodeURIComponent(tableNo)}`;
}
// ==================== 便捷函数已迁移 ====================
// getAvailableTables 和 getPrivateRooms 等商户端功能
// 已迁移到 table-device-management.ts
// ==================== 兼容性别名 ====================
/** @deprecated 使用 scanTable 替代 */
exports.getScanTableInfo = scanTable;
/** @deprecated 使用 getTableDetail 替代 */
exports.getTable = getTableDetail;
// ==================== 商户端功能迁移说明 ====================
/**
 * 商户端桌台和设备管理功能已迁移到新文件：
 * import {
 *   tableManagementService,
 *   deviceManagementService,
 *   displayConfigService
 * } from './table-device-management'
 */ 
