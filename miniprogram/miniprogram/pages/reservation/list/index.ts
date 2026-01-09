import { ReservationService, ReservationStatus, ReservationResponse } from '../../../api/reservation';
import ReservationAdapter from '../../../adapters/reservation';

Page({
    data: {
        currentTab: 'all',
        tabs: [
            { value: 'all', label: '全部' },
            { value: 'pending', label: '待确认' },
            { value: 'confirmed', label: '已确认' },
            { value: 'completed', label: '已完成' },
            { value: 'cancelled', label: '已取消' }
        ],
        reservations: [] as ReservationResponse[],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        refreshing: false
    },

    onLoad() {
        this.loadReservations(true);
    },

    onShow() {
        // Refresh list when returning from detail or create page
        if (this.data.reservations.length > 0) {
            this.loadReservations(true);
        }
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true });
        this.loadReservations(true).then(() => {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        });
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadReservations(false);
        }
    },

    onTabChange(e: any) {
        this.setData({
            currentTab: e.detail.value,
            reservations: [],
            page: 1,
            hasMore: true
        });
        this.loadReservations(true);
    },

    async loadReservations(reset: boolean) {
        if (this.data.loading && !reset) return;

        this.setData({ loading: true });

        try {
            const page = reset ? 1 : this.data.page;
            const status = this.data.currentTab === 'all' ? undefined : (this.data.currentTab as ReservationStatus);

            const res = await ReservationService.getReservations({
                page_id: page,
                page_size: this.data.pageSize,
                status
            });

            const formattedReservations = res.reservations.map(r => ({
                ...r,
                _statusText: ReservationAdapter.formatStatus(r.status),
                _statusTheme: ReservationAdapter.getStatusTheme(r.status),
                _timeText: ReservationAdapter.formatDateTime(r.reservation_time)
            }));

            this.setData({
                reservations: reset ? formattedReservations : [...this.data.reservations, ...formattedReservations],
                page: page + 1,
                hasMore: formattedReservations.length === this.data.pageSize,
                loading: false
            });

        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
            wx.showToast({ title: '加载失败', icon: 'none' });
        }
    },

    onToDetail(e: any) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({
            url: `/pages/reservation/detail/index?id=${id}`
        });
    },

    onToCreate() {
        // For demo, list 10 dummy merchants or assume global merchant selection
        // In real app, this might go to merchant list or take a specific merchant ID
        wx.navigateTo({
            url: '/pages/merchant/list/index?action=reservation' // Assuming this exists or just go to home
        });
    }
});
