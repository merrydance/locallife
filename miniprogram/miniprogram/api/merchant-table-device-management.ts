/**
 * 商户桌台和设备管理API接口
 * 基于swagger.json重构，提供桌台管理和二维码生成功能
 */

import { request, API_BASE } from '../utils/request';
import { getToken } from '../utils/auth';

// ==================== 类型定义 ====================

export interface Table {
    id: number;
    table_no: string;
    table_type: 'table' | 'room';
    capacity: number;
    status: 'available' | 'occupied' | 'disabled';
    description?: string;
    minimum_spend?: number;
    qr_code_url?: string;
    merchant_id: number;
    current_reservation_id?: number;
    tags?: Array<{ id: number; name: string; }>;
    created_at: string;
    updated_at: string;
}

/** 创建桌台请求 - 对齐 api.createTableRequest */
export interface CreateTableRequest {
    capacity: number;
    description?: string;
    minimum_spend?: number;
    qr_code_url?: string;
    table_no: string;
    table_type: 'table' | 'room';
}

export interface UpdateTableRequest {
    table_no?: string;
    capacity?: number;
    description?: string;
    minimum_spend?: number;
    status?: 'available' | 'occupied' | 'disabled';
    qr_code_url?: string;
}

export interface GetTablesParams {
    page?: number;
    page_size?: number;
    table_type?: 'table' | 'room';
    status?: 'available' | 'occupied' | 'disabled';
}

export interface QRCodeResponse {
    table_no: string;
    qr_code_url: string;
}

export interface Device {
    id: number;
    device_type: 'printer' | 'display' | 'scanner';
    device_name: string;
    device_model: string;
    connection_type: 'usb' | 'network' | 'bluetooth';
    connection_config: Record<string, any>;
    status: 'online' | 'offline' | 'error';
    last_heartbeat?: string;
    created_at: string;
    updated_at: string;
}

export interface CreateDeviceRequest {
    device_type: 'printer' | 'display' | 'scanner';
    device_name: string;
    device_model: string;
    connection_type: 'usb' | 'network' | 'bluetooth';
    connection_config: Record<string, any>;
}

export interface UpdateDeviceRequest {
    device_name?: string;
    device_model?: string;
    connection_type?: 'usb' | 'network' | 'bluetooth';
    connection_config?: Record<string, any>;
}

export interface DisplayConfig {
    id: number;
    screen_orientation: 'portrait' | 'landscape';
    font_size: 'small' | 'medium' | 'large';
    theme: 'light' | 'dark';
    show_logo: boolean;
    show_weather: boolean;
    auto_refresh_interval: number;
    created_at: string;
    updated_at: string;
}

/** 更新显示配置请求 - 对齐 api.updateDisplayConfigRequest */
export interface UpdateDisplayConfigRequest {
    enable_kds?: boolean;
    enable_print?: boolean;
    enable_voice?: boolean;
    kds_url?: string;
    print_dine_in?: boolean;
    print_reservation?: boolean;
    print_takeout?: boolean;
    voice_dine_in?: boolean;
    voice_takeout?: boolean;
}

// ==================== 桌台管理接口 ====================

/**
 * 获取桌台列表
 */
export async function getTables(params: GetTablesParams = {}) {
    // 手动构建查询字符串（小程序不支持 URLSearchParams）
    const queryParts: string[] = [];

    if (params.page) queryParts.push(`page=${params.page}`);
    if (params.page_size) queryParts.push(`page_size=${params.page_size}`);
    if (params.table_type) queryParts.push(`table_type=${params.table_type}`);
    if (params.status) queryParts.push(`status=${params.status}`);

    const queryString = queryParts.length > 0 ? '?' + queryParts.join('&') : '';
    const url = `/v1/tables${queryString}`;

    return request<{
        tables: Table[];
        total: number;
    }>({
        url,
        method: 'GET'
    });
}

/**
 * 获取单个桌台信息
 */
export async function getTable(tableId: number) {
    return request<Table>({
        url: `/v1/tables/${tableId}`,
        method: 'GET'
    });
}

/**
 * 创建桌台
 */
export async function createTable(data: CreateTableRequest) {
    return request<Table>({
        url: '/v1/tables',
        method: 'POST',
        data
    });
}

