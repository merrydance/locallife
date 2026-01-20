"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const personal_1 = require("../../../api/personal");
const util_1 = require("../../../utils/util");
const getDatasetId = (event) => {
    const dataset = event.currentTarget.dataset;
    const id = dataset.id;
    const numericId = typeof id === 'number' ? id : Number(id);
    return Number.isFinite(numericId) ? numericId : null;
};
Page({
    data: {
        activeTab: 'AVAILABLE',
        coupons: [],
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.loadCoupons();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadCoupons() {
        this.setData({ loading: true });
        try {
            const { activeTab } = this.data;
            let coupons = [];
            if (activeTab === 'AVAILABLE') {
                // 获取可领取的优惠券
                const response = await (0, personal_1.getMyAvailableVouchers)();
                coupons = response.vouchers.map((v) => {
                    var _a;
                    return ({
                        id: v.id,
                        merchant_name: v.merchant_name || (v.merchant_id === 0 ? '平台通用' : `商户${v.merchant_id}`),
                        name: v.name,
                        threshold: v.min_order_amount,
                        thresholdDisplay: (0, util_1.formatPriceNoSymbol)(v.min_order_amount || 0),
                        discount: v.amount,
                        discountDisplay: (0, util_1.formatPriceNoSymbol)(v.amount || 0),
                        end_date: ((_a = v.valid_until) === null || _a === void 0 ? void 0 : _a.split('T')[0]) || '',
                        can_claim: true
                    });
                });
            }
            else {
                // 获取我的优惠券
                const response = await (0, personal_1.getMyVouchers)();
                coupons = response.vouchers.map((v) => {
                    var _a;
                    return ({
                        id: v.id,
                        merchant_name: v.merchant_name || '平台通用',
                        name: v.name,
                        threshold: v.min_order_amount,
                        thresholdDisplay: (0, util_1.formatPriceNoSymbol)(v.min_order_amount || 0),
                        discount: v.amount,
                        discountDisplay: (0, util_1.formatPriceNoSymbol)(v.amount || 0),
                        end_date: ((_a = v.expires_at) === null || _a === void 0 ? void 0 : _a.split('T')[0]) || '',
                        status: v.status
                    });
                });
            }
            this.setData({
                coupons,
                loading: false
            });
        }
        catch (error) {
            console.error('加载优惠券失败:', error);
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false, coupons: [] });
        }
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadCoupons();
    },
    async onClaimCoupon(e) {
        const id = getDatasetId(e);
        if (!id)
            return;
        try {
            await (0, personal_1.claimVoucher)(Number(id));
            wx.showToast({ title: '领取成功', icon: 'success' });
            this.loadCoupons();
        }
        catch (error) {
            console.error('领取优惠券失败:', error);
            wx.showToast({ title: '领取失败', icon: 'error' });
        }
    }
});
