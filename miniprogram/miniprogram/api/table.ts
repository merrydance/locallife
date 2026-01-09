/**
 * 扫码点餐相关API接口 (顾客端功能)
 * 基于swagger.json中的扫码点餐接口
 * 商户端桌台管理功能已迁移到 table-device-management.ts
 */

import { request } from '../utils/request'
import type { DishResponse } from './dish'

// ==================== 数据类型定义 ====================

/** 桌台状态枚举 */
export type TableStatus =
    | 'available'   // 可用
    | 'occupied'    // 占用中
    | 'reserved'    // 已预定
    | 'disabled'    // 已停用

/** 桌台类型枚举 */
export type TableType =
    | 'regular'     // 普通桌台
    | 'private'     // 包间
    | 'bar'         // 吧台
    | 'outdoor'     // 户外

/** 扫码点餐商户信息 - 对齐 api.scanTableMerchantInfo */
export interface ScanTableMerchantInfo {
    address: string
    description?: string
    id: number
    logo_url: string
    name: string
    phone: string
    status: string
}

/** 扫码点餐桌台信息 */
export interface ScanTableTableInfo {
    id: number
    table_no: string
    table_type: TableType
    capacity: number
    minimum_spend?: number
    description?: string
    status: TableStatus
}

/** 扫码点餐分类信息 */
export interface ScanTableCategoryInfo {
    id: number
    name: string
    sort_order: number
    dishes: DishResponse[]
}

/** 扫码点餐套餐信息 - 对齐 api.scanTableComboInfo */
export interface ScanTableComboInfo {
    description?: string
    id: number
    image_url?: string
    is_available: boolean
    name: string
    original_price?: number
    price: number
}

/** 扫码点餐促销信息 - 对齐 api.scanTablePromotionInfo */
export interface ScanTablePromotionInfo {
    description: string
    min_amount: number                           // 最低金额
    return_value: number                         // 满返金额或满减金额
    type: string                                 // delivery_return / discount
}

/** 扫码点餐响应 */
export interface ScanTableResponse {
    merchant: ScanTableMerchantInfo
    table: ScanTableTableInfo
    categories: ScanTableCategoryInfo[]
    combos: ScanTableComboInfo[]
    promotions: ScanTablePromotionInfo[]
}

/** 桌台详情响应 - 对齐 api.tableResponse */
export interface TableResponse {
    capacity: number
    created_at: string
    current_reservation_id?: number              // 当前预定ID
    description?: string
    id: number
    merchant_id: number
    minimum_spend?: number
    qr_code_url?: string
    status: string
    table_no: string
    table_type: string
    tags?: TableTag[]
    updated_at: string
}

/** 桌台标签 */
export interface TableTag {
    id: number
    name: string
    color?: string
}

/** 桌台标签信息 - 对齐 api.tableTagInfo */
export interface TableTagInfo {
    id: number                                   // 标签ID
    name: string                                 // 标签名称
    type: string                                 // 标签类型
}

/** 标签响应 - 对齐 api.tagResponse */
export interface TagResponse {
    id: number                                   // 标签ID
    name: string                                 // 标签名称
}

/** 更新桌台状态请求 - 对齐 api.updateTableStatusRequest */
export interface UpdateTableStatusRequest extends Record<string, unknown> {
    current_reservation_id?: number              // 当前预定ID
    status: 'available' | 'occupied' | 'disabled'
}

/** 添加桌台图片请求 - 对齐 api.addTableImageRequest */
export interface AddTableImageRequest extends Record<string, unknown> {
    image_url: string                            // 图片URL（最大500字符，必填）
    is_primary?: boolean                         // 是否主图
    sort_order?: number                          // 排序（0-100）
}

/** 添加桌台标签请求 - 对齐 api.addTableTagRequest */
export interface AddTableTagRequest extends Record<string, unknown> {
    tag_id: number                               // 标签ID（必填）
}

/** 生成桌台二维码响应 - 对齐 api.generateTableQRCodeResponse */
export interface GenerateTableQRCodeResponse {
    merchant_id: number                          // 商户ID
    table_no: string                             // 桌台编号
    qr_code_url: string                          // 二维码URL
}

/** 扫码点餐菜品信息 - 对齐 api.scanTableDishInfo */
export interface ScanTableDishInfo {
    id: number                                   // 菜品ID
    name: string                                 // 菜品名称
    description?: string                         // 菜品描述
    price: number                                // 价格（分）
    member_price?: number                        // 会员价（分）
    image_url?: string                           // 图片URL
    is_available: boolean                        // 是否可用
    category_id: number                          // 分类ID
    category_name: string                        // 分类名称
    sort_order: number                           // 排序
}

// ==================== API接口函数 ====================

/**
 * 扫码点餐 - 顾客扫码获取商户和桌台信息
 * 顾客扫描桌台二维码后，获取完整的菜单信息进行点餐
 * @param merchantId 商户ID
 * @param tableNo 桌台编号
 */
export async function scanTable(merchantId: number, tableNo: string): Promise<ScanTableResponse> {
    return request({
        url: '/v1/scan/table',
        method: 'GET',
        data: { merchant_id: merchantId, table_no: tableNo }
    })
}

/**
 * 获取桌台详情
 * @param tableId 桌台ID
 */
export async function getTableDetail(tableId: number): Promise<TableResponse> {
    return request({
        url: `/v1/tables/${tableId}`,
        method: 'GET'
    })
}

// ==================== 注意 ====================
// 商户端桌台管理功能已迁移到 table-device-management.ts
// 包括：桌台CRUD、状态管理、二维码管理、图片管理、标签管理等

// ==================== 便捷方法 ====================

/**
 * 通过二维码URL解析商户和桌台信息
 * @param qrCodeUrl 二维码URL
 */
export function parseQRCodeUrl(qrCodeUrl: string): { merchantId: number; tableNo: string } | null {
    try {
        const url = new URL(qrCodeUrl)
        const merchantId = url.searchParams.get('merchant_id')
        const tableNo = url.searchParams.get('table_no')

        if (merchantId && tableNo) {
            return {
                merchantId: parseInt(merchantId),
                tableNo
            }
        }
    } catch (error) {
        console.error('Invalid QR code URL:', error)
    }

    return null
}

/**
 * 生成桌台二维码URL
 * @param merchantId 商户ID
 * @param tableNo 桌台编号
 * @param baseUrl 基础URL
 */
export function generateQRCodeUrl(merchantId: number, tableNo: string, baseUrl: string = 'https://api.example.com'): string {
    return `${baseUrl}/v1/scan/table?merchant_id=${merchantId}&table_no=${encodeURIComponent(tableNo)}`
}

// ==================== 便捷函数已迁移 ====================
// getAvailableTables 和 getPrivateRooms 等商户端功能
// 已迁移到 table-device-management.ts

// ==================== 兼容性别名 ====================

/** @deprecated 使用 scanTable 替代 */
export const getScanTableInfo = scanTable

/** @deprecated 使用 getTableDetail 替代 */
export const getTable = getTableDetail

// ==================== 商户端功能迁移说明 ====================
/**
 * 商户端桌台和设备管理功能已迁移到新文件：
 * import { 
 *   tableManagementService, 
 *   deviceManagementService, 
 *   displayConfigService 
 * } from './table-device-management'
 */