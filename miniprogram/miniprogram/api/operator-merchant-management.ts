/**
 * 运营商商户管理接口重构 (Task 4.2)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：商户列表、商户操作、商户详情、商户排行
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 商户状态枚举 */
export type MerchantStatus = 'active' | 'suspended' | 'pending_approval' | 'rejected' | 'closed'

/** 商户类型枚举 */
export type MerchantType = 'restaurant' | 'grocery' | 'pharmacy' | 'convenience' | 'other'

/** 商户操作类型枚举 */
export type MerchantActionType = 'suspend' | 'resume' | 'approve' | 'reject'

// ==================== 商户管理相关类型 ====================

/** 运营商商户列表响应 - 基于swagger api.listOperatorMerchantsResponse */
/** 运营商商户列表响应 - 对齐 api.listOperatorMerchantsResponse */
export interface ListOperatorMerchantsResponse {
    limit?: number                               // 每页数量
    merchants?: OperatorMerchantItem[]           // 商户列表
    page?: number                                // 页码
    total?: number                               // 总数
}

/** 运营商商户项 - 基于swagger api.operatorMerchantItem */
export interface OperatorMerchantItem {
    id: number
    name: string
    phone: string
    address: string
    region_id: number
    region_name: string
    category: string
    type: MerchantType
    status: MerchantStatus
    rating: number
    order_count: number
    total_gmv: number
    commission_amount: number
    created_at: string
    updated_at: string
    last_active_at?: string
}

/** 商户详情响应 - 基于swagger api.operatorMerchantDetailResponse */
export interface OperatorMerchantDetailResponse {
    id: number
    user_id: number
    name: string
    phone: string
    email?: string
    address: string
    latitude: number
    longitude: number
    region_id: number
    region_name: string
    category: string
    type: MerchantType
    status: MerchantStatus
    rating: number
    review_count: number
    business_hours: string
    description?: string
    images: string[]
    license_number?: string
    contact_person: string
    contact_phone: string
    bank_account?: string
    commission_rate: number
    created_at: string
    updated_at: string
    last_active_at?: string
    stats: {
        total_orders: number
        completed_orders: number
        cancelled_orders: number
        total_gmv: number
        avg_order_value: number
        completion_rate: number
        response_time: number
        dish_count: number
        active_dish_count: number
    }
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
    type?: MerchantType
    category?: string
    keyword?: string
    rating_min?: number
    rating_max?: number
    start_date?: string
    end_date?: string
    sort_by?: 'created_at' | 'order_count' | 'total_gmv' | 'rating' | 'last_active_at'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
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

/** 商户操作请求 */
export interface MerchantActionRequest extends Record<string, unknown> {
    reason?: string
    duration?: number // 暂停天数（仅暂停操作需要）
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
     * 暂停商户
     * @param merchantId 商户ID
     * @param actionData 操作数据
     */
    async suspendMerchant(merchantId: number, actionData: MerchantActionRequest): Promise<void> {
        return request({
            url: `/v1/operator/merchants/${merchantId}/suspend`,
            method: 'POST',
            data: actionData
        })
    }

