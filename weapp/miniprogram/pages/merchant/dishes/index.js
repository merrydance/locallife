"use strict";
/**
 * 菜品管理 - 桌面级 SaaS 实现
 * 对齐后端 API：
 * - GET/POST/PUT/DELETE /v1/dishes - 菜品 CRUD
 * - GET/POST/PATCH/DELETE /v1/dishes/categories - 分类管理
 * - POST /v1/dishes/images/upload - 图片上传
 */
Object.defineProperty(exports, "__esModule", { value: true });
const dish_1 = require("../../../api/dish");
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
        selectedDishId: null, // 用于列表高亮，比对比整个对象更快
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
        // 定制选项（完整分组）
        customizationTags: [], // 可用的定制标签
        customizationGroupsDraft: [],
        activeCustomizationGroupIndex: 0,
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
    async initData() {
        // 获取商户信息
        const merchantId = app.globalData.merchantId;
        if (merchantId) {
            await this.loadCategories();
            await this.loadDishes();
            await this.loadAvailableTags();
            await this.loadCustomizationTags();
        }
        else {
            // 等待商户信息
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    await this.loadCategories();
                    await this.loadDishes();
                    await this.loadAvailableTags();
                    await this.loadCustomizationTags();
                }
            };
        }
    },
    // 加载可用标签
    async loadAvailableTags() {
        try {
            const tags = await dish_1.TagService.listDishTags();
            this.setData({ availableTags: tags });
        }
        catch (error) {
            logger_1.logger.error('加载标签失败', error, 'Dishes');
        }
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
    async loadCustomizationTags() {
        try {
            const tags = await dish_1.TagService.listCustomizationTags();
            this.setData({ customizationTags: tags });
        }
        catch (error) {
            logger_1.logger.error('加载定制标签失败', error, 'Dishes');
        }
    },
    async loadCategories() {
        try {
            const result = await dish_1.DishManagementService.getDishCategories();
            const allDishes = this.data.allDishes || [];
            // 计算每个分类的菜品数量
            const categoriesWithCount = result.map(cat => ({
                ...cat,
                dish_count: allDishes.filter(d => d.category_id === cat.id).length
            }));
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
    },
    async loadDishes() {
        this.setData({ loading: true });
        try {
            const response = await dish_1.DishManagementService.listDishes({
                page_id: 1,
                page_size: 50 // 后端限制最大50
            });
            // 处理图片 URL
            const processedDishes = await Promise.all(response.dishes.map(async (dish) => {
                let imageUrl = dish.image_url;
                if (imageUrl) {
                    imageUrl = await (0, image_security_1.resolveImageURL)(imageUrl);
                }
                return {
                    ...dish,
                    image_url: imageUrl,
                    priceDisplay: (0, util_1.formatPriceNoSymbol)(dish.price || 0)
                };
            }));
            this.setData({
                allDishes: processedDishes,
                loading: false
            });
            this.filterDishes();
            // 重新计算分类数量
            await this.loadCategories();
        }
        catch (error) {
            logger_1.logger.error('加载菜品失败', error, 'Dishes');
            this.setData({ loading: false });
        }
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
    async onSelectDish(e) {
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
            customizationGroupsDraft: [],
            activeCustomizationGroupIndex: -1
        });
        // 从API获取完整的菜品信息（包含标签和定制选项）
        let dish = dishFromList;
        try {
            dish = await dish_1.DishManagementService.getDishDetail(dishFromList.id);
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
                imageUrlDisplay = await (0, image_security_1.resolveImageURL)(dish.image_url);
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
        // 回填定制选项分组
        const customizationGroupsDraft = (dish.customization_groups || []).map((group, index) => {
            var _a;
            const options = (group.options || []).map((opt, optIndex) => {
                var _a;
                return ({
                    tag_id: opt.tag_id,
                    tag_name: opt.tag_name,
                    extra_price: opt.extra_price || 0,
                    sort_order: (_a = opt.sort_order) !== null && _a !== void 0 ? _a : optIndex
                });
            });
            return {
                name: group.name,
                is_required: !!group.is_required,
                sort_order: (_a = group.sort_order) !== null && _a !== void 0 ? _a : index,
                options,
                tag_ids: options.map((opt) => opt.tag_id)
            };
        });
        this.setData({
            selectedDish: {
                ...dish,
                category_name: (category === null || category === void 0 ? void 0 : category.name) || dish.category_name || '',
                image_url_display: imageUrlDisplay // 用于显示的完整URL
            },
            isAdding: false,
            categoryPickerIndex: categoryIndex >= 0 ? categoryIndex : 0,
            selectedTagIds: tagIds,
            customizationGroupsDraft,
            activeCustomizationGroupIndex: customizationGroupsDraft.length > 0 ? 0 : -1
        });
    },
    // 加载菜品定制选项 - 保留用于新建菜品后刷新
    async loadDishCustomizations(dishId) {
        try {
            const result = await dish_1.DishManagementService.getDishCustomizations(dishId);
            const customizationGroupsDraft = (result || []).map((group, index) => {
                var _a;
                const options = (group.options || []).map((opt, optIndex) => {
                    var _a;
                    return ({
                        tag_id: opt.tag_id,
                        tag_name: opt.tag_name,
                        extra_price: opt.extra_price || 0,
                        sort_order: (_a = opt.sort_order) !== null && _a !== void 0 ? _a : optIndex
                    });
                });
                return {
                    name: group.name,
                    is_required: !!group.is_required,
                    sort_order: (_a = group.sort_order) !== null && _a !== void 0 ? _a : index,
                    options,
                    tag_ids: options.map((opt) => opt.tag_id)
                };
            });
            this.setData({
                customizationGroupsDraft,
                activeCustomizationGroupIndex: customizationGroupsDraft.length > 0 ? 0 : -1
            });
        }
        catch (error) {
            logger_1.logger.error('加载定制选项失败', error, 'Dishes');
        }
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
    async onUploadImage() {
        try {
            const res = await wx.chooseMedia({
                count: 1,
                mediaType: ['image'],
                sourceType: ['album', 'camera']
            });
            const filePath = res.tempFiles[0].tempFilePath;
            wx.showLoading({ title: '上传中...' });
            // 后端返回相对路径，保存原始路径用于API请求
            const imageUrl = await dish_1.DishManagementService.uploadDishImage(filePath);
            // 转换为完整URL仅用于显示
            const displayUrl = await (0, image_security_1.resolveImageURL)(imageUrl);
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
    async onSaveDish() {
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
                let customizationGroups = undefined;
                if (this.data.customizationGroupsDraft.length > 0) {
                    customizationGroups = this.data.customizationGroupsDraft
                        .filter((group) => group.options && group.options.length > 0)
                        .map((group, groupIndex) => {
                        var _a;
                        return ({
                            name: group.name,
                            is_required: !!group.is_required,
                            sort_order: (_a = group.sort_order) !== null && _a !== void 0 ? _a : groupIndex,
                            options: (group.options || []).map((opt, optIndex) => {
                                var _a;
                                return ({
                                    tag_id: opt.tag_id,
                                    extra_price: opt.extra_price || 0,
                                    sort_order: (_a = opt.sort_order) !== null && _a !== void 0 ? _a : optIndex
                                });
                            })
                        });
                    });
                }
                // 创建菜品（包含标签和定制选项）
                await dish_1.DishManagementService.createDish({
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
                await dish_1.DishManagementService.updateDish(selectedDish.id, {
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
                if (this.data.customizationGroupsDraft.length > 0 || selectedDish.id) {
                    await this.saveDishCustomizations();
                }
                wx.showToast({ title: '保存成功', icon: 'success' });
            }
            this.setData({
                isAdding: false,
                selectedDish: null, // 清除选中状态
                customizationGroupsDraft: [],
                activeCustomizationGroupIndex: -1
            });
            await this.loadDishes();
        }
        catch (error) {
            logger_1.logger.error('保存菜品失败', error, 'Dishes');
            wx.showToast({ title: error.message || '保存失败', icon: 'error' });
        }
        finally {
            this.setData({ saving: false });
        }
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
    async onDeleteDish() {
        const { selectedDish } = this.data;
        if (!selectedDish || !selectedDish.id)
            return;
        const res = await wx.showModal({
            title: '确认删除',
            content: `确定要删除菜品「${selectedDish.name}」吗？此操作不可恢复。`,
            confirmColor: '#ff4d4f'
        });
        if (res.confirm) {
            wx.showLoading({ title: '删除中...' });
            try {
                await dish_1.DishManagementService.deleteDish(selectedDish.id);
                wx.showToast({ title: '已删除', icon: 'success' });
                this.setData({ selectedDish: null });
                await this.loadDishes();
            }
            catch (error) {
                logger_1.logger.error('删除菜品失败', error, 'Dishes');
                wx.showToast({ title: '删除失败', icon: 'error' });
            }
            finally {
                wx.hideLoading();
            }
        }
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
    async onConfirmAddCategory() {
        const { newCategoryName } = this.data;
        const name = newCategoryName === null || newCategoryName === void 0 ? void 0 : newCategoryName.trim();
        if (!name) {
            wx.showToast({ title: '请输入分类名称', icon: 'none' });
            return;
        }
        wx.showLoading({ title: '添加中...' });
        try {
            await dish_1.DishManagementService.createDishCategory({ name });
            this.setData({ newCategoryName: '' });
            await this.loadCategories();
            wx.showToast({ title: '添加成功', icon: 'success' });
        }
        catch (error) {
            logger_1.logger.error('添加分类失败', error, 'Dishes');
            wx.showToast({ title: '添加失败', icon: 'error' });
        }
        finally {
            wx.hideLoading();
        }
    },
    async onDeleteCategory(e) {
        const { id, name } = e.currentTarget.dataset;
        const res = await wx.showModal({
            title: '确认删除',
            content: `确定删除分类「${name}」吗？该分类下的菜品将变为未分类。`,
            confirmColor: '#ff4d4f'
        });
        if (res.confirm) {
            wx.showLoading({ title: '删除中...' });
            try {
                await dish_1.DishManagementService.deleteDishCategory(id);
                await this.loadCategories();
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
    async onBatchOnline() {
        const { selectedDishIds } = this.data;
        if (selectedDishIds.length === 0)
            return;
        const res = await wx.showModal({
            title: '确认上架',
            content: `确定要上架选中的 ${selectedDishIds.length} 个菜品吗？`,
            confirmColor: '#1890ff'
        });
        if (res.confirm) {
            wx.showLoading({ title: '处理中...' });
            try {
                await dish_1.DishManagementService.batchUpdateDishStatus({
                    dish_ids: selectedDishIds,
                    is_online: true
                });
                wx.showToast({ title: '批量上架成功', icon: 'success' });
                this.setData({ selectedDishIds: [] });
                await this.loadDishes();
            }
            catch (error) {
                logger_1.logger.error('批量上架失败', error, 'Dishes');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
            finally {
                wx.hideLoading();
            }
        }
    },
    async onBatchOffline() {
        const { selectedDishIds } = this.data;
        if (selectedDishIds.length === 0)
            return;
        const res = await wx.showModal({
            title: '确认下架',
            content: `确定要下架选中的 ${selectedDishIds.length} 个菜品吗？`,
            confirmColor: '#ff4d4f'
        });
        if (res.confirm) {
            wx.showLoading({ title: '处理中...' });
            try {
                await dish_1.DishManagementService.batchUpdateDishStatus({
                    dish_ids: selectedDishIds,
                    is_online: false
                });
                wx.showToast({ title: '批量下架成功', icon: 'success' });
                this.setData({ selectedDishIds: [] });
                await this.loadDishes();
            }
            catch (error) {
                logger_1.logger.error('批量下架失败', error, 'Dishes');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
            finally {
                wx.hideLoading();
            }
        }
    },
    // ========== 定制选项管理（完整分组） ==========
    onAddCustomizationGroup() {
        const { customizationGroupsDraft } = this.data;
        const nextIndex = customizationGroupsDraft.length + 1;
        const newGroup = {
            name: `分组${nextIndex}`,
            is_required: false,
            sort_order: customizationGroupsDraft.length,
            options: [],
            tag_ids: []
        };
        this.setData({
            customizationGroupsDraft: [...customizationGroupsDraft, newGroup],
            activeCustomizationGroupIndex: customizationGroupsDraft.length
        });
    },
    onRemoveCustomizationGroup(e) {
        const index = Number(e.currentTarget.dataset.index);
        const { customizationGroupsDraft } = this.data;
        if (index < 0 || index >= customizationGroupsDraft.length)
            return;
        const nextGroups = customizationGroupsDraft.filter((_, i) => i !== index);
        this.setData({
            customizationGroupsDraft: nextGroups,
            activeCustomizationGroupIndex: nextGroups.length > 0 ? Math.min(index, nextGroups.length - 1) : -1
        });
    },
    onSelectCustomizationGroup(e) {
        const index = Number(e.currentTarget.dataset.index);
        if (Number.isNaN(index))
            return;
        this.setData({ activeCustomizationGroupIndex: index });
    },
    onCustomizationGroupNameInput(e) {
        const index = Number(e.currentTarget.dataset.index);
        const value = e.detail.value || '';
        if (index < 0)
            return;
        this.setData({
            [`customizationGroupsDraft[${index}].name`]: value
        });
    },
    onCustomizationGroupRequiredChange(e) {
        const index = Number(e.currentTarget.dataset.index);
        const checked = !!e.detail.value;
        if (index < 0)
            return;
        this.setData({
            [`customizationGroupsDraft[${index}].is_required`]: checked
        });
    },
    // 切换定制标签选中状态（作用于当前分组）
    onCustomizationTagToggle(e) {
        const tagId = e.currentTarget.dataset.id;
        const tagName = e.currentTarget.dataset.name;
        const { customizationGroupsDraft, activeCustomizationGroupIndex } = this.data;
        if (activeCustomizationGroupIndex < 0 || activeCustomizationGroupIndex >= customizationGroupsDraft.length) {
            wx.showToast({ title: '请先添加分组', icon: 'none' });
            return;
        }
        const group = customizationGroupsDraft[activeCustomizationGroupIndex];
        const tagIds = new Set(group.tag_ids || []);
        const options = [...(group.options || [])];
        if (!tagIds.has(tagId)) {
            tagIds.add(tagId);
            options.push({
                tag_id: tagId,
                tag_name: tagName,
                extra_price: 0,
                sort_order: options.length
            });
        }
        else {
            tagIds.delete(tagId);
            const nextOptions = options.filter((o) => o.tag_id !== tagId);
            options.length = 0;
            options.push(...nextOptions);
        }
        this.setData({
            [`customizationGroupsDraft[${activeCustomizationGroupIndex}].options`]: options,
            [`customizationGroupsDraft[${activeCustomizationGroupIndex}].tag_ids`]: Array.from(tagIds)
        });
    },
    // 修改定制选项加价
    onCustomizationPriceChange(e) {
        const tagId = e.currentTarget.dataset.tagId;
        const groupIndex = Number(e.currentTarget.dataset.groupIndex);
        const value = e.detail.value;
        const priceInCents = value ? Math.round(parseFloat(value) * 100) : 0;
        const { customizationGroupsDraft } = this.data;
        const group = customizationGroupsDraft[groupIndex];
        if (!group)
            return;
        const optionIndex = (group.options || []).findIndex((o) => o.tag_id === tagId);
        if (optionIndex >= 0) {
            this.setData({
                [`customizationGroupsDraft[${groupIndex}].options[${optionIndex}].extra_price`]: priceInCents
            });
        }
    },
    // 保存定制选项
    async saveDishCustomizations() {
        const { selectedDish, customizationGroupsDraft } = this.data;
        console.log('[DEBUG] saveDishCustomizations 被调用', {
            dishId: selectedDish === null || selectedDish === void 0 ? void 0 : selectedDish.id,
            groupCount: customizationGroupsDraft.length,
            groups: customizationGroupsDraft
        });
        if (!selectedDish || !selectedDish.id) {
            console.log('[DEBUG] saveDishCustomizations 跳过：无有效菜品ID');
            return;
        }
        if (customizationGroupsDraft.length === 0 || customizationGroupsDraft.every((g) => !g.options || g.options.length === 0)) {
            try {
                console.log('[DEBUG] 清空定制选项');
                await dish_1.DishManagementService.setDishCustomizations(selectedDish.id, { groups: [] });
                return true;
            }
            catch (error) {
                logger_1.logger.error('保存定制选项失败', error, 'Dishes');
                throw error;
            }
        }
        const groups = customizationGroupsDraft
            .filter((group) => group.options && group.options.length > 0)
            .map((group, groupIndex) => {
            var _a;
            return ({
                name: group.name,
                is_required: !!group.is_required,
                sort_order: (_a = group.sort_order) !== null && _a !== void 0 ? _a : groupIndex,
                options: (group.options || []).map((opt, optIndex) => {
                    var _a;
                    return ({
                        tag_id: opt.tag_id,
                        extra_price: opt.extra_price || 0,
                        sort_order: (_a = opt.sort_order) !== null && _a !== void 0 ? _a : optIndex
                    });
                })
            });
        });
        console.log('[DEBUG] 保存定制选项', { groups });
        try {
            await dish_1.DishManagementService.setDishCustomizations(selectedDish.id, { groups });
            console.log('[DEBUG] 保存定制选项成功');
            return true;
        }
        catch (error) {
            logger_1.logger.error('保存定制选项失败', error, 'Dishes');
            throw error;
        }
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
    async onAddTag() {
        const { newTagName, tagManagerType } = this.data;
        const name = newTagName.trim();
        if (!name) {
            wx.showToast({ title: '请输入标签名称', icon: 'none' });
            return;
        }
        try {
            await dish_1.TagService.createTag({
                name,
                type: tagManagerType
            });
            wx.showToast({ title: '添加成功', icon: 'success' });
            this.setData({ newTagName: '' });
            // 刷新标签列表
            if (tagManagerType === 'dish') {
                await this.loadAvailableTags();
            }
            else {
                await this.loadCustomizationTags();
            }
        }
        catch (error) {
            logger_1.logger.error('添加标签失败', error, 'Dishes');
            wx.showToast({ title: '添加失败', icon: 'error' });
        }
    },
    async onDeleteTag(e) {
        const { id, name } = e.currentTarget.dataset;
        const { tagManagerType } = this.data;
        wx.showModal({
            title: '确认删除',
            content: `确定要删除标签"${name}"吗？`,
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await dish_1.TagService.deleteTag(id);
                        wx.showToast({ title: '删除成功', icon: 'success' });
                        // 刷新标签列表
                        if (tagManagerType === 'dish') {
                            await this.loadAvailableTags();
                        }
                        else {
                            await this.loadCustomizationTags();
                        }
                    }
                    catch (error) {
                        logger_1.logger.error('删除标签失败', error, 'Dishes');
                        wx.showToast({ title: '删除失败', icon: 'error' });
                    }
                }
            }
        });
    }
});
