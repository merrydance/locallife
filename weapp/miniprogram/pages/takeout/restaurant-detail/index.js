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
const merchant_1 = require("../../../api/merchant");
const room_1 = require("../../../api/room");
const cart_1 = require("../../../api/cart");
const image_1 = require("../../../utils/image");
const image_security_1 = require("../../../utils/image-security");
const util_1 = require("../../../utils/util");
Page({
    data: {
        restaurantId: '',
        restaurant: null,
        activeTab: 'dishes',
        activeCategoryId: '',
        categories: [],
        dishes: [],
        filteredDishes: [],
        combos: [],
        rooms: [],
        cartCount: 0,
        cartPrice: 0,
        cartPriceDisplay: '0.00',
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
            var _a;
            this.setData({ loading: true });
            try {
                const merchantId = parseInt(this.data.restaurantId);
                // 并行加载商户信息、菜品、套餐和包间
                const [merchantResult, dishesResult, combosResult, roomsResult] = yield Promise.all([
                    this.loadMerchantInfo(merchantId),
                    this.loadDishes(merchantId),
                    this.loadCombos(merchantId),
                    this.loadRooms(merchantId)
                ]);
                if (!merchantResult) {
                    wx.showToast({ title: '商家不存在', icon: 'error' });
                    this.setData({ loading: false });
                    return;
                }
                // 从菜品中提取分类
                const categories = this.extractCategories(dishesResult);
                const firstCategoryId = ((_a = categories[0]) === null || _a === void 0 ? void 0 : _a.id) || '';
                this.setData({
                    restaurant: merchantResult,
                    categories,
                    dishes: dishesResult.dishes,
                    combos: combosResult,
                    rooms: roomsResult,
                    activeCategoryId: firstCategoryId,
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
            try {
                const merchant = yield (0, merchant_1.getPublicMerchantDetail)(merchantId);
                if (merchant) {
                    // 私有图片需要签名（营业执照、食品许可证）
                    const [coverImage, businessLicense, foodPermit] = yield Promise.all([
                        (0, image_security_1.resolveImageURL)(merchant.cover_image || merchant.logo_url || ''),
                        (0, image_security_1.resolveImageURL)(merchant.business_license_image_url || ''),
                        (0, image_security_1.resolveImageURL)(merchant.food_permit_url || '')
                    ]);
                    // 格式化营业时间
                    let businessHoursDisplay = '';
                    if (merchant.business_hours && merchant.business_hours.length > 0) {
                        const today = new Date().getDay();
                        const todayHours = merchant.business_hours.find(h => h.day_of_week === today);
                        if (todayHours) {
                            businessHoursDisplay = `${todayHours.open_time} - ${todayHours.close_time}`;
                        }
                        else if (merchant.business_hours[0]) {
                            const first = merchant.business_hours[0];
                            businessHoursDisplay = `${first.open_time} - ${first.close_time}`;
                        }
                    }
                    // 格式化所有营业时间
                    const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
                    const formattedHours = (merchant.business_hours || []).map(h => (Object.assign(Object.assign({}, h), { day_name: dayNames[h.day_of_week] })));
                    return {
                        id: merchant.id,
                        name: merchant.name,
                        cover_image: coverImage,
                        logo_url: (0, image_1.getPublicImageUrl)(merchant.logo_url || ''),
                        address: merchant.address,
                        phone: merchant.phone,
                        latitude: merchant.latitude,
                        longitude: merchant.longitude,
                        tags: merchant.tags || [],
                        monthly_sales: merchant.monthly_sales || 0,
                        avg_prep_minutes: merchant.avg_prep_minutes || 15,
                        biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
                        description: merchant.description || '',
                        business_license_image_url: businessLicense,
                        food_permit_url: foodPermit,
                        business_hours: formattedHours,
                        business_hours_display: businessHoursDisplay,
                        discount_rules: merchant.discount_rules || [],
                        vouchers: merchant.vouchers || [],
                        delivery_promotions: merchant.delivery_promotions || []
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
                const result = yield (0, merchant_1.getPublicMerchantDishes)(merchantId);
                const dishes = (result.dishes || []).map((dish) => ({
                    id: dish.id,
                    name: dish.name,
                    image_url: (0, image_1.getPublicImageUrl)(dish.image_url || ''),
                    price: dish.price,
                    priceDisplay: (0, util_1.formatPriceNoSymbol)(dish.price || 0),
                    member_price: dish.member_price,
                    memberPriceDisplay: dish.member_price ? (0, util_1.formatPriceNoSymbol)(dish.member_price) : null,
                    original_price: dish.original_price,
                    originalPriceDisplay: dish.original_price ? (0, util_1.formatPriceNoSymbol)(dish.original_price) : null,
                    category_id: dish.category_id || 0,
                    category_name: dish.category_name || '未分类',
                    monthly_sales: dish.monthly_sales || 0,
                    prepare_time: dish.prepare_time || 10,
                    tags: dish.tags || [],
                    is_available: true
                }));
                return { dishes, categories: result.categories || [] };
            }
            catch (error) {
                console.error('加载菜品失败:', error);
                return { dishes: [], categories: [] };
            }
        });
    },
    loadCombos(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield (0, merchant_1.getPublicMerchantCombos)(merchantId);
                return (result.combos || []).map((combo) => ({
                    id: combo.id,
                    name: combo.name,
                    description: combo.description || '',
                    image_url: (0, image_1.getPublicImageUrl)(combo.image_url || ''),
                    combo_price: combo.combo_price,
                    comboPriceDisplay: (0, util_1.formatPriceNoSymbol)(combo.combo_price || 0),
                    original_price: combo.original_price,
                    originalPriceDisplay: (0, util_1.formatPriceNoSymbol)(combo.original_price || 0),
                    savingsDisplay: (0, util_1.formatPriceNoSymbol)((combo.original_price || 0) - (combo.combo_price || 0)),
                    dishes: combo.dishes || []
                }));
            }
            catch (error) {
                console.error('加载套餐失败:', error);
                return [];
            }
        });
    },
    loadRooms(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield (0, room_1.getPublicMerchantRooms)(merchantId);
                // 包间图片是公共图片，使用getPublicImageUrl处理
                return (result.rooms || []).map((room) => (Object.assign(Object.assign({}, room), { primary_image: room.primary_image ? (0, image_1.getPublicImageUrl)(room.primary_image) : '' })));
            }
            catch (error) {
                console.error('加载包间失败:', error);
                return [];
            }
        });
    },
    extractCategories(dishesResult) {
        const categoryMap = new Map();
        categoryMap.set(0, { id: 0, name: '全部', sort_order: -1 });
        // 优先使用API返回的分类
        if (dishesResult.categories && dishesResult.categories.length > 0) {
            dishesResult.categories.forEach(cat => {
                if (!categoryMap.has(cat.id)) {
                    categoryMap.set(cat.id, cat);
                }
            });
        }
        else {
            // 回退：从菜品中提取分类
            dishesResult.dishes.forEach(dish => {
                if (dish.category_id && !categoryMap.has(dish.category_id)) {
                    categoryMap.set(dish.category_id, {
                        id: dish.category_id,
                        name: dish.category_name || `分类${dish.category_id}`,
                        sort_order: dish.category_id
                    });
                }
            });
        }
        return Array.from(categoryMap.values()).sort((a, b) => a.sort_order - b.sort_order);
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
    },
    onCategoryChange(e) {
        const id = e.currentTarget.dataset.id;
        this.setData({ activeCategoryId: id });
        this.filterDishes();
    },
    filterDishes() {
        const { dishes, activeCategoryId } = this.data;
        if (activeCategoryId === 0 || activeCategoryId === '0' || !activeCategoryId) {
            this.setData({ filteredDishes: dishes });
        }
        else {
            const filtered = dishes.filter((d) => d.category_id == activeCategoryId);
            this.setData({ filteredDishes: filtered });
        }
    },
    onDishTap(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${this.data.restaurantId}` });
    },
    onComboTap(e) {
        const id = e.currentTarget.dataset.id;
        wx.showToast({ title: '套餐详情开发中', icon: 'none' });
    },
    onAddCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
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
                if (success) {
                    this.updateCartDisplay();
                    wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 });
                }
            }
        });
    },
    onAddComboCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            const combo = this.data.combos.find((c) => c.id === id);
            if (combo) {
                const CartService = require('../../../services/cart').default;
                const success = yield CartService.addItem({
                    merchantId: this.data.restaurant.id,
                    comboId: combo.id,
                    dishName: combo.name,
                    shopName: this.data.restaurant.name,
                    imageUrl: combo.image_url,
                    price: combo.combo_price,
                    priceDisplay: `¥${(combo.combo_price / 100).toFixed(2)}`,
                    quantity: 1,
                    isCombo: true
                });
                if (success) {
                    this.updateCartDisplay();
                    wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 });
                }
            }
        });
    },
    updateCartDisplay() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            try {
                // 使用与外卖首页相同的方式获取购物车状态
                const userCarts = yield (0, cart_1.getUserCarts)('takeout');
                const totalCount = ((_a = userCarts.summary) === null || _a === void 0 ? void 0 : _a.total_items) || 0;
                const totalPrice = ((_b = userCarts.summary) === null || _b === void 0 ? void 0 : _b.total_amount) || 0;
                this.setData({
                    cartCount: totalCount,
                    cartPrice: totalPrice,
                    cartPriceDisplay: (0, util_1.formatPriceNoSymbol)(totalPrice)
                });
            }
            catch (error) {
                // 获取失败时重置为0
                this.setData({
                    cartCount: 0,
                    cartPrice: 0,
                    cartPriceDisplay: '0.00'
                });
            }
        });
    },
    onCheckout() {
        wx.navigateTo({ url: '/pages/takeout/cart/index' });
    },
    onCartTap() {
        // 点击购物车栏跳转到购物车页面
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
        const restaurant = this.data.restaurant;
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
    },
    onPreviewLicense(e) {
        const src = e.currentTarget.dataset.src;
        if (src) {
            wx.previewImage({
                current: src,
                urls: [src]
            });
        }
    },
    onRoomTap(e) {
        const roomId = e.currentTarget.dataset.id;
        if (roomId) {
            wx.navigateTo({
                url: `/pages/reservation/room-detail/index?id=${roomId}`
            });
        }
    }
});
