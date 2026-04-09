import { getCart, calculateCart, CartResponse, CalculateCartResponse } from '../../../api/cart'
import { getPublicMerchantDetail, PublicMerchantDetail } from '../../../api/merchant'
import { createOrderFromCart } from '../../../api/order'
import { getDiningSessionMenu } from '../../../api/dining-session'
import { createOrderPayment, invokeWechatPay } from '../../../api/payment'
import { getMyMemberships, MembershipResponse } from '../../../api/personal'
import type { ScanTableTableInfo } from '../../../api/table'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'
import Navigation from '../../../utils/navigation'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { getDineInSessionContext, saveDineInSessionFromMenu } from '../../../services/dine-in-session'

type PromotionItem = NonNullable<CalculateCartResponse['applied_promotions']>[number] & {
    amountDisplay: string
}

type LadderItem = NonNullable<CalculateCartResponse['ladder_promotions']>[number] & {
    thresholdDisplay: string
    discountDisplay: string
    missingNeedDisplay: string
}

type VoucherTrialItem = NonNullable<CalculateCartResponse['voucher_trials']>[number] & {
    amountDisplay: string
    trialPayableDisplay: string
}

type PaymentAssessmentItem = NonNullable<CalculateCartResponse['payment_assessment']>

interface CalculationView {
    subtotal: number
    discount_amount: number
    total_amount: number
    subtotalDisplay: string
    totalDisplay: string
    applied_promotions: PromotionItem[]
    ladder_promotions: LadderItem[]
    voucher_trials: VoucherTrialItem[]
    payment_assessment: PaymentAssessmentItem | null
}

interface PaymentMethodView {
    id: string
    name: string
    icon: string
    disabled: boolean
}

type CartItemView = CartResponse['items'][number] & {
    image_url?: string
    dish_image?: string
    priceDisplay: string
    subtotalDisplay: string
}

type CartView = CartResponse & {
    items: CartItemView[]
}

type CheckoutMerchantInfo = PublicMerchantDetail | {
    id: number
    name: string
    logo_url?: string
    cover_image?: string
    address?: string
}

type CheckoutTableInfo = ScanTableTableInfo | {
    table_no: string
}

