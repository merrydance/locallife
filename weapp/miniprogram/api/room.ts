/**
 * 包间相关API接口
 * 基于swagger.json中的包间浏览接口
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 包间状态枚举 */
export type RoomStatus =
    | 'available'   // 可用
    | 'occupied'    // 占用中
    | 'reserved'    // 已预定
    | 'maintenance' // 维护中
    | 'disabled'    // 已停用

/** 包间类型枚举 */
export type RoomType =
    | 'small'       // 小包间
    | 'medium'      // 中包间
    | 'large'       // 大包间
    | 'vip'         // VIP包间
    | 'luxury'      // 豪华包间

/** 包间响应 */
export interface RoomResponse {
    id: number
    merchant_id: number
    name: string
    room_type: RoomType
    capacity: number
    hourly_rate: number
    minimum_spend: number
    description?: string
    amenities: string[]
    images: RoomImage[]
    status: RoomStatus
    monthly_sales?: number
    tags?: string[]
    created_at: string
    updated_at: string
}

/** 包间图片 */
export interface RoomImage {
    id: number
    image_url: string
    is_primary: boolean
    sort_order: number
}

/** 包间可用性检查参数 */
export interface CheckRoomAvailabilityParams extends Record<string, unknown> {
    date: string // YYYY-MM-DD
}

/** 包间可用性响应 */
/** 包间可用性响应 - 对齐 api.roomAvailabilityResponse */
export interface RoomAvailabilityResponse {
    date?: string                                // 日期
    room_id?: number                             // 包间ID
    room_no?: string                             // 包间编号
    time_slots?: Array<{                         // 时间段列表
        available: boolean                       // 是否可预约
        time: string                             // 时间如 "11:00", "11:30"
    }>
}

/** 包间列表响应 */
export interface ListRoomsResponse {
    rooms: RoomResponse[]
    total: number
}

/** 时间段 - 对齐 api.timeSlot */
export interface TimeSlot {
    time: string                                 // 时间如 "11:00", "11:30"
    available: boolean                           // 是否可预约
}

/** 探索包间项 - 对齐 api.exploreRoomItem */
export interface ExploreRoomItem {
    capacity?: number                            // 容量
    description?: string                         // 描述
    distance?: number                            // 距离（米）
    estimated_delivery_fee?: number              // 预估配送费（分）
    id?: number                                  // 包间ID
    merchant_address?: string                    // 商户地址
    merchant_id?: number                         // 商户ID
    merchant_latitude?: number                   // 商户纬度
    merchant_logo?: string                       // 商户Logo
    merchant_longitude?: number                  // 商户经度
    merchant_name?: string                       // 商户名称
    merchant_phone?: string                      // 商户电话
    minimum_spend?: number                       // 最低消费（分）
    monthly_reservations?: number                // 近30天预订量
    primary_image?: string                       // 主图
    status?: string                              // 状态
    table_no?: string                            // 包间编号
}

/** 探索包间响应 - 对齐 api.exploreRoomsResponse */
export interface ExploreRoomsResponse {
    rooms: ExploreRoomItem[]                     // 包间列表
    total: number                                // 总数
    page_id: number                              // 页码
    page_size: number                            // 每页数量
}

/** 包间详情响应 - 对齐 api.roomDetailResponse */
export interface RoomDetailResponse {
    id: number                                   // 包间ID
    room_no: string                              // 包间编号
    capacity: number                             // 容量
    minimum_spend: number                        // 最低消费（分）
    description?: string                         // 描述
    status: string                               // 状态
    primary_image?: string                       // 主图
    images?: string[]                            // 图片列表
    tags?: TableTagInfo[]                        // 标签列表
    monthly_reservations: number                 // 近30天预订量
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    merchant_logo?: string                       // 商户Logo
    merchant_address: string                     // 商户地址
    merchant_phone: string                       // 商户电话
    merchant_latitude: number                    // 商户纬度
    merchant_longitude: number                   // 商户经度
}

/** 桌台标签信息 - 对齐 api.tableTagInfo */
export interface TableTagInfo {
    id: number                                   // 标签ID
    name: string                                 // 标签名称
    type: string                                 // 标签类型
}

/** 公开包间信息（消费侧） */
export interface PublicRoom {
    id: number                                   // 包间ID
    name: string                                 // 包间名称
    capacity: number                             // 容纳人数
    minimum_spend?: number                       // 最低消费（分）
    description?: string                         // 描述
    image_url?: string                           // 主图URL
    monthly_sales: number                        // 月销量（预订数）
    status: string                               // 状态
    tags: string[]                               // 标签列表
}

/** 公开包间列表响应 */
export interface PublicMerchantRoomsResponse {
    rooms: PublicRoom[]
}

// ==================== 消费侧API接口函数 ====================

/**
 * 获取商户包间列表（消费者端）
 * @param merchantId 商户ID
 */
export async function getPublicMerchantRooms(merchantId: number): Promise<PublicMerchantRoomsResponse> {
    return request({
        url: `/v1/public/merchants/${merchantId}/rooms`,
        method: 'GET'
    })
}

// ==================== API接口函数 ====================

/**
 * 获取商户可用包间列表
 * @param merchantId 商户ID
 */
