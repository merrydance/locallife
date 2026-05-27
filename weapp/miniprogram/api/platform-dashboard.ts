/**
 * 平台统计大屏接口重构 (Task 5.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：实时数据、增长数据、排行榜、区域对比
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 订单状态枚举 */
export type OrderStatus = 'pending' | 'confirmed' | 'preparing' | 'ready' | 'delivering' | 'completed' | 'cancelled'

/** 用户角色枚举 */
export type UserRole = 'customer' | 'merchant' | 'rider' | 'operator' | 'admin'

// ==================== 实时数据相关类型 ====================

/** 实时大盘数据响应 - 对齐 api.realtimeDashboardResponse */
export interface RealtimeDashboardData {
    active_merchants_24h: number      // 24小时活跃商户数
    active_users_24h: number          // 24小时活跃用户数
    delivering_orders: number         // 代取中订单数
    gmv_24h: number                   // 24小时GMV(分)
    orders_24h: number                // 24小时订单数
    pending_orders: number            // 待接单订单数
    preparing_orders: number          // 制作中订单数
    ready_orders: number              // 待取餐订单数
}

/** 平台概览响应 - 对齐 api.platformOverviewResponse */
export interface PlatformOverviewResponse {
    active_merchants: number          // 活跃商户数
    active_users: number              // 活跃用户数
    total_commission: number          // 平台总佣金(分)
    total_gmv: number                 // 总GMV(分)
    total_orders: number              // 总订单数
}

// ==================== 增长数据相关类型 ====================

/** 商户增长数据响应 - 基于swagger api.merchantGrowthResponse */
export interface MerchantGrowthResponse {
    growth_data: Array<{
        date: string
        new_merchants: number
        active_merchants: number
        total_merchants: number
        activation_rate: number
    }>
    summary: {
        total_new_merchants: number
        avg_daily_new_merchants: number
        peak_day: {
            date: string
            new_merchants: number
        }
        growth_trend: 'up' | 'down' | 'stable'
        growth_rate: number
    }
    category_growth: Array<{
        category: string
        new_merchants: number
        growth_rate: number
    }>
    regional_growth: Array<{
        region_id: number
        region_name: string
        new_merchants: number
        growth_rate: number
    }>
}

/** 用户增长数据响应 - 基于swagger api.userGrowthResponse */
export interface UserGrowthResponse {
    growth_data: Array<{
        date: string
        new_users: number
        active_users: number
        total_users: number
        retention_rate: number
    }>
    summary: {
        total_new_users: number
        avg_daily_new_users: number
        peak_day: {
            date: string
            new_users: number
        }
        growth_trend: 'up' | 'down' | 'stable'
        growth_rate: number
    }
    acquisition_channels: Array<{
        channel: string
        new_users: number
        percentage: number
        conversion_rate: number
    }>
    user_segments: Array<{
        segment: string
        users: number
        percentage: number
        avg_order_value: number
    }>
}

// ==================== 排行榜相关类型 ====================

/** 商户排行榜行 - 对齐 api.merchantRankingRow */
export interface MerchantRankingRow {
    avg_order_amount: number          // 平均订单金额(分)
    merchant_id: number               // 商户ID
    merchant_name: string             // 商户名称
    order_count: number               // 订单数
    region_id: number                 // 区域ID
    region_name: string               // 区域名称
    total_commission: number          // 总佣金(分)
    total_sales: number               // 总销售额(分)
}

/** 骑手排行榜行 - 对齐 api.riderRankingRow */
export interface RiderRankingRow {
    avg_delivery_time_seconds: number // 平均代取时长(秒)
    completed_count: number           // 完成次数
    delivery_count: number            // 代取次数
    rider_id: number                  // 骑手ID
    rider_name: string                // 骑手姓名
    total_earnings: number            // 总收入(分)
}

// ==================== 区域对比相关类型 ====================

/** 区域对比行 - 对齐 api.regionComparisonRow */
export interface RegionComparisonRow {
    active_users: number              // 活跃用户数
    avg_order_amount: number          // 平均订单金额(分)
    merchant_count: number            // 商户数
    order_count: number               // 订单数
    region_id: number                 // 区域ID
    region_name: string               // 区域名称
    total_commission: number          // 总佣金(分)
    total_gmv: number                 // 总GMV(分)

    // Missing fields
    performance_score: number
    gmv: number
    orders: number
    completion_rate: number
    growth_rate: number
}

// ==================== 查询参数类型 ====================

/** 平台概览查询参数 */
export interface PlatformOverviewParams extends Record<string, unknown> {
    start_date: string
    end_date: string
}

