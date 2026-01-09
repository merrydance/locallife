/**
 * 运营商骑手管理接口重构 (Task 4.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：骑手列表、骑手操作、骑手详情、骑手排行
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 骑手状态枚举 */
export type RiderStatus = 'active' | 'suspended' | 'pending_approval' | 'rejected' | 'offline'

/** 骑手在线状态枚举 */
export type RiderOnlineStatus = 'online' | 'offline' | 'busy' | 'break'

/** 骑手操作类型枚举 */
export type RiderActionType = 'suspend' | 'resume' | 'approve' | 'reject'

// ==================== 骑手管理相关类型 ====================

/** 运营商骑手列表响应 - 基于swagger api.listOperatorRidersResponse */
/** 运营商骑手列表响应 - 对齐 api.listOperatorRidersResponse */
export interface ListOperatorRidersResponse {
    limit?: number                               // 每页数量
    page?: number                                // 页码
    riders?: OperatorRiderItem[]                 // 骑手列表
    total?: number                               // 总数
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

/** 骑手排行查询参数 */
export interface RiderRankingParams extends Record<string, unknown> {
    region_id?: number
    start_date: string
    end_date: string
    rank_by?: 'delivery_count' | 'completion_rate' | 'rating' | 'efficiency_score'
    page?: number
    limit?: number
}

/** 骑手操作请求 */
export interface RiderActionRequest extends Record<string, unknown> {
    reason?: string
    duration?: number // 暂停天数（仅暂停操作需要）
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
    high_value_qualified: boolean                // 是否有高值单资格
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

/** 骑手高值单积分响应 - 对齐 api.riderPremiumScoreResponse */
export interface RiderPremiumScoreResponse {
    rider_id: number                             // 骑手ID
    real_name: string                            // 真实姓名
    premium_score: number                        // 高值单资格积分（可为负）
    can_accept_premium_order: boolean            // 是否可以接高值单（积分≥0）
}

/** 高值单积分日志项 - 对齐 api.premiumScoreLogItem */
export interface PremiumScoreLogItem {
    id: number                                   // 日志ID
    change_type: string                          // 变更类型：normal_order/premium_order/adjustment
    change_type_name: string                     // 变更类型中文名
    change_amount: number                        // 变更量（正数为增加，负数为减少）
    old_score: number                            // 变更前积分
    new_score: number                            // 变更后积分
    related_order_id?: number                    // 关联订单ID
    related_delivery_id?: number                 // 关联配送单ID
    remark?: string                              // 备注
    created_at: string                           // 变更时间
}

/** 骑手高值单积分历史响应 - 对齐 api.listRiderPremiumScoreHistoryResponse */
export interface ListRiderPremiumScoreHistoryResponse {
    current_score: number                        // 当前积分
    logs: PremiumScoreLogItem[]                  // 历史记录
    total: number                                // 总记录数
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
            data: params
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
     * 暂停骑手
     * @param riderId 骑手ID
     * @param actionData 操作数据
     */
    async suspendRider(riderId: number, actionData: RiderActionRequest): Promise<void> {
        return request({
            url: `/v1/operator/riders/${riderId}/suspend`,
            method: 'POST',
            data: actionData
        })
    }

