/**
 * 库存管理页面
 * - 左侧：分类列表
 * - 右侧：菜品库存 Grid
 * - 交互：输入库存后，点击保存按钮批量提交
 */
import { InventoryService } from '../../../api/inventory'
import { DishManagementService, DishCategory } from '../../../api/dish'
import { resolveImageURL } from '../../../utils/image-security'
import { logger } from '../../../utils/logger'

const app = getApp<IAppOption>()

interface DishWithInventory {
    id: number
    name: string
    price: number
    image_url: string
    category_id: number
    category_name: string
    is_online: boolean
    inventory: number  // -1 表示无限
}

// 格式化数字为两位
function pad(n: number): string {
    return n < 10 ? '0' + n : '' + n
}

Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,

        // 分类
        categories: [] as DishCategory[],
        activeCategoryId: 'all' as string | number,

        // 菜品数据
        allDishes: [] as DishWithInventory[],
        filteredDishes: [] as DishWithInventory[],

        // 日期
        todayDate: '',

        // 状态
        loading: true,
        saving: false,

        // 修改追踪
        changedDishIds: [] as number[],
        hasChanges: false
    },

    onLoad() {
        // 设置今日日期
        const today = new Date()
        const dateStr = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`
        this.setData({ todayDate: dateStr })

        this.initData()
    },

    onShow() {
        // 如果有未保存的修改，不刷新
        if (!this.data.loading && !this.data.hasChanges) {
            // 可以选择刷新
        }
    },

    async initData() {
        const merchantId = app.globalData.merchantId

        if (merchantId) {
            this.setData({ merchantName: app.globalData.merchantName || '' })
            await this.loadCategories()
            await this.loadDishes()
            await this.loadInventory()  // 加载已保存的库存
            this.setData({ loading: false })
        } else {
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantName: app.globalData.merchantName || '' })
                    await this.loadCategories()
                    await this.loadDishes()
                    await this.loadInventory()  // 加载已保存的库存
                    this.setData({ loading: false })
                }
            }
        }
    },

    async loadCategories() {
        try {
            const categories = await DishManagementService.getDishCategories()
            this.setData({ categories: categories || [] })
        } catch (error) {
            logger.error('加载分类失败', error, 'Inventory')
        }
    },

    async loadDishes() {
        try {
            const result = await DishManagementService.listDishes({
                page_id: 1,
                page_size: 50
            })
            const dishes = result.dishes || []

            // 处理图片 URL 并初始化库存
            const dishesWithInventory: DishWithInventory[] = await Promise.all(
                dishes.map(async (d: any) => {
                    let imageUrl = d.image_url
                    if (imageUrl) {
                        imageUrl = await resolveImageURL(imageUrl)
                    }
                    return {
                        id: d.id,
                        name: d.name,
                        price: d.price,
                        image_url: imageUrl,
                        category_id: d.category_id,
                        category_name: d.category_name || '',
                        is_online: d.is_online,
                        inventory: -1  // 默认无限
                    }
                })
            )

            this.setData({
                allDishes: dishesWithInventory,
                filteredDishes: dishesWithInventory
            })

            // 更新分类数量
            this.updateCategoryCounts()
        } catch (error) {
            logger.error('加载菜品失败', error, 'Inventory')
        }
    },

    // 计算每个分类的菜品数量
    // 加载已保存的库存数据
    async loadInventory() {
        try {
            const inventoryList = await InventoryService.listTodayInventory()
            if (inventoryList && inventoryList.length > 0) {
                const { allDishes } = this.data
                // 将库存数据合并到菜品列表
                const updatedDishes = allDishes.map(dish => {
                    const inv = inventoryList.find(i => i.dish_id === dish.id)
                    if (inv) {
                        return { ...dish, inventory: inv.total_quantity }
                    }
                    return dish
                })
                this.setData({
                    allDishes: updatedDishes,
                    filteredDishes: updatedDishes
                })
                this.filterDishes()
            }
        } catch (error) {
            // 库存加载失败不影响页面显示，静默处理
            console.log('[Inventory] loadInventory failed:', error)
        }
    },

    updateCategoryCounts() {
        const { categories, allDishes } = this.data
        const updatedCategories = categories.map(cat => {
            const count = allDishes.filter(d => d.category_id === cat.id).length
            return { ...cat, dish_count: count }
        })
        this.setData({ categories: updatedCategories })
    },

    // ========== 分类筛选 ==========
    onSelectCategory(e: WechatMiniprogram.TouchEvent) {
        const categoryId = e.currentTarget.dataset.id
        this.setData({ activeCategoryId: categoryId })
        this.filterDishes()
    },

    filterDishes() {
        const { allDishes, activeCategoryId } = this.data
        if (activeCategoryId === 'all') {
            this.setData({ filteredDishes: allDishes })
        } else {
            const filtered = allDishes.filter(d => d.category_id === activeCategoryId)
            this.setData({ filteredDishes: filtered })
        }
    },

    // ========== 库存编辑 ==========
    onInventoryInput(e: WechatMiniprogram.Input) {
        const dishId = e.currentTarget.dataset.dishId as number
        const inputValue = e.detail.value

        // 解析库存值
        let quantity: number
        if (inputValue === '' || inputValue === '无限' || inputValue === '-1') {
            quantity = -1  // 无限库存
        } else {
            quantity = parseInt(inputValue, 10)
            if (isNaN(quantity) || quantity < 0) {
                quantity = -1
            }
        }

        // 更新本地数据
        const { allDishes, changedDishIds } = this.data
        const dish = allDishes.find(d => d.id === dishId)
        if (!dish || dish.inventory === quantity) {
            return
        }

        const updatedDishes = allDishes.map(d =>
            d.id === dishId ? { ...d, inventory: quantity } : d
        )

        // 标记为已修改
        const newChangedIds = changedDishIds.includes(dishId)
            ? changedDishIds
            : [...changedDishIds, dishId]

        this.setData({
            allDishes: updatedDishes,
            changedDishIds: newChangedIds,
            hasChanges: newChangedIds.length > 0
        })
        this.filterDishes()
    },

    // ========== 保存修改 ==========
    async saveChanges() {
        const { allDishes, changedDishIds } = this.data

        if (changedDishIds.length === 0) {
            wx.showToast({ title: '没有修改', icon: 'none' })
            return
        }

        this.setData({ saving: true })

        let successCount = 0
        let failCount = 0
        let lastError = ''

        for (const dishId of changedDishIds) {
            const dish = allDishes.find(d => d.id === dishId)
            if (!dish) continue

            try {
                console.log(`[Inventory] Saving: dishId=${dishId}, inventory=${dish.inventory}`)
                await InventoryService.setInventory(dishId, dish.inventory)
                successCount++
                console.log(`[Inventory] Success: ${dish.name}`)
            } catch (error: any) {
                const errMsg = error?.message || error?.userMessage || JSON.stringify(error)
                console.error(`[Inventory] Failed: ${dish.name}`, errMsg)
                logger.error(`保存库存失败: ${dish.name}`, error, 'Inventory')
                lastError = errMsg
                failCount++
            }
        }

        this.setData({
            saving: false,
            changedDishIds: [],
            hasChanges: false
        })

        if (failCount === 0) {
            wx.showToast({
                title: `已保存 ${successCount} 项`,
                icon: 'success'
            })
        } else {
            // 显示详细错误
            wx.showModal({
                title: `成功 ${successCount}，失败 ${failCount}`,
                content: `错误详情: ${lastError.substring(0, 200)}`,
                showCancel: false
            })
        }
    },

    // ========== 侧边栏 ==========
    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    goBack() {
        wx.navigateBack()
    }
})
