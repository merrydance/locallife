/**
 * 运营商户管理页面
 * 使用真实后端API
 */

import { isLargeScreen } from '@/utils/responsive'
import { operatorMerchantManagementService, OperatorMerchantItem } from '../../../api/operator-merchant-management'

interface OperatorMerchantView {
  id: number
  name: string
  phone: string
  status: string
  region_id: number
  created_at: string
  is_open: boolean
  address: string
}

Page({
  data: {
    merchants: [] as OperatorMerchantView[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    page: 1,
    hasMore: true
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadMerchants()
  },

  onShow() {
    // 返回时静默刷新
    if (!this.data.initialLoading) {
      this.loadMerchants(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadMerchants(reset = true, silent = false) {
    if (this.data.loading && !this.data.initialLoading) return

    if (reset) {
      if (!silent) {
        this.setData({ initialLoading: true, error: null })
      }
      this.setData({ page: 1, merchants: [], hasMore: true })
    }

    this.setData({ loading: true, error: null })

    try {
      const result = await operatorMerchantManagementService.getMerchantList({
        page_id: this.data.page,
        page_size: 20
      })

      // 直接使用后端返回的数据，不添加假数据
      // 遵循 SSOT 原则：所有数据应来自后端
      const merchants = (result.merchants || []).map((m: OperatorMerchantItem) => ({
        id: m.id,
        name: m.name,
        phone: m.phone,
        status: m.status?.toUpperCase() || 'UNKNOWN',
        region_id: m.region_id,
        created_at: m.created_at,
        is_open: m.is_open,
        address: m.address
      }))

      const newMerchants = reset ? merchants : [...this.data.merchants, ...merchants]

      this.setData({
        merchants: newMerchants,
        hasMore: merchants.length === 20,
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      console.error('加载商户列表失败:', error)
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载商户列表失败'
      })
    }
  },

  onRetry() {
    this.loadMerchants(true)
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
