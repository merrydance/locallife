"use strict";
/**
 * 扫码点餐页面
 * 基于重构后的API接口实现堂食扫码点餐功能
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
const customer_basic_1 = require("../../api/customer-basic");
const customer_dish_browsing_1 = require("../../api/customer-dish-browsing");
const customer_cart_order_1 = require("../../api/customer-cart-order");
const customer_cart_order_2 = require("../../api/customer-cart-order");
Page({
    data: {
        tableInfo: null,
        dishes: [],
        categories: [],
        selectedCategory: 0,
        cartItems: [],
        cartTotal: 0,
        cartCount: 0,
        loading: true,
        showCart: false,
        searchKeyword: ''
    },
    /**
     * 页面加载
     */
    onLoad(options) {
        console.log('扫码点餐页面加载', options);
        if (options.scene) {
            // 通过小程序码进入
            this.handleQRCodeScan(options.scene);
        }
        else if (options.table_id) {
            // 直接传入桌台ID
            this.loadTableInfo(parseInt(options.table_id));
        }
        else {
            // 手动扫码
            this.startQRCodeScan();
        }
    },
    /**
     * 页面显示时刷新购物车
     */
    onShow() {
        if (this.data.tableInfo) {
            this.loadCartData();
        }
    },
    /**
     * 开始扫码
     */
    startQRCodeScan() {
        wx.scanCode({
            success: (res) => {
                console.log('扫码结果:', res);
                this.handleQRCodeScan(res.result);
            },
            fail: (error) => {
                console.error('扫码失败:', error);
                wx.showToast({
                    title: '扫码失败，请重试',
                    icon: 'none'
                });
                // 返回上一页
                wx.navigateBack();
            }
        });
    },
    /**
     * 处理二维码扫描结果
     */
    handleQRCodeScan(qrData) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                wx.showLoading({ title: '加载中...' });
                // 调用扫码接口获取桌台信息
                const scanResult = yield (0, customer_basic_1.scanTable)({ qr_data: qrData });
                if (scanResult.table_id) {
                    yield this.loadTableInfo(scanResult.table_id);
                }
                else {
                    throw new Error('无效的二维码');
                }
            }
            catch (error) {
                console.error('处理扫码结果失败:', error);
                wx.showModal({
                    title: '提示',
                    content: '无效的二维码，请扫描正确的桌台二维码',
                    showCancel: false,
                    success: () => {
                        wx.navigateBack();
                    }
                });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    /**
     * 加载桌台信息
     */
    loadTableInfo(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ loading: true });
                // 获取桌台详细信息
                const tableInfo = yield (0, customer_basic_1.getTableInfo)(tableId);
                this.setData({ tableInfo });
                // 加载商户菜品
                yield this.loadMerchantDishes(tableInfo.merchant_id);
                // 加载购物车数据
                yield this.loadCartData();
            }
            catch (error) {
                console.error('加载桌台信息失败:', error);
                wx.showToast({
                    title: '加载失败，请重试',
                    icon: 'none'
                });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 加载商户菜品
     */
    loadMerchantDishes(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 获取菜品列表
                const dishesResult = yield (0, customer_dish_browsing_1.getDishes)({
                    merchant_id: merchantId,
                    page: 1,
                    page_size: 100
                });
                const dishes = dishesResult.data;
                // 提取分类
                const categoryMap = new Map();
                dishes.forEach(dish => {
                    if (!categoryMap.has(dish.category_id)) {
                        categoryMap.set(dish.category_id, {
                            id: dish.category_id,
                            name: dish.category_name
                        });
                    }
                });
                const categories = [
                    { id: 0, name: '全部' },
                    ...Array.from(categoryMap.values())
                ];
                this.setData({
                    dishes,
                    categories,
                    selectedCategory: 0
                });
            }
            catch (error) {
                console.error('加载菜品失败:', error);
                wx.showToast({
                    title: '加载菜品失败',
                    icon: 'none'
                });
            }
        });
    },
    /**
     * 加载购物车数据
     */
    loadCartData() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const cartResult = yield (0, customer_cart_order_1.getCart)();
                const cartItems = cartResult.items || [];
                // 计算购物车总计
                let cartTotal = 0;
                let cartCount = 0;
                cartItems.forEach(item => {
                    cartTotal += item.price * item.quantity;
                    cartCount += item.quantity;
                });
                this.setData({
                    cartItems,
                    cartTotal,
                    cartCount
                });
            }
            catch (error) {
                console.error('加载购物车失败:', error);
            }
        });
    },
    /**
     * 选择分类
     */
    onCategorySelect(e) {
        const categoryId = e.currentTarget.dataset.id;
        this.setData({ selectedCategory: categoryId });
    },
    /**
     * 搜索菜品
     */
    onSearchInput(e) {
        const keyword = e.detail.value;
        this.setData({ searchKeyword: keyword });
    },
    /**
     * 获取过滤后的菜品列表
     */
    getFilteredDishes() {
        const { dishes, selectedCategory, searchKeyword } = this.data;
        let filteredDishes = dishes;
        // 按分类过滤
        if (selectedCategory > 0) {
            filteredDishes = filteredDishes.filter(dish => dish.category_id === selectedCategory);
        }
        // 按关键词过滤
        if (searchKeyword.trim()) {
            const keyword = searchKeyword.trim().toLowerCase();
            filteredDishes = filteredDishes.filter(dish => dish.name.toLowerCase().includes(keyword) ||
                dish.description.toLowerCase().includes(keyword));
        }
        return filteredDishes;
    },
    /**
     * 添加到购物车
     */
    onAddToCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const dish = e.currentTarget.dataset.dish;
            try {
                yield (0, customer_cart_order_1.addToCart)({
                    dish_id: dish.id,
                    quantity: 1,
                    customizations: [],
                    order_type: 'dine_in', // 堂食类型
                    table_id: (_a = this.data.tableInfo) === null || _a === void 0 ? void 0 : _a.id
                });
                // 刷新购物车
                yield this.loadCartData();
                wx.showToast({
                    title: '已添加到购物车',
                    icon: 'success'
                });
            }
            catch (error) {
                console.error('添加到购物车失败:', error);
                wx.showToast({
                    title: '添加失败，请重试',
                    icon: 'none'
                });
            }
        });
    },
    /**
     * 更新购物车商品数量
     */
    onUpdateCartItem(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { itemId, quantity } = e.currentTarget.dataset;
            try {
                if (quantity <= 0) {
                    yield (0, customer_cart_order_1.removeFromCart)(itemId);
                }
                else {
                    yield (0, customer_cart_order_1.updateCartItem)(itemId, { quantity });
                }
                // 刷新购物车
                yield this.loadCartData();
            }
            catch (error) {
                console.error('更新购物车失败:', error);
                wx.showToast({
                    title: '操作失败，请重试',
                    icon: 'none'
                });
            }
        });
    },
    /**
     * 显示/隐藏购物车
     */
    onToggleCart() {
        this.setData({
            showCart: !this.data.showCart
        });
    },
    /**
     * 去结算
     */
    onCheckout() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cartItems, tableInfo } = this.data;
            if (!cartItems.length) {
                wx.showToast({
                    title: '购物车为空',
                    icon: 'none'
                });
                return;
            }
            if (!tableInfo) {
                wx.showToast({
                    title: '桌台信息异常',
                    icon: 'none'
                });
                return;
            }
            try {
                wx.showLoading({ title: '创建订单中...' });
                // 计算订单金额
                const cartCalculation = yield (0, customer_cart_order_1.calculateCart)();
                // 创建堂食订单
                const order = yield (0, customer_cart_order_2.createOrder)({
                    order_type: 'dine_in',
                    table_id: tableInfo.id,
                    merchant_id: tableInfo.merchant_id,
                    items: cartItems.map(item => ({
                        dish_id: item.dish_id,
                        quantity: item.quantity,
                        price: item.price,
                        customizations: item.customizations || []
                    })),
                    total_amount: cartCalculation.total_amount,
                    remark: ''
                });
                wx.hideLoading();
                // 跳转到订单确认页面
                wx.navigateTo({
                    url: `/pages/order-confirm/order-confirm?order_id=${order.id}&order_type=dine_in`
                });
            }
            catch (error) {
                wx.hideLoading();
                console.error('创建订单失败:', error);
                wx.showToast({
                    title: '创建订单失败，请重试',
                    icon: 'none'
                });
            }
        });
    },
    /**
     * 查看菜品详情
     */
    onDishDetail(e) {
        const dish = e.currentTarget.dataset.dish;
        wx.navigateTo({
            url: `/pages/dish-detail/dish-detail?id=${dish.id}&from=scan_order`
        });
    },
    /**
     * 返回首页
     */
    onBackHome() {
        wx.switchTab({
            url: '/pages/index/index'
        });
    },
    /**
     * 查看商户详情
     */
    onMerchantDetail() {
        const { tableInfo } = this.data;
        if (tableInfo) {
            wx.navigateTo({
                url: `/pages/merchant-detail/merchant-detail?id=${tableInfo.merchant_id}`
            });
        }
    }
});
