import { ReservationItem, ReservationResponse, ReservationService } from './_main_shared/api/reservation'
import { getPublicMerchantDishes, DishDTO } from '../../../api/merchant'
import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { completePaymentWorkflow } from '../../payment/_main_shared/services/payment-workflow'
import { PaymentOrderResponse } from '../../payment/_main_shared/api/payment'
import Navigation from '../../../utils/navigation'

interface DishView {
    id: number
    category_id?: number
    category_name?: string
    name: string
    description?: string
    price: number
    image_url?: string
    priceDisplay: string
    selectedQty: number
}

interface ReservationView extends ReservationResponse {
    _timeText: string
    _guestCount: string
}

interface ComboItemView {
    combo_id: number
    name: string
    price: number
    priceDisplay: string
    quantity: number
}

interface OrphanDishView {
    dish_id: number
    name: string
    price: number
    priceDisplay: string
    quantity: number
}

function buildAdjustmentPaymentOrder(
    reservationId: number,
    responsePayment: { payment_order_id: number, amount: number, pay_params?: PaymentOrderResponse['pay_params'] }
): PaymentOrderResponse {
    return {
        id: responsePayment.payment_order_id,
        user_id: 0,
        order_id: reservationId,
        out_trade_no: '',
        amount: responsePayment.amount,
        status: 'pending',
        payment_type: 'miniprogram',
        business_type: 'reservation_addon',
        pay_params: responsePayment.pay_params,
        created_at: ''
    }
}

