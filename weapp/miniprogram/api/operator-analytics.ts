/**
 * 运营商数据统计和分析接口重构 (Task 4.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：区域统计、趋势分析、申诉处理
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 申诉状态枚举 */
export type AppealStatus = 'pending' | 'processing' | 'resolved' | 'rejected' | 'closed'

/** 申诉类型枚举 */
export type AppealType = 'order_issue' | 'payment_issue' | 'service_issue' | 'delivery_issue' | 'other'

/** 申诉优先级枚举 */
export type AppealPriority = 'low' | 'medium' | 'high' | 'urgent'

// ==================== 区域统计相关类型 ====================

/** 区域统计响应 - 基于swagger api.operatorRegionStatsResponse */
export interface OperatorRegionStatsResponse {
    region_id: number
    region_name: string
    date_range: {
        start_date: string
        end_date: string
    }
    merchant_stats: {
        total_merchants: number
        active_merchants: number
        new_merchants: number
        suspended_merchants: number
        avg_rating: number
        top_categories: Array<{
            category: string
            count: number
            percentage: number
        }>
    }
    rider_stats: {
        total_riders: number
        active_riders: number
        online_riders: number
        new_riders: number
        suspended_riders: number
        avg_rating: number
        avg_delivery_time: number
    }
    order_stats: {
        total_orders: number
        completed_orders: number
        cancelled_orders: number
        completion_rate: number
        avg_order_value: number
        total_gmv: number
        peak_hours: Array<{
            hour: number
            order_count: number
        }>
    }
    financial_stats: {
        total_commission: number
        merchant_commission: number
        delivery_commission: number
        platform_fee: number
        settlement_amount: number
    }
    growth_stats: {
        merchant_growth_rate: number
        rider_growth_rate: number
        order_growth_rate: number
        gmv_growth_rate: number
    }
}

/** 趋势分析响应 - 基于swagger api.operatorTrendDailyResponse */
export interface OperatorTrendDailyResponse {
    trends: OperatorDailyTrendItem[]
    summary: {
        total_days: number
        avg_daily_orders: number
        avg_daily_gmv: number
        avg_daily_commission: number
        best_day: {
            date: string
            orders: number
            gmv: number
        }
        worst_day: {
            date: string
            orders: number
            gmv: number
        }
    }
}

/** 日趋势项 - 基于swagger api.operatorDailyTrendItem */
export interface OperatorDailyTrendItem {
    date: string
    merchant_count: number
    active_merchant_count: number
    rider_count: number
    online_rider_count: number
    order_count: number
    completed_order_count: number
    cancelled_order_count: number
    total_gmv: number
    avg_order_value: number
    completion_rate: number
    total_commission: number
    weather?: string
    special_events?: string[]
}

// ==================== 申诉处理相关类型 ====================

/** 运营商申诉列表响应 - 基于swagger api.listOperatorAppealsResponse */
export interface ListOperatorAppealsResponse {
    appeals: OperatorAppealItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
    stats: {
        pending_count: number
        processing_count: number
        resolved_count: number
        avg_resolution_time: number
    }
}

/** 运营商申诉项 - 基于swagger api.operatorAppealItem */
export interface OperatorAppealItem {
    id: number
    appeal_type: AppealType
    status: AppealStatus
    priority: AppealPriority
    title: string
    description: string
    user_id: number
    user_name: string
    user_phone: string
    order_id?: number
    merchant_id?: number
    rider_id?: number
    region_id: number
    created_at: string
    updated_at: string
    resolved_at?: string
    assigned_to?: string
    resolution_time?: number
    satisfaction_rating?: number
}

/** 申诉详情响应 - 基于swagger api.operatorAppealDetailResponse */
export interface OperatorAppealDetailResponse {
    id: number
    appeal_type: AppealType
    status: AppealStatus
    priority: AppealPriority
    title: string
    description: string
    user_id: number
    user_name: string
    user_phone: string
    user_email?: string
    order_id?: number
    merchant_id?: number
    rider_id?: number
    region_id: number
    region_name: string
    evidence_files: string[]
    created_at: string
    updated_at: string
    resolved_at?: string
    assigned_to?: string
    resolution_time?: number
    satisfaction_rating?: number
    resolution_notes?: string
    related_order?: {
        id: number
        order_number: string
        merchant_name: string
        rider_name?: string
        order_amount: number
        status: string
        created_at: string
    }
    timeline: Array<{
        action: string
        operator: string
        timestamp: string
        notes?: string
    }>
}