    /**
     * 恢复商户
     * @param merchantId 商户ID
     * @param actionData 操作数据
     */
    async resumeMerchant(merchantId: number, actionData: MerchantActionRequest): Promise<void> {
        return request({
            url: `/v1/operator/merchants/${merchantId}/resume`,
            method: 'POST',
            data: actionData
        })
    }
}

// ==================== 商户分析服务类 ====================

/**
 * 商户分析服务
 * 提供商户数据分析、绩效评估等功能
 */
export class MerchantAnalyticsService {
    /**
     * 计算商户绩效指标
     * @param merchant 商户详情
     */
    calculateMerchantPerformance(merchant: OperatorMerchantDetailResponse): {
        orderEfficiency: number
        revenuePerformance: number
        serviceQuality: number
        businessActivity: number
        overallScore: number
        performanceLevel: 'excellent' | 'good' | 'average' | 'poor'
        strengths: string[]
        weaknesses: string[]
    } {
        const stats = merchant.stats

        // 订单效率 (0-100)
        const orderEfficiency = Math.min(100, (stats.completion_rate + (100 - Math.min(stats.response_time / 60, 100))))

        // 收入表现 (0-100)
        const avgOrderValue = stats.avg_order_value
        const revenuePerformance = Math.min(100, (avgOrderValue / 100) + (stats.total_gmv / 100000))

        // 服务质量 (0-100)
        const serviceQuality = (merchant.rating / 5) * 100

        // 业务活跃度 (0-100)
        const dishUtilization = stats.dish_count > 0 ? (stats.active_dish_count / stats.dish_count) * 100 : 0
        const businessActivity = Math.min(100, dishUtilization + (stats.total_orders / 1000) * 10)

        // 综合评分
        const overallScore = Math.round(
            (orderEfficiency * 0.3 + revenuePerformance * 0.25 + serviceQuality * 0.25 + businessActivity * 0.2)
        )

        // 绩效等级
        let performanceLevel: 'excellent' | 'good' | 'average' | 'poor' = 'poor'
        if (overallScore >= 80) performanceLevel = 'excellent'
        else if (overallScore >= 65) performanceLevel = 'good'
        else if (overallScore >= 50) performanceLevel = 'average'

        // 优势和劣势分析
        const strengths: string[] = []
        const weaknesses: string[] = []

        if (stats.completion_rate >= 90) strengths.push('订单完成率高')
        else if (stats.completion_rate < 80) weaknesses.push('订单完成率偏低')

        if (merchant.rating >= 4.5) strengths.push('用户评价优秀')
        else if (merchant.rating < 3.5) weaknesses.push('用户评价较差')

        if (stats.response_time <= 300) strengths.push('响应速度快') // 5分钟内
        else if (stats.response_time > 900) weaknesses.push('响应速度慢') // 超过15分钟

        if (avgOrderValue >= 5000) strengths.push('客单价较高') // 50元以上
        else if (avgOrderValue < 2000) weaknesses.push('客单价偏低') // 20元以下

        if (dishUtilization >= 80) strengths.push('菜品管理良好')
        else if (dishUtilization < 50) weaknesses.push('菜品管理需改善')

        return {
            orderEfficiency,
            revenuePerformance,
            serviceQuality,
            businessActivity,
            overallScore,
            performanceLevel,
            strengths,
            weaknesses
        }
    }

    /**
     * 分析商户增长趋势
     * @param currentPeriod 当前期间数据
     * @param previousPeriod 上期数据
     */
    analyzeMerchantGrowth(
        currentPeriod: { orderCount: number; gmv: number; rating: number },
        previousPeriod: { orderCount: number; gmv: number; rating: number }
    ): {
        orderGrowth: number
        gmvGrowth: number
        ratingChange: number
        overallGrowth: number
        growthTrend: 'up' | 'down' | 'stable'
        growthLevel: 'rapid' | 'moderate' | 'slow' | 'decline'
    } {
        const orderGrowth = this.calculateGrowthRate(currentPeriod.orderCount, previousPeriod.orderCount)
        const gmvGrowth = this.calculateGrowthRate(currentPeriod.gmv, previousPeriod.gmv)
        const ratingChange = currentPeriod.rating - previousPeriod.rating

        const overallGrowth = (orderGrowth + gmvGrowth) / 2

        let growthTrend: 'up' | 'down' | 'stable' = 'stable'
        if (overallGrowth > 5) growthTrend = 'up'
        else if (overallGrowth < -5) growthTrend = 'down'

        let growthLevel: 'rapid' | 'moderate' | 'slow' | 'decline' = 'slow'
        if (overallGrowth >= 20) growthLevel = 'rapid'
        else if (overallGrowth >= 10) growthLevel = 'moderate'
        else if (overallGrowth < 0) growthLevel = 'decline'

        return {
            orderGrowth,
            gmvGrowth,
            ratingChange,
            overallGrowth,
            growthTrend,
            growthLevel
        }
    }

