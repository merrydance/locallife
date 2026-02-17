import { request } from '../../../utils/request'

type RiderItem = {
  id: number
  user_id: number
  real_name: string
  phone: string
  status: string
  is_online: boolean
  region_id: number
  deposit_amount: number
  total_orders: number
  total_earnings: number
  created_at: string
}

type RiderItemView = RiderItem & {
  total_earnings_display: string
}

type RiderListResponse = {
  riders: RiderItem[]
  total: number
  page: number
  limit: number
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: '',
    page: 1,
    limit: 20,
    hasMore: true,
    riders: [] as RiderItemView[]
  },

  onLoad() {
    this.loadRiders(true)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.loadRiders(false)
    }
  },

  async loadRiders(reset: boolean) {
    if (this.data.loading) return
    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, error: '' })

    try {
      const res = await request<RiderListResponse>({
        url: '/v1/operator/riders',
        method: 'GET',
        data: { page, limit: this.data.limit }
      })

      const incoming = (res.riders || []).map((item) => ({
        ...item,
        total_earnings_display: (Number(item.total_earnings || 0) / 100).toFixed(2)
      }))
      const riders = reset ? incoming : [...this.data.riders, ...incoming]
      this.setData({
        riders,
        page,
        hasMore: incoming.length === this.data.limit,
        loading: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载骑手失败'
      this.setData({ loading: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadRiders(true)
  },

  onTapRider(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/riders/detail/index?id=${id}` })
  }
})
