import { getPaymentById, closePayment, getPaymentRefunds, getPayments, Payment, Refund } from '../../../api/payment-refund'
import { logger } from '../../../utils/logger'

Page({
    data: {
        paymentId: 0,
        payment: null as Payment | null,
        refunds: [] as Refund[],
        navBarHeight: 88,
        loading: true,
        // 显示字段
        amountDisplay: '',
        statusText: '',
        statusClass: '',
        paymentMethodText: '',
        showCloseButton: false,
        showRefundList: false
    },

    onLoad(options: { id?: string; orderId?: string }) {
        if (options.id) {
            this.setData({ paymentId: parseInt(options.id) })
            this.loadPaymentDetail()
        } else if (options.orderId) {
            // 通过订单ID查找支付记录
            this.loadPaymentByOrder(parseInt(options.orderId))
        }
    },

    async loadPaymentDetail() {
        this.setData({ loading: true })
        try {
            const payment = await getPaymentById(this.data.paymentId)
            this.processPayment(payment)

            // 如果有退款，加载退款列表
            if (payment.refund_status && payment.refund_status !== 'none') {
                await this.loadRefunds()
            }
        } catch (error) {
            logger.error('加载支付详情失败', error, 'payment-detail.loadPaymentDetail')
            wx.showToast({ title: '加载失败', icon: 'error' })
        } finally {
            this.setData({ loading: false })
        }
    },

    async loadPaymentByOrder(orderId: number) {
        this.setData({ loading: true })
        try {
            // 通过订单ID获取支付列表，取第一条
            const result = await getPayments({ order_id: orderId, page: 1, page_size: 1 })
            if (result.data && result.data.length > 0) {
                const payment = result.data[0]
                this.setData({ paymentId: payment.id })
                this.processPayment(payment)

                if (payment.refund_status && payment.refund_status !== 'none') {
                    await this.loadRefunds()
                }
            } else {
                wx.showToast({ title: '未找到支付记录', icon: 'none' })
            }
        } catch (error) {
            logger.error('加载支付详情失败', error, 'payment-detail.loadPaymentByOrder')
            wx.showToast({ title: '加载失败', icon: 'error' })
        } finally {
            this.setData({ loading: false })
        }
    },

    processPayment(payment: Payment) {
        const amountDisplay = `¥${(payment.amount / 100).toFixed(2)}`
        const statusText = this.getStatusText(payment.status)
        const statusClass = payment.status
        const paymentMethodText = this.getPaymentMethodText(payment.payment_method)
        const showCloseButton = payment.status === 'pending'
        const showRefundList = payment.refund_status && payment.refund_status !== 'none'

        this.setData({
            payment,
            amountDisplay,
            statusText,
            statusClass,
            paymentMethodText,
            showCloseButton,
            showRefundList
        })
    },

    async loadRefunds() {
        try {
            const refunds = await getPaymentRefunds(this.data.paymentId)
            // 处理退款显示字段
            const processedRefunds = refunds.map(refund => ({
                ...refund,
                _amountDisplay: `¥${(refund.amount / 100).toFixed(2)}`,
                _statusText: this.getRefundStatusText(refund.status),
                _statusClass: refund.status
            }))
            this.setData({ refunds: processedRefunds })
        } catch (error) {
            logger.error('加载退款列表失败', error, 'payment-detail.loadRefunds')
        }
    },

    getStatusText(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待支付',
            'paid': '已支付',
            'failed': '支付失败',
            'cancelled': '已取消',
            'refunded': '已退款'
        }
        return statusMap[status] || status
    },

    getPaymentMethodText(method: string): string {
        const methodMap: Record<string, string> = {
            'wechat_pay': '微信支付',
            'alipay': '支付宝',
            'balance': '余额支付',
            'credit': '信用支付'
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
        const { id } = e.currentTarget.dataset
        wx.navigateTo({
            url: `/pages/user_center/refund-detail/index?id=${id}`
        })
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    }
})
