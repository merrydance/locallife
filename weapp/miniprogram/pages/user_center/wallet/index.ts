Page({
  data: {
    balance: 0,
    transactions: [] as Array<{
            id: string
            type: 'PAYMENT' | 'REFUND' | 'TOPUP'
            amount: number
            title: string
            time: string
        }>,
    loading: false,
    navBarHeight: 88
  },

  onLoad() {
    this.loadWallet()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadWallet() {
    this.setData({ loading: true })
    try {
      // Mock data - GET /api/v1/customers/wallet
      const mockWallet = {
        balance: 5800,
        transactions: [
          {
            id: 'tx_1',
            type: 'PAYMENT' as const,
            amount: -3800,
            title: '外卖订单支付',
            time: '2024-11-19 18:30'
          },
          {
            id: 'tx_2',
            type: 'REFUND' as const,
            amount: 1200,
            title: '订单退款',
            time: '2024-11-18 10:00'
          },
          {
            id: 'tx_3',
            type: 'TOPUP' as const,
            amount: 10000,
            title: '余额充值',
            time: '2024-11-15 09:00'
          }
        ]
      }
      this.setData({
        balance: mockWallet.balance,
        transactions: mockWallet.transactions,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onTopUp() {
    wx.showToast({ title: '充值功能开发中', icon: 'none' })
  },

  onWithdraw() {
    wx.showToast({ title: '提现功能开发中', icon: 'none' })
  }
})
