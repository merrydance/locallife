"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const logger_1 = require("../../utils/logger");
const search_1 = require("../../api/search");
const global_store_1 = require("../../utils/global-store");
const image_1 = require("../../utils/image");
const dish_1 = require("../../adapters/dish");
const app = getApp();
Page({
    data: {
        keyword: '',
        itemList: [],
        activeTab: 'room',
        // UI State
        navBarHeight: 88,
        scrollViewHeight: 600,
        address: '定位中...',
        loading: false,
        hasMore: true,
        page: 1,
        pageSize: 10,
        refresherTriggered: false,
        // Applied Filters (The actual filters used for API calls)
        appliedFilters: {
            guestCount: undefined,
            priceRange: '',
            minSpend: undefined, // 分
            maxSpend: undefined, // 分
            selectedTime: '',
            date: '',
            startTime: '',
            endTime: ''
        },
        // Filter Popup UI State (Temporary)
        filterVisible: false,
        uiSelectedDate: '',
        uiSelectedTimeSlot: '',
        uiGuestCount: 2, // 默认2人
        uiPriceRange: '',
        // Helpers
        guestOptionsShort: [
            { label: '2人', value: 2 },
            { label: '4人', value: 4 },
            { label: '6人', value: 6 },
            { label: '8人+', value: 8 }
        ],
        priceOptions: [
            { label: '不限', value: '', min: undefined, max: undefined },
            { label: '¥100以下', value: '0-100', min: undefined, max: 100 },
            { label: '¥100-300', value: '100-300', min: 100, max: 300 },
            { label: '¥300以上', value: '300-9999', min: 300, max: undefined }
        ],
        // Inline Options
        dateOptions: [],
        timeOptions: [
            { label: '11:00', value: '11:00' }, { label: '11:30', value: '11:30' },
            { label: '12:00', value: '12:00' }, { label: '12:30', value: '12:30' },
            { label: '13:00', value: '13:00' }, { label: '17:00', value: '17:00' },
            { label: '18:00', value: '18:00' }, { label: '19:00', value: '19:00' }
        ],
    },
    onLoad() {
        // 计算导航栏高度和滚动区域高度
        const navBarHeight = global_store_1.globalStore.get('navBarHeight') || 88;
        const windowInfo = wx.getWindowInfo();
        // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
        const scrollViewHeight = windowInfo.windowHeight - navBarHeight;
        this.setData({ navBarHeight, scrollViewHeight });
        this.generateDateOptions();
        const loc = app.globalData.location;
        if (loc && loc.name) {
            this.setData({ address: loc.name });
        }
        else {
            app.getLocation(); // Async
        }
        // Default load (No keyword, no applied filters initially)
        this.loadItems(true);
    },
    onShow() {
        const loc = app.globalData.location;
        if (loc && loc.name && loc.name !== this.data.address) {
            this.setData({ address: loc.name });
        }
    },
    // ==================== Data Loading ====================
    async loadItems(reset = false) {
        var _a, _b;
        if (this.data.loading)
            return;
        this.setData({ loading: true });
        if (reset) {
            this.setData({ page: 1, itemList: [], hasMore: true });
        }
        try {
            const { activeTab, page, pageSize, keyword, appliedFilters } = this.data;
            const latitude = app.globalData.latitude || undefined;
            const longitude = app.globalData.longitude || undefined;
            let newList = [];
            if (activeTab === 'room') {
                // 统一走 search 路由；缺省日期/时段使用默认值
                const effectiveDate = appliedFilters.date || ((_a = this.data.dateOptions[0]) === null || _a === void 0 ? void 0 : _a.value) || this.formatDateLocal(new Date());
                const effectiveTime = appliedFilters.startTime || ((_b = this.data.timeOptions[0]) === null || _b === void 0 ? void 0 : _b.value) || '18:00';
                const results = await (0, search_1.searchRooms)({
                    reservation_date: effectiveDate,
                    reservation_time: effectiveTime,
                    min_capacity: appliedFilters.guestCount,
                    min_minimum_spend: appliedFilters.minSpend,
                    max_minimum_spend: appliedFilters.maxSpend,
                    user_latitude: latitude,
                    user_longitude: longitude,
                    page_id: page,
                    page_size: pageSize
                });
                // 在 TypeScript 中预处理距离和图片
                newList = results.map((r) => ({
                    ...r,
                    type: 'room',
                    primary_image: (0, image_1.getPublicImageUrl)(r.primary_image) || '',
                    distance_display: r.distance !== undefined ? dish_1.DishAdapter.formatDistance(r.distance) : ''
                }));
            }
            else {
                // Restaurant Stream - 与外卖页 loadRestaurants 保持一致的数据格式
                let merchantResults = [];
                if (keyword) {
                    merchantResults = await (0, search_1.searchMerchants)({
                        keyword,
                        page_id: page,
                        page_size: pageSize,
                        user_latitude: latitude,
                        user_longitude: longitude
                    });
                }
                else {
                    const result = await (0, search_1.getRecommendedMerchants)({
                        user_latitude: latitude,
                        user_longitude: longitude,
                        limit: pageSize
                    });
                    merchantResults = result;
                }
                // 转换为与外卖页一致的 ViewModel 格式
                newList = merchantResults.map((m) => ({
                    id: m.id,
                    name: m.name,
                    imageUrl: (0, image_1.getPublicImageUrl)(m.logo_url) || '',
                    cuisineType: m.tags ? m.tags.slice(0, 2) : [],
                    distance: m.distance !== undefined ? dish_1.DishAdapter.formatDistance(m.distance) : '',
                    address: m.address || '',
                    tags: m.tags ? m.tags.slice(0, 3) : [],
                    type: 'restaurant'
                }));
            }
            this.setData({
                itemList: reset ? newList : [...this.data.itemList, ...newList],
                loading: false,
                hasMore: newList.length === pageSize
            });
        }
        catch (error) {
            logger_1.logger.error('Load items failed', error, 'Reservation');
            wx.showToast({ title: '加载失败', icon: 'none' });
            this.setData({ loading: false });
        }
    },
    // ==================== Interactions ====================
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onLocationChange(e) {
        this.loadItems(true);
    },
    onTabChange(e) {
        const { value } = e.detail;
        if (value === this.data.activeTab)
            return;
        this.setData({ activeTab: value });
        this.loadItems(true);
    },
    onSearch(e) {
        var _a;
        const keyword = ((_a = e.detail.value) === null || _a === void 0 ? void 0 : _a.trim()) || '';
        // 如果有搜索词且在包间 tab，切换到餐厅 tab 搜索
        // 因为后端 searchRooms API 不支持关键词搜索
        if (keyword && this.data.activeTab === 'room') {
            this.setData({
                keyword,
                activeTab: 'restaurant'
            });
        }
        else {
            this.setData({ keyword });
        }
        this.loadItems(true);
    },
    onItemTap(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/detail/index?id=${id}` });
    },
    onReachBottom() {
        if (this.data.hasMore) {
            this.setData({ page: this.data.page + 1 });
            this.loadItems(false);
        }
    },
    /**
     * scroll-view 下拉刷新事件处理
     * 在 Skyline 模式下实现下拉刷新
     */
    async onRefresh() {
        this.setData({ refresherTriggered: true, page: 1 });
        try {
            await this.loadItems(true);
        }
        finally {
            setTimeout(() => {
                this.setData({ refresherTriggered: false });
            }, 300);
        }
    },
    // ==================== Filter Popup ====================
    showFilterPopup() {
        var _a;
        const { appliedFilters } = this.data;
        this.setData({
            filterVisible: true,
            uiGuestCount: appliedFilters.guestCount || 2,
            uiPriceRange: appliedFilters.priceRange,
            uiSelectedDate: appliedFilters.date || this.data.dateOptions[0].value,
            uiSelectedTimeSlot: appliedFilters.startTime || ((_a = this.data.timeOptions[0]) === null || _a === void 0 ? void 0 : _a.value) || ''
        });
    },
    hideFilterPopup() {
        this.setData({ filterVisible: false });
    },
    onFilterPopupChange(e) {
        this.setData({ filterVisible: e.detail.visible });
    },
    resetFilter() {
        var _a, _b;
        // 重置为默认值
        this.setData({
            uiGuestCount: 2,
            uiPriceRange: '',
            uiSelectedDate: ((_a = this.data.dateOptions[0]) === null || _a === void 0 ? void 0 : _a.value) || this.formatDateLocal(new Date()),
            uiSelectedTimeSlot: ((_b = this.data.timeOptions[0]) === null || _b === void 0 ? void 0 : _b.value) || '18:00'
        });
    },
    applyFilter() {
        var _a, _b;
        const { uiGuestCount, uiPriceRange, uiSelectedDate, uiSelectedTimeSlot } = this.data;
        const date = uiSelectedDate || ((_a = this.data.dateOptions[0]) === null || _a === void 0 ? void 0 : _a.value) || this.formatDateLocal(new Date());
        const startTime = uiSelectedTimeSlot || ((_b = this.data.timeOptions[0]) === null || _b === void 0 ? void 0 : _b.value) || '18:00';
        const [h, m] = startTime.split(':').map(Number);
        const endTime = `${h + 2}:${m}`;
        const priceOption = this.data.priceOptions.find(p => p.value === uiPriceRange);
        const minSpend = (priceOption === null || priceOption === void 0 ? void 0 : priceOption.min) !== undefined ? priceOption.min * 100 : undefined;
        const maxSpend = (priceOption === null || priceOption === void 0 ? void 0 : priceOption.max) !== undefined ? priceOption.max * 100 : undefined;
        this.setData({
            appliedFilters: {
                guestCount: uiGuestCount,
                priceRange: uiPriceRange,
                minSpend,
                maxSpend,
                selectedTime: (date && startTime) ? `${date} ${startTime}` : '',
                date,
                startTime,
                endTime
            }
        }, () => {
            this.hideFilterPopup();
            this.loadItems(true);
        });
    },
    // ==================== Inline Tags Selection ====================
    onDateTagChange(e) {
        const { value } = e.currentTarget.dataset;
        this.setData({ uiSelectedDate: value });
    },
    onTimeTagChange(e) {
        const { value } = e.currentTarget.dataset;
        this.setData({ uiSelectedTimeSlot: value === this.data.uiSelectedTimeSlot ? '' : value });
    },
    onGuestTagChange(e) {
        const { value } = e.currentTarget.dataset;
        this.setData({ uiGuestCount: value });
    },
    onPriceTagChange(e) {
        const { value } = e.currentTarget.dataset;
        this.setData({ uiPriceRange: value === this.data.uiPriceRange ? '' : value });
    },
    // ==================== Utils ====================
    formatDateLocal(date) {
        const y = date.getFullYear();
        const m = String(date.getMonth() + 1).padStart(2, '0');
        const d = String(date.getDate()).padStart(2, '0');
        return `${y}-${m}-${d}`;
    },
    generateDateOptions() {
        const options = [];
        const today = new Date();
        for (let i = 0; i < 7; i++) {
            const date = new Date(today);
            date.setDate(today.getDate() + i);
            const label = i === 0 ? '今天' : i === 1 ? '明天' : `${date.getMonth() + 1}月${date.getDate()}日`;
            options.push({ label, value: this.formatDateLocal(date) });
        }
        this.setData({ dateOptions: options });
    }
});
