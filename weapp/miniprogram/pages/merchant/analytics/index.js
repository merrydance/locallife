"use strict";
/**
 * 商户数据分析页面
 * 使用真实后端API
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
const responsive_1 = require("../../../utils/responsive");
const merchant_analytics_1 = require("../../../api/merchant-analytics");
Page({
    data: {
        timeRange: 'TODAY',
        metrics: {
            gmv: { value: 0, change: 0 },
            orderCount: { value: 0, change: 0 },
            avgOrderValue: { value: 0, change: 0 },
            repeatRate: { value: 0, change: 0 }
        },
        topDishes: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadAnalytics();
    },
    onShow() {
        // 返回时刷新
        this.loadAnalytics();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    getDateRange() {
        const today = new Date();
        const endDate = this.formatDate(today);
        let startDate = endDate;
        switch (this.data.timeRange) {
            case 'WEEK':
                const weekAgo = new Date(today);
                weekAgo.setDate(weekAgo.getDate() - 7);
                startDate = this.formatDate(weekAgo);
                break;
            case 'MONTH':
                const monthAgo = new Date(today);
                monthAgo.setMonth(monthAgo.getMonth() - 1);
                startDate = this.formatDate(monthAgo);
                break;
            default:
                // TODAY
                break;
        }
        return { start_date: startDate, end_date: endDate };
    },
    formatDate(date) {
        const year = date.getFullYear();
        const month = ('0' + (date.getMonth() + 1)).slice(-2);
        const day = ('0' + date.getDate()).slice(-2);
        return `${year}-${month}-${day}`;
    },
    loadAnalytics() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const dateRange = this.getDateRange();
                // 并行加载概览和热门菜品
                const [overview, topDishes] = yield Promise.all([
                    merchant_analytics_1.MerchantStatsService.getStatsOverview(dateRange),
                    merchant_analytics_1.MerchantStatsService.getTopDishes(Object.assign(Object.assign({}, dateRange), { limit: 10 }))
                ]);
                // 更新指标
                this.setData({
                    metrics: {
                        gmv: {
                            value: Math.round((overview.total_revenue || 0) / 100),
                            change: Math.round((overview.growth_rate || 0) * 100)
                        },
                        orderCount: {
                            value: overview.total_orders || 0,
                            change: 0
                        },
                        avgOrderValue: {
                            value: Math.round((overview.avg_order_value || 0) / 100),
                            change: 0
                        },
                        repeatRate: {
                            value: Math.round((overview.completion_rate || 0) * 100),
                            change: 0
                        }
                    },
                    topDishes: (topDishes || []).map((dish) => ({
                        name: dish.dish_name,
                        sales: dish.sales_count,
                        revenue: dish.revenue
                    })),
                    loading: false
                });
            }
            catch (error) {
                console.error('加载分析数据失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onTimeRangeChange(e) {
        this.setData({ timeRange: e.detail.value });
        this.loadAnalytics();
    }
});
