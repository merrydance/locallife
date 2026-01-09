"use strict";
/**
 * 财务和统计分析接口重构 (Task 2.6)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：财务概览、财务明细、统计分析、客户分析、菜品分析
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
exports.dishAnalyticsService = exports.customerAnalyticsService = exports.statsAnalyticsService = exports.financeManagementService = exports.FinanceAnalyticsAdapter = exports.DishAnalyticsService = exports.CustomerAnalyticsService = exports.StatsAnalyticsService = exports.FinanceManagementService = void 0;
exports.getComprehensiveAnalytics = getComprehensiveAnalytics;
exports.calculateFinanceMetrics = calculateFinanceMetrics;
exports.analyzeSalesTrend = analyzeSalesTrend;
exports.generateBusinessSuggestions = generateBusinessSuggestions;
exports.formatAmount = formatAmount;
exports.formatPercentage = formatPercentage;
exports.calculateGrowthRate = calculateGrowthRate;
const request_1 = require("../utils/request");
// ==================== 财务管理服务类 ====================
/**
 * 财务管理服务
 * 提供财务概览、明细查询、结算记录等功能
 */
class FinanceManagementService {
    /**
     * 获取财务概览
     * @param params 查询参数
     */
    getFinanceOverview(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/overview',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取每日财务汇总
     * @param params 查询参数
     */
    getDailyFinanceSummary(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/daily',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单收入明细
     * @param params 查询参数
     */
    getFinanceOrders(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/orders',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取结算记录
     * @param params 查询参数
     */
    getSettlements(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/settlements',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取促销支出明细
     * @param params 查询参数
     */
    getPromotionExpenses(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/promotions',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取服务费明细
     * @param params 查询参数
     */
    getServiceFees(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/finance/service-fees',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.FinanceManagementService = FinanceManagementService;
// ==================== 统计分析服务类 ====================
/**
 * 统计分析服务
 * 提供商户统计概览、日报、时段分析等功能
 */
class StatsAnalyticsService {
    /**
     * 获取商户概览统计
     * @param params 查询参数
     */
    getMerchantOverview(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/overview',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商户日报统计
     * @param params 查询参数
     */
    getDailyStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/daily',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取小时分布统计
     * @param params 查询参数
     */
    getHourlyStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/hourly',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单来源统计
     * @param params 查询参数
     */
    getSourceStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/sources',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.StatsAnalyticsService = StatsAnalyticsService;
// ==================== 客户分析服务类 ====================
/**
 * 客户分析服务
 * 提供客户列表、复购率分析等功能
 */
class CustomerAnalyticsService {
    /**
     * 获取商户顾客列表
     * @param params 查询参数
     */
    getCustomers(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/customers',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取顾客详情
     * @param userId 用户ID
     */
    getCustomerDetail(userId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/stats/customers/${userId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取复购率统计
     * @param params 查询参数
     */
    getRepurchaseRate(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/repurchase',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.CustomerAnalyticsService = CustomerAnalyticsService;
// ==================== 菜品分析服务类 ====================
/**
 * 菜品分析服务
 * 提供热门菜品、分类统计等功能
 */
class DishAnalyticsService {
    /**
     * 获取菜品销量排行
     * @param params 查询参数
     */
    getTopSellingDishes(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/dishes/top',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取菜品分类统计
     * @param params 查询参数
     */
    getCategoryStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/stats/categories',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.DishAnalyticsService = DishAnalyticsService;
// ==================== 数据适配器 ====================
/**
 * 财务和统计分析数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class FinanceAnalyticsAdapter {
    /**
     * 适配财务概览响应数据
     */
    static adaptFinanceOverviewResponse(data) {
        return {
            totalGmv: data.total_gmv,
            totalIncome: data.total_income,
            netIncome: data.net_income,
            pendingIncome: data.pending_income,
            totalServiceFee: data.total_service_fee,
            totalPlatformFee: data.total_platform_fee,
            totalOperatorFee: data.total_operator_fee,
            totalPromotionExp: data.total_promotion_exp,
            completedOrders: data.completed_orders,
            pendingOrders: data.pending_orders,
            promotionOrders: data.promotion_orders
        };
    }
    /**
     * 适配商户概览响应数据
     */
    static adaptMerchantOverviewResponse(data) {
        return {
            totalDays: data.total_days,
            totalOrders: data.total_orders,
            totalSales: data.total_sales,
            totalCommission: data.total_commission,
            avgDailySales: data.avg_daily_sales
        };
    }
    /**
     * 适配日报统计数据
     */
    static adaptDailyStatRow(data) {
        return {
            date: data.date,
            orderCount: data.order_count,
            totalSales: data.total_sales,
            commission: data.commission,
            takeoutOrders: data.takeout_orders,
            dineInOrders: data.dine_in_orders
        };
    }
    /**
     * 适配复购率响应数据
     */
    static adaptRepurchaseRateResponse(data) {
        return {
            totalUsers: data.total_users,
            repeatUsers: data.repeat_users,
            repurchaseRate: data.repurchase_rate,
            avgOrdersPerUser: data.avg_orders_per_user
        };
    }
    /**
     * 适配热门菜品数据
     */
    static adaptTopSellingDishRow(data) {
        return {
            dishId: data.dish_id,
            dishName: data.dish_name,
            dishPrice: data.dish_price,
            totalSold: data.total_sold,
            totalRevenue: data.total_revenue
        };
    }
    /**
     * 适配菜品分类统计数据
     */
    static adaptDishCategoryStatsRow(data) {
        return {
            categoryName: data.category_name,
            orderCount: data.order_count,
            totalQuantity: data.total_quantity,
            totalSales: data.total_sales
        };
    }
}
exports.FinanceAnalyticsAdapter = FinanceAnalyticsAdapter;
// ==================== 导出服务实例 ====================
exports.financeManagementService = new FinanceManagementService();
exports.statsAnalyticsService = new StatsAnalyticsService();
exports.customerAnalyticsService = new CustomerAnalyticsService();
exports.dishAnalyticsService = new DishAnalyticsService();
// ==================== 便捷函数 ====================
/**
 * 获取完整的商户分析报告
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
function getComprehensiveAnalytics(startDate, endDate) {
    return __awaiter(this, void 0, void 0, function* () {
        const params = { start_date: startDate, end_date: endDate };
        const [financeOverview, merchantOverview, dailyStats, repurchaseRate, topDishes, categoryStats] = yield Promise.all([
            exports.financeManagementService.getFinanceOverview(params),
            exports.statsAnalyticsService.getMerchantOverview(params),
            exports.statsAnalyticsService.getDailyStats(params),
            exports.customerAnalyticsService.getRepurchaseRate(params),
            exports.dishAnalyticsService.getTopSellingDishes(Object.assign(Object.assign({}, params), { limit: 10 })),
            exports.dishAnalyticsService.getCategoryStats(params)
        ]);
        return {
            financeOverview,
            merchantOverview,
            dailyStats,
            repurchaseRate,
            topDishes,
            categoryStats
        };
    });
}
/**
 * 计算财务指标
 * @param financeData 财务数据
 */
function calculateFinanceMetrics(financeData) {
    const totalOrders = financeData.completed_orders + financeData.pending_orders;
    return {
        // 利润率 = 净收入 / 总GMV
        profitMargin: financeData.total_gmv > 0 ? (financeData.net_income / financeData.total_gmv) * 100 : 0,
        // 服务费率 = 总服务费 / 总GMV
        serviceFeeRate: financeData.total_gmv > 0 ? (financeData.total_service_fee / financeData.total_gmv) * 100 : 0,
        // 促销支出率 = 促销支出 / 总GMV
        promotionRate: financeData.total_gmv > 0 ? (financeData.total_promotion_exp / financeData.total_gmv) * 100 : 0,
        // 平均订单价值 = 总GMV / 总订单数
        avgOrderValue: totalOrders > 0 ? financeData.total_gmv / totalOrders : 0
    };
}
/**
 * 分析销售趋势
 * @param dailyStats 日报统计数据
 */
function analyzeSalesTrend(dailyStats) {
    var _a;
    if (dailyStats.length < 2) {
        return {
            trend: 'stable',
            growthRate: 0,
            peakDay: dailyStats[0] || null,
            avgDailySales: ((_a = dailyStats[0]) === null || _a === void 0 ? void 0 : _a.total_sales) || 0
        };
    }
    // 计算增长率（最后一天相比第一天）
    const firstDay = dailyStats[0];
    const lastDay = dailyStats[dailyStats.length - 1];
    const growthRate = firstDay.total_sales > 0
        ? ((lastDay.total_sales - firstDay.total_sales) / firstDay.total_sales) * 100
        : 0;
    // 确定趋势
    let trend = 'stable';
    if (growthRate > 5)
        trend = 'up';
    else if (growthRate < -5)
        trend = 'down';
    // 找到销售额最高的一天
    const peakDay = dailyStats.reduce((max, current) => current.total_sales > max.total_sales ? current : max);
    // 计算平均日销售额
    const avgDailySales = dailyStats.reduce((sum, day) => sum + day.total_sales, 0) / dailyStats.length;
    return {
        trend,
        growthRate,
        peakDay,
        avgDailySales
    };
}
/**
 * 生成经营建议
 * @param analytics 分析数据
 */
function generateBusinessSuggestions(analytics) {
    const suggestions = [];
    const { financeOverview, repurchaseRate, topDishes, categoryStats } = analytics;
    // 财务建议
    const metrics = calculateFinanceMetrics(financeOverview);
    if (metrics.profitMargin < 10) {
        suggestions.push('利润率偏低，建议优化成本结构或调整菜品定价');
    }
    if (metrics.promotionRate > 15) {
        suggestions.push('促销支出占比较高，建议评估促销活动效果');
    }
    // 复购率建议
    if (repurchaseRate.repurchase_rate < 30) {
        suggestions.push('客户复购率较低，建议加强会员营销和客户关系维护');
    }
    // 菜品建议
    if (topDishes.length > 0) {
        const topDish = topDishes[0];
        suggestions.push(`${topDish.dish_name}是您的招牌菜品，建议重点推广`);
    }
    // 分类建议
    if (categoryStats.length > 0) {
        const topCategory = categoryStats.reduce((max, current) => current.total_sales > max.total_sales ? current : max);
        suggestions.push(`${topCategory.category_name}分类销售表现最佳，建议丰富该分类菜品`);
    }
    return suggestions;
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
 * @param rate 比率
 * @param decimals 小数位数
 */
function formatPercentage(rate, decimals = 1) {
    return `${rate.toFixed(decimals)}%`;
}
/**
 * 计算同比增长率
 * @param current 当前值
 * @param previous 上期值
 */
function calculateGrowthRate(current, previous) {
    if (previous === 0)
        return current > 0 ? 100 : 0;
    return ((current - previous) / previous) * 100;
}
