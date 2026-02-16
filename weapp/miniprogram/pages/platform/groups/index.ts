import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type AdminGroupApplicationItem
} from '@/api/platform-management'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

function getGroupStatusLabel(status: string): string {
  if (status === 'approved') return '已通过'
  if (status === 'rejected') return '已驳回'
  if (status === 'submitted') return '待审核'
  if (status === 'draft') return '草稿'
  return status || '未知'
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    requesting: false,
    refreshing: false,
    error: null as string | null,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    applications: [] as AdminGroupApplicationItem[]
  },

  onLoad() {
    this.loadApplications(true)
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadApplications(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    await this.loadApplications(false)
  },

  async loadApplications(reset: boolean) {
    if (this.data.requesting) return

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })

    try {
      const response = await platformManagementService.getAdminGroupApplications({
        page,
        limit: this.data.limit
      })

      this.setData({
        applications: reset ? response.applications : this.data.applications.concat(response.applications),
        total: response.total,
        page: response.page,
        hasMore: response.has_more
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载集团申请失败，请稍后重试'
      this.setData({ error: message })
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadApplications(true)
  },

  getStatusLabel(status: string) {
    return getGroupStatusLabel(status)
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return

    wx.navigateTo({
      url: `/pages/platform/groups/detail?id=${id}`
    })
  }
})
