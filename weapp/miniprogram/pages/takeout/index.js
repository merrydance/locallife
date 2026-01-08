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
const dish_1 = require("../../adapters/dish");
const dish_2 = require("../../api/dish");
const merchant_1 = require("../../api/merchant");
const cart_1 = __importDefault(require("../../services/cart"));
const cart_2 = require("../../api/cart");
const geo_1 = require("../../utils/geo");
const navigation_1 = __importDefault(require("../../utils/navigation"));
const logger_1 = require("../../utils/logger");
const error_handler_1 = require("../../utils/error-handler");
const global_store_1 = require("../../utils/global-store");
const request_manager_1 = require("../../utils/request-manager");
const dish_3 = require("../../api/dish");
const responsive_1 = require("../../utils/responsive");
const image_1 = require("../../utils/image");
const PAGE_CONTEXT = 'takeout_index';
const PAGE_SIZE = 10; // 每页条数，用于无限滚动分页
Page({
    data: {
        activeTab: 'dishes',
        dishes: [],
        restaurants: [],
        packages: [],
        categories: [],
        activeCategoryId: '1',
        cartTotalCount: 0,
        cartTotalPrice: 0,
        address: '点此获取位置',
        navBarHeight: 88,
        scrollViewHeight: 600, // 动态计算
        searchKeyword: '',
        page: 1,
        hasMore: true,
        loading: false,
        // 位置状态
        needLocation: false, // 是否需要用户手动定位
        // 下拉刷新状态
        refresherTriggered: false,
        // 预加载状态
        isPrefetching: false
    },
    // 预加载缓存 (不放在 data 中以免触发渲染)
    _prefetchedDishes: [],
    _prefetchedRestaurants: [],
    _prefetchedPackages: [],
    _prefetchHasMore: true,
    onLoad() {
        // 设置导航栏高度和滚动区域高度
        const { navBarHeight } = (0, responsive_1.getStableBarHeights)();
        const windowInfo = wx.getWindowInfo();
        // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
        const scrollViewHeight = windowInfo.windowHeight - navBarHeight;
        this.setData({
            navBarHeight,
            scrollViewHeight
        });
        // 立即加载分类
        this.loadCategories();
        // 从全局获取位置信息
        const app = getApp();
        const loc = app.globalData.location;
        if (loc && loc.name) {
            // 有缓存位置信息,直接使用
            this.setData({ address: loc.name });
        }
        else {
            // 没有位置信息，显示提示
            this.setData({ address: '定位中...' });
        }
        // 订阅位置变化
        this._unsubscribeLocation = global_store_1.globalStore.subscribe('location', (newLocation) => {
            logger_1.logger.info('[Takeout] 收到位置更新', newLocation, 'Takeout.onLoad');
            if (newLocation.name) {
                // 隐藏位置引导提示，更新地址显示
                this.setData({
                    address: newLocation.name,
                    needLocation: false
                });
                // 如果还没有数据，位置更新后自动加载
                if (this.data.dishes.length === 0 && !this.data.loading) {
                    logger_1.logger.info('[Takeout] 位置已更新，开始加载数据', undefined, 'Takeout.onLoad');
                    this.loadData();
                }
            }
        });
    },
    onLocationTap() {
        // 导航栏已经处理位置获取，这里只需要响应位置变化事件
        // 如果用户点击页面内的位置，可以打开位置选择器
        wx.chooseLocation({
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                const app = getApp();
                // 更新全局位置
                app.globalData.latitude = res.latitude;
                app.globalData.longitude = res.longitude;
                app.globalData.location = {
                    name: res.name || res.address,
                    address: res.address
                };
                // 更新页面显示
                this.setData({ address: res.name || res.address });
                // 重新加载基于新位置的推荐
                this.onLocationChange();
                wx.showToast({ title: '已更新位置推荐', icon: 'success', duration: 1500 });
            }),
            fail: () => {
                // 用户取消选择
            }
        });
    },
    onShow() {
        logger_1.logger.debug('[Takeout.onShow] 页面显示', {
            dishesCount: this.data.dishes.length,
            loading: this.data.loading
        }, 'Takeout.onShow');
        // 从购物车返回时更新购物车数据
        this.updateCartDisplay();
        // 更新位置显示
        const app = getApp();
        const loc = app.globalData.location;
        if (loc && loc.name) {
            this.setData({ address: loc.name });
        }
        // 检查是否需要加载数据
        if (this.data.dishes.length === 0 && !this.data.loading) {
            logger_1.logger.info('[Takeout.onShow] 开始 tryLoadData', undefined, 'Takeout.onShow');
            this.tryLoadData();
        }
        else {
            logger_1.logger.debug('[Takeout.onShow] 跳过 tryLoadData', {
                reason: this.data.dishes.length > 0 ? '已有数据' : '正在加载中'
            }, 'Takeout.onShow');
        }
    },
    // 尝试加载数据，等待 token 准备好，位置未授权则直接引导
    tryLoadData() {
        return __awaiter(this, arguments, void 0, function* (retryCount = 0) {
            var _a;
            const MAX_TOKEN_RETRIES = 10; // Token 最多等待 5 秒
            const RETRY_INTERVAL = 500;
            const { getToken } = require('../../utils/auth');
            const app = getApp();
            const token = getToken();
            const hasLocation = !!(app.globalData.latitude && app.globalData.longitude);
            // 1. 先等待 Token（登录通常很快）
            if (!token) {
                if (retryCount >= MAX_TOKEN_RETRIES) {
                    logger_1.logger.error('❌ 登录超时', { waitedTime: `${(retryCount * RETRY_INTERVAL) / 1000}秒` }, 'Takeout.tryLoadData');
                    wx.showModal({
                        title: '登录超时',
                        content: '请检查网络连接后重试',
                        confirmText: '重新加载',
                        success: (res) => {
                            if (res.confirm) {
                                wx.reLaunch({ url: '/pages/takeout/index' });
                            }
                        }
                    });
                    return;
                }
                if (retryCount === 0) {
                    logger_1.logger.info('等待登录...', undefined, 'Takeout.tryLoadData');
                }
                setTimeout(() => this.tryLoadData(retryCount + 1), RETRY_INTERVAL);
                return;
            }
            // 2. Token 已就绪，检查位置
            if (!hasLocation) {
                // 位置未授权，直接显示引导，不再疯狂重试
                logger_1.logger.info('位置未授权，显示引导界面', undefined, 'Takeout.tryLoadData');
                this.showLocationGuide();
                return;
            }
            // 3. Token 和位置都准备好，加载数据
            logger_1.logger.info('✅ Token 和位置都已准备好，开始加载数据', {
                tokenLength: token.length,
                locationName: ((_a = app.globalData.location) === null || _a === void 0 ? void 0 : _a.name) || '未知'
            }, 'Takeout.tryLoadData');
            this.loadData();
        });
    },
    /**
     * 显示位置引导（页面内提示，不弹窗）
     */
    showLocationGuide() {
        // 设置页面状态，显示位置引导提示
        this.setData({
            needLocation: true,
            loading: false,
            address: '请先定位'
        });
        logger_1.logger.info('显示位置引导提示', undefined, 'Takeout.showLocationGuide');
    },
    /**
     * 用户点击"手动定位"按钮
     */
    onManualLocation() {
        this.openLocationPicker();
    },
    /**
     * 用户点击"重新定位"按钮
     */
    onRetryLocation() {
        const app = getApp();
        this.setData({ address: '定位中...' });
        // 重新获取位置
        app.getLocationCoordinates();
        // 位置更新后会通过 globalStore 订阅自动触发 loadData
    },
    /**
     * 打开位置选择器
     */
    openLocationPicker() {
        wx.chooseLocation({
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                const app = getApp();
                // 更新全局位置
                app.globalData.latitude = res.latitude;
                app.globalData.longitude = res.longitude;
                app.globalData.location = {
                    name: res.name || res.address,
                    address: res.address
                };
                // 同步到 globalStore
                const { globalStore } = require('../../utils/global-store');
                globalStore.updateLocation(res.latitude, res.longitude, res.name || res.address, res.address);
                logger_1.logger.info('用户手动选择位置', {
                    latitude: res.latitude,
                    longitude: res.longitude,
                    name: res.name
                }, 'Takeout.openLocationPicker');
                // 更新导航栏显示
                this.setData({ address: res.name || res.address });
                // 重新加载数据
                this.loadData();
                wx.showToast({ title: '位置已更新', icon: 'success', duration: 1500 });
            }),
            fail: (err) => {
                logger_1.logger.warn('用户取消选择位置', err, 'Takeout.openLocationPicker');
                // 用户取消，再次提示
                wx.showModal({
                    title: '需要位置信息',
                    content: '本地生活服务必须基于您的位置才能使用',
                    confirmText: '重新选择',
                    cancelText: '退出',
                    success: (res) => {
                        if (res.confirm) {
                            this.openLocationPicker();
                        }
                        else {
                            wx.switchTab({ url: '/pages/user_center/index' });
                        }
                    }
                });
            }
        });
    },
    onHide() {
        // 页面隐藏时取消所有pending请求
        request_manager_1.requestManager.cancelByContext(PAGE_CONTEXT);
        logger_1.logger.debug('页面隐藏,已取消所有请求', undefined, 'Takeout.onHide');
    },
    onUnload() {
        // 页面卸载时清理
        request_manager_1.requestManager.cancelByContext(PAGE_CONTEXT);
        // 取消位置订阅
        if (this._unsubscribeLocation) {
            this._unsubscribeLocation();
        }
    },
    onLocationChange() {
        this.setData({ page: 1 });
        this.loadData();
    },
    onTabChange(e) {
        const { value } = e.detail;
        this.setData({
            activeTab: value,
            page: 1
        });
        // 切换 Tab 时重新加载对应类型的标签
        this.loadCategories();
        this.loadData();
    },
    loadCategories() {
        return __awaiter(this, void 0, void 0, function* () {
            // 根据当前 Tab 获取对应类型的标签
            const { activeTab } = this.data;
            let tagType = 'dish'; // 默认菜品标签
            if (activeTab === 'packages') {
                tagType = 'combo';
            }
            else if (activeTab === 'restaurants') {
                tagType = 'merchant';
            }
            try {
                const tags = yield (0, dish_2.getTags)(tagType);
                // 添加"全部"选项作为第一个
                const categories = [
                    { id: '', name: '全部' },
                    ...tags.map(tag => ({ id: String(tag.id), name: tag.name }))
                ];
                this.setData({
                    categories,
                    activeCategoryId: '' // 默认选中"全部"
                });
            }
            catch (error) {
                console.error('加载标签失败', error);
                // 后备：硬编码标签
                const categories = [
                    { id: '', name: '全部' },
                    { id: '1', name: '热销' },
                    { id: '2', name: '主食' },
                    { id: '3', name: '小吃' }
                ];
                this.setData({ categories, activeCategoryId: '' });
            }
        });
    },
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            try {
                const { activeTab } = this.data;
                if (activeTab === 'dishes') {
                    yield this.loadDishes(this.data.page === 1);
                }
                else if (activeTab === 'restaurants') {
                    yield this.loadRestaurants(this.data.page === 1);
                }
                else {
                    yield this.loadPackages(this.data.page === 1);
                }
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Takeout.loadData');
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    loadDishes() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (reset) {
                // 重置时清空缓存和数据
                this._prefetchedDishes = [];
                this._prefetchHasMore = true;
                this.setData({
                    page: 1,
                    dishes: [],
                    hasMore: true
                });
            }
            try {
                const app = getApp();
                const currentPage = reset ? 1 : this.data.page;
                let newDishes = [];
                let hasMore = true;
                // 1. 优先使用预加载的缓存数据（无延迟）
                if (!reset && this._prefetchedDishes.length > 0) {
                    newDishes = this._prefetchedDishes;
                    hasMore = this._prefetchHasMore;
                    this._prefetchedDishes = []; // 清空缓存
                }
                else {
                    // 2. 没有缓存则请求当前页
                    // 获取选中的标签ID用于过滤（空字符串表示"全部"，不传 tag_id）
                    const tagId = this.data.activeCategoryId ? parseInt(this.data.activeCategoryId) : null;
                    const params = {
                        user_latitude: app.globalData.latitude || undefined,
                        user_longitude: app.globalData.longitude || undefined,
                        limit: PAGE_SIZE,
                        page: currentPage
                    };
                    // 只有选择了具体标签时才传 tag_id
                    if (tagId && !isNaN(tagId)) {
                        params.tag_id = tagId;
                    }
                    const result = yield (0, dish_2.getRecommendedDishes)(params);
                    newDishes = result.dishes.map((dish) => dish_1.DishAdapter.fromSummaryDTO(dish));
                    hasMore = result.has_more;
                }
                // 更新视图
                if (reset) {
                    this.setData({
                        dishes: newDishes,
                        hasMore
                    });
                }
                else {
                    this.setData({
                        dishes: [...this.data.dishes, ...newDishes],
                        hasMore
                    });
                }
                // 3. 异步预加载下一页（不阻塞当前渲染）
                if (hasMore) {
                    this.prefetchNextDishes(currentPage + 1);
                }
                // 预加载图片
                const { preloadImages } = require('../../utils/image');
                const imageUrls = newDishes.map((dish) => dish.imageUrl).filter(Boolean);
                setTimeout(() => {
                    preloadImages(imageUrls, false);
                }, 100);
            }
            catch (error) {
                logger_1.logger.error('加载菜品失败', error, 'Takeout.loadDishes');
                throw error;
            }
        });
    },
    /**
     * 预加载下一页菜品（后台静默执行）
     */
    prefetchNextDishes(nextPage) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.isPrefetching || this._prefetchedDishes.length > 0)
                return;
            this.setData({ isPrefetching: true });
            try {
                const app = getApp();
                const result = yield (0, dish_2.getRecommendedDishes)({
                    user_latitude: app.globalData.latitude || undefined,
                    user_longitude: app.globalData.longitude || undefined,
                    limit: PAGE_SIZE,
                    page: nextPage
                });
                this._prefetchedDishes = result.dishes.map((dish) => dish_1.DishAdapter.fromSummaryDTO(dish));
                this._prefetchHasMore = result.has_more;
            }
            catch (error) {
                logger_1.logger.debug('预加载下一页失败', error, 'Takeout.prefetchNextDishes');
            }
            finally {
                this.setData({ isPrefetching: false });
            }
        });
    },
    loadRestaurants() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (reset) {
                this.setData({ page: 1, restaurants: [], hasMore: true });
            }
            try {
                // 使用推荐商户接口，传递当前页码
                const app = getApp();
                const currentPage = reset ? 1 : this.data.page;
                const result = yield (0, merchant_1.getRecommendedMerchants)({
                    user_latitude: app.globalData.latitude || undefined,
                    user_longitude: app.globalData.longitude || undefined,
                    limit: PAGE_SIZE,
                    page: currentPage
                });
                // Map for enrichment (if lat/lng available)
                const merchantsForEnrich = result.merchants.map((m) => (Object.assign(Object.assign({}, m), { merchant_latitude: m.latitude, merchant_longitude: m.longitude })));
                const enrichedMerchants = yield (0, geo_1.enrichMerchantsWithDistance)(merchantsForEnrich);
                const restaurantViewModels = enrichedMerchants.map((m) => ({
                    id: m.id,
                    name: m.name,
                    imageUrl: m.logo_url,
                    cuisineType: m.tags ? m.tags.slice(0, 2) : [],
                    avgPrice: 0,
                    avgPriceDisplay: '人均未知',
                    distance: dish_1.DishAdapter.formatDistance(m.distance),
                    address: m.address,
                    businessHoursDisplay: '营业中',
                    availableRooms: 0,
                    availableRoomsBadge: '',
                    tags: m.tags ? m.tags.slice(0, 3) : []
                }));
                if (reset) {
                    this.setData({
                        restaurants: restaurantViewModels,
                        hasMore: result.has_more
                    });
                }
                else {
                    this.setData({
                        restaurants: [...this.data.restaurants, ...restaurantViewModels],
                        hasMore: result.has_more
                    });
                }
                // 预加载图片
                const { preloadImages } = require('../../utils/image');
                const imageUrls = restaurantViewModels.map((r) => r.imageUrl).filter(Boolean);
                setTimeout(() => {
                    preloadImages(imageUrls, false);
                }, 100);
            }
            catch (error) {
                logger_1.logger.error('加载商户失败', error, 'Takeout.loadRestaurants');
                throw error;
            }
        });
    },
    loadPackages() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (reset) {
                this.setData({ page: 1, packages: [], hasMore: true });
            }
            try {
                // 调用后端推荐套餐接口，传递当前页码和位置
                const app = getApp();
                const currentPage = reset ? 1 : this.data.page;
                const result = yield (0, dish_3.getRecommendedCombos)({
                    limit: PAGE_SIZE,
                    page: currentPage,
                    user_latitude: app.globalData.latitude || undefined,
                    user_longitude: app.globalData.longitude || undefined
                });
                const packageViewModels = result.combos.map((combo) => ({
                    id: combo.id,
                    name: combo.name,
                    merchantId: combo.merchant_id,
                    merchantName: combo.merchant_name || '',
                    imageUrl: (0, image_1.getPublicImageUrl)(combo.image_url) || '/assets/placeholder_food.png',
                    price: combo.combo_price,
                    priceDisplay: `¥${(combo.combo_price / 100).toFixed(2)}`,
                    originalPrice: combo.original_price,
                    originalPriceDisplay: `¥${(combo.original_price / 100).toFixed(2)}`,
                    savingsPercent: Math.round(combo.savings_percent || 0),
                    monthlySales: combo.monthly_sales || 0,
                    salesBadge: `月售${combo.monthly_sales || 0}`,
                    distance: dish_1.DishAdapter.formatDistance(combo.distance),
                    deliveryFee: combo.estimated_delivery_fee,
                    deliveryFeeDisplay: combo.estimated_delivery_fee
                        ? `配送费¥${(combo.estimated_delivery_fee / 100).toFixed(0)}起`
                        : '',
                    tags: combo.tags || [],
                    is_online: true // 推荐API只返回上架套餐
                }));
                if (reset) {
                    this.setData({
                        packages: packageViewModels,
                        hasMore: result.has_more
                    });
                }
                else {
                    this.setData({
                        packages: [...this.data.packages, ...packageViewModels],
                        hasMore: result.has_more
                    });
                }
            }
            catch (error) {
                logger_1.logger.error('加载套餐失败', error, 'Takeout.loadPackages');
                throw error;
            }
        });
    },
    onTabCategoryChange(e) {
        const { id } = e.detail;
        this.setData({
            activeCategoryId: id,
            page: 1 // 重置页码
        });
        // 切换标签时重新加载数据（带标签过滤）
        this.loadData();
    },
    onAddCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.detail;
            const dish = this.data.dishes.find((d) => d.id === id);
            if (dish) {
                const success = yield cart_1.default.addItem({
                    merchantId: String(dish.merchantId),
                    dishId: String(dish.id)
                });
                if (success) {
                    yield this.updateCartDisplay();
                    wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 });
                }
            }
        });
    },
    /**
     * 套餐加购物车
     */
    onAddComboToCart(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, merchantId } = e.currentTarget.dataset;
            if (!id || !merchantId) {
                wx.showToast({ title: '套餐信息错误', icon: 'error' });
                return;
            }
            const success = yield cart_1.default.addItem({
                merchantId: String(merchantId),
                comboId: String(id)
            });
            if (success) {
                yield this.updateCartDisplay();
                wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 });
            }
        });
    },
    updateCartDisplay() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            try {
                console.log('[updateCartDisplay] 开始调用API');
                // 直接从后端获取最新购物车汇总，确保数据准确
                const userCarts = yield (0, cart_2.getUserCarts)('takeout');
                console.log('[updateCartDisplay] API返回:', JSON.stringify(userCarts));
                const totalCount = ((_a = userCarts.summary) === null || _a === void 0 ? void 0 : _a.total_items) || 0;
                const totalPrice = ((_b = userCarts.summary) === null || _b === void 0 ? void 0 : _b.total_amount) || 0;
                console.log('[updateCartDisplay] 设置数据:', { totalCount, totalPrice, activeTab: this.data.activeTab });
                this.setData({
                    cartTotalCount: totalCount,
                    cartTotalPrice: totalPrice
                });
                // 同时更新 globalStore 供其他组件使用
                global_store_1.globalStore.set('cart', {
                    items: [],
                    totalCount: totalCount,
                    totalPrice: totalPrice,
                    totalPriceDisplay: `¥${(totalPrice / 100).toFixed(2)}`
                });
            }
            catch (error) {
                // API 调用失败时重置为 0
                console.error('[updateCartDisplay] 错误:', error);
                logger_1.logger.warn('获取购物车汇总失败', error, 'Takeout.updateCartDisplay');
                this.setData({
                    cartTotalCount: 0,
                    cartTotalPrice: 0
                });
            }
        });
    },
    // ==================== 导航方法 ====================
    /**
       * 点击菜品卡片 - 跳转到菜品详情
       */
    onDishClick(e) {
        const { id } = e.detail;
        navigation_1.default.toDishDetail(id);
    },
    /**
       * 点击商户名称 - 跳转到商户详情
       */
    onMerchantClick(e) {
        const { id } = e.detail;
        navigation_1.default.toRestaurantDetail(id);
    },
    /**
       * 点击套餐卡片 - 暂时提示（后续可跳转到套餐详情）
       */
    onPackageTap(e) {
        const id = e.currentTarget.dataset.id;
        wx.showToast({ title: `套餐ID: ${id}`, icon: 'none' });
        // TODO: Navigation.toComboDetail(id)
    },
    /**
       * 点击购物车 - 跳转到购物车页
       */
    onCheckout() {
        if (this.data.cartTotalCount === 0) {
            wx.showToast({ title: '购物车是空的', icon: 'none' });
            return;
        }
        navigation_1.default.toCart();
    },
    /**
       * 搜索功能 - 内联搜索，直接在当前页面显示结果
       */
    onSearch(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const keyword = ((_a = e.detail.value) === null || _a === void 0 ? void 0 : _a.trim()) || '';
            // 如果关键词为空，恢复原列表
            if (!keyword) {
                this.setData({ searchKeyword: '' });
                this.loadData();
                return;
            }
            this.setData({
                searchKeyword: keyword,
                loading: true
            });
            try {
                yield this.searchInline(keyword);
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
    /**
     * 内联搜索 - 根据当前 Tab 搜索对应内容
     */
    searchInline(keyword) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            const { activeTab } = this.data;
            const app = getApp();
            if (activeTab === 'dishes') {
                // 搜索菜品 - 使用推荐接口的 keyword 参数，返回格式与列表完全一致
                const result = yield (0, dish_2.getRecommendedDishes)({
                    keyword,
                    page: 1,
                    limit: 20,
                    user_latitude: (_a = app.globalData.latitude) !== null && _a !== void 0 ? _a : undefined,
                    user_longitude: (_b = app.globalData.longitude) !== null && _b !== void 0 ? _b : undefined
                });
                // 使用 DishAdapter 转换为卡片展示格式（与列表加载保持一致）
                const adaptedDishes = result.dishes.map((dish) => dish_1.DishAdapter.fromSummaryDTO(dish));
                this.setData({
                    dishes: adaptedDishes,
                    hasMore: result.has_more,
                    page: 1
                });
                if (result.dishes.length === 0) {
                    wx.showToast({ title: '未找到相关菜品', icon: 'none' });
                }
            }
            else if (activeTab === 'restaurants') {
                // 搜索餐厅 - 复用现有 searchMerchants API
                const { searchMerchants } = require('../../api/merchant');
                const result = yield searchMerchants({
                    keyword,
                    page_id: 1,
                    page_size: 20,
                    user_latitude: app.globalData.latitude || undefined,
                    user_longitude: app.globalData.longitude || undefined
                });
                // 转换为与列表一致的展示格式（与 loadRestaurants 保持一致）
                const restaurants = (result || []).map((m) => ({
                    id: m.id,
                    name: m.name,
                    imageUrl: m.logo_url,
                    cuisineType: m.tags ? m.tags.slice(0, 2) : [],
                    avgPrice: 0,
                    avgPriceDisplay: '人均未知',
                    distance: dish_1.DishAdapter.formatDistance(m.distance),
                    address: m.address,
                    businessHoursDisplay: '营业中',
                    availableRooms: 0,
                    availableRoomsBadge: '',
                    tags: m.tags ? m.tags.slice(0, 3) : []
                }));
                this.setData({
                    restaurants,
                    hasMore: false,
                    page: 1
                });
                if (restaurants.length === 0) {
                    wx.showToast({ title: '未找到相关餐厅', icon: 'none' });
                }
            }
            else if (activeTab === 'packages') {
                // 搜索套餐 - 使用推荐接口的 keyword 参数，返回格式与列表完全一致
                const { getRecommendedCombos } = require('../../api/dish');
                const result = yield getRecommendedCombos({
                    keyword,
                    page: 1,
                    limit: 20
                });
                // 转换为与列表一致的展示格式（与 loadPackages 保持一致）
                const combos = result.combos || [];
                const packageViewModels = combos.map((combo) => ({
                    id: combo.id,
                    name: combo.name,
                    description: combo.description || '',
                    price: combo.combo_price,
                    priceDisplay: (combo.combo_price / 100).toFixed(2),
                    original_price: combo.original_price || combo.combo_price,
                    originalPriceDisplay: ((combo.original_price || combo.combo_price) / 100).toFixed(2),
                    image_url: combo.image_url || '',
                    is_online: combo.is_online,
                    // 新增的丰富字段
                    shopName: combo.merchant_name || '',
                    distance: combo.distance,
                    monthlySales: combo.monthly_sales || 0
                }));
                this.setData({
                    packages: packageViewModels,
                    hasMore: result.has_more || false,
                    page: 1
                });
                if (combos.length === 0) {
                    wx.showToast({ title: '未找到相关套餐', icon: 'none' });
                }
            }
        });
    },
    onReachBottom() {
        // 防抖：防止快速滚动触发多次请求
        if (!this.data.hasMore || this.data.loading)
            return;
        // 简单的时间戳防抖
        const now = Date.now();
        if (this._lastLoadTime && now - this._lastLoadTime < 300) {
            return;
        }
        this._lastLoadTime = now;
        // 增加页码 (loadData 会设置 loading: true)
        const nextPage = this.data.page + 1;
        this.setData({ page: nextPage });
        // 加载数据，失败时回滚页码
        this.loadData().catch(() => {
            logger_1.logger.error('加载更多失败，回滚页码', { page: nextPage }, 'Takeout.onReachBottom');
            this.setData({ page: nextPage - 1 });
            wx.showToast({ title: '加载失败，请重试', icon: 'none' });
        });
    },
    _lastLoadTime: 0,
    /**
     * scroll-view 下拉刷新事件处理
     * 在 Skyline 模式下替代 onPullDownRefresh
     */
    onRefresh() {
        return __awaiter(this, void 0, void 0, function* () {
            // 刷新时清空搜索框，恢复推荐列表
            this.setData({ refresherTriggered: true, page: 1, searchKeyword: '' });
            try {
                yield this.loadData();
            }
            finally {
                // 延迟关闭刷新动画，给用户视觉反馈
                setTimeout(() => {
                    this.setData({ refresherTriggered: false });
                }, 300);
            }
        });
    },
    /**
     * 页面下拉刷新事件（WebView 兼容）
     */
    onPullDownRefresh() {
        // 刷新时清空搜索框，恢复推荐列表
        this.setData({ page: 1, searchKeyword: '' });
        this.loadData().then(() => {
            wx.stopPullDownRefresh();
        });
    }
});
