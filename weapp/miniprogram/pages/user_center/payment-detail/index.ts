import { getPaymentById, closePayment, getPaymentRefunds, getPayments, PaymentOrder, RefundOrder } from '../../../api/payment-refund'
import { logger } from '../../../utils/logger'
import { createPayment as createPaymentOrder, invokeWechatPay } from '../../../api/payment'
import Navigation from '../../../utils/navigation'

type RefundView = RefundOrder & {
    _amountDisplay: string
    _statusText: string
    _statusClass: string
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
        paymentMethodText: '',
        showCloseButton: false,
        showPayButton: false,
        paying: false,
        showRefundList: false
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
            const result = await getPayments({ order_id: orderId, page: 1, page_size: 1 })
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
        const amountDisplay = `¥${(payment.amount / 100).toFixed(2)}`
        const statusText = this.getStatusText(payment.status)
        const statusClass = payment.status
        const paymentMethodText = this.getPaymentMethodText(payment.payment_type)
        const showCloseButton = payment.status === 'pending'
        const showPayButton = payment.status === 'pending' && !!payment.order_id && payment.payment_type === 'miniprogram'
        const showRefundList = false

        this.setData({
            payment,
            amountDisplay,
            statusText,
            statusClass,
            paymentMethodText,
            showCloseButton,
            showPayButton,
            showRefundList
        })
    },

    async onContinuePay() {
        if (this.data.paying) return

        const payment = this.data.payment
        if (!payment || !payment.order_id) {
            wx.showToast({ title: '订单信息缺失', icon: 'none' })
            return
        }

        this.setData({ paying: true })
        wx.showLoading({ title: '拉起支付...' })
        try {
            const latestPayment = await createPaymentOrder({
                order_id: payment.order_id,
                payment_type: 'miniprogram',
                business_type: payment.business_type || 'order'
            })

            if (latestPayment.pay_params) {
                try {
                    await invokeWechatPay(latestPayment.pay_params)
                } catch (error: unknown) {
                    const wxError = error as { errMsg?: string }
                    if (wxError?.errMsg?.includes('cancel')) {
                        closePayment(latestPayment.id).catch(() => {})
                        wx.showToast({ title: '已取消支付', icon: 'none' })
                        return
                    }
                    throw error
                }
            } else if (latestPayment.status !== 'paid') {
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

            wx.showToast({ title: '支付成功', icon: 'success' })
            if (latestPayment.id) {
                this.setData({ paymentId: latestPayment.id })
            }
            setTimeout(() => this.loadPaymentDetail(), 1200)
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
            const processedRefunds: RefundView[] = refundsResponse.refund_orders.map((refund) => ({
                ...refund,
                _amountDisplay: `¥${(refund.refund_amount / 100).toFixed(2)}`,
                _statusText: this.getRefundStatusText(refund.status),
                _statusClass: refund.status
            }))
            this.setData({ refunds: processedRefunds, showRefundList: processedRefunds.length > 0 })
        } catch (error) {
            logger.error('加载退款列表失败', error, 'payment-detail.loadRefunds')
        }
    },

    getStatusText(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待支付',
            'paid': '已支付',
            'refunded': '已退款',
            'closed': '已关闭'
        }
        return statusMap[status] || status
    },

    getPaymentMethodText(method: string): string {
        const methodMap: Record<string, string> = {
            'miniprogram': '小程序支付',
            'native': '扫码支付'
        }
        return methodMap[method] || method
    },

    getRefundStatusText(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '退款中',
            'processing': '处理中',
            'success': '退款成功',
            'failed': '退款失败'
        }
        return statusMap[status] || status
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
                        wx.hideLoading()
                        wx.showToast({ title: '已关闭', icon: 'success' })
                        setTimeout(() => this.loadPaymentDetail(), 1500)
                    } catch (error) {
                        wx.hideLoading()
                        logger.error('关闭支付失败', error, 'payment-detail.onClosePayment')
                        wx.showToast({ title: '操作失败', icon: 'error' })
                    }
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
