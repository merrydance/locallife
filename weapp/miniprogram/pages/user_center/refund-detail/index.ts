import { buildRefundProgress, getRefundById, getRefundReturns, getRefundStatusView, ProfitSharingReturn, RefundOrder, RefundProgressView } from '../../../api/payment'
import { logger } from '../../../utils/logger'

Page({
    data: {
        refundId: 0,
        refund: null as RefundOrder | null,
        navBarHeight: 88,
        loading: false,
        initialLoading: true,
        error: null as string | null,
        // 显示字段
        amountDisplay: '',
        statusText: '',
        statusClass: '',
        statusIcon: 'info-circle-filled',
        refundTypeText: '',
        progress: [] as RefundProgressView[],
        profitSharingReturns: [] as ProfitSharingReturn[],
        showPendingTip: false
    },

    onLoad(options: { id?: string }) {
        if (options.id) {
            this.setData({ refundId: parseInt(options.id) })
            this.loadRefundDetail()
        }
    },

    async loadRefundDetail() {
        if (!this.data.refundId) return
        this.setData({ loading: true, error: null })
        try {
            const refund = await getRefundById(this.data.refundId)
            this.processRefund(refund)
            try {
                const returns = await getRefundReturns(this.data.refundId)
                this.setData({ profitSharingReturns: returns || [] })
            } catch (returnErr) {
                logger.warn('加载分账回退记录失败', returnErr, 'refund-detail.loadRefundDetail')
            }
            this.setData({ initialLoading: false, loading: false })
        } catch (error) {
            logger.error('加载退款详情失败', error, 'refund-detail.loadRefundDetail')
            this.setData({ 
                initialLoading: false, 
                loading: false,
                error: '加载退款详情失败'
            })
        }
    },

    onRetry() {
        this.loadRefundDetail()
    },

    processRefund(refund: RefundOrder) {
        const statusView = getRefundStatusView(refund.status)
        const amountDisplay = `¥${(refund.refund_amount / 100).toFixed(2)}`
        const statusText = statusView.text
        const statusClass = statusView.className
        const refundTypeText = refund.refund_type === 'full' ? '全额退款' : '部分退款'
        const progress = buildRefundProgress(refund, this.formatTime)

        this.setData({
            refund,
            amountDisplay,
            statusText,
            statusClass,
            statusIcon: statusView.icon,
            refundTypeText,
            progress,
            showPendingTip: statusView.showPendingTip
        })
    },

    formatTime(timeStr: string): string {
        if (!timeStr) return ''
        try {
            const date = new Date(timeStr)
            const m = ('0' + (date.getMonth() + 1)).slice(-2)
            const d = ('0' + date.getDate()).slice(-2)
            const h = ('0' + date.getHours()).slice(-2)
            const min = ('0' + date.getMinutes()).slice(-2)
            return `${m}-${d} ${h}:${min}`
        } catch {
            return timeStr
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    }
})