Page({
    data: {
        reservationId: 0,
        reservation: null as ReservationView | null,
        loading: true,
        hasError: false,
        errorMessage: '',
        navBarHeight: 88,

        categories: [] as Array<{ id: number, name: string, dishes: DishView[] }>,
        currentCategoryId: 0,
        currentDishes: [] as DishView[],

        dishQtyMap: {} as Record<number, number>,
        dishPriceMap: {} as Record<number, number>,
        comboItems: [] as ComboItemView[],
        orphanItems: [] as OrphanDishView[],

        totalCount: 0,
        totalAmountDisplay: '0.00',
        submitting: false
    },

    onLoad(options: { id?: string }) {
        if (options.id) {
            this.setData({ reservationId: parseInt(options.id) })
            this.loadData()
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
    },

    async loadData() {
        this.setData({ loading: true, hasError: false })
        try {
            const reservationId = this.data.reservationId
            const reservation = await ReservationService.getReservationDetail(reservationId)
            const dishesResponse = await getPublicMerchantDishes(Number(reservation.merchant_id))

            const dishList: DishView[] = (dishesResponse.dishes || []).map((dish: DishDTO) => {
                const id = Number(dish.id)
                return {
                    id,
                    category_id: Number(dish.category_id) || 0,
                    category_name: dish.category_name || '其他',
                    name: dish.name,
                    description: dish.description,
                    price: Number(dish.price) || 0,
                    image_url: getPublicImageUrl(dish.image_url || ''),
                    priceDisplay: formatPriceNoSymbol(Number(dish.price) || 0),
                    selectedQty: 0
                }
            })

            const dishPriceMap: Record<number, number> = {}
            dishList.forEach((dish) => {
                dishPriceMap[dish.id] = dish.price
            })

            const dishQtyMap: Record<number, number> = {}
            const comboItems: ComboItemView[] = []
            const orphanItems: OrphanDishView[] = []

            const knownDishIds = new Set(dishList.map((dish) => dish.id))

            ;(reservation.items || []).forEach((item) => {
                if (item.dish_id) {
                    const dishId = Number(item.dish_id)
                    dishQtyMap[dishId] = (dishQtyMap[dishId] || 0) + (item.quantity || 0)
                    if (!knownDishIds.has(dishId)) {
                        orphanItems.push({
                            dish_id: dishId,
                            name: item.name || '已下架菜品',
                            price: Number(item.unit_price ?? 0),
                            priceDisplay: formatPriceNoSymbol(Number(item.unit_price ?? 0)),
                            quantity: item.quantity || 0
                        })
                    }
                }
                if (item.combo_id) {
                    comboItems.push({
                        combo_id: Number(item.combo_id),
                        name: item.name || '套餐',
                        price: Number(item.unit_price ?? 0),
                        priceDisplay: formatPriceNoSymbol(Number(item.unit_price ?? 0)),
                        quantity: item.quantity || 0
                    })
                }
            })

            const dishesWithQty = dishList.map((dish) => ({
                ...dish,
                selectedQty: dishQtyMap[dish.id] || 0
            }))

            const categories: Array<{ id: number, name: string, dishes: DishView[] }> = []
            const categoryMap = new Map<number, { id: number, name: string, dishes: DishView[] }>()

            categories.push({ id: 0, name: '全部', dishes: [...dishesWithQty] })

            dishesWithQty.forEach((dish) => {
                const catId = dish.category_id || 0
                const catName = dish.category_name || '其他'
                if (!categoryMap.has(catId)) {
                    categoryMap.set(catId, { id: catId, name: catName, dishes: [] })
                }
                categoryMap.get(catId)!.dishes.push(dish)
            })

            categories.push(...Array.from(categoryMap.values()).sort((a, b) => a.id - b.id))

            const view: ReservationView = {
                ...reservation,
                _timeText: this.formatReservationDateTime(reservation.reservation_date, reservation.reservation_time),
                _guestCount: reservation.guest_count ? `${reservation.guest_count}人` : '--'
            }

            this.setData({
                reservation: view,
                categories,
                currentCategoryId: 0,
                currentDishes: categories[0]?.dishes || [],
                dishQtyMap,
                dishPriceMap,
                comboItems,
                orphanItems,
                loading: false
            })

            this.updateTotals()
        } catch (error) {
            const errMessage = getErrorUserMessage(error, '加载失败，请稍后重试')
            console.error(error)
            this.setData({
                loading: false,
                hasError: true,
                errorMessage: errMessage || '加载失败'
            })
        }
    },

    formatReservationDateTime(dateStr?: string, timeStr?: string): string {
        const datePart = (dateStr || '').trim()
        const timePart = (timeStr || '').trim()
        if (!datePart && !timePart) return '--'

        if (datePart && timePart) return `${datePart} ${timePart}`
        if (datePart) return datePart
        return timePart
    },

    switchCategory(e: WechatMiniprogram.CustomEvent) {
        const categoryId = Number(e.currentTarget.dataset.id)
        const category = this.data.categories.find((c) => c.id === categoryId)
        this.setData({
            currentCategoryId: categoryId,
            currentDishes: category?.dishes || []
        })
    },

    onIncrease(e: WechatMiniprogram.CustomEvent) {
        const id = Number(e.currentTarget.dataset.id)
        const type = e.currentTarget.dataset.type || 'dish'
        if (type === 'combo') {
            this.updateComboQty(id, 1)
            return
        }
        this.updateDishQty(id, 1)
    },

    onDecrease(e: WechatMiniprogram.CustomEvent) {
        const id = Number(e.currentTarget.dataset.id)
        const type = e.currentTarget.dataset.type || 'dish'
        if (type === 'combo') {
            this.updateComboQty(id, -1)
            return
        }
        this.updateDishQty(id, -1)
    },

    updateDishQty(dishId: number, delta: number) {
        const dishQtyMap = { ...this.data.dishQtyMap }
        const next = (dishQtyMap[dishId] || 0) + delta
        if (next < 0) return
        dishQtyMap[dishId] = next

        const categories = this.data.categories.map((cat) => ({
            ...cat,
            dishes: (cat.dishes || []).map((dish) =>
                dish.id === dishId ? { ...dish, selectedQty: next } : dish
            )
        }))

        const orphanItems = this.data.orphanItems.map((item) =>
            item.dish_id === dishId ? { ...item, quantity: next } : item
        )

        const currentCategory = categories.find((c) => c.id === this.data.currentCategoryId)

        this.setData({
            dishQtyMap,
            categories,
            currentDishes: currentCategory?.dishes || [],
            orphanItems
        })

        this.updateTotals()
    },

    updateComboQty(comboId: number, delta: number) {
        const comboItems = this.data.comboItems.map((item) => {
            if (item.combo_id !== comboId) return item
            const next = item.quantity + delta
            return { ...item, quantity: next < 0 ? 0 : next }
        })

        this.setData({ comboItems })
        this.updateTotals()
    },

    updateTotals() {
        const dishQtyMap = this.data.dishQtyMap
        const dishPriceMap = this.data.dishPriceMap
        const orphanPriceMap: Record<number, number> = {}
        this.data.orphanItems.forEach((item) => {
            orphanPriceMap[item.dish_id] = item.price
        })

        let totalCount = 0
        let totalAmount = 0

        Object.keys(dishQtyMap).forEach((key) => {
            const dishId = Number(key)
            const qty = dishQtyMap[dishId] || 0
            if (qty <= 0) return
            totalCount += qty
            const price = dishPriceMap[dishId] ?? orphanPriceMap[dishId] ?? 0
            totalAmount += price * qty
        })

        this.data.comboItems.forEach((item) => {
            if (item.quantity <= 0) return
            totalCount += item.quantity
            totalAmount += item.price * item.quantity
        })

        this.setData({
            totalCount,
            totalAmountDisplay: formatPriceNoSymbol(totalAmount)
        })
    },

    async onSubmit() {
        if (this.data.submitting) return
        const items: ReservationItem[] = []
        Object.keys(this.data.dishQtyMap).forEach((key) => {
            const dishId = Number(key)
            const qty = this.data.dishQtyMap[dishId] || 0
            if (qty > 0) {
                items.push({ dish_id: dishId, quantity: qty })
            }
        })
        this.data.comboItems.forEach((item) => {
            if (item.quantity > 0) {
                items.push({ combo_id: item.combo_id, quantity: item.quantity })
            }
        })

        if (items.length === 0) {
            wx.showToast({ title: '至少保留一道菜', icon: 'none' })
            return
        }

        try {
            this.setData({ submitting: true })
            const result = await ReservationService.modifyDishes(this.data.reservationId, items)
            if (result.outcome === 'payment_required' && result.payment) {
                const payment = buildAdjustmentPaymentOrder(this.data.reservationId, result.payment)
                const paymentResult = await completePaymentWorkflow(payment, {
                    context: this,
                    paymentMessage: '正在调起补差支付...',
                    confirmingMessage: '支付结果确认中...'
                })
                Navigation.toPaymentResult({
                    status: paymentResult.status,
                    paymentOrderId: paymentResult.paymentOrderId,
                    businessId: this.data.reservationId,
                    businessType: 'reservation',
                    orderNo: paymentResult.outTradeNo,
                    amount: formatPriceNoSymbol(result.payment.amount)
                })
                return
            }
            if (result.outcome === 'refund_initiated') {
                const refundAmountText = typeof result.refund_amount === 'number'
                    ? `，预计退款¥${formatPriceNoSymbol(result.refund_amount)}`
                    : ''
                wx.showModal({
                    title: '退款处理中',
                    content: `改菜已生效${refundAmountText}，到账结果以预订详情同步为准。`,
                    showCancel: false,
                    confirmText: '查看详情',
                    success: () => {
                        wx.navigateBack()
                    }
                })
                return
            }
            wx.navigateBack()
        } catch (error) {
            const errMessage = getErrorUserMessage(error, '修改失败，请稍后重试')
            wx.showToast({ title: errMessage || '修改失败', icon: 'none' })
        } finally {
            this.setData({ submitting: false })
        }
    },

    onRetry() {
        this.loadData()
    }
})
