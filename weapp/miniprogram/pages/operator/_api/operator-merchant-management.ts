/**
 * 运营商商户管理接口重构 (Task 4.2)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：商户列表、商户详情、商户排行
 */

import { request } from '../../../utils/request'

// ==================== 数据类型定义 ====================

/** 商户状态枚举 */
export type MerchantStatus = 'approved' | 'suspended' | 'pending' | 'rejected' | 'closed'

// ==================== 商户管理相关类型 ====================

/** 运营商商户列表响应 - 基于swagger api.listOperatorMerchantsResponse */
/** 运营商商户列表响应 - 对齐 api.listOperatorMerchantsResponse */
export interface ListOperatorMerchantsResponse {
    limit?: number                               // 每页数量
    merchants?: OperatorMerchantItem[]           // 商户列表
    page?: number                                // 页码
    total?: number                               // 总数
}

export interface OperatorMerchantSummaryResponse {
    total: number
    pending: number
    approved: number
    rejected: number
    suspended: number
}

/** 运营商商户项 - 对齐后端 api.merchantListItem 结构 */
export interface OperatorMerchantItem {
    id: number
    name: string
    phone: string
    address: string
    status: string           // 商户状态
    is_open: boolean         // 是否营业中
    owner_user_id: number    // 店主用户ID
    region_id: number        // 区域ID
    latitude: number         // 纬度
    longitude: number        // 经度
    created_at: string       // 创建时间
}

/** 商户详情响应 - 对齐后端 merchantDetailResponse */
export interface OperatorMerchantDetailResponse {
    id: number
    name: string
    description?: string
    logo_url?: string
    phone: string
    address: string
    status: string
    is_open: boolean
    owner_user_id: number
    region_id: number
    latitude: number
    longitude: number
    version: number
    created_at: string
    updated_at: string
}

export type MerchantCapabilityStatus = 'unknown' | 'yes' | 'no'

export interface OperatorMerchantCapabilitiesResponse {
    merchant_id: number
    open_kitchen_status: MerchantCapabilityStatus
    dine_in_status: MerchantCapabilityStatus
    system_labels?: string[]
    source: string
    note?: string
    updated_at?: string
}

export interface UpdateOperatorMerchantCapabilitiesRequest extends Record<string, unknown> {
    open_kitchen_status?: MerchantCapabilityStatus
    dine_in_status?: MerchantCapabilityStatus
    note?: string
}

/** 商户排行响应 - 基于swagger api.operatorMerchantRankingResponse */
export interface OperatorMerchantRankingResponse {
    rankings: OperatorMerchantRankingItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
}

/** 商户排行项 - 基于swagger api.operatorMerchantRankingItem */
export interface OperatorMerchantRankingItem {
    rank: number
    merchant_id: number
    merchant_name: string
    region_name: string
    order_count: number
    total_gmv: number
    commission_amount: number
    rating: number
    growth_rate: number
}

/** 商户查询参数 */
export interface MerchantQueryParams extends Record<string, unknown> {
    region_id?: number
    status?: MerchantStatus
    keyword?: string
    start_date?: string
    end_date?: string
    sort_by?: 'created_at'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

export function parseMerchantStatusFilter(status?: string): MerchantStatus | '' {
    const validStatuses = new Set<MerchantStatus>(['approved', 'suspended', 'pending', 'rejected', 'closed'])
    return status && validStatuses.has(status as MerchantStatus) ? (status as MerchantStatus) : ''
}

/** 商户排行查询参数 */
export interface MerchantRankingParams extends Record<string, unknown> {
    region_id?: number
    start_date: string
    end_date: string
    rank_by?: 'order_count' | 'total_gmv' | 'commission_amount' | 'rating'
    page?: number
    limit?: number
}

/** 运营商商户排行行 - 对齐 api.operatorMerchantRankingRow */
export interface OperatorMerchantRankingRow {
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    order_count: number                          // 订单数
    total_sales: number                          // 总销售额（分）
    total_commission: number                     // 总佣金（分）
    avg_order_amount: number                     // 平均订单金额（分）
}

/** 暂停运营商商户请求 - 对齐 api.suspendOperatorMerchantRequest */
export interface SuspendOperatorMerchantRequest extends Record<string, unknown> {
    reason: string                               // 暂停原因（5-500字符，必填）
    duration_hours: number                       // 暂停时长（小时，1-720，必填）
}

/** 恢复运营商商户请求 - 对齐 api.resumeOperatorMerchantRequest */
export interface ResumeOperatorMerchantRequest extends Record<string, unknown> {
    reason: string                               // 恢复原因（5-500字符，必填）
}

/** 商户经营统计热销菜品 - 对齐 api.merchantStatsDish */
export interface MerchantStatsDish {
    dish_name: string
    total_sold: number
    total_revenue: number
}

/** 商户经营统计响应 - 对齐 api.merchantStatsResponse */
export interface MerchantStatsResponse {
    days: number
    total_orders: number
    total_sales: number
    total_commission: number
    avg_daily_sales: number
    total_customers: number
    repeat_customers: number
    repurchase_rate_basis_points: number
    avg_orders_per_user_cents: number
    top_dishes: MerchantStatsDish[]
}

// ==================== 运营商商户管理服务类 ====================

/**
 * 运营商商户管理服务
 * 提供商户列表、详情、操作、排行等功能
 */
export class OperatorMerchantManagementService {
    /**
     * 获取商户列表
     * @param params 查询参数
     */
    async getMerchantList(params: MerchantQueryParams): Promise<ListOperatorMerchantsResponse> {
        return request({
            url: '/v1/operator/merchants',
            method: 'GET',
            data: params
        })
    }

