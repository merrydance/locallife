import { getCurrentDiningSession, openDiningSession, createDiningOrder, DiningSessionDTO } from '../../api/reservation'
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
        dishes: [] as any[],
        categories: [] as any[],
        activeCategoryId: 'all',
        cartCount: 0,
        cartPrice: 0,
        cartPriceDisplay: '0.00',
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
        try {
            const session = await getCurrentDiningSession(this.data.tableId)
            this.setData({ session })
        } catch (error) {
            // Session likely doesn't exist, try to open
            // Ask for person count
            return new Promise((resolve, reject) => {
                wx.showModal({
                    title: '开台',
                    content: '请输入用餐人数',
                    editable: true,
                    placeholderText: '1',
                    success: async (res) => {
                        if (res.confirm) {
                            const person = parseInt(res.content || '1')
                            try {
                                const session = await openDiningSession(this.data.tableId, person)
                                this.setData({ session })
                                resolve(session)
                            } catch (err) {
                                reject(err)
                            }
                        } else {
                            wx.navigateBack()
                            reject(new Error('User cancelled'))
                        }
                    }
                })
            })
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
        const { session, cartCount } = this.data

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
                            extra_options: []
                        }))

                        await createDiningOrder(session.id, { items })

                        wx.showToast({ title: '下单成功', icon: 'success' })
                        CartService.clear()
                        this.setData({ cartCount: 0, cartPrice: 0 })
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
