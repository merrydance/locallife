"use strict";
/**
 * 堂食点餐菜单页面
 * 基于重构后的API接口实现堂食场景的点餐功能
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
const customer_dish_browsing_1 = require("../../../api/customer-dish-browsing");
const customer_cart_order_1 = require("../../../api/customer-cart-order");
const customer_reservation_1 = require("../../../api/customer-reservation");
Page({
    data: {
        tableId: 0,
        merchantId: 0,
        tableInfo: null,
        // 菜品数据
        categories: [],
        dishes: [],
        currentCategoryId: 0,
        // 购物车数据
        cart: {
            items: [],
            total_amount: 0,
            total_quantity: 0
        },
        // 界面状态
        loading: true,
        cartVisible: false,
        selectedDish: null,
    },
    onLoad(options) {
        const { table_id, merchant_id } = options;
        if (!table_id || !merchant_id) {
            wx.showToast({
                title: '参数错误',
                icon: 'error'
            });
            wx.navigateBack();
            return;
        }
        this.setData({
            tableId: parseInt(table_id),
            merchantId: parseInt(merchant_id)
        });
        this.initPage();
    },
    /**
     * 初始化页面数据
     */
    initPage() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            try {
                this.setData({ loading: true });
                // 并行加载数据
                const [tableInfo, categories, cart] = yield Promise.all([
                    (0, customer_reservation_1.getTableInfo)(this.data.tableId),
                    (0, customer_dish_browsing_1.getDishCategories)(this.data.merchantId),
                    this.loadCart()
                ]);
                this.setData({
                    tableInfo,
                    categories,
                    cart,
                    currentCategoryId: ((_a = categories[0]) === null || _a === void 0 ? void 0 : _a.id) || 0
                });
                // 加载第一个分类的菜品
                if (categories.length > 0) {
                    yield this.loadDishes(categories[0].id);
                }
            }
            catch (error) {
                console.error('初始化页面失败:', error);
                wx.showToast({
                    title: error.message || '加载失败',
                    icon: 'error'
                });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 加载购物车
     */
    loadCart() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const cart = yield (0, customer_cart_order_1.getCart)();
                return cart;
            }
            catch (error) {
                console.warn('加载购物车失败:', error);
                return {
                    items: [],
                    total_amount: 0,
                    total_quantity: 0
                };
            }
        });
    },
    /**
     * 加载菜品列表
     */
    loadDishes(categoryId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const dishes = yield (0, customer_dish_browsing_1.getDishes)({
                    merchant_id: this.data.merchantId,
                    category_id: categoryId,
                    page: 1,
                    page_size: 100
                });
                this.setData({
                    dishes: dishes.data,
                    currentCategoryId: categoryId
                });
            }
            catch (error) {
                console.error('加载菜品失败:', error);
                wx.showToast({
                    title: '加载菜品失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 切换分类
     */
    switchCategory(e) {
        const categoryId = e.currentTarget.dataset.id;
        this.loadDishes(categoryId);
    },
    /**
     * 查看菜品详情
     */
    viewDishDetail(e) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);
        if (dish) {
            this.setData({ selectedDish: dish });
        }
    },
    /**
     * 关闭菜品详情
     */
    closeDishDetail() {
        this.setData({ selectedDish: null });
    },
    /**
     * 添加到购物车
     */
    addToCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const dishId = e.currentTarget.dataset.id;
            const dish = this.data.dishes.find(d => d.id === dishId);
            if (!dish || !dish.is_available) {
                wx.showToast({
                    title: '菜品暂不可用',
                    icon: 'error'
                });
                return;
            }
            try {
                yield (0, customer_cart_order_1.addToCart)({
                    dish_id: dishId,
                    quantity: 1,
                    order_type: 'dine_in',
                    table_id: this.data.tableId
                });
                // 重新加载购物车
                const cart = yield this.loadCart();
                this.setData({ cart });
                wx.showToast({
                    title: '已添加到购物车',
                    icon: 'success'
                });
            }
            catch (error) {
                console.error('添加到购物车失败:', error);
                wx.showToast({
                    title: error.message || '添加失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 更新购物车商品数量
     */
    updateCartQuantity(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { dishId, quantity } = e.currentTarget.dataset;
            try {
                if (quantity <= 0) {
                    yield (0, customer_cart_order_1.removeFromCart)(dishId);
                }
                else {
                    yield (0, customer_cart_order_1.updateCartItem)(dishId, { quantity });
                }
                // 重新加载购物车
                const cart = yield this.loadCart();
                this.setData({ cart });
            }
            catch (error) {
                console.error('更新购物车失败:', error);
                wx.showToast({
                    title: '更新失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 显示购物车
     */
    showCart() {
        this.setData({ cartVisible: true });
    },
    /**
     * 隐藏购物车
     */
    hideCart() {
        this.setData({ cartVisible: false });
    },
    /**
     * 去结算
     */
    goToCheckout() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cart, tableId, merchantId } = this.data;
            if (cart.items.length === 0) {
                wx.showToast({
                    title: '购物车为空',
                    icon: 'error'
                });
                return;
            }
            try {
                // 计算订单金额
                const calculation = yield (0, customer_cart_order_1.calculateCart)();
                // 跳转到结算页面
                wx.navigateTo({
                    url: `/pages/dine-in/checkout/checkout?table_id=${tableId}&merchant_id=${merchantId}&order_type=dine_in`
                });
            }
            catch (error) {
                console.error('结算失败:', error);
                wx.showToast({
                    title: error.message || '结算失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 获取购物车中菜品数量
     */
    getCartQuantity(dishId) {
        const item = this.data.cart.items.find(item => item.dish_id === dishId);
        return item ? item.quantity : 0;
    },
    /**
     * 呼叫服务员
     */
    callWaiter() {
        wx.showModal({
            title: '呼叫服务员',
            content: '确定要呼叫服务员吗？',
            success: (res) => {
                if (res.confirm) {
                    // 这里可以调用呼叫服务员的接口
                    wx.showToast({
                        title: '已呼叫服务员',
                        icon: 'success'
                    });
                }
            }
        });
    },
    /**
     * 分享菜单
     */
    onShareAppMessage() {
        const { tableInfo, merchantId } = this.data;
        return {
            title: `${(tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.merchant_name) || '餐厅'}的菜单`,
            path: `/pages/dine-in/scan-entry/scan-entry?table_id=${this.data.tableId}`,
            imageUrl: tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.merchant_logo
        };
    }
});
