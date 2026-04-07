import {
    operatorAppealReviewService,
    claimManagementService,
    ClaimRecoveryResponse,
    getAppealStatusDisplay,
    getClaimRecoveryStatusDisplay,
    OperatorAppealDetailResponse
} from '../../../../api/appeals-customer-service'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface AppealDetailOptions {
    id?: string
}

interface AppealDetailView extends OperatorAppealDetailResponse {
    status_label: string
    status_theme: 'warning' | 'success' | 'danger'
    can_review: boolean
    claim_amount_display: string
    compensation_amount_display: string
    approved_amount_display: string
    order_amount_display: string
    evidence_files_view: string[]
    timeline_view: Array<{
        title: string
        operator: string
        timestamp: string
        notes: string
    }>
    related_order_view: {
        order_no: string
        merchant_name: string
        rider_name: string
        order_amount_display: string
        status: string
        created_at: string
    } | null
}

interface RecoveryView extends ClaimRecoveryResponse {
    status_label: string
    status_theme: 'warning' | 'success' | 'danger'
    can_waive: boolean
    recovery_amount_display: string
}

function fen2yuan(value?: number): string {
    return `¥${(Number(value || 0) / 100).toFixed(2)}`
}

function buildTimeline(detail: OperatorAppealDetailResponse) {
    const statusDisplay = getAppealStatusDisplay(detail.status)
    const backendTimeline = Array.isArray(detail.timeline) ? detail.timeline : []
    if (backendTimeline.length > 0) {
        return backendTimeline.map((item) => ({
            title: String(item.action || '状态更新'),
            operator: String(item.operator || '系统'),
            timestamp: String(item.timestamp || ''),
            notes: String(item.notes || '')
        }))
    }

    const fallback = [
        {
            title: '提交申诉',
            operator: String(detail.user_name || detail.appellant_id || '申诉人'),
            timestamp: String(detail.created_at || ''),
            notes: String(detail.reason || '')
        },
        detail.reviewed_at ? {
            title: statusDisplay.isApproved || statusDisplay.isCompensated ? '审核通过' : '审核驳回',
            operator: detail.reviewer_id ? `审核人 ${detail.reviewer_id}` : '运营审核',
            timestamp: String(detail.reviewed_at),
            notes: String(detail.review_notes || '')
        } : null,
        detail.compensated_at ? {
            title: '补偿完成',
            operator: '系统赔付',
            timestamp: String(detail.compensated_at),
            notes: detail.compensation_amount ? `赔付 ${fen2yuan(detail.compensation_amount)}` : ''
        } : null
    ]

    return fallback.filter(Boolean) as Array<{
        title: string
        operator: string
        timestamp: string
        notes: string
    }>
}

function adaptAppealDetail(detail: OperatorAppealDetailResponse): AppealDetailView {
    const statusDisplay = getAppealStatusDisplay(detail.status)
    const lookback = (detail.lookback_result as Record<string, unknown> | undefined) || {}
    const evidenceFiles = Array.isArray(detail.evidence_files)
        ? detail.evidence_files
        : Array.isArray(lookback.evidence_files)
            ? lookback.evidence_files as string[]
            : []
    const relatedOrder = detail.related_order || {}

    return {
        ...detail,
        status_label: statusDisplay.label,
        status_theme: statusDisplay.theme,
        can_review: statusDisplay.isPending,
        claim_amount_display: fen2yuan(detail.claim_amount),
        compensation_amount_display: fen2yuan(detail.compensation_amount),
        approved_amount_display: fen2yuan(detail.claim_approved_amount),
        order_amount_display: fen2yuan(detail.order_amount),
        evidence_files_view: evidenceFiles,
        timeline_view: buildTimeline(detail),
        related_order_view: detail.order_no || relatedOrder.order_number ? {
            order_no: String(relatedOrder.order_number || detail.order_no || '-'),
            merchant_name: String(relatedOrder.merchant_name || detail.merchant_name || '-'),
            rider_name: String(relatedOrder.rider_name || (detail.rider_id ? `骑手 ${detail.rider_id}` : '-')),
            order_amount_display: fen2yuan(relatedOrder.order_amount || detail.order_amount),
            status: String(relatedOrder.status || detail.order_status || '-'),
            created_at: String(relatedOrder.created_at || detail.order_created_at || '')
        } : null
    }
}

