import { getStableBarHeights } from '../../../utils/responsive'
import {
  createMerchantWithdraw,
  getMerchantAccountBalance,
  listMerchantWithdrawals,
  MerchantAccountBalanceResponse,
  MerchantWithdrawItem
} from '../../../api/merchant-finance'
import { logger } from '../../../utils/logger'

type InputChangeDetail = {
  value: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    balance: {
      sub_mch_id: '',
      available_amount: 0,
      pending_amount: 0,
      withdrawable_amount: 0
    } as MerchantAccountBalanceResponse,
    withdrawAmountYuan: '',
    withdrawRemark: '',
    withdrawals: [] as MerchantWithdrawItem[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadData()
  },

  onPullDownRefresh() {
    this.loadData()
  },

  async loadData() {
    this.setData({ loading: true })
    try {
      const [balance, records] = await Promise.all([
        getMerchantAccountBalance(),
        listMerchantWithdrawals(1, 20)
      ])

      this.setData({
        balance,
        withdrawals: records.withdrawals || []
      })
    } catch (error) {
      logger.error('Load merchant finance data failed', error, 'merchant-finance')
      wx.showToast({ title: '加载资金数据失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onWithdrawAmountChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawAmountYuan: e.detail.value })
  },

  onWithdrawRemarkChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawRemark: e.detail.value })
  },

  async onSubmitWithdraw() {
    if (this.data.submitting) return

    const amountYuan = Number(this.data.withdrawAmountYuan)
    if (!Number.isFinite(amountYuan) || amountYuan < 1) {
      wx.showToast({ title: '提现金额至少1元', icon: 'none' })
      return
    }

    if (!this.data.withdrawRemark.trim()) {
      wx.showToast({ title: '请输入提现备注', icon: 'none' })
      return
    }

    const amount = Math.round(amountYuan * 100)
    if (amount > this.data.balance.withdrawable_amount) {
      wx.showToast({ title: '超过可提现余额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await createMerchantWithdraw({
        amount,
        remark: this.data.withdrawRemark.trim()
      })

      wx.showToast({ title: '提现申请已提交', icon: 'success' })
      this.setData({
        withdrawAmountYuan: '',
        withdrawRemark: ''
      })
      await this.loadData()
    } catch (error) {
      logger.error('Submit merchant withdraw failed', error, 'merchant-finance')
      wx.showToast({ title: '提现申请失败', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  formatAmount(fen: number): string {
    return (fen / 100).toFixed(2)
  },

  getStatusText(status: string): string {
    switch (status) {
      case 'pending':
        return '处理中'
      case 'success':
        return '成功'
      case 'failed':
        return '失败'
      default:
        return status
    }
  },

  getStatusTheme(status: string): string {
    switch (status) {
      case 'pending':
        return 'warning'
      case 'success':
        return 'success'
      case 'failed':
        return 'danger'
      default:
        return 'default'
    }
  }
})
