/**
 * 运营商骑手管理接口重构 (Task 4.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：骑手列表、骑手详情、骑手排行
 */

import { request } from '../../../utils/request'

// ==================== 数据类型定义 ====================

/** 骑手状态枚举 */
export type RiderStatus = 'approved' | 'active' | 'suspended'

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
    approved: number
    active: number
    suspended: number
    online: number
}

/** 运营商骑手项 - 基于swagger api.operatorRiderItem */
export interface OperatorRiderItem {
    id: number
    user_id: number
    real_name: string
    phone: string
    status: string
    is_online: boolean
    region_id: number
    deposit_amount: number
    total_orders: number
    total_earnings: number
    created_at: string
}

/** 骑手详情响应 - 对齐后端 riderDetailResponse */
export interface OperatorRiderDetailResponse {
    id: number
    user_id: number
    real_name: string
    phone: string
    id_card_no?: string
    region_id: number
    status: string
    is_online: boolean
    deposit_amount: number
    frozen_deposit: number
    total_orders: number
    total_earnings: number
    current_latitude?: number
    current_longitude?: number
    location_updated_at?: string
    credit_score: number
    created_at: string
    updated_at: string
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
    keyword?: string
    start_date?: string
    end_date?: string
    sort_by?: 'created_at'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

function normalizeRiderStatus(status?: RiderStatus): RiderStatus | undefined {
    if (!status) {
        return undefined
    }

    return status
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

    const validStatuses = new Set<RiderStatus>(['approved', 'active', 'suspended'])
    return validStatuses.has(status as RiderStatus) ? (status as RiderStatus) : ''
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
    delivery_count: number                       // 代取次数
    completed_count: number                      // 完成次数
    avg_delivery_time_seconds: number            // 平均代取时长（秒）
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

/** 骑手代取统计响应 - 对齐 api.riderStatsResponse */
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
     * 获取骑手代取统计
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

export const operatorRiderManagementService = new OperatorRiderManagementService()

/**
 * 格式化骑手状态显示
 * @param status 骑手状态
 */
export function formatRiderStatus(status: RiderStatus | string): string {
    const statusMap: Record<RiderStatus, string> = {
        approved: '待激活',
        active: '正常',
        suspended: '暂停'
    }
    return statusMap[status as RiderStatus] || '状态未知'
}

export type RiderStatusTheme = 'success' | 'warning' | 'danger' | 'default'

export function getRiderStatusDisplay(status: RiderStatus | string) {
    const normalizedStatus = status
    const themeMap: Record<RiderStatus, RiderStatusTheme> = {
        approved: 'warning',
        active: 'success',
        suspended: 'danger'
    }

    return {
        normalizedStatus,
        label: formatRiderStatus(normalizedStatus),
        theme: themeMap[normalizedStatus as RiderStatus] || 'default'
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
