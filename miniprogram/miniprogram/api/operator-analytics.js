"use strict";
/**
 * 运营商数据统计和分析接口重构 (Task 4.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：区域统计、趋势分析、申诉处理
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
exports.dataAnalysisService = exports.operatorAppealService = exports.operatorAnalyticsService = exports.OperatorAnalyticsAdapter = exports.DataAnalysisService = exports.OperatorAppealService = exports.OperatorAnalyticsService = void 0;
exports.getOperatorAnalyticsDashboard = getOperatorAnalyticsDashboard;
exports.generateRegionAnalysisReport = generateRegionAnalysisReport;
exports.formatAppealStatus = formatAppealStatus;
exports.formatAppealType = formatAppealType;
exports.formatAppealPriority = formatAppealPriority;
exports.formatResolutionTime = formatResolutionTime;
exports.validateAppealQueryParams = validateAppealQueryParams;
const request_1 = require("../utils/request");
// ==================== 运营商数据统计服务类 ====================
/**
 * 运营商数据统计服务
 * 提供区域统计、趋势分析等功能
 */
class OperatorAnalyticsService {
    /**
     * 获取区域统计数据
     * @param regionId 区域ID
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    getRegionStats(regionId, startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/regions/${regionId}/stats`,
                method: 'GET',
                data: {
                    start_date: startDate,
                    end_date: endDate
                }
            });
        });
    }
    /**
     * 获取日趋势分析数据
     * @param regionId 区域ID（可选）
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    getDailyTrend(regionId, startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operator/trend/daily',
                method: 'GET',
                data: {
                    region_id: regionId,
                    start_date: startDate,
                    end_date: endDate
                }
            });
        });
    }
}
exports.OperatorAnalyticsService = OperatorAnalyticsService;
// ==================== 运营商申诉处理服务类 ====================
/**
 * 运营商申诉处理服务
 * 提供申诉列表、详情、审核等功能
 */
class OperatorAppealService {
    /**
     * 获取申诉列表
     * @param params 查询参数
     */
    getAppealList(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operator/appeals',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    getAppealDetail(appealId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/appeals/${appealId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 审核申诉
     * @param appealId 申诉ID
     * @param reviewData 审核数据
     */
    reviewAppeal(appealId, reviewData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/appeals/${appealId}/review`,
                method: 'POST',
                data: reviewData
            });
        });
    }
}
exports.OperatorAppealService = OperatorAppealService;
// ==================== 数据分析服务类 ====================
/**
 * 数据分析服务
 * 提供深度数据分析和洞察功能
 */
class DataAnalysisService {
    /**
     * 分析区域绩效趋势
     * @param regionStats 区域统计数据
     * @param previousStats 上期统计数据（可选）
     */
    analyzeRegionPerformanceTrend(regionStats, previousStats) {
        const stats = regionStats;
        // 计算各维度健康度 (0-100)
        const merchantHealth = this.calculateMerchantHealth(stats.merchant_stats);
        const riderHealth = this.calculateRiderHealth(stats.rider_stats);
        const orderHealth = this.calculateOrderHealth(stats.order_stats);
        const financialHealth = this.calculateFinancialHealth(stats.financial_stats, stats.order_stats.total_gmv);
        // 综合绩效评分
        const performanceScore = Math.round((merchantHealth * 0.25 + riderHealth * 0.25 + orderHealth * 0.3 + financialHealth * 0.2));
        // 绩效等级
        let performanceLevel = 'poor';
        if (performanceScore >= 80)
            performanceLevel = 'excellent';
        else if (performanceScore >= 65)
            performanceLevel = 'good';
        else if (performanceScore >= 50)
            performanceLevel = 'average';
        // 增长分析
        let growthAnalysis = undefined;
        if (previousStats) {
            growthAnalysis = {
                merchantGrowth: stats.growth_stats.merchant_growth_rate,
                riderGrowth: stats.growth_stats.rider_growth_rate,
                orderGrowth: stats.growth_stats.order_growth_rate,
                gmvGrowth: stats.growth_stats.gmv_growth_rate,
                overallGrowth: (stats.growth_stats.merchant_growth_rate +
                    stats.growth_stats.rider_growth_rate +
                    stats.growth_stats.order_growth_rate +
                    stats.growth_stats.gmv_growth_rate) / 4
            };
        }
        // 生成洞察和建议
        const insights = this.generateInsights(stats, performanceScore, growthAnalysis);
        const recommendations = this.generateRecommendations(stats, performanceLevel, growthAnalysis);
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
        };
    }
    /**
     * 分析申诉处理效率
     * @param appeals 申诉列表
     */
    analyzeAppealEfficiency(appeals) {
        const now = Date.now();
        const resolvedAppeals = appeals.filter(a => a.status === 'resolved' && a.resolution_time);
        const totalAppeals = appeals.length;
        // 效率指标
        const avgResolutionTime = resolvedAppeals.length > 0
            ? resolvedAppeals.reduce((sum, a) => sum + (a.resolution_time || 0), 0) / resolvedAppeals.length
            : 0;
        const resolutionRate = totalAppeals > 0 ? (resolvedAppeals.length / totalAppeals) * 100 : 0;
        const satisfactionAppeals = resolvedAppeals.filter(a => a.satisfaction_rating);
        const satisfactionRate = satisfactionAppeals.length > 0
            ? satisfactionAppeals.reduce((sum, a) => sum + (a.satisfaction_rating || 0), 0) / satisfactionAppeals.length
            : 0;
        const workload = appeals.filter(a => ['pending', 'processing'].includes(a.status)).length;
        // 分布统计
        const statusDistribution = new Map();
        const typeDistribution = new Map();
        const priorityDistribution = new Map();
        appeals.forEach(appeal => {
            statusDistribution.set(appeal.status, (statusDistribution.get(appeal.status) || 0) + 1);
            typeDistribution.set(appeal.appeal_type, (typeDistribution.get(appeal.appeal_type) || 0) + 1);
            priorityDistribution.set(appeal.priority, (priorityDistribution.get(appeal.priority) || 0) + 1);
        });
        // 趋势分析（简化版）
        const dailyVolumeMap = new Map();
        const resolutionTimeMap = new Map();
        appeals.forEach(appeal => {
            const date = appeal.created_at.split('T')[0];
            dailyVolumeMap.set(date, (dailyVolumeMap.get(date) || 0) + 1);
            if (appeal.status === 'resolved' && appeal.resolved_at && appeal.resolution_time) {
                const resolvedDate = appeal.resolved_at.split('T')[0];
                const existing = resolutionTimeMap.get(resolvedDate) || { total: 0, count: 0 };
                resolutionTimeMap.set(resolvedDate, {
                    total: existing.total + appeal.resolution_time,
                    count: existing.count + 1
                });
            }
        });
        const dailyVolume = Array.from(dailyVolumeMap.entries())
            .map(([date, count]) => ({ date, count }))
            .sort((a, b) => a.date.localeCompare(b.date));
        const resolutionTrend = Array.from(resolutionTimeMap.entries())
            .map(([date, data]) => ({ date, avgTime: data.total / data.count }))
            .sort((a, b) => a.date.localeCompare(b.date));
        // 生成洞察和行动项
        const insights = this.generateAppealInsights({
            avgResolutionTime,
            resolutionRate,
            satisfactionRate,
            workload
        }, { statusDistribution, typeDistribution, priorityDistribution });
        const actionItems = this.generateAppealActionItems({
            avgResolutionTime,
            resolutionRate,
            satisfactionRate,
            workload
        }, { statusDistribution, typeDistribution, priorityDistribution });
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
        };
    }
    /**
     * 计算商户健康度
     */
    calculateMerchantHealth(merchantStats) {
        const activeRate = merchantStats.total_merchants > 0
            ? (merchantStats.active_merchants / merchantStats.total_merchants) * 100
            : 0;
        const ratingScore = (merchantStats.avg_rating / 5) * 100;
        const suspendedPenalty = merchantStats.total_merchants > 0
            ? (merchantStats.suspended_merchants / merchantStats.total_merchants) * 50
            : 0;
        return Math.max(0, Math.min(100, (activeRate + ratingScore) / 2 - suspendedPenalty));
    }
    /**
     * 计算骑手健康度
     */
    calculateRiderHealth(riderStats) {
        const activeRate = riderStats.total_riders > 0
            ? (riderStats.active_riders / riderStats.total_riders) * 100
            : 0;
        const onlineRate = riderStats.active_riders > 0
            ? (riderStats.online_riders / riderStats.active_riders) * 100
            : 0;
        const ratingScore = (riderStats.avg_rating / 5) * 100;
        const timeScore = Math.max(0, 100 - (riderStats.avg_delivery_time / 60)); // 假设60分钟为基准
        return Math.max(0, Math.min(100, (activeRate + onlineRate + ratingScore + timeScore) / 4));
    }
    /**
     * 计算订单健康度
     */
    calculateOrderHealth(orderStats) {
        const completionRate = orderStats.completion_rate;
        const volumeScore = Math.min(100, (orderStats.total_orders / 10000) * 100); // 假设10000单为满分
        const valueScore = Math.min(100, (orderStats.avg_order_value / 5000) * 100); // 假设50元为满分
        return Math.max(0, Math.min(100, (completionRate + volumeScore + valueScore) / 3));
    }
    /**
     * 计算财务健康度
     */
    calculateFinancialHealth(financialStats, totalGmv) {
        const commissionRate = totalGmv > 0 ? (financialStats.total_commission / totalGmv) * 100 : 0;
        const settlementRate = financialStats.total_commission > 0
            ? (financialStats.settlement_amount / financialStats.total_commission) * 100
            : 0;
        // 假设合理的佣金率在5-15%之间
        const commissionScore = commissionRate >= 5 && commissionRate <= 15 ? 100 : Math.max(0, 100 - Math.abs(commissionRate - 10) * 10);
        return Math.max(0, Math.min(100, (commissionScore + settlementRate) / 2));
    }
    /**
     * 生成区域洞察
     */
    generateInsights(stats, performanceScore, growthAnalysis) {
        const insights = [];
        // 绩效洞察
        if (performanceScore >= 80) {
            insights.push('区域整体表现优秀，各项指标均达到良好水平');
        }
        else if (performanceScore < 50) {
            insights.push('区域表现需要重点关注，建议制定改善计划');
        }
        // 商户洞察
        if (stats.merchant_stats.active_merchants / stats.merchant_stats.total_merchants < 0.7) {
            insights.push('活跃商户比例偏低，需要加强商户运营');
        }
        // 骑手洞察
        if (stats.rider_stats.online_riders / stats.rider_stats.active_riders < 0.5) {
            insights.push('在线骑手比例偏低，可能影响配送效率');
        }
        // 订单洞察
        if (stats.order_stats.completion_rate < 85) {
            insights.push('订单完成率偏低，需要分析取消原因');
        }
        // 增长洞察
        if (growthAnalysis) {
            if (growthAnalysis.overallGrowth > 10) {
                insights.push('区域呈现良好增长态势，各项指标稳步提升');
            }
            else if (growthAnalysis.overallGrowth < 0) {
                insights.push('区域增长出现下滑，需要及时采取措施');
            }
        }
        return insights;
    }
    /**
     * 生成区域建议
     */
    generateRecommendations(stats, performanceLevel, growthAnalysis) {
        const recommendations = [];
        // 基于绩效等级的建议
        if (performanceLevel === 'poor') {
            recommendations.push('建议制定区域改善计划，重点关注薄弱环节');
        }
        // 商户相关建议
        if (stats.merchant_stats.suspended_merchants > stats.merchant_stats.total_merchants * 0.1) {
            recommendations.push('暂停商户数量较多，建议加强商户管理和培训');
        }
        // 骑手相关建议
        if (stats.rider_stats.avg_delivery_time > 3600) {
            recommendations.push('平均配送时间较长，建议优化配送路线和调度');
        }
        // 订单相关建议
        if (stats.order_stats.avg_order_value < 2000) {
            recommendations.push('客单价偏低，建议推广高价值商品和套餐');
        }
        // 增长相关建议
        if (growthAnalysis && growthAnalysis.merchantGrowth < 0) {
            recommendations.push('商户数量下降，建议加强招商和留存工作');
        }
        return recommendations;
    }
    /**
     * 生成申诉洞察
     */
    generateAppealInsights(efficiency, distribution) {
        const insights = [];
        if (efficiency.avgResolutionTime > 24 * 60) { // 超过24小时
            insights.push('平均处理时间较长，可能影响用户满意度');
        }
        if (efficiency.resolutionRate < 80) {
            insights.push('申诉解决率偏低，需要提高处理效率');
        }
        if (efficiency.workload > 50) {
            insights.push('待处理申诉数量较多，建议增加处理人员');
        }
        const orderIssueCount = distribution.byType.get('order_issue') || 0;
        const totalCount = Array.from(distribution.byType.values()).reduce((sum, count) => sum + count, 0);
        if (orderIssueCount / totalCount > 0.5) {
            insights.push('订单相关申诉占比较高，建议重点关注订单流程');
        }
        return insights;
    }
    /**
     * 生成申诉行动项
     */
    generateAppealActionItems(efficiency, distribution) {
        const actionItems = [];
        if (efficiency.avgResolutionTime > 24 * 60) {
            actionItems.push('优化申诉处理流程，缩短平均处理时间');
        }
        if (efficiency.workload > 50) {
            actionItems.push('考虑增加客服人员或优化工作分配');
        }
        const urgentCount = distribution.byPriority.get('urgent') || 0;
        if (urgentCount > 10) {
            actionItems.push('优先处理紧急申诉，建立快速响应机制');
        }
        if (efficiency.satisfactionRate < 4.0) {
            actionItems.push('提升申诉处理质量，改善用户满意度');
        }
        return actionItems;
    }
}
exports.DataAnalysisService = DataAnalysisService;
// ==================== 数据适配器 ====================
/**
 * 运营商数据统计适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class OperatorAnalyticsAdapter {
    /**
     * 适配区域统计响应数据
     */
    static adaptRegionStatsResponse(data) {
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
        };
    }
    /**
     * 适配申诉项数据
     */
    static adaptAppealItem(data) {
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
        };
    }
}
exports.OperatorAnalyticsAdapter = OperatorAnalyticsAdapter;
// ==================== 导出服务实例 ====================
exports.operatorAnalyticsService = new OperatorAnalyticsService();
exports.operatorAppealService = new OperatorAppealService();
exports.dataAnalysisService = new DataAnalysisService();
// ==================== 便捷函数 ====================
/**
 * 获取运营商分析工作台数据
 * @param regionId 区域ID（可选）
 */
function getOperatorAnalyticsDashboard(regionId) {
    return __awaiter(this, void 0, void 0, function* () {
        const endDate = new Date().toISOString().split('T')[0];
        const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const [regionStats, trendAnalysis, appealList] = yield Promise.all([
            exports.operatorAnalyticsService.getRegionStats(regionId || 1, startDate, endDate),
            exports.operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate),
            exports.operatorAppealService.getAppealList({
                region_id: regionId,
                limit: 20,
                sort_by: 'created_at',
                sort_order: 'desc'
            })
        ]);
        // 分析区域绩效
        const performanceAnalysis = exports.dataAnalysisService.analyzeRegionPerformanceTrend(regionStats);
        // 申诉摘要
        const appealSummary = {
            totalAppeals: appealList.total,
            pendingAppeals: appealList.stats.pending_count,
            avgResolutionTime: appealList.stats.avg_resolution_time,
            satisfactionRate: 4.2 // 模拟数据，实际应该从API获取
        };
        return {
            regionStats,
            trendAnalysis,
            performanceAnalysis,
            appealSummary,
            recentAppeals: appealList.appeals.slice(0, 10)
        };
    });
}
/**
 * 生成区域分析报告
 * @param regionId 区域ID
 * @param days 分析天数
 */
function generateRegionAnalysisReport(regionId_1) {
    return __awaiter(this, arguments, void 0, function* (regionId, days = 30) {
        const endDate = new Date().toISOString().split('T')[0];
        const startDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const [regionStats, trendAnalysis] = yield Promise.all([
            exports.operatorAnalyticsService.getRegionStats(regionId, startDate, endDate),
            exports.operatorAnalyticsService.getDailyTrend(regionId, startDate, endDate)
        ]);
        const detailedAnalysis = exports.dataAnalysisService.analyzeRegionPerformanceTrend(regionStats);
        // 生成关键发现
        const keyFindings = [
            ...detailedAnalysis.insights.slice(0, 3),
            `区域综合绩效评分: ${detailedAnalysis.performanceScore}分`,
            `商户健康度: ${detailedAnalysis.keyMetrics.merchantHealth.toFixed(1)}分`,
            `骑手健康度: ${detailedAnalysis.keyMetrics.riderHealth.toFixed(1)}分`
        ];
        // 生成行动计划
        const actionPlan = generateActionPlan(detailedAnalysis);
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
        };
    });
}
/**
 * 生成行动计划
 * @param analysis 分析结果
 */
function generateActionPlan(analysis) {
    const immediateActions = [];
    const shortTermGoals = [];
    const longTermStrategy = [];
    // 基于绩效等级制定计划
    if (analysis.performanceLevel === 'poor') {
        immediateActions.push('召开紧急会议，分析问题根因');
        immediateActions.push('暂停表现差的商户和骑手');
        shortTermGoals.push('制定30天改善计划');
        longTermStrategy.push('重新评估区域运营策略');
    }
    else if (analysis.performanceLevel === 'average') {
        immediateActions.push('识别关键改善点');
        shortTermGoals.push('提升绩效至良好水平');
        longTermStrategy.push('建立持续改善机制');
    }
    // 基于具体指标制定计划
    if (analysis.keyMetrics.merchantHealth < 60) {
        immediateActions.push('加强商户沟通和支持');
        shortTermGoals.push('提升商户活跃度至80%以上');
    }
    if (analysis.keyMetrics.riderHealth < 60) {
        immediateActions.push('优化骑手激励机制');
        shortTermGoals.push('提升骑手在线率和满意度');
    }
    // 基于增长趋势制定计划
    if (analysis.growthAnalysis && analysis.growthAnalysis.overallGrowth < 0) {
        immediateActions.push('分析增长下滑原因');
        shortTermGoals.push('扭转负增长趋势');
        longTermStrategy.push('制定可持续增长策略');
    }
    return {
        immediateActions,
        shortTermGoals,
        longTermStrategy
    };
}
/**
 * 格式化申诉状态显示
 * @param status 申诉状态
 */
function formatAppealStatus(status) {
    const statusMap = {
        pending: '待处理',
        processing: '处理中',
        resolved: '已解决',
        rejected: '已拒绝',
        closed: '已关闭'
    };
    return statusMap[status] || status;
}
/**
 * 格式化申诉类型显示
 * @param type 申诉类型
 */
function formatAppealType(type) {
    const typeMap = {
        order_issue: '订单问题',
        payment_issue: '支付问题',
        service_issue: '服务问题',
        delivery_issue: '配送问题',
        other: '其他'
    };
    return typeMap[type] || type;
}
/**
 * 格式化申诉优先级显示
 * @param priority 申诉优先级
 */
function formatAppealPriority(priority) {
    const priorityMap = {
        low: '低',
        medium: '中',
        high: '高',
        urgent: '紧急'
    };
    return priorityMap[priority] || priority;
}
/**
 * 格式化时间显示（分钟）
 * @param minutes 分钟数
 */
function formatResolutionTime(minutes) {
    if (minutes < 60) {
        return `${minutes}分钟`;
    }
    else if (minutes < 1440) {
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        return remainingMinutes > 0 ? `${hours}小时${remainingMinutes}分钟` : `${hours}小时`;
    }
    else {
        const days = Math.floor(minutes / 1440);
        const remainingHours = Math.floor((minutes % 1440) / 60);
        return remainingHours > 0 ? `${days}天${remainingHours}小时` : `${days}天`;
    }
}
/**
 * 验证申诉查询参数
 * @param params 查询参数
 */
function validateAppealQueryParams(params) {
    if (params.start_date && params.end_date) {
        const startDate = new Date(params.start_date);
        const endDate = new Date(params.end_date);
        if (startDate > endDate) {
            return { valid: false, message: '开始日期不能晚于结束日期' };
        }
        const daysDiff = (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24);
        if (daysDiff > 365) {
            return { valid: false, message: '查询时间范围不能超过365天' };
        }
    }
    if (params.page && params.page < 1) {
        return { valid: false, message: '页码必须大于0' };
    }
    if (params.limit && (params.limit < 1 || params.limit > 100)) {
        return { valid: false, message: '每页数量必须在1-100之间' };
    }
    return { valid: true };
}