function adaptRecovery(recovery: ClaimRecoveryResponse): RecoveryView {
    const statusDisplay = getClaimRecoveryStatusDisplay(recovery.status)
    return {
        ...recovery,
        status_label: statusDisplay.label,
        status_theme: statusDisplay.theme,
        can_waive: statusDisplay.canWaive,
        recovery_amount_display: fen2yuan(recovery.recovery_amount)
    }
}

Page({
    data: {
        id: 0,
        appeal: null as AppealDetailView | null,
        recovery: null as RecoveryView | null,
        replyContent: '',
        compensationAmount: '',
        showRejectDialog: false,
        initialLoading: true,
        loading: false,
        submitting: false,
        recoverySubmitting: false,
        error: null as string | null,
        navBarHeight: 88
    },

    onLoad(options: AppealDetailOptions) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) })
            this.loadDetail(parseInt(options.id))
        } else {
            this.setData({ 
                initialLoading: false,
                error: '未提供申诉ID'
            })
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async loadDetail(id: number) {
        this.setData({ loading: true, error: null })
        try {
            const appeal = adaptAppealDetail(await operatorAppealReviewService.getAppealDetailForReview(id))
            this.setData({ 
                appeal,
                replyContent: appeal.review_notes || '',
                compensationAmount: appeal.compensation_amount ? (appeal.compensation_amount / 100).toFixed(2) : '',
                loading: false,
                initialLoading: false
            })
            if (appeal.claim_id) {
                await this.loadRecovery(appeal.claim_id)
            }
        } catch (error) {
            console.error('加载详情失败:', error)
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: getErrorUserMessage(error, '加载详情失败，请稍后重试')
            })
        }
    },

    async loadRecovery(claimId: number) {
        try {
            const recovery = await claimManagementService.getOperatorClaimRecovery(claimId)
            this.setData({ recovery: adaptRecovery(recovery) })
        } catch (error) {
            this.setData({ recovery: null })
        }
    },

    onRetry() {
        if (this.data.id) {
            this.loadDetail(this.data.id)
        }
    },

    onInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        this.setData({ replyContent: e.detail.value })
    },

    onCompensationInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        this.setData({ compensationAmount: e.detail.value })
    },

    onPreviewEvidence(e: WechatMiniprogram.TouchEvent) {
        const { url } = e.currentTarget.dataset as { url?: string }
        const urls = this.data.appeal?.evidence_files_view || []
        if (!url) return

        wx.previewImage({ current: url, urls })
    },

    async onApprove() {
        await this.handleAppeal('approve')
    },

    onReject() {
        this.setData({ showRejectDialog: true })
    },

    async onRejectConfirm() {
        await this.handleAppeal('reject')
        this.setData({ showRejectDialog: false })
    },

    onRejectCancel() {
        this.setData({ showRejectDialog: false })
    },

    async handleAppeal(reviewAction: 'approve' | 'reject') {
        const { id, replyContent, compensationAmount, appeal } = this.data
        if (!replyContent || replyContent.trim().length < 5) {
            wx.showToast({ title: '审核备注至少5个字符', icon: 'none' })
            return
        }

        let compensationFen: number | undefined
        if (reviewAction === 'approve') {
            const fallbackClaimAmount = Number((appeal as unknown as Record<string, unknown>)?.claim_amount || 0)
            const parsed = compensationAmount ? Math.floor(parseFloat(compensationAmount) * 100) : fallbackClaimAmount
            if (!parsed || parsed <= 0) {
                wx.showToast({ title: '通过时需填写补偿金额', icon: 'none' })
                return
            }
            compensationFen = parsed
        }

        try {
            this.setData({ submitting: true })
            wx.showLoading({ title: '处理中...', mask: true })
            await operatorAppealReviewService.reviewAppeal(id, {
                status: reviewAction === 'approve' ? 'approved' : 'rejected',
                review_notes: replyContent,
                compensation_amount: compensationFen
            })
            wx.navigateBack()
        } catch (error: unknown) {
            const message = getErrorUserMessage(error, '处理失败，请稍后重试')
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            this.setData({ submitting: false })
            wx.hideLoading()
        }
    },

    async onWaiveRecovery() {
        const { appeal } = this.data
        if (!appeal?.claim_id) {
            return
        }

        try {
            this.setData({ recoverySubmitting: true })
            wx.showLoading({ title: '处理中...', mask: true })
            await claimManagementService.waiveOperatorClaimRecovery(appeal.claim_id)
            await this.loadRecovery(appeal.claim_id)
        } catch (error: unknown) {
            const message = getErrorUserMessage(error, '核销失败，请稍后重试')
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            this.setData({ recoverySubmitting: false })
            wx.hideLoading()
        }
    }
})
