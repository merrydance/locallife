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
const coupon_1 = require("../../../api/coupon");
const util_1 = require("../../../utils/util");
Page({
    data: {
        coupons: [],
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
    loadCoupons(reset) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.loading && !reset)
                return;
            this.setData({ loading: true });
            try {
                const page = reset ? 1 : this.data.page;
                const res = yield coupon_1.CouponService.getAvailableCoupons({
                    page_id: page,
                    page_size: this.data.pageSize
                });
                const newCoupons = res.coupons.map(c => (Object.assign(Object.assign({}, c), { valueDisplay: c.type === 'discount' ? String(c.value / 10) : (0, util_1.formatPriceNoSymbol)(c.value || 0), _formatValue: c.type === 'discount' ? `${c.value / 10}折` : `¥${(0, util_1.formatPriceNoSymbol)(c.value || 0)}`, _formatMinSpend: c.min_spend > 0 ? `满${(0, util_1.formatPriceNoSymbol)(c.min_spend)}可用` : '无门槛', _percent: Math.round((c.claimed_count / c.total_count) * 100) })));
                this.setData({
                    coupons: reset ? newCoupons : [...this.data.coupons, ...newCoupons],
                    page: page + 1,
                    hasMore: newCoupons.length === this.data.pageSize,
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onClaim(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, index } = e.currentTarget.dataset;
            const coupon = this.data.coupons[index];
            if (coupon.is_claimed)
                return;
            try {
                wx.showLoading({ title: '领取中' });
                yield coupon_1.CouponService.claimCoupon(id);
                // Update local state
                const key = `coupons[${index}].is_claimed`;
                this.setData({ [key]: true });
                wx.showToast({ title: '领取成功', icon: 'success' });
            }
            catch (error) {
                wx.showToast({ title: error.message || '领取失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
