"use strict";
/**
 * 菜品详情页面
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
const tracker_1 = require("../../../utils/tracker");
const dish_1 = require("../../../api/dish");
const personal_1 = require("../../../api/personal");
const image_1 = require("../../../utils/image");
const util_1 = require("../../../utils/util");
Page({
    data: {
        dishId: '',
        merchantId: '',
        dish: null,
        selectedSpecs: {},
        quantity: 1,
        navBarHeight: 88,
        currentImageIndex: 0,
        loading: true,
        totalPrice: 0,
        totalPriceDisplay: '0.00'
    },
    onLoad(options) {
        const dishId = options.id;
        const merchantId = options.merchant_id || '';
        // 从列表页传递过来的额外信息
        const shopName = decodeURIComponent(options.shop_name || '');
        const monthSales = parseInt(options.month_sales || '0');
        const distanceMeters = parseInt(options.distance || '0');
        const deliveryTimeMinutes = parseInt(options.delivery_time || '0');
        if (!dishId) {
            wx.showToast({ title: '菜品ID缺失', icon: 'error' });
            setTimeout(() => wx.navigateBack(), 1500);
            return;
        }
        this.setData({
            dishId,
            merchantId,
            extraInfo: { shopName, monthSales, distanceMeters, deliveryTimeMinutes }
        });
        this.loadDishDetail();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadDishDetail() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            this.setData({ loading: true });
            try {
                const dishId = parseInt(this.data.dishId);
                // 获取菜品详情
                const dishData = yield dish_1.DishManagementService.getDishDetail(dishId);
                if (!dishData) {
                    wx.showToast({ title: '菜品不存在', icon: 'error' });
                    this.setData({ loading: false });
                    return;
                }
                // 加载评价（如果有商户ID）
                let reviews = [];
                if (dishData.merchant_id) {
                    try {
                        const reviewsResult = yield (0, personal_1.getMerchantReviews)(dishData.merchant_id, {
                            page_id: 1,
                            page_size: 5
                        });
                        reviews = (reviewsResult.reviews || []).map(r => ({
                            user_name: '用户' + r.user_id,
                            content: r.content,
                            images: r.images || [],
                            created_at: r.created_at
                        }));
                    }
                    catch (e) {
                        console.warn('加载评价失败:', e);
                    }
                }
                // 从 URL 参数获取额外信息
                const extraInfo = this.data.extraInfo || {};
                const imageUrl = (0, image_1.getPublicImageUrl)(dishData.image_url);
                // 构建菜品视图模型
                const dish = {
                    id: dishData.id,
                    name: dishData.name,
                    shop_name: extraInfo.shopName || '商家',
                    shop_id: dishData.merchant_id,
                    merchant_id: dishData.merchant_id,
                    images: imageUrl ? [imageUrl] : [],
                    image_url: imageUrl,
                    price: dishData.price,
                    priceDisplay: (0, util_1.formatPriceNoSymbol)(dishData.price || 0),
                    original_price: dishData.price,
                    originalPriceDisplay: (0, util_1.formatPriceNoSymbol)(dishData.price || 0),
                    member_price: dishData.member_price,
                    memberPriceDisplay: dishData.member_price ? (0, util_1.formatPriceNoSymbol)(dishData.member_price) : null,
                    description: dishData.description || '',
                    is_available: dishData.is_available,
                    is_online: dishData.is_online,
                    prepare_time: dishData.prepare_time,
                    spec_groups: this.convertCustomizationGroups(dishData.customization_groups),
                    reviews,
                    tags: ((_a = dishData.tags) === null || _a === void 0 ? void 0 : _a.map(t => t.name)) || [],
                    ingredients: dishData.ingredients || [],
                    // 额外展示字段（从列表页传递）
                    month_sales: extraInfo.monthSales || 0,
                    distance_meters: extraInfo.distanceMeters || 0,
                    delivery_time_minutes: extraInfo.deliveryTimeMinutes || Math.round((dishData.prepare_time || 10) + 15) // 制作时间+配送时间
                };
                // 初始化规格选择
                const selectedSpecs = {};
                if (dish.spec_groups) {
                    dish.spec_groups.forEach((group) => {
                        if (group.specs && group.specs.length > 0) {
                            selectedSpecs[group.id] = group.specs[0].id;
                        }
                    });
                }
                this.setData({
                    dish,
                    selectedSpecs,
                    loading: false
                });
                this.updateTotalPrice();
                // 埋点
                tracker_1.tracker.log(tracker_1.EventType.VIEW_DISH, String(dish.id), {
                    shop_id: dish.shop_id,
                    price: dish.price,
                    tags: dish.tags
                });
            }
            catch (error) {
                console.error('加载菜品详情失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    // 转换定制化分组为规格组格式
    convertCustomizationGroups(groups) {
        if (!groups || groups.length === 0)
            return [];
        return groups.map(group => ({
            id: group.id.toString(),
            name: group.name,
            is_required: group.is_required,
            specs: (group.options || []).map((opt) => ({
                id: opt.id.toString(),
                name: opt.tag_name,
                price_diff: opt.extra_price || 0,
                priceDiffDisplay: opt.extra_price ? (0, util_1.formatPriceNoSymbol)(opt.extra_price) : null
            }))
        }));
    },
    onImageChange(e) {
        this.setData({ currentImageIndex: e.detail.current });
    },
    onSpecTap(e) {
        const { groupId, specId } = e.currentTarget.dataset;
        const { selectedSpecs } = this.data;
        if (selectedSpecs[groupId] === specId)
            return;
        this.setData({
            [`selectedSpecs.${groupId}`]: specId
        });
        this.updateTotalPrice();
    },
    updateTotalPrice() {
        const { dish, selectedSpecs } = this.data;
        if (!dish)
            return;
        let totalPrice = dish.price;
        if (dish.spec_groups) {
            dish.spec_groups.forEach((group) => {
                var _a;
                const selectedSpecId = selectedSpecs[group.id];
                const spec = (_a = group.specs) === null || _a === void 0 ? void 0 : _a.find((s) => s.id === selectedSpecId);
                if (spec) {
                    totalPrice += spec.price_diff;
                }
            });
        }
        this.setData({
            totalPrice,
            totalPriceDisplay: (0, util_1.formatPriceNoSymbol)(totalPrice)
        });
    },
    onQuantityChange(e) {
        const { type } = e.currentTarget.dataset;
        let { quantity } = this.data;
        if (type === 'minus' && quantity > 1) {
            quantity--;
        }
        else if (type === 'plus') {
            quantity++;
        }
        this.setData({ quantity });
    },
    onAddToCart() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const { dish, selectedSpecs, quantity, totalPrice } = this.data;
            if (!dish)
                return;
            // 构建规格描述
            const specNames = [];
            if (dish.spec_groups) {
                dish.spec_groups.forEach((group) => {
                    var _a;
                    const selectedSpecId = selectedSpecs[group.id];
                    const spec = (_a = group.specs) === null || _a === void 0 ? void 0 : _a.find((s) => s.id === selectedSpecId);
                    if (spec) {
                        specNames.push(spec.name);
                    }
                });
            }
            const specDesc = specNames.length > 0 ? `(${specNames.join('/')})` : '';
            const CartService = require('../../../services/cart').default;
            const success = yield CartService.addItem({
                merchantId: dish.shop_id || dish.merchant_id,
                dishId: dish.id,
                dishName: `${dish.name}${specDesc}`,
                shopName: dish.shop_name,
                imageUrl: ((_a = dish.images) === null || _a === void 0 ? void 0 : _a[0]) || dish.image_url,
                price: totalPrice,
                priceDisplay: `¥${(totalPrice / 100).toFixed(2)}`,
                quantity
            });
            if (!success) {
                return;
            }
            tracker_1.tracker.log(tracker_1.EventType.ADD_CART, dish.id, {
                shop_id: dish.shop_id,
                quantity,
                price: totalPrice,
                tags: dish.tags
            });
            wx.showToast({ title: '已加入购物车', icon: 'success' });
        });
    },
    onBuyNow() {
        this.onAddToCart();
        wx.navigateTo({ url: '/pages/takeout/cart/index' });
    },
    onShopTap() {
        const { dish } = this.data;
        if (dish && dish.shop_id) {
            wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${dish.shop_id}` });
        }
    }
});