/** 申诉审核请求 - 基于swagger api.reviewAppealRequest */
/** 申诉审核请求 - 对齐 api.reviewAppealRequest */
export interface ReviewAppealRequest extends Record<string, unknown> {
    compensation_amount?: number                 // 补偿金额（分，最大10万元）
    review_notes: string                         // 审核备注（5-500字符）
    status: 'approved' | 'rejected'              // 审核状态
}

/** 申诉查询参数 */
export interface AppealQueryParams extends Record<string, unknown> {
    region_id?: number
    status?: AppealStatus
    type?: AppealType
    priority?: AppealPriority
    user_id?: number
    order_id?: number
    merchant_id?: number
    rider_id?: number
    assigned_to?: string
    start_date?: string
    end_date?: string
    keyword?: string
    sort_by?: 'created_at' | 'updated_at' | 'priority' | 'resolution_time'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

// ==================== 运营商数据统计服务类 ====================

/**
 * 运营商数据统计服务
 * 提供区域统计、趋势分析等功能
 */
export class OperatorAnalyticsService {
    /**
     * 获取区域统计数据
     * @param regionId 区域ID
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getRegionStats(regionId: number, startDate: string, endDate: string): Promise<OperatorRegionStatsResponse> {
        return request({
            url: `/v1/operator/regions/${regionId}/stats`,
            method: 'GET',
            data: {
                start_date: startDate,
                end_date: endDate
            }
        })
    }

    /**
     * 获取日趋势分析数据
     * @param regionId 区域ID（可选）
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getDailyTrend(regionId?: number, startDate?: string, endDate?: string): Promise<OperatorTrendDailyResponse> {
        return request({
            url: '/v1/operator/trend/daily',
            method: 'GET',
            data: {
                region_id: regionId,
                start_date: startDate,
                end_date: endDate
            }
        })
    }
}

// ==================== 运营商申诉处理服务类 ====================

/**
 * 运营商申诉处理服务
 * 提供申诉列表、详情、审核等功能
 */
export class OperatorAppealService {
    /**
     * 获取申诉列表
     * @param params 查询参数
     */
    async getAppealList(params: AppealQueryParams): Promise<ListOperatorAppealsResponse> {
        return request({
            url: '/v1/operator/appeals',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    async getAppealDetail(appealId: number): Promise<OperatorAppealDetailResponse> {
        return request({
            url: `/v1/operator/appeals/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 审核申诉
     * @param appealId 申诉ID
     * @param reviewData 审核数据
     */
    async reviewAppeal(appealId: number, reviewData: ReviewAppealRequest): Promise<OperatorAppealDetailResponse> {
        return request({
            url: `/v1/operator/appeals/${appealId}/review`,
            method: 'POST',
            data: reviewData
        })
    }
}

// ==================== 数据分析服务类 ====================

/**
 * 数据分析服务
 * 提供深度数据分析和洞察功能
 */
export class DataAnalysisService {
    /**
     * 分析区域绩效趋势
     * @param regionStats 区域统计数据
     * @param previousStats 上期统计数据（可选）
     */
    analyzeRegionPerformanceTrend(
        regionStats: OperatorRegionStatsResponse,
        previousStats?: OperatorRegionStatsResponse
    ): {
        performanceScore: number
        performanceLevel: 'excellent' | 'good' | 'average' | 'poor'
        keyMetrics: {
            merchantHealth: number
            riderHealth: number
            orderHealth: number
            financialHealth: number
        }
        growthAnalysis?: {
            merchantGrowth: number
            riderGrowth: number
            orderGrowth: number
            gmvGrowth: number
            overallGrowth: number
        }
        insights: string[]
        recommendations: string[]
    } {
        const stats = regionStats

        // 计算各维度健康度 (0-100)
        const merchantHealth = this.calculateMerchantHealth(stats.merchant_stats)
        const riderHealth = this.calculateRiderHealth(stats.rider_stats)
        const orderHealth = this.calculateOrderHealth(stats.order_stats)
        const financialHealth = this.calculateFinancialHealth(stats.financial_stats, stats.order_stats.total_gmv)

        // 综合绩效评分
        const performanceScore = Math.round(
            (merchantHealth * 0.25 + riderHealth * 0.25 + orderHealth * 0.3 + financialHealth * 0.2)
        )

        // 绩效等级
        let performanceLevel: 'excellent' | 'good' | 'average' | 'poor' = 'poor'
        if (performanceScore >= 80) performanceLevel = 'excellent'
        else if (performanceScore >= 65) performanceLevel = 'good'
        else if (performanceScore >= 50) performanceLevel = 'average'

        // 增长分析
        let growthAnalysis: any = undefined
        if (previousStats) {
            growthAnalysis = {
                merchantGrowth: stats.growth_stats.merchant_growth_rate,
                riderGrowth: stats.growth_stats.rider_growth_rate,
                orderGrowth: stats.growth_stats.order_growth_rate,
                gmvGrowth: stats.growth_stats.gmv_growth_rate,
                overallGrowth: (
                    stats.growth_stats.merchant_growth_rate +
                    stats.growth_stats.rider_growth_rate +
                    stats.growth_stats.order_growth_rate +
                    stats.growth_stats.gmv_growth_rate
                ) / 4
            }
        }

        // 生成洞察和建议
        const insights = this.generateInsights(stats, performanceScore, growthAnalysis)
        const recommendations = this.generateRecommendations(stats, performanceLevel, growthAnalysis)

        return {
            performanceScore,
            performanceLevel,
            keyMetrics: {
                merchantHealth,
                riderHealth,
                orderHealth,
                financialHealth
            },
            growthAnalysis,
            insights,
            recommendations
        }
    }

    /**
     * 分析申诉处理效率
     * @param appeals 申诉列表
     */
    analyzeAppealEfficiency(appeals: OperatorAppealItem[]): {
        efficiency: {
            avgResolutionTime: number
            resolutionRate: number
            satisfactionRate: number
            workload: number
        }
        distribution: {
            byStatus: Map<AppealStatus, number>
            byType: Map<AppealType, number>
            byPriority: Map<AppealPriority, number>
        }
        trends: {
            dailyVolume: Array<{ date: string; count: number }>
            resolutionTrend: Array<{ date: string; avgTime: number }>
        }
        insights: string[]
        actionItems: string[]
    } {
        const now = Date.now()
        const resolvedAppeals = appeals.filter(a => a.status === 'resolved' && a.resolution_time)
        const totalAppeals = appeals.length

        // 效率指标
        const avgResolutionTime = resolvedAppeals.length > 0
            ? resolvedAppeals.reduce((sum, a) => sum + (a.resolution_time || 0), 0) / resolvedAppeals.length
            : 0

        const resolutionRate = totalAppeals > 0 ? (resolvedAppeals.length / totalAppeals) * 100 : 0

        const satisfactionAppeals = resolvedAppeals.filter(a => a.satisfaction_rating)
        const satisfactionRate = satisfactionAppeals.length > 0
            ? satisfactionAppeals.reduce((sum, a) => sum + (a.satisfaction_rating || 0), 0) / satisfactionAppeals.length
            : 0

        const workload = appeals.filter(a => ['pending', 'processing'].includes(a.status)).length

        // 分布统计
        const statusDistribution = new Map<AppealStatus, number>()
        const typeDistribution = new Map<AppealType, number>()
        const priorityDistribution = new Map<AppealPriority, number>()

        appeals.forEach(appeal => {
            statusDistribution.set(appeal.status, (statusDistribution.get(appeal.status) || 0) + 1)
            typeDistribution.set(appeal.appeal_type, (typeDistribution.get(appeal.appeal_type) || 0) + 1)
            priorityDistribution.set(appeal.priority, (priorityDistribution.get(appeal.priority) || 0) + 1)
        })

        // 趋势分析（简化版）
        const dailyVolumeMap = new Map<string, number>()
        const resolutionTimeMap = new Map<string, { total: number; count: number }>()

        appeals.forEach(appeal => {
            const date = appeal.created_at.split('T')[0]
            dailyVolumeMap.set(date, (dailyVolumeMap.get(date) || 0) + 1)

            if (appeal.status === 'resolved' && appeal.resolved_at && appeal.resolution_time) {
                const resolvedDate = appeal.resolved_at.split('T')[0]
                const existing = resolutionTimeMap.get(resolvedDate) || { total: 0, count: 0 }
                resolutionTimeMap.set(resolvedDate, {
                    total: existing.total + appeal.resolution_time,
                    count: existing.count + 1
                })
            }
        })

        const dailyVolume = Array.from(dailyVolumeMap.entries())
            .map(([date, count]) => ({ date, count }))
            .sort((a, b) => a.date.localeCompare(b.date))

        const resolutionTrend = Array.from(resolutionTimeMap.entries())
            .map(([date, data]) => ({ date, avgTime: data.total / data.count }))
            .sort((a, b) => a.date.localeCompare(b.date))

        // 生成洞察和行动项
        const insights = this.generateAppealInsights({
            avgResolutionTime,
            resolutionRate,
            satisfactionRate,
            workload
        }, { statusDistribution, typeDistribution, priorityDistribution })

        const actionItems = this.generateAppealActionItems({
            avgResolutionTime,
            resolutionRate,
            satisfactionRate,
            workload
        }, { statusDistribution, typeDistribution, priorityDistribution })

        return {
            efficiency: {
                avgResolutionTime,
                resolutionRate,
                satisfactionRate,
                workload
            },
            distribution: {
                byStatus: statusDistribution,
                byType: typeDistribution,
                byPriority: priorityDistribution
            },
            trends: {
                dailyVolume,
                resolutionTrend
            },
            insights,
            actionItems
        }
    }

    /**
     * 计算商户健康度
     */
    private calculateMerchantHealth(merchantStats: OperatorRegionStatsResponse['merchant_stats']): number {
        const activeRate = merchantStats.total_merchants > 0
            ? (merchantStats.active_merchants / merchantStats.total_merchants) * 100
            : 0
        const ratingScore = (merchantStats.avg_rating / 5) * 100
        const suspendedPenalty = merchantStats.total_merchants > 0
            ? (merchantStats.suspended_merchants / merchantStats.total_merchants) * 50
            : 0

        return Math.max(0, Math.min(100, (activeRate + ratingScore) / 2 - suspendedPenalty))
    }

    /**
     * 计算骑手健康度
     */
    private calculateRiderHealth(riderStats: OperatorRegionStatsResponse['rider_stats']): number {
        const activeRate = riderStats.total_riders > 0
            ? (riderStats.active_riders / riderStats.total_riders) * 100
            : 0
        const onlineRate = riderStats.active_riders > 0
            ? (riderStats.online_riders / riderStats.active_riders) * 100
            : 0
        const ratingScore = (riderStats.avg_rating / 5) * 100
        const timeScore = Math.max(0, 100 - (riderStats.avg_delivery_time / 60)) // 假设60分钟为基准

        return Math.max(0, Math.min(100, (activeRate + onlineRate + ratingScore + timeScore) / 4))
    }

    /**
     * 计算订单健康度
     */
    private calculateOrderHealth(orderStats: OperatorRegionStatsResponse['order_stats']): number {
        const completionRate = orderStats.completion_rate
        const volumeScore = Math.min(100, (orderStats.total_orders / 10000) * 100) // 假设10000单为满分
        const valueScore = Math.min(100, (orderStats.avg_order_value / 5000) * 100) // 假设50元为满分

        return Math.max(0, Math.min(100, (completionRate + volumeScore + valueScore) / 3))
    }

    /**
     * 计算财务健康度
     */
    private calculateFinancialHealth(
        financialStats: OperatorRegionStatsResponse['financial_stats'],
        totalGmv: number
    ): number {
        const commissionRate = totalGmv > 0 ? (financialStats.total_commission / totalGmv) * 100 : 0
        const settlementRate = financialStats.total_commission > 0
            ? (financialStats.settlement_amount / financialStats.total_commission) * 100
            : 0

        // 假设合理的佣金率在5-15%之间
        const commissionScore = commissionRate >= 5 && commissionRate <= 15 ? 100 : Math.max(0, 100 - Math.abs(commissionRate - 10) * 10)

        return Math.max(0, Math.min(100, (commissionScore + settlementRate) / 2))
    }

    /**
     * 生成区域洞察
     */
    private generateInsights(
        stats: OperatorRegionStatsResponse,
        performanceScore: number,
        growthAnalysis?: any
    ): string[] {
        const insights: string[] = []

        // 绩效洞察
        if (performanceScore >= 80) {
            insights.push('区域整体表现优秀，各项指标均达到良好水平')
        } else if (performanceScore < 50) {
            insights.push('区域表现需要重点关注，建议制定改善计划')
        }

        // 商户洞察
        if (stats.merchant_stats.active_merchants / stats.merchant_stats.total_merchants < 0.7) {
            insights.push('活跃商户比例偏低，需要加强商户运营')
        }

        // 骑手洞察
        if (stats.rider_stats.online_riders / stats.rider_stats.active_riders < 0.5) {
            insights.push('在线骑手比例偏低，可能影响配送效率')
        }

        // 订单洞察
        if (stats.order_stats.completion_rate < 85) {
            insights.push('订单完成率偏低，需要分析取消原因')
        }

        // 增长洞察
        if (growthAnalysis) {
            if (growthAnalysis.overallGrowth > 10) {
                insights.push('区域呈现良好增长态势，各项指标稳步提升')
            } else if (growthAnalysis.overallGrowth < 0) {
                insights.push('区域增长出现下滑，需要及时采取措施')
            }
        }

        return insights
    }

    /**
     * 生成区域建议
     */
    private generateRecommendations(
        stats: OperatorRegionStatsResponse,
        performanceLevel: string,
        growthAnalysis?: any
    ): string[] {
        const recommendations: string[] = []

        // 基于绩效等级的建议
        if (performanceLevel === 'poor') {
            recommendations.push('建议制定区域改善计划，重点关注薄弱环节')
        }

        // 商户相关建议
        if (stats.merchant_stats.suspended_merchants > stats.merchant_stats.total_merchants * 0.1) {
            recommendations.push('暂停商户数量较多，建议加强商户管理和培训')
        }

        // 骑手相关建议
        if (stats.rider_stats.avg_delivery_time > 3600) {
            recommendations.push('平均配送时间较长，建议优化配送路线和调度')
        }

        // 订单相关建议
        if (stats.order_stats.avg_order_value < 2000) {
            recommendations.push('客单价偏低，建议推广高价值商品和套餐')
        }

        // 增长相关建议
        if (growthAnalysis && growthAnalysis.merchantGrowth < 0) {
            recommendations.push('商户数量下降，建议加强招商和留存工作')
        }

        return recommendations
    }

    /**
     * 生成申诉洞察
     */
    private generateAppealInsights(
        efficiency: any,
        distribution: any
    ): string[] {
        const insights: string[] = []

        if (efficiency.avgResolutionTime > 24 * 60) { // 超过24小时
            insights.push('平均处理时间较长，可能影响用户满意度')
        }

        if (efficiency.resolutionRate < 80) {
            insights.push('申诉解决率偏低，需要提高处理效率')
        }

        if (efficiency.workload > 50) {
            insights.push('待处理申诉数量较多，建议增加处理人员')
        }

        const orderIssueCount = distribution.byType.get('order_issue') || 0
        const totalCount = Array.from(distribution.byType.values()).reduce((sum, count) => sum + count, 0)
        if (orderIssueCount / totalCount > 0.5) {
            insights.push('订单相关申诉占比较高，建议重点关注订单流程')
        }

        return insights
    }

    /**
     * 生成申诉行动项
     */
    private generateAppealActionItems(
        efficiency: any,
        distribution: any
    ): string[] {
        const actionItems: string[] = []

        if (efficiency.avgResolutionTime > 24 * 60) {
            actionItems.push('优化申诉处理流程，缩短平均处理时间')
        }

        if (efficiency.workload > 50) {
            actionItems.push('考虑增加客服人员或优化工作分配')
        }

        const urgentCount = distribution.byPriority.get('urgent') || 0
        if (urgentCount > 10) {
            actionItems.push('优先处理紧急申诉，建立快速响应机制')
        }

        if (efficiency.satisfactionRate < 4.0) {
            actionItems.push('提升申诉处理质量，改善用户满意度')
        }

        return actionItems
    }
}

// ==================== 数据适配器 ====================

/**
 * 运营商数据统计适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class OperatorAnalyticsAdapter {
    /**
     * 适配区域统计响应数据
     */
    static adaptRegionStatsResponse(data: OperatorRegionStatsResponse): {
        regionId: number
        regionName: string
        dateRange: {
            startDate: string
            endDate: string
        }
        merchantStats: {
            totalMerchants: number
            activeMerchants: number
            newMerchants: number
            suspendedMerchants: number
            avgRating: number
            topCategories: Array<{
                category: string
                count: number
                percentage: number
            }>
        }
        riderStats: {
            totalRiders: number
            activeRiders: number
            onlineRiders: number
            newRiders: number
            suspendedRiders: number
            avgRating: number
            avgDeliveryTime: number
        }
        orderStats: {
            totalOrders: number
            completedOrders: number
            cancelledOrders: number
            completionRate: number
            avgOrderValue: number
            totalGmv: number
            peakHours: Array<{
                hour: number
                orderCount: number
            }>
        }
        financialStats: {
            totalCommission: number
            merchantCommission: number
            deliveryCommission: number
            platformFee: number
            settlementAmount: number
        }
        growthStats: {
            merchantGrowthRate: number
            riderGrowthRate: number
            orderGrowthRate: number
            gmvGrowthRate: number
        }
    } {
        return {
            regionId: data.region_id,
            regionName: data.region_name,
            dateRange: {
                startDate: data.date_range.start_date,
                endDate: data.date_range.end_date
            },
            merchantStats: {
                totalMerchants: data.merchant_stats.total_merchants,
                activeMerchants: data.merchant_stats.active_merchants,
                newMerchants: data.merchant_stats.new_merchants,
                suspendedMerchants: data.merchant_stats.suspended_merchants,
                avgRating: data.merchant_stats.avg_rating,
                topCategories: data.merchant_stats.top_categories
            },
            riderStats: {
                totalRiders: data.rider_stats.total_riders,
                activeRiders: data.rider_stats.active_riders,
                onlineRiders: data.rider_stats.online_riders,
                newRiders: data.rider_stats.new_riders,
                suspendedRiders: data.rider_stats.suspended_riders,
                avgRating: data.rider_stats.avg_rating,
                avgDeliveryTime: data.rider_stats.avg_delivery_time
            },
            orderStats: {
                totalOrders: data.order_stats.total_orders,
                completedOrders: data.order_stats.completed_orders,
                cancelledOrders: data.order_stats.cancelled_orders,
                completionRate: data.order_stats.completion_rate,
                avgOrderValue: data.order_stats.avg_order_value,
                totalGmv: data.order_stats.total_gmv,
                peakHours: data.order_stats.peak_hours.map(item => ({
                    hour: item.hour,
                    orderCount: item.order_count
                }))
            },
            financialStats: {
                totalCommission: data.financial_stats.total_commission,
                merchantCommission: data.financial_stats.merchant_commission,
                deliveryCommission: data.financial_stats.delivery_commission,
                platformFee: data.financial_stats.platform_fee,
                settlementAmount: data.financial_stats.settlement_amount
            },
            growthStats: {
                merchantGrowthRate: data.growth_stats.merchant_growth_rate,
                riderGrowthRate: data.growth_stats.rider_growth_rate,
                orderGrowthRate: data.growth_stats.order_growth_rate,
                gmvGrowthRate: data.growth_stats.gmv_growth_rate
            }
        }
    }

    /**
     * 适配申诉项数据
     */
    static adaptAppealItem(data: OperatorAppealItem): {
        id: number
        appealType: AppealType
        status: AppealStatus
        priority: AppealPriority
        title: string
        description: string
        userId: number
        userName: string
        userPhone: string
        orderId?: number
        merchantId?: number
        riderId?: number
        regionId: number
        createdAt: string
        updatedAt: string
        resolvedAt?: string
        assignedTo?: string
        resolutionTime?: number
        satisfactionRating?: number
    } {
        return {
            id: data.id,
            appealType: data.appeal_type,
            status: data.status,
            priority: data.priority,
            title: data.title,
            description: data.description,
            userId: data.user_id,
            userName: data.user_name,
            userPhone: data.user_phone,
            orderId: data.order_id,
            merchantId: data.merchant_id,
            riderId: data.rider_id,
            regionId: data.region_id,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            resolvedAt: data.resolved_at,
            assignedTo: data.assigned_to,
            resolutionTime: data.resolution_time,
            satisfactionRating: data.satisfaction_rating
        }
    }
}

// ==================== 导出服务实例 ====================

export const operatorAnalyticsService = new OperatorAnalyticsService()
export const operatorAppealService = new OperatorAppealService()
export const dataAnalysisService = new DataAnalysisService()

// ==================== 便捷函数 ====================

/**
 * 获取运营商分析工作台数据
 * @param regionId 区域ID（可选）
 */
export async function getOperatorAnalyticsDashboard(regionId?: number): Promise<{
    regionStats: OperatorRegionStatsResponse
    trendAnalysis: OperatorTrendDailyResponse
    performanceAnalysis: ReturnType<DataAnalysisService['analyzeRegionPerformanceTrend']>
    appealSummary: {
        totalAppeals: number
        pendingAppeals: number
        avgResolutionTime: number
        satisfactionRate: number
    }
    recentAppeals: OperatorAppealItem[]
}> {
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    const [regionStats, trendAnalysis, appealList] = await Promise.all([
        operatorAnalyticsService.getRegionStats(regionId || 1, startDate, endDate),
        operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate),
        operatorAppealService.getAppealList({
            region_id: regionId,
            limit: 20,
            sort_by: 'created_at',
            sort_order: 'desc'
        })
    ])

    // 分析区域绩效
    const performanceAnalysis = dataAnalysisService.analyzeRegionPerformanceTrend(regionStats)

    // 申诉摘要
    const appealSummary = {
        totalAppeals: appealList.total,
        pendingAppeals: appealList.stats.pending_count,
        avgResolutionTime: appealList.stats.avg_resolution_time,
        satisfactionRate: 4.2 // 模拟数据，实际应该从API获取
    }

    return {
        regionStats,
        trendAnalysis,
        performanceAnalysis,
        appealSummary,
        recentAppeals: appealList.appeals.slice(0, 10)
    }
}

/**
 * 生成区域分析报告
 * @param regionId 区域ID
 * @param days 分析天数
 */
export async function generateRegionAnalysisReport(regionId: number, days: number = 30): Promise<{
    summary: {
        regionName: string
        analysisPeriod: string
        performanceScore: number
        performanceLevel: string
        keyFindings: string[]
    }
    detailedAnalysis: ReturnType<DataAnalysisService['analyzeRegionPerformanceTrend']>
    trendAnalysis: OperatorTrendDailyResponse
    actionPlan: {
        immediateActions: string[]
        shortTermGoals: string[]
        longTermStrategy: string[]
    }
}> {
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    const [regionStats, trendAnalysis] = await Promise.all([
        operatorAnalyticsService.getRegionStats(regionId, startDate, endDate),
        operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
    ])

    const detailedAnalysis = dataAnalysisService.analyzeRegionPerformanceTrend(regionStats)

    // 生成关键发现
    const keyFindings = [
        ...detailedAnalysis.insights.slice(0, 3),
        `区域综合绩效评分: ${detailedAnalysis.performanceScore}分`,
        `商户健康度: ${detailedAnalysis.keyMetrics.merchantHealth.toFixed(1)}分`,
        `骑手健康度: ${detailedAnalysis.keyMetrics.riderHealth.toFixed(1)}分`
    ]

    // 生成行动计划
    const actionPlan = generateActionPlan(detailedAnalysis)

    return {
        summary: {
            regionName: regionStats.region_name,
            analysisPeriod: `${startDate} 至 ${endDate}`,
            performanceScore: detailedAnalysis.performanceScore,
            performanceLevel: detailedAnalysis.performanceLevel,
            keyFindings
        },
        detailedAnalysis,
        trendAnalysis,
        actionPlan
    }
}

/**
 * 生成行动计划
 * @param analysis 分析结果
 */
function generateActionPlan(analysis: ReturnType<DataAnalysisService['analyzeRegionPerformanceTrend']>): {
    immediateActions: string[]
    shortTermGoals: string[]
    longTermStrategy: string[]
} {
    const immediateActions: string[] = []
    const shortTermGoals: string[] = []
    const longTermStrategy: string[] = []

    // 基于绩效等级制定计划
    if (analysis.performanceLevel === 'poor') {
        immediateActions.push('召开紧急会议，分析问题根因')
        immediateActions.push('暂停表现差的商户和骑手')
        shortTermGoals.push('制定30天改善计划')
        longTermStrategy.push('重新评估区域运营策略')
    } else if (analysis.performanceLevel === 'average') {
        immediateActions.push('识别关键改善点')
        shortTermGoals.push('提升绩效至良好水平')
        longTermStrategy.push('建立持续改善机制')
    }

    // 基于具体指标制定计划
    if (analysis.keyMetrics.merchantHealth < 60) {
        immediateActions.push('加强商户沟通和支持')
        shortTermGoals.push('提升商户活跃度至80%以上')
    }

    if (analysis.keyMetrics.riderHealth < 60) {
        immediateActions.push('优化骑手激励机制')
        shortTermGoals.push('提升骑手在线率和满意度')
    }

    // 基于增长趋势制定计划
    if (analysis.growthAnalysis && analysis.growthAnalysis.overallGrowth < 0) {
        immediateActions.push('分析增长下滑原因')
        shortTermGoals.push('扭转负增长趋势')
        longTermStrategy.push('制定可持续增长策略')
    }

    return {
        immediateActions,
        shortTermGoals,
        longTermStrategy
    }
}

/**
 * 格式化申诉状态显示
 * @param status 申诉状态
 */
export function formatAppealStatus(status: AppealStatus): string {
    const statusMap: Record<AppealStatus, string> = {
        pending: '待处理',
        processing: '处理中',
        resolved: '已解决',
        rejected: '已拒绝',
        closed: '已关闭'
    }
    return statusMap[status] || status
}

/**
 * 格式化申诉类型显示
 * @param type 申诉类型
 */
export function formatAppealType(type: AppealType): string {
    const typeMap: Record<AppealType, string> = {
        order_issue: '订单问题',
        payment_issue: '支付问题',
        service_issue: '服务问题',
        delivery_issue: '配送问题',
        other: '其他'
    }
    return typeMap[type] || type
}

/**
 * 格式化申诉优先级显示
 * @param priority 申诉优先级
 */
export function formatAppealPriority(priority: AppealPriority): string {
    const priorityMap: Record<AppealPriority, string> = {
        low: '低',
        medium: '中',
        high: '高',
        urgent: '紧急'
    }
    return priorityMap[priority] || priority
}

/**
 * 格式化时间显示（分钟）
 * @param minutes 分钟数
 */
export function formatResolutionTime(minutes: number): string {
    if (minutes < 60) {
        return `${minutes}分钟`
    } else if (minutes < 1440) {
        const hours = Math.floor(minutes / 60)
        const remainingMinutes = minutes % 60
        return remainingMinutes > 0 ? `${hours}小时${remainingMinutes}分钟` : `${hours}小时`
    } else {
        const days = Math.floor(minutes / 1440)
        const remainingHours = Math.floor((minutes % 1440) / 60)
        return remainingHours > 0 ? `${days}天${remainingHours}小时` : `${days}天`
    }
}

/**
 * 验证申诉查询参数
 * @param params 查询参数
 */
export function validateAppealQueryParams(params: AppealQueryParams): { valid: boolean; message?: string } {
    if (params.start_date && params.end_date) {
        const startDate = new Date(params.start_date)
        const endDate = new Date(params.end_date)

        if (startDate > endDate) {
            return { valid: false, message: '开始日期不能晚于结束日期' }
        }

        const daysDiff = (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24)
        if (daysDiff > 365) {
            return { valid: false, message: '查询时间范围不能超过365天' }
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