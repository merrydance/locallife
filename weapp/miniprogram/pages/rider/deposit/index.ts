import RiderService from '../../../api/rider'
import { invokeWechatPay, pollPaymentStatus } from '../../../api/payment'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

interface DepositRecord {
    id: number
    amount: number
    type: string
    created_at: string
    status?: string
    remark?: string
}

interface DepositRecordView extends DepositRecord {
    display_type_text: string
    display_remark?: string
    display_time: string
    icon_name: 'add-circle' | 'remove-circle'
    status_text: string
    status_theme: 'primary' | 'success' | 'warning' | 'default'
}

interface AmountInputDetail {
    value: string
}

interface AmountInputDataset {
    field?: 'rechargeAmount' | 'withdrawAmount'
}

interface UserMessageError {
    userMessage?: string
}

const transactionTypeTextMap: Record<string, string> = {
    deposit: '押金充值',
    recharge: '押金充值',
    freeze: '接单预扣',
    unfreeze: '押金解冻',
    deduct: '押金扣减',
    refund: '押金退回',
    withdraw: '账单变动',
    withdraw_rollback: '提现回滚',
    split_income: '分账入账（微信零钱）',
    income: '分账入账（微信零钱）',
    earning: '分账入账（微信零钱）'
}

const transactionAmountSignMap: Record<string, 1 | -1> = {
    deposit: 1,
    recharge: 1,
    unfreeze: 1,
    refund: 1,
    withdraw_rollback: 1,
    split_income: 1,
    income: 1,
    earning: 1,
    freeze: -1,
    deduct: -1,
    withdraw: -1
}

function getTransactionSign(type: string): 1 | -1 | 0 {
    return transactionAmountSignMap[type] || 0
}

function formatFenToYuan(amount: number): string {
    return `¥${(Math.max(amount, 0) / 100).toFixed(2)}`
}

function formatFenValue(amount: number): string {
    return (Math.max(amount, 0) / 100).toFixed(2)
}

