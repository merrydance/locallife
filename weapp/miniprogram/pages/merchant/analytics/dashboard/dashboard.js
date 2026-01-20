"use strict";
/**
 * 商户BI分析仪表盘页面
 * 包含销售统计、财务分析、客户分析等
 * 使用TDesign组件库实现统一的UI风格
 */
Object.defineProperty(exports, "__esModule", { value: true });
const merchant_analytics_1 = require("@/api/merchant-analytics");
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        isLargeScreen: false,
        // 当前Tab
        currentTab: 'overview', // overview, sales, finance, customer
        // 日期范围
        dateRange: {
            start_date: '',
            end_date: ''
        },
        // 统计概览数据
        statsOverview: null,
        // 每日统计数据
        dailyStats: [],
        // 热门菜品数据
        topDishes: [],
        // 分类统计数据
        categoryStats: [],
        // 客户统计数据
        customerStats: [],
        customerPage: 1,
        customerPageSize: 20,
        // 复购率数据
        repurchaseStats: null,
        // 财务概览数据
        financeOverview: null,
        // 界面状态
        loading: true,
        refreshing: false,
        // 日期选择器
        showDatePicker: false,
        datePickerMode: 'start' // start, end
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.initPage();
    },
    onShow() {
        this.loadData();
    },
    /**
     * 初始化页面
     */
    async initPage() {
        // 设置默认日期范围（最近30天）
        const endDate = new Date();
        const startDate = new Date();
        startDate.setDate(startDate.getDate() - 30);
        this.setData({
            dateRange: {
                start_date: this.formatDate(startDate),
                end_date: this.formatDate(endDate)
            }
        });
        try {
            this.setData({ loading: true });
            await this.loadData();
        }
        catch (error) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        }
        finally {
            this.setData({ loading: false });
        }
    },
    /**
     * 加载数据
     */
    async loadData() {
        const { currentTab } = this.data;
        switch (currentTab) {
            case 'overview':
                await this.loadOverviewData();
                break;
            case 'sales':
                await this.loadSalesData();
                break;
            case 'finance':
                await this.loadFinanceData();
                break;
            case 'customer':
                await this.loadCustomerData();
                break;
        }
    },
    /**
     * 切换Tab
     */
    onTabChange(e) {
        const tab = e.detail.value;
        this.setData({ currentTab: tab });
        this.loadData();
    },
    // ==================== 概览数据 ====================
    /**
     * 加载概览数据
     */
    async loadOverviewData() {
        try {
            const { dateRange } = this.data;
            const [statsOverview, dailyStats] = await Promise.all([
                merchant_analytics_1.MerchantStatsService.getStatsOverview(dateRange),
                merchant_analytics_1.MerchantStatsService.getDailyStats(dateRange)
            ]);
            this.setData({
                statsOverview,
                dailyStats
            });
        }
        catch (error) {
            console.error('加载概览数据失败:', error);
            wx.showToast({
                title: '加载概览数据失败',
                icon: 'error'
            });
        }
    },
    // ==================== 销售数据 ====================
    /**
     * 加载销售数据
     */
    async loadSalesData() {
        try {
            const { dateRange } = this.data;
            const [topDishes, categoryStats] = await Promise.all([
                merchant_analytics_1.MerchantStatsService.getTopDishes({
                    ...dateRange,
                    limit: 10
                }),
                merchant_analytics_1.MerchantStatsService.getCategoryStats(dateRange)
            ]);
            this.setData({
                topDishes,
                categoryStats
            });
        }
        catch (error) {
            console.error('加载销售数据失败:', error);
            wx.showToast({
                title: '加载销售数据失败',
                icon: 'error'
            });
        }
    },
    // ==================== 财务数据 ====================
    /**
     * 加载财务数据
     */
    async loadFinanceData() {
        try {
            const { dateRange } = this.data;
            const financeOverview = await merchant_analytics_1.MerchantFinanceService.getFinanceOverview(dateRange);
            this.setData({ financeOverview });
        }
        catch (error) {
            console.error('加载财务数据失败:', error);
            wx.showToast({
                title: '加载财务数据失败',
                icon: 'error'
            });
        }
    },
    // ==================== 客户数据 ====================
    /**
     * 加载客户数据
     */
    async loadCustomerData() {
        try {
            const { dateRange, customerPage, customerPageSize } = this.data;
            const [customerStats, repurchaseStats] = await Promise.all([
                merchant_analytics_1.MerchantStatsService.getCustomerStats({
                    ...dateRange,
                    page_id: customerPage,
                    page_size: customerPageSize
                }),
                merchant_analytics_1.MerchantStatsService.getRepurchaseStats(dateRange)
            ]);
            this.setData({
                customerStats,
                repurchaseStats
            });
        }
        catch (error) {
            console.error('加载客户数据失败:', error);
            wx.showToast({
                title: '加载客户数据失败',
                icon: 'error'
            });
        }
    },
    // ==================== 日期选择 ====================
    /**
     * 显示日期选择器
     */
    showDatePickerModal(e) {
        const mode = e.currentTarget.dataset.mode;
        this.setData({
            showDatePicker: true,
            datePickerMode: mode
        });
    },
    /**
     * 日期选择确认
     */
    onDateConfirm(e) {
        const { value } = e.detail;
        const { datePickerMode, dateRange } = this.data;
        if (datePickerMode === 'start') {
            dateRange.start_date = value;
        }
        else {
            dateRange.end_date = value;
        }
        this.setData({
            dateRange,
            showDatePicker: false
        });
        // 重新加载数据
        this.loadData();
    },
    /**
     * 关闭日期选择器
     */
    closeDatePicker() {
        this.setData({ showDatePicker: false });
    },
    /**
     * 快速选择日期范围
     */
    quickSelectDate(e) {
        const { days } = e.currentTarget.dataset;
        const endDate = new Date();
        const startDate = new Date();
        startDate.setDate(startDate.getDate() - days);
        this.setData({
            dateRange: {
                start_date: this.formatDate(startDate),
                end_date: this.formatDate(endDate)
            }
        });
        this.loadData();
    },
    // ==================== 刷新数据 ====================
    /**
     * 下拉刷新
     */
    async onPullDownRefresh() {
        try {
            this.setData({ refreshing: true });
            await this.loadData();
            wx.showToast({
                title: '刷新成功',
                icon: 'success'
            });
        }
        catch (error) {
            wx.showToast({
                title: '刷新失败',
                icon: 'error'
            });
        }
        finally {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        }
    },
    // ==================== 工具方法 ====================
    /**
     * 格式化日期
     */
    formatDate(date) {
        const year = date.getFullYear();
        const month = ('0' + (date.getMonth() + 1)).slice(-2);
        const day = ('0' + date.getDate()).slice(-2);
        return `${year}-${month}-${day}`;
    },
    /**
     * 格式化金额
     */
    formatAmount(amount) {
        return merchant_analytics_1.AnalyticsAdapter.formatAmount(amount);
    },
    /**
     * 格式化百分比
     */
    formatPercentage(value) {
        return merchant_analytics_1.AnalyticsAdapter.formatPercentage(value);
    },
    /**
     * 格式化增长率
     */
    formatGrowthRate(rate) {
        return merchant_analytics_1.AnalyticsAdapter.formatGrowthRate(rate);
    },
    /**
     * 获取增长率颜色
     */
    getGrowthColor(rate) {
        return merchant_analytics_1.AnalyticsAdapter.getGrowthColor(rate);
    },
    /**
     * 返回工作台
     */
    onBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
            }
        });
    }
});
