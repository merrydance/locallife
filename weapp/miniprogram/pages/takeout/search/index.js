"use strict";
/**
 * 搜索页面
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
const dish_1 = require("../../../api/dish");
const merchant_1 = require("../../../api/merchant");
const dish_2 = require("../../../adapters/dish");
const responsive_1 = require("../../../utils/responsive");
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        keyword: '',
        activeTab: 'dishes',
        dishes: [],
        restaurants: [],
        loading: false
    },
    onLoad(options) {
        if (options.keyword) {
            this.setData({ keyword: options.keyword });
            this.onSearch();
        }
        if (options.type) {
            this.setData({ activeTab: options.type });
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onSearchChange(e) {
        this.setData({ keyword: e.detail.value });
    },
    onSearch() {
        return __awaiter(this, void 0, void 0, function* () {
            const { keyword, activeTab } = this.data;
            if (!keyword.trim())
                return;
            this.setData({ loading: true });
            try {
                const app = getApp();
                if (activeTab === 'dishes') {
                    yield this.searchDishes(keyword);
                }
                else {
                    yield this.searchRestaurants(keyword, app.globalData.latitude || undefined, app.globalData.longitude || undefined);
                }
            }
            catch (error) {
                console.error('搜索失败:', error);
                wx.showToast({ title: '搜索失败', icon: 'error' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    searchDishes(keyword) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, dish_1.searchDishes)({
                keyword,
                page_id: 1,
                page_size: 20
            });
            const dishes = (result || []).map((dish) => ({
                id: dish.id,
                name: dish.name,
                shop_name: dish.merchant_name,
                shop_id: dish.merchant_id,
                image_url: dish.image_url,
                price: dish.price,
                month_sales: dish.monthly_sales || 0,
                distance: dish_2.DishAdapter.formatDistance(dish.distance || 0),
                is_available: dish.is_available
            }));
            this.setData({ dishes });
        });
    },
    searchRestaurants(keyword, latitude, longitude) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, merchant_1.searchMerchants)({
                keyword,
                page_id: 1,
                page_size: 20,
                user_latitude: latitude,
                user_longitude: longitude
            });
            const restaurants = (result || []).map((merchant) => ({
                id: merchant.id,
                name: merchant.name,
                cover_image: merchant.logo_url,
                address: merchant.address,
                distance: dish_2.DishAdapter.formatDistance(merchant.distance || 0),
                tags: merchant.tags || []
            }));
            this.setData({ restaurants });
        });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        // 切换 tab 时重新搜索
        if (this.data.keyword.trim()) {
            this.onSearch();
        }
    },
    onDishTap(e) {
        const { id } = e.currentTarget.dataset;
        const dish = this.data.dishes.find(d => d.id === id);
        wx.navigateTo({
            url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${(dish === null || dish === void 0 ? void 0 : dish.shop_id) || ''}`
        });
    },
    onRestaurantTap(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` });
    }
});