    /**
     * 恢复骑手
     * @param riderId 骑手ID
     * @param actionData 操作数据
     */
    async resumeRider(riderId: number, actionData: RiderActionRequest): Promise<void> {
        return request({
            url: `/v1/operator/riders/${riderId}/resume`,
            method: 'POST',
            data: actionData
        })
    }
}

// ==================== 骑手分析服务类 ====================

/**
 * 骑手分析服务
 * 提供骑手数据分析、绩效评估等功能
 */
export class RiderAnalyticsService {
    /**
     * 计算骑手绩效指标
     * @param rider 骑手详情
     */
    calculateRiderPerformance(rider: OperatorRiderDetailResponse): {
        deliveryEfficiency: number
        serviceQuality: number
        reliability: number
        activity: number
        overallScore: number
        performanceLevel: 'excellent' | 'good' | 'average' | 'poor'
        strengths: string[]
        weaknesses: string[]
    } {
        const stats = rider.stats

        // 配送效率 (0-100)
        const avgTimeScore = Math.max(0, 100 - (stats.avg_delivery_time / 60)) // 假设60分钟为基准
        const completionRateScore = stats.completion_rate
        const deliveryEfficiency = (avgTimeScore + completionRateScore) / 2

        // 服务质量 (0-100)
        const ratingScore = (stats.avg_rating / 5) * 100
        const punctualityScore = stats.punctuality_rate
        const serviceQuality = (ratingScore + punctualityScore) / 2

        // 可靠性 (0-100)
        const deliveryCountScore = Math.min(100, (stats.total_deliveries / 1000) * 100)
        const onlineHoursScore = Math.min(100, (stats.online_hours / 2000) * 100) // 假设2000小时为满分
        const reliability = (deliveryCountScore + onlineHoursScore) / 2

        // 活跃度 (0-100)
        const daysSinceLastActive = rider.last_active_at
            ? Math.floor((Date.now() - new Date(rider.last_active_at).getTime()) / (1000 * 60 * 60 * 24))
            : 999
        const activityScore = Math.max(0, 100 - daysSinceLastActive * 10)
        const activity = Math.min(100, activityScore + (rider.online_status === 'online' ? 20 : 0))

        // 综合评分
        const overallScore = Math.round(
            (deliveryEfficiency * 0.3 + serviceQuality * 0.3 + reliability * 0.25 + activity * 0.15)
        )

        // 绩效等级
        let performanceLevel: 'excellent' | 'good' | 'average' | 'poor' = 'poor'
        if (overallScore >= 80) performanceLevel = 'excellent'
        else if (overallScore >= 65) performanceLevel = 'good'
        else if (overallScore >= 50) performanceLevel = 'average'

        // 优势和劣势分析
        const strengths: string[] = []
        const weaknesses: string[] = []

        if (stats.completion_rate >= 95) strengths.push('完成率优秀')
        else if (stats.completion_rate < 85) weaknesses.push('完成率偏低')

        if (stats.avg_rating >= 4.8) strengths.push('用户评价极高')
        else if (stats.avg_rating < 4.0) weaknesses.push('用户评价较低')

        if (stats.avg_delivery_time <= 1800) strengths.push('配送速度快') // 30分钟内
        else if (stats.avg_delivery_time > 3600) weaknesses.push('配送速度慢') // 超过60分钟

        if (stats.punctuality_rate >= 90) strengths.push('准时率高')
        else if (stats.punctuality_rate < 80) weaknesses.push('准时率偏低')

        if (stats.total_deliveries >= 1000) strengths.push('配送经验丰富')
        else if (stats.total_deliveries < 100) weaknesses.push('配送经验不足')

        if (rider.online_status === 'online') strengths.push('当前在线')
        else if (daysSinceLastActive > 7) weaknesses.push('长时间未活跃')

        return {
            deliveryEfficiency,
            serviceQuality,
            reliability,
            activity,
            overallScore,
            performanceLevel,
            strengths,
            weaknesses
        }
    }

    /**
     * 分析骑手增长趋势
     * @param currentPeriod 当前期间数据
     * @param previousPeriod 上期数据
     */
    analyzeRiderGrowth(
        currentPeriod: { deliveryCount: number; earnings: number; rating: number; onlineHours: number },
        previousPeriod: { deliveryCount: number; earnings: number; rating: number; onlineHours: number }
    ): {
        deliveryGrowth: number
        earningsGrowth: number
        ratingChange: number
        activityGrowth: number
        overallGrowth: number
        growthTrend: 'up' | 'down' | 'stable'
        growthLevel: 'rapid' | 'moderate' | 'slow' | 'decline'
    } {
        const deliveryGrowth = this.calculateGrowthRate(currentPeriod.deliveryCount, previousPeriod.deliveryCount)
        const earningsGrowth = this.calculateGrowthRate(currentPeriod.earnings, previousPeriod.earnings)
        const ratingChange = currentPeriod.rating - previousPeriod.rating
        const activityGrowth = this.calculateGrowthRate(currentPeriod.onlineHours, previousPeriod.onlineHours)

        const overallGrowth = (deliveryGrowth + earningsGrowth + activityGrowth) / 3

        let growthTrend: 'up' | 'down' | 'stable' = 'stable'
        if (overallGrowth > 5) growthTrend = 'up'
        else if (overallGrowth < -5) growthTrend = 'down'

        let growthLevel: 'rapid' | 'moderate' | 'slow' | 'decline' = 'slow'
        if (overallGrowth >= 20) growthLevel = 'rapid'
        else if (overallGrowth >= 10) growthLevel = 'moderate'
        else if (overallGrowth < 0) growthLevel = 'decline'

        return {
            deliveryGrowth,
            earningsGrowth,
            ratingChange,
            activityGrowth,
            overallGrowth,
            growthTrend,
            growthLevel
        }
    }

