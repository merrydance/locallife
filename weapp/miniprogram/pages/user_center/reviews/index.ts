import ReviewService, { Review } from '../../../api/review'
import ConsumerProfileAdapter from '../../../adapters/consumer-profile'
import { formatTime } from '../../../utils/util'
import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import Navigation from '../../../utils/navigation'

interface IdDataset {
  id?: number
}

interface PreviewDataset {
  reviewId?: number
  imageIndex?: number
}

interface ReviewDisplay {
  id: number
  orderId: number
  orderIdLabel: string
  merchantId: number
  merchantName: string
  logoUrl: string
  content: string
  images: string[]
  createdAt: string
  visibilityLabel: string
  visibilityTheme: 'success' | 'warning'
  merchantReply?: string
  repliedAt?: string
}

function normalizeReviewImages(review: Review): string[] {
  const imageUrls = Array.isArray(review.image_urls)
    ? review.image_urls
    : Array.isArray(review.imageUrls)
      ? review.imageUrls
      : Array.isArray(review.images)
        ? review.images
        : []

  return imageUrls.filter((url): url is string => typeof url === 'string' && url.length > 0)
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
    hasMore: true,
    error: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadReviews(true)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    if (e.detail.navBarHeight) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  async loadReviews(reset = false) {
    if (this.data.loading && !this.data.initialLoading && !this.data.refreshing) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true, error: reset ? '' : this.data.error })

    try {
      const page = reset ? 1 : this.data.page
      const res = await ReviewService.listMyReviews(page, this.data.pageSize)

      const newReviews: ReviewDisplay[] = res.reviews.map((r: Review) => ({
        ...ConsumerProfileAdapter.toReviewMerchantViewModel(r),
        id: r.id,
        orderId: r.order_id,
        orderIdLabel: `订单 #${r.order_id}`,
        content: r.content,
        images: normalizeReviewImages(r),
        createdAt: formatTime(new Date(r.created_at)),
        visibilityLabel: r.is_visible ? '公开展示' : '安全审核中',
        visibilityTheme: r.is_visible ? 'success' : 'warning',
        merchantReply: r.merchant_reply,
        repliedAt: r.replied_at ? formatTime(new Date(r.replied_at)) : undefined
      }))

      const reviews = reset ? newReviews : [...this.data.reviews, ...newReviews]

      this.setData({
        reviews,
        page: res.page + 1,
        hasMore: res.hasMore,
        loading: false,
        initialLoading: false,
        refreshing: false,
        error: ''
      })
    } catch (error) {
      logger.error('加载我的评价失败', error, 'Reviews.listMyReviews')
      const errorMessage = getErrorUserMessage(error, '评价列表加载失败，请稍后重试')
      if (!reset) {
        wx.showToast({ title: errorMessage, icon: 'none' })
      }
      this.setData({ 
        loading: false,
        initialLoading: false,
        refreshing: false,
        error: reset ? errorMessage : this.data.error
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
    const { id } = e.currentTarget.dataset as IdDataset
    if (!id) return
    Navigation.toRestaurantDetail(id)
  },

  onOrderDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as IdDataset
    if (!id) return
    Navigation.toOrderDetail(String(id))
  },

  onImagePreview(e: WechatMiniprogram.TouchEvent) {
    const { reviewId, imageIndex = 0 } = e.currentTarget.dataset as PreviewDataset
    if (!reviewId) return

    const review = this.data.reviews.find((item) => item.id === reviewId)
    if (!review || review.images.length === 0) return

    const current = review.images[imageIndex] || review.images[0]
    wx.previewImage({
      urls: review.images,
      current
    })
  },

  onRetry() {
    this.loadReviews(true)
  },

  onGoHome() {
    Navigation.toTakeoutHome()
  }
})
