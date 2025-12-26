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
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        activeTab: 'ACTIVE',
        coupons: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadCoupons();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCoupons() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Mock data - GET /api/v1/merchant/coupons?status=xxx
                const mockCoupons = [
                    {
                        id: 'coupon_1',
                        name: '满30减5',
                        type: 'CASH',
                        threshold: 3000,
                        discount: 500,
                        total_count: 500,
                        claimed_count: 125,
                        used_count: 45,
                        start_date: '2024-11-01',
                        end_date: '2024-11-30',
                        status: 'ACTIVE'
                    },
                    {
                        id: 'coupon_2',
                        name: '满50减10',
                        type: 'CASH',
                        threshold: 5000,
                        discount: 1000,
                        total_count: 300,
                        claimed_count: 80,
                        used_count: 30,
                        start_date: '2024-11-01',
                        end_date: '2024-11-30',
                        status: 'ACTIVE'
                    }
                ];
                this.setData({
                    coupons: mockCoupons,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadCoupons();
    },
    onAddCoupon() {
        wx.navigateTo({ url: '/pages/merchant/marketing/coupons/edit/index' });
    },
    onEditCoupon(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/marketing/coupons/edit/index?id=${id}` });
    },
    onToggleStatus(e) {
        const { id } = e.currentTarget.dataset;
        wx.showModal({
            title: '状态变更',
            content: '确认变更优惠券状态?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    // PATCH /api/v1/merchant/coupons/{id}/toggle
                    wx.showToast({ title: '状态已更新', icon: 'success' });
                    this.loadCoupons();
                }
            })
        });
    }
});
