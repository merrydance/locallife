import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type AdminRiderItem
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
      name?: string
    }
  }
}

function getRiderStatusLabel(status: string): string {
  if (status === 'approved') return '已通过'
  if (status === 'rejected') return '已驳回'
  if (status === 'pending' || status === 'submitted' || status === 'reviewing') return '待审核'
  return status || '未知'
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    requesting: false,
    refreshing: false,
    submitting: false,
    error: null as string | null,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    riders: [] as AdminRiderItem[],
    showRejectDialog: false,
    selectedID: 0,
    selectedName: '',
    rejectReason: ''
  },

  onLoad() {
    this.loadRiders(true)
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadRiders(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) {
      return
    }
    await this.loadRiders(false)
  },

  async loadRiders(reset: boolean) {
    if (this.data.requesting) {
      return
    }

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.getAdminRiders({
        page,
        limit: this.data.limit
      })

      this.setData({
        riders: reset ? response.riders : this.data.riders.concat(response.riders),
        total: response.total,
        page: response.page,
        hasMore: response.has_more
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载骑手列表失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadRiders(true)
  },

  getStatusLabel(status: string) {
    return getRiderStatusLabel(status)
  },

  async onApproveTap(e: TapEvent) {
    const riderID = Number(e.currentTarget.dataset.id || 0)
    if (!riderID || this.data.submitting) return

    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '通过申请',
        content: '确认通过该骑手申请？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      await platformManagementService.approveRider(riderID, {})
      await this.loadRiders(true)
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '审核失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onRejectTap(e: TapEvent) {
    const riderID = Number(e.currentTarget.dataset.id || 0)
    if (!riderID || this.data.submitting) return

    this.setData({
      selectedID: riderID,
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
    const riderID = this.data.selectedID
    const rejectionReason = this.data.rejectReason.trim()
    if (!riderID || this.data.submitting) {
      this.onRejectCancel()
      return
    }
    if (!rejectionReason) {
      wx.showToast({ title: '请输入驳回原因', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      await platformManagementService.rejectRider(riderID, { rejection_reason: rejectionReason })
      this.onRejectCancel()
      await this.loadRiders(true)
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '驳回失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
