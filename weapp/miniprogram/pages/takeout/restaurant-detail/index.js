"use strict";
/**
 * 餐厅详情页面
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
const search_recommendation_1 = require("../../../api/search-recommendation");
const dish_1 = require("../../../api/dish");
const personal_1 = require("../../../api/personal");
Page({
    data: {
        restaurantId: '',
        restaurant: null,
        activeTab: 'dishes',
        activeCategoryId: '',
        categories: [],
        dishes: [],
        filteredDishes: [],
        reviews: [],
        cartCount: 0,
        cartPrice: 0,
        navBarHeight: 88,
        loading: true
    },
    onLoad(options) {
        const restaurantId = options.id;
        if (!restaurantId) {
            wx.showToast({ title: '商家ID缺失', icon: 'error' });
            setTimeout(() => wx.navigateBack(), 1500);
            return;
        }
        this.setData({ restaurantId });
        this.loadRestaurantDetail();
    },
    onShow() {
        this.updateCartDisplay();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadRestaurantDetail() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            this.setData({ loading: true });
            try {
                const merchantId = parseInt(this.data.restaurantId);
                // 并行加载商户信息、菜品和评价
                const [merchantResult, dishesResult, reviewsResult] = yield Promise.all([
                    this.loadMerchantInfo(merchantId),
                    this.loadDishes(merchantId),
                    this.loadReviews(merchantId)
                ]);
                if (!merchantResult) {
                    wx.showToast({ title: '商家不存在', icon: 'error' });
                    this.setData({ loading: false });
                    return;
                }
                // 从菜品中提取分类
                const categories = this.extractCategories(dishesResult);
                this.setData({
                    restaurant: merchantResult,
                    categories,
                    dishes: dishesResult,
                    reviews: reviewsResult,
                    activeCategoryId: ((_b = (_a = categories[0]) === null || _a === void 0 ? void 0 : _a.id) === null || _b === void 0 ? void 0 : _b.toString()) || '',
                    loading: false
                });
                this.filterDishes();
            }
            catch (error) {
                console.error('加载商户详情失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    loadMerchantInfo(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            try {
                // 使用搜索接口获取商户信息
                const result = yield (0, search_recommendation_1.searchMerchants)({
                    keyword: '',
                    page: 1,
                    page_size: 100
                });
                const merchant = (_a = result.data) === null || _a === void 0 ? void 0 : _a.find((m) => m.id === merchantId);
                if (merchant) {
                    return {
                        id: merchant.id,
                        name: merchant.name,
                        cover_image: merchant.cover_image || merchant.logo_url,
                        address: merchant.address,
                        phone: '', // 搜索结果不包含电话
                        rating: merchant.rating ? (merchant.rating / 10).toFixed(1) : '暂无',
                        review_count: merchant.review_count || 0,
                        tags: merchant.category ? [merchant.category] : [],
                        distance_meters: merchant.distance || 0,
                        delivery_fee: merchant.delivery_fee || 0,
                        delivery_time_minutes: merchant.estimated_delivery_time || 30,
                        biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
                        description: merchant.description || ''
                    };
                }
                return null;
            }
            catch (error) {
                console.error('加载商户信息失败:', error);
                return null;
            }
        });
    },
    loadDishes(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield (0, dish_1.searchDishes)({
                    keyword: '',
                    merchant_id: merchantId,
                    page_id: 1,
                    page_size: 100
                });
                return (result || []).map((dish) => ({
                    id: dish.id,
                    name: dish.name,
                    image_url: dish.image_url,
                    price: dish.price,
                    original_price: dish.price,
                    category_id: '1', // 默认分类，后端暂不返回
                    month_sales: dish.monthly_sales || 0,
                    rating: '5.0',
                    tags: dish.tags || [],
                    is_available: dish.is_available
                }));
            }
            catch (error) {
                console.error('加载菜品失败:', error);
                return [];
            }
        });
    },
    loadReviews(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield (0, personal_1.getMerchantReviews)(merchantId, {
                    page_id: 1,
                    page_size: 20
                });
                return (result.reviews || []).map(review => ({
                    id: review.id,
                    user_name: '用户' + review.user_id,
                    user_avatar: '/assets/default-avatar.png',
                    content: review.content,
                    images: review.images || [],
                    created_at: review.created_at,
                    reply: review.merchant_reply ? {
                        content: review.merchant_reply,
                        created_at: review.replied_at
                    } : null
                }));
            }
            catch (error) {
                console.error('加载评价失败:', error);
                return [];
            }
        });
    },
    extractCategories(dishes) {
        // 由于后端暂不返回分类信息，创建默认分类
        return [
            { id: 1, name: '全部', sort_order: 0 },
            { id: 2, name: '热销', sort_order: 1 },
            { id: 3, name: '推荐', sort_order: 2 }
        ];
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
    },
    onCategoryChange(e) {
        const { id } = e.currentTarget.dataset;
        this.setData({ activeCategoryId: id });
        this.filterDishes();
    },
    filterDishes() {
        const { dishes, activeCategoryId } = this.data;
        // 由于后端暂不返回分类，显示全部菜品
        this.setData({ filteredDishes: dishes });
    },
    onDishTap(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${this.data.restaurantId}` });
    },
    onAddCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            const dish = this.data.dishes.find((d) => d.id === id);
            if (dish) {
                const CartService = require('../../../services/cart').default;
                const success = yield CartService.addItem({
                    merchantId: this.data.restaurant.id,
                    dishId: dish.id,
                    dishName: dish.name,
                    shopName: this.data.restaurant.name,
                    imageUrl: dish.image_url,
                    price: dish.price,
                    priceDisplay: `¥${(dish.price / 100).toFixed(2)}`,
                    quantity: 1
                });
                if (!success) {
                    return;
                }
                this.updateCartDisplay();
                wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 });
            }
        });
    },
    updateCartDisplay() {
        const CartService = require('../../../services/cart').default;
        const cart = CartService.getCart();
        this.setData({
            cartCount: cart.totalCount,
            cartPrice: cart.totalPrice
        });
    },
    onCheckout() {
        wx.navigateTo({ url: '/pages/takeout/cart/index' });
    },
    onCall() {
        var _a;
        const phone = (_a = this.data.restaurant) === null || _a === void 0 ? void 0 : _a.phone;
        if (phone) {
            wx.makePhoneCall({ phoneNumber: phone });
        }
        else {
            wx.showToast({ title: '暂无联系电话', icon: 'none' });
        }
    },
    onMapTap() {
        const { restaurant } = this.data;
        if (restaurant && restaurant.latitude && restaurant.longitude) {
            wx.openLocation({
                latitude: parseFloat(restaurant.latitude),
                longitude: parseFloat(restaurant.longitude),
                name: restaurant.name,
                address: restaurant.address
            });
        }
        else {
            wx.showToast({ title: '暂无位置信息', icon: 'none' });
        }
    }
});
