/**
 * 运营商骑手管理接口重构 (Task 4.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：骑手列表、骑手详情、骑手排行
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 骑手状态枚举 */
export type RiderStatus = 'pending' | 'active' | 'suspended' | 'pending_approval' | 'rejected' | 'offline'

/** 骑手在线状态枚举 */
export type RiderOnlineStatus = 'online' | 'offline' | 'busy' | 'break'

// ==================== 骑手管理相关类型 ====================

/** 运营商骑手列表响应 - 基于swagger api.listOperatorRidersResponse */
/** 运营商骑手列表响应 - 对齐 api.listOperatorRidersResponse */
export interface ListOperatorRidersResponse {
    limit?: number                               // 每页数量
    page?: number                                // 页码
    riders?: OperatorRiderItem[]                 // 骑手列表
    total?: number                               // 总数
}

export interface OperatorRiderSummaryResponse {
    total: number
    pending_approval: number
    active: number
    rejected: number
    suspended: number
    online: number
}

/** 运营商骑手项 - 基于swagger api.operatorRiderItem */
export interface OperatorRiderItem {
    id: number
    name: string
    phone: string
    region_id: number
    region_name: string
    status: RiderStatus
    online_status: RiderOnlineStatus
    rating: number
    score: number
    delivery_count: number
    completion_rate: number
    avg_delivery_time: number
    total_earnings: number
    created_at: string
    updated_at: string
    last_active_at?: string
    last_location?: {
        latitude: number
        longitude: number
        updated_at: string
    }
}

/** 骑手详情响应 - 基于swagger api.operatorRiderDetailResponse */
export interface OperatorRiderDetailResponse {
    id: number
    user_id: number
    name: string
    phone: string
    email?: string
    id_card: string
    region_id: number
    region_name: string
    status: RiderStatus
    online_status: RiderOnlineStatus
    rating: number
    score: number
    vehicle_type: 'bicycle' | 'electric' | 'motorcycle'
    vehicle_number?: string
    emergency_contact: string
    emergency_phone: string
    bank_account?: string
    created_at: string
    updated_at: string
    last_active_at?: string
    last_location?: {
        latitude: number
        longitude: number
        address: string
        updated_at: string
    }
    stats: {
        total_deliveries: number
        completed_deliveries: number
        cancelled_deliveries: number
        completion_rate: number
        avg_delivery_time: number
        avg_rating: number
        total_earnings: number
        total_distance: number
        online_hours: number
        punctuality_rate: number
    }
    documents: {
        id_card_front?: string
        id_card_back?: string
        health_certificate?: string
        vehicle_license?: string
    }
}

/** 骑手排行响应 - 基于swagger api.operatorRiderRankingResponse */
export interface OperatorRiderRankingResponse {
    rankings: OperatorRiderRankingItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
}

/** 骑手排行项 - 基于swagger api.operatorRiderRankingItem */
export interface OperatorRiderRankingItem {
    rank: number
    rider_id: number
    rider_name: string
    region_name: string
    delivery_count: number
    completion_rate: number
    avg_delivery_time: number
    rating: number
    total_earnings: number
    efficiency_score: number
}

