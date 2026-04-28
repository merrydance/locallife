import { buildRefundProgress, getRefundById, getRefundReturns, getRefundStatusView, isRefundStatusTerminal, ProfitSharingReturn, RefundOrder, RefundProgressView } from '../../../api/payment'
import { logger } from '../../../utils/logger'
import { getProfitSharingReturnStatusView, isProfitSharingReturnTerminal, ProfitSharingReturnStatusTheme } from '../../../utils/profit-sharing-return-view'

const REFUND_TERMINAL_POLL_INTERVAL_MS = 2000

let refundTerminalWaitToken = 0

type ProfitSharingReturnView = ProfitSharingReturn & {
    amountDisplay: string
    statusText: string
    statusTheme: ProfitSharingReturnStatusTheme
    createdAtDisplay: string
    finishedAtDisplay: string
    displayTime: string
    failReasonText: string
}

function formatFen(amount: number): string {
    return (amount / 100).toFixed(2)
}

function buildProfitSharingReturnView(item: ProfitSharingReturn, formatTime: (timeStr: string) => string): ProfitSharingReturnView {
    const statusView = getProfitSharingReturnStatusView(item.status)
    return {
        ...item,
        amountDisplay: formatFen(item.amount),
        statusText: statusView.statusText,
        statusTheme: statusView.statusTheme,
        createdAtDisplay: formatTime(item.created_at),
        finishedAtDisplay: item.finished_at ? formatTime(item.finished_at) : '',
        displayTime: item.finished_at ? formatTime(item.finished_at) : formatTime(item.created_at),
        failReasonText: item.fail_reason || ''
    }
}

function delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms))
}

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
        profitSharingReturns: [] as ProfitSharingReturnView[],
        showPendingTip: false,
        waitingForTerminal: false,
        statusNote: '',
        refundReasonText: '',
        outRefundNoText: '',
        createdAtDisplay: '',
        refundedAtDisplay: ''
    },

    onLoad(options: { id?: string }) {
        if (options.id) {
            this.setData({ refundId: parseInt(options.id) })
            this.loadRefundDetail()
        }
    },

    onUnload() {
        refundTerminalWaitToken += 1
    },

    async loadRefundDetail() {
        if (!this.data.refundId) return
        this.setData({ loading: true, error: null })
        try {
            const refund = await getRefundById(this.data.refundId)
            if (!isRefundStatusTerminal(refund.status)) {
                this.startRefundTerminalWait()
                return
            }

            this.processRefund(refund)
            await this.loadProfitSharingReturns()
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

    startRefundTerminalWait() {
        const token = refundTerminalWaitToken + 1
        refundTerminalWaitToken = token
        this.setData({
            initialLoading: false,
            loading: true,
            waitingForTerminal: true,
            statusNote: '',
            refund: null,
            profitSharingReturns: []
        })
        void this.waitForTerminalRefund(token)
    },

    async waitForTerminalRefund(token: number) {
        while (token === refundTerminalWaitToken && this.data.refundId) {
            try {
                const refund = await getRefundById(this.data.refundId)
                if (isRefundStatusTerminal(refund.status)) {
                    if (token !== refundTerminalWaitToken) {
                        return
                    }
                    this.processRefund(refund)
                    await this.loadProfitSharingReturns()
                    this.setData({ loading: false, waitingForTerminal: false, statusNote: '', initialLoading: false })
                    return
                }

                this.setData({ statusNote: '' })
            } catch (error) {
                logger.warn('等待退款终态失败，将继续重试', error, 'refund-detail.waitForTerminalRefund')
                this.setData({ statusNote: '退款结果还没有回写完成，系统正在继续确认。' })
            }

            await delay(REFUND_TERMINAL_POLL_INTERVAL_MS)
        }
    },

    async loadProfitSharingReturns() {
        try {
            const returns = await getRefundReturns(this.data.refundId)
            this.setData({
                profitSharingReturns: (returns || [])
                    .filter((item) => isProfitSharingReturnTerminal(item.status))
                    .map((item) => buildProfitSharingReturnView(item, this.formatTime))
            })
        } catch (returnErr) {
            logger.warn('加载分账回退记录失败', returnErr, 'refund-detail.loadProfitSharingReturns')
        }
    },

    onRetry() {
        refundTerminalWaitToken += 1
        this.loadRefundDetail()
    },

    processRefund(refund: RefundOrder) {
        const statusView = getRefundStatusView(refund.status)
        const amountDisplay = formatFen(refund.refund_amount)
        const statusText = statusView.text
        const statusClass = statusView.className
        const refundTypeText = refund.refund_type === 'full' ? '全额退款' : '部分退款'
        const progress = buildRefundProgress(refund, this.formatTime)
        const refundReasonText = refund.refund_reason || '无'
        const outRefundNoText = refund.out_refund_no || '等待生成'
        const createdAtDisplay = this.formatTime(refund.created_at)
        const refundedAtDisplay = refund.refunded_at ? this.formatTime(refund.refunded_at) : ''

        this.setData({
            refund,
            amountDisplay,
            statusText,
            statusClass,
            statusIcon: statusView.icon,
            refundTypeText,
            progress,
            showPendingTip: statusView.showPendingTip,
            refundReasonText,
            outRefundNoText,
            createdAtDisplay,
            refundedAtDisplay
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
