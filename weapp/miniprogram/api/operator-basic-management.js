"use strict";
/**
 * 运营商基础管理接口重构 (Task 4.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：区域管理、财务概览、佣金管理
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
exports.regionAnalyticsService = exports.operatorBasicManagementService = exports.OperatorBasicManagementAdapter = exports.RegionAnalyticsService = exports.OperatorBasicManagementService = void 0;
exports.getOperatorDashboard = getOperatorDashboard;
exports.getRegionAnalysisReport = getRegionAnalysisReport;
exports.formatRegionStatus = formatRegionStatus;
exports.formatOperatorStatus = formatOperatorStatus;
exports.formatSettlementStatus = formatSettlementStatus;
exports.formatAmount = formatAmount;
exports.formatPercentage = formatPercentage;
exports.formatGrowthRate = formatGrowthRate;
exports.validateRegionQueryParams = validateRegionQueryParams;
exports.validateCommissionQueryParams = validateCommissionQueryParams;
const request_1 = require("../utils/request");
// ==================== 运营商基础管理服务类 ====================
/**
 * 运营商基础管理服务
 * 提供区域管理、财务概览、运营商信息管理等功能
 */
class OperatorBasicManagementService {
    /**
     * 获取运营商管理的区域列表
     * @param params 查询参数
     */
    getOperatorRegions(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operator/regions',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取指定区域的统计数据
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
     * 获取运营商财务概览
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    getFinanceOverview(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operators/me/finance/overview',
                method: 'GET',
                data: {
                    start_date: startDate,
                    end_date: endDate
                }
            });
        });
    }
    /**
     * 获取佣金明细列表
     * @param params 查询参数
     */
    getCommissionList(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operators/me/commission',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取运营商信息
     */
    getOperatorInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operators/me',
                method: 'GET'
            });
        });
    }
    /**
     * 更新运营商信息
     * @param updateData 更新数据
     */
    updateOperatorInfo(updateData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operators/me',
                method: 'PATCH',
                data: updateData
            });
        });
    }
}
exports.OperatorBasicManagementService = OperatorBasicManagementService;
// ==================== 区域统计分析服务类 ====================
/**
 * 区域统计分析服务
 * 提供区域数据分析、对比等功能
 */
