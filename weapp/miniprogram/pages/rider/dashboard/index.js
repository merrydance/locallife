"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const rider_1 = require("../../../api/rider");
const logger_1 = require("../../../utils/logger");
const responsive_1 = require("../../../utils/responsive");
const app = getApp();
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        isOnline: false,
        hasActiveTask: false,
        currentTask: null,
        riderId: '',
        loading: false
    },
    pollTimer: null,
    onLoad() {
        const userInfo = app.globalData.userInfo;
        this.setData({ riderId: (userInfo === null || userInfo === void 0 ? void 0 : userInfo.id) ? String(userInfo.id) : '' });
        this.loadDashboard();
    },
    onShow() {
        this.startPolling();
    },
    onHide() {
        this.stopPolling();
    },
    onUnload() {
        this.stopPolling();
    },
    async onToggleOnline(e) {
        const isOnline = e.detail.value;
        this.setData({ isOnline });
        try {
            if (isOnline) {
                await (0, rider_1.setRiderOnline)();
                this.loadDashboard();
            }
            else {
                await (0, rider_1.setRiderOffline)();
                this.setData({ hasActiveTask: false, currentTask: null });
            }
        }
        catch (error) {
            this.setData({ isOnline: !isOnline });
            wx.showToast({ title: '操作失败', icon: 'none' });
        }
    },
    async loadDashboard() {
        try {
            const dashboard = await (0, rider_1.getRiderDashboard)();
            this.setData({ isOnline: !!dashboard.rider_id }); // Simple check for online
            const activeTasks = dashboard.active_tasks || [];
            if (activeTasks.length > 0) {
                this.setData({
                    hasActiveTask: true,
                    currentTask: this.mapTask(activeTasks[0])
                });
            }
            else {
                this.setData({ hasActiveTask: false, currentTask: null });
            }
        }
        catch (error) {
            console.error('Load dashboard failed', error);
        }
        finally {
            this.setData({ loading: false });
        }
    },
    startPolling() {
        this.stopPolling();
        this.pollTimer = setInterval(() => {
            this.loadDashboard();
        }, 5000);
    },
    stopPolling() {
        if (this.pollTimer) {
            clearInterval(this.pollTimer);
            this.pollTimer = null;
        }
    },
    mapTask(dto) {
        let status = 0;
        if (dto.status === 'ACCEPTED' || dto.status === 'CONFIRMED')
            status = 1;
        else if (dto.status === 'DELIVERING')
            status = 2;
        else if (dto.status === 'COMPLETED')
            status = 3;
        else
            status = 0;
        const dist = status === 2 ? dto.distance_to_deliver : dto.distance_to_shop;
        return {
            id: dto.id,
            shopName: dto.merchant_name,
            shopAddress: dto.merchant_address,
            customerAddress: dto.customer_address,
            distance: dist ? `${(dist / 1000).toFixed(1)}km` : '未知',
            income: `¥${(dto.fee / 100).toFixed(2)}`,
            status
        };
    },
    async onTaskAction(e) {
        const { action } = e.detail;
        if (!this.data.currentTask)
            return;
        const orderId = this.data.currentTask.id;
        wx.showLoading({ title: '处理中' });
        try {
            if (action === 'accepted') {
                await (0, rider_1.acceptOrder)(orderId);
                wx.showToast({ title: '接单成功', icon: 'success' });
                this.loadDashboard();
            }
            else if (action === 'picked_up') {
                await (0, rider_1.pickupOrder)(orderId);
                wx.showToast({ title: '已确认取货', icon: 'success' });
                this.loadDashboard();
            }
            else if (action === 'delivered') {
                await (0, rider_1.deliverOrder)(orderId);
                wx.showToast({ title: '配送完成', icon: 'success' });
                this.setData({
                    hasActiveTask: false,
                    currentTask: null
                });
            }
        }
        catch (error) {
            logger_1.logger.error('Action failed', error, 'Dashboard');
            wx.showToast({ title: '操作失败', icon: 'none' });
        }
        finally {
            wx.hideLoading();
        }
    }
});
