import ReviewService, { Review } from '../../../api/review'
import ConsumerProfileAdapter from '../../../adapters/consumer-profile'
import { formatTime } from '../../../utils/util'
import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import Navigation from '../../../utils/navigation'
import { getPublicImageUrl } from '../../../utils/image'

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
  orderNo: string
  orderIdLabel: string
  merchantId: number
  merchantName: string
  content: string
  images: string[]
  createdAt: string
  merchantReply?: string
  repliedAt?: string
  deleting?: boolean
}

function normalizeReviewImages(review: Review): string[] {
  const imageUrls = Array.isArray(review.image_urls)
    ? review.image_urls
    : Array.isArray(review.imageUrls)
      ? review.imageUrls
      : Array.isArray(review.images)
        ? review.images
        : []

  return imageUrls
    .filter((url): url is string => typeof url === 'string' && url.length > 0)
    .map((url) => getPublicImageUrl(url) || url)
}

function getReviewOrderNo(review: Review): string {
  return review.order_no || review.orderNo || ''
}

function getReviewOrderLabel(review: Review): string {
  const orderNo = getReviewOrderNo(review)
  return orderNo ? `订单号 ${orderNo}` : `订单 ID ${review.order_id}`
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
    error: '',
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogReviewId: 0,
    deleteDialogMerchantName: ''
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
        orderNo: r.order_no || r.orderNo || '',
        orderIdLabel: getReviewOrderLabel(r),
        content: r.content,
        images: normalizeReviewImages(r),
        createdAt: formatTime(new Date(r.created_at)),
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

  onEditReview(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as IdDataset
    if (!id) return
    wx.navigateTo({
      url: `/pages/user_center/reviews/create/index?reviewId=${id}`
    })
  },

  onDeleteReview(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as IdDataset
    if (!id || this.data.deleteDialogSubmitting) return

    const review = this.data.reviews.find((item) => item.id === id)
    if (!review) return

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogReviewId: id,
      deleteDialogMerchantName: review.merchantName || '该商家'
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) return
    this.setData({
      deleteDialogVisible: false,
      deleteDialogSubmitting: false,
      deleteDialogReviewId: 0,
      deleteDialogMerchantName: ''
    })
  },

  async onConfirmDeleteReview() {
    const id = Number(this.data.deleteDialogReviewId || 0)
    if (!id || this.data.deleteDialogSubmitting) {
      this.onCancelDeleteDialog()
      return
    }

    const pendingReviews = this.data.reviews.map((review) => (
      review.id === id ? { ...review, deleting: true } : review
    ))
    this.setData({
      reviews: pendingReviews,
      deleteDialogSubmitting: true
    })

    try {
      await ReviewService.deleteReview(id)
      this.setData({
        reviews: pendingReviews.filter((review) => review.id !== id),
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogReviewId: 0,
        deleteDialogMerchantName: ''
      })
      wx.showToast({ title: '评价已删除', icon: 'none' })
    } catch (error) {
      logger.error('删除评价失败', error, 'Reviews.deleteReview')
      this.setData({
        reviews: pendingReviews.map((review) => (
          review.id === id ? { ...review, deleting: false } : review
        )),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(error, '删除失败，请稍后重试'), icon: 'none' })
    }
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
  },

  onReviewUpdated() {
    this.loadReviews(true)
  }
})
