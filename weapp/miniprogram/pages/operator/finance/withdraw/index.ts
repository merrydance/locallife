import { withdrawOperator } from '../../../../api/operator-finance'

interface AmountChangeDetail {
  value: string
}

Page({
  data: {
    amount: ''
  },

  onAmountChange(e: WechatMiniprogram.CustomEvent<AmountChangeDetail>) {
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
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '提现失败'
      wx.showToast({ title: message, icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  }
})