class RegionAnalyticsService {
    /**
     * 计算区域绩效指标
     * @param stats 区域统计数据
     */
    calculateRegionPerformance(stats) {
        const merchantDensity = stats.active_merchant_count / Math.max(stats.merchant_count, 1);
        const riderDensity = stats.active_rider_count / Math.max(stats.rider_count, 1);
        const orderDensity = stats.completed_order_count / Math.max(stats.order_count, 1);
        const avgOrderValue = stats.total_gmv / Math.max(stats.completed_order_count, 1);
        const completionRate = stats.completion_rate;
        const commissionRate = stats.total_commission / Math.max(stats.total_gmv, 1);
        // 计算综合绩效分数 (0-100)
        const performanceScore = Math.min(100, Math.round(merchantDensity * 20 +
            riderDensity * 20 +
            orderDensity * 25 +
            (completionRate / 100) * 25 +
            Math.min(commissionRate * 1000, 10) // 佣金率权重较小
        ));
        let performanceLevel = 'poor';
        if (performanceScore >= 80)
            performanceLevel = 'excellent';
        else if (performanceScore >= 65)
            performanceLevel = 'good';
        else if (performanceScore >= 50)
            performanceLevel = 'average';
        return {
            merchantDensity,
            riderDensity,
            orderDensity,
            avgOrderValue,
            completionRate,
            commissionRate,
            performanceScore,
            performanceLevel
        };
    }
    /**
     * 对比多个区域的绩效
     * @param regionStats 多个区域的统计数据
     */
    compareRegionPerformance(regionStats) {
        var _a, _b;
        if (regionStats.length === 0) {
            return {
                bestRegion: null,
                worstRegion: null,
                avgPerformance: 0,
                regionRankings: []
            };
        }
        const regionRankings = regionStats
            .map(region => ({
            region,
            performance: this.calculateRegionPerformance(region),
            rank: 0
        }))
            .sort((a, b) => b.performance.performanceScore - a.performance.performanceScore)
            .map((item, index) => (Object.assign(Object.assign({}, item), { rank: index + 1 })));
        const avgPerformance = regionRankings.reduce((sum, item) => sum + item.performance.performanceScore, 0) / regionRankings.length;
        return {
            bestRegion: ((_a = regionRankings[0]) === null || _a === void 0 ? void 0 : _a.region) || null,
            worstRegion: ((_b = regionRankings[regionRankings.length - 1]) === null || _b === void 0 ? void 0 : _b.region) || null,
            avgPerformance,
            regionRankings
        };
    }
    /**
     * 分析区域增长趋势
     * @param currentStats 当前统计数据
     * @param previousStats 上期统计数据
     */
    analyzeRegionGrowth(currentStats, previousStats) {
        const merchantGrowth = this.calculateGrowthRate(currentStats.active_merchant_count, previousStats.active_merchant_count);
        const riderGrowth = this.calculateGrowthRate(currentStats.active_rider_count, previousStats.active_rider_count);
        const orderGrowth = this.calculateGrowthRate(currentStats.completed_order_count, previousStats.completed_order_count);
        const gmvGrowth = this.calculateGrowthRate(currentStats.total_gmv, previousStats.total_gmv);
        const commissionGrowth = this.calculateGrowthRate(currentStats.total_commission, previousStats.total_commission);
        const overallGrowth = (merchantGrowth + riderGrowth + orderGrowth + gmvGrowth + commissionGrowth) / 5;
        let growthTrend = 'stable';
        if (overallGrowth > 5)
            growthTrend = 'up';
        else if (overallGrowth < -5)
            growthTrend = 'down';
        return {
            merchantGrowth,
            riderGrowth,
            orderGrowth,
            gmvGrowth,
            commissionGrowth,
            overallGrowth,
            growthTrend
        };
    }
    /**
     * 计算增长率
     * @param current 当前值
     * @param previous 上期值
     */
    calculateGrowthRate(current, previous) {
        if (previous === 0)
            return current > 0 ? 100 : 0;
        return ((current - previous) / previous) * 100;
    }
}
exports.RegionAnalyticsService = RegionAnalyticsService;
// ==================== 数据适配器 ====================
/**
 * 运营商基础管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class OperatorBasicManagementAdapter {
    /**
     * 适配区域响应数据
     */
    static adaptRegionResponse(data) {
        return {
            id: data.id,
            name: data.name,
            code: data.code,
            parentId: data.parent_id,
            level: data.level,
            status: data.status,
            operatorId: data.operator_id,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        };
    }
    /**
     * 适配区域统计响应数据
     */
    static adaptRegionStatsResponse(data) {
        return {
            regionId: data.region_id,
            regionName: data.region_name,
            merchantCount: data.merchant_count,
            activeMerchantCount: data.active_merchant_count,
            riderCount: data.rider_count,
            activeRiderCount: data.active_rider_count,
            orderCount: data.order_count,
            completedOrderCount: data.completed_order_count,
            totalGmv: data.total_gmv,
            totalCommission: data.total_commission,
            avgOrderValue: data.avg_order_value,
            completionRate: data.completion_rate,
            createdAt: data.created_at
        };
    }
    /**
     * 适配财务概览响应数据
     */
    static adaptFinanceOverviewResponse(data) {
        return {
            totalCommission: data.total_commission,
            todayCommission: data.today_commission,
            weekCommission: data.week_commission,
            monthCommission: data.month_commission,
            pendingSettlement: data.pending_settlement,
            settledAmount: data.settled_amount,
            commissionRate: data.commission_rate,
            merchantCount: data.merchant_count,
            activeMerchantCount: data.active_merchant_count,
            orderCount: data.order_count,
            gmv: data.gmv
        };
    }
    /**
     * 适配佣金响应数据
     */
    static adaptCommissionResponse(data) {
        return {
            id: data.id,
            operatorId: data.operator_id,
            orderId: data.order_id,
            merchantId: data.merchant_id,
            commissionAmount: data.commission_amount,
            commissionRate: data.commission_rate,
            orderAmount: data.order_amount,
            settlementStatus: data.settlement_status,
            settlementDate: data.settlement_date,
            createdAt: data.created_at
        };
    }
    /**
     * 适配运营商响应数据
     */
    static adaptOperatorResponse(data) {
        return {
            id: data.id,
            userId: data.user_id,
            name: data.name,
            phone: data.phone,
            email: data.email,
            regionIds: data.region_ids,
            status: data.status,
            commissionRate: data.commission_rate,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        };
    }
}
exports.OperatorBasicManagementAdapter = OperatorBasicManagementAdapter;
// ==================== 导出服务实例 ====================
exports.operatorBasicManagementService = new OperatorBasicManagementService();
exports.regionAnalyticsService = new RegionAnalyticsService();
// ==================== 便捷函数 ====================
/**
 * 获取运营商工作台数据
 */
function getOperatorDashboard() {
    return __awaiter(this, void 0, void 0, function* () {
        const [operatorInfo, financeOverview, regions] = yield Promise.all([
            exports.operatorBasicManagementService.getOperatorInfo(),
            exports.operatorBasicManagementService.getFinanceOverview(),
            exports.operatorBasicManagementService.getOperatorRegions({ limit: 100 })
        ]);
        // 获取各区域统计数据
        const regionStatsPromises = regions.regions.map(region => exports.operatorBasicManagementService.getRegionStats(region.id));
        const regionStats = yield Promise.all(regionStatsPromises);
        // 分析区域绩效
        const regionPerformance = exports.regionAnalyticsService.compareRegionPerformance(regionStats);
        // 获取最近的佣金记录
        const commissionResult = yield exports.operatorBasicManagementService.getCommissionList({
            page: 1,
            limit: 10
        });
        return {
            operatorInfo,
            financeOverview,
            regionStats,
            regionPerformance,
            recentCommissions: commissionResult.commissions
        };
    });
}
/**
 * 获取区域详细分析报告
 * @param regionId 区域ID
 * @param days 分析天数
 */
