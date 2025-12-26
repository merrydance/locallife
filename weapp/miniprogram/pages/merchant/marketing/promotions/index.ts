import { isLargeScreen } from '@/utils/responsive'

Page({
  data: {
    promotions: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadPromotions()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadPromotions() {
    this.setData({ loading: true })

    try {
      // Mock data - GET /api/v1/merchant/promotions
      const mockPromotions = [
        {
          id: 'promo_1',
          name: '开业大酬宾',
          type: 'DISCOUNT',
          description: '全场8折',
          start_date: '2024-11-01',
          end_date: '2024-11-30',
          status: 'ACTIVE'
        },
        {
          id: 'promo_2',
          name: '新品尝鲜',
          type: 'GIFT',
          description: '点新品送饮料',
          start_date: '2024-11-15',
          end_date: '2024-11-20',
          status: 'EXPIRED'
        }
      ]

      this.setData({
        promotions: mockPromotions,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onAddPromotion() {
    wx.showToast({ title: '功能开发中', icon: 'none' })
  },

  onToggleStatus(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showToast({ title: '状态已更新', icon: 'success' })
    this.loadPromotions()
  }
})