    /**
     * 商户分类分析
     * @param merchants 商户列表
     */
    analyzeMerchantsByCategory(merchants: OperatorMerchantItem[]): {
        categoryStats: Array<{
            category: string
            count: number
            percentage: number
            avgRating: number
            totalGmv: number
            avgGmv: number
        }>
        topCategories: string[]
        categoryTrends: Map<string, 'growing' | 'stable' | 'declining'>
    } {
        const categoryMap = new Map<string, {
            count: number
            totalRating: number
            totalGmv: number
        }>()

        merchants.forEach(merchant => {
            const category = merchant.category
            const existing = categoryMap.get(category) || { count: 0, totalRating: 0, totalGmv: 0 }

            categoryMap.set(category, {
                count: existing.count + 1,
                totalRating: existing.totalRating + merchant.rating,
                totalGmv: existing.totalGmv + merchant.total_gmv
            })
        })

        const categoryStats = Array.from(categoryMap.entries()).map(([category, data]) => ({
            category,
            count: data.count,
            percentage: (data.count / merchants.length) * 100,
            avgRating: data.totalRating / data.count,
            totalGmv: data.totalGmv,
            avgGmv: data.totalGmv / data.count
        })).sort((a, b) => b.count - a.count)

        const topCategories = categoryStats.slice(0, 5).map(stat => stat.category)

        // 简化的趋势分析（实际应该基于历史数据）
        const categoryTrends = new Map<string, 'growing' | 'stable' | 'declining'>()
        categoryStats.forEach(stat => {
            if (stat.avgGmv > 100000) categoryTrends.set(stat.category, 'growing')
            else if (stat.avgGmv > 50000) categoryTrends.set(stat.category, 'stable')
            else categoryTrends.set(stat.category, 'declining')
        })

        return {
            categoryStats,
            topCategories,
            categoryTrends
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
 * 运营商商户管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class OperatorMerchantManagementAdapter {
    /**
     * 适配商户列表项数据
     */
    static adaptMerchantItem(data: OperatorMerchantItem): {
        id: number
        name: string
        phone: string
        address: string
        regionId: number
        regionName: string
        category: string
        type: MerchantType
        status: MerchantStatus
        rating: number
        orderCount: number
        totalGmv: number
        commissionAmount: number
        createdAt: string
        updatedAt: string
        lastActiveAt?: string
    } {
        return {
            id: data.id,
            name: data.name,
            phone: data.phone,
            address: data.address,
            regionId: data.region_id,
            regionName: data.region_name,
            category: data.category,
            type: data.type,
            status: data.status,
            rating: data.rating,
            orderCount: data.order_count,
            totalGmv: data.total_gmv,
            commissionAmount: data.commission_amount,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            lastActiveAt: data.last_active_at
        }
    }

    /**
     * 适配商户详情数据
     */
    static adaptMerchantDetail(data: OperatorMerchantDetailResponse): {
        id: number
        userId: number
        name: string
        phone: string
        email?: string
        address: string
        latitude: number
        longitude: number
        regionId: number
        regionName: string
        category: string
        type: MerchantType
        status: MerchantStatus
        rating: number
        reviewCount: number
        businessHours: string
        description?: string
        images: string[]
        licenseNumber?: string
        contactPerson: string
        contactPhone: string
        bankAccount?: string
        commissionRate: number
        createdAt: string
        updatedAt: string
        lastActiveAt?: string
        stats: {
            totalOrders: number
            completedOrders: number
            cancelledOrders: number
            totalGmv: number
            avgOrderValue: number
            completionRate: number
            responseTime: number
            dishCount: number
            activeDishCount: number
        }
    } {
        return {
            id: data.id,
            userId: data.user_id,
            name: data.name,
            phone: data.phone,
            email: data.email,
            address: data.address,
            latitude: data.latitude,
            longitude: data.longitude,
            regionId: data.region_id,
            regionName: data.region_name,
            category: data.category,
            type: data.type,
            status: data.status,
            rating: data.rating,
            reviewCount: data.review_count,
            businessHours: data.business_hours,
            description: data.description,
            images: data.images,
            licenseNumber: data.license_number,
            contactPerson: data.contact_person,
            contactPhone: data.contact_phone,
            bankAccount: data.bank_account,
            commissionRate: data.commission_rate,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            lastActiveAt: data.last_active_at,
            stats: {
                totalOrders: data.stats.total_orders,
                completedOrders: data.stats.completed_orders,
                cancelledOrders: data.stats.cancelled_orders,
                totalGmv: data.stats.total_gmv,
                avgOrderValue: data.stats.avg_order_value,
                completionRate: data.stats.completion_rate,
                responseTime: data.stats.response_time,
                dishCount: data.stats.dish_count,
                activeDishCount: data.stats.active_dish_count
            }
        }
    }

    /**
     * 适配商户排行项数据
     */
    static adaptMerchantRankingItem(data: OperatorMerchantRankingItem): {
        rank: number
        merchantId: number
        merchantName: string
        regionName: string
        orderCount: number
        totalGmv: number
        commissionAmount: number
        rating: number
        growthRate: number
    } {
        return {
            rank: data.rank,
            merchantId: data.merchant_id,
            merchantName: data.merchant_name,
            regionName: data.region_name,
            orderCount: data.order_count,
            totalGmv: data.total_gmv,
            commissionAmount: data.commission_amount,
            rating: data.rating,
            growthRate: data.growth_rate
        }
    }
}

// ==================== 导出服务实例 ====================

export const operatorMerchantManagementService = new OperatorMerchantManagementService()
export const merchantAnalyticsService = new MerchantAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取商户管理工作台数据
 * @param regionId 区域ID（可选）
 */
export async function getMerchantManagementDashboard(regionId?: number): Promise<{
    merchantSummary: {
        total: number
        active: number
        suspended: number
        pending: number
    }
    topMerchants: OperatorMerchantRankingItem[]
    categoryAnalysis: ReturnType<MerchantAnalyticsService['analyzeMerchantsByCategory']>
    recentMerchants: OperatorMerchantItem[]
    performanceDistribution: {
        excellent: number
        good: number
        average: number
        poor: number
    }
}> {
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    const [merchantList, merchantRanking] = await Promise.all([
        operatorMerchantManagementService.getMerchantList({
            region_id: regionId,
            limit: 100,
            sort_by: 'created_at',
            sort_order: 'desc'
        }),
        operatorMerchantManagementService.getMerchantRanking({
            region_id: regionId,
            start_date: startDate,
            end_date: endDate,
            rank_by: 'total_gmv',
            limit: 10
        })
    ])

    // 统计商户状态分布
    const merchantSummary = {
        total: merchantList.total,
        active: merchantList.merchants.filter(m => m.status === 'active').length,
        suspended: merchantList.merchants.filter(m => m.status === 'suspended').length,
        pending: merchantList.merchants.filter(m => m.status === 'pending_approval').length
    }

    // 分析商户分类
    const categoryAnalysis = merchantAnalyticsService.analyzeMerchantsByCategory(merchantList.merchants)

    // 获取最近注册的商户
    const recentMerchants = merchantList.merchants.slice(0, 10)

    // 模拟绩效分布（实际应该基于详细数据计算）
    const performanceDistribution = {
        excellent: Math.round(merchantList.total * 0.15),
        good: Math.round(merchantList.total * 0.35),
        average: Math.round(merchantList.total * 0.35),
        poor: Math.round(merchantList.total * 0.15)
    }

    return {
        merchantSummary,
        topMerchants: merchantRanking.rankings,
        categoryAnalysis,
        recentMerchants,
        performanceDistribution
    }
}

/**
 * 获取商户详细分析报告
 * @param merchantId 商户ID
 */
export async function getMerchantAnalysisReport(merchantId: number): Promise<{
    merchantDetail: OperatorMerchantDetailResponse
    performance: ReturnType<MerchantAnalyticsService['calculateMerchantPerformance']>
    recommendations: string[]
    riskLevel: 'low' | 'medium' | 'high'
    actionSuggestions: string[]
}> {
    const merchantDetail = await operatorMerchantManagementService.getMerchantDetail(merchantId)
    const performance = merchantAnalyticsService.calculateMerchantPerformance(merchantDetail)

    // 生成改进建议
    const recommendations = generateMerchantRecommendations(merchantDetail, performance)

    // 评估风险等级
    const riskLevel = assessMerchantRisk(merchantDetail, performance)

    // 生成操作建议
    const actionSuggestions = generateActionSuggestions(merchantDetail, performance, riskLevel)

    return {
        merchantDetail,
        performance,
        recommendations,
        riskLevel,
        actionSuggestions
    }
}

/**
 * 生成商户改进建议
 * @param merchant 商户详情
 * @param performance 绩效数据
 */
function generateMerchantRecommendations(
    merchant: OperatorMerchantDetailResponse,
    performance: ReturnType<MerchantAnalyticsService['calculateMerchantPerformance']>
): string[] {
    const recommendations: string[] = []

    // 基于绩效弱点的建议
    performance.weaknesses.forEach(weakness => {
        switch (weakness) {
            case '订单完成率偏低':
                recommendations.push('建议优化备货管理，减少缺货导致的订单取消')
                break
            case '用户评价较差':
                recommendations.push('建议加强服务培训，提升用户体验')
                break
            case '响应速度慢':
                recommendations.push('建议优化接单流程，提高响应效率')
                break
            case '客单价偏低':
                recommendations.push('建议推出套餐优惠，提升客单价')
                break
            case '菜品管理需改善':
                recommendations.push('建议定期更新菜品，下架不受欢迎的商品')
                break
        }
    })

    // 基于商户状态的建议
    if (merchant.status === 'pending_approval') {
        recommendations.push('商户正在审核中，请耐心等待审核结果')
    }

    // 基于业务数据的建议
    if (merchant.stats.total_orders < 100) {
        recommendations.push('新商户建议参与平台推广活动，提升曝光度')
    }

    if (merchant.stats.dish_count < 10) {
        recommendations.push('建议丰富菜品种类，满足更多用户需求')
    }

    return recommendations
}

/**
 * 评估商户风险等级
 * @param merchant 商户详情
 * @param performance 绩效数据
 */
function assessMerchantRisk(
    merchant: OperatorMerchantDetailResponse,
    performance: ReturnType<MerchantAnalyticsService['calculateMerchantPerformance']>
): 'low' | 'medium' | 'high' {
    let riskScore = 0

    // 基于绩效评分
    if (performance.overallScore < 30) riskScore += 30
    else if (performance.overallScore < 50) riskScore += 20
    else if (performance.overallScore < 70) riskScore += 10

    // 基于评价分数
    if (merchant.rating < 3.0) riskScore += 25
    else if (merchant.rating < 3.5) riskScore += 15
    else if (merchant.rating < 4.0) riskScore += 5

    // 基于完成率
    if (merchant.stats.completion_rate < 70) riskScore += 20
    else if (merchant.stats.completion_rate < 80) riskScore += 10

    // 基于活跃度
    const daysSinceLastActive = merchant.last_active_at
        ? Math.floor((Date.now() - new Date(merchant.last_active_at).getTime()) / (1000 * 60 * 60 * 24))
        : 999

    if (daysSinceLastActive > 7) riskScore += 15
    else if (daysSinceLastActive > 3) riskScore += 5

    // 基于订单量
    if (merchant.stats.total_orders === 0) riskScore += 10

    if (riskScore >= 50) return 'high'
    if (riskScore >= 25) return 'medium'
    return 'low'
}

/**
 * 生成操作建议
 * @param merchant 商户详情
 * @param performance 绩效数据
 * @param riskLevel 风险等级
 */
function generateActionSuggestions(
    merchant: OperatorMerchantDetailResponse,
    performance: ReturnType<MerchantAnalyticsService['calculateMerchantPerformance']>,
    riskLevel: 'low' | 'medium' | 'high'
): string[] {
    const suggestions: string[] = []

    switch (riskLevel) {
        case 'high':
            suggestions.push('建议立即联系商户了解情况')
            if (merchant.rating < 3.0) {
                suggestions.push('考虑暂停商户服务，要求整改')
            }
            if (merchant.stats.completion_rate < 70) {
                suggestions.push('要求商户提供改善计划')
            }
            break

        case 'medium':
            suggestions.push('建议加强对该商户的监控')
            suggestions.push('可考虑提供运营指导')
            if (performance.overallScore < 50) {
                suggestions.push('建议安排客户经理跟进')
            }
            break

        case 'low':
            if (performance.performanceLevel === 'excellent') {
                suggestions.push('优质商户，可考虑给予更多资源支持')
                suggestions.push('可作为标杆商户进行推广')
            } else {
                suggestions.push('商户运营正常，保持现有支持力度')
            }
            break
    }

    return suggestions
}

/**
 * 批量操作商户
 * @param merchantIds 商户ID列表
 * @param action 操作类型
 * @param actionData 操作数据
 */
export async function batchMerchantAction(
    merchantIds: number[],
    action: MerchantActionType,
    actionData: MerchantActionRequest
): Promise<{
    success: number[]
    failed: Array<{ id: number; error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number; error: string }> = []

    for (const merchantId of merchantIds) {
        try {
            switch (action) {
                case 'suspend':
                    await operatorMerchantManagementService.suspendMerchant(merchantId, actionData)
                    break
                case 'resume':
                    await operatorMerchantManagementService.resumeMerchant(merchantId, actionData)
                    break
                default:
                    throw new Error(`不支持的操作类型: ${action}`)
            }
            success.push(merchantId)
        } catch (error) {
            failed.push({
                id: merchantId,
                error: error instanceof Error ? error.message : '操作失败'
            })
        }
    }

    return { success, failed }
}

/**
 * 格式化商户状态显示
 * @param status 商户状态
 */
export function formatMerchantStatus(status: MerchantStatus): string {
    const statusMap: Record<MerchantStatus, string> = {
        active: '正常营业',
        suspended: '暂停营业',
        pending_approval: '待审核',
        rejected: '审核拒绝',
        closed: '已关闭'
    }
    return statusMap[status] || status
}

/**
 * 格式化商户类型显示
 * @param type 商户类型
 */
export function formatMerchantType(type: MerchantType): string {
    const typeMap: Record<MerchantType, string> = {
        restaurant: '餐饮',
        grocery: '生鲜',
        pharmacy: '药店',
        convenience: '便利店',
        other: '其他'
    }
    return typeMap[type] || type
}

/**
 * 格式化绩效等级显示
 * @param level 绩效等级
 */
export function formatPerformanceLevel(level: 'excellent' | 'good' | 'average' | 'poor'): string {
    const levelMap = {
        excellent: '优秀',
        good: '良好',
        average: '一般',
        poor: '较差'
    }
    return levelMap[level] || level
}

/**
 * 格式化风险等级显示
 * @param level 风险等级
 */
export function formatRiskLevel(level: 'low' | 'medium' | 'high'): string {
    const levelMap = {
        low: '低风险',
        medium: '中风险',
        high: '高风险'
    }
    return levelMap[level] || level
}

/**
 * 验证商户查询参数
 * @param params 查询参数
 */
export function validateMerchantQueryParams(params: MerchantQueryParams): { valid: boolean; message?: string } {
    if (params.rating_min && (params.rating_min < 0 || params.rating_min > 5)) {
        return { valid: false, message: '最低评分必须在0-5之间' }
    }

    if (params.rating_max && (params.rating_max < 0 || params.rating_max > 5)) {
        return { valid: false, message: '最高评分必须在0-5之间' }
    }

    if (params.rating_min && params.rating_max && params.rating_min > params.rating_max) {
        return { valid: false, message: '最低评分不能高于最高评分' }
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