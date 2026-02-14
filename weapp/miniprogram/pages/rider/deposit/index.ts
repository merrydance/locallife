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
    field?: 'withdrawAmount' | 'rechargeAmount'
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
    isWithdrawVisible: false,
    withdrawAmount: '',
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

  onShowWithdraw() {
    this.setData({ isWithdrawVisible: true, withdrawAmount: '' })
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
        isRechargeVisible: false,
        isWithdrawVisible: false
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

  /**
   * 提交提现
   */
  async confirmWithdraw() {
    const amount = parseFloat(this.data.withdrawAmount)
    if (isNaN(amount) || amount <= 0) {
        wx.showToast({ title: '请输入正确金额', icon: 'none' })
        return
    }

    if (amount * 100 > this.data.availableDeposit) {
        wx.showToast({ title: '可用余额不足', icon: 'none' })
        return
    }

    wx.showLoading({ title: '提现处理中...' })
    try {
        await RiderService.request('/v1/rider/withdraw', 'POST', {
            amount: Math.round(amount * 100)
        })
        wx.showToast({ title: '提现申请已提交', icon: 'success' })
        this.setData({ isWithdrawVisible: false })
        this.refreshAccount()
        this.setData({ pageID: 1 }, () => this.loadTransactions())
    } catch (err: unknown) {
        const userMessage = (err as UserMessageError).userMessage
        const message = typeof userMessage === 'string' && userMessage ? userMessage : '提现失败'
        wx.showToast({ title: message, icon: 'none' })
    } finally {
        wx.hideLoading()
    }
  },

  onReachBottom() {
    if (this.data.hasMore) {
        this.setData({ pageID: this.data.pageID + 1 }, () => this.loadTransactions())
    }
  }
})
