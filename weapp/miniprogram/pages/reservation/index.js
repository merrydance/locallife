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
Object.defineProperty(exports, "__esModule", { value: true });
const logger_1 = require("../../utils/logger");
const search_1 = require("../../api/search");
const global_store_1 = require("../../utils/global-store");
const app = getApp();
Page({
    data: {
        keyword: '',
        itemList: [],
        activeTab: 'room',
        // UI State
        navBarHeight: 88,
        address: '定位中...',
        loading: false,
        hasMore: true,
        page: 1,
        pageSize: 10,
        // Applied Filters (The actual filters used for API calls)
        appliedFilters: {
            guestCount: undefined,
            priceRange: '',
            selectedTime: '',
            date: '',
            startTime: '',
            endTime: ''
        },
        // Filter Popup UI State (Temporary)
        filterVisible: false,
        uiSelectedDate: '',
        uiSelectedTimeSlot: '',
        uiGuestCount: 2,
        uiPriceRange: '',
        // Helpers
        guestOptionsShort: [
            { label: '2人', value: 2 },
            { label: '4人', value: 4 },
            { label: '6人', value: 6 },
            { label: '8人+', value: 8 }
        ],
        priceOptions: [
            { label: '不限', value: '' },
            { label: '¥100以下', value: '0-100' },
            { label: '¥100-300', value: '100-300' },
            { label: '¥300以上', value: '300-9999' }
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
        this.setData({ navBarHeight: global_store_1.globalStore.get('navBarHeight') });
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
    loadItems() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
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
                // Parse price range from APPLIED filters
                let min_price, max_price;
                if (appliedFilters.priceRange) {
                    const parts = appliedFilters.priceRange.split('-');
                    min_price = Number(parts[0]);
                    max_price = Number(parts[1]);
                }
                if (activeTab === 'room') {
                    const hasTimeFilter = appliedFilters.date && appliedFilters.startTime;
                    if (keyword || hasTimeFilter) {
                        // Search Mode
                        const results = yield (0, search_1.searchRooms)({
                            page_id: page,
                            page_size: pageSize,
                            keyword,
                            user_latitude: latitude,
                            user_longitude: longitude,
                            date: appliedFilters.date || new Date().toISOString().split('T')[0],
                            start_time: appliedFilters.startTime || '18:00',
                            end_time: appliedFilters.endTime || '20:00',
                            guest_count: appliedFilters.guestCount || 2,
                            min_price,
                            max_price
                        });
                        newList = results.map((r) => (Object.assign(Object.assign({}, r), { type: 'room' })));
                    }
                    else {
                        // Feed Mode
                        const results = yield (0, search_1.getRecommendedRooms)({
                            page_id: page,
                            limit: pageSize,
                            user_latitude: latitude,
                            user_longitude: longitude,
                            guest_count: appliedFilters.guestCount,
                            min_price,
                            max_price
                        });
                        newList = results.map((r) => (Object.assign(Object.assign({}, r), { type: 'room' })));
                    }
                }
                else {
                    // Restaurant Stream
                    if (keyword) {
                        const { searchMerchants } = require('../../api/search');
                        const results = yield searchMerchants({
                            keyword,
                            page_id: page,
                            page_size: pageSize,
                            user_latitude: latitude,
                            user_longitude: longitude
                        });
                        newList = results.map((m) => (Object.assign(Object.assign({}, m), { type: 'restaurant' })));
                    }
                    else {
                        const results = yield (0, search_1.getRecommendedMerchants)({
                            user_latitude: latitude,
                            user_longitude: longitude,
                            limit: pageSize
                        });
                        newList = results.map((m) => (Object.assign(Object.assign({}, m), { type: 'restaurant' })));
                    }
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
        });
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
        this.setData({ keyword: e.detail.value });
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
    // ==================== Filter Popup ====================
    showFilterPopup() {
        const { appliedFilters } = this.data;
        this.setData({
            filterVisible: true,
            uiGuestCount: appliedFilters.guestCount || 2,
            uiPriceRange: appliedFilters.priceRange,
            uiSelectedDate: appliedFilters.date || this.data.dateOptions[0].value,
            uiSelectedTimeSlot: appliedFilters.startTime || ''
        });
    },
    hideFilterPopup() {
        this.setData({ filterVisible: false });
    },
    onFilterPopupChange(e) {
        this.setData({ filterVisible: e.detail.visible });
    },
    resetFilter() {
        this.setData({
            uiGuestCount: 2,
            uiPriceRange: '',
            uiSelectedDate: this.data.dateOptions[0].value,
            uiSelectedTimeSlot: ''
        });
    },
    applyFilter() {
        const { uiGuestCount, uiPriceRange, uiSelectedDate, uiSelectedTimeSlot } = this.data;
        let date = '', startTime = '', endTime = '';
        if (uiSelectedDate && uiSelectedTimeSlot) {
            date = uiSelectedDate;
            startTime = uiSelectedTimeSlot;
            const [h, m] = startTime.split(':').map(Number);
            endTime = `${h + 2}:${m}`;
        }
        this.setData({
            appliedFilters: {
                guestCount: uiGuestCount,
                priceRange: uiPriceRange,
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
    generateDateOptions() {
        const options = [];
        const today = new Date();
        for (let i = 0; i < 7; i++) {
            const date = new Date(today);
            date.setDate(today.getDate() + i);
            const label = i === 0 ? '今天' : i === 1 ? '明天' : `${date.getMonth() + 1}月${date.getDate()}日`;
            options.push({ label, value: date.toISOString().split('T')[0] });
        }
        this.setData({ dateOptions: options });
    }
});
