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
        currentTab: 'available',
        tabs: [
            { value: 'available', label: '未使用' },
            { value: 'used', label: '已使用' },
            { value: 'expired', label: '已过期' }
        ],
        coupons: [],
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
    onTabChange(e) {
        this.setData({
            currentTab: e.detail.value,
            coupons: [],
            page: 1,
            hasMore: true
        });
        this.loadCoupons(true);
    },
    loadCoupons(reset) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.loading && !reset)
                return;
            this.setData({ loading: true });
            try {
                const page = reset ? 1 : this.data.page;
                const res = yield coupon_1.CouponService.getMyCoupons({
                    page_id: page,
                    page_size: this.data.pageSize,
                    status: this.data.currentTab
                });
                const newCoupons = res.coupons.map(c => (Object.assign(Object.assign({}, c), { valueDisplay: c.type === 'discount' ? String(c.value / 10) : (0, util_1.formatPriceNoSymbol)(c.value || 0), _formatValue: c.type === 'discount' ? `${c.value / 10}折` : `¥${(0, util_1.formatPriceNoSymbol)(c.value || 0)}`, _formatMinSpend: c.min_spend > 0 ? `满${(0, util_1.formatPriceNoSymbol)(c.min_spend)}可用` : '无门槛', _formatTime: c.end_time.split(' ')[0] })));
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
            }
        });
    },
    onGoUse() {
        // Go to merchant list or specific merchant
        wx.switchTab({ url: '/pages/index/index' });
    }
});
