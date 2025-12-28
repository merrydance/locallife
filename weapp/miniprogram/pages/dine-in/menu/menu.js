"use strict";
/**
 * 堂食点餐菜单页面
 * 支持两种入口：
 * 1. 页面跳转：直接传 table_id 和 merchant_id
 * 2. 扫描小程序码：scene 参数格式 m=商户ID&t=桌号
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
const table_1 = require("../../../api/table");
const cart_1 = require("../../../api/cart");
Page({
    data: {
        tableId: 0,
        merchantId: 0,
        tableNo: '',
        // 商户和桌台信息
        merchantInfo: null,
        tableInfo: null,
        // 菜品数据
        categories: [],
        combos: [],
        promotions: [],
        currentCategoryId: 0,
        currentDishes: [],
        // 购物车数据
        cart: null,
        cartTotal: 0,
        cartCount: 0,
        // 界面状态
        loading: true,
        cartVisible: false,
        selectedDish: null,
        // 错误状态
        hasError: false,
        errorMessage: ''
    },
    onLoad(options) {
        let tableId = null;
        let merchantId = null;
        let tableNo = null;
        // 方式1: 直接参数 (从页面跳转)
        if (options.table_id && options.merchant_id) {
            tableId = parseInt(options.table_id);
            merchantId = parseInt(options.merchant_id);
            this.setData({ tableId, merchantId });
            this.initPageById(tableId, merchantId);
            return;
        }
        // 方式2: scene参数 (从小程序码扫描)
        // scene格式: m_商户ID-t_桌号 或 tid_桌台ID
        if (options.scene) {
            const scene = decodeURIComponent(options.scene);
            // 解析新格式: m_1-t_A01
            const mMatch = scene.match(/m_(\d+)/);
            const tMatch = scene.match(/t_([^-]+)/);
            const tidMatch = scene.match(/tid_(\d+)/);
            if (mMatch && tMatch) {
                merchantId = parseInt(mMatch[1]);
                tableNo = tMatch[1];
                this.setData({ merchantId, tableNo });
                this.initPageByTableNo(merchantId, tableNo);
                return;
            }
            else if (tidMatch) {
                tableId = parseInt(tidMatch[1]);
                this.setData({ tableId });
                this.showError('暂不支持此扫码格式');
                return;
            }
        }
        // 参数错误 - 显示友好提示
        this.showError('请通过扫描桌台二维码进入点餐页面');
    },
    /**
     * 显示错误状态
     */
    showError(message) {
        this.setData({
            loading: false,
            hasError: true,
            errorMessage: message
        });
    },
    /**
     * 返回上一页
     */
    goBack() {
        const pages = getCurrentPages();
        if (pages.length > 1) {
            wx.navigateBack();
        }
        else {
            wx.switchTab({ url: '/pages/index/index' });
        }
    },
    /**
     * 通过桌台ID和商户ID初始化页面
     */
    initPageById(tableId, merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            // 暂时用 initPageByTableNo 的方式，需要查询桌号
            // 后续可以优化为直接用 tableId
            wx.showToast({ title: '加载中...', icon: 'loading' });
            this.setData({ loading: true });
            try {
                // 先获取桌台信息
                const { request } = require('../../../utils/request');
                const tableDetail = yield request({
                    url: `/v1/tables/${tableId}`,
                    method: 'GET'
                });
                if (tableDetail && tableDetail.table_no) {
                    yield this.initPageByTableNo(merchantId, tableDetail.table_no);
                }
                else {
                    throw new Error('无法获取桌台信息');
                }
            }
            catch (error) {
                console.error('初始化失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 通过桌号初始化页面（扫码场景）
     */
    initPageByTableNo(merchantId, tableNo) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            try {
                this.setData({ loading: true });
                wx.showLoading({ title: '加载菜单...' });
                // 调用扫码API获取完整信息
                const scanResult = yield (0, table_1.scanTable)(merchantId, tableNo);
                // 设置桌台和商户信息
                this.setData({
                    tableId: scanResult.table.id,
                    merchantId: scanResult.merchant.id,
                    tableNo: scanResult.table.table_no,
                    merchantInfo: scanResult.merchant,
                    tableInfo: scanResult.table,
                    categories: scanResult.categories || [],
                    combos: scanResult.combos || [],
                    promotions: scanResult.promotions || [],
                    currentCategoryId: (scanResult.categories && ((_a = scanResult.categories[0]) === null || _a === void 0 ? void 0 : _a.id)) || 0,
                    currentDishes: (scanResult.categories && ((_b = scanResult.categories[0]) === null || _b === void 0 ? void 0 : _b.dishes)) || []
                });
                // 设置页面标题
                wx.setNavigationBarTitle({ title: scanResult.merchant.name });
                // 加载购物车
                yield this.loadCart();
                wx.hideLoading();
            }
            catch (error) {
                wx.hideLoading();
                console.error('扫码初始化失败:', error);
                wx.showToast({
                    title: error.userMessage || '加载失败',
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
                const cart = yield (0, cart_1.getCart)(this.data.merchantId);
                this.setData({
                    cart,
                    cartTotal: cart.subtotal,
                    cartCount: cart.total_count
                });
            }
            catch (error) {
                console.warn('加载购物车失败:', error);
                this.setData({
                    cart: null,
                    cartTotal: 0,
                    cartCount: 0
                });
            }
        });
    },
    /**
     * 切换分类
     */
    switchCategory(e) {
        const categoryId = e.currentTarget.dataset.id;
        const category = this.data.categories.find(c => c.id === categoryId);
        this.setData({
            currentCategoryId: categoryId,
            currentDishes: (category === null || category === void 0 ? void 0 : category.dishes) || []
        });
    },
    /**
     * 查看菜品详情
     */
    viewDishDetail(e) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.currentDishes.find(d => d.id === dishId);
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
    onAddToCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const dishId = e.currentTarget.dataset.id;
            const dish = this.data.currentDishes.find(d => d.id === dishId);
            if (!dish || !dish.is_available) {
                wx.showToast({ title: '菜品暂不可用', icon: 'none' });
                return;
            }
            try {
                yield (0, cart_1.addToCart)({
                    merchant_id: this.data.merchantId,
                    dish_id: dishId,
                    quantity: 1
                });
                yield this.loadCart();
                wx.showToast({ title: '已添加', icon: 'success' });
            }
            catch (error) {
                console.error('添加到购物车失败:', error);
                wx.showToast({ title: error.userMessage || '添加失败', icon: 'none' });
            }
        });
    },
    /**
     * 增加数量
     */
    onIncreaseQuantity(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const itemId = e.currentTarget.dataset.itemId;
            const item = (_a = this.data.cart) === null || _a === void 0 ? void 0 : _a.items.find(i => i.id === itemId);
            if (!item)
                return;
            try {
                yield (0, cart_1.updateCartItem)(itemId, { quantity: item.quantity + 1 });
                yield this.loadCart();
            }
            catch (error) {
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    /**
     * 减少数量
     */
    onDecreaseQuantity(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const itemId = e.currentTarget.dataset.itemId;
            const item = (_a = this.data.cart) === null || _a === void 0 ? void 0 : _a.items.find(i => i.id === itemId);
            if (!item)
                return;
            try {
                if (item.quantity <= 1) {
                    yield (0, cart_1.removeFromCart)(itemId);
                }
                else {
                    yield (0, cart_1.updateCartItem)(itemId, { quantity: item.quantity - 1 });
                }
                yield this.loadCart();
            }
            catch (error) {
                wx.showToast({ title: '操作失败', icon: 'none' });
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
            if (!cart || cart.items.length === 0) {
                wx.showToast({ title: '购物车为空', icon: 'none' });
                return;
            }
            try {
                // 计算订单金额
                yield (0, cart_1.calculateCart)({ merchant_id: merchantId });
                // 跳转到结算页面
                wx.navigateTo({
                    url: `/pages/dine-in/checkout/checkout?table_id=${tableId}&merchant_id=${merchantId}&order_type=dine_in`
                });
            }
            catch (error) {
                console.error('结算失败:', error);
                wx.showToast({ title: error.userMessage || '结算失败', icon: 'none' });
            }
        });
    },
    /**
     * 获取购物车中菜品数量
     */
    getCartQuantity(dishId) {
        var _a;
        const item = (_a = this.data.cart) === null || _a === void 0 ? void 0 : _a.items.find(item => item.dish_id === dishId);
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
                    wx.showToast({ title: '已呼叫服务员', icon: 'success' });
                }
            }
        });
    },
    /**
     * 分享菜单
     */
    onShareAppMessage() {
        const { merchantInfo, tableId } = this.data;
        return {
            title: `${(merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.name) || '餐厅'}的菜单`,
            path: `/pages/dine-in/menu/menu?table_id=${tableId}&merchant_id=${this.data.merchantId}`,
            imageUrl: merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.logo_url
        };
    }
});