Page({
    data: {
        sessionId: 0,
        billingGroupId: 0,
        merchantId: 0,
        tableId: 0,
        reservationId: 0,
        orderType: 'dine_in' as 'dine_in' | 'reservation',
        
        loading: true,
        merchantInfo: null as CheckoutMerchantInfo | null,
        tableInfo: null as CheckoutTableInfo | null,
        reservationInfo: null as Record<string, unknown> | null,
        cart: null as CartResponse | null,
        calculation: {
            subtotal: 0,
            subtotalDisplay: '0.00',
            discount_amount: 0,
            discountDisplay: '0.00',
            total_amount: 0,
            totalDisplay: '0.00',
            applied_promotions: [] as PromotionItem[],
            ladder_promotions: [] as LadderItem[],
            voucher_trials: [] as VoucherTrialItem[],
            payment_assessment: null as PaymentAssessmentItem | null
        } as CalculationView,
        
        remark: '',
        selectedPaymentMethod: 'wechat_pay',
        paymentMethods: [] as PaymentMethodView[],
        memberBalance: 0,
        memberBalanceDisplay: '0.00',
        membershipId: 0,
        balanceInsufficient: false,
        
        diningInfo: {
            guest_count: 2
        },

        // 导航栏高度
        navBarHeight: 88,

        // 错误状态
        isError: false,
        errorMessage: '',
        submitting: false
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async onLoad(options: { session_id?: string, billing_group_id?: string, merchant_id?: string, table_id?: string, reservation_id?: string, order_type?: 'dine_in' | 'reservation', table_no?: string }) {
        const directSessionId = options.session_id ? parseInt(options.session_id) : 0
        const directBillingGroupId = options.billing_group_id ? parseInt(options.billing_group_id) : 0

        if (directSessionId > 0) {
            this.setData({ sessionId: directSessionId, billingGroupId: directBillingGroupId, orderType: 'dine_in' })
            await this.initData()
            return
        }

        if (!options.reservation_id && !options.merchant_id && !options.table_id) {
            const storedSession = getDineInSessionContext()
            if (storedSession?.session_id) {
                this.setData({ sessionId: storedSession.session_id, billingGroupId: storedSession.billing_group_id, orderType: 'dine_in' })
                await this.initData()
                return
            }
        }

        const merchantId = parseInt(options.merchant_id || '0')
        const tableId = options.table_id ? parseInt(options.table_id) : 0
        const reservationId = options.reservation_id ? parseInt(options.reservation_id) : 0
        const orderType = options.order_type || (reservationId ? 'reservation' : 'dine_in')
        const tableNo = options.table_no ? decodeURIComponent(options.table_no) : ''

        if (orderType === 'dine_in') {
            if (tableId > 0) {
                wx.redirectTo({ url: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableId}` })
                return
            }
            this.setData({
                loading: false,
                isError: true,
                errorMessage: '请通过扫描桌台二维码进入结账页面'
            })
            return
        }

        this.setData({ 
            merchantId, 
            tableId, 
            reservationId, 
            orderType,
            tableInfo: tableNo ? { table_no: tableNo } : null
        })

        await this.initData()
    },

    /**
     * 重试加载
     */
    onRetry() {
        this.initData()
    },

    /**
     * 初始化数据（SSOT：一切以 calculateCart 为准）
     */
    async initData() {
        this.setData({ loading: true, isError: false })
        const { sessionId } = this.data

        try {
            if (sessionId > 0) {
                const menuResponse = await getDiningSessionMenu(sessionId)
                saveDineInSessionFromMenu(menuResponse)

                const merchantId = menuResponse.session.merchant_id
                const tableId = menuResponse.session.table_id
                const reservationId = menuResponse.session.reservation_id || 0
                const orderType = 'dine_in' as const

                this.setData({
                    sessionId: menuResponse.session.id,
                    billingGroupId: this.data.billingGroupId || menuResponse.billing_group.id,
                    merchantId,
                    tableId,
                    reservationId,
                    orderType,
                    merchantInfo: menuResponse.merchant,
                    tableInfo: menuResponse.table
                })

                const [cart, calculationResult] = await Promise.all([
                    getCart({ merchant_id: merchantId, order_type: orderType, table_id: tableId || undefined, reservation_id: reservationId || undefined }),
                    calculateCart({ merchant_id: merchantId, order_type: orderType, table_id: tableId || undefined, reservation_id: reservationId || undefined })
                ])

                await this.loadMembershipInfo(merchantId)
                this.renderData(menuResponse.merchant, cart, calculationResult)
            } else {
                const { merchantId, tableId, reservationId, orderType } = this.data
                const [merchantInfo, cart] = await Promise.all([
                    getPublicMerchantDetail(merchantId),
                    getCart({ merchant_id: merchantId, order_type: orderType, table_id: tableId || undefined, reservation_id: reservationId || undefined })
                ])

                const calculationResult = await calculateCart({
                    merchant_id: merchantId,
                    order_type: orderType,
                    table_id: tableId || undefined,
                    reservation_id: reservationId || undefined
                })

                await this.loadMembershipInfo(merchantId)
                this.renderData(merchantInfo, cart, calculationResult)
            }

        } catch (error: unknown) {
            const message = getErrorUserMessage(error, '加载失败，请重试')
            console.error('初始化失败:', error)
            this.setData({ 
                isError: true, 
                errorMessage: message
            })
        } finally {
            this.setData({ loading: false })
        }
    },

    async loadMembershipInfo(merchantId?: number) {
        const targetMerchantId = merchantId ?? this.data.merchantId
        try {
            const membershipsResult = await getMyMemberships()
            const membership = membershipsResult.memberships?.find(
                (m: MembershipResponse) => m.merchant_id === targetMerchantId
            )
            if (membership) {
                const balance = membership.balance || 0
                this.setData({
                    memberBalance: balance,
                    memberBalanceDisplay: formatPriceNoSymbol(balance),
                    membershipId: membership.id
                })
            }
        } catch (err) {
            console.warn('获取余额失败', err)
        }
    },

    renderData(merchantInfo: CheckoutMerchantInfo, cart: CartResponse, calculation: CalculateCartResponse) {
        const processedCalculation: CalculationView = {
            ...calculation,
            subtotalDisplay: formatPriceNoSymbol(calculation.subtotal || 0),
            totalDisplay: formatPriceNoSymbol(calculation.total_amount || 0),
            applied_promotions: (calculation.applied_promotions || []).map((p) => ({
                ...p,
                amountDisplay: formatPriceNoSymbol(p.amount || 0)
            })),
            ladder_promotions: (calculation.ladder_promotions || []).map((rule) => ({
                ...rule,
                thresholdDisplay: formatPriceNoSymbol(rule.threshold || 0),
                discountDisplay: formatPriceNoSymbol(rule.discount || 0),
                missingNeedDisplay: formatPriceNoSymbol(rule.missing_need || 0)
            })),
            voucher_trials: (calculation.voucher_trials || []).map((trial) => ({
                ...trial,
                amountDisplay: formatPriceNoSymbol(trial.amount || 0),
                trialPayableDisplay: formatPriceNoSymbol(trial.trial_payable || 0)
            })),
            payment_assessment: calculation.payment_assessment || null
        }

        const processedCart: CartView = {
            ...cart,
            items: (cart.items || []).map((item) => {
                const rawDishImage = (item as unknown as { dish_image?: string }).dish_image
                const normalizedImage = getPublicImageUrl(item.image_url || rawDishImage || '')
                return {
                    ...item,
                    image_url: normalizedImage,
                    dish_image: normalizedImage,
                    priceDisplay: formatPriceNoSymbol(item.unit_price || 0),
                    subtotalDisplay: formatPriceNoSymbol(item.subtotal || 0)
                }
            })
        }

        const balanceInsufficient = this.data.memberBalance < calculation.total_amount

        const paymentMethods: PaymentMethodView[] = [
            { id: 'wechat_pay', name: '微信支付', icon: 'logo-wechat', disabled: false },
            { 
                id: 'balance', 
                name: `储值支付 (¥${this.data.memberBalanceDisplay})`, 
                icon: 'wallet', 
                disabled: this.data.memberBalance <= 0 
            }
        ]

        this.setData({
            merchantInfo: {
                ...merchantInfo,
                logo_url: getPublicImageUrl(merchantInfo.logo_url || merchantInfo.cover_image || '')
            },
            cart: processedCart,
            calculation: processedCalculation,
            balanceInsufficient,
            paymentMethods,
            selectedPaymentMethod: balanceInsufficient ? 'wechat_pay' : this.data.selectedPaymentMethod
        })
    },

    onPaymentMethodChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        this.setData({ selectedPaymentMethod: e.detail.value })
    },

    onRemarkChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        this.setData({ remark: e.detail.value })
    },

    onRecharged() {
        this.initData() // 重新初始化，自动刷新余额和计算结果
    },

    onVoucherClaimed() {
        this.initData() // 重新初始化，自动应用新领的券
    },

    async onSubmit() {
        if (this.data.submitting) return
        this.setData({ submitting: true })

        const { merchantId, orderType, tableId, reservationId, selectedPaymentMethod, remark, billingGroupId } = this.data

        try {
            const order = await createOrderFromCart(merchantId, orderType, {
                table_id: tableId || undefined,
                reservation_id: reservationId || undefined,
                billing_group_id: billingGroupId || undefined,
                notes: remark,
                use_balance: selectedPaymentMethod === 'balance'
            })

            await this.handlePayment(order.id)

        } catch (error: unknown) {
            const message = getErrorUserMessage(error, '下单失败，请稍后重试')
            wx.showToast({ title: message, icon: 'error' })
            this.setData({ submitting: false })
        }
    },

    async handlePayment(orderId: number) {
        try {
            const payment = await createOrderPayment(orderId)
            if (payment.pay_params) {
                await invokeWechatPay(payment.pay_params)
            }
            Navigation.toDineInPaymentSuccess({
                orderId: String(orderId),
                amount: formatPriceNoSymbol(payment.amount || this.data.calculation.total_amount || 0),
                merchantName: this.data.merchantInfo?.name,
                tableNumber: String((this.data.tableInfo?.table_no as string | undefined) || '')
            })
        } catch (error) {
            console.error('支付失败', error)
            wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })
        }
    }
})