/**
 * 运营商户管理页面
 * 使用真实后端API
 */

import { isLargeScreen } from '@/utils/responsive'
import { operatorMerchantManagementService, OperatorMerchantItem } from '../../../api/operator-merchant-management'

Page({
  data: {
    merchants: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    page: 1,
    hasMore: true
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadMerchants()
  },

  onShow() {
    // 返回时刷新
    if (this.data.merchants.length > 0) {
      this.loadMerchants()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadMerchants(reset = true) {
    if (reset) {
      this.setData({ page: 1, merchants: [], hasMore: true })
    }

    this.setData({ loading: true })

    try {
      const result = await operatorMerchantManagementService.getMerchantList({
        page_id: this.data.page,
        page_size: 20
      })

      const merchants = (result.merchants || []).map((m: OperatorMerchantItem) => ({
        id: m.id,
        name: m.name,
        phone: m.phone,
        status: m.status?.toUpperCase() || 'UNKNOWN',
        region_id: m.region_id,
        created_at: m.created_at
      }))

      const newMerchants = reset ? merchants : [...this.data.merchants, ...merchants]

      this.setData({
        merchants: newMerchants,
        hasMore: merchants.length === 20,
        loading: false
      })
    } catch (error) {
      console.error('加载商户列表失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.setData({ page: this.data.page + 1 })
      this.loadMerchants(false)
    }
  },

  async onToggleStatus(e: WechatMiniprogram.CustomEvent) {
    const { id, status } = e.currentTarget.dataset
    const isActive = status === 'ACTIVE'
    const action = isActive ? '封禁' : '解封'

    wx.showModal({
      title: '确认操作',
      content: `确认${action}该商户?`,
      success: async (res) => {
        if (res.confirm) {
          try {
            if (isActive) {
              await operatorMerchantManagementService.suspendMerchant(id, { reason: '运营封禁' })
            } else {
              await operatorMerchantManagementService.resumeMerchant(id, { reason: '运营解封' })
            }
            wx.showToast({ title: '操作成功', icon: 'success' })
            this.loadMerchants()
          } catch (error) {
            console.error('操作失败:', error)
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  onViewDetail(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` })
  }
})
