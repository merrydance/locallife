import ReviewService, { Review } from '../../../api/review'
import { formatTime } from '../../../utils/util'
import { getStableBarHeights } from '../../../utils/responsive'
import { ErrorHandler } from '../../../utils/error-handler'

interface MerchantDataset {
  id?: number
}

interface PreviewDataset {
  urls?: string[]
  current?: string
}

interface ReviewDisplay {
  id: number
  merchantId: number
  merchantName: string
  logoUrl: string
  content: string
  images: string[]
  rating: number
  tags: string[]
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
    refreshing: false,
    page: 1,
    pageSize: 10,
    hasMore: true
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadReviews(true)
  },

  async loadReviews(reset = false) {
    if (this.data.loading && !this.data.initialLoading && !this.data.refreshing) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true })

    try {
      const page = reset ? 1 : this.data.page
      const res = await ReviewService.listMyReviews(page, this.data.pageSize)

      const newReviews: ReviewDisplay[] = res.reviews.map((r: Review) => ({
        id: r.id,
        merchantId: r.merchant_id,
        merchantName: r.merchant_name || `商户 #${r.merchant_id}`,
        logoUrl: r.merchant_logo || '/assets/icons/shop.svg',
        rating: r.rating || 5,
        tags: r.tags || [],
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
        initialLoading: false,
        refreshing: false
      })
    } catch (error) {
      ErrorHandler.handle(error, 'Reviews.listMyReviews')
      this.setData({ 
        loading: false,
        initialLoading: false,
        refreshing: false
      })
      if (reset) this.setData({ reviews: [] })
    }
  },

  onReachBottom() {
    this.loadReviews()
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    this.loadReviews(true).then(() => {
      wx.stopPullDownRefresh()
    })
  },

  onMerchantClick(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as MerchantDataset
    if (!id) return
    wx.navigateTo({
        url: `/pages/takeout/restaurant-detail/index?id=${id}`
    })
  },

  onImagePreview(e: WechatMiniprogram.TouchEvent) {
    const { urls, current } = e.currentTarget.dataset as PreviewDataset
    if (!urls || urls.length === 0) return
    wx.previewImage({
        urls,
        current: current || urls[0]
    })
  },

  onGoHome() {
    wx.switchTab({ url: '/pages/takeout/index' })
  }
})