    async getMerchantSummary(regionId?: number): Promise<OperatorMerchantSummaryResponse> {
        return request({
            url: '/v1/operator/merchants/summary',
            method: 'GET',
            data: regionId ? { region_id: regionId } : undefined
        })
    }

    /**
     * 获取商户详情
     * @param merchantId 商户ID
     */
    async getMerchantDetail(merchantId: number): Promise<OperatorMerchantDetailResponse> {
        return request({
            url: `/v1/operator/merchants/${merchantId}`,
            method: 'GET'
        })
    }

    async getMerchantCapabilities(merchantId: number): Promise<OperatorMerchantCapabilitiesResponse> {
        return request({
            url: `/v1/operator/merchants/${merchantId}/capabilities`,
            method: 'GET'
        })
    }

    async updateMerchantCapabilities(
        merchantId: number,
        data: UpdateOperatorMerchantCapabilitiesRequest
    ): Promise<OperatorMerchantCapabilitiesResponse> {
        return request({
            url: `/v1/operator/merchants/${merchantId}/capabilities`,
            method: 'PATCH',
            data
        })
    }

    /**
     * 获取商户排行榜
     * @param params 查询参数
     */
    async getMerchantRanking(params: MerchantRankingParams): Promise<OperatorMerchantRankingResponse> {
        return request({
            url: '/v1/operator/merchants/ranking',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户经营统计
     * @param merchantId 商户ID
     * @param days 统计天数（默认30）
     */
    async getMerchantStats(merchantId: number, days = 30): Promise<MerchantStatsResponse> {
        return request({
            url: `/v1/operator/merchants/${merchantId}/stats`,
            method: 'GET',
            data: { days }
        })
    }
}

export const operatorMerchantManagementService = new OperatorMerchantManagementService()

/**
 * 格式化商户状态显示
 * @param status 商户状态
 */
export function formatMerchantStatus(status: MerchantStatus): string {
    const statusMap: Record<string, string> = {
        active: '正常营业',
        approved: '正常营业',
        suspended: '暂停营业',
        pending: '待入驻',
        rejected: '未通过',
        closed: '已关闭'
    }
    return statusMap[status] || status
}

export type MerchantStatusTheme = 'success' | 'warning' | 'default'

export function getMerchantStatusDisplay(status: MerchantStatus | string) {
    const normalizedStatus = status === 'active' ? 'approved' : status
    const isOpen = normalizedStatus === 'approved'
    const themeMap: Record<string, MerchantStatusTheme> = {
        approved: 'success',
        suspended: 'warning',
        pending: 'default',
        rejected: 'default',
        closed: 'default'
    }

    return {
        normalizedStatus,
        label: formatMerchantStatus(normalizedStatus as MerchantStatus),
        theme: themeMap[normalizedStatus] || 'default',
        isOpen,
        businessStateLabel: isOpen ? '营业中' : '已打烊',
        businessStateTheme: isOpen ? ('success' as const) : ('default' as const)
    }
}