export async function getMerchantAvailableRooms(merchantId: number): Promise<ListRoomsResponse> {
    return request({
        url: `/v1/merchants/${merchantId}/rooms`,
        method: 'GET'
    })
}

/**
 * 获取商户全部包间列表
 * @param merchantId 商户ID
 */
export async function getMerchantAllRooms(merchantId: number): Promise<ListRoomsResponse> {
    return request({
        url: `/v1/merchants/${merchantId}/rooms/all`,
        method: 'GET'
    })
}

/**
 * 获取包间详情（消费者端）
 * @param roomId 包间ID
 */
export async function getRoomDetail(roomId: number): Promise<RoomDetailResponse> {
    return request({
        url: `/v1/rooms/${roomId}`,
        method: 'GET'
    })
}

/**
 * 检查包间可用性
 * @param roomId 包间ID
 * @param params 检查参数
 */
export async function checkRoomAvailability(roomId: number, params: CheckRoomAvailabilityParams): Promise<RoomAvailabilityResponse> {
    return request({
        url: `/v1/rooms/${roomId}/availability`,
        method: 'GET',
        data: params
    })
}

// ==================== 便捷方法 ====================

/**
 * 根据容量筛选包间
 * @param merchantId 商户ID
 * @param minCapacity 最小容量
 * @param maxCapacity 最大容量
 */
export async function getRoomsByCapacity(merchantId: number, minCapacity: number, maxCapacity?: number): Promise<RoomResponse[]> {
    const response = await getMerchantAvailableRooms(merchantId)
    return response.rooms.filter(room => {
        if (maxCapacity) {
            return room.capacity >= minCapacity && room.capacity <= maxCapacity
        }
        return room.capacity >= minCapacity
    })
}

/**
 * 根据价格筛选包间
 * @param merchantId 商户ID
 * @param maxHourlyRate 最大时租
 * @param maxMinimumSpend 最大最低消费
 */
export async function getRoomsByPrice(merchantId: number, maxHourlyRate?: number, maxMinimumSpend?: number): Promise<RoomResponse[]> {
    const response = await getMerchantAvailableRooms(merchantId)
    return response.rooms.filter(room => {
        let match = true
        if (maxHourlyRate && room.hourly_rate > maxHourlyRate) {
            match = false
        }
        if (maxMinimumSpend && room.minimum_spend > maxMinimumSpend) {
            match = false
        }
        return match
    })
}

/**
 * 根据包间类型筛选
 * @param merchantId 商户ID
 * @param roomType 包间类型
 */
export async function getRoomsByType(merchantId: number, roomType: RoomType): Promise<RoomResponse[]> {
    const response = await getMerchantAvailableRooms(merchantId)
    return response.rooms.filter(room => room.room_type === roomType)
}

/**
 * 获取VIP包间
 * @param merchantId 商户ID
 */
export async function getVIPRooms(merchantId: number): Promise<RoomResponse[]> {
    return getRoomsByType(merchantId, 'vip')
}

/**
 * 获取豪华包间
 * @param merchantId 商户ID
 */
export async function getLuxuryRooms(merchantId: number): Promise<RoomResponse[]> {
    return getRoomsByType(merchantId, 'luxury')
}

/**
 * 检查多个包间的可用性
 * @param roomIds 包间ID列表
 * @param params 检查参数
 */
export async function checkMultipleRoomsAvailability(
    roomIds: number[],
    params: CheckRoomAvailabilityParams
): Promise<RoomAvailabilityResponse[]> {
    const results = await Promise.all(
        roomIds.map(roomId => checkRoomAvailability(roomId, params))
    )
    return results
}

/**
 * 获取指定时间段的可用包间
 * @param merchantId 商户ID
 * @param date 日期
 * @param startTime 开始时间
 * @param endTime 结束时间
 */
export async function getAvailableRoomsForTimeSlot(
    merchantId: number,
    date: string,
    startTime: string,
    endTime: string
): Promise<RoomResponse[]> {
    const response = await getMerchantAvailableRooms(merchantId)
    const availabilityChecks = await Promise.all(
        response.rooms.map(room =>
            checkRoomAvailability(room.id, { date, start_time: startTime, end_time: endTime })
        )
    )

    return response.rooms.filter((_room, index) => {
        const check = availabilityChecks[index]
        // 检查所有时间段是否都可用
        return check.time_slots?.every(slot => slot.available) ?? false
    })
}

/**
 * 计算包间费用
 * @param room 包间信息
 * @param hours 使用小时数
 */
export function calculateRoomCost(room: RoomResponse, hours: number): {
    hourlyFee: number
    minimumSpend: number
    totalCost: number
} {
    const hourlyFee = room.hourly_rate * hours
    const minimumSpend = room.minimum_spend
    const totalCost = Math.max(hourlyFee, minimumSpend)

    return {
        hourlyFee,
        minimumSpend,
        totalCost
    }
}

// ==================== 兼容性别名 ====================

/** @deprecated 使用 getMerchantAvailableRooms 替代 */
export const getRooms = getMerchantAvailableRooms

/** @deprecated 使用 RoomResponse 替代 */
export type RoomDTO = RoomResponse

/** @deprecated 使用 getRoomDetail 替代 */
export const getRoom = getRoomDetail