/** 骑手查询参数 */
export interface RiderQueryParams extends Record<string, unknown> {
    region_id?: number
    status?: RiderStatus
    online_status?: RiderOnlineStatus
    vehicle_type?: 'bicycle' | 'electric' | 'motorcycle'
    keyword?: string
    rating_min?: number
    rating_max?: number
    score_min?: number
    score_max?: number
    start_date?: string
    end_date?: string
    sort_by?: 'created_at' | 'delivery_count' | 'rating' | 'score' | 'last_active_at'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

function normalizeRiderStatus(status?: RiderStatus): RiderStatus | undefined {
    if (!status) {
        return undefined
    }

    return status === 'pending' ? 'pending_approval' : status
}

function normalizeRiderQueryParams(params: RiderQueryParams): RiderQueryParams {
    return {
        ...params,
        status: normalizeRiderStatus(params.status)
    }
}

export function parseRiderStatusFilter(status?: string): RiderStatus | '' {
    if (!status) {
        return ''
    }

    return normalizeRiderStatus(status as RiderStatus) || ''
}

/** 骑手排行查询参数 */
export interface RiderRankingParams extends Record<string, unknown> {
    region_id?: number
    start_date: string
    end_date: string
    rank_by?: 'delivery_count' | 'completion_rate' | 'rating' | 'efficiency_score'
    page?: number
    limit?: number
}

/** 运营商骑手排行行 - 对齐 api.operatorRiderRankingRow */
export interface OperatorRiderRankingRow {
    rider_id: number                             // 骑手ID
    rider_name: string                           // 骑手姓名
    delivery_count: number                       // 配送次数
    completed_count: number                      // 完成次数
    avg_delivery_time_seconds: number            // 平均配送时长（秒）
    total_earnings: number                       // 总收入（分）
}

/** 暂停运营商骑手请求 - 对齐 api.suspendOperatorRiderRequest */
export interface SuspendOperatorRiderRequest extends Record<string, unknown> {
    reason: string                               // 暂停原因（5-500字符，必填）
    duration_hours: number                       // 暂停时长（小时，1-720，必填）
}

/** 恢复运营商骑手请求 - 对齐 api.resumeOperatorRiderRequest */
export interface ResumeOperatorRiderRequest extends Record<string, unknown> {
    reason: string                               // 恢复原因（5-500字符，必填）
}

/** 骑手配送统计响应 - 对齐 api.riderStatsResponse */
export interface RiderStatsResponse {
    days: number
    total_deliveries: number
    completed_deliveries: number
    completion_rate_basis_points: number
    avg_delivery_seconds: number
    period_earnings: number
    delayed_count: number
}

/** 骑手详情响应 - 对齐 api.riderDetailResponse */
export interface RiderDetailResponse {
    id: number                                   // 骑手ID
    user_id: number                              // 用户ID
    real_name: string                            // 真实姓名
    phone: string                                // 电话
    id_card_no?: string                          // 身份证号
    region_id: number                            // 区域ID
    status: string                               // 状态
    is_online: boolean                           // 是否在线
    credit_score: number                         // 信用分
    deposit_amount: number                       // 押金金额（分）
    frozen_deposit: number                       // 冻结押金（分）
    total_orders: number                         // 总订单数
    total_earnings: number                       // 总收入（分）
    current_latitude?: number                    // 当前纬度
    current_longitude?: number                   // 当前经度
    location_updated_at?: string                 // 位置更新时间
    created_at: string                           // 创建时间
    updated_at: string                           // 更新时间
}

/** 骑手列表项 - 对齐 api.riderListItem */
export interface RiderListItem {
    id: number                                   // 骑手ID
    user_id: number                              // 用户ID
    real_name: string                            // 真实姓名
    phone: string                                // 电话
    region_id: number                            // 区域ID
    status: string                               // 状态
    is_online: boolean                           // 是否在线
    deposit_amount: number                       // 押金金额（分）
    total_orders: number                         // 总订单数
    total_earnings: number                       // 总收入（分）
    created_at: string                           // 创建时间
}

// ==================== 运营商骑手管理服务类 ====================

/**
 * 运营商骑手管理服务
 * 提供骑手列表、详情、操作、排行等功能
 */
export class OperatorRiderManagementService {
    /**
     * 获取骑手列表
     * @param params 查询参数
     */
    async getRiderList(params: RiderQueryParams): Promise<ListOperatorRidersResponse> {
        return request({
            url: '/v1/operator/riders',
            method: 'GET',
            data: normalizeRiderQueryParams(params)
        })
    }

    async getRiderSummary(regionId?: number): Promise<OperatorRiderSummaryResponse> {
        return request({
            url: '/v1/operator/riders/summary',
            method: 'GET',
            data: regionId ? { region_id: regionId } : undefined
        })
    }

