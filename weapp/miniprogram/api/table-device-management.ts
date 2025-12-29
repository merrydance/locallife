/**
 * 桌台和设备管理接口重构 (Task 2.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：桌台管理、设备管理、显示配置、二维码管理
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 桌台类型枚举 - 基于swagger定义 */
export type TableType = 'table' | 'room'

/** 桌台状态枚举 */
export type TableStatus = 'available' | 'occupied' | 'reserved' | 'disabled'

/** 打印机类型枚举 - 基于swagger定义 */
export type PrinterType = 'feieyun' | 'yilianyun' | 'other'

// ==================== 桌台管理相关类型 ====================

/** 创建桌台请求 - 基于swagger api.createTableRequest */
export interface CreateTableRequest extends Record<string, unknown> {
    table_no: string
    table_type: TableType
    capacity: number
    description?: string
    minimum_spend?: number
    qr_code_url?: string
    tag_ids?: number[]  // 标签ID列表
}

/** 更新桌台请求 - 基于swagger api.updateTableRequest */
/** 更新桌台请求 - 对齐 api.updateTableRequest */
export interface UpdateTableRequest extends Record<string, unknown> {
    capacity?: number
    description?: string
    minimum_spend?: number
    qr_code_url?: string
    status?: 'available' | 'occupied' | 'disabled'
    table_no?: string
    tag_ids?: number[]  // 标签ID列表
}

/** 桌台标签信息 - 基于swagger api.tagInfo */
export interface TagInfo {
    id: number
    name: string
}

/** 桌台响应 - 基于swagger api.tableResponse */
export interface TableResponse {
    id: number
    merchant_id: number
    table_no: string
    table_type: TableType
    capacity: number
    description?: string
    minimum_spend?: number
    qr_code_url?: string
    status: string
    tags?: TagInfo[]
    current_reservation_id?: number
    created_at: string
    updated_at: string
}

/** 桌台列表响应 - 对齐 api.listTablesResponse */
export interface ListTablesResponse {
    count?: number                               // 总数
    tables?: TableResponse[]
}

/** 桌台图片响应 - 对齐 api.tableImageResponse */
export interface TableImageResponse {
    id?: number
    image_url?: string
    is_primary?: boolean
    sort_order?: number
    table_id?: number
}

/** 更新桌台状态请求 - 对齐 api.updateTableStatusRequest */
export interface UpdateTableStatusRequest extends Record<string, unknown> {
    current_reservation_id?: number              // 当前预定ID
    status: 'available' | 'occupied' | 'disabled'
}

// ==================== 设备管理相关类型 ====================

/** 创建打印机请求 - 基于swagger api.createPrinterRequest */
export interface CreatePrinterRequest extends Record<string, unknown> {
    printer_name: string
    printer_type: PrinterType
    printer_sn: string
    printer_key: string
    print_takeout?: boolean
    print_dine_in?: boolean
    print_reservation?: boolean
}

/** 更新打印机请求 - 对齐 api.updatePrinterRequest */
export interface UpdatePrinterRequest extends Record<string, unknown> {
    is_active?: boolean
    print_dine_in?: boolean
    print_reservation?: boolean
    print_takeout?: boolean
    printer_key?: string                         // 打印机密钥
    printer_name?: string
}

/** 打印机响应 - 基于swagger api.printerResponse */
export interface PrinterResponse {
    id: number
    merchant_id: number
    printer_name: string
    printer_type: PrinterType
    printer_sn: string
    is_active: boolean
    print_takeout: boolean
    print_dine_in: boolean
    print_reservation: boolean
    created_at: string
    updated_at: string
}

/** 打印机测试请求 */
export interface TestPrinterRequest extends Record<string, unknown> {
    test_content?: string
}

// ==================== 显示配置相关类型 ====================

/** 显示配置响应 - 基于swagger api.getDisplayConfigResponse */
export interface DisplayConfigResponse {
    id: number
    merchant_id: number
    enable_print: boolean
    enable_voice: boolean
    enable_kds: boolean
    print_takeout: boolean
    print_dine_in: boolean
    print_reservation: boolean
    voice_takeout: boolean
    voice_dine_in: boolean
    kds_url?: string
    created_at: string
    updated_at: string
}

/** 更新显示配置请求 - 基于swagger api.updateDisplayConfigRequest */
export interface UpdateDisplayConfigRequest extends Record<string, unknown> {
    enable_print?: boolean
    enable_voice?: boolean
    enable_kds?: boolean
    print_takeout?: boolean
    print_dine_in?: boolean
    print_reservation?: boolean
    voice_takeout?: boolean
    voice_dine_in?: boolean
    kds_url?: string
}

// ==================== 桌台管理服务类 ====================

/**
 * 桌台管理服务
 * 提供桌台的CRUD操作、状态管理、二维码管理等功能
 */
