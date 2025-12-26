import { isLargeScreen } from '@/utils/responsive'

Page({
  data: {
    activeTab: 'ACTIVE' as 'ACTIVE' | 'INACTIVE' | 'EXPIRED',
    coupons: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadCoupons()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadCoupons() {
    this.setData({ loading: true })

    try {
      // Mock data - GET /api/v1/merchant/coupons?status=xxx
      const mockCoupons = [
        {
          id: 'coupon_1',
          name: '满30减5',
          type: 'CASH',
          threshold: 3000,
          discount: 500,
          total_count: 500,
          claimed_count: 125,
          used_count: 45,
          start_date: '2024-11-01',
          end_date: '2024-11-30',
          status: 'ACTIVE'
        },
        {
          id: 'coupon_2',
          name: '满50减10',
          type: 'CASH',
          threshold: 5000,
          discount: 1000,
          total_count: 300,
          claimed_count: 80,
          used_count: 30,
          start_date: '2024-11-01',
          end_date: '2024-11-30',
          status: 'ACTIVE'
        }
      ]

      this.setData({
        coupons: mockCoupons,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    this.loadCoupons()
  },

  onAddCoupon() {
    wx.navigateTo({ url: '/pages/merchant/marketing/coupons/edit/index' })
  },

  onEditCoupon(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/marketing/coupons/edit/index?id=${id}` })
  },

  onToggleStatus(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showModal({
      title: '状态变更',
      content: '确认变更优惠券状态?',
      success: async (res) => {
        if (res.confirm) {
          // PATCH /api/v1/merchant/coupons/{id}/toggle
          wx.showToast({ title: '状态已更新', icon: 'success' })
          this.loadCoupons()
        }
      }
    })
  }
})
