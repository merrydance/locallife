import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { getMyMerchantProfile } from '../../../api/merchant'
import ReviewService, { Review } from '../_main_shared/api/review'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantReviewManagementAccess } from '../../../utils/console-access'
import { getMerchantReviewReplyErrorMessage } from '../_utils/merchant-review-reply-error'

const REVIEWS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

interface MerchantReviewCardView {
  id: number
  orderId: number
  orderIdLabel: string
  content: string
  imageUrls: string[]
  isVisible: boolean
  visibilityLabel: string
  visibilityTheme: 'primary' | 'warning'
  replyStatusLabel: string
  replyStatusTheme: 'success' | 'warning'
  merchantReply: string
  hasReply: boolean
  repliedAtLabel: string
  createdAtLabel: string
  replyHint: string
  replyActionLabel: string
}

interface PreviewDataset {
  urls?: string[]
  current?: string
}

interface ReviewDataset {
  id?: number
}

function formatTime(value?: string) {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function buildReplyHint(review: Review) {
  if (review.merchant_reply) {
    return review.is_visible ? '已回复顾客评价，可继续补充说明。' : '该评价当前已被平台隐藏，如需补充说明仍可更新回复。'
  }

  return review.is_visible
    ? '建议尽快回复顾客反馈，减少后续咨询成本。'
    : '该评价当前不对外展示，但仍建议回复说明处理结果。'
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function toMerchantReviewCard(review: Review, previous?: MerchantReviewCardView): MerchantReviewCardView {
  const imageUrls = Array.isArray(review.images)
    ? review.images
    : Array.isArray(review.image_urls)
      ? review.image_urls
      : previous?.imageUrls || []
  const merchantReply = review.merchant_reply || ''
  const hasReply = Boolean(merchantReply)

  return {
    id: review.id,
    orderId: review.order_id,
    orderIdLabel: `订单 #${review.order_id}`,
    content: review.content || '',
    imageUrls,
    isVisible: Boolean(review.is_visible),
    visibilityLabel: review.is_visible ? '平台可见' : '平台隐藏',
    visibilityTheme: review.is_visible ? 'primary' : 'warning',
    replyStatusLabel: hasReply ? '已回复' : '待回复',
    replyStatusTheme: hasReply ? 'success' : 'warning',
    merchantReply,
    hasReply,
    repliedAtLabel: hasReply ? formatTime(review.replied_at) : '暂无',
    createdAtLabel: formatTime(review.created_at),
    replyHint: buildReplyHint(review),
    replyActionLabel: hasReply ? '更新回复' : '回复评价'
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessDeniedMessage: '',
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    actionNoticeMessage: '',
    loading: false,
    loadingMore: false,
    lastLoadedAt: 0,
    merchantId: 0,
    merchantName: '',
    reviews: [] as MerchantReviewCardView[],
    total: 0,
    page: 0,
    pageSize: 20,
    hasMore: false,
    replyPopupVisible: false,
    activeReviewId: 0,
    activeReviewHasReply: false,
    replyDraft: '',
    replySubmitting: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantReviewManagementAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadReviews({ reset: true })
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessDeniedMessage: '', accessErrorMessage: '', initialLoading: true })
    this.onLoad()
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    if (
      !this.data.initialLoading
      && !this.data.loading
      && !this.data.loadingMore
      && shouldAutoRefresh(this.data.lastLoadedAt, REVIEWS_AUTO_REFRESH_WINDOW_MS)
    ) {
      this.loadReviews({ reset: true, silent: true })
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadReviews({ reset: true, silent: this.data.reviews.length > 0 || this.data.lastLoadedAt > 0, force: true })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadReviews({ reset: true })
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadReviews({ reset: true, silent: true, force: true })
  },

  onLoadMore() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.hasMore || this.data.loadingMore) return
    this.loadReviews({ reset: false })
  },

  onPreviewImage(e: WechatMiniprogram.TouchEvent) {
    const { urls, current } = e.currentTarget.dataset as PreviewDataset
    if (!urls || !urls.length) return

    wx.previewImage({
      urls,
      current: current || urls[0]
    })
  },

  onOpenReplyPopup(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as ReviewDataset
    if (!id) return

    const current = this.data.reviews.find((item) => item.id === id)
    if (!current) return

    this.setData({
      replyPopupVisible: true,
      activeReviewId: id,
      activeReviewHasReply: current.hasReply,
      replyDraft: current.merchantReply,
      actionNoticeMessage: ''
    })
  },

  onCloseReplyPopup() {
    if (this.data.replySubmitting) return
    this.setData({
      replyPopupVisible: false,
      activeReviewId: 0,
      activeReviewHasReply: false,
      replyDraft: ''
    })
  },

  onReplyPopupVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    if (!e.detail.visible) {
      this.onCloseReplyPopup()
    }
  },

  onReplyInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ replyDraft: e.detail.value || '' })
  },

  async onSubmitReply() {
    if (this.data.replySubmitting) return

    const reply = this.data.replyDraft.trim()
    if (!reply) {
      wx.showToast({ title: '请输入回复内容', icon: 'none' })
      return
    }
    if (reply.length > 500) {
      wx.showToast({ title: '回复内容需控制在 500 字内', icon: 'none' })
      return
    }

    this.setData({ replySubmitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const review = await ReviewService.replyToReview(this.data.activeReviewId, { reply })
      const nextReviews = this.data.reviews.map((item) => {
        if (item.id !== review.id) return item
        return toMerchantReviewCard(review, item)
      })

      this.setData({
        reviews: nextReviews,
        replyPopupVisible: false,
        activeReviewId: 0,
        activeReviewHasReply: false,
        replyDraft: '',
        refreshErrorMessage: '',
        actionNoticeMessage: '商户回复已更新，顾客侧会按最新内容展示。'
      })
    } catch (err) {
      logger.error('Reply merchant review failed', err)
      wx.showToast({ title: getMerchantReviewReplyErrorMessage(err), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ replySubmitting: false })
    }
  },

  async ensureMerchantContext() {
    if (this.data.merchantId > 0) {
      return this.data.merchantId
    }

    const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number, name?: string } | null
    const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
    if (cachedMerchantId > 0) {
      this.setData({
        merchantId: cachedMerchantId,
        merchantName: cached?.name || this.data.merchantName
      })
      return cachedMerchantId
    }

    const profile = await getMyMerchantProfile()
    this.setData({
      merchantId: profile.id,
      merchantName: profile.name || ''
    })

    try {
      const currentMerchant = wx.getStorageSync('current_merchant') || {}
      wx.setStorageSync('current_merchant', {
        ...currentMerchant,
        id: profile.id,
        merchant_id: profile.id,
        name: profile.name
      })
    } catch (storageErr) {
      logger.warn('Sync merchant cache for reviews failed', storageErr)
    }

    return profile.id
  },

  async loadReviews(options: { reset: boolean, silent?: boolean, force?: boolean }) {
    const { reset, silent = false, force = false } = options
    if (reset && this.data.loading) return
    if (!reset && (this.data.loadingMore || !this.data.hasMore)) return

    const nextPage = reset ? 1 : this.data.page + 1
    const hasExistingReviews = this.data.reviews.length > 0 || this.data.lastLoadedAt > 0

    if (reset && !force && hasExistingReviews && !shouldAutoRefresh(this.data.lastLoadedAt, REVIEWS_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData(reset
      ? (silent && hasExistingReviews
          ? {
              loading: true,
              refreshErrorMessage: ''
            }
          : {
              loading: true,
              initialError: false,
              initialErrorMessage: '',
              refreshErrorMessage: '',
              actionNoticeMessage: '',
              reviews: [],
              total: 0,
              page: 0,
              hasMore: false
            })
      : {
          loadingMore: true
        })

    try {
      const merchantId = await this.ensureMerchantContext()
      const response = await ReviewService.listMerchantAllReviews(merchantId, nextPage, this.data.pageSize)
      const nextItems = Array.isArray(response.reviews)
        ? response.reviews.map((item) => toMerchantReviewCard(item))
        : []
      const reviews = reset ? nextItems : this.data.reviews.concat(nextItems)

      this.setData({
        reviews,
        total: response.total,
        page: response.page,
        hasMore: response.hasMore,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load merchant reviews failed', err)
      const message = getErrorMessage(err, '评价列表加载失败，请稍后重试')

      if (reset) {
        if (silent && hasExistingReviews) {
          this.setData({
            initialLoading: false,
            refreshErrorMessage: `${message}，当前已保留上次同步结果`
          })
        } else {
          this.setData({
            initialLoading: false,
            initialError: true,
            initialErrorMessage: message,
            reviews: [],
            total: 0,
            page: 0,
            hasMore: false
          })
        }
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
      wx.stopPullDownRefresh()
    }
  }
})
