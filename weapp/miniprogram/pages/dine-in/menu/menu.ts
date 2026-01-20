/**
 * 堂食点餐/预订点菜菜单页面
 * 支持三种入口：
 * 1. 页面跳转：直接传 table_id 和 merchant_id
 * 2. 扫描小程序码：scene 参数格式 m=商户ID&t=桌号
 * 3. 预订点菜：直接传 reservation_id 和 merchant_id
 */

import { scanTable, getTableDetail, ScanTableResponse, ScanTableCategoryInfo, ScanTableMerchantInfo, ScanTableTableInfo, ScanTableComboInfo, ScanTablePromotionInfo } from '../../../api/table'
import {
    getCart,
    addToCart,
    updateCartItem,
    removeFromCart,
    calculateCart,
    CartResponse,
    CartItemResponse
} from '../../../api/cart'
import { getReservationDetail } from '../../../api/reservation'
import { getMerchantDishes } from '../../../api/merchant'
import type { DishResponse, CustomizationGroup, CustomizationOption } from '../../../api/dish'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'
import { getStableBarHeights } from '../../../utils/responsive'

type MenuDish = DishResponse & {
    image_url: string
    priceDisplay: string
    memberPriceDisplay: string | null
    hasCustomizations: boolean
    cartQty: number
}

type MenuCategory = {
    id: number
    name: string
    sort_order?: number
    dishes: MenuDish[]
}

type CartItemView = CartItemResponse & {
    priceDisplay: string
    subtotalDisplay: string
}

type CartView = CartResponse & {
    total_quantity: number
    subtotalDisplay: string
    items: CartItemView[]
}

type MerchantInfoView = ScanTableMerchantInfo | { id: number; name: string; logo_url?: string }
type TableInfoView = ScanTableTableInfo | { table_no: string }

type DrawerDish = MenuDish & {
    spec_groups?: Array<{
        id: string
        name: string
        is_required: boolean
        specs: Array<{
            id: string
            name: string
            price_diff: number
            priceDiffDisplay: string | null
        }>
    }>
}

