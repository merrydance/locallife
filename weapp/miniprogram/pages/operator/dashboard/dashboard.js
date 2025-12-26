"use strict";
/**
 * 运营商工作台
 * 提供区域管理、商户管理、骑手管理、数据统计等功能入口
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
const operator_basic_management_1 = require("@/api/operator-basic-management");
const operator_merchant_management_1 = require("@/api/operator-merchant-management");
const operator_rider_management_1 = require("@/api/operator-rider-management");
const operator_analytics_1 = require("@/api/operator-analytics");
Page({
    data: {
        loading: true,
        refreshing: false,
        // 运营商信息
        operatorInfo: null,
        // 财务概览
        financeOverview: null,
        // 区域统计
        regionStats: [],
        selectedRegionId: 0,
        // 商户摘要
        merchantSummary: {
            total: 0,
            active: 0,
            suspended: 0,
            pending: 0
        },
        // 骑手摘要
        riderSummary: {
            total: 0,
            active: 0,
            online: 0,
            suspended: 0,
            pending: 0
        },
        // 申诉摘要
        appealSummary: {
            totalAppeals: 0,
            pendingAppeals: 0,
            avgResolutionTime: 0,
            satisfactionRate: 0
        },
        // 快捷入口
        quickActions: [
            { id: 'merchants', icon: 'shop', label: '商户管理', url: '/pages/operator/merchants/list/list' },
            { id: 'riders', icon: 'user', label: '骑手管理', url: '/pages/operator/riders/list/list' },
            { id: 'analytics', icon: 'chart', label: '数据分析', url: '/pages/operator/analytics/dashboard/dashboard' },
            { id: 'appeals', icon: 'service', label: '客诉处理', url: '/pages/operator/appeals/list/list' }
        ]
    },
    onLoad() {
        this.loadDashboardData();
    },
    onPullDownRefresh() {
        this.setData({ refreshing: true });
        this.loadDashboardData().finally(() => {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        });
    },
    /**
     * 加载工作台数据
     */
    loadDashboardData() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            try {
                this.setData({ loading: true });
                // 并行加载所有数据
                const [dashboardData, merchantData, riderData, analyticsData] = yield Promise.all([
                    (0, operator_basic_management_1.getOperatorDashboard)(),
                    (0, operator_merchant_management_1.getMerchantManagementDashboard)(),
                    (0, operator_rider_management_1.getRiderManagementDashboard)(),
                    (0, operator_analytics_1.getOperatorAnalyticsDashboard)()
                ]);
                this.setData({
                    operatorInfo: dashboardData.operatorInfo,
                    financeOverview: dashboardData.financeOverview,
                    regionStats: dashboardData.regionStats,
                    selectedRegionId: ((_a = dashboardData.regionStats[0]) === null || _a === void 0 ? void 0 : _a.id) || 0,
                    merchantSummary: merchantData.merchantSummary,
                    riderSummary: riderData.riderSummary,
                    appealSummary: analyticsData.appealSummary
                });
            }
            catch (error) {
                console.error('加载工作台数据失败:', error);
                wx.showToast({
                    title: '加载失败',
                    icon: 'none'
                });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 切换区域
     */
    onRegionChange(e) {
        const regionId = parseInt(e.detail.value);
        this.setData({ selectedRegionId: regionId });
        this.loadRegionData(regionId);
    },
    /**
     * 加载指定区域的数据
     */
    loadRegionData(regionId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                wx.showLoading({ title: '加载中...' });
                const [merchantData, riderData] = yield Promise.all([
                    (0, operator_merchant_management_1.getMerchantManagementDashboard)(regionId),
                    (0, operator_rider_management_1.getRiderManagementDashboard)(regionId)
                ]);
                this.setData({
                    merchantSummary: merchantData.merchantSummary,
                    riderSummary: riderData.riderSummary
                });
            }
            catch (error) {
                console.error('加载区域数据失败:', error);
                wx.showToast({
                    title: '加载失败',
                    icon: 'none'
                });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    /**
     * 快捷入口点击
     */
    onQuickActionTap(e) {
        const { url } = e.currentTarget.dataset;
        wx.navigateTo({ url });
    },
    /**
     * 查看财务详情
     */
    onFinanceDetailTap() {
        wx.navigateTo({
            url: '/pages/operator/finance/overview/overview'
        });
    },
    /**
     * 查看区域详情
     */
    onRegionDetailTap() {
        const { selectedRegionId } = this.data;
        if (selectedRegionId) {
            wx.navigateTo({
                url: `/pages/operator/regions/detail/detail?id=${selectedRegionId}`
            });
        }
    },
    /**
     * 格式化金额
     */
    formatAmount(amount) {
        return `¥${(amount / 100).toFixed(2)}`;
    },
    /**
     * 格式化百分比
     */
    formatPercentage(value) {
        return `${value.toFixed(1)}%`;
    }
});
