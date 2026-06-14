import RiderService from '../_main_shared/api/rider'
import {
    buildRiderDepositFinanceView,
    getRiderDepositWithdrawStatusView,
    type RiderDepositFinanceView
} from '../_services/rider-deposit-finance'
import {
    continueStoredRiderDepositRecharge,
    getPendingRiderDepositRecharge,
    getRiderDepositRechargeWorkflowStatusView,
    recoverStoredRiderDepositRecharge,
    submitRiderDepositRecharge,
    type RiderDepositPendingRechargeContext,
    type RiderDepositRechargeWorkflowResult
} from '../_main_shared/services/rider-deposit-payment'
import {
    buildRiderDepositWithdrawalStatusData, clearPendingRiderDepositWithdrawal,
    buildStoredRiderDepositWithdrawalSyncFailedView, recoverStoredRiderDepositWithdrawalStatus,
    waitForSubmittedRiderDepositWithdrawalTerminalStatus,
    type RiderDepositWithdrawalStatusView
} from '../_services/rider-deposit-withdrawal'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { buildDepositBillRecordView, formatFenValue, type DepositRecordView } from '../_utils/rider-deposit-record-view'

interface AmountInputDataset {
    field?: 'rechargeAmount' | 'withdrawAmount'
}

interface UserMessageError {
    userMessage?: string
}

interface RechargeWorkflowOptions {
    silent?: boolean
    refreshIfNeeded?: boolean
}

type WithdrawalStatusOptions = { silent?: boolean, refreshIfTerminal?: boolean, waitForTerminal?: boolean }

type ActionFeedbackTheme = 'success' | 'warning'

const FINANCE_REFRESH_DELAY_MS = [1200, 4000] as const

let financeRefreshPromise: Promise<void> | null = null
let financeRefreshTimerIds: number[] = []

function buildDepositWithdrawalIdempotencyKey() {
    return `rider-deposit-withdrawal:${Date.now()}:${Math.random().toString(36).slice(2, 10)}`
}

function formatFenDraftAmount(amount: number | undefined): string {
    if (!Number.isFinite(amount)) {
        return ''
    }

    return (Math.max(amount || 0, 0) / 100).toFixed(2)
}

function showDepositLoadingToast(context: WechatMiniprogram.Page.TrivialInstance, message: string) {
    void context
    wx.showLoading({ title: message, mask: true })
}

function hideDepositToast(context: WechatMiniprogram.Page.TrivialInstance) {
    void context
    wx.hideLoading()
}

function showDepositResultToast(
    context: WechatMiniprogram.Page.TrivialInstance,
    message: string,
    theme: 'success' | 'warning' | 'error' = 'warning'
) {
    void context
    wx.showToast({
        title: message,
        icon: theme === 'success' ? 'success' : 'none',
        duration: 1800
    })
}

