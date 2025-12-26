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
        promotions: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadPromotions();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadPromotions() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Mock data - GET /api/v1/merchant/promotions
                const mockPromotions = [
                    {
                        id: 'promo_1',
                        name: '开业大酬宾',
                        type: 'DISCOUNT',
                        description: '全场8折',
                        start_date: '2024-11-01',
                        end_date: '2024-11-30',
                        status: 'ACTIVE'
                    },
                    {
                        id: 'promo_2',
                        name: '新品尝鲜',
                        type: 'GIFT',
                        description: '点新品送饮料',
                        start_date: '2024-11-15',
                        end_date: '2024-11-20',
                        status: 'EXPIRED'
                    }
                ];
                this.setData({
                    promotions: mockPromotions,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onAddPromotion() {
        wx.showToast({ title: '功能开发中', icon: 'none' });
    },
    onToggleStatus(e) {
        const { id } = e.currentTarget.dataset;
        wx.showToast({ title: '状态已更新', icon: 'success' });
        this.loadPromotions();
    }
});
