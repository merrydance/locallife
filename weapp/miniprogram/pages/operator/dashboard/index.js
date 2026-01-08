"use strict";
/**
 * 运营仪表盘页面
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
const responsive_1 = require("@/utils/responsive");
const util_1 = require("@/utils/util");
const platform_dashboard_1 = require("../../../api/platform-dashboard");
const operator_merchant_management_1 = require("../../../api/operator-merchant-management");
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        stats: {
            total_gmv: 0,
            total_gmv_display: '0.00',
            total_orders: 0,
            active_merchants: 0,
            active_riders: 0
        },
        pending_approvals: [],
        loading: false
    },
    onLoad() {
        // Layout data is automatically injected by responsiveBehavior
        this.loadDashboardData();
    },
    onShow() {
        // 返回时刷新
        this.loadDashboardData();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadDashboardData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // 获取实时大盘数据
                const realtimeData = yield platform_dashboard_1.platformDashboardService.getRealtimeDashboard();
                this.setData({
                    stats: {
                        total_gmv: realtimeData.gmv_24h || 0,
                        total_gmv_display: (0, util_1.formatPriceNoSymbol)(realtimeData.gmv_24h || 0),
                        total_orders: realtimeData.orders_24h || 0,
                        active_merchants: realtimeData.active_merchants_24h || 0,
                        active_riders: 0 // 后端暂无骑手数据
                    },
                    loading: false
                });
                // 加载待审批列表
                yield this.loadPendingApprovals();
            }
            catch (error) {
                console.error('加载仪表盘数据失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    loadPendingApprovals() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 获取待审批商户列表
                const merchantList = yield operator_merchant_management_1.operatorMerchantManagementService.getMerchantList({
                    page_id: 1,
                    page_size: 10
                });
                // 筛选待审批的商户
                const pendingMerchants = (merchantList.merchants || []).filter(m => m.status === 'pending_approval');
                const approvals = pendingMerchants.map(m => ({
                    id: m.id,
                    type: 'MERCHANT_JOIN',
                    name: `商户入驻申请 - ${m.name || '未知商户'}`,
                    created_at: m.created_at
                }));
                this.setData({ pending_approvals: approvals });
            }
            catch (error) {
                console.warn('加载待审批列表失败:', error);
            }
        });
    },
    onApprove(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            try {
                yield operator_merchant_management_1.operatorMerchantManagementService.resumeMerchant(id, { reason: '审批通过' });
                wx.showToast({ title: '已通过', icon: 'success' });
                const newList = this.data.pending_approvals.filter((i) => i.id !== id);
                this.setData({ pending_approvals: newList });
            }
            catch (error) {
                console.error('审批失败:', error);
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    onReject(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            try {
                yield operator_merchant_management_1.operatorMerchantManagementService.suspendMerchant(id, { reason: '审批拒绝' });
                wx.showToast({ title: '已拒绝', icon: 'none' });
                const newList = this.data.pending_approvals.filter((i) => i.id !== id);
                this.setData({ pending_approvals: newList });
            }
            catch (error) {
                console.error('拒绝失败:', error);
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    }
});
