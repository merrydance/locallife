import ReviewService, { Review } from '../../../api/review'
import { formatTime } from '../../../utils/util'
import { ErrorHandler } from '../../../utils/error-handler'

interface ReviewDisplay {
  id: number
  merchantId: number
  merchantName: string
  logoUrl: string
  content: string
  images: string[]
  createdAt: string
  isVisible: boolean
  merchantReply?: string
  repliedAt?: string
}

Page({
  data: {
    reviews: [] as ReviewDisplay[],
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    page: 1,
    pageSize: 10,
    hasMore: true
  },

  onLoad() {
    this.loadReviews(true)
  },

  onShow() {
    // If we have items, perform silent refresh
    if (this.data.reviews.length > 0) {
      // this.loadReviews(true) // Optional: might be too aggressive
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadReviews(reset = false) {
    if (this.data.loading && !this.data.initialLoading) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true, error: null })

    try {
      const page = reset ? 1 : this.data.page
      const res = await ReviewService.listMyReviews(page, this.data.pageSize)

      const newReviews: ReviewDisplay[] = res.reviews.map((r: Review) => ({
        id: r.id,
        merchantId: r.merchant_id,
        merchantName: `商户 ${r.merchant_id}`, // Backend missing name, placeholder
        logoUrl: '/assets/icons/shop.svg',      // Backend missing logo, placeholder
        content: r.content,
        images: r.images || [],
        createdAt: formatTime(new Date(r.created_at)),
        isVisible: r.is_visible,
        merchantReply: r.merchant_reply,
        repliedAt: r.replied_at ? formatTime(new Date(r.replied_at)) : undefined
      }))

      const reviews = reset ? newReviews : [...this.data.reviews, ...newReviews]

      this.setData({
        reviews,
        page: page + 1,
        hasMore: newReviews.length === this.data.pageSize, 
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      ErrorHandler.handle(error, 'Reviews.list')
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载评价列表失败'
      })
      if (reset) this.setData({ reviews: [] })
    }
  },

  onRetry() {
    this.loadReviews(true)
  },

  onReachBottom() {
    this.loadReviews()
  },

  onPullDownRefresh() {
    this.loadReviews(true).then(() => {
      wx.stopPullDownRefresh()
    })
  },

  onItemClick(e: WechatMiniprogram.BaseEvent) {
    // Navigate to merchant detail? Or Order detail?
    // Given we don't have review detail page, maybe Merchant.
    const id = e.currentTarget.dataset.id
    const item = this.data.reviews.find(r => r.id === id)
    if (item) {
        wx.navigateTo({
            url: `/pages/takeout/restaurant-detail/index?id=${item.merchantId}`
        })
    }
  },

  onImagePreview(e: WechatMiniprogram.BaseEvent) {
    const { urls, current } = e.currentTarget.dataset
    wx.previewImage({
        urls,
        current
    })
  },

  onGoHome() {
    wx.switchTab({ url: '/pages/takeout/index' })
  }
})
