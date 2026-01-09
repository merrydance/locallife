"use strict";
/**
 * 商户菜品管理页面
 * 基于重构后的API接口实现菜品CRUD操作
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
const merchant_dish_combo_management_1 = require("../../../../api/merchant-dish-combo-management");
const merchant_dish_combo_management_2 = require("../../../../api/merchant-dish-combo-management");
const merchant_basic_management_1 = require("../../../../api/merchant-basic-management");
Page({
    data: {
        // 菜品数据
        dishes: [],
        categories: [],
        currentCategoryId: 0,
        // 界面状态
        loading: true,
        refreshing: false,
        // 搜索和筛选
        searchKeyword: '',
        filterStatus: 'all', // all, available, unavailable
        // 选择模式
        selectionMode: false,
        selectedDishes: [],
        // 编辑弹窗
        showEditModal: false,
        editingDish: null,
        editForm: {
            name: '',
            description: '',
            price: '',
            original_price: '',
            category_id: 0,
            category_name: '',
            image_url: ''
        },
        // 库存管理弹窗
        showInventoryModal: false,
        inventoryForm: {
            dish_id: 0,
            dish_name: '',
            current_count: 0,
            adjustment: 0,
            reason: ''
        }
    },
    onLoad() {
        this.initPage();
    },
    onShow() {
        // 页面显示时刷新数据
        this.loadDishes();
    },
    onPullDownRefresh() {
        this.refreshData();
    },
    /**
     * 初始化页面
     */
    initPage() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ loading: true });
                // 加载分类和菜品数据
                yield Promise.all([
                    this.loadCategories(),
                    this.loadDishes()
                ]);
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
     * 加载分类数据
     */
    loadCategories() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 这里需要调用获取分类的接口
                // 暂时使用模拟数据
                const categories = [
                    { id: 0, name: '全部' },
                    { id: 1, name: '热菜' },
                    { id: 2, name: '凉菜' },
                    { id: 3, name: '主食' },
                    { id: 4, name: '饮品' }
                ];
                this.setData({ categories });
            }
            catch (error) {
                console.error('加载分类失败:', error);
            }
        });
    },
    /**
     * 加载菜品数据
     */
    loadDishes() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const { currentCategoryId, searchKeyword, filterStatus } = this.data;
                const params = {
                    page: 1,
                    page_size: 100
                };
                if (currentCategoryId > 0) {
                    params.category_id = currentCategoryId;
                }
                if (searchKeyword) {
                    params.keyword = searchKeyword;
                }
                if (filterStatus !== 'all') {
                    params.is_available = filterStatus === 'available';
                }
                const result = yield (0, merchant_dish_combo_management_1.getDishes)(params);
                this.setData({
                    dishes: result.data
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
     * 刷新数据
     */
    refreshData() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ refreshing: true });
                yield this.loadDishes();
                wx.showToast({
                    title: '刷新成功',
                    icon: 'success'
                });
            }
            catch (error) {
                wx.showToast({
                    title: '刷新失败',
                    icon: 'error'
                });
            }
            finally {
                this.setData({ refreshing: false });
                wx.stopPullDownRefresh();
            }
        });
    },
    /**
     * 切换分类
     */
    switchCategory(e) {
        const categoryId = e.currentTarget.dataset.id;
        this.setData({ currentCategoryId: categoryId });
        this.loadDishes();
    },
    /**
     * 搜索菜品
     */
    onSearchInput(e) {
        const keyword = e.detail.value;
        this.setData({ searchKeyword: keyword });
        // 防抖搜索
        clearTimeout(this.searchTimer);
        this.searchTimer = setTimeout(() => {
            this.loadDishes();
        }, 500);
    },
    /**
     * 筛选状态
     */
    onFilterChange(e) {
        const status = e.detail.value;
        this.setData({ filterStatus: status });
        this.loadDishes();
    },
    /**
     * 切换选择模式
     */
    toggleSelectionMode() {
        const { selectionMode } = this.data;
        this.setData({
            selectionMode: !selectionMode,
            selectedDishes: []
        });
    },
    /**
     * 选择菜品
     */
    toggleDishSelection(e) {
        const dishId = e.currentTarget.dataset.id;
        const { selectedDishes } = this.data;
        const index = selectedDishes.indexOf(dishId);
        if (index > -1) {
            selectedDishes.splice(index, 1);
        }
        else {
            selectedDishes.push(dishId);
        }
        this.setData({ selectedDishes });
    },
    /**
     * 全选/取消全选
     */
    toggleSelectAll() {
        const { dishes, selectedDishes } = this.data;
        const allSelected = selectedDishes.length === dishes.length;
        this.setData({
            selectedDishes: allSelected ? [] : dishes.map(dish => dish.id)
        });
    },
    /**
     * 批量操作
     */
    batchOperation(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const operation = e.currentTarget.dataset.operation;
            const { selectedDishes } = this.data;
            if (selectedDishes.length === 0) {
                wx.showToast({
                    title: '请选择菜品',
                    icon: 'error'
                });
                return;
            }
            try {
                switch (operation) {
                    case 'enable':
                        yield (0, merchant_dish_combo_management_1.batchUpdateDishStatus)(selectedDishes, true);
                        wx.showToast({ title: '批量上架成功', icon: 'success' });
                        break;
                    case 'disable':
                        yield (0, merchant_dish_combo_management_1.batchUpdateDishStatus)(selectedDishes, false);
                        wx.showToast({ title: '批量下架成功', icon: 'success' });
                        break;
                    case 'delete':
                        yield this.batchDeleteDishes(selectedDishes);
                        break;
                }
                this.setData({
                    selectionMode: false,
                    selectedDishes: []
                });
                this.loadDishes();
            }
            catch (error) {
                wx.showToast({
                    title: error.message || '操作失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 批量删除菜品
     */
    batchDeleteDishes(dishIds) {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve, reject) => {
                wx.showModal({
                    title: '确认删除',
                    content: `确定要删除选中的 ${dishIds.length} 个菜品吗？`,
                    success: (res) => __awaiter(this, void 0, void 0, function* () {
                        if (res.confirm) {
                            try {
                                yield Promise.all(dishIds.map(id => (0, merchant_dish_combo_management_1.deleteDish)(id)));
                                wx.showToast({ title: '批量删除成功', icon: 'success' });
                                resolve();
                            }
                            catch (error) {
                                reject(error);
                            }
                        }
                        else {
                            resolve();
                        }
                    })
                });
            });
        });
    },
    /**
     * 切换菜品状态
     */
    toggleDishStatus(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, available } = e.currentTarget.dataset;
            try {
                yield (0, merchant_dish_combo_management_1.updateDishStatus)(id, !available);
                wx.showToast({
                    title: available ? '已下架' : '已上架',
                    icon: 'success'
                });
                this.loadDishes();
            }
            catch (error) {
                wx.showToast({
                    title: error.message || '操作失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 添加菜品
     */
    addDish() {
        const defaultCategory = this.data.categories[1];
        this.setData({
            showEditModal: true,
            editingDish: null,
            editForm: {
                name: '',
                description: '',
                price: '',
                original_price: '',
                category_id: (defaultCategory === null || defaultCategory === void 0 ? void 0 : defaultCategory.id) || 1,
                category_name: (defaultCategory === null || defaultCategory === void 0 ? void 0 : defaultCategory.name) || '',
                image_url: ''
            }
        });
    },
    /**
     * 编辑菜品
     */
    editDish(e) {
        var _a;
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);
        if (dish) {
            this.setData({
                showEditModal: true,
                editingDish: dish,
                editForm: {
                    name: dish.name,
                    description: dish.description,
                    price: dish.price.toString(),
                    original_price: ((_a = dish.original_price) === null || _a === void 0 ? void 0 : _a.toString()) || '',
                    category_id: dish.category_id,
                    category_name: dish.category_name,
                    image_url: dish.image_url
                }
            });
        }
    },
    /**
     * 关闭编辑弹窗
     */
    closeEditModal() {
        this.setData({
            showEditModal: false,
            editingDish: null
        });
    },
    /**
     * 表单输入
     */
    onFormInput(e) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`editForm.${field}`]: value
        });
    },
    /**
     * 选择分类
     */
    onCategoryChange(e) {
        const index = parseInt(e.detail.value);
        // categories.slice(1) 去掉了"全部"，所以索引需要+1
        const category = this.data.categories[index + 1];
        this.setData({
            'editForm.category_id': (category === null || category === void 0 ? void 0 : category.id) || 0,
            'editForm.category_name': (category === null || category === void 0 ? void 0 : category.name) || ''
        });
    },
    /**
     * 选择图片
     */
    chooseImage() {
        wx.chooseImage({
            count: 1,
            sizeType: ['compressed'],
            sourceType: ['album', 'camera'],
            success: (res) => {
                this.uploadImage(res.tempFilePaths[0]);
            }
        });
    },
    /**
     * 上传图片
     */
    uploadImage(filePath) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                wx.showLoading({ title: '上传中...' });
                const result = yield (0, merchant_basic_management_1.uploadImage)(filePath, 'dish');
                this.setData({
                    'editForm.image_url': result.url
                });
                wx.hideLoading();
                wx.showToast({
                    title: '上传成功',
                    icon: 'success'
                });
            }
            catch (error) {
                wx.hideLoading();
                wx.showToast({
                    title: error.message || '上传失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 保存菜品
     */
    saveDish() {
        return __awaiter(this, void 0, void 0, function* () {
            const { editingDish, editForm } = this.data;
            // 表单验证
            if (!editForm.name.trim()) {
                wx.showToast({ title: '请输入菜品名称', icon: 'error' });
                return;
            }
            if (!editForm.price || parseFloat(editForm.price) <= 0) {
                wx.showToast({ title: '请输入正确的价格', icon: 'error' });
                return;
            }
            if (!editForm.image_url) {
                wx.showToast({ title: '请上传菜品图片', icon: 'error' });
                return;
            }
            try {
                wx.showLoading({ title: '保存中...' });
                const dishData = {
                    name: editForm.name.trim(),
                    description: editForm.description.trim(),
                    price: parseFloat(editForm.price),
                    original_price: editForm.original_price ? parseFloat(editForm.original_price) : undefined,
                    category_id: editForm.category_id,
                    image_url: editForm.image_url
                };
                if (editingDish) {
                    // 更新菜品
                    yield (0, merchant_dish_combo_management_1.updateDish)(editingDish.id, dishData);
                    wx.showToast({ title: '更新成功', icon: 'success' });
                }
                else {
                    // 创建菜品
                    yield (0, merchant_dish_combo_management_1.createDish)(dishData);
                    wx.showToast({ title: '添加成功', icon: 'success' });
                }
                this.closeEditModal();
                this.loadDishes();
            }
            catch (error) {
                wx.showToast({
                    title: error.message || '保存失败',
                    icon: 'error'
                });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    /**
     * 删除菜品
     */
    deleteDish(e) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);
        wx.showModal({
            title: '确认删除',
            content: `确定要删除菜品"${dish === null || dish === void 0 ? void 0 : dish.name}"吗？`,
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield (0, merchant_dish_combo_management_1.deleteDish)(dishId);
                        wx.showToast({ title: '删除成功', icon: 'success' });
                        this.loadDishes();
                    }
                    catch (error) {
                        wx.showToast({
                            title: error.message || '删除失败',
                            icon: 'error'
                        });
                    }
                }
            })
        });
    },
    /**
     * 管理库存
     */
    manageInventory(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const dishId = e.currentTarget.dataset.id;
            const dish = this.data.dishes.find(d => d.id === dishId);
            if (!dish)
                return;
            try {
                // 获取当前库存
                const inventory = yield (0, merchant_dish_combo_management_2.getInventory)(dishId);
                this.setData({
                    showInventoryModal: true,
                    inventoryForm: {
                        dish_id: dishId,
                        dish_name: dish.name,
                        current_count: inventory.available_count,
                        adjustment: 0,
                        reason: ''
                    }
                });
            }
            catch (error) {
                wx.showToast({
                    title: error.message || '获取库存失败',
                    icon: 'error'
                });
            }
        });
    },
    /**
     * 关闭库存弹窗
     */
    closeInventoryModal() {
        this.setData({ showInventoryModal: false });
    },
    /**
     * 库存调整输入
     */
    onInventoryInput(e) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`inventoryForm.${field}`]: field === 'adjustment' ? parseInt(value) || 0 : value
        });
    },
    /**
     * 保存库存调整
     */
    saveInventoryAdjustment() {
        return __awaiter(this, void 0, void 0, function* () {
            const { inventoryForm } = this.data;
            if (inventoryForm.adjustment === 0) {
                wx.showToast({ title: '请输入调整数量', icon: 'error' });
                return;
            }
            if (!inventoryForm.reason.trim()) {
                wx.showToast({ title: '请输入调整原因', icon: 'error' });
                return;
            }
            try {
                wx.showLoading({ title: '保存中...' });
                yield (0, merchant_dish_combo_management_2.updateInventory)(inventoryForm.dish_id, {
                    adjustment: inventoryForm.adjustment,
                    reason: inventoryForm.reason.trim()
                });
                wx.showToast({ title: '库存调整成功', icon: 'success' });
                this.closeInventoryModal();
                this.loadDishes();
            }
            catch (error) {
                wx.showToast({
                    title: error.message || '调整失败',
                    icon: 'error'
                });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
