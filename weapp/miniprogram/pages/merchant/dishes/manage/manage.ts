/**
 * 商户菜品管理页面
 * 基于重构后的API接口实现菜品CRUD操作
 */

import {
    getDishes,
    createDish,
    updateDish,
    deleteDish,
    updateDishStatus,
    batchUpdateDishStatus
} from '../../../../api/merchant-dish-combo-management';
import {
    getInventory,
    updateInventory
} from '../../../../api/merchant-dish-combo-management';
import { uploadImage } from '../../../../api/merchant-basic-management';

interface Dish {
    id: number;
    name: string;
    description: string;
    price: number;
    original_price?: number;
    image_url: string;
    category_id: number;
    category_name: string;
    is_available: boolean;
    sales_count: number;
    rating: number;
    inventory_count?: number;
    created_at: string;
    updated_at: string;
}

interface Category {
    id: number;
    name: string;
}

Page({
    data: {
        // 菜品数据
        dishes: [] as Dish[],
        categories: [] as Category[],
        currentCategoryId: 0,

        // 界面状态
        loading: true,
        refreshing: false,

        // 搜索和筛选
        searchKeyword: '',
        filterStatus: 'all', // all, available, unavailable

        // 选择模式
        selectionMode: false,
        selectedDishes: [] as number[],

        // 编辑弹窗
        showEditModal: false,
        editingDish: null as Dish | null,
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
    async initPage() {
        try {
            this.setData({ loading: true });

            // 加载分类和菜品数据
            await Promise.all([
                this.loadCategories(),
                this.loadDishes()
            ]);

        } catch (error: any) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            this.setData({ loading: false });
        }
    },

    /**
     * 加载分类数据
     */
    async loadCategories() {
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
        } catch (error) {
            console.error('加载分类失败:', error);
        }
    },

    /**
     * 加载菜品数据
     */
    async loadDishes() {
        try {
            const { currentCategoryId, searchKeyword, filterStatus } = this.data;

            const params: any = {
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

            const result = await getDishes(params);

            this.setData({
                dishes: result.data
            });

        } catch (error: any) {
            console.error('加载菜品失败:', error);
            wx.showToast({
                title: '加载菜品失败',
                icon: 'error'
            });
        }
    },

    /**
     * 刷新数据
     */
    async refreshData() {
        try {
            this.setData({ refreshing: true });
            await this.loadDishes();
            wx.showToast({
                title: '刷新成功',
                icon: 'success'
            });
        } catch (error) {
            wx.showToast({
                title: '刷新失败',
                icon: 'error'
            });
        } finally {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        }
    },

    /**
     * 切换分类
     */
    switchCategory(e: any) {
        const categoryId = e.currentTarget.dataset.id;
        this.setData({ currentCategoryId: categoryId });
        this.loadDishes();
    },

    /**
     * 搜索菜品
     */
    onSearchInput(e: any) {
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
    onFilterChange(e: any) {
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
    toggleDishSelection(e: any) {
        const dishId = e.currentTarget.dataset.id;
        const { selectedDishes } = this.data;

        const index = selectedDishes.indexOf(dishId);
        if (index > -1) {
            selectedDishes.splice(index, 1);
        } else {
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
    async batchOperation(e: any) {
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
                    await batchUpdateDishStatus(selectedDishes, true);
                    wx.showToast({ title: '批量上架成功', icon: 'success' });
                    break;
                case 'disable':
                    await batchUpdateDishStatus(selectedDishes, false);
                    wx.showToast({ title: '批量下架成功', icon: 'success' });
                    break;
                case 'delete':
                    await this.batchDeleteDishes(selectedDishes);
                    break;
            }

            this.setData({
                selectionMode: false,
                selectedDishes: []
            });
            this.loadDishes();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
    },

    /**
     * 批量删除菜品
     */
    async batchDeleteDishes(dishIds: number[]) {
        return new Promise<void>((resolve, reject) => {
            wx.showModal({
                title: '确认删除',
                content: `确定要删除选中的 ${dishIds.length} 个菜品吗？`,
                success: async (res) => {
                    if (res.confirm) {
                        try {
                            await Promise.all(dishIds.map(id => deleteDish(id)));
                            wx.showToast({ title: '批量删除成功', icon: 'success' });
                            resolve();
                        } catch (error) {
                            reject(error);
                        }
                    } else {
                        resolve();
                    }
                }
            });
        });
    },

    /**
     * 切换菜品状态
     */
    async toggleDishStatus(e: any) {
        const { id, available } = e.currentTarget.dataset;

        try {
            await updateDishStatus(id, !available);
            wx.showToast({
                title: available ? '已下架' : '已上架',
                icon: 'success'
            });
            this.loadDishes();
        } catch (error: any) {
            wx.showToast({
                title: error.message || '操作失败',
                icon: 'error'
            });
        }
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
                category_id: defaultCategory?.id || 1,
                category_name: defaultCategory?.name || '',
                image_url: ''
            }
        });
    },

    /**
     * 编辑菜品
     */
    editDish(e: any) {
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
                    original_price: dish.original_price?.toString() || '',
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
    onFormInput(e: any) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;

        this.setData({
            [`editForm.${field}`]: value
        });
    },

    /**
     * 选择分类
     */
    onCategoryChange(e: any) {
        const index = parseInt(e.detail.value);
        // categories.slice(1) 去掉了"全部"，所以索引需要+1
        const category = this.data.categories[index + 1];
        this.setData({
            'editForm.category_id': category?.id || 0,
            'editForm.category_name': category?.name || ''
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
    async uploadImage(filePath: string) {
        try {
            wx.showLoading({ title: '上传中...' });

            const result = await uploadImage(filePath, 'dish');

            this.setData({
                'editForm.image_url': result.url
            });

            wx.hideLoading();
            wx.showToast({
                title: '上传成功',
                icon: 'success'
            });

        } catch (error: any) {
            wx.hideLoading();
            wx.showToast({
                title: error.message || '上传失败',
                icon: 'error'
            });
        }
    },

    /**
     * 保存菜品
     */
    async saveDish() {
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
                await updateDish(editingDish.id, dishData);
                wx.showToast({ title: '更新成功', icon: 'success' });
            } else {
                // 创建菜品
                await createDish(dishData);
                wx.showToast({ title: '添加成功', icon: 'success' });
            }

            this.closeEditModal();
            this.loadDishes();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '保存失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 删除菜品
     */
    deleteDish(e: any) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);

        wx.showModal({
            title: '确认删除',
            content: `确定要删除菜品"${dish?.name}"吗？`,
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await deleteDish(dishId);
                        wx.showToast({ title: '删除成功', icon: 'success' });
                        this.loadDishes();
                    } catch (error: any) {
                        wx.showToast({
                            title: error.message || '删除失败',
                            icon: 'error'
                        });
                    }
                }
            }
        });
    },

    /**
     * 管理库存
     */
    async manageInventory(e: any) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);

        if (!dish) return;

        try {
            // 获取当前库存
            const inventory = await getInventory(dishId);

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

        } catch (error: any) {
            wx.showToast({
                title: error.message || '获取库存失败',
                icon: 'error'
            });
        }
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
    onInventoryInput(e: any) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;

        this.setData({
            [`inventoryForm.${field}`]: field === 'adjustment' ? parseInt(value) || 0 : value
        });
    },

    /**
     * 保存库存调整
     */
    async saveInventoryAdjustment() {
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

            await updateInventory(inventoryForm.dish_id, {
                adjustment: inventoryForm.adjustment,
                reason: inventoryForm.reason.trim()
            });

            wx.showToast({ title: '库存调整成功', icon: 'success' });
            this.closeInventoryModal();
            this.loadDishes();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '调整失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    }
});