"use strict";
/**
 * 平台统计大屏接口重构 (Task 5.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：实时数据、增长数据、排行榜、区域对比
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.platformAnalyticsService = exports.platformDashboardService = exports.PlatformDashboardAdapter = exports.PlatformAnalyticsService = exports.PlatformDashboardService = void 0;
exports.getPlatformDashboardData = getPlatformDashboardData;
exports.generatePlatformReport = generatePlatformReport;
exports.formatOrderStatus = formatOrderStatus;
exports.formatHealthLevel = formatHealthLevel;
exports.formatGrowthTrend = formatGrowthTrend;
exports.formatAmount = formatAmount;
exports.formatLargeNumber = formatLargeNumber;
exports.validateDateRange = validateDateRange;
const request_1 = require("../utils/request");
// ==================== 平台统计大屏服务类 ====================
/**
 * 平台统计大屏服务
 * 提供实时数据、概览、增长分析等功能
 */
class PlatformDashboardService {
    /**
     * 获取实时大盘数据
     */
    getRealtimeDashboard() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/realtime',
                method: 'GET'
            });
        });
    }
    /**
     * 获取平台概览数据
     * @param params 查询参数
     */
    getPlatformOverview(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/overview',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商户增长数据
     * @param params 查询参数
     */
    getMerchantGrowth(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/growth/merchants',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取用户增长数据
     * @param params 查询参数
     */
    getUserGrowth(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/growth/users',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商户排行榜
     * @param params 查询参数
     */
    getMerchantRanking(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/merchants/ranking',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取骑手排行榜
     * @param params 查询参数
     */
    getRiderRanking(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/riders/ranking',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取区域对比数据
     * @param params 查询参数
     */
    getRegionComparison(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/platform/stats/regions/compare',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.PlatformDashboardService = PlatformDashboardService;
// ==================== 数据分析服务类 ====================
/**
 * 平台数据分析服务
 * 提供深度数据分析和洞察功能
 */
class PlatformAnalyticsService {
    /**
     * 分析平台健康度
     * @param overview 平台概览数据
     * @param realtime 实时数据
     */
    analyzePlatformHealth(overview, realtime) {
        // 计算各维度健康度
        const orderHealth = this.calculateOrderHealth(overview.summary, realtime.order_distribution);
        const userHealth = this.calculateUserHealth(overview.growth_metrics, realtime.today_stats);
        const merchantHealth = this.calculateMerchantHealth(overview.summary, overview.growth_metrics);
        const riderHealth = this.calculateRiderHealth(overview.summary, realtime.realtime_stats);
        const financialHealth = this.calculateFinancialHealth(overview.summary);
        // 综合健康度评分
        const healthScore = Math.round((orderHealth * 0.3 + userHealth * 0.2 + merchantHealth * 0.2 + riderHealth * 0.2 + financialHealth * 0.1));
        // 健康等级
        let healthLevel = 'critical';
        if (healthScore >= 85)
            healthLevel = 'excellent';
        else if (healthScore >= 70)
            healthLevel = 'good';
        else if (healthScore >= 50)
            healthLevel = 'warning';
        // 生成告警
        const alerts = this.generateHealthAlerts(overview, realtime);
        // 生成洞察
        const insights = this.generatePlatformInsights(overview, realtime, healthScore);
        return {
            healthScore,
            healthLevel,
            keyMetrics: {
                orderHealth,
                userHealth,
                merchantHealth,
                riderHealth,
                financialHealth
            },
            alerts,
            insights
        };
    }
    /**
     * 分析增长趋势
     * @param merchantGrowth 商户增长数据
     * @param userGrowth 用户增长数据
     */
    analyzeGrowthTrends(merchantGrowth, userGrowth) {
        // 计算增长评分
        const userGrowthScore = this.calculateGrowthScore(userGrowth.summary.growth_rate);
        const merchantGrowthScore = this.calculateGrowthScore(merchantGrowth.summary.growth_rate);
        const growthScore = (userGrowthScore + merchantGrowthScore) / 2;
        // 判断整体趋势
        let overallTrend = 'stable';
        if (growthScore >= 80)
            overallTrend = 'accelerating';
        else if (growthScore >= 60)
            overallTrend = 'growing';
        else if (growthScore < 40)
            overallTrend = 'declining';
        // 简化的预测模型
        const predictions = this.generateGrowthPredictions(merchantGrowth, userGrowth);
        // 生成建议和风险因素
        const recommendations = this.generateGrowthRecommendations(merchantGrowth, userGrowth, overallTrend);
        const riskFactors = this.identifyGrowthRisks(merchantGrowth, userGrowth);
        return {
            overallTrend,
            growthScore,
            predictions,
            recommendations,
            riskFactors
        };
    }
    /**
     * 分析区域绩效
     * @param regions 区域对比数据
     */
    analyzeRegionalPerformance(regions) {
        if (regions.length === 0) {
            return {
                topPerformers: [],
                underPerformers: [],
                averageMetrics: { avgGmv: 0, avgOrders: 0, avgCompletionRate: 0, avgGrowthRate: 0 },
                insights: [],
                balanceRecommendations: []
            };
        }
        // 按绩效评分排序
        const sortedRegions = [...regions].sort((a, b) => b.performance_score - a.performance_score);
        // 识别表现优异和落后的区域
        const topCount = Math.max(1, Math.floor(regions.length * 0.2));
        const bottomCount = Math.max(1, Math.floor(regions.length * 0.2));
        const topPerformers = sortedRegions.slice(0, topCount);
        const underPerformers = sortedRegions.slice(-bottomCount);
        // 计算平均指标
        const totalGmv = regions.reduce((sum, r) => sum + r.gmv, 0);
        const totalOrders = regions.reduce((sum, r) => sum + r.orders, 0);
        const totalCompletionRate = regions.reduce((sum, r) => sum + r.completion_rate, 0);
        const totalGrowthRate = regions.reduce((sum, r) => sum + r.growth_rate, 0);
        const averageMetrics = {
            avgGmv: totalGmv / regions.length,
            avgOrders: totalOrders / regions.length,
            avgCompletionRate: totalCompletionRate / regions.length,
            avgGrowthRate: totalGrowthRate / regions.length
        };
        // 生成洞察
        const insights = this.generateRegionalInsights(topPerformers, underPerformers, averageMetrics);
        // 生成平衡建议
        const balanceRecommendations = this.generateBalanceRecommendations(topPerformers, underPerformers);
        return {
            topPerformers,
            underPerformers,
            averageMetrics,
            insights,
            balanceRecommendations
        };
    }
    /**
     * 计算订单健康度
     */
    calculateOrderHealth(summary, distribution) {
        const completionRate = summary.completion_rate;
        const activeOrderRatio = (distribution.pending + distribution.confirmed + distribution.preparing + distribution.ready + distribution.delivering) /
            Math.max(summary.total_orders, 1) * 100;
        return Math.min(100, (completionRate + Math.min(activeOrderRatio, 20)) / 1.2);
    }
    /**
     * 计算用户健康度
     */
    calculateUserHealth(growth, today) {
        const userGrowthScore = Math.min(100, Math.max(0, growth.user_growth_rate + 50));
        const newUserScore = Math.min(100, (today.new_users / 1000) * 100);
        return (userGrowthScore + newUserScore) / 2;
    }
    /**
     * 计算商户健康度
     */
    calculateMerchantHealth(summary, growth) {
        const merchantGrowthScore = Math.min(100, Math.max(0, growth.merchant_growth_rate + 50));
        const activeMerchantScore = Math.min(100, (summary.active_merchants / 10000) * 100);
        return (merchantGrowthScore + activeMerchantScore) / 2;
    }
    /**
     * 计算骑手健康度
     */
    calculateRiderHealth(summary, realtime) {
        const activeRiderScore = Math.min(100, (summary.active_riders / 5000) * 100);
        const onlineRiderScore = Math.min(100, (realtime.online_riders / 2000) * 100);
        return (activeRiderScore + onlineRiderScore) / 2;
    }
    /**
     * 计算财务健康度
     */
    calculateFinancialHealth(summary) {
        const gmvScore = Math.min(100, (summary.total_gmv / 10000000) * 100); // 1000万为满分
        const avgOrderValueScore = Math.min(100, (summary.avg_order_value / 5000) * 100); // 50元为满分
        return (gmvScore + avgOrderValueScore) / 2;
    }
    /**
     * 生成健康告警
     */
    generateHealthAlerts(overview, realtime) {
        const alerts = [];
        // 完成率告警
        if (overview.summary.completion_rate < 80) {
            alerts.push({
                level: overview.summary.completion_rate < 70 ? 'error' : 'warning',
                message: '订单完成率偏低',
                metric: 'completion_rate',
                value: overview.summary.completion_rate,
                threshold: 80
            });
        }
        // 在线商户告警
        if (realtime.realtime_stats.online_merchants < 100) {
            alerts.push({
                level: 'warning',
                message: '在线商户数量偏少',
                metric: 'online_merchants',
                value: realtime.realtime_stats.online_merchants,
                threshold: 100
            });
        }
        // 在线骑手告警
        if (realtime.realtime_stats.online_riders < 50) {
            alerts.push({
                level: 'warning',
                message: '在线骑手数量偏少',
                metric: 'online_riders',
                value: realtime.realtime_stats.online_riders,
                threshold: 50
            });
        }
        // 增长率告警
        if (overview.growth_metrics.order_growth_rate < 0) {
            alerts.push({
                level: 'error',
                message: '订单增长率为负',
                metric: 'order_growth_rate',
                value: overview.growth_metrics.order_growth_rate,
                threshold: 0
            });
        }
        return alerts;
    }
    /**
     * 生成平台洞察
     */
    generatePlatformInsights(overview, realtime, healthScore) {
        const insights = [];
        if (healthScore >= 85) {
            insights.push('平台整体运营状况优秀，各项指标表现良好');
        }
        else if (healthScore < 50) {
            insights.push('平台运营状况需要重点关注，建议制定改善计划');
        }
        if (overview.growth_metrics.gmv_growth_rate > 20) {
            insights.push('GMV增长强劲，平台商业价值快速提升');
        }
        if (realtime.realtime_stats.orders_per_minute > 10) {
            insights.push('订单频率较高，平台活跃度良好');
        }
        if (overview.summary.avg_order_value > 4000) {
            insights.push('客单价较高，用户消费能力强');
        }
        return insights;
    }
    /**
     * 计算增长评分
     */
    calculateGrowthScore(growthRate) {
        // 将增长率转换为0-100的评分
        if (growthRate >= 20)
            return 100;
        if (growthRate >= 10)
            return 80;
        if (growthRate >= 5)
            return 60;
        if (growthRate >= 0)
            return 40;
        return Math.max(0, 40 + growthRate * 2);
    }
    /**
     * 生成增长预测
     */
    generateGrowthPredictions(merchantGrowth, userGrowth) {
        // 简化的线性预测模型
        const avgDailyUsers = userGrowth.summary.avg_daily_new_users;
        const avgDailyMerchants = merchantGrowth.summary.avg_daily_new_merchants;
        const nextMonthUsers = Math.round(avgDailyUsers * 30);
        const nextMonthMerchants = Math.round(avgDailyMerchants * 30);
        // 基于增长趋势计算置信度
        const userTrendStability = userGrowth.summary.growth_trend === 'stable' ? 0.8 : 0.6;
        const merchantTrendStability = merchantGrowth.summary.growth_trend === 'stable' ? 0.8 : 0.6;
        const confidence = (userTrendStability + merchantTrendStability) / 2;
        return {
            nextMonthUsers,
            nextMonthMerchants,
            confidence
        };
    }
    /**
     * 生成增长建议
     */
    generateGrowthRecommendations(merchantGrowth, userGrowth, overallTrend) {
        const recommendations = [];
        if (overallTrend === 'declining') {
            recommendations.push('增长趋势下滑，建议分析原因并制定挽回策略');
        }
        if (userGrowth.summary.growth_rate < 5) {
            recommendations.push('用户增长缓慢，建议加强市场推广和用户获取');
        }
        if (merchantGrowth.summary.growth_rate < 5) {
            recommendations.push('商户增长缓慢，建议优化招商策略和激励政策');
        }
        if (overallTrend === 'accelerating') {
            recommendations.push('增长势头良好，建议保持现有策略并适度扩大投入');
        }
        return recommendations;
    }
    /**
     * 识别增长风险
     */
    identifyGrowthRisks(merchantGrowth, userGrowth) {
        const risks = [];
        if (userGrowth.summary.growth_rate < 0) {
            risks.push('用户增长为负，存在用户流失风险');
        }
        if (merchantGrowth.summary.growth_rate < 0) {
            risks.push('商户增长为负，可能影响平台供给能力');
        }
        // 检查增长数据的波动性
        const userGrowthData = userGrowth.growth_data;
        if (userGrowthData.length > 7) {
            const recentWeek = userGrowthData.slice(-7);
            const weeklyVariance = this.calculateVariance(recentWeek.map(d => d.new_users));
            if (weeklyVariance > 1000) {
                risks.push('用户增长波动较大，增长稳定性存在风险');
            }
        }
        return risks;
    }
    /**
     * 生成区域洞察
     */
    generateRegionalInsights(topPerformers, underPerformers, averageMetrics) {
        const insights = [];
        if (topPerformers.length > 0) {
            const bestRegion = topPerformers[0];
            insights.push(`${bestRegion.region_name}表现最佳，绩效评分${bestRegion.performance_score.toFixed(1)}`);
        }
        if (underPerformers.length > 0) {
            const worstRegion = underPerformers[underPerformers.length - 1];
            insights.push(`${worstRegion.region_name}需要重点关注，绩效评分${worstRegion.performance_score.toFixed(1)}`);
        }
        if (averageMetrics.avgGrowthRate > 10) {
            insights.push('各区域整体增长良好，平台扩张势头强劲');
        }
        else if (averageMetrics.avgGrowthRate < 0) {
            insights.push('多个区域增长放缓，需要制定区域振兴计划');
        }
        return insights;
    }
    /**
     * 生成平衡建议
     */
    generateBalanceRecommendations(topPerformers, underPerformers) {
        const recommendations = [];
        if (topPerformers.length > 0 && underPerformers.length > 0) {
            recommendations.push('建议将优秀区域的成功经验推广到表现较差的区域');
            recommendations.push('考虑将部分资源从饱和区域转移到潜力区域');
        }
        if (underPerformers.length > 0) {
            recommendations.push('为表现较差的区域制定专项扶持计划');
            recommendations.push('加强对落后区域的运营指导和资源投入');
        }
        return recommendations;
    }
    /**
     * 计算方差
     */
    calculateVariance(values) {
        if (values.length === 0)
            return 0;
        const mean = values.reduce((sum, val) => sum + val, 0) / values.length;
        const squaredDiffs = values.map(val => Math.pow(val - mean, 2));
        return squaredDiffs.reduce((sum, val) => sum + val, 0) / values.length;
    }
}
exports.PlatformAnalyticsService = PlatformAnalyticsService;
// ==================== 数据适配器 ====================
/**
 * 平台统计大屏数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class PlatformDashboardAdapter {
    /**
     * 适配实时大盘数据
     */
    static adaptRealtimeDashboard(data) {
        return {
            timestamp: data.timestamp,
            realtimeStats: {
                onlineUsers: data.realtime_stats.online_users,
                onlineMerchants: data.realtime_stats.online_merchants,
                onlineRiders: data.realtime_stats.online_riders,
                activeOrders: data.realtime_stats.active_orders,
                ordersPerMinute: data.realtime_stats.orders_per_minute,
                gmvPerMinute: data.realtime_stats.gmv_per_minute
            },
            todayStats: {
                totalOrders: data.today_stats.total_orders,
                completedOrders: data.today_stats.completed_orders,
                cancelledOrders: data.today_stats.cancelled_orders,
                totalGmv: data.today_stats.total_gmv,
                avgOrderValue: data.today_stats.avg_order_value,
                completionRate: data.today_stats.completion_rate,
                newUsers: data.today_stats.new_users,
                newMerchants: data.today_stats.new_merchants,
                newRiders: data.today_stats.new_riders
            },
            orderDistribution: data.order_distribution,
            hourlyTrends: data.hourly_trends.map(item => ({
                hour: item.hour,
                orders: item.orders,
                gmv: item.gmv,
                completionRate: item.completion_rate
            })),
            topRegions: data.top_regions.map(item => ({
                regionId: item.region_id,
                regionName: item.region_name,
                orders: item.orders,
                gmv: item.gmv,
                merchants: item.merchants,
                riders: item.riders
            }))
        };
    }
}
exports.PlatformDashboardAdapter = PlatformDashboardAdapter;
// ==================== 导出服务实例 ====================
exports.platformDashboardService = new PlatformDashboardService();
exports.platformAnalyticsService = new PlatformAnalyticsService();
// ==================== 便捷函数 ====================
/**
 * 获取平台大屏完整数据
 */
function getPlatformDashboardData() {
    return __awaiter(this, void 0, void 0, function* () {
        const endDate = new Date().toISOString().split('T')[0];
        const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const [realtime, overview, merchantGrowth, userGrowth, merchantRanking, riderRanking, regionComparison] = yield Promise.all([
            exports.platformDashboardService.getRealtimeDashboard(),
            exports.platformDashboardService.getPlatformOverview({ start_date: startDate, end_date: endDate }),
            exports.platformDashboardService.getMerchantGrowth({ start_date: startDate, end_date: endDate }),
            exports.platformDashboardService.getUserGrowth({ start_date: startDate, end_date: endDate }),
            exports.platformDashboardService.getMerchantRanking({ start_date: startDate, end_date: endDate, limit: 20 }),
            exports.platformDashboardService.getRiderRanking({ start_date: startDate, end_date: endDate, limit: 20 }),
            exports.platformDashboardService.getRegionComparison({ start_date: startDate, end_date: endDate })
        ]);
        // 进行数据分析
        const healthAnalysis = exports.platformAnalyticsService.analyzePlatformHealth(overview, realtime);
        const growthAnalysis = exports.platformAnalyticsService.analyzeGrowthTrends(merchantGrowth, userGrowth);
        const regionalAnalysis = exports.platformAnalyticsService.analyzeRegionalPerformance(regionComparison);
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
        };
    });
}
/**
 * 生成平台运营报告
 * @param days 分析天数
 */
function generatePlatformReport() {
    return __awaiter(this, arguments, void 0, function* (days = 30) {
        const dashboardData = yield getPlatformDashboardData();
        const endDate = new Date().toISOString().split('T')[0];
        const startDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        // 生成执行摘要
        const executiveSummary = {
            healthScore: dashboardData.healthAnalysis.healthScore,
            healthLevel: dashboardData.healthAnalysis.healthLevel,
            keyMetrics: [
                `总订单数: ${dashboardData.overview.summary.total_orders.toLocaleString()}`,
                `总GMV: ¥${(dashboardData.overview.summary.total_gmv / 100).toLocaleString()}`,
                `完成率: ${dashboardData.overview.summary.completion_rate.toFixed(1)}%`,
                `活跃商户: ${dashboardData.overview.summary.active_merchants.toLocaleString()}`,
                `活跃骑手: ${dashboardData.overview.summary.active_riders.toLocaleString()}`
            ],
            majorAlerts: dashboardData.healthAnalysis.alerts
                .filter(alert => alert.level === 'error')
                .map(alert => alert.message)
        };
        // 生成行动项
        const actionItems = generateReportActionItems(dashboardData);
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
        };
    });
}
/**
 * 生成报告行动项
 */
function generateReportActionItems(dashboardData) {
    const immediate = [];
    const shortTerm = [];
    const longTerm = [];
    // 基于健康分析生成行动项
    dashboardData.healthAnalysis.alerts.forEach((alert) => {
        if (alert.level === 'error') {
            immediate.push(`紧急处理: ${alert.message}`);
        }
        else if (alert.level === 'warning') {
            shortTerm.push(`关注改善: ${alert.message}`);
        }
    });
    // 基于增长分析生成行动项
    dashboardData.growthAnalysis.recommendations.forEach((rec) => {
        shortTerm.push(rec);
    });
    dashboardData.growthAnalysis.riskFactors.forEach((risk) => {
        immediate.push(`风险防控: ${risk}`);
    });
    // 基于区域分析生成行动项
    dashboardData.regionalAnalysis.balanceRecommendations.forEach((rec) => {
        longTerm.push(rec);
    });
    // 默认行动项
    if (immediate.length === 0) {
        immediate.push('持续监控关键指标，确保平台稳定运行');
    }
    if (shortTerm.length === 0) {
        shortTerm.push('优化用户体验，提升平台服务质量');
    }
    if (longTerm.length === 0) {
        longTerm.push('制定长期发展战略，扩大市场份额');
    }
    return { immediate, shortTerm, longTerm };
}
/**
 * 格式化订单状态显示
 * @param status 订单状态
 */
function formatOrderStatus(status) {
    const statusMap = {
        pending: '待确认',
        confirmed: '已确认',
        preparing: '制作中',
        ready: '待取餐',
        delivering: '配送中',
        completed: '已完成',
        cancelled: '已取消'
    };
    return statusMap[status] || status;
}
/**
 * 格式化健康等级显示
 * @param level 健康等级
 */
function formatHealthLevel(level) {
    const levelMap = {
        excellent: '优秀',
        good: '良好',
        warning: '警告',
        critical: '严重'
    };
    return levelMap[level] || level;
}
/**
 * 格式化增长趋势显示
 * @param trend 增长趋势
 */
function formatGrowthTrend(trend) {
    const trendMap = {
        accelerating: '加速增长',
        growing: '稳定增长',
        stable: '保持稳定',
        declining: '增长放缓'
    };
    return trendMap[trend] || trend;
}
/**
 * 格式化金额显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
function formatAmount(amount, showUnit = true) {
    const yuan = (amount / 100).toFixed(2);
    return showUnit ? `¥${yuan}` : yuan;
}
/**
 * 格式化大数字显示
 * @param num 数字
 * @param precision 精度
 */
function formatLargeNumber(num, precision = 1) {
    if (num >= 100000000) {
        return `${(num / 100000000).toFixed(precision)}亿`;
    }
    else if (num >= 10000) {
        return `${(num / 10000).toFixed(precision)}万`;
    }
    else if (num >= 1000) {
        return `${(num / 1000).toFixed(precision)}千`;
    }
    return num.toString();
}
/**
 * 验证日期范围参数
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
function validateDateRange(startDate, endDate) {
    const start = new Date(startDate);
    const end = new Date(endDate);
    if (isNaN(start.getTime()) || isNaN(end.getTime())) {
        return { valid: false, message: '日期格式不正确' };
    }
    if (start > end) {
        return { valid: false, message: '开始日期不能晚于结束日期' };
    }
    const daysDiff = (end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24);
    if (daysDiff > 365) {
        return { valid: false, message: '查询时间范围不能超过365天' };
    }
    return { valid: true };
}
