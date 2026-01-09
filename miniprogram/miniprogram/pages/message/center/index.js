"use strict";
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
const notification_1 = require("../../../api/notification");
Page({
    data: {
        notifications: [],
        activeTab: 0, // 0: All, 1: System, 2: Order
        loading: false,
        page: 1,
        hasMore: true
    },
    onLoad() {
        this.loadNotifications(true);
    },
    onPullDownRefresh() {
        this.loadNotifications(true);
    },
    onReachBottom() {
        if (this.data.hasMore) {
            this.loadNotifications(false);
        }
    },
    loadNotifications() {
        return __awaiter(this, arguments, void 0, function* (refresh = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            const page = refresh ? 1 : this.data.page + 1;
            const typeMap = ['all', 'system', 'order'];
            const type = typeMap[this.data.activeTab] === 'all' ? undefined : typeMap[this.data.activeTab];
            try {
                const res = yield notification_1.notificationService.getNotifications({ page, page_size: 20, type });
                // Mock data if API fails or returns empty in dev
                const list = res.list || (refresh ? this.getMockData() : []);
                this.setData({
                    notifications: refresh ? list : [...this.data.notifications, ...list],
                    page,
                    hasMore: list.length === 20,
                    loading: false
                });
                wx.stopPullDownRefresh();
            }
            catch (error) {
                console.error(error);
                // Fallback mock
                const list = refresh ? this.getMockData() : [];
                this.setData({
                    notifications: refresh ? list : [...this.data.notifications, ...list],
                    loading: false
                });
                wx.stopPullDownRefresh();
            }
        });
    },
    getMockData() {
        return [
            { id: 1, type: 'system', title: '系统维护通知', content: '系统将于今晚24:00进行维护升级，预计2小时。', is_read: false, created_at: '2023-10-27 10:00' },
            { id: 2, type: 'order', title: '订单已送达', content: '您的订单ORD-8821已成功送达，祝您用餐愉快！', is_read: true, created_at: '2023-10-26 12:30' },
            { id: 3, type: 'promotion', title: '周末大促', content: '全场满30减10，快来看看吧！', is_read: false, created_at: '2023-10-25 09:00' }
        ];
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadNotifications(true);
    },
    onItemClick(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, index } = e.currentTarget.dataset;
            const notification = this.data.notifications[index];
            if (!notification.is_read) {
                try {
                    yield notification_1.notificationService.markAsRead(id);
                    // Update local state
                    this.setData({
                        [`notifications[${index}].is_read`]: true
                    });
                }
                catch (e) {
                    console.error(e);
                }
            }
            // Navigation logic based on type or action_url
            if (notification.type === 'order') {
                // wx.navigateTo({ url: '/pages/order/detail/index?id=...' });
            }
        });
    },
    onMarkAllRead() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                yield notification_1.notificationService.markAllAsRead();
                const updated = this.data.notifications.map(n => (Object.assign(Object.assign({}, n), { is_read: true })));
                this.setData({ notifications: updated });
                wx.showToast({ title: '已全部已读', icon: 'success' });
            }
            catch (e) {
                console.error(e);
            }
        });
    }
});
