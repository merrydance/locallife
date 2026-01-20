import { notificationService, Notification } from '../../../api/notification';

Page({
    data: {
        notifications: [] as Notification[],
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

    async loadNotifications(refresh: boolean = false) {
        if (this.data.loading) return;
        this.setData({ loading: true });

        const page = refresh ? 1 : this.data.page + 1;
        const typeMap: Array<'all' | 'system' | 'order'> = ['all', 'system', 'order'];
        const tabType = typeMap[this.data.activeTab] || 'all';
        const type = tabType === 'all' ? undefined : tabType;
        const pageSize = 20;

        try {
            const res = await notificationService.getNotifications({ page_id: page, page_size: pageSize, type });
            const list = res.notifications || (refresh ? this.getMockData() : []);
            const totalCount = typeof res.total_count === 'number' ? res.total_count : list.length;

            this.setData({
                notifications: refresh ? list : [...this.data.notifications, ...list],
                page,
                hasMore: page * pageSize < totalCount,
                loading: false
            });
            wx.stopPullDownRefresh();
        } catch (error) {
            console.error(error);
            // Fallback mock
            const list = refresh ? this.getMockData() : [];
            this.setData({
                notifications: refresh ? list : [...this.data.notifications, ...list],
                loading: false
            });
            wx.stopPullDownRefresh();
        }
    },

    getMockData() {
        return [
            { id: 1, user_id: 0, type: 'system', title: '系统维护通知', content: '系统将于今晚24:00进行维护升级，预计2小时。', is_read: false, is_pushed: false, created_at: '2023-10-27 10:00' },
            { id: 2, user_id: 0, type: 'order', title: '订单已送达', content: '您的订单ORD-8821已成功送达，祝您用餐愉快！', is_read: true, is_pushed: true, created_at: '2023-10-26 12:30' },
            { id: 3, user_id: 0, type: 'system', title: '周末大促', content: '全场满30减10，快来看看吧！', is_read: false, is_pushed: false, created_at: '2023-10-25 09:00' }
        ] as Notification[];
    },

    onTabChange(e: WechatMiniprogram.CustomEvent) {
        this.setData({ activeTab: e.detail.value });
        this.loadNotifications(true);
    },

    async onItemClick(e: WechatMiniprogram.CustomEvent) {
        const { id, index } = e.currentTarget.dataset as { id?: number; index?: number };
        if (index === undefined) return;
        const notification = this.data.notifications[index];
        const notificationId = id ?? notification?.id;

        if (!notification.is_read) {
            try {
                if (notificationId) {
                    await notificationService.markAsRead(notificationId);
                }
                // Update local state
                this.setData({
                    [`notifications[${index}].is_read`]: true
                });
            } catch (e) { console.error(e); }
        }

        // Navigation logic based on type or action_url
        if (notification.type === 'order') {
            // wx.navigateTo({ url: '/pages/order/detail/index?id=...' });
        }
    },

    async onMarkAllRead() {
        try {
            await notificationService.markAllAsRead();
            const updated = this.data.notifications.map(n => ({ ...n, is_read: true }));
            this.setData({ notifications: updated });
            wx.showToast({ title: '已全部已读', icon: 'success' });
        } catch (e) { console.error(e); }
    }
});
