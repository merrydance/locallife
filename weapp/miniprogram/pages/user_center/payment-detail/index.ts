import { BusinessType, closePayment, createPayment as createPaymentOrder, getPaymentById, getPaymentRefunds, getPayments, getPaymentStatusView, getRefundStatusView, invokeWechatPay, isPaymentStatusSuccessful, PaymentOrder, RefundOrder } from '../../../api/payment'
import {
    getRiderDepositRechargeWorkflowStatusView,
    submitRiderDepositRecharge
} from '../../../services/rider-deposit-payment'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'

type RefundView = RefundOrder & {
    _amountDisplay: string
    _statusText: string
    _statusClass: string
    _statusTheme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
}

const getDatasetId = (event: WechatMiniprogram.BaseEvent): number | null => {
    const dataset = event.currentTarget.dataset as { id?: string | number }
    const id = dataset.id
    const numericId = typeof id === 'number' ? id : Number(id)
    return Number.isFinite(numericId) ? numericId : null
}

type CurrentPageWithOptions = {
        options?: {
                id?: string
                orderId?: string
        }
}

const BUSINESS_TYPES: BusinessType[] = ['order', 'reservation', 'reservation_addon', 'membership_recharge', 'rider_deposit', 'claim_recovery']

function normalizeBusinessType(value?: string): BusinessType {
    return BUSINESS_TYPES.includes(value as BusinessType) ? value as BusinessType : 'order'
}

