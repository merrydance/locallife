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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const reservation_1 = require("../../api/reservation");
const merchant_1 = require("../../api/merchant");
const cart_1 = __importDefault(require("../../services/cart"));
const logger_1 = require("../../utils/logger");
const error_handler_1 = require("../../utils/error-handler");
Page({
    data: {
        tableId: '',
        merchantId: '',
        session: null,
        dishes: [],
        categories: [],
        activeCategoryId: 'all',
        cartCount: 0,
        cartPrice: 0,
        navBarHeight: 88,
        loading: true
    },
    onLoad(options) {
        // 微信扫码进入,参数格式: ?table_id=xxx&merchant_id=xxx
        if (options.table_id && options.merchant_id) {
            this.setData({
                tableId: options.table_id,
                merchantId: options.merchant_id
            });
            this.init();
        }
        else {
            // For dev testing without scan
            if (options.dev) {
                this.setData({ tableId: '1', merchantId: '1' });
                this.init();
            }
            else {
                wx.showToast({ title: '无效的二维码', icon: 'error' });
                setTimeout(() => wx.navigateBack(), 1500);
            }
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    init() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                yield this.checkAndOpenSession();
                yield this.loadMenu();
                this.setData({ loading: false });
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Dining.init');
                this.setData({ loading: false });
            }
        });
    },
    checkAndOpenSession() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const session = yield (0, reservation_1.getCurrentDiningSession)(this.data.tableId);
                this.setData({ session });
            }
            catch (error) {
                // Session likely doesn't exist, try to open
                // Ask for person count
                return new Promise((resolve, reject) => {
                    wx.showModal({
                        title: '开台',
                        content: '请输入用餐人数',
                        editable: true,
                        placeholderText: '1',
                        success: (res) => __awaiter(this, void 0, void 0, function* () {
                            if (res.confirm) {
                                const person = parseInt(res.content || '1');
                                try {
                                    const session = yield (0, reservation_1.openDiningSession)(this.data.tableId, person);
                                    this.setData({ session });
                                    resolve(session);
                                }
                                catch (err) {
                                    reject(err);
                                }
                            }
                            else {
                                wx.navigateBack();
                                reject(new Error('User cancelled'));
                            }
                        })
                    });
                });
            }
        });
    },
    loadMenu() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const dishes = yield (0, merchant_1.getMerchantDishes)(this.data.merchantId);
                const categories = [{ id: 'all', name: '全部' }];
                const categoryMap = new Map();
                dishes.forEach((dish) => {
                    if (dish.category_id && !categoryMap.has(dish.category_id)) {
                        categoryMap.set(dish.category_id, {
                            id: dish.category_id,
                            name: dish.category_id // 注意：这里使用 category_id 作为 name，可能需要后端返回 category_name
                        });
                    }
                });
                categories.push(...Array.from(categoryMap.values()));
                this.setData({ dishes, categories });
            }
            catch (error) {
                logger_1.logger.error('加载菜单失败', error, 'Dining.loadMenu');
                wx.showToast({ title: '加载菜单失败', icon: 'error' });
            }
        });
    },
    onCategoryChange(e) {
        const detail = e.detail;
        const categoryId = typeof detail === 'string' ? detail : (detail.id || '');
        this.setData({ activeCategoryId: categoryId });
    },
    onAddCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.detail;
            const dish = this.data.dishes.find((d) => d.id === id);
            if (dish) {
                const success = yield cart_1.default.addItem({
                    merchantId: this.data.merchantId,
                    dishId: dish.id,
                    dishName: dish.name,
                    shopName: '当前餐厅',
                    imageUrl: dish.image_url,
                    price: dish.price,
                    priceDisplay: `¥${(dish.price / 100).toFixed(2)}`
                });
                if (!success) {
                    return;
                }
                this.updateCartDisplay();
                wx.showToast({ title: '已加入', icon: 'success', duration: 500 });
            }
        });
    },
    updateCartDisplay() {
        const cart = cart_1.default.getCart();
        this.setData({
            cartCount: cart.totalCount,
            cartPrice: cart.totalPrice
        });
    },
    onSubmitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            const { session, cartCount } = this.data;
            if (cartCount === 0) {
                wx.showToast({ title: '请先选择菜品', icon: 'none' });
                return;
            }
            if (!session) {
                wx.showToast({ title: '会话无效', icon: 'none' });
                return;
            }
            const cart = cart_1.default.getCart();
            wx.showModal({
                title: '确认下单',
                content: `共${cart.totalCount}件菜品，${cart.totalPriceDisplay}`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            const items = cart.items.map((i) => ({
                                dish_id: i.dishId,
                                quantity: i.quantity,
                                extra_options: []
                            }));
                            yield (0, reservation_1.createDiningOrder)(session.id, { items });
                            wx.showToast({ title: '下单成功', icon: 'success' });
                            cart_1.default.clear();
                            this.setData({ cartCount: 0, cartPrice: 0 });
                        }
                        catch (error) {
                            logger_1.logger.error('下单失败', error, 'Dining.onSubmitOrder');
                            wx.showToast({ title: '下单失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    onCallWaiter() {
        wx.showToast({ title: '已呼叫服务员', icon: 'success' });
    }
});
