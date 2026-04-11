import { responsiveBehavior } from '@/utils/responsive'
import { getAdminApprovalStatusDisplay, type AdminApprovalTheme } from '@/adapters/admin-review'
import {
  platformManagementService,
  type AdminGroupApplicationItem
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

type AdminGroupApplicationView = AdminGroupApplicationItem & {
  statusLabel: string
  statusTheme: AdminApprovalTheme
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
    applications: [] as AdminGroupApplicationView[]
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

      const tagged = response.applications.map((a) => {
        const statusDisplay = getAdminApprovalStatusDisplay(a.status, { unknownTheme: 'primary' })
        return {
          ...a,
          statusLabel: statusDisplay.label,
          statusTheme: statusDisplay.theme
        }
      })
      this.setData({
        applications: reset ? tagged : this.data.applications.concat(tagged),
        total: response.total,
        page: response.page,
        hasMore: response.has_more
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载集团申请失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadApplications(true)
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return

    wx.navigateTo({
      url: `/pages/platform/groups/detail?id=${id}`
    })
  }
})
