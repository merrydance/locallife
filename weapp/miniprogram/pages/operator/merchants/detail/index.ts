import { operatorMerchantManagementService } from '../../../../api/operator-merchant-management'

type MerchantDetail = {
  id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  address: string
  status: string
  is_open: boolean
  owner_user_id: number
  region_id: number
  latitude: number
  longitude: number
  created_at: string
  updated_at: string
}

Page({
  data: {
    id: 0,
    loading: true,
    error: '',
    navBarHeight: 88,
    detail: null as MerchantDetail | null
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '商户ID无效' })
      return
    }
    this.setData({ id })
    this.loadDetail()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadDetail() {
    if (!this.data.id) return
    this.setData({ loading: true, error: '' })
    try {
      const detail = await operatorMerchantManagementService.getMerchantDetail(this.data.id) as unknown as MerchantDetail
      this.setData({ detail, loading: false })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载商户详情失败'
      this.setData({ loading: false, error: message })
    }
  },

  onRetry() {
    this.loadDetail()
  }
})
