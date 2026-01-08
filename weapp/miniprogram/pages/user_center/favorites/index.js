"use strict";
/**
 * 收藏页面
 * 使用真实后端API
 */
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
const logger_1 = require("../../../utils/logger");
const util_1 = require("../../../utils/util");
Page({
    data: {
        favorites: [],
        activeTab: 'dishes',
        loading: false,
        navBarHeight: 88
    },
    onLoad() {
        this.loadFavorites();
    },
    onShow() {
        // 返回时刷新数据
        if (this.data.favorites.length > 0) {
            this.loadFavorites();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadFavorites();
    },
    loadFavorites() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const { activeTab } = this.data;
                if (activeTab === 'dishes') {
                    yield this.loadFavoriteDishes();
                }
                else {
                    yield this.loadFavoriteMerchants();
                }
            }
            catch (error) {
                console.error('加载收藏失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    loadFavoriteDishes() {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, personal_1.getFavoriteDishes)({ page_id: 1, page_size: 50 });
            const favorites = (result.dishes || []).map((item) => ({
                id: item.dish_id,
                type: 'DISH',
                name: item.dish_name,
                image: item.dish_image_url || '/assets/default-dish.png',
                price: item.price,
                priceDisplay: (0, util_1.formatPriceNoSymbol)(item.price || 0),
                merchantId: item.merchant_id,
                merchantName: item.merchant_name,
                desc: item.merchant_name
            }));
            this.setData({ favorites });
        });
    },
    loadFavoriteMerchants() {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, personal_1.getFavoriteMerchants)({ page_id: 1, page_size: 50 });
            const favorites = (result.merchants || []).map((item) => {
                var _a;
                return ({
                    id: item.merchant_id,
                    type: 'MERCHANT',
                    name: item.merchant_name,
                    image: item.merchant_logo_url || '/assets/default-merchant.png',
                    monthlySales: item.monthly_sales,
                    deliveryFee: item.estimated_delivery_fee,
                    tags: item.tags || [],
                    desc: ((_a = item.tags) === null || _a === void 0 ? void 0 : _a.slice(0, 2).join(' · ')) || ''
                });
            });
            this.setData({ favorites });
        });
    },
    onItemClick(e) {
        const { id, type } = e.currentTarget.dataset;
        if (type === 'DISH') {
            const item = this.data.favorites.find(f => f.id === id);
            wx.navigateTo({
                url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${(item === null || item === void 0 ? void 0 : item.merchantId) || ''}`
            });
        }
        else {
            wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` });
        }
    },
    onRemoveFavorite(e) {
        const { id, type } = e.currentTarget.dataset;
        wx.showModal({
            title: '取消收藏',
            content: '确定要取消收藏吗？',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    yield this.doRemoveFavorite(id, type);
                }
            })
        });
    },
    doRemoveFavorite(id, type) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '处理中...' });
            try {
                if (type === 'DISH') {
                    yield (0, personal_1.removeDishFromFavorites)(id);
                }
                else {
                    yield (0, personal_1.removeMerchantFromFavorites)(id);
                }
                wx.hideLoading();
                wx.showToast({ title: '已取消收藏', icon: 'success' });
                // 从列表中移除
                const favorites = this.data.favorites.filter(f => !(f.id === id && f.type === type));
                this.setData({ favorites });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('取消收藏失败', error, 'favorites.doRemoveFavorite');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    }
});
