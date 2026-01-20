"use strict";
/**
 * 商户BI分析接口
 * 基于swagger.json完全重构，包含销售统计、财务分析、客户分析等
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.AnalyticsAdapter = exports.MerchantFinanceService = exports.MerchantStatsService = void 0;
const request_1 = require("../utils/request");
// ==================== 统计分析服务 ====================
/**
 * 统计分析服务
 */
class MerchantStatsService {
    /**
     * 获取统计概览
     * GET /v1/merchant/stats/overview
     */
    static async getStatsOverview(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/overview',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取每日统计
     * GET /v1/merchant/stats/daily
     */
    static async getDailyStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/daily',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取小时统计
     * GET /v1/merchant/stats/hourly
     */
    static async getHourlyStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/hourly',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取热门菜品
     * GET /v1/merchant/stats/dishes/top
     */
    static async getTopDishes(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/dishes/top',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取分类统计
     * GET /v1/merchant/stats/categories
     */
    static async getCategoryStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/categories',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取客户统计
     * GET /v1/merchant/stats/customers
     */
    static async getCustomerStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/customers',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取复购率统计
     * GET /v1/merchant/stats/repurchase
     */
    static async getRepurchaseStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/repurchase',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取订单来源统计
     * GET /v1/merchant/stats/sources
     */
    static async getOrderSourceStats(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/stats/sources',
            method: 'GET',
            data: params
        });
    }
}
exports.MerchantStatsService = MerchantStatsService;
// ==================== 财务分析服务 ====================
/**
 * 财务分析服务
 */
class MerchantFinanceService {
    /**
     * 获取财务概览
     * GET /v1/merchant/finance/overview
     */
    static async getFinanceOverview(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/finance/overview',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取每日财务
     * GET /v1/merchant/finance/daily
     */
    static async getDailyFinance(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/finance/daily',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取订单财务明细
     * GET /v1/merchant/finance/orders
     */
    static async getOrderFinance(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/finance/orders',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取结算记录
     * GET /v1/merchant/finance/settlements
     */
    static async getSettlements(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/finance/settlements',
            method: 'GET',
            data: params
        });
    }
}
exports.MerchantFinanceService = MerchantFinanceService;
// ==================== 分析数据适配器 ====================
/**
 * 分析数据适配器
 */
class AnalyticsAdapter {
    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents) {
        return (amountInCents / 100).toFixed(2);
    }
    /**
     * 格式化百分比
     */
    static formatPercentage(value) {
        return `${(value * 100).toFixed(1)}%`;
    }
    /**
     * 格式化增长率
     */
    static formatGrowthRate(rate) {
        const sign = rate >= 0 ? '+' : '';
        return `${sign}${(rate * 100).toFixed(1)}%`;
    }
    /**
     * 计算环比增长
     */
    static calculateGrowth(current, previous) {
        if (previous === 0)
            return current > 0 ? 1 : 0;
        return (current - previous) / previous;
    }
    /**
     * 格式化日期范围
     */
    static formatDateRange(startDate, endDate) {
        return `${startDate} 至 ${endDate}`;
    }
    /**
     * 获取增长率颜色
     */
    static getGrowthColor(rate) {
        if (rate > 0)
            return '#52c41a'; // 绿色
        if (rate < 0)
            return '#ff4d4f'; // 红色
        return '#999'; // 灰色
    }
    /**
     * 转换为图表数据格式
     */
    static toChartData(data) {
        return {
            labels: data.map(d => d.date),
            datasets: [
                data.map(d => d.orders),
                data.map(d => d.revenue / 100)
            ]
        };
    }
    /**
     * 转换为饼图数据格式
     */
    static toPieChartData(data) {
        return {
            labels: data.map(d => d.category_name),
            values: data.map(d => d.percentage)
        };
    }
}
exports.AnalyticsAdapter = AnalyticsAdapter;
// ==================== 导出默认服务 ====================
exports.default = {
    MerchantStatsService,
    MerchantFinanceService,
    AnalyticsAdapter
};
