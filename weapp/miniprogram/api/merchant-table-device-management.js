"use strict";
/**
 * 商户桌台和设备管理API接口
 * 基于swagger.json重构，提供桌台管理和二维码生成功能
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
exports.DeviceAdapter = exports.TableAdapter = void 0;
exports.getTables = getTables;
exports.getTable = getTable;
exports.createTable = createTable;
exports.updateTable = updateTable;
exports.deleteTable = deleteTable;
exports.updateTableStatus = updateTableStatus;
exports.generateTableQRCode = generateTableQRCode;
exports.getTableQRCode = getTableQRCode;
exports.uploadTableImages = uploadTableImages;
exports.uploadTableImage = uploadTableImage;
exports.getDevices = getDevices;
exports.getDevice = getDevice;
exports.createDevice = createDevice;
exports.updateDevice = updateDevice;
exports.deleteDevice = deleteDevice;
exports.testDevice = testDevice;
exports.getDisplayConfig = getDisplayConfig;
exports.updateDisplayConfig = updateDisplayConfig;
exports.batchUpdateTableStatus = batchUpdateTableStatus;
exports.batchGenerateQRCodes = batchGenerateQRCodes;
exports.getTableTypeStats = getTableTypeStats;
exports.checkTableNumberAvailable = checkTableNumberAvailable;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== 桌台管理接口 ====================
/**
 * 获取桌台列表
 */
function getTables() {
    return __awaiter(this, arguments, void 0, function* (params = {}) {
        // 手动构建查询字符串（小程序不支持 URLSearchParams）
        const queryParts = [];
        if (params.page)
            queryParts.push(`page=${params.page}`);
        if (params.page_size)
            queryParts.push(`page_size=${params.page_size}`);
        if (params.table_type)
            queryParts.push(`table_type=${params.table_type}`);
        if (params.status)
            queryParts.push(`status=${params.status}`);
        const queryString = queryParts.length > 0 ? '?' + queryParts.join('&') : '';
        const url = `/v1/tables${queryString}`;
        return (0, request_1.request)({
            url,
            method: 'GET'
        });
    });
}
/**
 * 获取单个桌台信息
 */
function getTable(tableId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}`,
            method: 'GET'
        });
    });
}
/**
 * 创建桌台
 */
function createTable(data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/tables',
            method: 'POST',
            data
        });
    });
}
/**
 * 更新桌台信息
 */
function updateTable(tableId, data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}`,
            method: 'PATCH',
            data
        });
    });
}
/**
 * 删除桌台
 */
function deleteTable(tableId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}`,
            method: 'DELETE'
        });
    });
}
/**
 * 更新桌台状态
 */
function updateTableStatus(tableId, status) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}/status`,
            method: 'PATCH',
            data: { status }
        });
    });
}
// ==================== 二维码管理接口 ====================
/**
 * 生成桌台二维码
 */
function generateTableQRCode(tableId_1) {
    return __awaiter(this, arguments, void 0, function* (tableId, regenerate = false) {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}/qrcode`,
            method: 'GET'
        });
    });
}
/**
 * 获取桌台二维码
 */
function getTableQRCode(tableId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}/qrcode`,
            method: 'GET'
        });
    });
}
/**
 * 上传桌台图片
 */
function uploadTableImages(tableId, images) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/tables/${tableId}/images`,
            method: 'POST',
            data: { images }
        });
    });
}
/**
 * 上传桌台图片文件
 */
function uploadTableImage(filePath) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve, reject) => {
            const token = (0, auth_1.getToken)();
            wx.uploadFile({
                url: `${request_1.API_BASE}/v1/tables/images/upload`,
                filePath,
                name: 'image',
                header: {
                    'Authorization': `Bearer ${token}`
                },
                success: (res) => {
                    var _a;
                    if (res.statusCode === 200) {
                        try {
                            const data = JSON.parse(res.data);
                            // api.uploadImageResponse
                            if (data.code === 0 && data.data && data.data.image_url) {
                                resolve(data.data.image_url);
                            }
                            else if (data.image_url) {
                                resolve(data.image_url);
                            }
                            else {
                                // Try fallback if data IS the object
                                resolve(((_a = data.data) === null || _a === void 0 ? void 0 : _a.image_url) || data.image_url);
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
                fail: reject
            });
        });
    });
}
// ==================== 设备管理接口 ====================
/**
 * 获取设备列表
 */
function getDevices() {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/merchant/devices',
            method: 'GET'
        });
    });
}
/**
 * 获取单个设备信息
 */
function getDevice(deviceId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchant/devices/${deviceId}`,
            method: 'GET'
        });
    });
}
/**
 * 添加设备
 */
function createDevice(data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/merchant/devices',
            method: 'POST',
            data
        });
    });
}
/**
 * 更新设备信息
 */
function updateDevice(deviceId, data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchant/devices/${deviceId}`,
            method: 'PATCH',
            data
        });
    });
}
/**
 * 删除设备
 */
function deleteDevice(deviceId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchant/devices/${deviceId}`,
            method: 'DELETE'
        });
    });
}
/**
 * 测试设备连接
 */