export class TableManagementService {
    /**
     * 获取桌台列表
     * @param tableType 桌台类型筛选
     */
    async listTables(tableType?: TableType): Promise<ListTablesResponse> {
        const params: Record<string, unknown> = {}
        if (tableType) {
            params.table_type = tableType
        }

        return request({
            url: '/v1/tables',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取桌台详情
     * @param tableId 桌台ID
     */
    async getTableDetail(tableId: number): Promise<TableResponse> {
        return request({
            url: `/v1/tables/${tableId}`,
            method: 'GET'
        })
    }

    /**
     * 创建桌台
     * @param tableData 桌台数据
     */
    async createTable(tableData: CreateTableRequest): Promise<TableResponse> {
        return request({
            url: '/v1/tables',
            method: 'POST',
            data: tableData
        })
    }

    /**
     * 更新桌台信息
     * @param tableId 桌台ID
     * @param tableData 更新数据
     */
    async updateTable(tableId: number, tableData: UpdateTableRequest): Promise<TableResponse> {
        return request({
            url: `/v1/tables/${tableId}`,
            method: 'PATCH',
            data: tableData
        })
    }

    /**
     * 删除桌台
     * @param tableId 桌台ID
     */
    async deleteTable(tableId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/tables/${tableId}`,
            method: 'DELETE'
        })
    }

    /**
     * 更新桌台状态
     * @param tableId 桌台ID
     * @param statusData 状态数据
     */
    async updateTableStatus(tableId: number, statusData: UpdateTableStatusRequest): Promise<TableResponse> {
        return request({
            url: `/v1/tables/${tableId}/status`,
            method: 'PATCH',
            data: statusData
        })
    }

    /**
     * 获取桌台二维码
     * @param tableId 桌台ID
     */
    async getTableQRCode(tableId: number): Promise<{ qr_code_url: string }> {
        return request({
            url: `/v1/tables/${tableId}/qrcode`,
            method: 'GET'
        })
    }

    /**
     * 获取桌台图片列表
     * @param tableId 桌台ID
     */
    async getTableImages(tableId: number): Promise<{ images: TableImageResponse[] }> {
        return request({
            url: `/v1/tables/${tableId}/images`,
            method: 'GET'
        })
    }

    /**
     * 上传桌台图片
     * @param tableId 桌台ID
     * @param imageData 图片数据
     */
    async uploadTableImage(tableId: number, imageData: { image_url: string; sort_order?: number }): Promise<TableImageResponse> {
        return request({
            url: `/v1/tables/${tableId}/images`,
            method: 'POST',
            data: imageData
        })
    }

    /**
     * 删除桌台图片
     * @param tableId 桌台ID
     * @param imageId 图片ID
     */
    async deleteTableImage(tableId: number, imageId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/tables/${tableId}/images/${imageId}`,
            method: 'DELETE'
        })
    }

    /**
     * 设置主图片
     * @param tableId 桌台ID
     * @param imageId 图片ID
     */
    async setPrimaryTableImage(tableId: number, imageId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/tables/${tableId}/images/${imageId}/primary`,
            method: 'PUT'
        })
    }

    /**
     * 获取桌台标签
     * @param tableId 桌台ID
     */
    async getTableTags(tableId: number): Promise<{ tags: TagInfo[] }> {
        return request({
            url: `/v1/tables/${tableId}/tags`,
            method: 'GET'
        })
    }

    /**
     * 添加桌台标签
     * @param tableId 桌台ID
     * @param tagData 标签数据
     */
    async addTableTag(tableId: number, tagData: { name: string }): Promise<TagInfo> {
        return request({
            url: `/v1/tables/${tableId}/tags`,
            method: 'POST',
            data: tagData
        })
    }

    /**
     * 删除桌台标签
     * @param tableId 桌台ID
     * @param tagId 标签ID
     */
    async deleteTableTag(tableId: number, tagId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/tables/${tableId}/tags/${tagId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 设备管理服务类 ====================

/**
 * 设备管理服务
 * 提供打印机设备的注册、配置、测试等功能
 */
export class DeviceManagementService {
    /**
     * 获取打印机列表
     * @param onlyActive 是否只返回启用的打印机
     */
    async listPrinters(onlyActive?: boolean): Promise<{ printers: PrinterResponse[] }> {
        const params: Record<string, unknown> = {}
        if (onlyActive !== undefined) {
            params.only_active = onlyActive
        }

        return request({
            url: '/v1/merchant/devices',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取打印机详情
     * @param printerId 打印机ID
     */
    async getPrinterDetail(printerId: number): Promise<PrinterResponse> {
        return request({
            url: `/v1/merchant/devices/${printerId}`,
            method: 'GET'
        })
    }

    /**
     * 注册打印机
     * @param printerData 打印机数据
     */
    async createPrinter(printerData: CreatePrinterRequest): Promise<PrinterResponse> {
        return request({
            url: '/v1/merchant/devices',
            method: 'POST',
            data: printerData
        })
    }

    /**
     * 更新打印机配置
     * @param printerId 打印机ID
     * @param printerData 更新数据
     */
    async updatePrinter(printerId: number, printerData: UpdatePrinterRequest): Promise<PrinterResponse> {
        return request({
            url: `/v1/merchant/devices/${printerId}`,
            method: 'PUT',
            data: printerData
        })
    }

    /**
     * 删除打印机
     * @param printerId 打印机ID
     */
    async deletePrinter(printerId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/merchant/devices/${printerId}`,
            method: 'DELETE'
        })
    }

    /**
     * 测试打印机
     * @param printerId 打印机ID
     * @param testData 测试数据
     */
    async testPrinter(printerId: number, testData?: TestPrinterRequest): Promise<{ message: string; success: boolean }> {
        return request({
            url: `/v1/merchant/devices/${printerId}/test`,
            method: 'POST',
            data: testData || {}
        })
    }
}

