"use strict";
/**
 * 商户BI分析接口
 * 基于swagger.json完全重构，包含销售统计、财务分析、客户分析等
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
    static getStatsOverview(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/overview',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取每日统计
     * GET /v1/merchant/stats/daily
     */
    static getDailyStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/daily',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取小时统计
     * GET /v1/merchant/stats/hourly
     */
    static getHourlyStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/hourly',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取热门菜品
     * GET /v1/merchant/stats/dishes/top
     */
    static getTopDishes(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/dishes/top',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取分类统计
     * GET /v1/merchant/stats/categories
     */
    static getCategoryStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/categories',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取客户统计
     * GET /v1/merchant/stats/customers
     */
    static getCustomerStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/customers',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取复购率统计
     * GET /v1/merchant/stats/repurchase
     */
    static getRepurchaseStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/repurchase',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单来源统计
     * GET /v1/merchant/stats/sources
     */
    static getOrderSourceStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/stats/sources',
                method: 'GET',
                data: params
            });
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
    static getFinanceOverview(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/finance/overview',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取每日财务
     * GET /v1/merchant/finance/daily
     */
    static getDailyFinance(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/finance/daily',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单财务明细
     * GET /v1/merchant/finance/orders
     */
    static getOrderFinance(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/finance/orders',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取结算记录
     * GET /v1/merchant/finance/settlements
     */
    static getSettlements(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/finance/settlements',
                method: 'GET',
                data: params
            });
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
