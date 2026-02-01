import { withdrawOperator } from '../../../../api/operator-finance'

Page({
  data: {
    amount: '',
  },

  onAmountChange(e: any) {
    this.setData({ amount: e.detail.value })
  },

  async onSubmit() {
    const amount = parseFloat(this.data.amount)
    if (!amount || amount < 1) {
      wx.showToast({ title: '金额至少1元', icon: 'none' })
      return
    }

    try {
      wx.showLoading({ title: '提交中' })
      // 转换为分
      await withdrawOperator({ amount: Math.floor(amount * 100) })
      wx.showToast({ title: '提交成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1500)
    } catch (error: any) {
      wx.showToast({ title: error.message || '提现失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  }
})
