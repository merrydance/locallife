import { CouponService, Coupon } from '../../../api/coupon';
import { formatPriceNoSymbol } from '../../../utils/util';

Page({
    data: {
        coupons: [] as Coupon[],
        loading: true,
        refreshing: false,
        page: 1,
        pageSize: 10,
        hasMore: true
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

    async loadCoupons(reset: boolean) {
        if (this.data.loading && !reset) return;

        this.setData({ loading: true });

        try {
            const page = reset ? 1 : this.data.page;
            const res = await CouponService.getAvailableCoupons({
                page_id: page,
                page_size: this.data.pageSize
            });

            const newCoupons = res.coupons.map(c => ({
                ...c,
                valueDisplay: c.type === 'discount' ? String(c.value / 10) : formatPriceNoSymbol(c.value || 0),
                _formatValue: c.type === 'discount' ? `${c.value / 10}折` : `¥${formatPriceNoSymbol(c.value || 0)}`,
                _formatMinSpend: c.min_spend > 0 ? `满${formatPriceNoSymbol(c.min_spend)}可用` : '无门槛',
                _percent: Math.round((c.claimed_count / c.total_count) * 100)
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
            wx.showToast({ title: '加载失败', icon: 'none' });
        }
    },

    async onClaim(e: any) {
        const { id, index } = e.currentTarget.dataset;
        const coupon = this.data.coupons[index];

        if (coupon.is_claimed) return;

        try {
            wx.showLoading({ title: '领取中' });
            await CouponService.claimCoupon(id);

            // Update local state
            const key = `coupons[${index}].is_claimed`;
            this.setData({ [key]: true });

            wx.showToast({ title: '领取成功', icon: 'success' });
        } catch (error: any) {
            wx.showToast({ title: error.message || '领取失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    }
});