Page({
    data: {
        tableId: 0,
        merchantId: 0,
        tableNo: '',
        navBarHeight: 64,

        // 预订点菜场景
        reservationId: 0,
        orderType: 'dine_in' as 'dine_in' | 'reservation',

        // 商户和桌台信息
        merchantInfo: null as MerchantInfoView | null,
        tableInfo: null as TableInfoView | null,

        // 菜品数据
        categories: [] as MenuCategory[],
        combos: [] as ScanTableComboInfo[],
        promotions: [] as ScanTablePromotionInfo[],
        currentCategoryId: 0,
        currentDishes: [] as MenuDish[],

        // 购物车数据
        cart: null as CartResponse | null,
        cartTotal: 0,
        cartCount: 0,

        // 界面状态
        loading: true,
        cartVisible: false,
        selectedDish: null as DishResponse | null,

        // 定制 Drawer 状态
        drawerVisible: false,
        drawerDish: null as DrawerDish | null,
        drawerSpecs: {} as Record<string, string>,
        drawerQty: 1,

        // 错误状态
        hasError: false,
        errorMessage: ''
    },

    onLoad(options: { reservation_id?: string; merchant_id?: string; table_id?: string; scene?: string }) {
        // 设置导航栏高度
        const { navBarHeight } = getStableBarHeights()
        this.setData({ navBarHeight })

        let tableId: number | null = null
        let merchantId: number | null = null
        let tableNo: string | null = null

        // 方式0: 预订点菜入口 (从预订详情页跳转)
        if (options.reservation_id && options.merchant_id) {
            const reservationId = parseInt(options.reservation_id)
            merchantId = parseInt(options.merchant_id)
            this.setData({
                reservationId,
                merchantId,
                orderType: 'reservation'
            })
            this.initPageForReservation(reservationId, merchantId)
            return
        }

        // 方式1: 直接参数 (从页面跳转)
        if (options.table_id && options.merchant_id) {
            tableId = parseInt(options.table_id)
            merchantId = parseInt(options.merchant_id)
            this.setData({ tableId, merchantId, orderType: 'dine_in' })
            this.initPageById(tableId, merchantId)
            return
        }

        // 方式2: scene参数 (从小程序码扫描)
        // scene格式: m_商户ID-t_桌号 或 tid_桌台ID
        if (options.scene) {
            const scene = decodeURIComponent(options.scene)

            // 解析新格式: m_1-t_A01
            const mMatch = scene.match(/m_(\d+)/)
            const tMatch = scene.match(/t_([^-]+)/)
            const tidMatch = scene.match(/tid_(\d+)/)

            if (mMatch && tMatch) {
                merchantId = parseInt(mMatch[1])
                tableNo = tMatch[1]
                this.setData({ merchantId, tableNo, orderType: 'dine_in' })
                this.initPageByTableNo(merchantId, tableNo)
                return
            } else if (tidMatch) {
                tableId = parseInt(tidMatch[1])
                this.setData({ tableId, orderType: 'dine_in' })
                this.showError('暂不支持此扫码格式')
                return
            }
        }

        // 参数错误 - 显示友好提示
        this.showError('请通过扫描桌台二维码进入点餐页面')
    },
    /**
     * 显示错误状态
     */
    showError(message: string) {
        this.setData({
            loading: false,
            hasError: true,
            errorMessage: message
        })
    },

    /**
     * 返回上一页
     */
    goBack() {
        const pages = getCurrentPages()
        if (pages.length > 1) {
            wx.navigateBack()
        } else {
            wx.switchTab({ url: '/pages/index/index' })
        }
    },

    /**
     * 通过桌台ID和商户ID初始化页面
     */
    async initPageById(tableId: number, merchantId: number) {
        // 暂时用 initPageByTableNo 的方式，需要查询桌号
        // 后续可以优化为直接用 tableId
        wx.showToast({ title: '加载中...', icon: 'loading' })
        this.setData({ loading: true })

        try {
            const tableDetail = await getTableDetail(tableId)

            if (tableDetail && tableDetail.table_no) {
                await this.initPageByTableNo(merchantId, tableDetail.table_no)
            } else {
                throw new Error('无法获取桌台信息')
            }
        } catch (error) {
            console.error('初始化失败:', error)
            wx.showToast({ title: '加载失败', icon: 'error' })
            this.setData({ loading: false })
        }
    },

    /**
     * 预订点菜初始化（从预订详情页跳转）
     */
    async initPageForReservation(reservationId: number, merchantId: number) {
        try {
            this.setData({ loading: true })
            wx.showLoading({ title: '加载菜单...' })

            // 并行获取预订详情、商户信息和菜品列表
            const { getPublicMerchantDetail } = require('../../../api/merchant')
            const [reservation, merchantDetail, dishesResponse] = await Promise.all([
                getReservationDetail(reservationId),
                getPublicMerchantDetail(merchantId),
                getMerchantDishes(String(merchantId))
            ])

            // 从预订详情提取桌号（预订必须有桌台）
            const tableNo = reservation.table_no
            if (!tableNo) {
                throw new Error('预订信息缺少桌台号')
            }

            // 从响应中提取菜品列表，并预处理价格、图片和定制标志
            const dishes: MenuDish[] = (dishesResponse.dishes || []).map((dish) => {
                const dishData = dish as unknown as DishResponse
                return {
                    ...dishData,
                    image_url: getPublicImageUrl(dishData.image_url || ''),
                    priceDisplay: formatPriceNoSymbol(dishData.price || 0),
                    memberPriceDisplay: dishData.member_price ? formatPriceNoSymbol(dishData.member_price) : null,
                    hasCustomizations: Array.isArray(dishData.customization_groups) && dishData.customization_groups.length > 0,
                    cartQty: 0
                }
            })

            // 按分类整理菜品
            const finalCategories: MenuCategory[] = []
            const categoryMap = new Map<number, MenuCategory>()

            // 添加"全部"分类
            finalCategories.push({ id: 0, name: '全部', sort_order: -1, dishes: [...dishes] })

            dishes.forEach((dish) => {
                const catId = dish.category_id || 0
                const catName = dish.category_name || '其他'
                if (!categoryMap.has(catId)) {
                    categoryMap.set(catId, { id: catId, name: catName, dishes: [] })
                }
                categoryMap.get(catId)!.dishes.push(dish)
            })

            // 合并其他分类
            const otherCategories = Array.from(categoryMap.values()).sort((a, b) => a.id - b.id)
            finalCategories.push(...otherCategories)

            // 从商户详情获取商户名
            const merchantName = merchantDetail.name
            if (!merchantName) {
                throw new Error('无法获取商户信息')
            }

            this.setData({
                reservationId,
                merchantId,
                tableNo,
                merchantInfo: {
                    id: merchantId,
                    name: merchantName
                },
                tableInfo: {
                    table_no: tableNo
                },
                categories: finalCategories,
                currentCategoryId: 0,
                currentDishes: dishes,
                loading: false
            })

            // 设置页面标题
            wx.setNavigationBarTitle({ title: merchantName })

            // 加载购物车
            await this.loadCart()

            wx.hideLoading()
        } catch (error) {
            wx.hideLoading()
            console.error('预订初始化失败:', error)
            wx.showToast({
                title: error instanceof Error ? error.message : '加载失败',
                icon: 'error'
            })
        } finally {
            this.setData({ loading: false })
        }
    },

    /**
     * 通过桌号初始化页面（扫码场景）
     */
    async initPageByTableNo(merchantId: number, tableNo: string) {
        try {
            this.setData({ loading: true })
            wx.showLoading({ title: '加载菜单...' })

            // 调用扫码API获取完整信息
            const scanResult = await scanTable(merchantId, tableNo)

            // 预处理菜品价格、图片和定制标志
            const allDishes: MenuDish[] = []
            const processedCategories: MenuCategory[] = (scanResult.categories || []).map((cat: ScanTableCategoryInfo) => {
                const dishes = (cat.dishes || []).map((dish: DishResponse) => {
                    const processedDish: MenuDish = {
                        ...dish,
                        image_url: getPublicImageUrl(dish.image_url || ''),
                        priceDisplay: formatPriceNoSymbol(dish.price || 0),
                        memberPriceDisplay: dish.member_price ? formatPriceNoSymbol(dish.member_price) : null,
                        hasCustomizations: Array.isArray(dish.customization_groups) && dish.customization_groups.length > 0,
                        cartQty: 0
                    }
                    allDishes.push(processedDish)
                    return processedDish
                })
                return { id: cat.id, name: cat.name, sort_order: cat.sort_order, dishes }
            })

            // 添加"全部"分类
            const finalCategories: MenuCategory[] = [
                { id: 0, name: '全部', sort_order: -1, dishes: allDishes },
                ...processedCategories
            ]

            // 设置桌台和商户信息
            this.setData({
                tableId: scanResult.table.id,
                merchantId: scanResult.merchant.id,
                tableNo: scanResult.table.table_no,
                merchantInfo: scanResult.merchant,
                tableInfo: scanResult.table,
                categories: finalCategories,
                combos: scanResult.combos || [],
                promotions: scanResult.promotions || [],
                currentCategoryId: 0,
                currentDishes: allDishes
            })

            // 设置页面标题
            wx.setNavigationBarTitle({ title: scanResult.merchant.name })

            // 加载购物车
            await this.loadCart()

            wx.hideLoading()
        } catch (error) {
            wx.hideLoading()
            console.error('扫码初始化失败:', error)
            wx.showToast({
                title: error instanceof Error ? error.message : '加载失败',
                icon: 'error'
            })
        } finally {
            this.setData({ loading: false })
        }
    },

    /**
     * 加载购物车
     */
    async loadCart() {
        try {
            const cart = await getCart({
                merchant_id: this.data.merchantId,
                order_type: this.data.orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })
            this.applyCartData(cart)
        } catch (error) {
            console.warn('加载购物车失败:', error)
            this.setData({
                cart: null,
                cartTotal: 0,
                cartCount: 0
            })
        }
    },

    /**
     * 延迟同步购物车（合并多次点击，避免频繁全量更新）
     */
    scheduleCartSync() {
        const page = this as WechatMiniprogram.Page.Instance<WechatMiniprogram.IAnyObject, WechatMiniprogram.IAnyObject> & { _cartSyncTimer?: number }
        if (page._cartSyncTimer) {
            clearTimeout(page._cartSyncTimer)
        }
        page._cartSyncTimer = setTimeout(async () => {
            try {
                const cart = await getCart({
                    merchant_id: this.data.merchantId,
                    order_type: this.data.orderType,
                    table_id: this.data.tableId || undefined,
                    reservation_id: this.data.reservationId || undefined
                }, { loading: false })
                this.applyCartData(cart)
            } catch (error) {
                // 忽略后台同步失败，避免打断操作
            }
        }, 300) as unknown as number
    },

    /**
     * 应用购物车数据并同步菜品数量（避免整页重绘）
     */
    applyCartData(cart: CartResponse) {
        // 预处理购物车价格，添加 total_quantity 别名
        const processedCart: CartView = {
            ...cart,
            total_quantity: cart.total_count || 0,
            subtotalDisplay: formatPriceNoSymbol(cart.subtotal || 0),
            items: (cart.items || []).map((item: CartItemResponse) => ({
                ...item,
                image_url: getPublicImageUrl(item.image_url),
                priceDisplay: formatPriceNoSymbol(item.unit_price || 0),
                subtotalDisplay: formatPriceNoSymbol(item.subtotal || (item.unit_price || 0) * (item.quantity || 1))
            }))
        }

        // 构建菜品ID到购物车数量的映射
        const cartQtyMap = new Map<number, number>()
        for (const item of processedCart.items) {
            if (item.dish_id) {
                cartQtyMap.set(item.dish_id, (cartQtyMap.get(item.dish_id) || 0) + item.quantity)
            }
        }

        const dataUpdate: WechatMiniprogram.IAnyObject = {
            cart: processedCart,
            cartTotal: cart.subtotal,
            cartCount: cart.total_count,
            totalPrice: cart.subtotal,
            totalCount: cart.total_count
        }

        // 仅更新变化的菜品数量，避免整页重绘导致滚动复位
        this.data.currentDishes.forEach((dish, index) => {
            const nextQty = cartQtyMap.get(dish.id) || 0
            if (dish.cartQty !== nextQty) {
                dataUpdate[`currentDishes[${index}].cartQty`] = nextQty
            }
        })

        this.data.categories.forEach((cat, catIndex) => {
            (cat.dishes || []).forEach((dish, dishIndex) => {
                const nextQty = cartQtyMap.get(dish.id) || 0
                if (dish.cartQty !== nextQty) {
                    dataUpdate[`categories[${catIndex}].dishes[${dishIndex}].cartQty`] = nextQty
                }
            })
        })

        this.setData(dataUpdate)
    },

    /**
     * 仅更新指定菜品的数量显示（避免整页重绘）
     */
    updateDishQtyInLists(dishId: number, nextQty: number) {
        const dataUpdate: WechatMiniprogram.IAnyObject = {}

        this.data.currentDishes.forEach((dish, index) => {
            if (dish.id === dishId && dish.cartQty !== nextQty) {
                dataUpdate[`currentDishes[${index}].cartQty`] = nextQty
            }
        })

        this.data.categories.forEach((cat, catIndex) => {
            (cat.dishes || []).forEach((dish, dishIndex) => {
                if (dish.id === dishId && dish.cartQty !== nextQty) {
                    dataUpdate[`categories[${catIndex}].dishes[${dishIndex}].cartQty`] = nextQty
                }
            })
        })

        if (Object.keys(dataUpdate).length > 0) {
            this.setData(dataUpdate)
        }
    },

    /**
     * 查找无定制的菜品购物车项
     */
    findPlainDishCartItem(dishId: number) {
        return this.data.cart?.items.find((item) => {
            const hasCustomizations = item.customizations && Object.keys(item.customizations).length > 0
            return item.dish_id === dishId && !hasCustomizations
        })
    },

    /**
     * 乐观更新：根据购物车项ID调整数量
     */
    applyOptimisticCartItemChange(itemId: number, nextQty: number) {
        const cart = this.data.cart
        if (!cart) return

        const items = [...(cart.items || [])] as CartItemView[]
        const index = items.findIndex((item) => item.id === itemId)
        if (index < 0) return

        const target = items[index]
        const unitPrice = target.unit_price || (target as any).price || 0
        const prevQty = target.quantity || 0
        const safeNextQty = Math.max(0, nextQty)

        if (safeNextQty <= 0) {
            items.splice(index, 1)
        } else {
            items[index] = {
                ...target,
                quantity: safeNextQty,
                subtotal: unitPrice * safeNextQty,
                subtotalDisplay: formatPriceNoSymbol(unitPrice * safeNextQty)
            } as CartItemView
        }

        const nextTotalCount = Math.max(0, (cart.total_count || 0) - prevQty + safeNextQty)
        const nextSubtotal = Math.max(0, (cart.subtotal || 0) - unitPrice * prevQty + unitPrice * safeNextQty)
        const dataUpdate: WechatMiniprogram.IAnyObject = {
            'cart.items': items,
            'cart.total_count': nextTotalCount,
            'cart.total_quantity': nextTotalCount,
            'cart.subtotal': nextSubtotal,
            'cart.subtotalDisplay': formatPriceNoSymbol(nextSubtotal),
            cartTotal: nextSubtotal,
            cartCount: nextTotalCount,
            totalPrice: nextSubtotal,
            totalCount: nextTotalCount
        }

        this.setData(dataUpdate)

        if (target.dish_id) {
            const nextDishQty = items
                .filter((item) => item.dish_id === target.dish_id)
                .reduce((sum, item) => sum + (item.quantity || 0), 0)
            this.updateDishQtyInLists(target.dish_id, nextDishQty)
        }
    },

    /**
     * 乐观更新：无定制菜品加减（仅更新数量与合计）
     */
    applyOptimisticDishDelta(dishId: number, deltaQty: number) {
        const cart = this.data.cart
        if (!cart) return

        const dish = this.data.currentDishes.find((d) => d.id === dishId)
        const unitPrice = dish?.price || 0
        const currentQty = dish?.cartQty || 0
        const nextQty = Math.max(0, currentQty + deltaQty)

        const item = this.findPlainDishCartItem(dishId)
        if (item) {
            const nextItemQty = Math.max(0, (item.quantity || 0) + deltaQty)
            this.applyOptimisticCartItemChange(item.id, nextItemQty)
        } else {
            const nextTotalCount = Math.max(0, (cart.total_count || 0) + deltaQty)
            const nextSubtotal = Math.max(0, (cart.subtotal || 0) + unitPrice * deltaQty)
            this.setData({
                'cart.total_count': nextTotalCount,
                'cart.total_quantity': nextTotalCount,
                'cart.subtotal': nextSubtotal,
                'cart.subtotalDisplay': formatPriceNoSymbol(nextSubtotal),
                cartTotal: nextSubtotal,
                cartCount: nextTotalCount,
                totalPrice: nextSubtotal,
                totalCount: nextTotalCount
            })
            this.updateDishQtyInLists(dishId, nextQty)
        }
    },

    /**
     * 切换分类
     */
    switchCategory(e: WechatMiniprogram.CustomEvent) {
        const categoryId = e.currentTarget.dataset.id
        const category = this.data.categories.find(c => c.id === categoryId)

        this.setData({
            currentCategoryId: categoryId,
            currentDishes: category?.dishes || []
        })
    },

    /**
     * 查看菜品详情
     */
    viewDishDetail(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const dish = this.data.currentDishes.find(d => d.id === dishId)

        if (dish) {
            this.setData({ selectedDish: dish })
        }
    },

    /**
     * 关闭菜品详情
     */
    closeDishDetail() {
        this.setData({ selectedDish: null })
    },

    /**
     * 更新购物车数量（WXML 事件绑定）
     */
    async updateItemQuantity(e: WechatMiniprogram.CustomEvent) {
        const { itemId, quantity } = e.currentTarget.dataset
        try {
            this.applyOptimisticCartItemChange(itemId, quantity)
            if (quantity <= 0) {
                await removeFromCart(itemId, { loading: false })
            } else {
                await updateCartItem(itemId, { quantity }, { loading: false })
            }
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    /**
     * 显示/隐藏购物车
     */
    toggleCartVisible() {
        this.setData({ cartVisible: !this.data.cartVisible })
    },

    /**
     * 显示购物车
     */
    showCart() {
        this.setData({ cartVisible: true })
    },

    /**
     * 隐藏购物车
     */
    hideCart() {
        this.setData({ cartVisible: false })
    },

    /**
     * 去结算
     */
    async goToCheckout() {
        const { cart, tableId, merchantId, orderType, reservationId } = this.data

        if (!cart || cart.items.length === 0) {
            wx.showToast({ title: '购物车为空', icon: 'none' })
            return
        }

        try {
            // 计算订单金额
            await calculateCart({
                merchant_id: merchantId,
                order_type: orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })

            // 根据订单类型拼接参数
            let url = `/pages/dine-in/checkout/checkout?merchant_id=${merchantId}&order_type=${orderType}`
            if (orderType === 'dine_in') {
                url += `&table_id=${tableId}`
            } else if (orderType === 'reservation') {
                url += `&reservation_id=${reservationId}`
            }

            // 跳转到结算页面
            wx.navigateTo({ url })
        } catch (error) {
            console.error('结算失败:', error)
            wx.showToast({ title: error instanceof Error ? error.message : '结算失败', icon: 'none' })
        }
    },

    /**
     * 获取购物车中菜品数量
     */
    getCartQuantity(dishId: number): number {
        const item = this.data.cart?.items.find(item => item.dish_id === dishId)
        return item ? item.quantity : 0
    },

    /**
     * 呼叫服务员
     */
    callService() {
        wx.showModal({
            title: '呼叫服务',
            content: '确定要呼叫服务员吗？',
            success: (res) => {
                if (res.confirm) {
                    wx.showToast({ title: '已呼叫服务员', icon: 'success' })
                }
            }
        })
    },

    /**
     * 重试加载
     */
    onRetry() {
        const { merchantId, tableNo, reservationId, tableId } = this.data
        if (reservationId) {
            this.initPageForReservation(reservationId, merchantId)
        } else if (tableNo) {
            this.initPageByTableNo(merchantId, tableNo)
        } else if (tableId) {
            this.initPageById(tableId, merchantId)
        }
    },

    /**
     * 分享菜单
     */
    onShareAppMessage() {
        const { merchantInfo, tableId } = this.data

        return {
            title: `${merchantInfo?.name || '餐厅'}的菜单`,
            path: `/pages/dine-in/menu/menu?table_id=${tableId}&merchant_id=${this.data.merchantId}`,
            imageUrl: merchantInfo?.logo_url
        }
    },

    // ==================== 菜品加减控制 ====================

    /**
     * 增加菜品数量（无定制）
     */
    async onIncrease(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        try {
            this.applyOptimisticDishDelta(dishId, 1)
            await addToCart({
                merchant_id: this.data.merchantId,
                dish_id: dishId,
                quantity: 1,
                order_type: this.data.orderType,
                table_id: this.data.tableId || 0,
                reservation_id: this.data.reservationId || 0
            }, { loading: false })
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: error instanceof Error ? error.message : '添加失败', icon: 'none' })
        }
    },

    /**
     * 减少菜品数量（无定制）
     */
    async onDecrease(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const cartItem = this.data.cart?.items.find((i) => i.dish_id === dishId)
        if (!cartItem) return

        try {
            this.applyOptimisticDishDelta(dishId, -1)
            if (cartItem.quantity <= 1) {
                await removeFromCart(cartItem.id, { loading: false })
            } else {
                await updateCartItem(cartItem.id, { quantity: cartItem.quantity - 1 }, { loading: false })
            }
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    // ==================== 定制 Drawer ====================

    /**
     * 打开定制 Drawer
     */
    openCustomDrawer(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const dish = this.data.currentDishes.find((d) => d.id === dishId)
        if (!dish) return

        // 将 customization_groups 转换为 spec_groups 格式
        const specGroups = (dish.customization_groups || []).map((group: CustomizationGroup) => ({
            id: String(group.id),
            name: group.name,
            is_required: group.is_required,
            specs: (group.options || []).map((opt: CustomizationOption) => ({
                id: String(opt.id),
                name: opt.tag_name || (opt as unknown as { name?: string }).name || '',
                price_diff: opt.extra_price || 0,
                priceDiffDisplay: opt.extra_price ? formatPriceNoSymbol(opt.extra_price) : null
            }))
        }))

        // 初始化规格选择（每组选第一个）
        const defaultSpecs: Record<string, string> = {}
        specGroups.forEach((group) => {
            if (group.specs && group.specs.length > 0) {
                defaultSpecs[group.id] = group.specs[0].id
            }
        })

        this.setData({
            drawerVisible: true,
            drawerDish: { ...dish, spec_groups: specGroups },
            drawerSpecs: defaultSpecs,
            drawerQty: 1
        })
    },

    /**
     * 关闭定制 Drawer
     */
    closeCustomDrawer() {
        this.setData({ drawerVisible: false, drawerDish: null })
    },

    /**
     * 选择规格
     */
    onDrawerSpecTap(e: WechatMiniprogram.CustomEvent) {
        const { groupId, specId } = e.currentTarget.dataset
        this.setData({ [`drawerSpecs.${groupId}`]: specId })
    },

    /**
     * Drawer 增加数量
     */
    onDrawerIncrease() {
        this.setData({ drawerQty: this.data.drawerQty + 1 })
    },

    /**
     * Drawer 减少数量
     */
    onDrawerDecrease() {
        if (this.data.drawerQty > 1) {
            this.setData({ drawerQty: this.data.drawerQty - 1 })
        }
    },

    /**
     * 确认定制加入购物车
     */
    async onConfirmCustom() {
        const { drawerDish, drawerSpecs, drawerQty, merchantId } = this.data
        if (!drawerDish) return

        try {
            // 构建定制信息
            const customizations: Record<string, number | string> = {}
            for (const groupId in drawerSpecs) {
                if (Object.prototype.hasOwnProperty.call(drawerSpecs, groupId)) {
                    customizations[groupId] = drawerSpecs[groupId]
                }
            }

            const updatedCart = await addToCart({
                merchant_id: merchantId,
                dish_id: drawerDish.id,
                quantity: drawerQty,
                customizations,
                order_type: this.data.orderType,
                table_id: this.data.tableId || 0,
                reservation_id: this.data.reservationId || 0
            }, { loading: false })

            this.setData({ drawerVisible: false, drawerDish: null })
            this.applyCartData(updatedCart)
            wx.showToast({ title: '已添加', icon: 'success' })
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: error instanceof Error ? error.message : '添加失败', icon: 'none' })
        }
    }
})