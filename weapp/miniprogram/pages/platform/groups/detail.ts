import { responsiveBehavior } from '@/utils/responsive'
import { getAdminApprovalStatusDisplay, type AdminApprovalTheme } from '@/adapters/admin-review'
import {
  platformManagementService,
  type AdminGroupApplicationItem
} from '@/api/platform-management'
import { getPrivateMediaUrl } from '@/utils/image-security'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type ImageTapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      url?: string
    }
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: true,
    requesting: false,
    submittingAction: '' as '' | 'approve' | 'reject',
    error: null as string | null,
    applicationID: 0,
    application: null as AdminGroupApplicationItem | null,
    licenseImageUrl: '',
    statusLabel: '',
    statusTheme: 'primary' as AdminApprovalTheme,
    showRejectReason: false,
    canReview: false,
    showRejectDialog: false,
    rejectReason: ''
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '申请ID无效' })
      return
    }

    this.setData({ applicationID: id })
    this.loadDetail()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadDetail() {
    if (this.data.requesting || !this.data.applicationID) return

    this.setData({ loading: true, requesting: true, error: null })
    try {
      const detail = await platformManagementService.getAdminGroupApplicationDetail(this.data.applicationID)
      const licenseImageUrl = detail.license_image_asset_id
        ? await getPrivateMediaUrl(detail.license_image_asset_id)
        : ''

      const status = getAdminApprovalStatusDisplay(detail.status, { unknownTheme: 'primary' })
      this.setData({
        application: detail,
        licenseImageUrl,
        statusLabel: status.label,
        statusTheme: status.theme,
        showRejectReason: status.isRejected && !!detail.reject_reason,
        canReview: status.isPending
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载详情失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  onPreviewImage(e: ImageTapEvent) {
    const url = String(e.currentTarget.dataset.url || '').trim()
    if (!url) return
    wx.previewImage({ urls: [url], current: url })
  },

  async onApproveTap() {
    const id = this.data.applicationID
    if (!id || this.data.submittingAction) return

    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '通过申请',
        content: '确认通过该集团入驻申请？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submittingAction: 'approve' })
      await platformManagementService.reviewAdminGroupApplication(id, { status: 'approved' })
      await this.loadDetail()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '审核失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submittingAction: '' })
    }
  },

  onRejectTap() {
    if (this.data.submittingAction) return
    this.setData({
      showRejectDialog: true,
      rejectReason: ''
    })
  },

  onRejectReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ rejectReason: e.detail.value || '' })
  },

  onRejectCancel() {
    this.setData({
      showRejectDialog: false,
      rejectReason: ''
    })
  },

  async onRejectConfirm() {
    const id = this.data.applicationID
    const reason = this.data.rejectReason.trim()
    if (!id || this.data.submittingAction) {
      this.onRejectCancel()
      return
    }
    if (!reason) {
      wx.showToast({ title: '请输入驳回原因', icon: 'none' })
      return
    }

    try {
      this.setData({ submittingAction: 'reject' })
      await platformManagementService.reviewAdminGroupApplication(id, {
        status: 'rejected',
        reject_reason: reason
      })
      this.onRejectCancel()
      await this.loadDetail()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '驳回失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submittingAction: '' })
    }
  }
})
