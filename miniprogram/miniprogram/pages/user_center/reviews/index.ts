/**
 * 我的评价页面
 * 使用真实后端API
 */

import { getMyReviews, ReviewResponse } from '../../../api/personal'

Page({
  data: {
    reviews: [] as Array<{
      id: number
      order_id: number
      merchant_id: number
      content: string
      images: string[]
      created_at: string
      reply?: string
      replied_at?: string
    }>,
    loading: false,
    navBarHeight: 88,
    activeTab: 0,
    page: 1,
    pageSize: 10,
    hasMore: true
  },

  onLoad() {
    this.loadReviews(true)
  },

  onShow() {
    // 返回时刷新
    if (this.data.reviews.length > 0) {
      this.loadReviews(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    this.loadReviews(true)
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.setData({ page: this.data.page + 1 })
      this.loadReviews(false)
    }
  },

  async loadReviews(reset = false) {
    if (this.data.loading) return
    this.setData({ loading: true })

    if (reset) {
      this.setData({ page: 1, reviews: [], hasMore: true })
    }

    try {
      const { page, pageSize } = this.data

      const result = await getMyReviews({
        page_id: page,
        page_size: pageSize
      })

      const reviews = (result.reviews || []).map((review: ReviewResponse) => ({
        id: review.id,
        order_id: review.order_id,
        merchant_id: review.merchant_id,
        content: review.content,
        images: review.images || [],
        created_at: review.created_at,
        reply: review.merchant_reply,
        replied_at: review.replied_at,
        is_visible: review.is_visible
      }))

      const hasMore = reviews.length === pageSize
      const newReviews = reset ? reviews : [...this.data.reviews, ...reviews]

      this.setData({
        reviews: newReviews,
        loading: false,
        hasMore
      })
    } catch (error) {
      console.error('加载评价失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  }
})