/**
 * 更新桌台信息
 */
export async function updateTable(tableId: number, data: UpdateTableRequest) {
    return request<Table>({
        url: `/v1/tables/${tableId}`,
        method: 'PATCH',
        data
    });
}

/**
 * 删除桌台
 */
export async function deleteTable(tableId: number) {
    return request<void>({
        url: `/v1/tables/${tableId}`,
        method: 'DELETE'
    });
}

/**
 * 更新桌台状态
 */
export async function updateTableStatus(tableId: number, status: string) {
    return request<Table>({
        url: `/v1/tables/${tableId}/status`,
        method: 'PATCH',
        data: { status }
    });
}

// ==================== 二维码管理接口 ====================

/**
 * 生成桌台二维码
 */
export async function generateTableQRCode(tableId: number, regenerate: boolean = false) {
    return request<QRCodeResponse>({
        url: `/v1/tables/${tableId}/qrcode`,
        method: 'GET'
    });
}

/**
 * 获取桌台二维码
 */
export async function getTableQRCode(tableId: number) {
    return request<QRCodeResponse>({
        url: `/v1/tables/${tableId}/qrcode`,
        method: 'GET'
    });
}

/**
 * 上传桌台图片
 */
export async function uploadTableImages(tableId: number, images: string[]) {
    return request<{ image_urls: string[] }>({
        url: `/v1/tables/${tableId}/images`,
        method: 'POST',
        data: { images }
    });
}

/**
 * 上传桌台图片文件
 */
export async function uploadTableImage(filePath: string): Promise<string> {
    return new Promise((resolve, reject) => {
        const token = getToken();
        wx.uploadFile({
            url: `${API_BASE}/v1/tables/images/upload`,
            filePath,
            name: 'image',
            header: {
                'Authorization': `Bearer ${token}`
            },
            success: (res) => {
                if (res.statusCode === 200) {
                    try {
                        const data = JSON.parse(res.data);
                        // api.uploadImageResponse
                        if (data.code === 0 && data.data && data.data.image_url) {
                            resolve(data.data.image_url);
                        } else if (data.image_url) {
                            resolve(data.image_url);
                        } else {
                            // Try fallback if data IS the object
                            resolve(data.data?.image_url || data.image_url);
                        }
                    } catch (e) {
                        reject(new Error('Parse response failed'));
                    }
                } else {
                    reject(new Error(`HTTP ${res.statusCode}`));
                }
            },
            fail: reject
        });
    });
}

// ==================== 设备管理接口 ====================

/**
 * 获取设备列表
 */
export async function getDevices() {
    return request<{
        data: Device[];
        total: number;
    }>({
        url: '/v1/merchant/devices',
        method: 'GET'
    });
}

/**
 * 获取单个设备信息
 */
export async function getDevice(deviceId: number) {
    return request<Device>({
        url: `/v1/merchant/devices/${deviceId}`,
        method: 'GET'
    });
}

/**
 * 添加设备
 */
export async function createDevice(data: CreateDeviceRequest) {
    return request<Device>({
        url: '/v1/merchant/devices',
        method: 'POST',
        data
    });
}

/**
 * 更新设备信息
 */
export async function updateDevice(deviceId: number, data: UpdateDeviceRequest) {
    return request<Device>({
        url: `/v1/merchant/devices/${deviceId}`,
        method: 'PATCH',
        data
    });
}

/**
 * 删除设备
 */
export async function deleteDevice(deviceId: number) {
    return request<void>({
        url: `/v1/merchant/devices/${deviceId}`,
        method: 'DELETE'
    });
}

/**
 * 测试设备连接
 */
export async function testDevice(deviceId: number) {
    return request<{
        success: boolean;
        message: string;
        response_time?: number;
    }>({
        url: `/v1/merchant/devices/${deviceId}/test`,
        method: 'POST'
    });
}

// ==================== 显示配置接口 ====================

/**
 * 获取显示配置
 */
export async function getDisplayConfig() {
    return request<DisplayConfig>({
        url: '/v1/merchant/display-config',
        method: 'GET'
    });
}

/**
 * 更新显示配置
 */
