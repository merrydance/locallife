import { buildRefundProgress, getRefundById, getRefundReturns, getRefundStatusView, isRefundStatusTerminal, ProfitSharingReturn, RefundOrder, RefundProgressView } from '../../../api/payment'
import { logger } from '../../../utils/logger'
import { getProfitSharingReturnStatusView, isProfitSharingReturnTerminal, ProfitSharingReturnStatusTheme } from '../../../utils/profit-sharing-return-view'
import { waitForRefundTerminalResult } from '../../../services/refund-workflow'

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
        try {
            const result = await waitForRefundTerminalResult(this.data.refundId, {
                maxAttempts: 8,
                initialIntervalMs: 1000,
                maxIntervalMs: 8000,
                backoffFactor: 2,
                shouldContinue: () => token === refundTerminalWaitToken,
                onAttempt: (refund) => {
                    if (token !== refundTerminalWaitToken) {
                        return
                    }
                    this.processRefund(refund)
                    this.setData({
                        statusNote: isRefundStatusTerminal(refund.status)
                            ? ''
                            : '退款结果还在同步中，请稍后查看。'
                    })
                }
            })

            if (token !== refundTerminalWaitToken) {
                return
            }

            this.processRefund(result.refund)
            await this.loadProfitSharingReturns()
            this.setData({
                loading: false,
                waitingForTerminal: false,
                statusNote: result.terminal ? '' : '退款结果还在同步中，请稍后查看。',
                initialLoading: false
            })
        } catch (error) {
            if (token !== refundTerminalWaitToken) {
                return
            }
            logger.warn('等待退款终态失败', error, 'refund-detail.waitForTerminalRefund')
            this.setData({
                loading: false,
                waitingForTerminal: false,
                statusNote: '退款结果还在同步中，请稍后查看。',
                initialLoading: false
            })
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
