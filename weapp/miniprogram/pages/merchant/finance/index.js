"use strict";
/**
 * 财务管理页面
 * 集成全部 6 个后端财务 API，提供全面的财务分析
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
// 财务服务
const FinanceService = {
    getOverview(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/finance/overview', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getOrders(startDate_1, endDate_1) {
        return __awaiter(this, arguments, void 0, function* (startDate, endDate, page = 1, limit = 20) {
            return (0, request_1.request)({ url: '/v1/merchant/finance/orders', method: 'GET', data: { start_date: startDate, end_date: endDate, page, limit } });
        });
    },
    getServiceFees(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/finance/service-fees', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getPromotions(startDate_1, endDate_1) {
        return __awaiter(this, arguments, void 0, function* (startDate, endDate, page = 1, limit = 20) {
            return (0, request_1.request)({ url: '/v1/merchant/finance/promotions', method: 'GET', data: { start_date: startDate, end_date: endDate, page, limit } });
        });
    },
    getDailyFinance(startDate, endDate) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({ url: '/v1/merchant/finance/daily', method: 'GET', data: { start_date: startDate, end_date: endDate } });
        });
    },
    getSettlements(startDate_1, endDate_1, status_1) {
        return __awaiter(this, arguments, void 0, function* (startDate, endDate, status, page = 1, limit = 20) {
            const data = { start_date: startDate, end_date: endDate, page, limit };
            if (status)
                data.status = status;
            return (0, request_1.request)({ url: '/v1/merchant/finance/settlements', method: 'GET', data });
        });
    }
};
Page({
    data: {
        sidebarCollapsed: false,
        loading: true,
        // 日期范围
        dateRange: 'month',
        startDate: '',
        endDate: '',
        // Tab
        activeTab: 'overview',
        // 数据
        overview: null,
        dailyFinance: [],
        orders: [],
        serviceFees: [],
        promotions: [],
        settlements: []
    },
    onLoad() {
        this.setDateRange('month');
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // 设置日期范围
    setDateRange(range) {
        const today = new Date();
        const endDate = this.formatDate(today);
        let startDate;
        if (range === 'week') {
            const weekAgo = new Date(today);
            weekAgo.setDate(weekAgo.getDate() - 6);
            startDate = this.formatDate(weekAgo);
        }
        else {
            const monthAgo = new Date(today);
            monthAgo.setDate(monthAgo.getDate() - 29);
            startDate = this.formatDate(monthAgo);
        }
        this.setData({ dateRange: range, startDate, endDate });
        this.loadData();
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
    // 切换 Tab
    onTabChange(e) {
        const tab = e.currentTarget.dataset.tab;
        this.setData({ activeTab: tab });
        this.loadTabData(tab);
    },
    // 加载当前 Tab 数据
    loadTabData(tab) {
        return __awaiter(this, void 0, void 0, function* () {
            const { startDate, endDate } = this.data;
            this.setData({ loading: true });
            try {
                switch (tab) {
                    case 'overview':
                        const overview = yield FinanceService.getOverview(startDate, endDate);
                        this.setData({ overview, loading: false });
                        break;
                    case 'daily':
                        const dailyFinance = yield FinanceService.getDailyFinance(startDate, endDate);
                        this.setData({ dailyFinance: dailyFinance || [], loading: false });
                        break;
                    case 'orders':
                        const orders = yield FinanceService.getOrders(startDate, endDate);
                        this.setData({ orders: orders || [], loading: false });
                        break;
                    case 'fees':
                        const serviceFees = yield FinanceService.getServiceFees(startDate, endDate);
                        this.setData({ serviceFees: serviceFees || [], loading: false });
                        break;
                    case 'promotions':
                        const promotions = yield FinanceService.getPromotions(startDate, endDate);
                        this.setData({ promotions: promotions || [], loading: false });
                        break;
                    case 'settlements':
                        const settlements = yield FinanceService.getSettlements(startDate, endDate);
                        this.setData({ settlements: settlements || [], loading: false });
                        break;
                }
            }
            catch (error) {
                console.error('加载财务数据失败:', error);
                wx.showToast({ title: '加载失败', icon: 'none' });
                this.setData({ loading: false });
            }
        });
    },
    // 加载初始数据
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.loadTabData(this.data.activeTab);
        });
    },
    // 格式化状态
    formatStatus(status) {
        const map = {
            'pending': '待处理',
            'processing': '处理中',
            'finished': '已完成',
            'failed': '失败'
        };
        return map[status] || status;
    }
});