/** 增长数据查询参数 */
export interface GrowthDataParams extends Record<string, unknown> {
    start_date: string
    end_date: string
    region_id?: number
    category?: string
}

/** 排行榜查询参数 */
export interface RankingParams extends Record<string, unknown> {
    start_date: string
    end_date: string
    page?: number
    limit?: number
}

/** 区域对比查询参数 */
export interface RegionComparisonParams extends Record<string, unknown> {
    start_date: string
    end_date: string
}

export interface PlatformProfitSharingDetailsParams extends PlatformOverviewParams {
    page_id?: number
    page_size?: number
}

/** 平台日统计行 - 对齐 api.platformDailyStatRow */
export interface PlatformDailyStatRow {
    date: string                      // 日期
    order_count: number               // 订单数
    total_gmv: number                 // 总GMV(分)
    total_commission: number          // 平台佣金(分)
    takeout_orders: number            // 外卖订单数
    dine_in_orders: number            // 堂食订单数
    active_users: number              // 活跃用户数
    active_merchants: number          // 活跃商户数
}

/** 分账对账状态汇总行 - 对齐 api.platformProfitSharingReconciliationRow */
export interface PlatformProfitSharingReconciliationRow {
    status: string
    total_orders: number
    total_amount: number
    total_merchant_flow: number
    total_rider_flow: number
    total_platform_commission: number
    total_operator_commission: number
    total_merchant_amount: number
    total_rider_amount: number
}

/** 分账对账明细行 - 对齐 api.platformProfitSharingDetailResponse */
export interface PlatformProfitSharingDetailRow {
    id: number
    payment_order_id: number
    merchant_id: number
    operator_id?: number
    rider_id?: number
    order_source: string
    status: string
    total_amount: number
    merchant_flow: number
    rider_flow: number
    platform_commission: number
    operator_commission: number
    merchant_amount: number
    rider_amount: number
    out_order_no: string
    sharing_order_id?: string
    reconciliation_date: string
    created_at: string
    finished_at?: string
    provider: string
    channel: string
    calculation_version: string
    settlement_mode: string
    platform_receiver_amount: number
}

export interface PlatformProfitSharingDetailsResponse {
    items: PlatformProfitSharingDetailRow[]
    total: number
    page_id: number
    page_size: number
    has_more: boolean
}

/** 分账 SLA 汇总 - 对齐 api.platformProfitSharingSlaSummaryResponse */
export interface PlatformProfitSharingSlaResponse {
    total_orders: number
    finished_orders: number
    failed_orders: number
    pending_orders: number
    avg_finish_seconds: number
    p95_finish_seconds: number
}

/** 宝付每日对账汇总行 - 对齐 api.platformBaofuDailyReconciliationRow */
export interface PlatformBaofuDailyReconciliationRow {
    date: string
    provider: string
    channel: string
    paid_amount: number
    payment_fee: number
    provider_payment_fee: number
    merchant_payment_fee: number
    rider_payment_fee: number
    platform_payment_fee_income: number
    platform_net_payment_fee_margin: number
    merchant_amount: number
    rider_amount: number
    platform_commission: number
    operator_commission: number
    withdraw_succeeded_amount: number
    withdraw_processing_amount: number
    unapplied_fact_count: number
    unknown_command_count: number
    fee_ledger_mismatch_count: number
}

/** 分类统计行 - 对齐 api.categoryStatRow */
export interface CategoryStatRow {
    category_name: string             // 分类名称
    order_count: number               // 订单数
    total_sales: number               // 总销售额(分)
    merchant_count: number            // 商户数
}

/** 增长统计行 - 对齐 api.growthStatRow */
export interface GrowthStatRow {
    date: string                      // 日期
    count: number                     // 数量
}

/** 小时分布行 - 对齐 api.hourlyDistributionRow */
export interface HourlyDistributionRow {
    hour: number                      // 小时(0-23)
    order_count: number               // 订单数
    total_gmv: number                 // 总GMV(分)

    // Fields used in Adapter
    orders: number
    gmv: number
    completion_rate: number
}

/** 区域日趋势行 - 对齐 api.regionDailyTrendRow */
export interface RegionDailyTrendRow {
    date: string                      // 日期
    order_count: number               // 订单数
    total_gmv: number                 // 总GMV(分)
    total_commission: number          // 总佣金(分)
    active_users: number              // 活跃用户数
    active_merchants: number          // 活跃商户数
}

// ==================== 平台统计大屏服务类 ====================

/**
 * 平台统计大屏服务
 * 提供实时数据、概览、增长分析等功能
 */
