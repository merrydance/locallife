import { request } from '../../../../utils/request'

type RiderDetail = {
  id: number
  user_id: number
  real_name: string
  phone: string
  id_card_no?: string
  status: string
  is_online: boolean
  region_id: number
  deposit_amount: number
  frozen_deposit: number
  total_orders: number
  total_earnings: number
  current_latitude?: number
  current_longitude?: number
  location_updated_at?: string
  credit_score: number
  high_value_qualified: boolean
  created_at: string
  updated_at: string
}

type RiderDetailView = RiderDetail & {
  deposit_amount_display: string
  frozen_deposit_display: string
  total_earnings_display: string
}

Page({
  data: {
    id: 0,
    loading: true,
    error: '',
    navBarHeight: 88,
    detail: null as RiderDetailView | null
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '骑手ID无效' })
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
      const detail = await request<RiderDetail>({
        url: `/v1/operator/riders/${this.data.id}`,
        method: 'GET'
      })
      const detailView: RiderDetailView = {
        ...detail,
        deposit_amount_display: (Number(detail.deposit_amount || 0) / 100).toFixed(2),
        frozen_deposit_display: (Number(detail.frozen_deposit || 0) / 100).toFixed(2),
        total_earnings_display: (Number(detail.total_earnings || 0) / 100).toFixed(2)
      }
      this.setData({ detail: detailView, loading: false })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载骑手详情失败'
      this.setData({ loading: false, error: message })
    }
  },

  onRetry() {
    this.loadDetail()
  }
})
