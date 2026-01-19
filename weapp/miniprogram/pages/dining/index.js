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
const dining_session_1 = require("../../api/dining-session");
const billing_group_1 = require("../../api/billing-group");
const order_1 = require("../../api/order");
const merchant_1 = require("../../api/merchant");
const cart_1 = __importDefault(require("../../services/cart"));
const logger_1 = require("../../utils/logger");
const error_handler_1 = require("../../utils/error-handler");
const util_1 = require("../../utils/util");
Page({
    data: {
        tableId: '',
        merchantId: '',
        session: null,
        billingGroup: null,
        billingGroupId: undefined,
        reservationId: undefined,
        dishes: [],
        categories: [],
        activeCategoryId: 'all',
        cartCount: 0,
        cartPrice: 0,
        cartPriceDisplay: '0.00',
        sharedDishCounts: {},
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
            var _a;
            // 先做预检，判断是否存在属于当前用户的预订
            const precheck = yield (0, reservation_1.precheckDiningSession)(Number(this.data.tableId));
            const reservationId = precheck.reserved && precheck.is_reservation_owner ? precheck.reservation_id : undefined;
            this.setData({ reservationId });
            const result = yield (0, reservation_1.openDiningSession)({
                table_id: Number(this.data.tableId),
                reservation_id: reservationId
            });
            this.setData({ session: result.session, billingGroup: result.billing_group, billingGroupId: (_a = result.billing_group) === null || _a === void 0 ? void 0 : _a.id });
                try {
                    wx.setStorageSync('activeDiningSession', {
                        id: result.session.id,
                        table_id: result.session.table_id,
                        merchant_id: result.session.merchant_id,
                        reservation_id: result.session.reservation_id,
                        status: result.session.status,
                        updated_at: result.session.updated_at || result.session.created_at
                    });
                }
                catch (error) {
                    logger_1.logger.warn('缓存用餐会话失败', error, 'Dining.checkAndOpenSession');
                }
            yield this.chooseBillingGroup(result.session.id, result.billing_group);
            yield this.loadSharedOrderSummary();
            return result.session;
        });
    },
    chooseBillingGroup(sessionId, defaultGroup) {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve) => {
                wx.showModal({
                    title: '结算方式',
                    content: '是否单独结算？',
                    confirmText: '单独结算',
                    cancelText: '一起点餐',
                    success: (res) => __awaiter(this, void 0, void 0, function* () {
                        if (res.confirm) {
                            try {
                                const group = yield (0, billing_group_1.createBillingGroup)(sessionId);
                                this.setData({ billingGroup: group, billingGroupId: group.id });
                            }
                            catch (error) {
                                logger_1.logger.error('创建账单组失败', error, 'Dining.chooseBillingGroup');
                                wx.showToast({ title: '创建账单组失败', icon: 'error' });
                                this.setData({ billingGroup: defaultGroup, billingGroupId: defaultGroup.id });
                            }
                        }
                        else {
                            this.setData({ billingGroup: defaultGroup, billingGroupId: defaultGroup.id });
                        }
                        resolve();
                    })
                });
            });
        });
    },
    loadSharedOrderSummary() {
        return __awaiter(this, void 0, void 0, function* () {
            const billingGroupId = this.data.billingGroupId;
            if (!billingGroupId) {
                return;
            }
            try {
                const { orders } = yield (0, billing_group_1.listBillingGroupOrders)(billingGroupId);
                const summary = {};
                for (const order of orders) {
                    try {
                        const detail = yield (0, order_1.getOrderDetail)(order.order_id);
                        for (const item of detail.items || []) {
                            if (item.dish_id) {
                                summary[item.dish_id] = (summary[item.dish_id] || 0) + item.quantity;
                            }
                        }
                    }
                    catch (error) {
                        logger_1.logger.warn('获取订单详情失败', error, 'Dining.loadSharedOrderSummary');
                    }
                }
                this.setData({ sharedDishCounts: summary });
            }
            catch (error) {
                logger_1.logger.warn('获取账单组订单失败', error, 'Dining.loadSharedOrderSummary');
            }
        });
    },
    loadMenu() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield (0, merchant_1.getMerchantDishes)(this.data.merchantId);
                // 预处理菜品价格
                const dishes = (response.dishes || []).map((dish) => (Object.assign(Object.assign({}, dish), { priceDisplay: (0, util_1.formatPriceNoSymbol)(dish.price || 0), memberPriceDisplay: dish.member_price ? (0, util_1.formatPriceNoSymbol)(dish.member_price) : null })));
                const categories = [{ id: 'all', name: '全部' }];
                const categoryMap = new Map();
                dishes.forEach((dish) => {
                    if (dish.category_id && !categoryMap.has(dish.category_id)) {
                        categoryMap.set(dish.category_id, {
                            id: dish.category_id,
                            name: dish.category_name || String(dish.category_id)
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
                const sharedCount = this.data.sharedDishCounts[dish.id] || 0;
                if (sharedCount > 0) {
                    const proceed = yield new Promise((resolve) => {
                        wx.showModal({
                            title: '同伴已点',
                            content: `同伴已点 ${sharedCount} 份该菜，是否继续添加？`,
                            confirmText: '继续添加',
                            cancelText: '取消',
                            success: (res) => resolve(res.confirm)
                        });
                    });
                    if (!proceed) {
                        return;
                    }
                }
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
            cartPrice: cart.totalPrice,
            cartPriceDisplay: (0, util_1.formatPriceNoSymbol)(cart.totalPrice || 0)
        });
    },
    onSubmitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            const { session, cartCount, billingGroupId } = this.data;
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
                                customizations: []
                            }));
                            yield (0, dining_session_1.createDiningOrder)({
                                merchant_id: Number(this.data.merchantId),
                                table_id: Number(this.data.tableId),
                                reservation_id: this.data.reservationId,
                                items,
                                order_type: 'dine_in',
                                billing_group_id: billingGroupId
                            });
                            wx.showToast({ title: '下单成功', icon: 'success' });
                            cart_1.default.clear();
                            this.setData({ cartCount: 0, cartPrice: 0 });
                            yield this.loadSharedOrderSummary();
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