    /**
     * 获取骑手详情
     * @param riderId 骑手ID
     */
    async getRiderDetail(riderId: number): Promise<OperatorRiderDetailResponse> {
        return request({
            url: `/v1/operator/riders/${riderId}`,
            method: 'GET'
        })
    }

    /**
     * 获取骑手排行榜
     * @param params 查询参数
     */
    async getRiderRanking(params: RiderRankingParams): Promise<OperatorRiderRankingResponse> {
        return request({
            url: '/v1/operator/riders/ranking',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取骑手配送统计
     * @param riderId 骑手ID
     * @param days 统计天数（默认30）
     */
    async getRiderStats(riderId: number, days = 30): Promise<RiderStatsResponse> {
        return request({
            url: `/v1/operator/riders/${riderId}/stats`,
            method: 'GET',
            data: { days }
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 运营商骑手管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class OperatorRiderManagementAdapter {
    /**
     * 适配骑手列表项数据
     */
    static adaptRiderItem(data: OperatorRiderItem): {
        id: number
        name: string
        phone: string
        regionId: number
        regionName: string
        status: RiderStatus
        onlineStatus: RiderOnlineStatus
        rating: number
        score: number
        deliveryCount: number
        completionRate: number
        avgDeliveryTime: number
        totalEarnings: number
        createdAt: string
        updatedAt: string
        lastActiveAt?: string
        lastLocation?: {
            latitude: number
            longitude: number
            updatedAt: string
        }
    } {
        return {
            id: data.id,
            name: data.name,
            phone: data.phone,
            regionId: data.region_id,
            regionName: data.region_name,
            status: data.status,
            onlineStatus: data.online_status,
            rating: data.rating,
            score: data.score,
            deliveryCount: data.delivery_count,
            completionRate: data.completion_rate,
            avgDeliveryTime: data.avg_delivery_time,
            totalEarnings: data.total_earnings,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            lastActiveAt: data.last_active_at,
            lastLocation: data.last_location ? {
                latitude: data.last_location.latitude,
                longitude: data.last_location.longitude,
                updatedAt: data.last_location.updated_at
            } : undefined
        }
    }

    /**
     * 适配骑手详情数据
     */
    static adaptRiderDetail(data: OperatorRiderDetailResponse): {
        id: number
        userId: number
        name: string
        phone: string
        email?: string
        idCard: string
        regionId: number
        regionName: string
        status: RiderStatus
        onlineStatus: RiderOnlineStatus
        rating: number
        score: number
        vehicleType: 'bicycle' | 'electric' | 'motorcycle'
        vehicleNumber?: string
        emergencyContact: string
        emergencyPhone: string
        bankAccount?: string
        createdAt: string
        updatedAt: string
        lastActiveAt?: string
        lastLocation?: {
            latitude: number
            longitude: number
            address: string
            updatedAt: string
        }
        stats: {
            totalDeliveries: number
            completedDeliveries: number
            cancelledDeliveries: number
            completionRate: number
            avgDeliveryTime: number
            avgRating: number
            totalEarnings: number
            totalDistance: number
            onlineHours: number
            punctualityRate: number
        }
        documents: {
            idCardFront?: string
            idCardBack?: string
            healthCertificate?: string
            vehicleLicense?: string
        }
    } {
        return {
            id: data.id,
            userId: data.user_id,
            name: data.name,
            phone: data.phone,
            email: data.email,
            idCard: data.id_card,
            regionId: data.region_id,
            regionName: data.region_name,
            status: data.status,
            onlineStatus: data.online_status,
            rating: data.rating,
            score: data.score,
            vehicleType: data.vehicle_type,
            vehicleNumber: data.vehicle_number,
            emergencyContact: data.emergency_contact,
            emergencyPhone: data.emergency_phone,
            bankAccount: data.bank_account,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            lastActiveAt: data.last_active_at,
            lastLocation: data.last_location ? {
                latitude: data.last_location.latitude,
                longitude: data.last_location.longitude,
                address: data.last_location.address,
                updatedAt: data.last_location.updated_at
            } : undefined,
            stats: {
                totalDeliveries: data.stats.total_deliveries,
                completedDeliveries: data.stats.completed_deliveries,
                cancelledDeliveries: data.stats.cancelled_deliveries,
                completionRate: data.stats.completion_rate,
                avgDeliveryTime: data.stats.avg_delivery_time,
                avgRating: data.stats.avg_rating,
                totalEarnings: data.stats.total_earnings,
                totalDistance: data.stats.total_distance,
                onlineHours: data.stats.online_hours,
                punctualityRate: data.stats.punctuality_rate
            },
            documents: {
                idCardFront: data.documents.id_card_front,
                idCardBack: data.documents.id_card_back,
                healthCertificate: data.documents.health_certificate,
                vehicleLicense: data.documents.vehicle_license
            }
        }
    }

    /**
     * 适配骑手排行项数据
     */
    static adaptRiderRankingItem(data: OperatorRiderRankingItem): {
        rank: number
        riderId: number
        riderName: string
        regionName: string
        deliveryCount: number
        completionRate: number
        avgDeliveryTime: number
        rating: number
        totalEarnings: number
        efficiencyScore: number
    } {
        return {
            rank: data.rank,
            riderId: data.rider_id,
            riderName: data.rider_name,
            regionName: data.region_name,
            deliveryCount: data.delivery_count,
            completionRate: data.completion_rate,
            avgDeliveryTime: data.avg_delivery_time,
            rating: data.rating,
            totalEarnings: data.total_earnings,
            efficiencyScore: data.efficiency_score
        }
    }
}

export const operatorRiderManagementService = new OperatorRiderManagementService()

/**
 * 格式化骑手状态显示
 * @param status 骑手状态
 */
export function formatRiderStatus(status: RiderStatus): string {
    const statusMap: Record<RiderStatus, string> = {
        active: '正常',
        pending: '待处理',
        suspended: '暂停',
        pending_approval: '待审核',
        rejected: '审核拒绝',
        offline: '离线'
    }
    return statusMap[status] || status
}

export type RiderStatusTheme = 'success' | 'warning' | 'danger' | 'default'

export function getRiderStatusDisplay(status: RiderStatus) {
    const normalizedStatus = status === 'pending' ? 'pending_approval' : status
    const themeMap: Record<RiderStatus, RiderStatusTheme> = {
        active: 'success',
        pending: 'warning',
        pending_approval: 'warning',
        suspended: 'danger',
        rejected: 'danger',
        offline: 'default'
    }

    return {
        normalizedStatus,
        label: formatRiderStatus(normalizedStatus),
        theme: themeMap[normalizedStatus] || 'default'
    }
}

/**
 * 格式化在线状态显示
 * @param status 在线状态
 */
export function formatOnlineStatus(status: RiderOnlineStatus): string {
    const statusMap: Record<RiderOnlineStatus, string> = {
        online: '在线',
        offline: '离线',
        busy: '忙碌',
        break: '休息'
    }
    return statusMap[status] || status
}

/**
 * 格式化车辆类型显示
 * @param type 车辆类型
 */
export function formatVehicleType(type: 'bicycle' | 'electric' | 'motorcycle'): string {
    const typeMap = {
        bicycle: '自行车',
        electric: '电动车',
        motorcycle: '摩托车'
    }
    return typeMap[type] || type
}

/**
 * 格式化时间显示（秒转分钟）
 * @param seconds 秒数
 */
export function formatDeliveryTime(seconds: number): string {
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) {
        return `${minutes}分钟`
    } else {
        const hours = Math.floor(minutes / 60)
        const remainingMinutes = minutes % 60
        return `${hours}小时${remainingMinutes}分钟`
    }
}

/**
 * 格式化距离显示（米转公里）
 * @param meters 米数
 */
export function formatDistance(meters: number): string {
    if (meters < 1000) {
        return `${meters}米`
    } else {
        const km = (meters / 1000).toFixed(1)
        return `${km}公里`
    }
}
