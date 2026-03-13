/**
 * 运营商户管理页面
 * 使用真实后端API
 */

import { isLargeScreen } from '@/utils/responsive'
import { operatorMerchantManagementService, OperatorMerchantItem } from '../../../api/operator-merchant-management'

const MERCHANT_STATUS_LABEL: Record<string, string> = {
  active: '正常营业',
  approved: '正常营业',
  suspended: '已暂停',
  pending: '待审核',
  rejected: '审核拒绝',
  closed: '已关闭'
}

interface OperatorMerchantView {
  id: number
  name: string
  phone: string
  status: string
  status_label: string
  region_id: number
  created_at: string
  is_open: boolean
  address: string
  owner_user_id: number
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
    hasMore: true,
    regionId: 0
  },

  onLoad(options: { region_id?: string }) {
    const regionId = options.region_id ? parseInt(options.region_id) : 0
    this.setData({ isLargeScreen: isLargeScreen(), regionId })
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
        page: this.data.page,
        limit: 20,
        ...(this.data.regionId ? { region_id: this.data.regionId } : {})
      })

      // 直接使用后端返回的数据，不添加假数据
      // 遵循 SSOT 原则：所有数据应来自后端
      const merchants = (result.merchants || []).map((m: OperatorMerchantItem) => ({
        id: m.id,
        name: m.name,
        phone: m.phone,
        status: m.status || 'unknown',
        status_label: MERCHANT_STATUS_LABEL[m.status] || m.status,
        region_id: m.region_id,
        created_at: m.created_at,
        is_open: m.is_open,
        address: m.address,
        owner_user_id: m.owner_user_id
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

  onViewDetail(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/operator/merchants/detail/index?id=${id}` })
  }
})