function testDevice(deviceId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchant/devices/${deviceId}/test`,
            method: 'POST'
        });
    });
}
// ==================== 显示配置接口 ====================
/**
 * 获取显示配置
 */
function getDisplayConfig() {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/merchant/display-config',
            method: 'GET'
        });
    });
}
/**
 * 更新显示配置
 */
function updateDisplayConfig(data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/merchant/display-config',
            method: 'PATCH',
            data
        });
    });
}
// ==================== 数据适配器 ====================
/**
 * 桌台数据适配器
 */
class TableAdapter {
    /**
     * 格式化桌台状态
     */
    static formatStatus(status) {
        const statusMap = {
            'available': '空闲',
            'occupied': '就餐中',
            'reserved': '已预定',
            'cleaning': '清洁中',
            'maintenance': '维护中'
        };
        return statusMap[status] || status;
    }
    /**
     * 获取状态颜色
     */
    static getStatusColor(status) {
        const colorMap = {
            'available': '#52c41a',
            'occupied': '#1890ff',
            'reserved': '#faad14',
            'cleaning': '#722ed1',
            'maintenance': '#ff4d4f'
        };
        return colorMap[status] || '#999';
    }
    /**
     * 格式化桌台信息
     */
    static formatTableInfo(table) {
        const parts = [];
        if (table.capacity)
            parts.push(`${table.capacity}人桌`);
        // if (table.area) parts.push(table.area); // area removed from interface
        if (table.description)
            parts.push(table.description);
        return parts.join(' · ');
    }
    /**
     * 验证桌台编号格式
     */
    static validateTableNumber(tableNumber) {
        // 桌台编号应该是字母+数字的组合，如：A01, B12
        return /^[A-Z]\d{2,3}$/.test(tableNumber);
    }
    /**
     * 生成桌台编号建议
     */
    static generateTableNumberSuggestion(area, existingNumbers) {
        const areaCode = area.charAt(0).toUpperCase() || 'A';
        let number = 1;
        while (true) {
            const numStr = number.toString();
            const paddedNum = numStr.length < 2 ? '0' + numStr : numStr;
            const suggestion = `${areaCode}${paddedNum}`;
            if (!existingNumbers.includes(suggestion)) {
                return suggestion;
            }
            number++;
        }
    }
}
exports.TableAdapter = TableAdapter;
/**
 * 设备数据适配器
 */
class DeviceAdapter {
    /**
     * 格式化设备类型
     */
    static formatDeviceType(type) {
        const typeMap = {
            'printer': '打印机',
            'display': '显示屏',
            'scanner': '扫码器'
        };
        return typeMap[type] || type;
    }
    /**
     * 格式化连接类型
     */
    static formatConnectionType(type) {
        const typeMap = {
            'usb': 'USB连接',
            'network': '网络连接',
            'bluetooth': '蓝牙连接'
        };
        return typeMap[type] || type;
    }
    /**
     * 格式化设备状态
     */
    static formatDeviceStatus(status) {
        const statusMap = {
            'online': '在线',
            'offline': '离线',
            'error': '错误'
        };
        return statusMap[status] || status;
    }
    /**
     * 获取设备状态颜色
     */
    static getDeviceStatusColor(status) {
        const colorMap = {
            'online': '#52c41a',
            'offline': '#999',
            'error': '#ff4d4f'
        };
        return colorMap[status] || '#999';
    }
    /**
     * 验证网络连接配置
     */
    static validateNetworkConfig(config) {
        return config.ip && config.port &&
            /^(\d{1,3}\.){3}\d{1,3}$/.test(config.ip) &&
            config.port > 0 && config.port < 65536;
    }
    /**
     * 验证蓝牙连接配置
     */
    static validateBluetoothConfig(config) {
        return config.mac_address &&
            /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/.test(config.mac_address);
    }
}
exports.DeviceAdapter = DeviceAdapter;
// ==================== 便捷函数 ====================
/**
 * 批量更新桌台状态
 */
function batchUpdateTableStatus(tableIds, status) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = tableIds.map(id => updateTableStatus(id, status));
        return Promise.all(promises);
    });
}
/**
 * 批量生成桌台二维码
 */
function batchGenerateQRCodes(tableIds) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = tableIds.map(id => generateTableQRCode(id));
        return Promise.all(promises);
    });
}
/**
 * 获取桌台类型统计信息
 */
function getTableTypeStats() {
    return __awaiter(this, void 0, void 0, function* () {
        const tables = yield getTables({ page_size: 1000 });
        const typeStats = new Map();
        tables.tables.forEach(table => {
            const type = table.table_type || 'table';
            if (!typeStats.has(type)) {
                typeStats.set(type, {
                    total: 0,
                    available: 0,
                    occupied: 0,
                    disabled: 0
                });
            }
            const stats = typeStats.get(type);
            stats.total++;
            stats[table.status]++;
        });
        return Array.from(typeStats.entries()).map(([type, stats]) => (Object.assign({ type }, stats)));
    });
}
/**
 * 检查桌台编号是否可用
 */
function checkTableNumberAvailable(tableNo, excludeId) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const tables = yield getTables({ page_size: 1000 });
            return !tables.tables.some(table => table.table_no === tableNo && table.id !== excludeId);
        }
        catch (error) {
            console.error('检查桌台编号失败:', error);
            return false;
        }
    });
}
