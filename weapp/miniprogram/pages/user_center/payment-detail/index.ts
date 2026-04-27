import { BusinessType, closePayment, getPaymentById, getPaymentRefunds, getPayments, getPaymentStatusView, getRefundStatusView, PaymentOrder, RefundOrder } from '../../../api/payment'
import {
    continuePendingRiderDepositRecharge,
    getRiderDepositRechargeWorkflowStatusView,
    type RiderDepositPendingRechargeContext
} from '../../../services/rider-deposit-payment'
import { startPaymentOrderWorkflow } from '../../../services/payment-workflow'
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
            this.applyPaymentDetail(payment)

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
                this.applyPaymentDetail(payment)
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

    applyPaymentDetail(payment: PaymentOrder) {
        const statusView = getPaymentStatusView(payment.status)
        const amountDisplay = (payment.amount / 100).toFixed(2)
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
                const rechargeContext: RiderDepositPendingRechargeContext = {
                    paymentOrderId: payment.id,
                    amount: payment.amount,
                    outTradeNo: payment.out_trade_no,
                    updatedAt: new Date().toISOString()
                }
                const rechargeResult = await continuePendingRiderDepositRecharge(rechargeContext)
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

            const paymentResult = await startPaymentOrderWorkflow({
                orderId: payment.order_id,
                paymentType: 'miniprogram',
                businessType: normalizeBusinessType(payment.business_type),
                maxAttempts: 5,
                interval: 1500
            })

            if (paymentResult.paymentOrderId) {
                this.setData({ paymentId: paymentResult.paymentOrderId })
            }

            Navigation.toPaymentResult({
                status: paymentResult.status,
                paymentOrderId: paymentResult.paymentOrderId,
                businessId: paymentResult.businessId || payment.order_id,
                businessType: paymentResult.businessType || payment.business_type,
                orderNo: paymentResult.outTradeNo,
                amount: paymentResult.amountFen ? (paymentResult.amountFen / 100).toFixed(2) : this.data.amountDisplay
            })
        } catch (error) {
            logger.error('继续支付失败', error, 'payment-detail.onContinuePay')
            wx.showToast({ title: '支付未完成，请稍后重试', icon: 'none' })
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
                    _amountDisplay: (refund.refund_amount / 100).toFixed(2),
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
            title: '关闭支付单',
            content: '关闭后该支付单无法继续支付，如仍需付款需要重新发起。',
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