function formatTransactionTime(timeText?: string): string {
    if (!timeText) {
        return '--'
    }

    const date = new Date(timeText)
    if (Number.isNaN(date.getTime())) {
        return timeText
    }

    return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function buildWithdrawHint(availableDeposit: number, deliveryFrozenDeposit: number, withdrawalProcessingAmount: number): {
    canWithdraw: boolean
    withdrawHint: string
} {
    if (withdrawalProcessingAmount > 0 && deliveryFrozenDeposit > 0) {
        return {
            canWithdraw: false,
            withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 配送冻结，另有 ${formatFenToYuan(withdrawalProcessingAmount)} 提现处理中，暂不可再次提现`
        }
    }

    if (withdrawalProcessingAmount > 0) {
        return {
            canWithdraw: false,
            withdrawHint: `当前有 ${formatFenToYuan(withdrawalProcessingAmount)} 正在提现处理中，到账前暂不可再次提现`
        }
    }

    if (deliveryFrozenDeposit > 0) {
        return {
            canWithdraw: false,
            withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 配送冻结，待订单完成或取消后可提现`
        }
    }

    if (availableDeposit >= 100) {
        return {
            canWithdraw: true,
            withdrawHint: `当前可提现 ${formatFenToYuan(availableDeposit)}，提现将退回至微信零钱`
        }
    }

    return {
        canWithdraw: false,
        withdrawHint: `当前可提现 ${formatFenToYuan(availableDeposit)}，至少需满 ¥1.00 才能提现`
    }
}

function decorateDepositRecord(record: DepositRecord): DepositRecordView {
    const remark = record.remark || ''
    let displayTypeText = transactionTypeTextMap[record.type] || '账单变动'
    let displayRemark = remark
    let statusText = '已完成'
    let statusTheme: DepositRecordView['status_theme'] = 'default'

    if (record.type === 'freeze') {
        if (remark === '接单冻结押金') {
            displayTypeText = '配送冻结'
            displayRemark = '订单配送中，押金暂时冻结。'
            statusText = '冻结中'
            statusTheme = 'warning'
        } else if (remark === '押金提现冻结') {
            displayTypeText = '提现处理中'
            displayRemark = '提现申请处理中，到账前金额暂不可用。'
            statusText = '处理中'
            statusTheme = 'warning'
        }
    }

    if (record.type === 'unfreeze') {
        if (remark === '配送完成解冻押金') {
            displayTypeText = '配送解冻'
            displayRemark = '订单已完成，配送冻结已释放。'
            statusText = '已释放'
            statusTheme = 'success'
        } else if (remark === '订单取消解冻押金') {
            displayTypeText = '取消退回'
            displayRemark = '订单取消后，配送冻结已退回可用押金。'
            statusText = '已退回'
            statusTheme = 'success'
        } else if (remark === '押金退款失败解冻') {
            displayTypeText = '提现退回'
            displayRemark = '提现未成功，金额已退回可用押金。'
            statusText = '已退回'
            statusTheme = 'default'
        }
    }

    if (record.type === 'withdraw' && remark === '押金退款提现成功') {
        displayTypeText = '提现完成'
        displayRemark = '提现已退回微信零钱。'
        statusText = '已到账'
        statusTheme = 'success'
    }

    if ((record.type === 'deposit' || record.type === 'recharge') && remark === '微信支付充值') {
        displayRemark = '已通过微信支付完成押金充值。'
        statusText = '已充值'
        statusTheme = 'primary'
    }

    const sign = getTransactionSign(record.type)
    const iconName = sign === 1 ? 'add-circle' : 'remove-circle'

    return {
        ...record,
        display_type_text: displayTypeText,
        display_remark: displayRemark || undefined,
        display_time: formatTransactionTime(record.created_at),
        icon_name: iconName
        ,status_text: statusText,
        status_theme: statusTheme
    }
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
        refreshing: false,
        loadingMore: false,
        accountError: '',
        listError: '',
                actionNoticeMessage: '',
                actionNoticeTheme: 'success' as 'success' | 'warning',
    
    // 账户余额数据
    totalDeposit: 0,
    frozenDeposit: 0,
    deliveryFrozenDeposit: 0,
    withdrawalProcessingAmount: 0,
    availableDeposit: 0,
        canWithdraw: false,
        withdrawHint: '可提现金额需至少 1.00 元',
    
    // 提现/充值 状态
    transactions: [] as DepositRecordView[],
    pageID: 1,
    hasMore: true,
    
    // 弹窗控制
    isRechargeVisible: false,
        rechargeAmount: '',
        isWithdrawVisible: false,
        withdrawAmount: '',
        rechargeSubmitting: false,
        withdrawSubmitting: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
        this.reloadPage(true)
  },

    setActionNotice(message: string, theme: 'success' | 'warning' = 'success') {
        this.setData({ actionNoticeMessage: message, actionNoticeTheme: theme })
    },

    clearActionNotice() {
        if (!this.data.actionNoticeMessage) {
            return
        }
        this.setData({ actionNoticeMessage: '' })
    },

    async reloadPage(showLoading: boolean = false) {
        if (showLoading) {
            this.setData({ loading: true, actionNoticeMessage: '' })
        } else {
            this.setData({ refreshing: true })
        }

        this.setData({ pageID: 1, hasMore: true })

    try {
            await Promise.all([this.refreshAccount(), this.loadTransactions(1, true)])
    } finally {
            this.setData({ loading: false, refreshing: false })
    }
  },

    updateWithdrawState(availableDeposit: number, deliveryFrozenDeposit: number, withdrawalProcessingAmount: number) {
        const { canWithdraw, withdrawHint } = buildWithdrawHint(
            availableDeposit,
            deliveryFrozenDeposit,
            withdrawalProcessingAmount
        )

        this.setData({ canWithdraw, withdrawHint })
    },

    async refreshAccount() {
    try {
            const balance = await RiderService.getDepositBalance()
            const withdrawalProcessingAmount = balance.withdrawal_processing_amount || 0
            const deliveryFrozenDeposit = typeof balance.delivery_frozen_deposit === 'number'
                ? balance.delivery_frozen_deposit
                : Math.max((balance.frozen_deposit || 0) - withdrawalProcessingAmount, 0)
            this.setData({
                totalDeposit: balance.total_deposit,
                frozenDeposit: balance.frozen_deposit,
                deliveryFrozenDeposit,
                withdrawalProcessingAmount,
                availableDeposit: balance.available_deposit,
                accountError: ''
            })
            this.updateWithdrawState(balance.available_deposit, deliveryFrozenDeposit, withdrawalProcessingAmount)
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
            const list = (resp.deposits || []).map((item) => decorateDepositRecord(item))
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

    onRefresh() {
        this.reloadPage(false)
    },

  onShowRecharge() {
        this.clearActionNotice()
    this.setData({ isRechargeVisible: true, rechargeAmount: '' })
  },

    onShowWithdraw() {
        if (!this.data.canWithdraw) {
            wx.showToast({ title: this.data.withdrawHint, icon: 'none' })
            return
        }
                this.clearActionNotice()
        this.setData({ isWithdrawVisible: true, withdrawAmount: '' })
    },

    onInputAmount(e: WechatMiniprogram.CustomEvent<AmountInputDetail>) {
        const { field } = e.currentTarget.dataset as AmountInputDataset
        if (!field) return
        this.clearActionNotice()
    this.setData({ [field]: e.detail.value })
  },

    onCloseRechargeDialog() {
        this.setData({ isRechargeVisible: false, rechargeAmount: '' })
    },

    onCloseWithdrawDialog() {
        this.setData({ isWithdrawVisible: false, withdrawAmount: '' })
    },

    scheduleFinanceRefresh() {
        this.reloadPage(false)
        setTimeout(() => this.reloadPage(false), 1500)
        setTimeout(() => this.reloadPage(false), 4000)
  },

  /**
   * 提交充值
   */
  async confirmRecharge() {
        if (this.data.rechargeSubmitting) {
            return
        }

    const amount = parseFloat(this.data.rechargeAmount)
        if (isNaN(amount) || amount < 1 || amount > 10000) {
        wx.showToast({ title: '请输入正确金额', icon: 'none' })
        return
    }

        this.setData({ rechargeSubmitting: true })
        wx.showLoading({ title: '正在发起支付...' })
    try {
            const res = await RiderService.rechargeDeposit({
                amount: Math.round(amount * 100)
            })

            if (!res.pay_params) {
                wx.showToast({ title: '未获取到支付参数', icon: 'none' })
                return
            }

            try {
                await invokeWechatPay(res.pay_params)
            } catch (error: unknown) {
                const errMsg = error && typeof error === 'object' && 'errMsg' in error ? (error as { errMsg?: string }).errMsg : ''
                if (typeof errMsg === 'string' && errMsg.includes('cancel')) {
                    wx.showToast({ title: '已取消支付', icon: 'none' })
                    return
        }
                throw error
            }

            if (res.payment_order_id) {
                try {
                    await pollPaymentStatus(res.payment_order_id, 5, 1500)
                    this.setActionNotice('充值已完成，账户余额和账单已同步更新。')
                } catch (_error) {
                    this.setActionNotice('支付已提交，余额和账单会在稍后自动刷新。', 'warning')
                }
            } else {
                this.setActionNotice('支付已提交，余额和账单会在稍后自动刷新。', 'warning')
            }

            this.setData({ isRechargeVisible: false, rechargeAmount: '' })
            this.scheduleFinanceRefresh()
    } catch (err: unknown) {
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '充值失败'
            wx.showToast({ title: message, icon: 'none' })
    } finally {
            this.setData({ rechargeSubmitting: false })
            wx.hideLoading()
    }
  },

    async confirmWithdraw() {
        if (this.data.withdrawSubmitting) {
            return
        }

        const amount = parseFloat(this.data.withdrawAmount)
        if (isNaN(amount) || amount < 1 || amount > 50000) {
            wx.showToast({ title: '请输入正确提现金额', icon: 'none' })
            return
        }

        const amountFen = Math.round(amount * 100)
        if (amountFen > this.data.availableDeposit) {
            wx.showToast({ title: '可用押金不足', icon: 'none' })
            return
        }

        this.setData({ withdrawSubmitting: true })
        wx.showLoading({ title: '正在提交提现...' })
        try {
            const result = await RiderService.withdrawDeposit({
                amount: amountFen,
                remark: '骑手押金提现'
            })

            const message = result.status === 'success'
                ? '提现已完成，账单记录已经同步更新。'
                : '提现申请已提交，到账进度会同步到账单列表。'
            this.setActionNotice(message, result.status === 'success' ? 'success' : 'warning')
            this.setData({ isWithdrawVisible: false, withdrawAmount: '' })
            this.scheduleFinanceRefresh()
        } catch (err: unknown) {
            const userMessage = (err as UserMessageError).userMessage
            const message = typeof userMessage === 'string' && userMessage ? userMessage : '提现失败'
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            this.setData({ withdrawSubmitting: false })
            wx.hideLoading()
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
    },

    formatTransactionAmount(amount: number, type: string): string {
        const sign = getTransactionSign(type)
        if (sign === 1) {
            return `+${(Math.abs(amount) / 100).toFixed(2)}`
        }
        if (sign === -1) {
            return `-${(Math.abs(amount) / 100).toFixed(2)}`
        }
        const raw = amount / 100
        const prefix = raw > 0 ? '+' : ''
        return `${prefix}${raw.toFixed(2)}`
    },

    getTransactionAmountClass(amount: number, type: string): string {
        const sign = getTransactionSign(type)
        if (sign === 1) return 'positive'
        if (sign === -1) return 'negative'
        return amount >= 0 ? 'positive' : 'negative'
  }
})