Page({
    data: {
        paymentId: 0,
        payment: null as PaymentOrder | null,
        refunds: [] as RefundView[],
        navBarHeight: 88,
        loading: false,
        initialLoading: true,
        error: null as string | null,
        // 显示字段
        amountDisplay: '',
        statusText: '',
        statusClass: '',
        statusIcon: 'info-circle-filled',
        paymentMethodText: '',
        referenceLabel: '订单编号',
        referenceValue: '-',
        showCloseButton: false,
        showPayButton: false,
        paying: false,
        showRefundList: false,
        showPendingTip: false
    },

    onLoad(options: { id?: string, orderId?: string }) {
        if (options.id) {
            this.setData({ paymentId: parseInt(options.id) })
            this.loadPaymentDetail()
        } else if (options.orderId) {
            // 通过订单ID查找支付记录
            this.loadPaymentByOrder(parseInt(options.orderId))
        }
    },

    async loadPaymentDetail() {
        if (!this.data.paymentId) return
        this.setData({ loading: true, error: null })
        try {
            const payment = await getPaymentById(this.data.paymentId)
            this.processPayment(payment)

            await this.loadRefunds()
            this.setData({ initialLoading: false, loading: false })
        } catch (error) {
            logger.error('加载支付详情失败', error, 'payment-detail.loadPaymentDetail')
            this.setData({ 
                initialLoading: false, 
                loading: false,
                error: '加载支付详情失败'
            })
        }
    },

    async loadPaymentByOrder(orderId: number) {
        this.setData({ loading: true, error: null })
        try {
            // 通过订单ID获取支付列表，取第一条
            const result = await getPayments({ order_id: orderId, page_id: 1, page_size: 1 })
            if (result.payment_orders && result.payment_orders.length > 0) {
                const payment = result.payment_orders[0]
                this.setData({ paymentId: payment.id })
                this.processPayment(payment)
                await this.loadRefunds()
                this.setData({ initialLoading: false, loading: false })
            } else {
                this.setData({ 
                    initialLoading: false, 
                    loading: false,
                    error: '未找到支付记录'
                })
            }
        } catch (error) {
            logger.error('加载支付详情失败', error, 'payment-detail.loadPaymentByOrder')
            this.setData({ 
                initialLoading: false, 
                loading: false,
                error: '加载支付详情失败'
            })
        }
    },

    onRetry() {
        const pages = getCurrentPages()
        const currentPage = pages[pages.length - 1] as CurrentPageWithOptions | undefined
        const options = currentPage?.options || {}
        if (options.id) {
            this.loadPaymentDetail()
        } else if (options.orderId) {
            this.loadPaymentByOrder(parseInt(options.orderId))
        }
    },

    processPayment(payment: PaymentOrder) {
        const statusView = getPaymentStatusView(payment.status)
        const amountDisplay = `¥${(payment.amount / 100).toFixed(2)}`
        const statusText = statusView.text
        const statusClass = statusView.className
        const paymentMethodText = this.getPaymentMethodText(payment.payment_type)
        const showCloseButton = statusView.isPending
        const showPayButton = statusView.isPending
            && payment.payment_type === 'miniprogram'
            && (!!payment.order_id || payment.business_type === 'rider_deposit')
        const showRefundList = false
        const referenceLabel = payment.order_id ? '订单编号' : '支付单号'
        const referenceValue = payment.order_id
            ? String(payment.order_id)
            : (payment.out_trade_no || String(payment.id))

        this.setData({
            payment,
            amountDisplay,
            statusText,
            statusClass,
            statusIcon: statusView.icon,
            paymentMethodText,
            referenceLabel,
            referenceValue,
            showCloseButton,
            showPayButton,
            showRefundList,
            showPendingTip: statusView.showPendingTip
        })
    },

    async onContinuePay() {
        if (this.data.paying) return

        const payment = this.data.payment
        if (!payment) {
            wx.showToast({ title: '订单信息缺失', icon: 'none' })
            return
        }

        this.setData({ paying: true })
        wx.showLoading({ title: '拉起支付...' })
        try {
            if (payment.business_type === 'rider_deposit') {
                const rechargeResult = await submitRiderDepositRecharge(payment.amount)
                const rechargeStatusView = getRiderDepositRechargeWorkflowStatusView(rechargeResult.status)

                if (rechargeStatusView.isCancelled) {
                    wx.showToast({ title: '已取消支付', icon: 'none' })
                    await this.loadPaymentDetail()
                    return
                }

                if (rechargeStatusView.isPaid) {
                    wx.showToast({ title: '充值已完成', icon: 'success' })
                    await this.loadPaymentDetail()
                    return
                }

                if (rechargeStatusView.isPendingConfirmation) {
                    wx.showToast({ title: '支付已提交，请稍后确认', icon: 'none' })
                    await this.loadPaymentDetail()
                    return
                }

                wx.showToast({ title: '支付未完成', icon: 'none' })
                await this.loadPaymentDetail()
                return
            }

            if (!payment.order_id) {
                wx.showToast({ title: '订单信息缺失', icon: 'none' })
                return
            }

            const latestPayment = await createPaymentOrder({
                order_id: payment.order_id,
                payment_type: 'miniprogram',
                business_type: normalizeBusinessType(payment.business_type)
            })

            if (latestPayment.pay_params) {
                try {
                    await invokeWechatPay(latestPayment.pay_params)
                } catch (error: unknown) {
                    const wxError = error as { errMsg?: string }
                    if (wxError?.errMsg?.includes('cancel')) {
                        wx.showToast({ title: '已取消支付', icon: 'none' })
                        return
                    }
                    throw error
                }
            } else if (!isPaymentStatusSuccessful(latestPayment.status)) {
                throw new Error('支付参数缺失')
            }

            if (latestPayment.order_id) {
                Navigation.toPaymentSuccess({
                    orderId: String(latestPayment.order_id),
                    orderNo: latestPayment.out_trade_no || String(latestPayment.order_id),
                    amount: (latestPayment.amount / 100).toFixed(2)
                })
                return
            }

            if (latestPayment.id) {
                this.setData({ paymentId: latestPayment.id })
            }
            await new Promise((resolve) => setTimeout(resolve, 1200))
            await this.loadPaymentDetail()
        } catch (error) {
            logger.error('继续支付失败', error, 'payment-detail.onContinuePay')
            wx.showToast({ title: '支付失败', icon: 'none' })
        } finally {
            wx.hideLoading()
            this.setData({ paying: false })
        }
    },

    async loadRefunds() {
        try {
            const refundsResponse = await getPaymentRefunds(this.data.paymentId)
            // 处理退款显示字段
            const processedRefunds: RefundView[] = refundsResponse.refund_orders.map((refund) => {
                const statusView = getRefundStatusView(refund.status)
                return {
                    ...refund,
                    _amountDisplay: `¥${(refund.refund_amount / 100).toFixed(2)}`,
                    _statusText: statusView.text,
                    _statusClass: statusView.className,
                    _statusTheme: statusView.theme
                }
            })
            this.setData({ refunds: processedRefunds, showRefundList: processedRefunds.length > 0 })
        } catch (error) {
            logger.error('加载退款列表失败', error, 'payment-detail.loadRefunds')
        }
    },

    getPaymentMethodText(method: string): string {
        const methodMap: Record<string, string> = {
            'miniprogram': '小程序支付',
            'native': '扫码支付'
        }
        return methodMap[method] || method
    },

    formatTime(timeStr: string): string {
        if (!timeStr) return ''
        try {
            const date = new Date(timeStr)
            const y = date.getFullYear()
            const m = ('0' + (date.getMonth() + 1)).slice(-2)
            const d = ('0' + date.getDate()).slice(-2)
            const h = ('0' + date.getHours()).slice(-2)
            const min = ('0' + date.getMinutes()).slice(-2)
            return `${y}-${m}-${d} ${h}:${min}`
        } catch {
            return timeStr
        }
    },

    async onClosePayment() {
        wx.showModal({
            title: '关闭支付',
            content: '确定要关闭此支付订单吗？关闭后将无法继续支付。',
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '处理中...' })
                    try {
                        await closePayment(this.data.paymentId)
                        await this.loadPaymentDetail()
                    } catch (error) {
                        wx.hideLoading()
                        logger.error('关闭支付失败', error, 'payment-detail.onClosePayment')
                        wx.showToast({ title: '操作失败', icon: 'error' })
                        return
                    }
                    wx.hideLoading()
                }
            }
        })
    },

    onViewRefund(e: WechatMiniprogram.BaseEvent) {
        const id = getDatasetId(e)
        if (!id) return
        wx.navigateTo({
            url: `/pages/user_center/refund-detail/index?id=${id}`
        })
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    }
})
