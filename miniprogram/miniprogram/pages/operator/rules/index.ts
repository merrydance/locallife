import { isLargeScreen } from '@/utils/responsive'

Page({
  data: {
    rules: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadRules()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRules() {
    this.setData({ loading: true })
    try {
      // Mock data - GET /api/v1/operator/rules
      const mockRules = [
        {
          id: 'rule_1',
          name: '商户入驻保证金',
          key: 'MERCHANT_DEPOSIT',
          value: '5000',
          unit: '元',
          desc: '商户入驻需缴纳的保证金金额'
        },
        {
          id: 'rule_2',
          name: '骑手入驻押金',
          key: 'RIDER_DEPOSIT',
          value: '500',
          unit: '元',
          desc: '骑手接单前需缴纳的押金金额'
        },
        {
          id: 'rule_3',
          name: '平台抽成比例',
          key: 'PLATFORM_COMMISSION',
          value: '15',
          unit: '%',
          desc: '每笔订单平台收取的服务费比例'
        }
      ]
      this.setData({
        rules: mockRules,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onEditRule(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    const rule = this.data.rules.find((r) => r.id === id)
    if (!rule) return

    wx.showModal({
      title: `修改${rule.name}`,
      content: rule.value,
      editable: true,
      placeholderText: '请输入新值',
      success: async (res) => {
        if (res.confirm && res.content) {
          // PATCH /api/v1/operator/rules/{id}
          wx.showToast({ title: '修改成功', icon: 'success' })
          this.loadRules()
        }
      }
    })
  }
})