// ==================== 显示配置服务类 ====================

/**
 * 显示配置服务
 * 提供订单展示配置管理，包括打印、语音播报、KDS等设置
 */
export class DisplayConfigService {
    /**
     * 获取订单展示配置
     */
    async getDisplayConfig(): Promise<DisplayConfigResponse> {
        return request({
            url: '/v1/merchant/display-config',
            method: 'GET'
        })
    }

    /**
     * 更新订单展示配置
     * @param configData 配置数据
     */
    async updateDisplayConfig(configData: UpdateDisplayConfigRequest): Promise<DisplayConfigResponse> {
        return request({
            url: '/v1/merchant/display-config',
            method: 'PUT',
            data: configData
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 桌台和设备管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class TableDeviceAdapter {
    /**
     * 适配创建桌台请求数据
     */
    static adaptCreateTableRequest(data: {
        tableNo: string
        tableType: TableType
        capacity: number
        description?: string
        minimumSpend?: number
    }): CreateTableRequest {
        return {
            table_no: data.tableNo,
            table_type: data.tableType,
            capacity: data.capacity,
            description: data.description,
            minimum_spend: data.minimumSpend
        }
    }

    /**
     * 适配桌台响应数据
     */
    static adaptTableResponse(data: TableResponse): {
        id: number
        merchantId: number
        tableNo: string
        tableType: TableType
        capacity: number
        description?: string
        minimumSpend?: number
        qrCodeUrl?: string
        status: string
        tags?: TagInfo[]
        currentReservationId?: number
        createdAt: string
        updatedAt: string
    } {
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
        }
    }

    /**
     * 适配创建打印机请求数据
     */
    static adaptCreatePrinterRequest(data: {
        printerName: string
        printerType: PrinterType
        printerSn: string
        printerKey: string
        printTakeout?: boolean
        printDineIn?: boolean
        printReservation?: boolean
    }): CreatePrinterRequest {
        return {
            printer_name: data.printerName,
            printer_type: data.printerType,
            printer_sn: data.printerSn,
            printer_key: data.printerKey,
            print_takeout: data.printTakeout,
            print_dine_in: data.printDineIn,
            print_reservation: data.printReservation
        }
    }

    /**
     * 适配打印机响应数据
     */
    static adaptPrinterResponse(data: PrinterResponse): {
        id: number
        merchantId: number
        printerName: string
        printerType: PrinterType
        printerSn: string
        isActive: boolean
        printTakeout: boolean
        printDineIn: boolean
        printReservation: boolean
        createdAt: string
        updatedAt: string
    } {
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
        }
    }

    /**
     * 适配显示配置响应数据
     */
    static adaptDisplayConfigResponse(data: DisplayConfigResponse): {
        id: number
        merchantId: number
        enablePrint: boolean
        enableVoice: boolean
        enableKds: boolean
        printTakeout: boolean
        printDineIn: boolean
        printReservation: boolean
        voiceTakeout: boolean
        voiceDineIn: boolean
        kdsUrl?: string
        createdAt: string
        updatedAt: string
    } {
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
        }
    }
}

// ==================== 导出服务实例 ====================

export const tableManagementService = new TableManagementService()
export const deviceManagementService = new DeviceManagementService()
export const displayConfigService = new DisplayConfigService()

// ==================== 便捷函数 ====================

/**
 * 获取可用桌台列表
 */
export async function getAvailableTables(): Promise<TableResponse[]> {
    const response = await tableManagementService.listTables('table')
    return response.tables.filter(table => table.status === 'available')
}

/**
 * 获取包间列表
 */
export async function getPrivateRooms(): Promise<TableResponse[]> {
    const response = await tableManagementService.listTables('room')
    return response.tables
}

/**
 * 获取启用的打印机列表
 */
export async function getActivePrinters(): Promise<PrinterResponse[]> {
    const response = await deviceManagementService.listPrinters(true)
    return response.printers
}

/**
 * 批量测试打印机
 * @param printerIds 打印机ID列表
 */
export async function batchTestPrinters(printerIds: number[]): Promise<{ printerId: number; success: boolean; message: string }[]> {
    const promises = printerIds.map(async (printerId) => {
        try {
            const result = await deviceManagementService.testPrinter(printerId)
            return { printerId, success: result.success, message: result.message }
        } catch (error: any) {
            return {
                printerId,
                success: false,
                message: error?.message || '测试失败'
            }
        }
    })

    return Promise.all(promises)
}