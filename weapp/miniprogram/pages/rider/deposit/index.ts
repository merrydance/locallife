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

Page({
  data: {
    navBarHeight: 88,
    loading: false,
        refreshing: false,
        loadingMore: false,
        accountError: '',
        listError: '',
    
    // 账户余额数据
    totalDeposit: 0,
    frozenDeposit: 0,
    availableDeposit: 0,
        canWithdraw: false,
        withdrawHint: '可提现金额需至少 1.00 元',
    
    // 提现/充值 状态
    transactions: [] as DepositRecord[],
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

    async reloadPage(showLoading: boolean = false) {
        if (showLoading) {
            this.setData({ loading: true })
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

    updateWithdrawState(availableDeposit: number, frozenDeposit: number) {
        let canWithdraw = false
        let withdrawHint = '可提现金额需至少 1.00 元'

        if (frozenDeposit > 0) {
            withdrawHint = '存在冻结押金时暂不可提现'
        } else if (availableDeposit >= 100) {
            canWithdraw = true
            withdrawHint = '提现将退回至微信零钱，如有进行中配送会被拦截'
        }

        this.setData({ canWithdraw, withdrawHint })
    },

    async refreshAccount() {
    try {
            const balance = await RiderService.getDepositBalance()
            this.setData({
                totalDeposit: balance.total_deposit,
                frozenDeposit: balance.frozen_deposit,
                availableDeposit: balance.available_deposit,
                accountError: ''
            })
            this.updateWithdrawState(balance.available_deposit, balance.frozen_deposit)
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
            const list = resp.deposits || []
            this.setData({
                transactions: reset ? list : [...this.data.transactions, ...list],
                hasMore: list.length >= (resp.page_size || 20),
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
    this.setData({ isRechargeVisible: true, rechargeAmount: '' })
  },

    onShowWithdraw() {
        if (!this.data.canWithdraw) {
            wx.showToast({ title: this.data.withdrawHint, icon: 'none' })
            return
        }
        this.setData({ isWithdrawVisible: true, withdrawAmount: '' })
    },

    onInputAmount(e: WechatMiniprogram.CustomEvent<AmountInputDetail>) {
        const { field } = e.currentTarget.dataset as AmountInputDataset
        if (!field) return
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
                    wx.showToast({ title: '充值成功', icon: 'success' })
                } catch (_error) {
                    wx.showToast({ title: '支付已提交，余额稍后刷新', icon: 'none' })
                }
            } else {
                wx.showToast({ title: '支付已提交', icon: 'success' })
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

            const message = result.status === 'success' ? '提现已完成' : '提现申请已提交'
            wx.showToast({ title: message, icon: 'success' })
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

    getTransactionTypeText(type: string): string {
        return transactionTypeTextMap[type] || '账单变动'
    },

    formatTransactionAmount(amount: number, type: string): string {
        const sign = transactionAmountSignMap[type]
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
        const sign = transactionAmountSignMap[type]
        if (sign === 1) return 'positive'
        if (sign === -1) return 'negative'
        return amount >= 0 ? 'positive' : 'negative'
  }
})
