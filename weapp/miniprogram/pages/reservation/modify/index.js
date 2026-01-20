"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const reservation_1 = require("../../../api/reservation");
const merchant_1 = require("../../../api/merchant");
const image_1 = require("../../../utils/image");
const util_1 = require("../../../utils/util");
Page({
    data: {
        reservationId: 0,
        reservation: null,
        loading: true,
        hasError: false,
        errorMessage: '',
        navBarHeight: 88,
        categories: [],
        currentCategoryId: 0,
        currentDishes: [],
        dishQtyMap: {},
        dishPriceMap: {},
        comboItems: [],
        orphanItems: [],
        totalCount: 0,
        totalAmountDisplay: '0.00',
        submitting: false
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ reservationId: parseInt(options.id) });
            this.loadData();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 });
    },
    async loadData() {
        var _a;
        this.setData({ loading: true, hasError: false });
        try {
            const reservationId = this.data.reservationId;
            const reservation = await reservation_1.ReservationService.getReservationDetail(reservationId);
            const dishesResponse = await (0, merchant_1.getMerchantDishes)(String(reservation.merchant_id));
            const dishList = (dishesResponse.dishes || []).map((dish) => {
                const id = Number(dish.id);
                return {
                    id,
                    category_id: Number(dish.category_id) || 0,
                    category_name: dish.category_name || '其他',
                    name: dish.name,
                    description: dish.description,
                    price: Number(dish.price) || 0,
                    image_url: (0, image_1.getPublicImageUrl)(dish.image_url || ''),
                    priceDisplay: (0, util_1.formatPriceNoSymbol)(Number(dish.price) || 0),
                    selectedQty: 0
                };
            });
            const dishPriceMap = {};
            dishList.forEach((dish) => {
                dishPriceMap[dish.id] = dish.price;
            });
            const dishQtyMap = {};
            const comboItems = [];
            const orphanItems = [];
            const knownDishIds = new Set(dishList.map((dish) => dish.id));
            (reservation.items || []).forEach((item) => {
                var _a, _b, _c, _d;
                if (item.dish_id) {
                    const dishId = Number(item.dish_id);
                    dishQtyMap[dishId] = (dishQtyMap[dishId] || 0) + (item.quantity || 0);
                    if (!knownDishIds.has(dishId)) {
                        orphanItems.push({
                            dish_id: dishId,
                            name: item.name || '已下架菜品',
                            price: Number((_a = item.unit_price) !== null && _a !== void 0 ? _a : 0),
                            priceDisplay: (0, util_1.formatPriceNoSymbol)(Number((_b = item.unit_price) !== null && _b !== void 0 ? _b : 0)),
                            quantity: item.quantity || 0
                        });
                    }
                }
                if (item.combo_id) {
                    comboItems.push({
                        combo_id: Number(item.combo_id),
                        name: item.name || '套餐',
                        price: Number((_c = item.unit_price) !== null && _c !== void 0 ? _c : 0),
                        priceDisplay: (0, util_1.formatPriceNoSymbol)(Number((_d = item.unit_price) !== null && _d !== void 0 ? _d : 0)),
                        quantity: item.quantity || 0
                    });
                }
            });
            const dishesWithQty = dishList.map((dish) => ({
                ...dish,
                selectedQty: dishQtyMap[dish.id] || 0
            }));
            const categories = [];
            const categoryMap = new Map();
            categories.push({ id: 0, name: '全部', dishes: [...dishesWithQty] });
            dishesWithQty.forEach((dish) => {
                const catId = dish.category_id || 0;
                const catName = dish.category_name || '其他';
                if (!categoryMap.has(catId)) {
                    categoryMap.set(catId, { id: catId, name: catName, dishes: [] });
                }
                categoryMap.get(catId).dishes.push(dish);
            });
            categories.push(...Array.from(categoryMap.values()).sort((a, b) => a.id - b.id));
            const view = {
                ...reservation,
                _timeText: this.formatReservationDateTime(reservation.reservation_date, reservation.reservation_time),
                _guestCount: reservation.guest_count ? `${reservation.guest_count}人` : '--'
            };
            this.setData({
                reservation: view,
                categories,
                currentCategoryId: 0,
                currentDishes: ((_a = categories[0]) === null || _a === void 0 ? void 0 : _a.dishes) || [],
                dishQtyMap,
                dishPriceMap,
                comboItems,
                orphanItems,
                loading: false
            });
            this.updateTotals();
        }
        catch (error) {
            const errMessage = error instanceof Error ? error.message : String(error);
            console.error(error);
            this.setData({
                loading: false,
                hasError: true,
                errorMessage: errMessage || '加载失败'
            });
        }
    },
    formatReservationDateTime(dateStr, timeStr) {
        const datePart = (dateStr || '').trim();
        const timePart = (timeStr || '').trim();
        if (!datePart && !timePart)
            return '--';
        if (datePart && timePart)
            return `${datePart} ${timePart}`;
        if (datePart)
            return datePart;
        return timePart;
    },
    switchCategory(e) {
        const categoryId = Number(e.currentTarget.dataset.id);
        const category = this.data.categories.find((c) => c.id === categoryId);
        this.setData({
            currentCategoryId: categoryId,
            currentDishes: (category === null || category === void 0 ? void 0 : category.dishes) || []
        });
    },
    onIncrease(e) {
        const id = Number(e.currentTarget.dataset.id);
        const type = e.currentTarget.dataset.type || 'dish';
        if (type === 'combo') {
            this.updateComboQty(id, 1);
            return;
        }
        this.updateDishQty(id, 1);
    },
    onDecrease(e) {
        const id = Number(e.currentTarget.dataset.id);
        const type = e.currentTarget.dataset.type || 'dish';
        if (type === 'combo') {
            this.updateComboQty(id, -1);
            return;
        }
        this.updateDishQty(id, -1);
    },
    updateDishQty(dishId, delta) {
        const dishQtyMap = { ...this.data.dishQtyMap };
        const next = (dishQtyMap[dishId] || 0) + delta;
        if (next < 0)
            return;
        dishQtyMap[dishId] = next;
        const categories = this.data.categories.map((cat) => ({
            ...cat,
            dishes: (cat.dishes || []).map((dish) => dish.id === dishId ? { ...dish, selectedQty: next } : dish)
        }));
        const orphanItems = this.data.orphanItems.map((item) => item.dish_id === dishId ? { ...item, quantity: next } : item);
        const currentCategory = categories.find((c) => c.id === this.data.currentCategoryId);
        this.setData({
            dishQtyMap,
            categories,
            currentDishes: (currentCategory === null || currentCategory === void 0 ? void 0 : currentCategory.dishes) || [],
            orphanItems
        });
        this.updateTotals();
    },
    updateComboQty(comboId, delta) {
        const comboItems = this.data.comboItems.map((item) => {
            if (item.combo_id !== comboId)
                return item;
            const next = item.quantity + delta;
            return { ...item, quantity: next < 0 ? 0 : next };
        });
        this.setData({ comboItems });
        this.updateTotals();
    },
    updateTotals() {
        const dishQtyMap = this.data.dishQtyMap;
        const dishPriceMap = this.data.dishPriceMap;
        const orphanPriceMap = {};
        this.data.orphanItems.forEach((item) => {
            orphanPriceMap[item.dish_id] = item.price;
        });
        let totalCount = 0;
        let totalAmount = 0;
        Object.keys(dishQtyMap).forEach((key) => {
            var _a, _b;
            const dishId = Number(key);
            const qty = dishQtyMap[dishId] || 0;
            if (qty <= 0)
                return;
            totalCount += qty;
            const price = (_b = (_a = dishPriceMap[dishId]) !== null && _a !== void 0 ? _a : orphanPriceMap[dishId]) !== null && _b !== void 0 ? _b : 0;
            totalAmount += price * qty;
        });
        this.data.comboItems.forEach((item) => {
            if (item.quantity <= 0)
                return;
            totalCount += item.quantity;
            totalAmount += item.price * item.quantity;
        });
        this.setData({
            totalCount,
            totalAmountDisplay: (0, util_1.formatPriceNoSymbol)(totalAmount)
        });
    },
    async onSubmit() {
        if (this.data.submitting)
            return;
        const items = [];
        Object.keys(this.data.dishQtyMap).forEach((key) => {
            const dishId = Number(key);
            const qty = this.data.dishQtyMap[dishId] || 0;
            if (qty > 0) {
                items.push({ dish_id: dishId, quantity: qty });
            }
        });
        this.data.comboItems.forEach((item) => {
            if (item.quantity > 0) {
                items.push({ combo_id: item.combo_id, quantity: item.quantity });
            }
        });
        if (items.length === 0) {
            wx.showToast({ title: '至少保留一道菜', icon: 'none' });
            return;
        }
        try {
            this.setData({ submitting: true });
            await reservation_1.ReservationService.modifyDishes(this.data.reservationId, items);
            wx.showToast({ title: '修改成功', icon: 'success' });
            setTimeout(() => {
                wx.navigateBack();
            }, 1200);
        }
        catch (error) {
            const errMessage = error instanceof Error ? error.message : String(error);
            wx.showToast({ title: errMessage || '修改失败', icon: 'none' });
        }
        finally {
            this.setData({ submitting: false });
        }
    },
    onRetry() {
        this.loadData();
    }
});
