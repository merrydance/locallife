import { getRefundById, Refund } from '../../../api/payment-refund'
import { logger } from '../../../utils/logger'

interface RefundProgress {
    title: string
    time: string
    done: boolean
    active: boolean
}

Page({
    data: {
        refundId: 0,
        refund: null as Refund | null,
        navBarHeight: 88,
        loading: true,
        // 显示字段
        amountDisplay: '',
        statusText: '',
        statusClass: '',
        refundTypeText: '',
        progress: [] as RefundProgress[]
    },

    onLoad(options: { id?: string }) {
        if (options.id) {
            this.setData({ refundId: parseInt(options.id) })
            this.loadRefundDetail()
        }
    },

    async loadRefundDetail() {
        this.setData({ loading: true })
        try {
            const refund = await getRefundById(this.data.refundId)
            this.processRefund(refund)
        } catch (error) {
            logger.error('加载退款详情失败', error, 'refund-detail.loadRefundDetail')
            wx.showToast({ title: '加载失败', icon: 'error' })
        } finally {
            this.setData({ loading: false })
        }
    },

    processRefund(refund: Refund) {
        const amountDisplay = `¥${(refund.amount / 100).toFixed(2)}`
        const statusText = this.getStatusText(refund.status)
        const statusClass = refund.status
        const refundTypeText = refund.refund_type === 'full' ? '全额退款' : '部分退款'
        const progress = this.generateProgress(refund)

        this.setData({
            refund,
            amountDisplay,
            statusText,
            statusClass,
            refundTypeText,
            progress
        })
    },

    getStatusText(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '退款申请中',
            'processing': '退款处理中',
            'success': '退款成功',
            'failed': '退款失败'
        }
        return statusMap[status] || status
    },

    generateProgress(refund: Refund): RefundProgress[] {
        const progress: RefundProgress[] = [
            {
                title: '提交申请',
                time: this.formatTime(refund.created_at),
                done: true,
                active: refund.status === 'pending'
            },
            {
                title: '审核中',
                time: '',
                done: ['processing', 'success', 'failed'].includes(refund.status),
                active: refund.status === 'processing'
            },
            {
                title: '退款处理',
                time: '',
                done: ['success', 'failed'].includes(refund.status),
                active: false
            },
            {
                title: refund.status === 'failed' ? '退款失败' : '退款完成',
                time: refund.processed_at ? this.formatTime(refund.processed_at) : '',
                done: ['success', 'failed'].includes(refund.status),
                active: ['success', 'failed'].includes(refund.status)
            }
        ]
        return progress
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
