/**
 * 堂食点餐/预订点菜菜单页面
 * 堂食仅允许基于已建立的用餐会话进入；预订点菜允许 reservation 直达。
 */

import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { clearDineInSessionContext, getDineInSessionContext, saveDineInSessionFromMenu } from '../../../services/dine-in-session'
import {
    addMenuItemToCart,
    calculateMenuCart,
    getMenuCart,
    loadDineInSessionMenu,
    loadReservationMenuSource,
    removeMenuCartItem,
    updateMenuCartItem,
    type MenuComboInfo,
    type MenuPromotionInfo
} from '../../../services/dine-in-menu'
import {
    buildDishQtyUpdate,
    buildDrawerState,
    buildMenuCartDataUpdate,
    buildOptimisticCartItemUpdate,
    buildOptimisticDishDeltaUpdate,
    buildReservationMenuState,
    buildSessionMenuState,
    findPlainDishCartItem,
    validateDrawerSelection,
    type CartView,
    type DrawerDish,
    type MenuCategory,
    type MenuDish,
    type MerchantInfoView,
    type TableInfoView
} from '../../../utils/dine-in-menu-view'

Page({
    data: {
        sessionId: 0,
        billingGroupId: 0,
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
        combos: [] as MenuComboInfo[],
        promotions: [] as MenuPromotionInfo[],
        currentCategoryId: 0,
        currentDishes: [] as MenuDish[],

        // 购物车数据
        cart: null as CartView | null,
        cartTotal: 0,
        cartCount: 0,

        // 界面状态
        loading: true,
        cartVisible: false,
        selectedDish: null as MenuDish | null,

        // 定制 Drawer 状态
        drawerVisible: false,
        drawerDish: null as DrawerDish | null,
        drawerSpecs: {} as Record<string, string>,
        drawerQty: 1,

        // 错误状态
        hasError: false,
        errorMessage: ''
    },

    onLoad(options: { session_id?: string, reservation_id?: string, merchant_id?: string, table_id?: string, scene?: string }) {
        wx.showShareMenu({
            withShareTicket: true,
            menus: ['shareAppMessage', 'shareTimeline']
        })

        const { navBarHeight } = getStableBarHeights()
        this.setData({ navBarHeight })

        if (options.session_id) {
            const sessionId = parseInt(options.session_id)
            this.setData({ sessionId, orderType: 'dine_in' })
            this.initPageBySession(sessionId)
            return
        }

        const storedSession = getDineInSessionContext()
        if (storedSession?.session_id) {
            this.setData({ sessionId: storedSession.session_id, orderType: 'dine_in' })
            this.initPageBySession(storedSession.session_id)
            return
        }

        if (options.reservation_id && options.merchant_id) {
            const reservationId = parseInt(options.reservation_id)
            const merchantId = parseInt(options.merchant_id)
            this.setData({
                reservationId,
                merchantId,
                orderType: 'reservation'
            })
            this.initPageForReservation(reservationId, merchantId)
            return
        }

        if (options.table_id && options.merchant_id) {
            wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${options.table_id}` })
            return
        }

        if (options.scene) {
            wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?scene=${encodeURIComponent(options.scene)}` })
            return
        }

        this.showError('请通过扫描桌台二维码进入点餐页面')
    },

    showError(message: string) {
        this.setData({
            loading: false,
            hasError: true,
            errorMessage: message
        })
    },

    goBack() {
        const pages = getCurrentPages()
        if (pages.length > 1) {
            wx.navigateBack()
        } else {
            const tableId = this.data.tableId || getDineInSessionContext()?.table_id
            if (tableId) {
                wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableId}` })
                return
            }
            wx.switchTab({ url: '/pages/user_center/index' })
        }
    },

    async initPageBySession(sessionId: number) {
        this.setData({ loading: true, hasError: false })

        try {
            const menuResponse = await loadDineInSessionMenu(sessionId)
            saveDineInSessionFromMenu(menuResponse)
            const nextView = buildSessionMenuState(menuResponse)
            this.setData(nextView.state)
            wx.setNavigationBarTitle({ title: nextView.title })
            await this.loadCart()
        } catch (error: unknown) {
            clearDineInSessionContext()
            this.setData({
                loading: false,
                hasError: true,
                errorMessage: getErrorUserMessage(error, '堂食会话已失效，请重新扫码入座')
            })
        }
    },

    async initPageForReservation(reservationId: number, merchantId: number) {
        try {
            this.setData({ loading: true, hasError: false })
            const { reservation, merchantDetail, dishesResponse } = await loadReservationMenuSource(reservationId, merchantId)
            const nextView = buildReservationMenuState(reservationId, merchantId, {
                reservation,
                merchantDetail,
                dishesResponse
            })
            this.setData(nextView.state)
            wx.setNavigationBarTitle({ title: nextView.title })
            await this.loadCart()
        } catch (error: unknown) {
            console.error('预订初始化失败:', error)
            const message =
                error && typeof error === 'object' && 'message' in error
                    ? String((error as { message?: string }).message || '')
                    : ''
            this.setData({
                hasError: true,
                errorMessage: message || '加载失败'
            })
        } finally {
            this.setData({ loading: false })
        }
    },

    async loadCart() {
        try {
            const cart = await getMenuCart({
                merchant_id: this.data.merchantId,
                order_type: this.data.orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })
            this.setData(buildMenuCartDataUpdate({
                cart,
                currentDishes: this.data.currentDishes,
                categories: this.data.categories
            }))
        } catch (error) {
            console.warn('加载购物车失败:', error)
            this.setData({
                cart: null,
                cartTotal: 0,
                cartCount: 0
            })
        }
    },

    scheduleCartSync() {
        const page = this as WechatMiniprogram.Page.Instance<WechatMiniprogram.IAnyObject, WechatMiniprogram.IAnyObject> & { _cartSyncTimer?: number }
        if (page._cartSyncTimer) {
            clearTimeout(page._cartSyncTimer)
        }
        page._cartSyncTimer = setTimeout(async () => {
            try {
                const cart = await getMenuCart({
                    merchant_id: this.data.merchantId,
                    order_type: this.data.orderType,
                    table_id: this.data.tableId || undefined,
                    reservation_id: this.data.reservationId || undefined
                })
                this.setData(buildMenuCartDataUpdate({
                    cart,
                    currentDishes: this.data.currentDishes,
                    categories: this.data.categories
                }))
            } catch (error) {
                void error
            }
        }, 300) as unknown as number
    },

    updateDishQtyInLists(dishId: number, nextQty: number) {
        const dataUpdate = buildDishQtyUpdate({
            currentDishes: this.data.currentDishes,
            categories: this.data.categories,
            dishId,
            nextQty
        })
        if (Object.keys(dataUpdate).length > 0) {
            this.setData(dataUpdate)
        }
    },

    findPlainDishCartItem(dishId: number) {
        return findPlainDishCartItem(this.data.cart, dishId)
    },

    applyOptimisticCartItemChange(itemId: number, nextQty: number) {
        const dataUpdate = buildOptimisticCartItemUpdate({
            cart: this.data.cart,
            itemId,
            nextQty,
            currentDishes: this.data.currentDishes,
            categories: this.data.categories
        })
        if (dataUpdate) {
            this.setData(dataUpdate)
        }
    },

    applyOptimisticDishDelta(dishId: number, deltaQty: number) {
        const dataUpdate = buildOptimisticDishDeltaUpdate({
            cart: this.data.cart,
            currentDishes: this.data.currentDishes,
            categories: this.data.categories,
            dishId,
            deltaQty
        })
        if (dataUpdate) {
            this.setData(dataUpdate)
        }
    },

    switchCategory(e: WechatMiniprogram.CustomEvent) {
        const categoryId = e.currentTarget.dataset.id
        const category = this.data.categories.find((c) => c.id === categoryId)

        this.setData({
            currentCategoryId: categoryId,
            currentDishes: category?.dishes || []
        })
    },

    viewDishDetail(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const dish = this.data.currentDishes.find((d) => d.id === dishId)

        if (dish) {
            this.setData({ selectedDish: dish })
        }
    },

    closeDishDetail() {
        this.setData({ selectedDish: null })
    },

    openCustomDrawer(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const dish = this.data.currentDishes.find((d) => d.id === dishId)
        if (!dish) return

        if (this.data.selectedDish) {
            this.closeDishDetail()
        }

        this.setData(buildDrawerState(dish))
    },

    closeCustomDrawer() {
        this.setData({ drawerVisible: false })
    },

    onDrawerSpecTap(e: WechatMiniprogram.CustomEvent) {
        const { groupId, specId } = e.currentTarget.dataset
        const drawerSpecs = { ...this.data.drawerSpecs }
        drawerSpecs[groupId] = specId
        this.setData({ drawerSpecs })
    },

    onDrawerDecrease() {
        if (this.data.drawerQty > 1) {
            this.setData({ drawerQty: this.data.drawerQty - 1 })
        }
    },

    onDrawerIncrease() {
        this.setData({ drawerQty: this.data.drawerQty + 1 })
    },

    async onConfirmCustom() {
        const { drawerDish, drawerSpecs, drawerQty } = this.data
        if (!drawerDish) {
            return
        }
        const validationMessage = validateDrawerSelection(drawerDish, drawerSpecs)
        if (validationMessage) {
            wx.showToast({ title: validationMessage, icon: 'none' })
            return
        }

        try {
            wx.showLoading({ title: '加入购物车...' })
            
            await addMenuItemToCart({
                merchant_id: this.data.merchantId,
                dish_id: drawerDish.id,
                quantity: drawerQty,
                customizations: drawerSpecs,
                order_type: this.data.orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })

            this.closeCustomDrawer()
            wx.showToast({ title: '已加入', icon: 'success' })
            this.scheduleCartSync()
            
        } catch (error) {
            console.error('加入购物车失败:', error)
            wx.showToast({ title: '加入失败', icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    },

    async updateItemQuantity(e: WechatMiniprogram.CustomEvent) {
        const { itemId, quantity } = e.currentTarget.dataset
        try {
            this.applyOptimisticCartItemChange(itemId, quantity)
            if (quantity <= 0) {
                await removeMenuCartItem(itemId, { loading: false })
            } else {
                await updateMenuCartItem(itemId, { quantity }, { loading: false })
            }
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    toggleCartVisible() {
        this.setData({ cartVisible: !this.data.cartVisible })
    },

    showCart() {
        this.setData({ cartVisible: true })
    },

    hideCart() {
        this.setData({ cartVisible: false })
    },

    async goToCheckout() {
        const { cart, tableId, merchantId, orderType, reservationId, sessionId, billingGroupId } = this.data

        if (!cart || cart.items.length === 0) {
            wx.showToast({ title: '购物车为空', icon: 'none' })
            return
        }

        try {
            // 计算订单金额
            await calculateMenuCart({
                merchant_id: merchantId,
                order_type: orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })

            let url = `/pages/dine-in/checkout/checkout?merchant_id=${merchantId}&order_type=${orderType}`
            if (sessionId > 0) {
                url += `&session_id=${sessionId}&billing_group_id=${billingGroupId}`
            } else if (orderType === 'reservation') {
                url += `&reservation_id=${reservationId}`
            } else {
                const fallbackTableId = tableId || getDineInSessionContext()?.table_id
                if (fallbackTableId) {
                    wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${fallbackTableId}` })
                    return
                }
                wx.showToast({ title: '堂食会话已失效，请重新扫码', icon: 'none' })
                return
            }

            wx.navigateTo({ url })
        } catch (error) {
            console.error('结算失败:', error)
            wx.showToast({ title: getErrorUserMessage(error, '结算失败，请稍后重试'), icon: 'none' })
        }
    },

    getCartQuantity(dishId: number): number {
        const item = this.data.cart?.items.find((item) => item.dish_id === dishId)
        return item ? item.quantity : 0
    },

    onRetry() {
        const { sessionId, reservationId, merchantId, tableId } = this.data
        if (sessionId) {
            this.initPageBySession(sessionId)
        } else if (reservationId) {
            this.initPageForReservation(reservationId, merchantId)
        } else if (tableId) {
            wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableId}` })
        }
    },

    onShareAppMessage() {
        const { merchantInfo, tableId } = this.data

        return {
            title: `${merchantInfo?.name || '餐厅'}的菜单`,
            path: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableId}`,
            imageUrl: merchantInfo?.logo_url
        }
    },

    onShareTimeline() {
        const { merchantInfo, tableId } = this.data

        return {
            title: `${merchantInfo?.name || '餐厅'}的菜单`,
            query: `table_id=${tableId}`,
            imageUrl: merchantInfo?.logo_url
        }
    },

    async onIncrease(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        try {
            this.applyOptimisticDishDelta(dishId, 1)
            await addMenuItemToCart({
                merchant_id: this.data.merchantId,
                dish_id: dishId,
                quantity: 1,
                order_type: this.data.orderType,
                table_id: this.data.tableId || undefined,
                reservation_id: this.data.reservationId || undefined
            })
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: getErrorUserMessage(error, '添加失败，请稍后重试'), icon: 'none' })
        }
    },

    async onDecrease(e: WechatMiniprogram.CustomEvent) {
        const dishId = e.currentTarget.dataset.id
        const cartItem = this.data.cart?.items.find((i) => i.dish_id === dishId)
        if (!cartItem) return

        try {
            this.applyOptimisticDishDelta(dishId, -1)
            if (cartItem.quantity <= 1) {
                await removeMenuCartItem(cartItem.id, { loading: false })
            } else {
                await updateMenuCartItem(cartItem.id, { quantity: cartItem.quantity - 1 }, { loading: false })
            }
            this.scheduleCartSync()
        } catch (error) {
            this.loadCart()
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    }

})