"use strict";
/**
 * 菜品管理 - 桌面级 SaaS 实现
 * 对齐后端 API：
 * - GET/POST/PUT/DELETE /v1/dishes - 菜品 CRUD
 * - GET/POST/PATCH/DELETE /v1/dishes/categories - 分类管理
 * - POST /v1/dishes/images/upload - 图片上传
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
const dish_supabase_1 = require("../../../api/dish_supabase");
const image_security_1 = require("../../../utils/image-security");
const logger_1 = require("../../../utils/logger");
const util_1 = require("../../../utils/util");
const app = getApp();
Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,
        // 分类
        categories: [],
        activeCategoryId: 'all',
        // 菜品
        dishes: [],
        allDishes: [],
        selectedDish: null,
        selectedDishId: null, // Used for list highlighting
        // 状态
        loading: true,
        saving: false,
        searchKeyword: '',
        isAdding: false,
        // 弹窗
        showCategoryManager: false,
        showCategorySelector: false,
        newCategoryName: '',
        // 自定义下拉选择器
        showCategoryDropdown: false,
        categoryOptions: [],
        // 标签选择
        availableTags: [],
        selectedTagIds: [],
        // 批量操作
        isMultiSelectMode: false,
        selectedDishIds: [],
        // 定制选项 - 简化版
        customizationTags: [], // 可用的定制标签
        selectedCustomizationTagIds: [], // 已选中的定制标签 ID
        selectedCustomizationOptions: [], // 已选中的定制选项（带加价）
        // 标签管理弹窗
        showTagManager: false,
        tagManagerType: 'dish',
        newTagName: ''
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
            // 获取商户信息
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                yield this.loadCategories();
                yield this.loadDishes();
                yield this.loadAvailableTags();
                yield this.loadCustomizationTags();
            }
            else {
                // 等待商户信息
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        yield this.loadCategories();
                        yield this.loadDishes();
                        yield this.loadAvailableTags();
                        yield this.loadCustomizationTags();
                    }
                });
            }
        });
    },
    // 加载可用标签
    loadAvailableTags() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const tags = yield dish_1.TagService.listDishTags();
                this.setData({ availableTags: tags });
            }
            catch (error) {
                logger_1.logger.error('加载标签失败', error, 'Dishes');
            }
        });
    },
    // 切换标签选中状态
    onTagToggle(e) {
        const tagId = e.currentTarget.dataset.id;
        const { selectedTagIds } = this.data;
        const index = selectedTagIds.indexOf(tagId);
        if (index === -1) {
            // 添加标签
            this.setData({ selectedTagIds: [...selectedTagIds, tagId] });
        }
        else {
            // 移除标签
            const newIds = selectedTagIds.filter(id => id !== tagId);
            this.setData({ selectedTagIds: newIds });
        }
    },
    // 加载定制标签
    loadCustomizationTags() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const tags = yield dish_1.TagService.listCustomizationTags();
                this.setData({ customizationTags: tags });
            }
            catch (error) {
                logger_1.logger.error('加载定制标签失败', error, 'Dishes');
            }
        });
    },
    loadCategories() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield dish_1.DishManagementService.getDishCategories();
                const allDishes = this.data.allDishes || [];
                // 计算每个分类的菜品数量
                const categoriesWithCount = result.map(cat => (Object.assign(Object.assign({}, cat), { dish_count: allDishes.filter(d => d.category_id === cat.id).length })));
                this.setData({
                    categories: [
                        { id: 'all', name: '全部菜品', dish_count: allDishes.length },
                        ...categoriesWithCount
                    ]
                });
                // 更新分类选项数据
                this.updateCategoryOptions();
            }
            catch (error) {
                logger_1.logger.error('加载分类失败', error, 'Dishes');
            }
        });
    },
    loadDishes() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // [Supabase POC] Use DishSupabaseService
                const response = yield dish_supabase_1.DishSupabaseService.listDishes({
                    merchant_id: 'b2c3d4e5-f6a7-4890-9123-4567890abcde', // Hardcoded for POC
                    page_id: 1,
                    page_size: 50
                });
                // 处理图片 URL
                const processedDishes = yield Promise.all(response.dishes.map((dish) => __awaiter(this, void 0, void 0, function* () {
                    let imageUrl = dish.image_url;
                    if (imageUrl) {
                        imageUrl = yield (0, image_security_1.resolveImageURL)(imageUrl);
                    }
                    return Object.assign(Object.assign({}, dish), { image_url: imageUrl, priceDisplay: (0, util_1.formatPriceNoSymbol)(dish.price || 0) });
                })));
                this.setData({
                    allDishes: processedDishes,
                    loading: false
                });
                this.filterDishes();
                // 重新计算分类数量
                yield this.loadCategories();
            }
            catch (error) {
                logger_1.logger.error('加载菜品失败', error, 'Dishes');
                this.setData({ loading: false });
            }
        });
    },
    filterDishes() {
        const { allDishes, activeCategoryId, searchKeyword } = this.data;
        let filtered = allDishes;
        // 按分类筛选
        if (activeCategoryId !== 'all') {
            filtered = filtered.filter(d => d.category_id === activeCategoryId);
        }
        // 按关键词筛选
        if (searchKeyword) {
            const kw = searchKeyword.toLowerCase();
            filtered = filtered.filter(d => d.name.toLowerCase().includes(kw));
        }
        this.setData({ dishes: filtered });
    },
    onCategoryChange(e) {
        const id = e.currentTarget.dataset.id;
        this.setData({ activeCategoryId: id });
        this.filterDishes();
    },
    onSearch(e) {
        this.setData({ searchKeyword: e.detail.value });
        this.filterDishes();
    },
    // 统一处理菜品点击（修复微信小程序不支持动态 bindtap）
    onDishTap(e) {
        console.log('[onDishTap] isMultiSelectMode:', this.data.isMultiSelectMode);
        console.log('[onDishTap] e.currentTarget.dataset:', e.currentTarget.dataset);
        if (this.data.isMultiSelectMode) {
            // 多选模式：切换选中状态
            const dishId = e.currentTarget.dataset.id;
            console.log('[onDishTap] dishId:', dishId, 'type:', typeof dishId);
            const { selectedDishIds } = this.data;
            console.log('[onDishTap] current selectedDishIds:', selectedDishIds);
            const index = selectedDishIds.indexOf(dishId);
            console.log('[onDishTap] indexOf result:', index);
            if (index === -1) {
                const newIds = [...selectedDishIds, dishId];
                console.log('[onDishTap] adding, new array:', newIds);
                this.setData({ selectedDishIds: newIds });
            }
            else {
                const newIds = selectedDishIds.filter((id) => id !== dishId);
                console.log('[onDishTap] removing, new array:', newIds);
                this.setData({ selectedDishIds: newIds });
            }
            console.log('[onDishTap] after setData, selectedDishIds:', this.data.selectedDishIds);
        }
        else {
            // 普通模式：选择编辑
            this.onSelectDish(e);
        }
    },
    onSelectDish(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            const dishFromList = e.currentTarget.dataset.item;
            const { categories } = this.data;
            const requestedDishId = dishFromList.id; // 记录本次请求的菜品ID
            // 第一步：只设置 ID，这是最快的操作，立即给用户视觉反馈
            this.setData({ selectedDishId: requestedDishId });
            // 第二步：设置完整选中状态
            this.setData({
                selectedDish: dishFromList,
                isAdding: false,
                selectedTagIds: [],
                selectedCustomizationTagIds: [],
                selectedCustomizationOptions: []
            });
            // 从API获取完整的菜品信息（包含标签和定制选项）
            let dish = dishFromList;
            try {
                dish = yield dish_1.DishManagementService.getDishDetail(dishFromList.id);
            }
            catch (error) {
                logger_1.logger.error('获取菜品详情失败，使用列表数据', error, 'Dishes');
            }
            // 检查是否已经选择了其他菜品（防止旧请求覆盖新选择）
            if (((_a = this.data.selectedDish) === null || _a === void 0 ? void 0 : _a.id) !== requestedDishId) {
                console.log('[onSelectDish] 已选择其他菜品，忽略旧响应', requestedDishId);
                return;
            }
            // 处理图片URL - 需要转换为完整URL用于显示
            let imageUrlDisplay = dish.image_url;
            if (dish.image_url) {
                try {
                    imageUrlDisplay = yield (0, image_security_1.resolveImageURL)(dish.image_url);
                }
                catch (error) {
                    logger_1.logger.error('解析图片URL失败', error, 'Dishes');
                }
            }
            // 再次检查（图片加载也是异步的）
            if (((_b = this.data.selectedDish) === null || _b === void 0 ? void 0 : _b.id) !== requestedDishId) {
                return;
            }
            // 处理分类数据回填
            const category = categories.find((c) => c.id === dish.category_id);
            const categoryIndex = categories.findIndex((c) => c.id === dish.category_id);
            // 回填已有属性标签
            const tagIds = (dish.tags || []).map((t) => t.id);
            // 回填定制选项 - 直接从 getDishDetail 返回的 customization_groups 中提取
            const customizationOptions = [];
            const customizationTagIds = [];
            if (dish.customization_groups && dish.customization_groups.length > 0) {
                for (const group of dish.customization_groups) {
                    for (const opt of (group.options || [])) {
                        customizationOptions.push({
                            tag_id: opt.tag_id,
                            tag_name: opt.tag_name,
                            extra_price: opt.extra_price || 0,
                            sort_order: opt.sort_order || 0
                        });
                        customizationTagIds.push(opt.tag_id);
                    }
                }
            }
            this.setData({
                selectedDish: Object.assign(Object.assign({}, dish), { category_name: (category === null || category === void 0 ? void 0 : category.name) || dish.category_name || '', image_url_display: imageUrlDisplay // 用于显示的完整URL
                 }),
                isAdding: false,
                categoryPickerIndex: categoryIndex >= 0 ? categoryIndex : 0,
                selectedTagIds: tagIds,
                selectedCustomizationTagIds: customizationTagIds,
                selectedCustomizationOptions: customizationOptions
            });
        });
    },
    // 加载菜品定制选项 - 保留用于新建菜品后刷新
    loadDishCustomizations(dishId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield dish_1.DishManagementService.getDishCustomizations(dishId);
                // 从所有分组中提取选项到扁平列表
                const options = [];
                const tagIds = [];
                for (const group of (result || [])) {
                    for (const opt of (group.options || [])) {
                        options.push({
                            tag_id: opt.tag_id,
                            tag_name: opt.tag_name,
                            extra_price: opt.extra_price || 0,
                            sort_order: opt.sort_order || 0
                        });
                        tagIds.push(opt.tag_id);
                    }
                }
                this.setData({
                    selectedCustomizationTagIds: tagIds,
                    selectedCustomizationOptions: options
                });
            }
            catch (error) {
                logger_1.logger.error('加载定制选项失败', error, 'Dishes');
            }
        });
    },
    onAddDish() {
        const { activeCategoryId, categories } = this.data;
        const categoryId = activeCategoryId === 'all' ? null : activeCategoryId;
        const category = categories.find((c) => c.id === categoryId);
        this.setData({
            isAdding: true,
            selectedDish: {
                id: 0,
                merchant_id: 0,
                name: '',
                description: '',
                image_url: '',
                price: 0,
                member_price: undefined,
                category_id: categoryId,
                category_name: (category === null || category === void 0 ? void 0 : category.name) || '',
                is_online: true,
                is_available: true,
                sort_order: 0,
                prepare_time: 10
            },
            selectedTagIds: [] // 清空标签选择
        });
    },
    onFieldChange(e) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`selectedDish.${field}`]: value
        });
    },
    onPriceFieldChange(e) {
        const { field } = e.currentTarget.dataset;
        const value = e.detail.value;
        // 转换为分
        const priceInCents = value ? Math.round(parseFloat(value) * 100) : 0;
        this.setData({
            [`selectedDish.${field}`]: priceInCents
        });
    },
    onToggleOnline() {
        const { selectedDish } = this.data;
        if (!selectedDish)
            return;
        this.setData({
            'selectedDish.is_online': !selectedDish.is_online
        });
    },
    onUploadImage() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield wx.chooseMedia({
                    count: 1,
                    mediaType: ['image'],
                    sourceType: ['album', 'camera']
                });
                const filePath = res.tempFiles[0].tempFilePath;
                wx.showLoading({ title: '上传中...' });
                // 后端返回相对路径，保存原始路径用于API请求
                const imageUrl = yield dish_1.DishManagementService.uploadDishImage(filePath);
                // 转换为完整URL仅用于显示
                const displayUrl = yield (0, image_security_1.resolveImageURL)(imageUrl);
                this.setData({
                    'selectedDish.image_url': imageUrl, // 原始路径用于API
                    'selectedDish.image_url_display': displayUrl // 完整URL用于显示
                });
                wx.hideLoading();
                wx.showToast({ title: '上传成功', icon: 'success' });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('上传图片失败', error, 'Dishes');
                wx.showToast({ title: '上传失败', icon: 'error' });
            }
        });
    },
    // 提取图片路径：如果是完整URL则提取相对路径
    extractImagePath(url) {
        if (!url)
            return '';
        // 如果包含 http 开头，提取 /uploads/ 后的相对路径
        if (url.startsWith('http')) {
            const match = url.match(/(\/uploads\/[^?]+)/);
            if (match) {
                return match[1];
            }
        }
        return url;
    },
    onSaveDish() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedDish, isAdding } = this.data;
            if (!selectedDish)
                return;
            // 验证
            if (!selectedDish.name || selectedDish.name.trim().length < 1) {
                wx.showToast({ title: '请输入菜品名称', icon: 'none' });
                return;
            }
            // 后端要求 price >= 1 (分)，即至少 0.01 元
            if (!selectedDish.price || selectedDish.price < 1) {
                wx.showToast({ title: '请输入有效价格（至少0.01元）', icon: 'none' });
                return;
            }
            this.setData({ saving: true });
            // 提取正确的图片路径
            const imageUrl = this.extractImagePath(selectedDish.image_url);
            try {
                if (isAdding) {
                    // 构建定制选项分组（如果有选中的定制标签）
                    let customizationGroups = undefined;
                    if (this.data.selectedCustomizationOptions.length > 0) {
                        customizationGroups = [{
                                name: '定制选项',
                                is_required: false,
                                sort_order: 0,
                                options: this.data.selectedCustomizationOptions.map((o, i) => ({
                                    tag_id: o.tag_id,
                                    extra_price: o.extra_price || 0,
                                    sort_order: i
                                }))
                            }];
                    }
                    // 创建菜品（包含标签和定制选项）
                    yield dish_1.DishManagementService.createDish({
                        name: selectedDish.name.trim(),
                        description: selectedDish.description || '',
                        image_url: imageUrl,
                        price: selectedDish.price,
                        member_price: selectedDish.member_price || undefined,
                        category_id: selectedDish.category_id || undefined,
                        is_online: selectedDish.is_online !== false,
                        is_available: selectedDish.is_available !== false,
                        prepare_time: selectedDish.prepare_time || 10,
                        sort_order: selectedDish.sort_order || 0,
                        tag_ids: this.data.selectedTagIds.length > 0 ? this.data.selectedTagIds : undefined,
                        customization_groups: customizationGroups
                    });
                    wx.showToast({ title: '创建成功', icon: 'success' });
                }
                else {
                    // 更新菜品 - 只发送需要更新的字段
                    yield dish_1.DishManagementService.updateDish(selectedDish.id, {
                        name: selectedDish.name.trim(),
                        description: selectedDish.description || '',
                        image_url: imageUrl,
                        price: selectedDish.price,
                        member_price: selectedDish.member_price || undefined,
                        category_id: selectedDish.category_id || undefined,
                        is_online: selectedDish.is_online,
                        is_available: selectedDish.is_available,
                        prepare_time: selectedDish.prepare_time || 10,
                        sort_order: selectedDish.sort_order || 0,
                        tag_ids: this.data.selectedTagIds.length > 0 ? this.data.selectedTagIds : []
                    });
                    // 保存定制选项
                    if (this.data.selectedCustomizationOptions.length > 0 || selectedDish.id) {
                        yield this.saveDishCustomizations();
                    }
                    wx.showToast({ title: '保存成功', icon: 'success' });
                }
                this.setData({
                    isAdding: false,
                    selectedDish: null, // 清除选中状态
                    selectedCustomizationTagIds: [], // 清除定制选项
                    selectedCustomizationOptions: []
                });
                yield this.loadDishes();
            }
            catch (error) {
                logger_1.logger.error('保存菜品失败', error, 'Dishes');
                wx.showToast({ title: error.message || '保存失败', icon: 'error' });
            }
            finally {
                this.setData({ saving: false });
            }
        });
    },
    // 更新分类选项数据
    updateCategoryOptions() {
        const { categories } = this.data;
        // 过滤掉 'all' 选项
        const options = categories.filter((c) => c.id !== 'all');
        this.setData({ categoryOptions: options });
    },
    // 切换下拉选择器显示
    onToggleCategoryDropdown() {
        this.setData({ showCategoryDropdown: !this.data.showCategoryDropdown });
    },
    // 选择分类
    onSelectCategory(e) {
        const category = e.currentTarget.dataset.item;
        if (category) {
            this.setData({
                showCategoryDropdown: false,
                'selectedDish.category_id': category.id,
                'selectedDish.category_name': category.name
            });
        }
    },
    // 阻止事件冒泡
    stopPropagation() {
        // 空函数，仅用于阻止冒泡
    },
    onCancelEdit() {
        // 统一清空选中状态
        this.setData({
            isAdding: false,
            selectedDish: null
        });
    },
    onDeleteDish() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedDish } = this.data;
            if (!selectedDish || !selectedDish.id)
                return;
            const res = yield wx.showModal({
                title: '确认删除',
                content: `确定要删除菜品「${selectedDish.name}」吗？此操作不可恢复。`,
                confirmColor: '#ff4d4f'
            });
            if (res.confirm) {
                wx.showLoading({ title: '删除中...' });
                try {
                    yield dish_1.DishManagementService.deleteDish(selectedDish.id);
                    wx.showToast({ title: '已删除', icon: 'success' });
                    this.setData({ selectedDish: null });
                    yield this.loadDishes();
                }
                catch (error) {
                    logger_1.logger.error('删除菜品失败', error, 'Dishes');
                    wx.showToast({ title: '删除失败', icon: 'error' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    // ========== 分类管理 ==========
    onOpenCategoryManager() {
        this.setData({ showCategoryManager: true });
    },
    onCloseCategoryManager() {
        this.setData({ showCategoryManager: false, newCategoryName: '' });
    },
    onNewCategoryNameChange(e) {
        this.setData({ newCategoryName: e.detail.value });
    },
    onConfirmAddCategory() {
        return __awaiter(this, void 0, void 0, function* () {
            const { newCategoryName } = this.data;
            const name = newCategoryName === null || newCategoryName === void 0 ? void 0 : newCategoryName.trim();
            if (!name) {
                wx.showToast({ title: '请输入分类名称', icon: 'none' });
                return;
            }
            wx.showLoading({ title: '添加中...' });
            try {
                yield dish_1.DishManagementService.createDishCategory({ name });
                this.setData({ newCategoryName: '' });
                yield this.loadCategories();
                wx.showToast({ title: '添加成功', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('添加分类失败', error, 'Dishes');
                wx.showToast({ title: '添加失败', icon: 'error' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onDeleteCategory(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, name } = e.currentTarget.dataset;
            const res = yield wx.showModal({
                title: '确认删除',
                content: `确定删除分类「${name}」吗？该分类下的菜品将变为未分类。`,
                confirmColor: '#ff4d4f'
            });
            if (res.confirm) {
                wx.showLoading({ title: '删除中...' });
                try {
                    yield dish_1.DishManagementService.deleteDishCategory(id);
                    yield this.loadCategories();
                    wx.showToast({ title: '已删除', icon: 'success' });
                }
                catch (error) {
                    logger_1.logger.error('删除分类失败', error, 'Dishes');
                    wx.showToast({ title: '删除失败', icon: 'error' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    // ========== 分类选择器 ==========
    onOpenCategorySelector() {
        this.setData({ showCategorySelector: true });
    },
    onCloseCategorySelector() {
        this.setData({ showCategorySelector: false });
    },
    // ========== 批量操作 ==========
    onToggleMultiSelect() {
        const { isMultiSelectMode } = this.data;
        this.setData({
            isMultiSelectMode: !isMultiSelectMode,
            selectedDishIds: [], // 切换模式时清空选择
            selectedDish: null // 退出编辑状态
        });
    },
    onDishCheck(e) {
        const dishId = e.currentTarget.dataset.id;
        const { selectedDishIds } = this.data;
        const index = selectedDishIds.indexOf(dishId);
        if (index === -1) {
            this.setData({ selectedDishIds: [...selectedDishIds, dishId] });
        }
        else {
            const newIds = selectedDishIds.filter(id => id !== dishId);
            this.setData({ selectedDishIds: newIds });
        }
    },
    onSelectAll() {
        const { dishes, selectedDishIds } = this.data;
        if (selectedDishIds.length === dishes.length) {
            // 取消全选
            this.setData({ selectedDishIds: [] });
        }
        else {
            // 全选
            const allIds = dishes.map(d => d.id);
            this.setData({ selectedDishIds: allIds });
        }
    },
    onBatchOnline() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedDishIds } = this.data;
            if (selectedDishIds.length === 0)
                return;
            const res = yield wx.showModal({
                title: '确认上架',
                content: `确定要上架选中的 ${selectedDishIds.length} 个菜品吗？`,
                confirmColor: '#1890ff'
            });
            if (res.confirm) {
                wx.showLoading({ title: '处理中...' });
                try {
                    yield dish_1.DishManagementService.batchUpdateDishStatus({
                        dish_ids: selectedDishIds,
                        is_online: true
                    });
                    wx.showToast({ title: '批量上架成功', icon: 'success' });
                    this.setData({ selectedDishIds: [] });
                    yield this.loadDishes();
                }
                catch (error) {
                    logger_1.logger.error('批量上架失败', error, 'Dishes');
                    wx.showToast({ title: '操作失败', icon: 'error' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    onBatchOffline() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedDishIds } = this.data;
            if (selectedDishIds.length === 0)
                return;
            const res = yield wx.showModal({
                title: '确认下架',
                content: `确定要下架选中的 ${selectedDishIds.length} 个菜品吗？`,
                confirmColor: '#ff4d4f'
            });
            if (res.confirm) {
                wx.showLoading({ title: '处理中...' });
                try {
                    yield dish_1.DishManagementService.batchUpdateDishStatus({
                        dish_ids: selectedDishIds,
                        is_online: false
                    });
                    wx.showToast({ title: '批量下架成功', icon: 'success' });
                    this.setData({ selectedDishIds: [] });
                    yield this.loadDishes();
                }
                catch (error) {
                    logger_1.logger.error('批量下架失败', error, 'Dishes');
                    wx.showToast({ title: '操作失败', icon: 'error' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    // ========== 定制选项管理 (简化版) ==========
    // 切换定制标签选中状态
    onCustomizationTagToggle(e) {
        const tagId = e.currentTarget.dataset.id;
        const tagName = e.currentTarget.dataset.name;
        const { selectedCustomizationTagIds, selectedCustomizationOptions } = this.data;
        const index = selectedCustomizationTagIds.indexOf(tagId);
        if (index === -1) {
            // 添加标签
            const newOption = {
                tag_id: tagId,
                tag_name: tagName,
                extra_price: 0,
                sort_order: selectedCustomizationOptions.length
            };
            this.setData({
                selectedCustomizationTagIds: [...selectedCustomizationTagIds, tagId],
                selectedCustomizationOptions: [...selectedCustomizationOptions, newOption]
            });
        }
        else {
            // 移除标签
            const newTagIds = selectedCustomizationTagIds.filter((id) => id !== tagId);
            const newOptions = selectedCustomizationOptions.filter((o) => o.tag_id !== tagId);
            this.setData({
                selectedCustomizationTagIds: newTagIds,
                selectedCustomizationOptions: newOptions
            });
        }
    },
    // 修改定制选项加价
    onCustomizationPriceChange(e) {
        const tagId = e.currentTarget.dataset.tagId;
        const value = e.detail.value;
        const priceInCents = value ? Math.round(parseFloat(value) * 100) : 0;
        const { selectedCustomizationOptions } = this.data;
        const index = selectedCustomizationOptions.findIndex((o) => o.tag_id === tagId);
        if (index >= 0) {
            this.setData({
                [`selectedCustomizationOptions[${index}].extra_price`]: priceInCents
            });
        }
    },
    // 保存定制选项 - 简化版
    saveDishCustomizations() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedDish, selectedCustomizationOptions } = this.data;
            console.log('[DEBUG] saveDishCustomizations 被调用', {
                dishId: selectedDish === null || selectedDish === void 0 ? void 0 : selectedDish.id,
                optionsCount: selectedCustomizationOptions.length,
                options: selectedCustomizationOptions
            });
            if (!selectedDish || !selectedDish.id) {
                console.log('[DEBUG] saveDishCustomizations 跳过：无有效菜品ID');
                return;
            }
            // 如果没有选中任何定制选项，清空定制
            if (selectedCustomizationOptions.length === 0) {
                try {
                    console.log('[DEBUG] 清空定制选项');
                    yield dish_1.DishManagementService.setDishCustomizations(selectedDish.id, { groups: [] });
                    return true;
                }
                catch (error) {
                    logger_1.logger.error('保存定制选项失败', error, 'Dishes');
                    throw error;
                }
            }
            // 创建单一默认分组存储所有选项
            const groups = [{
                    name: '定制选项',
                    is_required: false,
                    sort_order: 0,
                    options: selectedCustomizationOptions.map((o, i) => ({
                        tag_id: o.tag_id,
                        extra_price: o.extra_price || 0,
                        sort_order: i
                    }))
                }];
            console.log('[DEBUG] 保存定制选项', { groups });
            try {
                yield dish_1.DishManagementService.setDishCustomizations(selectedDish.id, { groups });
                console.log('[DEBUG] 保存定制选项成功');
                return true;
            }
            catch (error) {
                logger_1.logger.error('保存定制选项失败', error, 'Dishes');
                throw error;
            }
        });
    },
    // ========== 标签管理 ==========
    onOpenTagManager(e) {
        const type = e.currentTarget.dataset.type || 'dish';
        this.setData({
            showTagManager: true,
            tagManagerType: type,
            newTagName: ''
        });
    },
    onCloseTagManager() {
        this.setData({
            showTagManager: false,
            newTagName: ''
        });
    },
    onNewTagNameInput(e) {
        this.setData({ newTagName: e.detail.value });
    },
    onAddTag() {
        return __awaiter(this, void 0, void 0, function* () {
            const { newTagName, tagManagerType } = this.data;
            const name = newTagName.trim();
            if (!name) {
                wx.showToast({ title: '请输入标签名称', icon: 'none' });
                return;
            }
            try {
                yield dish_1.TagService.createTag({
                    name,
                    type: tagManagerType
                });
                wx.showToast({ title: '添加成功', icon: 'success' });
                this.setData({ newTagName: '' });
                // 刷新标签列表
                if (tagManagerType === 'dish') {
                    yield this.loadAvailableTags();
                }
                else {
                    yield this.loadCustomizationTags();
                }
            }
            catch (error) {
                logger_1.logger.error('添加标签失败', error, 'Dishes');
                wx.showToast({ title: '添加失败', icon: 'error' });
            }
        });
    },
    onDeleteTag(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, name } = e.currentTarget.dataset;
            const { tagManagerType } = this.data;
            wx.showModal({
                title: '确认删除',
                content: `确定要删除标签"${name}"吗？`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield dish_1.TagService.deleteTag(id);
                            wx.showToast({ title: '删除成功', icon: 'success' });
                            // 刷新标签列表
                            if (tagManagerType === 'dish') {
                                yield this.loadAvailableTags();
                            }
                            else {
                                yield this.loadCustomizationTags();
                            }
                        }
                        catch (error) {
                            logger_1.logger.error('删除标签失败', error, 'Dishes');
                            wx.showToast({ title: '删除失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    }
});
