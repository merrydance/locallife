/**
 * KDS后厨显示系统页面
 * 基于重构后的API接口实现后厨订单管理功能
 * 适配大屏显示，支持自动刷新、语音播报、订单超时预警
 */

import {
    KitchenDisplayService,
    OrderManagementAdapter,
    type KitchenOrdersResponse,
    type KitchenOrderResponse,
    type KitchenStats
} from '@/api/order-management';
import { responsiveBehavior } from '@/utils/responsive';

Page({
    behaviors: [responsiveBehavior],
    data: {
        // 订单数据
        newOrders: [] as KitchenOrderResponse[],
        preparingOrders: [] as KitchenOrderResponse[],
        readyOrders: [] as KitchenOrderResponse[],

        // 统计数据
        stats: {
            total_pending: 0,
            avg_preparation_time: 0,
            orders_behind_schedule: 0,
            avg_prepare_time: 0,
            completed_today_count: 0,
            new_count: 0,
            preparing_count: 0,
            ready_count: 0
        } as KitchenStats,

        // 界面状态
        loading: true,
        autoRefresh: true,
        refreshInterval: 10000, // 10秒自动刷新

        // 选中的订单
        selectedOrder: null as KitchenOrderResponse | null,
        showDetailModal: false,

        // 语音播报
        voiceEnabled: true,

        // 定时器
        refreshTimer: null as any,

        // SaaS 布局相关
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true
    },

    onLoad() {
        // Layout data injected by responsiveBehavior
        this.initPage();
    },

    onShow() {
        this.startAutoRefresh();
    },

    onHide() {
        this.stopAutoRefresh();
    },

    onUnload() {
        this.stopAutoRefresh();
    },

    /**
     * 初始化页面
     */
    async initPage() {
        try {
            this.setData({ loading: true });
            await this.loadKitchenOrders();
        } catch (error: any) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            this.setData({ loading: false });
        }
    },

    /**
     * 加载后厨订单
     */
    async loadKitchenOrders() {
        try {
            const result = await KitchenDisplayService.getKitchenOrders();

            // 1. 新订单：按创建时间正序 (FIFO)
            const newOrders = (result.new_orders || []).sort((a, b) =>
                new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
            );

            // 2. 制作中：按剩余时间正序 (紧急的在前)
            const preparingOrders = (result.preparing_orders || []).sort((a, b) => {
                const remainingA = OrderManagementAdapter.getRemainingTime(a);
                const remainingB = OrderManagementAdapter.getRemainingTime(b);
                return remainingA - remainingB;
            });

            // 3. 待取餐：按完成时间倒序 (最新完成的在前)
            const readyOrders = (result.ready_orders || []).sort((a, b) =>
                new Date(b.ready_at!).getTime() - new Date(a.ready_at!).getTime()
            );

            this.setData({
                newOrders,
                preparingOrders,
                readyOrders,
                stats: result.stats
            });

            // 检查新订单并播报
            this.checkNewOrders(newOrders);

            // 检查超时订单
            this.checkOverdueOrders(preparingOrders);

        } catch (error: any) {
            console.error('加载订单失败:', error);
            throw error;
        }
    },

    /**
     * 开始自动刷新
     */
    startAutoRefresh() {
        if (!this.data.autoRefresh) return;

        this.stopAutoRefresh();

        const timer = setInterval(() => {
            this.loadKitchenOrders();
        }, this.data.refreshInterval);

        this.setData({ refreshTimer: timer });
    },

    /**
     * 停止自动刷新
     */
    stopAutoRefresh() {
        if (this.data.refreshTimer) {
            clearInterval(this.data.refreshTimer);
            this.setData({ refreshTimer: null });
        }
    },

    /**
     * 切换自动刷新
     */
    toggleAutoRefresh() {
        const autoRefresh = !this.data.autoRefresh;
        this.setData({ autoRefresh });

        if (autoRefresh) {
            this.startAutoRefresh();
        } else {
            this.stopAutoRefresh();
        }

        wx.showToast({
            title: autoRefresh ? '已开启自动刷新' : '已关闭自动刷新',
            icon: 'success'
        });
    },

    /**
     * 手动刷新
     */
    async refreshOrders() {
        try {
            wx.showLoading({ title: '刷新中...' });
            await this.loadKitchenOrders();
            wx.showToast({
                title: '刷新成功',
                icon: 'success'
            });
        } catch (error: any) {
            wx.showToast({
                title: '刷新失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 查看订单详情
     */
    async viewOrderDetail(e: any) {
        const orderId = e.currentTarget.dataset.id;

        try {
            wx.showLoading({ title: '加载中...' });

            const order = await KitchenDisplayService.getKitchenOrderDetail(orderId);

            this.setData({
                showDetailModal: true,
                selectedOrder: order
            });

        } catch (error: any) {
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 关闭订单详情弹窗
     */
    closeDetailModal() {
        this.setData({
            showDetailModal: false,
            selectedOrder: null
        });
    },

    /**
     * 开始制作订单
     */
    async startPreparing(e: any) {
        const orderId = e.currentTarget.dataset.id;

        try {
            wx.showLoading({ title: '处理中...' });

            await KitchenDisplayService.startPreparing(orderId);

            wx.showToast({
                title: '已开始制作',
                icon: 'success'
            });

            await this.loadKitchenOrders();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 标记订单制作完成
     */
    async markOrderReady(e: any) {
        const orderId = e.currentTarget.dataset.id;

        try {
            wx.showLoading({ title: '处理中...' });

            await KitchenDisplayService.markKitchenOrderReady(orderId);

            wx.showToast({
                title: '制作完成',
                icon: 'success'
            });

            // 播报完成提示
            this.playVoice('订单制作完成，请通知配送');

            await this.loadKitchenOrders();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 检查新订单
     */
    checkNewOrders(newOrders: KitchenOrderResponse[]) {
        const previousCount = this.data.newOrders.length;
        const currentCount = newOrders.length;

        if (currentCount > previousCount) {
            const newCount = currentCount - previousCount;
            this.playVoice(`您有${newCount}个新订单，请及时处理`);
        }
    },

    /**
     * 检查超时订单
     */
    checkOverdueOrders(preparingOrders: KitchenOrderResponse[]) {
        const overdueOrders = preparingOrders.filter(order =>
            OrderManagementAdapter.isOrderOverdue(order)
        );

        if (overdueOrders.length > 0) {
            this.playVoice(`有${overdueOrders.length}个订单已超时，请加快制作`);
        }
    },

    /**
     * 播放语音提示
     */
    playVoice(text: string) {
        if (!this.data.voiceEnabled) return;

        // 1. 播放提示音
        const innerAudioContext = wx.createInnerAudioContext();
        innerAudioContext.autoplay = true;
        innerAudioContext.src = '/assets/audio/new_order.mp3'; // 默认提示音
        innerAudioContext.onEnded(() => {
            innerAudioContext.destroy();
        });
        innerAudioContext.onError((res) => {
            console.warn('Audio play error', res);
            innerAudioContext.destroy();
        });

        // 2. 显示文本提示 (模拟语音内容)
        wx.showToast({
            title: text,
            icon: 'none',
            duration: 3000
        });

        // Todo: 如果有 TTS 服务，这里调用 Text-to-Speech API
        console.log('语音播报:', text);
    },

    /**
     * 切换语音播报
     */
    toggleVoice() {
        const voiceEnabled = !this.data.voiceEnabled;
        this.setData({ voiceEnabled });

        wx.showToast({
            title: voiceEnabled ? '已开启语音播报' : '已关闭语音播报',
            icon: 'success'
        });
    },

    /**
     * 获取订单剩余时间
     */
    getRemainingTime(order: KitchenOrderResponse): string {
        const remaining = OrderManagementAdapter.getRemainingTime(order);

        if (remaining <= 0) {
            return '已超时';
        }

        return `${Math.ceil(remaining)}分钟`;
    },

    /**
     * 判断订单是否超时
     */
    isOrderOverdue(order: KitchenOrderResponse): boolean {
        return OrderManagementAdapter.isOrderOverdue(order);
    },

    /**
     * 格式化时间
     */
    formatTime(dateString: string): string {
        const date = new Date(dateString);
        const hours = ('0' + date.getHours()).slice(-2);
        const minutes = ('0' + date.getMinutes()).slice(-2);
        return `${hours}:${minutes}`;
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
    },

    /**
     * 侧边栏折叠
     */
    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },

    /**
     * 进入简化模式（中等屏幕使用）
     */
    enterSimpleMode() {
        // 直接进入简化版KDS显示
        wx.showToast({ title: '简化模式开发中', icon: 'none' });
    }
});
