"use strict";
/**
 * 桌台和设备管理接口重构 (Task 2.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：桌台管理、设备管理、显示配置、二维码管理
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
exports.displayConfigService = exports.deviceManagementService = exports.tableManagementService = exports.TableDeviceAdapter = exports.DisplayConfigService = exports.DeviceManagementService = exports.TableManagementService = void 0;
exports.getAvailableTables = getAvailableTables;
exports.getPrivateRooms = getPrivateRooms;
exports.getActivePrinters = getActivePrinters;
exports.batchTestPrinters = batchTestPrinters;
const request_1 = require("../utils/request");
// ==================== 桌台管理服务类 ====================
/**
 * 桌台管理服务
 * 提供桌台的CRUD操作、状态管理、二维码管理等功能
 */
class TableManagementService {
    /**
     * 获取桌台列表
     * @param tableType 桌台类型筛选
     */
    listTables(tableType) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = {};
            if (tableType) {
                params.table_type = tableType;
            }
            return (0, request_1.request)({
                url: '/v1/tables',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取桌台详情
     * @param tableId 桌台ID
     */
    getTableDetail(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建桌台
     * @param tableData 桌台数据
     */
    createTable(tableData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/tables',
                method: 'POST',
                data: tableData
            });
        });
    }
    /**
     * 更新桌台信息
     * @param tableId 桌台ID
     * @param tableData 更新数据
     */
    updateTable(tableId, tableData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}`,
                method: 'PATCH',
                data: tableData
            });
        });
    }
    /**
     * 删除桌台
     * @param tableId 桌台ID
     */
    deleteTable(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 更新桌台状态
     * @param tableId 桌台ID
     * @param statusData 状态数据
     */
    updateTableStatus(tableId, statusData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/status`,
                method: 'PATCH',
                data: statusData
            });
        });
    }
    /**
     * 获取桌台二维码
     * @param tableId 桌台ID
     */
    getTableQRCode(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/qrcode`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取桌台图片列表
     * @param tableId 桌台ID
     */
    getTableImages(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/images`,
                method: 'GET'
            });
        });
    }
    /**
     * 上传桌台图片
     * @param tableId 桌台ID
     * @param imageData 图片数据
     */
    uploadTableImage(tableId, imageData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/images`,
                method: 'POST',
                data: imageData
            });
        });
    }
    /**
     * 删除桌台图片
     * @param tableId 桌台ID
     * @param imageId 图片ID
     */
    deleteTableImage(tableId, imageId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/images/${imageId}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 设置主图片
     * @param tableId 桌台ID
     * @param imageId 图片ID
     */
    setPrimaryTableImage(tableId, imageId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/images/${imageId}/primary`,
                method: 'PUT'
            });
        });
    }
    /**
     * 获取桌台标签
     * @param tableId 桌台ID
     */
    getTableTags(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/tags`,
                method: 'GET'
            });
        });
    }
    /**
     * 添加桌台标签
     * @param tableId 桌台ID
     * @param tagData 标签数据
     */
    addTableTag(tableId, tagData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/tags`,
                method: 'POST',
                data: tagData
            });
        });
    }
    /**
     * 删除桌台标签
     * @param tableId 桌台ID
     * @param tagId 标签ID
     */
    deleteTableTag(tableId, tagId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/tables/${tableId}/tags/${tagId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.TableManagementService = TableManagementService;
// ==================== 设备管理服务类 ====================
/**
 * 设备管理服务
 * 提供打印机设备的注册、配置、测试等功能
 */
class DeviceManagementService {
    /**
     * 获取打印机列表
     * @param onlyActive 是否只返回启用的打印机
     */
    listPrinters(onlyActive) {
        return __awaiter(this, void 0, void 0, function* () {
            const params = {};
            if (onlyActive !== undefined) {
                params.only_active = onlyActive;
            }
            return (0, request_1.request)({
                url: '/v1/merchant/devices',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取打印机详情
     * @param printerId 打印机ID
     */
    getPrinterDetail(printerId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/devices/${printerId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 注册打印机
     * @param printerData 打印机数据
     */
    createPrinter(printerData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/devices',
                method: 'POST',
                data: printerData
            });
        });
    }
    /**
     * 更新打印机配置
     * @param printerId 打印机ID
     * @param printerData 更新数据
     */
    updatePrinter(printerId, printerData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/devices/${printerId}`,
                method: 'PUT',
                data: printerData
            });
        });
    }
    /**
     * 删除打印机
     * @param printerId 打印机ID
     */
    deletePrinter(printerId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/devices/${printerId}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 测试打印机
     * @param printerId 打印机ID
     * @param testData 测试数据
     */
    testPrinter(printerId, testData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/devices/${printerId}/test`,
                method: 'POST',
                data: testData || {}
            });
        });
    }
}
exports.DeviceManagementService = DeviceManagementService;
// ==================== 显示配置服务类 ====================
/**
 * 显示配置服务
 * 提供订单展示配置管理，包括打印、语音播报、KDS等设置
 */
class DisplayConfigService {
    /**
     * 获取订单展示配置
     */
    getDisplayConfig() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/display-config',
                method: 'GET'
            });
        });
    }
    /**
     * 更新订单展示配置
     * @param configData 配置数据
     */
    updateDisplayConfig(configData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/display-config',
                method: 'PUT',
                data: configData
            });
        });
    }
}
exports.DisplayConfigService = DisplayConfigService;
// ==================== 数据适配器 ====================
/**
 * 桌台和设备管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class TableDeviceAdapter {
    /**
     * 适配创建桌台请求数据
     */
    static adaptCreateTableRequest(data) {
        return {
            table_no: data.tableNo,
            table_type: data.tableType,
            capacity: data.capacity,
            description: data.description,
            minimum_spend: data.minimumSpend
        };
    }
    /**
     * 适配桌台响应数据
     */
    static adaptTableResponse(data) {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            tableNo: data.table_no,
            tableType: data.table_type,
            capacity: data.capacity,
            description: data.description,
            minimumSpend: data.minimum_spend,
            qrCodeUrl: data.qr_code_url,
            status: data.status,
            tags: data.tags,
            currentReservationId: data.current_reservation_id,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        };
    }
    /**
     * 适配创建打印机请求数据
     */
    static adaptCreatePrinterRequest(data) {
        return {
            printer_name: data.printerName,
            printer_type: data.printerType,
            printer_sn: data.printerSn,
            printer_key: data.printerKey,
            print_takeout: data.printTakeout,
            print_dine_in: data.printDineIn,
            print_reservation: data.printReservation
        };
    }
    /**
     * 适配打印机响应数据
     */
    static adaptPrinterResponse(data) {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            printerName: data.printer_name,
            printerType: data.printer_type,
            printerSn: data.printer_sn,
            isActive: data.is_active,
            printTakeout: data.print_takeout,
            printDineIn: data.print_dine_in,
            printReservation: data.print_reservation,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        };
    }
    /**
     * 适配显示配置响应数据
     */
    static adaptDisplayConfigResponse(data) {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            enablePrint: data.enable_print,
            enableVoice: data.enable_voice,
            enableKds: data.enable_kds,
            printTakeout: data.print_takeout,
            printDineIn: data.print_dine_in,
            printReservation: data.print_reservation,
            voiceTakeout: data.voice_takeout,
            voiceDineIn: data.voice_dine_in,
            kdsUrl: data.kds_url,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        };
    }
}
exports.TableDeviceAdapter = TableDeviceAdapter;
// ==================== 导出服务实例 ====================
exports.tableManagementService = new TableManagementService();
exports.deviceManagementService = new DeviceManagementService();
exports.displayConfigService = new DisplayConfigService();
// ==================== 便捷函数 ====================
/**
 * 获取可用桌台列表
 */
function getAvailableTables() {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield exports.tableManagementService.listTables('table');
        return response.tables.filter(table => table.status === 'available');
    });
}
/**
 * 获取包间列表
 */
function getPrivateRooms() {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield exports.tableManagementService.listTables('room');
        return response.tables;
    });
}
/**
 * 获取启用的打印机列表
 */
function getActivePrinters() {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield exports.deviceManagementService.listPrinters(true);
        return response.printers;
    });
}
/**
 * 批量测试打印机
 * @param printerIds 打印机ID列表
 */
function batchTestPrinters(printerIds) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = printerIds.map((printerId) => __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield exports.deviceManagementService.testPrinter(printerId);
                return { printerId, success: result.success, message: result.message };
            }
            catch (error) {
                return {
                    printerId,
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || '测试失败'
                };
            }
        }));
        return Promise.all(promises);
    });
}
