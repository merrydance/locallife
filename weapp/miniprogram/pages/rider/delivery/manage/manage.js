"use strict";
/**
 * 骑手配送管理页面
 * 包含配送任务列表、收入统计、异常处理等
 * 使用TDesign组件库实现统一的UI风格
 */
Object.defineProperty(exports, "__esModule", { value: true });
const rider_delivery_1 = require("@/api/rider-delivery");
Page({
    data: {
        // 当前Tab
        currentTab: 'available', // available, active, history
        // 骑手状态
        riderStatus: null,
        isOnline: false,
        // 可接任务列表
        availableTasks: [],
        // 当前配送任务
        activeTasks: [],
        // 历史任务
        historyTasks: [],
        historyPage: 1,
        historyPageSize: 20,
        historyHasMore: true,
        // 界面状态
        loading: true,
        refreshing: false,
        // 异常上报弹窗
        showExceptionModal: false,
        selectedTask: null,
        exceptionForm: {
            exception_type: '',
            description: ''
        },
        // 延迟上报弹窗
        showDelayModal: false,
        delayForm: {
            delay_reason: '',
            estimated_delay: 0
        },
        // 位置上报定时器
        locationTimer: null
    },
    onLoad() {
        this.initPage();
    },
    onShow() {
        this.loadData();
        this.startLocationReporting();
    },
    onHide() {
        this.stopLocationReporting();
    },
    onUnload() {
        this.stopLocationReporting();
    },
    /**
     * 初始化页面
     */
    async initPage() {
        try {
            this.setData({ loading: true });
            await Promise.all([
                this.loadRiderStatus(),
                this.loadData()
            ]);
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
            case 'available':
                await this.loadAvailableTasks();
                break;
            case 'active':
                await this.loadActiveTasks();
                break;
            case 'history':
                await this.loadHistoryTasks();
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
    // ==================== 骑手状态管理 ====================
    /**
     * 加载骑手状态
     */
    async loadRiderStatus() {
        try {
            const status = await rider_delivery_1.RiderInfoService.getRiderStatus();
            this.setData({
                riderStatus: status,
                isOnline: status.status === 'online'
            });
        }
        catch (error) {
            console.error('加载骑手状态失败:', error);
        }
    },
    /**
     * 切换在线状态
     */
    async toggleOnlineStatus() {
        const { isOnline } = this.data;
        try {
            wx.showLoading({ title: isOnline ? '下线中...' : '上线中...' });
            if (isOnline) {
                await rider_delivery_1.RiderInfoService.goOffline();
            }
            else {
                await rider_delivery_1.RiderInfoService.goOnline();
            }
            await this.loadRiderStatus();
            wx.showToast({
                title: isOnline ? '已下线' : '已上线',
                icon: 'success'
            });
        }
        catch (error) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    // ==================== 配送任务管理 ====================
    /**
     * 加载可接任务
     */
    async loadAvailableTasks() {
        try {
            const result = await rider_delivery_1.DeliveryTaskService.getRecommendedTasks();
            this.setData({ availableTasks: result.tasks });
        }
        catch (error) {
            console.error('加载可接任务失败:', error);
            wx.showToast({
                title: '加载任务失败',
                icon: 'error'
            });
        }
    },
    /**
     * 加载当前任务
     */
    async loadActiveTasks() {
        try {
            const tasks = await rider_delivery_1.DeliveryTaskService.getActiveTasks();
            this.setData({ activeTasks: tasks });
        }
        catch (error) {
            console.error('加载当前任务失败:', error);
            wx.showToast({
                title: '加载任务失败',
                icon: 'error'
            });
        }
    },
    /**
     * 加载历史任务
     */
    async loadHistoryTasks(reset = true) {
        try {
            const { historyPage, historyPageSize } = this.data;
            if (reset) {
                this.setData({ historyPage: 1, historyTasks: [], historyHasMore: true });
            }
            const result = await rider_delivery_1.DeliveryTaskService.getDeliveryHistory({
                page_id: reset ? 1 : historyPage,
                page_size: historyPageSize
            });
            const newTasks = reset ? result.deliveries : [...this.data.historyTasks, ...result.deliveries];
            this.setData({
                historyTasks: newTasks,
                historyHasMore: result.deliveries.length === historyPageSize,
                historyPage: reset ? 2 : historyPage + 1
            });
        }
        catch (error) {
            console.error('加载历史任务失败:', error);
            wx.showToast({
                title: '加载历史失败',
                icon: 'error'
            });
        }
    },
    /**
     * 抢单
     */
    async grabOrder(e) {
        const orderId = e.detail.id || e.currentTarget.dataset.id;
        try {
            wx.showLoading({ title: '抢单中...' });
            await rider_delivery_1.DeliveryTaskService.grabOrder(orderId);
            wx.showToast({
                title: '抢单成功',
                icon: 'success'
            });
            // 切换到当前任务Tab
            this.setData({ currentTab: 'active' });
            await this.loadActiveTasks();
        }
        catch (error) {
            wx.showToast({
                title: error.message || '抢单失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    /**
     * 开始取餐
     */
    async startPickup(e) {
        const deliveryId = e.detail.id || e.currentTarget.dataset.id;
        try {
            wx.showLoading({ title: '处理中...' });
            await rider_delivery_1.DeliveryTaskService.startPickup(deliveryId);
            wx.showToast({
                title: '已开始取餐',
                icon: 'success'
            });
            await this.loadActiveTasks();
        }
        catch (error) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    /**
     * 确认取餐
     */
    async confirmPickup(e) {
        const deliveryId = e.detail.id || e.currentTarget.dataset.id;
        try {
            wx.showLoading({ title: '处理中...' });
            await rider_delivery_1.DeliveryTaskService.confirmPickup(deliveryId);
            wx.showToast({
                title: '已确认取餐',
                icon: 'success'
            });
            await this.loadActiveTasks();
        }
        catch (error) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    /**
     * 开始配送
     */
    async startDelivery(e) {
        const deliveryId = e.detail.id || e.currentTarget.dataset.id;
        try {
            wx.showLoading({ title: '处理中...' });
            await rider_delivery_1.DeliveryTaskService.startDelivery(deliveryId);
            wx.showToast({
                title: '已开始配送',
                icon: 'success'
            });
            await this.loadActiveTasks();
        }
        catch (error) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    /**
     * 确认送达
     */
    async confirmDelivery(e) {
        const deliveryId = e.detail.id || e.currentTarget.dataset.id;
        wx.showModal({
            title: '确认送达',
            content: '确定已将订单送达顾客手中？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        wx.showLoading({ title: '处理中...' });
                        await rider_delivery_1.DeliveryTaskService.confirmDelivery(deliveryId);
                        wx.showToast({
                            title: '配送完成',
                            icon: 'success'
                        });
                        await this.loadActiveTasks();
                        await this.loadRiderStatus();
                    }
                    catch (error) {
                        wx.showToast({
                            title: error.message || '操作失败',
                            icon: 'error'
                        });
                    }
                    finally {
                        wx.hideLoading();
                    }
                }
            }
        });
    },
    // ==================== 异常处理 ====================
    /**
     * 显示异常上报弹窗
     */
    showExceptionDialog(e) {
        const taskId = e.detail.id || e.currentTarget.dataset.id;
        const task = this.data.activeTasks.find(t => t.delivery_id === taskId);
        this.setData({
            showExceptionModal: true,
            selectedTask: task || null,
            exceptionForm: {
                exception_type: '',
                description: ''
            }
        });
    },
    /**
     * 关闭异常上报弹窗
     */
    closeExceptionModal() {
        this.setData({ showExceptionModal: false });
    },
    /**
     * 上报异常
     */
    async reportException() {
        const { selectedTask, exceptionForm } = this.data;
        if (!selectedTask)
            return;
        if (!exceptionForm.exception_type || !exceptionForm.description) {
            wx.showToast({
                title: '请填写完整信息',
                icon: 'error'
            });
            return;
        }
        try {
            wx.showLoading({ title: '上报中...' });
            await rider_delivery_1.ExceptionHandlingService.reportException(selectedTask.order_id, exceptionForm);
            wx.showToast({
                title: '上报成功',
                icon: 'success'
            });
            this.closeExceptionModal();
        }
        catch (error) {
            wx.showToast({
                title: error.message || '上报失败',
                icon: 'error'
            });
        }
        finally {
            wx.hideLoading();
        }
    },
    // ==================== 位置上报 ====================
    /**
     * 开始位置上报
     */
    startLocationReporting() {
        if (!this.data.isOnline)
            return;
        this.stopLocationReporting();
        const timer = setInterval(() => {
            this.reportLocation();
        }, 30000); // 每30秒上报一次
        this.setData({ locationTimer: timer });
    },
    /**
     * 停止位置上报
     */
    stopLocationReporting() {
        if (this.data.locationTimer) {
            clearInterval(this.data.locationTimer);
            this.setData({ locationTimer: null });
        }
    },
    /**
     * 上报位置
     */
    async reportLocation() {
        try {
            const location = await this.getCurrentLocation();
            await rider_delivery_1.RiderInfoService.reportLocation(location);
        }
        catch (error) {
            console.error('位置上报失败:', error);
        }
    },
    /**
     * 获取当前位置
     */
    getCurrentLocation() {
        return new Promise((resolve, reject) => {
            wx.getLocation({
                type: 'gcj02',
                success: (res) => {
                    resolve({
                        latitude: res.latitude,
                        longitude: res.longitude
                    });
                },
                fail: reject
            });
        });
    },
    // ==================== 工具方法 ====================
    /**
     * 格式化金额
     */
    formatAmount(amount) {
        return rider_delivery_1.DeliveryAdapter.formatAmount(amount);
    },
    /**
     * 格式化距离
     */
    formatDistance(distance) {
        return rider_delivery_1.DeliveryAdapter.formatDistance(distance);
    },
    /**
     * 格式化配送状态
     */
    formatDeliveryStatus(status) {
        return rider_delivery_1.DeliveryAdapter.formatDeliveryStatus(status);
    },
    /**
     * 获取状态颜色
     */
    getStatusColor(status) {
        return rider_delivery_1.DeliveryAdapter.getStatusColor(status);
    },
    /**
     * 格式化骑手状态
     */
    formatRiderStatus(status) {
        return rider_delivery_1.DeliveryAdapter.formatRiderStatus(status);
    },
    /**
     * 获取骑手状态颜色
     */
    getRiderStatusColor(status) {
        return rider_delivery_1.DeliveryAdapter.getRiderStatusColor(status);
    },
    /**
     * 计算预计送达时间
     */
    calculateEstimatedArrival(createdAt, estimatedTime) {
        return rider_delivery_1.DeliveryAdapter.calculateEstimatedArrival(createdAt, estimatedTime);
    }
});
