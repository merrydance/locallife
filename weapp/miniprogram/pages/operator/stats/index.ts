import { PlatformDashboardService } from '../../../api/platform-dashboard';

const platformDashboardService = new PlatformDashboardService();

Page({
    data: {
        stats: {
            gmv: '0.00',
            active_merchants: 0,
            active_users: 0,
            total_orders: 0
        },
        loading: false
    },

    onLoad() {
        this.loadStats();
    },

    async loadStats() {
        this.setData({ loading: true });
        try {
            const today = new Date().toISOString().split('T')[0];
            const result = await platformDashboardService.getPlatformOverview({
                start_date: today,
                end_date: today
            });

            this.setData({
                stats: {
                    gmv: (result.total_gmv / 100).toFixed(2),
                    active_merchants: result.active_merchants,
                    active_users: result.active_users,
                    total_orders: result.total_orders
                },
                loading: false
            });

        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
    }
});