Page({
    data: {
        navBarHeight: 88,
        loading: false,
        refreshing: false,
        loadingMore: false,
        hasLoadedOnce: false,
        accountError: '',
        listError: '',
        actionFeedbackMessage: '',
        actionFeedbackTheme: 'success' as ActionFeedbackTheme,

        totalDeposit: 0,
        totalDepositDisplay: '0.00',
        frozenDeposit: 0,
        deliveryFrozenDeposit: 0,
        deliveryFrozenDepositDisplay: '0.00',
        withdrawalProcessingAmount: 0,
        withdrawalProcessingAmountDisplay: '0.00',
        activeDeliveries: 0,
        availableDeposit: 0,
        availableDepositDisplay: '0.00',
        canWithdraw: false,
        withdrawHint: '可提现金额需至少 1.00 元',

        hasPendingRecharge: false,
        pendingRechargeTitle: '',
        pendingRechargeDescription: '',
        pendingRechargeAmountDisplay: '',
        pendingRechargePaymentId: 0,
        syncingPendingRecharge: false,

        hasPendingWithdrawal: false,
        pendingWithdrawalTitle: '',
        pendingWithdrawalDescription: '',
        pendingWithdrawalAmountDisplay: '',
        pendingWithdrawalStatusText: '',
        pendingWithdrawalTagTheme: 'warning',
        pendingWithdrawalPanelTheme: 'warning',
        pendingWithdrawalCanRefresh: false,
        syncingPendingWithdrawal: false,

        transactions: [] as DepositRecordView[],
        pageID: 1,
        hasMore: true,

        isRechargeVisible: false,
        rechargeAmount: '',
        isWithdrawVisible: false,
        withdrawAmount: '',
        withdrawErrorMessage: '',
        withdrawalIdempotencyKey: '',
        withdrawalIdempotencyKeySubmitted: false,
        withdrawalSubmittedAmount: '',
        rechargeSubmitting: false,
        withdrawSubmitting: false
    },

    onLoad() {
        const { navBarHeight } = getStableBarHeights()
        this.setData({ navBarHeight })
        this.reloadPage(true)
    },

    onHide() {
        this.clearScheduledFinanceRefreshes()
    },

    onUnload() {
        this.clearScheduledFinanceRefreshes()
        financeRefreshPromise = null
    },

    onShow() {
        if (!this.data.hasLoadedOnce || this.data.loading || this.data.refreshing) {
            return
        }

        void this.refreshAccount()
        void this.syncPendingRechargeState({ silent: false })
        void this.syncPendingWithdrawalState({ silent: true, refreshIfTerminal: true })
    },

    setActionFeedback(message: string, theme: ActionFeedbackTheme = 'success') {
        this.setData({ actionFeedbackMessage: message, actionFeedbackTheme: theme })
    },

    clearActionFeedback() {
        if (!this.data.actionFeedbackMessage) {
            return
        }

        this.setData({ actionFeedbackMessage: '' })
    },

    updateFinanceView(pendingRecharge: RiderDepositPendingRechargeContext | null) {
        const financeView: RiderDepositFinanceView = buildRiderDepositFinanceView({
            availableDeposit: this.data.availableDeposit,
            deliveryFrozenDeposit: this.data.deliveryFrozenDeposit,
            withdrawalProcessingAmount: this.data.withdrawalProcessingAmount,
            activeDeliveries: this.data.activeDeliveries,
            pendingRecharge
        })

        this.setData({
            canWithdraw: financeView.canWithdraw,
            withdrawHint: financeView.withdrawHint,
            hasPendingRecharge: financeView.hasPendingRecharge,
            pendingRechargeTitle: financeView.pendingRechargeTitle,
            pendingRechargeDescription: financeView.pendingRechargeDescription,
            pendingRechargeAmountDisplay: financeView.pendingRechargeAmountDisplay,
            pendingRechargePaymentId: pendingRecharge?.paymentOrderId || 0
        })
    },

    clearScheduledFinanceRefreshes() {
        financeRefreshTimerIds.forEach((timerId) => clearTimeout(timerId))
        financeRefreshTimerIds = []
    },

    applyWithdrawalStatusView(view: RiderDepositWithdrawalStatusView | null) {
        this.setData(buildRiderDepositWithdrawalStatusData(view))
    },

    clearWithdrawalDraftIdempotencyKey(idempotencyKey?: string) {
        if (!this.data.withdrawalIdempotencyKey) {
            return
        }
        if (idempotencyKey && this.data.withdrawalIdempotencyKey !== idempotencyKey) {
            return
        }

        this.setData({
            withdrawalIdempotencyKey: '',
            withdrawalIdempotencyKeySubmitted: false,
            withdrawalSubmittedAmount: ''
        })
    },

    async refreshFinanceSurfaces() {
        if (financeRefreshPromise) {
            return financeRefreshPromise
        }

        financeRefreshPromise = Promise.all([this.refreshAccount(), this.loadTransactions(1, true)])
            .then(() => undefined)
            .finally(() => {
                financeRefreshPromise = null
            })

        return financeRefreshPromise
    },

    async refreshFinanceAndPendingWithdrawal() {
        await this.refreshFinanceSurfaces()
        await this.syncPendingWithdrawalState({ silent: true, refreshIfTerminal: false })
    },

    async reloadPage(showLoading: boolean = false) {
        if (showLoading) {
            this.setData({ loading: true, actionFeedbackMessage: '' })
        } else {
            this.setData({ refreshing: true })
        }

        this.setData({ pageID: 1, hasMore: true })

        try {
            await Promise.all([
                this.refreshAccount(),
                this.loadTransactions(1, true),
                this.syncPendingRechargeState({ silent: true }),
                this.syncPendingWithdrawalState({ silent: true, refreshIfTerminal: false })
            ])
            this.setData({ hasLoadedOnce: true })
        } finally {
            this.setData({ loading: false, refreshing: false })
        }
    },

    async refreshAccount() {
        try {
            const [balance, riderStatus] = await Promise.all([
                RiderService.getDepositBalance(),
                RiderService.getStatus().catch((error: unknown) => {
                    logger.warn('Fetch rider deposit status failed', error)
                    return null
                })
            ])
            const withdrawalProcessingAmount = balance.withdrawal_processing_amount || 0
            const deliveryFrozenDeposit = typeof balance.delivery_frozen_deposit === 'number'
                ? balance.delivery_frozen_deposit
                : Math.max((balance.frozen_deposit || 0) - withdrawalProcessingAmount, 0)
            const activeDeliveries = riderStatus?.active_deliveries || 0
            this.setData({
                totalDeposit: balance.total_deposit,
                totalDepositDisplay: formatFenValue(balance.total_deposit),
                frozenDeposit: balance.frozen_deposit,
                deliveryFrozenDeposit,
                deliveryFrozenDepositDisplay: formatFenValue(deliveryFrozenDeposit),
                withdrawalProcessingAmount,
                withdrawalProcessingAmountDisplay: formatFenValue(withdrawalProcessingAmount),
                activeDeliveries,
                availableDeposit: balance.available_deposit,
                availableDepositDisplay: formatFenValue(balance.available_deposit),
                accountError: ''
            })
            this.updateFinanceView(getPendingRiderDepositRecharge())
        } catch (err: unknown) {
            logger.error('Fetch deposit balance failed', err)
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '押金账户加载失败，请稍后重试'
            this.setData({ accountError: message })
        }
    },

    async loadTransactions(page: number = 1, reset: boolean = false) {
        this.setData({ loadingMore: !reset && page > 1 })
        try {
            const resp = await RiderService.listDepositRecords({ page, limit: 20 })
            const list = (resp.deposits || []).map((item) => buildDepositBillRecordView(item)).filter((item): item is DepositRecordView => Boolean(item))
            const pageSize = resp.page_size || 20
            const total = typeof resp.total === 'number' ? resp.total : 0
            this.setData({
                transactions: reset ? list : [...this.data.transactions, ...list],
                hasMore: page * pageSize < total,
                pageID: resp.page_id || page,
                listError: ''
            })
        } catch (err: unknown) {
            logger.error('Fetch deposit logs failed', err)
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '账单明细加载失败，请稍后重试'
            this.setData({
                listError: message,
                transactions: reset ? [] : this.data.transactions,
                hasMore: false
            })
        } finally {
            this.setData({ loadingMore: false })
        }
    },

    async syncPendingWithdrawalState(options: WithdrawalStatusOptions = {}) {
        this.setData({ syncingPendingWithdrawal: true })
        try {
            const result = await recoverStoredRiderDepositWithdrawalStatus({ waitForTerminal: options.waitForTerminal })
            if (!result) {
                this.applyWithdrawalStatusView(null)
                return
            }

            this.applyWithdrawalStatusView(result.view)
            if (result.idempotencyKey && !this.data.withdrawalIdempotencyKey) {
                const requestedAmount = formatFenDraftAmount(result.requestedAmount)
                this.setData({
                    withdrawalIdempotencyKey: result.idempotencyKey,
                    withdrawalIdempotencyKeySubmitted: true,
                    withdrawalSubmittedAmount: requestedAmount,
                    withdrawAmount: requestedAmount || this.data.withdrawAmount
                })
            }
            if (!options.silent) {
                this.setActionFeedback(result.view.feedbackMessage, result.view.feedbackTheme)
            }

            if (result.isTerminal) {
                this.clearWithdrawalDraftIdempotencyKey(result.idempotencyKey)
                if (options.refreshIfTerminal !== false && result.shouldRefreshFinance) {
                    await this.refreshFinanceSurfaces()
                }
            }
        } catch (err: unknown) {
            logger.error('Recover rider deposit withdrawal failed', err)
            const failedView = buildStoredRiderDepositWithdrawalSyncFailedView()
            this.applyWithdrawalStatusView(failedView)
            if (!options.silent) {
                this.setActionFeedback('提现状态同步失败，系统会继续同步，也可稍后查看账单明细。', 'warning')
            }
        } finally {
            this.setData({ syncingPendingWithdrawal: false })
        }
    },

    async syncPendingRechargeState(options: { silent?: boolean } = {}) {
        const pendingRecharge = getPendingRiderDepositRecharge()
        if (!pendingRecharge) {
            this.updateFinanceView(null)
            return
        }

        this.setData({ syncingPendingRecharge: true })
        try {
            const result = await recoverStoredRiderDepositRecharge()
            if (!result) {
                this.updateFinanceView(null)
                return
            }

            await this.applyRechargeWorkflowResult(result, {
                silent: options.silent,
                refreshIfNeeded: result.shouldRefresh
            })
        } catch (err: unknown) {
            logger.error('Recover rider deposit recharge failed', err)
            this.updateFinanceView(pendingRecharge)
            if (!options.silent) {
                this.setActionFeedback('待确认充值状态同步失败，可稍后再试。', 'warning')
            }
        } finally {
            this.setData({ syncingPendingRecharge: false })
        }
    },

    async applyRechargeWorkflowResult(
        result: RiderDepositRechargeWorkflowResult,
        options: RechargeWorkflowOptions = {}
    ) {
        const statusView = getRiderDepositRechargeWorkflowStatusView(result.status)
        this.updateFinanceView(result.pendingContext)

        if (statusView.isPaid) {
            if (!options.silent) {
                this.setActionFeedback(statusView.feedbackMessage, statusView.feedbackTheme)
            }
            if (options.refreshIfNeeded) {
                await this.refreshFinanceSurfaces()
            }
            return
        }

        if (statusView.isFailed) {
            if (!options.silent) {
                this.setActionFeedback(statusView.feedbackMessage, statusView.feedbackTheme)
            }
            if (options.refreshIfNeeded) {
                await this.refreshFinanceSurfaces()
            }
            return
        }

        if (statusView.isCancelled) {
            if (!options.silent) {
                this.setActionFeedback(statusView.feedbackMessage, statusView.feedbackTheme)
            }
            return
        }

        if (statusView.isPendingConfirmation) {
            if (!options.silent) {
                this.setActionFeedback(statusView.feedbackMessage, statusView.feedbackTheme)
            }
        }
    },

    onRefresh() {
        this.reloadPage(false)
    },

    async onRefreshPendingWithdrawal() {
        if (this.data.syncingPendingWithdrawal) {
            return
        }

        this.clearActionFeedback()
        showDepositLoadingToast(this, '正在等待微信提现结果...')
        try {
            await this.syncPendingWithdrawalState({ silent: false, refreshIfTerminal: true, waitForTerminal: true })
        } finally {
            hideDepositToast(this)
        }
    },

    onShowRecharge() {
        if (this.data.hasPendingRecharge) {
            showDepositResultToast(this, '当前有待确认充值，请先完成该笔支付')
            return
        }

        this.clearActionFeedback()
        this.setData({ isRechargeVisible: true, rechargeAmount: '' })
    },

    onShowWithdraw() {
        if (!this.data.canWithdraw) {
            showDepositResultToast(this, this.data.withdrawHint)
            return
        }

        this.clearActionFeedback()
        const submittedIdempotencyKey = this.data.withdrawalIdempotencyKeySubmitted
            ? this.data.withdrawalIdempotencyKey
            : ''
        const submittedAmount = submittedIdempotencyKey ? this.data.withdrawalSubmittedAmount : ''
        this.setData({
            isWithdrawVisible: true,
            withdrawAmount: submittedAmount,
            withdrawErrorMessage: '',
            withdrawalIdempotencyKey: submittedIdempotencyKey || buildDepositWithdrawalIdempotencyKey(),
            withdrawalIdempotencyKeySubmitted: Boolean(submittedIdempotencyKey),
            withdrawalSubmittedAmount: submittedAmount
        })
    },

    onInputAmount(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        const { field } = e.currentTarget.dataset as AmountInputDataset
        if (!field) {
            return
        }

        this.clearActionFeedback()
        if (field === 'withdrawAmount' && this.data.withdrawalIdempotencyKeySubmitted) {
            this.setData({ withdrawErrorMessage: '本次提现已提交，请等待结果同步后再重新填写金额' })
            return
        }

        const nextState: Record<string, string> = { [field]: e.detail.value }
        if (field === 'withdrawAmount' && this.data.withdrawErrorMessage) {
            nextState.withdrawErrorMessage = ''
        }
        this.setData(nextState)
    },

    onCloseRechargeDialog() {
        this.setData({ isRechargeVisible: false, rechargeAmount: '' })
    },

    onCloseWithdrawDialog() {
        if (this.data.withdrawSubmitting) {
            return
        }

        if (this.data.withdrawalIdempotencyKeySubmitted) {
            this.setData({
                isWithdrawVisible: false,
                withdrawErrorMessage: ''
            })
            return
        }

        this.setData({
            isWithdrawVisible: false,
            withdrawAmount: '',
            withdrawErrorMessage: '',
            withdrawalIdempotencyKey: '',
            withdrawalIdempotencyKeySubmitted: false,
            withdrawalSubmittedAmount: ''
        })
    },

    scheduleFinanceRefresh() {
        this.clearScheduledFinanceRefreshes()
        financeRefreshTimerIds = FINANCE_REFRESH_DELAY_MS.map((delayMs) => setTimeout(() => {
            void this.refreshFinanceAndPendingWithdrawal()
        }, delayMs) as unknown as number)
    },

    async confirmRecharge() {
        if (this.data.rechargeSubmitting) {
            return
        }

        if (this.data.hasPendingRecharge) {
            showDepositResultToast(this, '当前有待确认充值，请先完成该笔支付')
            return
        }

        const amount = parseFloat(this.data.rechargeAmount)
        if (Number.isNaN(amount) || amount < 1 || amount > 10000) {
            showDepositResultToast(this, '请输入正确金额')
            return
        }

        this.clearActionFeedback()
        this.setData({ rechargeSubmitting: true })
        showDepositLoadingToast(this, '正在发起支付...')
        try {
            const result = await submitRiderDepositRecharge(Math.round(amount * 100), { context: this })
            hideDepositToast(this)
            this.setData({ isRechargeVisible: false, rechargeAmount: '' })
            await this.applyRechargeWorkflowResult(result, { refreshIfNeeded: result.shouldRefresh })
        } catch (err: unknown) {
            hideDepositToast(this)
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '充值失败'
            showDepositResultToast(this, message, 'error')
        } finally {
            this.setData({ rechargeSubmitting: false })
        }
    },

    async onContinuePendingRecharge() {
        if (this.data.syncingPendingRecharge || this.data.rechargeSubmitting) {
            return
        }

        const pendingRecharge = getPendingRiderDepositRecharge()
        if (!pendingRecharge) {
            showDepositResultToast(this, '暂无待确认充值')
            this.updateFinanceView(null)
            return
        }

        this.clearActionFeedback()
        this.setData({ syncingPendingRecharge: true })
        showDepositLoadingToast(this, '正在拉起支付...')
        try {
            const result = await continueStoredRiderDepositRecharge({ context: this })
            if (!result) {
                hideDepositToast(this)
                this.updateFinanceView(null)
                showDepositResultToast(this, '暂无待确认充值')
                return
            }

            hideDepositToast(this)
            await this.applyRechargeWorkflowResult(result, { refreshIfNeeded: result.shouldRefresh })
        } catch (err: unknown) {
            hideDepositToast(this)
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '继续支付失败'
            showDepositResultToast(this, message, 'error')
        } finally {
            this.setData({ syncingPendingRecharge: false })
        }
    },

    onViewPendingRechargeDetail() {
        const paymentOrderId = this.data.pendingRechargePaymentId
        if (!paymentOrderId) {
            showDepositResultToast(this, '暂无支付进度可查看')
            return
        }

        wx.navigateTo({ url: `/pages/user_center/payment-detail/index?id=${paymentOrderId}` })
    },

    async confirmWithdraw() {
        if (this.data.withdrawSubmitting) {
            return
        }

        const amount = parseFloat(this.data.withdrawAmount)
        if (Number.isNaN(amount) || amount < 1 || amount > 50000) {
            this.setData({ withdrawErrorMessage: '请输入正确提现金额' })
            return
        }

        const amountFen = Math.round(amount * 100)
        if (amountFen > this.data.availableDeposit) {
            this.setData({ withdrawErrorMessage: '可用押金不足' })
            return
        }

        this.clearActionFeedback()
        const idempotencyKey = this.data.withdrawalIdempotencyKey || buildDepositWithdrawalIdempotencyKey()
        this.setData({
            withdrawSubmitting: true,
            withdrawErrorMessage: '',
            withdrawalIdempotencyKey: idempotencyKey,
            withdrawalIdempotencyKeySubmitted: true,
            withdrawalSubmittedAmount: this.data.withdrawAmount
        })
        showDepositLoadingToast(this, '正在提交提现...')
        try {
            const result = await RiderService.withdrawDeposit({
                amount: amountFen,
                remark: '骑手押金提现'
            }, {
                idempotencyKey
            })
            hideDepositToast(this)
            const withdrawStatusView = getRiderDepositWithdrawStatusView(result.status)
            const hasPendingWithdrawal = result.status !== 'success' && result.status !== 'failed' && (result.refunds || []).length > 0

            if (hasPendingWithdrawal) {
                showDepositLoadingToast(this, '提现已受理，正在等待微信确认...')
                const terminalResult = await waitForSubmittedRiderDepositWithdrawalTerminalStatus(result, { idempotencyKey })
                hideDepositToast(this)
                if (!terminalResult) {
                    throw new Error('提现状态同步失败')
                }

                this.applyWithdrawalStatusView(terminalResult.view)
                this.setActionFeedback(
                    terminalResult.isTerminal
                        ? terminalResult.view.feedbackMessage
                        : '微信仍在处理本次提现，系统会继续同步，也可稍后查看账单明细。',
                    terminalResult.view.feedbackTheme
                )
                this.setData({ isWithdrawVisible: false, withdrawAmount: '' })
                if (terminalResult.shouldRefreshFinance) {
                    await this.refreshFinanceSurfaces()
                }
                if (!terminalResult.isTerminal) {
                    this.scheduleFinanceRefresh()
                } else {
                    this.clearWithdrawalDraftIdempotencyKey(idempotencyKey)
                }
                return
            } else {
                clearPendingRiderDepositWithdrawal()
                this.applyWithdrawalStatusView(null)
                this.setActionFeedback(withdrawStatusView.feedbackMessage, withdrawStatusView.feedbackTheme)
                this.clearWithdrawalDraftIdempotencyKey()
            }

            this.setData({ isWithdrawVisible: false, withdrawAmount: '' })
            await this.refreshFinanceSurfaces()
            if (withdrawStatusView.shouldScheduleRefresh || hasPendingWithdrawal) {
                this.scheduleFinanceRefresh()
            }
        } catch (err: unknown) {
            hideDepositToast(this)
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '提现失败'
            this.setData({ withdrawErrorMessage: message })
        } finally {
            this.setData({ withdrawSubmitting: false })
        }
    },

    onReachBottom() {
        if (this.data.loading || this.data.loadingMore || !this.data.hasMore || this.data.listError) {
            return
        }
        const nextPage = this.data.pageID + 1
        this.loadTransactions(nextPage, false)
    },

    onRetryPage() {
        this.reloadPage(true)
    },

    onRetryTransactions() {
        this.loadTransactions(1, true)
    },

    formatBalanceAmount(amount: number): string {
        return formatFenValue(amount)
    }
})
