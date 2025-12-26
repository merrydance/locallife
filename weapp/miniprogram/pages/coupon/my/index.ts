import { CouponService, UserCoupon, CouponStatus } from '../../../api/coupon';

Page({
    data: {
        currentTab: 'available',
        tabs: [
            { value: 'available', label: '未使用' },
            { value: 'used', label: '已使用' },
            { value: 'expired', label: '已过期' }
        ],
        coupons: [] as UserCoupon[],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        refreshing: false
    },

    onLoad() {
        this.loadCoupons(true);
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true });
        this.loadCoupons(true).then(() => {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        });
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadCoupons(false);
        }
    },

    onTabChange(e: any) {
        this.setData({
            currentTab: e.detail.value,
            coupons: [],
            page: 1,
            hasMore: true
        });
        this.loadCoupons(true);
    },

    async loadCoupons(reset: boolean) {
        if (this.data.loading && !reset) return;

        this.setData({ loading: true });

        try {
            const page = reset ? 1 : this.data.page;
            const res = await CouponService.getMyCoupons({
                page_id: page,
                page_size: this.data.pageSize,
                status: this.data.currentTab as CouponStatus
            });

            const newCoupons = res.coupons.map(c => ({
                ...c,
                _formatValue: c.type === 'discount' ? `${c.value / 10}折` : `¥${c.value / 100}`,
                _formatMinSpend: c.min_spend > 0 ? `满${c.min_spend / 100}可用` : '无门槛',
                _formatTime: c.end_time.split(' ')[0]
            }));

            this.setData({
                coupons: reset ? newCoupons : [...this.data.coupons, ...newCoupons],
                page: page + 1,
                hasMore: newCoupons.length === this.data.pageSize,
                loading: false
            });
        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
    },

    onGoUse() {
        // Go to merchant list or specific merchant
        wx.switchTab({ url: '/pages/index/index' });
    }
});