    /**
     * 骑手分布分析
     * @param riders 骑手列表
     */
    analyzeRiderDistribution(riders: OperatorRiderItem[]): {
        statusDistribution: Map<RiderStatus, number>
        onlineDistribution: Map<RiderOnlineStatus, number>
        vehicleDistribution: Map<string, number>
        performanceDistribution: {
            excellent: number
            good: number
            average: number
            poor: number
        }
        regionDistribution: Array<{
            regionId: number
            regionName: string
            count: number
            avgRating: number
            avgScore: number
        }>
    } {
        const statusDistribution = new Map<RiderStatus, number>()
        const onlineDistribution = new Map<RiderOnlineStatus, number>()
        const vehicleDistribution = new Map<string, number>()
        const regionMap = new Map<number, {
            regionName: string
            count: number
            totalRating: number
            totalScore: number
        }>()

        let excellentCount = 0
        let goodCount = 0
        let averageCount = 0
        let poorCount = 0

        riders.forEach(rider => {
            // 状态分布
            statusDistribution.set(rider.status, (statusDistribution.get(rider.status) || 0) + 1)
            onlineDistribution.set(rider.online_status, (onlineDistribution.get(rider.online_status) || 0) + 1)

            // 区域分布
            const existing = regionMap.get(rider.region_id) || {
                regionName: rider.region_name,
                count: 0,
                totalRating: 0,
                totalScore: 0
            }
            regionMap.set(rider.region_id, {
                regionName: rider.region_name,
                count: existing.count + 1,
                totalRating: existing.totalRating + rider.rating,
                totalScore: existing.totalScore + rider.score
            })

            // 绩效分布（简化计算）
            const performanceScore = (rider.rating / 5) * 50 + (rider.completion_rate / 100) * 50
            if (performanceScore >= 80) excellentCount++
            else if (performanceScore >= 65) goodCount++
            else if (performanceScore >= 50) averageCount++
            else poorCount++
        })

        const regionDistribution = Array.from(regionMap.entries()).map(([regionId, data]) => ({
            regionId,
            regionName: data.regionName,
            count: data.count,
            avgRating: data.totalRating / data.count,
            avgScore: data.totalScore / data.count
        })).sort((a, b) => b.count - a.count)

        return {
            statusDistribution,
            onlineDistribution,
            vehicleDistribution,
            performanceDistribution: {
                excellent: excellentCount,
                good: goodCount,
                average: averageCount,
                poor: poorCount
            },
            regionDistribution
        }
    }

