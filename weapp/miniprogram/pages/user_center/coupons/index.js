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
const personal_1 = require("../../../api/personal");
const util_1 = require("../../../utils/util");
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
    loadCoupons() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const { activeTab } = this.data;
                let coupons = [];
                if (activeTab === 'AVAILABLE') {
                    // 获取可领取的优惠券
                    const response = yield (0, personal_1.getMyAvailableVouchers)();
                    coupons = response.vouchers.map(v => {
                        var _a;
                        return ({
                            id: v.id,
                            merchant_name: v.merchant_name || '平台通用',
                            name: v.name,
                            threshold: v.min_order_amount,
                            thresholdDisplay: (0, util_1.formatPriceNoSymbol)(v.min_order_amount || 0),
                            discount: v.discount_amount,
                            discountDisplay: (0, util_1.formatPriceNoSymbol)(v.discount_amount || 0),
                            end_date: ((_a = v.end_time) === null || _a === void 0 ? void 0 : _a.split('T')[0]) || '',
                            can_claim: true
                        });
                    });
                }
                else {
                    // 获取我的优惠券
                    const response = yield (0, personal_1.getMyVouchers)();
                    coupons = response.vouchers.map((v) => {
                        var _a;
                        return ({
                            id: v.id,
                            merchant_name: v.merchant_name || '平台通用',
                            name: v.voucher_name,
                            threshold: v.min_order_amount,
                            thresholdDisplay: (0, util_1.formatPriceNoSymbol)(v.min_order_amount || 0),
                            discount: v.discount_amount,
                            discountDisplay: (0, util_1.formatPriceNoSymbol)(v.discount_amount || 0),
                            end_date: ((_a = v.end_time) === null || _a === void 0 ? void 0 : _a.split('T')[0]) || '',
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
        });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadCoupons();
    },
    onClaimCoupon(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            if (!id)
                return;
            try {
                yield (0, personal_1.claimVoucher)(Number(id));
                wx.showToast({ title: '领取成功', icon: 'success' });
                this.loadCoupons();
            }
            catch (error) {
                console.error('领取优惠券失败:', error);
                wx.showToast({ title: '领取失败', icon: 'error' });
            }
        });
    }
});