export class PlatformDashboardService {
    /**
     * 获取实时大盘数据
     */
    async getRealtimeDashboard(): Promise<RealtimeDashboardData> {
        return request({
            url: '/v1/platform/stats/realtime',
            method: 'GET',
            timeout: 45000,
            retry: 1
        })
    }

    /**
     * 获取平台概览数据
     * @param params 查询参数
     */
    async getPlatformOverview(params: PlatformOverviewParams): Promise<PlatformOverviewResponse> {
        return request({
            url: '/v1/platform/stats/overview',
            method: 'GET',
            data: params,
            timeout: 45000,
            retry: 1
        })
    }

    /**
     * 获取商户增长数据
     * @param params 查询参数
     */
    async getMerchantGrowth(params: GrowthDataParams): Promise<MerchantGrowthResponse> {
        return request({
            url: '/v1/platform/stats/growth/merchants',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取用户增长数据
     * @param params 查询参数
     */
    async getUserGrowth(params: GrowthDataParams): Promise<UserGrowthResponse> {
        return request({
            url: '/v1/platform/stats/growth/users',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户排行榜
     * @param params 查询参数
     */
    async getMerchantRanking(params: RankingParams): Promise<MerchantRankingRow[]> {
        return request({
            url: '/v1/platform/stats/merchants/ranking',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取骑手排行榜
     * @param params 查询参数
     */
    async getRiderRanking(params: RankingParams): Promise<RiderRankingRow[]> {
        return request({
            url: '/v1/platform/stats/riders/ranking',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取区域对比数据
     * @param params 查询参数
     */
    async getRegionComparison(params: RegionComparisonParams): Promise<RegionComparisonRow[]> {
        return request({
            url: '/v1/platform/stats/regions/compare',
            method: 'GET',
            data: params
        })
    }

    async getProfitSharingReconciliation(params: PlatformOverviewParams): Promise<PlatformProfitSharingReconciliationRow[]> {
        return request({
            url: '/v1/platform/stats/profit-sharing/reconciliation',
            method: 'GET',
            data: params
        })
    }

    async getProfitSharingDetails(params: PlatformProfitSharingDetailsParams): Promise<PlatformProfitSharingDetailsResponse> {
        return request({
            url: '/v1/platform/stats/profit-sharing/details',
            method: 'GET',
            data: params
        })
    }

    async getProfitSharingSla(params: PlatformOverviewParams): Promise<PlatformProfitSharingSlaResponse> {
        return request({
            url: '/v1/platform/stats/profit-sharing/sla',
            method: 'GET',
            data: params
        })
    }

    async getBaofuDailyReconciliation(params: PlatformOverviewParams): Promise<PlatformBaofuDailyReconciliationRow[]> {
        return request({
            url: '/v1/platform/stats/baofu/reconciliation/daily',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 数据分析服务类 ====================

/**
 * 平台数据分析服务
 * 提供深度数据分析和洞察功能
 */
export class PlatformAnalyticsService {
    /**
     * 分析平台健康度
     * @param overview 平台概览数据
     * @param realtime 实时数据
     */
    analyzePlatformHealth(
        overview: PlatformOverviewResponse,
        realtime: RealtimeDashboardData
    ): {
        healthScore: number
        healthLevel: 'excellent' | 'good' | 'warning' | 'critical'
        keyMetrics: {
            orderHealth: number
            userHealth: number
            merchantHealth: number
            financialHealth: number
        }
        alerts: Array<{
            level: 'info' | 'warning' | 'error'
            message: string
            metric: string
            value: number
            threshold: number
        }>
        insights: string[]
    } {
        const orderHealth = this.calculateOrderHealth(realtime)
        const userHealth = this.calculateUserHealth(realtime)
        const merchantHealth = this.calculateMerchantHealth(overview, realtime)
        const financialHealth = this.calculateFinancialHealth(overview)

        const healthScore = Math.round(
            (orderHealth * 0.35 + userHealth * 0.2 + merchantHealth * 0.25 + financialHealth * 0.2)
        )

        let healthLevel: 'excellent' | 'good' | 'warning' | 'critical' = 'critical'
        if (healthScore >= 85) healthLevel = 'excellent'
        else if (healthScore >= 70) healthLevel = 'good'
        else if (healthScore >= 50) healthLevel = 'warning'

        const alerts = this.generateHealthAlerts(overview, realtime)
        const insights = this.generatePlatformInsights(overview, realtime, healthScore)

        return {
            healthScore,
            healthLevel,
            keyMetrics: {
                orderHealth,
                userHealth,
                merchantHealth,
                financialHealth
            },
            alerts,
            insights
        }
    }

    /**
     * 分析增长趋势
     * @param merchantGrowth 商户增长数据
     * @param userGrowth 用户增长数据
     */
    analyzeGrowthTrends(
        merchantGrowth: MerchantGrowthResponse,
        userGrowth: UserGrowthResponse
    ): {
        overallTrend: 'accelerating' | 'growing' | 'stable' | 'declining'
        growthScore: number
        predictions: {
            nextMonthUsers: number
            nextMonthMerchants: number
            confidence: number
        }
        recommendations: string[]
        riskFactors: string[]
    } {
        // 计算增长评分
        const userGrowthScore = this.calculateGrowthScore(userGrowth.summary.growth_rate)
        const merchantGrowthScore = this.calculateGrowthScore(merchantGrowth.summary.growth_rate)
        const growthScore = (userGrowthScore + merchantGrowthScore) / 2

        // 判断整体趋势
        let overallTrend: 'accelerating' | 'growing' | 'stable' | 'declining' = 'stable'
        if (growthScore >= 80) overallTrend = 'accelerating'
        else if (growthScore >= 60) overallTrend = 'growing'
        else if (growthScore < 40) overallTrend = 'declining'

        // 简化的预测模型
        const predictions = this.generateGrowthPredictions(merchantGrowth, userGrowth)

        // 生成建议和风险因素
        const recommendations = this.generateGrowthRecommendations(merchantGrowth, userGrowth, overallTrend)
        const riskFactors = this.identifyGrowthRisks(merchantGrowth, userGrowth)

        return {
            overallTrend,
            growthScore,
            predictions,
            recommendations,
            riskFactors
        }
    }

    /**
     * 分析区域绩效
     * @param regions 区域对比数据
     */
    analyzeRegionalPerformance(regions: RegionComparisonRow[]): {
        topPerformers: RegionComparisonRow[]
        underPerformers: RegionComparisonRow[]
        averageMetrics: {
            avgGmv: number
            avgOrders: number
            avgCompletionRate: number
            avgGrowthRate: number
        }
        insights: string[]
        balanceRecommendations: string[]
    } {
        if (regions.length === 0) {
            return {
                topPerformers: [],
                underPerformers: [],
                averageMetrics: { avgGmv: 0, avgOrders: 0, avgCompletionRate: 0, avgGrowthRate: 0 },
                insights: [],
                balanceRecommendations: []
            }
        }

        // 按绩效评分排序
        const sortedRegions = [...regions].sort((a, b) => b.performance_score - a.performance_score)

        // 识别表现优异和落后的区域
        const topCount = Math.max(1, Math.floor(regions.length * 0.2))
        const bottomCount = Math.max(1, Math.floor(regions.length * 0.2))

        const topPerformers = sortedRegions.slice(0, topCount)
        const underPerformers = sortedRegions.slice(-bottomCount)

        // 计算平均指标
        const totalGmv = regions.reduce((sum, r) => sum + r.gmv, 0)
        const totalOrders = regions.reduce((sum, r) => sum + r.orders, 0)
        const totalCompletionRate = regions.reduce((sum, r) => sum + r.completion_rate, 0)
        const totalGrowthRate = regions.reduce((sum, r) => sum + r.growth_rate, 0)

        const averageMetrics = {
            avgGmv: totalGmv / regions.length,
            avgOrders: totalOrders / regions.length,
            avgCompletionRate: totalCompletionRate / regions.length,
            avgGrowthRate: totalGrowthRate / regions.length
        }

        // 生成洞察
        const insights = this.generateRegionalInsights(topPerformers, underPerformers, averageMetrics)

        // 生成平衡建议
        const balanceRecommendations = this.generateBalanceRecommendations(topPerformers, underPerformers)

        return {
            topPerformers,
            underPerformers,
            averageMetrics,
            insights,
            balanceRecommendations
        }
    }

    /**
     * 计算订单健康度
     */
    private calculateOrderHealth(realtime: RealtimeDashboardData): number {
        const orders24h = Math.max(realtime.orders_24h, 1)
        const activeOrders = realtime.pending_orders
            + realtime.preparing_orders
            + realtime.ready_orders
            + realtime.delivering_orders
        const activePressure = activeOrders / orders24h
        const pendingPressure = realtime.pending_orders / orders24h

        return Math.max(0, Math.round(100 - activePressure * 60 - pendingPressure * 30))
    }

    /**
     * 计算用户健康度
     */
    private calculateUserHealth(realtime: RealtimeDashboardData): number {
        return Math.min(100, Math.round((realtime.active_users_24h / 1000) * 100))
    }

    /**
     * 计算商户健康度
     */
    private calculateMerchantHealth(
        overview: PlatformOverviewResponse,
        realtime: RealtimeDashboardData
    ): number {
        if (overview.active_merchants <= 0) {
            return realtime.active_merchants_24h > 0 ? 100 : 0
        }

        return Math.min(100, Math.round((realtime.active_merchants_24h / overview.active_merchants) * 100))
    }

    /**
     * 计算财务健康度
     */
    private calculateFinancialHealth(overview: PlatformOverviewResponse): number {
        const gmvScore = Math.min(100, (overview.total_gmv / 10000000) * 100)
        const commissionScore = Math.min(100, (overview.total_commission / 1000000) * 100)

        return Math.round((gmvScore + commissionScore) / 2)
    }

    /**
     * 生成健康告警
     */
    private generateHealthAlerts(
        overview: PlatformOverviewResponse,
        realtime: RealtimeDashboardData
    ): Array<{
        level: 'info' | 'warning' | 'error'
        message: string
        metric: string
        value: number
        threshold: number
    }> {
        const alerts: Array<{
            level: 'info' | 'warning' | 'error'
            message: string
            metric: string
            value: number
            threshold: number
        }> = []

        if (realtime.pending_orders > 0) {
            alerts.push({
                level: 'warning',
                message: '存在待接单订单',
                metric: 'pending_orders',
                value: realtime.pending_orders,
                threshold: 0
            })
        }

        const activeOrders = realtime.pending_orders
            + realtime.preparing_orders
            + realtime.ready_orders
            + realtime.delivering_orders
        const activeOrderThreshold = Math.max(10, Math.round(realtime.orders_24h * 0.5))
        if (activeOrders > activeOrderThreshold) {
            alerts.push({
                level: 'warning',
                message: '履约中订单占比较高',
                metric: 'active_orders',
                value: activeOrders,
                threshold: activeOrderThreshold
            })
        }

        if (overview.total_orders > 0 && overview.total_gmv === 0) {
            alerts.push({
                level: 'error',
                message: '订单金额统计异常',
                metric: 'total_gmv',
                value: overview.total_gmv,
                threshold: 1
            })
        }

        return alerts
    }

    /**
     * 生成平台洞察
     */
    private generatePlatformInsights(
        overview: PlatformOverviewResponse,
        realtime: RealtimeDashboardData,
        healthScore: number
    ): string[] {
        const insights: string[] = []

        if (healthScore >= 85) {
            insights.push('平台核心运营指标稳定')
        } else if (healthScore < 50) {
            insights.push('平台运营状况需要重点关注，建议制定改善计划')
        }

        if (realtime.orders_24h > 0) {
            insights.push(`近24小时产生${realtime.orders_24h}单`)
        }

        if (realtime.active_merchants_24h > 0) {
            insights.push(`近24小时有${realtime.active_merchants_24h}家商户活跃`)
        }

        return insights
    }

    /**
     * 计算增长评分
     */
    private calculateGrowthScore(growthRate: number): number {
        // 将增长率转换为0-100的评分
        if (growthRate >= 20) return 100
        if (growthRate >= 10) return 80
        if (growthRate >= 5) return 60
        if (growthRate >= 0) return 40
        return Math.max(0, 40 + growthRate * 2)
    }

    /**
     * 生成增长预测
     */
    private generateGrowthPredictions(
        merchantGrowth: MerchantGrowthResponse,
        userGrowth: UserGrowthResponse
    ): {
        nextMonthUsers: number
        nextMonthMerchants: number
        confidence: number
    } {
        // 简化的线性预测模型
        const avgDailyUsers = userGrowth.summary.avg_daily_new_users
        const avgDailyMerchants = merchantGrowth.summary.avg_daily_new_merchants

        const nextMonthUsers = Math.round(avgDailyUsers * 30)
        const nextMonthMerchants = Math.round(avgDailyMerchants * 30)

        // 基于增长趋势计算置信度
        const userTrendStability = userGrowth.summary.growth_trend === 'stable' ? 0.8 : 0.6
        const merchantTrendStability = merchantGrowth.summary.growth_trend === 'stable' ? 0.8 : 0.6
        const confidence = (userTrendStability + merchantTrendStability) / 2

        return {
            nextMonthUsers,
            nextMonthMerchants,
            confidence
        }
    }

    /**
     * 生成增长建议
     */
    private generateGrowthRecommendations(
        merchantGrowth: MerchantGrowthResponse,
        userGrowth: UserGrowthResponse,
        overallTrend: string
    ): string[] {
        const recommendations: string[] = []

        if (overallTrend === 'declining') {
            recommendations.push('增长趋势下滑，建议分析原因并制定挽回策略')
        }

        if (userGrowth.summary.growth_rate < 5) {
            recommendations.push('用户增长缓慢，建议加强市场推广和用户获取')
        }

        if (merchantGrowth.summary.growth_rate < 5) {
            recommendations.push('商户增长缓慢，建议优化招商策略和激励政策')
        }

        if (overallTrend === 'accelerating') {
            recommendations.push('增长势头良好，建议保持现有策略并适度扩大投入')
        }

        return recommendations
    }

    /**
     * 识别增长风险
     */
    private identifyGrowthRisks(
        merchantGrowth: MerchantGrowthResponse,
        userGrowth: UserGrowthResponse
    ): string[] {
        const risks: string[] = []

        if (userGrowth.summary.growth_rate < 0) {
            risks.push('用户增长为负，存在用户流失风险')
        }

        if (merchantGrowth.summary.growth_rate < 0) {
            risks.push('商户增长为负，可能影响平台供给能力')
        }

        // 检查增长数据的波动性
        const userGrowthData = userGrowth.growth_data
        if (userGrowthData.length > 7) {
            const recentWeek = userGrowthData.slice(-7)
            const weeklyVariance = this.calculateVariance(recentWeek.map((d) => d.new_users))
            if (weeklyVariance > 1000) {
                risks.push('用户增长波动较大，增长稳定性存在风险')
            }
        }

        return risks
    }

    /**
     * 生成区域洞察
     */
    private generateRegionalInsights(
        topPerformers: RegionComparisonRow[],
        underPerformers: RegionComparisonRow[],
        averageMetrics: { avgGrowthRate: number }
    ): string[] {
        const insights: string[] = []

        if (topPerformers.length > 0) {
            const bestRegion = topPerformers[0]
            insights.push(`${bestRegion.region_name}表现最佳，绩效评分${bestRegion.performance_score.toFixed(1)}`)
        }

        if (underPerformers.length > 0) {
            const worstRegion = underPerformers[underPerformers.length - 1]
            insights.push(`${worstRegion.region_name}需要重点关注，绩效评分${worstRegion.performance_score.toFixed(1)}`)
        }

        if (averageMetrics.avgGrowthRate > 10) {
            insights.push('各区域整体增长良好，平台扩张势头强劲')
        } else if (averageMetrics.avgGrowthRate < 0) {
            insights.push('多个区域增长放缓，需要制定区域振兴计划')
        }

        return insights
    }

    /**
     * 生成平衡建议
     */
    private generateBalanceRecommendations(
        topPerformers: RegionComparisonRow[],
        underPerformers: RegionComparisonRow[]
    ): string[] {
        const recommendations: string[] = []

        if (topPerformers.length > 0 && underPerformers.length > 0) {
            recommendations.push('建议将优秀区域的成功经验推广到表现较差的区域')
            recommendations.push('考虑将部分资源从饱和区域转移到潜力区域')
        }

        if (underPerformers.length > 0) {
            recommendations.push('为表现较差的区域制定专项扶持计划')
            recommendations.push('加强对落后区域的运营指导和资源投入')
        }

        return recommendations
    }



    /**
     * 计算方差
     */
    private calculateVariance(values: number[]): number {
        if (values.length === 0) return 0

        const mean = values.reduce((sum, val) => sum + val, 0) / values.length
        const squaredDiffs = values.map((val) => Math.pow(val - mean, 2))
        return squaredDiffs.reduce((sum, val) => sum + val, 0) / values.length
    }
}

// ==================== 数据适配器 ====================

/**
 * 平台统计大屏数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class PlatformDashboardAdapter {
    /**
     * 适配实时大盘数据
     */
    static adaptRealtimeDashboard(data: RealtimeDashboardData): {
        recent24h: {
            totalOrders: number
            totalGmv: number
            activeUsers: number
            activeMerchants: number
            activeOrders: number
        }
        orderStates: {
            pending: number
            preparing: number
            ready: number
            delivering: number
        }
    } {
        const activeOrders = data.pending_orders
            + data.preparing_orders
            + data.ready_orders
            + data.delivering_orders

        return {
            recent24h: {
                totalOrders: data.orders_24h,
                totalGmv: data.gmv_24h,
                activeUsers: data.active_users_24h,
                activeMerchants: data.active_merchants_24h,
                activeOrders
            },
            orderStates: {
                pending: data.pending_orders,
                preparing: data.preparing_orders,
                ready: data.ready_orders,
                delivering: data.delivering_orders
            }
        }
    }
}

// ==================== 导出服务实例 ====================

export const platformDashboardService = new PlatformDashboardService()
export const platformAnalyticsService = new PlatformAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取平台大屏完整数据
 */
export async function getPlatformDashboardData(options?: {
    includeRegionComparison?: boolean
}): Promise<{
    realtime: RealtimeDashboardData
    overview: PlatformOverviewResponse
    merchantGrowth: MerchantGrowthResponse
    userGrowth: UserGrowthResponse
    merchantRanking: MerchantRankingRow[]
    riderRanking: RiderRankingRow[]
    regionComparison: RegionComparisonRow[]
    healthAnalysis: ReturnType<PlatformAnalyticsService['analyzePlatformHealth']>
    growthAnalysis: ReturnType<PlatformAnalyticsService['analyzeGrowthTrends']>
    regionalAnalysis: ReturnType<PlatformAnalyticsService['analyzeRegionalPerformance']>
}> {
    const includeRegionComparison = options?.includeRegionComparison === true
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    const [
        realtime,
        overview,
        merchantGrowth,
        userGrowth,
        merchantRanking,
        riderRanking
    ] = await Promise.all([
        platformDashboardService.getRealtimeDashboard(),
        platformDashboardService.getPlatformOverview({ start_date: startDate, end_date: endDate }),
        platformDashboardService.getMerchantGrowth({ start_date: startDate, end_date: endDate }),
        platformDashboardService.getUserGrowth({ start_date: startDate, end_date: endDate }),
        platformDashboardService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 20 }),
        platformDashboardService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 20 })
    ])

    const regionComparison = includeRegionComparison
        ? await platformDashboardService.getRegionComparison({ start_date: startDate, end_date: endDate })
        : []

    // 进行数据分析
    const healthAnalysis = platformAnalyticsService.analyzePlatformHealth(overview, realtime)
    const growthAnalysis = platformAnalyticsService.analyzeGrowthTrends(merchantGrowth, userGrowth)
    const regionalAnalysis = platformAnalyticsService.analyzeRegionalPerformance(regionComparison)

    return {
        realtime,
        overview,
        merchantGrowth,
        userGrowth,
        merchantRanking,
        riderRanking,
        regionComparison,
        healthAnalysis,
        growthAnalysis,
        regionalAnalysis
    }
}

/**
 * 生成平台运营报告
 * @param days 分析天数
 */
export async function generatePlatformReport(days: number = 30): Promise<{
    reportTitle: string
    reportPeriod: string
    executiveSummary: {
        healthScore: number
        healthLevel: string
        keyMetrics: string[]
        majorAlerts: string[]
    }
    detailedAnalysis: {
        healthAnalysis: ReturnType<PlatformAnalyticsService['analyzePlatformHealth']>
        growthAnalysis: ReturnType<PlatformAnalyticsService['analyzeGrowthTrends']>
        regionalAnalysis: ReturnType<PlatformAnalyticsService['analyzeRegionalPerformance']>
    }
    actionItems: {
        immediate: string[]
        shortTerm: string[]
        longTerm: string[]
    }
    appendix: {
        dataSource: string
        methodology: string
        limitations: string[]
    }
}> {
    const dashboardData = await getPlatformDashboardData({ includeRegionComparison: true })
    const endDate = new Date().toISOString().split('T')[0]
    const startDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

    // 生成执行摘要
    const executiveSummary = {
        healthScore: dashboardData.healthAnalysis.healthScore,
        healthLevel: dashboardData.healthAnalysis.healthLevel,
        keyMetrics: [
            `总订单数: ${dashboardData.overview.total_orders.toLocaleString()}`,
            `总GMV: ¥${(dashboardData.overview.total_gmv / 100).toLocaleString()}`,
            `平台佣金: ¥${(dashboardData.overview.total_commission / 100).toLocaleString()}`,
            `活跃商户: ${dashboardData.overview.active_merchants.toLocaleString()}`,
            `活跃用户: ${dashboardData.overview.active_users.toLocaleString()}`
        ],
        majorAlerts: dashboardData.healthAnalysis.alerts
            .filter((alert) => alert.level === 'error')
            .map((alert) => alert.message)
    }

    // 生成行动项
    const actionItems = generateReportActionItems(dashboardData)

    return {
        reportTitle: '平台运营分析报告',
        reportPeriod: `${startDate} 至 ${endDate}`,
        executiveSummary,
        detailedAnalysis: {
            healthAnalysis: dashboardData.healthAnalysis,
            growthAnalysis: dashboardData.growthAnalysis,
            regionalAnalysis: dashboardData.regionalAnalysis
        },
        actionItems,
        appendix: {
            dataSource: '平台实时数据库和统计系统',
            methodology: '基于多维度指标的综合分析模型',
            limitations: [
                '数据基于历史趋势，未来预测存在不确定性',
                '部分指标可能受季节性因素影响',
                '外部市场环境变化可能影响分析结果'
            ]
        }
    }
}

/**
 * 生成报告行动项
 */
function generateReportActionItems(dashboardData: Awaited<ReturnType<typeof getPlatformDashboardData>>): {
    immediate: string[]
    shortTerm: string[]
    longTerm: string[]
} {
    const immediate: string[] = []
    const shortTerm: string[] = []
    const longTerm: string[] = []

    // 基于健康分析生成行动项
    dashboardData.healthAnalysis.alerts.forEach((alert) => {
        if (alert.level === 'error') {
            immediate.push(`紧急处理: ${alert.message}`)
        } else if (alert.level === 'warning') {
            shortTerm.push(`关注改善: ${alert.message}`)
        }
    })

    // 基于增长分析生成行动项
    dashboardData.growthAnalysis.recommendations.forEach((rec: string) => {
        shortTerm.push(rec)
    })

    dashboardData.growthAnalysis.riskFactors.forEach((risk: string) => {
        immediate.push(`风险防控: ${risk}`)
    })

    // 基于区域分析生成行动项
    dashboardData.regionalAnalysis.balanceRecommendations.forEach((rec: string) => {
        longTerm.push(rec)
    })

    // 默认行动项
    if (immediate.length === 0) {
        immediate.push('持续监控关键指标，确保平台稳定运行')
    }

    if (shortTerm.length === 0) {
        shortTerm.push('优化用户体验，提升平台服务质量')
    }

    if (longTerm.length === 0) {
        longTerm.push('制定长期发展战略，扩大市场份额')
    }

    return { immediate, shortTerm, longTerm }
}

/**
 * 格式化订单状态显示
 * @param status 订单状态
 */
export function formatOrderStatus(status: OrderStatus): string {
    const statusMap: Record<OrderStatus, string> = {
        pending: '待确认',
        confirmed: '已确认',
        preparing: '制作中',
        ready: '待取餐',
        delivering: '代取中',
        completed: '已完成',
        cancelled: '已取消'
    }
    return statusMap[status] || status
}

/**
 * 格式化健康等级显示
 * @param level 健康等级
 */
export function formatHealthLevel(level: 'excellent' | 'good' | 'warning' | 'critical'): string {
    const levelMap = {
        excellent: '优秀',
        good: '良好',
        warning: '警告',
        critical: '严重'
    }
    return levelMap[level] || level
}

/**
 * 格式化增长趋势显示
 * @param trend 增长趋势
 */
export function formatGrowthTrend(trend: 'accelerating' | 'growing' | 'stable' | 'declining'): string {
    const trendMap = {
        accelerating: '加速增长',
        growing: '稳定增长',
        stable: '保持稳定',
        declining: '增长放缓'
    }
    return trendMap[trend] || trend
}

/**
 * 格式化金额显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
export function formatAmount(amount: number, showUnit: boolean = true): string {
    const yuan = (amount / 100).toFixed(2)
    return showUnit ? `¥${yuan}` : yuan
}

/**
 * 格式化大数字显示
 * @param num 数字
 * @param precision 精度
 */
export function formatLargeNumber(num: number, precision: number = 1): string {
    if (num >= 100000000) {
        return `${(num / 100000000).toFixed(precision)}亿`
    } else if (num >= 10000) {
        return `${(num / 10000).toFixed(precision)}万`
    } else if (num >= 1000) {
        return `${(num / 1000).toFixed(precision)}千`
    }
    return num.toString()
}

/**
 * 验证日期范围参数
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
export function validateDateRange(startDate: string, endDate: string): { valid: boolean, message?: string } {
    const start = new Date(startDate)
    const end = new Date(endDate)

    if (isNaN(start.getTime()) || isNaN(end.getTime())) {
        return { valid: false, message: '日期格式不正确' }
    }

    if (start > end) {
        return { valid: false, message: '开始日期不能晚于结束日期' }
    }

    const daysDiff = (end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24)
    if (daysDiff > 365) {
        return { valid: false, message: '查询时间范围不能超过365天' }
    }

    return { valid: true }
}
