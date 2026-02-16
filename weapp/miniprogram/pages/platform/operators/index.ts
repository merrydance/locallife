import { responsiveBehavior } from '@/utils/responsive'
import { platformManagementService, type AdminOperatorApplicationItem } from '@/api/platform-management'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
      name?: string
    }
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    refreshing: false,
    submitting: false,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    applications: [] as AdminOperatorApplicationItem[],
    showRejectDialog: false,
    selectedID: 0,
    selectedName: '',
    rejectReason: ''
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
    if (!this.data.hasMore || this.data.loading) {
      return
    }
    await this.loadApplications(false)
  },

  async loadApplications(reset: boolean) {
    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true })
    try {
      const response = await platformManagementService.getAdminOperatorApplications({
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
      const message = error instanceof Error ? error.message : '加载申请失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async onApproveTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return

    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '通过申请',
        content: '通过后将创建运营商账号并开通对应区域权限。',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      await platformManagementService.approveOperatorApplication(id)
      wx.showToast({ title: '审核通过', icon: 'success' })
      await this.loadApplications(true)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '审核失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onRejectTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return

    this.setData({
      selectedID: id,
      selectedName: String(e.currentTarget.dataset.name || ''),
      rejectReason: '',
      showRejectDialog: true
    })
  },

  onRejectReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ rejectReason: e.detail.value || '' })
  },

  onRejectCancel() {
    this.setData({
      showRejectDialog: false,
      selectedID: 0,
      selectedName: '',
      rejectReason: ''
    })
  },

  async onRejectConfirm() {
    const id = this.data.selectedID
    const reason = this.data.rejectReason.trim()
    if (!id) {
      this.onRejectCancel()
      return
    }
    if (!reason) {
      wx.showToast({ title: '请输入驳回原因', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      await platformManagementService.rejectOperatorApplication(id, { reject_reason: reason })
      wx.showToast({ title: '已驳回', icon: 'success' })
      this.onRejectCancel()
      await this.loadApplications(true)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '驳回失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
