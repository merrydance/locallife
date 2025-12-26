/**
 * 堂食点餐菜单页面
 * 基于重构后的API接口实现堂食场景的点餐功能
 */

import { getDishes, getDishCategories } from '../../../api/customer-dish-browsing';
import {
    getCart,
    addToCart,
    updateCartItem,
    removeFromCart,
    calculateCart
} from '../../../api/customer-cart-order';
import { getTableInfo } from '../../../api/customer-reservation';

interface Dish {
    id: number;
    name: string;
    description: string;
    price: number;
    original_price?: number;
    image_url: string;
    category_id: number;
    is_available: boolean;
    sales_count: number;
    rating: number;
    customizations?: any[];
}

interface Category {
    id: number;
    name: string;
    sort_order: number;
}

interface CartItem {
    dish_id: number;
    quantity: number;
    customizations?: any[];
    subtotal: number;
}

Page({
    data: {
        tableId: 0,
        merchantId: 0,
        tableInfo: null as any,

        // 菜品数据
        categories: [] as Category[],
        dishes: [] as Dish[],
        currentCategoryId: 0,

        // 购物车数据
        cart: {
            items: [] as CartItem[],
            total_amount: 0,
            total_quantity: 0
        },

        // 界面状态
        loading: true,
        cartVisible: false,
        selectedDish: null as Dish | null,


    },

    onLoad(options: any) {
        const { table_id, merchant_id } = options;

        if (!table_id || !merchant_id) {
            wx.showToast({
                title: '参数错误',
                icon: 'error'
            });
            wx.navigateBack();
            return;
        }

        this.setData({
            tableId: parseInt(table_id),
            merchantId: parseInt(merchant_id)
        });

        this.initPage();
    },

    /**
     * 初始化页面数据
     */
    async initPage() {
        try {
            this.setData({ loading: true });

            // 并行加载数据
            const [tableInfo, categories, cart] = await Promise.all([
                getTableInfo(this.data.tableId),
                getDishCategories(this.data.merchantId),
                this.loadCart()
            ]);

            this.setData({
                tableInfo,
                categories,
                cart,
                currentCategoryId: categories[0]?.id || 0
            });

            // 加载第一个分类的菜品
            if (categories.length > 0) {
                await this.loadDishes(categories[0].id);
            }

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
     * 加载购物车
     */
    async loadCart() {
        try {
            const cart = await getCart();
            return cart;
        } catch (error) {
            console.warn('加载购物车失败:', error);
            return {
                items: [],
                total_amount: 0,
                total_quantity: 0
            };
        }
    },

    /**
     * 加载菜品列表
     */
    async loadDishes(categoryId: number) {
        try {
            const dishes = await getDishes({
                merchant_id: this.data.merchantId,
                category_id: categoryId,
                page: 1,
                page_size: 100
            });

            this.setData({
                dishes: dishes.data,
                currentCategoryId: categoryId
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
     * 切换分类
     */
    switchCategory(e: any) {
        const categoryId = e.currentTarget.dataset.id;
        this.loadDishes(categoryId);
    },



    /**
     * 查看菜品详情
     */
    viewDishDetail(e: any) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);

        if (dish) {
            this.setData({ selectedDish: dish });
        }
    },

    /**
     * 关闭菜品详情
     */
    closeDishDetail() {
        this.setData({ selectedDish: null });
    },

    /**
     * 添加到购物车
     */
    async addToCart(e: any) {
        const dishId = e.currentTarget.dataset.id;
        const dish = this.data.dishes.find(d => d.id === dishId);

        if (!dish || !dish.is_available) {
            wx.showToast({
                title: '菜品暂不可用',
                icon: 'error'
            });
            return;
        }

        try {
            await addToCart({
                dish_id: dishId,
                quantity: 1,
                order_type: 'dine_in',
                table_id: this.data.tableId
            });

            // 重新加载购物车
            const cart = await this.loadCart();
            this.setData({ cart });

            wx.showToast({
                title: '已添加到购物车',
                icon: 'success'
            });

        } catch (error: any) {
            console.error('添加到购物车失败:', error);
            wx.showToast({
                title: error.message || '添加失败',
                icon: 'error'
            });
        }
    },

    /**
     * 更新购物车商品数量
     */
    async updateCartQuantity(e: any) {
        const { dishId, quantity } = e.currentTarget.dataset;

        try {
            if (quantity <= 0) {
                await removeFromCart(dishId);
            } else {
                await updateCartItem(dishId, { quantity });
            }

            // 重新加载购物车
            const cart = await this.loadCart();
            this.setData({ cart });

        } catch (error: any) {
            console.error('更新购物车失败:', error);
            wx.showToast({
                title: '更新失败',
                icon: 'error'
            });
        }
    },

    /**
     * 显示购物车
     */
    showCart() {
        this.setData({ cartVisible: true });
    },

    /**
     * 隐藏购物车
     */
    hideCart() {
        this.setData({ cartVisible: false });
    },

    /**
     * 去结算
     */
    async goToCheckout() {
        const { cart, tableId, merchantId } = this.data;

        if (cart.items.length === 0) {
            wx.showToast({
                title: '购物车为空',
                icon: 'error'
            });
            return;
        }

        try {
            // 计算订单金额
            const calculation = await calculateCart();

            // 跳转到结算页面
            wx.navigateTo({
                url: `/pages/dine-in/checkout/checkout?table_id=${tableId}&merchant_id=${merchantId}&order_type=dine_in`
            });

        } catch (error: any) {
            console.error('结算失败:', error);
            wx.showToast({
                title: error.message || '结算失败',
                icon: 'error'
            });
        }
    },

    /**
     * 获取购物车中菜品数量
     */
    getCartQuantity(dishId: number): number {
        const item = this.data.cart.items.find(item => item.dish_id === dishId);
        return item ? item.quantity : 0;
    },

    /**
     * 呼叫服务员
     */
    callWaiter() {
        wx.showModal({
            title: '呼叫服务员',
            content: '确定要呼叫服务员吗？',
            success: (res) => {
                if (res.confirm) {
                    // 这里可以调用呼叫服务员的接口
                    wx.showToast({
                        title: '已呼叫服务员',
                        icon: 'success'
                    });
                }
            }
        });
    },

    /**
     * 分享菜单
     */
    onShareAppMessage() {
        const { tableInfo, merchantId } = this.data;

        return {
            title: `${tableInfo?.merchant_name || '餐厅'}的菜单`,
            path: `/pages/dine-in/scan-entry/scan-entry?table_id=${this.data.tableId}`,
            imageUrl: tableInfo?.merchant_logo
        };
    }
});