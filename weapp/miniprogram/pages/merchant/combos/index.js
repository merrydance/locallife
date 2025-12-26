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
            }
            else {
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        yield this.loadCombos();
                        yield this.loadAllDishes();
                    }
                });
            }
        });
    },
    loadCombos() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const response = yield dish_1.ComboManagementService.listCombos({
                    page_id: 1,
                    page_size: 50
                });
                // 获取每个套餐的详情以填充菜品数量
                const combosWithDetails = yield Promise.all(response.combos.map((combo) => __awaiter(this, void 0, void 0, function* () {
                    var _a;
                    try {
                        const detail = yield dish_1.ComboManagementService.getComboDetail(combo.id);
                        return Object.assign(Object.assign({}, detail), { dish_count: ((_a = detail.dishes) === null || _a === void 0 ? void 0 : _a.length) || 0 });
                    }
                    catch (_b) {
                        return Object.assign(Object.assign({}, combo), { dishes: [], dish_count: 0 });
                    }
                })));
                this.setData({
                    combos: combosWithDetails,
                    filteredCombos: combosWithDetails,
                    loading: false
                });
            }
            catch (error) {
                logger_1.logger.error('加载套餐失败', error, 'Combos');
                this.setData({ loading: false });
            }
        });
    },
    loadAllDishes() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield dish_1.DishManagementService.listDishes({
                    page_id: 1,
                    page_size: 200
                });
                this.setData({
                    allDishes: response.dishes,
                    filteredDishes: response.dishes
                });
            }
            catch (error) {
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
        this.loadComboDetail(item.id);
    },
    loadComboDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            try {
                const detail = yield dish_1.ComboManagementService.getComboDetail(id);
                this.setData({
                    selectedCombo: Object.assign(Object.assign({}, detail), { dish_count: ((_a = detail.dishes) === null || _a === void 0 ? void 0 : _a.length) || 0 }),
                    isAdding: false
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
        if (!selectedCombo || !selectedCombo.dishes) {
            this.setData({ originalPrice: '' });
            return;
        }
        const total = selectedCombo.dishes.reduce((sum, d) => sum + (d.price || 0), 0);
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
                dishes: []
            },
            originalPrice: ''
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
            var _a;
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
            try {
                if (isAdding) {
                    // 创建套餐
                    const dishIds = ((_a = selectedCombo.dishes) === null || _a === void 0 ? void 0 : _a.map((d) => d.id)) || [];
                    yield dish_1.ComboManagementService.createCombo({
                        name: selectedCombo.name.trim(),
                        description: selectedCombo.description || '',
                        combo_price: selectedCombo.combo_price,
                        is_online: selectedCombo.is_online !== false,
                        dish_ids: dishIds
                    });
                    wx.showToast({ title: '创建成功', icon: 'success' });
                }
                else {
                    // 更新套餐
                    yield dish_1.ComboManagementService.updateCombo(selectedCombo.id, {
                        name: selectedCombo.name.trim(),
                        description: selectedCombo.description || '',
                        combo_price: selectedCombo.combo_price,
                        is_online: selectedCombo.is_online
                    });
                    wx.showToast({ title: '保存成功', icon: 'success' });
                }
                this.setData({
                    isAdding: false,
                    selectedCombo: null
                });
                yield this.loadCombos();
            }
            catch (error) {
                logger_1.logger.error('保存套餐失败', error, 'Combos');
                wx.showToast({ title: error.message || '保存失败', icon: 'error' });
            }
            finally {
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
                    wx.showToast({ title: '已删除', icon: 'success' });
                    this.setData({ selectedCombo: null });
                    yield this.loadCombos();
                }
                catch (error) {
                    logger_1.logger.error('删除套餐失败', error, 'Combos');
                    wx.showToast({ title: '删除失败', icon: 'error' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    // ========== 菜品选择弹窗 ==========
    onOpenDishPicker() {
        const { selectedCombo, allDishes } = this.data;
        const selectedIds = new Set(((selectedCombo === null || selectedCombo === void 0 ? void 0 : selectedCombo.dishes) || []).map((d) => d.id));
        // 标记已选中的菜品
        const dishesWithSelection = allDishes.map(d => (Object.assign(Object.assign({}, d), { selected: selectedIds.has(d.id) })));
        this.setData({
            showDishPicker: true,
            filteredDishes: dishesWithSelection,
            dishSearchKeyword: ''
        });
    },
    onCloseDishPicker() {
        this.setData({ showDishPicker: false });
    },
    onDishSearch(e) {
        const keyword = e.detail.value.toLowerCase();
        this.setData({ dishSearchKeyword: keyword });
        const { allDishes, selectedCombo } = this.data;
        const selectedIds = new Set(((selectedCombo === null || selectedCombo === void 0 ? void 0 : selectedCombo.dishes) || []).map((d) => d.id));
        let filtered = allDishes.map(d => (Object.assign(Object.assign({}, d), { selected: selectedIds.has(d.id) })));
        if (keyword) {
            filtered = filtered.filter(d => d.name.toLowerCase().includes(keyword));
        }
        this.setData({ filteredDishes: filtered });
    },
    onSelectDish(e) {
        const item = e.currentTarget.dataset.item;
        const { filteredDishes } = this.data;
        const updated = filteredDishes.map(d => (Object.assign(Object.assign({}, d), { selected: d.id === item.id ? !d.selected : d.selected })));
        this.setData({ filteredDishes: updated });
    },
    onConfirmDishSelection() {
        const { filteredDishes, selectedCombo, allDishes } = this.data;
        // 合并选中状态到所有菜品
        const selectionMap = new Map(filteredDishes.map(d => [d.id, d.selected]));
        const allWithSelection = allDishes.map(d => (Object.assign(Object.assign({}, d), { selected: selectionMap.has(d.id) ? selectionMap.get(d.id) : false })));
        // 获取所有选中的菜品
        const selectedDishes = allWithSelection.filter(d => d.selected);
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
        const updatedDishes = (selectedCombo.dishes || []).filter((d) => d.id !== dishId);
        this.setData({
            'selectedCombo.dishes': updatedDishes
        });
        this.calculateOriginalPrice();
    }
});
