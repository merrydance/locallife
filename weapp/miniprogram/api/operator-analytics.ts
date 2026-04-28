/**
 * 运营商数据统计和分析接口重构 (Task 4.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：区域统计、趋势分析
 */

import { request } from '../utils/request'

// ==================== 区域统计相关类型 ====================

/** 实时统计响应 - 对齐 api.operatorRealtimeStatsResponse */
export interface OperatorRealtimeStatsResponse {
    active_merchant_count: number
    active_rider_count: number
    pending_merchant_count: number
    pending_rider_count: number
}

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

/** 趋势分析响应 - 对齐后端 /v1/operator/trend/daily（数组） */
export type OperatorTrendDailyResponse = OperatorDailyTrendItem[]

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
    operator_income: number  // 运营商当日可得金额（后端计算）
    weather?: string
    special_events?: string[]
}

type GrowthAnalysis = {
    merchantGrowth: number
    riderGrowth: number
    orderGrowth: number
    gmvGrowth: number
    overallGrowth: number
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
        const data: Record<string, string | number> = {}
        if (typeof regionId === 'number') data.region_id = regionId
        if (startDate) data.start_date = startDate
        if (endDate) data.end_date = endDate

        return request({
            url: '/v1/operator/trend/daily',
            method: 'GET',
            data
        })
    }

    /**
     * 获取实时统计数据
     */
    async getRealtimeStats(regionId?: number): Promise<OperatorRealtimeStatsResponse> {
        return request({
            url: '/v1/operator/stats/realtime',
            method: 'GET',
            data: regionId ? { region_id: regionId } : undefined
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
        let growthAnalysis: GrowthAnalysis | undefined = undefined
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
        growthAnalysis?: GrowthAnalysis
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
        growthAnalysis?: GrowthAnalysis
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

}

export const operatorAnalyticsService = new OperatorAnalyticsService()
export const dataAnalysisService = new DataAnalysisService()

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
