import { operatorBasicManagementService } from '../../../../api/operator-basic-management'

type SafetyStatusFilter = '' | 'pending' | 'resolved' | 'rejected'

interface TabChangeDetail {
  value: SafetyStatusFilter
}

Page({
  data: {
    reports: [] as Array<{
      id: number
      title: string
      level: 'low' | 'medium' | 'high' | 'critical'
      status: 'pending' | 'resolved' | 'rejected'
      created_at: string
    }>,
    loading: false,
    initialLoading: true,
    navBarHeight: 88,
    status: '' as SafetyStatusFilter,
    page: 1,
    limit: 20,
    hasMore: false
  },

  onLoad() {
    this.loadReports(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadReports(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadReports(reset = false) {
    if (this.data.loading) return
    const nextPage = reset ? 1 : this.data.page
    this.setData({ loading: true })

    try {
      const res = await operatorBasicManagementService.getSafetyReports({
        page: nextPage,
        limit: this.data.limit,
        status: this.data.status || undefined
      })
      const current = reset ? [] : this.data.reports
      this.setData({
        reports: [...current, ...(res.items || [])],
        page: nextPage + 1,
        hasMore: Boolean(res.has_more),
        loading: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载食安事件失败'
      this.setData({ loading: false, initialLoading: false })
      wx.showToast({ title: message, icon: 'none' })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<TabChangeDetail>) {
    this.setData({ status: e.detail.value })
    this.loadReports(true)
  },

  onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    this.loadReports(false)
  },

  onDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/safety/detail/index?id=${id}` })
  }
})
