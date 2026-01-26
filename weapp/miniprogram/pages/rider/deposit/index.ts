import RiderService, { RiderInfo } from '../../../api/rider'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    
    // 账户余额数据
    totalDeposit: 0,
    frozenDeposit: 0,
    availableDeposit: 0,
    
    // 提现/充值 状态
    transactions: [] as any[],
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
        const balance = await RiderService.request('/v1/rider/deposit', 'GET')
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
        })
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

  onInputAmount(e: any) {
    const { field } = e.currentTarget.dataset
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
        })
        
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
    } catch (err: any) {
        wx.showToast({ title: err.userMessage || '充值失败', icon: 'none' })
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
    } catch (err: any) {
        wx.showToast({ title: err.userMessage || '提现失败', icon: 'none' })
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
