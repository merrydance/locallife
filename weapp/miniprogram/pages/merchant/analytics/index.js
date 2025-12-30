"use strict";
/**
 * 经营统计页面
 * 集成全部 9 个后端统计 API，提供全面的数据分析
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
const request_1 = require("@/utils/request");
// 统计服务
const StatsService = {
    getOverview(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/overview', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getDailyStats(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/daily', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getTopDishes(startDate_1, endDate_1) {
        return __awaiter(this, arguments, void 0, function* (startDate, endDate, limit = 10) {
            return (0, request_1.request)({ url: '/v1/merchant/stats/dishes/top', method: 'GET', data: { start_date: startDate, end_date: endDate, limit } });
        });
    },
    getHourlyStats(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/hourly', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getOrderSourceStats(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/sources', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getRepurchaseRate(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/repurchase', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getCategoryStats(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/stats/categories', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getCustomers() {
        return __awaiter(this, arguments, void 0, function* (orderBy = 'total_amount', page = 1, limit = 10) {
            return (0, request_1.request)({ url: '/v1/merchant/stats/customers', method: 'GET', data: { order_by: orderBy, page, limit } });
        });
    }
};
Page({
    data: {
        sidebarCollapsed: false,
        loading: true,
        // 日期范围
        dateRange: 'week',
        startDate: '',
        endDate: '',
        // 概览数据
        overview: null,
        // 日报数据
        dailyStats: [],
        // 热门菜品
        topDishes: [],
        // 时段分析
        hourlyStats: [],
        // 订单来源
        sourceStats: [],
        // 复购率
        repurchaseRate: null,
        // 分类销售
        categoryStats: [],
        // 顾客排行
        topCustomers: []
    },
    onLoad() {
        this.setDateRange('week');
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // 设置日期范围
    setDateRange(range) {
        const today = new Date();
        let startDate = this.formatDate(today);
        const endDate = this.formatDate(today);
        if (range === 'week') {
            const weekAgo = new Date(today);
            weekAgo.setDate(weekAgo.getDate() - 6);
            startDate = this.formatDate(weekAgo);
        }
        else if (range === 'month') {
            const monthAgo = new Date(today);
            monthAgo.setDate(monthAgo.getDate() - 29);
            startDate = this.formatDate(monthAgo);
        }
        this.setData({ dateRange: range, startDate, endDate });
        this.loadAllData();
    },
    onDateRangeChange(e) {
        const range = e.currentTarget.dataset.range;
        this.setDateRange(range);
    },
    formatDate(date) {
        const year = date.getFullYear();
        const month = ('0' + (date.getMonth() + 1)).slice(-2);
        const day = ('0' + date.getDate()).slice(-2);
        return `${year}-${month}-${day}`;
    },
    // 加载所有数据
    loadAllData() {
        return __awaiter(this, void 0, void 0, function* () {
            const { startDate, endDate } = this.data;
            this.setData({ loading: true });
            try {
                const [overview, dailyStats, topDishes, hourlyStats, sourceStats, repurchaseRate, categoryStats, topCustomers] = yield Promise.all([
                    StatsService.getOverview(startDate, endDate),
                    StatsService.getDailyStats(startDate, endDate),
                    StatsService.getTopDishes(startDate, endDate, 10),
                    StatsService.getHourlyStats(startDate, endDate),
                    StatsService.getOrderSourceStats(startDate, endDate),
                    StatsService.getRepurchaseRate(startDate, endDate),
                    StatsService.getCategoryStats(startDate, endDate),
                    StatsService.getCustomers('total_amount', 1, 10)
                ]);
                this.setData({
                    overview,
                    dailyStats: dailyStats || [],
                    topDishes: topDishes || [],
                    hourlyStats: hourlyStats || [],
                    sourceStats: sourceStats || [],
                    repurchaseRate,
                    categoryStats: categoryStats || [],
                    topCustomers: topCustomers || [],
                    loading: false
                });
            }
            catch (error) {
                console.error('加载统计数据失败:', error);
                wx.showToast({ title: '加载失败', icon: 'none' });
                this.setData({ loading: false });
            }
        });
    },
    // 格式化金额（分转元）
    formatAmount(fen) {
        return (fen / 100).toFixed(2);
    },
    // 订单类型中文
    formatOrderType(type) {
        const map = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'pickup': '自取'
        };
        return map[type] || type;
    }
});
