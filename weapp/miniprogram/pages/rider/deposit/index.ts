import RiderService from '../../../api/rider'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

interface DepositRecord {
    id: number
    amount: number
    type: string
    created_at: string
    status?: string
}

interface AmountInputDetail {
    value: string
}

interface AmountInputDataset {
    field?: 'rechargeAmount'
}

interface DepositBalanceResponse {
    total_deposit: number
    frozen_deposit: number
    available_deposit: number
}

interface DepositListResponse {
    deposits?: DepositRecord[]
}

interface DepositPayResponse {
    pay_params?: WechatMiniprogram.RequestPaymentOption
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
    
    // 账户余额数据
    totalDeposit: 0,
    frozenDeposit: 0,
    availableDeposit: 0,
    
    // 提现/充值 状态
    transactions: [] as DepositRecord[],
    pageID: 1,
    hasMore: true,
    
    // 弹窗控制
    isRechargeVisible: false,
    rechargeAmount: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.refreshAccount()
    this.loadTransactions()
  },

  async refreshAccount() {
    this.setData({ loading: true })
    try {
        const balance = await RiderService.request('/v1/rider/deposit', 'GET') as DepositBalanceResponse
        this.setData({
            totalDeposit: balance.total_deposit,
            frozenDeposit: balance.frozen_deposit,
            availableDeposit: balance.available_deposit
        })
    } catch (err) {
        logger.error('Fetch deposit balance failed', err)
    } finally {
        this.setData({ loading: false })
    }
  },

  async loadTransactions() {
    try {
        const resp = await RiderService.request('/v1/rider/deposits', 'GET', { 
            page: this.data.pageID, 
            limit: 20 
        }) as DepositListResponse
        const list = resp.deposits || []
        this.setData({
            transactions: this.data.pageID === 1 ? list : [...this.data.transactions, ...list],
            hasMore: list.length === 20
        })
    } catch (err) {
        logger.error('Fetch deposit logs failed', err)
    }
  },

  onShowRecharge() {
    this.setData({ isRechargeVisible: true, rechargeAmount: '' })
  },

    onInputAmount(e: WechatMiniprogram.CustomEvent<AmountInputDetail>) {
        const { field } = e.currentTarget.dataset as AmountInputDataset
        if (!field) return
    this.setData({ [field]: e.detail.value })
  },

  onCloseDialog() {
    this.setData({
                isRechargeVisible: false
    })
  },

  /**
   * 提交充值
   */
  async confirmRecharge() {
    const amount = parseFloat(this.data.rechargeAmount)
    if (isNaN(amount) || amount <= 0) {
        wx.showToast({ title: '请输入正确金额', icon: 'none' })
        return
    }

    wx.showLoading({ title: '正在发起支付...' })
    try {
        const res = await RiderService.request('/v1/rider/deposit', 'POST', {
            amount: Math.round(amount * 100) // 转为分
        }) as DepositPayResponse
        
        if (res.pay_params) {
            wx.requestPayment({
                ...res.pay_params,
                success: () => {
                    wx.showToast({ title: '充值成功', icon: 'success' })
                    this.setData({ isRechargeVisible: false })
                    this.refreshAccount()
                    this.setData({ pageID: 1 }, () => this.loadTransactions())
                },
                fail: () => {
                    wx.showToast({ title: '已取消支付', icon: 'none' })
                }
            })
        }
    } catch (err: unknown) {
        const userMessage = (err as UserMessageError).userMessage
        const message = typeof userMessage === 'string' && userMessage ? userMessage : '充值失败'
        wx.showToast({ title: message, icon: 'none' })
    } finally {
        wx.hideLoading()
    }
  },

  onReachBottom() {
    if (this.data.hasMore) {
        this.setData({ pageID: this.data.pageID + 1 }, () => this.loadTransactions())
    }
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
