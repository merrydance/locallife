import { isLargeScreen } from '@/utils/responsive'

Page({
  data: {
    automations: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadAutomations()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadAutomations() {
    this.setData({ loading: true })
    try {
      // Mock data - GET /api/v1/operator/automations
      const mockAutomations = [
        {
          id: 'auto_1',
          name: '超时自动取消订单',
          desc: '顾客下单后15分钟未支付，自动取消订单',
          status: 'ACTIVE',
          trigger: 'ORDER_CREATED',
          condition: 'UNPAID_DURATION > 15min',
          action: 'CANCEL_ORDER'
        },
        {
          id: 'auto_2',
          name: '骑手超时自动惩罚',
          desc: '骑手接单后超过预计送达时间30分钟未送达，自动扣除信用分',
          status: 'INACTIVE',
          trigger: 'ORDER_DELIVERING',
          condition: 'DELAY > 30min',
          action: 'DEDUCT_CREDIT'
        }
      ]
      this.setData({
        automations: mockAutomations,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onToggleStatus(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail
    wx.showToast({ title: '状态已更新', icon: 'success' })
    // Mock toggle
    const newList = this.data.automations.map((item) => {
      if (item.id === id) {
        return { ...item, status: item.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE' }
      }
      return item
    })
    this.setData({ automations: newList })
  }
})