function getRegionAnalysisReport(regionId_1) {
    return __awaiter(this, arguments, void 0, function* (regionId, days = 30) {
        const endDate = new Date().toISOString().split('T')[0];
        const startDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const previousEndDate = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const previousStartDate = new Date(Date.now() - days * 2 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
        const [regions, currentStats] = yield Promise.all([
            exports.operatorBasicManagementService.getOperatorRegions({ limit: 1000 }),
            exports.operatorBasicManagementService.getRegionStats(regionId, startDate, endDate)
        ]);
        const regionInfo = regions.regions.find(r => r.id === regionId);
        if (!regionInfo) {
            throw new Error('区域不存在');
        }
        const performance = exports.regionAnalyticsService.calculateRegionPerformance(currentStats);
        // 尝试获取上期数据进行对比
        let previousStats;
        let growth;
        try {
            previousStats = yield exports.operatorBasicManagementService.getRegionStats(regionId, previousStartDate, previousEndDate);
            growth = exports.regionAnalyticsService.analyzeRegionGrowth(currentStats, previousStats);
        }
        catch (error) {
            console.warn('无法获取上期数据:', error);
        }
        // 生成改进建议
        const recommendations = generateRegionRecommendations(performance, growth);
        return {
            regionInfo,
            currentStats,
            previousStats,
            performance,
            growth,
            recommendations
        };
    });
}
/**
 * 生成区域改进建议
 * @param performance 绩效数据
 * @param growth 增长数据
 */
function generateRegionRecommendations(performance, growth) {
    const recommendations = [];
    // 基于绩效水平的建议
    if (performance.performanceLevel === 'poor') {
        recommendations.push('区域整体绩效较差，建议重点关注商户和骑手的活跃度提升');
    }
    // 基于商户密度的建议
    if (performance.merchantDensity < 0.6) {
        recommendations.push('活跃商户比例偏低，建议加强商户运营和激励措施');
    }
    // 基于骑手密度的建议
    if (performance.riderDensity < 0.7) {
        recommendations.push('活跃骑手比例偏低，建议优化配送任务分配和奖励机制');
    }
    // 基于完成率的建议
    if (performance.completionRate < 85) {
        recommendations.push('订单完成率偏低，建议分析取消原因并优化服务流程');
    }
    // 基于增长趋势的建议
    if (growth) {
        if (growth.growthTrend === 'down') {
            recommendations.push('区域增长趋势下降，建议制定针对性的市场拓展策略');
        }
        if (growth.merchantGrowth < 0) {
            recommendations.push('商户数量下降，建议加强商户招募和留存工作');
        }
        if (growth.riderGrowth < 0) {
            recommendations.push('骑手数量下降，建议优化骑手福利和工作环境');
        }
    }
    // 基于订单价值的建议
    if (performance.avgOrderValue < 3000) { // 30元
        recommendations.push('平均订单价值偏低，建议推广高价值商品和套餐优惠');
    }
    return recommendations;
}
/**
 * 格式化区域状态显示
 * @param status 区域状态
 */
function formatRegionStatus(status) {
    const statusMap = {
        active: '正常',
        inactive: '停用',
        pending: '待审核'
    };
    return statusMap[status] || status;
}
/**
 * 格式化运营商状态显示
 * @param status 运营商状态
 */
function formatOperatorStatus(status) {
    const statusMap = {
        active: '正常',
        suspended: '暂停',
        pending_approval: '待审核'
    };
    return statusMap[status] || status;
}
/**
 * 格式化佣金结算状态显示
 * @param status 结算状态
 */
function formatSettlementStatus(status) {
    const statusMap = {
        pending: '待结算',
        settled: '已结算',
        cancelled: '已取消'
    };
    return statusMap[status] || status;
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
 * 格式化百分比显示
 * @param value 数值
 * @param decimals 小数位数
 */
function formatPercentage(value, decimals = 1) {
    return `${value.toFixed(decimals)}%`;
}
/**
 * 格式化增长率显示
 * @param growth 增长率
 * @param showSign 是否显示正负号
 */
function formatGrowthRate(growth, showSign = true) {
    const sign = showSign && growth > 0 ? '+' : '';
    return `${sign}${growth.toFixed(1)}%`;
}
/**
 * 验证区域查询参数
 * @param params 查询参数
 */
function validateRegionQueryParams(params) {
    if (params.page && params.page < 1) {
        return { valid: false, message: '页码必须大于0' };
    }
    if (params.limit && (params.limit < 1 || params.limit > 100)) {
        return { valid: false, message: '每页数量必须在1-100之间' };
    }
    if (params.level && (params.level < 1 || params.level > 5)) {
        return { valid: false, message: '区域级别必须在1-5之间' };
    }
    return { valid: true };
}
/**
 * 验证佣金查询参数
 * @param params 查询参数
 */
function validateCommissionQueryParams(params) {
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