    /**
     * 计算增长率
     * @param current 当前值
     * @param previous 上期值
     */
    private calculateGrowthRate(current: number, previous: number): number {
        if (previous === 0) return current > 0 ? 100 : 0
        return ((current - previous) / previous) * 100
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

// ==================== 导出服务实例 ====================

export const operatorRiderManagementService = new OperatorRiderManagementService()
export const riderAnalyticsService = new RiderAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取骑手管理工作台数据
 * @param regionId 区域ID（可选）
 */
export async function getRiderManagementDashboard(regionId?: number): Promise<{
    riderSummary: {
        total: number
        active: number
        online: number
        suspended: number
        pending: number
    }
    topRiders: OperatorRiderRankingItem[]
    distribution: ReturnType<RiderAnalyticsService['analyzeRiderDistribution']>
    recentRiders: OperatorRiderItem[]
    onlineRiders: OperatorRiderItem[]
}> {
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    const [riderList, riderRanking] = await Promise.all([
        operatorRiderManagementService.getRiderList({
            region_id: regionId,
            limit: 100,
            sort_by: 'created_at',
            sort_order: 'desc'
        }),
        operatorRiderManagementService.getRiderRanking({
            region_id: regionId,
            start_date: startDate,
            end_date: endDate,
            rank_by: 'efficiency_score',
            limit: 10
        })
    ])

    // 统计骑手状态分布
    const riderSummary = {
        total: riderList.total,
        active: riderList.riders.filter(r => r.status === 'active').length,
        online: riderList.riders.filter(r => r.online_status === 'online').length,
        suspended: riderList.riders.filter(r => r.status === 'suspended').length,
        pending: riderList.riders.filter(r => r.status === 'pending_approval').length
    }

    // 分析骑手分布
    const distribution = riderAnalyticsService.analyzeRiderDistribution(riderList.riders)

    // 获取最近注册的骑手
    const recentRiders = riderList.riders.slice(0, 10)

    // 获取在线骑手
    const onlineRiders = riderList.riders.filter(r => r.online_status === 'online').slice(0, 20)

    return {
        riderSummary,
        topRiders: riderRanking.rankings,
        distribution,
        recentRiders,
        onlineRiders
    }
}

/**
 * 获取骑手详细分析报告
 * @param riderId 骑手ID
 */
export async function getRiderAnalysisReport(riderId: number): Promise<{
    riderDetail: OperatorRiderDetailResponse
    performance: ReturnType<RiderAnalyticsService['calculateRiderPerformance']>
    recommendations: string[]
    riskLevel: 'low' | 'medium' | 'high'
    actionSuggestions: string[]
}> {
    const riderDetail = await operatorRiderManagementService.getRiderDetail(riderId)
    const performance = riderAnalyticsService.calculateRiderPerformance(riderDetail)

    // 生成改进建议
    const recommendations = generateRiderRecommendations(riderDetail, performance)

    // 评估风险等级
    const riskLevel = assessRiderRisk(riderDetail, performance)

    // 生成操作建议
    const actionSuggestions = generateRiderActionSuggestions(riderDetail, performance, riskLevel)

    return {
        riderDetail,
        performance,
        recommendations,
        riskLevel,
        actionSuggestions
    }
}

/**
 * 生成骑手改进建议
 * @param rider 骑手详情
 * @param performance 绩效数据
 */
function generateRiderRecommendations(
    rider: OperatorRiderDetailResponse,
    performance: ReturnType<RiderAnalyticsService['calculateRiderPerformance']>
): string[] {
    const recommendations: string[] = []

    // 基于绩效弱点的建议
    performance.weaknesses.forEach(weakness => {
        switch (weakness) {
            case '完成率偏低':
                recommendations.push('建议加强配送技能培训，提高订单完成率')
                break
            case '用户评价较低':
                recommendations.push('建议改善服务态度，提升用户满意度')
                break
            case '配送速度慢':
                recommendations.push('建议优化配送路线，提高配送效率')
                break
            case '准时率偏低':
                recommendations.push('建议加强时间管理，提高准时配送率')
                break
            case '配送经验不足':
                recommendations.push('新骑手建议参加培训课程，积累配送经验')
                break
            case '长时间未活跃':
                recommendations.push('建议联系骑手了解情况，鼓励重新上线')
                break
        }
    })

    // 基于骑手状态的建议
    if (rider.status === 'pending_approval') {
        recommendations.push('骑手正在审核中，请耐心等待审核结果')
    }

    // 基于业务数据的建议
    if (rider.stats.total_deliveries < 50) {
        recommendations.push('新骑手建议安排导师指导，快速适应工作')
    }

    if (rider.stats.online_hours < 100) {
        recommendations.push('建议增加在线时长，提高收入水平')
    }

    if (!rider.documents.health_certificate) {
        recommendations.push('建议尽快上传健康证，确保合规经营')
    }

    return recommendations
}

/**
 * 评估骑手风险等级
 * @param rider 骑手详情
 * @param performance 绩效数据
 */
function assessRiderRisk(
    rider: OperatorRiderDetailResponse,
    performance: ReturnType<RiderAnalyticsService['calculateRiderPerformance']>
): 'low' | 'medium' | 'high' {
    let riskScore = 0

    // 基于绩效评分
    if (performance.overallScore < 30) riskScore += 30
    else if (performance.overallScore < 50) riskScore += 20
    else if (performance.overallScore < 70) riskScore += 10

    // 基于评价分数
    if (rider.stats.avg_rating < 3.0) riskScore += 25
    else if (rider.stats.avg_rating < 3.5) riskScore += 15
    else if (rider.stats.avg_rating < 4.0) riskScore += 5

    // 基于完成率
    if (rider.stats.completion_rate < 70) riskScore += 20
    else if (rider.stats.completion_rate < 80) riskScore += 10

    // 基于活跃度
    const daysSinceLastActive = rider.last_active_at
        ? Math.floor((Date.now() - new Date(rider.last_active_at).getTime()) / (1000 * 60 * 60 * 24))
        : 999

    if (daysSinceLastActive > 7) riskScore += 15
    else if (daysSinceLastActive > 3) riskScore += 5

    // 基于配送数量
    if (rider.stats.total_deliveries === 0) riskScore += 10

    // 基于文档完整性
    const documentCount = Object.values(rider.documents).filter(doc => doc).length
    if (documentCount < 2) riskScore += 10

    if (riskScore >= 50) return 'high'
    if (riskScore >= 25) return 'medium'
    return 'low'
}

/**
 * 生成骑手操作建议
 * @param rider 骑手详情
 * @param performance 绩效数据
 * @param riskLevel 风险等级
 */
function generateRiderActionSuggestions(
    rider: OperatorRiderDetailResponse,
    performance: ReturnType<RiderAnalyticsService['calculateRiderPerformance']>,
    riskLevel: 'low' | 'medium' | 'high'
): string[] {
    const suggestions: string[] = []

    switch (riskLevel) {
        case 'high':
            suggestions.push('建议立即联系骑手了解情况')
            if (rider.stats.avg_rating < 3.0) {
                suggestions.push('考虑暂停骑手服务，要求整改')
            }
            if (rider.stats.completion_rate < 70) {
                suggestions.push('要求骑手提供改善计划')
            }
            break

        case 'medium':
            suggestions.push('建议加强对该骑手的监控')
            suggestions.push('可考虑提供技能培训')
            if (performance.overallScore < 50) {
                suggestions.push('建议安排客户经理跟进')
            }
            break

        case 'low':
            if (performance.performanceLevel === 'excellent') {
                suggestions.push('优秀骑手，可考虑给予奖励激励')
                suggestions.push('可作为标杆骑手进行推广')
            } else {
                suggestions.push('骑手表现正常，保持现有支持力度')
            }
            break
    }

    return suggestions
}

/**
 * 批量操作骑手
 * @param riderIds 骑手ID列表
 * @param action 操作类型
 * @param actionData 操作数据
 */
export async function batchRiderAction(
    riderIds: number[],
    action: RiderActionType,
    actionData: RiderActionRequest
): Promise<{
    success: number[]
    failed: Array<{ id: number; error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number; error: string }> = []

    for (const riderId of riderIds) {
        try {
            switch (action) {
                case 'suspend':
                    await operatorRiderManagementService.suspendRider(riderId, actionData)
                    break
                case 'resume':
                    await operatorRiderManagementService.resumeRider(riderId, actionData)
                    break
                default:
                    throw new Error(`不支持的操作类型: ${action}`)
            }
            success.push(riderId)
        } catch (error) {
            failed.push({
                id: riderId,
                error: error instanceof Error ? error.message : '操作失败'
            })
        }
    }

    return { success, failed }
}

/**
 * 格式化骑手状态显示
 * @param status 骑手状态
 */
export function formatRiderStatus(status: RiderStatus): string {
    const statusMap: Record<RiderStatus, string> = {
        active: '正常',
        suspended: '暂停',
        pending_approval: '待审核',
        rejected: '审核拒绝',
        offline: '离线'
    }
    return statusMap[status] || status
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

/**
 * 验证骑手查询参数
 * @param params 查询参数
 */
export function validateRiderQueryParams(params: RiderQueryParams): { valid: boolean; message?: string } {
    if (params.rating_min && (params.rating_min < 0 || params.rating_min > 5)) {
        return { valid: false, message: '最低评分必须在0-5之间' }
    }

    if (params.rating_max && (params.rating_max < 0 || params.rating_max > 5)) {
        return { valid: false, message: '最高评分必须在0-5之间' }
    }

    if (params.rating_min && params.rating_max && params.rating_min > params.rating_max) {
        return { valid: false, message: '最低评分不能高于最高评分' }
    }

    if (params.score_min && params.score_min < 0) {
        return { valid: false, message: '最低积分不能小于0' }
    }

    if (params.score_max && params.score_max < 0) {
        return { valid: false, message: '最高积分不能小于0' }
    }

    if (params.score_min && params.score_max && params.score_min > params.score_max) {
        return { valid: false, message: '最低积分不能高于最高积分' }
    }

    if (params.start_date && params.end_date) {
        const startDate = new Date(params.start_date)
        const endDate = new Date(params.end_date)

        if (startDate > endDate) {
            return { valid: false, message: '开始日期不能晚于结束日期' }
        }
    }

    if (params.page && params.page < 1) {
        return { valid: false, message: '页码必须大于0' }
    }

    if (params.limit && (params.limit < 1 || params.limit > 100)) {
        return { valid: false, message: '每页数量必须在1-100之间' }
    }

    return { valid: true }
}