export async function updateDisplayConfig(data: UpdateDisplayConfigRequest) {
    return request<DisplayConfig>({
        url: '/v1/merchant/display-config',
        method: 'PATCH',
        data
    });
}

// ==================== 数据适配器 ====================

/**
 * 桌台数据适配器
 */
export class TableAdapter {
    /**
     * 格式化桌台状态
     */
    static formatStatus(status: string): string {
        const statusMap: Record<string, string> = {
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
    static getStatusColor(status: string): string {
        const colorMap: Record<string, string> = {
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
    static formatTableInfo(table: Table): string {
        const parts = [];
        if (table.capacity) parts.push(`${table.capacity}人桌`);
        // if (table.area) parts.push(table.area); // area removed from interface
        if (table.description) parts.push(table.description);
        return parts.join(' · ');
    }

    /**
     * 验证桌台编号格式
     */
    static validateTableNumber(tableNumber: string): boolean {
        // 桌台编号应该是字母+数字的组合，如：A01, B12
        return /^[A-Z]\d{2,3}$/.test(tableNumber);
    }

    /**
     * 生成桌台编号建议
     */
    static generateTableNumberSuggestion(area: string, existingNumbers: string[]): string {
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

/**
 * 设备数据适配器
 */
export class DeviceAdapter {
    /**
     * 格式化设备类型
     */
    static formatDeviceType(type: string): string {
        const typeMap: Record<string, string> = {
            'printer': '打印机',
            'display': '显示屏',
            'scanner': '扫码器'
        };
        return typeMap[type] || type;
    }

    /**
     * 格式化连接类型
     */
    static formatConnectionType(type: string): string {
        const typeMap: Record<string, string> = {
            'usb': 'USB连接',
            'network': '网络连接',
            'bluetooth': '蓝牙连接'
        };
        return typeMap[type] || type;
    }

    /**
     * 格式化设备状态
     */
    static formatDeviceStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'online': '在线',
            'offline': '离线',
            'error': '错误'
        };
        return statusMap[status] || status;
    }

    /**
     * 获取设备状态颜色
     */
    static getDeviceStatusColor(status: string): string {
        const colorMap: Record<string, string> = {
            'online': '#52c41a',
            'offline': '#999',
            'error': '#ff4d4f'
        };
        return colorMap[status] || '#999';
    }

    /**
     * 验证网络连接配置
     */
    static validateNetworkConfig(config: any): boolean {
        return config.ip && config.port &&
            /^(\d{1,3}\.){3}\d{1,3}$/.test(config.ip) &&
            config.port > 0 && config.port < 65536;
    }

    /**
     * 验证蓝牙连接配置
     */
    static validateBluetoothConfig(config: any): boolean {
        return config.mac_address &&
            /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/.test(config.mac_address);
    }
}

// ==================== 便捷函数 ====================

/**
 * 批量更新桌台状态
 */
export async function batchUpdateTableStatus(tableIds: number[], status: string) {
    const promises = tableIds.map(id => updateTableStatus(id, status));
    return Promise.all(promises);
}

/**
 * 批量生成桌台二维码
 */
export async function batchGenerateQRCodes(tableIds: number[]) {
    const promises = tableIds.map(id => generateTableQRCode(id));
    return Promise.all(promises);
}

/**
 * 获取桌台类型统计信息
 */
export async function getTableTypeStats() {
    const tables = await getTables({ page_size: 1000 });
    const typeStats = new Map<string, {
        total: number;
        available: number;
        occupied: number;
        disabled: number;
    }>();

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

        const stats = typeStats.get(type)!;
        stats.total++;
        stats[table.status as keyof typeof stats]++;
    });

    return Array.from(typeStats.entries()).map(([type, stats]) => ({
        type,
        ...stats
    }));
}

/**
 * 检查桌台编号是否可用
 */
export async function checkTableNumberAvailable(tableNo: string, excludeId?: number) {
    try {
        const tables = await getTables({ page_size: 1000 });
        return !tables.tables.some(table =>
            table.table_no === tableNo && table.id !== excludeId
        );
    } catch (error) {
        console.error('检查桌台编号失败:', error);
        return false;
    }
}