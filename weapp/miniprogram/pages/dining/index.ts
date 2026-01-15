import { precheckDiningSession, openDiningSession, DiningSessionDTO, BillingGroupDTO } from '../../api/reservation'
import { createDiningOrder } from '../../api/dining-session'
import { createBillingGroup, listBillingGroupOrders } from '../../api/billing-group'
import { getOrderDetail } from '../../api/order'
import { getMerchantDishes, DishDTO } from '../../api/merchant'
import CartService from '../../services/cart'
import { logger } from '../../utils/logger'
import { ErrorHandler } from '../../utils/error-handler'
import { BehaviorTracker, EventType } from '../../utils/tracker'
import { formatPriceNoSymbol } from '../../utils/util'

Page({
    data: {
        tableId: '',
        merchantId: '',
        session: null as DiningSessionDTO | null,
        billingGroup: null as BillingGroupDTO | null,
        billingGroupId: undefined as number | undefined,
        reservationId: undefined as number | undefined,
        dishes: [] as any[],
        categories: [] as any[],
        activeCategoryId: 'all',
        cartCount: 0,
        cartPrice: 0,
        cartPriceDisplay: '0.00',
        sharedDishCounts: {} as Record<number, number>,
        navBarHeight: 88,
        loading: true
    },

    onLoad(options: any) {
        // 微信扫码进入,参数格式: ?table_id=xxx&merchant_id=xxx
        if (options.table_id && options.merchant_id) {
            this.setData({
                tableId: options.table_id,
                merchantId: options.merchant_id
            })
            this.init()
        } else {
            // For dev testing without scan
            if (options.dev) {
                this.setData({ tableId: '1', merchantId: '1' })
                this.init()
            } else {
                wx.showToast({ title: '无效的二维码', icon: 'error' })
                setTimeout(() => wx.navigateBack(), 1500)
            }
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async init() {
        this.setData({ loading: true })
        try {
            await this.checkAndOpenSession()
            await this.loadMenu()
            this.setData({ loading: false })
        } catch (error) {
            ErrorHandler.handle(error, 'Dining.init')
            this.setData({ loading: false })
        }
    },

    async checkAndOpenSession() {
        // 先做预检，判断是否存在属于当前用户的预订
        const precheck = await precheckDiningSession(Number(this.data.tableId))
        const reservationId = precheck.reserved && precheck.is_reservation_owner ? precheck.reservation_id : undefined
        this.setData({ reservationId })

        const result = await openDiningSession({
            table_id: Number(this.data.tableId),
            reservation_id: reservationId
        })

        this.setData({ session: result.session, billingGroup: result.billing_group, billingGroupId: result.billing_group?.id })

        await this.chooseBillingGroup(result.session.id, result.billing_group)
        await this.loadSharedOrderSummary()
        return result.session
    },

    async chooseBillingGroup(sessionId: number, defaultGroup: BillingGroupDTO) {
        return new Promise<void>((resolve) => {
            wx.showModal({
                title: '结算方式',
                content: '是否单独结算？',
                confirmText: '单独结算',
                cancelText: '一起点餐',
                success: async (res) => {
                    if (res.confirm) {
                        try {
                            const group = await createBillingGroup(sessionId)
                            this.setData({ billingGroup: group, billingGroupId: group.id })
                        } catch (error) {
                            logger.error('创建账单组失败', error, 'Dining.chooseBillingGroup')
                            wx.showToast({ title: '创建账单组失败', icon: 'error' })
                            this.setData({ billingGroup: defaultGroup, billingGroupId: defaultGroup.id })
                        }
                    } else {
                        this.setData({ billingGroup: defaultGroup, billingGroupId: defaultGroup.id })
                    }
                    resolve()
                }
            })
        })
    },

    async loadSharedOrderSummary() {
        const billingGroupId = this.data.billingGroupId
        if (!billingGroupId) {
            return
        }

        try {
            const { orders } = await listBillingGroupOrders(billingGroupId)
            const summary: Record<number, number> = {}
            for (const order of orders) {
                try {
                    const detail = await getOrderDetail(order.order_id)
                    for (const item of detail.items || []) {
                        if (item.dish_id) {
                            summary[item.dish_id] = (summary[item.dish_id] || 0) + item.quantity
                        }
                    }
                } catch (error) {
                    logger.warn('获取订单详情失败', error, 'Dining.loadSharedOrderSummary')
                }
            }
            this.setData({ sharedDishCounts: summary })
        } catch (error) {
            logger.warn('获取账单组订单失败', error, 'Dining.loadSharedOrderSummary')
        }
    },

    async loadMenu() {
        try {
            const response = await getMerchantDishes(this.data.merchantId)
            // 预处理菜品价格
            const dishes = (response.dishes || []).map((dish: any) => ({
                ...dish,
                priceDisplay: formatPriceNoSymbol(dish.price || 0),
                memberPriceDisplay: dish.member_price ? formatPriceNoSymbol(dish.member_price) : null
            }))

            const categories = [{ id: 'all', name: '全部' }]
            const categoryMap = new Map()

            dishes.forEach((dish: any) => {
                if (dish.category_id && !categoryMap.has(dish.category_id)) {
                    categoryMap.set(dish.category_id, {
                        id: dish.category_id,
                        name: dish.category_name || String(dish.category_id)
                    })
                }
            })

            categories.push(...Array.from(categoryMap.values()))

            this.setData({ dishes, categories })
        } catch (error) {
            logger.error('加载菜单失败', error, 'Dining.loadMenu')
            wx.showToast({ title: '加载菜单失败', icon: 'error' })
        }
    },

    onCategoryChange(e: WechatMiniprogram.CustomEvent) {
        const detail = e.detail as unknown as { id?: string } | string
        const categoryId = typeof detail === 'string' ? detail : (detail.id || '')
        this.setData({ activeCategoryId: categoryId })
    },

    async onAddCart(e: WechatMiniprogram.CustomEvent) {
        const { id } = e.detail
        const dish = this.data.dishes.find((d: DishDTO) => d.id === id)

        if (dish) {
            const sharedCount = this.data.sharedDishCounts[dish.id] || 0
            if (sharedCount > 0) {
                const proceed = await new Promise<boolean>((resolve) => {
                    wx.showModal({
                        title: '同伴已点',
                        content: `同伴已点 ${sharedCount} 份该菜，是否继续添加？`,
                        confirmText: '继续添加',
                        cancelText: '取消',
                        success: (res) => resolve(res.confirm)
                    })
                })
                if (!proceed) {
                    return
                }
            }
            const success = await CartService.addItem({
                merchantId: this.data.merchantId,
                dishId: dish.id,
                dishName: dish.name,
                shopName: '当前餐厅',
                imageUrl: dish.image_url,
                price: dish.price,
                priceDisplay: `¥${(dish.price / 100).toFixed(2)}`
            })

            if (!success) {
                return
            }

            this.updateCartDisplay()
            wx.showToast({ title: '已加入', icon: 'success', duration: 500 })
        }
    },

    updateCartDisplay() {
        const cart = CartService.getCart()
        this.setData({
            cartCount: cart.totalCount,
            cartPrice: cart.totalPrice,
            cartPriceDisplay: formatPriceNoSymbol(cart.totalPrice || 0)
        })
    },

    async onSubmitOrder() {
        const { session, cartCount, billingGroupId } = this.data

        if (cartCount === 0) {
            wx.showToast({ title: '请先选择菜品', icon: 'none' })
            return
        }

        if (!session) {
            wx.showToast({ title: '会话无效', icon: 'none' })
            return
        }

        const cart = CartService.getCart()

        wx.showModal({
            title: '确认下单',
            content: `共${cart.totalCount}件菜品，${cart.totalPriceDisplay}`,
            success: async (res) => {
                if (res.confirm) {
                    try {
                        const items = cart.items.map((i) => ({
                            dish_id: i.dishId,
                            quantity: i.quantity,
                            customizations: []
                        }))

                        await createDiningOrder({
                            merchant_id: Number(this.data.merchantId),
                            table_id: Number(this.data.tableId),
                            reservation_id: this.data.reservationId,
                            items,
                            order_type: 'dine_in',
                            billing_group_id: billingGroupId
                        })

                        wx.showToast({ title: '下单成功', icon: 'success' })
                        CartService.clear()
                        this.setData({ cartCount: 0, cartPrice: 0 })
                        await this.loadSharedOrderSummary()
                    } catch (error) {
                        logger.error('下单失败', error, 'Dining.onSubmitOrder')
                        wx.showToast({ title: '下单失败', icon: 'error' })
                    }
                }
            }
        })
    },

    onCallWaiter() {
        wx.showToast({ title: '已呼叫服务员', icon: 'success' })
    }
})
