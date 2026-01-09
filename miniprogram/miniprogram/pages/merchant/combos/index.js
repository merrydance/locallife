"use strict";
/**
 * 套餐管理 - 桌面级 SaaS 实现
 * 对齐后端 API：
 * - GET/POST/PUT/DELETE /v1/combos - 套餐 CRUD
 * - PUT /v1/combos/{id}/online - 上下架
 * - POST /v1/combos/{id}/dishes - 添加菜品
 * - DELETE /v1/combos/{id}/dishes/{dish_id} - 移除菜品
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
const dish_1 = require("../../../api/dish");
const logger_1 = require("../../../utils/logger");
const util_1 = require("../../../utils/util");
const app = getApp();
Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,
        // 套餐
        combos: [],
        filteredCombos: [],
        selectedCombo: null,
        // 状态
        loading: true,
        saving: false,
        searchKeyword: '',
        isAdding: false,
        // 菜品选择弹窗
        showDishPicker: false,
        allDishes: [],
        filteredDishes: [],
        dishSearchKeyword: '',
        pickerSelectedIds: [],
        pickerDishCount: 0,
        // 属性标签
        availableTags: [],
        selectedTagIds: [],
        // 计算字段
        originalPrice: ''
    },
    onLoad() {
        this.initData();
    },
    goBack() {
        wx.navigateBack();
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                yield this.loadCombos();
                yield this.loadAllDishes();
                yield this.loadAvailableTags();
            }
            else {
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        yield this.loadCombos();
                        yield this.loadAllDishes();
                        yield this.loadAvailableTags();
                    }
                });
            }
        });
    },
    loadAvailableTags() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const tags = yield dish_1.TagService.listDishTags();
                this.setData({ availableTags: tags });
            }
            catch (error) {
                logger_1.logger.error('加载标签失败', error, 'Combos');
            }
        });
    },
    loadCombos() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                console.log('[Combos] 开始加载套餐列表...');
                const response = yield dish_1.ComboManagementService.listCombos({
                    page_id: 1,
                    page_size: 50
                });
                console.log('[Combos] API 响应:', JSON.stringify(response));
                // 后端返回的是 combo_sets 字段
                const combos = (response.combo_sets || []).map(combo => (Object.assign(Object.assign({}, combo), { comboPriceDisplay: (0, util_1.formatPriceNoSymbol)(combo.combo_price || 0), dishes: [], dish_count: 0 })));
                console.log('[Combos] 加载套餐成功，数量:', combos.length);
                this.setData({
                    combos,
                    filteredCombos: combos,
                    loading: false
                });
            }
            catch (error) {
                console.error('[Combos] 加载套餐失败:', error);
                logger_1.logger.error('加载套餐失败', error, 'Combos');
                this.setData({ loading: false });
            }
        });
    },
    loadAllDishes() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            try {
                const response = yield dish_1.DishManagementService.listDishes({
                    page_id: 1,
                    page_size: 50
                });
                console.log('[Combos] 加载菜品成功，数量:', ((_a = response.dishes) === null || _a === void 0 ? void 0 : _a.length) || 0);
                // 预处理价格
                const dishesWithPrice = (response.dishes || []).map(d => (Object.assign(Object.assign({}, d), { priceDisplay: (0, util_1.formatPriceNoSymbol)(d.price || 0) })));
                this.setData({
                    allDishes: dishesWithPrice,
                    filteredDishes: dishesWithPrice
                });
            }
            catch (error) {
                console.error('[Combos] 加载菜品失败:', error);
                logger_1.logger.error('加载菜品失败', error, 'Combos');
            }
        });
    },
    onSearch(e) {
        const keyword = e.detail.value.toLowerCase();
        this.setData({ searchKeyword: keyword });
        this.filterCombos();
    },
    filterCombos() {
        const { combos, searchKeyword } = this.data;
        if (!searchKeyword) {
            this.setData({ filteredCombos: combos });
            return;
        }
        const filtered = combos.filter(c => c.name.toLowerCase().includes(searchKeyword));
        this.setData({ filteredCombos: filtered });
    },
    onSelectCombo(e) {
        const item = e.currentTarget.dataset.item;
        // 立即设置选中状态，提供即时反馈
        this.setData({
            selectedCombo: item,
            isAdding: false,
            selectedTagIds: []
        });
        // 然后异步加载完整详情
        this.loadComboDetail(item.id);
    },
    loadComboDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            const requestedId = id; // 记录本次请求的套餐ID
            try {
                const detail = yield dish_1.ComboManagementService.getComboDetail(id);
                console.log('[Combos] 套餐详情:', JSON.stringify(detail));
                // 检查是否已经选择了其他套餐（防止旧请求覆盖新选择）
                if (((_a = this.data.selectedCombo) === null || _a === void 0 ? void 0 : _a.id) !== requestedId) {
                    console.log('[loadComboDetail] 已选择其他套餐，忽略旧响应', requestedId);
                    return;
                }
                // 回填已有标签
                const tagIds = (detail.tags || []).map((t) => t.id);
                // 更新完整数据
                this.setData({
                    selectedCombo: Object.assign(Object.assign({}, detail), { dish_count: ((_b = detail.dishes) === null || _b === void 0 ? void 0 : _b.length) || 0 }),
                    selectedTagIds: tagIds
                });
                this.calculateOriginalPrice();
            }
            catch (error) {
                logger_1.logger.error('加载套餐详情失败', error, 'Combos');
            }
        });
    },
    calculateOriginalPrice() {
        const { selectedCombo } = this.data;
        if (!selectedCombo || !selectedCombo.dishes || selectedCombo.dishes.length === 0) {
            this.setData({ originalPrice: '' });
            return;
        }
        // 计算总价：单价 × 数量
        const total = selectedCombo.dishes.reduce((sum, d) => {
            const price = d.dish_price || d.price || 0;
            const qty = d.quantity || 1;
            return sum + (price * qty);
        }, 0);
        this.setData({ originalPrice: (total / 100).toFixed(2) });
    },
    onAddCombo() {
        this.setData({
            isAdding: true,
            selectedCombo: {
                id: 0,
                name: '',
                description: '',
                combo_price: 0,
                is_online: true,
                dishes: [],
                tags: []
            },
            originalPrice: '',
            selectedTagIds: [] // 清空标签选中
        });
    },
    onCancelEdit() {
        this.setData({
            isAdding: false,
            selectedCombo: null,
            originalPrice: ''
        });
    },
    onFieldChange(e) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`selectedCombo.${field}`]: value
        });
    },
    onPriceFieldChange(e) {
        const value = e.detail.value;
        const priceInCents = value ? Math.round(parseFloat(value) * 100) : 0;
        this.setData({
            'selectedCombo.combo_price': priceInCents
        });
    },
    onToggleOnline() {
        const { selectedCombo } = this.data;
        if (!selectedCombo)
            return;
        this.setData({
            'selectedCombo.is_online': !selectedCombo.is_online
        });
    },
    onSaveCombo() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedCombo, isAdding } = this.data;
            if (!selectedCombo)
                return;
            // 验证
            if (!selectedCombo.name || selectedCombo.name.trim().length < 1) {
                wx.showToast({ title: '请输入套餐名称', icon: 'none' });
                return;
            }
            if (!selectedCombo.combo_price || selectedCombo.combo_price < 1) {
                wx.showToast({ title: '请输入有效价格', icon: 'none' });
                return;
            }
            this.setData({ saving: true });
            wx.showLoading({ title: '保存中...' });
            try {
                if (isAdding) {
                    // 创建套餐：传递带数量的菜品列表
                    const dishes = (selectedCombo.dishes || []).map((d) => ({
                        dish_id: d.dish_id || d.id,
                        quantity: d.quantity || 1
                    }));
                    yield dish_1.ComboManagementService.createCombo({
                        name: selectedCombo.name.trim(),
                        description: selectedCombo.description || '',
                        combo_price: selectedCombo.combo_price,
                        is_online: selectedCombo.is_online !== false,
                        dishes: dishes,
                        tag_ids: this.data.selectedTagIds // 包含标签
                    });
                    wx.hideLoading();
                    wx.showToast({ title: '创建成功', icon: 'success', duration: 2000 });
                }
                else {
                    // 更新套餐：传递带数量的菜品列表
                    const dishes = (selectedCombo.dishes || []).map((d) => ({
                        dish_id: d.dish_id || d.id,
                        quantity: d.quantity || 1
                    }));
                    yield dish_1.ComboManagementService.updateCombo(selectedCombo.id, {
                        name: selectedCombo.name.trim(),
                        description: selectedCombo.description || '',
                        combo_price: selectedCombo.combo_price,
                        is_online: selectedCombo.is_online,
                        dishes: dishes,
                        tag_ids: this.data.selectedTagIds // 包含标签
                    });
                    wx.hideLoading();
                    wx.showToast({ title: '保存成功', icon: 'success', duration: 2000 });
                }
                this.setData({
                    isAdding: false,
                    selectedCombo: null,
                    saving: false
                });
                // 延迟刷新列表，让 Toast 有时间显示
                setTimeout(() => {
                    this.loadCombos();
                }, 500);
            }
            catch (error) {
                wx.hideLoading();
                console.error('[Combos] 保存失败:', error);
                logger_1.logger.error('保存套餐失败', error, 'Combos');
                wx.showToast({ title: error.message || '保存失败', icon: 'error', duration: 2000 });
                this.setData({ saving: false });
            }
        });
    },
    onDeleteCombo() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedCombo } = this.data;
            if (!selectedCombo || !selectedCombo.id)
                return;
            const res = yield wx.showModal({
                title: '确认删除',
                content: `确定要删除套餐「${selectedCombo.name}」吗？此操作不可恢复。`,
                confirmColor: '#ff4d4f'
            });
            if (res.confirm) {
                wx.showLoading({ title: '删除中...' });
                try {
                    yield dish_1.ComboManagementService.deleteCombo(selectedCombo.id);
                    wx.hideLoading();
                    wx.showToast({ title: '已删除', icon: 'success' });
                    this.setData({ selectedCombo: null });
                    yield this.loadCombos();
                }
                catch (error) {
                    wx.hideLoading();
                    logger_1.logger.error('删除套餐失败', error, 'Combos');
                    wx.showToast({ title: '删除失败', icon: 'error' });
                }
            }
        });
    },
    // ========== 菜品选择弹窗 ==========
    onOpenDishPicker() {
        const { selectedCombo, allDishes } = this.data;
        // 从当前套餐获取已选菜品及数量（使用后端格式 dish_id）
        const dishQuantityMap = new Map();
        ((selectedCombo === null || selectedCombo === void 0 ? void 0 : selectedCombo.dishes) || []).forEach((d) => {
            // 兼容两种格式：dish_id（后端格式）和 id（前端格式）
            const dishId = d.dish_id || d.id;
            dishQuantityMap.set(dishId, d.quantity || 1);
        });
        const dishesWithQuantity = allDishes.map(d => (Object.assign(Object.assign({}, d), { quantity: dishQuantityMap.get(d.id) || 0 })));
        this.setData({
            showDishPicker: true,
            filteredDishes: dishesWithQuantity,
            dishSearchKeyword: ''
        });
        this.updatePickerDishCount();
    },
    onCloseDishPicker() {
        this.setData({ showDishPicker: false });
    },
    onDishSearch(e) {
        const keyword = e.detail.value.toLowerCase();
        this.setData({ dishSearchKeyword: keyword });
        this.updateFilteredDishes(keyword);
    },
    updateFilteredDishes(keyword = '') {
        const { allDishes, filteredDishes } = this.data;
        // 保留当前的数量设置
        const qtyMap = new Map(filteredDishes.map(d => [d.id, d.quantity || 0]));
        let filtered = allDishes.map(d => (Object.assign(Object.assign({}, d), { quantity: qtyMap.get(d.id) || 0 })));
        if (keyword) {
            filtered = filtered.filter(d => d.name.toLowerCase().includes(keyword));
        }
        this.setData({ filteredDishes: filtered });
    },
    // 点击菜品名称快速添加/移除
    onToggleDish(e) {
        const item = e.currentTarget.dataset.item;
        const { filteredDishes } = this.data;
        const updated = filteredDishes.map(d => (Object.assign(Object.assign({}, d), { quantity: d.id === item.id ? (d.quantity ? 0 : 1) : d.quantity })));
        this.setData({ filteredDishes: updated });
        this.updatePickerDishCount();
    },
    // 增加数量
    onIncreaseDishQty(e) {
        const dishId = e.currentTarget.dataset.id;
        const { filteredDishes } = this.data;
        const updated = filteredDishes.map(d => (Object.assign(Object.assign({}, d), { quantity: d.id === dishId ? (d.quantity || 0) + 1 : d.quantity })));
        this.setData({ filteredDishes: updated });
        this.updatePickerDishCount();
    },
    // 减少数量
    onDecreaseDishQty(e) {
        const dishId = e.currentTarget.dataset.id;
        const { filteredDishes } = this.data;
        const updated = filteredDishes.map(d => (Object.assign(Object.assign({}, d), { quantity: d.id === dishId ? Math.max(0, (d.quantity || 0) - 1) : d.quantity })));
        this.setData({ filteredDishes: updated });
        this.updatePickerDishCount();
    },
    // 更新已选菜品数量统计（优化：直接从 filteredDishes 统计）
    updatePickerDishCount() {
        const { filteredDishes } = this.data;
        const count = filteredDishes.filter(d => (d.quantity || 0) > 0).length;
        this.setData({ pickerDishCount: count });
    },
    onConfirmDishSelection() {
        const { filteredDishes, allDishes, selectedCombo } = this.data;
        if (!selectedCombo)
            return;
        // 合并数量到完整列表
        const qtyMap = new Map(filteredDishes.map(d => [d.id, d.quantity || 0]));
        // 获取所有数量 > 0 的菜品，并转换为后端格式（dish_id, dish_name, dish_price）
        const selectedDishes = allDishes
            .map(d => ({
            dish_id: d.id,
            dish_name: d.name,
            dish_price: d.price,
            dishPriceDisplay: (0, util_1.formatPriceNoSymbol)(d.price || 0),
            dish_image_url: d.image_url,
            quantity: qtyMap.get(d.id) || 0
        }))
            .filter(d => d.quantity > 0);
        this.setData({
            'selectedCombo.dishes': selectedDishes,
            showDishPicker: false
        });
        this.calculateOriginalPrice();
    },
    onRemoveDish(e) {
        const dishId = e.currentTarget.dataset.id;
        const { selectedCombo } = this.data;
        if (!selectedCombo)
            return;
        // 使用 dish_id 字段进行过滤（后端格式）
        const updatedDishes = (selectedCombo.dishes || []).filter((d) => d.dish_id !== dishId);
        this.setData({
            'selectedCombo.dishes': updatedDishes
        });
        this.calculateOriginalPrice();
    },
    // ========== 标签选择 ==========
    onTagToggle(e) {
        const tagId = e.currentTarget.dataset.id;
        const { selectedTagIds } = this.data;
        let newTagIds;
        if (selectedTagIds.includes(tagId)) {
            // 已选中，移除
            newTagIds = selectedTagIds.filter(id => id !== tagId);
        }
        else {
            // 未选中，添加（最多10个）
            if (selectedTagIds.length >= 10) {
                wx.showToast({ title: '最多选择10个标签', icon: 'none' });
                return;
            }
            newTagIds = [...selectedTagIds, tagId];
        }
        this.setData({ selectedTagIds: newTagIds });
    }
});
